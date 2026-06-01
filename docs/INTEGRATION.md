# Integration Guide

How to integrate the OmniRAG Agent with your existing retrieval system.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Your Frontend App                        │
│                 (React / Vue / Vanilla JS)                   │
└──────────────────────────┬──────────────────────────────────┘
                          │
        ┌─────────────────┴──────────────────┐
        │                                    │
        ▼                                    ▼
┌──────────────────┐              ┌──────────────────┐
│  PDF Search UI   │              │  Agent Reasoning │
│  /api/search     │              │  /api/agent/*    │
└──────────────────┘              └──────────────────┘
        │                                    │
        │         (same network)             │
        │                                    │
        ▼                                    ▼
┌──────────────────────────────────────────────────────┐
│              OmniRAG Retrieval Backend                │
│          (port 8081 - existing endpoint)             │
│     [PDF parsing, embeddings, vector search]         │
└──────────────┬───────────────────────────────────────┘
               │
        ┌──────┴──────┐
        │             │
        ▼             ▼
   ┌─────────┐  ┌──────────┐
   │ MongoDB │  │ Pinecone │
   │ GridFS  │  │ (vector) │
   └─────────┘  └──────────┘

┌──────────────────────────────────────────────────────┐
│           OmniRAG Agent (New)                        │
│          (port 8082 - this service)                 │
│    [ReAct reasoning, tool orchestration]            │
└──────────────┬───────────────────────────────────────┘
               │
        ┌──────┴──────┐
        │             │
        ▼             ▼
   ┌──────────┐  ┌──────────┐
   │  Ollama  │  │  Tavily  │
   │ (LLM)    │  │ (web)    │
   └──────────┘  └──────────┘
```

## Setup Steps

### Step 1: Deploy Agent Service

Run the agent on a separate port (default 8082):

```bash
cd /path/to/agentic-ai
go build -o agent main.go
./agent -port 8082
```

Or in production (systemd/Docker):

```bash
[Unit]
Description=OmniRAG Agent
After=network.target

[Service]
Type=simple
User=appuser
WorkingDirectory=/opt/agentic-ai
ExecStart=/opt/agentic-ai/agent -port 8082
Restart=on-failure
Environment="CONFIG_PATH=/etc/agentic-ai/config.json"

[Install]
WantedBy=multi-user.target
```

### Step 2: Configure Connection

Edit `/Users/dharmendra/golang-projects/agentic-ai/config.json`:

```json
{
  "OLLAMA_HOST": "http://localhost:11434",
  "OLLAMA_MODEL": "gemma4:e4b",
  "TAVILY_API_KEY": "tvly-xxx...",
  "MAX_STEPS": 10,
  "MAX_RETRIES": 3,
  "MAX_TOKENS": 1024,
  "SEARCH_ENDPOINT": "http://localhost:8081/api/search"
}
```

Key points:
- `SEARCH_ENDPOINT` points to your existing retrieval API
- Both services must be on same network or reachable
- No auth credentials needed (update if your API requires auth)

### Step 3: Expose Endpoints

Option A: Direct exposure (development)

Frontend calls both endpoints:

```javascript
// PDF search
fetch('http://localhost:8081/api/search', ...)

// Agent reasoning
fetch('http://localhost:8082/api/agent/query', ...)
```

Option B: Proxy through main server (production)

Add to your Go server:

```go
import "net/http/httputil"

// Agent proxy
agentProxy := httputil.NewSingleHostReverseProxy(
  mustParseURL("http://localhost:8082"))

http.HandleFunc("/api/agent/", agentProxy.ServeHTTP)
http.HandleFunc("/agent", agentProxy.ServeHTTP)
```

Or in your load balancer (nginx):

```nginx
location /api/agent/ {
  proxy_pass http://agent-service:8082;
  proxy_set_header Host $host;
}

location /agent {
  proxy_pass http://agent-service:8082;
}
```

### Step 4: Update Your Frontend

Your existing OmniRAG UI can now call the agent:

```javascript
async function queryWithReasoning(question) {
  const response = await fetch('/api/agent/query', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ query: question })
  });
  return response.json();
}
```

Or link to the agent UI:

```html
<a href="http://localhost:8082/agent" target="_blank">
  Ask with Reasoning
</a>
```

## API Changes

### Existing API (unchanged)

```
POST /api/search
- Request: { "query": "string" }
- Response: { "chunks": [...], ... }
- Still available on port 8081
```

### New Agent API

```
POST /api/agent/query
- Request: { "query": "string" }
- Response: { "answer": "string" }
- Available on port 8082 (or proxied)

GET /agent
- Returns HTML/JS UI
- Matches OmniRAG theme
```

## Request/Response Examples

### Direct Agent Query

```bash
curl -X POST http://localhost:8082/api/agent/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "How does the PDF explain pump installation? Compare with industry standards."
  }'
```

Response:

```json
{
  "answer": "According to the PDF, pump installation requires...\n\nIndustry standards differ by...\n\nBased on web research, modern best practices include..."
}
```

### Through Frontend

```javascript
const userQuestion = "What are the maintenance steps?";

const response = await fetch('/api/agent/query', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ query: userQuestion })
});

const { answer } = await response.json();
console.log(answer);
```

## Error Handling

Agent gracefully handles errors:

```javascript
try {
  const resp = await fetch('/api/agent/query', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ query: userQuestion })
  });

  if (!resp.ok) {
    console.error(`Agent error (${resp.status}):`, await resp.text());
    return;
  }

  const { answer } = await resp.json();
  displayAnswer(answer);

} catch (err) {
  console.error('Network error:', err.message);
}
```

## Performance Tuning

### For Faster Responses

1. **Reduce MAX_STEPS** in config.json
   ```json
   { "MAX_STEPS": 5 }
   ```

2. **Lower MAX_TOKENS** for faster generation
   ```json
   { "MAX_TOKENS": 512 }
   ```

3. **Use faster Ollama model**
   ```json
   { "OLLAMA_MODEL": "mistral" }
   ```

### For Better Answers

1. **Increase MAX_STEPS** for more thorough reasoning
   ```json
   { "MAX_STEPS": 15 }
   ```

2. **Increase MAX_TOKENS** for longer responses
   ```json
   { "MAX_TOKENS": 2048 }
   ```

3. **Use larger Ollama model**
   ```json
   { "OLLAMA_MODEL": "neural-chat" }
   ```

## Scaling Considerations

### Single Server

```
┌─────────────────────────────────────────┐
│         Single Host                     │
│  ┌───────────────┐  ┌────────────────┐  │
│  │ Retrieval API │  │  Agent Service │  │
│  │  (port 8081)  │  │  (port 8082)   │  │
│  └───────────────┘  └────────────────┘  │
│         ↓                   ↓            │
│       Ollama            Ollama           │
│       (shared)          (shared)         │
└─────────────────────────────────────────┘
```

### Microservices

```
Load Balancer
  ├── Retrieval API (8081, replicas: 2)
  ├── Agent Service (8082, replicas: 2)
  └── Ollama Service (shared)
```

For multiple agent instances, use a reverse proxy:

```nginx
upstream agents {
  server agent1.local:8082;
  server agent2.local:8082;
  server agent3.local:8082;
}

location /api/agent/ {
  proxy_pass http://agents;
}
```

## Docker Deployment

Build agent image:

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o agent main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/agent .
COPY config.json .
EXPOSE 8082
CMD ["./agent", "-port", "8082"]
```

Run together:

```yaml
version: '3.8'
services:
  ollama:
    image: ollama/ollama:latest
    ports:
      - "11434:11434"
    volumes:
      - ollama_data:/root/.ollama

  retrieval:
    image: your-retrieval-api:latest
    ports:
      - "8081:8081"
    depends_on:
      - ollama

  agent:
    image: agentic-ai:latest
    ports:
      - "8082:8082"
    environment:
      - SEARCH_ENDPOINT=http://retrieval:8081/api/search
      - OLLAMA_HOST=http://ollama:11434
    depends_on:
      - ollama
      - retrieval

volumes:
  ollama_data:
```

## Monitoring

Add logging to track agent usage:

```go
import "log"

func handleAgentQuery(w http.ResponseWriter, r *http.Request) {
  var req AgentRequest
  json.NewDecoder(r.Body).Decode(&req)
  
  log.Printf("[AGENT] Query: %s", req.Query)
  
  answer, err := runReAct(req.Query)
  if err != nil {
    log.Printf("[AGENT] Error: %v", err)
    http.Error(w, err.Error(), 500)
    return
  }
  
  log.Printf("[AGENT] Answer length: %d chars", len(answer))
  // ... respond ...
}
```

Monitor endpoints:

```bash
# Check if agent is responsive
curl -s http://localhost:8082/api/agent/query \
  -H "Content-Type: application/json" \
  -d '{"query":"test"}' > /dev/null && echo "OK" || echo "FAIL"

# Check retrieval API
curl -s http://localhost:8081/api/search \
  -H "Content-Type: application/json" \
  -d '{"query":"test"}' | jq .
```

## Troubleshooting Integration

### Agent can't reach retrieval API

Check config:
```bash
curl -X POST http://localhost:8081/api/search \
  -H "Content-Type: application/json" \
  -d '{"query":"test"}'
```

If fails, update `SEARCH_ENDPOINT` in `config.json`.

### Slow agent responses

1. Check Ollama is not overloaded
2. Reduce MAX_TOKENS
3. Use faster model
4. Check network latency

### Web search not working

- Verify TAVILY_API_KEY is set
- Test with curl:
  ```bash
  curl "https://api.tavily.com/search?api_key=YOUR_KEY&query=test"
  ```

## Summary

1. **Run agent service** on port 8082
2. **Configure SEARCH_ENDPOINT** to point to your API
3. **Expose endpoints** via proxy or direct (http://localhost:8082)
4. **Update frontend** to call `/api/agent/query` for reasoning queries
5. **Monitor** both endpoints for health

The agent integrates seamlessly with your existing OmniRAG retrieval system—no changes needed to your existing API.
