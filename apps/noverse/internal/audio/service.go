package audio

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Service 是一个故事的音频服务：目录、设置、分派、队列、事件、渲染与导出都挂在它上面。
// 网关按故事缓存一个 Service（不依赖 worker），所以 TUI 持锁时音频照常可用。

// Synthesizer 是 voice.Service 在本模块的窄接口（15-audio-impl §1）。
// 单独定义而不是直接依赖 internal/voice：单元测试可以塞假引擎，
// 换引擎（或换协议）不需要改音频逻辑。
type Synthesizer interface {
	Capabilities() Capabilities
	Status(ctx context.Context) Status
	Voices(ctx context.Context) ([]Voice, error)
	Synthesize(ctx context.Context, req SynthRequest) (Audio, error)
	AddVoice(ctx context.Context, name, wavPath string) (Voice, error)
}

// Capabilities describes synthesis-only text features.
type Capabilities struct {
	EmotionCues bool
}

// Status 是引擎状态（与 voice.Status 同形，便于一行适配）。
type Status struct {
	Ready  bool     `json:"ready"`
	Engine string   `json:"engine"`
	Model  string   `json:"model"`
	Device string   `json:"device"`
	State  string   `json:"state"` // missing|installing|stopped|starting|ready|error
	Error  string   `json:"error"`
	Note   string   `json:"note,omitempty"`
	RTF    *float64 `json:"rtf"`
	// ModelID 是配置里的模型 id（进缓存键）。Model 是给人看的描述，引擎运行时会带上
	// 采样率/精度（"vieneu-v3-turbo (48000 Hz) · int8"），停着时只有 id——不能拿它当键。
	ModelID string `json:"-"`
}

// SynthRequest 是一段合成请求。引擎忽略 Speed（VieNeu 的 OpenAI 兼容服务没有语速参数），
// 语速与变调由本模块用 ffmpeg 在缓存前处理。
type SynthRequest struct {
	Text     string
	VoiceID  string
	RefAudio string
	Tone     string
	Quality  string
	Speed    float64
	Pitch    float64
}

// Audio 是一段合成结果。
type Audio struct {
	WAV         []byte
	SampleRate  int
	DurationSec float64
}

// Voice 是引擎可用的一个声音（预设或克隆）。
type Voice struct {
	ID          string
	Name        string
	Gender      string
	Source      string // builtin|project|clone
	File        string
	DurationSec float64
}

// ErrEngineMissing 表示本机还没装好读音引擎（handler 映射 503，脚本功能仍可用）。
var ErrEngineMissing = errors.New("Bộ đọc trên máy chưa sẵn sàng. Vào Cài đặt › Giọng đọc để cài bộ đọc.")

// NullSynth 是"还没有引擎"的实现：状态如实报告 missing，合成直接报错。
// 它存在的意义是让"soạn/duyệt bản đọc"在没装引擎时完全可用（15-audio-impl §3）。
type NullSynth struct{ Reason string }

func (NullSynth) Capabilities() Capabilities { return Capabilities{} }

// Status 报告 missing。
func (n NullSynth) Status(context.Context) Status {
	reason := n.Reason
	if reason == "" {
		reason = ErrEngineMissing.Error()
	}
	return Status{State: "missing", Error: reason}
}

// Voices 没有声音。
func (n NullSynth) Voices(context.Context) ([]Voice, error) { return nil, ErrEngineMissing }

// Synthesize 直接报错。
func (n NullSynth) Synthesize(context.Context, SynthRequest) (Audio, error) {
	return Audio{}, ErrEngineMissing
}

// AddVoice 不支持。
func (n NullSynth) AddVoice(context.Context, string, string) (Voice, error) {
	return Voice{}, ErrEngineMissing
}

// SourceChapter 是音频模块需要的章节信息（由 gateway 用 StoryReader 组装）。
type SourceChapter struct {
	Chapter   int
	Title     string
	Text      string
	Completed bool
}

// VolumeView 对应 GET S/audio/volumes/:v。
type VolumeView struct {
	Chapters []ChapterView `json:"chapters"`
}

// ChapterView 是一章的音频状态（左栏那几行）。
type ChapterView struct {
	Chapter         int               `json:"chapter"`
	Title           string            `json:"title"`
	Status          string            `json:"status"`
	Issues          int               `json:"issues"`
	DurationSec     *float64          `json:"durationSec"`
	LockedAt        *time.Time        `json:"lockedAt"`
	Segments        int               `json:"segments"`
	Speakers        int               `json:"speakers"`
	Audio           *ChapterAudioView `json:"audio"`
	ChangedSegments int               `json:"changedSegments"`
	ChapterChanged  bool              `json:"chapterChanged"`
	ScriptChanged   bool              `json:"scriptChanged"`
	StaleReason     string            `json:"staleReason"`
	// UpdateEtaSec 是"只读回改动段"的生成耗时估计（§AK）：引擎报过实测 RTF
	// 才有值；测不出来时整个键缺省（界面显示"chưa đo"）。
	UpdateEtaSec *float64 `json:"updateEtaSec,omitempty"`
}

// ChapterAudioView 是"xong · 16:40 / đang tạo 38% / 12/46 đoạn có giọng"。
type ChapterAudioView struct {
	SegmentsDone int      `json:"segmentsDone"`
	Segments     int      `json:"segments"`
	DurationSec  *float64 `json:"durationSec"`
	Rendering    bool     `json:"rendering"`
	Percent      float64  `json:"percent"`
	// Version là dấu vân tay nội dung audio cả chương khi đã đủ giọng; client gắn
	// vào URL phát để thẻ audio tải lại sau khi chương được dựng lại.
	Version string `json:"version,omitempty"`
}

// ScriptView 对应 GET S/audio/volumes/:v/scripts/:ch。
type ScriptView struct {
	Chapter        int           `json:"chapter"`
	Volume         int           `json:"volume"`
	Status         string        `json:"status"`
	LockedAt       *time.Time    `json:"lockedAt"`
	SourceHash     string        `json:"sourceHash"`
	Stale          bool          `json:"stale"`
	Segments       []SegmentView `json:"segments"`
	ChapterChanged bool          `json:"chapterChanged"`
	ScriptChanged  bool          `json:"scriptChanged"`
	StaleReason    string        `json:"staleReason"`
}

// SegmentView 是段落 + "这段有没有音频 / 要不要重读"。
type SegmentView struct {
	Segment
	// Keep list fields present in the API even when the persisted segment omits them.
	Normalized []Change          `json:"normalized"`
	Flags      []string          `json:"flags"`
	Changed    bool              `json:"changed"`
	Audio      *SegmentAudioView `json:"audio"`
}

// SegmentAudioView 给界面直接用的音频状态。
type SegmentAudioView struct {
	URL         string  `json:"url"`
	State       string  `json:"state"` // none|ready|stale
	DurationSec float64 `json:"durationSec"`
}

// ReadyView 是"这台机器能不能出声"的汇总（Audio 屏缺 ffmpeg/引擎时的引导卡）。
type ReadyView struct {
	FFmpeg  bool   `json:"ffmpeg"`
	Engine  Status `json:"engine"`
	Ready   bool   `json:"ready"`
	Message string `json:"message"`
}

// Service 是单故事的音频服务。
type Service struct {
	layout         Layout
	synth          Synthesizer
	hub            *Hub
	queue          *Queue
	queuedComposer QueueComposer

	mu            sync.Mutex
	settings      Settings
	composing     map[chapterKey]bool
	exports       map[int]ExportRequest
	exportCancels map[int]context.CancelFunc
	basePath      string
	// Gói AF: 故事名与 {Tên ngắn}（story.json title/shortTitle，handler 在
	// 建服务/发导出时注入）。文件名、专辑名、生成封面都用它；空时 runExport
	// 退回 bookTitle。
	storyName  string
	shortTitle string

	ctx    context.Context
	cancel context.CancelFunc

	runMu   sync.Mutex
	wake    bool
	running bool
	runWG   sync.WaitGroup
	compWG  sync.WaitGroup

	// buildMu 串行化"拼整章 / 拼整卷"这类重活（一次 ffmpeg 链，别让两个请求同时压 CPU）。
	buildMu sync.Mutex

	// exportFileMu 序列化 pending-exports.json 的落盘，且必须在 s.mu 之外持有：
	// fsync 可能因磁盘压力停顿数秒，不能让所有进度读者跟着卡住。
	exportFileMu sync.Mutex

	identMu     sync.Mutex
	identAt     time.Time
	identEngine string
	identModel  string
}

// NewService 创建服务：建目录、读设置与队列，并在有排队任务且引擎就绪时自动续跑。
func NewService(layout Layout, synth Synthesizer, composer ...QueueComposer) (*Service, error) {
	if err := layout.ensureAudioDirs(); err != nil {
		return nil, err
	}
	q, err := LoadQueue(layout.queuePath())
	if err != nil {
		return nil, err
	}
	settings, err := ReadSettings(layout)
	if err != nil {
		return nil, err
	}
	if synth == nil {
		synth = NullSynth{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		layout:    layout,
		synth:     synth,
		hub:       NewHub(),
		queue:     q,
		settings:  settings,
		composing: map[chapterKey]bool{},
		ctx:       ctx,
		cancel:    cancel,
	}
	if len(composer) > 0 {
		s.queuedComposer = composer[0]
	}
	if err := s.loadExportRequests(); err != nil {
		cancel()
		return nil, err
	}
	if job, ok := q.NextQueued(); ok {
		if job.NeedsScript || synthUsable(synth.Status(ctx)) {
			s.kick()
		}
	} else if len(s.exports) > 0 {
		s.kick()
	}
	return s, nil
}

// Close 停止后台渲染与事件订阅。
func (s *Service) Close() {
	s.cancel()
	s.hub.Close()
	// 渲染/拼音频在 ctx 取消后自然退出；最多等 5 秒再放手（避免进程关停时把 queue.json 写一半）。
	done := make(chan struct{})
	go func() {
		s.runWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
	// soạn bản đọc 用独立的 ctx，不等模型返回。
	done = make(chan struct{})
	go func() {
		s.compWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
}

// Layout 暴露目录（handler 读导出/章节文件用）。
func (s *Service) Layout() Layout { return s.layout }

// Hub 暴露事件中心（SSE）。
func (s *Service) Hub() *Hub { return s.hub }

// SetBasePath 设置对外 URL 前缀（"/api/projects/:p/stories/:s"），用于生成段音频地址。
func (s *Service) SetBasePath(p string) {
	s.mu.Lock()
	s.basePath = strings.TrimSuffix(p, "/")
	s.mu.Unlock()
}

func (s *Service) base() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.basePath
}

// SetStoryNames injects the canonical title (book metadata, then story.json) and shortTitle.
func (s *Service) SetStoryNames(title, shortTitle string) {
	s.mu.Lock()
	s.storyName = strings.TrimSpace(title)
	s.shortTitle = strings.TrimSpace(shortTitle)
	s.mu.Unlock()
}

// storyMeta 返回导出用的 {tên truyện, tên ngắn}：shortTitle 缺省则按 A7
// 规则从标题截断（词边界 ≤32）。
func (s *Service) storyMeta(bookTitle string) (storyTitle, short string) {
	s.mu.Lock()
	title, st := s.storyName, s.shortTitle
	s.mu.Unlock()
	if canonical := CleanStoryTitle(bookTitle); canonical != "" {
		title = canonical
	} else {
		title = CleanStoryTitle(title)
	}
	if title == "" {
		title = "Truyện"
	}
	return title, ShortName(st, title)
}

// Settings 返回当前设置。
func (s *Service) Settings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

// SaveSettings 校验并保存设置。
func (s *Service) SaveSettings(in Settings) (Settings, error) {
	if !ValidateQuality(in.Quality) {
		return Settings{}, conflict("quality phải là fast, standard hoặc high")
	}
	s.mu.Lock()
	s.settings = in
	s.mu.Unlock()
	if err := WriteSettings(s.layout, in); err != nil {
		return Settings{}, err
	}
	return in, nil
}

// Pronunciations gộp từ điển gói dưới từ điển dự án (mục người dùng thắng).
func (s *Service) Pronunciations() ([]Pronunciation, error) {
	user, err := ReadPronunciations(s.layout)
	if err != nil || s.layout.Pack == nil || len(s.layout.Pack.Dictionary) == 0 {
		return user, err
	}
	merged := make([]Pronunciation, 0, len(s.layout.Pack.Dictionary)+len(user))
	merged = append(merged, s.layout.Pack.Dictionary...)
	merged = append(merged, user...)
	return NormalizePronunciations(merged)
}

// SavePronunciations 保存项目读音词典。
func (s *Service) SavePronunciations(entries []Pronunciation) ([]Pronunciation, error) {
	if err := WritePronunciations(s.layout, entries); err != nil {
		return nil, err
	}
	return s.Pronunciations()
}

// Cast 返回分派；不存在时按故事角色初始化并落盘（首次打开面板就有内容）。
func (s *Service) Cast(chars CharacterRef) (*Cast, error) {
	cur, err := ReadCast(s.layout)
	if err != nil {
		return nil, err
	}
	merged, err := s.ensureCast(cur, chars)
	if err != nil {
		return nil, err
	}
	if cur == nil {
		c := merged
		if err := WriteCast(s.layout, &c); err != nil {
			return nil, err
		}
		return &c, nil
	}
	if len(merged.Speakers) != len(cur.Speakers) {
		if err := WriteCast(s.layout, &merged); err != nil {
			return nil, err
		}
	}
	return &merged, nil
}

// SaveCast 保存分派。
func (s *Service) SaveCast(c *Cast) (*Cast, error) {
	if c == nil {
		return nil, errors.New("phân vai rỗng")
	}
	if c.Mode != ModeCast && c.Mode != ModeSingle {
		return nil, conflict("mode phải là cast hoặc single")
	}
	for i := range c.Speakers {
		c.Speakers[i].ID = strings.TrimSpace(c.Speakers[i].ID)
		c.Speakers[i].Name = strings.TrimSpace(c.Speakers[i].Name)
		if c.Speakers[i].ID == "" || c.Speakers[i].Name == "" {
			return nil, conflict("mỗi vai cần id và tên")
		}
	}
	if err := WriteCast(s.layout, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Voices 合并"giọng của dự án"与引擎预设（builtIn 区分）。
func (s *Service) Voices(ctx context.Context) ([]ProjectVoice, error) {
	return ListVoices(ctx, s.layout, s.synth)
}

// voiceListTimeout là trần chờ máy đọc trả danh sách giọng cho các màn chỉ đọc.
// Máy đọc chưa bật thì lần nạp mô hình đầu mất hàng chục giây tới vài phút; màn
// đọc không được đứng chờ việc đó (tập audio, phân vai). Thiếu preset chỉ thiếu
// tên giọng có sẵn — giọng của dự án vẫn đủ, và khoá "cần tạo lại" không phụ
// thuộc danh sách này (ResolveVoice lấy VoiceID từ vai, không từ preset).
var voiceListTimeout = 2 * time.Second

// ListVoices 合并项目声音与引擎预设；引擎 chưa sẵn sàng (hoặc quá voiceListTimeout)
// thì chỉ trả giọng của dự án — panel vẫn mở được thay vì treo chờ nạp mô hình.
func ListVoices(ctx context.Context, layout Layout, synth Synthesizer) ([]ProjectVoice, error) {
	project, err := ReadProjectVoices(layout)
	if err != nil {
		return nil, err
	}
	out := append([]ProjectVoice{}, project...)
	if synth == nil {
		return out, nil
	}
	engine := synthVoices(ctx, synth)
	for _, v := range engine {
		out = append(out, ProjectVoice{
			ID:          v.ID,
			Name:        v.Name,
			File:        v.File,
			DurationSec: v.DurationSec,
			Gender:      v.Gender,
			BuiltIn:     true,
		})
	}
	return out, nil
}

// synthVoices hỏi giọng có sẵn của máy đọc với trần voiceListTimeout; lỗi hay hết
// giờ đều trả rỗng. Đây là dữ liệu phụ của màn đọc nên không được phép chặn nó —
// và việc máy đọc tự bật tiếp tục ở nền (engine giữ lượt bật, không theo ctx này).
func synthVoices(ctx context.Context, synth Synthesizer) []Voice {
	if synth == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	callCtx, cancel := context.WithTimeout(ctx, voiceListTimeout)
	defer cancel()
	list, err := synth.Voices(callCtx)
	if err != nil {
		return nil
	}
	return list
}

// Ready 汇总 ffmpeg 与引擎状态。
func (s *Service) Ready(ctx context.Context) ReadyView {
	st := s.synth.Status(ctx)
	view := ReadyView{FFmpeg: FFmpegAvailable(), Engine: st}
	switch {
	case !view.FFmpeg:
		view.Message = ErrFFmpegMissing.Error()
	case !synthUsable(st):
		view.Message = st.Error
		if view.Message == "" {
			view.Message = ErrEngineMissing.Error()
		}
	default:
		view.Ready = true
		view.Message = "Sẵn sàng tạo giọng đọc trên máy."
	}
	return view
}

// synthUsable 报告"能不能发起合成"。与 Ready() 不同：引擎已装好但进程
// 没跑（stopped）也是可用的——Synthesize 会自动 bật lại。
func synthUsable(st Status) bool {
	return st.Ready || st.State == "stopped" || st.State == "starting"
}

// engineIdent 返回当前引擎标识（进缓存键）。缓存 30 秒，避免每次列表都问引擎。
func (s *Service) engineIdent(ctx context.Context) (string, string) {
	s.identMu.Lock()
	if time.Since(s.identAt) < 30*time.Second {
		engine, model := s.identEngine, s.identModel
		s.identMu.Unlock()
		return engine, model
	}
	s.identMu.Unlock()
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	st := s.synth.Status(callCtx)
	engine, model := statusIdent(st)
	s.identMu.Lock()
	s.identAt = time.Now()
	s.identEngine, s.identModel = engine, model
	s.identMu.Unlock()
	return engine, model
}

// statusIdent 取引擎的稳定标识：优先 ModelID，没有才退回 Model。
func statusIdent(st Status) (string, string) {
	if st.ModelID != "" {
		return st.Engine, st.ModelID
	}
	return st.Engine, st.Model
}

// ── 卷 / 章 视图 ──

// Volumes 汇总一卷里每章的音频状态。
func (s *Service) Volumes(ctx context.Context, source []SourceChapter, volume int) (VolumeView, error) {
	settings := s.Settings()
	cast, err := s.castForRender()
	if err != nil {
		return VolumeView{}, err
	}
	voices, err := s.Voices(ctx)
	if err != nil {
		return VolumeView{}, err
	}
	voiceMap := VoiceMap(voices)
	engine, model := s.engineIdent(ctx)
	// §AK updateEtaSec: một lần hỏi RTF cho cả tập, không hỏi từng chương.
	rtf := s.synth.Status(ctx).RTF
	jobs := s.queue.Snapshot()
	out := VolumeView{Chapters: make([]ChapterView, 0, len(source))}
	for _, sc := range source {
		script, err := ReadScript(s.layout, volume, sc.Chapter)
		if err != nil {
			return VolumeView{}, err
		}
		hash := ""
		if strings.TrimSpace(sc.Text) != "" {
			hash = SourceHash(sc.Text)
		}
		s.mu.Lock()
		composing := s.composing[chapterKey{volume: volume, chapter: sc.Chapter}]
		s.mu.Unlock()
		view := ChapterView{
			Chapter: sc.Chapter,
			Title:   sc.Title,
			Status:  DeriveStatus(script, composing, hash),
		}
		if script != nil {
			view.ChapterChanged, view.ScriptChanged, view.StaleReason = script.freshness(hash)
			// no_voice phụ thuộc phân vai hiện tại, không phải lúc soạn.
			applyVoiceFlags(script, cast)
			view.LockedAt = script.LockedAt
			view.Issues = script.IssueSegments()
			view.Segments = len(script.Segments)
			view.Speakers = script.SpeakerCount()
			done, duration := script.AudioStats()
			view.ChangedSegments = script.ChangedSegments(func(seg Segment) string {
				return s.segmentKey(seg, cast, voiceMap, settings, engine, model).Hash()
			})
			view.UpdateEtaSec = s.updateETA(script, cast, voiceMap, settings, engine, model, rtf)
			audioView := &ChapterAudioView{SegmentsDone: done, Segments: len(script.Segments)}
			if len(script.Segments) > 0 {
				audioView.Percent = float64(done) / float64(len(script.Segments)) * 100
			}
			if done == len(script.Segments) && len(script.Segments) > 0 {
				d := duration
				view.DurationSec = &d
				audioView.DurationSec = &d
				// Cùng một URL cho mọi lần dựng lại: thẻ audio đang mở (nhất là trên
				// điện thoại) sẽ giữ tệp cũ. Version đổi khi nội dung audio đổi.
				audioView.Version = chapterHash(script, cast)[:16]
			}
			view.Audio = audioView
		}
		for _, j := range jobs {
			if j.Volume != volume || j.Chapter != sc.Chapter {
				continue
			}
			if view.Audio == nil {
				view.Audio = &ChapterAudioView{}
			}
			if j.State == JobRendering || j.State == JobPaused || j.State == JobQueued {
				view.Audio.Rendering = j.State == JobRendering
				if j.Segments > 0 {
					view.Audio.Percent = float64(j.SegmentsDone) / float64(j.Segments) * 100
				}
			}
		}
		out.Chapters = append(out.Chapters, view)
	}
	return out, nil
}

// updateETA 估算"只读回要重读的段"需要多久（§AK updateEtaSec）：
// 键对不上或还没有音频的段，取已知时长（没有就按字数估）× 引擎实测 RTF。
// rtf 由调用方取一次（引擎没报过就是 nil）；测不出时返回 nil（JSON 缺省，
// 界面显示"chưa đo"而不是编一个数）。
func (s *Service) updateETA(script *Script, cast *Cast, voices map[string]ProjectVoice, settings Settings, engine, model string, rtf *float64) *float64 {
	if script == nil || len(script.Segments) == 0 || rtf == nil || *rtf <= 0 {
		return nil
	}
	pending := 0.0
	for _, seg := range script.Segments {
		key := s.segmentKey(seg, cast, voices, settings, engine, model).Hash()
		if seg.Rendered != nil && seg.Rendered.Key == key && seg.Rendered.DurationSec > 0 {
			continue
		}
		if seg.Rendered != nil && seg.Rendered.DurationSec > 0 {
			pending += seg.Rendered.DurationSec
		} else {
			pending += (&Script{Segments: []Segment{seg}}).EstimatedDuration()
		}
	}
	if pending <= 0 {
		return nil
	}
	eta := pending * *rtf
	return &eta
}

// castForRender 读分派；缺失时返回空分派（不落盘——只有打开面板才初始化）。
func (s *Service) castForRender() (*Cast, error) {
	c, err := ReadCast(s.layout)
	if err != nil {
		return nil, err
	}
	if c == nil {
		c = &Cast{Mode: ModeCast, Single: SingleCast{DialogueShift: true, KeepTones: true}}
		if s.layout.Pack != nil {
			fresh, err := s.ensureCast(nil, CharacterRef{})
			if err != nil {
				return nil, err
			}
			c = &fresh
		}
	}
	return c, nil
}

// segmentKey 是某段"按当前参数应该有的缓存键"。
func (s *Service) segmentKey(seg Segment, cast *Cast, voices map[string]ProjectVoice, settings Settings, engine, model string) SynthKey {
	ref := ResolveVoice(cast, seg, voices)
	tone := seg.Tone
	if cast.Mode == ModeSingle && !cast.Single.KeepTones {
		tone = ""
	}
	speed := 1.0
	pitch := ref.Pitch
	voiceID := ref.VoiceID
	if seg.Rendered != nil && seg.Rendered.Rerendered {
		voiceID = seg.Rendered.VoiceID
		tone = seg.Rendered.Tone
		speed = seg.Rendered.Speed
		pitch = seg.Rendered.Pitch
	}
	if engine == "" && seg.Rendered != nil {
		engine = seg.Rendered.Engine
	}
	if model == "" && seg.Rendered != nil {
		model = seg.Rendered.Model
	}
	return SynthKey{
		Text: s.synthesisText(seg.Text, tone), VoiceID: voiceID, Tone: tone, Speed: speed, Pitch: pitch,
		Quality: settings.Quality, Engine: engine, Model: model,
	}
}

// ScriptView 返回一章的 bản đọc（含每段音频状态）。
func (s *Service) ScriptView(ctx context.Context, volume, chapter int, currentHash string) (*ScriptView, error) {
	script, err := ReadScript(s.layout, volume, chapter)
	if err != nil {
		return nil, err
	}
	if script == nil {
		return nil, ErrNotFound
	}
	settings := s.Settings()
	cast, err := s.castForRender()
	if err != nil {
		return nil, err
	}
	applyVoiceFlags(script, cast)
	voices, err := s.Voices(ctx)
	if err != nil {
		return nil, err
	}
	voiceMap := VoiceMap(voices)
	engine, model := s.engineIdent(ctx)
	view := &ScriptView{
		Chapter:    script.Chapter,
		Volume:     script.Volume,
		Status:     script.Status,
		LockedAt:   script.LockedAt,
		SourceHash: script.SourceHash,
		Stale:      script.Stale(currentHash),
		Segments:   make([]SegmentView, 0, len(script.Segments)),
	}
	view.ChapterChanged, view.ScriptChanged, view.StaleReason = script.freshness(currentHash)
	for _, seg := range script.Segments {
		key := s.segmentKey(seg, cast, voiceMap, settings, engine, model)
		sv := SegmentView{Segment: seg}
		sv.Normalized = seg.Normalized
		if sv.Normalized == nil {
			sv.Normalized = []Change{}
		}
		sv.Flags = seg.Flags
		if sv.Flags == nil {
			sv.Flags = []string{}
		}
		switch {
		case seg.Rendered == nil:
			sv.Audio = &SegmentAudioView{State: "none", URL: s.segmentURL(script.Volume, script.Chapter, seg.ID)}
		case seg.Rendered.Key != key.Hash():
			sv.Changed = true
			sv.Audio = &SegmentAudioView{State: "stale", URL: s.segmentURL(script.Volume, script.Chapter, seg.ID), DurationSec: seg.Rendered.DurationSec}
		default:
			sv.Audio = &SegmentAudioView{State: "ready", URL: s.segmentURL(script.Volume, script.Chapter, seg.ID), DurationSec: seg.Rendered.DurationSec}
		}
		view.Segments = append(view.Segments, sv)
	}
	return view, nil
}

// segmentURL 返回段的音频地址（web 层再补 story 前缀）。
func (s *Service) segmentURL(volume, chapter int, segID string) string {
	return fmt.Sprintf("%s/audio/segments/%d-%s/audio?volume=%d", s.base(), chapter, segID, volume)
}

// Script 返回原始 script（内部用）。
func (s *Service) Script(volume, chapter int) (*Script, error) {
	return ReadScript(s.layout, volume, chapter)
}

// ── 锁定 / 解锁 ──

// Lock records approval without claiming that the script matches today's text.
func (s *Service) Lock(ctx context.Context, volume, chapter int, currentHash string) (*Script, error) {
	script, err := ReadScript(s.layout, volume, chapter)
	if err != nil {
		return nil, err
	}
	if script == nil || len(script.Segments) == 0 {
		return nil, conflict("Chương %d chưa có bản đọc để chốt.", chapter)
	}
	if err := foldScriptUnspeakable(script); err != nil {
		return nil, err
	}
	now := time.Now()
	script.Status = StatusLocked
	script.LockedAt = &now
	// Only composition can establish source provenance. A missing legacy hash
	// stays unknown so the next render composes from the current chapter.
	if err := WriteScript(s.layout, script); err != nil {
		return nil, err
	}
	if s.Settings().AutoOnLock {
		if _, err := s.Enqueue(volume, chapter, currentHash); err != nil {
			return nil, err
		}
	}
	s.hub.Publish(NewEvent(EventQueue, map[string]any{"jobs": s.queue.Snapshot()}))
	return script, nil
}

// Unlock 解锁一章：回到 review，取消该章未完成的任务（已生成的段保留）。
func (s *Service) Unlock(volume, chapter int) (*Script, error) {
	script, err := ReadScript(s.layout, volume, chapter)
	if err != nil {
		return nil, err
	}
	if script == nil {
		return nil, ErrNotFound
	}
	script.Status = StatusReview
	script.LockedAt = nil
	if err := WriteScript(s.layout, script); err != nil {
		return nil, err
	}
	for _, j := range s.queue.Snapshot() {
		if j.Volume != volume || j.Chapter != chapter {
			continue
		}
		if j.State == JobQueued || j.State == JobRendering || j.State == JobPaused {
			s.cancelJob(j.ID)
		}
	}
	return script, nil
}

// ── 队列 ──

// Queue 返回队列快照。
func (s *Service) Queue() []Job { return s.queue.Snapshot() }

// Enqueue 排队一章并唤醒渲染循环。currentHash 是当前正文指纹（调用方有就给，
// 没有传 ""）：用 freshness 判定任务是 create 还是 update（§AK kind）。
func (s *Service) Enqueue(volume, chapter int, currentHash ...string) (string, error) {
	script, err := ReadScript(s.layout, volume, chapter)
	if err != nil {
		return "", err
	}
	if script == nil || len(script.Segments) == 0 {
		return "", conflict("Chương %d chưa có bản đọc để tạo giọng.", chapter)
	}
	if script.Status != StatusLocked {
		return "", conflict("Chương %d chưa chốt — hãy chốt bản đọc trước khi tạo giọng.", chapter)
	}
	hash := ""
	if len(currentHash) > 0 {
		hash = currentHash[0]
	}
	kind := JobKindCreate
	if cc, sc, _ := script.freshness(hash); cc || sc {
		kind = JobKindUpdate
	}
	id, _ := s.queue.Enqueue(volume, chapter)
	s.queue.Update(id, func(j *Job) {
		j.Kind = kind
		j.Segments = len(script.Segments)
		j.Quality = s.Settings().Quality
		// create 从已读段接着算；update 的进度只算要重读的段（runJob 重新数）。
		j.SegmentsDone = 0
		if kind == JobKindCreate {
			j.SegmentsDone, _ = script.AudioStats()
		}
	})
	s.hub.Publish(NewEvent(EventQueue, map[string]any{"jobs": s.queue.Snapshot()}))
	s.kick()
	return id, nil
}

// Pause 暂停任务。
func (s *Service) Pause(jobID string) error {
	job, ok := s.queue.Find(jobID)
	if !ok {
		return ErrNotFound
	}
	if job.State != JobQueued && job.State != JobRendering {
		return conflict("Chỉ tạm dừng được việc đang chờ hoặc đang tạo.")
	}
	s.queue.Update(jobID, func(j *Job) { j.State = JobPaused })
	s.hub.Publish(NewEvent(EventQueue, map[string]any{"jobs": s.queue.Snapshot()}))
	return nil
}

// Resume 继续任务。
func (s *Service) Resume(jobID string) error {
	job, ok := s.queue.Find(jobID)
	if !ok {
		return ErrNotFound
	}
	if job.State != JobPaused && job.State != JobError {
		return conflict("Chỉ tiếp tục được việc đang tạm dừng hoặc bị lỗi.")
	}
	s.queue.Update(jobID, func(j *Job) {
		j.State = JobQueued
		j.Error = ""
		j.ErrorAt = nil
	})
	s.hub.Publish(NewEvent(EventQueue, map[string]any{"jobs": s.queue.Snapshot()}))
	s.kick()
	return nil
}

// Cancel 取消任务（已生成的段保留）。
func (s *Service) Cancel(jobID string) error {
	job, ok := s.queue.Find(jobID)
	if !ok {
		return ErrNotFound
	}
	if job.State == JobDone {
		return conflict("Việc này đã xong.")
	}
	s.cancelJob(jobID)
	return nil
}

func (s *Service) cancelJob(jobID string) {
	s.queue.Update(jobID, func(j *Job) {
		j.State = JobCanceled
		j.CurrentText = ""
		j.SegmentID = ""
	})
	s.hub.Publish(NewEvent(EventQueue, map[string]any{"jobs": s.queue.Snapshot()}))
}

// kick 在没有渲染循环时启动一个（每个 Service 同时只跑一条，引擎本身也是串行的）。
func (s *Service) kick() {
	s.runMu.Lock()
	if s.running {
		s.wake = true
		s.runMu.Unlock()
		return
	}
	s.running = true
	s.runMu.Unlock()
	s.runWG.Add(1)
	go func() {
		defer s.runWG.Done()
		for {
			s.runJobs()
			s.runMu.Lock()
			if s.wake {
				s.wake = false
				s.runMu.Unlock()
				continue
			}
			s.running = false
			s.runMu.Unlock()
			return
		}
	}()
}

// logf 默认写 slog；测试可替换。
func (s *Service) logf(msg string, args ...any) {
	slog.Info(msg, args...)
}

// chapterAudioDir 返回章节播放文件目录（测试/清理用）。
func (s *Service) chapterAudioDir(volume int) string {
	return filepath.Join(s.layout.AudioDir, chaptersSubdir, fmt.Sprintf("tap-%02d", volume))
}

// sortJobs 让队列输出稳定（创建时间升序）。
func sortJobs(jobs []Job) {
	sort.SliceStable(jobs, func(i, j int) bool { return jobs[i].CreatedAt.Before(jobs[j].CreatedAt) })
}

// chapterKey 标识"某卷某章"（composing 集合的键）。
type chapterKey struct {
	volume  int
	chapter int
}

// setComposing 标记/清除"正在 soạn"。
func (s *Service) setComposing(volume, chapter int, on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := chapterKey{volume: volume, chapter: chapter}
	if on {
		s.composing[key] = true
		return
	}
	delete(s.composing, key)
}

// isComposing 查询是否正在 soạn。
func (s *Service) isComposing(volume, chapter int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.composing[chapterKey{volume: volume, chapter: chapter}]
}

// CharacterRef 是"故事角色"在本模块的输入（避免 audio 依赖 host/store）。
type CharacterRef struct {
	Characters []Character
	Support    []SupportCharacter
}

// Character 是核心角色档案的最小投影。
type Character struct {
	Name    string
	Aliases []string
	Role    string
	Ini     string // 用户自定头像缩写，生成 speaker 时继承
	Kind    string // loại vai (lead/antagonist/ally/support)，头像上色用（Gói S）
}

// SupportCharacter 是配角名册的最小投影。
type SupportCharacter struct {
	Name string
}
