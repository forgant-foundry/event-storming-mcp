# Example Dialog: finant-api

A real session conducted with Claude Code using the `run-event-storming` playbook.
Domain: an event-sourced personal finance tracker API.
Output: 8 feature files, a glossary, and a context map committed to `specs/`.

---

## Setup

**User:** Wire up the MCP tool `~/go/bin/event-storming-mcp`, please.

*(Facilitator registered the MCP server, fetched the `run-event-storming` playbook, started a session, and configured `sync_path` to `.event-storming/finant-api.json`.)*

---

## Phase 1 — Big Picture (Domain Event Discovery)

**Facilitator:** Let's start with what this system does. What's the core domain?

**User:** This is an API responsible for an event-sourced finance tracker. Help me establish specifications for it using an event storming approach.

**Facilitator:** Let's start with what *happens* in this system — domain events in past tense. What are the main things that occur? Think about accounts, money moving, time passing.

**User:** Accounts, transactions, budgets, categories, recurring entries.

**Facilitator:** Good starting points. Accounts — what happens to them over their lifetime? What does "moving money" look like? What happens at the end of a month?

**User:** Accounts can be opened and closed. Transfers between accounts can be made — like checking to credit card, or savings to a loan. Transactions can be recorded, reversed, categorized. Transactions are given a mixed percent of fixed/variable. Budgets are not necessarily the point — it's more "projections." Combined with recurring entries like paychecks, mortgage payments, or subscriptions, the system projects out each month for the next 12 months. Categories are less important; more important is fixed/variable — entertainment is often 100% variable, a natural gas utility bill is often 100% fixed. Recurring entries are anticipated in the future for projections, and applied when they're realized/actualized in the present.

*(Facilitator captured ~20 events: AccountOpened, AccountClosed, TransferInitiated, TransactionRecorded, TransactionCategorized, TransactionVariabilitySet, ProjectionRolled, RecurringEntryScheduled, RecurringEntryActualized, etc.)*

**Facilitator:** What happens when things go wrong? And what happens at the very end of a month or year?

**User:** For recurring entries, when actualized they're immediately applied as actual, but a manual actual can later be applied — the original estimate is considered just that. Accounts can absolutely go negative — we may even see it in projections. When I mentioned "reversals" earlier — it's more that we're making corrections. Given this is event sourcing, a correction event is created on the day it's recorded but applied on a day in the past, defaulting to the original transaction's date.

For users and households: instead of users there is a *ledger* which one or more users may be "accountants." A ledger holds a collection of accounts.

Let's not worry about categories for now. The 12-month window rolls forward automatically. Event sourcing requires those monthly projections to be re-created as either an aggregate or snapshot. We even do snapshots in the past for read performance on recent event streams.

This also includes investment accounts and mortgage accounts — for those we'll occasionally need to collect actuals from elsewhere. We'll also want to apply "fantasies" to the ledger — for instance, what our investment balance would look like if we hadn't performed a sell 2 months ago.

*(Facilitator captured remaining events: AccountOverdrafted, CorrectionRecorded, TransactionReversed, LedgerCreated, AccountantAssigned, ProjectedOverdraftDetected, FantasyCreated, FantasyApplied, ExternalBalanceSynced, ProjectionSnapshotTaken, etc.)*

**Facilitator:** For fantasies on investment accounts — do we need to track individual holdings (shares), or can we operate on dollar amounts directly?

**User:** I think we can operate on dollar amounts directly and not track shares. For now we'll be manually performing the sync. I just mean that this domain service will support a user at a UI viewing a dashboard of the ledger's current state and some projection into the immediate future (1, 2, or 3 months).

---

## Phase 2 — Process Modeling (Commands, Actors, Policies)

**Facilitator:** Let's move to Phase 2. Hot spot triage first. I flagged several open questions:

*1. Should a period close be an explicit event?*
*3. Does a variability change trigger a projection recalculation?*
*4. Is a "reversal" different from a "correction"?*
*5. Is projection recalculation synchronous or async?*
*7. Does RecurringEntryActualized also record a transaction, or is that separate?*
*8. What triggers a period to be "closed" for correction?*
*9. Do we need explicit period aggregates?*

**User:** 1. One event. 3. Not per category, but yes on recurring — applied at individual transactions. 4. Both are an "adjustment"; reverse/corrected are types of adjustments. 5. Parallel, and multiple are possible. 7. Both. 8. Instead let's not have a close strength — I don't think we even need a period closure. Our snapshots/aggregates accommodate that. 9. Unnecessary.

*(Facilitator resolved hot spots and deprecated the Period aggregate. Captured ~31 commands: OpenAccount, CloseAccount, RecordTransaction, RecordCorrection, InitiateTransfer, ScheduleRecurringEntry, ActualizeRecurringEntry, ApplyRecurringEntryActual, AmendRecurringEntry, SkipRecurringEntry, PauseRecurringEntry, ResumeRecurringEntry, CancelRecurringEntry, RollMonthlyProjection, RecalculateProjection, TakeProjectionSnapshot, CreateLedger, AssignAccountant, RemoveAccountant, AddAccountToLedger, SyncExternalBalance, RecordInvestmentHolding, CreateFantasy, AmendFantasy, ApplyFantasy, RemoveFantasy.)*

**Facilitator:** Is a recurring entry just a type of transaction?

**User:** Perhaps I'm getting into too much implementation detail. You're looking for the domain-driven / behavior-driven artifacts. Let's keep it separate.

**Facilitator:** Noted — RecurringEntry stays as its own aggregate. Moving on to policies...

*(Facilitator captured policies: WhenRecurringEntryScheduled→ActualizeOnTargetDate, WhenMonthAdvances→RollProjection, WhenTransactionRecorded→RecalculateProjection, WhenCorrectionRecorded→RecalculateProjection, WhenExternalBalanceSynced→RecalculateProjection.)*

---

## Phase 3 — Software Design (Aggregates, Bounded Contexts, Ubiquitous Language)

*(Facilitator clustered events and commands into aggregates: Ledger, Account, Transaction, RecurringEntry, Projection, Fantasy.)*

**Facilitator:** Three bounded contexts emerge: **Ledger Management** (Ledger, Account), **Bookkeeping** (Transaction, RecurringEntry), **Forecasting** (Projection, Fantasy). Does that feel right?

**User:** That's great!

*(Facilitator defined ubiquitous language: Projection not "budget", Correction not "reversal", Accountant not "user", Fantasy not "scenario", Ledger not "household/account group", RecurringEntry not "standing order/subscription", Actualize not "apply/realize", Variability not "category".)*

---

## Emit

*(Facilitator performed a linking pass to wire events to their trigger commands, then emitted to `specs/`.)*

---

## Output

```
specs/
  glossary.md
  context-map.json
  bookkeeping/
    transaction.feature        (5 scenarios)
    recurring-entry.feature    (8 scenarios)
  forecasting/
    projection.feature         (4 scenarios)
    fantasy.feature            (4 scenarios)
  ledger-management/
    ledger.feature             (4 scenarios)
    account.feature            (6 scenarios)
```

31 scenarios total. The session produced a usable ubiquitous language glossary and a full set of Gherkin specs covering all aggregates across three bounded contexts.
