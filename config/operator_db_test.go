package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestInsertAndResolveOperatorEvents(t *testing.T) {
	dir := t.TempDir()
	db, err := NewDatabase(filepath.Join(dir, "cfg.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	until := time.Now().UTC().Add(2 * time.Hour)
	if _, err := db.InsertOperatorEvent("grok", "pause_opens", "NIM slow", &until); err != nil {
		t.Fatal(err)
	}
	if _, err := db.InsertOperatorEvent("grok", "note", "no new ETH", &until); err != nil {
		t.Fatal(err)
	}
	d, err := db.CurrentOperatorDirective(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if !d.PauseOpens || d.PauseActor != "grok" {
		t.Fatalf("pause: %+v", d)
	}
	if len(d.Notes) != 1 {
		t.Fatalf("notes: %+v", d.Notes)
	}

	if _, err := db.InsertOperatorEvent("web-ui", "resume_opens", "", nil); err != nil {
		t.Fatal(err)
	}
	d, err = db.CurrentOperatorDirective(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if d.PauseOpens {
		t.Fatalf("resume should clear pause: %+v", d)
	}
}
