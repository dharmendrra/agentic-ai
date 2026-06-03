package tools

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Tool represents a tool that can be used by the agent
type Tool interface {
	Execute(input string) (string, error)
	Name() string
	Description() string
}

// Manager manages available tools
type Manager struct {
	tools map[string]Tool
}

// NewManager creates a new tool manager
func NewManager() *Manager {
	return &Manager{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the manager
func (m *Manager) Register(tool Tool) {
	m.tools[tool.Name()] = tool
}

// Execute runs a tool by name
func (m *Manager) Execute(name, input string) (string, error) {
	tool, exists := m.tools[name]
	if !exists {
		return "", ErrToolNotFound(name)
	}
	return tool.Execute(input)
}

// GetTool returns a tool by name
func (m *Manager) GetTool(name string) (Tool, bool) {
	tool, exists := m.tools[name]
	return tool, exists
}

// ListTools returns all available tools
func (m *Manager) ListTools() map[string]Tool {
	return m.tools
}
