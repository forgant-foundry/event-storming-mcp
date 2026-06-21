package session

import "time"

type ActorType string

const (
	ActorTypePerson         ActorType = "person"
	ActorTypeRole           ActorType = "role"
	ActorTypeExternalSystem ActorType = "external_system"
)

type Event struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	TriggerCommandID string    `json:"trigger_command_id,omitempty"`
	AggregateID      string    `json:"aggregate_id,omitempty"`
	ContextID        string    `json:"context_id,omitempty"`
	Notes            string    `json:"notes,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type Command struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	ActorID       string    `json:"actor_id,omitempty"`
	AggregateID   string    `json:"aggregate_id,omitempty"`
	Preconditions []string  `json:"preconditions,omitempty"`
	Notes         string    `json:"notes,omitempty"`
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
	ResultingCommandID string    `json:"resulting_command_id,omitempty"`
	ContextID          string    `json:"context_id,omitempty"`
	Notes              string    `json:"notes,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

type ReadModel struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ActorID   string    `json:"actor_id,omitempty"`
	ContextID string    `json:"context_id,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type ExternalSystem struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type HotSpot struct {
	ID        string    `json:"id"`
	Question  string    `json:"question"`
	ContextID string    `json:"context_id,omitempty"`
	Resolved  bool      `json:"resolved"`
	CreatedAt time.Time `json:"created_at"`
}

type Aggregate struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	ContextID  string    `json:"context_id,omitempty"`
	CommandIDs []string  `json:"command_ids,omitempty"`
	EventIDs   []string  `json:"event_ids,omitempty"`
	Notes      string    `json:"notes,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type BoundedContext struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description,omitempty"`
	AggregateIDs    []string  `json:"aggregate_ids,omitempty"`
	GlossaryTermIDs []string  `json:"glossary_term_ids,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type GlossaryTerm struct {
	ID                string    `json:"id"`
	Term              string    `json:"term"`
	Definition        string    `json:"definition"`
	CanonicalPhrasing string    `json:"canonical_phrasing,omitempty"`
	Aliases           []string  `json:"aliases,omitempty"`
	ContextID         string    `json:"context_id,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type Session struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

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
}
