package config

import (
	"database/sql"
	"fmt"
	"time"
)

func (d *Database) ensureOperatorEventsTable() error {
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS operator_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			actor TEXT NOT NULL,
			action TEXT NOT NULL,
			note TEXT NOT NULL DEFAULT '',
			expires_at DATETIME
		)
	`)
	return err
}

// InsertOperatorEvent appends an intervention. It never updates older rows.
func (d *Database) InsertOperatorEvent(actor, action, note string, expiresAt *time.Time) (*OperatorEvent, error) {
	action, err := NormalizeOperatorAction(action)
	if err != nil {
		return nil, err
	}
	actor = NormalizeActor(actor)
	note = ClampNote(note)
	if action == OperatorNote && note == "" {
		return nil, fmt.Errorf("note action requires a note")
	}

	var res sql.Result
	if expiresAt == nil {
		res, err = d.db.Exec(`
			INSERT INTO operator_events (ts, actor, action, note, expires_at)
			VALUES (datetime('now'), ?, ?, ?, NULL)
		`, actor, action, note)
	} else {
		res, err = d.db.Exec(`
			INSERT INTO operator_events (ts, actor, action, note, expires_at)
			VALUES (datetime('now'), ?, ?, ?, ?)
		`, actor, action, note, expiresAt.UTC().Format("2006-01-02 15:04:05"))
	}
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return d.GetOperatorEvent(id)
}

func (d *Database) GetOperatorEvent(id int64) (*OperatorEvent, error) {
	row := d.db.QueryRow(`
		SELECT id, ts, actor, action, COALESCE(note,''), expires_at
		FROM operator_events WHERE id = ?
	`, id)
	return scanOperatorEvent(row)
}

// ListOperatorEvents returns newest-first, capped.
func (d *Database) ListOperatorEvents(limit int) ([]OperatorEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := d.db.Query(`
		SELECT id, ts, actor, action, COALESCE(note,''), expires_at
		FROM operator_events
		ORDER BY id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]OperatorEvent, 0)
	for rows.Next() {
		e, err := scanOperatorEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

// ListOperatorEventsChrono returns oldest-first for ResolveDirective.
func (d *Database) ListOperatorEventsChrono(limit int) ([]OperatorEvent, error) {
	newest, err := d.ListOperatorEvents(limit)
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(newest)-1; i < j; i, j = i+1, j-1 {
		newest[i], newest[j] = newest[j], newest[i]
	}
	return newest, nil
}

func (d *Database) CurrentOperatorDirective(now time.Time) (OperatorDirective, error) {
	events, err := d.ListOperatorEventsChrono(200)
	if err != nil {
		return OperatorDirective{}, err
	}
	return ResolveDirective(events, now), nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanOperatorEvent(s scanner) (*OperatorEvent, error) {
	var e OperatorEvent
	var ts string
	var exp sql.NullString
	if err := s.Scan(&e.ID, &ts, &e.Actor, &e.Action, &e.Note, &exp); err != nil {
		return nil, err
	}
	parsed, err := parseSQLiteTime(ts)
	if err != nil {
		e.Ts = time.Now().UTC()
	} else {
		e.Ts = parsed
	}
	if exp.Valid && exp.String != "" {
		if t, err := parseSQLiteTime(exp.String); err == nil {
			e.ExpiresAt = &t
		}
	}
	return &e, nil
}

func parseSQLiteTime(raw string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("parse time %q", raw)
}
