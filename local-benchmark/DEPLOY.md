# Deployment and reproduction

- Guests: Multipass/QEMU ARM64 Ubuntu 22.04. app 2 vCPU/4 GiB/30 GB; bench 4 vCPU/8 GiB/15 GB.
- No shared host mounts or agent forwarding. Outbound firewall permits loopback, established control connections, guest peer, DNS/DHCP, and public internet; blocks other RFC1918/link-local addresses and disables guest IPv6. Persistent isolate-guest.service restores rules at boot. This is VM/filesystem/network separation, not a guarantee against hypervisor vulnerabilities.
- cloud-init-app.yaml pins official source and fixes obsolete Compose/MySQL package installation and retired Debian security-package URLs. These changes precede the performance baseline. cloud-init-bench.yaml builds the same official commit with only the local hostname substitution.
- App initial logic remains the supplied Go implementation. profile.go is disabled unless ISUCON_PROFILE=1; profiling listens only on guest loopback.
- `python3 local-benchmark/scripts/deploy.py` copies the complete managed Go source set and SQL initialization/schema, deletes stale managed Go files, builds, restarts, and verifies source hashes. Hostname substitution is deterministic: t.isucon.dev -> t.isucon.local.
- Before benchmark: check readiness, current container/image, binary hash, configuration and source manifest. No deployment during a trial. Only one benchmark at a time.
- `python3 local-benchmark/scripts/trial.py start N --label LABEL [--kind diagnostic]` starts a detached official benchmark in the bench guest. Once its exit file exists, use `collect N`. Never reuse an attempt ID.
- The official benchmark initializes before compatibility checks and 60-second load. No duration, strictness, scenario, worker or reproduce changes.
- Plot: `/private/tmp/isucon12-plot/bin/python local-benchmark/scripts/graphs.py` (matplotlib 3.9.4); PNG and SVG from every meta.json, including failures.
- Rollback requires restoring Go sources, SQL init/schema, runtime configuration and affected DB structures, then initialization/restart verification. A Git checkout alone does not restore deployed state.
- Provisioning encountered an initial shell heredoc indentation error, unset cloud-init Go cache environment, obsolete Compose API, double mysql-client substitution and retired Debian security URLs. None produced a scored benchmark. A deployment archive flush error was detected before benchmarking and corrected.

- Deployment now also installs the tracked nginx.conf and checks it; `--keep-nginx` is reserved for exact baseline comparisons. The deploy tool checks for a live official bench process in the bench guest before mutating the app guest.
- From attempt 015 the detached benchmark process has LimitNOFILE=65536. Attempt 016 repeats the exact H4 source, and attempt 031 repeats baseline 002 with matching source hashes and pristine official initial data under that corrected harness.
- Final/reference trials capture compiled app SHA256, Docker image ID, nginx/Compose config hashes and source manifest. Earlier trials have source manifests plus diagnostic/build records; do not infer unrecorded runtime hashes.
- Source-only rollback from a WAL experiment is insufficient: stop app, remove database WAL/SHM sidecars before initial-data restoration, and verify template versus restored journal modes. Attempt 029 records the discovered contamination.
- Initial-data archive URL is the official data/20220712_1505-745a3fdfb5783afc048ecaebd054acd20151872d release; SHA256 024fc58d1468ac827caf023d0bfdf0e7375fe1d0ce9dea78a2fb3bedf5786506.
- Run the guest-only `reboot-check.py create` after a final benchmark, normally restart the app VM, then run `reboot-check.py check` BEFORE any next `/initialize`. It exercises actual blackauth cookies and production HTTPS APIs. Tokens stay in guest memory and are not saved in the report.
- Portable graph reproduction: create a Python virtual environment, install matplotlib==3.9.4, then run `python local-benchmark/scripts/graphs.py`.

## Postmortem-informed adopted state

Before first deployment, stop isuports and run compact-seed.py as root inside the isolated app guest. This is one-time offline preparation on official initial_data templates/native admin DB, preserving originals and verifying canonical row hashes; do not rerun after completed compaction or overwrite backups. Save the returned audit. The runtime init.sh restores these compressed templates and clears later visit rows. CSV processing still reports the original input count and validates all rows/IDs.

Deploy copies the tracked nginx-site.conf and sysctl-capacity.conf, persists sysctl settings before restarting Go, checks nginx syntax, and restarts nginx to create per-worker reuseport queues. Readiness now tests actual HTTPS443 with the local practice certificate; Go backend remains TCP3000. Application/benchmark CPU and memory remain2CPU/4GiB and4CPU/8GiB. Include sysctl/site hashes and listen-drop counter deltas in the runtime audit. Normal reboot verification must check data BEFORE next initialize and confirm zero errors/queue drops in a subsequent official benchmark.
