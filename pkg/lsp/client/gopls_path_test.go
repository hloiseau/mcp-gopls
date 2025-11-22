package client

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveGoplsExecutableExplicitPath(t *testing.T) {
	dir := t.TempDir()
	fake := writeFakeGopls(t, dir)

	path, err := resolveGoplsExecutable(fake)
	if err != nil {
		t.Fatalf("resolveGoplsExecutable returned error: %v", err)
	}
	if path != fake {
		t.Fatalf("expected %q, got %q", fake, path)
	}
}

func TestResolveGoplsExecutablePathLookup(t *testing.T) {
	dir := t.TempDir()
	fake := writeFakeGopls(t, dir)

	t.Setenv("PATH", dir)
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")

	path, err := resolveGoplsExecutable("")
	if err != nil {
		t.Fatalf("resolveGoplsExecutable returned error: %v", err)
	}
	if path != fake {
		t.Fatalf("expected %q, got %q", fake, path)
	}
}

func TestResolveGoplsExecutableGoBinFallback(t *testing.T) {
	dir := t.TempDir()
	fake := writeFakeGopls(t, dir)

	t.Setenv("PATH", "")
	t.Setenv("GOBIN", dir)
	t.Setenv("GOPATH", "")

	path, err := resolveGoplsExecutable("")
	if err != nil {
		t.Fatalf("resolveGoplsExecutable returned error: %v", err)
	}
	if path != fake {
		t.Fatalf("expected %q, got %q", fake, path)
	}
}

func TestResolveGoplsExecutableDefaultHomeFallback(t *testing.T) {
	dir := t.TempDir()
	goBin := filepath.Join(dir, "go", "bin")
	fake := writeFakeGopls(t, goBin)

	t.Setenv("PATH", "")
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")
	t.Setenv("HOME", dir)

	path, err := resolveGoplsExecutable("")
	if err != nil {
		t.Fatalf("resolveGoplsExecutable returned error: %v", err)
	}
	if path != fake {
		t.Fatalf("expected %q, got %q", fake, path)
	}
}

func TestResolveGoplsExecutableNotFound(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")
	t.Setenv("HOME", t.TempDir())

	if _, err := resolveGoplsExecutable(""); err == nil {
		t.Fatal("expected error when gopls is missing")
	}
}

func writeFakeGopls(t *testing.T, dir string) string {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}

	name := goplsBinaryName()
	path := filepath.Join(dir, name)
	content := []byte("#!/bin/sh\n")
	if runtime.GOOS == "windows" {
		content = []byte("@echo off\r\n")
	}

	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatalf("write fake gopls: %v", err)
	}

	return path
}
