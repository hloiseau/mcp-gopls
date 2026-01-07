package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpsrv "github.com/mark3labs/mcp-go/server"

	"github.com/hloiseau/mcp-gopls/v2/pkg/lsp/client"
	"github.com/hloiseau/mcp-gopls/v2/pkg/lsp/protocol"
)

func TestToolsEndToEnd(t *testing.T) {
	origLookup := lookupGovulncheckBinary
	t.Cleanup(func() { lookupGovulncheckBinary = origLookup })

	fakeClient := &fakeLSPClient{
		definitions: []protocol.Location{{URI: "file://tmp/main.go"}},
		references:  []protocol.Location{{URI: "file://tmp/main.go"}},
		diagnostics: []protocol.Diagnostic{{Message: "boom"}},
		hover:       "hover info",
		completions: []string{"CompleteMe"},
		edits:       []protocol.TextEdit{{NewText: "fmt"}},
		rename:      &protocol.WorkspaceEdit{Changes: map[string][]protocol.TextEdit{"file://tmp/main.go": {{NewText: "name"}}}},
		actions:     []protocol.CodeAction{{Title: "Fix"}},
		symbols:     []protocol.SymbolInformation{{Name: "Symbol"}},
	}

	tools := NewLSPTools(fakeClient, "/workspace")
	fakeRunner := &fakeCommandRunner{
		results: map[string]commandResult{
			"go test ./... -cover":                  {Command: []string{"go", "test", "./...", "-cover"}, Stdout: "ok"},
			"go test ./pkg -coverprofile cover.out": {Command: []string{"go", "test", "./pkg", "-coverprofile", "cover.out"}, Stdout: "ok"},
			"go tool cover -func cover.out":         {Command: []string{"go", "tool", "cover", "-func", "cover.out"}, Stdout: "ok"},
			"go test ./...":                         {Command: []string{"go", "test", "./..."}, Stdout: "ok"},
			"go mod tidy":                           {Command: []string{"go", "mod", "tidy"}, Stdout: "ok"},
			"/usr/bin/govulncheck ./...":            {Command: []string{"/usr/bin/govulncheck", "./..."}, Stdout: "ok"},
			"go mod graph":                          {Command: []string{"go", "mod", "graph"}, Stdout: "ok"},
		},
	}
	tools.commandRunner = fakeRunner.Run
	tools.clientGetter = func() client.LSPClient { return fakeClient }

	lookupGovulncheckBinary = func(string) (string, error) {
		return "/usr/bin/govulncheck", nil
	}

	server := mcpsrv.NewMCPServer("test", "1.0")
	tools.Register(server)

	meta := &mcp.Meta{ProgressToken: "token"}

	assertTool := func(name string, args map[string]any, check func(t *testing.T, content map[string]any)) {
		t.Helper()
		request := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      name,
				Arguments: args,
				Meta:      meta,
			},
		}

		tool := server.GetTool(name)
		if tool == nil {
			t.Fatalf("tool %s not registered", name)
		}
		result, err := tool.Handler(context.Background(), request)
		if err != nil {
			t.Fatalf("%s returned error: %v", name, err)
		}
		payload := structured(result)
		check(t, payload)
	}

	assertTool("go_to_definition", map[string]any{
		"file_uri": "file://tmp/main.go",
		"position": map[string]any{"line": 0, "character": 0},
	}, func(t *testing.T, content map[string]any) {
		if content["file_uri"] != "file://tmp/main.go" {
			t.Fatalf("unexpected file uri %v", content["file_uri"])
		}
	})

	assertTool("find_references", map[string]any{
		"file_uri": "file://tmp/main.go",
		"position": map[string]any{"line": 0, "character": 0},
	}, func(t *testing.T, content map[string]any) {
		if _, ok := content["references"]; !ok {
			t.Fatalf("missing references field")
		}
	})

	assertTool("check_diagnostics", map[string]any{
		"file_uri": "file://tmp/main.go",
	}, func(t *testing.T, content map[string]any) {
		if len(content["diagnostics"].([]any)) != 1 {
			t.Fatalf("unexpected diagnostics %#v", content["diagnostics"])
		}
	})

	assertTool("get_hover_info", map[string]any{
		"file_uri": "file://tmp/main.go",
		"position": map[string]any{"line": 0, "character": 0},
	}, func(t *testing.T, content map[string]any) {
		if content["hover"] != "hover info" {
			t.Fatalf("unexpected hover %#v", content)
		}
	})

	assertTool("get_completion", map[string]any{
		"file_uri": "file://tmp/main.go",
		"position": map[string]any{"line": 0, "character": 0},
	}, func(t *testing.T, content map[string]any) {
		if len(content["completions"].([]any)) != 1 {
			t.Fatalf("unexpected completions %#v", content)
		}
	})

	assertTool("format_document", map[string]any{
		"file_uri": "file://tmp/main.go",
	}, func(t *testing.T, content map[string]any) {
		if len(content["edits"].([]any)) != 1 {
			t.Fatalf("unexpected edits %#v", content)
		}
	})

	assertTool("rename_symbol", map[string]any{
		"file_uri": "file://tmp/main.go",
		"position": map[string]any{"line": 0, "character": 0},
		"new_name": "name",
	}, func(t *testing.T, content map[string]any) {
		if content["new_name"] != "name" {
			t.Fatalf("unexpected rename payload %#v", content)
		}
	})

	assertTool("list_code_actions", map[string]any{
		"file_uri": "file://tmp/main.go",
		"range": map[string]any{
			"start": map[string]any{"line": 0, "character": 0},
			"end":   map[string]any{"line": 0, "character": 1},
		},
	}, func(t *testing.T, content map[string]any) {
		if len(content["actions"].([]any)) != 1 {
			t.Fatalf("unexpected code actions %#v", content)
		}
	})

	assertTool("analyze_coverage", map[string]any{}, func(t *testing.T, content map[string]any) {
		if content["mode"] != "summary" {
			t.Fatalf("unexpected mode %#v", content["mode"])
		}
	})

	assertTool("run_go_test", map[string]any{}, func(t *testing.T, content map[string]any) {
		if content["target"] != "./..." {
			t.Fatalf("unexpected test target %#v", content["target"])
		}
	})

	assertTool("search_workspace_symbols", map[string]any{
		"query": "Symbol",
	}, func(t *testing.T, content map[string]any) {
		if len(content["symbols"].([]any)) != 1 {
			t.Fatalf("unexpected symbols %#v", content)
		}
	})

	assertTool("run_go_mod_tidy", map[string]any{}, func(t *testing.T, content map[string]any) {
		if _, ok := content["result"]; !ok {
			t.Fatalf("expected tidy result")
		}
	})

	assertTool("run_govulncheck", map[string]any{}, func(t *testing.T, content map[string]any) {
		if _, ok := content["result"]; !ok {
			t.Fatalf("expected govulncheck result")
		}
	})

	assertTool("module_graph", map[string]any{}, func(t *testing.T, content map[string]any) {
		if _, ok := content["result"]; !ok {
			t.Fatalf("expected module graph result")
		}
	})

	if len(fakeRunner.calls) == 0 {
		t.Fatal("expected command runner to be invoked")
	}
}

func structured(result *mcp.CallToolResult) map[string]any {
	data, _ := json.Marshal(result.StructuredContent)
	var payload map[string]any
	_ = json.Unmarshal(data, &payload)
	return payload
}

type fakeCommandRunner struct {
	results map[string]commandResult
	calls   []string
}

func (f *fakeCommandRunner) Run(t *LSPTools, ctx context.Context, srv *mcpsrv.MCPServer, token mcp.ProgressToken, name string, args ...string) (commandResult, error) {
	command := append([]string{name}, args...)
	key := strings.Join(command, " ")
	f.calls = append(f.calls, key)
	if res, ok := f.results[key]; ok {
		return res, nil
	}
	return commandResult{Command: command}, nil
}

type fakeLSPClient struct {
	definitions []protocol.Location
	references  []protocol.Location
	diagnostics []protocol.Diagnostic
	hover       string
	completions []string
	edits       []protocol.TextEdit
	rename      *protocol.WorkspaceEdit
	actions     []protocol.CodeAction
	symbols     []protocol.SymbolInformation
}

func (f *fakeLSPClient) Initialize(ctx context.Context) error { return nil }
func (f *fakeLSPClient) Shutdown(ctx context.Context) error   { return nil }
func (f *fakeLSPClient) Close(ctx context.Context) error      { return nil }
func (f *fakeLSPClient) GoToDefinition(ctx context.Context, uri string, line, character int) ([]protocol.Location, error) {
	return f.definitions, nil
}
func (f *fakeLSPClient) FindReferences(ctx context.Context, uri string, line, character int, includeDeclaration bool) ([]protocol.Location, error) {
	return f.references, nil
}
func (f *fakeLSPClient) GetDiagnostics(ctx context.Context, uri string) ([]protocol.Diagnostic, error) {
	return f.diagnostics, nil
}
func (f *fakeLSPClient) DidOpen(ctx context.Context, uri, languageID, text string) error { return nil }
func (f *fakeLSPClient) DidClose(ctx context.Context, uri string) error                  { return nil }
func (f *fakeLSPClient) GetHover(ctx context.Context, uri string, line, character int) (string, error) {
	return f.hover, nil
}
func (f *fakeLSPClient) GetCompletion(ctx context.Context, uri string, line, character int) ([]string, error) {
	return f.completions, nil
}
func (f *fakeLSPClient) DocumentFormatting(ctx context.Context, uri string) ([]protocol.TextEdit, error) {
	return f.edits, nil
}
func (f *fakeLSPClient) Rename(ctx context.Context, uri string, line, character int, newName string) (*protocol.WorkspaceEdit, error) {
	return f.rename, nil
}
func (f *fakeLSPClient) CodeActions(ctx context.Context, uri string, rng protocol.Range) ([]protocol.CodeAction, error) {
	return f.actions, nil
}
func (f *fakeLSPClient) WorkspaceSymbols(ctx context.Context, query string) ([]protocol.SymbolInformation, error) {
	return f.symbols, nil
}
func (f *fakeLSPClient) OnDiagnostics(handler client.DiagnosticsHandler) func() { return func() {} }
