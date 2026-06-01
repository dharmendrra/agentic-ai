# Project Structure

The OmniRAG Agent project follows the same structure as the existing Omni-RAG retrieval system.

## Directory Layout

```
agentic-ai/
├── main.go                    # Main application (386 lines, core logic)
├── config.json               # Configuration (external, no rebuild needed)
├── go.mod                    # Go module definition
├── agent                     # Compiled binary (executable)
│
├── static/                   # Static web assets
│   ├── index.html           # Main UI page
│   └── app.js              # Client-side logic
│
├── .gitignore              # Git ignore rules
├── test.sh                 # API test script
│
└── Documentation/
    ├── README.md               # Complete documentation
    ├── QUICKSTART.md          # 5-minute setup guide
    ├── GETTING_STARTED.txt    # Quick reference
    ├── INTEGRATION.md         # Integration guide
    ├── IMPLEMENTATION.md      # Technical details
    ├── COMPARISON.md          # Search vs Agent
    └── STRUCTURE.md          # This file
```

## Matching Omni-RAG Structure

Your existing Omni-RAG project:
```
pinecone-rag/
├── main.go
├── go.mod
├── retrieval (binary)
└── static/
    ├── index.html
    └── app.js
```

The agent service:
```
agentic-ai/
├── main.go
├── go.mod
├── agent (binary)
└── static/
    ├── index.html
    └── app.js
```

✓ Same structure, easy to manage both services

## File Descriptions

### Core Files

| File | Purpose | Size |
|------|---------|------|
| `main.go` | Agent logic, ReAct loop, HTTP handlers | 386 lines |
| `config.json` | Configuration (endpoints, models, keys) | 200 bytes |
| `go.mod` | Go module definition | ~50 bytes |
| `agent` | Compiled binary, ready to run | 9.8 MB |

### Static Files

| File | Purpose | Size |
|------|---------|------|
| `static/index.html` | UI HTML page | 5.4 KB |
| `static/app.js` | Client-side JavaScript | 5.9 KB |

### Documentation

| File | Purpose |
|------|---------|
| `README.md` | Complete feature documentation |
| `QUICKSTART.md` | Setup & deployment guide |
| `GETTING_STARTED.txt` | Quick reference card |
| `INTEGRATION.md` | Integration with existing systems |
| `IMPLEMENTATION.md` | Technical architecture & design |
| `COMPARISON.md` | Search vs Agent comparison |
| `STRUCTURE.md` | This file |

### Scripts

| File | Purpose |
|------|---------|
| `test.sh` | API endpoint test script |

## Code Organization

### main.go Sections

```go
1. Imports & Config Loading (lines 1-30)
   - Load config.json
   - Parse configuration values

2. Type Definitions (lines 30-60)
   - Config struct
   - Request/Response types
   - OllamaRequest structure
   - Ollama/Tavily types

3. Tool Implementations (lines 60-190)
   - searchPDF()      → Calls your PDF search API
   - webSearch()      → Calls Tavily API
   - callOllama()     → Calls Ollama LLM
   - runReAct()       → Main ReAct loop

4. HTTP Handlers (lines 190-220)
   - handleAgentQuery()    → POST /api/agent/query
   - handleStatic()        → Serve static files

5. Main & Routing (lines 350-385)
   - Port configuration
   - Route registration
   - Server startup
```

### static/index.html Sections

```html
1. Head (lines 1-40)
   - Meta tags
   - Tailwind CSS setup
   - Custom styles

2. Body (lines 40+)
   - Header with branding
   - Input area with textarea
   - Results container
   - Error banner
   - Script load

3. Theme
   - Dark mode (bg-slate-950)
   - Emerald accent (emerald-400/500/600)
   - Tailwind CSS framework
```

### static/app.js Sections

```javascript
1. DOM References (lines 1-20)
   - Cache all element references

2. State Management (lines 20-40)
   - isLoading flag
   - Result tracking

3. Helper Functions (lines 40-100)
   - setLoading()      → Update button state
   - showError()       → Display error
   - hideError()       → Clear error
   - createAnswerCard() → Build result card

4. Agent Handler (lines 100-150)
   - runAgent()        → Main query handler
   - API call logic
   - Response handling

5. Input Binding (lines 150-170)
   - Click handler
   - Keyboard shortcuts (Cmd+Enter / Ctrl+Enter)

6. Initialization (lines 170+)
   - Focus input on page load
```

## Static File Serving

How routing works:

```
Request          Handler              Path
─────────────────────────────────────────────────
GET /            Root handler         → /index.html
GET /app.js      File server          → /static/app.js
GET /index.html  File server          → /static/index.html
POST /api/agent/query  handleAgentQuery → ReAct loop
```

## Building & Deployment

### Development

```bash
# Edit code in main.go
nano main.go

# Edit UI in static/
nano static/index.html
nano static/app.js

# Rebuild
go build -o agent main.go

# Run
./agent
```

### Production

Same binary works everywhere:

```bash
# Copy to server
scp agent config.json user@server:/opt/agentic-ai/
scp -r static/ user@server:/opt/agentic-ai/

# Run on server
./agent -port 8082
```

## Configuration

`config.json` is loaded at startup:

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

Change any value without rebuilding—just restart the service.

## Adding New Files

### Add a new static file

1. Create file in `static/` folder
   ```bash
   echo "..." > static/style.css
   ```

2. Reference in HTML
   ```html
   <link rel="stylesheet" href="/style.css">
   ```

3. No code changes needed—server automatically serves it

### Add a new API endpoint

1. Add handler function in main.go
   ```go
   func handleNewEndpoint(w http.ResponseWriter, r *http.Request) {
     // implementation
   }
   ```

2. Register route in main()
   ```go
   http.HandleFunc("/api/new", handleNewEndpoint)
   ```

3. Rebuild
   ```bash
   go build -o agent main.go
   ```

## Comparison with Omni-RAG

Both projects have:

✓ Single `main.go` entry point
✓ `static/` folder for web assets
✓ External `config.json`
✓ Compiled binary ready to deploy
✓ Same HTTP routing patterns
✓ Similar error handling

Differences:

| Aspect | Omni-RAG | Agent |
|--------|----------|-------|
| Purpose | Vector search | ReAct reasoning |
| Dependencies | Vector DB | Ollama LLM |
| Routes | `/api/search` | `/api/agent/query` |
| UI | Search results | Reasoning answers |
| Database | Pinecone | None |

## Deployment Checklist

- [ ] `main.go` builds without errors
- [ ] `static/` folder exists with HTML and JS
- [ ] `config.json` has correct endpoints
- [ ] `go build -o agent main.go` succeeds
- [ ] `./agent` starts without errors
- [ ] `http://localhost:8082/` loads HTML
- [ ] `http://localhost:8082/app.js` returns JavaScript
- [ ] `curl http://localhost:8082/api/agent/query` works
- [ ] Both services can reach each other (network check)

## Next Steps

1. **Run locally**: `./agent` and test at `http://localhost:8082`
2. **Customize**: Edit `config.json` for your endpoints
3. **Deploy**: Copy `agent`, `config.json`, and `static/` to server
4. **Scale**: Run multiple instances behind a load balancer

See QUICKSTART.md for detailed instructions.
