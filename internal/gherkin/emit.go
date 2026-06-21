package gherkin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/forgant-foundry/event-storming-mcp/internal/session"
)

// Summary is returned by Emit and describes what was written.
type Summary struct {
	FeatureFiles   []string `json:"feature_files"`
	GlossaryFile   string   `json:"glossary_file"`
	ContextMapFile string   `json:"context_map_file"`
	ScenarioCount  int      `json:"scenario_count"`
	Warnings       []string `json:"warnings,omitempty"`
}

// Emit writes the output triple to outDir:
//   - one .feature file per aggregate
//   - glossary.md
//   - context-map.json
func Emit(s *session.Session, outDir string) (*Summary, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create output dir: %w", err)
	}

	// Build lookup tables.
	contextByID := make(map[string]session.BoundedContext, len(s.BoundedContexts))
	for _, c := range s.BoundedContexts {
		contextByID[c.ID] = c
	}
	actorByID := make(map[string]session.Actor, len(s.Actors))
	for _, a := range s.Actors {
		actorByID[a.ID] = a
	}
	eventByID := make(map[string]session.Event, len(s.Events))
	for _, e := range s.Events {
		eventByID[e.ID] = e
	}
	// Events keyed by the command that triggered them.
	eventsByCmd := make(map[string][]session.Event)
	for _, e := range s.Events {
		if e.TriggerCommandID != "" {
			eventsByCmd[e.TriggerCommandID] = append(eventsByCmd[e.TriggerCommandID], e)
		}
	}
	// Commands keyed by the aggregate they target.
	commandsByAgg := make(map[string][]session.Command)
	for _, c := range s.Commands {
		if c.AggregateID != "" {
			commandsByAgg[c.AggregateID] = append(commandsByAgg[c.AggregateID], c)
		}
	}

	sum := &Summary{}

	for _, agg := range s.Aggregates {
		ctx, hasCtx := contextByID[agg.ContextID]

		ctxDirName := "uncategorised"
		ctxTagName := "unknown"
		if hasCtx {
			ctxDirName = Slug(ctx.Name)
			ctxTagName = ctx.Name
		}

		dir := filepath.Join(outDir, ctxDirName)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}

		// Collect aggregate events for the stub case.
		var aggEvents []session.Event
		for _, eid := range agg.EventIDs {
			if e, ok := eventByID[eid]; ok {
				aggEvents = append(aggEvents, e)
			}
		}

		content, warns, n := buildFeature(agg, ctxTagName, commandsByAgg[agg.ID], aggEvents, eventsByCmd, actorByID)
		sum.Warnings = append(sum.Warnings, warns...)
		sum.ScenarioCount += n

		path := filepath.Join(dir, Slug(agg.Name)+".feature")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return nil, err
		}
		sum.FeatureFiles = append(sum.FeatureFiles, path)
	}

	// glossary.md
	gPath := filepath.Join(outDir, "glossary.md")
	if err := os.WriteFile(gPath, []byte(buildGlossary(s, contextByID)), 0o644); err != nil {
		return nil, err
	}
	sum.GlossaryFile = gPath

	// context-map.json
	cmPath := filepath.Join(outDir, "context-map.json")
	cmData, err := buildContextMap(s, contextByID, eventByID, commandsByAgg)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(cmPath, cmData, 0o644); err != nil {
		return nil, err
	}
	sum.ContextMapFile = cmPath

	return sum, nil
}

func buildFeature(
	agg session.Aggregate,
	ctxTag string,
	commands []session.Command,
	aggEvents []session.Event,
	eventsByCmd map[string][]session.Event,
	actorByID map[string]session.Actor,
) (content string, warnings []string, scenarioCount int) {
	var b strings.Builder

	tags := fmt.Sprintf("@context:%s @aggregate:%s", ctxTag, agg.Name)
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

	for _, cmd := range commands {
		allEvents := eventsByCmd[cmd.ID]
		if len(allEvents) == 0 {
			warnings = append(warnings, fmt.Sprintf(
				"command %q (%s) has no linked events — scenario omitted; add events with trigger_command_id=%s",
				cmd.Name, cmd.ID, cmd.ID,
			))
			continue
		}

		actor := actorByID[cmd.ActorID]

		// Partition into unconditional (happy-path) and conditional events.
		var unconditional, conditional []session.Event
		for _, e := range allEvents {
			if e.Condition == "" {
				unconditional = append(unconditional, e)
			} else {
				conditional = append(conditional, e)
			}
		}

		// Happy-path scenario (unconditional events only).
		if len(unconditional) > 0 {
			scenarioCount++
			b.WriteString("\n")
			b.WriteString(fmt.Sprintf("  Scenario: %s\n", ScenarioTitle(cmd.Name, actor.Name)))
			for _, pre := range cmd.Preconditions {
				b.WriteString(fmt.Sprintf("    Given %s\n", pre))
			}
			b.WriteString(fmt.Sprintf("    When %s\n", WhenStep(cmd.Name, actor.Name)))
			for i, evt := range unconditional {
				kw := "Then"
				if i > 0 {
					kw = "And "
				}
				b.WriteString(fmt.Sprintf("    %s %s\n", kw, ThenStep(evt.Name)))
			}
		}

		// One extra scenario per conditional event.
		for _, evt := range conditional {
			scenarioCount++
			b.WriteString("\n")
			title := ScenarioTitle(cmd.Name, actor.Name) + " (" + strings.ToLower(evt.Condition) + ")"
			b.WriteString(fmt.Sprintf("  Scenario: %s\n", title))
			for _, pre := range cmd.Preconditions {
				b.WriteString(fmt.Sprintf("    Given %s\n", pre))
			}
			b.WriteString(fmt.Sprintf("    And %s\n", evt.Condition))
			b.WriteString(fmt.Sprintf("    When %s\n", WhenStep(cmd.Name, actor.Name)))
			b.WriteString(fmt.Sprintf("    Then %s\n", ThenStep(evt.Name)))
		}

		// If every event was conditional, warn that there is no happy-path scenario.
		if len(unconditional) == 0 {
			warnings = append(warnings, fmt.Sprintf(
				"command %q has only conditional events — no happy-path scenario emitted",
				cmd.Name,
			))
		}
	}

	// If no commands were emitted but events exist, emit a stub.
	if scenarioCount == 0 {
		if len(aggEvents) > 0 {
			warnings = append(warnings, fmt.Sprintf(
				"aggregate %q has no commands with linked events; emitting event stub",
				agg.Name,
			))
			b.WriteString("\n  # TODO: add commands and link events via trigger_command_id\n")
			b.WriteString("  Scenario: [uncaptured commands]\n")
			b.WriteString("    Given [preconditions unknown]\n")
			b.WriteString("    When [command unknown]\n")
			for i, evt := range aggEvents {
				kw := "Then"
				if i > 0 {
					kw = "And "
				}
				b.WriteString(fmt.Sprintf("    %s %s\n", kw, ThenStep(evt.Name)))
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

func buildGlossary(s *session.Session, contextByID map[string]session.BoundedContext) string {
	var b strings.Builder
	b.WriteString("# Glossary\n\n")
	b.WriteString("Terms defined during this event storming session.\n\n")

	// Group terms by context.
	byContext := make(map[string][]session.GlossaryTerm)
	for _, t := range s.Glossary {
		byContext[t.ContextID] = append(byContext[t.ContextID], t)
	}

	// Emit each context's terms.
	for _, ctx := range s.BoundedContexts {
		terms, ok := byContext[ctx.ID]
		if !ok {
			continue
		}
		b.WriteString(fmt.Sprintf("## %s\n\n", ctx.Name))
		b.WriteString("| Term | Definition | Canonical Phrasing | Aliases |\n")
		b.WriteString("|---|---|---|---|\n")
		for _, t := range terms {
			aliases := strings.Join(t.Aliases, ", ")
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				t.Term, t.Definition, t.CanonicalPhrasing, aliases))
		}
		b.WriteString("\n")
		delete(byContext, ctx.ID)
	}

	// Terms not assigned to any context.
	if unassigned := byContext[""]; len(unassigned) > 0 {
		b.WriteString("## (No context)\n\n")
		b.WriteString("| Term | Definition | Canonical Phrasing | Aliases |\n")
		b.WriteString("|---|---|---|---|\n")
		for _, t := range unassigned {
			aliases := strings.Join(t.Aliases, ", ")
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				t.Term, t.Definition, t.CanonicalPhrasing, aliases))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// contextMapJSON is the structure written to context-map.json.
type contextMapJSON struct {
	SessionID            string          `json:"session_id"`
	SessionName          string          `json:"session_name"`
	GeneratedAt          time.Time       `json:"generated_at"`
	BoundedContexts      []ctxEntry      `json:"bounded_contexts"`
	UncategorisedAggregates []aggEntry   `json:"uncategorised_aggregates,omitempty"`
}

type ctxEntry struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Aggregates  []aggEntry    `json:"aggregates"`
	Glossary    []termEntry   `json:"glossary_terms,omitempty"`
}

type aggEntry struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Events   []string `json:"events"`
	Commands []string `json:"commands"`
}

type termEntry struct {
	Term              string   `json:"term"`
	Definition        string   `json:"definition"`
	CanonicalPhrasing string   `json:"canonical_phrasing,omitempty"`
	Aliases           []string `json:"aliases,omitempty"`
}

func buildContextMap(
	s *session.Session,
	contextByID map[string]session.BoundedContext,
	eventByID map[string]session.Event,
	commandsByAgg map[string][]session.Command,
) ([]byte, error) {
	aggEntryFor := func(agg session.Aggregate) aggEntry {
		var events, commands []string
		for _, eid := range agg.EventIDs {
			if e, ok := eventByID[eid]; ok {
				events = append(events, e.Name)
			}
		}
		for _, cmd := range commandsByAgg[agg.ID] {
			commands = append(commands, cmd.Name)
		}
		return aggEntry{ID: agg.ID, Name: agg.Name, Events: events, Commands: commands}
	}

	termEntryFor := func(t session.GlossaryTerm) termEntry {
		return termEntry{
			Term:              t.Term,
			Definition:        t.Definition,
			CanonicalPhrasing: t.CanonicalPhrasing,
			Aliases:           t.Aliases,
		}
	}

	// Index aggregates and terms by context.
	aggByCtx := make(map[string][]session.Aggregate)
	for _, a := range s.Aggregates {
		aggByCtx[a.ContextID] = append(aggByCtx[a.ContextID], a)
	}
	termByCtx := make(map[string][]session.GlossaryTerm)
	for _, t := range s.Glossary {
		termByCtx[t.ContextID] = append(termByCtx[t.ContextID], t)
	}

	cm := contextMapJSON{
		SessionID:   s.ID,
		SessionName: s.Name,
		GeneratedAt: time.Now().UTC(),
	}

	for _, ctx := range s.BoundedContexts {
		entry := ctxEntry{
			ID:          ctx.ID,
			Name:        ctx.Name,
			Description: ctx.Description,
		}
		for _, agg := range aggByCtx[ctx.ID] {
			entry.Aggregates = append(entry.Aggregates, aggEntryFor(agg))
		}
		for _, t := range termByCtx[ctx.ID] {
			entry.Glossary = append(entry.Glossary, termEntryFor(t))
		}
		cm.BoundedContexts = append(cm.BoundedContexts, entry)
	}

	for _, agg := range aggByCtx[""] {
		cm.UncategorisedAggregates = append(cm.UncategorisedAggregates, aggEntryFor(agg))
	}

	return json.MarshalIndent(cm, "", "  ")
}
