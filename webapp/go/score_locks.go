package isuports

import "sync"

// Single application process: serialize each competition's commit and cache
// publication. Immutable readers release the lock before response encoding.
var competitionLocks sync.Map

func scoreLock(tenantID int64, competitionID string) *sync.RWMutex {
	lock, _ := competitionLocks.LoadOrStore(competitionCacheKey{tenantID, competitionID}, &sync.RWMutex{})
	return lock.(*sync.RWMutex)
}

func resetScoreLocks() {
	competitionLocks.Range(func(key, _ interface{}) bool { competitionLocks.Delete(key); return true })
}
