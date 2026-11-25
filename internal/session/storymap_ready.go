package session

import (
	"fmt"
	"slices"
	"strings"
)

// OrderedSteps returns the map's steps in narrative order: activities left to
// right, and each activity's steps in their order.
func (m *StoryMap) OrderedSteps() []Step {
	var out []Step
	for _, a := range m.Activities {
		for _, st := range m.Steps {
			if st.ActivityID == a.ID {
				out = append(out, st)
			}
		}
	}
	return out
}

// SliceSteps returns, in narrative order, the steps with at least one story
// in the slice.
func (m *StoryMap) SliceSteps(sliceID string) []Step {
	in := make(map[string]bool)
	for _, st := range m.Stories {
		if st.SliceID == sliceID {
			in[st.StepID] = true
		}
	}
	var out []Step
	for _, st := range m.OrderedSteps() {
		if in[st.ID] {
			out = append(out, st)
		}
	}
	return out
}

// ScreenPhrase is how steps refer to a screen: "Account" → "the account
// screen", "Sign-in Screen" → "the sign-in screen".
func ScreenPhrase(name string) string {
	p := strings.ToLower(name)
	if !strings.HasSuffix(p, "screen") {
		p += " screen"
	}
	return "the " + p
}

// SliceReadiness says whether a slice is ready for a factory, and why not.
type SliceReadiness struct {
	SliceID         string    `json:"slice_id"`
	Name            string    `json:"name"`
	Kind            SliceKind `json:"kind"`
	WalkingSkeleton bool      `json:"walking_skeleton,omitempty"`
	Ready           bool      `json:"ready"`
	Blocking        []string  `json:"blocking,omitempty"`
	Notes           []string  `json:"notes,omitempty"`
}

// uiWords are mechanics that belong in a test's driver, not the spec
// (Continuous Delivery ch. 8; Cohn ch. 14, "including user interface details
// too soon").
var uiWords = []string{"click", "clicks", "button", "buttons", "field", "fields", "dropdown", "checkbox", "tap", "taps", "textbox"}

// layerWords suggest a story split along technical layers rather than what a
// person gets (Cohn ch. 7, "slice the cake").
var layerWords = []string{"database", "db", "api", "endpoint", "table", "schema", "form", "backend", "frontend"}

func containsWord(text string, words []string) string {
	toks := tokenize(text)
	for _, w := range words {
		if slices.Contains(toks, w) {
			return w
		}
	}
	return ""
}

// Readiness checks every slice, top to bottom. A slice is ready when nothing
// blocks it; notes are questions worth an answer but don't block.
func Readiness(s *Session) []SliceReadiness {
	m := s.StoryMap
	if m == nil {
		return nil
	}
	cmd := make(map[string]Command)
	for _, c := range s.Commands {
		cmd[c.ID] = c
	}
	rmName := make(map[string]string)
	for _, r := range s.ReadModels {
		rmName[r.ID] = r.Name
	}
	hasEvents := make(map[string]bool)
	for _, e := range s.Events {
		hasEvents[e.TriggerCommandID] = true
	}
	screen := make(map[string]Screen)
	for _, sc := range m.Screens {
		screen[sc.ID] = sc
	}

	// Commands any slice's journey issues, for "offered but no journey uses it".
	journeyCommands := make(map[string]bool)
	for _, sl := range m.Slices {
		for _, st := range m.SliceSteps(sl.ID) {
			for _, c := range st.CommandIDs {
				journeyCommands[c] = true
			}
		}
	}

	var out []SliceReadiness
	for _, sl := range m.Slices {
		r := SliceReadiness{SliceID: sl.ID, Name: sl.Name, Kind: sl.Kind, WalkingSkeleton: sl.WalkingSkeleton}
		block := func(format string, args ...any) { r.Blocking = append(r.Blocking, fmt.Sprintf(format, args...)) }
		note := func(format string, args ...any) { r.Notes = append(r.Notes, fmt.Sprintf(format, args...)) }
		lint := func(where, text string) {
			for _, issue := range LintStep(s, "", text) {
				block("%s %q: %s", where, text, issue.Message)
			}
		}

		steps := m.SliceSteps(sl.ID)
		if len(steps) == 0 {
			block("the slice has no stories")
		}
		inSlice := make(map[string]bool)
		for _, st := range steps {
			inSlice[st.ID] = true
		}

		for _, story := range m.Stories {
			if story.SliceID != sl.ID {
				continue
			}
			if story.Why == "" {
				block("story %q has no why", story.Title)
			}
			if len(story.Criteria) == 0 {
				block("story %q has no acceptance criteria", story.Title)
			}
			for _, c := range story.Criteria {
				if c.When == "" || len(c.Then) == 0 {
					block("story %q: criterion %q needs a When and at least one Then", story.Title, c.Text)
				}
				for _, g := range c.Given {
					lint("criterion step", g)
				}
				if c.When != "" {
					lint("criterion step", c.When)
				}
				for _, t := range c.Then {
					lint("criterion step", t)
				}
				for _, text := range append(append(slices.Clone(c.Given), c.When), c.Then...) {
					if w := containsWord(text, uiWords); w != "" {
						note("story %q: %q names UI mechanics (%q); the test's driver knows the UI, the spec shouldn't", story.Title, text, w)
					}
				}
			}
			if len(story.Criteria) > 6 {
				note("story %q has %d criteria; it may be several stories", story.Title, len(story.Criteria))
			}
			if w := containsWord(story.Title, layerWords); w != "" {
				note("story %q mentions %q: is it split along a technical layer rather than what the person gets?", story.Title, w)
			}
		}

		usedScreens := make(map[string]bool)
		for _, st := range steps {
			if len(st.CommandIDs) == 0 && len(st.ReadModelIDs) == 0 {
				block("step %q uses no command and shows no read model", st.Name)
			}
			if st.RoleID == "" {
				block("step %q has no role", st.Name)
			}
			for _, c := range st.CommandIDs {
				if !hasEvents[c] {
					block("step %q issues %s, which has no linked events in the storm", st.Name, cmd[c].Name)
				}
			}
			if st.AfterStepID != "" && !inSlice[st.AfterStepID] {
				block("step %q waits for a step that isn't in this slice, so its journey can't be staged", st.Name)
			}
			if st.JourneyWhen != "" {
				lint("journey step", st.JourneyWhen)
			}
			if st.JourneyThen != "" {
				lint("journey step", st.JourneyThen)
			}
			if st.OffScreen != "" {
				continue
			}
			sc, ok := screen[st.ScreenID]
			if !ok {
				block("step %q has no screen; set screen_id, or off_screen with the reason it's taken off any screen", st.Name)
				continue
			}
			usedScreens[sc.ID] = true
			phrase := ScreenPhrase(sc.Name)
			for _, c := range st.CommandIDs {
				i := slices.IndexFunc(sc.Offers, func(o Offer) bool { return o.CommandID == c })
				if i < 0 {
					block("step %q is on %s, which doesn't offer %s", st.Name, phrase, cmd[c].Name)
					continue
				}
				if sc.Offers[i].ShowsAfter == "" && st.JourneyThen == "" {
					block("step %q: what %s shows after %s isn't recorded, so the journey can't read the outcome from the screen; set the offer's shows_after or the step's journey_then", st.Name, phrase, cmd[c].Name)
				}
			}
			for _, rm := range st.ReadModelIDs {
				if !slices.Contains(sc.Shows, rm) {
					block("step %q is on %s, which doesn't show %s", st.Name, phrase, rmName[rm])
				}
			}
			if st.JourneyWhen != "" && !strings.Contains(strings.ToLower(st.JourneyWhen), strings.TrimPrefix(phrase, "the ")) {
				block("step %q: journey_when %q doesn't name %s, so it could be taken without the screen", st.Name, st.JourneyWhen, phrase)
			}
		}

		for _, sc := range m.Screens {
			if !usedScreens[sc.ID] {
				continue
			}
			for _, o := range sc.Offers {
				if !hasEvents[o.CommandID] {
					block("%s offers %s, which has no linked events: the control would do nothing", ScreenPhrase(sc.Name), cmd[o.CommandID].Name)
				}
				if !journeyCommands[o.CommandID] {
					note("%s offers %s, which no journey uses; its screen scenario is the only check that it works", ScreenPhrase(sc.Name), cmd[o.CommandID].Name)
				}
			}
		}

		for _, c := range m.Constraints {
			applies := c.ScopeKind == "map" ||
				(c.ScopeKind == KindSlice && c.ScopeID == sl.ID) ||
				(c.ScopeKind == KindStep && inSlice[c.ScopeID]) ||
				(c.ScopeKind == KindScreen && usedScreens[c.ScopeID])
			if !applies {
				continue
			}
			if c.Measure == "" || c.Target == "" {
				block("constraint %q needs a measure and a target", c.Text)
			}
			if w := containsWord(c.Text+" "+c.Target, []string{"fast", "quick", "easy", "intuitive", "user-friendly", "friendly", "simple", "responsive"}); w != "" && c.Target == "" {
				note("constraint %q says %q, which isn't testable on its own", c.Text, w)
			}
		}

		r.Ready = len(r.Blocking) == 0
		out = append(out, r)
	}
	return out
}
