# Viderse — Logic sản phẩm và workflow

Đây là logic chuẩn của Viderse. Đọc tài liệu này như nguồn sự thật cho hành vi. Giao diện cũ của YT Factory **không** phải chuẩn tham chiếu; chỗ nào code hiện tại làm khác thì theo tài liệu này. Cách chuyển từ code hiện tại sang nằm riêng ở [09-migration-yt-factory.md](09-migration-yt-factory.md).

Từ khoá trong code (job, step, stage, manifest, recipe…) giữ nguyên như `GLOSSARY.md` của yt-factory (bản tham chiếu khi port). Tài liệu này nói bằng từ của người dùng; bảng §2 nối hai thứ tiếng.

---

## 0. Hành trình A→Z <!-- r3 -->

Một người dùng mới đi đúng một đường, không phải biết trước có những trang nào. Mỗi bước có **một** việc chính và **một** nút chính; việc chạy nền không bắt người dùng ngồi chờ.

| Bước | Người dùng làm | Máy làm (chạy nền) | Màn |
|---|---|---|---|
| **A · Bắt đầu** | Đăng nhập ChatGPT; chọn đối thủ → **Tạo kênh** | Cài công cụ; đặt tên, vẽ nhận diện, tạo Mẫu chuẩn | Lần đầu mở (`Onboard`); kênh thứ hai trở đi: Tạo kênh (`CreateChannel`, `MCreate`) |
| **B · Tìm video** | Xem video nổi trội → chọn một hoặc nhiều | Quét đối thủ định kỳ, chấm điểm nổi trội | Khám phá (`Explore`) |
| **C · Làm video** | **Làm video** (một link, nhiều link, hoặc từ Khám phá) | Xếp hàng chờ theo luật L1–L4 | Video mới (`NewVideo`) |
| **D · Máy tự chạy** | Không cần làm gì; xem khi muốn | Trinh sát → Đạo diễn → Hoạ sĩ; Tự lấy video mỗi giờ | Bàn dựng (`Main`), Hàng chờ (`Queue`), Việc nền (`Activity`) |
| **E · Duyệt & sửa** | **Duyệt cả video & dựng**; vẽ lại ảnh chưa ưng; sửa trong editor nếu chưa đạt | Kiểm định dựng, đo, xuất | Duyệt (`Review`), Editor (`Editor`) |
| **F · Đăng** | **Tải gói đăng** → đăng YouTube → Đánh dấu đã đăng | Gợi ý xoá MP4 để lấy lại dung lượng | Bàn dựng `stage=done`, Kênh (`Channel`) |

Thiết lập (**G**: Kênh › Nguồn & tự động / Template luân phiên / Hồ sơ kênh, Thư viện, Template, Cài đặt) và hệ thống (**H**: hộp thoại, trạng thái hệ thống) chỉ mở khi cần; mặc định đã chạy được ngay sau bước A.

Thông báo đưa người dùng quay lại đúng bước khi máy cần họ: **Chờ duyệt → E**, **Cần xử lý → D/E**, **Sẵn sàng → F** (L13).

### 0.1 Đã cắt so với YT Factory

| Đã cắt | Vì sao | Thay bằng |
|---|---|---|
| Trang **Video đối thủ** (`/videos/[id]`): lấy/dán transcript, đọc bản chép lời | App tự chép lời từ âm thanh; đọc transcript không phải việc của người làm kênh | Panel **Video đã chọn** ngay trong Khám phá (`Explore` · `preview`) |
| Chế độ **Chọn video nổi trội** trong Video mới | Trùng Khám phá | Video mới chỉ còn **Một link · Nhiều link**; gợi ý video ở dưới dẫn sang Khám phá |
| Tạo kênh 4 bước (tạo → nhận diện → template → xong) | Bắt người dùng chờ và chọn thứ chưa hiểu | 3 bước: Chuẩn bị máy → Tạo kênh (mặc định sẵn) → Xong; nhận diện và Mẫu chuẩn chạy nền |
| Tab **Nhận diện** và **Cài đặt** tách nhau | Hai tab cùng là “thông tin của kênh” | Một tab **Hồ sơ kênh** |
| Danh sách **Dự án editor** (`/editor`) | Project luôn thuộc một video hoặc một template; danh sách riêng gây hiểu nhầm là phải tự tạo project | Mở editor từ video (Bàn dựng ⋯) hoặc từ template |
| **Tổng quan**, **Tất cả video** (`/clone`), chi tiết job, tour hướng dẫn | Ba trang cùng trả lời “việc gì chờ tôi” | Bàn dựng + Việc nền; Lần đầu mở + trang rỗng có hướng dẫn |
| Thiết lập chạy máy trong Cài đặt | Người dùng hỏi “sao chưa chạy” ở Hàng chờ chứ không ở Cài đặt | Hàng chờ › Quy tắc chạy |

### 0.2 Mô hình Assets tập trung <!-- r9-assets -->

Mọi thứ quyết định một video trông thế nào được **tạo một lần ở Thư viện**, rồi chỉ được **chọn lại** ở nơi khác. Mỗi tầng chỉ làm đúng việc của nó:

| Tầng | Ở đâu | Làm gì | Không làm |
|---|---|---|---|
| **1 · Kết nối** | Cài đặt | Đăng nhập gói, khoá API, model của từng trợ lý, nguồn vẽ và dự phòng, công cụ trên máy | Không chứa gì về giao diện video |
| **2 · Assets** | Thư viện | Tạo và sửa: **Prompt ảnh cảnh** (chỉ cách vẽ), **Prompt ảnh bìa** (bố cục + cách vẽ), **Video nền** (kênh nền + video lẻ), **Lớp phủ**, và **Template** = gói lắp từ các asset trên + cách ghép ảnh vào video + số ảnh, chuyển động, phụ đề, xuất | Template không có ô tự viết; muốn thứ mới thì tạo asset trong Thư viện |
| **3 · Kênh** | Kênh | Chọn: đối thủ, template luân phiên, nhịp, công tắc tự động. Dữ liệu riêng của kênh: tên, link, mô tả kênh, **quy tắc viết** (ngôn ngữ, giọng văn, đoạn cuối mô tả, tag luôn thêm). **Nhận diện** (tên, mô tả, avatar, ảnh bìa, phong cách nhận diện) app tự sinh từ đối thủ tham chiếu; người dùng chỉ sửa chữ hoặc bấm tạo lại | Không tạo asset; không có ô chỉnh phong cách ảnh <!-- r10-channel --> |
| **4 · Video** | Bàn dựng | Kết quả. Ngoại lệ ảnh chỉ ở chip **Ảnh** (prompt ảnh cảnh, prompt ảnh bìa, dùng ảnh bìa của bạn, không tạo ảnh). Nội dung đăng (tiêu đề, mô tả, tag) sửa hoặc viết lại ở tab **Nội dung** | Không có ngoại lệ cho từng ảnh; vẽ lại chỉ nhận góp ý bằng chữ |

**Chuẩn tự động hoá:** mỗi việc tự động có **một công tắc ở đúng nhà của nó**, mặc định bật, và nơi khác chỉ hiện nhãn “Tự động” kèm link về nhà.

| Việc tự động | Nhà | Mặc định |
|---|---|---|
| Nhịp mỗi ngày, Mức tự chủ (lấy video, duyệt ảnh, lỗi, video chưa đạt), luân phiên template | Kênh › Nguồn & tự động · Template luân phiên | Nhịp 5 · Cân bằng (lấy mọi video mới, duyệt ảnh luôn hỏi) | <!-- r16-agent -->
| Quét đối thủ, luồng chạy, tạm dừng hàng chờ | Hàng chờ › Quy tắc chạy | Mỗi giờ · 2 luồng |
| Bốc clip nền | Template › Nền & hiệu ứng | Tự bốc |
| Vẽ bằng nguồn dự phòng khi lỗi / hết hạn mức | Cài đặt › Trợ lý AI | Bật |
| Dựng, chấm, xuất khi đạt độ hoàn thiện ≥ 90 | Luật L8 (không có công tắc) | — |
| Đặt tên, viết mô tả kênh, vẽ avatar và ảnh bìa kênh khi tạo kênh | Kênh › Hồ sơ kênh (tạo lại từng phần) | Luôn chạy khi tạo kênh |
| Viết tiêu đề, mô tả, tag cho video (theo quy tắc viết của kênh) | Kênh › Hồ sơ kênh › Khi viết tiêu đề, mô tả, tag | Luôn chạy (step `metadata`) |

**Đã bỏ vì trùng hoặc gây xung đột** (đợt 8):
- Kênh › Hồ sơ kênh › **Phong cách ảnh chung** (ô tự viết): trùng với prompt ảnh. Thay bằng **phong cách nhận diện** app tự rút từ đối thủ tham chiếu (code: `branding.look`), chỉ đọc, và chỉ áp vào ảnh video khi template chọn prompt có sẵn **Theo nhận diện kênh** (đợt 9).
- Prompt ảnh › **Cách đặt vào video**: là cách *ghép* chứ không phải cách *vẽ*; chuyển về Template (“Cách ghép ảnh vào video”). Ba prompt “Mặc định · Toàn khung / Tách nền / Cận cảnh” gộp thành một **Theo nhận diện kênh**.
- Prompt ảnh › **Kết hợp phong cách kênh / Tự viết toàn bộ**: không cần nút gạt. Prompt có sẵn **Theo nhận diện kênh** = `blend` với phong cách nhận diện; prompt của bạn = toàn bộ cách vẽ (`custom`). Luật luôn giữ áp cho mọi prompt.
- Video mới › chip **Ảnh cảnh / Ảnh bìa**: ngoại lệ của video chỉ ở một chỗ (Bàn dựng).
- Duyệt › **Prompt riêng cho từng ảnh**: thêm lớp thứ ba, làm khó hiểu vì sao một ảnh khác các ảnh còn lại.
- Bàn dựng › menu ⋯ **Không tạo ảnh cho video này**, **Tải ảnh bìa của bạn** và thẻ **Phong cách kênh** trong Phòng vẽ: gộp vào chip Ảnh.

### 0.3 Workflow gọn — đã chốt 02/11 <a id="workflow"></a> <!-- r40 -->

Board **`Workflow`** (đầu trang Máy tính của canvas) là nguồn chuẩn cho việc sửa các màn từ đợt 40. Tóm tắt:

- **Vòng lặp:** Tìm video (Khám phá) → Làm video (hộp Làm video) → Duyệt ảnh (Duyệt) → Đăng (Bàn dựng › Sẵn sàng). Máy chạy phần giữa; chỉ ba thông báo kéo người dùng về: Chờ duyệt → Duyệt, Cần xử lý → Bàn dựng, Sẵn sàng → Bàn dựng.
- **Thanh trên (máy tính):** Bàn dựng · Duyệt · Khám phá · Kênh · **Thư viện** + nút **Làm video**. Việc nền, Cài đặt, avatar như cũ.
- **Sáu quy ước tương tác:**
  1. Video chỉ có một nhà: bấm video ở đâu cũng mở Bàn dựng với video đó.
  2. Bấm ảnh nào cũng mở **Trình xem ảnh** (một thành phần; chế độ Duyệt · Xem · Xem + Thay · Mẫu). Video chờ duyệt thì mở thẳng Duyệt tại ảnh đó.
  3. Xem/sửa chi tiết: panel phải hoặc sửa tại chỗ, không dùng hộp thoại.
  4. Hộp thoại chỉ hai kiểu: **Xác nhận** (xoá, tốn hạn mức, không hoàn tác) và **Nhập nhanh** (Làm video, Theo dõi đối thủ, Nhập template). Không hộp thoại chồng hộp thoại.
  5. Một lớp trên cùng, Esc đóng; mỗi lớp có địa chỉ (`?video=…&img=…`).
  6. Bấm gì cũng có phản hồi (mở lớp, đổi trạng thái, hoặc toast); board nào cũng về 0 nút bấm không phản hồi.
- **Đã chốt bỏ/gộp:** trang Video mới → hộp Làm video; Kênh bỏ tab Video, cột Đang áp dụng, lịch đăng đối thủ; Thư viện lên thanh trên; Bàn dựng bỏ Bản tin, ô Nhắn trợ lý mọi kênh và thẻ Kế hoạch, đề xuất thành dòng trong Chờ bạn, cột phải còn hai tab (Nội dung đăng · Thông tin). Giữ Hàng chờ và Duyệt.
- **Đã chốt giảm logic nghiệp vụ:** đề xuất của trợ lý còn 3 loại (video nổi trội chưa làm, template hay lỗi, dọn MP4 đã đăng); bỏ **Vì sao?**, bỏ học quy tắc từ việc sửa (Lưu thành quy tắc, Quy tắc đã học, Ghi chú đã học), bỏ hiệu quả sau đăng (số liệu 24 giờ/7/28 ngày, Trợ lý nhận ra, Ưu tiên kênh theo Lượt xem, khoá YouTube Data API); **Mức tự chủ** thành một công tắc “Tự duyệt khi mọi ảnh ≥ 85” (máy luôn tự thử sửa lỗi 1 lần, Tự lấy bật là lấy mọi video mới); bỏ **ảnh riêng cho một video** (chip Ảnh); bỏ **luân phiên template** (mỗi kênh một template mặc định, đổi trong hộp Làm video); bỏ **Ưu tiên cả N** (video bạn tự thêm luôn lên đầu hàng); Quy tắc chạy chỉ còn Số luồng và Nhịp kênh. §9 và các luật L3 (phần template), L9, L14 sửa theo khi tới đợt tương ứng.
- **Lộ trình:** đợt 40 Trình xem ảnh + nối nút (xong) · 41 Bàn dựng gọn (xong 03/11) · 42 Kênh + Thư viện (xong 03/11) · 43 Làm video thành hộp thoại (xong 03/11) · 44 điện thoại (xong 03/11, xem §6.2) · 45 Trình xem ảnh bản điện thoại. W3, W4, W5, W10, W11, W12, W15, §6.1 và §9.4–9.5 đã viết lại theo đợt 41–43; điện thoại theo đợt 44; chỗ nào lệch với board `Workflow` thì theo board. <!-- r41g -->


**Mục lục màn** <a id="toc"></a> <!-- r41 --> — mỗi màn một số theo vòng lặp, giống nhau giữa máy tính và điện thoại; tiêu đề board trên canvas bắt đầu bằng số này. `00` là board `Workflow`.

| Số | Màn | Máy tính | Điện thoại |
|---|---|---|---|
| 0.1 | Đăng nhập & tài khoản (Bắt đầu) | `Auth` | `MAuth` |
| 0.2 | Lần đầu mở (Bắt đầu) | `Onboard` | chỉ máy tính |
| 0.3 | Tạo kênh (Bắt đầu) | `CreateChannel` | `MCreate` |
| 1.1 | Khám phá (Tìm video) | `Explore` | `MExplore` |
| 1.2 | Chi tiết video đối thủ (Tìm video) | panel phải của 1.1 | `MCompVideo` |
| 2.1 | Làm video (Làm video) | `NewVideo` | `MNew` |
| 2.2 | Hàng chờ (Làm video) | `Queue` | `MQueue` |
| 2.3 | Việc nền & thông báo (Làm video) | `Activity` | sheet trong 4.1 |
| 3.1 | Duyệt (Duyệt & sửa) | `Review` | `MReview` |
| 3.2 | Editor (Duyệt & sửa) | `Editor` | chỉ máy tính |
| 4.1 | Bàn dựng (Bàn dựng) | `Main` | `MMain` |
| 5.1 | Kênh (Thiết lập) | `Channel` | `MChannel` |
| 5.2 | Thư viện (Thiết lập) | `Library` | `MLibrary` |
| 5.3 | Sửa template (Thiết lập) | `Template` | `MTemplate` |
| 5.4 | Cài đặt (Thiết lập) | `Settings` | `MSettings` |
| 6.1 | Hộp thoại & toast (Hệ thống) | `Dialogs` | `MSystem` |
| 6.2 | Trạng thái hệ thống (Hệ thống) | `System` | trong `MSystem` |
| 6.3 | UI Kit (Hệ thống) — **page UI Kit riêng**: UK.0 `UkOverview` · UK.1 `UkFoundation` · UK.2 `UkMotion` · UK.3 `UkControls` · UK.4 `UkInputs` · UK.5 `UkFeedback` · UK.6 `UkOverlays` · UK.7 `UkShell` · UK.8 `UkViderse` | page UI Kit | page UI Kit |
| 6.4 | Thương hiệu (Hệ thống) — UK.9 ở page UI Kit | `Brand` | — |

`Kit` / `MKit` cũ nằm ở cuối page UI Kit, nhãn **Lưu trữ** (còn các thành phần đã bỏ ngày 02/11); không dùng làm chuẩn.

## 1. Người dùng muốn gì

Một người vận hành nhiều kênh YouTube, chạy app trên máy của mình. Mỗi ngày họ cần:

1. Biết **việc nào đang chờ mình** — chỉ có hai loại: duyệt ảnh, xử lý sự cố.
2. **Đưa video vào làm** nhanh nhất có thể, từ một link hay một loạt video nổi trội.
3. **Tin rằng máy tự chạy** đúng nhịp, đúng thứ tự, không phải trông — và **giao thêm việc cho máy** khi đã tin (Mức tự chủ, §9). <!-- r16-agent -->
4. **Lấy video xong đi đăng.**

Mọi màn hình phục vụ một trong bốn việc này. Thứ không phục vụ bốn việc đó (thiết lập, thư viện, thông số) nằm sau một lớp bấm và chỉ có một nhà.

## 2. Mô hình đối tượng

| Đối tượng (UI) | Trong code | Một câu |
|---|---|---|
| **Kênh** | owner channel | Kênh YouTube của bạn. Có đối thủ, nhịp, template luân phiên, nhận diện (tự sinh), quy tắc viết |
| **Đối thủ** | competitor channel | Kênh YouTube người khác để lấy video nguồn |
| **Video** | clone job | Một video bạn làm cho một kênh, từ một video nguồn |
| **Hàng chờ** | queue + scheduler | Danh sách video chưa bắt đầu của mọi kênh, một hàng duy nhất |
| **Trợ lý** | nhóm step | 4 vai làm việc trên video (§4) |
| **Template** | template project + recipe | Cách một video trông và được dựng |
| **Prompt ảnh** | image prompt (mới) <!-- r7-prompts --> | Cách vẽ ảnh cảnh (chất liệu, màu, ánh sáng, góc máy) hoặc ảnh bìa (bố cục + cách vẽ). Asset trong Thư viện; template chọn một prompt cảnh và một prompt bìa |
| **Thư viện** | templates, image prompts, backdrop channels, overlays | Thứ dùng chung giữa các kênh |

Quan hệ: Kênh 1–n Đối thủ · Kênh 1–n Template (theo thứ tự luân phiên) · Template chọn 1 Prompt ảnh cảnh + 1 Prompt ảnh bìa (một prompt dùng được ở nhiều template) · Video thuộc đúng 1 Kênh, 1 video nguồn, 1 Template (chốt lúc bắt đầu, gồm nội dung prompt lúc đó).

## 3. Vòng đời một video

```
                   ┌─────────── Cần xử lý ◄────────┐ (lỗi, hết hạn mức, chưa đạt)
                   │   Thử lại / Sửa / Vẫn xuất     │
                   ▼                                │
 Chờ lượt ──► Đang làm ──────────────────► Chờ duyệt ──► Đang làm ──► Sẵn sàng ──► Đã đăng
 (hàng chờ)  Trinh sát → Đạo diễn → Hoạ sĩ   (bạn)     Kiểm định     (MP4)      (bạn đánh dấu)
                   │                          ▲
                   └── Duyệt theo điểm / Tự duyệt (Mức tự chủ §9.4): bỏ qua ──┘
 Tạm dừng (bạn bấm, Esc) ở bất kỳ lúc Đang làm → nằm trong "Chờ bạn" với nút Chạy tiếp
```

### 3.1 Bảng trạng thái

| Trạng thái | Màu | Ai đang giữ | Dòng phụ | Hành động chính |
|---|---|---|---|---|
| **Chờ lượt** | `#8E8E99` | Hàng chờ | “Bắt đầu khoảng 00:40” · “Ngày mai · kênh đủ nhịp 5/5” | Lên đầu |
| **Đang làm** | `#B9D0F7` | Một trợ lý | “Hoạ sĩ đang vẽ cảnh 5/6” | Tạm dừng |
| **Chờ duyệt** | `#F6CB8C` | Bạn | “7 ảnh · 42 phút trước” | Duyệt ảnh |
| **Cần xử lý** | `#FF9A5C` | Bạn | Một câu lý do | Đúng một cách sửa |
| **Tạm dừng** | `#8E8E99` | Bạn | “Bạn tạm dừng lúc 23:50 · giữ phần đã làm” | Chạy tiếp |
| **Sẵn sàng** | `#A6EDC1` | Bạn | “MP4 38:12 · 172 MB” | Tải gói đăng |
| **Đã đăng** | `#8B8B97` | — | “Đăng 26/10” | — |

- **Chờ bạn** = Chờ duyệt + Cần xử lý + Tạm dừng. Đây là con số duy nhất người dùng phải để ý; nó hiện ở danh sách Bàn dựng, số liệu kênh, chấm vàng cạnh tên kênh, badge Duyệt (chỉ phần Chờ duyệt).
- Mỗi video chỉ có **một** trạng thái và **một** hành động chính tại một thời điểm.
- Lỗi và cảnh báo dùng cam; vàng là “tới lượt bạn”; đỏ là màu nhấn (đang chọn, đang bật, focus), không làm nền nút — nút chính là nút sáng. <!-- r21-data -->
- **Tên video:** tới khi Đạo diễn viết xong tiêu đề, video mang **tiêu đề gốc** của video đối thủ — ở Hàng chờ, Bàn dựng, Kênh, thông báo. Khám phá luôn hiện tiêu đề gốc; video đã vào hàng chờ hoặc đã làm cho một kênh thì thẻ ghi kênh + trạng thái (“Midnight Tales · Chờ lượt”) thay nút Làm. <!-- r21-data -->

## 4. Bốn trợ lý

| Trợ lý | Màu chữ | Step | Việc |
|---|---|---|---|
| **Trinh sát** | `#B7CEFF` | download, backdrop, transcript | Tải video nguồn, phụ đề, bốc clip nền; xem đối thủ định kỳ |
| **Đạo diễn** | `#CDB5FF` | metadata, scenes | Tiêu đề, mô tả, tag; chia cảnh; đặt tên kênh |
| **Hoạ sĩ** | `#FFD08A` | images | Ảnh cảnh, ảnh bìa, bộ nhận diện; vẽ lại theo góp ý |
| **Kiểm định** | `#9DECC9` | build, score, export | Dựng, đo độ hoàn thiện, xuất MP4 |

Trạng thái mỗi trợ lý trên một video: **Chờ lượt · Đang chạy · Xong · Dừng · Chờ bạn**. Người dùng không bao giờ thấy tên step trừ trong “Chi tiết”.

**Model của trợ lý** <!-- r8-ai -->: Đạo diễn và Hoạ sĩ dùng model chọn ở **Cài đặt › Trợ lý AI**. Hoạ sĩ có hai phần: model viết mô tả ảnh (phải đọc được ảnh) và nguồn vẽ ảnh. Trinh sát và Kiểm định chạy bằng công cụ trên máy, không cần model.

## 5. Luật chạy

Đây là phần logic quan trọng nhất. Mọi màn hình phải nói đúng các luật này bằng lời.

**L1 · Một hành động bắt đầu: “Làm video”.** Không có “Bắt đầu ngay” tách khỏi “Thêm vào hàng chờ”. Mọi video mới vào **một hàng chờ chung**; còn luồng trống thì video đầu hàng bắt đầu ngay trong cùng giây.

**L2 · Hai mức ưu tiên.**
- **Bạn chọn từng cái** (Video mới một link, Khám phá › Làm video cho kênh, Làm mới nhất ở một đối thủ): vào **đầu hàng**, theo thứ tự bạn bấm.
- **Hàng loạt và tự lấy** (nhiều link, Khám phá › chọn nhiều, Làm tất cả của một đối thủ, Tự lấy video): vào **cuối hàng**.
- Hàng loạt có công tắc **Ưu tiên cả N** để đưa cả loạt lên đầu.

**L3 · Luân phiên kênh.** Trong phần cuối hàng, mỗi luồng rảnh lấy video của kênh kế tiếp (MT → CL → QH → MT…). Một kênh bị bỏ lượt khi đã đủ nhịp ngày hoặc không còn template dùng được; lý do hiện trên Hàng chờ.

**L4 · Nhịp kênh.** Mỗi kênh có **nhịp mỗi ngày** (1–50, mặc định 5). Nhịp chỉ giới hạn video **cuối hàng** của kênh đó (hàng loạt + tự lấy). Video bạn chọn từng cái **không** tính vào nhịp. Đếm lại lúc 00:00 giờ máy.

**L5 · Luồng.** Máy chạy N video cùng lúc (1–6, mặc định 2) và M ảnh cùng lúc (1–8, mặc định 3). Video **Chờ duyệt / Cần xử lý / Tạm dừng không giữ luồng**.

**L6 · Hạn mức ảnh.** <!-- r8-ai --> Nguồn vẽ chính hết hạn mức hoặc lỗi → thử lần lượt các nguồn dự phòng ở Cài đặt › Trợ lý AI › Hoạ sĩ (khi bật “Vẽ lại bằng nguồn khác khi lỗi”). Nguồn trả theo ảnh (khoá API) không có hạn mức ngày. Hết mọi nguồn → chỉ bước của Hoạ sĩ chờ tới giờ đặt lại; bước của trợ lý khác vẫn chạy cho video khác. Video không chuyển sang Cần xử lý vì hết hạn mức; nó ở Đang làm với dòng phụ “Chờ hạn mức ảnh · 07:00”.

**L7 · Điểm dừng duy nhất là duyệt ảnh.** Mọi ảnh mới (kể cả vẽ lại) cần duyệt, trừ khi Mức tự chủ của kênh cho phép (§9.4: **Duyệt theo điểm** khi mọi ảnh ≥ ngưỡng, hoặc **Tự duyệt hết**). Duyệt theo **cả video**, không theo từng ảnh. Sau duyệt, Kiểm định tự dựng, đo và xuất.

**L8 · Độ hoàn thiện.** Điểm ≥ 90, không mục chưa đo, không mục bắt buộc thiếu → tự xuất. Chưa đạt → Cần xử lý, hai lựa chọn: **Sửa trong editor** (chính) hoặc **Vẫn xuất**.

**L9 · Thiết lập chốt lúc bắt đầu.** Video giữ template và recipe lúc nó bắt đầu. Đổi template/kênh chỉ áp cho video bắt đầu sau đó. Muốn áp cho video đang làm thì làm trên chính video đó, và app nói trước phải làm lại phần nào và tốn bao nhiêu ảnh.

**L10 · Sửa giữa chừng.** Sửa một phần (cảnh, phụ đề, nền, ảnh) chỉ làm lại phần phụ thuộc theo step chain. Phần đang chạy thì khoá sửa kèm lý do “Đợi Hoạ sĩ vẽ xong cảnh 5”.

**L11 · Xoá và huỷ.** Chỉ làm được khi video không chạy. Hộp xác nhận nêu đúng hệ quả: xoá cái gì, giữ cái gì, có lấy lại được không.

**L12 · Một nhà cho mỗi thiết lập.**

| Thiết lập | Nhà duy nhất |
|---|---|
| Luồng video, luồng ảnh, chu kỳ xem đối thủ, tạm dừng hàng chờ | **Hàng chờ › Quy tắc chạy** |
| Nhịp mỗi ngày, **Mức tự chủ** (Tự lấy video, Duyệt ảnh, Khi gặp lỗi, Video chưa đạt), **Ưu tiên kênh**, **liên kết / bỏ liên kết đối thủ** | **Kênh › Nguồn & tự động** (lối tắt: Tạo kênh, sheet Theo dõi ở Khám phá) | <!-- r11-audit --> <!-- r16-agent -->
| Quy tắc đã học khi viết (từ việc bạn sửa) | **Kênh › Hồ sơ kênh › Khi viết…** (cùng nhà với quy tắc viết) |
| Ghi chú đã học khi vẽ (từ góp ý vẽ lại) | **Thư viện › Prompt ảnh** (cuối nội dung prompt) |
| Giới hạn ảnh trả phí mỗi ngày mà máy được tự tiêu | **Cài đặt › Kết nối AI** (theo nhà cung cấp trả theo ảnh) |
| Loại đề xuất bật/tắt, Bản tin tự mở | **Cài đặt thông báo** (popover Việc nền › bánh răng, mục Trợ lý chủ động) |
| Theo dõi / bỏ theo dõi đối thủ (kho đối thủ) | **Khám phá** |
| Thứ tự template của kênh | **Kênh › Template luân phiên** |
| Tên, link, mô tả kênh, nhận diện (tạo lại), xoá kênh | **Kênh › Hồ sơ kênh** |
| Quy tắc viết: ngôn ngữ, giọng văn, đoạn cuối mô tả, tag luôn thêm | **Kênh › Hồ sơ kênh › Khi viết tiêu đề, mô tả, tag** | <!-- r10-channel -->
| Tiêu đề, mô tả, tag của **một video** | **Bàn dựng › Nội dung** |
| Recipe template (**cách ghép ảnh vào video**, số ảnh, chuyển động, **chọn** prompt ảnh), ảnh bìa mặc định, nền, xuất | **Cài đặt template** |
| Nội dung prompt ảnh (cách vẽ; bố cục bìa) | **Thư viện › Prompt ảnh** |
| Video nền (kênh nền, video lẻ: thêm bằng link), lớp phủ | **Thư viện** | <!-- r22-sources -->
| Ngoại lệ ảnh của **một video** (prompt cảnh, prompt bìa, ảnh bìa của bạn, không tạo ảnh) | **Bàn dựng › chip Ảnh** | <!-- r9-assets -->
| Nhà cung cấp AI, đăng nhập gói, khoá API, model được chọn | **Cài đặt › Kết nối AI** | <!-- r8-ai -->
| Model của từng trợ lý, nguồn vẽ ảnh, nguồn dự phòng | **Cài đặt › Trợ lý AI** |
| Công cụ, tài khoản nền tảng (đăng nhập, trạng thái từng nền tảng), dọn dữ liệu, log | **Cài đặt** | <!-- r22-sources -->
| Thư mục lưu dự án (assets của mọi video) | **Cài đặt › Lưu trữ** | <!-- r17-storage -->

Nơi khác chỉ hiển thị giá trị và link tới nhà của nó.

**L13 · Việc nền và thông báo.** Mọi việc dài (video, tạo nhận diện, quét lịch sử, đọc kênh nền, vẽ thử ảnh bìa, dựng bản xem thử, xuất từ editor, cài công cụ, dọn dữ liệu, xuất log) là một **việc nền** cùng một dạng: ai làm, làm gì, đang ở đâu (thanh + % hoặc x/y), còn bao lâu nếu đo được, một hành động. Việc nền nằm trong popover **Việc nền** mở từ viên ở đầu trang, không chiếm màn hình.
- **Đề xuất và Bản tin không bao giờ toast** (§9). <!-- r16-agent -->
- **Toast chỉ khi:** (a) kết quả thao tác của chính bạn (viên giữa đáy, 6 giây, có Hoàn tác thì 5 giây) <!-- r35 -->; (b) việc nền xong mà cần bạn hoặc bạn đang đợi: **Chờ duyệt**, **Cần xử lý** (cam, ở lại tới khi đóng), **Sẵn sàng**, và việc khác bạn tự bắt đầu (Nhận diện kênh đã xong). **Không bao giờ toast tiến độ.**
- **Gộp:** nhiều kết quả trong khoảng 10 giây → một toast (“3 video chờ duyệt · Duyệt ảnh”). Thẻ việc nền tối đa 2, góc phải dưới, mới nhất ở dưới, thẻ cũ lùi sau (đợt 35, như Noverse); rê chuột thì dừng đếm, Esc đóng.
- **Lịch sử:** mọi thứ đã toast đều nằm trong tab **Thông báo** của popover (chưa đọc có chấm; viên ở đầu trang có chấm vàng khi có thông báo cần bạn).
- **Cửa sổ ẩn:** thông báo hệ thống cho Chờ duyệt, Cần xử lý, Sẵn sàng; số trên biểu tượng app = số Chờ bạn. Có giờ yên lặng (vẫn ghi lịch sử).
- **Cài đặt thông báo** bật/tắt theo loại × kênh hiển thị (trong app / hệ thống).

**L14 · Ba lớp ảnh: template → video → từng ảnh.** <!-- r9-assets --> <!-- r33-img --> Video lấy mọi thứ từ template. Cả video: chip **Ảnh** ở Bàn dựng (đổi prompt ảnh cảnh, prompt ảnh bìa, hoặc không tạo ảnh). Từng ảnh: khi vẽ lại ở Duyệt hoặc Editor, sửa prompt và ảnh tham chiếu của riêng ảnh đó (đợt 33).
- Cách ghép ảnh vào video thuộc template; muốn đổi thì đổi template (Làm lại từ đầu) hoặc sửa template.
- Mỗi lần vẽ lưu đúng prompt đã gửi và ảnh tham chiếu (một **phiên bản**). Sửa prompt của một ảnh chỉ áp cho lần vẽ tiếp theo của ảnh đó; không đổi template, prompt thư viện hay ảnh khác. Cùng một kiểu sửa lặp lại 2 lần → gợi ý Ghi chú đã học (§9.3), không tự học.
- Prompt được chốt lúc dùng (L9): sửa prompt trong Thư viện không đổi ảnh đã vẽ; video bắt đầu sau đó và lần vẽ lại sau đó dùng nội dung mới.
- Đổi ngoại lệ khi đã có ảnh luôn nói trước số ảnh phải vẽ lại, hạn mức còn và có phải dựng lại không. Video còn ở Chờ lượt đổi miễn phí.

**L15 · Model và nhà cung cấp.** <!-- r8-ai --> Mỗi trợ lý dùng một model đã chọn ở Kết nối AI. Model phải làm được việc: Đạo diễn cần **Chữ**, Hoạ sĩ viết mô tả cần **đọc ảnh**, nguồn vẽ cần **Vẽ ảnh**. Đổi model có hiệu lực từ bước chạy tiếp theo; bước đang chạy dùng nốt model cũ. Không xoá được nhà cung cấp đang có trợ lý dùng: app hỏi chọn model thay thế trước.

## 6. Điều hướng

### 6.1 Máy tính
Thanh trên: **Bàn dựng · Duyệt · Kênh · Khám phá · Thư viện** + nút **Làm video** (phím N) bên phải. <!-- r41g --> Mọi màn máy tính đã dùng thanh này (đợt 42). Mục đang mở sáng trên thanh; trang Sửa template sáng ở Thư viện. Nút Làm video (phím N) mở **hộp Làm video** (Nhập nhanh, W4) ngay trên màn đang mở; đóng thì về đúng chỗ cũ.

Bên trái thanh trên: logo và **bộ đổi kênh** (“Mọi kênh” với 3 avatar chồng nhau, hoặc tên một kênh). Menu gồm Mọi kênh (nhịp hôm nay và số Chờ bạn của cả 3 kênh), từng kênh (nhịp hôm nay, chấm vàng khi có việc chờ bạn), Tạo kênh. Bộ đổi kênh chỉ có ở **Bàn dựng** (đổi phạm vi danh sách). Kênh và Khám phá dùng breadcrumb (“Kênh của tôi”, “Khám phá / <đối thủ>”); đổi kênh ở cột trái trang Kênh. <!-- r5-scope --> <!-- r11-audit -->

Bên phải thanh trên:
- Viên **“2 đang chạy · 12 chờ lượt”** mở popover **Việc nền** (Đang chạy · Thông báo); cuối popover có “Mở Hàng chờ”. Có thông báo cần bạn thì viên có chấm vàng. <!-- r31-v27 -->
  - Bốn trạng thái, cùng khung 30px: **đang chạy** chấm đỏ “2 đang chạy · 12 chờ lượt” · **máy rảnh** chấm xám “Máy rảnh” · **đã tạm dừng** icon tạm dừng “Đã tạm dừng lúc 00:04 · 12 chờ lượt” · **mất kết nối** icon cam “Mất kết nối · đang thử lại”. Điện thoại dùng **nút tròn 36px** (vòng tiến độ + số việc đang chạy; rảnh chỉ icon) — đợt 36 <!-- r36 -->. Artboard `Main` / `MMain` · `jobs=running|idle|paused|offline`.
  - Mất kết nối = `GET /api/status` lỗi 2 lần liền (hoặc SSE đứt quá 10 giây). Giữ số cũ trong popover kèm giờ cập nhật cuối; nút Tạm dừng khoá tới khi có kết nối.
- Bánh răng mở **Cài đặt**, có chấm khi có việc cần xem.
- Avatar.

Trang phụ:
- **Thư viện** ở thanh trên. Trang Kênh không còn mục Thư viện ở cột trái (đợt 42).
- **Tạo kênh** (`CreateChannel`) mở từ cột trái trang Kênh và từ bộ đổi kênh; cùng màn với bước Tạo kênh của Lần đầu mở, nhưng không có bước Chuẩn bị máy. <!-- r13-create -->
- **Editor** mở từ “Mở trong editor”.

### 6.2 Điện thoại
Thanh dưới: **Bàn dựng · Duyệt · [+] Video mới · Kênh · Khám phá**.
- **Hai loại màn:** <!-- r14-mobile -->
  - **Màn gốc** (5 mục của thanh dưới: Bàn dựng, Duyệt, Kênh, Khám phá; Video mới là nút giữa): thanh trên = logo · bộ đổi kênh (chỉ Bàn dựng và Kênh; chưa có kênh thì “+ Tạo kênh”) · nút Việc nền · avatar, mọi phần tử cao 36px (đợt 36). **Thông báo trên điện thoại** nằm ở tab *Thông báo* của sheet Việc nền; chấm vàng trên nút Việc nền dẫn tới đó; dưới đó là tiêu đề lớn 24px. Có thanh dưới.
  - **Màn con** (chi tiết video, video đối thủ, Video mới, Tạo kênh, Cài đặt và các mục con): đầu trang 52px = nút Quay lại (‹) hoặc Đóng (×, cho luồng tạo mới) · tiêu đề 16px căn trái (có thể thêm dòng phụ) · thao tác bên phải. Không có thanh dưới.
- Khám phá và Duyệt không có bộ đổi kênh (giống máy tính: chỉ Bàn dựng đổi phạm vi, Kênh đổi kênh đang mở).
- Viên **Việc nền** ở mọi màn gốc mở cùng một sheet (đang chạy + “Mở hàng chờ”); ở Bàn dựng là sheet Hàng chờ có Tạm dừng, “Mở cả hàng chờ” và “Quy tắc chạy”. Cả hai dẫn tới màn **Hàng chờ** (`MQueue`). <!-- r31-v27 -->
- Cài đặt mở từ avatar, là màn con (không có thanh dưới).
- Trạng thái hệ thống của Bàn dựng và mọi hộp xác nhận có bản điện thoại dạng sheet đáy: `MSystem` · `state`. <!-- r39 -->
- Editor, Lần đầu mở chỉ có trên máy tính. Link tới các trang đó trên điện thoại hiện “Mở trên máy tính”.
- **Thư viện dùng chung** và **Sửa template** có trên điện thoại (Gói V34): `MLibrary`, `MTemplate`. Cùng dữ liệu, cùng tuỳ chọn với máy tính; bố cục theo điện thoại (tab dạng nhóm chọn, ⋯ mở sheet, mỗi mục của template là một màn con, tự lưu). Lối vào: Kênh › Template › *Mở Thư viện dùng chung*. <!-- r38 -->
- **Tạo kênh** có trên điện thoại (`MCreate`), mở từ sheet Đổi kênh: cùng trường, cùng mặc định với máy tính; nhận diện vẽ sau khi tạo nên không cần màn lớn. <!-- r13-create -->
- Trong trang Kênh, chỉ máy tính có: **Chọn nhiều** (thao tác hàng loạt) và ô **Dặn trợ lý về kênh này**. Mọi thiết lập còn lại của kênh có đủ trên điện thoại.
- **Đợt 44 (03/11/2026) — điện thoại theo workflow** <!-- r44 -->. Cùng chức năng, tuỳ chọn, nhãn, mặc định với máy tính đợt 41–43; khác ở bố cục: danh sách thả và menu ⋯ là **sheet đáy**, kết quả là **toast** phía trên thanh dưới, việc xoá/đăng xuất hỏi bằng sheet xác nhận có nói hệ quả. Mọi nút trên 13 board điện thoại đều có phản hồi (bấm thử tự động, mọi giá trị tweak).
  - **Bàn dựng** (`MMain`): bỏ Bản tin, dải Đề xuất (thành 3 dòng trong Chờ bạn), Ảnh của video, Vì sao?, ô nhắn Hoạ sĩ; khối Nhật ký đổi tên **Tiến độ**; ⋯ ở chi tiết mở sheet Làm lại / Xoá; Sao chép gói có toast.
  - **Làm video** (`MNew`): màn phủ như hộp Làm video (W4): ô dán một/nhiều link hoặc video chọn ở Khám phá; Kênh / Template mở sheet; video tự thêm lên đầu hàng chờ.
  - **Kênh** (`MChannel`): 3 tab **Nguồn & tự động · Template mặc định · Hồ sơ**; sheet Đổi template, Liên kết đối thủ; bỏ tab Video.
  - **Khám phá** (`MExplore`): chip Dài / Đăng trong mở sheet; công tắc *Chưa làm cho kênh nào* lọc thật; chọn nhiều → **Làm N video** mở `MNew`; Top 10 chọn thật; sheet thông tin đối thủ có ⋯ (Kiểm tra video mới, Mở YouTube, Liên kết với kênh, Bỏ theo dõi) và Tạm dừng / Tiếp tục quét.
  - **Video đối thủ** (`MCompVideo`): bỏ Vì sao?; sheet chọn kênh thêm thẳng vào đầu hàng chờ (kênh đó thành “Vừa thêm”), link phụ sang `MNew` để đổi template; chương mở YouTube từ mốc giờ.
  - **Duyệt** (`MReview`): bỏ Vì sao? và gợi ý *Thêm vào Ghi chú đã học*; prompt đã gửi còn 4 phần (Cảnh · Phong cách · Tham chiếu · Tránh); **Video sau** đi qua 3 video chờ duyệt; *Bỏ ảnh này* có Hoàn tác; chip góp ý chèn/gỡ câu trong ô góp ý.
  - **Hàng chờ** (`MQueue`): Quy tắc chạy còn Số luồng (ảnh vẽ cùng lúc và chu kỳ xem đối thủ do máy chọn); template từng video theo mặc định của kênh.
  - **Thư viện** (`MLibrary`): tab chia đều vừa khung (số lượng ở dòng mô tả); bấm prompt / kênh nền / lớp phủ mở sheet chi tiết; sheet Đổi tên, Thêm kênh nền, Prompt mới; ⋯ template có *Đặt làm mặc định cho kênh*.
  - **Sửa template** (`MTemplate`): *Kênh dùng làm mặc định* thay luân phiên; chọn video xem thử, gắn kênh nền mở sheet; Dựng lại, Vẽ thử, Xuất tệp, Nhân bản có phản hồi.
  - **Cài đặt** (`MSettings`, kèm `Settings`): bỏ thẻ YouTube Data API; Model · N của mọi nhà cung cấp mở danh mục; Đăng xuất hỏi xác nhận; Đổi khoá / Đổi địa chỉ mở sheet nhập; Xoá dữ liệu hỏi xác nhận kèm số GB.
  - **Hệ thống** (`MSystem`): bỏ 2 sheet trợ lý (`state=ask|unknown`, theo ô nhắn đã bỏ); câu luân phiên thành template mặc định.
  - **Đăng nhập** (`MAuth`), **Tạo kênh** (`MCreate`, kèm `CreateChannel`): nút mắt hiện mật khẩu, Đăng xuất về màn đăng nhập (giữ tên đăng nhập), Thử lại có trạng thái; ô **Theo dõi** đối thủ mới thêm dòng và tự chọn, ô trống thì báo.
  - **Nhóm chọn có viên trượt** (`.vv-seg`): đo lại khi nút đổi cỡ hoặc font tải xong (ResizeObserver + `document.fonts`), không còn lệch vài px sau khi font về. Áp mọi board.

## 7. Workflow

Mỗi workflow: mục tiêu → các bước → luật áp dụng → artboard (`Tên` · `tweak=giá trị`).

### W0 · Đăng nhập và tài khoản (dùng chung với Noverse) <a id="w0-auth"></a> <!-- r30-sso -->
- **Mục tiêu:** một tài khoản, một lần đăng nhập cho Noverse (`nov.aarverse.studio`) và Viderse (`vid.aarverse.studio`). Đăng nhập ở app này thì mở app kia không phải đăng nhập lại; đăng xuất, đổi mật khẩu hay bị thu quyền ở một app thì app kia cũng đòi đăng nhập lại.
- **Quyền:** quản trị viên cấp quyền từng app ở **Noverse › Thành viên** (ô *Noverse* · *Viderse*). Quản trị viên luôn vào được cả hai app. Viderse không có màn thành viên thứ hai.
- **Đăng nhập:** logo, “Đăng nhập”, dòng “Một tài khoản dùng cho Noverse và Viderse”, tên đăng nhập, mật khẩu (hiện/ẩn), một nút sáng, dòng “Phiên giữ 30 ngày trên trình duyệt này, dùng chung cho Noverse và Viderse.” Sai: viền cam hai ô + “Sai tên đăng nhập hoặc mật khẩu.”, giữ chữ đã gõ. Thử sai nhiều lần: nút khoá, đếm ngược. Chân trang: quên mật khẩu hay chưa có tài khoản thì nhờ quản trị viên ở Noverse › Thành viên.
- **Vì sao phải đăng nhập lại:** một khối ngắn trên form, theo lý do máy chủ trả: tự đăng xuất (xanh) · đăng xuất ở app kia · vừa đổi mật khẩu · quản trị viên đổi quyền hoặc đặt lại mật khẩu · hết hạn 30 ngày · không rõ lý do (câu chung). Câu ở [06-copy › Đăng nhập](06-copy.md#copy-auth).
- **Chưa được cấp quyền** (đúng mật khẩu nhưng thiếu quyền Viderse): giữ phiên, không phải màn đăng nhập. Tiêu đề, một câu nói rõ nhờ quản trị viên bật ô Viderse ở Noverse › Thành viên, thẻ tài khoản; *Mở Noverse* (nút sáng, chỉ khi có quyền Noverse) · *Thử lại* (nút sáng khi không có Noverse) · *Chép lời nhờ quản trị viên cấp quyền*; dưới cùng *Đăng xuất* + “Đăng xuất khỏi cả Noverse và Viderse trên trình duyệt này.”
- **Đặt mật khẩu tạm** (bắt buộc, không có Để sau): thẻ tài khoản, Mật khẩu tạm · Mật khẩu mới · Nhập lại; câu “Mật khẩu mới dùng cho cả Noverse và Viderse”; đổi xong máy chủ thu hồi mọi phiên → màn đăng nhập với khối “Mật khẩu vừa được đổi”.
- **Thiết lập lần đầu** (chỉ khi kho tài khoản chung chưa có ai): 3 bước Mã thiết lập · Tài khoản · Mã khôi phục; câu “Tài khoản quản trị tạo ở đây dùng được cho cả Noverse và Viderse”; bước cuối phải đánh dấu *Tôi đã lưu mã khôi phục* mới vào được.
- **Máy chủ không phản hồi:** hai loại. Máy chủ Viderse không trả lời (Noverse có thể vẫn dùng được) · máy chủ chạy nhưng chưa đọc được kho tài khoản chung (vẫn đang đăng nhập, Noverse có thể cũng lỗi). *Thử lại* + tự thử lại; không đá về đăng nhập, không giả thiết lập.
- **Menu tài khoản** (bấm avatar ở đầu trang): thẻ tên, vai trò, @tên đăng nhập · nhóm **Ứng dụng** (app đang mở có nhãn Đang mở, app kia có tên + địa chỉ, mở ở thẻ mới; chỉ hiện khi có quyền app kia, không vẽ trạng thái khoá) · *Tài khoản* · *Quản lý thành viên* (chỉ quản trị viên, mở Noverse › Thành viên ở thẻ mới) · *Đăng xuất* (cam) + “Khỏi cả Noverse và Viderse trên trình duyệt này”. Cài đặt › cột trái › Ở trang khác cũng có *Thành viên · ở Noverse* cho quản trị viên.
- **Trang Tài khoản** (`#/account`): thông tin tài khoản · Ứng dụng của bạn · Thành viên (quản trị viên) · Đổi mật khẩu (câu nhắc mọi thiết bị phải đăng nhập lại ở cả hai app) · Thiết bị đang đăng nhập (một phiên mỗi thiết bị, chung cho hai app; *Đăng xuất* từng cái, *Đăng xuất mọi thiết bị khác*) · Đăng xuất.
- **Điện thoại:** cùng các màn; avatar mở trang Tài khoản (nhóm Ứng dụng, Quản lý thành viên, Bảo mật › Đổi mật khẩu (sheet) · Thiết bị, Đăng xuất). Noverse: tab Thêm có thêm nhóm *Ứng dụng* (dòng Viderse) và câu dưới *Đăng xuất*.
- **Không có:** đăng nhập bằng Google/GitHub, mời qua email, quyền chi tiết theo app.
- **Artboard:** `Auth` · `screen=login|denied|password|setup|down|account|menu`, `kicked=none|self|other|password|admin|expired|unknown`, `err=none|wrong|limited`, `role=admin|member`, `other=yes|no`, `app=viderse|noverse` (menu), `step=code|account|recovery`, `down=server|accounts`, `pwerr=none|mismatch|wrong` · `MAuth` · cùng tweak, `screen=…|account`, `app=noverse` là tab Thêm của Noverse, `sheet=none|password`.

### W1 · Máy sẵn sàng (lần đầu mở)
- **Mục tiêu:** từ cài app tới máy làm được video, không cần biết công cụ nào.
- **Bước:** app tự kiểm tra và cài lần lượt 6 việc (Tải video và phụ đề · Xử lý hình và tiếng · Tách tiếng và tạo phụ đề · Dựng và kiểm tra video · Công cụ vẽ ảnh · Đăng nhập ChatGPT). Năm việc đầu tự chạy; chỉ **Đăng nhập ChatGPT** cần bạn và là nút chính duy nhất khi tới lượt. Xong → W2.
- **Lỗi:** một dòng cam với một cách sửa (Thử lại / Mở Cài đặt). Các việc khác vẫn cài tiếp.
- **Artboard:** `Onboard` · `step=setup` · `setup=running|login|failed`.

### W2 · Có kênh đầu tiên (Tạo kênh)
- **Mục tiêu:** một kênh làm được video ngay sau một màn hình.
- **Bước:**
  1. **Tạo kênh** — chọn đối thủ đã theo dõi (bắt buộc ≥ 1), hoặc dán @handle / link để **theo dõi và chọn luôn** ngay tại màn này (chưa theo dõi ai thì màn hướng dẫn làm việc này); tên và link YouTube tuỳ chọn. Khối **Cách kênh chạy** đã điền sẵn mặc định: Template = Mẫu chuẩn (tạo tự động), Nhịp 5/ngày, **Mức tự chủ Cân bằng** (lấy mọi video mới, duyệt ảnh luôn hỏi; §9.4). Nút chính **Tạo kênh**. <!-- r16-agent -->
  2. **Xong** — thẻ **Đang chạy nền**: Đạo diễn đặt tên & mô tả, Hoạ sĩ vẽ avatar + ảnh bìa, tạo Mẫu chuẩn. Không cần đợi; xong sẽ có thông báo (L13). Lối tiếp: **Làm video mới nhất của <đối thủ>** (đỏ) · Mở kênh · Khám phá.
- **Cách kênh chạy (gọn, đợt 41)** <!-- r41b -->: ba dòng, không còn Mức tự chủ ba mức. **Template** chọn ngay tại chỗ (máy tính: danh sách nổi ngay trên ô chọn, không đẩy nội dung; điện thoại: sheet đáy; không sang trang Template; mặc định **Mẫu chuẩn · gợi ý** — Viderse tạo theo nhận diện kênh mới) · **Tự duyệt ảnh** (công tắc, mặc định tắt; bật thì video có mọi ảnh từ 85 điểm tự sang dựng) · **Video mỗi ngày** (1–50, mặc định 5: số video máy tự lấy từ đối thủ mỗi ngày). Artboard `CreateChannel` / `Onboard` / `MCreate` · `tplMenu`.
- Nhận diện theo **đối thủ chọn đầu tiên** (tham chiếu): cùng ngôn ngữ, cùng thể loại; tên mới cùng nghĩa, ảnh cùng định dạng nhưng là tác phẩm mới. <!-- r10-channel -->
- **Mẫu chuẩn** tạo tự động = cách ghép **Tách nền** + prompt ảnh cảnh **Theo nhận diện kênh** + prompt bìa **Chữ hook**, 6 ảnh. Kênh mới vẽ đúng phong cách của mình mà không phải chọn gì. <!-- r11-audit -->
- Ngôn ngữ viết tiêu đề, mô tả lưu theo ngôn ngữ đoán từ đối thủ tham chiếu; đổi ở Hồ sơ kênh.
- Kênh vừa tạo, chưa có video: `Channel` · `fresh=true` (Tự lấy đang bật; lối làm video mới nhất của đối thủ — cùng video với màn Xong: video đăng gần nhất của Nocturne Stories). <!-- r21-empty --> Điện thoại: `MChannel` · `fresh=true` — số liệu 0, tab Video 0 · Nguồn 2 · Template 1, cùng ba lối (Làm video cho kênh, Tự lấy video, Chọn video nổi trội trong Khám phá). <!-- r38 -->
- Vẽ nhận diện lỗi (hết hạn mức…): tên và mô tả vẫn lưu; Hồ sơ kênh hiện khối cam “Chưa vẽ được avatar và ảnh bìa” + Thử lại + giờ tự thử lại. Kênh vẫn làm video được.
- **Đạo diễn chưa đặt được tên và mô tả** (bước Xong, `identity=director`): kênh vẫn được tạo với tên tạm **“Kênh mới”**, avatar chữ “K” nền trung tính. Thẻ đổi thành **“Một việc cần bạn”**: dòng Đạo diễn “chưa đặt được tên và mô tả kênh: <lý do>” + **Thử lại**; Hoạ sĩ “chờ Đạo diễn”; Kiểm định Xong. Thêm nút **Đặt tên ở Hồ sơ kênh**. Đặt tên tay xong thì Hoạ sĩ vẽ theo tên đó. <!-- r38 -->
- Xem/sửa nhận diện sau ở Kênh › **Hồ sơ kênh**.
- **Luật:** L4, L12, L13.
- **Artboard:** `Onboard` · `step=create|done`, `identity=running|failed`, `rivals=some|none|empty` · `CreateChannel` (cùng tweak, mở thẳng ở Tạo kênh) · `MCreate` · `step=create|done`, `rivals`, `identity` · `Channel` · `tab=settings`, `identity=ready|generating|failed` · `Channel` · `fresh=true`. <!-- r13-create --> <!-- r21-empty -->

### W3 · Tìm video đáng làm
- **Mục tiêu:** thấy video nổi trội của đối thủ mà kênh mình chưa làm.
- **Khám phá:**
  - Cột trái: đối thủ, avatar chữ cái đầu, không dùng orb.
  - Giữa: video lọc theo Nổi trội / Mới nhất / Lượt xem, độ dài, khoảng thời gian, và “Chưa làm cho kênh nào”. Mỗi thẻ hiện điểm nổi trội và trạng thái nếu đã làm cho kênh nào.
  - Phải: thông tin đối thủ, quét lịch sử, **lịch đăng hiện mọi khung đăng lặp lại** (không chỉ một khung nhiều nhất): mỗi khung có các thứ, khoảng giờ (VN / New York, đổi cả thứ khi qua nửa đêm), số video trên 30 và độ đều; bấm một khung để làm nổi các ô của nó trên bảng thứ × giờ. Trang Kênh gộp khung của mọi đối thủ liên kết, ghi đối thủ nào đăng khung đó.
- **Video đã chọn** (bấm một thẻ): cột phải đổi sang xem trước — ảnh, điểm nổi trội, chương (của video gốc), chọn kênh (kênh đã làm video này hiện trạng thái + “Mở ›” ngay trong danh sách, không chọn được), <!-- r12-small --> dòng kế hoạch và nút chính **Làm video cho <kênh>**. “← Thông tin đối thủ” để quay lại. Không có trang chi tiết riêng.
- **Chọn nhiều:** tích video → **Làm N video** (mở hộp Làm video với các video đã chọn; chọn kênh và template trong hộp, W4) · Sao chép link · Top 10/20 (chọn video chưa làm có điểm cao nhất) · Bỏ chọn. Video tự thêm lên đầu hàng chờ. <!-- r43 -->
- **Lọc (đợt 43):** chip Dài (Mọi độ dài · ≥ 8 · ≥ 20 · ≥ 40 phút), Đăng trong (7 · 30 · 90 ngày · Mọi lúc), Trạng thái (Tất cả · Chưa làm · Đã làm) và **Giới hạn quét** (500 · 1.000 · 2.000 · 5.000) là danh sách nổi; khác mặc định thì chip sáng. Menu ⋯ của đối thủ: Kiểm tra video mới, Mở trên YouTube, Liên kết với kênh… (mở Kênh), Bỏ theo dõi (hộp Xác nhận). Panel xem trước ghi template mặc định của từng kênh. <!-- r43 -->
- **Tìm tiêu đề** <!-- r28-explore -->: ô 240px trước Chọn nhiều, phím `/`; tìm trong đối thủ đang chọn, cộng với lọc đang bật, không phân biệt hoa thường và dấu; không có kết quả → khối giữa lưới + Xoá tìm.
- **Chưa có kênh / hàng chờ** (trước V4, V6): các nút làm video, chọn kênh, liên kết kênh và lọc Trạng thái khoá kèm một câu lý do; không ẩn.
- Ví dụ ở ô thêm đối thủ: một kênh Anh, một kênh Việt, một mã UC.
- **Artboard:** `Explore` · `preview`, `select`, `rivalMenu`, `scan`, `search=none|match|nomatch`, `empty` · điện thoại `MExplore` (làm ở V10) · `MCompVideo` (Chi tiết video) · `sheet=none|picker`.

- ~~**Vì sao nổi trội?**~~ đã bỏ ở máy tính (đợt 43; điện thoại đợt 44). Trước đây: cạnh điểm ở Video đã chọn (`Explore` · `preview=true`, `why=true`) và ở chi tiết video trên điện thoại (`MCompVideo` · `sheet=why`) (§9.1); video đạt ≥ ×3 mà Tự lấy không lấy thành đề xuất `hot_video` (§9.2). <!-- r16-agent -->
### W4 · Làm video <!-- r43 -->
Viết lại theo đợt 43 (03/11/2026): trang Video mới thành **hộp Làm video** (hộp thoại Nhập nhanh, §0.3 quy ước 4).
- **Mục tiêu:** từ link tới video đang làm trong một bấm, không rời màn đang mở.
- **Mở từ:** nút **Làm video** (phím N) trên thanh trên mọi màn; Khám phá › **Làm N video** (mang các video đã chọn) và **Làm video cho <kênh>** ở panel xem trước; Hàng chờ trống; Bàn dựng chưa có video.
- **Trong hộp:**
  - **Một ô dán link** (textarea): một link → thẻ xem trước (ảnh, điểm nổi trội, thời lượng, đối thủ · lượt xem · tuổi, nhãn “Có phụ đề do kênh đăng”, “Đối thủ đã liên kết với kênh”); nhiều link (mỗi dòng một link) → danh sách “N link đọc được · M đã làm cho kênh này, sẽ bỏ qua”, mỗi dòng bỏ được bằng ×. Ô trống có lối “Chọn video nổi trội ở Khám phá”.
  - Mở từ Khám phá: không có ô dán, chỉ danh sách “Video đã chọn ở Khám phá”.
  - **Kênh** (danh sách nổi: avatar, template mặc định, “hôm nay còn N lượt”) và **Template** (danh sách nổi 5 template của Thư viện). Template điền sẵn mặc định của kênh, nhãn “mặc định”; đổi sang template khác thì nhãn “**chỉ video này**”. Đổi kênh thì template về mặc định của kênh mới.
  - Một dòng **kế hoạch**: video tự thêm luôn **lên đầu hàng chờ**, không tính vào nhịp kênh (“Lên đầu hàng chờ · bắt đầu khoảng 00:40…”, nhiều video: “Cả 3 lên đầu hàng chờ, chạy lần lượt…”). Template đang lỗi thì nói thêm video sẽ dừng ở bước dựng.
  - Nút chính **Làm video** / **Làm N video** (Enter); khoá khi không có video làm được, kèm lý do. Esc / Huỷ / × / bấm nền đều đóng.
- **Chặn:** link đã làm cho kênh đang chọn → banner “<kênh> đã có video từ link này” + **Mở ở Bàn dựng**; đổi kênh thì làm được. Link lỗi → câu cam dưới ô.
- **Xong:** hộp đóng, toast “Đã thêm … vào đầu hàng chờ của <kênh>” (+ template nếu khác mặc định); viên Việc nền cập nhật.
- **Đã bỏ:** trang riêng Video mới, hai chế độ Một link / Nhiều link, công tắc **Ưu tiên cả N**, gợi ý video nổi trội, dải “Điều sẽ diễn ra”, 3 thẻ mẹo.
- **Luật:** L1, L2, L4.
- **Artboard:** `NewVideo` (nền là Bàn dựng) · `input=empty|pasted|duplicate|error|multi` · `from=topbar|explore` · `picker=none|channel|template`. Điện thoại `MNew` đổi ở đợt 44.

### W5 · Theo dõi video đang làm (Bàn dựng) <!-- r41g -->
Viết lại theo đợt 41 (Bàn dựng gọn, 03/11/2026). Phần cũ (Bản tin, nút Đề xuất, chip Ảnh, ô Nhắn trợ lý, thẻ Kế hoạch, Vì sao?, Lưu thành quy tắc, cột phải 4 tab) đã bỏ.
- **Mục tiêu:** một màn trả lời “việc gì đang chờ tôi ở mọi kênh” và “video này đang ở đâu, ai đang làm, còn bao lâu”.
- **Phạm vi (bộ đổi kênh trên đầu trang):** <!-- r5-scope -->
  - **Mọi kênh** (mặc định khi mở app) là trang tổng quan. Đầu cột trái là khối **Kênh**: mỗi kênh một dòng gồm avatar, tên, nhịp hôm nay (`3/5`) kèm thanh, “N chờ bạn · N sẵn sàng đăng”, và icon cam khi template của kênh cần sửa. Bấm một dòng thì lọc theo kênh đó.
  - Danh sách gộp cả 3 kênh, mỗi dòng có avatar kênh 20px. Đầu cột: “Video mọi kênh” + nút **Chọn**; thanh tiến độ và “6 / 13 hôm nay” ở dòng dưới.
  - **Một kênh:** chỉ video của kênh đó. Chân cột ghi Tự lấy video và **Template mặc định** của kênh (bỏ luân phiên).
  - Bàn dựng nhớ phạm vi đã chọn. Đổi phạm vi không đóng video đang mở.
- **Cột trái:** nhóm **Chờ bạn · Đang làm · Sẵn sàng đăng · Chờ lượt** (thêm **Đã đăng** khi xem một kênh); nhóm rỗng thì ẩn.
  - **Bấm dòng nào cũng mở video đó** ở cột giữa (dòng là `button`, `aria-current` cho dòng đang mở). Không còn link sang trang khác.
  - **Đề xuất của trợ lý** là dòng trong nhóm Chờ bạn, xếp sau video chờ bạn: orb trợ lý + câu đề xuất + “<Trợ lý> đề xuất · <việc>” + nút × (Bỏ qua). Chỉ ba loại (§0.3): video nổi trội chưa làm (Trinh sát), template hay lỗi (Kiểm định), dọn MP4 đã đăng (Kiểm định). Bấm dòng thì làm việc của đề xuất; × ẩn dòng (cùng đề xuất không quay lại trong 7 ngày).
  - **Chọn nhiều:** nút Chọn bật hộp kiểm trên mỗi dòng video (đề xuất không chọn được); dưới cùng hiện thanh dính **Tải gói N video** (chỉ tính video sẵn sàng đăng; chưa chọn video sẵn sàng thì nút khoá). **Xong** thoát chọn. Thay cho Chọn nhiều ở Kênh › Video (bỏ ở đợt 42).
  - Chờ lượt chỉ hiện vài video sắp chạy và link “Xem cả N trong Hàng chờ”; Đã đăng hiện video gần nhất.
- **Cột giữa:** tiêu đề + chip (trạng thái, nguồn, template, độ dài) + Tạm dừng (Esc) / Chạy tiếp + ⋯.
  - Một khối chính theo trạng thái (Phòng vẽ trực tiếp, Duyệt, Đang dựng, Cần xử lý, Thành phẩm). Đầu khối là **Tiến độ**: 2 dòng nhật ký gần nhất + “Nhật ký · N bước” mở đủ. Không còn khối Nhật ký riêng.
  - Ảnh: bấm ô ảnh mở **Trình xem ảnh**; video chờ duyệt thì sang Duyệt đúng ảnh đó. Video không có bộ ảnh (lỗi sớm, chỉ dựng từ nền…) dùng bố cục tóm tắt: ảnh bìa 16:9 + các dòng sự kiện + nút việc tiếp theo.
  - Thành phẩm: ảnh bìa/khung hình 16:9 (không kéo giãn) + 4 thẻ: độ hoàn thiện, MP4, ảnh bìa, hướng dẫn đăng.
  - Menu ⋯: Mở trong editor, Làm lại từ đầu, Xoá video (hai mục cuối mở hộp xác nhận 6.1).
  - Toast kết quả ở góc dưới phải, tự đóng sau 4 giây.
- **Cột phải — hai tab** (mặc định **Thông tin**):
  - **Thông tin:** Bốn trợ lý (một dòng mỗi trợ lý: trạng thái, việc đang làm) · **Nguồn** (ảnh video đối thủ, số liệu, Mở trên YouTube, Khám phá, đoạn đầu bản chép lời) · **Template** (một dòng, mở 5.3) · **Nền** (thanh độ phủ, clip; clip cố định không đổi được; “Xem cả N clip” mở Thư viện).
  - **Nội dung đăng:** tiêu đề, mô tả, tag (như cũ: bộ đếm theo giới hạn YouTube, nhãn “bạn đã sửa”, đoạn cuối của kênh, tag của kênh đứng đầu, Viết lại). Sao chép từng phần (nút icon cạnh Tiêu đề, Mô tả, Tag) hoặc **Sao chép cả gói đăng**; tag bỏ bằng ×, thêm bằng ô “Thêm tag” (Enter). Gói đăng tải về luôn dùng bản mới nhất.
- **Mở Bàn dựng chọn video nào:** Cần xử lý cũ nhất → Chờ duyệt cũ nhất → Đang làm mới nhất → Sẵn sàng mới nhất (trong phạm vi đang xem).
- **Ảnh của video:** không còn ngoại lệ ảnh cho riêng một video (chip Ảnh). Muốn đổi cách vẽ thì đổi template mặc định của kênh, hoặc chọn template khác khi Làm video.
- **Luật:** L6, L9, L10, L14.
- **Artboard:** `Main` · `video=nurse|mall|lastbus|ferry|hotel|hem` (video đang mở) · `stage=images|review|render|issue|quality|done|paused` (cho video nurse) · `viewer=none|view|redraw` · `panel=info|content` · `meta=pending|writing|failed|ai|edited|rewrite|history` (với `panel=content`) · `backdropShort` · `scope=all|channel` · `jobs=running|idle|paused|offline` · `select` (Chọn nhiều). Tweak cũ `agent`, `promptPop`, `panel=agents|source|backdrop` đã bỏ.
- **Điện thoại:** `MMain` chưa đổi (đợt 44). Hiện vẫn là bố cục cũ: dải thẻ kênh, sheet Đổi kênh, chi tiết video có Bản tin và “Ảnh của video”.

### W6 · Duyệt ảnh
- **Mục tiêu:** duyệt một video trong dưới một phút, không rời bàn phím; ảnh nào chưa ưng thì sửa đúng ảnh đó ngay tại chỗ. <!-- r33-img -->
- **Bước:**
  - Video chờ duyệt của mọi kênh xếp từ cũ tới mới.
  - Xem ảnh lớn và lời kể của cảnh. Ảnh chưa ưng có ba cách, nhanh tới chậm:
    1. **Góp ý → Vẽ lại (R)**: gõ một câu; Hoạ sĩ sửa phần *Cảnh* của prompt theo góp ý rồi vẽ bản mới.
    2. **Sửa prompt (E)**: sửa thẳng prompt đã gửi (từng phần), bỏ hoặc thêm ảnh tham chiếu, rồi **Vẽ lại với prompt này (⌘↵)**.
    3. **Bỏ ảnh** (cảnh dùng ảnh cảnh bên cạnh).
  - Bấm **Duyệt cả video & dựng (A)**: Kiểm định bắt đầu, và màn hình tự mở video kế tiếp.
- **Ảnh và prompt** (cột phải, 360px; 440px khi sửa; 420/520px từ 1600px): **Phiên bản** (mỗi lần vẽ là một bản, chấm xanh = bản đang dùng cho video; xem bản cũ, *Dùng bản này*) → **Ảnh tham chiếu** (ảnh gửi kèm, mỗi ảnh có vai trò: Bố cục · Nhân vật · Phong cách · Đồ vật; *Thêm* từ ảnh khác trong video, khung video gốc hoặc tải lên; tối đa theo nhà cung cấp, ChatGPT 4) → **Prompt đã gửi** (đúng văn bản gửi máy chủ, chia phần có nguồn: Cảnh · Phong cách · Tham chiếu · Ghi chú đã học · Tránh; *Chép*, *Sửa*, *Xem nguyên văn gửi máy chủ*; bản vẽ theo góp ý hiện thay đổi so với bản trước) → dòng model · kích thước · thời gian vẽ · giờ · số ảnh tốn → Lời kể (gập).
- **Sửa prompt:** mỗi phần là một ô sửa; phần đã đổi viền xanh + *Đã sửa · Hoàn tác*. Chỉ áp cho ảnh này: template, prompt thư viện và ảnh khác không đổi. Thanh dưới đổi thành “Đã sửa N chỗ…” · *Huỷ sửa (Esc)* · **Vẽ lại với prompt này** (nút chính duy nhất lúc đó; khoá khi chưa sửa gì).
- **Đang vẽ:** ảnh cũ mờ đi (blur + tối), vệt sáng màu Hoạ sĩ quét chéo và hạt nhiễu nhẹ; thẻ giữa ảnh: “Hoạ sĩ đang vẽ bản 3” · thời gian · 3 bước *Gửi prompt → Đang vẽ → Tải về* · *Huỷ lần vẽ*. Nhà cung cấp gửi ảnh xem dần thì thay nền mờ bằng ảnh xem dần (bớt mờ). Bản cũ vẫn dùng cho video tới khi bản mới xong; người dùng duyệt tiếp ảnh khác được.
- **Vẽ xong:** ảnh mới hiện ra từ mờ sang nét (≈1 giây) + viền sáng chạy một lần; dải ảnh có nhãn **MỚI**; bản mới thành bản đang dùng. *So với bản 2 (V)* để xem lại bản trước.
- **Lỗi khi vẽ:** thẻ cam “Chưa vẽ được bản 3” nói lý do thật (hết hạn mức · bị bộ lọc chặn · mất kết nối) + hướng đi (*Vẽ bằng Grok*, *Sửa prompt*); bản cũ và prompt đã sửa được giữ.
- **Chọn nhiều:** Shift + bấm ảnh trên dải → ô tích; thanh dưới “3 ảnh đã chọn” · góp ý chung · **Vẽ lại 3 ảnh · tốn 3 ảnh**. Mỗi ảnh giữ prompt riêng; góp ý ghép vào phần Cảnh của từng ảnh.
- **Khung ảnh:** giữ 16:9, **không lớn hơn kích thước gốc của ảnh** (ảnh 1280×720 hiện tối đa 1280px CSS), căn giữa; màn rộng không phóng to làm mờ ảnh. Muốn soi chi tiết thì xem 100%.
- **Khoá:** đang có ảnh vẽ lại → nút duyệt khoá với lý do.
- **Phím:** ←/→ ảnh · J/K video · R vẽ lại · E sửa prompt · ⌘↵ vẽ lại với prompt đã sửa · Esc huỷ sửa / bỏ chọn · V so sánh với bản trước · A duyệt cả video.
- **Điện thoại:** ảnh 16:9 rộng hết màn; dưới dải ảnh là **Phiên bản** và hàng **Prompt đã gửi** (dòng đầu prompt + ảnh tham chiếu tròn) mở sheet đọc đủ prompt, *Chép prompt*, **Sửa prompt rồi vẽ lại**. Sheet Vẽ lại có hai tab **Góp ý** · **Sửa prompt** (phần Cảnh + ảnh tham chiếu). Cùng hiệu ứng đang vẽ / vẽ xong.
- **Luật:** L7, L14.
- **Đã bỏ (đợt 44, theo §0.3):** **Vì sao?** cạnh điểm ảnh và gợi ý *Thêm vào Ghi chú đã học*; prompt đã gửi còn 4 phần: Cảnh · Phong cách · Tham chiếu · Tránh (câu “không có chữ” chuyển vào Tránh). Thanh tóm tắt điểm ghi “<kênh> chưa bật Tự duyệt ảnh nên vẫn hỏi bạn” thay cho mức tự chủ.
- **Artboard:** `Review` · `state=review|redrawing|reveal|failed|batch|blocked|empty`, `prompt=view|edit|raw`, `refs=view|add` · `MReview` · `state=review|prompt|feedback|edit|redrawing|reveal|empty` · `Dialogs` · `dialog=redraw-all`.

- **Điểm ảnh** trên từng ảnh khi kênh dùng Duyệt theo điểm hoặc khi có ảnh dưới 85; ảnh dưới ngưỡng viền cam, dòng tóm tắt trên dải ảnh có nút đi thẳng tới nó; cột phải có **Điểm của Kiểm định** + **Vì sao?**. Vẽ lại kèm góp ý lần thứ 2 với cùng prompt → gợi ý **Thêm vào Ghi chú đã học** (§9.3). Artboard `Review` / `MReview` · `score=true`, `state=learn`. <!-- r16-agent -->
### W7 · Xử lý sự cố
- **Mục tiêu:** mỗi sự cố có một câu giải thích và một cách sửa.

| Sự cố | Hiện ở | Cách sửa |
|---|---|---|
| Lỗi tạm thời (mạng, máy chủ AI bận) | Không hiện; app tự thử lại | — |
| Lỗi cần người (đăng nhập hết hạn, thiếu công cụ) | Cần xử lý, banner cam | Mở đúng chỗ sửa trong Cài đặt rồi quay lại |
| Hết hạn mức ảnh | Đang làm, dòng phụ “Chờ hạn mức · 07:00” | Không cần làm gì (L6) |
| Ảnh bị chặn hai lần | Ô ảnh “Bị chặn · dùng ảnh bên cạnh” | Vẽ lại với góp ý, hoặc để vậy |
| Chưa đạt độ hoàn thiện | Cần xử lý, danh sách mục chưa đạt | Sửa trong editor · Vẫn xuất |
| Nền không phủ đủ | Tab Nền, thanh vàng “Nền sẽ lặp 2:00” | Thêm clip (không chặn xuất) |

- **Artboard:** `Main` · `stage=issue|quality` · `Queue` · `state=limited`.

### W8 · Đăng video
- **Mục tiêu:** từ Sẵn sàng tới đăng YouTube với ít bước nhất.
- **Bước:**
  - Xem MP4.
  - **Tải gói đăng** (đỏ): một ZIP gồm MP4, ảnh bìa JPG, tiêu đề/mô tả/tag, ảnh cảnh.
  - Đăng trên YouTube Studio.
  - Bấm **Đánh dấu đã đăng**. App gợi ý một lần: “Xoá MP4 để lấy lại 172 MB? Giữ project, ảnh và thông tin đăng.”
- **Artboard:** `Main` · `stage=done`.

### W9 · Làm việc với nhiều video
- **Mục tiêu:** làm cùng một việc cho nhiều video trong một lần.
- **Bước:** Kênh › Video → **Chọn nhiều** → tích (Shift chọn khoảng). Thanh dưới hiện, thay ô nhắn trợ lý:
  - Thử lại (n) · Duyệt & dựng (n) · Tải gói (n) · Đánh dấu đã đăng (n).
  - Menu ⋯: Huỷ, Xoá.
  - Thoát chọn (Esc).
- **Đếm và khoá:** mỗi nút đếm số video hợp lệ trong số đã chọn; 0 thì khoá và nói vì sao.
- **Mọi kênh:** “Mọi video của 3 kênh” ở đầu cột trái trang Kênh.
- **Artboard:** `Channel` · `tab=videos` · `select=true`.

### W10 · Nhịp chạy (Hàng chờ)
- **Mục tiêu:** trả lời “vì sao video này chưa chạy, khi nào chạy”.
- **Nội dung:**
  - **Đang chạy:** trợ lý + thanh 9 đoạn.
  - **Luân phiên giữa các kênh:** kênh kế tiếp và lý do bỏ lượt.
  - **Chờ lượt:** video bạn chọn ghi “Ưu tiên · bạn chọn” ở đầu. Mỗi dòng có giờ bắt đầu dự kiến và **vì sao chưa chạy** (`waitReason`, câu ở 06-copy › Hàng chờ), lọc theo kênh, kéo thả, Lên đầu, ⋯ (Mở kênh, Xem video gốc, Bỏ khỏi hàng chờ). <!-- r31-v27 -->
  - **Quy tắc chạy (đợt 43):** chỉ còn **Luồng chạy cùng lúc** (1–6) và **Nhịp kênh mỗi ngày** (chỉ xem, sửa ở Kênh). Ảnh vẽ cùng lúc (3) và chu kỳ xem đối thủ (1 giờ) dùng mặc định, ghi một dòng. Dưới vẫn có hạn mức ảnh và Xong hôm nay. <!-- r43 -->
  - Cột Template của Chờ lượt: video Tự lấy dùng template mặc định của kênh; video bạn tự thêm có thể dùng template khác (chọn trong hộp Làm video).
- **Tạm dừng:**
  - **Tạm dừng hàng chờ** là nút trung tính. Video đang chạy làm xong bước hiện tại rồi dừng, và không video mới nào bắt đầu.
  - Khi đã tạm dừng, **Chạy tiếp** là nút chính.
- **Luật:** L1–L6.
- **Việc nền & thông báo:** popover từ viên đầu trang (L13).
- **Điện thoại** (`MQueue`, mở từ sheet Hàng chờ ở Bàn dựng hoặc sheet Việc nền): đầu trang có viên trạng thái + Tạm dừng/Chạy tiếp; Đang chạy (thẻ có thanh 9 đoạn) · Tiếp theo (lọc kênh, mỗi dòng: ảnh, kênh · template · thời lượng, giờ + lý do chờ, ⋯ mở sheet Lên đầu / Xem video gốc / Bỏ khỏi hàng chờ, có Hoàn tác) · thẻ **Quy tắc chạy** mở bottom sheet (luồng video 1–6, ảnh cùng lúc 1–8, xem đối thủ mỗi 1–24 giờ; áp dụng ngay, bớt luồng không dừng video đang chạy) · Hạn mức vẽ ảnh · Nhịp kênh (chỉ xem, “Sửa ở trang kênh”) · Xong hôm nay. Không kéo thả trên điện thoại. <!-- r31-v27 -->
- **Không hiện** câu đo sức máy (“Máy này chạy ổn 2 luồng”): chưa có phép đo (#55).
- **Artboard:** `Queue` · `state=running|paused|limited|empty` · `MQueue` · `state=…`, `sheet=none|rules|row`.
- **Artboard:** `Queue` · `state=running|paused|limited|empty` · `Activity` · `view=running|inbox|toasts|system|settings|empty` · `MMain` · `screen=jobs`.

### W11 · Thiết lập kênh <!-- r10-channel --> <!-- r11-audit --> <!-- r42 -->
Viết lại theo đợt 42 (03/11/2026): trang Kênh chỉ còn thiết lập. Đã bỏ tab Video, cột “Đang áp dụng cho kênh”, Lịch đăng của đối thủ, Mức tự chủ 3 mức, Ưu tiên kênh, Template luân phiên, Quy tắc đã học, Trợ lý nhận ra.
- **Cột trái:** Kênh của tôi (mỗi kênh: avatar, tên, “Hôm nay 3/5 · N chờ bạn”, chấm vàng khi có việc chờ) + Tạo kênh. Bấm dòng đổi kênh ngay tại trang. Không còn mục Thư viện ở đây.
- **Đầu trang:** avatar, tên, chip **Tự lấy · N video/ngày** (hoặc “Tự lấy đang tắt”), chip **Tự duyệt ảnh ≥ 85** khi bật, chip ngôn ngữ; handle · ngày tạo · số đối thủ; **Mở YouTube** (kênh chưa có link: **Liên kết YouTube** mở Hồ sơ kênh) và **Làm video cho kênh** (nút chính).
- **Số liệu:** 4 ô Video đã làm · Đã đăng · Chờ bạn · Chờ lượt; dưới là một dòng “N video của kênh nằm ở Bàn dựng…” + **Xem trong Bàn dựng** (mở Bàn dựng lọc theo kênh). Danh sách video, Chọn nhiều, Tải gói đã về Bàn dựng (W5).
- **Ba tab:** **Nguồn & tự động** (mặc định) · **Template** · **Hồ sơ kênh**. Giống nhau trên máy tính và điện thoại (điện thoại đổi ở đợt 44).
- **Nguồn & tự động:**
  - Đối thủ liên kết: mỗi đối thủ có **Làm video mới nhất** · **Chọn video nổi trội** (Khám phá) · Khám phá · **Bỏ liên kết**. Đối thủ đầu tiên có nhãn **Tham chiếu nhận diện**.
  - **Bỏ liên kết** luôn hỏi xác nhận (Xác nhận, 6.1): video đã làm và đang chờ lượt giữ nguyên; Tự lấy không lấy thêm; đối thủ kế tiếp thành tham chiếu; bỏ đối thủ cuối thì Tự lấy tạm dừng.
  - **Liên kết đối thủ** chọn từ đối thủ đang theo dõi (danh sách nổi, một đối thủ dùng được cho nhiều kênh); nút **Liên kết N**.
  - Khối **Tự động** ba dòng:
    - **Tự lấy video mới** (công tắc): lấy mọi video mới của các đối thủ liên kết, xếp cuối hàng theo nhịp kênh.
    - **Video mỗi ngày** (1–20, bước chỉnh) + “Hôm nay 3/5 · lần xem gần nhất” + **Xem đối thủ ngay**. Video bạn tự thêm luôn làm trước, không tính vào nhịp.
    - **Tự duyệt ảnh** (công tắc, ngưỡng cố định 85): mọi ảnh ≥ 85 thì video tự sang dựng; một ảnh dưới 85 thì cả video vẫn chờ duyệt. Bật/tắt không hỏi xác nhận, có toast; áp cho ảnh vẽ từ lúc đổi.
    - Ghi chú cố định: gặp lỗi máy tự thử sửa 1 lần rồi mới báo; không bao giờ tự đăng, xoá, đổi template, bỏ liên kết đối thủ.
- **Template:** một thẻ **Template mặc định** (ảnh, tên, công thức, sức khoẻ) + **Đổi template** (danh sách nổi các template trong Thư viện + “Xem trong Thư viện”) + **Sửa template** (5.3). Template đang lỗi: banner cam “video mới sẽ dừng ở bước dựng”. Ghi chú: mọi video của kênh (kể cả Tự lấy) dùng template này; muốn một video dùng template khác thì chọn trong hộp Làm video; đổi mặc định áp cho video bắt đầu từ lúc đổi.
- **Hồ sơ kênh:**
  - **Nhận diện:** ảnh bìa kênh (vùng hiện trên điện thoại 1546×423), avatar, Vẽ lại avatar · 1 ảnh, Vẽ lại ảnh bìa kênh · 1 ảnh, Đặt tên khác, Tải avatar + ảnh bìa kênh. Đang vẽ: ảnh cũ giữ, nút vẽ lại khoá. Lỗi: khối cam + Thử lại.
  - **Mô tả kênh** (≤ 2.000 ký tự), **Phong cách nhận diện** (chỉ đọc) + Góp ý cho lần tạo lại + **Tạo lại cả bộ · 2 ảnh**.
  - **Khi viết tiêu đề, mô tả, tag:** Ngôn ngữ (Tiếng Anh / Tiếng Việt / Theo đối thủ), Giọng văn, Đoạn cuối mô tả (≤ 1.000), Tag luôn thêm (≤ 10, bỏ bằng ×, thêm bằng ô “Tag mới” + Enter). Quy tắc viết chỉ sửa tay ở đây (không còn học từ việc sửa).
  - Thông tin kênh (tên, link YouTube), lối sang Nguồn & tự động, Template mặc định, Cài đặt › Trợ lý AI.
  - **Xoá kênh:** khoá khi kênh có video đang làm (“Xem N video đang làm” mở Bàn dựng); không có thì mở hộp Xác nhận gõ tên kênh (6.1 `delete-channel`).
- **Ô Dặn trợ lý về kênh này** (dưới trang): chọn trợ lý bằng danh sách nổi mở lên (Trinh sát · Đạo diễn · Hoạ sĩ · Kiểm định), gửi thì có toast; áp cho video tiếp theo của kênh.
- **Dữ liệu mẫu theo kênh** (khớp Bàn dựng): Midnight Tales 18 video · 9 đã đăng · 2 chờ bạn · 4 chờ lượt · 3 đối thủ · Mẫu chuẩn · Tách nền; Chuyện Lạ Đêm Khuya 8 · 1 · 1 · 5 · Hồ Sơ Bí Ẩn · Truyện kể đêm; Quiet Hours 5 · 1 · 1 · 3 · Slow Rooms · Mẫu chuẩn · Tách nền.
- **Artboard:** `Channel` · `tab=sources|templates|settings` · `channel=MT|CL|QH` · `autoApprove` · `tplMenu` · `identity=ready|generating|failed` · `link=none|picker|empty` · `fresh` (After Dark Stories, kênh mới chưa có video). Tweak cũ `tab=videos`, `filter`, `select`, `autonomy` đã bỏ. `MChannel` chưa đổi (đợt 44).

### W12 · Template và thư viện
- **Xem ảnh mẫu (đợt 41):** mỗi thẻ prompt ảnh cảnh và ảnh bìa có nút **mắt** (góc trên trái) mở Trình xem ảnh chế độ **Mẫu**: ảnh mẫu, nội dung prompt, template đang dùng; nút **Dùng prompt này** (đang dùng thì ghi “Template đang dùng”). Ô “Dựng từ video đã làm” (cột Xem thử) và ô video của **Vẽ thử ảnh bìa** mở danh sách nổi, không đẩy nội dung. <!-- r41g -->
- **Template tự lưu.** Mỗi thay đổi làm bản xem thử thành “cũ”. **Dựng lại bản xem thử** là nút chính duy nhất và chỉ hiện khi bản xem thử đã cũ.
- **Vẽ thử ảnh bìa** dùng hạn mức ChatGPT nên là nút trung tính, có ghi giá: “1 ảnh”.
- **Template mặc định của kênh** (đợt 42): mỗi kênh đúng một template mặc định, chọn ở Kênh › Template; hộp Làm video cho chọn template khác cho từng video. Thư viện ghi “Mặc định của <kênh>” hoặc “Chưa kênh nào dùng làm mặc định”; Sửa template có mục **Kênh dùng làm mặc định** (mỗi dòng **Đổi ở Kênh**) và nút **Đặt làm mặc định cho kênh…** (mở Kênh). Không còn danh sách luân phiên. <!-- r42 -->
- **Template đang là mặc định của kênh:** không xoá được. Bị chặn kèm lý do “Đang là template mặc định của 2 kênh. Đổi template mặc định ở Kênh › Template trước rồi xoá.”
- **Video nền** (Thư viện, trước là Kênh nền) <!-- r22-sources --> — kho video chạy phía sau ảnh cảnh, dùng chung cho mọi template. Hai nhóm:
  - **Kênh nền:** kênh Twitch (clip, highlight, phát lại) hoặc trang Bilibili (video đã đăng). App đọc danh sách video và lấy clip mới theo thời gian; template bốc clip từ đây.
  - **Video lẻ:** một video cụ thể (≤ 10 phút đoạn dùng) kèm **đoạn dùng** (Từ–Đến). Gắn vào template thành **clip cố định** — luôn có mặt trong nền, không bị đổi ngẫu nhiên. Đổi đoạn ở Thư viện thì mọi template đang gắn đổi theo.
  - **Một ô dán link** cho cả hai: app tự nhận ra link kênh hay link video và của nền tảng nào, ghi ngay dưới ô (“Link kênh Twitch · đang đọc…”, “Link video Bilibili · lưu thành video lẻ”). Link `b23.tv` được mở một lần để biết đích.
  - **Nền tảng chưa hỗ trợ** (ví dụ Douyin): báo “Chưa hỗ trợ <nền tảng>. App lấy nguồn nền từ Twitch và Bilibili.” + nút **Gửi yêu cầu hỗ trợ <nền tảng>** (một lần; sau đó “Đã gửi yêu cầu”). Nền tảng mới được hỗ trợ thì chỉ thêm vào danh sách lọc và một thẻ ở Cài đặt › Tài khoản nền tảng, không đổi bố cục.
  - **Bố cục (đợt 25)** <!-- r25-backdrops -->: ô dán link ở trên; dưới là **danh sách nguồn** bên trái và **video của nguồn đang chọn** bên phải.
    - Danh sách nguồn: nhóm **Kênh nền** chia theo nền tảng (Twitch 3 · Bilibili 1), mỗi dòng có tên, số video, số template đang gắn; kênh bị chặn có icon đồng hồ cam. Nhóm **Video lẻ** là một dòng “Tất cả video lẻ”. Nền tảng mới chỉ thêm một nhóm.
    - Chọn một kênh: đầu trang có tên, nền tảng, link kênh, lần cập nhật, **Làm mới**, ⋯; dòng **Dùng ở** liệt kê template và cách lấy (“Mẫu chuẩn · Tách nền · chỉ 4 video đã chọn”), bấm để mở template. Lọc loại (Tất cả · Clip · Highlight · Phát lại, có số; loại 0 video mờ), sắp xếp Mới nhất. **Lưới 4 cột** ảnh 16:9: loại, thời lượng, tên 2 dòng, “Clip · 3 ngày trước”. Video được template chọn có nhãn tick “Mẫu chuẩn”. Video không còn xem được mờ đi, ghi “bỏ qua khi bốc clip”. **Hiện thêm 24 video** ở cuối.
    - Kênh mới thêm: banner “Đang đọc danh sách video”, lưới hiện dần (ô xương), gắn vào template được ngay.
    - Kênh bị chặn: banner trong trang kênh “Bilibili tạm chặn tới 02:10 · đã đọc 300/1.240 video” + **Tài khoản nền tảng**; Làm mới khoá tới giờ đó. Không còn banner chung cho cả tab.
    - Video lẻ: lưới thẻ (ảnh, tên, nền tảng · thời lượng · chất lượng, **Đoạn dùng** + **Chọn đoạn khác**, “Clip cố định ở <template>”); video đang tải có thanh tiến độ.
  - Bỏ khỏi thư viện bị chặn khi còn template gắn (“gỡ khỏi <template> trước”).
- **Cài đặt template › Nền & hiệu ứng** chỉ **chọn** từ Video nền (đợt 25 gộp lại cho dễ hiểu) <!-- r25-backdrops -->:
  - **Lấy nền từ** — một danh sách cho cả kênh và video lẻ. Kênh có nhãn **Ngẫu nhiên** (“Twitch · từ mọi video · 312 video · Clip, Highlight” hoặc “chỉ từ 4 video đã chọn”) và nút **Chọn video**; video lẻ có nhãn **Luôn có** (“Bilibili · video · đoạn 0:00–1:30”). Mỗi dòng có vạch màu khớp thanh bên dưới; nút ✕ bỏ khỏi template (nguồn vẫn ở Thư viện). Chưa có gì: ô trống nói hai cách dùng + nút chính.
  - **Một nút “Gắn kênh hoặc video”** mở hộp thoại: trái là Video nền trong thư viện (Kênh nền, Video lẻ; cái đã gắn có nhãn “Đã gắn”; ô dán link mới ở trên cùng), phải là nguồn đang chọn. Kênh: chọn **Ngẫu nhiên từ mọi video** hoặc **Chỉ từ video tôi chọn** (bấm ảnh để chọn), bật/tắt loại video (giữ ít nhất một), xem 8 video. Video lẻ: ảnh lớn + đoạn dùng (đổi ở Thư viện). Chân hộp thoại tóm tắt một dòng + **Gắn vào template** / **Lưu**; chế độ chọn mà chưa chọn video nào thì khoá nút và nói vì sao. **Chọn video** trên một dòng mở cùng hộp thoại ở kênh đó.
  - **Nền của một video** — thanh ngang cho video ví dụ 42 phút: phần **Luôn có** (clip cố định) + phần của từng kênh (≈ số clip) + phần **Lặp lại** nếu không đủ. Cách chia như code: clip cố định trước; phần còn lại chia vòng giữa các kênh, kênh hết video được chọn thì nhường kênh khác, mọi kênh hết mới lặp video. Quá 40 clip: dòng cam + nút **Tăng mỗi clip lên N giây**.
  - **Cách cắt clip** gập sẵn (tóm tắt “Tự phủ kín · mỗi clip 60 giây”); mở ra là Tự phủ kín / Tự đặt số clip và độ dài mỗi clip.
  - Không còn khối “Clip cố định” và hai bảng chọn riêng.
- **Nền của một video** (Bàn dựng › Nền):
  - Thanh độ phủ.
  - Mỗi clip có Đổi ngẫu nhiên · Chọn đoạn khác · Bỏ.
  - Đổi ngẫu nhiên tất cả giữ clip bạn tự chọn.
  - Khoá khi Kiểm định đang dựng.
- **Prompt ảnh (kho trong Thư viện)** <!-- r7-prompts -->
  - Template ghép asset: **Cách ghép ảnh vào video** (Toàn khung / Tách nền / Cận cảnh, chọn bằng ô có ảnh) + **Prompt ảnh cảnh** + **Prompt ảnh bìa** từ kho. Không có ô “Mô tả phong cách ảnh”. <!-- r9-assets -->
  - Prompt ảnh cảnh gồm: tên · nội dung (≤ 2.000 ký tự, chỉ cách vẽ: chất liệu, màu, ánh sáng, góc máy) · ảnh mẫu. Dùng được cho mọi cách ghép.
  - Prompt ảnh bìa gồm: tên · **bố cục bìa** (Ảnh sạch, Chữ hook, Khoanh tròn + mũi tên, Chữ hook + mũi tên, Trước / sau, Cận mặt) · nội dung · ảnh mẫu.
  - Nội dung từng cảnh và chữ hook vẫn do Đạo diễn viết từ câu chuyện. **Luật luôn giữ** (ảnh cảnh không chữ, trang phục kín đáo, nhân vật đủ sáng rõ mặt) áp cho mọi prompt và hiện trong trang sửa prompt.
  - **Có sẵn** (của Viderse): 1 prompt ảnh cảnh **Theo nhận diện kênh** (vẽ theo phong cách nhận diện của kênh đang làm video, nên mỗi kênh một vẻ; `blend` + `branding.look`) và 6 prompt ảnh bìa (6 bố cục). Không sửa, không xoá; muốn sửa thì **Nhân bản để sửa**.
  - **Của bạn:** tạo mới, nhân bản, sửa, vẽ thử (1 ảnh trong hạn mức, chạy nền, ảnh vẽ thử được giữ làm ảnh mẫu), nhập/xuất tệp `.json`.
  - Sửa prompt đang dùng: cảnh báo “Lưu thì N template đổi theo”; áp dụng cho video bắt đầu sau khi lưu (L9). Không đổi được loại (cảnh/bìa) khi đang dùng. **Xoá** bị chặn khi có template dùng: “Đang dùng ở <template>. Đổi prompt ở template đó trước rồi xoá.”
  - Trong Cài đặt template: ô Cách ghép (3 ô ảnh), lưới chọn prompt 4 cột (ảnh mẫu, tên, dấu ✓), ô “Prompt mới” dẫn sang kho, và khung tóm tắt prompt đang chọn (nội dung, template khác cũng dùng, link “Sửa trong kho”). Đổi gì cũng làm bản xem thử thành cũ.
- **Thư viện (đợt 42):** ở thanh trên, không còn cột trái (kênh + mục thư viện trùng tab); nội dung rộng hết màn. Menu ⋯ của template có **Đặt làm mặc định cho kênh…**. Ô “Vẽ thử với” (sửa prompt) và Sắp xếp video nền là danh sách nổi; menu ⋯ của kênh nền có Mở kênh và Bỏ khỏi thư viện (khoá khi còn template gắn). <!-- r42 -->
- **Artboard:** `Library` · `tab=templates|prompts|backdrops|overlays`, `prompt=list|mine|builtin|new` (sửa bìa: bấm “Ảnh bìa” ở Dùng cho), `menu`, `nguon=channels|videos|adding|addvideo|unsupported`, `kenh=slowroads|nightdrivefm|bilibili|lofilanes` (với `tab=backdrops`) · `Template` · `section=images|cover|backdrop|output`, `picker=none|add|choose|video` (với `section=backdrop`), `preview`, `fewClips` · `Main` · `panel=backdrop`.

### W13 · Cài đặt máy
Sáu mục:
- **Công cụ trên máy:** tên theo việc làm. Mỗi công cụ có một nhãn sẵn sàng; chỉ khi có vấn đề mới hiện một nút.
- **Kết nối AI** <!-- r8-ai --> (giống Noverse): danh sách nhà cung cấp, mỗi thẻ có loại kết nối (Đăng nhập gói · Khoá API · Trên máy này), việc làm được (Chữ · Vẽ ảnh), hạn mức hoặc chi phí, và ba nút **Model · N** / **Thử kết nối** / Đăng xuất · Đổi khoá · Đổi địa chỉ.
  - **Model · N** mở danh mục lấy thẳng từ nhà cung cấp: tìm, lọc Đã chọn / Gợi ý / Tất cả, mỗi model có nhãn Chữ · Chữ · đọc ảnh · Vẽ ảnh, ngữ cảnh và giá. “Không thấy model? Thêm theo tên” gọi thử model rồi mới thêm.
  - **Thêm nhà cung cấp:** OpenAI, Gemini, OpenRouter, DeepSeek, xAI, Anthropic, Ollama, Tương thích OpenAI. Dán khoá → **Kết nối và lấy model** → thẻ mới lên đầu danh sách, nhắc chọn ít nhất một model. Khoá chỉ hiện dạng che.
  - ~~**Số liệu YouTube** (thẻ YouTube Data API)~~ — **đã bỏ** ở đợt 44 cùng hiệu quả sau đăng (§0.3); `Settings`/`MSettings` không còn tweak `ytkey`.
  - Lỗi khoá: thẻ viền cam, câu nói rõ nhà cung cấp từ chối khoá, nút Đổi khoá · Thử lại · Thêm theo tên.
  - Thay đổi chưa lưu: thanh dính ở đầu mục (Huỷ · Lưu).
- **Trợ lý AI:** bốn trợ lý. Trinh sát, Kiểm định ghi “Không dùng model”. Đạo diễn: model, mức suy nghĩ (Tự động/Thấp/Vừa/Cao), chuỗi dự phòng. Hoạ sĩ: model viết mô tả ảnh (mờ model không đọc được ảnh), **Vẽ ảnh bằng** (ChatGPT · Grok · Gemini…), công tắc “Vẽ lại bằng nguồn khác khi lỗi” và thứ tự nguồn dự phòng. Ô chọn model chỉ liệt kê model đã chọn ở Kết nối AI; model không làm được việc đó hiện mờ kèm lý do.
- **Tài khoản nền tảng** (trước là Tài khoản phụ) <!-- r22-sources --> — mỗi nền tảng app lấy video từ đó một thẻ cùng mẫu: tên, dùng cho việc gì, trạng thái (Sẵn sàng · Tạm chặn tới HH:MM · Cần cookie), chất lượng tải, số nguồn đang dùng + link (YouTube → Khám phá; Twitch, Bilibili → Thư viện › Video nền), phần đăng nhập.
  - **YouTube:** video đối thủ (video gốc 360p + phụ đề). Cookie lấy từ hồ sơ trình duyệt. Khi YouTube báo “xác nhận bạn không phải bot”: thẻ viền cam, banner nói video nào bị ảnh hưởng, nút **Thử lại**; chấm cam ở mục bên trái.
  - **Twitch:** nguồn nền 480p; không cần đăng nhập.
  - **Bilibili:** nguồn nền 1080p; đăng nhập tài khoản phụ tuỳ chọn. Bị chặn: “Tạm chặn tới 02:10”, banner nói app tự quét lại, không cần làm gì (thời gian chờ tăng dần 15 → 120 phút theo số lần bị chặn liên tiếp).
    - **Đăng nhập bằng mã QR** (cách chính, đợt 24) <!-- r24-biliqr -->: nút **Đăng nhập bằng mã QR** mở hộp thoại có mã, ba bước (mở app 哔哩哔哩 bản Trung Quốc → quét → xác nhận) và dòng trạng thái. Mã sống 3 phút, có thanh đếm ngược; app hỏi Bilibili mỗi 2 giây. Trạng thái: Đang tạo mã → **Đang chờ quét** → **Đã quét, bấm xác nhận trên điện thoại** → **Đã đăng nhập: <tài khoản>** (nút Xong). Lỗi: **Mã đã hết hạn**, **Bilibili chưa chấp nhận đăng nhập** (bấm Huỷ trên điện thoại hoặc bị kiểm tra thêm), **Chưa kết nối được Bilibili**; cả ba có **Tạo mã mới**. Đóng hộp thoại là dừng hỏi.
    - Đăng nhập xong: app **bỏ giờ chờ** của lần bị chặn và quét lại kênh Bilibili ngay; thẻ hiện tài khoản, nhãn **Mã QR**, “Tự gia hạn mỗi ngày · lần cuối 08:12”, nút **Gia hạn ngay** và **Xoá đăng nhập**.
    - **Tự gia hạn:** trước khi quét kênh, tối đa một lần mỗi ngày, app kiểm tra và làm mới đăng nhập. Không làm mới được: thẻ viền cam, nhãn **Cần đăng nhập lại**, banner “Đăng nhập Bilibili đã hết hạn” + **Quét mã QR**; trong lúc chờ app quét kênh không đăng nhập (không chặn việc đang làm). Chấm cam ở mục Tài khoản nền tảng bên trái.
    - **Cookie dán tay** là lối phụ: **Dán cookie thủ công** (mở ra tại chỗ, hoặc từ hộp thoại: “Không quét được? Dán cookie thủ công”). App kiểm tra với Bilibili trước khi lưu; chỉ nhận cookie bilibili.com. Cookie dán tay không tự gia hạn; thẻ nhắc quét mã một lần để bật tự gia hạn.
    - Điện thoại: nút trên thẻ (**Đăng nhập** · **Quét mã QR** · **Quét lại** · **Gia hạn**) mở sheet cùng nội dung; có thêm dòng **Bilibili trên chính máy này? → Mở app** (mở link đăng nhập trong app, không cần quét) — chỉ khi dev thử trên máy thật thấy mở được app; không được thì thay bằng câu “Quét bằng điện thoại khác, hoặc mở Cài đặt trên máy tính” (#6). <!-- r32 --> Dán cookie chỉ làm trên máy tính.
  - Cuối danh sách: dòng **Nền tảng khác** dẫn sang Thư viện › Video nền (dán link nền tảng chưa hỗ trợ ở đó để gửi yêu cầu).
  - Điện thoại: màn Tài khoản nền tảng cùng thẻ; đổi cookie trình duyệt chỉ trên máy tính.
- **Lưu trữ** <!-- r17-storage --> (trước là Dọn dữ liệu):
  - **Thư mục lưu dự án** — nơi lưu assets của mọi video: video gốc, ảnh đã vẽ, MP4, project editor (mặc định `~/.viderse/projects`; bản web, #10) <!-- r34 -->. Cơ sở dữ liệu, mô hình và công cụ luôn ở máy này để app mở nhanh và không hỏng khi rút ổ.
  - **Đổi thư mục:** ô đường dẫn + **Chọn thư mục…** (hộp chọn của hệ điều hành); gợi ý nhanh các ổ đang cắm. Chọn gốc một ổ thì tạo thư mục con “Viderse Projects”. Kiểm tra trước khi cho bấm: đủ chỗ (dữ liệu + 10%), ghi được, không phải thư mục đồng bộ đám mây (Google Drive, iCloud, Dropbox), cảnh báo exFAT.
  - **Dữ liệu đang có:** *Chuyển sang thư mục mới* (mặc định; cùng ổ thì đổi tên trong vài giây) hoặc *Chỉ video mới lưu ở thư mục mới* (dự án cũ ở lại, vẫn mở được).
  - **Đang chuyển** là một việc nền: video đang làm tạm dừng rồi tự chạy tiếp; Bàn dựng, Duyệt vẫn mở được nhưng không sửa video; Huỷ thì thư mục cũ vẫn nguyên. Xong: bản cũ giữ 7 ngày rồi tự xoá, có **Xoá bản cũ ngay** và **Chuyển về**.
  - **Ổ bị rút / không thấy thư mục:** video đang làm tạm dừng, hàng chờ chưa chạy, thông báo Cần xử lý (“Không thấy ổ Samsung T7”) với **Thử lại** và **Lưu video mới vào máy này**; cắm lại thì tự chạy tiếp.
  - Điện thoại: đổi được sang các ổ đang cắm vào máy; chọn thư mục tuỳ ý chỉ làm trên máy tính.
  - Dung lượng và dọn dẹp như cũ: Xem trước → Dọn.
  - **Chạy trên máy chủ** <!-- r29-s3 --> (Gói V25). Viderse chạy trên Mac mini (`vid.aarverse.studio`), đăng nhập chung với Noverse. Luồng, thuật ngữ và cách hiện lỗi giống Noverse Gói AN (Tuỳ chỉnh › Lưu trữ tệp): chọn nhà cung cấp → điền → Kiểm tra kết nối → Lưu → chuyển dữ liệu. Khác Noverse chỉ ở dữ liệu riêng của Viderse.
    - **Ai thấy gì:** quản trị viên thấy nơi lưu, chọn kho, bộ nhớ đệm và hoạt động. Thành viên chỉ thấy Dung lượng theo nhóm và Dọn dữ liệu (`role=member`). Đầu trang ghi “Cài đặt · máy chủ”, nhóm đầu cột trái là “Máy chủ”.
    - **Hai cách lưu:** *Ổ đĩa máy chủ* (mặc định; như V22 nhưng bỏ Chọn thư mục…, Mở trong Finder và gợi ý ổ đang cắm) và *Kho S3*. Viderse và Noverse dùng chung máy chủ Garage nhưng mỗi app một kho, một khoá. Bản desktop vẫn như V22.
    - **Tổng quan:** thẻ nơi lưu (tên, trạng thái, bảng giá trị: nhà cung cấp, tên kho, địa chỉ chỉ có host, vùng, cách gửi tệp, kiểu đường dẫn, 4 ký tự cuối mã khoá, nguồn cấu hình) với *Kiểm tra kết nối* (kết quả một dòng ngay trong thẻ) và *Đổi kho…*. Đang ở ổ máy chủ thì có thẻ mời *Chuyển sang kho S3* → *Chọn kho S3*. Bên dưới là **Dung lượng theo nhóm** (ảnh đã vẽ, project editor, thư viện gồm lớp phủ, video nền và ảnh kênh, video nguồn, bản xuất MP4; số tệp và dung lượng; tổng; nhóm chưa đếm được ghi “chưa có số” và tổng ghi “Đã đếm …”, tweak `usage=partial`), **Bộ nhớ đệm làm việc trên máy chủ** (đã dùng / giới hạn), **Hoạt động từ lần khởi động** (ghi, đọc, tạo link, xoá: số lần, lỗi, dữ liệu, trung bình, chậm nhất; lỗi > 0 tô cam; gửi qua máy chủ thì không có dòng Tạo link) và **Chỗ cũ** sau khi chuyển.
    - **Chọn kho S3:** lưới 8 thẻ: Garage, Cloudflare R2, Amazon S3, Google Cloud Storage, Wasabi, Backblaze B2, DigitalOcean Spaces, Khác (tương thích S3). Logo đơn sắc là chữ viết tắt trong vòng tròn, một dòng gợi ý và cách gửi gợi ý. Không có MinIO. Kho đang dùng có nhãn Đang dùng; chọn lại nó thì khoá hiện dạng đã lưu.
    - **Điền thông tin:** ô theo nhà cung cấp (R2: Account ID; AWS, Wasabi, B2, DigitalOcean: chọn vùng; Garage: địa chỉ máy chủ nội bộ, vùng điền sẵn “garage”, công tắc *Cho phép http (chỉ mạng nội bộ)* kèm cảnh báo khi bật; Khác: địa chỉ, vùng, kiểu đường dẫn, http) cộng tên kho, mã khoá truy cập, khoá bí mật (đã lưu: `••••c41d` + *Đổi khoá*). *Lấy ở đâu?* gập sẵn ở đầu thẻ; với Garage là các bước và 3 lệnh tạo kho và khoá riêng cho Viderse, có *Chép*. Garage có thêm dòng “Nên dùng kho và khoá riêng, không dùng chung kho với Noverse.”
    - **Cách gửi tệp tới trình duyệt:** *Trực tiếp từ kho* (link ký ngắn hạn; cần CORS và địa chỉ ra internet) hoặc *Qua máy chủ Viderse* (không cần CORS). Gợi ý: Garage → qua máy chủ, đám mây → trực tiếp. Chọn trực tiếp mà địa chỉ là nội bộ thì hiện ô *Địa chỉ kho cho trình duyệt*, để trống thì cảnh báo. Video tua được ở cả hai cách (đọc theo đoạn).
    - **Kiểm tra kết nối:** 6 bước chạy lần lượt: Ghi thử · Đọc lại · Đọc một đoạn · Tạo link ký (“không cần” khi qua máy chủ) · Trình duyệt tải được (CORS) hoặc Máy chủ gửi được một đoạn cho trình duyệt · Xoá tệp thử. Mỗi bước có trạng thái và thời gian (ms). Đạt: dòng xanh có tổng thời gian, *Lưu…* sáng lên. Lỗi: khối cam ghi lý do và cách sửa. Thiếu CORS thì có đoạn cấu hình theo nhà cung cấp, *Chép*, *Kiểm tra lại*, *Chuyển sang gửi qua máy chủ*. Trình duyệt không tới được kho thì có *Điền địa chỉ cho trình duyệt* · *Chuyển sang gửi qua máy chủ*. Mỗi lúc chỉ một nút sáng: chưa kiểm tra là *Kiểm tra kết nối*, đạt là *Lưu…*, lỗi CORS hay địa chỉ là nút trong khối lỗi.
    - **Thanh lưu** dính ở đáy: một câu trạng thái, *Huỷ*, *Lưu…*. Chưa đạt, thiếu ô hoặc máy chủ chưa có khoá mã hoá thì *Lưu…* khoá, câu nói lý do. Sửa bất kỳ ô nào thì kết quả kiểm tra bị xoá, phải kiểm tra lại.
    - **Do máy chủ đặt** (biến môi trường): mọi giá trị chỉ đọc, nhãn “do máy chủ đặt · chỉ đọc” ở đầu thẻ, không có *Đổi kho…*, vẫn bấm được *Kiểm tra kết nối*. Khối *Chi tiết cho người quản trị máy chủ* (gập sẵn) liệt kê tên biến, đúng tên của Noverse (`STORAGE_DRIVER`, `S3_*`, `BLOB_*`; hỏi đáp #49).
    - **Chưa có khoá mã hoá:** banner vàng ở đầu form; vẫn kiểm tra được, *Lưu…* khoá. Khối chi tiết ghi đường dẫn tệp khoá `secret.key` (Viderse tự tạo, 0600), lý do (thư mục cấu hình chỉ đọc, tệp sai quyền, tệp hỏng) và cách sửa rồi khởi động lại; không có biến môi trường nào đặt khoá.
    - **Xác nhận chuyển:** hộp thoại “Chuyển 412 tệp · 18,6 GB sang Garage?” có bảng nhóm và 4 câu cam kết: video đang làm vẫn chạy, Bàn dựng và Duyệt vẫn sửa được, tệp mới ghi vào chỗ cũ tới khi xong rồi được chép nốt; mỗi tệp chép rồi đọc lại; chỗ cũ không bị xoá; tạm dừng hoặc huỷ lúc nào cũng được. Nút *Huỷ* · *Bắt đầu chuyển*.
    - **Đang chuyển** là một việc nền (hiện trong Hoạt động): hai thanh (số tệp, dung lượng), tiến độ theo nhóm, tốc độ, thời gian còn lại ước từ tốc độ thật, Đã chép · Đã kiểm tra · Lỗi; *Tạm dừng / Tiếp tục* · *Huỷ chuyển* (hỏi lại tại chỗ: vẫn dùng chỗ cũ, tệp đã chép nằm lại trong kho mới và được lần chuyển sau dùng lại, không chép lại; không có nút xoá phần đã chép). **Khác V22:** video không tạm dừng, Bàn dựng và Duyệt vẫn sửa được.
    - **Lỗi chuyển:** bảng Nhóm · Video · Lý do và *Thử lại 3 tệp lỗi*. Còn tệp lỗi thì chưa đổi kho.
    - **Chỗ cũ** sau khi chuyển: “Chỗ cũ: ổ đĩa máy chủ vẫn giữ 18,6 GB · 412 tệp”. Viderse không tự xoá; quản trị viên dọn bằng lệnh bảo trì khi đã yên tâm (đường dẫn trong khối chi tiết; không gợi ý xoá tay). Khác V22 (giữ 7 ngày rồi tự xoá) để giống Noverse AN, vì trên máy chủ không xoá dữ liệu tự động.
    - **Mất kết nối kho:** banner cam ở mục Lưu trữ, dùng lại khung “ổ bị rút” (video đang làm tạm dừng, hàng chờ chưa chạy, kết nối lại thì tự chạy tiếp) với *Kiểm tra lại*; một dòng cảnh báo cho quản trị viên ở đầu mọi trang (*Mở Lưu trữ*); thông báo Cần xử lý. Thành viên chỉ thấy một dòng thông tin ở mục Lưu trữ. Màu theo Viderse: lỗi là cam `#FF9A5C`, không thêm đỏ.
    - **Dọn dữ liệu khi dùng kho S3:** Xem trước → Dọn như cũ. Danh sách gồm tệp trên kho của video đã xoá, media không còn ai dùng, bộ nhớ đệm làm việc (giữ tệp của video đang dựng) và tiến trình bỏ quên. Câu chân thêm “Tệp xoá khỏi kho không lấy lại được.” Thành viên không dọn bộ nhớ đệm. Không có cảnh báo exFAT, đồng bộ đám mây hay ổ bị rút.
    - **Điện thoại:** hàng Lưu trữ ghi “Garage · 18,6 GB” (ổ máy chủ: “Ổ máy chủ · 18,6 GB”; mất kết nối: “Garage mất kết nối” màu cam; đang chuyển: “Đang chuyển · 42%”). Trang chỉ đọc: banner mất kết nối, tiến độ chuyển, thẻ nơi lưu, *Kiểm tra kết nối*, Dung lượng theo nhóm, dòng “Đổi kho … làm trên máy tính”.
    - **Không có:** chi phí tiền, biểu đồ theo ngày, dung lượng kho của Noverse, lịch dọn tự động, dòng “Dùng chung máy chủ lưu trữ với Noverse” (GAP).
- **Báo lỗi:** xuất log đã che thông tin.

**Artboard:** `Settings` · `section=tools|connect|agents|accounts|storage|logs`, `store=here|pick|lowspace|cloud|moving|done|donenew|offline` (máy tính) `|server|s3pick|s3form|s3testing|s3testok|s3testfail|s3envlocked|s3nokey|s3confirm|s3moving|s3moveerr|s3active|s3down` (máy chủ, Gói V25) với `section=storage`, `sprov=garage|r2|aws|other`, `sfail=key|bucket|perm|net|clock|cors|reach`, `skey=new|saved`, `role=admin|member`, `usage=full|partial`, `conn=list|add|added|models|manual|failed`, `grok=signed|code|out`, `ai=none|director|artist`, `tools`, `bili=blocked|saved|manual|expired|empty`, `qr=none|loading|waiting|scanned|expired|rejected|error|success`, `yt=ok|bot` (với `section=accounts`), `clean` · `MSettings` · `screen=list|tools|connect|agents|accounts|storage`, `bili`, `qr` (sheet mã QR), `store=here|pick|lowspace|moving|done|offline|server|s3active|s3moving|s3down`, `usage=full|partial`, `sheet=none|add|models|director|artist`.

**Điện thoại:** cùng hai mục. Danh mục model và ô chọn model mở dạng sheet; Thêm nhà cung cấp là sheet có lưới nhà cung cấp và ô khoá. Mức suy nghĩ và chuỗi dự phòng của Đạo diễn chỉ chỉnh trên máy tính.

### W14 · Nhắn trợ lý
- **Mục tiêu:** sai khiến trợ lý bằng câu tự nhiên thay vì tìm nút.
- **Ba ô nhắn:** **Nhắn cho <trợ lý>** (Bàn dựng, phạm vi một video), **Dặn trợ lý về kênh này** (Kênh, phạm vi một kênh) và **Nhắn trợ lý · mọi kênh** (Bản tin). <!-- r16-agent -->
- **Cách xử lý:** mỗi câu thành một **kế hoạch** 1–5 bước, mỗi bước là một thao tác có sẵn, rồi hiện **thẻ Kế hoạch** trước khi chạy (§9.7). Câu một việc vẫn là kế hoạch 1 bước. Thiếu thông tin → hỏi lại một câu kèm lựa chọn, không đoán. Câu không chuyển được → “Chưa làm được việc này” + 2 gợi ý gần nhất.
- **Danh sách thao tác v1:** xem [08-dev-prompt.md › Gói V11](08-dev-prompt.md#goi-v11).

### W15 · Sửa trong editor <!-- r27-editor -->
- **Mục tiêu:** video chưa đạt độ hoàn thiện → sửa đúng chỗ, đo lại, xuất. Người dùng không phải biết dựng phim.
- **Vào:** Bàn dựng › Cần xử lý › **Sửa trong editor** (hoặc ⋯ › Mở trong editor). Mở sẵn bảng **Cần sửa**, chọn sẵn đoạn lệch đầu tiên, playhead ở đó.
- **Bố cục:** thanh trên (← Bàn dựng · tên · Đã lưu · hoàn tác/làm lại · ? · chip **Độ hoàn thiện 84/100** · Xuất) · thanh trái 68px **có chữ**: Cần sửa (số việc) · Cảnh · Phụ đề · Nền · Âm thanh · Media · bảng trái 316px · khung xem · bảng phải 316px · timeline.
- **Cần sửa:** điểm + vạch 90; **Chưa đạt** (mỗi mục một thẻ cam, liệt kê từng đoạn: giờ · câu · “sớm 0,6 s”, bấm để tới; **Khớp theo giọng cả N đoạn**); **Cần đo lại** (mục bị ảnh hưởng bởi thay đổi chưa đo); **Đã đạt N** (gập). Chân bảng: **Đo lại độ hoàn thiện** (khoá khi chưa đổi gì; thành nút chính khi đã sửa hết) + giờ đo lần cuối. Đang đo: Kiểm định + tiến độ. Đạt: “Đã đạt. Xuất được rồi”, chip xanh 93/100, **Xuất** thành nút chính.
- **Bảng phải theo thứ đang chọn** (không còn tab Clip/Chuyển cảnh/Khung hình):
  - **Đoạn phụ đề:** “Đoạn lệch 2/3” + ‹ › · trạng thái (“Chữ hiện sớm 0,6 giây so với tiếng” / “Đã khớp tiếng”) · dải **Tiếng / Chữ** 3 giây quanh đoạn để thấy lệch · **Khớp theo giọng** (nút chính) · Dời cả đoạn −0,1 s / +0,1 s · Nghe lại đoạn này (L) · sửa chữ của đoạn. Khung xem có nhãn nhỏ trên phụ đề.
  - **Cảnh:** ảnh (16:9, có hiệu ứng đang vẽ) · **Bản** 1 · 2… (chấm xanh = đang dùng, *Dùng bản N*) · **Prompt đã gửi** (4 dòng, *Xem đủ*, *Sửa* phần Cảnh; ảnh tham chiếu thu nhỏ; *Mở ở Duyệt* để sửa đủ) · ô góp ý + **Vẽ lại** · **Thay ảnh**. Vẽ xong ảnh mới **tự thay vào timeline**, giữ chuyển động, keyframe, thời lượng; toast “Cảnh 5 đã dùng bản 2” + *Hoàn tác (⌘Z)*; ảnh mới đánh dấu cần duyệt lại và Độ hoàn thiện kiểm lại. Trong lúc vẽ vẫn chỉnh editor được; khung xem và clip trên timeline có vòng chờ. <!-- r33-img --> · Chuyển động (Đứng yên / Zoom chậm / Trôi + co giãn) · Sang cảnh sau (Cắt / Mờ 1,2 s / Trượt) · **Nâng cao** gập (vị trí, kích thước, độ trong suốt, keyframe).
  - **Clip nền:** nguồn, đoạn lấy · Đổi ngẫu nhiên · Chọn đoạn khác · Bỏ (nói trước nền sẽ lặp). Chỉ cho video này.
  - **Không chọn gì:** cách bắt đầu + khung hình 1920×1080 · 30 fps (khoá, có lý do).
- **Ảnh cảnh (đợt 41):** bấm ảnh ở bảng phải mở Trình xem ảnh chế độ **Xem + Thay** (7 ảnh của video, ← →, Thay ảnh). <!-- r41g -->
- **Bảng trái khác:** Cảnh (7 dòng ảnh + giờ), Phụ đề (tìm, đoạn lệch có nhãn), Nền & lớp phủ (độ phủ, clip của video, Đổi ngẫu nhiên tất cả, nhắc đổi cho mọi video ở Template), Âm thanh (LUFS đo được, âm lượng giọng gốc / nền / lớp phủ), Media (như cũ).
- **Timeline:** thanh công cụ **Tách (S) · Xoá · Bắt dính** + chip **Đoạn lệch 2/3 ‹ ›** + thu phóng. Bỏ công cụ chữ “V/T” và nhãn “Vùng In–Out” (chỉ hiện khi đã đặt). Đoạn lệch cam trên thước và track; thứ đang chọn có viền 2px đỏ; đoạn đã khớp trở về màu track.
- **Xuất khi chưa đạt:** hỏi “Xuất khi chưa đạt độ hoàn thiện?” — liệt kê mục còn lại · **Sửa tiếp** (chính) · **Vẫn xuất** (ghi lại là người dùng chọn bỏ qua; giống Vẫn xuất ở Bàn dựng).
- **Artboard:** `Editor` · `state=editing|export-gate|exporting|export-done|conflict|narrow`, `fix=todo|partial|done|measuring|passed`, `panel=fix|scenes|subs|bg|audio|media`, `focus=caption|scene|bg|none`, `img=view|edit|drawing|done` (với `focus=scene`).

## 8. Quy tắc giao diện gắn với logic

- **Đỏ:**
  - Một nút chính mỗi màn, và đó là hành động chính của trạng thái hiện tại.
  - Nút gửi tin nhắn màu trắng, không đỏ.
- **Mỗi trạng thái nói hai điều:** đang thế nào, và tiếp theo là gì (dòng phụ).
- **Việc chờ lâu:**
  - Luôn có thời điểm dự kiến (“khoảng 00:40”).
  - Không có số chắc chắn thì ghi chữ (“sau 2 video đang chạy”), không bịa số.
- **Thao tác bị khoá:**
  - Vẫn nhìn thấy và focus được.
  - Lý do nằm ngay cạnh: `aria-disabled` + câu giải thích.
- **Số tốn kém** (ảnh, lượt ChatGPT) hiện trước khi bấm, không sau.
<!-- r4-slots -->

## 9. Trợ lý chủ động <!-- r16-agent -->

> **Đợt 41 (03/11):** Bàn dựng đã bỏ **Vì sao?** (§9.1), học quy tắc (§9.3), **Bản tin** (§9.6), **Kế hoạch / Nhắn trợ lý** (§9.7). Đề xuất (§9.2) chỉ còn 3 loại `hot_video`, `template_failing`, `cleanup`, hiện thành dòng trong Chờ bạn (W5). §9.4 Mức tự chủ và §9.5 đổi ở đợt 42. Các mục dưới giữ để đối chiếu, **không làm phần đã bỏ**. <!-- r41g -->

Bốn trợ lý không chỉ chạy các bước. Chúng **giải thích**, **đề xuất**, **học** từ việc bạn sửa, **tự làm** phần bạn đã giao, và **kể lại** khi bạn quay về. Mục này là luật cho lớp đó; giao diện ở `Main` (`agent=…`), `Explore` (`why`), `Channel`, `Review` và các board điện thoại cùng tên.

### 9.0 Năm nguyên tắc
1. **Trợ lý đề xuất, bạn quyết** — trừ phần bạn đã giao ở Mức tự chủ (§9.4).
2. **Có lý do kiểm chứng được.** Mỗi quyết định và đề xuất ghi số cụ thể và nguồn của số. Không có số thì không đề xuất.
3. **Tự làm thì kể lại.** Mọi việc máy tự quyết nằm trong Bản tin và tab Thông báo, kèm **Hoàn tác** khi hoàn tác được.
4. **Năm việc không bao giờ tự làm:** đăng YouTube, xoá, đổi template hay prompt, bỏ liên kết đối thủ, tiêu ảnh trả phí vượt giới hạn ngày.
5. **Ít mà đúng.** Tối đa 5 đề xuất đang mở. Bấm **Bỏ qua** thì cùng đề xuất đó không quay lại trong 7 ngày; **Đừng đề xuất kiểu này** tắt cả loại cho kênh đó (bật lại ở Cài đặt thông báo › Đề xuất của trợ lý).

### 9.1 Vì sao?
- Link chữ nhỏ **Vì sao?** cạnh quyết định của trợ lý. Bấm mở popover (điện thoại: sheet) gồm: một câu lý do, 2–4 dòng bằng chứng (số + nguồn), và lối sửa nếu bạn không đồng ý.

  | Chỗ | Trợ lý | Lý do trả lời |
  |---|---|---|
  | Điểm nổi trội (Khám phá, thẻ, xem trước) | Trinh sát | Lượt xem/giờ so với trung vị 30 video gần nhất của đối thủ; tuổi video; vị trí trong tuần của đối thủ |
  | Tiêu đề, mô tả, tag (Bàn dựng › Nội dung) | Đạo diễn | Dựa trên tiêu đề gốc nào, quy tắc viết nào, quy tắc đã học nào |
  | Điểm ảnh (Duyệt) | Kiểm định | Mục bị trừ: khớp mô tả cảnh, chữ lạ trong ảnh, lệch nhận diện kênh, trùng ảnh trước |
  | Video được chọn ở Tự lấy | Trinh sát | Vì sao lấy (mới, qua bộ lọc) hoặc bỏ (dưới ngưỡng, quá ngắn, đăng trước lúc bật) — ở dòng “Tự lấy N video” của Bản tin và ở Video đã chọn của Khám phá |
  | Đề xuất | trợ lý của đề xuất | Bằng chứng của đề xuất |
  | Chờ lượt (đã có) | Hàng chờ | W10, không đổi |

### 9.2 Đề xuất
- **Thẻ đề xuất:** orb trợ lý + câu đề xuất (≤ 2 dòng) + một dòng bằng chứng + **Vì sao?** (mở lý do ngay trong thẻ) + nút chính (sáng) + **Bỏ qua** + ⋯ (Đừng đề xuất kiểu này). Nút chính ghi đúng việc và giá (“Làm cho Midnight Tales · ~7 ảnh”, “Bật duyệt theo điểm”). Đề xuất đổi thiết lập áp dụng ngay và hiện toast 5 giây có **Hoàn tác**.
- **Ở đâu:** nút **Đề xuất N** đầu cột trái Bàn dựng mở nhóm thẻ gọn (lọc theo phạm vi kênh, tối đa 3 + “Xem cả N trong Bản tin”), mục Đề xuất trong Bản tin (3 thẻ). Nhận xét từ số liệu (`perf_insight`) còn hiện ở Kênh › Video: một dòng “Trợ lý nhận ra…” ở bộ lọc Tất cả, đầy đủ ở bộ lọc Đã đăng. Điện thoại: thẻ Bản tin + thẻ đề xuất cuộn ngang đầu Bàn dựng.
- **Loại v1** (chỉ các loại này):

  | Loại | Trợ lý | Hiện khi | Nút chính |
  |---|---|---|---|
  | `hot_video` | Trinh sát | Video đối thủ liên kết đạt ≥ ×3 trong 72 giờ mà chưa bắt đầu: chưa vào hàng (Tự lấy tắt, bị lọc, hoặc video đăng trước lúc bật Tự lấy — Khám phá ghi rõ lý do này ở Video đã chọn) hoặc đang ở cuối hàng | **Làm cho <kênh>** (vào đầu hàng) · hoặc **Lên đầu hàng** |
  | `template_failing` | Kiểm định | Template dừng hoặc chưa đạt ≥ 3 lần trong 7 ngày cùng một lý do | Mở template (máy tính) · Bỏ khỏi luân phiên (điện thoại, vì sửa template chỉ có trên máy tính) |
  | `learn_rule` | Đạo diễn / Hoạ sĩ | Cùng một kiểu sửa lặp lại (§9.3) | Lưu quy tắc |
  | `raise_autonomy` | Kiểm định | 15 video liền của kênh duyệt không vẽ lại ảnh nào và mọi ảnh ≥ 85 | Bật Duyệt theo điểm |
  | `pace` | Trinh sát | Kênh đủ nhịp trước 12:00 trong 5 ngày liền và còn ≥ 10 video của kênh trong hàng | Tăng nhịp lên N |
  | `perf_insight` | Trinh sát | Có nhận xét đạt mức tin cậy (§9.5) | Áp dụng (quy tắc viết / thứ tự template) |
  | `cleanup` | Kiểm định | MP4 đã đăng > 7 ngày chiếm > 5 GB | Xoá MP4 đã đăng |

### 9.3 Học từ việc bạn sửa
- **Đạo diễn** so bản trợ lý viết với bản bạn sửa (tiêu đề, mô tả, tag). Cùng một kiểu sửa lặp lại **3 lần trong một kênh** (bỏ phần trong ngoặc cuối tiêu đề, bỏ emoji, rút ngắn dưới N ký tự, thêm năm, đổi ngôi kể, luôn thêm/bỏ một tag) → đề xuất **Lưu thành quy tắc** hiện ngay dưới ô vừa sửa (lần thứ 3) và trong nhóm Đề xuất.
- **Hoạ sĩ:** một góp ý vẽ lại giống nhau xuất hiện **2 lần với cùng prompt** (“không có chữ trong ảnh”, “tông lạnh hơn”) → đề xuất thêm vào **Ghi chú đã học** của prompt đó.
- **Không bao giờ học ngầm.** Quy tắc chỉ có hiệu lực khi bạn bấm Lưu. Mỗi quy tắc ghi nguồn (“Học từ 3 lần bạn sửa · 26–28/10”), có công tắc, **Xem ví dụ**, **Xoá**. Tối đa 20 quy tắc mỗi kênh.
- Hiệu lực từ lần viết/vẽ tiếp theo (L9, L14). Video đã viết xong không tự đổi; có thể bấm Viết lại.

### 9.4 Mức tự chủ
> **Đợt 42 (03/11):** thay bằng khối **Tự động** ở Kênh › Nguồn & tự động (W11): Tự lấy video (công tắc, lấy mọi video mới), Video mỗi ngày, **Tự duyệt ảnh** (công tắc, ngưỡng cố định 85). Máy luôn tự thử sửa lỗi 1 lần; video chưa đạt 90 luôn hỏi (Cần xử lý). Bảng ba mức dưới đây chỉ để đối chiếu, không làm. <!-- r42 -->

Mỗi kênh có một **Mức tự chủ** ở Kênh › Nguồn & tự động, chọn bằng 3 mức sẵn hoặc chỉnh từng dòng (hiện “Tuỳ chỉnh”).

| Việc | Cẩn thận | **Cân bằng** (mặc định) | Tự chạy |
|---|---|---|---|
| Lấy video mới của đối thủ | Chỉ đề xuất video ≥ ×3 | Lấy mọi video mới | Lấy mọi video mới |
| Duyệt ảnh | Luôn hỏi | Luôn hỏi | **Duyệt theo điểm**: tự duyệt khi mọi ảnh ≥ 85 |
| Khi gặp lỗi | Dừng và báo | Tự thử sửa 1 lần, không được mới báo | Tự thử sửa 1 lần, không được mới báo |
| Video chưa đạt 90 điểm | Hỏi (Cần xử lý) | Hỏi (Cần xử lý) | Tự sửa mục sửa được, đo lại; vẫn chưa đạt mới hỏi |

- Lựa chọn từng dòng: Lấy video (Chỉ đề xuất · Video ≥ ×2 · Mọi video mới); Duyệt ảnh (Luôn hỏi · Theo điểm ≥ N, N = 70–95 · Tự duyệt hết); Khi gặp lỗi (Dừng và báo · Tự thử 1 lần); Video chưa đạt (Hỏi · Tự sửa rồi hỏi).
- **Điểm ảnh** do Kiểm định chấm cho từng ảnh (0–100): khớp mô tả cảnh, không có chữ lạ hay chi tiết méo, đúng nhận diện kênh, không trùng ảnh đã dùng. Duyệt theo điểm vẫn duyệt **cả video** (L7): một ảnh dưới ngưỡng thì cả video vào Chờ duyệt; ảnh đó viền cam, Duyệt có dòng “6/7 ảnh từ 85 điểm · cảnh 4 được 72” và nút **Xem cảnh 4**.
- **Tự duyệt hết** và mức **Tự chạy** hỏi xác nhận khi bật (thay hộp `auto-approve` cũ), cùng một nội dung ở Kênh, điện thoại và `Dialogs` · `dialog=autonomy`. Ngoại lệ: ở **Tạo kênh** chọn Tự chạy không hỏi lại vì kênh chưa có video nào; dòng phụ của lựa chọn ghi rõ hệ quả.
- **Duyệt theo điểm** có ngưỡng chỉnh được 70–95, bước 5 (bước chỉnh hiện ngay dưới lựa chọn). Mức cũ “Tự duyệt ảnh bật” chuyển thành Tự duyệt hết; “tắt” chuyển thành Cân bằng.
- **Tự thử sửa 1 lần** chỉ cho lỗi có cách sửa tự động: tải lại nguồn, vẽ lại ảnh lỗi bằng nguồn dự phòng (L6), dựng lại. Lỗi cần bạn (đăng nhập hết hạn, thiếu clip nền) báo ngay.
- **Giới hạn chi tiêu:** máy chỉ tự tiêu ảnh trả phí (khoá API) tới giới hạn ngày ở Cài đặt › Kết nối AI; quá giới hạn thì chờ như hết hạn mức (L6) và báo trong Bản tin.
- Mọi việc tự làm ghi vào Thông báo (`action_result`, nguồn “Tự làm theo Mức tự chủ”) và Bản tin.

### 9.5 Ưu tiên kênh và hiệu quả sau đăng
> **Đợt 42 (03/11):** bỏ cả mục này (số liệu sau đăng, Trợ lý nhận ra, Ưu tiên kênh). Đánh dấu đã đăng chỉ ghi ngày; kênh luôn chạy đều theo thứ tự mới. Không làm phần dưới. <!-- r42 -->

- **Số liệu sau đăng:** khi bạn **Đánh dấu đã đăng** (dán link YouTube, hoặc app tự khớp theo link kênh), Trinh sát đọc lượt xem, thích, bình luận công khai ở mốc 24 giờ, 7 ngày, 28 ngày (nguồn chờ quyết định D5). Tab Video › Đã đăng hiện lượt xem 7 ngày và so với trung vị kênh (“×1,4”).
- **Trợ lý nhận ra:** khi kênh có ≥ 6 video đã đăng có số 7 ngày, Trinh sát so theo template, kiểu tiêu đề, độ dài, giờ đăng. Một nhận xét chỉ hiện khi chênh ≥ ×1,3 và mỗi nhóm ≥ 3 video; dưới 20 video gắn nhãn **Sơ bộ**. Mỗi nhận xét có một nút ghi đúng việc (“Thêm vào quy tắc viết”, “Cho chạy nhiều hơn”) — không tự áp dụng. Chỉ đếm video đã có số 7 ngày.
- **Ưu tiên kênh** (Kênh › Nguồn & tự động): **Đều đặn** (mặc định: giữ nhịp, lấy theo thứ tự mới) hoặc **Lượt xem**. Lượt xem: Tự lấy xếp video điểm nổi trội cao trước trong phần của kênh, và Template luân phiên cho template có lượt xem 7 ngày tốt hơn chạy nhiều hơn (tối đa gấp đôi lượt). Chưa đủ 6 video có số thì chạy như Đều đặn và ghi “Cần thêm N video có số liệu”.

### 9.6 Bản tin
- **Tự mở** khi bạn mở app sau ≥ 3 giờ không dùng, hoặc lần đầu trong ngày sau 05:00, và có điều mới. Mở tay: nút **Bản tin** đầu cột trái Bàn dựng.
- **Máy tính:** cột giữa Bàn dựng hiện Bản tin thay video; **Vào việc** mở video theo thứ tự W5. **Điện thoại:** thẻ Bản tin đầu Bàn dựng, bấm mở màn con.
- **Thứ tự nội dung:**
  1. Một câu: “Trong lúc bạn vắng: 2 video sẵn sàng đăng, 3 chờ bạn duyệt, 1 cần xử lý.” (khoảng thời gian ghi ở dòng trên).
  2. **Cần bạn** — danh sách ngắn + nút chính **Duyệt 3 video** (hành động chính của màn).
  3. **Máy đã tự làm** — Tự lấy N video, tự sửa N lỗi, tự duyệt N video (nếu có); mỗi dòng có Xem, và Hoàn tác khi được.
  4. **Đề xuất** — tối đa 3 thẻ.
  5. **Hôm nay** — chờ lượt, chờ bạn, hạn mức ảnh còn của nguồn đang vẽ (kể cả nguồn dự phòng), ảnh trả phí đã tự tiêu. Có điều chặn cả hàng (ví dụ phiên ChatGPT hết hạn khiến Đạo diễn không viết được) thì nói ngay dưới, kèm nút sửa. Không dự báo số video xong khi chưa tính đủ các nguồn dự phòng.
  6. Theo kênh — mỗi kênh một dòng (nhịp hôm nay, chờ bạn, sẵn sàng).
- Ô **Nhắn trợ lý · mọi kênh** ở cuối Bản tin.

### 9.7 Kế hoạch (Nhắn trợ lý)
- Một câu → **thẻ Kế hoạch**: mỗi bước một dòng có ô tích (bỏ được từng bước), trợ lý làm, giá; dòng tổng giá; nút **Làm N bước** (luôn sáng như board; bước đưa video vào làm ghi rõ trong dòng bước và dòng tổng giá, không đổi màu nút — #74) và **Sửa câu**. <!-- r32 -->
- Đang chạy: từng bước hiện trạng thái trong thẻ. Bước lỗi không chặn bước sau, trừ khi bước sau phụ thuộc.
- Thao tác thêm cho kế hoạch: `prioritize` (đưa video lên đầu), `set_autonomy`, `add_rule`, `make_videos` (nhiều video nổi trội của một đối thủ). Vẫn không có xoá, đăng, đổi template.

## 10. Đo thành công <!-- r16-agent -->

| Chỉ số | Cách đo | Mục tiêu bản đầu |
|---|---|---|
| Thời gian tới video đầu tiên Sẵn sàng | từ mở app lần đầu | ≤ 45 phút (máy chạy), ≤ 5 phút người dùng thao tác |
| Thời gian người dùng mỗi video | tổng thời gian ở Duyệt + Cần xử lý / số video Sẵn sàng | ≤ 2 phút |
| Video không cần bạn sửa | % video từ Đang làm tới Sẵn sàng không qua Cần xử lý và không vẽ lại | ≥ 70% |
| Đề xuất được nhận | % đề xuất bấm nút chính | ≥ 30%; loại nào < 10% sau 2 tuần thì xem lại |
| Hoàn tác việc tự làm | % việc tự làm bị hoàn tác | ≤ 5% |

**Kiểm thử với người dùng** trước khi code xong V1: 5 người vận hành kênh, mỗi người 5 việc trên prototype — tạo kênh đầu tiên, làm 2 video từ Khám phá, duyệt và vẽ lại 1 ảnh, xử lý một lỗi, đổi Mức tự chủ sang Tự chạy. Ghi thời gian và chỗ dừng lại; lỗi lặp ở ≥ 2 người phải sửa trước khi chốt.

