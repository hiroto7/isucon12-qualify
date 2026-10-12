#!/usr/bin/env python3
"""Summarize SQLite query frequency and time; query time excludes row iteration."""
import collections
import json
from pathlib import Path
import sys

groups=collections.defaultdict(lambda:[0,0.0,0.0])
for line in Path(sys.argv[1]).open():
    try:
        row=json.loads(line)
    except json.JSONDecodeError:
        continue
    group=groups[row['statement']]
    group[0]+=1;group[1]+=row['query_time'];group[2]=max(group[2],row['query_time'])
print('SQLite trace: cumulative query start/exec time; excludes Rows.Next consumption and lock acquisition.')
for sql,(n,total,max_time) in sorted(groups.items(),key=lambda r:r[1][1],reverse=True):
    print(f'{n:8d} calls {total:10.3f}s total {max_time:8.4f}s max  {sql}')
