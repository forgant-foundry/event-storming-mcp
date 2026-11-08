package mcp

import (
	"context"
	"fmt"
	"strings"

	libmcp "github.com/mark3labs/mcp-go/mcp"

	"github.com/forgant-foundry/event-storming-mcp/internal/session"
)

// Update tools take every field as optional: an omitted field is left
// unchanged, an empty string clears it.

const sourceDescription = "Who this came from: expert (the domain expert said it), facilitator (the facilitator proposed it and the expert confirmed), or imported (seeded from existing material)"

func withSource() libmcp.ToolOption {
	return libmcp.WithString("source",
		libmcp.Enum(string(session.SourceExpert), string(session.SourceFacilitator), string(session.SourceImported)),
		libmcp.Description(sourceDescription),
	)
}

func withEventPhrasing() libmcp.ToolOption {
	return libmcp.WithString("phrasing",
		libmcp.Description("Then-step text used verbatim in emitted Gherkin, without the keyword (e.g. 'the scope is passed to the factory'). Set it when the name is long or doesn't read as '<noun> is <participle>'; empty string reverts to the derived text."),
	)
}

func withCommandPhrasing() libmcp.ToolOption {
	return libmcp.WithString("phrasing",
		libmcp.Description("When-step text used verbatim in emitted Gherkin, including the actor, without the keyword (e.g. 'the product manager prioritizes the surfaces'). Also used as the scenario title. Set it when the name doesn't read as '<verb> a <object>'; empty string reverts to the derived text."),
	)
}

// optString returns nil when key is absent, so the caller can tell "leave
// unchanged" from "set to empty".
func optString(req libmcp.CallToolRequest, key string) (*string, error) {
	v, ok := req.GetArguments()[key]
	if !ok || v == nil {
		return nil, nil
	}
	s, ok := v.(string)
	if !ok {
		return nil, fmt.Errorf("%s must be a string", key)
	}
	return &s, nil
}

func optStrings(req libmcp.CallToolRequest, key string) (*[]string, error) {
	v, ok := req.GetArguments()[key]
	if !ok || v == nil {
		return nil, nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array of strings", key)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s must be an array of strings", key)
		}
		out = append(out, s)
	}
	return &out, nil
}

func optSource(req libmcp.CallToolRequest) (*session.Source, error) {
	s, err := optString(req, "source")
	if s == nil || err != nil {
		return nil, err
	}
	src := session.Source(*s)
	return &src, nil
}

// optStringFields reads several optional string arguments into patch fields.
func optStringFields(req libmcp.CallToolRequest, fields map[string]**string) error {
	for key, dst := range fields {
		v, err := optString(req, key)
		if err != nil {
			return err
		}
		*dst = v
	}
	return nil
}

func requireIDs(req libmcp.CallToolRequest, keys ...string) ([]string, error) {
	ids := make([]string, len(keys))
	for i, k := range keys {
		v, err := req.RequireString(k)
		if err != nil {
			return nil, err
		}
		ids[i] = v
	}
	return ids, nil
}

func updateEventTool() libmcp.Tool {
	return libmcp.NewTool("update_event",
		libmcp.WithDescription("Edit an event's name, notes, phrasing or source. Omitted fields are unchanged. Use link_event to change its command and set_event_condition to change its condition."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("event_id", libmcp.Required(), libmcp.Description("ID of the event to edit")),
		libmcp.WithString("name", libmcp.Description("New event name in past tense")),
		libmcp.WithString("notes", libmcp.Description("New notes; empty string clears them")),
		withEventPhrasing(),
		withSource(),
	)
}

func handleUpdateEvent(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "event_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	var p session.EventPatch
	if err := optStringFields(req, map[string]**string{"name": &p.Name, "notes": &p.Notes, "phrasing": &p.Phrasing}); err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	if p.Source, err = optSource(req); err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	_, e, err := session.UpdateEvent(ids[0], ids[1], p)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(e)
}

func updateCommandTool() libmcp.Tool {
	return libmcp.NewTool("update_command",
		libmcp.WithDescription("Edit a command's name, notes, actor, preconditions, phrasing or source. Omitted fields are unchanged; preconditions, when given, replace the whole list. Use assign_to_aggregate to change its aggregate."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("command_id", libmcp.Required(), libmcp.Description("ID of the command to edit")),
		libmcp.WithString("name", libmcp.Description("New command name in imperative form")),
		libmcp.WithString("notes", libmcp.Description("New notes; empty string clears them")),
		libmcp.WithString("actor_id", libmcp.Description("ID of the actor issuing the command; empty string clears it")),
		libmcp.WithArray("preconditions", libmcp.WithStringItems(), libmcp.Description("Replacement list of preconditions; empty array clears them")),
		withCommandPhrasing(),
		withSource(),
	)
}

func handleUpdateCommand(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "command_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	var p session.CommandPatch
	if err := optStringFields(req, map[string]**string{"name": &p.Name, "notes": &p.Notes, "actor_id": &p.ActorID, "phrasing": &p.Phrasing}); err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	if p.Preconditions, err = optStrings(req, "preconditions"); err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	if p.Source, err = optSource(req); err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	_, c, err := session.UpdateCommand(ids[0], ids[1], p)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(c)
}

func updateActorTool() libmcp.Tool {
	return libmcp.NewTool("update_actor",
		libmcp.WithDescription("Edit an actor's name, type or notes. Omitted fields are unchanged."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("actor_id", libmcp.Required(), libmcp.Description("ID of the actor to edit")),
		libmcp.WithString("name", libmcp.Description("New actor name")),
		libmcp.WithString("type", libmcp.Enum("person", "role", "external_system"), libmcp.Description("One of: person, role, external_system")),
		libmcp.WithString("notes", libmcp.Description("New notes; empty string clears them")),
	)
}

func handleUpdateActor(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "actor_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	var p session.ActorPatch
	var typ *string
	if err := optStringFields(req, map[string]**string{"name": &p.Name, "notes": &p.Notes, "type": &typ}); err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	if typ != nil {
		t := session.ActorType(*typ)
		p.Type = &t
	}
	_, a, err := session.UpdateActor(ids[0], ids[1], p)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(a)
}

func updatePolicyTool() libmcp.Tool {
	return libmcp.NewTool("update_policy",
		libmcp.WithDescription("Edit a policy. Omitted fields are unchanged; an empty string clears a field. Use condition for policies that apply only in some cases, e.g. 'whenever CompletenessAssessed with no gaps'."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("policy_id", libmcp.Required(), libmcp.Description("ID of the policy to edit")),
		libmcp.WithString("name", libmcp.Description("New policy name")),
		libmcp.WithString("trigger_event_id", libmcp.Description("ID of the event that triggers the policy")),
		libmcp.WithString("resulting_command_id", libmcp.Description("ID of the command the policy issues")),
		libmcp.WithString("context_id", libmcp.Description("ID of the bounded context the policy belongs to")),
		libmcp.WithString("condition", libmcp.Description("Condition under which the policy applies, beyond the trigger event")),
		libmcp.WithString("notes", libmcp.Description("New notes")),
		withSource(),
	)
}

func handleUpdatePolicy(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "policy_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	var p session.PolicyPatch
	if err := optStringFields(req, map[string]**string{
		"name": &p.Name, "notes": &p.Notes, "trigger_event_id": &p.TriggerEventID,
		"resulting_command_id": &p.ResultingCommandID, "context_id": &p.ContextID, "condition": &p.Condition,
	}); err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	if p.Source, err = optSource(req); err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	_, pol, err := session.UpdatePolicy(ids[0], ids[1], p)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(pol)
}

func updateReadModelTool() libmcp.Tool {
	return libmcp.NewTool("update_read_model",
		libmcp.WithDescription("Edit a read model. Omitted fields are unchanged; actor_ids, when given, replaces the whole list of readers."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("read_model_id", libmcp.Required(), libmcp.Description("ID of the read model to edit")),
		libmcp.WithString("name", libmcp.Description("New read model name")),
		libmcp.WithArray("actor_ids", libmcp.WithStringItems(), libmcp.Description("Replacement list of the actors who consult this read model")),
		libmcp.WithString("context_id", libmcp.Description("ID of the bounded context; empty string clears it")),
		libmcp.WithString("notes", libmcp.Description("New notes")),
		withSource(),
	)
}

func handleUpdateReadModel(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "read_model_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	var p session.ReadModelPatch
	if err := optStringFields(req, map[string]**string{"name": &p.Name, "notes": &p.Notes, "context_id": &p.ContextID}); err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	if p.ActorIDs, err = optStrings(req, "actor_ids"); err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	if p.Source, err = optSource(req); err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	_, rm, err := session.UpdateReadModel(ids[0], ids[1], p)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(rm)
}

func updateAggregateTool() libmcp.Tool {
	return libmcp.NewTool("update_aggregate",
		libmcp.WithDescription("Edit an aggregate's name or notes (a 'Rule: …' note is emitted as a Gherkin Rule). Omitted fields are unchanged. Use assign_to_aggregate to change its members and assign_to_context to change its context."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("aggregate_id", libmcp.Required(), libmcp.Description("ID of the aggregate to edit")),
		libmcp.WithString("name", libmcp.Description("New aggregate name")),
		libmcp.WithString("notes", libmcp.Description("New notes; empty string clears them")),
	)
}

func handleUpdateAggregate(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "aggregate_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	var p session.AggregatePatch
	if err := optStringFields(req, map[string]**string{"name": &p.Name, "notes": &p.Notes}); err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	_, a, err := session.UpdateAggregate(ids[0], ids[1], p)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(a)
}

func updateContextTool() libmcp.Tool {
	return libmcp.NewTool("update_bounded_context",
		libmcp.WithDescription("Edit a bounded context's name or description. Omitted fields are unchanged. Use assign_to_context to change its members."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("context_id", libmcp.Required(), libmcp.Description("ID of the bounded context to edit")),
		libmcp.WithString("name", libmcp.Description("New context name")),
		libmcp.WithString("description", libmcp.Description("New description")),
	)
}

func handleUpdateContext(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "context_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	var p session.ContextPatch
	if err := optStringFields(req, map[string]**string{"name": &p.Name, "description": &p.Description}); err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	_, c, err := session.UpdateContext(ids[0], ids[1], p)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(c)
}

func assignToAggregateTool() libmcp.Tool {
	return libmcp.NewTool("assign_to_aggregate",
		libmcp.WithDescription("Add existing events and commands to an existing aggregate, e.g. a command that turned up after clustering. An item already in a different aggregate is rejected unless move is true."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("aggregate_id", libmcp.Required(), libmcp.Description("ID of the aggregate to add to")),
		libmcp.WithArray("event_ids", libmcp.WithStringItems(), libmcp.Description("IDs of events to add")),
		libmcp.WithArray("command_ids", libmcp.WithStringItems(), libmcp.Description("IDs of commands to add")),
		libmcp.WithBoolean("move", libmcp.Description("Move items out of the aggregate they belong to now (default false)")),
	)
}

func handleAssignToAggregate(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "aggregate_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	_, a, err := session.AssignToAggregate(ids[0], ids[1],
		req.GetStringSlice("event_ids", nil),
		req.GetStringSlice("command_ids", nil),
		req.GetBool("move", false),
	)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(a)
}

func assignToContextTool() libmcp.Tool {
	return libmcp.NewTool("assign_to_context",
		libmcp.WithDescription("Add existing aggregates and glossary terms to an existing bounded context. An item already in a different context is rejected unless move is true."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("context_id", libmcp.Required(), libmcp.Description("ID of the bounded context to add to")),
		libmcp.WithArray("aggregate_ids", libmcp.WithStringItems(), libmcp.Description("IDs of aggregates to add")),
		libmcp.WithArray("glossary_term_ids", libmcp.WithStringItems(), libmcp.Description("IDs of glossary terms to add")),
		libmcp.WithBoolean("move", libmcp.Description("Move items out of the context they belong to now (default false)")),
	)
}

func handleAssignToContext(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	ids, err := requireIDs(req, "session_id", "context_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	_, c, err := session.AssignToContext(ids[0], ids[1],
		req.GetStringSlice("aggregate_ids", nil),
		req.GetStringSlice("glossary_term_ids", nil),
		req.GetBool("move", false),
	)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(c)
}

func removeItemTool() libmcp.Tool {
	return libmcp.NewTool("remove_item",
		libmcp.WithDescription("Delete any record and detach it from everything that references it (e.g. removing a command clears trigger_command_id on its events and resulting_command_id on policies). Returns the list of detachments so you can tell the expert what else changed. Prefer deprecate_aggregate when an aggregate's history should stay visible."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("kind", libmcp.Required(), libmcp.Enum(session.Kinds...),
			libmcp.Description("One of: "+strings.Join(session.Kinds, ", ")),
		),
		libmcp.WithString("id", libmcp.Required(), libmcp.Description("ID of the record to remove")),
	)
}

func handleRemoveItem(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	args, err := requireIDs(req, "session_id", "kind", "id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	_, detached, err := session.RemoveItem(args[0], args[1], args[2])
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(map[string]any{
		"removed":  args[2],
		"kind":     args[1],
		"detached": detached,
	})
}
