package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
	mcpsrv "github.com/mark3labs/mcp-go/server"
)

type resourceDefinition struct {
	resource mcp.Resource
	handler  mcpsrv.ResourceHandlerFunc
}

func (s *Service) resourceDefinitions() []resourceDefinition {
	return []resourceDefinition{
		{
			resource: mcp.Resource{
				URI:         "resource://workspace/overview",
				Name:        "Workspace Overview",
				Description: "High-level summary of top-level directories and Go files.",
				MIMEType:    "application/json",
			},
			handler: s.handleWorkspaceOverview,
		},
		{
			resource: mcp.Resource{
				URI:         "resource://workspace/go.mod",
				Name:        "go.mod",
				Description: "Contents of the workspace go.mod file.",
				MIMEType:    "text/plain",
			},
			handler: s.handleGoModFile,
		},
	}
}

func (s *Service) registerResources() {
	if s.server == nil {
		return
	}

	for _, def := range s.resourceDefinitions() {
		s.server.AddResource(def.resource, def.handler)
	}
}

func (s *Service) handleWorkspaceOverview(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	summary, err := buildWorkspaceSummary(s.config.WorkspaceDir)
	if err != nil {
		return nil, err
	}

	content := mcp.TextResourceContents{
		URI:      request.Params.URI,
		MIMEType: "application/json",
		Text:     summary,
	}
	return []mcp.ResourceContents{content}, nil
}

func (s *Service) handleGoModFile(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	goModPath := filepath.Join(s.config.WorkspaceDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, fmt.Errorf("read go.mod: %w", err)
	}

	content := mcp.TextResourceContents{
		URI:      request.Params.URI,
		MIMEType: "text/plain",
		Text:     string(data),
	}
	return []mcp.ResourceContents{content}, nil
}

func buildWorkspaceSummary(root string) (string, error) {
	type summary struct {
		Root        string   `json:"root"`
		Directories []string `json:"directories"`
		GoFiles     []string `json:"go_files"`
	}

	dirEntries, err := os.ReadDir(root)
	if err != nil {
		return "", fmt.Errorf("read workspace: %w", err)
	}

	result := summary{
		Root: root,
	}

	for _, entry := range dirEntries {
		name := entry.Name()
		if entry.IsDir() {
			result.Directories = append(result.Directories, name)
		} else if filepath.Ext(name) == ".go" {
			result.GoFiles = append(result.GoFiles, name)
		}
		if len(result.Directories) >= 10 && len(result.GoFiles) >= 10 {
			break
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
