---
description: Configure claude-hud-go as your statusline
allowed-tools: Bash, Read, Edit, AskUserQuestion
---

# claude-hud-go setup

This configures the Go HUD binary as your Claude Code `statusLine`. Unlike the
Node version, there is **no runtime to install and no version-lookup launcher** —
the plugin ships prebuilt native binaries and setup simply points `statusLine`
at the one matching your OS/arch.

**Placeholders** like `{BIN}`, `{GENERATED_COMMAND}` are substituted with detected values.

## Step 1: Detect platform, shell, and arch

Use the environment context values (`Platform:` and `Shell:`). On `win32`, also
run `echo $OSTYPE` via the Bash tool: some Windows sessions report
`Shell: powershell` while the command environment is Git Bash/MSYS2, where bash
expands `$env:VAR`/`$(...)` before PowerShell runs.

| Platform | Shell | OSTYPE | Command format |
|----------|-------|--------|----------------|
| `darwin` | any | any | bash (macOS) |
| `linux`  | any | any | bash (Linux) |
| `win32`  | `bash` | any | bash — Windows + Git Bash |
| `win32`  | `powershell`/`pwsh`/`cmd` | `msys`/`cygwin` | bash — Windows + Git Bash |
| `win32`  | `powershell`/`pwsh`/`cmd` | other/empty | PowerShell — Windows |

Map the arch:
- **bash**: `uname -m` → `x86_64`→`amd64`, `arm64`/`aarch64`→`arm64`.
- **PowerShell**: `$env:PROCESSOR_ARCHITECTURE` → `AMD64`→`amd64`, `ARM64`→`arm64`.

The binary name is `claude-hud-<os>-<arch>` (`.exe` on Windows), e.g.
`claude-hud-darwin-arm64`, `claude-hud-windows-amd64.exe`.

## Step 2: Resolve the installed binary

Find the installed plugin directory (marketplace cache) and select the binary:

**macOS/Linux/Git Bash:**
```bash
CLAUDE_DIR="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
PLUGIN_DIR=$(ls -1d "$CLAUDE_DIR"/plugins/cache/*/claude-hud-go 2>/dev/null | head -1)
BIN="$PLUGIN_DIR/bin/claude-hud-<os>-<arch>"     # add .exe on Windows
chmod +x "$BIN" 2>/dev/null || true
ls -la "$BIN"
```

**Windows PowerShell:**
```powershell
$claudeDir = if ($env:CLAUDE_CONFIG_DIR) { $env:CLAUDE_CONFIG_DIR } else { Join-Path $HOME ".claude" }
$pluginDir = (Get-ChildItem (Join-Path $claudeDir "plugins\cache\*\claude-hud-go") -Directory -ErrorAction SilentlyContinue | Select-Object -First 1).FullName
$bin = Join-Path $pluginDir "bin\claude-hud-windows-<arch>.exe"
Test-Path $bin
```

If the binary is missing, the plugin is not installed (or your arch has no
prebuilt binary — build one with `make build`, see INSTALL.md). Ask the user to
`/plugin install claude-hud-go` first.

## Step 3: Build the statusLine command

Claude Code pipes the statusline's stdout, so the process cannot query terminal
width. The command exports `COLUMNS` (real width → `stty size` → 120, minus 4
for the input padding) before exec'ing the binary. The binary also honors
`COLUMNS`/`stty` as a fallback.

**macOS/Linux/Git Bash** — `{GENERATED_COMMAND}`:
```
sh -c 'cols=${COLUMNS:-}; case "$cols" in ""|*[!0-9]*) cols=$(stty size </dev/tty 2>/dev/null | awk "{print \$2}");; esac; case "$cols" in ""|*[!0-9]*) cols=120;; esac; export COLUMNS=$(( cols > 4 ? cols - 4 : 1 )); exec "{BIN}"'
```

**Windows** — `{GENERATED_COMMAND}` is simply the **absolute path to the `.exe`**
(no `cmd.exe` wrapper). It is a native executable and Claude Code pipes the JSON
straight to its stdin, so no shell layer is needed:
```
{BIN}
```
`{BIN}` must be a fully-resolved absolute path (expand `$pluginDir`, do not leave
`%USERPROFILE%` in the stored command). The binary reads `COLUMNS` from the
environment when Claude Code provides it; otherwise it uses 10-wide bars.

## Step 4: Back up and detect existing statusLine

Always create a timestamped backup of `settings.json` before writing.

**macOS/Linux:**
```bash
SETTINGS="${CLAUDE_CONFIG_DIR:-$HOME/.claude}/settings.json"
[ -f "$SETTINGS" ] && cp "$SETTINGS" "$SETTINGS.bak.$(date +%Y%m%d-%H%M%S)"
```
**Windows PowerShell:**
```powershell
$settings = if ($env:CLAUDE_CONFIG_DIR) { Join-Path $env:CLAUDE_CONFIG_DIR "settings.json" } else { Join-Path $HOME ".claude\settings.json" }
if (Test-Path $settings) { Copy-Item $settings "$settings.bak.$(Get-Date -Format 'yyyyMMdd-HHmmss')" }
```

If a `statusLine.command` already exists and does **not** contain
`claude-hud-go`, use AskUserQuestion to confirm before replacing it (options:
Replace / Keep and exit / Cancel). If it already contains `claude-hud-go`, this
is an idempotent reinstall — proceed.

## Step 5: Write the config

Merge (do not overwrite) into `settings.json` using a real JSON serializer:
```json
{ "statusLine": { "type": "command", "command": "{GENERATED_COMMAND}" } }
```

**Windows PowerShell 5.1 BOM**: write with
`[System.IO.File]::WriteAllText($path, $json, (New-Object System.Text.UTF8Encoding $false))`
to avoid a UTF-8 BOM (RFC 8259 forbids it in JSON).

Then tell the user:
> ✅ Config written. **Restart Claude Code** — quit and run `claude` again. The HUD appears below your input.

## Step 6: Optional features

Ask (multiSelect) whether to enable extras. Write them to
`${CLAUDE_CONFIG_DIR:-$HOME/.claude}/plugins/claude-hud-go/config.json` (merge if
present, only write selected keys):

| Selection | Config keys |
|-----------|-------------|
| Tools activity | `display.showTools: true` |
| Agents & Todos | `display.showAgents: true, display.showTodos: true` |
| Session info | `display.showDuration: true, display.showConfigCounts: true` |
| Session name | `display.showSessionName: true` |
| Custom line | `display.customLine: "<user text>"` |

See `configure.md` for the full option reference.

## Step 7: Verify

Run `{GENERATED_COMMAND}` with sample stdin to confirm output:
```bash
echo '{"model":{"display_name":"Opus"},"context_window":{"current_usage":{"input_tokens":45000},"context_window_size":200000}}' | {GENERATED_COMMAND}
```
It should print the HUD lines within a moment. If it errors, re-check Steps 1–3
(wrong arch binary, missing execute bit, or a Windows shell mismatch).
