package store

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/datnt1998/noverse/internal/domain"
)

// DraftStore 管理章节构思、草稿和终稿。
type fileDraftStore struct{ io *IO }

func newFileDraftStore(io *IO) *fileDraftStore { return &fileDraftStore{io: io} }

// SaveChapterPlan 保存章节构思到 drafts/{ch}.plan.json。
func (s *fileDraftStore) SaveChapterPlan(plan domain.ChapterPlan) error {
	return s.io.WriteJSON(fmt.Sprintf("drafts/%02d.plan.json", plan.Chapter), plan)
}

// LoadChapterPlan 读取章节构思。
func (s *fileDraftStore) LoadChapterPlan(chapter int) (*domain.ChapterPlan, error) {
	var plan domain.ChapterPlan
	if err := s.io.ReadJSON(fmt.Sprintf("drafts/%02d.plan.json", chapter), &plan); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &plan, nil
}

// SaveDraft 保存整章草稿到 drafts/{ch}.draft.md。
func (s *fileDraftStore) SaveDraft(chapter int, content string) error {
	return s.io.WriteMarkdown(fmt.Sprintf("drafts/%02d.draft.md", chapter), content)
}

// AppendDraft 追加内容到现有草稿（续写模式）。
func (s *fileDraftStore) AppendDraft(chapter int, content string) error {
	rel := fmt.Sprintf("drafts/%02d.draft.md", chapter)
	return s.io.WithWriteLock(func() error {
		existing, err := s.io.ReadFileUnlocked(rel)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		var merged string
		if len(existing) > 0 {
			merged = string(existing) + "\n\n" + content
		} else {
			merged = content
		}
		return s.io.WriteFileUnlocked(rel, []byte(merged))
	})
}

// LoadDraft 读取整章草稿。
func (s *fileDraftStore) LoadDraft(chapter int) (string, error) {
	data, err := s.io.ReadFile(fmt.Sprintf("drafts/%02d.draft.md", chapter))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// LoadChapterContent 加载章节草稿正文及字数（按词计）。
func (s *fileDraftStore) LoadChapterContent(chapter int) (string, int, error) {
	draft, err := s.LoadDraft(chapter)
	if err != nil {
		return "", 0, err
	}
	if draft != "" {
		return draft, domain.WordCount(draft), nil
	}
	return "", 0, nil
}

// SaveFinalChapter 保存最终章节正文到 chapters/{ch}.md。
func (s *fileDraftStore) SaveFinalChapter(chapter int, content string) error {
	return s.io.WriteMarkdown(fmt.Sprintf("chapters/%02d.md", chapter), content)
}

// LoadChapterText 读取已提交的终稿原文。
func (s *fileDraftStore) LoadChapterText(chapter int) (string, error) {
	data, err := s.io.ReadFile(fmt.Sprintf("chapters/%02d.md", chapter))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// LoadChapterRange 读取指定范围的终稿原文片段。
func (s *fileDraftStore) LoadChapterRange(from, to, maxRunes int) (map[int]string, error) {
	result := make(map[int]string)
	for ch := from; ch <= to; ch++ {
		text, err := s.LoadChapterText(ch)
		if err != nil {
			return nil, err
		}
		if text == "" {
			continue
		}
		if maxRunes > 0 {
			runes := []rune(text)
			if len(runes) > maxRunes {
				text = string(runes[:maxRunes]) + "..."
			}
		}
		result[ch] = text
	}
	return result, nil
}

var dialogueRe = regexp.MustCompile(`"[^"]*"`)

// ExtractDialogue 从已提交章节中提取指定角色的对话片段。
// maxCompletedChapter 由调用方传入，避免跨域依赖。
func (s *fileDraftStore) ExtractDialogue(characterName string, aliases []string, maxSamples, maxCompletedChapter int) ([]string, error) {
	if maxSamples <= 0 {
		maxSamples = 5
	}
	names := append([]string{characterName}, aliases...)

	var samples []string
	var readErrs []error
	for ch := maxCompletedChapter; ch >= 1 && len(samples) < maxSamples; ch-- {
		text, err := s.LoadChapterText(ch)
		if err != nil {
			readErrs = append(readErrs, fmt.Errorf("chapter %d: %w", ch, err))
			continue
		}
		if text == "" {
			continue
		}
		paragraphs := strings.Split(text, "\n")
		for _, para := range paragraphs {
			if len(samples) >= maxSamples {
				break
			}
			found := false
			for _, name := range names {
				if strings.Contains(para, name) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
			matches := dialogueRe.FindAllString(para, -1)
			for _, m := range matches {
				if len(samples) >= maxSamples {
					break
				}
				// 旧阈值 >5 字：5 个汉字 ≈ 5 音节 ≈ 4 个越南语词，短到只够一声应答的
				// 对话不具代表性，跳过。
				if domain.WordCount(m) >= 4 {
					samples = append(samples, characterName+": "+m)
				}
			}
		}
	}
	return samples, errors.Join(readErrs...)
}

// ExtractStyleAnchors 从已提交章节中提取代表性段落作为风格锚点。
// maxCompletedChapter 由调用方传入，避免跨域依赖。
func (s *fileDraftStore) ExtractStyleAnchors(maxAnchors, maxCompletedChapter int) ([]string, error) {
	if maxAnchors <= 0 {
		maxAnchors = 5
	}

	var anchors []string
	var readErrs []error
	for ch := 1; ch <= maxCompletedChapter && len(anchors) < maxAnchors; ch++ {
		text, err := s.LoadChapterText(ch)
		if err != nil {
			readErrs = append(readErrs, fmt.Errorf("chapter %d: %w", ch, err))
			continue
		}
		if text == "" {
			continue
		}
		paragraphs := strings.Split(text, "\n\n")
		for _, para := range paragraphs {
			if len(anchors) >= maxAnchors {
				break
			}
			para = strings.TrimSpace(para)
			// 旧窗口 50-300 字按词换算：一个汉字 ≈ 一个越南语音节 ≈ 一个空白分隔词，
			// 故 1:1 取 50-300 词（≈225-1350 字符），既是一整段，也不吃掉太多上下文预算。
			words := domain.WordCount(para)
			if words < 50 || words > 300 {
				continue
			}
			// 对话密集段落不做风格锚点：旧实现只数中文左引号，越南语正文用的是
			// 直引号或弯引号，两种都算。
			if strings.Count(para, "\u201c")+strings.Count(para, "\"") > 2 {
				continue
			}
			anchors = append(anchors, para)
		}
	}
	return anchors, errors.Join(readErrs...)
}
