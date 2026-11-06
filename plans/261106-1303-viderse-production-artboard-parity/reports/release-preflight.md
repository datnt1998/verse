# Direct Mac mini rollout · 06/11/2026

User explicitly requested deploy on Mac mini to test directly, superseding pre-deploy Safari ordering (#93). Physical Safari and full artboard acceptance remain pending; not waived.

Target https://vid.aarverse.studio, launchd com.dat.viderse,127.0.0.1:8789. Pin main c436aa74a73ae0cf7157445e487dbee75effe2d2; only documentation differs from tested source main60b47664. Native artifact SHA256 ffac589155f30bfd5193b1a9e13de48ce2a55e7f7e37bef39ab9a6bf36bd61b4, staged remote and checksum matched. No Docker rebuild or pruning; no shared-source reset.

Observed before cutover: production marker520d4ba3946ad458bbfa1b023a31f9087c1b2d3e, schema37;1channel,3templates,2persisted job snapshots;0missing rotations,0perDay outside1–20,0active job/identity leases,0unfinished image receipts. Jobs1failed/1succeeded,queue2started,14settled ok image receipts; scans2complete,10failed editor_export task records. Auto-pick on one channel; no notification.preferences row, so migration38 uses documented missing-preference behavior. Default derives from first rotation position/template ID; perDay5,balanced ask yields autoApprovefalse,autoFetchtrue. Snapshot digest b9e9672784083763ca07f9c7381514c5;queue template digest32dbac2b3983d42a21ed2799173f8003. No live providers invoked for preflight.

Owning deployment script hardened at ~/server/bin/build-deploy-viderse.sh: explicit pinned preverified artifact/checksum and --confirm-maintenance,main-ref check,shared lock,no reset checkout; backup previous binary/marker/plist;bootout launchd prevents autorestart; assert no listener/app sessions/active or unsettled operations;pg_dump application DB;restore into separate DB;compare counts/snapshot/queue template digests;rehearse actual migration binary;production migrate38–41;READ ONLY SQL compares rehearsed/real invariants before binary install and writer restart. No automatic binary-only or snapshot rollback.

Operational limit explicitly retained: serve has no no-writer mode. Read-only acceptance is offline SQL while launchd is unloaded; bootstrap is the writer-reopen boundary. Process/public asset/auth checks follow start; a failure stops launchd and preserves DB/receipts for forward recovery. Never claim an invented read-only runtime flag. No identity migration/reset or media deletion. Backup/restore DB and secret config remain remote private,not in repo.

Syntax check and live execution results recorded separately in release proof; this preflight is not deployment success.
