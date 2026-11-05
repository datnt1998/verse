package editor_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/migrations"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/pgmigrate"
	"github.com/datnt1998/verse/platform/pgtest"
	"github.com/jackc/pgx/v5/pgxpool"
)

func editorPersistencePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	appDSN, maintenanceDSN := pgtest.ProvisionRoles(t, pgtest.RoleSpec{
		MigrateRole: "viderse_migrate", AppRole: "viderse_app", LockKey1: 2790, LockKey2: 1,
		InitScriptRef: "deploy/postgres-init/01-roles.sh",
	})
	db, err := pgmigrate.OpenDB(maintenanceDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = pgmigrate.Up(context.Background(), db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	pool, err := postgres.OpenPool(context.Background(), appDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func seedEditorCloneChannel(t *testing.T, pool *pgxpool.Pool, id string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `WITH template AS (
 INSERT INTO templates(id,name,recipe) VALUES(gen_random_uuid(),'Mẫu chuẩn','{}') RETURNING id
)
INSERT INTO owner_channels(id,name,default_template_id) SELECT $1,'Owner',id FROM template`, id)
	if err != nil {
		t.Fatal(err)
	}
}

func editorPersistenceStore(t *testing.T) (editor.Store, *postgres.EditorRepository, *media.Service, *pgxpool.Pool, string) {
	t.Helper()
	pool := editorPersistencePool(t)
	root := t.TempDir()
	blobs := postgres.NewBlobRepository(pool)
	_, err := blobs.Initialize(context.Background(), media.Descriptor{ID: "editor-test", Config: blobstore.Config{Driver: "local", LocalRoot: root}})
	if err != nil {
		t.Fatal(err)
	}
	transport := media.New(blobs, nil, func(d media.Descriptor) (blobstore.BlobStore, error) { return blobstore.NewLocal(d.Config.LocalRoot) })
	repository := postgres.NewEditorRepository(pool)
	store := editor.Store{Media: transport, Repository: repository, StagingDir: t.TempDir(), Now: func() time.Time { return time.UnixMilli(2000) }}
	return store, repository, transport, pool, root
}

func persistenceProject(t *testing.T, id string) editor.Project {
	t.Helper()
	p, err := editor.New(id, "Initial", 1920, 1080, 30, time.UnixMilli(1000))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Ported from yt-factory@2c201e6 projects.test.ts: create/load/save keeps
// sha256 wire revisions, rejects stale saves, and preserves createdAt.
func TestStoreProjectPointerRevisions(t *testing.T) {
	store, repo, _, pool, root := editorPersistenceStore(t)
	ctx := context.Background()
	project := persistenceProject(t, "revision")
	revision, err := store.Create(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	pointer, err := repo.Get(ctx, "projects/revision")
	if err != nil {
		t.Fatal(err)
	}
	if pointer.Revision != revision || pointer.Version != 1 || pointer.Path == "editor/projects/revision/project.json" {
		t.Fatalf("first pointer: %+v", pointer)
	}
	if _, err = os.Stat(filepath.Join(root, filepath.FromSlash(pointer.Path))); err != nil {
		t.Fatal(err)
	}
	loaded, loadedRevision, err := store.Read(ctx, "revision")
	if err != nil || loadedRevision != revision {
		t.Fatalf("read revision=%s err=%v", loadedRevision, err)
	}
	loaded["description"] = "edited"
	loaded["createdAt"] = float64(99)
	loaded["id"] = "wrong"
	nextRevision, err := store.Write(ctx, "revision", loaded, revision)
	if err != nil {
		t.Fatal(err)
	}
	if nextRevision == revision || !strings.HasPrefix(nextRevision, "sha256:") {
		t.Fatalf("wire revision %q", nextRevision)
	}
	if _, err = store.Write(ctx, "revision", project, revision); !errors.Is(err, editor.ErrConflict) {
		t.Fatalf("stale save: %v", err)
	}
	if _, err = store.Create(ctx, project); !errors.Is(err, editor.ErrConflict) {
		t.Fatalf("duplicate create: %v", err)
	}
	next, err := repo.Get(ctx, "projects/revision")
	if err != nil {
		t.Fatal(err)
	}
	if next.Version != 2 || next.Path == pointer.Path || next.Revision != nextRevision {
		t.Fatalf("next pointer: %+v", next)
	}
	current, gotRevision, err := store.Read(ctx, "revision")
	if err != nil || gotRevision != nextRevision {
		t.Fatalf("current read: %s %v", gotRevision, err)
	}
	if current["createdAt"] != float64(1000) || current["updatedAt"] != float64(2000) || current["id"] != "revision" {
		t.Fatalf("immutable identity/timestamps: %+v", current)
	}
	if loaded["id"] != "wrong" || loaded["createdAt"] != float64(99) {
		t.Fatal("save mutated caller project")
	}
	// The new revision lives at a new path; the released old object is never
	// rewritten in place and is discarded after the publication commit.
	if _, err = os.Stat(filepath.Join(root, filepath.FromSlash(pointer.Path))); !os.IsNotExist(err) {
		t.Fatalf("released old object still on disk: %v", err)
	}
	var oldBindings int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM blob_refs WHERE logical_path=$1", pointer.Path).Scan(&oldBindings); err != nil || oldBindings != 0 {
		t.Fatalf("obsolete binding remains: %d %v", oldBindings, err)
	}
	var durable int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM storage_live_refs WHERE logical_path=$1", next.Path).Scan(&durable); err != nil || durable != 1 {
		t.Fatalf("new project is not a durable GC root: %d %v", durable, err)
	}
}

// Ported from projects.test.ts: concurrent saves sharing a base revision have
// exactly one winner, and the loser cannot publish a visible project revision.
func TestStoreConcurrentSavesSameRevisionExactlyOneWins(t *testing.T) {
	store, repo, _, _, _ := editorPersistenceStore(t)
	ctx := context.Background()
	project := persistenceProject(t, "racing")
	revision, err := store.Create(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	outcomes := make(chan error, 2)
	for _, name := range []string{"one", "two"} {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			next := editor.Clone(project)
			next["name"] = name
			_, err := store.Write(ctx, "racing", next, revision)
			outcomes <- err
		}(name)
	}
	wg.Wait()
	close(outcomes)
	wins, conflicts := 0, 0
	for err := range outcomes {
		if err == nil {
			wins++
		} else if errors.Is(err, editor.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("wins=%d conflicts=%d", wins, conflicts)
	}
	pointer, err := repo.Get(ctx, "projects/racing")
	if err != nil || pointer.Version != 2 {
		t.Fatalf("pointer=%+v err=%v", pointer, err)
	}
	current, _, err := store.Read(ctx, "racing")
	if err != nil || (current["name"] != "one" && current["name"] != "two") {
		t.Fatalf("current=%+v err=%v", current, err)
	}
}

func TestStoreLegacyProjectCutoverReleasesCanonicalBytes(t *testing.T) {
	store, repo, transport, pool, root := editorPersistenceStore(t)
	ctx := context.Background()
	p := persistenceProject(t, "legacy")
	artifact, revision, err := store.Stage(p)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Path != "editor/projects/legacy/project.json" {
		t.Fatalf("build staging contract changed: %s", artifact.Path)
	}
	if err = transport.Publish(ctx, []media.Artifact{artifact}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, filepath.FromSlash(artifact.Path))); err != nil {
		t.Fatal(err)
	}
	loaded, gotRevision, err := store.Read(ctx, "legacy")
	if err != nil || gotRevision != revision {
		t.Fatalf("legacy read: %s %v", gotRevision, err)
	}
	loaded["name"] = "Edited"
	if _, err = store.Write(ctx, "legacy", loaded, revision); err != nil {
		t.Fatal(err)
	}
	pointer, err := repo.Get(ctx, "projects/legacy")
	if err != nil || pointer.Version != 1 || pointer.Path == artifact.Path {
		t.Fatalf("cutover pointer=%+v err=%v", pointer, err)
	}
	// The legacy canonical file is never overwritten: the cutover writes a new
	// revision path and discards the released legacy bytes after commit.
	if _, err = os.Stat(filepath.Join(root, filepath.FromSlash(artifact.Path))); !os.IsNotExist(err) {
		t.Fatalf("released legacy object still on disk: %v", err)
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM blob_refs WHERE logical_path=$1", artifact.Path).Scan(&count); err != nil || count != 0 {
		t.Fatalf("unowned canonical binding remains: %d %v", count, err)
	}
}

func TestEditorRepositoryCASPublicationRollbackAndRace(t *testing.T) {
	_, repo, transport, pool, _ := editorPersistenceStore(t)
	ctx := context.Background()
	key := "workspace/effect-presets"
	makeArtifact := func(path, content string) media.Artifact {
		t.Helper()
		file := filepath.Join(t.TempDir(), "value.json")
		if err := os.WriteFile(file, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		return media.Artifact{Path: path, File: file, ContentType: "application/json"}
	}
	initial := makeArtifact("editor/resources/first.json", `{"value":1}`)
	first := editor.Resource{Key: key, Path: initial.Path, Revision: editor.Revision([]byte(`{"value":1}`)), Version: 999, UpdatedAt: time.UnixMilli(1000)}
	if err := repo.CAS(ctx, key, "", first); err == nil {
		t.Fatal("CAS accepted outside publication transaction")
	}
	if err := transport.Publish(ctx, []media.Artifact{initial}, func(tx context.Context) error { return repo.CAS(tx, key, "", first) }); err != nil {
		t.Fatal(err)
	}
	stored, err := repo.Get(ctx, key)
	if err != nil || stored.Version != 1 {
		t.Fatalf("repository must generate numeric versions: %+v %v", stored, err)
	}
	abort := errors.New("abort after pointer mutation")
	rolled := makeArtifact("editor/resources/rolled-back.json", `{"value":2}`)
	second := editor.Resource{Key: key, Path: rolled.Path, Revision: editor.Revision([]byte(`{"value":2}`)), UpdatedAt: time.UnixMilli(2000)}
	if err = transport.Publish(ctx, []media.Artifact{rolled}, func(tx context.Context) error {
		if err := repo.CAS(tx, key, first.Revision, second); err != nil {
			return err
		}
		return abort
	}); !errors.Is(err, abort) {
		t.Fatalf("rollback result: %v", err)
	}
	stored, err = repo.Get(ctx, key)
	if err != nil || stored.Path != first.Path || stored.Version != 1 {
		t.Fatalf("pointer escaped rollback: %+v %v", stored, err)
	}
	var refs int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM blob_refs WHERE logical_path=$1", rolled.Path).Scan(&refs); err != nil || refs != 0 {
		t.Fatalf("rolled back ref visible: %d %v", refs, err)
	}
	artifacts := []media.Artifact{makeArtifact("editor/resources/race-a.json", `{"value":3}`), makeArtifact("editor/resources/race-b.json", `{"value":4}`)}
	outcomes := make(chan error, 2)
	var wg sync.WaitGroup
	for i, artifact := range artifacts {
		wg.Add(1)
		go func(i int, artifact media.Artifact) {
			defer wg.Done()
			revision := editor.Revision([]byte([]string{`{"value":3}`, `{"value":4}`}[i]))
			outcomes <- transport.Publish(ctx, []media.Artifact{artifact}, func(tx context.Context) error {
				return repo.CAS(tx, key, first.Revision, editor.Resource{Key: key, Path: artifact.Path, Revision: revision, UpdatedAt: time.UnixMilli(3000)})
			})
		}(i, artifact)
	}
	wg.Wait()
	close(outcomes)
	wins, conflicts := 0, 0
	for err := range outcomes {
		if err == nil {
			wins++
		} else if errors.Is(err, editor.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("database CAS wins=%d conflicts=%d", wins, conflicts)
	}
	stored, err = repo.Get(ctx, key)
	if err != nil || stored.Version != 2 {
		t.Fatalf("race pointer=%+v err=%v", stored, err)
	}
	if err = transport.Publish(ctx, nil, func(tx context.Context) error { return repo.Delete(tx, key, first.Revision) }); !errors.Is(err, editor.ErrConflict) {
		t.Fatalf("stale delete=%v", err)
	}
	if err = transport.Publish(ctx, nil, func(tx context.Context) error { return repo.Delete(tx, key, stored.Revision) }); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Get(ctx, key); !errors.Is(err, blobstore.ErrNotFound) {
		t.Fatalf("deleted pointer=%v", err)
	}
	paths, err := repo.Paths(ctx, "editor/resources/")
	if err != nil || len(paths) != 0 {
		t.Fatalf("deleted/losing revisions in paths: %v %v", paths, err)
	}
}

func TestEditorRepositoryPathsAndListHaveLiteralPrefixes(t *testing.T) {
	store, repo, transport, _, _ := editorPersistenceStore(t)
	ctx := context.Background()
	for _, id := range []string{"prefix_a", "prefixXa"} {
		if _, err := store.Create(ctx, persistenceProject(t, id)); err != nil {
			t.Fatal(err)
		}
	}
	list, err := repo.List(ctx, "projects/prefix_")
	if err != nil || len(list) != 1 || list[0].Key != "projects/prefix_a" {
		t.Fatalf("literal pointer prefix: %+v %v", list, err)
	}
	artifact, _, err := store.Stage(persistenceProject(t, "unpointed"))
	if err != nil {
		t.Fatal(err)
	}
	if err = transport.Publish(ctx, []media.Artifact{artifact}, nil); err != nil {
		t.Fatal(err)
	}
	paths, err := repo.Paths(ctx, "editor/projects/unpointed/")
	if err != nil || len(paths) != 1 || paths[0] != artifact.Path {
		t.Fatalf("Paths must discover all blob_refs, not just pointers: %+v %v", paths, err)
	}
	all, err := repo.List(ctx, "projects/")
	if err != nil || len(all) != 2 {
		t.Fatalf("List must return only resource pointers: %+v %v", all, err)
	}
}

func TestStageRetainsBuildArtifactContract(t *testing.T) {
	store := editor.Store{StagingDir: t.TempDir()}
	p := persistenceProject(t, "build")
	artifact, revision, err := store.Stage(p)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(artifact.File)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Path != "editor/projects/build/project.json" || revision != editor.Revision(data) {
		t.Fatalf("stage=%+v revision=%s", artifact, revision)
	}
	if _, err = os.Stat(artifact.File + ".part"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stage left part file: %v", err)
	}
}

func TestNewProjectRejectsNonfiniteFPSAndUnsafeIDs(t *testing.T) {
	for _, id := range []string{"-leading", "_leading", strings.Repeat("a", 65), "../escape", ""} {
		if _, err := editor.New(id, "Name", 1, 1, 30, time.Now()); err == nil {
			t.Fatalf("accepted id %q", id)
		}
	}
	for _, fps := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 0, -1} {
		if _, err := editor.New("valid", "Name", 1, 1, fps, time.Now()); err == nil {
			t.Fatalf("accepted fps %v", fps)
		}
	}
}

func TestEditorRepositoryPhase05PointersAndCurrentAnchors(t *testing.T) {
	store, repo, transport, pool, root := editorPersistenceStore(t)
	ctx := context.Background()
	channelID := "00000000-0000-4000-8000-000000000001"
	jobID := "00000000-0000-4000-8000-000000000002"
	seedEditorCloneChannel(t, pool, channelID)
	if _, err := pool.Exec(ctx, `INSERT INTO clone_jobs(id,channel_id,source_key,source_snapshot,generation) VALUES($1,$2,'phase05','{}',2)`, jobID, channelID); err != nil {
		t.Fatal(err)
	}
	project := persistenceProject(t, "phase05")
	artifact, revision, err := store.Stage(project)
	if err != nil {
		t.Fatal(err)
	}
	canonical := artifact.Path
	artifact.Path = "clone/" + jobID + "/" + strings.TrimPrefix(revision, "sha256:") + "/" + canonical
	if err = transport.Publish(ctx, []media.Artifact{artifact}, nil); err != nil {
		t.Fatal(err)
	}
	manifestArtifacts := []editor.Object{{"path": artifact.Path, "sha256": strings.TrimPrefix(revision, "sha256:"), "step": "build"}}
	publishLegacy := func(logical string, body []byte, mime string) {
		t.Helper()
		digest := strings.TrimPrefix(editor.Revision(body), "sha256:")
		file := filepath.Join(t.TempDir(), "artifact")
		if err := os.WriteFile(file, body, 0600); err != nil {
			t.Fatal(err)
		}
		path := "clone/" + jobID + "/" + digest + "/" + logical
		if err := transport.Publish(ctx, []media.Artifact{{Path: path, File: file, ContentType: mime}}, nil); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO clone_artifacts(job_id,id,step,path,sha256,bytes) VALUES($1,$2,'build',$3,$4,$5)`, jobID, logical, path, digest, len(body)); err != nil {
			t.Fatal(err)
		}
		manifestArtifacts = append(manifestArtifacts, editor.Object{"path": path, "sha256": digest, "step": "build"})
	}
	mediaHash := editor.Revision([]byte("image"))
	mediaID := "media-" + strings.TrimPrefix(mediaHash, "sha256:")[:32]
	metadata, err := json.Marshal(editor.Object{"id": mediaID, "fileName": "image.png", "sourceFile": "image.png", "mime": "image/png", "size": 5, "contentHash": mediaHash, "createdAt": 1000, "tags": []string{"clone-scene"}, "probe": editor.Object{"kind": "image", "mime": "image/png", "width": 100, "height": 100, "durationSec": 0, "fps": 0}})
	if err != nil {
		t.Fatal(err)
	}
	publishLegacy("editor/media/"+mediaID+"/metadata.json", metadata, "application/json")
	publishLegacy("editor/media/"+mediaID+"/image.png", []byte("image"), "image/png")
	links, err := json.Marshal(editor.Object{"version": "1.0", "mediaIds": []editor.Object{{"id": mediaID, "addedAt": 1000}}})
	if err != nil {
		t.Fatal(err)
	}
	publishLegacy("editor/projects/phase05/links.json", links, "application/json")
	build, err := json.Marshal(editor.Object{"data": editor.Object{"projectId": "phase05"}, "artifacts": manifestArtifacts})
	if err != nil {
		t.Fatal(err)
	}
	anchorsPath := "clone/" + jobID + "/" + strings.Repeat("a", 64) + "/transcript/asr-anchors.json"
	transcript, err := json.Marshal(editor.Object{"artifacts": []editor.Object{{"path": anchorsPath, "sha256": strings.Repeat("a", 64), "step": "transcript"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO clone_steps(job_id,step,status,output_manifest) VALUES($1,'build','waiting',$2),($1,'transcript','succeeded',$3)`, jobID, build, transcript); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO clone_artifacts(job_id,id,step,path,sha256,bytes) VALUES($1,'project','build',$2,$3,1),($1,'anchors','transcript',$4,$5,1),($1,'stale-anchors','transcript',$6,$5,1)`, jobID, artifact.Path, strings.TrimPrefix(revision, "sha256:"), anchorsPath, strings.Repeat("a", 64), "clone/"+jobID+"/stale/transcript/asr-anchors.json"); err != nil {
		t.Fatal(err)
	}
	oldJobID := "00000000-0000-4000-8000-000000000003"
	if _, err = pool.Exec(ctx, `INSERT INTO clone_jobs(id,channel_id,source_key,source_snapshot,generation,superseded_at) VALUES($1,$2,'phase05','{}',1,now())`, oldJobID, channelID); err != nil {
		t.Fatal(err)
	}
	stale, err := json.Marshal(editor.Object{"data": editor.Object{"projectId": "phase05"}, "artifacts": []editor.Object{{"path": "clone/" + oldJobID + "/" + strings.Repeat("b", 64) + "/" + canonical, "sha256": strings.Repeat("b", 64), "step": "build"}, {"path": "clone/" + oldJobID + "/" + strings.Repeat("b", 64) + "/editor/media/stale-only/metadata.json", "sha256": strings.Repeat("b", 64), "step": "build"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO clone_steps(job_id,step,status,output_manifest) VALUES($1,'build','waiting',$2)`, oldJobID, stale); err != nil {
		t.Fatal(err)
	}
	pointer, err := repo.Get(ctx, "projects/phase05")
	if err != nil || pointer.Version != 0 || pointer.Path != artifact.Path || pointer.Revision != revision {
		t.Fatalf("synthetic pointer=%+v err=%v", pointer, err)
	}
	owner, err := repo.ProjectOwner(ctx, "phase05")
	if err != nil || owner != jobID {
		t.Fatalf("owner=%q err=%v", owner, err)
	}
	anchors, err := repo.AnchorsPath(ctx, "phase05")
	if err != nil || anchors != anchorsPath {
		t.Fatalf("anchors selected stale artifact: %q %v", anchors, err)
	}
	resolved, err := repo.ResolvePath(ctx, canonical)
	if err != nil || resolved != artifact.Path {
		t.Fatalf("canonical resolution=%q err=%v", resolved, err)
	}
	paths, err := repo.Paths(ctx, "editor/projects/")
	if err != nil || len(paths) != 2 || paths[0] != "editor/projects/phase05/links.json" || paths[1] != canonical {
		t.Fatalf("phase05 aliases=%v err=%v", paths, err)
	}
	service := editor.Service{Store: store, Repository: repo, Media: transport}
	loaded, gotRevision, err := service.LoadProject(ctx, "phase05")
	if err != nil || gotRevision != revision {
		t.Fatalf("phase05 load=%v %s %v", loaded, gotRevision, err)
	}
	listed, err := service.ListMedia(ctx, "")
	if err != nil || len(listed) != 1 || listed[0].ID != mediaID {
		t.Fatalf("phase05 workspace media=%+v %v", listed, err)
	}
	linked, err := service.ListMedia(ctx, "phase05")
	if err != nil || len(linked) != 1 || linked[0].ID != mediaID {
		t.Fatalf("phase05 project media=%+v %v", linked, err)
	}
	paths, err = repo.Paths(ctx, "editor/media/stale")
	if err != nil || len(paths) != 0 {
		t.Fatalf("stale generation aliases=%v %v", paths, err)
	}
	loaded["name"] = "Edited phase05"
	if _, err = store.Write(ctx, "phase05", loaded, revision); err != nil {
		t.Fatal(err)
	}
	pointer, err = repo.Get(ctx, "projects/phase05")
	if err != nil || pointer.Version != 1 || pointer.Path == artifact.Path {
		t.Fatalf("first edit=%+v %v", pointer, err)
	}
	var retained int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM blob_refs WHERE logical_path=$1`, artifact.Path).Scan(&retained); err != nil || retained != 1 {
		t.Fatalf("clone-owned canonical binding was released: %d %v", retained, err)
	}
	bytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(artifact.Path)))
	if err != nil || editor.Revision(bytes) != revision {
		t.Fatalf("clone-owned bytes changed: %v", err)
	}
	if err = transport.Publish(ctx, nil, func(tx context.Context) error { return repo.Delete(tx, "projects/phase05", pointer.Revision) }); err != nil {
		t.Fatal(err)
	}
	if _, _, err = store.Read(ctx, "phase05"); !errors.Is(err, blobstore.ErrNotFound) {
		t.Fatalf("deleted phase05 project resurrected: %v", err)
	}
	if _, err = repo.ResolvePath(ctx, canonical); !errors.Is(err, blobstore.ErrNotFound) {
		t.Fatalf("deleted canonical alias resolved: %v", err)
	}
	paths, err = repo.Paths(ctx, "editor/projects/phase05/")
	if err != nil || len(paths) != 0 {
		t.Fatalf("deleted phase05 alias listed: %v %v", paths, err)
	}
	if _, err = store.Create(ctx, project); err != nil {
		t.Fatalf("explicit recreate: %v", err)
	}
	pointer, err = repo.Get(ctx, "projects/phase05")
	if err != nil || pointer.Version != 1 {
		t.Fatalf("recreated pointer=%+v %v", pointer, err)
	}
}

func TestEditorRepositoryTemplateDeletionPolicyAndMediaReferences(t *testing.T) {
	_, repo, transport, pool, _ := editorPersistenceStore(t)
	ctx := context.Background()
	id := "00000000-0000-4000-8000-000000000011"
	channelID := "00000000-0000-4000-8000-000000000012"
	project := persistenceProject(t, id)
	project["timeline"] = editor.Object{"items": []editor.Object{{"mediaId": "template-media"}}}
	raw, err := editor.Marshal(project)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO templates(id,name,recipe,project,preview_revision,preview_built_at) VALUES($1,'Template','{"scenes":{"coverMediaId":"cover-media"}}',$2,1,now())`, id, raw); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO owner_channels(id,name,default_template_id) VALUES($1,'Policy owner',$2)`, channelID, id); err != nil {
		t.Fatal(err)
	}
	refs, err := repo.MediaReferences(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, id := range refs {
		got[id] = true
	}
	if !got["template-media"] || !got["cover-media"] {
		t.Fatalf("missing template media protection: %v", refs)
	}
	err = transport.Publish(ctx, nil, func(tx context.Context) error { return repo.DeleteProjectPolicy(tx, id) })
	var apiErr *editor.Error
	if !errors.As(err, &apiErr) || apiErr.Code != "conflict" || apiErr.Extra["templateOfChannel"] != "Policy owner" {
		t.Fatalf("template deletion policy=%v", err)
	}
	if _, err = pool.Exec(ctx, `WITH replacement AS (
 INSERT INTO templates(id,name,recipe) VALUES(gen_random_uuid(),'Mẫu thay thế','{}') RETURNING id
)
UPDATE owner_channels SET default_template_id=(SELECT id FROM replacement) WHERE id=$1`, channelID); err != nil {
		t.Fatal(err)
	}
	abort := errors.New("rollback registry deletion")
	err = transport.Publish(ctx, nil, func(tx context.Context) error {
		if err := repo.DeleteProjectPolicy(tx, id); err != nil {
			return err
		}
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatal(err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM templates WHERE id::text=$1`, id).Scan(&count); err != nil || count != 1 {
		t.Fatalf("registry escaped rollback=%d %v", count, err)
	}
	if err = transport.Publish(ctx, nil, func(tx context.Context) error { return repo.DeleteProjectPolicy(tx, id) }); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM templates WHERE id::text=$1`, id).Scan(&count); err != nil || count != 0 {
		t.Fatalf("registry not removed=%d %v", count, err)
	}
	// Reinsert a malformed reference document independently of the deleted
	// registry row: malformed durable references must stop reclamation.
	if _, err = pool.Exec(ctx, `INSERT INTO templates(id,name,recipe) VALUES($1,'Invalid references','{"mediaIds":"invalid"}')`, id); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.MediaReferences(ctx); err == nil {
		t.Fatal("media reference decode failed open")
	}
	if _, err = pool.Exec(ctx, `DELETE FROM templates WHERE id::text=$1`, id); err != nil {
		t.Fatal(err)
	}
}

func TestStoreRejectedMutationDoesNotAdvancePointer(t *testing.T) {
	store, repo, transport, _, _ := editorPersistenceStore(t)
	ctx := context.Background()
	project := persistenceProject(t, "invalid-mutation")
	revision, err := store.Create(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []editor.Project{nil, {"id": "invalid-mutation", "unserializable": func() {}}} {
		if _, err = store.Write(ctx, "invalid-mutation", invalid, revision); err == nil {
			t.Fatal("accepted invalid project JSON")
		}
	}
	current, err := repo.Get(ctx, "projects/invalid-mutation")
	if err != nil || current.Version != 1 || current.Revision != revision {
		t.Fatalf("rejected mutation advanced pointer: %+v %v", current, err)
	}
	artifact, nextRevision, err := store.Stage(persistenceProject(t, "wrong-digest"))
	if err != nil {
		t.Fatal(err)
	}
	artifact.Path = "editor/resources/wrong-digest.json"
	err = transport.Publish(ctx, []media.Artifact{artifact}, func(tx context.Context) error {
		return repo.CAS(tx, "projects/wrong-digest", "", editor.Resource{Key: "projects/wrong-digest", Path: artifact.Path, Revision: revision})
	})
	if !errors.Is(err, blobstore.ErrDigestMismatch) {
		t.Fatalf("published digest mismatch accepted: actual=%s supplied=%s err=%v", nextRevision, revision, err)
	}
	if _, err = repo.Get(ctx, "projects/wrong-digest"); !errors.Is(err, blobstore.ErrNotFound) {
		t.Fatalf("invalid digest created pointer: %v", err)
	}
}

func TestStoreIdenticalConcurrentSavesConsumeRevision(t *testing.T) {
	store, repo, _, _, _ := editorPersistenceStore(t)
	ctx := context.Background()
	project := persistenceProject(t, "identical")
	project["updatedAt"] = int64(2000) // Equal to the fixed Store.Now clock.
	revision, err := store.Create(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Write(ctx, "identical", project, revision)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	wins, conflicts := 0, 0
	for err := range results {
		if err == nil {
			wins++
		} else if errors.Is(err, editor.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("identical-save wins=%d conflicts=%d", wins, conflicts)
	}
	current, actual, err := store.Read(ctx, "identical")
	if err != nil || actual == revision || current["updatedAt"] != float64(2001) {
		t.Fatalf("revision not consumed: %+v %s %v", current, actual, err)
	}
	pointer, err := repo.Get(ctx, "projects/identical")
	if err != nil || pointer.Version != 2 {
		t.Fatalf("identical-save pointer=%+v %v", pointer, err)
	}
}

func TestEditorRepositoryLegacyMediaLinksAdoptAndPreferPhase05(t *testing.T) {
	store, repo, transport, _, _ := editorPersistenceStore(t)
	ctx := context.Background()
	if _, err := store.Create(ctx, persistenceProject(t, "legacy-links")); err != nil {
		t.Fatal(err)
	}
	publish := func(logical, body string) string {
		t.Helper()
		file := filepath.Join(t.TempDir(), "links.json")
		if err := os.WriteFile(file, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if err := transport.Publish(ctx, []media.Artifact{{Path: logical, File: file, ContentType: "application/json"}}, nil); err != nil {
			t.Fatal(err)
		}
		return editor.Revision([]byte(body))
	}
	legacyOnlyRevision := publish("editor/projects/legacy-only/media-links.json", `{"version":"1.0","mediaIds":[]}`)
	adoptionFile := filepath.Join(t.TempDir(), "adopt.json")
	adoptionBody := []byte(`{"version":"1.0","mediaIds":[{"id":"adopted","addedAt":1}]}`)
	if err := os.WriteFile(adoptionFile, adoptionBody, 0600); err != nil {
		t.Fatal(err)
	}
	if err := transport.Publish(ctx, []media.Artifact{{Path: "editor/resources/adopt-only-links.json", File: adoptionFile, ContentType: "application/json"}}, func(tx context.Context) error {
		return repo.CAS(tx, "projects/legacy-only/links", legacyOnlyRevision, editor.Resource{Key: "projects/legacy-only/links", Path: "editor/resources/adopt-only-links.json", Revision: editor.Revision(adoptionBody), PreviousPath: "editor/projects/legacy-only/links.json"})
	}); err != nil {
		t.Fatalf("old media-links CAS could not adopt fallback: %v", err)
	}
	adoptedOnly, err := repo.Get(ctx, "projects/legacy-only/links")
	if err != nil || adoptedOnly.Version != 1 {
		t.Fatalf("old media-links first pointer=%+v %v", adoptedOnly, err)
	}
	oldPath := "editor/projects/legacy-links/media-links.json"
	oldRevision := publish(oldPath, `{"version":"1.0","mediaIds":[]}`)
	legacy, err := repo.Get(ctx, "projects/legacy-links/links")
	if err != nil || legacy.Path != oldPath || legacy.Revision != oldRevision || legacy.Version != 0 {
		t.Fatalf("legacy links=%+v %v", legacy, err)
	}
	phase05Path := "editor/projects/legacy-links/links.json"
	phase05Revision := publish(phase05Path, `{"version":"1.0","mediaIds":[{"id":"new","addedAt":1}]}`)
	preferred, err := repo.Get(ctx, "projects/legacy-links/links")
	if err != nil || preferred.Path != phase05Path || preferred.Revision != phase05Revision {
		t.Fatalf("phase05 links did not win: %+v %v", preferred, err)
	}
	file := filepath.Join(t.TempDir(), "next.json")
	body := []byte(`{"version":"1.0","mediaIds":[{"id":"next","addedAt":2}]}`)
	if err = os.WriteFile(file, body, 0600); err != nil {
		t.Fatal(err)
	}
	err = transport.Publish(ctx, []media.Artifact{{Path: "editor/resources/next-links.json", File: file, ContentType: "application/json"}}, func(tx context.Context) error {
		return repo.CAS(tx, "projects/legacy-links/links", phase05Revision, editor.Resource{Key: "projects/legacy-links/links", Path: "editor/resources/next-links.json", Revision: editor.Revision(body), PreviousPath: phase05Path})
	})
	if err != nil {
		t.Fatal(err)
	}
	adopted, err := repo.Get(ctx, "projects/legacy-links/links")
	if err != nil || adopted.Version != 1 {
		t.Fatalf("links adoption=%+v %v", adopted, err)
	}
	if err = transport.Publish(ctx, nil, func(tx context.Context) error {
		return repo.Delete(tx, "projects/legacy-links/links", adopted.Revision)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Get(ctx, "projects/legacy-links/links"); !errors.Is(err, blobstore.ErrNotFound) {
		t.Fatalf("old media-links resurrected: %v", err)
	}
	if _, err = repo.ResolvePath(ctx, oldPath); !errors.Is(err, blobstore.ErrNotFound) {
		t.Fatalf("old media-links alias bypassed tombstone: %v", err)
	}
}

type testTemplateProjects struct {
	read  func(context.Context, string) (editor.Project, string, error)
	write func(context.Context, string, editor.Project, string) (string, error)
}

func (p testTemplateProjects) ReadEditorProject(ctx context.Context, id string) (editor.Project, string, error) {
	if p.read == nil {
		return nil, "", blobstore.ErrNotFound
	}
	return p.read(ctx, id)
}
func (p testTemplateProjects) WriteEditorProject(ctx context.Context, id string, project editor.Project, expected string) (string, error) {
	return p.write(ctx, id, project, expected)
}

func TestStoreTemplateProjectsUseAuthoritativeDecimalRevision(t *testing.T) {
	ctx := context.Background()
	project := persistenceProject(t, "template")
	store := editor.Store{Templates: testTemplateProjects{
		read: func(context.Context, string) (editor.Project, string, error) { return project, "7", nil },
		write: func(_ context.Context, id string, next editor.Project, expected string) (string, error) {
			if id != "template" || expected != "7" || next["name"] != "Changed template" {
				t.Fatalf("invalid template write: %s %s %+v", id, expected, next)
			}
			return "8", nil
		},
	}}
	loaded, revision, err := store.Read(ctx, "template")
	if err != nil || revision != "7" || loaded["id"] != "template" {
		t.Fatalf("template read=%+v %q %v", loaded, revision, err)
	}
	next := editor.Clone(loaded)
	next["name"] = "Changed template"
	revision, err = store.Write(ctx, "template", next, "7")
	if err != nil || revision != "8" {
		t.Fatalf("template write did not preserve decimal CAS: %q %v", revision, err)
	}
}

func TestStoreTemplateErrorsDoNotFallBackToWorkspace(t *testing.T) {
	failure := errors.New("authoritative template is unreadable")
	store := editor.Store{Templates: testTemplateProjects{read: func(context.Context, string) (editor.Project, string, error) { return nil, "", failure }}}
	ctx := context.Background()
	if _, _, err := store.Read(ctx, "template"); !errors.Is(err, failure) {
		t.Fatalf("template read error swallowed: %v", err)
	}
	if _, err := store.Write(ctx, "template", persistenceProject(t, "template"), "7"); !errors.Is(err, failure) {
		t.Fatalf("template write error swallowed: %v", err)
	}
}

func TestStoreMissingTemplateFallsBackToWorkspace(t *testing.T) {
	store, _, _, _, _ := editorPersistenceStore(t)
	store.Templates = testTemplateProjects{}
	ctx := context.Background()
	project := persistenceProject(t, "workspace-fallback")
	revision, err := store.Create(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	loaded, actual, err := store.Read(ctx, "workspace-fallback")
	if err != nil || actual != revision || loaded["id"] != "workspace-fallback" {
		t.Fatalf("missing template fallback=%+v %s %v", loaded, actual, err)
	}
	loaded["name"] = "Workspace edited"
	if _, err = store.Write(ctx, "workspace-fallback", loaded, revision); err != nil {
		t.Fatalf("missing template write fallback=%v", err)
	}
}

func TestEditorRepositoryLegacyResourcesDeleteWithoutResurrection(t *testing.T) {
	_, repo, transport, _, _ := editorPersistenceStore(t)
	ctx := context.Background()
	cases := []struct{ key, path string }{
		{"workspace/effect-presets", "editor/app/effect-presets.json"},
		{"workspace/meta", "editor/.freecut-workspace.json"},
		{"workspace/media-index", "editor/media-index.json"},
		{"workspace/media-reclaim-pending", "editor/media-reclaim-pending.json"},
		{"projects/project/sidecars/animation-presets", "editor/projects/project/animation-presets.json"},
		{"exports/project/export", "editor/projects/project/exports/export/meta.json"},
		{"exports/project/export/diagnostics", "editor/projects/project/exports/export/gate-diagnostics.json"},
	}
	for _, test := range cases {
		t.Run(test.key, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "resource.json")
			body := []byte(`{"value":"legacy"}`)
			if err := os.WriteFile(file, body, 0600); err != nil {
				t.Fatal(err)
			}
			if err := transport.Publish(ctx, []media.Artifact{{Path: test.path, File: file, ContentType: "application/json"}}, nil); err != nil {
				t.Fatal(err)
			}
			resource, err := repo.Get(ctx, test.key)
			// A deleted export also hides all of its diagnostic aliases.
			if strings.HasSuffix(test.key, "/diagnostics") {
				if !errors.Is(err, blobstore.ErrNotFound) {
					t.Fatalf("deleted export diagnostics visible: %+v %v", resource, err)
				}
				return
			}
			if err != nil || resource.Version != 0 || resource.Path != test.path || resource.Revision != editor.Revision(body) {
				t.Fatalf("legacy resource=%+v %v", resource, err)
			}
			if err = transport.Publish(ctx, nil, func(tx context.Context) error { return repo.Delete(tx, test.key, resource.Revision) }); err != nil {
				t.Fatal(err)
			}
			if _, err = repo.Get(ctx, test.key); !errors.Is(err, blobstore.ErrNotFound) {
				t.Fatalf("deleted resource resurrected: %v", err)
			}
			if _, err = repo.ResolvePath(ctx, test.path); !errors.Is(err, blobstore.ErrNotFound) {
				t.Fatalf("deleted canonical fallback visible: %v", err)
			}
		})
	}
}
