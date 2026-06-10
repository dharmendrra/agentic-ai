# Tools Architecture

The agent now uses a modular tools system with separation of concerns.

## Structure

```
agentic-ai/
├── main.go                    ← Main application (ReAct loop orchestrator)
├── tools/                     ← Tool services
│   ├── tools.go              ← Tool manager & interface
│   ├── errors.go             ← Tool errors
│   ├── pdf_search.go         ← PDF search service
│   └── web_search.go         ← Web search service
├── static/                    ← Web UI
├── config.json               ← Configuration
└── agent                     ← Binary
```

## Tool Interface

All tools implement the `Tool` interface:

```go
type Tool interface {
    Execute(input string) (string, error)
    Name() string
    Description() string
}
```

## Tool Manager

Central registry for all tools:

```go
// Create manager
toolMgr := tools.NewManager()

// Register tools
toolMgr.Register(tools.NewPDFSearchTool(endpoint, maxRetries))
toolMgr.Register(tools.NewWebSearchTool(apiKey, maxRetries))

// Execute tool
result, err := toolMgr.Execute("search_pdf", query)
```

## Available Tools

### PDFSearchTool (`tools/pdf_search.go`)

Searches PDF documents via HTTP endpoint.

```go
tool := tools.NewPDFSearchTool(
    "http://localhost:8081/api/search",
    3, // maxRetries
)

result, err := tool.Execute("pump maintenance")
```

**Features:**
- Retry logic with exponential backoff
- Comprehensive logging
- Error handling

### WebSearchTool (`tools/web_search.go`)

Searches the web via Tavily API.

```go
tool := tools.NewWebSearchTool(
    "tvly-xxx...", // apiKey
    3, // maxRetries
)

result, err := tool.Execute("modern pump standards")
```

**Features:**
- Graceful degradation if API key not set
- Retry logic
- Comprehensive logging

## How the Agent Uses Tools

### In ReAct Loop

```go
// Parse action from LLM response
action := "search_pdf"
input := "pump maintenance"

// Execute via tool manager
result, err := toolMgr.Execute(action, input)

// Use result in next LLM prompt
```

### Logging

Each tool execution is logged:

```
[AGENT] Step 1: Calling tool 'search_pdf'
[TOOL] search_pdf: Querying PDF endpoint with: 'pump maintenance'
[TOOL] search_pdf: Attempt 1/3 to http://localhost:8081/api/search
[TOOL] search_pdf: Got status 200
[TOOL] search_pdf: SUCCESS - got 1250 chars
[AGENT] Step 1: Tool 'search_pdf' SUCCESS - got 1250 chars
```

## Adding New Tools

### Step 1: Create Tool File

Create `tools/new_tool.go`:

```go
package tools

import "fmt"

type MyToolConfig struct {
    // config fields
}

type MyTool struct {
    config MyToolConfig
}

func NewMyTool(/* params */) *MyTool {
    return &MyTool{
        config: MyToolConfig{
            // initialize
        },
    }
}

func (t *MyTool) Name() string {
    return "my_tool"
}

func (t *MyTool) Description() string {
    return "Does something useful"
}

func (t *MyTool) Execute(input string) (string, error) {
    // implementation
    return result, nil
}
```

### Step 2: Register in main()

```go
// In main() after creating toolMgr
toolMgr.Register(tools.NewMyTool(param1, param2))
```

### Step 3: Update System Prompt

Add to agent's system prompt:

```go
systemPrompt := `...
Available tools:
1. my_tool - Does something useful
...`
```

### Step 4: Rebuild

```bash
go build -o agent main.go
./agent
```

## Tool Error Handling

Errors are automatically handled:

```go
result, err := toolMgr.Execute("search_pdf", input)
if err != nil {
    // Tool not found or execution failed
    log.Printf("[AGENT] Tool error: %v", err)
    observation = fmt.Sprintf("Error: %v", err)
}
```

## Tool Logging

Each tool logs its execution:

- Connection attempts
- Status codes
- Retries
- Success/failure
- Data size returned

Example:
```
[TOOL] search_pdf: Attempt 1/3 to http://localhost:8081/api/search
[TOOL] search_pdf: Got status 200
[TOOL] search_pdf: SUCCESS - got 1250 chars
```

## Configuration

Tools are configured via:

1. **config.json** for endpoints and API keys
2. **main.go** for tool registration

```json
{
  "SEARCH_ENDPOINT": "http://localhost:8081/api/search",
  "TAVILY_API_KEY": "tvly-xxx...",
  "MAX_RETRIES": 3
}
```

```go
toolMgr.Register(
    tools.NewPDFSearchTool(cfg.SearchEndpoint, cfg.MaxRetries)
)
```

## Tool Listing

List all available tools:

```go
tools := toolMgr.ListTools()
for name, tool := range tools {
    fmt.Printf("%s: %s\n", name, tool.Description())
}
```

Output:
```
search_pdf: Search the user's PDF documents. Use this FIRST for any factual questions...
web_search: Search the internet. Use only if PDF search doesn't return relevant results...
```

## Benefits of This Architecture

1. **Separation of Concerns**
   - Tools are isolated in their own files
   - Main.go focuses on ReAct loop

2. **Extensibility**
   - Easy to add new tools
   - No modification to main loop needed

3. **Testability**
   - Each tool can be tested independently
   - Tool manager can be mocked

4. **Maintainability**
   - Tool logic is separate from orchestration
   - Changes to one tool don't affect others

5. **Reusability**
   - Tools can be used outside the agent
   - Tool manager is decoupled from ReAct loop

## File Organization

| File | Purpose | Lines |
|------|---------|-------|
| `tools.go` | Manager & interface | ~40 |
| `errors.go` | Error definitions | ~10 |
| `pdf_search.go` | PDF search tool | ~80 |
| `web_search.go` | Web search tool | ~80 |
| **Total** | **Tool services** | **~210** |

## Code Cleanliness

**Before:** Main.go had 605 lines with embedded tool code
**After:**
- main.go: 290 lines (ReAct loop + handlers)
- tools/: 210 lines (isolated tool services)
- Total: Same logic, better organized

## Example: Adding a Calculator Tool

```go
// tools/calculator.go
type CalculatorTool struct{}

func NewCalculatorTool() *CalculatorTool {
    return &CalculatorTool{}
}

func (t *CalculatorTool) Name() string {
    return "calculate"
}

func (t *CalculatorTool) Description() string {
    return "Perform mathematical calculations"
}

func (t *CalculatorTool) Execute(expr string) (string, error) {
    // Parse and evaluate expression
    result := eval(expr)
    return fmt.Sprintf("Result: %f", result), nil
}
```

Then in main():
```go
toolMgr.Register(tools.NewCalculatorTool())
```

Now the agent can use it:
```
Action: calculate
Action Input: 5 + 3 * 2
```

## Per-request tool gating (Your Library Knowledge Base)

As of the conversational-retrieval work, the agent no longer uses a single global
`Manager`. Instead `react.go` assembles an **active tool set per request** based on
the source toggles (`use_web`, `use_library`) sent by the UI. See PLAN §4.5 and
`docs/API_ENDPOINTS.md`.

| Toggle state | Active tools |
|---|---|
| (always) | `recall_history` — read-only conversation memory (when Mongo is up) |
| `use_library` on | + `search_pdf` (Pinecone) + `mcp` (MongoDB CRUD) |
| `use_web` on | + `web_search` (Tavily) |
| both off | only `recall_history` — model answers from its own knowledge |

`buildActiveToolSet(convID, useWeb, useLibrary)` builds a fresh `tools.Manager`
holding only the gated tools; `buildSystemPrompt(...)` branches the instructions by
toggle state (including the clarify-back rule when My Library is on).

### Memory is not a knowledge source
`recall_history` (`tools/recall_history.go`) is **always** offered when a conversation
store exists, regardless of toggles. It is read-only and conversation-scoped: the
backend builds it per request, closing over the current `conversation_id` via
`history_provider.go`, so the model can never read another conversation or
write/delete. See `docs/RECALL_HISTORY_TOOL.md`.

### Tool observations are transient
Inside one ReAct turn, tool observations (Pinecone chunks, web text) are used and then
**discarded** — only the final answer is persisted to `messages`. Memory replay uses
the budgeted summary + last-K turns, never raw observations. See PLAN §6 and
`docs/CONVERSATION_STORAGE_FORMAT.md`.

## Design Decisions

| Decision | Reason |
|----------|--------|
| Standard library + Mongo driver | Mongo driver only for deterministic conversation persistence (not the model's MCP tool) |
| Per-request tool gating | User chooses sources via toggles; keeps prompts tight and respects safety (MCP CRUD only with My Library) |
| Stateful conversations, stateless turns | History lives in Mongo; each turn rebuilds budgeted context — no in-process session state |
| External `config.json` | Config changes don't require a recompile |
| Regex-based ReAct parser | Handles LLM output variations; gracefully falls through on parse errors |
| `len/4` token heuristic | Budget context without a tokenizer dependency |
