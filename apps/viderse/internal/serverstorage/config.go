package serverstorage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

func normalizeConfig(c S3Config, old *blobstore.Config) (blobstore.Config, error) {
	c.Provider = strings.TrimSpace(c.Provider)
	switch c.Provider {
	case "garage", "r2", "aws", "gcs", "wasabi", "b2", "do", "other":
	default:
		return blobstore.Config{}, errors.New("Nhà cung cấp kho không hợp lệ")
	}
	c.Endpoint = strings.TrimSpace(c.Endpoint)
	c.Region = strings.TrimSpace(c.Region)
	c.Bucket = strings.TrimSpace(c.Bucket)
	c.PresignEndpoint = strings.TrimSpace(c.PresignEndpoint)
	if c.Provider == "r2" && c.Endpoint == "" {
		if c.AccountID == "" || strings.ContainsAny(c.AccountID, "./:@?# ") {
			return blobstore.Config{}, errors.New("Mã tài khoản R2 không hợp lệ")
		}
		c.Endpoint = "https://" + c.AccountID + ".r2.cloudflarestorage.com"
	}
	provider := c.Provider
	if provider == "do" {
		provider = "spaces"
	}
	s3 := blobstore.ProviderDefaults(provider, blobstore.S3Config{Endpoint: c.Endpoint, PresignEndpoint: c.PresignEndpoint, Region: c.Region, Bucket: c.Bucket, AccessKeyID: c.AccessKeyID, SecretAccessKey: c.SecretAccessKey, ForcePathStyle: c.ForcePathStyle, AllowPlainHTTP: c.AllowInsecure})
	cfg := blobstore.Config{Driver: "s3", Provider: provider, Delivery: c.Delivery, S3: s3}
	if c.Delivery != "proxy" && c.Delivery != "redirect" {
		return cfg, errors.New("Cách giao tệp không hợp lệ")
	}
	if (c.AccessKeyID == "") != (c.SecretAccessKey == "") {
		return cfg, errors.New("Cần nhập đầy đủ cặp khoá truy cập")
	}
	if c.AccessKeyID == "" && old != nil && old.Driver == "s3" && media.NamespaceID(cfg) == media.NamespaceID(*old) {
		cfg.S3.AccessKeyID = old.S3.AccessKeyID
		cfg.S3.SecretAccessKey = old.S3.SecretAccessKey
	}
	if _, err := blobstore.NewS3(cfg.S3); err != nil {
		return cfg, errors.New("Cấu hình kho chưa hợp lệ; kiểm tra địa chỉ, vùng và cặp khoá truy cập")
	}
	return cfg, nil
}
func configFingerprint(cfg blobstore.Config) string {
	// Includes credential identity and every delivery/safety field; it is never
	// returned or logged, and the process-local test session expires on restart.
	raw, _ := json.Marshal(struct {
		Driver, Provider, Delivery, Root string
		S3                               blobstore.S3Config
	}{cfg.Driver, cfg.Provider, cfg.Delivery, cfg.LocalRoot, cfg.S3})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func providerWire(provider string) string {
	if provider == "spaces" {
		return "do"
	}
	return provider
}
func storeLabel(d media.Descriptor) StoreLabel {
	if d.Config.Driver == "local" {
		return StoreLabel{Kind: "disk", Label: "Thư mục trên máy chủ"}
	}
	return StoreLabel{Kind: "s3", Label: d.Config.S3.Bucket}
}
func endpointHost(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return u.Host
}
