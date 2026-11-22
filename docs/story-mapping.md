# Story Mapping: Design

**Status:** draft 3, 2026-11-21. Decisions on the open questions recorded under
"Decisions". Implements TODO.md §4 and §13–15, extended with
research across forgant-library (see "What the library says").

## Why

finant-api's event storm specified the domain fully and the person using it barely
at all, because it started from an API, not a product. Its web front end arrived
with 228 scenarios passing and couldn't be used: 25 buttons on 7 screens, none
wired, and no Web scenario that issued a command (TODO.md §14). A factory builds
exactly what the specs say. Whatever the specs leave out, it leaves out too.

Event storming models the domain: what happens, what causes it, which boundaries
keep it consistent. Story mapping models what a person does with the product, in
what order, and what's worth building first. This design adds story mapping beside
the storm, sharing its session and glossary.

The goal isn't a story map for its own sake. It's the **best possible first input
for a software factory**: specs that say what to build, for whom, why, in what
order, how to check it, and how to tell afterwards whether it worked.

## What the library says

Most of the feature set comes from these findings, each with its source.

### On stories

- **A story is card, conversation, confirmation.** Confirmation answers "If we
  build what we agree to, what will we check to see that we're done?" (Patton
  ch. 6). Patton's demonstration question, "how will we do that?", "often reveals
  some holes", such as needing realistic data. Cohn agrees: acceptance tests
  "express details that result from conversations between a customer and a
  developer" (ch. 6).
- **Who, what, why.** The Connextra template makes teams "pause and think about:
  who, what, and why. And, if they didn't know, they might question whether they
  should be writing the story at all" (Patton ch. 7). Patton's checklist adds
  "Talk about questions and assumptions", both about users ("Will they really use
  this solution?") and technical ones (ch. 7).
- **Detail at the last responsible moment.** Cohn's smells include "thinking too
  far ahead", "adding too many details to stories" and "including user interface
  details too soon" (ch. 14). Patton puts the "last best conversations", the story
  workshop, just before a development cycle, where "each story satisfies as little
  as one agreement" (ch. 16–17).
- **Testable, or it isn't done.** "A user must find the software easy to use" is
  Cohn's example of an untestable story (ch. 2). Non-functional requirements
  belong on constraint cards, and "an automated test can be written (and run at
  least daily) to ensure compliance with a constraint" (ch. 7, 16).
- **Slice the cake.** Don't split along technical lines ("fill out a resume form"
  / "written to the database"), because "neither story on its own is very useful
  to users" (Cohn ch. 7). Use a spike story when something can't be estimated
  (ch. 2).
- **Roles, not one user.** "Most project teams consider only a single type of
  user" (Cohn ch. 3). He gives roles attributes, gives personas only to the
  important ones, and uses extreme characters to find stories "you would be
  likely to miss otherwise".
- **Domain-expert proxies have a known failure.** An expert proxy risks "a system
  that is perfect for experts, but that is not usable by those with less than
  expert level domain knowledge" (Cohn ch. 5). Our sessions are facilitated with
  exactly such an expert.

### On products

- **Start with the opportunity.** Patton's opportunity canvas, built on Cagan's
  Opportunity Assessment, covers problems, solution ideas, users, how they cope
  today, and how success is measured. "This isn't a form to fill out" (Patton
  ch. 13). A "now" map of today's flow, with its pains and joys, shows where the
  opportunities cluster (ch. 13–14).
- **Four risks, tackled first:** value, usability, feasibility and business
  viability, all "prior to deciding to build anything" (Cagan ch. 7). Feasibility
  risks get a feasibility prototype: "just enough code to mitigate the feasibility
  risk" (Cagan ch. 46).
- **Outcomes, not output.** "It's all about solving problems, not implementing
  features" (Cagan ch. 7). "Value can only be defined by the ultimate customer"
  (Lean Thinking). After each release, "build metrics into your product that
  allow you to track usage of new features" and observe users (Patton ch. 18).
- **A prototype can be the spec, with help.** It "will likely need to be
  supplemented with additional details (usually, use cases, business rules, and
  acceptance criteria)" (Cagan ch. 45). Cagan calls the story map one of his two
  go-to techniques (ch. 38).

### On delivery

- **Acceptance criteria are executable specifications** written in the customer's
  language. Tests have three layers, and only the bottom one, the application or
  window driver, knows how to use the UI (Continuous Delivery ch. 8). "Blindly
  automating badly written acceptance criteria is one of the major causes of
  unmaintainable acceptance test suites."
- **Non-functional requirements are cross-functional.** They "should be specified
  as part of your application's requirements in exactly the same way" as
  functional ones, and prioritised alongside them (CD ch. 9). "Define SLOs like a
  user" (SRE, Appendix B), using indicators such as latency, error rate and
  availability (ch. 4).
- **Operators are users, too** (Release It!, *Foundations*).
- **Small batches, fast feedback.** Slice work "into small pieces that can be
  completed in a week or less", gather customer feedback, and let teams
  experiment (Accelerate, Appendix A). Smaller steps limit "the time-horizon over
  which our assumptions need to hold" (Modern Software Engineering ch. 4).

### On security and privacy

- **Stories don't yield security requirements.** "I am not aware of any structured
  approach for doing so for security requirements. The primary challenge is that
  security is rarely a goal of a feature" (Shostack ch. 12).
- **Privacy analysis starts from a data flow diagram** of external entities,
  processes, data stores and data flows, with trust boundaries. Threats are then
  documented as misuse cases (LINDDUN tutorial, steps 1–3).

The storm and the story map together already contain that diagram (see Tier 3
below). That's the right place for the map to help security: give the factory's
threat work an accurate data flow diagram, rather than pretend to generate
security requirements.

## The feature set

The tiers are ordered by how directly each one changes what the factory builds,
and how it checks it.

### Tier 1: what the factory builds and tests

1. **The map.** Frame, roles, activities, steps in narrative flow, stories,
   screens and slices (Patton ch. 5).
2. **Stories with who, what, why and confirmation.** Each story has a *why*, and,
   once it's headed for the factory, **acceptance criteria**. Each criterion
   becomes one scenario, and examples with realistic data become a Scenario
   Outline table (Patton ch. 6, Cohn ch. 6, CD ch. 8).
3. **Journeys and screens.** One `@running` journey per role per slice. For each
   screen, a scenario per read model it shows and per command it offers, so no
   button ships without a handler (TODO.md §14).
4. **Constraints as testable cards.** These are cross-functional requirements
   (performance, availability, security, privacy, accessibility, compliance,
   operability, technology) attached to the map, a slice, a step or a screen.
   Each has a measure and a target in the user's terms, such as "the balance
   shows within 1 second for 95% of requests". The lint flags untestable
   wording ("fast", "easy", "user-friendly", "intuitive") (Cohn ch. 2, 7, 16; CD
   ch. 9; SRE ch. 4).
5. **Readiness, per slice.** A slice is *ready for the factory* when:
   - every story in it has a why and acceptance criteria;
   - every step links to the storm;
   - every offered command has domain behaviour;
   - every constraint has a measure;
   - the glossary lint is clean.

   Readiness is checked per slice, so the rest of the map can stay a mile wide
   and an inch deep. That resolves the tension between Cohn's "thinking too far
   ahead" and a factory's need for full detail: full detail is required only for
   the slice being handed off.
6. **A handoff manifest.** `handoff.json` lists the slices in build order, their
   readiness, and the files that make up each one. It's the single document a
   factory reads.

### Tier 2: lowering the risk of building the wrong thing

7. **The frame as an opportunity canvas:** problems, users, how they cope today,
   solution ideas, why it matters to the business, and how success is measured
   (Patton ch. 13). The playbook treats it as topics to discuss, not a form.
8. **Outcomes and metrics on slices.** Each release or learning slice states its
   outcome and how it will be observed (Patton ch. 18). The emit turns each
   metric into an **instrumentation requirement**: the product must emit what
   the metric needs. The spec says only that the product emits it, never who
   consumes it or how. A factory can build that like any other scenario; observing
   outcomes can't be added after release if nothing was measured.
9. **Assumptions and risks.** Each is recorded with its kind (value, usability,
   feasibility or viability, from Cagan ch. 7), the assumption itself, and how
   it will be tested. Learning slices exist to retire them. Feasibility risks
   become **spike** stories, scheduled ahead of the slices that depend on them
   (Cohn ch. 2, Cagan ch. 46, Patton ch. 4's risk stories). The handoff orders
   spikes and the walking skeleton first.
10. **Richer roles.** Each role gets attributes (frequency of use, expertise,
    goal) and a persona only where it helps (Cohn ch. 3). The playbook prompts
    for an **operator** role (Release It!), for extreme characters, and for the
    novice-user question in Cohn ch. 5: "Could someone without the expert's
    knowledge do this step?"

### Tier 3: safety

11. **A data flow diagram, emitted as Mermaid (`dfd.mmd`)** from the storm and the
    map:

    | DFD element | Taken from |
    |---|---|
    | External entities | Roles, and actors of type `external_system` |
    | Processes | Commands, grouped by aggregate |
    | Data stores | Aggregates and read models |
    | Data flows | A role's step to its command, a screen's read model to the role, and storm policies between aggregates |
    | Trust boundaries | Between external entities and the system, and between bounded contexts |

    This is LINDDUN step 1 and the starting point for STRIDE, ready for
    forgant-factory's threat work. In Mermaid, external entities are rectangles,
    processes are rounded nodes, data stores are cylinders, flows are labelled
    arrows (carrying their data classification), and each trust boundary is a
    `subgraph`. It renders on GitHub and in most editors, and `story-map.json`
    carries the same elements for tools.
12. **Data classification.** Read models and commands record the data they carry:
    personal, sensitive, payment, credentials or none. The diagram labels its
    flows with it, and a step that shows personal data to a role is listed for
    privacy review.
13. **Misuse stories.** A story `kind: misuse` ("a signed-out visitor reads another
    accountant's ledger") is recorded when the expert names one. Each becomes a
    scenario expecting refusal. The map never invents them; that's the threat
    cell's job, working from the diagram.

### Tier 4: quality checks

14. **Story lint**, reported like the step lint:
    - a story with no why;
    - a story in a ready slice with no acceptance criteria;
    - a story split along technical layers (its text is about a form, a
      database, an API or a table, not the person; Cohn ch. 7);
    - a story with many criteria, which may need splitting (Patton ch. 17);
    - UI-mechanics wording in steps, stories or criteria ("click", "button",
      "field", "dropdown"), which belongs in the test driver layer, not the spec
      (CD ch. 8; Cohn ch. 14's "including user interface details too soon").
15. **The link check**, run both ways (unchanged from draft 1):
    - a step that uses no command and shows no read model;
    - a screen offering a command with no linked events;
    - a step on a screen that uses a command the screen doesn't offer;
    - a command that no step uses and no policy issues;
    - a read model no screen shows;
    - a role with no steps.
16. **A "now" map (optional).** For a product that replaces an existing way of
    working, map today's flow first, with pains and joys, then the future map
    over it (Patton ch. 14). The pains become problems in the frame. This is the
    lowest priority, and can come later.

### Not building

- **No requirements form.** Patton says the canvas "isn't a form", and Cohn
  warns against templates "to capture all of the details". Every field beyond
  name, kind and links is optional, and required only by readiness, per slice.
- **No generated security requirements.** We provide the diagram and record
  misuse stories the expert names.
- **No estimates or velocity.** The factory plans its own work. The map's job is
  to say what to build and in what order, not how long it takes.

## Architecture decisions

1. **One binary, renamed, with two servers.** The executable becomes
   `forgant-discovery`, with `forgant-discovery serve events`
   for the storm and `forgant-discovery serve stories` for story mapping. A storm
   session doesn't load story-map tools, and vice versa.

   The rename touches:
   - forglet: `.forglet.yml`'s `name`, which names the delivered binary;
   - `cmd/`: the root command, plus a `serve` parent with `events` and `stories`
     children;
   - every `.mcp.json` that runs the server today: forgant-factory, finant-api
     and any other repo with an `event-storming-mcp` entry, which become
     `forgant-discovery` with `["serve", "events"]`;
   - the README and `CLAUDE.md`.

   The Go module path changes only if the repo is renamed too, which can be done
   separately. During the switch, `event-storming-mcp` keeps working where it's
   installed: it's the old binary, and nothing removes it until we do.
2. **One session document.** The story map is a `story_map` section of the
   storm's session. Record IDs survive `import_session`; only the session ID
   changes. Both servers take an exclusive `flock` on a lock file next to
   `session.json` around each load-modify-save.
3. **A role is a storm actor** with story-mapping detail added.
4. **Shared code, not a published schema.** Types go in
   `internal/session/storymap.go`, tools in `internal/mcp/storymap.go`, and the
   emitters in `internal/gherkin/` reuse `Phraser` and `LintStep`. Shared tools
   (sessions, glossary, lint, hot spots, `remove_item`, playbooks) are
   registered by both servers.

## The model

Every field beyond identity and links is optional; readiness says which ones a
slice needs.

```
StoryMap {
  frame:       Frame
  roles:       [Role]
  activities:  [Activity]        // backbone, narrative order
  steps:       [Step]            // narrative order
  stories:     [Story]           // vertical rank within a step
  screens:     [Screen]
  slices:      [Slice]           // top to bottom
  constraints: [Constraint]
  risks:       [Risk]
}

Frame      { for_whom, problems[], today, solution_ideas[], why, success_measures[] }
Role       { id, actor_id, goal, frequency?, expertise?, persona? }
Activity   { id, name, role_id? }
Step       { id, activity_id, role_id, name,
             command_ids[], read_model_ids[], screen_id?,
             after_step_id? }   // a hand-off: another role's step this one waits for
Story      { id, step_id, title, why?, kind, slice_id?, event_id?,
             criteria: [Criterion], source }
             // kind: detail | alternative | failure | misuse | spike
Criterion  { text, examples?: { columns[], rows[][] } }   // → Scenario / Scenario Outline
Screen     { id, name, shows: [read_model_id], offers: [Offer] }
             // platform and user type: decision postponed, see Decisions
Offer      { command_id, shows_after? }
Slice      { id, name, kind, outcome?, metrics: [Metric], walking_skeleton }
             // kind: release | learning | development
Metric     { name, emits }            // what to measure, and what the product emits for it
Constraint { id, text, kind, scope_kind, scope_id?, measure?, target? }
             // scope_kind: map | slice | step | screen
Risk       { id, kind, assumption, test, slice_id?, status }
             // kind: value | usability | feasibility | viability
             // status: open | retired | realised
```

The storm side gains one field: `data` on read models and commands, `[personal |
sensitive | payment | credentials]`, for the data flow diagram and privacy
review.

## What's emitted

`emit_story_map(session_id, output_dir, only?, slice?)` writes:

```
<output_dir>/story-map/
  handoff.json                        build order, readiness, files per slice
  journeys/<slice>/<role>.feature     @running @slice:<s> @role:<r>
  stories/<activity>.feature          one scenario per acceptance criterion
  screens/<screen>.feature            per read model shown, per command offered
  constraints.feature                 @constraint scenarios with measure and target
  instrumentation.feature             @measure scenarios from slice metrics
  dfd.mmd                             Mermaid data flow diagram, trust boundaries as subgraphs
  story-map.json                      the whole map, names resolved
  story-map.md                        the map as a table, plus risks, constraints and readiness
```

### Journeys

A journey is one scenario per role per slice: the role's steps that have stories in
that slice, in narrative order, meant to run against a running product.

```gherkin
@running @slice:walking-skeleton @role:accountant
Feature: Accountant — walking skeleton

  Scenario: the accountant records a month's spending
    When the accountant opens a ledger
    Then a ledger is opened
    When the accountant records a transaction from the transactions screen
    Then a transaction is recorded
    And the transactions screen shows the new balance
```

Several When/Then pairs in one scenario break the usual one-behaviour rule on
purpose: a journey checks that the steps connect, and the story and aggregate
scenarios check each behaviour. A journey walks every step itself; it doesn't
use a `Background` for the earlier ones.

**Hand-offs between roles.** Each journey belongs to one role. When a step waits
on another role's step (`after_step_id`), the later journey is staged by
**replaying the earlier role's journey up to that step as `Given`s**: each of its
outcomes becomes a precondition, then the role's own walk begins.

```gherkin
@running @slice:first-release @role:approver
Feature: Approver — first release

  Scenario: the approver approves a large transfer
    Given a ledger is opened
    And a large transfer is submitted for approval
    When the approver reviews the pending transfer from the approvals screen
    Then the approvals screen shows the transfer
    When the approver approves the transfer
    Then the transfer is approved
```

The two `Given`s are the accountant's journey up to the hand-off, replayed as
state rather than as the accountant's actions. That keeps the approver's journey
about the approver, and it stays correct when the accountant's journey changes,
since both are derived from the same steps. Because journeys are derived from the
map and the storm's phrasing, any change there re-emits every journey it touches.
Nothing is written by hand.

### Stories

Each acceptance criterion becomes a scenario tagged with its story, step and slice:

```gherkin
@story:record-a-split-transaction @slice:first-release
Scenario Outline: a split transaction's parts add up to its total
  Given a transaction of <total>
  When the accountant splits it into <parts>
  Then the split is <result>

  Examples:
    | total  | parts         | result   |
    | 100.00 | 60.00, 40.00  | accepted |
    | 100.00 | 60.00, 30.00  | refused  |
```

### Constraints and instrumentation

```gherkin
@constraint:performance @screen:transactions
Scenario: the transactions screen shows the balance promptly
  Then the balance is shown within 1 second for 95% of requests

@measure @slice:first-release
Scenario: the product emits a measure for each month close
  When the accountant closes a month
  Then the product emits "month closed"
```

### Readiness and the handoff

`handoff.json` is the factory's entry point:

```json
{
  "build_order": ["spike: bank import format", "walking-skeleton", "first-release"],
  "slices": [{
    "name": "walking-skeleton",
    "ready": false,
    "blocking": [
      "story \"record a transaction\" has no acceptance criteria",
      "screen \"Transactions\" offers RecordTransfer, which has no linked events"
    ],
    "files": ["journeys/walking-skeleton/accountant.feature", "…"]
  }]
}
```

The handoff lists not-ready slices too, with what's blocking them, so the factory
can refuse a slice cleanly instead of building a guess.

## The playbook: `map-the-users-story`

This follows Patton's six steps, with Cohn's role modelling and Patton's
opportunity canvas at the front.

1. **Frame the opportunity:** problems, users, how they cope today, why it
   matters, how we'll know it worked. These are topics to talk through, not a
   form.
2. **Model the roles** (Cohn ch. 3). Brainstorm, organize, consolidate, refine.
   Prompt for an operator role and one extreme character.
3. **Map the big picture.** For each role, write steps in narrative flow,
   "…and then I…", a mile wide and an inch deep, then group them into
   activities named the way the role names them.
4. **Explore.** Cover other roles, other ways of doing each step, what goes
   wrong, and the novice-user question. Sketch each screen as what it shows and
   offers, as a table. Record assumptions and risks as they surface.
5. **Slice.** Cut a release strategy (outcomes and metrics), a learning strategy
   (which risks each slice retires) and a development strategy (the walking
   skeleton, with spikes ahead of it).
6. **Confirm the next slice, link and emit.** Only for the slice going to the
   factory: give each story its why and acceptance criteria with realistic
   examples, and give each constraint its measure. Run `check_links`, raise a
   hot spot for each gap the expert confirms, and emit until the slice is ready.

The playbook has the same propose-and-confirm mode as `run-event-storming`:
proposals as one table, recorded only once confirmed, with `source:
facilitator`. It also carries over "could the person observe this?": a step is
something the person does, never something the system does internally.

Step 6 is the story workshop compressed (Patton ch. 16). Patton wants a
developer, a tester and someone who understands the UI in the room. The
facilitator plays the tester: for each criterion it asks "what about…?" and
proposes the failure and misuse examples the expert might be too optimistic to
consider.

**Map or storm first?** Either works. For a product with a front end, the playbook
suggests mapping first, since that's what finant-api lacked.

## Vocabulary

These words reach the specs, so they're fixed now:
- **Story:** a card on the map.
- **Criterion:** one thing we'll check to confirm a story is done.
- **Journey:** an emitted `@running` scenario, a role's steps through a slice.
- **Screen:** where a step happens. It isn't an aggregate.
- **Spike:** a story that retires a feasibility risk.

Don't call the practice "journey mapping". In UX that term means mapping a current
experience, which is what our optional "now" map does, not what to build.

## Migrating screens modelled as aggregates

finant-api's Web context modelled each screen as an aggregate. Converting one:
- the aggregate becomes a screen;
- each `View<Screen>` read model becomes a `shows` entry;
- each offered command named in the context description becomes an `offers`
  entry;
- a conditional "shown as stale" becomes a failure story.

Then remove the aggregate, its `View` command and its `Shown` events from the
storm. Do the first conversion by hand with the playbook; it only becomes a tool
if a second session needs it.

## Phases

Each phase ends with something forgant-factory can use.

1. **The rename, the map and its link.** The `forgant-discovery` rename with
   `serve events` / `serve stories`, model and storage (with the lock), capture
   and edit tools, `check_links`, and the playbook through step 5. *Done when* finant-api's Web context can be mapped and checked.
2. **Confirmation and the handoff.** Stories with why and criteria, examples
   tables, constraints, readiness, and the emits for journeys, stories, screens
   and constraints, plus `handoff.json`, `story-map.md` and the story lint.
   *Done when* a walking-skeleton slice for finant-api is ready, its features
   parse, and every screen command has a scenario.
3. **Outcomes and risks.** The opportunity frame, slice metrics and
   instrumentation, risks and spikes, and build order. *Done when* the handoff
   orders spikes and the skeleton first, and every release slice has a metric
   the product emits.
4. **Safety.** `data` classification, `dfd.mmd` and misuse stories. *Done when*
   forgant-factory's threat work runs from `dfd.mmd`.
5. **Migration and the "now" map.** Convert finant-api's screens, then add the
   optional "now" map.

## Decisions

Decided 2026-11-21:

1. **Name and subcommands.** Rename the executable, with `serve events` and
   `serve stories`. The name is `forgant-discovery` (from TODO.md §13, confirmed).
2. **Journey shape.** One scenario walks the whole journey; no `Background`. The
   emitted files may change shape as journeys evolve, and because they're
   derived, they're regenerated rather than edited.
3. **Screen categories.** Screens will be categorised by platform and by user
   type; how is postponed. It doesn't block progress:
   - **User type** is already derivable: a screen's user types are the roles
     whose steps happen on it. The emit and `story-map.md` can group by it with
     no new data.
   - **Platform** needs new data, but as an optional field added later, so
     nothing captured now has to change. Until then, every screen is one
     platform.
4. **Multi-role journeys.** One role per journey. A hand-off (`after_step_id`)
   stages the later journey by replaying the earlier role's journey up to that
   step as `Given`s.
5. **Instrumentation wording.** The spec says the product *emits* the measure,
   never who consumes it.
6. **Data flow diagram format.** Mermaid, `dfd.mmd`, with the same elements
   available in `story-map.json`.
