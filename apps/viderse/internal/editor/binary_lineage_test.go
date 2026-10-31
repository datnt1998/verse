package editor

import (
	"crypto/sha256"
	"encoding"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestHeld9OriginalUploadStateMatchesPersistedBinding(t *testing.T) {
	// Captured from the real held9 export's authoritative ready metadata. The
	// current replacement SHA differs from its original, still-bound upload.
	meta := Object{"stagedHashState": "c2hhA9Cp7t6UTrw+sutM+5t5u09IpADr8M38NCg9A6CxganUAF7FAABqYwAAhUcACppGAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACseP", "uploadedBytes": 706447, "size": 622337, "stagedSha256": "66b5e8eb21dfbe5da660fcae7c40fcdd2e33d508832e30941aba72e0f55756d2"}
	digest, size, ok := originalUploadLineage(meta)
	if !ok || digest != "c80e3d99c6767054e52345f25d0b204b5917736654f6164c057f645fbdfea8ca" || size != 706447 {
		t.Fatalf("held9 lineage=%s %d %v", digest, size, ok)
	}
}

func TestOriginalUploadLineageExplicitProofDoesNotFallback(t *testing.T) {
	h := sha256.New()
	h.Write([]byte("original"))
	state, err := h.(encoding.BinaryMarshaler).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	original := hex.EncodeToString(h.Sum(nil))
	legacy := Object{"uploadedBytes": 8, "stagedHashState": base64.StdEncoding.EncodeToString(state)}
	digest, size, ok := originalUploadLineage(legacy)
	if !ok || digest != original || size != 8 {
		t.Fatalf("legacy=%s %d %v", digest, size, ok)
	}
	explicit := Object{"originalSha256": original, "originalSize": 8}
	digest, size, ok = originalUploadLineage(explicit)
	if !ok || digest != original || size != 8 {
		t.Fatalf("explicit=%s %d %v", digest, size, ok)
	}
	for _, meta := range []Object{
		{"originalSha256": "bad", "originalSize": 8, "uploadedBytes": 8, "stagedHashState": legacy["stagedHashState"]},
		{"originalSize": 8, "uploadedBytes": 8, "stagedHashState": legacy["stagedHashState"]},
		{"originalSha256": original, "originalSize": 0, "uploadedBytes": 8, "stagedHashState": legacy["stagedHashState"]},
		{"originalSha256": original, "originalSize": 1.5},
		{"uploadedBytes": 8, "stagedHashState": "corrupt"},
	} {
		if _, _, ok = originalUploadLineage(meta); ok {
			t.Fatalf("invalid lineage accepted: %v", meta)
		}
	}
}
