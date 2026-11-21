# event-storming-mcp

An MCP server that facilitates [event storming](https://www.eventstorming.com/)
workshops. Connect it to an LLM like Claude and it acts as a structured facilitator:
capturing domain events, commands, actors, and policies; clustering them into
aggregates and bounded contexts; and emitting Gherkin feature files + a glossary.

The LLM drives the conversation. The server owns the grammar and durable state.

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
git clone https://github.com/forgant-foundry/event-storming-mcp
cd event-storming-mcp
go install .
# binary at $(go env GOPATH)/bin/event-storming-mcp — make sure that directory is on your PATH
```

Or `make build` to produce `./bin/event-storming-mcp` without installing.

## Setup

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "event-storming-mcp": {
      "command": "/path/to/event-storming-mcp",
      "args": ["serve"]
    }
  }
}
```

Restart Claude Desktop. The 35 tools will appear automatically.

### Claude Code

Claude Code reads MCP servers from `.mcp.json` (project-scoped, commit it to share)
or `~/.claude.json` (user-scoped). It does not read them from `.claude/settings.json`.

To enable the server for one project, add it to that project's `.mcp.json`:

```json
{
  "mcpServers": {
    "event-storming-mcp": {
      "type": "stdio",
      "command": "event-storming-mcp",
      "args": ["serve"]
    }
  }
}
```

Using the bare command name keeps the file portable; it requires the binary to be on
the `PATH` of the shell that launches Claude Code.

Or add it from the Claude Code CLI:

```sh
claude mcp add --scope project event-storming-mcp -- event-storming-mcp serve   # writes .mcp.json
claude mcp add --scope user    event-storming-mcp -- event-storming-mcp serve   # all projects
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

## Tools (35)

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

### Playbooks
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

## Session Storage

Sessions are stored as JSON files at:

```
$XDG_DATA_HOME/event-storming-mcp/<session-id>/session.json
```

`XDG_DATA_HOME` defaults to `~/.local/share` on Linux/macOS.

## License

MIT — see [LICENSE](LICENSE).
