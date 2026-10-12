#!/usr/bin/env python3
"""Start detached official benchmarks and collect every attempt, including failures."""
import argparse
import csv
import datetime
import hashlib
import json
from pathlib import Path
import re
import shlex
import subprocess

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / 'local-benchmark'

def run(*args):
    return subprocess.check_output(args, text=True).strip()

def remote(vm, command):
    return run('multipass', 'exec', vm, '--', 'sudo', 'bash', '-lc', command)

def manifest():
    paths = sorted((ROOT / 'webapp/go').rglob('*.go')) + [ROOT / 'webapp/sql/init.sh',ROOT / 'webapp/sql/init.sql', ROOT / 'webapp/sql/tenant/10_schema.sql', ROOT / 'webapp/sql/admin/10_schema.sql', ROOT / 'webapp/docker-compose-go.yml', ROOT / 'webapp/go/Dockerfile', ROOT / 'webapp/go/go.mod', ROOT / 'webapp/go/go.sum']
    return {str(p.relative_to(ROOT)): hashlib.sha256(p.read_bytes()).hexdigest() for p in paths}

parser = argparse.ArgumentParser()
parser.add_argument('action', choices=['start', 'collect'])
parser.add_argument('id', type=int)
parser.add_argument('--label', default='')
parser.add_argument('--kind', choices=['scoring', 'diagnostic', 'validation'], default='scoring')
a = parser.parse_args()
trial = OUT / 'results' / f'{a.id:03d}'
guest = f'/home/ubuntu/trials/{a.id:03d}'
if a.action == 'start':
    if trial.exists():
        raise SystemExit('Trial already exists; never overwrite a trial')
    remote('isucon12q-bench', "! pgrep -x bench")
    remote('isucon12q-app', 'systemctl is-active nginx isuports mysql blackauth')
    trial.mkdir(parents=True)
    app_ip = json.loads(run('multipass', 'info', 'isucon12q-app', '--format', 'json'))['info']['isucon12q-app']['ipv4'][0]
    meta = dict(id=a.id, label=a.label, kind=a.kind, started=datetime.datetime.now(datetime.timezone.utc).isoformat(), commit=run('git', '-C', str(ROOT), 'rev-parse', 'HEAD'), dirty=run('git', '-C', str(ROOT), 'status', '--porcelain', '--', 'webapp'), manifest=manifest(), target=app_ip, passed=False, score=None, reported_score=None, status='running', decision='pending')
    meta['runtime'] = remote('isucon12q-app', "pid=$(pgrep -x main | head -1); test -n \"$pid\"; sha256sum /proc/$pid/exe /etc/nginx/nginx.conf /home/isucon/webapp/docker-compose-go.yml; docker inspect --format '{{.Image}}' webapp-webapp-1")
    meta['mysql_environment'] = remote('isucon12q-app', "mysql -N -uisucon -pisucon isuports -e \"SHOW VARIABLES WHERE Variable_name IN ('innodb_buffer_pool_size','max_connections','innodb_flush_log_at_trx_commit','sync_binlog')\"; test ! -f /etc/mysql/mysql.conf.d/isucon-tenant-experiment.cnf || sha256sum /etc/mysql/mysql.conf.d/isucon-tenant-experiment.cnf")
    meta['bench_nofile'] = 65536
    if a.id >= 40:
        meta['data_environment'] = remote('isucon12q-app', "sha256sum /home/isucon/initial_data/*.db; mysql -N -uisucon -pisucon isuports -e 'SELECT COUNT(*) FROM visit_history WHERE created_at<1654041600'; test ! -f /home/ubuntu/compact-seed-audit.json || sha256sum /home/ubuntu/compact-seed-audit.json")
    (trial / 'meta.json').write_text(json.dumps(meta, indent=2)+'\n')
    command = f'cd /home/ubuntu/isucon12-qualify/bench; date -u +%FT%TZ > {guest}/start; ./bench -target-addr {app_ip}:443 -exit-error-on-fail > {guest}/bench.log 2>&1; rc=$?; date -u +%FT%TZ > {guest}/end; echo $rc > {guest}/exit'
    remote('isucon12q-bench', f'mkdir -p {guest}; chown ubuntu:ubuntu {guest}; systemd-run --quiet --unit=isucon-trial-{a.id:03d} --uid=ubuntu --property=LimitNOFILE=65536 /bin/bash -lc {shlex.quote(command)}')
    print(f'Started trial {a.id:03d}: {a.label}')
else:
    meta = json.loads((trial / 'meta.json').read_text())
    if remote('isucon12q-bench', f'if test -f {guest}/exit; then echo done; else echo running; fi') != 'done':
        raise SystemExit('Trial still running; no result collected')
    for name in ['bench.log', 'start', 'end', 'exit']:
        subprocess.run(['multipass', 'transfer', f'isucon12q-bench:{guest}/{name}', str(trial / name)], check=True)
    log = (trial / 'bench.log').read_text()
    score = re.search(r'SCORE: (\d+)', log)
    errors = re.search(r'Error (\d+) \(Critical:(\d+)\)', log)
    meta['errors'] = int(errors[1]) if errors else None
    meta['critical_errors'] = int(errors[2]) if errors else None
    passed = 'PASSED: true' in log and (trial/'exit').read_text().strip() == '0'
    meta.update(passed=passed, score=int(score[1]) if passed and score else None, reported_score=int(score[1]) if score else None, status='pass' if passed else 'fail', ended=(trial/'end').read_text().strip())
    (trial / 'meta.json').write_text(json.dumps(meta, indent=2)+'\n')
    rows = [json.loads(p.read_text()) for p in sorted((OUT/'results').glob('*/meta.json'))]
    fields = ['id','label','kind','started','ended','commit','status','passed','score','reported_score','errors','critical_errors','decision']
    with (OUT/'all-trials.csv').open('w') as f:
        w=csv.DictWriter(f, fieldnames=fields, extrasaction='ignore',lineterminator='\n'); w.writeheader(); w.writerows(rows)
    print(json.dumps({k:meta.get(k) for k in fields}, ensure_ascii=False))
