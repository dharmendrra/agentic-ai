# System Architecture

Professional overview of how the agentic-ai system works.

---

## 01 — RAG RETRIEVAL
**Retrieve relevant document chunks from vector database.**

```
User Query
    │
    ▼
┌─────────────────────────────┐
│  EMBEDDING MODEL            │
│  (Ollama gemma4:e2b)        │
│  Convert query to vector    │
└──────────┬──────────────────┘
           │
           ▼
┌─────────────────────────────┐
│  VECTOR DATABASE            │
│  (Pinecone)                 │
│  Search for similar chunks  │
└──────────┬──────────────────┘
           │
           ▼
┌─────────────────────────────┐
│  RETRIEVED CHUNKS           │
│  • Text content             │
│  • Metadata (page, score)   │
│  • Similarity scores        │
└─────────────────────────────┘
```

**What happens:**
1. User question is converted to a 768-dimensional vector
2. System finds 3 most similar document chunks
3. Each chunk includes: text, page number, relevance score

---

## 02 — REACT AGENT LOOP
**Multi-step reasoning with tool selection.**

```
START
  │
  ▼
┌────────────────────────────────┐
│  LLM DECISION                  │
│  "Which tool should I use?"    │
│                                │
│  Available Tools:              │
│  • search_pdf (primary)        │
│  • web_search (fallback)       │
└────────┬─────────────────────┐
         │                     │
    [PDF Found?]          [No PDF]
         │                     │
         ▼                     ▼
    ┌──────────┐         ┌──────────┐
    │ search_  │         │ web_     │
    │ pdf tool │         │ search   │
    │ [SUCCESS]│         │ tool     │
    └────┬─────┘         └────┬─────┘
         │                    │
         └──────┬─────────────┘
                │
                ▼
        ┌──────────────────┐
        │ Add Observation  │
        │ to History       │
        └────────┬─────────┘
                 │
                 ▼
        ┌──────────────────┐
        │ More Steps?      │
        │ Max: 10 steps    │
        └────┬─────────┬───┘
             │ YES     │ NO
             ▼         ▼
          LOOP      FINAL ANSWER
```

**What happens:**
1. LLM analyzes query and decides: PDF search or web search?
2. Tool executes and returns observation
3. LLM decides: continue reasoning or provide answer?
4. Loop continues until answer found (max 10 steps)

---

## 03 — PDF SEARCH TOOL
**Search and extract from your documents.**

```
User Query
    │
    ▼
┌─────────────────────────────────┐
│  PDF Search Endpoint            │
│  POST /api/search               │
│  (Your Omni-RAG service)        │
└──────────┬──────────────────────┘
           │
           ▼
┌─────────────────────────────────┐
│  Processing Pipeline            │
│                                 │
│  1. Embed query                 │
│  2. Query Pinecone              │
│  3. Generate answer             │
│  4. Stream tokens               │
└──────────┬──────────────────────┘
           │
           ▼
┌─────────────────────────────────┐
│  SSE Stream Response            │
│                                 │
│  event: stage                   │
│  event: token (streaming)       │
│  event: sources (3 chunks)      │
│  event: done                    │
└──────────┬──────────────────────┘
           │
           ▼
┌─────────────────────────────────┐
│  Tool Extracts                  │
│  • Chunks from sources event    │
│  • [PDF_SUCCESS|Found N chunks] │
│    or                           │
│    [PDF_EMPTY|No results]       │
└─────────────────────────────────┘
```

**What happens:**
1. Tool calls your PDF search endpoint
2. Endpoint streams back results (SSE format)
3. Tool extracts text chunks from response
4. Returns structured result to agent

---

## 04 — WEB SEARCH TOOL
**Fallback: search the internet.**

```
User Query
    │
    ▼
┌─────────────────────────────────┐
│  Tavily Web Search API          │
│  POST /search                   │
│  (api.tavily.com)               │
└──────────┬──────────────────────┘
           │
           ▼
┌─────────────────────────────────┐
│  Web Search Response            │
│                                 │
│  {                              │
│    "answer": "...",             │
│    "results": [                 │
│      {"url": "...", ...},       │
│      {"url": "...", ...}        │
│    ]                            │
│  }                              │
└──────────┬──────────────────────┘
           │
           ▼
┌─────────────────────────────────┐
│  Tool Processes                 │
│  Extract answer field           │
│  Return top 3 results           │
│  [WEB_SUCCESS|Found N results]  │
└─────────────────────────────────┘
```

**What happens:**
1. Tool calls Tavily API with query
2. Gets web search results
3. Extracts answer text and top results
4. Returns to agent

---

## 05 — REQUEST/RESPONSE FLOW
**Complete journey of a user query.**

```
┌──────────────────────────────────────────────────────────────────┐
│  USER                                                            │
│  POST /api/agent/query                                          │
│  { "query": "How does PDF explain pump maintenance?" }          │
└────────────────────────────┬─────────────────────────────────────┘
                             │
                             ▼
                    ┌────────────────────┐
                    │  AGENT HANDLER     │
                    │  Process request   │
                    └────────┬───────────┘
                             │
                             ▼
                    ┌────────────────────┐
                    │  REACT LOOP        │
                    │  • Step 1: search_ │
                    │    pdf             │
                    │  • Step 2: check   │
                    │    results         │
                    │  • Step 3: answer  │
                    └────────┬───────────┘
                             │
                             ▼
                    ┌────────────────────┐
                    │  LLM Response      │
                    │  Final Answer      │
                    │  with sources      │
                    └────────┬───────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│  HTTP RESPONSE (200 OK)                                         │
│  {                                                              │
│    "answer": "Based on the provided PDF excerpts,              │
│              pump maintenance involves..."                     │
│  }                                                              │
└──────────────────────────────────────────────────────────────────┘
```

---

## 06 — SYSTEM COMPONENTS
**All the pieces working together.**

```
┌─────────────────────────────────────────────────────────────┐
│                                                              │
│  WEB UI (Frontend)                                          │
│  ┌────────────────────────────────────────────────────┐    │
│  │ • Input textarea for queries                       │    │
│  │ • Real-time answer display                         │    │
│  │ • Source citations                                 │    │
│  │ • Keyboard shortcuts (Cmd+Enter)                   │    │
│  │ • Dark theme (Tailwind CSS)                        │    │
│  └────────────────────────────────────────────────────┘    │
│           │ (HTTP POST)                                     │
│           ▼                                                  │
│  AGENT SERVER (Go - Port 8082)                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │ • ReAct Loop Engine                                │    │
│  │ • Tool Manager (PDF + Web search)                  │    │
│  │ • LLM Orchestration                                │    │
│  │ • Configuration Management                         │    │
│  └──┬──────────────────────────┬──────────────────┬──┘    │
│     │                          │                  │         │
│     ▼                          ▼                  ▼         │
│  PDF Search              Web Search            LLM         │
│  Endpoint                (Tavily)              (Ollama)     │
│  (Port 8081)            (api.tavily.com)     (Port 11434)  │
│     │                          │                  │         │
│     ▼                          ▼                  ▼         │
│  Pinecone VectorDB       Internet Search    Ollama Server  │
│  (Your documents)        (Top 3 results)    (2 models)     │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Key Flows at a Glance

### Flow A: PDF Answer Found
```
Query → search_pdf [SUCCESS] → Found chunks → LLM answers → Response
```

### Flow B: PDF Empty, Use Web
```
Query → search_pdf [EMPTY] → web_search [SUCCESS] → LLM answers → Response
```

### Flow C: Multi-step Reasoning
```
Query → search_pdf [SUCCESS] 
      → LLM says "Need more info" 
      → web_search [SUCCESS]
      → LLM combines both 
      → Final Answer
```

---

## Configuration

All behavior is configurable in `config.json`:

```json
{
  "OLLAMA_HOST": "http://localhost:11434",      // Your LLM
  "OLLAMA_MODEL": "gemma4:e4b",                 // Generation model
  "TAVILY_API_KEY": "your-key-here",            // Web search
  "MAX_STEPS": 10,                              // Loop limit
  "MAX_RETRIES": 3,                             // HTTP retries
  "MAX_TOKENS": 1024,                           // LLM output limit
  "SEARCH_ENDPOINT": "http://localhost:8081/api/search"  // PDF search
}
```

---

## Summary

| Component | Purpose | Endpoint |
|-----------|---------|----------|
| **Web UI** | Query input & result display | http://localhost:8082/agent |
| **Agent Server** | ReAct loop & orchestration | http://localhost:8082/api/agent/query |
| **PDF Search** | Vector search in documents | http://localhost:8081/api/search |
| **Web Search** | Internet fallback | api.tavily.com |
| **LLM** | Reasoning & answer generation | http://localhost:11434 |
| **Vector DB** | Embedded documents | Pinecone |

Everything works together to turn your question into a grounded, cited answer.
