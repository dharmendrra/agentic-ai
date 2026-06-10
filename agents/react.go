package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/user/agentic-ai/tools"
)

// approxTokens is a cheap len/4 token estimate (no tokenizer dependency).
func approxTokens(s string) int { return len(s) / 4 }

// buildActiveToolSet assembles the per-request tool registry based on the
// source toggles. recall_history is ALWAYS included (memory is not a knowledge
// source). search_pdf + mcp are added only with My Library; web_search only
// with Web. See PLAN §4.5.
func buildActiveToolSet(convID string, useWeb, useLibrary bool) *tools.Manager {
	m := tools.NewManager()

	// Memory tool — read-only, conversation-scoped. Only when a store exists.
	if store != nil && convID != "" {
		m.Register(tools.NewRecallHistoryTool(newHistoryProvider(store, convID)))
	}

	if useLibrary {
		if pdfTool != nil {
			m.Register(pdfTool)
		}
		if mcpTool != nil {
			m.Register(mcpTool)
		}
	}
	if useWeb {
		if webTool != nil {
			m.Register(webTool)
		}
	}
	return m
}

// buildSystemPrompt branches the instructions by toggle state and lists the
// active tools.
func buildSystemPrompt(m *tools.Manager, useWeb, useLibrary bool) string {
	schemas := m.Schemas()
	toolNames := make([]string, 0, len(schemas))
	for _, s := range schemas {
		toolNames = append(toolNames, s.Name)
	}

	var guidance string
	switch {
	case !useWeb && !useLibrary:
		guidance = `SOURCE MODE: No retrieval sources are enabled.
- Answer from your own knowledge only.
- If you do not know the answer, say so plainly and suggest the user enable the 🌐 Web toggle.`
	case useLibrary && !useWeb:
		guidance = `SOURCE MODE: My Library only (internal: PDF book search + MongoDB).
- Use search_pdf for factual/book questions; use the mcp tool for database operations.
- When the user names a specific book, you MUST pass it in the "book" field by setting Action Input to a JSON object, e.g. {"query":"who sits on the iron throne","book":"Game of Thrones"}. This narrows the search to that book.
- Do NOT claim information from the web.
- If search_pdf returns "[NEEDS_CLARIFICATION|...]", do NOT guess — emit a "Clarification:" line asking which of the listed books they mean.`
	case useWeb && !useLibrary:
		guidance = `SOURCE MODE: Web only (Tavily).
- Use web_search for up-to-date or external information.
- Do NOT claim information from the internal library.`
	default: // both on
		guidance = `SOURCE MODE: Both My Library (PDF + MongoDB) and Web are enabled.
- Prefer internal library sources, then supplement/verify with web_search, and merge into one answer.
- When the user names a specific book, pass it in search_pdf's "book" field as JSON: {"query":"...","book":"Game of Thrones"}.
- If search_pdf returns "[NEEDS_CLARIFICATION|...]", emit a "Clarification:" line instead of guessing.`
	}

	return fmt.Sprintf(`You are a helpful conversational assistant.

AVAILABLE TOOLS:
%s
REASONING FORMAT (ReAct):
You MUST follow this exact format for each reasoning step:

Thought: [your reasoning about what to do next]
Action: %s
Action Input: [plain text query, OR a JSON object — see rules]
Observation: [result from the tool]

When you have sufficient information:
Final Answer: [your complete answer]

If you need the user to disambiguate before you can answer (e.g. which book), instead emit:
Clarification: [your question back to the user]

%s

RULES:
- Follow the Thought-Action-Observation format exactly.
- DB and memory tool inputs (mcp, recall_history) must be valid JSON objects.
- web_search input is plain text. search_pdf input is plain text for a general search, OR a JSON object {"query":"...","book":"..."} — use the JSON form with "book" whenever the user names a specific book.
- Never invent tool names — only use the ones listed above.
- The user's earlier messages are provided as conversation context; for exact recall of earlier questions use the recall_history tool.
- Maximum %d steps allowed.`,
		m.BuildPromptSection(),
		strings.Join(toolNames, " | "),
		guidance,
		cfg.MaxSteps)
}

// assembleContext builds the conversation context block within the token budget:
// rolling summary (oldest, compressed) + last K turns verbatim, newest-first
// until the budget is hit. See PLAN §6.
func assembleContext(convID string) string {
	if store == nil || convID == "" {
		return ""
	}

	var sb strings.Builder
	budget := cfg.HistoryTokenBudget

	conv, err := store.GetConversation(convID)
	if err != nil {
		log.Printf("[AGENT] WARN: could not load conversation %s: %v", convID, err)
	}
	if conv != nil && strings.TrimSpace(conv.Summary) != "" {
		summaryBlock := "Summary of earlier conversation:\n" + conv.Summary + "\n\n"
		budget -= approxTokens(summaryBlock)
		sb.WriteString(summaryBlock)
	}

	recent, err := store.GetRecentMessages(convID, cfg.HistoryTurns*2) // user+assistant per turn
	if err != nil {
		log.Printf("[AGENT] WARN: could not load recent messages for %s: %v", convID, err)
		return sb.String()
	}

	// Skip messages already folded into the summary.
	var kept []Message
	for _, m := range recent {
		if conv != nil && m.Seq <= conv.SummaryUptoSeq {
			continue
		}
		kept = append(kept, m)
	}

	// Fill newest-first until the budget is hit, then render oldest-first.
	var lines []string
	for i := len(kept) - 1; i >= 0; i-- {
		m := kept[i]
		line := fmt.Sprintf("%s: %s", roleLabel(m.Role), m.Content)
		cost := approxTokens(line)
		if budget-cost < 0 && len(lines) > 0 {
			break
		}
		budget -= cost
		lines = append([]string{line}, lines...)
	}

	if len(lines) > 0 {
		sb.WriteString("Recent conversation:\n")
		sb.WriteString(strings.Join(lines, "\n"))
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func roleLabel(role string) string {
	if role == "assistant" {
		return "Assistant"
	}
	return "User"
}

// runReAct runs one conversational turn. It builds the active tool set, assembles
// budgeted context, runs the ReAct loop (tool observations are transient and never
// persisted), and returns the final answer, the sources that fed it, and whether
// the model asked for clarification.
// splitWebCitations separates the human-readable web_search observation from the
// trailing [[WEBSRC]] marker, returning the cleaned text + parsed citations.
func splitWebCitations(result string) (string, []Citation) {
	idx := strings.Index(result, tools.WebSrcMarker)
	if idx < 0 {
		return result, nil
	}
	cleaned := strings.TrimSpace(result[:idx])
	raw := result[idx+len(tools.WebSrcMarker):]
	var cites []Citation
	for _, item := range strings.Split(raw, " || ") {
		parts := strings.SplitN(item, " :: ", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
			cites = append(cites, Citation{Title: strings.TrimSpace(parts[0]), URL: strings.TrimSpace(parts[1])})
		}
	}
	return cleaned, cites
}

func runReAct(convID, userQuery string, useWeb, useLibrary bool) (answer string, sources []string, citations []Citation, needsClarify bool, err error) {
	log.Printf("[AGENT] ReAct turn conv=%s web=%v library=%v query=%s", convID, useWeb, useLibrary, userQuery)

	activeTools := buildActiveToolSet(convID, useWeb, useLibrary)
	systemPrompt := buildSystemPrompt(activeTools, useWeb, useLibrary)

	context := assembleContext(convID)

	state := &ReActState{}
	messages := []string{}
	if context != "" {
		messages = append(messages, context)
	}
	messages = append(messages, fmt.Sprintf("Current user question: %s", userQuery))

	usedSources := map[string]bool{}

	for state.Steps < cfg.MaxSteps {
		state.Steps++
		log.Printf("[AGENT] Step %d of %d", state.Steps, cfg.MaxSteps)

		user := strings.Join(messages, "\n\n")
		log.Printf("[AGENT] Calling %s", llmCaller.ModelName())
		response, callErr := llmCaller.Call(systemPrompt, user)
		if callErr != nil {
			log.Printf("[AGENT] ERROR calling LLM: %v", callErr)
			return "", nil, nil, false, callErr
		}

		preview := response[:min(len(response), 200)]
		log.Printf("[AGENT] LLM Response (step %d): %s", state.Steps, preview)
		messages = append(messages, response)

		// Clarify-back: treated like Final Answer, but flags needs_clarification.
		if strings.Contains(response, "Clarification:") {
			log.Printf("[AGENT] Found 'Clarification' - exiting loop")
			parts := strings.Split(response, "Clarification:")
			if len(parts) > 1 {
				state.Answer = strings.TrimSpace(parts[1])
			}
			return state.Answer, sourcesList(usedSources), citations, true, nil
		}

		if strings.Contains(response, "Final Answer:") {
			log.Printf("[AGENT] Found 'Final Answer' - exiting loop")
			parts := strings.Split(response, "Final Answer:")
			if len(parts) > 1 {
				state.Answer = strings.TrimSpace(parts[1])
			}
			break
		}

		// Plain-LLM mode: with no retrieval tools enabled there is nothing to
		// iterate on, so a direct response (no tool Action) IS the answer — accept
		// it instead of looping to MaxSteps. Weak local models often answer
		// without the "Final Answer:" prefix. See PLAN §4.5 (no-tools branch).
		if !useWeb && !useLibrary && !strings.Contains(response, "Action:") {
			log.Printf("[AGENT] No tools active and no Action - treating response as final answer")
			state.Answer = strings.TrimSpace(response)
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

			// Some models emit "Action: Final Answer" instead of "Final Answer:".
			// Treat it as the final answer rather than a nonexistent tool.
			if strings.EqualFold(action, "final") || strings.EqualFold(action, "finalanswer") {
				if i := strings.Index(response, "Action Input:"); i >= 0 {
					state.Answer = strings.TrimSpace(response[i+len("Action Input:"):])
				} else {
					state.Answer = input
				}
				log.Printf("[AGENT] Step %d: 'Action: Final' treated as Final Answer", state.Steps)
				break
			}

			result, execErr := activeTools.Execute(action, input)
			var observation string
			if execErr != nil {
				log.Printf("[AGENT] Step %d: Tool '%s' ERROR: %v", state.Steps, action, execErr)
				observation = fmt.Sprintf("Error: %v", execErr)
			} else {
				log.Printf("[AGENT] Step %d: Tool '%s' SUCCESS - got %d chars", state.Steps, action, len(result))
				observation = result
				// Track which knowledge sources actually fed this answer.
				switch action {
				case "search_pdf":
					if !strings.HasPrefix(result, "[PDF_EMPTY") {
						usedSources["pdf"] = true
					}
				case "web_search":
					usedSources["web"] = true
					cleaned, cites := splitWebCitations(result)
					citations = append(citations, cites...)
					observation = cleaned
				case "mcp":
					usedSources["mongo"] = true
				}
			}
			// Tool observations are TRANSIENT — they live only inside this loop
			// and are discarded after the answer is formed (never persisted).
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

	return state.Answer, sourcesList(usedSources), citations, false, nil
}

// sourcesList flattens the used-sources set into a stable slice.
func sourcesList(set map[string]bool) []string {
	var out []string
	for _, k := range []string{"pdf", "web", "mongo"} {
		if set[k] {
			out = append(out, k)
		}
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
