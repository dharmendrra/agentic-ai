---
layout: default
title: Architecture
---

# Architecture

The system is two independent Go services wired together over the Model Context Protocol (MCP):

- **`agents/`** — the ReAct agent (LLM brain + tools)
- **`mcp/`** — a standalone MongoDB MCP server

The agent treats MCP as a black-box protocol: it knows only that there is **one** `mcp` tool, and discovers what that server can do at runtime via the standard `tools/list` action.

---

## System Overview

```mermaid
graph TD
    subgraph agentic-ai
        subgraph agents
            A[ReAct Agent<br/>:8082]
            T1[search_pdf tool]
            T2[web_search tool]
            T3[mcp tool<br/>single umbrella]
        end

        subgraph mcp
            M[MCP Server<br/>:8083 / SSE]
            DB[(MongoDB<br/>agentic_mcps)]
            C1[learning_todo]
            C2[links_tracker]
            C3[job_portals]
        end
    end

    LLM[LLM Backend<br/>Anthropic Claude or Ollama] -->|ReAct loop| A
    A --> T1 --> PDF[PDF Search<br/>:8081]
    A --> T2 --> Web[Tavily Web Search]
    A --> T3 -->|MCP over SSE| M
    M -->|CRUD| DB
    DB --> C1
    DB --> C2
    DB --> C3
```

---

## How a Query Flows

```mermaid
sequenceDiagram
    participant U as User
    participant A as Agent :8082
    participant L as LLM (Claude / Ollama)
    participant M as MCP Server :8083
    participant DB as MongoDB

    U->>A: POST /api/agent/query
    A->>M: initialize (handshake)

    loop ReAct steps
        A->>L: Thought prompt
        L-->>A: Action + Action Input
        alt search_pdf / web_search
            A->>A: call local tool
        else mcp (action: list_tools)
            A->>M: tools/list
            M-->>A: available operations
        else mcp (action: query_documents, ...)
            A->>M: tools/call
            M->>DB: CRUD
            DB-->>M: result
            M-->>A: CallToolResult
        end
        A->>L: Observation
    end

    L-->>A: Final Answer
    A-->>U: JSON response
```

---

## Design Principles

| Principle | What it means |
|---|---|
| **MCP as a black box** | The agent has a single `mcp` tool. It does not pre-register individual MongoDB operations — it discovers them at query time via `action: list_tools`. |
| **Tool schema registry** | Every tool owns its complete definition (`name`, `description`, `input_schema`) through a `Schema()` method. The registry compiles these into the system prompt — no hardcoded prompt text. |
| **Pluggable LLM backend** | Ollama by default; Anthropic Claude when configured. Both implement the same `LLMCaller` interface. |
| **Standard protocol** | The MCP server speaks plain MCP over SSE — any client (Claude Desktop, Cursor, custom agent) can connect with no special coupling. |

---

## Ports

| Service | Port | Endpoint |
|---|---|---|
| Agent | `8082` | `POST /api/agent/query`, UI at `/` |
| MCP server | `8083` | SSE at `/sse` |
| PDF search | `8081` | `POST /api/search` (external) |

---

See **[Agents](./agents)** and **[MCP](./mcp)** for module-level detail.
