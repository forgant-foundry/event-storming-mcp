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
                Mutation functions: New, Load, Save, SetSyncPath, ExportSession,
                ImportSession, AddEvent, AddCommand, AddActor, SetEventCondition,
                ResolveHotSpot, DeprecateAggregate, LinkEvent, RemoveEvent, AddPolicy,
                AddReadModel, AddHotSpot, ClusterAggregate, ProposeContext, DefineTerm;
                reconcile (two-sided membership repair, run by Load and Save)
    edit.go     Update*/patch structs, AssignToAggregate, AssignToContext, RemoveItem
                (cascading delete), reference/source validation helpers
    lint.go     LintStep — checks step text against glossary (alias violations,
                cross-context leakage)

  gherkin/
    style.go    Text helpers: CamelCase → words, WhenStep, ThenStep, ScenarioTitle, Slug
    emit.go     Emit(session, dir, Options{Only}) — writes aggregate .feature files,
                policies.feature per context, glossary.md, context-map.json

  playbooks/
    playbooks.go  Embedded markdown registry; All() and Get(slug). Every tool a
                  playbook names must exist — internal/mcp/playbooks_test.go checks
    data/
      run-event-storming.md
      extract-ubiquitous-language.md
      emit-gherkin-specs.md

  mcp/
    server.go   Session, capture, clustering, emission, portability and playbook
                tools; server version comes from Go build info (VCS tag), "dev"
                when unavailable
    edit.go     update_*, assign_to_*, remove_item; optString/optStrings helpers
                that tell an omitted argument from an empty one

main.go
```

## Session Storage

Sessions are stored as JSON files at:

```
$XDG_DATA_HOME/event-storming-mcp/<session-id>/session.json
```

`XDG_DATA_HOME` defaults to `~/.local/share` when not set.

The file contains the entire session graph in a single document. There is no database.

If `Session.SyncPath` is set (via `start_session(sync_path)` or `set_sync_path`), every
`Save` also writes the same JSON to that project-local path.

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

## MCP Tools (33 total)

### Session management
- `start_session(name, description, sync_path?)` → session_id
- `resume_session(session_id)` → full Session

### Sticky note capture
- `add_event(session_id, name, notes?, trigger_command_id?, aggregate_id?, phrasing?, source?)`
- `add_command(session_id, name, notes?, actor_id?, aggregate_id?, preconditions?, phrasing?, source?)`

`phrasing` overrides the derived step text verbatim (for a command it's the whole
When step including the actor, and the scenario title).
- `add_actor(session_id, name, type)` — type: person | role | external_system
- `add_policy(session_id, name, trigger_event_id?, resulting_command_id?, context_id?, condition?, notes?, source?)`
- `add_read_model(session_id, name, actor_ids?, context_id?, notes?, source?)` — `actor_id` still accepted

`source` is expert | facilitator | imported. Facilitator-sourced commands/events emit
their scenarios tagged `@proposed`.
- `add_hot_spot(session_id, question, context_id?)`

### Refinement
- `link_event(session_id, event_id, command_id)` — set trigger_command_id in place (no duplicate)
- `remove_event(session_id, event_id)` — same as `remove_item(kind=event)`
- `set_event_condition(session_id, event_id, condition)` — conditional/rejection event
- `resolve_hotspot(session_id, hotspot_id, resolution)`
- `deprecate_aggregate(session_id, aggregate_id, reason?)`

### Editing
Update tools change only the fields passed; an empty string clears a field, and an
array replaces the whole list.
- `update_event(session_id, event_id, name?, notes?, phrasing?, source?)`
- `update_command(session_id, command_id, name?, notes?, actor_id?, preconditions?, phrasing?, source?)`
- `update_actor(session_id, actor_id, name?, type?, notes?)`
- `update_policy(session_id, policy_id, name?, trigger_event_id?, resulting_command_id?, context_id?, condition?, notes?, source?)`
- `update_read_model(session_id, read_model_id, name?, actor_ids?, context_id?, notes?, source?)`
- `update_aggregate(session_id, aggregate_id, name?, notes?)`
- `update_bounded_context(session_id, context_id, name?, description?)`
- `assign_to_aggregate(session_id, aggregate_id, event_ids?, command_ids?, move?)`
- `assign_to_context(session_id, context_id, aggregate_ids?, glossary_term_ids?, move?)`
- `remove_item(session_id, kind, id)` → { removed, kind, detached } — cascades: clears
  every reference to the removed record and reports each one

### Glossary
- `define_term(session_id, term, definition, canonical_phrasing?, aliases?, context_id?, shared_with?)`
  — `shared_with` lists contexts that use the term on purpose; lint_steps won't flag it there
- `lint_steps(session_id, step_text, context_id?)` → { clean, issues }

### Clustering
- `cluster_aggregate(session_id, name, event_ids?, command_ids?, context_id?)` — enforces one-aggregate-per-event
- `propose_bounded_context(session_id, name, description?, aggregate_ids?, glossary_term_ids?)` — enforces one-context-per-aggregate

### Emission
- `emit_gherkin(session_id, output_dir, only?)` → Summary (feature files, glossary.md,
  context-map.json, scenario_count, open_hot_spots, warnings); `only` is any of
  features | glossary | context_map

### Portability
- `export_session(session_id, path)` — write session JSON verbatim
- `import_session(path, sync_path?)` → new session_id (original ID is replaced to avoid
  collisions); `sync_path` overrides the file's own before the first save
- `set_sync_path(session_id, path)` — empty path disables syncing

### Playbooks
- `list_playbooks()` → [{ slug, title, description }]
- `get_playbook(slug)` → markdown facilitation script

## Validation Invariants

- An event may belong to at most one aggregate (enforced in `ClusterAggregate`)
- A command may belong to at most one aggregate (enforced in `ClusterAggregate`)
- An aggregate may belong to at most one bounded context (enforced in `ProposeContext`)
- Every ID a tool accepts as a reference (actor, command, event, aggregate, context)
  must exist; unknown IDs are rejected
- Mutations set only the member side of a membership; `Save` runs `reconcile`, which
  rebuilds the owner's list
- Membership is stored on both sides (`Aggregate.EventIDs`/`CommandIDs` vs. the
  member's `AggregateID`; `BoundedContext.AggregateIDs`/`GlossaryTermIDs` vs. the
  member's `ContextID`). `Load` and `Save` run `reconcile`, which repairs
  drift: a valid member-side owner wins, list-only entries are claimed, dangling
  owner IDs are cleared
- "Already assigned to a different X" errors are returned as tool-level errors (not
  Go errors), so the LLM sees them as structured messages it can relay to the user

## Gherkin Emission

`emit.go` derives scenarios from the command→event graph:

```
Given  command.Preconditions (each precondition becomes a Given step)
When   command.Phrasing, else WhenStep(command.Name, actor.Name)
Then   event.Phrasing, else ThenStep(event.Name), for each event where
       event.TriggerCommandID == command.ID
And    ... (additional events from the same command)
```

`ThenStep` splits at the last word when it's a participle, otherwise at the first
participle ("ScopePassedToFactory" → "a scope is passed to factory"); a name opening
with an -ed word is used bare. Plurals (trailing s, not -ss/-us/-is) drop the article
and take "are". Anything the heuristics get wrong should set `phrasing`.

Policies emit one scenario each into `<context>/policies.feature`, tagged
`@policy:<slug>`: `Given condition` (if any), `When` the trigger event's Then text,
`Then` the resulting command's When text. A policy's context is its own, else the
resulting command's aggregate's, else the trigger event's aggregate's. A policy
missing either end warns.

Open hot spots are written as `# Open question (id): …` comments at the top of every
feature file in their context. context-map.json also lists actors, and per context
the policies, read models (with readers) and hot spots (open and resolved).
Scenarios from facilitator-sourced commands, events or policies are tagged `@proposed`.

Events with a `Condition` are excluded from the happy-path scenario and each gets its
own scenario: preconditions as `Given`, the condition as `And` (or `Given` when there
are no preconditions), then the command and that single event.

Commands with no linked events produce a warning and are omitted.
A command with exactly one conditional event and no unconditional one warns (likely a
gap); two or more conditional events with none unconditional is a branch and doesn't.
Tags are `@context:<slug>` and `@aggregate:<slug>` — Gherkin tags can't contain
whitespace. `emit_test.go` runs every emitted file through the cucumber Gherkin parser.
Deprecated aggregates are tagged `@deprecated` and emit no scenarios.
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
