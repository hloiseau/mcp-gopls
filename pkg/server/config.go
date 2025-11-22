package server

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// Config controls the behaviour of the MCP <-> gopls bridge.
type Config struct {
	WorkspaceDir    string
	GoplsPath       string
	LogFile         string
	LogJSON         bool
	LogLevel        slog.Level
	ShutdownTimeout time.Duration
	RPCTimeout      time.Duration
}

// DefaultConfig returns sensible defaults for local development.
func DefaultConfig() Config {
	return Config{
		WorkspaceDir:    ".",
		LogLevel:        slog.LevelInfo,
		LogJSON:         false,
		ShutdownTimeout: 15 * time.Second,
		RPCTimeout:      45 * time.Second,
	}
}

// Normalize validates and normalizes the configuration.
func (c *Config) Normalize() error {
	if c.WorkspaceDir == "" {
		c.WorkspaceDir = "."
	}

	abs, err := filepath.Abs(c.WorkspaceDir)
	if err != nil {
		return fmt.Errorf("resolve workspace dir: %w", err)
	}

	if stat, statErr := os.Stat(abs); statErr != nil || !stat.IsDir() {
		return fmt.Errorf("workspace dir invalid: %w", statErr)
	}
	c.WorkspaceDir = abs

	if c.ShutdownTimeout <= 0 {
		c.ShutdownTimeout = 15 * time.Second
	}
	if c.RPCTimeout <= 0 {
		c.RPCTimeout = 45 * time.Second
	}

	return nil
}
