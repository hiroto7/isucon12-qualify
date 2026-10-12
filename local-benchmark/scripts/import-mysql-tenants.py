#!/usr/bin/env python3
"""Guest preparation only: migrate and digest-verify every original SQLite row."""
import hashlib
import json
from pathlib import Path
import sqlite3
import subprocess
import tempfile

BASE=Path('/home/isucon/initial_data')
SCHEMA=Path('/home/isucon/webapp/sql/tenant/10_schema.sql').read_text()
TEXT={'id','title','display_name','player_id','competition_id'}

def mysql(sql, db=None):
    args=['mysql','-uroot','-proot','--default-character-set=utf8mb4','--local-infile=1','-N','-B','--raw']
    if db: args.append(db)
    return subprocess.check_output(args,input=sql,text=True)

def encoded(v):
    if v is None:return r'\N'
    return str(v).replace('\\','\\\\').replace('\t',r'\t').replace('\r',r'\r').replace('\n',r'\n').replace('\0',r'\0').replace('\x1a',r'\Z')

audit=[]
for source in sorted(BASE.glob('*.db'),key=lambda p:int(p.stem)):
    tenant=int(source.stem); db='t12_tenant_'+str(tenant)
    mysql('DROP DATABASE IF EXISTS `'+db+'`; CREATE DATABASE `'+db+'` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin;')
    mysql(SCHEMA,db)
    conn=sqlite3.connect('file:'+str(source)+'?mode=ro',uri=True)
    record={'tenant':tenant,'tables':{}}
    for table in ['player','competition','player_score']:
        columns=[r[1] for r in conn.execute('PRAGMA table_info('+table+')')]
        expected=hashlib.sha256(); count=0
        with tempfile.NamedTemporaryFile(mode='w',dir='/tmp',prefix='t12-import-',encoding='utf-8') as f:
            for row in conn.execute('SELECT '+','.join(columns)+' FROM '+table+' ORDER BY id COLLATE BINARY'):
                f.write('\t'.join(encoded(v) for v in row)+'\n');count+=1
                canonical='|'.join('NULL' if v is None else str(v).encode().hex().upper() if c in TEXT else str(v) for c,v in zip(columns,row))+'\n'
                expected.update(canonical.encode())
            f.flush()
            sql="LOAD DATA LOCAL INFILE '"+f.name+"' INTO TABLE "+table+" CHARACTER SET utf8mb4 FIELDS TERMINATED BY '\\t' ESCAPED BY '\\\\' LINES TERMINATED BY '\\n' ("+','.join(columns)+"); SHOW WARNINGS;"
            warnings=mysql(sql,db)
            assert not warnings,(tenant,table,warnings)
        expressions=['HEX('+c+')' if c in TEXT else "COALESCE(CAST("+c+" AS CHAR),'NULL')" for c in columns]
        query='SELECT CONCAT_WS(\'|\','+','.join(expressions)+') FROM '+table+' ORDER BY id;'
        actual=hashlib.sha256()
        args=['mysql','-uroot','-proot','--default-character-set=utf8mb4','-N','-B','--raw',db,'-e',query]
        with subprocess.Popen(args,stdout=subprocess.PIPE) as process:
            actual_count=0
            for line in process.stdout:
                actual.update(line);actual_count+=1
            assert process.wait()==0
        assert count==actual_count and expected.hexdigest()==actual.hexdigest(),(tenant,table,count,actual_count,expected.hexdigest(),actual.hexdigest())
        record['tables'][table]={'rows':count,'sha256':actual.hexdigest()}
    conn.close();audit.append(record)
    print(json.dumps(record),flush=True)
Path('/home/ubuntu/mysql-tenant-import-audit.json').write_text(json.dumps(audit,indent=2)+'\n')
print('PASS: every original row digest matched in MySQL',flush=True)
