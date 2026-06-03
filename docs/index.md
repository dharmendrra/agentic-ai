---
layout: default
title: agentic-ai
---

# agentic-ai

A local agentic AI system built in Go. An LLM-powered ReAct agent that reasons over PDF documents, the web, and a MongoDB database — all wired together via the Model Context Protocol (MCP).

The agent runs fully local on Ollama by default, and can transparently switch to Anthropic Claude when an API key with credits is configured — **no code change required**.

---

## Key Features

- **ReAct Agent Loop** — Reason → Plan → Act → Observe → Repeat
- **Dual LLM Backend** — Local Ollama by default, Anthropic Claude with fallback
- **Dynamic Tool Discovery** — Tools are discovered at runtime via MCP protocol
- **MongoDB Integration** — Full CRUD operations via MCP server
- **Web Search** — Tavily API integration for live web queries
- **PDF Search** — Vector search over PDF documents
- **Tool Schema Registry** — Each tool owns its complete definition

---

## Repository Structure

```
agentic-ai/
├── agents/   — ReAct agent (Ollama/Anthropic + PDF search + web search + MCP client)
└── mcp/      — MongoDB MCP server (exposes agentic_mcps DB as MCP tools)
```

---

## Quick Links

- **[Architecture](./architecture)** — System design and data flow
- **[Agents Module](./agents)** — ReAct loop, tool registry, LLM backend
- **[MCP Module](./mcp)** — MongoDB server, SSE transport, protocol
- **[Quick Start](./quickstart)** — Get up and running in 5 minutes

---

## Prerequisites

| Dependency | Purpose |
|---|---|
| Go 1.21+ | Build both modules |
| MongoDB | Backing store for MCP server |
| Ollama | Local LLM inference (default backend) |
| Anthropic API key | Optional — cloud LLM backend (Claude) |
| Tavily API key | Web search fallback |
| PDF search endpoint | Optional — `http://localhost:8081/api/search` |

---

## Get Started

See [Quick Start](./quickstart) for detailed setup instructions.

```bash
# 1. Clone and configure
git clone https://github.com/dharmendrra/agentic-ai.git
cd agentic-ai
cp agents/config.example.json agents/config.json
cp mcp/config.example.json mcp/config.json

# 2. Start MongoDB and MCP server
mongosh --eval "db.adminCommand({ping:1})"
cd mcp && go run .

# 3. Start the agent
cd agents && go run .

# 4. Query the agent
curl -X POST http://localhost:8082/api/agent/query \
  -H "Content-Type: application/json" \
  -d '{"query": "What is stored in my database?"}'
```

---

## License

Same as parent project.
