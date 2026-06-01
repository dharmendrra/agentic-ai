# Logging & Debugging Guide

The agent now includes comprehensive logging to track execution. Watch the terminal where you run `./agent` to see detailed logs.

## Log Prefixes

Each log line has a prefix showing the component:

| Prefix | Component |
|--------|-----------|
| `[API]` | HTTP endpoint handling |
| `[AGENT]` | ReAct loop orchestration |
| `[TOOL]` | Tool execution (search_pdf, web_search) |
| `[OLLAMA]` | LLM API calls |

## Example Output

When you make a query, you'll see:

```
[API] POST /api/agent/query received
[API] Processing query: How does the PDF explain pump maintenance?
[AGENT] Starting ReAct loop for query: How does the PDF explain pump maintenance?
[AGENT] Step 1 of 10
[AGENT] Calling Ollama model: gemma4:e4b
[OLLAMA] Calling Ollama at http://localhost:11434 with model 'gemma4:e4b' (max_tokens=1024)
[OLLAMA] Request successful, parsing response...
[AGENT] LLM Response (step 1): Thought: The user is asking about pump maintenance. Let me search the PDF first...
[AGENT] Step 1: Action = 'search_pdf', Input = 'pump maintenance'
[AGENT] Step 1: Calling search_pdf tool
[TOOL] search_pdf: Querying PDF endpoint with: 'pump maintenance'
[TOOL] search_pdf: Attempt 1/3 to http://localhost:8081/api/search
[TOOL] search_pdf: Got status 200
[TOOL] search_pdf: SUCCESS - got 1250 chars
[AGENT] Step 1: search_pdf SUCCESS - got 1250 chars
[AGENT] Step 2 of 10
[AGENT] Calling Ollama model: gemma4:e4b
[OLLAMA] Calling Ollama at http://localhost:11434 with model 'gemma4:e4b' (max_tokens=1024)
[OLLAMA] Request successful, parsing response...
[AGENT] LLM Response (step 2): Final Answer: The PDF explains pump maintenance in 5 steps...
[AGENT] Found 'Final Answer' - exiting loop
[AGENT] Reached max steps (10) - returning best answer found
[API] SUCCESS: Returning answer (450 chars)
```

## What Each Log Tells You

### API Logs
```
[API] POST /api/agent/query received           → Request received
[API] Processing query: ...                    → Query being processed
[API] SUCCESS: Returning answer (X chars)      → Successfully completed
[API] ERROR: ...                               → Error occurred
```

### Agent Logs
```
[AGENT] Starting ReAct loop for query: ...     → Loop starting
[AGENT] Step N of M                            → Which step we're on
[AGENT] Calling Ollama model: ...              → About to call LLM
[AGENT] LLM Response (step N): ...             → What LLM returned (first 200 chars)
[AGENT] Step N: Action = 'X', Input = 'Y'     → Parsed action and input
[AGENT] Step N: Calling search_pdf tool       → About to call search_pdf
[AGENT] Step N: search_pdf SUCCESS - got X chars → Tool returned data
[AGENT] Found 'Final Answer' - exiting loop   → Loop completed with answer
[AGENT] Step N: Could not parse Action/Input  → LLM format issue
[AGENT] Reached max steps (N) - returning...  → Max steps hit
```

### Tool Logs

**search_pdf:**
```
[TOOL] search_pdf: Querying PDF endpoint with: '...'        → Starting search
[TOOL] search_pdf: Attempt N/M to http://localhost:8081     → Retry attempt
[TOOL] search_pdf: Got status 200                           → HTTP response
[TOOL] search_pdf: SUCCESS - got X chars                    → Data received
[TOOL] search_pdf: ERROR: ...                               → Failed
```

**web_search:**
```
[TOOL] web_search: Querying Tavily API with: '...'          → Starting web search
[TOOL] web_search: SKIPPED - TAVILY_API_KEY not configured  → Web search disabled
[TOOL] web_search: Attempt N/M                              → Retry attempt
[TOOL] web_search: Got status 200                           → HTTP response
[TOOL] web_search: SUCCESS - got X chars                    → Data received
```

### Ollama Logs
```
[OLLAMA] Calling Ollama at http://localhost:11434 with model 'gemma4:e4b' (max_tokens=1024)
                                                      → API call with config
[OLLAMA] Request successful, parsing response...    → Got response
[OLLAMA] ERROR: Connection failed: ...              → Network error
[OLLAMA] ERROR: Status 500 returned                 → HTTP error
```

## Common Scenarios

### Scenario 1: search_pdf tool is called

```
[AGENT] Step 1: Action = 'search_pdf', Input = 'pump maintenance'
[AGENT] Step 1: Calling search_pdf tool
[TOOL] search_pdf: Querying PDF endpoint with: 'pump maintenance'
[TOOL] search_pdf: Attempt 1/3 to http://localhost:8081/api/search
[TOOL] search_pdf: Got status 200
[TOOL] search_pdf: SUCCESS - got 1250 chars
[AGENT] Step 1: search_pdf SUCCESS - got 1250 chars
```

**What it means:** Agent decided to search PDF, call succeeded, got 1250 characters of data.

### Scenario 2: web_search tool is called

```
[AGENT] Step 2: Action = 'web_search', Input = 'modern pump standards'
[AGENT] Step 2: Calling web_search tool
[TOOL] web_search: Querying Tavily API with: 'modern pump standards'
[TOOL] web_search: Attempt 1/3
[TOOL] web_search: Got status 200
[TOOL] web_search: SUCCESS - got 890 chars
[AGENT] Step 2: web_search SUCCESS - got 890 chars
```

**What it means:** Agent decided to search web after PDF wasn't sufficient. Web search succeeded.

### Scenario 3: web_search is skipped

```
[TOOL] web_search: SKIPPED - TAVILY_API_KEY not configured
```

**What it means:** Web search is disabled because TAVILY_API_KEY is not set in config.json.

### Scenario 4: Tool fails with retry

```
[TOOL] search_pdf: Attempt 1/3 to http://localhost:8081/api/search
[TOOL] search_pdf: Connection error on attempt 1: connection refused
[TOOL] search_pdf: Retrying after status 500...
[TOOL] search_pdf: Attempt 2/3 to http://localhost:8081/api/search
[TOOL] search_pdf: Got status 200
[TOOL] search_pdf: SUCCESS - got 450 chars
```

**What it means:** First attempt failed, but succeeded on retry (automatically handled by max retries logic).

### Scenario 5: LLM formatting issue

```
[AGENT] Step 1: No Action/Input found in response
```

**What it means:** LLM didn't respond in expected ReAct format. Check:
- System prompt clarity
- LLM model capability
- Response content in the logs

### Scenario 6: Final answer found

```
[AGENT] LLM Response (step 2): Final Answer: The PDF explains pump maintenance in 5 steps...
[AGENT] Found 'Final Answer' - exiting loop
```

**What it means:** LLM generated a final answer and loop is exiting.

## Troubleshooting with Logs

### Problem: Endpoint at localhost:8081 not found
```
[TOOL] search_pdf: Attempt 1/3 to http://localhost:8081/api/search
[TOOL] search_pdf: Connection error on attempt 1: connection refused
```
**Solution:** Make sure your PDF search API is running on port 8081.

### Problem: Ollama not responding
```
[OLLAMA] ERROR: Connection failed: connection refused
```
**Solution:** Start Ollama: `ollama serve`

### Problem: Tool never called
```
[AGENT] Step N: No Action/Input found in response
```
**Solution:** LLM isn't generating proper ReAct format. Check:
- Is the LLM model good enough? (try `gemma4:e4b` or larger)
- Is the system prompt clear? (see main.go line ~213)
- What was the actual LLM response? (shown in logs)

### Problem: Max steps reached
```
[AGENT] Reached max steps (10) - returning best answer found
```
**Solution:** Increase MAX_STEPS in config.json, or improve system prompt clarity.

## Viewing Logs in Real-Time

### On Local Machine

```bash
# Run agent in one terminal
./agent

# Logs appear in same terminal as the server runs
```

### In Production

```bash
# Run in background and log to file
./agent > agent.log 2>&1 &

# Watch logs in real-time
tail -f agent.log

# Search logs for errors
grep ERROR agent.log

# See all Ollama calls
grep "\[OLLAMA\]" agent.log

# See all tool calls
grep "\[TOOL\]" agent.log
```

### With Systemd

```bash
# View logs
journalctl -u agentic-ai -f

# View last 100 lines
journalctl -u agentic-ai -n 100

# View errors only
journalctl -u agentic-ai | grep ERROR
```

## Log Levels

Currently using simple prefixes. Could be enhanced with severity levels if needed:

```
[INFO] - Informational messages
[WARN] - Warnings (e.g., retries)
[ERROR] - Errors that affect functionality
[DEBUG] - Detailed debugging info
```

(Currently all logs are INFO level)

## Performance Insights from Logs

You can calculate timing by looking at log timestamps:

```
[AGENT] Step 1 of 10          ← Time A
[OLLAMA] Request successful   ← Time B
[AGENT] Step 2 of 10          ← Time C
```

- Step 1 processing time: B - A
- Total step time: C - A

## Summary

The logging helps you see:

1. **What's happening** - Each step of the ReAct loop
2. **Which tools are called** - search_pdf, web_search, or neither
3. **What the LLM says** - First 200 chars of response
4. **Errors and retries** - Connection issues, status codes
5. **Performance** - Via timestamps

Use the logs to understand the agent's reasoning process and debug any issues!
