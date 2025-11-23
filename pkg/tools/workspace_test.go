package tools

import (
	"errors"
	"os/exec"
	"testing"
)

func TestDetermineGovulncheckCommandBinaryPresent(t *testing.T) {
	originalLookup := lookupGovulncheckBinary
	defer func() { lookupGovulncheckBinary = originalLookup }()

	lookupGovulncheckBinary = func(name string) (string, error) {
		if name != "govulncheck" {
			return "", errors.New("unexpected binary")
		}
		return "/tmp/govulncheck", nil
	}

	cmd, args, fallback := determineGovulncheckCommand()
	if cmd != "/tmp/govulncheck" {
		t.Fatalf("expected /tmp/govulncheck, got %s", cmd)
	}
	if fallback {
		t.Fatalf("expected fallback=false when binary present")
	}
	if len(args) != 1 || args[0] != "./..." {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestDetermineGovulncheckCommandFallback(t *testing.T) {
	originalLookup := lookupGovulncheckBinary
	defer func() { lookupGovulncheckBinary = originalLookup }()

	lookupGovulncheckBinary = func(string) (string, error) {
		return "", &exec.Error{Name: "govulncheck", Err: exec.ErrNotFound}
	}

	cmd, args, fallback := determineGovulncheckCommand()
	if cmd != "go" {
		t.Fatalf("expected go command, got %s", cmd)
	}
	if !fallback {
		t.Fatalf("expected fallback=true when binary missing")
	}
	expected := []string{"run", "golang.org/x/vuln/cmd/govulncheck@latest", "./..."}
	if len(args) != len(expected) {
		t.Fatalf("unexpected args length: %#v", args)
	}
	for i, exp := range expected {
		if args[i] != exp {
			t.Fatalf("arg %d mismatch, expected %s got %s", i, exp, args[i])
		}
	}
}
