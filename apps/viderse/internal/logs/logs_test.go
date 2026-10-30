// Mẫu che bí mật lấy theo REDACTED_PATTERNS của
// src/server/clone/artifacts.ts trong yt-factory; test viết lại bằng Go.
package logs

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRedactRemovesSecretsAndHomeUser(t *testing.T) {
	s := New(t.TempDir(), "/Users/dat")
	cases := []struct {
		name string
		in   string
		bad  []string
		keep []string
	}{
		{
			name: "bearer token",
			in:   `GET /x Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abcDEF123456`,
			bad:  []string{"eyJhbGciOiJIUzI1NiJ9", "abcDEF123456"},
			keep: []string{"Authorization"},
		},
		{
			name: "cookie header",
			in:   `Cookie: SID=abc123; HSID=zzz`,
			bad:  []string{"abc123"},
			keep: []string{"Cookie"},
		},
		{
			name: "api key assignment",
			in:   `openai api_key=sk-live-9f8e7d6c5b4a`,
			bad:  []string{"sk-live-9f8e7d6c5b4a"},
			keep: []string{"api_key="},
		},
		{
			name: "json token field",
			in:   `{"access_token":"ak_12345","refresh_token":"rt_67890","name":"x"}`,
			bad:  []string{"ak_12345", "rt_67890"},
			keep: []string{`"name":"x"`},
		},
		{
			name: "jwt bare",
			in:   `device login eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0.Zm9vYmFyYmF6`,
			bad:  []string{"eyJhbGciOiJIUzI1NiJ9"},
			keep: []string{"device login"},
		},
		{
			name: "query params",
			in:   `https://example.test/cb?code=xyz&token=tok_999&secret=sss&sig=abc`,
			bad:  []string{"tok_999", "sss"},
			keep: []string{"https://example.test/cb"},
		},
		{
			name: "home path",
			in:   `error at /Users/dat/Library/Application Support/Viderse/logs/ffmpeg.log`,
			bad:  []string{"/Users/dat"},
			keep: []string{"~/Library/Application Support/Viderse"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := s.Redact(tc.in)
			for _, bad := range tc.bad {
				if strings.Contains(out, bad) {
					t.Fatalf("còn lộ %q trong %q", bad, out)
				}
			}
			for _, keep := range tc.keep {
				if !strings.Contains(out, keep) {
					t.Fatalf("mất %q trong %q", keep, out)
				}
			}
		})
	}
}

func TestAppendWritesJSONLAndRotates(t *testing.T) {
	dir := t.TempDir()
	s := New(dir, "")
	defer s.Close()
	// Mỗi dòng ~700 byte; 500 dòng vượt trần 256 KB.
	payload := strings.Repeat("x", 650)
	for range 500 {
		s.Append("ffmpeg", LevelInfo, payload)
	}
	file := filepath.Join(dir, "ffmpeg.log")
	st, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() > logLimitBytes {
		t.Fatalf("log không bị cắt: %d byte", st.Size())
	}
	s.Append("ffmpeg", LevelError, "dòng cuối cùng")
	entries, _, err := s.Read("ffmpeg", LevelInfo, 10, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("không đọc được dòng nào")
	}
	if got := entries[len(entries)-1].Message; got != "dòng cuối cùng" {
		t.Fatalf("dòng mới nhất = %q", got)
	}
}

func TestReadFiltersByLevelAndLimit(t *testing.T) {
	s := New(t.TempDir(), "")
	defer s.Close()
	s.Append("chrome", LevelInfo, "info one")
	s.Append("chrome", LevelWarn, "warn one")
	s.Append("chrome", LevelError, "error one")

	all, _, err := s.Read("chrome", LevelInfo, 10, time.Time{})
	if err != nil || len(all) != 3 {
		t.Fatalf("info: %d %v", len(all), err)
	}
	warn, _, _ := s.Read("chrome", LevelWarn, 10, time.Time{})
	if len(warn) != 2 {
		t.Fatalf("warn: %d", len(warn))
	}
	errs, _, _ := s.Read("chrome", LevelError, 10, time.Time{})
	if len(errs) != 1 || errs[0].Message != "error one" {
		t.Fatalf("error: %+v", errs)
	}
	limited, _, _ := s.Read("chrome", LevelInfo, 1, time.Time{})
	if len(limited) != 1 || limited[0].Message != "error one" {
		t.Fatalf("limit: %+v", limited)
	}
	if unknown, _, _ := s.Read("nope", LevelInfo, 10, time.Time{}); len(unknown) != 0 {
		t.Fatalf("công cụ lạ trả %d", len(unknown))
	}
}

func TestWriterRedactsAndFlushesPartialLine(t *testing.T) {
	s := New(t.TempDir(), "/Users/dat")
	defer s.Close()
	w := s.Writer("yt-dlp", LevelWarn)
	if _, err := w.Write([]byte("first api_key=sk-secret-1\nsecond /Users/dat/x")); err != nil {
		t.Fatal(err)
	}
	w.Flush()
	entries, _, err := s.Read("yt-dlp", LevelInfo, 10, time.Time{})
	if err != nil || len(entries) != 2 {
		t.Fatalf("entries = %+v, err = %v", entries, err)
	}
	if strings.Contains(entries[0].Message, "sk-secret-1") {
		t.Fatalf("chưa che khoá: %q", entries[0].Message)
	}
	if strings.Contains(entries[1].Message, "/Users/dat") {
		t.Fatalf("chưa che đường nhà: %q", entries[1].Message)
	}
}

func TestBuildBundleHasSummaryLogsAndRedactedExtras(t *testing.T) {
	s := New(t.TempDir(), "/Users/dat")
	defer s.Close()
	s.Append("ffmpeg", LevelError, `Authorization: Bearer tok_abcdef1234567890 failed at /Users/dat/x`)
	s.Append("yt-dlp", LevelInfo, "ok")

	bundle, err := s.BuildBundle(context.Background(), BundleOptions{
		Version: "1.2.3",
		Extras: []ExtraFile{
			func(context.Context) (string, []byte, error) {
				return "settings.json", []byte(`{"openai":{"api_key":"sk-live-xyz"}}`), nil
			},
		},
		Now: time.Date(2026, 10, 30, 8, 30, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(bundle.FileName, ".zip") {
		t.Fatalf("tên gói = %q", bundle.FileName)
	}
	zr, err := zip.NewReader(bytes.NewReader(bundle.Bytes), int64(len(bundle.Bytes)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		files[f.Name] = string(data)
	}
	for _, want := range []string{"summary.json", "logs/ffmpeg.log", "logs/yt-dlp.log", "settings.json"} {
		if _, ok := files[want]; !ok {
			t.Fatalf("thiếu %s; có %v", want, keys(files))
		}
	}
	if !strings.Contains(files["summary.json"], `"appVersion": "1.2.3"`) {
		t.Fatalf("summary = %s", files["summary.json"])
	}
	if strings.Contains(files["logs/ffmpeg.log"], "tok_abcdef1234567890") {
		t.Fatalf("log chưa che trong gói: %s", files["logs/ffmpeg.log"])
	}
	if strings.Contains(files["settings.json"], "sk-live-xyz") {
		t.Fatalf("settings chưa che: %s", files["settings.json"])
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

type fakeVersionSource struct {
	tag string
	err error
}

func (f fakeVersionSource) Latest(context.Context) (string, error) { return f.tag, f.err }

func TestVersionCheckerCachesAndCheckForces(t *testing.T) {
	src := fakeVersionSource{tag: "1.4.0"}
	v := NewVersionChecker("1.3.0", src)
	now := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	v.now = func() time.Time { return now }

	// GET thuần đọc: cache trống không được gọi mạng.
	if cold := v.Get(context.Background()); cold.Latest != nil || !cold.CheckedAt.IsZero() {
		t.Fatalf("GET gọi nguồn: %+v", cold)
	}
	got := v.Check(context.Background())
	if got.Latest == nil || *got.Latest != "1.4.0" || got.Version != "1.3.0" {
		t.Fatalf("info = %+v", got)
	}
	// Trong TTL không hỏi lại dù nguồn đổi.
	src.tag = "9.9.9"
	v.source = src
	if again := v.Get(context.Background()); again.Latest == nil || *again.Latest != "1.4.0" {
		t.Fatalf("hỏi lại trong TTL: %+v", again)
	}
	// Check bỏ qua TTL.
	if forced := v.Check(context.Background()); forced.Latest == nil || *forced.Latest != "9.9.9" {
		t.Fatalf("Check không hỏi lại: %+v", forced)
	}
}

// Che toàn bộ header Cookie/Set-Cookie và trường cookie đặt tên — không chỉ
// giá trị đầu; không còn mẩu bí mật nào trong tệp hay gói xuất.
func TestRedactWholeCookieHeaders(t *testing.T) {
	dir := t.TempDir()
	st := New(dir, "/Users/dat")
	lines := []string{
		`Cookie: SID=abc123; HSID=zzz; SSID=yyy`,
		`set-cookie: SESSDATA=sess-secret; bili_jct=jct-secret; buvid3=buv-secret`,
		`{"msg":"Cookie: SESSDATA=json-secret; bili_jct=json-jct"}`,
		`curl -H 'Authorization: Bearer tok-secret' https://x`,
	}
	for _, l := range lines {
		st.Append("chrome", LevelError, l)
	}
	for _, l := range lines {
		for _, secret := range []string{"abc123", "zzz", "yyy", "sess-secret", "jct-secret", "buv-secret", "json-secret", "json-jct", "tok-secret"} {
			if strings.Contains(st.Redact(l), secret) {
				t.Fatalf("%q chưa được che: %q", secret, st.Redact(l))
			}
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "chrome.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"abc123", "zzz", "yyy", "sess-secret", "jct-secret", "buv-secret", "json-secret", "json-jct", "tok-secret"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("tệp log còn %q", secret)
		}
	}
	bundle, err := st.BuildBundle(context.Background(), BundleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"sess-secret", "jct-secret", "buv-secret", "json-secret", "tok-secret"} {
		if strings.Contains(string(bundle.Bytes), secret) {
			t.Fatalf("gói xuất còn %q", secret)
		}
	}
}
