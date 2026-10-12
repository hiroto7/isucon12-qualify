package isuports

import (
	"context"
	"database/sql"
	"os"
	"sync"

	"github.com/lestrrat-go/jwx/v2/jwk"
)

type metadataEntry struct {
	mu    sync.Mutex
	value interface{}
}

func (db *tenantConnection) metadata(query, id string) *metadataEntry {
	entry, _ := db.metadataRows.LoadOrStore(query+"\x00"+id, &metadataEntry{})
	return entry.(*metadataEntry)
}

func (db *tenantConnection) getMetadata(ctx context.Context, dest interface{}, query, id string) error {
	entry := db.metadata(query, id)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.value == nil {
		stmt, err := db.prepare(ctx, query)
		if err != nil {
			return err
		}
		switch dest.(type) {
		case *PlayerRow:
			var row PlayerRow
			if err := stmt.GetContext(ctx, &row, id); err != nil {
				return err
			}
			entry.value = row
		case *CompetitionRow:
			var row CompetitionRow
			if err := stmt.GetContext(ctx, &row, id); err != nil {
				return err
			}
			entry.value = row
		}
	}
	switch row := entry.value.(type) {
	case PlayerRow:
		*dest.(*PlayerRow) = row
	case CompetitionRow:
		*dest.(*CompetitionRow) = row
	}
	return nil
}

func (db *tenantConnection) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	var cachedQuery string
	switch query {
	case "UPDATE player SET is_disqualified = ?, updated_at = ? WHERE id = ?":
		cachedQuery = "SELECT * FROM player WHERE id = ?"
	case "UPDATE competition SET finished_at = ?, updated_at = ? WHERE id = ?":
		cachedQuery = "SELECT * FROM competition WHERE id = ?"
	}
	if cachedQuery == "" {
		return db.DB.ExecContext(ctx, query, args...)
	}
	entry := db.metadata(cachedQuery, args[len(args)-1].(string))
	entry.mu.Lock()
	defer entry.mu.Unlock()
	result, err := db.DB.ExecContext(ctx, query, args...)
	if err == nil {
		entry.value = nil
	}
	return result, err
}

var tenantsByName sync.Map
var jwtKeys sync.Map

func signingKey(filename string) (interface{}, error) {
	if key, ok := jwtKeys.Load(filename); ok {
		return key, nil
	}
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	key, _, err := jwk.DecodePEM(content)
	if err != nil {
		return nil, err
	}
	actual, _ := jwtKeys.LoadOrStore(filename, key)
	return actual, nil
}

func resetTenantMetadata() {
	tenantsByName.Range(func(key, _ interface{}) bool { tenantsByName.Delete(key); return true })
}
