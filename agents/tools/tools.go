package tools

import (
	"fmt"
	"sort"
	"strings"
)

// ToolSchema is the standard JSON schema format for a tool,
// compatible with OpenAI, Anthropic, and MCP tool definitions.
type ToolSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// Tool represents a tool that can be used by the agent.
type Tool interface {
	Name() string
	Schema() ToolSchema
	Execute(input string) (string, error)
}

// Manager is the tool registry — it holds all registered tools and compiles their schemas.
type Manager struct {
	tools map[string]Tool
	order []string // preserves registration order
}

func NewManager() *Manager {
	return &Manager{tools: make(map[string]Tool)}
}

func (m *Manager) Register(tool Tool) {
	if _, exists := m.tools[tool.Name()]; !exists {
		m.order = append(m.order, tool.Name())
	}
	m.tools[tool.Name()] = tool
}

func (m *Manager) Execute(name, input string) (string, error) {
	tool, exists := m.tools[name]
	if !exists {
		return "", ErrToolNotFound(name)
	}
	return tool.Execute(input)
}

func (m *Manager) GetTool(name string) (Tool, bool) {
	tool, exists := m.tools[name]
	return tool, exists
}

func (m *Manager) ListTools() map[string]Tool {
	return m.tools
}

// Schemas returns all tool schemas in registration order — the compiled registry.
func (m *Manager) Schemas() []ToolSchema {
	schemas := make([]ToolSchema, 0, len(m.order))
	for _, name := range m.order {
		schemas = append(schemas, m.tools[name].Schema())
	}
	return schemas
}

// BuildPromptSection formats all tool schemas as a text block for the system prompt.
func (m *Manager) BuildPromptSection() string {
	var sb strings.Builder
	for _, s := range m.Schemas() {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, s.Description))
		if props := formatInputSchema(s.InputSchema); props != "" {
			sb.WriteString(fmt.Sprintf("  Input: %s\n", props))
		}
	}
	return sb.String()
}

// formatInputSchema renders the required/optional params from a JSON schema map
// as a compact one-line string for the system prompt.
func formatInputSchema(schema map[string]any) string {
	if schema == nil {
		return ""
	}
	props, _ := schema["properties"].(map[string]any)
	if len(props) == 0 {
		return ""
	}
	reqList, _ := schema["required"].([]any)
	reqSet := make(map[string]bool, len(reqList))
	for _, r := range reqList {
		if s, ok := r.(string); ok {
			reqSet[s] = true
		}
	}
	var parts []string
	for name, v := range props {
		prop, _ := v.(map[string]any)
		typ, _ := prop["type"].(string)
		prefix := ""
		if reqSet[name] {
			prefix = "*"
		}
		parts = append(parts, fmt.Sprintf("%s%s(%s)", prefix, name, typ))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}
