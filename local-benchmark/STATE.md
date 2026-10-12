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
