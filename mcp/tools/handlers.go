package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Handlers struct {
	client *mongo.Client
	dbName string
}

func NewHandlers(client *mongo.Client, dbName string) *Handlers {
	return &Handlers{client: client, dbName: dbName}
}

func (h *Handlers) db() *mongo.Database {
	return h.client.Database(h.dbName)
}

func (h *Handlers) Register(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("list_collections",
		mcp.WithDescription("List all MongoDB collections in agentic_mcps with their document counts"),
	), h.listCollections)

	s.AddTool(mcp.NewTool("query_documents",
		mcp.WithDescription("Query documents from a collection. Filter is an optional JSON object (e.g. {\"Status\":\"To Do\"}). Returns up to `limit` documents."),
		mcp.WithString("collection", mcp.Required(), mcp.Description("Collection name: learning_todo, links_tracker, or job_portals")),
		mcp.WithString("filter", mcp.Description("Optional JSON filter object, e.g. {\"Status\":\"To Do\"}")),
		mcp.WithNumber("limit", mcp.Description("Max documents to return (default 20)")),
	), h.queryDocuments)

	s.AddTool(mcp.NewTool("insert_document",
		mcp.WithDescription("Insert a new document into a collection"),
		mcp.WithString("collection", mcp.Required(), mcp.Description("Collection name")),
		mcp.WithString("document", mcp.Required(), mcp.Description("JSON document to insert, e.g. {\"Name\":\"Study Go\",\"Status\":\"To Do\"}")),
	), h.insertDocument)

	s.AddTool(mcp.NewTool("update_document",
		mcp.WithDescription("Update documents matching a filter in a collection"),
		mcp.WithString("collection", mcp.Required(), mcp.Description("Collection name")),
		mcp.WithString("filter", mcp.Required(), mcp.Description("JSON filter to match documents, e.g. {\"Name\":\"Study Go\"}")),
		mcp.WithString("update", mcp.Required(), mcp.Description("JSON fields to set, e.g. {\"Status\":\"Done\"}")),
	), h.updateDocument)

	s.AddTool(mcp.NewTool("delete_document",
		mcp.WithDescription("Delete documents matching a filter from a collection"),
		mcp.WithString("collection", mcp.Required(), mcp.Description("Collection name")),
		mcp.WithString("filter", mcp.Required(), mcp.Description("JSON filter to match documents, e.g. {\"Name\":\"Study Go\"}")),
	), h.deleteDocument)
}

func (h *Handlers) listCollections(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	names, err := h.db().ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}

	type collInfo struct {
		Name  string `json:"name"`
		Count int64  `json:"count"`
	}
	var result []collInfo
	for _, name := range names {
		count, err := h.db().Collection(name).CountDocuments(ctx, bson.D{})
		if err != nil {
			count = -1
		}
		result = append(result, collInfo{Name: name, Count: count})
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}

func (h *Handlers) queryDocuments(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	collName := req.GetString("collection", "")
	if collName == "" {
		return nil, fmt.Errorf("collection is required")
	}
	filterStr := req.GetString("filter", "{}")
	limit := int64(req.GetInt("limit", 20))

	var filter bson.M
	if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
		return nil, fmt.Errorf("invalid filter JSON: %w", err)
	}

	opts := options.Find().SetLimit(limit)
	cursor, err := h.db().Collection(collName).Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []bson.M
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("read cursor: %w", err)
	}

	if len(docs) == 0 {
		return mcp.NewToolResultText("no documents found"), nil
	}

	out, _ := json.MarshalIndent(docs, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}

func (h *Handlers) insertDocument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	collName := req.GetString("collection", "")
	if collName == "" {
		return nil, fmt.Errorf("collection is required")
	}
	docStr := req.GetString("document", "")
	if docStr == "" {
		return nil, fmt.Errorf("document is required")
	}

	var doc bson.M
	if err := json.Unmarshal([]byte(docStr), &doc); err != nil {
		return nil, fmt.Errorf("invalid document JSON: %w", err)
	}

	res, err := h.db().Collection(collName).InsertOne(ctx, doc)
	if err != nil {
		return nil, fmt.Errorf("insert: %w", err)
	}

	return mcp.NewToolResultText(fmt.Sprintf("inserted with _id: %v", res.InsertedID)), nil
}

func (h *Handlers) updateDocument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	collName := req.GetString("collection", "")
	filterStr := req.GetString("filter", "")
	updateStr := req.GetString("update", "")

	if collName == "" || filterStr == "" || updateStr == "" {
		return nil, fmt.Errorf("collection, filter, and update are all required")
	}

	var filter, updateFields bson.M
	if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
		return nil, fmt.Errorf("invalid filter JSON: %w", err)
	}
	if err := json.Unmarshal([]byte(updateStr), &updateFields); err != nil {
		return nil, fmt.Errorf("invalid update JSON: %w", err)
	}

	res, err := h.db().Collection(collName).UpdateMany(ctx, filter, bson.M{"$set": updateFields})
	if err != nil {
		return nil, fmt.Errorf("update: %w", err)
	}

	return mcp.NewToolResultText(fmt.Sprintf("matched: %d, modified: %d", res.MatchedCount, res.ModifiedCount)), nil
}

func (h *Handlers) deleteDocument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	collName := req.GetString("collection", "")
	filterStr := req.GetString("filter", "")

	if collName == "" || filterStr == "" {
		return nil, fmt.Errorf("collection and filter are required")
	}

	var filter bson.M
	if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
		return nil, fmt.Errorf("invalid filter JSON: %w", err)
	}

	res, err := h.db().Collection(collName).DeleteMany(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("delete: %w", err)
	}

	return mcp.NewToolResultText(fmt.Sprintf("deleted: %d document(s)", res.DeletedCount)), nil
}
