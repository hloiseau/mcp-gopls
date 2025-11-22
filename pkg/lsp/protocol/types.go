package protocol

// Position représente une position dans un document texte
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range représente une plage dans un document texte
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location représente un emplacement dans un document texte
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// TextDocumentIdentifier identifie un document texte
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// TextDocumentPositionParams paramètres pour les requêtes basées sur la position
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// ReferenceContext contexte pour une requête de références
type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// ReferenceParams paramètres pour les requêtes de références
type ReferenceParams struct {
	TextDocumentPositionParams
	Context ReferenceContext `json:"context"`
}

// Diagnostic représente un diagnostic comme un problème de code
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity,omitempty"` // 1=Error, 2=Warning, 3=Info, 4=Hint
	Code     string `json:"code,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

// PublishDiagnosticsParams représente la charge utile pour textDocument/publishDiagnostics.
type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Version     *int         `json:"version,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// DiagnosticSeverity énumère les niveaux de sévérité des diagnostics
type DiagnosticSeverity int

const (
	SeverityError   DiagnosticSeverity = 1
	SeverityWarning DiagnosticSeverity = 2
	SeverityInfo    DiagnosticSeverity = 3
	SeverityHint    DiagnosticSeverity = 4
)

// TextEdit représente une modification textuelle.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// FormattingOptions représente les options de formatage LSP.
type FormattingOptions struct {
	TabSize      int  `json:"tabSize"`
	InsertSpaces bool `json:"insertSpaces"`
}

// DocumentFormattingParams paramètres pour textDocument/formatting.
type DocumentFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Options      FormattingOptions      `json:"options"`
}

// WorkspaceEdit représente un ensemble de modifications.
type WorkspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes,omitempty"`
}

// RenameParams paramètres pour textDocument/rename.
type RenameParams struct {
	TextDocumentPositionParams
	NewName string `json:"newName"`
}

// CodeActionContext fournit le contexte d'une action de code.
type CodeActionContext struct {
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
	Only        []string     `json:"only,omitempty"`
}

// CodeActionParams paramètres pour textDocument/codeAction.
type CodeActionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Context      CodeActionContext      `json:"context"`
}

// CodeAction représente une action de code disponible.
type CodeAction struct {
	Title       string         `json:"title"`
	Kind        string         `json:"kind,omitempty"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
	Edit        *WorkspaceEdit `json:"edit,omitempty"`
}

// WorkspaceSymbolParams paramètres pour workspace/symbol.
type WorkspaceSymbolParams struct {
	Query string `json:"query"`
}

// SymbolInformation décrit un symbole trouvé.
type SymbolInformation struct {
	Name     string   `json:"name"`
	Kind     int      `json:"kind"`
	Location Location `json:"location"`
}
