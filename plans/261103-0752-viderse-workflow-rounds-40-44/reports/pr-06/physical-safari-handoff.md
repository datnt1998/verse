# Safari iPhone thật — gate #93 còn mở

## Route QA
- Mở Safari trên iPhone **cùng mạng LAN với Mac mini**: http://192.168.1.13:8045/qa . Không phải URL production; không đăng nhập.
- Source UI `24bf453d757d56c1353d3f533c5da066a40cbe53`; assets archive SHA256 `2a72469a7a5abb3d0985af406b1ddc78a2dd4741d0e47ca38b366d764298f43b`. Python stdlib preview chỉ phục vụ assets thật + dữ liệu mẫu rõ nhãn. Không chứa auth/env/database, không forward native API, không gọi provider hoặc cài công cụ.
- Probe tool trong preview chỉ trả dữ liệu mẫu. Proof probe thật và persistence ở [native-tools.json](native-tools.json), [native-ui-undo.json](native-ui-undo.json), không lấy preview thay backend.
- LAN HTTP200 và [Chromium3flow](safari-preview-lan-smoke.json) qua, 0pageerror. Các API phụ ngoài ba flow chưa có fixture trả404, được giữ trong receipt; không tuyên bố preview bao trùm toàn app.
- HTTP LAN không phải secure context: không dùng hạn chế clipboard/PWA của route này để kết luận app production hỏng.

## Người vận hành cần trả bằng chứng
Ghi iPhone/model, phiên bản iOS, Safari, hướng màn hình; quay màn hình liên tục và ghi qua/lỗi từng dòng. Không dùng emulation thay thiết bị.
1. Bàn dựng → Nội dung đăng: mở bàn phím, gõ vài dòng, cuộn đến cuối, đóng bằng Xong; đổi focus rồi mở lại. CTA phải tới được, không bị bàn phím che.
2. Duyệt → Vẽ lại: nhập góp ý, cuộn, đóng bàn phím rồi đóng Sheet bằng scrim; mở lại và đóng bằng nút. Chỉ một overlay tương tác; không còn scrim/scroll lock. Preview không gửi vẽ thật.
3. Duyệt → Xem prompt đã gửi → Sửa chỉ dẫn: kiểm handoff Sheet, bàn phím và focus; không hai modal chồng nhau. Không lưu dữ liệu fixture.
4. Cuộn lên/xuống làm thanh địa chỉ Safari thu/mở; xoay dọc→ngang→dọc; mở/đóng bàn phím ở mỗi hướng. Thanh điều hướng phải về đúng đáy/safe area, **không cần vuốt ngược để kéo xuống**; không overflow ngang.
5. Kiểm lặp ít nhất3vòng; bật Giảm chuyển động của iOS rồi lặp. Gửi clip và kết quả cụ thể, không chỉ “đã mở được trang”.

## Vận hành / vòng đời
- Host macmini, thư mục `~/server/qa/viderse-safari-v40-20261106`, TCP8045. Không thay binary, reverse proxy, dữ liệu hoặc cổng production.
- Service owner harness `viderse-macmini-safari-qa`; local SSH controller PID93412, remote Python PID93598 tại lúc khởi động (kiểm lại command/port trước khi dừng).
- Dừng bằng `write proc://viderse-macmini-safari-qa/kill`; nếu SSH đã mất, kiểm `ssh macmini 'lsof -nP -iTCP:8045 -sTCP:LISTEN'`, xác nhận đúng script QA rồi SIGTERM PID đó. Không pkill rộng.
- Chỉ giữ service này trong thời gian gate thiết bị thật; sau nhận proof phải dừng và dọn đúng thư mục QA. Native QA/worker guard/local preview và Chrome test không cần giữ.
- Nếu mạng điện thoại khác LAN, cần người vận hành cùng LAN; chưa cấu hình tunnel/public hostname và không tự đổi DNS/auth.

## Trạng thái
**AWAITING_PHYSICAL_IPHONE_OPERATOR — chưa merge PR#7/chưa deploy.** Người dùng đã chọn giữ gate; không xin miễn lại hoặc coi Chromium là Safari.
