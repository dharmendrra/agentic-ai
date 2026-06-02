# Setup & Deployment Guide

## Prerequisites

1. **Ollama** running on `http://localhost:11434`
   ```bash
   ollama serve
   ollama pull gemma4:e4b
   ```

2. **Omni-RAG retrieval service** running on `http://localhost:8081`

3. **Tavily API key** (optional — for web search fallback)
   - Sign up at https://tavily.com

---

## Configure

Edit `config.json`:

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

| Key | Effect |
|-----|--------|
| `OLLAMA_MODEL` | Which model does the reasoning — must be pulled first |
| `MAX_STEPS` | Max ReAct loop iterations (lower = faster, less thorough) |
| `MAX_TOKENS` | Token limit per Ollama call |
| `MAX_RETRIES` | HTTP retry attempts with exponential backoff |
| `SEARCH_ENDPOINT` | URL of your Omni-RAG `/api/search` endpoint |
| `TAVILY_API_KEY` | Leave empty `""` to disable web search |

---

## Build & Run

```bash
go build -o agent main.go
./agent                  # default port 8082
./agent -port 8080       # custom port
```

Expected output:
```
Agent server listening on http://localhost:8082
Endpoint: POST http://localhost:8082/api/agent/query
UI: http://localhost:8082/agent
```

---

## Test

**Web UI:**
```
http://localhost:8082
```

**curl:**
```bash
curl -X POST http://localhost:8082/api/agent/query \
  -H "Content-Type: application/json" \
  -d '{"query": "How does the PDF explain pump maintenance?"}'
```

Response:
```json
{ "answer": "According to the PDF, pump maintenance involves..." }
```

---

## Tuning

**Faster responses:**
```json
{ "MAX_STEPS": 5, "MAX_TOKENS": 512 }
```

**More thorough reasoning:**
```json
{ "MAX_STEPS": 15, "MAX_TOKENS": 2048 }
```

**Disable web search:**
```json
{ "TAVILY_API_KEY": "" }
```

**Swap model:**
```bash
ollama pull llama3.2
```
```json
{ "OLLAMA_MODEL": "llama3.2" }
```

---

## Troubleshooting

**Connection refused at startup:**
```bash
curl http://localhost:11434/api/tags                  # check Ollama
curl -X POST http://localhost:8081/api/search \
  -H "Content-Type: application/json" \
  -d '{"query":"test"}'                               # check retrieval API
```

**Tool never called (agent reasons but doesn't act):**
- The LLM must output `Action: search_pdf` or `Action: web_search` exactly
- Try a larger model — smaller models sometimes don't follow ReAct format reliably

**Web search returns nothing:**
```bash
curl "https://api.tavily.com/search?api_key=YOUR_KEY&query=test"
```

**PDF returns `[PDF_EMPTY]`:**
- Query has no matching vectors in Pinecone
- Agent will automatically fall back to web search

---

## Proxy Integration

**Go reverse proxy (add to your existing server):**
```go
import "net/http/httputil"

agentProxy := httputil.NewSingleHostReverseProxy(mustParseURL("http://localhost:8082"))
http.HandleFunc("/api/agent/", agentProxy.ServeHTTP)
http.HandleFunc("/agent", agentProxy.ServeHTTP)
```

**nginx:**
```nginx
location /api/agent/ {
    proxy_pass http://localhost:8082;
    proxy_set_header Host $host;
}
location /agent {
    proxy_pass http://localhost:8082;
}
```

**Frontend JS:**
```javascript
const resp = await fetch('/api/agent/query', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ query: userQuestion })
});
const { answer } = await resp.json();
```

---

## Docker

`Dockerfile`:
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

`docker-compose.yml`:
```yaml
services:
  ollama:
    image: ollama/ollama:latest
    ports: ["11434:11434"]
    volumes: [ollama_data:/root/.ollama]

  retrieval:
    image: your-retrieval-api:latest
    ports: ["8081:8081"]
    depends_on: [ollama]

  agent:
    image: agentic-ai:latest
    ports: ["8082:8082"]
    environment:
      - SEARCH_ENDPOINT=http://retrieval:8081/api/search
      - OLLAMA_HOST=http://ollama:11434
    depends_on: [ollama, retrieval]

volumes:
  ollama_data:
```

---

## systemd

`/etc/systemd/system/omnirag-agent.service`:
```ini
[Unit]
Description=OmniRAG Agent
After=network.target

[Service]
Type=simple
User=appuser
WorkingDirectory=/opt/agentic-ai
ExecStart=/opt/agentic-ai/agent -port 8082
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

---

## Typical Latency

| Scenario | Time |
|----------|------|
| 1-step (direct PDF hit) | 2–5s |
| 3-step (PDF + web fallback) | 10–20s |
| 5-step complex reasoning | 30–60s |

*Varies with model size, Ollama hardware, and Pinecone latency.*
