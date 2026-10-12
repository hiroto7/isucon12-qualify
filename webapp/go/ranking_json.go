package isuports

import (
	"encoding/json"
	"sync"
)

type rankSnapshot struct {
	ranks   []CompetitionRank
	mu      sync.Mutex
	encoded [2][]byte
}

func (snapshot *rankSnapshot) firstPage(comp *CompetitionRow) ([]byte, error) {
	index := 0
	if comp.FinishedAt.Valid {
		index = 1
	}
	snapshot.mu.Lock()
	defer snapshot.mu.Unlock()
	if snapshot.encoded[index] != nil {
		return snapshot.encoded[index], nil
	}
	payload, err := json.Marshal(SuccessResult{Status: true, Data: CompetitionRankingHandlerResult{
		Competition: CompetitionDetail{ID: comp.ID, Title: comp.Title, IsFinished: comp.FinishedAt.Valid},
		Ranks:       pageRanks(snapshot.ranks, 0),
	}})
	if err != nil {
		return nil, err
	}
	snapshot.encoded[index] = payload
	return payload, nil
}
