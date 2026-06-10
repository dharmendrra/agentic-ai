package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ledongthuc/pdf"
	"github.com/user/agentic-ai/tools"
)

// Ingestion dependencies, wired in main.go (PLAN §2A). Nil when Pinecone/embedder
// are unavailable — handleIngest then returns an error.
var (
	ingestEmbedder *tools.OllamaEmbedder
	ingestQuerier  *tools.PineconeQuerier
	ingestCatalog  *tools.BooksCatalog
)

// extractPDFText extracts plain text per page from a PDF on disk.
func extractPDFText(path string) ([]string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	defer f.Close()

	n := r.NumPage()
	pages := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			pages = append(pages, "")
			continue
		}
		txt, err := p.GetPlainText(nil)
		if err != nil {
			return nil, fmt.Errorf("extract page %d: %w", i, err)
		}
		pages = append(pages, strings.TrimSpace(txt))
	}
	return pages, nil
}

// chunkText splits text into ~size-rune chunks with overlap.
func chunkText(text string, size, overlap int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if size <= 0 {
		size = 1000
	}
	if overlap < 0 || overlap >= size {
		overlap = 0
	}
	runes := []rune(text)
	var chunks []string
	for start := 0; start < len(runes); start += size - overlap {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		if chunk := strings.TrimSpace(string(runes[start:end])); chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(runes) {
			break
		}
	}
	return chunks
}

type ingestSummary struct {
	BookTitle    string `json:"book_title"`
	SourceFileID string `json:"source_file_id"`
	Pages        int    `json:"pages"`
	Chunks       int    `json:"chunks"`
	Upserted     int    `json:"upserted"`
	Message      string `json:"message"`
}

// ingestPDFFile runs extract → chunk → embed → upsert → catalog for a PDF on disk.
func ingestPDFFile(ctx context.Context, path, bookTitle string) (*ingestSummary, error) {
	if ingestQuerier == nil || ingestEmbedder == nil {
		return nil, fmt.Errorf("ingestion not configured (Pinecone/embedder unavailable)")
	}
	pages, err := extractPDFText(path)
	if err != nil {
		return nil, err
	}
	sourceFileID := uuid.NewString()

	var vectors []tools.PineconeVector
	chunkIndex := 0
	for pageIdx, pageText := range pages {
		for _, chunk := range chunkText(pageText, cfg.ChunkSize, cfg.ChunkOverlap) {
			vec, err := ingestEmbedder.Embed(ctx, chunk)
			if err != nil {
				return nil, fmt.Errorf("embed chunk %d: %w", chunkIndex, err)
			}
			vectors = append(vectors, tools.PineconeVector{
				ID:     fmt.Sprintf("%s-%d", sourceFileID[:8], chunkIndex),
				Values: vec,
				Metadata: map[string]any{
					"text_content":   chunk,
					"book_title":     bookTitle,
					"source_file_id": sourceFileID,
					"page_number":    pageIdx + 1,
					"chapter":        0,
					"chunk_index":    chunkIndex,
				},
			})
			chunkIndex++
		}
	}

	if len(vectors) == 0 {
		return &ingestSummary{
			BookTitle:    bookTitle,
			SourceFileID: sourceFileID,
			Pages:        len(pages),
			Message:      "no extractable text found (scanned/image-only PDF?)",
		}, nil
	}

	upserted := 0
	for i := 0; i < len(vectors); i += 100 {
		end := i + 100
		if end > len(vectors) {
			end = len(vectors)
		}
		if err := ingestQuerier.Upsert(ctx, vectors[i:end]); err != nil {
			return nil, fmt.Errorf("upsert: %w", err)
		}
		upserted += end - i
	}

	if ingestCatalog != nil {
		if err := ingestCatalog.Upsert(ctx, tools.Book{ID: sourceFileID, Title: bookTitle, ChunkCount: len(vectors)}); err != nil {
			log.Printf("[INGEST] WARN: catalog upsert failed: %v", err)
		}
	}

	return &ingestSummary{
		BookTitle:    bookTitle,
		SourceFileID: sourceFileID,
		Pages:        len(pages),
		Chunks:       len(vectors),
		Upserted:     upserted,
		Message:      fmt.Sprintf("Ingested %q: %d chunks across %d pages", bookTitle, upserted, len(pages)),
	}, nil
}

// handleIngest accepts a multipart PDF upload (field "file", optional "book_title")
// and ingests it into Pinecone + the books catalog. POST /api/ingest.
func handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, "invalid multipart upload: "+err.Error(), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing multipart field 'file'", http.StatusBadRequest)
		return
	}
	defer file.Close()

	title := strings.TrimSpace(r.FormValue("book_title"))
	if title == "" {
		title = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	}

	tmp, err := os.CreateTemp("", "ingest-*.pdf")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, file); err != nil {
		tmp.Close()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmp.Close()

	log.Printf("[INGEST] ingesting %q as book %q", header.Filename, title)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	summary, err := ingestPDFFile(ctx, tmp.Name(), title)
	if err != nil {
		log.Printf("[INGEST] ERROR: %v", err)
		http.Error(w, "ingest failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("[INGEST] %s", summary.Message)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}
