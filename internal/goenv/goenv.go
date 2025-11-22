package goenv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var (
	goRootOnce sync.Once
	goRootPath string
	goRootErr  error
)

// GoRoot returns the system GOROOT by asking `go env GOROOT` once per process.
func GoRoot() (string, error) {
	goRootOnce.Do(func() {
		cmd := exec.Command("go", "env", "GOROOT")
		cmd.Env = os.Environ()
		output, err := cmd.Output()
		if err != nil {
			goRootErr = fmt.Errorf("go env GOROOT: %w", err)
			return
		}
		goRootPath = strings.TrimSpace(string(output))
	})
	return goRootPath, goRootErr
}

// GoBin returns the path to the system Go toolchain's bin directory, if available.
func GoBin() (string, error) {
	root, err := GoRoot()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(root) == "" {
		return "", nil
	}
	return filepath.Join(root, "bin"), nil
}
