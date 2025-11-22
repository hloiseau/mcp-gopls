package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/hloiseaufcms/mcp-gopls/internal/goenv"
	"github.com/hloiseaufcms/mcp-gopls/pkg/lsp/client"
	"github.com/hloiseaufcms/mcp-gopls/pkg/lsp/protocol"
)

type commandRunner func(*LSPTools, context.Context, *server.MCPServer, mcp.ProgressToken, string, ...string) (commandResult, error)

type LSPTools struct {
	client        client.LSPClient
	clientGetter  func() client.LSPClient
	resetFunc     func(error) bool
	workspaceDir  string
	commandRunner commandRunner
}

func NewLSPTools(lspClient client.LSPClient, workspaceDir string) *LSPTools {
	return &LSPTools{
		client:        lspClient,
		clientGetter:  func() client.LSPClient { return lspClient },
		resetFunc:     func(error) bool { return false },
		workspaceDir:  workspaceDir,
		commandRunner: defaultCommandRunner,
	}
}

func (t *LSPTools) SetClientGetter(getter func() client.LSPClient) {
	t.clientGetter = getter
}

func (t *LSPTools) SetResetFunc(resetFunc func(error) bool) {
	t.resetFunc = resetFunc
}

func (t *LSPTools) getClient() client.LSPClient {
	if t.clientGetter != nil {
		return t.clientGetter()
	}
	return t.client
}

func (t *LSPTools) handleLSPError(err error) error {
	if err != nil {
		if t.resetFunc != nil && t.resetFunc(err) {
			return fmt.Errorf("LSP error (client reinitialized, please try again): %w", err)
		}

		return fmt.Errorf("LSP error: %w", err)
	}
	return nil
}

func (t *LSPTools) Register(s *server.MCPServer) {
	t.registerNavigationTools(s)
	t.registerDiagnosticsTools(s)
	t.registerInsightTools(s)
	t.registerTestingTools(s)
	t.registerRefactorTools(s)
	t.registerWorkspaceTools(s)
}

func convertPathToURI(path string) string {
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err == nil {
			path = filepath.Join(cwd, path)
		}
	}

	path = filepath.Clean(path)

	if !strings.HasPrefix(path, "file://") {
		if filepath.Separator == '\\' {
			path = strings.ReplaceAll(path, "\\", "/")
			if len(path) >= 2 && path[1] == ':' {
				drive := strings.ToLower(string(path[0]))
				path = "/" + drive + ":" + path[2:]
			}
			path = "/" + strings.TrimPrefix(path, "/")
		}
		path = "file://" + path
	}

	return path
}

func getArguments(request mcp.CallToolRequest) (map[string]any, error) {
	args := request.GetArguments()
	if args == nil {
		return nil, errors.New("arguments must be an object")
	}
	return args, nil
}

func getStringArg(args map[string]any, key string) (string, error) {
	val, ok := args[key]
	if !ok {
		return "", fmt.Errorf("%s argument is required", key)
	}
	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	return str, nil
}

func getObjectArg(args map[string]any, key string) (map[string]any, error) {
	val, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("%s argument is required", key)
	}
	obj, ok := val.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", key)
	}
	return obj, nil
}

func getIntFromObject(obj map[string]any, key string) (int, error) {
	val, ok := obj[key]
	if !ok {
		return 0, fmt.Errorf("%s is required", key)
	}
	switch v := val.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	default:
		return 0, fmt.Errorf("%s must be a number", key)
	}
}

func parsePosition(args map[string]any) (int, int, error) {
	position, err := getObjectArg(args, "position")
	if err != nil {
		return 0, 0, err
	}
	line, err := getIntFromObject(position, "line")
	if err != nil {
		return 0, 0, err
	}
	character, err := getIntFromObject(position, "character")
	if err != nil {
		return 0, 0, err
	}
	return line, character, nil
}

func parseRangeArg(args map[string]any, key string) (protocol.Range, error) {
	rangeObj, err := getObjectArg(args, key)
	if err != nil {
		return protocol.Range{}, err
	}

	startObj, err := getObjectArg(rangeObj, "start")
	if err != nil {
		return protocol.Range{}, err
	}
	endObj, err := getObjectArg(rangeObj, "end")
	if err != nil {
		return protocol.Range{}, err
	}

	startLine, err := getIntFromObject(startObj, "line")
	if err != nil {
		return protocol.Range{}, err
	}
	startChar, err := getIntFromObject(startObj, "character")
	if err != nil {
		return protocol.Range{}, err
	}
	endLine, err := getIntFromObject(endObj, "line")
	if err != nil {
		return protocol.Range{}, err
	}
	endChar, err := getIntFromObject(endObj, "character")
	if err != nil {
		return protocol.Range{}, err
	}

	return protocol.Range{
		Start: protocol.Position{Line: startLine, Character: startChar},
		End:   protocol.Position{Line: endLine, Character: endChar},
	}, nil
}

type commandResult struct {
	Command  []string `json:"command"`
	ExitCode int      `json:"exit_code"`
	Stdout   string   `json:"stdout"`
	Stderr   string   `json:"stderr"`
	Duration string   `json:"duration"`
}

func (t *LSPTools) runCommand(ctx context.Context, srv *server.MCPServer, token mcp.ProgressToken, name string, args ...string) (commandResult, error) {
	return t.commandRunner(t, ctx, srv, token, name, args...)
}

func defaultCommandRunner(t *LSPTools, ctx context.Context, srv *server.MCPServer, token mcp.ProgressToken, name string, args ...string) (commandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if t.workspaceDir != "" {
		cmd.Dir = t.workspaceDir
	}
	cmd.Env = ensureLocalToolchainEnv(os.Environ())

	var stdout, stderr bytes.Buffer
	stdoutEmitter := newLineEmitter(ctx, srv, token, "stdout")
	stderrEmitter := newLineEmitter(ctx, srv, token, "stderr")
	cmd.Stdout = io.MultiWriter(&stdout, stdoutEmitter)
	cmd.Stderr = io.MultiWriter(&stderr, stderrEmitter)

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	stdoutEmitter.flush()
	stderrEmitter.flush()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	result := commandResult{
		Command:  append([]string{name}, args...),
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration.String(),
	}

	if err != nil {
		return result, err
	}
	return result, nil
}

type lineEmitter struct {
	ctx    context.Context
	srv    *server.MCPServer
	token  mcp.ProgressToken
	stream string
	buf    bytes.Buffer
}

func newLineEmitter(ctx context.Context, srv *server.MCPServer, token mcp.ProgressToken, stream string) *lineEmitter {
	return &lineEmitter{
		ctx:    ctx,
		srv:    srv,
		token:  token,
		stream: stream,
	}
}

func (e *lineEmitter) Write(p []byte) (int, error) {
	total := len(p)
	for len(p) > 0 {
		if idx := bytes.IndexByte(p, '\n'); idx >= 0 {
			e.buf.Write(p[:idx])
			e.emitLine(e.buf.String())
			e.buf.Reset()
			p = p[idx+1:]
			continue
		}
		e.buf.Write(p)
		break
	}
	return total, nil
}

func (e *lineEmitter) emitLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	sendProgressNotification(e.ctx, e.srv, e.token, fmt.Sprintf("[%s] %s", e.stream, line))
}

func (e *lineEmitter) flush() {
	if e.buf.Len() == 0 {
		return
	}
	e.emitLine(e.buf.String())
	e.buf.Reset()
}

func ensureLocalToolchainEnv(env []string) []string {
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

	currentPath := os.Getenv("PATH")
	if pathIdx >= 0 {
		currentPath = strings.TrimPrefix(cloned[pathIdx], "PATH=")
	}

	updatedPath := prependPreferredGoBins(currentPath)
	pathEntry := "PATH=" + updatedPath
	if pathIdx >= 0 {
		cloned[pathIdx] = pathEntry
	} else {
		cloned = append(cloned, pathEntry)
	}

	return cloned
}

func prependPreferredGoBins(pathValue string) string {
	preferred := preferredGoBinDirs()
	if len(preferred) == 0 {
		return pathValue
	}

	entries := splitPathEntries(pathValue)
	seen := make(map[string]struct{}, len(entries)+len(preferred))
	var result []string

	add := func(entry string) {
		if entry == "" {
			return
		}
		entry = filepath.Clean(entry)
		if _, ok := seen[entry]; ok {
			return
		}
		seen[entry] = struct{}{}
		result = append(result, entry)
	}

	for _, dir := range preferred {
		add(dir)
	}
	for _, entry := range entries {
		add(entry)
	}

	return strings.Join(result, string(os.PathListSeparator))
}

func preferredGoBinDirs() []string {
	candidates := []string{
		filepath.Join("/usr/local/go", "bin"),
	}

	if goBin, err := goenv.GoBin(); err == nil && goBin != "" {
		candidates = append(candidates, goBin)
	}

	var dirs []string
	seen := make(map[string]struct{}, len(candidates))
	for _, dir := range candidates {
		dir = filepath.Clean(dir)
		if dir == "" {
			continue
		}
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		if hasGoBinary(dir) {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

func hasGoBinary(dir string) bool {
	if dir == "" {
		return false
	}
	goPath := filepath.Join(dir, "go")
	if runtime.GOOS == "windows" {
		goPath += ".exe"
	}
	if info, err := os.Stat(goPath); err == nil && !info.IsDir() {
		return true
	}
	return false
}

func splitPathEntries(pathValue string) []string {
	if strings.TrimSpace(pathValue) == "" {
		return nil
	}
	return strings.Split(pathValue, string(os.PathListSeparator))
}

func (t *LSPTools) commandFailureResult(action string, result commandResult, err error) (*mcp.CallToolResult, error) {
	message := buildCommandErrorMessage(action, result, err)
	return mcp.NewToolResultError(message), nil
}

func buildCommandErrorMessage(action string, result commandResult, err error) string {
	var builder strings.Builder

	name := strings.TrimSpace(action)
	if name == "" {
		name = strings.Join(result.Command, " ")
		name = strings.TrimSpace(name)
	}
	if name == "" {
		name = "command"
	}

	builder.WriteString(fmt.Sprintf("%s failed", name))
	if result.ExitCode != 0 {
		builder.WriteString(fmt.Sprintf(" (exit code %d)", result.ExitCode))
	}
	if err != nil {
		builder.WriteString(fmt.Sprintf(": %v", err))
	}

	if stderr := strings.TrimSpace(result.Stderr); stderr != "" {
		builder.WriteString("\nstderr:\n")
		builder.WriteString(limitOutputLines(stderr, 20))
	}
	return builder.String()
}

func limitOutputLines(output string, max int) string {
	if max <= 0 {
		return output
	}
	lines := strings.Split(output, "\n")
	if len(lines) <= max {
		return output
	}
	truncated := append([]string{"... (stderr truncated) ..."}, lines[len(lines)-max:]...)
	return strings.Join(truncated, "\n")
}
