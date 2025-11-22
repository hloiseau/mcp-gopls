## MCP-gopls Usage Guide

### Configure the server

```bash
# Run inside your Go repo
mcp-gopls \
  --workspace /path/to/repo \
  --log-level debug \
  --rpc-timeout 60s
```

> **Note:** `mcp-gopls` ensures `GOTOOLCHAIN=local` for the embedded `gopls` process so that it can run even when the requested Go toolchain hasn’t been published yet. Export your own `GOTOOLCHAIN` before starting the server if you prefer a different setting.

Environment variables:

| Variable | Purpose |
|----------|---------|
| `MCP_GOPLS_WORKSPACE` | Default workspace root |
| `MCP_GOPLS_BIN` | Path to `gopls` binary |
| `MCP_GOPLS_LOG_FILE` | Optional log file |
| `MCP_GOPLS_LOG_LEVEL` | debug, info, warn, error |
| `MCP_GOPLS_RPC_TIMEOUT` | LSP call timeout |
| `MCP_GOPLS_SHUTDOWN_TIMEOUT` | Graceful shutdown timeout |

### Resource & prompt catalog

| Resource URI | Description |
|--------------|-------------|
| `resource://workspace/overview` | JSON summary of top-level directories & Go files |
| `resource://workspace/go.mod` | Raw contents of go.mod |

| Prompt | Description | Arguments |
|--------|-------------|-----------|
| `summarize_diagnostics` | Summarize diagnostics into actionable guidance | none |
| `refactor_plan` | Produce a quick refactor checklist based on diagnostics JSON | `diagnostics` |

### Recommended workflow

1. `check_diagnostics` > feed diagnostics into `summarize_diagnostics` prompt.
2. Read `resource://workspace/overview` to understand layout.
3. Run `run_go_test` or `analyze_coverage` to validate fixes.
4. Use `format_document` / `rename_symbol` / `list_code_actions` for refactors.
5. Finish with `run_go_mod_tidy`, `run_govulncheck`, and `module_graph` to keep dependencies healthy.

