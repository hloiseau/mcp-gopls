package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizePackageTargetDefaultsToAllPackages(t *testing.T) {
	workspace := t.TempDir()
	got := normalizePackageTarget(workspace, "")
	if got != "./..." {
		t.Fatalf("expected ./..., got %s", got)
	}
}

func TestNormalizePackageTargetFallsBackWhenRootHasNoGoFiles(t *testing.T) {
	workspace := t.TempDir()
	pkgDir := filepath.Join(workspace, "pkg")
	if err := os.Mkdir(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir pkg: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "main.go"), []byte("package pkg\n"), 0o644); err != nil {
		t.Fatalf("write go file: %v", err)
	}

	got := normalizePackageTarget(workspace, ".")
	if got != "./..." {
		t.Fatalf("expected ./... fallback, got %s", got)
	}
}

func TestNormalizePackageTargetKeepsDotWhenRootHasGoFiles(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write go file: %v", err)
	}

	got := normalizePackageTarget(workspace, ".")
	if got != "." {
		t.Fatalf("expected '.', got %s", got)
	}
}

func TestNormalizePackageTargetLeavesDotOnStatError(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "missing")
	got := normalizePackageTarget(workspace, ".")
	if got != "." {
		t.Fatalf("expected '.' on stat error, got %s", got)
	}
}
