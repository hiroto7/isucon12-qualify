# Hypotheses

Pending initial measurement. Static inspection suggests:

1. CSV performs a MySQL ID round-trip and a separate SQLite autocommit for every row, under a tenant-wide file lock. Hypothesis: a single atomic replacement transaction can remove row-count-scaled fsync and reader blocking.
2. Ranking scans all score rows, deduplicates, fetches each player's metadata separately and sorts under the same tenant lock on every read. Hypothesis: derive reusable ranking data per successful upload, preserving last-row and tie order.
3. Player detail performs one score lookup per historical competition, and billing repeatedly scans history. Hypothesis: indexed or precomputed current values reduce historical-data-scaled work.

Not accepted findings: priority and expected score effect require load measurements. Further inspection must include all call sites/branches, not just shared helpers.
