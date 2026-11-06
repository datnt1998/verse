---
phase: 3
title: "Deploy main nguyên tử UI/backend/schema"
status: pending
priority: P1
effort: ""
dependencies: [2]
---
# Phase 03 · Phát hành nguyên tử, không chỉ đổi nhãn
## Mục tiêu / evidence
Production https://vid.aarverse.studio chạy main có V36–V40 và cutover V38/V39 đúng dữ liệu. [Backend contract đã duyệt](../261103-0752-viderse-workflow-rounds-40-44/backend-contracts.md), mục Chuyển dữ liệu V38, dòng23–34. [Runbook](../../apps/viderse/deploy/README.md). Phase này chỉ xác minh rollout, KHÔNG đủ kết luận khớp artboard.
## File inventory / ownership
- Đọc/sửa có kiểm soát:/Users/dat/Projects/verse/apps/viderse/deploy/README.md; migration owner /Users/dat/Projects/verse/apps/viderse/internal/persistence/migrations/; channels/default-template/queue API và test owner từ phase cũ. Không sửa migration đã deploy.
- Operational owner Mac mini: ~/server/bin/build-deploy-viderse.sh; ~/server/bin/viderse-run.sh; ~/server/apps/viderse/bin/viderse và viderse.commit; launchd com.dat.viderse; source ~/server/src/verse.
- Runtime secret source đã xác minh: viderse-run.sh source ~/server/apps/viderse/.env; chỉ đọc tên/metadata, không in nội dung/token/DSN. Không thay auth/grants/cookie domain.
- Tạo reports/release-preflight.md, release-proof.json và read-only acceptance ledger tại plan này; backup/credential ngoài repo mode0600.
## Lỗ hổng script đã đọc trực tiếp khi planning
Remote build-deploy-viderse.sh:20–37 fetch/reset main, build từ Dockerfile ngoài repo ~/server/apps/viderse/Dockerfile, install binary/marker TRƯỚC migrate, rồi kickstart;39–49 chỉ readyz rồi binary rollback. Không có quiesce/backup/restore/schema-aware rollback. Không chạy nguyên trạng cho V38; readiness tự rollback binary cũ sau drop schema có thể làm sự cố nặng hơn. Kiểm lại script/Dockerfile/version trước cutover vì có thể thay đổi sau planning.
## Bước thực hiện
1. Xác nhận phase02 PASS, PR#7 merged main và hotfix playback/Editor revision/template đã tích hợp; giữ UI/backend từ cùng immutable release SHA. Serialize với Noverse deploy vì chung source checkout; không reset checkout có thay đổi người dùng. Pin expected main SHA và đối chiếu fetch/build/install, có drift thì dừng. Kiểm Dockerfile ngoài repo có build tracked web assets đúng SHA, không embed dist cũ hoặc overlay ngoài source.
2. READ-ONLY preflight production: migrate status/version; legacy schema; channel rotation position/id, template tồn tại/quyền, effective autoApprove/autoFetch/perDay, notification preferences; pending/running jobs/paid receipts; snapshot đã persist, storage/identity health. Kiểm từng trường theo backend-contracts, không chỉ chữ trên UI. perDay ngoài1–20, rotation rỗng/dangling, prefs không map được: dừng, ghi câu hỏi11-hoi-dap, không clamp/random/fake ID/auto-disable. Nếu schema đã cutover một phần, không chạy lại migration mù; xác định đúng version và hậu điều kiện.
3. Chuẩn bị và review thay đổi nhỏ nhất cho script vận hành: build/stage trước; lock deploy; explicit expected SHA; maintenance gate quiesce; backup+restore proof; migrate role; schema-aware failure branch và trì hoãn writer resume. Cập nhật runbook đúng route thay đổi. Không thêm một deploy stack thứ hai. Thử logic trên DB QA upgrade từ legacy, không chạy destructive rehearsal trên production. Cơ chế rollback readiness KHÔNG được tự khởi binary cũ trên schema mới; migration error giữ maintenance và evidence.
4. Chốt cửa sổ maintenance với chủ vận hành trước tác động; đóng HTTP mutations và mọi writer: discovery/admission/queue/workers/receipt finalization. Drain/reconcile paid attempts; ngăn launchd auto-restart writer cũ, không lấy SIGTERM hay listener tạm vắng làm quiesced. Nếu chưa drain/reconcile được, dừng trước backup. Account/session/login vẫn theo policy thật; không mở origin/auth tạm.
5. Chụp backup nhất quán DB nghiệp vụ và các tài nguyên bị thay đổi (giữ media manifest/blob references, key/config và binary/marker cũ); kiểm checksum, restore vào DB trống riêng và xác minh counts/invariants. Không reset identity production, không backup secret vào repo. Giữ quiesced từ backup qua read-only acceptance; workspace tar không thay pg_dump. Các lệnh backup/restore cụ thể lấy từ runtime DB transport/roles hiện tại, không paste DSN vào argv/log; không dùng identity dump làm backup data DB.
6. Migrate bằng role VIDERSE_MIGRATION_DATABASE_URL. Backfill defaultTemplateId=rotation đầu theo position/id, không cursor/tên mẫu; autoFetch giữ AutoPick AND resolved AutoPicks; autoApprove theo effective approve, chỉ tự qua khi TẤT CẢ ảnh có score≥85 (thiếu score không qua); perDay giữ1–20. Giữ templates/media, clone_jobs.template_snapshot, explicit queue template; queue chưa admission không có recipe snapshot để hứa giữ. Ba suggestion types thực trong Chờ bạn cùng producer/data contract, không đổi nhãn feature cũ.
7. Start candidate trong chế độ maintenance/read-only đã chứng minh writer tắt; nếu app không có read-only mode thì dùng route hạn chế và worker disable thực sự sẵn có hoặc bổ sung tối thiểu ở bước3, không giả tưởng một env flag. Xác minh default FK/delete-restrict/shared refs, prefs, job snapshot/admission lock, scheduler manual-front/auto-tail, retired collector/routes, migration version. Đối chiếu before/after counts và jobs/receipts. Chưa reopen writes cho tới gate này đạt.
8. Chứng minh release: source SHA→tracked build→binary SHA/marker→PID executable inode→public entry/lazy chunk byte SHA. launchd có thể giữ inode cũ; không dùng file SHA đơn lẻ. Kiểm loopback/public readyz, healthz. Trước mount desktop shell, dùng phiên hợp lệ GET /api/setup/state: incomplete có thể tự chuyển setup/start install, nên phải có quyền cài trước navigation hoặc giữ blocked, không giả state/cờ skip. Sau gate này đăng nhập thật và đọc Kênh/Bàn dựng; không dùng Cloudflare raw HTML hash làm version. Kiểm tab mới/cache cũ không làm UI/BE lẫn contract, giữ tab có unsaved data.
9. Read-only gate đạt mới reopen writers có giám sát; trên entity nghiệm thu được phép, tạo job qua POST/api/jobs, đọc lại default hoặc override và snapshot khi admission, thấy proposal thật đúng ba loại với tác dụng. Quan sát job đang có tiếp tục không lặp paid attempts. Chưa có thực thi an toàn thì blocked acceptance, không bịa PASS từ202.
10. Bắt đầu phase04 ngay. Các sửa parity sau phải merge/deploy main bằng cùng route rồi chụp lại; không báo source-local là đã khớp production.
## Verification commands / cơ chế
Chỉ là lệnh tương lai; host macmini, env do wrapper nạp, không in secret:
- ~/server/bin/viderse-run.sh migrate status; ~/server/bin/viderse-run.sh migrate version.
- shasum -a256 ~/server/apps/viderse/bin/viderse; đọc marker bằng read; launchctl list com.dat.viderse; lsof -nP -a -p <actual-PID> -d txt; stat -f 'device=%d inode=%i' ~/server/apps/viderse/bin/viderse.
- curl -fsS http://127.0.0.1:8789/readyz và https://vid.aarverse.studio/readyz; healthz chỉ process.
- Trước release sau thay schema/query: focused channels/queue/postgres rồi go test ./apps/viderse/... với Postgres QA riêng; race package đổi; sqlc regenerate khi query đổi; npm tests/typecheck/build/size touched surface. Native UI/API smoke phải đọc dữ liệu thật, không chỉ test suite.
## Test matrix / hậu điều kiện
| Mức | Ca | Pass |
|---|---|---|
| Critical | Upgrade legacy + restore rehearsal | Backfill đúng, counts/snapshots giữ, restore DB trống chứng minh |
| Critical | Failure migrate/readiness | Giữ maintenance; không binary-only rollback qua schema incompatible |
| Critical | Default A→B concurrent admission, explicit T | Admission snapshot serialized; job cũ bất biến; override không đổi default |
| High | Job manual/auto/dedup/restart và score84/85/missing | Ordering/no double charge; autoApprove đúng biên |
| High | Process/public assets/auth UI | Cùng deployed SHA; thực sự đọc contract mới |
## Rollback boundary
Trước reopen writes, rollback chỉ khi writers vẫn quiesced liên tục: phục hồi DB nghiệp vụ từ backup đã restore-test và đúng binary/config cũ theo compatibility; giữ media/receipt journal. Nếu có side effect bên ngoài từ trước đó phải reconcile trước, không claim down lossless. Sau reopen writes hoặc accepted paid attempt mới: CẤM auto-restore snapshot cũ; giữ receipts/jobs, forward-fix hoặc recovery được duyệt riêng. Không chạy script auto binary rollback nguyên trạng. Bất kỳ failure nào giữ trạng thái release-blocked, không gọi production xong.
## Checklist
- [ ] Preflight/mapping/maintenance/backup/restore gate đạt; script vận hành an toàn schema đã review/thử riêng.
- [ ] Main SHA duy nhất chạy FE+BE+schema; source/binary/inode/public chunks/auth read proof đạt trước reopen writes.
- [ ] Safe real write/admission/proposals readback đạt; rollback boundary và deployed SHA được ghi; chuyển nhóm01.
