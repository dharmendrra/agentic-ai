package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

// DiscoverTools calls tools/list on the server and returns:
//   - a slice of Tool (ready to Register on a Manager)
//   - a formatted description string for the LLM system prompt
func (m *MCPClient) DiscoverTools(ctx context.Context) ([]Tool, string, error) {
	result, err := m.cli.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, "", fmt.Errorf("tools/list: %w", err)
	}

	var tools []Tool
	var sb strings.Builder

	for _, t := range result.Tools {
		tools = append(tools, &mcpToolCall{
			toolName: t.Name,
			client:   m,
		})
		sb.WriteString(fmt.Sprintf("- %s: %s\n", t.Name, t.Description))
		if props := inputSchemaProps(t); props != "" {
			sb.WriteString(fmt.Sprintf("  Input: %s\n", props))
		}
	}

	return tools, sb.String(), nil
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
	toolName string
	client   *MCPClient
}

func (t *mcpToolCall) Name() string        { return t.toolName }
func (t *mcpToolCall) Description() string { return "" } // description comes from server via DiscoverTools

func (t *mcpToolCall) Execute(input string) (string, error) {
	return t.client.Call(context.Background(), t.toolName, input)
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

// inputSchemaProps formats the required properties of a tool's input schema
// into a compact one-line description for the system prompt.
func inputSchemaProps(t mcp.Tool) string {
	data, err := json.Marshal(t.InputSchema)
	if err != nil {
		return ""
	}
	var schema struct {
		Properties map[string]struct {
			Type        string `json:"type"`
			Description string `json:"description"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(data, &schema); err != nil || len(schema.Properties) == 0 {
		return ""
	}

	var parts []string
	for name, prop := range schema.Properties {
		required := false
		for _, r := range schema.Required {
			if r == name {
				required = true
				break
			}
		}
		req := ""
		if required {
			req = "*"
		}
		parts = append(parts, fmt.Sprintf("%s%s(%s)", req, name, prop.Type))
	}
	return strings.Join(parts, ", ")
}
