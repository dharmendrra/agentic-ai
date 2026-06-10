# Agent API Endpoints

Backend HTTP API for `agents/` (default port `8082`). JSON request/response; SSE
streaming is deferred (see `todo.md`). Source of truth: PLAN §4.6.

---

## `POST /api/agent/query`

Run one conversational turn.

### Request

```json
{
  "query": "How do binary trees stay balanced?",
  "conversation_id": "",      // empty => start a new conversation
  "use_web": false,           // enable web_search (Tavily)
  "use_library": true         // enable internal: search_pdf (Pinecone) + mcp (MongoDB)
}
```

| Field | Type | Notes |
|---|---|---|
| `query` | string | Required. |
| `conversation_id` | string | Empty → backend creates a new conversation and returns its id. |
| `use_web` | bool | When `true`, `web_search` is in the active tool set. |
| `use_library` | bool | When `true`, `search_pdf` + `mcp` are in the active tool set. |

Toggle matrix (PLAN §1): both off = model's own knowledge only (suggests enabling Web
if it doesn't know); library only = internal; web only = Tavily; both = merge.

### Response

```json
{
  "answer": "A red-black tree maintains balance by ...",
  "conversation_id": "0d6f8a3e-2c1b-4f9a-9b8a-1f2e3d4c5b6a",
  "title": "How do binary trees stay balanced?",
  "sources": ["pdf"],
  "needs_clarification": false
}
```

| Field | Type | Notes |
|---|---|---|
| `answer` | string | Final answer (or clarification question text). |
| `conversation_id` | string | Echoes existing id, or the newly created one. |
| `title` | string | Conversation title (set on first turn). |
| `sources` | []string | Which sources actually fed the answer: `pdf`, `web`, `mongo`. Omitted when none. |
| `needs_clarification` | bool | `true` when the model emitted `Clarification:` (e.g. ambiguous book) instead of a final answer. |

### Errors
- `400` empty `query` or bad JSON.
- `500` LLM/ReAct failure or conversation create failure.

### Notes
- If Mongo is unavailable, the turn still runs (no memory): `conversation_id`/`title`
  may be empty and `recall_history` is not offered.
- Raw tool observations are never persisted; only the final answer is stored.

---

## `GET /api/conversations`

List conversations for the sidebar, newest first.

### Response

```json
[
  {
    "id": "0d6f8a3e-...",
    "title": "How do binary trees stay balanced?",
    "created_at": "2026-06-10T12:00:00Z",
    "updated_at": "2026-06-10T12:03:21Z",
    "summary": "",
    "summary_upto_seq": 0
  }
]
```

Returns `[]` when Mongo is unavailable or there are no conversations.

---

## `GET /api/conversations/{id}`

Full conversation for reopening in the thread.

### Response

```json
{
  "conversation": {
    "id": "0d6f8a3e-...",
    "title": "How do binary trees stay balanced?",
    "created_at": "2026-06-10T12:00:00Z",
    "updated_at": "2026-06-10T12:03:21Z",
    "summary": "",
    "summary_upto_seq": 0
  },
  "messages": [
    { "id": "m1", "conversation_id": "0d6f8a3e-...", "seq": 1, "role": "user",
      "content": "How do binary trees stay balanced?", "created_at": "..." },
    { "id": "m2", "conversation_id": "0d6f8a3e-...", "seq": 2, "role": "assistant",
      "content": "A red-black tree ...", "sources": ["pdf"], "created_at": "..." }
  ]
}
```

### Errors
- `400` missing id.
- `404` conversation not found.
- `503` Mongo unavailable.

---

## `DELETE /api/conversations/{id}`

Delete a conversation and all its messages. `204 No Content` on success; `503` when
Mongo is unavailable.

---

## curl smoke tests

```bash
# New conversation, library on
curl -s localhost:8082/api/agent/query -H 'Content-Type: application/json' \
  -d '{"query":"hello","use_web":false,"use_library":false}'

# Continue it (use the returned conversation_id)
curl -s localhost:8082/api/agent/query -H 'Content-Type: application/json' \
  -d '{"query":"what did I ask first?","conversation_id":"<ID>"}'

curl -s localhost:8082/api/conversations
curl -s localhost:8082/api/conversations/<ID>
curl -s -X DELETE localhost:8082/api/conversations/<ID>
```
