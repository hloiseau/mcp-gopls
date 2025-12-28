package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func (t *LSPTools) registerRefactorTools(s *server.MCPServer) {
	t.registerFormatDocument(s)
	t.registerRenameSymbol(s)
	t.registerCodeActionsTool(s)
}

func (t *LSPTools) registerFormatDocument(s *server.MCPServer) {
	tool := mcp.NewTool("format_document",
		mcp.WithDescription("Return formatting edits for a Go file"),
		mcp.WithTitleAnnotation("Format Document"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("file_uri",
			mcp.Required(),
			mcp.Description("URI of the file to format"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		edits, err := lspClient.DocumentFormatting(ctx, fileURI)
		if err != nil {
			return nil, t.handleLSPError(err)
		}

		result, err := mcp.NewToolResultJSON(map[string]any{
			"file_uri": fileURI,
			"edits":    edits,
		})
		if err != nil {
			return nil, err
		}
		return result, nil
	})
}

func (t *LSPTools) registerRenameSymbol(s *server.MCPServer) {
	tool := mcp.NewTool("rename_symbol",
		mcp.WithDescription("Compute rename edits for a symbol"),
		mcp.WithTitleAnnotation("Rename Symbol"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("file_uri",
			mcp.Required(),
			mcp.Description("URI of the file"),
		),
		mcp.WithObject("position",
			mcp.Required(),
			mcp.Description("Position of the symbol"),
		),
		mcp.WithString("new_name",
			mcp.Required(),
			mcp.Description("New identifier name"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		newName, err := getStringArg(args, "new_name")
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

		edit, err := lspClient.Rename(ctx, fileURI, line, character, newName)
		if err != nil {
			return nil, t.handleLSPError(err)
		}

		payload := map[string]any{
			"file_uri": fileURI,
			"new_name": newName,
			"edits":    edit,
		}
		result, err := mcp.NewToolResultJSON(payload)
		if err != nil {
			return nil, err
		}
		return result, nil
	})
}

func (t *LSPTools) registerCodeActionsTool(s *server.MCPServer) {
	tool := mcp.NewTool("list_code_actions",
		mcp.WithDescription("List available code actions for a given range"),
		mcp.WithTitleAnnotation("List Code Actions"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("file_uri",
			mcp.Required(),
			mcp.Description("URI of the file"),
		),
		mcp.WithObject("range",
			mcp.Required(),
			mcp.Description("Range to inspect for code actions"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, err := getArguments(request)
		if err != nil {
			return nil, err
		}

		fileURI, err := getStringArg(args, "file_uri")
		if err != nil {
			return nil, err
		}

		rng, err := parseRangeArg(args, "range")
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

		actions, err := lspClient.CodeActions(ctx, fileURI, rng)
		if err != nil {
			return nil, t.handleLSPError(err)
		}

		result, err := mcp.NewToolResultJSON(map[string]any{
			"file_uri": fileURI,
			"actions":  actions,
		})
		if err != nil {
			return nil, err
		}
		return result, nil
	})
}
