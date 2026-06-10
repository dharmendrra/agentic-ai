package tools

import (
	"strings"
	"testing"
)

type fakeProvider struct {
	all   []RecalledMessage
	first []RecalledMessage
	found []RecalledMessage
}

func (f *fakeProvider) AllUserMessages() ([]RecalledMessage, error)         { return f.all, nil }
func (f *fakeProvider) FirstNUserMessages(n int) ([]RecalledMessage, error) { return f.first, nil }
func (f *fakeProvider) Search(q string) ([]RecalledMessage, error)          { return f.found, nil }

func TestRecallHistoryAll(t *testing.T) {
	p := &fakeProvider{all: []RecalledMessage{{Seq: 1, Role: "user", Content: "first q"}}}
	tool := NewRecallHistoryTool(p)
	out, err := tool.Execute(`{"mode":"all"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "first q") {
		t.Errorf("expected 'first q' in output, got: %s", out)
	}
}

func TestRecallHistoryFirstN(t *testing.T) {
	p := &fakeProvider{first: []RecalledMessage{{Seq: 1, Role: "user", Content: "early"}}}
	tool := NewRecallHistoryTool(p)
	out, err := tool.Execute(`{"mode":"first_n","n":1}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "early") {
		t.Errorf("expected 'early', got: %s", out)
	}
}

func TestRecallHistorySearchRequiresQuery(t *testing.T) {
	tool := NewRecallHistoryTool(&fakeProvider{})
	if _, err := tool.Execute(`{"mode":"search"}`); err == nil {
		t.Error("expected error for search without query")
	}
}

func TestRecallHistoryUnknownMode(t *testing.T) {
	tool := NewRecallHistoryTool(&fakeProvider{})
	if _, err := tool.Execute(`{"mode":"delete"}`); err == nil {
		t.Error("expected error for unknown mode")
	}
}

func TestRecallHistoryEmpty(t *testing.T) {
	tool := NewRecallHistoryTool(&fakeProvider{})
	out, err := tool.Execute(`{"mode":"all"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "No matching") {
		t.Errorf("expected empty message, got: %s", out)
	}
}

func TestRecallHistoryNilProvider(t *testing.T) {
	tool := NewRecallHistoryTool(nil)
	out, _ := tool.Execute(`{"mode":"all"}`)
	if !strings.Contains(out, "unavailable") {
		t.Errorf("expected unavailable message, got: %s", out)
	}
}
