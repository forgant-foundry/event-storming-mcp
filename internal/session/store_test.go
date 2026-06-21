package session_test

import (
	"os"
	"strings"
	"testing"

	"github.com/forgant-foundry/event-storming-mcp/internal/session"
)

func TestSessionLifecycle(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	s, err := session.New("E-commerce checkout", "Discovering the checkout bounded context")
	if err != nil {
		t.Fatal(err)
	}
	if s.ID == "" {
		t.Fatal("expected non-empty session ID")
	}

	loaded, err := session.Load(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "E-commerce checkout" {
		t.Errorf("name: got %q, want %q", loaded.Name, "E-commerce checkout")
	}

	_, actor, err := session.AddActor(s.ID, "Customer", session.ActorTypePerson, "")
	if err != nil {
		t.Fatal(err)
	}
	if actor.ID == "" {
		t.Fatal("expected non-empty actor ID")
	}

	_, cmd, err := session.AddCommand(s.ID, "PlaceOrder", "", actor.ID, "", []string{"cart is not empty"})
	if err != nil {
		t.Fatal(err)
	}

	_, evt, err := session.AddEvent(s.ID, "OrderPlaced", "emitted when order is confirmed", cmd.ID, "")
	if err != nil {
		t.Fatal(err)
	}

	_, term, err := session.DefineTerm(s.ID, "Order", "A request to purchase one or more products", "the order", "", []string{"cart", "basket"})
	if err != nil {
		t.Fatal(err)
	}

	loaded, err = session.Load(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Actors) != 1 || loaded.Actors[0].ID != actor.ID {
		t.Errorf("actors: got %v", loaded.Actors)
	}
	if len(loaded.Commands) != 1 || loaded.Commands[0].ID != cmd.ID {
		t.Errorf("commands: got %v", loaded.Commands)
	}
	if len(loaded.Events) != 1 || loaded.Events[0].ID != evt.ID {
		t.Errorf("events: got %v", loaded.Events)
	}
	if len(loaded.Glossary) != 1 || loaded.Glossary[0].ID != term.ID {
		t.Errorf("glossary: got %v", loaded.Glossary)
	}

	_, updated, err := session.DefineTerm(s.ID, "Order", "Updated definition", "the order", "", []string{"cart"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != term.ID {
		t.Errorf("upsert should preserve term ID: got %s, want %s", updated.ID, term.ID)
	}

	loaded, err = session.Load(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Glossary) != 1 {
		t.Errorf("upsert should not duplicate: got %d terms", len(loaded.Glossary))
	}
	if loaded.Glossary[0].Definition != "Updated definition" {
		t.Errorf("upsert should update definition: got %q", loaded.Glossary[0].Definition)
	}
}

func TestLinkEvent(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	s, _ := session.New("Linking test", "")
	_, evt, _ := session.AddEvent(s.ID, "OrderPlaced", "", "", "")
	_, cmd, _ := session.AddCommand(s.ID, "PlaceOrder", "", "", "", nil)

	_, linked, err := session.LinkEvent(s.ID, evt.ID, cmd.ID)
	if err != nil {
		t.Fatal(err)
	}
	if linked.TriggerCommandID != cmd.ID {
		t.Errorf("TriggerCommandID: got %q, want %q", linked.TriggerCommandID, cmd.ID)
	}
	if linked.ID != evt.ID {
		t.Error("link_event must not change the event ID")
	}

	loaded, _ := session.Load(s.ID)
	if len(loaded.Events) != 1 {
		t.Errorf("link_event must not create duplicates: got %d events", len(loaded.Events))
	}

	_, _, err = session.LinkEvent(s.ID, "evt_doesnotexist", cmd.ID)
	if err == nil {
		t.Error("expected error for unknown event ID")
	}
}

func TestRemoveEvent(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	s, _ := session.New("Remove test", "")
	_, evt1, _ := session.AddEvent(s.ID, "OrderPlaced", "", "", "")
	_, evt2, _ := session.AddEvent(s.ID, "OrderCancelled", "", "", "")
	session.ClusterAggregate(s.ID, "Order", "", "", []string{evt1.ID, evt2.ID}, nil)

	_, err := session.RemoveEvent(s.ID, evt1.ID)
	if err != nil {
		t.Fatal(err)
	}

	loaded, _ := session.Load(s.ID)
	if len(loaded.Events) != 1 || loaded.Events[0].ID != evt2.ID {
		t.Errorf("wrong events after remove: %v", loaded.Events)
	}
	// Must also be removed from the aggregate
	if len(loaded.Aggregates[0].EventIDs) != 1 || loaded.Aggregates[0].EventIDs[0] != evt2.ID {
		t.Errorf("aggregate EventIDs not updated: %v", loaded.Aggregates[0].EventIDs)
	}

	_, err = session.RemoveEvent(s.ID, "evt_doesnotexist")
	if err == nil {
		t.Error("expected error for unknown event ID")
	}
}

func TestSyncPath(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	syncFile := t.TempDir() + "/.event-storming/session.json"

	s, _ := session.New("Sync test", "")
	session.SetSyncPath(s.ID, syncFile)

	// Adding an event should dual-write.
	session.AddEvent(s.ID, "ThingHappened", "", "", "")

	data, err := os.ReadFile(syncFile)
	if err != nil {
		t.Fatalf("sync file not written: %v", err)
	}
	if !strings.Contains(string(data), "ThingHappened") {
		t.Error("sync file missing ThingHappened event")
	}

	// Disabling sync path stops dual-writes.
	session.SetSyncPath(s.ID, "")
	session.AddEvent(s.ID, "AnotherThing", "", "", "")

	data2, _ := os.ReadFile(syncFile)
	if strings.Contains(string(data2), "AnotherThing") {
		t.Error("sync file should not have been updated after path was cleared")
	}
}

func TestExportImport(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	s, _ := session.New("Export test", "A session to export")
	session.AddActor(s.ID, "User", session.ActorTypePerson, "")
	session.DefineTerm(s.ID, "Widget", "A thing", "the widget", "", nil)

	exportPath := t.TempDir() + "/session.json"
	if err := session.ExportSession(s.ID, exportPath); err != nil {
		t.Fatal(err)
	}

	// File must exist and be valid JSON
	if _, err := os.Stat(exportPath); err != nil {
		t.Fatal("export file not created")
	}

	// Import gives a fresh session ID
	imported, originalID, err := session.ImportSession(exportPath)
	if err != nil {
		t.Fatal(err)
	}
	if imported.ID == s.ID {
		t.Error("import should assign a new session ID")
	}
	if originalID != s.ID {
		t.Errorf("originalID: got %s, want %s", originalID, s.ID)
	}
	if imported.Name != "Export test" {
		t.Errorf("name not preserved: got %q", imported.Name)
	}

	// Imported session is persisted and loadable
	loaded, err := session.Load(imported.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Actors) != 1 {
		t.Errorf("actors not preserved: got %d", len(loaded.Actors))
	}
	if len(loaded.Glossary) != 1 {
		t.Errorf("glossary not preserved: got %d", len(loaded.Glossary))
	}
}

func TestLoadNonexistent(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	_, err := session.Load("ses_doesnotexist")
	if err == nil {
		t.Fatal("expected error loading nonexistent session")
	}
}

func TestClusterAggregate(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	s, _ := session.New("Ordering", "")
	_, actor, _ := session.AddActor(s.ID, "Customer", session.ActorTypePerson, "")
	_, cmd, _ := session.AddCommand(s.ID, "PlaceOrder", "", actor.ID, "", nil)
	_, evt, _ := session.AddEvent(s.ID, "OrderPlaced", "", cmd.ID, "")

	_, agg, err := session.ClusterAggregate(s.ID, "Order", "", "", []string{evt.ID}, []string{cmd.ID})
	if err != nil {
		t.Fatal(err)
	}
	if agg.Name != "Order" {
		t.Errorf("aggregate name: got %q", agg.Name)
	}

	loaded, _ := session.Load(s.ID)
	if loaded.Events[0].AggregateID != agg.ID {
		t.Errorf("event aggregate_id not updated: got %q", loaded.Events[0].AggregateID)
	}
	if loaded.Commands[0].AggregateID != agg.ID {
		t.Errorf("command aggregate_id not updated: got %q", loaded.Commands[0].AggregateID)
	}
}

func TestClusterAggregate_Conflicts(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	s, _ := session.New("Ordering", "")
	_, evt1, _ := session.AddEvent(s.ID, "OrderPlaced", "", "", "")
	_, evt2, _ := session.AddEvent(s.ID, "OrderCancelled", "", "", "")

	session.ClusterAggregate(s.ID, "Order", "", "", []string{evt1.ID}, nil)

	// evt1 already belongs to Order aggregate — conflict
	_, _, err := session.ClusterAggregate(s.ID, "OrderV2", "", "", []string{evt1.ID, evt2.ID}, nil)
	if err == nil {
		t.Fatal("expected conflict error when re-assigning event to a second aggregate")
	}

	// Unknown event ID
	_, _, err = session.ClusterAggregate(s.ID, "Foo", "", "", []string{"evt_unknown"}, nil)
	if err == nil {
		t.Fatal("expected error for unknown event ID")
	}
}

func TestProposeContext(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	s, _ := session.New("E-commerce", "")
	_, evt, _ := session.AddEvent(s.ID, "OrderPlaced", "", "", "")
	_, agg, _ := session.ClusterAggregate(s.ID, "Order", "", "", []string{evt.ID}, nil)
	_, term, _ := session.DefineTerm(s.ID, "Order", "A purchase request", "the order", "", nil)

	_, ctx, err := session.ProposeContext(s.ID, "Ordering", "Handles order lifecycle",
		[]string{agg.ID}, []string{term.ID})
	if err != nil {
		t.Fatal(err)
	}

	loaded, _ := session.Load(s.ID)
	if loaded.Aggregates[0].ContextID != ctx.ID {
		t.Errorf("aggregate context_id not updated: got %q", loaded.Aggregates[0].ContextID)
	}
	if loaded.Glossary[0].ContextID != ctx.ID {
		t.Errorf("glossary term context_id not updated: got %q", loaded.Glossary[0].ContextID)
	}

	// Aggregate already in a context — conflict
	_, _, err = session.ProposeContext(s.ID, "OrderingV2", "", []string{agg.ID}, nil)
	if err == nil {
		t.Fatal("expected conflict error when re-assigning aggregate to a second context")
	}
}

func TestLintStep(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	s, _ := session.New("Shop", "")
	_, ctx1, _ := session.ProposeContext(s.ID, "Ordering", "", nil, nil)
	_, ctx2, _ := session.ProposeContext(s.ID, "Payments", "", nil, nil)

	session.DefineTerm(s.ID, "Order",
		"A purchase request", "the order", ctx1.ID, []string{"cart", "basket"})
	session.DefineTerm(s.ID, "Payment",
		"A financial transaction", "the payment", ctx2.ID, nil)

	loaded, _ := session.Load(s.ID)

	t.Run("clean step", func(t *testing.T) {
		issues := session.LintStep(loaded, ctx1.ID, "the order has been confirmed")
		if len(issues) != 0 {
			t.Errorf("expected no issues, got %v", issues)
		}
	})

	t.Run("alias used", func(t *testing.T) {
		issues := session.LintStep(loaded, ctx1.ID, "the user adds items to their cart")
		if len(issues) == 0 {
			t.Fatal("expected alias issue for 'cart'")
		}
		if issues[0].Type != "alias" || issues[0].Found != "cart" {
			t.Errorf("unexpected issue: %+v", issues[0])
		}
	})

	t.Run("cross-context term", func(t *testing.T) {
		issues := session.LintStep(loaded, ctx1.ID, "the payment is processed")
		found := false
		for _, i := range issues {
			if i.Type == "cross_context" && i.Term == "Payment" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected cross_context issue for Payment, got %v", issues)
		}
	})

	t.Run("no context id skips cross-context check", func(t *testing.T) {
		issues := session.LintStep(loaded, "", "the payment is processed")
		for _, i := range issues {
			if i.Type == "cross_context" {
				t.Errorf("should not flag cross_context when contextID is empty")
			}
		}
	})
}

func TestAddPolicyReadModelHotSpot(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	s, _ := session.New("Payments", "")
	_, evt, _ := session.AddEvent(s.ID, "PaymentAuthorised", "", "", "")
	_, cmd, _ := session.AddCommand(s.ID, "CapturePayment", "", "", "", nil)
	_, actor, _ := session.AddActor(s.ID, "Payment Gateway", session.ActorTypeExternalSystem, "")

	_, pol, err := session.AddPolicy(s.ID, "Capture on authorise", "", evt.ID, cmd.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if pol.TriggerEventID != evt.ID {
		t.Errorf("policy trigger: got %q", pol.TriggerEventID)
	}

	_, rm, err := session.AddReadModel(s.ID, "Payment Status", "", actor.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if rm.ActorID != actor.ID {
		t.Errorf("read model actor: got %q", rm.ActorID)
	}

	_, hs, err := session.AddHotSpot(s.ID, "Who owns refund approval?", "")
	if err != nil {
		t.Fatal(err)
	}
	if hs.Resolved {
		t.Error("new hot spot should not be resolved")
	}

	loaded, _ := session.Load(s.ID)
	if len(loaded.Policies) != 1 || len(loaded.ReadModels) != 1 || len(loaded.HotSpots) != 1 {
		t.Errorf("persistence: policies=%d readmodels=%d hotspots=%d",
			len(loaded.Policies), len(loaded.ReadModels), len(loaded.HotSpots))
	}
}
