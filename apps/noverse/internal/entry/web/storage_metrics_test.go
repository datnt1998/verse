package web

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
)

func TestStorageMetricsAdminPrivacy(t *testing.T) {
	tg := newConfigTestGateway(t)
	local, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	measured := blobstore.NewMetrics(local, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second)
	_, err = measured.Put(context.Background(), "private-object", strings.NewReader("secret bytes"), blobstore.PutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	tg.g.SetAudioStore(&PGAudioDeps{Blobs: measured})
	const path = "/api/system/storage"
	if res := tg.do(http.MethodGet, path, "", ""); res.Code != 401 {
		t.Fatalf("anonymous metrics = %d", res.Code)
	}
	for _, role := range []string{"reader", "writer"} {
		if res := tg.do(http.MethodGet, path, tg.login(role, role+"-pass"), ""); res.Code != 403 {
			t.Fatalf("%s metrics = %d", role, res.Code)
		}
	}
	res := tg.do(http.MethodGet, path, tg.login("admin", "admin-pass"), "")
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"op":"put"`) {
		t.Fatalf("admin metrics = %d %s", res.Code, res.Body.String())
	}
	if strings.Contains(res.Body.String(), "private-object") || strings.Contains(res.Body.String(), "secret bytes") {
		t.Fatal("diagnostics disclosed object identity/content")
	}
	if res.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("diagnostics cacheable")
	}
}
