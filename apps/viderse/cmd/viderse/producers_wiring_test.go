package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/channels"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/status"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/jackc/pgx/v5/pgxpool"
)

type producerFixture struct {
	pool     *pgxpool.Pool
	repo     *postgres.BlobRepository
	media    *media.Service
	config   blobstore.Config
	cacheDir string
}

func producerMedia(t *testing.T, driver string) *producerFixture {
	t.Helper()
	pool, _ := provisionCLI(t)
	cfg := blobstore.Config{Driver: driver, Delivery: "proxy", LocalRoot: t.TempDir()}
	if driver == "s3" {
		endpoint := os.Getenv("S3_TEST_ENDPOINT")
		if endpoint == "" {
			if os.Getenv("REQUIRE_S3_TEST") != "" {
				t.Fatal("S3_TEST_ENDPOINT is required for producer acceptance")
			}
			t.Skip("S3_TEST_ENDPOINT is not configured")
		}
		parsed, err := blobstore.ParseEnv(func(k string) (string, bool) {
			values := map[string]string{"STORAGE_DRIVER": "s3", "S3_PROVIDER": "garage", "S3_ENDPOINT": endpoint, "S3_REGION": os.Getenv("S3_TEST_REGION"), "S3_BUCKET": os.Getenv("S3_TEST_BUCKET"), "S3_ACCESS_KEY_ID": os.Getenv("S3_TEST_ACCESS_KEY"), "S3_SECRET_ACCESS_KEY": os.Getenv("S3_TEST_SECRET_KEY"), "S3_ALLOW_INSECURE_ENDPOINT": "true", "BLOB_DELIVERY": "proxy"}
			v, ok := values[k]
			return v, ok
		}, "")
		if err != nil {
			t.Fatal(err)
		}
		cfg = parsed
	}
	repo := postgres.NewBlobRepository(pool)
	if _, err := repo.Initialize(context.Background(), media.Descriptor{ID: media.NamespaceID(cfg), Config: cfg}); err != nil {
		t.Fatal(err)
	}
	fx := &producerFixture{pool: pool, repo: repo, config: cfg, cacheDir: t.TempDir()}
	fx.reopen(t)
	return fx
}

func (f *producerFixture) reopen(t *testing.T) {
	t.Helper()
	cache, err := media.NewCache(f.cacheDir, 64<<20)
	if err != nil {
		t.Fatal(err)
	}
	f.media = media.New(f.repo, cache, func(d media.Descriptor) (blobstore.BlobStore, error) {
		// Persistence deliberately omits credentials from descriptors. The
		// fixture supplies the same env credentials as the production factory.
		if d.Config.Driver == "s3" {
			d.Config.S3.AccessKeyID = f.config.S3.AccessKeyID
			d.Config.S3.SecretAccessKey = f.config.S3.SecretAccessKey
		}
		return blobstore.Open(d.Config)
	})
}

func producerPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 24, 16))
	for y := range 16 {
		for x := range 24 {
			img.Set(x, y, color.RGBA{uint8(x * 10), uint8(y * 10), 100, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func producerArtifact(t *testing.T, logical string, data []byte) media.Artifact {
	t.Helper()
	file := filepath.Join(t.TempDir(), "input.png")
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	return media.Artifact{Path: logical, File: file, ContentType: "image/png", Filename: "ảnh.png"}
}

func TestProducerPublicationRollback(t *testing.T) {
	f := producerMedia(t, "local")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	rejected := errors.New("reject publication after domain write")
	store := postgres.NewLibraryStore(f.pool)
	data := producerPNG(t)
	digest := sha256.Sum256(data)
	overlay := domain.Overlay{ID: "10000000-0000-4000-8000-000000000001", SHA256: hex.EncodeToString(digest[:]), Name: "Atomic", FilePath: "library/overlays/atomic.png", Mime: "image/png", Bytes: int64(len(data)), Background: domain.BgNone, CreatedAt: time.Now()}
	t.Run("insert", func(t *testing.T) {
		err := f.media.Publish(ctx, []media.Artifact{producerArtifact(t, overlay.FilePath, data)}, func(txctx context.Context) error {
			if err := store.InsertOverlay(txctx, overlay); err != nil {
				return err
			}
			return rejected
		})
		if !errors.Is(err, rejected) {
			t.Fatalf("callback error = %v", err)
		}
		row, _, err := store.OverlayBySHA(ctx, overlay.SHA256)
		if err != nil || row != nil {
			t.Fatalf("rolled-back publication left overlay row: %+v %v", row, err)
		}
		if _, err := f.repo.Resolve(ctx, media.NamespaceID(f.config), overlay.FilePath); err == nil {
			t.Fatal("rolled-back publication left manifest entry")
		}
	})
	if err := store.InsertOverlay(ctx, overlay); err != nil {
		t.Fatal(err)
	}
	if err := store.RemoveOverlay(ctx, overlay.ID); err != nil {
		t.Fatal(err)
	}
	t.Run("restore", func(t *testing.T) {
		err := f.media.Publish(ctx, []media.Artifact{producerArtifact(t, overlay.FilePath, data)}, func(txctx context.Context) error {
			if err := store.RestoreOverlay(txctx, overlay.ID, "Changed"); err != nil {
				return err
			}
			return rejected
		})
		if !errors.Is(err, rejected) {
			t.Fatal(err)
		}
		row, removed, err := store.OverlayBySHA(ctx, overlay.SHA256)
		if err != nil || !removed || row.Name != "Atomic" {
			t.Fatalf("restore escaped publication transaction: %+v removed=%v %v", row, removed, err)
		}
		if _, err := f.repo.Resolve(ctx, media.NamespaceID(f.config), overlay.FilePath); err == nil {
			t.Fatal("restore rollback left manifest")
		}
	})
	t.Run("backdrop", func(t *testing.T) {
		bs := postgres.NewBackdropStore(f.pool)
		now := time.Now()
		if _, err := bs.InsertVideo(ctx, domain.BackdropVideo{ID: "twitch:Atomic", Platform: domain.PlatformTwitch, URL: "https://www.twitch.tv/x/clip/Atomic", Title: "Atomic", Segment: domain.BackdropSegment{FromSec: 0, ToSec: 10}, State: domain.BackdropDownloading, AddedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		gen, ok, err := bs.ClaimDownload(ctx, "twitch:Atomic", "producer", now.Add(time.Minute), now, false)
		if err != nil || !ok {
			t.Fatalf("claim: %v %v", ok, err)
		}
		logical := "library/backdrops/atomic.mp4"
		err = f.media.Publish(ctx, []media.Artifact{producerArtifact(t, logical, data)}, func(txctx context.Context) error {
			ok, err := bs.FinishDownload(txctx, "twitch:Atomic", "producer", gen, logical, 480, int64(len(data)), now)
			if err != nil {
				return err
			}
			if !ok {
				return media.ErrConflict
			}
			return rejected
		})
		if !errors.Is(err, rejected) {
			t.Fatal(err)
		}
		row, _, err := bs.Video(ctx, "twitch:Atomic")
		if err != nil || row.State == domain.BackdropReady || row.FilePath != "" {
			t.Fatalf("FinishDownload escaped transaction: %+v %v", row, err)
		}
		if _, err := f.repo.Resolve(ctx, media.NamespaceID(f.config), logical); err == nil {
			t.Fatal("backdrop rollback left manifest")
		}
	})
	t.Run("identity", func(t *testing.T) {
		cs := postgres.NewChannelStore(f.pool)
		id := "20000000-0000-4000-8000-000000000001"
		template, err := library.New(library.Deps{Store: postgres.NewLibraryStore(f.pool), Media: f.media}).Create(ctx, "Mẫu kiểm thử nhận diện")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := cs.CreateChannel(ctx, channels.CreateInput{ID: id, Name: "Before", Language: domain.LangVI, PerDay: 5, DefaultTemplateID: template.ID}); err != nil {
			t.Fatal(err)
		}
		token := "30000000-0000-4000-8000-000000000001"
		if ok, err := cs.AcquireIdentityLease(ctx, channels.LeaseInput{ChannelID: id, Part: domain.IdentityAvatar, Owner: "producer", Token: token, Until: time.Now().Add(time.Minute)}); err != nil || !ok {
			t.Fatalf("lease: %v %v", ok, err)
		}
		logical := "channels/" + id + "/40000000-0000-4000-8000-000000000001-avatar.png"
		err = f.media.Publish(ctx, []media.Artifact{producerArtifact(t, logical, data)}, func(txctx context.Context) error {
			ok, err := cs.PublishIdentity(txctx, channels.PublishInput{ChannelID: id, Token: token, Name: "After", ExpectedName: "Before", AvatarPath: logical})
			if err != nil {
				return err
			}
			if !ok {
				return media.ErrConflict
			}
			return rejected
		})
		if !errors.Is(err, rejected) {
			t.Fatal(err)
		}
		row, _, err := cs.GetChannel(ctx, id)
		if err != nil || row.Name != "Before" || row.AvatarPath != nil {
			t.Fatalf("PublishIdentity escaped transaction: %+v %v", row, err)
		}
		if _, err := f.repo.Resolve(ctx, media.NamespaceID(f.config), logical); err == nil {
			t.Fatal("identity rollback left manifest")
		}
	})
}

type producerRunner struct{}

func (producerRunner) Run(ctx context.Context, file string, args []string) (string, error) {
	b, err := exec.CommandContext(ctx, file, args...).Output()
	return string(b), err
}

type producerBins struct{ ffmpeg, ffprobe string }

func (b producerBins) Tools(context.Context) (string, string, bool) { return b.ffmpeg, b.ffprobe, true }

func producerLibrary(t *testing.T, f *producerFixture) *library.Service {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal(err)
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal(err)
	}
	return library.New(library.Deps{Store: postgres.NewLibraryStore(f.pool), Media: f.media, Runner: producerRunner{}, Bins: producerBins{ffmpeg, ffprobe}})
}

func producerGateway(t *testing.T, pool *pgxpool.Pool) (*web.Gateway, *http.Cookie) {
	t.Helper()
	accounts := identity.NewStore(filepath.Join(t.TempDir(), "accounts.json"), identity.Options{})
	if _, err := accounts.CreateUser(identity.NewUser{Username: "admin", Role: identity.RoleAdmin, Password: "admin-pass"}); err != nil {
		t.Fatal(err)
	}
	hub := web.NewEventHub()
	t.Cleanup(hub.Close)
	g := web.NewGateway(accounts, hub, status.New(postgres.NewSettingsStore(pool), nil))
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"username":"admin","password":"admin-pass"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login: %d %s", w.Code, w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == "verse_session" {
			return g, c
		}
	}
	t.Fatal("login omitted session cookie")
	return nil, nil
}

func assertProducerHTTP(t *testing.T, g *web.Gateway, cookie *http.Cookie, url string, want []byte) {
	t.Helper()
	for _, tc := range []struct {
		method, rng string
		status      int
		body        []byte
	}{
		{http.MethodGet, "", 200, want}, {http.MethodHead, "", 200, nil},
		{http.MethodGet, "bytes=1-7", 206, want[1:8]},
		{http.MethodGet, "bytes=-5", 206, want[len(want)-5:]},
		{http.MethodGet, "bytes=999999999-", 416, nil},
	} {
		req := httptest.NewRequest(tc.method, url, nil)
		req.AddCookie(cookie)
		if tc.rng != "" {
			req.Header.Set("Range", tc.rng)
		}
		w := httptest.NewRecorder()
		g.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Fatalf("%s %s: %d %s", tc.method, tc.rng, w.Code, w.Body.String())
		}
		if tc.status != 416 && !bytes.Equal(w.Body.Bytes(), tc.body) {
			t.Fatalf("%s %s bytes differ", tc.method, tc.rng)
		}
		if w.Header().Get("Location") != "" {
			t.Fatal("proxy leaked storage URL")
		}
	}
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous media: %d", w.Code)
	}
}

type producerDrawer struct{}

func (producerDrawer) QuotaReset() (time.Time, bool) { return time.Time{}, false }
func (producerDrawer) Draw(_ context.Context, req imagegen.Request) (imagegen.Receipt, error) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 20))
	img.Set(0, 0, color.White)
	out, err := os.Create(req.Output)
	if err != nil {
		return imagegen.Receipt{}, err
	}
	encErr := png.Encode(out, img)
	closeErr := out.Close()
	if encErr != nil {
		return imagegen.Receipt{}, encErr
	}
	return imagegen.Receipt{}, closeErr
}

type producerAI struct{}

func (producerAI) Director() []channels.TextModel { return nil }
func (producerAI) Drawer() channels.Drawer        { return producerDrawer{} }

func TestProducerBackendColdCacheHTTP(t *testing.T) {
	for _, driver := range []string{"local", "s3"} {
		t.Run(driver, func(t *testing.T) { testProducerBackendColdCacheHTTP(t, driver) })
	}
}

func testProducerBackendColdCacheHTTP(t *testing.T, driver string) {
	f := producerMedia(t, driver)
	ctx := context.Background()
	lib := producerLibrary(t, f)
	data := producerPNG(t)
	overlay, reused, err := lib.AddOverlayReader(ctx, "ảnh.png", "Một ảnh", bytes.NewReader(data), int64(len(data)))
	if err != nil || reused {
		t.Fatalf("add: %v reused=%v", err, reused)
	}
	if err := lib.RemoveOverlay(ctx, overlay.ID); err != nil {
		t.Fatal(err)
	}
	restored, reused, err := lib.AddOverlayReader(ctx, "ảnh.png", "Đã khôi phục", bytes.NewReader(data), int64(len(data)))
	if err != nil || !reused || restored.ID != overlay.ID || restored.Name != "Đã khôi phục" {
		t.Fatalf("restore: %+v %v %v", restored, reused, err)
	}
	if added, err := lib.SeedBuiltins(ctx); err != nil || added != 12 {
		t.Fatalf("seed: %d %v", added, err)
	}
	if added, err := lib.SeedBuiltins(ctx); err != nil || added != 0 {
		t.Fatalf("seed again: %d %v", added, err)
	}

	cs := postgres.NewChannelStore(f.pool)
	id := "50000000-0000-4000-8000-000000000001"
	template, err := lib.Create(ctx, "Mẫu kiểm thử công bố")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cs.CreateChannel(ctx, channels.CreateInput{ID: id, Name: "Channel", Language: domain.LangVI, PerDay: 5, DefaultTemplateID: template.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE owner_channels SET branding='{"brief":"brief","look":"look","avatar":"avatar","banner":"banner"}'::jsonb WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	ch := channels.New(channels.Options{Store: cs, Media: f.media, AI: producerAI{}, Tasks: channelTasks{svc: activity.New(postgres.NewActivityStore(f.pool), nil)}})
	if err := ch.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ch.Close(context.Background()) })
	for _, part := range []string{domain.IdentityAvatar, domain.IdentityBanner} {
		if _, err := ch.RunIdentity(ctx, id, channels.IdentityInput{Part: part}); err != nil {
			t.Fatalf("identity %s: %v", part, err)
		}
	}
	view, err := ch.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}

	videoData, err := os.ReadFile(filepath.Join("..", "..", "internal", "library", "assets", "overlays", "rain-loop.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	run := func(_ context.Context, _ domain.PlatformID, args []string, _ int, _ time.Duration, _ func(string)) (string, error) {
		for _, arg := range args {
			if arg == "-J" {
				return `{"title":"Fixture","duration":120}`, nil
			}
		}
		for i, arg := range args {
			if arg == "-o" && i+1 < len(args) {
				return "", os.WriteFile(strings.ReplaceAll(args[i+1], "%(ext)s", "mp4"), videoData, 0600)
			}
		}
		return "", errors.New("missing operation workspace")
	}
	bd := backdrop.New(backdrop.Deps{Store: postgres.NewBackdropStore(f.pool), Media: f.media, RunYtDlp: run})
	bv, err := bd.AddVideo(ctx, "https://www.twitch.tv/test/clip/Fixture", &domain.BackdropSegment{FromSec: 0, ToSec: 10})
	if err != nil {
		t.Fatal(err)
	}
	first, err := bd.AcquireVideo(ctx, bv.Video.ID)
	if err != nil {
		t.Fatal(err)
	}
	oldPath := first.Ref.Path
	first.Release()
	videoData = append(append([]byte(nil), videoData...), []byte("revision")...)
	if _, err := bd.UpdateVideoSegment(ctx, bv.Video.ID, domain.BackdropSegment{FromSec: 1, ToSec: 11}); err != nil {
		t.Fatal(err)
	}
	old, err := f.media.Acquire(ctx, oldPath)
	if err != nil {
		t.Fatal(err)
	}
	oldData, err := os.ReadFile(old.Path)
	old.Release()
	if err != nil || bytes.Equal(oldData, videoData) {
		t.Fatal("new download overwrote old snapshot")
	}

	// Populate, release, erase and recreate cache: HTTP and file consumers must
	// read verified remote bytes rather than retained operation scratch files.
	h, _, err := lib.AcquireOverlay(ctx, overlay.ID)
	if err != nil {
		t.Fatal(err)
	}
	h.Release()
	if err := os.RemoveAll(f.cacheDir); err != nil {
		t.Fatal(err)
	}
	f.reopen(t)
	lib = producerLibrary(t, f)
	g, cookie := producerGateway(t, f.pool)
	web.MountLibrary(g, lib, nil)
	web.MountChannels(g, ch)
	web.MountBackdrop(g, bd)
	assertProducerHTTP(t, g, cookie, "/api/overlays/"+overlay.ID+"/file", data)
	assertProducerHTTP(t, g, cookie, "/api/backdrop/videos/"+bv.Video.ID+"/file", videoData)
	for _, asset := range []*string{view.AvatarPath, view.BannerPath} {
		if asset == nil {
			t.Fatal("identity omitted asset")
		}
		h, err := f.media.Acquire(ctx, *asset)
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(h.Path)
		h.Release()
		if err != nil {
			t.Fatal(err)
		}
		assertProducerHTTP(t, g, cookie, "/api/owner-channels/"+id+"/branding/"+filepath.Base(*asset), want)
	}
	h, _, err = lib.AcquireOverlay(ctx, overlay.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(h.Path)
	h.Release()
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("cold local consumer differs: %v", err)
	}
}

func TestProducerCLIUsesSharedBackend(t *testing.T) {
	pool, dsn := provisionCLI(t)
	if os.Getenv("S3_TEST_ENDPOINT") == "" {
		if os.Getenv("REQUIRE_S3_TEST") != "" {
			t.Fatal("S3_TEST_ENDPOINT is required for CLI acceptance")
		}
		t.Skip("S3_TEST_ENDPOINT is not configured")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("VIDERSE_DATABASE_URL", dsn)
	cacheDir := t.TempDir()
	for _, pair := range [][2]string{{"STORAGE_DRIVER", "s3"}, {"S3_PROVIDER", "garage"}, {"S3_ENDPOINT", os.Getenv("S3_TEST_ENDPOINT")}, {"S3_REGION", os.Getenv("S3_TEST_REGION")}, {"S3_BUCKET", os.Getenv("S3_TEST_BUCKET")}, {"S3_ACCESS_KEY_ID", os.Getenv("S3_TEST_ACCESS_KEY")}, {"S3_SECRET_ACCESS_KEY", os.Getenv("S3_TEST_SECRET_KEY")}, {"S3_ALLOW_INSECURE_ENDPOINT", "true"}, {"BLOB_DELIVERY", "proxy"}, {"BLOB_CACHE_DIR", cacheDir}} {
		t.Setenv(pair[0], pair[1])
	}
	ctx := context.Background()
	svc, closeLib, err := openLibrary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeLib)
	if svc.Media() == nil {
		closeLib()
		t.Fatal("CLI library omitted shared media backend")
	}
	// Use the installed ffprobe directly rather than the CLI managed-tools
	// prerequisite; the production factory's backend is left unchanged.
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal(err)
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal(err)
	}
	writer := library.New(library.Deps{Store: postgres.NewLibraryStore(pool), Media: svc.Media(), Runner: producerRunner{}, Bins: producerBins{ffmpeg, ffprobe}})
	data := producerPNG(t)
	input := filepath.Join(t.TempDir(), "cli.png")
	if err := os.WriteFile(input, data, 0600); err != nil {
		t.Fatal(err)
	}
	row, _, err := writer.AddOverlayFile(ctx, input, "CLI")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SeedBuiltins(ctx); err != nil {
		t.Fatal(err)
	}
	closeLib()
	if err := os.RemoveAll(cacheDir); err != nil {
		t.Fatal(err)
	}
	svc, closeLib, err = openLibrary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer closeLib()
	h, _, err := svc.AcquireOverlay(ctx, row.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(h.Path)
	h.Release()
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("CLI restarted bytes differ: %v", err)
	}
	ch, closeCh, err := openChannelService(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	defer closeCh()
	if ch.Media() == nil {
		t.Fatal("CLI channel omitted shared media backend")
	}
	bd, closeBD, err := openBackdropCLI(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer closeBD()
	if bd.Media() == nil {
		t.Fatal("CLI backdrop omitted shared media backend")
	}
}
