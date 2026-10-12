package isuports

import (
	"context"
	"github.com/jmoiron/sqlx"
)

func insertPlayers(ctx context.Context, db *sqlx.DB, rows []PlayerRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareNamedContext(ctx, `INSERT INTO player (id,tenant_id,display_name,is_disqualified,created_at,updated_at) VALUES (:id,:tenant_id,:display_name,:is_disqualified,:created_at,:updated_at)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, row := range rows {
		if _, err := stmt.ExecContext(ctx, row); err != nil {
			return err
		}
	}
	return tx.Commit()
}
