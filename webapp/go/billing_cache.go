package isuports

import "sync"

type billingSnapshot struct {
	mu         sync.Mutex
	version    uint64
	finishedAt int64
	report     *BillingReport
}

var billingSnapshots sync.Map

func billingState(key competitionCacheKey) *billingSnapshot {
	state, _ := billingSnapshots.LoadOrStore(key, &billingSnapshot{})
	return state.(*billingSnapshot)
}

func invalidateBilling(tenantID int64, competitionID string) {
	s := billingState(competitionCacheKey{tenantID, competitionID})
	s.mu.Lock()
	s.version++
	s.report = nil
	s.mu.Unlock()
}

func resetBillingSnapshots() {
	billingSnapshots.Range(func(key, _ interface{}) bool { billingSnapshots.Delete(key); return true })
}

func (s *billingSnapshot) get(finishedAt int64, load func() (*BillingReport, error)) (*BillingReport, error) {
	for attempt := 0; ; attempt++ {
		s.mu.Lock()
		version := s.version
		if s.report != nil && s.finishedAt == finishedAt {
			copy := *s.report
			s.mu.Unlock()
			return &copy, nil
		}
		s.mu.Unlock()
		report, err := load()
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		if s.version == version {
			copy := *report
			s.report = &copy
			s.finishedAt = finishedAt
			s.mu.Unlock()
			return report, nil
		}
		s.mu.Unlock()
		// Under continuous concurrent invalidation, return the latest computed
		// DB snapshot without caching it rather than making progress depend on
		// the writer becoming idle.
		if attempt >= 2 {
			return report, nil
		}
	}
}
