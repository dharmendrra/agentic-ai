package main

type AgentRequest struct {
	Query string `json:"query"`
}

type AgentResponse struct {
	Answer string `json:"answer"`
}

type ReActState struct {
	Steps  int
	Answer string
}
