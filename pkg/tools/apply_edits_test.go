package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpsrv "github.com/mark3labs/mcp-go/server"

	"github.com/hloiseau/mcp-gopls/v2/pkg/lsp/protocol"
)

func TestApplyTextEditsSingleReplacement(t *testing.T) {
	content := "package main\n\nfunc oldName() {}\n"
	updated, err := applyTextEdits(content, []protocol.TextEdit{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 2, Character: 5},
				End:   protocol.Position{Line: 2, Character: 12},
			},
			NewText: "newName",
		},
	})
	if err != nil {
		t.Fatalf("applyTextEdits returned error: %v", err)
	}
	want := "package main\n\nfunc newName() {}\n"
	if updated != want {
		t.Fatalf("unexpected content:\ngot:  %q\nwant: %q", updated, want)
	}
}

func TestApplyTextEditsReverseOrder(t *testing.T) {
	content := "alpha beta gamma\n"
	updated, err := applyTextEdits(content, []protocol.TextEdit{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: 0, Character: 5},
			},
			NewText: "first",
		},
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 6},
				End:   protocol.Position{Line: 0, Character: 10},
			},
			NewText: "second",
		},
	})
	if err != nil {
		t.Fatalf("applyTextEdits returned error: %v", err)
	}
	want := "first second gamma\n"
	if updated != want {
		t.Fatalf("unexpected content:\ngot:  %q\nwant: %q", updated, want)
	}
}

func TestApplyWorkspaceEditChangesFormat(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "main.go")
	original := "package main\n\nfunc foo() {}\n"
	if err := os.WriteFile(filePath, []byte(original), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	fileURI := convertPathToURI(filePath)
	tools := NewLSPTools(nil, dir)

	result, err := tools.applyWorkspaceEdit(protocol.WorkspaceEdit{
		Changes: map[string][]protocol.TextEdit{
			fileURI: {
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 2, Character: 5},
						End:   protocol.Position{Line: 2, Character: 8},
					},
					NewText: "bar",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("applyWorkspaceEdit returned error: %v", err)
	}
	if len(result.Modified) != 1 {
		t.Fatalf("expected 1 modified file, got %#v", result.Modified)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("unexpected failures: %#v", result.Failed)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	want := "package main\n\nfunc bar() {}\n"
	if string(data) != want {
		t.Fatalf("unexpected file content:\ngot:  %q\nwant: %q", string(data), want)
	}
}

func TestApplyWorkspaceEditDocumentChangesFormat(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "main.go")
	original := "package main\n\nvar count = 1\n"
	if err := os.WriteFile(filePath, []byte(original), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	fileURI := convertPathToURI(filePath)
	tools := NewLSPTools(nil, dir)

	result, err := tools.applyWorkspaceEdit(protocol.WorkspaceEdit{
		DocumentChanges: []protocol.TextDocumentEdit{
			{
				TextDocument: protocol.OptionalVersionedTextDocumentIdentifier{URI: fileURI},
				Edits: []protocol.TextEdit{
					{
						Range: protocol.Range{
							Start: protocol.Position{Line: 2, Character: 4},
							End:   protocol.Position{Line: 2, Character: 9},
						},
						NewText: "total",
					},
				},
			},
		},
		Changes: map[string][]protocol.TextEdit{
			"file:///should-be-ignored.go": {
				{NewText: "ignored"},
			},
		},
	})
	if err != nil {
		t.Fatalf("applyWorkspaceEdit returned error: %v", err)
	}
	if len(result.Modified) != 1 {
		t.Fatalf("expected 1 modified file, got %#v", result.Modified)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	want := "package main\n\nvar total = 1\n"
	if string(data) != want {
		t.Fatalf("unexpected file content:\ngot:  %q\nwant: %q", string(data), want)
	}
}

func TestApplyWorkspaceEditRejectsPathOutsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "outside.go")
	if err := os.WriteFile(outsideFile, []byte("package outside\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	tools := NewLSPTools(nil, workspace)
	result, err := tools.applyWorkspaceEdit(protocol.WorkspaceEdit{
		Changes: map[string][]protocol.TextEdit{
			convertPathToURI(outsideFile): {
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 0, Character: 8},
						End:   protocol.Position{Line: 0, Character: 15},
					},
					NewText: "main",
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for path outside workspace")
	}
	if len(result.Failed) != 1 {
		t.Fatalf("expected 1 failure, got %#v", result.Failed)
	}
	if !strings.Contains(result.Failed[0].Error, "outside workspace") {
		t.Fatalf("unexpected failure message: %q", result.Failed[0].Error)
	}

	data, err := os.ReadFile(outsideFile)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "package outside\n" {
		t.Fatalf("outside file should be unchanged, got %q", string(data))
	}
}

func TestApplyWorkspaceEditToolRegistered(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	tools := NewLSPTools(nil, dir)
	server := mcpsrv.NewMCPServer("test", "1.0")
	tools.Register(server)

	tool := server.GetTool("apply_workspace_edit")
	if tool == nil {
		t.Fatal("apply_workspace_edit tool not registered")
	}

	fileURI := convertPathToURI(filePath)
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "apply_workspace_edit",
			Arguments: map[string]any{
				"workspace_edit": map[string]any{
					"changes": map[string]any{
						fileURI: []any{
							map[string]any{
								"range": map[string]any{
									"start": map[string]any{"line": float64(0), "character": float64(8)},
									"end":   map[string]any{"line": float64(0), "character": float64(12)},
								},
								"newText": "demo",
							},
						},
					},
				},
			},
		},
	}

	result, err := tool.Handler(context.Background(), request)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %#v", result)
	}

	payload := structured(result)
	modified, ok := payload["modified"].([]any)
	if !ok || len(modified) != 1 {
		t.Fatalf("unexpected modified files: %#v", payload["modified"])
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "package demo\n" {
		t.Fatalf("unexpected file content: %q", string(data))
	}
}
