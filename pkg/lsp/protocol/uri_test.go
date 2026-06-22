package protocol

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPathToURI(t *testing.T) {
	t.Parallel()

	abs := filepath.Join(t.TempDir(), "main.go")
	uri := PathToURI(abs)

	if !strings.HasPrefix(uri, "file://") {
		t.Fatalf("expected file:// prefix, got %q", uri)
	}
	if !strings.HasSuffix(uri, "main.go") {
		t.Fatalf("expected path to end with main.go, got %q", uri)
	}

	if runtime.GOOS == "windows" {
		if !strings.Contains(uri, ":/") {
			t.Fatalf("expected Windows drive letter in URI, got %q", uri)
		}
	}
}

func TestNormalizeFileURI(t *testing.T) {
	t.Parallel()

	existing := "file:///tmp/example.go"
	if got := NormalizeFileURI(existing); got != existing {
		t.Fatalf("expected %q, got %q", existing, got)
	}

	abs := filepath.Join(t.TempDir(), "pkg", "foo.go")
	got := NormalizeFileURI(abs)
	if !strings.HasPrefix(got, "file://") {
		t.Fatalf("expected file:// prefix, got %q", got)
	}
}
