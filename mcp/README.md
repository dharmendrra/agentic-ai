# agentic-mcps — MongoDB MCP Server

A standalone [Model Context Protocol](https://modelcontextprotocol.io) server written in Go that exposes a MongoDB database as MCP tools. Any MCP-compatible client (Claude Desktop, Cursor, custom agents) can connect to it.

The server is **general-purpose**: it stores whatever you want. The tools operate on any collection and any document shape — nothing is hardcoded to a particular schema. The collections listed below are just the example data currently in this database.

---

## Flow

```mermaid
sequenceDiagram
    participant C as MCP Client<br/>(Claude / Cursor / Agent)
    participant S as MCP Server<br/>:8083
    participant M as MongoDB<br/>agentic_mcps

    C->>S: GET /sse (open SSE stream)
    S-->>C: event: endpoint<br/>data: /message?sessionId=xxx

    C->>S: POST /message — initialize
    S-->>C: serverInfo + capabilities

    C->>S: POST /message — tools/list
    S-->>C: [list_collections, query_documents,<br/>insert_document, update_document,<br/>delete_document]

    Note over C,S: LLM picks a tool and calls it

    C->>S: POST /message — tools/call<br/>{"name":"query_documents","arguments":{...}}
    S->>M: db.Collection(...).Find(filter)
    M-->>S: []documents
    S-->>C: CallToolResult (JSON)
```

---

## Architecture

```mermaid
graph TD
    subgraph Clients
        A[Claude Desktop]
        B[Cursor]
        C[Custom Agent]
    end

    subgraph MCP Server :8083
        E[SSE Transport<br/>GET /sse]
        F[Message Handler<br/>POST /message]
        G[Tool Registry<br/>list_collections<br/>query_documents<br/>insert_document<br/>update_document<br/>delete_document]
    end

    subgraph MongoDB
        H[(agentic_mcps)]
        I[collection A]
        J[collection B]
        K[collection C]
    end

    A -->|MCP over SSE| E
    B -->|MCP over SSE| E
    C -->|MCP over SSE| E

    E --> F
    F --> G
    G -->|CRUD| H
    H --> I
    H --> J
    H --> K
```

---

## Quick Start

```bash
# 1. Make sure MongoDB is running
mongosh --eval "db.adminCommand({ping:1})"

# 2. Run the server
cd mcp
go run .
# [MCP] Server starting on http://localhost:8083
# [MCP] SSE endpoint: http://localhost:8083/sse
```

---

## Configuration

Copy the example to create your local config (`config.json` is gitignored):

```bash
cp config.example.json config.json
```

`config.json`:
```json
{
  "MONGO_URI": "mongodb://localhost:27017",
  "PORT": "8083"
}
```

---

## Tools

All tools are discovered dynamically by clients via `tools/list`.

| Tool | Description | Required inputs |
|---|---|---|
| `list_collections` | List all collections with document counts | — |
| `query_documents` | Query documents with optional filter | `collection` |
| `insert_document` | Insert a new document | `collection`, `document` |
| `update_document` | Update matching documents | `collection`, `filter`, `update` |
| `delete_document` | Delete matching documents | `collection`, `filter` |

### Example inputs

```jsonc
// query_documents
{"collection": "job_portals", "filter": {"Category": "Free"}, "limit": 10}

// insert_document
{"collection": "learning_todo", "document": {"Name": "Study MCP", "Status": "To Do"}}

// update_document
{"collection": "learning_todo", "filter": {"Name": "Study MCP"}, "update": {"Status": "Done"}}

// delete_document
{"collection": "learning_todo", "filter": {"Name": "Study MCP"}}
```

---

## Connecting clients

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:
```json
{
  "mcpServers": {
    "agentic-mcps": {
      "url": "http://localhost:8083/sse"
    }
  }
}
```

### Cursor

Add to `.cursor/mcp.json` in your project:
```json
{
  "mcpServers": {
    "agentic-mcps": {
      "url": "http://localhost:8083/sse"
    }
  }
}
```

### Any MCP client

SSE endpoint: `http://localhost:8083/sse`

The server speaks standard MCP over SSE — no custom headers or auth required.

---

## Example Collections

These are the collections currently in this database — illustrative examples, not a fixed schema. Add any collection you like and the same tools work unchanged.

| Collection | Documents | Schema |
|---|---|---|
| `learning_todo` | 16 | `Name`, `Assign`, `Status`, `Target end date` |
| `links_tracker` | 26 | `Task name`, `Assignee`, `Date`, `Description`, `Due date`, `Effort level`, `Priority`, `Status`, `Task type`, `URL` |
| `job_portals` | 68 | `Name`, `Category`, `Link`, `Link Type`, `Remarks/Findings`, `Web Description` |
