package isuports

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestPlayerScoreCacheReplacementEmptyAndRollback(t *testing.T) {
	db := sqlx.MustOpen("sqlite3", ":memory:")
	db.SetMaxOpenConns(1)
	defer db.Close()
	db.MustExec(`CREATE TABLE competition(id TEXT PRIMARY KEY,tenant_id INTEGER,title TEXT,finished_at INTEGER,created_at INTEGER,updated_at INTEGER);
 CREATE TABLE player_score(id TEXT PRIMARY KEY,tenant_id INTEGER,player_id TEXT,competition_id TEXT,score INTEGER,row_num INTEGER,created_at INTEGER,updated_at INTEGER);
 INSERT INTO competition VALUES('a',704,'A',NULL,1,1),('b',704,'B',NULL,2,2);`)
	ctx := context.Background()
	row := func(id, comp, player string, score int64) PlayerScoreRow {
		return PlayerScoreRow{ID: id, TenantID: 704, CompetitionID: comp, PlayerID: player, Score: score, RowNum: score}
	}
	write := func(comp string, rows ...PlayerScoreRow) {
		if err := replaceCompetitionScores(ctx, db, 704, comp, rows); err != nil {
			t.Fatal(err)
		}
	}
	read := func(player string) []PlayerScoreDetail {
		rows, err := playerScoreDetails(ctx, db, 704, player)
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	write("a", row("ap", "a", "p", 10), row("aq", "a", "q", 1))
	write("b", row("bp", "b", "p", 20))
	if rows := read("p"); len(rows) != 2 || rows[0].CompetitionTitle != "A" || rows[1].Score != 20 {
		t.Fatal(rows)
	}
	if len(read("q")) != 1 {
		t.Fatal("missing old scorer")
	}
	write("a", row("ap", "a", "p", 30))
	if rows := read("p"); len(rows) != 2 || rows[0].Score != 30 {
		t.Fatal("stale replacement", rows)
	}
	if len(read("q")) != 0 {
		t.Fatal("removed scorer cache retained")
	}
	// A conflicting ID owned by another competition must leave the cached old version.
	if err := replaceCompetitionScores(ctx, db, 704, "a", []PlayerScoreRow{row("bp", "a", "p", 99)}); err == nil {
		t.Fatal("expected rollback")
	}
	if rows := read("p"); len(rows) != 2 || rows[0].Score != 30 {
		t.Fatal("failed write changed cache", rows)
	}
	write("b")
	if rows := read("p"); len(rows) != 1 || rows[0].Score != 30 {
		t.Fatal("empty replacement stale", rows)
	}
	resetPlayerScoreSnapshots()
	if rows := read("p"); len(rows) != 1 || rows[0].Score != 30 {
		t.Fatal("cold restoration", rows)
	}
}
