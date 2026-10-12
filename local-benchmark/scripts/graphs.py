#!/usr/bin/env python3
"""Regenerate complete attempt history and observed record highs from meta.json."""
import csv
import json
import re
from pathlib import Path
import os
import tempfile
os.environ.setdefault("MPLCONFIGDIR", str(Path(tempfile.gettempdir()) / "isucon12-matplotlib"))
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt

root = Path(__file__).resolve().parents[1]
rows = [json.loads(p.read_text()) for p in sorted((root/'results').glob('*/meta.json'))]
for r in rows:
    logpath=root/'results'/f"{r['id']:03d}"/'bench.log'
    if logpath.exists():
        log=logpath.read_text()
        errors=re.search(r'Error (\d+) \(Critical:(\d+)\)',log)
        raw=re.search(r'SCORE: \d+ \(\+(\d+) -?\d+\((\d+)%\)\)',log)
        if errors:r.update(errors=int(errors[1]),critical_errors=int(errors[2]))
        if raw:r.update(raw_score=int(raw[1]),deduction_percent=int(raw[2]))
        (logpath.parent/'meta.json').write_text(json.dumps(r,indent=2)+'\n')
fields = ['id','label','kind','started','ended','commit','status','passed','score','reported_score','raw_score','deduction_percent','errors','critical_errors','decision']
with (root/'all-trials.csv').open('w') as f:
    w=csv.DictWriter(f,fieldnames=fields,extrasaction='ignore',lineterminator='\n');w.writeheader();w.writerows(rows)
baseline_rows = [r['score'] for r in rows if r['label'].startswith('baseline') and r['kind']=='scoring' and r['passed']][:3]
baseline = sorted(baseline_rows)[len(baseline_rows)//2] if baseline_rows else None
record = 0
highs=[]
for r in rows:
    if r['passed'] and r['kind']=='scoring' and r['score']>record:
        record=r['score'];highs.append(r)
for name,selected in [('all-trials',rows),('record-highs',highs)]:
    fig,ax=plt.subplots(figsize=(13,6))
    for kind,marker,color in [('scoring','o','#1769aa'),('diagnostic','s','#d57b00'),('validation','D','#7b42a8')]:
        valid=[r for r in selected if r['passed'] and r['kind']==kind and not r.get('decision','').startswith('rejected')]
        ax.scatter([r['id'] for r in valid],[r['score'] for r in valid],marker=marker,color=color,label=kind,s=50)
    adopted=[r for r in selected if r['passed'] and r.get('decision') in ['accepted','accepted-provisional']]
    if adopted:
        ax.scatter([r['id'] for r in adopted],[r['score'] for r in adopted],facecolors='none',edgecolors='#188038',s=130,linewidths=2,label='adopted (provisional noted in CSV)')
    rejected=[r for r in selected if r['passed'] and r.get('decision','').startswith('rejected')]
    if rejected:
        ax.scatter([r['id'] for r in rejected],[r['score'] for r in rejected],marker='x',color='#c62828',s=70,label='official pass, not adopted')
    invalid=[r for r in selected if not r['passed']]
    if invalid:
        ax.scatter([r['id'] for r in invalid],[0]*len(invalid),marker='x',color='#c62828',label='failed / interrupted / incomplete',s=70)
    if highs:
        ax.step([r['id'] for r in highs],[r['score'] for r in highs],where='post',color='#1769aa',alpha=.4,label='observed scoring record (not adopted state)')
    if name=='all-trials' and any(r['id']>=15 for r in rows):
        ax.axvline(14.5,color='gray',linestyle='--',alpha=.5,label='bench FD limit corrected (control 16)')
    if name=='all-trials' and any(r['id']>=36 for r in rows):
        ax.axvspan(35.5,37.5,color='gray',alpha=.08,label='same MySQL capacity for 36/37')
    if baseline:
        ax.axhline(baseline,color='gray',linestyle=':',label=f'initial median {baseline:,}; goal {baseline*100:,}')
    for r in selected:
        if r.get('decision')=='accepted' or r['label'].startswith('final') or (r['id']>=34 and r.get('score') is not None):
            ax.annotate(f"#{r['id']:02d} {r['score'] or 0:,}",(r['id'],r['score'] or 0),xytext=(3,8),textcoords='offset points',fontsize=8,rotation=20)
    ax.set(title=f'ISUCON12 qualify — {name}',xlabel='Attempt number (all attempts retained)',ylabel='Official valid score')
    ax.set_xlim(min(r['id'] for r in selected)-1,max(r['id'] for r in selected)+3)
    plot_max=max([r.get('score') or 0 for r in selected]+[1])
    ax.set_ylim(-plot_max*.04,plot_max*1.18)
    ax.set_xticks([r['id'] for r in selected]);ax.tick_params(axis='x',labelsize=8)
    ax.grid(alpha=.2);ax.legend(fontsize=8,loc='upper left');fig.tight_layout()
    for ext in ['png','svg']:
        output=root/f'score-{name}.{ext}'
        fig.savefig(output,dpi=160)
        if ext=='svg':
            output.write_text('\n'.join(line.rstrip() for line in output.read_text().splitlines())+'\n')
    plt.close(fig)
