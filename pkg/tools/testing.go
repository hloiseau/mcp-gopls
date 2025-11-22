package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func (t *LSPTools) registerTestingTools(s *server.MCPServer) {
	t.registerCoverageAnalysis(s)
	t.registerGoTest(s)
}

func (t *LSPTools) registerCoverageAnalysis(s *server.MCPServer) {
	coverageTool := mcp.NewTool("analyze_coverage",
		mcp.WithDescription("Analyze test coverage for Go code"),
		mcp.WithString("path",
			mcp.Description("Path to the package or directory to analyze. Defaults to ./..."),
		),
		mcp.WithString("output_format",
			mcp.Description("Format of the coverage output: 'summary' (default) or 'func' (per function)"),
		),
	)

	s.AddTool(coverageTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		token := getProgressToken(request.Params.Meta)
		args := request.GetArguments()
		var packagePath, outputFormat string
		if args != nil {
			if path, ok := args["path"].(string); ok {
				packagePath = path
			}
			if format, ok := args["output_format"].(string); ok {
				outputFormat = format
			}
		}

		packagePath = normalizePackageTarget(t.workspaceDir, packagePath)
		if outputFormat == "" {
			outputFormat = "summary"
		}

		payload := map[string]any{
			"target": packagePath,
			"mode":   outputFormat,
		}

		if outputFormat == "func" {
			sendProgressNotification(ctx, s, token, fmt.Sprintf("Running go test with coverage for %s", packagePath))
			result, err := t.runCoverageByFunction(ctx, s, token, packagePath)
			if err != nil {
				failing := result.test
				if result.cover != nil {
					failing = *result.cover
				}
				return t.commandFailureResult("coverage analysis", failing, err)
			}
			payload["test"] = result.test
			if result.cover != nil {
				payload["cover"] = result.cover
			}
			sendProgressNotification(ctx, s, token, fmt.Sprintf("Coverage analysis finished for %s", packagePath))
		} else {
			sendProgressNotification(ctx, s, token, fmt.Sprintf("Running go test -cover for %s", packagePath))
			testResult, err := t.runCommand(ctx, s, token, "go", "test", packagePath, "-cover")
			if err != nil {
				return t.commandFailureResult("go test -cover", testResult, err)
			}
			payload["test"] = testResult
		}

		result, err := mcp.NewToolResultJSON(payload)
		if err != nil {
			return nil, err
		}
		return result, nil
	})
}

type coverageCommandResult struct {
	test  commandResult
	cover *commandResult
}

func (t *LSPTools) runCoverageByFunction(ctx context.Context, srv *server.MCPServer, token mcp.ProgressToken, target string) (coverageCommandResult, error) {
	tempFile, err := os.CreateTemp("", "coverage-*.out")
	if err != nil {
		return coverageCommandResult{}, err
	}
	defer os.Remove(tempFile.Name())
	_ = tempFile.Close()

	testResult, err := t.runCommand(ctx, srv, token, "go", "test", target, "-coverprofile", tempFile.Name())
	if err != nil {
		return coverageCommandResult{test: testResult}, err
	}

	coverResult, coverErr := t.runCommand(ctx, srv, token, "go", "tool", "cover", "-func", tempFile.Name())
	if coverErr != nil {
		return coverageCommandResult{test: testResult, cover: &coverResult}, coverErr
	}

	return coverageCommandResult{
		test:  testResult,
		cover: &coverResult,
	}, nil
}

func (t *LSPTools) registerGoTest(s *server.MCPServer) {
	runTool := mcp.NewTool("run_go_test",
		mcp.WithDescription("Run go test for a package or pattern"),
		mcp.WithString("path",
			mcp.Description("Package path or pattern. Defaults to ./..."),
		),
	)

	s.AddTool(runTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		token := getProgressToken(request.Params.Meta)
		target := "./..."
		if args := request.GetArguments(); args != nil {
			if path, ok := args["path"].(string); ok {
				target = path
			}
		}
		target = normalizePackageTarget(t.workspaceDir, target)

		sendProgressNotification(ctx, s, token, fmt.Sprintf("Running go test for %s", target))
		result, err := t.runCommand(ctx, s, token, "go", "test", target)
		if err != nil {
			return t.commandFailureResult("go test", result, err)
		}

		payload := map[string]any{
			"target": target,
			"result": result,
		}

		toolResult, err := mcp.NewToolResultJSON(payload)
		if err != nil {
			return nil, err
		}
		return toolResult, nil
	})
}

func normalizePackageTarget(workspaceDir, requested string) string {
	target := strings.TrimSpace(requested)
	if target == "" {
		return "./..."
	}
	if workspaceDir == "" {
		return target
	}

	if target == "." || target == "./" {
		hasGoFiles, err := dirHasGoFiles(workspaceDir)
		if err == nil && !hasGoFiles {
			return "./..."
		}
	}

	return target
}

func dirHasGoFiles(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".go") {
			return true, nil
		}
	}
	return false, nil
}
