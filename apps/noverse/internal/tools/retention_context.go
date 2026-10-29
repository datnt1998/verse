package tools

import (
	"fmt"
	"path/filepath"

	"github.com/datnt1998/noverse/internal/library"
	"github.com/datnt1998/noverse/internal/store"
)

func (t *ContextTool) buildRetentionContext(target map[string]any, architect bool, reads *contextReads) {
	fields, err := t.store.Outline.LoadStoryRetention()
	if err != nil {
		reads.require("story_retention", err)
	} else if len(fields.NguoiNghe) > 0 || fields.DiemKhac != "" || fields.HatGiongPhanHai != "" {
		target["story_retention"] = fields
	}
	if !architect {
		return
	}
	stories, err := siblingStories(t.store)
	if err != nil {
		reads.warn("other_stories", err)
		return
	}
	type otherStory struct {
		Title   string `json:"title"`
		Premise string `json:"premise"`
	}
	others := make([]otherStory, 0, len(stories))
	for _, story := range stories {
		// SiblingStories đã loại truyện hiện tại khỏi nguồn đối chiếu.
		// 书库标准引擎目录；不扫描任意路径或读取其他项目。
		sibling := store.NewStore(filepath.Join(story.Dir, "output", "novel"))
		book, err := sibling.Book.Load()
		if err != nil {
			reads.warn(fmt.Sprintf("other_stories.%s.book", story.Name), err)
			continue
		}
		premise, err := sibling.Outline.LoadPremise()
		if err != nil {
			reads.warn(fmt.Sprintf("other_stories.%s.premise", story.Name), err)
			continue
		}
		title := story.Name
		if book != nil && book.Title != "" {
			title = book.Title
		} else if file, err := library.LoadStoryFile(story.Dir); err != nil {
			reads.warn(fmt.Sprintf("other_stories.%s", story.Name), err)
		} else if file != nil && file.Title != "" {
			title = file.Title
		}
		others = append(others, otherStory{Title: title, Premise: premise})
	}
	// Mảng rỗng nói rõ chưa có truyện trước, không buộc mô hình đoán từ khoá vắng mặt.
	target["other_stories"] = others
}
