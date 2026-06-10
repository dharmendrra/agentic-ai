package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// SourceMatch is a single document chunk returned from Pinecone.
// Aligned with Omni-RAG's metadata schema + book_title (PLAN §2A).
type SourceMatch struct {
	ID           string  `json:"id"`
	Score        float64 `json:"score"`
	SourceFileID string  `json:"source_file_id"`
	BookTitle    string  `json:"book_title"`
	TextContent  string  `json:"text_content"`
	Chapter      int     `json:"chapter"`
	PageNumber   int     `json:"page_number"`
}

// PineconeQuerier queries a Pinecone index directly via the REST API.
// Ported from Omni-RAG pinecone-rag/retrieval/pinecone.go and enhanced with
// metadata filter support for per-book narrowing — see PLAN §2A.
type PineconeQuerier struct {
	APIKey    string
	Host      string
	Namespace string
	TopK      int
	Client    *http.Client
}

// NewPineconeQuerier creates a querier. Host should be the full Pinecone index
// URL (e.g. "https://my-index-abc123.svc.pinecone.io"). If empty, Query will
// return an error.
func NewPineconeQuerier(apiKey, host, namespace string, topK int) *PineconeQuerier {
	return &PineconeQuerier{
		APIKey:    apiKey,
		Host:      strings.TrimRight(host, "/"),
		Namespace: namespace,
		TopK:      topK,
		Client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// PineconeVector is a single vector to upsert (id + values + metadata).
type PineconeVector struct {
	ID       string         `json:"id"`
	Values   []float32      `json:"values"`
	Metadata map[string]any `json:"metadata"`
}

// Upsert writes vectors to the index in one request (caller batches if needed).
// Used by ingestion (PLAN §2A).
func (p *PineconeQuerier) Upsert(ctx context.Context, vectors []PineconeVector) error {
	if len(vectors) == 0 {
		return nil
	}
	if p.APIKey == "" || p.Host == "" {
		return errors.New("pinecone upsert: API key and host required")
	}
	payload := map[string]any{"vectors": vectors}
	if p.Namespace != "" {
		payload["namespace"] = p.Namespace
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal pinecone upsert: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Host+"/vectors/upsert", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", p.APIKey)

	resp, err := p.Client.Do(req)
	if err != nil {
		return fmt.Errorf("pinecone upsert request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("pinecone upsert status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

type pineconeQueryRequest struct {
	Vector          []float32      `json:"vector"`
	TopK            int            `json:"topK"`
	IncludeMetadata bool           `json:"includeMetadata"`
	Namespace       string         `json:"namespace,omitempty"`
	Filter          map[string]any `json:"filter,omitempty"`
}

type pineconeMatch struct {
	ID       string                 `json:"id"`
	Score    float64                `json:"score"`
	Metadata map[string]interface{} `json:"metadata"`
}

type pineconeQueryResponse struct {
	Matches []pineconeMatch `json:"matches"`
}

// Query sends a vector similarity query to Pinecone. The optional filter map
// supports Pinecone metadata filtering (e.g. {"source_file_id": {"$in": [...]}})
// for book-title narrowing (PLAN §2A).
func (p *PineconeQuerier) Query(ctx context.Context, vector []float32, filter map[string]any) ([]SourceMatch, error) {
	if p.APIKey == "" {
		return nil, errors.New("PINECONE_API_KEY is not configured")
	}
	if p.Host == "" {
		return nil, errors.New("PINECONE_HOST is not configured")
	}

	reqBody := pineconeQueryRequest{
		Vector:          vector,
		TopK:            p.TopK,
		IncludeMetadata: true,
	}
	if p.Namespace != "" {
		reqBody.Namespace = p.Namespace
	}
	if len(filter) > 0 {
		reqBody.Filter = filter
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal pinecone query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Host+"/query", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", p.APIKey)

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pinecone query request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("close pinecone query response body: %v", closeErr)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("pinecone query status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var decoded pineconeQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode pinecone query response: %w", err)
	}

	sources := make([]SourceMatch, 0, len(decoded.Matches))
	for _, m := range decoded.Matches {
		src := SourceMatch{ID: m.ID, Score: m.Score}
		if meta := m.Metadata; meta != nil {
			if v, ok := meta["source_file_id"].(string); ok {
				src.SourceFileID = v
			}
			if v, ok := meta["text_content"].(string); ok {
				src.TextContent = v
			}
			if v, ok := meta["book_title"].(string); ok {
				src.BookTitle = v
			}
			if v, ok := meta["chapter"].(float64); ok {
				src.Chapter = int(v)
			}
			if v, ok := meta["page_number"].(float64); ok {
				src.PageNumber = int(v)
			}
		}
		sources = append(sources, src)
	}
	return sources, nil
}
