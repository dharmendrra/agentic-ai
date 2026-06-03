package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mark3labs/mcp-go/server"
	"github.com/user/agentic-ai/mcp/db"
	mcptools "github.com/user/agentic-ai/mcp/tools"
)

type Config struct {
	MongoURI string `json:"MONGO_URI"`
	Port     string `json:"PORT"`
}

func main() {
	data, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatalf("config.json not found: %v", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("invalid config.json: %v", err)
	}
	if cfg.Port == "" {
		cfg.Port = "8083"
	}

	mongoClient, err := db.Connect(cfg.MongoURI)
	if err != nil {
		log.Fatalf("MongoDB connect failed: %v", err)
	}
	defer mongoClient.Disconnect(context.Background())
	log.Printf("[DB] Connected to MongoDB at %s", cfg.MongoURI)

	s := server.NewMCPServer("agentic-mcps", "1.0.0")

	handlers := mcptools.NewHandlers(mongoClient, "agentic_mcps")
	handlers.Register(s)

	baseURL := fmt.Sprintf("http://localhost:%s", cfg.Port)
	sseServer := server.NewSSEServer(s, server.WithBaseURL(baseURL))

	log.Printf("[MCP] Server starting on %s", baseURL)
	log.Printf("[MCP] SSE endpoint: %s/sse", baseURL)
	log.Printf("[MCP] Tools: list_collections, query_documents, insert_document, update_document, delete_document")

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Println("[MCP] Shutting down...")
		mongoClient.Disconnect(context.Background())
		os.Exit(0)
	}()

	if err := sseServer.Start(":" + cfg.Port); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
