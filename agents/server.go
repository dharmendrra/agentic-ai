package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

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

	if req.Query == "" {
		log.Printf("[API] ERROR: Query is empty")
		http.Error(w, "Query is required", http.StatusBadRequest)
		return
	}

	log.Printf("[API] Processing query: %s", req.Query)
	answer, err := runReAct(req.Query)
	if err != nil {
		log.Printf("[API] ERROR: ReAct failed: %v", err)
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("[API] SUCCESS: Returning answer (%d chars)", len(answer))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AgentResponse{Answer: answer})
}

func serveStaticFile(filePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filePath)
	}
}
