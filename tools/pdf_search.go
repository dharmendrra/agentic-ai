package tools

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// PDFSearchConfig holds configuration for PDF search tool
type PDFSearchConfig struct {
	Endpoint  string
	MaxRetries int
}

// PDFSearchTool searches PDF documents
type PDFSearchTool struct {
	config PDFSearchConfig
}

// NewPDFSearchTool creates a new PDF search tool
func NewPDFSearchTool(endpoint string, maxRetries int) *PDFSearchTool {
	return &PDFSearchTool{
		config: PDFSearchConfig{
			Endpoint:   endpoint,
			MaxRetries: maxRetries,
		},
	}
}

// Name returns the tool name
func (t *PDFSearchTool) Name() string {
	return "search_pdf"
}

// Description returns the tool description
func (t *PDFSearchTool) Description() string {
	return "Search the user's PDF documents. Use this FIRST for any factual questions. Returns direct excerpts from ingested PDFs."
}

// Execute runs the PDF search
func (t *PDFSearchTool) Execute(query string) (string, error) {
	log.Printf("[TOOL] search_pdf: Querying PDF endpoint with: '%s'", query)
	body := map[string]string{"query": query}
	bodyBytes, _ := json.Marshal(body)

	var result string
	for attempt := 0; attempt < t.config.MaxRetries; attempt++ {
		log.Printf("[TOOL] search_pdf: Attempt %d/%d to %s", attempt+1, t.config.MaxRetries, t.config.Endpoint)
		resp, err := http.Post(t.config.Endpoint, "application/json", bytes.NewReader(bodyBytes))
		if err != nil {
			log.Printf("[TOOL] search_pdf: Connection error on attempt %d: %v", attempt+1, err)
			if attempt < t.config.MaxRetries-1 {
				time.Sleep(time.Second * time.Duration(attempt+1))
				continue
			}
			return "", fmt.Errorf("search_pdf failed after %d retries: %v", t.config.MaxRetries, err)
		}
		defer resp.Body.Close()

		log.Printf("[TOOL] search_pdf: Got status %d", resp.StatusCode)
		if resp.StatusCode != http.StatusOK {
			if attempt < t.config.MaxRetries-1 {
				log.Printf("[TOOL] search_pdf: Retrying after status %d...", resp.StatusCode)
				time.Sleep(time.Second * time.Duration(attempt+1))
				continue
			}
			return "", fmt.Errorf("search_pdf returned status %d", resp.StatusCode)
		}

		// Parse SSE stream
		log.Printf("[TOOL] search_pdf: Parsing SSE stream response")
		scanner := bufio.NewScanner(resp.Body)
		var chunks []string

		for scanner.Scan() {
			line := scanner.Text()

			// Look for "event: sources" followed by "data: {...}"
			if strings.HasPrefix(line, "event: sources") {
				// Next line should be "data: ..."
				if scanner.Scan() {
					dataLine := scanner.Text()
					if strings.HasPrefix(dataLine, "data: ") {
						jsonStr := strings.TrimPrefix(dataLine, "data: ")

						var sourcesResp struct {
							Sources []struct {
								TextContent string `json:"text_content"`
							} `json:"sources"`
						}

						if err := json.Unmarshal([]byte(jsonStr), &sourcesResp); err != nil {
							log.Printf("[TOOL] search_pdf: Error parsing sources event: %v", err)
							continue
						}

						for _, source := range sourcesResp.Sources {
							if source.TextContent != "" {
								chunks = append(chunks, source.TextContent)
							}
						}

						log.Printf("[TOOL] search_pdf: Extracted %d chunks from SSE stream", len(chunks))
					}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			log.Printf("[TOOL] search_pdf: Error reading stream: %v", err)
			return "", fmt.Errorf("search_pdf: stream read error: %v", err)
		}

		if len(chunks) > 0 {
			result = strings.Join(chunks, "\n---\n")
		}
		break
	}

	if result == "" {
		return "No results found in PDF", nil
	}
	return result, nil
}
