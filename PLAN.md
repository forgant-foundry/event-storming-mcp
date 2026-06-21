# event-storming-mcp: Implementation Plan

## What this is and why it makes a good MCP tool

Event storming is a workshop technique for discovering a domain by surfacing
**domain events** (orange sticky notes: past-tense facts), then layering on
**commands** (blue), **actors** (yellow), **aggregates** (large yellow), **policies**
(purple), **read models** (green), and **bounded contexts** (groupings). The output
is a shared **ubiquitous language** and a model that can be turned into executable
behavioral specifications.

An MCP server is a natural home for this because the work is inherently iterative
and conversational: the LLM interviews the domain expert, classifies sticky notes,
clusters aggregates, and proposes bounded contexts — but it needs *durable state*
across turns and a *grammar* it can't accidentally violate. The server owns the
grammar (event-storming stencil + Gherkin emission); the LLM drives the conversation.

**Output triple per session:**
- `*.feature` — Gherkin scenarios (one feature per bounded context or aggregate)
- `glossary.md` — every glossary term with definition + canonical phrasing
- `context-map.json` — bounded contexts, aggregates, and their relationships

---

## Non-goals

- **Not a BDD runner.** We emit `.feature` files; Cucumber / Behave / godog / etc.
  execute them. We don't bind steps to code.
- **Not a generic DSL designer.** Gherkin is the only emit target in v1. The "DSL"
  the user contributes to *is* declarative Gherkin constrained by their glossary.
- **Not a property-based or example-table fuzzer.** Scenario Outlines are allowed
  where helpful but invariants ("balance is never negative") are out of scope for v1.

---

## Why Gherkin-only (v1)

Gherkin maps cleanly onto event storming:

| Gherkin clause | Event storming concept |
|---|---|
| `Given` | preconditions / prior events / standing state |
| `When` | command (issued by an actor) |
| `Then` | domain event(s) emitted by the receiving aggregate |
| `And` (in `Then`) | additional events from the same command |
| Tag `@context:X` | bounded context membership |
| Tag `@aggregate:Y` | receiving aggregate |
| Tag `@policy:Z` | scenario was generated from a reactive policy |
| Background | standing actors / world state for the feature |

Tradeoff accepted: Gherkin has no native notion of aggregate / bounded context /
policy. We encode them as **tag conventions** and keep the structural model in
`context-map.json`. The `.feature` files remain runnable by any BDD tool.

**Step style is enforced declarative**, not imperative. "When the customer cancels
their subscription" — not "When I click the cancel button." This is what makes the
ubiquitous language show up in the steps. The server lints emitted steps against
the glossary and rejects undefined terms.

---

## Architecture

```
event-storming-mcp/
├── cmd/
│   ├── root.go         # cobra root
│   └── serve.go        # MCP server (stdio + http)
│
├── internal/
│   ├── session/        # session state model + on-disk persistence
│   │   ├── types.go    # Event, Command, Actor, Aggregate, Policy, ReadModel,
│   │   │               # ExternalSystem, BoundedContext, GlossaryTerm, HotSpot
│   │   ├── store.go    # JSON-on-disk per session ($XDG_DATA_HOME/event-storming-mcp/)
│   │   └── lint.go     # glossary conformance check for emitted step text
│   │
│   ├── gherkin/        # emission: model -> .feature files
│   │   ├── emit.go     # scenarios per aggregate, tagged with context/aggregate
│   │   └── style.go    # declarative-style helpers, Background factoring
│   │
│   ├── playbooks/      # embedded markdown facilitation scripts
│   │   ├── playbooks.go
│   │   └── data/
│   │       ├── run-event-storming.md
│   │       ├── extract-ubiquitous-language.md
│   │       └── emit-gherkin-specs.md
│   │
│   └── mcp/server.go   # all tool handlers
│
├── main.go
├── go.mod              # github.com/forgant-foundry/event-storming-mcp
├── Makefile
├── CLAUDE.md           # written after Phase 2
└── PLAN.md
```

**Deliberately no rel_store or vec_store.** Session state fits in JSON; we don't
need search across sessions in v1. Add later if multi-session glossary reuse
becomes a real workflow.

---

## MCP Tools

| Tool | Description |
|---|---|
| `start_session` | Begin a new session with a name and short domain description; returns session id |
| `resume_session` | Reattach to a session by id; returns full current model |
| `add_event` | Record a domain event (past tense); optionally attach trigger command + aggregate |
| `add_command` | Record a command (imperative); optionally attach issuing actor + target aggregate |
| `add_actor` | Record an actor (person / role / external system) |
| `add_policy` | Record a reactive policy: "whenever event X, then command Y" |
| `add_read_model` | Record a view an actor consults to decide on a command |
| `add_hot_spot` | Flag an unresolved question / disagreement for follow-up |
| `define_term` | Add or update a glossary entry (term, definition, canonical phrasing, aliases) |
| `cluster_aggregate` | Group events + commands into an aggregate (consistency boundary) |
| `propose_bounded_context` | Group aggregates + glossary terms into a bounded context |
| `emit_gherkin` | Generate the output triple (`.feature` files + `glossary.md` + `context-map.json`) |
| `lint_steps` | Check a candidate step text against the current glossary; returns undefined terms |
| `list_playbooks` | All playbooks with titles + descriptions |
| `get_playbook` | Return a playbook (facilitation script) as markdown by slug |

---

## Session state schemas (sketch)

```json
// Event
{ "id": "evt_...", "name": "OrderPlaced", "trigger_command_id": "cmd_...",
  "aggregate_id": "agg_...", "context_id": "ctx_...", "notes": "..." }

// Command
{ "id": "cmd_...", "name": "PlaceOrder", "actor_id": "act_...",
  "aggregate_id": "agg_...", "preconditions": ["..."] }

// Aggregate
{ "id": "agg_...", "name": "Order", "context_id": "ctx_...",
  "command_ids": ["cmd_..."], "event_ids": ["evt_..."] }

// BoundedContext
{ "id": "ctx_...", "name": "Ordering", "description": "...",
  "aggregate_ids": ["agg_..."], "glossary_term_ids": ["term_..."] }

// GlossaryTerm
{ "id": "term_...", "term": "Order", "definition": "...",
  "canonical_phrasing": "the order", "aliases": ["cart", "basket"],
  "context_id": "ctx_..." }
```

Identifiers are stable across edits so the LLM can refer back to prior records.

---

## Initial playbooks

- **`run-event-storming`** — the full three-phase facilitation:
  1. *Big Picture*: chaotic event discovery (call `add_event` rapidly, defer ordering)
  2. *Process Modeling*: add commands, actors, policies, read models; order events on timeline
  3. *Software Design*: cluster into aggregates, propose bounded contexts, name the language
- **`extract-ubiquitous-language`** — focused glossary-building pass over an existing session;
  prompts for definitions, surfaces synonyms, asks the expert to pick canonical phrasing
- **`emit-gherkin-specs`** — runs `emit_gherkin`, then walks the user through reviewing each
  feature file, factoring Backgrounds, and resolving any glossary lint failures

Each playbook is a markdown script the host LLM follows, identical in shape to
dora-ref-mcp's playbooks (`get_playbook(slug)` returns the markdown).

---

## Implementation Phases

### Phase 1 — Session state + minimal server

- `internal/session/types.go` — all sticky-note types + GlossaryTerm + BoundedContext
- `internal/session/store.go` — JSON files under `$XDG_DATA_HOME/event-storming-mcp/<id>/`
- `cmd/serve.go` + `internal/mcp/server.go` — wire `start_session`, `resume_session`,
  `add_event`, `add_command`, `add_actor`, `define_term`
- Smoke test: a script that drives a tiny session via MCP and inspects the JSON

### Phase 2 — Clustering + bounded contexts

- `add_policy`, `add_read_model`, `add_hot_spot`
- `cluster_aggregate`, `propose_bounded_context`
- Validation: an event must belong to exactly one aggregate; an aggregate to one context

### Phase 3 — Gherkin emission + glossary lint

- `internal/gherkin/emit.go` — one Feature per aggregate, tagged
  `@context:X @aggregate:Y`; scenarios from (preceding-events × command → emitted-events)
- `internal/session/lint.go` — tokenize step text, flag any noun/verb not in the glossary
  for the scenario's bounded context
- `emit_gherkin` and `lint_steps` tools

### Phase 4 — Playbooks + docs

- Write `run-event-storming.md`, `extract-ubiquitous-language.md`, `emit-gherkin-specs.md`
- `list_playbooks` / `get_playbook` tools
- CLAUDE.md (architecture as built), README.md (usage), Makefile

---

## Open Design Questions

1. **Lint external `.feature` files?** The server already has a glossary linter. Exposing
   it on user-supplied Gherkin (not just emitted output) lets teams keep existing scenarios
   conformant with the ubiquitous language. Recommended: yes, low marginal cost.

2. **One Feature per aggregate vs. per bounded context?** Per-aggregate gives smaller,
   focused files but multiplies tag noise. Per-context gives a wider lens but feature files
   get long. Recommended: per-aggregate, with `context-map.json` as the index.

3. **Scenario factoring (Background, Scenario Outline).** The LLM tends to repeat Givens.
   Should the server auto-factor common Givens into a Background section, or just lint and
   let the LLM rewrite? Recommended: lint-and-suggest in v1, auto-factor in v2.

4. **Property-style invariants as a second emit target.** Some behaviors are better
   expressed as invariants ("balance is never negative") than scenarios. Defer to v2 —
   would likely emit as a sidecar `invariants.md` with `@invariant:` tags pointing back
   from scenarios.

5. **Multi-session glossary reuse.** A team running event storming on multiple subdomains
   wants shared terms (`Customer`, `Order`) to stay consistent across sessions. Defer:
   pulls in a real store (chromem/sqlite). v1 is single-session.

6. **Session export format for human review.** JSON is the source of truth, but a
   rendered `session.md` (events timeline, aggregates, glossary) is what a domain expert
   will actually read. Recommended: include as part of `emit_gherkin` output.
