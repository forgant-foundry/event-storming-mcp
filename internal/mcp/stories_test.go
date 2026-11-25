package mcp

import (
	"encoding/json"
	"testing"

	libmcp "github.com/mark3labs/mcp-go/mcp"

	"github.com/forgant-foundry/forgant-discovery/internal/session"
)

func decode(t *testing.T, res *libmcp.CallToolResult, v any) {
	t.Helper()
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res.Content)
	}
	if err := json.Unmarshal([]byte(res.Content[0].(libmcp.TextContent).Text), v); err != nil {
		t.Fatal(err)
	}
}

func TestStories_CaptureAndCheckLinks(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Ledger", "")
	_, cmd, _ := session.AddCommand(s.ID, session.Command{Name: "RecordTransaction"})
	_, rm, _ := session.AddReadModel(s.ID, "Account Balance", "", nil, "", "")

	var role session.Role
	decode(t, call(t, handleAddRole, map[string]any{"session_id": s.ID, "actor_name": "Accountant", "goal": "keep the books"}), &role)
	var act session.Activity
	decode(t, call(t, handleAddActivity, map[string]any{"session_id": s.ID, "name": "manage my ledgers"}), &act)
	var screen session.Screen
	decode(t, call(t, handleAddScreen, map[string]any{
		"session_id": s.ID, "name": "Transactions", "shows": []any{rm.ID},
		"offers": []any{map[string]any{"command_id": cmd.ID, "shows_after": "the new balance"}},
	}), &screen)
	if len(screen.Offers) != 1 || screen.Offers[0].ShowsAfter != "the new balance" {
		t.Errorf("offers should parse as objects: %+v", screen.Offers)
	}
	var step session.Step
	decode(t, call(t, handleAddStep, map[string]any{
		"session_id": s.ID, "activity_id": act.ID, "role_id": role.ID, "name": "record a transaction",
		"command_ids": []any{cmd.ID}, "screen_id": screen.ID,
	}), &step)

	// The command has no events yet, so the screen's control would do nothing.
	var report struct {
		Clean    bool
		Counts   map[string]int
		Findings []session.LinkFinding
	}
	decode(t, call(t, handleCheckLinks, map[string]any{"session_id": s.ID}), &report)
	if report.Clean || report.Counts["warning"] != 1 || report.Findings[0].Kind != "screen" {
		t.Errorf("expected one screen warning: %+v", report)
	}

	session.AddEvent(s.ID, session.Event{Name: "TransactionRecorded", TriggerCommandID: cmd.ID})
	decode(t, call(t, handleCheckLinks, map[string]any{"session_id": s.ID}), &report)
	if !report.Clean {
		t.Errorf("expected a clean link once the command has an event: %+v", report)
	}

	if res := call(t, handleAddStory, map[string]any{"session_id": s.ID, "step_id": step.ID, "title": "x", "kind": "epic"}); !res.IsError {
		t.Error("expected a tool error for an unknown story kind")
	}
}

func TestStories_UpdateSliceWalkingSkeletonFlag(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Ledger", "")
	var sl session.Slice
	decode(t, call(t, handleAddSlice, map[string]any{"session_id": s.ID, "name": "skeleton", "kind": "development"}), &sl)
	decode(t, call(t, handleUpdateSlice, map[string]any{"session_id": s.ID, "slice_id": sl.ID, "walking_skeleton": true}), &sl)
	if !sl.WalkingSkeleton {
		t.Error("walking_skeleton should be set")
	}
	// Omitting the flag leaves it alone.
	decode(t, call(t, handleUpdateSlice, map[string]any{"session_id": s.ID, "slice_id": sl.ID, "outcome": "see it end to end"}), &sl)
	if !sl.WalkingSkeleton || sl.Outcome != "see it end to end" {
		t.Errorf("omitted flag should be unchanged: %+v", sl)
	}
}

func TestStories_CriteriaReadinessAndEmit(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Ledger", "")
	_, cmd, _ := session.AddCommand(s.ID, session.Command{Name: "RecordTransaction"})
	session.AddEvent(s.ID, session.Event{Name: "TransactionRecorded", TriggerCommandID: cmd.ID})
	_, web, _ := session.ProposeContext(s.ID, "Web", "", nil, nil)

	var role session.Role
	decode(t, call(t, handleAddRole, map[string]any{"session_id": s.ID, "actor_name": "Accountant"}), &role)
	var act session.Activity
	decode(t, call(t, handleAddActivity, map[string]any{"session_id": s.ID, "name": "keep my records"}), &act)
	var screen session.Screen
	decode(t, call(t, handleAddScreen, map[string]any{"session_id": s.ID, "name": "Account", "context_id": web.ID,
		"offers": []any{map[string]any{"command_id": cmd.ID, "shows_after": "the transaction"}}}), &screen)
	var step session.Step
	decode(t, call(t, handleAddStep, map[string]any{"session_id": s.ID, "activity_id": act.ID, "role_id": role.ID,
		"name": "record a transaction", "command_ids": []any{cmd.ID}, "screen_id": screen.ID}), &step)
	var sl session.Slice
	decode(t, call(t, handleAddSlice, map[string]any{"session_id": s.ID, "name": "walking skeleton", "kind": "development",
		"walking_skeleton": true, "preconditions": []any{"the product is running"}}), &sl)
	var story session.Story
	decode(t, call(t, handleAddStory, map[string]any{"session_id": s.ID, "step_id": step.ID, "title": "record a transaction",
		"why": "my balance is right", "slice_id": sl.ID}), &story)

	// Not ready yet: no criteria.
	var ready []session.SliceReadiness
	decode(t, call(t, handleCheckReadiness, map[string]any{"session_id": s.ID}), &ready)
	if len(ready) != 1 || ready[0].Ready {
		t.Fatalf("expected not ready: %+v", ready)
	}

	decode(t, call(t, handleAddCriterion, map[string]any{"session_id": s.ID, "story_id": story.ID,
		"text": "a transaction changes the balance", "given": []any{"a balance of <opening>"}, "when": "the accountant records <amount>",
		"then":     []any{"the balance is <closing>"},
		"examples": map[string]any{"columns": []any{"opening", "amount", "closing"}, "rows": []any{[]any{"1000.00", "-42.50", "957.50"}}},
	}), &story)
	if len(story.Criteria) != 1 || story.Criteria[0].Examples == nil || story.Criteria[0].Examples.Rows[0][2] != "957.50" {
		t.Fatalf("criterion should parse with its examples: %+v", story.Criteria)
	}

	var c session.Constraint
	decode(t, call(t, handleAddConstraint, map[string]any{"session_id": s.ID, "text": "the account screen opens promptly",
		"kind": "performance", "scope_kind": "screen", "scope_id": screen.ID, "measure": "the time to open it", "target": "under 1 second"}), &c)

	decode(t, call(t, handleCheckReadiness, map[string]any{"session_id": s.ID, "slice_id": sl.ID}), &ready)
	if len(ready) != 1 || !ready[0].Ready {
		t.Fatalf("expected ready: %+v", ready)
	}

	var sum struct {
		FeatureFiles  []string `json:"feature_files"`
		SlicesFile    string   `json:"slices_file"`
		ScenarioCount int      `json:"scenario_count"`
	}
	decode(t, call(t, handleEmitStoryMap, map[string]any{"session_id": s.ID, "output_dir": t.TempDir()}), &sum)
	// Journey, story criterion, screen offer, constraint.
	if sum.ScenarioCount != 4 || len(sum.FeatureFiles) != 4 || sum.SlicesFile == "" {
		t.Errorf("emit summary: %+v", sum)
	}
}
