# PDF Search Endpoint SSE Format

Real-world SSE stream format from OmniRAG Pinecone retrieval service at `POST http://localhost:8081/api/search`

## Request

```json
{
  "query": "How does the PDF explain pump maintenance?"
}
```

## SSE Stream Response Format

### Success Case (Documents Found)

```
event: stage
data: {"stage": "embedding", "message": "Generating query embedding via Ollama…"}

event: stage
data: {"stage": "retrieval", "message": "Querying Pinecone for top 3 matches…"}

event: stage
data: {"stage": "generating", "message": "Generating answer with Gemma 4…"}

event: token
data: {"text": "The pump "}

event: token
data: {"text": "maintenance "}

event: token
data: {"text": "involves..."}

event: sources
data: {"sources": [{"id": "doc123", "score": 0.95, "source_file_id": "file_xyz", "text_content": "Pump maintenance procedure: 1. Check pressure gauge...", "chapter": 3, "page_number": 42}, {"id": "doc124", "score": 0.88, "source_file_id": "file_xyz", "text_content": "Regular maintenance reduces downtime by...", "chapter": 4, "page_number": 51}]}

event: done
data: {"ok": true}
```

### Empty/Error Case (No Documents Found)

```
event: stage
data: {"stage": "embedding", "message": "Generating query embedding via Ollama…"}

event: stage
data: {"stage": "retrieval", "message": "Querying Pinecone for top 3 matches…"}

event: error
data: {"stage": "retrieval", "message": "No matching documents found in the knowledge base for this query."}
```

## Event Types

### 1. `event: stage`
Indicates progress through the pipeline.

**Fields:**
- `stage` (string): One of `embedding`, `retrieval`, `generating`
- `message` (string): Human-readable status message

**When to expect:**
- Always comes before actual processing starts
- Multiple stage events during the pipeline

### 2. `event: token`
Streamed text tokens from LLM generation.

**Fields:**
- `text` (string): A single token or partial word

**When to expect:**
- After retrieval completes
- Multiple tokens stream one at a time
- Only in success cases

### 3. `event: sources`
Retrieved document chunks from vector database.

**Fields:**
- `sources` (array): List of matching documents
  - `id` (string): Unique chunk ID
  - `score` (number): Similarity score (0-1)
  - `source_file_id` (string): Which document file
  - `text_content` (string): **The actual text chunk** (what we use)
  - `chapter` (number): Chapter number
  - `page_number` (number): Page number

**When to expect:**
- After all tokens are streamed
- Only in success cases
- Contains the source citations

### 4. `event: error`
Pipeline error or no results found.

**Fields:**
- `stage` (string): Where error occurred (`embedding`, `retrieval`, `generating`)
- `message` (string): Error description

**When to expect:**
- If no documents match the query
- If Pinecone query fails
- If LLM generation fails
- **Stream ends after error event** (no sources, no done)

### 5. `event: done`
Successful completion marker.

**Fields:**
- `ok` (boolean): Always `true`

**When to expect:**
- After sources event
- Indicates stream is complete

## How agentic-ai Tool Parses It

```go
// PDF search tool reads SSE stream line-by-line:

1. Skip `event: stage` events (informational only)
2. Skip `event: token` events (not used by agent)
3. Check for `event: error`:
   - Return [PDF_EMPTY|No matching documents found]
   - **Stop reading** (stream ends)
4. Look for `event: sources`:
   - Extract `sources` array
   - Get `text_content` from each source
   - Join with "\n---\n" separator
   - Return [PDF_SUCCESS|Found N chunks]
5. Expect `event: done` to confirm completion
```

## Empty Result Detection

**The endpoint signals "no results" in TWO ways:**

### Way 1: Error Event (Most Common)
```
event: error
data: {"stage": "retrieval", "message": "No matching documents found..."}
```

Stream **stops after error** — no sources or done event.

### Way 2: Empty Sources Array
Theoretically possible but not in current code:
```
event: sources
data: {"sources": []}
```

## Integration Notes

- **Tool returns metadata markers:**
  - `[PDF_SUCCESS|Found 3 chunks]` → LLM sees good results
  - `[PDF_EMPTY|...]` → LLM must try web_search fallback

- **Parsing strategy:**
  - Use `bufio.Scanner` to read lines
  - Parse line-by-line SSE format
  - Look for `event: error` first (means empty)
  - Extract sources array from `event: sources`

- **Error handling:**
  - Network errors: Retry with exponential backoff
  - No results: Return `[PDF_EMPTY]` marker
  - Malformed SSE: Log and continue gracefully

## Example Real-World Response

```bash
curl -X POST http://localhost:8081/api/search \
  -H "Content-Type: application/json" \
  -d '{"query": "pump maintenance"}'
```

Response (SSE stream):
```
event: stage
data: {"stage":"embedding","message":"Generating query embedding via Ollama…"}
event: stage
data: {"stage":"retrieval","message":"Querying Pinecone for top 3 matches…"}
event: stage
data: {"stage":"generating","message":"Generating answer with Gemma 4…"}
event: token
data: {"text":"The"}
event: token
data: {"text":" pump"}
event: sources
data: {"sources":[{"id":"chunk_42","score":0.95,"source_file_id":"manual_v2","text_content":"Regular pump maintenance involves checking...","chapter":3,"page_number":45}]}
event: done
data: {"ok":true}
```
