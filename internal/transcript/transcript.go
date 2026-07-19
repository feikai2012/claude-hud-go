// Package transcript ports src/transcript.ts: streaming the session JSONL and
// extracting tools, agents, todos, skills, MCP servers, token totals, session
// name, compaction markers, advisor model, and ultracode state — with an
// mtime+size keyed on-disk cache.
package transcript

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jarrodwatts/claude-hud-go/internal/config"
	"github.com/jarrodwatts/claude-hud-go/internal/types"
)

const (
	cacheVersion       = 12
	activityNameMaxLen = 64
	messageIDMaxLen    = 128
	seenMessageIDsMax  = 4096
	advisorModelMaxLen = 64
	maxTools           = 20
	maxAgents          = 10
)

var (
	mcpNamePattern    = regexp.MustCompile(`^mcp__(.+?)__(.+)$`)
	ctrlRe            = regexp.MustCompile(`[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]`)
	taskIDTag         = regexp.MustCompile(`<task-id>([^<]+)</task-id>`)
	toolUseIDTag      = regexp.MustCompile(`<tool-use-id>([^<]+)</tool-use-id>`)
	effortStdoutRe    = regexp.MustCompile(`^<local-command-stdout>Set effort level to (\w+)`)
	whitespaceRe      = regexp.MustCompile(`\s+`)
)

// line mirrors the union of transcript record shapes we read.
type line struct {
	Timestamp      string          `json:"timestamp"`
	Type           string          `json:"type"`
	Subtype        string          `json:"subtype"`
	Operation      string          `json:"operation"`
	Content        string          `json:"content"`
	Slug           string          `json:"slug"`
	CustomTitle    string          `json:"customTitle"`
	AdvisorModel   string          `json:"advisorModel"`
	Message        *message        `json:"message"`
	CompactMetadata *compactMeta   `json:"compactMetadata"`
	Attachment     *attachment     `json:"attachment"`
}

type message struct {
	ID      json.RawMessage `json:"id"`
	Content json.RawMessage `json:"content"` // array of blocks OR string
	Model   json.RawMessage `json:"model"`
	Usage   *usage          `json:"usage"`
}

type usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type compactMeta struct {
	Trigger    string `json:"trigger"`
	PreTokens  int    `json:"preTokens"`
	PostTokens *int   `json:"postTokens"`
	DurationMs int    `json:"durationMs"`
}

type attachment struct {
	Type string `json:"type"`
}

type contentBlock struct {
	Type      string                 `json:"type"`
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Input     map[string]interface{} `json:"input"`
	ToolUseID string                 `json:"tool_use_id"`
	IsError   bool                   `json:"is_error"`
}

// Parse reads and returns the transcript data (using the cache when fresh).
func Parse(transcriptPath string) types.TranscriptData {
	result := types.TranscriptData{
		Tools: []types.ToolEntry{}, Skills: []string{},
		McpServers: []string{}, Agents: []types.AgentEntry{}, Todos: []types.TodoItem{},
	}
	if transcriptPath == "" {
		return result
	}
	canonical, err := filepath.EvalSymlinks(transcriptPath)
	if err != nil {
		return result
	}
	fi, err := os.Stat(canonical)
	if err != nil || fi.IsDir() {
		return result
	}
	mtime := fi.ModTime().UnixMilli()
	size := fi.Size()

	if cached, ok := readCache(canonical, mtime, size); ok {
		return cached
	}

	data := parseFile(canonical)
	writeCache(canonical, mtime, size, data)
	return data
}

func parseFile(path string) types.TranscriptData {
	result := types.TranscriptData{
		Tools: []types.ToolEntry{}, Skills: []string{},
		McpServers: []string{}, Agents: []types.AgentEntry{}, Todos: []types.TodoItem{},
	}
	f, err := os.Open(path)
	if err != nil {
		return result
	}
	defer f.Close()

	toolMap := map[string]*types.ToolEntry{}
	var toolOrder []string
	skillSet := newOrderedSet()
	mcpSet := newOrderedSet()
	agentMap := map[string]*types.AgentEntry{}
	var agentOrder []string
	latestTodos := &[]types.TodoItem{}
	taskIDToIndex := map[string]int{}
	queueCompletion := map[string]time.Time{}
	var latestSlug, customTitle, latestAdvisor string
	var latestUltracode *bool
	var lastCompactBoundaryAt *time.Time
	var lastCompactPostTokens *int
	compactionCount := 0
	tokens := types.SessionTokenUsage{}
	seenMessageIDs := map[string]struct{}{}
	var seenOrder []string
	var lastUsageKey string

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		raw := sc.Text()
		if strings.TrimSpace(raw) == "" {
			lastUsageKey = ""
			continue
		}
		var e line
		if err := json.Unmarshal([]byte(raw), &e); err != nil {
			lastUsageKey = ""
			continue
		}

		if e.Type == "custom-title" && e.CustomTitle != "" {
			customTitle = e.CustomTitle
		} else if e.Slug != "" {
			latestSlug = e.Slug
		}
		if e.Type == "assistant" && e.AdvisorModel != "" {
			latestAdvisor = capStr(e.AdvisorModel, advisorModelMaxLen)
		}
		if e.Type == "attachment" && e.Attachment != nil {
			switch e.Attachment.Type {
			case "ultra_effort_enter":
				b := true
				latestUltracode = &b
			case "ultra_effort_exit":
				b := false
				latestUltracode = &b
			}
		}
		if e.Type == "user" && e.Message != nil {
			if s, ok := asString(e.Message.Content); ok {
				if m := effortStdoutRe.FindStringSubmatch(s); m != nil {
					b := strings.ToLower(m[1]) == "ultracode"
					latestUltracode = &b
				}
			}
		}
		if e.Type == "assistant" && e.Message != nil {
			if tm := sanitizeModel(rawToString(e.Message.Model)); tm != "" {
				result.LastAssistantModel = tm
			}
		}
		// Session token accumulation with dedup by message.id.
		if e.Type == "assistant" && e.Message != nil && e.Message.Usage != nil {
			u := e.Message.Usage
			msgID := normalizeMessageID(rawToString(e.Message.ID))
			shouldCount := false
			if msgID != "" {
				lastUsageKey = ""
				if _, seen := seenMessageIDs[msgID]; !seen {
					rememberID(seenMessageIDs, &seenOrder, msgID)
					shouldCount = true
				}
			} else {
				key := strconv.Itoa(u.InputTokens) + "|" + strconv.Itoa(u.OutputTokens) + "|" +
					strconv.Itoa(u.CacheCreationInputTokens) + "|" + strconv.Itoa(u.CacheReadInputTokens)
				shouldCount = key != lastUsageKey
				lastUsageKey = key
			}
			if shouldCount {
				tokens.InputTokens += maxInt(0, u.InputTokens)
				tokens.OutputTokens += maxInt(0, u.OutputTokens)
				tokens.CacheCreationTokens += maxInt(0, u.CacheCreationInputTokens)
				tokens.CacheReadTokens += maxInt(0, u.CacheReadInputTokens)
			}
		} else {
			lastUsageKey = ""
		}
		// Compaction markers.
		if e.Type == "system" && e.Subtype == "compact_boundary" {
			if ts, ok := parseTime(e.Timestamp); ok {
				compactionCount++
				if lastCompactBoundaryAt == nil || ts.After(*lastCompactBoundaryAt) {
					t := ts
					lastCompactBoundaryAt = &t
					if e.CompactMetadata != nil && e.CompactMetadata.PostTokens != nil && *e.CompactMetadata.PostTokens >= 0 {
						p := *e.CompactMetadata.PostTokens
						lastCompactPostTokens = &p
					} else {
						lastCompactPostTokens = nil
					}
				}
			}
		}
		// Background-agent completion timestamps from queue-operation enqueue.
		if e.Type == "queue-operation" && e.Operation == "enqueue" && e.Content != "" {
			tid := taskIDTag.FindStringSubmatch(e.Content)
			tuid := toolUseIDTag.FindStringSubmatch(e.Content)
			if tid != nil && tuid != nil {
				if ts, ok := parseTime(e.Timestamp); ok {
					queueCompletion[tuid[1]] = ts
				}
			}
		}

		processEntry(&e, toolMap, &toolOrder, skillSet, mcpSet, agentMap, &agentOrder, taskIDToIndex, latestTodos, &result)
	}

	// Resolve agent completion.
	for tuid, endTime := range queueCompletion {
		if a := agentMap[tuid]; a != nil && a.Background {
			t := endTime
			a.EndTime = &t
			a.Status = "completed"
		}
	}
	for _, a := range agentMap {
		if a.Status == "running" && a.EndTime != nil {
			a.Status = "completed"
		}
	}

	result.Tools = collectTools(toolMap, toolOrder)
	result.Skills = skillSet.values()
	result.McpServers = mcpSet.values()
	result.Agents = collectAgents(agentMap, agentOrder)
	result.Todos = *latestTodos
	if customTitle != "" {
		result.SessionName = customTitle
	} else {
		result.SessionName = latestSlug
	}
	result.SessionTokens = &tokens
	result.LastCompactBoundaryAt = lastCompactBoundaryAt
	result.LastCompactPostTokens = lastCompactPostTokens
	result.CompactionCount = compactionCount
	result.AdvisorModel = latestAdvisor
	result.UltracodeActive = latestUltracode
	return result
}

func processEntry(e *line, toolMap map[string]*types.ToolEntry, toolOrder *[]string,
	skillSet, mcpSet *orderedSet, agentMap map[string]*types.AgentEntry, agentOrder *[]string,
	taskIDToIndex map[string]int, latestTodos *[]types.TodoItem, result *types.TranscriptData) {

	ts, hasValid := parseTime(e.Timestamp)
	if !hasValid {
		ts = time.Now()
	}
	if result.SessionStart == nil && e.Timestamp != "" && hasValid {
		t := ts
		result.SessionStart = &t
	}
	if e.Type == "assistant" && e.Timestamp != "" && hasValid {
		t := ts
		result.LastAssistantResponseAt = &t
	}

	if e.Message == nil {
		return
	}
	blocks, ok := asBlocks(e.Message.Content)
	if !ok {
		return
	}
	for _, b := range blocks {
		if b.Type == "tool_use" && b.ID != "" && b.Name != "" {
			if b.Name == "Skill" {
				if sk := normalizeActivityName(mapString(b.Input, "skill")); sk != "" {
					skillSet.add(sk)
				}
			}
			if mcp := extractMcpServerName(b.Name); mcp != "" {
				mcpSet.add(mcp)
			}

			switch b.Name {
			case "Task", "Agent":
				a := &types.AgentEntry{
					ID:          b.ID,
					Type:        orDefault(mapString(b.Input, "subagent_type"), "agent"),
					Model:       mapString(b.Input, "model"),
					Description: mapString(b.Input, "description"),
					Status:      "running",
					StartTime:   ts,
					Background:  mapBool(b.Input, "run_in_background"),
				}
				if _, exists := agentMap[b.ID]; !exists {
					*agentOrder = append(*agentOrder, b.ID)
				}
				agentMap[b.ID] = a
			case "TodoWrite":
				applyTodoWrite(b.Input, taskIDToIndex, latestTodos)
			case "TaskCreate":
				applyTaskCreate(b, taskIDToIndex, latestTodos)
			case "TaskUpdate":
				applyTaskUpdate(b.Input, taskIDToIndex, latestTodos)
			default:
				if _, exists := toolMap[b.ID]; !exists {
					*toolOrder = append(*toolOrder, b.ID)
				}
				toolMap[b.ID] = &types.ToolEntry{
					ID: b.ID, Name: b.Name,
					Target: extractTarget(b.Name, b.Input), Status: "running", StartTime: ts,
				}
			}
		}
		if b.Type == "tool_result" && b.ToolUseID != "" {
			if t := toolMap[b.ToolUseID]; t != nil {
				if b.IsError {
					t.Status = "error"
				} else {
					t.Status = "completed"
				}
				end := ts
				t.EndTime = &end
			}
			if a := agentMap[b.ToolUseID]; a != nil && !a.Background {
				end := ts
				a.EndTime = &end
			}
		}
	}
}

func applyTodoWrite(input map[string]interface{}, taskIDToIndex map[string]int, latestTodos *[]types.TodoItem) {
	rawTodos, ok := input["todos"].([]interface{})
	if !ok {
		return
	}
	// Rebuild taskId↔index mapping by content, FIFO per content string.
	contentToTaskIDs := map[string][]string{}
	type pair struct {
		idx    int
		taskID string
	}
	var byOldIndex []pair
	for taskID, idx := range taskIDToIndex {
		if idx < len(*latestTodos) {
			byOldIndex = append(byOldIndex, pair{idx, taskID})
		}
	}
	sort.Slice(byOldIndex, func(i, j int) bool { return byOldIndex[i].idx < byOldIndex[j].idx })
	for _, p := range byOldIndex {
		c := (*latestTodos)[p.idx].Content
		contentToTaskIDs[c] = append(contentToTaskIDs[c], p.taskID)
	}

	newTodos := make([]types.TodoItem, 0, len(rawTodos))
	for _, rt := range rawTodos {
		m, _ := rt.(map[string]interface{})
		newTodos = append(newTodos, types.TodoItem{
			Content: str(m["content"]),
			Status:  str(m["status"]),
		})
	}
	*latestTodos = newTodos
	for k := range taskIDToIndex {
		delete(taskIDToIndex, k)
	}
	for i := range *latestTodos {
		ids := contentToTaskIDs[(*latestTodos)[i].Content]
		if len(ids) > 0 {
			taskIDToIndex[ids[0]] = i
			contentToTaskIDs[(*latestTodos)[i].Content] = ids[1:]
		}
	}
}

func applyTaskCreate(b contentBlock, taskIDToIndex map[string]int, latestTodos *[]types.TodoItem) {
	subject := mapString(b.Input, "subject")
	description := mapString(b.Input, "description")
	content := subject
	if content == "" {
		content = description
	}
	if content == "" {
		content = "Untitled task"
	}
	status := normalizeTaskStatus(mapString(b.Input, "status"))
	if status == "" {
		status = "pending"
	}
	*latestTodos = append(*latestTodos, types.TodoItem{Content: content, Status: status})
	taskID := mapTaskID(b.Input["taskId"])
	if taskID == "" {
		taskID = b.ID
	}
	if taskID != "" {
		taskIDToIndex[taskID] = len(*latestTodos) - 1
	}
}

func applyTaskUpdate(input map[string]interface{}, taskIDToIndex map[string]int, latestTodos *[]types.TodoItem) {
	idx, ok := resolveTaskIndex(input["taskId"], taskIDToIndex, *latestTodos)
	if !ok {
		return
	}
	if status := normalizeTaskStatus(mapString(input, "status")); status != "" {
		(*latestTodos)[idx].Status = status
	}
	subject := mapString(input, "subject")
	description := mapString(input, "description")
	content := subject
	if content == "" {
		content = description
	}
	if content != "" {
		(*latestTodos)[idx].Content = content
	}
}

func extractTarget(toolName string, input map[string]interface{}) string {
	if input == nil {
		return ""
	}
	switch toolName {
	case "Read", "Write", "Edit":
		if fp := mapString(input, "file_path"); fp != "" {
			return fp
		}
		return mapString(input, "path")
	case "Glob", "Grep":
		return mapString(input, "pattern")
	case "Skill":
		return normalizeActivityName(mapString(input, "skill"))
	case "Bash":
		cmd, ok := input["command"].(string)
		if !ok {
			return ""
		}
		cmd = strings.TrimSpace(whitespaceRe.ReplaceAllString(cmd, " "))
		if cmd == "" {
			return ""
		}
		if len(cmd) > 30 {
			return strings.TrimRight(cmd[:30], " ") + "..."
		}
		return cmd
	}
	return ""
}

func extractMcpServerName(toolName string) string {
	m := mcpNamePattern.FindStringSubmatch(toolName)
	if m == nil {
		return ""
	}
	return normalizeActivityName(m[1])
}

func resolveTaskIndex(taskID interface{}, taskIDToIndex map[string]int, todos []types.TodoItem) (int, bool) {
	key := mapTaskID(taskID)
	if key == "" {
		return 0, false
	}
	if idx, ok := taskIDToIndex[key]; ok {
		return idx, true
	}
	if numericRe.MatchString(key) {
		n, _ := strconv.Atoi(key)
		n--
		if n >= 0 && n < len(todos) {
			return n, true
		}
	}
	return 0, false
}

func normalizeTaskStatus(status string) string {
	switch status {
	case "pending", "not_started":
		return "pending"
	case "in_progress", "running":
		return "in_progress"
	case "completed", "complete", "done":
		return "completed"
	}
	return ""
}

func collectTools(m map[string]*types.ToolEntry, order []string) []types.ToolEntry {
	out := make([]types.ToolEntry, 0, len(order))
	for _, id := range order {
		if t := m[id]; t != nil {
			out = append(out, *t)
		}
	}
	if len(out) > maxTools {
		out = out[len(out)-maxTools:]
	}
	return out
}

func collectAgents(m map[string]*types.AgentEntry, order []string) []types.AgentEntry {
	out := make([]types.AgentEntry, 0, len(order))
	for _, id := range order {
		if a := m[id]; a != nil {
			out = append(out, *a)
		}
	}
	if len(out) > maxAgents {
		out = out[len(out)-maxAgents:]
	}
	return out
}

// --- helpers ---

var numericRe = regexp.MustCompile(`^\d+$`)

func normalizeActivityName(v string) string {
	s := strings.TrimSpace(ctrlRe.ReplaceAllString(v, ""))
	if s == "" {
		return ""
	}
	if len([]rune(s)) <= activityNameMaxLen {
		return s
	}
	r := []rune(s)
	return string(r[:activityNameMaxLen-1]) + "…"
}

func sanitizeModel(m string) string {
	m = strings.TrimSpace(ctrlRe.ReplaceAllString(m, ""))
	if len(m) > 80 {
		m = m[:80]
	}
	return m
}

func normalizeMessageID(id string) string {
	if id != "" && len(id) <= messageIDMaxLen {
		return id
	}
	return ""
}

func rememberID(seen map[string]struct{}, order *[]string, id string) {
	if len(seen) >= seenMessageIDsMax && len(*order) > 0 {
		oldest := (*order)[0]
		*order = (*order)[1:]
		delete(seen, oldest)
	}
	seen[id] = struct{}{}
	*order = append(*order, id)
}

func parseTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func capStr(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

func str(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func mapString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	return str(m[key])
}

func mapBool(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	b, _ := m[key].(bool)
	return b
}

func mapTaskID(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return ""
}

// asBlocks decodes message.content as an array of content blocks.
func asBlocks(raw json.RawMessage) ([]contentBlock, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	if raw[0] != '[' {
		return nil, false
	}
	var blocks []contentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, false
	}
	return blocks, true
}

// asString decodes message.content when it is a raw string.
func asString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || raw[0] != '"' {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

func rawToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == '"' {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s
		}
	}
	return ""
}

// --- ordered set ---

type orderedSet struct {
	seen  map[string]struct{}
	order []string
}

func newOrderedSet() *orderedSet {
	return &orderedSet{seen: map[string]struct{}{}}
}

func (s *orderedSet) add(v string) {
	if _, ok := s.seen[v]; ok {
		return
	}
	s.seen[v] = struct{}{}
	s.order = append(s.order, v)
}

func (s *orderedSet) values() []string {
	out := make([]string, len(s.order))
	copy(out, s.order)
	return out
}

// --- cache ---

type cacheFile struct {
	Version         int                  `json:"version"`
	TranscriptPath  string               `json:"transcriptPath"`
	TranscriptState cacheState           `json:"transcriptState"`
	Data            types.TranscriptData `json:"data"`
}

type cacheState struct {
	MtimeMs int64 `json:"mtimeMs"`
	Size    int64 `json:"size"`
}

func cachePath(transcriptPath string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(transcriptPath)))
	return filepath.Join(config.GetHudPluginDir(), "transcript-cache", hex.EncodeToString(sum[:])+".json")
}

func readCache(transcriptPath string, mtime, size int64) (types.TranscriptData, bool) {
	data, err := os.ReadFile(cachePath(transcriptPath))
	if err != nil {
		return types.TranscriptData{}, false
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return types.TranscriptData{}, false
	}
	if cf.Version != cacheVersion || cf.TranscriptPath != filepath.Clean(transcriptPath) ||
		cf.TranscriptState.MtimeMs != mtime || cf.TranscriptState.Size != size {
		return types.TranscriptData{}, false
	}
	return cf.Data, true
}

func writeCache(transcriptPath string, mtime, size int64, data types.TranscriptData) {
	p := cachePath(transcriptPath)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return
	}
	cf := cacheFile{
		Version: cacheVersion, TranscriptPath: filepath.Clean(transcriptPath),
		TranscriptState: cacheState{MtimeMs: mtime, Size: size}, Data: data,
	}
	b, err := json.Marshal(cf)
	if err != nil {
		return
	}
	_ = os.WriteFile(p, b, 0o600)
}
