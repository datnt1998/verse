package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/noverse/internal/webauth"
)

func TestReviewCommandLineAuditActor(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	original := os.Stdin
	defer func() { os.Stdin = original }()
	for _, args := range [][]string{{"add", "admin", "--role", "admin"}, {"add", "member"}, {"passwd", "member"}, {"rm", "member"}} {
		path := filepath.Join(t.TempDir(), "password")
		if err := os.WriteFile(path, []byte("temporary-pass\n"), 0600); err != nil {
			t.Fatal(err)
		}
		input, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		os.Stdin = input
		err = runServeUser(args)
		input.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	path, err := webauth.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	page, err := webauth.NewStore(path).Audit(webauth.AuditQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) < 4 {
		t.Fatal("missing CLI actions")
	}
	for _, entry := range page.Items {
		if entry.Actor == nil || entry.Actor.Name != "Dòng lệnh" {
			t.Fatalf("CLI action attributed incorrectly: %+v", entry)
		}
	}
}
