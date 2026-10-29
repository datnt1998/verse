package webauthtest_test

import (
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth/webauthtest"
)

// TestAccountContractFileBackend chạy contract suite trên file backend —
// baseline mà mọi backend khác phải khớp ngữ nghĩa.
func TestAccountContractFileBackend(t *testing.T) {
	webauthtest.Run(t, func(t *testing.T) *webauth.Store {
		t.Helper()
		return webauth.NewStore(filepath.Join(t.TempDir(), "web-accounts.json"))
	})
}
