#!/usr/bin/env python3
import argparse
import subprocess
p=argparse.ArgumentParser();p.add_argument('mode',choices=['on','off']);a=p.parse_args()
script=r'''
import pathlib
p=pathlib.Path('/home/isucon/webapp/docker-compose-go.yml')
s=p.read_text()
s='\n'.join(line for line in s.splitlines() if 'ISUCON_PROFILE:' not in line and 'ISUCON_SQLITE_TRACE_FILE:' not in line)+'\n'
if MODE=='on':
 s=s.replace('    environment:\n','    environment:\n      ISUCON_PROFILE: "1"\n      ISUCON_SQLITE_TRACE_FILE: /home/isucon/tmp/sqlite-trace.jsonl\n')
p.write_text(s)
'''.replace('MODE',repr(a.mode))
subprocess.run(['multipass','exec','isucon12q-app','--','sudo','python3','-c',script],check=True)
subprocess.run(['multipass','exec','isucon12q-app','--','sudo','bash','-lc','rm -f /home/isucon/tmp/sqlite-trace.jsonl; systemctl restart isuports'],check=True)
