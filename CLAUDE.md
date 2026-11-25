# forgant-discovery — Architecture

One binary, `forgant-discovery`, with two MCP servers that share a session:
`serve events` facilitates event storming, and `serve stories` facilitates story
mapping linked to the storm. The LLM drives the conversation; the servers own the
grammar (event storming stencil, story map, Gherkin emission) and durable state.
The story-mapping design, with its sources and phases, is `docs/story-mapping.md`.

The repo, Go module (`github.com/forgant-foundry/forgant-discovery`) and executable
are all `forgant-discovery`; the project was `event-storming-mcp` until 2026-11.
Two old names remain on purpose: the session data directory (below) and the
`event-storming-mcp` key existing `.mcp.json` files use for the events server.

## Package Layout

```
cmd/forgant-discovery/
  main.go       entry point; releases build this with GoReleaser

internal/
  cli/
    root.go     cobra root command
    serve.go    "serve events" and "serve stories" — each an MCP server over stdio

  session/
    types.go    All storm types: Event, Command, Actor, Policy, ReadModel,
                ExternalSystem, HotSpot, Aggregate, BoundedContext, GlossaryTerm, Session
    storymap.go Story-map types: StoryMap, Frame, Role, Activity, Step, Story,
                Screen, Offer, Slice (Session.StoryMap)
    storymap_store.go  SetFrame, Add*/Update* for each story-map type,
                AssignToSlice, MoveItem; story-map removal and storm cascades
    storymap_check.go  CheckLinks — the story map against the storm, both ways
    lock_unix.go / lock_other.go  flock around Save (no-op where unavailable)
    store.go    JSON-on-disk persistence ($XDG_DATA_HOME/event-storming-mcp/<id>/session.json)
                Mutation functions: New, Load, Save, SetSyncPath, ExportSession,
                ImportSession, AddEvent, AddCommand, AddActor, SetEventCondition,
                ResolveHotSpot, DeprecateAggregate, LinkEvent, RemoveEvent, AddPolicy,
                AddReadModel, AddHotSpot, ClusterAggregate, ProposeContext, DefineTerm;
                reconcile (two-sided membership repair, run by Load and Save)
    edit.go     Update*/patch structs, AssignToAggregate, AssignToContext, RemoveItem
                (cascading delete), MergeEvents, ReopenHotSpot, AliasWarnings,
                reference/source validation helpers
    lint.go     LintStep — checks step text against glossary (alias violations,
                cross-context leakage); a match inside a longer occurrence of
                another term's name or canonical phrasing doesn't count

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
      map-the-users-story.md     (stories server)

  mcp/
    server.go   New() — the events server; addSharedTools() — sessions, glossary,
                hot spots, remove_item, portability and per-server playbooks,
                registered by both servers; version from Go build info
    edit.go     update_*, assign_to_*, merge_events, remove_item, reopen_hotspot;
                optString/optStrings helpers that tell an omitted argument from
                an empty one
    stories.go  NewStories() — the stories server and its tools
```

Each playbook names its server; `list_playbooks` and `get_playbook` serve only the
server's own.

## Session Storage

Sessions are stored as JSON files at:

```
$XDG_DATA_HOME/event-storming-mcp/<session-id>/session.json
```

`XDG_DATA_HOME` defaults to `~/.local/share` when not set.

The file contains the entire session graph in a single document. There is no database.

If `Session.SyncPath` is set (via `start_session(sync_path)` or `set_sync_path`), every
`Save` also writes the same JSON to that project-local path.

The data directory keeps the name `event-storming-mcp` so existing sessions stay
where they are. Both servers write the same file, so `Save` is revision-checked:
it takes an exclusive lock on `session.json.lock`, refuses (`ErrConflict`) if the
file's `revision` has moved on since this copy was loaded, then writes the next
revision. A refused save surfaces as a tool error to retry.

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
| `role_` | Role |
| `actv_` | Activity |
| `step_` | Step |
| `sty_` | Story |
| `scr_` | Screen |
| `slc_` | Slice |

## Events server tools (35)

`forgant-discovery serve events`. The tools marked *shared* are also on the stories
server.

### Session management
- `start_session(name, description, sync_path?)` → session_id *(shared)*
- `resume_session(session_id)` → full Session *(shared)*

### Sticky note capture
- `add_event(session_id, name, notes?, trigger_command_id?, aggregate_id?, phrasing?, source?, allow_duplicate?)`
- `add_command(session_id, name, notes?, actor_id?, aggregate_id?, preconditions?, phrasing?, source?, allow_duplicate?)`

Both refuse a name that already exists (case-insensitive) unless `allow_duplicate`;
the error names the existing note and points at link/update.

`phrasing` overrides the derived step text verbatim (for a command it's the whole
When step including the actor, and the scenario title).
- `add_actor(session_id, name, type)` — type: person | role | external_system
- `add_policy(session_id, name, trigger_event_id? | trigger_time?, resulting_command_id?, context_id?, condition?, notes?, source?)`
  — `trigger_time` ("a calendar month ends") replaces an event trigger; not both
- `add_read_model(session_id, name, actor_ids?, context_id?, notes?, source?)` — `actor_id` still accepted

`source` is expert | facilitator | imported. Facilitator-sourced commands/events emit
their scenarios tagged `@facilitator` (provenance — the expert confirmed them; it
doesn't mean unconfirmed).
- `add_hot_spot(session_id, question, context_id?)` *(shared)*

### Refinement
- `link_event(session_id, event_id, command_id)` — set trigger_command_id in place (no duplicate)
- `remove_event(session_id, event_id)` — same as `remove_item(kind=event)`
- `set_event_condition(session_id, event_id, condition, negates?)` — conditional/rejection
  event; `negates` is the trigger command's precondition it contradicts, word for word
- `resolve_hotspot(session_id, hotspot_id, resolution)` *(shared)*
- `reopen_hotspot(session_id, hotspot_id, reason)` — moves the resolution into `history` *(shared)*
- `deprecate_aggregate(session_id, aggregate_id, reason?)`

### Editing
Update tools change only the fields passed; an empty string clears a field, and an
array replaces the whole list.
- `update_event(session_id, event_id, name?, notes?, phrasing?, source?)`
- `update_command(session_id, command_id, name?, notes?, actor_id?, preconditions?, phrasing?, source?)`
- `update_actor(session_id, actor_id, name?, type?, notes?)`
- `update_policy(session_id, policy_id, name?, trigger_event_id?, trigger_time?, resulting_command_id?, context_id?, condition?, notes?, source?)`
- `update_read_model(session_id, read_model_id, name?, actor_ids?, context_id?, notes?, source?)`
- `update_aggregate(session_id, aggregate_id, name?, notes?, preconditions?)`
- `update_bounded_context(session_id, context_id, name?, description?, preconditions?)`
  — aggregate/context preconditions are emitted as `Background`
- `assign_to_aggregate(session_id, aggregate_id, event_ids?, command_ids?, move?)`
- `assign_to_context(session_id, context_id, aggregate_ids?, glossary_term_ids?, move?)`
- `remove_item(session_id, kind, id)` → { removed, kind, detached } — cascades: clears *(shared)*
  every reference to the removed record and reports each one
- `merge_events(session_id, keep_event_id, remove_event_id)` → { event, removed, report }
  — moves policies, fills keep's empty fields from remove, concatenates notes, reports
  conflicts (keep wins)

### Glossary
- `define_term(session_id, term, definition, canonical_phrasing?, aliases?, context_id?, shared_with?)` *(shared)*
  → { term, alias_warnings } — warns about aliases that are Gherkin keywords, sit
  inside another term's canonical phrasing, or appear in definitions or notes
  — `shared_with` lists contexts that use the term on purpose; lint_steps won't flag it there
- `lint_steps(session_id, step_text, context_id?)` → { clean, issues } *(shared)*

### Clustering
- `cluster_aggregate(session_id, name, event_ids?, command_ids?, context_id?)` — enforces one-aggregate-per-event
- `propose_bounded_context(session_id, name, description?, aggregate_ids?, glossary_term_ids?)` — enforces one-context-per-aggregate

### Emission
- `emit_gherkin(session_id, output_dir, only?, include_deprecated?)` → Summary (feature
  files, glossary.md, context-map.json, scenario_count, open_hot_spots, warnings, lint);
  `only` is any of features | glossary | context_map

### Portability
- `export_session(session_id, path)` — write session JSON verbatim *(shared)*
- `import_session(path, sync_path?)` → new session_id (original ID is replaced to avoid *(shared)*
  collisions); `sync_path` overrides the file's own before the first save
- `set_sync_path(session_id, path)` — empty path disables syncing *(shared)*

### Playbooks
- `list_playbooks()` → [{ slug, title, description }] *(shared)*
- `get_playbook(slug)` → markdown facilitation script *(shared)*

## Stories server tools (29)

`forgant-discovery serve stories`: the 13 shared tools (`start_session`,
`resume_session`, `define_term`, `lint_steps`, `add_hot_spot`, `resolve_hotspot`,
`reopen_hotspot`, `remove_item`, `export_session`, `import_session`,
`set_sync_path`, `list_playbooks`, `get_playbook`) plus:

- `frame_story_map(session_id, for_whom?, problems?, today?, solution_ideas?, why?, success_measures?)`
- `add_role(session_id, actor_id? | actor_name?, actor_type?, goal?, frequency?, expertise?, persona?, source?)`
  — a role is a storm actor; `actor_name` creates it; one role per actor
- `update_role(session_id, role_id, goal?, frequency?, expertise?, persona?, source?)`
- `add_activity(session_id, name, role_id?, notes?, before_id?, source?)` / `update_activity`
- `add_step(session_id, activity_id, role_id, name, command_ids?, read_model_ids?, screen_id?, after_step_id?, notes?, before_id?, source?)` / `update_step`
  — `after_step_id` is a hand-off: another role's step this one waits for
- `add_story(session_id, step_id, title, why?, kind?, slice_id?, event_id?, notes?, before_id?, source?)` / `update_story`
  — kind: detail (default) | alternative | failure | misuse | spike
- `add_screen(session_id, name, shows?, offers?: [{command_id, shows_after?}], notes?, source?)` / `update_screen`
- `add_slice(session_id, name, kind, outcome?, walking_skeleton?, notes?, before_id?)` / `update_slice`
  — kind: release | learning | development; at most one walking skeleton, a development slice
- `assign_to_slice(session_id, slice_id, story_ids)`
- `move_item(session_id, kind, id, before_id?)` — activity | step | story | slice
- `check_links(session_id)` → { clean, counts, findings }

List order is meaningful: activities and steps are narrative order (steps within
their activity), stories are rank order within their step, slices are top to
bottom. `remove_item` takes the story-map kinds too: removing an activity with
steps or a step with stories is refused; other references are cleared. Removing
a storm actor, command, read model or event detaches it from roles, steps,
screens and stories.

`CheckLinks` warns about a step with no command or read model, a step with no
role, a screen offering a command with no linked events, and a step using a
command its screen doesn't offer. It notes commands no step uses and no policy
issues (once the map has steps), read models no screen shows (once there are
screens), and roles with no steps. It lints activity, step, story and screen text
against the glossary.

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
Scenarios from facilitator-sourced commands, events or policies are tagged `@facilitator`.

Context and aggregate preconditions are emitted once as `Background:` (context first,
deduplicated), and a command's own copy of one is dropped from its scenarios. They
can't be `negates` targets. Time-triggered policies emit `When <trigger_time>`.
Events or commands sharing a name (case-insensitive) warn, pointing at `merge_events`.

Preconditions are written as one `Given` then `And`s, with the first word lower-cased
unless it's an acronym. A conditional event's `Negates` (set via
`set_event_condition(negates)`) names the precondition its condition contradicts;
that precondition is dropped from its scenario, and a stale `Negates` warns.

Derived noun phrases that exactly match a glossary term (one in the feature's context,
shared with it, or context-free) use the term's canonical phrasing — see `Phraser` in
style.go. Every emitted step is run through `LintStep`; violations go in
`Summary.Lint`, not `Warnings`.

Also warned: an event whose aggregate differs from its trigger command's aggregate, and
a policy name mentioning a PascalCase name that's no longer an event or command.
Deprecated aggregates are skipped unless `Options.IncludeDeprecated`; context-map.json
always lists them with `deprecated` and the reason.

Events with a `Condition` are excluded from the happy-path scenario and each gets its
own scenario: preconditions as `Given`, the condition as `And` (or `Given` when there
are no preconditions), then the command and that single event.

Commands with no linked events produce a warning and are omitted.
A command with exactly one conditional event and no unconditional one warns (likely a
gap); two or more conditional events with none unconditional is a branch and doesn't.
Tags are `@context:<slug>` and `@aggregate:<slug>` — Gherkin tags can't contain
whitespace. `emit_test.go` runs every emitted file through the cucumber Gherkin parser.
With `include_deprecated`, deprecated aggregates are tagged `@deprecated` and emit no
scenarios.
Aggregates with no commands produce a stub scenario with `# TODO` comments.

## Adding a New Tool

1. Add a mutation function to `internal/session/store.go` (storm) or
   `storymap_store.go` (story map), or a query-only function if no state changes
2. Add a tool definition function and handler to `internal/mcp/` (`server.go` /
   `edit.go` for the storm, `stories.go` for the story map)
3. Register it in `New()`, `NewStories()`, or `addSharedTools()` for both
4. Add tests in the relevant `_test.go` file

## Running Locally

```sh
make build
./bin/forgant-discovery serve events    # event storming over stdio
./bin/forgant-discovery serve stories   # story mapping over stdio
make install                            # go install ./cmd/forgant-discovery
```

## Tests

```sh
make test
```

Tests use `t.Setenv("XDG_DATA_HOME", t.TempDir())` to isolate session storage.

## CI and Releases

Plain Go and GitHub, hand-maintained:

- `.github/workflows/ci.yml` runs on pushes to main and on pull requests: gofmt,
  `go mod tidy -diff`, `go vet`, `go test -race`, and govulncheck. It uses the
  latest stable Go; `go.mod`'s `go` line is only the minimum.
- `.github/workflows/release.yml` runs on a pushed `v*` tag and publishes
  binaries for linux, darwin and windows (amd64 and arm64) with GoReleaser
  (`.goreleaser.yaml`). Release by tagging: `git tag v0.7.0 && git push origin v0.7.0`.
- The version the server reports comes from Go's build info, which Go stamps from
  the tag when building a clean tagged checkout; `go install …@v0.7.0` gets it too.
- `.github/dependabot.yml` updates Go modules and Actions weekly.
