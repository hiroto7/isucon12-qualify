package isuports

import (
	"context"
	"fmt"
	"sync"
)

const idBlockSize int64 = 4096

type idBlockAllocator struct {
	mu        sync.Mutex
	next, end int64
}

var generatedIDs idBlockAllocator

func (a *idBlockAllocator) reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.next, a.end = 0, 0
}

func (a *idBlockAllocator) take(ctx context.Context, reserve func(context.Context, int64) (int64, error)) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.next == a.end {
		high, err := reserve(ctx, idBlockSize)
		if err != nil {
			return "", err
		}
		if high < idBlockSize {
			return "", fmt.Errorf("invalid ID reservation: %d", high)
		}
		a.next, a.end = high-idBlockSize+1, high+1
	}
	id := a.next
	a.next++
	return fmt.Sprintf("%x", id), nil
}

// LAST_INSERT_ID(expr) returns this connection's reserved high-water mark in
// the OK packet. The durable row lock makes blocks disjoint across processes.
func reserveIDBlock(ctx context.Context, size int64) (int64, error) {
	res, err := adminDB.ExecContext(ctx, "UPDATE id_generator SET id=LAST_INSERT_ID(id+?) WHERE stub=?", size, "a")
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n != 1 {
		return 0, fmt.Errorf("ID generator row missing")
	}
	return res.LastInsertId()
}
