package isuports

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

type blockedVisitDB struct {
	*sqlx.DB
	started, release chan struct{}
}

func (db blockedVisitDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	close(db.started)
	<-db.release
	return db.DB.ExecContext(ctx, query, args...)
}

func queuedVisitBatch(db dbOrTx, rows []VisitHistoryRow) []chan error {
	writer := &visitBatcher{db: db, input: make(chan visitWrite, len(rows))}
	replies := make([]chan error, len(rows))
	for i, row := range rows {
		replies[i] = make(chan error, 1)
		writer.input <- visitWrite{row: row, done: replies[i]}
	}
	close(writer.input)
	go writer.run()
	return replies
}

func TestVisitBatchAcknowledgesOnlyCommittedRows(t *testing.T) {
	db := sqlx.MustOpen("sqlite3", ":memory:")
	db.SetMaxOpenConns(1)
	defer db.Close()
	db.MustExec("CREATE TABLE visit_history(player_id TEXT,tenant_id INTEGER,competition_id TEXT,created_at INTEGER,updated_at INTEGER)")
	blocked := blockedVisitDB{DB: db, started: make(chan struct{}), release: make(chan struct{})}
	rows := []VisitHistoryRow{{PlayerID: "p", TenantID: 7, CompetitionID: "a", CreatedAt: 100}, {PlayerID: "q", TenantID: 8, CompetitionID: "b", CreatedAt: 200}}
	replies := queuedVisitBatch(blocked, rows)
	<-blocked.started
	for _, reply := range replies {
		select {
		case err := <-reply:
			t.Fatalf("acknowledged before durable execution: %v", err)
		default:
		}
	}
	close(blocked.release)
	for _, reply := range replies {
		if err := <-reply; err != nil {
			t.Fatal(err)
		}
	}
	var count int
	db.Get(&count, "SELECT COUNT(*) FROM visit_history")
	if count != 2 {
		t.Fatal(count)
	}
	db.MustExec("CREATE TRIGGER reject BEFORE INSERT ON visit_history WHEN NEW.player_id='q' BEGIN SELECT RAISE(ABORT,'failure'); END")
	for _, reply := range queuedVisitBatch(db, rows) {
		if err := <-reply; err == nil {
			t.Fatal("failed batch acknowledged success")
		}
	}
	db.Get(&count, "SELECT COUNT(*) FROM visit_history")
	if count != 2 {
		t.Fatal("partial failed batch persisted", count)
	}
}

func TestVisitBatchMySQLAtomicAndColdRead(t *testing.T) {
	if os.Getenv("ISUCON_ID_INTEGRATION") != "1" {
		t.Skip("isolated guest MySQL")
	}
	db, err := connectAdminDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tenantID := int64(9223372036854775000)
	comp := fmt.Sprintf("batch-%d", time.Now().UnixNano())
	defer db.Exec("DELETE FROM visit_history WHERE tenant_id=? AND competition_id=?", tenantID, comp)
	var rows []VisitHistoryRow
	for i := 0; i < 40; i++ {
		rows = append(rows, VisitHistoryRow{PlayerID: fmt.Sprintf("p%d", i), TenantID: tenantID, CompetitionID: comp, CreatedAt: 100, UpdatedAt: 100})
	}
	for _, reply := range queuedVisitBatch(db, rows) {
		if err := <-reply; err != nil {
			t.Fatal(err)
		}
	}
	var count int
	db.Get(&count, "SELECT COUNT(*) FROM visit_history WHERE tenant_id=? AND competition_id=?", tenantID, comp)
	if count != 40 {
		t.Fatal(count)
	}
	var earliest int64
	db.Get(&earliest, "SELECT MIN(created_at) FROM visit_history WHERE tenant_id=? AND competition_id=? AND player_id='p0'", tenantID, comp)
	if earliest != 100 {
		t.Fatal("committed first visit missing", earliest)
	}
	bad := rows[0]
	bad.PlayerID = strings.Repeat("x", 300)
	for _, reply := range queuedVisitBatch(db, []VisitHistoryRow{rows[0], bad}) {
		if err := <-reply; err == nil {
			t.Fatal("MySQL accepted failing batch")
		}
	}
	db.Get(&count, "SELECT COUNT(*) FROM visit_history WHERE tenant_id=? AND competition_id=?", tenantID, comp)
	if count != 40 {
		t.Fatal("partial MySQL batch persisted", count)
	}
}
