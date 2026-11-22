package session

import (
	"fmt"
	"slices"
)

// Severities for link findings.
const (
	SeverityWarning = "warning" // the map and the storm disagree; something will be built wrong or not at all
	SeverityNote    = "note"    // worth a question, not always wrong
	SeverityLint    = "lint"    // a glossary violation in story-map text
)

// LinkFinding is one problem in the link between the story map and the storm.
type LinkFinding struct {
	Severity string `json:"severity"`
	Kind     string `json:"kind"` // the kind of record it's about
	ID       string `json:"id"`
	Message  string `json:"message"`
}

// CheckLinks checks the story map against the storm, both ways:
//   - a step that uses no command and shows no read model (a gap in the storm)
//   - a step with no role
//   - a screen offering a command with no linked events (a button with nothing behind it)
//   - a step on a screen that uses a command the screen doesn't offer
//   - a command no step uses and no policy issues
//   - a read model no screen shows
//   - a role with no steps
//
// and lints step, activity, story and screen text against the glossary.
func CheckLinks(s *Session) []LinkFinding {
	m := s.StoryMap
	if m == nil {
		return nil
	}
	var out []LinkFinding
	add := func(sev, kind, id, format string, args ...any) {
		out = append(out, LinkFinding{Severity: sev, Kind: kind, ID: id, Message: fmt.Sprintf(format, args...)})
	}
	cmdName := func(id string) string {
		if i := slices.IndexFunc(s.Commands, func(c Command) bool { return c.ID == id }); i >= 0 {
			return s.Commands[i].Name
		}
		return id
	}
	hasEvents := make(map[string]bool)
	for _, e := range s.Events {
		hasEvents[e.TriggerCommandID] = true
	}
	screenByID := make(map[string]Screen)
	for _, sc := range m.Screens {
		screenByID[sc.ID] = sc
	}

	usedCommands := make(map[string]bool)
	usedRoles := make(map[string]bool)
	for _, st := range m.Steps {
		usedRoles[st.RoleID] = true
		for _, c := range st.CommandIDs {
			usedCommands[c] = true
		}
		if len(st.CommandIDs) == 0 && len(st.ReadModelIDs) == 0 {
			add(SeverityWarning, KindStep, st.ID,
				"step %q uses no command and shows no read model: a gap in the storm, or a step to link with update_step", st.Name)
		}
		if st.RoleID == "" {
			add(SeverityWarning, KindStep, st.ID, "step %q has no role", st.Name)
		}
		if sc, ok := screenByID[st.ScreenID]; ok {
			for _, c := range st.CommandIDs {
				if !slices.ContainsFunc(sc.Offers, func(o Offer) bool { return o.CommandID == c }) {
					add(SeverityWarning, KindStep, st.ID,
						"step %q happens on screen %q and uses %s, which the screen doesn't offer", st.Name, sc.Name, cmdName(c))
				}
			}
		}
	}
	for _, p := range s.Policies {
		usedCommands[p.ResultingCommandID] = true
	}

	shown := make(map[string]bool)
	for _, sc := range m.Screens {
		for _, rm := range sc.Shows {
			shown[rm] = true
		}
		for _, o := range sc.Offers {
			if !hasEvents[o.CommandID] {
				add(SeverityWarning, KindScreen, sc.ID,
					"screen %q offers %s, which has no linked events: the control would do nothing", sc.Name, cmdName(o.CommandID))
			}
		}
	}

	// Coverage notes only make sense once the map has been started.
	if len(m.Steps) > 0 {
		for _, c := range s.Commands {
			if !usedCommands[c.ID] {
				add(SeverityNote, KindCommand, c.ID,
					"command %q is used by no step and issued by no policy: who issues it?", c.Name)
			}
		}
	}
	if len(m.Screens) > 0 {
		for _, rm := range s.ReadModels {
			if !shown[rm.ID] {
				add(SeverityNote, KindReadModel, rm.ID, "read model %q is shown on no screen", rm.Name)
			}
		}
	}
	for _, r := range m.Roles {
		if !usedRoles[r.ID] {
			add(SeverityNote, KindRole, r.ID, "role %q has no steps", s.actorName(r.ActorID))
		}
	}

	lint := func(kind, id, text string) {
		for _, issue := range LintStep(s, "", text) {
			add(SeverityLint, kind, id, "%q: %s", text, issue.Message)
		}
	}
	for _, a := range m.Activities {
		lint(KindActivity, a.ID, a.Name)
	}
	for _, st := range m.Steps {
		lint(KindStep, st.ID, st.Name)
	}
	for _, st := range m.Stories {
		lint(KindStory, st.ID, st.Title)
	}
	for _, sc := range m.Screens {
		lint(KindScreen, sc.ID, sc.Name)
	}
	return out
}

func (s *Session) actorName(id string) string {
	if i := slices.IndexFunc(s.Actors, func(a Actor) bool { return a.ID == id }); i >= 0 {
		return s.Actors[i].Name
	}
	return id
}
