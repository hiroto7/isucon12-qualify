package isuports

import (
	"context"
	"github.com/jmoiron/sqlx"
	"testing"
)

func TestLatestScoreSnapshotCommitAndCold(t *testing.T) {
	resetLatestScores()
	defer resetLatestScores()
	db := sqlx.MustOpen("sqlite3", ":memory:")
	db.SetMaxOpenConns(1)
	defer db.Close()
	db.MustExec(`CREATE TABLE player_score(id TEXT PRIMARY KEY,tenant_id INTEGER,player_id TEXT,competition_id TEXT,score INTEGER,row_num INTEGER,created_at INTEGER,updated_at INTEGER)`)
	ctx := context.Background()
	rows := []PlayerScoreRow{{ID: "1", TenantID: 1, PlayerID: "p", CompetitionID: "a", Score: 100, RowNum: 1}, {ID: "2", TenantID: 1, PlayerID: "p", CompetitionID: "a", Score: 10, RowNum: 2}}
	if err := replaceCompetitionScores(ctx, db, 1, "a", rows); err != nil {
		t.Fatal(err)
	}
	values, err := competitionScores(ctx, db, 1, "a")
	if err != nil || values["p"] != 10 {
		t.Fatalf("latest row mismatch: %v %v", values, err)
	}
	resetLatestScores()
	values, err = competitionScores(ctx, db, 1, "a")
	if err != nil || values["p"] != 10 {
		t.Fatal("cold DB restoration failed")
	}
	if err := replaceCompetitionScores(ctx, db, 1, "a", []PlayerScoreRow{rows[0], rows[0]}); err == nil {
		t.Fatal("expected failed transaction")
	}
	values, err = competitionScores(ctx, db, 1, "a")
	if err != nil || values["p"] != 10 {
		t.Fatal("failed write published a snapshot")
	}
	if err := replaceCompetitionScores(ctx, db, 1, "a", nil); err != nil {
		t.Fatal(err)
	}
	values, err = competitionScores(ctx, db, 1, "a")
	if err != nil || len(values) != 0 {
		t.Fatal("empty replacement retained score")
	}
}
