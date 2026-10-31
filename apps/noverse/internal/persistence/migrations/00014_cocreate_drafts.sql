-- +goose Up
-- Drafts use the same facts and fencing as stories, but have no public identity
-- until transactional promotion. Keep the application history/blob grants from
-- 00010/00011 unchanged: abandoning drops references; storage gc reaps bytes.
ALTER TABLE stories DROP CONSTRAINT stories_status_check;
ALTER TABLE stories ADD CONSTRAINT stories_status_check
    CHECK (status IN ('draft','idea','writing','paused','done','published'));
ALTER TABLE stories ADD CONSTRAINT stories_draft_slug_check
    CHECK (status <> 'draft' OR slug LIKE 'draft-%');

-- A saved web conversation is a singleton, not the append-only engine log.
ALTER TABLE story_state DROP CONSTRAINT story_state_kind_check;
ALTER TABLE story_state ADD CONSTRAINT story_state_kind_check CHECK (kind IN (
    'book','progress','run_meta','user_rules','user_rules_runtime','signals','pending_commit','pending_revision',
    'world_rules','foreshadow','timeline','cast','address_terms','relationships','style_rules',
    'foundation_audit','simulation','format','config','import_manifest','import_intent','import_state','export_preferences','rewrites','comment_events',
    'user_rules_pin','user_rules_cache','lesson_rules',
    'last_commit','last_review','usage','project_format','author_revision_style','outline_feedback_resolution','characters','state_changes',
    'cocreate_session'
));

-- +goose Down
-- Deliberately refuse rollback when draft/session facts still exist.
ALTER TABLE stories DROP CONSTRAINT stories_draft_slug_check;
ALTER TABLE stories DROP CONSTRAINT stories_status_check;
ALTER TABLE stories ADD CONSTRAINT stories_status_check
    CHECK (status IN ('idea','writing','paused','done','published'));
ALTER TABLE story_state DROP CONSTRAINT story_state_kind_check;
ALTER TABLE story_state ADD CONSTRAINT story_state_kind_check CHECK (kind IN (
    'book','progress','run_meta','user_rules','user_rules_runtime','signals','pending_commit','pending_revision',
    'world_rules','foreshadow','timeline','cast','address_terms','relationships','style_rules',
    'foundation_audit','simulation','format','config','import_manifest','import_intent','import_state','export_preferences','rewrites','comment_events',
    'user_rules_pin','user_rules_cache','lesson_rules',
    'last_commit','last_review','usage','project_format','author_revision_style','outline_feedback_resolution','characters','state_changes'
));
