package audio

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 音频设置（audio/settings.json）与项目读音词典（<project>/.ainovel/cach-doc.json）。
//
// 设置是"truyện"级的：品质与"tự tạo khi chốt"影响整本的生成行为；
// 读音词典是"dự án"级的：同一部剧的角色名、口头禅在每个 truyện 里读法一致。

// 生成品质（对应引擎 precision：fast=int8、standard=fp32、high=fp32+khử ồn）。
const (
	QualityFast     = "fast"
	QualityStandard = "standard"
	QualityHigh     = "high"
)

// Settings 对应 GET/PUT S/audio/settings。
type Settings struct {
	Quality    string           `json:"quality"`
	AutoOnLock bool             `json:"autoOnLock"`
	Normalize  NormalizeOptions `json:"normalize"`
}

// DefaultSettings 是缺省：Chuẩn + 锁定即生成 + 全部规范化打开。
func DefaultSettings() Settings {
	return Settings{Quality: QualityStandard, AutoOnLock: true, Normalize: DefaultNormalizeOptions()}
}

// ValidateQuality 校验品质取值。
func ValidateQuality(q string) bool {
	return q == QualityFast || q == QualityStandard || q == QualityHigh
}

// ReadSettings 读取设置；文件不存在返回默认值。坏文件报错（用户主动放的配置不该被静默吞掉）。
func ReadSettings(l Layout) (Settings, error) {
	def := DefaultSettings()
	data, err := os.ReadFile(l.settingsPath())
	if errors.Is(err, os.ErrNotExist) {
		return def, nil
	}
	if err != nil {
		return def, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return def, fmt.Errorf("settings.json hỏng: %w", err)
	}
	if !ValidateQuality(s.Quality) {
		s.Quality = def.Quality
	}
	return s, nil
}

// WriteSettings 原子写设置。
func WriteSettings(l Layout, s Settings) error {
	if !ValidateQuality(s.Quality) {
		return conflict("quality phải là fast, standard hoặc high")
	}
	if err := os.MkdirAll(l.AudioDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(l.settingsPath(), data)
}

// pronunciationFile 是 cach-doc.json 的磁盘格式。
type pronunciationFile struct {
	Entries []Pronunciation `json:"entries"`
}

// 词典规模上限：再多就不是"读音词典"而是翻译表了，也防手滑粘贴整个文件。
const maxPronunciations = 500

// ReadPronunciations 读项目读音词典；不存在返回空切片。
func ReadPronunciations(l Layout) ([]Pronunciation, error) {
	data, err := os.ReadFile(l.PronouncePath)
	if errors.Is(err, os.ErrNotExist) {
		return []Pronunciation{}, nil
	}
	if err != nil {
		return nil, err
	}
	var f pronunciationFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("cach-doc.json hỏng: %w", err)
	}
	if f.Entries == nil {
		return []Pronunciation{}, nil
	}
	return f.Entries, nil
}

// NormalizePronunciations 校验并清洗词典：去空白、去重（后者覆盖前者）、限长。
func NormalizePronunciations(in []Pronunciation) ([]Pronunciation, error) {
	if len(in) > maxPronunciations {
		return nil, conflict("từ điển cách đọc tối đa %d mục", maxPronunciations)
	}
	seen := map[string]int{}
	out := make([]Pronunciation, 0, len(in))
	for _, e := range in {
		w := strings.TrimSpace(e.Written)
		s := strings.TrimSpace(e.Spoken)
		if w == "" && s == "" {
			continue
		}
		if w == "" || s == "" {
			return nil, conflict("mỗi mục cách đọc cần cả chữ viết và chữ đọc")
		}
		if len([]rune(w)) > 100 || len([]rune(s)) > 200 {
			return nil, conflict("mục cách đọc quá dài: %q", w)
		}
		if i, ok := seen[strings.ToLower(w)]; ok {
			out[i] = Pronunciation{Written: w, Spoken: s}
			continue
		}
		seen[strings.ToLower(w)] = len(out)
		out = append(out, Pronunciation{Written: w, Spoken: s})
	}
	return out, nil
}

// WritePronunciations 原子写词典。
func WritePronunciations(l Layout, entries []Pronunciation) error {
	clean, err := NormalizePronunciations(entries)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(l.PronouncePath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(pronunciationFile{Entries: clean}, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(l.PronouncePath, data)
}
