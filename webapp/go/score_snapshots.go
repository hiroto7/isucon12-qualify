package isuports

import (
	"context"
	"sync"
)

var latestScores sync.Map

func resetLatestScores() {
	latestScores.Range(func(key, _ interface{}) bool { latestScores.Delete(key); return true })
}

// Capture an immutable score version under the competition read lock.
func competitionScores(ctx context.Context, db dbOrTx, tenantID int64, competitionID string) (map[string]int64, error) {
	lock := scoreLock(tenantID, competitionID)
	lock.RLock()
	defer lock.RUnlock()
	key := competitionCacheKey{tenantID, competitionID}
	if cached, ok := latestScores.Load(key); ok {
		return cached.(map[string]int64), nil
	}
	var rows []struct {
		PlayerID string `db:"player_id"`
		Score    int64  `db:"score"`
	}
	err := db.SelectContext(ctx, &rows, `SELECT ps.player_id,ps.score FROM player_score ps
JOIN (SELECT player_id,MAX(row_num) AS row_num FROM player_score WHERE tenant_id=? AND competition_id=? GROUP BY player_id) latest
ON latest.player_id=ps.player_id AND latest.row_num=ps.row_num
WHERE ps.tenant_id=? AND ps.competition_id=?`, tenantID, competitionID, tenantID, competitionID)
	if err != nil {
		return nil, err
	}
	values := make(map[string]int64, len(rows))
	for _, row := range rows {
		values[row.PlayerID] = row.Score
	}
	latestScores.Store(key, values)
	return values, nil
}

func publishLatestScores(tenantID int64, competitionID string, rows []PlayerScoreRow) {
	values := make(map[string]int64, len(rows))
	positions := make(map[string]int64, len(rows))
	for _, row := range rows {
		if position, ok := positions[row.PlayerID]; !ok || row.RowNum > position {
			positions[row.PlayerID] = row.RowNum
			values[row.PlayerID] = row.Score
		}
	}
	latestScores.Store(competitionCacheKey{tenantID, competitionID}, values)
}
