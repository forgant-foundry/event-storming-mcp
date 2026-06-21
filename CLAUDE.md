# event-storming-mcp — Architecture

MCP server that facilitates event storming workshops. The LLM drives the conversation;
the server owns the grammar (event storming stencil + Gherkin emission) and durable
state.

## Package Layout

```
cmd/
  root.go       cobra root command
  serve.go      "serve" subcommand — starts MCP server over stdio

internal/
  session/
    types.go    All domain types: Event, Command, Actor, Policy, ReadModel,
                ExternalSystem, HotSpot, Aggregate, BoundedContext, GlossaryTerm, Session
    store.go    JSON-on-disk persistence ($XDG_DATA_HOME/event-storming-mcp/<id>/session.json)
                Mutation functions: New, Load, Save, AddEvent, AddCommand, AddActor,
                AddPolicy, AddReadModel, AddHotSpot, ClusterAggregate, ProposeContext,
                DefineTerm
    lint.go     LintStep — checks step text against glossary (alias violations,
                cross-context leakage)

  gherkin/
    style.go    Text helpers: CamelCase → words, WhenStep, ThenStep, ScenarioTitle, Slug
    emit.go     Emit — writes .feature files + glossary.md + context-map.json

  playbooks/
    playbooks.go  Embedded markdown registry; All() and Get(slug)
    data/
      run-event-storming.md
      extract-ubiquitous-language.md
      emit-gherkin-specs.md

  mcp/
    server.go   All 15 MCP tool definitions and handlers

main.go
```

## Session Storage

Sessions are stored as JSON files at:

```
$XDG_DATA_HOME/event-storming-mcp/<session-id>/session.json
```

`XDG_DATA_HOME` defaults to `~/.local/share` when not set.

The file contains the entire session graph in a single document. There is no database.

## ID Scheme

All IDs are `prefix_<8 random hex bytes>`:

| Prefix | Type |
|---|---|
| `ses_` | Session |
| `evt_` | Event |
| `cmd_` | Command |
| `act_` | Actor |
| `pol_` | Policy |
| `rm_` | ReadModel |
| `hs_` | HotSpot |
| `agg_` | Aggregate |
| `ctx_` | BoundedContext |
| `term_` | GlossaryTerm |

## MCP Tools (15 total)

### Session management
- `start_session(name, description)` → session_id
- `resume_session(session_id)` → full Session

### Sticky note capture
- `add_event(session_id, name, notes?, trigger_command_id?, aggregate_id?)`
- `add_command(session_id, name, notes?, actor_id?, aggregate_id?, preconditions?)`
- `add_actor(session_id, name, type)` — type: person | role | external_system
- `add_policy(session_id, name, trigger_event_id?, resulting_command_id?)`
- `add_read_model(session_id, name, actor_id?)`
- `add_hot_spot(session_id, question)`

### Glossary
- `define_term(session_id, term, definition, canonical_phrasing?, aliases?, context_id?)`
- `lint_steps(session_id, step_text, context_id?)` → { clean, issues }

### Clustering
- `cluster_aggregate(session_id, name, event_ids?, command_ids?)` — enforces one-aggregate-per-event
- `propose_bounded_context(session_id, name, description?, aggregate_ids?, glossary_term_ids?)` — enforces one-context-per-aggregate

### Emission
- `emit_gherkin(session_id, output_dir)` → Summary (feature files, glossary.md, context-map.json)

### Playbooks
- `list_playbooks()` → [{ slug, title, description }]
- `get_playbook(slug)` → markdown facilitation script

## Validation Invariants

- An event may belong to at most one aggregate (enforced in `ClusterAggregate`)
- A command may belong to at most one aggregate (enforced in `ClusterAggregate`)
- An aggregate may belong to at most one bounded context (enforced in `ProposeContext`)
- "Already assigned to a different X" errors are returned as tool-level errors (not
  Go errors), so the LLM sees them as structured messages it can relay to the user

## Gherkin Emission

`emit.go` derives scenarios from the command→event graph:

```
Given  command.Preconditions (each precondition becomes a Given step)
When   WhenStep(command.Name, actor.Name)
Then   ThenStep(event.Name) for each event where event.TriggerCommandID == command.ID
And    ... (additional events from the same command)
```

Commands with no linked events produce a warning and are omitted.
Aggregates with no commands produce a stub scenario with `# TODO` comments.

## Adding a New Tool

1. Add a mutation function to `internal/session/store.go` (or a query-only function
   if no state change is needed)
2. Add a tool definition function and handler to `internal/mcp/server.go`
3. Register the tool in `New()` in `server.go`
4. Add tests in the relevant `_test.go` file

## Running Locally

```sh
make build
./bin/event-storming-mcp serve   # starts MCP server on stdio
```

## Tests

```sh
make test
```

Tests use `t.Setenv("XDG_DATA_HOME", t.TempDir())` to isolate session storage.
