package main

import (
	"context"
	"fmt"

	"github.com/datnt1998/verse/apps/viderse/internal/bootstrap"
	"github.com/datnt1998/verse/platform/identity/identitycli"
)

const userUsage = `Quản lý quyền app trên kho tài khoản chung:
  viderse user grant <tên> --app noverse|viderse   cấp quyền dùng một app
  viderse user revoke <tên> --app noverse|viderse  thu hồi quyền một app
Người chạy là operator cục bộ giữ VERSE_IDENTITY_DATABASE_URL — không nhận
--actor, không cấp role, không tạo tài khoản từ đây.`

// runUser chạy các lệnh operator trên kho tài khoản chung. Chỉ có grant/
// revoke: vòng đời tài khoản thuộc app chủ kho; Viderse chỉ cầm role
// viderse_identity nên không thể (và không được) tạo user.
func runUser(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("thiếu lệnh\n%s", userUsage)
	}
	// Parse trước khi mở kho: lỗi ngữ pháp không đụng DB.
	cmd, err := identitycli.ParseGrantArgs(args)
	if err != nil {
		return err
	}
	identityDSN := bootstrap.IdentityDSNFromEnv()
	if identityDSN == "" {
		return fmt.Errorf("thiếu VERSE_IDENTITY_DATABASE_URL (xem apps/viderse/deploy/README.md)")
	}
	accounts, closeAccounts, err := openAccounts(context.Background(), identityDSN)
	if err != nil {
		return err
	}
	defer closeAccounts()
	if err := identitycli.RunGrant(accounts, cmd); err != nil {
		return err
	}
	verb := "cấp"
	if cmd.Verb == identitycli.GrantVerbRevoke {
		verb = "thu hồi"
	}
	fmt.Printf("Đã %s quyền dùng %s cho %q.\n", verb, cmd.App, cmd.Username)
	return nil
}
