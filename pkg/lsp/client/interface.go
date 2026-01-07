package client

import (
	"context"

	"github.com/hloiseau/mcp-gopls/v2/pkg/lsp/protocol"
)

// DiagnosticsHandler is invoked whenever gopls publishes diagnostics.
type DiagnosticsHandler func(protocol.PublishDiagnosticsParams)

// LSPClient définit l'interface pour un client LSP.
type LSPClient interface {
	// Méthodes de base du protocole
	Initialize(ctx context.Context) error
	Shutdown(ctx context.Context) error
	Close(ctx context.Context) error

	// Méthodes de navigation de code
	GoToDefinition(ctx context.Context, uri string, line, character int) ([]protocol.Location, error)
	FindReferences(ctx context.Context, uri string, line, character int, includeDeclaration bool) ([]protocol.Location, error)

	// Méthodes de diagnostic
	GetDiagnostics(ctx context.Context, uri string) ([]protocol.Diagnostic, error)

	// Méthodes de document
	DidOpen(ctx context.Context, uri, languageID, text string) error
	DidClose(ctx context.Context, uri string) error

	// Support avancé
	GetHover(ctx context.Context, uri string, line, character int) (string, error)
	GetCompletion(ctx context.Context, uri string, line, character int) ([]string, error)

	DocumentFormatting(ctx context.Context, uri string) ([]protocol.TextEdit, error)
	Rename(ctx context.Context, uri string, line, character int, newName string) (*protocol.WorkspaceEdit, error)
	CodeActions(ctx context.Context, uri string, rng protocol.Range) ([]protocol.CodeAction, error)
	WorkspaceSymbols(ctx context.Context, query string) ([]protocol.SymbolInformation, error)

	// Observability
	OnDiagnostics(handler DiagnosticsHandler) func()
}
