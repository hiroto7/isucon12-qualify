package isuports

import (
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestUploadCompactionPreservesLastRowAndInputCount(t *testing.T) {
	rows, count, err := readScoreUpload(csv.NewReader(strings.NewReader("p,100\nq,50\np,50\n")), 7, "c")
	if err != nil || count != 3 || len(rows) != 2 {
		t.Fatalf("%v %d %v", rows, count, err)
	}
	if rows[0].PlayerID != "q" || rows[0].RowNum != 2 || rows[1].Score != 50 || rows[1].RowNum != 3 {
		t.Fatal(rows)
	}
	if _, _, err := readScoreUpload(csv.NewReader(strings.NewReader("p,broken\np,50\n")), 7, "c"); err == nil {
		t.Fatal("overwritten invalid score accepted")
	}
	rows, count, err = readScoreUpload(csv.NewReader(strings.NewReader("")), 7, "c")
	if err != nil || count != 0 || len(rows) != 0 {
		t.Fatal("empty upload changed")
	}
}

func TestUploadPlayerValidationAcrossBatches(t *testing.T) {
	db := sqlx.MustOpen("sqlite3", ":memory:")
	db.SetMaxOpenConns(1)
	defer db.Close()
	db.MustExec("CREATE TABLE player (id TEXT PRIMARY KEY)")
	rows := make([]PlayerScoreRow, 1001)
	for i := range rows {
		rows[i].PlayerID = fmt.Sprintf("p%d", i)
		db.MustExec("INSERT INTO player VALUES (?)", rows[i].PlayerID)
	}
	if err := validateScorePlayers(context.Background(), db, rows); err != nil {
		t.Fatal(err)
	}
	rows[999].PlayerID = "missing"
	if err := validateScorePlayers(context.Background(), db, rows); err == nil {
		t.Fatal("missing player in second batch accepted")
	}
	if err := validateScorePlayers(context.Background(), db, nil); err != nil {
		t.Fatal(err)
	}
}
