# Map the User's Story

A story mapping workshop (Patton, *User Story Mapping*) linked to the event storm.
The storm models the domain; the map models what people do with the product, in
what order, and what to build first. You are the facilitator; the human is the
expert on the people who'll use the product.

The map lives in the same session as the storm. If the project has a storm, call
`resume_session` with its ID; otherwise `start_session`. Either can come first:
mapping first reveals the commands the storm needs, and storming first gives the
map commands to link to. For a product with a front end, map first.

**Capture first, prune later.** Record as you go. `update_*`, `move_item` and
`remove_item` fix anything later.

**Keep it observable.** A step is something the person does or sees. Anything
the system does internally belongs in the storm, or in a design document.

**Record where each note came from:** `source: expert` for what the expert said,
`source: facilitator` for what you proposed and they confirmed.

## Propose-and-Confirm (only when the expert asks)

When the expert isn't a designer and asks for suggestions ("what screens would
this need?"), switch to proposals for that request:

1. Draft one batch, such as the steps of one activity or the screens of one
   journey, as a table: name, kind, what it links to, and one line of why.
2. Ask the expert to confirm, edit or reject each row in a single reply.
3. Record only confirmed rows, with `source: facilitator`.

Then go back to asking questions.

---

## 1. Frame the Opportunity

Ask: "Who is this for, and why are we building it?" Then talk through the rest
of the frame in whatever order the conversation goes:

- What problems does it solve for them?
- How do they cope today, and what hurts?
- What are we imagining building?
- Why does it matter to the business?
- How will we know it worked?

Record what you learn with `frame_story_map`. These are topics, not a form: an
unanswered one is fine. Write success measures as something the product could
emit ("months closed per accountant"), not as a feeling.

## 2. Model the Roles

From Cohn's user role modelling. Ask: "Who uses this?" Then:

1. **Brainstorm.** Collect every kind of user, without discussion.
2. **Organize.** Group roles that overlap.
3. **Consolidate.** Merge roles that use the product the same way.
4. **Refine.** For each role, ask about its goal, how often it uses the product,
   and its expertise.

Record each with `add_role`. Use `actor_id` if the storm already has the actor,
or `actor_name` to create it. Give a persona only to a role the team needs to
picture.

Always ask two more questions:
- "Who operates or supports this?" Operators are users too.
- "Who's the most unusual person who might use it?" An extreme character finds
  stories you'd otherwise miss.

## 3. Map the Big Picture

For one role at a time, ask them to tell the story: "First I…, and then I…".
Each "and then I" is a **step**: a short verb phrase in their words.

Go **a mile wide and an inch deep**. Get to the end of the story before
exploring any step. Then group the steps into **activities**, the backbone,
named the way the role would name them ("manage my ledgers"):

```
add_activity(session_id, name, role_id)
add_step(session_id, activity_id, role_id, name)
```

Steps are in narrative order. Pass `before_id` to place a step that comes
earlier, or reorder later with `move_item`.

**Hand-offs.** When a step waits for another role ("…and then the approver
approves it"), record the waiting step with `after_step_id` set to the other
role's step. Each journey belongs to one role. The later journey starts by
replaying the earlier one up to the hand-off.

## 4. Explore

Go back through the map and go deep. For each step:

- **Link it to the storm.** Ask "What does the system do when they do this?"
  That's a command: set `command_ids` with `update_step`. Ask "What do they
  look at here?" That's a read model: set `read_model_ids`. If the storm has
  nothing that fits, that's a gap. Raise it with `add_hot_spot` and keep going.
  It's resolved in the storm, with the events server.
- **Other ways.** "Is there another way to do this?" Record it as an
  `alternative` story.
- **What goes wrong.** "What could go wrong here?" Record it as a `failure`
  story. If the storm has the conditional event for it, pass it as `event_id`.
- **Misuse.** "Could anyone use this against its purpose?" Record only what the
  expert names, as a `misuse` story. Threat analysis comes later; don't invent
  threats here.
- **The novice.** "Could someone without your expertise do this step?" A domain
  expert's map can describe a product only experts can use.

Record details with `add_story(session_id, step_id, title, kind, why)`. Ask for
the **why** of each one: a story the expert can't give a why for may not be
worth building. Rank stories under a step with the most critical first.

**Screens.** For each place the steps happen, sketch the screen as a table:
what it shows (read models) and what it offers (commands), plus what it shows
after each offered command. Don't draw it and don't describe controls: no
buttons, fields or dropdowns.

```
add_screen(session_id, name, context_id, shows, offers: [{command_id, shows_after}])
update_step(session_id, step_id, screen_id)
```

Put each screen in its front end's bounded context (`context_id`, e.g. Web); its
features are emitted there. Always record `shows_after`: it's how a journey reads
the outcome from the screen instead of asking the API. A step a person takes off
any screen (an operator's `docker compose up`) gets `off_screen` with the reason
instead of a screen.

Every offered command will get its own scenario. A screen that offers a command
with no domain behaviour behind it is exactly how a product ships buttons that do
nothing.

**Risks.** When a "will they really…?" or "can we really…?" question comes up,
raise it with `add_hot_spot`. A feasibility risk that needs building to answer
is a `spike` story, placed in a slice ahead of the work that depends on it.

## 5. Slice

There's always too much to build. Cut the map into horizontal slices, top to
bottom:

1. **Release strategy.** Ask: "What's the smallest set of stories that reaches
   the outcome in the frame?" That's a `release` slice; give it its outcome.
2. **Learning strategy.** Ask: "Which of our assumptions is riskiest? What's the
   smallest thing we could put in front of people to learn whether it's true?"
   That's a `learning` slice.
3. **Development strategy.** Within a release, ask: "What's the thinnest path
   through every step that we could see working end to end?" That's the
   **walking skeleton**: a `development` slice with `walking_skeleton: true`. It
   works, but not well enough to release. Later development slices fill in the
   rest.

```
add_slice(session_id, name, kind, outcome, walking_skeleton)
assign_to_slice(session_id, slice_id, story_ids)
```

A step is in a slice when one of its stories is, so every step on the path needs
at least one story in the slice.

## 6. Check the Link

Call `check_links` and work through the findings with the expert:

- **Warnings** are things that would be built wrong or not at all: steps with no
  command or read model, screens offering a command with no domain behaviour,
  steps using a command their screen doesn't offer. Fix the map with the
  `update_*` tools, or raise a hot spot for the storm.
- **Notes** are questions: a command no step uses ("who issues this?"), a read
  model no screen shows ("who sees this?"), a role with no steps.
- **Lint** findings are glossary violations in the map's wording. Use the
  canonical phrasing.

The map is linked when `check_links` has no warnings and no lint, and every note
has an answer.

## 7. Confirm the Next Slice

Only for the slice going to the factory next, usually the walking skeleton. The
rest of the map stays a mile wide and an inch deep. This is the story workshop,
compressed into **example mapping**: for each story in the slice,

1. **The why.** If the story has none, ask for it.
2. **The rules.** Ask: "What has to be true for this to be done?" Each answer is
   a rule, recorded with `add_criterion` as its `text`.
3. **An example per rule.** Ask: "Give me one concrete case." Write it as
   Given/When/Then in the domain's words, with realistic values. If several
   cases differ only in values, give them as `examples` rows and refer to the
   columns as `<column>` in the steps.
4. **Questions** nobody can answer now become hot spots (`add_hot_spot`).

Play the tester while you do it: for each rule ask "what about…?", and propose
the failure and misuse examples the expert might be too optimistic to consider.
Write behaviour, not mechanics: no clicks, buttons or fields. Those belong to the
test's driver.

Then the journey itself. For each step in the slice, ask: "What would we type in,
and what would we see?" Record it as `journey_when` and `journey_then` with
`update_step`, naming the screen ("the accountant records a transaction of -42.50
on the account screen" / "the account screen shows a balance of 957.50"). Keep the
values consistent from step to step: they're one person's path through the
product. If every journey starts the same way ("the developer rung is running"),
record it as the slice's `preconditions`.

**Constraints.** Ask: "How fast, how available, how accessible does this need to
be, and for whom?" Record each as `add_constraint` with what's measured and its
target in the user's terms. "Fast" or "easy" isn't a target.

## 8. Check Readiness and Emit

Call `check_readiness` for the slice. Work through every blocking item:
- a story without a why or criteria;
- a step with no screen and no `off_screen`;
- a step using a command or read model its screen doesn't offer or show;
- an outcome that can't be read from the screen (no `shows_after` or
  `journey_then`);
- an offered command with no domain behaviour in the storm (raise it for the
  events server);
- a constraint without a measure and target;
- glossary lint.

Read the notes back to the expert: UI mechanics in a criterion, a story that
sounds split by technical layer, an offered command no journey uses.

When the slice is ready, call `emit_story_map` with the project's specs
directory, the same one the storm's specs go to. It writes:
- the slice's journeys, one scenario per role;
- each story's criteria as scenarios;
- a scenario per read model each screen shows and per command it offers;
- `constraints.feature`;
- `story-map/slices.json`, which a factory reads for readiness and build order;
- `story-map/story-map.md` for people.

Read one journey and one screen feature back to the expert.

## Wrap Up

Read back the frame, the roles, the backbone, and each slice with its outcome
and whether it's ready.
Read back the open hot spots, and say which are gaps for the storm. If you
proposed anything, say how much of the map is `source: facilitator`.
