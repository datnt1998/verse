package blobstore

import (
	"strings"
	"testing"
)

func lookup(values map[string]string) LookupEnv {
	return func(key string) (string, bool) { v, ok := values[key]; return v, ok }
}

func TestParseEnvContract(t *testing.T) {
	env := map[string]string{"STORAGE_DRIVER": "s3", "S3_PROVIDER": "garage", "S3_ENDPOINT": "http://garage:3900", "S3_PRESIGN_ENDPOINT": "https://files.example.com", "S3_ALLOW_INSECURE_ENDPOINT": "true", "S3_BUCKET": "media", "S3_ACCESS_KEY_ID": "key", "S3_SECRET_ACCESS_KEY": "secret", "BLOB_DELIVERY": "proxy", "BLOB_CACHE_MAX_BYTES": "1234"}
	cfg, err := ParseEnv(lookup(env), "/default")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Driver != "s3" || cfg.S3.Region != "garage" || !cfg.S3.ForcePathStyle || !cfg.S3.AllowPlainHTTP || cfg.S3.PresignEndpoint != "https://files.example.com" || cfg.Delivery != "proxy" || cfg.Cache.MaxBytes != 1234 {
		t.Fatalf("unexpected configuration: %#v", cfg)
	}
	env["S3_PROVIDER"] = "do"
	env["S3_REGION"] = "nyc3"
	cfg, err = ParseEnv(lookup(env), "")
	if err != nil || cfg.Provider != "spaces" {
		t.Fatalf("provider normalization: %q %v", cfg.Provider, err)
	}
	cfg, err = ParseEnv(lookup(nil), "/default")
	if err != nil || cfg.LocalRoot != "/default" || cfg.Delivery != "" {
		t.Fatalf("app defaults: %#v %v", cfg, err)
	}
}

func TestParseEnvRejectsInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct{ key, value, want string }{
		{"STORAGE_DRIVER", "disk", "local|s3"}, {"S3_FORCE_PATH_STYLE", "yes", "true|false"}, {"S3_ALLOW_INSECURE_ENDPOINT", "yes", "true|false"}, {"BLOB_DELIVERY", "public", "redirect hoặc proxy"}, {"S3_REGION", "", "yêu cầu S3_REGION"}, {"S3_SECRET_ACCESS_KEY", "", "yêu cầu S3_REGION"}, {"BLOB_CACHE_MAX_BYTES", "0", "positive byte count"},
	} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			env := map[string]string{"STORAGE_DRIVER": "s3", "S3_REGION": "garage", "S3_BUCKET": "media", "S3_ACCESS_KEY_ID": "key", "S3_SECRET_ACCESS_KEY": "secret"}
			env[tc.key] = tc.value
			_, err := ParseEnv(lookup(env), "")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

func TestEnvLockIgnoresCacheAndEncryption(t *testing.T) {
	if EnvLocked(lookup(map[string]string{"BLOB_CACHE_DIR": "/cache", "CONFIG_ENCRYPTION_KEY": "bad"})) {
		t.Fatal("cache and encryption must not lock transport")
	}
	if !EnvLocked(lookup(map[string]string{"S3_BUCKET": ""})) {
		t.Fatal("explicit empty infrastructure env must lock")
	}
	if !EnvVarLocks("S3_FUTURE_OPTION") || EnvVarLocks("BLOB_CACHE_MAX_BYTES") {
		t.Fatal("lock name predicate drift")
	}
}
