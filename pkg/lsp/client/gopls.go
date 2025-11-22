package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hloiseaufcms/mcp-gopls/internal/goenv"
	"github.com/hloiseaufcms/mcp-gopls/pkg/lsp/protocol"
)

const (
	defaultCallTimeout = 45 * time.Second
	clientName         = "mcp-gopls"
	clientVersion      = "2.0.0-dev"
)

// Option configures the gopls client.
type Option func(*clientOptions)

type clientOptions struct {
	executable   string
	workspaceDir string
	logger       *slog.Logger
	callTimeout  time.Duration
}

// WithExecutable overrides the gopls binary path.
func WithExecutable(path string) Option {
	return func(cfg *clientOptions) {
		cfg.executable = path
	}
}

// WithWorkspaceDir sets the workspace root used for gopls initialization.
func WithWorkspaceDir(dir string) Option {
	return func(cfg *clientOptions) {
		cfg.workspaceDir = dir
	}
}

// WithLogger injects a custom slog logger.
func WithLogger(logger *slog.Logger) Option {
	return func(cfg *clientOptions) {
		cfg.logger = logger
	}
}

// WithCallTimeout configures the RPC timeout used for blocking requests.
func WithCallTimeout(timeout time.Duration) Option {
	return func(cfg *clientOptions) {
		if timeout > 0 {
			cfg.callTimeout = timeout
		}
	}
}

// GoplsClient implements the LSPClient interface using a managed gopls process.
type GoplsClient struct {
	cmd          *exec.Cmd
	transport    *protocol.Transport
	logger       *slog.Logger
	callTimeout  time.Duration
	callOverride func(context.Context, string, any) (*protocol.JSONRPCMessage, error)

	workspaceDir string
	workspaceURI string

	sendMu      sync.Mutex
	nextID      atomic.Int64
	closed      atomic.Bool
	initialized atomic.Bool

	diagnosticsMu       sync.RWMutex
	diagnosticsCache    map[string][]protocol.Diagnostic
	diagnosticsHandlers map[int64]DiagnosticsHandler
	handlerCounter      atomic.Int64

	openedDocs sync.Map

	readerCtx    context.Context
	readerCancel context.CancelFunc
	readerDone   chan struct{}

	pendingMu sync.Mutex
	pending   map[int64]chan rpcResponse

	diagnosticsWaiters map[string][]chan struct{}

	closeOnce sync.Once
}

type rpcResponse struct {
	msg *protocol.JSONRPCMessage
	err error
}

// NewGoplsClient starts a new gopls subprocess and wires it to the MCP bridge.
func NewGoplsClient(opts ...Option) (*GoplsClient, error) {
	cfg := clientOptions{
		callTimeout: defaultCallTimeout,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.logger == nil {
		cfg.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	execPath, err := resolveGoplsExecutable(cfg.executable)
	if err != nil {
		return nil, fmt.Errorf("resolve gopls executable: %w", err)
	}

	workspaceDir, workspaceURI, err := resolveWorkspace(cfg.workspaceDir)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(execPath, "serve", "-rpc.trace", "-logfile=auto")
	cmd.Env = buildGoplsEnv(os.Environ())

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("create stderr pipe: %w", err)
	}

	go pipeLogs(cfg.logger.With("component", "gopls"), stderr)

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("start gopls: %w", err)
	}

	client := &GoplsClient{
		cmd:                 cmd,
		transport:           protocol.NewTransport(bufio.NewReader(stdout), bufio.NewWriter(stdin)),
		logger:              cfg.logger.With("component", "lsp_client"),
		callTimeout:         cfg.callTimeout,
		workspaceDir:        workspaceDir,
		workspaceURI:        workspaceURI,
		diagnosticsCache:    make(map[string][]protocol.Diagnostic),
		diagnosticsHandlers: make(map[int64]DiagnosticsHandler),
		pending:             make(map[int64]chan rpcResponse),
		diagnosticsWaiters:  make(map[string][]chan struct{}),
	}

	client.nextID.Store(1)
	client.closed.Store(false)

	client.readerCtx, client.readerCancel = context.WithCancel(context.Background())
	client.readerDone = make(chan struct{})
	go client.readerLoop()

	client.logger.Info("gopls client started",
		"exec", execPath,
		"workspace", workspaceDir,
	)
	return client, nil
}

func (c *GoplsClient) readerLoop() {
	defer close(c.readerDone)

	for {
		msg, err := c.transport.ReceiveMessage(c.readerCtx)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				c.logger.Warn("transport receive error", "error", err)
			}
			c.failAllPending(err)
			return
		}

		if msg == nil {
			continue
		}

		if msg.ID == nil {
			c.handleNotification(c.readerCtx, msg)
			continue
		}

		respID, ok := parseMessageID(msg.ID)
		if !ok {
			c.logger.Warn("response id has unexpected type", "id", msg.ID)
			continue
		}

		c.deliverResponse(respID, rpcResponse{msg: msg})
	}
}

func (c *GoplsClient) addPending(id int64, ch chan rpcResponse) error {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()

	if c.closed.Load() {
		return errors.New("client closed")
	}
	c.pending[id] = ch
	return nil
}

func (c *GoplsClient) removePending(id int64) {
	c.pendingMu.Lock()
	delete(c.pending, id)
	c.pendingMu.Unlock()
}

func (c *GoplsClient) deliverResponse(id int64, resp rpcResponse) {
	c.pendingMu.Lock()
	ch, ok := c.pending[id]
	if ok {
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()

	if !ok {
		return
	}

	select {
	case ch <- resp:
	default:
	}
}

func (c *GoplsClient) failAllPending(err error) {
	c.pendingMu.Lock()
	pending := c.pending
	c.pending = make(map[int64]chan rpcResponse)
	c.pendingMu.Unlock()

	if err == nil {
		err = errors.New("transport closed")
	}

	c.closed.Store(true)

	for _, ch := range pending {
		select {
		case ch <- rpcResponse{err: err}:
		default:
		}
	}
}

func (c *GoplsClient) call(ctx context.Context, method string, params any) (*protocol.JSONRPCMessage, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if c.callTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.callTimeout)
		defer cancel()
	}

	id := c.nextID.Add(1)
	respCh := make(chan rpcResponse, 1)

	if err := c.addPending(id, respCh); err != nil {
		return nil, err
	}

	if err := c.sendRequest(id, method, params); err != nil {
		c.removePending(id)
		return nil, err
	}

	select {
	case <-ctx.Done():
		c.removePending(id)
		return nil, ctx.Err()
	case resp := <-respCh:
		if resp.err != nil {
			return nil, resp.err
		}
		if resp.msg.Error != nil {
			return nil, fmt.Errorf("lsp error: %s (code %d)", resp.msg.Error.Message, resp.msg.Error.Code)
		}
		return resp.msg, nil
	}
}

func (c *GoplsClient) sendRequest(id int64, method string, params any) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	if c.closed.Load() {
		return errors.New("client closed")
	}
	if method != "initialize" && method != "shutdown" && !c.initialized.Load() {
		return errors.New("client not initialized")
	}

	req, err := protocol.NewRequest(id, method, params)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if err := c.transport.SendMessage(req); err != nil {
		c.closed.Store(true)
		return fmt.Errorf("send request: %w", err)
	}

	c.logger.Debug("sent request", "method", method, "id", id)
	return nil
}

func (c *GoplsClient) notify(ctx context.Context, method string, params any) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	if c.closed.Load() {
		return errors.New("client closed")
	}

	notif, err := protocol.NewNotification(method, params)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}

	if err := c.transport.SendMessage(notif); err != nil {
		c.closed.Store(true)
		return fmt.Errorf("send notification: %w", err)
	}

	c.logger.Debug("sent notification", "method", method)
	return nil
}

func (c *GoplsClient) invoke(ctx context.Context, method string, params any) (*protocol.JSONRPCMessage, error) {
	if c.callOverride != nil {
		return c.callOverride(ctx, method, params)
	}
	return c.call(ctx, method, params)
}

// Initialize satisfies the LSPClient interface.
func (c *GoplsClient) Initialize(ctx context.Context) error {
	if c.initialized.Load() {
		return nil
	}

	initParams := map[string]any{
		"processId": nil,
		"clientInfo": map[string]any{
			"name":    clientName,
			"version": clientVersion,
		},
		"rootUri": c.workspaceURI,
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"synchronization": map[string]any{
					"dynamicRegistration": true,
					"didSave":             true,
				},
				"completion": map[string]any{
					"dynamicRegistration": true,
					"completionItem": map[string]any{
						"snippetSupport": true,
					},
				},
				"hover": map[string]any{
					"dynamicRegistration": true,
					"contentFormat":       []string{"markdown", "plaintext"},
				},
				"definition": map[string]any{
					"dynamicRegistration": true,
				},
				"references": map[string]any{
					"dynamicRegistration": true,
				},
				"publishDiagnostics": map[string]any{
					"relatedInformation": true,
				},
			},
			"workspace": map[string]any{
				"applyEdit": true,
				"symbol": map[string]any{
					"dynamicRegistration": true,
				},
			},
		},
		"trace": "messages",
	}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		_, lastErr = c.call(ctx, "initialize", initParams)
		if lastErr == nil {
			c.initialized.Store(true)
			break
		}

		if !errors.Is(lastErr, context.DeadlineExceeded) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if lastErr != nil {
		return fmt.Errorf("initialize gopls: %w", lastErr)
	}

	if err := c.notify(ctx, "initialized", map[string]any{}); err != nil {
		c.initialized.Store(false)
		return fmt.Errorf("send initialized notification: %w", err)
	}

	c.logger.Info("gopls initialized", "workspace", c.workspaceDir)
	return nil
}

// Shutdown gracefully shuts gopls down.
func (c *GoplsClient) Shutdown(ctx context.Context) error {
	if !c.initialized.Load() {
		return nil
	}
	if _, err := c.call(ctx, "shutdown", nil); err != nil {
		return fmt.Errorf("shutdown gopls: %w", err)
	}
	return nil
}

// Close terminates the gopls process.
func (c *GoplsClient) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	var errs []error

	c.closeOnce.Do(func() {
		c.closed.Store(true)

		if c.readerCancel != nil {
			c.readerCancel()
		}
		if c.readerDone != nil {
			<-c.readerDone
		}

		if c.initialized.Load() {
			if err := c.Shutdown(ctx); err != nil {
				errs = append(errs, err)
			}
			if err := c.notify(ctx, "exit", map[string]any{}); err != nil {
				errs = append(errs, err)
			}
			c.initialized.Store(false)
		}

		if c.transport != nil {
			_ = c.transport.Close()
		}

		if c.cmd != nil && c.cmd.Process != nil {
			if err := c.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				errs = append(errs, fmt.Errorf("kill gopls: %w", err))
			}
			_, _ = c.cmd.Process.Wait()
		}
	})

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}

	return nil
}

// GoToDefinition implements LSPClient.
func (c *GoplsClient) GoToDefinition(ctx context.Context, uri string, line, character int) ([]protocol.Location, error) {
	params := protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
		Position: protocol.Position{
			Line:      line,
			Character: character,
		},
	}

	resp, err := c.invoke(ctx, "textDocument/definition", params)
	if err != nil {
		return nil, err
	}

	var locations []protocol.Location
	if err := resp.ParseResult(&locations); err != nil {
		return nil, fmt.Errorf("decode definition: %w", err)
	}
	return locations, nil
}

// FindReferences implements LSPClient.
func (c *GoplsClient) FindReferences(ctx context.Context, uri string, line, character int, includeDeclaration bool) ([]protocol.Location, error) {
	params := protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position: protocol.Position{
				Line:      line,
				Character: character,
			},
		},
		Context: protocol.ReferenceContext{
			IncludeDeclaration: includeDeclaration,
		},
	}

	resp, err := c.invoke(ctx, "textDocument/references", params)
	if err != nil {
		return nil, err
	}

	var locations []protocol.Location
	if err := resp.ParseResult(&locations); err != nil {
		return nil, fmt.Errorf("decode references: %w", err)
	}
	return locations, nil
}

func (c *GoplsClient) waitForDiagnostics(ctx context.Context, uri string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	c.diagnosticsMu.Lock()
	if _, ok := c.diagnosticsCache[uri]; ok {
		c.diagnosticsMu.Unlock()
		return nil
	}

	ch := make(chan struct{}, 1)
	c.diagnosticsWaiters[uri] = append(c.diagnosticsWaiters[uri], ch)
	c.diagnosticsMu.Unlock()

	select {
	case <-ctx.Done():
		c.removeDiagnosticsWaiter(uri, ch)
		return ctx.Err()
	case <-ch:
		return nil
	}
}

func (c *GoplsClient) removeDiagnosticsWaiter(uri string, target chan struct{}) {
	c.diagnosticsMu.Lock()
	defer c.diagnosticsMu.Unlock()

	waiters := c.diagnosticsWaiters[uri]
	for i, ch := range waiters {
		if ch == target {
			waiters = append(waiters[:i], waiters[i+1:]...)
			break
		}
	}

	if len(waiters) == 0 {
		delete(c.diagnosticsWaiters, uri)
		return
	}

	c.diagnosticsWaiters[uri] = waiters
}

// GetDiagnostics returns the cached diagnostics for the provided URI.
func (c *GoplsClient) GetDiagnostics(ctx context.Context, uri string) ([]protocol.Diagnostic, error) {
	opened, err := c.ensureDocumentOpen(ctx, uri, "go", "")
	if err != nil {
		return nil, err
	}
	if opened {
		defer c.DidClose(ctx, uri)
	}

	if err := c.waitForDiagnostics(ctx, uri); err != nil {
		return nil, err
	}

	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()

	items := c.diagnosticsCache[uri]
	cloned := make([]protocol.Diagnostic, len(items))
	copy(cloned, items)
	return cloned, nil
}

// DidOpen sends a textDocument/didOpen notification (idempotent).
func (c *GoplsClient) DidOpen(ctx context.Context, uri, languageID, text string) error {
	_, err := c.ensureDocumentOpen(ctx, uri, languageID, text)
	return err
}

func (c *GoplsClient) ensureDocumentOpen(ctx context.Context, uri, languageID, text string) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if uri == "" {
		return false, errors.New("uri is required")
	}

	if _, alreadyOpen := c.openedDocs.LoadOrStore(uri, struct{}{}); alreadyOpen {
		return false, nil
	}

	if text == "" {
		if filePath, err := uriToPath(uri); err == nil {
			if data, readErr := os.ReadFile(filePath); readErr == nil {
				text = string(data)
			} else {
				c.logger.Warn("unable to read file contents", "uri", uri, "error", readErr)
			}
		}
	}

	params := map[string]any{
		"textDocument": map[string]any{
			"uri":        uri,
			"languageId": languageID,
			"version":    1,
			"text":       text,
		},
	}

	if err := c.notify(ctx, "textDocument/didOpen", params); err != nil {
		c.openedDocs.Delete(uri)
		return false, err
	}

	return true, nil
}

// DidClose sends a textDocument/didClose notification and clears caches.
func (c *GoplsClient) DidClose(ctx context.Context, uri string) error {
	c.openedDocs.Delete(uri)
	c.diagnosticsMu.Lock()
	delete(c.diagnosticsCache, uri)
	c.diagnosticsMu.Unlock()
	params := map[string]any{
		"textDocument": map[string]any{
			"uri": uri,
		},
	}
	return c.notify(ctx, "textDocument/didClose", params)
}

// GetHover implements LSPClient.
func (c *GoplsClient) GetHover(ctx context.Context, uri string, line, character int) (string, error) {
	opened, err := c.ensureDocumentOpen(ctx, uri, "go", "")
	if err != nil {
		return "", err
	}
	if opened {
		defer c.DidClose(ctx, uri)
	}
	params := protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
		Position: protocol.Position{
			Line:      line,
			Character: character,
		},
	}

	resp, err := c.invoke(ctx, "textDocument/hover", params)
	if err != nil {
		return "", err
	}

	if resp.Result == nil || string(resp.Result) == "null" {
		return "", errors.New("no hover information available")
	}

	var payload map[string]any
	if err := resp.ParseResult(&payload); err != nil {
		return "", fmt.Errorf("decode hover: %w", err)
	}

	if contents, ok := payload["contents"]; ok {
		switch v := contents.(type) {
		case string:
			return v, nil
		case map[string]any:
			if value, ok := v["value"].(string); ok {
				return value, nil
			}
		case []any:
			if len(v) > 0 {
				if first, ok := v[0].(map[string]any); ok {
					if value, ok := first["value"].(string); ok {
						return value, nil
					}
				}
			}
		}
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// GetCompletion implements LSPClient.
func (c *GoplsClient) GetCompletion(ctx context.Context, uri string, line, character int) ([]string, error) {
	opened, err := c.ensureDocumentOpen(ctx, uri, "go", "")
	if err != nil {
		return nil, err
	}
	if opened {
		defer c.DidClose(ctx, uri)
	}
	params := protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
		Position: protocol.Position{
			Line:      line,
			Character: character,
		},
	}

	resp, err := c.invoke(ctx, "textDocument/completion", params)
	if err != nil {
		return nil, err
	}

	var payload map[string]any
	if err := resp.ParseResult(&payload); err != nil {
		return nil, fmt.Errorf("decode completion: %w", err)
	}

	var completions []string
	if items, ok := payload["items"].([]any); ok {
		for _, item := range items {
			if dict, ok := item.(map[string]any); ok {
				if label, ok := dict["label"].(string); ok {
					completions = append(completions, label)
				}
			}
		}
	}

	return completions, nil
}

func (c *GoplsClient) DocumentFormatting(ctx context.Context, uri string) ([]protocol.TextEdit, error) {
	params := protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
		Options: protocol.FormattingOptions{
			TabSize:      4,
			InsertSpaces: true,
		},
	}

	resp, err := c.invoke(ctx, "textDocument/formatting", params)
	if err != nil {
		return nil, err
	}

	var edits []protocol.TextEdit
	if err := resp.ParseResult(&edits); err != nil {
		return nil, fmt.Errorf("decode formatting edits: %w", err)
	}
	return edits, nil
}

func (c *GoplsClient) Rename(ctx context.Context, uri string, line, character int, newName string) (*protocol.WorkspaceEdit, error) {
	params := protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position: protocol.Position{
				Line:      line,
				Character: character,
			},
		},
		NewName: newName,
	}

	resp, err := c.invoke(ctx, "textDocument/rename", params)
	if err != nil {
		return nil, err
	}

	var edit protocol.WorkspaceEdit
	if err := resp.ParseResult(&edit); err != nil {
		return nil, fmt.Errorf("decode rename result: %w", err)
	}
	return &edit, nil
}

func (c *GoplsClient) CodeActions(ctx context.Context, uri string, rng protocol.Range) ([]protocol.CodeAction, error) {
	params := protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
		Range:        rng,
		Context: protocol.CodeActionContext{
			Diagnostics: []protocol.Diagnostic{},
		},
	}

	resp, err := c.invoke(ctx, "textDocument/codeAction", params)
	if err != nil {
		return nil, err
	}

	var actions []protocol.CodeAction
	if err := resp.ParseResult(&actions); err != nil {
		return nil, fmt.Errorf("decode code actions: %w", err)
	}
	return actions, nil
}

func (c *GoplsClient) WorkspaceSymbols(ctx context.Context, query string) ([]protocol.SymbolInformation, error) {
	params := protocol.WorkspaceSymbolParams{Query: query}
	resp, err := c.invoke(ctx, "workspace/symbol", params)
	if err != nil {
		return nil, err
	}

	var symbols []protocol.SymbolInformation
	if err := resp.ParseResult(&symbols); err != nil {
		return nil, fmt.Errorf("decode workspace symbols: %w", err)
	}
	return symbols, nil
}

// OnDiagnostics registers a handler for publishDiagnostics notifications.
func (c *GoplsClient) OnDiagnostics(handler DiagnosticsHandler) func() {
	if handler == nil {
		return func() {}
	}

	id := c.handlerCounter.Add(1)
	c.diagnosticsMu.Lock()
	c.diagnosticsHandlers[id] = handler
	c.diagnosticsMu.Unlock()

	return func() {
		c.diagnosticsMu.Lock()
		delete(c.diagnosticsHandlers, id)
		c.diagnosticsMu.Unlock()
	}
}

func (c *GoplsClient) handleNotification(ctx context.Context, msg *protocol.JSONRPCMessage) {
	switch msg.Method {
	case "textDocument/publishDiagnostics":
		var params protocol.PublishDiagnosticsParams
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			c.logger.Warn("failed to decode diagnostics", "error", err)
			return
		}
		c.updateDiagnostics(params)
	default:
		c.logger.Debug("ignoring notification", "method", msg.Method)
	}
}

func (c *GoplsClient) updateDiagnostics(params protocol.PublishDiagnosticsParams) {
	c.diagnosticsMu.Lock()
	c.diagnosticsCache[params.URI] = params.Diagnostics

	handlers := make([]DiagnosticsHandler, 0, len(c.diagnosticsHandlers))
	for _, h := range c.diagnosticsHandlers {
		handlers = append(handlers, h)
	}
	waiters := c.diagnosticsWaiters[params.URI]
	if len(waiters) > 0 {
		delete(c.diagnosticsWaiters, params.URI)
	}
	c.diagnosticsMu.Unlock()

	for _, waiter := range waiters {
		select {
		case waiter <- struct{}{}:
		default:
		}
	}

	for _, handler := range handlers {
		handler(params)
	}
}

func parseMessageID(id any) (int64, bool) {
	switch v := id.(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case json.Number:
		value, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return value, true
	default:
		return 0, false
	}
}

func resolveGoplsExecutable(explicit string) (string, error) {
	if candidate := strings.TrimSpace(explicit); candidate != "" {
		return validateGoplsPath(candidate)
	}

	if path, err := exec.LookPath("gopls"); err == nil {
		return path, nil
	}

	if path, ok := searchGoBinDirs(); ok {
		return path, nil
	}

	return "", errors.New("gopls not found; install via `go install golang.org/x/tools/gopls@latest` or set MCP_GOPLS_BIN")
}

func validateGoplsPath(candidate string) (string, error) {
	if path, err := exec.LookPath(candidate); err == nil {
		return path, nil
	}

	abs := candidate
	if !filepath.IsAbs(abs) {
		if cwd, err := os.Getwd(); err == nil {
			abs = filepath.Join(cwd, candidate)
		}
	}
	abs = filepath.Clean(abs)

	if isExecutableFile(abs) {
		return abs, nil
	}

	return "", fmt.Errorf("gopls executable not found at %q", candidate)
}

func searchGoBinDirs() (string, bool) {
	name := goplsBinaryName()
	for _, dir := range goBinDirs() {
		candidate := filepath.Join(dir, name)
		if isExecutableFile(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func goBinDirs() []string {
	var dirs []string
	seen := make(map[string]struct{})

	add := func(dir string) {
		if dir == "" {
			return
		}
		cleaned := filepath.Clean(dir)
		if _, ok := seen[cleaned]; ok {
			return
		}
		seen[cleaned] = struct{}{}
		dirs = append(dirs, cleaned)
	}

	add(os.Getenv("GOBIN"))

	for _, entry := range splitPathList(os.Getenv("GOPATH")) {
		if entry == "" {
			continue
		}
		add(filepath.Join(entry, "bin"))
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		add(filepath.Join(home, "go", "bin"))
	}

	return dirs
}

func splitPathList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return strings.Split(value, string(os.PathListSeparator))
}

func goplsBinaryName() string {
	if runtime.GOOS == "windows" {
		return "gopls.exe"
	}
	return "gopls"
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

func resolveWorkspace(dir string) (string, string, error) {
	var err error
	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			return "", "", fmt.Errorf("determine working directory: %w", err)
		}
	}

	dir, err = filepath.Abs(dir)
	if err != nil {
		return "", "", fmt.Errorf("resolve workspace path: %w", err)
	}

	dir = findGoRoot(dir)
	if stat, statErr := os.Stat(dir); statErr != nil || !stat.IsDir() {
		return "", "", fmt.Errorf("workspace directory invalid: %w", statErr)
	}

	return dir, pathToURI(dir), nil
}

func buildGoplsEnv(env []string) []string {
	cloned := append([]string(nil), env...)
	hasGoto := false
	pathIdx := -1

	for i, kv := range cloned {
		if strings.HasPrefix(kv, "GOTOOLCHAIN=") {
			hasGoto = true
		}
		if strings.HasPrefix(kv, "PATH=") {
			pathIdx = i
		}
	}

	if !hasGoto {
		cloned = append(cloned, "GOTOOLCHAIN=local")
	}

	goBin, err := goenv.GoBin()
	if err != nil || goBin == "" {
		return cloned
	}

	var currentPath string
	if pathIdx >= 0 {
		currentPath = strings.TrimPrefix(cloned[pathIdx], "PATH=")
	} else {
		currentPath = os.Getenv("PATH")
	}

	if strings.HasPrefix(currentPath, goBin) {
		return cloned
	}

	newPath := goBin
	if currentPath != "" {
		newPath = goBin + string(os.PathListSeparator) + currentPath
	}

	pathEntry := "PATH=" + newPath
	if pathIdx >= 0 {
		cloned[pathIdx] = pathEntry
	} else {
		cloned = append(cloned, pathEntry)
	}

	return cloned
}

func findGoRoot(start string) string {
	current := start
	for {
		if fileExists(filepath.Join(current, "go.work")) || fileExists(filepath.Join(current, "go.mod")) {
			return current
		}
		next := filepath.Dir(current)
		if next == current {
			return start
		}
		current = next
	}
}

func pathToURI(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		path = strings.ReplaceAll(path, "\\", "/")
		if len(path) >= 2 && path[1] == ':' {
			drive := strings.ToLower(string(path[0]))
			path = "/" + drive + ":" + path[2:]
		}
	}
	u := url.URL{Scheme: "file", Path: path}
	return u.String()
}

func uriToPath(uri string) (string, error) {
	if !strings.HasPrefix(uri, "file://") {
		return "", fmt.Errorf("unsupported uri: %s", uri)
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	path := parsed.Path
	if runtime.GOOS == "windows" {
		path = strings.TrimPrefix(path, "/")
		path = strings.ReplaceAll(path, "/", "\\")
	}
	return path, nil
}

func pipeLogs(logger *slog.Logger, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		logger.Debug(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		logger.Warn("stderr stream error", "error", err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
