package audio

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// 服务侧"soạn bản đọc"：异步跑模型，逐章广播 audio.compose；
// 以及人工编辑后的保存（PUT script）与"soạn lại khi stale"（refresh）。

// Compose 开始异步 soạn bản đọc。只做同步校验（缺模型/缺章节立即报错），
// 模型调用在后台跑，进度走 SSE audio.compose。
func (s *Service) Compose(source []SourceChapter, volume int, chapters []int, composer Composer, ref CharacterRef) error {
	if composer == nil {
		return ErrNoModel
	}
	if len(chapters) == 0 {
		return conflict("Chưa chọn chương nào để soạn.")
	}
	byChapter := make(map[int]SourceChapter, len(source))
	for _, sc := range source {
		byChapter[sc.Chapter] = sc
	}
	list := make([]int, 0, len(chapters))
	for _, ch := range chapters {
		if _, ok := byChapter[ch]; !ok {
			return conflict("Không có chương %d trong tập này.", ch)
		}
		if s.isComposing(volume, ch) {
			continue
		}
		list = append(list, ch)
	}
	if len(list) == 0 {
		return conflict("Các chương này đang được soạn rồi.")
	}
	s.compWG.Add(1)
	go func() {
		defer s.compWG.Done()
		s.composeChapters(volume, list, byChapter, composer, ref, false)
	}()
	return nil
}

// Refresh 在 stale 时重跑模型并把"没变的段"的音频保留下来。
// 没 stale 的章节直接跳过（不浪费模型调用）。
func (s *Service) Refresh(source []SourceChapter, volume int, chapter int, composer Composer, ref CharacterRef) error {
	if composer == nil {
		return ErrNoModel
	}
	sc, ok := findSource(source, chapter)
	if !ok {
		return conflict("Không có chương %d trong tập này.", chapter)
	}
	if !sc.Completed {
		return conflict("Chương %d chưa nghiệm thu — chỉ soạn bản đọc cho chương đã nghiệm thu.", chapter)
	}
	script, err := ReadScript(s.layout, volume, chapter)
	if err != nil {
		return err
	}
	if script == nil {
		return conflict("Chương %d chưa có bản đọc — hãy soạn trước.", chapter)
	}
	if !script.needsComposition(SourceHash(sc.Text)) {
		return nil
	}
	if s.isComposing(volume, chapter) {
		return conflict("Chương này đang được soạn rồi.")
	}
	s.compWG.Add(1)
	go func() {
		defer s.compWG.Done()
		s.composeChapters(volume, []int{chapter}, map[int]SourceChapter{chapter: sc}, composer, ref, true)
	}()
	return nil
}

func findSource(source []SourceChapter, chapter int) (SourceChapter, bool) {
	for _, sc := range source {
		if sc.Chapter == chapter {
			return sc, true
		}
	}
	return SourceChapter{}, false
}

// composeChapters 顺序处理（模型调用不打并发，避免本地/云端限流）。
func (s *Service) composeChapters(volume int, chapters []int, source map[int]SourceChapter, composer Composer, ref CharacterRef, refresh bool) {
	settings := s.Settings()
	dict, err := s.Pronunciations()
	if err != nil {
		dict = nil
	}
	cast, err := s.castForRender()
	if err != nil {
		cast = &Cast{Mode: ModeCast}
	}
	// cast 可能还没初始化：用角色档案补齐，让模型拿到完整的说话人清单，
	// 同时保证 speaker id 与日后打开分派面板时生成的一致（EnsureCast 是确定的）。
	enriched := EnsureCast(cast, ref.Characters, ref.Support)
	cast = &enriched
	speakers := composeSpeakers(cast, ref)
	for _, chapter := range chapters {
		sc := source[chapter]
		if !sc.Completed {
			s.hub.Publish(NewEvent(EventCompose, map[string]any{
				"chapter": chapter, "state": "error", "segments": 0, "percent": float64(0),
				"error": fmt.Sprintf("Chương %d chưa nghiệm thu — chỉ soạn bản đọc cho chương đã nghiệm thu.", chapter),
			}))
			continue
		}
		s.setComposing(volume, chapter, true)
		s.hub.Publish(NewEvent(EventCompose, map[string]any{
			"chapter": chapter, "state": "drafting", "segments": 0, "percent": float64(0),
		}))
		in := ComposeInput{
			Volume: volume, Chapter: chapter, Title: sc.Title, Text: sc.Text,
			Characters: speakers, Mode: cast.Mode, Normalize: settings.Normalize, Dict: dict,
		}
		res, cerr := composer.Compose(s.ctx, in)
		if cerr != nil {
			s.setComposing(volume, chapter, false)
			s.hub.Publish(NewEvent(EventCompose, map[string]any{
				"chapter": chapter, "state": "error", "segments": 0, "percent": float64(0),
				"error": cerr.Error(),
			}))
			continue
		}
		now := time.Now()
		fresh := &Script{
			Chapter: chapter, Volume: volume, Status: StatusReview,
			SourceHash: SourceHash(sc.Text), ComposedAt: &now, Model: res.Model, Segments: res.Segments,
		}
		if refresh {
			if old, rerr := ReadScript(s.layout, volume, chapter); rerr == nil && old != nil {
				merged, _ := mergeScript(old, fresh)
				fresh = merged
				if old.Status == StatusLocked {
					fresh.Status = StatusLocked
					fresh.LockedAt = old.LockedAt
				}
			}
		}
		applyVoiceFlags(fresh, cast)
		if werr := WriteScript(s.layout, fresh); werr != nil {
			s.setComposing(volume, chapter, false)
			s.hub.Publish(NewEvent(EventCompose, map[string]any{
				"chapter": chapter, "state": "error", "segments": 0, "percent": float64(0), "error": werr.Error(),
			}))
			continue
		}
		s.setComposing(volume, chapter, false)
		s.hub.Publish(NewEvent(EventCompose, map[string]any{
			"chapter": chapter, "state": "done", "segments": len(fresh.Segments), "percent": float64(100),
		}))
		if fresh.Status == StatusLocked {
			if _, eerr := s.Enqueue(volume, chapter, fresh.SourceHash); eerr == nil {
				continue
			}
		}
	}
}

// composeSpeakers 把分派 + 角色别名拼成给模型看的清单。
func composeSpeakers(cast *Cast, ref CharacterRef) []ComposeSpeaker {
	aliases := map[string][]string{}
	for _, ch := range ref.Characters {
		aliases[strings.ToLower(strings.TrimSpace(ch.Name))] = ch.Aliases
	}
	out := make([]ComposeSpeaker, 0, len(cast.Speakers)+1)
	out = append(out, ComposeSpeaker{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator})
	for _, sp := range cast.Speakers {
		if sp.ID == NarratorID {
			continue
		}
		out = append(out, ComposeSpeaker{
			ID: sp.ID, Name: sp.Name, Role: sp.Role,
			Aliases: aliases[strings.ToLower(sp.Name)],
		})
	}
	return out
}

// mergeScript 把新 soạn的结果与旧 bản đọc合并：
// 同一段原文（Source）且文本未变 ⇒ 保留 id 与已生成的音频；
// 文本变了的段丢掉音频（橙色"cần đọc lại"）。返回合并结果与变更段数。
func mergeScript(old, fresh *Script) (*Script, int) {
	if old == nil {
		return fresh, len(fresh.Segments)
	}
	bySource := map[string]Segment{}
	old.captureLegacyRenderSource()
	fresh.RenderedFrom = old.RenderedFrom
	for _, seg := range old.Segments {
		key := strings.TrimSpace(seg.Source)
		if key == "" {
			key = strings.TrimSpace(seg.Text)
		}
		if _, exists := bySource[key]; !exists {
			bySource[key] = seg
		}
	}
	changed := 0
	for i := range fresh.Segments {
		seg := fresh.Segments[i]
		key := strings.TrimSpace(seg.Source)
		if key == "" {
			key = strings.TrimSpace(seg.Text)
		}
		prev, ok := bySource[key]
		if !ok {
			changed++
			continue
		}
		seg.ID = prev.ID
		if prev.Text == seg.Text {
			seg.Rendered = prev.Rendered
		} else {
			changed++
		}
		if seg.Tone == "" {
			seg.Tone = prev.Tone
		}
		fresh.Segments[i] = seg
	}
	// Reserve all existing IDs before assigning replacements: a position-based
	// replacement can itself collide after the model inserts or removes a line.
	reserved := make(map[string]bool, len(fresh.Segments))
	for _, seg := range fresh.Segments {
		reserved[seg.ID] = true
	}
	seen := make(map[string]bool, len(fresh.Segments))
	next := 1
	for i := range fresh.Segments {
		if fresh.Segments[i].ID == "" || seen[fresh.Segments[i].ID] {
			for {
				id := fmt.Sprintf("s%04d", next)
				next++
				if !reserved[id] {
					fresh.Segments[i].ID = id
					reserved[id] = true
					break
				}
			}
		}
		seen[fresh.Segments[i].ID] = true
	}
	return fresh, changed
}

// applyVoiceFlags 按分派给"还没选声音"的说话人加 no_voice 标记（Cần xem 面板）。
func applyVoiceFlags(script *Script, cast *Cast) {
	if cast == nil {
		return
	}
	for i := range script.Segments {
		seg := &script.Segments[i]
		flags := make([]string, 0, len(seg.Flags)+1)
		for _, f := range seg.Flags {
			if f != FlagNoVoice {
				flags = append(flags, f)
			}
		}
		if !hasVoice(cast, seg.Speaker) {
			flags = append(flags, FlagNoVoice)
		}
		seg.Flags = sortFlags(flags)
	}
}

func hasVoice(cast *Cast, speaker string) bool {
	if cast.Mode == ModeSingle {
		return cast.Single.VoiceID != ""
	}
	sp, ok := cast.SpeakerByID(speaker)
	if !ok {
		sp, ok = cast.SpeakerByName(speaker)
	}
	if !ok {
		// 不认识的说话人：只要有叙述者或统一声音就能兜底。
		if n, found := cast.SpeakerByID(NarratorID); found && n.VoiceID != nil && *n.VoiceID != "" {
			return true
		}
		return cast.Single.VoiceID != ""
	}
	if sp.VoiceID != nil && *sp.VoiceID != "" {
		return true
	}
	if sp.SameAs != "" {
		if other, found := cast.SpeakerByID(sp.SameAs); found && other.VoiceID != nil && *other.VoiceID != "" {
			return true
		}
	}
	if sp.ID == NarratorID {
		return false
	}
	if n, found := cast.SpeakerByID(NarratorID); found && n.VoiceID != nil && *n.VoiceID != "" {
		return true
	}
	return cast.Single.VoiceID != ""
}

// ScriptEdit 是 PUT script 的请求体（只允许改段落内容与停顿/语气）。
type ScriptEdit struct {
	Segments []Segment `json:"segments"`
}

// SaveScript 保存人工编辑。保留"文本没改"的段的已生成音频；
// 改过的段清掉 Normalized（无法反推）并重算 english/long_sentence 标记。
func (s *Service) SaveScript(volume, chapter int, edit ScriptEdit) (*Script, error) {
	script, err := ReadScript(s.layout, volume, chapter)
	if err != nil {
		return nil, err
	}
	if script == nil {
		return nil, ErrNotFound
	}
	if len(edit.Segments) == 0 {
		return nil, conflict("Bản đọc phải còn ít nhất một đoạn.")
	}
	cast, err := s.castForRender()
	if err != nil {
		return nil, err
	}
	prev := map[string]Segment{}
	// Preserve provenance for legacy audio before editing drops changed segments.
	script.captureLegacyRenderSource()
	for _, seg := range script.Segments {
		prev[seg.ID] = seg
	}
	for i := range edit.Segments {
		edit.Segments[i].Text = strings.TrimSpace(edit.Segments[i].Text)
		if edit.Segments[i].Text == "" {
			return nil, conflict("Đoạn %d không có chữ để đọc.", i+1)
		}
	}
	// Đoạn chỉ có dấu câu không đọc thành tiếng: gộp vào đoạn liền kề trước khi
	// so văn bản, để đoạn nhận dấu câu được đọc lại còn các đoạn khác giữ giọng.
	edit.Segments = foldUnspeakable(edit.Segments)
	if len(edit.Segments) == 0 {
		return nil, conflict("Bản đọc chỉ có dấu câu, không có chữ để đọc.")
	}
	out := make([]Segment, 0, len(edit.Segments))
	for i, seg := range edit.Segments {
		seg.Source = strings.TrimSpace(seg.Source)
		seg.Speaker = strings.TrimSpace(seg.Speaker)
		if seg.Speaker == "" {
			seg.Speaker = NarratorID
		}
		seg.PauseMs = clampPause(seg.PauseMs)
		seg.Tone = strings.TrimSpace(seg.Tone)
		seg.ID = strings.TrimSpace(seg.ID)
		if seg.ID == "" {
			seg.ID = fmt.Sprintf("s%04d", i+1)
		}
		old, hadOld := prev[seg.ID]
		if hadOld && old.Text == seg.Text {
			seg.Rendered = old.Rendered
			seg.Normalized = old.Normalized
			seg.Source = old.Source
		} else {
			seg.Rendered = nil
			seg.Normalized = nil
		}
		flags := make([]string, 0, 3)
		for _, f := range seg.Flags {
			switch f {
			case FlagUnknownSpeaker, FlagNoVoice:
				flags = append(flags, f)
			}
		}
		if countWords(seg.Source) > LongSentenceWords || (seg.Source == "" && countWords(seg.Text) > LongSentenceWords) {
			flags = append(flags, FlagLongSentence)
		}
		if len(englishWords(seg.Text)) > 0 {
			flags = append(flags, FlagEnglish)
		}
		seg.Flags = sortFlags(flags)
		out = append(out, seg)
	}
	script.Segments = out
	applyVoiceFlags(script, cast)
	if err := WriteScript(s.layout, script); err != nil {
		return nil, err
	}
	if err := s.invalidateChapterAudio(volume, chapter); err != nil {
		return nil, err
	}
	return script, nil
}

// sortedChapters 去重并升序（API 输入宽容一些）。
func sortedChapters(in []int) []int {
	seen := map[int]bool{}
	out := make([]int, 0, len(in))
	for _, ch := range in {
		if ch <= 0 || seen[ch] {
			continue
		}
		seen[ch] = true
		out = append(out, ch)
	}
	sort.Ints(out)
	return out
}
