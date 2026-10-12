package isuports

import (
	"context"
	"fmt"
	"github.com/jmoiron/sqlx"
	"strings"
	"sync"
)

type tenantConnection struct {
	*sqlx.DB
	mu                sync.Mutex
	reads             map[string]*sqlx.Stmt
	metadataRows      sync.Map
	competitionListMu sync.RWMutex
	competitionLists  map[string][]CompetitionRow
}

var tenantConnections = struct {
	sync.Mutex
	dbs map[int64]*tenantConnection
}{dbs: make(map[int64]*tenantConnection)}

// Handler Close releases no shared resource; initialize/process exit own it.
func (db *tenantConnection) Close() error { return nil }

func (db *tenantConnection) prepare(ctx context.Context, query string) (*sqlx.Stmt, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if stmt, ok := db.reads[query]; ok {
		return stmt, nil
	}
	stmt, err := db.DB.PreparexContext(ctx, query)
	if err != nil {
		return nil, err
	}
	db.reads[query] = stmt
	return stmt, nil
}

func (db *tenantConnection) GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	if query == "SELECT * FROM player WHERE id = ?" || query == "SELECT * FROM competition WHERE id = ?" {
		return db.getMetadata(ctx, dest, query, args[0].(string))
	}
	stmt, err := db.prepare(ctx, query)
	if err != nil {
		return err
	}
	return stmt.GetContext(ctx, dest, args...)
}
func (db *tenantConnection) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	if strings.HasPrefix(query, "SELECT * FROM competition WHERE tenant_id") {
		if rows, ok := dest.(*[]CompetitionRow); ok {
			return db.competitionList(ctx, rows, query, args...)
		}
	}
	stmt, err := db.prepare(ctx, query)
	if err != nil {
		return err
	}
	return stmt.SelectContext(ctx, dest, args...)
}

func pooledTenantDB(id int64) (*tenantConnection, error) {
	tenantConnections.Lock()
	defer tenantConnections.Unlock()
	if db, ok := tenantConnections.dbs[id]; ok {
		return db, nil
	}
	db, err := sqlx.Open(sqliteDriverName, fmt.Sprintf("file:%s?mode=rw", tenantDBPath(id)))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	conn := &tenantConnection{DB: db, reads: make(map[string]*sqlx.Stmt)}
	tenantConnections.dbs[id] = conn
	return conn, nil
}

func resetTenantConnections() {
	tenantConnections.Lock()
	old := tenantConnections.dbs
	tenantConnections.dbs = make(map[int64]*tenantConnection)
	tenantConnections.Unlock()
	for _, db := range old {
		db.mu.Lock()
		for _, stmt := range db.reads {
			stmt.Close()
		}
		db.mu.Unlock()
		db.DB.Close()
	}
}
