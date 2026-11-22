package session_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/forgant-foundry/event-storming-mcp/internal/session"
)

// ledgerMap is a small storm plus story map: an accountant who opens a ledger
// and records transactions on a screen, and an approver who approves large
// transfers after the accountant submits them.
type ledgerMap struct {
	s                                 *session.Session
	open, record, submit, approve     *session.Command
	opened, recorded, submitted       *session.Event
	balance                           *session.ReadModel
	accountant, approver              *session.Role
	manage                            *session.Activity
	openStep, recordStep, approveStep *session.Step
	screen                            *session.Screen
}

func buildLedgerMap(t *testing.T) ledgerMap {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	var l ledgerMap
	l.s, _ = session.New("Ledger", "")
	id := l.s.ID
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var err error
	_, l.open, err = session.AddCommand(id, session.Command{Name: "OpenLedger"})
	must(err)
	_, l.record, _ = session.AddCommand(id, session.Command{Name: "RecordTransaction"})
	_, l.submit, _ = session.AddCommand(id, session.Command{Name: "SubmitTransfer"})
	_, l.approve, _ = session.AddCommand(id, session.Command{Name: "ApproveTransfer"})
	_, l.opened, _ = session.AddEvent(id, session.Event{Name: "LedgerOpened", TriggerCommandID: l.open.ID})
	_, l.recorded, _ = session.AddEvent(id, session.Event{Name: "TransactionRecorded", TriggerCommandID: l.record.ID})
	_, l.submitted, _ = session.AddEvent(id, session.Event{Name: "TransferSubmitted", TriggerCommandID: l.submit.ID})
	_, l.balance, _ = session.AddReadModel(id, "Account Balance", "", nil, "", "")

	_, l.accountant, err = session.AddRole(id, session.Role{Goal: "keep the books straight"}, "Accountant", session.ActorTypeRole)
	must(err)
	_, l.approver, err = session.AddRole(id, session.Role{}, "Approver", "")
	must(err)
	_, l.manage, err = session.AddActivity(id, session.Activity{Name: "manage my ledgers", RoleID: l.accountant.ID}, "")
	must(err)
	_, l.screen, err = session.AddScreen(id, session.Screen{Name: "Transactions", Shows: []string{l.balance.ID},
		Offers: []session.Offer{{CommandID: l.record.ID, ShowsAfter: "the new balance"}}})
	must(err)
	_, l.recordStep, err = session.AddStep(id, session.Step{ActivityID: l.manage.ID, RoleID: l.accountant.ID,
		Name: "record a transaction", CommandIDs: []string{l.record.ID}, ScreenID: l.screen.ID}, "")
	must(err)
	// Added second but placed first: narrative order is list order.
	_, l.openStep, err = session.AddStep(id, session.Step{ActivityID: l.manage.ID, RoleID: l.accountant.ID,
		Name: "open a ledger", CommandIDs: []string{l.open.ID}}, l.recordStep.ID)
	must(err)
	submitStep := session.Step{ActivityID: l.manage.ID, RoleID: l.accountant.ID, Name: "submit a large transfer", CommandIDs: []string{l.submit.ID}}
	_, sub, err := session.AddStep(id, submitStep, "")
	must(err)
	_, l.approveStep, err = session.AddStep(id, session.Step{ActivityID: l.manage.ID, RoleID: l.approver.ID,
		Name: "approve the transfer", CommandIDs: []string{l.approve.ID}, AfterStepID: sub.ID}, "")
	must(err)
	return l
}

func TestStoryMap_Capture(t *testing.T) {
	l := buildLedgerMap(t)
	loaded, _ := session.Load(l.s.ID)
	m := loaded.StoryMap

	// AddRole created the actors, defaulting the type to role.
	if len(loaded.Actors) != 2 || loaded.Actors[1].Type != session.ActorTypeRole || m.Roles[0].ActorID != loaded.Actors[0].ID {
		t.Errorf("roles should create their actors: %+v / %+v", loaded.Actors, m.Roles)
	}
	var names []string
	for _, st := range m.Steps {
		names = append(names, st.Name)
	}
	if !slices.Equal(names, []string{"open a ledger", "record a transaction", "submit a large transfer", "approve the transfer"}) {
		t.Errorf("steps out of narrative order: %v", names)
	}
}

func TestStoryMap_Validation(t *testing.T) {
	l := buildLedgerMap(t)
	id := l.s.ID
	cases := map[string]func() error{
		"second role for an actor": func() error {
			_, _, err := session.AddRole(id, session.Role{ActorID: l.accountant.ActorID}, "", "")
			return err
		},
		"role for a new actor whose name exists": func() error {
			_, _, err := session.AddRole(id, session.Role{}, "Accountant", "")
			return err
		},
		"step with an unknown command": func() error {
			_, _, err := session.AddStep(id, session.Step{ActivityID: l.manage.ID, RoleID: l.accountant.ID, Name: "x", CommandIDs: []string{"cmd_gone"}}, "")
			return err
		},
		"step with no role": func() error {
			_, _, err := session.AddStep(id, session.Step{ActivityID: l.manage.ID, Name: "x"}, "")
			return err
		},
		"hand-off to the same role": func() error {
			_, _, err := session.AddStep(id, session.Step{ActivityID: l.manage.ID, RoleID: l.accountant.ID, Name: "x", AfterStepID: l.openStep.ID}, "")
			return err
		},
		"story with an unknown kind": func() error {
			_, _, err := session.AddStory(id, session.Story{StepID: l.openStep.ID, Title: "x", Kind: "epic"}, "")
			return err
		},
		"walking skeleton that isn't a development slice": func() error {
			_, _, err := session.AddSlice(id, session.Slice{Name: "x", Kind: session.SliceRelease, WalkingSkeleton: true}, "")
			return err
		},
		"screen offering a command twice": func() error {
			_, _, err := session.AddScreen(id, session.Screen{Name: "Twice", Offers: []session.Offer{{CommandID: l.open.ID}, {CommandID: l.open.ID}}})
			return err
		},
		"duplicate screen name": func() error {
			_, _, err := session.AddScreen(id, session.Screen{Name: "Transactions"})
			return err
		},
	}
	for name, f := range cases {
		if f() == nil {
			t.Errorf("%s: expected an error", name)
		}
	}

	_, sk, err := session.AddSlice(id, session.Slice{Name: "walking skeleton", Kind: session.SliceDevelopment, WalkingSkeleton: true}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := session.AddSlice(id, session.Slice{Name: "another", Kind: session.SliceDevelopment, WalkingSkeleton: true}, ""); err == nil {
		t.Error("expected an error for a second walking skeleton")
	}
	if _, _, err := session.UpdateSlice(id, sk.ID, session.SlicePatch{Outcome: ptr("see a month's spending end to end")}); err != nil {
		t.Errorf("updating the skeleton itself shouldn't conflict with itself: %v", err)
	}
}

func TestStoryMap_StoriesSlicesAndMoves(t *testing.T) {
	l := buildLedgerMap(t)
	id := l.s.ID
	_, a, _ := session.AddStory(id, session.Story{StepID: l.recordStep.ID, Title: "record a simple transaction"}, "")
	_, b, _ := session.AddStory(id, session.Story{StepID: l.recordStep.ID, Title: "the account is closed", Kind: session.StoryFailure}, "")
	_, other, _ := session.AddStory(id, session.Story{StepID: l.openStep.ID, Title: "open my first ledger"}, "")
	if a.Kind != session.StoryDetail {
		t.Errorf("kind should default to detail, got %q", a.Kind)
	}
	if _, _, err := session.AddStory(id, session.Story{StepID: l.recordStep.ID, Title: "x"}, other.ID); err == nil {
		t.Error("expected an error ranking a story among another step's stories")
	}
	if _, err := session.MoveItem(id, session.KindStory, b.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	_, rel, _ := session.AddSlice(id, session.Slice{Name: "first release", Kind: session.SliceRelease}, "")
	if _, _, err := session.AssignToSlice(id, rel.ID, []string{a.ID, b.ID}); err != nil {
		t.Fatal(err)
	}

	loaded, _ := session.Load(id)
	m := loaded.StoryMap
	if m.Stories[0].ID != b.ID || m.Stories[1].ID != a.ID {
		t.Errorf("move should rank the failure story first: %v, %v", m.Stories[0].Title, m.Stories[1].Title)
	}
	if m.Stories[0].SliceID != rel.ID || m.Stories[2].SliceID != "" {
		t.Errorf("slice assignment: %+v", m.Stories)
	}
}

func TestStoryMap_RemoveRules(t *testing.T) {
	l := buildLedgerMap(t)
	id := l.s.ID
	_, st, _ := session.AddStory(id, session.Story{StepID: l.recordStep.ID, Title: "record a transaction"}, "")
	_, sl, _ := session.AddSlice(id, session.Slice{Name: "first", Kind: session.SliceRelease}, "")
	session.AssignToSlice(id, sl.ID, []string{st.ID})

	if _, _, err := session.RemoveItem(id, session.KindActivity, l.manage.ID); err == nil || !strings.Contains(err.Error(), "step(s)") {
		t.Errorf("removing an activity with steps should be refused, got %v", err)
	}
	if _, _, err := session.RemoveItem(id, session.KindStep, l.recordStep.ID); err == nil {
		t.Error("removing a step with stories should be refused")
	}
	_, detached, err := session.RemoveItem(id, session.KindSlice, sl.ID)
	if err != nil || len(detached) != 1 {
		t.Errorf("removing a slice should clear its stories' slice_id: %v %v", detached, err)
	}
	_, detached, _ = session.RemoveItem(id, session.KindScreen, l.screen.ID)
	if len(detached) != 1 || !strings.Contains(detached[0], "screen_id") {
		t.Errorf("removing a screen should clear its steps' screen_id: %v", detached)
	}
}

func TestStoryMap_StormCascades(t *testing.T) {
	l := buildLedgerMap(t)
	id := l.s.ID
	_, story, _ := session.AddStory(id, session.Story{StepID: l.recordStep.ID, Title: "x", Kind: session.StoryFailure, EventID: l.recorded.ID}, "")

	_, detached, _ := session.RemoveItem(id, session.KindCommand, l.record.ID)
	joined := strings.Join(detached, "\n")
	if !strings.Contains(joined, "removed from command_ids") || !strings.Contains(joined, "no longer offers it") {
		t.Errorf("removing a command should detach it from steps and screens:\n%s", joined)
	}
	session.RemoveItem(id, session.KindEvent, l.recorded.ID)
	session.RemoveItem(id, session.KindReadModel, l.balance.ID)
	_, detached, _ = session.RemoveItem(id, session.KindActor, l.approver.ActorID)
	if !strings.Contains(strings.Join(detached, "\n"), "removed with its actor") {
		t.Errorf("removing an actor should remove its role: %v", detached)
	}

	loaded, _ := session.Load(id)
	m := loaded.StoryMap
	if len(m.Roles) != 1 || m.Steps[3].RoleID != "" {
		t.Errorf("approver role and its steps' role_id should be gone: %+v", m.Steps[3])
	}
	if len(m.Screens[0].Shows) != 0 || len(m.Screens[0].Offers) != 0 {
		t.Errorf("screen should lose the removed read model and command: %+v", m.Screens[0])
	}
	if i := slices.IndexFunc(m.Stories, func(s session.Story) bool { return s.ID == story.ID }); m.Stories[i].EventID != "" {
		t.Errorf("story should lose the removed event")
	}
}

func TestCheckLinks(t *testing.T) {
	l := buildLedgerMap(t)
	id := l.s.ID
	// A step with no storm behind it, a screen offering a command with no
	// events, a step on the screen using a command it doesn't offer, and an
	// alias in a step name.
	_, gap, _ := session.AddStep(id, session.Step{ActivityID: l.manage.ID, RoleID: l.accountant.ID, Name: "export the books"}, "")
	_, sc, _ := session.AddScreen(id, session.Screen{Name: "Approvals", Offers: []session.Offer{{CommandID: l.approve.ID}}})
	session.UpdateStep(id, l.openStep.ID, session.StepPatch{ScreenID: &sc.ID})
	session.DefineTerm(id, "Ledger", "A set of books.", "the ledger", "", []string{"books"}, nil)
	_, idle, _ := session.AddRole(id, session.Role{}, "Auditor", "")

	loaded, _ := session.Load(id)
	findings := session.CheckLinks(loaded)
	has := func(sev, kind, rid, substr string) bool {
		return slices.ContainsFunc(findings, func(f session.LinkFinding) bool {
			return f.Severity == sev && f.Kind == kind && f.ID == rid && strings.Contains(f.Message, substr)
		})
	}
	checks := []struct {
		ok   bool
		what string
	}{
		{has(session.SeverityWarning, session.KindStep, gap.ID, "uses no command"), "gap step"},
		{has(session.SeverityWarning, session.KindScreen, sc.ID, "ApproveTransfer, which has no linked events"), "dead control"},
		{has(session.SeverityWarning, session.KindStep, l.openStep.ID, "doesn't offer"), "step using an unoffered command"},
		{has(session.SeverityNote, session.KindReadModel, l.balance.ID, "") == false, "balance is shown, so no note"},
		{has(session.SeverityNote, session.KindRole, idle.ID, `"Auditor" has no steps`), "idle role"},
		{has(session.SeverityLint, session.KindStep, gap.ID, `alias "books"`), "alias in a step name"},
	}
	for _, c := range checks {
		if !c.ok {
			t.Errorf("%s: not found in\n%+v", c.what, findings)
		}
	}
	for _, f := range findings {
		if f.Kind == session.KindCommand {
			t.Errorf("every command is used by a step; unexpected %+v", f)
		}
	}
}
