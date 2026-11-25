# forgant-discovery

Two MCP servers for product discovery, in one binary, sharing one session:

- **`forgant-discovery serve events`** facilitates [event storming](https://www.eventstorming.com/):
  capturing domain events, commands, actors and policies, clustering them into
  aggregates and bounded contexts, and emitting Gherkin feature files and a glossary.
- **`forgant-discovery serve stories`** facilitates story mapping (Patton): roles,
  the steps they take in narrative order, the screens those steps happen on, and
  slices that cut the map into releases. Steps and screens link to the storm's
  commands and read models, and `check_links` checks the link both ways.

Connect them to an LLM like Claude and it acts as a structured facilitator. The LLM
drives the conversation; the servers own the grammar and durable state. The story
mapping design is in [`docs/story-mapping.md`](docs/story-mapping.md).

## Output

Each session produces:

| File | Contents |
|---|---|
| `{context}/{aggregate}.feature` | Gherkin scenarios runnable by Cucumber / Behave / godog; open hot spots appear as comments |
| `{context}/policies.feature` | One scenario per policy: when the trigger event happens, then the command is issued |
| `glossary.md` | Ubiquitous language reference |
| `context-map.json` | Machine-readable structure: bounded contexts, aggregates, policies, read models, actors, hot spots |

`emit_gherkin` takes an optional `only` (`features`, `glossary`, `context_map`), so you
can refresh the glossary and context map without overwriting hand-edited feature files.

## Prerequisites

- Go 1.25+

## Build

```sh
go install github.com/forgant-foundry/forgant-discovery/cmd/forgant-discovery@latest
# or, from a checkout:
make install
# binary at $(go env GOPATH)/bin/forgant-discovery — make sure that directory is on your PATH
```

Or `make build` to produce `./bin/forgant-discovery` without installing.

Release binaries for linux, macOS and Windows are attached to each
[GitHub release](https://github.com/forgant-foundry/forgant-discovery/releases).
Releases are cut by pushing a `v*` tag; CI runs gofmt, vet, race-enabled tests and
govulncheck on every push and pull request.

## Setup

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "event-storming": {
      "command": "/path/to/forgant-discovery",
      "args": ["serve", "events"]
    },
    "story-mapping": {
      "command": "/path/to/forgant-discovery",
      "args": ["serve", "stories"]
    }
  }
}
```

Restart Claude Desktop. Enable either server or both.

### Claude Code

Claude Code reads MCP servers from `.mcp.json` (project-scoped, commit it to share)
or `~/.claude.json` (user-scoped). It does not read them from `.claude/settings.json`.

To enable the server for one project, add it to that project's `.mcp.json`:

```json
{
  "mcpServers": {
    "event-storming-mcp": {
      "type": "stdio",
      "command": "forgant-discovery",
      "args": ["serve", "events"]
    },
    "story-mapping": {
      "type": "stdio",
      "command": "forgant-discovery",
      "args": ["serve", "stories"]
    }
  }
}
```

The key `event-storming-mcp` predates the rename; keeping it for the events server
keeps its tool names (`mcp__event-storming-mcp__…`) unchanged for anything that
refers to them. A new project can name it anything.

Using the bare command name keeps the file portable; it requires the binary to be on
the `PATH` of the shell that launches Claude Code.

Or add it from the Claude Code CLI:

```sh
claude mcp add --scope project event-storming-mcp -- forgant-discovery serve events   # writes .mcp.json
claude mcp add --scope project story-mapping      -- forgant-discovery serve stories
claude mcp add --scope user    event-storming-mcp -- forgant-discovery serve events   # all projects
```

Claude Code asks you to approve project-scoped servers the first time it starts in
that project. Run `/mcp` to confirm the server is connected.

## Quick Start

Once connected, tell Claude:

> "Get the run-event-storming playbook and let's run an event storming session on
> [your domain]."

Claude will call `get_playbook("run-event-storming")` and follow the facilitation
script, calling tools as it goes. At the end, call `emit_gherkin` to write the
output files.

For story mapping, with the stories server connected:

> "Get the map-the-users-story playbook and let's map how people will use [your
> product]. Our storm session is ses_…"

## Events server tools (35)

### Session
| Tool | Description |
|---|---|
| `start_session` | Begin a new session; returns session ID |
| `resume_session` | Reattach to an existing session; returns full model |

### Capture
| Tool | Description |
|---|---|
| `add_event` | Record a domain event (past tense) |
| `add_command` | Record a command (imperative) |
| `add_actor` | Record an actor (person / role / external system) |
| `add_policy` | Record a reactive policy: "whenever X, then Y" |
| `add_read_model` | Record a view an actor consults before acting |
| `add_hot_spot` | Flag an unresolved question for follow-up |

### Refinement
| Tool | Description |
|---|---|
| `link_event` | Set the triggering command on an existing event (avoids duplicates) |
| `remove_event` | Delete an event and detach it from its aggregate and policies |
| `set_event_condition` | Mark an event as conditional (e.g. a rejection); emitted as its own scenario. `negates` names the precondition it contradicts, which that scenario then leaves out |
| `resolve_hotspot` | Close a hot spot with the decision that resolved it |
| `reopen_hotspot` | Reopen a resolved hot spot when a decision is reversed; keeps the old resolution as history |
| `deprecate_aggregate` | Mark an aggregate obsolete; its feature file is tagged `@deprecated` with no scenarios |

### Editing
| Tool | Description |
|---|---|
| `update_event`, `update_command`, `update_actor`, `update_policy`, `update_read_model`, `update_aggregate`, `update_bounded_context` | Change only the fields passed; an empty string clears a field |
| `assign_to_aggregate` | Add events and commands to an existing aggregate (`move` to take them from another) |
| `assign_to_context` | Add aggregates and glossary terms to an existing bounded context |
| `remove_item` | Delete any record; every reference to it is cleared and reported |
| `merge_events` | Fold a duplicate event into the one to keep, moving its links and notes |

Events and commands take an optional `phrasing` used verbatim in the emitted steps.
Capture and update tools take an optional `source` — `expert`, `facilitator` or
`imported` — and scenarios built from facilitator-sourced notes are tagged `@facilitator`.
Policies take an optional `condition`, and a `trigger_time` instead of an event when
time passing triggers them; read models take several readers in `actor_ids`.
Aggregates and contexts take `preconditions`, emitted once as a Gherkin `Background`.
`add_event` and `add_command` refuse a name that already exists unless
`allow_duplicate` is passed.

### Glossary
| Tool | Description |
|---|---|
| `define_term` | Add or update a glossary entry (upsert by term name); `shared_with` marks contexts that use it on purpose |
| `lint_steps` | Check step text for alias violations and cross-context leakage |

### Clustering
| Tool | Description |
|---|---|
| `cluster_aggregate` | Group events + commands into an aggregate |
| `propose_bounded_context` | Group aggregates + glossary terms into a bounded context |

### Emission
| Tool | Description |
|---|---|
| `emit_gherkin` | Write .feature files, glossary.md, and context-map.json; `only` limits which. Reports warnings and glossary lint for every emitted step; skips deprecated aggregates unless `include_deprecated` |

### Portability
| Tool | Description |
|---|---|
| `export_session` | Write session to a JSON file for version control or sharing |
| `import_session` | Load a session from an exported file; returns a new local session ID. Pass `sync_path` to replace the file's own |
| `set_sync_path` | Configure a project-local file to stay in sync on every save |

### Stories server tools (34)

The stories server shares the session, glossary, hot spot, removal, portability and
playbook tools with the events server, and adds:

| Tool | Description |
|---|---|
| `frame_story_map` | Record the opportunity: who it's for, problems, how they cope today, why, success measures |
| `add_role` / `update_role` | A role that uses the product; it is a storm actor (created if needed) |
| `add_activity` / `update_activity` | An activity on the map's backbone |
| `add_step` / `update_step` | One thing a role does, linked to storm commands and read models, on a screen; `after_step_id` marks a hand-off between roles |
| `add_story` / `update_story` | A detail, alternative, failure, misuse or spike under a step, with its why |
| `add_screen` / `update_screen` | What a screen shows (read models) and offers (commands) |
| `add_slice` / `update_slice` / `assign_to_slice` | Release, learning and development slices, including the walking skeleton |
| `move_item` | Reorder activities, steps, stories and slices |
| `check_links` | Check the map against the storm both ways, and lint its wording against the glossary |
| `add_criterion` | Example mapping: a story's rule with a concrete Given/When/Then, optionally with example rows |
| `add_constraint` / `update_constraint` | A testable cross-functional requirement with what's measured and its target |
| `check_readiness` | Per slice, what blocks it going to a factory |
| `emit_story_map` | Journeys, story criteria, screen scenarios, constraints, `slices.json` and `story-map.md` |

## Playbooks
| Tool | Description |
|---|---|
| `list_playbooks` | List available facilitation scripts |
| `get_playbook` | Return a facilitation script as markdown |

## Playbooks

| Slug | When to use |
|---|---|
| `run-event-storming` | Starting a new session from scratch; includes a propose-and-confirm mode for experts who ask for suggestions |
| `extract-ubiquitous-language` | Focused glossary pass after event discovery |
| `emit-gherkin-specs` | Generating and reviewing the outputs, with one batched phrasing pass |
| `map-the-users-story` | (stories server) Story mapping linked to the storm: frame, roles, steps, screens, slices, link check |

## Session Storage

Sessions are stored as JSON files at:

```
$XDG_DATA_HOME/event-storming-mcp/<session-id>/session.json
```

`XDG_DATA_HOME` defaults to `~/.local/share` on Linux/macOS.

## License

MIT — see [LICENSE](LICENSE).
