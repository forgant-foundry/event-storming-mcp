package gherkin_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	cucumber "github.com/cucumber/gherkin/go/v42"

	"github.com/forgant-foundry/event-storming-mcp/internal/gherkin"
	"github.com/forgant-foundry/event-storming-mcp/internal/session"
)

// parseFeatures runs every emitted feature file through the reference Gherkin
// parser, so the emitter can't produce files a BDD runner would reject.
func parseFeatures(t *testing.T, sum *gherkin.Summary) {
	t.Helper()
	for _, path := range sum.FeatureFiles {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		_, err = cucumber.ParseGherkinDocument(f, func() string { n++; return fmt.Sprint(n) })
		f.Close()
		if err != nil {
			data, _ := os.ReadFile(path)
			t.Errorf("%s does not parse as Gherkin: %v\n\n%s", path, err, data)
		}
	}
}

// buildChatSession builds a minimal but realistic session: one aggregate in one
// bounded context, with an actor, a command, and a linked event.
func buildChatSession(t *testing.T) *session.Session {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	s, _ := session.New("Chat App", "Team messaging")

	_, actor, _ := session.AddActor(s.ID, "User", session.ActorTypePerson, "")
	_, cmd, _ := session.AddCommand(s.ID, session.Command{Name: "SendMessage", ActorID: actor.ID, Preconditions: []string{"user is a member of the channel"}})
	_, evt, _ := session.AddEvent(s.ID, session.Event{Name: "MessageSent", TriggerCommandID: cmd.ID})
	_, _, _ = session.AddEvent(s.ID, session.Event{Name: "NotificationSent", TriggerCommandID: cmd.ID})

	_, agg, _ := session.ClusterAggregate(s.ID, "Message", "",
		"", []string{evt.ID}, []string{cmd.ID})

	_, term, _ := session.DefineTerm(s.ID, "Message",
		"A text payload sent by a user into a channel.", "the message",
		"", []string{"msg", "post"}, nil)

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

	sum, err := gherkin.Emit(s, outDir, gherkin.Options{})
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

	sum, err := gherkin.Emit(s, outDir, gherkin.Options{})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(sum.FeatureFiles[0])
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	checks := []string{
		"@context:messaging",
		"@aggregate:message",
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

	sum, _ := gherkin.Emit(s, outDir, gherkin.Options{})
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

	sum, _ := gherkin.Emit(s, outDir, gherkin.Options{})
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
	_, evt, _ := session.AddEvent(s.ID, session.Event{Name: "ThingHappened"})
	_, cmd, _ := session.AddCommand(s.ID, session.Command{Name: "DoThing"})
	// Link event to command so a scenario is emitted.
	session.AddEvent(s.ID, session.Event{Name: "ThingHappened2", TriggerCommandID: cmd.ID})
	session.ClusterAggregate(s.ID, "Thing", "", "", []string{evt.ID}, []string{cmd.ID})

	loaded, _ := session.Load(s.ID)
	outDir := t.TempDir()
	sum, err := gherkin.Emit(loaded, outDir, gherkin.Options{})
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
	_, cmd, _ := session.AddCommand(s.ID, session.Command{Name: "DoSomething", ActorID: actor.ID})
	_, evt, _ := session.AddEvent(s.ID, session.Event{Name: "SomethingDone"}) // NOT linked to cmd
	session.ClusterAggregate(s.ID, "Thing", "", "", []string{evt.ID}, []string{cmd.ID})

	loaded, _ := session.Load(s.ID)
	outDir := t.TempDir()
	sum, err := gherkin.Emit(loaded, outDir, gherkin.Options{})
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
	_, evt, _ := session.AddEvent(s.ID, session.Event{Name: "PeriodClosed"})
	_, agg, _ := session.ClusterAggregate(s.ID, "", "", "", []string{evt.ID}, nil)
	session.DeprecateAggregate(s.ID, agg.ID, "snapshots made this redundant")

	loaded, _ := session.Load(s.ID)
	outDir := t.TempDir()
	sum, err := gherkin.Emit(loaded, outDir, gherkin.Options{})
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
	_, cmd, _ := session.AddCommand(s.ID, session.Command{Name: "ProcessPayment", ActorID: actor.ID})
	_, evtOk, _ := session.AddEvent(s.ID, session.Event{Name: "PaymentProcessed", TriggerCommandID: cmd.ID})
	_, evtFail, _ := session.AddEvent(s.ID, session.Event{Name: "PaymentDeclined", TriggerCommandID: cmd.ID})
	session.SetEventCondition(s.ID, evtFail.ID, "the payment method is invalid")
	session.ClusterAggregate(s.ID, "Payment", "", "", []string{evtOk.ID, evtFail.ID}, []string{cmd.ID})

	loaded, _ := session.Load(s.ID)
	outDir := t.TempDir()
	sum, err := gherkin.Emit(loaded, outDir, gherkin.Options{})
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(sum.FeatureFiles[0])
	content := string(data)

	if sum.ScenarioCount != 2 {
		t.Errorf("expected 2 scenarios (happy path + conditional), got %d", sum.ScenarioCount)
	}
	if !strings.Contains(content, "    Given the payment method is invalid\n") {
		t.Errorf("condition should open with Given when the command has no preconditions:\n%s", content)
	}
}

func TestEmit_ConditionalEventWithPreconditions(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Test", "")
	_, actor, _ := session.AddActor(s.ID, "User", session.ActorTypePerson, "")
	_, cmd, _ := session.AddCommand(s.ID, session.Command{Name: "ProcessPayment", ActorID: actor.ID, Preconditions: []string{"an order is awaiting payment"}})
	_, evtOk, _ := session.AddEvent(s.ID, session.Event{Name: "PaymentProcessed", TriggerCommandID: cmd.ID})
	_, evtFail, _ := session.AddEvent(s.ID, session.Event{Name: "PaymentDeclined", TriggerCommandID: cmd.ID})
	session.SetEventCondition(s.ID, evtFail.ID, "the payment method is invalid")
	session.ClusterAggregate(s.ID, "Payment", "", "", []string{evtOk.ID, evtFail.ID}, []string{cmd.ID})

	loaded, _ := session.Load(s.ID)
	sum, err := gherkin.Emit(loaded, t.TempDir(), gherkin.Options{})
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(sum.FeatureFiles[0])
	content := string(data)

	want := "    Given an order is awaiting payment\n    And the payment method is invalid\n"
	if !strings.Contains(content, want) {
		t.Errorf("condition should follow preconditions with And:\n%s", content)
	}
}

func TestSlug_SpaceSeparated(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Ledger Management", "ledger-management"},
		{"BoundedContext", "bounded-context"},
		{"Order", "order"},
		{"ledger management", "ledger-management"},
		{"Ledger_Management", "ledger-management"},
		{"Whenever ScopeValidated, then StartFactoryRun", "whenever-scope-validated-then-start-factory-run"},
		{"Order (v2)", "order-v2"},
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
		// Participle followed by a prepositional phrase.
		{func() string { return gherkin.ThenStep("ScopePassedToFactory") }, "a scope is passed to factory"},
		{func() string { return gherkin.ThenStep("AppropriateCompletenessArrivedAt") }, "an appropriate completeness is arrived at"},
		{func() string { return gherkin.ThenStep("SpecificationFoundWrongOrImpossible") }, "a specification is found wrong or impossible"},
		// Irregular participle at the end; a leading irregular word is a noun.
		{func() string { return gherkin.ThenStep("BudgetSet") }, "a budget is set"},
		{func() string { return gherkin.ThenStep("RunStarted") }, "a run is started"},
		// A name that opens with its participle has no subject.
		{func() string { return gherkin.ThenStep("MergedToMain") }, "merged to main"},
		// Plurals take no article and "are"; -ss, -us and -is aren't plurals.
		{func() string { return gherkin.ThenStep("SurfacesPrioritized") }, "surfaces are prioritized"},
		{func() string { return gherkin.ThenStep("StatusChanged") }, "a status is changed"},
		{func() string { return gherkin.WhenStep("PrioritizeSurfaces", "Product Manager") }, "the product manager prioritizes surfaces"},
		{func() string { return gherkin.WhenStep("RunAnalysis", "") }, "the system runs an analysis"},
		// An object opening with a preposition takes no article.
		{func() string { return gherkin.WhenStep("MergeToMain", "Director") }, "the director merges to main"},
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

	sum, err := gherkin.Emit(s, outDir, gherkin.Options{})
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

// TestEmit_ParsesAsGherkin covers every emitter path with names that contain
// spaces, which Gherkin tags may not.
func TestEmit_ParsesAsGherkin(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Test", "")
	_, actor, _ := session.AddActor(s.ID, "Cell Agent", session.ActorTypeRole, "")

	// Happy path with two events, plus conditional events with and without preconditions.
	_, rate, _ := session.AddCommand(s.ID, session.Command{Name: "RateThreat", ActorID: actor.ID, Preconditions: []string{"a threat is identified"}})
	_, rated, _ := session.AddEvent(s.ID, session.Event{Name: "ThreatRated", TriggerCommandID: rate.ID})
	_, logged, _ := session.AddEvent(s.ID, session.Event{Name: "RatingLogged", TriggerCommandID: rate.ID})
	_, above, _ := session.AddEvent(s.ID, session.Event{Name: "ThreatEscalated", TriggerCommandID: rate.ID})
	session.SetEventCondition(s.ID, above.ID, "the threat is above the bug bar")
	_, start, _ := session.AddCommand(s.ID, session.Command{Name: "StartReview", ActorID: actor.ID, Source: session.SourceFacilitator})
	_, fresh, _ := session.AddEvent(s.ID, session.Event{Name: "FreshReviewStarted", TriggerCommandID: start.ID})
	_, cont, _ := session.AddEvent(s.ID, session.Event{Name: "ReviewResumed", TriggerCommandID: start.ID})
	session.SetEventCondition(s.ID, fresh.ID, "no review exists")
	session.SetEventCondition(s.ID, cont.ID, "a review exists")
	_, agg, _ := session.ClusterAggregate(s.ID, "Threat Register", "Rule: every threat is rated against the bug bar", "",
		[]string{rated.ID, logged.ID, above.ID, fresh.ID, cont.ID}, []string{rate.ID, start.ID})

	// Stub aggregate (events, no commands) and a deprecated aggregate.
	_, e1, _ := session.AddEvent(s.ID, session.Event{Name: "BudgetSet"})
	_, e2, _ := session.AddEvent(s.ID, session.Event{Name: "BudgetRaised"})
	_, stub, _ := session.ClusterAggregate(s.ID, "Budget Envelope", "", "", []string{e1.ID, e2.ID}, nil)
	_, e3, _ := session.AddEvent(s.ID, session.Event{Name: "LegacyThing"})
	_, dep, _ := session.ClusterAggregate(s.ID, "Old Ledger", "", "", []string{e3.ID}, nil)
	session.DeprecateAggregate(s.ID, dep.ID, "superseded")

	session.ProposeContext(s.ID, "Cell Judgment", "", []string{agg.ID, stub.ID, dep.ID}, nil)

	loaded, _ := session.Load(s.ID)
	sum, err := gherkin.Emit(loaded, t.TempDir(), gherkin.Options{})
	if err != nil {
		t.Fatal(err)
	}
	parseFeatures(t, sum)

	data, _ := os.ReadFile(filepath.Join(filepath.Dir(sum.FeatureFiles[0]), "threat-register.feature"))
	content := string(data)
	if !strings.HasPrefix(content, "@context:cell-judgment @aggregate:threat-register\n") {
		t.Errorf("tags should be slugged:\n%s", content)
	}
	if !strings.Contains(content, "  @proposed\n  Scenario: cell agent starts a review (no review exists)\n") {
		t.Errorf("facilitator-sourced scenarios should be tagged @proposed:\n%s", content)
	}
	if strings.Count(content, "@proposed") != 2 {
		t.Errorf("only StartReview's two scenarios should be @proposed:\n%s", content)
	}
	if strings.Contains(content, "And  ") {
		t.Errorf("And should be followed by a single space:\n%s", content)
	}
}

func TestEmit_ConditionalOnlyWarnings(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Test", "")

	// Two conditional outcomes and no unconditional one: a branch, no warning.
	_, branch, _ := session.AddCommand(s.ID, session.Command{Name: "StartEventStorming"})
	_, empty, _ := session.AddEvent(s.ID, session.Event{Name: "GreenfieldStarted", TriggerCommandID: branch.ID})
	_, existing, _ := session.AddEvent(s.ID, session.Event{Name: "BrownfieldStarted", TriggerCommandID: branch.ID})
	session.SetEventCondition(s.ID, empty.ID, "the target repo has no code")
	session.SetEventCondition(s.ID, existing.ID, "the target repo has existing code")

	// One conditional outcome and no happy path: a gap, warned.
	_, gap, _ := session.AddCommand(s.ID, session.Command{Name: "ApproveScope"})
	_, rejected, _ := session.AddEvent(s.ID, session.Event{Name: "ScopeRejected", TriggerCommandID: gap.ID})
	session.SetEventCondition(s.ID, rejected.ID, "the scope is incomplete")

	session.ClusterAggregate(s.ID, "Scope", "", "",
		[]string{empty.ID, existing.ID, rejected.ID}, []string{branch.ID, gap.ID})

	loaded, _ := session.Load(s.ID)
	sum, err := gherkin.Emit(loaded, t.TempDir(), gherkin.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if sum.ScenarioCount != 3 {
		t.Errorf("expected 3 scenarios, got %d", sum.ScenarioCount)
	}
	if len(sum.Warnings) != 1 || !strings.Contains(sum.Warnings[0], "ApproveScope") {
		t.Errorf("expected exactly one warning, for ApproveScope; got %v", sum.Warnings)
	}
}

// buildFactorySession has a policy-heavy context with phrasing overrides,
// open and resolved hot spots, and a shared read model.
func buildFactorySession(t *testing.T) *session.Session {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Factory", "")
	_, director, _ := session.AddActor(s.ID, "Director", session.ActorTypeRole, "")
	_, cell, _ := session.AddActor(s.ID, "Cell Agent", session.ActorTypeRole, "")

	_, assess, _ := session.AddCommand(s.ID, session.Command{Name: "AssessCompleteness", ActorID: director.ID})
	_, assessed, _ := session.AddEvent(s.ID, session.Event{Name: "CompletenessAssessed", TriggerCommandID: assess.ID})
	_, gap, _ := session.AddEvent(s.ID, session.Event{Name: "CompletenessGapFound", TriggerCommandID: assess.ID})
	session.SetEventCondition(s.ID, gap.ID, "a surface is not covered")
	_, convene, _ := session.AddCommand(s.ID, session.Command{Name: "ConveneCells", ActorID: director.ID})
	_, convened, _ := session.AddEvent(s.ID, session.Event{Name: "CellsConvened", TriggerCommandID: convene.ID})
	_, declare, _ := session.AddCommand(s.ID, session.Command{Name: "DeclareArrival", ActorID: director.ID,
		Phrasing: "the director declares that the factory has arrived"})
	_, arrived, _ := session.AddEvent(s.ID, session.Event{Name: "AppropriateCompletenessArrivedAt", TriggerCommandID: declare.ID,
		Phrasing: "appropriate completeness has been reached"})
	_, agg, _ := session.ClusterAggregate(s.ID, "Completeness Ledger", "", "",
		[]string{assessed.ID, gap.ID, convened.ID, arrived.ID}, []string{assess.ID, convene.ID, declare.ID})
	_, ctx, _ := session.ProposeContext(s.ID, "Cell Judgment", "", []string{agg.ID}, nil)

	// No context_id: resolved from the resulting command's aggregate.
	session.AddPolicy(s.ID, "Convene on gap", "", gap.ID, convene.ID, "", "", "")
	session.AddPolicy(s.ID, "Declare when complete", "", assessed.ID, declare.ID, ctx.ID, "the assessment found no gaps", session.SourceFacilitator)
	session.AddPolicy(s.ID, "Dangling", "", assessed.ID, "", ctx.ID, "", "")

	session.AddReadModel(s.ID, "Completeness Ledger View", "", []string{director.ID, cell.ID}, ctx.ID, "")
	session.AddHotSpot(s.ID, "Who decides a surface is covered?", ctx.ID)
	_, closed, _ := session.AddHotSpot(s.ID, "Is the budget per run?", ctx.ID)
	session.ResolveHotSpot(s.ID, closed.ID, "Yes, per run")

	loaded, err := session.Load(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	return loaded
}

func TestEmit_PoliciesHotSpotsAndPhrasing(t *testing.T) {
	s := buildFactorySession(t)
	outDir := t.TempDir()
	sum, err := gherkin.Emit(s, outDir, gherkin.Options{})
	if err != nil {
		t.Fatal(err)
	}
	parseFeatures(t, sum)

	if sum.OpenHotSpots != 1 {
		t.Errorf("open hot spots: got %d, want 1", sum.OpenHotSpots)
	}
	// 3 command scenarios + 1 conditional + 2 complete policies.
	if sum.ScenarioCount != 6 {
		t.Errorf("scenario count: got %d, want 6", sum.ScenarioCount)
	}
	if len(sum.Warnings) != 1 || !strings.Contains(sum.Warnings[0], `policy "Dangling"`) {
		t.Errorf("expected one warning, for the dangling policy; got %v", sum.Warnings)
	}

	policies, err := os.ReadFile(filepath.Join(outDir, "cell-judgment", "policies.feature"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"@context:cell-judgment\nFeature: Cell Judgment policies\n",
		"  @policy:convene-on-gap\n  Scenario: Convene on gap\n    When a completeness gap is found\n    Then the director convenes cells\n",
		"  @policy:declare-when-complete @proposed\n",
		"    Given the assessment found no gaps\n    When a completeness is assessed\n    Then the director declares that the factory has arrived\n",
	} {
		if !strings.Contains(string(policies), want) {
			t.Errorf("policies.feature missing %q:\n%s", want, policies)
		}
	}

	ledger, _ := os.ReadFile(filepath.Join(outDir, "cell-judgment", "completeness-ledger.feature"))
	for _, want := range []string{
		"# Open question (",
		"Who decides a surface is covered?",
		"  Scenario: the director declares that the factory has arrived\n",
		"    When the director declares that the factory has arrived\n    Then appropriate completeness has been reached\n",
	} {
		if !strings.Contains(string(ledger), want) {
			t.Errorf("completeness-ledger.feature missing %q:\n%s", want, ledger)
		}
	}
	if strings.Contains(string(ledger), "Is the budget per run?") {
		t.Error("resolved hot spots should not be emitted as open questions")
	}

	var cm struct {
		Actors          []struct{ Name string }
		BoundedContexts []struct {
			Policies []struct {
				Name         string
				TriggerEvent string `json:"trigger_event"`
				Condition    string
			}
			ReadModels []struct{ Readers []string } `json:"read_models"`
			HotSpots   []struct{ Resolved bool }    `json:"hot_spots"`
		} `json:"bounded_contexts"`
	}
	raw, _ := os.ReadFile(sum.ContextMapFile)
	if err := json.Unmarshal(raw, &cm); err != nil {
		t.Fatal(err)
	}
	if len(cm.Actors) != 2 {
		t.Errorf("context map actors: got %d", len(cm.Actors))
	}
	c := cm.BoundedContexts[0]
	if len(c.Policies) != 3 || len(c.HotSpots) != 2 {
		t.Errorf("context map policies %d, hot spots %d", len(c.Policies), len(c.HotSpots))
	}
	if len(c.ReadModels) != 1 || !slices.Equal(c.ReadModels[0].Readers, []string{"Director", "Cell Agent"}) {
		t.Errorf("context map read models: %+v", c.ReadModels)
	}
}

func TestEmit_Only(t *testing.T) {
	s := buildFactorySession(t)
	outDir := t.TempDir()

	featurePath := filepath.Join(outDir, "cell-judgment", "completeness-ledger.feature")
	os.MkdirAll(filepath.Dir(featurePath), 0o755)
	os.WriteFile(featurePath, []byte("hand-written\n"), 0o644)

	sum, err := gherkin.Emit(s, outDir, gherkin.Options{Only: []string{gherkin.OutputGlossary, gherkin.OutputContextMap}})
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.FeatureFiles) != 0 || sum.ScenarioCount != 0 {
		t.Errorf("features should not be emitted: %+v", sum)
	}
	if sum.GlossaryFile == "" || sum.ContextMapFile == "" {
		t.Errorf("glossary and context map should be emitted: %+v", sum)
	}
	if data, _ := os.ReadFile(featurePath); string(data) != "hand-written\n" {
		t.Errorf("hand-written feature was overwritten:\n%s", data)
	}

	if _, err := gherkin.Emit(s, outDir, gherkin.Options{Only: []string{"specs"}}); err == nil {
		t.Error("expected error for unknown output")
	}
}

func TestEmit_GlossarySharedWith(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Factory", "")
	_, scoping, _ := session.ProposeContext(s.ID, "Scoping", "", nil, nil)
	_, judgment, _ := session.ProposeContext(s.ID, "Cell Judgment", "", nil, nil)
	session.DefineTerm(s.ID, "Bug Bar", "The severity threshold.", "the bug bar", scoping.ID, nil, []string{judgment.ID})

	loaded, _ := session.Load(s.ID)
	sum, err := gherkin.Emit(loaded, t.TempDir(), gherkin.Options{})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(sum.GlossaryFile)
	if !strings.Contains(string(data), "| Bug Bar | The severity threshold. | the bug bar |  | Cell Judgment |") {
		t.Errorf("glossary should list shared contexts:\n%s", data)
	}
}
