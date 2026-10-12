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

## H4 — durable ID blocks

Trial 011 observed 36,233 ID reservations and cumulative 186.985 seconds (prepared SQL slow log) under increased workload. Reserve 4096 IDs per durable MySQL row update using LAST_INSERT_ID(expr), then assign locally under a mutex. All processes get disjoint DB-reserved blocks; restart skips unused IDs; initialize resets process allocator after database reset. Preserve hexadecimal ID alphabet. Tests cover parallel unique IDs, restart/unused ranges, failed reservation, and actual MySQL OK-packet/persisted watermark. Trial 012 preparation failed due to Docker missing parent snapshot, not code/benchmark validation.

H4 outcome: unit tests with race detector and actual guest MySQL reservation/persisted watermark passed. Trial 013: 28258, zero errors, 2.26x H3 and 4.46x initial median. Accepted.

## H5 — ranking snapshots

Trial 011 had 678,010 player metadata reads and ranking mean 1100 ms. Replace per-row metadata requests with one indexed last-row/group join, store immutable derived sorted ranks per competition and invalidate after atomic successful CSV replacement under the existing tenant lock. Keep authorization and current competition metadata outside the snapshot. Initialization clears snapshots; process restart reloads persisted rows. Tests cover last-row-lower-score, duplicate player, tie ordering, offset/negative/huge offset, empty replacement, invalidation and cold restore. Trial 014 pending.

H5 outcome: 014 failed because the benchmark guest soft FD limit was 1024 (104 ordinary errors; score invalid). Set only transient official benchmark process LimitNOFILE=65536. Trial 015 passed 40420, zero errors. Exact H4 source-manifest control 016 under the new benchmark FD setting passed 28828, zero errors (old H4 run 28258). Ranking candidate accepted: +40.2% against same-condition control. Initial baseline under the new benchmark limit will also be rechecked before final reporting.

## H6 — reuse finished billing with versioned invalidation

Trial 011 CPU profile attributed 45.4% cumulative samples to billingReportByCompetition; visit grouping remained 9697 calls and 26M examined rows. Skip all aggregation for unfinished (zero-priced) competitions. Cache finished reports; successful score replacement, every persisted ranking visit, and every completion invalidate its version. Computation cannot install a stale report across invalidation; repeated completion cutoff must match the cached cutoff. This intentionally invalidates even post-finish visits to handle completion races safely. Initialization clears caches; cold process recomputes from authoritative MySQL/SQLite. Tests cover concurrent invalidation, non-aliasing, cutoff change, inclusive finish boundary, distinct first visits and late eligible writes. Trial 017 pending.

H6 outcome: 017 failed due to proven nginx worker_connections=768 exhaustion (100 ordinary resets), not accepted. Set nginx worker_connections=8192 and worker_rlimit_nofile=16384 within the app guest. Trial 018 passed 68407, zero errors. Accept versioned finished billing + required nginx capacity together; no isolated cache-only score claim.

## H7 — atomic player batches

Trial 019: 8577 individual player INSERTs, 47.493 seconds cumulative execution. Replace them with one prepared transaction per request and form created-player responses from the committed input records. Race tests cover duplicate-key rollback, empty batch, blank and Unicode names. Trial 020 passed 83561, zero errors; +22.2% versus H6. Accepted.

## H8 — single-statement player details (not adopted)

Tests passed latest-row, creation order, empty array and tenant separation. Trial 021: official pass 85534 (raw 91972), 7 dial timeouts; nginx did not emit a new capacity alert and listen drops/overflows were zero. No grounded capacity classification for these errors. Overall gain is small; source reverted to H7 rather than spending retries on it. Candidate retained privately for later only if a new profile makes it a major bottleneck.

## H9 — first-visit persistence

Trial 019 slow log: 14133 visit INSERTs, 42.145 seconds cumulative. Billing only uses per-tenant/competition/player MIN(created_at); no API exposes each duplicate visit. Persist the first visit before returning, consult existing MIN using the covering index on cold cache, then skip redundant later writes. Still persist an earlier timestamp if the clock goes backwards. Reset process cache on initialize; after restart recover from DB. Invalidate billing only when a write actually changes history.

H9 outcome: tests passed concurrent single persistence, cold recovery, earlier-clock minimum, cross-tenant keys and failed-write retry. Trial 022 passed 104623 with zero errors, +25.2% versus H7. Accepted.
