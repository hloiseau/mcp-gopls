package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/hloiseau/mcp-gopls/v2/pkg/lsp/protocol"
)

func getProgressToken(meta *mcp.Meta) mcp.ProgressToken {
	if meta == nil {
		return nil
	}
	return meta.ProgressToken
}

func sendProgressNotification(ctx context.Context, srv *server.MCPServer, token mcp.ProgressToken, message string) {
	if srv == nil || token == nil {
		return
	}

	if ctx == nil {
		ctx = context.Background()
	}

	payload, err := protocol.NewProgressNotification(token, 0, message)
	if err != nil {
		return
	}
	params := map[string]any{
		"progressToken": payload.ProgressToken,
		"progress":      payload.Progress,
	}
	if payload.Message != "" {
		params["message"] = payload.Message
	}
	_ = srv.SendNotificationToClient(ctx, protocol.ProgressMethod, params)
}
