package main

import "github.com/user/agentic-ai/tools"

// historyProvider adapts the Mongo Store to tools.HistoryProvider, scoped to a
// single conversation. The convID is fixed at construction by the backend, so
// the model can never read another conversation or perform writes/deletes.
type historyProvider struct {
	store  *Store
	convID string
}

func newHistoryProvider(s *Store, convID string) *historyProvider {
	return &historyProvider{store: s, convID: convID}
}

func toRecalled(msgs []Message) []tools.RecalledMessage {
	out := make([]tools.RecalledMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, tools.RecalledMessage{Seq: m.Seq, Role: m.Role, Content: m.Content})
	}
	return out
}

func (h *historyProvider) AllUserMessages() ([]tools.RecalledMessage, error) {
	msgs, err := h.store.GetAllUserMessages(h.convID)
	if err != nil {
		return nil, err
	}
	return toRecalled(msgs), nil
}

func (h *historyProvider) FirstNUserMessages(n int) ([]tools.RecalledMessage, error) {
	msgs, err := h.store.GetFirstNUserMessages(h.convID, n)
	if err != nil {
		return nil, err
	}
	return toRecalled(msgs), nil
}

func (h *historyProvider) Search(query string) ([]tools.RecalledMessage, error) {
	msgs, err := h.store.SearchMessages(h.convID, query)
	if err != nil {
		return nil, err
	}
	return toRecalled(msgs), nil
}
