package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ProgressMethod is the JSON-RPC method used for streaming MCP tool progress.
const ProgressMethod = "notifications/progress"

// ProgressNotification represents the payload for MCP progress updates.
type ProgressNotification struct {
	ProgressToken any     `json:"progressToken"`
	Progress      float64 `json:"progress"`
	Message       string  `json:"message,omitempty"`
}

// NewProgressNotification builds a validated progress payload.
func NewProgressNotification(token any, progress float64, message string) (ProgressNotification, error) {
	if token == nil {
		return ProgressNotification{}, errors.New("progress token is required")
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	return ProgressNotification{
		ProgressToken: token,
		Progress:      progress,
		Message:       message,
	}, nil
}

// BuildProgressMessage constructs a JSON-RPC notification for progress updates.
func BuildProgressMessage(token any, progress float64, message string) (*JSONRPCMessage, error) {
	payload, err := NewProgressNotification(token, progress, message)
	if err != nil {
		return nil, err
	}
	return NewNotification(ProgressMethod, payload)
}

// ParseProgressNotification decodes a JSON-RPC message into a progress payload.
func ParseProgressNotification(msg *JSONRPCMessage) (ProgressNotification, error) {
	if msg == nil {
		return ProgressNotification{}, errors.New("message is nil")
	}
	if msg.Method != ProgressMethod {
		return ProgressNotification{}, fmt.Errorf("unexpected method %q", msg.Method)
	}
	if len(msg.Params) == 0 {
		return ProgressNotification{}, errors.New("missing params")
	}
	var payload ProgressNotification
	if err := json.Unmarshal(msg.Params, &payload); err != nil {
		return ProgressNotification{}, fmt.Errorf("decode progress notification: %w", err)
	}
	return payload, nil
}

