#!/usr/bin/env python3
"""Loopback-only isolated-guest initialization; clean full-MySQL snapshot restore."""
from http.server import BaseHTTPRequestHandler,HTTPServer
import json
from pathlib import Path
import subprocess
import time

SEED=Path('/var/lib/mysql-isucon12-seed')
class Reset(BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path!='/reset':self.send_error(404);return
        start=time.monotonic()
        try:
            assert (SEED/'.seed-ready').is_file(),'verified seed missing'
            subprocess.run(['systemctl','stop','mysql'],check=True,timeout=10)
            subprocess.run(['rm','-rf','/var/lib/mysql'],check=True,timeout=10)
            subprocess.run(['cp','-a',str(SEED),'/var/lib/mysql'],check=True,timeout=20)
            subprocess.run(['systemctl','start','mysql'],check=True,timeout=15)
            subprocess.run(['mysqladmin','-uroot','-proot','ping'],check=True,timeout=5,stdout=subprocess.DEVNULL)
            body=json.dumps({'restored':True,'seconds':time.monotonic()-start}).encode()
            self.send_response(200);self.end_headers();self.wfile.write(body)
            print(body.decode(),flush=True)
        except Exception as e:
            self.send_error(500,str(e));print(str(e),flush=True)
HTTPServer(('127.0.0.1',3002),Reset).serve_forever()
