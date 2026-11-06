---
title: "Viderse · phát hành workflow V36–V40 và khớp artboard production"
description: "Đóng V40, giữ gate iPhone Safari, cutover UI/backend đồng bộ và nghiệm thu production từng nhóm màn."
status: in-progress
priority: P1
effort: ""
branch: refactor/viderse-v40-mobile-parity
tags: [frontend, backend, database, infra, auth]
blockedBy: []
blocks: []
created: 2026-11-06
---
# Viderse · production là tiêu chuẩn hoàn tất
## Kết quả / phạm vi
HOLD SCOPE, mode **deep**: nhiều màn, cutover schema và release có rủi ro. Không làm lại V36–V39 đã merge, không thêm feature/artboard/viewer45/mobile Editor. Tái dùng kit, contracts, matrix/tool hiện hữu. Mười phase giữ ba gate phát hành và đúng bảy nhóm người dùng yêu cầu; không phải mười kiến trúc mới. Phase01 có inventory chi tiết; re-scout từng phase trước sửa.
Repo thực:/Users/dat/Projects/verse. Checkout ainovel-cli là repo Noverse cũ, không phải nơi triển khai. [Audit production](../261103-0752-viderse-workflow-rounds-40-44/reports/pr-06/channel-workbench-audit.md) xác nhận hotfix không phải workflow mới. Planning không merge/deploy hoặc nghiệm thu production.
Nguồn:[Workflow v97](../../apps/viderse/docs/design/v1/artboards/Workflow.dc.html), [IA §0.3/§6.2 Đợt44](../../apps/viderse/docs/design/v1/01-ia-flows.md), [V36–V40](../../apps/viderse/docs/design/v1/08-dev-prompt.md), footer và [catalog](../../apps/viderse/docs/design/v1/artboards/README.md). Không sửa artboard; muốn lệch phải có số11-hoi-dap chốt.
## Roadmap tuần tự
- [01 · Đóng source V40, #91/#92 và merge PR](phase-01-start.md) — completed; main `60b47664`, source closure không phải deploy.
- [02 · Safari iPhone thật #93](phase-02-safari-gate.md) — chờ người dùng test trực tiếp production; thứ tự trước deploy đã được user thay thế06/11.
- [03 · Deploy main UI/backend/schema nguyên tử](phase-03-atomic-production-release.md) — đã deploy `c436aa74`, schema41; smoke auth/read đạt, chưa đủ paid-action/proposal acceptance.
- [04 · Nhóm 01 · Bàn dựng và Editor](phase-04-main-workbench.md) — pending; phụ thuộc 03.
- [05 · Nhóm 02 · Duyệt](phase-05-review.md) — pending; phụ thuộc 04.
- [06 · Nhóm 03 · Kênh](phase-06-channels.md) — pending; phụ thuộc 05.
- [07 · Nhóm 04 · Thư viện và Template](phase-07-library-template.md) — pending; phụ thuộc 06.
- [08 · Nhóm 05 · Làm video, Khám phá và Hàng chờ](phase-08-new-explore-queue.md) — pending; phụ thuộc 07.
- [09 · Nhóm 06 · Cài đặt](phase-09-settings.md) — pending; phụ thuộc 08.
- [10 · Nhóm 07 · Đăng nhập, Tạo kênh và Hệ thống](phase-10-auth-create-system.md) — pending; phụ thuộc 09.
## Hợp đồng dùng chung
[Ma trận, ảnh, controls, vòng sửa và báo cáo](acceptance-contract.md). Mỗi nhóm: mọi tweak dựng bằng dữ liệu thật; ảnh production↔artboard đúng trạng thái1440×900/390×844;0dead/0enabled chưa kiểm; overlay không tràn;slider≤2px sau font;touch≥44px. Report mỗi nhóm ngay khi đạt: bảng Màn | Đã khớp | Còn lệch, tối đa6ảnh, controls/dead/chưa kiểm, deployed SHA và URL. Gap mỗi gói một dòng ngày/SHA đã deploy/chỗ lệch.
Desktop Auth được bổ sung vì matrix cũ loại; Editor desktop ở nhóm Bàn dựng. Workflow/UK.0–UK.9/Brand là chuẩn tham chiếu tại các thành phần thực, không tạo page sản phẩm giả; Kit/MKit archive. Không lặng lẽ bỏ trạng thái khó hoặc dùng QA/mock thay authenticated production.
## Gate và rủi ro đã xác định
- #91/#92 đã chốtA/triển khai theo source; phase01 xác minh close bằng readback và commit merge, không chọn lại quyết định.
- #93 người dùng đã thay quyết định06/11: **deploy lên Mac mini rồi tự test trực tiếp production**, không đợi preview. Physical Safari vẫn pending, không miễn nghiệm thu. [Release proof](reports/release-proof.json).
- #94A **đã được người dùng chốt**: chỉ first-run Onboard/identity setup không thể dựng an toàn trên instance đã có dữ liệu được test cùng release commit trên instance biệt lập backend/dữ liệu thật, không mock; báo rõ ngoại lệ, không gọi ảnh đó production. Chuẩn bị phase01, nghiệm thu nhóm07; các state khác không được miễn.
- Script remote đã được harden: nhận artifact/main SHA/hash đã kiểm, bootout/backup/restore rehearsal/migrate/read-only SQL rồi reopen; không binary-only rollback. [Preflight](reports/release-preflight.md), [runbook](../../apps/viderse/deploy/README.md).
- FE+BE+schema cùng main SHA. Quiesce mọi writer, reconcile paid attempts, backup và restore-test; không reopen writes trước read-only gate. Sau reopen cấm restore snapshot cũ tự động.
- Baseline và interaction config hiện loopback-only/QA-only; không nới guard hoặc coi ảnh fixture là ảnh production. Capture production bằng browser thật được cấp phiên; cookie/secret ngoài repo.
- Production state/credentials/permissions/paid budget chưa được thử trong planning; các mutation machine-scope phải có approval và recovery theo action. Missing prerequisite=blocked, không reduced acceptance.
- Trước desktop navigation phải GET /api/setup/state để tránh auto-install chưa có quyền; mutation ledger phải đóng vòng đời autoFetch sau create/link/toggle. Kênh nghiệm thu#87–#89 đủ bốn người nhận, consumer snapshot và unsupported guard; Editor dùng đúng editor/v13 tests.
## Validation Log
06/11/2026,2câu quyết định: **#93:Tự test iPhone trong1ngày**, giữ KEEP; **#94:Ngoại lệ hẹp cho first-run**, cùng release/backend/dữ liệu thật trên instance biệt lập, không ảnh production giả. Options và trả lời đầy đủ ở planning validation và11-hoi-dap. Đã phản ánh phase01/02/03/10 và contract; không có Safari waiver hoặc quyền tự đổi artboard.
Kiểm format/link/coverage và red-team evidence:[Planning validation](reports/planning-validation.md). Kết quả runtime source/native thật và merge:[V40 source closure](reports/v40-source-closure.md); chưa có production acceptance.
## Red Team Review
4reviewer /4lens;6finding unique (2High,4Medium), đều có source evidence, đã xử lý. Hai blocker feasibility first-run được user chốt#94A; các correction còn lại giữ nguyên scope. Chi tiết và whole-plan sweep ở [Planning validation](reports/planning-validation.md).
## Tiêu chí đóng toàn bộ
- [ ] V40 merged,#91/#92 closed bằng evidence,#93 physical proof hợp lệ.
- [ ] Production chạy đúng main SHA với V38/V39 data cutover; binary/process/public asset/auth checks khớp.
- [ ] Bảy nhóm theo thứ tự đều đạt hợp đồng; không còn blocker/control chưa kiểm/lệch chưa có quyết định; các nhóm bị fix sau ảnh hưởng đã retest.
## Handoff
Production https://vid.aarverse.studio hiện chạy main `9871dfb09d97b9e69a32bb42092857c8d6fe0342`,schema41; PR#16 sửa cover grid2×2 và false-stale metadata lookup. [Bằng chứng bugfix/retry](reports/grid-and-media-fix.md); bản phát hành đầu ở reports/release-proof.json. Người dùng kiểm iPhone trực tiếp theo#93; các phase nghiệm thu7nhóm vẫn chưa hoàn tất. Không gọi deploy/smoke là parity1:1.
