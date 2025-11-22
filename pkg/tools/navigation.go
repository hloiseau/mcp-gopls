package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func (t *LSPTools) registerNavigationTools(s *server.MCPServer) {
	t.registerGoToDefinition(s)
	t.registerFindReferences(s)
}

func (t *LSPTools) registerGoToDefinition(s *server.MCPServer) {
	definitionTool := mcp.NewTool("go_to_definition",
		mcp.WithDescription("Navigate to the definition of a symbol"),
		mcp.WithString("file_uri",
			mcp.Required(),
			mcp.Description("URI of the file"),
		),
		mcp.WithObject("position",
			mcp.Required(),
			mcp.Description("Position of the symbol"),
		),
	)

	s.AddTool(definitionTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, err := getArguments(request)
		if err != nil {
			return nil, err
		}

		fileURI, err := getStringArg(args, "file_uri")
		if err != nil {
			return nil, err
		}

		line, character, err := parsePosition(args)
		if err != nil {
			return nil, err
		}

		if !strings.HasPrefix(fileURI, "file://") {
			fileURI = convertPathToURI(fileURI)
		}

		lspClient := t.getClient()
		if lspClient == nil {
			return nil, fmt.Errorf("LSP client not available")
		}

		locations, err := lspClient.GoToDefinition(ctx, fileURI, line, character)
		if err != nil {
			return nil, t.handleLSPError(err)
		}

		payload := map[string]any{
			"file_uri":  fileURI,
			"positions": locations,
		}
		result, err := mcp.NewToolResultJSON(payload)
		if err != nil {
			return nil, err
		}
		return result, nil
	})
}

func (t *LSPTools) registerFindReferences(s *server.MCPServer) {
	referencesTool := mcp.NewTool("find_references",
		mcp.WithDescription("Find all references to a symbol"),
		mcp.WithString("file_uri",
			mcp.Required(),
			mcp.Description("URI of the file"),
		),
		mcp.WithObject("position",
			mcp.Required(),
			mcp.Description("Position of the symbol"),
		),
	)

	s.AddTool(referencesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, err := getArguments(request)
		if err != nil {
			return nil, err
		}

		fileURI, err := getStringArg(args, "file_uri")
		if err != nil {
			return nil, err
		}

		line, character, err := parsePosition(args)
		if err != nil {
			return nil, err
		}

		if !strings.HasPrefix(fileURI, "file://") {
			fileURI = convertPathToURI(fileURI)
		}

		lspClient := t.getClient()
		if lspClient == nil {
			return nil, fmt.Errorf("LSP client not available")
		}

		locations, err := lspClient.FindReferences(ctx, fileURI, line, character, true)
		if err != nil {
			if strings.Contains(err.Error(), "client closed") {
				return nil, fmt.Errorf("LSP client not available, please restart the server: %w", err)
			}
			return nil, t.handleLSPError(err)
		}

		payload := map[string]any{
			"file_uri":   fileURI,
			"references": locations,
		}
		result, err := mcp.NewToolResultJSON(payload)
		if err != nil {
			return nil, err
		}
		return result, nil
	})
}
