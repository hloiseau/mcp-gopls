package tools

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var lookupGovulncheckBinary = exec.LookPath

func (t *LSPTools) registerWorkspaceTools(s *server.MCPServer) {
	t.registerWorkspaceSymbols(s)
	t.registerGoModTidy(s)
	t.registerGovulncheck(s)
	t.registerModuleGraph(s)
}

func (t *LSPTools) registerWorkspaceSymbols(s *server.MCPServer) {
	tool := mcp.NewTool("search_workspace_symbols",
		mcp.WithDescription("Search workspace symbols via LSP"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, err := getArguments(request)
		if err != nil {
			return nil, err
		}

		query, err := getStringArg(args, "query")
		if err != nil {
			return nil, err
		}

		lspClient := t.getClient()
		if lspClient == nil {
			return nil, fmt.Errorf("LSP client not initialized")
		}

		symbols, err := lspClient.WorkspaceSymbols(ctx, query)
		if err != nil {
			return nil, t.handleLSPError(err)
		}

		result, err := mcp.NewToolResultJSON(map[string]any{
			"query":   query,
			"symbols": symbols,
		})
		if err != nil {
			return nil, err
		}
		return result, nil
	})
}

func (t *LSPTools) registerGoModTidy(s *server.MCPServer) {
	tool := mcp.NewTool("run_go_mod_tidy",
		mcp.WithDescription("Execute go mod tidy in the workspace"),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		token := getProgressToken(request.Params.Meta)
		sendProgressNotification(ctx, s, token, "Running go mod tidy")
		result, err := t.runCommand(ctx, s, token, "go", "mod", "tidy")
		if err != nil {
			return t.commandFailureResult("go mod tidy", result, err)
		}

		payload := map[string]any{"result": result}
		toolResult, err := mcp.NewToolResultJSON(payload)
		if err != nil {
			return nil, err
		}
		return toolResult, nil
	})
}

func (t *LSPTools) registerGovulncheck(s *server.MCPServer) {
	tool := mcp.NewTool("run_govulncheck",
		mcp.WithDescription("Execute govulncheck ./... in the workspace"),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		token := getProgressToken(request.Params.Meta)
		cmd, args, fallback := determineGovulncheckCommand()
		if fallback {
			sendProgressNotification(ctx, s, token, "Running govulncheck via go run (binary not found in PATH)")
		} else {
			sendProgressNotification(ctx, s, token, "Running govulncheck ./...")
		}

		result, err := t.runCommand(ctx, s, token, cmd, args...)
		if err != nil {
			var execErr *exec.Error
			if errors.As(err, &execErr) {
				return mcp.NewToolResultError(fmt.Sprintf("%s binary not found", execErr.Name)), nil
			}
			if errors.Is(err, context.DeadlineExceeded) {
				return mcp.NewToolResultError("govulncheck timed out"), nil
			}
			return t.commandFailureResult(strings.Join(result.Command, " "), result, err)
		}

		payload := map[string]any{"result": result}
		toolResult, err := mcp.NewToolResultJSON(payload)
		if err != nil {
			return nil, err
		}
		return toolResult, nil
	})
}

func determineGovulncheckCommand() (string, []string, bool) {
	if path, err := lookupGovulncheckBinary("govulncheck"); err == nil {
		return path, []string{"./..."}, false
	}
	return "go", []string{"run", "golang.org/x/vuln/cmd/govulncheck@latest", "./..."}, true
}

func (t *LSPTools) registerModuleGraph(s *server.MCPServer) {
	tool := mcp.NewTool("module_graph",
		mcp.WithDescription("Return the Go module dependency graph"),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		token := getProgressToken(request.Params.Meta)
		sendProgressNotification(ctx, s, token, "Building module graph")
		// Disable per-line progress streaming here to avoid overwhelming clients
		// with thousands of dependency lines (they remain in the tool result).
		result, err := t.runCommand(ctx, s, nil, "go", "mod", "graph")
		if err != nil {
			return t.commandFailureResult("go mod graph", result, err)
		}

		payload := map[string]any{
			"result": result,
		}
		toolResult, err := mcp.NewToolResultJSON(payload)
		if err != nil {
			return nil, err
		}
		return toolResult, nil
	})
}
