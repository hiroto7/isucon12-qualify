package isuports

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestConcurrentScoreCommitAndColdSnapshots(t *testing.T) {
	db := sqlx.MustOpen("sqlite3", ":memory:")
	db.SetMaxOpenConns(1)
	defer db.Close()
	db.MustExec(`CREATE TABLE player(id TEXT PRIMARY KEY,display_name TEXT);
 CREATE TABLE player_score(id TEXT PRIMARY KEY,tenant_id INTEGER,player_id TEXT,competition_id TEXT,score INTEGER,row_num INTEGER,created_at INTEGER,updated_at INTEGER);
 INSERT INTO player VALUES('p','P'),('q','Q');`)
	ctx := context.Background()
	upload := func(value int64) error {
		return replaceCompetitionScores(ctx, db, 701, "concurrent", []PlayerScoreRow{
			{ID: "p", TenantID: 701, CompetitionID: "concurrent", PlayerID: "p", Score: value, RowNum: 1},
			{ID: "q", TenantID: 701, CompetitionID: "concurrent", PlayerID: "q", Score: value, RowNum: 2},
		})
	}
	if err := upload(0); err != nil {
		t.Fatal(err)
	}
	rankSnapshots.Delete(competitionCacheKey{701, "concurrent"})
	latestScores.Delete(competitionCacheKey{701, "concurrent"})
	failures := make(chan error, 4)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for value := int64(1); value <= 40; value++ {
			if err := upload(value); err != nil {
				failures <- err
				return
			}
		}
	}()
	for worker := 0; worker < 3; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				ranks, err := competitionRanks(ctx, db, 701, "concurrent")
				if err != nil {
					failures <- err
					return
				}
				if len(ranks) != 2 || ranks[0].Score != ranks[1].Score || ranks[0].PlayerID != "p" {
					failures <- fmt.Errorf("torn ranks: %v", ranks)
					return
				}
				values, err := competitionScores(ctx, db, 701, "concurrent")
				if err != nil {
					failures <- err
					return
				}
				if len(values) != 2 || values["p"] != values["q"] {
					failures <- fmt.Errorf("torn scores: %v", values)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	ranks, err := competitionRanks(ctx, db, 701, "concurrent")
	if err != nil || len(ranks) != 2 || ranks[0].Score != 40 {
		t.Fatalf("old cache survived committed upload: %v %v", ranks, err)
	}
}
