package vieneu

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
	"github.com/datnt1998/verse/apps/noverse/internal/voice/openaispeech"
)

func testEngine(t *testing.T) *Engine {
	t.Helper()
	eng, err := newEngine(voice.Config{Home: t.TempDir(), Model: "vieneu-v3-turbo", Device: "cpu"}.WithDefaults())
	if err != nil {
		t.Fatalf("newEngine: %v", err)
	}
	return eng.(*Engine)
}

func TestInstalledRequiresSourceAndVenv(t *testing.T) {
	e := testEngine(t)
	if e.Installed() {
		t.Fatal("thư mục trống phải là chưa cài")
	}
	src := filepath.Join(e.srcDir(), "apps")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "openai_speech.py"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if e.Installed() {
		t.Fatal("có mã nguồn nhưng chưa có môi trường Python thì vẫn là chưa cài")
	}
	py := e.venvPython()
	if err := os.MkdirAll(filepath.Dir(py), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(py, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !e.Installed() {
		t.Fatal("đủ mã nguồn + python phải là đã cài")
	}
	if got := e.Status(nil); got.State != voice.StateStopped {
		t.Fatalf("State = %q, muốn stopped", got.State)
	}
}

func TestPrecisionFollowsQualityAndCPU(t *testing.T) {
	e := testEngine(t)
	prec, note := e.precisionFor(voice.QualityStandard)
	if prec != "fp32" || note != "" {
		t.Fatalf("standard → %q/%q, muốn fp32/không ghi chú", prec, note)
	}
	prec, note = e.precisionFor(voice.QualityHigh)
	if prec != "fp32" {
		t.Fatalf("high → %q, muốn fp32", prec)
	}
	prec, note = e.precisionFor(voice.QualityFast)
	if supportsVNNI() {
		if prec != "int8" {
			t.Fatalf("fast trên CPU có VNNI → %q, muốn int8", prec)
		}
		return
	}
	if prec != "fp32" || note == "" {
		t.Fatalf("fast trên CPU không VNNI → %q/%q, muốn fp32 + ghi chú", prec, note)
	}

	gpu := testEngine(t)
	gpu.cfg.Device = "cuda"
	if prec, _ := gpu.precisionFor(voice.QualityFast); prec != "fp32" {
		t.Fatalf("GPU → %q, muốn fp32 (precision chỉ áp cho ONNX/CPU)", prec)
	}
}

func TestSupportsVNNIIsArchGated(t *testing.T) {
	if runtime.GOARCH != "amd64" && supportsVNNI() {
		t.Fatalf("%s không thể có VNNI", runtime.GOARCH)
	}
}

func TestCloneIndexRoundTrip(t *testing.T) {
	e := testEngine(t)
	first := cloneEntry{Name: "Giọng A", File: filepath.Join(e.voicesDir(), "giong-a.wav"), DurationSec: 6.5, CreatedAt: time.Now()}
	if err := e.saveClone(first); err != nil {
		t.Fatalf("saveClone: %v", err)
	}
	second := cloneEntry{Name: "Giọng B", File: filepath.Join(e.voicesDir(), "giong-b.wav"), DurationSec: 5}
	if err := e.saveClone(second); err != nil {
		t.Fatalf("saveClone 2: %v", err)
	}
	// Cùng tên thì ghi đè, không nhân đôi.
	first.DurationSec = 7
	if err := e.saveClone(first); err != nil {
		t.Fatalf("saveClone 3: %v", err)
	}
	got := e.loadClones()
	if len(got) != 2 {
		t.Fatalf("index có %d giọng, muốn 2: %+v", len(got), got)
	}
	if got[0].Name != "Giọng A" || got[0].DurationSec != 7 {
		t.Fatalf("mục đầu = %+v, muốn Giọng A 7 giây", got[0])
	}
	if _, err := os.Stat(e.indexPath()); err != nil {
		t.Fatalf("index chưa được ghi: %v", err)
	}
}

// Giọng clone do tiến trình khác thêm (chỉ có trong index) phải được nạp bù vào máy
// đọc đang chạy, và chỉ những giọng máy chủ chưa có.
func TestSyncClonesEnrollsOnlyMissing(t *testing.T) {
	e := testEngine(t)
	if err := os.MkdirAll(e.voicesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"Giọng A", "Giọng B"} {
		f := filepath.Join(e.voicesDir(), slug(n)+".wav")
		if err := os.WriteFile(f, []byte("RIFF"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := e.saveClone(cloneEntry{Name: n, File: f}); err != nil {
			t.Fatal(err)
		}
	}
	var mu sync.Mutex
	var posted []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/voices" {
			mu.Lock()
			posted = append(posted, r.FormValue("name"))
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	c := openaispeech.NewClient(openaispeech.ClientOptions{Endpoint: srv.URL})

	if n := e.syncClones(context.Background(), c, map[string]bool{"Giọng A": true}); n != 1 {
		t.Fatalf("syncClones nạp %d giọng, muốn 1", n)
	}
	if len(posted) != 1 || posted[0] != "Giọng B" {
		t.Fatalf("đã nạp %v, muốn [Giọng B]", posted)
	}
	// Lần sau không nạp lại giọng đã nạp trong tiến trình này.
	if n := e.syncClones(context.Background(), c, map[string]bool{}); n != 1 || len(posted) != 2 || posted[1] != "Giọng A" {
		t.Fatalf("lần 2 nạp %d, posted=%v; muốn chỉ thêm Giọng A", n, posted)
	}
	if n := e.syncClones(context.Background(), c, map[string]bool{}); n != 0 {
		t.Fatalf("lần 3 nạp %d giọng, muốn 0 (đã nạp hết)", n)
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Trúc Ly":        "trúc-ly",
		"  Adam bựa  ":   "adam-bựa",
		"giong_tri_ha":   "giong-tri-ha",
		"...":            "giong",
		"Ngọc Huyền (2)": "ngọc-huyền-2",
	}
	for in, want := range cases {
		if got := slug(in); got != want {
			t.Fatalf("slug(%q) = %q, muốn %q", in, got, want)
		}
	}
}

func TestServerEnvPinsLoopbackAndPrivateCaches(t *testing.T) {
	e := testEngine(t)
	env := strings.Join(e.serverEnv(12345, "token-abc", "int8"), "\n")
	for _, want := range []string{
		"HOST=127.0.0.1",
		"PORT=12345",
		"VIENEU_API_KEY=token-abc",
		"VIENEU_BACKEND=onnx",
		"VIENEU_DEVICE=cpu",
		"VIENEU_PRECISION=int8",
		"HF_HOME=" + filepath.Join(e.dir, "hf"),
	} {
		if !strings.Contains(env, want) {
			t.Fatalf("env thiếu %q:\n%s", want, env)
		}
	}
	e.cfg.Device = "cuda"
	env = strings.Join(e.serverEnv(1, "t", "fp32"), "\n")
	if !strings.Contains(env, "VIENEU_DEVICE=cuda") || !strings.Contains(env, "VIENEU_BACKEND=auto") {
		t.Fatalf("GPU phải chạy backend auto/cuda:\n%s", env)
	}
}

func TestLogTailReturnsLastLines(t *testing.T) {
	e := testEngine(t)
	if err := os.MkdirAll(filepath.Dir(e.logPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.Repeat("dòng cũ\n", 500) + "lỗi cuối cùng\n"
	if err := os.WriteFile(e.logPath(), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	tail := e.logTail(2)
	if !strings.Contains(tail, "lỗi cuối cùng") {
		t.Fatalf("logTail = %q, muốn chứa dòng cuối", tail)
	}
	if len(tail) > 600 {
		t.Fatalf("logTail dài %d ký tự, muốn ≤ 600", len(tail))
	}
}

func TestFindUVFindsBinaryOnPath(t *testing.T) {
	dir := t.TempDir()
	uv := filepath.Join(dir, "uv")
	if err := os.WriteFile(uv, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	got, err := findUV()
	if err != nil {
		t.Fatalf("findUV: %v", err)
	}
	if got != uv {
		t.Fatalf("findUV = %q, muốn %q", got, uv)
	}
}

func TestFindUVReportsMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	// Các đường dẫn /opt/homebrew, /usr/local có thể có uv thật trên máy dev → bỏ qua
	// trường hợp đó thay vì báo sai.
	if got, err := findUV(); err == nil && !strings.HasSuffix(got, "uv") {
		t.Fatalf("findUV trả %q", got)
	}
}
