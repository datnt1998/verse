package audio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
)

// 项目的"giọng của dự án"：<assets>/giong/*.wav + giong.json（10-audio §3）。
// 这是"mẫu giọng"（10–30 秒清晰录音），不是引擎的预设声音；两种声音在
// GET /api/projects/:p/voices 里合并返回，用 builtIn 区分。

// 声音样本时长限制（10-audio §2：10–30 giây）。
const (
	MinVoiceSec = 10.0
	MaxVoiceSec = 30.0
)

// ProjectVoice 对应 GET /api/projects/:p/voices 的一项。
type ProjectVoice struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	File        string  `json:"file"` // 相对 <project>：tai-san/giong/xxx.wav
	DurationSec float64 `json:"durationSec"`
	Gender      string  `json:"gender"` // nam | nu | ""
	BuiltIn     bool    `json:"builtIn"`
	Note        string  `json:"note,omitempty"`
	SampleURL   string  `json:"sampleUrl,omitempty"`
}

// voiceIndex 是 giong.json 的内容。
type voiceIndex struct {
	Voices []ProjectVoice `json:"voices"`
}

// ReadProjectVoices 读取项目声音列表；文件不存在返回空切片。
func ReadProjectVoices(l Layout) ([]ProjectVoice, error) {
	data, err := os.ReadFile(filepath.Join(l.VoicesDir(), voicesIndexFile))
	if errors.Is(err, os.ErrNotExist) {
		return []ProjectVoice{}, nil
	}
	if err != nil {
		return nil, err
	}
	var idx voiceIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("%s hỏng: %w", voicesIndexFile, err)
	}
	out := make([]ProjectVoice, 0, len(idx.Voices))
	for _, v := range idx.Voices {
		if v.ID == "" {
			continue
		}
		v.BuiltIn = false
		out = append(out, v)
	}
	return out, nil
}

// WriteProjectVoices 原子写 giong.json。
func WriteProjectVoices(l Layout, vs []ProjectVoice) error {
	if vs == nil {
		vs = []ProjectVoice{}
	}
	if err := os.MkdirAll(l.VoicesDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(voiceIndex{Voices: vs}, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(l.VoicesDir(), voicesIndexFile), data)
}

// VoiceMap 把列表变成 id → 声音，供 ResolveVoice 使用。
func VoiceMap(vs []ProjectVoice) map[string]ProjectVoice {
	m := make(map[string]ProjectVoice, len(vs))
	for _, v := range vs {
		m[v.ID] = v
	}
	return m
}

// ValidateVoiceClip 校验样本时长（10–30 秒）。返回越南语错误给界面直接用。
func ValidateVoiceClip(durationSec float64) error {
	if durationSec <= 0 {
		return errors.New("Không đọc được tệp ghi âm này. Hãy chọn tệp .mp3, .wav hoặc .m4a nghe được.")
	}
	if durationSec < MinVoiceSec {
		return fmt.Errorf("Mẫu giọng chỉ có %.0f giây — cần từ 10 đến 30 giây.", durationSec)
	}
	if durationSec > MaxVoiceSec {
		return fmt.Errorf("Mẫu giọng dài %.0f giây — cần từ 10 đến 30 giây.", durationSec)
	}
	return nil
}

// StoreVoice 把一个已经转成 wav 的样本放进 <assets>/giong，并更新 giong.json。
// wavPath 必须是最终文件（调用方负责转码）；返回入库后的条目。
func StoreVoice(l Layout, name, gender, wavPath string, durationSec float64) (ProjectVoice, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ProjectVoice{}, errors.New("thiếu tên giọng")
	}
	if err := ValidateVoiceClip(durationSec); err != nil {
		return ProjectVoice{}, err
	}
	dir := l.VoicesDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ProjectVoice{}, err
	}
	existing, err := ReadProjectVoices(l)
	if err != nil {
		return ProjectVoice{}, err
	}
	id := voiceSlug(name)
	used := map[string]bool{}
	for _, v := range existing {
		used[v.ID] = true
	}
	base := id
	for i := 2; used[id]; i++ {
		id = fmt.Sprintf("%s-%d", base, i)
	}
	fileName := id + ".wav"
	dest := filepath.Join(dir, fileName)
	if wavPath != dest {
		if err := os.Rename(wavPath, dest); err != nil {
			// 跨设备时退化为复制。
			data, rerr := os.ReadFile(wavPath)
			if rerr != nil {
				return ProjectVoice{}, rerr
			}
			if werr := writeFileAtomic(dest, data); werr != nil {
				return ProjectVoice{}, werr
			}
			_ = os.Remove(wavPath)
		}
	}
	rel, err := filepath.Rel(filepath.Dir(l.AssetsDir), dest)
	if err != nil {
		rel = dest
	}
	v := ProjectVoice{
		ID:          id,
		Name:        name,
		File:        filepath.ToSlash(rel),
		DurationSec: durationSec,
		Gender:      normalizeGender(gender),
	}
	existing = append(existing, v)
	if err := WriteProjectVoices(l, existing); err != nil {
		return ProjectVoice{}, err
	}
	return v, nil
}

// ConvertVoiceClip 把上传的录音转成引擎要的 48 kHz 单声道 wav。
// 上传可能是 mp3/m4a/立体声，统一转一次，后面克隆与合成都不用再想格式。
func ConvertVoiceClip(ctx context.Context, src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return toPCM(ctx, src, dest)
}

// DeleteVoice 删除一个项目声音（文件 + 索引）。引擎预设声音不能删。
func DeleteVoice(l Layout, id string) error {
	vs, err := ReadProjectVoices(l)
	if err != nil {
		return err
	}
	out := make([]ProjectVoice, 0, len(vs))
	var removed *ProjectVoice
	for _, v := range vs {
		if v.ID == id {
			cp := v
			removed = &cp
			continue
		}
		out = append(out, v)
	}
	if removed == nil {
		return library.ErrNotFound
	}
	if err := WriteProjectVoices(l, out); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(filepath.Dir(l.AssetsDir), filepath.FromSlash(removed.File)))
	return nil
}

func normalizeGender(g string) string {
	switch strings.ToLower(strings.TrimSpace(g)) {
	case "nam", "male", "m":
		return "nam"
	case "nu", "nữ", "female", "f":
		return "nu"
	}
	return ""
}

// voiceSlug 用名字做文件名（保留越南语字母，去掉路径危险字符）。
func voiceSlug(name string) string {
	slug := library.Slugify(name, 4)
	if slug == "" || slug == "truyen-moi" {
		slug = "giong"
	}
	return slug
}
