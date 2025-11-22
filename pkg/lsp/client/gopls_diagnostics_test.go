package client

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/hloiseaufcms/mcp-gopls/pkg/lsp/protocol"
)

func newTestClient() *GoplsClient {
	return &GoplsClient{
		logger:              slog.New(slog.NewTextHandler(io.Discard, nil)),
		diagnosticsCache:    make(map[string][]protocol.Diagnostic),
		diagnosticsHandlers: make(map[int64]DiagnosticsHandler),
		diagnosticsWaiters:  make(map[string][]chan struct{}),
	}
}

func TestWaitForDiagnosticsReturnsAfterPublish(t *testing.T) {
	client := newTestClient()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		if err := client.waitForDiagnostics(ctx, "file://test.go"); err != nil {
			t.Errorf("waitForDiagnostics returned error: %v", err)
		}
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	client.updateDiagnostics(protocol.PublishDiagnosticsParams{
		URI: "file://test.go",
		Diagnostics: []protocol.Diagnostic{
			{Message: "failure"},
		},
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("waitForDiagnostics did not unblock after update")
	}
}

func TestWaitForDiagnosticsImmediateAndTimeout(t *testing.T) {
	client := newTestClient()
	client.diagnosticsCache["file://ready.go"] = nil

	if err := client.waitForDiagnostics(context.Background(), "file://ready.go"); err != nil {
		t.Fatalf("waitForDiagnostics should return immediately: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := client.waitForDiagnostics(ctx, "file://never.go")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}
