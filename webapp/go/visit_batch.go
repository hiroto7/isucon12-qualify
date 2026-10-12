package isuports

import (
	"context"
	"github.com/jmoiron/sqlx"
	"strings"
	"sync"
	"time"
)

type visitWrite struct {
	row  VisitHistoryRow
	done chan error
}
type visitBatcher struct {
	db    dbOrTx
	input chan visitWrite
}

var durableVisitBatches = struct {
	sync.Mutex
	writers map[*sqlx.DB]*visitBatcher
}{writers: make(map[*sqlx.DB]*visitBatcher)}

func writeDurableVisit(ctx context.Context, db dbOrTx, row VisitHistoryRow) error {
	if sqlDB, ok := db.(*sqlx.DB); ok && sqlDB == adminDB && sqlDB.DriverName() == "mysql" {
		durableVisitBatches.Lock()
		writer := durableVisitBatches.writers[sqlDB]
		if writer == nil {
			writer = newVisitBatcher(sqlDB)
			durableVisitBatches.writers[sqlDB] = writer
		}
		durableVisitBatches.Unlock()
		return writer.write(row)
	}
	_, err := db.ExecContext(ctx, "INSERT INTO visit_history (player_id,tenant_id,competition_id,created_at,updated_at) VALUES (?,?,?,?,?)", row.PlayerID, row.TenantID, row.CompetitionID, row.CreatedAt, row.UpdatedAt)
	return err
}

func newVisitBatcher(db dbOrTx) *visitBatcher {
	writer := &visitBatcher{db: db, input: make(chan visitWrite, 1024)}
	go writer.run()
	return writer
}

func (writer *visitBatcher) write(row VisitHistoryRow) error {
	request := visitWrite{row: row, done: make(chan error, 1)}
	writer.input <- request
	return <-request.done
}

func (writer *visitBatcher) run() {
	for first := range writer.input {
		batch := []visitWrite{first}
		timer := time.NewTimer(time.Millisecond)
		closed := false
	collect:
		for len(batch) < 128 {
			select {
			case next, ok := <-writer.input:
				if !ok {
					closed = true
					break collect
				}
				batch = append(batch, next)
			case <-timer.C:
				break collect
			}
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		placeholders := make([]string, len(batch))
		args := make([]interface{}, 0, len(batch)*5)
		for i, request := range batch {
			row := request.row
			placeholders[i] = "(?,?,?,?,?)"
			args = append(args, row.PlayerID, row.TenantID, row.CompetitionID, row.CreatedAt, row.UpdatedAt)
		}
		_, err := writer.db.ExecContext(context.Background(), "INSERT INTO visit_history (player_id,tenant_id,competition_id,created_at,updated_at) VALUES "+strings.Join(placeholders, ","), args...)
		// MySQL autocommit has completed durably before any HTTP caller is released.
		// A failed batch acknowledges failure to every caller; none publishes known=true.
		for _, request := range batch {
			request.done <- err
		}
		if closed {
			return
		}
	}
}
