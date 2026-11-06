# Safari iPhone thật · chờ người dùng

**Đã đổi sang test production theo yêu cầu mới của người dùng06/11:** https://vid.aarverse.studio chạy main `c436aa74`,schema41. Preview8045 đã dừng; URL/hạn phía dưới chỉ là lịch sử, không dùng cho vòng test hiện tại. Clip/model/iOS/Safari vẫn chờ; không có physical PASS. [Bằng chứng deploy](release-proof.json).

- Gate#93: KEEP, chưa có clip/device result, không deploy.
- Giao preview: 2026-11-06T15:17:54.848Z (22:17 ngày06/11 giờ Việt Nam).
- URL: http://192.168.1.13:8045/candidate-f58f95ac/qa.html — iPhone cùng LAN với Mac mini; không đăng nhập.
- Hạn dự kiến nếu truy cập được khi giao: 2026-11-07T15:17:54.848Z (22:17 ngày07/11 giờ Việt Nam). Người dùng chưa xác nhận đường LAN; nếu không tới được, sửa route và tính1ngày từ lúc nhận preview dùng được. Không coi timeout là waiver.
- Source đã merge: main `60b4766429a852f17c6e305e71b9179bbc0d632a`, PR#7. Tree `7c248a6008df723538998ee9f25b4a6d04154f95`. Controller đối chiếu359file dist sạch với Git blobs và SHA256 của359file remote; khớp. [Artifact manifest](http://192.168.1.13:8045/candidate-f58f95ac/source.json).

## Giới hạn bắt buộc
Preview dùng bundle thật nhưng GET fixture và SSE hello tổng hợp để mở UI; không phải backend/native/production proof hoặc ngoại lệ#94. Ảnh placeholder1×1 không phải chất lượng ảnh. Loader/sample guard riêng, không đổi bundle canonical; mọi mutation bị chặn. Không dùng bản candidate.html trực tiếp hoặc URL /qa cũ cho vòng test này. HTTP LAN không chứng nhận clipboard/PWA secure-context.

## Controller đã kiểm, không thay physical gate
Trình duyệt Chromium390×844: landing và4route; Main→Mô tả mở Sheet, nhập nhiều dòng, Đóng; Review→Vẽ lại mở Góp ý, nhập, submit không vẽ thật; Huỷ→Prompt→Sửa chỉ dẫn handoff còn1dialog, đáy844, không overflow; Template rename+cancel; Library menu→rename+cancel còn1dialog, không overflow. Sau sửa fixture handshake, UI báo máy rảnh và thao tác Review mở được. Một lần eval fetch ngoài guard nhận server409; không gọi đó là proof shim403. Canonical native smoke riêng đã ghi trong source closure. Không có kết quả iPhone/Safari.

## Checklist người dùng trả cùng clip
| Mục | Kết quả thiết bị thật |
|---|---|
| Nội dung đăng: mở/đóng bàn phím, nhập nhiều dòng, cuộn, đổi focus; CTA tới được | Chờ |
| Review: Góp ý, cuộn, dismiss scrim/nút rồi mở lại; không overlay/scroll lock còn sót | Chờ |
| Prompt→Sửa chỉ dẫn: một overlay tương tác, focus và bàn phím đúng | Chờ |
| Thanh địa chỉ co/mở; dọc→ngang→dọc; dock/safe area về đáy; không tràn ngang | Chờ |
| Ít nhất3vòng normal, bật Giảm chuyển động rồi lặp; Template/Library đổi tên và Huỷ | Chờ |

Cần mẫu iPhone, phiên bản iOS/Safari, hướng màn hình, clip liên tục và kết quả từng dòng. Không bấm thao tác paid/submit để kiểm backend.

## Vòng đời
Reuse service `viderse-macmini-safari-qa`,port8045,remote script `~/server/qa/viderse-safari-v40-20261106/serve-preview.py`; không restart/duplicate hoặc sửa script. Chỉ giữ tới khi gate thiết bị xong; kiểm đúng command/owner trước dừng. Native services của phiên đã dừng. Không thay production binary/schema/data.
