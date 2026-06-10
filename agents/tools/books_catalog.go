package tools

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Book represents a book in the catalog. Written at ingestion time, queried at
// search time for title resolution + clarify-back. See PLAN §2A.
type Book struct {
	ID         string    `bson:"_id" json:"id"`          // = source_file_id
	Title      string    `bson:"title" json:"title"`     // canonical title
	Aliases    []string  `bson:"aliases" json:"aliases"` // lowercase search aliases
	ChunkCount int       `bson:"chunk_count" json:"chunk_count"`
	CreatedAt  time.Time `bson:"created_at" json:"created_at"`
}

// BooksCatalog manages the Mongo `books` collection. It is used by search_pdf
// to resolve a spoken book title to one or more source_file_id values.
type BooksCatalog struct {
	coll *mongo.Collection
}

// NewBooksCatalog creates a catalog backed by the given Mongo database.
func NewBooksCatalog(db *mongo.Database) *BooksCatalog {
	return &BooksCatalog{coll: db.Collection("books")}
}

// ConnectBooksCatalog connects to Mongo and returns a BooksCatalog + the
// underlying client (caller should defer client.Disconnect). Mirrors the
// graceful-degradation pattern from store.go.
func ConnectBooksCatalog(mongoURI, mongoDB string) (*BooksCatalog, *mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, nil, fmt.Errorf("connect mongo for books catalog: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, nil, fmt.Errorf("ping mongo for books catalog: %w", err)
	}
	db := client.Database(mongoDB)
	return NewBooksCatalog(db), client, nil
}

// Upsert creates or updates a book entry. Called during ingestion (PLAN §2A).
func (c *BooksCatalog) Upsert(ctx context.Context, book Book) error {
	// Build lowercase aliases from the title if none provided.
	if len(book.Aliases) == 0 {
		book.Aliases = []string{strings.ToLower(book.Title)}
	}
	if book.CreatedAt.IsZero() {
		book.CreatedAt = time.Now().UTC()
	}

	filter := bson.M{"_id": book.ID}
	update := bson.M{
		"$set": bson.M{
			"title":       book.Title,
			"aliases":     book.Aliases,
			"chunk_count": book.ChunkCount,
		},
		"$setOnInsert": bson.M{
			"created_at": book.CreatedAt,
		},
	}
	opts := options.Update().SetUpsert(true)
	_, err := c.coll.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("upsert book %q: %w", book.Title, err)
	}
	log.Printf("[CATALOG] Upserted book %q (id=%s, chunks=%d)", book.Title, book.ID, book.ChunkCount)
	return nil
}

// ResolveTitle searches for a book by title or alias. Returns:
//   - exactly 1 match → the matching Book
//   - multiple matches → all candidates (caller should clarify-back)
//   - 0 matches → empty slice
//
// The search is case-insensitive and checks both the title field and the
// aliases array. See PLAN §2A (book-title narrowing + clarify-back).
func (c *BooksCatalog) ResolveTitle(ctx context.Context, spoken string) ([]Book, error) {
	spoken = strings.ToLower(strings.TrimSpace(spoken))
	if spoken == "" {
		return nil, nil
	}

	// Case-insensitive regex match on title or exact match on aliases.
	filter := bson.M{
		"$or": bson.A{
			bson.M{"title": bson.M{"$regex": spoken, "$options": "i"}},
			bson.M{"aliases": spoken},
		},
	}

	cursor, err := c.coll.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("resolve book title %q: %w", spoken, err)
	}
	defer cursor.Close(ctx)

	var books []Book
	if err := cursor.All(ctx, &books); err != nil {
		return nil, fmt.Errorf("decode book results for %q: %w", spoken, err)
	}
	log.Printf("[CATALOG] Resolved title %q → %d match(es)", spoken, len(books))
	return books, nil
}

// ListBooks returns all books in the catalog (for UI / diagnostics).
func (c *BooksCatalog) ListBooks(ctx context.Context) ([]Book, error) {
	cursor, err := c.coll.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "title", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("list books: %w", err)
	}
	defer cursor.Close(ctx)

	var books []Book
	if err := cursor.All(ctx, &books); err != nil {
		return nil, fmt.Errorf("decode books list: %w", err)
	}
	return books, nil
}
