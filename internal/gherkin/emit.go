package gherkin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/forgant-foundry/event-storming-mcp/internal/session"
)

// Outputs Emit can write. Options.Only selects a subset.
const (
	OutputFeatures   = "features"
	OutputGlossary   = "glossary"
	OutputContextMap = "context_map"
)

// Outputs lists every output Emit can write.
var Outputs = []string{OutputFeatures, OutputGlossary, OutputContextMap}

// Options controls what Emit writes.
type Options struct {
	// Only limits the outputs written; empty means all of them. Use it to
	// regenerate glossary.md or context-map.json without overwriting
	// hand-edited .feature files.
	Only []string
}

// Summary is returned by Emit and describes what was written.
type Summary struct {
	FeatureFiles   []string `json:"feature_files,omitempty"`
	GlossaryFile   string   `json:"glossary_file,omitempty"`
	ContextMapFile string   `json:"context_map_file,omitempty"`
	ScenarioCount  int      `json:"scenario_count"`
	OpenHotSpots   int      `json:"open_hot_spots"`
	Warnings       []string `json:"warnings,omitempty"`
}

// emitter holds the lookup tables shared by every output.
type emitter struct {
	s             *session.Session
	contextByID   map[string]session.BoundedContext
	actorByID     map[string]session.Actor
	eventByID     map[string]session.Event
	commandByID   map[string]session.Command
	aggByID       map[string]session.Aggregate
	eventsByCmd   map[string][]session.Event
	commandsByAgg map[string][]session.Command
	openHotSpots  map[string][]session.HotSpot // by context ID
}

func newEmitter(s *session.Session) *emitter {
	e := &emitter{
		s:             s,
		contextByID:   make(map[string]session.BoundedContext),
		actorByID:     make(map[string]session.Actor),
		eventByID:     make(map[string]session.Event),
		commandByID:   make(map[string]session.Command),
		aggByID:       make(map[string]session.Aggregate),
		eventsByCmd:   make(map[string][]session.Event),
		commandsByAgg: make(map[string][]session.Command),
		openHotSpots:  make(map[string][]session.HotSpot),
	}
	for _, c := range s.BoundedContexts {
		e.contextByID[c.ID] = c
	}
	for _, a := range s.Actors {
		e.actorByID[a.ID] = a
	}
	for _, a := range s.Aggregates {
		e.aggByID[a.ID] = a
	}
	for _, ev := range s.Events {
		e.eventByID[ev.ID] = ev
		if ev.TriggerCommandID != "" {
			e.eventsByCmd[ev.TriggerCommandID] = append(e.eventsByCmd[ev.TriggerCommandID], ev)
		}
	}
	for _, c := range s.Commands {
		e.commandByID[c.ID] = c
		if c.AggregateID != "" {
			e.commandsByAgg[c.AggregateID] = append(e.commandsByAgg[c.AggregateID], c)
		}
	}
	for _, h := range s.HotSpots {
		if !h.Resolved {
			e.openHotSpots[h.ContextID] = append(e.openHotSpots[h.ContextID], h)
		}
	}
	return e
}

// Emit writes the outputs to outDir:
//   - one .feature file per aggregate, plus policies.feature per context
//   - glossary.md
//   - context-map.json
func Emit(s *session.Session, outDir string, opts Options) (*Summary, error) {
	for _, o := range opts.Only {
		if !slices.Contains(Outputs, o) {
			return nil, fmt.Errorf("unknown output %q; expected one of %v", o, Outputs)
		}
	}
	want := func(o string) bool { return len(opts.Only) == 0 || slices.Contains(opts.Only, o) }

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create output dir: %w", err)
	}

	e := newEmitter(s)
	sum := &Summary{}
	for _, h := range s.HotSpots {
		if !h.Resolved {
			sum.OpenHotSpots++
		}
	}

	if want(OutputFeatures) {
		if err := e.writeFeatures(outDir, sum); err != nil {
			return nil, err
		}
	}

	if want(OutputGlossary) {
		gPath := filepath.Join(outDir, "glossary.md")
		if err := os.WriteFile(gPath, []byte(e.buildGlossary()), 0o644); err != nil {
			return nil, err
		}
		sum.GlossaryFile = gPath
	}

	if want(OutputContextMap) {
		cmPath := filepath.Join(outDir, "context-map.json")
		cmData, err := e.buildContextMap()
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(cmPath, cmData, 0o644); err != nil {
			return nil, err
		}
		sum.ContextMapFile = cmPath
	}

	return sum, nil
}

// contextDir returns the directory name and tag value for a context ID.
func (e *emitter) contextDir(ctxID string) (dir, tag string) {
	if ctx, ok := e.contextByID[ctxID]; ok {
		return Slug(ctx.Name), Slug(ctx.Name)
	}
	return "uncategorised", "unknown"
}

func (e *emitter) writeFeature(outDir, ctxID, name, content string, sum *Summary) error {
	dirName, _ := e.contextDir(ctxID)
	dir := filepath.Join(outDir, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, name+".feature")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	sum.FeatureFiles = append(sum.FeatureFiles, path)
	return nil
}

func (e *emitter) writeFeatures(outDir string, sum *Summary) error {
	for _, agg := range e.s.Aggregates {
		content, warns, n := e.buildFeature(agg)
		sum.Warnings = append(sum.Warnings, warns...)
		sum.ScenarioCount += n
		if err := e.writeFeature(outDir, agg.ContextID, Slug(agg.Name), content, sum); err != nil {
			return err
		}
	}

	// Policies, grouped by the context they resolve to, in session order.
	byCtx := make(map[string][]session.Policy)
	var ctxOrder []string
	for _, p := range e.s.Policies {
		id := e.policyContext(p)
		if _, seen := byCtx[id]; !seen {
			ctxOrder = append(ctxOrder, id)
		}
		byCtx[id] = append(byCtx[id], p)
	}
	for _, ctxID := range ctxOrder {
		content, warns, n := e.buildPolicyFeature(ctxID, byCtx[ctxID])
		sum.Warnings = append(sum.Warnings, warns...)
		sum.ScenarioCount += n
		if err := e.writeFeature(outDir, ctxID, "policies", content, sum); err != nil {
			return err
		}
	}
	return nil
}

// policyContext is the policy's own context, else the context of the
// aggregate that receives its command, else that of its trigger event's
// aggregate.
func (e *emitter) policyContext(p session.Policy) string {
	if p.ContextID != "" {
		return p.ContextID
	}
	if c, ok := e.commandByID[p.ResultingCommandID]; ok {
		if agg, ok := e.aggByID[c.AggregateID]; ok && agg.ContextID != "" {
			return agg.ContextID
		}
	}
	if ev, ok := e.eventByID[p.TriggerEventID]; ok {
		if agg, ok := e.aggByID[ev.AggregateID]; ok {
			return agg.ContextID
		}
	}
	return ""
}

func whenText(cmd session.Command, actor session.Actor) string {
	if cmd.Phrasing != "" {
		return cmd.Phrasing
	}
	return WhenStep(cmd.Name, actor.Name)
}

func titleText(cmd session.Command, actor session.Actor) string {
	if cmd.Phrasing != "" {
		return cmd.Phrasing
	}
	return ScenarioTitle(cmd.Name, actor.Name)
}

func thenText(ev session.Event) string {
	if ev.Phrasing != "" {
		return ev.Phrasing
	}
	return ThenStep(ev.Name)
}

// writeHotSpots lists a context's open hot spots as comments at the top of
// each of its feature files, where reviewers of the specs will see them.
func (e *emitter) writeHotSpots(b *strings.Builder, ctxID string) {
	if ctxID == "" {
		return
	}
	for _, h := range e.openHotSpots[ctxID] {
		b.WriteString(fmt.Sprintf("# Open question (%s): %s\n", h.ID, h.Question))
	}
}

func (e *emitter) buildFeature(agg session.Aggregate) (content string, warnings []string, scenarioCount int) {
	var b strings.Builder
	_, ctxTag := e.contextDir(agg.ContextID)

	e.writeHotSpots(&b, agg.ContextID)
	// Gherkin tags may not contain whitespace, so names are slugged.
	tags := fmt.Sprintf("@context:%s @aggregate:%s", ctxTag, Slug(agg.Name))
	if agg.Deprecated {
		tags += " @deprecated"
	}
	b.WriteString(tags + "\n")
	b.WriteString(fmt.Sprintf("Feature: %s\n", agg.Name))
	if agg.Deprecated {
		reason := agg.DeprecationReason
		if reason == "" {
			reason = "no reason given"
		}
		b.WriteString(fmt.Sprintf("\n  # Deprecated: %s\n", reason))
		return b.String(), warnings, 0
	}
	if agg.Notes != "" {
		b.WriteString(fmt.Sprintf("\n  %s\n", agg.Notes))
	}

	for _, cmd := range e.commandsByAgg[agg.ID] {
		allEvents := e.eventsByCmd[cmd.ID]
		if len(allEvents) == 0 {
			warnings = append(warnings, fmt.Sprintf(
				"command %q (%s) has no linked events — scenario omitted; add events with trigger_command_id=%s",
				cmd.Name, cmd.ID, cmd.ID,
			))
			continue
		}

		actor := e.actorByID[cmd.ActorID]

		// Partition into unconditional (happy-path) and conditional events.
		var unconditional, conditional []session.Event
		for _, ev := range allEvents {
			if ev.Condition == "" {
				unconditional = append(unconditional, ev)
			} else {
				conditional = append(conditional, ev)
			}
		}

		// Happy-path scenario (unconditional events only).
		if len(unconditional) > 0 {
			scenarioCount++
			b.WriteString("\n")
			writeProposedTag(&b, cmd.Source == session.SourceFacilitator, unconditional...)
			b.WriteString(fmt.Sprintf("  Scenario: %s\n", titleText(cmd, actor)))
			for _, pre := range cmd.Preconditions {
				b.WriteString(fmt.Sprintf("    Given %s\n", pre))
			}
			b.WriteString(fmt.Sprintf("    When %s\n", whenText(cmd, actor)))
			for i, ev := range unconditional {
				kw := "Then"
				if i > 0 {
					kw = "And"
				}
				b.WriteString(fmt.Sprintf("    %s %s\n", kw, thenText(ev)))
			}
		}

		// One extra scenario per conditional event.
		for _, ev := range conditional {
			scenarioCount++
			b.WriteString("\n")
			title := titleText(cmd, actor) + " (" + strings.ToLower(ev.Condition) + ")"
			writeProposedTag(&b, cmd.Source == session.SourceFacilitator, ev)
			b.WriteString(fmt.Sprintf("  Scenario: %s\n", title))
			for _, pre := range cmd.Preconditions {
				b.WriteString(fmt.Sprintf("    Given %s\n", pre))
			}
			condKw := "And"
			if len(cmd.Preconditions) == 0 {
				condKw = "Given"
			}
			b.WriteString(fmt.Sprintf("    %s %s\n", condKw, ev.Condition))
			b.WriteString(fmt.Sprintf("    When %s\n", whenText(cmd, actor)))
			b.WriteString(fmt.Sprintf("    Then %s\n", thenText(ev)))
		}

		// A lone conditional event with no happy path is usually a gap. Two or
		// more conditional events with no unconditional one is a branch, and
		// each branch already has its own scenario.
		if len(unconditional) == 0 && len(conditional) == 1 {
			warnings = append(warnings, fmt.Sprintf(
				"command %q has a single conditional event and no happy path — add the unconditional outcome or a second branch",
				cmd.Name,
			))
		}
	}

	// If no commands were emitted but events exist, emit a stub.
	if scenarioCount == 0 {
		var aggEvents []session.Event
		for _, eid := range agg.EventIDs {
			if ev, ok := e.eventByID[eid]; ok {
				aggEvents = append(aggEvents, ev)
			}
		}
		if len(aggEvents) > 0 {
			warnings = append(warnings, fmt.Sprintf(
				"aggregate %q has no commands with linked events; emitting event stub",
				agg.Name,
			))
			b.WriteString("\n  # TODO: add commands and link events via trigger_command_id\n")
			b.WriteString("  Scenario: [uncaptured commands]\n")
			b.WriteString("    Given [preconditions unknown]\n")
			b.WriteString("    When [command unknown]\n")
			for i, ev := range aggEvents {
				kw := "Then"
				if i > 0 {
					kw = "And"
				}
				b.WriteString(fmt.Sprintf("    %s %s\n", kw, thenText(ev)))
			}
			scenarioCount = 1
		} else {
			warnings = append(warnings, fmt.Sprintf(
				"aggregate %q has no commands or events — feature file is empty",
				agg.Name,
			))
			b.WriteString("\n  # TODO: no commands or events recorded for this aggregate\n")
		}
	}

	return b.String(), warnings, scenarioCount
}

// buildPolicyFeature emits one scenario per policy: the trigger event is the
// When, the resulting command is the Then, and a policy condition is a Given.
func (e *emitter) buildPolicyFeature(ctxID string, policies []session.Policy) (content string, warnings []string, scenarioCount int) {
	var b strings.Builder
	_, ctxTag := e.contextDir(ctxID)
	name := "Uncategorised"
	if ctx, ok := e.contextByID[ctxID]; ok {
		name = ctx.Name
	}

	e.writeHotSpots(&b, ctxID)
	b.WriteString(fmt.Sprintf("@context:%s\n", ctxTag))
	b.WriteString(fmt.Sprintf("Feature: %s policies\n", name))

	for _, p := range policies {
		ev, hasEvent := e.eventByID[p.TriggerEventID]
		cmd, hasCmd := e.commandByID[p.ResultingCommandID]
		if !hasEvent || !hasCmd {
			warnings = append(warnings, fmt.Sprintf(
				"policy %q (%s) needs both a trigger event and a resulting command — scenario omitted",
				p.Name, p.ID,
			))
			continue
		}
		scenarioCount++
		b.WriteString("\n")
		tags := "@policy:" + Slug(p.Name)
		if p.Source == session.SourceFacilitator {
			tags += " @proposed"
		}
		b.WriteString(fmt.Sprintf("  %s\n", tags))
		b.WriteString(fmt.Sprintf("  Scenario: %s\n", p.Name))
		if p.Condition != "" {
			b.WriteString(fmt.Sprintf("    Given %s\n", p.Condition))
		}
		b.WriteString(fmt.Sprintf("    When %s\n", thenText(ev)))
		b.WriteString(fmt.Sprintf("    Then %s\n", whenText(cmd, e.actorByID[cmd.ActorID])))
	}
	if scenarioCount == 0 {
		b.WriteString("\n  # TODO: no policy has both a trigger event and a resulting command\n")
	}
	return b.String(), warnings, scenarioCount
}

// writeProposedTag tags a scenario @proposed when its command or any of its
// events came from the facilitator rather than the domain expert, so those
// scenarios get an extra review.
func writeProposedTag(b *strings.Builder, proposed bool, events ...session.Event) {
	for _, ev := range events {
		proposed = proposed || ev.Source == session.SourceFacilitator
	}
	if proposed {
		b.WriteString("  @proposed\n")
	}
}

func (e *emitter) contextNames(ids []string) []string {
	var names []string
	for _, id := range ids {
		if c, ok := e.contextByID[id]; ok {
			names = append(names, c.Name)
		}
	}
	return names
}

func (e *emitter) buildGlossary() string {
	var b strings.Builder
	b.WriteString("# Glossary\n\n")
	b.WriteString("Terms defined during this event storming session.\n\n")

	// Group terms by context.
	byContext := make(map[string][]session.GlossaryTerm)
	for _, t := range e.s.Glossary {
		byContext[t.ContextID] = append(byContext[t.ContextID], t)
	}

	writeTable := func(terms []session.GlossaryTerm) {
		b.WriteString("| Term | Definition | Canonical Phrasing | Aliases | Shared With |\n")
		b.WriteString("|---|---|---|---|---|\n")
		for _, t := range terms {
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
				t.Term, t.Definition, t.CanonicalPhrasing,
				strings.Join(t.Aliases, ", "), strings.Join(e.contextNames(t.SharedWith), ", ")))
		}
		b.WriteString("\n")
	}

	// Emit each context's terms.
	for _, ctx := range e.s.BoundedContexts {
		terms, ok := byContext[ctx.ID]
		if !ok {
			continue
		}
		b.WriteString(fmt.Sprintf("## %s\n\n", ctx.Name))
		writeTable(terms)
		delete(byContext, ctx.ID)
	}

	// Terms not assigned to any context.
	if unassigned := byContext[""]; len(unassigned) > 0 {
		b.WriteString("## (No context)\n\n")
		writeTable(unassigned)
	}

	return b.String()
}

// contextMapJSON is the structure written to context-map.json.
type contextMapJSON struct {
	SessionID               string           `json:"session_id"`
	SessionName             string           `json:"session_name"`
	GeneratedAt             time.Time        `json:"generated_at"`
	Actors                  []actorEntry     `json:"actors,omitempty"`
	BoundedContexts         []ctxEntry       `json:"bounded_contexts"`
	UncategorisedAggregates []aggEntry       `json:"uncategorised_aggregates,omitempty"`
	UncategorisedPolicies   []policyEntry    `json:"uncategorised_policies,omitempty"`
	UncategorisedReadModels []readModelEntry `json:"uncategorised_read_models,omitempty"`
	UncategorisedHotSpots   []hotSpotEntry   `json:"uncategorised_hot_spots,omitempty"`
}

type ctxEntry struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Aggregates  []aggEntry       `json:"aggregates"`
	Policies    []policyEntry    `json:"policies,omitempty"`
	ReadModels  []readModelEntry `json:"read_models,omitempty"`
	HotSpots    []hotSpotEntry   `json:"hot_spots,omitempty"`
	Glossary    []termEntry      `json:"glossary_terms,omitempty"`
}

type aggEntry struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Events   []string `json:"events"`
	Commands []string `json:"commands"`
}

type actorEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type policyEntry struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	TriggerEvent string `json:"trigger_event,omitempty"`
	Command      string `json:"command,omitempty"`
	Condition    string `json:"condition,omitempty"`
}

type readModelEntry struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Readers []string `json:"readers,omitempty"`
	Notes   string   `json:"notes,omitempty"`
}

type hotSpotEntry struct {
	ID         string `json:"id"`
	Question   string `json:"question"`
	Resolved   bool   `json:"resolved"`
	Resolution string `json:"resolution,omitempty"`
}

type termEntry struct {
	Term              string   `json:"term"`
	Definition        string   `json:"definition"`
	CanonicalPhrasing string   `json:"canonical_phrasing,omitempty"`
	Aliases           []string `json:"aliases,omitempty"`
	SharedWith        []string `json:"shared_with,omitempty"`
}

func (e *emitter) buildContextMap() ([]byte, error) {
	aggEntryFor := func(agg session.Aggregate) aggEntry {
		var events, commands []string
		for _, eid := range agg.EventIDs {
			if ev, ok := e.eventByID[eid]; ok {
				events = append(events, ev.Name)
			}
		}
		for _, cmd := range e.commandsByAgg[agg.ID] {
			commands = append(commands, cmd.Name)
		}
		return aggEntry{ID: agg.ID, Name: agg.Name, Events: events, Commands: commands}
	}
	policyEntryFor := func(p session.Policy) policyEntry {
		return policyEntry{
			ID:           p.ID,
			Name:         p.Name,
			TriggerEvent: e.eventByID[p.TriggerEventID].Name,
			Command:      e.commandByID[p.ResultingCommandID].Name,
			Condition:    p.Condition,
		}
	}
	readModelEntryFor := func(rm session.ReadModel) readModelEntry {
		var readers []string
		for _, id := range rm.ActorIDs {
			if a, ok := e.actorByID[id]; ok {
				readers = append(readers, a.Name)
			}
		}
		return readModelEntry{ID: rm.ID, Name: rm.Name, Readers: readers, Notes: rm.Notes}
	}
	hotSpotEntryFor := func(h session.HotSpot) hotSpotEntry {
		return hotSpotEntry{ID: h.ID, Question: h.Question, Resolved: h.Resolved, Resolution: h.Resolution}
	}

	// Index everything by context.
	aggByCtx := make(map[string][]aggEntry)
	for _, a := range e.s.Aggregates {
		aggByCtx[a.ContextID] = append(aggByCtx[a.ContextID], aggEntryFor(a))
	}
	policiesByCtx := make(map[string][]policyEntry)
	for _, p := range e.s.Policies {
		id := e.policyContext(p)
		policiesByCtx[id] = append(policiesByCtx[id], policyEntryFor(p))
	}
	readModelsByCtx := make(map[string][]readModelEntry)
	for _, rm := range e.s.ReadModels {
		readModelsByCtx[rm.ContextID] = append(readModelsByCtx[rm.ContextID], readModelEntryFor(rm))
	}
	hotSpotsByCtx := make(map[string][]hotSpotEntry)
	for _, h := range e.s.HotSpots {
		hotSpotsByCtx[h.ContextID] = append(hotSpotsByCtx[h.ContextID], hotSpotEntryFor(h))
	}
	termByCtx := make(map[string][]termEntry)
	for _, t := range e.s.Glossary {
		termByCtx[t.ContextID] = append(termByCtx[t.ContextID], termEntry{
			Term:              t.Term,
			Definition:        t.Definition,
			CanonicalPhrasing: t.CanonicalPhrasing,
			Aliases:           t.Aliases,
			SharedWith:        e.contextNames(t.SharedWith),
		})
	}

	cm := contextMapJSON{
		SessionID:               e.s.ID,
		SessionName:             e.s.Name,
		GeneratedAt:             time.Now().UTC(),
		UncategorisedAggregates: aggByCtx[""],
		UncategorisedPolicies:   policiesByCtx[""],
		UncategorisedReadModels: readModelsByCtx[""],
		UncategorisedHotSpots:   hotSpotsByCtx[""],
	}
	for _, a := range e.s.Actors {
		cm.Actors = append(cm.Actors, actorEntry{ID: a.ID, Name: a.Name, Type: string(a.Type)})
	}
	for _, ctx := range e.s.BoundedContexts {
		cm.BoundedContexts = append(cm.BoundedContexts, ctxEntry{
			ID:          ctx.ID,
			Name:        ctx.Name,
			Description: ctx.Description,
			Aggregates:  aggByCtx[ctx.ID],
			Policies:    policiesByCtx[ctx.ID],
			ReadModels:  readModelsByCtx[ctx.ID],
			HotSpots:    hotSpotsByCtx[ctx.ID],
			Glossary:    termByCtx[ctx.ID],
		})
	}

	return json.MarshalIndent(cm, "", "  ")
}
