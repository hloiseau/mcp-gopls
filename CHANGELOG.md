## v2.0.0 – 2025-11-22

### Added
- **Expanded MCP tool suite** – the server now registers navigation (`go_to_definition`, `find_references`), diagnostics (`check_diagnostics`), insights (`get_hover_info`, `get_completion`), refactor helpers (`format_document`, `rename_symbol`, `list_code_actions`), testing (`run_go_test`, `analyze_coverage`), workspace commands (`search_workspace_symbols`, `run_go_mod_tidy`, `run_govulncheck`, `module_graph`), and notification plumbing, all backed by new end-to-end tests under `pkg/tools`.
- **Prompt catalog** – `summarize_diagnostics` and `refactor_plan` prompts expose curated messaging for Cursor/Claude via the MCP prompt API.
- **CI + tooling enforcement** – `make verify` now runs gofmt/goimports checks, golangci-lint, go vet, go test, and `go mod tidy` validation, and `.github/workflows/ci.yml` runs the suite on every push/PR. `scripts/tidy-check.sh` ensures dependency files stay clean.
- **Documentation overhaul** – Quick Start instructions, tool invocation table, prompt usage guide, and troubleshooting tips were added to `README.md` and `docs/usage.md` for Cursor, Claude, Copilot, and MCP Inspector integrations.

### Changed
- **CLI entrypoint** – `cmd/mcp-gopls` validates workspace paths, log settings, RPC/shutdown timeouts, and custom `gopls` binaries before startup, handles SIGINT/SIGTERM, and reports shutdown status.
- **Go requirements** – bumped to Go 1.25 with `github.com/mark3labs/mcp-go v0.43.0` plus the new JSON/protocol dependencies used by the extended LSP surface.
- **Progress notifications** – long-running tools stream logs with namespaced tokens, eliminating “unknown token” errors in Cursor and surfacing clearer status updates.
- **Govulncheck automation** – `run_govulncheck` automatically falls back to `go run golang.org/x/vuln/cmd/govulncheck@latest` when the binary is missing, with explicit progress messaging.

### Fixed
- **Coverage fallback** – `analyze_coverage` always returns aggregate results even when per-function parsing fails, so release artifacts never miss coverage numbers.
- **Transport robustness** – the JSON-RPC transport reads exactly one message per call and closes cleanly on pipe errors; the gopls client now discovers `GOROOT` via `go env` to avoid stale runtime paths.

