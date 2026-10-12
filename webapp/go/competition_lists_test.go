package isuports

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestCompetitionListCacheTracksInsertAndFinish(t *testing.T) {
	raw := sqlx.MustOpen("sqlite3", ":memory:")
	raw.SetMaxOpenConns(1)
	defer raw.Close()
	raw.MustExec("CREATE TABLE competition(id TEXT PRIMARY KEY,tenant_id INTEGER,title TEXT,finished_at INTEGER,created_at INTEGER,updated_at INTEGER)")
	db := &tenantConnection{DB: raw, reads: make(map[string]*sqlx.Stmt)}
	ctx := context.Background()
	query := "SELECT * FROM competition WHERE tenant_id = ? ORDER BY created_at ASC"
	read := func() []CompetitionRow {
		var rows []CompetitionRow
		if err := db.SelectContext(ctx, &rows, query, int64(7)); err != nil {
			t.Fatal(err)
		}
		return rows
	}
	if len(read()) != 0 {
		t.Fatal("unexpected rows")
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO competition (id, tenant_id, title, finished_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)", "a", 7, "A", nil, 1, 1); err != nil {
		t.Fatal(err)
	}
	if rows := read(); len(rows) != 1 || rows[0].FinishedAt.Valid {
		t.Fatal(rows)
	}
	if _, err := db.ExecContext(ctx, "UPDATE competition SET finished_at = ?, updated_at = ? WHERE id = ?", 100, 100, "a"); err != nil {
		t.Fatal(err)
	}
	if rows := read(); len(rows) != 1 || rows[0].FinishedAt.Int64 != 100 {
		t.Fatal("stale list", rows)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO competition (id, tenant_id, title, finished_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)", "b", 7, "B", nil, 2, 2); err != nil {
		t.Fatal(err)
	}
	if rows := read(); len(rows) != 2 || rows[0].ID != "a" || rows[1].ID != "b" {
		t.Fatal(rows)
	}
	var other []CompetitionRow
	if err := db.SelectContext(ctx, &other, query, int64(8)); err != nil || len(other) != 0 {
		t.Fatal("tenant list isolation", other, err)
	}
}
