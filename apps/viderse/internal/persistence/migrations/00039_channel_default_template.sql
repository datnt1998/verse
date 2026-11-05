-- +goose Up
-- Kênh chỉ còn Nguồn & tự động / Template mặc định / Hồ sơ.
--
-- Hợp đồng mới của kênh: default_template_id (bắt buộc, FK RESTRICT),
-- auto_fetch, auto_approve, per_day 1–20. Bỏ autonomy (Mức tự chủ), priority
-- và danh sách template luân phiên (owner_channel_templates + template_cursor).
--
-- STOP condition (không clamp, không tạo template, không tự tắt kênh):
--   * kênh không có template luân phiên nào → không có default hợp lệ;
--   * per_day ngoài 1..20 (kể cả dữ liệu cũ 21..50);
--   * default_template_id còn NULL hoặc trỏ template không tồn tại.
-- Migration chạy trong một transaction: RAISE EXCEPTION rollback sạch, không
-- để schema nửa vời. Chủ dữ liệu nhận ID nội bộ để sửa trong môi trường quản
-- trị; không ghi đè dữ liệu production cho khớp fixture.
--
-- Chính sách tự sửa lỗi không còn cấu hình (onError/belowQuality), nhưng hành
-- vi vẫn cố định: mỗi job-generation được máy tự thử sửa đúng một lần; video
-- dưới 90 điểm luôn hỏi. clone_jobs.auto_fix và clone_auto_actions
-- auto_retry/auto_fix tiếp tục được ghi.

-- (A) STOP nếu có kênh không có template luân phiên nào.
-- +goose StatementBegin
DO $$
DECLARE bad text;
BEGIN
  SELECT string_agg(c.id::text, ',') INTO bad
  FROM owner_channels c
  WHERE NOT EXISTS (SELECT 1 FROM owner_channel_templates t WHERE t.owner_channel_id = c.id);
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION 'Dừng di trú: kênh không có template: %', bad;
  END IF;
END $$;
-- +goose StatementEnd

-- (B) STOP nếu pace_per_day ngoài 1..20 (kể cả dữ liệu cũ 21..50).
-- +goose StatementBegin
DO $$
DECLARE bad text;
BEGIN
  SELECT string_agg(c.id::text || '=' || c.pace_per_day::text, ',') INTO bad
  FROM owner_channels c WHERE c.pace_per_day NOT BETWEEN 1 AND 20;
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION 'Dừng di trú: số video mỗi ngày ngoài 1..20: %', bad;
  END IF;
END $$;
-- +goose StatementEnd

-- (C) Cột mới (chưa NOT NULL để backfill trong cùng transaction).
ALTER TABLE owner_channels
  ADD COLUMN default_template_id uuid,
  ADD COLUMN auto_approve boolean NOT NULL DEFAULT false;

-- (D) default = template đầu tiên của rotation theo position, tiebreak
-- template_id. Không cursor, không weight, không tên fixture.
UPDATE owner_channels c
SET default_template_id = (
  SELECT t.template_id
  FROM owner_channel_templates t
  WHERE t.owner_channel_id = c.id
  ORDER BY t.position, t.template_id
  LIMIT 1
);

-- (E) STOP nếu còn NULL (rotation đổi giữa preflight và cutover) hoặc template
-- không tồn tại (FK của bảng rotation lẽ ra đã chặn; assert lại để không fake).
-- +goose StatementBegin
DO $$
DECLARE bad text;
BEGIN
  SELECT string_agg(c.id::text, ',') INTO bad
  FROM owner_channels c
  WHERE c.default_template_id IS NULL
     OR NOT EXISTS (SELECT 1 FROM templates t WHERE t.id = c.default_template_id);
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION 'Dừng di trú: template mặc định không hợp lệ: %', bad;
  END IF;
END $$;
-- +goose StatementEnd

-- (F) Backfill hai bool từ autonomy HIỆU LỰC (ParseAutonomy + Validate của Go).
-- Predicate dưới đây phải khớp Autonomy.Validate() tại thời điểm cutover:
--   * đủ 5 khóa đúng kiểu và đúng miền giá trị; khóa thừa vẫn hợp lệ;
--   * min phải là số nguyên literal trong 70..95 bước 5 (85.0 / "85" / 85.5
--     đều invalid như Go unmarshal vào int);
--   * missing / NULL / sai kiểu → Cân bằng (invalid), không suy thành bật.
-- auto_fetch giữ điều kiện hiệu lực cũ auto_pick AND AutoPicks(): không bật
-- kênh đã tắt auto_pick, và pick=suggest giữ nguyên tắt.
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

-- auto_approve = Duyệt theo điểm/hết đang có hiệu lực; ngưỡng sau cutover cố
-- định 85 (kênh min khác 85 vẫn là true, nhưng duyệt theo 85).
UPDATE owner_channels c
SET auto_approve = COALESCE(r.valid AND (c.autonomy->'approve'->>'mode') IN ('score','all'), false)
FROM v38_resolved r WHERE r.id = c.id;

-- auto_fetch = cờ Tự lấy hiện tại AND cách lấy hiện tại không phải "chỉ đề xuất".
UPDATE owner_channels c
SET auto_pick = c.auto_pick AND (NOT r.valid OR (c.autonomy->>'pick') IS DISTINCT FROM 'suggest')
FROM v38_resolved r WHERE r.id = c.id;

DROP TABLE v38_resolved;

-- (G) Đổi tên cột theo hợp đồng mới; dữ liệu giữ nguyên.
ALTER TABLE owner_channels RENAME COLUMN auto_pick TO auto_fetch;
ALTER TABLE owner_channels RENAME COLUMN pace_per_day TO per_day;

-- (H) Siết CHECK ngày. DROP không IF EXISTS: tên lệch thì migration fail lớn
-- tiếng thay vì để sót CHECK 1..50.
ALTER TABLE owner_channels DROP CONSTRAINT owner_channels_pace_per_day_check;
ALTER TABLE owner_channels ADD CONSTRAINT owner_channels_per_day_check CHECK (per_day BETWEEN 1 AND 20);

-- (I) Bắt buộc default + FK RESTRICT + index tra cứu ngược từ template.
ALTER TABLE owner_channels ALTER COLUMN default_template_id SET NOT NULL;
ALTER TABLE owner_channels
  ADD CONSTRAINT owner_channels_default_template_fk
  FOREIGN KEY (default_template_id) REFERENCES templates (id) ON DELETE RESTRICT;
CREATE INDEX owner_channels_default_template_idx ON owner_channels (default_template_id);

-- (J) Bỏ cột/bảng luân phiên khi không còn caller. weight (00034) rơi theo bảng.
ALTER TABLE owner_channels
  DROP COLUMN autonomy,
  DROP COLUMN priority,
  DROP COLUMN template_cursor;
DROP TABLE owner_channel_templates;

-- (K) Bỏ bảng chỉ phục vụ collector đã retire. clone_post_facts giữ làm owner
-- metadata mark-posted (link YouTube + tiêu đề/độ dài lịch sử).
DROP TABLE clone_post_stats;
DROP TABLE performance_insights_applied;

-- +goose Down
-- Throwaway cho DB rỗng/dev, KHÔNG lossless: rotation/weight, số liệu sau đăng
-- và nhận xét đã áp dụng đã mất không thể tạo lại. Production rollback = restore
-- backup đã kiểm khi mọi writer còn quiesced, rồi mở write và forward-fix —
-- không restore snapshot cũ sau khi đã nhận job/paid receipt mới.
ALTER TABLE owner_channels DROP CONSTRAINT owner_channels_default_template_fk;
DROP INDEX owner_channels_default_template_idx;
ALTER TABLE owner_channels DROP COLUMN default_template_id;
ALTER TABLE owner_channels DROP CONSTRAINT owner_channels_per_day_check;
ALTER TABLE owner_channels RENAME COLUMN per_day TO pace_per_day;
ALTER TABLE owner_channels RENAME COLUMN auto_fetch TO auto_pick;
ALTER TABLE owner_channels ADD CONSTRAINT owner_channels_pace_per_day_check CHECK (pace_per_day BETWEEN 1 AND 50);
ALTER TABLE owner_channels
  ADD COLUMN autonomy jsonb NOT NULL DEFAULT '{"preset":"balanced","pick":"all","approve":{"mode":"ask","min":85},"onError":"retry_once","belowQuality":"ask"}'::jsonb,
  ADD COLUMN priority text NOT NULL DEFAULT 'steady' CHECK (priority IN ('steady','views')),
  ADD COLUMN template_cursor integer NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS owner_channel_templates (
    owner_channel_id uuid NOT NULL REFERENCES owner_channels (id) ON DELETE CASCADE,
    template_id      uuid NOT NULL REFERENCES templates (id) ON DELETE RESTRICT,
    position         integer NOT NULL CHECK (position >= 0),
    PRIMARY KEY (owner_channel_id, template_id)
);
CREATE TABLE IF NOT EXISTS clone_post_stats (
    job_id uuid NOT NULL REFERENCES clone_jobs(id) ON DELETE CASCADE,
    milestone text NOT NULL CHECK (milestone IN ('24h','7d','28d')),
    views bigint NOT NULL CHECK (views >= 0),
    likes bigint CHECK (likes >= 0),
    comments bigint CHECK (comments >= 0),
    source text NOT NULL CHECK (source IN ('data_api','public_page')),
    read_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (job_id, milestone)
);
CREATE TABLE IF NOT EXISTS performance_insights_applied (
    channel_id uuid NOT NULL REFERENCES owner_channels(id) ON DELETE CASCADE,
    insight_id text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('rule','rotation')),
    rule_id text,
    applied_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (channel_id, insight_id)
);
