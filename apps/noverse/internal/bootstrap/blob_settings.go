package bootstrap

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/jackc/pgx/v5"
)

var ErrStorageConflict = errors.New("cấu hình lưu trữ đã thay đổi; vui lòng tải lại")
var ErrStorageLocked = errors.New("Kho tệp do máy chủ đặt sẵn; hãy sửa cấu hình máy chủ để thay đổi")
var ErrStorageEncryption = errors.New("Máy chủ chưa có khoá mã hoá; chưa thể lưu khoá bí mật")

// StorageCandidate is the admin write contract. Nil credentials retain the saved values.
type StorageCandidate struct {
	Version         int64   `json:"version"`
	Current         bool    `json:"current,omitempty"`
	Driver          string  `json:"driver"`
	Provider        string  `json:"provider"`
	LocalRoot       string  `json:"localRoot,omitempty"`
	Endpoint        string  `json:"endpoint,omitempty"`
	PresignEndpoint string  `json:"presignEndpoint,omitempty"`
	Region          string  `json:"region,omitempty"`
	Bucket          string  `json:"bucket,omitempty"`
	PathStyle       bool    `json:"pathStyle"`
	AllowHTTP       bool    `json:"allowHTTP"`
	Delivery        string  `json:"delivery"`
	AccessKeyID     *string `json:"accessKeyId,omitempty"`
	Secret          *string `json:"secretAccessKey,omitempty"`
}

type configCipher struct {
	aead cipher.AEAD
	id   string
}

func encryptionFromEnv() (configCipher, error) {
	raw, exists := os.LookupEnv("CONFIG_ENCRYPTION_KEY")
	if !exists {
		return configCipher{}, nil
	}
	key, err := base64.StdEncoding.Strict().DecodeString(raw)
	if err != nil || len(key) != 32 || base64.StdEncoding.EncodeToString(key) != raw {
		return configCipher{}, errors.New("CONFIG_ENCRYPTION_KEY phải là base64 chuẩn của đúng 32 byte")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return configCipher{}, errors.New("không mở được khoá mã hoá cấu hình")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return configCipher{}, err
	}
	sum := sha256.Sum256(key)
	return configCipher{aead: aead, id: "aes256gcm-v1:" + hex.EncodeToString(sum[:8])}, nil
}

// ValidateConfigEncryptionKey is called even in file mode: malformed keys are startup errors.
func ValidateConfigEncryptionKey() error { _, err := encryptionFromEnv(); return err }

func BlobEnvLocked() bool {
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if name == "STORAGE_DRIVER" || name == "BLOB_LOCAL_ROOT" || name == "BLOB_DELIVERY" || strings.HasPrefix(name, "S3_") {
			return true
		}
	}
	return false
}

func privateEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if u.Scheme == "http" {
		return true
	}
	if ip != nil {
		return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()
	}
	return !strings.Contains(host, ".")
}

func normalizeBlobConfig(cfg BlobConfig) BlobConfig {
	if cfg.Provider == "" {
		cfg.Provider = string(cfg.Driver)
		if cfg.Driver == BlobDriverS3 {
			cfg.Provider = "other"
		}
	}
	if cfg.Delivery == "" {
		cfg.Delivery = "redirect"
		if cfg.Driver == BlobDriverLocal || privateEndpoint(cfg.S3.Endpoint) {
			cfg.Delivery = "proxy"
		}
	}
	return cfg
}

func candidateConfig(c StorageCandidate, old BlobConfig) (BlobConfig, error) {
	cfg := BlobConfig{Driver: BlobDriver(c.Driver), Provider: c.Provider, LocalRoot: c.LocalRoot, Delivery: c.Delivery}
	if cfg.Driver == BlobDriverLocal && cfg.LocalRoot == "" {
		fallback, err := defaultBlobConfig()
		if err != nil {
			return cfg, err
		}
		cfg.LocalRoot = fallback.LocalRoot
	}
	cfg.S3 = blobstore.S3Config{Endpoint: c.Endpoint, PresignEndpoint: c.PresignEndpoint, Region: c.Region, Bucket: c.Bucket, ForcePathStyle: c.PathStyle, AllowPlainHTTP: c.AllowHTTP}
	cfg.S3.AccessKeyID, cfg.S3.SecretAccessKey = old.S3.AccessKeyID, old.S3.SecretAccessKey
	if c.AccessKeyID != nil {
		cfg.S3.AccessKeyID = *c.AccessKeyID
	}
	if c.Secret != nil {
		cfg.S3.SecretAccessKey = *c.Secret
	}
	if cfg.Driver == BlobDriverLocal {
		cfg.S3 = blobstore.S3Config{}
	}
	if c.Provider == "garage" {
		if cfg.S3.Region == "" {
			cfg.S3.Region = "garage"
		}
		cfg.S3.ForcePathStyle = true
	}
	cfg = normalizeBlobConfig(cfg)
	if cfg.Delivery != "redirect" && cfg.Delivery != "proxy" {
		return cfg, errors.New("Cách giao tệp phải là chuyển hướng hoặc truyền qua app")
	}
	return cfg, nil
}

func (m *ManagedBlobs) saveConfig(ctx context.Context, tx pgx.Tx, cfg BlobConfig) (string, error) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	id := hex.EncodeToString(token[:])
	var encrypted []byte
	if cfg.S3.AccessKeyID != "" || cfg.S3.SecretAccessKey != "" {
		if m.cipher.aead == nil {
			return "", ErrStorageEncryption
		}
		creds, _ := json.Marshal([]string{cfg.S3.AccessKeyID, cfg.S3.SecretAccessKey})
		nonce := make([]byte, m.cipher.aead.NonceSize())
		if _, err := rand.Read(nonce); err != nil {
			return "", err
		}
		encrypted = m.cipher.aead.Seal(nonce, nonce, creds, []byte(id))
	}
	cfg.S3.AccessKeyID, cfg.S3.SecretAccessKey = "", ""
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `INSERT INTO blob_stores(id,config,credentials,key_id) VALUES($1,$2,$3,$4)`, id, data, encrypted, m.cipher.id)
	return id, err
}

func (m *ManagedBlobs) loadConfig(ctx context.Context, id string) (BlobConfig, error) {
	return m.loadConfigFrom(ctx, m.pool.Pool(), id)
}

func (m *ManagedBlobs) loadConfigFrom(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string) (BlobConfig, error) {
	var cfg BlobConfig
	var data, encrypted []byte
	var keyID string
	err := db.QueryRow(ctx, `SELECT config,credentials,key_id FROM blob_stores WHERE id=$1`, id).Scan(&data, &encrypted, &keyID)
	if err != nil {
		return cfg, err
	}
	if json.Unmarshal(data, &cfg) != nil {
		return cfg, errors.New("Cấu hình kho đã lưu không hợp lệ")
	}
	if len(encrypted) > 0 {
		if m.cipher.aead == nil {
			return cfg, ErrStorageEncryption
		}
		if keyID != m.cipher.id || len(encrypted) < m.cipher.aead.NonceSize() {
			return cfg, errors.New("Khoá mã hoá không khớp cấu hình đã lưu")
		}
		n := m.cipher.aead.NonceSize()
		plain, err := m.cipher.aead.Open(nil, encrypted[:n], encrypted[n:], []byte(id))
		if err != nil {
			return cfg, errors.New("Không giải mã được cấu hình kho đã lưu")
		}
		var creds []string
		if json.Unmarshal(plain, &creds) != nil || len(creds) != 2 {
			return cfg, errors.New("Thông tin khoá đã lưu không hợp lệ")
		}
		cfg.S3.AccessKeyID, cfg.S3.SecretAccessKey = creds[0], creds[1]
	}
	return normalizeBlobConfig(cfg), nil
}

// ResolveBlobConfig is shared by serving and every maintenance command.
func ResolveBlobConfig(ctx context.Context, pool *postgres.Pool) (BlobConfig, error) {
	crypt, err := encryptionFromEnv()
	if err != nil {
		return BlobConfig{}, err
	}
	if BlobEnvLocked() {
		cfg, err := BlobConfigFromEnv()
		return normalizeBlobConfig(cfg), err
	}
	var id *string
	if err := pool.Pool().QueryRow(ctx, `SELECT active_store FROM storage_settings WHERE singleton`).Scan(&id); err != nil {
		return BlobConfig{}, err
	}
	if id != nil {
		return (&ManagedBlobs{pool: pool, cipher: crypt}).loadConfig(ctx, *id)
	}
	return defaultBlobConfig()
}

func defaultBlobConfig() (BlobConfig, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return BlobConfig{}, errors.New("Không xác định được thư mục dữ liệu máy chủ")
	}
	root = filepath.Join(root, "noverse", "blobs")
	if os.Getenv("NOVERSE_CONTAINER") == "1" {
		root = "/var/lib/noverse/blobs"
	}
	return normalizeBlobConfig(BlobConfig{Driver: BlobDriverLocal, LocalRoot: root}), nil
}
