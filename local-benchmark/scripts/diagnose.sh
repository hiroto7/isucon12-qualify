#!/bin/bash
# Run inside the app guest, before starting a diagnostic trial.
set -eu
out=$1
mkdir -p "$out"
# Low-rate system samples; detach inside the guest so the control client can exit.
nohup pidstat -h -u -r -d -w -p ALL 2 60 > "$out/pidstat.txt" 2>&1 < /dev/null &
nohup vmstat -w 2 60 > "$out/vmstat.txt" 2>&1 < /dev/null &
# CPU capture begins after preparation, aligned to the load phase by the caller.
