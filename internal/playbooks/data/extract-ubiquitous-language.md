# Extract Ubiquitous Language

A focused glossary-building pass over an existing event storming session. Run this
after Phase 2 (Process Modeling) and before `emit_gherkin` to ensure that emitted
Gherkin steps use the domain's precise, agreed-on language.

## When to Use This Playbook

- After a rapid Big Picture session where naming was loose
- When you notice the expert using multiple words for the same concept
- Before calling `emit_gherkin` to prevent glossary lint failures
- When onboarding a new team member who needs the vocabulary explained

---

## Step 1 — Resume the Session

Call `resume_session(session_id)` and review the full model. Before you begin,
make a mental note of:

- Event or command names that look like synonyms (e.g. `OrderCreated` and
  `OrderPlaced`)
- Informal names that use dev jargon rather than business language (e.g.
  `UserRecordUpdated` vs `CustomerProfileChanged`)
- Names that are ambiguous across bounded contexts (e.g. `Account` could mean a
  bank account or a user account)

---

## Step 2 — Walk Through the Events

For each event, ask:

"Is `[EventName]` the right name? If a new team member joined tomorrow, what would
*they* call this?"

If the expert prefers a different name, rename it in place with `update_event` or
`update_command` — the ID and every link stay the same. Do the same for actors,
policies, read models, aggregates and contexts with their `update_*` tools.

Two names for the same thing (`OrderCreated` and `OrderPlaced`) are a duplicate, not a
synonym: keep one, move its links over (`link_event`, `assign_to_aggregate`), and
delete the other with `remove_item`.

---

## Step 3 — Surface Synonym Clusters

Look for groups of terms that might refer to the same concept:

```
cart / basket / order → which one wins?
user / customer / member / account holder → are these the same or different?
send / submit / dispatch / publish → which verb does the business use?
cancel / void / reverse / refund → do these mean different things in this domain?
```

For each cluster, ask the expert:

"Do these all mean the same thing, or do they describe different situations?"

- If they are the same: pick one canonical term, add the others as aliases
- If they are different: define each separately

---

## Step 4 — Define Each Term

For every significant concept, call:

```
define_term(
  session_id,
  term,             // the canonical name
  definition,       // precise, expert-agreed definition (1–3 sentences)
  canonical_phrasing, // how it appears in a Gherkin step ("the order", "the customer")
  aliases,          // synonyms to flag during lint ["cart", "basket"]
  context_id,       // which bounded context owns this term (optional but recommended)
  shared_with       // other contexts that use the term on purpose (optional)
)
```

**Tips for good definitions:**
- Use the expert's words, not your paraphrase
- Include what makes this concept distinct from similar ones
- If a term means different things in different contexts, define it once per context

**Tips for canonical phrasings:**
- Lowercase, as it will appear mid-sentence in a Gherkin step
- Include the article: "the order", "an invoice", "the customer's subscription"
- If the term is used as a verb: "the customer cancels", "the system processes"

---

## Step 5 — Verify With Lint

After defining terms, validate step text with:

```
lint_steps(session_id, "the step text to check", context_id)
```

A clean result means:
- No aliases are present (only canonical phrasings)
- No terms from other bounded contexts appear (no cross-context leakage)

Work through any issues by updating the glossary or rewording the step.

---

## Step 6 — Assign Terms to Contexts

If you have proposed bounded contexts, make sure each term is assigned to the right
one by passing `context_id` to `define_term`. This enables cross-context lint checks
later.

Ask: "Is [term] only meaningful inside [context], or is it shared across contexts?"

Terms shared across contexts are a design smell — they may indicate a concept that
needs its own bounded context, or they may represent a shared kernel that needs
careful management.

When the sharing is deliberate — a shared kernel, or a published language one context
hands to another (the bug bar the product manager sets in Scoping, which cells rate
against in Cell Judgment) — keep the term's home context and list the others in
`shared_with`. `lint_steps` then stops reporting it as leakage there.

---

## Done When

- Every recurring noun in the event/command names has a glossary entry
- Every glossary entry has a definition, canonical phrasing, and known aliases
- `lint_steps` returns clean for a representative sample of step text from each
  bounded context
