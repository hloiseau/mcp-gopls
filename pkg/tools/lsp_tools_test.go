package tools

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hloiseaufcms/mcp-gopls/pkg/lsp/protocol"
)

func TestParsePosition(t *testing.T) {
	args := map[string]any{
		"position": map[string]any{
			"line":      float64(5),
			"character": float64(12),
		},
	}
	line, character, err := parsePosition(args)
	if err != nil {
		t.Fatalf("parsePosition returned error: %v", err)
	}
	if line != 5 || character != 12 {
		t.Fatalf("unexpected position (%d,%d)", line, character)
	}
}

func TestParseRangeArg(t *testing.T) {
	args := map[string]any{
		"range": map[string]any{
			"start": map[string]any{"line": float64(1), "character": float64(2)},
			"end":   map[string]any{"line": float64(3), "character": float64(4)},
		},
	}
	rng, err := parseRangeArg(args, "range")
	if err != nil {
		t.Fatalf("parseRangeArg returned error: %v", err)
	}
	expected := protocol.Range{
		Start: protocol.Position{Line: 1, Character: 2},
		End:   protocol.Position{Line: 3, Character: 4},
	}
	if rng != expected {
		t.Fatalf("unexpected range %#v", rng)
	}
}

func TestGetArguments(t *testing.T) {
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{"key": "value"},
		},
	}
	args, err := getArguments(request)
	if err != nil {
		t.Fatalf("getArguments returned error: %v", err)
	}
	if args["key"] != "value" {
		t.Fatalf("unexpected argument value: %#v", args["key"])
	}
}
