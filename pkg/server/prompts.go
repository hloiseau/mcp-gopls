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
				message := mcp.PromptMessage{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: "You are reviewing Go diagnostics. Provide a concise summary highlighting root causes and suggested fixes.",
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
