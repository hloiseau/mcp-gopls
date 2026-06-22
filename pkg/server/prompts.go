package server

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	mcpsrv "github.com/mark3labs/mcp-go/server"
)

type promptDefinition struct {
	prompt  mcp.Prompt
	handler mcpsrv.PromptHandlerFunc
}

func (s *Service) promptDefinitions() []promptDefinition {
	diagPrompt := mcp.NewPrompt("summarize_diagnostics",
		mcp.WithPromptDescription("Summarize Go diagnostics returned by the check_diagnostics tool."),
		mcp.WithArgument("file_uri",
			mcp.ArgumentDescription("Optional file URI the diagnostics refer to"),
		),
		mcp.WithArgument("diagnostics",
			mcp.ArgumentDescription("JSON diagnostics payload from check_diagnostics (paste the tool output here)"),
		),
	)

	refactorPrompt := mcp.NewPrompt("refactor_plan",
		mcp.WithPromptDescription("Create a short refactor plan based on workspace overview and diagnostics."),
		mcp.WithArgument("diagnostics",
			mcp.ArgumentDescription("JSON diagnostics payload"),
			mcp.RequiredArgument(),
		),
	)

	return []promptDefinition{
		{
			prompt: diagPrompt,
			handler: func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				fileURI := request.Params.Arguments["file_uri"]
				diagnostics := request.Params.Arguments["diagnostics"]

				messageText := fmt.Sprintf(`You are reviewing Go diagnostics for a Go workspace.
Workspace root: %s`, s.config.WorkspaceDir)
				if fileURI != "" {
					messageText += fmt.Sprintf("\nFile URI: %s", fileURI)
				}
				if diagnostics != "" {
					messageText += fmt.Sprintf("\n\nDiagnostics JSON:\n%s", diagnostics)
				} else {
					messageText += `

No diagnostics were provided. Run the check_diagnostics tool first, then paste its JSON output into the diagnostics argument before invoking this prompt again.`
				}
				messageText += "\n\nProvide a concise summary highlighting root causes and suggested fixes."

				message := mcp.PromptMessage{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: messageText,
					},
				}
				return &mcp.GetPromptResult{
					Description: diagPrompt.Description,
					Messages:    []mcp.PromptMessage{message},
				}, nil
			},
		},
		{
			prompt: refactorPrompt,
			handler: func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				diag := request.Params.Arguments["diagnostics"]
				messageText := fmt.Sprintf(`Use the provided diagnostics JSON to draft a quick refactor checklist.
Workspace root: %s
Diagnostics:
%v`, s.config.WorkspaceDir, diag)

				message := mcp.PromptMessage{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: messageText,
					},
				}

				return &mcp.GetPromptResult{
					Description: refactorPrompt.Description,
					Messages:    []mcp.PromptMessage{message},
				}, nil
			},
		},
	}
}

func (s *Service) registerPrompts() {
	if s.server == nil {
		return
	}

	for _, def := range s.promptDefinitions() {
		s.server.AddPrompt(def.prompt, def.handler)
	}
}
