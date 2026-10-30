# SettingsWebA — khung Cài đặt, Công cụ, Lưu trữ, Nhật ký, Chuẩn bị máy

## Đã làm

**Khung + mobile**
- `screens/settings/SettingsScreen.tsx`: export named `SettingsScreen({ route })` (App dùng) + default; desktop 3 cột (mục trái 248px / nội dung 760px / thẻ “Khi nào có hiệu lực” 320px) đúng artboard `Settings`; `?section=` routing, legacy `?section=images` → `agents` (router `LEGACY_SECTIONS`).
- Chấm mục: đọc `useStatus().data.attention`, parse `section=` trong `href`, chấm cam (`vv-dot--warn`) cạnh tên mục.
- Mobile (`<768px`): lớp phủ toàn màn `.vv-mset` (position fixed, z-index token `--vv-z-overlay`), danh sách 7 hàng đúng `MSettings` (Máy này: Công cụ/Kết nối/Trợ lý; Dữ liệu: Tài khoản/Lưu trữ/Báo lỗi; Ở trang khác: Luồng chạy), hàng Báo lỗi tải thẳng zip như artboard; màn con có nút quay lại, không thanh tab dưới.
- Ba mục của SettingsWebB nạp qua `import.meta.glob('./*/*Section.tsx')` → khung biên dịch/chạy được kể cả khi tệp của B chưa đất; nay đã có `connect/ConnectSection.tsx`, `agents/AgentsSection.tsx`, `accounts/AccountsSection.tsx` nên hiển thị bình thường.

**Công cụ (`screens/settings/tools/ToolsSection.tsx`)**
- Sáu hàng theo *việc* (Tải video/Biên tập/Phụ đề/Đăng video/Trợ lý AI/Đăng nhập ChatGPT), một nhãn trạng thái, nút chỉ khi `missing|update|failed|needs_login|installing`, tiến độ qua `ProgressBar` + SSE `tools`/`activity` (invalidate `keys.tools()` + `keys.setup()`).
- `Đăng nhập ChatGPT` điều hướng `#/settings?section=connect`.
- `<details>` “Chi tiết” chứa phiên bản + đường dẫn/lỗi thô (V7: phiên bản chỉ ở đây).

**Lưu trữ (`screens/settings/storage/StorageSection.tsx`)**
- Thẻ thư mục dự án: tên ổ + loại (`volumeKinds`), dung lượng trống, đường dẫn mono cắt giữa (`midTrunc`, `title` đầy đủ), `Mở trong Finder` (`POST /api/storage/open`), `Đổi thư mục…`.
- Hộp chọn: `Dialog` desktop / `Sheet` mobile; duyệt thư mục phía server (`GET /api/storage/browse?path=` → `{path, roots, dirs}`, hỏi đáp #10), gợi ý ổ từ `roots`/`suggestions`, hai lựa chọn dữ liệu (`move` / `new_only`), checklist từ `POST /api/storage/check` với khoá + lý do (`disabledReason` = câu lỗi/cảnh báo đầu tiên).
- Chuyển: tiến độ theo nhóm từ task `storage_move` của `/api/activity` (SSE `activity`), `Huỷ chuyển` có xác nhận cam.
- Xong: banner xanh + `Xoá bản cũ ngay` + `Chuyển về …`, cả hai có xác nhận cam inline. Offline: banner lỗi + `Thử lại` (`/api/storage/recheck`) + `Lưu video mới vào …`.
- Biểu đồ dung lượng: thanh xếp + legend (sources/exports/editor/images + stay), `Ổ còn trống`.
- Dọn dẹp hai bước `preview → run` với `previewId`; xoá nhóm (`src`/`mp4`) mỗi thẻ một xác nhận cam inline. Mọi xoá đều qua xác nhận cam.

**Nhật ký (`screens/settings/logs/LogsSection.tsx`)**
- Thẻ `Xuất log` (một nút primary, tải `/api/tools/logs/export`), `Lỗi gần đây` từ `/api/errors` (time/what/why/status/tone/href/link), thẻ phiên bản `GET /api/version` + `Kiểm tra bản mới` (`POST /api/version/check`).
- Trình xem log lọc công cụ + mức (mức = ngưỡng tối thiểu), `limit=200` (server-side, ≤200 dòng DOM), chọn công cụ lấy từ `/api/tools`.

**Chuẩn bị máy (`screens/setup/SetupScreen.tsx`)**
- `#/setup` (router + App `case 'setup'`), lớp phủ riêng theo `Onboard step=setup`: sáu hàng với %, phase; tổng `x/6` + progress; bước lỗi = một dòng cam + `Thử lại` trong khi bước khác chạy tiếp; ChatGPT `needs_user` → `ChatGPTLoginAction` (lazy, `variant` primary khi là bước kế tiếp, ghost khi đang cài).
- Xong → tự chuyển `#/channels/new` + nút primary “Máy sẵn sàng”.
- Mobile: “Mở Viderse trên máy tính”.
- AppShell: desktop mở `#/setup` một lần mỗi phiên khi `/api/setup` chưa `complete` (sessionStorage `viderse:setup-checked`), không lặp khi người dùng bỏ qua.
- Gear header: `href`/click trỏ tới `#/settings?section=…` của attention đầu tiên (nếu có).

**Kit (additive, Noverse không đổi)**
- `platform/ui/src/Banner.tsx`: thêm `actions?: {label, onClick, variant?, icon?}[]` (giữ `action` cũ; `action` nay optional) để banner cần hai nút (offline: Thử lại + Lưu video mới).
- Phần còn lại dùng đồ có sẵn: `ProgressBar`, `RadioCard`, `Segmented`, `Sheet`, `Dialog`, `KitSelect`, `useAsyncAction`, `EmptyState` — không thêm primitive nào khác.

**Dữ liệu/API**
- `api/{tools,setup,logs,storage,activity,errors,version}.ts` typed client theo wire MachineSetup/Storage xác nhận.
- `data/keys.ts`: `tools, setup, storage, storageBrowse, activity, logs, appVersion, errors`.
- `data/eventKeys.ts`: thêm `tools`, `activity`, `storage` (`tools → tools+setup`, `activity → activity+setup+storage`, `storage → storage+activity`).
- `i18n/{settings,tools,storage,logs,setup}.ts`, merge vào `vi.ts` (`settings/tools/storage/logs/prep`).
- `styles/viderse.css`: thêm bước `--vv-s-{9,28,30,32,36}`, `--vv-r-2`, `--vv-set-side-w/help-w`, `--vv-warn-*`, `--vv-info-*`, `--vv-z-overlay`.

## Vòng review (Review-p2-web — các mục của mình)

- **H1** `request.ts` + `mutate` gộp một transport; POST/PUT/PATCH luôn gửi body JSON
  (`{}` khi rỗng) + `Content-Type`, đúng gateway.
- **H2/M9** auto-start chỉ `state === 'idle'` → POST `/api/setup/start` một lần
  (server-side effect-free GET; contract mới từ MachineSetup); desktop only; thêm
  `checkSetup` → `POST /api/setup/check` (`?fresh` đã bỏ).
- **H3** `GET /api/storage` giờ có `move:{active,taskId,failed}` — thẻ lỗi riêng
  `FailedMoveCard` (`Thử lại` → `POST /api/storage/move/retry`, `Gác` → `POST
  /api/storage/move/dismiss`); activity filter chỉ `running|waiting|paused`.
- **D1** kit `useAsyncAction`: `busy`/`pending` reset trong `finally` + test
  regression (`platform/ui/src/useAsyncAction.test.ts`, 4 tests).
- **M2** folder browser chuyển sang phân trang cửa sổ trượt ≤100 dòng + Trước/Sau.
- **M6** radiogroup `move|new_only` roving tabindex + mũi tên.
- **M7** hint/error server chỉ trong Chi tiết (`<details>`); hàng chính hiện copy
  địa phương theo trạng thái.
- **M7a** `aria-live="polite"` cho `MoveProgress` + hàng `installing`.
- **M8** gear đã link tới section attention đầu tiên (đã có từ trước).
- **M9** low-disk card + `Kiểm tra lại`; `viderse:setup-skipped` khi bấm Quay lại
  chưa xong → AppShell không ép quay + banner "Máy chưa sẵn sàng" bền; check
  abortable + chỉ đánh dấu checked khi fetch thành công.

## Vòng bổ sung (theo yêu cầu của Main)

- **Nạp mục của SettingsWebB**: bỏ `import.meta.glob`, dùng `lazy(() => import(...))` thường cho `connect/agents/accounts` — build tách chunk bình thường, không còn cơ chế dự phòng.
- **Phân trang nhật ký phía server**: `GET /api/tools/logs?tool=&level=&limit=200&cursor=<đục>` → `{entries, nextCursor, nextBefore?}` (mới nhất trước; `limit` chặn trên 200; `level` = ngưỡng tối thiểu). UI giữ ngăn xếp con trỏ: `Cũ hơn` / `Mới hơn`, mỗi trang ≤200 dòng DOM, toàn bộ nhật ký vẫn với tới. Không còn `limit` tuỳ tiện phía client.
- **Đường dẫn setup**: `GET /api/setup/state` (platform/webapp giữ `GET /api/setup` cho luồng thiết lập tài khoản `{needed, attemptsLeft}`); `POST /api/setup/start|retry` giữ nguyên, mọi POST đều gửi `{}` + `Content-Type: application/json` (helper `request` tự đặt header khi có body).
- **Hộp chọn thư mục**: cửa sổ 100 dòng + `Tải thêm · còn N` (mở rộng dần tới hết), ô lọc tên thư mục debounce 250 ms; thư mục nghìn mục không nhồi DOM và không mất mục nào.
- **Xoá nhóm**: invalidate cả `storage` và `['storage-reclaim']` sau khi xoá.
- **Sửa theo test**: dòng “Có bản mới” chỉ nêu bản mới (bản đang dùng chỉ ở “Chi tiết”) đúng V7; thẻ phiên bản hiện ngay kết quả `POST /api/version/check` (state cục bộ + `setData` + invalidate).
- **Hợp đồng Storage cuối**: `POST /api/storage/move` → `{taskId, done}` — `done:true` (new_only/đã ở đích) đóng hộp chọn + invalidate, không chờ SSE; `done:false` để mục Lưu trữ hiện tiến độ theo nhóm. `reclaim` trả chính item (không `freedBytes`) và **vẫn render card khi `count/bytes = 0`** với nút khoá + lý do (server chưa có bảng video). Cleanup 409 `preview_required` → tự xem lại preview + câu “Danh sách vừa đổi…”. Check 404 `volume_gone` → banner “Ổ chưa cắm hoặc đã rút” thay cho bảng check. Offline đổi câu theo `volume.kind` (`internal` → “Không thấy thư mục dự án”, ngoài → “Không thấy ổ <tên>”). `projects.previous.mode` quyết định banner: `move` → “Đã chuyển … GB sang …” có `Xoá bản cũ ngay`/`Chuyển về`; `new_only` → “Video mới lưu ở …” **đầy đủ path + GB** (server trả `previous` cả cho new_only) và **không có nút xoá** vì server trả 409 `not_deletable` — bản cũ vẫn là dữ liệu đang đọc. `previous.deleteAfter` vắng → câu “máy không tự xoá” thay cho mốc ngày. `lastSwitch` chỉ là dự phòng khi `previous` đã bị xoá.

## Kiểm chứng

- `npx tsc --noEmit -p tsconfig.json` (apps/viderse/web) → **sạch** (0 lỗi).
- `npm run build` → **pass**; `SettingsScreen` 2,35 kB gzip, `StorageSection` 4,48, `ToolsSection` 1,62, `LogsSection` 1,66, `SetupScreen` 1,88; `index` 127,48 kB gzip (< 250 kB).
- `vitest run src/screens/settings/SettingsScreen.test.tsx` → **6/6 pass** (mục mặc định, ?section=storage, alias `images`→agents, chấm attention, thẻ phải, mobile không tab bar).
- `vitest run src/screens/setup/SetupScreen.test.tsx` → **5/5 pass** (thứ tự 6 bước, chặn nút primary khi chờ đăng nhập, bước lỗi một dòng cam + Thử lại trong khi bước khác chạy, xong → Máy sẵn sàng, mobile nhắc mở máy tính).
- **Test trạng thái theo fixture đúng wire** (mới): `ToolsSection.test.tsx` 6/6 (update → một nút “Cập nhật” + POST install, installing → 42% + progressbar, failed → “Thử lại”, ready → không nút, phiên bản chỉ trong “Chi tiết”); `StorageSection.test.tsx` 12/12 (thêm `done` và `donenew` đầy đủ) (here → Finder/Đổi thư mục + biểu đồ, offline → banner + Thử lại/Lưu video mới, pick+lowspace → checklist khoá CTA kèm lý do cam + POST move `mode`, cleanup preview→run đúng `previewId`, xoá nhóm qua xác nhận cam: Huỷ không gọi, xác nhận mới gọi); `LogsSection.test.tsx` 4/4 (lọc gửi `limit=200`, con trỏ `before` + Cũ hơn/Mới hơn, version check POST + hiện bản mới, lỗi gần đây + link).
- `vitest run` toàn bộ web (chạy lại sau `/api/setup/state`, `cursor`, hợp đồng Storage cuối + `new_only previous`) → **15 tệp / 114 test pass** (gồm `checks/design-rules` 6/6 và `shell` 13/13).
- `npm run build` → ✓; `npm run size` → **124,5 kB gzip** cho JS tải lần đầu, 1 chunk (ngưỡng 250 kB). Settings/Setup là chunk riêng: `SettingsScreen` 2,22 · `StorageSection` 4,77 · `LogsSection` 1,85 · `ToolsSection` 1,62 · `SetupScreen` 1,88 kB gzip.
- Số request mỗi lần mở mục Lưu trữ: 1 `GET /api/storage` + 1 `GET /api/activity` + 1 `GET /api/storage/reclaim`; mỗi trang nhật ký: 1 `GET /api/tools/logs` (+1 `GET /api/tools` cho danh sách bộ lọc, dùng chung khoá `tools`).
- `vitest run src/checks` → 3/6 fail, **toàn bộ vi phạm thuộc `screens/settings/{accounts,agents,connect}/*` và `settings-ai.css` của SettingsWebB**; 0 vi phạm trong tệp của tôi (đã lọc theo path).
- Chưa chạy: screenshot artboard đối chiếu, đo p95 GET bằng backend thật.

## Còn thiếu

1. Ảnh so artboard `Settings`/`MSettings`/`Onboard` trên backend thật (port 18795, DB tạm) — chưa chụp được.
2. Trạng thái `moving` và `cloud` chưa có test riêng (moving cần task `storage_move` trong `/api/activity`); `here/offline/pick/lowspace/cleanup/reclaim/volume_gone/done/donenew` đã có test.

## Lệch thiết kế / còn mở

1. **Chưa có test** (`*.test.tsx` cho 4 mục + khung + setup) và **chưa có ảnh so artboard**. Đây là phần còn thiếu lớn nhất.
2. Storage chưa có `mode` trên `previous` → `donenew` và `done` hiện cùng một banner; tôi đã thêm `mode?: 'moved'|'kept'` vào client và chờ Storage trả (đã nhắn). Hiện code dùng `DoneBanner` cho mọi `previous`, `DoneNewBanner` đã viết sẵn nhưng chưa nối.
3. `Mở trong Finder` là no-op thật trên máy không có Finder/không phải macOS — server trả `{ok}`; UI không giả lập.
4. Nhãn số đếm mục trái: `6`, `3` là hằng theo artboard (chưa nối số thật từ API).
5. Footer “Cài đặt toàn cục · bản …” đang để trống phiên bản (chưa có field app version trong `/api/status`); thẻ phiên bản ở Nhật ký mới là nơi hiện bản thật.
6. `ChatGPTLoginAction` lazy qua `import.meta.glob` (không import tĩnh) để tránh vỡ build khi B chưa đất; nay đã có file nên giữ nguyên cơ chế này cũng không sao.

## Kiến trúc & hiệu năng (theo yêu cầu của Main)

- Settings và `#/setup` đều là chunk lazy (`lazy(() => import(...))` trong `App.tsx`); các mục bên trong cũng `lazy` riêng.
- Một request cho Lưu trữ (`GET /api/storage` gộp usage/clean — Storage đã gộp), một request cho activity, một cho reclaim; nhật ký `limit=200` server-side, không render quá 200 dòng DOM; không có timer tự tạo, mọi làm mới đi qua SSE invalidate đúng khoá.
- Tiến độ dùng prop (`value`, text) — không remount hàng; `useAsyncAction` chặn bấm lặp và giữ spinner ≥400 ms.
- Chưa đo số cụ thể (GET latency, số dòng DOM, sự kiện/giây) vì hết lượt trước bước chạy server thật.

## Trạng thái sau vòng review (cập nhật)

- Tệp của tôi: `tsc` sạch; vitest 108/114 pass — 6 fail còn lại đều trong
  `connect/` + `agents/` (SettingsWebB đang refactor `ModelPage`/ProviderCard).
- `npm run build` hiện gãy vì `connect/ProviderCard.tsx` (`capTags`,
  `ModelCatalog` chưa khai) — SettingsWebB đang sửa dở; `npm run size` vẫn
  **124,5 kB gzip** 1 chunk.
- `go build ./...` gãy vì `cmd/viderse/wire_ai.go` (AIConn, `key` unused) —
  chưa chạy được smoke backend cho bước chụp artboard.
- Review đã xử lý trong code: H1, H2, H3, D1, M2, M6, M7, M7a, M8, M9 (chi tiết
  ở mục “Vòng review”).
- Mở: screenshot artboard đối chiếu + test `storage` state `moving`/`cloud`
  chuyên biệt (cần backend compile được).
