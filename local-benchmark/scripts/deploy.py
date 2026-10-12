#!/usr/bin/env python3
"""Deploy a managed source set; stop immediately on failed build or verification."""
import argparse
import hashlib
import io
import json
from pathlib import Path
import subprocess
import tarfile
import tempfile

root = Path(__file__).resolve().parents[2]
vm = 'isucon12q-app'
parser = argparse.ArgumentParser()
parser.add_argument('--keep-nginx', action='store_true', help='for exact initial-reference comparisons only')
args = parser.parse_args()
subprocess.run(['multipass','exec','isucon12q-bench','--','sudo','bash','-lc','! pgrep -x bench'],check=True)
def remote(command):
    subprocess.run(['multipass','exec',vm,'--','sudo','bash','-lc',command],check=True)

managed = sorted((root/'webapp/go').rglob('*.go')) + [root/'webapp/go/go.mod',root/'webapp/go/go.sum',root/'webapp/go/Dockerfile',root/'webapp/sql/init.sh',root/'webapp/sql/init.sql',root/'webapp/sql/tenant/10_schema.sql',root/'webapp/sql/admin/10_schema.sql',root/'webapp/docker-compose-go.yml']
hashes={}
with tempfile.NamedTemporaryFile(suffix='.tar.gz') as f:
    with tarfile.open(fileobj=f,mode='w:gz') as archive:
        for p in managed:
            content=p.read_bytes().replace(b't.isucon.dev',b't.isucon.local')
            name=str(p.relative_to(root/'webapp'))
            hashes[name]=hashlib.sha256(content).hexdigest()
            info=tarfile.TarInfo(name);info.size=len(content);info.mode=0o755 if p.suffix=='.sh' else 0o644
            archive.addfile(info,io.BytesIO(content))
    f.flush()
    subprocess.run(['multipass','transfer',f.name,f'{vm}:/tmp/deploy.tar.gz'],check=True)
    remote('set -eu; tar tzf /tmp/deploy.tar.gz >/dev/null; ! pgrep -x bench; systemctl stop isuports; find /home/isucon/webapp/go -name "*.go" -delete; tar xzf /tmp/deploy.tar.gz -C /home/isucon/webapp; chown -R isucon:isucon /home/isucon/webapp/go; cd /home/isucon/webapp; sudo -u isucon docker compose -f docker-compose-go.yml build > /tmp/deploy-build.log 2>&1; systemctl reset-failed isuports; systemctl start isuports')
with tempfile.NamedTemporaryFile(mode='w',suffix='.json') as f:
    json.dump(hashes,f);f.flush()
    subprocess.run(['multipass','transfer',f.name,f'{vm}:/tmp/deploy-manifest.json'],check=True)
remote("python3 - <<'PY'\nimport hashlib,json,pathlib\nexpected=json.load(open('/tmp/deploy-manifest.json'))\nbase=pathlib.Path('/home/isucon/webapp')\nactual={str(p.relative_to(base)) for p in (base/'go').rglob('*.go')}\nassert actual=={p for p in expected if p.endswith('.go')}\nfor name,digest in expected.items():\n assert hashlib.sha256((base/name).read_bytes()).hexdigest()==digest,name\nprint('Managed source set verified')\nPY")
if not args.keep_nginx:
    subprocess.run(['multipass','transfer',str(root/'local-benchmark/nginx.conf'),f'{vm}:/tmp/deploy-nginx.conf'],check=True)
    site = root/'local-benchmark/nginx-site.conf'
    if site.exists():
        subprocess.run(['multipass','transfer',str(site),f'{vm}:/tmp/deploy-nginx-site.conf'],check=True)
        remote('install -m 644 /tmp/deploy-nginx-site.conf /etc/nginx/sites-available/isuports.conf')
    remote('install -m 644 /tmp/deploy-nginx.conf /etc/nginx/nginx.conf; nginx -t; systemctl reload nginx')
print('Deployed source; verify running container readiness before starting the benchmark.')

remote("for n in $(seq 1 30); do curl -fsS -H 'Host: admin.t.isucon.local' http://127.0.0.1:3000/api/me >/dev/null 2>&1 && exit 0; sleep 1; done; exit 1")
