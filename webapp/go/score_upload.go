package isuports

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

// Every input row is parsed, including rows overwritten by a later score.
func readScoreUpload(r *csv.Reader, tenantID int64, competitionID string) ([]PlayerScoreRow, int64, error) {
	rows := make([]PlayerScoreRow, 0)
	var count int64
	for {
		fields, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("read score CSV: %w", err)
		}
		if len(fields) != 2 {
			return nil, 0, fmt.Errorf("row must have two columns: %#v", fields)
		}
		score, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return nil, 0, echo.NewHTTPError(http.StatusBadRequest, "invalid score")
		}
		count++
		rows = append(rows, PlayerScoreRow{TenantID: tenantID, CompetitionID: competitionID, PlayerID: fields[0], Score: score, RowNum: count})
	}
	return latestScoreRows(rows), count, nil
}

func latestScoreRows(rows []PlayerScoreRow) []PlayerScoreRow {
	latest := make(map[string]PlayerScoreRow, len(rows))
	for _, row := range rows {
		old, ok := latest[row.PlayerID]
		if !ok || old.RowNum < row.RowNum {
			latest[row.PlayerID] = row
		}
	}
	compact := make([]PlayerScoreRow, 0, len(latest))
	for _, row := range latest {
		compact = append(compact, row)
	}
	sort.Slice(compact, func(i, j int) bool { return compact[i].RowNum < compact[j].RowNum })
	return compact
}

// Bound placeholders below SQLite's conservative 999-variable limit.
func validateScorePlayers(ctx context.Context, db dbOrTx, rows []PlayerScoreRow) error {
	const batch = 900
	for start := 0; start < len(rows); start += batch {
		end := start + batch
		if end > len(rows) {
			end = len(rows)
		}
		ids := make([]string, 0, end-start)
		for _, row := range rows[start:end] {
			ids = append(ids, row.PlayerID)
		}
		query, args, err := sqlx.In("SELECT id FROM player WHERE id IN (?)", ids)
		if err != nil {
			return err
		}
		var found []string
		if err := db.SelectContext(ctx, &found, query, args...); err != nil {
			return err
		}
		known := make(map[string]bool, len(found))
		for _, id := range found {
			known[id] = true
		}
		for _, id := range ids {
			if !known[id] {
				return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("player not found: %s", id))
			}
		}
	}
	return nil
}
