#!/usr/bin/env python3
"""Guest-only API persistence check; check runs before benchmark initialization."""
import http.client
import json
from pathlib import Path
import ssl
import sys
import time
from urllib.parse import urlencode

state_path = Path('/home/ubuntu/reboot-check-state.json')

def request(tenant, path, cookie='', form=None, body=None, content_type=None, expected=200):
    conn = http.client.HTTPSConnection('127.0.0.1', context=ssl._create_unverified_context(), timeout=30)
    headers = {'Host': tenant+'.t.isucon.local'}
    if cookie: headers['Cookie'] = cookie
    if form is not None:
        body = urlencode(form, doseq=True).encode()
        content_type = 'application/x-www-form-urlencoded'
    if content_type: headers['Content-Type'] = content_type
    conn.request('POST' if body is not None or path.startswith('/auth/login') else 'GET', path, body, headers)
    response = conn.getresponse()
    data = response.read()
    assert response.status == expected, (path, response.status, data)
    cookie = (response.getheader('Set-Cookie') or '').split(';')[0]
    conn.close()
    return (json.loads(data) if data else None), cookie

def login(tenant, role, player=''):
    _, cookie = request(tenant, '/auth/login/'+role+('?id='+player if player else ''), body=b'')
    assert cookie
    return cookie

def observe(s):
    tenant, ids, comp = s['tenant'], s['players'], s['competition']
    organizer = login(tenant, 'organizer')
    player = login(tenant, 'player', ids[0])
    ranking, _ = request(tenant, '/api/player/competition/'+comp+'/ranking', player)
    detail, _ = request(tenant, '/api/player/player/'+ids[0], player)
    billing, _ = request(tenant, '/api/organizer/billing', organizer)
    assert [r['player_id'] for r in ranking['data']['ranks']] == [ids[1], ids[0]]
    assert [r['score'] for r in ranking['data']['ranks']] == [100, 50]
    assert ranking['data']['competition']['is_finished']
    assert detail['data']['scores'] == [{'competition_title': 'Persistent API test', 'score': 50}]
    report = billing['data']['reports'][0]
    assert report['billing_yen'] == 210, report
    request(tenant, '/api/organizer/billing', player, expected=403)
    request(tenant, '/api/player/competitions', organizer, expected=403)
    request('admin', '/api/player/competitions', player, expected=401)
    request(tenant, '/api/player/competitions', login(tenant, 'player', ids[1]), expected=403)
    request(tenant, '/api/player/competitions', expected=401)
    return {'ranking': ranking, 'detail': detail, 'billing': billing}

if sys.argv[1] == 'create':
    tenant = 'reboot-check-'+str(int(time.time()))
    request('admin', '/api/admin/tenants/add', login('admin', 'admin'), form={'name': tenant, 'display_name': 'Persistence check'})
    organizer = login(tenant, 'organizer')
    players, _ = request(tenant, '/api/organizer/players/add', organizer, form={'display_name[]': ['alpha', 'beta', 'spectator']})
    ids = [p['id'] for p in players['data']['players']]
    comp, _ = request(tenant, '/api/organizer/competitions/add', organizer, form={'title': 'Persistent API test'})
    comp = comp['data']['competition']['id']
    boundary = 'isucon-reboot-test'
    csv = 'player_id,score\n'+ids[0]+',100\n'+ids[1]+',100\n'+ids[0]+',50\n'
    body = ('--'+boundary+'\r\nContent-Disposition: form-data; name="scores"; filename="scores.csv"\r\nContent-Type: text/csv\r\n\r\n'+csv+'\r\n--'+boundary+'--\r\n').encode()
    request(tenant, '/api/organizer/competition/'+comp+'/score', organizer, body=body, content_type='multipart/form-data; boundary='+boundary)
    request(tenant, '/api/player/competition/'+comp+'/ranking', login(tenant, 'player', ids[2]))
    request(tenant, '/api/organizer/competition/'+comp+'/finish', organizer, body=b'')
    request(tenant, '/api/organizer/player/'+ids[1]+'/disqualified', organizer, body=b'')
    s = {'tenant': tenant, 'players': ids, 'competition': comp}
    s['before'] = observe(s)
    state_path.write_text(json.dumps(s, indent=2)+'\n')
    print('PASS: real blackauth login, writes, duplicate last-score semantics, ranking, detail, 210 yen billing, disqualification and role/tenant isolation; saved before reboot')
elif sys.argv[1] == 'check':
    s = json.loads(state_path.read_text())
    assert observe(s) == s['before'], 'data changed across reboot'
    players, _ = request(s['tenant'], '/api/organizer/players/add', login(s['tenant'], 'organizer'), form={'display_name[]': ['after-reboot']})
    new_id = players['data']['players'][0]['id']
    assert int(new_id, 16) > max(int(v, 16) for v in s['players']+[s['competition']])
    assert new_id not in s['players']
    print('PASS: identical persisted API responses before initialize, cold caches, renewed blackauth authentication and new non-overlapping ID after normal VM restart')
else:
    raise SystemExit('usage: reboot-check.py create|check')
