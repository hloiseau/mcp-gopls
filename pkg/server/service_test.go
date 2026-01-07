package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpsrv "github.com/mark3labs/mcp-go/server"

	"github.com/hloiseau/mcp-gopls/v2/pkg/lsp/client"
	"github.com/hloiseau/mcp-gopls/v2/pkg/lsp/protocol"
)

type stubLSPClient struct {
	initializeErr error
	closeErr      error
}

func (s *stubLSPClient) Initialize(ctx context.Context) error { return s.initializeErr }
func (s *stubLSPClient) Shutdown(ctx context.Context) error   { return nil }
func (s *stubLSPClient) Close(ctx context.Context) error      { return s.closeErr }
func (s *stubLSPClient) GoToDefinition(ctx context.Context, uri string, line, character int) ([]protocol.Location, error) {
	return nil, nil
}
func (s *stubLSPClient) FindReferences(ctx context.Context, uri string, line, character int, includeDeclaration bool) ([]protocol.Location, error) {
	return nil, nil
}
func (s *stubLSPClient) GetDiagnostics(ctx context.Context, uri string) ([]protocol.Diagnostic, error) {
	return nil, nil
}
func (s *stubLSPClient) DidOpen(ctx context.Context, uri, languageID, text string) error { return nil }
func (s *stubLSPClient) DidClose(ctx context.Context, uri string) error                  { return nil }
func (s *stubLSPClient) GetHover(ctx context.Context, uri string, line, character int) (string, error) {
	return "", nil
}
func (s *stubLSPClient) GetCompletion(ctx context.Context, uri string, line, character int) ([]string, error) {
	return nil, nil
}
func (s *stubLSPClient) DocumentFormatting(ctx context.Context, uri string) ([]protocol.TextEdit, error) {
	return nil, nil
}
func (s *stubLSPClient) Rename(ctx context.Context, uri string, line, character int, newName string) (*protocol.WorkspaceEdit, error) {
	return nil, nil
}
func (s *stubLSPClient) CodeActions(ctx context.Context, uri string, rng protocol.Range) ([]protocol.CodeAction, error) {
	return nil, nil
}
func (s *stubLSPClient) WorkspaceSymbols(ctx context.Context, query string) ([]protocol.SymbolInformation, error) {
	return nil, nil
}
func (s *stubLSPClient) OnDiagnostics(handler client.DiagnosticsHandler) func() { return func() {} }

func TestResourceDefinitions(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example.com/test"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.Mkdir(filepath.Join(tmp, "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	svc := &Service{config: Config{WorkspaceDir: tmp}}
	defs := svc.resourceDefinitions()
	if len(defs) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(defs))
	}

	var overview resourceDefinition
	var goMod resourceDefinition
	for _, def := range defs {
		switch def.resource.URI {
		case "resource://workspace/overview":
			overview = def
		case "resource://workspace/go.mod":
			goMod = def
		}
	}

	if overview.resource.URI == "" || goMod.resource.URI == "" {
		t.Fatal("missing expected resource definitions")
	}

	ctx := context.Background()
	contents, err := overview.handler(ctx, mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: overview.resource.URI}})
	if err != nil {
		t.Fatalf("overview handler error: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("expected one overview content, got %d", len(contents))
	}
	text := contents[0].(mcp.TextResourceContents).Text
	var summary struct {
		Root string `json:"root"`
	}
	if err := json.Unmarshal([]byte(text), &summary); err != nil {
		t.Fatalf("unmarshal overview: %v", err)
	}
	if !strings.HasSuffix(summary.Root, filepath.Base(tmp)) {
		t.Fatalf("expected root suffix %s, got %s", filepath.Base(tmp), summary.Root)
	}

	modContents, err := goMod.handler(ctx, mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: goMod.resource.URI}})
	if err != nil {
		t.Fatalf("gomod handler error: %v", err)
	}
	gotGoMod := modContents[0].(mcp.TextResourceContents).Text
	if !strings.Contains(gotGoMod, "module example.com/test") {
		t.Fatalf("unexpected go.mod contents %q", gotGoMod)
	}
}

func TestPromptDefinitions(t *testing.T) {
	svc := &Service{config: Config{WorkspaceDir: "/workspace"}}
	defs := svc.promptDefinitions()
	if len(defs) != 2 {
		t.Fatalf("expected two prompt definitions, got %d", len(defs))
	}

	var diag promptDefinition
	var refactor promptDefinition
	for _, def := range defs {
		switch def.prompt.Name {
		case "summarize_diagnostics":
			diag = def
		case "refactor_plan":
			refactor = def
		}
	}
	if diag.prompt.Name == "" || refactor.prompt.Name == "" {
		t.Fatal("missing prompt definitions")
	}

	res, err := diag.handler(context.Background(), mcp.GetPromptRequest{})
	if err != nil {
		t.Fatalf("diag handler error: %v", err)
	}
	if len(res.Messages) != 1 || !strings.Contains(res.Messages[0].Content.(mcp.TextContent).Text, "diagnostics") {
		t.Fatalf("unexpected diagnostics prompt message: %#v", res.Messages)
	}

	refReq := mcp.GetPromptRequest{
		Params: mcp.GetPromptParams{
			Arguments: map[string]string{"diagnostics": `{"issues":1}`},
		},
	}
	refRes, err := refactor.handler(context.Background(), refReq)
	if err != nil {
		t.Fatalf("refactor handler error: %v", err)
	}
	text := refRes.Messages[0].Content.(mcp.TextContent).Text
	if !strings.Contains(text, "/workspace") || !strings.Contains(text, "diagnostics") {
		t.Fatalf("unexpected refactor prompt text %q", text)
	}
}

func TestRegisterToolsUsesFactory(t *testing.T) {
	origFactory := newLSPTools
	t.Cleanup(func() { newLSPTools = origFactory })

	fake := &fakeToolset{}
	newLSPTools = func(client.LSPClient, string) toolRegistrar {
		return fake
	}

	svc := &Service{
		config:    Config{WorkspaceDir: "."},
		server:    mcpsrv.NewMCPServer("test", "1.0"),
		lspClient: &stubLSPClient{},
	}
	svc.RegisterTools()

	if !fake.setClientGetter {
		t.Fatal("expected client getter to be set")
	}
	if !fake.setResetFunc {
		t.Fatal("expected reset func to be set")
	}
	if fake.registeredWith != svc.server {
		t.Fatal("tools not registered with server")
	}
}

func TestResetLSPClientIfNeeded(t *testing.T) {
	origFactory := newLSPClient
	t.Cleanup(func() { newLSPClient = origFactory })

	var initCount int
	newLSPClient = func(opts ...client.Option) (client.LSPClient, error) {
		initCount++
		return &stubLSPClient{}, nil
	}

	svc := &Service{
		config: Config{WorkspaceDir: "."},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	if !svc.resetLSPClientIfNeeded(errors.New("client closed: io.EOF")) {
		t.Fatal("expected reset to trigger")
	}
	if initCount != 1 {
		t.Fatalf("expected init once, got %d", initCount)
	}
	if svc.resetLSPClientIfNeeded(nil) {
		t.Fatal("should not reset on nil errors")
	}
}

func TestServiceStartInvokesStdioServer(t *testing.T) {
	origFactory := newLSPTools
	origStdio := newStdioServer
	t.Cleanup(func() {
		newLSPTools = origFactory
		newStdioServer = origStdio
	})

	fakeTools := &fakeToolset{}
	newLSPTools = func(client.LSPClient, string) toolRegistrar {
		return fakeTools
	}

	fakeStdio := &fakeStdioServer{}
	newStdioServer = func(*mcpsrv.MCPServer) stdioServer {
		return fakeStdio
	}

	svc := &Service{
		config:    Config{WorkspaceDir: "."},
		server:    mcpsrv.NewMCPServer("test", "1.0"),
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		lspClient: &stubLSPClient{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("start returned error: %v", err)
	}

	if !fakeTools.registerCalled {
		t.Fatal("expected tools to register during Start")
	}
	if !fakeStdio.listenCalled {
		t.Fatal("expected stdio server Listen to be invoked")
	}
}

func TestConfigNormalize(t *testing.T) {
	tmp := t.TempDir()
	cfg := Config{
		WorkspaceDir: tmp,
	}
	if err := cfg.Normalize(); err != nil {
		t.Fatalf("normalize failed: %v", err)
	}
	if cfg.WorkspaceDir == tmp {
		if !filepath.IsAbs(cfg.WorkspaceDir) {
			t.Fatalf("expected absolute workspace, got %s", cfg.WorkspaceDir)
		}
	}
	if cfg.RPCTimeout != 45*time.Second || cfg.ShutdownTimeout != 15*time.Second {
		t.Fatalf("unexpected defaults %+v", cfg)
	}

	cfg.WorkspaceDir = filepath.Join(tmp, "missing")
	if err := cfg.Normalize(); err == nil {
		t.Fatal("expected error for invalid workspace")
	}
}

func TestBuildWorkspaceSummary(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	if err := os.Mkdir(filepath.Join(tmp, "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir pkg: %v", err)
	}

	summary, err := buildWorkspaceSummary(tmp)
	if err != nil {
		t.Fatalf("build summary error: %v", err)
	}
	if !strings.Contains(summary, `"GoFiles"`) && !strings.Contains(summary, "go_files") {
		t.Fatalf("summary missing go files: %s", summary)
	}
	if !strings.Contains(summary, "pkg") {
		t.Fatalf("summary missing directory: %s", summary)
	}
}

type fakeToolset struct {
	setClientGetter bool
	setResetFunc    bool
	registerCalled  bool
	registeredWith  *mcpsrv.MCPServer
}

func (f *fakeToolset) SetClientGetter(func() client.LSPClient) {
	f.setClientGetter = true
}

func (f *fakeToolset) SetResetFunc(func(error) bool) {
	f.setResetFunc = true
}

func (f *fakeToolset) Register(s *mcpsrv.MCPServer) {
	f.registerCalled = true
	f.registeredWith = s
}

type fakeStdioServer struct {
	listenCalled bool
}

func (f *fakeStdioServer) Listen(ctx context.Context, _ io.Reader, _ io.Writer) error {
	f.listenCalled = true
	return context.Canceled
}
