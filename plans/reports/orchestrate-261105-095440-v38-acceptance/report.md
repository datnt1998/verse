# Kết quả team task — V38

Đây là kết quả điều phối và kiểm thử, không phải thông báo hoàn tất V38 hay deploy.

## Nhánh đã bàn giao và parent kiểm

| Nhánh | Role / model cấu hình | Bàn giao | Parent kiểm |
|---|---|---|---|
| UI acceptance | task / openai-codex/gpt-6.1-sol | Controller, handoff, không sửa product | 32 nhóm PC/mobile thường/giảm chuyển động qua; 0 mutation/page error. Giữ attempt1 fail và giải thích sửa probe |
| QA contracts | task / openai-codex/gpt-6.1-sol | Bốn nhóm regression và review consumer | Regression thực thi qua, gồm DB riêng; phát hiện lỗi điểm/cảnh báo của bản ảnh lịch sử. Hai test UI fail trước sửa, qua sau sửa/so sánh |

Parent giữ source, tích hợp, chạy build/test/native và xem ảnh bề mặt. Review cùng họ model, không tuyên bố đa dạng họ model.

## Bằng chứng

- [Luồng lưu thật](../../261103-0752-viderse-workflow-rounds-40-44/reports/pr-04/runs/004-instructions-save-readback/README.md): 8 lượt lưu/GET/reload PC/mobile; map khôi phục, số lượng job/step/receipt không đổi.
- [UI chỉ đọc](../../261103-0752-viderse-workflow-rounds-40-44/reports/pr-04/runs/005-independent-ui-acceptance/README.md): 32 nhóm kiểm, có JSON trước/sau sửa harness.
- [Cảnh báo hiển thị](../../261103-0752-viderse-workflow-rounds-40-44/reports/pr-04/runs/006-qa-warning-visual/README.md): fixture response override, không giả là model/backend thật.
- [Review Kiểm định](../../261103-0752-viderse-workflow-rounds-40-44/reports/pr-04/qa-instruction-review.md): phạm vi enforcement và giới hạn ngữ nghĩa model.

## Đường chạy và an toàn

Task mặc định trước đó hết quota. Người dùng chọn “Cho phép bật”; chỉ đổi cấu hình session để dùng model hiện chạy, không lưu global và không đổi provider ứng dụng. Per-call selector xung đột override nên cuối cùng đặt session override task rồi dispatch không model parameter. Hai native task handles đã nhận. `runtimes.json` và `native-dispatch.json` ghi route/giới hạn isolation; private artifact dirs là quyền theo prompt, không phải OS sandbox.

Không commit/push/deploy trong đợt team này. Không sửa normal git index hay source model người dùng. Không inference/vẽ ảnh sản phẩm; không xuất credentials. Controller browser đóng finally. Ba find của task QA từng thử internal judging nhưng thất bại authorization với 0 token/cost; không dùng chúng làm bằng chứng QA model.

## Tiếp tục

Các audit Channel/Create và Library/Template đã bàn giao; bốn task sửa source được parent tích hợp. Parent đã chạy controller bổ sung:52ca Library,6ca Channel native,12ca Channel populated fixture,19ca Template recovery qua.13state Channel thiếu dữ liệu native vẫn ghi chưa chứng minh; fixture không thay backend proof. Ledger giữ3job/27step/2receipt. Review cuối frontend/backend đã bàn giao; ba lỗi frontend có bằng chứng fail trước/qua sau sửa. Source V38 đã commit tại `acbc8c5192365520f56447756b2b2c868ffd95e3`; chưa merge, V39/V40 chưa nghiệm thu và chưa deploy full.

Ảnh đối chiếu và kết quả chạy mới: [24cặp](../../261103-0752-viderse-workflow-rounds-40-44/reports/pr-04/runs/007-current-surface-comparisons/README.md), [Library52ca](../../261103-0752-viderse-workflow-rounds-40-44/reports/pr-04/runs/008-library-final-acceptance/results.json), [Channel native](../../261103-0752-viderse-workflow-rounds-40-44/reports/pr-04/runs/009-channel-native-proof/results.json), [Channel fixtures](../../261103-0752-viderse-workflow-rounds-40-44/reports/pr-04/runs/010-populated-channel-fixtures/results.json), [Template recovery](../../261103-0752-viderse-workflow-rounds-40-44/reports/pr-04/runs/011-library-recovery-transitions/results.json). Parent đã xem ảnh code thật; không dùng các số ca này để tuyên bố pixel parity hoặc kiểm mọi tweak.

Chi phí/cache/telemetry đầy đủ không có: không quy về 0. Không có yêu cầu independent-family được chứng minh; parent arbitration chỉ áp dụng cho hai bàn giao trên, không thay acceptance toàn gói.

## Team tăng cường theo yêu cầu mới

Hai task-role tiếp tục song song, cùng route session đã được người dùng duyệt:

| Handle | Quyền ghi riêng | Đầu ra / nghiệm thu |
|---|---|---|
| `TemplateHarnessRepair` | Controller Template trong thư mục QA tạm | Sửa locator/helper có bằng chứng sai; giữ nguyên assertion của lỗi product; parent chạy |
| `TweakCoverageCompletion` | Controller extra-states và ghi chú riêng trong thư mục QA tạm | Bổ sung trạng thái import/source còn thiếu bằng response fixture có nhãn; parent chạy |

Parent giữ tích hợp source, shared-kit/typecheck/build, restart native QA và nghiệm thu cả hai controller. Hai bàn giao đã được parent chạy: Kênh44ca, Template58ca, extra11ca qua trước vòng review thị giác;0inference/download và không tiếp tục mutation fixture tới server. Hai lần lưu output Template native thật được parent thực thi riêng, GET/reload ở PC/mobile và khôi phục recipe; ledger3/27/2. Không tạo thêm worktree, không đổi index/credential nguồn.

## Review thị giác và proposal sửa song song

| Task | Bàn giao | Phân quyền |
|---|---|---|
| `ChannelVisualAcceptance` | [Review19cặp](channel-visual-acceptance.md):5layout mismatch có nguồn; các cặp sai state/data được đánh dấu riêng | Chỉ đọc ảnh/source, ghi report |
| `TemplateVisualAcceptance` | [Review25cặp+2picker](template-visual-acceptance.md):header/framing, output, tên import; không ký1:1/motion | Chỉ đọc ảnh/source, ghi report |
| `ChannelVisualRepairPatch` | Proposal F1–F3:card PC ngang/max860 token, selector mobile đúng một hàng, banner chrome không bị generation che | Chỉ ghi patch tạm, parent đọc anchors và áp source |
| `ChannelConfirmRepairPatch` | Proposal F4–F5:confirm mobile gọn, pending/error/retry và hướng dẫn menu đối thủ | Chỉ ghi patch tạm, parent áp source; fallback lỗi thô được parent đổi sang copy Việt |

Parent bổ sung sửa Template/Import và regression desktop-only Editor. [Disposition vòng sửa](../../261103-0752-viderse-workflow-rounds-40-44/reports/pr-04/runs/019-visual-review-repairs/README.md) ghi fail-before/pass-after và giới hạn. Sau tích hợp, full frontend441file/3665test qua/4skip, typecheck/Vite/native build qua. Native Editor2viewport qua; controller toàn bộ trạng thái đang chạy lại. 124cặp chỉ là mapping ảnh candidate, không phải124pixel-parity pass. Còn chọn đúng context/tweak và kiểm picker sau chuyển cảnh. Chưa có quyền suy fixture thành nativeprovider proof hay báo deploy full.
