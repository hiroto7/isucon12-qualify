# Hypotheses

Pending initial measurement. Static inspection suggests:

1. CSV performs a MySQL ID round-trip and a separate SQLite autocommit for every row, under a tenant-wide file lock. Hypothesis: a single atomic replacement transaction can remove row-count-scaled fsync and reader blocking.
2. Ranking scans all score rows, deduplicates, fetches each player's metadata separately and sorts under the same tenant lock on every read. Hypothesis: derive reusable ranking data per successful upload, preserving last-row and tie order.
3. Player detail performs one score lookup per historical competition, and billing repeatedly scans history. Hypothesis: indexed or precomputed current values reduce historical-data-scaled work.

Not accepted findings: priority and expected score effect require load measurements. Further inspection must include all call sites/branches, not just shared helpers.

## H1 — atomic CSV replacement (trial 006)

- Evidence: trial 005 counted 16,140 score INSERTs, 34.632 seconds cumulative exec time; ranking mean 699 ms, upload mean 1033 ms. CPU profile attributed 35% cumulative samples to player detail and 25% to ranking; samples include cgo SQLite row stepping.
- Change: one SQLite transaction and one prepared insert for complete DELETE+INSERT replacement. Existing tenant lock and last-CSV-row semantics retained. No MySQL ID allocation change.
- Validation: rollback on insertion failure, duplicate-player last row, empty replacement and other-competition isolation tests pass in the app container; official scoring trials 006/007 passed with 8165/7896 and zero errors. Accepted; 24.7–28.9% above baseline.
- Rollback: restore isuports.go and remove scores.go/scores_test.go, redeploy; no schema migration.

## Next discriminator — billing amplification

- The initial heavy tenant contains 1,674,797 score rows and 5,001 players; player-score lookups use full scans without indexes.
- Admin billing mean 6.0 seconds per request, repeatedly groups MySQL visit_history per competition. Existing index covers tenant_id only. Inspect slow query rows/time to determine whether a competition/player covering index unlocks more admin cycles and additional organizer/reader workers.
- Trial 005 digest summary omits prepared statements, so it is not complete DB analysis. Use slow query data to fill this gap; do not treat summary collection as completion.

## H2 — covering visit-history index

Trial 005 slow log confirms 2465 visit grouping calls, 102.182 seconds cumulative and 43,915,092 rows examined. Add (tenant_id, competition_id, player_id, created_at) to partition each group by competition and cover player/MIN(created_at). Keep existing semantics and old index. Schema creation and idempotent initialize migration agree. Build migration once outside the measured benchmark; subsequent initialize keeps it.

H2 outcome: trial 008 official pass, 11178 (raw 11290, one normal heavy-tenant billing timeout). Provisional adoption as a major workload-unlocking change; not a zero-error final state. Follow-on fix targets proven full scans of the 1.67M-row SQLite table.

## H3 — partition SQLite score lookup and match FD capacity

Index (competition_id,player_id,row_num DESC) removes the observed huge full scans. Restoration templates receive the same indexes once and fresh tenant schema agrees; rows/count/sums unchanged. Trial 009 enabled 262 reader workers but hit actual process FD limit 1024: key reads/SQLite opens failed with too many open files. Official pass 1450, 78 errors (1 critical), rejected. Trial 010 retains the index and sets the app container FD limit to 16384; required capacity repair is recorded as part of H3, not attributed solely to the index.

H3 outcome: trial 010 passed 12526 with zero errors. Accepted index + FD capacity; the heavy billing timeout and open-file failures are absent. Index-only trial 009 remains rejected. Next focus is structural ID round-trip elimination and repeated ranking/detail reconstruction.
