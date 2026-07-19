# Installing claude-hud-go

**claude-hud-go** is a real-time statusline HUD for [Claude Code](https://claude.ai/code),
reimplemented in Go as a single self-contained native binary. It shows context
health, subscriber usage, tool activity, running agents, and todo progress
below your input.

It is a Go port of [jarrodwatts/claude-hud](https://github.com/jarrodwatts/claude-hud)
(MIT). Same output, no Node.js runtime — just one small executable.

```
[Opus] │ my-project git:(main*)
Context █████░░░░░ 45% │ Usage ██░░░░░░░░ 25% (resets in 1h 30m)
◐ Edit: auth.ts | ✓ Read ×3 | ✓ Grep ×2
▸ Fix authentication bug (2/5)
```

---

## Prerequisites

- **Claude Code v1.0.80 or later** (`claude --version`).
- **No Node.js required.** The plugin ships prebuilt binaries.
- Go toolchain **only** if you build from source or your OS/arch has no prebuilt binary.

Supported prebuilt targets (in `bin/`):

| OS | Architectures |
|----|---------------|
| macOS (`darwin`) | `amd64`, `arm64` |
| Linux | `amd64`, `arm64` |
| Windows | `amd64`, `arm64` |

---

## Option A — Install via marketplace (recommended)

Inside Claude Code:

```
/plugin marketplace add jarrodwatts/claude-hud-go
/plugin install claude-hud-go
/claude-hud-go:setup
```

`/claude-hud-go:setup` detects your OS/arch, selects the matching binary from the
plugin's `bin/` directory, backs up your `settings.json`, and writes the
`statusLine` command. **Restart Claude Code** afterward — the HUD appears below
your input.

To install from a local checkout instead of GitHub:

```
/plugin marketplace add /path/to/claude-hud-go
/plugin install claude-hud-go
/claude-hud-go:setup
```

---

## Option B — Manual install

1. **Get the binary.** Download the file for your platform from `bin/` (or a
   GitHub release), e.g. `claude-hud-darwin-arm64`. Put it somewhere stable and,
   on macOS/Linux, make it executable:

   ```bash
   mkdir -p ~/.claude/plugins/claude-hud-go/bin
   cp claude-hud-darwin-arm64 ~/.claude/plugins/claude-hud-go/bin/claude-hud
   chmod +x ~/.claude/plugins/claude-hud-go/bin/claude-hud
   ```

2. **Add the `statusLine` command** to `~/.claude/settings.json`. Claude Code
   pipes stdout, so the wrapper exports `COLUMNS` (real terminal width) before
   running the binary.

   **macOS / Linux / Git Bash:**
   ```json
   {
     "statusLine": {
       "type": "command",
       "command": "sh -c 'cols=${COLUMNS:-}; case \"$cols\" in \"\"|*[!0-9]*) cols=$(stty size </dev/tty 2>/dev/null | awk \"{print \\$2}\");; esac; case \"$cols\" in \"\"|*[!0-9]*) cols=120;; esac; export COLUMNS=$(( cols > 4 ? cols - 4 : 1 )); exec \"$HOME/.claude/plugins/claude-hud-go/bin/claude-hud\"'"
     }
   }
   ```

   **Windows** — point directly at the absolute binary path (simplest and most
   robust; no `cmd.exe` wrapper or `%VAR%` expansion needed since it's a native
   `.exe` and Claude Code pipes JSON straight to it):
   ```json
   {
     "statusLine": {
       "type": "command",
       "command": "C:\\Users\\<you>\\.claude\\plugins\\claude-hud-go\\bin\\claude-hud-windows-amd64.exe"
     }
   }
   ```
   Use a real absolute path (backslashes escaped as `\\`). Terminal width comes
   from Claude Code's `COLUMNS`; if unset, the HUD uses 10-wide bars and does not
   wrap — still fully usable.

3. **Restart Claude Code.**

If you keep binaries elsewhere or use `CLAUDE_CONFIG_DIR`, adjust the paths
accordingly. The binary resolves config/cache under `$CLAUDE_CONFIG_DIR`
(default `~/.claude`) → `plugins/claude-hud-go/`.

---

## Configuration

All options live in a single JSON file (created on demand):

- bash: `${CLAUDE_CONFIG_DIR:-$HOME/.claude}/plugins/claude-hud-go/config.json`
- PowerShell: `plugins\claude-hud-go\config.json` under `$env:CLAUDE_CONFIG_DIR` or `~\.claude`

Only include the keys you want to change; the rest use defaults.

```json
{
  "language": "en",
  "lineLayout": "expanded",
  "pathLevels": 1,
  "display": {
    "showTools": true,
    "showAgents": true,
    "showTodos": true,
    "showConfigCounts": true,
    "showUsage": true,
    "contextWarningThreshold": 70,
    "contextCriticalThreshold": 85
  },
  "colors": { "context": "green", "usage": "brightBlue", "barFilled": "█", "barEmpty": "░" }
}
```

### Selected options

| Key | Default | Effect |
|-----|---------|--------|
| `language` | `en` | `en`, `zh`/`zh-Hans`, `zh-Hant`/`zh-TW` |
| `lineLayout` | `expanded` | `expanded` (multi-line) or `compact` |
| `pathLevels` | `1` | Project path depth shown (1–3) |
| `display.showTools` | `false` | Tool activity line |
| `display.showAgents` | `false` | Subagent status line |
| `display.showTodos` | `false` | Todo progress line |
| `display.showConfigCounts` | `false` | CLAUDE.md / rules / MCP / hooks counts |
| `display.showUsage` | `true` | Subscriber 5h / weekly usage windows |
| `display.showCost` | `false` | Session cost from stdin |
| `display.contextWarningThreshold` | `70` | Yellow bar at/above this % |
| `display.contextCriticalThreshold` | `85` | Red bar + token breakdown |
| `colors.*` | see defaults | Named preset, 256-index, or `#rrggbb` |

The full reference is in `commands/configure.md` (or run `/claude-hud-go:configure`).

Disable the HUD for one session without editing config:
`CLAUDE_HUD_DISABLE=1 claude`.

---

## Build from source

Requires Go 1.22+.

```bash
git clone https://github.com/jarrodwatts/claude-hud-go
cd claude-hud-go
make build          # host-native binary → bin/claude-hud
make build-all      # all six release targets → bin/
```

Point your `statusLine` command at the produced binary (see Option B).

Quick manual test (matches the reference HUD output):

```bash
echo '{"model":{"display_name":"Opus"},"context_window":{"current_usage":{"input_tokens":45000},"context_window_size":200000}}' | COLUMNS=120 ./bin/claude-hud
```

---

## Troubleshooting

**Nothing appears after setup.** Restart Claude Code fully (quit and relaunch) —
the `statusLine` config only loads on startup.

**`permission denied` / no output (macOS/Linux).** The binary needs the execute
bit: `chmod +x <binary>`. Verify the path in `settings.json` points at a real file.

**Wrong architecture / `exec format error`.** You selected the wrong binary.
Check with `uname -m` (bash) or `$env:PROCESSOR_ARCHITECTURE` (PowerShell) and
use the matching `claude-hud-<os>-<arch>` file. No prebuilt binary for your
arch? Build one with `make build`.

**Windows: HUD stays blank.** Confirm the command runs the `.exe` through
`cmd.exe` (Option B). If your session is Git Bash (`echo $OSTYPE` → `msys`),
use the bash `statusLine` variant instead of the PowerShell one.

**Bars misalign in a CJK terminal.** Set `"language": "zh-Hans"` (or your
locale) so ambiguous-width glyphs are measured as 2 cells.

**Verify the raw command.** Run the exact `statusLine.command` from your
`settings.json` with sample stdin (see the build test above) and read the error.

---

## Uninstall

Restore the timestamped backup setup created:

```bash
ls -t ~/.claude/settings.json.bak.* | head -1     # find latest
cp ~/.claude/settings.json.bak.<timestamp> ~/.claude/settings.json
```

Then `/plugin uninstall claude-hud-go` and restart Claude Code.
