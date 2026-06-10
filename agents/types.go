package main

type AgentRequest struct {
	Query          string `json:"query"`
	ConversationID string `json:"conversation_id"` // empty => new conversation
	UseWeb         bool   `json:"use_web"`
	UseLibrary     bool   `json:"use_library"`
}

// Citation is a clickable web source surfaced from web_search (Tavily results).
type Citation struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type AgentResponse struct {
	Answer             string     `json:"answer"`
	ConversationID     string     `json:"conversation_id"`
	Title              string     `json:"title"`
	Sources            []string   `json:"sources,omitempty"`
	Citations          []Citation `json:"citations,omitempty"`
	NeedsClarification bool       `json:"needs_clarification,omitempty"`
}

type ReActState struct {
	Steps  int
	Answer string
}
