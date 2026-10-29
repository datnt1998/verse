package auth

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testApp là định danh app giả cho mọi test của package — cố tình không phải
// app thật nào để test còn phát hiện được chuỗi app cứng lọt vào trang/lỗi.
var testApp = Options{
	AppName:              "Viderse",
	Purpose:              "để quay phim",
	LoginCommand:         "viderse login",
	StoreDir:             ".viderse",
	ClaudeCodeVersionEnv: "VIDERSE_CLAUDE_CODE_VERSION",
}

// useTestApp cài định danh app giả cho test hiện tại (không khôi phục — mọi
// test dùng chung một bộ giá trị nên thứ tự cài không đổi kết quả).
func useTestApp(t *testing.T) {
	t.Helper()
	Configure(testApp)
}

// Kho credential phải đi theo Options.StoreDir: credential ghi qua
// DefaultStore rơi vào $HOME/.viderse/auth.json, không đụng ~/.ainovel.
func TestDefaultStoreFollowsStoreDir(t *testing.T) {
	useTestApp(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := DefaultStorePath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".viderse", "auth.json")
	if path != want {
		t.Fatalf("DefaultStorePath = %q, want %q", path, want)
	}

	store, err := DefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	cred := Credential{Access: "at", Refresh: "rt", Expires: 1_700_000_000_000}
	if err := store.Put("openai-codex", cred); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("credential không được ghi vào %s: %v", want, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".ainovel")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("không được tạo ~/.ainovel, stat err = %v", err)
	}

	got, ok, err := store.Get("openai-codex")
	if err != nil || !ok || got.Access != "at" {
		t.Fatalf("Get = %+v ok=%v err=%v", got, ok, err)
	}
}

// Chưa Configure thì DefaultStorePath/DefaultStore phải báo lỗi thay vì rơi
// vào một đường mặc định ngầm.
func TestDefaultStorePathRequiresConfigure(t *testing.T) {
	Configure(Options{})
	if _, err := DefaultStorePath(); err == nil {
		t.Fatal("DefaultStorePath phải lỗi khi chưa cấu hình StoreDir")
	}
	if _, err := DefaultStore(); err == nil {
		t.Fatal("DefaultStore phải lỗi khi chưa cấu hình StoreDir")
	}
	useTestApp(t)
}

// Trang callback phải mang định danh app được cài và không để lọt chuỗi của
// app khác vào bất kỳ trạng thái nào.
func TestCallbackPageUsesConfiguredApp(t *testing.T) {
	useTestApp(t)
	for _, kind := range []callbackKind{callbackSuccess, callbackDenied, callbackStateMismatch, callbackMissingCode} {
		for _, from := range []string{"cli", "web"} {
			_, _, body := renderCallback(t, kind, from, "boom")
			if strings.Contains(body, "Noverse") || strings.Contains(body, "ainovel") {
				t.Errorf("%s/%s: trang lọt chuỗi Noverse/ainovel", kind, from)
			}
			if !strings.Contains(body, "Viderse") {
				t.Errorf("%s/%s: trang thiếu tên app Viderse", kind, from)
			}
		}
	}
	_, _, body := renderCallback(t, callbackSuccess, "cli", "")
	if !strings.Contains(body, "Viderse đã được phép dùng gói ChatGPT của bạn để quay phim.") {
		t.Error("vế Purpose không được ghép đúng")
	}
}

// Hint trong lỗi phải dùng LoginCommand đã cài; chưa cấu hình thì rơi về câu
// trung lập không kèm lệnh.
func TestNotLoggedInErrorFollowsLoginCommand(t *testing.T) {
	useTestApp(t)
	err := NotLoggedInError("openai-codex")
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("NotLoggedInError phải bọc ErrNotLoggedIn: %v", err)
	}
	if !strings.Contains(err.Error(), "`viderse login openai-codex`") {
		t.Fatalf("lỗi thiếu lệnh đã cấu hình: %v", err)
	}
	if strings.Contains(err.Error(), "ainovel") || strings.Contains(err.Error(), "Noverse") {
		t.Fatalf("lỗi lọt chuỗi app khác: %v", err)
	}

	Configure(Options{})
	neutral := NotLoggedInError("openai-codex").Error()
	if strings.Contains(neutral, "viderse") || strings.Contains(neutral, "ainovel") {
		t.Fatalf("lỗi trung lập lọt lệnh: %v", neutral)
	}
	useTestApp(t)
}
