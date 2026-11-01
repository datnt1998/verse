package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/serverstorage"
	"github.com/datnt1998/verse/apps/viderse/internal/storage"
	"github.com/datnt1998/verse/platform/webguard"
)

// 01-ia-flows W13 › Lưu trữ › “Chạy trên máy chủ” + 08-dev-prompt V25 FE:
// chế độ máy chủ = bản chạy trên máy chủ (PUBLIC_URL) HOẶC kho media ở S3.
// Bản desktop để kho local giữ V22: thao tác thư mục mở, và /api/storage/server
// trả 404 để giao diện chọn thẻ lưu trữ trên máy (api/storageServer.ts chỉ 404
// mới chọn bản desktop).

type fakeServerStorage struct{ snap serverstorage.Snapshot }

func (f *fakeServerStorage) Snapshot(context.Context, bool) (serverstorage.Snapshot, error) {
	return f.snap, nil
}
func (f *fakeServerStorage) Test(context.Context, string, serverstorage.S3Config) (serverstorage.TestResult, error) {
	return serverstorage.TestResult{}, nil
}
func (f *fakeServerStorage) Probe(context.Context, string, string, bool, string) (serverstorage.TestResult, error) {
	return serverstorage.TestResult{}, nil
}
func (f *fakeServerStorage) Save(context.Context, string, serverstorage.S3Config, string) (serverstorage.SaveResult, error) {
	return serverstorage.SaveResult{}, nil
}
func (f *fakeServerStorage) Migrate(context.Context) (string, error) { return "", nil }
func (f *fakeServerStorage) MigrationAction(context.Context, string, string) error {
	return nil
}

func storageModeRig(t *testing.T, hosted, remote bool) (*testGateway, string) {
	t.Helper()
	tg, store, _, cookie := newStorageHandlers(t)
	if hosted {
		tg.g.SetProxyConfig(&webguard.Config{PublicURL: &url.URL{Scheme: "https", Host: "vid.aarverse.studio"}})
	} else {
		tg.g.SetProxyConfig(&webguard.Config{})
	}
	store.snap = storage.Snapshot{Projects: storage.Location{Path: "/tmp/p", Online: true,
		Volume: storage.Volume{Name: "Macintosh HD", Kind: storage.VolumeInternal, FS: "APFS", FreeBytes: 1 << 30}}}
	mode := func() bool { return tg.g.Hosted() || remote }
	tg.g.SetServerStorageMode(mode)
	MountServerStorage(tg.g, &fakeServerStorage{snap: serverstorage.Snapshot{Role: "admin"}}, mode)
	return tg, cookie
}

func TestStorageModeHostedDiskIsServerView(t *testing.T) {
	tg, cookie := storageModeRig(t, true, false)
	if w := tg.do(http.MethodGet, "/api/storage/server", cookie, ""); w.Code != http.StatusOK {
		t.Fatalf("hosted: /api/storage/server = %d, muốn 200", w.Code)
	}
	w := tg.do(http.MethodGet, "/api/storage", cookie, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("hosted: /api/storage = %d, muốn 403", w.Code)
	}
	var body struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Code != "storage_path_unavailable" {
		t.Fatalf("hosted: code = %q", body.Code)
	}
}

func TestStorageModeDesktopDiskKeepsLocalCard(t *testing.T) {
	tg, cookie := storageModeRig(t, false, false)
	if w := tg.do(http.MethodGet, "/api/storage/server", cookie, ""); w.Code != http.StatusNotFound {
		t.Fatalf("desktop: /api/storage/server = %d, muốn 404 (rơi về thẻ V22/V30)", w.Code)
	}
	if w := tg.do(http.MethodGet, "/api/storage", cookie, ""); w.Code != http.StatusOK {
		t.Fatalf("desktop: /api/storage = %d, muốn 200 (chọn thư mục dùng được)", w.Code)
	}
}

func TestStorageModeDesktopS3IsServerView(t *testing.T) {
	tg, cookie := storageModeRig(t, false, true)
	if w := tg.do(http.MethodGet, "/api/storage/server", cookie, ""); w.Code != http.StatusOK {
		t.Fatalf("desktop+S3: /api/storage/server = %d, muốn 200", w.Code)
	}
	if w := tg.do(http.MethodGet, "/api/storage", cookie, ""); w.Code != http.StatusForbidden {
		t.Fatalf("desktop+S3: /api/storage = %d, muốn 403", w.Code)
	}
}
