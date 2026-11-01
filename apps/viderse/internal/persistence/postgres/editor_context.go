package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/jackc/pgx/v5"
)

var _ editor.ResourcePathResolver = (*EditorRepository)(nil)

// Current step manifests, not the append-only clone_artifacts table, identify
// this job generation's active outputs. Replaced jobs retain artifacts for GC
// ownership but must never supply editor aliases or ASR anchors.
const currentEditorArtifactsSQL = `SELECT a.value->>'path',a.value->>'sha256',b.updated_at,b.output_manifest->'data'->>'projectId',j.id::text
FROM clone_steps b JOIN clone_jobs j ON j.id=b.job_id
CROSS JOIN LATERAL jsonb_array_elements(coalesce(b.output_manifest->'artifacts','[]'::jsonb)) a(value)
WHERE b.step='build' AND b.status IN ('waiting','succeeded')
AND j.superseded_at IS NULL AND coalesce(b.output_manifest->'data'->>'projectId','')<>''`

type legacyEditorArtifact struct {
	Path, Digest string
	UpdatedAt    time.Time
}

func editorArtifactAlias(path string) string {
	if strings.HasPrefix(path, "editor/") {
		return path
	}
	if strings.HasPrefix(path, "clone/") {
		parts := strings.SplitN(path, "/", 4)
		if len(parts) == 4 && strings.HasPrefix(parts[3], "editor/") {
			return parts[3]
		}
	}
	return ""
}

func editorAliasKey(path string) string {
	switch path {
	case "editor/app/effect-presets.json":
		return "workspace/effect-presets"
	case "editor/.freecut-workspace.json":
		return "workspace/meta"
	case "editor/media-index.json":
		return "workspace/media-index"
	case "editor/media-reclaim-pending.json":
		return "workspace/media-reclaim-pending"
	}
	parts := strings.Split(path, "/")
	if len(parts) == 4 && parts[0] == "editor" && parts[1] == "media" && parts[3] == "metadata.json" {
		return "media/" + parts[2]
	}
	if len(parts) == 6 && parts[0] == "editor" && parts[1] == "projects" && parts[3] == "exports" {
		key := "exports/" + parts[2] + "/" + parts[4]
		if parts[5] == "gate-diagnostics.json" {
			return key + "/diagnostics"
		}
		return key
	}
	if len(parts) == 4 && parts[0] == "editor" && parts[1] == "projects" {
		switch parts[3] {
		case "project.json":
			return "projects/" + parts[2]
		case "links.json", "media-links.json":
			return "projects/" + parts[2] + "/links"
		default:
			if strings.HasSuffix(parts[3], ".json") {
				return "projects/" + parts[2] + "/sidecars/" + strings.TrimSuffix(parts[3], ".json")
			}
		}
	}
	return ""
}

func editorKeyAlias(key string) string {
	switch key {
	case "workspace/effect-presets":
		return "editor/app/effect-presets.json"
	case "workspace/meta":
		return "editor/.freecut-workspace.json"
	case "workspace/media-index":
		return "editor/media-index.json"
	case "workspace/media-reclaim-pending":
		return "editor/media-reclaim-pending.json"
	}
	parts := strings.Split(key, "/")
	if len(parts) == 2 {
		switch parts[0] {
		case "projects":
			return "editor/projects/" + parts[1] + "/project.json"
		case "media":
			return "editor/media/" + parts[1] + "/metadata.json"
		}
	}
	if len(parts) == 3 && parts[0] == "projects" && parts[2] == "links" {
		return "editor/projects/" + parts[1] + "/links.json"
	}
	if len(parts) == 4 && parts[0] == "projects" && parts[2] == "sidecars" {
		return "editor/projects/" + parts[1] + "/" + parts[3] + ".json"
	}
	if len(parts) == 3 && parts[0] == "exports" {
		return "editor/projects/" + parts[1] + "/exports/" + parts[2] + "/meta.json"
	}
	if len(parts) == 4 && parts[0] == "exports" && parts[3] == "diagnostics" {
		return "editor/projects/" + parts[1] + "/exports/" + parts[2] + "/gate-diagnostics.json"
	}
	return ""
}

func (r *EditorRepository) deleted(ctx context.Context, key string) (bool, error) {
	var deleted bool
	err := mediaDB(ctx, r.pool).QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM editor_deleted_resources WHERE resource_key=$1)`, key).Scan(&deleted)
	return deleted, err
}

func (r *EditorRepository) deletedAlias(ctx context.Context, path string) (bool, error) {
	alias := editorArtifactAlias(path)
	if alias == "" {
		return false, nil
	}
	parts := strings.Split(alias, "/")
	keys := []string{}
	if key := editorAliasKey(alias); key != "" {
		keys = append(keys, key)
	}
	if len(parts) >= 3 && (parts[1] == "projects" || parts[1] == "media") {
		keys = append(keys, parts[1]+"/"+parts[2])
	}
	if len(parts) >= 5 && parts[1] == "projects" && parts[3] == "exports" {
		keys = append(keys, "exports/"+parts[2]+"/"+parts[4])
	}
	if len(keys) == 0 {
		return false, nil
	}
	var deleted bool
	err := mediaDB(ctx, r.pool).QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM editor_deleted_resources WHERE resource_key=ANY($1::text[]))`, keys).Scan(&deleted)
	return deleted, err
}

func (r *EditorRepository) legacyArtifacts(ctx context.Context, alias string) ([]legacyEditorArtifact, error) {
	query := currentEditorArtifactsSQL
	args := []any{}
	if alias != "" {
		query += ` AND (a.value->>'path'=$1 OR right(a.value->>'path',length($1)+1)='/'||$1)`
		args = append(args, alias)
	}
	query += ` ORDER BY b.updated_at DESC, a.value->>'path'`
	rows, err := mediaDB(ctx, r.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	artifacts := []legacyEditorArtifact{}
	for rows.Next() {
		var artifact legacyEditorArtifact
		var projectID, jobID string
		if err = rows.Scan(&artifact.Path, &artifact.Digest, &artifact.UpdatedAt, &projectID, &jobID); err != nil {
			return nil, err
		}
		canonical := editorArtifactAlias(artifact.Path)
		if canonical == "" {
			continue
		}
		if strings.HasPrefix(artifact.Path, "clone/") && !strings.HasPrefix(artifact.Path, "clone/"+jobID+"/") {
			return nil, errors.New("editor artifact belongs to a different clone job")
		}
		if _, err = editor.Path(projectID); err != nil {
			return nil, err
		}
		if strings.HasPrefix(canonical, "editor/projects/") && !strings.HasPrefix(canonical, "editor/projects/"+projectID+"/") {
			return nil, errors.New("editor artifact does not match current build project")
		}
		if err = media.ValidatePath(artifact.Path); err != nil {
			return nil, err
		}
		if err = validateEditorRevision("sha256:" + artifact.Digest); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, rows.Err()
}

// pickArtifact chooses the current generation of an alias. Content-addressed
// media aliases (editor/media/media-<sha>/…) may legitimately differ between
// jobs for artifacts published before the build stopped writing its own clock
// into the record, so the newest build wins there. Everything else keeps the
// strict conflict check.
func pickArtifact(alias string, artifacts []legacyEditorArtifact) (legacyEditorArtifact, error) {
	if len(artifacts) == 0 {
		return legacyEditorArtifact{}, blobstore.ErrNotFound
	}
	first := artifacts[0]
	if strings.HasPrefix(alias, "editor/media/") {
		return first, nil
	}
	for _, artifact := range artifacts[1:] {
		if artifact.Digest != first.Digest {
			return legacyEditorArtifact{}, fmt.Errorf("ambiguous current editor artifact: %s", alias)
		}
	}
	return first, nil
}

func (r *EditorRepository) synthetic(ctx context.Context, key string) (editor.Resource, error) {
	alias := editorKeyAlias(key)
	if alias == "" {
		return editor.Resource{}, blobstore.ErrNotFound
	}
	if deleted, err := r.deletedAlias(ctx, alias); err != nil {
		return editor.Resource{}, err
	} else if deleted {
		return editor.Resource{}, blobstore.ErrNotFound
	}
	aliases := []string{alias}
	if strings.HasSuffix(key, "/links") && strings.HasPrefix(key, "projects/") {
		aliases = append(aliases, strings.TrimSuffix(alias, "links.json")+"media-links.json")
	}
	for _, alias := range aliases {
		resource := editor.Resource{Key: key, Path: alias}
		err := mediaDB(ctx, r.pool).QueryRow(ctx, `SELECT 'sha256:'||b.sha256,r.created_at FROM blob_refs r JOIN blobs b USING(object_key) WHERE r.logical_path=$1`, alias).Scan(&resource.Revision, &resource.UpdatedAt)
		if err == nil {
			return resource, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return editor.Resource{}, err
		}
		artifacts, err := r.legacyArtifacts(ctx, alias)
		if err != nil {
			return editor.Resource{}, err
		}
		if len(artifacts) == 0 {
			continue
		}
		first, err := pickArtifact(alias, artifacts)
		if err != nil {
			return editor.Resource{}, err
		}
		return editor.Resource{Key: key, Revision: "sha256:" + first.Digest, Path: first.Path, UpdatedAt: first.UpdatedAt}, nil
	}
	return editor.Resource{}, blobstore.ErrNotFound
}

// ResolvePath supports phase-05 canonical editor URLs without choosing an old
// generation by suffix. A literal manifest binding always wins unless deleted.
func (r *EditorRepository) ResolvePath(ctx context.Context, logical string) (string, error) {
	if err := media.ValidatePath(logical); err != nil {
		return "", err
	}
	if deleted, err := r.deletedAlias(ctx, logical); err != nil {
		return "", err
	} else if deleted {
		return "", blobstore.ErrNotFound
	}
	var exists bool
	if err := mediaDB(ctx, r.pool).QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM blob_refs WHERE logical_path=$1)`, logical).Scan(&exists); err != nil {
		return "", err
	}
	if exists {
		return logical, nil
	}
	if !strings.HasPrefix(logical, "editor/") {
		return "", blobstore.ErrNotFound
	}
	artifacts, err := r.legacyArtifacts(ctx, logical)
	if err != nil {
		return "", err
	}
	if len(artifacts) == 0 {
		return "", blobstore.ErrNotFound
	}
	first, err := pickArtifact(logical, artifacts)
	if err != nil {
		return "", err
	}
	return first.Path, nil
}

func (r *EditorRepository) ProjectOwner(ctx context.Context, projectID string) (string, error) {
	if _, err := editor.Path(projectID); err != nil {
		return "", err
	}
	rows, err := mediaDB(ctx, r.pool).Query(ctx, `SELECT j.id::text FROM clone_jobs j JOIN clone_steps b ON b.job_id=j.id AND b.step='build' WHERE j.superseded_at IS NULL AND b.status IN ('waiting','succeeded') AND b.output_manifest->'data'->>'projectId'=$1 ORDER BY j.id`, projectID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	owner := ""
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return "", err
		}
		if owner != "" {
			return "", fmt.Errorf("project belongs to multiple current clone jobs: %s", projectID)
		}
		owner = id
	}
	return owner, rows.Err()
}

func (r *EditorRepository) AnchorsPath(ctx context.Context, projectID string) (string, error) {
	owner, err := r.ProjectOwner(ctx, projectID)
	if err != nil {
		return "", err
	}
	if owner == "" {
		return "", blobstore.ErrNotFound
	}
	rows, err := mediaDB(ctx, r.pool).Query(ctx, `SELECT a.value->>'path' FROM clone_steps t CROSS JOIN LATERAL jsonb_array_elements(coalesce(t.output_manifest->'artifacts','[]'::jsonb)) a(value) JOIN clone_artifacts owned ON owned.job_id=t.job_id AND owned.path=a.value->>'path' AND owned.step='transcript' WHERE t.job_id=$1::uuid AND t.step='transcript' AND t.status='succeeded' AND (a.value->>'path'='transcript/asr-anchors.json' OR right(a.value->>'path',length('/transcript/asr-anchors.json'))='/transcript/asr-anchors.json') ORDER BY a.value->>'path'`, owner)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	path := ""
	for rows.Next() {
		var found string
		if err = rows.Scan(&found); err != nil {
			return "", err
		}
		if err = media.ValidatePath(found); err != nil {
			return "", err
		}
		if path != "" && path != found {
			return "", fmt.Errorf("ambiguous current ASR anchors for project %s", projectID)
		}
		path = found
	}
	if err = rows.Err(); err != nil {
		return "", err
	}
	if path == "" {
		return "", blobstore.ErrNotFound
	}
	return path, nil
}

// MediaReferences deliberately protects all retained snapshots, including queued
// and completed jobs. False retention is safe; a query/decode error must stop GC.
func (r *EditorRepository) MediaReferences(ctx context.Context) ([]string, error) {
	rows, err := mediaDB(ctx, r.pool).Query(ctx, `SELECT source_snapshot FROM clone_jobs UNION ALL SELECT template_snapshot FROM clone_jobs UNION ALL SELECT output_manifest FROM clone_steps WHERE output_manifest IS NOT NULL UNION ALL SELECT source_snapshot FROM clone_queue UNION ALL SELECT recipe FROM templates UNION ALL SELECT project FROM templates WHERE project IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := map[string]bool{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var value any // Snapshot/project JSON is intentionally heterogeneous.
		if err = json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("decode durable media references: %w", err)
		}
		if err = collectEditorMediaIDs(value, ids); err != nil {
			return nil, err
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(ids))
	for id := range ids {
		result = append(result, id)
	}
	sort.Strings(result)
	return result, nil
}

func collectEditorMediaIDs(value any, ids map[string]bool) error {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			switch key {
			case "mediaId", "coverMediaId", "sourceMediaId", "backdropMediaId":
				if child != nil {
					id, ok := child.(string)
					if !ok {
						return fmt.Errorf("invalid durable %s reference", key)
					}
					if id != "" {
						ids[id] = true
					}
				}
			case "mediaIds":
				if child != nil {
					list, ok := child.([]any)
					if !ok {
						return errors.New("invalid durable mediaIds references")
					}
					for _, entry := range list {
						switch entry := entry.(type) {
						case string:
							if entry != "" {
								ids[entry] = true
							}
						case map[string]any:
							id, ok := entry["id"].(string)
							if !ok || id == "" {
								return errors.New("invalid durable media link")
							}
							ids[id] = true
						default:
							return errors.New("invalid durable media link")
						}
					}
				}
			}
			if err := collectEditorMediaIDs(child, ids); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range value {
			if err := collectEditorMediaIDs(child, ids); err != nil {
				return err
			}
		}
	}
	return nil
}

// DeleteProjectPolicy runs before pointer removal in that same publication tx.
// The consuming schema ties template project IDs to templates.id; previews have
// revision/time fields only, not a separate preview-project registry.
func (r *EditorRepository) DeleteProjectPolicy(ctx context.Context, id string) error {
	if _, err := editor.Path(id); err != nil {
		return err
	}
	if _, ok := ctx.Value(mediaTxKey{}).(pgx.Tx); !ok {
		return errors.New("project deletion policy requires publication transaction")
	}
	db := mediaDB(ctx, r.pool)
	// Lock matching template rows so a concurrent channel attachment cannot slip
	// between the policy check and deletion (its FK needs a conflicting key lock).
	rows, err := db.Query(ctx, `SELECT id::text FROM templates WHERE id::text=$1 OR project->>'id'=$1 ORDER BY id FOR UPDATE`, id)
	if err != nil {
		return err
	}
	templates := []string{}
	for rows.Next() {
		var template string
		if err = rows.Scan(&template); err != nil {
			rows.Close()
			return err
		}
		templates = append(templates, template)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var channel string
	err = db.QueryRow(ctx, `SELECT c.name FROM owner_channel_templates linked JOIN owner_channels c ON c.id=linked.owner_channel_id WHERE linked.template_id::text=ANY($1::text[]) ORDER BY c.id LIMIT 1`, templates).Scan(&channel)
	if err == nil {
		return &editor.Error{Code: "conflict", Message: "Project đang là template của kênh " + channel + " — bỏ template trước", Extra: editor.Object{"templateOfChannel": channel}}
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if _, err = db.Exec(ctx, `UPDATE templates SET preview_revision=NULL,preview_built_at=NULL WHERE id::text=ANY($1::text[])`, templates); err != nil {
		return err
	}
	_, err = db.Exec(ctx, `DELETE FROM templates WHERE id::text=ANY($1::text[])`, templates)
	return err
}
