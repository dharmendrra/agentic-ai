---
layout: default
title: Quick Start
---

# Quick Start

Get the full system running in about five minutes.

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

## 1. Configure

`config.json` files are gitignored because they contain API keys. Copy the examples and fill in your keys:

```bash
cp agents/config.example.json agents/config.json
cp mcp/config.example.json   mcp/config.json
# then fill in your keys (Tavily, optionally Anthropic) in agents/config.json
```

---

## 2. Start MongoDB

```bash
mongosh --eval "db.adminCommand({ping:1})"
```

---

## 3. Start the MCP Server

```bash
cd mcp && go run .
# [MCP] Server starting on http://localhost:8083
# [MCP] SSE endpoint: http://localhost:8083/sse
```

---

## 4. Start the Agent

```bash
cd agents && go run .
# [LLM] Backend: Ollama (gemma4:e4b)
# [MCP] Registered MCP tool (LLM will discover resources dynamically)
# Agent server listening on http://localhost:8082
```

---

## 5. Query the Agent

```bash
curl -X POST http://localhost:8082/api/agent/query \
  -H "Content-Type: application/json" \
  -d '{"query": "Show me all free job portals"}'
```

Or open the web UI at [http://localhost:8082](http://localhost:8082).

---

## Switching to Anthropic Claude

To use Claude instead of Ollama, set all three in `agents/config.json`:

```jsonc
"ANTHROPIC_API_KEY": "sk-ant-...",
"ANTHROPIC_MODEL":   "claude-haiku-4-5-20251001",
"ANTHROPIC_CREDIT_BALANCE": true
```

Flip `ANTHROPIC_CREDIT_BALANCE` to `false` to force Ollama without removing the key.

---

## Troubleshooting

| Symptom | Fix |
|---|---|
| `static directory not found` | Run the agent from inside the `agents/` directory |
| `Could not connect to MCP server` | Make sure the MCP server is running on `:8083` first |
| Agent uses Ollama when you expected Claude | Check all three Anthropic keys are set and `ANTHROPIC_CREDIT_BALANCE: true` |
| `config.json not found` | Copy from `config.example.json` |

---

See **[Architecture](./architecture)** for the full system design.
