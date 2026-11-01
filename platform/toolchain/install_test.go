// Test chuyển từ src/server/tools/tool-installer.test.ts của yt-factory.
package toolchain

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeToolAsset là asset nhỏ gửi qua httptest.
type fakeToolAsset struct {
	content  []byte
	checksum string
}

func fakePin(content []byte) (ToolPin, string) {
	sum := sha256.Sum256(content)
	return ToolPin{
		Version:    "1.0.0",
		ReleaseURL: "https://example.invalid/release/1.0.0",
		License:    ToolLicense{SPDX: "MIT"},
		Build:      "fixture",
		Assets: map[PlatformKey][]PinnedAsset{
			CurrentPlatformKey(): {},
		},
		RequiredBytes: 1,
	}, hex.EncodeToString(sum[:])
}

// toolFixture là một bản cài giả phục vụ qua httptest.
type toolFixture struct {
	base      string
	toolDir   string
	payload   []byte
	pin       ToolPin
	installer *Installer
}

func newToolFixture(t *testing.T, payload []byte) *toolFixture {
	t.Helper()
	base := t.TempDir()
	pin, checksum := fakePin(payload)
	pin.Assets[CurrentPlatformKey()] = []PinnedAsset{
		{Name: "tool", URL: "https://assets.example/tool", SHA256: checksum, Kind: AssetFile, Executable: true},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tool" {
			w.Header().Set("content-type", "application/octet-stream")
			_, _ = w.Write(payload)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)
	pin.Assets[CurrentPlatformKey()][0].URL = server.URL + "/tool"
	return &toolFixture{
		base:      base,
		toolDir:   ToolRoot(base, "fixture"),
		payload:   payload,
		pin:       pin,
		installer: &Installer{Client: server.Client(), BaseDir: base},
	}
}

func (f *toolFixture) spec() InstallSpec {
	return InstallSpec{
		ToolID: "fixture",
		Pin:    f.pin,
		SelfCheck: func(ctx context.Context, ic *InstallContext, staging string) (string, error) {
			if !fileExists(filepath.Join(staging, "tool")) {
				return "", fmt.Errorf("tool không có trong staging")
			}
			return "1.0.0", nil
		},
	}
}

func TestInstallerInstallsVerifiesAndActivatesAsset(t *testing.T) {
	f := newToolFixture(t, []byte("fake-binary-v1"))
	res, err := f.installer.Install(context.Background(), f.spec())
	if err != nil {
		t.Fatal(err)
	}
	if res.Version != "1.0.0" || res.State != "installed" || res.Previous != "" {
		t.Fatalf("res = %+v", res)
	}
	exe := filepath.Join(f.toolDir, "v-1.0.0", "tool")
	data, err := os.ReadFile(exe)
	if err != nil || string(data) != "fake-binary-v1" {
		t.Fatalf("asset = %q %v", data, err)
	}
	info, _ := os.Stat(exe)
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatal("asset không có bit thực thi")
	}
	marker, _ := os.ReadFile(filepath.Join(f.toolDir, "current"))
	if strings.TrimSpace(string(marker)) != "v-1.0.0" {
		t.Fatalf("current = %q", marker)
	}
	installed, _ := os.ReadFile(filepath.Join(f.toolDir, "v-1.0.0", "installed.json"))
	if !strings.Contains(string(installed), `"license": "MIT"`) {
		t.Fatalf("installed.json = %s", installed)
	}
	if ActiveVersionDir(f.base, "fixture") != filepath.Join(f.toolDir, "v-1.0.0") {
		t.Fatal("ActiveVersionDir sai")
	}
}

func TestInstallerSecondCallReportsAlreadyCurrent(t *testing.T) {
	f := newToolFixture(t, []byte("fake-binary-v1"))
	if _, err := f.installer.Install(context.Background(), f.spec()); err != nil {
		t.Fatal(err)
	}
	res, err := f.installer.Install(context.Background(), f.spec())
	if err != nil {
		t.Fatal(err)
	}
	if res.State != "already-current" {
		t.Fatalf("res = %+v", res)
	}
}

func TestInstallerConcurrentCallersShareOneInstall(t *testing.T) {
	f := newToolFixture(t, []byte("fake-binary-v1"))
	spec := f.spec()
	var checks atomic.Int32
	spec.SelfCheck = func(ctx context.Context, ic *InstallContext, staging string) (string, error) {
		checks.Add(1)
		return "1.0.0", nil
	}
	resCh := make(chan InstallResult, 2)
	errCh := make(chan error, 2)
	for range 2 {
		go func() {
			res, err := f.installer.Install(context.Background(), spec)
			resCh <- res
			errCh <- err
		}()
	}
	for range 2 {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
		res := <-resCh
		if res.Version != "1.0.0" {
			t.Fatalf("res = %+v", res)
		}
	}
	if checks.Load() != 1 {
		t.Fatalf("self-check chạy %d lần", checks.Load())
	}
}

func TestInstallerRejectsChecksumMismatch(t *testing.T) {
	f := newToolFixture(t, []byte("fake-binary-v1"))
	spec := f.spec()
	spec.Pin = f.pin
	spec.Pin.Assets = map[PlatformKey][]PinnedAsset{
		CurrentPlatformKey(): {
			{Name: "tool", URL: f.pin.Assets[CurrentPlatformKey()][0].URL,
				SHA256: strings.Repeat("f", 64), Kind: AssetFile, Executable: true},
		},
	}
	_, err := f.installer.Install(context.Background(), spec)
	var ce *ChecksumError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v, muốn ChecksumError", err)
	}
	// Hash lệch → tệp lỗi bị xoá, không có gì được active.
	entries, _ := os.ReadDir(f.toolDir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "staging-") || strings.HasPrefix(e.Name(), ".dl-") {
			t.Fatalf("đồ dư còn sót: %s", e.Name())
		}
	}
	if st := f.installer.ReadInstallState("fixture"); st == nil || st.Phase != PhaseFailed {
		t.Fatalf("state = %+v", st)
	}
}

func TestInstallerSharedCallSeesRejectionWithoutCorruptingDisk(t *testing.T) {
	f := newToolFixture(t, []byte("fake-binary-v1"))
	spec := f.spec()
	spec.Pin.Assets = map[PlatformKey][]PinnedAsset{
		CurrentPlatformKey(): {
			{Name: "tool", URL: f.pin.Assets[CurrentPlatformKey()][0].URL,
				SHA256: strings.Repeat("0", 64), Kind: AssetFile, Executable: true},
		},
	}
	errCh := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := f.installer.Install(context.Background(), spec)
			errCh <- err
		}()
	}
	for range 2 {
		if err := <-errCh; err == nil {
			t.Fatal("lời gọi chia sẻ không thấy lỗi")
		}
	}
	if !os.IsNotExist(statErr(filepath.Join(f.toolDir, "current"))) {
		t.Fatal("phiên bản hỏng vẫn được active")
	}
}

func TestInstallerCancelSentinelAbortsMidInstall(t *testing.T) {
	f := newToolFixture(t, []byte("fake-binary-v1"))
	spec := f.spec()
	cancelled := make(chan struct{})
	spec.SelfCheck = func(ctx context.Context, ic *InstallContext, staging string) (string, error) {
		f.installer.RequestCancel("fixture")
		close(cancelled)
		if err := ic.CheckCancelled(); err != nil {
			return "", err
		}
		return "1.0.0", nil
	}
	_, err := f.installer.Install(context.Background(), spec)
	<-cancelled
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("err = %v, muốn ErrCancelled", err)
	}
	entries, _ := os.ReadDir(f.toolDir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "staging-") {
			t.Fatalf("staging còn sót: %s", e.Name())
		}
	}
}

func TestInstallerPreStartSentinelAbortsInstall(t *testing.T) {
	// Cancel-request đang đợi khi lượt mới bắt đầu hủy ngay lượt đó (bản cũ:
	// sentinel không bao giờ được bỏ qua) và tệp sentinel được gỡ.
	f := newToolFixture(t, []byte("fake-binary-v1"))
	if err := os.MkdirAll(f.toolDir, 0o755); err != nil {
		t.Fatal(err)
	}
	f.installer.RequestCancel("fixture")
	_, err := f.installer.Install(context.Background(), f.spec())
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("err = %v, muốn ErrCancelled", err)
	}
	if f.installer.CancelRequested("fixture") {
		t.Fatal("sentinel còn sót sau khi được nhận")
	}
	st := f.installer.ReadInstallState("fixture")
	if st == nil || st.Phase != PhaseCancelled {
		t.Fatalf("state = %+v, muốn cancelled", st)
	}
	// Lượt sau không còn sentinel nào phải cài bình thường.
	if _, err := f.installer.Install(context.Background(), f.spec()); err != nil {
		t.Fatal(err)
	}
}

func TestInstallerCanceledRunLeavesCurrentVersionUntouched(t *testing.T) {
	f := newToolFixture(t, []byte("fake-binary-v1"))
	if _, err := f.installer.Install(context.Background(), f.spec()); err != nil {
		t.Fatal(err)
	}
	currentBefore, _ := os.ReadFile(filepath.Join(f.toolDir, "current"))

	// Lượt sau nhắm phiên bản khác nên không bị short-circuit "đã hiện hành";
	// hash lệch làm nó hỏng trước khi active.
	spec := f.spec()
	spec.Pin.Version = "2.0.0"
	spec.Pin.Assets = map[PlatformKey][]PinnedAsset{
		CurrentPlatformKey(): {
			{Name: "tool", URL: f.pin.Assets[CurrentPlatformKey()][0].URL,
				SHA256: strings.Repeat("0", 64), Kind: AssetFile, Executable: true},
		},
	}
	if _, err := f.installer.Install(context.Background(), spec); err == nil {
		t.Fatal("cài lỗi vẫn thành công")
	}
	currentAfter, _ := os.ReadFile(filepath.Join(f.toolDir, "current"))
	if string(currentAfter) != string(currentBefore) {
		t.Fatal("lượt lỗi lật current")
	}
	if !fileExists(filepath.Join(f.toolDir, "v-1.0.0", "tool")) {
		t.Fatal("bản đang dùng bị đụng")
	}
}

func TestInstallStateRoundTrip(t *testing.T) {
	f := newToolFixture(t, []byte("x"))
	if f.installer.ReadInstallState("fixture") != nil {
		t.Fatal("state vắng lúc đầu")
	}
	if _, err := f.installer.Install(context.Background(), f.spec()); err != nil {
		t.Fatal(err)
	}
	st := f.installer.ReadInstallState("fixture")
	if st == nil || st.Phase != PhaseDone || st.Version != "1.0.0" {
		t.Fatalf("state = %+v", st)
	}
}

func TestDownloadVerifiedStreamsAndHashes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("some-bytes"))
	}))
	t.Cleanup(server.Close)
	sum := sha256.Sum256([]byte("some-bytes"))
	inst := &Installer{Client: server.Client(), BaseDir: t.TempDir()}
	out := filepath.Join(t.TempDir(), "dl.bin")
	var ticks int
	err := inst.DownloadVerified(context.Background(), PinnedAsset{
		Name: "x", URL: server.URL + "/x", SHA256: hex.EncodeToString(sum[:]),
	}, out, func(done int64) error {
		ticks++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if ticks == 0 {
		t.Fatal("tick không chạy")
	}
	data, _ := os.ReadFile(out)
	if string(data) != "some-bytes" {
		t.Fatalf("payload = %q", data)
	}
}

func TestUnzipRestoresSymlinks(t *testing.T) {
	if isWindows() {
		t.Skip("POSIX")
	}
	dir := t.TempDir()
	zipFile := filepath.Join(dir, "test.zip")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("bundle/real")
	_, _ = w.Write([]byte("real-content"))
	hdr := &zip.FileHeader{Name: "bundle/link", Method: zip.Store}
	hdr.SetMode(0o777 | os.ModeSymlink)
	lw, _ := zw.CreateHeader(hdr)
	_, _ = lw.Write([]byte("real"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zipFile, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "out")
	if err := extractArchive(AssetZip, zipFile, dest, ""); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(filepath.Join(dest, "bundle", "link"))
	if err != nil || target != "real" {
		t.Fatalf("symlink = %q %v", target, err)
	}
	data, _ := os.ReadFile(filepath.Join(dest, "bundle", "real"))
	if string(data) != "real-content" {
		t.Fatalf("real = %q", data)
	}
}

func TestUntarGzExtractsMemberOnly(t *testing.T) {
	if isWindows() {
		t.Skip("POSIX")
	}
	dir := t.TempDir()
	tgz := filepath.Join(dir, "test.tar.gz")
	f, err := os.Create(tgz)
	if err != nil {
		t.Fatal(err)
	}
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)
	for _, name := range []string{"pkg/keep", "pkg/drop"} {
		body := "content-of-" + name
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		_, _ = tw.Write([]byte(body))
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "out")
	if err := extractArchive(AssetTarGZ, tgz, dest, "pkg/keep"); err != nil {
		t.Fatal(err)
	}
	if !fileExists(filepath.Join(dest, "pkg", "keep")) {
		t.Fatal("member không được trích")
	}
	if fileExists(filepath.Join(dest, "pkg", "drop")) {
		t.Fatal("member khác cũng được trích")
	}
}

func statErr(path string) error {
	_, err := os.Stat(path)
	return err
}

// Archive extraction phải đi qua os.Root: member ".." và symlink thoát khỏi
// dest đều bị từ chối — kể cả symlink trước rồi file ghi qua nó sau.
func TestExtractArchiveRejectsTraversalAndEscapeSymlink(t *testing.T) {
	if isWindows() {
		t.Skip("POSIX")
	}
	dir := t.TempDir()
	outside := filepath.Join(dir, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	// tar: member traversal.
	tgz := filepath.Join(dir, "evil.tar.gz")
	f, err := os.Create(tgz)
	if err != nil {
		t.Fatal(err)
	}
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)
	body := "x"
	_ = tw.WriteHeader(&tar.Header{Name: "../escape.txt", Mode: 0o644, Size: int64(len(body))})
	_, _ = tw.Write([]byte(body))
	_ = tw.Close()
	_ = gzw.Close()
	_ = f.Close()
	dest := filepath.Join(dir, "dest")
	if err := extractArchive(AssetTarGZ, tgz, dest, ""); err == nil {
		t.Fatal("member traversal không bị từ chối")
	}
	if fileExists(filepath.Join(dir, "escape.txt")) {
		t.Fatal("ghi thoát khỏi dest")
	}

	// tar: symlink ../escape → file ghi qua symlink đó.
	tgz2 := filepath.Join(dir, "evil2.tar.gz")
	f, _ = os.Create(tgz2)
	gzw = gzip.NewWriter(f)
	tw = tar.NewWriter(gzw)
	_ = tw.WriteHeader(&tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: filepath.Join("..", "outside")})
	_ = tw.WriteHeader(&tar.Header{Name: "link/pwned.txt", Mode: 0o644, Size: int64(len(body))})
	_, _ = tw.Write([]byte(body))
	_ = tw.Close()
	_ = gzw.Close()
	_ = f.Close()
	if err := extractArchive(AssetTarGZ, tgz2, dest, ""); err == nil {
		t.Fatal("symlink escape không bị từ chối")
	}
	if fileExists(filepath.Join(outside, "pwned.txt")) {
		t.Fatal("ghi qua symlink thoát khỏi dest")
	}
}

// Symlink tuyệt đối trong zip (như cấu trúc .app) chỉ được giữ khi trỏ vào
// trong dest; trỏ ra ngoài bị từ chối.
func TestExtractArchiveRejectsZipSymlinkOutside(t *testing.T) {
	if isWindows() {
		t.Skip("POSIX")
	}
	dir := t.TempDir()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	hdr := &zip.FileHeader{Name: "link", Method: zip.Store}
	hdr.SetMode(0o777 | os.ModeSymlink)
	lw, _ := zw.CreateHeader(hdr)
	_, _ = lw.Write([]byte("/etc"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zipFile := filepath.Join(dir, "links.zip")
	if err := os.WriteFile(zipFile, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := extractArchive(AssetZip, zipFile, filepath.Join(dir, "dest"), ""); err == nil {
		t.Fatal("symlink tuyệt đối không bị từ chối")
	}
}

// Cancel-request ghi giữa chừng phải cắt đang tải, không đợi body đọc xong.
func TestInstallerCancelFileInterruptsActiveDownload(t *testing.T) {
	base := t.TempDir()
	payload := bytes.Repeat([]byte("0123456789"), 4096)
	sum := sha256.Sum256(payload)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/octet-stream")
		_, _ = w.Write(payload[:512])
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-release // giữ body mở: tải chỉ thoát khi ctx huỷ
	}))
	t.Cleanup(func() { close(release); server.Close() })
	pin, _ := fakePin(payload)
	pin.Assets = map[PlatformKey][]PinnedAsset{
		CurrentPlatformKey(): {
			{Name: "tool", URL: server.URL + "/tool", SHA256: hex.EncodeToString(sum[:]), Kind: AssetFile, Executable: true},
		},
	}
	inst := &Installer{Client: server.Client(), BaseDir: base}
	spec := InstallSpec{
		ToolID: "fixture",
		Pin:    pin,
		SelfCheck: func(ctx context.Context, ic *InstallContext, staging string) (string, error) {
			return "1.0.0", nil
		},
	}
	done := make(chan error, 1)
	go func() {
		_, err := inst.Install(context.Background(), spec)
		done <- err
	}()
	// Đợi tới khi state bước vào downloading rồi ghi cancel-request.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if st := inst.ReadInstallState("fixture"); st != nil && st.Phase == PhaseDownloading {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	inst.RequestCancel("fixture")
	select {
	case err := <-done:
		if !errors.Is(err, ErrCancelled) {
			t.Fatalf("err = %v, muốn ErrCancelled", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("cancel không cắt được tải đang chạy")
	}
	st := inst.ReadInstallState("fixture")
	if st == nil || st.Phase != PhaseCancelled {
		t.Fatalf("state = %+v, muốn cancelled", st)
	}
}

// DownloadTimeout chặn máy chủ giữ kết nối mở mãi: lượt cài lỗi thay vì treo.
func TestInstallerDownloadTimeoutAbortsStalledServer(t *testing.T) {
	base := t.TempDir()
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/octet-stream")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-release
	}))
	t.Cleanup(func() { close(release); server.Close() })
	pin, _ := fakePin([]byte("payload"))
	pin.Assets = map[PlatformKey][]PinnedAsset{
		CurrentPlatformKey(): {
			{Name: "tool", URL: server.URL + "/tool", SHA256: strings.Repeat("0", 64), Kind: AssetFile},
		},
	}
	inst := &Installer{Client: server.Client(), BaseDir: base, DownloadTimeout: 300 * time.Millisecond}
	spec := InstallSpec{
		ToolID:    "fixture",
		Pin:       pin,
		SelfCheck: func(ctx context.Context, ic *InstallContext, staging string) (string, error) { return "1.0.0", nil },
	}
	start := time.Now()
	_, err := inst.Install(context.Background(), spec)
	if err == nil {
		t.Fatal("tải treo vẫn thành công")
	}
	if time.Since(start) > 10*time.Second {
		t.Fatalf("timeout không áp được: %s", time.Since(start))
	}
	st := inst.ReadInstallState("fixture")
	if st == nil || st.Phase != PhaseFailed {
		t.Fatalf("state = %+v, muốn failed", st)
	}
}

// Caller ctx huỷ giữa tải: caller đó rút với ErrCancelled nhưng lượt chia sẻ
// vẫn chạy tiếp; waiter thứ hai vẫn nhận kết quả cài.
func TestInstallerCallerCancelLeavesSharedRunAlive(t *testing.T) {
	base := t.TempDir()
	payload := []byte("fake-binary-v1")
	sum := sha256.Sum256(payload)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(server.Close)
	pin, _ := fakePin(payload)
	pin.Assets = map[PlatformKey][]PinnedAsset{
		CurrentPlatformKey(): {
			{Name: "tool", URL: server.URL + "/tool", SHA256: hex.EncodeToString(sum[:]), Kind: AssetFile, Executable: true},
		},
	}
	inst := &Installer{Client: server.Client(), BaseDir: base}
	slowCheck := make(chan struct{})
	spec := InstallSpec{
		ToolID: "fixture",
		Pin:    pin,
		SelfCheck: func(ctx context.Context, ic *InstallContext, staging string) (string, error) {
			<-slowCheck // đủ lâu để caller A rút
			return "1.0.0", nil
		},
	}
	ctxA, cancelA := context.WithCancel(context.Background())
	errA := make(chan error, 1)
	go func() {
		_, err := inst.Install(ctxA, spec)
		errA <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for !inst.InstallInProgress("fixture") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	resB := make(chan error, 1)
	go func() {
		_, err := inst.Install(context.Background(), spec)
		resB <- err
	}()
	// Cho B gộp vào lượt rồi A rút: lượt vẫn sống cho B.
	time.Sleep(100 * time.Millisecond)
	cancelA()
	if err := <-errA; !errors.Is(err, ErrCancelled) {
		t.Fatalf("caller A err = %v, muốn ErrCancelled", err)
	}
	close(slowCheck)
	if err := <-resB; err != nil {
		t.Fatalf("waiter B err = %v", err)
	}
	if st := inst.ReadInstallState("fixture"); st == nil || st.Phase != PhaseDone {
		t.Fatalf("state = %+v, muốn done", st)
	}
}

// bytesDone của mỗi asset cộng đúng một lần: report cuối phải bằng tổng
// kích cỡ thật, không bị bội số theo số chunk.
func TestInstallerBytesDoneMatchesDownloadedTotal(t *testing.T) {
	base := t.TempDir()
	payload1 := bytes.Repeat([]byte("a"), 400*1024)
	payload2 := bytes.Repeat([]byte("b"), 100*1024)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "2") {
			_, _ = w.Write(payload2)
			return
		}
		_, _ = w.Write(payload1)
	}))
	t.Cleanup(server.Close)
	sum1 := sha256.Sum256(payload1)
	sum2 := sha256.Sum256(payload2)
	pin, _ := fakePin(payload1)
	pin.Assets = map[PlatformKey][]PinnedAsset{
		CurrentPlatformKey(): {
			{Name: "a1", URL: server.URL + "/a1", SHA256: hex.EncodeToString(sum1[:]), Kind: AssetFile},
			{Name: "a2", URL: server.URL + "/a2", SHA256: hex.EncodeToString(sum2[:]), Kind: AssetFile},
		},
	}
	inst := &Installer{Client: server.Client(), BaseDir: base}
	spec := InstallSpec{
		ToolID:    "fixture",
		Pin:       pin,
		SelfCheck: func(ctx context.Context, ic *InstallContext, staging string) (string, error) { return "1.0.0", nil },
	}
	if _, err := inst.Install(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	st := inst.ReadInstallState("fixture")
	want := int64(len(payload1) + len(payload2))
	if st == nil || st.BytesDone != want {
		t.Fatalf("bytesDone = %+v, muốn %d", st, want)
	}
}

// Tải dở giữ .part và lượt sau nối tiếp bằng Range (sống qua lần chạy lại).
func TestDownloadVerifiedResumesFromPartFile(t *testing.T) {
	payload := bytes.Repeat([]byte("abcd"), 50000)
	sum := sha256.Sum256(payload)
	var mu sync.Mutex
	var ranges []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rng := r.Header.Get("Range")
		mu.Lock()
		ranges = append(ranges, rng)
		mu.Unlock()
		if rng == "" {
			_, _ = w.Write(payload)
			return
		}
		var start int
		if _, err := fmt.Sscanf(rng, "bytes=%d-", &start); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(payload)-1, len(payload)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(payload[start:])
	}))
	t.Cleanup(server.Close)
	inst := &Installer{Client: server.Client(), BaseDir: t.TempDir()}
	out := filepath.Join(t.TempDir(), "asset.bin")
	asset := PinnedAsset{Name: "asset.bin", URL: server.URL + "/a", SHA256: hex.EncodeToString(sum[:])}

	ctx, cancel := context.WithCancel(context.Background())
	err := inst.DownloadVerified(ctx, asset, out, func(n int64) error {
		cancel()
		return ctx.Err()
	})
	if err == nil {
		t.Fatal("lượt đầu phải bị huỷ")
	}
	part := out + ".part"
	st, statErr := os.Stat(part)
	if statErr != nil || st.Size() == 0 {
		t.Fatalf(".part không được giữ: %v %v", statErr, st)
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Fatal("huỷ không được để tệp cuối thành công")
	}
	if err := inst.DownloadVerified(context.Background(), asset, out, nil); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	last := ranges[len(ranges)-1]
	mu.Unlock()
	if !strings.HasPrefix(last, "bytes=") {
		t.Fatalf("lượt hai không gửi Range: %q (ranges %v)", last, ranges)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, payload) {
		t.Fatalf("nội dung sau khi nối sai: %d byte", len(data))
	}
	if _, err := os.Stat(part); !os.IsNotExist(err) {
		t.Fatal(".part phải được đổi tên sau khi xong")
	}
}

// isWindows cho phép bỏ qua các test cần POSIX (symlink, quyền tệp).
func isWindows() bool { return os.PathSeparator == '\\' }
