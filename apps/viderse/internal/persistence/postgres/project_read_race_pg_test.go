package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/blobstore"
)

// movingMedia chạy một save canonical trước Acquire thứ i — đúng khoảng giữa
// "đọc head" và "đọc bytes" của editor.Store.readWorkspace, nơi hai snapshot
// READ COMMITTED tách rời nhau.
type movingMedia struct {
	*media.Service
	mu    sync.Mutex
	moves []func() error
	calls int
}

func (m *movingMedia) Acquire(ctx context.Context, path string) (*media.Handle, error) {
	m.mu.Lock()
	i := m.calls
	m.calls++
	m.mu.Unlock()
	if i < len(m.moves) {
		if err := m.moves[i](); err != nil {
			return nil, err
		}
	}
	return m.Service.Acquire(ctx, path)
}

func (m *movingMedia) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// vanishingMedia xoá file local ngay sau khi Acquire trả handle — đúng điều
// discardReleasedLocal làm với revision cũ sau khi CAS dời head.
type vanishingMedia struct {
	*media.Service
	calls int
}

func (m *vanishingMedia) Acquire(ctx context.Context, path string) (*media.Handle, error) {
	m.calls++
	h, err := m.Service.Acquire(ctx, path)
	if err != nil {
		return nil, err
	}
	if err := os.Remove(h.Path); err != nil {
		return nil, err
	}
	return h, nil
}

// countingRepository đếm số lần đọc head để chứng minh reader không lặp khi head
// không đổi (mỗi lượt đọc = 1 Get, mỗi lần kiểm tra snapshot = 1 Get).
type countingRepository struct {
	*postgres.EditorRepository
	gets int
}

func (r *countingRepository) Get(ctx context.Context, key string) (editor.Resource, error) {
	r.gets++
	return r.EditorRepository.Get(ctx, key)
}

type projectFixture struct {
	t         *testing.T
	f         *cloneDeleteFixture
	ctx       context.Context
	pid       string
	revisions []string
	paths     []string
	moves     int
	ran       int
}

func newProjectFixture(t *testing.T, pid string, bodies ...string) *projectFixture {
	t.Helper()
	f := newCloneDeleteFixture(t)
	ctx := context.Background()
	pf := &projectFixture{t: t, f: f, ctx: ctx, pid: pid, moves: len(bodies) - 1}
	for i, body := range bodies {
		path := "editor/projects/" + pid + "/revisions/" + fmt.Sprintf("r%d.json", i+1)
		file := filepath.Join(t.TempDir(), "revision.json")
		if err := os.WriteFile(file, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if err := f.files.Publish(ctx, []media.Artifact{{Path: path, File: file, ContentType: "application/json"}}, nil); err != nil {
			t.Fatal(err)
		}
		pf.paths = append(pf.paths, path)
		pf.revisions = append(pf.revisions, editor.Revision([]byte(body)))
	}
	if err := f.files.Publish(ctx, nil, func(tx context.Context) error {
		return f.editor.CAS(tx, "projects/"+pid, "", editor.Resource{Key: "projects/" + pid, Path: pf.paths[0], Revision: pf.revisions[0]})
	}); err != nil {
		t.Fatal(err)
	}
	return pf
}

func (pf *projectFixture) head(i int) editor.Resource {
	return editor.Resource{Key: "projects/" + pf.pid, Path: pf.paths[i], Revision: pf.revisions[i]}
}

// move trả về save canonical dời head từ revision i sang i+1, kèm nhả binding cũ
// (CAS releaseBinding trong cùng transaction).
func (pf *projectFixture) move(i int) func() error {
	return func() error {
		pf.ran++
		return pf.f.files.Publish(pf.ctx, nil, func(tx context.Context) error {
			return pf.f.editor.CAS(tx, "projects/"+pf.pid, pf.revisions[i], pf.head(i+1))
		})
	}
}

func (pf *projectFixture) store(m editor.Media) editor.Store {
	return editor.Store{Media: m, Repository: pf.f.editor, StagingDir: pf.t.TempDir()}
}

// Mỗi lượt đọc thấy head cũ nhưng binding/file đã bị save kế tiếp nhả. Reader
// phải lặp theo số lần head thật sự đổi, không phải số lần cố định: 4 lần dời
// liên tiếp vẫn đọc được bản mới nhất (bản cũ dừng ở 3 lượt).
func TestProjectReadSurvivesSuccessiveHeadMoves(t *testing.T) {
	pf := newProjectFixture(t, "read-race",
		`{"id":"read-race","name":"one"}`,
		`{"id":"read-race","name":"two"}`,
		`{"id":"read-race","name":"three"}`,
		`{"id":"read-race","name":"four"}`,
		`{"id":"read-race","name":"five"}`,
	)
	raced := &movingMedia{Service: pf.f.files}
	for i := range pf.moves {
		raced.moves = append(raced.moves, pf.move(i))
	}
	project, revision, err := pf.store(raced).Read(pf.ctx, pf.pid)
	if err != nil {
		t.Fatalf("đọc project khi head dời %d lần liên tiếp: %v", pf.moves, err)
	}
	if pf.ran != pf.moves {
		t.Fatalf("chỉ chạy %d/%d lần dời head", pf.ran, pf.moves)
	}
	if raced.count() != pf.moves+1 {
		t.Fatalf("số lượt đọc = %d, muốn %d", raced.count(), pf.moves+1)
	}
	last := len(pf.revisions) - 1
	if project["name"] != "five" || revision != pf.revisions[last] {
		t.Fatalf("đọc project = %v @ %s, muốn bản năm", project["name"], revision)
	}
	// Chứng minh đây là race thật: revision cũ đã bị nhả khỏi manifest.
	if _, err := pf.f.files.Acquire(pf.ctx, pf.paths[0]); !errors.Is(err, blobstore.ErrNotFound) {
		t.Fatalf("revision cũ vẫn đọc được: %v", err)
	}
}

// Head đứng yên thì missing là mất thật: trả nguyên lỗi, không lặp. Ở đây head
// còn nguyên nhưng file local của revision vừa bị xoá.
func TestProjectReadStopsWhenHeadIsUnchanged(t *testing.T) {
	pf := newProjectFixture(t, "steady-head", `{"id":"steady-head","name":"one"}`)
	vanishing := &vanishingMedia{Service: pf.f.files}
	_, _, err := pf.store(vanishing).Read(pf.ctx, pf.pid)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("đọc bytes mất: %v, muốn os.ErrNotExist", err)
	}
	if vanishing.calls != 1 {
		t.Fatalf("số lượt Acquire = %d, muốn 1 (head không đổi thì không lặp)", vanishing.calls)
	}
}

// Head vắng mặt ở cả hai lần đọc (project không tồn tại) nên chỉ một lượt.
func TestProjectReadUnknownIDOneAttempt(t *testing.T) {
	pf := newProjectFixture(t, "known-head", `{"id":"known-head","name":"one"}`)
	repo := &countingRepository{EditorRepository: pf.f.editor}
	store := editor.Store{Media: pf.f.files, Repository: repo, StagingDir: t.TempDir()}
	if _, _, err := store.Read(pf.ctx, "khong-co-project"); !errors.Is(err, blobstore.ErrNotFound) {
		t.Fatalf("project thiếu: %v, muốn ErrNotFound", err)
	}
	if repo.gets != 2 {
		t.Fatalf("số lần đọc head = %d, muốn 2 (một lượt đọc + một lần so snapshot)", repo.gets)
	}
}

// Project đã bị xoá (tombstone) vẫn 404 sau khi đọc lại head, không bị che.
func TestProjectReadKeepsNotFoundForDeletedProject(t *testing.T) {
	pf := newProjectFixture(t, "deleted-project", `{"id":"deleted-project","name":"gone"}`)
	if err := pf.f.files.Publish(pf.ctx, nil, func(tx context.Context) error {
		return pf.f.editor.Delete(tx, "projects/"+pf.pid, pf.revisions[0])
	}); err != nil {
		t.Fatal(err)
	}
	repo := &countingRepository{EditorRepository: pf.f.editor}
	store := editor.Store{Media: pf.f.files, Repository: repo, StagingDir: t.TempDir()}
	if _, _, err := store.Read(pf.ctx, pf.pid); !errors.Is(err, blobstore.ErrNotFound) {
		t.Fatalf("project đã xoá: %v, muốn ErrNotFound", err)
	}
	if repo.gets != 2 {
		t.Fatalf("số lần đọc head = %d, muốn 2", repo.gets)
	}
}
