package isuports

import (
	"context"
	"sync"
)

type tenantScoreState struct {
	sync.RWMutex
	players sync.Map
}

var tenantScores sync.Map

func tenantScoreSnapshots(tenantID int64) *tenantScoreState {
	state, _ := tenantScores.LoadOrStore(tenantID, &tenantScoreState{})
	return state.(*tenantScoreState)
}

func resetPlayerScoreSnapshots() {
	tenantScores.Range(func(key, _ interface{}) bool { tenantScores.Delete(key); return true })
}

// Score writers invalidate only old/new scorers after durable commit. The
// tenant read lock prevents a cold reader installing a superseded snapshot.
func playerScoreDetails(ctx context.Context, db dbOrTx, tenantID int64, playerID string) ([]PlayerScoreDetail, error) {
	state := tenantScoreSnapshots(tenantID)
	state.RLock()
	defer state.RUnlock()
	if cached, ok := state.players.Load(playerID); ok {
		return cached.([]PlayerScoreDetail), nil
	}
	var competitions []CompetitionRow
	if err := db.SelectContext(ctx, &competitions, "SELECT * FROM competition WHERE tenant_id = ? ORDER BY created_at ASC", tenantID); err != nil {
		return nil, err
	}
	details := make([]PlayerScoreDetail, 0)
	for _, comp := range competitions {
		scores, err := competitionScores(ctx, db, tenantID, comp.ID)
		if err != nil {
			return nil, err
		}
		if score, ok := scores[playerID]; ok {
			details = append(details, PlayerScoreDetail{CompetitionTitle: comp.Title, Score: score})
		}
	}
	state.players.Store(playerID, details)
	return details, nil
}
