package protocol

// Position represents a position in a text document.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range represents a span in a text document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location represents a location in a text document.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// TextDocumentIdentifier identifies a text document.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// TextDocumentPositionParams holds parameters for position-based requests.
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// ReferenceContext provides context for a references request.
type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// ReferenceParams holds parameters for references requests.
type ReferenceParams struct {
	TextDocumentPositionParams
	Context ReferenceContext `json:"context"`
}

// Diagnostic represents a code diagnostic (error, warning, etc.).
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity,omitempty"` // 1=Error, 2=Warning, 3=Info, 4=Hint
	Code     string `json:"code,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

// PublishDiagnosticsParams holds the payload for textDocument/publishDiagnostics.
type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Version     *int         `json:"version,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// DiagnosticSeverity enumerates diagnostic severity levels.
type DiagnosticSeverity int

const (
	SeverityError   DiagnosticSeverity = 1
	SeverityWarning DiagnosticSeverity = 2
	SeverityInfo    DiagnosticSeverity = 3
	SeverityHint    DiagnosticSeverity = 4
)

// TextEdit represents a textual edit.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// FormattingOptions represents LSP formatting options.
type FormattingOptions struct {
	TabSize      int  `json:"tabSize"`
	InsertSpaces bool `json:"insertSpaces"`
}

// DocumentFormattingParams holds parameters for textDocument/formatting.
type DocumentFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Options      FormattingOptions      `json:"options"`
}

// TextDocumentEdit represents a set of edits for a specific document.
type TextDocumentEdit struct {
	TextDocument OptionalVersionedTextDocumentIdentifier `json:"textDocument"`
	Edits        []TextEdit                              `json:"edits"`
}

// OptionalVersionedTextDocumentIdentifier identifies a text document with an optional version.
type OptionalVersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version *int   `json:"version"`
}

// WorkspaceEdit represents a set of workspace modifications.
type WorkspaceEdit struct {
	Changes         map[string][]TextEdit `json:"changes,omitempty"`
	DocumentChanges []TextDocumentEdit    `json:"documentChanges,omitempty"`
}

// RenameParams holds parameters for textDocument/rename.
type RenameParams struct {
	TextDocumentPositionParams
	NewName string `json:"newName"`
}

// CodeActionContext provides context for a code action request.
type CodeActionContext struct {
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
	Only        []string     `json:"only,omitempty"`
}

// CodeActionParams holds parameters for textDocument/codeAction.
type CodeActionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Context      CodeActionContext      `json:"context"`
}

// CodeAction represents an available code action.
type CodeAction struct {
	Title       string         `json:"title"`
	Kind        string         `json:"kind,omitempty"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
	Edit        *WorkspaceEdit `json:"edit,omitempty"`
}

// WorkspaceSymbolParams holds parameters for workspace/symbol.
type WorkspaceSymbolParams struct {
	Query string `json:"query"`
}

// SymbolInformation describes a workspace symbol.
type SymbolInformation struct {
	Name     string   `json:"name"`
	Kind     int      `json:"kind"`
	Location Location `json:"location"`
}

// FileChangeType represents the kind of file change (LSP spec 3.17).
type FileChangeType int

const (
	// FileCreated indicates the file was created.
	FileCreated FileChangeType = 1
	// FileChanged indicates the file was modified.
	FileChanged FileChangeType = 2
	// FileDeleted indicates the file was deleted.
	FileDeleted FileChangeType = 3
)

// FileEvent represents a single file-system change event.
type FileEvent struct {
	// URI of the changed file in file:///... format.
	URI string `json:"uri"`
	// Type is the kind of change (created / changed / deleted).
	Type FileChangeType `json:"type"`
}

// DidChangeWatchedFilesParams holds the parameters for the
// workspace/didChangeWatchedFiles notification.
type DidChangeWatchedFilesParams struct {
	// Changes is the list of file change events.
	Changes []FileEvent `json:"changes"`
}
