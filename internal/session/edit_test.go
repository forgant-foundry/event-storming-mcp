package session_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/forgant-foundry/event-storming-mcp/internal/session"
)

func ptr[T any](v T) *T { return &v }

func TestUpdateCommand(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Ordering", "")
	_, actor, _ := session.AddActor(s.ID, "Customer", session.ActorTypePerson, "")
	_, cmd, _ := session.AddCommand(s.ID, session.Command{Name: "PlaceOrder", Notes: "first draft", ActorID: actor.ID, Preconditions: []string{"a cart exists"}, Source: session.SourceFacilitator})

	_, got, err := session.UpdateCommand(s.ID, cmd.ID, session.CommandPatch{
		Notes:         ptr(""),
		Preconditions: ptr([]string{"a cart exists", "the cart is not empty"}),
		Source:        ptr(session.SourceExpert),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "PlaceOrder" || got.ActorID != actor.ID {
		t.Errorf("omitted fields should be unchanged: %+v", got)
	}
	if got.Notes != "" || len(got.Preconditions) != 2 || got.Source != session.SourceExpert {
		t.Errorf("given fields should be replaced: %+v", got)
	}

	if _, _, err := session.UpdateCommand(s.ID, cmd.ID, session.CommandPatch{ActorID: ptr("act_missing")}); err == nil {
		t.Error("expected error for unknown actor")
	}
	if _, _, err := session.UpdateCommand(s.ID, cmd.ID, session.CommandPatch{Name: ptr("")}); err == nil {
		t.Error("expected error for empty name")
	}
	if _, _, err := session.UpdateCommand(s.ID, cmd.ID, session.CommandPatch{Source: ptr(session.Source("llm"))}); err == nil {
		t.Error("expected error for unknown source")
	}
}

func TestUpdateAggregateNotes(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Factory", "")
	_, agg, _ := session.ClusterAggregate(s.ID, "Handoff", "Rule: surface priority, bug bar and budget are all set", "", nil, nil)

	newNotes := "Rule: surface priority, bug bar, budget and scale envelope are all set"
	_, got, err := session.UpdateAggregate(s.ID, agg.ID, session.AggregatePatch{Notes: &newNotes})
	if err != nil {
		t.Fatal(err)
	}
	if got.Notes != newNotes || got.Name != "Handoff" {
		t.Errorf("got %+v", got)
	}
}

func TestUpdatePolicyCondition(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Factory", "")
	_, evt, _ := session.AddEvent(s.ID, session.Event{Name: "CompletenessAssessed"})
	_, cmd, _ := session.AddCommand(s.ID, session.Command{Name: "DeclareArrival"})
	_, pol, err := session.AddPolicy(s.ID, "Declare arrival", "", evt.ID, cmd.ID, "", "the assessment found no gaps", session.SourceExpert)
	if err != nil {
		t.Fatal(err)
	}
	if pol.Condition != "the assessment found no gaps" {
		t.Errorf("condition: got %q", pol.Condition)
	}

	_, got, err := session.UpdatePolicy(s.ID, pol.ID, session.PolicyPatch{Condition: ptr("")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Condition != "" || got.TriggerEventID != evt.ID {
		t.Errorf("got %+v", got)
	}

	if _, _, err := session.AddPolicy(s.ID, "Bad", "", "evt_missing", "", "", "", ""); err == nil {
		t.Error("expected error for unknown trigger event")
	}
}

func TestReadModelActors(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	s, _ := session.New("Factory", "")
	_, director, _ := session.AddActor(s.ID, "Director", session.ActorTypeRole, "")
	_, cell, _ := session.AddActor(s.ID, "Cell Agent", session.ActorTypeRole, "")

	_, rm, err := session.AddReadModel(s.ID, "Completeness Ledger", "", []string{director.ID, cell.ID}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(rm.ActorIDs, []string{director.ID, cell.ID}) {
		t.Errorf("actor_ids: got %v", rm.ActorIDs)
	}
	if _, _, err := session.AddReadModel(s.ID, "Bad", "", []string{"act_missing"}, "", ""); err == nil {
		t.Error("expected error for unknown actor")
	}

	// A legacy file with a single actor_id is migrated on load.
	path := filepath.Join(dataHome, "event-storming-mcp", s.ID, "session.json")
	raw, _ := os.ReadFile(path)
	legacy := strings.Replace(string(raw), `"actor_ids": [
        "`+director.ID+`",
        "`+cell.ID+`"
      ],`, `"actor_id": "`+director.ID+`",`, 1)
	if legacy == string(raw) {
		t.Fatal("test setup: failed to rewrite actor_ids as legacy actor_id")
	}
	os.WriteFile(path, []byte(legacy), 0o600)
	loaded, _ := session.Load(s.ID)
	got := loaded.ReadModels[0]
	if got.ActorID != "" || !slices.Equal(got.ActorIDs, []string{director.ID}) {
		t.Errorf("legacy actor_id should fold into actor_ids: %+v", got)
	}
}

func TestAssignToAggregate(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Factory", "")
	_, c1, _ := session.AddCommand(s.ID, session.Command{Name: "SetBugBar"})
	_, c2, _ := session.AddCommand(s.ID, session.Command{Name: "SetScaleEnvelope"})
	_, e2, _ := session.AddEvent(s.ID, session.Event{Name: "ScaleEnvelopeSet", TriggerCommandID: c2.ID})
	_, handoff, _ := session.ClusterAggregate(s.ID, "Handoff", "", "", nil, []string{c1.ID})
	_, other, _ := session.ClusterAggregate(s.ID, "Other", "", "", nil, nil)

	_, got, err := session.AssignToAggregate(s.ID, handoff.ID, []string{e2.ID}, []string{c2.ID}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.CommandIDs, []string{c1.ID, c2.ID}) || !slices.Equal(got.EventIDs, []string{e2.ID}) {
		t.Errorf("handoff members: commands %v events %v", got.CommandIDs, got.EventIDs)
	}

	// Re-assigning to the same aggregate is a no-op, not a conflict.
	if _, _, err := session.AssignToAggregate(s.ID, handoff.ID, nil, []string{c2.ID}, false); err != nil {
		t.Errorf("re-assign to same aggregate: %v", err)
	}
	// Taking from another aggregate needs move.
	if _, _, err := session.AssignToAggregate(s.ID, other.ID, nil, []string{c2.ID}, false); err == nil {
		t.Error("expected conflict without move")
	}
	if _, _, err := session.AssignToAggregate(s.ID, other.ID, nil, []string{c2.ID}, true); err != nil {
		t.Fatal(err)
	}
	loaded, _ := session.Load(s.ID)
	if !slices.Equal(loaded.Aggregates[0].CommandIDs, []string{c1.ID}) {
		t.Errorf("moved command should leave Handoff: %v", loaded.Aggregates[0].CommandIDs)
	}
	if !slices.Equal(loaded.Aggregates[1].CommandIDs, []string{c2.ID}) {
		t.Errorf("moved command should join Other: %v", loaded.Aggregates[1].CommandIDs)
	}
	if _, _, err := session.AssignToAggregate(s.ID, "agg_missing", nil, []string{c1.ID}, false); err == nil {
		t.Error("expected error for unknown aggregate")
	}
}

func TestAssignToContext(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Factory", "")
	_, a1, _ := session.ClusterAggregate(s.ID, "Handoff", "", "", nil, nil)
	_, a2, _ := session.ClusterAggregate(s.ID, "ThreatDecision", "", "", nil, nil)
	_, term, _ := session.DefineTerm(s.ID, "Bug Bar", "The severity threshold.", "", "", nil, nil)
	_, ctx, _ := session.ProposeContext(s.ID, "Scoping", "", []string{a1.ID}, nil)

	_, got, err := session.AssignToContext(s.ID, ctx.ID, []string{a2.ID}, []string{term.ID}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.AggregateIDs, []string{a1.ID, a2.ID}) || !slices.Equal(got.GlossaryTermIDs, []string{term.ID}) {
		t.Errorf("context members: aggregates %v terms %v", got.AggregateIDs, got.GlossaryTermIDs)
	}
}

func TestRemoveItemCascades(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Ordering", "")
	_, actor, _ := session.AddActor(s.ID, "Customer", session.ActorTypePerson, "")
	_, cmd, _ := session.AddCommand(s.ID, session.Command{Name: "PlaceOrder", ActorID: actor.ID})
	_, evt, _ := session.AddEvent(s.ID, session.Event{Name: "OrderPlaced", TriggerCommandID: cmd.ID})
	_, pol, _ := session.AddPolicy(s.ID, "Reorder", "", evt.ID, cmd.ID, "", "", "")
	_, rm, _ := session.AddReadModel(s.ID, "Order History", "", []string{actor.ID}, "", "")
	_, agg, _ := session.ClusterAggregate(s.ID, "Order", "", "", []string{evt.ID}, []string{cmd.ID})
	_, ctx, _ := session.ProposeContext(s.ID, "Sales", "", []string{agg.ID}, nil)
	_, hs, _ := session.AddHotSpot(s.ID, "Who approves refunds?", ctx.ID)

	// Removing the command detaches its event and the policy.
	_, detached, err := session.RemoveItem(s.ID, session.KindCommand, cmd.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detached) != 2 {
		t.Errorf("expected 2 detachments, got %v", detached)
	}
	loaded, _ := session.Load(s.ID)
	if loaded.Events[0].TriggerCommandID != "" || loaded.Policies[0].ResultingCommandID != "" {
		t.Error("references to the removed command should be cleared")
	}
	if len(loaded.Aggregates[0].CommandIDs) != 0 {
		t.Errorf("aggregate should drop the removed command: %v", loaded.Aggregates[0].CommandIDs)
	}

	// Removing the actor drops it from the read model.
	session.RemoveItem(s.ID, session.KindActor, actor.ID)
	loaded, _ = session.Load(s.ID)
	if len(loaded.ReadModels[0].ActorIDs) != 0 {
		t.Errorf("read model %s should drop the removed actor: %v", rm.ID, loaded.ReadModels[0].ActorIDs)
	}

	// Removing the event clears the policy trigger (remove_event cascades too).
	if _, err := session.RemoveEvent(s.ID, evt.ID); err != nil {
		t.Fatal(err)
	}
	loaded, _ = session.Load(s.ID)
	if loaded.Policies[0].TriggerEventID != "" {
		t.Errorf("policy %s trigger should be cleared", pol.ID)
	}

	// Removing the context clears the aggregate's and hot spot's context.
	session.RemoveItem(s.ID, session.KindContext, ctx.ID)
	loaded, _ = session.Load(s.ID)
	if loaded.Aggregates[0].ContextID != "" || loaded.HotSpots[0].ContextID != "" {
		t.Errorf("context references should be cleared (hot spot %s)", hs.ID)
	}

	if _, _, err := session.RemoveItem(s.ID, "sticky", "x"); err == nil {
		t.Error("expected error for unknown kind")
	}
	if _, _, err := session.RemoveItem(s.ID, session.KindPolicy, "pol_missing"); err == nil {
		t.Error("expected error for unknown id")
	}
}

func TestImportSessionSyncPathOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))

	s, _ := session.New("Ordering", "")
	original := filepath.Join(dir, "original.json")
	session.SetSyncPath(s.ID, original)
	exported := filepath.Join(dir, "export.json")
	session.ExportSession(s.ID, exported)

	replacement := filepath.Join(dir, "replacement.json")
	imported, _, err := session.ImportSession(exported, &replacement)
	if err != nil {
		t.Fatal(err)
	}
	if imported.SyncPath != replacement {
		t.Errorf("sync_path: got %q, want %q", imported.SyncPath, replacement)
	}
	raw, err := os.ReadFile(replacement)
	if err != nil {
		t.Fatalf("replacement sync file not written: %v", err)
	}
	var synced session.Session
	json.Unmarshal(raw, &synced)
	if synced.ID != imported.ID {
		t.Errorf("replacement holds %s, want %s", synced.ID, imported.ID)
	}

	disabled := ""
	imported, _, _ = session.ImportSession(exported, &disabled)
	if imported.SyncPath != "" {
		t.Errorf("empty override should disable sync, got %q", imported.SyncPath)
	}
}
