package bootstrap

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/core/auth"
)

// init() trong identity.go phải cài đúng định danh Noverse: credential đi vào
// ~/.ainovel/auth.json và thông báo lỗi đăng nhập giữ nguyên văn bản hiện hữu.
func TestAuthIdentityIsNoverse(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := auth.DefaultStorePath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".ainovel", "auth.json")
	if path != want {
		t.Fatalf("DefaultStorePath = %q, want %q", path, want)
	}

	err = auth.NotLoggedInError("openai-codex")
	if !errors.Is(err, auth.ErrNotLoggedIn) {
		t.Fatalf("NotLoggedInError phải bọc ErrNotLoggedIn: %v", err)
	}
	const wantMsg = "chưa đăng nhập openai-codex — chạy `ainovel-cli login openai-codex` để đăng nhập"
	if err.Error() != wantMsg {
		t.Fatalf("NotLoggedInError = %q, want %q", err.Error(), wantMsg)
	}
}
