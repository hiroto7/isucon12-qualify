package isuports

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestRankingJSONTracksScoreAndFinishedState(t *testing.T) {
	db := sqlx.MustOpen("sqlite3", ":memory:")
	db.SetMaxOpenConns(1)
	defer db.Close()
	db.MustExec(`CREATE TABLE player(id TEXT PRIMARY KEY,display_name TEXT);
 CREATE TABLE player_score(id TEXT PRIMARY KEY,tenant_id INTEGER,player_id TEXT,competition_id TEXT,score INTEGER,row_num INTEGER,created_at INTEGER,updated_at INTEGER);
 INSERT INTO player VALUES('p','<P>');`)
	ctx := context.Background()
	comp := &CompetitionRow{ID: "json", Title: "日本語"}
	write := func(score int64) {
		if err := replaceCompetitionScores(ctx, db, 705, "json", []PlayerScoreRow{{ID: "p", TenantID: 705, CompetitionID: "json", PlayerID: "p", Score: score, RowNum: 1}}); err != nil {
			t.Fatal(err)
		}
	}
	read := func() (int64, bool) {
		snapshot, err := competitionRankData(ctx, db, 705, "json")
		if err != nil {
			t.Fatal(err)
		}
		payload, err := snapshot.firstPage(comp)
		if err != nil {
			t.Fatal(err)
		}
		var response struct {
			Status bool                            `json:"status"`
			Data   CompetitionRankingHandlerResult `json:"data"`
		}
		if err := json.Unmarshal(payload, &response); err != nil {
			t.Fatal(err)
		}
		if !response.Status || response.Data.Competition.Title != "日本語" || response.Data.Ranks[0].PlayerDisplayName != "<P>" {
			t.Fatal(response)
		}
		return response.Data.Ranks[0].Score, response.Data.Competition.IsFinished
	}
	write(100)
	for i := 0; i < 2; i++ {
		if score, finished := read(); score != 100 || finished {
			t.Fatal(score, finished)
		}
	}
	comp.FinishedAt = sql.NullInt64{Int64: 100, Valid: true}
	if score, finished := read(); score != 100 || !finished {
		t.Fatal("stale finished flag", score, finished)
	}
	write(50)
	if score, finished := read(); score != 50 || !finished {
		t.Fatal("stale JSON after replacement", score, finished)
	}
}
