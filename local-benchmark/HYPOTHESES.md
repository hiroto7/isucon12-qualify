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

## H10 — shared SQLite connection/read statement lifetime

Saved profile 019 focus on sqlite3_prepare/SQLiteDriver.Open/database/sql.Prepare accounts for 17.96% of late Go CPU samples. Use four connections per tenant and retain prepared read statements. Handler Close is a no-op; initialize closes statements/DB handles before restoration. Data values/authorization remain live, not cached. Tests passed shared concurrent read, statement reuse and file replacement/cold reopen. Trial 023 passed 120445, zero errors, +15.1% over H9; adopted as resource-work reduction, without repeated small-gain tuning.

## H11 — latest-score snapshots for player detail

Eliminate the 352790 per-competition score queries and subsequent title refetches observed in 019. Publish immutable per-competition player->last-score maps only after successful atomic commit, using maximum CSV row number; cold snapshots rebuild from authoritative SQLite under the existing tenant lock. Use already listed competition titles and order. Tests passed latest-row, cold load, failed-write non-publication and empty replacement. Trial 024 passed 137684, zero errors (+14.3%). Accepted without extra micro-variance runs; next profiling must establish remaining major work.

## H12 — separate the generated ID namespace (correctness)

Profiler trial 025 passed with one critical score insertion error. Server log proves UNIQUE player_score.id; initial tenant 1 has existing ID 9fa74710 above old reset value 9fa52400. At higher volume the original reset seed itself enters provided IDs. Inspect every initial player/competition/score ID and reserve from 2^62, beyond their verified maximum while fitting signed BIGINT and preserving hexadecimal strings. Durable blocks remain authoritative; this is correctness repair, not a performance score claim.

H12 validation: all 3,742,406 initial player/competition/score IDs inspected, max 26783990078 (63c73413e), invalid hex 0, far below 2^62. Trial 026 no critical/ID insertion error, 127980 after 6 ranking body decode errors. Keep namespace correctness fix; do not claim new performance record or classify remaining decode errors as officially allowed.

## H13 — shared reader locks (rejected)

Parallel readers with exclusive score/player/competition mutations preserved zero-error correctness in 027, but score dropped to 101702. No gain; revert the complete candidate. Do not tune this regression with repeated runs.

## H14 — SQLite WAL with FULL synchronization

Earlier diagnostics showed substantial cumulative write latency; durable transactions still force file synchronization. Try WAL to reduce rollback-journal work while retaining FULL synchronization, bounded pool and existing update locks. Initialization closes all pools before deleting database and WAL/SHM files, preventing stale sidecar recovery. A normal reboot and cold reopen must retain committed data.

H14 outcome: 028 passed 115336 with zero errors, below the existing 137684 best and 127980 namespace validation. Rejected; restore rollback journaling and original initialization lifecycle.

## H15 — remove per-request debug formatting/log output

Profile 025: JSON encoder cumulative 13.19%, syscall writes 12.99%, json.Indent flat 6.39%. Disable Echo debug pretty-printing and routine request middleware logs, retaining error handling and error logs. JSON fields/status/headers and frontend assets remain unchanged. This removes repeated non-business output on every request, without relaxing validation or authentication. Measure once; do not chase marginal changes.

H15 outcome: 029 official pass 100222 with 1 critical score insertion error. Investigation found leftover WAL/SHM files from rejected H14: initial template journal_mode=delete, restored tenant DB journal_mode=wal and residual high-ID rows. The initial rollback script deleted only *.db; stale WAL recovered old rows after restore. Therefore 029 is contaminated and cannot establish a performance effect. Revert H15 and keep explicit sidecar cleanup as a correctness fix. No logging optimization adopted.

Remaining candidates reviewed: RSA signature verification, live authorization/metadata queries and response transport remain; 025 parseViewer CPU 22.92%, retrievePlayer 14.14%. They offer bounded CPU savings and introduce additional invalidation/security boundaries. The large repeated scoring/billing/transaction/setup work has been removed. Stop marginal exploration and validate the final adopted state, reference baseline and restart persistence; 100x is a target, not a result claim.

## H16 — replace tenant SQLite with MySQL (follow-up, rejected)

User requested this previously untested major alternative. Migrate all runtime tenant tables to separate MySQL databases, preserve binary NO PAD ID semantics, cached snapshots/transactions, and use200-row parameterized INSERT chunks. Digest-check all3,742,406 original rows. Restore a cleanly stopped2.9GiB full-MySQL seed for initialization (8.22s precheck,8.85s official). Real MySQL race/vet and normal-reboot API persistence passed.

034 old SQLite control147252 zero errors.035 deployment preparation failed with Docker/containerd missing parent snapshot, no bench.036 same candidate binary/backendSQLite and same1GiB/1024 MySQL settings142942 zero errors.037 backendMySQL103150 zero errors (72.16% of same-condition control), rejected. Retain patch and audit, restore original adopted implementation/configuration.038 verifies rollback. This is evidence against this port, not proof that every SQLite alternative fails or that the current architecture has reached its limit. See MYSQL_EXPERIMENT.md.

## H17-H29 postmortem-informed follow-up

See POSTMORTEM_EXPERIMENTS.md for all13 structural/capacity candidates, measured039-057, patches for rejected candidates and final reboot proof. Article permission begins after038. Fixed CPU/RAM and official load unchanged.57 after capacity repair415120, zero errors;100x remains unmet.
