package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/voice"
	"github.com/datnt1998/verse/platform/identity"
)

// Audio 端点（GAP-17…20）。全部走网关进程、直接读写故事目录：
// 不启动 worker、不拿故事租约，所以 TUI 正在写这本 truyện 时音频照样能用
// （15-audio-impl §2 最后一条）。
//
// 路由由 projects.go 转发进来：
//   - serveStoryAudio        ← /api/projects/:p/stories/:s/audio/...
//   - serveProjectVoices     ← /api/projects/:p/voices
//   - serveProjectPronunciations ← /api/projects/:p/pronunciations

// ── voice.Service 适配 ──

// voiceSynthesizer 把 internal/voice 的 Service 适配成 audio.Synthesizer。
// 单独一层是为了让 internal/audio 的单元测试能塞假引擎，也让换引擎不动音频逻辑。
type voiceSynthesizer struct{}

func (voiceSynthesizer) Capabilities() audio.Capabilities {
	return audio.Capabilities{EmotionCues: ttsService().Capabilities().EmotionCues}
}

func (voiceSynthesizer) Status(ctx context.Context) audio.Status {
	svc := ttsService()
	st := svc.Status(ctx)
	return audio.Status{
		Ready: st.Ready, Engine: st.Engine, Model: st.Model, Device: st.Device,
		State: st.State, Error: st.Error, Note: st.Note, RTF: st.RTF,
		ModelID: svc.Config().Model,
	}
}

func (voiceSynthesizer) Voices(ctx context.Context) ([]audio.Voice, error) {
	list, err := ttsService().Voices(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]audio.Voice, 0, len(list))
	for _, v := range list {
		out = append(out, audio.Voice{
			ID: v.ID, Name: v.Name, Gender: v.Gender, Source: v.Source,
			File: v.File, DurationSec: v.DurationSec,
		})
	}
	return out, nil
}

func (voiceSynthesizer) Synthesize(ctx context.Context, req audio.SynthRequest) (audio.Audio, error) {
	res, err := ttsService().Synthesize(ctx, voice.SynthRequest{
		Text: req.Text, VoiceID: req.VoiceID, RefAudio: req.RefAudio,
		Tone: req.Tone, Quality: req.Quality, Speed: req.Speed,
	})
	if err != nil {
		return audio.Audio{}, err
	}
	return audio.Audio{WAV: res.WAV, SampleRate: res.SampleRate, DurationSec: res.DurationSec}, nil
}

func (voiceSynthesizer) AddVoice(ctx context.Context, name, wavPath string) (audio.Voice, error) {
	v, err := ttsService().AddVoice(ctx, name, wavPath)
	if err != nil {
		return audio.Voice{}, err
	}
	return audio.Voice{ID: v.ID, Name: v.Name, Gender: v.Gender, Source: v.Source, File: v.File, DurationSec: v.DurationSec}, nil
}

// audioSynthesizer 是测试替换点（生产走 voice.Shared()）。
var audioSynthesizer = func() audio.Synthesizer { return voiceSynthesizer{} }

// audioServices 按故事目录缓存音频服务（队列与事件中心必须跨请求共享）。
var audioServices sync.Map // dir → audioEntry

// audioEntry 记下服务属于哪个故事（任务中心按项目/故事过滤）。
type audioEntry struct {
	svc            *audio.Service
	project, story string
	root           string
	style          string
}

// audioServicesChanged 在新建服务时关闭并换新，让任务中心的 Watch 接上新故事。
var (
	audioCreateLocks     sync.Map // dir → *sync.Mutex
	audioServicesMu      sync.Mutex
	audioServicesChanged = make(chan struct{})
)

// audioCreateLock serializes service construction for one story without making
// other stories wait on its file I/O or service shutdown.
func audioCreateLock(dir string) *sync.Mutex {
	lock, _ := audioCreateLocks.LoadOrStore(dir, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func audioServicesSignal() <-chan struct{} {
	audioServicesMu.Lock()
	defer audioServicesMu.Unlock()
	return audioServicesChanged
}

// audioService returns the story's audio backend: the Postgres service when
// the hosted store is wired, otherwise the file service cached per directory.
func (g *Gateway) audioService(st library.Story) (audioBackend, error) {
	if g.pgAudio != nil {
		return g.pgAudioService(st)
	}
	svc, err := g.fileAudioService(st)
	if err != nil {
		return nil, err
	}
	return &fileAudioBackend{svc: svc}, nil
}

// pgAudioService returns the Postgres audio backend for a hosted story. The
// cache lives on the gateway (not a package map) so two gateways bound to
// different pools never share a service for the same story slug.
func (g *Gateway) pgAudioService(st library.Story) (*audio.PGService, error) {
	key := st.ID
	if key == "" {
		key = st.Project + "/" + st.Story
	}
	g.pgAudioMu.Lock()
	defer g.pgAudioMu.Unlock()
	if cached, ok := g.pgAudioSvcs[key]; ok {
		return cached, nil
	}
	svc := audio.NewPGService(audio.PGDeps{
		Jobs: g.pgAudio.Jobs, Audio: g.pgAudio.Audio, Cache: g.pgAudio.Cache,
		Nar: g.pgAudio.Nar, Blobs: g.pgAudio.Blobs,
		Keys:    g.pgAudio.Keys,
		Project: st.Project, Story: st.Story,
		Engine: g.pgAudio.Engine, Model: g.pgAudio.Model,
	})
	svc.SetBasePath(audioBasePath(st))
	// A queued needsScript row starts composing immediately instead of waiting
	// for the next restart; the hook is non-blocking.
	svc.SetPreparationWake(g.wakeAudioPreparation)
	if g.pgAudioSvcs == nil {
		g.pgAudioSvcs = map[string]*audio.PGService{}
	}
	g.pgAudioSvcs[key] = svc
	return svc, nil
}

// closePGAudioServices 关闭所有 hosted 音频服务的 hub。
func (g *Gateway) closePGAudioServices() {
	g.pgAudioMu.Lock()
	defer g.pgAudioMu.Unlock()
	for _, svc := range g.pgAudioSvcs {
		svc.Close()
	}
	g.pgAudioSvcs = nil
}

// fileAudioService 返回故事的文件音频服务（目录变化时重建）。
func (g *Gateway) fileAudioService(st library.Story) (*audio.Service, error) {
	lock := audioCreateLock(st.Dir)
	lock.Lock()
	defer lock.Unlock()
	if info, err := os.Stat(st.Dir); err != nil {
		return nil, err
	} else if !info.IsDir() {
		return nil, os.ErrNotExist
	}
	g.jobsHub().allowStory(st.Project, st.Story)
	layout, err := audio.ResolveLayout(st)
	if err != nil {
		return nil, err
	}
	cfg, err := bootstrap.LoadConfigAt(st.Dir)
	if err != nil {
		return nil, err
	}
	if cached, ok := audioServices.Load(st.Dir); ok {
		entry := cached.(audioEntry)
		if entry.svc.Layout().AudioDir == layout.AudioDir && entry.style == cfg.Style {
			entry.svc.SetBasePath(audioBasePath(st))
			name, short := storyFileNames(st)
			entry.svc.SetStoryNames(name, short)
			return entry.svc, nil
		}
		entry.svc.Close()
		audioServices.Delete(st.Dir)
	}
	bundle := assets.Load(cfg.Style, assets.DefaultLoadOptions(st.Dir))
	if bundle.Err != nil {
		return nil, bundle.Err
	}
	if bundle.Pack != nil {
		layout.Pack, err = audio.ReadPackDefaults(bundle.Pack, bundle.Pack.NguoiNghe)
		if err != nil {
			return nil, err
		}
	}
	svc, err := audio.NewService(layout, audioSynthesizer(), func(volume, chapter int) (audio.SourceChapter, audio.Composer, audio.CharacterRef, error) {
		reader := host.NewStoryReader(st.Dir)
		completed, err := reader.CompletedSet()
		if err != nil {
			return audio.SourceChapter{}, nil, audio.CharacterRef{}, err
		}
		text, err := reader.ChapterText(chapter, "committed")
		if err != nil {
			return audio.SourceChapter{}, nil, audio.CharacterRef{}, err
		}
		selected := audio.SourceChapter{Chapter: chapter, Title: text.Title, Text: text.Text, Completed: completed[chapter]}
		composer, err := audioComposer(st)
		if err != nil {
			return selected, nil, audio.CharacterRef{}, err
		}
		ref, err := storyCharacters(reader)
		return selected, composer, ref, err
	})
	if err != nil {
		return nil, err
	}
	svc.SetBasePath(audioBasePath(st))
	if name, short := storyFileNames(st); name != "" || short != "" {
		svc.SetStoryNames(name, short)
	}
	audioServices.Store(st.Dir, audioEntry{svc: svc, project: st.Project, story: st.Story, root: st.Root, style: cfg.Style})
	// 后台生成改了章节状态（xong/lỗi）也要让其它标签页刷新；服务关闭时事件流结束。
	go func(events <-chan audio.Event) {
		for ev := range events {
			if ev.Type == audio.EventQueue || ev.Type == audio.EventExport {
				g.audioChanged(st)
				// 导出完成后"đã xuất"页/导出列表也要刷新（worker 的 job.done 走
				// publishWorkerChange；这里 services 直连，自己补一条 exports part）。
				if ev.Type == audio.EventExport && ev.Data["state"] == audio.JobDone {
					g.storyChanged(st.Project, st.Story, "exports")
				}
			}
			// Một kết nối SSE cho mỗi tab: khung audio đi kèm /api/events dưới dạng
			// `live` (kind = "audio." + loại), /audio/events giữ lại cho bản cũ.
			g.events().Broadcast(Event{
				Type: liveDataType, Project: st.Project, Story: st.Story,
				// ev.Type 已经是 "audio.render" 一类，别再拼一次前缀。
				Data: liveFrame{Project: st.Project, Story: st.Story, Kind: ev.Type, Data: liveJSON(ev)},
			})
		}
	}(svc.Hub().Subscribe(context.Background()))
	audioServicesMu.Lock()
	close(audioServicesChanged)
	audioServicesChanged = make(chan struct{})
	audioServicesMu.Unlock()
	return svc, nil
}

// audioBasePath 是故事 API 前缀（段音频 URL 用）。
func audioBasePath(st library.Story) string {
	return "/api/projects/" + url.PathEscape(st.Project) + "/stories/" + url.PathEscape(st.Story)
}

// storyFileNames follows the same title priority as storySummary: canonical
// meta/book.json, then the imported story display title; never a project slug.
func storyFileNames(st library.Story) (title, shortTitle string) {
	if f, err := library.LoadStoryFile(st.Dir); err == nil && f != nil {
		title, shortTitle = f.Title, strings.TrimSpace(f.ShortTitle)
	}
	if canonical := host.NewStoryReader(st.Dir).BookTitle(); strings.TrimSpace(canonical) != "" {
		title = canonical
	}
	return audio.CleanStoryTitle(title), shortTitle
}

// ── 故事级路由 ──

// serveStoryAudio 处理 S/audio 之后的全部路径。返回 false 表示不归它管。
// 授权已在 serveStories 的 storyAccessAllowed 完成（GET 需可读、其余需可写）。
func (g *Gateway) serveStoryAudio(w http.ResponseWriter, r *http.Request, user identity.User, st library.Story, rest string) bool {
	tail := strings.TrimPrefix(rest, "/audio")
	tail = strings.TrimPrefix(tail, "/")
	if tail == "" {
		jsonError(w, http.StatusNotFound, "không tìm thấy")
		return true
	}
	segs := strings.Split(tail, "/")
	svc, err := g.audioService(st)
	if err != nil {
		audioError(w, err)
		return true
	}
	sr, err := g.openStoryReader(user, st)
	if err != nil {
		audioError(w, err)
		return true
	}

	// 任何成功的写操作都广播 story.changed{parts:["audio"]}（预览/重读候选不改已存数据）。
	if r.Method != http.MethodGet && segs[0] != "candidates" && !(segs[0] == "segments" && strings.HasSuffix(tail, "/preview")) {
		rec := &audioStatusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if rec.status < 300 {
				g.audioChanged(st)
			}
		}()
		w = rec
	}

	switch segs[0] {
	case "export-prefs":
		if len(segs) != 1 {
			return false
		}
		switch r.Method {
		case http.MethodGet:
			prefs, err := svc.ExportPrefs()
			if err != nil {
				audioError(w, err)
				return true
			}
			writeJSON(w, http.StatusOK, prefs)
		case http.MethodPut:
			var prefs audio.ExportPrefs
			if !decodeJSON(w, r, &prefs) {
				return true
			}
			if err := svc.SaveExportPrefs(prefs); err != nil {
				audioError(w, err)
				return true
			}
			writeJSON(w, http.StatusOK, prefs)
		default:
			return false
		}
		return true
	case "ready":
		if len(segs) != 1 || r.Method != http.MethodGet {
			return false
		}
		writeJSON(w, http.StatusOK, svc.Ready(r.Context()))
		return true
	case "events":
		if len(segs) != 1 || r.Method != http.MethodGet {
			return false
		}
		serveAudioEvents(w, r, svc)
		return true
	case "volumes":
		return g.serveAudioVolume(w, r, user, st, svc, sr, segs[1:])
	case "voices":
		return g.serveStoryVoices(w, r, user, st, svc, segs[1:])
	case "cast":
		if len(segs) == 2 && r.Method == http.MethodPut {
			g.serveAudioRoleVoice(w, r, svc, segs[1])
			return true
		}
		if len(segs) != 1 {
			return false
		}
		g.serveAudioCast(w, r, st, svc, sr)
		return true
	case "settings":
		if len(segs) != 1 {
			return false
		}
		g.serveAudioSettings(w, r, st, svc)
		return true
	case "queue":
		if len(segs) != 1 && r.Method != http.MethodPost {
			return false
		}
		volume := 1
		if raw := r.URL.Query().Get("volume"); raw != "" {
			var err error
			volume, err = strconv.Atoi(raw)
			if err != nil || volume < 1 {
				jsonError(w, http.StatusBadRequest, "Tập không hợp lệ.")
				return true
			}
		}
		if r.Method == http.MethodPost {
			if len(segs) == 2 {
				var body struct {
					Quality string `json:"quality"`
				}
				if r.ContentLength != 0 && !decodeJSON(w, r, &body) {
					return true
				}
				found := false
				for _, job := range svc.Queue() {
					if job.Volume != volume {
						continue
					}
					var actionErr error
					switch segs[1] {
					case "pause":
						if job.State != audio.JobQueued && job.State != audio.JobRendering {
							continue
						}
						actionErr = svc.Pause(job.ID)
					case "resume":
						if job.State != audio.JobPaused {
							continue
						}
						actionErr = svc.Resume(job.ID)
					case "retry":
						if job.State != audio.JobError {
							continue
						}
						actionErr = svc.Retry(r.Context(), job.ID, body.Quality)
					default:
						return false
					}
					if actionErr != nil {
						audioError(w, actionErr)
						return true
					}
					found = true
				}
				if !found {
					jsonError(w, http.StatusConflict, "Không có việc phù hợp để thực hiện.")
					return true
				}
				writeJSON(w, http.StatusOK, map[string]any{"ok": true})
				return true
			}
			var in audio.QueueInput
			if !decodeJSON(w, r, &in) {
				return true
			}
			source, _, err := audioVolumeSource(sr, volume)
			if err != nil {
				audioError(w, err)
				return true
			}
			result, err := svc.QueueChapters(r.Context(), source, volume, in)
			if err != nil {
				audioError(w, err)
				return true
			}
			writeJSON(w, http.StatusOK, result)
			return true
		}
		if r.Method != http.MethodGet || len(segs) != 1 {
			return false
		}
		source, _, err := audioVolumeSource(sr, volume)
		if err != nil {
			audioError(w, err)
			return true
		}
		view, err := svc.Timeline(r.Context(), source, volume)
		if err != nil {
			audioError(w, err)
			return true
		}
		// Việc tạo giọng xếp hàng mà không có máy đọc sẵn sàng: đánh dấu từng
		// chương/việc `waitingMachine` để UI ghi "đang chờ máy đọc" (§AT §8.3).
		// A needsScript row is the web process composing the bản đọc, not work
		// waiting for the reader machine, so neither it nor its chapter may
		// carry the flag (GenPickEmpty: the view already says step=compose).
		if !g.voiceMachineReady(r.Context()) {
			composing := map[int]bool{}
			for i := range view.Jobs {
				job := view.Jobs[i]
				if job.NeedsScript || job.Step == "compose" {
					if job.State == audio.JobQueued || job.State == audio.JobRendering || job.State == audio.JobPaused {
						composing[job.Chapter] = true
						continue
					}
				}
				if job.State == audio.JobQueued {
					view.Jobs[i].WaitingMachine = true
				}
			}
			for i := range view.Chapters {
				if view.Chapters[i].State == "queued" && !composing[view.Chapters[i].Chapter] {
					view.Chapters[i].WaitingMachine = true
				}
			}
		}
		writeJSON(w, http.StatusOK, view)
		return true
	case "renders":
		if len(segs) != 3 || r.Method != http.MethodPost {
			return false
		}
		g.serveAudioRenderAction(w, r, st, svc, segs[1], segs[2])
		return true
	case "segments":
		g.serveAudioSegment(w, r, st, svc, segs[1:])
		return true
	case "candidates":
		if len(segs) != 2 || r.Method != http.MethodGet {
			return false
		}
		obj, err := svc.OpenCandidate(r.Context(), segs[1])
		if err != nil {
			audioError(w, err)
			return true
		}
		serveAudioObject(w, r, obj)
		return true
	case "flags":
		g.serveAudioFlags(w, r, st, svc, segs[1:])
		return true
	}
	jsonError(w, http.StatusNotFound, "không tìm thấy")
	return true
}

// serveAudioVolume 处理 volumes/:v[/...]。
func (g *Gateway) serveAudioVolume(w http.ResponseWriter, r *http.Request, user identity.User, st library.Story, svc audioBackend, sr *host.StoryReader, segs []string) bool {
	if len(segs) < 1 {
		return false
	}
	volume, err := strconv.Atoi(segs[0])
	if err != nil || volume <= 0 {
		jsonError(w, http.StatusNotFound, "không tìm thấy tập")
		return true
	}

	source, title, err := audioVolumeSource(sr, volume)
	if err != nil {
		audioError(w, err)
		return true
	}
	switch {
	case len(segs) == 1 && r.Method == http.MethodGet:
		view, err := svc.Volumes(r.Context(), source, volume)
		if err != nil {
			audioError(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, view)
		return true

	case len(segs) == 2 && segs[1] == "scripts" && r.Method == http.MethodPost:
		var body struct {
			Chapters []int `json:"chapters"`
		}
		if !decodeJSON(w, r, &body) {
			return true
		}
		chapters := sortedChaptersInput(body.Chapters)
		if len(chapters) == 0 {
			chapters = chaptersWithoutScript(source, volume, svc, r.Context())
		}
		composer, cerr := g.audioComposerForStory(r.Context(), user, st)
		if cerr != nil {
			audioError(w, cerr)
			return true
		}
		ref, rerr := storyCharacters(sr)
		if rerr != nil {
			audioError(w, rerr)
			return true
		}
		if err := svc.Compose(source, volume, chapters, composer, ref); err != nil {
			audioError(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "chapters": chapters})
		return true

	case len(segs) >= 3 && segs[1] == "scripts":
		chapter, cerr := strconv.Atoi(segs[2])
		if cerr != nil || chapter <= 0 {
			jsonError(w, http.StatusNotFound, "không có chương")
			return true
		}
		currentHash := chapterHashOf(sr, chapter)
		switch {
		case len(segs) == 3 && r.Method == http.MethodGet:
			view, err := svc.ScriptView(r.Context(), volume, chapter, currentHash)
			if err != nil {
				audioError(w, err)
				return true
			}
			version := writeVersionToken(w, svc.ScriptVersion(volume, chapter))
			writeJSON(w, http.StatusOK, struct {
				*audio.ScriptView
				Version string `json:"version"`
			}{view, version})
			return true
		case len(segs) == 3 && r.Method == http.MethodPut:
			token := svc.ScriptVersion(volume, chapter)
			if !checkIfMatchV(w, r, token, func() (any, error) {
				view, err := svc.ScriptView(r.Context(), volume, chapter, currentHash)
				return versionedScript{view, svc.ScriptVersion(volume, chapter)}, err
			}) {
				return true
			}
			var edit audio.ScriptEdit
			if !decodeJSON(w, r, &edit) {
				return true
			}
			if _, err := svc.SaveScript(r.Context(), volume, chapter, token, edit); err != nil {
				if errors.Is(err, errs.ErrStorageConflict) {
					// The precheck passed but the row moved before commit: the
					// same 409 shape as If-Match keeps the "Ghi đè" flow alive.
					view, verr := svc.ScriptView(r.Context(), volume, chapter, currentHash)
					if verr != nil {
						audioError(w, verr)
						return true
					}
					currentToken := svc.ScriptVersion(volume, chapter)
					writeVersionToken(w, currentToken)
					writeJSON(w, http.StatusConflict, map[string]any{
						"error": "Đã có thay đổi từ nơi khác", "conflict": true,
						"version": currentToken, "current": versionedScript{view, currentToken},
					})
					return true
				}
				audioError(w, err)
				return true
			}
			view, err := svc.ScriptView(r.Context(), volume, chapter, currentHash)
			if err != nil {
				audioError(w, err)
				return true
			}
			version := writeVersionToken(w, svc.ScriptVersion(volume, chapter))
			writeJSON(w, http.StatusOK, struct {
				*audio.ScriptView
				Version string `json:"version"`
			}{view, version})
			return true
		case len(segs) == 4 && r.Method == http.MethodPost:
			g.serveAudioScriptAction(w, r, user, st, svc, sr, source, volume, chapter, segs[3])
			return true
		}
		jsonError(w, http.StatusNotFound, "không tìm thấy")
		return true

	case len(segs) >= 2 && segs[1] == "chapters":
		g.serveAudioChapter(w, r, st, svc, sr, segs[2:], volume)
		return true

	case len(segs) == 2 && segs[1] == "exports" && r.Method == http.MethodGet:
		info, err := svc.ExportInfo(r.Context(), volume)
		if err != nil {
			audioError(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, info)
		return true

	case len(segs) == 2 && segs[1] == "exports" && r.Method == http.MethodPost:
		var opts audio.ExportOptions
		if !decodeJSON(w, r, &opts) {
			return true
		}
		book := strings.TrimSpace(sr.BookTitle())
		if book == "" {
			book = strings.TrimSpace(title)
		}
		jobID := st.Project + "/" + st.Story + "/audio/" + segs[0] + "/export"
		if err := svc.StartExport(r.Context(), volume, book, source, opts); err != nil {
			var busy *audio.ExportBusyError
			if errors.As(err, &busy) {
				jsonConflictJob(w, busy.Error(), jobID)
				return true
			}
			audioError(w, err)
			return true
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"jobId": jobID})
		return true
	}
	jsonError(w, http.StatusNotFound, "không tìm thấy")
	return true
}

// serveAudioScriptAction 处理 lock / unlock / refresh。
func (g *Gateway) serveAudioScriptAction(w http.ResponseWriter, r *http.Request, user identity.User, st library.Story, svc audioBackend, sr *host.StoryReader, source []audio.SourceChapter, volume, chapter int, action string) {
	switch action {
	case "lock":
		if _, err := svc.Lock(r.Context(), volume, chapter, chapterHashOf(sr, chapter)); err != nil {
			audioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case "unlock":
		if _, err := svc.Unlock(r.Context(), volume, chapter); err != nil {
			audioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case "refresh":
		composer, err := g.audioComposerForStory(r.Context(), user, st)
		if err != nil {
			audioError(w, err)
			return
		}
		ref, err := storyCharacters(sr)
		if err != nil {
			audioError(w, err)
			return
		}
		if err := svc.Refresh(source, volume, chapter, composer, ref); err != nil {
			audioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		jsonError(w, http.StatusNotFound, "không tìm thấy")
	}
}

// serveAudioChapter 处理 chapters/:ch.mp3 与 chapters/:ch/peaks。
func (g *Gateway) serveAudioChapter(w http.ResponseWriter, r *http.Request, st library.Story, svc audioBackend, sr *host.StoryReader, segs []string, volume int) {
	if len(segs) == 0 {
		jsonError(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	if len(segs) == 1 {
		name := segs[0]
		if !strings.HasSuffix(name, ".mp3") {
			jsonError(w, http.StatusNotFound, "không tìm thấy")
			return
		}
		chapter, err := strconv.Atoi(strings.TrimSuffix(name, ".mp3"))
		if err != nil || chapter <= 0 {
			jsonError(w, http.StatusNotFound, "không có chương")
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, http.StatusNotFound, "không tìm thấy")
			return
		}
		obj, err := svc.OpenChapterAudio(r.Context(), volume, chapter)
		if err != nil {
			audioError(w, err)
			return
		}
		serveAudioObject(w, r, obj)
		return
	}
	if len(segs) == 2 && (segs[1] == "rerender" || segs[1] == "prioritize") && r.Method == http.MethodPost {
		chapter, err := strconv.Atoi(segs[0])
		if err != nil || chapter < 1 {
			jsonError(w, http.StatusBadRequest, "Chương không hợp lệ.")
			return
		}
		if segs[1] == "prioritize" {
			if err := svc.Prioritize(r.Context(), volume, chapter); err != nil {
				audioError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			return
		}
		var in audio.QueueInput
		if !decodeJSON(w, r, &in) {
			return
		}
		// §AK："tạo lại chương" = đọc lại từ đầu (redo)，不是只补改动段。
		in.Redo = []int{chapter}
		source, _, err := audioVolumeSource(sr, volume)
		if err != nil {
			audioError(w, err)
			return
		}
		out, err := svc.QueueChapters(r.Context(), source, volume, in)
		if err != nil {
			audioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	if len(segs) == 2 && segs[1] == "render" && r.Method == http.MethodPost {
		chapter, err := strconv.Atoi(segs[0])
		if err != nil || chapter <= 0 {
			jsonError(w, http.StatusNotFound, "không có chương")
			return
		}
		jobID, err := svc.Enqueue(r.Context(), volume, chapter, chapterHashOf(sr, chapter))
		if err != nil {
			audioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "jobId": jobID})
		return
	}
	if len(segs) == 2 && segs[1] == "peaks" && r.Method == http.MethodGet {
		chapter, err := strconv.Atoi(segs[0])
		if err != nil || chapter <= 0 {
			jsonError(w, http.StatusNotFound, "không có chương")
			return
		}
		peaks, duration, err := svc.Peaks(r.Context(), volume, chapter)
		if err != nil {
			audioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"peaks": peaks, "durationSec": duration})
		return
	}
	jsonError(w, http.StatusNotFound, "không tìm thấy")
}

// serveAudioSegment 处理 segments/:id/{preview,rerender,accept,reject,audio}。
func (g *Gateway) serveAudioSegment(w http.ResponseWriter, r *http.Request, st library.Story, svc audioBackend, segs []string) {
	if len(segs) < 2 {
		jsonError(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	volume, chapter, segID, err := parseSegmentRef(r, segs[0])
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	switch segs[1] {
	case "preview":
		if r.Method != http.MethodPost {
			jsonError(w, http.StatusNotFound, "không tìm thấy")
			return
		}
		res, err := svc.Preview(r.Context(), volume, chapter, segID)
		if err != nil {
			audioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, res)
	case "audio":
		if r.Method != http.MethodGet {
			jsonError(w, http.StatusNotFound, "không tìm thấy")
			return
		}
		obj, err := svc.OpenSegmentAudio(r.Context(), volume, chapter, segID)
		if err != nil {
			audioError(w, err)
			return
		}
		serveAudioObject(w, r, obj)
	case "rerender":
		if r.Method != http.MethodPost {
			jsonError(w, http.StatusNotFound, "không tìm thấy")
			return
		}
		var opts audio.RerenderOptions
		if !decodeJSON(w, r, &opts) {
			return
		}
		res, err := svc.Rerender(r.Context(), volume, chapter, segID, opts)
		if err != nil {
			audioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, res)
	case "accept":
		if r.Method != http.MethodPost {
			jsonError(w, http.StatusNotFound, "không tìm thấy")
			return
		}
		var body struct {
			CandidateID string `json:"candidateId"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		if err := svc.AcceptCandidate(r.Context(), volume, chapter, segID, body.CandidateID); err != nil {
			audioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case "reject":
		if r.Method != http.MethodPost {
			jsonError(w, http.StatusNotFound, "không tìm thấy")
			return
		}
		var body struct {
			CandidateID string `json:"candidateId"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		if err := svc.RejectCandidate(r.Context(), body.CandidateID); err != nil {
			audioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		jsonError(w, http.StatusNotFound, "không tìm thấy")
	}
}

// serveAudioRenderAction 处理 renders/:job/{pause,resume,cancel}。
func (g *Gateway) serveAudioRenderAction(w http.ResponseWriter, r *http.Request, st library.Story, svc audioBackend, jobID, action string) {
	var err error
	switch action {
	case "pause":
		err = svc.Pause(jobID)
	case "resume":
		err = svc.Resume(jobID)
	case "cancel":
		err = svc.Cancel(jobID)
	default:
		jsonError(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	if err != nil {
		audioError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// serveAudioFlags 处理 flags 列表、新建与删除。
func (g *Gateway) serveAudioFlags(w http.ResponseWriter, r *http.Request, st library.Story, svc audioBackend, segs []string) {
	switch {
	case len(segs) == 0 && r.Method == http.MethodGet:
		flags, err := svc.Flags()
		if err != nil {
			audioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"flags": flags})
	case len(segs) == 0 && r.Method == http.MethodPost:
		var flag audio.Flag
		if !decodeJSON(w, r, &flag) {
			return
		}
		saved, err := svc.AddFlag(r.Context(), flag)
		if err != nil {
			audioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, saved)
	case len(segs) == 1 && r.Method == http.MethodDelete:
		if err := svc.DeleteFlag(r.Context(), segs[0]); err != nil {
			audioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		jsonError(w, http.StatusNotFound, "không tìm thấy")
	}
}

// serveAudioCast 处理 GET/PUT cast。
func (g *Gateway) serveAudioCast(w http.ResponseWriter, r *http.Request, st library.Story, svc audioBackend, sr *host.StoryReader) {
	switch r.Method {
	case http.MethodGet:
		ref, err := storyCharacters(sr)
		if err != nil {
			audioError(w, err)
			return
		}
		cast, err := svc.Cast(ref)
		if err != nil {
			audioError(w, err)
			return
		}
		if err := svc.CastOldSegments(cast); err != nil {
			audioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, versionedCast{cast, writeVersionToken(w, svc.CastVersion())})
	case http.MethodPut:
		if !checkIfMatchV(w, r, svc.CastVersion(), func() (any, error) {
			ref, err := storyCharacters(sr)
			if err != nil {
				return nil, err
			}
			cast, err := svc.Cast(ref)
			return versionedCast{cast, svc.CastVersion()}, err
		}) {
			return
		}
		var cast audio.Cast
		if !decodeJSON(w, r, &cast) {
			return
		}
		if err := svc.ApplyPackVoice(&cast); err != nil {
			audioError(w, err)
			return
		}
		saved, err := svc.SaveCast(&cast)
		if err != nil {
			audioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, versionedCast{saved, writeVersionToken(w, svc.CastVersion())})
	default:
		jsonError(w, http.StatusNotFound, "không tìm thấy")
	}
}

// serveAudioSettings 处理 GET/PUT settings。
func (g *Gateway) serveAudioSettings(w http.ResponseWriter, r *http.Request, st library.Story, svc audioBackend) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, versionedSettings{svc.Settings(), writeVersionToken(w, svc.SettingsVersion())})
	case http.MethodPut:
		if !checkIfMatchV(w, r, svc.SettingsVersion(), func() (any, error) {
			return versionedSettings{svc.Settings(), svc.SettingsVersion()}, nil
		}) {
			return
		}
		in := svc.Settings()
		if !decodeJSON(w, r, &in) {
			return
		}
		saved, err := svc.SaveSettings(in)
		if err != nil {
			audioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, versionedSettings{saved, writeVersionToken(w, svc.SettingsVersion())})
	default:
		jsonError(w, http.StatusNotFound, "không tìm thấy")
	}
}

// ── 项目级路由 ──

// serveProjectVoices 处理 /api/projects/:p/voices（GET 列表 / POST 上传）。
func (g *Gateway) serveProjectVoices(w http.ResponseWriter, r *http.Request, user identity.User, p string) {
	lib, err := g.lib(r.Context(), actorOf(user))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !g.requireProject(w, lib, r.Context(), actorOf(user), user, p) {
		return
	}
	ref, ok := lib.ResolveProject(r.Context(), actorOf(user), p)
	if !ok {
		jsonError(w, http.StatusNotFound, "không tìm thấy dự án")
		return
	}
	files, err := g.openProjectAudioFiles(r.Context(), user, p, ref, lib.Config())
	if err != nil {
		audioError(w, err)
		return
	}
	defer files.cleanup()
	layout := files.layout
	tail := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/projects/"+url.PathEscape(p)+"/voices"), "/")
	if tail != "" {
		parts := strings.Split(tail, "/")
		if len(parts) == 2 && parts[1] == "sample" && r.Method == http.MethodGet {
			if files.projectID.Valid {
				g.serveHostedProjectVoiceSample(w, r, layout, files.projectID, parts[0])
			} else {
				serveVoiceSample(w, r, layout, parts[0])
			}
			return
		}
		if len(parts) == 1 && r.Method == http.MethodDelete {
			if !user.CanCollaborateProject(p) {
				jsonError(w, http.StatusForbidden, "Không có quyền")
				return
			}
			if err := audio.DeleteVoice(layout, parts[0]); err != nil {
				audioError(w, err)
				return
			}
			if files.projectID.Valid {
				if err := g.deleteHostedProjectVoice(r.Context(), files.projectID, parts[0]); err != nil {
					audioError(w, err)
					return
				}
			}
			g.projectChanged(p)
			list, err := audio.ReadProjectVoices(layout)
			if err != nil {
				audioError(w, err)
				return
			}
			projectVoiceSamples(list, "/api/projects/"+url.PathEscape(p)+"/voices")
			writeJSON(w, http.StatusOK, list)
			return
		}
		jsonError(w, http.StatusNotFound, "Không tìm thấy mẫu giọng.")
		return
	}
	switch r.Method {
	case http.MethodGet:
		list, err := audio.ListVoices(r.Context(), layout, audioSynthesizer())
		if err != nil {
			audioError(w, err)
			return
		}
		projectVoiceSamples(list, "/api/projects/"+url.PathEscape(p)+"/voices")
		writeJSON(w, http.StatusOK, list)
	case http.MethodPost:
		if !user.CanCollaborateProject(p) {
			jsonError(w, http.StatusForbidden, "Không có quyền")
			return
		}
		rec := &audioStatusRecorder{ResponseWriter: w, status: http.StatusOK}
		if files.projectID.Valid {
			g.uploadProjectVoice(rec, r, layout, func(v audio.ProjectVoice) error {
				return g.publishProjectVoice(r.Context(), files, v)
			})
		} else {
			g.uploadProjectVoice(rec, r, layout)
		}
		if rec.status < 300 {
			g.events().Publish(Event{Type: "story.changed", Project: p, Data: map[string]any{"parts": []string{"audio"}}})
		}
	default:
		jsonError(w, http.StatusNotFound, "không tìm thấy")
	}
}

// uploadProjectVoice 接收 multipart（file + name + gender），校验 10–30 秒后入库。
func (g *Gateway) uploadProjectVoice(w http.ResponseWriter, r *http.Request, layout audio.Layout, publish ...func(audio.ProjectVoice) error) {
	if !audio.FFmpegAvailable() {
		jsonError(w, http.StatusServiceUnavailable, audio.ErrFFmpegMissing.Error())
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		jsonError(w, http.StatusBadRequest, "Không đọc được tệp gửi lên.")
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "Thiếu tệp ghi âm.")
		return
	}
	defer file.Close()
	switch strings.ToLower(filepath.Ext(header.Filename)) {
	case ".mp3", ".wav", ".m4a":
	default:
		jsonError(w, http.StatusBadRequest, "Định dạng chưa được hỗ trợ. Hãy chọn tệp .mp3, .wav hoặc .m4a.")
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(header.Filename), filepath.Ext(header.Filename))
	}
	gender := r.FormValue("gender")
	dir, err := os.MkdirTemp("", "ainovel-voice-*")
	if err != nil {
		audioError(w, err)
		return
	}
	defer os.RemoveAll(dir)
	src := filepath.Join(dir, "upload"+filepath.Ext(header.Filename))
	out, err := os.Create(src)
	if err != nil {
		audioError(w, err)
		return
	}
	if _, err := io.Copy(out, io.LimitReader(file, 64<<20)); err != nil {
		out.Close()
		audioError(w, err)
		return
	}
	if err := out.Close(); err != nil {
		audioError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	duration, err := audio.ProbeDuration(ctx, src)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "Không đọc được tệp ghi âm. Hãy chọn tệp .mp3, .wav hoặc .m4a nghe được.")
		return
	}
	if err := audio.ValidateVoiceClip(duration); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	wav := filepath.Join(dir, "voice.wav")
	if err := audio.ConvertVoiceClip(ctx, src, wav); err != nil {
		jsonError(w, http.StatusBadRequest, "Không xử lý được tệp ghi âm này. Hãy chọn mẫu khác.")
		return
	}
	v, err := audio.StoreVoice(layout, name, gender, wav, duration)
	if err != nil {
		audioError(w, err)
		return
	}
	if len(publish) > 0 {
		if err := publish[0](v); err != nil {
			audioError(w, err)
			return
		}
	}
	v.SampleURL = strings.TrimSuffix(r.URL.Path, "/") + "/" + url.PathEscape(v.ID) + "/sample"
	writeJSON(w, http.StatusOK, v)
}

// serveProjectPronunciations 处理 /api/projects/:p/pronunciations。
func (g *Gateway) serveProjectPronunciations(w http.ResponseWriter, r *http.Request, user identity.User, p string) {
	lib, err := g.lib(r.Context(), actorOf(user))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !g.requireProject(w, lib, r.Context(), actorOf(user), user, p) {
		return
	}
	ref, ok := lib.ResolveProject(r.Context(), actorOf(user), p)
	if !ok {
		jsonError(w, http.StatusNotFound, "không tìm thấy dự án")
		return
	}
	if ref.Dir == "" {
		g.serveHostedProjectPronunciations(w, r, user, p)
		return
	}
	layout, err := projectAudioLayout(g.RootPath(), ref.Dir)
	if err != nil {
		audioError(w, err)
		return
	}
	switch r.Method {
	case http.MethodGet:
		entries, err := audio.ReadPronunciations(layout)
		if err != nil {
			audioError(w, err)
			return
		}
		writeVersion(w, layout.PronouncePath) // 契约是裸数组：version 走 ETag
		writeJSON(w, http.StatusOK, entries)
	case http.MethodPut:
		if !user.CanCollaborateProject(p) {
			jsonError(w, http.StatusForbidden, "Không có quyền")
			return
		}
		if !checkIfMatch(w, r, layout.PronouncePath, func() (any, error) { return audio.ReadPronunciations(layout) }) {
			return
		}
		// 契约是裸数组；也接受 {entries:[...]}，方便前端带元信息。
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			jsonError(w, http.StatusBadRequest, "Dữ liệu gửi lên không đọc được")
			return
		}
		var entries []audio.Pronunciation
		if err := json.Unmarshal(raw, &entries); err != nil {
			var wrapped struct {
				Entries []audio.Pronunciation `json:"entries"`
			}
			if werr := json.Unmarshal(raw, &wrapped); werr != nil {
				jsonError(w, http.StatusBadRequest, "Dữ liệu gửi lên không đọc được")
				return
			}
			entries = wrapped.Entries
		}
		if err := audio.WritePronunciations(layout, entries); err != nil {
			audioError(w, err)
			return
		}
		g.events().Publish(Event{Type: "story.changed", Project: p, Data: map[string]any{"parts": []string{"audio"}}})
		list, err := audio.ReadPronunciations(layout)
		if err != nil {
			audioError(w, err)
			return
		}
		writeVersion(w, layout.PronouncePath)
		writeJSON(w, http.StatusOK, list)
	default:
		jsonError(w, http.StatusNotFound, "không tìm thấy")
	}
}

// ── 工具 ──

// audioStatusRecorder 记下状态码，决定写操作是否成功。
type audioStatusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *audioStatusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// versionedCast 是带 version 的分配表响应（16-feedback-refresh §4）。
type versionedCast struct {
	*audio.Cast
	Version string `json:"version"`
}

type versionedScript struct {
	*audio.ScriptView
	Version string `json:"version"`
}

type versionedSettings struct {
	audio.Settings
	Version string `json:"version"`
}

// writeVersion 把版本写进 ETag 并返回。
func writeVersion(w http.ResponseWriter, path string) string {
	v := fileVersion(path)
	w.Header().Set("ETag", strconv.Quote(v))
	return v
}

// checkIfMatch 校验 If-Match；不带则放行（"Ghi đè" 就是不带 If-Match 重发）。
// A stale write returns the same resource shape as GET, without dropping edits.
func checkIfMatch(w http.ResponseWriter, r *http.Request, path string, resource func() (any, error)) bool {
	want := strings.TrimSpace(r.Header.Get("If-Match"))
	if want == "" || want == "*" {
		return true
	}
	if unq, err := strconv.Unquote(want); err == nil {
		want = unq
	}
	current := fileVersion(path)
	if want == current {
		return true
	}
	value, err := resource()
	if err != nil {
		audioError(w, err)
		return false
	}
	w.Header().Set("ETag", strconv.Quote(current))
	writeJSON(w, http.StatusConflict, map[string]any{"error": "Đã có thay đổi từ nơi khác", "conflict": true, "version": current, "current": value})
	return false
}

// audioChanged 广播 story.changed{parts:["audio"]}，其它标签页据此刷新。
func (g *Gateway) audioChanged(st library.Story) {
	g.events().Publish(Event{Type: "story.changed", Project: st.Project, Story: st.Story, Data: map[string]any{"parts": []string{"audio"}}})
}

// projectAudioLayout 构造项目级路径（没有故事上下文时只要 assets 与词典路径）。
func projectAudioLayout(root, projectDir string) (audio.Layout, error) {
	cfg, err := library.LoadConfig(root)
	if err != nil {
		return audio.Layout{}, err
	}
	assets := cfg.Dirs.Assets
	if assets == "" {
		assets = "tai-san"
	}
	return audio.Layout{
		AssetsDir:     filepath.Join(projectDir, assets),
		PronouncePath: filepath.Join(projectDir, ".ainovel", "cach-doc.json"),
	}, nil
}

// audioVolumeSource 读取某卷的章节（标题、正文、是否已验收）。
func audioVolumeSource(sr *host.StoryReader, volume int) ([]audio.SourceChapter, string, error) {
	ranges, err := sr.VolumeRanges()
	if err != nil {
		return nil, "", err
	}
	var target *host.VolumeRange
	for i := range ranges {
		if ranges[i].Volume == volume {
			target = &ranges[i]
			break
		}
	}
	if target == nil {
		return nil, "", &audio.StateError{Msg: fmt.Sprintf("Không có tập %d.", volume)}
	}
	completed, err := sr.CompletedSet()
	if err != nil {
		return nil, "", err
	}
	out := make([]audio.SourceChapter, 0, target.To-target.From+1)
	for ch := target.From; ch <= target.To; ch++ {
		sc := audio.SourceChapter{Chapter: ch, Completed: completed[ch]}
		if ct, cerr := sr.ChapterText(ch, "committed"); cerr == nil {
			sc.Text = ct.Text
			sc.Title = ct.Title
		} else if rec, rerr := sr.Store().ChapterRecords.Load(ch); rerr == nil && rec != nil &&
			strings.TrimSpace(rec.Content) != "" {
			// Hosted chapters can be accepted through the narrative/revision
			// table without a chapter_text prose artifact (mobile commit). The
			// accepted record is the authority for that chapter's "chính văn";
			// without this fallback the compose and queue paths would see an
			// empty chapter even though it is committed.
			sc.Text = rec.Content
			if plan, perr := sr.ChapterPlan(ch); perr == nil && plan != nil {
				sc.Title = plan.Title
			}
		} else if plan, perr := sr.ChapterPlan(ch); perr == nil && plan != nil {
			sc.Title = plan.Title
		}
		out = append(out, sc)
	}
	return out, sr.BookTitle(), nil
}

// chapterHashOf 返回章节正文当前指纹（读不到正文时为空，表示"没有可比对的内容"）。
func chapterHashOf(sr *host.StoryReader, chapter int) string {
	ct, err := sr.ChapterText(chapter, "committed")
	if err != nil {
		return ""
	}
	return audio.SourceHash(ct.Text)
}

// chaptersWithoutScript 返回"已验收但还没有 bản đọc"的章节（"Soạn các chương còn lại"）。
func chaptersWithoutScript(source []audio.SourceChapter, volume int, svc audioBackend, ctx context.Context) []int {
	var out []int
	for _, sc := range source {
		if !sc.Completed {
			continue
		}
		if script, err := svc.Script(ctx, volume, sc.Chapter); err == nil && script != nil && len(script.Segments) > 0 {
			continue
		}
		out = append(out, sc.Chapter)
	}
	return out
}

// sortedChaptersInput 清洗 API 传来的章号。
func sortedChaptersInput(in []int) []int {
	seen := map[int]bool{}
	out := make([]int, 0, len(in))
	for _, ch := range in {
		if ch <= 0 || seen[ch] {
			continue
		}
		seen[ch] = true
		out = append(out, ch)
	}
	return out
}

// storyCharacters 把故事角色档案投影成音频模块要的形状。
func storyCharacters(sr *host.StoryReader) (audio.CharacterRef, error) {
	chars, support, err := sr.Characters()
	if err != nil {
		return audio.CharacterRef{}, err
	}
	ref := audio.CharacterRef{Characters: make([]audio.Character, 0, len(chars)), Support: make([]audio.SupportCharacter, 0, len(support))}
	for _, ch := range chars {
		ref.Characters = append(ref.Characters, audio.Character{Name: ch.Name, Aliases: ch.Aliases, Role: ch.Role, Ini: ch.Ini, Kind: string(ch.Kind)})
	}
	for _, e := range support {
		ref.Support = append(ref.Support, audio.SupportCharacter{Name: e.Name})
	}
	return ref, nil
}

// audioComposer 用故事三级配置里的模型（与写作同一份配置，不另设"音频模型"）。
func audioComposer(st library.Story) (audio.Composer, error) {
	cfg, err := bootstrap.LoadConfigAt(st.Dir)
	if err != nil {
		return nil, fmt.Errorf("%w (%v)", audio.ErrNoModel, err)
	}
	if strings.TrimSpace(cfg.Provider) == "" || strings.TrimSpace(cfg.ModelName) == "" {
		return nil, audio.ErrNoModel
	}
	models, err := bootstrap.NewModelSet(cfg)
	if err != nil {
		return nil, fmt.Errorf("%w (%v)", audio.ErrNoModel, err)
	}
	return &audio.ChatComposer{Model: models.Default, ID: cfg.Provider + "/" + cfg.ModelName}, nil
}

// audioComposerForStory resolves the model that composes bản đọc. A story
// directory carries its own config file. Hosted stories have no directory, so
// the gateway reads the same global/project/story chain the engine worker uses
// from the repository; a missing provider/model is the existing ErrNoModel
// (409 needsConfig), never a silent fallback.
func (g *Gateway) audioComposerForStory(ctx context.Context, user identity.User, st library.Story) (audio.Composer, error) {
	if st.Dir != "" {
		return audioComposer(st)
	}
	if g.pgAudio == nil {
		// No story directory and no hosted repository: the previous behaviour
		// (nil composer → 409 needsConfig) is the honest answer.
		return nil, audio.ErrNoModel
	}
	docs, err := g.hostedConfigDocuments()
	if err != nil {
		return nil, err
	}
	_, cfg, err := bootstrap.LoadHostedConfigChain(ctx, docs, configActor(user), st.Project, st.Story)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Provider) == "" || strings.TrimSpace(cfg.ModelName) == "" {
		return nil, audio.ErrNoModel
	}
	models, err := bootstrap.NewModelSet(cfg)
	if err != nil {
		return nil, fmt.Errorf("%w (%v)", audio.ErrNoModel, err)
	}
	return &audio.ChatComposer{Model: models.Default, ID: cfg.Provider + "/" + cfg.ModelName}, nil
}

// audioComposerInternal resolves the story's compose model for background work
// that has no request account (the preparation runner): same hosted
// global/project/story chain, read as the trusted internal actor. A missing
// provider/model stays ErrNoModel — the retryable job error, never a fallback.
func (g *Gateway) audioComposerInternal(ctx context.Context, st library.Story) (audio.Composer, error) {
	if st.Dir != "" {
		return audioComposer(st)
	}
	if g.pgAudio == nil {
		return nil, audio.ErrNoModel
	}
	docs, err := g.hostedConfigDocuments()
	if err != nil {
		return nil, err
	}
	_, cfg, err := bootstrap.LoadHostedConfigChain(ctx, docs, nil, st.Project, st.Story)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Provider) == "" || strings.TrimSpace(cfg.ModelName) == "" {
		return nil, audio.ErrNoModel
	}
	models, err := bootstrap.NewModelSet(cfg)
	if err != nil {
		return nil, fmt.Errorf("%w (%v)", audio.ErrNoModel, err)
	}
	return &audio.ChatComposer{Model: models.Default, ID: cfg.Provider + "/" + cfg.ModelName}, nil
}

// parseSegmentRef 解析 :id（"3-s0007"，或 "s0007" + ?chapter=）与卷号（?volume=，默认 1）。
func parseSegmentRef(r *http.Request, raw string) (volume, chapter int, segID string, err error) {
	volume = 1
	if raw := r.URL.Query().Get("volume"); raw != "" {
		v, cerr := strconv.Atoi(raw)
		if cerr != nil || v <= 0 {
			return 0, 0, "", errors.New("volume không hợp lệ")
		}
		volume = v
	}
	q := r.URL.Query()
	ref := strings.TrimSpace(raw)
	if i := strings.Index(ref, "-"); i > 0 {
		if ch, cerr := strconv.Atoi(ref[:i]); cerr == nil && ch > 0 {
			chapter = ch
			segID = ref[i+1:]
		}
	}
	if chapter == 0 {
		ch, cerr := strconv.Atoi(strings.TrimSpace(q.Get("chapter")))
		if cerr != nil || ch <= 0 {
			return 0, 0, "", errors.New("thiếu số chương cho đoạn này")
		}
		chapter = ch
		segID = ref
	}
	if segID == "" {
		return 0, 0, "", errors.New("thiếu id đoạn")
	}
	return volume, chapter, segID, nil
}

// serveAudioFile 用 ServeContent 输出音频文件（自带 Range 支持）。
func serveAudioFile(w http.ResponseWriter, r *http.Request, path string) {
	f, err := os.Open(path)
	if err != nil {
		jsonError(w, http.StatusNotFound, "không tìm thấy tệp âm thanh")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, filepath.Base(path), info.ModTime(), f)
}

// serveAudioEvents 是 SSE：audio.render / audio.compose / audio.queue / audio.export。
func serveAudioEvents(w http.ResponseWriter, r *http.Request, svc audioBackend) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, http.StatusInternalServerError, "streaming không được hỗ trợ")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, ": audio\n\n")
	flusher.Flush()

	ch := svc.Hub().Subscribe(r.Context())
	keepAlive := time.NewTicker(20 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepAlive.C:
			_, _ = io.WriteString(w, ": ping\n\n")
			flusher.Flush()
		case ev, open := <-ch:
			if !open {
				return
			}
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, data)
			flusher.Flush()
		}
	}
}

// audioError 把音频模块的错误映射成 HTTP 状态 + 越南语文案。
func audioError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	var state *audio.StateError
	switch {
	case errors.As(err, &state):
		jsonError(w, http.StatusConflict, state.Msg)
	case errors.Is(err, audio.ErrNotFound):
		jsonError(w, http.StatusNotFound, "không tìm thấy")
	case errors.Is(err, audio.ErrChapterIncomplete):
		jsonError(w, http.StatusConflict, err.Error())
	case errors.Is(err, audio.ErrFFmpegMissing):
		jsonError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, audio.ErrNoModel):
		writeJSON(w, http.StatusConflict, map[string]any{"error": needsModelMessage, "needsConfig": true})
	case errors.Is(err, audio.ErrEngineMissing):
		jsonError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, voice.ErrNotInstalled):
		jsonError(w, http.StatusServiceUnavailable, "Bộ đọc trên máy chưa được cài. Vào Cài đặt › Giọng đọc để cài.")
	case errors.Is(err, voice.ErrBusy):
		jsonError(w, http.StatusServiceUnavailable, "Bộ đọc đang bận, thử lại sau ít giây.")
	case errors.Is(err, voice.ErrUnsupported):
		jsonError(w, http.StatusBadRequest, err.Error())
	default:
		// Shared storage family (forbidden/not-found/conflict/unsupported),
		// cancellation/timeout and unknown causes all follow the one policy;
		// the handler only keeps the audio-specific envelopes above.
		jsonMappedError(w, err)
	}
}
