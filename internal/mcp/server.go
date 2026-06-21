package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	libmcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/forgant-foundry/event-storming-mcp/internal/gherkin"
	"github.com/forgant-foundry/event-storming-mcp/internal/playbooks"
	"github.com/forgant-foundry/event-storming-mcp/internal/session"
)

func New() *server.MCPServer {
	s := server.NewMCPServer("event-storming-mcp", "0.1.0")

	s.AddTool(startSessionTool(), handleStartSession)
	s.AddTool(resumeSessionTool(), handleResumeSession)
	s.AddTool(addEventTool(), handleAddEvent)
	s.AddTool(addCommandTool(), handleAddCommand)
	s.AddTool(addActorTool(), handleAddActor)
	s.AddTool(linkEventTool(), handleLinkEvent)
	s.AddTool(removeEventTool(), handleRemoveEvent)
	s.AddTool(addPolicyTool(), handleAddPolicy)
	s.AddTool(addReadModelTool(), handleAddReadModel)
	s.AddTool(addHotSpotTool(), handleAddHotSpot)
	s.AddTool(defineTermTool(), handleDefineTerm)
	s.AddTool(clusterAggregateTool(), handleClusterAggregate)
	s.AddTool(proposeContextTool(), handleProposeContext)
	s.AddTool(emitGherkinTool(), handleEmitGherkin)
	s.AddTool(lintStepsTool(), handleLintSteps)
	s.AddTool(listPlaybooksTool(), handleListPlaybooks)
	s.AddTool(getPlaybookTool(), handleGetPlaybook)
	s.AddTool(exportSessionTool(), handleExportSession)
	s.AddTool(importSessionTool(), handleImportSession)
	s.AddTool(setSyncPathTool(), handleSetSyncPath)

	return s
}

func resultJSON(v any) (*libmcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return libmcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
	}
	return libmcp.NewToolResultText(string(data)), nil
}

func startSessionTool() libmcp.Tool {
	return libmcp.NewTool("start_session",
		libmcp.WithDescription("Begin a new event storming session. Returns the session ID to use in subsequent calls."),
		libmcp.WithString("name",
			libmcp.Required(),
			libmcp.Description("Short name for the session (e.g. 'E-commerce checkout')"),
		),
		libmcp.WithString("description",
			libmcp.Required(),
			libmcp.Description("One or two sentences describing the domain being explored"),
		),
		libmcp.WithString("sync_path",
			libmcp.Description("Optional project-local file path to keep in sync on every save (e.g. '.event-storming/checkout.json'). Commit this file to share the session."),
		),
	)
}

func handleStartSession(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	name, err := req.RequireString("name")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	s, err := session.New(name, req.GetString("description", ""))
	if err != nil {
		return libmcp.NewToolResultError(fmt.Sprintf("could not create session: %v", err)), nil
	}
	if syncPath := req.GetString("sync_path", ""); syncPath != "" {
		s, err = session.SetSyncPath(s.ID, syncPath)
		if err != nil {
			return libmcp.NewToolResultError(fmt.Sprintf("could not set sync path: %v", err)), nil
		}
	}
	return resultJSON(map[string]any{
		"session_id":  s.ID,
		"name":        s.Name,
		"description": s.Description,
		"sync_path":   s.SyncPath,
		"created_at":  s.CreatedAt,
	})
}

func resumeSessionTool() libmcp.Tool {
	return libmcp.NewTool("resume_session",
		libmcp.WithDescription("Reattach to an existing session. Returns the full current model."),
		libmcp.WithString("session_id",
			libmcp.Required(),
			libmcp.Description("Session ID returned by start_session"),
		),
	)
}

func handleResumeSession(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	id, err := req.RequireString("session_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	s, err := session.Load(id)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(s)
}

func addEventTool() libmcp.Tool {
	return libmcp.NewTool("add_event",
		libmcp.WithDescription("Record a domain event (something that happened, past tense). Returns the event record."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("name",
			libmcp.Required(),
			libmcp.Description("Event name in past tense (e.g. 'OrderPlaced', 'PaymentFailed')"),
		),
		libmcp.WithString("notes", libmcp.Description("Optional clarifying notes")),
		libmcp.WithString("trigger_command_id", libmcp.Description("ID of the command that triggered this event")),
		libmcp.WithString("aggregate_id", libmcp.Description("ID of the aggregate this event belongs to")),
	)
}

func handleAddEvent(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	_, e, err := session.AddEvent(
		sessionID, name,
		req.GetString("notes", ""),
		req.GetString("trigger_command_id", ""),
		req.GetString("aggregate_id", ""),
	)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(e)
}

func addCommandTool() libmcp.Tool {
	return libmcp.NewTool("add_command",
		libmcp.WithDescription("Record a command (an action a user or system can perform). Returns the command record."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("name",
			libmcp.Required(),
			libmcp.Description("Command name in imperative form (e.g. 'PlaceOrder', 'CancelSubscription')"),
		),
		libmcp.WithString("notes", libmcp.Description("Optional clarifying notes")),
		libmcp.WithString("actor_id", libmcp.Description("ID of the actor issuing this command")),
		libmcp.WithString("aggregate_id", libmcp.Description("ID of the aggregate receiving this command")),
		libmcp.WithArray("preconditions",
			libmcp.Description("Precondition strings that must hold before the command can be issued"),
		),
	)
}

func handleAddCommand(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	_, c, err := session.AddCommand(
		sessionID, name,
		req.GetString("notes", ""),
		req.GetString("actor_id", ""),
		req.GetString("aggregate_id", ""),
		req.GetStringSlice("preconditions", nil),
	)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(c)
}

func addActorTool() libmcp.Tool {
	return libmcp.NewTool("add_actor",
		libmcp.WithDescription("Record an actor (person, role, or external system) who issues commands. Returns the actor record."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("name",
			libmcp.Required(),
			libmcp.Description("Actor name (e.g. 'Customer', 'Fraud Detection Service')"),
		),
		libmcp.WithString("type",
			libmcp.Required(),
			libmcp.Description("One of: person, role, external_system"),
		),
		libmcp.WithString("notes", libmcp.Description("Optional clarifying notes")),
	)
}

func handleAddActor(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	actorType := session.ActorType(req.GetString("type", ""))
	switch actorType {
	case session.ActorTypePerson, session.ActorTypeRole, session.ActorTypeExternalSystem:
	default:
		return libmcp.NewToolResultError(
			fmt.Sprintf("type must be one of: person, role, external_system; got %q", actorType),
		), nil
	}
	_, a, err := session.AddActor(sessionID, name, actorType, req.GetString("notes", ""))
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(a)
}

func defineTermTool() libmcp.Tool {
	return libmcp.NewTool("define_term",
		libmcp.WithDescription("Add or update a glossary entry. If the term already exists it is updated in place. Returns the glossary term record."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("term",
			libmcp.Required(),
			libmcp.Description("The term to define (e.g. 'Order')"),
		),
		libmcp.WithString("definition",
			libmcp.Required(),
			libmcp.Description("Clear, concise definition agreed on by domain experts"),
		),
		libmcp.WithString("canonical_phrasing",
			libmcp.Description("Preferred phrasing in Gherkin steps (e.g. 'the order')"),
		),
		libmcp.WithArray("aliases",
			libmcp.Description("Synonyms or alternative names to lint against (e.g. ['cart', 'basket'])"),
		),
		libmcp.WithString("context_id", libmcp.Description("ID of the bounded context this term belongs to")),
	)
}

func handleDefineTerm(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	term, err := req.RequireString("term")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	definition, err := req.RequireString("definition")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	_, gt, err := session.DefineTerm(
		sessionID, term, definition,
		req.GetString("canonical_phrasing", ""),
		req.GetString("context_id", ""),
		req.GetStringSlice("aliases", nil),
	)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(gt)
}

func linkEventTool() libmcp.Tool {
	return libmcp.NewTool("link_event",
		libmcp.WithDescription("Set the trigger_command_id on an existing event, linking it to the command that caused it. Use this during the Phase 2 linking pass rather than re-adding the event (which would create a duplicate)."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("event_id", libmcp.Required(), libmcp.Description("ID of the event to update")),
		libmcp.WithString("command_id", libmcp.Required(), libmcp.Description("ID of the command that triggers this event")),
	)
}

func handleLinkEvent(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	eventID, err := req.RequireString("event_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	commandID, err := req.RequireString("command_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	_, e, err := session.LinkEvent(sessionID, eventID, commandID)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(e)
}

func removeEventTool() libmcp.Tool {
	return libmcp.NewTool("remove_event",
		libmcp.WithDescription("Delete an event from the session and remove it from any aggregate that references it. Use this to prune spurious events or clean up duplicates created by re-adding events instead of using link_event."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("event_id", libmcp.Required(), libmcp.Description("ID of the event to remove")),
	)
}

func handleRemoveEvent(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	eventID, err := req.RequireString("event_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	_, err = session.RemoveEvent(sessionID, eventID)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(map[string]any{"removed": eventID})
}

func addPolicyTool() libmcp.Tool {
	return libmcp.NewTool("add_policy",
		libmcp.WithDescription("Record a reactive policy: whenever a trigger event occurs, issue a resulting command. Returns the policy record."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("name",
			libmcp.Required(),
			libmcp.Description("Policy name (e.g. 'Notify warehouse on order placed')"),
		),
		libmcp.WithString("trigger_event_id", libmcp.Description("ID of the event that triggers this policy")),
		libmcp.WithString("resulting_command_id", libmcp.Description("ID of the command this policy issues")),
		libmcp.WithString("context_id", libmcp.Description("ID of the bounded context this policy belongs to")),
		libmcp.WithString("notes", libmcp.Description("Optional clarifying notes")),
	)
}

func handleAddPolicy(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	_, p, err := session.AddPolicy(
		sessionID, name,
		req.GetString("notes", ""),
		req.GetString("trigger_event_id", ""),
		req.GetString("resulting_command_id", ""),
		req.GetString("context_id", ""),
	)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(p)
}

func addReadModelTool() libmcp.Tool {
	return libmcp.NewTool("add_read_model",
		libmcp.WithDescription("Record a read model (a view an actor consults before issuing a command). Returns the read model record."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("name",
			libmcp.Required(),
			libmcp.Description("Read model name (e.g. 'Order Summary', 'Inventory Levels')"),
		),
		libmcp.WithString("actor_id", libmcp.Description("ID of the actor who consults this read model")),
		libmcp.WithString("context_id", libmcp.Description("ID of the bounded context this read model belongs to")),
		libmcp.WithString("notes", libmcp.Description("Optional clarifying notes")),
	)
}

func handleAddReadModel(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	_, rm, err := session.AddReadModel(
		sessionID, name,
		req.GetString("notes", ""),
		req.GetString("actor_id", ""),
		req.GetString("context_id", ""),
	)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(rm)
}

func addHotSpotTool() libmcp.Tool {
	return libmcp.NewTool("add_hot_spot",
		libmcp.WithDescription("Flag an unresolved question or disagreement for follow-up. Returns the hot spot record."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("question",
			libmcp.Required(),
			libmcp.Description("The open question or point of disagreement (e.g. 'Who owns refund approval?')"),
		),
		libmcp.WithString("context_id", libmcp.Description("ID of the bounded context this question relates to")),
	)
}

func handleAddHotSpot(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	question, err := req.RequireString("question")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	_, hs, err := session.AddHotSpot(sessionID, question, req.GetString("context_id", ""))
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(hs)
}

func clusterAggregateTool() libmcp.Tool {
	return libmcp.NewTool("cluster_aggregate",
		libmcp.WithDescription("Group events and commands into an aggregate (consistency boundary). Each event and command may only belong to one aggregate. Returns the aggregate record."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("name",
			libmcp.Required(),
			libmcp.Description("Aggregate name (e.g. 'Order', 'Payment')"),
		),
		libmcp.WithArray("event_ids",
			libmcp.Description("IDs of events to assign to this aggregate"),
		),
		libmcp.WithArray("command_ids",
			libmcp.Description("IDs of commands to assign to this aggregate"),
		),
		libmcp.WithString("context_id", libmcp.Description("ID of the bounded context this aggregate belongs to")),
		libmcp.WithString("notes", libmcp.Description("Optional clarifying notes")),
	)
}

func handleClusterAggregate(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	_, agg, err := session.ClusterAggregate(
		sessionID, name,
		req.GetString("notes", ""),
		req.GetString("context_id", ""),
		req.GetStringSlice("event_ids", nil),
		req.GetStringSlice("command_ids", nil),
	)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(agg)
}

func proposeContextTool() libmcp.Tool {
	return libmcp.NewTool("propose_bounded_context",
		libmcp.WithDescription("Group aggregates and glossary terms into a bounded context. Each aggregate may only belong to one context. Returns the bounded context record."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("name",
			libmcp.Required(),
			libmcp.Description("Bounded context name (e.g. 'Ordering', 'Payments', 'Fulfillment')"),
		),
		libmcp.WithString("description", libmcp.Description("What this context is responsible for")),
		libmcp.WithArray("aggregate_ids",
			libmcp.Description("IDs of aggregates to place in this context"),
		),
		libmcp.WithArray("glossary_term_ids",
			libmcp.Description("IDs of glossary terms that belong to this context's ubiquitous language"),
		),
	)
}

func handleProposeContext(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	_, ctx, err := session.ProposeContext(
		sessionID, name,
		req.GetString("description", ""),
		req.GetStringSlice("aggregate_ids", nil),
		req.GetStringSlice("glossary_term_ids", nil),
	)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(ctx)
}

func emitGherkinTool() libmcp.Tool {
	return libmcp.NewTool("emit_gherkin",
		libmcp.WithDescription("Generate the output triple for this session: .feature files (one per aggregate), glossary.md, and context-map.json. Returns a summary of what was written and any warnings."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("output_dir",
			libmcp.Required(),
			libmcp.Description("Filesystem path to write output files into (will be created if absent)"),
		),
	)
}

func handleEmitGherkin(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	outDir, err := req.RequireString("output_dir")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	s, err := session.Load(sessionID)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	sum, err := gherkin.Emit(s, outDir)
	if err != nil {
		return libmcp.NewToolResultError(fmt.Sprintf("emit failed: %v", err)), nil
	}
	return resultJSON(sum)
}

func lintStepsTool() libmcp.Tool {
	return libmcp.NewTool("lint_steps",
		libmcp.WithDescription("Check a candidate Gherkin step string against the session glossary. Returns alias violations (use canonical phrasing instead) and cross-context term leakage."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("step_text",
			libmcp.Required(),
			libmcp.Description("The step text to check, e.g. 'the user adds items to their cart'"),
		),
		libmcp.WithString("context_id",
			libmcp.Description("ID of the bounded context the step belongs to (enables cross-context checks)"),
		),
	)
}

func handleLintSteps(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	stepText, err := req.RequireString("step_text")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	s, err := session.Load(sessionID)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	issues := session.LintStep(s, req.GetString("context_id", ""), stepText)
	return resultJSON(map[string]any{
		"clean":  len(issues) == 0,
		"issues": issues,
	})
}

func listPlaybooksTool() libmcp.Tool {
	return libmcp.NewTool("list_playbooks",
		libmcp.WithDescription("List all available facilitation playbooks with their slugs, titles, and descriptions."),
	)
}

func handleListPlaybooks(_ context.Context, _ libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	return resultJSON(playbooks.All())
}

func getPlaybookTool() libmcp.Tool {
	return libmcp.NewTool("get_playbook",
		libmcp.WithDescription("Return the full facilitation script for a playbook as markdown."),
		libmcp.WithString("slug",
			libmcp.Required(),
			libmcp.Description("Playbook slug (from list_playbooks): run-event-storming | extract-ubiquitous-language | emit-gherkin-specs"),
		),
	)
}

func handleGetPlaybook(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	slug, err := req.RequireString("slug")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	p, ok := playbooks.Get(slug)
	if !ok {
		return libmcp.NewToolResultError(fmt.Sprintf("playbook %q not found; call list_playbooks to see available slugs", slug)), nil
	}
	return libmcp.NewToolResultText(p.Content), nil
}

func exportSessionTool() libmcp.Tool {
	return libmcp.NewTool("export_session",
		libmcp.WithDescription("Export a session to a portable JSON file. Commit this file to version control to share the session with teammates or resume it on another machine."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("path",
			libmcp.Required(),
			libmcp.Description("Destination file path (e.g. '.event-storming/checkout.json')"),
		),
	)
}

func handleExportSession(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	path, err := req.RequireString("path")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	if err := session.ExportSession(sessionID, path); err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	s, _ := session.Load(sessionID)
	return resultJSON(map[string]any{
		"path":       path,
		"session_id": sessionID,
		"name":       s.Name,
	})
}

func importSessionTool() libmcp.Tool {
	return libmcp.NewTool("import_session",
		libmcp.WithDescription("Import a session from a portable JSON file. Assigns a fresh local session ID to avoid collisions. Use the returned session_id for all subsequent calls."),
		libmcp.WithString("path",
			libmcp.Required(),
			libmcp.Description("Path to an exported session JSON file"),
		),
	)
}

func handleImportSession(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	path, err := req.RequireString("path")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	s, originalID, err := session.ImportSession(path)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(map[string]any{
		"session_id":          s.ID,
		"original_session_id": originalID,
		"name":                s.Name,
		"description":         s.Description,
	})
}

func setSyncPathTool() libmcp.Tool {
	return libmcp.NewTool("set_sync_path",
		libmcp.WithDescription("Configure a project-local file that is kept in sync with every save going forward. Pass an empty string to disable. Useful for enabling live sync on a session that was started without one."),
		libmcp.WithString("session_id", libmcp.Required(), libmcp.Description("Session ID")),
		libmcp.WithString("path",
			libmcp.Required(),
			libmcp.Description("Project-local file path (e.g. '.event-storming/checkout.json'), or empty string to disable"),
		),
	)
}

func handleSetSyncPath(_ context.Context, req libmcp.CallToolRequest) (*libmcp.CallToolResult, error) {
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	path, err := req.RequireString("path")
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	s, err := session.SetSyncPath(sessionID, path)
	if err != nil {
		return libmcp.NewToolResultError(err.Error()), nil
	}
	return resultJSON(map[string]any{
		"session_id": s.ID,
		"sync_path":  s.SyncPath,
	})
}
