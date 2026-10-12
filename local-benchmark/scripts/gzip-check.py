#!/usr/bin/env python3
"""Guest-only blackauth/HTTPS check; tokens are neither printed nor saved."""
import gzip
import http.client
import json
import sqlite3
import ssl
import subprocess

name = subprocess.check_output(['mysql', '-N', '-uisucon', '-pisucon', 'isuports', '-e', 'SELECT name FROM tenant WHERE id=1'], text=True).strip()
db = sqlite3.connect('file:/home/isucon/webapp/tenant_db/1.db?mode=ro', uri=True)
player = db.execute('SELECT id FROM player WHERE is_disqualified=0 LIMIT 1').fetchone()[0]
comp = db.execute('SELECT competition_id FROM player_score GROUP BY competition_id ORDER BY COUNT(*) DESC LIMIT 1').fetchone()[0]
headers = {'Host': name+'.t.isucon.local'}
connection = http.client.HTTPSConnection('127.0.0.1', context=ssl._create_unverified_context(), timeout=30)
connection.request('POST', '/auth/login/player?id='+player, body=b'', headers=headers)
response = connection.getresponse()
response.read()
assert response.status == 200
headers['Cookie'] = response.getheader('Set-Cookie').split(';')[0]
headers['Accept-Encoding'] = 'gzip'
connection.request('GET', '/api/player/competition/'+comp+'/ranking', headers=headers)
response = connection.getresponse()
encoded = response.read()
assert response.status == 200 and response.getheader('Content-Encoding') == 'gzip'
assert 'Accept-Encoding' in response.getheader('Vary')
decoded = gzip.decompress(encoded)
result = json.loads(decoded)
assert result['status'] and len(result['data']['ranks']) == 100
print(json.dumps(dict(status=200, content_encoding='gzip', compressed_bytes=len(encoded), decoded_bytes=len(decoded), ranks=100)))
