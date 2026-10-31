-- +goose Up
-- Mức tự chủ của kênh (07 §20) thay hai công tắc cũ. Chưa từng có cột
-- auto_approve trong schema này, nên "Tự duyệt ảnh" cũ luôn là tắt → Cân bằng.
-- auto_pick giữ nguyên làm cờ bật/tạm dừng Tự lấy (bỏ đối thủ cuối vẫn tắt nó);
-- pick trong autonomy là cách lấy khi cờ này bật. Kênh đã tắt Tự lấy có chủ ý
-- (còn đối thủ liên kết) chuyển sang "Chỉ đề xuất".
ALTER TABLE owner_channels
    ADD COLUMN autonomy jsonb NOT NULL DEFAULT '{"preset":"balanced","pick":"all","approve":{"mode":"ask","min":85},"onError":"retry_once","belowQuality":"ask"}'::jsonb,
    ADD COLUMN priority text NOT NULL DEFAULT 'steady' CHECK (priority IN ('steady','views'));

UPDATE owner_channels c
SET autonomy = '{"preset":"custom","pick":"suggest","approve":{"mode":"ask","min":85},"onError":"retry_once","belowQuality":"ask"}'::jsonb
WHERE NOT c.auto_pick
  AND EXISTS (SELECT 1 FROM owner_channel_sources s WHERE s.owner_channel_id = c.id);

-- Việc máy tự thử lại một lần (job.autoFix) và kết quả duyệt theo điểm của
-- revision ảnh đã xét gần nhất (để mỗi revision chỉ xét một lần).
ALTER TABLE clone_jobs
    ADD COLUMN auto_fix jsonb,
    ADD COLUMN auto_review jsonb;

-- Hành động tự làm mới: Tự duyệt theo điểm, tự thử lại, tự sửa.
ALTER TABLE clone_auto_actions DROP CONSTRAINT clone_auto_actions_kind_check;
ALTER TABLE clone_auto_actions ADD CONSTRAINT clone_auto_actions_kind_check
    CHECK (kind IN ('safety_redraw','step_retry','auto_approve','auto_retry','auto_fix'));

-- +goose Down
DELETE FROM clone_auto_actions WHERE kind NOT IN ('safety_redraw','step_retry');
ALTER TABLE clone_auto_actions DROP CONSTRAINT clone_auto_actions_kind_check;
ALTER TABLE clone_auto_actions ADD CONSTRAINT clone_auto_actions_kind_check
    CHECK (kind IN ('safety_redraw','step_retry'));
ALTER TABLE clone_jobs DROP COLUMN auto_review, DROP COLUMN auto_fix;
ALTER TABLE owner_channels DROP COLUMN priority, DROP COLUMN autonomy;
