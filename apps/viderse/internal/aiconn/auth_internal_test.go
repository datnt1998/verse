package aiconn

// Cửa sổ refresh cho auth.json của Codex CLI: CLI tự refresh khi token còn
// ≤5 phút (codex-rs/login/src/auth/manager.rs, CHATGPT_ACCESS_TOKEN_REFRESH_
// WINDOW_MINUTES), mà auth.json cô lập không có refresh token — nên token phải
// còn dư cho TRỌN một lượt CLI (20' gọi + 5' ngưỡng + 1' chuẩn bị).

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/core/auth"
)

func TestNeedsCLIRefresh(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		expires time.Time
		want    bool
	}{
		{"không hạn", time.Time{}, false},
		{"còn 1 giờ", now.Add(time.Hour), false},
		{"còn đúng cửa sổ", now.Add(cliRefreshWindow), false},
		{"còn cửa sổ trừ 1 phút", now.Add(cliRefreshWindow - time.Minute), true},
		{"còn 4 phút (CLI sẽ tự refresh)", now.Add(4 * time.Minute), true},
		{"đã hết hạn", now.Add(-time.Minute), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cred := auth.Credential{}
			if !tc.expires.IsZero() {
				cred.Expires = tc.expires.UnixMilli()
			}
			if got := needsCLIRefresh(cred, now); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

// fakeCLI là provider OAuth giả cho test CLITokens (không gọi mạng).
type fakeCLI struct {
	mu        sync.Mutex
	refreshTo int64 // Expires mới sau refresh; 0 → refresh lỗi
	refreshes int
}

func (f *fakeCLI) ID() string   { return "cli-test" }
func (f *fakeCLI) Name() string { return "CLI test" }

func (f *fakeCLI) Login(context.Context, auth.Controller, auth.LoginOptions) (auth.Credential, error) {
	return auth.Credential{}, errors.New("không dùng trong test")
}

func (f *fakeCLI) Refresh(ctx context.Context, cred auth.Credential) (auth.Credential, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refreshes++
	if f.refreshTo == 0 {
		return auth.Credential{}, errors.New("invalid_grant")
	}
	cred.Access = "fresh-access"
	cred.Expires = f.refreshTo
	return cred, nil
}

// CLITokens không bao giờ phát token mà Codex CLI sẽ tự refresh giữa lượt:
// refresh hỏng hoặc token còn ít thời gian là LỖI, không phải "dùng tạm".
func TestCLITokensOnlyIssuesCLISafeTokens(t *testing.T) {
	fake := &fakeCLI{}
	auth.Register(fake)
	dir := t.TempDir()
	store := auth.NewStore(filepath.Join(dir, "auth.json"))
	mgr := auth.NewManager(store)
	core := NewCoreAuth(mgr, nil)
	now := time.Now()

	cases := []struct {
		name      string
		remaining time.Duration
		refreshTo time.Duration // 0 → refresh lỗi
		wantErr   bool
		wantToken string
	}{
		{"còn 10 phút + refresh hỏng", 10 * time.Minute, 0, true, ""},
		{"còn 10 phút + refresh ra 2 giờ", 10 * time.Minute, 2 * time.Hour, false, "fresh-access"},
		{"còn 10 phút + refresh vẫn ngắn", 10 * time.Minute, 20 * time.Minute, true, ""},
		{"còn 1 giờ (không cần refresh)", time.Hour, 0, false, "old-access"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cred := auth.Credential{Access: "old-access", Refresh: "r", AccountID: "acct",
				Expires: now.Add(tc.remaining).UnixMilli()}
			if err := store.Put("cli-test", cred); err != nil {
				t.Fatal(err)
			}
			fake.mu.Lock()
			fake.refreshes, fake.refreshTo = 0, 0
			if tc.refreshTo > 0 {
				fake.refreshTo = now.Add(tc.refreshTo).UnixMilli()
			}
			fake.mu.Unlock()

			token, account, err := core.CLITokens(context.Background(), "cli-test")
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, muốn lỗi = %v", err, tc.wantErr)
			}
			if tc.wantErr {
				if token != "" || account != "" {
					t.Fatalf("lỗi mà vẫn phát token: %q %q", token, account)
				}
				return
			}
			if token != tc.wantToken || account != "acct" {
				t.Fatalf("token = %q %q, muốn %q", token, account, tc.wantToken)
			}
		})
	}
}
