package isuports

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// The competition lock covers the durable commit and derived cache publication.
func replaceCompetitionScores(ctx context.Context, db *sqlx.DB, tenantID int64, competitionID string, rows []PlayerScoreRow) error {
	lock := scoreLock(tenantID, competitionID)
	lock.Lock()
	defer lock.Unlock()
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM player_score WHERE tenant_id = ? AND competition_id = ?", tenantID, competitionID); err != nil {
		return err
	}
	stmt, err := tx.PrepareNamedContext(ctx, "INSERT INTO player_score (id, tenant_id, player_id, competition_id, score, row_num, created_at, updated_at) VALUES (:id, :tenant_id, :player_id, :competition_id, :score, :row_num, :created_at, :updated_at)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	rows = latestScoreRows(rows)
	for _, row := range rows {
		if _, err := stmt.ExecContext(ctx, row); err != nil {
			return fmt.Errorf("insert score row: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	publishLatestScores(tenantID, competitionID, rows)
	invalidateBilling(tenantID, competitionID)
	rankSnapshots.Delete(competitionCacheKey{tenantID, competitionID})
	return nil
}
