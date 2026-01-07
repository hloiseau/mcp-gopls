# mcp-gopls – MCP server for Go (gopls)
<!-- markdownlint-disable MD022 MD012 MD029 MD060 -->
[![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
![Go version](https://img.shields.io/badge/Go-1.25+-informational)
[![CI](https://github.com/hloiseau/mcp-gopls/actions/workflows/ci.yml/badge.svg)](https://github.com/hloiseau/mcp-gopls/actions)

A Model Context Protocol (MCP) server that lets AI assistants use Go’s LSP (`gopls`) for navigation, diagnostics, testing, coverage, and more.

> **TL;DR:** If you use Claude / Cursor / Copilot with Go, `mcp-gopls` gives the AI full LSP powers:
> go-to-definition, references, hover, completion, `go test`, coverage, `go mod tidy`, `govulncheck`, etc.

![Demo Animation](docs/Animation.gif)


## Overview

This MCP server helps AI assistants to:

- Use LSP to analyze Go workspaces
- Navigate to definitions, references, and workspace symbols
- Format, rename, and inspect code actions without leaving MCP
- Run Go tests, coverage, `go mod tidy`, `govulncheck`, and module graph commands with structured results
- Read workspace resources (overview + go.mod) and consume curated prompts

> **Status:** Actively developed – used in real projects.  
> Tested with Go 1.25.x and `gopls@latest`.  

## Architecture

This project uses the [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) library to implement the Model Context Protocol. The MCP integration enables seamless communication between AI assistants and Go tools.

The server communicates with [gopls](https://github.com/golang/tools/tree/master/gopls), the official language server for Go, via the Language Server Protocol (LSP).

## Features

- **Configurable runtime**: `--workspace`, `--gopls-path`, `--log-level`, `--rpc-timeout`, and `--shutdown-timeout` flags + env vars (`MCP_GOPLS_*`)
- **Structured logging**: Text/JSON logging with slog and optional file output
- **Extended LSP surface**: navigation, diagnostics, formatting, rename, code actions, hover, completion, workspace symbols
- **Test & tooling helpers**: coverage analysis, `go test`, `go mod tidy`, `govulncheck`, `go mod graph`
- **MCP extras**: resources (`resource://workspace/overview`, `resource://workspace/go.mod`) and prompts (`summarize_diagnostics`, `refactor_plan`)
- **Progress streaming**: long-running commands emit `notifications/progress` events so clients can surface status updates

### Feature comparison: `mcp-gopls` vs built-in `gopls` MCP

As of `gopls` v0.20.0, the built-in MCP server exposes these tools:
`go_context`, `go_diagnostics`, `go_file_context`, `go_file_diagnostics`,
`go_file_metadata`, `go_package_api`, `go_references`, `go_rename_symbol`,
`go_search`, `go_symbol_references`, `go_workspace`, `go_vulncheck`.

| Feature / capability | `mcp-gopls` (this project) | Built-in `gopls` MCP |
|----------------------|----------------------------|----------------------|
| Go-to-definition | Yes (`go_to_definition` tool) | No dedicated MCP tool (not in tool list) |
| Find references | Yes (`find_references`) | Yes (`go_references`, `go_symbol_references`) |
| Diagnostics (file / workspace) | Yes (`check_diagnostics`) | Yes (`go_diagnostics`, `go_file_diagnostics`) |
| Hover information | Yes (`get_hover_info`) | No dedicated MCP tool (not in tool list) |
| Completion | Yes (`get_completion`) | No dedicated MCP tool (not in tool list) |
| Formatting | Yes (`format_document`) | No dedicated MCP tool (not in tool list) |
| Rename symbol | Yes (`rename_symbol`) | Yes (`go_rename_symbol`) |
| Code actions | Yes (`list_code_actions`) | No dedicated MCP tool (not in tool list) |
| Workspace symbol search | Yes (`search_workspace_symbols`) | Yes (`go_search`) |
| Package / workspace API/context tools | No dedicated MCP tool | Yes (`go_package_api`, `go_file_context`, `go_file_metadata`, `go_workspace`, `go_context`) |
| Run `go test` | Yes (`run_go_test`) | No MCP tool for running tests |
| Coverage analysis | Yes (`analyze_coverage`) | No MCP tool for coverage |
| `go mod tidy` | Yes (`run_go_mod_tidy`) | No MCP tool for `go mod tidy` |
| `govulncheck` | Yes (`run_govulncheck`) | Yes (`go_vulncheck`) |
| Module graph (`go mod graph`) | Yes (`module_graph`) | No MCP tool for module graph |
| Extra MCP resources | Yes (`resource://workspace/overview`, `resource://workspace/go.mod`) | Not documented as MCP resources |
| Custom MCP prompts | Yes (`summarize_diagnostics`, `refactor_plan`) | Not exposed as MCP prompts (only model instructions) |
| Model instructions shipped with server | No special mechanism (documented in README/docs) | Yes: `gopls mcp -instructions` prints usage workflows |

If you want full LSP-like editing + tooling from MCP (definition, hover, completion, format, rename, code actions, go test, coverage, go mod tidy, module graph), mcp-gopls is strictly richer.

If you mostly want read-only/introspective tools (diagnostics, symbol search, references, package API, workspace/file context, vulncheck) with no extra binary, the built-in gopls MCP is enough.

> **Note:** The built-in `gopls` MCP server is still marked experimental and its tool set may change over time. This comparison is accurate as of `gopls` v0.20.x.


## Project Structure

```bash
.
├── cmd
│   └── mcp-gopls        # Application entry point
├── pkg
│   ├── lsp             # LSP client to communicate with gopls
│   │   ├── client      # LSP client implementation
│   │   └── protocol    # LSP protocol types and features
│   ├── server          # MCP server
│   └── tools           # MCP tools exposing LSP features
```

## Installation

```bash
go install github.com/hloiseau/mcp-gopls/v2/cmd/mcp-gopls@latest
```

## Quick Start

1. **Install** the server:

```bash
go install github.com/hloiseau/mcp-gopls/v2/cmd/mcp-gopls@latest
```

2. **Verify** it's on your `$PATH`:

```bash
mcp-gopls --help
```

3. **Configure** your AI client (see examples below for Cursor, Claude Desktop, or GitHub Copilot).

---

## Detailed Client Setup

> **Note:** All clients point to the same command:  
> `mcp-gopls --workspace /absolute/path/to/your/go/project`  
> The configuration format differs slightly per client, but the binary and arguments remain identical.

### 1. Connect from Cursor

1. Open **Settings → MCP Servers → Edit JSON**.
2. Add or update the `mcp-gopls` entry:

```json
{
  "mcpServers": {
    "mcp-gopls": {
      "command": "mcp-gopls",
      "args": ["--workspace", "/absolute/path/to/your/go/project"],
      "env": {
        "MCP_GOPLS_LOG_LEVEL": "info"
      }
    }
  }
}
```

3. Run **Developer: Reload Window** so Cursor reconnects.
4. Open the **Tools** drawer in Cursor Chat and enable `mcp-gopls`.

### 2. Invoke the tools

| Tool / Prompt | Example request inside Cursor Chat |
|---------------|------------------------------------|
| `go_to_definition` | “Use `go_to_definition` on `pkg/server/server.go:42`.” |
| `find_references` | “Ask the tool for references to `ServeStdio`.” |
| `check_diagnostics` | “Request diagnostics for `cmd/mcp-gopls/main.go`.” |
| `get_hover_info` | “Call `get_hover_info` on `pkg/tools/workspace.go:88`.” |
| `get_completion` | “Trigger completions at `pkg/server/server.go:55`.” |
| `format_document` | “Run the formatter over `pkg/tools/refactor.go`.” |
| `rename_symbol` | “Rename `clientFactory` to `newClientFactory` via the tool.” |
| `list_code_actions` | “List code actions for `pkg/server/server.go:80-90`.” |
| `search_workspace_symbols` | “Search workspace symbols for `NewWorkspaceConfig`.” |
| `analyze_coverage` | “Run `analyze_coverage` for `./pkg/...` with per-function stats.” |
| `run_go_test` | “Execute `run_go_test` on `./cmd/...`.” |
| `run_go_mod_tidy` | “Invoke `run_go_mod_tidy` to sync go.mod.” |
| `run_govulncheck` | “Run `run_govulncheck` and stream findings.” |
| `module_graph` | “Call `module_graph` to inspect dependencies.” |
| `summarize_diagnostics` | “Use the `summarize_diagnostics` prompt on the latest diagnostics.” |
| `refactor_plan` | “Feed `refactor_plan` the diagnostics JSON to plan fixes.” |

## Client Setup Examples

### Claude Desktop (macOS, Windows, Linux)

1. Install `mcp-gopls` and make sure it is on your `$PATH`.
2. Create or edit `claude_desktop_config.json`.
   - macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
   - Linux: `~/.config/Claude/claude_desktop_config.json`
   - Windows: `%APPDATA%\Claude\claude_desktop_config.json`
3. Add the server entry:

```json
{
  "mcpServers": {
    "mcp-gopls": {
      "command": "mcp-gopls",
      "args": ["--workspace", "/absolute/path/to/your/go/project"],
      "env": {
        "MCP_GOPLS_LOG_LEVEL": "info"
      }
    }
  }
}
```

Restart Claude Desktop, open a chat, and ask it to connect to the `mcp-gopls` tool (Claude will show a “Tools” tab once the server is detected). Typical prompts include “list diagnostics for `cmd/api/server.go`” or “rename `userService` to `accountService`.”

### Cursor IDE

In Cursor open **Settings → MCP Servers → Edit JSON** (this writes to `~/.cursor/config.json` or the project-local override). Add:

```json
{
  "mcpServers": {
    "mcp-gopls": {
      "command": "mcp-gopls",
      "args": ["--workspace", "/absolute/path/to/your/go/project"]
    }
  }
}
```

Reload Cursor (or run the `Developer: Reload Window` command) and the server will appear inside the “Tools” drawer. You can now ask Cursor Chat things like “run `go test ./pkg/server` with coverage” or “show hover info for `pkg/tools/tests.go:42`.”

### GitHub Copilot (Agent Mode)

GitHub Copilot’s Agent Mode can talk to local MCP servers across VS Code, JetBrains IDEs, Eclipse, and Xcode ([docs](https://docs.github.com/en/copilot/customizing-copilot/extending-copilot-chat-with-mcp)). To wire `mcp-gopls` in VS Code:

1. Update GitHub Copilot (requires VS Code 1.99+), opt into **Agent Mode**.
2. Create `.vscode/mcp.json` in your workspace (or edit the global file shown in the Copilot “Edit config” dialog).
3. Add:

```json
{
  "servers": {
    "mcp-gopls": {
      "type": "stdio",
      "command": "mcp-gopls",
      "args": ["--workspace", "/absolute/path/to/your/go/project"],
      "env": {
        "MCP_GOPLS_LOG_LEVEL": "warn"
      }
    }
  }
}
```

4. Reload Agent Mode (toggle off/on) so Copilot discovers the new tool; the chat “Tools” picker will now expose every MCP action (`run_go_test`, `run_govulncheck`, etc.). JetBrains and other IDEs share the same JSON schema via their Copilot settings panel.

### MCP Inspector / CLI testing

For quick smoke tests or demos you can use [mark3labs/mcp-inspector](https://github.com/mark3labs/mcp-inspector):

```bash
npx -y @mark3labs/mcp-inspector \
  --command mcp-gopls \
  --args "--workspace" "/absolute/path/to/your/go/project"
```

The inspector lets you call each tool/resource/prompt manually, which is handy for debugging server configuration before wiring it into an AI assistant.

## MCP Tools

| Tool | Description |
|------|-------------|
| `go_to_definition` | Navigate to the definition of a symbol |
| `find_references` | List all references for a symbol |
| `check_diagnostics` | Fetch cached diagnostics for a file |
| `get_hover_info` | Return hover markdown for a symbol |
| `get_completion` | Return completion labels at a position |
| `format_document` | Return formatting edits for an entire document |
| `rename_symbol` | Return workspace edits for a rename |
| `list_code_actions` | List available code actions for a range |
| `search_workspace_symbols` | Search workspace-wide symbols |
| `analyze_coverage` | Run `go test` with coverage + optional per-function report |
| `run_go_test` | Execute `go test` for a package/pattern |
| `run_go_mod_tidy` | Execute `go mod tidy` |
| `run_govulncheck` | Execute `govulncheck ./...` |
| `module_graph` | Return `go mod graph` output |

## Progress Notifications

Long-running tools emit structured `notifications/progress` events so IDEs can show rich status indicators:

- **Streaming progress** (`run_go_test`, `analyze_coverage`, `run_govulncheck`, `run_go_mod_tidy`) forwards incremental log lines and percentage updates. Cursor displays these as a live log.
- **Start/complete events only** (`go_to_definition`, `find_references`, `rename_symbol`, etc.) fire a quick “started” event so the UI can show a spinner, followed by a completion payload with the final result.
- Each progress token is now namespaced (e.g., `run_go_test/<rand>`) to avoid “unknown token” errors when multiple tools run concurrently.

When integrating new tools, opt into streaming mode only if the underlying LSP/golang command produces meaningful interim output; otherwise stick to the lightweight start/complete flow to minimize noise.

## Prompt Instructions

Both prompts are accessible from any MCP-aware client via the “Prompts” catalog.

### `summarize_diagnostics`

- **When to use:** After `check_diagnostics` or `run_go_test` to turn raw diagnostics into actionable steps.
- **Arguments:** None. The server automatically reads the last diagnostics payload cached by the tools layer.
- **Typical workflow:** `check_diagnostics` → copy the returned array into the prompt input field (Cursor’s UI pastes it automatically when you select “Use last result”).

### `refactor_plan`

- **When to use:** You already have a diagnostics JSON array and want a concise change checklist.
- **Arguments:** Requires a `diagnostics` object containing the raw Go diagnostics (the same payload returned by `check_diagnostics`).
- **Example invocation payload:**

```json
{
  "diagnostics": [
    {
      "uri": "file:///path/to/pkg/tools/workspace.go",
      "range": {"start": {"line": 12, "character": 5}, "end": {"line": 12, "character": 25}},
      "severity": 1,
      "message": "unused variable testHelper"
    }
  ]
}
```

The prompt responds with a numbered set of refactor steps plus suggested validation commands (`go test`, `analyze_coverage`, etc.).

## Configuration

The server supports various configuration options via command-line flags and environment variables:

### Command-Line Flags

| Flag                  | Default | Description                                    |
|-----------------------|---------|------------------------------------------------|
| `--workspace`         | `.`     | Absolute path to your Go project root          |
| `--gopls-path`        | `gopls` | Path to the gopls binary                       |
| `--log-level`         | `info`  | Log level (`debug`, `info`, `warn`, `error`)   |
| `--rpc-timeout`       | `30s`   | RPC timeout for LSP calls                      |
| `--shutdown-timeout`  | `5s`    | Timeout for graceful shutdown                  |

### Environment Variables

All flags can be set via environment variables with the `MCP_GOPLS_` prefix:

| Environment Variable       | Equivalent Flag       | Description                                    |
|---------------------------|-----------------------|------------------------------------------------|
| `MCP_GOPLS_WORKSPACE`     | `--workspace`         | Absolute path to your Go project root          |
| `MCP_GOPLS_GOPLS_PATH`    | `--gopls-path`        | Path to the gopls binary                       |
| `MCP_GOPLS_LOG_LEVEL`     | `--log-level`         | Log level (`debug`, `info`, `warn`, `error`)   |
| `MCP_GOPLS_RPC_TIMEOUT`   | `--rpc-timeout`       | RPC timeout for LSP calls (e.g., `30s`, `1m`)  |
| `MCP_GOPLS_SHUTDOWN_TIMEOUT` | `--shutdown-timeout` | Timeout for graceful shutdown                |

Command-line flags take precedence over environment variables.

## Troubleshooting

- **“column is beyond end of line”** – gopls could not map the provided position. Confirm the file is saved and the position uses zero-based lines/columns; run `go fmt` to ensure tabs vs. spaces align with gopls expectations.
- **“no hover information available”** – the symbol might belong to a generated file or a module outside the configured workspace. Ensure the `--workspace` flag points to the module root and that `go list ./...` succeeds.
- **“workspace not initialized”** – the server did not finish its initial sync. Wait for the `workspace initialized` log line or restart `mcp-gopls` after deleting stale `.gopls` caches.
- **`run_govulncheck` missing binary** – the tool now falls back to `go run golang.org/x/vuln/cmd/govulncheck@latest`, but the machine still needs outbound network access. Install the binary manually if the fallback is blocked.

## Usage Example

Using the server with AI assistants that support MCP:

```Markdown
# Ask the AI to get information about the code
Can you find the definition of the `ServeStdio` function in this project?

# Ask for diagnostics
Are there any errors in my main.go file?

# Ask for information about a symbol
What does the Context.WithTimeout function do in Go?
```

## Development

```bash
git clone https://github.com/hloiseau/mcp-gopls.git
cd mcp-gopls
go mod tidy
go test ./...
go build ./cmd/mcp-gopls
```

Table-driven tests live under `pkg/tools` and CI runs via `.github/workflows/ci.yml`.

### Documentation

- `docs/usage.md` – quickstart and tool catalog walkthrough
- Workspace resources expose `resource://workspace/overview` and `resource://workspace/go.mod`
- Prompts (`summarize_diagnostics`, `refactor_plan`) help assistants produce consistent outputs

## Contributing

PRs and issues are welcome!

- Check [open issues](https://github.com/hloiseau/mcp-gopls/issues) or file a new one if you hit a bug or want a feature.
- Run `go test ./...` before opening a PR.
- For bigger changes (new tools, protocol changes), please open a design issue first so we can discuss the approach.

All contributions should maintain test coverage and adhere to Go best practices. See the [Development](#development) section for setup instructions.

## Prerequisites

- Go 1.25+ (tested with `go1.25.4`)
- `gopls` installed (`go install golang.org/x/tools/gopls@latest`)
- Optional: `govulncheck` (`go install golang.org/x/vuln/cmd/govulncheck@latest`)
- The server forces `GOTOOLCHAIN=local` for its nested `gopls` process. If you need a different toolchain, set `GOTOOLCHAIN` in the environment before launching `mcp-gopls`.

## Integration with Ollama

This MCP server can be used with any tool that supports the MCP protocol. For Ollama integration:

1. Make sure Ollama is running
2. The MCP server runs independently and communicates through stdin/stdout
3. Configure your client to use the MCP server as a tool provider

## License

Apache License 2.0
