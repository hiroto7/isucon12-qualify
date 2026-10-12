#!/usr/bin/env python3
import collections
from pathlib import Path
import re
import sys

groups=collections.defaultdict(lambda:[0,0.0,0,0.0])
for block in Path(sys.argv[1]).read_text().split('# Time:')[1:]:
    if len(sys.argv)>2 and block.strip().splitlines()[0] < sys.argv[2]:continue
    m=re.search(r'Query_time: ([\d.]+).*Rows_examined: (\d+)',block)
    if not m:continue
    lines=[line for line in block.splitlines()[1:] if not line.startswith(('#','SET timestamp','use ')) and line.strip()]
    sql=' '.join(lines)
    if not re.match(r'^(SELECT|INSERT|REPLACE|UPDATE|DELETE|ALTER|TRUNCATE)',sql):continue
    sql=re.sub(r"'[^']*'",'?',sql)
    sql=re.sub(r'\b\d+\b','?',sql)
    item=groups[sql];item[0]+=1;item[1]+=float(m[1]);item[2]+=int(m[2]);item[3]=max(item[3],float(m[1]))
print('calls sum_seconds rows_examined max_seconds SQL')
for sql,(n,total,rows,maximum) in sorted(groups.items(),key=lambda x:x[1][1],reverse=True)[:30]:
    print(n,round(total,3),rows,round(maximum,3),sql)
