package audio

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/datnt1998/noverse/internal/library"
)

// 分派：整本 truyện 的"谁说谁"（audio/vai.json）。
//
// 两种模式（10-audio §2 面板 Chế độ giọng）：
//   - cast：每个角色一个声音（audio drama）
//   - single：一个人读全部，可选"đổi giọng nhẹ khi là lời thoại"
// 模式切换不改 bản đọc：speaker 一直保留，切回来不用重新 soạn。

// 模式。
const (
	ModeCast   = "cast"
	ModeSingle = "single"
)

// NarratorID 是叙述者的固定 id（"Dẫn truyện"）。
const NarratorID = "dan"

// 说话人角色。
const (
	RoleNarrator = "narrator"
	RoleMain     = "main"
	RoleSupport  = "support"
)

// Cast 对应 GET/PUT S/audio/cast。
type Cast struct {
	Mode                string     `json:"mode"`
	VoiceMode           string     `json:"voiceMode,omitempty"`
	NarratorCharacterID string     `json:"narratorCharacterId,omitempty"`
	Single              SingleCast `json:"single"`
	Speakers            []Speaker  `json:"speakers"`
}

// SingleCast 是"một giọng"模式的设置。
type SingleCast struct {
	VoiceID         string `json:"voiceId"`
	DialogueShift   bool   `json:"dialogueShift"`
	KeepTones       bool   `json:"keepTones"`
	DialoguePauseMs int    `json:"dialoguePauseMs"`
}

// Speaker 是一个说话人。VoiceID 为空表示"还没选声音"（界面橙框 [Chọn giọng cho …]）。
type Speaker struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Ini  string `json:"ini,omitempty"` // 用户自定的头像缩写，优先于按名推算
	Role string `json:"role"`
	// Kind 是角色档案的 loại vai（lead/antagonist/ally/support，Gói S）——头像上色用，
	// 叙述者与名册里的群演留空。每次读分派时按角色档案补齐，不靠 vai.json 里存过。
	Kind        string  `json:"kind,omitempty"`
	VoiceID     *string `json:"voiceId"`
	SameAs      string  `json:"sameAs,omitempty"`
	OldSegments int     `json:"oldSegments,omitempty"`
	OldETASec   float64 `json:"oldEtaSec,omitempty"`
}

// DefaultCast 从故事的角色档案生成初始分派：叙述者 + 核心角色 + 配角名册。
// 已有分派保留（EnsureCast 只在缺人时补）。
func DefaultCast(chars []Character, support []SupportCharacter) Cast {
	c := Cast{Mode: ModeCast, Single: SingleCast{DialogueShift: true, KeepTones: true}}
	c.Speakers = append(c.Speakers, Speaker{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator})
	used := map[string]bool{NarratorID: true}
	for _, ch := range chars {
		name := strings.TrimSpace(ch.Name)
		if name == "" {
			continue
		}
		id := uniqueSpeakerID(name, used)
		role := RoleSupport
		if len(c.Speakers) <= 3 {
			role = RoleMain
		}
		c.Speakers = append(c.Speakers, Speaker{ID: id, Name: name, Ini: ch.Ini, Role: role, Kind: ch.Kind})
	}
	for _, e := range support {
		name := strings.TrimSpace(e.Name)
		if name == "" {
			continue
		}
		if c.HasName(name) {
			continue
		}
		id := uniqueSpeakerID(name, used)
		c.Speakers = append(c.Speakers, Speaker{ID: id, Name: name, Role: RoleSupport})
	}
	return c
}

// HasName 判断分派里是否已有同名说话人（忽略大小写）。
func (c *Cast) HasName(name string) bool {
	for _, s := range c.Speakers {
		if strings.EqualFold(s.Name, name) || strings.EqualFold(s.ID, name) {
			return true
		}
	}
	return false
}

// SpeakerByID 按 id 查说话人。
func (c *Cast) SpeakerByID(id string) (Speaker, bool) {
	for _, s := range c.Speakers {
		if s.ID == id {
			return s, true
		}
	}
	return Speaker{}, false
}

// SpeakerByName 按名字（或别名）查说话人。
func (c *Cast) SpeakerByName(name string) (Speaker, bool) {
	for _, s := range c.Speakers {
		if strings.EqualFold(s.Name, name) {
			return s, true
		}
	}
	return Speaker{}, false
}

// CastDefaults đặt chế độ và giọng kể khi tạo phân vai lần đầu.
type CastDefaults struct {
	Mode    string
	VoiceID string
}

// EnsureCast 合并角色档案：新角色补进分派，已有角色保留声音选择。
func EnsureCast(cur *Cast, chars []Character, support []SupportCharacter, defaults ...CastDefaults) Cast {
	base := DefaultCast(chars, support)
	if cur == nil {
		if len(defaults) > 0 {
			if defaults[0].Mode != "" {
				base.Mode = defaults[0].Mode
			}
			base.Single.VoiceID = defaults[0].VoiceID
			if defaults[0].VoiceID != "" {
				voice := defaults[0].VoiceID
				base.Speakers[0].VoiceID = &voice
			}
		}
		return base
	}
	out := *cur
	if out.Mode == "" {
		out.Mode = ModeCast
	}
	// 现有顺序优先，新角色按 DefaultCast 顺序追加。
	byName := map[string]Speaker{}
	for _, s := range out.Speakers {
		byName[strings.ToLower(s.Name)] = s
	}
	for _, s := range base.Speakers {
		if _, ok := byName[strings.ToLower(s.Name)]; ok {
			continue
		}
		out.Speakers = append(out.Speakers, s)
	}
	speakers := make([]Speaker, len(out.Speakers))
	copy(speakers, out.Speakers)
	fillSpeakerKinds(speakers, chars)
	out.Speakers = speakers
	return out
}

// fillSpeakerKinds 按角色档案补齐已有说话人的 loại vai（Gói S）。vai.json 里存的
// kind 只是副本：缺了或旧了都以角色档案为准，叙述者/群演没有对应角色则保持空。
func fillSpeakerKinds(speakers []Speaker, chars []Character) {
	kinds := make(map[string]string, len(chars))
	for _, ch := range chars {
		if name := strings.ToLower(strings.TrimSpace(ch.Name)); name != "" && ch.Kind != "" {
			kinds[name] = ch.Kind
		}
	}
	for i := range speakers {
		if kind, ok := kinds[strings.ToLower(strings.TrimSpace(speakers[i].Name))]; ok {
			speakers[i].Kind = kind
		}
	}
}

// uniqueSpeakerID 生成短 id（"Hứa Hoài Châu" → "hhc"），冲突时加序号。
func uniqueSpeakerID(name string, used map[string]bool) string {
	id := SpeakerID(name)
	if id == "" {
		id = "sp"
	}
	candidate := id
	for i := 2; used[candidate]; i++ {
		candidate = fmt.Sprintf("%s%d", id, i)
	}
	used[candidate] = true
	return candidate
}

// SpeakerID 取名字各词首字母（去声调、小写）。
func SpeakerID(name string) string {
	fields := strings.Fields(strings.TrimSpace(name))
	if len(fields) == 0 {
		return ""
	}
	var b strings.Builder
	for _, f := range fields {
		for _, r := range f {
			if !unicode.IsLetter(r) {
				continue
			}
			b.WriteRune(stripTone(r))
			break
		}
	}
	id := b.String()
	if len([]rune(id)) > 6 {
		id = string([]rune(id)[:6])
	}
	return id
}

// stripTone 去越南语声调（保留 đ → d），非字母原样小写。
func stripTone(r rune) rune {
	lowered := unicode.ToLower(r)
	if lowered < unicode.MaxASCII {
		return lowered
	}
	slug := library.Slugify(string(r), 1)
	if slug == "" || slug == "truyen-moi" {
		return lowered
	}
	return rune(slug[0])
}

// VoiceRef 是"这一段该用哪个声音"的解析结果。
type VoiceRef struct {
	VoiceID  string
	RefAudio string
	Pitch    float64
	Speaker  string
	OK       bool
}

// ResolveVoice 解析某段该用的声音。
//
//   - single 模式：统一用 single.voiceId；叙述者不变调，对白按 dialogueShift 升 5%（pitch）
//   - cast 模式：说话人的 voiceId；sameAs 指向别人的声音；缺声音时退回叙述者/统一声音
//   - keepTones=false 时丢掉段落 sắc thái（tone 只进缓存键，不影响声音）
func ResolveVoice(c *Cast, seg Segment, voices map[string]ProjectVoice) VoiceRef {
	ref := VoiceRef{Speaker: seg.Speaker}
	if c == nil {
		return ref
	}
	pick := func(id string) (ProjectVoice, bool) {
		v, ok := voices[id]
		return v, ok
	}
	if c.Mode == ModeSingle {
		ref.VoiceID = c.Single.VoiceID
		if ref.VoiceID == "" {
			return ref
		}
		if v, ok := pick(ref.VoiceID); ok {
			ref.RefAudio = v.File
		}
		ref.Pitch = 1
		if c.Single.DialogueShift && seg.Speaker != NarratorID {
			ref.Pitch = 1.05
		}
		ref.OK = true
		return ref
	}
	sp, ok := c.SpeakerByID(seg.Speaker)
	if !ok {
		sp, ok = c.SpeakerByName(seg.Speaker)
	}
	if !ok {
		// 模型给了个不存在的名字：退回叙述者，界面另有 unknown_speaker 标记。
		sp, ok = c.SpeakerByID(NarratorID)
	}
	if ok {
		id := ""
		if sp.VoiceID != nil {
			id = *sp.VoiceID
		}
		if id == "" && sp.SameAs != "" {
			if other, found := c.SpeakerByID(sp.SameAs); found && other.VoiceID != nil {
				id = *other.VoiceID
			}
		}
		if id == "" && sp.ID != NarratorID {
			if narrator, found := c.SpeakerByID(NarratorID); found && narrator.VoiceID != nil {
				id = *narrator.VoiceID
			}
		}
		if id == "" {
			id = c.Single.VoiceID
		}
		ref.VoiceID = id
		if v, found := pick(id); found {
			ref.RefAudio = v.File
		}
	}
	ref.Pitch = 1
	ref.OK = ref.VoiceID != ""
	return ref
}

// ReadCast 读 vai.json；不存在返回 (nil, nil)。
func ReadCast(l Layout) (*Cast, error) {
	data, err := os.ReadFile(l.castPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var c Cast
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// WriteCast 原子写 vai.json。
func WriteCast(l Layout, c *Cast) error {
	if c == nil {
		return errors.New("phân vai rỗng")
	}
	if c.Mode != ModeCast && c.Mode != ModeSingle {
		return errors.New("mode phải là cast hoặc single")
	}
	if c.Speakers == nil {
		c.Speakers = []Speaker{}
	}
	if err := os.MkdirAll(l.AudioDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(l.castPath(), data)
}

// sortSpeakers 让输出稳定：叙述者第一，其余按名字。
func sortSpeakers(sp []Speaker) {
	sort.SliceStable(sp, func(i, j int) bool {
		if sp[i].ID == NarratorID {
			return true
		}
		if sp[j].ID == NarratorID {
			return false
		}
		return sp[i].Name < sp[j].Name
	})
}

// StoredScripts reads all volumes, not only the currently open mobile volume.
func (s *Service) StoredScripts() ([]*Script, error) {
	paths, err := filepath.Glob(filepath.Join(s.layout.AudioDir, scriptsSubdir, "tap-*", "*.json"))
	if err != nil {
		return nil, err
	}
	out := make([]*Script, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var script Script
		if err := json.Unmarshal(data, &script); err != nil {
			return nil, err
		}
		out = append(out, &script)
	}
	return out, nil
}

func (s *Service) CastOldSegments(c *Cast) error {
	scripts, err := s.StoredScripts()
	if err != nil {
		return err
	}
	speakers := make(map[string]*Speaker, len(c.Speakers))
	for i := range c.Speakers {
		c.Speakers[i].OldSegments = 0
		c.Speakers[i].OldETASec = 0
		speakers[c.Speakers[i].ID] = &c.Speakers[i]
	}
	rtf := s.synth.Status(s.ctx).RTF
	for _, script := range scripts {
		if script.Status != StatusLocked {
			continue
		}
		for _, seg := range script.Segments {
			if seg.Rendered == nil {
				continue
			}
			if speaker := speakers[seg.Speaker]; speaker != nil {
				speaker.OldSegments++
				if rtf != nil && *rtf > 0 {
					speaker.OldETASec += seg.Rendered.DurationSec * *rtf
				}
			}
		}
	}
	return nil
}

// ChangeRoleVoice changes the shared cast and optionally schedules only this
// role's already-rendered lines; other roles keep their original audio.
func (s *Service) ChangeRoleVoice(role, voiceID, sameAs string, rerenderOld bool) (*Cast, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cast, err := ReadCast(s.layout)
	if err != nil {
		return nil, 0, err
	}
	if cast == nil {
		return nil, 0, conflict("Chưa có phân vai. Hãy mở Giọng đọc trước.")
	}
	voices, err := s.Voices(s.ctx)
	if err != nil {
		return nil, 0, err
	}
	if _, ok := VoiceMap(voices)[voiceID]; !ok {
		return nil, 0, conflict("Giọng đã chọn không còn có sẵn. Hãy chọn lại.")
	}
	found := role == "single"
	if role == "single" {
		cast.Single.VoiceID = voiceID
	}
	for i := range cast.Speakers {
		if cast.Speakers[i].ID == NarratorID && cast.Speakers[i].SameAs == role {
			cast.NarratorCharacterID = role
			cast.Speakers[i].SameAs = ""
		}
	}
	for i := range cast.Speakers {
		if cast.Speakers[i].ID != role {
			continue
		}
		if sameAs != "" && (sameAs != NarratorID || role == NarratorID) {
			return nil, 0, conflict("Không dùng chung giọng này được.")
		}
		cast.Speakers[i].VoiceID = &voiceID
		cast.Speakers[i].SameAs = sameAs
		found = true
	}
	if !found {
		return nil, 0, conflict("Không tìm thấy vai cần đổi giọng.")
	}
	scripts, err := s.StoredScripts()
	if err != nil {
		return nil, 0, err
	}
	if cast.VoiceMode == "" {
		cast.VoiceMode = string(cast.Mode)
	}
	if err := WriteCast(s.layout, cast); err != nil {
		return nil, 0, err
	}
	queued := 0
	if rerenderOld {
		for _, script := range scripts {
			if script.Status != StatusLocked {
				continue
			}
			n := 0
			for _, seg := range script.Segments {
				if seg.Rendered != nil && (seg.Speaker == role || role == "single") {
					n++
				}
			}
			if n == 0 {
				continue
			}
			if _, err := s.queue.EnqueueRole(script.Volume, script.Chapter, role, n); err != nil {
				return nil, queued, err
			}
			queued += n
		}
	}
	if queued > 0 {
		s.hub.Publish(NewEvent(EventQueue, map[string]any{"jobs": s.queue.Snapshot()}))
		s.kick()
	}
	return cast, queued, nil
}

// ApplyPackVoice resolves the actual pack defaults, never a guessed stock voice.
func (s *Service) ApplyPackVoice(c *Cast) error {
	if c.VoiceMode != "pack" {
		return nil
	}
	if s.layout.Pack == nil {
		return conflict("Gói đang dùng chưa có giọng đề xuất.")
	}
	defaults, err := s.ensureCast(nil, CharacterRef{})
	if err != nil {
		return err
	}
	voice := defaults.Single.VoiceID
	if voice == "" {
		return conflict("Chưa có giọng phù hợp với gói đang dùng.")
	}
	c.Mode = ModeCast
	for i := range c.Speakers {
		if c.Speakers[i].ID == NarratorID {
			c.Speakers[i].VoiceID = &voice
			c.Speakers[i].SameAs = ""
		}
	}
	return nil
}
