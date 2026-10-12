#!/usr/bin/env python3
import collections
from pathlib import Path
import re
import sys
stats=collections.defaultdict(list)
for line in Path(sys.argv[1]).open():
 parts=line.split()
 if len(parts)!=6: continue
 method,path,status,duration,upstream,size=parts
 path=re.sub(r'(/api/(?:organizer|player)/(?:competition|player))/[^/]+', r'\1/:id', path)
 stats[(method,path,status)].append(float(duration))
print('method route status count sum_seconds avg_ms p95_ms max_ms')
for key,vals in sorted(stats.items(),key=lambda x:sum(x[1]),reverse=True):
 vals.sort();n=len(vals)
 print(*key,n,round(sum(vals),3),round(sum(vals)/n*1000,2),round(vals[min(n-1,int(n*.95))]*1000,2),round(vals[-1]*1000,2))
