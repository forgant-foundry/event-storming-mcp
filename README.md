# event-storming-mcp

An MCP server that facilitates [event storming](https://www.eventstorming.com/)
workshops. Connect it to an LLM like Claude and it acts as a structured facilitator:
capturing domain events, commands, actors, and policies; clustering them into
aggregates and bounded contexts; and emitting Gherkin feature files + a glossary.

The LLM drives the conversation. The server owns the grammar and durable state.

## Output

Each session produces three files:

| File | Contents |
|---|---|
| `{context}/{aggregate}.feature` | Gherkin scenarios runnable by Cucumber / Behave / godog |
| `glossary.md` | Ubiquitous language reference |
| `context-map.json` | Machine-readable bounded context structure |

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

Restart Claude Desktop. The 23 tools will appear automatically.

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

## Tools (23)

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
| `remove_event` | Delete an event and detach it from its aggregate |
| `set_event_condition` | Mark an event as conditional (e.g. a rejection); emitted as its own scenario |
| `resolve_hotspot` | Close a hot spot with the decision that resolved it |
| `deprecate_aggregate` | Mark an aggregate obsolete; its feature file is tagged `@deprecated` with no scenarios |

### Glossary
| Tool | Description |
|---|---|
| `define_term` | Add or update a glossary entry (upsert by term name) |
| `lint_steps` | Check step text for alias violations and cross-context leakage |

### Clustering
| Tool | Description |
|---|---|
| `cluster_aggregate` | Group events + commands into an aggregate |
| `propose_bounded_context` | Group aggregates + glossary terms into a bounded context |

### Emission
| Tool | Description |
|---|---|
| `emit_gherkin` | Write .feature files, glossary.md, and context-map.json |

### Portability
| Tool | Description |
|---|---|
| `export_session` | Write session to a JSON file for version control or sharing |
| `import_session` | Load a session from an exported file; returns a new local session ID |
| `set_sync_path` | Configure a project-local file to stay in sync on every save |

### Playbooks
| Tool | Description |
|---|---|
| `list_playbooks` | List available facilitation scripts |
| `get_playbook` | Return a facilitation script as markdown |

## Playbooks

| Slug | When to use |
|---|---|
| `run-event-storming` | Starting a new session from scratch |
| `extract-ubiquitous-language` | Focused glossary pass after event discovery |
| `emit-gherkin-specs` | Generating and reviewing the output triple |

## Session Storage

Sessions are stored as JSON files at:

```
$XDG_DATA_HOME/event-storming-mcp/<session-id>/session.json
```

`XDG_DATA_HOME` defaults to `~/.local/share` on Linux/macOS.

## License

MIT — see [LICENSE](LICENSE).
