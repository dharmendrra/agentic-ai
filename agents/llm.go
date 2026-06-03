package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// LLMCaller abstracts the LLM backend so Anthropic and Ollama are interchangeable.
type LLMCaller interface {
	Call(system, user string) (string, error)
	ModelName() string
}

var llmCaller LLMCaller

// ─── Ollama ───────────────────────────────────────────────────────────────────

type OllamaRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	Options map[string]interface{} `json:"options"`
}

type OllamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

type OllamaCaller struct{}

func (o *OllamaCaller) ModelName() string { return cfg.OllamaModel }

func (o *OllamaCaller) Call(system, user string) (string, error) {
	prompt := system + "\n\n" + user
	log.Printf("[OLLAMA] model=%s max_tokens=%d", cfg.OllamaModel, cfg.MaxTokens)
	req := OllamaRequest{
		Model:  cfg.OllamaModel,
		Prompt: prompt,
		Stream: false,
		Options: map[string]interface{}{
			"num_predict": cfg.MaxTokens,
			"temperature": 0.7,
		},
	}
	body, _ := json.Marshal(req)
	resp, err := http.Post(cfg.OllamaHost+"/api/generate", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var ollamaResp OllamaResponse
	if err := json.Unmarshal(raw, &ollamaResp); err != nil {
		return "", err
	}
	return ollamaResp.Response, nil
}

// ─── Anthropic ────────────────────────────────────────────────────────────────

type AnthropicCaller struct {
	apiKey string
	model  string
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func (a *AnthropicCaller) ModelName() string { return a.model }

func (a *AnthropicCaller) Call(system, user string) (string, error) {
	log.Printf("[ANTHROPIC] model=%s max_tokens=%d", a.model, cfg.MaxTokens)
	reqBody := anthropicRequest{
		Model:     a.model,
		MaxTokens: cfg.MaxTokens,
		System:    system,
		Messages:  []anthropicMessage{{Role: "user", Content: user}},
	}
	body, _ := json.Marshal(reqBody)
	httpReq, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("anthropic request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("anthropic returned status %d: %s", resp.StatusCode, string(raw))
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var ar anthropicResponse
	if err := json.Unmarshal(raw, &ar); err != nil {
		return "", err
	}
	for _, block := range ar.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	return "", fmt.Errorf("anthropic response had no text content")
}
