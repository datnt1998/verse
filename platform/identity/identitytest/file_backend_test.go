package identitytest_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/identity/identitytest"
)

// TestAccountContractFileBackend chạy contract suite trên file backend —
// baseline mà mọi backend khác phải khớp ngữ nghĩa.
// fileTestOptions là định danh project mà suite này cần: project ID dạng
// "du-an"/"p" hợp lệ, project legacy "_" gom các grant không tên miền.
var fileTestOptions = identity.Options{
	ValidProjectID: func(id string) bool {
		return id != "" && id != "." && id != ".." && !strings.HasPrefix(id, ".") && !strings.ContainsAny(id, `/\\`)
	},
	LegacyProjectID: "_",
}

func TestAccountContractFileBackend(t *testing.T) {
	identitytest.Run(t, func(t *testing.T) *identity.Store {
		t.Helper()
		return identity.NewStore(filepath.Join(t.TempDir(), "web-accounts.json"), fileTestOptions)
	})
}
