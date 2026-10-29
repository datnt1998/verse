package version

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// D1 “Có gì mới”: CheckLatest 必须把 release 说明与发布时间一起缓存下来，
// 24h 内第二次调用不联网也能给出同样的 notes。
func TestCheckLatestCachesReleaseNotes(t *testing.T) {
	const body = "## Mới\n- Xuất bản đọc TTS\n## Sửa\n- Biên tập viên ghi số mục luật"
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		payload, _ := json.Marshal(map[string]any{
			"tag_name":     "v1.3.0",
			"body":         body,
			"published_at": "2026-10-25T10:00:00Z",
			"assets":       []any{},
		})
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	opts := CheckOptions{CurrentVersion: "v1.2.0", Client: stubLatestClient(server), CacheDir: cacheDir}
	res, err := CheckLatest(context.Background(), opts)
	if err != nil || res == nil {
		t.Fatalf("CheckLatest = %+v, %v", res, err)
	}
	if res.LatestNotes != body || res.LatestDate != "2026-10-25T10:00:00Z" || !res.HasUpdate {
		t.Fatalf("first check = %+v", res)
	}

	cached, err := CheckLatest(context.Background(), opts)
	if err != nil || cached == nil {
		t.Fatalf("cached CheckLatest = %+v, %v", cached, err)
	}
	if hits != 1 {
		t.Fatalf("24h cache must not hit the network, hits=%d", hits)
	}
	if cached.LatestNotes != body || cached.LatestDate != "2026-10-25T10:00:00Z" {
		t.Fatalf("cached notes/date lost: %+v", cached)
	}

	// 缓存文件里确实存了 notes（离线重启用缓存也能出“Có gì mới”）。
	data, err := os.ReadFile(filepath.Join(cacheDir, latestCacheFile))
	if err != nil {
		t.Fatal(err)
	}
	var raw latestCache
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if raw.Notes != body || raw.Date != "2026-10-25T10:00:00Z" {
		t.Fatalf("cache file = %+v", raw)
	}
	if time.Since(time.Unix(raw.CheckedAt, 0)) > time.Hour {
		t.Fatalf("cache timestamp not refreshed: %d", raw.CheckedAt)
	}
}

// D1: releases[] = bản mới (khi có) + bản đang chạy; notes rỗng khi không có dữ liệu.
func TestChangelogShapes(t *testing.T) {
	const buildDate = "2026-10-20T09:00:00Z"
	cases := []struct {
		name    string
		res     CheckResult
		current string
		want    []ReleaseNote
	}{
		{
			name:    "có bản mới",
			res:     CheckResult{Latest: "v1.3.0", HasUpdate: true, LatestNotes: "N3", LatestDate: "2026-10-25T10:00:00Z"},
			current: "v1.2.0",
			want: []ReleaseNote{
				{Version: "v1.3.0", Date: "2026-10-25T10:00:00Z", Notes: "N3"},
				{Version: "v1.2.0", Date: buildDate, Notes: ""},
			},
		},
		{
			name:    "đang dùng bản mới nhất",
			res:     CheckResult{Latest: "v1.2.0", LatestNotes: "N2", LatestDate: "2026-10-20T09:00:00Z"},
			current: "v1.2.0",
			want:    []ReleaseNote{{Version: "v1.2.0", Date: buildDate, Notes: "N2"}},
		},
		{
			name:    "ngoại tuyến (không có dữ liệu)",
			res:     CheckResult{},
			current: "v1.2.0",
			want:    []ReleaseNote{{Version: "v1.2.0", Date: buildDate}},
		},
		{
			name:    "bản dev",
			res:     CheckResult{},
			current: "dev",
			want:    []ReleaseNote{{Version: "dev", Date: buildDate}},
		},
		{
			name:    "remote cũ hơn bản đang chạy",
			res:     CheckResult{Latest: "v1.0.0", LatestNotes: "N1", LatestDate: "2026-09-01T00:00:00Z"},
			current: "v1.2.0",
			want:    []ReleaseNote{{Version: "v1.2.0", Date: buildDate}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Changelog(tc.res, tc.current, buildDate)
			if len(got) != len(tc.want) {
				t.Fatalf("releases = %+v, want %+v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("releases[%d] = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}

	// Thời điểm build "unknown" không được lọt vào date.
	if got := Changelog(CheckResult{}, "v1.2.0", "unknown"); got[0].Date != "" {
		t.Fatalf("unknown build date leaked: %+v", got[0])
	}
}
