package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RecalledMessage is one past message returned by a HistoryProvider.
// It mirrors the store's Message but keeps the tools package decoupled from
// package main (no import cycle): the backend passes in an implementation.
type RecalledMessage struct {
	Seq     int
	Role    string
	Content string
}

// HistoryProvider is the read-only, conversation-scoped view the recall_history
// tool is allowed to use. The backend implements this over its Mongo store and
// closes over a fixed conversation id — the model can never target another
// conversation or perform writes/deletes.
type HistoryProvider interface {
	AllUserMessages() ([]RecalledMessage, error)
	FirstNUserMessages(n int) ([]RecalledMessage, error)
	Search(query string) ([]RecalledMessage, error)
}

// RecallHistoryTool is a read-only memory tool scoped to one conversation.
// It is NOT a knowledge source and NEVER exposes write/delete operations.
// A fresh instance is built per request (see backend) because it is
// conversation-scoped.
type RecallHistoryTool struct {
	provider HistoryProvider
}

// NewRecallHistoryTool builds the tool over a conversation-scoped provider.
func NewRecallHistoryTool(provider HistoryProvider) *RecallHistoryTool {
	return &RecallHistoryTool{provider: provider}
}

func (t *RecallHistoryTool) Name() string { return "recall_history" }

func (t *RecallHistoryTool) Schema() ToolSchema {
	return ToolSchema{
		Name:        "recall_history",
		Description: "Read-only memory of the CURRENT conversation. Use it to answer questions about what was said earlier (e.g. \"what did I ask first\", \"summarize our chat\"). It returns past messages exactly as stored — it cannot write or delete anything.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"description": "One of: 'all' (all of your past questions), 'first_n' (your first N questions), 'search' (questions/answers containing a phrase).",
				},
				"n": map[string]any{
					"type":        "integer",
					"description": "Number of messages for mode 'first_n'.",
				},
				"query": map[string]any{
					"type":        "string",
					"description": "Phrase to look for, required for mode 'search'.",
				},
			},
			"required": []string{"mode"},
		},
	}
}

// Execute parses the JSON input and returns matching past messages as text.
func (t *RecallHistoryTool) Execute(input string) (string, error) {
	if t.provider == nil {
		return "Conversation memory is unavailable.", nil
	}

	var req struct {
		Mode  string `json:"mode"`
		N     int    `json:"n"`
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return "", fmt.Errorf("invalid recall_history input (expected JSON object): %w", err)
	}

	var (
		msgs []RecalledMessage
		err  error
	)
	switch strings.ToLower(strings.TrimSpace(req.Mode)) {
	case "all":
		msgs, err = t.provider.AllUserMessages()
	case "first_n":
		n := req.N
		if n <= 0 {
			n = 1
		}
		msgs, err = t.provider.FirstNUserMessages(n)
	case "search":
		if strings.TrimSpace(req.Query) == "" {
			return "", fmt.Errorf("mode 'search' requires a non-empty 'query'")
		}
		msgs, err = t.provider.Search(req.Query)
	default:
		return "", fmt.Errorf("unknown mode %q (use 'all', 'first_n', or 'search')", req.Mode)
	}
	if err != nil {
		return "", err
	}

	if len(msgs) == 0 {
		return "No matching messages in this conversation.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d past message(s):\n", len(msgs)))
	for _, m := range msgs {
		sb.WriteString(fmt.Sprintf("[#%d %s] %s\n", m.Seq, m.Role, m.Content))
	}
	return sb.String(), nil
}
