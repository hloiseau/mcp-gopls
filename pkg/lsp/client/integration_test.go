//go:build integration

package client

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func requireGopls(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("gopls"); err != nil {
		t.Skip("gopls not found on PATH")
	}
}

func setupTestModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGoCmd(t, dir, "go", "mod", "init", "example.com/integrationtest")
	return dir
}

func writeGoFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGoCmd(t, dir, "go", "mod", "tidy")
	return pathToURI(path)
}

func runGoCmd(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v in %s: %v\n%s", name, args, dir, err, out)
	}
}

func newIntegrationClient(t *testing.T, workspaceDir string) (*GoplsClient, context.Context) {
	t.Helper()
	requireGopls(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	client, err := newBareIntegrationClient(t, workspaceDir)
	if err != nil {
		t.Fatalf("NewGoplsClient: %v", err)
	}
	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		_ = client.Shutdown(shutdownCtx)
		_ = client.Close(shutdownCtx)
	})

	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	return client, ctx
}

func TestIntegration_Initialize(t *testing.T) {
	t.Parallel()
	requireGopls(t)

	dir := setupTestModule(t)
	writeGoFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := newBareIntegrationClient(t, dir)
	if err != nil {
		t.Fatalf("NewGoplsClient: %v", err)
	}

	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := client.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	closeCtx, closeCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer closeCancel()
	if err := client.Close(closeCtx); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func newBareIntegrationClient(t *testing.T, workspaceDir string) (*GoplsClient, error) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewGoplsClient(
		WithWorkspaceDir(workspaceDir),
		WithLogger(logger),
		WithCallTimeout(90*time.Second),
	)
}

func TestIntegration_GoToDefinition(t *testing.T) {
	t.Parallel()

	dir := setupTestModule(t)
	uri := writeGoFile(t, dir, "main.go", `package main

func Foo() {}

func main() {
	Foo()
}
`)

	client, ctx := newIntegrationClient(t, dir)
	if err := client.DidOpen(ctx, uri, "go", ""); err != nil {
		t.Fatalf("DidOpen: %v", err)
	}

	// Call site: line 5 ("	Foo()"), character at 'F'.
	locations, err := client.GoToDefinition(ctx, uri, 5, 1)
	if err != nil {
		t.Fatalf("GoToDefinition: %v", err)
	}
	if len(locations) == 0 {
		t.Fatal("expected at least one definition location")
	}

	found := false
	for _, loc := range locations {
		if loc.URI != uri {
			continue
		}
		if loc.Range.Start.Line == 2 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("definition not at expected line 2: %#v", locations)
	}
}

func TestIntegration_Diagnostics(t *testing.T) {
	t.Parallel()

	dir := setupTestModule(t)
	uri := writeGoFile(t, dir, "main.go", `package main

func main() {
	x := 1
}
`)

	client, ctx := newIntegrationClient(t, dir)

	diagnostics, err := client.GetDiagnostics(ctx, uri)
	if err != nil {
		t.Fatalf("GetDiagnostics: %v", err)
	}
	if len(diagnostics) == 0 {
		t.Fatal("expected at least one diagnostic for unused variable")
	}
}

func TestIntegration_Hover(t *testing.T) {
	t.Parallel()

	dir := setupTestModule(t)
	uri := writeGoFile(t, dir, "main.go", `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`)

	client, ctx := newIntegrationClient(t, dir)

	// Line 5: "	fmt.Println(...)", cursor on 'P' in Println.
	hover, err := client.GetHover(ctx, uri, 5, 4)
	if err != nil {
		t.Fatalf("GetHover: %v", err)
	}
	if strings.TrimSpace(hover) == "" {
		t.Fatal("expected non-empty hover result")
	}
}

func TestIntegration_Completion(t *testing.T) {
	t.Parallel()

	dir := setupTestModule(t)
	uri := writeGoFile(t, dir, "main.go", `package main

import "fmt"

func main() {
	fmt.
}
`)

	client, ctx := newIntegrationClient(t, dir)

	// Line 5: "	fmt.", cursor after the dot.
	completions, err := client.GetCompletion(ctx, uri, 5, 5)
	if err != nil {
		t.Fatalf("GetCompletion: %v", err)
	}
	if len(completions) == 0 {
		t.Fatal("expected completion results")
	}

	found := false
	for _, label := range completions {
		if label == "Println" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected Println in completions, got: %v", completions)
	}
}

func TestIntegration_DocumentFormatting(t *testing.T) {
	t.Parallel()

	dir := setupTestModule(t)
	uri := writeGoFile(t, dir, "main.go", "package main\nfunc main(){fmt.Println(\"hi\")}\n")

	client, ctx := newIntegrationClient(t, dir)
	if err := client.DidOpen(ctx, uri, "go", ""); err != nil {
		t.Fatalf("DidOpen: %v", err)
	}

	edits, err := client.DocumentFormatting(ctx, uri)
	if err != nil {
		t.Fatalf("DocumentFormatting: %v", err)
	}
	if len(edits) == 0 {
		t.Fatal("expected formatting edits for unformatted file")
	}
}
