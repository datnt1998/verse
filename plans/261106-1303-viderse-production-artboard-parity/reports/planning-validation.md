# Kiểm chứng kế hoạch · 2026-11-06

## Phạm vi và trạng thái
Đây là evidence lập kế hoạch, không phải nghiệm thu sản phẩm hoặc production receipt. Không sửa source/artboard, không merge PR, không chạy migration/deploy, không phát sinh mutation/provider call. Repo nguồn thực là `/Users/dat/Projects/verse`; CWD `ainovel-cli` là repo Noverse cũ.

## Quyết định người dùng
Câu hỏi: “Chốt #93 theo phương án nào?”
- A: Tự test iPhone trong 1 ngày: giữ KEEP, test theo physical-safari-handoff.md trong vòng 1 ngày từ preview đúng V40 cuối, chỉ deploy khi đạt.
- B: Bỏ gate, test sau deploy: thay KEEP bằng miễn gate trước deploy, nhưng vẫn phải hoàn tất Safari/production acceptance.
- **Trả lời: A — Tự test iPhone trong 1 ngày.** Không miễn gate khi hết hạn. Đã phản ánh trong index, phase01/02/03 và acceptance-contract vòng sửa.

## Nguồn đã quan sát
- `apps/viderse/docs/design/v1/artboards/README.md:11-56`: 42 artboard, gồm Workflow, 29 màn sản phẩm, 12 UI Kit (10 chuẩn hiện hành và 2 archive). Glob29product board path đều tồn tại.
- `11-hoi-dap.md:670-685`: #91/#92 đã chốtA/triển khai, #93 KEEP chờ bằng chứng.
- Plan cũ `reports/pr-06/acceptance.md:28-37`: native/probe/undo evidence không đồng nghĩa production hoặc Safari; source có private index/SQLC overlay nên clean tracked artifact phải là gate.
- `reports/pr-06/channel-workbench-audit.md:3-28`: production hotfix riêng, workflow cũ còn, không có authenticated production parity.
- `reports/pr-06/physical-safari-handoff.md:3-26`: preview24bf453d là sample-only, checklist5mục/operator/device metadata, không thay backend proof.
- `backend-contracts.md:23-34,50-53`: mapping default/automation/snapshot và maintenance/restore boundary. Sau reopen writes không tự restore snapshot cũ.
- `apps/viderse/deploy/README.md:10-31,103-126`: script/launchd route, serialization với Noverse, public chunk bytes và inode proof.
- **Đã đọc remote thật** `ssh://macmini/~/server/bin/build-deploy-viderse.sh:20-49`: fetch/reset; Dockerfile ngoài repo; install+marker trước migrate; kickstart; readiness chỉ rollback binary. Không quiesce/backup/schema-aware recovery. Phase03 chặn dùng nguyên trạng.
- **Đã đọc wrapper thật** `ssh://macmini/~/server/bin/viderse-run.sh:5-15`: env từ runtime file, DB/identity và public SSO config; không đọc giá trị secret trong file env.
- `tests/viderse-workflow/capture-baseline.mjs:18-36`, `playwright.config.ts:3-7,23-25`, `interaction-audit.pw.ts:6-15,48-52`: loopback/QA-only, cookie ngoài repo và trace tắt; không phải route authenticated production.
- Current glob xác nhận interaction-audit.pw.ts, viewer/studio/review/settings/template/editor audits đã tồn tại. Không tái dùng lời ghi “chưa có suite” từ lịch sử làm sự thật hiện tại.

## Các lệnh/thử nghiệm thực sự đã chạy
| Check | Kết quả |
|---|---|
| `ak plan validate plans/261106-1303-viderse-production-artboard-parity --json` | valid:true, errors:null |
| `ak plan parse ... --json` | 10phase,30phase checklist items,0done,plan pending |
| `node tests/viderse-workflow/artboard-state-manifest.mjs --check` | Exit1, DRIFT, expected/found cùng2004376bytes; không được gọi pass |
| Extractor `--stdout` và deep-compare JSON với inventory cũ | 12khác biệt ở sourceHashes; rows/probes không đổi. Không ghi đè inventory cũ |
| Rà tất cả12file index/contract/phase | Lần đầu34link,66absolute path claims; phát hiện một suffix dòng trong link và report chưa tạo; link đã sửa, report này được tạo; final sweep sau review ghi bên dưới |
| `ak plan use ... --json` | Pin plan cho worktree verse; không sửa source |

Không chạy test/build sản phẩm vì chỉ thay tài liệu kế hoạch. Extractor và CLI parser là smoke cho deliverable planning, không phải production/UI proof. Production login, device proof, preflight data/actions, maintenance và release là công việc còn thực thi.

## Coverage và dependency
29product boards đều có owner: nhóm01 Main/MMain/Activity/Editor; nhóm02 Review/MReview; nhóm03 Channel/MChannel; nhóm04 Library/Template/MLibrary/MTemplate; nhóm05 NewVideo/MNew/Explore/MExplore/MCompVideo/Queue/MQueue; nhóm06 Settings/MSettings; nhóm07 Auth/MAuth/Onboard/CreateChannel/MCreate/System/MSystem. Dialogs instances phân theo parent action và nhóm07 chịu completeness sweep. Workflow/UK/Brand là source rules tại nơi render, không phải pages sản phẩm; Kit/MKit archive.

Phase1→2→3→4→5→6→7→8→9→10 là dependency chain không cycle. Phase04–10 báo ngay từng nhóm đạt nhưng chỉ kết luận toàn workflow khi final deployed revision và mọi affected earlier evidence đã retest. Mọi blocked state/sensitive control giữ nguyên blocker; không lấy sample/QA hoặc giảm count để tạo pass. Ngoại lệ first-run#94A bên dưới là quyết định mới rõ phạm vi, không miễn các state khác.

## Quyết định #94 — validation session 2
Câu hỏi: “Chốt #94: nghiệm thu các trạng thái first-run không thể dựng trên production đã có kênh/identity theo cách nào?”
- A: Ngoại lệ hẹp cho first-run — cùng release commit trên instance first-run biệt lập, backend/dữ liệu thật, không mock; báo rõ#94 và không gọi ảnh đó production.
- B: Giữ tuyệt đối production — giữ blocker đến khi có đường vận hành an toàn được duyệt; không reset dữ liệu/không tuyên bố hoàn tất.
- **Trả lời: A — Ngoại lệ hẹp cho first-run.** Ghi vào11-hoi-dap#94. Chuẩn bị phase01; thực hiện group07; environment/source/hash/evidence tách biệt. Không áp dụng cho paid draws/QR/storage/errors hoặc controls khác chỉ vì khó dựng.

## Red Team Review
4reviewer đọc toàn plan theo Security/Failure/Assumptions/Scope;7finding thô,6unique sau gộp Onboard.0Critical,2High,4Medium.6Accept,0Reject. Không chạy build/test/lint; không có reviewer nào sửa source.

| Finding | Mức | Evidence nguồn | Disposition / nơi xử lý |
|---|---|---|---|
| Onboard cần instance rỗng, không account mới | High | channels.sql:5–8; ChannelNewScreen.tsx:22–26; AppShell.tsx:129–134 | Accept; user#94A; phase01/10,contract,index |
| Desktop navigation có thể tự cài trước click | High | AppShell.tsx:117–120; screens/setup/SetupScreen.tsx:165–186; api/setup.ts:45–57 | Accept; GET /api/setup/state trước mount + installation approval; contract,phase01/03/10 |
| Automation lifetime vượt cửa sổ mutation | Medium | postgres/channels.go:429–437; channels/channels.go:500–503; queue/scheduler.go:43–54 | Accept; terminal readback/admission closure/reconcile; contract,phase06/10 |
| Thiếu explicit consumer acceptance chỉ dẫn bốn trợ lý | Medium | 11-hoi-dap.md:640,649,654–657; postgres/clone.go:449–458; autonomy/scorer.go:121–125,214–226 | Accept; phase06#87–#89 và retest nhóm trước |
| Filter screens/editor không chọn Editor thực | Medium | screens/editor/EditorScreen.tsx:1–6; vitest.editor.config.ts:9–10; editor/v13/Editor.tsx:17–29 | Accept; phase04 inventory editor/v13 và test filter tương ứng |
| Auth first-user setup bất khả thi trên identity đang có user | Medium | platform/webapp/server.go:596–607,622–634; deploy/README.md:3–8 | Accept; user#94A; phase01/10,contract,index |

### Decision deltas cần whole-plan sweep
1.29product boards (không30); Auth desktop giữ, Editor nhóm01.
2.#93 KEEP/user1ngày đúng candidate; không timeout waiver.
3.#94A first-run riêng/cùng release/real/no mock, không account-scoped giả, không production-pass.
4.Setup GET pre-navigation và permission cho auto-install; không dùng cờ skip/intercept.
5.Automation terminal config + lifetime closure sau action cuối.
6.#87–#89 đủ consumer/snapshot/unsupported guard và Editor test đúng owner.
7.Matrix cũ drift12sourceHashes; regenerate separate artifact, không ghi đè lịch sử.

### Verification Results
CLI validate sau propagation trả valid:true/errors:null; CLI status10phase/30unchecked tasks/0completed. Product paths29/29 tồn tại. Các gate runtime vẫn pending; không gọi đây là production verification.

### Whole-Plan Consistency Sweep
- Đã đọc lại13file: index,contract,10phase và report này sau propagation.
- Kiểm35link tương đối:0broken;62path tuyệt đối duy nhất hiện hữu:0missing.
- Dependency1→10 đúng,30phase checklist items đều chưa đánh xong; không cycle/owner overlap.
- Tìm các mâu thuẫn superseded:30board/account-scoped Onboard/recipe tài khoản mới;0còn sót. #94A được phản ánh index/contract/phase01/10 và canonical11-hoi-dap; setup pre-navigation áp phase01/03/10; automation closure áp contract/phase06/10; Editor filter đã đúng editor/v13.
-7decision deltas đã đối chiếu.0unresolved plan contradiction. Runtime prerequisites vẫn là gate chưa thực hiện, không phải finding đã pass.
- Journal đã lưu bằng CLI tại plans/journals/2026-11-06-viderse-production-release-plan-and-acceptance-gates.md. AgentWiki publish skipped; không có yêu cầu publish.
