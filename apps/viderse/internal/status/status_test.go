package status

import (
	"context"
	"errors"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

type staticSource struct {
	rep Report
	err error
}

func (s staticSource) Report(context.Context) (Report, error) { return s.rep, s.err }

type staticSettings struct {
	paused bool
	err    error
}

func (s staticSettings) QueuePaused(context.Context) (bool, error) { return s.paused, s.err }

func TestSnapshotAggregatesRegisteredSources(t *testing.T) {
	svc := New(staticSettings{}, nil)
	svc.RegisterSource(staticSource{rep: Report{
		Running: 2, Queued: 5, ReviewCount: 3,
		Attention: []Attention{{Kind: domain.AttentionLoginExpired, Text: "Đăng nhập lại", Href: "/settings/tools"}},
	}})
	svc.RegisterSource(staticSource{rep: Report{Running: 1, Queued: 4, ReviewCount: 1}})

	snap, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.Running != 3 || snap.Queued != 9 || snap.ReviewCount != 4 {
		t.Fatalf("tổng sai: %+v", snap)
	}
	if len(snap.Attention) != 1 || snap.Attention[0].Kind != domain.AttentionLoginExpired {
		t.Fatalf("attention sai: %+v", snap.Attention)
	}
}

// Một nguồn lỗi không được làm mất số liệu của các nguồn khác.
func TestSnapshotSkipsFailingSource(t *testing.T) {
	svc := New(staticSettings{}, nil)
	svc.sourceErr = func(string, error) {}
	svc.RegisterSource(staticSource{rep: Report{Running: 1}})
	svc.RegisterSource(staticSource{err: errors.New("db chập")})

	snap, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.Running != 1 {
		t.Fatalf("nguồn lỗi làm mất số liệu: %+v", snap)
	}
}

// Lỗi đọc settings trả ra ngoài — handler đổi thành 500, không giả false.
func TestSnapshotPropagatesSettingsError(t *testing.T) {
	svc := New(staticSettings{err: errors.New("pool đóng")}, nil)
	if _, err := svc.Snapshot(context.Background()); err == nil {
		t.Fatal("lỗi settings phải trả ra ngoài")
	}
}

// Bump tăng revision và phát đúng rev mới qua publish.
func TestBumpPublishesNewRevision(t *testing.T) {
	var published []uint64
	svc := New(staticSettings{}, func(rev uint64) { published = append(published, rev) })

	if rev := svc.Bump(); rev != 1 {
		t.Fatalf("rev = %d", rev)
	}
	if rev := svc.Bump(); rev != 2 {
		t.Fatalf("rev = %d", rev)
	}
	if len(published) != 2 || published[0] != 1 || published[1] != 2 {
		t.Fatalf("published = %v", published)
	}
	if svc.Rev() != 2 {
		t.Fatalf("Rev = %d", svc.Rev())
	}

	snap, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.Rev != 2 {
		t.Fatalf("snapshot.Rev = %d", snap.Rev)
	}
}

// Attention không có mục nào vẫn là mảng rỗng (JSON [] thay vì null) để web
// lặp mà không cần kiểm nil.
func TestSnapshotEmptyAttentionIsEmptySlice(t *testing.T) {
	svc := New(staticSettings{}, nil)
	snap, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.Attention == nil {
		t.Fatal("Attention phải là slice rỗng, không nil")
	}
}
