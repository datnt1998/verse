package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/host/exp"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/identity"
)

// openStoryReader mở đầu đọc cho một truyện: file backend → StoryReader từ
// thư mục; hosted (factStore factory đã cài) → reader trên store scoped,
// resolve đã kiểm membership của user. Lỗi lưu trữ lan đúng nguyên trạng —
// đường proxy/handler quyết 404/403/500.
func (g *Gateway) openStoryReader(user identity.User, st library.Story) (*host.StoryReader, error) {
	if g.factStore != nil {
		engine, err := g.openFactStore(st, &user)
		if err != nil {
			return nil, err
		}
		return host.NewStoryReaderWithStore(engine), nil
	}
	return host.NewStoryReader(st.Dir), nil
}

// serveStoryRead 处理 "/api/projects/:p/stories/:s" 之后的直读端点（契约 §4）。
// Đọc từ store của truyện (file hoặc hosted), không giữ lease của worker.
// rest = 故事前缀后的剩余路径（以 "/" 开头，或为 ""）。
// 返回 false 表示路径不属于本钩子，调用方继续转 worker 代理。
func (g *Gateway) serveStoryRead(w http.ResponseWriter, r *http.Request, user identity.User, st library.Story, rest string) bool {
	rest = "/" + strings.TrimPrefix(rest, "/")
	sr, err := g.openStoryReader(user, st)
	if err != nil {
		// Repository resolve failure (missing membership, deleted story) is a
		// scoped 404/403 — never an existence leak; anything else follows the
		// shared policy (logged server-side, generic 500).
		jsonMappedError(w, err)
		return true
	}

	head, tail, _ := strings.Cut(strings.TrimPrefix(rest, "/"), "/")
	switch {
	case head == "events" && tail == "" && r.Method == http.MethodGet:
		g.serveActivityHistory(w, r, user, sr)
	case head == "chapters" && tail == "":
		if r.Method != http.MethodGet {
			return false
		}
		g.serveChaptersList(w, r, sr)
	case head == "chapters":
		if r.Method != http.MethodGet {
			return false
		}
		g.serveChapter(w, r, sr, tail)
	case head == "outline" && tail == "":
		if r.Method != http.MethodGet {
			return false
		}
		g.serveOutline(w, r, sr)
	case head == "premise" && tail == "":
		if r.Method != http.MethodGet {
			return false
		}
		premise, err := sr.Premise()
		if err != nil {
			jsonMappedError(w, err)
			break
		}
		writeJSON(w, http.StatusOK, premise)
	case head == "world-rules" && tail == "":
		if r.Method != http.MethodGet {
			return false
		}
		rules, err := sr.WorldRules()
		if err != nil {
			jsonMappedError(w, err)
			break
		}
		writeJSON(w, http.StatusOK, rules)
	case head == "characters" && tail == "":
		switch r.Method {
		case http.MethodGet:
			g.serveCharacters(w, r, sr)
		case http.MethodPut:
			g.serveCharactersUpdate(w, r, st, sr)
		default:
			return false
		}
	case head == "foreshadow" && tail == "":
		if r.Method != http.MethodGet {
			return false
		}
		g.serveForeshadow(w, r, sr)
	case head == "quality" && tail == "":
		if r.Method != http.MethodGet {
			return false
		}
		g.serveQuality(w, r, st, sr)
	case head == "reviews":
		if r.Method != http.MethodGet {
			return false
		}
		g.serveReviews(w, r, st, sr, tail)
	case head == "review":
		// GAP-47: POST/DELETE …/review/items/{itemId}/keep
		return g.serveReviewKeep(w, r, user, st, sr, tail)
	case head == "export" && tail == "":
		if r.Method != http.MethodGet {
			return false
		}
		g.serveExportDownload(w, r, st, sr)
	case head == "exports" && tail == "":
		switch r.Method {
		case http.MethodGet:
			g.serveExportsList(w, r, st, sr)
		case http.MethodPost:
			g.serveExportCreate(w, r, st, sr)
		default:
			return false
		}
	case head == "exports" && tail == "file":
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			return false
		}
		g.serveExportFile(w, r, st, sr)
	case head == "exports" && tail == "download":
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			return false
		}
		g.serveExportDownloadFile(w, r, st, sr)
	case head == "cover" && tail == "":
		if r.Method != http.MethodGet {
			return false
		}
		g.serveStoryCover(w, r, st, sr)
	case head == "exports":
		jsonError(w, http.StatusNotFound, "không tìm thấy")
	case head == "rules":
		g.serveStoryRules(w, r, st, sr, tail)
	case head == "style-samples":
		g.serveStyleSamples(w, r, user, st, tail)
	case head == "frame" && tail == "":
		g.serveStoryFrame(w, r, st, sr)
	case head == "retention" && tail == "":
		g.serveStoryRetention(w, r, st, sr)
	case head == "metrics" && tail == "":
		g.serveStoryMetrics(w, r, st, sr)
	case head == "address-terms" || head == "relations" || head == "brain":
		if !g.serveStoryNao(w, r, st, sr, "/"+head) {
			return false
		}
	default:
		return false
	}
	return true
}

func (g *Gateway) serveChaptersList(w http.ResponseWriter, r *http.Request, sr *host.StoryReader) {
	chapters, err := sr.Chapters()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if chapters == nil {
		chapters = []host.StoryChapter{}
	}
	if cs := sr.Store().Comments(); cs != nil {
		if counts, err := cs.CountsByChapter(); err == nil && len(counts) > 0 {
			for i := range chapters {
				if c, ok := counts[chapters[i].Chapter]; ok {
					c := c
					chapters[i].Comments = &c
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"chapters": chapters, "revision": storyRevision(sr.EngineDir())})
}

// storyRevision 用 chapters/drafts/reviews 下文件的名字+大小+mtime 生成不透明版本串（G10）：
// 任何 chương/bản nháp/thẩm duyệt đổi → chuỗi đổi → UI tự làm mới. Hosted
// backend 没有 engine 目录 → ""（行内版本号留待后续阶段）。
func storyRevision(engineDir string) string {
	if engineDir == "" {
		return ""
	}
	h := sha256.New()
	for _, sub := range []string{"chapters", "drafts", "reviews", "reviews/rounds"} {
		entries, err := os.ReadDir(filepath.Join(engineDir, sub))
		if err != nil {
			continue
		}
		names := make([]string, 0, len(entries))
		infos := map[string]os.FileInfo{}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			names = append(names, e.Name())
			infos[e.Name()] = info
		}
		sort.Strings(names)
		for _, name := range names {
			info := infos[name]
			fmt.Fprintf(h, "%s/%s|%d|%d\n", sub, name, info.Size(), info.ModTime().UnixNano())
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// serveChapter 处理 chapters/:n 与 chapters/:n/{plan,facts,revisions}。
func (g *Gateway) serveChapter(w http.ResponseWriter, r *http.Request, sr *host.StoryReader, tail string) {
	id, sub, hasSub := strings.Cut(tail, "/")
	n, err := strconv.Atoi(id)
	if err != nil || n <= 0 {
		jsonError(w, http.StatusNotFound, "không có chương")
		return
	}
	switch {
	case !hasSub:
		source := r.URL.Query().Get("source")
		res, err := sr.ChapterText(n, source)
		if errors.Is(err, host.ErrChapterNotFound) {
			jsonError(w, http.StatusNotFound, "không có chương")
			return
		}
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, res)
	case sub == "plan":
		plan, err := sr.ChapterPlan(n)
		writeEngineJSON(w, plan, err)
	case sub == "facts":
		facts, err := sr.ChapterFacts(n)
		writeEngineJSON(w, facts, err)
	case sub == "revisions":
		revisions, err := sr.ChapterRevisions(n)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"revisions": revisions})
	default:
		jsonError(w, http.StatusNotFound, "không tìm thấy")
	}
}

// writeEngineJSON 把"可能不存在"的引擎读取统一映射为 200 / 404 / 500。
func writeEngineJSON(w http.ResponseWriter, v any, err error) {
	if errors.Is(err, host.ErrChapterNotFound) {
		jsonError(w, http.StatusNotFound, "không có chương")
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if facts, ok := v.(*domain.ChapterFacts); ok && facts != nil {
		if facts.Characters == nil {
			facts.Characters = []string{}
		}
		if facts.KeyEvents == nil {
			facts.KeyEvents = []string{}
		}
		if facts.TimelineEvents == nil {
			facts.TimelineEvents = []domain.TimelineEvent{}
		}
		if facts.ForeshadowUpdates == nil {
			facts.ForeshadowUpdates = []domain.ForeshadowUpdate{}
		}
		if facts.RelationshipChanges == nil {
			facts.RelationshipChanges = []domain.RelationshipEntry{}
		}
		if facts.StateChanges == nil {
			facts.StateChanges = []domain.StateChange{}
		}
		if facts.CastIntros == nil {
			facts.CastIntros = []domain.CastIntro{}
		}
		if facts.Places == nil {
			facts.Places = []domain.FactEntity{}
		}
		if facts.Objects == nil {
			facts.Objects = []domain.FactEntity{}
		}
	}
	writeJSON(w, http.StatusOK, v)
}

// serveOutline trả dàn ý; ?volume=N (hợp đồng AG §3) chỉ kèm chapters của tập N,
// ?volume=0 chỉ trả header các tập — sidebar mục lục của truyện >200 chương tải theo tập.
func (g *Gateway) serveOutline(w http.ResponseWriter, r *http.Request, sr *host.StoryReader) {
	volume := host.OutlineAllVolumes
	if raw := strings.TrimSpace(r.URL.Query().Get("volume")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			jsonError(w, http.StatusBadRequest, "volume không hợp lệ")
			return
		}
		volume = n
	}
	outline, err := sr.Outline(volume)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, outline)
}

func (g *Gateway) serveCharacters(w http.ResponseWriter, r *http.Request, sr *host.StoryReader) {
	writeCharactersPayload(w, sr)
}

// writeCharactersPayload trả danh sách nhân vật + danh bạ phụ (hình dạng dùng chung
// của GET và PUT /characters).
func writeCharactersPayload(w http.ResponseWriter, sr *host.StoryReader) {
	chars, cast, err := sr.Characters()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if chars == nil {
		chars = []domain.Character{}
	}
	if cast == nil {
		cast = []domain.CastEntry{}
	}
	for i := range chars {
		if chars[i].Traits == nil {
			chars[i].Traits = []string{}
		}
	}
	for i := range cast {
		if cast[i].AppearanceChapters == nil {
			cast[i].AppearanceChapters = []int{}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"characters": chars, "supporting": cast})
}

// characterPatch là một dòng sửa trong PUT /characters (Gói S): chọn nhân vật bằng
// `id` hoặc `name`, chỉ gửi trường muốn đổi. `revealAt: null` xoá bí mật; thiếu khoá
// thì giữ nguyên — nên phải đọc thô để phân biệt "không gửi" với "gửi null".
type characterPatch struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Kind     *string         `json:"kind"`
	RevealAt json.RawMessage `json:"revealAt"`
}

// serveCharactersUpdate xử lý PUT /characters: người dùng sửa “Loại vai” và “Lộ vai ở
// chương” ở Dự án › Nhân vật. Cả lô kiểm xong mới ghi một lần (characters.json +
// characters.md); lỗi nào cũng không ghi gì. 404 khi không tìm thấy nhân vật.
func (g *Gateway) serveCharactersUpdate(w http.ResponseWriter, r *http.Request, st library.Story, sr *host.StoryReader) {
	var body struct {
		Characters []characterPatch `json:"characters"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if len(body.Characters) == 0 {
		jsonError(w, http.StatusBadRequest, "Thiếu danh sách nhân vật cần sửa")
		return
	}
	chars, err := sr.Store().Characters.Load()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Không đọc được nhân vật: "+err.Error())
		return
	}
	type update struct {
		at       int
		kind     *domain.CharacterKind
		revealAt bool
		value    *int
	}
	var updates []update
	seen := map[int]bool{}
	for _, patch := range body.Characters {
		at, ok := findCharacterIndex(chars, patch.ID, patch.Name)
		if !ok {
			jsonError(w, http.StatusNotFound, "Không tìm thấy nhân vật "+firstNonEmpty(patch.Name, patch.ID))
			return
		}
		if seen[at] {
			jsonError(w, http.StatusBadRequest, "Một nhân vật xuất hiện hai lần trong yêu cầu")
			return
		}
		seen[at] = true
		item := update{at: at}
		if patch.Kind != nil {
			kind := domain.CharacterKind(strings.TrimSpace(*patch.Kind))
			if !domain.ValidCharacterKind(kind) {
				jsonError(w, http.StatusBadRequest, "Loại vai phải là lead, antagonist, ally hoặc support")
				return
			}
			item.kind = &kind
		}
		if len(patch.RevealAt) > 0 {
			item.revealAt = true
			if string(patch.RevealAt) != "null" {
				var chapter int
				if err := json.Unmarshal(patch.RevealAt, &chapter); err != nil || chapter < 1 {
					jsonError(w, http.StatusBadRequest, "Lộ vai ở chương phải là số chương từ 1 trở lên, hoặc null")
					return
				}
				item.value = &chapter
			}
		}
		updates = append(updates, item)
	}
	for _, item := range updates {
		if item.kind != nil {
			chars[item.at].Kind = *item.kind
		}
		if item.revealAt {
			chars[item.at].RevealAt = item.value
		}
	}
	if err := sr.Store().Characters.Save(chars); err != nil {
		jsonError(w, http.StatusInternalServerError, "Không lưu được nhân vật: "+err.Error())
		return
	}
	g.storyChanged(st.Project, st.Story, "characters", "snapshot")
	writeCharactersPayload(w, sr)
}

// findCharacterIndex tra nhân vật theo id rồi tới tên (không phân biệt hoa/thường).
func findCharacterIndex(chars []domain.Character, id, name string) (int, bool) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	for i, c := range chars {
		if id != "" && c.ID == id {
			return i, true
		}
	}
	for i, c := range chars {
		if name != "" && strings.EqualFold(strings.TrimSpace(c.Name), name) {
			return i, true
		}
	}
	return 0, false
}

func (g *Gateway) serveForeshadow(w http.ResponseWriter, r *http.Request, sr *host.StoryReader) {
	entries, err := sr.Foreshadow()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entries == nil {
		entries = []domain.ForeshadowEntry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

// serveReviews trả ReviewEntry kèm payload GAP-36 (items/round/pack/blocking);
// file reviews/NN.json của engine không đổi, mọi trường mới tính lúc đọc.
func (g *Gateway) serveReviews(w http.ResponseWriter, r *http.Request, st library.Story, sr *host.StoryReader, tail string) {
	switch tail {
	case "latest":
		view, ok, err := newReviewReader(st.Dir, sr).latest()
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			jsonError(w, http.StatusNotFound, "chưa có đánh giá")
			return
		}
		writeJSON(w, http.StatusOK, view)
	case "":
		jsonError(w, http.StatusNotFound, "không tìm thấy")
	default:
		n, err := strconv.Atoi(tail)
		if err != nil || n <= 0 || strings.Contains(tail, "/") {
			jsonError(w, http.StatusNotFound, "không tìm thấy")
			return
		}
		global, chapter, err := newReviewReader(st.Dir, sr).chapterViews(n)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"chapter": chapter, "global": global})
	}
}

// ── 导出 ──

// libraryExports 读取书库配置并解析故事的导出目录（<story>/<dirs.exports>）。
func libraryExports(st library.Story) (library.AutoExport, string, error) {
	cfg, err := library.LoadConfig(st.Root)
	if err != nil {
		return library.AutoExport{}, "", err
	}
	return cfg.AutoExport, filepath.Join(st.Dir, cfg.Dirs.Exports), nil
}

// serveExportDownload 对应 GET /export?format=&from=&to=：直接在内存导出到
// 临时文件并流回客户端（契约："như cũ, thêm khoảng chương"）。
func (g *Gateway) serveExportDownload(w http.ResponseWriter, r *http.Request, st library.Story, sr *host.StoryReader) {
	// Hosted cũng xuất qua cùng exp pipeline: chương đọc từ store, tệp ghi vào
	// thư mục tạm (không phải BlobStore — tải thẳng, không lưu trữ).
	q := r.URL.Query()
	format := q.Get("format")
	if format != "txt" && format != "epub" && format != "narration" {
		jsonError(w, http.StatusBadRequest, "format phải là txt, epub hoặc narration")
		return
	}
	from, err := parseOptionalInt(q.Get("from"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "from không hợp lệ")
		return
	}
	to, err := parseOptionalInt(q.Get("to"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "to không hợp lệ")
		return
	}
	if from > 0 && to > 0 && from > to {
		jsonError(w, http.StatusBadRequest, "khoảng chương không hợp lệ")
		return
	}
	dir, err := os.MkdirTemp("", "ainovel-export-")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer os.RemoveAll(dir)
	res, err := sr.Export(r.Context(), exp.Options{
		Format:      exp.Format(format),
		OutPath:     dir + string(os.PathSeparator),
		From:        from,
		To:          to,
		ReadNumbers: isTruthyQuery(q.Get("readNumbers")),
	})
	if err != nil {
		jsonError(w, http.StatusConflict, err.Error())
		return
	}
	file, err := os.Open(res.Path)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer file.Close()
	title := sr.BookTitle()
	if title == "" {
		title = st.Story
	}
	w.Header().Set("Content-Disposition", contentDisposition(title+exp.Format(format).FileSuffix()))
	if format == "epub" {
		w.Header().Set("Content-Type", "application/epub+zip")
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	_, _ = io.Copy(w, file)
}

func parseOptionalInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	return strconv.Atoi(s)
}

// isTruthyQuery 把查询串里的开关值归一成 bool（"1"/"true"/"yes" 为真）。
func isTruthyQuery(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// ── 导出目录浏览 ──

// ExportFileAPI 对应契约 ExportFile。
type ExportFileAPI struct {
	Path      string     `json:"path"` // 相对导出目录
	Name      string     `json:"name"`
	Format    string     `json:"format"` // txt | epub | audio | other
	Size      int64      `json:"size"`
	UpdatedAt *time.Time `json:"updatedAt"`
	Volume    *int       `json:"volume"`
}

// ExportRowAPI 对应契约 ExportRow。
type ExportRowAPI struct {
	Volume    int             `json:"volume"`
	Title     string          `json:"title"`
	From      int             `json:"from"`
	To        int             `json:"to"`
	Complete  bool            `json:"complete"`
	TXT       *ExportFileAPI  `json:"txt"`
	EPUB      *ExportFileAPI  `json:"epub"`
	Audio     []ExportFileAPI `json:"audio"`
	Narration *ExportFileAPI  `json:"narration"` // bản đọc TTS của tập; null = chưa xuất (D3)
}

var audioExts = map[string]bool{".mp3": true, ".m4a": true, ".wav": true, ".ogg": true, ".flac": true, ".aac": true, ".m4b": true}

func exportFormatOf(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".txt":
		return "txt"
	case ".epub":
		return "epub"
	}
	if audioExts[strings.ToLower(filepath.Ext(name))] {
		return "audio"
	}
	return "other"
}

var volumeNameRe = regexp.MustCompile(`(?i)(?:tap|vol)[-_. ]*(\d+)`)

// narrationSuffix 是 exp.FormatNarration 落盘时的中缀（exp.Format.FileSuffix）：
// bản đọc TTS 仍是 .txt，所以只能靠名字区分。取名“bản đọc”会与音频工坊的
// 伴随脚本（*.ban-doc.txt，audioCompanionMarks）撞车，故保留 -narration。
const narrationSuffix = "-narration.txt"

// isNarrationExport 判断导出目录里的文件是不是 bản đọc TTS。
func isNarrationExport(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), narrationSuffix)
}

// narrationPath 给显式输出路径加 -narration 中缀。导出 pattern 里的 {ext} 只会
// 给出 .txt，不补中缀时 bản đọc TTS 会与普通 Văn bản 同名（互相覆盖、无法在矩阵里分列）。
func narrationPath(path string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	if strings.HasSuffix(strings.ToLower(base), "-narration") {
		return path
	}
	return base + "-narration" + ext
}

// inferExportVolume 从导出文件相对路径猜卷号：任一路径段含 tap-N / vol-N。
func inferExportVolume(rel string) *int {
	for _, seg := range strings.Split(strings.ReplaceAll(rel, "\\", "/"), "/") {
		if m := volumeNameRe.FindStringSubmatch(seg); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
				return &n
			}
		}
	}
	return nil
}

// listExportFiles 递归列出导出目录内文件（跳过隐藏文件与目录）。
func listExportFiles(exportsDir string, singleVolume bool) ([]ExportFileAPI, error) {
	var files []ExportFileAPI
	err := filepath.WalkDir(exportsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != exportsDir {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(exportsDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		mod := info.ModTime()
		f := ExportFileAPI{
			Path:      rel,
			Name:      filepath.Base(rel),
			Format:    exportFormatOf(rel),
			Size:      info.Size(),
			UpdatedAt: &mod,
			Volume:    inferExportVolume(rel),
		}
		if f.Volume == nil && singleVolume {
			one := 1
			f.Volume = &one
		}
		files = append(files, f)
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return []ExportFileAPI{}, nil
	}
	if files == nil {
		files = []ExportFileAPI{}
	}
	return files, err
}

// serveExportsList 对应 GET /exports：目录列表 + 每卷导出矩阵。
func (g *Gateway) serveExportsList(w http.ResponseWriter, r *http.Request, st library.Story, sr *host.StoryReader) {
	ranges, err := sr.VolumeRanges()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var autoExport library.AutoExport
	var files []ExportFileAPI
	dir := ""
	if st.Dir == "" {
		// Hosted: tạo tác xuất bản sống trong export_cache (blob + manifest),
		// không có thư mục xuất bản trên đĩa.
		if !g.pgMediaOK() {
			jsonError(w, http.StatusInternalServerError, "kho đối tượng chưa cấu hình")
			return
		}
		if lib, lerr := g.lib(r.Context(), nil); lerr == nil {
			autoExport = lib.Config().AutoExport
		}
		if files, err = g.pgHostedExportFiles(r.Context(), st, len(ranges) <= 1); err != nil {
			jsonMappedError(w, err)
			return
		}
	} else {
		var exportsDir string
		autoExport, exportsDir, err = libraryExports(st)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if files, err = listExportFiles(exportsDir, len(ranges) <= 1); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if rel, rerr := filepath.Rel(st.Dir, exportsDir); rerr == nil {
			dir = filepath.ToSlash(rel)
		}
	}
	completed, err := sr.CompletedSet()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rows := make([]ExportRowAPI, 0, len(ranges))
	for _, vr := range ranges {
		row := ExportRowAPI{
			Volume:   vr.Volume,
			Title:    vr.Title,
			From:     vr.From,
			To:       vr.To,
			Complete: vr.From <= vr.To,
			Audio:    []ExportFileAPI{},
		}
		for ch := vr.From; ch <= vr.To; ch++ {
			if !completed[ch] {
				row.Complete = false
				break
			}
		}
		for _, f := range files {
			if f.Volume == nil || *f.Volume != vr.Volume {
				continue
			}
			f := f
			// 音频工坊的可读脚本（*.ban-doc.txt / *.moc-chuong.txt）留在 files 里可下载，
			// 但不能顶替本卷的 txt 导出。
			if isAudioCompanion(f.Name) {
				continue
			}
			if isNarrationExport(f.Name) {
				row.Narration = pickExport(row.Narration, &f)
				continue
			}
			switch f.Format {
			case "txt":
				row.TXT = pickExport(row.TXT, &f)
			case "epub":
				row.EPUB = pickExport(row.EPUB, &f)
			case "audio":
				row.Audio = append(row.Audio, f)
			}
		}
		rows = append(rows, row)
	}
	if rows == nil {
		rows = []ExportRowAPI{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"dir":        dir,
		"files":      files,
		"matrix":     rows,
		"autoExport": autoExport,
	})
}

// audioCompanionMarks 是音频工坊写出的可读脚本的中间段；它们以 .txt 结尾但不是整卷导出。
var audioCompanionMarks = []string{".ban-doc.", ".moc-chuong."}

// isAudioCompanion 判断文件名是否是音频工坊的伴随脚本。
func isAudioCompanion(name string) bool {
	name = strings.ToLower(name)
	for _, mark := range audioCompanionMarks {
		if strings.Contains(name, mark) {
			return true
		}
	}
	return false
}

// pickExport 同一卷同格式多个文件时取最新（按更新时间）。
func pickExport(cur, next *ExportFileAPI) *ExportFileAPI {
	if cur == nil {
		return next
	}
	if cur.UpdatedAt != nil && next.UpdatedAt != nil && next.UpdatedAt.After(*cur.UpdatedAt) {
		return next
	}
	return cur
}

// serveExportCreate 对应 POST /exports：按 pattern 生成导出文件（授权在 serveStories 完成）。
func (g *Gateway) serveExportCreate(w http.ResponseWriter, r *http.Request, st library.Story, sr *host.StoryReader) {
	hosted := st.Dir == ""
	var body struct {
		Format      string `json:"format"`
		From        int    `json:"from"`
		To          int    `json:"to"`
		Volume      int    `json:"volume"`
		OnExists    string `json:"onExists"`
		ReadNumbers bool   `json:"readNumbers"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Format != "txt" && body.Format != "epub" && body.Format != "narration" {
		jsonError(w, http.StatusBadRequest, "format phải là txt, epub hoặc narration")
		return
	}
	autoExport := library.DefaultConfig().AutoExport
	// Hosted không có thư mục xuất bản: chuỗi mẫu {truyen}/{tap}/… resolve
	// trên một gốc ảo, đường dẫn logic giữ nguyên hình dạng thư mục file.
	exportsDir := "hosted-export-root"
	if hosted {
		if !g.pgMediaOK() {
			jsonError(w, http.StatusInternalServerError, "kho đối tượng chưa cấu hình")
			return
		}
		if lib, lerr := g.lib(r.Context(), nil); lerr == nil {
			autoExport = lib.Config().AutoExport
		}
	} else {
		var err error
		autoExport, exportsDir, err = libraryExports(st)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	ranges, err := sr.VolumeRanges()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	volume := body.Volume
	from, to := body.From, body.To
	if volume > 0 {
		var vr *host.VolumeRange
		for i := range ranges {
			if ranges[i].Volume == volume {
				vr = &ranges[i]
				break
			}
		}
		if vr == nil {
			jsonError(w, http.StatusBadRequest, "không có tập "+strconv.Itoa(volume))
			return
		}
		from, to = vr.From, vr.To
	} else {
		volume = 1
	}
	if from <= 0 {
		from = 1
	}
	if to <= 0 {
		// 沿用 exp 的语义（最大完成章），这里只用于 {tu}/{den} 变量，
		// 取最近一次完成章避免渲染出 0。
		progress, err := sr.Progress()
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if progress != nil {
			to = progress.LatestCompleted()
		}
		if to <= 0 {
			to = from
		}
	}
	if from > to {
		jsonError(w, http.StatusBadRequest, "khoảng chương không hợp lệ")
		return
	}

	outPath, err := exp.ResolvePatternPath(exportsDir, autoExport.Pattern, exp.PatternVars{
		Story:  st.Story,
		Volume: volume,
		From:   from,
		To:     to,
		Date:   time.Now(),
		Format: exp.Format(body.Format).FileExt(),
	})
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	onExists := body.OnExists
	if onExists == "" {
		onExists = "fail"
	}
	if onExists != "replace" && onExists != "keep" && onExists != "fail" {
		jsonError(w, http.StatusBadRequest, "onExists phải là replace, keep hoặc fail")
		return
	}
	if body.Format == string(exp.FormatNarration) {
		outPath = narrationPath(outPath)
	}
	opts := exp.Options{
		Format:      exp.Format(body.Format),
		OutPath:     outPath,
		From:        from,
		To:          to,
		OnExists:    onExists,
		ReadNumbers: body.ReadNumbers,
	}
	var res *exp.Result
	var jobID string
	if hosted {
		// Đường dẫn logic của artefact (như file mode: exportsDir-địa phương).
		rel, relErr := filepath.Rel(exportsDir, outPath)
		if relErr != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			jsonError(w, http.StatusBadRequest, "đường dẫn xuất không hợp lệ")
			return
		}
		res, jobID, err = g.pgHostedExport(r.Context(), st, sr, opts, filepath.ToSlash(rel))
	} else {
		res, jobID, err = g.runGatewayExport(r.Context(), st, sr, opts, exportsDir)
	}
	if err != nil {
		writeCommandError(w, err)
		return
	}
	path := res.Path
	if !hosted {
		if rel, relErr := filepath.Rel(exportsDir, res.Path); relErr == nil && !strings.HasPrefix(rel, "..") {
			path = filepath.ToSlash(rel)
		}
	}
	if res.Skipped == nil {
		res.Skipped = []int{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"path":     path,
		"bytes":    res.Bytes,
		"chapters": res.Chapters,
		"skipped":  res.Skipped,
		"job":      jobID,
	})
}

// serveExportFile 对应 GET /exports/file?path=：下载导出目录内文件（拦截 ..）。
func (g *Gateway) serveExportFile(w http.ResponseWriter, r *http.Request, st library.Story, sr *host.StoryReader) {
	if st.Dir == "" {
		g.serveExportFileHosted(w, r, st, sr)
		return
	}
	_, exportsDir, err := libraryExports(st)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	target, err := safeExportPath(exportsDir, r.URL.Query().Get("path"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "đường dẫn không hợp lệ")
		return
	}
	info, err := os.Stat(target)
	if err != nil || info.IsDir() {
		jsonError(w, http.StatusNotFound, "không tìm thấy tệp")
		return
	}
	file, err := os.Open(target)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer file.Close()
	name := downloadFileName(st, sr, target, info.Name())
	w.Header().Set("Content-Disposition", contentDisposition(name))
	// ServeContent tự lo Range/206 + sniff Content-Type: iOS Safari bắt buộc
	// Range khi stream audio từ trình phát trong app (audioExportFileURL).
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

// serveExportFileHosted is the hosted twin of /exports/file: `path` is either
// the produced blob's object key (files[].path / job results) or the logical
// registry path (export-file/<rel>). Same 404 contract as the file handler.
func (g *Gateway) serveExportFileHosted(w http.ResponseWriter, r *http.Request, st library.Story, sr *host.StoryReader) {
	if !g.pgMediaOK() {
		jsonError(w, http.StatusInternalServerError, "kho đối tượng chưa cấu hình")
		return
	}
	blob, m, err := g.pgHostedExportResolve(r.Context(), st, r.URL.Query().Get("path"))
	if err != nil {
		if errors.Is(err, errs.ErrStorageNotFound) || errors.Is(err, postgres.ErrJobNotFound) {
			jsonError(w, http.StatusNotFound, "không tìm thấy tệp")
			return
		}
		jsonMappedError(w, err)
		return
	}
	name := m.Name
	if name == "" {
		name = blob.ObjectKey
	}
	if base := filepath.Base(name); base != "" && base != "." {
		name = base
	}
	if ct := mime.TypeByExtension(filepath.Ext(name)); ct != "" {
		blob.ContentType = ct
	}
	g.serveHostedBlob(w, r, blob, "private, max-age=60", name)
}

// safeExportPath 把相对路径解析到导出目录内：拒绝绝对路径与任何 .. 段，
// 并在 Clean 之后复核目标仍在目录内（防符号链接/分隔符变体）。
func safeExportPath(exportsDir, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" || filepath.IsAbs(rel) {
		return "", errors.New("invalid path")
	}
	for _, seg := range strings.FieldsFunc(rel, func(c rune) bool { return c == '/' || c == '\\' }) {
		if seg == ".." {
			return "", errors.New("invalid path")
		}
	}
	target := filepath.Join(exportsDir, rel)
	baseAbs, err := filepath.Abs(exportsDir)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if targetAbs != baseAbs && !strings.HasPrefix(targetAbs, baseAbs+string(filepath.Separator)) {
		return "", errors.New("invalid path")
	}
	return targetAbs, nil
}
