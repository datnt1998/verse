package secretbox_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/secretbox"
)

func TestReadinessRejectsUnsafeAndCorruptKeysWithoutReplacement(t *testing.T) {
	for _, tc := range []struct {
		name  string
		mode  os.FileMode
		size  int
		issue string
	}{
		{"world_readable", 0644, 32, "bad_permissions"},
		{"truncated", 0600, 31, "corrupt"},
		{"oversized", 0600, 33, "corrupt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "secret.key")
			original := bytes.Repeat([]byte{42}, tc.size)
			if err := os.WriteFile(path, original, tc.mode); err != nil {
				t.Fatal(err)
			}
			box := secretbox.New(dir)
			state := box.Readiness()
			if state.OK || state.Path != path || state.Issue != tc.issue {
				t.Fatalf("readiness=%+v", state)
			}
			if _, err := box.Seal([]byte("secret")); err == nil {
				t.Fatal("unsafe key accepted by Seal")
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(original, after) {
				t.Fatal("existing key replaced")
			}
		})
	}
}

func TestReadinessCreatesOneSecureReusableKey(t *testing.T) {
	box := secretbox.New(t.TempDir())
	state := box.Readiness()
	if !state.OK || state.Issue != "" {
		t.Fatalf("%+v", state)
	}
	sealed, err := box.Seal([]byte("test"))
	if err != nil {
		t.Fatal(err)
	}
	other := secretbox.New(filepath.Dir(box.Path()))
	if !other.Readiness().OK {
		t.Fatal("restart lost readiness")
	}
	plain, err := other.Reveal(sealed)
	if err != nil || string(plain) != "test" {
		t.Fatalf("%q %v", plain, err)
	}
	st, err := os.Stat(box.Path())
	if err != nil || st.Mode().Perm() != 0600 {
		t.Fatalf("insecure key: %v %v", st, err)
	}
}

func TestReadinessRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(target, make([]byte, 32), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "secret.key")); err != nil {
		t.Fatal(err)
	}
	state := secretbox.New(dir).Readiness()
	if state.OK || state.Issue != "bad_permissions" {
		t.Fatalf("%+v", state)
	}
}
