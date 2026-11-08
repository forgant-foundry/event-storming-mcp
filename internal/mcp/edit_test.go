package mcp

import (
	"context"
	"strings"
	"testing"

	libmcp "github.com/mark3labs/mcp-go/mcp"

	"github.com/forgant-foundry/event-storming-mcp/internal/session"
)

func call(t *testing.T, h func(context.Context, libmcp.CallToolRequest) (*libmcp.CallToolResult, error), args map[string]any) *libmcp.CallToolResult {
	t.Helper()
	var req libmcp.CallToolRequest
	req.Params.Arguments = args
	res, err := h(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// TestUpdateCommand_OmittedVersusEmpty checks that an omitted argument leaves
// the field alone while an empty one clears it.
func TestUpdateCommand_OmittedVersusEmpty(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Ordering", "")
	_, cmd, _ := session.AddCommand(s.ID, session.Command{Name: "PlaceOrder", Notes: "keep me", Preconditions: []string{"a cart exists"}})

	res := call(t, handleUpdateCommand, map[string]any{
		"session_id":    s.ID,
		"command_id":    cmd.ID,
		"name":          "SubmitOrder",
		"preconditions": []any{},
	})
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res.Content)
	}
	loaded, _ := session.Load(s.ID)
	got := loaded.Commands[0]
	if got.Name != "SubmitOrder" || got.Notes != "keep me" || len(got.Preconditions) != 0 {
		t.Errorf("got %+v", got)
	}

	res = call(t, handleUpdateCommand, map[string]any{"session_id": s.ID, "command_id": cmd.ID, "notes": ""})
	loaded, _ = session.Load(s.ID)
	if res.IsError || loaded.Commands[0].Notes != "" {
		t.Errorf("empty notes should clear: %+v", loaded.Commands[0])
	}

	res = call(t, handleUpdateCommand, map[string]any{"session_id": s.ID, "command_id": cmd.ID, "source": "llm"})
	if !res.IsError {
		t.Error("expected tool error for invalid source")
	}
}

func TestRemoveItem_ReportsDetachments(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Ordering", "")
	_, cmd, _ := session.AddCommand(s.ID, session.Command{Name: "PlaceOrder"})
	session.AddEvent(s.ID, session.Event{Name: "OrderPlaced", TriggerCommandID: cmd.ID})

	res := call(t, handleRemoveItem, map[string]any{"session_id": s.ID, "kind": "command", "id": cmd.ID})
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res.Content)
	}
	text := res.Content[0].(libmcp.TextContent).Text
	if want := `trigger_command_id cleared`; !strings.Contains(text, want) {
		t.Errorf("result should report detachment %q:\n%s", want, text)
	}
}
