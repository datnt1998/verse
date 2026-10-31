-- +goose Up
-- Bundled assets are immutable application-owned library files. Pin only the
-- exact shipped paths, not arbitrary files under a namespace or all blob refs.
CREATE OR REPLACE VIEW storage_live_refs AS
 SELECT file_path AS logical_path, mime AS content_type, 'library'::text AS usage_group FROM overlays
 UNION
 SELECT file_path, 'video/mp4', 'library' FROM backdrop_videos WHERE file_path IS NOT NULL AND file_path <> ''
 UNION
 SELECT avatar_path, 'image/png', 'library' FROM owner_channels WHERE avatar_path IS NOT NULL AND avatar_path <> ''
 UNION
 SELECT banner_path, 'image/jpeg', 'library' FROM owner_channels WHERE banner_path IS NOT NULL AND banner_path <> ''
 UNION
 SELECT path, content_type, 'clone' FROM clone_artifacts
 UNION
 SELECT logical_path, content_type, 'library' FROM blob_refs
 WHERE logical_path IN ('library/standard-template/scene-cool.webp', 'library/standard-template/scene-warm.webp', 'library/standard-template/voice-placeholder.wav', 'library/standard-template/snow-loop.mp4');

-- +goose Down
CREATE OR REPLACE VIEW storage_live_refs AS
 SELECT file_path AS logical_path, mime AS content_type, 'library'::text AS usage_group FROM overlays
 UNION
 SELECT file_path, 'video/mp4', 'library' FROM backdrop_videos WHERE file_path IS NOT NULL AND file_path <> ''
 UNION
 SELECT avatar_path, 'image/png', 'library' FROM owner_channels WHERE avatar_path IS NOT NULL AND avatar_path <> ''
 UNION
 SELECT banner_path, 'image/jpeg', 'library' FROM owner_channels WHERE banner_path IS NOT NULL AND banner_path <> ''
 UNION
 SELECT path, content_type, 'clone' FROM clone_artifacts;
