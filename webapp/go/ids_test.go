package isuports

import (
	"context"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
)

func TestIDsConcurrentAndRestart(t *testing.T) {
	var alloc idBlockAllocator
	high := int64(4611686018427387904)
	reservations := 0
	reserve := func(_ context.Context, size int64) (int64, error) { high += size; reservations++; return high, nil }
	ids := make(chan string, 16384)
	var wg sync.WaitGroup
	for i := 0; i < 128; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 128; j++ {
				id, err := alloc.take(context.Background(), reserve)
				if err != nil {
					t.Error(err)
					return
				}
				ids <- id
			}
		}()
	}
	wg.Wait()
	close(ids)
	seen := map[string]bool{}
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate ID %s", id)
		}
		seen[id] = true
	}
	if len(seen) != 16384 || reservations != 4 {
		t.Fatalf("ids=%d reservations=%d", len(seen), reservations)
	}
	// A restarted process reserves beyond the persisted watermark, even when
	// the old process has unused IDs in its block.
	var restarted idBlockAllocator
	id, err := restarted.take(context.Background(), reserve)
	if err != nil {
		t.Fatal(err)
	}
	value, _ := strconv.ParseInt(id, 16, 64)
	if value <= 4611686018427387904+16384 {
		t.Fatalf("restart reused ID: %s", id)
	}
	alloc.reset()
	id, err = alloc.take(context.Background(), reserve)
	if err != nil || seen[id] {
		t.Fatalf("reset reused ID: %s %v", id, err)
	}
}

func TestIDsReservationFailure(t *testing.T) {
	var alloc idBlockAllocator
	_, err := alloc.take(context.Background(), func(context.Context, int64) (int64, error) { return 0, errors.New("disk error") })
	if err == nil || alloc.next != 0 || alloc.end != 0 {
		t.Fatal("failed reservation consumed IDs")
	}
}

func TestIDReservationMySQL(t *testing.T) {
	if os.Getenv("ISUCON_ID_INTEGRATION") != "1" {
		t.Skip("requires isolated app guest MySQL")
	}
	db, err := connectAdminDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	old := adminDB
	adminDB = db
	defer func() { adminDB = old }()
	a, err := reserveIDBlock(context.Background(), idBlockSize)
	if err != nil {
		t.Fatal(err)
	}
	b, err := reserveIDBlock(context.Background(), idBlockSize)
	if err != nil || b-a != idBlockSize {
		t.Fatalf("non-disjoint durable blocks: %d %d %v", a, b, err)
	}
	var persisted int64
	if err := db.Get(&persisted, "SELECT id FROM id_generator WHERE stub='a'"); err != nil || persisted != b {
		t.Fatalf("watermark not saved: %d %v", persisted, err)
	}
}
