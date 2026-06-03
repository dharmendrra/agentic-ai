package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPClient holds a persistent connection to one MCP server.
type MCPClient struct {
	cli *client.Client
}

// Connect establishes the SSE connection, performs the MCP initialize handshake,
// and returns a ready-to-use MCPClient.
func Connect(ctx context.Context, serverURL string) (*MCPClient, error) {
	c, err := client.NewSSEMCPClient(serverURL + "/sse")
	if err != nil {
		return nil, fmt.Errorf("create SSE client: %w", err)
	}
	if err := c.Start(ctx); err != nil {
		return nil, fmt.Errorf("start SSE transport: %w", err)
	}

	req := mcp.InitializeRequest{}
	req.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	req.Params.ClientInfo = mcp.Implementation{Name: "agentic-ai", Version: "1.0.0"}
	if _, err := c.Initialize(ctx, req); err != nil {
		return nil, fmt.Errorf("MCP initialize: %w", err)
	}

	return &MCPClient{cli: c}, nil
}

// DiscoverTools calls tools/list and returns a slice of Tool ready to Register.
// Each tool carries its full schema from the server — no separate description string needed.
func (m *MCPClient) DiscoverTools(ctx context.Context) ([]Tool, error) {
	result, err := m.cli.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("tools/list: %w", err)
	}

	tools := make([]Tool, 0, len(result.Tools))
	for _, t := range result.Tools {
		tools = append(tools, &mcpToolCall{
			client: m,
			schema: mcpToolToSchema(t),
		})
	}
	return tools, nil
}

// Call invokes a named tool on the MCP server with JSON-encoded arguments.
func (m *MCPClient) Call(ctx context.Context, toolName, inputJSON string) (string, error) {
	var args map[string]any
	if inputJSON != "" && inputJSON != "{}" {
		if err := json.Unmarshal([]byte(inputJSON), &args); err != nil {
			return "", fmt.Errorf("input must be a JSON object: %w", err)
		}
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = toolName
	req.Params.Arguments = args

	res, err := m.cli.CallTool(ctx, req)
	if err != nil {
		return "", fmt.Errorf("tools/call %s: %w", toolName, err)
	}
	if res.IsError {
		return "", fmt.Errorf("tool error: %s", extractText(res.Content))
	}
	return extractText(res.Content), nil
}

// mcpToolCall implements the Tool interface for a single MCP server tool.
type mcpToolCall struct {
	schema ToolSchema
	client *MCPClient
}

func (t *mcpToolCall) Name() string        { return t.schema.Name }
func (t *mcpToolCall) Schema() ToolSchema  { return t.schema }

func (t *mcpToolCall) Execute(input string) (string, error) {
	return t.client.Call(context.Background(), t.schema.Name, input)
}

// extractText pulls the first TextContent out of a tool result.
func extractText(content []mcp.Content) string {
	for _, c := range content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

// mcpToolToSchema converts an mcp.Tool definition to the standard ToolSchema.
func mcpToolToSchema(t mcp.Tool) ToolSchema {
	data, _ := json.Marshal(t.InputSchema)
	var inputSchema map[string]any
	json.Unmarshal(data, &inputSchema)
	return ToolSchema{
		Name:        t.Name,
		Description: t.Description,
		InputSchema: inputSchema,
	}
}
