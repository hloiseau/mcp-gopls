package client

import (
	"os"
	"strings"
	"testing"

	"github.com/hloiseaufcms/mcp-gopls/internal/goenv"
)

func TestBuildGoplsEnv_AppendsWhenMissing(t *testing.T) {
	base := []string{"PATH=/usr/bin"}
	result := buildGoplsEnv(base)

	if val := getEnvValue(result, "GOTOOLCHAIN"); val != "local" {
		t.Fatalf("expected GOTOOLCHAIN=local, got %q", val)
	}

	assertPathPrefixed(t, result, "/usr/bin")
}

func TestBuildGoplsEnv_KeepsExistingSetting(t *testing.T) {
	base := []string{"PATH=/usr/bin", "GOTOOLCHAIN=auto"}
	result := buildGoplsEnv(base)

	if val := getEnvValue(result, "GOTOOLCHAIN"); val != "auto" {
		t.Fatalf("expected preserved GOTOOLCHAIN value, got %q", val)
	}

	assertPathPrefixed(t, result, "/usr/bin")
}

func TestBuildGoplsEnv_AddsPathWhenMissing(t *testing.T) {
	base := []string{"FOO=bar"}
	t.Setenv("PATH", "/usr/bin")
	result := buildGoplsEnv(base)

	if _, ok := findEnv(result, "PATH"); !ok {
		t.Fatal("expected PATH entry to be added")
	}

	assertPathPrefixed(t, result, "")
}

func assertPathPrefixed(t *testing.T, env []string, mustContain string) {
	t.Helper()
	goBin, err := goenv.GoBin()
	if err != nil {
		t.Fatalf("go env GOROOT failed: %v", err)
	}
	if goBin == "" {
		t.Skip("GOROOT not available in current environment")
	}
	pathVal := getEnvValue(env, "PATH")
	if pathVal == "" {
		t.Fatal("PATH not found")
	}
	if !strings.HasPrefix(pathVal, goBin) {
		t.Fatalf("expected PATH to start with %q, got %q", goBin, pathVal)
	}
	if mustContain != "" && !strings.Contains(pathVal, mustContain) {
		t.Fatalf("expected PATH to retain %q, got %q", mustContain, pathVal)
	}
}

func getEnvValue(env []string, key string) string {
	if val, ok := findEnv(env, key); ok {
		return val
	}
	return os.Getenv(key)
}

func findEnv(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return strings.TrimPrefix(kv, prefix), true
		}
	}
	return "", false
}
