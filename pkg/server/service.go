package server

import (
	"context"
	"fmt"
)

// NewService creates a fully configured MCP service ready to serve requests.
func NewService(cfg Config) (*Service, error) {
	if err := cfg.Normalize(); err != nil {
		return nil, err
	}

	logFile, logger, err := setupLogger(cfg)
	if err != nil {
		return nil, err
	}

	svc := &Service{
		config:  cfg,
		logger:  logger,
		logFile: logFile,
	}

	if err := svc.initLSPClient(context.Background()); err != nil {
		svc.cleanup(context.Background())
		return nil, fmt.Errorf("bootstrap lsp client: %w", err)
	}

	svc.server = setupServer(logger)
	svc.registerResources()
	svc.registerPrompts()
	return svc, nil
}
