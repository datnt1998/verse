package audio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func selectExportChapters(source []SourceChapter, volume int, chapters []int) ([]SourceChapter, []int, error) {
	if chapters == nil {
		return source, nil, nil
	}
	if len(chapters) == 0 {
		return nil, nil, conflict("Chọn ít nhất một chương để xuất.")
	}
	selected := make(map[int]bool, len(source))
	for _, chapter := range source {
		selected[chapter.Chapter] = false
	}
	for _, chapter := range chapters {
		if _, known := selected[chapter]; !known || chapter <= 0 {
			return nil, nil, conflict("Chương %d không thuộc Tập %d.", chapter, volume)
		}
		selected[chapter] = true
	}
	chosen := make([]SourceChapter, 0, min(len(chapters), len(source)))
	for _, chapter := range source {
		if selected[chapter.Chapter] {
			chosen = append(chosen, chapter)
		}
	}
	sort.Slice(chosen, func(i, j int) bool { return chosen[i].Chapter < chosen[j].Chapter })
	ids := make([]int, len(chosen))
	for i, chapter := range chosen {
		ids[i] = chapter.Chapter
	}
	return chosen, ids, nil
}

// Explicit selections always have a distinct name, even when they select the
// entire volume. Underscores distinguish disjoint ranges from contiguous ones.
func exportChapterSuffix(chapters []int) string {
	if chapters == nil {
		return ""
	}
	var name strings.Builder
	name.WriteString("-ch")
	for i := 0; i < len(chapters); {
		if i > 0 {
			name.WriteByte('_')
		}
		start := chapters[i]
		end := start
		i++
		for i < len(chapters) && chapters[i] == end+1 {
			end = chapters[i]
			i++
		}
		name.WriteString(itoa(start))
		if end != start {
			name.WriteByte('-')
			name.WriteString(itoa(end))
		}
	}
	return name.String()
}

// Keep follows host/exp's existing -YYYYMMDD-HHmmss naming convention.
// Audio historically replaces by default; callers can explicitly choose keep/fail.
func audioExportDestination(path, onExists string) (string, string, error) {
	if onExists == "" || onExists == "replace" {
		return path, "", nil
	}
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return path, "", nil
	}
	if err != nil {
		return "", "", err
	}
	if onExists == "fail" {
		return "", "", conflict("Tệp %s đã có. Chọn giữ cả hai hoặc thay thế.", filepath.Base(path))
	}
	stamp := "-" + time.Now().Format("20060102-150405")
	ext := filepath.Ext(path)
	candidate := strings.TrimSuffix(path, ext) + stamp + ext
	if _, err := os.Stat(candidate); err == nil {
		return "", "", conflict("Tệp %s đã có. Hãy thử lại sau một giây.", filepath.Base(candidate))
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("không kiểm tra được tệp xuất: %w", err)
	}
	return candidate, stamp, nil
}

func exportedFilesOnly(files []ExportFile, paths map[string]bool) []ExportFile {
	selected := files[:0]
	for _, file := range files {
		if paths[file.Path] {
			selected = append(selected, file)
		}
	}
	return selected
}
