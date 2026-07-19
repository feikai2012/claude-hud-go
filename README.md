# claude-hud-go

A real-time statusline HUD for [Claude Code](https://claude.ai/code), as a single
native Go binary. Go port of [jarrodwatts/claude-hud](https://github.com/jarrodwatts/claude-hud) (MIT).

```
[Opus] │ my-project git:(main*)
Context █████░░░░░ 45% │ Usage ██░░░░░░░░ 25% (resets in 1h 30m)
◐ Edit: auth.ts | ✓ Read ×3 | ✓ Grep ×2
▸ Fix authentication bug (2/5)
```

## Why a Go port?

A Claude Code statusline is just a program that reads a JSON blob on stdin and
writes ANSI to stdout every ~300ms. In Go that collapses to **one static binary**:

- No Node.js/Bun runtime to install.
- No `dist/` build step or version-lookup launcher script.
- Fast cold start (no interpreter warmup).

## Install

See **[INSTALL.md](./INSTALL.md)**. TL;DR inside Claude Code:

```
/plugin marketplace add feikai2012/claude-hud-go
/plugin install claude-hud-go
/claude-hud-go:setup
```

## How it works

```
Claude Code ─stdin JSON─▶ parse ─▶ render lines ─stdout─▶ Claude Code
            ╲ transcript_path ─▶ parse JSONL ─▶ tools/agents/todos/tokens
            ╲ config files ─▶ CLAUDE.md / rules / MCP / hooks counts
```

## Architecture

Mirrors the TypeScript source module-for-module for verifiability:

| Go package | Responsibility |
|------------|----------------|
| `cmd/claude-hud` | Entry point / orchestration |
| `internal/stdinp` | Parse stdin; context %, usage, model name |
| `internal/transcript` | JSONL parse + mtime cache |
| `internal/config` | Schema, defaults, merge, validate |
| `internal/configread` | Count CLAUDE.md/rules/MCP/hooks |
| `internal/gitstat` | Branch, dirty, ahead/behind, stats |
| `internal/i18n` | en / zh-Hans / zh-Hant catalogs |
| `internal/textwidth` | ANSI + grapheme + CJK width, wrap/truncate |
| `internal/render` | Lines, colors, layout, merge groups |

## Build

```bash
make build        # host binary → bin/claude-hud
make build-all    # 6 release targets → bin/
make test         # go test ./...
```

## Status / parity

Core rendering is byte-for-byte with the reference for the default layout and
the common opt-in lines (tools, agents, todos, skills, MCP, environment, usage,
context, session time/tokens, compactions, memory). The long-tail options in
`config.ts` are all accepted and validated; a few advanced renderers (prompt
cache countdown, added-dirs line, auth segment, output-speed, external usage
snapshots) are stubbed and render nothing yet — contributions welcome. See the
inline `// not yet ported` notes.

## License

MIT (same as the upstream project).
