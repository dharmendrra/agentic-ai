package main

import (
	"fmt"
	"log"
	"strings"
)

// maybeSummarize folds older-than-K turns into the conversation's rolling summary
// when their count exceeds SUMMARY_TRIGGER_TURNS. Recent turns stay verbatim;
// the summary is one LLM call. See PLAN §6. Best-effort: failures are logged, not fatal.
func maybeSummarize(convID string) {
	if store == nil || convID == "" {
		return
	}

	conv, err := store.GetConversation(convID)
	if err != nil || conv == nil {
		if err != nil {
			log.Printf("[SUMMARY] WARN: load conversation: %v", err)
		}
		return
	}

	// All messages, ascending.
	all, err := store.GetConversationMessages(convID)
	if err != nil {
		log.Printf("[SUMMARY] WARN: load messages: %v", err)
		return
	}

	keepVerbatim := cfg.HistoryTurns * 2 // user + assistant per turn
	if len(all) <= keepVerbatim {
		return
	}

	// Messages eligible to be folded: those older than the last K turns and not
	// already summarized.
	cutoffIdx := len(all) - keepVerbatim // [0, cutoffIdx) are "older"
	var older []Message
	for _, m := range all[:cutoffIdx] {
		if m.Seq > conv.SummaryUptoSeq {
			older = append(older, m)
		}
	}

	if len(older) < cfg.SummaryTriggerTurns {
		return
	}

	log.Printf("[SUMMARY] conv=%s folding %d older messages into rolling summary", convID, len(older))

	var sb strings.Builder
	if strings.TrimSpace(conv.Summary) != "" {
		sb.WriteString("Existing summary so far:\n")
		sb.WriteString(conv.Summary)
		sb.WriteString("\n\nNewer messages to fold in:\n")
	} else {
		sb.WriteString("Conversation messages to summarize:\n")
	}
	for _, m := range older {
		sb.WriteString(fmt.Sprintf("%s: %s\n", roleLabel(m.Role), m.Content))
	}

	system := "You compress chat history into a concise running summary. " +
		"Preserve key facts, decisions, the user's goals, and any unresolved questions. " +
		"Write 4-8 sentences of plain prose. Do not add commentary."

	summary, err := llmCaller.Call(system, sb.String())
	if err != nil {
		log.Printf("[SUMMARY] WARN: LLM summarization failed: %v", err)
		return
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return
	}

	uptoSeq := older[len(older)-1].Seq
	if err := store.UpdateSummary(convID, summary, uptoSeq); err != nil {
		log.Printf("[SUMMARY] WARN: persist summary: %v", err)
		return
	}
	log.Printf("[SUMMARY] conv=%s summary updated (upto_seq=%d, %d chars)", convID, uptoSeq, len(summary))
}
