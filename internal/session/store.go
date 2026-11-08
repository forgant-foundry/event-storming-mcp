package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"
)

func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

func dataDir() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "event-storming-mcp")
}

func sessionPath(id string) string {
	return filepath.Join(dataDir(), id, "session.json")
}

func New(name, description string) (*Session, error) {
	s := &Session{
		ID:              newID("ses"),
		Name:            name,
		Description:     description,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		Events:          []Event{},
		Commands:        []Command{},
		Actors:          []Actor{},
		Policies:        []Policy{},
		ReadModels:      []ReadModel{},
		ExternalSystems: []ExternalSystem{},
		HotSpots:        []HotSpot{},
		Aggregates:      []Aggregate{},
		BoundedContexts: []BoundedContext{},
		Glossary:        []GlossaryTerm{},
	}
	if err := Save(s); err != nil {
		return nil, err
	}
	return s, nil
}

func Load(id string) (*Session, error) {
	data, err := os.ReadFile(sessionPath(id))
	if err != nil {
		return nil, fmt.Errorf("session %s not found: %w", id, err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("corrupt session %s: %w", id, err)
	}
	reconcile(&s)
	return &s, nil
}

func Save(s *Session) error {
	path := sessionPath(s.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	reconcile(s)
	s.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	if s.SyncPath != "" {
		if err := os.MkdirAll(filepath.Dir(s.SyncPath), 0o755); err != nil {
			return fmt.Errorf("sync path mkdir: %w", err)
		}
		if err := os.WriteFile(s.SyncPath, data, 0o644); err != nil {
			return fmt.Errorf("sync path write: %w", err)
		}
	}
	return nil
}

// SetSyncPath configures a project-local file that is kept in sync with every
// save. Pass an empty string to disable syncing.
func SetSyncPath(sessionID, syncPath string) (*Session, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, err
	}
	s.SyncPath = syncPath
	if err := Save(s); err != nil {
		return nil, err
	}
	return s, nil
}

// ExportSession writes a session's JSON to destPath verbatim.
// The file is self-contained and can be committed to version control.
func ExportSession(sessionID, destPath string) error {
	data, err := os.ReadFile(sessionPath(sessionID))
	if err != nil {
		return fmt.Errorf("session %s not found: %w", sessionID, err)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destPath, data, 0o644)
}

// ImportSession reads a session JSON from srcPath, assigns it a fresh local
// session ID to avoid collisions, and saves it to the XDG store.
// Returns the new session so the caller can relay the new ID to the user.
//
// The imported session keeps its sync_path unless syncPath is non-nil, in
// which case it replaces it before the first save ("" disables syncing). A
// relative sync_path resolves against the server's working directory, so
// importing from another project should pass an override.
func ImportSession(srcPath string, syncPath *string) (*Session, string, error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, "", fmt.Errorf("cannot read %s: %w", srcPath, err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, "", fmt.Errorf("invalid session file: %w", err)
	}
	originalID := s.ID
	s.ID = newID("ses")
	if syncPath != nil {
		s.SyncPath = *syncPath
	}
	if err := Save(&s); err != nil {
		return nil, "", err
	}
	return &s, originalID, nil
}

func AddEvent(sessionID, name, notes, triggerCommandID, aggregateID string, source Source) (*Session, *Event, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	if err := checkSource(source); err != nil {
		return nil, nil, err
	}
	if err := s.checkRefs(KindCommand, triggerCommandID, KindAggregate, aggregateID); err != nil {
		return nil, nil, err
	}
	e := Event{
		ID:               newID("evt"),
		Name:             name,
		Notes:            notes,
		TriggerCommandID: triggerCommandID,
		AggregateID:      aggregateID,
		Source:           source,
		CreatedAt:        time.Now(),
	}
	s.Events = append(s.Events, e)
	if err := Save(s); err != nil {
		return nil, nil, err
	}
	return s, &s.Events[len(s.Events)-1], nil
}

func AddCommand(sessionID, name, notes, actorID, aggregateID string, preconditions []string, source Source) (*Session, *Command, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	if err := checkSource(source); err != nil {
		return nil, nil, err
	}
	if err := s.checkRefs(KindActor, actorID, KindAggregate, aggregateID); err != nil {
		return nil, nil, err
	}
	c := Command{
		ID:            newID("cmd"),
		Name:          name,
		Notes:         notes,
		ActorID:       actorID,
		AggregateID:   aggregateID,
		Preconditions: preconditions,
		Source:        source,
		CreatedAt:     time.Now(),
	}
	s.Commands = append(s.Commands, c)
	if err := Save(s); err != nil {
		return nil, nil, err
	}
	return s, &s.Commands[len(s.Commands)-1], nil
}

func AddActor(sessionID, name string, actorType ActorType, notes string) (*Session, *Actor, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	a := Actor{
		ID:        newID("act"),
		Name:      name,
		Type:      actorType,
		Notes:     notes,
		CreatedAt: time.Now(),
	}
	s.Actors = append(s.Actors, a)
	if err := Save(s); err != nil {
		return nil, nil, err
	}
	return s, &s.Actors[len(s.Actors)-1], nil
}

// SetEventCondition marks an event as conditionally emitted with the given condition
// text. The emitter will place it in a separate Scenario block rather than the
// happy-path scenario for its command.
func SetEventCondition(sessionID, eventID, condition string) (*Session, *Event, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	for i := range s.Events {
		if s.Events[i].ID == eventID {
			s.Events[i].Condition = condition
			if err := Save(s); err != nil {
				return nil, nil, err
			}
			return s, &s.Events[i], nil
		}
	}
	return nil, nil, fmt.Errorf("event %s not found", eventID)
}

// ResolveHotSpot marks a hot spot as resolved with the given resolution text.
func ResolveHotSpot(sessionID, hotSpotID, resolution string) (*Session, *HotSpot, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	for i := range s.HotSpots {
		if s.HotSpots[i].ID == hotSpotID {
			s.HotSpots[i].Resolved = true
			s.HotSpots[i].Resolution = resolution
			if err := Save(s); err != nil {
				return nil, nil, err
			}
			return s, &s.HotSpots[i], nil
		}
	}
	return nil, nil, fmt.Errorf("hot spot %s not found", hotSpotID)
}

// DeprecateAggregate marks an aggregate as deprecated. The emitter will tag its
// feature file with @deprecated and skip scenario generation.
func DeprecateAggregate(sessionID, aggregateID, reason string) (*Session, *Aggregate, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	for i := range s.Aggregates {
		if s.Aggregates[i].ID == aggregateID {
			s.Aggregates[i].Deprecated = true
			s.Aggregates[i].DeprecationReason = reason
			if err := Save(s); err != nil {
				return nil, nil, err
			}
			return s, &s.Aggregates[i], nil
		}
	}
	return nil, nil, fmt.Errorf("aggregate %s not found", aggregateID)
}

// LinkEvent sets trigger_command_id on an existing event without creating a duplicate.
func LinkEvent(sessionID, eventID, commandID string) (*Session, *Event, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	if err := s.checkRefs(KindCommand, commandID); err != nil {
		return nil, nil, err
	}
	for i := range s.Events {
		if s.Events[i].ID == eventID {
			s.Events[i].TriggerCommandID = commandID
			if err := Save(s); err != nil {
				return nil, nil, err
			}
			return s, &s.Events[i], nil
		}
	}
	return nil, nil, fmt.Errorf("event %s not found", eventID)
}

// RemoveEvent deletes an event by ID and detaches it from anything that references it.
func RemoveEvent(sessionID, eventID string) (*Session, error) {
	s, _, err := RemoveItem(sessionID, KindEvent, eventID)
	return s, err
}

func AddPolicy(sessionID, name, notes, triggerEventID, resultingCommandID, contextID, condition string, source Source) (*Session, *Policy, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	if err := checkSource(source); err != nil {
		return nil, nil, err
	}
	if err := s.checkRefs(KindEvent, triggerEventID, KindCommand, resultingCommandID, KindContext, contextID); err != nil {
		return nil, nil, err
	}
	p := Policy{
		ID:                 newID("pol"),
		Name:               name,
		Notes:              notes,
		TriggerEventID:     triggerEventID,
		ResultingCommandID: resultingCommandID,
		ContextID:          contextID,
		Condition:          condition,
		Source:             source,
		CreatedAt:          time.Now(),
	}
	s.Policies = append(s.Policies, p)
	if err := Save(s); err != nil {
		return nil, nil, err
	}
	return s, &s.Policies[len(s.Policies)-1], nil
}

func AddReadModel(sessionID, name, notes string, actorIDs []string, contextID string, source Source) (*Session, *ReadModel, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	if err := checkSource(source); err != nil {
		return nil, nil, err
	}
	if err := s.checkRefs(KindContext, contextID); err != nil {
		return nil, nil, err
	}
	for _, id := range actorIDs {
		if err := s.checkRefs(KindActor, id); err != nil {
			return nil, nil, err
		}
	}
	rm := ReadModel{
		ID:        newID("rm"),
		Name:      name,
		Notes:     notes,
		ActorIDs:  actorIDs,
		ContextID: contextID,
		Source:    source,
		CreatedAt: time.Now(),
	}
	s.ReadModels = append(s.ReadModels, rm)
	if err := Save(s); err != nil {
		return nil, nil, err
	}
	return s, &s.ReadModels[len(s.ReadModels)-1], nil
}

func AddHotSpot(sessionID, question, contextID string) (*Session, *HotSpot, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	if err := s.checkRefs(KindContext, contextID); err != nil {
		return nil, nil, err
	}
	hs := HotSpot{
		ID:        newID("hs"),
		Question:  question,
		ContextID: contextID,
		Resolved:  false,
		CreatedAt: time.Now(),
	}
	s.HotSpots = append(s.HotSpots, hs)
	if err := Save(s); err != nil {
		return nil, nil, err
	}
	return s, &s.HotSpots[len(s.HotSpots)-1], nil
}

// ClusterAggregate creates an aggregate and assigns the given events and commands to it.
// An event or command may only belong to one aggregate; assigning one that already belongs
// to a different (persisted) aggregate is an error.
func ClusterAggregate(sessionID, name, notes, contextID string, eventIDs, commandIDs []string) (*Session, *Aggregate, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	if err := s.checkRefs(KindContext, contextID); err != nil {
		return nil, nil, err
	}

	eventIdx := make(map[string]int, len(s.Events))
	for i, e := range s.Events {
		eventIdx[e.ID] = i
	}
	commandIdx := make(map[string]int, len(s.Commands))
	for i, c := range s.Commands {
		commandIdx[c.ID] = i
	}
	existingAggIDs := make(map[string]bool, len(s.Aggregates))
	for _, a := range s.Aggregates {
		existingAggIDs[a.ID] = true
	}

	for _, eid := range eventIDs {
		i, ok := eventIdx[eid]
		if !ok {
			return nil, nil, fmt.Errorf("event %s not found", eid)
		}
		if existingAggIDs[s.Events[i].AggregateID] {
			return nil, nil, fmt.Errorf("event %s already belongs to aggregate %s", eid, s.Events[i].AggregateID)
		}
	}
	for _, cid := range commandIDs {
		i, ok := commandIdx[cid]
		if !ok {
			return nil, nil, fmt.Errorf("command %s not found", cid)
		}
		if existingAggIDs[s.Commands[i].AggregateID] {
			return nil, nil, fmt.Errorf("command %s already belongs to aggregate %s", cid, s.Commands[i].AggregateID)
		}
	}

	agg := Aggregate{
		ID:         newID("agg"),
		Name:       name,
		Notes:      notes,
		ContextID:  contextID,
		EventIDs:   eventIDs,
		CommandIDs: commandIDs,
		CreatedAt:  time.Now(),
	}
	s.Aggregates = append(s.Aggregates, agg)
	aggID := s.Aggregates[len(s.Aggregates)-1].ID

	for _, eid := range eventIDs {
		s.Events[eventIdx[eid]].AggregateID = aggID
	}
	for _, cid := range commandIDs {
		s.Commands[commandIdx[cid]].AggregateID = aggID
	}

	if err := Save(s); err != nil {
		return nil, nil, err
	}
	return s, &s.Aggregates[len(s.Aggregates)-1], nil
}

// ProposeContext creates a bounded context and assigns the given aggregates (and optionally
// glossary terms) to it. An aggregate may only belong to one bounded context.
func ProposeContext(sessionID, name, description string, aggregateIDs, glossaryTermIDs []string) (*Session, *BoundedContext, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}

	aggIdx := make(map[string]int, len(s.Aggregates))
	for i, a := range s.Aggregates {
		aggIdx[a.ID] = i
	}
	termIdx := make(map[string]int, len(s.Glossary))
	for i, t := range s.Glossary {
		termIdx[t.ID] = i
	}
	existingCtxIDs := make(map[string]bool, len(s.BoundedContexts))
	for _, c := range s.BoundedContexts {
		existingCtxIDs[c.ID] = true
	}

	for _, aid := range aggregateIDs {
		i, ok := aggIdx[aid]
		if !ok {
			return nil, nil, fmt.Errorf("aggregate %s not found", aid)
		}
		if existingCtxIDs[s.Aggregates[i].ContextID] {
			return nil, nil, fmt.Errorf("aggregate %s already belongs to context %s", aid, s.Aggregates[i].ContextID)
		}
	}
	for _, tid := range glossaryTermIDs {
		if _, ok := termIdx[tid]; !ok {
			return nil, nil, fmt.Errorf("glossary term %s not found", tid)
		}
	}

	ctx := BoundedContext{
		ID:              newID("ctx"),
		Name:            name,
		Description:     description,
		AggregateIDs:    aggregateIDs,
		GlossaryTermIDs: glossaryTermIDs,
		CreatedAt:       time.Now(),
	}
	s.BoundedContexts = append(s.BoundedContexts, ctx)
	ctxID := s.BoundedContexts[len(s.BoundedContexts)-1].ID

	for _, aid := range aggregateIDs {
		s.Aggregates[aggIdx[aid]].ContextID = ctxID
	}
	for _, tid := range glossaryTermIDs {
		s.Glossary[termIdx[tid]].ContextID = ctxID
	}

	if err := Save(s); err != nil {
		return nil, nil, err
	}
	return s, &s.BoundedContexts[len(s.BoundedContexts)-1], nil
}

// DefineTerm adds a glossary term or updates it in place if the term already exists.
func DefineTerm(sessionID, term, definition, canonicalPhrasing, contextID string, aliases []string) (*Session, *GlossaryTerm, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	if err := s.checkRefs(KindContext, contextID); err != nil {
		return nil, nil, err
	}
	for i := range s.Glossary {
		if s.Glossary[i].Term == term {
			s.Glossary[i].Definition = definition
			s.Glossary[i].CanonicalPhrasing = canonicalPhrasing
			s.Glossary[i].Aliases = aliases
			s.Glossary[i].ContextID = contextID
			s.Glossary[i].UpdatedAt = time.Now()
			if err := Save(s); err != nil {
				return nil, nil, err
			}
			return s, &s.Glossary[i], nil
		}
	}
	gt := GlossaryTerm{
		ID:                newID("term"),
		Term:              term,
		Definition:        definition,
		CanonicalPhrasing: canonicalPhrasing,
		ContextID:         contextID,
		Aliases:           aliases,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	s.Glossary = append(s.Glossary, gt)
	if err := Save(s); err != nil {
		return nil, nil, err
	}
	return s, &s.Glossary[len(s.Glossary)-1], nil
}

// reconcile repairs membership that is stored on both sides: an aggregate's
// event_ids/command_ids against each member's aggregate_id, and a context's
// aggregate_ids/glossary_term_ids against each member's context_id. Sessions
// written by older versions could let the two sides drift apart. Save runs it
// too, so mutations only need to set the member side.
//
// It also folds the legacy single read model actor_id into actor_ids.
func reconcile(s *Session) {
	for i := range s.ReadModels {
		rm := &s.ReadModels[i]
		if rm.ActorID != "" {
			if !slices.Contains(rm.ActorIDs, rm.ActorID) {
				rm.ActorIDs = append([]string{rm.ActorID}, rm.ActorIDs...)
			}
			rm.ActorID = ""
		}
	}

	aggLists := make([]ownerList, len(s.Aggregates))
	aggCmdLists := make([]ownerList, len(s.Aggregates))
	for i := range s.Aggregates {
		aggLists[i] = ownerList{s.Aggregates[i].ID, &s.Aggregates[i].EventIDs}
		aggCmdLists[i] = ownerList{s.Aggregates[i].ID, &s.Aggregates[i].CommandIDs}
	}
	events := make([]member, len(s.Events))
	for i := range s.Events {
		events[i] = member{s.Events[i].ID, &s.Events[i].AggregateID}
	}
	commands := make([]member, len(s.Commands))
	for i := range s.Commands {
		commands[i] = member{s.Commands[i].ID, &s.Commands[i].AggregateID}
	}
	reconcileMembership(aggLists, events)
	reconcileMembership(aggCmdLists, commands)

	ctxAggLists := make([]ownerList, len(s.BoundedContexts))
	ctxTermLists := make([]ownerList, len(s.BoundedContexts))
	for i := range s.BoundedContexts {
		ctxAggLists[i] = ownerList{s.BoundedContexts[i].ID, &s.BoundedContexts[i].AggregateIDs}
		ctxTermLists[i] = ownerList{s.BoundedContexts[i].ID, &s.BoundedContexts[i].GlossaryTermIDs}
	}
	aggs := make([]member, len(s.Aggregates))
	for i := range s.Aggregates {
		aggs[i] = member{s.Aggregates[i].ID, &s.Aggregates[i].ContextID}
	}
	terms := make([]member, len(s.Glossary))
	for i := range s.Glossary {
		terms[i] = member{s.Glossary[i].ID, &s.Glossary[i].ContextID}
	}
	reconcileMembership(ctxAggLists, aggs)
	reconcileMembership(ctxTermLists, terms)
}

// ownerList is an owner's list of member IDs.
type ownerList struct {
	id   string
	list *[]string
}

// member is an item that points at its owner through a single ID field.
type member struct {
	id    string
	owner *string
}

// reconcileMembership makes each owner's list and each member's owner field
// agree. A member already pointing at an existing owner keeps it; a member
// with no valid owner is claimed by the first owner (in session order) whose
// list names it; owner
// fields pointing at a missing owner are cleared. Lists are rebuilt in their
// existing order, with newly found members appended in session order.
func reconcileMembership(owners []ownerList, members []member) {
	exists := make(map[string]bool, len(owners))
	for _, o := range owners {
		exists[o.id] = true
	}
	byID := make(map[string]member, len(members))
	for _, m := range members {
		byID[m.id] = m
	}
	for _, o := range owners {
		for _, id := range *o.list {
			if m, ok := byID[id]; ok && !exists[*m.owner] {
				*m.owner = o.id
			}
		}
	}
	for _, m := range members {
		if *m.owner != "" && !exists[*m.owner] {
			*m.owner = ""
		}
	}
	for _, o := range owners {
		owner, list := o.id, o.list
		seen := make(map[string]bool)
		var rebuilt []string
		for _, id := range *list {
			if m, ok := byID[id]; ok && *m.owner == owner && !seen[id] {
				seen[id] = true
				rebuilt = append(rebuilt, id)
			}
		}
		for _, m := range members {
			if *m.owner == owner && !seen[m.id] {
				seen[m.id] = true
				rebuilt = append(rebuilt, m.id)
			}
		}
		*list = rebuilt
	}
}
