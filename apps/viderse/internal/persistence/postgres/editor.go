package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EditorRepository stores only mutable heads. The bytes addressed by each head
// are immutable media artifacts, published in the same transaction as CAS.
type EditorRepository struct{ pool *pgxpool.Pool }

func NewEditorRepository(pool *pgxpool.Pool) *EditorRepository { return &EditorRepository{pool: pool} }

var _ editor.Repository = (*EditorRepository)(nil)

const editorPointersSQL = `SELECT 'projects/' || project_id AS resource_key, content_revision, logical_path, revision, updated_at FROM editor_projects
UNION ALL
SELECT resource_key, content_revision, logical_path, revision, updated_at FROM editor_resources`

func (r *EditorRepository) Get(ctx context.Context, key string) (editor.Resource, error) {
	if err := validateEditorKey(key); err != nil {
		return editor.Resource{}, err
	}
	var resource editor.Resource
	err := mediaDB(ctx, r.pool).QueryRow(ctx, `SELECT resource_key,content_revision,logical_path,revision,updated_at FROM (`+editorPointersSQL+`) heads WHERE resource_key=$1`, key).Scan(&resource.Key, &resource.Revision, &resource.Path, &resource.Version, &resource.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return r.synthetic(ctx, key)
	}
	return resource, err
}

func (r *EditorRepository) List(ctx context.Context, prefix string) ([]editor.Resource, error) {
	rows, err := mediaDB(ctx, r.pool).Query(ctx, `SELECT resource_key,content_revision,logical_path,revision,updated_at FROM (`+editorPointersSQL+`) heads WHERE left(resource_key,length($1))=$1 ORDER BY resource_key`, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	resources := []editor.Resource{}
	for rows.Next() {
		var resource editor.Resource
		if err := rows.Scan(&resource.Key, &resource.Revision, &resource.Path, &resource.Version, &resource.UpdatedAt); err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	seen := make(map[string]bool, len(resources))
	for _, resource := range resources {
		seen[resource.Key] = true
	}
	artifacts, err := r.legacyArtifacts(ctx, "")
	if err != nil {
		return nil, err
	}
	candidates := map[string]bool{}
	for _, artifact := range artifacts {
		key := editorAliasKey(editorArtifactAlias(artifact.Path))
		if key != "" && strings.HasPrefix(key, prefix) && !seen[key] {
			candidates[key] = true
		}
	}
	for key := range candidates {
		resource, err := r.synthetic(ctx, key)
		if errors.Is(err, blobstore.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	sort.Slice(resources, func(i, j int) bool { return resources[i].Key < resources[j].Key })
	return resources, nil
}

func (r *EditorRepository) Paths(ctx context.Context, prefix string) ([]string, error) {
	rows, err := mediaDB(ctx, r.pool).Query(ctx, `SELECT logical_path FROM blob_refs WHERE left(logical_path,length($1))=$1 ORDER BY logical_path`, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	paths := []string{}
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		if strings.HasPrefix(path, "editor/") {
			if deleted, err := r.deletedAlias(ctx, path); err != nil {
				return nil, err
			} else if deleted {
				continue
			}
		}
		seen[path] = true
	}
	if prefix == "" || strings.HasPrefix(prefix, "editor/") || strings.HasPrefix("editor/", prefix) {
		artifacts, err := r.legacyArtifacts(ctx, "")
		if err != nil {
			return nil, err
		}
		for _, artifact := range artifacts {
			alias := editorArtifactAlias(artifact.Path)
			if !strings.HasPrefix(alias, prefix) {
				continue
			}
			if deleted, err := r.deletedAlias(ctx, alias); err != nil {
				return nil, err
			} else if deleted {
				continue
			}
			seen[alias] = true
		}
	}
	paths = make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

func validateEditorKey(key string) error {
	if err := media.ValidatePath(key); err != nil {
		return err
	}
	if id, project := strings.CutPrefix(key, "projects/"); project && !strings.Contains(id, "/") {
		if _, err := editor.Path(id); err != nil {
			return err
		}
	}
	return nil
}

// The table/column names are chosen exclusively from constants, never input.
func editorPointerTable(key string) (table, column, id string) {
	if id, ok := strings.CutPrefix(key, "projects/"); ok && !strings.Contains(id, "/") {
		return "editor_projects", "project_id", id
	}
	return "editor_resources", "resource_key", key
}

func editorLegacyPath(key, hint string) (string, error) {
	if hint != "" {
		if err := media.ValidatePath(hint); err != nil {
			return "", err
		}
		return hint, nil
	}
	if id, ok := strings.CutPrefix(key, "projects/"); ok && !strings.Contains(id, "/") {
		return editor.Path(id)
	}
	return "", nil
}

func validateEditorRevision(revision string) error {
	if !strings.HasPrefix(revision, "sha256:") {
		return fmt.Errorf("invalid editor content revision")
	}
	_, err := media.ContentKey(strings.TrimPrefix(revision, "sha256:"))
	return err
}

func (r *EditorRepository) lock(ctx context.Context, key string) error {
	if err := validateEditorKey(key); err != nil {
		return err
	}
	if _, ok := ctx.Value(mediaTxKey{}).(pgx.Tx); !ok {
		return errors.New("editor pointer mutation requires publication transaction")
	}
	_, err := mediaDB(ctx, r.pool).Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,253026))`, "editor/"+key)
	return err
}

func (r *EditorRepository) CAS(ctx context.Context, key, expected string, next editor.Resource) error {
	if next.Key != "" && next.Key != key {
		return errors.New("editor resource key does not match CAS key")
	}
	if err := media.ValidatePath(next.Path); err != nil {
		return err
	}
	if err := validateEditorRevision(next.Revision); err != nil {
		return err
	}
	if expected != "" {
		if err := validateEditorRevision(expected); err != nil {
			return editor.ErrConflict
		}
	}
	legacyPath, err := editorLegacyPath(key, next.PreviousPath)
	if err != nil {
		return err
	}
	if err = r.lock(ctx, key); err != nil {
		return err
	}
	db := mediaDB(ctx, r.pool)
	deleted, err := r.deleted(ctx, key)
	if err != nil {
		return err
	}
	if deleted && expected != "" {
		return editor.ErrConflict
	}
	current, err := r.Get(ctx, key)
	exists := err == nil && current.Version > 0
	if err != nil && !errors.Is(err, blobstore.ErrNotFound) {
		return err
	}
	oldPath := current.Path
	actual := current.Revision
	if !exists && actual == "" && !deleted && legacyPath != "" {
		var digest string
		err = db.QueryRow(ctx, `SELECT b.sha256 FROM blob_refs r JOIN blobs b USING(object_key) WHERE r.logical_path=$1`, legacyPath).Scan(&digest)
		if err == nil {
			actual, oldPath = "sha256:"+digest, legacyPath
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}
	if actual != expected {
		return editor.ErrConflict
	}
	// The pointer must identify exactly the bytes already bound by Publish,
	// not a caller-supplied hash or an artifact which was never published.
	var published string
	if err = db.QueryRow(ctx, `SELECT 'sha256:' || b.sha256 FROM blob_refs r JOIN blobs b USING(object_key) WHERE r.logical_path=$1`, next.Path).Scan(&published); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return blobstore.ErrNotFound
		}
		return err
	}
	if published != next.Revision {
		return blobstore.ErrDigestMismatch
	}
	updatedAt := next.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	table, column, id := editorPointerTable(key)
	if exists {
		_, err = db.Exec(ctx, `UPDATE `+table+` SET revision=revision+1,logical_path=$2,content_revision=$3,updated_at=$4 WHERE `+column+`=$1`, id, next.Path, next.Revision, updatedAt)
	} else {
		_, err = db.Exec(ctx, `INSERT INTO `+table+` (`+column+`,revision,logical_path,content_revision,updated_at) VALUES($1,1,$2,$3,$4)`, id, next.Path, next.Revision, updatedAt)
	}
	if err != nil {
		return err
	}
	if deleted {
		if _, err = db.Exec(ctx, `DELETE FROM editor_deleted_resources WHERE resource_key=$1`, key); err != nil {
			return err
		}
	}
	if oldPath != "" && oldPath != next.Path {
		return r.releaseBinding(ctx, oldPath)
	}
	return nil
}

func (r *EditorRepository) Delete(ctx context.Context, key, expected string) error {
	if err := r.lock(ctx, key); err != nil {
		return err
	}
	current, err := r.Get(ctx, key)
	if errors.Is(err, blobstore.ErrNotFound) {
		if expected == "" {
			_, err = mediaDB(ctx, r.pool).Exec(ctx, `INSERT INTO editor_deleted_resources(resource_key) VALUES($1) ON CONFLICT DO NOTHING`, key)
			return err
		}
		return editor.ErrConflict
	}
	if err != nil {
		return err
	}
	if current.Revision != expected {
		return editor.ErrConflict
	}
	table, column, id := editorPointerTable(key)
	if _, err = mediaDB(ctx, r.pool).Exec(ctx, `DELETE FROM `+table+` WHERE `+column+`=$1`, id); err != nil {
		return err
	}
	if _, err = mediaDB(ctx, r.pool).Exec(ctx, `INSERT INTO editor_deleted_resources(resource_key) VALUES($1) ON CONFLICT(resource_key) DO UPDATE SET updated_at=now()`, key); err != nil {
		return err
	}
	return r.releaseBinding(ctx, current.Path)
}

// LockDeletedProject proves a persisted deletion while holding the same
// publication-scoped advisory lock as create/CAS/delete. It never classifies an
// unknown project or a recreated pointer as a released revision.
func (r *EditorRepository) LockDeletedProject(ctx context.Context, id string) (bool, error) {
	key := "projects/" + id
	if err := editor.ValidateID(id); err != nil {
		return false, err
	}
	if err := r.lock(ctx, key); err != nil {
		return false, err
	}
	var deleted bool
	err := mediaDB(ctx, r.pool).QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM editor_deleted_resources WHERE resource_key=$1
	) AND NOT EXISTS(SELECT 1 FROM editor_projects WHERE project_id=$2)`, key, id).Scan(&deleted)
	return deleted, err
}

// A phase-05 canonical project can still be owned by clone_artifacts; another
// editor pointer can also share a binding. Remove only the obsolete manifest
// binding, never the underlying local/S3 bytes, and preserve every live owner.
func (r *EditorRepository) releaseBinding(ctx context.Context, path string) error {
	var live bool
	if err := mediaDB(ctx, r.pool).QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM storage_live_refs WHERE logical_path=$1)`, path).Scan(&live); err != nil {
		return err
	}
	if live {
		return nil
	}
	state, err := readStorageState(ctx, mediaDB(ctx, r.pool))
	if err != nil {
		return err
	}
	blobs := BlobRepository{pool: r.pool}
	return blobs.Unbind(ctx, state.Active, path)
}
