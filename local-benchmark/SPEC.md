# Specification and score path

Sources: official contest/application manuals saved here, official initial source 9595877. No solutions or retrospectives were read through trial038. They were first read after explicit user permission;039 onward is the postmortem-informed phase.

- Each successful player detail/ranking/competition-list request scores 1. Organizer/admin requests score 10.
- Critical errors deduct 10% each, other validation errors 1% each; 100% deduction stops the run. Initialization has a 30-second deadline; official load lasts 60 seconds.
- Organizer completion drives new competition/player work and reader creation. Faster reads keep player workers alive; three slow rankings can drop a player worker. A locally faster SQL alone is not acceptance evidence.
- CSV replaces an entire competition's score set. Last CSV occurrence per player wins; rank sorts score descending, then last occurrence row number ascending. Up to 100 ranks after the supplied offset. Player detail reports scores in competition creation order.
- Tenant and player permissions are enforced on every player route. Disqualification must affect all player routes, not just the ranking payload.
- Billing: distinct scored players cost 100 yen; other players whose first ranking visit is no later than finish time cost 10 yen. Unfinished competitions bill zero. Finish-to-billing and disqualification reflection allowances are specific 3-second allowances, not general permission to lose writes.
- POST /initialize restores initial tenant SQLite data and admin state. New schema/derived structures must survive initialization. Successful writes must remain readable after a normal VM restart.
- Unmodified public assets and blackauth remain available for UI checks.
- Fixed local ARM64 single-app topology differs from official 3-node c5.large; scores measure this practice setup only.
