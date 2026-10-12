#!/usr/bin/env python3
"""Regenerate complete attempt history and observed record highs from meta.json."""
import csv
import json
from pathlib import Path
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt

root = Path(__file__).resolve().parents[1]
rows = [json.loads(p.read_text()) for p in sorted((root/'results').glob('*/meta.json'))]
fields = ['id','label','kind','started','ended','commit','status','passed','score','reported_score','decision']
with (root/'all-trials.csv').open('w') as f:
    w=csv.DictWriter(f,fieldnames=fields,extrasaction='ignore');w.writeheader();w.writerows(rows)
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
        valid=[r for r in selected if r['passed'] and r['kind']==kind]
        ax.scatter([r['id'] for r in valid],[r['score'] for r in valid],marker=marker,color=color,label=kind,s=50)
    invalid=[r for r in selected if not r['passed']]
    if invalid:
        ax.scatter([r['id'] for r in invalid],[0]*len(invalid),marker='x',color='#c62828',label='failed / interrupted / incomplete',s=70)
    if highs:
        ax.step([r['id'] for r in highs],[r['score'] for r in highs],where='post',color='#1769aa',alpha=.4,label='observed scoring record (not adopted state)')
    if baseline:
        ax.axhline(baseline,color='gray',linestyle=':',label=f'initial median {baseline:,}; goal {baseline*100:,}')
    for r in selected:
        if r.get('decision')=='accepted' or r['label'].startswith('final'):
            ax.annotate(r['label'],(r['id'],r['score'] or 0),xytext=(3,8),textcoords='offset points',fontsize=8,rotation=15)
    ax.set(title=f'ISUCON12 qualify — {name}',xlabel='Attempt number (all attempts retained)',ylabel='Official valid score')
    ax.grid(alpha=.2);ax.legend(fontsize=8,loc='upper left');fig.tight_layout()
    for ext in ['png','svg']:
        fig.savefig(root/f'score-{name}.{ext}',dpi=160)
    plt.close(fig)
