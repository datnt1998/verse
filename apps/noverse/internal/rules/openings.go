package rules

import (
	"os"
	"path/filepath"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
)

// SiblingOpenings đọc chương một đã nộp của các truyện anh em, chỉ trong cùng
// dự án. engineDir là <story>/output/novel; không quét sách ngoài dự án, không
// dùng bản nháp. Truyện cũ không có project/story metadata thì bỏ qua phép so.
func SiblingOpenings(engineDir string, chapter int) []Opening {
	if chapter != 1 {
		return nil
	}
	storyDir := filepath.Dir(filepath.Dir(engineDir))
	projectDir := library.FindProjectDir(storyDir)
	if projectDir == "" || projectDir == storyDir {
		return nil
	}
	if _, err := os.Stat(filepath.Join(storyDir, library.StoryFileName)); err != nil {
		return nil
	}
	parent := filepath.Dir(storyDir)
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil
	}
	var openings []Opening
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == filepath.Base(storyDir) {
			continue
		}
		dir := filepath.Join(parent, entry.Name())
		if _, err := os.Stat(filepath.Join(dir, library.StoryFileName)); err != nil {
			continue
		}
		if library.FindProjectDir(dir) != projectDir {
			continue
		}
		text, err := os.ReadFile(filepath.Join(dir, "output", "novel", "chapters", "01.md"))
		if err != nil || len(text) == 0 {
			continue
		}
		openings = append(openings, Opening{Name: entry.Name(), Text: metricChapterBody(string(text))})
	}
	return openings
}

// SiblingOpeningsFor is the store-aware entry point: an empty engineDir means a
// hosted store with no engine directory, which has no sibling story files to
// compare. Callers pass store.Dir() and never invent a directory.
func SiblingOpeningsFor(engineDir string, chapter int) []Opening {
	if engineDir == "" {
		return nil
	}
	return SiblingOpenings(engineDir, chapter)
}
