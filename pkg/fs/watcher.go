// Package fs implements filesystem watching for the gopls workspace.
// When .go, go.mod or go.sum files change on disk, the watcher sends a
// workspace/didChangeWatchedFiles notification to gopls so it can re-index
// without requiring a process restart.
package fs

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/hloiseau/mcp-gopls/v2/pkg/lsp/protocol"
)

// debounceDuration is the quiet period after the last FS event before the
// accumulated batch is flushed to gopls. Collapses bursts (e.g. go generate).
const debounceDuration = 150 * time.Millisecond

// Notifier sends workspace/didChangeWatchedFiles notifications to the LSP server.
// Implemented by *client.GoplsClient; extracted as a minimal interface for testability.
type Notifier interface {
	NotifyDidChangeWatchedFiles(ctx context.Context, changes []protocol.FileEvent) error
}

// Watcher watches .go, go.mod and go.sum files in the workspace and notifies
// gopls via workspace/didChangeWatchedFiles whenever they change on disk.
type Watcher struct {
	workspaceDir string
	notifier     Notifier
	logger       *slog.Logger
}

// NewWatcher creates a Watcher for the given workspace directory.
// notifier is called with batched file-change events after a short debounce.
func NewWatcher(workspaceDir string, notifier Notifier) *Watcher {
	return &Watcher{
		workspaceDir: workspaceDir,
		notifier:     notifier,
		logger:       slog.Default().With("component", "fs_watcher"),
	}
}

// WithLogger replaces the default logger.
func (w *Watcher) WithLogger(logger *slog.Logger) *Watcher {
	w.logger = logger
	return w
}

// Run starts filesystem watching and blocks until ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		w.logger.Error("failed to create fs watcher", "error", err)
		return
	}
	defer func() { _ = fsWatcher.Close() }()

	if err := w.addDirs(fsWatcher, w.workspaceDir); err != nil {
		w.logger.Error("failed to watch workspace", "dir", w.workspaceDir, "error", err)
		return
	}

	w.logger.Info("fs watcher started", "workspace", w.workspaceDir)
	w.eventLoop(ctx, fsWatcher)
	w.logger.Info("fs watcher stopped")
}

// addDirs recursively registers directories under fsnotify, skipping vendor and hidden dirs.
func (w *Watcher) addDirs(fsWatcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip directories we cannot access
		}
		if !d.IsDir() {
			return nil
		}
		// Skip hidden directories, vendor and testdata.
		name := d.Name()
		if strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata" {
			return filepath.SkipDir
		}
		if watchErr := fsWatcher.Add(path); watchErr != nil {
			w.logger.Warn("cannot watch dir", "path", path, "error", watchErr)
		}
		return nil
	})
}

// eventLoop processes fsnotify events with debouncing.
func (w *Watcher) eventLoop(ctx context.Context, fsWatcher *fsnotify.Watcher) {
	// pending accumulates change events until the debounce timer fires.
	pending := make(map[string]protocol.FileChangeType)
	var mu sync.Mutex
	var timer *time.Timer

	// flush drains pending events and forwards them to gopls.
	flush := func() {
		mu.Lock()
		if len(pending) == 0 {
			mu.Unlock()
			return
		}
		changes := make([]protocol.FileEvent, 0, len(pending))
		for uri, changeType := range pending {
			changes = append(changes, protocol.FileEvent{URI: uri, Type: changeType})
		}
		pending = make(map[string]protocol.FileChangeType)
		mu.Unlock()

		if err := w.notifier.NotifyDidChangeWatchedFiles(ctx, changes); err != nil {
			w.logger.Warn("failed to notify gopls about file changes", "error", err)
		} else {
			w.logger.Debug("notified gopls about file changes", "count", len(changes))
		}
	}

	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return

		case event, ok := <-fsWatcher.Events:
			if !ok {
				return
			}

			if !isGoRelatedFile(event.Name) {
				continue
			}

			changeType := toFileChangeType(event.Op)
			if changeType == 0 {
				continue
			}

			uri := protocol.PathToURI(event.Name)

			mu.Lock()
			// Preserve Created over Changed: if a file was just created, do not
			// downgrade the event type to Changed on the subsequent write flush.
			if existing, exists := pending[uri]; exists {
				if existing == protocol.FileCreated && changeType == protocol.FileChanged {
					mu.Unlock()
					continue
				}
			}
			pending[uri] = changeType
			mu.Unlock()

			// Reset the debounce timer on every new event.
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(debounceDuration, flush)

		case err, ok := <-fsWatcher.Errors:
			if !ok {
				return
			}
			w.logger.Warn("fs watcher error", "error", err)
		}
	}
}

// isGoRelatedFile reports whether a path should trigger a gopls notification.
func isGoRelatedFile(name string) bool {
	base := filepath.Base(name)
	if base == "go.mod" || base == "go.sum" {
		return true
	}
	return strings.HasSuffix(name, ".go")
}

// toFileChangeType maps a fsnotify.Op to its LSP FileChangeType equivalent.
// Returns 0 for operations that should not be forwarded to gopls.
func toFileChangeType(op fsnotify.Op) protocol.FileChangeType {
	switch {
	case op.Has(fsnotify.Create):
		return protocol.FileCreated
	case op.Has(fsnotify.Write):
		return protocol.FileChanged
	case op.Has(fsnotify.Remove), op.Has(fsnotify.Rename):
		return protocol.FileDeleted
	default:
		return 0
	}
}
