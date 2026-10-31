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
| Bộ lọc **Bản cũ** | Dữ liệu của bản app cũ, không phải trạng thái người dùng cần hiểu | Migration: job cũ có MP4 → Sẵn sàng/Đã đăng; job cũ dở → danh sách dọn trong Cài đặt › Dọn dữ liệu |
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
- **Toast chỉ khi:** (a) kết quả thao tác của chính bạn (5 giây, có Hoàn tác/Xem); (b) việc nền xong mà cần bạn hoặc bạn đang đợi: **Chờ duyệt**, **Cần xử lý** (cam, ở lại tới khi đóng), **Sẵn sàng**, và việc khác bạn tự bắt đầu (Nhận diện kênh đã xong). **Không bao giờ toast tiến độ.**
- **Gộp:** nhiều kết quả trong khoảng 10 giây → một toast (“3 video chờ duyệt · Duyệt ảnh”). Tối đa 3 toast, góc phải dưới, mới nhất trên cùng, rê chuột thì dừng đếm, Esc đóng.
- **Lịch sử:** mọi thứ đã toast đều nằm trong tab **Thông báo** của popover (chưa đọc có chấm; viên ở đầu trang có chấm vàng khi có thông báo cần bạn).
- **Cửa sổ ẩn:** thông báo hệ thống cho Chờ duyệt, Cần xử lý, Sẵn sàng; số trên biểu tượng app = số Chờ bạn. Có giờ yên lặng (vẫn ghi lịch sử).
- **Cài đặt thông báo** bật/tắt theo loại × kênh hiển thị (trong app / hệ thống).

**L14 · Hai lớp ảnh: template → video.** <!-- r9-assets --> Video lấy mọi thứ từ template. Ngoại lệ duy nhất là chip **Ảnh** ở Bàn dựng: đổi prompt ảnh cảnh, prompt ảnh bìa, hoặc không tạo ảnh cho riêng video đó.
- Cách ghép ảnh vào video thuộc template; muốn đổi thì đổi template (Làm lại từ đầu) hoặc sửa template.
- Không có ngoại lệ cho từng ảnh. Vẽ lại một ảnh nhận góp ý bằng chữ, vẫn theo prompt của video.
- Prompt được chốt lúc dùng (L9): sửa prompt trong Thư viện không đổi ảnh đã vẽ; video bắt đầu sau đó và lần vẽ lại sau đó dùng nội dung mới.
- Đổi ngoại lệ khi đã có ảnh luôn nói trước số ảnh phải vẽ lại, hạn mức còn và có phải dựng lại không. Video còn ở Chờ lượt đổi miễn phí.

**L15 · Model và nhà cung cấp.** <!-- r8-ai --> Mỗi trợ lý dùng một model đã chọn ở Kết nối AI. Model phải làm được việc: Đạo diễn cần **Chữ**, Hoạ sĩ viết mô tả cần **đọc ảnh**, nguồn vẽ cần **Vẽ ảnh**. Đổi model có hiệu lực từ bước chạy tiếp theo; bước đang chạy dùng nốt model cũ. Không xoá được nhà cung cấp đang có trợ lý dùng: app hỏi chọn model thay thế trước.

## 6. Điều hướng

### 6.1 Máy tính
Thanh trên: **Bàn dựng · Duyệt · Kênh · Khám phá · Video mới**.

Bên trái thanh trên: logo và **bộ đổi kênh** (“Mọi kênh” với 3 avatar chồng nhau, hoặc tên một kênh). Menu gồm Mọi kênh (nhịp hôm nay và số Chờ bạn của cả 3 kênh), từng kênh (nhịp hôm nay, chấm vàng khi có việc chờ bạn), Tạo kênh. Bộ đổi kênh chỉ có ở **Bàn dựng** (đổi phạm vi danh sách). Kênh và Khám phá dùng breadcrumb (“Kênh của tôi”, “Khám phá / <đối thủ>”); đổi kênh ở cột trái trang Kênh. <!-- r5-scope --> <!-- r11-audit -->

Bên phải thanh trên:
- Viên **“2 đang chạy · 12 chờ lượt”** mở popover **Việc nền** (Đang chạy · Thông báo); cuối popover có “Mở Hàng chờ”. Khi tạm dừng, viên đổi thành “Đã tạm dừng”; có thông báo cần bạn thì viên có chấm vàng.
- Bánh răng mở **Cài đặt**, có chấm khi có việc cần xem.
- Avatar.

Trang phụ:
- **Thư viện** mở từ cột trái trang Kênh.
- **Tạo kênh** (`CreateChannel`) mở từ cột trái trang Kênh và từ bộ đổi kênh; cùng màn với bước Tạo kênh của Lần đầu mở, nhưng không có bước Chuẩn bị máy. <!-- r13-create -->
- **Editor** mở từ “Mở trong editor”.

### 6.2 Điện thoại
Thanh dưới: **Bàn dựng · Duyệt · [+] Video mới · Kênh · Khám phá**.
- **Hai loại màn:** <!-- r14-mobile -->
  - **Màn gốc** (5 mục của thanh dưới: Bàn dựng, Duyệt, Kênh, Khám phá; Video mới là nút giữa): thanh trên = logo · bộ đổi kênh (chỉ Bàn dựng và Kênh) · viên Việc nền · avatar; dưới đó là tiêu đề lớn 24px. Có thanh dưới.
  - **Màn con** (chi tiết video, video đối thủ, Video mới, Tạo kênh, Cài đặt và các mục con): đầu trang 52px = nút Quay lại (‹) hoặc Đóng (×, cho luồng tạo mới) · tiêu đề 16px căn trái (có thể thêm dòng phụ) · thao tác bên phải. Không có thanh dưới.
- Khám phá và Duyệt không có bộ đổi kênh (giống máy tính: chỉ Bàn dựng đổi phạm vi, Kênh đổi kênh đang mở).
- Viên **Việc nền** ở mọi màn gốc mở cùng một sheet (đang chạy + “Mở hàng chờ đầy đủ ở Bàn dựng”); ở Bàn dựng là sheet Hàng chờ đầy đủ có Tạm dừng.
- Cài đặt mở từ avatar, là màn con (không có thanh dưới).
- Editor, Thư viện, Cài đặt template, Lần đầu mở chỉ có trên máy tính. Link tới các trang đó trên điện thoại hiện “Mở trên máy tính”.
- **Tạo kênh** có trên điện thoại (`MCreate`), mở từ sheet Đổi kênh: cùng trường, cùng mặc định với máy tính; nhận diện vẽ sau khi tạo nên không cần màn lớn. <!-- r13-create -->
- Trong trang Kênh, chỉ máy tính có: **Chọn nhiều** (thao tác hàng loạt) và ô **Dặn trợ lý về kênh này**. Mọi thiết lập còn lại của kênh có đủ trên điện thoại.

## 7. Workflow

Mỗi workflow: mục tiêu → các bước → luật áp dụng → artboard (`Tên` · `tweak=giá trị`).

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
- Nhận diện theo **đối thủ chọn đầu tiên** (tham chiếu): cùng ngôn ngữ, cùng thể loại; tên mới cùng nghĩa, ảnh cùng định dạng nhưng là tác phẩm mới. <!-- r10-channel -->
- **Mẫu chuẩn** tạo tự động = cách ghép **Tách nền** + prompt ảnh cảnh **Theo nhận diện kênh** + prompt bìa **Chữ hook**, 6 ảnh. Kênh mới vẽ đúng phong cách của mình mà không phải chọn gì. <!-- r11-audit -->
- Ngôn ngữ viết tiêu đề, mô tả lưu theo ngôn ngữ đoán từ đối thủ tham chiếu; đổi ở Hồ sơ kênh.
- Kênh vừa tạo, chưa có video: `Channel` · `fresh=true` (Tự lấy đang bật; lối làm video mới nhất của đối thủ — cùng video với màn Xong: video đăng gần nhất của Nocturne Stories). <!-- r21-empty -->
- Vẽ nhận diện lỗi (hết hạn mức…): tên và mô tả vẫn lưu; Hồ sơ kênh hiện khối cam “Chưa vẽ được avatar và ảnh bìa” + Thử lại + giờ tự thử lại. Kênh vẫn làm video được.
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
- **Chọn nhiều:** tích video → **Làm N video cho <kênh>** (đỏ) · Sao chép link · Top 10/20 · Bỏ chọn. Theo L2, các video này vào cuối hàng.
- **Tìm tiêu đề** <!-- r28-explore -->: ô 240px trước Chọn nhiều, phím `/`; tìm trong đối thủ đang chọn, cộng với lọc đang bật, không phân biệt hoa thường và dấu; không có kết quả → khối giữa lưới + Xoá tìm.
- **Chưa có kênh / hàng chờ** (trước V4, V6): các nút làm video, chọn kênh, liên kết kênh và lọc Trạng thái khoá kèm một câu lý do; không ẩn.
- Ví dụ ở ô thêm đối thủ: một kênh Anh, một kênh Việt, một mã UC.
- **Artboard:** `Explore` · `preview`, `select`, `rivalMenu`, `scan`, `search=none|match|nomatch`, `empty` · điện thoại `MExplore` (làm ở V10) · `MCompVideo` (Chi tiết video) · `sheet=none|picker`.

- **Vì sao nổi trội?** cạnh điểm ở Video đã chọn (`Explore` · `preview=true`, `why=true`) và ở chi tiết video trên điện thoại (`MCompVideo` · `sheet=why`) (§9.1); video đạt ≥ ×3 mà Tự lấy không lấy thành đề xuất `hot_video` (§9.2). <!-- r16-agent -->
### W4 · Làm video
- **Mục tiêu:** từ link tới video đang làm trong một bấm.
- **Bước:**
  - Dán **một link** hoặc **nhiều link** (chọn từ video nổi trội làm ở Khám phá; gợi ý ở dưới dẫn sang đó).
  - Hai chip đã điền sẵn: **Kênh** (kênh đang mở) và **Template** (“Luân phiên · <template tiếp theo>”). Mọi thứ khác lấy từ template; ngoại lệ cho riêng video đặt ở Bàn dựng (L14). <!-- r9-assets -->
  - Một dòng **kế hoạch** nói trước video sẽ chạy khi nào: “Lên đầu hàng chờ · bắt đầu khoảng 00:40” hoặc “Vào cuối hàng chờ, theo nhịp 5 video/ngày”.
  - Một nút chính **Làm video** / **Làm N video** (Enter).
- **Chặn:** link đã làm cho kênh này → dẫn tới video đó. Link lỗi → câu giải thích màu cam. Nhiều link: tự bỏ link trùng và ghi rõ số đã bỏ.
- **Luật:** L1, L2, L4.
- **Artboard:** `NewVideo` · `input=empty|pasted|duplicate|error|multi` · `MNew` · `input=empty|pasted|multi`.

### W5 · Theo dõi video đang làm (Bàn dựng)
- **Mục tiêu:** một màn trả lời “việc gì đang chờ tôi ở mọi kênh” và “video này đang ở đâu, ai đang làm, còn bao lâu”.
- **Phạm vi (bộ đổi kênh trên đầu trang):** <!-- r5-scope -->
  - **Mọi kênh** (mặc định khi mở app): đây là trang tổng quan. Không có trang Tổng quan riêng.
    - Đầu cột trái là khối **Kênh**: mỗi kênh một dòng gồm avatar, tên, nhịp hôm nay (`3/5`) kèm thanh, “N chờ bạn · N sẵn sàng đăng”, và icon cam khi template của kênh cần sửa. Bấm một dòng thì lọc theo kênh đó.
    - Danh sách video gộp cả 3 kênh; mỗi mục có avatar kênh 20px.
    - Tiêu đề cột là “Video mọi kênh · 6 / 13 hôm nay”; chân cột ghi “Tự lấy video: bật ở 2/3 kênh” (Quiet Hours ở mức Cẩn thận: chỉ đề xuất) và “Hàng chờ chung: 12 video”.
  - **Một kênh:** chỉ video của kênh đó. Chân cột ghi Tự lấy video và Template luân phiên của kênh.
  - Bàn dựng nhớ phạm vi đã chọn. Đổi phạm vi không đóng video đang mở; nếu video đó thuộc kênh khác phạm vi thì cạnh tiêu đề hiện avatar kênh của video.
- **Bố cục:**
  - Trái: video theo nhóm **Chờ bạn · Đang làm · Sẵn sàng đăng · Chờ lượt** (thêm **Đã đăng** khi xem một kênh). Nhóm rỗng thì ẩn.
    - Chờ lượt chỉ hiện vài video sắp chạy (Mọi kênh: 3, một kênh: 2) và link “Xem cả N trong Hàng chờ”.
    - Đã đăng hiện video gần nhất và link “Xem ở trang kênh”.
  - Giữa: tiêu đề + chip (trạng thái, nguồn, template, độ dài), rồi **Nhật ký tiến độ** (câu theo trợ lý), **Phòng vẽ** (ảnh bìa + cảnh: đã vẽ / đang vẽ / chờ / lỗi), và ô **Nhắn cho <trợ lý đang chạy>**.
  - Phải: tab **Trợ lý · Nội dung · Nguồn · Nền**.
- **Mở Bàn dựng chọn video nào:** Cần xử lý cũ nhất → Chờ duyệt cũ nhất → Đang làm mới nhất → Sẵn sàng mới nhất. Thứ tự này xét trong phạm vi đang xem (Mọi kênh thì xét cả 3 kênh).
- **Hành động trên video:**
  - Tạm dừng (Esc) / Chạy tiếp.
  - Menu ⋯: Mở trong editor, Làm lại từ đầu, Xoá video.
- **Chip Ảnh** <!-- r9-assets --> (cạnh chip Template; điện thoại: dòng “Ảnh của video” dưới bốn trợ lý) là chỗ ngoại lệ duy nhất của video:
  - Popover hai tab **Ảnh cảnh · <tên>** / **Ảnh bìa · <tên>**; dòng đầu là “Theo template · <tên>”, dòng cuối tab Ảnh cảnh là **Không tạo ảnh cho video này**, dòng cuối tab Ảnh bìa là **Dùng ảnh của bạn** (PNG/JPEG/WebP, không tốn ảnh).
  - Chọn khác thì hiện ngay **giá**: số ảnh phải vẽ lại, hạn mức còn, có phải dựng/xuất lại không. Nút chính ghi đúng việc: “Đổi và vẽ lại 4 ảnh”, “Đổi và vẽ lại ảnh bìa”, “Đổi · không tạo ảnh”.
  - Khác template thì chip có chữ “riêng video này”.
- **Tab Nội dung** <!-- r10-channel -->: tiêu đề, mô tả, tag do Đạo diễn viết theo video đối thủ và quy tắc viết của kênh.
  - Sửa trực tiếp từng ô; có bộ đếm theo giới hạn YouTube (100 · 5.000 · 30 tag / 500 ký tự). Sửa tay thì có nhãn “bạn đã sửa”.
  - Mô tả hiện **đoạn cuối của kênh** riêng một khối (không sửa ở đây, link “Sửa ở Hồ sơ kênh”). Tag luôn thêm của kênh đứng đầu, có biểu tượng kênh, không bỏ được ở đây.
  - **Viết lại**: chọn phần (Tiêu đề / Mô tả / Tag) + góp ý + gợi ý nhanh. Không tốn ảnh; phần đã sửa tay bị thay, bản cũ vào Lịch sử.
  - Sao chép từng phần hoặc cả gói. Gói đăng tải về luôn dùng bản mới nhất.
  - Điện thoại: mục **Nội dung đăng** trong chi tiết video, cùng các thao tác.
- **Luật:** L6, L9, L10, L14.
- **Artboard:** `Main` · `scope=all|channel` · `stage=images|review|render|issue|quality|done|paused` · `panel=agents|content|source|backdrop` · `backdropShort` · `promptPop=none|scene|cover` · `meta=pending|writing|failed|ai|edited|rewrite|history` (cần `panel=content`) · `MMain` · `scope=all|channel` · `screen=list|detail|jobs|switch` · `stage` · `promptPop=none|scene|cover` · `meta` (cần `screen=detail`). <!-- r11-audit -->
- **Điện thoại:** Mọi kênh có dải thẻ kênh cuộn ngang (rộng 158px) ở đầu danh sách. Bộ đổi kênh mở thành sheet **Đổi kênh** (Mọi kênh · 3 kênh · Tạo kênh → `MCreate`).

- **Trợ lý chủ động** (§9): nhóm **Đề xuất** đầu cột trái, nút **Bản tin**, **Vì sao?** ở Nội dung, gợi ý **Lưu thành quy tắc** dưới ô vừa sửa, ô nhắn trả về **thẻ Kế hoạch**. Artboard `Main` · `agent=recap|suggest|plan|learn|why`; điện thoại `MMain` · `screen=recap|plan`, `agent=learn|why`. <!-- r16-agent -->
### W6 · Duyệt ảnh
- **Mục tiêu:** duyệt một video trong dưới một phút, không rời bàn phím.
- **Bước:**
  - Video chờ duyệt của mọi kênh xếp từ cũ tới mới.
  - Xem ảnh lớn và lời kể của cảnh. Ảnh chưa ưng: góp ý rồi **Vẽ lại (R)**, hoặc **Bỏ ảnh này** (cảnh dùng ảnh cảnh bên cạnh).
  - Bấm **Duyệt cả video & dựng (A)**: Kiểm định bắt đầu, và màn hình tự mở video kế tiếp.
- **Khoá:** đang có ảnh vẽ lại → nút duyệt khoá với lý do.
- Vẽ lại một ảnh hay cả bộ đều theo prompt của video; muốn ảnh khác hẳn thì góp ý bằng chữ hoặc đổi chip Ảnh ở Bàn dựng. <!-- r9-assets -->
- **Phím:** ←/→ ảnh · J/K video · R vẽ lại · A duyệt cả video.
- **Điện thoại:** chạm trái/phải để đổi ảnh; góp ý mở dạng sheet có gợi ý nhanh.
- **Luật:** L7.
- **Artboard:** `Review` · `state=review|redrawing|empty` · `MReview` · `state=review|feedback|redrawing|empty` · `Dialogs` · `dialog=redraw-all`.

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
  - **Chờ lượt:** video bạn chọn ghi “Ưu tiên · bạn chọn” ở đầu. Mỗi dòng có giờ bắt đầu dự kiến, lọc theo kênh, kéo thả, Lên đầu, ⋯ (Mở kênh, Xem video gốc, Bỏ khỏi hàng chờ).
  - **Quy tắc chạy:** luồng video, luồng ảnh, chu kỳ xem đối thủ, nhịp từng kênh (chỉ xem), hạn mức ảnh, xong hôm nay.
- **Tạm dừng:**
  - **Tạm dừng hàng chờ** là nút trung tính. Video đang chạy làm xong bước hiện tại rồi dừng, và không video mới nào bắt đầu.
  - Khi đã tạm dừng, **Chạy tiếp** là nút chính.
- **Luật:** L1–L6.
- **Việc nền & thông báo:** popover từ viên đầu trang (L13).
- **Artboard:** `Queue` · `state=running|paused|limited|empty` · `Activity` · `view=running|inbox|toasts|system|settings|empty` · `MMain` · `screen=jobs`.

### W11 · Thiết lập kênh <!-- r10-channel --> <!-- r11-audit -->
Trang Kênh có 4 tab, giống nhau trên máy tính và điện thoại (trừ hai mục chỉ có trên máy tính ở §6.2):
- **Video:** danh sách video, lọc, chọn nhiều (máy tính). Video Sẵn sàng có **Tải gói đăng** ngay trên dòng.
- **Nguồn & tự động:**
  - Đối thủ liên kết: mỗi đối thủ có **Làm video mới nhất** · **Chọn video nổi trội** (mở Khám phá ở đối thủ đó, xếp theo Nổi trội, sẵn chế độ chọn nhiều; video chọn vào hàng chờ theo nhịp) · Mở trong Khám phá · **Bỏ liên kết…**. Điện thoại gom ba mục sau vào sheet ⋯.
  - **Bỏ liên kết** luôn hỏi xác nhận. Video đã làm và video đã vào hàng chờ giữ nguyên; Tự lấy không lấy thêm từ đối thủ đó; đối thủ vẫn ở Khám phá. Bỏ đối thủ tham chiếu thì đối thủ kế tiếp thành tham chiếu (không vẽ lại gì). Bỏ đối thủ cuối cùng thì Tự lấy tự tắt, ghi “cần ít nhất một đối thủ liên kết”.
  - Đối thủ đầu tiên có nhãn **Tham chiếu nhận diện**.
  - **Liên kết đối thủ** chọn từ đối thủ đang theo dõi (một đối thủ dùng được cho nhiều kênh). Theo dõi đối thủ mới làm ở Khám phá; sheet Theo dõi có lối tắt liên kết luôn với kênh.
  - **Nhịp mỗi ngày** (1–50) và **Mức tự chủ** (§9.4): 3 mức sẵn + 4 dòng chỉnh riêng; **Ưu tiên kênh** Đều đặn / Lượt xem (§9.5). Chọn Tự chạy hoặc Tự duyệt hết thì hỏi xác nhận (cùng nội dung trên máy tính và điện thoại). <!-- r16-agent -->
  - **Lịch đăng của đối thủ**: khung đăng lặp lại (máy tính ở cột phải, điện thoại tóm tắt 3 khung + link Khám phá).
- **Template luân phiên:** thứ tự, template kế tiếp, sức khoẻ template, thêm từ Thư viện, bỏ khỏi kênh. Sửa template ở Thư viện (máy tính).
- **Hồ sơ kênh:**
  - **Nhận diện:** ảnh bìa kênh (công tắc vùng hiện trên điện thoại 1546×423), avatar, **Vẽ lại avatar · 1 ảnh** · **Vẽ lại ảnh bìa kênh · 1 ảnh** · Đặt tên khác · Tải avatar + ảnh bìa kênh. Khi Hoạ sĩ đang vẽ: ảnh cũ giữ, các nút vẽ lại khoá. Khi lỗi: khối cam ở đầu + Thử lại + giờ tự thử lại.
  - **Mô tả kênh** (sửa được, ≤ 2.000 ký tự, dán vào Giới thiệu trên YouTube). Bản bạn sửa là hướng cho lần tạo lại sau.
  - **Phong cách nhận diện** (app tự rút, chỉ đọc): một câu + nguồn (đối thủ tham chiếu) + nơi dùng. Ô **Góp ý cho lần tạo lại** + **Tạo lại cả bộ · 2 ảnh**: Đạo diễn giữ ý của mô tả hiện tại, có thể đặt tên mới; ảnh cũ giữ tới khi ảnh mới xong; video đã làm không đổi.
  - **Khi viết tiêu đề, mô tả, tag:** Ngôn ngữ (Tiếng Anh / Tiếng Việt / **Theo đối thủ** = giữ ngôn ngữ của từng video nguồn), Giọng văn, **Đoạn cuối mô tả** (nguyên văn, ≤ 1.000 ký tự), **Tag luôn thêm** (≤ 10, đứng đầu mọi video). Tự lưu khi rời ô; áp cho video viết sau đó (video cũ: Viết lại ở Bàn dựng › Nội dung).
  - Thông tin kênh: tên, link YouTube. Xoá kênh.
  - Liên kết sang Cài đặt › Trợ lý AI (dùng chung mọi kênh).

Cột phải **Đang áp dụng cho kênh** (máy tính) chỉ tóm tắt: Tự lấy video (Bật · Nhịp 5/ngày), Mức tự chủ (Cân bằng), <!-- r16-agent --> Template luân phiên, Tiêu đề, mô tả, tag, Trợ lý AI (chung mọi kênh). Mỗi dòng có **Tuỳ chỉnh** mở đúng tab (Trợ lý AI mở Cài đặt). Nhãn **riêng** chỉ hiện khi khác mặc định của kênh mới (Nhịp 5, Mức tự chủ Cân bằng, chỉ Mẫu chuẩn, chưa có quy tắc viết).

**Xoá kênh:**
- Bị khoá khi kênh có video đang chạy.
- Xoá kênh giữ lại video đã làm và không động tới kênh trên YouTube.

**Artboard:** `Channel` · `tab=videos|sources|templates|settings`, `identity=ready|generating|failed`, `link=none|picker|empty`, `autonomy=balanced|careful|auto|custom|confirm` · `MChannel` · `tab=videos|sources|templates|settings`, `identity=ready|generating|failed`, `autonomy=balanced|careful|auto|custom|confirm`, `sheet=none|rival|unlink|row` · `MExplore` · `mode=add` (theo dõi + liên kết luôn).

- **Trợ lý chủ động ở Kênh:** Nguồn & tự động có Mức tự chủ và Ưu tiên kênh; Hồ sơ kênh có **Quy tắc đã học**; tab Video có **Trợ lý nhận ra** và lượt xem 7 ngày ở Đã đăng (§9.5). Artboard `Channel` · `tab=sources`, `tab=settings`, `tab=videos` + `filter=posted`, `autonomy=balanced|careful|auto|custom|confirm`; `MChannel` · cùng tab, `autonomy`, `sheet=row`. <!-- r16-agent -->
### W12 · Template và thư viện
- **Template tự lưu.** Mỗi thay đổi làm bản xem thử thành “cũ”. **Dựng lại bản xem thử** là nút chính duy nhất và chỉ hiện khi bản xem thử đã cũ.
- **Vẽ thử ảnh bìa** dùng hạn mức ChatGPT nên là nút trung tính, có ghi giá: “1 ảnh”.
- **Template đang được kênh dùng:** không xoá được. Bị chặn kèm lý do “Đang dùng ở 2 kênh. Bỏ khỏi kênh trước rồi xoá.”
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
- **Artboard:** `Library` · `tab=templates|prompts|backdrops|overlays`, `prompt=list|mine|builtin|new` (sửa bìa: bấm “Ảnh bìa” ở Dùng cho), `menu`, `nguon=channels|videos|adding|addvideo|unsupported`, `kenh=slowroads|nightdrivefm|bilibili|lofilanes` (với `tab=backdrops`) · `Template` · `section=images|cover|backdrop|output`, `picker=none|add|choose|video` (với `section=backdrop`), `preview`, `fewClips` · `Main` · `panel=backdrop`.

### W13 · Cài đặt máy
Sáu mục:
- **Công cụ trên máy:** tên theo việc làm. Mỗi công cụ có một nhãn sẵn sàng; chỉ khi có vấn đề mới hiện một nút.
- **Kết nối AI** <!-- r8-ai --> (giống Noverse): danh sách nhà cung cấp, mỗi thẻ có loại kết nối (Đăng nhập gói · Khoá API · Trên máy này), việc làm được (Chữ · Vẽ ảnh), hạn mức hoặc chi phí, và ba nút **Model · N** / **Thử kết nối** / Đăng xuất · Đổi khoá · Đổi địa chỉ.
  - **Model · N** mở danh mục lấy thẳng từ nhà cung cấp: tìm, lọc Đã chọn / Gợi ý / Tất cả, mỗi model có nhãn Chữ · Chữ · đọc ảnh · Vẽ ảnh, ngữ cảnh và giá. “Không thấy model? Thêm theo tên” gọi thử model rồi mới thêm.
  - **Thêm nhà cung cấp:** OpenAI, Gemini, OpenRouter, DeepSeek, xAI, Anthropic, Ollama, Tương thích OpenAI. Dán khoá → **Kết nối và lấy model** → thẻ mới lên đầu danh sách, nhắc chọn ít nhất một model. Khoá chỉ hiện dạng che.
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
    - Điện thoại: nút trên thẻ (**Đăng nhập** · **Quét mã QR** · **Quét lại** · **Gia hạn**) mở sheet cùng nội dung; có thêm dòng **Bilibili trên chính máy này? → Mở app** (mở link đăng nhập trong app, không cần quét; chờ dev xác nhận, xem hỏi đáp #6). Dán cookie chỉ làm trên máy tính.
  - Cuối danh sách: dòng **Nền tảng khác** dẫn sang Thư viện › Video nền (dán link nền tảng chưa hỗ trợ ở đó để gửi yêu cầu).
  - Điện thoại: màn Tài khoản nền tảng cùng thẻ; đổi cookie trình duyệt chỉ trên máy tính.
- **Lưu trữ** <!-- r17-storage --> (trước là Dọn dữ liệu):
  - **Thư mục lưu dự án** — nơi lưu assets của mọi video: video gốc, ảnh đã vẽ, MP4, project editor (mặc định `~/Library/Application Support/Viderse/data/projects`). Cơ sở dữ liệu, mô hình và công cụ luôn ở máy này để app mở nhanh và không hỏng khi rút ổ.
  - **Đổi thư mục:** ô đường dẫn + **Chọn thư mục…** (hộp chọn của hệ điều hành); gợi ý nhanh các ổ đang cắm. Chọn gốc một ổ thì tạo thư mục con “Viderse Projects”. Kiểm tra trước khi cho bấm: đủ chỗ (dữ liệu + 10%), ghi được, không phải thư mục đồng bộ đám mây (Google Drive, iCloud, Dropbox), cảnh báo exFAT.
  - **Dữ liệu đang có:** *Chuyển sang thư mục mới* (mặc định; cùng ổ thì đổi tên trong vài giây) hoặc *Chỉ video mới lưu ở thư mục mới* (dự án cũ ở lại, vẫn mở được).
  - **Đang chuyển** là một việc nền: video đang làm tạm dừng rồi tự chạy tiếp; Bàn dựng, Duyệt vẫn mở được nhưng không sửa video; Huỷ thì thư mục cũ vẫn nguyên. Xong: bản cũ giữ 7 ngày rồi tự xoá, có **Xoá bản cũ ngay** và **Chuyển về**.
  - **Ổ bị rút / không thấy thư mục:** video đang làm tạm dừng, hàng chờ chưa chạy, thông báo Cần xử lý (“Không thấy ổ Samsung T7”) với **Thử lại** và **Lưu video mới vào máy này**; cắm lại thì tự chạy tiếp.
  - Điện thoại: đổi được sang các ổ đang cắm vào máy; chọn thư mục tuỳ ý chỉ làm trên máy tính.
  - Dung lượng và dọn dẹp như cũ: Xem trước → Dọn.
  - **Chạy trên máy chủ** <!-- r29-s3 --> (Gói V25). Viderse chạy trên Mac mini (`vid.aarverse.studio`), đăng nhập chung với Noverse. Luồng, thuật ngữ và cách hiện lỗi giống Noverse Gói AN (Tuỳ chỉnh › Lưu trữ tệp): chọn nhà cung cấp → điền → Kiểm tra kết nối → Lưu → chuyển dữ liệu. Khác Noverse chỉ ở dữ liệu riêng của Viderse.
    - **Ai thấy gì:** quản trị viên thấy nơi lưu, chọn kho, bộ nhớ đệm và hoạt động. Thành viên chỉ thấy Dung lượng theo nhóm và Dọn dữ liệu (`role=member`). Đầu trang ghi “Cài đặt · máy chủ”, nhóm đầu cột trái là “Máy chủ”.
    - **Hai cách lưu:** *Ổ đĩa máy chủ* (mặc định; như V22 nhưng bỏ Chọn thư mục…, Mở trong Finder và gợi ý ổ đang cắm) và *Kho S3*. Viderse và Noverse dùng chung máy chủ Garage nhưng mỗi app một kho, một khoá. Bản desktop vẫn như V22.
    - **Tổng quan:** thẻ nơi lưu (tên, trạng thái, bảng giá trị: nhà cung cấp, tên kho, địa chỉ chỉ có host, vùng, cách gửi tệp, kiểu đường dẫn, 4 ký tự cuối mã khoá, nguồn cấu hình) với *Kiểm tra kết nối* (kết quả một dòng ngay trong thẻ) và *Đổi kho…*. Đang ở ổ máy chủ thì có thẻ mời *Chuyển sang kho S3* → *Chọn kho S3*. Bên dưới là **Dung lượng theo nhóm** (ảnh đã vẽ, project editor, thư viện, video nguồn, bản xuất MP4; số tệp và dung lượng; tổng), **Bộ nhớ đệm làm việc trên máy chủ** (đã dùng / giới hạn), **Hoạt động từ lần khởi động** (ghi, đọc, tạo link, xoá: số lần, lỗi, dữ liệu, trung bình, chậm nhất; lỗi > 0 tô cam; gửi qua máy chủ thì không có dòng Tạo link) và **Chỗ cũ** sau khi chuyển.
    - **Chọn kho S3:** lưới 8 thẻ: Garage, Cloudflare R2, Amazon S3, Google Cloud Storage, Wasabi, Backblaze B2, DigitalOcean Spaces, Khác (tương thích S3). Logo đơn sắc là chữ viết tắt trong vòng tròn, một dòng gợi ý và cách gửi gợi ý. Không có MinIO. Kho đang dùng có nhãn Đang dùng; chọn lại nó thì khoá hiện dạng đã lưu.
    - **Điền thông tin:** ô theo nhà cung cấp (R2: Account ID; AWS, Wasabi, B2, DigitalOcean: chọn vùng; Garage: địa chỉ máy chủ nội bộ, vùng điền sẵn “garage”, công tắc *Cho phép http (chỉ mạng nội bộ)* kèm cảnh báo khi bật; Khác: địa chỉ, vùng, kiểu đường dẫn, http) cộng tên kho, mã khoá truy cập, khoá bí mật (đã lưu: `••••c41d` + *Đổi khoá*). *Lấy ở đâu?* gập sẵn ở đầu thẻ; với Garage là các bước và 3 lệnh tạo kho và khoá riêng cho Viderse, có *Chép*. Garage có thêm dòng “Nên dùng kho và khoá riêng, không dùng chung kho với Noverse.”
    - **Cách gửi tệp tới trình duyệt:** *Trực tiếp từ kho* (link ký ngắn hạn; cần CORS và địa chỉ ra internet) hoặc *Qua máy chủ Viderse* (không cần CORS). Gợi ý: Garage → qua máy chủ, đám mây → trực tiếp. Chọn trực tiếp mà địa chỉ là nội bộ thì hiện ô *Địa chỉ kho cho trình duyệt*, để trống thì cảnh báo. Video tua được ở cả hai cách (đọc theo đoạn).
    - **Kiểm tra kết nối:** 6 bước chạy lần lượt: Ghi thử · Đọc lại · Đọc một đoạn · Tạo link ký (“không cần” khi qua máy chủ) · Trình duyệt tải được (CORS) hoặc Máy chủ gửi được một đoạn cho trình duyệt · Xoá tệp thử. Mỗi bước có trạng thái và thời gian (ms). Đạt: dòng xanh có tổng thời gian, *Lưu…* sáng lên. Lỗi: khối cam ghi lý do và cách sửa. Thiếu CORS thì có đoạn cấu hình theo nhà cung cấp, *Chép*, *Kiểm tra lại*, *Chuyển sang gửi qua máy chủ*. Trình duyệt không tới được kho thì có *Điền địa chỉ cho trình duyệt* · *Chuyển sang gửi qua máy chủ*. Mỗi lúc chỉ một nút sáng: chưa kiểm tra là *Kiểm tra kết nối*, đạt là *Lưu…*, lỗi CORS hay địa chỉ là nút trong khối lỗi.
    - **Thanh lưu** dính ở đáy: một câu trạng thái, *Huỷ*, *Lưu…*. Chưa đạt, thiếu ô hoặc máy chủ chưa có khoá mã hoá thì *Lưu…* khoá, câu nói lý do. Sửa bất kỳ ô nào thì kết quả kiểm tra bị xoá, phải kiểm tra lại.
    - **Do máy chủ đặt** (biến môi trường): mọi giá trị chỉ đọc, nhãn “do máy chủ đặt · chỉ đọc” ở đầu thẻ, không có *Đổi kho…*, vẫn bấm được *Kiểm tra kết nối*. Khối *Chi tiết cho người quản trị máy chủ* (gập sẵn) liệt kê tên biến.
    - **Chưa có khoá mã hoá:** banner vàng ở đầu form; vẫn kiểm tra được, *Lưu…* khoá. Tên biến hay tệp khoá chỉ nằm trong khối chi tiết.
    - **Xác nhận chuyển:** hộp thoại “Chuyển 412 tệp · 18,6 GB sang Garage?” có bảng nhóm và 4 câu cam kết: video đang làm vẫn chạy, Bàn dựng và Duyệt vẫn sửa được, tệp mới ghi vào chỗ cũ tới khi xong rồi được chép nốt; mỗi tệp chép rồi đọc lại; chỗ cũ không bị xoá; tạm dừng hoặc huỷ lúc nào cũng được. Nút *Huỷ* · *Bắt đầu chuyển*.
    - **Đang chuyển** là một việc nền (hiện trong Hoạt động): hai thanh (số tệp, dung lượng), tiến độ theo nhóm, tốc độ, thời gian còn lại ước từ tốc độ thật, Đã chép · Đã kiểm tra · Lỗi; *Tạm dừng / Tiếp tục* · *Huỷ chuyển* (hỏi lại tại chỗ: vẫn dùng chỗ cũ, tệp đã chép nằm lại trong kho mới, tự xoá nếu muốn). **Khác V22:** video không tạm dừng, Bàn dựng và Duyệt vẫn sửa được.
    - **Lỗi chuyển:** bảng Nhóm · Video · Lý do và *Thử lại 3 tệp lỗi*. Còn tệp lỗi thì chưa đổi kho.
    - **Chỗ cũ** sau khi chuyển: “Chỗ cũ: ổ đĩa máy chủ vẫn giữ 18,6 GB · 412 tệp”. Viderse không tự xoá; quản trị viên dọn khi đã yên tâm (đường dẫn trong khối chi tiết). Khác V22 (giữ 7 ngày rồi tự xoá) để giống Noverse AN, vì trên máy chủ không xoá dữ liệu tự động.
    - **Mất kết nối kho:** banner cam ở mục Lưu trữ, dùng lại khung “ổ bị rút” (video đang làm tạm dừng, hàng chờ chưa chạy, kết nối lại thì tự chạy tiếp) với *Kiểm tra lại*; một dòng cảnh báo cho quản trị viên ở đầu mọi trang (*Mở Lưu trữ*); thông báo Cần xử lý. Thành viên chỉ thấy một dòng thông tin ở mục Lưu trữ. Màu theo Viderse: lỗi là cam `#FF9A5C`, không thêm đỏ.
    - **Dọn dữ liệu khi dùng kho S3:** Xem trước → Dọn như cũ. Danh sách gồm tệp trên kho của video đã xoá, media không còn ai dùng, bộ nhớ đệm làm việc (giữ tệp của video đang dựng) và tiến trình bỏ quên. Câu chân thêm “Tệp xoá khỏi kho không lấy lại được.” Thành viên không dọn bộ nhớ đệm. Không có cảnh báo exFAT, đồng bộ đám mây hay ổ bị rút.
    - **Điện thoại:** hàng Lưu trữ ghi “Garage · 18,6 GB” (ổ máy chủ: “Ổ máy chủ · 18,6 GB”; mất kết nối: “Garage mất kết nối” màu cam; đang chuyển: “Đang chuyển · 42%”). Trang chỉ đọc: banner mất kết nối, tiến độ chuyển, thẻ nơi lưu, *Kiểm tra kết nối*, Dung lượng theo nhóm, dòng “Đổi kho … làm trên máy tính”.
    - **Không có:** chi phí tiền, biểu đồ theo ngày, dung lượng kho của Noverse, lịch dọn tự động, dòng “Dùng chung máy chủ lưu trữ với Noverse” (GAP).
- **Báo lỗi:** xuất log đã che thông tin.

**Artboard:** `Settings` · `section=tools|connect|agents|accounts|storage|logs`, `store=here|pick|lowspace|cloud|moving|done|donenew|offline` (máy tính) `|server|s3pick|s3form|s3testing|s3testok|s3testfail|s3envlocked|s3nokey|s3confirm|s3moving|s3moveerr|s3active|s3down` (máy chủ, Gói V25) với `section=storage`, `sprov=garage|r2|aws|other`, `sfail=key|bucket|perm|net|clock|cors|reach`, `skey=new|saved`, `role=admin|member`, `conn=list|add|added|models|manual|failed`, `grok=signed|code|out`, `ai=none|director|artist`, `tools`, `bili=blocked|saved|manual|expired|empty`, `qr=none|loading|waiting|scanned|expired|rejected|error|success`, `yt=ok|bot` (với `section=accounts`), `clean` · `MSettings` · `screen=list|tools|connect|agents|accounts|storage`, `bili`, `qr` (sheet mã QR), `store=here|pick|lowspace|moving|done|offline|server|s3active|s3moving|s3down`, `sheet=none|add|models|director|artist`.

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
  - **Cảnh:** ảnh · **Vẽ lại ảnh** (góp ý, ảnh cũ giữ tới khi duyệt) · **Thay ảnh** · Chuyển động (Đứng yên / Zoom chậm / Trôi + co giãn) · Sang cảnh sau (Cắt / Mờ 1,2 s / Trượt) · **Nâng cao** gập (vị trí, kích thước, độ trong suốt, keyframe).
  - **Clip nền:** nguồn, đoạn lấy · Đổi ngẫu nhiên · Chọn đoạn khác · Bỏ (nói trước nền sẽ lặp). Chỉ cho video này.
  - **Không chọn gì:** cách bắt đầu + khung hình 1920×1080 · 30 fps (khoá, có lý do).
- **Bảng trái khác:** Cảnh (7 dòng ảnh + giờ), Phụ đề (tìm, đoạn lệch có nhãn), Nền & lớp phủ (độ phủ, clip của video, Đổi ngẫu nhiên tất cả, nhắc đổi cho mọi video ở Template), Âm thanh (LUFS đo được, âm lượng giọng gốc / nền / lớp phủ), Media (như cũ).
- **Timeline:** thanh công cụ **Tách (S) · Xoá · Bắt dính** + chip **Đoạn lệch 2/3 ‹ ›** + thu phóng. Bỏ công cụ chữ “V/T” và nhãn “Vùng In–Out” (chỉ hiện khi đã đặt). Đoạn lệch cam trên thước và track; thứ đang chọn có viền 2px đỏ; đoạn đã khớp trở về màu track.
- **Xuất khi chưa đạt:** hỏi “Xuất khi chưa đạt độ hoàn thiện?” — liệt kê mục còn lại · **Sửa tiếp** (chính) · **Vẫn xuất** (ghi lại là người dùng chọn bỏ qua; giống Vẫn xuất ở Bàn dựng).
- **Artboard:** `Editor` · `state=editing|export-gate|exporting|export-done|conflict|narrow`, `fix=todo|partial|done|measuring|passed`, `panel=fix|scenes|subs|bg|audio|media`, `focus=caption|scene|bg|none`.

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
- Một câu → **thẻ Kế hoạch**: mỗi bước một dòng có ô tích (bỏ được từng bước), trợ lý làm, giá; dòng tổng giá; nút **Làm N bước** (sáng; đỏ nếu có bước đưa video vào làm) và **Sửa câu**.
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

