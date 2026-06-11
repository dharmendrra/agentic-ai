# Plan — Your Library Knowledge Base (+ web search and conversations)

Status: **locked, not yet implemented**
Scope: `agents/` module (backend + `static/` UI), plus a **native in-repo PDF search + ingestion** capability (no external `:8081`). The Go MCP server (`mcp/`) is unchanged.

> **Feature parity:** this is the Go counterpart of `~/python-projects/agentic-ai-py/PLAN_CONVERSATIONAL_RETRIEVAL.md`. Both plans expose the **same user-facing features** — Web / My Library toggles, conversation memory, `recall_history`, past-conversations sidebar, **book-title narrowing + clarify-back**, tiered context management, rolling summary, modern Lucide icons, JSON-now/SSE-later. Both now **own PDF search + ingestion in-repo** (talk to Pinecone directly). The differences are **implementation only**: (1) language; (2) the Go repo **reuses Omni-RAG's Go code** (`pinecone.go`, `embedder.go`, `ingestion/main.go`) lifted into `agents/`, while the Python repo builds the equivalent from scratch; (3) this repo keeps its dark/emerald theme, the Python repo uses a python.org theme. If a feature changes in one plan, change it in the other.

---

## 1. Goal

Turn the current single-shot ReAct agent into a **conversation-style assistant** where the user explicitly chooses where answers come from, via two toggle buttons in the search interface:

| Button | Effect when ON |
|---|---|
| 🌐 **Web** | Model may use `web_search` (Tavily) |
| 📚 **My Library** | Model may use internal sources: `search_pdf` (Pinecone book vectors) **and** the Mongo MCP CRUD tools |

Behaviour by combination:

| Web | My Library | Behaviour |
|---|---|---|
| off | off | **No retrieval tools.** Model answers from its own training only. If it doesn't know, it suggests enabling 🌐 Web. |
| off | on | Internal only — Pinecone PDF search + Mongo MCP. No web. |
| on | off | Web only — Tavily. No internal sources. |
| on | on | Use **both** internal + web, merge into the answer. |

Plus:

- **Conversation memory** — the model references earlier turns ("what did I ask first", "summarize our chat").
- **Past-conversations sidebar** (right side) — click to reopen any prior conversation.
- **Clarify-back** — when My Library is on but the topic can't be pinned to a specific book (thousands of books dilute vector accuracy), the model asks *"do you mean *Game of Thrones* or {X}?"* instead of guessing.
- **Everyday queries** — work fine with both toggles off (plain LLM).

---

## 2. Key architectural decisions (already settled)

1. **Vector search is Pinecone, queried DIRECTLY from `agents/` (native, no `:8081`).** We lift the reusable Go parts from Omni-RAG — `pinecone-rag/retrieval/pinecone.go` (Pinecone querier) and `embedder.go` (Ollama embeddings) — into `agents/tools/`, and `pinecone-rag/ingestion/main.go` for the ingest pipeline. agentic-ai now owns embedding + query + ingestion and holds the Pinecone key. MongoDB does **not** do vector search. (Supersedes the earlier design where `search_pdf` called the external `:8081` endpoint.)
2. **"My Library" = internal sources** = `search_pdf` (Pinecone) + Mongo MCP CRUD. (Honors the original "Both PDF + Mongo" choice.)
3. **Conversation history is stored in MongoDB via a direct Go driver in `agents/` — NOT via the MCP tool.** Persisting history is deterministic application plumbing; the MCP tool is the *model's* tool and includes destructive ops (`delete_document`). The two are kept on separate layers, same Mongo instance.
4. **Context management = token-budgeted tiered memory** (see §6), not "resend everything." Local Ollama has a small context window, so we budget.
5. **Deep recall = a dedicated read-only `recall_history` tool** scoped to the current conversation — never the general MCP CRUD.
6. **Response mode = simple JSON for now.** SSE streaming is deferred (see `todo.md`).
7. **Book-title narrowing via a Mongo `books` catalog + clarify-back** — ingestion writes `book_title` into Pinecone metadata AND a Mongo `books` catalog (title/aliases ↔ `source_file_id`). At query time the agent resolves a spoken title ("go in action") against the catalog → one match filters the vector search to that book; ambiguous → `Clarification:`; none → search all. See §2A.

---

## 2A. Native PDF search, ingestion & book catalog (reuses Omni-RAG)

Replaces the `:8081` dependency. We lift Omni-RAG's Go code into `agents/`.

### Reused/ported files → into `agents/tools/` (or `agents/pdf/`)
- `pinecone.go` — `PineconeQuerier`: query top-K with `include_metadata`, plus a metadata **filter** (`source_file_id $in [...]`) for book narrowing.
- `embedder.go` — Ollama embeddings (`nomic-embed-text`, 768-dim, cosine).
- `ingestion/main.go` — extract → chunk → embed → upsert pipeline.

### Config additions (`config.go` + `config.json`)
`PINECONE_API_KEY`, `PINECONE_INDEX` (`omni-rag-pdfs`), `PINECONE_HOST`, `PINECONE_NAMESPACE`, `EMBEDDING_MODEL` (`nomic-embed-text`), `VECTOR_DIMENSION` (768), `RETRIEVAL_TOP_K` (3), `CHUNK_SIZE`, `CHUNK_OVERLAP`. (Remove `SEARCH_ENDPOINT`.)

### Pinecone metadata schema
The existing index uses `text_content`, `source_file_id`, `page_number`, `chapter`. New ingests **add `book_title`**. (`source_file_id` is the stable per-book id used for filtering.)

### `books` catalog (MongoDB, direct Go driver)
```jsonc
{ "_id": "<source_file_id>", "title": "Go in Action", "aliases": ["go in action","gia"],
  "chunk_count": 412, "created_at": "<ts>" }
```
Written/updated at ingestion. Existing title-less books are handled by **re-ingesting** to populate titles (user-chosen).

### `search_pdf` tool (rewritten, native)
- Input: `{ "query": string, "book"?: string }` (the LLM passes `book` when the user names one).
- Flow: if `book` given → resolve against the `books` catalog (case-insensitive / alias / fuzzy) →
  - **1 match** → embed query → Pinecone query filtered to that `source_file_id` → chunks.
  - **multiple matches** → return a `NEEDS_CLARIFICATION` marker listing candidates → agent emits `Clarification:`.
  - **0 matches** → fall back to unfiltered search (or clarify).
  - no `book` → unfiltered search across all books.
- Returns chunks tagged with `book_title` + `page_number` for source tags ("from My Library · Go in Action").

### Ingestion entrypoint
A `POST /api/ingest` (multipart) handler **or** a CLI subcommand: upload PDF → extract (pdf lib) → chunk → embed → upsert to Pinecone with `{text_content, book_title, source_file_id, page_number, chunk_index}` → upsert the `books` catalog entry. (Optional small upload UI later.)

---

## 3. Data model (MongoDB, accessed directly from `agents/`)

Reuse the existing Mongo instance. New collections (DB name configurable, default `agentic_mcps` or a dedicated `agentic_chat`):

### `conversations`
```jsonc
{
  "_id": "<uuid>",
  "title": "Binary trees and balancing",   // generated from first user message
  "created_at": "<ts>",
  "updated_at": "<ts>",
  "summary": "",                            // rolling summary of older turns
  "summary_upto_seq": 0                     // messages <= this seq are folded into summary
}
```

### `messages`
```jsonc
{
  "_id": "<uuid>",
  "conversation_id": "<uuid>",
  "seq": 1,                                 // monotonic per conversation
  "role": "user" | "assistant",
  "content": "...",                         // FINAL text only — never raw tool observations
  "sources": ["pdf", "web"],               // which sources fed this answer (assistant only, optional)
  "created_at": "<ts>"
}
```

> Raw tool observations (Pinecone chunks, web text) are **never** stored in `messages.content`. They live only inside one ReAct loop and are discarded after the answer is formed.

---

## 4. Backend changes (`agents/`)

### 4.1 `config.go`
Add:
- `MONGO_URI` (e.g. `mongodb://localhost:27017`)
- `MONGO_DB` (default `agentic_mcps`)
- `HISTORY_TURNS` — K recent turns kept verbatim (default 6)
- `HISTORY_TOKEN_BUDGET` — approx token budget for replayed history (default ~6000)
- `SUMMARY_TRIGGER_TURNS` — when older-than-K turns exceed this, summarize (default 8)

### 4.2 New file `store.go` (or `store/` package) — conversation persistence
Direct Mongo driver (reuse the `mongo.Connect` pattern from `mcp/db/client.go`). Functions:
- `CreateConversation(title) (id, error)`
- `AppendMessage(convID, role, content, sources) (seq, error)`
- `GetRecentMessages(convID, k) ([]Message, error)`
- `GetMessagesAfterSeq(convID, seq) / GetMessagesUpToSeq(...)` — for summary windowing
- `GetAllUserMessages(convID) ([]Message, error)` — for `recall_history`
- `GetFirstNUserMessages(convID, n)` / `SearchMessages(convID, query)`
- `GetConversation(convID)` / `UpdateSummary(convID, summary, uptoSeq)`
- `ListConversations() ([]Conversation, error)` — for sidebar
- `GetConversationMessages(convID) ([]Message, error)` — for reopening

### 4.3 `types.go`
```go
type AgentRequest struct {
    Query          string `json:"query"`
    ConversationID string `json:"conversation_id"` // empty => new conversation
    UseWeb         bool   `json:"use_web"`
    UseLibrary     bool   `json:"use_library"`
}
type AgentResponse struct {
    Answer         string `json:"answer"`
    ConversationID string `json:"conversation_id"`
    Title          string `json:"title"`
    NeedsClarification bool `json:"needs_clarification,omitempty"`
}
```

### 4.4 `recall_history` tool (new, in `agents/tools/`)
- Read-only. Scoped to the current `conversation_id` (injected by the backend, not chosen by the model).
- Input: `{ "mode": "all" | "first_n" | "search", "n"?: int, "query"?: string }`
- Returns matching past **user** messages (and answers when asked). Cannot write/delete.
- Built **per request** (closes over the store + current convID), since it's conversation-scoped.

### 4.5 `react.go` — the core rework
New signature, e.g. `runReAct(ctx, convID, query string, useWeb, useLibrary bool) (answer, needsClarify, error)`.

Steps each turn:
1. **Build the active tool set per request** (don't use the global manager directly):
   - Always include `recall_history` (memory is not a knowledge source).
   - If `useLibrary`: add `search_pdf` + `mcp`.
   - If `useWeb`: add `web_search`.
2. **Assemble the prompt within the token budget** (§6): system prompt + rolling `summary` + last K turns (verbatim) + current user query.
3. **Run the ReAct loop** (existing logic), but the loop's internal `messages`/observations are transient — only the **Final Answer** is returned and persisted.
4. **Clarify-back**: the system prompt allows the model to emit `Clarification: ...` (treated like `Final Answer:` — exit loop, return text, set `needs_clarification=true`). No special UI handling needed — it's just the assistant's turn; the user replies next turn.
5. **System-prompt branches** by toggle state:
   - No tools: "Answer from your own knowledge. If you don't know, say so and suggest the user enable 🌐 Web search."
   - My Library on: include disambiguation instruction — "If a topic can't be confidently pinned to a specific book, ask the user to clarify which book rather than guessing."

### 4.6 `server.go` — handler + new endpoints
- `POST /api/agent/query`:
  1. Parse new fields. If `conversation_id` empty → `CreateConversation` (title from a short LLM/heuristic summary of the first query).
  2. `AppendMessage(user)`.
  3. `runReAct(...)`.
  4. `AppendMessage(assistant, sources)`.
  5. Maybe trigger rolling summary (§6).
  6. Return `{answer, conversation_id, title, needs_clarification}`.
- `GET /api/conversations` → list for sidebar (id, title, updated_at).
- `GET /api/conversations/{id}` → full message list to reopen.
- `DELETE /api/conversations/{id}` (optional) → delete a conversation.

---

## 5. Frontend changes (`agents/static/`)

Convert the single-shot UI into a **chat layout** while keeping the existing dark/emerald theme.

- **Layout:** main chat thread (left/center) + **"Past conversations" sidebar (right)**. Thread **appends** turns (stop clearing `results` each query).
- **Toggle buttons** next to the input: 🌐 **Web** and 📚 **My Library**. Visual active/inactive state. **Default: both off.** Send `use_web` / `use_library` with each request.
- **Conversation state in JS:** hold `conversationId`; send it with every query; set it from the response. **"New chat"** button clears it.
- **Sidebar:** fetch `GET /api/conversations` on load; render list; clicking an item loads `GET /api/conversations/{id}` into the thread and sets `conversationId`.
- **Clarification turns** render as a normal assistant bubble (optionally a subtle "needs your input" hint).
- Keep the reasoning-steps area; wire to real steps later when SSE lands.
- **UI copy:** toggle reads **`📚 My Library`** (paired with **`🌐 Web`**); loading state "Searching My Library…"; PDF answers show a source tag like **"from My Library · <book title>"**.

### 5.1 Icons — modern SVG icon set (no emoji)
**No emoji in the actual UI.** Use a modern, consistent **stroke-based SVG icon set — [Lucide](https://lucide.dev)** (MIT, the modern successor to Feather; the current UI already uses Feather-style inline SVGs in `static/`, so this is a natural upgrade). The emoji in this doc (🌐, 📚, …) are only shorthand for the Lucide icons below.

- **Delivery:** inline `<svg>` (themeable via `currentColor`) sized **16–18px**, stroke-width **2.2** to match the existing buttons. Icons inherit the emerald accent / slate palette per context.
- **Mapping:**

| UI element | Lucide icon |
|---|---|
| Web toggle | `globe` |
| My Library toggle | `library` (alt: `book-open`) |
| Ask Agent (primary button) | `arrow-up` (alt: `search`) |
| New chat | `square-pen` (alt: `plus`) |
| Past-conversation item | `message-square` |
| Delete conversation | `trash-2` |
| Clarification turn | `help-circle` |
| Web source citation | `external-link` |
| Per-book source tag | `book` |
| Loading / reasoning | `loader-circle` (animate-spin) |
| Error banner | `circle-alert` |

> Keep stroke style uniform — replace the existing hand-rolled SVGs with the matching Lucide icons; don't mix Lucide with emoji glyphs.

---

## 6. Context management (the important part)

The current `react.go` concatenates **every raw observation** into one growing string — fine for one shot, fatal for multi-turn. New strategy: **tiered, token-budgeted memory.**

| Part | Size | Strategy |
|---|---|---|
| **User queries** | tiny | Recent ones kept verbatim; older ones reachable on demand via `recall_history`. |
| **Assistant answers** | medium | Last K turns verbatim; older folded into rolling `summary`. |
| **Tool observations** (Pinecone/web) | huge | **Never persisted, never replayed.** Used within one turn, then dropped. Re-retrieved next turn if needed. ← biggest win. |

Per-turn prompt:
```
system prompt (branched by toggles)
+ rolling summary        (compressed older turns, from Mongo)
+ last K turns verbatim  (user + assistant)
+ current user query
+ [this turn's tool observations — transient, discarded after answer]
```

Mechanism:
1. **Token budget** via a cheap `len/4` heuristic (no tokenizer dependency). Fill newest-first until budget hit.
2. **Rolling summary** — when older-than-K turns exceed `SUMMARY_TRIGGER_TURNS`, one LLM call compresses them into `conversations.summary`; set `summary_upto_seq`. Recent turns stay verbatim.
3. **Full history always in Mongo** — we persist everything, send only the budgeted slice.
4. **Exact deep recall** — "what did I ask first" → `recall_history` reads the stored user messages directly (exact, not paraphrased from a summary).

### How this compares to ChatGPT/Claude
They mostly **resend the entire transcript every turn** (their context windows are huge) and only summarize when a chat overflows. We can't assume a huge window (local Ollama), so we budget + summarize earlier and add `recall_history` to keep prompts cheap. Cross-chat "Memory" (ChatGPT) is a separate fact-injection system and is **out of scope**.

---

## 7. Tool/permission safety

- `recall_history` is **read-only** and **conversation-scoped** (backend injects convID; model can't target another conversation or delete).
- The general MCP CRUD (`delete_document`, etc.) is only exposed when **My Library** is on, and is never used for conversation history.
- Conversation collections are written **only** by the backend's direct driver, never by the model.

---

## 8. Documentation (per repo standards)

When implemented, add/update under `agents/docs/`:
- `CONVERSATION_STORAGE_FORMAT.md` — `conversations`/`messages` schemas + example docs.
- `RECALL_HISTORY_TOOL.md` — tool input/output examples + edge cases.
- Update `DFD.md` and `TOOLS_ARCHITECTURE.md` for the new flow and tool gating.
- New API endpoints (`/api/conversations`, `/api/conversations/{id}`) documented with example payloads.

---

## 9. Out of scope / future

- **SSE streaming** of answers + reasoning steps → tracked in `todo.md`.
- **MongoDB Atlas `$vectorSearch`** (book embeddings in Mongo) — not needed; Pinecone covers semantic search.
- **Conversation-RAG** (embedding past messages for retrieval) — `recall_history` + summary is sufficient.
- **Cross-conversation memory** (ChatGPT-style global facts).
- **Auth / multi-user** — single-user assumed.

---

## 10. Implementation order

1. `config.go` + `store.go` (Mongo conversation CRUD) — foundation.
2. `types.go` request/response fields.
3. `recall_history` tool + per-request tool-set assembly.
4. `react.go` rework (tool gating, memory assembly, clarify-back, budget).
5. `server.go` handler + conversation endpoints.
6. Rolling summary logic.
7. Frontend: toggles, chat thread, sidebar, conversation state.
8. Docs under `agents/docs/`.
