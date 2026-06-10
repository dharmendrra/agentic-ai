package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// handleAgentQuery runs one conversational turn:
// parse → ensure conversation → persist user msg → ReAct → persist assistant msg
// → maybe summarize → respond. See PLAN §4.6.
func handleAgentQuery(w http.ResponseWriter, r *http.Request) {
	log.Printf("[API] POST /api/agent/query received")

	if r.Method != http.MethodPost {
		log.Printf("[API] ERROR: Invalid method %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[API] ERROR: Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Query) == "" {
		log.Printf("[API] ERROR: Query is empty")
		http.Error(w, "Query is required", http.StatusBadRequest)
		return
	}

	log.Printf("[API] query=%q conv=%q web=%v library=%v", req.Query, req.ConversationID, req.UseWeb, req.UseLibrary)

	convID := req.ConversationID
	title := ""

	// Ensure a conversation exists (only if the store is available).
	if store != nil {
		if convID == "" {
			title = makeTitle(req.Query)
			id, err := store.CreateConversation(title)
			if err != nil {
				log.Printf("[API] ERROR: create conversation: %v", err)
				http.Error(w, "Failed to create conversation", http.StatusInternalServerError)
				return
			}
			convID = id
			log.Printf("[API] Created conversation %s (%q)", convID, title)
		} else if conv, _ := store.GetConversation(convID); conv != nil {
			title = conv.Title
		}

		if _, err := store.AppendMessage(convID, "user", req.Query, nil, nil); err != nil {
			log.Printf("[API] WARN: append user message: %v", err)
		}
	}

	answer, sources, citations, needsClarify, err := runReAct(convID, req.Query, req.UseWeb, req.UseLibrary)
	if err != nil {
		log.Printf("[API] ERROR: ReAct failed: %v", err)
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}

	if store != nil {
		if _, err := store.AppendMessage(convID, "assistant", answer, sources, citations); err != nil {
			log.Printf("[API] WARN: append assistant message: %v", err)
		}
		maybeSummarize(convID)
	}

	log.Printf("[API] SUCCESS: answer=%d chars sources=%v clarify=%v", len(answer), sources, needsClarify)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AgentResponse{
		Answer:             answer,
		ConversationID:     convID,
		Title:              title,
		Sources:            sources,
		Citations:          citations,
		NeedsClarification: needsClarify,
	})
}

// handleConversations lists conversations for the sidebar (GET /api/conversations).
func handleConversations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if store == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Conversation{})
		return
	}
	convos, err := store.ListConversations()
	if err != nil {
		log.Printf("[API] ERROR: list conversations: %v", err)
		http.Error(w, "Failed to list conversations", http.StatusInternalServerError)
		return
	}
	if convos == nil {
		convos = []Conversation{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(convos)
}

// handleConversationByID returns a conversation's full message list (GET) or
// deletes it (DELETE): /api/conversations/{id}.
func handleConversationByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/conversations/")
	id = strings.Trim(id, "/")
	if id == "" {
		http.Error(w, "Conversation id required", http.StatusBadRequest)
		return
	}
	if store == nil {
		http.Error(w, "Conversation memory unavailable", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		conv, err := store.GetConversation(id)
		if err != nil {
			log.Printf("[API] ERROR: get conversation: %v", err)
			http.Error(w, "Failed to load conversation", http.StatusInternalServerError)
			return
		}
		if conv == nil {
			http.Error(w, "Conversation not found", http.StatusNotFound)
			return
		}
		msgs, err := store.GetConversationMessages(id)
		if err != nil {
			log.Printf("[API] ERROR: get messages: %v", err)
			http.Error(w, "Failed to load messages", http.StatusInternalServerError)
			return
		}
		if msgs == nil {
			msgs = []Message{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"conversation": conv,
			"messages":     msgs,
		})
	case http.MethodDelete:
		if err := store.DeleteConversation(id); err != nil {
			log.Printf("[API] ERROR: delete conversation: %v", err)
			http.Error(w, "Failed to delete conversation", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func serveStaticFile(filePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filePath)
	}
}
