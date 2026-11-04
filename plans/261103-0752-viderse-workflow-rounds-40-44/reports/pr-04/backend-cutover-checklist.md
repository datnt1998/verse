> **Parent review · 04/11/2026: bản nghiên cứu, KHÔNG phải authority thực thi.** Các chỉ dẫn bỏ AutoFix/retry ở §1, §3.4, bảng caller và rủi ro đã bị bác bằng nguồn `Workflow.dc.html:980`: “Máy luôn tự thử sửa lỗi1lần”. Giữ cơ chế thử sửa một lần/settlement/idempotency, chỉ bỏ cấu hình công khai. Giới hạn1–20 cũng chờ design #86 vì board tạo desktop/mobile còn1–50. Trước triển khai phải theo phase05/backend-contracts đã cập nhật, không thực hiện các đoạn SQL/retirement bị đánh dấu này.

# V38 · Checklist backend cutover (Kênh / Thư viện / Performance)

Lập ngày 2026-11-04. Phạm vi: `apps/viderse` backend + schema + contract API. **Chưa sửa source, chưa chạy migration/DB/build/test.** Đây là bản nghiệm thu kỹ thuật để thi công ngay sau khi V37 merge; mọi số dòng là bằng chứng source tại `93545e71` + working tree.

Điều kiện vào: V37 (`refactor/viderse-v37-studio`) đã merge/accepted. DB/FE/BE phải lên **cùng revision**; không alias/shim cho autonomy, priority, rotation hay performance reads.

---

## 1. Quyết định hợp đồng đã chốt (không mở lại)

| Hạng mục | Chốt | Nguồn |
|---|---|---|
| Trường công khai của kênh | `defaultTemplateId` (bắt buộc), `autoApprove: bool`, `autoFetch: bool`, `perDay 1–20` | `08-dev-prompt.md:1079`, `backend-contracts.md` bảng V38 |
| Bỏ | `autonomy` (pick/approve/onError/below/min), `priority` (steady/views), `templateCursor`, `templates[]` luân phiên | `08-dev-prompt.md:1078-1082`, `backend-contracts.md:15` |
| Ngưỡng tự duyệt | Cố định **85**; `autoApprove=true` chỉ khi **mọi** ảnh (không `skipped`) có điểm **≥85**; thiếu điểm/chưa sẵn sàng/đang vẽ/lỗi/đã bỏ → không tự duyệt | `backend-contracts.md:15`, `08-dev-prompt.md:1079` |
| `onError` / `belowQuality` | Bị bỏ cùng `autonomy` (không còn cấu hình). Hệ quả: **không còn auto-retry `AutoFix`** và **không còn `fix_then_ask`**; job lỗi về Cần xử lý như trạng thái `ask/stop`. Không hardcode `retry_once` để "giữ hành vi" — đó là auto-enable không có trong thiết kế. Phải ghi release note + đếm số kênh bị ảnh hưởng ở preflight | `08-dev-prompt.md:1079` (liệt kê onError/below trong phần bị thay), `code`: `autonomy/run.go:83-130`, `domain/autonomy.go:88-92` |
| `autoApprove` từ `approve=all` | Thành `true`, nhưng mất quyền "duyệt không xét điểm": sau cutover vẫn phải ≥85. Đây là thu hẹp automation (an toàn), không phải mở rộng | `08-dev-prompt.md:1082` + 1079 |
| Template mặc định | Chuyển giao từ **rotation đầu tiên theo `position`, tiebreak `template_id`**; không cursor, không weight, không tên fixture | `backend-contracts.md:26` |
| Default chia sẻ | Nhiều kênh được cùng một template làm mặc định; **không** cấm RenameTemplate dùng chung. Guard `NameChannelStandardTemplates` chỉ để không tự đổi tên template người dùng/đa kênh | `backend-contracts.md:29`, `library.sql:58-69` |
| Snapshot job | Job đã persist giữ `template_snapshot` bất biến. Queue chỉ có `template_id` override tùy chọn, **không có recipe snapshot** để hứa giữ. Default chỉ được chốt trong **admission transaction**; explicit override (kể cả restart dùng template của job cũ) do người dùng/ngữ cảnh quyết định và giữ nguyên | `backend-contracts.md:26,35` |
| Performance sau đăng | Retire collector/Run, DataAPI + public-page readers, route đọc `GET /api/channels/:id/performance`, route áp dụng `POST /api/channels/:id/insights/:id/apply`, weight "Cho chạy nhiều hơn". Giữ mark-posted/export metadata + `sharedYouTubePacer` | `backend-contracts.md:13`, `phase-05:24` |

---

## 2. Schema migration (append mới; số lấy lúc thi công)

Hiện `internal/persistence/migrations` max = `00038_global_suggestions.sql`; **không hardcode 00039** — lấy số kế tiếp tại thời điểm PR. Một file duy nhất, chạy trong maintenance window đã quiesce (`backend-contracts.md:25`).

### 2.1 Thứ tự DDL bắt buộc

Theo convention repo (`00034_post_performance.sql`): mỗi `DO $$ … $$` phải bọc `-- +goose StatementBegin` / `-- +goose StatementEnd`; migration chạy trong một transaction, nên mọi `RAISE EXCEPTION` rollback sạch (không schema nửa vời) và temp table ở (F) drop cuối file.

```sql
-- +goose Up
-- (A) STOP nếu có kênh không có template luân phiên nào → không có default hợp lệ.
DO $$
DECLARE bad text;
BEGIN
  SELECT string_agg(c.id::text, ',') INTO bad
  FROM owner_channels c
  WHERE NOT EXISTS (SELECT 1 FROM owner_channel_templates t WHERE t.owner_channel_id = c.id);
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION 'V38 stop: channel không có template: %', bad;
  END IF;
END $$;

-- (B) STOP nếu per_day ngoài 1..20 (kể cả dữ liệu cũ 21..50).
DO $$
DECLARE bad text;
BEGIN
  SELECT string_agg(c.id::text || '=' || c.pace_per_day::text, ',') INTO bad
  FROM owner_channels c WHERE c.pace_per_day NOT BETWEEN 1 AND 20;
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION 'V38 stop: per_day ngoài 1..20: %', bad;
  END IF;
END $$;

-- (C) Cột mới (chưa NOT NULL để backfill trong cùng tx).
ALTER TABLE owner_channels
  ADD COLUMN default_template_id uuid,
  ADD COLUMN auto_approve boolean NOT NULL DEFAULT false;

-- (D) default = rotation đầu theo position, tiebreak template_id (KHÔNG cursor/weight/name).
UPDATE owner_channels c
SET default_template_id = (
  SELECT t.template_id
  FROM owner_channel_templates t
  WHERE t.owner_channel_id = c.id
  ORDER BY t.position, t.template_id
  LIMIT 1
);

-- (E) STOP nếu còn NULL (rotation rỗng lọt qua (A) do thay đổi giữa preflight và cutover).
DO $$
DECLARE bad text;
BEGIN
  SELECT string_agg(id::text, ',') INTO bad FROM owner_channels WHERE default_template_id IS NULL;
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION 'V38 stop: default_template_id NULL: %', bad;
  END IF;
END $$;

-- (F) Backfill 2 bool từ autonomy HIỆU LỰC (xem §3). Predicate §3.3 phải dán NGUYÊN VĂN vào
--     temp table dưới đây; không viết lại rút gọn. Temp table tự drop cuối transaction.
CREATE TEMP TABLE v38_resolved AS
SELECT c.id,
       COALESCE(
         jsonb_typeof(c.autonomy) = 'object'
         AND jsonb_typeof(c.autonomy->'pick') = 'string'
         AND c.autonomy->>'pick' IN ('suggest','score2','all')
         AND jsonb_typeof(c.autonomy->'approve') = 'object'
         AND jsonb_typeof(c.autonomy->'approve'->'mode') = 'string'
         AND c.autonomy->'approve'->>'mode' IN ('ask','score','all')
         AND jsonb_typeof(c.autonomy->'approve'->'min') = 'number'
         AND c.autonomy->'approve'->>'min' IN ('70','75','80','85','90','95')
         AND jsonb_typeof(c.autonomy->'onError') = 'string'
         AND c.autonomy->>'onError' IN ('stop','retry_once')
         AND jsonb_typeof(c.autonomy->'belowQuality') = 'string'
         AND c.autonomy->>'belowQuality' IN ('ask','fix_then_ask'),
         false) AS valid
FROM owner_channels c;

UPDATE owner_channels c
SET auto_approve = COALESCE(r.valid AND (c.autonomy->'approve'->>'mode') IN ('score','all'), false)
FROM v38_resolved r WHERE r.id = c.id;

UPDATE owner_channels c
SET auto_pick = c.auto_pick AND (NOT r.valid OR (c.autonomy->>'pick') IS DISTINCT FROM 'suggest')
FROM v38_resolved r WHERE r.id = c.id;

DROP TABLE v38_resolved;

-- (G) Đổi tên cột theo hợp đồng mới; dữ liệu giữ nguyên.
ALTER TABLE owner_channels RENAME COLUMN auto_pick TO auto_fetch;
ALTER TABLE owner_channels RENAME COLUMN pace_per_day TO per_day;

-- (H) Siết CHECK ngày. Tên constraint autogenerated phải verify trước ở QA:
--     SELECT conname FROM pg_constraint
--     WHERE conrelid = 'owner_channels'::regclass AND pg_get_constraintdef(oid) LIKE '%pace_per_day%';
--     Dùng DROP không IF EXISTS để lệch tên thì migration fail lớn tiếng (không để sót CHECK 1..50).
ALTER TABLE owner_channels DROP CONSTRAINT owner_channels_pace_per_day_check;
ALTER TABLE owner_channels ADD CONSTRAINT owner_channels_per_day_check CHECK (per_day BETWEEN 1 AND 20);

-- (I) Bắt buộc default + FK RESTRICT + index tra cứu ngược từ template.
ALTER TABLE owner_channels ALTER COLUMN default_template_id SET NOT NULL;
ALTER TABLE owner_channels
  ADD CONSTRAINT owner_channels_default_template_fk
  FOREIGN KEY (default_template_id) REFERENCES templates(id) ON DELETE RESTRICT;
CREATE INDEX owner_channels_default_template_idx ON owner_channels (default_template_id);

-- (J) Bỏ cột/ bảng luân phiên khi không còn caller.
ALTER TABLE owner_channels DROP COLUMN autonomy, DROP COLUMN priority, DROP COLUMN template_cursor;
DROP TABLE owner_channel_templates;  -- weight (00034) rơi theo bảng

-- (K) Bỏ bảng chỉ phục vụ collector đã retire.
DROP TABLE clone_post_stats;
DROP TABLE performance_insights_applied;
```

Ghi chú:
- `clone_post_facts` **giữ** làm owner metadata mark-posted (link YouTube + title/duration lịch sử). Không drop cột trong V38; cột `read_attempts/next_read_at/published_at` thành legacy read-only, cắt ở migration retention riêng nếu chủ dữ liệu duyệt.
- `clone_jobs.auto_fix` và `clone_auto_actions` kinds `auto_retry/auto_fix` **giữ** để đọc lịch sử; ngừng ghi mới. Không đổi CHECK constraint (sẽ mất lịch sử).
- Không sửa `internal/persistence/postgres/gen/models.go` bằng tay: regenerate `sqlc generate -f apps/viderse/sqlc.yaml` sau khi sửa `postgres/queries/*`; giữ nguyên mọi thay đổi working-tree của người dùng trong file generated.
- Down: chỉ là throwaway cho DB rỗng/dev — drop FK/index/cột mới, không thể tái tạo rotation/weight/stats/applied đã mất. **Không claim Down lossless**; production rollback = restore backup đã kiểm khi mọi writer còn quiesced (`backend-contracts.md` mục Xác minh và rollback).

### 2.2 Preflight read-only (chạy trước maintenance; chỉ báo ID nội bộ)

| Kiểm tra | Query / nguồn | Hành động |
|---|---|---|
| Kênh + rotation | `SELECT c.id, count(t.template_id) FROM owner_channels c LEFT JOIN owner_channel_templates t ON t.owner_channel_id=c.id GROUP BY c.id` | count=0 → **STOP**, ghi câu hỏi kèm số cho chủ (mục hỏi-đáp của plan), không tự tạo template/default |
| Dangling | `LEFT JOIN templates tp ON tp.id=t.template_id WHERE tp.id IS NULL` (FK hiện đã chặn; vẫn assert) | có dòng → **STOP** |
| Trùng position | `GROUP BY owner_channel_id, position HAVING count(*)>1` | không stop; backfill dùng tiebreak `template_id` (ổn định), ghi nhận ID |
| `per_day` | `WHERE pace_per_day NOT BETWEEN 1 AND 20` (gồm >20) | có dòng → **STOP**, báo số + ID; không clamp |
| Template default dự kiến | truy vấn (D) + `JOIN templates` | thiếu template tương ứng → **STOP** (không fake ID) |
| Default đọc được | `SummaryStats`/`templateIssue` hiện có + đọc media refs của template default (không chạy `SnapshotTemplate` trên production) | lỗi media/recipe ở default → báo kênh + template; nếu không sửa được trước cutover (kênh sẽ không tạo được job) → **STOP**; không fake media |
| `auto_approve` dự kiến | predicate §3.3, nhóm `score/all` | báo số kênh; đặc biệt `mode=all` (mất duyệt-không-điểm) và `min≠85` (ngưỡng đổi) |
| `auto_fetch` hiệu lực | `auto_pick AND (NOT valid OR pick<>'suggest')` | báo số kênh bị chuyển true→false (pick=suggest) |
| AutoFix/belowQuality | `valid AND onError='retry_once'` / `belowQuality='fix_then_ask'` | báo số kênh; release note hành vi bị bỏ (§1) |
| Queue/job đang sống | `clone_queue state='queued'`, `clone_jobs status IN ('pending','running')`, `clone_queue.template_id IS NOT NULL` | **không stop**; chỉ chứng minh explicit IDs giữ nguyên, snapshot `clone_jobs.template_snapshot` không bị ghi đè |
| Pending review | `pg_stat`-free read qua đúng SQL `pendingReviewsSQL` hiện tại | báo số job sẽ được xét lại sau cutover; không requeue |
| Notification prefs | không thuộc V38; không đụng | — |

Mọi preflight chỉ đọc; không UPDATE để "làm sạch" dữ liệu production.

---

## 3. Truth table autonomy → bool (đã chốt)

### 3.1 `autoFetch`

Nguồn hiệu lực hiện tại: `web/channels.go:154` (`AutoPick && Autonomy.AutoPicks()`), discovery `autopick.go:63`; `AutoPicks() = pick != "suggest"` (`domain/autonomy.go:147`); JSON hỏng/missing đọc như Cân bằng `pick=all` (`ParseAutonomy`, `domain/autonomy.go:160-172`).

| `auto_pick` | autonomy raw | `ParseAutonomy` | `AutoPicks()` | `autoFetch` sau cutover |
|---|---|---|---|---|
| true | missing / `null` / `{}` / sai kiểu / thiếu khóa / giá trị lạ | Cân bằng | true | **true** |
| true | hợp lệ `pick=suggest` | nguyên trạng | false | **false** |
| true | hợp lệ `pick=score2` \| `all` | nguyên trạng | true | **true** |
| false | bất kỳ (kể cả invalid) | — | — | **false** |

Tuyệt đối không bật `autoFetch` chỉ vì `pick` cho phép khi cờ đã tắt. Không suy missing thành "bật mới"; missing giữ đúng hành vi hiệu lực cũ.

### 3.2 `autoApprove`

Nguồn hiệu lực: `DispatchReviews` chỉ chạy khi `ParseAutonomy(...).Approve.Mode ∈ {score, all}` (`autonomy/run.go:33-37`); `pendingReviewsSQL` lọc thô `autonomy->'approve'->>'mode' IN ('score','all')` nhưng `reviewOne` dùng bản đã Parse/Validate (`postgres/autonomy.go:104-140`).

| autonomy raw | Validate (Go) | approve mode hiệu lực | `autoApprove` |
|---|---|---|---|
| missing / NULL / `null` / `{}` / sai kiểu | fail → Cân bằng | `ask` | **false** |
| thiếu 1 trong 5 khóa, hoặc `min` ngoài `{70,75,80,85,90,95}` / không phải số nguyên | fail → Cân bằng | `ask` | **false** |
| hợp lệ | pass | `ask` | **false** |
| hợp lệ | pass | `score` | **true** (ngưỡng sau cutover = 85, bất kể `min` cũ) |
| hợp lệ | pass | `all` | **true** (vẫn phải mọi ảnh ≥85) |
| bất kỳ `preset` | `preset` không được validate; `Normalize` tự suy lại | theo rows | theo rows |

### 3.3 Predicate SQL phải khớp `Autonomy.Validate()` (`domain/autonomy.go:104-133`)

```sql
WITH resolved AS (
  SELECT c.id,
         COALESCE(
           jsonb_typeof(c.autonomy) = 'object'
           AND jsonb_typeof(c.autonomy->'pick') = 'string'
           AND c.autonomy->>'pick' IN ('suggest','score2','all')
           AND jsonb_typeof(c.autonomy->'approve') = 'object'
           AND jsonb_typeof(c.autonomy->'approve'->'mode') = 'string'
           AND c.autonomy->'approve'->>'mode' IN ('ask','score','all')
           AND jsonb_typeof(c.autonomy->'approve'->'min') = 'number'
           AND c.autonomy->'approve'->>'min' IN ('70','75','80','85','90','95')
           AND jsonb_typeof(c.autonomy->'onError') = 'string'
           AND c.autonomy->>'onError' IN ('stop','retry_once')
           AND jsonb_typeof(c.autonomy->'belowQuality') = 'string'
           AND c.autonomy->>'belowQuality' IN ('ask','fix_then_ask'),
           false) AS valid
  FROM owner_channels c
)
```

Điểm dễ sai (phải có test fixture): `min: 85.0` / `"85"` / `85.5` / `null` → **invalid** (Go unmarshal vào `int` lỗi hoặc Validate lỗi); `approve` là array/string → invalid; khóa thừa → vẫn valid; `pick=1` (number) → invalid. Viết unit test Go cho `ParseAutonomy` + test SQL trên fixture QA để hai bên khớp tuyệt đối.

Giới hạn ngữ nghĩa: JSONB đã chuẩn hoá số nên không phân biệt được `85` với `8.5e1`; Go decoder từ chối `8.5e1` khi ghi qua API/CLI, nên dữ liệu hợp lệ chỉ có literal nguyên. Bắt buộc preflight đối chiếu: đọc toàn bộ `autonomy` của snapshot legacy, chạy `domain.ParseAutonomy` (read-only) và so **từng hàng** với kết quả predicate SQL; lệch 1 hàng → **STOP** (không chọn một bên). Nếu có hàng do ghi thẳng DB với literal lạ, xử lý theo mục hỏi-đáp của plan, không tự map sang true.

### 3.4 Bảng xử lý `onError`/`belowQuality` (không còn cột)

| Giá trị cũ | Sau cutover |
|---|---|
| `onError=stop` | không auto-retry (giữ nguyên hành vi) |
| `onError=retry_once` | **mất auto-retry**; step lỗi về Cần xử lý. Báo số kênh ở preflight + release note |
| `belowQuality=ask` | giữ |
| `belowQuality=fix_then_ask` | **mất tự đo lại một lần**; video dưới cổng về chờ chủ. Báo số kênh + release note |
| `job.auto_fix`, `clone_auto_actions` auto_retry/auto_fix | chỉ đọc lịch sử; bỏ `domain.AutoFix`, `AutoFixCodeOf`, `AutoFixer` khỏi runner, `Runner.AutoFix`, `newAutonomyService` wiring tương ứng |

Lý do không chọn "retry cố định": với kênh trước đây `stop`, hardcode retry là tự bật quyền tốn tiền/thời gian không có trong thiết kế. Nếu chủ sản phẩm muốn giữ retry, phải mở decision mới (không nằm trong V38).

---

## 4. Rotation → default, cursor, weight

1. Default chọn theo `ORDER BY position, template_id` — không cursor, không weight, không fixture name.
2. `template_cursor` bị drop; xoá mọi chỗ đọc/ghi:
   - `postgres/clone.go:569` (UPDATE `template_cursor=template_cursor+1` sau admission) — **xoá**.
   - `plan.go:100-108`, `restart.go:36-50,136-150`, `cmd/viderse/clone_wiring.go:135-150`, `queue/view.go:61,137-145`, `queue_rotation.go:13-40`.
3. Weight/cursor queries xoá khỏi SQLC: `SetTemplateWeight`, `ListTemplateWeights`, `ListChannelTemplateViews7d` (`queries/performance.sql:88-102`) + `performance.RotationSequence/RotationWeights/OrderByScore`.
4. Multi-template channel: template không phải default **vẫn nằm trong Thư viện** (không xoá media/template từng dùng). Chỉ mất liên kết kênh–template.
5. Create kênh:
   - Body có `defaultTemplateId` → validate tồn tại (`TemplateExists`), dùng template đó, **không** tạo Mẫu chuẩn, không đụng default kênh khác (shared hợp lệ).
   - Vắng `defaultTemplateId` → giữ luồng hiện tại: `CreateStandardTemplate` trước, gắn làm default trong cùng transaction; lỗi tx thì bù trừ xoá template như `channels.go:520-545`.
   - Template chỉ định không tồn tại → 404 `not_found` "Template không tồn tại", **không fallback** tạo Mẫu chuẩn.
6. Xoá template: `DELETE /api/templates/{id}` giữ RESTRICT; `library.DeleteTemplate` pre-check `TemplateChannels` phải đọc **default refs** để trả 409 kèm tên kênh (`library.go:637-644,732-740`). FK mới là backstop; race giữa check và delete trả lỗi FK → map 409, không 500. Queue/job `template_id` vẫn RESTRICT như hiện tại (00012:7,89).
7. Auto-naming: đổi 2 subquery của `NameChannelStandardTemplates` (`queries/library.sql:58-69`) từ `owner_channel_templates` sang `owner_channels.default_template_id` ($channel_id giữ nguyên điều kiện "chỉ kênh này dùng"). Không đụng `RenameTemplate` của người dùng.

---

## 5. Schema/query/API/caller migration list

### 5.1 SQLC queries

| File | Thay đổi |
|---|---|
| `queries/channels.sql` | `InsertOwnerChannel`: bỏ `autonomy, priority`; thêm `auto_fetch, auto_approve, per_day, default_template_id`. Template default đã tồn tại (tạo hoặc validate) trước INSERT trong cùng transaction, nên INSERT ghi thẳng đủ cột — không placeholder rồi UPDATE. `PatchChannel`: bỏ autonomy/priority; thêm `default_template_id_set/default_template_id`, `auto_approve_set/auto_approve`, `auto_fetch_set/auto_fetch`, `per_day_set/per_day`; giữ nhánh reset `auto_baseline` theo auto_fetch. Default validate `TemplateExists` trước UPDATE để 404; FK bắt race, map 409. Xoá `ListChannelTemplates`, `ListChannelTemplateIDs`, `InsertChannelTemplate`, `DeleteChannelTemplate`, `SetChannelTemplatePosition`; đổi `SetAutoPick*` → `SetAutoFetch*`. Thêm `LockOwnerChannelForAdmission :one` trả `default_template_id` + `FOR UPDATE`; `TemplateForAdmission :one` chọn `revision, recipe, name` `FOR SHARE`; `DefaultTemplateNames :many` (channel_id, name) cho queue view. `TemplateExistsForChannels` giữ. `UnlinkSource` trả cờ pause dựa trên auto_fetch. |
| `queries/library.sql` | `ListTemplates`/`GetTemplate` lateral join đổi sang `owner_channels c ON c.default_template_id = t.id`, JSON `{id,name}` (bỏ `position`). `TemplateChannels` đổi sang default refs, bỏ position. `NameChannelStandardTemplates` theo §4.7. Xoá mọi tham chiếu `owner_channel_templates`. |
| `queries/performance.sql` | Giữ `EnsurePostFacts`, `SetPostYoutube`, `GetPostYoutubeID`. Xoá `DeletePostStats`, `SetPostPublished`, `RecordPostReadFailure`, `ClearPostReadFailures`, `InsertPostStat`, `ListPostedJobsWithoutFacts`, `ListChannelPosted`, `ListChannelPostStats`, `ListPostedChannelIDs`, `ListReadCandidates`, `ListAppliedInsights`, `InsertAppliedInsight`, `SetTemplateWeight`, `ListTemplateWeights`, `ListChannelTemplateViews7d`. |
| `queries/rivals.sql` | `ListAutoPickBaselines` → `ListAutoFetchBaselines`, cột `c.auto_fetch`. |
| `queries/queue.sql` / `queue_prompts.sql` | Không đổi cấu trúc; `LockQueueOwner` giữ. Không xoá `template_id` của queue (explicit override). |

### 5.2 Go domain/store/service

| File | Việc |
|---|---|
| `domain/channels.go` | `Channel`: bỏ `Autonomy`, `Priority`, `TemplateCursor`; thêm `DefaultTemplateID string`, `AutoFetch bool`, `AutoApprove bool`, `PerDay int`; bỏ `ChannelTemplate.Position` (còn `ID,Name`). `domain/library.go`: bỏ `ChannelRef.Position` (refs = default refs). `ChannelSource`/identity giữ. |
| `domain/autonomy.go` | Bỏ toàn bộ preset/pick/approve/onError/below/Autonomy/Validate/Normalize/ParseAutonomy/AutoPicks/RaisesTrust/AutoFix; giữ `ImageScore*` + `ScoreReason`; thêm `AutoApproveMin = 85`. |
| `channels/channels.go` | `Patch/CreateInput/UpdateRequest/CreateRequest`: bỏ autonomy/priority/confirmed; thêm `DefaultTemplateID *string`, `AutoFetch *bool`, `AutoApprove *bool`, `PerDay *int`. Bỏ `UpdateTemplates` khỏi `Store` và toàn bộ `Templates()`/`TemplateAction`/`channelNameForTemplate`; thay bằng validate default tồn tại + patch. `ChannelView.Templates` → `DefaultTemplate *domain.ChannelTemplate` (hoặc ID+Name). `PaceMin/Max/Default` → `PerDayMin=1/Max=20/Default=5`. **Giữ** `nameStandardTemplates` (chỉ đổi query refs) cho rename kênh. Bỏ import/domain liên quan autonomy. |
| `channels/autonomy.go` | **Xoá file** (`ResolveAutonomy`, `autonomyInvalid`, `errConfirmRequired`, `validPriority`); xoá `CodeConfirmRequired`. |
| `postgres/channels.go` | Mapping owner row mới; create/patch params mới; `UpdateTemplates` xoá; link/unlink `SetAutoFetch`; default validation race map FK → 409; `UnlinkSource` trả `autoFetchPaused`. |
| `postgres/queue_rotation.go` | Xoá `RotationTemplates`, `RivalVideoScores`; thay bằng `DefaultTemplate(ctx, channelID)`/name helpers. |
| `postgres/autonomy.go` | `pendingReviewsSQL`: thay điều kiện autonomy bằng `c.auto_approve`; `PendingReview` bỏ `Autonomy`, thêm `AutoApprove bool`; `RecordReview` luôn ghi `min:85` vào `auto_review` (audit), `approveReason` chỉ còn nhánh theo điểm 85 và copy “theo thiết lập Tự duyệt của kênh”, xoá nhánh `ApproveAll`/text “Tự duyệt hết”; xoá `ChannelAutonomy/SetChannelAutonomy/JobAutonomy/BeginAutoFix/SettleAutoFix/RaiseCandidates` + `autoFixLabels`; thêm `ChannelAutoApprove(ctx, channelID)` để re-check toggle trước khi duyệt. |
| `autonomy/run.go` | `DispatchReviews`: `mode` → `p.AutoApprove`; `Evaluate(domain.AutoApproveMin, state.Items)`; trước `ApproveAll` đọc lại `ChannelAutoApprove`, false → bỏ qua; `ReviewOutcome` bỏ Autonomy, thêm AutoApprove để reason/notification. Xoá `AutoFix`/`AttemptSettled`. `autonomy/autonomy.go` trim Store interface tương ứng; vẫn giữ `Evaluate`, `Verdict`, `Summary`, `MaxReviewTries`, `Reviewer`. |
| `pipeline/runner.go` | Bỏ interface `AutoFixer`, field `AutoFix`, hai call-site `AttemptSettled/AutoFix` (dòng 55-58,93-98,206-214,444); giữ `CloneErrRetryable` (retry queue-level không phải autonomy). |
| `pipeline/queue/autopick.go` | Enabled = `o.AutoFetch`; bỏ `Templates`/empty-template message; bỏ nhánh `PickScore2`/`outliersOnly`; bỏ priority-views `OrderByScore`; bỏ import `internal/performance`. |
| `pipeline/queue/plan.go` | Template: explicit → validate `TemplateExists` (toàn Thư viện, không chỉ rotation); vắng → default của kênh; bỏ cursor/`FromRotation`, đổi `Template.FromRotation` → `FromDefault`; vẫn giữ `Priority` request (V39 mới bỏ). |
| `pipeline/queue/restart.go` | Preview + enqueue dùng **template của job cũ** (`job.TemplateID`; job legacy NULL thì lấy default hiện tại và ghi rõ nguồn) — explicit override, không cursor/rotation. `RestartPlan`/`EnqueueRestart` set `TemplateID` đó, `TemplateRecipe` đọc revision hiện tại; không snapshot recipe lúc enqueue (V39 mới bàn). Giữ `Priority=pick`, vị trí đầu, conflict checks. Job cũ template đã mất là bất khả thi (FK RESTRICT); nếu gặp → lỗi rõ, không fallback im lặng. |
| `pipeline/queue/service.go` | `loadLanes`: bỏ `Templates`/`TemplateOK` (FK bảo đảm default tồn tại); bỏ clamp `Pace<1`; auto gate `!owner.AutoFetch`; bỏ `no_template` skip. |
| `pipeline/queue/view.go` | Gate ẩn item tự động `owners[..].AutoPick` (dòng 42) → `AutoFetch`; Template name cho job đang chạy (dòng 61) và item chờ (137-145): default name theo channel, override name theo `TemplateNames`; bỏ cursor; bỏ nhánh `no_template`. |
| `pipeline/queue/schedule.go` | Bỏ `Lane.TemplateOK` + reason `no_template`; `Lane.Pace` giữ nguyên nghĩa nhịp ngày. |
| `pipeline/service.go` | `Start`: đổi resolver thành `SnapshotCandidate(ctx, templateID)`; admission protocol §6 (cursor chỉ từng được ghi trong store, không thêm ở đây). `PlanRestart/EnqueueRestart` wiring giữ. |
| `postgres/clone.go` | `StartJob`: luôn lock channel (không chỉ khi có queue item); đọc default/override dưới lock; so template ID với candidate; `FOR SHARE` template row + so revision; **bỏ UPDATE template_cursor**; map stale/moved thành sentinel retry (§6). |
| `rivals/rivals.go` + `postgres/rivals.go` | `AutoPickBaseline` → `AutoFetchBaseline`, field `AutoFetch`; logic dùng `c.auto_fetch`. |
| `library/library.go` | `DeleteTemplate` pre-check/`inUseByChannels` dùng default refs (message giữ nguyên); `SnapshotTemplate` giữ nguyên (không đổi assembly). |
| `clone/images/service.go` + `types.go` | Interface `Store.JobAutonomy` → `JobAutoApprove(ctx, jobID) (bool,bool,error)`; field `Service.Autonomy` → `AutoApprove`; `View.Autonomy` → `View.Score` kiểu `ScoreSummaryView{AutoApprove bool, Min:85, Total, Passed, Pending, Below, Summary}` (luôn điền khi có ảnh); JSON key `autonomy` → `score`; `Evaluate(85, items)`. |
| `web/channels.go` | Wire: bỏ `autonomy/priority/templateCursor/templates`; thêm `autoFetch/autoApprove/perDay/defaultTemplateId` (+ `defaultTemplateName` nếu FE cần, dẫn xuất); create/patch body theo §5.3; xoá route action templates **và method `Templates` khỏi `ChannelService` interface** (cùng fake/test); `autoPickPaused` → `autoFetchPaused`; xoá `CodeConfirmRequired` khỏi mapping 409. |
| `web/channels_summary.go:66` | `AutoFetch: c.AutoFetch`; `Cap: c.PerDay`; không đổi `templateIssue` (SummaryStats media). |
| `web/library.go` | `channelRefWire` bỏ `Position`; `Channels` nghĩa "kênh dùng làm mặc định". |
| `web/gateway.go` + `web/performance.go` | Giữ hook `clonePostedRoute`; thay `performance.go` bằng `web/posted.go`: `MountPosted` chỉ gán `g.clonePostedRoute` = POST posted (không route performance/insights, không goroutine). |
| `cmd/viderse/channel.go` | CLI: `--pace` → `--per-day`, `--auto-pick`/`--no-auto-pick` → `--auto-fetch`/`--no-auto-fetch`, thêm `--auto-approve`, `--default-template`; xoá subcommand `channel template add|remove|move|standard`; create/update map field mới; JSON output qua wire mới. |
| `cmd/viderse/clone_wiring.go` | Resolver mới (ID→snapshot), không RotationTemplates/cursor; bỏ `mountPerformance`/`stopPerformance`; mount posted (không goroutine); runner bỏ `AutoFix`. |
| `internal/performance/**` | **Xoá toàn bộ package** (service/reader/datakey/types/insight/rotation) sau khi chuyển `Post` sang `internal/clone/posted`. |
| `internal/clone/posted` (mới, nhỏ) | `Service{Store, Jobs, Marker, Title, Publish}` + `Post` (logic `performance/service.go:109-166`), `ParseVideoURL`, `ErrBadURL/NotFound/NotReady`. Không Reader/Run/compute/insights. Wiring `cmd/viderse/posted_wiring.go` (không goroutine). |
| `cmd/viderse/performance_wiring.go` | **Xoá**; chuyển `sharedYouTubePacer = youtube.NewPacer()` sang `scan_wiring.go` (nơi duy nhất còn dùng: `scanServiceDeps`, `scan_wiring.go:66`; CLI scan dùng chung). |
| `postgres/performance.go` | Giữ 3 method facts/youtube; xoá toàn bộ read/stats/applied/weight; `SetYoutube` bỏ `DeletePostStats` (bảng đã drop). |

### 5.3 Hợp đồng API thay đổi (không alias)

| Endpoint | Trước | Sau |
|---|---|---|
| `POST /api/owner-channels` | `autonomy?, priority?, confirmed?` | `autoFetch?` (default true), `autoApprove?` (default false), `perDay?` (default 5), `defaultTemplateId?`; vắng `defaultTemplateId` = tạo Mẫu chuẩn |
| `PATCH /api/owner-channels/:id` | `autonomy?/priority?/confirmed?` | vắng = giữ nguyên. `defaultTemplateId` dùng `json.RawMessage` như trường nullable hiện có: **null tường minh và "" đều 400** (default bắt buộc), chuỗi phải tồn tại. `autoFetch?/autoApprove?/perDay?` null = không đổi |
| `POST /api/owner-channels/:id/templates` | add/remove/move/standard | **xoá route**; thay bằng PATCH `defaultTemplateId` (một default) |
| `DELETE /sources/:rid` | `{autoPickPaused}` | `{autoFetchPaused}` |
| Channel JSON | `autoPick, pacePerDay, autonomy, priority, templateCursor, templates[]` | `autoFetch, perDay, autoApprove, defaultTemplateId, defaultTemplateName?` |
| `GET /api/templates`, `/api/templates/:id` | `channels[{id,name,position}]` | `channels[{id,name}]` = "Mặc định của …" |
| `GET /api/clone/:id/images` | `autonomy{mode,min,...}` | `score{autoApprove,min:85,total,passed,pending,below,summary}` |
| `POST /api/clone/:id/posted` | trong performance.go, phụ thuộc `performance.Service` | giữ route/body/response; owner mới `internal/clone/posted` + `web/posted.go`; không đọc số liệu |
| `GET /api/channels/:id/performance`, `POST /api/channels/:id/insights/:id/apply` | có | **xoá** khi consumer FE cuối (VideosList) chết cùng PR4 |
| `GET /api/clone/plan`, restart preview | `template.fromRotation` | `template.fromDefault`; template = explicit hoặc default |
| Queue wait/skip reason | có `no_template` | producer mất (FK NOT NULL); xoá `WaitNoTemplate/SkipNoTemplate` + nhánh scheduler + case FE |

### 5.4 Caller phải migrate cùng PR4

Backend: `web/channels.go`, `web/channels_summary.go`, `web/library.go`, `web/clone.go` (posted), `web/gateway.go`, `cmd/viderse/{channel,clone_wiring,scan_wiring,posted_wiring}.go`, `channels/*`, `domain/*`, `autonomy/*`, `pipeline/queue/*`, `pipeline/{service,runner}.go`, `postgres/*`, `clone/images/*`, `clone/bulk` (không đổi logic; chỉ nhận `autoFetchPaused` nếu có).
FE/mobile (peer PR4, không để tới PR6): `web/src/api/channels.ts`, `api/autonomy.ts` (xoá `useAutonomy/saveAutonomy`; **giữ và đổi** `readReviewSummary` sang response `score` hoặc chuyển sang `api/cloneImages.ts` — không để mất consumer dòng “cảnh N được 72”), `api/performance.ts` (giữ `markPostedWithLink`, xoá `getPerformance/applyInsight`), `api/clone.ts` (`fromRotation`), `components/performance/*` (giữ `MarkPosted`/`requestDeleteExport`; xoá `PostedInsights`, `performance-logic`, insights i18n), `screens/channel/{VideosTab,VideoRow,VideosList,ChannelAside,applied,ScheduleCard}` xoá; `SourcesTab/TemplatesTab/SettingsTab/AutonomyMobile` viết lại; `screens/library/*` + mobile `MLibrary/MTemplate/AttachChannelDialog`; `screens/queue` bỏ `no_template`; `i18n/{channels,library,newScreen,performance}`. `MChannel/MCreate/MLibrary/MTemplate` phải chạy trên contract mới ngay PR4.

---

## 6. Admission + default update: lock order và snapshot boundary

### 6.1 Lock order thống nhất

`owner_channels` → `clone_queue` → `templates` (KEY SHARE/FOR SHARE) → `clone_jobs`.

1. **Default update** (`PATCH defaultTemplateId`):
   - Service: `TemplateExists` (đọc, cho 404 thân thiện).
   - Một UPDATE trên `owner_channels` (row lock) đặt `default_template_id`; FK xác nhận template tồn tại (race delete → FK error map 409/400).
   - Không lock `templates` trước `owner_channels` theo chiều ngược.
2. **Admission** (`CloneStore.StartJob`) — mọi đường, kể cả không có `QueueItemID`:
   - `SELECT default_template_id FROM owner_channels WHERE id=$1 FOR UPDATE` (query mới, thay `LockQueueOwner` chỉ-queue-item).
   - Nếu có queue item: `SELECT * FROM clone_queue WHERE id=$1 FOR UPDATE` (giữ); override = `item.template_id` **đọc trong tx**.
   - `def = override ?? default_template_id`; so với candidate; nếu khác → trả sentinel `ErrTemplateMoved{TemplateID:def}` (không ghi gì).
   - `SELECT revision, recipe, name FROM templates WHERE id=$def FOR SHARE`; `revision != snapshotRevision` → sentinel `ErrTemplateSnapshotStale` (không ghi).
   - Insert `clone_jobs` (template_id, template_snapshot, model_refs), mark queue started, refreshDerived. **Không** network/AI/file I/O trong tx; **không** UPDATE `template_cursor`.
3. **Enqueue / queue edit / restart**: giữ lock channel → queue như hiện tại (`LockOwnerChannel`, `LockQueueItemForAdmission`); restart ghi explicit `template_id` = template của job cũ (không đọc default lúc admission, không cursor).
4. **Template edit/delete**: lock `templates` row (UPDATE/DELETE); `DeleteTemplate` pre-check default refs bằng SELECT không lock — FK là backstop; không lock channel trong delete để tránh vòng chờ.
5. **Snapshot build** (library.SnapshotTemplate: media bind/import, advisory lock media) chạy **ngoài** tx. Revision fence trong tx bảo đảm job chỉ persist đúng revision đã build.

### 6.2 Candidate/retry protocol (thay `ResolveTemplate` hiện tại)

- `pipeline.TemplateSnapshotter func(ctx, templateID) (snapshot json.RawMessage, revision int64, models json.RawMessage, err error)`.
- `pipeline.Service.Start`:
  1. candidate = item.TemplateID nếu có, ngược lại `CandidateTemplate(ctx, channelID)` (SELECT không lock, chỉ là gợi ý).
  2. snapshot/revision/models = Snapshotter(candidate).
  3. Gọi `StartJob` kèm `TemplateID=candidate`, `SnapshotRevision`.
  4. Bắt `ErrTemplateMoved` → resolve lại đúng ID trả về (tối đa 3 lần); `ErrTemplateSnapshotStale` → snapshot lại cùng ID (tối đa 3). Hết lượt → trả lỗi, scheduler poll sau; **không** fallback sang template khác, không skip queue item.
- Barrier chấp nhận: worker chuẩn bị A, default B commit trước admission → job tạo với B (sau 1 vòng resolve); override T vẫn T; job đã persist A không đổi.
- Override validation ở Plan: template tồn tại trong Thư viện (không còn điều kiện "thuộc rotation").

### 6.3 Bất biến kiểm được

- Không job nào mang snapshot của revision khác `templates.revision` tại commit.
- Không job nào dùng default cũ sau khi UPDATE default đã commit trước admission.
- Không job mới nào dùng rotation/cursor/weight.
- `clone_jobs.template_snapshot` của job cũ không bị ghi lại.

---

## 7. Auto-approve: biên và cổng

Điều kiện duy nhất để tự duyệt (`autonomy/run.go:15-70` sau sửa):

1. `owner_channels.auto_approve = true` tại **thời điểm xét** (re-check trước `ApproveAll`; toggle tắt giữa poll không được duyệt).
2. `ReviewState.Ready = true` và `state.Revision == job.image_revision` (ảnh đang vẽ/đổi revision → bỏ qua, xét lại lượt sau).
3. `Evaluate(85, items)`: mọi item **không** `Skipped` phải có `Score != nil` và `Score.Value >= 85`; `Total > 0`.
4. `ApproveAll` qua đúng cổng duyệt + fence revision; lỗi cổng → `ResultError`, tối đa `MaxReviewTries=3`, không duyệt.
5. Ghi `auto_review` + `clone_auto_actions auto_approve` + notification "Tự duyệt theo điểm" với facts `min=85`, `passed/total`, `lowest`.

| Ảnh | Kết quả |
|---|---|
| 7 ảnh, tất cả ≥85, toggle bật | duyệt |
| một ảnh 84 | **không** duyệt; summary "cảnh N được 84" |
| một ảnh 85 | duyệt (>=, không >) |
| một ảnh chưa có điểm (nil) | **không** duyệt; "cảnh N chưa có điểm" |
| ảnh `skipped` | không tính; không chặn |
| 0 ảnh / Total=0 | **không** duyệt (`AllPass=false`) |
| mixed 85,85,84 | **không** duyệt toàn video |
| toggle tắt | không dispatch, không duyệt |
| revision đổi giữa xét và duyệt | không duyệt; lượt sau xét revision mới |
| cổng duyệt lỗi | không duyệt; tối đa 3 lần rồi để chủ xử lý |

Không hạ ngưỡng theo `min` cũ, không duyệt khi thiếu điểm, không tự duyệt ảnh lỗi/chưa vẽ.

---

## 8. perDay / validation / không clamp

- Hằng số: `PerDayMin=1`, `PerDayMax=20`, `PerDayDefault=5`. Validate ở cả Create và Update; lỗi 400 message "Nhịp mỗi ngày phải trong khoảng 1–20." (giữ style hiện tại).
- DB CHECK `per_day BETWEEN 1 AND 20`; migration STOP nếu dữ liệu cũ ngoài khoảng. Không UPDATE hạ 21→20, không auto-disable kênh.
- Bỏ clamp runtime `if l.Pace < 1 { l.Pace = DefaultPace }` (`queue/service.go:265-267`) vì bất biến DB đã bảo đảm; test 0/1/20/21.
- `today.cap` (summary) và `PaceLeftToday`/`OverflowDays` đọc `PerDay`.

---

## 9. Retire performance: readers/routes/giữ lại

### 9.1 Live readers phải chết (không chỉ xoá card UI)

| Thành phần | Bằng chứng |
|---|---|
| Goroutine `svc.Run(runCtx)` mỗi 15' → `Tick → seedFacts → ReadCandidates → Reader.Read` (DataAPI/public page) | `cmd/viderse/performance_wiring.go:28-56`, `performance/service.go:334-430` (Run 334, Tick 368, seedFacts 167) |
| Wiring startup + stop | `clone_wiring.go:475,487` |
| `AutoReader/DataAPIReader/PublicPageReader/KeyStore` | `performance/reader.go`, `datakey.go` |
| Route `GET /api/channels/:id/performance` | `web/performance.go:44-52` |
| Route `POST /api/channels/:id/insights/:id/apply` | `web/performance.go:53-56`, `service.go:284-340` |
| Insight/compare/rotation weight | `performance/insight.go`, `rotation.go`, `queries/performance.sql:88-102` |
| FE đọc số liệu + áp dụng insight + `postedDetail` | `web/src/api/performance.ts:102-120`, `components/performance/{PostedInsights,performance-logic}.ts*`, `screens/channel/VideosList.tsx:55,107,134` (chết cùng tab Video) |
| Test chỉ phục vụ feature | `internal/web/performance_test.go`, `performance/performance_test.go`, `performance/rotation_test.go` |

### 9.2 Giữ lại (không được xoá)

| Giữ | Owner sau V38 |
|---|---|
| `clone_jobs.posted_at` + `CloneStore.MarkPosted` + bulk `posted/unposted` | `clone.go`, `clone/bulk/service.go:111-140` |
| `POST /api/clone/:id/posted` (body `{youtubeUrl?}`, response `{youtubeId}`, 409 khi chưa `succeeded`) | `internal/clone/posted` + `web/posted.go` |
| Link YouTube tuỳ chọn + `clone_post_facts` (metadata mark-posted) | 3 query facts/youtube ở §5.1 |
| `deleteExport` + `RecordExportRemoved` + gating `PostedAt != nil` | `clone/bulk/service.go:133-140`, `editor/exports.go`, bundle |
| Template snapshot của job + group `posted` | clone pipeline |
| `sharedYouTubePacer` cho luồng nguồn (scan đối thủ, kiểm tra link) | chuyển sang `scan_wiring.go`; `clone_wiring.go:397` probe giữ nguyên client riêng (không thuộc V38) |
| Kiểm định ảnh + notification tự duyệt | `autonomy`, `clone/images` |

### 9.3 Bảng dữ liệu

- Drop `clone_post_stats`, `performance_insights_applied`.
- Giữ `clone_post_facts` (mark-posted link). Không có writer stats mới.
- Settings key `youtube.dataApiKey` (`performance/datakey.go:6`) còn trong bảng `settings`: không còn reader đọc; không xoá row trong V38 (dọn ở retention riêng).

---

## 10. Ma trận verification (thi công PR4)

| Mức | Kịch bản | Qua khi |
|---|---|---|
| Critical | Migration từ DB rỗng và từ snapshot legacy (rotation nhiều template, autonomy score/all, missing/invalid/custom, perDay 5) | Default = phần tử `(position,template_id)` đầu; bool khớp §3; `template_snapshot` job cũ byte-identical; không mất media/template; rollback backup chỉ trong quiesce |
| Critical | Migration gặp rotation rỗng / perDay>20 | Raise EXCEPTION, transaction rollback, **không** schema nửa vời; ghi câu hỏi kèm số cho chủ; không clamp/tạo template |
| Critical | Default đổi đúng lúc admission; AutoFetch=false; pick/override | A build, B commit trước admission → job B; override T → T; job cũ A bất biến; không tăng cursor; không deadlock (stress 2 writer) |
| Critical | Template sửa giữa lúc build snapshot và admission | `ErrTemplateSnapshotStale` → build lại; job cuối chỉ mang một revision; không job kẹt/không mất queue item |
| Critical | AutoApprove false/true; 84/85/missing/mixed/skipped/0 ảnh; toggle tắt giữa poll | Chỉ toàn bộ ≥85 + toggle bật mới duyệt; không duyệt khi thiếu điểm; revision đổi không duyệt; lỗi cổng không duyệt |
| High | `perDay` 0/1/20/21; dữ liệu cũ 21 | 1/20 ghi được; 0/21 → 400; migration STOP với 21; không clamp runtime |
| High | `autoFetch`/`autoApprove`/`defaultTemplateId` round-trip qua create/patch; default không tồn tại; null/"" | 404/400 đúng; không fallback; kênh update default không đổi default kênh khác; shared default OK |
| High | Shared default: 2 kênh cùng template; rename template; xoá default đang dùng | Rename không bị auto-naming đổi tên; delete trả 409 nêu tên kênh; FK chặn đường vòng; tạo kênh mới dùng template có sẵn không tạo Mẫu chuẩn |
| High | Link/unlink source cuối | Unlink cuối → `autoFetch=false` + `{autoFetchPaused:true}`; link đầu bật lại (hành vi hiện có; ghi rõ trong test); không đụng default |
| High | Retired post-publication ở startup + HTTP | Không goroutine/reader/log mạng ẩn; `GET performance` + `POST insights/apply` 404; mark-posted/deleteExport/scan pacer vẫn chạy thật; không route data-key |
| High | Queue: item không override hiển thị template default; restart sau khi template/ default đổi | TemplateName đúng; restart dùng **template của job cũ** (explicit, không cursor); plan mới trả `fromDefault`, restart explicit trả `fromDefault=false` |
| Medium | CLI | `channel create/update --per-day/--auto-fetch/--auto-approve/--default-template`; `channel template` báo lỗi dùng mới; JSON wire mới |
| Medium | API errors | Không còn `autonomy_confirm_required`; 400 field cũ không xuất hiện; FK race 409 không 500 |

Test cụ thể (chưa chạy): unit Go cho predicate §3.3 (raw JSON fixtures: missing, `{}`, null, 85.0, "85", 85.5, score2/suggest/all, invalid onError, approve array, extra keys); `channels` create/update/perDay/default; `postgres` admission barrier (2 connection, lock timing) + shared default FK; `autonomy` Evaluate boundary + dispatch re-check; `queue` plan/discover/restart/view; library delete/name guard; wiring test chứng minh không mount performance. SQLC regenerate; `go test ./apps/viderse/internal/{channels,autonomy,pipeline/queue,persistence/postgres,library,clone/...}`; migration CLI thật từ rỗng + snapshot legacy trên DB QA riêng theo `verification.md`. Không chạy build/test trong phiên chuẩn bị này.

---

## 11. Rủi ro / rollback

- DB/FE/BE cùng revision; nếu V37 chưa merge, V38 ở trạng thái **blocked release**, không thu nhỏ acceptance.
- Backup chỉ hợp lệ khi mọi writer quiesced liên tục từ trước backup tới hết acceptance; sau khi mở write, cấm restore snapshot cũ (mất job/paid receipt). Forward-fix/reconcile riêng.
- Không giả Down lossless; không giữ alias `autonomy/priority/rotation/performance` để né cutover.
- `onError/belowQuality` mất hành vi: bắt buộc release note + số kênh preflight; nếu chủ dữ liệu muốn giữ, mở decision mới (không tự chọn).
- Không sửa `artboards/`, không đụng git index, không regenerate `gen/models.go` bằng tay ngoài `sqlc generate`.
