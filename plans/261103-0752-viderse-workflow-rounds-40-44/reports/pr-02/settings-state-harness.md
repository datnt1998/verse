# V36 · Nghiệm thu Cài đặt bằng app thật

Ngày: 04/11/2026. **Đang kiểm, chưa merge/deploy.** Báo cáo này thay bản authoring chưa chạy của delegate; giữ các kết quả lỗi trong thư mục runs, không gán thành pass.

## Phạm vi và nguồn

`tests/viderse-workflow/settings-state-audit.pw.ts`, cấu hình `WORKFLOW_APP_URL`, cookie/fixture riêng ngoài repo, admin `workflow.qa`. Bốn ca kế thừa desktop/mobile và normal/reduced; viewport1440×900 và390×844.

Ma trận PR2 có Settings `section=tools/connect/agents/accounts/storage/logs`, `conn=list/add/added/models/manual/failed`. `quota` không phải SettingsSection/tweak bắt buộc của PR2, chỉ kiểm thêm khi API có dữ liệu thật. Không thêm những probe không có trong nguồn thiết kế.

## Kết quả đã chạy

| Lượt | Qua | Lỗi | Bỏ qua có tên | Nguyên nhân / phạm vi |
|---|---:|---:|---:|---|
| [43](runs/43-settings-native-states/interaction-results.json) | 4 | 12 | 0 | Harness cũ đóng đinh model ID kỹ thuật thay nhãn UI, sai selector catalog và wording pill; đồng thời mong control recheck trên mobile vốn không có ở code lẫn MSettings. Không sửa app theo các assertion incidental này. |
| [47](runs/47-settings-native-states-corrected/interaction-results.json) | 12 | 2 | 2 | Tất cả frame/section, Tools disclosure và desktop recheck qua; Connect mobile qua. Hai desktop Connect dừng ở oracle chờ response khi về Tất cả; đang phân biệt cache/default-query với lỗi tương tác thật. Hai skip là mobile recheck, giữ owner/gate PR6. |
| [52](runs/52-settings-server-oracle/interaction-results.json) | 14 | 0 | 2 | Oracle GET thật + native radio/list/search thay cho ép GET trên cache hit; filter/search/manual-cancel PC qua. Bốn project đủ frame/connect/disclosure; recheck desktop qua. Hai skip vẫn là chức năng recheck mobile có owner PR6, không được miễn parity. Wrapper đã gỡ access QA trong finally. |

Source provider failure đã có native8/8 và focused16/16 +typecheck trong [báo cáo riêng](provider-catalog-acceptance.md). Không cộng các lượt/suite khác nhau thành một lượt Settings toàn xanh.

## Màn → hành vi đã chứng minh / còn lệch

| Màn | Đã kiểm bằng tương tác thật | Còn kiểm / owner |
|---|---|---|
| Khung Cài đặt | Sidebar PC / danh sách phone mở Tools, Kết nối AI, Trợ lý AI, Tài khoản, Lưu trữ; logs PC và deep-link phone. Nhãn/title/URL và dữ liệu live đúng, không tràn ngang. | Mobile hàng Báo lỗi là export log, không bịa một menu child-screen. |
| Kết nối AI / thêm | Tám radio, đúng ô khoá/địa chỉ, disabled→enabled→disabled, không submit key mẫu. Phone scrim/Esc trả focus và scroll. | Lượt52 đã đóng oracle catalog desktop; các ảnh/state có trong receipt tương ứng. |
| Catalog / thêm model theo tên | Catalog ChatGPT thật12model; Gợi ý/Đã chọn/Tất cả, tìm thu hẹp và xoá khôi phục, manual nhập/Huỷ qua lượt52. Phone Sheet model và scrim/Esc qua; không gọi model check/inference. | Không khẳng định inference hoạt động từ việc danh mục đọc được. |
| Provider added / failed | Ollama thật catalog rỗng; địa chỉ refused; add/remove/readback có cleanup. Lượt51 đủ hai viewport/motion, toast không che CTA, ảnh lỗi sau Sheet unmount đúng surface. | Empty catalog không phải bằng chứng có model; [receipt riêng](provider-catalog-acceptance.md). |
| Công cụ trên máy | Details mở bằng click, đóng bằng Enter; đường dẫn kỹ thuật chỉ thấy trong khối nâng cao; không install/update/cancel. PC recheck gọi đúng POST `/api/tools/check`, trả200, GET/readback cùng trạng thái. | Phone thiếu recheck ở cả artboard lẫn code. Đây là gate parity PR6 cần chốt theo quy tắc design, không coi skip là đã làm đủ chức năng. |
| Trợ lý / Tài khoản nền tảng | Nhãn assistant/provider, quota khi API có, ba platform và trạng thái live; không đăng nhập/đổi cấu hình trợ lý. | Chỉ kiểm read-only trong harness này; không gán pass cho handshake ngoài scope. |

## An toàn

- Browser chặn tất cả API writes trừ probe `POST /api/tools/check`; probe chỉ kiểm binary, không cài. Cơ chế sẵn có có thể HEAD GitHub để kiểm phiên bản mới, không gọi provider AI.
- Models/catalog cần provider ready thật. Parent chỉ tạm sao chép access còn hạn, không refresh, dưới wrapper `finally` gỡ credential QA. Lượt47 và52 đã báo gỡ thành công. Không logout hoặc sửa kho production.
- Không mock catalog/đăng nhập, không CSS-state injection, không gửi paid/model-check request. Model chọn thủ công chỉ nhập rồi Huỷ.
- `conn=added/failed` trong harness read-only tự ghi gap theo giới hạn của nó; companion provider harness đóng các trạng thái bằng CRUD QA thật. Receipt riêng cần đọc cùng, không diễn giải annotation như miễn nghiệm thu.

## Điều kiện kết thúc V36

Catalog PC và toast/Sheet đã có proof native trong lượt52/51. V36 còn phải chốt lỗi semantic draft phiên bản ảnh [#84](../../../../apps/viderse/docs/design/v1/11-hoi-dap.md), kiểm source nhúng cuối, cập nhật ledger/ảnh đặt cạnh và review trước merge. Hai global gate #83 không được bỏ; Safari/iPhone vật lý vẫn chưa có bằng chứng. Không báo workflow V37–V40 hoặc deploy full từ những lượt này.
