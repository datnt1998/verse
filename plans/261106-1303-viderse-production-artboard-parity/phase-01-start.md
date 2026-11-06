---
phase: 1
title: "Đóng source V40 và chuẩn bị nghiệm thu"
status: completed
priority: P1
effort: ""
dependencies: []
---
# Phase 01 · Đóng source V40
## Mục tiêu / nguồn
Hoàn tất phần code còn lệch, xác minh#91/#92, merge PR#7 mà không coi là đã deploy. [V40 cũ](../261103-0752-viderse-workflow-rounds-40-44/phase-07-v40-mobile-parity.md), [acceptance cũ](../261103-0752-viderse-workflow-rounds-40-44/reports/pr-06/acceptance.md), [hợp đồng mới](acceptance-contract.md). Deep mode: inventory dưới là điểm bắt đầu; re-scout từng nhóm trước sửa, không tái dùng dòng inventory obsolete như code hiện tại.
## File inventory và ownership
Controller duy nhất tích hợp code/docs; không tạo worktree mới. Root nguồn:/Users/dat/Projects/verse; checkout planning đang refactor/viderse-v40-mobile-parity@e94405c4, không phải main ainovel-cli.
| Action | Path tuyệt đối / boundary | Quy mô / ảnh hưởng |
|---|---|---|
| Đọc/sửa theo lệch thật | /Users/dat/Projects/verse/apps/viderse/web/src/screens/workbench/mobile/; screens/review/mobile/; screens/settings/; shell/ (các boundary sau cùng thuộc web/src) | Nhiều component; mobile + regression PC |
| Đọc/sửa tối thiểu theo lỗi | /Users/dat/Projects/verse/apps/viderse/internal/aiconn/; internal/imagegen/; internal/web/ | Chỉ lỗi V40 có bằng chứng; không đổi provider/lifecycle ngoài scope |
| Sửa record | /Users/dat/Projects/verse/apps/viderse/docs/design/v1/11-hoi-dap.md | #91/#92 evidence merge/source và#93 quyết định user |
| Tạo | /Users/dat/Projects/verse/plans/261106-1303-viderse-production-artboard-parity/reports/production-state-matrix.json | Full coverage production, khác inventory pending cũ |
| Đọc/tái dùng; sửa khi cần | /Users/dat/Projects/verse/tests/viderse-workflow/artboard-state-manifest.mjs; capture-baseline.mjs; *.pw.ts | Không nới loopback guard baseline; không giả QA thành production |
## Bước thực hiện
1. Đọc Workflow, IA và V36–V40, footer/copy/kit; kiểm current PR#7/head/main và hotfix production đã được tích hợp vào candidate. Không reset index/worktree có thay đổi người dùng; bản acceptance cũ có overlay SQLC/private index nên phải chứng minh candidate sạch build được từ source tracked, không phụ thuộc overlay ngoài commit.
2. Re-scout và lập danh sách V40 còn thiếu; tái hiện từng lệch trước sửa. #91: Kiểm tra lại ngay dưới mô tả mobile, POST probe thật + GET readback giống PC, không install. #92: skip→đổi route→undo/reload khôi phục beforeSkip/version bền vững, không approve/redraw/quota; legacy thiếu snapshot hoặc stale/busy báo lỗi đúng, không suy đoán.
3. Giữ#91/#92 quyết địnhA đã chốt, chỉ bổ sung evidence đóng implementation khi commit thật đã merge; không viết lại quyết định hoặc đóng lần hai để lấy task count. Cập nhật#93 đúng lựa chọn06/11: người dùng tự test iPhone theo handoff trong1ngày từ preview đúng candidate; hết hạn không tự miễn gate.
4. Tạo matrix mới đầy đủ29product boards; Auth desktop phải có row, Editor gắn nhóm01. Đọc runtime footer để lấy full values (README bị ellipsis). Planning đã chạy extractor --check và thấy DRIFT: 12sourceHashes khác, không có thay đổi rows/probes trong phép so JSON. Không sửa receipt cũ để làm xanh: regenerate vào reports mới bằng --out, đối chiếu footer và map source hiện tại, rồi kiểm snapshot mới với --check --out. Resolve trạng thái phụ thuộc và chuẩn bị entity thật/safe action ledger trước production.
4a. Chuẩn bị từ phase01 ngoại lệ#94A đã chốt: Onboard đòi kênh toàn instance rỗng; Auth setup đòi identity chưa có user, tài khoản mới không đủ. Định nghĩa instance first-run thật biệt lập cùng release commit, tách mọi kho/config/credential/process và plan capture state trước khi khởi tạo. Không reset production. Chỉ ghi verified-exception-94 cho các state đúng ngoại lệ, không production-pass. Trước authenticated desktop navigation ở mọi môi trường, GET /api/setup/state và kiểm auto-install approval theo acceptance-contract.
5. Dùng browser thật cho production capture với phiên được cấp; baseline runner hiện reject remote origins nên KHÔNG chạy WORKFLOW_APP_URL production vào nó và không gỡ guard chung. Dự kiến capture/control ledger bằng browser runtime; chỉ thêm runner production chuyên biệt khi lặp nhiều mới thực sự cần, với exact-origin allowlist/read-only default/approved mutations và secret redaction. Không thêm framework/suite trùng có sẵn.
6. Focused tests và native smoke sau sửa; review cross-module/public contracts. Merge PR#7 khi code review đạt; merge không mở gate deploy, Safari kiểm artifact/source đã build trước merge và xác nhận không đổi nội dung liên quan sau merge.
## Verification / test matrix
Tất cả lệnh từ /Users/dat/Projects/verse; là lệnh thực thi tương lai, không phải kết quả planning.
```sh
node tests/viderse-workflow/artboard-state-manifest.mjs --production --out plans/261106-1303-viderse-production-artboard-parity/reports/production-state-matrix.json
node tests/viderse-workflow/artboard-state-manifest.mjs --production --check --out plans/261106-1303-viderse-production-artboard-parity/reports/production-state-matrix.json
npm test -w apps/viderse/web -- mobile screens/new screens/queue shell screens/settings
npm run typecheck -w apps/viderse/web
npm run build -w apps/viderse/web
npm run size -w apps/viderse/web
```
Sau tích hợp: npm test -w apps/viderse/web; shared kit test/typecheck nếu chạm kit; engine suite nếu chạm editor imports; Go focused/race+Postgres riêng nếu sửa backend; go test ./apps/viderse/... khi contract chung đổi. Không rerun chỉ để xác nhận lỗi user đã báo. Cold entry≤150KiB/phone≤200KiB/CSS≤40KiB và p95 Khám phá≤400ms theo#83; dùng tool/scenario trong report cũ, không nâng ngưỡng.
| Mức | Scenario | Oracle |
|---|---|---|
| Critical | #91 probe PC/mobile | Readback kết quả thật, không tải/cài |
| Critical | #92 snapshot/route/restart/conflict | Restore đúng previous state/version, không quota/approve/draw |
| High | Same entity PC↔phone; failed/pending/reduced | Cùng function/default, no dead control/overlay lỗi |
| High | Clean source build + hotfix giữ nguyên | Artifact không cần overlay ngoài commit; playback/Editor CAS/template regression giữ |
## Checklist
- [x] Đóng lệch V40 và#91/#92 bằng source/native evidence, không thay artboard.
- [x] Hoàn chỉnh production state/data/action manifest bao gồm Auth desktop và Editor.
- [x] Candidate tracked build/smoke/review đạt và PR#7 merge main;#93 vẫn KEEP.

## Tiến độ thực thi 06/11
Đã triển khai mode `--production`:517row/29board (37Auth desktop),40probe; toàn bộ pending/null, không có production/device pass. Legacy stdout giữ nguyên; lexical và symlink/hardlink receipt guard đã smoke với `--check`. Native QA dùng app DB/identity/HOME riêng, đã tái hiện dialog giữa màn phone trước vá và kiểm Sheet sau vá, save+GET template200 revision3, close44px và tools POST probe200+GET cùng checkedAt, không install. Clean candidate native build không cần overlay SQLC; production chưa deploy.
PR#7 đã merge main `60b4766429a852f17c6e305e71b9179bbc0d632a` ngày06/11 lúc15:10:12Z. Merge tree `7c248a6008df723538998ee9f25b4a6d04154f95` khớp chính xác tree candidate đã kiểm chứng. Chi tiết kết quả cuối ở [source closure](reports/v40-source-closure.md). Phase01 hoàn thành source, không phải hoàn thành production.
## Rủi ro / rollback / bước tiếp
Credential QA machine-scope phải isolate HOME/config/app+identity DB/media/browser trước draw/logout; không chạm credential thật. Revert fix source trên main nếu cần, không roll backend về rotation/autonomy. Phase02 giữ deploy blocked đến physical proof. Mọi data/route chưa biết được đánh blocked với owner, không pass giả.
