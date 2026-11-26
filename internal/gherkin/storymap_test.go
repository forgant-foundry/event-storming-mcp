package gherkin_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/forgant-foundry/forgant-discovery-mcp/internal/gherkin"
	"github.com/forgant-foundry/forgant-discovery-mcp/internal/session"
)

// buildFinanceMap is a small finant-api: an accountant records a transaction
// on the account screen, an approver approves a large transfer after the
// accountant submits it, and an operator starts the product off any screen.
func buildFinanceMap(t *testing.T) (*session.Session, map[string]string) {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, _ := session.New("Finance", "")
	id := s.ID
	ids := map[string]string{}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	_, record, _ := session.AddCommand(id, session.Command{Name: "RecordTransaction"})
	_, submit, _ := session.AddCommand(id, session.Command{Name: "SubmitTransfer"})
	_, approve, _ := session.AddCommand(id, session.Command{Name: "ApproveTransfer"})
	_, start, _ := session.AddCommand(id, session.Command{Name: "StartProduct"})
	session.AddEvent(id, session.Event{Name: "TransactionRecorded", TriggerCommandID: record.ID})
	session.AddEvent(id, session.Event{Name: "TransferSubmitted", TriggerCommandID: submit.ID})
	session.AddEvent(id, session.Event{Name: "TransferApproved", TriggerCommandID: approve.ID})
	session.AddEvent(id, session.Event{Name: "ProductStarted", TriggerCommandID: start.ID})
	_, register, _ := session.AddReadModel(id, "Account Register", "", nil, "", "")
	_, pending, _ := session.AddReadModel(id, "Pending Transfers", "", nil, "", "")
	_, web, _ := session.ProposeContext(id, "Web", "", nil, nil)

	_, acct, err := session.AddRole(id, session.Role{}, "Accountant", "")
	must(err)
	_, appr, _ := session.AddRole(id, session.Role{}, "Approver", "")
	_, op, _ := session.AddRole(id, session.Role{}, "Operator", "")

	_, account, err := session.AddScreen(id, session.Screen{Name: "Account", ContextID: web.ID, Shows: []string{register.ID},
		Offers: []session.Offer{
			{CommandID: record.ID, ShowsAfter: "the transaction in the account's register"},
			{CommandID: submit.ID, ShowsAfter: "the transfer as pending approval"},
		}})
	must(err)
	_, approvals, _ := session.AddScreen(id, session.Screen{Name: "Approvals", ContextID: web.ID, Shows: []string{pending.ID},
		Offers: []session.Offer{{CommandID: approve.ID, ShowsAfter: "the transfer as approved"}}})

	_, keep, _ := session.AddActivity(id, session.Activity{Name: "keep my records"}, "")
	_, run, _ := session.AddActivity(id, session.Activity{Name: "run the product"}, keep.ID)

	_, startStep, err := session.AddStep(id, session.Step{ActivityID: run.ID, RoleID: op.ID, Name: "start the product",
		CommandIDs: []string{start.ID}, OffScreen: "the operator's docker compose up"}, "")
	must(err)
	_, recordStep, _ := session.AddStep(id, session.Step{ActivityID: keep.ID, RoleID: acct.ID, Name: "record a transaction",
		CommandIDs: []string{record.ID}, ScreenID: account.ID,
		JourneyWhen: "the accountant records a transaction of -42.50 on the account screen",
		JourneyThen: "the account screen shows a balance of 957.50"}, "")
	_, submitStep, _ := session.AddStep(id, session.Step{ActivityID: keep.ID, RoleID: acct.ID, Name: "submit a large transfer",
		CommandIDs: []string{submit.ID}, ScreenID: account.ID}, "")
	_, approveStep, err := session.AddStep(id, session.Step{ActivityID: keep.ID, RoleID: appr.ID, Name: "approve the transfer",
		CommandIDs: []string{approve.ID}, ScreenID: approvals.ID, AfterStepID: submitStep.ID}, "")
	must(err)

	_, sk, _ := session.AddSlice(id, session.Slice{Name: "walking skeleton", Kind: session.SliceDevelopment, WalkingSkeleton: true,
		Outcome: "see a transaction recorded and a transfer approved end to end", Preconditions: []string{"The developer rung is running"}}, "")
	var storyIDs []string
	for _, st := range []struct {
		step, title, why string
		c                session.Criterion
	}{
		{startStep.ID, "start from a fresh clone", "the product runs where it's installed",
			session.Criterion{Text: "the product starts from a fresh clone", When: "the operator starts the product", Then: []string{"the sign-in screen is shown"}}},
		{recordStep.ID, "record a transaction", "my balance is right",
			session.Criterion{Text: "a transaction changes the balance by its amount",
				Given: []string{"An account with a balance of <opening>"}, When: "the accountant records a transaction of <amount>",
				Then: []string{"the account's balance is <closing>"},
				Examples: &session.Examples{Columns: []string{"opening", "amount", "closing"},
					Rows: [][]string{{"1000.00", "-42.50", "957.50"}, {"0.00", "10.00", "10.00"}}}}},
		{submitStep.ID, "submit a large transfer", "large transfers get a second pair of eyes",
			session.Criterion{Text: "a transfer over the limit waits for approval", When: "the accountant submits a transfer over the limit", Then: []string{"the transfer is pending approval"}}},
		{approveStep.ID, "approve a pending transfer", "the money moves once approved",
			session.Criterion{Text: "an approved transfer moves the money", When: "the approver approves the transfer", Then: []string{"the transfer is recorded"}}},
	} {
		_, story, err := session.AddStory(id, session.Story{StepID: st.step, Title: st.title, Why: st.why, Criteria: []session.Criterion{st.c}}, "")
		must(err)
		storyIDs = append(storyIDs, story.ID)
	}
	session.AssignToSlice(id, sk.ID, storyIDs)
	session.AddConstraint(id, session.Constraint{Text: "the register shows promptly", Kind: "performance", ScopeKind: session.KindScreen, ScopeID: account.ID,
		Measure: "the time to show the register", Target: "under 1 second for 95% of requests"})

	ids["slice"] = sk.ID
	ids["web"] = web.ID
	loaded, _ := session.Load(id)
	return loaded, ids
}

func TestEmitStoryMap(t *testing.T) {
	s, _ := buildFinanceMap(t)
	out := t.TempDir()
	sum, err := gherkin.EmitStoryMap(s, out, gherkin.StoryMapOptions{})
	if err != nil {
		t.Fatal(err)
	}
	parseFeatures(t, &gherkin.Summary{FeatureFiles: sum.FeatureFiles})
	if len(sum.Lint) != 0 {
		t.Errorf("unexpected lint: %+v", sum.Lint)
	}
	if len(sum.Slices) != 1 || !sum.Slices[0].Ready {
		t.Fatalf("the skeleton should be ready: %+v", sum.Slices)
	}

	read := func(rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(out, rel))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	journey := read("web/walking-skeleton.feature")
	for _, want := range []string{
		"@context:web @running @slice:walking-skeleton @walking-skeleton\nFeature: walking skeleton\n",
		// The accountant's journey: concrete text, and a derived step that names its screen.
		"  @role:accountant @journey\n  Scenario: the accountant's walking skeleton\n    Given the developer rung is running\n" +
			"    When the accountant records a transaction of -42.50 on the account screen\n" +
			"    Then the account screen shows a balance of 957.50\n" +
			"    When the accountant submits a transfer on the account screen\n" +
			"    Then the account screen shows the transfer as pending approval\n",
		// The approver's journey is staged by replaying the accountant's up to the hand-off.
		// Every other role's steps up to the hand-off, the operator's included.
		"  Scenario: the approver's walking skeleton\n    Given the developer rung is running\n" +
			"    And a product is started\n" +
			"    And the account screen shows a balance of 957.50\n    And the account screen shows the transfer as pending approval\n" +
			"    When the approver approves a transfer on the approvals screen\n",
		// The operator's steps are all off-screen: @running, not @journey.
		"  @role:operator\n  Scenario: the operator's walking skeleton\n",
	} {
		if !strings.Contains(journey, want) {
			t.Errorf("journey missing %q:\n%s", want, journey)
		}
	}

	screen := read("web/account-screen.feature")
	for _, want := range []string{
		"@context:web @screen:account-screen\nFeature: Account\n",
		"Then the account screen shows the account register\n",
		"  Scenario: accountant records a transaction on the account screen\n    Given the accountant is on the account screen\n" +
			"    When the accountant records a transaction on the account screen\n    Then a transaction is recorded\n" +
			"    And the account screen shows the transaction in the account's register\n",
	} {
		if !strings.Contains(screen, want) {
			t.Errorf("account screen missing %q:\n%s", want, screen)
		}
	}

	stories := read("web/keep-my-records.feature")
	for _, want := range []string{
		"  # Story: record a transaction, so that my balance is right\n",
		"  @story:record-a-transaction @slice:walking-skeleton\n  Scenario Outline: a transaction changes the balance by its amount\n" +
			"    Given an account with a balance of <opening>\n",
		"      | opening | amount | closing |\n      | 1000.00 | -42.50 | 957.50 |\n",
	} {
		if !strings.Contains(stories, want) {
			t.Errorf("stories missing %q:\n%s", want, stories)
		}
	}
	// The operator's story has no screen, so it goes under stories/.
	if !strings.Contains(read("stories/run-the-product.feature"), "Scenario: the product starts from a fresh clone") {
		t.Error("off-screen story should be emitted under stories/")
	}

	if c := read("constraints.feature"); !strings.Contains(c, "@constraint:performance @screen:account-screen\n  Scenario: the register shows promptly\n"+
		"    Then the time to show the register is under 1 second for 95% of requests\n") {
		t.Errorf("constraints:\n%s", c)
	}

	var slicesDoc struct {
		BuildOrder []string `json:"build_order"`
		Slices     []struct {
			Name  string
			Ready bool
			Files []string
		}
	}
	if err := json.Unmarshal([]byte(read("story-map/slices.json")), &slicesDoc); err != nil {
		t.Fatal(err)
	}
	if len(slicesDoc.Slices) != 1 || !slicesDoc.Slices[0].Ready || len(slicesDoc.Slices[0].Files) < 4 {
		t.Errorf("slices.json: %+v", slicesDoc)
	}
	if md := read("story-map/story-map.md"); !strings.Contains(md, "### walking skeleton (development): ready") ||
		!strings.Contains(md, "| start the product | Operator | off screen: the operator's docker compose up |") {
		t.Errorf("story-map.md:\n%s", md)
	}
}

func TestEmitStoryMap_Only(t *testing.T) {
	s, _ := buildFinanceMap(t)
	out := t.TempDir()
	sum, err := gherkin.EmitStoryMap(s, out, gherkin.StoryMapOptions{Only: []string{gherkin.OutputScreens}})
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.FeatureFiles) != 2 || sum.SlicesFile != "" {
		t.Errorf("only screens should be written: %+v", sum)
	}
	if _, err := gherkin.EmitStoryMap(s, out, gherkin.StoryMapOptions{Only: []string{"handoff"}}); err == nil {
		t.Error("expected an error for an unknown output")
	}
}
