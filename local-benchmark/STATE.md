# Current state

- Official initial commit: 95958774bde66d47fd26cc820fd2fbfbece798f1.
- Read allowed sources only: supplied ISUCON14/13 playbooks, ISUCON12 official manuals/source, matsuu provisioning. No ISUCON12 commentary, solutions, or participant reports.
- Existing local ISUCON12-related directories are not inspected.
- Fork: hiroto7/isucon12-qualify; PR must target this fork, never upstream.
- Old isucon13-app and isucon13-bench deleted.
- New Multipass QEMU ARM64 guests: app 2 CPUs/4 GiB/30 GB, bench 4 CPUs/8 GiB/15 GB; no host mounts.
- Initial valid scores: 6334, 6308, 6365. Baseline median 6334; goal 633400. All three zero errors. Trial 001 failed static expectation due to hostname setup; not a valid score.
- H1 atomic CSV replacement accepted: 8165/7896, zero errors; initial profiling 005 collected/analyzed and settings disabled.
- Next: cover MySQL visit grouping with an index; remeasure shifted bottlenecks after acceptance.
- Goal: 100 times initial three successful scoring runs' median; stop if grounded major improvements are exhausted.

- H2 visit covering index provisionally accepted, 11178. New heavy tenant billing timeout remains; final state must resolve it.

- H3 accepted: SQLite score partition index + app nofile 16384; trial 010 12526, zero errors. Next: profile higher load, then remove per-row durable ID round trips.

- H4 durable block ID reservation accepted, 28258 with zero errors (trial 013). MySQL/race integration tests passed. Trial 012 records failed deployment only; Docker guest-generated cache/image was rebuilt.

- H5 ranking snapshots accepted: 40420 zero errors; same-condition exact H4 control 28828. Bench process FD soft limit explicitly 65536 from trial 015. Candidate source restored; guest is temporarily still H4 control, so deploy next candidate before measuring.

- H6 billing reuse + nginx connections 8192/worker nofile 16384 accepted: 68407, zero errors. Trial 017 invalid due to nginx capacity. Source and guest match H6. Next: reprofile current workload for the next structural change.

- H7 atomic player batches accepted, trial 020 83561 with zero errors. H6 profiler 019 sampled mid/late load and was analyzed; diagnostics disabled. Next: remove 352790 per-competition score queries in player detail.

- H8 details SQL rejected (small gain and 7 timeouts), source reverted. H9 durable first visits accepted, 104623 zero errors. Next: inspect/reduce repeated SQLite connection/statement setup; no new schema/data shortcuts.
