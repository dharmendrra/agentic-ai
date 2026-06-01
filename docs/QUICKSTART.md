# Quick Start Guide

Get the agent endpoint running in 5 minutes.

## Prerequisites

1. **Ollama** running on `http://localhost:11434`
   - Install from https://ollama.ai
   - Run: `ollama serve`

2. **PDF Search Endpoint** running on `http://localhost:8081`
   - Your existing retrieval/search API

3. **Tavily API Key** (optional, for web search)
   - Sign up at https://tavily.com
   - Get your API key

## Setup

### 1. Configure

Edit `config.json`:

```bash
cat > config.json << 'EOF'
{
  "OLLAMA_HOST": "http://localhost:11434",
  "OLLAMA_MODEL": "gemma4:e4b",
  "TAVILY_API_KEY": "your-key-here",
  "MAX_STEPS": 10,
  "MAX_RETRIES": 3,
  "MAX_TOKENS": 1024,
  "SEARCH_ENDPOINT": "http://localhost:8081/api/search"
}
EOF
```

### 2. Build

```bash
go build -o agent main.go
```

### 3. Run

```bash
./agent
```

Output:
```
Agent server listening on http://localhost:8082
Endpoint: POST http://localhost:8082/api/agent/query
UI: http://localhost:8082/agent
```

## Test It

### Option A: Web UI

Open browser: http://localhost:8082/agent

Type a question and click "Ask Agent"

### Option B: API (curl)

```bash
curl -X POST http://localhost:8082/api/agent/query \
  -H "Content-Type: application/json" \
  -d '{"query": "How does the PDF explain pump maintenance?"}'
```

Response:
```json
{
  "answer": "The final answer from the agent's reasoning..."
}
```

### Option C: Test Script

```bash
# Start the agent first
./agent &

# Run test in another terminal
chmod +x test.sh
./test.sh
```

## How It Works

### Flow

1. User asks a question
2. Agent runs ReAct loop (max 10 steps)
3. Each step: Thought → Action → Observation
4. Actions: `search_pdf` or `web_search`
5. Returns "Final Answer" when done

### Tool Strategy

- **search_pdf** (PRIMARY): PDF documents, ground truth
  - Fast, accurate, sourced
  - Try this first

- **web_search** (FALLBACK): Internet
  - For topics not in PDF
  - Verification/context
  - Only if PDF insufficient

### Example ReAct Loop

```
Thought: The user is asking about pump maintenance. Let me search the PDFs first.
Action: search_pdf
Action Input: pump maintenance steps
Observation: [PDF results...]

Thought: Great! The PDF has good information. Let me answer based on this.
Final Answer: According to the PDF, pump maintenance involves...
```

## Customization

### Change Model

Edit `config.json`, update `OLLAMA_MODEL`:

```json
{
  "OLLAMA_MODEL": "llama2",
  ...
}
```

Pull the model first:
```bash
ollama pull llama2
```

### Change Max Steps

Edit `config.json`:

```json
{
  "MAX_STEPS": 5,
  ...
}
```

Lower = faster, less reasoning
Higher = thorough, slower

### Disable Web Search

Set empty TAVILY_API_KEY:

```json
{
  "TAVILY_API_KEY": "",
  ...
}
```

Agent will use only PDF search.

### Custom PDF Endpoint

Update `SEARCH_ENDPOINT`:

```json
{
  "SEARCH_ENDPOINT": "http://your-api.com/search",
  ...
}
```

## Troubleshooting

### "Connection refused" on startup

Check prerequisites are running:

```bash
# Check Ollama
curl http://localhost:11434/api/tags

# Check PDF search endpoint
curl -X POST http://localhost:8081/api/search \
  -H "Content-Type: application/json" \
  -d '{"query": "test"}'
```

### Slow responses

- Lower MAX_TOKENS in config
- Reduce MAX_STEPS
- Use a faster Ollama model
- Check Ollama is not overloaded

### Tool never runs

- Check ReAct parsing (agent outputs must match "Action: tool_name" format)
- Verify Ollama model supports ReAct reasoning
- Try a larger model (gemma4:e4b works well)

### Web search returns nothing

- Check TAVILY_API_KEY is valid
- Try simpler search queries
- Check internet connection

## Integration

### Integrate with Your App

Option 1: Proxy the agent endpoint

```go
http.HandleFunc("/api/agent/query", func(w http.ResponseWriter, r *http.Request) {
  proxy.ServeHTTP(w, r)  // proxy to http://localhost:8082
})
```

Option 2: Direct HTTP calls from frontend

```javascript
const resp = await fetch('http://localhost:8082/api/agent/query', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ query: userQuestion })
});
const data = await resp.json();
```

Option 3: Call from backend

```go
resp, _ := http.Post("http://localhost:8082/api/agent/query", "application/json", body)
```

## Next Steps

- [ ] Set TAVILY_API_KEY in config.json
- [ ] Start Ollama: `ollama serve`
- [ ] Build: `go build -o agent main.go`
- [ ] Run: `./agent`
- [ ] Test: `http://localhost:8082/agent`
- [ ] Integrate with your app

## API Reference

### POST /api/agent/query

**Request:**
```json
{
  "query": "Your question here"
}
```

**Response:**
```json
{
  "answer": "The agent's final answer based on reasoning"
}
```

### GET /agent

Returns HTML UI for interactive queries.

---

Questions? Check README.md for detailed docs.
