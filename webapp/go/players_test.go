package isuports

import (
	"context"
	"github.com/jmoiron/sqlx"
	"testing"
)

func TestPlayerBatchAtomic(t *testing.T) {
	db := sqlx.MustOpen("sqlite3", ":memory:")
	db.SetMaxOpenConns(1)
	defer db.Close()
	db.MustExec(`CREATE TABLE player(id TEXT PRIMARY KEY,tenant_id INTEGER,display_name TEXT,is_disqualified BOOLEAN,created_at INTEGER,updated_at INTEGER)`)
	ctx := context.Background()
	rows := []PlayerRow{{ID: "1", TenantID: 1, DisplayName: "", CreatedAt: 10, UpdatedAt: 10}, {ID: "2", TenantID: 1, DisplayName: "日本語🙂", CreatedAt: 10, UpdatedAt: 10}}
	if err := insertPlayers(ctx, db, rows); err != nil {
		t.Fatal(err)
	}
	p, err := retrievePlayer(ctx, db, "2")
	if err != nil || p.DisplayName != "日本語🙂" || p.IsDisqualified {
		t.Fatalf("stored player differs: %+v %v", p, err)
	}
	if err := insertPlayers(ctx, db, []PlayerRow{{ID: "3", TenantID: 1}, {ID: "1", TenantID: 1}}); err == nil {
		t.Fatal("expected duplicate failure")
	}
	var n int
	db.Get(&n, "SELECT COUNT(*) FROM player")
	if n != 2 {
		t.Fatalf("failed batch partially inserted: %d", n)
	}
	if err := insertPlayers(ctx, db, nil); err != nil {
		t.Fatal(err)
	}
}
