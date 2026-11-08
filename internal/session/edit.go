package session

import (
	"fmt"
	"slices"
)

// Record kinds, as accepted by RemoveItem and used in error messages.
const (
	KindEvent     = "event"
	KindCommand   = "command"
	KindActor     = "actor"
	KindPolicy    = "policy"
	KindReadModel = "read_model"
	KindHotSpot   = "hot_spot"
	KindAggregate = "aggregate"
	KindContext   = "bounded_context"
	KindTerm      = "glossary_term"
)

// Kinds lists every kind RemoveItem accepts.
var Kinds = []string{
	KindEvent, KindCommand, KindActor, KindPolicy, KindReadModel,
	KindHotSpot, KindAggregate, KindContext, KindTerm,
}

// has reports whether a record of the given kind and ID exists.
func (s *Session) has(kind, id string) bool {
	switch kind {
	case KindEvent:
		return slices.ContainsFunc(s.Events, func(e Event) bool { return e.ID == id })
	case KindCommand:
		return slices.ContainsFunc(s.Commands, func(c Command) bool { return c.ID == id })
	case KindActor:
		return slices.ContainsFunc(s.Actors, func(a Actor) bool { return a.ID == id })
	case KindPolicy:
		return slices.ContainsFunc(s.Policies, func(p Policy) bool { return p.ID == id })
	case KindReadModel:
		return slices.ContainsFunc(s.ReadModels, func(r ReadModel) bool { return r.ID == id })
	case KindHotSpot:
		return slices.ContainsFunc(s.HotSpots, func(h HotSpot) bool { return h.ID == id })
	case KindAggregate:
		return slices.ContainsFunc(s.Aggregates, func(a Aggregate) bool { return a.ID == id })
	case KindContext:
		return slices.ContainsFunc(s.BoundedContexts, func(c BoundedContext) bool { return c.ID == id })
	case KindTerm:
		return slices.ContainsFunc(s.Glossary, func(t GlossaryTerm) bool { return t.ID == id })
	}
	return false
}

// checkRefs takes alternating kind, id pairs and returns an error for the
// first non-empty id that doesn't exist.
func (s *Session) checkRefs(pairs ...string) error {
	for i := 0; i+1 < len(pairs); i += 2 {
		kind, id := pairs[i], pairs[i+1]
		if id != "" && !s.has(kind, id) {
			return fmt.Errorf("%s %s not found", kind, id)
		}
	}
	return nil
}

func checkSource(src Source) error {
	switch src {
	case "", SourceExpert, SourceFacilitator, SourceImported:
		return nil
	}
	return fmt.Errorf("source must be one of: expert, facilitator, imported; got %q", src)
}

// CheckActorType validates an actor type.
func CheckActorType(t ActorType) error {
	switch t {
	case ActorTypePerson, ActorTypeRole, ActorTypeExternalSystem:
		return nil
	}
	return fmt.Errorf("type must be one of: person, role, external_system; got %q", t)
}

func checkName(name *string) error {
	if name != nil && *name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	return nil
}

// set copies *v into *dst when v is non-nil. Patch fields are pointers so
// that an omitted field leaves the record unchanged.
func set[T any](dst *T, v *T) {
	if v != nil {
		*dst = *v
	}
}

type EventPatch struct {
	Name, Notes *string
	Source      *Source
}

// UpdateEvent edits an event's name, notes or source. Use LinkEvent and
// SetEventCondition for its trigger and condition.
func UpdateEvent(sessionID, eventID string, p EventPatch) (*Session, *Event, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	i := slices.IndexFunc(s.Events, func(e Event) bool { return e.ID == eventID })
	if i < 0 {
		return nil, nil, fmt.Errorf("event %s not found", eventID)
	}
	if err := checkName(p.Name); err != nil {
		return nil, nil, err
	}
	if p.Source != nil {
		if err := checkSource(*p.Source); err != nil {
			return nil, nil, err
		}
	}
	e := &s.Events[i]
	set(&e.Name, p.Name)
	set(&e.Notes, p.Notes)
	set(&e.Source, p.Source)
	if err := Save(s); err != nil {
		return nil, nil, err
	}
	return s, e, nil
}

type CommandPatch struct {
	Name, Notes, ActorID *string
	Preconditions        *[]string
	Source               *Source
}

// UpdateCommand edits a command. Use AssignToAggregate to change its aggregate.
func UpdateCommand(sessionID, commandID string, p CommandPatch) (*Session, *Command, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	i := slices.IndexFunc(s.Commands, func(c Command) bool { return c.ID == commandID })
	if i < 0 {
		return nil, nil, fmt.Errorf("command %s not found", commandID)
	}
	if err := checkName(p.Name); err != nil {
		return nil, nil, err
	}
	if p.Source != nil {
		if err := checkSource(*p.Source); err != nil {
			return nil, nil, err
		}
	}
	if p.ActorID != nil {
		if err := s.checkRefs(KindActor, *p.ActorID); err != nil {
			return nil, nil, err
		}
	}
	c := &s.Commands[i]
	set(&c.Name, p.Name)
	set(&c.Notes, p.Notes)
	set(&c.ActorID, p.ActorID)
	set(&c.Preconditions, p.Preconditions)
	set(&c.Source, p.Source)
	if err := Save(s); err != nil {
		return nil, nil, err
	}
	return s, c, nil
}

type ActorPatch struct {
	Name, Notes *string
	Type        *ActorType
}

func UpdateActor(sessionID, actorID string, p ActorPatch) (*Session, *Actor, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	i := slices.IndexFunc(s.Actors, func(a Actor) bool { return a.ID == actorID })
	if i < 0 {
		return nil, nil, fmt.Errorf("actor %s not found", actorID)
	}
	if err := checkName(p.Name); err != nil {
		return nil, nil, err
	}
	if p.Type != nil {
		if err := CheckActorType(*p.Type); err != nil {
			return nil, nil, err
		}
	}
	a := &s.Actors[i]
	set(&a.Name, p.Name)
	set(&a.Notes, p.Notes)
	set(&a.Type, p.Type)
	if err := Save(s); err != nil {
		return nil, nil, err
	}
	return s, a, nil
}

type PolicyPatch struct {
	Name, Notes, TriggerEventID, ResultingCommandID, ContextID, Condition *string
	Source                                                                *Source
}

func UpdatePolicy(sessionID, policyID string, p PolicyPatch) (*Session, *Policy, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	i := slices.IndexFunc(s.Policies, func(p Policy) bool { return p.ID == policyID })
	if i < 0 {
		return nil, nil, fmt.Errorf("policy %s not found", policyID)
	}
	if err := checkName(p.Name); err != nil {
		return nil, nil, err
	}
	if p.Source != nil {
		if err := checkSource(*p.Source); err != nil {
			return nil, nil, err
		}
	}
	deref := func(v *string) string {
		if v == nil {
			return ""
		}
		return *v
	}
	if err := s.checkRefs(
		KindEvent, deref(p.TriggerEventID),
		KindCommand, deref(p.ResultingCommandID),
		KindContext, deref(p.ContextID),
	); err != nil {
		return nil, nil, err
	}
	pol := &s.Policies[i]
	set(&pol.Name, p.Name)
	set(&pol.Notes, p.Notes)
	set(&pol.TriggerEventID, p.TriggerEventID)
	set(&pol.ResultingCommandID, p.ResultingCommandID)
	set(&pol.ContextID, p.ContextID)
	set(&pol.Condition, p.Condition)
	set(&pol.Source, p.Source)
	if err := Save(s); err != nil {
		return nil, nil, err
	}
	return s, pol, nil
}

type ReadModelPatch struct {
	Name, Notes, ContextID *string
	ActorIDs               *[]string
	Source                 *Source
}

func UpdateReadModel(sessionID, readModelID string, p ReadModelPatch) (*Session, *ReadModel, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	i := slices.IndexFunc(s.ReadModels, func(r ReadModel) bool { return r.ID == readModelID })
	if i < 0 {
		return nil, nil, fmt.Errorf("read model %s not found", readModelID)
	}
	if err := checkName(p.Name); err != nil {
		return nil, nil, err
	}
	if p.Source != nil {
		if err := checkSource(*p.Source); err != nil {
			return nil, nil, err
		}
	}
	if p.ContextID != nil {
		if err := s.checkRefs(KindContext, *p.ContextID); err != nil {
			return nil, nil, err
		}
	}
	if p.ActorIDs != nil {
		for _, id := range *p.ActorIDs {
			if err := s.checkRefs(KindActor, id); err != nil {
				return nil, nil, err
			}
		}
	}
	rm := &s.ReadModels[i]
	set(&rm.Name, p.Name)
	set(&rm.Notes, p.Notes)
	set(&rm.ContextID, p.ContextID)
	set(&rm.ActorIDs, p.ActorIDs)
	set(&rm.Source, p.Source)
	if err := Save(s); err != nil {
		return nil, nil, err
	}
	return s, rm, nil
}

type AggregatePatch struct {
	Name, Notes *string
}

// UpdateAggregate edits an aggregate's name or notes. Use AssignToAggregate to
// change its members and AssignToContext to change its context.
func UpdateAggregate(sessionID, aggregateID string, p AggregatePatch) (*Session, *Aggregate, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	i := slices.IndexFunc(s.Aggregates, func(a Aggregate) bool { return a.ID == aggregateID })
	if i < 0 {
		return nil, nil, fmt.Errorf("aggregate %s not found", aggregateID)
	}
	if err := checkName(p.Name); err != nil {
		return nil, nil, err
	}
	a := &s.Aggregates[i]
	set(&a.Name, p.Name)
	set(&a.Notes, p.Notes)
	if err := Save(s); err != nil {
		return nil, nil, err
	}
	return s, a, nil
}

type ContextPatch struct {
	Name, Description *string
}

func UpdateContext(sessionID, contextID string, p ContextPatch) (*Session, *BoundedContext, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	i := slices.IndexFunc(s.BoundedContexts, func(c BoundedContext) bool { return c.ID == contextID })
	if i < 0 {
		return nil, nil, fmt.Errorf("bounded context %s not found", contextID)
	}
	if err := checkName(p.Name); err != nil {
		return nil, nil, err
	}
	c := &s.BoundedContexts[i]
	set(&c.Name, p.Name)
	set(&c.Description, p.Description)
	if err := Save(s); err != nil {
		return nil, nil, err
	}
	return s, c, nil
}

// AssignToAggregate adds existing events and commands to an existing
// aggregate. An item already in a different aggregate is an error unless move
// is set, in which case it leaves the old aggregate.
func AssignToAggregate(sessionID, aggregateID string, eventIDs, commandIDs []string, move bool) (*Session, *Aggregate, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	ai := slices.IndexFunc(s.Aggregates, func(a Aggregate) bool { return a.ID == aggregateID })
	if ai < 0 {
		return nil, nil, fmt.Errorf("aggregate %s not found", aggregateID)
	}
	var owners []*string
	for _, id := range eventIDs {
		i := slices.IndexFunc(s.Events, func(e Event) bool { return e.ID == id })
		if i < 0 {
			return nil, nil, fmt.Errorf("event %s not found", id)
		}
		owners = append(owners, &s.Events[i].AggregateID)
	}
	for _, id := range commandIDs {
		i := slices.IndexFunc(s.Commands, func(c Command) bool { return c.ID == id })
		if i < 0 {
			return nil, nil, fmt.Errorf("command %s not found", id)
		}
		owners = append(owners, &s.Commands[i].AggregateID)
	}
	ids := append(slices.Clone(eventIDs), commandIDs...)
	if err := assign(s, KindAggregate, aggregateID, ids, owners, move); err != nil {
		return nil, nil, err
	}
	if err := Save(s); err != nil {
		return nil, nil, err
	}
	return s, &s.Aggregates[ai], nil
}

// AssignToContext adds existing aggregates and glossary terms to an existing
// bounded context, with the same move rule as AssignToAggregate.
func AssignToContext(sessionID, contextID string, aggregateIDs, termIDs []string, move bool) (*Session, *BoundedContext, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	ci := slices.IndexFunc(s.BoundedContexts, func(c BoundedContext) bool { return c.ID == contextID })
	if ci < 0 {
		return nil, nil, fmt.Errorf("bounded context %s not found", contextID)
	}
	var owners []*string
	for _, id := range aggregateIDs {
		i := slices.IndexFunc(s.Aggregates, func(a Aggregate) bool { return a.ID == id })
		if i < 0 {
			return nil, nil, fmt.Errorf("aggregate %s not found", id)
		}
		owners = append(owners, &s.Aggregates[i].ContextID)
	}
	for _, id := range termIDs {
		i := slices.IndexFunc(s.Glossary, func(t GlossaryTerm) bool { return t.ID == id })
		if i < 0 {
			return nil, nil, fmt.Errorf("glossary term %s not found", id)
		}
		owners = append(owners, &s.Glossary[i].ContextID)
	}
	ids := append(slices.Clone(aggregateIDs), termIDs...)
	if err := assign(s, KindContext, contextID, ids, owners, move); err != nil {
		return nil, nil, err
	}
	if err := Save(s); err != nil {
		return nil, nil, err
	}
	return s, &s.BoundedContexts[ci], nil
}

// assign points each owner field at ownerID, refusing to take an item from
// another existing owner unless move is set. All checks run before any write.
// Save's reconcile then updates the owners' lists.
func assign(s *Session, ownerKind, ownerID string, ids []string, owners []*string, move bool) error {
	if !move {
		for i, o := range owners {
			if *o != "" && *o != ownerID && s.has(ownerKind, *o) {
				return fmt.Errorf("%s already belongs to %s %s; pass move=true to move it", ids[i], ownerKind, *o)
			}
		}
	}
	for _, o := range owners {
		*o = ownerID
	}
	return nil
}

// RemoveItem deletes a record of the given kind and detaches it from every
// record that references it. It returns a description of each detachment so
// the facilitator can tell the expert what else changed.
func RemoveItem(sessionID, kind, id string) (*Session, []string, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, nil, err
	}
	if !slices.Contains(Kinds, kind) {
		return nil, nil, fmt.Errorf("kind must be one of %v; got %q", Kinds, kind)
	}
	if !s.has(kind, id) {
		return nil, nil, fmt.Errorf("%s %s not found", kind, id)
	}

	var detached []string
	detach := func(field *string, what, name, recID, fieldName string) {
		if *field == id {
			*field = ""
			detached = append(detached, fmt.Sprintf("%s %q (%s): %s cleared", what, name, recID, fieldName))
		}
	}

	switch kind {
	case KindEvent:
		s.Events = slices.DeleteFunc(s.Events, func(e Event) bool { return e.ID == id })
		for i := range s.Policies {
			p := &s.Policies[i]
			detach(&p.TriggerEventID, KindPolicy, p.Name, p.ID, "trigger_event_id")
		}
	case KindCommand:
		s.Commands = slices.DeleteFunc(s.Commands, func(c Command) bool { return c.ID == id })
		for i := range s.Events {
			e := &s.Events[i]
			detach(&e.TriggerCommandID, KindEvent, e.Name, e.ID, "trigger_command_id")
		}
		for i := range s.Policies {
			p := &s.Policies[i]
			detach(&p.ResultingCommandID, KindPolicy, p.Name, p.ID, "resulting_command_id")
		}
	case KindActor:
		s.Actors = slices.DeleteFunc(s.Actors, func(a Actor) bool { return a.ID == id })
		for i := range s.Commands {
			c := &s.Commands[i]
			detach(&c.ActorID, KindCommand, c.Name, c.ID, "actor_id")
		}
		for i := range s.ReadModels {
			rm := &s.ReadModels[i]
			if slices.Contains(rm.ActorIDs, id) {
				rm.ActorIDs = slices.DeleteFunc(rm.ActorIDs, func(a string) bool { return a == id })
				detached = append(detached, fmt.Sprintf("%s %q (%s): removed from actor_ids", KindReadModel, rm.Name, rm.ID))
			}
		}
	case KindPolicy:
		s.Policies = slices.DeleteFunc(s.Policies, func(p Policy) bool { return p.ID == id })
	case KindReadModel:
		s.ReadModels = slices.DeleteFunc(s.ReadModels, func(r ReadModel) bool { return r.ID == id })
	case KindHotSpot:
		s.HotSpots = slices.DeleteFunc(s.HotSpots, func(h HotSpot) bool { return h.ID == id })
	case KindAggregate:
		s.Aggregates = slices.DeleteFunc(s.Aggregates, func(a Aggregate) bool { return a.ID == id })
		for i := range s.Events {
			e := &s.Events[i]
			detach(&e.AggregateID, KindEvent, e.Name, e.ID, "aggregate_id")
		}
		for i := range s.Commands {
			c := &s.Commands[i]
			detach(&c.AggregateID, KindCommand, c.Name, c.ID, "aggregate_id")
		}
	case KindContext:
		s.BoundedContexts = slices.DeleteFunc(s.BoundedContexts, func(c BoundedContext) bool { return c.ID == id })
		for i := range s.Aggregates {
			a := &s.Aggregates[i]
			detach(&a.ContextID, KindAggregate, a.Name, a.ID, "context_id")
		}
		for i := range s.Glossary {
			t := &s.Glossary[i]
			detach(&t.ContextID, KindTerm, t.Term, t.ID, "context_id")
		}
		for i := range s.Events {
			e := &s.Events[i]
			detach(&e.ContextID, KindEvent, e.Name, e.ID, "context_id")
		}
		for i := range s.Policies {
			p := &s.Policies[i]
			detach(&p.ContextID, KindPolicy, p.Name, p.ID, "context_id")
		}
		for i := range s.ReadModels {
			rm := &s.ReadModels[i]
			detach(&rm.ContextID, KindReadModel, rm.Name, rm.ID, "context_id")
		}
		for i := range s.HotSpots {
			h := &s.HotSpots[i]
			detach(&h.ContextID, KindHotSpot, h.Question, h.ID, "context_id")
		}
	case KindTerm:
		s.Glossary = slices.DeleteFunc(s.Glossary, func(t GlossaryTerm) bool { return t.ID == id })
	}

	if err := Save(s); err != nil {
		return nil, nil, err
	}
	return s, detached, nil
}
