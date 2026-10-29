package web

import (
	"context"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/version"
)

// TestServeSystemReportsBuildInfo 锁死 P0-2：/api/system 必须回传经 ldflags
// 注入的构建身份（version/commit/built），而不是 debug.BuildInfo 兜底值。
func TestServeSystemReportsBuildInfo(t *testing.T) {
	// NewGateway 对非 dev 版本会起后台检查；这里替换检查函数，保证测试不出网
	// 且结果确定（模拟远端已有更新）。
	old := checkLatestRelease
	checkLatestRelease = func(context.Context, version.CheckOptions) (*version.CheckResult, error) {
		return &version.CheckResult{Latest: "v9.9.10", HasUpdate: true}, nil
	}
	defer func() { checkLatestRelease = old }()

	tg := newTestGatewayWithBuild(t, version.Info{Version: "v9.9.9", Commit: "abc1234", Date: "2026-10-25"})
	cookie := tg.login("admin", "admin-pass")

	res := tg.do("GET", "/api/system", cookie, "")
	if res.Code != 200 {
		t.Fatalf("GET /api/system = %d: %s", res.Code, res.Body.String())
	}
	body := decodeJSONBody(t, res)
	if body["version"] != "v9.9.9" {
		t.Fatalf("version = %v, want v9.9.9", body["version"])
	}
	if body["commit"] != "abc1234" {
		t.Fatalf("commit = %v, want abc1234", body["commit"])
	}
	if body["built"] != "2026-10-25" {
		t.Fatalf("built = %v, want 2026-10-25", body["built"])
	}

	// 后台检查是异步的：等它落地后再查一次，latestVersion/hasUpdate 必须出现。
	deadline := time.Now().Add(5 * time.Second)
	for {
		res = tg.do("GET", "/api/system", cookie, "")
		body = decodeJSONBody(t, res)
		if body["latestVersion"] == "v9.9.10" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("latestVersion never propagated: %v", body["latestVersion"])
		}
		time.Sleep(10 * time.Millisecond)
	}
	if body["hasUpdate"] != true {
		t.Fatalf("hasUpdate = %v, want true", body["hasUpdate"])
	}
}

func newTestGatewayWithBuild(t *testing.T, build version.Info) *testGateway {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	return newTestGatewayForBuild(t, build)
}
