package audio

// The chapter cache resumes interrupted work. This cache instead remembers a
// finished export: unchanged input returns it directly, metadata changes only
// retag/copy containers. Both survive a service restart.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type exportFileStamp struct {
	Size     int64 `json:"size"`
	Modified int64 `json:"modified"`
}
type exportResultEntry struct {
	TagsKey string                     `json:"tagsKey"`
	Result  ExportResult               `json:"result"`
	Meta    exportMeta                 `json:"meta"`
	Stamps  map[string]exportFileStamp `json:"stamps"`
}
type exportResults map[string]exportResultEntry

func (s *Service) exportResultsPath(volume int) string {
	return filepath.Join(s.exportCacheDir(volume), "results.json")
}
func (s *Service) readExportResults(volume int) exportResults {
	var entries exportResults
	data, err := os.ReadFile(s.exportResultsPath(volume))
	if err != nil || json.Unmarshal(data, &entries) != nil || entries == nil {
		return exportResults{}
	}
	return entries
}
func (s *Service) resultFilesValid(entry exportResultEntry) bool {
	if len(entry.Result.Files) == 0 || len(entry.Stamps) != len(entry.Result.Files) || len(entry.Meta.Paths) != len(entry.Result.Files) {
		return false
	}
	hasAudio := false
	for _, file := range entry.Result.Files {
		hasAudio = hasAudio || file.Format == "audio"
		stamp, ok := entry.Stamps[file.Path]
		info, err := os.Stat(filepath.Join(s.layout.ExportsDir, filepath.FromSlash(file.Path)))
		if !ok || err != nil || !info.Mode().IsRegular() || info.Size() != stamp.Size || info.ModTime().UnixNano() != stamp.Modified {
			return false
		}
	}
	for _, chapter := range entry.Result.ChapterFiles {
		if chapter.Chapter <= 0 || chapter.DurationSec <= 0 {
			return false
		}
	}
	return hasAudio
}
func (s *Service) saveExportResult(volume int, key, tagsKey string, result ExportResult) {
	data, err := os.ReadFile(filepath.Join(s.layout.VolumeExportDir(volume), exportMetaFile))
	if err != nil {
		s.logf("Không đọc được thông tin bộ đệm bản xuất", "warn", err)
		return
	}
	entry := exportResultEntry{TagsKey: tagsKey, Result: result, Stamps: map[string]exportFileStamp{}}
	if err = json.Unmarshal(data, &entry.Meta); err != nil {
		s.logf("Không đọc được thông tin bộ đệm bản xuất", "warn", err)
		return
	}
	for _, file := range result.Files {
		info, err := os.Stat(filepath.Join(s.layout.ExportsDir, filepath.FromSlash(file.Path)))
		if err != nil {
			return
		}
		entry.Stamps[file.Path] = exportFileStamp{Size: info.Size(), Modified: info.ModTime().UnixNano()}
	}
	entries := s.readExportResults(volume)
	for oldKey, old := range entries {
		if !s.resultFilesValid(old) {
			delete(entries, oldKey)
		}
	}
	entries[key] = entry
	data, err = json.Marshal(entries)
	if err == nil {
		err = writeFileAtomic(s.exportResultsPath(volume), data)
	}
	if err != nil {
		s.logf("Không lưu được bộ đệm bản xuất", "warn", err)
	}
}

func (s *Service) exportContentKey(volume int, source []SourceChapter, scripts []*Script, opts ExportOptions, cast *Cast, settings Settings, engine, model string) string {
	h := sha256.New()
	fmt.Fprintf(h, "finished-v3\x00%d\x00%d\x00%s\x00%s", exportCacheVersion, volume, ffmpegBin, settings.Quality)
	// Selection identity comes from the actual chapters, not -ch filename suffixes.
	opts.Chapters, opts.OnExists, opts.WhenReady = nil, "", false
	_ = json.NewEncoder(h).Encode(opts)
	if opts.ChapterTitles {
		_ = json.NewEncoder(h).Encode(settings)
		_ = json.NewEncoder(h).Encode(cast)
		fmt.Fprintf(h, "\x00%s\x00%s", engine, model)
	}
	for i, sc := range source {
		fmt.Fprintln(h, s.exportWavKey(volume, sc, scripts[i], cast, ""))
		if opts.ChapterTitles {
			fmt.Fprintf(h, "\x00%s", sc.Title)
		}
		if opts.Subtitles {
			for _, seg := range scripts[i].Segments {
				fmt.Fprintf(h, "\x00%s", seg.Text)
			}
		}
	}
	if opts.Intro {
		intro, outro, _ := pickMusic(s.layout.MusicDir())
		for _, path := range []string{intro, outro} {
			fmt.Fprintf(h, "\x00%s", path)
			if info, err := os.Stat(path); err == nil {
				fmt.Fprintf(h, "\x00%d\x00%d", info.Size(), info.ModTime().UnixNano())
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}
func exportTagsKey(title, short string, source []SourceChapter, total int, cover string, opts ExportOptions) string {
	h := sha256.New()
	fmt.Fprintf(h, "tags-v3\x00%s\x00%s\x00%s\x00%d\x00%d\x00%s", title, short, cover, total, time.Now().Year(), exportChapterSuffix(opts.Chapters))
	for _, sc := range source {
		fmt.Fprintf(h, "\x00%d\x00%s", sc.Chapter, sc.Title)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Service) reuseExportResult(ctx context.Context, volume int, key, tagsKey, short string, source []SourceChapter, scripts []*Script, cast *Cast, opts ExportOptions, volumeFile, chapterDir, suffix, cover string, chapterTags func(SourceChapter, bool) tagSet, volumeTags tagSet) (ExportResult, bool, error) {
	entry, ok := s.readExportResults(volume)[key]
	if !ok || !s.resultFilesValid(entry) {
		return ExportResult{}, false, nil
	}
	if entry.TagsKey == tagsKey && opts.OnExists != "keep" {
		data, err := json.Marshal(entry.Meta)
		if err == nil {
			err = writeFileAtomic(filepath.Join(s.layout.VolumeExportDir(volume), exportMetaFile), data)
		}
		return entry.Result, true, err
	}
	// Copy the value slices before changing paths: retained exports stay valid.
	result := entry.Result
	result.Files = append([]ExportFile(nil), result.Files...)
	result.ChapterFiles = append([]ExportFile(nil), result.ChapterFiles...)
	result.ChapterMarks = append([]ChapterMark(nil), result.ChapterMarks...)
	for i := range result.ChapterMarks {
		var chapter int
		if _, err := fmt.Sscanf(result.ChapterMarks[i].Label, "Chương %d", &chapter); err == nil {
			for _, sc := range source {
				if sc.Chapter == chapter {
					result.ChapterMarks[i].Label = chapterTitleTag(sc, false)
					break
				}
			}
		}
	}
	chapterByPath := make(map[string]ExportFile, len(result.ChapterFiles))
	for _, file := range result.ChapterFiles {
		chapterByPath[file.Path] = file
	}
	newChapters := make([]ExportFile, 0, len(result.ChapterFiles))
	files := make([]ExportFile, 0, len(result.Files))
	var oldZip string
	for _, file := range result.Files {
		oldPath := filepath.Join(s.layout.ExportsDir, filepath.FromSlash(file.Path))
		destination := oldPath
		_, isChapterFile := chapterByPath[file.Path]
		tags := volumeTags
		marks := result.ChapterMarks
		format := opts.Format
		if chapter, isChapter := chapterByPath[file.Path]; isChapter {
			file = chapter
			for _, sc := range source {
				if sc.Chapter == chapter.Chapter {
					format = strings.TrimPrefix(filepath.Ext(file.Name), ".")
					destination = filepath.Join(chapterDir, exportChapterFileName(short, volume, sc, opts.Trial, format))
					tags = chapterTags(sc, opts.Trial)
					break
				}
			}
			marks = nil
		} else if file.Format == "audio" {
			destination = volumeFile
		} else if strings.HasSuffix(file.Name, ".zip") {
			oldZip = file.Path
			continue
		} else if strings.HasSuffix(file.Name, ".srt") && opts.Split {
			// Chapter subtitles use the same base as their audio companion.
			for _, chapter := range result.ChapterFiles {
				if strings.TrimSuffix(chapter.Path, filepath.Ext(chapter.Path)) == strings.TrimSuffix(file.Path, ".srt") {
					for _, sc := range source {
						if sc.Chapter == chapter.Chapter {
							destination = filepath.Join(chapterDir, strings.TrimSuffix(exportChapterFileName(short, volume, sc, opts.Trial, opts.Format), "."+opts.Format)+".srt")
							break
						}
					}
				}
			}
		} else {
			for _, ext := range []string{".srt", ".ban-doc.txt", ".moc-chuong.txt"} {
				if strings.HasSuffix(file.Name, ext) {
					destination = filepath.Join(s.layout.VolumeExportDir(volume), fmt.Sprintf("Tập %d%s%s", volume, suffix, ext))
					break
				}
			}
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return ExportResult{}, true, err
		}
		if destination != oldPath {
			if err := copyFile(oldPath, destination); err != nil {
				return ExportResult{}, true, err
			}
		}
		if file.Format == "audio" && entry.TagsKey != tagsKey {
			if err := retagAudio(ctx, destination, format, tagSpec{tags: tags, coverPNG: cover, marks: marks, totalSec: result.DurationSec}); err != nil {
				return ExportResult{}, true, err
			}
		}
		if strings.HasSuffix(file.Name, ".ban-doc.txt") {
			if err := writeFileAtomic(destination, []byte(renderScriptText(cast, scripts, source))); err != nil {
				return ExportResult{}, true, err
			}
		}
		if strings.HasSuffix(file.Name, ".moc-chuong.txt") {
			if err := writeFileAtomic(destination, []byte(renderMarks(result.ChapterMarks))); err != nil {
				return ExportResult{}, true, err
			}
		}
		relative, _ := filepath.Rel(s.layout.ExportsDir, destination)
		file.Path, file.Name = filepath.ToSlash(relative), filepath.Base(destination)
		info, err := os.Stat(destination)
		if err != nil {
			return ExportResult{}, true, err
		}
		at := info.ModTime()
		file.UpdatedAt = &at
		file.Size = info.Size()
		file.SizeBytes = info.Size()
		file.URL = s.exportFileURL(file.Path)
		files = append(files, file)
		if isChapterFile {
			newChapters = append(newChapters, file)
		}
	}
	result.Files, result.ChapterFiles = files, newChapters
	if oldZip != "" {
		destination := filepath.Join(s.layout.VolumeExportDir(volume), exportZipName(short, volume, len(source)))
		if opts.OnExists == "keep" {
			destination = strings.TrimSuffix(destination, ".zip") + strings.TrimPrefix(suffix, exportChapterSuffix(opts.Chapters)) + ".zip"
		}
		if err := zipExportFiles(destination, s.layout.ExportsDir, result.Files); err != nil {
			return ExportResult{}, true, err
		}
		info, err := os.Stat(destination)
		if err != nil {
			return ExportResult{}, true, err
		}
		relative, _ := filepath.Rel(s.layout.ExportsDir, destination)
		at := info.ModTime()
		zipFile := ExportFile{Path: filepath.ToSlash(relative), Name: filepath.Base(destination), Format: "other", Size: info.Size(), SizeBytes: info.Size(), UpdatedAt: &at, Volume: &volume}
		result.ZipURL = s.exportFileURL(zipFile.Path)
		zipFile.URL = result.ZipURL
		result.Files = append(result.Files, zipFile)
	}
	meta := entry.Meta
	meta.ExportedAt = time.Now()
	meta.Paths = nil
	meta.ChapterFiles = result.ChapterFiles
	meta.ChapterMarks = result.ChapterMarks
	meta.ZipURL = result.ZipURL
	for _, file := range result.Files {
		meta.Paths = append(meta.Paths, file.Path)
	}
	data, err := json.Marshal(meta)
	if err == nil {
		err = writeFileAtomic(filepath.Join(s.layout.VolumeExportDir(volume), exportMetaFile), data)
	}
	if err != nil {
		return ExportResult{}, true, err
	}
	s.saveExportResult(volume, key, tagsKey, result)
	return result, true, nil
}

// MP3 tags are replaced in Go; MP4/WAV are stream-copied, never encoded or
// concatenated. M4B chapter timing is retained while labels and artwork change.
func retagAudio(ctx context.Context, path, format string, spec tagSpec) error {
	if format == "mp3" {
		return postTag(ctx, path, format, spec)
	}
	dir, err := os.MkdirTemp(filepath.Dir(path), ".retag-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	tmp, err := os.CreateTemp(filepath.Dir(path), ".retag-*."+format)
	if err != nil {
		return err
	}
	destination := tmp.Name()
	defer os.Remove(destination)
	if err := tmp.Close(); err != nil {
		return err
	}
	metadata := filepath.Join(dir, "chapters.ffmetadata")
	if err := os.WriteFile(metadata, []byte(chapterMetadata(spec.marks, spec.totalSec)), 0o600); err != nil {
		return err
	}
	args := []string{"-i", path, "-f", "ffmetadata", "-i", metadata}
	if format == "m4b" && spec.coverPNG != "" {
		args = append(args, "-i", spec.coverPNG)
	}
	args = append(args, "-map", "0:a", "-map_metadata", "1", "-map_chapters", "1", "-c:a", "copy")
	args = append(args, tagArgs(spec.tags)...)
	if format == "m4b" {
		if spec.coverPNG != "" {
			args = append(args, "-map", "2:v", "-c:v", "copy", "-disposition:v:0", "attached_pic")
		}
		args = append(args, "-metadata", "media_type="+m4bMediaType, "-f", "mp4", "-movflags", "+faststart")
	}
	args = append(args, destination)
	if err := runFFmpeg(ctx, args...); err != nil {
		return err
	}
	if err := postTag(ctx, destination, format, spec); err != nil {
		return err
	}
	return publishExportFile(destination, path)
}
