package main

import (
	"encoding/json"
	"log"
	"os"
)

type Config struct {
	OllamaHost          string `json:"OLLAMA_HOST"`
	OllamaModel         string `json:"OLLAMA_MODEL"`
	TavilyAPIKey        string `json:"TAVILY_API_KEY"`
	MaxSteps            int    `json:"MAX_STEPS"`
	MaxRetries          int    `json:"MAX_RETRIES"`
	MaxTokens           int    `json:"MAX_TOKENS"`
	SearchEndpoint      string `json:"SEARCH_ENDPOINT"`
	MCPServerURL        string `json:"MCP_SERVER_URL"`
	AnthropicAPIKey     string `json:"ANTHROPIC_API_KEY"`
	AnthropicModel      string `json:"ANTHROPIC_MODEL"`
	AnthropicHasCredits bool   `json:"ANTHROPIC_CREDIT_BALANCE"`

	// Conversation memory (direct Mongo driver, see store.go)
	MongoURI            string `json:"MONGO_URI"`
	MongoDB             string `json:"MONGO_DB"`
	HistoryTurns        int    `json:"HISTORY_TURNS"`        // K recent turns kept verbatim
	HistoryTokenBudget  int    `json:"HISTORY_TOKEN_BUDGET"` // approx token budget for replayed history
	SummaryTriggerTurns int    `json:"SUMMARY_TRIGGER_TURNS"`

	// Pinecone/embedding settings
	PineconeAPIKey string `json:"PINECONE_API_KEY"`
	PineconeIndex  string `json:"PINECONE_INDEX_NAME"`
	PineconeHost   string `json:"PINECONE_HOST"`
	PineconeNS     string `json:"PINECONE_NAMESPACE"`
	EmbeddingModel string `json:"EMBEDDING_MODEL"`
	RetrievalTopK  int    `json:"RETRIEVAL_TOP_K"`

	// Ingestion (PLAN §2A)
	ChunkSize    int `json:"CHUNK_SIZE"`
	ChunkOverlap int `json:"CHUNK_OVERLAP"`
}

var cfg Config

func init() {
	data, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatalf("Failed to read config.json: %v", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("Failed to parse config.json: %v", err)
	}

	// Defaults for conversation memory settings.
	if cfg.MongoURI == "" {
		cfg.MongoURI = "mongodb://localhost:27017"
	}
	if cfg.MongoDB == "" {
		cfg.MongoDB = "agentic_mcps"
	}
	if cfg.HistoryTurns == 0 {
		cfg.HistoryTurns = 6
	}
	if cfg.HistoryTokenBudget == 0 {
		cfg.HistoryTokenBudget = 6000
	}
	if cfg.SummaryTriggerTurns == 0 {
		cfg.SummaryTriggerTurns = 8
	}
	if cfg.EmbeddingModel == "" {
		cfg.EmbeddingModel = "nomic-embed-text"
	}
	if cfg.RetrievalTopK == 0 {
		cfg.RetrievalTopK = 3
	}
	if cfg.ChunkSize == 0 {
		cfg.ChunkSize = 1000
	}
	if cfg.ChunkOverlap == 0 {
		cfg.ChunkOverlap = 150
	}
}
