package gherkin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/forgant-foundry/event-storming-mcp/internal/gherkin"
	"github.com/forgant-foundry/event-storming-mcp/internal/session"
)

// buildChatSession builds a minimal but realistic session: one aggregate in one
// bounded context, with an actor, a command, and a linked event.
func buildChatSession(t *testing.T) *session.Session {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	s, _ := session.New("Chat App", "Team messaging")

	_, actor, _ := session.AddActor(s.ID, "User", session.ActorTypePerson, "")
	_, cmd, _ := session.AddCommand(s.ID, "SendMessage", "", actor.ID, "",
		[]string{"user is a member of the channel"})
	_, evt, _ := session.AddEvent(s.ID, "MessageSent", "", cmd.ID, "")
	_, _, _ = session.AddEvent(s.ID, "NotificationSent", "", cmd.ID, "")

	_, agg, _ := session.ClusterAggregate(s.ID, "Message", "",
		"", []string{evt.ID}, []string{cmd.ID})

	_, term, _ := session.DefineTerm(s.ID, "Message",
		"A text payload sent by a user into a channel.", "the message",
		"", []string{"msg", "post"})

	_, ctx, _ := session.ProposeContext(s.ID, "Messaging",
		"Owns message lifecycle", []string{agg.ID}, []string{term.ID})
	_ = ctx

	// Reload so all IDs are consistent.
	loaded, err := session.Load(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	return loaded
}

func TestEmit_WritesOutputTriple(t *testing.T) {
	s := buildChatSession(t)
	outDir := t.TempDir()

	sum, err := gherkin.Emit(s, outDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(sum.FeatureFiles) != 1 {
		t.Errorf("feature files: got %d, want 1", len(sum.FeatureFiles))
	}
	if sum.GlossaryFile == "" {
		t.Error("glossary file path is empty")
	}
	if sum.ContextMapFile == "" {
		t.Error("context-map file path is empty")
	}
	if sum.ScenarioCount < 1 {
		t.Errorf("scenario count: got %d, want >= 1", sum.ScenarioCount)
	}
}

func TestEmit_FeatureContent(t *testing.T) {
	s := buildChatSession(t)
	outDir := t.TempDir()

	sum, err := gherkin.Emit(s, outDir)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(sum.FeatureFiles[0])
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	checks := []string{
		"@context:Messaging",
		"@aggregate:Message",
		"Feature: Message",
		"Scenario:",
		"Given user is a member of the channel",
		"When the user",
		"Then",
	}
	for _, want := range checks {
		if !strings.Contains(content, want) {
			t.Errorf("feature file missing %q\n\nGot:\n%s", want, content)
		}
	}
}

func TestEmit_GlossaryContent(t *testing.T) {
	s := buildChatSession(t)
	outDir := t.TempDir()

	sum, _ := gherkin.Emit(s, outDir)
	data, _ := os.ReadFile(sum.GlossaryFile)
	content := string(data)

	if !strings.Contains(content, "## Messaging") {
		t.Error("glossary missing Messaging section")
	}
	if !strings.Contains(content, "Message") {
		t.Error("glossary missing Message term")
	}
	if !strings.Contains(content, "msg") {
		t.Error("glossary missing alias 'msg'")
	}
}

func TestEmit_ContextMap(t *testing.T) {
	s := buildChatSession(t)
	outDir := t.TempDir()

	sum, _ := gherkin.Emit(s, outDir)
	data, _ := os.ReadFile(sum.ContextMapFile)
	content := string(data)

	if !strings.Contains(content, `"Messaging"`) {
		t.Error("context-map missing Messaging context")
	}
	if !strings.Contains(content, `"Message"`) {
		t.Error("context-map missing Message aggregate")
	}
}

func TestEmit_UncategorisedAggregate(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Test", "")
	_, evt, _ := session.AddEvent(s.ID, "ThingHappened", "", "", "")
	_, cmd, _ := session.AddCommand(s.ID, "DoThing", "", "", "", nil)
	// Link event to command so a scenario is emitted.
	session.AddEvent(s.ID, "ThingHappened2", "", cmd.ID, "")
	session.ClusterAggregate(s.ID, "Thing", "", "", []string{evt.ID}, []string{cmd.ID})

	loaded, _ := session.Load(s.ID)
	outDir := t.TempDir()
	sum, err := gherkin.Emit(loaded, outDir)
	if err != nil {
		t.Fatal(err)
	}

	// Should write to uncategorised/
	if len(sum.FeatureFiles) == 0 {
		t.Fatal("expected at least one feature file")
	}
	if !strings.Contains(sum.FeatureFiles[0], "uncategorised") {
		t.Errorf("expected uncategorised path, got %s", sum.FeatureFiles[0])
	}
}

func TestEmit_WarnOnCommandWithNoEvents(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Test", "")
	_, actor, _ := session.AddActor(s.ID, "User", session.ActorTypePerson, "")
	_, cmd, _ := session.AddCommand(s.ID, "DoSomething", "", actor.ID, "", nil)
	_, evt, _ := session.AddEvent(s.ID, "SomethingDone", "", "", "") // NOT linked to cmd
	session.ClusterAggregate(s.ID, "Thing", "", "", []string{evt.ID}, []string{cmd.ID})

	loaded, _ := session.Load(s.ID)
	outDir := t.TempDir()
	sum, err := gherkin.Emit(loaded, outDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Warnings) == 0 {
		t.Error("expected a warning for command with no linked events")
	}
}

func TestEmit_DeprecatedAggregate(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Test", "")
	_, evt, _ := session.AddEvent(s.ID, "PeriodClosed", "", "", "")
	_, agg, _ := session.ClusterAggregate(s.ID, "", "", "", []string{evt.ID}, nil)
	session.DeprecateAggregate(s.ID, agg.ID, "snapshots made this redundant")

	loaded, _ := session.Load(s.ID)
	outDir := t.TempDir()
	sum, err := gherkin.Emit(loaded, outDir)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(sum.FeatureFiles[0])
	content := string(data)
	if !strings.Contains(content, "@deprecated") {
		t.Error("deprecated aggregate should have @deprecated tag")
	}
	if strings.Contains(content, "Scenario:") {
		t.Error("deprecated aggregate should not emit scenarios")
	}
	if sum.ScenarioCount != 0 {
		t.Errorf("scenario count should be 0 for deprecated aggregate, got %d", sum.ScenarioCount)
	}
}

func TestEmit_ConditionalEvent(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Test", "")
	_, actor, _ := session.AddActor(s.ID, "User", session.ActorTypePerson, "")
	_, cmd, _ := session.AddCommand(s.ID, "ProcessPayment", "", actor.ID, "", nil)
	_, evtOk, _ := session.AddEvent(s.ID, "PaymentProcessed", "", cmd.ID, "")
	_, evtFail, _ := session.AddEvent(s.ID, "PaymentDeclined", "", cmd.ID, "")
	session.SetEventCondition(s.ID, evtFail.ID, "the payment method is invalid")
	session.ClusterAggregate(s.ID, "Payment", "", "", []string{evtOk.ID, evtFail.ID}, []string{cmd.ID})

	loaded, _ := session.Load(s.ID)
	outDir := t.TempDir()
	sum, err := gherkin.Emit(loaded, outDir)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(sum.FeatureFiles[0])
	content := string(data)

	if sum.ScenarioCount != 2 {
		t.Errorf("expected 2 scenarios (happy path + conditional), got %d", sum.ScenarioCount)
	}
	if !strings.Contains(content, "the payment method is invalid") {
		t.Error("conditional event's condition should appear as a Given step")
	}
}

func TestSlug_SpaceSeparated(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Ledger Management", "ledger-management"},
		{"BoundedContext", "bounded-context"},
		{"Order", "order"},
		{"ledger management", "ledger-management"},
		{"Ledger_Management", "ledger-management"},
	}
	for _, tc := range cases {
		got := gherkin.Slug(tc.in)
		if got != tc.want {
			t.Errorf("Slug(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestStyleHelpers(t *testing.T) {
	cases := []struct {
		fn   func() string
		want string
	}{
		{func() string { return gherkin.WhenStep("SendMessage", "User") }, "the user sends a message"},
		{func() string { return gherkin.WhenStep("PlaceOrder", "Customer") }, "the customer places an order"},
		{func() string { return gherkin.WhenStep("CreateChannel", "Admin") }, "the admin creates a channel"},
		{func() string { return gherkin.ThenStep("MessageSent") }, "a message is sent"},
		{func() string { return gherkin.ThenStep("OrderPlaced") }, "an order is placed"},
		{func() string { return gherkin.ThenStep("ChannelCreated") }, "a channel is created"},
		{func() string { return gherkin.Slug("BoundedContext") }, "bounded-context"},
		{func() string { return gherkin.Slug("MessageSent") }, "message-sent"},
	}
	for _, tc := range cases {
		got := tc.fn()
		if got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}

func TestEmit_FileLayout(t *testing.T) {
	s := buildChatSession(t)
	outDir := t.TempDir()

	sum, err := gherkin.Emit(s, outDir)
	if err != nil {
		t.Fatal(err)
	}

	// Feature should be under messaging/message.feature
	if len(sum.FeatureFiles) > 0 {
		rel, _ := filepath.Rel(outDir, sum.FeatureFiles[0])
		want := filepath.Join("messaging", "message.feature")
		if rel != want {
			t.Errorf("feature path: got %q, want %q", rel, want)
		}
	}
}
