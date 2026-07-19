// Package gitstat ports src/git.ts: branch, dirty state, ahead/behind, and
// file/line stats via the git CLI.
package gitstat

import (
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/jarrodwatts/claude-hud-go/internal/types"
)

func run(cwd string, timeout time.Duration, args ...string) (string, bool) {
	if cwd == "" {
		return "", false
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

func resolveRef(cwd string) string {
	if branch, ok := run(cwd, time.Second, "rev-parse", "--abbrev-ref", "HEAD"); ok {
		if branch != "" && branch != "HEAD" {
			return branch
		}
	}
	if tag, ok := run(cwd, time.Second, "describe", "--tags", "--exact-match", "HEAD"); ok && tag != "" {
		return tag
	}
	if sha, ok := run(cwd, time.Second, "rev-parse", "--short", "HEAD"); ok && sha != "" {
		return "detached:" + sha
	}
	return ""
}

// Status returns the git status for cwd, or nil when not a repository.
func Status(cwd string) *types.GitStatus {
	branch := resolveRef(cwd)
	if branch == "" {
		return nil
	}
	st := &types.GitStatus{Branch: branch}

	if out, ok := run(cwd, time.Second, "-c", "core.quotePath=false", "--no-optional-locks", "status", "--porcelain"); ok {
		st.Dirty = out != ""
		if st.Dirty {
			st.FilesChanged = len(nonEmptyLines(out))
		}
	}
	if st.Dirty {
		if out, ok := run(cwd, 2*time.Second, "-c", "core.quotePath=false", "diff", "--numstat", "HEAD"); ok {
			for _, line := range nonEmptyLines(out) {
				parts := strings.Split(line, "\t")
				if len(parts) < 3 {
					continue
				}
				added, err1 := strconv.Atoi(parts[0])
				deleted, err2 := strconv.Atoi(parts[1])
				if err1 != nil || err2 != nil {
					continue // binary file (shown as "-")
				}
				st.Insertions += added
				st.Deletions += deleted
			}
		}
	}
	if out, ok := run(cwd, time.Second, "rev-list", "--left-right", "--count", "@{upstream}...HEAD"); ok {
		parts := strings.Fields(out)
		if len(parts) == 2 {
			st.Behind, _ = strconv.Atoi(parts[0])
			st.Ahead, _ = strconv.Atoi(parts[1])
		}
	}
	return st
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}
