package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/hloiseau/mcp-gopls/v2/pkg/lsp/protocol"
)

func (t *LSPTools) registerApplyWorkspaceEdit(s *server.MCPServer) {
	tool := mcp.NewTool("apply_workspace_edit",
		mcp.WithDescription("Apply a WorkspaceEdit to files on disk (e.g. from rename_symbol or format_document)"),
		mcp.WithTitleAnnotation("Apply Workspace Edit"),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithObject("workspace_edit",
			mcp.Required(),
			mcp.Description("WorkspaceEdit JSON as returned by rename_symbol or format_document"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, err := getArguments(request)
		if err != nil {
			return nil, err
		}

		editObj, err := getObjectArg(args, "workspace_edit")
		if err != nil {
			return nil, err
		}

		data, err := json.Marshal(editObj)
		if err != nil {
			return nil, fmt.Errorf("marshal workspace_edit: %w", err)
		}

		var edit protocol.WorkspaceEdit
		if err := json.Unmarshal(data, &edit); err != nil {
			return nil, fmt.Errorf("invalid workspace_edit: %w", err)
		}

		result, applyErr := t.applyWorkspaceEdit(edit)
		payload, err := mcp.NewToolResultJSON(result)
		if err != nil {
			return nil, err
		}
		if applyErr != nil {
			payload.IsError = true
		}
		return payload, nil
	})
}

type applyWorkspaceEditResult struct {
	Modified       []string        `json:"modified"`
	Failed         []fileApplyError `json:"failed,omitempty"`
	PartialFailure bool            `json:"partial_failure,omitempty"`
}

type fileApplyError struct {
	URI   string `json:"uri"`
	Path  string `json:"path,omitempty"`
	Error string `json:"error"`
}

func (t *LSPTools) applyWorkspaceEdit(edit protocol.WorkspaceEdit) (applyWorkspaceEditResult, error) {
	fileEdits := collectFileEdits(edit)
	if len(fileEdits) == 0 {
		return applyWorkspaceEditResult{}, fmt.Errorf("workspace_edit contains no file changes")
	}

	result := applyWorkspaceEditResult{
		Modified: make([]string, 0, len(fileEdits)),
		Failed:   make([]fileApplyError, 0),
	}

	var applyErr error
	for _, fe := range fileEdits {
		path, err := uriToFilesystemPath(fe.uri)
		if err != nil {
			result.Failed = append(result.Failed, fileApplyError{
				URI:   fe.uri,
				Error: err.Error(),
			})
			applyErr = fmt.Errorf("one or more files failed to apply")
			continue
		}

		if err := t.validatePathInWorkspace(path); err != nil {
			result.Failed = append(result.Failed, fileApplyError{
				URI:   fe.uri,
				Path:  path,
				Error: err.Error(),
			})
			applyErr = fmt.Errorf("one or more files failed to apply")
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			result.Failed = append(result.Failed, fileApplyError{
				URI:   fe.uri,
				Path:  path,
				Error: fmt.Sprintf("read file: %v", err),
			})
			applyErr = fmt.Errorf("one or more files failed to apply")
			continue
		}

		updated, err := applyTextEdits(string(content), fe.edits)
		if err != nil {
			result.Failed = append(result.Failed, fileApplyError{
				URI:   fe.uri,
				Path:  path,
				Error: fmt.Sprintf("apply edits: %v", err),
			})
			applyErr = fmt.Errorf("one or more files failed to apply")
			continue
		}

		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			result.Failed = append(result.Failed, fileApplyError{
				URI:   fe.uri,
				Path:  path,
				Error: fmt.Sprintf("write file: %v", err),
			})
			applyErr = fmt.Errorf("one or more files failed to apply")
			continue
		}

		result.Modified = append(result.Modified, path)
	}

	if len(result.Failed) > 0 && len(result.Modified) > 0 {
		result.PartialFailure = true
	}

	return result, applyErr
}

type fileEditEntry struct {
	uri   string
	edits []protocol.TextEdit
}

func collectFileEdits(edit protocol.WorkspaceEdit) []fileEditEntry {
	if len(edit.DocumentChanges) > 0 {
		entries := make([]fileEditEntry, 0, len(edit.DocumentChanges))
		for _, dc := range edit.DocumentChanges {
			if dc.TextDocument.URI == "" {
				continue
			}
			entries = append(entries, fileEditEntry{
				uri:   dc.TextDocument.URI,
				edits: dc.Edits,
			})
		}
		return entries
	}

	if len(edit.Changes) > 0 {
		entries := make([]fileEditEntry, 0, len(edit.Changes))
		for uri, edits := range edit.Changes {
			entries = append(entries, fileEditEntry{uri: uri, edits: edits})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].uri < entries[j].uri
		})
		return entries
	}

	return nil
}

func (t *LSPTools) validatePathInWorkspace(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	workspaceDir, err := filepath.Abs(t.workspaceDir)
	if err != nil {
		return fmt.Errorf("resolve workspace dir: %w", err)
	}

	rel, err := filepath.Rel(workspaceDir, absPath)
	if err != nil {
		return fmt.Errorf("path outside workspace: %s", path)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path outside workspace: %s", path)
	}
	return nil
}

func uriToFilesystemPath(uri string) (string, error) {
	if !strings.HasPrefix(uri, "file://") {
		return "", fmt.Errorf("unsupported uri: %s", uri)
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	path := parsed.Path
	if runtime.GOOS == "windows" {
		path = strings.TrimPrefix(path, "/")
		path = strings.ReplaceAll(path, "/", "\\")
	}
	return path, nil
}

type editWithOffsets struct {
	start   int
	end     int
	newText string
}

func applyTextEdits(content string, edits []protocol.TextEdit) (string, error) {
	if len(edits) == 0 {
		return content, nil
	}

	parsed := make([]editWithOffsets, 0, len(edits))
	for _, edit := range edits {
		start, err := positionToOffset(content, edit.Range.Start)
		if err != nil {
			return "", fmt.Errorf("start position: %w", err)
		}
		end, err := positionToOffset(content, edit.Range.End)
		if err != nil {
			return "", fmt.Errorf("end position: %w", err)
		}
		if start > end {
			return "", fmt.Errorf("invalid range: start offset %d > end offset %d", start, end)
		}
		parsed = append(parsed, editWithOffsets{
			start:   start,
			end:     end,
			newText: edit.NewText,
		})
	}

	sort.Slice(parsed, func(i, j int) bool {
		if parsed[i].start != parsed[j].start {
			return parsed[i].start > parsed[j].start
		}
		return parsed[i].end > parsed[j].end
	})

	result := content
	for _, edit := range parsed {
		result = result[:edit.start] + edit.newText + result[edit.end:]
	}
	return result, nil
}

func positionToOffset(content string, pos protocol.Position) (int, error) {
	if pos.Line < 0 || pos.Character < 0 {
		return 0, fmt.Errorf("negative position (%d, %d)", pos.Line, pos.Character)
	}

	lineStart := 0
	currentLine := 0
	for lineStart < len(content) && currentLine < pos.Line {
		next := strings.IndexByte(content[lineStart:], '\n')
		if next < 0 {
			return 0, fmt.Errorf("line %d out of range", pos.Line)
		}
		lineStart += next + 1
		currentLine++
	}
	if currentLine != pos.Line {
		return 0, fmt.Errorf("line %d out of range", pos.Line)
	}

	lineEnd := len(content)
	if idx := strings.IndexByte(content[lineStart:], '\n'); idx >= 0 {
		lineEnd = lineStart + idx
	}

	charOffset, err := utf16OffsetToByteOffset(content[lineStart:lineEnd], pos.Character)
	if err != nil {
		return 0, err
	}
	return lineStart + charOffset, nil
}

func utf16OffsetToByteOffset(line string, utf16Offset int) (int, error) {
	byteOffset := 0
	utf16Count := 0
	for len(line) > 0 {
		if utf16Count == utf16Offset {
			return byteOffset, nil
		}
		r, size := utf8.DecodeRuneInString(line)
		units := utf16CodeUnits(r)
		if utf16Count+units > utf16Offset {
			return 0, fmt.Errorf("character offset %d falls within a code point", utf16Offset)
		}
		utf16Count += units
		byteOffset += size
		line = line[size:]
	}
	if utf16Count != utf16Offset {
		return 0, fmt.Errorf("character offset %d out of range (line has %d UTF-16 units)", utf16Offset, utf16Count)
	}
	return byteOffset, nil
}

func utf16CodeUnits(r rune) int {
	if r >= 0x10000 {
		return 2
	}
	return 1
}
