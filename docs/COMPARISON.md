# OmniRAG Search vs Agent: Comparison

## Side-by-Side Comparison

### Existing PDF Search (`POST /api/search`)

```
User Question: "How does the PDF explain pump maintenance?"

┌──────────────────────┐
│  Embed Query         │
└──────────┬───────────┘
           ▼
┌──────────────────────┐
│  Vector Search       │
│  (Pinecone/Vector DB)│
└──────────┬───────────┘
           ▼
┌──────────────────────┐
│  Return Top-3 Chunks │
│  (with similarity %) │
└──────────────────────┘

Response (direct):
{
  "chunks": [
    {"text": "Pump maintenance requires...", "score": 0.92},
    {"text": "Step 1: Inspect seals...", "score": 0.88},
    {"text": "Chapter 4 covers maintenance...", "score": 0.85}
  ]
}

✓ Fast
✓ Simple
✓ Low latency (< 1 sec)
✗ No reasoning
✗ Raw chunks only
✗ No cross-reference
✗ No synthesis
```

### New Agent (`POST /api/agent/query`)

```
User Question: "How does the PDF explain pump maintenance?"

┌──────────────────────┐
│  System Prompt       │
│  (PDF-first strategy)│
└──────────┬───────────┘
           ▼
┌──────────────────────────────────────┐
│ ReAct Loop (Step 1)                  │
│ Thought: I should search the PDF     │
│ Action: search_pdf                   │
│ Input: "pump maintenance"            │
└──────────┬───────────────────────────┘
           ▼
┌──────────────────────────────────────┐
│ Tool Execution: search_pdf           │
│ → Calls /api/search                  │
│ ← Returns top chunks                 │
└──────────┬───────────────────────────┘
           ▼
┌──────────────────────────────────────┐
│ ReAct Loop (Observation + Analysis)  │
│ Observation: [PDF results from tool] │
│ [LLM synthesizes answer]             │
│ Final Answer: [synthesized response] │
└──────────────────────────────────────┘

Response (synthesized):
{
  "answer": "The PDF explains pump maintenance in 5 key steps: 1) Inspect seals for leakage... 2) Check pressure gauges... [continues with synthesis of multiple PDF chunks]"
}

✓ Reasoning & synthesis
✓ Multi-step problem solving
✓ Can cross-reference multiple chunks
✓ Intelligent tool selection
✓ Can verify with web search if needed
✗ Slower (~10-30s depending on steps)
✗ More complex
✗ Depends on LLM quality
```

## Use Cases Comparison

### Simple Lookup: "What's on page 42?"

| Feature | Search | Agent |
|---------|--------|-------|
| Response Time | < 1s | 10-30s |
| Accuracy | High | High |
| Usefulness | Get raw text | Get synthesized answer |
| **Best for** | **✓ Search** | ✗ Agent |

→ Use existing `/api/search` endpoint

### Synthesis: "Summarize the pump installation section"

| Feature | Search | Agent |
|---------|--------|-------|
| Response Time | < 1s | 15-30s |
| Accuracy | Returns chunks | Synthesized summary |
| Usefulness | Raw data | Ready-to-use summary |
| **Best for** | ✗ Search | **✓ Agent** |

→ Use new `/api/agent/query` endpoint

### Reasoning: "Compare PDF approach with modern standards"

| Feature | Search | Agent |
|---------|--------|-------|
| Response Time | < 1s | 20-40s |
| Accuracy | One API, no cross-reference | Multi-step reasoning |
| Usefulness | Just PDF chunks | Reasoning + web comparison |
| **Best for** | ✗ Search | **✓ Agent** |

→ Use new `/api/agent/query` endpoint (with web search enabled)

### Verification: "Is the PDF method still current?"

| Feature | Search | Agent |
|---------|--------|-------|
| Response Time | < 1s | 20-60s |
| Accuracy | Static PDF text | PDF + current web knowledge |
| Usefulness | What PDF says | What PDF says + context |
| **Best for** | ✗ Search | **✓ Agent** |

→ Use new `/api/agent/query` endpoint (leverages web search)

## Decision Tree

```
User asks a question
│
├─ "Get exact text/chunk from PDF?"
│  └─→ Use POST /api/search ✓
│      (fast, direct, simple)
│
├─ "Synthesize or reason about content?"
│  └─→ Use POST /api/agent/query ✓
│      (multi-step reasoning, synthesis)
│
├─ "Compare PDF with external sources?"
│  └─→ Use POST /api/agent/query ✓
│      (can use web_search tool)
│
└─ "Need current/recent information?"
   └─→ Use POST /api/agent/query ✓
       (web search for latest info)
```

## Example: Same Question, Different Endpoints

**Question:** "What are the key steps to maintain a pump?"

### Using Search API

```bash
curl -X POST http://localhost:8081/api/search \
  -H "Content-Type: application/json" \
  -d '{"query": "pump maintenance steps"}'
```

**Response:**
```json
{
  "chunks": [
    {
      "id": "doc_123_chunk_45",
      "text": "Pump maintenance requires regular inspection of seals, bearings, and impellers. Key steps: 1. Check for leaks 2. Inspect bearings 3. Verify impeller clearance...",
      "score": 0.92,
      "chapter": 4,
      "page_number": 42
    },
    {
      "id": "doc_123_chunk_46",
      "text": "Maintenance schedule: Daily checks, weekly lubrication, monthly bearing inspection, quarterly seal replacement...",
      "score": 0.88,
      "chapter": 4,
      "page_number": 43
    }
  ]
}
```

**User must:**
- Parse multiple chunks
- Extract information
- Synthesize answer
- Cross-reference details

### Using Agent API

```bash
curl -X POST http://localhost:8082/api/agent/query \
  -H "Content-Type: application/json" \
  -d '{"query": "What are the key steps to maintain a pump?"}'
```

**Response:**
```json
{
  "answer": "According to the PDF documentation (Chapter 4), pump maintenance requires five key steps:\n\n1. **Daily Checks** - Inspect seals and bearings for any signs of wear or leakage\n\n2. **Weekly Lubrication** - Apply appropriate lubricant to all moving parts as specified in the maintenance schedule\n\n3. **Monthly Bearing Inspection** - Test bearing temperature and listen for unusual sounds\n\n4. **Quarterly Seal Replacement** - Replace mechanical seals every quarter to prevent leakage\n\n5. **Pressure Testing** - Verify system pressure is within specifications (see page 43)\n\nThe PDF emphasizes that regular maintenance prevents 80% of pump failures and extends equipment lifespan by 2-3 years."
}
```

**User gets:**
- Synthesized answer
- Structured steps
- Key insights
- Ready to use

## Architecture Implications

### Existing System (Search Only)

```
┌──────────────┐
│   Frontend   │
└──────┬───────┘
       │
       ▼
┌──────────────────┐
│  PDF Search API  │
│  (port 8081)     │
└──────┬───────────┘
       │
       ▼
┌──────────────────┐
│  Vector DB       │
│  (embeddings)    │
└──────────────────┘
```

### With Agent

```
┌──────────────┐
│   Frontend   │
├──────┬───────┤
│      │       │
│      │       └─→ Raw search (fast)
│      │
│      └─→ Agent query (reasoning)
│
├─────────────────────┬─────────────┤
│                     │             │
▼                     ▼             ▼
PDF Search      Agent Service    (Web Search)
(port 8081)     (port 8082)      (optional)
                     │
                ┌────┼────┐
                ▼         ▼
            Ollama    Tavily
            (LLM)     (web)
```

## Performance Metrics

### Search API
- Avg latency: 500ms - 1s
- P95 latency: 1.5s
- Max tokens: N/A (returns raw chunks)
- Cost: Low (vector search only)

### Agent API
- Avg latency: 10-30s (3-5 steps)
- P95 latency: 40-60s (max reasoning depth)
- Max tokens: 1024 (configurable)
- Cost: Higher (LLM calls + search calls)

## Cost Considerations

### Search: Pay for embeddings + vector search
- One-time: Document embeddings
- Per-query: Vector search (minimal cost)

### Agent: Pay for LLM + tool calls
- Per-query: Multiple LLM calls (steps 1, 2, 3...)
- Per-query: Tool calls (PDF search + web search)
- Cost per query: ~10x search cost
- Cost worth it for: Complex reasoning, synthesis, verification

## When to Use Which

### Use Search API When:
- User wants raw information
- Speed is critical (< 1 second needed)
- Simple lookup query
- Citation/source matters more than synthesis
- Cost is primary concern

### Use Agent API When:
- User wants synthesized answer
- Reasoning is needed
- Cross-referencing information
- Need to combine PDF + web sources
- Verification with external sources
- User satisfaction > latency

## Migration Path

Your existing app doesn't change:
1. Keep `/api/search` endpoint as-is
2. Add new `/api/agent/query` endpoint (separate service)
3. In frontend, choose which to call based on query type
4. Users can use both endpoints for different needs

**No breaking changes.** Complete backwards compatibility.

## Summary

| Aspect | Search | Agent |
|--------|--------|-------|
| **Latency** | <1s | 10-30s |
| **Cost** | Low | Higher |
| **Reasoning** | No | Yes |
| **Synthesis** | No | Yes |
| **Web Search** | No | Yes (optional) |
| **Best For** | Simple lookups | Complex questions |
| **Learning Curve** | None | Understands ReAct |

**Use both.** They serve different purposes and work well together.
