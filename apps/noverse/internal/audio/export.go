package audio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// 导出：把"已生成的段"拼成一卷的成品（10-audio §2 A3、§3）。
// 这一步不读任何新文字——只拼接、加音乐、读章名、平衡音量、编格式、写字幕/清单。
// 因此它很快（"Ghép mất khoảng 1 phút"），也绝不会改动 bản đọc。

// ExportOptions 对应 POST S/audio/volumes/:v/exports。
type ExportOptions struct {
	Format        string  `json:"format"` // m4b | mp3 | wav
	ChapterMarks  bool    `json:"chapterMarks"`
	Subtitles     bool    `json:"subtitles"`
	WhenReady     bool    `json:"whenReady"`
	Chapters      []int   `json:"chapters,omitempty"`
	Split         bool    `json:"split,omitempty"`
	Pack          string  `json:"pack,omitempty"`
	Trial         bool    `json:"trial,omitempty"`
	OnExists      string  `json:"onExists,omitempty"` // replace (default) | keep | fail
	Intro         bool    `json:"intro"`
	ChapterTitles bool    `json:"chapterTitles"`
	Loudness      bool    `json:"loudness"`
	GapSec        float64 `json:"gapSec"`
}

// ExportFile 与 storyread 的 ExportFile 同形（path 相对导出目录）。
type ExportFile struct {
	Path        string     `json:"path"`
	Name        string     `json:"name"`
	Format      string     `json:"format"`
	Size        int64      `json:"size"`
	UpdatedAt   *time.Time `json:"updatedAt"`
	Volume      *int       `json:"volume"`
	Chapter     int        `json:"chapter,omitempty"`
	SizeBytes   int64      `json:"sizeBytes"`
	DurationSec float64    `json:"durationSec,omitempty"`
	URL         string     `json:"url,omitempty"`
}

// ExportResult 对应导出响应。
type ExportResult struct {
	Files        []ExportFile  `json:"files"`
	ChapterMarks []ChapterMark `json:"chapterMarks"`
	DurationSec  float64       `json:"durationSec"`
	Pending      bool          `json:"pending,omitempty"`
	ChapterFiles []ExportFile  `json:"chapterFiles"`
	Split        bool          `json:"split,omitempty"`
	ZipURL       string        `json:"zipUrl,omitempty"`
}

// ExportInfo 对应 GET S/audio/volumes/:v/exports（"đã xuất" 状态页）。
type ExportInfo struct {
	Volume       int           `json:"volume"`
	Dir          string        `json:"dir"` // 相对故事目录，给"mở thư mục"用
	TotalBytes   int64         `json:"totalBytes"`
	Files        []ExportFile  `json:"files"`
	ChapterMarks []ChapterMark `json:"chapterMarks"`
	ExportedAt   *time.Time    `json:"exportedAt"`
	DurationSec  float64       `json:"durationSec"`
	Voices       int           `json:"voices"`
	ChapterCount int           `json:"chapterCount"`
	Format       string        `json:"format,omitempty"`
	Peaks        []float64     `json:"peaks,omitempty"` // 成品波形，2 点/秒，0..1
	ChapterFiles []ExportFile  `json:"chapterFiles"`
	Split        bool          `json:"split,omitempty"`
	ZipURL       string        `json:"zipUrl,omitempty"`
}

// exportMeta 是导出目录里的隐藏清单（列表接口跳过隐藏文件）。
type exportMeta struct {
	ExportedAt   time.Time     `json:"exportedAt"`
	DurationSec  float64       `json:"durationSec"`
	Voices       int           `json:"voices"`
	ChapterCount int           `json:"chapterCount"`
	Format       string        `json:"format"`
	ChapterMarks []ChapterMark `json:"chapterMarks"`
	Peaks        []float64     `json:"peaks"`
	Paths        []string      `json:"paths,omitempty"`
	ChapterFiles []ExportFile  `json:"chapterFiles"`
	Split        bool          `json:"split,omitempty"`
	ZipURL       string        `json:"zipUrl,omitempty"`
}

const exportMetaFile = ".audio-export.json"

// Export 拼一卷并写出全部成品。
// Export 拼一卷并写出全部成品。同步执行版（内部用）；网页层走
// StartExport → 后台任务（§AE），进度经 exportReporter 上报。
func (s *Service) Export(ctx context.Context, volume int, bookTitle string, source []SourceChapter, opts ExportOptions) (ExportResult, error) {
	return s.runExport(ctx, volume, bookTitle, source, opts, nil)
}

// normalizeExportOptions 做"纯参数"校验与章节选择（不进 ffmpeg、不读段缓存），
// StartExport 同步调用它保证表单错误立刻 409，而不是等后台任务失败。
func normalizeExportOptions(source []SourceChapter, volume int, opts ExportOptions) ([]SourceChapter, []int, ExportOptions, error) {
	if opts.Format != "mp3" && opts.Format != "wav" && opts.Format != "m4b" {
		return nil, nil, opts, conflict("format phải là m4b, mp3 hoặc wav")
	}
	if opts.OnExists != "" && opts.OnExists != "replace" && opts.OnExists != "keep" && opts.OnExists != "fail" {
		return nil, nil, opts, conflict("onExists phải là replace, keep hoặc fail")
	}
	if opts.Pack != "" && opts.Pack != "files" && opts.Pack != "zip" {
		return nil, nil, opts, conflict("Cách nhận tệp không hợp lệ.")
	}
	if opts.Split {
		if len(opts.Chapters) == 0 {
			return nil, nil, opts, conflict("Chọn ít nhất một chương để xuất.")
		}
		opts.Intro, opts.GapSec, opts.ChapterMarks = false, 0, false
	}
	opts.ChapterMarks = opts.ChapterMarks && opts.Format == "m4b" && !opts.Split
	selected, chapters, err := selectExportChapters(source, volume, opts.Chapters)
	if err != nil {
		return nil, nil, opts, err
	}
	source, opts.Chapters = selected, chapters
	if opts.Trial && (len(chapters) != 1 || opts.WhenReady) {
		return nil, nil, opts, conflict("Bản thử chỉ xuất một chương đã có giọng.")
	}
	if opts.GapSec < 0 {
		opts.GapSec = 0
	}
	if opts.GapSec > 10 {
		opts.GapSec = 10
	}
	if len(source) == 0 {
		return nil, nil, opts, conflict("Tập này chưa có chương nào để ghép.")
	}
	return source, chapters, opts, nil
}

// exportVolumeName 算成品文件名（与 runExport 的命名一致，StartExport 拿它做
// onExists=fail 的同步检查）。Gói AF：{Tên ngắn} · Tập V( · …).{ext}。
func (s *Service) exportVolumeName(volume int, bookTitle string, source []SourceChapter, opts ExportOptions) string {
	_, short := s.storyMeta(bookTitle)
	if opts.Trial {
		for _, sc := range source {
			if sc.Chapter == opts.Chapters[0] {
				return exportChapterFileName(short, volume, sc, true, opts.Format)
			}
		}
	}
	return exportVolumeFileName(short, volume, opts.Chapters, opts.Format)
}

// runExport 是导出主体；prog != nil 时按章/段/编码进度上报（≤4 次/秒、单调）。
func (s *Service) runExport(ctx context.Context, volume int, bookTitle string, source []SourceChapter, opts ExportOptions, prog *exportReporter) (ExportResult, error) {
	volumeSource := source
	selected, _, opts, err := normalizeExportOptions(source, volume, opts)
	if err != nil {
		return ExportResult{}, err
	}
	source = selected
	if !FFmpegAvailable() {
		return ExportResult{}, &ExportStageError{Cause: "ffmpeg_missing", Msg: ErrFFmpegMissing.Error()}
	}
	if opts.WhenReady {
		return s.scheduleExport(volume, bookTitle, volumeSource, opts)
	}
	cast, err := s.castForRender()
	if err != nil {
		return ExportResult{}, err
	}
	voices, err := s.Voices(ctx)
	if err != nil {
		return ExportResult{}, err
	}
	voiceMap := VoiceMap(voices)
	settings := s.Settings()
	engine, model := s.engineIdent(ctx)

	scripts := make([]*Script, 0, len(source))
	for _, sc := range source {
		script, err := ReadScript(s.layout, volume, sc.Chapter)
		if err != nil {
			return ExportResult{}, &ExportStageError{Cause: "script_missing", Chapter: sc.Chapter, Msg: fmt.Sprintf("Chương %d chưa có bản đọc.", sc.Chapter), Err: err}
		}
		if script == nil || len(script.Segments) == 0 {
			return ExportResult{}, &ExportStageError{Cause: "script_missing", Chapter: sc.Chapter, Msg: fmt.Sprintf("Chương %d chưa có bản đọc.", sc.Chapter)}
		}
		for _, seg := range script.Segments {
			if seg.Rendered == nil || seg.Rendered.DurationSec <= 0 {
				return ExportResult{}, &ExportStageError{Cause: "segments_missing", Chapter: sc.Chapter, Msg: fmt.Sprintf("Chương %d còn đoạn chưa có giọng đọc — tạo giọng xong mới ghép được.", sc.Chapter)}
			}
		}
		scripts = append(scripts, script)
	}
	if prog != nil {
		units := 0.0
		for _, script := range scripts {
			// 每章：N 段 + 1 次拼接 + 1 次编码。
			units += float64(len(script.Segments)) + 2
		}
		prog.begin(units, opts.Split)
	} else if s.hub != nil {
		s.hub.Publish(NewEvent(EventExport, map[string]any{"volume": volume, "percent": float64(0), "state": "rendering"}))
	}
	s.buildMu.Lock()
	defer s.buildMu.Unlock()

	dir, err := os.MkdirTemp("", "ainovel-export-*")
	if err != nil {
		return ExportResult{}, err
	}
	defer os.RemoveAll(dir)
	exportDir := s.layout.VolumeExportDir(volume)
	storyTitle, short := s.storyMeta(bookTitle)
	if storyTitle == "" {
		storyTitle = "Tập " + itoa(volume)
	}
	if short == "" {
		short = storyTitle
	}
	// Gói AF/A7：文件名统一 "{Tên ngắn} · Tập V[ · …]"（含 keep 后缀仅
	// 追加在成品文件名与章目录后缀上，不影响显示名）。
	volumeFile := filepath.Join(exportDir, s.exportVolumeName(volume, bookTitle, source, opts))
	volumeFile, keepSuffix, err := audioExportDestination(volumeFile, opts.OnExists)
	if err != nil {
		return ExportResult{}, err
	}
	suffix := exportChapterSuffix(opts.Chapters) + keepSuffix
	chapterDir := filepath.Join(exportDir, "chuong"+suffix)
	produced := make(map[string]bool, len(source)+4)
	record := func(path string) {
		relative, _ := filepath.Rel(s.layout.ExportsDir, path)
		produced[filepath.ToSlash(relative)] = true
	}
	if err := os.MkdirAll(chapterDir, 0o755); err != nil {
		return ExportResult{}, err
	}

	// Whole-volume exports retain their MP3 chapter companions. The selected
	// format applies to each chapter only for an explicit split export.
	chapterFormat := "mp3"
	if opts.Split {
		chapterFormat = opts.Format
	}
	// Gói AF：每个成品文件都要内嵌封面（APIC/covr/id3）。cover 解析失败
	// （字体/bìa 解码坏）不致命 —— 记日志继续，封面字段留空。
	coverPNG, coverErr := s.layout.CoverPNG(short, volume, coverMasterSize)
	if coverErr != nil {
		s.logf("Không dựng được bìa cho tệp xuất", "warn", coverErr)
		coverPNG = ""
	}
	coverFP := ""
	if coverPNG != "" {
		if data, err := os.ReadFile(coverPNG); err == nil {
			coverFP = tagFingerprint(data)
		}
	}
	album := fmt.Sprintf("%s · Tập %d", storyTitle, volume)
	totalCh := len(volumeSource)
	chapterTags := func(sc SourceChapter, trial bool) tagSet {
		track := 1
		for i, chapter := range volumeSource {
			if chapter.Chapter == sc.Chapter {
				track = i + 1
				break
			}
		}
		return tagSet{
			Title:   chapterTitleTag(sc, trial),
			Artist:  ArtistAudio,
			Album:   album,
			Track:   track,
			Total:   totalCh,
			Disc:    volume,
			Genre:   genreAudiobook,
			Year:    time.Now().Year(),
			Comment: commentAudio,
			Lang:    languageAudio,
		}
	}
	volumeTags := tagSet{
		Title:   fmt.Sprintf("%s · Tập %d", short, volume),
		Artist:  ArtistAudio,
		Album:   album,
		Track:   1,
		Total:   1,
		Disc:    volume,
		Genre:   genreAudiobook,
		Year:    time.Now().Year(),
		Comment: commentAudio,
		Lang:    languageAudio,
	}
	if opts.Trial {
		volumeTags = chapterTags(source[0], true)
	}
	contentKey := s.exportContentKey(volume, source, scripts, opts, cast, settings, engine, model)
	tagsKey := exportTagsKey(storyTitle, short, source, totalCh, coverFP, opts)
	if cached, ok, err := s.reuseExportResult(ctx, volume, contentKey, tagsKey, short, source, scripts, cast, opts, volumeFile, chapterDir, suffix, coverPNG, chapterTags, volumeTags); ok || err != nil {
		return cached, err
	}
	if opts.ChapterTitles {
		if st := s.synth.Status(ctx); !synthUsable(st) {
			return ExportResult{}, &ExportStageError{Cause: "engine", Msg: ErrEngineMissing.Error()}
		}
	}
	finishResult := func(result ExportResult) (ExportResult, error) {
		s.saveExportResult(volume, contentKey, tagsKey, result)
		return result, nil
	}
	// 1) 每章：章名（可选）+ 段 + 停顿，各自导出 mp3。
	type chapterBuild struct {
		source  SourceChapter
		script  *Script
		wav     string
		startAt float64
		dur     float64
	}
	builds := make([]chapterBuild, 0, len(scripts))
	chapterDurations := make([]float64, 0, len(scripts))
	introDur := 0.0
	musicIntro, musicOutro, err := pickMusic(s.layout.MusicDir())
	if err != nil {
		return ExportResult{}, &ExportStageError{Cause: "music_decode", Msg: "Không đọc được thư mục nhạc.", Err: err}
	}
	firstChapter := source[0].Chapter
	if opts.Intro && musicIntro == "" {
		return ExportResult{}, &ExportStageError{Cause: "intro_missing", Chapter: firstChapter,
			Msg: "Chưa có nhạc mở đầu trong thư mục nhạc — thêm nhạc, hoặc bấm “Ghép không nhạc”."}
	}
	cursor := 0.0
	marks := make([]ChapterMark, 0, len(scripts)+1)
	var musicInputs []string
	if opts.Intro && musicIntro != "" {
		prog.step("Đang xử lý nhạc mở đầu")
		pcm := filepath.Join(dir, "intro.wav")
		if err := toPCM(ctx, musicIntro, pcm); err != nil {
			return ExportResult{}, &ExportStageError{Cause: "music_decode", Chapter: firstChapter,
				Msg: "Nhạc mở đầu không đọc được — đổi tệp nhạc, hoặc bấm “Ghép không nhạc”.", Err: err}
		}
		musicInputs = append(musicInputs, pcm)
		introDur, _ = WAVDurationSecFile(pcm)
		cursor += introDur
		marks = append(marks, ChapterMark{AtSec: 0, Label: "Mở đầu"})
	}

	var srtItems []SRTItem
	voicesUsed := map[string]bool{}
	cacheDir := s.exportCacheDir(volume)
	cache := s.readExportCache(volume)
	nextCache := &exportCacheManifest{Chapters: map[int]*exportCacheEntry{}}
	for i, script := range scripts {
		sc := source[i]
		start := cursor
		segUnits := len(script.Segments)
		// 章名朗读先算出来：它是 wav 指纹的一部分（标题文本 + 引擎缓存的音频）。
		titlePath, titlePCM := "", ""
		titleDur := 0.0
		if opts.ChapterTitles && strings.TrimSpace(sc.Title) != "" {
			prog.step(fmt.Sprintf("Đang đọc tên chương %d", sc.Chapter))
			path, dur, terr := s.chapterTitleAudio(ctx, volume, sc.Chapter, sc.Title, cast, voiceMap, settings, engine, model)
			if terr != nil {
				return ExportResult{}, &ExportStageError{Cause: "engine", Chapter: sc.Chapter,
					Msg: fmt.Sprintf("Không đọc được tên chương %d: %s", sc.Chapter, terr.Error()), Err: terr}
			}
			if dur > 0 {
				titlePath = path
				titleDur = dur
			}
		}
		wavKey := s.exportWavKey(volume, sc, script, cast, titlePath)
		encKey := exportEncKey(wavKey, chapterFormat, opts.Loudness, fmt.Sprintf("%s:%s:%+v", settings.Quality, coverFP, chapterTags(sc, opts.Trial)))
		segTimed := make([]TimedSegment, 0, len(script.Segments))
		for _, seg := range script.Segments {
			pause := effectivePauseMs(cast, seg)
			segTimed = append(segTimed, TimedSegment{
				Text:        seg.Text,
				DurationSec: seg.Rendered.DurationSec,
				PauseMs:     pause,
			})
			voicesUsed[seg.Speaker] = true
		}
		marks = append(marks, ChapterMark{AtSec: start, Label: fmt.Sprintf("Chương %d · %s", sc.Chapter, sc.Title)})

		chapterWav := filepath.Join(dir, fmt.Sprintf("chapter-%02d.wav", sc.Chapter))
		chapterFile := filepath.Join(chapterDir, exportChapterFileName(short, volume, sc, opts.Trial, chapterFormat))
		var chapterDur float64
		entry := &exportCacheEntry{WavKey: wavKey, EncKey: encKey, Wav: exportCacheFileName(sc.Chapter, wavKey, "wav"),
			EncFmt: chapterFormat, BuiltAt: time.Now()}
		hit := cache.lookup(volume, sc.Chapter, cacheDir, wavKey, encKey, chapterFormat)
		if hit != nil {
			// 缓存命中：重跑/重试只复用 wav 和（指纹吻合的）成品编码。
			chapterWav = filepath.Join(cacheDir, hit.Wav)
			chapterDur = hit.Dur
			prog.step(fmt.Sprintf("Đang ghép chương %d · giữ lại phần đã ghép", sc.Chapter))
			if hit.Enc != "" {
				if err := copyFile(filepath.Join(cacheDir, hit.Enc), chapterFile); err != nil {
					return ExportResult{}, &ExportStageError{Cause: "ffmpeg", Chapter: sc.Chapter, Msg: fmt.Sprintf("Không ghi được tệp chương %d.", sc.Chapter), Err: err}
				}
				entry.Enc = hit.Enc
			} else {
				prog.step(fmt.Sprintf("Đang mã hoá chương %d", sc.Chapter))
				if err := encodeAudioTagged(ctx, chapterWav, chapterFile, chapterFormat, opts.Loudness,
					tagSpec{tags: chapterTags(sc, opts.Trial), coverPNG: coverPNG, quality: settings.Quality}, ""); err != nil {
					return ExportResult{}, &ExportStageError{Cause: "ffmpeg", Chapter: sc.Chapter, Msg: fmt.Sprintf("Không mã hoá được chương %d.", sc.Chapter), Err: err}
				}
				entry.Enc = exportCacheFileName(sc.Chapter, encKey, chapterFormat)
				if err := copyFile(chapterFile, filepath.Join(cacheDir, entry.Enc)); err != nil {
					// 副本失败不致命：wav 已缓存，下次只重编码。
					entry.Enc = ""
				}
			}
			entry.Dur = hit.Dur
			prog.advance(float64(segUnits) + 2)
		} else {
			inputs := make([]string, 0, len(script.Segments)*2+2)
			if titlePath != "" {
				titlePCM = filepath.Join(dir, fmt.Sprintf("title-%02d.wav", sc.Chapter))
				if err := toPCM(ctx, titlePath, titlePCM); err != nil {
					return ExportResult{}, &ExportStageError{Cause: "ffmpeg", Chapter: sc.Chapter, Msg: fmt.Sprintf("Không ghép được tên chương %d.", sc.Chapter), Err: err}
				}
				inputs = append(inputs, titlePCM)
			}
			for j, seg := range script.Segments {
				prog.step(fmt.Sprintf("Đang ghép chương %d · đoạn %d/%d", sc.Chapter, j+1, segUnits))
				pcm := filepath.Join(dir, fmt.Sprintf("c%02d-s%04d.wav", sc.Chapter, j))
				if err := toPCM(ctx, s.layout.CachePath(volume, sc.Chapter, seg.Rendered.Key), pcm); err != nil {
					return ExportResult{}, &ExportStageError{Cause: "ffmpeg", Chapter: sc.Chapter, Msg: fmt.Sprintf("Không ghép được đoạn %d của chương %d.", j+1, sc.Chapter), Err: err}
				}
				inputs = append(inputs, pcm)
				pause := effectivePauseMs(cast, seg)
				if pause > 0 {
					sil := filepath.Join(dir, fmt.Sprintf("c%02d-p%04d.wav", sc.Chapter, j))
					if err := writeSilenceWAV(sil, pause); err != nil {
						return ExportResult{}, err
					}
					inputs = append(inputs, sil)
				}
				prog.advance(1)
			}
			chapterTotal := titleDur
			for _, st := range segTimed {
				chapterTotal += st.DurationSec + float64(st.PauseMs)/1000
			}
			prog.step(fmt.Sprintf("Đang nối chương %d", sc.Chapter))
			if err := concatWAVsProgress(ctx, inputs, chapterWav, chapterTotal, func(f float64) { prog.frac(f) }); err != nil {
				return ExportResult{}, &ExportStageError{Cause: "ffmpeg", Chapter: sc.Chapter, Msg: fmt.Sprintf("Không nối được chương %d.", sc.Chapter), Err: err}
			}
			chapterDur, _ = WAVDurationSecFile(chapterWav)
			prog.advance(1)
			prog.step(fmt.Sprintf("Đang mã hoá chương %d", sc.Chapter))
			if err := encodeAudioTagged(ctx, chapterWav, chapterFile, chapterFormat, opts.Loudness,
				tagSpec{tags: chapterTags(sc, opts.Trial), coverPNG: coverPNG, quality: settings.Quality}, ""); err != nil {
				return ExportResult{}, &ExportStageError{Cause: "ffmpeg", Chapter: sc.Chapter, Msg: fmt.Sprintf("Không mã hoá được chương %d.", sc.Chapter), Err: err}
			}
			prog.advance(1)
			// 存中间产物：wav 直接移进缓存目录；编码副本尽量复制。
			entry.Dur = chapterDur
			cachedWav := filepath.Join(cacheDir, entry.Wav)
			if err := os.MkdirAll(cacheDir, 0o755); err != nil {
				return ExportResult{}, err
			}
			if err := os.Rename(chapterWav, cachedWav); err != nil {
				if cerr := copyFile(chapterWav, cachedWav); cerr != nil {
					return ExportResult{}, cerr
				}
			}
			chapterWav = cachedWav
			entry.Enc = exportCacheFileName(sc.Chapter, encKey, chapterFormat)
			if err := copyFile(chapterFile, filepath.Join(cacheDir, entry.Enc)); err != nil {
				entry.Enc = ""
			}
			// 每章完成即落盘：失败/重启后重试能直接用已完成章。
			nextCache.store(sc.Chapter, entry)
			if err := s.saveExportCache(volume, nextCache); err != nil {
				s.logf("Không lưu được bộ đệm xuất audio", "error", err)
			}
		}
		if hit != nil {
			// 命中的章也要写进新 manifest，否则 prune 会把它当旧键删掉。
			nextCache.store(sc.Chapter, entry)
		}
		chapterDurations = append(chapterDurations, chapterDur)
		// chapterStart = 章名之后（与缓存的 wav 内容一致：title 在 wav 里）。
		chapterStart := start
		if titleDur > 0 {
			chapterStart = start + titleDur
		}
		cursor = start + chapterDur
		record(chapterFile)
		items := SegmentTimeline(int64(chapterStart*1000), segTimed)
		offset := len(srtItems)
		for _, it := range items {
			it.Index = offset + it.Index
			srtItems = append(srtItems, it)
		}
		if opts.Split && opts.Subtitles {
			localItems := SegmentTimeline(int64((chapterStart-start)*1000), segTimed)
			subtitleFile := strings.TrimSuffix(chapterFile, "."+chapterFormat) + ".srt"
			if err := writeFileAtomic(subtitleFile, []byte(BuildSRT(localItems))); err != nil {
				return ExportResult{}, err
			}
			record(subtitleFile)
		}
		if opts.GapSec > 0 && i < len(scripts)-1 {
			gap := filepath.Join(dir, fmt.Sprintf("gap-%02d.wav", sc.Chapter))
			if err := writeSilenceWAV(gap, int(opts.GapSec*1000)); err != nil {
				return ExportResult{}, err
			}
			cursor += opts.GapSec
		}
		prog.chapterDone(sc.Chapter)
		builds = append(builds, chapterBuild{source: sc, script: script, wav: chapterWav, startAt: start, dur: chapterDur})
	}
	if opts.Split {
		prog.beginPhase(0.97, 1.0)
		prog.step("Đang gói các tệp đã tách")
		files, err := s.exportFiles(volume)
		if err != nil {
			return ExportResult{}, err
		}
		files = exportedFilesOnly(files, produced)
		chapterFiles := s.annotateChapterFiles(files, source, chapterDurations, volume, opts.Format, short, opts.Trial)
		zipURL := ""
		if opts.Pack == "zip" {
			zipPath := filepath.Join(exportDir, exportZipName(short, volume, len(source)))
			if keepSuffix != "" {
				zipPath = filepath.Join(exportDir, strings.TrimSuffix(exportZipName(short, volume, len(source)), ".zip")+keepSuffix+".zip")
			}
			if err := zipExportFiles(zipPath, s.layout.ExportsDir, files); err != nil {
				return ExportResult{}, err
			}
			record(zipPath)
			relative, _ := filepath.Rel(s.layout.ExportsDir, zipPath)
			zipURL = s.exportFileURL(filepath.ToSlash(relative))
			files, err = s.exportFiles(volume)
			if err != nil {
				return ExportResult{}, err
			}
			files = exportedFilesOnly(files, produced)
		}
		files = withChapterDetails(files, chapterFiles)
		meta := exportMeta{ExportedAt: time.Now(), DurationSec: cursor, Voices: len(voicesUsed), ChapterCount: len(source), Format: opts.Format, Split: true, ChapterFiles: chapterFiles, ZipURL: zipURL}
		for path := range produced {
			meta.Paths = append(meta.Paths, path)
		}
		data, err := json.Marshal(meta)
		if err != nil {
			return ExportResult{}, err
		}
		if err := writeFileAtomic(filepath.Join(exportDir, exportMetaFile), data); err != nil {
			return ExportResult{}, err
		}
		s.pruneExportCache(volume, nextCache)
		s.hub.Publish(NewEvent(EventExport, map[string]any{"volume": volume, "percent": float64(100), "state": "done", "durationSec": cursor}))
		return finishResult(ExportResult{Files: files, ChapterFiles: chapterFiles, ChapterMarks: []ChapterMark{}, DurationSec: cursor, Split: true, ZipURL: zipURL})
	}
	if opts.Intro && musicOutro != "" {
		prog.step("Đang xử lý nhạc kết thúc")
		pcm := filepath.Join(dir, "outro.wav")
		if err := toPCM(ctx, musicOutro, pcm); err != nil {
			return ExportResult{}, &ExportStageError{Cause: "music_decode", Chapter: firstChapter,
				Msg: "Nhạc kết thúc không đọc được — đổi tệp nhạc, hoặc bấm “Ghép không nhạc”.", Err: err}
		}
		musicInputs = append(musicInputs, pcm)
	}

	// 2) 整卷：片头 + 各章（章名 + 段 + 停顿）+ 片尾。
	ordered := make([]string, 0, len(builds)*2+2)
	if len(musicInputs) > 0 {
		ordered = append(ordered, musicInputs[0])
	}
	for _, b := range builds {
		ordered = append(ordered, b.wav)
		gap := filepath.Join(dir, fmt.Sprintf("gap-%02d.wav", b.source.Chapter))
		if opts.GapSec > 0 {
			if _, statErr := os.Stat(gap); statErr == nil {
				ordered = append(ordered, gap)
			}
		}
	}
	if len(musicInputs) > 1 {
		ordered = append(ordered, musicInputs[len(musicInputs)-1])
	}
	volumeWav := filepath.Join(dir, "volume.wav")
	volumeTotal := introDur
	for _, b := range builds {
		volumeTotal += b.dur
	}
	if opts.GapSec > 0 {
		volumeTotal += opts.GapSec * float64(len(builds)-1)
	}
	prog.beginPhase(0.70, 0.85)
	prog.step("Đang nối cả tập")
	if err := concatWAVsProgress(ctx, ordered, volumeWav, volumeTotal, func(f float64) { prog.frac(f) }); err != nil {
		return ExportResult{}, &ExportStageError{Cause: "ffmpeg", Msg: "Không nối được các chương thành một tệp.", Err: err}
	}
	prog.endPhase()
	totalDur, _ := WAVDurationSecFile(volumeWav)
	metadata := ""
	// §AF：整卷文件永远嵌章节。m4b 章节轨 + wav cue 块都由 ffmpeg 从
	// ffmetadata 写；mp3 的 CHAP/CTOC 由 Go 写进 ID3v2.4。
	if !opts.Split && (opts.Format == "m4b" || opts.Format == "wav") {
		metadata = filepath.Join(dir, "chapters.ffmetadata")
		if err := os.WriteFile(metadata, []byte(chapterMetadata(marks, totalDur)), 0o600); err != nil {
			return ExportResult{}, err
		}
	}
	formatName := strings.ToUpper(opts.Format)
	prog.beginPhase(0.85, 0.97)
	prog.step(fmt.Sprintf("Đang mã hoá %s", formatName))
	if err := encodeAudioTaggedProgress(ctx, volumeWav, volumeFile, opts.Format, opts.Loudness,
		tagSpec{tags: volumeTags, coverPNG: coverPNG, marks: marks, totalSec: totalDur, quality: settings.Quality}, metadata, totalDur,
		func(f float64) {
			prog.step(fmt.Sprintf("Đang mã hoá %s · %d %%", formatName, int(f*100+0.5)))
			prog.frac(f)
		}); err != nil {
		return ExportResult{}, &ExportStageError{Cause: "ffmpeg", Msg: "Không mã hoá được tệp của cả tập.", Err: err}
	}
	prog.endPhase()
	prog.beginPhase(0.97, 1.0)
	prog.step("Đang ghi tệp kèm theo")

	// 3) 字幕 / bản đọc / mốc chương。
	srtPath := filepath.Join(exportDir, fmt.Sprintf("Tập %d%s.srt", volume, suffix))
	if opts.Subtitles {
		if err := writeFileAtomic(srtPath, []byte(BuildSRT(srtItems))); err != nil {
			return ExportResult{}, err
		}
		record(srtPath)
	} else if err := os.Remove(srtPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return ExportResult{}, err
	}
	scriptPath := filepath.Join(exportDir, fmt.Sprintf("Tập %d%s.ban-doc.txt", volume, suffix))
	if err := writeFileAtomic(scriptPath, []byte(renderScriptText(cast, scripts, source))); err != nil {
		return ExportResult{}, err
	}
	marksPath := filepath.Join(exportDir, fmt.Sprintf("Tập %d%s.moc-chuong.txt", volume, suffix))
	if err := writeFileAtomic(marksPath, []byte(renderMarks(marks))); err != nil {
		return ExportResult{}, err
	}

	// 4) 隐藏清单（给 "đã xuất" 页用）；波形取成品文件（响度处理之后），2 点/秒。
	peaks, err := filePeaks(ctx, volumeFile, dir)
	if err != nil {
		return ExportResult{}, err
	}
	record(volumeFile)
	record(scriptPath)
	record(marksPath)
	allFiles, err := s.exportFiles(volume)
	if err != nil {
		return ExportResult{}, err
	}
	chapterFiles := s.annotateChapterFiles(exportedFilesOnly(allFiles, produced), source, chapterDurations, volume, chapterFormat, short, opts.Trial)
	meta := exportMeta{
		ExportedAt: time.Now(), DurationSec: totalDur, Voices: len(voicesUsed),
		ChapterCount: len(builds), Format: opts.Format, ChapterMarks: marks, Peaks: peaks, ChapterFiles: chapterFiles,
	}
	for path := range produced {
		meta.Paths = append(meta.Paths, path)
	}
	sort.Strings(meta.Paths)
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return ExportResult{}, err
	}
	if !opts.Trial {
		if err := writeFileAtomic(filepath.Join(exportDir, exportMetaFile), metaData); err != nil {
			return ExportResult{}, err
		}
	}

	files, err := s.exportFiles(volume)
	if err != nil {
		return ExportResult{}, err
	}
	files = exportedFilesOnly(files, produced)
	s.pruneExportCache(volume, nextCache)
	prog.endPhase()
	s.hub.Publish(NewEvent(EventExport, map[string]any{
		"volume": volume, "percent": float64(100), "state": "done", "durationSec": totalDur,
	}))
	return finishResult(ExportResult{Files: files, ChapterFiles: chapterFiles, ChapterMarks: marks, DurationSec: totalDur})
}

// copyFile 复制文件（缓存与临时目录可能不同盘时兜底）。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	srcInfo, err := in.Stat()
	if err != nil {
		return err
	}
	if dstInfo, statErr := os.Stat(dst); statErr == nil && os.SameFile(srcInfo, dstInfo) {
		if src != dst && strings.EqualFold(src, dst) {
			return renameFileCase(src, dst)
		}
		return nil
	}
	out, err := os.CreateTemp(filepath.Dir(dst), ".tmp-*")
	if err != nil {
		return err
	}
	name := out.Name()
	defer os.Remove(name)
	if err := out.Chmod(srcInfo.Mode().Perm()); err != nil {
		out.Close()
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return publishExportFile(name, dst)
}

// Case-insensitive filesystems can resolve both spellings to one inode. Use a
// sibling hop so the directory entry changes spelling without truncating audio.
func renameFileCase(src, dst string) error {
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(src, name); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, dst); err != nil {
		if restoreErr := os.Rename(name, src); restoreErr != nil {
			return fmt.Errorf("không đổi được tên %q thành %q: %w; không khôi phục được %q: %v", src, dst, err, name, restoreErr)
		}
		return err
	}
	// POSIX rename is a no-op when dst is another hard link to the same inode.
	// The destination is intact; remove only the remaining temporary alias.
	_ = os.Remove(name)
	return nil
}

// APFS may retain an existing entry's spelling when a new inode replaces it.
// Normalize that spelling first, then publish the completed temporary file.
func publishExportFile(src, dst string) error {
	if existing, err := os.Stat(dst); err == nil {
		dir, wanted := filepath.Dir(dst), filepath.Base(dst)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.Name() == wanted {
				return os.Rename(src, dst)
			}
		}
		for _, entry := range entries {
			if strings.EqualFold(entry.Name(), wanted) {
				actual := filepath.Join(dir, entry.Name())
				info, err := os.Stat(actual)
				if err != nil {
					return err
				}
				if os.SameFile(existing, info) {
					if err := renameFileCase(actual, dst); err != nil {
						return err
					}
					break
				}
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Rename(src, dst)
}

// chapterTitleAudio 生成（并缓存）"Chương N. Tên chương" 的朗读。
func (s *Service) chapterTitleAudio(ctx context.Context, volume, chapter int, title string, cast *Cast, voices map[string]ProjectVoice, settings Settings, engine, model string) (string, float64, error) {
	text, _ := Normalize(fmt.Sprintf("Chương %d. %s", chapter, title), settings.Normalize, nil)
	ref := ResolveVoice(cast, Segment{Speaker: NarratorID}, voices)
	if !ref.OK {
		return "", 0, fmt.Errorf("Chưa chọn giọng dẫn truyện để đọc tên chương.")
	}
	key := SynthKey{Text: text, VoiceID: ref.VoiceID, Tone: "", Speed: 1, Pitch: 1, Quality: settings.Quality, Engine: engine, Model: model}
	hash := key.Hash()
	path := s.layout.CachePath(volume, chapter, "title-"+hash)
	if data, err := os.ReadFile(path); err == nil {
		return path, WAVDurationSec(data), nil
	}
	audio, err := s.synth.Synthesize(ctx, SynthRequest{
		Text: text, VoiceID: key.VoiceID, RefAudio: ref.RefAudio, Quality: settings.Quality, Speed: 1,
	})
	if err != nil {
		return "", 0, err
	}
	if err := writeFileAtomic(path, audio.WAV); err != nil {
		return "", 0, err
	}
	dur := WAVDurationSec(audio.WAV)
	if dur == 0 {
		dur = audio.DurationSec
	}
	return path, dur, nil
}

// WAVDurationSecFile 读文件头拿时长。
func WAVDurationSecFile(path string) (float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return WAVDurationSec(data), nil
}

// pickMusic 选片头/片尾音乐：名字含 intro/mở 的当片头，含 outro/kết 的当片尾，
// 都没有时按文件名排序取第一个作片头、最后一个（≥2 个时）作片尾。
func pickMusic(dir string) (intro, outro string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", nil
		}
		return "", "", err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if !isAudioFile(e.Name()) {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	sort.Strings(files)
	for _, f := range files {
		lower := strings.ToLower(filepath.Base(f))
		if intro == "" && (strings.Contains(lower, "intro") || strings.Contains(lower, "mo-dau") || strings.Contains(lower, "mở")) {
			intro = f
			continue
		}
		if outro == "" && (strings.Contains(lower, "outro") || strings.Contains(lower, "ket") || strings.Contains(lower, "kết")) {
			outro = f
		}
	}
	if intro == "" && len(files) > 0 {
		intro = files[0]
	}
	if outro == "" && len(files) > 1 {
		outro = files[len(files)-1]
	}
	if outro == intro {
		outro = ""
	}
	return intro, outro, nil
}

// renderScriptText 生成 "Tập N.ban-doc.txt"：按章列出说话人与文本。
func renderScriptText(cast *Cast, scripts []*Script, source []SourceChapter) string {
	var b strings.Builder
	for i, script := range scripts {
		title := ""
		if i < len(source) {
			title = source[i].Title
		}
		fmt.Fprintf(&b, "Chương %d", script.Chapter)
		if strings.TrimSpace(title) != "" {
			fmt.Fprintf(&b, " · %s", title)
		}
		b.WriteString("\n\n")
		for _, seg := range script.Segments {
			fmt.Fprintf(&b, "%s: %s\n", speakerLabel(cast, seg.Speaker), seg.Text)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// renderMarks 生成 "Tập N.moc-chuong.txt"（贴到视频简介）。
func renderMarks(marks []ChapterMark) string {
	var b strings.Builder
	for _, m := range marks {
		fmt.Fprintf(&b, "%s %s\n", FormatMarkTime(int(m.AtSec)), m.Label)
	}
	return b.String()
}

// sanitizeFileName 去掉文件名里不能出现的字符（保留越南语字母）。
func sanitizeFileName(name string) string {
	ext := filepath.Ext(name)
	out := cleanFileNamePart(strings.TrimSuffix(name, ext))
	if out == "" {
		out = "audio"
	}
	if ext = cleanFileNamePart(strings.TrimPrefix(ext, ".")); ext != "" {
		out += "." + ext
	}
	return out
}

// exportFiles 列出 <exports>/tap-NN 下的成品（跳过隐藏清单）。
func (s *Service) exportFiles(volume int) ([]ExportFile, error) {
	exportDir := s.layout.VolumeExportDir(volume)
	var out []ExportFile
	err := filepath.WalkDir(exportDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != exportDir {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		rel, rerr := filepath.Rel(s.layout.ExportsDir, path)
		if rerr != nil {
			return rerr
		}
		mod := info.ModTime()
		v := volume
		out = append(out, ExportFile{
			Path:      filepath.ToSlash(rel),
			Name:      d.Name(),
			Format:    exportFormat(rel),
			Size:      info.Size(),
			SizeBytes: info.Size(),
			UpdatedAt: &mod,
			Volume:    &v,
		})
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return []ExportFile{}, nil
	}
	if out == nil {
		out = []ExportFile{}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, err
}

// exportFormat 与 storyread 的 format 取值一致。
func exportFormat(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".txt":
		return "txt"
	case ".epub":
		return "epub"
	case ".srt":
		return "other"
	}
	if isAudioFile(name) {
		return "audio"
	}
	return "other"
}

// ExportInfo 读取"đã xuất"状态。
func (s *Service) ExportInfo(volume int) (ExportInfo, error) {
	files, err := s.exportFiles(volume)
	if err != nil {
		return ExportInfo{}, err
	}
	info := ExportInfo{Volume: volume, Files: files, ChapterMarks: []ChapterMark{}}
	if rel, rerr := filepath.Rel(s.layout.StoryDir, s.layout.VolumeExportDir(volume)); rerr == nil {
		info.Dir = filepath.ToSlash(rel)
	}
	for _, f := range files {
		info.TotalBytes += f.Size
	}
	data, err := os.ReadFile(filepath.Join(s.layout.VolumeExportDir(volume), exportMetaFile))
	if errors.Is(err, os.ErrNotExist) {
		return info, nil
	}
	if err != nil {
		return info, err
	}
	var meta exportMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return info, nil
	}
	if meta.Paths != nil {
		paths := make(map[string]bool, len(meta.Paths))
		for _, path := range meta.Paths {
			paths[path] = true
		}
		info.Files = exportedFilesOnly(info.Files, paths)
		info.TotalBytes = 0
		for _, file := range info.Files {
			info.TotalBytes += file.Size
		}
	}
	at := meta.ExportedAt
	info.ExportedAt = &at
	info.DurationSec = meta.DurationSec
	info.Voices = meta.Voices
	info.ChapterCount = meta.ChapterCount
	info.Format = meta.Format
	info.Peaks = meta.Peaks
	info.ChapterFiles, info.Split, info.ZipURL = meta.ChapterFiles, meta.Split, meta.ZipURL
	if info.Split {
		info.Files = withChapterDetails(info.Files, info.ChapterFiles)
	}
	if meta.ChapterMarks != nil {
		info.ChapterMarks = meta.ChapterMarks
	}
	return info, nil
}

// filePeaks 把成品解码成 4 kHz 单声道再取峰值：整卷原始 wav 可能上百 MB，不整个读进内存。
func filePeaks(ctx context.Context, path, tmpDir string) ([]float64, error) {
	small := filepath.Join(tmpDir, "peaks.wav")
	if err := runFFmpeg(ctx, "-i", path, "-ac", "1", "-ar", "4000", "-c:a", "pcm_s16le", small); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(small)
	if err != nil {
		return nil, err
	}
	return WAVPeaks(data, 2)
}
