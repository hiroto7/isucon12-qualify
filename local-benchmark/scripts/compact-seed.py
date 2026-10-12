#!/usr/bin/env python3
"""Run offline inside the app guest, with isuports stopped. Preserve originals."""
import hashlib
import json
import pathlib
import shutil
import sqlite3
import subprocess

base = pathlib.Path('/home/isucon/initial_data')
backup = pathlib.Path('/home/isucon/uncompressed-initial-data')
assert subprocess.run(['systemctl', 'is-active', '--quiet', 'isuports']).returncode != 0
if not backup.exists():
    shutil.copytree(base, backup)

def digest(rows):
    h = hashlib.sha256()
    for row in rows:
        h.update(json.dumps(row, ensure_ascii=False, separators=(',', ':')).encode())
        h.update(b'\n')
    return h.hexdigest()

audit = {'sqlite': [], 'mysql': {}}
for path in sorted(base.glob('*.db')):
    db = sqlite3.connect(path)
    original = sqlite3.connect(f'file:{backup / path.name}?mode=ro', uri=True)
    count = original.execute('SELECT COUNT(*) FROM player_score').fetchone()[0]
    latest = '''SELECT ps.* FROM player_score ps JOIN
      (SELECT competition_id,player_id,MAX(row_num) AS row_num FROM player_score GROUP BY competition_id,player_id) last
      ON last.competition_id=ps.competition_id AND last.player_id=ps.player_id AND last.row_num=ps.row_num'''
    expected = digest(original.execute(latest + ' ORDER BY ps.id'))
    original.close()
    db.execute('DELETE FROM player_score WHERE id NOT IN (SELECT id FROM (' + latest + '))')
    db.commit()
    db.execute('VACUUM')
    actual = digest(db.execute('SELECT * FROM player_score ORDER BY id'))
    assert expected == actual, path
    after = db.execute('SELECT COUNT(*) FROM player_score').fetchone()[0]
    audit['sqlite'].append(dict(file=path.name, before=count, after=after, semantic_sha256=actual))
    db.close()

def mysql(sql):
    return subprocess.check_output(['mysql', '-N', '-B', '-uroot', '-proot', 'isuports', '-e', sql], text=True)

mysql('DELETE FROM visit_history WHERE created_at>=1654041600')
before = int(mysql('SELECT COUNT(*) FROM visit_history'))
query = '''SELECT tenant_id,HEX(competition_id),HEX(player_id),MIN(created_at),MIN(updated_at)
 FROM visit_history GROUP BY tenant_id,competition_id,player_id ORDER BY tenant_id,competition_id,player_id'''
expected = hashlib.sha256(mysql(query).encode()).hexdigest()
mysql('CREATE TABLE visit_history_compact LIKE visit_history; INSERT INTO visit_history_compact '
      '(tenant_id,competition_id,player_id,created_at,updated_at) '
      'SELECT tenant_id,competition_id,player_id,MIN(created_at),MIN(updated_at) FROM visit_history '
      'GROUP BY tenant_id,competition_id,player_id')
actual = hashlib.sha256(mysql(query.replace('FROM visit_history ', 'FROM visit_history_compact ')).encode()).hexdigest()
assert actual == expected
after = int(mysql('SELECT COUNT(*) FROM visit_history_compact'))
mysql('RENAME TABLE visit_history TO visit_history_uncompressed, visit_history_compact TO visit_history')
audit['mysql'] = dict(before=before, after=after, semantic_sha256=actual)
print(json.dumps(audit, indent=2))
