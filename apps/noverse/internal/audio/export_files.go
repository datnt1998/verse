package audio

import (
	"archive/zip"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// annotateChapterFiles 给每章成品补显示名/时长/URL（Gói AF：磁盘名 =
// 显示名，"{Tên ngắn} · Tập V · Chương NN · {tên}( (bản thử)).{ext}"）。
func (s *Service) annotateChapterFiles(files []ExportFile, source []SourceChapter, durations []float64, volume int, format string, short string, trial bool) []ExportFile {
	out := make([]ExportFile, 0, len(source))
	for i, chapter := range source {
		name := exportChapterFileName(short, volume, chapter, trial, format)
		for _, file := range files {
			if file.Name != name {
				continue
			}
			file.Chapter = chapter.Chapter
			file.DurationSec = durations[i]
			file.SizeBytes = file.Size
			file.URL = s.exportFileURL(file.Path)
			out = append(out, file)
			break
		}
	}
	return out
}

func (s *Service) exportFileURL(path string) string {
	return s.base() + "/exports/file?path=" + url.QueryEscape(path)
}

func withChapterDetails(files, chapters []ExportFile) []ExportFile {
	for i := range files {
		for _, chapter := range chapters {
			if files[i].Path == chapter.Path {
				files[i] = chapter
				break
			}
		}
	}
	return files
}

func zipExportFiles(destination, root string, files []ExportFile) error {
	output, err := os.Create(destination)
	if err != nil {
		return err
	}
	archive := zip.NewWriter(output)
	for _, file := range files {
		input, err := os.Open(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			archive.Close()
			output.Close()
			return err
		}
		entry, err := archive.Create(strings.ReplaceAll(file.Name, "/", "-"))
		if err == nil {
			_, err = io.Copy(entry, input)
		}
		input.Close()
		if err != nil {
			archive.Close()
			output.Close()
			return err
		}
	}
	if err := archive.Close(); err != nil {
		output.Close()
		return err
	}
	return output.Close()
}
