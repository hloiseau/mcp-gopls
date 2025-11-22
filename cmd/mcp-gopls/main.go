package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hloiseaufcms/mcp-gopls/pkg/server"
)

type serviceRunner interface {
	Start(context.Context) error
	Close(context.Context)
}

var (
	newServiceFn   = func(cfg server.Config) (serviceRunner, error) { return server.NewService(cfg) }
	notifyContextF = signal.NotifyContext
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("%v", err)
	}
}

func run() error {
	cfg, err := buildConfigFromFlags()
	if err != nil {
		return err
	}

	svc, err := newServiceFn(cfg)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer svc.Close(context.Background())

	ctx, stop := notifyContextF(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := svc.Start(ctx); err != nil {
		return fmt.Errorf("service error: %w", err)
	}

	fmt.Println("mcp-gopls shutdown complete")
	return nil
}

func buildConfigFromFlags() (server.Config, error) {
	var (
		flagWorkspace       = flag.String("workspace", envOrDefault("MCP_GOPLS_WORKSPACE", ""), "Workspace root (default: current directory)")
		flagGoplsPath       = flag.String("gopls-path", envOrDefault("MCP_GOPLS_BIN", ""), "Path to gopls binary")
		flagLogFile         = flag.String("log-file", envOrDefault("MCP_GOPLS_LOG_FILE", ""), "Log file path")
		flagLogLevel        = flag.String("log-level", envOrDefault("MCP_GOPLS_LOG_LEVEL", "info"), "Log level (debug, info, warn, error)")
		flagLogJSON         = flag.Bool("log-json", envBool("MCP_GOPLS_LOG_JSON"), "Emit JSON logs")
		flagRPCTimeout      = flag.Duration("rpc-timeout", envDuration("MCP_GOPLS_RPC_TIMEOUT", 45*time.Second), "LSP RPC timeout")
		flagShutdownTimeout = flag.Duration("shutdown-timeout", envDuration("MCP_GOPLS_SHUTDOWN_TIMEOUT", 15*time.Second), "Graceful shutdown timeout")
	)
	flag.Parse()

	cfg := server.DefaultConfig()
	if *flagWorkspace != "" {
		if err := ensureDirectory(*flagWorkspace); err != nil {
			return server.Config{}, err
		}
		cfg.WorkspaceDir = *flagWorkspace
	}
	if *flagGoplsPath != "" {
		resolved, err := resolveExecutable(*flagGoplsPath)
		if err != nil {
			return server.Config{}, err
		}
		cfg.GoplsPath = resolved
	}
	if *flagLogFile != "" {
		cfg.LogFile = *flagLogFile
	}
	cfg.LogJSON = *flagLogJSON
	if flagRPCTimeout != nil {
		cfg.RPCTimeout = *flagRPCTimeout
	}
	if flagShutdownTimeout != nil {
		cfg.ShutdownTimeout = *flagShutdownTimeout
	}

	level, err := parseLogLevel(*flagLogLevel)
	if err != nil {
		return server.Config{}, fmt.Errorf("parse log level: %w", err)
	}
	cfg.LogLevel = level

	if err := validateTimeouts(cfg); err != nil {
		return server.Config{}, err
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string) bool {
	value := os.Getenv(key)
	value = strings.ToLower(value)
	return value == "1" || value == "true" || value == "yes"
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return fallback
}

func parseLogLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level %q", level)
	}
}

func ensureDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("workspace dir %q: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("workspace dir %q is not a directory", path)
	}
	return nil
}

func resolveExecutable(path string) (string, error) {
	resolved, err := exec.LookPath(path)
	if err != nil {
		return "", fmt.Errorf("resolve gopls binary %q: %w", path, err)
	}

	info, statErr := os.Stat(resolved)
	if statErr != nil {
		return "", fmt.Errorf("stat gopls binary %q: %w", resolved, statErr)
	}
	if info.IsDir() {
		return "", fmt.Errorf("gopls binary %q is a directory", resolved)
	}
	if info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("gopls binary %q is not executable", resolved)
	}
	return resolved, nil
}

func validateTimeouts(cfg server.Config) error {
	if cfg.RPCTimeout <= 0 {
		return fmt.Errorf("rpc-timeout must be positive, got %s", cfg.RPCTimeout)
	}
	if cfg.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown-timeout must be positive, got %s", cfg.ShutdownTimeout)
	}
	return nil
}
