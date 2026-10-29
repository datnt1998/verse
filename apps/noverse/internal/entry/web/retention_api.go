package web

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/diag"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// frameSourcePath là dấu "ai đặt khung truyện" (không phải dữ liệu truyện):
// save_foundation của Kiến trúc sư không ghi tệp này, nên có tệp = người dùng
// đã lưu từ web. Tách khỏi story_retention.json để không đổi định dạng tệp
// người dùng và không lọt vào ngữ cảnh của mô hình.
const frameSourcePath = "meta/frame_source.json"

// audienceGroup là một nhóm người nghe của Master §1.2, đọc từ gói thể loại.
type audienceGroup struct {
	N     int    `json:"n"`
	Label string `json:"label"`
	Wants string `json:"wants"`
}

type frameResponse struct {
	Audience   []int           `json:"audience"`
	Difference string          `json:"difference"`
	SequelSeed string          `json:"sequelSeed"`
	Source     string          `json:"source"`
	Audiences  []audienceGroup `json:"audiences"`
}

// serveStoryFrame xử lý GET/PUT /frame (D5): khung truyện của một truyện.
// 授权已在 serveStories 的 storyAccessAllowed 完成（GET 需可读、PUT 需可写）。
func (g *Gateway) serveStoryFrame(w http.ResponseWriter, r *http.Request, st library.Story, sr *host.StoryReader) {
	switch r.Method {
	case http.MethodGet, http.MethodPut:
	default:
		jsonError(w, http.StatusMethodNotAllowed, "không hỗ trợ phương thức này")
		return
	}
	pack, err := storyPack(st, sr)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Không đọc được gói thể loại: "+err.Error())
		return
	}
	audiences := packAudiences(pack)
	stored, err := sr.Store().Outline.LoadStoryRetention()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Không đọc được khung truyện: "+err.Error())
		return
	}
	if r.Method == http.MethodPut {
		stored, err = g.updateFrame(w, r, sr, pack, audiences, stored)
		if err != nil {
			return
		}
	}
	out := buildFrameResponse(stored, pack, audiences, sr)
	others, err := library.SiblingStories(sr.EngineDir())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Không đọc được truyện để đối chiếu")
		return
	}
	if len(others) == 0 {
		out.Difference = ""
	}
	writeJSON(w, http.StatusOK, out)
}

// updateFrame áp PUT /frame lên story_retention.json. Trả lỗi đã ghi response.
func (g *Gateway) updateFrame(w http.ResponseWriter, r *http.Request, sr *host.StoryReader, pack *assets.Pack, audiences []audienceGroup, stored domain.StoryRetention) (domain.StoryRetention, error) {
	var body struct {
		Audience   *[]int  `json:"audience"`
		Difference *string `json:"difference"`
		SequelSeed *string `json:"sequelSeed"`
	}
	if !decodeJSON(w, r, &body) {
		return stored, errors.New("body")
	}
	if body.Audience != nil {
		if len(audiences) == 0 {
			jsonError(w, http.StatusBadRequest, "Gói thể loại chưa khai nhóm người nghe")
			return stored, errors.New("audience")
		}
		if !validAudienceIndices(audiences, *body.Audience) {
			jsonError(w, http.StatusBadRequest, fmt.Sprintf("Nhóm người nghe phải từ 0 đến %d", len(audiences)-1))
			return stored, errors.New("audience")
		}
		stored.NguoiNghe = append([]int{}, (*body.Audience)...)
	}
	if body.Difference != nil {
		value, err := frameLine(*body.Difference)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "Điểm khác "+err.Error())
			return stored, err
		}
		stored.DiemKhac = value
	}
	if body.SequelSeed != nil {
		value, err := frameLine(*body.SequelSeed)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "Hạt giống phần hai "+err.Error())
			return stored, err
		}
		stored.HatGiongPhanHai = value
	}
	if err := sr.Store().Outline.SaveStoryRetention(stored); err != nil {
		jsonError(w, http.StatusInternalServerError, "Không lưu được khung truyện: "+err.Error())
		return stored, err
	}
	if err := store.NewIO(sr.EngineDir()).WriteJSON(frameSourcePath, map[string]string{"source": "user"}); err != nil {
		jsonError(w, http.StatusInternalServerError, "Không lưu được khung truyện: "+err.Error())
		return stored, err
	}
	return stored, nil
}

// frameLine chuẩn hoá ô chữ một dòng của khung truyện: bỏ khoảng trắng hai đầu,
// từ chối xuống dòng và quá 200 ký tự.
func frameLine(value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "\r\n") {
		return "", errors.New("phải là một dòng")
	}
	if utf8.RuneCountInString(value) > 200 {
		return "", errors.New("tối đa 200 ký tự")
	}
	return value, nil
}

// buildFrameResponse ghép khung truyện trả về: giá trị đã lưu, nhóm người nghe
// (lấy từ gói khi truyện chưa ghi nhóm nào — đúng nhóm Kiến trúc sư đang dùng),
// và nguồn của giá trị hiện tại.
func buildFrameResponse(stored domain.StoryRetention, pack *assets.Pack, audiences []audienceGroup, sr *host.StoryReader) frameResponse {
	out := frameResponse{
		Difference: domain.StoryDifference(stored.DiemKhac),
		SequelSeed: stored.HatGiongPhanHai,
		Source:     "pack",
		Audiences:  audiences,
		Audience:   []int{},
	}
	if stored.NguoiNghe != nil || stored.DiemKhac != "" || stored.HatGiongPhanHai != "" {
		out.Source = frameSource(sr)
	}
	if stored.NguoiNghe != nil {
		out.Audience = stored.NguoiNghe
	} else if pack != nil {
		out.Audience = audienceIndices(audiences, pack.NguoiNghe)
	}
	// story_retention.json có thể ghi nguoi_nghe: null → vẫn gửi [] trên dây.
	if out.Audience == nil {
		out.Audience = []int{}
	}
	return out
}

// frameSource đọc dấu người dùng đã lưu khung truyện từ web; thiếu tệp nghĩa là
// giá trị do Kiến trúc sư ghi trong save_foundation.
func frameSource(sr *host.StoryReader) string {
	var marker struct {
		Source string `json:"source"`
	}
	if err := store.NewIO(sr.EngineDir()).ReadJSON(frameSourcePath, &marker); err == nil && marker.Source == "user" {
		return "user"
	}
	return "architect"
}

// packAudiences dùng bảng của gói; thiếu gói/bảng thì vẫn cho chọn năm nhóm
// Master §1.2 theo đúng enum của công cụ nền tảng, không tự đoán nhóm đã chọn.
func packAudiences(pack *assets.Pack) []audienceGroup {
	if pack != nil {
		var parsed []audienceGroup
		if ok, err := pack.Section("audience_groups", &parsed); err == nil && ok && len(parsed) > 0 {
			return parsed
		}
	}
	groups := make([]audienceGroup, len(domain.AudienceLabels))
	for n, label := range domain.AudienceLabels {
		groups[n] = audienceGroup{N: n, Label: label}
	}
	return groups
}

// audienceIndices đọc một chuỗi nhãn ("Nam trung niên · Nữ 18–35") thành chỉ số
// nhóm theo thứ tự bảng Master §1.2. Nhãn lạ bị bỏ; không nhãn nào khớp → rỗng.
func audienceIndices(groups []audienceGroup, labels string) []int {
	out := []int{}
	seen := map[int]bool{}
	for _, label := range strings.Split(labels, " · ") {
		idx := audienceIndex(groups, label)
		if idx < 0 || seen[idx] {
			continue
		}
		seen[idx] = true
		out = append(out, idx)
	}
	return out
}

// validAudienceIndices kiểm chỉ số do khách gửi: ngoài bảng hoặc lặp lại là 400.
func validAudienceIndices(groups []audienceGroup, indices []int) bool {
	for i, idx := range indices {
		if idx < 0 || idx >= len(groups) {
			return false
		}
		for _, previous := range indices[:i] {
			if previous == idx {
				return false
			}
		}
	}
	return true
}

// audienceIndex tìm nhóm theo nhãn (không phân biệt hoa thường, bỏ khoảng trắng
// hai đầu). Trả -1 khi giá trị đã lưu không phải một trong năm nhóm.
func audienceIndex(groups []audienceGroup, label string) int {
	label = strings.ToLower(strings.TrimSpace(label))
	if label == "" {
		return -1
	}
	for i, group := range groups {
		if strings.ToLower(strings.TrimSpace(group.Label)) == label {
			return i
		}
	}
	return -1
}

// storyPack nạp gói thể loại đang chọn của truyện (khoá style trong cấu hình).
// Hosted không có config file: style engine ghi trong run meta chọn gói
// embedded — cùng hợp đồng với reviewReader, không đọc đường dẫn host.
func storyPack(st library.Story, sr *host.StoryReader) (*assets.Pack, error) {
	if st.Dir == "" {
		style := ""
		if s := sr.Store(); s != nil {
			if meta, err := s.RunMeta.Load(); err == nil && meta != nil {
				style = meta.Style
			}
		}
		bundle := assets.Load(style, assets.LoadOptions{})
		if bundle.Err != nil {
			return nil, bundle.Err
		}
		return bundle.Pack, nil
	}
	cfg, err := bootstrap.LoadConfigAt(st.Dir)
	if err != nil {
		return nil, err
	}
	bundle := assets.Load(cfg.Style, assets.DefaultLoadOptions(sr.EngineDir()))
	if bundle.Err != nil {
		return nil, bundle.Err
	}
	return bundle.Pack, nil
}

// ── Bản đồ giữ chân (D5) ──

type retentionPayoff struct {
	Tag  string `json:"tag"`
	Text string `json:"text"`
}

type retentionHook struct {
	Type    string `json:"type"`
	Waiting string `json:"waiting"`
}

type retentionChapter struct {
	N         int              `json:"n"`
	Title     string           `json:"title"`
	Status    string           `json:"status"`
	Questions []string         `json:"questions"`
	Promise   string           `json:"promise"`
	Payoff    *retentionPayoff `json:"payoff"`
	Hook      *retentionHook   `json:"hook"`
}

type retentionCheck struct {
	Ref      string `json:"ref"`
	OK       bool   `json:"ok"`
	Text     string `json:"text"`
	Chapters []int  `json:"chapters"`
}

type retentionResponse struct {
	// Empty: dàn ý chưa khai trường giữ chân nào → không chạy 4 phép kiểm (Câu 19).
	Empty     bool               `json:"empty"`
	Chapters  []retentionChapter `json:"chapters"`
	PromiseAt *float64           `json:"promiseAt"`
	TwistAt   *float64           `json:"twistAt"`
	Checks    []retentionCheck   `json:"checks"`
}

// hookTypeEnum ánh xạ kiểu móc đã lưu (ASCII của save_foundation hoặc nhãn
// tiếng Việt của Master §7) sang enum của hợp đồng. Giá trị lạ → không có móc.
// Năm kiểu cũ (crisis/mystery/choice/desire/emotion) đổi theo bảng Câu 15; dữ
// liệu mới chỉ ghi bốn kiểu Master §7 cộng `seed` ở chương cuối (Câu 19).
var hookTypeEnum = map[string]string{
	"info":               "info",
	"threat":             "threat",
	"decision":           "decision",
	"line":               "line",
	"lat_thong_tin":      "info",
	"de_doa_moi":         "threat",
	"quyet_dinh_treo":    "decision",
	"cau_thoai_no":       "line",
	"seed":               "seed",
	"hat_giong_phan_hai": "seed",
	"crisis":             "threat",
	"mystery":            "info",
	"choice":             "decision",
	"desire":             "decision",
	"emotion":            "line",
	"lật thông tin":      "info",
	"đe dọa mới":         "threat",
	"quyết định treo":    "decision",
	"câu thoại nổ":       "line",
	"hạt giống phần hai": "seed",
}

// hookTypeLabel là nhãn tiếng Việt của Master §7 dùng trong câu kiểm §7.
var hookTypeLabel = map[string]string{
	"info":     "Lật thông tin",
	"threat":   "Đe dọa mới",
	"decision": "Quyết định treo",
	"line":     "Câu thoại nổ",
	"seed":     "Hạt giống phần hai",
}

// serveStoryRetention xử lý GET /retention (D5): bản đồ giữ chân theo chương và
// bốn dòng kiểm §2/§3/§7/§8 tính ở server. 授权已在 serveStories 完成（GET 需可读）。
func (g *Gateway) serveStoryRetention(w http.ResponseWriter, r *http.Request, st library.Story, sr *host.StoryReader) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "không hỗ trợ phương thức này")
		return
	}
	outline, err := sr.Store().Outline.LoadOutline()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Không đọc được đại cương: "+err.Error())
		return
	}
	progress, err := sr.Progress()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Không đọc được tiến độ: "+err.Error())
		return
	}
	volumes, err := sr.Store().Outline.LoadLayeredOutline()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Không đọc được đại cương: "+err.Error())
		return
	}
	// Chuẩn hóa cả chuỗi móc trước khi kiểm, để kiểu cũ và kiểu Master tương
	// đương không bị tính thành hai kiểu khác nhau (Câu 15).
	for i := range outline {
		if kind, ok := hookTypeEnum[strings.ToLower(strings.TrimSpace(outline[i].HookType))]; ok {
			outline[i].HookType = kind
		}
	}
	snap := &diag.Snapshot{Outline: outline, Volumes: volumes}
	rmap := diag.BuildRetentionMap(snap)

	out := retentionResponse{Chapters: make([]retentionChapter, 0, len(outline)), Checks: []retentionCheck{}}
	last := len(outline)
	for i, entry := range outline {
		chapter := retentionChapter{
			N:         entry.Chapter,
			Title:     entry.Title,
			Status:    retentionStatus(entry.Chapter, progress),
			Questions: append([]string{}, entry.OpenQuestions...),
			Promise:   entry.Promise,
		}
		if chapter.N == 0 {
			chapter.N = i + 1
		}
		chapter.Payoff = retentionPayoffOf(entry, i+1 == last)
		chapter.Hook = retentionHookOf(entry, i+1 == last)
		out.Chapters = append(out.Chapters, chapter)
	}
	if rmap.Total > 0 {
		if rmap.PromiseChapter > 0 {
			out.PromiseAt = position(rmap.PromiseChapter, rmap.Total)
		}
		if rmap.TwistChapter > 0 {
			out.TwistAt = position(rmap.TwistChapter, rmap.Total)
		}
	}
	// Dàn ý cũ chưa khai trường giữ chân nào: không chạy phép kiểm, giao diện
	// hiện trạng thái rỗng "Chưa lập bản đồ giữ chân" (Câu 19).
	if !rmap.PromiseDeclared && !rmap.TwistDeclared && !rmap.HooksDeclared && !rmap.QuestionsDeclared {
		out.Empty = true
		writeJSON(w, http.StatusOK, out)
		return
	}
	out.Checks = retentionChecks(rmap, outline)
	writeJSON(w, http.StatusOK, out)
}

// retentionStatus quy trạng thái chương về ba giá trị của hợp đồng.
func retentionStatus(chapter int, progress *domain.Progress) string {
	if progress != nil && progress.InProgressChapter == chapter {
		return "writing"
	}
	if progress != nil {
		for _, n := range progress.CompletedChapters {
			if n == chapter {
				return "written"
			}
		}
	}
	return "unwritten"
}

// position là vị trí 0..1 của chương trong tổng số chương (mốc cuối chương).
func position(chapter, total int) *float64 {
	if total <= 0 {
		return nil
	}
	v := float64(chapter) / float64(total)
	return &v
}

// retentionPayoffOf: cú lật giữa truyện thắng cảnh lời hứa, chương cuối là cao
// trào; mô tả lấy từ sự kiện chính của chương (cột "Trả thưởng · cú lật").
func retentionPayoffOf(entry domain.OutlineEntry, last bool) *retentionPayoff {
	tag := ""
	switch {
	case entry.MidpointTwist:
		tag = "midpoint"
	case domain.PromiseFulfilled(entry.Promise):
		tag = "promise"
	case last:
		tag = "climax"
	}
	if tag == "" && strings.TrimSpace(entry.CoreEvent) == "" {
		return nil
	}
	return &retentionPayoff{Tag: tag, Text: entry.CoreEvent}
}

// retentionHookOf ánh xạ kiểu móc sang enum; kiểu lạ (kể cả ket_truyen của
// chương cuối) trả nil thay vì đoán. `seed` chỉ hợp lệ ở chương cuối: chương
// khác bị bỏ và báo lý do ở dòng kiểm §7 (Câu 19).
func retentionHookOf(entry domain.OutlineEntry, last bool) *retentionHook {
	kind, ok := hookTypeEnum[strings.ToLower(strings.TrimSpace(entry.HookType))]
	if !ok || (kind == "seed" && !last) {
		return nil
	}
	return &retentionHook{Type: kind, Waiting: entry.Hook}
}

// misplacedSeeds là các chương không phải chương cuối nhưng ghi kiểu móc
// `seed` — dữ liệu sai chỗ, không phải móc hợp lệ.
func misplacedSeeds(outline []domain.OutlineEntry) []int {
	out := []int{}
	for i, entry := range outline {
		if i == len(outline)-1 {
			continue
		}
		kind, ok := hookTypeEnum[strings.ToLower(strings.TrimSpace(entry.HookType))]
		if !ok || kind != "seed" {
			continue
		}
		n := entry.Chapter
		if n == 0 {
			n = i + 1
		}
		out = append(out, n)
	}
	return out
}

// retentionChecks dựng bốn dòng kiểm theo thứ tự bảng D5: §3, §8, §7, §2.
// `chapters` đạt thì rỗng — chuẩn hoá về [] để JSON không gửi null.
func retentionChecks(rmap diag.RetentionMap, outline []domain.OutlineEntry) []retentionCheck {
	if len(outline) == 0 {
		return []retentionCheck{}
	}
	checks := []retentionCheck{
		promiseCheck(rmap, outline),
		twistCheck(rmap, outline),
		hookChainCheck(rmap, outline),
		questionsCheck(rmap),
	}
	for i := range checks {
		if checks[i].Chapters == nil {
			checks[i].Chapters = []int{}
		}
	}
	return checks
}

func promiseCheck(rmap diag.RetentionMap, outline []domain.OutlineEntry) retentionCheck {
	check := retentionCheck{Ref: "Master §3"}
	if rmap.PromiseChapter == 0 {
		check.Text = "Chưa đánh dấu cảnh lời hứa · cần nằm trong vùng 35–60%."
		return check
	}
	percent := percentOf(rmap.PromiseChapter, rmap.Total)
	check.Chapters = []int{rmap.PromiseChapter}
	// So vùng bằng tỉ lệ chưa làm tròn (5/8 = 62,5%): làm tròn chỉ để hiển thị (Câu 19).
	if at := fractionOf(rmap.PromiseChapter, rmap.Total); at >= 0.35 && at <= 0.60 {
		check.OK = true
		check.Chapters = nil
		check.Text = fmt.Sprintf("Cảnh lời hứa (%s) nằm ở %d%%, trong vùng 35–60%%.",
			promiseScene(outline[rmap.PromiseChapter-1].Promise), percent)
		return check
	}
	check.Text = fmt.Sprintf("Cảnh lời hứa nằm ở %d%%, ngoài vùng 35–60%%.", percent)
	return check
}

func twistCheck(rmap diag.RetentionMap, outline []domain.OutlineEntry) retentionCheck {
	check := retentionCheck{Ref: "Master §8"}
	if rmap.TwistChapter == 0 {
		check.Text = "Chưa đánh dấu cú lật giữa truyện · cần nằm trong vùng 45–60%."
		return check
	}
	percent := percentOf(rmap.TwistChapter, rmap.Total)
	check.Chapters = []int{rmap.TwistChapter}
	if at := fractionOf(rmap.TwistChapter, rmap.Total); at >= 0.45 && at <= 0.60 {
		check.OK = true
		check.Chapters = nil
		check.Text = fmt.Sprintf("Cú lật giữa truyện (%s) nằm ở %d%%, trong vùng 45–60%%.",
			strings.TrimSpace(outline[rmap.TwistChapter-1].CoreEvent), percent)
		return check
	}
	check.Text = fmt.Sprintf("Cú lật giữa truyện nằm ở %d%%, ngoài vùng 45–60%%.", percent)
	return check
}

func hookChainCheck(rmap diag.RetentionMap, outline []domain.OutlineEntry) retentionCheck {
	check := retentionCheck{Ref: "Master §7"}
	parts := make([]string, 0, len(rmap.HookChains)+1)
	for _, chain := range rmap.HookChains {
		check.Chapters = append(check.Chapters, chain.Chapters...)
		label, ok := hookTypeLabel[hookTypeEnum[chain.Type]]
		if !ok {
			label = chain.Type
		}
		parts = append(parts, fmt.Sprintf("Chương %d, %d và %d cùng kết bằng “%s”. Đổi chương %d sang kiểu móc khác.",
			chain.Chapters[0], chain.Chapters[1], chain.Chapters[2], label, chain.Chapters[1]))
	}
	if seeds := misplacedSeeds(outline); len(seeds) > 0 {
		check.Chapters = append(check.Chapters, seeds...)
		parts = append(parts, fmt.Sprintf("%s dùng “Hạt giống phần hai” khi chưa phải chương cuối; kiểu này chỉ hợp lệ ở chương cuối.",
			chapterList(seeds)))
	}
	if len(parts) == 0 {
		check.OK = true
		check.Text = "Không chương nào lặp kiểu móc quá hai chương liền nhau."
		return check
	}
	check.Text = strings.Join(parts, " ")
	return check
}

func questionsCheck(rmap diag.RetentionMap) retentionCheck {
	check := retentionCheck{Ref: "Master §2"}
	few, many := rmap.FewQuestions, rmap.ManyQuestions
	if len(few) == 0 && len(many) == 0 {
		check.OK = true
		check.Text = "Chương nào cũng có 2–4 câu hỏi đang treo."
		return check
	}
	check.Chapters = append(append([]int{}, few...), many...)
	var parts []string
	switch {
	case len(few) == 1 && len(many) == 0:
		parts = append(parts, fmt.Sprintf("Chương %d chỉ có 1 câu hỏi đang treo, nên có 2–4.", few[0]))
	case len(few) > 0 && len(many) == 0:
		parts = append(parts, fmt.Sprintf("%s có dưới 2 câu hỏi đang treo, nên có 2–4.", chapterList(few)))
	case len(many) == 1 && len(few) == 0:
		parts = append(parts, fmt.Sprintf("Chương %d có hơn 4 câu hỏi đang treo, nên có 2–4.", many[0]))
	case len(many) > 0 && len(few) == 0:
		parts = append(parts, fmt.Sprintf("%s có hơn 4 câu hỏi đang treo, nên có 2–4.", chapterList(many)))
	default:
		parts = append(parts, fmt.Sprintf("%s chưa đúng 2–4 câu hỏi đang treo.", chapterList(check.Chapters)))
	}
	check.Text = strings.Join(parts, " ")
	return check
}

// chapterList đọc "Chương 1", "Chương 1 và 4", "Chương 1, 4 và 6".
func chapterList(chapters []int) string {
	parts := make([]string, 0, len(chapters))
	for _, n := range chapters {
		parts = append(parts, fmt.Sprintf("%d", n))
	}
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return "Chương " + parts[0]
	}
	return "Chương " + strings.Join(parts[:len(parts)-1], ", ") + " và " + parts[len(parts)-1]
}

// promiseScene bỏ tiền tố da_tra: để lấy mô tả cảnh lời hứa.
func promiseScene(promise string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(promise), domain.PromiseFulfilledPrefix))
}

// percentOf là phần trăm (làm tròn) của chương trong tổng số chương — chỉ dùng
// để hiển thị; phép so vùng dùng fractionOf.
func percentOf(chapter, total int) int {
	if total <= 0 {
		return 0
	}
	return int(math.Round(float64(chapter) * 100 / float64(total)))
}

// fractionOf là vị trí chưa làm tròn của chương trong tổng số chương (5/8 = 0,625).
func fractionOf(chapter, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(chapter) / float64(total)
}
