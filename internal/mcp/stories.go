package mcp

import (
	"context"
	"fmt"

	libmcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/forgant-foundry/forgant-discovery/internal/playbooks"
	"github.com/forgant-foundry/forgant-discovery/internal/session"
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
	_, st, err := session.AddStory(ids[0], session.Story{
		StepID:  ids[1],
		Title:   ids[2],
		Why:     req.GetString("why", ""),
		Kind:    session.StoryKind(req.GetString("kind", "")),
		SliceID: req.GetString("slice_id", ""),
		EventID: req.GetString("event_id", ""),
		Notes:   req.GetString("notes", ""),
		Source:  session.Source(req.GetString("source", "")),
	}, req.GetString("before_id", ""))
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
		Name:   ids[1],
		Shows:  req.GetStringSlice("shows", nil),
		Notes:  req.GetString("notes", ""),
		Source: session.Source(req.GetString("source", "")),
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
	if err := optStringFields(req, map[string]**string{"name": &p.Name, "notes": &p.Notes}); err != nil {
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
