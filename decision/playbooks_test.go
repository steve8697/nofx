package decision

import "testing"

func TestSelectPlaybooksEmptyAccount(t *testing.T) {
	got := SelectPlaybooks(0, 3)
	want := []string{"entry", "filters", "sizing", "context"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestSelectPlaybooksAtMaxSkipsEntry(t *testing.T) {
	got := SelectPlaybooks(3, 3)
	for _, n := range got {
		if n == "entry" || n == "filters" || n == "sizing" {
			t.Fatalf("maxed account should not inject %s: %v", n, got)
		}
	}
	hasManage, hasCtx := false, false
	for _, n := range got {
		if n == "manage" {
			hasManage = true
		}
		if n == "context" {
			hasCtx = true
		}
	}
	if !hasManage || !hasCtx {
		t.Fatalf("expected manage+context, got %v", got)
	}
}
