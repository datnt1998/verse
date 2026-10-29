package audio

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
)

// 整章播放文件：把已生成的段按顺序 + 段间停顿拼成一个 mp3。
// 只有"每段都有音频"才拼（缺段时界面显示 12/46，不该给一个半截文件）。
// 拼好的文件带 manifest（段 key 列表的哈希），段一变就重建。

// ErrChapterIncomplete 表示本章还有段没生成音频（handler 映射 409）。
var ErrChapterIncomplete = errors.New("Chương này chưa có đủ giọng đọc.")

// chapterManifest 记录整章文件是由哪些段拼出来的。
type chapterManifest struct {
	Hash         string        `json:"hash"`
	DurationSec  float64       `json:"durationSec"`
	BuiltAt      time.Time     `json:"builtAt"`
	RenderedFrom *RenderSource `json:"renderedFrom,omitempty"`
}

func (s *Service) chapterManifestPath(volume, chapter int) string {
	return filepath.Join(s.chapterAudioDir(volume), fmt.Sprintf("%02d.json", chapter))
}

// effectivePauseMs 返回段后实际停顿：单声模式可"ngừng lâu hơn trước lời thoại"。
func effectivePauseMs(cast *Cast, seg Segment) int {
	pause := seg.PauseMs
	if cast != nil && cast.Mode == ModeSingle && seg.Speaker != NarratorID && cast.Single.DialoguePauseMs > 0 {
		pause += cast.Single.DialoguePauseMs
	}
	return pause
}

// chapterHash 是"本章音频内容"的指纹：段顺序 + 缓存键 + 时长 + 停顿。
func chapterHash(script *Script, cast *Cast) string {
	h := sha256.New()
	for _, seg := range script.Segments {
		key := ""
		dur := 0.0
		if seg.Rendered != nil {
			key = seg.Rendered.Key
			dur = seg.Rendered.DurationSec
		}
		fmt.Fprintf(h, "%s\x00%s\x00%.3f\x00%d\n", seg.ID, key, dur, effectivePauseMs(cast, seg))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ensureChapterAudio 确保整章 mp3 存在且是最新的，返回文件路径。
func (s *Service) ensureChapterAudio(ctx context.Context, volume, chapter int, script *Script) (string, error) {
	if script == nil {
		var err error
		script, err = ReadScript(s.layout, volume, chapter)
		if err != nil {
			return "", err
		}
	}
	if script == nil || len(script.Segments) == 0 {
		return "", ErrNotFound
	}
	cast, err := s.castForRender()
	if err != nil {
		return "", err
	}
	for _, seg := range script.Segments {
		if seg.Rendered == nil || seg.Rendered.DurationSec <= 0 {
			return "", ErrChapterIncomplete
		}
	}
	hash := chapterHash(script, cast)
	out := s.layout.ChapterAudioPath(volume, chapter, "mp3")
	if manifest, err := s.readChapterManifest(volume, chapter); err == nil && manifest != nil {
		if manifest.Hash == hash {
			if _, statErr := os.Stat(out); statErr == nil {
				return out, nil
			}
		}
	}
	s.buildMu.Lock()
	defer s.buildMu.Unlock()
	// 双检：另一个请求可能刚拼完。
	if manifest, err := s.readChapterManifest(volume, chapter); err == nil && manifest != nil {
		if manifest.Hash == hash {
			if _, statErr := os.Stat(out); statErr == nil {
				return out, nil
			}
		}
	}
	if !FFmpegAvailable() {
		return "", ErrFFmpegMissing
	}
	duration, err := s.buildChapterAudio(ctx, volume, chapter, script, cast, out)
	if err != nil {
		return "", err
	}
	manifest := chapterManifest{Hash: hash, DurationSec: duration, BuiltAt: time.Now(), RenderedFrom: script.RenderedFrom}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	if err := writeFileAtomic(s.chapterManifestPath(volume, chapter), data); err != nil {
		return "", err
	}
	return out, nil
}

// buildChapterAudio 拼接段 + 静音并编码 mp3。
func (s *Service) buildChapterAudio(ctx context.Context, volume, chapter int, script *Script, cast *Cast, out string) (float64, error) {
	dir, err := os.MkdirTemp("", "ainovel-chapter-*")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	inputs := make([]string, 0, len(script.Segments)*2)
	total := 0.0
	for i, seg := range script.Segments {
		src := s.layout.CachePath(volume, chapter, seg.Rendered.Key)
		pcm := filepath.Join(dir, fmt.Sprintf("s%04d.wav", i))
		if err := toPCM(ctx, src, pcm); err != nil {
			return 0, err
		}
		inputs = append(inputs, pcm)
		total += seg.Rendered.DurationSec
		pause := effectivePauseMs(cast, seg)
		if pause > 0 {
			sil := filepath.Join(dir, fmt.Sprintf("p%04d.wav", i))
			if err := writeSilenceWAV(sil, pause); err != nil {
				return 0, err
			}
			inputs = append(inputs, sil)
			total += float64(pause) / 1000
		}
	}
	wav := filepath.Join(dir, "chapter.wav")
	if err := concatWAVs(ctx, inputs, wav); err != nil {
		return 0, err
	}
	storyTitle, _ := s.storyMeta("")
	album := fmt.Sprintf("%s · Tập %d", storyTitle, volume)
	if storyTitle == "" {
		album = fmt.Sprintf("Tập %d", volume)
	}
	if err := encodeAudioTagged(ctx, wav, out, "mp3", false, tagSpec{tags: tagSet{
		Title:   fmt.Sprintf("Chương %d", chapter),
		Artist:  ArtistAudio,
		Album:   album,
		Disc:    volume,
		Genre:   genreAudiobook,
		Year:    time.Now().Year(),
		Comment: commentAudio,
		Lang:    languageAudio,
	}}, ""); err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Service) readChapterManifest(volume, chapter int) (*chapterManifest, error) {
	data, err := os.ReadFile(s.chapterManifestPath(volume, chapter))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m chapterManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// invalidateChapterAudio 删掉整章文件与 manifest（段变了以后强制重建）。
func (s *Service) invalidateChapterAudio(volume, chapter int) error {
	if err := os.Remove(s.layout.ChapterAudioPath(volume, chapter, "mp3")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Remove(s.chapterManifestPath(volume, chapter)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// ChapterAudioFile 返回可直接播放的整章文件（必要时先拼）。
func (s *Service) ChapterAudioFile(ctx context.Context, volume, chapter int) (string, error) {
	return s.ensureChapterAudio(ctx, volume, chapter, nil)
}

// Peaks 返回整章波形的峰值（≈ 2 个点/秒）与总时长。
func (s *Service) Peaks(ctx context.Context, volume, chapter int) ([]float64, float64, error) {
	path, err := s.ChapterAudioFile(ctx, volume, chapter)
	if err != nil {
		return nil, 0, err
	}
	dir, err := os.MkdirTemp("", "ainovel-peaks-*")
	if err != nil {
		return nil, 0, err
	}
	defer os.RemoveAll(dir)
	wav := filepath.Join(dir, "peaks.wav")
	if err := toPCM(ctx, path, wav); err != nil {
		return nil, 0, err
	}
	data, err := os.ReadFile(wav)
	if err != nil {
		return nil, 0, err
	}
	peaks, err := WAVPeaks(data, 2)
	if err != nil {
		return nil, 0, err
	}
	return peaks, WAVDurationSec(data), nil
}

// ChapterDuration 返回整章时长（没有整章文件时用段时长累加）。
func (s *Service) ChapterDuration(volume, chapter int) (float64, error) {
	script, err := ReadScript(s.layout, volume, chapter)
	if err != nil {
		return 0, err
	}
	if script == nil {
		return 0, ErrNotFound
	}
	cast, err := s.castForRender()
	if err != nil {
		return 0, err
	}
	total := 0.0
	for _, seg := range script.Segments {
		if seg.Rendered == nil {
			return 0, ErrChapterIncomplete
		}
		total += seg.Rendered.DurationSec + float64(effectivePauseMs(cast, seg))/1000
	}
	return total, nil
}

// chapterFileSlug 生成导出用的章节文件名（"01-vai-cua-toi-la-do-canh"）。
func chapterFileSlug(chapter int, title string) string {
	slug := librarySlug(title)
	if slug == "" {
		return fmt.Sprintf("%02d", chapter)
	}
	return fmt.Sprintf("%02d-%s", chapter, slug)
}

// itoa 是小工具（避免每处 strconv 都写一遍）。
func itoa(n int) string { return strconv.Itoa(n) }

// librarySlug 把章节标题变成文件名（去声调、小写、前 6 个词，与 10-audio §3 示例一致）。
func librarySlug(title string) string {
	slug := library.Slugify(strings.TrimSpace(title), 6)
	if slug == "truyen-moi" {
		return ""
	}
	return slug
}
