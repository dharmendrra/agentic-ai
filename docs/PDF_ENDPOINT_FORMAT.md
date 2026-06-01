# PDF Search Endpoint Format

## Expected Format

The agent expects your PDF search endpoint (`POST /api/search`) to return JSON like this:

```json
{
  "chunks": [
    {
      "text_content": "The actual text from the PDF...",
      "id": "doc_123_chunk_45",
      "source_file_id": "file_xyz",
      "chapter": 4,
      "page_number": 42,
      "score": 0.95
    },
    {
      "text_content": "Another text chunk...",
      "id": "doc_123_chunk_46",
      "source_file_id": "file_xyz",
      "chapter": 4,
      "page_number": 43,
      "score": 0.88
    }
  ]
}
```

## What the Agent Uses

The agent only uses `text_content` from each chunk:

```json
{
  "chunks": [
    {
      "text_content": "This is what the agent reads"
    }
  ]
}
```

Other fields (id, source_file_id, etc.) are optional and not used.

## If You Get a Parse Error

Error: `invalid character 'e' looking for beginning of value`

This means the endpoint returned something that's not valid JSON. Common causes:

### 1. Endpoint Returns Error Text

Instead of:
```json
{"chunks": [...]}
```

It returns:
```
error: endpoint not found
```

**Fix:** Check your `config.json` `SEARCH_ENDPOINT` value. Should be exactly:
```json
{
  "SEARCH_ENDPOINT": "http://localhost:8081/api/search"
}
```

### 2. Endpoint Returns Different Format

Your endpoint might return:
```json
{
  "results": [...],
  "data": [...]
}
```

Instead of `chunks`.

**Fix:** Contact your API developer or check API docs. The structure must have:
```json
{
  "chunks": [
    {
      "text_content": "..."
    }
  ]
}
```

### 3. Endpoint Returns HTML (404/500)

Server might return HTML error page instead of JSON:
```html
<html><body>404 Not Found</body></html>
```

**Fix:** Verify endpoint is running:
```bash
curl -X POST http://localhost:8081/api/search \
  -H "Content-Type: application/json" \
  -d '{"query": "test"}'
```

Should return JSON, not HTML.

## Debugging Steps

1. **Check endpoint directly:**
   ```bash
   curl -X POST http://localhost:8081/api/search \
     -H "Content-Type: application/json" \
     -d '{"query": "Game of thrones"}'
   ```

2. **Copy the response** and check if it's valid JSON:
   ```bash
   # Paste response, should be pretty-printed:
   cat << 'EOF' | jq .
   {paste response here}
   EOF
   ```

3. **Run agent with logging:**
   ```bash
   ./agent
   # Make a query
   # Look for: [TOOL] search_pdf: Full response: ...
   ```

4. **Share the response** with your team to fix format mismatch.

## Expected Request Format

The agent sends to your endpoint:

```bash
POST http://localhost:8081/api/search
Content-Type: application/json

{
  "query": "Game of thrones"
}
```

Your endpoint should:
1. ✅ Accept this JSON request
2. ✅ Parse the `query` field
3. ✅ Search for matching PDF chunks
4. ✅ Return JSON with `chunks` array
5. ✅ Each chunk has `text_content`

## Common Response Formats

### Format 1: Pinecone-style
```json
{
  "chunks": [
    {
      "text_content": "...",
      "source_file_id": "file_id",
      "page_number": 42,
      "score": 0.95
    }
  ]
}
```
✅ **Works** - Agent uses `text_content`

### Format 2: Minimal
```json
{
  "chunks": [
    {
      "text_content": "..."
    }
  ]
}
```
✅ **Works** - All required fields present

### Format 3: No chunks key
```json
{
  "results": [
    {
      "text": "..."
    }
  ]
}
```
❌ **Fails** - Agent expects `chunks` key

### Format 4: String instead of object
```json
"This is the result text"
```
❌ **Fails** - Not a JSON object with chunks

### Format 5: Array instead of object
```json
[
  {"text_content": "..."},
  {"text_content": "..."}
]
```
❌ **Fails** - Agent expects object with `chunks` key

## Fixing Your Endpoint

If your endpoint returns a different format, you have two options:

### Option A: Modify Your Endpoint
Change your API to return the expected format:

```go
// Before:
return results

// After:
type Response struct {
    Chunks []struct {
        TextContent string `json:"text_content"`
        // ... other fields
    } `json:"chunks"`
}

response := Response{
    Chunks: results,
}
return response
```

### Option B: Modify the Agent Tool
Update `tools/pdf_search.go` to parse your format:

```go
var searchResp struct {
    Results []struct {  // Change "chunks" to your key
        Text string `json:"text"`  // Change "text_content" to your key
    } `json:"results"`  // Change "chunks" to your key
}

// Then adapt the chunk reading:
for _, r := range searchResp.Results {
    chunks = append(chunks, r.Text)  // Use your field name
}
```

## Testing Locally

Create a test file `test_search.sh`:

```bash
#!/bin/bash

# Test your PDF search endpoint
echo "Testing PDF search endpoint..."

RESPONSE=$(curl -s -X POST http://localhost:8081/api/search \
  -H "Content-Type: application/json" \
  -d '{"query": "test query"}')

echo "Response:"
echo "$RESPONSE" | jq . 2>/dev/null || echo "$RESPONSE"

# Check structure
echo ""
echo "Has 'chunks' key?"
echo "$RESPONSE" | jq 'has("chunks")'

echo "First chunk has 'text_content' key?"
echo "$RESPONSE" | jq '.chunks[0] | has("text_content")'

echo "First chunk text_content:"
echo "$RESPONSE" | jq '.chunks[0].text_content'
```

Run it:
```bash
chmod +x test_search.sh
./test_search.sh
```

## If Still Having Issues

1. Run agent with your query
2. Look for: `[TOOL] search_pdf: Full response:`
3. Copy the entire response from logs
4. Verify it's valid JSON: `echo '...' | jq .`
5. Check it has `chunks` with `text_content` fields
6. Share response format with your API developer to fix

The agent is working correctly - the issue is the endpoint format doesn't match expectations!
