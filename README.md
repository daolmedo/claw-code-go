# claw-code-go

<p align="center">
  <img src="assets/claw-code-go.png" alt="claw-code-go logo" width="320" />
</p>

A Go port of Claude Code — the Anthropic CLI coding assistant.

## Workspace Layout

```
claw-code-go/
├── go.mod
├── cmd/
│   └── claw-code-go/    # CLI entry point
│       └── main.go
└── internal/
    ├── api/             # Anthropic API client (SSE streaming, types)
    │   ├── client.go
    │   └── types.go
    ├── runtime/         # Conversation loop, session, config, permissions
    │   ├── config.go
    │   ├── conversation.go
    │   ├── permissions.go
    │   └── session.go
    ├── tools/           # Tool implementations
    │   ├── bash.go
    │   ├── files.go
    │   ├── glob.go
    │   └── grep.go
    ├── commands/        # Slash command registry
    │   └── registry.go
    └── compat/          # Upstream TS source reference data
        └── manifest.go
```

## Prerequisites

- Go 1.22+
- `ANTHROPIC_API_KEY` environment variable

## Building

```sh
go build ./...
go build -o claw-code-go ./cmd/claw-code-go
```

## Usage

### One-shot mode

```sh
export ANTHROPIC_API_KEY=sk-ant-...
./claw-code-go --prompt "List the files in the current directory"
```

### Interactive REPL

```sh
./claw-code-go --repl
```

Or just run without flags:

```sh
./claw-code-go
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--prompt` | — | Single prompt (one-shot mode) |
| `--model` | `claude-sonnet-4-20250514` | Model to use |
| `--repl` | false | Force interactive REPL mode |
| `--session` | — | Resume a saved session by ID |
| `--session-dir` | `~/.claw-code/sessions` | Directory for session files |

### Slash Commands (REPL)

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/clear` | Clear current session messages |
| `/session-list` | List saved sessions |
| `/exit` or `/quit` | Exit the REPL |

## Tools Available

| Tool | Description |
|------|-------------|
| `bash` | Execute a bash command (30s timeout) |
| `read_file` | Read a file from disk |
| `write_file` | Write content to a file |
| `glob` | Find files matching a glob pattern |
| `grep` | Search files with a regex pattern |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Required. Your Anthropic API key |
| `ANTHROPIC_MODEL` | Override the default model |
| `ANTHROPIC_BASE_URL` | Override the API base URL |

## Session Persistence

Sessions are saved as JSON files in `~/.claw-code/sessions/` (or `--session-dir`). Each session stores the full message history and can be resumed with `--session <id>`.

## Architecture

The package structure mirrors the Rust port's crate layout:

- **internal/api** — HTTP client with real SSE streaming, parses `data:` frames from the Anthropic messages API
- **internal/runtime** — Agentic conversation loop with full tool-use support (loops until `end_turn`), session JSON persistence, environment-based config
- **internal/tools** — Bash execution, file I/O, recursive glob, regex grep
- **internal/commands** — Slash command registry with duck-typed callback into the conversation loop
- **internal/compat** — Walks the upstream TypeScript source tree to produce a file manifest
- **cmd/claw-code-go** — CLI with `--prompt` one-shot and interactive REPL, SIGINT-safe session saving
