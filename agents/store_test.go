package main

import (
	"testing"
)

func TestMakeTitle(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"   How do binary trees balance?  ", "How do binary trees balance?"},
		{"", "New conversation"},
		{"   ", "New conversation"},
		{"a b   c", "a b c"},
	}
	for _, c := range cases {
		if got := makeTitle(c.in); got != c.want {
			t.Errorf("makeTitle(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	long := ""
	for i := 0; i < 100; i++ {
		long += "x"
	}
	got := makeTitle(long)
	if len([]rune(got)) > 62 { // 60 chars + ellipsis-ish
		t.Errorf("makeTitle did not truncate long title: len=%d", len([]rune(got)))
	}
}

func TestRegexpQuoteMeta(t *testing.T) {
	in := "a.b*c(d)"
	want := `a\.b\*c\(d\)`
	if got := regexpQuoteMeta(in); got != want {
		t.Errorf("regexpQuoteMeta(%q) = %q, want %q", in, got, want)
	}
}

func TestApproxTokens(t *testing.T) {
	if approxTokens("abcd") != 1 {
		t.Errorf("approxTokens(4 chars) = %d, want 1", approxTokens("abcd"))
	}
	if approxTokens("") != 0 {
		t.Errorf("approxTokens(empty) = %d, want 0", approxTokens(""))
	}
}

func TestSourcesList(t *testing.T) {
	set := map[string]bool{"web": true, "pdf": true}
	got := sourcesList(set)
	// Stable order: pdf, web, mongo
	if len(got) != 2 || got[0] != "pdf" || got[1] != "web" {
		t.Errorf("sourcesList = %v, want [pdf web]", got)
	}
	if len(sourcesList(map[string]bool{})) != 0 {
		t.Errorf("sourcesList(empty) should be empty")
	}
}

func TestRoleLabel(t *testing.T) {
	if roleLabel("assistant") != "Assistant" {
		t.Errorf("roleLabel(assistant) wrong")
	}
	if roleLabel("user") != "User" {
		t.Errorf("roleLabel(user) wrong")
	}
}
