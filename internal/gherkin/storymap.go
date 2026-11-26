package gherkin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/forgant-foundry/forgant-discovery-mcp/internal/session"
)

// Story-map outputs EmitStoryMap can write. StoryMapOptions.Only selects a subset.
const (
	OutputJourneys    = "journeys"
	OutputStories     = "stories"
	OutputScreens     = "screens"
	OutputConstraints = "constraints"
	OutputSlices      = "slices"
	OutputMap         = "map"
)

var StoryMapOutputs = []string{OutputJourneys, OutputStories, OutputScreens, OutputConstraints, OutputSlices, OutputMap}

type StoryMapOptions struct {
	Only []string
}

// StoryMapSummary describes what EmitStoryMap wrote.
type StoryMapSummary struct {
	FeatureFiles  []string                 `json:"feature_files,omitempty"`
	SlicesFile    string                   `json:"slices_file,omitempty"`
	MapFiles      []string                 `json:"map_files,omitempty"`
	ScenarioCount int                      `json:"scenario_count"`
	Slices        []session.SliceReadiness `json:"slices"`
	Lint          []LintFinding            `json:"lint,omitempty"`
}

// mapEmitter derives the story map's outputs.
type mapEmitter struct {
	*emitter
	m       *session.StoryMap
	role    map[string]session.Role
	screen  map[string]session.Screen
	rmName  map[string]string
	files   map[string][]string // slice ID → files that carry it
	outDir  string
	summary *StoryMapSummary
}

// EmitStoryMap writes the story map's outputs to outDir:
//   - <context>/<slice>.feature: a slice's journeys, one scenario per role
//   - <context>/<activity>.feature: the stories' acceptance criteria
//   - <context>/<screen>.feature: what each screen shows and offers
//   - constraints.feature
//   - story-map/slices.json: each slice's readiness, build order and files
//   - story-map/story-map.json and story-map.md
//
// <context> is the slug of the screens' front-end bounded context, or
// "journeys" / "stories" / "screens" when there isn't one.
func EmitStoryMap(s *session.Session, outDir string, opts StoryMapOptions) (*StoryMapSummary, error) {
	for _, o := range opts.Only {
		if !slices.Contains(StoryMapOutputs, o) {
			return nil, fmt.Errorf("unknown output %q; expected one of %v", o, StoryMapOutputs)
		}
	}
	if s.StoryMap == nil {
		return nil, fmt.Errorf("session %s has no story map yet", s.ID)
	}
	want := func(o string) bool { return len(opts.Only) == 0 || slices.Contains(opts.Only, o) }

	me := &mapEmitter{
		emitter: newEmitter(s),
		m:       s.StoryMap,
		role:    make(map[string]session.Role),
		screen:  make(map[string]session.Screen),
		rmName:  make(map[string]string),
		files:   make(map[string][]string),
		outDir:  outDir,
		summary: &StoryMapSummary{Slices: session.Readiness(s)},
	}
	for _, r := range me.m.Roles {
		me.role[r.ID] = r
	}
	for _, sc := range me.m.Screens {
		me.screen[sc.ID] = sc
	}
	for _, r := range s.ReadModels {
		me.rmName[r.ID] = r.Name
	}

	steps := []struct {
		output string
		emit   func() error
	}{
		{OutputJourneys, me.emitJourneys},
		{OutputStories, me.emitStories},
		{OutputScreens, me.emitScreens},
		{OutputConstraints, me.emitConstraints},
	}
	for _, st := range steps {
		if want(st.output) {
			if err := st.emit(); err != nil {
				return nil, err
			}
		}
	}
	if want(OutputSlices) {
		if err := me.emitSlices(); err != nil {
			return nil, err
		}
	}
	if want(OutputMap) {
		if err := me.emitMap(); err != nil {
			return nil, err
		}
	}
	return me.summary, nil
}

// write writes a feature file under <outDir>/<dir>, lints its steps, and
// records it against the slices it carries.
func (me *mapEmitter) write(dir, name, content string, sliceIDs []string) error {
	path := filepath.Join(me.outDir, dir, name+".feature")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	me.summary.FeatureFiles = append(me.summary.FeatureFiles, path)
	me.summary.Lint = append(me.summary.Lint, me.lintSteps(path, "", content)...)
	for _, id := range sliceIDs {
		if !slices.Contains(me.files[id], path) {
			me.files[id] = append(me.files[id], path)
		}
	}
	return nil
}

func (me *mapEmitter) roleActor(roleID string) session.Actor {
	return me.actorByID[me.role[roleID].ActorID]
}

// rolePhrase is "the accountant".
func (me *mapEmitter) rolePhrase(roleID string) string {
	if a := me.roleActor(roleID); a.Name != "" {
		return "the " + strings.ToLower(a.Name)
	}
	return "the person"
}

// readModelPhrase uses the glossary's canonical phrasing when the read
// model's name is a term, else "the <name>".
func (me *mapEmitter) readModelPhrase(ph Phraser, id string) string {
	words := nameWords(me.rmName[id])
	if c, ok := ph.canonical[strings.Join(words, " ")]; ok {
		return c
	}
	return "the " + strings.Join(words, " ")
}

// screenDir is where a screen's features go: its context's slug.
func (me *mapEmitter) screenDir(sc session.Screen, fallback string) string {
	if ctx, ok := me.contextByID[sc.ContextID]; ok {
		return Slug(ctx.Name)
	}
	return fallback
}

func (me *mapEmitter) contextTag(dir string) string {
	return "@context:" + dir
}

// stepLines derives a journey step's When and Then lines, naming its screen.
func (me *mapEmitter) stepLines(st session.Step) (whens, thens []string) {
	sc, onScreen := me.screen[st.ScreenID]
	ph := me.phraser(sc.ContextID)
	actor := me.roleActor(st.RoleID)
	on := ""
	if onScreen {
		on = " on " + session.ScreenPhrase(sc.Name)
	}
	for _, id := range st.CommandIDs {
		c := me.commandByID[id]
		whens = append(whens, ph.When(c.Name, actor.Name)+on)
		if onScreen {
			if i := slices.IndexFunc(sc.Offers, func(o session.Offer) bool { return o.CommandID == id }); i >= 0 && sc.Offers[i].ShowsAfter != "" {
				thens = append(thens, session.ScreenPhrase(sc.Name)+" shows "+sc.Offers[i].ShowsAfter)
				continue
			}
		}
		for _, ev := range me.eventsByCmd[id] {
			if ev.Condition == "" {
				thens = append(thens, thenText(ph, ev))
				break
			}
		}
	}
	if len(st.CommandIDs) == 0 && len(st.ReadModelIDs) > 0 {
		target := me.readModelPhrase(ph, st.ReadModelIDs[0])
		if onScreen {
			target = session.ScreenPhrase(sc.Name)
		}
		whens = append(whens, me.rolePhrase(st.RoleID)+" views "+target)
	}
	for _, rm := range st.ReadModelIDs {
		if onScreen {
			thens = append(thens, session.ScreenPhrase(sc.Name)+" shows "+me.readModelPhrase(ph, rm))
		} else {
			thens = append(thens, me.readModelPhrase(ph, rm)+" is shown")
		}
	}
	if st.JourneyWhen != "" {
		whens = []string{st.JourneyWhen}
	}
	if st.JourneyThen != "" {
		thens = []string{st.JourneyThen}
	}
	return whens, thens
}

// journeyDir is the context most of a slice's screens belong to.
func (me *mapEmitter) journeyDir(steps []session.Step) string {
	count := make(map[string]int)
	best, n := "journeys", 0
	for _, st := range steps {
		if sc, ok := me.screen[st.ScreenID]; ok {
			d := me.screenDir(sc, "")
			if d == "" {
				continue
			}
			count[d]++
			if count[d] > n {
				best, n = d, count[d]
			}
		}
	}
	return best
}

// screenSlug names a screen's feature file and tag: "Account" → "account-screen".
func screenSlug(name string) string {
	slug := Slug(name)
	if !strings.HasSuffix(slug, "screen") {
		slug += "-screen"
	}
	return slug
}

func writeLines(b *strings.Builder, kw string, lines []string, first *bool) {
	for _, l := range lines {
		k := kw
		if !*first {
			k = "And"
		}
		b.WriteString(fmt.Sprintf("    %s %s\n", k, l))
		*first = false
	}
}

func (me *mapEmitter) emitJourneys() error {
	for _, sl := range me.m.Slices {
		steps := me.m.SliceSteps(sl.ID)
		if len(steps) == 0 {
			continue
		}
		var roles []string
		for _, st := range steps {
			if st.RoleID != "" && !slices.Contains(roles, st.RoleID) {
				roles = append(roles, st.RoleID)
			}
		}
		dir := me.journeyDir(steps)

		var b strings.Builder
		tags := []string{me.contextTag(dir), "@running", "@slice:" + Slug(sl.Name)}
		if sl.WalkingSkeleton {
			tags = append(tags, "@walking-skeleton")
		}
		b.WriteString(strings.Join(tags, " ") + "\n")
		b.WriteString(fmt.Sprintf("Feature: %s\n", sl.Name))
		if sl.Outcome != "" {
			b.WriteString(fmt.Sprintf("\n  %s\n", sl.Outcome))
		}
		b.WriteString("\n  Each scenario is one role's journey through the slice, in narrative order.\n")
		b.WriteString("  A @journey scenario is taken on the screens it names, as a person would;\n")
		b.WriteString("  no step calls the API, and every outcome is read from the screen.\n")

		for _, rid := range roles {
			own := slices.DeleteFunc(slices.Clone(steps), func(st session.Step) bool { return st.RoleID != rid })
			surface := slices.ContainsFunc(own, func(st session.Step) bool { return st.OffScreen == "" })
			me.summary.ScenarioCount++
			b.WriteString("\n")
			stags := "@role:" + Slug(me.roleActor(rid).Name)
			if surface {
				stags += " @journey"
			}
			b.WriteString("  " + stags + "\n")
			b.WriteString(fmt.Sprintf("  Scenario: %s's %s\n", me.rolePhrase(rid), strings.ToLower(sl.Name)))

			var pre []string
			for _, g := range sl.Preconditions {
				pre = append(pre, lowerFirst(g))
			}
			first := true
			writeLines(&b, "Given", pre, &first)
			// replayed is how far through the slice's steps a hand-off has
			// already replayed, so a second hand-off continues from there.
			replayed := 0
			started := false
			for _, st := range own {
				// A hand-off replays the other roles' journeys up to the step
				// waited for, as state rather than as their actions.
				if st.AfterStepID != "" {
					var state []string
					for i := replayed; i < len(steps); i++ {
						other := steps[i]
						if other.RoleID != rid {
							_, thens := me.stepLines(other)
							state = append(state, thens...)
						}
						replayed = i + 1
						if other.ID == st.AfterStepID {
							break
						}
					}
					// Before the role's first When, the replayed state continues
					// the opening Givens; after it, it starts a new Given.
					if !started {
						writeLines(&b, "Given", state, &first)
					} else {
						given := true
						writeLines(&b, "Given", state, &given)
					}
				}
				started = true
				whens, thens := me.stepLines(st)
				w := true
				writeLines(&b, "When", whens, &w)
				t := true
				writeLines(&b, "Then", thens, &t)
			}
		}
		if err := me.write(dir, Slug(sl.Name), b.String(), []string{sl.ID}); err != nil {
			return err
		}
	}
	return nil
}

func (me *mapEmitter) emitStories() error {
	type group struct {
		dir        string
		activityID string
	}
	activityByID := make(map[string]session.Activity)
	for _, a := range me.m.Activities {
		activityByID[a.ID] = a
	}
	var order []group
	byGroup := make(map[group][]session.Story)
	stepByID := make(map[string]session.Step)
	for _, st := range me.m.Steps {
		stepByID[st.ID] = st
	}
	for _, st := range me.m.OrderedSteps() {
		dir := "stories"
		if sc, ok := me.screen[st.ScreenID]; ok {
			dir = me.screenDir(sc, "stories")
		}
		g := group{dir, st.ActivityID}
		for _, story := range me.m.Stories {
			if story.StepID != st.ID || len(story.Criteria) == 0 {
				continue
			}
			if _, seen := byGroup[g]; !seen {
				order = append(order, g)
			}
			byGroup[g] = append(byGroup[g], story)
		}
	}
	sliceName := make(map[string]string)
	for _, sl := range me.m.Slices {
		sliceName[sl.ID] = sl.Name
	}
	for _, g := range order {
		var b strings.Builder
		var sliceIDs []string
		activity := activityByID[g.activityID]
		b.WriteString(fmt.Sprintf("%s @activity:%s\n", me.contextTag(g.dir), Slug(activity.Name)))
		b.WriteString(fmt.Sprintf("Feature: %s\n", activity.Name))
		for _, story := range byGroup[g] {
			b.WriteString(fmt.Sprintf("\n  # Story: %s", story.Title))
			if story.Why != "" {
				b.WriteString(fmt.Sprintf(", so that %s", story.Why))
			}
			b.WriteString("\n")
			tags := "@story:" + Slug(story.Title)
			if story.SliceID != "" {
				tags += " @slice:" + Slug(sliceName[story.SliceID])
				sliceIDs = append(sliceIDs, story.SliceID)
			}
			if story.Source == session.SourceFacilitator {
				tags += " @facilitator"
			}
			for _, c := range story.Criteria {
				me.summary.ScenarioCount++
				kw := "Scenario"
				if c.Examples != nil {
					kw = "Scenario Outline"
				}
				b.WriteString(fmt.Sprintf("\n  %s\n  %s: %s\n", tags, kw, c.Text))
				if c.When == "" && len(c.Then) == 0 {
					b.WriteString("    # TODO: give this criterion a When and a Then\n")
				}
				writeGivens(&b, c.Given)
				if c.When != "" {
					b.WriteString(fmt.Sprintf("    When %s\n", c.When))
				}
				t := true
				writeLines(&b, "Then", c.Then, &t)
				if c.Examples != nil {
					b.WriteString("\n    Examples:\n")
					b.WriteString("      | " + strings.Join(c.Examples.Columns, " | ") + " |\n")
					for _, row := range c.Examples.Rows {
						b.WriteString("      | " + strings.Join(row, " | ") + " |\n")
					}
				}
			}
		}
		if err := me.write(g.dir, Slug(activity.Name), b.String(), sliceIDs); err != nil {
			return err
		}
	}
	return nil
}

func (me *mapEmitter) emitScreens() error {
	for _, sc := range me.m.Screens {
		dir := me.screenDir(sc, "screens")
		ph := me.phraser(sc.ContextID)
		phrase := session.ScreenPhrase(sc.Name)
		// The person who uses the screen: the first role with a step on it.
		person := "the person"
		var sliceIDs []string
		for _, st := range me.m.OrderedSteps() {
			if st.ScreenID != sc.ID {
				continue
			}
			if person == "the person" && st.RoleID != "" {
				person = me.rolePhrase(st.RoleID)
			}
			for _, story := range me.m.Stories {
				if story.StepID == st.ID && story.SliceID != "" && !slices.Contains(sliceIDs, story.SliceID) {
					sliceIDs = append(sliceIDs, story.SliceID)
				}
			}
		}
		actorName := strings.TrimPrefix(person, "the ")

		var b strings.Builder
		slug := screenSlug(sc.Name)
		b.WriteString(fmt.Sprintf("%s @screen:%s\n", me.contextTag(dir), slug))
		b.WriteString(fmt.Sprintf("Feature: %s\n", sc.Name))
		if len(sc.Shows) > 0 {
			b.WriteString(fmt.Sprintf("\n  Rule: %s shows what the person needs to see there\n", phrase))
			for _, rm := range sc.Shows {
				me.summary.ScenarioCount++
				shown := me.readModelPhrase(ph, rm)
				b.WriteString(fmt.Sprintf("\n  Scenario: %s shows %s\n", phrase, shown))
				b.WriteString(fmt.Sprintf("    When %s opens %s\n", person, phrase))
				b.WriteString(fmt.Sprintf("    Then %s shows %s\n", phrase, shown))
			}
		}
		if len(sc.Offers) > 0 {
			b.WriteString(fmt.Sprintf("\n  Rule: every command %s offers works when it's used there, and %s shows the result. A command it offers that does nothing when used is a defect.\n", phrase, phrase))
			for _, o := range sc.Offers {
				me.summary.ScenarioCount++
				c := me.commandByID[o.CommandID]
				when := ph.When(c.Name, actorName) + " on " + phrase
				b.WriteString(fmt.Sprintf("\n  Scenario: %s\n", strings.TrimPrefix(when, "the ")))
				b.WriteString(fmt.Sprintf("    Given %s is on %s\n", person, phrase))
				b.WriteString(fmt.Sprintf("    When %s\n", when))
				first := true
				for _, ev := range me.eventsByCmd[o.CommandID] {
					if ev.Condition == "" {
						writeLines(&b, "Then", []string{thenText(ph, ev)}, &first)
						break
					}
				}
				if o.ShowsAfter != "" {
					writeLines(&b, "Then", []string{phrase + " shows " + o.ShowsAfter}, &first)
				}
				if first {
					b.WriteString("    # TODO: the storm has no outcome for this command, and the offer has no shows_after\n")
				}
			}
		}
		if len(sc.Shows) == 0 && len(sc.Offers) == 0 {
			b.WriteString("\n  # TODO: record what this screen shows and offers\n")
		}
		if err := me.write(dir, slug, b.String(), sliceIDs); err != nil {
			return err
		}
	}
	return nil
}

func (me *mapEmitter) emitConstraints() error {
	if len(me.m.Constraints) == 0 {
		return nil
	}
	var b strings.Builder
	var sliceIDs []string
	b.WriteString("@constraints\nFeature: Constraints\n\n  Cross-functional requirements, each with what's measured and its target.\n")
	for _, c := range me.m.Constraints {
		me.summary.ScenarioCount++
		tags := "@constraint:" + string(c.Kind)
		switch c.ScopeKind {
		case session.KindSlice:
			for _, sl := range me.m.Slices {
				if sl.ID == c.ScopeID {
					tags += " @slice:" + Slug(sl.Name)
					sliceIDs = append(sliceIDs, sl.ID)
				}
			}
		case session.KindScreen:
			tags += " @screen:" + screenSlug(me.screen[c.ScopeID].Name)
		case session.KindStep:
			for _, st := range me.m.Steps {
				if st.ID == c.ScopeID {
					tags += " @step:" + Slug(st.Name)
				}
			}
		default:
			for _, sl := range me.m.Slices {
				sliceIDs = append(sliceIDs, sl.ID)
			}
		}
		b.WriteString(fmt.Sprintf("\n  %s\n  Scenario: %s\n", tags, c.Text))
		if c.Measure == "" || c.Target == "" {
			b.WriteString("    # TODO: give this constraint a measure and a target\n")
			continue
		}
		verb := " is "
		if t := strings.ToLower(c.Target); strings.HasPrefix(t, "is ") || strings.HasPrefix(t, "are ") {
			verb = " "
		}
		b.WriteString(fmt.Sprintf("    Then %s%s%s\n", lowerFirst(c.Measure), verb, c.Target))
	}
	return me.write(".", "constraints", b.String(), sliceIDs)
}

// buildOrder puts the walking skeleton first, then the other slices top to
// bottom.
func (me *mapEmitter) buildOrder() []session.Slice {
	var out []session.Slice
	for _, sl := range me.m.Slices {
		if sl.WalkingSkeleton {
			out = append(out, sl)
		}
	}
	for _, sl := range me.m.Slices {
		if !sl.WalkingSkeleton {
			out = append(out, sl)
		}
	}
	return out
}

func (me *mapEmitter) emitSlices() error {
	type sliceEntry struct {
		session.SliceReadiness
		Outcome string   `json:"outcome,omitempty"`
		Files   []string `json:"files,omitempty"`
	}
	ready := make(map[string]session.SliceReadiness)
	for _, r := range me.summary.Slices {
		ready[r.SliceID] = r
	}
	doc := struct {
		SessionID  string       `json:"session_id"`
		BuildOrder []string     `json:"build_order"`
		Slices     []sliceEntry `json:"slices"`
	}{SessionID: me.s.ID}
	for _, sl := range me.buildOrder() {
		doc.BuildOrder = append(doc.BuildOrder, sl.Name)
		var files []string
		for _, f := range me.files[sl.ID] {
			if rel, err := filepath.Rel(me.outDir, f); err == nil {
				files = append(files, rel)
			}
		}
		doc.Slices = append(doc.Slices, sliceEntry{SliceReadiness: ready[sl.ID], Outcome: sl.Outcome, Files: files})
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(me.outDir, "story-map", "slices.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	me.summary.SlicesFile = path
	return nil
}

func (me *mapEmitter) emitMap() error {
	dir := filepath.Join(me.outDir, "story-map")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(me.m, "", "  ")
	if err != nil {
		return err
	}
	jsonPath := filepath.Join(dir, "story-map.json")
	if err := os.WriteFile(jsonPath, data, 0o644); err != nil {
		return err
	}
	mdPath := filepath.Join(dir, "story-map.md")
	if err := os.WriteFile(mdPath, []byte(me.mapMarkdown()), 0o644); err != nil {
		return err
	}
	me.summary.MapFiles = []string{jsonPath, mdPath}
	return nil
}

func (me *mapEmitter) mapMarkdown() string {
	var b strings.Builder
	f := me.m.Frame
	b.WriteString("# Story Map\n\n")
	if f.ForWhom != "" {
		b.WriteString(fmt.Sprintf("**For:** %s\n\n", f.ForWhom))
	}
	if f.Why != "" {
		b.WriteString(fmt.Sprintf("**Why:** %s\n\n", f.Why))
	}
	for _, m := range f.SuccessMeasures {
		b.WriteString(fmt.Sprintf("- Success: %s\n", m))
	}

	sliceName := make(map[string]string)
	for _, sl := range me.m.Slices {
		sliceName[sl.ID] = sl.Name
	}
	b.WriteString("\n## Backbone\n")
	for _, a := range me.m.Activities {
		b.WriteString(fmt.Sprintf("\n### %s\n\n", a.Name))
		b.WriteString("| Step | Role | Screen | Stories |\n|---|---|---|---|\n")
		for _, st := range me.m.Steps {
			if st.ActivityID != a.ID {
				continue
			}
			screen := me.screen[st.ScreenID].Name
			if st.OffScreen != "" {
				screen = "off screen: " + st.OffScreen
			}
			var stories []string
			for _, story := range me.m.Stories {
				if story.StepID != st.ID {
					continue
				}
				label := story.Title
				if story.SliceID != "" {
					label += " [" + sliceName[story.SliceID] + "]"
				}
				stories = append(stories, label)
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", st.Name, me.roleActor(st.RoleID).Name, screen, strings.Join(stories, "; ")))
		}
	}
	if len(me.m.Screens) > 0 {
		b.WriteString("\n## Screens\n\n| Screen | Shows | Offers |\n|---|---|---|\n")
		for _, sc := range me.m.Screens {
			var shows, offers []string
			for _, rm := range sc.Shows {
				shows = append(shows, me.rmName[rm])
			}
			for _, o := range sc.Offers {
				offers = append(offers, me.commandByID[o.CommandID].Name)
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", sc.Name, strings.Join(shows, ", "), strings.Join(offers, ", ")))
		}
	}
	b.WriteString("\n## Slices\n")
	for _, r := range me.summary.Slices {
		status := "ready"
		if !r.Ready {
			status = fmt.Sprintf("not ready (%d blocking)", len(r.Blocking))
		}
		b.WriteString(fmt.Sprintf("\n### %s (%s): %s\n", r.Name, r.Kind, status))
		for _, x := range r.Blocking {
			b.WriteString("- Blocking: " + x + "\n")
		}
		for _, x := range r.Notes {
			b.WriteString("- Note: " + x + "\n")
		}
	}
	return b.String()
}
