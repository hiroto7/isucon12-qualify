package isuports

import (
	"context"
	"fmt"
)

// Returned slices are immutable; callers build separate API response slices.
func (db *tenantConnection) competitionList(ctx context.Context, dest *[]CompetitionRow, query string, args ...interface{}) error {
	key := query + fmt.Sprint(args...)
	db.competitionListMu.RLock()
	rows, ok := db.competitionLists[key]
	db.competitionListMu.RUnlock()
	if ok {
		*dest = rows
		return nil
	}
	db.competitionListMu.Lock()
	defer db.competitionListMu.Unlock()
	if rows, ok := db.competitionLists[key]; ok {
		*dest = rows
		return nil
	}
	stmt, err := db.prepare(ctx, query)
	if err != nil {
		return err
	}
	rows = make([]CompetitionRow, 0)
	if err := stmt.SelectContext(ctx, &rows, args...); err != nil {
		return err
	}
	if db.competitionLists == nil {
		db.competitionLists = make(map[string][]CompetitionRow)
	}
	db.competitionLists[key] = rows
	*dest = rows
	return nil
}
