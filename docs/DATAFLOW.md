# ReAct Agent Data Flow Diagram

Complete flow showing how user queries move through the ReAct reasoning loop with tool execution.

## High-Level Flow

```mermaid
graph TD
    A["👤 User"] -->|types query| B["🖥️ Web UI<br/>app.js"]
    B -->|POST /api/agent/query| C["📡 HTTP Request<br/>{query: string}"]
    C --> D["🎯 Agent Handler<br/>main.go"]
    D --> E["🧠 ReAct Loop<br/>runReAct()"]
    E --> F["🤖 Ollama LLM<br/>Reasoning"]
    F --> G{"Tool<br/>Decision"}
    G -->|search_pdf| H["🔍 PDF Search Tool"]
    G -->|web_search| I["🌐 Web Search Tool"]
    H --> J["📦 PDF Endpoint<br/>SSE Stream"]
    I --> K["🔗 Tavily API"]
    J --> L["📄 Parse Chunks<br/>bufio.Scanner"]
    K --> M["🔎 Web Results<br/>Top 3"]
    L --> N["📝 Observations<br/>Tool Output"]
    M --> N
    N --> O{"More<br/>Steps?"}
    O -->|Yes| F
    O -->|No| P["✨ Final Answer<br/>Aggregation"]
    P --> Q["📤 JSON Response<br/>{answer: string}"]
    Q --> R["🎨 Display Result<br/>UI Card"]
    R --> A
```

## Detailed Step-by-Step Flow

### 1️⃣ User Query Submission

```mermaid
graph TD
    A["User Types Query"] -->|'How does PDF explain X?'| B["Input Textarea<br/>agent-input"]
    B -->|Cmd+Enter or Click| C["runAgent()<br/>app.js"]
    C -->|Trim & Validate| D{Query<br/>Empty?}
    D -->|Yes| E["❌ Return early"]
    D -->|No| F["Hide error banner"]
    F --> G["setLoading(true)"]
    G -->|Show spinner| H["Fetch POST /api/agent/query"]
    H -->|JSON payload| I["HTTP Wire"]
```

### 2️⃣ Server-Side ReAct Loop

```mermaid
graph TD
    A["POST /api/agent/query<br/>{query: string}"] --> B["Load config.json"]
    B --> C["Parse MAX_STEPS<br/>MAX_RETRIES<br/>MAX_TOKENS"]
    C --> D["Initialize Tool Manager"]
    D --> E["Register Tools:<br/>search_pdf, web_search"]
    E --> F["Start ReAct Loop"]
    F --> G["Step Counter = 0"]
    G --> H{"Step < MAX_STEPS?"}
    H -->|No| I["🛑 Max steps reached<br/>Return best answer"]
    H -->|Yes| J["Build LLM Prompt:<br/>Query + History + Tools"]
    J --> K["Call Ollama API"]
```

### 3️⃣ LLM Decision & Tool Selection

```mermaid
graph TD
    A["🤖 Ollama LLM"] -->|system prompt| B["ReAct Format<br/>Thought → Action → Observation"]
    B --> C["Available Tools:<br/>1. search_pdf<br/>2. web_search"]
    C --> D["LLM Analyzes:<br/>- Query complexity<br/>- Available tools<br/>- Previous observations"]
    D --> E{"What's best?"}
    E -->|Factual about PDF| F["🔴 Action: search_pdf"]
    E -->|Need extra info| G["🟠 Action: web_search"]
    E -->|Done reasoning| H["🟢 Final Answer:<br/>Complete response"]
    F --> I["Tool Parameter:<br/>Query extraction"]
    G --> I
    H --> J["Return to frontend"]
```

### 4️⃣ PDF Search Tool Execution

```mermaid
graph TD
    A["search_pdf Tool Called"] --> B["Extract query from LLM"]
    B --> C["Prepare JSON Payload:<br/>{query: user_input}"]
    C --> D["📡 POST to PDF Endpoint<br/>http://localhost:8081/api/search"]
    D --> E{"Retry Logic<br/>MAX_RETRIES"}
    E -->|Connection Error| F["Exponential Backoff<br/>Sleep(attempt_s)"]
    F --> E
    E -->|HTTP 200 OK| G["🔄 Receive SSE Stream"]
    G --> H["bufio.Scanner<br/>Read line by line"]
    H --> I{"Line Type?"}
    I -->|'event: sources'| J["Next line: data: {...}"]
    I -->|'event: done'| K["✅ End of stream"]
    I -->|Other| H
    J --> L["Parse JSON<br/>Extract sources array"]
    L --> M["Loop through sources"]
    M --> N["Extract text_content<br/>from each source"]
    N --> O["Append to chunks[]"]
    O --> P["More sources?"]
    P -->|Yes| M
    P -->|No| Q["Join chunks:<br/>separator = '\\n---\\n'"]
    Q --> R["📝 Return combined<br/>text to LLM"]
```

### 5️⃣ Web Search Tool Execution

```mermaid
graph TD
    A["web_search Tool Called"] --> B["Extract query from LLM"]
    B --> C{"API Key<br/>Set?"}
    C -->|No| D["Return graceful<br/>fallback message"]
    C -->|Yes| E["Build Tavily URL<br/>api.tavily.com/search"]
    E --> F["Parameters:<br/>- query<br/>- max_results=3<br/>- api_key"]
    F --> G{"Retry Logic<br/>MAX_RETRIES"}
    G -->|Error| H["Exponential Backoff"]
    H --> G
    G -->|HTTP 200 OK| I["🌐 Receive JSON<br/>Response"]
    I --> J["Parse JSON<br/>Extract answer field"]
    J --> K["Extract top 3<br/>web results"]
    K --> L["📝 Return results<br/>to LLM"]
```

### 6️⃣ ReAct Loop Continuation

```mermaid
graph TD
    A["Tool Returns<br/>Observation"] --> B["Add to<br/>Thought history"]
    B --> C["Build next<br/>LLM prompt"]
    C --> D["Include:<br/>- Query<br/>- Previous thoughts<br/>- Tool observations"]
    D --> E["Call Ollama<br/>Again"]
    E --> F{"Response<br/>Type?"}
    F -->|'Action: search_pdf'| G["Execute PDF search"]
    F -->|'Action: web_search'| H["Execute web search"]
    F -->|'Final Answer:'| I["✨ Complete!"]
    G --> J["Add results to history"]
    H --> J
    J --> K["Loop back:<br/>Step++"]
    K --> L["Max steps check"]
    L -->|Continue| E
    L -->|Stop| I
    I --> M["Extract answer text<br/>after 'Final Answer:'"]
    M --> N["Return to frontend"]
```

### 7️⃣ Response & Frontend Display

```mermaid
graph TD
    A["ReAct Complete"] --> B["Build JSON Response:<br/>{answer: string}"]
    B --> C["HTTP 200 OK"]
    C -->|JSON| D["Browser Receives"]
    D --> E["Parse data.answer"]
    E --> F["createAnswerCard()"]
    F --> G["Create DOM Card:<br/>- Query display<br/>- Answer text<br/>- Styling"]
    G --> H["Prepend to results<br/>#results element"]
    H --> I["Scroll into view"]
    I --> J["setLoading(false)"]
    J -->|Update UI| K["Clear input textarea"]
    K --> L["Focus input again"]
    L --> M["Ready for next<br/>query"]
    M --> N["👤 User sees answer"]
    N --> O{"Another<br/>Question?"}
    O -->|Yes| P["Repeat from Step 1"]
    O -->|No| Q["Done"]
```

## Data Structures

### Request Payload
```json
{
  "query": "How does the PDF explain pump maintenance?"
}
```

### PDF Endpoint Response (SSE Stream)
```
event: sources
data: {"sources": [{"text_content": "...", "page": 42}, ...]}

event: done
data: {}
```

### Final API Response
```json
{
  "answer": "Based on the PDF excerpts, pump maintenance involves..."
}
```

### ReAct Loop State (Internal)
```json
{
  "step": 3,
  "max_steps": 10,
  "query": "...",
  "thoughts": ["Thought 1...", "Thought 2..."],
  "actions": ["Action: search_pdf", "Action: web_search"],
  "observations": ["Found PDF chunks...", "Web results..."],
  "current_reasoning": "LLM output..."
}
```

## Key Decision Points

| Decision | Logic |
|----------|-------|
| Tool Selection | LLM analyzes query & chooses tool based on description priority |
| Retry Decision | Automatic backoff if HTTP error (max 3 retries) |
| Loop Continuation | Check if LLM says "Final Answer" or step count exceeds MAX_STEPS |
| Graceful Degradation | If web_search key missing, return helpful message instead of error |

## Performance Notes

- **PDF Search Latency**: Depends on vector DB query time (streaming reduces timeout risk)
- **LLM Latency**: Ollama response time (respects MAX_TOKENS limit)
- **Web Search Latency**: Tavily API response (typically <1s for top 3 results)
- **Total Loop Time**: Sum of tool calls + LLM reasoning (typically 5-15s for 2-3 steps)

## Error Handling

```
User Query
    ↓
[Network Error] → Retry with backoff → [Success] OR [Final Retry] → Error Banner
    ↓
[LLM Error] → Return error message
    ↓
[Tool Failure] → Skip tool, try alternative
    ↓
[Max Steps Hit] → Return best answer so far (NOT an error)
```
