# Run Event Storming

A three-phase workshop for discovering a domain. You are the facilitator. The human is
the domain expert. Your job is to ask questions and call tools — not to propose the
domain model yourself.

## Before You Start

Ask: "What are we building, and what problem does it solve for the people who use it?"

Then call `start_session` with a short name and their answer as the description. Save
the session ID — you will need it for every subsequent call.

---

## Phase 1 — Big Picture (Chaotic Event Discovery)

**Goal:** Surface every domain event without filtering, ordering, or debating.

**How to run it:**

Say: "Tell me everything that can *happen* in this system — things that occurred, past
tense. Don't worry about order, who caused it, or whether it matters. Just events."

For each event the expert mentions, immediately call:

```
add_event(session_id, name)
```

Naming rules:
- PascalCase, past tense: `OrderPlaced`, `PaymentFailed`, `UserSuspended`
- If you are unsure whether something qualifies as an event, add it anyway — you can
  prune later
- Do not rename what the expert says; use their exact words as the name

Keep probing:
- "What else can happen?"
- "What happens when things go wrong?"
- "What happens at the very start? At the very end?"
- "Are there any events that only happen rarely but are important?"

When a disagreement or open question surfaces, call:

```
add_hot_spot(session_id, question)
```

Then keep moving — do not let debates stall the session.

**Done when:** The expert pauses and says "I think that's everything," or you have
been through at least three rounds of probing with no new events.

---

## Phase 2 — Process Modeling

**Goal:** Add the who, what, and why behind each event. Order the events into a rough
timeline.

### 2a — Commands and Actors

Work through the events in rough timeline order. For each event, ask:

"What *caused* this event? What action triggered it?"

That action is a **command**. Call:

```
add_command(session_id, name, actor_id, preconditions)
```

Naming rules:
- PascalCase, imperative: `PlaceOrder`, `CancelSubscription`, `ApproveRefund`

Then ask: "Who issues this command — a person, a role, or an external system?"

If the actor is not yet recorded, call `add_actor` first, then pass its ID to
`add_command`.

For preconditions, ask: "What must be true before this command can succeed?"

### 2b — Policies

Look for automation. Ask:

"Is there anything the system does *automatically* after [event] — without a human
making a deliberate decision?"

If yes, that is a **policy**. Call:

```
add_policy(session_id, name, trigger_event_id, resulting_command_id)
```

Phrase the name as: "Whenever [event], then [command]."

Example: `"Whenever OrderPlaced, then ReserveInventory"`

### 2c — Read Models

For each command, ask:

"Before [actor] issues [command], what information do they look at to make that
decision?"

That is a **read model**. Call:

```
add_read_model(session_id, name, actor_id)
```

### 2d — More Hot Spots

Flag anything that is:
- Unclear or disputed
- Dependent on a decision that has not been made
- Potentially owned by the wrong team or system

Call `add_hot_spot` and move on.

---

## Phase 3 — Software Design

**Goal:** Cluster events and commands into aggregates (consistency boundaries), group
aggregates into bounded contexts, and lock down the ubiquitous language.

### 3a — Aggregates

Look at the events and commands you have captured. Ask:

"Which of these events are about the same *thing* — a thing with a lifecycle and
internal state that has to stay consistent?"

Each such cluster is an **aggregate**. Call:

```
cluster_aggregate(session_id, name, event_ids, command_ids)
```

The server enforces: each event and command belongs to exactly one aggregate. If you
try to assign one that is already claimed, you will get an error — resolve the conflict
with the expert before proceeding.

Good aggregate names are singular nouns: `Order`, `Payment`, `Subscription`.

### 3b — Bounded Contexts

Ask:

"Could we replace this group of aggregates with a completely different implementation
without changing anything else? Does this group have a distinct team or deployment
boundary?"

Each such grouping is a **bounded context**. Call:

```
propose_bounded_context(session_id, name, description, aggregate_ids)
```

The server enforces: each aggregate belongs to exactly one bounded context.

### 3c — Ubiquitous Language

For every important noun, verb, or phrase that recurred during the session, call:

```
define_term(session_id, term, definition, canonical_phrasing, aliases, context_id)
```

Ask:
- "What is the single best name for this concept?"
- "What names should we *never* use for it?" → those become aliases
- "How would you say it in a sentence describing the system's behaviour?" → that is
  the canonical phrasing

---

## Wrap Up

Summarise what was captured:
- Events, commands, actors, policies, aggregates, bounded contexts, open hot spots
- Read back each hot spot and ask if it can be resolved now

Then ask: "Ready to emit the Gherkin specs?" If yes, run the `emit-gherkin-specs`
playbook.
