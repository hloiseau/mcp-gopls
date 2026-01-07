package client

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/hloiseau/mcp-gopls/v2/pkg/lsp/protocol"
)

func TestNavigationAndRefactorHelpers(t *testing.T) {
	uri := "file://tmp/main.go"
	cases := []struct {
		name         string
		expectMethod string
		call         func(*GoplsClient) (any, error)
		response     any
		checkParams  func(*testing.T, any)
		verify       func(*testing.T, any)
	}{
		{
			name:         "definition",
			expectMethod: "textDocument/definition",
			call: func(c *GoplsClient) (any, error) {
				return c.GoToDefinition(context.Background(), uri, 1, 2)
			},
			response: []protocol.Location{{URI: uri}},
			checkParams: func(t *testing.T, params any) {
				t.Helper()
				p, ok := params.(protocol.TextDocumentPositionParams)
				if !ok || p.TextDocument.URI != uri || p.Position.Line != 1 {
					t.Fatalf("unexpected params %#v", params)
				}
			},
			verify: func(t *testing.T, result any) {
				t.Helper()
				locs := result.([]protocol.Location)
				if len(locs) != 1 || locs[0].URI != uri {
					t.Fatalf("unexpected locations %#v", locs)
				}
			},
		},
		{
			name:         "references",
			expectMethod: "textDocument/references",
			call: func(c *GoplsClient) (any, error) {
				return c.FindReferences(context.Background(), uri, 3, 4, true)
			},
			response: []protocol.Location{{URI: uri}},
			checkParams: func(t *testing.T, params any) {
				t.Helper()
				p, ok := params.(protocol.ReferenceParams)
				if !ok || !p.Context.IncludeDeclaration {
					t.Fatalf("unexpected reference params %#v", params)
				}
			},
			verify: func(t *testing.T, result any) {
				t.Helper()
				locs := result.([]protocol.Location)
				if len(locs) != 1 {
					t.Fatalf("unexpected references %#v", locs)
				}
			},
		},
		{
			name:         "hover",
			expectMethod: "textDocument/hover",
			call: func(c *GoplsClient) (any, error) {
				return c.GetHover(context.Background(), uri, 0, 0)
			},
			response: map[string]any{"contents": map[string]any{"value": "hover text"}},
			checkParams: func(t *testing.T, params any) {
				t.Helper()
				if _, ok := params.(protocol.TextDocumentPositionParams); !ok {
					t.Fatalf("unexpected hover params %#v", params)
				}
			},
			verify: func(t *testing.T, result any) {
				t.Helper()
				if result.(string) != "hover text" {
					t.Fatalf("unexpected hover %v", result)
				}
			},
		},
		{
			name:         "completion",
			expectMethod: "textDocument/completion",
			call: func(c *GoplsClient) (any, error) {
				return c.GetCompletion(context.Background(), uri, 1, 1)
			},
			response: map[string]any{
				"items": []any{
					map[string]any{"label": "Foo"},
				},
			},
			checkParams: func(t *testing.T, params any) {
				t.Helper()
				if _, ok := params.(protocol.TextDocumentPositionParams); !ok {
					t.Fatalf("unexpected completion params %#v", params)
				}
			},
			verify: func(t *testing.T, result any) {
				t.Helper()
				items := result.([]string)
				if len(items) != 1 || items[0] != "Foo" {
					t.Fatalf("unexpected completion %#v", items)
				}
			},
		},
		{
			name:         "formatting",
			expectMethod: "textDocument/formatting",
			call: func(c *GoplsClient) (any, error) {
				return c.DocumentFormatting(context.Background(), uri)
			},
			response: []protocol.TextEdit{{NewText: "fmt"}},
			checkParams: func(t *testing.T, params any) {
				t.Helper()
				if _, ok := params.(protocol.DocumentFormattingParams); !ok {
					t.Fatalf("unexpected formatting params %#v", params)
				}
			},
			verify: func(t *testing.T, result any) {
				t.Helper()
				edits := result.([]protocol.TextEdit)
				if len(edits) != 1 || edits[0].NewText != "fmt" {
					t.Fatalf("unexpected edits %#v", edits)
				}
			},
		},
		{
			name:         "rename",
			expectMethod: "textDocument/rename",
			call: func(c *GoplsClient) (any, error) {
				return c.Rename(context.Background(), uri, 2, 2, "bar")
			},
			response: protocol.WorkspaceEdit{Changes: map[string][]protocol.TextEdit{uri: {{NewText: "bar"}}}},
			checkParams: func(t *testing.T, params any) {
				t.Helper()
				if p, ok := params.(protocol.RenameParams); !ok || p.NewName != "bar" {
					t.Fatalf("unexpected rename params %#v", params)
				}
			},
			verify: func(t *testing.T, result any) {
				t.Helper()
				edit := result.(*protocol.WorkspaceEdit)
				if len(edit.Changes[uri]) != 1 {
					t.Fatalf("unexpected rename changes %#v", edit)
				}
			},
		},
		{
			name:         "code actions",
			expectMethod: "textDocument/codeAction",
			call: func(c *GoplsClient) (any, error) {
				return c.CodeActions(context.Background(), uri, protocol.Range{})
			},
			response: []protocol.CodeAction{{Title: "Fix"}},
			checkParams: func(t *testing.T, params any) {
				t.Helper()
				if _, ok := params.(protocol.CodeActionParams); !ok {
					t.Fatalf("unexpected code action params %#v", params)
				}
			},
			verify: func(t *testing.T, result any) {
				t.Helper()
				actions := result.([]protocol.CodeAction)
				if len(actions) != 1 || actions[0].Title != "Fix" {
					t.Fatalf("unexpected actions %#v", actions)
				}
			},
		},
		{
			name:         "workspace symbols",
			expectMethod: "workspace/symbol",
			call: func(c *GoplsClient) (any, error) {
				return c.WorkspaceSymbols(context.Background(), "Foo")
			},
			response: []protocol.SymbolInformation{{Name: "Foo"}},
			checkParams: func(t *testing.T, params any) {
				t.Helper()
				if p, ok := params.(protocol.WorkspaceSymbolParams); !ok || p.Query != "Foo" {
					t.Fatalf("unexpected workspace symbol params %#v", params)
				}
			},
			verify: func(t *testing.T, result any) {
				t.Helper()
				syms := result.([]protocol.SymbolInformation)
				if len(syms) != 1 || syms[0].Name != "Foo" {
					t.Fatalf("unexpected symbols %#v", syms)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &GoplsClient{
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}
			client.openedDocs.Store(uri, struct{}{})
			client.callOverride = func(ctx context.Context, method string, params any) (*protocol.JSONRPCMessage, error) {
				if method != tc.expectMethod {
					t.Fatalf("expected method %s, got %s", tc.expectMethod, method)
				}
				tc.checkParams(t, params)
				data, err := json.Marshal(tc.response)
				if err != nil {
					t.Fatalf("marshal response: %v", err)
				}
				return &protocol.JSONRPCMessage{Result: data}, nil
			}

			result, err := tc.call(client)
			if err != nil {
				t.Fatalf("call returned error: %v", err)
			}
			tc.verify(t, result)
		})
	}
}
