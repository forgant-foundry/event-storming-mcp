package session

import (
	"fmt"
	"slices"
	"time"
)

func isStoryMapKind(kind string) bool {
	switch kind {
	case KindRole, KindActivity, KindStep, KindStory, KindScreen, KindSlice, KindConstraint:
		return true
	}
	return false
}

// mutateMap loads a session, runs fn on its story map, and saves it.
func mutateMap(sessionID string, fn func(s *Session, m *StoryMap) error) (*Session, error) {
	s, err := Load(sessionID)
	if err != nil {
		return nil, err
	}
	if err := fn(s, s.storyMap()); err != nil {
		return nil, err
	}
	if err := Save(s); err != nil {
		return nil, err
	}
	return s, nil
}

// insertBefore inserts item before the element with beforeID, or appends it
// when beforeID is empty. sameGroup, when set, rejects a beforeID outside the
// item's group (a step under another activity, a story under another step).
func insertBefore[T any](list []T, item T, beforeID string, id func(T) string, sameGroup func(T) bool) ([]T, error) {
	if beforeID == "" {
		return append(list, item), nil
	}
	i := slices.IndexFunc(list, func(x T) bool { return id(x) == beforeID })
	if i < 0 {
		return nil, fmt.Errorf("before_id %s not found", beforeID)
	}
	if sameGroup != nil && !sameGroup(list[i]) {
		return nil, fmt.Errorf("before_id %s is in a different group; it must share this item's activity or step", beforeID)
	}
	return slices.Insert(list, i, item), nil
}

func checkNonEmpty(field, v string) error {
	if v == "" {
		return fmt.Errorf("%s cannot be empty", field)
	}
	return nil
}

// FramePatch edits the map's frame; omitted fields are unchanged.
type FramePatch struct {
	ForWhom, Today, Why                      *string
	Problems, SolutionIdeas, SuccessMeasures *[]string
}

func SetFrame(sessionID string, p FramePatch) (*Session, *Frame, error) {
	s, err := mutateMap(sessionID, func(_ *Session, m *StoryMap) error {
		f := &m.Frame
		set(&f.ForWhom, p.ForWhom)
		set(&f.Today, p.Today)
		set(&f.Why, p.Why)
		set(&f.Problems, p.Problems)
		set(&f.SolutionIdeas, p.SolutionIdeas)
		set(&f.SuccessMeasures, p.SuccessMeasures)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s, &s.StoryMap.Frame, nil
}

// AddRole records a role for an existing actor, or creates the actor from
// actorName and actorType when r.ActorID is empty. An actor has at most one
// role.
func AddRole(sessionID string, r Role, actorName string, actorType ActorType) (*Session, *Role, error) {
	var out *Role
	s, err := mutateMap(sessionID, func(s *Session, m *StoryMap) error {
		if err := checkSource(r.Source); err != nil {
			return err
		}
		if r.ActorID == "" {
			if actorName == "" {
				return fmt.Errorf("pass actor_id for an existing actor, or actor_name to create one")
			}
			if i := slices.IndexFunc(s.Actors, func(a Actor) bool { return a.Name == actorName }); i >= 0 {
				return fmt.Errorf("actor %q already exists (%s); pass it as actor_id", actorName, s.Actors[i].ID)
			}
			if actorType == "" {
				actorType = ActorTypeRole
			}
			if err := CheckActorType(actorType); err != nil {
				return err
			}
			a := Actor{ID: newID("act"), Name: actorName, Type: actorType, CreatedAt: time.Now()}
			s.Actors = append(s.Actors, a)
			r.ActorID = a.ID
		} else if err := s.checkRefs(KindActor, r.ActorID); err != nil {
			return err
		}
		if i := slices.IndexFunc(m.Roles, func(x Role) bool { return x.ActorID == r.ActorID }); i >= 0 {
			return fmt.Errorf("actor %s already has role %s; update_role instead", r.ActorID, m.Roles[i].ID)
		}
		r.ID = newID("role")
		r.CreatedAt = time.Now()
		m.Roles = append(m.Roles, r)
		out = &m.Roles[len(m.Roles)-1]
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s, out, nil
}

type RolePatch struct {
	Goal, Frequency, Expertise, Persona *string
	Source                              *Source
}

func UpdateRole(sessionID, roleID string, p RolePatch) (*Session, *Role, error) {
	var out *Role
	s, err := mutateMap(sessionID, func(_ *Session, m *StoryMap) error {
		i := slices.IndexFunc(m.Roles, func(r Role) bool { return r.ID == roleID })
		if i < 0 {
			return fmt.Errorf("role %s not found", roleID)
		}
		r := m.Roles[i]
		set(&r.Goal, p.Goal)
		set(&r.Frequency, p.Frequency)
		set(&r.Expertise, p.Expertise)
		set(&r.Persona, p.Persona)
		set(&r.Source, p.Source)
		if err := checkSource(r.Source); err != nil {
			return err
		}
		m.Roles[i] = r
		out = &m.Roles[i]
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s, out, nil
}

func validateActivity(s *Session, a Activity) error {
	if err := checkNonEmpty("name", a.Name); err != nil {
		return err
	}
	if err := checkSource(a.Source); err != nil {
		return err
	}
	return s.checkRefs(KindRole, a.RoleID)
}

// AddActivity adds an activity to the backbone, before beforeID or at the end.
func AddActivity(sessionID string, a Activity, beforeID string) (*Session, *Activity, error) {
	var out *Activity
	s, err := mutateMap(sessionID, func(s *Session, m *StoryMap) error {
		if err := validateActivity(s, a); err != nil {
			return err
		}
		a.ID = newID("actv")
		a.CreatedAt = time.Now()
		list, err := insertBefore(m.Activities, a, beforeID, func(x Activity) string { return x.ID }, nil)
		if err != nil {
			return err
		}
		m.Activities = list
		out = &m.Activities[slices.IndexFunc(m.Activities, func(x Activity) bool { return x.ID == a.ID })]
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s, out, nil
}

type ActivityPatch struct {
	Name, RoleID, Notes *string
	Source              *Source
}

func UpdateActivity(sessionID, activityID string, p ActivityPatch) (*Session, *Activity, error) {
	var out *Activity
	s, err := mutateMap(sessionID, func(s *Session, m *StoryMap) error {
		i := slices.IndexFunc(m.Activities, func(a Activity) bool { return a.ID == activityID })
		if i < 0 {
			return fmt.Errorf("activity %s not found", activityID)
		}
		a := m.Activities[i]
		set(&a.Name, p.Name)
		set(&a.RoleID, p.RoleID)
		set(&a.Notes, p.Notes)
		set(&a.Source, p.Source)
		if err := validateActivity(s, a); err != nil {
			return err
		}
		m.Activities[i] = a
		out = &m.Activities[i]
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s, out, nil
}

func validateStep(s *Session, m *StoryMap, st Step) error {
	if err := checkNonEmpty("name", st.Name); err != nil {
		return err
	}
	if err := checkSource(st.Source); err != nil {
		return err
	}
	if st.ActivityID == "" || st.RoleID == "" {
		return fmt.Errorf("a step needs an activity_id and a role_id")
	}
	if err := s.checkRefs(KindActivity, st.ActivityID, KindRole, st.RoleID, KindScreen, st.ScreenID); err != nil {
		return err
	}
	for _, id := range st.CommandIDs {
		if err := s.checkRefs(KindCommand, id); err != nil {
			return err
		}
	}
	for _, id := range st.ReadModelIDs {
		if err := s.checkRefs(KindReadModel, id); err != nil {
			return err
		}
	}
	if st.OffScreen != "" && st.ScreenID != "" {
		return fmt.Errorf("a step is either on a screen or off_screen, not both")
	}
	if st.AfterStepID != "" {
		i := slices.IndexFunc(m.Steps, func(x Step) bool { return x.ID == st.AfterStepID })
		if i < 0 {
			return fmt.Errorf("after_step_id %s not found", st.AfterStepID)
		}
		if st.AfterStepID == st.ID || m.Steps[i].RoleID == st.RoleID {
			return fmt.Errorf("after_step_id must be another role's step: it marks a hand-off between roles")
		}
	}
	return nil
}

// AddStep adds a step under its activity, before beforeID (a step in the same
// activity) or at the end.
func AddStep(sessionID string, st Step, beforeID string) (*Session, *Step, error) {
	var out *Step
	s, err := mutateMap(sessionID, func(s *Session, m *StoryMap) error {
		if err := validateStep(s, m, st); err != nil {
			return err
		}
		st.ID = newID("step")
		st.CreatedAt = time.Now()
		list, err := insertBefore(m.Steps, st, beforeID, func(x Step) string { return x.ID },
			func(x Step) bool { return x.ActivityID == st.ActivityID })
		if err != nil {
			return err
		}
		m.Steps = list
		out = &m.Steps[slices.IndexFunc(m.Steps, func(x Step) bool { return x.ID == st.ID })]
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s, out, nil
}

type StepPatch struct {
	Name, ActivityID, RoleID, ScreenID, AfterStepID, Notes *string
	JourneyWhen, JourneyThen, OffScreen                    *string
	CommandIDs, ReadModelIDs                               *[]string
	Source                                                 *Source
}

func UpdateStep(sessionID, stepID string, p StepPatch) (*Session, *Step, error) {
	var out *Step
	s, err := mutateMap(sessionID, func(s *Session, m *StoryMap) error {
		i := slices.IndexFunc(m.Steps, func(st Step) bool { return st.ID == stepID })
		if i < 0 {
			return fmt.Errorf("step %s not found", stepID)
		}
		st := m.Steps[i]
		set(&st.Name, p.Name)
		set(&st.ActivityID, p.ActivityID)
		set(&st.RoleID, p.RoleID)
		set(&st.ScreenID, p.ScreenID)
		set(&st.AfterStepID, p.AfterStepID)
		set(&st.JourneyWhen, p.JourneyWhen)
		set(&st.JourneyThen, p.JourneyThen)
		set(&st.OffScreen, p.OffScreen)
		set(&st.Notes, p.Notes)
		set(&st.CommandIDs, p.CommandIDs)
		set(&st.ReadModelIDs, p.ReadModelIDs)
		set(&st.Source, p.Source)
		if err := validateStep(s, m, st); err != nil {
			return err
		}
		m.Steps[i] = st
		out = &m.Steps[i]
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s, out, nil
}

func validateStory(s *Session, st Story) error {
	if err := checkNonEmpty("title", st.Title); err != nil {
		return err
	}
	if err := checkSource(st.Source); err != nil {
		return err
	}
	switch st.Kind {
	case StoryDetail, StoryAlternative, StoryFailure, StoryMisuse, StorySpike:
	default:
		return fmt.Errorf("kind must be one of: detail, alternative, failure, misuse, spike; got %q", st.Kind)
	}
	if st.StepID == "" {
		return fmt.Errorf("a story needs a step_id")
	}
	for i, c := range st.Criteria {
		if err := validateCriterion(c); err != nil {
			return fmt.Errorf("criterion %d: %w", i+1, err)
		}
	}
	return s.checkRefs(KindStep, st.StepID, KindSlice, st.SliceID, KindEvent, st.EventID)
}

func validateCriterion(c Criterion) error {
	if err := checkNonEmpty("text", c.Text); err != nil {
		return err
	}
	if c.Examples != nil {
		if len(c.Examples.Columns) == 0 || len(c.Examples.Rows) == 0 {
			return fmt.Errorf("examples need columns and at least one row")
		}
		for i, row := range c.Examples.Rows {
			if len(row) != len(c.Examples.Columns) {
				return fmt.Errorf("example row %d has %d values for %d columns", i+1, len(row), len(c.Examples.Columns))
			}
		}
	}
	return nil
}

// AddCriterion appends one acceptance criterion to a story.
func AddCriterion(sessionID, storyID string, c Criterion) (*Session, *Story, error) {
	var out *Story
	s, err := mutateMap(sessionID, func(_ *Session, m *StoryMap) error {
		i := slices.IndexFunc(m.Stories, func(st Story) bool { return st.ID == storyID })
		if i < 0 {
			return fmt.Errorf("story %s not found", storyID)
		}
		if err := validateCriterion(c); err != nil {
			return err
		}
		m.Stories[i].Criteria = append(m.Stories[i].Criteria, c)
		out = &m.Stories[i]
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s, out, nil
}

// AddStory adds a story under its step, before beforeID (a story under the
// same step, so ranked above it) or at the bottom. Kind defaults to detail.
func AddStory(sessionID string, st Story, beforeID string) (*Session, *Story, error) {
	var out *Story
	s, err := mutateMap(sessionID, func(s *Session, m *StoryMap) error {
		if st.Kind == "" {
			st.Kind = StoryDetail
		}
		if err := validateStory(s, st); err != nil {
			return err
		}
		st.ID = newID("sty")
		st.CreatedAt = time.Now()
		list, err := insertBefore(m.Stories, st, beforeID, func(x Story) string { return x.ID },
			func(x Story) bool { return x.StepID == st.StepID })
		if err != nil {
			return err
		}
		m.Stories = list
		out = &m.Stories[slices.IndexFunc(m.Stories, func(x Story) bool { return x.ID == st.ID })]
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s, out, nil
}

type StoryPatch struct {
	Title, Why, StepID, SliceID, EventID, Notes *string
	Kind                                        *StoryKind
	Criteria                                    *[]Criterion
	Source                                      *Source
}

func UpdateStory(sessionID, storyID string, p StoryPatch) (*Session, *Story, error) {
	var out *Story
	s, err := mutateMap(sessionID, func(s *Session, m *StoryMap) error {
		i := slices.IndexFunc(m.Stories, func(st Story) bool { return st.ID == storyID })
		if i < 0 {
			return fmt.Errorf("story %s not found", storyID)
		}
		st := m.Stories[i]
		set(&st.Title, p.Title)
		set(&st.Why, p.Why)
		set(&st.StepID, p.StepID)
		set(&st.SliceID, p.SliceID)
		set(&st.EventID, p.EventID)
		set(&st.Notes, p.Notes)
		set(&st.Kind, p.Kind)
		set(&st.Criteria, p.Criteria)
		set(&st.Source, p.Source)
		if err := validateStory(s, st); err != nil {
			return err
		}
		m.Stories[i] = st
		out = &m.Stories[i]
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s, out, nil
}

func validateScreen(s *Session, sc Screen) error {
	if err := checkNonEmpty("name", sc.Name); err != nil {
		return err
	}
	if err := checkSource(sc.Source); err != nil {
		return err
	}
	if err := s.checkRefs(KindContext, sc.ContextID); err != nil {
		return err
	}
	for _, id := range sc.Shows {
		if err := s.checkRefs(KindReadModel, id); err != nil {
			return err
		}
	}
	seen := make(map[string]bool)
	for _, o := range sc.Offers {
		if err := s.checkRefs(KindCommand, o.CommandID); err != nil {
			return err
		}
		if o.CommandID == "" {
			return fmt.Errorf("an offer needs a command_id")
		}
		if seen[o.CommandID] {
			return fmt.Errorf("command %s is offered twice", o.CommandID)
		}
		seen[o.CommandID] = true
	}
	return nil
}

func AddScreen(sessionID string, sc Screen) (*Session, *Screen, error) {
	var out *Screen
	s, err := mutateMap(sessionID, func(s *Session, m *StoryMap) error {
		if err := validateScreen(s, sc); err != nil {
			return err
		}
		if i := slices.IndexFunc(m.Screens, func(x Screen) bool { return x.Name == sc.Name }); i >= 0 {
			return fmt.Errorf("screen %q already exists (%s); update_screen instead", sc.Name, m.Screens[i].ID)
		}
		sc.ID = newID("scr")
		sc.CreatedAt = time.Now()
		m.Screens = append(m.Screens, sc)
		out = &m.Screens[len(m.Screens)-1]
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s, out, nil
}

type ScreenPatch struct {
	Name, Notes, ContextID *string
	Shows                  *[]string
	Offers                 *[]Offer
	Source                 *Source
}

func UpdateScreen(sessionID, screenID string, p ScreenPatch) (*Session, *Screen, error) {
	var out *Screen
	s, err := mutateMap(sessionID, func(s *Session, m *StoryMap) error {
		i := slices.IndexFunc(m.Screens, func(sc Screen) bool { return sc.ID == screenID })
		if i < 0 {
			return fmt.Errorf("screen %s not found", screenID)
		}
		sc := m.Screens[i]
		set(&sc.Name, p.Name)
		set(&sc.Notes, p.Notes)
		set(&sc.ContextID, p.ContextID)
		set(&sc.Shows, p.Shows)
		set(&sc.Offers, p.Offers)
		set(&sc.Source, p.Source)
		if err := validateScreen(s, sc); err != nil {
			return err
		}
		m.Screens[i] = sc
		out = &m.Screens[i]
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s, out, nil
}

func validateSlice(m *StoryMap, sl Slice) error {
	if err := checkNonEmpty("name", sl.Name); err != nil {
		return err
	}
	switch sl.Kind {
	case SliceRelease, SliceLearning, SliceDevelopment:
	default:
		return fmt.Errorf("kind must be one of: release, learning, development; got %q", sl.Kind)
	}
	if sl.WalkingSkeleton {
		if sl.Kind != SliceDevelopment {
			return fmt.Errorf("only a development slice can be the walking skeleton")
		}
		if i := slices.IndexFunc(m.Slices, func(x Slice) bool { return x.WalkingSkeleton && x.ID != sl.ID }); i >= 0 {
			return fmt.Errorf("slice %q (%s) is already the walking skeleton", m.Slices[i].Name, m.Slices[i].ID)
		}
	}
	return nil
}

// AddSlice adds a slice, before beforeID (so above it) or at the bottom.
func AddSlice(sessionID string, sl Slice, beforeID string) (*Session, *Slice, error) {
	var out *Slice
	s, err := mutateMap(sessionID, func(_ *Session, m *StoryMap) error {
		if err := validateSlice(m, sl); err != nil {
			return err
		}
		sl.ID = newID("slc")
		sl.CreatedAt = time.Now()
		list, err := insertBefore(m.Slices, sl, beforeID, func(x Slice) string { return x.ID }, nil)
		if err != nil {
			return err
		}
		m.Slices = list
		out = &m.Slices[slices.IndexFunc(m.Slices, func(x Slice) bool { return x.ID == sl.ID })]
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s, out, nil
}

type SlicePatch struct {
	Name, Outcome, Notes *string
	Kind                 *SliceKind
	WalkingSkeleton      *bool
	Preconditions        *[]string
}

func UpdateSlice(sessionID, sliceID string, p SlicePatch) (*Session, *Slice, error) {
	var out *Slice
	s, err := mutateMap(sessionID, func(_ *Session, m *StoryMap) error {
		i := slices.IndexFunc(m.Slices, func(sl Slice) bool { return sl.ID == sliceID })
		if i < 0 {
			return fmt.Errorf("slice %s not found", sliceID)
		}
		sl := m.Slices[i]
		set(&sl.Name, p.Name)
		set(&sl.Outcome, p.Outcome)
		set(&sl.Notes, p.Notes)
		set(&sl.Kind, p.Kind)
		set(&sl.WalkingSkeleton, p.WalkingSkeleton)
		set(&sl.Preconditions, p.Preconditions)
		if err := validateSlice(m, sl); err != nil {
			return err
		}
		m.Slices[i] = sl
		out = &m.Slices[i]
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s, out, nil
}

// AssignToSlice puts stories in a slice. A story is in at most one slice, so
// this moves any that were in another.
func AssignToSlice(sessionID, sliceID string, storyIDs []string) (*Session, *Slice, error) {
	var out *Slice
	s, err := mutateMap(sessionID, func(s *Session, m *StoryMap) error {
		si := slices.IndexFunc(m.Slices, func(sl Slice) bool { return sl.ID == sliceID })
		if si < 0 {
			return fmt.Errorf("slice %s not found", sliceID)
		}
		var idx []int
		for _, id := range storyIDs {
			i := slices.IndexFunc(m.Stories, func(st Story) bool { return st.ID == id })
			if i < 0 {
				return fmt.Errorf("story %s not found", id)
			}
			idx = append(idx, i)
		}
		for _, i := range idx {
			m.Stories[i].SliceID = sliceID
		}
		out = &m.Slices[si]
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s, out, nil
}

// moveIn moves the element with id before beforeID (or to the end) within list.
func moveIn[T any](list []T, id, beforeID string, getID func(T) string, group func(T) string) ([]T, error) {
	i := slices.IndexFunc(list, func(x T) bool { return getID(x) == id })
	if i < 0 {
		return nil, fmt.Errorf("%s not found", id)
	}
	if id == beforeID {
		return nil, fmt.Errorf("an item can't be moved before itself")
	}
	item := list[i]
	rest := slices.Delete(slices.Clone(list), i, i+1)
	var sameGroup func(T) bool
	if group != nil {
		sameGroup = func(x T) bool { return group(x) == group(item) }
	}
	return insertBefore(rest, item, beforeID, getID, sameGroup)
}

// MoveItem reorders an activity, step, story or slice: before beforeID, or to
// the end of its group when beforeID is empty. Steps move within their
// activity and stories within their step; to move one elsewhere, change its
// activity_id or step_id.
func MoveItem(sessionID, kind, id, beforeID string) (*Session, error) {
	return mutateMap(sessionID, func(_ *Session, m *StoryMap) error {
		var err error
		switch kind {
		case KindActivity:
			m.Activities, err = moveIn(m.Activities, id, beforeID, func(a Activity) string { return a.ID }, nil)
		case KindStep:
			m.Steps, err = moveIn(m.Steps, id, beforeID, func(st Step) string { return st.ID }, func(st Step) string { return st.ActivityID })
		case KindStory:
			m.Stories, err = moveIn(m.Stories, id, beforeID, func(st Story) string { return st.ID }, func(st Story) string { return st.StepID })
		case KindSlice:
			m.Slices, err = moveIn(m.Slices, id, beforeID, func(sl Slice) string { return sl.ID }, nil)
		default:
			return fmt.Errorf("kind must be one of: activity, step, story, slice; got %q", kind)
		}
		return err
	})
}

// removeStoryMapItem deletes a story-map record. Containers must be emptied
// first: an activity with steps, or a step with stories, is refused rather
// than deleting what's under it. Other references are cleared and reported.
func (s *Session) removeStoryMapItem(kind, id string, detached *[]string) error {
	m := s.storyMap()
	note := func(what, name, recID, field string) {
		*detached = append(*detached, fmt.Sprintf("%s %q (%s): %s cleared", what, name, recID, field))
	}
	switch kind {
	case KindRole:
		m.Roles = slices.DeleteFunc(m.Roles, func(r Role) bool { return r.ID == id })
		s.clearRole(id, note)
	case KindActivity:
		if n := countFunc(m.Steps, func(st Step) bool { return st.ActivityID == id }); n > 0 {
			return fmt.Errorf("activity %s still has %d step(s); move or remove them first", id, n)
		}
		m.Activities = slices.DeleteFunc(m.Activities, func(a Activity) bool { return a.ID == id })
	case KindStep:
		if n := countFunc(m.Stories, func(st Story) bool { return st.StepID == id }); n > 0 {
			return fmt.Errorf("step %s still has %d story(ies); move or remove them first", id, n)
		}
		m.Steps = slices.DeleteFunc(m.Steps, func(st Step) bool { return st.ID == id })
		for i := range m.Steps {
			if m.Steps[i].AfterStepID == id {
				m.Steps[i].AfterStepID = ""
				note(KindStep, m.Steps[i].Name, m.Steps[i].ID, "after_step_id")
			}
		}
	case KindStory:
		m.Stories = slices.DeleteFunc(m.Stories, func(st Story) bool { return st.ID == id })
	case KindConstraint:
		m.Constraints = slices.DeleteFunc(m.Constraints, func(c Constraint) bool { return c.ID == id })
	case KindScreen:
		m.Screens = slices.DeleteFunc(m.Screens, func(sc Screen) bool { return sc.ID == id })
		for i := range m.Steps {
			if m.Steps[i].ScreenID == id {
				m.Steps[i].ScreenID = ""
				note(KindStep, m.Steps[i].Name, m.Steps[i].ID, "screen_id")
			}
		}
	case KindSlice:
		m.Slices = slices.DeleteFunc(m.Slices, func(sl Slice) bool { return sl.ID == id })
		for i := range m.Stories {
			if m.Stories[i].SliceID == id {
				m.Stories[i].SliceID = ""
				note(KindStory, m.Stories[i].Title, m.Stories[i].ID, "slice_id")
			}
		}
	}
	switch kind {
	case KindStep, KindScreen, KindSlice:
		s.rescopeConstraints(id, detached)
	}
	return nil
}

// rescopeConstraints makes constraints scoped to a removed record map-wide,
// rather than silently dropping them, and reports each.
func (s *Session) rescopeConstraints(id string, detached *[]string) {
	m := s.storyMap()
	for i := range m.Constraints {
		if m.Constraints[i].ScopeID == id {
			m.Constraints[i].ScopeKind = "map"
			m.Constraints[i].ScopeID = ""
			*detached = append(*detached, fmt.Sprintf("%s %q (%s): now applies to the whole map", KindConstraint, m.Constraints[i].Text, m.Constraints[i].ID))
		}
	}
}

func (s *Session) clearRole(roleID string, note func(what, name, recID, field string)) {
	m := s.storyMap()
	for i := range m.Steps {
		if m.Steps[i].RoleID == roleID {
			m.Steps[i].RoleID = ""
			note(KindStep, m.Steps[i].Name, m.Steps[i].ID, "role_id")
		}
	}
	for i := range m.Activities {
		if m.Activities[i].RoleID == roleID {
			m.Activities[i].RoleID = ""
			note(KindActivity, m.Activities[i].Name, m.Activities[i].ID, "role_id")
		}
	}
}

// cascadeIntoStoryMap detaches a removed storm record from the story map.
func (s *Session) cascadeIntoStoryMap(kind, id string, detached *[]string) {
	m := s.StoryMap
	if m == nil {
		return
	}
	note := func(what, name, recID, field string) {
		*detached = append(*detached, fmt.Sprintf("%s %q (%s): %s", what, name, recID, field))
	}
	without := func(ids []string) ([]string, bool) {
		if !slices.Contains(ids, id) {
			return ids, false
		}
		return slices.DeleteFunc(slices.Clone(ids), func(x string) bool { return x == id }), true
	}
	switch kind {
	case KindActor:
		for _, r := range m.Roles {
			if r.ActorID == id {
				note(KindRole, r.ID, r.ID, "removed with its actor")
				s.clearRole(r.ID, func(what, name, recID, field string) { note(what, name, recID, field+" cleared") })
			}
		}
		m.Roles = slices.DeleteFunc(m.Roles, func(r Role) bool { return r.ActorID == id })
	case KindCommand:
		for i := range m.Steps {
			var changed bool
			if m.Steps[i].CommandIDs, changed = without(m.Steps[i].CommandIDs); changed {
				note(KindStep, m.Steps[i].Name, m.Steps[i].ID, "removed from command_ids")
			}
		}
		for i := range m.Screens {
			n := len(m.Screens[i].Offers)
			m.Screens[i].Offers = slices.DeleteFunc(m.Screens[i].Offers, func(o Offer) bool { return o.CommandID == id })
			if len(m.Screens[i].Offers) != n {
				note(KindScreen, m.Screens[i].Name, m.Screens[i].ID, "no longer offers it")
			}
		}
	case KindReadModel:
		for i := range m.Steps {
			var changed bool
			if m.Steps[i].ReadModelIDs, changed = without(m.Steps[i].ReadModelIDs); changed {
				note(KindStep, m.Steps[i].Name, m.Steps[i].ID, "removed from read_model_ids")
			}
		}
		for i := range m.Screens {
			var changed bool
			if m.Screens[i].Shows, changed = without(m.Screens[i].Shows); changed {
				note(KindScreen, m.Screens[i].Name, m.Screens[i].ID, "no longer shows it")
			}
		}
	case KindContext:
		for i := range m.Screens {
			if m.Screens[i].ContextID == id {
				m.Screens[i].ContextID = ""
				note(KindScreen, m.Screens[i].Name, m.Screens[i].ID, "context_id cleared")
			}
		}
	case KindEvent:
		for i := range m.Stories {
			if m.Stories[i].EventID == id {
				m.Stories[i].EventID = ""
				note(KindStory, m.Stories[i].Title, m.Stories[i].ID, "event_id cleared")
			}
		}
	}
}

func countFunc[T any](list []T, f func(T) bool) int {
	n := 0
	for _, x := range list {
		if f(x) {
			n++
		}
	}
	return n
}

func validateConstraint(s *Session, c Constraint) error {
	if err := checkNonEmpty("text", c.Text); err != nil {
		return err
	}
	if err := checkSource(c.Source); err != nil {
		return err
	}
	if !slices.Contains(ConstraintKinds, c.Kind) {
		return fmt.Errorf("kind must be one of %v; got %q", ConstraintKinds, c.Kind)
	}
	switch c.ScopeKind {
	case "map":
		if c.ScopeID != "" {
			return fmt.Errorf("a map-wide constraint has no scope_id")
		}
		return nil
	case KindSlice, KindStep, KindScreen:
		if c.ScopeID == "" {
			return fmt.Errorf("a %s constraint needs a scope_id", c.ScopeKind)
		}
		return s.checkRefs(c.ScopeKind, c.ScopeID)
	}
	return fmt.Errorf("scope_kind must be one of: map, slice, step, screen; got %q", c.ScopeKind)
}

// AddConstraint records a cross-functional requirement. ScopeKind defaults to map.
func AddConstraint(sessionID string, c Constraint) (*Session, *Constraint, error) {
	var out *Constraint
	s, err := mutateMap(sessionID, func(s *Session, m *StoryMap) error {
		if c.ScopeKind == "" {
			c.ScopeKind = "map"
		}
		if err := validateConstraint(s, c); err != nil {
			return err
		}
		c.ID = newID("cons")
		c.CreatedAt = time.Now()
		m.Constraints = append(m.Constraints, c)
		out = &m.Constraints[len(m.Constraints)-1]
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s, out, nil
}

type ConstraintPatch struct {
	Text, ScopeKind, ScopeID, Measure, Target, Notes *string
	Kind                                             *ConstraintKind
	Source                                           *Source
}

func UpdateConstraint(sessionID, constraintID string, p ConstraintPatch) (*Session, *Constraint, error) {
	var out *Constraint
	s, err := mutateMap(sessionID, func(s *Session, m *StoryMap) error {
		i := slices.IndexFunc(m.Constraints, func(c Constraint) bool { return c.ID == constraintID })
		if i < 0 {
			return fmt.Errorf("constraint %s not found", constraintID)
		}
		c := m.Constraints[i]
		set(&c.Text, p.Text)
		set(&c.ScopeKind, p.ScopeKind)
		set(&c.ScopeID, p.ScopeID)
		set(&c.Measure, p.Measure)
		set(&c.Target, p.Target)
		set(&c.Notes, p.Notes)
		set(&c.Kind, p.Kind)
		set(&c.Source, p.Source)
		if err := validateConstraint(s, c); err != nil {
			return err
		}
		m.Constraints[i] = c
		out = &m.Constraints[i]
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s, out, nil
}
