package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/platform/blobstore"
)

// TestBlobConfig covers the STORAGE_DRIVER/S3_* env contract phase 03 locks:
// local requires an absolute root, s3 requires bucket/region/credentials,
// booleans parse strictly, and plain-http endpoints need explicit opt-in.
func TestBlobConfig(t *testing.T) {
	clear := func(t *testing.T) {
		t.Helper()
		for _, key := range []string{
			"STORAGE_DRIVER", "BLOB_LOCAL_ROOT", "S3_ENDPOINT", "S3_REGION",
			"S3_BUCKET", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY",
			"S3_FORCE_PATH_STYLE", "S3_ALLOW_INSECURE_ENDPOINT",
		} {
			t.Setenv(key, "")
		}
	}
	s3env := func(t *testing.T) {
		t.Helper()
		t.Setenv("STORAGE_DRIVER", "s3")
		t.Setenv("S3_REGION", "us-east-1")
		t.Setenv("S3_BUCKET", "bucket")
		t.Setenv("S3_ACCESS_KEY_ID", "akid")
		t.Setenv("S3_SECRET_ACCESS_KEY", "secret")
	}

	t.Run("local_requires_root", func(t *testing.T) {
		clear(t)
		t.Setenv("STORAGE_DRIVER", "local")
		if _, err := BlobConfigFromEnv(); err == nil {
			t.Fatal("local without BLOB_LOCAL_ROOT must fail")
		}
	})

	t.Run("local_root", func(t *testing.T) {
		clear(t)
		t.Setenv("BLOB_LOCAL_ROOT", t.TempDir())
		cfg, err := BlobConfigFromEnv()
		if err != nil {
			t.Fatalf("local env: %v", err)
		}
		if cfg.Driver != BlobDriverLocal {
			t.Fatalf("driver = %q", cfg.Driver)
		}
	})

	t.Run("invalid_driver_fails", func(t *testing.T) {
		clear(t)
		t.Setenv("STORAGE_DRIVER", "azure")
		if _, err := BlobConfigFromEnv(); err == nil {
			t.Fatal("unknown driver must fail")
		}
	})

	t.Run("s3_requires_region_bucket_creds", func(t *testing.T) {
		clear(t)
		t.Setenv("STORAGE_DRIVER", "s3")
		if _, err := BlobConfigFromEnv(); err == nil {
			t.Fatal("s3 without bucket/region/creds must fail")
		}
	})

	t.Run("s3_force_path_style_strict", func(t *testing.T) {
		clear(t)
		s3env(t)
		t.Setenv("S3_FORCE_PATH_STYLE", "yes") // not the strict contract
		if _, err := BlobConfigFromEnv(); err == nil {
			t.Fatal("non-strict bool must fail")
		}
		t.Setenv("S3_FORCE_PATH_STYLE", "true")
		cfg, err := BlobConfigFromEnv()
		if err != nil {
			t.Fatalf("strict true: %v", err)
		}
		if !cfg.S3.ForcePathStyle {
			t.Fatal("ForcePathStyle not propagated")
		}
	})

	t.Run("s3_http_endpoint_requires_optin", func(t *testing.T) {
		clear(t)
		s3env(t)
		t.Setenv("S3_ENDPOINT", "http://minio:9000")
		if _, err := OpenBlobStore(mustBlobCfg(t)); err == nil {
			t.Fatal("http endpoint without opt-in must fail")
		}
		t.Setenv("S3_ALLOW_INSECURE_ENDPOINT", "true")
		cfg, err := BlobConfigFromEnv()
		if err != nil {
			t.Fatalf("opt-in env: %v", err)
		}
		if !cfg.S3.AllowPlainHTTP {
			t.Fatal("insecure flag not propagated to S3Config")
		}
	})

	t.Run("s3_safe_summary_hides_secrets", func(t *testing.T) {
		clear(t)
		s3env(t)
		t.Setenv("S3_SECRET_ACCESS_KEY", "Sup3rSecret")
		cfg := mustBlobCfg(t)
		if strings.Contains(cfg.SafeSummary(), "Sup3rSecret") || strings.Contains(cfg.SafeSummary(), "akid") {
			t.Fatalf("SafeSummary leaked credentials: %s", cfg.SafeSummary())
		}
		// Summaries must also remain safe before configuration validation.
		for _, endpoint := range []string{
			"https://user:EndpointSecret@s3.example.com/path?token=QuerySecret#FragmentSecret",
			"not a URL:EndpointSecret",
		} {
			cfg.S3.Endpoint = endpoint
			summary := cfg.SafeSummary()
			for _, secret := range []string{"EndpointSecret", "QuerySecret", "FragmentSecret", "/path"} {
				if strings.Contains(summary, secret) {
					t.Fatalf("SafeSummary leaked endpoint content: %s", summary)
				}
			}
		}
	})

	t.Run("config_file_requires_0600", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "s3.json")
		body := `{"driver":"s3","s3Region":"us-east-1","s3Bucket":"b","s3AccessKeyId":"k","s3SecretAccessKey":"s"}`
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := BlobConfigFromFile(p); err == nil {
			t.Fatal("group/world-readable credential file must be rejected")
		}
		if err := os.Chmod(p, 0o600); err != nil {
			t.Fatal(err)
		}
		cfg, err := BlobConfigFromFile(p)
		if err != nil {
			t.Fatalf("0600 file: %v", err)
		}
		if cfg.Driver != BlobDriverS3 || cfg.S3.Bucket != "b" {
			t.Fatalf("cfg = %+v", cfg)
		}
	})

	t.Run("config_file_local", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "local.json")
		root := t.TempDir()
		body := `{"driver":"local","localRoot":"` + root + `"}`
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg, err := BlobConfigFromFile(p)
		if err != nil {
			t.Fatal(err)
		}
		st, err := OpenBlobStore(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := st.(*blobstore.Local); !ok {
			t.Fatalf("driver = %T, want *Local", st)
		}
	})
}

func mustBlobCfg(t *testing.T) BlobConfig {
	t.Helper()
	cfg, err := BlobConfigFromEnv()
	if err != nil {
		t.Fatalf("env parse: %v", err)
	}
	return cfg
}
