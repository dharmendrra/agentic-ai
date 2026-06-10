package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// PDFSearchTool searches PDF documents via native Pinecone vector search.
// Replaces the old SSE-endpoint proxy — see PLAN §2A.
type PDFSearchTool struct {
	embedder *OllamaEmbedder
	querier  *PineconeQuerier
	catalog  *BooksCatalog // may be nil if Mongo is unavailable
}

// NewPDFSearchTool creates a native PDF search tool.
func NewPDFSearchTool(embedder *OllamaEmbedder, querier *PineconeQuerier, catalog *BooksCatalog) *PDFSearchTool {
	return &PDFSearchTool{
		embedder: embedder,
		querier:  querier,
		catalog:  catalog,
	}
}

func (t *PDFSearchTool) Name() string { return "search_pdf" }

func (t *PDFSearchTool) Schema() ToolSchema {
	return ToolSchema{
		Name:        "search_pdf",
		Description: "Search the user's PDF book library via semantic vector search. Returns direct excerpts from ingested PDFs. If the user names a specific book, pass it in the 'book' field to narrow results.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "The search query to find relevant content in PDFs",
				},
				"book": map[string]any{
					"type":        "string",
					"description": "Optional book title to narrow the search to a specific book",
				},
			},
			"required": []string{"query"},
		},
	}
}

// pdfInput parses the LLM's Action Input (plain text for query-only, or JSON
// when the LLM passes a book field).
type pdfInput struct {
	Query string `json:"query"`
	Book  string `json:"book"`
}

func parsePDFInput(raw string) pdfInput {
	raw = strings.TrimSpace(raw)

	// Try JSON first (LLM may send {"query":"...", "book":"..."}).
	var inp pdfInput
	if json.Unmarshal([]byte(raw), &inp) == nil && inp.Query != "" {
		return inp
	}
	// Fall back to plain text (just the query).
	return pdfInput{Query: raw}
}

// Execute runs the native PDF search. input is either plain-text query or JSON
// {"query": "...", "book": "..."}.
func (t *PDFSearchTool) Execute(input string) (string, error) {
	inp := parsePDFInput(input)
	log.Printf("[TOOL] search_pdf: query=%q book=%q", inp.Query, inp.Book)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// --- Book-title narrowing (PLAN §2A) ---
	var filter map[string]any
	var resolvedTitle string
	if inp.Book != "" && t.catalog != nil {
		matches, err := t.catalog.ResolveTitle(ctx, inp.Book)
		if err != nil {
			log.Printf("[TOOL] search_pdf: catalog lookup error: %v", err)
			// Fall through to unfiltered search.
		} else if len(matches) > 0 {
			// Group by canonical title: the same book may have several
			// source_file_ids (re-ingested copies). One distinct title → treat as
			// a single book and filter by $in over all its ids. Multiple distinct
			// titles → genuinely ambiguous → clarify-back.
			byTitle := map[string][]string{}
			var order []string
			for _, b := range matches {
				key := strings.ToLower(strings.TrimSpace(b.Title))
				if _, ok := byTitle[key]; !ok {
					order = append(order, b.Title)
				}
				byTitle[key] = append(byTitle[key], b.ID)
			}
			if len(byTitle) == 1 {
				resolvedTitle = order[0]
				ids := byTitle[strings.ToLower(strings.TrimSpace(resolvedTitle))]
				log.Printf("[TOOL] search_pdf: narrowing to book %q (%d file id(s))", resolvedTitle, len(ids))
				filter = map[string]any{"source_file_id": map[string]any{"$in": ids}}
			} else {
				log.Printf("[TOOL] search_pdf: %d distinct titles for %q - needs clarification", len(byTitle), inp.Book)
				return fmt.Sprintf("[NEEDS_CLARIFICATION|Multiple books match '%s': %s]", inp.Book, strings.Join(order, ", ")), nil
			}
		} else {
			log.Printf("[TOOL] search_pdf: no book match for %q - searching all", inp.Book)
		}
	}

	// --- Embed the query ---
	log.Printf("[TOOL] search_pdf: generating embedding via Ollama")
	vector, err := t.embedder.Embed(ctx, inp.Query)
	if err != nil {
		return "", fmt.Errorf("search_pdf: embed query: %w", err)
	}
	log.Printf("[TOOL] search_pdf: got %d-dim embedding", len(vector))

	// --- Query Pinecone ---
	log.Printf("[TOOL] search_pdf: querying Pinecone (topK=%d, filtered=%v)", t.querier.TopK, filter != nil)
	sources, err := t.querier.Query(ctx, vector, filter)
	if err != nil {
		return "", fmt.Errorf("search_pdf: pinecone query: %w", err)
	}

	if len(sources) == 0 {
		log.Printf("[TOOL] search_pdf: EMPTY - no matching vectors")
		return "[PDF_EMPTY|No matching documents found in PDF library]", nil
	}

	// --- Format results ---
	var chunks []string
	for _, src := range sources {
		if src.TextContent == "" {
			continue
		}
		book := firstNonEmpty(src.BookTitle, resolvedTitle, src.SourceFileID)
		if book != "" {
			chunks = append(chunks, fmt.Sprintf("[book: %s] %s", book, src.TextContent))
		} else {
			chunks = append(chunks, src.TextContent)
		}
	}

	if len(chunks) == 0 {
		log.Printf("[TOOL] search_pdf: EMPTY - all chunks were empty text")
		return "[PDF_EMPTY|No matching documents found in PDF library]", nil
	}

	result := strings.Join(chunks, "\n---\n")
	log.Printf("[TOOL] search_pdf: SUCCESS - returning %d chunks", len(chunks))
	return fmt.Sprintf("[PDF_SUCCESS|Found %d matching chunks]\n%s", len(chunks), result), nil
}

// firstNonEmpty returns the first non-blank string from vals, or "".
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
