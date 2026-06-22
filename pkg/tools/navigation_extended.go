package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func (t *LSPTools) registerGoToTypeDefinition(s *server.MCPServer) {
	typeDefinitionTool := mcp.NewTool("go_to_type_definition",
		mcp.WithDescription("Navigate to the type definition of a symbol"),
		mcp.WithTitleAnnotation("Go To Type Definition"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("file_uri",
			mcp.Required(),
			mcp.Description("URI of the file"),
		),
		mcp.WithObject("position",
			mcp.Required(),
			mcp.Description("Position of the symbol"),
		),
	)

	s.AddTool(typeDefinitionTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		locations, err := lspClient.GoToTypeDefinition(ctx, fileURI, line, character)
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

func (t *LSPTools) registerGoToImplementation(s *server.MCPServer) {
	implementationTool := mcp.NewTool("go_to_implementation",
		mcp.WithDescription("Navigate to the implementation of a symbol"),
		mcp.WithTitleAnnotation("Go To Implementation"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("file_uri",
			mcp.Required(),
			mcp.Description("URI of the file"),
		),
		mcp.WithObject("position",
			mcp.Required(),
			mcp.Description("Position of the symbol"),
		),
	)

	s.AddTool(implementationTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		locations, err := lspClient.GoToImplementation(ctx, fileURI, line, character)
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

func (t *LSPTools) registerDocumentSymbols(s *server.MCPServer) {
	documentSymbolsTool := mcp.NewTool("document_symbols",
		mcp.WithDescription("Get the hierarchical symbol outline of a document"),
		mcp.WithTitleAnnotation("Document Symbols"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("file_uri",
			mcp.Required(),
			mcp.Description("URI of the file"),
		),
	)

	s.AddTool(documentSymbolsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
			return nil, fmt.Errorf("LSP client not available")
		}

		symbols, err := lspClient.DocumentSymbols(ctx, fileURI)
		if err != nil {
			return nil, t.handleLSPError(err)
		}

		payload := map[string]any{
			"file_uri": fileURI,
			"symbols":  symbols,
		}
		result, err := mcp.NewToolResultJSON(payload)
		if err != nil {
			return nil, err
		}
		return result, nil
	})
}
