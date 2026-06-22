package client

import (
	"context"

	"github.com/hloiseau/mcp-gopls/v2/pkg/lsp/protocol"
)

// LSPClient defines the interface for an LSP client.
type LSPClient interface {
	// Core protocol methods
	Initialize(ctx context.Context) error
	Shutdown(ctx context.Context) error
	Close(ctx context.Context) error

	// Code navigation methods
	GoToDefinition(ctx context.Context, uri string, line, character int) ([]protocol.Location, error)
	FindReferences(ctx context.Context, uri string, line, character int, includeDeclaration bool) ([]protocol.Location, error)

	// Diagnostic methods
	GetDiagnostics(ctx context.Context, uri string) ([]protocol.Diagnostic, error)

	// Document methods
	DidOpen(ctx context.Context, uri, languageID, text string) error
	DidClose(ctx context.Context, uri string) error

	// Advanced support
	GetHover(ctx context.Context, uri string, line, character int) (string, error)
	GetCompletion(ctx context.Context, uri string, line, character int) ([]string, error)

	DocumentFormatting(ctx context.Context, uri string) ([]protocol.TextEdit, error)
	Rename(ctx context.Context, uri string, line, character int, newName string) (*protocol.WorkspaceEdit, error)
	CodeActions(ctx context.Context, uri string, rng protocol.Range) ([]protocol.CodeAction, error)
	WorkspaceSymbols(ctx context.Context, query string) ([]protocol.SymbolInformation, error)

	// NotifyDidChangeWatchedFiles signals gopls that files changed on disk,
	// prompting it to invalidate its index for those paths.
	NotifyDidChangeWatchedFiles(ctx context.Context, changes []protocol.FileEvent) error
}
