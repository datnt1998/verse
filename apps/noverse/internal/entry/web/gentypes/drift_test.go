package main

import (
	"os"
	"testing"
)

// TestGeneratedTypesAreCurrent fails when web/src/api/types.gen.ts no longer
// matches the Go DTOs. Fix: go generate ./internal/entry/web
func TestGeneratedTypesAreCurrent(t *testing.T) {
	want, err := generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	path, err := outputPath()
	if err != nil {
		t.Fatalf("output path: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (run go generate ./internal/entry/web)", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s is stale: run go generate ./internal/entry/web", path)
	}
}
