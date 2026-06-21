# Emit Gherkin Specs

Walk through generating and reviewing the output triple after a completed event
storming session. The output triple is:

| File | Contents |
|---|---|
| `{context}/{aggregate}.feature` | Gherkin scenarios, one per command with linked events |
| `glossary.md` | Ubiquitous language reference table |
| `context-map.json` | Machine-readable structural model |

---

## Prerequisites

Before emitting, verify the session has:

- [ ] Events linked to commands via `trigger_command_id` — use `link_event` to patch
  existing events **without re-adding them**. Re-adding creates a duplicate with a new
  ID while the original (already in an aggregate) remains unlinked. If you have
  duplicates from a previous re-add attempt, use `remove_event` to clean them up.
- [ ] Commands assigned to aggregates via `cluster_aggregate`
- [ ] Aggregates assigned to bounded contexts via `propose_bounded_context`
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

---

## Step 2 — Emit

Call:

```
emit_gherkin(session_id, output_dir)
```

Review the summary:

- `scenario_count` — how many scenarios were written (should match your command count
  minus any commands with no linked events)
- `feature_files` — full paths of every file written
- `warnings` — commands with no linked events, aggregates with no commands, etc.

---

## Step 3 — Review Each Feature File

For each `.feature` file in the summary, read it and ask the domain expert:

"Does this scenario correctly describe the real behaviour of the system?"

Check:
1. **Scenario title** — is it recognisable to a domain expert?
2. **Given steps** — do the preconditions capture the real state required?
3. **When step** — does it name the right actor and the right action?
4. **Then steps** — are these the events you would actually expect?
5. **Tag correctness** — `@context:X` and `@aggregate:Y` match the design

---

## Step 4 — Lint Step Text

For any step that looks wrong or uses informal language, call:

```
lint_steps(session_id, "the step text here", context_id)
```

**Alias violation** (`type: "alias"`): An alias is used instead of the canonical
phrasing. Fix: update the step to use the `use_instead` value from the lint result.

**Cross-context leakage** (`type: "cross_context"`): A term from a different bounded
context appears in the step. Fix: either the step is wrong (it belongs in a different
feature file) or the term definition needs to be split per context.

After fixing lint issues, re-emit to regenerate clean feature files.

---

## Step 5 — Resolve Warnings

### "command X has no linked events"

The command was captured but no events have `trigger_command_id` pointing to it.

Fix: identify the event(s) that should result from this command and add them:

```
add_event(session_id, name, trigger_command_id=<the command id>)
```

Then re-emit.

### "aggregate X has no commands"

Events were clustered into the aggregate but no commands target it.

Fix: either the commands exist but weren't assigned (`cluster_aggregate` with the
correct `command_ids`), or the commands haven't been captured yet (`add_command`).

### "aggregate X has no commands or events"

The aggregate is empty. Either it was created speculatively and is not needed, or
the clustering step was skipped. Resolve with the expert.

---

## Step 6 — Review context-map.json

Open `context-map.json` and verify:

- Every aggregate appears under the right bounded context
- Every glossary term is assigned to a context
- The `uncategorised_aggregates` array is empty (or intentionally populated)

---

## Step 7 — Review glossary.md

For every term in each section, verify:

- The definition is precise and would make sense to a new team member
- The canonical phrasing reads naturally in a Gherkin step
- All known aliases are listed

---

## Done When

- `emit_gherkin` produces zero warnings
- `lint_steps` returns clean for a sample from each bounded context
- The domain expert has read and approved at least one scenario per aggregate
- `context-map.json` has no `uncategorised_aggregates`
- `glossary.md` has a term entry for every significant noun in the feature files
