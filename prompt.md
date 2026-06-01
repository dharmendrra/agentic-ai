

I have a Go web app with a PDF RAG system. Existing endpoint `POST http://localhost:8081/api/search` takes `{"query": "string"}` and returns matched chunks from a vector DB. Only header needed is `Content-Type: application/json`.

Build an agent endpoint `POST /api/agent/query` that:
- Takes `{"query": "string"}`
- Runs a ReAct loop against Ollama API (not Anthropic)
- Has two tools: `search_pdf` (calls `POST http://localhost:8081/api/search`) and `web_search` (calls Tavily API)
- System prompt positions PDF as primary source, web as fallback
- Returns `{"answer": "string"}`

Tool descriptions should make LLM prefer PDF first, web only as fallback.

**Config:** read from `config.json` in project root:
```json
{
  "OLLAMA_HOST": "http://localhost:11434",
  "OLLAMA_MODEL": "gemma4:e4b",
  "TAVILY_API_KEY": "",
  "MAX_STEPS": 10,
  "MAX_RETRIES": 3,
  "MAX_TOKENS": 1024
}
```

**Guardrails:**
- Max steps loop (from config)
- Max retries on failed HTTP calls (from config)
- If max steps hit, return best partial answer so far, not an error
- Set `max_tokens` from config on every Ollama API call

Add a UI page for the agent endpoint. Match the existing UI theme exactly — same colors, fonts, and component patterns from the frontend files already in this folder (/Users/dharmendra/golang-projects/Omni-RAG/pinecone-rag/retrieval/static).

No frameworks. Standard Go `net/http` and `encoding/json` only.