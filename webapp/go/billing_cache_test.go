package isuports

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestBillingCacheConcurrentInvalidation(t *testing.T) {
	s := &billingSnapshot{}
	started, continueLoad := make(chan struct{}), make(chan struct{})
	go func() {
		<-started
		s.mu.Lock()
		s.version++
		s.report = nil
		s.mu.Unlock()
		close(continueLoad)
	}()
	calls := 0
	load := func() (*BillingReport, error) {
		calls++
		if calls == 1 {
			close(started)
			<-continueLoad
			return &BillingReport{BillingYen: 100}, nil
		}
		return &BillingReport{BillingYen: 200}, nil
	}
	r, err := s.get(100, load)
	if err != nil || r.BillingYen != 200 || calls != 2 {
		t.Fatalf("stale concurrent load: %+v calls=%d err=%v", r, calls, err)
	}
	r.BillingYen = 999 // Caller must not be able to corrupt the cached value.
	r, err = s.get(100, load)
	if err != nil || r.BillingYen != 200 || calls != 2 {
		t.Fatal("cache aliasing or missed hit")
	}
	if _, err = s.get(101, load); err != nil || calls != 3 {
		t.Fatal("finish cutoff change reused old report")
	}
}

func TestBillingFinishedBoundaryAndInvalidation(t *testing.T) {
	resetBillingSnapshots()
	defer resetBillingSnapshots()
	t.Setenv("ISUCON_TENANT_DB_DIR", t.TempDir())
	tenant := sqlx.MustOpen("sqlite3", ":memory:")
	tenant.SetMaxOpenConns(1)
	defer tenant.Close()
	admin := sqlx.MustOpen("sqlite3", ":memory:")
	admin.SetMaxOpenConns(1)
	defer admin.Close()
	old := adminDB
	adminDB = admin
	defer func() { adminDB = old }()
	tenant.MustExec(`CREATE TABLE competition(id TEXT,tenant_id INTEGER,title TEXT,finished_at INTEGER,created_at INTEGER,updated_at INTEGER);
CREATE TABLE player_score(id TEXT,tenant_id INTEGER,player_id TEXT,competition_id TEXT,score INTEGER,row_num INTEGER,created_at INTEGER,updated_at INTEGER);
INSERT INTO competition VALUES('a',1,'A',NULL,1,1);
INSERT INTO player_score VALUES('1',1,'p','a',10,1,1,1);`)
	admin.MustExec(`CREATE TABLE visit_history(player_id TEXT,tenant_id INTEGER,competition_id TEXT,created_at INTEGER,updated_at INTEGER);
INSERT INTO visit_history VALUES('p',1,'a',90,90),('q',1,'a',100,100),('q',1,'a',120,120),('r',1,'a',101,101);`)
	ctx := context.Background()
	r, err := billingReportByCompetition(ctx, tenant, 1, "a")
	if err != nil || r.BillingYen != 0 {
		t.Fatalf("unfinished billing: %+v %v", r, err)
	}
	tenant.MustExec("UPDATE competition SET finished_at=100")
	invalidateBilling(1, "a")
	r, err = billingReportByCompetition(ctx, tenant, 1, "a")
	if err != nil || r.PlayerCount != 1 || r.VisitorCount != 1 || r.BillingYen != 110 {
		t.Fatalf("finish boundary or distinct visitors: %+v %v", r, err)
	}
	// A request begun before finish can persist after the first billing read.
	admin.MustExec("INSERT INTO visit_history VALUES('r',1,'a',99,99)")
	invalidateBilling(1, "a")
	r, err = billingReportByCompetition(ctx, tenant, 1, "a")
	if err != nil || r.BillingYen != 120 {
		t.Fatalf("late eligible visit not reflected: %+v %v", r, err)
	}
	resetBillingSnapshots()
	r, err = billingReportByCompetition(ctx, tenant, 1, "a")
	if err != nil || r.BillingYen != 120 {
		t.Fatal("cold/restart billing mismatch")
	}
}
