package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// WebSearchConfig holds configuration for web search tool
type WebSearchConfig struct {
	APIKey     string
	MaxRetries int
}

// WebSearchTool searches the web
type WebSearchTool struct {
	config WebSearchConfig
}

// NewWebSearchTool creates a new web search tool
func NewWebSearchTool(apiKey string, maxRetries int) *WebSearchTool {
	return &WebSearchTool{
		config: WebSearchConfig{
			APIKey:     apiKey,
			MaxRetries: maxRetries,
		},
	}
}

func (t *WebSearchTool) Name() string { return "web_search" }

func (t *WebSearchTool) Schema() ToolSchema {
	return ToolSchema{
		Name:        "web_search",
		Description: "Search the internet. Use only if PDF search doesn't return relevant results. Provides additional context or verification.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "The search query to find information on the internet",
				},
			},
			"required": []string{"query"},
		},
	}
}

// Execute runs the web search
func (t *WebSearchTool) Execute(query string) (string, error) {
	if t.config.APIKey == "" {
		log.Printf("[TOOL] web_search: SKIPPED - TAVILY_API_KEY not configured")
		return "Web search not configured (missing TAVILY_API_KEY)", nil
	}

	log.Printf("[TOOL] web_search: Querying Tavily API with: '%s'", query)
	// Tavily requires POST with the key in the JSON body; the old GET ?api_key=
	// form now returns 401.
	reqBody, _ := json.Marshal(map[string]any{
		"api_key":        t.config.APIKey,
		"query":          query,
		"max_results":    3,
		"include_answer": true,
	})

	var result string
	for attempt := 0; attempt < t.config.MaxRetries; attempt++ {
		log.Printf("[TOOL] web_search: Attempt %d/%d", attempt+1, t.config.MaxRetries)
		resp, err := http.Post("https://api.tavily.com/search", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			log.Printf("[TOOL] web_search: Connection error on attempt %d: %v", attempt+1, err)
			if attempt < t.config.MaxRetries-1 {
				time.Sleep(time.Second * time.Duration(attempt+1))
				continue
			}
			return "", fmt.Errorf("web_search failed after %d retries: %v", t.config.MaxRetries, err)
		}
		defer resp.Body.Close()

		log.Printf("[TOOL] web_search: Got status %d", resp.StatusCode)
		if resp.StatusCode != http.StatusOK {
			if attempt < t.config.MaxRetries-1 {
				log.Printf("[TOOL] web_search: Retrying after status %d...", resp.StatusCode)
				time.Sleep(time.Second * time.Duration(attempt+1))
				continue
			}
			return "", fmt.Errorf("web_search returned status %d", resp.StatusCode)
		}

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}

		var tavilyResp struct {
			Answer  string `json:"answer"`
			Results []struct {
				Title string `json:"title"`
				URL   string `json:"url"`
			} `json:"results"`
		}
		if err := json.Unmarshal(respBody, &tavilyResp); err != nil {
			return "", err
		}

		result = tavilyResp.Answer
		if result == "" && len(tavilyResp.Results) > 0 {
			result = tavilyResp.Results[0].Title
		}
		// Append a machine-readable sources marker that react.go strips out and
		// turns into clickable citations (the LLM never needs to render it).
		var parts []string
		for _, r := range tavilyResp.Results {
			if r.URL == "" {
				continue
			}
			parts = append(parts, strings.TrimSpace(r.Title)+" :: "+r.URL)
		}
		if len(parts) > 0 {
			result += "\n\n" + WebSrcMarker + strings.Join(parts, " || ")
		}
		break
	}

	if strings.TrimSpace(result) == "" {
		return "No web results found", nil
	}
	return result, nil
}

// WebSrcMarker prefixes the machine-readable web sources line appended to a
// web_search observation. react.go parses + strips it (see splitWebCitations).
const WebSrcMarker = "[[WEBSRC]]"
