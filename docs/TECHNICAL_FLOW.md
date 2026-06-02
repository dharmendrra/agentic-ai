# Technical Flow - For Engineers

Complete technical breakdown of what happens to your query.

---

## Step 1: User Submits Query via HTTP

**Frontend sends:**
```bash
POST http://localhost:8082/api/agent/query
Content-Type: application/json

{
  "query": "How does the PDF explain pump maintenance?"
}
```

**Backend receives and parses:**
```go
type AgentRequest struct {
  Query string `json:"query"`
}
```

---

## Step 2: Tokenization (LLM Agent)

**Who does it:** The LLM (Ollama) internally during prompt processing

**What happens:**
- Your query string is tokenized by the LLM's tokenizer
- The LLM breaks it into tokens it understands
- Example: `"How does the PDF explain..."` → `[How][does][the][PDF][explain]...`

**Why:** LLMs work with token IDs, not raw text

---

## Step 3: ReAct Loop Starts

**Location:** `main.go` → `runReAct()` function

**System prompt defines behavior:**
```go
systemPrompt := `You are a helpful assistant with access to two tools:
1. search_pdf - PRIMARY SOURCE
   - Searches the user's PDF documents
   - Use this FIRST for any factual questions

2. web_search - FALLBACK SOURCE
   - Searches the internet
   - Use only if PDF search doesn't return relevant results

...format rules...
```

**First LLM call:**
```bash
POST http://localhost:11434/api/generate
{
  "model": "gemma4:e4b",
  "prompt": "[system_prompt]\n[user_query]",
  "stream": false,
  "options": {
    "num_predict": 1024
  }
}
```

---

## Step 4: LLM Decides Which Tool

**LLM Output:**
```
Thought: The user is asking about PDF content for "pump maintenance"
Action: search_pdf
Action Input: How does the PDF explain pump maintenance?
```

**Code parses this:**
```go
actionRe := regexp.MustCompile(`Action:\s*(\w+)`)
inputRe := regexp.MustCompile(`Action Input:\s*(.+?)(?:\n|$)`)

action := actionRe.FindStringSubmatch(response)[1]     // "search_pdf"
input := inputRe.FindStringSubmatch(response)[1]       // The user query
```

---

## Step 5: PDF Search Tool Executes

**Tool:** `tools/pdf_search.go` → `Execute(query string)`

**Makes HTTP POST to YOUR PDF endpoint:**
```bash
POST http://localhost:8081/api/search
Content-Type: application/json

{
  "query": "How does the PDF explain pump maintenance?"
}
```

**Your endpoint (Omni-RAG) does:**
1. **Vectorize the query** using embedding model:
   ```bash
   POST http://localhost:11434/api/embed
   {
     "model": "gemma4:e2b",
     "input": ["How does the PDF explain pump maintenance?"]
   }
   ```
   Returns: `{"embeddings": [[0.123, -0.456, 0.789, ... (768 dims)]]}`

2. **Query Pinecone vector database:**
   ```bash
   POST https://[pinecone-host]/query
   {
     "vector": [0.123, -0.456, 0.789, ...],
     "topK": 3,
     "includeMetadata": true
   }
   ```

3. **Generate answer using retrieved chunks:**
   ```bash
   POST http://localhost:11434/api/generate
   {
     "model": "gemma4:e4b",
     "prompt": "[system_instructions]\n[chunks]\n[query]",
     "stream": true
   }
   ```

4. **Streams response back as SSE:**
   ```
   event: stage
   data: {"stage":"embedding","message":"..."}
   
   event: stage
   data: {"stage":"retrieval","message":"..."}
   
   event: stage
   data: {"stage":"generating","message":"..."}
   
   event: token
   data: {"text":"Based"}
   
   event: token
   data: {"text":" on"}
   
   ...more tokens...
   
   event: sources
   data: {"sources":[
     {
       "id":"chunk_123",
       "score":0.998,
       "source_file_id":"doc_xyz",
       "text_content":"Pump maintenance involves...",
       "chapter":3,
       "page_number":45
     },
     {...},
     {...}
   ]}
   
   event: done
   data: {"ok":true}
   ```

**Tool parses SSE stream:**
```go
scanner := bufio.NewScanner(resp.Body)

for scanner.Scan() {
  line := scanner.Text()
  
  if strings.HasPrefix(line, "event: sources") {
    // Next line: "data: {...}"
    // Extract JSON, parse sources array
    // Get text_content from each source
    chunks = append(chunks, source.TextContent)
  }
}

// Return to agent:
if len(chunks) == 0 {
  return "[PDF_EMPTY|No matching documents found in PDF]"
}
return "[PDF_SUCCESS|Found 3 chunks]\n" + strings.Join(chunks, "\n---\n")
```

---

## Step 6: LLM Sees Observation

**Tool result added to conversation history:**
```go
messages = append(messages, fmt.Sprintf("Observation: %s", observation))
```

**LLM sees:**
```
...previous thoughts...
Observation: [PDF_SUCCESS|Found 3 chunks]
Pump maintenance involves checking pressure gauge, oil level, and seals...
---
Regular maintenance reduces downtime by 40%...
---
Tools needed: wrench, gauge, sealant...
```

---

## Step 7: LLM Decides: Continue or Answer?

**LLM checks for "Final Answer":**
```go
if strings.Contains(response, "Final Answer:") {
  parts := strings.Split(response, "Final Answer:")
  state.Answer = strings.TrimSpace(parts[1])
  break  // Exit loop
}
```

**Or continues looping:**
- If no "Final Answer" found
- And step count < MAX_STEPS (10)
- Go back to Step 4 (decide next tool)

---

## Step 8: Response Sent to Frontend

**HTTP Response:**
```bash
200 OK
Content-Type: application/json

{
  "answer": "Based on the provided PDF excerpts, pump maintenance involves..."
}
```

---

## Step 9: Frontend Displays Answer

**JavaScript:**
```javascript
const data = await resp.json();
card.textEl.textContent = data.answer;
```

**User sees:**
```
Answer:
Based on the provided PDF excerpts, pump maintenance involves checking
the pressure gauge, verifying oil levels, and inspecting seals...

Sources:
✓ Source 1: Page 45, Chapter 3 (99.8% match)
✓ Source 2: Page 51, Chapter 4 (87.6% match)
✓ Source 3: Page 67, Chapter 5 (75.5% match)
```

---

## The Complete Request Chain

```
1. Frontend HTTP POST
   ↓
2. main.go: handleAgentQuery()
   ↓
3. runReAct() loop starts
   ↓
4. callOllama() → gemma4:e4b at localhost:11434
   ↓
5. LLM returns action (search_pdf or web_search)
   ↓
6. toolMgr.Execute(action, input)
   ↓
   ├─ If search_pdf:
   │  └─ HTTP POST to localhost:8081/api/search
   │     └─ Your endpoint does:
   │        ├─ POST localhost:11434/api/embed (tokenize → vectorize)
   │        ├─ POST pinecone/query (find chunks)
   │        └─ POST localhost:11434/api/generate (answer generation)
   │           └─ Returns SSE stream
   │           └─ Tool parses and returns observation
   │
   └─ If web_search:
      └─ HTTP POST to api.tavily.com/search
         └─ Returns JSON with answer
         └─ Tool extracts and returns observation
   ↓
7. Observation added to message history
   ↓
8. Loop again? Check step count, check for "Final Answer"
   ↓
9. Return answer to frontend
   ↓
10. Frontend displays
```

---

## Key Technical Details

### Embedding Model
- **Service:** Ollama
- **Model:** `gemma4:e2b`
- **Endpoint:** `http://localhost:11434/api/embed`
- **Input:** Text string
- **Output:** 768-dimensional vector (list of 768 floats)
- **Used by:** Your PDF endpoint (Omni-RAG) at ingestion + query time

### Generation Model
- **Service:** Ollama
- **Model:** `gemma4:e4b`
- **Endpoint:** `http://localhost:11434/api/generate`
- **Input:** Full prompt (system + context + question)
- **Output:** Text tokens
- **Used by:** Agent for reasoning + your endpoint for answer generation

### Vector Database
- **Service:** Pinecone
- **Stores:** Vectors (768-dim) + metadata (text, page, chapter, etc)
- **Query Type:** Cosine similarity search
- **Used by:** Your PDF endpoint (/api/search)

### SSE Stream Format (from your PDF endpoint)
```
event: stage
event: token (1 per token during LLM generation)
event: sources (once, contains all chunks)
event: done (final event)
event: error (if something fails)
```

---

## Configuration Used

From `config.json`:
```json
{
  "OLLAMA_HOST": "http://localhost:11434",           // Embedding + generation
  "OLLAMA_MODEL": "gemma4:e4b",                      // Generation model
  "TAVILY_API_KEY": "...",                           // Web search fallback
  "MAX_STEPS": 10,                                   // Loop iterations max
  "MAX_RETRIES": 3,                                  // HTTP retry attempts
  "MAX_TOKENS": 1024,                                // LLM output limit
  "SEARCH_ENDPOINT": "http://localhost:8081/api/search"  // Your PDF search
}
```

---

## Error Handling

**PDF Search Fails:**
- Retry up to MAX_RETRIES times with exponential backoff
- If all retries fail: Tool returns error observation
- LLM sees error and may try web_search instead

**Web Search Fails:**
- Graceful degradation (returns "Not available" message)
- Agent continues with PDF results if available

**Max Steps Hit:**
- Loop exits at step 10 (or configured MAX_STEPS)
- Returns best answer found so far (not an error state)

**LLM Doesn't Format Correctly:**
- If response doesn't match "Action: X" or "Final Answer:" patterns
- Tool is not called, loop continues
- Eventually hits max steps and returns partial answer

---

## This Is What Actually Happens

No magic, no black boxes. Just:
1. HTTP calls between services
2. JSON/SSE data formats
3. Vector math for similarity
4. LLM token generation
5. Parse + transform at each step

All configurable, all debuggable, all traceable in logs.
