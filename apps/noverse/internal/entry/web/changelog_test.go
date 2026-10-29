package web

import (
	"context"
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/version"
)

// D1 “Có gì mới”: /api/system › releases[] = bản mới (kèm notes markdown) + bản
// đang chạy; notes rỗng khi ngoại tuyến. Không có lần gọi mạng nào thêm ở đây —
// số liệu lấy từ kết quả check đã cache (CheckLatest 24h).
func TestServeSystemReleases(t *testing.T) {
	old := checkLatestRelease
	defer func() { checkLatestRelease = old }()
	checkLatestRelease = func(context.Context, version.CheckOptions) (*version.CheckResult, error) {
		return &version.CheckResult{
			Latest:      "v9.9.10",
			HasUpdate:   true,
			LatestNotes: "## Mới\n- Bản đọc TTS",
			LatestDate:  "2026-10-25T10:00:00Z",
		}, nil
	}

	tg := newTestGatewayWithBuild(t, version.Info{Version: "v9.9.9", Commit: "abc1234", Date: "2026-10-20T09:00:00Z"})
	cookie := tg.login("admin", "admin-pass")

	deadline := time.Now().Add(5 * time.Second)
	for {
		res := tg.do("GET", "/api/system", cookie, "")
		if res.Code != 200 {
			t.Fatalf("GET /api/system = %d: %s", res.Code, res.Body.String())
		}
		body := decodeJSONBody(t, res)
		if body["latestVersion"] == "v9.9.10" {
			releases, ok := body["releases"].([]any)
			if !ok || len(releases) != 2 {
				t.Fatalf("releases = %#v", body["releases"])
			}
			fresh := releases[0].(map[string]any)
			current := releases[1].(map[string]any)
			if fresh["version"] != "v9.9.10" || fresh["notes"] != "## Mới\n- Bản đọc TTS" || fresh["date"] != "2026-10-25T10:00:00Z" {
				t.Fatalf("new release = %#v", fresh)
			}
			if current["version"] != "v9.9.9" || current["notes"] != "" || current["date"] != "2026-10-20T09:00:00Z" {
				t.Fatalf("current release = %#v", current)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("latestVersion never propagated: %v", body["latestVersion"])
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Ngoại tuyến: chỉ còn bản đang chạy, notes rỗng, không lỗi.
	checkLatestRelease = func(context.Context, version.CheckOptions) (*version.CheckResult, error) {
		return nil, nil
	}
	offline := newTestGatewayWithBuild(t, version.Info{Version: "v9.9.9", Commit: "abc1234", Date: "2026-10-20T09:00:00Z"})
	offlineCookie := offline.login("admin", "admin-pass")
	res := offline.do("GET", "/api/system", offlineCookie, "")
	if res.Code != 200 {
		t.Fatalf("offline GET /api/system = %d: %s", res.Code, res.Body.String())
	}
	body := decodeJSONBody(t, res)
	releases, ok := body["releases"].([]any)
	if !ok || len(releases) != 1 {
		t.Fatalf("offline releases = %#v", body["releases"])
	}
	only := releases[0].(map[string]any)
	if only["version"] != "v9.9.9" || only["notes"] != "" {
		t.Fatalf("offline release = %#v", only)
	}
}
