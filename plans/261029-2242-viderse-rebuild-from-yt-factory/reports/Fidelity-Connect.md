# Fidelity — Connect (Kết nối AI + Trợ lý AI)

Phạm vi: `settings?section=connect` (mọi tweak `conn=`, `grok=`), `section=agents`
(`ai=`), MSettings `screen=connect|agents` + `sheet=add|models|director|artist`,
ở 1440×900 và 390×844. Artboard là nguồn sự thật.

Vòng này: soi lại **từng state bằng cặp ảnh chụp trên bản build thật** (Go + dist
nhúng, không Vite) và sửa những chỗ còn lệch: cỡ nút, dòng giá trong thẻ nguồn vẽ
(điện thoại), chữ thường của pill năng lực, radio 22 trong sheet, thẻ `failed`
không được mở danh mục rỗng, ô “Đã kết nối” chỉ tính provider dùng được.

## 1. Bảng cặp ảnh (artboard ↔ app)

Ảnh: `/tmp/viderse-fidelity/connect/shots/` — `<state>-artboard.webp` ↔
`<state>-app.webp`. (Bộ `.png` vòng trước đã xoá.)

| Artboard | App | Cặp ảnh | Kết quả |
|---|---|---|---|
| `Settings?section=connect&conn=list` | `#/settings?section=connect` | `conn-list-artboard.webp` ↔ `conn-list-app.webp` | Khớp: avatar 34, pill loại + năng lực, phụ 12.5, nút `ghost btn` **32/13/r8/pad12**, hạn mức 4px + `đặt lại`, hàng dashed 46. Lệch dữ liệu: provider mới thêm đứng đầu (xem §4.1), ChatGPT/Grok chưa đăng nhập (không có tài khoản thật). |
| `…conn=list` (đã đăng nhập) | fixture `GET /api/ai/providers` | `conn-list-signed-artboard.webp` ↔ `conn-list-signed-app.webp` | Khớp: `Đã đăng nhập · gói Plus` / `· SuperGrok`, hạn mức 43/50 + 210/400, nút `Đăng xuất`. |
| `…conn=add` | nút dashed | `conn-add-artboard.webp` ↔ `conn-add-app.webp` | Khớp: lưới 4 cột 8 tile (77 cao, tên 14, phụ 11.5), tile “Đã kết nối” mờ 0.5, ô khoá cùng hàng CTA 36/600. |
| `…conn=added` | thêm provider thật (`fake_provider.py` local) | `conn-added-artboard.webp` ↔ `conn-added-app.webp` | Khớp: ghi chú “Đã kết nối … lấy được N model…”, danh mục mở sẵn lọc Gợi ý, tổng kết + `Gán cho trợ lý`. Lệch: 4 model thật ≠ 48. |
| `…conn=models` | bấm `Model · N` | `conn-models-artboard.webp` ↔ `conn-models-app.webp` | Khớp: ô tìm 32 + 3 chip lọc 26, hàng grid, tick 18 đỏ `#FF4D63`, id mono, **pill “Chữ · đọc ảnh”** (chữ thường sau dấu chấm giữa — artboard `04-components` §ModelCatalog), giá phải `min-width 132`, chân nguồn, tổng kết, `Thêm theo tên`. Lệch: ghi chú biên tập (§4.3). |
| `…conn=manual` | link “Thêm theo tên” | `conn-manual-artboard.webp` ↔ `conn-manual-app.webp` | Khớp: ô tên mono 34/r9 + `Kiểm tra & thêm` (ghost btn 32) + `Đóng`; hàng nằm **dưới** danh mục + chân như artboard. |
| `…conn=failed` | thêm OpenAI với khoá sai (401 thật) | `conn-failed-artboard.webp` ↔ `conn-failed-app.webp` | Khớp: chỉ header + hộp lỗi hổ phách (`rgba(255,154,92,.45)`, chữ `#FFC09A`), 3 nút `Đổi khoá`(warn) `Thử lại` `Thêm theo tên`; **không** mở danh mục rỗng nữa; phụ “Khoá API ••••lity · chưa lấy được model”. |
| `…grok=out` | thẻ Grok chưa đăng nhập | `grok-out-artboard.webp` ↔ `grok-out-app.webp` | Khớp: phụ xám “Chưa đăng nhập · cần gói SuperGrok hoặc X Premium”, chỉ còn primary `Đăng nhập Grok`. |
| `…grok=code` | bấm `Đăng nhập Grok` (device flow thật) | `grok-code-artboard.webp` ↔ `grok-code-app.webp` | Khớp: viền hổ phách .3, phụ “Đang đăng nhập · chờ bạn nhập mã trên trang xAI”, hộp mã mono 22 `.14em` + `Chép mã`, nút `Mở trang xAI`/`Huỷ` (32), đếm ngược `Đang chờ bạn xác nhận · mã hết hạn sau …`. |
| `…section=agents` | `#/settings?section=agents` | `agents-artboard.webp` ↔ `agents-app.webp` | Khớp: 4 hàng trợ lý, Đạo diễn (picker + Mức suy nghĩ + `+ Thêm`), Hoạ sĩ (picker + lưới nguồn vẽ + switch + `+ Thêm nguồn`), chân info. Lệch dữ liệu: chưa có fallback nên chỉ `+ Thêm`. |
| `…section=agents` (đã đăng nhập) | fixture providers | `agents-signed-artboard.webp` ↔ `agents-signed-app.webp` | Khớp từng dòng: `Gói Plus · 50 ảnh mỗi ngày` + `43/50 hôm nay / đặt lại 07:00`; `SuperGrok · 400 ảnh mỗi tuần` + `210/400 tuần này / đặt lại thứ Ba`; `Khoá API · trả theo ảnh · máy tự tiêu tối đa 50 ảnh mỗi ngày` + `$0,039 mỗi ảnh / ≈ $0,27 mỗi video` (giá video làm tròn 2 chữ số như artboard); radio thẻ nguồn 15/chấm 7. |
| `…ai=director` | mở picker Đạo diễn | `agents-director-artboard.webp` ↔ `agents-director-app.webp` (+ `agents-signed-director-app.webp`) | Khớp: popover 400, nền `#1A1A1D`, hàng `7px 10px` r9, nhóm `CHATGPT · GÓI PLUS` (tên gói từ wire), note 12, pill 20, tick đỏ, chân `Thêm model ở Kết nối AI` + icon chìa. Lệch: thứ tự nhóm (§4.1), ghi chú (§4.3). |
| `…ai=artist` | mở picker Hoạ sĩ | `agents-artist-artboard.webp` ↔ `agents-artist-app.webp` (+ `agents-signed-artist-app.webp`) | Khớp, kể cả mờ 0.45 + lý do “Không đọc được ảnh” / “Chỉ vẽ ảnh, không viết chữ”, giá `$1,25 vào · $10 ra`. |
| `MSettings?screen=connect` | 390×844 | `m-connect-artboard.webp` ↔ `m-connect-app.webp` (+ `m-connect-signed-app.webp`) | Khớp: câu dẫn 13/1.55, thẻ r16 (`12px 14px`), avatar 34, tên 15, pill 20, phụ 12.5, thanh 4px, `Ảnh hôm nay 43/50 / đặt lại 07:00`, 2 nút **44/13.5**, hàng dashed 48/14. |
| `…sheet=add` | nút dashed | `m-add-artboard.webp` ↔ `m-add-app.webp` | Khớp: sheet `md` (r22, `#141416`, tiêu đề 16, phụ 12.5, grabber 36×4, home indicator), tile 2 cột 58, label 13, input **48/15/r12** mono, CTA `xl` 48/13/15/600. |
| `…sheet=models` | `Model · N` | `m-models-artboard.webp` ↔ `m-models-app.webp` | Khớp: hàng 56, tick **22/r5 đỏ `#FF4D63` + tick mực nền**, id mono 13, note (giá nếu có), pill 22, chân “Đã chọn N model · giá tính…”. |
| `…screen=agents` | 390×844 | `m-agents-artboard.webp` ↔ `m-agents-app.webp` (+ `m-agents-signed-app.webp`) | Khớp: hàng trợ lý `min-height 60; 10px 14px`, orb 22, tên 14.5/500, vai 12; thẻ nguồn vẽ r16 (`12px 14px`), radio 20, tên 15, model mono 11.5, phụ 12.5, dòng số 12; thẻ switch 44. Bản signed khớp cả 3 dòng hạn mức/giá. |
| `…sheet=director` | mở hàng Đạo diễn | `m-director-artboard.webp` ↔ `m-director-app.webp` | Khớp: **radio 22** (`vv-radio--m`), nhóm 11/600, hàng 56 r12, pill, chân có icon. |
| `…sheet=artist` | mở hàng Hoạ sĩ | `m-artist-artboard.webp` ↔ `m-artist-app.webp` | Khớp, mờ 0.45 + lý do; phụ đúng câu artboard “Cần đọc được ảnh vì Hoạ sĩ xem khung đầu thủ lâm mẫu.” |

State backend không tạo được (đã đăng nhập gói, Gemini có khoá, hạn mức) dùng
`tab.route` trả JSON đúng kiểu wire của `internal/web/ai.go`; `conn=failed` và
`grok=code` chạy thật (401 từ OpenAI; device flow xAI không hoàn tất — quy tắc).
Không có fixture trong mã sản phẩm.

## 2. Sửa trong vòng này

1. **Cỡ nút theo đúng chỗ artboard** (`ProviderCard`, `ConnectSection`,
   `AddProvider`, `ModelCatalog`, `ChatGPTLogin`): nút desktop = kit
   `size="compact"` (32/r8/13/pad12 — artboard `.ghost btn`); nút thẻ điện thoại
   `lg` (44) + 13.5; CTA sheet `xl` (48/13/15/600 = `.btn-pri`); “Xong” trong
   sheet chuỗi `lg`. Bỏ hết metric override trong `.conn-act`/`.conn-m-act` để kit
   sở hữu cỡ; chỉ còn `gap` (và chữ 13.5 của hàng nút điện thoại, 14 của hàng
   dashed — hai chỗ artboard ghi khác kit).
2. **“Đổi khoá” dùng `variant="warnGhost"`** (`.ghost btn-warn` = viền
   `rgba(255,154,92,.45)`, chữ `#FFC09A`) thay class cục bộ; xác nhận trên build:
   `borderColor rgba(255,154,92,0.45)`, `color rgb(255,192,154)`, cao 32, weight 400.
3. **`ConfirmDialog size="compact"`** cho hộp xoá provider (artboard cũng
   `.ghost btn` trong dialog). Kit mới của FidelityMachine.
4. **Thẻ `failed` không mở danh mục rỗng**: `isNew && !failed` mới render
   `ModelCatalog`; trước đó thẻ lỗi vừa hiện hộp lỗi vừa hiện “Tìm trong 0 model”
   + “Không có model nào khớp.” + tổng kết — artboard chỉ có hộp lỗi.
5. **Trạng thái lỗi tách khỏi rỗng**: thêm `catalogFail`
   (“Không tải được danh mục model của nhà cung cấp này.”) cho lần tải hỏng; giữ
   `catalogEmpty` (“Không có model nào khớp.”) cho lọc không ra kết quả — và chỉ
   hiện khi thật sự 0 hàng.
6. **Pill năng lực viết thường** (`capsLine` = “Chữ · đọc ảnh”, “Vẽ ảnh” độc lập)
   đúng `04-components` §ModelCatalog; trước đó là “Chữ · Đọc ảnh”.
7. **Giá trong note của model** (`priceText` dùng chung cho catalog + picker):
   hàng khoá API ghi `$1,25 vào · $10 ra` / `$0,039 mỗi ảnh` thay vì lặp pill
   năng lực; subscription giữ note năng lực.
8. **Thẻ nguồn vẽ trên điện thoại có dòng giá**: trước chỉ hiện
   `$0,039 mỗi ảnh`, thiếu `≈ $0,27 mỗi video` bên phải. Nay tính cùng
   `drawMeta` như desktop; giá video làm tròn 2 chữ số (`usd2`) đúng artboard.
9. **Radio thẻ nguồn desktop 15/chấm 7** (`.agt-drawcard .agt-radio`), **radio
   trong sheet điện thoại 22** (`.agt-sheet .agt-opt .agt-radio`) — trước là 18/9
   và 18. Thêm token `--vv-s-15`.
10. **Ô “Đã kết nối” chỉ tính provider dùng được**: provider `auth_failed` /
    `unreachable` không khoá tile nữa, nên vẫn dán được khoá mới (artboard
    `conn=add` bật OpenRouter dù thẻ của nó `failed` ở frame khác).
11. **Hàng “+ Thêm” của chuỗi Đạo diễn có icon cộng** (`addIcon`) như artboard;
    `sheet=add` input điện thoại r12; hàng dashed điện thoại chữ 14.
12. `settings-ai.css` đã xoá từ vòng trước; CSS nằm ở `connect/connect.css` (316
    dòng) và `agents/agents.css` (248 dòng), không còn rule chết (đã bỏ
    `.conn-err__acts .conn-act--warn`, metric của `.conn-act`, `.conn-m__acts`).
13. **`ChatGPTLoginAction` nhận `size`** (mặc định `'compact'`): Cài đặt cần 32
    (`grok=code` — nút `Mở trang xAI` là `.primary` 32/r8/13), còn màn Chuẩn bị
    máy cần 36/r10/13.5 (`Onboard?setup=login`). Onboard truyền `size="md"` ở call
    site của nó; mặc định giữ nguyên nên ảnh Cài đặt không đổi.

## 3. File đã đổi

Vòng này:

- `apps/viderse/web/src/screens/settings/connect/{ProviderCard,ConnectSection,AddProvider,ModelCatalog,ChatGPTLogin}.tsx`
- `apps/viderse/web/src/screens/settings/connect/connect.css`
- `apps/viderse/web/src/screens/settings/agents/{AgentsSection,ModelPicker}.tsx`
- `apps/viderse/web/src/screens/settings/agents/agents.css`
- `apps/viderse/web/src/i18n/ai.ts` (`catalogFail`, `caps.vision` thường, `subSignedIn` nhận cụm gói)
- `apps/viderse/web/src/styles/viderse.css` (`--vv-s-15: 15px`)

Vòng trước (vẫn trong diff): `api/ai.ts`, toàn bộ `connect/**`, `agents/**`,
`i18n/ai.ts`; `settings-ai.css` đã xoá. Không đổi file Go nào trong vòng này
(không đụng handler GET, không đổi wire).

## 4. Đề xuất hỏi-đáp

1. **Thứ tự provider trong danh sách**: `created_at DESC` (mới nhất lên đầu) —
   đúng hai frame `conn=added` / `conn=failed` (thẻ vừa thêm/vừa lỗi nằm trên),
   nhưng `conn=list` của artboard xếp ChatGPT, Grok, Gemini, Ollama (cũ trước).
   Giữ “mới nhất trước”, hay đổi thành “lỗi trước, còn lại cũ trước”?
2. **Provider lỗi có khoá lại được không**: hiện tile vẫn bật (dán khoá mới tạo
   thẻ thứ hai cùng kind). Chấp nhận thẻ trùng, hay chặn và bắt dùng `Đổi khoá`?
3. **Ghi chú model**: artboard có câu biên tập theo model (“Trong gói · viết và
   đọc ảnh tốt nhất”, “Miễn phí · chậm hơn”); wire chỉ có caps/context/giá nên
   note = giá hoặc “chữ · đọc ảnh · 272K ngữ cảnh”. Có muốn bảng câu theo model id?
4. **`spendMonth`**: “tháng này $1,84” cần pipeline ghi giá mỗi lần gọi (chưa có).
5. **Giờ đặt lại hạn mức**: artboard ghi 07:00; provider không báo mốc nên app
   đang dùng nửa đêm địa phương (fixture mới ra đúng 07:00). Cần cấu hình?
6. **Giá video = 7 ảnh**: `≈ $0,27 mỗi video` đang là `perImage × 7` (artboard
   khớp với 0,039 × 7). Xác nhận 7 hay lấy từ cấu hình?
7. **4 nguồn vẽ**: kho có nguồn thứ 4 (Tương thích OpenAI) nên lưới 3 cột xuống
   dòng 2; artboard chỉ có 3 (ChatGPT/Grok/Gemini). Để xuống dòng, hay cuộn
   ngang / cắt còn 3?
8. **Nút primary disable khi ô khoá trống** (sheet thêm provider): artboard luôn
   có giá trị sẵn nên không thấy state này. Giữ disable (đúng hành vi) hay cho
   bấm rồi báo lỗi?

## 5. Kiến trúc & hiệu năng (số đo trên build cuối)

- `npx tsc --noEmit`: sạch. `npx vitest run --maxWorkers=2`: **148/148** (15 file),
  gồm `design-rules` (6/6: không hex cứng, không radius/margin số cứng).
- `npm run build`: OK. `npm run size`: **126.1 kB gzip** JS lần đầu (1 chunk,
  ngưỡng 250). `index-*.js` 420 kB raw / 128.9 kB gzip.
- Chunk theo mục (raw/gzip): `ConnectSection` 12 kB/3.0 kB,
  `ProviderCard` 24 kB/6.2 kB, `AgentsSection` 20 kB/5.5 kB,
  `ConnectSection.css` 16 kB/2.6 kB, `AgentsSection.css` 12 kB/2.1 kB — cả hai
  section vẫn là lazy chunk của Settings.
- Hàng DOM: danh mục model ≤ 200 hàng/trang (artboard 6, app 4 thật); sheet model
  ≤ 12 hàng; danh sách provider = số provider (≤ 8 kind + compat); picker chỉ render
  model của provider đã chọn. Tìm kiếm vẫn debounce, không list nào vượt 200.
- Không đụng handler GET nào ⇒ p95 không đổi; không thêm spawn/tree-walk trong
  GET. SSE chỉ invalidate `keys.aiProviders()` / `keys.aiProviderModels()`.
- Go (không sửa nhưng chạy lại vì cây làm việc có file phase-02):
  `go vet` sạch; `go test -p 2 ./internal/aiconn ./internal/secretbox
  ./internal/web ./internal/domain ./cmd/viderse` ok; `./archtest` ok;
  `go test -race -p 1` trên `aiconn/web/cmd-viderse` ok.

Status: DONE
Summary: Đã đối chiếu lại toàn bộ state connect/agents (desktop 1440×900 + điện
thoại 390×844) bằng cặp ảnh trên build thật rồi sửa nốt phần lệch: nút theo đúng
class artboard (`compact` 32 cho desktop, `lg` 44 cho thẻ điện thoại, `xl` 48 cho
CTA sheet, `warnGhost` hổ phách cho “Đổi khoá”, `ConfirmDialog size="compact"`),
thẻ nguồn vẽ điện thoại có dòng `$0,039 mỗi ảnh / ≈ $0,27 mỗi video`, pill năng
lực “Chữ · đọc ảnh”, giá model vào note của catalog/picker, radio 15 (thẻ nguồn
desktop) và 22 (sheet điện thoại), thẻ `failed` chỉ còn hộp lỗi (tách `catalogFail`
khỏi `catalogEmpty`) và ô “Đã kết nối” chỉ tính provider dùng được. tsc sạch,
vitest 148/148, build + size 126.1 kB gzip, Go vet/test/archtest/race xanh.

## 6. Sửa theo QA3-Connect (vòng sau)

1. **`conn=added` hiện thanh Huỷ/Lưu ngay** (`ConnectSection.tsx`): thêm cờ
   `unsavedNew` — thẻ vừa kết nối tự thân nó là “1 thay đổi chưa lưu” kể cả khi
   chưa tick model nào (đúng artboard `conn=added`). `Lưu` PATCH thật kể cả
   selected rỗng (ghi nháp lần kết nối); `Huỷ` gỡ cả nháp lẫn cờ. Thẻ mới mất
   khỏi list (xoá/refetch) thì không tính nữa — không treo thanh. Test mới:
   `conn=added: thanh Huỷ/Lưu hiện NGAY khi chưa tick model nào; Lưu ghi nháp;
   Huỷ gỡ thanh`.
2. **Checkbox desktop về NATIVE** (`ModelCatalog.tsx`, `connect.css`): bỏ ô vẽ
   tay `.conn-check`; input thật `accent-color: var(--color-accent)` +
   `color-scheme: light` để ô trống mặt trắng, ô chọn đỏ đặc + dấu trắng đúng
   artboard. Ô 22px vẽ tay của sheet điện thoại giữ nguyên (QA3 xác nhận đã đúng).
3. **Mở picker không cuộn trang** (`ModelPicker.tsx`): `focus({preventScroll:true})`
   cho `.agt-pop`, và thay `scrollIntoView` (cuộn cả trang) bằng chỉnh `scrollTop`
   thủ công của `.agt-pop__body` — hàng đang chọn vẫn vào khung nhìn.
4. **`m-add` sát số đo artboard** (`connect.css`): `conn-add__field` điện thoại
   gap 12 (label cách input 12); `.conn-sheet-primary` đệm dưới `12 + 26` (dưới
   nút chính còn 12 + thanh home 26 = 38 trước đáy sheet) → title y≈295, primary
   y≈758 ở 390×844 (trước 312/770).
5. **Chân sheet chọn model điện thoại là chữ trần** (`AgentsSection.tsx`): bỏ
   icon chìa khỏi `agt-sheet-link` — artboard MSettings `sh.foot` chỉ có text.
6. **Câu trần tự tiêu** (`i18n/ai.ts`): desktop “máy tự tiêu tối đa 50 ảnh/ngày”,
   điện thoại “tự tiêu tối đa 50/ngày” (`planPaid` / `planPaidMobile` mới, dùng
   trong `drawMeta` đã refactor của Main).

Header co 55→48px (QA3 §46) thuộc khung — FidelityMachine đã sửa `.vv-header
flex-shrink: 0`, không chạm file Connect.

Kiểm chứng: `tsc --noEmit` sạch; `vitest run --maxWorkers=2` **167/167** (15 file,
design-rules 11/11). Không build lại dist (dist do FidelityMachine điều phối).

Status: DONE
Summary: Đã sửa hết 5 lỗi QA3-Connect thuộc mình: thanh chưa lưu hiện ngay sau khi
kết nối (kể cả 0 model, Lưu ghi nháp thật), checkbox desktop về native mặt trắng/
đỏ trắng tick, mở picker không kéo trang, sheet m-add đúng nhịp 295/758, chân
sheet điện thoại bỏ icon chìa, copy trần tiêu tách bản điện thoại; regression
test cho luồng add; tsc sạch, vitest 167/167. Header 55px do FidelityMachine.
