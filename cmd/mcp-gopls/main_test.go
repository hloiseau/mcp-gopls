package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/hloiseau/mcp-gopls/v2/pkg/server"
)

type stubService struct {
	startCalled chan struct{}
	closeOnce   sync.Once

	startCtx    context.Context
	startErr    error
	closeCalled bool
}

func newStubService() *stubService {
	return &stubService{
		startCalled: make(chan struct{}),
	}
}

func (s *stubService) Start(ctx context.Context) error {
	s.startCtx = ctx
	close(s.startCalled)
	if s.startErr != nil {
		return s.startErr
	}
	<-ctx.Done()
	return nil
}

func (s *stubService) Close(ctx context.Context) {
	s.closeOnce.Do(func() {
		s.closeCalled = true
	})
}

func TestRunHandlesSignalShutdown(t *testing.T) {
	origService := newServiceFn
	origNotify := notifyContextF
	t.Cleanup(func() {
		newServiceFn = origService
		notifyContextF = origNotify
	})

	withFreshFlags(t, nil, func() {
		service := newStubService()
		newServiceFn = func(cfg server.Config) (serviceRunner, error) {
			return service, nil
		}

		var cancel context.CancelFunc
		notifyContextF = func(parent context.Context, sig ...os.Signal) (context.Context, context.CancelFunc) {
			ctx, c := context.WithCancel(parent)
			cancel = c
			return ctx, c
		}

		// Capture stdout to assert shutdown message.
		origStdout := os.Stdout
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("pipe: %v", err)
		}
		os.Stdout = w
		defer func() {
			os.Stdout = origStdout
		}()

		errCh := make(chan error, 1)
		go func() {
			errCh <- run()
		}()

		select {
		case <-service.startCalled:
		case <-time.After(time.Second):
			t.Fatal("service start not invoked")
		}

		cancel()

		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("run returned error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("run did not complete after cancel")
		}

		_ = w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		if !bytes.Contains(buf.Bytes(), []byte("mcp-gopls shutdown complete")) {
			t.Fatalf("expected shutdown message, got %q", buf.String())
		}

		if !service.closeCalled {
			t.Fatal("expected service.Close to be called")
		}
		if service.startCtx.Err() == nil {
			t.Fatal("expected start context to be cancelled")
		}
	})
}

func TestRunPropagatesStartError(t *testing.T) {
	origService := newServiceFn
	origNotify := notifyContextF
	t.Cleanup(func() {
		newServiceFn = origService
		notifyContextF = origNotify
	})

	withFreshFlags(t, nil, func() {
		service := newStubService()
		service.startErr = errors.New("boom")
		newServiceFn = func(cfg server.Config) (serviceRunner, error) {
			return service, nil
		}
		notifyContextF = func(parent context.Context, sig ...os.Signal) (context.Context, context.CancelFunc) {
			return context.WithCancel(parent)
		}

		if err := run(); err == nil || err.Error() != "service error: boom" {
			t.Fatalf("expected service error, got %v", err)
		}
	})
}

func TestRunReturnsFactoryError(t *testing.T) {
	origService := newServiceFn
	t.Cleanup(func() { newServiceFn = origService })

	withFreshFlags(t, nil, func() {
		newServiceFn = func(server.Config) (serviceRunner, error) {
			return nil, errors.New("factory failure")
		}

		if err := run(); err == nil || err.Error() != "create service: factory failure" {
			t.Fatalf("expected factory failure error, got %v", err)
		}
	})
}

func TestBuildConfigFromFlagsAndEnv(t *testing.T) {
	tmp := t.TempDir()
	setEnv(t, "MCP_GOPLS_WORKSPACE", tmp)
	setEnv(t, "MCP_GOPLS_LOG_LEVEL", "debug")
	setEnv(t, "MCP_GOPLS_RPC_TIMEOUT", "2s")
	setEnv(t, "MCP_GOPLS_SHUTDOWN_TIMEOUT", "3s")
	withFreshFlags(t, []string{"-log-json", "-log-file", "app.log"}, func() {
		cfg, err := buildConfigFromFlags()
		if err != nil {
			t.Fatalf("buildConfigFromFlags returned error: %v", err)
		}
		if cfg.WorkspaceDir != tmp {
			t.Fatalf("expected workspace %s, got %s", tmp, cfg.WorkspaceDir)
		}
		if !cfg.LogJSON {
			t.Fatal("expected log JSON enabled")
		}
		if cfg.LogFile != "app.log" {
			t.Fatalf("unexpected log file %s", cfg.LogFile)
		}
		if cfg.RPCTimeout != 2*time.Second {
			t.Fatalf("unexpected rpc timeout %s", cfg.RPCTimeout)
		}
		if cfg.ShutdownTimeout != 3*time.Second {
			t.Fatalf("unexpected shutdown timeout %s", cfg.ShutdownTimeout)
		}
	})
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    slog.Level
		wantErr bool
	}{
		{"info", "info", slog.LevelInfo, false},
		{"warn-alias", "warning", slog.LevelWarn, false},
		{"invalid", "verbose", slog.LevelInfo, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLogLevel(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestEnvHelpers(t *testing.T) {
	setEnv(t, "BOOL_TRUE", "yes")
	setEnv(t, "DURATION", "150ms")
	if !envBool("BOOL_TRUE") {
		t.Fatal("expected envBool true")
	}
	if envBool("MISSING_BOOL") {
		t.Fatal("expected envBool false")
	}
	if got := envDuration("DURATION", time.Second); got != 150*time.Millisecond {
		t.Fatalf("expected 150ms, got %s", got)
	}
	if got := envOrDefault("UNSET", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %s", got)
	}
}

func withFreshFlags(t *testing.T, args []string, fn func()) {
	t.Helper()
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = append([]string{"mcp-gopls"}, args...)
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	fn()
}

func setEnv(t *testing.T, key, value string) {
	t.Helper()
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("setenv %s: %v", key, err)
	}
	t.Cleanup(func() {
		_ = os.Unsetenv(key)
	})
}
