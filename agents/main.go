package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/user/agentic-ai/tools"
)

type Config struct {
	OllamaHost      string `json:"OLLAMA_HOST"`
	OllamaModel     string `json:"OLLAMA_MODEL"`
	TavilyAPIKey    string `json:"TAVILY_API_KEY"`
	MaxSteps        int    `json:"MAX_STEPS"`
	MaxRetries      int    `json:"MAX_RETRIES"`
	MaxTokens       int    `json:"MAX_TOKENS"`
	SearchEndpoint  string `json:"SEARCH_ENDPOINT"`
	MCPServerURL    string `json:"MCP_SERVER_URL"`
}

var cfg Config
var toolMgr *tools.Manager
var mcpToolsDesc string // populated at startup from tools/list

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	data, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatalf("Failed to read config.json: %v", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("Failed to parse config.json: %v", err)
	}
}

type AgentRequest struct {
	Query string `json:"query"`
}

type AgentResponse struct {
	Answer string `json:"answer"`
}

type ReActState struct {
	Steps  int
	Answer string
}

type OllamaRequest struct {
	Model   string `json:"model"`
	Prompt  string `json:"prompt"`
	Stream  bool   `json:"stream"`
	Options map[string]interface{} `json:"options"`
}

type OllamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}


func callOllama(prompt string) (string, error) {
	log.Printf("[OLLAMA] Calling Ollama at %s with model '%s' (max_tokens=%d)", cfg.OllamaHost, cfg.OllamaModel, cfg.MaxTokens)
	ollamaReq := OllamaRequest{
		Model:  cfg.OllamaModel,
		Prompt: prompt,
		Stream: false,
		Options: map[string]interface{}{
			"num_predict": cfg.MaxTokens,
			"temperature": 0.7,
		},
	}

	bodyBytes, _ := json.Marshal(ollamaReq)

	resp, err := http.Post(cfg.OllamaHost+"/api/generate", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		log.Printf("[OLLAMA] ERROR: Connection failed: %v", err)
		return "", fmt.Errorf("ollama request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[OLLAMA] ERROR: Status %d returned", resp.StatusCode)
		return "", fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	log.Printf("[OLLAMA] Request successful, parsing response...")

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return "", err
	}

	return ollamaResp.Response, nil
}

func runReAct(userQuery string) (string, error) {
	log.Printf("[AGENT] Starting ReAct loop for query: %s", userQuery)
	// Build the list of available tool names for the Action line
	toolNames := make([]string, 0, len(toolMgr.ListTools()))
	for name := range toolMgr.ListTools() {
		toolNames = append(toolNames, name)
	}

	dbSection := ""
	if mcpToolsDesc != "" {
		dbSection = "\nDB TOOLS (via MCP server — input must be a JSON object):\n" + mcpToolsDesc
	}

	systemPrompt := fmt.Sprintf(`You are a helpful assistant with access to tools for research and database management.

AVAILABLE TOOLS:
1. search_pdf - Searches the user's PDF documents. Use this FIRST for factual questions.
2. web_search - Searches the internet. Use as fallback when PDF has no results.
%s
REASONING FORMAT (ReAct):
You MUST follow this exact format for each reasoning step:

Thought: [your reasoning about what to do next]
Action: %s
Action Input: [plain text query for search tools | JSON object for DB tools]
Observation: [result from the tool]

When you have sufficient information:
Final Answer: [your complete answer]

RULES:
- Follow Thought-Action-Observation format exactly
- DB tool inputs must be valid JSON objects
- Search tool inputs are plain text
- Never invent tool names — only use the ones listed above
- Maximum %d steps allowed`, dbSection, strings.Join(toolNames, " | "), cfg.MaxSteps)

	state := &ReActState{
		Steps:  0,
		Answer: "",
	}

	userMessage := fmt.Sprintf("User question: %s", userQuery)
	messages := []string{systemPrompt, userMessage}

	for state.Steps < cfg.MaxSteps {
		state.Steps++
		log.Printf("[AGENT] Step %d of %d", state.Steps, cfg.MaxSteps)

		prompt := strings.Join(messages, "\n\n")
		log.Printf("[AGENT] Calling Ollama model: %s", cfg.OllamaModel)
		response, err := callOllama(prompt)
		if err != nil {
			log.Printf("[AGENT] ERROR calling Ollama: %v", err)
			return "", err
		}

		log.Printf("[AGENT] LLM Response (step %d): %s", state.Steps, response[:min(len(response), 200)])
		messages = append(messages, response)

		if strings.Contains(response, "Final Answer:") {
			log.Printf("[AGENT] Found 'Final Answer' - exiting loop")
			parts := strings.Split(response, "Final Answer:")
			if len(parts) > 1 {
				state.Answer = strings.TrimSpace(parts[1])
			}
			break
		}

		if strings.Contains(response, "Action:") && strings.Contains(response, "Action Input:") {
			actionRe := regexp.MustCompile(`Action:\s*(\w+)`)
			inputRe := regexp.MustCompile(`Action Input:\s*(.+?)(?:\n|$)`)

			actionMatch := actionRe.FindStringSubmatch(response)
			inputMatch := inputRe.FindStringSubmatch(response)

			if len(actionMatch) < 2 || len(inputMatch) < 2 {
				log.Printf("[AGENT] Step %d: Could not parse Action/Input from response", state.Steps)
				continue
			}

			action := actionMatch[1]
			input := strings.TrimSpace(inputMatch[1])

			log.Printf("[AGENT] Step %d: Action = '%s', Input = '%s'", state.Steps, action, input)

			var observation string

			log.Printf("[AGENT] Step %d: Calling tool '%s'", state.Steps, action)
			result, err := toolMgr.Execute(action, input)
			if err != nil {
				log.Printf("[AGENT] Step %d: Tool '%s' ERROR: %v", state.Steps, action, err)
				observation = fmt.Sprintf("Error: %v", err)
			} else {
				log.Printf("[AGENT] Step %d: Tool '%s' SUCCESS - got %d chars", state.Steps, action, len(result))
				observation = result
			}

			messages = append(messages, fmt.Sprintf("Observation: %s", observation))
		} else {
			log.Printf("[AGENT] Step %d: No Action/Input found in response", state.Steps)
		}
	}

	if state.Steps >= cfg.MaxSteps {
		log.Printf("[AGENT] Reached max steps (%d) - returning best answer found", cfg.MaxSteps)
	}

	if state.Answer == "" {
		if len(messages) > 0 {
			state.Answer = messages[len(messages)-1]
		} else {
			state.Answer = "Unable to generate an answer after maximum steps."
		}
	}

	return state.Answer, nil
}

func handleAgentQuery(w http.ResponseWriter, r *http.Request) {
	log.Printf("[API] POST /api/agent/query received")

	if r.Method != http.MethodPost {
		log.Printf("[API] ERROR: Invalid method %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[API] ERROR: Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		log.Printf("[API] ERROR: Query is empty")
		http.Error(w, "Query is required", http.StatusBadRequest)
		return
	}

	log.Printf("[API] Processing query: %s", req.Query)
	answer, err := runReAct(req.Query)
	if err != nil {
		log.Printf("[API] ERROR: ReAct failed: %v", err)
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("[API] SUCCESS: Returning answer (%d chars)", len(answer))
	resp := AgentResponse{Answer: answer}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func serveStaticFile(filePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filePath)
	}
}

func main() {
	port := flag.String("port", "8082", "Port to listen on")
	flag.Parse()

	// Get static directory
	staticDir := filepath.Join(".", "static")
	if _, err := os.Stat(staticDir); err != nil {
		log.Fatalf("static directory not found: %v", err)
	}

	// Initialize tool manager with static tools
	toolMgr = tools.NewManager()
	toolMgr.Register(tools.NewPDFSearchTool(cfg.SearchEndpoint, cfg.MaxRetries))
	toolMgr.Register(tools.NewWebSearchTool(cfg.TavilyAPIKey, cfg.MaxRetries))

	// Connect to MCP server and discover tools via tools/list
	mcpURL := cfg.MCPServerURL
	if mcpURL == "" {
		mcpURL = "http://localhost:8083"
	}
	mcpClient, err := tools.Connect(context.Background(), mcpURL)
	if err != nil {
		log.Printf("[MCP] WARNING: Could not connect to MCP server at %s: %v (DB tools unavailable)", mcpURL, err)
	} else {
		discovered, desc, err := mcpClient.DiscoverTools(context.Background())
		if err != nil {
			log.Printf("[MCP] WARNING: tools/list failed: %v", err)
		} else {
			for _, t := range discovered {
				toolMgr.Register(t)
			}
			mcpToolsDesc = desc
			log.Printf("[MCP] Discovered %d tools from %s", len(discovered), mcpURL)
		}
	}

	log.Printf("[MAIN] Tool manager initialized with %d tools", len(toolMgr.ListTools()))

	// API endpoints
	http.HandleFunc("/api/agent/query", handleAgentQuery)

	// Static files - serve directly
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
		} else {
			http.ServeFile(w, r, filepath.Join(staticDir, r.URL.Path))
		}
	})

	log.Printf("Agent server listening on http://localhost:%s", *port)
	log.Printf("Endpoint: POST http://localhost:%s/api/agent/query", *port)
	log.Printf("UI: http://localhost:%s", *port)

	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

