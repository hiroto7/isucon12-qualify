package isuports

import (
	"context"
	"github.com/jmoiron/sqlx"
	"os"
	"sync"
	"testing"
)

func TestTenantPoolLifetimeAndReplacement(t *testing.T) {
	resetTenantConnections()
	defer resetTenantConnections()
	t.Setenv("ISUCON_TENANT_DB_DIR", t.TempDir())
	create := func(value string) {
		db := sqlx.MustOpen("sqlite3", tenantDBPath(1))
		defer db.Close()
		db.MustExec("CREATE TABLE test_value(value TEXT)")
		db.MustExec("INSERT INTO test_value VALUES (?)", value)
	}
	create("before")
	db, err := connectToTenantDB(1)
	if err != nil {
		t.Fatal(err)
	}
	again, err := connectToTenantDB(1)
	if err != nil || again != db {
		t.Fatal("pool not reused")
	}
	db.Close()
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var value string
			if err := db.GetContext(context.Background(), &value, "SELECT value FROM test_value"); err != nil || value != "before" {
				t.Errorf("shared read: %s %v", value, err)
			}
		}()
	}
	wg.Wait()
	if len(db.reads) != 1 {
		t.Fatal("read statement not reused")
	}
	resetTenantConnections()
	if err := os.Remove(tenantDBPath(1)); err != nil {
		t.Fatal(err)
	}
	create("after")
	fresh, err := connectToTenantDB(1)
	if err != nil || fresh == db {
		t.Fatal("old pool retained after reset")
	}
	var value string
	if err := fresh.GetContext(context.Background(), &value, "SELECT value FROM test_value"); err != nil || value != "after" {
		t.Fatalf("stale restored DB: %s %v", value, err)
	}
}
