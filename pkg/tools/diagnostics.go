package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func (t *LSPTools) registerDiagnosticsTools(s *server.MCPServer) {
	t.registerCheckDiagnostics(s)
}

func (t *LSPTools) registerCheckDiagnostics(s *server.MCPServer) {
	diagnosticsTool := mcp.NewTool("check_diagnostics",
		mcp.WithDescription("Get diagnostics for a file"),
		mcp.WithString("file_uri",
			mcp.Required(),
			mcp.Description("URI of the file"),
		),
	)

	s.AddTool(diagnosticsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, err := getArguments(request)
		if err != nil {
			return nil, err
		}

		fileURI, err := getStringArg(args, "file_uri")
		if err != nil {
			return nil, err
		}

		if !strings.HasPrefix(fileURI, "file://") {
			fileURI = convertPathToURI(fileURI)
		}

		lspClient := t.getClient()
		if lspClient == nil {
			return nil, fmt.Errorf("LSP client not initialized")
		}

		diagnostics, err := lspClient.GetDiagnostics(ctx, fileURI)
		if err != nil {
			if strings.Contains(err.Error(), "client closed") {
				return nil, fmt.Errorf("LSP service not available, please restart the server: %w", err)
			}
			return nil, fmt.Errorf("failed to get diagnostics: %w", err)
		}

		result, err := mcp.NewToolResultJSON(map[string]any{
			"file_uri":    fileURI,
			"diagnostics": diagnostics,
		})
		if err != nil {
			return nil, err
		}
		return result, nil
	})
}
