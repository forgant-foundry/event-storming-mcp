package session

import "time"

type ActorType string

const (
	ActorTypePerson         ActorType = "person"
	ActorTypeRole           ActorType = "role"
	ActorTypeExternalSystem ActorType = "external_system"
)

// Source records who a sticky note came from: what the domain expert said,
// what the facilitator proposed and the expert confirmed, or what was seeded
// from existing material.
type Source string

const (
	SourceExpert      Source = "expert"
	SourceFacilitator Source = "facilitator"
	SourceImported    Source = "imported"
)

type Event struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	TriggerCommandID string    `json:"trigger_command_id,omitempty"`
	AggregateID      string    `json:"aggregate_id,omitempty"`
	ContextID        string    `json:"context_id,omitempty"`
	Notes            string    `json:"notes,omitempty"`
	Condition        string    `json:"condition,omitempty"` // non-empty = conditionally emitted; emitted as a separate Scenario with this as an additional Given
	Negates          string    `json:"negates,omitempty"`   // the trigger command's precondition that Condition negates; dropped from this event's scenario
	Phrasing         string    `json:"phrasing,omitempty"`  // Then step text used verbatim instead of one derived from Name
	Source           Source    `json:"source,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type Command struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	ActorID       string    `json:"actor_id,omitempty"`
	AggregateID   string    `json:"aggregate_id,omitempty"`
	Preconditions []string  `json:"preconditions,omitempty"`
	Notes         string    `json:"notes,omitempty"`
	Phrasing      string    `json:"phrasing,omitempty"` // When step text (including the actor) used verbatim instead of one derived from Name
	Source        Source    `json:"source,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type Actor struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      ActorType `json:"type"`
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Policy struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	TriggerEventID     string    `json:"trigger_event_id,omitempty"`
	TriggerTime        string    `json:"trigger_time,omitempty"` // a moment in time that triggers the policy instead of an event, e.g. "a calendar month ends"
	ResultingCommandID string    `json:"resulting_command_id,omitempty"`
	ContextID          string    `json:"context_id,omitempty"`
	Condition          string    `json:"condition,omitempty"` // the policy applies only when this holds, e.g. "the assessment found no gaps"
	Notes              string    `json:"notes,omitempty"`
	Source             Source    `json:"source,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

type ReadModel struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ActorIDs  []string  `json:"actor_ids,omitempty"`
	ActorID   string    `json:"actor_id,omitempty"` // legacy single reader; folded into ActorIDs on load
	ContextID string    `json:"context_id,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	Source    Source    `json:"source,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type ExternalSystem struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type HotSpot struct {
	ID         string           `json:"id"`
	Question   string           `json:"question"`
	ContextID  string           `json:"context_id,omitempty"`
	Resolved   bool             `json:"resolved"`
	Resolution string           `json:"resolution,omitempty"`
	History    []PastResolution `json:"history,omitempty"` // earlier resolutions, oldest first, each reopened
	CreatedAt  time.Time        `json:"created_at"`
}

// PastResolution is a resolution that was later reopened.
type PastResolution struct {
	Resolution      string    `json:"resolution"`
	ReopenedBecause string    `json:"reopened_because"`
	ReopenedAt      time.Time `json:"reopened_at"`
}

type Aggregate struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	ContextID         string    `json:"context_id,omitempty"`
	CommandIDs        []string  `json:"command_ids,omitempty"`
	EventIDs          []string  `json:"event_ids,omitempty"`
	Preconditions     []string  `json:"preconditions,omitempty"` // hold for every command on this aggregate; emitted as Background
	Notes             string    `json:"notes,omitempty"`
	Deprecated        bool      `json:"deprecated,omitempty"`
	DeprecationReason string    `json:"deprecation_reason,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

type BoundedContext struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description,omitempty"`
	AggregateIDs    []string  `json:"aggregate_ids,omitempty"`
	GlossaryTermIDs []string  `json:"glossary_term_ids,omitempty"`
	Preconditions   []string  `json:"preconditions,omitempty"` // hold for every command in this context; emitted as Background
	CreatedAt       time.Time `json:"created_at"`
}

type GlossaryTerm struct {
	ID                string    `json:"id"`
	Term              string    `json:"term"`
	Definition        string    `json:"definition"`
	CanonicalPhrasing string    `json:"canonical_phrasing,omitempty"`
	Aliases           []string  `json:"aliases,omitempty"`
	ContextID         string    `json:"context_id,omitempty"`
	SharedWith        []string  `json:"shared_with,omitempty"` // other contexts that use this term on purpose (shared kernel / published language)
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type Session struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SyncPath    string `json:"sync_path,omitempty"`
	// Revision counts saves. Save refuses to overwrite a file whose revision
	// has moved on since this copy was loaded, which is how the events and
	// stories servers avoid losing each other's writes to one session.
	Revision       int `json:"revision"`
	loadedRevision int
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	Events          []Event          `json:"events"`
	Commands        []Command        `json:"commands"`
	Actors          []Actor          `json:"actors"`
	Policies        []Policy         `json:"policies"`
	ReadModels      []ReadModel      `json:"read_models"`
	ExternalSystems []ExternalSystem `json:"external_systems"`
	HotSpots        []HotSpot        `json:"hot_spots"`
	Aggregates      []Aggregate      `json:"aggregates"`
	BoundedContexts []BoundedContext `json:"bounded_contexts"`
	Glossary        []GlossaryTerm   `json:"glossary"`

	StoryMap *StoryMap `json:"story_map,omitempty"`
}
