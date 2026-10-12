# Current verified state

- Work complete; 100x goal not reached. Official source pinned 95958774bde66d47fd26cc820fd2fbfbece798f1. Allowed supplied playbooks + official initial code/manuals/data/provisioning only; no ISUCON12 solutions/postmortems/participant articles read. Existing local ISUCON12-related code was not inspected.
- Initial valid scores 6334/6308/6365, median6334; target633400. Corrected-harness pristine reference031=5935, zero errors, exact original managed hashes/configs/data.
- Highest zero-error scoring observation024=137684 (21.74x); before namespace/restore correctness repairs. Final validation030/032/033=114131/105255/120983, all zero errors, median114131 (18.02x). Final after normal restart=120983 (19.10x). Reboot-before032=105255; after is114.94%, passing85% criterion.
- Adopted H1-H7, H9-H11 plus ID namespace2^62 and SQLite WAL/SHM restore cleanup. H8 small/errorful SQL, H13 shared readers, H14 WAL and H15 log/debug candidate are reverted. H15 trial029 is contaminated and not a performance comparison. See HYPOTHESES.md/REPORT.md for evidence and limits.
- Final Go source at fb6ba7d; correctness implementation in a719dd8. Later commits record artifacts only. Deployed whole managed source set verified after reboot. No dirty app sources. Final trial033 hashes equal032 and current source. Runtime binary SHA256 69295a44cdfa9d81431a12a625b14d3e8af1cc13734a7c0e779ce77da1ea27d3.
- Guest app2vCPU/4GiB/30GB and bench4vCPU/8GiB/15GB kept fixed. No host mounts/agent forwarding. Both normal reboot and firewall autostart verified. Old isucon13-app/bench deleted. Diagnostics off, profiler6060 absent; normal nginx/app configuration restored.
- Go race suite + live MySQL ID integration + go vet passed in production container. Real blackauth/HTTPS persistence checks passed BEFORE post-reboot initialize; synthetic before data preserved in validation/reboot-data-before.json. Official strict prepare/static asset validation passed. Browser visual inspection not performed.
- All33 attempts retained, including preparation failure012 and contaminated029; graphs/CSV rebuilt from raw logs.
- Repository fork hiroto7/isucon12-qualify, branch codex/local-benchmark. Draft PR targets fork main, never upstream. Publication/CI state is recorded in final delivery; no merge performed.

## Follow-up SQLite replacement experiment (2026-10-13)

- H16 MySQL runtime tenant databases +200-row bulk inserts:037103150, zero errors. Same compiled binary with backendSQLite and sameMySQL1GiB/1024 configuration:036142942, zero errors. MySQL72.16% of control, rejected. Original configuration/control034147252, zero errors.035Docker preparation failure, no official bench.
- All3,742,406 seed rows digest matched; real MySQL race/vet and normal-reboot API persistence passed. Runtime mysql backend and274 tenant connections verified. Official initialization reset8.85s. Candidate source saved in experiments/mysql-tenants.patch.
- All managed app files restored byte-for-byte to ccd78e7, extra tenant DBs/grants removed, MySQL capacity restored128MiB/151, snapshot service stopped/disabled.038restoration official benchmark144735, zero errors. App CPU/RAM and benchmark settings unchanged. No MySQL runtime implementation adopted.
- Updated total38 attempts and graphs. Existing fork Draft PR#1 receives documentation/experiment records only; app implementation unchanged. See MYSQL_EXPERIMENT.md.
