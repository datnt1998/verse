package blobstore

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestCopyResumeHash is the copy acceptance case: local→(MinIO or
// local)→new local, resume after a killed run, and same-size corruption —
// every target object is verified by re-download SHA-256+size.
func TestCopyResumeHash(t *testing.T) {
	ctx := context.Background()
	src, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dst, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer dst.Close()

	// 5 objects incl. empty + >5MiB.
	var keys []string
	for i := range 5 {
		key := fmt.Sprintf("tenant/obj-%02d", i)
		size := (i + 1) * 1024
		if i == 4 {
			size = 6 << 20
		}
		payload := make([]byte, size)
		_, _ = rand.Read(payload)
		if _, err := src.Put(ctx, key, bytes.NewReader(payload), PutOptions{}); err != nil {
			t.Fatalf("seed Put %s: %v", key, err)
		}
		keys = append(keys, key)
	}

	manifest := filepath.Join(t.TempDir(), "copy-manifest.jsonl")

	// Killed run: ledger pre-seeded as if the first two keys finished.
	f, err := os.OpenFile(manifest, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		rc, _, _ := src.Open(ctx, keys[i], FullRange)
		sha, size, _ := DigestAll(rc)
		rc.Close()
		// Physically copy it too — a copied ledger line must match reality.
		rc2, info, _ := src.Open(ctx, keys[i], FullRange)
		_, _ = dst.Put(ctx, keys[i], rc2, PutOptions{ContentType: info.ContentType})
		rc2.Close()
		fmt.Fprintf(f, `{"key":%q,"size":%d,"sha256":%q,"status":"copied","at":%q}`+"\n",
			keys[i], size, sha, time.Now().UTC().Format(time.RFC3339))
	}
	f.Close()

	// Without --resume the existing manifest must hard-fail.
	if _, err := NewCopier(src, dst).Run(ctx, CopyOptions{ManifestPath: manifest}); err == nil {
		t.Fatal("run without --resume on existing manifest must fail")
	}

	rep, err := NewCopier(src, dst).Run(ctx, CopyOptions{ManifestPath: manifest, Resume: true})
	if err != nil {
		t.Fatalf("resume run: %v", err)
	}
	if rep.SourceObjects != 5 || rep.Copied != 5 || rep.Mismatches+rep.Conflicts+rep.Errors != 0 {
		t.Fatalf("report = %+v, want source=5 copied=5 clean", rep)
	}

	// Full hash verification of every target object.
	for _, key := range keys {
		sSHA, sSize, _ := hashOf(ctx, src, key)
		dSHA, dSize, err := hashOf(ctx, dst, key)
		if err != nil {
			t.Fatalf("target missing %s: %v", key, err)
		}
		if sSHA != dSHA || sSize != dSize {
			t.Fatalf("%s: src %s/%d dst %s/%d", key, sSHA, sSize, dSHA, dSize)
		}
	}

	// Second run: everything already matched.
	rep2, err := NewCopier(src, dst).Run(ctx, CopyOptions{ManifestPath: manifest, Resume: true})
	if err != nil {
		t.Fatalf("idempotent run: %v", err)
	}
	if rep2.Matched != 0 || rep2.Copied != 5 {
		// resume path counts ledger entries as copied
		t.Fatalf("resume report = %+v", rep2)
	}
}

// TestCopyConflictAndCorruption: a target object of the SAME size but
// different bytes must surface as conflict (existing) or mismatch (written
// wrong), never silently overwritten.
func TestCopyConflictAndCorruption(t *testing.T) {
	ctx := context.Background()
	src, _ := NewLocal(t.TempDir())
	defer src.Close()
	dst, _ := NewLocal(t.TempDir())
	defer dst.Close()

	payloadA := bytes.Repeat([]byte("A"), 100)
	payloadB := bytes.Repeat([]byte("B"), 100) // same size, different hash

	if _, err := src.Put(ctx, "k/same", bytes.NewReader(payloadA), PutOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := dst.Put(ctx, "k/same", bytes.NewReader(payloadB), PutOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := src.Put(ctx, "k/new", bytes.NewReader(payloadA), PutOptions{}); err != nil {
		t.Fatal(err)
	}

	rep, err := NewCopier(src, dst).Run(ctx, CopyOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if rep.Conflicts != 1 || rep.Copied != 1 {
		t.Fatalf("report = %+v, want conflicts=1 copied=1", rep)
	}
	// Conflict target must be untouched.
	rc, _, _ := dst.Open(ctx, "k/same", FullRange)
	body, _ := readAll(rc)
	rc.Close()
	if !bytes.Equal(body, payloadB) {
		t.Fatal("conflicting target was overwritten")
	}
}

func readAll(r interface{ Read([]byte) (int, error) }) ([]byte, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r)
	return buf.Bytes(), err
}

// TestCopyLocalToMinIO runs the same verified copy against real MinIO as the
// destination (also exercises s3↔local direction via a chained copy).
func TestCopyLocalToMinIO(t *testing.T) {
	ctx := context.Background()
	src, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dst := newMinioStore(t)

	payloads := map[string][]byte{}
	for i := range 4 {
		key := fmt.Sprintf("chain/obj-%d", i)
		p := make([]byte, (i+2)*77777)
		_, _ = rand.Read(p)
		payloads[key] = p
		if _, err := src.Put(ctx, key, bytes.NewReader(p), PutOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	manifest := filepath.Join(t.TempDir(), "m.jsonl")
	rep, err := NewCopier(src, dst).Run(ctx, CopyOptions{ManifestPath: manifest, Concurrency: 3})
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if rep.Copied != 4 || rep.Mismatches+rep.Conflicts+rep.Errors != 0 {
		t.Fatalf("report = %+v", rep)
	}
	for key, p := range payloads {
		want, _, _ := DigestAll(bytes.NewReader(p))
		got, _, err := hashOf(ctx, dst, key)
		if err != nil {
			t.Fatalf("target read %s: %v", key, err)
		}
		if got != want {
			t.Fatalf("%s: target sha %s want %s", key, got, want)
		}
	}

	// Chain: MinIO → a third local dir proves s3→local direction.
	dst2, _ := NewLocal(t.TempDir())
	defer dst2.Close()
	rep2, err := NewCopier(dst, dst2).Run(ctx, CopyOptions{})
	if err != nil {
		t.Fatalf("s3→local copy: %v", err)
	}
	if rep2.Copied != 4 || rep2.Mismatches+rep2.Conflicts+rep2.Errors != 0 {
		t.Fatalf("s3→local report = %+v", rep2)
	}
	for key, p := range payloads {
		want, _, _ := DigestAll(bytes.NewReader(p))
		got, _, err := hashOf(ctx, dst2, key)
		if err != nil || got != want {
			t.Fatalf("%s roundtrip: sha=%s err=%v", key, got, err)
		}
	}
}
