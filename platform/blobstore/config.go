package blobstore

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// LookupEnv isolates transport configuration from the process environment.
type LookupEnv func(string) (string, bool)

// Config contains byte transport configuration only. Apps own local-root defaults,
// saved settings, encryption, authorization and business error translation.
type Config struct {
	Driver    string
	Delivery  string
	Provider  string
	LocalRoot string
	S3        S3Config
	Cache     CacheConfig
}

type CacheConfig struct {
	Dir      string
	MaxBytes int64
}

const DefaultCacheMaxBytes int64 = 2 << 30

// ParseCacheEnv parses the working-cache budget without selecting a storage driver.
// The caller supplies its application-specific default directory.
func ParseCacheEnv(lookup LookupEnv, defaultDir string) (CacheConfig, error) {
	root, _ := lookup("BLOB_CACHE_DIR")
	root = strings.TrimSpace(root)
	if root == "" {
		root = defaultDir
	}
	var err error
	if root != "" {
		root, err = filepath.Abs(root)
		if err != nil {
			return CacheConfig{}, err
		}
	}
	cfg := CacheConfig{Dir: root, MaxBytes: DefaultCacheMaxBytes}
	if raw, _ := lookup("BLOB_CACHE_MAX_BYTES"); strings.TrimSpace(raw) != "" {
		cfg.MaxBytes, err = strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil || cfg.MaxBytes <= 0 {
			return CacheConfig{}, errors.New("BLOB_CACHE_MAX_BYTES must be a positive byte count")
		}
	}
	return cfg, nil
}

// ParseEnv preserves the local|s3 wire contract and leaves delivery defaults to
// the application. Endpoint validation is performed by Open/NewS3, without I/O.
func ParseEnv(lookup LookupEnv, defaultLocalRoot string) (Config, error) {
	get := func(key string) string { v, _ := lookup(key); return strings.TrimSpace(v) }
	cfg := Config{Driver: "local"}
	switch d := get("STORAGE_DRIVER"); d {
	case "", "local":
	case "s3":
		cfg.Driver = "s3"
	default:
		return cfg, fmt.Errorf("STORAGE_DRIVER=%q không hợp lệ (local|s3)", d)
	}
	insecure := false
	if v := get("S3_ALLOW_INSECURE_ENDPOINT"); v != "" {
		b, err := parseBool("S3_ALLOW_INSECURE_ENDPOINT", v)
		if err != nil {
			return cfg, err
		}
		insecure = b
	}
	cfg.Delivery = get("BLOB_DELIVERY")
	if cfg.Delivery != "" && cfg.Delivery != "redirect" && cfg.Delivery != "proxy" {
		return cfg, errors.New("BLOB_DELIVERY phải là redirect hoặc proxy")
	}
	cfg.Provider = NormalizeProvider(get("S3_PROVIDER"))
	if cfg.Driver == "local" {
		cfg.LocalRoot = get("BLOB_LOCAL_ROOT")
		if cfg.LocalRoot == "" {
			cfg.LocalRoot = defaultLocalRoot
		}
		if cfg.LocalRoot == "" {
			return cfg, errors.New("STORAGE_DRIVER=local yêu cầu BLOB_LOCAL_ROOT")
		}
	} else {
		cfg.S3 = S3Config{Endpoint: get("S3_ENDPOINT"), PresignEndpoint: get("S3_PRESIGN_ENDPOINT"), Region: get("S3_REGION"), Bucket: get("S3_BUCKET"), AccessKeyID: get("S3_ACCESS_KEY_ID"), SecretAccessKey: get("S3_SECRET_ACCESS_KEY"), AllowPlainHTTP: insecure}
		if v := get("S3_FORCE_PATH_STYLE"); v != "" {
			b, err := parseBool("S3_FORCE_PATH_STYLE", v)
			if err != nil {
				return cfg, err
			}
			cfg.S3.ForcePathStyle = b
		}
		cfg.S3 = ProviderDefaults(cfg.Provider, cfg.S3)
		if cfg.S3.Region == "" || cfg.S3.Bucket == "" || cfg.S3.AccessKeyID == "" || cfg.S3.SecretAccessKey == "" {
			return cfg, errors.New("STORAGE_DRIVER=s3 yêu cầu S3_REGION, S3_BUCKET, S3_ACCESS_KEY_ID, S3_SECRET_ACCESS_KEY")
		}
	}
	var err error
	cfg.Cache, err = ParseCacheEnv(lookup, "")
	return cfg, err
}

func parseBool(name, v string) (bool, error) {
	switch v {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	}
	return false, fmt.Errorf("%s=%q không hợp lệ (true|false)", name, v)
}

// NormalizeProvider maps the public DigitalOcean preset to its transport name.
func NormalizeProvider(provider string) string {
	if provider == "do" {
		return "spaces"
	}
	return provider
}

func ProviderDefaults(provider string, cfg S3Config) S3Config {
	if provider == "garage" {
		if cfg.Region == "" {
			cfg.Region = "garage"
		}
		cfg.ForcePathStyle = true
	}
	return cfg
}

// EnvVarLocks applies to all S3_* variables, including future provider options.
// Cache settings do not lock the app's provider control plane.
func EnvVarLocks(name string) bool {
	return name == "STORAGE_DRIVER" || name == "BLOB_LOCAL_ROOT" || name == "BLOB_DELIVERY" || strings.HasPrefix(name, "S3_")
}

// EnvLocked checks the complete known transport contract using presence, not value.
// Apps enumerating process variables should use EnvVarLocks for future S3_* keys.
func EnvLocked(lookup LookupEnv) bool {
	for _, key := range []string{"STORAGE_DRIVER", "BLOB_LOCAL_ROOT", "BLOB_DELIVERY", "S3_ENDPOINT", "S3_REGION", "S3_BUCKET", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY", "S3_FORCE_PATH_STYLE", "S3_ALLOW_INSECURE_ENDPOINT", "S3_PROVIDER", "S3_PRESIGN_ENDPOINT"} {
		if _, ok := lookup(key); ok {
			return true
		}
	}
	return false
}

// Open constructs exactly the requested driver. Invalid configuration never falls back.
func Open(cfg Config) (BlobStore, error) {
	switch cfg.Driver {
	case "local":
		st, err := NewLocal(cfg.LocalRoot)
		if err != nil {
			return nil, err
		}
		return st, nil
	case "s3":
		st, err := NewS3(cfg.S3)
		if err != nil {
			return nil, err
		}
		return st, nil
	default:
		return nil, fmt.Errorf("STORAGE_DRIVER=%q không được hỗ trợ", cfg.Driver)
	}
}
