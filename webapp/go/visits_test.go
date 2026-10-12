package isuports

import (
	"context"
	"github.com/jmoiron/sqlx"
	"sync"
	"testing"
)

func TestFirstVisitConcurrentDurableAndCold(t *testing.T) {
	resetFirstVisits()
	defer resetFirstVisits()
	db := sqlx.MustOpen("sqlite3", ":memory:")
	db.SetMaxOpenConns(1)
	defer db.Close()
	db.MustExec(`CREATE TABLE visit_history(player_id TEXT,tenant_id INTEGER,competition_id TEXT,created_at INTEGER,updated_at INTEGER)`)
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := recordFirstVisit(ctx, db, 1, "a", "p", 100); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	var n int
	db.Get(&n, "SELECT COUNT(*) FROM visit_history")
	if n != 1 {
		t.Fatalf("duplicate concurrent first visits: %d", n)
	}
	resetFirstVisits()
	changed, err := recordFirstVisit(ctx, db, 1, "a", "p", 101)
	if err != nil || changed {
		t.Fatal("restart duplicated persisted minimum")
	}
	changed, err = recordFirstVisit(ctx, db, 1, "a", "p", 99)
	if err != nil || !changed {
		t.Fatal("earlier clock timestamp lost")
	}
	var earliest int64
	db.Get(&earliest, "SELECT MIN(created_at) FROM visit_history")
	if earliest != 99 {
		t.Fatal("wrong persisted minimum")
	}
	if _, err = recordFirstVisit(ctx, db, 2, "a", "p", 100); err != nil {
		t.Fatal(err)
	}
	db.Get(&n, "SELECT COUNT(*) FROM visit_history")
	if n != 3 {
		t.Fatal("tenant isolation failed")
	}
}

func TestFirstVisitFailedWriteNotCached(t *testing.T) {
	resetFirstVisits()
	defer resetFirstVisits()
	db := sqlx.MustOpen("sqlite3", ":memory:")
	db.SetMaxOpenConns(1)
	defer db.Close()
	db.MustExec(`CREATE TABLE visit_history(player_id TEXT,tenant_id INTEGER,competition_id TEXT,created_at INTEGER,updated_at INTEGER);
CREATE TRIGGER reject_visit BEFORE INSERT ON visit_history BEGIN SELECT RAISE(ABORT,'failure'); END;`)
	if _, err := recordFirstVisit(context.Background(), db, 1, "a", "p", 100); err == nil {
		t.Fatal("expected durable write failure")
	}
	db.MustExec("DROP TRIGGER reject_visit")
	changed, err := recordFirstVisit(context.Background(), db, 1, "a", "p", 100)
	if err != nil || !changed {
		t.Fatal("failed write was marked persisted")
	}
}
