package host

import (
	"context"

	"github.com/datnt1998/noverse/internal/auth"
)

// OAuthAccount trả về nhãn tài khoản đã đăng nhập cho provider OAuth id
// (openai-codex, xai-oauth, anthropic-oauth); "" nếu chưa đăng nhập hoặc không đọc được kho credential.
func OAuthAccount(id string) string {
	store, err := auth.DefaultStore()
	if err != nil {
		return ""
	}
	cred, ok, err := store.Get(id)
	if err != nil || !ok {
		return ""
	}
	return accountLabel(cred)
}

// LoginOAuth chạy luồng đăng nhập của provider id (ChatGPT/Claude: trình duyệt + callback
// localhost; xAI: device-code), lưu credential và trả về nhãn tài khoản. ctrl nhận
// mã thiết bị/URL để UI hiển thị. Huỷ bằng ctx.
func LoginOAuth(ctx context.Context, id string, ctrl auth.Controller) (string, error) {
	store, err := auth.DefaultStore()
	if err != nil {
		return "", err
	}
	cred, err := auth.Login(ctx, store, id, ctrl, auth.LoginOptions{})
	if err != nil {
		return "", err
	}
	return accountLabel(cred), nil
}

func accountLabel(cred auth.Credential) string {
	if label := cred.Label(); label != "" {
		return label
	}
	return "Đã đăng nhập"
}
