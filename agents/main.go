package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/user/agentic-ai/tools"
)

// Base tool instances. Per request, react.go assembles an active subset of
// these based on the source toggles (see buildActiveToolSet).
var (
	pdfTool *tools.PDFSearchTool
	webTool *tools.WebSearchTool
	mcpTool *tools.MCPTool
)

func main() {
	port := flag.String("port", "8082", "Port to listen on")
	flag.Parse()

	staticDir := filepath.Join(".", "static")
	if _, err := os.Stat(staticDir); err != nil {
		log.Fatalf("static directory not found: %v", err)
	}

	// Pick LLM backend
	if cfg.AnthropicAPIKey != "" && cfg.AnthropicModel != "" && cfg.AnthropicHasCredits {
		llmCaller = &AnthropicCaller{apiKey: cfg.AnthropicAPIKey, model: cfg.AnthropicModel}
		log.Printf("[LLM] Backend: Anthropic (%s)", cfg.AnthropicModel)
	} else {
		llmCaller = &OllamaCaller{}
		log.Printf("[LLM] Backend: Ollama (%s)", cfg.OllamaModel)
	}

	// Connect conversation store (direct Mongo driver). Degrades gracefully.
	initStore()

	// Build base tool instances. react.go assembles the active subset per request.
	// Native PDF search (PLAN §2A): embed via Ollama, query Pinecone directly,
	// narrow by book via the Mongo `books` catalog. Degrades gracefully.
	embedder := tools.NewOllamaEmbedder(cfg.OllamaHost, cfg.EmbeddingModel)
	querier := tools.NewPineconeQuerier(cfg.PineconeAPIKey, cfg.PineconeHost, cfg.PineconeNS, cfg.RetrievalTopK)
	var catalog *tools.BooksCatalog
	if c, _, cErr := tools.ConnectBooksCatalog(cfg.MongoURI, cfg.MongoDB); cErr != nil {
		log.Printf("[CATALOG] WARNING: books catalog unavailable: %v (book-title narrowing disabled)", cErr)
	} else {
		catalog = c
		log.Printf("[CATALOG] Connected books catalog (db=%s)", cfg.MongoDB)
	}
	pdfTool = tools.NewPDFSearchTool(embedder, querier, catalog)
	webTool = tools.NewWebSearchTool(cfg.TavilyAPIKey, cfg.MaxRetries)

	// Ingestion shares the same embedder/querier/catalog (PLAN §2A).
	ingestEmbedder = embedder
	ingestQuerier = querier
	ingestCatalog = catalog

	// Connect to MCP server (LLM will discover tools dynamically via action: "list_tools")
	mcpURL := cfg.MCPServerURL
	if mcpURL == "" {
		mcpURL = "http://localhost:8083"
	}
	mcpClient, err := tools.Connect(context.Background(), mcpURL)
	if err != nil {
		log.Printf("[MCP] WARNING: Could not connect to MCP server at %s: %v (MCP tool unavailable)", mcpURL, err)
	} else {
		mcpTool = tools.NewMCPTool(mcpClient)
		log.Printf("[MCP] Registered MCP tool (LLM will discover resources dynamically)")
	}

	http.HandleFunc("/api/agent/query", handleAgentQuery)
	http.HandleFunc("/api/ingest", handleIngest)
	http.HandleFunc("/api/conversations", handleConversations)
	http.HandleFunc("/api/conversations/", handleConversationByID)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
		} else {
			http.ServeFile(w, r, filepath.Join(staticDir, r.URL.Path))
		}
	})

	log.Printf("Agent server listening on http://localhost:%s", *port)
	log.Printf("Endpoint: POST http://localhost:%s/api/agent/query", *port)
	log.Printf("UI: http://localhost:%s", *port)

	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
