# Example Dialog: Event Storming a Chat App

This is an example conversation between an LLM facilitator and a domain expert,
showing how the MCP tools are called during a live event storming session.

---

## Session Setup

**LLM:** Let's run an event storming session on your chat app. I'll start by capturing
everything, then we'll cluster it into a model. In a sentence or two — what's the core domain?

**Expert:** It's a team messaging app. Users send messages in channels, can DM each other,
and get push notifications when mentioned.

```json
// start_session
{ "name": "Team messaging app",
  "description": "Real-time team messaging with channels, DMs, and mention notifications" }
// → { "session_id": "ses_4a1b2c3d", ... }
```

---

## Phase 1 — Big Picture Event Discovery

**LLM:** Phase 1 — just throw events at me. Things that *happened* in the system, past
tense. Don't worry about order yet.

**Expert:** Message was sent. User joined a channel. User got mentioned. Notification was delivered.

```json
// add_event x4
{ "session_id": "ses_4a1b2c3d", "name": "MessageSent" }        // → evt_001
{ "session_id": "ses_4a1b2c3d", "name": "UserJoinedChannel" }   // → evt_002
{ "session_id": "ses_4a1b2c3d", "name": "UserMentioned" }       // → evt_003
{ "session_id": "ses_4a1b2c3d", "name": "NotificationDelivered" } // → evt_004
```

**LLM:** Good. Keep going — what else happens?

**Expert:** Message was read. User went offline. Channel was created. User was removed
from a channel. Message was deleted.

```json
{ "session_id": "ses_4a1b2c3d", "name": "MessageRead" }             // → evt_005
{ "session_id": "ses_4a1b2c3d", "name": "UserWentOffline" }         // → evt_006
{ "session_id": "ses_4a1b2c3d", "name": "ChannelCreated" }          // → evt_007
{ "session_id": "ses_4a1b2c3d", "name": "UserRemovedFromChannel" }  // → evt_008
{ "session_id": "ses_4a1b2c3d", "name": "MessageDeleted" }          // → evt_009
```

**LLM:** What about the notification side — any events there beyond delivery?

**Expert:** Notification was sent, and sometimes a notification fails to deliver if the
device is offline.

```json
{ "session_id": "ses_4a1b2c3d", "name": "NotificationSent" }   // → evt_010
{ "session_id": "ses_4a1b2c3d", "name": "NotificationFailed" } // → evt_011
```

---

## Phase 2 — Process Modeling

### Actors

**LLM:** Phase 2 — let's add the actors and commands. Who does things in this system?

**Expert:** Regular users do most things. Admins can remove people from channels. The push
notification service is external — Firebase or APNs.

```json
// add_actor x3
{ "session_id": "ses_4a1b2c3d", "name": "User",                     "type": "person" }          // → act_001
{ "session_id": "ses_4a1b2c3d", "name": "Admin",                    "type": "role" }             // → act_002
{ "session_id": "ses_4a1b2c3d", "name": "Push Notification Service","type": "external_system" }  // → act_003
```

### Commands

**LLM:** Now commands — the actions that cause those events. What does a User do?

**Expert:** Send a message, join a channel, leave a channel, mark a message as read,
delete their own message.

```json
// add_command x5
{ "session_id": "ses_4a1b2c3d", "name": "SendMessage",     "actor_id": "act_001",
  "preconditions": ["user is a member of the channel"] }                            // → cmd_001
{ "session_id": "ses_4a1b2c3d", "name": "JoinChannel",     "actor_id": "act_001" } // → cmd_002
{ "session_id": "ses_4a1b2c3d", "name": "LeaveChannel",    "actor_id": "act_001" } // → cmd_003
{ "session_id": "ses_4a1b2c3d", "name": "MarkMessageRead", "actor_id": "act_001" } // → cmd_004
{ "session_id": "ses_4a1b2c3d", "name": "DeleteMessage",   "actor_id": "act_001",
  "preconditions": ["user is the author"] }                                          // → cmd_005
```

**LLM:** What can an Admin do beyond what a regular User can?

**Expert:** Create channels, remove users from channels.

```json
{ "session_id": "ses_4a1b2c3d", "name": "CreateChannel",         "actor_id": "act_002" } // → cmd_006
{ "session_id": "ses_4a1b2c3d", "name": "RemoveUserFromChannel", "actor_id": "act_002" } // → cmd_007
```

### Policies

**LLM:** I see `UserMentioned` and `NotificationSent` — how does that flow work? Is it a
policy? "Whenever a message is sent and contains a mention, then send a notification"?

**Expert:** Exactly. It's automatic. The system detects mentions and fires off notifications.

**LLM:** I don't have a "SendNotification" command yet — is that issued by the system or
the Push Notification Service?

**Expert:** The system issues it; the Push Notification Service executes it externally.

```json
// add_command
{ "session_id": "ses_4a1b2c3d", "name": "SendNotification", "actor_id": "act_003" } // → cmd_008

// add_policy
{ "session_id": "ses_4a1b2c3d",
  "name": "Notify on mention",
  "trigger_event_id": "evt_001",
  "resulting_command_id": "cmd_008" } // → pol_001
```

### Hot Spots

**LLM:** One open question: `NotificationFailed` — is there a retry policy, or does that
get surfaced to the user as "delivered to X of Y devices"?

**Expert:** Honestly, we haven't decided yet. Firebase handles retries but we're not sure
what we surface.

```json
// add_hot_spot
{ "session_id": "ses_4a1b2c3d",
  "question": "NotificationFailed: do we retry? Do we surface per-device delivery status to the sender?" }
// → hs_001
```

---

## Phase 3 — Software Design

### Aggregates

**LLM:** Let's cluster. I see three natural aggregates: **Message** (sending, reading,
deleting), **Channel** (membership, creation), and **User** (presence). Does that match
your mental model?

**Expert:** Yes. Though I'd call the third one just "User" — presence is part of the
user's state.

```json
// cluster_aggregate x3
{ "session_id": "ses_4a1b2c3d", "name": "Message",
  "event_ids":   ["evt_001", "evt_005", "evt_009"],
  "command_ids": ["cmd_001", "cmd_004", "cmd_005"] } // → agg_001

{ "session_id": "ses_4a1b2c3d", "name": "Channel",
  "event_ids":   ["evt_002", "evt_007", "evt_008"],
  "command_ids": ["cmd_002", "cmd_003", "cmd_006", "cmd_007"] } // → agg_002

{ "session_id": "ses_4a1b2c3d", "name": "User",
  "event_ids":   ["evt_006"],
  "command_ids": [] } // → agg_003
```

**LLM:** Where does notification live? `NotificationSent`, `NotificationDelivered`,
`NotificationFailed` — is that a fourth aggregate or a separate service entirely?

**Expert:** Separate. It's a bounded context on its own. We might replace Firebase with
something else later.

```json
{ "session_id": "ses_4a1b2c3d", "name": "Notification",
  "event_ids":   ["evt_003", "evt_004", "evt_010", "evt_011"],
  "command_ids": ["cmd_008"] } // → agg_004
```

### Ubiquitous Language

**LLM:** Let's lock down the language before we propose contexts. What's a "channel"
precisely — does it cover DMs?

**Expert:** A channel is any named conversation space. DMs are just private channels with
exactly two members. We call them the same thing internally.

```json
// define_term x3
{ "session_id": "ses_4a1b2c3d",
  "term": "Channel",
  "definition": "A named conversation space. DMs are private channels with exactly two members.",
  "canonical_phrasing": "the channel" }

{ "session_id": "ses_4a1b2c3d",
  "term": "Message",
  "definition": "A text payload sent by a user into a channel at a point in time.",
  "canonical_phrasing": "the message" }

{ "session_id": "ses_4a1b2c3d",
  "term": "Mention",
  "definition": "An explicit @reference to a user within a message body that triggers notification.",
  "canonical_phrasing": "the mention" }
```

### Bounded Contexts

**LLM:** Now bounded contexts. I see two: **Messaging** (Message + Channel + User) and
**Notifications** (Notification). Sound right?

**Expert:** Yes. Presence might eventually split off but for now it lives in Messaging.

```json
// propose_bounded_context x2
{ "session_id": "ses_4a1b2c3d",
  "name": "Messaging",
  "description": "Owns message lifecycle, channel membership, and user presence",
  "aggregate_ids": ["agg_001", "agg_002", "agg_003"] } // → ctx_001

{ "session_id": "ses_4a1b2c3d",
  "name": "Notifications",
  "description": "Owns fan-out of mention alerts to external push providers",
  "aggregate_ids": ["agg_004"] } // → ctx_002
```

---

## Summary

**LLM:** We have a solid model. Here's what we captured:

| | |
|---|---|
| Domain events | 11 |
| Commands | 8 |
| Actors | 3 (1 person, 1 role, 1 external system) |
| Policies | 1 |
| Aggregates | 4 |
| Bounded contexts | 2 |
| Glossary terms | 3 |
| Open hot spots | 1 |

**Open question (hs_001):** NotificationFailed — retry behaviour and per-device delivery
visibility are unresolved. Flag for the next sprint planning session.

**Key language decision:** DMs are private Channels with two members, not a separate
concept. This collapses a potential split across two aggregates.

Ready to emit Gherkin specs? Use `emit_gherkin` to generate `.feature` files per
aggregate, tagged `@context:Messaging @aggregate:Message`, with scenarios derived from
the command → event flows captured here.
