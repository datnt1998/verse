package serverstorage

import (
	"github.com/datnt1998/verse/platform/blobstore"
	"testing"
)

func TestCandidateCredentialOmissionAndProviderNormalization(t *testing.T) {
	old := blobstore.Config{Driver: "s3", Provider: "spaces", Delivery: "proxy", S3: blobstore.S3Config{Endpoint: "https://nyc3.digitaloceanspaces.com", Region: "nyc3", Bucket: "media", AccessKeyID: "saved-id", SecretAccessKey: "saved-secret"}}
	candidate := S3Config{Provider: "do", Endpoint: old.S3.Endpoint, Region: "nyc3", Bucket: "media", Delivery: "proxy"}
	cfg, err := normalizeConfig(candidate, &old)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "spaces" || cfg.S3.AccessKeyID != "saved-id" || cfg.S3.SecretAccessKey != "saved-secret" {
		t.Fatalf("omitted credentials not retained: provider=%q", cfg.Provider)
	}
	candidate.Bucket = "other"
	if _, err := normalizeConfig(candidate, &old); err == nil {
		t.Fatal("new namespace inherited unrelated credentials")
	}
	candidate.Bucket = "media"
	candidate.AccessKeyID = "new-id"
	if _, err := normalizeConfig(candidate, &old); err == nil {
		t.Fatal("partial credential pair accepted")
	}
}
func TestCandidateRejectsUnsafeEndpointAndDriverAlias(t *testing.T) {
	for _, endpoint := range []string{"http://example.org/path", "https://user:secret@example.org", "https://example.org?token=secret", "http://example.org"} {
		_, err := normalizeConfig(S3Config{Provider: "other", Endpoint: endpoint, Region: "garage", Bucket: "media", Delivery: "proxy", AccessKeyID: "id", SecretAccessKey: "secret"}, nil)
		if err == nil {
			t.Fatalf("unsafe endpoint accepted %q", endpoint)
		}
	}
	_, err := normalizeConfig(S3Config{Provider: "invented", Region: "garage", Bucket: "media", Delivery: "proxy", AccessKeyID: "id", SecretAccessKey: "secret"}, nil)
	if err == nil {
		t.Fatal("unknown provider accepted")
	}
}
