package isuports

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

func TestMetadataCommitInvalidationAndMissingThenInsert(t *testing.T) {
	raw := sqlx.MustOpen("sqlite3", ":memory:")
	raw.SetMaxOpenConns(1)
	defer raw.Close()
	raw.MustExec(`CREATE TABLE player(tenant_id INTEGER,id TEXT PRIMARY KEY,display_name TEXT,is_disqualified INTEGER,created_at INTEGER,updated_at INTEGER);
 CREATE TABLE competition(tenant_id INTEGER,id TEXT PRIMARY KEY,title TEXT,finished_at INTEGER,created_at INTEGER,updated_at INTEGER);
 INSERT INTO player VALUES(7,'p','P',0,0,0);
 INSERT INTO competition VALUES(7,'c','C',NULL,0,0);`)
	db := &tenantConnection{DB: raw, reads: make(map[string]*sqlx.Stmt)}
	ctx := context.Background()
	if err := authorizePlayer(ctx, db, "p"); err != nil {
		t.Fatal(err)
	}
	comp, err := retrieveCompetition(ctx, db, "c")
	if err != nil || comp.FinishedAt.Valid {
		t.Fatal(comp, err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			retrievePlayer(ctx, db, "p")
		}
	}()
	if _, err := db.ExecContext(ctx, "UPDATE player SET is_disqualified = ?, updated_at = ? WHERE id = ?", true, 1, "p"); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	var denied *echo.HTTPError
	if err := authorizePlayer(ctx, db, "p"); !errors.As(err, &denied) || denied.Code != 403 {
		t.Fatalf("warm player cache bypassed disqualification: %v", err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE competition SET finished_at = ?, updated_at = ? WHERE id = ?", 100, 100, "c"); err != nil {
		t.Fatal(err)
	}
	comp, err = retrieveCompetition(ctx, db, "c")
	if err != nil || !comp.FinishedAt.Valid || comp.FinishedAt.Int64 != 100 {
		t.Fatal("stale finish", comp, err)
	}
	if _, err := retrievePlayer(ctx, db, "new"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
	raw.MustExec("INSERT INTO player VALUES(7,'new','New',0,1,1)")
	if row, err := retrievePlayer(ctx, db, "new"); err != nil || row.DisplayName != "New" {
		t.Fatal(row, err)
	}
	// Simulate process restart: a fresh connection cache restores persisted state.
	cold := &tenantConnection{DB: raw, reads: make(map[string]*sqlx.Stmt)}
	if row, err := retrievePlayer(ctx, cold, "p"); err != nil || !row.IsDisqualified {
		t.Fatal(row, err)
	}
}
