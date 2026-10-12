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
