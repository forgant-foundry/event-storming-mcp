package session

import "time"

// StoryMap is the story mapping half of a session (Patton, User Story Mapping):
// who the product is for, the roles that use it, what they do in narrative
// order, the details under each step, the screens the steps happen on, and the
// slices that cut the map into releases. It links into the storm by ID: a
// role is an actor, a step uses commands and read models, a screen shows read
// models and offers commands.
//
// List order is meaningful: activities and steps are in narrative order
// (left to right), stories are in rank order under their step, and slices
// are top to bottom.
type StoryMap struct {
	Frame      Frame      `json:"frame"`
	Roles      []Role     `json:"roles"`
	Activities []Activity `json:"activities"`
	Steps      []Step     `json:"steps"`
	Stories    []Story    `json:"stories"`
	Screens    []Screen   `json:"screens"`
	Slices     []Slice    `json:"slices"`
}

// Frame is the opportunity the map is for (Patton ch. 13's opportunity
// canvas): topics to discuss, not a form, so every field is optional.
type Frame struct {
	ForWhom         string   `json:"for_whom,omitempty"`
	Problems        []string `json:"problems,omitempty"`
	Today           string   `json:"today,omitempty"` // how they cope today
	SolutionIdeas   []string `json:"solution_ideas,omitempty"`
	Why             string   `json:"why,omitempty"` // why it matters to the business
	SuccessMeasures []string `json:"success_measures,omitempty"`
}

// Role adds story-mapping detail to a storm actor (Cohn ch. 3).
type Role struct {
	ID        string    `json:"id"`
	ActorID   string    `json:"actor_id"`
	Goal      string    `json:"goal,omitempty"`
	Frequency string    `json:"frequency,omitempty"` // how often they use the product
	Expertise string    `json:"expertise,omitempty"` // in the domain and with software
	Persona   string    `json:"persona,omitempty"`
	Source    Source    `json:"source,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Activity groups steps on the map's backbone ("manage my ledgers").
type Activity struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	RoleID    string    `json:"role_id,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	Source    Source    `json:"source,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Step is one thing a role does, a short verb phrase in narrative flow
// ("…and then I send a transfer"). It uses storm commands, looks at storm
// read models, happens on a screen, and may wait on another role's step.
type Step struct {
	ID           string    `json:"id"`
	ActivityID   string    `json:"activity_id"`
	RoleID       string    `json:"role_id"`
	Name         string    `json:"name"`
	CommandIDs   []string  `json:"command_ids,omitempty"`
	ReadModelIDs []string  `json:"read_model_ids,omitempty"`
	ScreenID     string    `json:"screen_id,omitempty"`
	AfterStepID  string    `json:"after_step_id,omitempty"` // a hand-off: another role's step this one waits for
	Notes        string    `json:"notes,omitempty"`
	Source       Source    `json:"source,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type StoryKind string

const (
	StoryDetail      StoryKind = "detail"
	StoryAlternative StoryKind = "alternative"
	StoryFailure     StoryKind = "failure"
	StoryMisuse      StoryKind = "misuse"
	StorySpike       StoryKind = "spike"
)

// Story is a card under a step: a detail, another way of doing it, something
// that goes wrong, a misuse, or a spike that retires a feasibility risk.
type Story struct {
	ID        string    `json:"id"`
	StepID    string    `json:"step_id"`
	Title     string    `json:"title"`
	Why       string    `json:"why,omitempty"`
	Kind      StoryKind `json:"kind"`
	SliceID   string    `json:"slice_id,omitempty"`
	EventID   string    `json:"event_id,omitempty"` // the storm's conditional event a failure story describes
	Notes     string    `json:"notes,omitempty"`
	Source    Source    `json:"source,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Screen is where steps happen: the read models it shows and the commands it
// offers (Cohn ch. 4's low-fidelity prototype). It isn't an aggregate.
type Screen struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Shows     []string  `json:"shows,omitempty"` // read model IDs
	Offers    []Offer   `json:"offers,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	Source    Source    `json:"source,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Offer is a command a screen offers, and optionally what the screen shows
// once it's done ("the new balance").
type Offer struct {
	CommandID  string `json:"command_id"`
	ShowsAfter string `json:"shows_after,omitempty"`
}

type SliceKind string

const (
	SliceRelease     SliceKind = "release"
	SliceLearning    SliceKind = "learning"
	SliceDevelopment SliceKind = "development"
)

// Slice is a horizontal cut through the map (Patton ch. 5, steps 4–6). It
// contains stories; a step is in a slice when one of its stories is. At most
// one development slice is the walking skeleton.
type Slice struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Kind            SliceKind `json:"kind"`
	Outcome         string    `json:"outcome,omitempty"`
	WalkingSkeleton bool      `json:"walking_skeleton,omitempty"`
	Notes           string    `json:"notes,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// Story-map record kinds, accepted by RemoveItem and MoveItem.
const (
	KindRole     = "role"
	KindActivity = "activity"
	KindStep     = "step"
	KindStory    = "story"
	KindScreen   = "screen"
	KindSlice    = "slice"
)

// storyMap returns the session's story map, creating it on first use.
func (s *Session) storyMap() *StoryMap {
	if s.StoryMap == nil {
		s.StoryMap = &StoryMap{}
	}
	return s.StoryMap
}
