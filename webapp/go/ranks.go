package isuports

import (
	"context"
	"sync"

	"github.com/jmoiron/sqlx"
)

type competitionCacheKey struct {
	TenantID      int64
	CompetitionID string
}

var rankSnapshots sync.Map

func resetRankSnapshots() {
	rankSnapshots.Range(func(key, _ interface{}) bool { rankSnapshots.Delete(key); return true })
}

// Cache entries are immutable. A restart reloads persisted rows.
func competitionRanks(ctx context.Context, db *sqlx.DB, tenantID int64, competitionID string) ([]CompetitionRank, error) {
	snapshot, err := competitionRankData(ctx, db, tenantID, competitionID)
	if err != nil {
		return nil, err
	}
	return snapshot.ranks, nil
}

func competitionRankData(ctx context.Context, db *sqlx.DB, tenantID int64, competitionID string) (*rankSnapshot, error) {
	lock := scoreLock(tenantID, competitionID)
	lock.RLock()
	defer lock.RUnlock()
	key := competitionCacheKey{tenantID, competitionID}
	if cached, ok := rankSnapshots.Load(key); ok {
		return cached.(*rankSnapshot), nil
	}
	ranks := make([]CompetitionRank, 0)
	err := db.SelectContext(ctx, &ranks, `SELECT ps.player_id, p.display_name AS player_display_name, ps.score, ps.row_num
FROM player_score ps
JOIN (SELECT player_id, MAX(row_num) AS row_num FROM player_score WHERE competition_id=? AND tenant_id=? GROUP BY player_id) latest
ON latest.player_id=ps.player_id AND latest.row_num=ps.row_num
JOIN player p ON p.id=ps.player_id
WHERE ps.competition_id=? AND ps.tenant_id=?
ORDER BY ps.score DESC, ps.row_num ASC`, competitionID, tenantID, competitionID, tenantID)
	if err != nil {
		return nil, err
	}
	for i := range ranks {
		ranks[i].Rank = int64(i + 1)
	}
	snapshot := &rankSnapshot{ranks: ranks}
	rankSnapshots.Store(key, snapshot)
	return snapshot, nil
}

func pageRanks(ranks []CompetitionRank, after int64) []CompetitionRank {
	page := make([]CompetitionRank, 0, 100)
	if after < 0 {
		after = 0
	}
	if after >= int64(len(ranks)) {
		return page
	}
	end := after + 100
	if end > int64(len(ranks)) {
		end = int64(len(ranks))
	}
	return append(page, ranks[after:end]...)
}
