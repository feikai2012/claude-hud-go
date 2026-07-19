---
description: Configure claude-hud-go display options
allowed-tools: Bash, Read, Edit, AskUserQuestion
---

# claude-hud-go configuration

All options live in a single JSON file:

- **bash**: `${CLAUDE_CONFIG_DIR:-$HOME/.claude}/plugins/claude-hud-go/config.json`
- **PowerShell**: `config.json` inside `$env:CLAUDE_CONFIG_DIR` (or `Join-Path $HOME ".claude"`) under `plugins\claude-hud-go\`

Create the directory if needed and write valid JSON. Only include keys you want
to change — everything else uses the defaults below. Changes take effect on the
next statusline refresh (no restart needed).

## Common toggles (`display.*`)

| Key | Default | Effect |
|-----|---------|--------|
| `showTools` | `false` | Tool activity line (`◐ Edit: file | ✓ Read ×3`) |
| `showAgents` | `false` | Subagent status line |
| `showTodos` | `false` | Todo progress line (`▸ task (2/5)`) |
| `showSkills` / `showMcp` | `false` | Skill / MCP activity lines |
| `showConfigCounts` | `false` | Environment line (CLAUDE.md / rules / MCP / hooks) |
| `showUsage` | `true` | Subscriber usage windows (5h / weekly) |
| `showContextBar` | `true` | Context progress bar |
| `showCost` | `false` | Session cost (from stdin) |
| `showSessionName` | `false` | Session slug / custom title |
| `showAdvisor` | `false` | Advisor model inline on the project line |
| `showDuration` | `false` | Session duration `⏱️` |
| `customLine` | `""` | Free text (`customLinePosition`: `first`/`last`) |

## Layout & thresholds

| Key | Default | Notes |
|-----|---------|-------|
| `lineLayout` | `expanded` | `expanded` (multi-line) or `compact` (single line) |
| `pathLevels` | `1` | Project path depth (1–3) |
| `elementOrder` | see below | Order of rendered elements |
| `display.mergeGroups` | `[["context","usage"]]` | Elements combined on one row |
| `display.contextWarningThreshold` | `70` | Yellow at/above this % |
| `display.contextCriticalThreshold` | `85` | Red + token breakdown at/above this % |
| `display.sevenDayThreshold` | `80` | Only show weekly window at/above this % |
| `language` | `en` | `en`, `zh` / `zh-Hans`, `zh-Hant` / `zh-TW` |

Default `elementOrder`:
`["project","addedDirs","context","usage","promptCache","memory","environment","tools","skills","mcp","agents","todos","sessionTime"]`

## Colors (`colors.*`)

Each accepts a named preset (`dim`,`red`,`green`,`yellow`,`magenta`,`cyan`,
`brightBlue`,`brightMagenta`), a 256-color index (0–255), or a `#rrggbb` hex
string. `barFilled`/`barEmpty` are single characters (default `█`/`░`).

## Example

```json
{
  "language": "en",
  "display": {
    "showTools": true,
    "showTodos": true,
    "showConfigCounts": true,
    "contextCriticalThreshold": 90
  },
  "colors": { "context": "#4ade80", "usage": 33 }
}
```

To disable the HUD for a single session without touching config, launch with
`CLAUDE_HUD_DISABLE=1 claude`.
