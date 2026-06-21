# User Feedback: event-storming-mcp

**Session context:** Full three-phase event storming (Big Picture → Process Modeling →
Software Design → Gherkin emit) for an event-sourced finance tracker API. ~35 events,
31 commands, 7 aggregates, 3 bounded contexts.

---

## 1. Playbook gap: no linking pass between Phase 2 and emit

**The biggest friction point.** `add_event` accepts an optional `trigger_command_id`,
but the `run-event-storming` playbook never instructs the facilitator to wire events to
commands after Phase 2. Phase 1 adds events before commands exist, Phase 2 adds
commands but doesn't loop back to update events, and Phase 3 clusters without linking.
The `emit-gherkin` playbook catches this as a prerequisite check — but by that point
the facilitation is "done" and you're doing cleanup, not discovery.

(Note: `LINKING_GAP.md` already tracks this — flagging here for completeness and
to add the suggested fix below.)

**Suggestion:** Add an explicit **Phase 2d — Link events to commands** step to the
`run-event-storming` playbook, immediately after commands are captured: *"For each
command, identify which event(s) it produces and call `add_event` with
`trigger_command_id` set."*

---

## 2. No `update_event` tool — re-adding creates duplicates

When attempting the linking pass by re-calling `add_event` with the same name and a
`trigger_command_id`, the tool created **new event records** with new IDs instead of
updating the existing ones. The original (unlinked) events remained in their aggregates;
the new (linked) events were orphaned outside any aggregate.

The emitter appeared to handle this gracefully (likely deduplicating by name), but the
session data is polluted with duplicate event names and the behavior is undocumented.

**Suggestion:** Either support upsert-by-name in `add_event` (same session + same name
= update in place), or add an explicit `update_event(session_id, event_id,
trigger_command_id)` tool.

---

## 3. No way to mark hot spots as resolved

Hot spots can be flagged but never closed. When decisions were made during the session
that resolved open questions, the only option was to create new hot spots prefixed with
`"RESOLVED:"` — a hack that pollutes the hot spot list and makes the session export
noisier.

**Suggestion:** Add a `resolve_hotspot(session_id, hotspot_id, resolution)` tool, and
surface resolved vs. open hot spots separately in the session export and
`context-map.json`.

---

## 4. No way to deprecate aggregates or events

When the domain expert decided the `Period` aggregate was unnecessary (snapshots made
period closes redundant), there was no way to remove or deprecate it from the session.
It continued to emit a `period.feature` file that had to be manually deleted. The
events `PeriodClosed` and `PeriodReopened` remained in the session with `"DEPRECATED"`
notes added manually.

**Suggestion:** Add a `deprecate_aggregate(session_id, aggregate_id, reason)` tool (and
equivalent for events). The emitter should skip deprecated aggregates when generating
feature files, or emit them with a `@deprecated` tag.

---

## 5. Emitter doesn't distinguish always-emitted vs. conditionally-emitted events

When multiple events share the same trigger command, the emitter lists all of them as
unconditional `Then` steps in a single scenario. Two cases hit this:

- `RecordCorrection` → `TransactionReversed` **and** `CorrectionRecorded` (mutually
  exclusive subtypes of the same command)
- `RecalculateProjection` → `ProjectionRecalculated`, `ProjectionGenerated`, **and**
  `ProjectedOverdraftDetected` (the last is conditional on a future balance going
  negative)

The generated scenarios were incorrect and required manual editing post-emit.

**Suggestion:** During the linking pass, prompt: *"Is this event always emitted, or
only under certain conditions?"* Conditional events could be flagged and emitted as
separate `Scenario` blocks with an added `Given` precondition, rather than being merged
into the happy-path scenario.

---

## 6. Bounded context name becomes directory name — spaces break convention

The bounded context `"Ledger Management"` was emitted as `ledger -management/` (with a
space-hyphen-space pattern) which is non-standard and breaks some test runners. Required
a manual `mv` to fix.

**Suggestion:** The emitter should normalize bounded context names to `kebab-case` for
directory names (`"Ledger Management"` → `ledger-management/`).

---

## 7. Output directory not prompted during facilitation

The `emit-gherkin` playbook lists conventional output paths (`features/`,
`test/features/`, `specs/features/`) but doesn't instruct the facilitator to ask the
domain expert where their project's BDD specs live. The facilitator defaulted to
`.event-storming/specs` (wrong), and the user had to correct it.

**Suggestion:** Add an explicit prompt to Step 1 of the emit-gherkin playbook: *"Ask:
where does your project keep its BDD specs? Check for an existing `features/`,
`specs/`, or `test/features/` directory before choosing an output path."*

---

## What worked well

- The three-phase playbook structure is excellent — it naturally surfaced the right
  questions in the right order.
- Probing prompts in Phase 1 ("what happens when things go wrong?", "what happens at
  the very end?") reliably drew out events that wouldn't have been volunteered.
- Policy modeling (`Whenever X, then Y`) made automation decisions explicit and easy
  to review.
- The ubiquitous language tooling (aliases, canonical phrasings, context assignment)
  is the strongest part of the tool — produced a glossary that's immediately usable.
- `sync_path` auto-persisting the session to a project file is a great feature for
  committing the session alongside the codebase.
