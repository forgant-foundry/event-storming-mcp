# Emit Gherkin Specs

Walk through generating and reviewing the outputs after a completed event storming
session:

| File | Contents |
|---|---|
| `{context}/{aggregate}.feature` | Gherkin scenarios, one per command with linked events, plus one per conditional event |
| `{context}/policies.feature` | One scenario per policy: when the trigger event happens, then the command is issued |
| `glossary.md` | Ubiquitous language reference table |
| `context-map.json` | Machine-readable model: contexts, aggregates, policies, read models, actors, hot spots |

Open hot spots appear as `# Open question` comments at the top of their context's
feature files.

---

## Prerequisites

Before emitting, verify the session has:

- [ ] Events linked to commands via `trigger_command_id` — use `link_event` to patch
  existing events **without re-adding them**. Re-adding creates a duplicate with a new
  ID. If the emit warns that events share a name, `merge_events` folds each duplicate
  into the one to keep, moving its links and notes.
- [ ] Commands assigned to aggregates via `cluster_aggregate`, or
  `assign_to_aggregate` for ones that turned up later
- [ ] Aggregates assigned to bounded contexts via `propose_bounded_context` or
  `assign_to_context`
- [ ] Policies with both a `trigger_event_id` and a `resulting_command_id`
- [ ] Key terms defined via `define_term` with canonical phrasings

Call `resume_session` to inspect the full event list and spot duplicate names before
emitting.

The emitter will produce output even if these are incomplete, but will include
warnings for each gap. Resolve warnings before handing the feature files to a BDD
team.

---

## Step 1 — Choose an Output Directory

Ask the domain expert: **"Where does your project keep its BDD specs?"**

Check for an existing directory before suggesting one:
- Is there a `features/` directory? (typical Cucumber / Behave / godog layout)
- Is there a `test/features/` or `spec/features/` directory?
- Does the project have a `cucumber.yml`, `.behat.yml`, or `pytest.ini` that names a path?

Use whatever the project already has. Only default to `features/` if none of the above
exist. Do not use `.event-storming/` or any other tool-internal path.

If feature files in that directory have already been edited by hand, don't overwrite
them — see "Re-emitting After Hand Edits" below.

---

## Step 2 — Emit

Call:

```
emit_gherkin(session_id, output_dir)
```

Review the summary:

- `scenario_count` — scenarios written, including policy scenarios
- `feature_files` — full paths of every file written
- `open_hot_spots` — unresolved questions; they're in the feature files as comments
- `warnings` — gaps to resolve in Step 6
- `lint` — glossary violations in the emitted steps, Givens included, per file; fix
  them in Step 5

Deprecated aggregates are skipped (they're listed in `context-map.json`); pass
`include_deprecated: true` only if the consumer wants their empty feature files.

---

## Step 3 — Phrasing Pass

Step text is derived from the event and command names, and some names don't turn
into good English — mass nouns ("a completeness is assessed"), multi-clause names,
names where the verb isn't where the emitter expects it.

When a derived noun phrase is exactly a glossary term, the emitter already uses the
term's canonical phrasing ("the cell knowledge", not "a cell knowledge"). So defining
the term often fixes a whole family of steps at once — prefer that to a phrasing per
step when the noun recurs.

Read every When and Then step yourself first. Collect the ones that read badly into
**one table** — current step, your proposed rewrite — and ask the expert to approve or
correct the whole table in a single reply. Don't ask about steps one at a time, and
don't ask about steps that already read well.

For each approved rewrite:

```
update_event(session_id, event_id, phrasing)      // Then text, without "Then"
update_command(session_id, command_id, phrasing)  // When text with the actor, without "When"
```

A command's phrasing is also its scenario title. Check each rewrite with `lint_steps`
before recording it, then re-emit once at the end.

---

## Step 4 — Review Each Feature File

For each `.feature` file in the summary, read it and ask the domain expert:

"Does this scenario correctly describe the real behaviour of the system?"

Check:
1. **Scenario title** — is it recognisable to a domain expert?
2. **Given steps** — do the preconditions capture the real state required?
3. **When step** — does it name the right actor and the right action?
4. **Then steps** — are these the events you would actually expect?
5. **Tag correctness** — `@context:X` and `@aggregate:Y` match the design

Give extra attention to:
- **`@facilitator` scenarios** — built from notes you proposed and the expert
  confirmed in the table. The tag records where they came from; leave `source` as it
  is. Read them once more as a set, since the expert approved them as rows, not as
  scenarios.
- **`policies.feature`** — for each policy ask "Does [command] really happen every time
  [event] happens? Is there a case where it shouldn't?" A case where it shouldn't is a
  `condition` (`update_policy`).
- **Open question comments** — read each one out; resolve it with `resolve_hotspot` if
  the expert can answer now.

Wrong notes, aggregates or rules are fixed in the session, not in the file: the
`update_*` tools, `remove_item`, and `update_aggregate` for a stale `Rule:` note.

For every step, ask: could the expert observe this? A step about streams, retries or
database transactions describes how the system is built, not how it behaves; rewrite
it or remove the note it came from.

If the same `Given` opens every scenario in a feature, move it to the aggregate's or
context's `preconditions` (`update_aggregate` / `update_bounded_context`): it's emitted
once as `Background`.

---

## Step 5 — Lint Step Text

The emit summary's `lint` already lists every glossary violation in the emitted steps.
For a rewrite you're considering, or any other text, call:

```
lint_steps(session_id, "the step text here", context_id)
```

**Alias violation** (`type: "alias"`): An alias is used instead of the canonical
phrasing. Fix: set the step's `phrasing` using the `use_instead` value from the lint
result, or rename the event/command. In a `Given`, reword the precondition with
`update_command(preconditions)` — and re-set any `negates` that quoted it.

**Cross-context leakage** (`type: "cross_context"`): A term from a different bounded
context appears in the step. Ask the expert whether the contexts share it on purpose:
- Yes — it's a shared kernel or published language: re-run `define_term` with this
  context in `shared_with`
- No — either the step belongs in a different context, or the term needs a separate
  definition per context

After fixing lint issues, re-emit to regenerate clean feature files.

---

## Step 6 — Resolve Warnings

### "command X has no linked events"

The command was captured but no events have `trigger_command_id` pointing to it.

Fix: if the event already exists, `link_event(session_id, event_id, command_id)`. If
not, add it:

```
add_event(session_id, name, trigger_command_id=<the command id>)
```

If the command was a mistake, `remove_item(kind: command)`. Then re-emit.

### "command X has a single conditional event and no happy path"

The command's only outcome is conditional, so nothing describes what happens when the
condition doesn't hold. Ask the expert: "What happens when [condition] isn't true?"
That's either the unconditional outcome (`add_event` with no condition) or a second
branch (`add_event` + `set_event_condition`). Two or more conditional outcomes with no
unconditional one is a branch, and doesn't warn.

### "aggregate X has no commands with linked events"

Events were clustered into the aggregate but no commands target it.

Fix: either the commands exist but weren't assigned (`assign_to_aggregate`), or they
haven't been captured yet (`add_command` with `aggregate_id`).

### "aggregate X has no commands or events"

The aggregate is empty. Either it was created speculatively and is not needed
(`remove_item(kind: aggregate)`, or `deprecate_aggregate` to keep it visible), or the
clustering step was skipped. Resolve with the expert.

### "policy X needs a trigger (an event or a trigger_time) and a resulting command"

Fix with `update_policy(trigger_event_id, resulting_command_id)` — or `trigger_time`
if what triggers it is time passing — or remove it if it was really a note about
something else.

### "N events share the name X"

A note was re-added instead of linked. For each, `merge_events(keep, remove)`: it
moves policies and fills the kept event's empty fields from the duplicate, and reports
any conflicts to settle with the expert.

### "policy X mentions Y, which is no longer an event or command name"

An event or command was renamed and the policy's name — its scenario title — wasn't.
Fix with `update_policy(name)`.

### "event X is in aggregate A but its trigger command Y is in B"

One command is changing two aggregates, which breaks the consistency boundary an
aggregate exists to draw. Ask the expert what in A reacts to Y's outcome. Usually it's
a policy: re-link X to a command in A (`link_event`), and add a policy from Y's event
in B to that command.

### "event X negates Y, which is no longer a precondition of Z"

The precondition was reworded after the condition was set. Call `set_event_condition`
again with the current wording in `negates`.

---

## Step 7 — Review context-map.json

Open `context-map.json` and verify:

- Every aggregate appears under the right bounded context
- Every glossary term is assigned to a context
- Policies, read models and hot spots sit under the context that owns them
- The `uncategorised_*` arrays are empty (or intentionally populated)

---

## Step 8 — Review glossary.md

For every term in each section, verify:

- The definition is precise and would make sense to a new team member
- The canonical phrasing reads naturally in a Gherkin step
- All known aliases are listed
- Terms used in more than one context list those contexts under Shared With

---

## Re-emitting After Hand Edits

Once the generated scenarios have been rewritten by hand (concrete examples, data
tables), a full re-emit would overwrite them. To refresh only the glossary and context
map:

```
emit_gherkin(session_id, output_dir, only: ["glossary", "context_map"])
```

To regenerate scenarios without losing the edits, emit to a scratch directory and
compare.

---

## Done When

- `emit_gherkin` produces zero warnings and an empty `lint`
- The expert has approved the phrasing table, and no step reads awkwardly
- Every rejection scenario's condition `negates` the precondition it contradicts
- The `@facilitator` scenarios have been read once more as a set
- The domain expert has read and approved at least one scenario per aggregate and
  every policy scenario
- Every open hot spot has been read out, and the expert has either resolved it or
  agreed to leave it open
- `context-map.json` has no unintended `uncategorised_*` entries
- `glossary.md` has a term entry for every significant noun in the feature files
