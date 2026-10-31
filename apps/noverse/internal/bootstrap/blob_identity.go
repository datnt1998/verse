package bootstrap

import (
	"net/url"
	"path/filepath"
	"strings"
)

// SamePhysicalBlobStore ignores credentials, signing region, presign endpoint,
// delivery and provider labels: changing those does not move any object.
func SamePhysicalBlobStore(a, b BlobConfig) bool {
	if a.Driver != b.Driver {
		return false
	}
	if a.Driver == BlobDriverLocal {
		root := func(s string) string {
			if real, err := filepath.EvalSymlinks(s); err == nil {
				s = real
			}
			absolute, err := filepath.Abs(s)
			if err == nil {
				s = absolute
			}
			return filepath.Clean(s)
		}
		return root(a.LocalRoot) == root(b.LocalRoot)
	}
	endpoint := func(raw string) string {
		u, err := url.Parse(raw)
		if err != nil {
			return raw
		}
		u.Scheme = strings.ToLower(u.Scheme)
		u.Host = strings.ToLower(u.Host)
		if (u.Scheme == "http" && u.Port() == "80") || (u.Scheme == "https" && u.Port() == "443") {
			u.Host = u.Hostname()
		}
		u.Path = strings.TrimSuffix(u.Path, "/")
		return u.String()
	}
	return endpoint(a.S3.Endpoint) == endpoint(b.S3.Endpoint) && a.S3.Bucket == b.S3.Bucket
}
