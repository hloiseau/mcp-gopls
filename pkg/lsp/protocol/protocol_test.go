package protocol

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"
)

func TestJSONRPCMessageUnmarshal(t *testing.T) {
	raw := `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`
	var msg JSONRPCMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.ID == nil || msg.JSONRPC != "2.0" {
		t.Fatalf("unexpected message %#v", msg)
	}
	if err := msg.ParseResult(&struct{}{}); err != nil {
		t.Fatalf("parse result: %v", err)
	}
}

func TestJSONRPCParseResultErrors(t *testing.T) {
	msg := JSONRPCMessage{}
	if err := msg.ParseResult(&struct{}{}); err == nil {
		t.Fatal("expected error for missing result")
	}
	msg = JSONRPCMessage{
		Error: &JSONRPCError{Code: -32000, Message: "boom"},
	}
	if err := msg.ParseResult(&struct{}{}); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestTransportReceiveLogsMessages(t *testing.T) {
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	var buf bytes.Buffer
	log.SetOutput(&buf)

	body := `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`
	var wire bytes.Buffer
	fmt.Fprintf(&wire, "Content-Length: %d\r\n\r\n%s", len(body), body)
	tr := NewTransport(bufio.NewReader(&wire), io.Discard)

	msg, err := tr.ReceiveMessage(context.Background())
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message")
	}
	if !strings.Contains(buf.String(), "📥 response message received") {
		t.Fatalf("expected log line, got %q", buf.String())
	}
}

func TestProgressNotificationHelpers(t *testing.T) {
	payload, err := NewProgressNotification("token", 1.5, "done")
	if err != nil {
		t.Fatalf("new progress: %v", err)
	}
	if payload.Progress != 1 {
		t.Fatalf("expected capped progress, got %f", payload.Progress)
	}

	msg, err := BuildProgressMessage("token", 0.25, "quarter")
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	parsed, err := ParseProgressNotification(msg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Message != "quarter" || parsed.Progress != 0.25 {
		t.Fatalf("unexpected payload %#v", parsed)
	}

	if _, err := NewProgressNotification(nil, 0, ""); err == nil {
		t.Fatal("expected error for nil token")
	}
	if _, err := ParseProgressNotification(&JSONRPCMessage{Method: "other"}); err == nil {
		t.Fatal("expected method mismatch error")
	}
	if _, err := ParseProgressNotification(nil); err == nil {
		t.Fatal("expected nil message error")
	}
}
