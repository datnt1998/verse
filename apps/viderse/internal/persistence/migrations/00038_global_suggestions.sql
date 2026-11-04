-- +goose Up
-- Một công tắc Đề xuất chung thay off[]/recapOnReturn, đóng đề xuất mở
-- của loại đã bỏ và dọn việc nền của trợ lý. Toàn bộ chạy trong transaction
-- của goose: gặp dữ liệu không biểu diễn được thì RAISE EXCEPTION để dừng,
-- không tự chọn thay người dùng.

-- 1) notifications.preferences ────────────────────────────────────────────
-- Bảng quy ước:
--   * thiếu suggestions / suggestions.off rỗng → bật.
--   * tắt đủ cả ba loại còn sống, không đặt theo kênh → tắt.
--   * off[] chỉ chứa loại đã bỏ → bỏ các mục đó rồi xét phần còn lại.
--   * mục đặt theo kênh hoặc chỉ tắt một phần ba loại → dừng để chủ dữ liệu chọn
--     chính sách chung, không tự bật lại điều họ đã tắt.
--   * loại lạ hoặc sai hình → dừng migration, không tự đoán.
-- +goose StatementBegin
DO $$
DECLARE
    prefs jsonb;
    suggested jsonb;
    entry jsonb;
    known_off text[] := ARRAY[]::text[];
    live_off text[];
    live_kinds constant text[] := ARRAY['hot_video', 'template_failing', 'cleanup'];
    retired_kinds constant text[] := ARRAY['learn_rule', 'raise_autonomy', 'pace', 'perf_insight'];
    enabled boolean;
BEGIN
    SELECT value INTO prefs FROM settings WHERE key = 'notifications.preferences' FOR UPDATE;
    IF NOT FOUND THEN
        RETURN;
    END IF;
    IF jsonb_typeof(prefs) <> 'object' THEN
        RAISE EXCEPTION 'notifications.preferences không phải JSON object; dừng để không tự đoán';
    END IF;

    suggested := prefs->'suggestions';
    IF suggested IS NULL OR jsonb_typeof(suggested) = 'null' THEN
        enabled := true;
    ELSIF jsonb_typeof(suggested) = 'boolean' THEN
        enabled := (suggested #>> '{}')::boolean;
    ELSIF jsonb_typeof(suggested) = 'object' THEN
        IF jsonb_typeof(COALESCE(suggested->'off', '[]'::jsonb)) <> 'array' THEN
            RAISE EXCEPTION 'suggestions.off không phải mảng; dừng để không tự đoán';
        END IF;
        known_off := ARRAY[]::text[];
        FOR entry IN SELECT * FROM jsonb_array_elements(COALESCE(suggested->'off', '[]'::jsonb)) LOOP
            IF jsonb_typeof(entry) <> 'object' OR entry->>'kind' IS NULL THEN
                RAISE EXCEPTION 'suggestions.off có mục không hợp lệ; dừng để không tự đoán';
            END IF;
            IF NOT (entry->>'kind' = ANY(retired_kinds)) AND NOT (entry->>'kind' = ANY(live_kinds)) THEN
                RAISE EXCEPTION 'suggestions.off có loại lạ %; dừng để không tự đoán', entry->>'kind';
            END IF;
            IF entry ? 'channelId' AND jsonb_typeof(entry->'channelId') NOT IN ('null', 'string') THEN
                RAISE EXCEPTION 'suggestions.off.channelId phải là chuỗi hoặc null; dừng để không tự đoán';
            END IF;
            IF entry->>'kind' = ANY(live_kinds) AND jsonb_typeof(entry->'channelId') = 'string' THEN
                RAISE EXCEPTION 'suggestions.off theo kênh không biểu diễn được bằng một công tắc; cần chủ dữ liệu chốt trước migration';
            END IF;
            IF entry->>'kind' = ANY(live_kinds) AND COALESCE(jsonb_typeof(entry->'channelId'), 'null') <> 'string' THEN
                known_off := known_off || (entry->>'kind');
            END IF;
        END LOOP;
        live_off := ARRAY(SELECT DISTINCT unnest(known_off));
        IF cardinality(live_off) = cardinality(live_kinds) THEN
            enabled := false;
        ELSIF cardinality(live_off) > 0 THEN
            RAISE EXCEPTION 'suggestions.off chỉ tắt một phần các loại còn dùng; cần chủ dữ liệu chốt trước migration';
        ELSE
            enabled := true;
        END IF;
    ELSE
        RAISE EXCEPTION 'suggestions phải là object hoặc boolean; dừng để không tự đoán';
    END IF;

    prefs := jsonb_set(prefs, '{suggestions}', to_jsonb(enabled), true);
    prefs := prefs - 'recapOnReturn';
    UPDATE settings SET value = prefs, updated_at = now() WHERE key = 'notifications.preferences';
END $$;
-- +goose StatementEnd

-- 2) Đề xuất mở của loại đã bỏ về trạng thái kết thúc; giữ lịch sử đã xử lý.
UPDATE suggestions
SET state = 'expired', resolved_at = now()
WHERE state = 'open' AND kind NOT IN ('hot_video', 'template_failing', 'cleanup');

-- Loại cũ chỉ tồn tại trong lịch sử; không producer nào được mở lại chúng.
ALTER TABLE suggestions ADD CONSTRAINT suggestions_open_kind_check
CHECK (state <> 'open' OR kind IN ('hot_video', 'template_failing', 'cleanup'));

-- 3) Việc nền của trợ lý không còn producer. background_tasks chỉ giữ việc
-- chưa kết thúc (Finish xoá dòng), nên xoá dòng assistant_plan là kết thúc
-- đúng nghĩa; lịch sử lỗi vẫn nằm trong error_events.
DELETE FROM background_tasks WHERE kind = 'assistant_plan';

-- 4) Con trỏ Bản tin và bộ đếm ứng viên quy tắc đã bỏ.
DELETE FROM settings WHERE key IN ('recap.lastSeenAt', 'learn.model.candidates');

-- +goose Down
ALTER TABLE suggestions DROP CONSTRAINT suggestions_open_kind_check;

-- Chỉ dùng cho môi trường nháp: dựng lại hình cài đặt cũ từ một công tắc.
-- Không thể biết lại tiền sử recapOnReturn, không mở lại đề xuất đã đóng và
-- không dựng lại việc nền trợ lý đã xoá.
-- +goose StatementBegin
DO $$
DECLARE
    prefs jsonb;
    enabled boolean;
BEGIN
    SELECT value INTO prefs FROM settings WHERE key = 'notifications.preferences' FOR UPDATE;
    IF NOT FOUND THEN
        RETURN;
    END IF;
    IF prefs->'suggestions' IS NULL OR jsonb_typeof(prefs->'suggestions') <> 'boolean' THEN
        RETURN;
    END IF;
    enabled := (prefs->'suggestions' #>> '{}')::boolean;
    IF enabled THEN
        prefs := jsonb_set(prefs, '{suggestions}', '{"off": []}'::jsonb, true);
    ELSE
        prefs := jsonb_set(prefs, '{suggestions}', '{"off": [{"kind": "hot_video"}, {"kind": "template_failing"}, {"kind": "cleanup"}]}'::jsonb, true);
    END IF;
    prefs := jsonb_set(prefs, '{recapOnReturn}', 'true'::jsonb, true);
    UPDATE settings SET value = prefs, updated_at = now() WHERE key = 'notifications.preferences';
END $$;
-- +goose StatementEnd
