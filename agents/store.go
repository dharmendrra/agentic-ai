package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Conversation is a stored chat thread. See docs/CONVERSATION_STORAGE_FORMAT.md.
type Conversation struct {
	ID             string    `bson:"_id" json:"id"`
	Title          string    `bson:"title" json:"title"`
	CreatedAt      time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time `bson:"updated_at" json:"updated_at"`
	Summary        string    `bson:"summary" json:"summary"`
	SummaryUptoSeq int       `bson:"summary_upto_seq" json:"summary_upto_seq"`
}

// Message is one stored turn. content holds FINAL text only — never raw tool observations.
type Message struct {
	ID             string     `bson:"_id" json:"id"`
	ConversationID string     `bson:"conversation_id" json:"conversation_id"`
	Seq            int        `bson:"seq" json:"seq"`
	Role           string     `bson:"role" json:"role"` // "user" | "assistant"
	Content        string     `bson:"content" json:"content"`
	Sources        []string   `bson:"sources,omitempty" json:"sources,omitempty"`
	Citations      []Citation `bson:"citations,omitempty" json:"citations,omitempty"`
	CreatedAt      time.Time  `bson:"created_at" json:"created_at"`
}

// Store is the conversation persistence layer. It uses a direct Mongo driver
// (NOT the MCP tool) — persisting history is deterministic application plumbing,
// kept on a separate layer from the model's MCP CRUD tools.
type Store struct {
	client        *mongo.Client
	conversations *mongo.Collection
	messages      *mongo.Collection
}

// store is the package-level singleton, nil when Mongo is unreachable.
var store *Store

// NewStore connects to Mongo (reusing the mcp/db Connect pattern) and verifies
// reachability with a Ping. Returns an error the caller can degrade on.
func NewStore(uri, dbName string) (*Store, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}

	db := client.Database(dbName)
	s := &Store{
		client:        client,
		conversations: db.Collection("conversations"),
		messages:      db.Collection("messages"),
	}

	// Index for ordered per-conversation reads.
	_, _ = s.messages.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "conversation_id", Value: 1}, {Key: "seq", Value: 1}},
	})

	return s, nil
}

// CreateConversation inserts a new conversation with a generated id and returns it.
func (s *Store) CreateConversation(title string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	c := Conversation{
		ID:             uuid.NewString(),
		Title:          title,
		CreatedAt:      now,
		UpdatedAt:      now,
		Summary:        "",
		SummaryUptoSeq: 0,
	}
	if _, err := s.conversations.InsertOne(ctx, c); err != nil {
		return "", fmt.Errorf("create conversation: %w", err)
	}
	return c.ID, nil
}

// nextSeq returns the next monotonic sequence number for a conversation.
func (s *Store) nextSeq(ctx context.Context, convID string) (int, error) {
	opts := options.FindOne().SetSort(bson.D{{Key: "seq", Value: -1}})
	var last Message
	err := s.messages.FindOne(ctx, bson.M{"conversation_id": convID}, opts).Decode(&last)
	if err == mongo.ErrNoDocuments {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	return last.Seq + 1, nil
}

// AppendMessage stores one turn and bumps the conversation's updated_at. Returns the seq.
func (s *Store) AppendMessage(convID, role, content string, sources []string, citations []Citation) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	seq, err := s.nextSeq(ctx, convID)
	if err != nil {
		return 0, fmt.Errorf("next seq: %w", err)
	}
	m := Message{
		ID:             uuid.NewString(),
		ConversationID: convID,
		Seq:            seq,
		Role:           role,
		Content:        content,
		Sources:        sources,
		Citations:      citations,
		CreatedAt:      time.Now().UTC(),
	}
	if _, err := s.messages.InsertOne(ctx, m); err != nil {
		return 0, fmt.Errorf("append message: %w", err)
	}
	_, _ = s.conversations.UpdateByID(ctx, convID, bson.M{"$set": bson.M{"updated_at": time.Now().UTC()}})
	return seq, nil
}

// find runs a filter+sort query and decodes the resulting messages.
func (s *Store) find(ctx context.Context, filter bson.M, opts *options.FindOptions) ([]Message, error) {
	cur, err := s.messages.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []Message
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetRecentMessages returns the last k messages in chronological (ascending seq) order.
func (s *Store) GetRecentMessages(convID string, k int) ([]Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "seq", Value: -1}}).SetLimit(int64(k))
	msgs, err := s.find(ctx, bson.M{"conversation_id": convID}, opts)
	if err != nil {
		return nil, err
	}
	// Reverse to ascending.
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

// GetMessagesUpToSeq returns messages with seq in (afterSeq, uptoSeq], ascending.
// Used to build the rolling summary over older-than-K turns.
func (s *Store) GetMessagesUpToSeq(convID string, afterSeq, uptoSeq int) ([]Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "seq", Value: 1}})
	return s.find(ctx, bson.M{
		"conversation_id": convID,
		"seq":             bson.M{"$gt": afterSeq, "$lte": uptoSeq},
	}, opts)
}

// GetAllUserMessages returns all user messages in a conversation, ascending. (recall_history)
func (s *Store) GetAllUserMessages(convID string) ([]Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "seq", Value: 1}})
	return s.find(ctx, bson.M{"conversation_id": convID, "role": "user"}, opts)
}

// GetFirstNUserMessages returns the first n user messages, ascending. (recall_history)
func (s *Store) GetFirstNUserMessages(convID string, n int) ([]Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "seq", Value: 1}}).SetLimit(int64(n))
	return s.find(ctx, bson.M{"conversation_id": convID, "role": "user"}, opts)
}

// SearchMessages returns user+assistant messages whose content matches query (case-insensitive substring).
func (s *Store) SearchMessages(convID, query string) ([]Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Escape regex metacharacters so user input is treated literally.
	safe := regexpQuoteMeta(query)
	opts := options.Find().SetSort(bson.D{{Key: "seq", Value: 1}})
	return s.find(ctx, bson.M{
		"conversation_id": convID,
		"content":         bson.M{"$regex": safe, "$options": "i"},
	}, opts)
}

// GetConversation returns a single conversation by id.
func (s *Store) GetConversation(convID string) (*Conversation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var c Conversation
	err := s.conversations.FindOne(ctx, bson.M{"_id": convID}).Decode(&c)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpdateSummary writes a new rolling summary and the seq it covers up to.
func (s *Store) UpdateSummary(convID, summary string, uptoSeq int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.conversations.UpdateByID(ctx, convID, bson.M{"$set": bson.M{
		"summary":          summary,
		"summary_upto_seq": uptoSeq,
		"updated_at":       time.Now().UTC(),
	}})
	return err
}

// ListConversations returns all conversations newest-first (for the sidebar).
func (s *Store) ListConversations() ([]Conversation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}})
	cur, err := s.conversations.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []Conversation
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetConversationMessages returns the full message list for a conversation, ascending (for reopening).
func (s *Store) GetConversationMessages(convID string) ([]Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "seq", Value: 1}})
	return s.find(ctx, bson.M{"conversation_id": convID}, opts)
}

// DeleteConversation removes a conversation and all its messages.
func (s *Store) DeleteConversation(convID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := s.messages.DeleteMany(ctx, bson.M{"conversation_id": convID}); err != nil {
		return err
	}
	_, err := s.conversations.DeleteOne(ctx, bson.M{"_id": convID})
	return err
}

// makeTitle derives a short conversation title from the first user message.
func makeTitle(firstQuery string) string {
	t := strings.TrimSpace(firstQuery)
	t = strings.Join(strings.Fields(t), " ") // collapse whitespace
	const maxLen = 60
	if len(t) > maxLen {
		t = strings.TrimSpace(t[:maxLen]) + "…"
	}
	if t == "" {
		t = "New conversation"
	}
	return t
}

// regexpQuoteMeta escapes regex metacharacters (avoids importing regexp just for this).
func regexpQuoteMeta(s string) string {
	const special = `\.+*?()|[]{}^$`
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(special, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// initStore connects the package-level store, logging a warning (not fatal) on failure.
func initStore() {
	s, err := NewStore(cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		log.Printf("[STORE] WARNING: Mongo unavailable at %s: %v (conversation memory disabled)", cfg.MongoURI, err)
		return
	}
	store = s
	log.Printf("[STORE] Connected to Mongo at %s (db=%s)", cfg.MongoURI, cfg.MongoDB)
}
