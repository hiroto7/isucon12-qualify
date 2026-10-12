package isuports

import (
	"context"
	"database/sql"
	"sync"
)

type visitCacheKey struct {
	TenantID                int64
	CompetitionID, PlayerID string
}
type firstVisit struct {
	mu    sync.Mutex
	known bool
	at    int64
}

var firstVisits sync.Map

func resetFirstVisits() {
	firstVisits.Range(func(key, _ interface{}) bool { firstVisits.Delete(key); return true })
}

// The persisted minimum is the billing input. Cache only successful durable
// observations, not pending writes, and keep old timestamps if the clock moves.
func recordFirstVisit(ctx context.Context, db dbOrTx, tenantID int64, competitionID, playerID string, now int64) (bool, error) {
	key := visitCacheKey{tenantID, competitionID, playerID}
	entry, _ := firstVisits.LoadOrStore(key, &firstVisit{})
	state := entry.(*firstVisit)
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.known {
		var earliest sql.NullInt64
		if err := db.GetContext(ctx, &earliest, "SELECT MIN(created_at) FROM visit_history WHERE tenant_id=? AND competition_id=? AND player_id=?", tenantID, competitionID, playerID); err != nil {
			return false, err
		}
		if earliest.Valid {
			state.known = true
			state.at = earliest.Int64
		}
	}
	if state.known && state.at <= now {
		return false, nil
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO visit_history (player_id,tenant_id,competition_id,created_at,updated_at) VALUES (?,?,?,?,?)", playerID, tenantID, competitionID, now, now); err != nil {
		return false, err
	}
	state.known = true
	state.at = now
	return true, nil
}
