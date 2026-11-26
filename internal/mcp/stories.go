package mcp

import (
	"context"
	"fmt"
	"slices"

	libmcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/forgant-foundry/forgant-discovery-mcp/internal/gherkin"
	"github.com/forgant-foundry/forgant-discovery-mcp/internal/playbooks"
	"github.com/forgant-foundry/forgant-discovery-mcp/internal/session"
)

// NewStories builds the story mapping server ("serve stories"). It shares
// the session, glossary and hot spots with the events server, and links its
// steps and screens to the storm's commands and read models by ID.
func NewStories() *server.MCPServer {
	s := server.NewMCPServer("forgant-discovery-stories", version())
	addSharedTools(s, playbooks.ServerStories)

	s.AddTool(frameStoryMapTool(), handleFrameStoryMap)
	s.AddTool(addRoleTool(), handleAddRole)
	s.AddTool(updateRoleTool(), handleUpdateRole)
	s.AddTool(addActivityTool(), handleAddActivity)
	s.AddTool(updateActivityTool(), handleUpdateActivity)
	s.AddTool(addStepTool(), handleAddStep)
	s.AddTool(updateStepTool(), handleUpdateStep)
	s.AddTool(addStoryTool(), handleAddStory)
	s.AddTool(updateStoryTool(), handleUpdateStory)
	s.AddTool(addScreenTool(), handleAddScreen)
	s.AddTool(updateScreenTool(), handleUpdateScreen)
	s.AddTool(addSliceTool(), handleAddSlice)
	s.AddTool(updateSliceTool(), handleUpdateSlice)
	s.AddTool(assignToSliceTool(), handleAssignToSlice)
	s.AddTool(moveItemTool(), handleMoveItem)
	s.AddTool(checkLinksTool(), handleCheckLinks)
	s.AddTool(addCriterionTool(), handleAddCriterion)
	s.AddTool(addConstraintTool(), handleAddConstraint)
	s.AddTool(updateConstraintTool(), handleUpdateConstraint)
	s.AddTool(checkReadinessTool(), handleCheckReadiness)
	s.AddTool(emitStoryMapTool(), handleEmitStoryMap)
	return s
}

func toolError(err error) (*libmcp.CallToolResult, error) {
	return libmcp.NewToolResultError(err.Error()), nil
}

func optBool(req libmcp.CallToolRequest, key string) (*bool, error) {
	v, ok := req.GetArguments()[key]
	if !ok || v == nil {
		return nil, nil
	}
	b, ok := v.(bool)
	if !ok {
		return nil, fmt.Errorf("%s must be a boolean", key)
	}
	return &b, nil
}

// optOffers reads an array of {command_id, shows_after} objects.
func optOffers(req libmcp.CallToolRequest) (*[]session.Offer, error) {
	v, ok := req.GetArguments()["offers"]
	if !ok || v == nil {
		return nil, nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("offers must be an array of {command_id, shows_after} objects")
	}
	out := make([]session.Offer, 0, len(items))
	for _, item := range items {
		o, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("offers must be an array of {command_id, shows_after} objects")
		}
		cmd, _ := o["command_id"].(string)
		after, _ := o["shows_after"].(string)
		out = append(out, session.Offer{CommandID: cmd, ShowsAfter: after})
	}
	return &out, nil
}

func withOffers() libmcp.ToolOption {
	return libmcp.WithArray("offers",
		libmcp.Items(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command_id":  map[string]any{"type": "string", "description": "ID of a storm command the screen offers"},
				"shows_after": map[string]any{"type": "string", "description": "What the screen shows once it's done, e.g. 'the new balance'"},
			},
			"required": []string{"command_id"},
		}),
		libmcp.Description("Commands the screen offers. Each needs domain behaviour in the storm (linked events), or check_links reports a control that would do nothing."),
	)
}

// --- frame ---

func frameStoryMapTool() libmcp.Tool {
	return libmcp.NewTool("frame_story_map",
		libmcp.WithDescription("Record the opportunity the story map is for (Patton's opportunity canvas): who it's for, their problems, how they cope today, solution ideas, why it matters to the business, and how success is measured. Topics to discuss, not a form: set what you know, in any order. Omitted fields are unchanged; arrays replace."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("for_whom", libmcp.Description("Who the product or feature is for")),
		libmcp.WithArray("problems", libmcp.WithStringItems(), libmcp.Description("Problems it solves for them")),
		libmcp.WithString("today", libmcp.Description("How they cope today, and what hurts")),
		libmcp.WithArray("solution_ideas", libmcp.WithStringItems(), libmcp.Description("Ideas for what to build")),
		libmcp.WithString("why", libmcp.Description("Why it matters to the business")),
		libmcp.WithArray("success_measures", libmcp.WithStringItems(), libmcp.Description("How we'll know it worked, in terms the product can emit")),
	)
}

func handleFrameStoryMap(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return toolError(err)
	}
	var p session.FramePatch
	if err := optStringFields(req, map[string]**string{"for_whom": &p.ForWhom, "today": &p.Today, "why": &p.Why}); err != nil {
		return toolError(err)
	}
	for key, dst := range map[string]**[]string{"problems": &p.Problems, "solution_ideas": &p.SolutionIdeas, "success_measures": &p.SuccessMeasures} {
		if *dst, err = optStrings(req, key); err != nil {
			return toolError(err)
		}
	}
	_, f, err := session.SetFrame(sessionID, p)
	if err != nil {
		return toolError(err)
	}
	return resultJSON(f)
}

// --- roles ---

func addRoleTool() libmcp.Tool {
	return libmcp.NewTool("add_role",
		libmcp.WithDescription("Record a role that uses the product (Cohn's user role modelling). A role is a storm actor: pass actor_id to use one the storm already has, or actor_name to create it. An actor has at most one role."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("actor_id", libmcp.Description("ID of an existing storm actor")),
		libmcp.WithString("actor_name", libmcp.Description("Name of a new actor to create for this role (e.g. 'Accountant')")),
		libmcp.WithString("actor_type", libmcp.Enum("person", "role", "external_system"), libmcp.Description("Type for a new actor; default role")),
		libmcp.WithString("goal", libmcp.Description("What this role is trying to achieve")),
		libmcp.WithString("frequency", libmcp.Description("How often they use the product")),
		libmcp.WithString("expertise", libmcp.Description("Their expertise in the domain and with software")),
		libmcp.WithString("persona", libmcp.Description("A persona, only where it helps the team picture the role")),
		withSource(),
	)
}

func handleAddRole(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return toolError(err)
	}
	_, r, err := session.AddRole(sessionID, session.Role{
		ActorID:   req.GetString("actor_id", ""),
		Goal:      req.GetString("goal", ""),
		Frequency: req.GetString("frequency", ""),
		Expertise: req.GetString("expertise", ""),
		Persona:   req.GetString("persona", ""),
		Source:    session.Source(req.GetString("source", "")),
	}, req.GetString("actor_name", ""), session.ActorType(req.GetString("actor_type", "")))
	if err != nil {
		return toolError(err)
	}
	return resultJSON(r)
}

func updateRoleTool() libmcp.Tool {
	return libmcp.NewTool("update_role",
		libmcp.WithDescription("Edit a role's goal, frequency, expertise, persona or source. Omitted fields are unchanged. Rename the role with update_actor on its actor."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("role_id", libmcp.Required(), libmcp.Description("ID of the role")),
		libmcp.WithString("goal", libmcp.Description("What this role is trying to achieve")),
		libmcp.WithString("frequency", libmcp.Description("How often they use the product")),
		libmcp.WithString("expertise", libmcp.Description("Their expertise")),
		libmcp.WithString("persona", libmcp.Description("A persona; empty string clears it")),
		withSource(),
	)
}

func handleUpdateRole(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "role_id")
	if err != nil {
		return toolError(err)
	}
	var p session.RolePatch
	if err := optStringFields(req, map[string]**string{"goal": &p.Goal, "frequency": &p.Frequency, "expertise": &p.Expertise, "persona": &p.Persona}); err != nil {
		return toolError(err)
	}
	if p.Source, err = optSource(req); err != nil {
		return toolError(err)
	}
	_, r, err := session.UpdateRole(ids[0], ids[1], p)
	if err != nil {
		return toolError(err)
	}
	return resultJSON(r)
}

// --- activities ---

func addActivityTool() libmcp.Tool {
	return libmcp.NewTool("add_activity",
		libmcp.WithDescription("Add an activity to the map's backbone: a short verb phrase grouping steps, named the way the role would name it ('manage my ledgers'). Activities are in narrative order; pass before_id to place it, or it goes at the end."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("name", libmcp.Required(), libmcp.Description("Short verb phrase")),
		libmcp.WithString("role_id", libmcp.Description("The role whose activity it is, if only one")),
		libmcp.WithString("notes", libmcp.Description("Optional notes")),
		libmcp.WithString("before_id", libmcp.Description("ID of the activity to place this one before")),
		withSource(),
	)
}

func handleAddActivity(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "name")
	if err != nil {
		return toolError(err)
	}
	_, a, err := session.AddActivity(ids[0], session.Activity{
		Name:   ids[1],
		RoleID: req.GetString("role_id", ""),
		Notes:  req.GetString("notes", ""),
		Source: session.Source(req.GetString("source", "")),
	}, req.GetString("before_id", ""))
	if err != nil {
		return toolError(err)
	}
	return resultJSON(a)
}

func updateActivityTool() libmcp.Tool {
	return libmcp.NewTool("update_activity",
		libmcp.WithDescription("Edit an activity. Omitted fields are unchanged. Reorder with move_item."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("activity_id", libmcp.Required(), libmcp.Description("ID of the activity")),
		libmcp.WithString("name", libmcp.Description("New name")),
		libmcp.WithString("role_id", libmcp.Description("The role whose activity it is; empty string clears it")),
		libmcp.WithString("notes", libmcp.Description("New notes")),
		withSource(),
	)
}

func handleUpdateActivity(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "activity_id")
	if err != nil {
		return toolError(err)
	}
	var p session.ActivityPatch
	if err := optStringFields(req, map[string]**string{"name": &p.Name, "role_id": &p.RoleID, "notes": &p.Notes}); err != nil {
		return toolError(err)
	}
	if p.Source, err = optSource(req); err != nil {
		return toolError(err)
	}
	_, a, err := session.UpdateActivity(ids[0], ids[1], p)
	if err != nil {
		return toolError(err)
	}
	return resultJSON(a)
}

// --- steps ---

func stepLinkOptions() []libmcp.ToolOption {
	return []libmcp.ToolOption{
		libmcp.WithArray("command_ids", libmcp.WithStringItems(), libmcp.Description("Storm commands the step issues")),
		libmcp.WithArray("read_model_ids", libmcp.WithStringItems(), libmcp.Description("Storm read models the role looks at in this step")),
		libmcp.WithString("screen_id", libmcp.Description("The screen the step happens on")),
		libmcp.WithString("after_step_id", libmcp.Description("A hand-off: another role's step this one waits for. The later role's journey replays the earlier role's journey up to that step as Givens.")),
		libmcp.WithString("journey_when", libmcp.Description("The step's When in a journey, with concrete values and naming its screen, e.g. 'the accountant records a transaction of -42.50 on the account screen'. Without it, the text is derived from the command.")),
		libmcp.WithString("journey_then", libmcp.Description("The step's Then in a journey, read from the screen, e.g. 'the account screen shows a balance of 957.50'. Without it, the offer's shows_after is used.")),
		libmcp.WithString("off_screen", libmcp.Description("Why the step is taken off any screen (e.g. 'the operator's docker compose up'). Without it, a step in a slice needs a screen.")),
		libmcp.WithString("notes", libmcp.Description("Optional notes")),
	}
}

func addStepTool() libmcp.Tool {
	opts := []libmcp.ToolOption{
		libmcp.WithDescription("Add a step: one thing a role does, a short verb phrase in narrative flow ('…and then I record a transaction'). Link it to the storm with command_ids and read_model_ids; a step with neither is a gap check_links reports. Steps are in narrative order within their activity; pass before_id to place it."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("activity_id", libmcp.Required(), libmcp.Description("The activity the step belongs to")),
		libmcp.WithString("role_id", libmcp.Required(), libmcp.Description("The role doing the step")),
		libmcp.WithString("name", libmcp.Required(), libmcp.Description("Short verb phrase, in the role's words")),
		libmcp.WithString("before_id", libmcp.Description("ID of a step in the same activity to place this one before")),
		withSource(),
	}
	return libmcp.NewTool("add_step", append(opts, stepLinkOptions()...)...)
}

func handleAddStep(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "activity_id", "role_id", "name")
	if err != nil {
		return toolError(err)
	}
	_, st, err := session.AddStep(ids[0], session.Step{
		ActivityID:   ids[1],
		RoleID:       ids[2],
		Name:         ids[3],
		CommandIDs:   req.GetStringSlice("command_ids", nil),
		ReadModelIDs: req.GetStringSlice("read_model_ids", nil),
		ScreenID:     req.GetString("screen_id", ""),
		AfterStepID:  req.GetString("after_step_id", ""),
		JourneyWhen:  req.GetString("journey_when", ""),
		JourneyThen:  req.GetString("journey_then", ""),
		OffScreen:    req.GetString("off_screen", ""),
		Notes:        req.GetString("notes", ""),
		Source:       session.Source(req.GetString("source", "")),
	}, req.GetString("before_id", ""))
	if err != nil {
		return toolError(err)
	}
	return resultJSON(st)
}

func updateStepTool() libmcp.Tool {
	opts := []libmcp.ToolOption{
		libmcp.WithDescription("Edit a step, including its links to the storm. Omitted fields are unchanged; arrays replace; an empty string clears. Changing activity_id moves it to the end of that activity; reorder with move_item."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("step_id", libmcp.Required(), libmcp.Description("ID of the step")),
		libmcp.WithString("name", libmcp.Description("New verb phrase")),
		libmcp.WithString("activity_id", libmcp.Description("Move it under another activity")),
		libmcp.WithString("role_id", libmcp.Description("The role doing the step")),
		withSource(),
	}
	return libmcp.NewTool("update_step", append(opts, stepLinkOptions()...)...)
}

func handleUpdateStep(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "step_id")
	if err != nil {
		return toolError(err)
	}
	var p session.StepPatch
	if err := optStringFields(req, map[string]**string{
		"name": &p.Name, "activity_id": &p.ActivityID, "role_id": &p.RoleID,
		"screen_id": &p.ScreenID, "after_step_id": &p.AfterStepID, "notes": &p.Notes,
		"journey_when": &p.JourneyWhen, "journey_then": &p.JourneyThen, "off_screen": &p.OffScreen,
	}); err != nil {
		return toolError(err)
	}
	if p.CommandIDs, err = optStrings(req, "command_ids"); err != nil {
		return toolError(err)
	}
	if p.ReadModelIDs, err = optStrings(req, "read_model_ids"); err != nil {
		return toolError(err)
	}
	if p.Source, err = optSource(req); err != nil {
		return toolError(err)
	}
	_, st, err := session.UpdateStep(ids[0], ids[1], p)
	if err != nil {
		return toolError(err)
	}
	return resultJSON(st)
}

// --- stories ---

func storyKindOption() libmcp.ToolOption {
	return libmcp.WithString("kind",
		libmcp.Enum(string(session.StoryDetail), string(session.StoryAlternative), string(session.StoryFailure), string(session.StoryMisuse), string(session.StorySpike)),
		libmcp.Description("detail (default), alternative (another way to do the step), failure (something that goes wrong), misuse (someone using it against its purpose; expected to be refused), or spike (work that retires a feasibility risk)"),
	)
}

func addStoryTool() libmcp.Tool {
	return libmcp.NewTool("add_story",
		libmcp.WithDescription("Add a story under a step: a detail, an alternative, something that goes wrong, a misuse or a spike. Stories under a step are ranked, most critical first; pass before_id to rank it above another story under the same step."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("step_id", libmcp.Required(), libmcp.Description("The step it's under")),
		libmcp.WithString("title", libmcp.Required(), libmcp.Description("Short title")),
		libmcp.WithString("why", libmcp.Description("Why the role wants it: the benefit. A story the expert can't give a why for may not be worth building.")),
		storyKindOption(),
		libmcp.WithString("slice_id", libmcp.Description("The slice it belongs to")),
		libmcp.WithString("event_id", libmcp.Description("For a failure story: the storm's conditional event it describes")),
		withCriteria(),
		libmcp.WithString("notes", libmcp.Description("Optional notes")),
		libmcp.WithString("before_id", libmcp.Description("ID of a story under the same step to rank this one above")),
		withSource(),
	)
}

func handleAddStory(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "step_id", "title")
	if err != nil {
		return toolError(err)
	}
	criteria, err := optCriteria(req)
	if err != nil {
		return toolError(err)
	}
	story := session.Story{
		StepID:  ids[1],
		Title:   ids[2],
		Why:     req.GetString("why", ""),
		Kind:    session.StoryKind(req.GetString("kind", "")),
		SliceID: req.GetString("slice_id", ""),
		EventID: req.GetString("event_id", ""),
		Notes:   req.GetString("notes", ""),
		Source:  session.Source(req.GetString("source", "")),
	}
	if criteria != nil {
		story.Criteria = *criteria
	}
	_, st, err := session.AddStory(ids[0], story, req.GetString("before_id", ""))
	if err != nil {
		return toolError(err)
	}
	return resultJSON(st)
}

func updateStoryTool() libmcp.Tool {
	return libmcp.NewTool("update_story",
		libmcp.WithDescription("Edit a story. Omitted fields are unchanged; an empty string clears. Changing step_id moves it to the bottom of that step; re-rank with move_item."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("story_id", libmcp.Required(), libmcp.Description("ID of the story")),
		libmcp.WithString("title", libmcp.Description("New title")),
		libmcp.WithString("why", libmcp.Description("Why the role wants it")),
		libmcp.WithString("step_id", libmcp.Description("Move it under another step")),
		storyKindOption(),
		libmcp.WithString("slice_id", libmcp.Description("The slice it belongs to; empty string takes it out of any slice")),
		libmcp.WithString("event_id", libmcp.Description("The storm's conditional event it describes")),
		withCriteria(),
		libmcp.WithString("notes", libmcp.Description("New notes")),
		withSource(),
	)
}

func handleUpdateStory(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "story_id")
	if err != nil {
		return toolError(err)
	}
	var p session.StoryPatch
	var kind *string
	if err := optStringFields(req, map[string]**string{
		"title": &p.Title, "why": &p.Why, "step_id": &p.StepID, "slice_id": &p.SliceID,
		"event_id": &p.EventID, "notes": &p.Notes, "kind": &kind,
	}); err != nil {
		return toolError(err)
	}
	if kind != nil {
		k := session.StoryKind(*kind)
		p.Kind = &k
	}
	if p.Criteria, err = optCriteria(req); err != nil {
		return toolError(err)
	}
	if p.Source, err = optSource(req); err != nil {
		return toolError(err)
	}
	_, st, err := session.UpdateStory(ids[0], ids[1], p)
	if err != nil {
		return toolError(err)
	}
	return resultJSON(st)
}

// --- screens ---

func addScreenTool() libmcp.Tool {
	return libmcp.NewTool("add_screen",
		libmcp.WithDescription("Add a screen: where steps happen, described as the read models it shows and the commands it offers (Cohn's low-fidelity prototype). A screen isn't an aggregate. Every offered command gets its own scenario, so nothing ships as a button with no handler."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("name", libmcp.Required(), libmcp.Description("Screen name, e.g. 'Transactions'")),
		libmcp.WithArray("shows", libmcp.WithStringItems(), libmcp.Description("IDs of storm read models the screen shows")),
		withOffers(),
		withScreenContext(),
		libmcp.WithString("notes", libmcp.Description("Optional notes")),
		withSource(),
	)
}

func handleAddScreen(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "name")
	if err != nil {
		return toolError(err)
	}
	offers, err := optOffers(req)
	if err != nil {
		return toolError(err)
	}
	sc := session.Screen{
		Name:      ids[1],
		ContextID: req.GetString("context_id", ""),
		Shows:     req.GetStringSlice("shows", nil),
		Notes:     req.GetString("notes", ""),
		Source:    session.Source(req.GetString("source", "")),
	}
	if offers != nil {
		sc.Offers = *offers
	}
	_, out, err := session.AddScreen(ids[0], sc)
	if err != nil {
		return toolError(err)
	}
	return resultJSON(out)
}

func updateScreenTool() libmcp.Tool {
	return libmcp.NewTool("update_screen",
		libmcp.WithDescription("Edit a screen. Omitted fields are unchanged; shows and offers, when given, replace the whole list."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("screen_id", libmcp.Required(), libmcp.Description("ID of the screen")),
		libmcp.WithString("name", libmcp.Description("New name")),
		libmcp.WithArray("shows", libmcp.WithStringItems(), libmcp.Description("Replacement list of read model IDs")),
		withOffers(),
		withScreenContext(),
		libmcp.WithString("notes", libmcp.Description("New notes")),
		withSource(),
	)
}

func handleUpdateScreen(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "screen_id")
	if err != nil {
		return toolError(err)
	}
	var p session.ScreenPatch
	if err := optStringFields(req, map[string]**string{"name": &p.Name, "notes": &p.Notes, "context_id": &p.ContextID}); err != nil {
		return toolError(err)
	}
	if p.Shows, err = optStrings(req, "shows"); err != nil {
		return toolError(err)
	}
	if p.Offers, err = optOffers(req); err != nil {
		return toolError(err)
	}
	if p.Source, err = optSource(req); err != nil {
		return toolError(err)
	}
	_, sc, err := session.UpdateScreen(ids[0], ids[1], p)
	if err != nil {
		return toolError(err)
	}
	return resultJSON(sc)
}

// --- slices ---

func sliceKindOption() libmcp.ToolOption {
	return libmcp.WithString("kind",
		libmcp.Enum(string(session.SliceRelease), string(session.SliceLearning), string(session.SliceDevelopment)),
		libmcp.Description("release (a minimum solution that reaches an outcome), learning (an experiment that retires a risk) or development (build order within a release; the first is usually the walking skeleton)"),
	)
}

func addSliceTool() libmcp.Tool {
	return libmcp.NewTool("add_slice",
		libmcp.WithDescription("Add a slice: a horizontal cut through the map. Slices are ordered top to bottom; pass before_id to place it above another. Put stories in it with assign_to_slice."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("name", libmcp.Required(), libmcp.Description("Slice name, e.g. 'walking skeleton', 'first release'")),
		sliceKindOption(),
		libmcp.WithString("outcome", libmcp.Description("What the slice achieves, or for a learning slice what we'll learn")),
		libmcp.WithBoolean("walking_skeleton", libmcp.Description("This development slice is the walking skeleton: just enough to see the product working end to end. At most one.")),
		libmcp.WithArray("preconditions", libmcp.WithStringItems(), libmcp.Description("Opening Givens of every journey in the slice, e.g. 'the developer rung is running'")),
		libmcp.WithString("notes", libmcp.Description("Optional notes")),
		libmcp.WithString("before_id", libmcp.Description("ID of the slice to place this one above")),
	)
}

func handleAddSlice(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "name", "kind")
	if err != nil {
		return toolError(err)
	}
	_, sl, err := session.AddSlice(ids[0], session.Slice{
		Name:            ids[1],
		Kind:            session.SliceKind(ids[2]),
		Outcome:         req.GetString("outcome", ""),
		WalkingSkeleton: req.GetBool("walking_skeleton", false),
		Preconditions:   req.GetStringSlice("preconditions", nil),
		Notes:           req.GetString("notes", ""),
	}, req.GetString("before_id", ""))
	if err != nil {
		return toolError(err)
	}
	return resultJSON(sl)
}

func updateSliceTool() libmcp.Tool {
	return libmcp.NewTool("update_slice",
		libmcp.WithDescription("Edit a slice. Omitted fields are unchanged. Reorder with move_item."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("slice_id", libmcp.Required(), libmcp.Description("ID of the slice")),
		libmcp.WithString("name", libmcp.Description("New name")),
		sliceKindOption(),
		libmcp.WithString("outcome", libmcp.Description("What it achieves")),
		libmcp.WithBoolean("walking_skeleton", libmcp.Description("Whether it's the walking skeleton")),
		libmcp.WithArray("preconditions", libmcp.WithStringItems(), libmcp.Description("Replacement list of the journeys' opening Givens")),
		libmcp.WithString("notes", libmcp.Description("New notes")),
	)
}

func handleUpdateSlice(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "slice_id")
	if err != nil {
		return toolError(err)
	}
	var p session.SlicePatch
	var kind *string
	if err := optStringFields(req, map[string]**string{"name": &p.Name, "outcome": &p.Outcome, "notes": &p.Notes, "kind": &kind}); err != nil {
		return toolError(err)
	}
	if kind != nil {
		k := session.SliceKind(*kind)
		p.Kind = &k
	}
	if p.WalkingSkeleton, err = optBool(req, "walking_skeleton"); err != nil {
		return toolError(err)
	}
	if p.Preconditions, err = optStrings(req, "preconditions"); err != nil {
		return toolError(err)
	}
	_, sl, err := session.UpdateSlice(ids[0], ids[1], p)
	if err != nil {
		return toolError(err)
	}
	return resultJSON(sl)
}

func assignToSliceTool() libmcp.Tool {
	return libmcp.NewTool("assign_to_slice",
		libmcp.WithDescription("Put stories in a slice. A story is in at most one slice, so this moves any that were in another. A step is in a slice when one of its stories is."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("slice_id", libmcp.Required(), libmcp.Description("ID of the slice")),
		libmcp.WithArray("story_ids", libmcp.Required(), libmcp.WithStringItems(), libmcp.Description("IDs of the stories")),
	)
}

func handleAssignToSlice(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "slice_id")
	if err != nil {
		return toolError(err)
	}
	stories, err := req.RequireStringSlice("story_ids")
	if err != nil {
		return toolError(err)
	}
	_, sl, err := session.AssignToSlice(ids[0], ids[1], stories)
	if err != nil {
		return toolError(err)
	}
	return resultJSON(map[string]any{"slice": sl, "assigned": stories})
}

// --- order and checks ---

func moveItemTool() libmcp.Tool {
	return libmcp.NewTool("move_item",
		libmcp.WithDescription("Reorder the map: move an activity, step, story or slice before another of the same kind, or to the end of its group when before_id is omitted. Steps move within their activity and stories within their step."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("kind", libmcp.Required(), libmcp.Enum(session.KindActivity, session.KindStep, session.KindStory, session.KindSlice), libmcp.Description("activity, step, story or slice")),
		libmcp.WithString("id", libmcp.Required(), libmcp.Description("ID of the item to move")),
		libmcp.WithString("before_id", libmcp.Description("ID of the item to place it before")),
	)
}

func handleMoveItem(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "kind", "id")
	if err != nil {
		return toolError(err)
	}
	if _, err := session.MoveItem(ids[0], ids[1], ids[2], req.GetString("before_id", "")); err != nil {
		return toolError(err)
	}
	return resultJSON(map[string]any{"moved": ids[2], "before": req.GetString("before_id", "")})
}

func checkLinksTool() libmcp.Tool {
	return libmcp.NewTool("check_links",
		libmcp.WithDescription("Check the story map against the event storm, both ways. Warnings: steps with no command or read model (gaps in the storm), screens offering commands with no domain behaviour, steps using a command their screen doesn't offer. Notes: commands no step uses and no policy issues, read models no screen shows, roles with no steps. Lint: glossary violations in step, activity, story and screen text. Raise a storm hot spot for each gap the expert confirms."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
	)
}

func handleCheckLinks(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return toolError(err)
	}
	s, err := session.Load(sessionID)
	if err != nil {
		return toolError(err)
	}
	findings := session.CheckLinks(s)
	counts := map[string]int{}
	for _, f := range findings {
		counts[f.Severity]++
	}
	return resultJSON(map[string]any{
		"clean":    counts[session.SeverityWarning] == 0 && counts[session.SeverityLint] == 0,
		"counts":   counts,
		"findings": findings,
	})
}

// --- phase 2: confirmation, constraints, readiness, emit ---

func withScreenContext() libmcp.ToolOption {
	return libmcp.WithString("context_id",
		libmcp.Description("The front-end bounded context (surface) the screen belongs to, e.g. Web. Its features are emitted under that context's directory."),
	)
}

var criterionSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"text":  map[string]any{"type": "string", "description": "The rule, used as the scenario title"},
		"given": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Preconditions of the example"},
		"when":  map[string]any{"type": "string", "description": "The action"},
		"then":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "What we'll check"},
		"examples": map[string]any{
			"type":        "object",
			"description": "Rows for a Scenario Outline; steps refer to columns as <column>",
			"properties": map[string]any{
				"columns": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"rows":    map[string]any{"type": "array", "items": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}},
			},
		},
	},
	"required": []string{"text"},
}

func withCriteria() libmcp.ToolOption {
	return libmcp.WithArray("criteria", libmcp.Items(criterionSchema),
		libmcp.Description("Acceptance criteria from example mapping: each rule with a concrete example as Given/When/Then, and optional example rows for a Scenario Outline. Replaces the list; add one at a time with add_criterion."),
	)
}

func toStrings(v any) []string {
	items, _ := v.([]any)
	var out []string
	for _, it := range items {
		if s, ok := it.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func parseCriterion(v any) (session.Criterion, error) {
	o, ok := v.(map[string]any)
	if !ok {
		return session.Criterion{}, fmt.Errorf("a criterion must be an object with text, given, when, then and examples")
	}
	c := session.Criterion{Given: toStrings(o["given"]), Then: toStrings(o["then"])}
	c.Text, _ = o["text"].(string)
	c.When, _ = o["when"].(string)
	if ex, ok := o["examples"].(map[string]any); ok {
		e := &session.Examples{Columns: toStrings(ex["columns"])}
		rows, _ := ex["rows"].([]any)
		for _, r := range rows {
			e.Rows = append(e.Rows, toStrings(r))
		}
		c.Examples = e
	}
	return c, nil
}

func optCriteria(req libmcp.CallToolRequest) (*[]session.Criterion, error) {
	v, ok := req.GetArguments()["criteria"]
	if !ok || v == nil {
		return nil, nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("criteria must be an array of objects")
	}
	out := make([]session.Criterion, 0, len(items))
	for _, it := range items {
		c, err := parseCriterion(it)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return &out, nil
}

func addCriterionTool() libmcp.Tool {
	return libmcp.NewTool("add_criterion",
		libmcp.WithDescription("Add one acceptance criterion to a story (example mapping): the rule as text, a concrete example as Given/When/Then, and optional example rows (a Scenario Outline, steps referring to columns as <column>). Emitted as one scenario. Write the domain behaviour, not UI mechanics: no clicks, buttons or fields."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("story_id", libmcp.Required(), libmcp.Description("ID of the story")),
		libmcp.WithString("text", libmcp.Required(), libmcp.Description("The rule, e.g. 'a split transaction's parts add up to its total'")),
		libmcp.WithArray("given", libmcp.WithStringItems(), libmcp.Description("Preconditions")),
		libmcp.WithString("when", libmcp.Description("The action")),
		libmcp.WithArray("then", libmcp.WithStringItems(), libmcp.Description("What we'll check")),
		libmcp.WithObject("examples", libmcp.Description("{columns: [...], rows: [[...], ...]} for a Scenario Outline")),
	)
}

func handleAddCriterion(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "story_id", "text")
	if err != nil {
		return toolError(err)
	}
	args := req.GetArguments()
	c, err := parseCriterion(map[string]any{
		"text": ids[2], "given": args["given"], "when": args["when"], "then": args["then"], "examples": args["examples"],
	})
	if err != nil {
		return toolError(err)
	}
	_, st, err := session.AddCriterion(ids[0], ids[1], c)
	if err != nil {
		return toolError(err)
	}
	return resultJSON(st)
}

func constraintKinds() []string {
	var out []string
	for _, k := range session.ConstraintKinds {
		out = append(out, string(k))
	}
	return out
}

func constraintOptions() []libmcp.ToolOption {
	return []libmcp.ToolOption{
		libmcp.WithString("kind", libmcp.Enum(constraintKinds()...), libmcp.Description("What kind of cross-functional requirement it is")),
		libmcp.WithString("scope_kind", libmcp.Enum("map", session.KindSlice, session.KindStep, session.KindScreen), libmcp.Description("What it applies to; default map")),
		libmcp.WithString("scope_id", libmcp.Description("ID of the slice, step or screen it applies to")),
		libmcp.WithString("measure", libmcp.Description("What's measured, in the user's terms, e.g. 'the time to show the balance'")),
		libmcp.WithString("target", libmcp.Description("The target, e.g. 'under 1 second for 95% of requests'. 'Fast' or 'easy' isn't one.")),
		libmcp.WithString("notes", libmcp.Description("Optional notes")),
		withSource(),
	}
}

func addConstraintTool() libmcp.Tool {
	opts := []libmcp.ToolOption{
		libmcp.WithDescription("Record a constraint: a testable cross-functional requirement (performance, availability, security, privacy, accessibility, usability, compliance, operability, technology) with what's measured and its target. Emitted as a scenario in constraints.feature; a slice isn't ready while a constraint that applies to it lacks a measure or target."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("text", libmcp.Required(), libmcp.Description("The constraint, e.g. 'the register shows promptly'")),
	}
	return libmcp.NewTool("add_constraint", append(opts, constraintOptions()...)...)
}

func handleAddConstraint(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "text")
	if err != nil {
		return toolError(err)
	}
	_, c, err := session.AddConstraint(ids[0], session.Constraint{
		Text:      ids[1],
		Kind:      session.ConstraintKind(req.GetString("kind", "")),
		ScopeKind: req.GetString("scope_kind", ""),
		ScopeID:   req.GetString("scope_id", ""),
		Measure:   req.GetString("measure", ""),
		Target:    req.GetString("target", ""),
		Notes:     req.GetString("notes", ""),
		Source:    session.Source(req.GetString("source", "")),
	})
	if err != nil {
		return toolError(err)
	}
	return resultJSON(c)
}

func updateConstraintTool() libmcp.Tool {
	opts := []libmcp.ToolOption{
		libmcp.WithDescription("Edit a constraint. Omitted fields are unchanged. Remove one with remove_item (kind constraint)."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("constraint_id", libmcp.Required(), libmcp.Description("ID of the constraint")),
		libmcp.WithString("text", libmcp.Description("New text")),
	}
	return libmcp.NewTool("update_constraint", append(opts, constraintOptions()...)...)
}

func handleUpdateConstraint(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "constraint_id")
	if err != nil {
		return toolError(err)
	}
	var p session.ConstraintPatch
	var kind *string
	if err := optStringFields(req, map[string]**string{
		"text": &p.Text, "scope_kind": &p.ScopeKind, "scope_id": &p.ScopeID,
		"measure": &p.Measure, "target": &p.Target, "notes": &p.Notes, "kind": &kind,
	}); err != nil {
		return toolError(err)
	}
	if kind != nil {
		k := session.ConstraintKind(*kind)
		p.Kind = &k
	}
	if p.Source, err = optSource(req); err != nil {
		return toolError(err)
	}
	_, c, err := session.UpdateConstraint(ids[0], ids[1], p)
	if err != nil {
		return toolError(err)
	}
	return resultJSON(c)
}

func checkReadinessTool() libmcp.Tool {
	return libmcp.NewTool("check_readiness",
		libmcp.WithDescription("Check whether each slice is ready for a factory, and what blocks it: stories without a why or acceptance criteria, steps with no screen (unless off_screen), steps using a command or read model their screen doesn't offer or show, outcomes that can't be read from the screen, offered commands with no domain behaviour, constraints without a measure and target, and glossary violations. Notes (UI mechanics in criteria, stories split by technical layer, offered commands no journey uses) don't block."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("slice_id", libmcp.Description("Only this slice")),
	)
}

func handleCheckReadiness(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return toolError(err)
	}
	s, err := session.Load(sessionID)
	if err != nil {
		return toolError(err)
	}
	r := session.Readiness(s)
	if id := req.GetString("slice_id", ""); id != "" {
		r = slices.DeleteFunc(r, func(x session.SliceReadiness) bool { return x.SliceID != id })
		if len(r) == 0 {
			return toolError(fmt.Errorf("slice %s not found", id))
		}
	}
	return resultJSON(r)
}

func emitStoryMapTool() libmcp.Tool {
	return libmcp.NewTool("emit_story_map",
		libmcp.WithDescription("Emit the story map's specifications beside the storm's: per slice, one journey feature with a scenario per role (@running, @journey when taken through screens, @walking-skeleton on the skeleton); each story's acceptance criteria as scenarios; per screen, a scenario per read model it shows and per command it offers; constraints.feature; story-map/slices.json (each slice's readiness, build order and files); and story-map/story-map.json and .md. Features go under the screens' front-end context directory. Returns what was written, each slice's readiness, and glossary lint."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("output_dir", libmcp.Required(), libmcp.Description("Directory to write into, usually the same one the storm's specs go to")),
		libmcp.WithArray("only", libmcp.WithStringItems(libmcp.Enum(gherkin.StoryMapOutputs...)),
			libmcp.Description("Write only these outputs: journeys, stories, screens, constraints, slices, map. Default is all."),
		),
	)
}

func handleEmitStoryMap(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "output_dir")
	if err != nil {
		return toolError(err)
	}
	s, err := session.Load(ids[0])
	if err != nil {
		return toolError(err)
	}
	sum, err := gherkin.EmitStoryMap(s, ids[1], gherkin.StoryMapOptions{Only: req.GetStringSlice("only", nil)})
	if err != nil {
		return toolError(err)
	}
	return resultJSON(sum)
}
