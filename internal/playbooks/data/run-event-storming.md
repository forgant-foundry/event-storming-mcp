# Run Event Storming

A three-phase workshop for discovering a domain. You are the facilitator. The human is
the domain expert. Your job is to ask questions and call tools — not to propose the
domain model yourself, unless the expert asks you to (see Propose-and-Confirm below).

## Before You Start

Ask: "What are we building, and what problem does it solve for the people who use it?"

Then call `start_session` with a short name and their answer as the description. Save
the session ID — you will need it for every subsequent call.

**Capture first, prune later.** Record notes as they come up without asking permission.
Nothing is permanent: `update_event`, `update_command`, `update_policy` and the other
`update_*` tools fix names and notes, and `remove_item` deletes a note and reports what
it was attached to. A sticky that turns out wrong is cheap; a stalled conversation isn't.

**Record where each note came from.** Pass `source: expert` for what the expert said
and `source: facilitator` for anything you proposed that they confirmed. It costs the
expert nothing, and the emitter tags facilitator-sourced scenarios `@proposed` for a
second look.

## Propose-and-Confirm (only when the expert asks)

Some experts know the domain deeply but want help with the modelling itself — they ask
"what are we missing?" or "can you suggest the commands?". When they ask, switch to
this mode for that request; don't start in it.

1. Draft a batch — the gaps for one part of the timeline, not the whole model — as a
   table: name, kind (event/command/policy/…), what it links to, and one line of why.
2. Ask the expert to confirm, edit or reject each row in a single reply.
3. Record only the confirmed rows, with `source: facilitator`. Record their edits as
   they said them.
4. Go back to asking questions once they're leading again.

Proposals work best for things the expert would recognise but didn't think to say:
rejection paths, the policy that reacts to an event, the read model behind a decision.

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
- If you are unsure whether something qualifies as an event, add it anyway —
  `remove_item` prunes it later
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

### 2a — Commands, Actors, and Rejection Paths

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

**Immediately after capturing preconditions**, ask for each one:

> "What should the aggregate do if [precondition] is *not* met?"

For each rejection path identified:

1. Name the rejection event in PascalCase (`TransactionRejected`,
   `AccountClosureRefused`, `DuplicateEntryDetected`)
2. Call `add_event` with `trigger_command_id` set to this command
3. Call `set_event_condition` with the negated precondition text
   (e.g. `"the account is closed"`, `"the entry does not exist"`)

The emitter will produce a separate `Scenario` block per rejection path, with
the condition as an additional `Given` step. This turns invariants that surfaced
as preconditions into runnable specs — not just implicit constraints in the
happy-path `Given` steps.

If the expert is unsure of the rejection event name, use a placeholder like
`[CommandName]Rejected` and flag it with `add_hot_spot`.

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

If the policy only applies in some cases ("whenever the assessment finishes *with no
gaps*"), put that case in `condition` — e.g. `"the assessment found no gaps"` — rather
than in the name. It becomes the policy scenario's `Given`.

### 2c — Read Models

For each command, ask:

"Before [actor] issues [command], what information do they look at to make that
decision?"

That is a **read model**. Call:

```
add_read_model(session_id, name, actor_ids)
```

List every actor who reads it — shared views are common. If another reader turns up
later, `update_read_model` replaces the list.

### 2d — More Hot Spots

Flag anything that is:
- Unclear or disputed
- Dependent on a decision that has not been made
- Potentially owned by the wrong team or system

Call `add_hot_spot` and move on.

### 2e — Link Events to Commands

Now that all commands are captured, wire each event to the command that caused it.
This is the step that makes Gherkin emission work — the emitter derives scenarios from
command→event links.

Call `resume_session` to get the full list of events and commands, then for each event
ask: "Which command in this session triggered this?"

For each clear pairing, call:

```
link_event(session_id, event_id, command_id)
```

Most pairings are obvious from the names (`AccountOpened` ← `OpenAccount`,
`PaymentFailed` ← `ProcessPayment`). Work through the event list systematically.

**Important:** use `link_event` — do not re-add the event with `add_event`. Re-adding
creates a duplicate with a new ID while the original remains in any aggregate that
already references it, producing broken output at emission time.

Some events have no clear command trigger — they represent background facts, time-based
triggers, or external system inputs. Leave those unlinked; note them as hot spots if
they represent unresolved design questions.

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

Put the aggregate's invariant in `notes` as `Rule: …` — it is emitted as a Gherkin
`Rule`. If the rule changes later, `update_aggregate` rewrites it. When a command or
event turns up after clustering, add it with `assign_to_aggregate`; don't create a
second aggregate with the same name.

### 3b — Bounded Contexts

Ask:

"Could we replace this group of aggregates with a completely different implementation
without changing anything else? Does this group have a distinct team or deployment
boundary?"

Each such grouping is a **bounded context**. Call:

```
propose_bounded_context(session_id, name, description, aggregate_ids)
```

The server enforces: each aggregate belongs to exactly one bounded context. Add a
late aggregate to an existing context with `assign_to_context`.

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
- "Does another context use this term on purpose?" → pass those contexts as
  `shared_with`, so the term isn't reported as leakage there

---

## Wrap Up

Summarise what was captured:
- Events, commands, actors, policies, aggregates, bounded contexts, open hot spots
- Read back each hot spot and ask if it can be resolved now
- If you proposed anything, say how many notes are `source: facilitator` — they'll be
  tagged `@proposed` in the specs

Then ask: "Ready to emit the Gherkin specs?" If yes, run the `emit-gherkin-specs`
playbook.
