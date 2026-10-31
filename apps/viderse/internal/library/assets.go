package library

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Gói hiệu ứng có sẵn (assets/): 12 hiệu ứng của yt-factory — 11 tệp trong
// assets/overlays/ (sinh bằng scripts/generate-overlay-pack.mts) và tệp tuyết
// của Mẫu chuẩn (assets/standard-template/snow-loop.mp4). Nhúng thẳng vào
// binary để `seedBuiltins` ghi ra thư mục dự án mà không cần mạng hay gói cài.
//
//go:embed assets/overlays/*.mp4 assets/overlays/manifest.json assets/standard-template/snow-loop.mp4
var builtinAssets embed.FS

// builtinEffect là một hiệu ứng có sẵn.
type builtinEffect struct {
	ID   string // rain, fog, snow…
	Name string // tên hiển thị tiếng Việt (không đuôi tệp)
	File string // đường dẫn trong assets/ (tương đối)
	SHA  string
	Size int64
}

// builtinManifest là hình của assets/overlays/manifest.json.
type builtinManifest struct {
	GeneratorVersion string `json:"generatorVersion"`
	Effects          []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		File   string `json:"file"`
		Size   int64  `json:"size"`
		SHA256 string `json:"sha256"`
	} `json:"effects"`
}

// BuiltinEffects trả 12 hiệu ứng có sẵn theo thứ tự manifest (tuyết trước).
func BuiltinEffects() ([]builtinEffect, error) {
	raw, err := builtinAssets.ReadFile("assets/overlays/manifest.json")
	if err != nil {
		return nil, fmt.Errorf("đọc manifest hiệu ứng: %w", err)
	}
	var manifest builtinManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("manifest hiệu ứng hỏng: %w", err)
	}
	out := make([]builtinEffect, 0, len(manifest.Effects))
	for _, effect := range manifest.Effects {
		out = append(out, builtinEffect{
			ID:   effect.ID,
			Name: strings.TrimSuffix(effect.Name, path.Ext(effect.Name)),
			File: effect.File,
			SHA:  effect.SHA256,
			Size: effect.Size,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("manifest hiệu ứng không có mục nào")
	}
	return out, nil
}

// builtinOverlayBytes đọc nội dung một hiệu ứng và kiểm nó khớp sha256 của
// manifest — tệp nhúng bị sửa là lỗi build/hỏng binary, phải nói rõ.
func builtinOverlayBytes(effect builtinEffect) ([]byte, error) {
	data, err := builtinAssets.ReadFile(path.Join("assets", effect.File))
	if err != nil {
		return nil, fmt.Errorf("đọc tệp hiệu ứng %s: %w", effect.File, err)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != effect.SHA {
		return nil, fmt.Errorf("tệp hiệu ứng %s đã thay đổi (sha256 %s, manifest %s)", effect.File, got, effect.SHA)
	}
	return data, nil
}

// BuiltinOverlayID trả id cố định của một hiệu ứng có sẵn (dùng trong recipe
// Mẫu chuẩn và để nhận ra hiệu ứng đã bị bỏ khỏi thư viện).
func BuiltinOverlayID(effect string) string { return "builtin:" + effect }

// builtinAssetFiles liệt kê tệp mp4 nhúng (dùng cho test đối chiếu manifest).
func builtinAssetFiles() ([]string, error) {
	var out []string
	err := fs.WalkDir(builtinAssets, "assets", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".mp4") {
			out = append(out, strings.TrimPrefix(p, "assets/"))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}
