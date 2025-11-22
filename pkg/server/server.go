package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	mcpsrv "github.com/mark3labs/mcp-go/server"

	"github.com/hloiseaufcms/mcp-gopls/pkg/lsp/client"
	"github.com/hloiseaufcms/mcp-gopls/pkg/tools"
)

var (
	newLSPClient = func(opts ...client.Option) (client.LSPClient, error) {
		return client.NewGoplsClient(opts...)
	}

	newLSPTools = func(lsp client.LSPClient, workspace string) toolRegistrar {
		return tools.NewLSPTools(lsp, workspace)
	}

	newStdioServer = func(s *mcpsrv.MCPServer) stdioServer {
		return &stdioServerAdapter{inner: mcpsrv.NewStdioServer(s)}
	}
)

type toolRegistrar interface {
	SetClientGetter(func() client.LSPClient)
	SetResetFunc(func(error) bool)
	Register(*mcpsrv.MCPServer)
}

type stdioServer interface {
	Listen(context.Context, io.Reader, io.Writer) error
}

type stdioServerAdapter struct {
	inner *mcpsrv.StdioServer
}

func (a *stdioServerAdapter) Listen(ctx context.Context, in io.Reader, out io.Writer) error {
	return a.inner.Listen(ctx, in, out)
}

type Service struct {
	config Config

	server  *mcpsrv.MCPServer
	logger  *slog.Logger
	logFile *os.File

	lspClient   client.LSPClient
	clientMutex sync.RWMutex
}

func (s *Service) initLSPClient(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	if s.lspClient != nil {
		_ = s.lspClient.Close(context.Background())
		s.lspClient = nil
	}

	opts := []client.Option{
		client.WithWorkspaceDir(s.config.WorkspaceDir),
		client.WithLogger(s.logger.With("component", "gopls")),
		client.WithCallTimeout(s.config.RPCTimeout),
	}
	if s.config.GoplsPath != "" {
		opts = append(opts, client.WithExecutable(s.config.GoplsPath))
	}

	lspClient, err := newLSPClient(opts...)
	if err != nil {
		return fmt.Errorf("create lsp client: %w", err)
	}

	initCtx, cancel := context.WithTimeout(ctx, s.config.ShutdownTimeout)
	defer cancel()

	if err := lspClient.Initialize(initCtx); err != nil {
		_ = lspClient.Close(context.Background())
		return fmt.Errorf("initialize lsp client: %w", err)
	}

	s.logger.Info("lsp client initialized")
	s.lspClient = lspClient
	return nil
}

func (s *Service) resetLSPClientIfNeeded(err error) bool {
	if err == nil {
		return false
	}

	if strings.Contains(err.Error(), "client closed") || strings.Contains(err.Error(), "not initialized") {
		s.logger.Warn("detected closed LSP client, reinitializing", "error", err)
		if initErr := s.initLSPClient(context.Background()); initErr != nil {
			s.logger.Error("failed to reinitialize LSP client", "error", initErr)
			return false
		}
		return true
	}

	return false
}

func (s *Service) GetLSPClient() client.LSPClient {
	s.clientMutex.RLock()
	defer s.clientMutex.RUnlock()
	return s.lspClient
}

func (s *Service) RegisterTools() {
	lspTools := newLSPTools(s.GetLSPClient(), s.config.WorkspaceDir)
	lspTools.SetClientGetter(func() client.LSPClient {
		return s.GetLSPClient()
	})
	lspTools.SetResetFunc(func(err error) bool {
		return s.resetLSPClientIfNeeded(err)
	})
	lspTools.Register(s.server)
}

func (s *Service) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.RegisterTools()

	stdioServer := newStdioServer(s.server)

	s.logger.Info("serving MCP over stdio")
	if err := stdioServer.Listen(ctx, os.Stdin, os.Stdout); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func (s *Service) Close(ctx context.Context) {
	s.cleanup(ctx)
}

func (s *Service) cleanup(ctx context.Context) {
	s.clientMutex.Lock()
	client := s.lspClient
	s.lspClient = nil
	s.clientMutex.Unlock()

	if client != nil {
		_ = client.Close(ctx)
	}

	if s.logFile != nil {
		_ = s.logFile.Close()
		s.logFile = nil
	}
}

func setupLogger(cfg Config) (*os.File, *slog.Logger, error) {
	var writer io.Writer = os.Stdout
	var file *os.File
	if cfg.LogFile != "" {
		logFile, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, nil, fmt.Errorf("open log file: %w", err)
		}
		writer = logFile
		file = logFile
	}

	handlerOpts := &slog.HandlerOptions{Level: cfg.LogLevel}
	var handler slog.Handler
	if cfg.LogJSON {
		handler = slog.NewJSONHandler(writer, handlerOpts)
	} else {
		handler = slog.NewTextHandler(writer, handlerOpts)
	}

	return file, slog.New(handler), nil
}

func setupServer(cfg Config, logger *slog.Logger) *mcpsrv.MCPServer {
	srv := mcpsrv.NewMCPServer(
		"MCP LSP Go",
		"2.0.0",
		mcpsrv.WithLogging(),
		mcpsrv.WithToolCapabilities(true),
		mcpsrv.WithResourceCapabilities(true, true),
		mcpsrv.WithPromptCapabilities(true),
	)

	if logger != nil {
		logger.Info("MCP server initialized")
	}

	return srv
}
