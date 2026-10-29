package sim

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/utils"
)

type scannedSource struct {
	domain.SimulationSource
	absPath string
	content string
}

func scanSources(root string) ([]scannedSource, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("Chưa có văn mẫu nào để mô phỏng văn phong")
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Chưa có văn mẫu nào để phân tích")
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("Văn mẫu phải nằm trong một thư mục")
	}

	var out []scannedSource
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !isSupportedSource(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		sum := sha256.Sum256(data)
		sha := hex.EncodeToString(sum[:])
		out = append(out, scannedSource{
			SimulationSource: domain.SimulationSource{
				RelativePath: rel,
				SHA256:       sha,
				Fingerprint:  domain.SimulationSourceFingerprint(rel, sha),
				SizeBytes:    info.Size(),
				ModTime:      info.ModTime().Format(time.RFC3339),
			},
			absPath: path,
			// 指纹算在原始字节上（文件身份，增量去重稳定）；content 解码后供
			// LLM 分析——GBK 语料直接当 UTF-8 读是乱码，画像会被静默喂垃圾。
			content: utils.DecodeText(data),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].RelativePath < out[j].RelativePath
	})
	return out, nil
}

func isSupportedSource(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".txt", ".md", ".markdown":
		return true
	default:
		return false
	}
}

// sourceDisplayName 是 văn mẫu 在界面上显示的名字：只取文件名、去掉扩展名，
// 不带任何目录层级（Invariant #7：界面不出现路径/文件名）。
func sourceDisplayName(rel string) string {
	base := filepath.Base(filepath.FromSlash(rel))
	return strings.TrimSuffix(base, filepath.Ext(base))
}
