# Contributing to mcp-gopls

Thank you for your interest in contributing! This guide covers local setup, testing, and pull request expectations.

## Development setup

### Prerequisites

- Go 1.26 or later
- `gopls` installed: `go install golang.org/x/tools/gopls@latest`
- Optional: `govulncheck` — `go install golang.org/x/vuln/cmd/govulncheck@latest`

### Clone and build

```bash
git clone https://github.com/hloiseau/mcp-gopls.git
cd mcp-gopls
make deps
make build
```

## Running checks locally

Run the same checks CI runs before opening a pull request:

```bash
make fmt-check   # verify gofmt and goimports formatting
make lint        # golangci-lint
make vet         # go vet
make test        # unit tests
make coverage    # tests with coverage report
make tidy-check  # verify go.mod / go.sum are tidy
```

Or run everything at once:

```bash
make verify
```

To auto-format code:

```bash
make fmt
```

## Pull request guidelines

### Branch naming

Use a short, descriptive prefix:

- `feat/` — new features or tools
- `fix/` — bug fixes
- `ci/` — CI, automation, or build changes
- `docs/` — documentation only
- `refactor/` — internal restructuring without behavior change

Example: `feat/add-hover-tool`, `fix/diagnostics-cache`

### Commit messages

Write clear, imperative commit messages:

- `feat: add workspace symbol filter`
- `fix: handle missing gopls binary`
- `ci: add coverage reporting`

Keep the subject line under 72 characters. Add a body when the change needs context.

### Test expectations

- All existing tests must pass (`make test`).
- New behavior should include table-driven tests where practical (see `pkg/tools` for examples).
- Run `make verify` before pushing — CI runs the same pipeline.

## Code style

- Write comments and documentation in English.
- Follow standard Go conventions (`gofmt`, `goimports`).
- Keep imports grouped and alphabetically sorted within each group (stdlib, third-party, local).
- Match the style of surrounding code — read nearby files before adding new code.
- Prefer small, focused changes over large refactors mixed with feature work.

## Getting help

- Check [open issues](https://github.com/hloiseau/mcp-gopls/issues) before filing a new one.
- For larger changes (new tools, protocol changes), open a design issue first to discuss the approach.
