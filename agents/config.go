package main

import (
	"encoding/json"
	"log"
	"os"
)

type Config struct {
	OllamaHost      string `json:"OLLAMA_HOST"`
	OllamaModel     string `json:"OLLAMA_MODEL"`
	TavilyAPIKey    string `json:"TAVILY_API_KEY"`
	MaxSteps        int    `json:"MAX_STEPS"`
	MaxRetries      int    `json:"MAX_RETRIES"`
	MaxTokens       int    `json:"MAX_TOKENS"`
	SearchEndpoint  string `json:"SEARCH_ENDPOINT"`
	MCPServerURL    string `json:"MCP_SERVER_URL"`
	AnthropicAPIKey      string `json:"ANTHROPIC_API_KEY"`
	AnthropicModel       string `json:"ANTHROPIC_MODEL"`
	AnthropicHasCredits  bool   `json:"ANTHROPIC_CREDIT_BALANCE"`
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
}
