package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

func runReAct(userQuery string) (string, error) {
	log.Printf("[AGENT] Starting ReAct loop for query: %s", userQuery)

	schemas := toolMgr.Schemas()
	toolNames := make([]string, 0, len(schemas))
	for _, s := range schemas {
		toolNames = append(toolNames, s.Name)
	}

	systemPrompt := fmt.Sprintf(`You are a helpful assistant with access to tools for research and database management.

AVAILABLE TOOLS:
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
- Maximum %d steps allowed`, toolMgr.BuildPromptSection(), strings.Join(toolNames, " | "), cfg.MaxSteps)

	state := &ReActState{}
	messages := []string{fmt.Sprintf("User question: %s", userQuery)}

	for state.Steps < cfg.MaxSteps {
		state.Steps++
		log.Printf("[AGENT] Step %d of %d", state.Steps, cfg.MaxSteps)

		user := strings.Join(messages, "\n\n")
		log.Printf("[AGENT] Calling %s", llmCaller.ModelName())
		response, err := llmCaller.Call(systemPrompt, user)
		if err != nil {
			log.Printf("[AGENT] ERROR calling LLM: %v", err)
			return "", err
		}

		preview := response[:min(len(response), 200)]
		log.Printf("[AGENT] LLM Response (step %d): %s", state.Steps, preview)
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

			result, err := toolMgr.Execute(action, input)
			var observation string
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
