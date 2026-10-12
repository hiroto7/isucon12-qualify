package isuports

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestReplaceScoresAtomic(t *testing.T) {
	db := sqlx.MustOpen("sqlite3", ":memory:")
	db.SetMaxOpenConns(1)
	defer db.Close()
	db.MustExec(`CREATE TABLE player_score (id TEXT PRIMARY KEY, tenant_id INTEGER, player_id TEXT, competition_id TEXT, score INTEGER, row_num INTEGER, created_at INTEGER, updated_at INTEGER)`)
	ctx := context.Background()
	row := func(id, comp, player string, score, pos int64) PlayerScoreRow {
		return PlayerScoreRow{ID: id, TenantID: 1, PlayerID: player, CompetitionID: comp, Score: score, RowNum: pos}
	}
	if err := replaceCompetitionScores(ctx, db, 1, "a", []PlayerScoreRow{row("old", "a", "p", 10, 1)}); err != nil {
		t.Fatal(err)
	}
	if err := replaceCompetitionScores(ctx, db, 1, "b", []PlayerScoreRow{row("other", "b", "p", 20, 1)}); err != nil {
		t.Fatal(err)
	}
	// A primary-key failure halfway through must roll back both delete and insert.
	if err := replaceCompetitionScores(ctx, db, 1, "a", []PlayerScoreRow{row("new", "a", "p", 30, 1), row("other", "a", "q", 40, 2)}); err == nil {
		t.Fatal("expected insertion failure")
	}
	var ids []string
	if err := db.Select(&ids, "SELECT id FROM player_score WHERE competition_id='a'"); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "old" {
		t.Fatalf("old version lost: %v", ids)
	}
	if err := replaceCompetitionScores(ctx, db, 1, "a", []PlayerScoreRow{row("n1", "a", "p", 50, 1), row("n2", "a", "p", 30, 2)}); err != nil {
		t.Fatal(err)
	}
	var score int64
	if err := db.Get(&score, "SELECT score FROM player_score WHERE competition_id='a' AND player_id='p' ORDER BY row_num DESC LIMIT 1"); err != nil || score != 30 {
		t.Fatalf("last row must win: %d %v", score, err)
	}
	var retained int
	db.Get(&retained, "SELECT count(*) FROM player_score WHERE competition_id='a'")
	if retained != 1 {
		t.Fatalf("overwritten rows retained: %d", retained)
	}
	if err := replaceCompetitionScores(ctx, db, 1, "a", nil); err != nil {
		t.Fatal(err)
	}
	var count int
	db.Get(&count, "SELECT count(*) FROM player_score WHERE competition_id='a'")
	if count != 0 {
		t.Fatalf("empty replacement retained rows: %d", count)
	}
	db.Get(&count, "SELECT count(*) FROM player_score WHERE competition_id='b'")
	if count != 1 {
		t.Fatal("other competition changed")
	}
}
