package audio

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

// speakable 判断一段里有没有可读出的字（字母或数字）。纯标点段（"," "." "—"）
// 交给 TTS 会合成出 0 秒音频：这一段永远算"没有音频"，整章凑不齐、拼不出也导不出，
// 重新生成又命中同一份 0 秒缓存——用户没有任何出口。
func speakable(text string) bool {
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return true
		}
	}
	return false
}

// foldUnspeakable 把纯标点段并进前一段（位于开头的并进后一段），停顿取较长者。
// 接收方文字变了，已生成的音频作废（Rendered 清空），只需重读这一段。
// 全是标点时返回空切片，由调用方报错。
func foldUnspeakable(segs []Segment) []Segment {
	out := make([]Segment, 0, len(segs))
	var lead *Segment
	for _, seg := range segs {
		if !speakable(seg.Text) {
			if len(out) == 0 {
				if lead == nil {
					l := seg
					lead = &l
				} else {
					lead.Text = joinPunctuation(lead.Text, seg.Text)
					lead.Source = joinPunctuation(lead.Source, seg.Source)
					lead.PauseMs = max(lead.PauseMs, seg.PauseMs)
				}
				continue
			}
			prev := &out[len(out)-1]
			prev.Text = joinPunctuation(prev.Text, seg.Text)
			prev.Source = joinPunctuation(prev.Source, seg.Source)
			prev.PauseMs = max(prev.PauseMs, seg.PauseMs)
			prev.Rendered = nil
			continue
		}
		if lead != nil {
			seg.Text = joinPunctuation(lead.Text, seg.Text)
			seg.Source = joinPunctuation(lead.Source, seg.Source)
			seg.PauseMs = max(lead.PauseMs, seg.PauseMs)
			seg.Rendered = nil
			lead = nil
		}
		out = append(out, seg)
	}
	return out
}

// joinPunctuation 拼接两段文字：标点前、开引号/开括号后不加空格。
func joinPunctuation(a, b string) string {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a == "" || b == "" {
		return a + b
	}
	if strings.ContainsRune(",.;:!?…)]}”’»", []rune(b)[0]) || strings.ContainsRune("([{“‘«", []rune(a)[len([]rune(a))-1]) {
		return a + b
	}
	return a + " " + b
}

// foldScriptUnspeakable 就地修复已存盘、早于合并规则的 bản đọc。
func foldScriptUnspeakable(script *Script) error {
	folded := foldUnspeakable(script.Segments)
	if len(folded) == 0 {
		return conflict("Bản đọc chương %d chỉ có dấu câu, không có chữ để đọc.", script.Chapter)
	}
	// Capture before replacing segments: folding changes audible text/pauses and
	// clears the recipient's old audio, but not the chapter source.
	script.captureLegacyRenderSource()
	script.Segments = folded
	return nil
}

// 章节 bản đọc的四种展示状态（10-audio §2 A1/A2）。
const (
	StatusTodo   = "todo"   // 尚未 soạn
	StatusDraft  = "draft"  // AI đang soạn
	StatusReview = "review" // 已 soạn，等人工 duyệt
	StatusLocked = "locked" // đã chốt
	StatusStale  = "stale"  // 锁定后正文又被改过
)

// 段落级"需要人看一眼"的标记（10-audio §2 A1）。
const (
	FlagLongSentence   = "long_sentence"
	FlagUnknownSpeaker = "unknown_speaker"
	FlagEnglish        = "english"
	FlagNoVoice        = "no_voice"
)

// LongSentenceWords 是"句子太长建议断开"的词数阈值（artboard: "Câu dài 41 từ"）。
const LongSentenceWords = 35

// Segment 是 bản đọc 里的一段（10-audio §3 schema）。
type Segment struct {
	ID           string        `json:"id"`
	Speaker      string        `json:"speaker"` // 说话人 id（"dan" = 叙述者）
	Text         string        `json:"text"`    // 已规范化、可直接朗读
	Source       string        `json:"source,omitempty"`
	PauseMs      int           `json:"pauseMs"`
	Tone         string        `json:"tone"`
	Normalized   []Change      `json:"normalized,omitempty"`
	SpeakerGuess []SpeakerPick `json:"speakerGuess,omitempty"`
	Flags        []string      `json:"flags,omitempty"`
	// Rendered 记录"这段已生成过的音频"，只在服务端维护。
	Rendered *SegmentAudio `json:"rendered,omitempty"`
}

// SpeakerPick 是模型给出的"可能是谁在说 + 概率"（10-audio §3 speakerGuess）。
type SpeakerPick struct {
	Who string  `json:"who"`
	P   float64 `json:"p"`
}

// SegmentAudio 是一段已生成音频的元信息；Key 与当前参数不符即"cần đọc lại"。
type SegmentAudio struct {
	Key         string     `json:"key"`
	VoiceID     string     `json:"voiceId"`
	Tone        string     `json:"tone"`
	Speed       float64    `json:"speed"`
	Pitch       float64    `json:"pitch"`
	DurationSec float64    `json:"durationSec"`
	Engine      string     `json:"engine,omitempty"`
	Model       string     `json:"model,omitempty"`
	Quality     string     `json:"quality,omitempty"`
	RenderedAt  *time.Time `json:"renderedAt"`
	Rerendered  bool       `json:"rerendered,omitempty"`
}

// RenderSource records the inputs of the last completed chapter render.
type RenderSource struct {
	ScriptHash string `json:"scriptHash"`
	CastHash   string `json:"castHash,omitempty"`
	Quality    string `json:"quality,omitempty"`
}

// Script 是一章的 bản đọc（落盘 <audio>/ban-doc/tap-NN/CC.json）。
type Script struct {
	Chapter      int           `json:"chapter"`
	Volume       int           `json:"volume"`
	Status       string        `json:"status"` // review | locked
	LockedAt     *time.Time    `json:"lockedAt,omitempty"`
	SourceHash   string        `json:"sourceHash"`
	ComposedAt   *time.Time    `json:"composedAt,omitempty"`
	Model        string        `json:"model,omitempty"`
	Segments     []Segment     `json:"segments"`
	RenderedFrom *RenderSource `json:"renderedFrom,omitempty"`
	// pgRevision is the audio_scripts.revision_no this script was loaded from
	// on the Postgres backend; unset on the file backend. Not persisted in the
	// JSON payload — the row key is authoritative, like the file path.
	pgRevision int32
}

// SourceHash 是章节正文的指纹：换行统一后取 sha256。正文一改就与 script 里的
// sourceHash 不符，界面显示 "Sửa sau khi chốt"。
func SourceHash(text string) string {
	norm := strings.ReplaceAll(text, "\r\n", "\n")
	norm = strings.TrimSpace(norm)
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:])
}

// ScriptHash excludes render metadata and composition timestamps: only audible
// script inputs invalidate the audio, including order, speaker and pauses.
func (s *Script) ScriptHash() string {
	h := sha256.New()
	for _, seg := range s.Segments {
		fmt.Fprintf(h, "%q %q %q %q %d\n", seg.ID, seg.Speaker, seg.Text, seg.Tone, seg.PauseMs)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func castHash(cast *Cast) string {
	h := sha256.New()
	fmt.Fprintf(h, "%q %q %v\n", cast.Mode, cast.NarratorCharacterID, cast.Single)
	for _, sp := range cast.Speakers {
		voice := ""
		if sp.VoiceID != nil {
			voice = *sp.VoiceID
		}
		fmt.Fprintf(h, "%q %q %q\n", sp.ID, voice, sp.SameAs)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Script) freshness(currentHash string) (chapterChanged, scriptChanged bool, reason string) {
	if s == nil {
		return
	}
	chapterChanged = currentHash != "" && s.SourceHash != "" && s.SourceHash != currentHash
	scriptChanged = s.RenderedFrom != nil && s.RenderedFrom.ScriptHash != "" && s.RenderedFrom.ScriptHash != s.ScriptHash()
	if chapterChanged {
		reason = "Chương này đã được sửa sau khi chốt. Tạo lại để cập nhật bản đọc và giọng đọc."
	} else if scriptChanged {
		reason = "Bản đọc đã sửa sau khi tạo. Tạo lại để cập nhật giọng đọc."
	}
	return
}

// captureLegacyRenderSource preserves the last known script before an edit or
// re-composition discards its rendered segments. Historical cast/quality remain
// unknown rather than being inferred from today's settings.
func (s *Script) captureLegacyRenderSource() {
	if s.RenderedFrom != nil && s.RenderedFrom.ScriptHash != "" {
		return
	}
	for _, seg := range s.Segments {
		if seg.Rendered != nil && seg.Rendered.Key != "" {
			s.RenderedFrom = &RenderSource{ScriptHash: s.ScriptHash()}
			return
		}
	}
}

// Stale 判断锁定后的 bản đọc是否已经落后于正文。
func (s *Script) Stale(currentHash string) bool {
	if s == nil || s.Status != StatusLocked {
		return false
	}
	return currentHash != "" && s.SourceHash != "" && s.SourceHash != currentHash
}

// Unknown provenance is not proof of an edit, but an explicit regeneration must
// still compose from today's chapter instead of trusting an untracked script.
func (s *Script) needsComposition(currentHash string) bool {
	return s == nil || len(s.Segments) == 0 || currentHash != "" && (s.SourceHash == "" || s.SourceHash != currentHash)
}

// DeriveStatus 汇总"没有 script / 正在 soạn / 等 duyệt / 已 chốt / 落后"。
func DeriveStatus(script *Script, composing bool, currentHash string) string {
	switch {
	case script == nil:
		if composing {
			return StatusDraft
		}
		return StatusTodo
	case composing:
		return StatusDraft
	case script.Status == StatusLocked:
		if script.Stale(currentHash) {
			return StatusStale
		}
		return StatusLocked
	default:
		return StatusReview
	}
}

// IssueSegments 返回"需要人看一眼"的段数（Cần xem：chưa rõ người nói / câu dài / tiếng Anh / vai chưa có giọng）。
func (s *Script) IssueSegments() int {
	if s == nil {
		return 0
	}
	n := 0
	for _, seg := range s.Segments {
		if len(seg.Flags) > 0 {
			n++
		}
	}
	return n
}

// SpeakerCount 是本章不同说话人的数量（"46 đoạn · 5 người nói"）。
func (s *Script) SpeakerCount() int {
	if s == nil {
		return 0
	}
	seen := map[string]bool{}
	for _, seg := range s.Segments {
		seen[seg.Speaker] = true
	}
	return len(seen)
}

// AudioStats 统计已生成音频的段数与总时长（含段间停顿）。
func (s *Script) AudioStats() (done int, duration float64) {
	if s == nil {
		return 0, 0
	}
	for _, seg := range s.Segments {
		if seg.Rendered != nil && seg.Rendered.DurationSec > 0 {
			done++
			duration += seg.Rendered.DurationSec
			duration += float64(seg.PauseMs) / 1000
		}
	}
	return done, duration
}

// EstimatedDuration 是"≈ 17 phút đọc"用的估算：越南语平均每秒约 4.2 音节，
// 这里按词数 × 1.15 音节近似（一个词常是 1–2 音节），再按 4.2 音节/秒折算。
func (s *Script) EstimatedDuration() float64 {
	if s == nil {
		return 0
	}
	words := 0
	for _, seg := range s.Segments {
		words += countWords(seg.Text)
	}
	return math.Round(float64(words)*1.15/4.2*10) / 10
}

// ChangedSegments 返回"文字改了、音频旧了"的段数（橙色圆点 / "2 đoạn cần đọc lại"）。
func (s *Script) ChangedSegments(key func(seg Segment) string) int {
	if s == nil {
		return 0
	}
	n := 0
	for _, seg := range s.Segments {
		if seg.Rendered == nil {
			continue
		}
		if seg.Rendered.Key != key(seg) {
			n++
		}
	}
	return n
}

// SynthKey 是音频缓存的输入：只要有一项变了就必须重新生成。
// 契约（10-audio §3）为 text+voice+tone+speed+quality+engine+model；
// 这里额外带上 pitch（"đổi giọng nhẹ khi là lời thoại" 的变调量），
// 否则单声模式下叙述/对白的变调会共用同一个缓存。
type SynthKey struct {
	Text    string
	VoiceID string
	Tone    string
	Speed   float64
	Pitch   float64
	Quality string
	Engine  string
	Model   string
}

// Hash 返回稳定哈希（缓存文件名）。
func (k SynthKey) Hash() string {
	parts := []string{
		k.Text, k.VoiceID, k.Tone,
		fmt.Sprintf("%.3f", k.Speed), fmt.Sprintf("%.3f", k.Pitch),
		k.Quality, k.Engine, k.Model,
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

// scriptFile 是磁盘格式；与 Script 一致，但允许未来加字段时不丢未知键。
type scriptFile struct {
	Chapter      int           `json:"chapter"`
	Volume       int           `json:"volume"`
	Status       string        `json:"status"`
	LockedAt     *time.Time    `json:"lockedAt"`
	SourceHash   string        `json:"sourceHash"`
	ComposedAt   *time.Time    `json:"composedAt"`
	Model        string        `json:"model"`
	Segments     []Segment     `json:"segments"`
	RenderedFrom *RenderSource `json:"renderedFrom,omitempty"`
}

// ReadScript 读取某章 bản đọc；不存在返回 (nil, nil)。
func ReadScript(l Layout, volume, chapter int) (*Script, error) {
	data, err := os.ReadFile(l.ScriptPath(volume, chapter))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var f scriptFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("bản đọc chương %d hỏng: %w", chapter, err)
	}
	script := &Script{
		Chapter: f.Chapter, Volume: f.Volume, Status: f.Status, LockedAt: f.LockedAt,
		SourceHash: f.SourceHash, ComposedAt: f.ComposedAt, Model: f.Model, Segments: f.Segments,
		RenderedFrom: f.RenderedFrom,
	}
	// Cờ "english" được suy ra từ text; bộ lọc từng đánh dấu nhầm âm tiết Việt
	// (in, to, hot…). Tính lại khi đọc để bản lưu cũ không thổi phồng "Cần xem".
	for i := range script.Segments {
		refreshEnglishFlag(&script.Segments[i])
	}
	return script, nil
}

// WriteScript 原子写入 bản đọc（同目录临时文件 + rename），并回填章/卷号。
func WriteScript(l Layout, s *Script) error {
	if s == nil {
		return errors.New("bản đọc rỗng")
	}
	path := l.ScriptPath(s.Volume, s.Chapter)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if s.Segments == nil {
		s.Segments = []Segment{}
	}
	f := scriptFile{
		Chapter: s.Chapter, Volume: s.Volume, Status: s.Status, LockedAt: s.LockedAt,
		SourceHash: s.SourceHash, ComposedAt: s.ComposedAt, Model: s.Model, Segments: s.Segments,
		RenderedFrom: s.RenderedFrom,
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data)
}

// writeFileAtomic 写临时文件再 rename，避免进程中断留下半个 JSON。
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// sortFlags 让 flags 输出稳定（测试与 diff 友好）。
func sortFlags(flags []string) []string {
	if len(flags) == 0 {
		return nil
	}
	sort.Strings(flags)
	return flags
}

// refreshEnglishFlag tính lại cờ FlagEnglish từ text, giữ nguyên các cờ khác.
// Dùng khi đọc bản lưu cũ và khi soạn/sửa để cờ luôn khớp bộ lọc hiện hành.
func refreshEnglishFlag(seg *Segment) {
	kept := seg.Flags[:0]
	for _, f := range seg.Flags {
		if f != FlagEnglish {
			kept = append(kept, f)
		}
	}
	if len(englishWords(seg.Text)) > 0 {
		kept = append(kept, FlagEnglish)
	}
	seg.Flags = sortFlags(kept)
}
