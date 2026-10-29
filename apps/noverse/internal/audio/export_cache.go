package audio

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// 导出的整章中间产物缓存（Gói AE · 06-data-contract §AE）：
// 每章的 48 kHz wav（与可选的成品编码副本）按"输入指纹"缓存在
// audio/export-cache/tap-NN/。段音频按文件大小+修改时间参与指纹——同一缓存键
// 重新生成过（文件变了）也会换指纹，不会用过期音频拼卷。
// 指纹变 → 该章重拼；其它章原样复用，重跑导出只剩 concat/encode。
// 磁盘占用有界：每次成功 export 只保留当前这次用到的键。

// exportCacheVersion 是中间产物管线的版本号：拼章算法/参数变化时 +1。
const exportCacheVersion = 1

const exportCacheSubdir = "export-cache"

// exportCacheManifest 是 tap-NN 目录里的 manifest.json。
type exportCacheManifest struct {
	Chapters map[int]*exportCacheEntry `json:"chapters"`
}

// exportCacheEntry 记录一章可用的中间产物。
type exportCacheEntry struct {
	WavKey  string    `json:"wavKey"`           // 段 + 章名 + 停顿 + 管线版本
	EncKey  string    `json:"encKey"`           // wavKey + 格式 + 响度
	Wav     string    `json:"wav"`              // export-cache 内的文件名
	Enc     string    `json:"enc,omitempty"`    // 成品编码副本（mp3 等）
	EncFmt  string    `json:"encFmt,omitempty"` // Enc 的格式
	Dur     float64   `json:"dur"`
	BuiltAt time.Time `json:"builtAt"`
}

func (s *Service) exportCacheDir(volume int) string {
	return filepath.Join(s.layout.AudioDir, exportCacheSubdir, fmt.Sprintf("tap-%02d", volume))
}

func (s *Service) readExportCache(volume int) *exportCacheManifest {
	data, err := os.ReadFile(filepath.Join(s.exportCacheDir(volume), "manifest.json"))
	if err != nil {
		return &exportCacheManifest{Chapters: map[int]*exportCacheEntry{}}
	}
	var m exportCacheManifest
	if json.Unmarshal(data, &m) != nil || m.Chapters == nil {
		return &exportCacheManifest{Chapters: map[int]*exportCacheEntry{}}
	}
	return &m
}

func (s *Service) saveExportCache(volume int, m *exportCacheManifest) error {
	dir := s.exportCacheDir(volume)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(dir, "manifest.json"), data)
}

// exportWavKey 是一章 wav 的输入指纹：段顺序 + 段音频（键 + 文件大小/修改时间）+
// 段间停顿 + 章名朗读（文本 + 音频文件）+ 管线版本。
func (s *Service) exportWavKey(volume int, sc SourceChapter, script *Script, cast *Cast, titlePath string) string {
	h := sha256.New()
	title := ""
	if titlePath != "" {
		title = sc.Title
	}
	fmt.Fprintf(h, "v%d\x00%d\x00%s\x00%s\n", exportCacheVersion, sc.Chapter, title, ffmpegBin)
	for _, seg := range script.Segments {
		key, dur := "", 0.0
		if seg.Rendered != nil {
			key, dur = seg.Rendered.Key, seg.Rendered.DurationSec
		}
		fmt.Fprintf(h, "%s\x00%s\x00%.3f\x00%d", seg.ID, key, dur, effectivePauseMs(cast, seg))
		if st, err := os.Stat(s.layout.CachePath(volume, sc.Chapter, key)); err == nil {
			fmt.Fprintf(h, "\x00%d\x00%d", st.Size(), st.ModTime().UnixNano())
		}
		fmt.Fprintf(h, "\n")
	}
	if titlePath != "" {
		fmt.Fprintf(h, "title\x00%s", titlePath)
		if st, err := os.Stat(titlePath); err == nil {
			fmt.Fprintf(h, "\x00%d\x00%d", st.Size(), st.ModTime().UnixNano())
		}
		fmt.Fprintf(h, "\n")
	}
	return hex.EncodeToString(h.Sum(nil))
}

// exportEncKey 在 wav 指纹上叠加影响成品编码的选项（格式、响度、采样率策略）
// 与标签指纹（Gói AF：换封面/规则 → 重新编码重贴标签）。
func exportEncKey(wavKey, format string, loudness bool, tagFP string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%t\x00%s\x00%d", wavKey, format, loudness, tagFP, exportCacheVersion)
	return hex.EncodeToString(h.Sum(nil))
}

// lookupExportChapter 返回一章可复用的 wav/编码副本；miss 返回 nil。
func (m *exportCacheManifest) lookup(volume, chapter int, dir, wavKey, encKey, encFmt string) *exportCacheEntry {
	e, ok := m.Chapters[chapter]
	if !ok || e.WavKey != wavKey {
		return nil
	}
	if _, err := os.Stat(filepath.Join(dir, e.Wav)); err != nil {
		return nil
	}
	if e.Enc != "" {
		if e.EncKey != encKey || e.EncFmt != encFmt {
			e = &exportCacheEntry{WavKey: e.WavKey, Wav: e.Wav, Dur: e.Dur, BuiltAt: e.BuiltAt}
			return e
		}
		if _, err := os.Stat(filepath.Join(dir, e.Enc)); err != nil {
			e.Enc = ""
		}
	}
	return e
}

// storeExportChapter 记录/更新一章的缓存条目。
func (m *exportCacheManifest) store(chapter int, e *exportCacheEntry) {
	if m.Chapters == nil {
		m.Chapters = map[int]*exportCacheEntry{}
	}
	m.Chapters[chapter] = e
}

// pruneExportCache 只保留本次 export 用到的章节与文件（避免无界增长）。
func (s *Service) pruneExportCache(volume int, keep *exportCacheManifest) {
	dir := s.exportCacheDir(volume)
	keepFiles := map[string]bool{"manifest.json": true, "results.json": true}
	for _, e := range keep.Chapters {
		keepFiles[e.Wav] = true
		if e.Enc != "" {
			keepFiles[e.Enc] = true
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, ent := range entries {
		if ent.IsDir() || keepFiles[ent.Name()] {
			continue
		}
		_ = os.Remove(filepath.Join(dir, ent.Name()))
	}
	// 顺带清掉旧版缓存目录残留（没有 manifest 的孤立文件由上面删掉）。
	_ = s.saveExportCache(volume, keep)
}

// exportCacheFileName 是缓存里的稳定文件名：<ch>-<key前缀>.wav。
func exportCacheFileName(chapter int, key, ext string) string {
	short := key
	if len(short) > 16 {
		short = short[:16]
	}
	return fmt.Sprintf("%02d-%s.%s", chapter, short, ext)
}
