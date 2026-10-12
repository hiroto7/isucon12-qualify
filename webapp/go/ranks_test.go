package isuports

import (
	"context"
	"math"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestRankSnapshotLatestTieAndInvalidation(t *testing.T) {
	resetRankSnapshots()
	defer resetRankSnapshots()
	db := sqlx.MustOpen("sqlite3", ":memory:")
	db.SetMaxOpenConns(1)
	defer db.Close()
	db.MustExec(`CREATE TABLE player (id TEXT PRIMARY KEY, display_name TEXT);
CREATE TABLE player_score (id TEXT PRIMARY KEY,tenant_id INTEGER,player_id TEXT,competition_id TEXT,score INTEGER,row_num INTEGER,created_at INTEGER,updated_at INTEGER);
INSERT INTO player VALUES ('p','P'),('q','Q'),('r','R');`)
	ctx := context.Background()
	rows := []PlayerScoreRow{
		{ID: "1", TenantID: 1, PlayerID: "p", CompetitionID: "a", Score: 100, RowNum: 1},
		{ID: "2", TenantID: 1, PlayerID: "q", CompetitionID: "a", Score: 10, RowNum: 2},
		{ID: "3", TenantID: 1, PlayerID: "r", CompetitionID: "a", Score: 20, RowNum: 3},
		{ID: "4", TenantID: 1, PlayerID: "q", CompetitionID: "a", Score: 10, RowNum: 4},
		{ID: "5", TenantID: 1, PlayerID: "p", CompetitionID: "a", Score: 10, RowNum: 5},
	}
	if err := replaceCompetitionScores(ctx, db, 1, "a", rows); err != nil {
		t.Fatal(err)
	}
	ranks, err := competitionRanks(ctx, db, 1, "a")
	if err != nil {
		t.Fatal(err)
	}
	if len(ranks) != 3 || ranks[0].PlayerID != "r" || ranks[1].PlayerID != "q" || ranks[2].PlayerID != "p" || ranks[2].Score != 10 {
		t.Fatalf("wrong latest/tie order: %+v", ranks)
	}
	for i, r := range ranks {
		if r.Rank != int64(i+1) {
			t.Fatal("wrong global rank")
		}
	}
	if len(pageRanks(ranks, 1)) != 2 || pageRanks(ranks, 1)[0].Rank != 2 || len(pageRanks(ranks, math.MaxInt64)) != 0 || len(pageRanks(ranks, -1)) != 3 {
		t.Fatal("paging mismatch")
	}
	rows = rows[:1]
	rows[0].Score = 500
	if err := replaceCompetitionScores(ctx, db, 1, "a", rows); err != nil {
		t.Fatal(err)
	}
	ranks, err = competitionRanks(ctx, db, 1, "a")
	if err != nil || len(ranks) != 1 || ranks[0].Score != 500 {
		t.Fatalf("stale cache: %+v %v", ranks, err)
	}
	resetRankSnapshots()
	ranks, err = competitionRanks(ctx, db, 1, "a")
	if err != nil || len(ranks) != 1 || ranks[0].Score != 500 {
		t.Fatal("cold/restart restoration failed")
	}
	if err := replaceCompetitionScores(ctx, db, 1, "a", nil); err != nil {
		t.Fatal(err)
	}
	ranks, err = competitionRanks(ctx, db, 1, "a")
	if err != nil || len(ranks) != 0 {
		t.Fatal("empty replacement retained ranking")
	}
}
