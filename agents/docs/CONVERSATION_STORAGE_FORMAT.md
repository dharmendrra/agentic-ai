# Conversation Storage Format

How `agents/` persists conversation memory in MongoDB.

> **Direct driver, not the MCP tool.** History is written **only** by the backend's
> direct Mongo driver (`agents/store.go`), never by the model. The model's MCP CRUD
> tool (which includes `delete_document`) operates on a separate layer, same Mongo
> instance. See PLAN §2.3 / §7.

- **Driver:** `go.mongodb.org/mongo-driver` (reuses the `mcp/db/client.go` connect pattern).
- **Connection:** `MONGO_URI` (default `mongodb://localhost:27017`), database `MONGO_DB` (default `agentic_mcps`).
- **Collections:** `conversations`, `messages`.
- **Degrades gracefully:** if Mongo is unreachable, `store` is `nil`; the agent still
  answers but without memory, and `/api/conversations*` return empty / `503`.

---

## Collection: `conversations`

One document per chat thread.

```jsonc
{
  "_id": "0d6f8a3e-2c1b-4f9a-9b8a-1f2e3d4c5b6a", // uuid (string)
  "title": "How do binary trees balance?",        // from first user message, max 60 chars
  "created_at": "2026-06-10T12:00:00Z",            // BSON datetime (UTC)
  "updated_at": "2026-06-10T12:03:21Z",            // bumped on every append / summary
  "summary": "",                                   // rolling summary of folded older turns
  "summary_upto_seq": 0                            // messages with seq <= this are folded into summary
}
```

| Field | Type | Notes |
|---|---|---|
| `_id` | string (uuid) | Conversation id, returned to the client as `conversation_id`. |
| `title` | string | Derived by `makeTitle()` from the first user query (collapsed whitespace, truncated to 60 chars + `…`). |
| `created_at` / `updated_at` | datetime | UTC. `updated_at` drives sidebar ordering (newest first). |
| `summary` | string | Rolling LLM summary of older turns; empty until the trigger fires. |
| `summary_upto_seq` | int | Highest message `seq` already folded into `summary`. Newer turns stay verbatim. |

## Collection: `messages`

One document per turn. `content` is **FINAL text only** — raw tool observations
(Pinecone chunks, web text) are **never** stored here (PLAN §3, §6).

```jsonc
{
  "_id": "8c2b...-uuid",
  "conversation_id": "0d6f8a3e-...",   // FK to conversations._id
  "seq": 3,                            // monotonic per conversation, starts at 1
  "role": "assistant",                 // "user" | "assistant"
  "content": "A red-black tree keeps balance by ...",
  "sources": ["pdf", "web"],           // assistant only, optional: which sources fed the answer
  "created_at": "2026-06-10T12:03:21Z"
}
```

| Field | Type | Notes |
|---|---|---|
| `_id` | string (uuid) | Message id. |
| `conversation_id` | string | References `conversations._id`. |
| `seq` | int | Monotonic per conversation (`max(seq)+1` on append). Compound index `{conversation_id:1, seq:1}`. |
| `role` | string | `user` or `assistant`. |
| `content` | string | Final answer/question text. Clarification turns are stored as `assistant`. |
| `sources` | []string | Optional, assistant only. Values: `pdf`, `web`, `mongo`. Omitted when none. |
| `created_at` | datetime | UTC. |

---

## Store API (`agents/store.go`)

| Function | Purpose |
|---|---|
| `CreateConversation(title)` | Insert a new conversation, returns uuid. |
| `AppendMessage(convID, role, content, sources)` | Insert a turn at the next `seq`, bumps `updated_at`. |
| `GetRecentMessages(convID, k)` | Last `k` messages, ascending — for budgeted replay. |
| `GetMessagesUpToSeq(convID, after, upto)` | Window for the rolling summary. |
| `GetAllUserMessages` / `GetFirstNUserMessages` / `SearchMessages` | Back the `recall_history` tool. |
| `GetConversation` / `UpdateSummary` | Read/update the rolling summary. |
| `ListConversations` | Sidebar list (newest first). |
| `GetConversationMessages` | Full thread for reopening. |
| `DeleteConversation` | Remove a conversation + its messages. |

## Context windowing (PLAN §6)

- Per turn the prompt = system prompt (toggle-branched) + rolling `summary` +
  last `HISTORY_TURNS` turns verbatim + current query, filled newest-first until
  `HISTORY_TOKEN_BUDGET` (`len/4` token heuristic).
- When older-than-K turns exceed `SUMMARY_TRIGGER_TURNS`, one LLM call folds them
  into `summary` and advances `summary_upto_seq`.

## Example: a 2-turn conversation

`conversations`:
```json
{ "_id": "c1", "title": "Pump maintenance", "summary": "", "summary_upto_seq": 0,
  "created_at": "2026-06-10T12:00:00Z", "updated_at": "2026-06-10T12:01:10Z" }
```
`messages`:
```json
{ "_id": "m1", "conversation_id": "c1", "seq": 1, "role": "user", "content": "How do I maintain the pump?" }
{ "_id": "m2", "conversation_id": "c1", "seq": 2, "role": "assistant", "content": "Check the pressure gauge weekly...", "sources": ["pdf"] }
```
