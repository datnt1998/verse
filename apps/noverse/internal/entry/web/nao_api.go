package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// Não truyện (GAP-38 xưng hô & quan hệ, GAP-39 brain). Các endpoint đọc/ghi thẳng tệp
// của truyện trong gateway, cùng chỗ với đọc truyện: dữ liệu gốc là JSON của engine
// (docs/adr/0001), không có kho riêng và không cần worker sống để đọc.
// Quyền xem/ghi đã được serveStories kiểm trước khi gọi vào đây.

const (
	naoFormMaxRunes   = 200
	naoReasonMaxRunes = 500
	// naoSnapshotTimeout chặn trường hợp worker treo: ghi dữ liệu không được chờ mãi.
	naoSnapshotTimeout = 2 * time.Second
)

// naoWriting là trạng thái khoá sửa (D8): engine đang viết thì không nhận ghi.
type naoWriting struct {
	Chapter int    `json:"chapter"`
	Role    string `json:"role"`
	// State là trạng thái chương đang khoá (Câu 23): "writing" khi viết lần đầu,
	// "rewriting" khi viết lại/gọt giũa — cùng bộ từ với trạng thái thanh chương.
	State string `json:"state"`
}

type naoAddressTerm struct {
	ID           string `json:"id"`
	From         string `json:"from"`
	FromName     string `json:"fromName"`
	To           string `json:"to"`
	ToName       string `json:"toName"`
	Call         string `json:"call"`
	Self         string `json:"self"`
	FromChapter  int    `json:"fromChapter"`
	UntilChapter int    `json:"untilChapter"`
	Reason       string `json:"reason"`
}

type naoRelation struct {
	Chapter int    `json:"chapter"`
	A       string `json:"a"`
	AName   string `json:"aName"`
	B       string `json:"b"`
	BName   string `json:"bName"`
	Before  string `json:"before"`
	After   string `json:"after"`
	Reason  string `json:"reason"`
	// Type là nhóm quan hệ để tô chip (Câu 20): enemy/ally/work/own/secret/rel.
	// Dữ kiện cũ chưa phân loại đọc là "rel".
	Type string `json:"type"`
}

type naoAddressTermsResponse struct {
	Terms         []naoAddressTerm `json:"terms"`
	Relations     []naoRelation    `json:"relations"`
	LatestWritten int              `json:"latestWritten"`
	Writing       *naoWriting      `json:"writing"`
}

type naoTermRequest struct {
	Term struct {
		ID          string `json:"id"`
		From        string `json:"from"`
		To          string `json:"to"`
		Call        string `json:"call"`
		Self        string `json:"self"`
		FromChapter int    `json:"fromChapter"`
		Reason      string `json:"reason"`
	} `json:"term"`
	Apply string `json:"apply"`
}

type naoRelationRequest struct {
	A           string `json:"a"`
	B           string `json:"b"`
	After       string `json:"after"`
	Reason      string `json:"reason"`
	Type        string `json:"type"`
	FromChapter int    `json:"fromChapter"`
	Apply       string `json:"apply"`
}

type naoBrainChapter struct {
	N      int    `json:"n"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// naoBrainMoment là một mốc "Diễn biến" của thực thể, kèm chương xảy ra.
type naoBrainMoment struct {
	Chapter int    `json:"chapter"`
	Text    string `json:"text"`
}

type naoBrainEntity struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
	Role string `json:"role"`
	// Ini là chữ viết tắt avatar người dùng tự đặt (04 › Avatar nhân vật); Kind/RevealAt
	// chỉ có nghĩa với type "char" (06-data-contract › Bổ sung §S).
	Ini         string `json:"ini,omitempty"`
	Kind        string `json:"kind,omitempty"`
	RevealAt    *int   `json:"revealAt,omitempty"`
	Desc        string `json:"desc"`
	FromChapter int    `json:"fromChapter"`
	Appears     []int  `json:"appears"`
	// Aliases là các tên khác đã dùng cho thực thể trong truyện (Não identity),
	// để trình đọc gạch chân đúng tên khi họ xuất hiện trong đoạn văn.
	Aliases []string `json:"aliases"`
	// Timeline là diễn biến theo chương, dựng từ relationship_log + address_terms.
	Timeline []naoBrainMoment `json:"timeline"`
}

type naoBrainPhase struct {
	FromChapter int    `json:"fromChapter"`
	Type        string `json:"type"`
	Label       string `json:"label"`
}

type naoBrainEdge struct {
	A      string          `json:"a"`
	B      string          `json:"b"`
	Phases []naoBrainPhase `json:"phases"`
}

type naoBrainResponse struct {
	Chapters []naoBrainChapter `json:"chapters"`
	Entities []naoBrainEntity  `json:"entities"`
	Edges    []naoBrainEdge    `json:"edges"`
}

// serveStoryNao 处理 story 前缀后的 /address-terms, /relations, /brain。
// 授权已在 serveStories 的 storyAccessAllowed 完成（GET 需可读、其余需可写）。
// 返回 false 表示不是本钩子的路径。
func (g *Gateway) serveStoryNao(w http.ResponseWriter, r *http.Request, st library.Story, sr *host.StoryReader, rest string) bool {
	switch rest {
	case "/address-terms":
		switch r.Method {
		case http.MethodGet:
			resp, err := g.naoAddressTerms(st, sr)
			if err != nil {
				jsonError(w, http.StatusInternalServerError, err.Error())
				return true
			}
			writeJSON(w, http.StatusOK, resp)
		case http.MethodPut:
			g.naoPutAddressTerm(w, r, st, sr)
		default:
			return false
		}
		return true
	case "/relations":
		if r.Method != http.MethodPost {
			return false
		}
		g.naoPostRelation(w, r, st, sr)
		return true
	case "/brain":
		if r.Method != http.MethodGet {
			return false
		}
		upTo, ok := naoUpTo(r)
		if !ok {
			jsonError(w, http.StatusBadRequest, naoUpToError)
			return true
		}
		resp, err := g.naoBrain(sr, upTo)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return true
		}
		writeJSON(w, http.StatusOK, resp)
		return true
	}
	return false
}

// naoUpToError là câu trả 400 khi ?upTo= không phải số chương dương.
const naoUpToError = "upTo không hợp lệ"

// naoUpTo đọc ?upTo=N: chương người đọc đang đọc (M8). Không có tham số nghĩa là
// không cắt (màn Não truyện của người viết/quản trị thấy toàn bộ).
func naoUpTo(r *http.Request) (int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("upTo"))
	if raw == "" {
		return 0, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

// ── Đọc ──

func (g *Gateway) naoAddressTerms(st library.Story, sr *host.StoryReader) (naoAddressTermsResponse, error) {
	s := sr.Store()
	chars, err := s.Characters.Load()
	if err != nil {
		return naoAddressTermsResponse{}, err
	}
	terms, err := s.World.LoadAddressTerms()
	if err != nil {
		return naoAddressTermsResponse{}, err
	}
	log, err := s.World.LoadRelationshipLog()
	if err != nil {
		return naoAddressTermsResponse{}, err
	}
	progress, err := s.Progress.Load()
	if err != nil {
		return naoAddressTermsResponse{}, err
	}
	resp := naoAddressTermsResponse{
		Terms:     naoTermRows(chars, terms),
		Relations: naoRelationRows(chars, log),
		Writing:   g.naoWriting(st),
	}
	if progress != nil {
		resp.LatestWritten = progress.LatestCompleted()
	}
	return resp, nil
}

func naoTermID(term domain.AddressTerm) string {
	return term.From + "|" + term.To + "|" + strconv.Itoa(term.FromChapter)
}

func naoName(chars []domain.Character, ref string) string {
	if c, ok := store.FindCharacter(chars, ref); ok {
		return c.Name
	}
	return ref
}

func naoTermRows(chars []domain.Character, terms []domain.AddressTerm) []naoAddressTerm {
	rows := make([]naoAddressTerm, 0, len(terms))
	for _, term := range terms {
		rows = append(rows, naoAddressTerm{
			ID:           naoTermID(term),
			From:         term.From,
			FromName:     naoName(chars, term.From),
			To:           term.To,
			ToName:       naoName(chars, term.To),
			Call:         term.Calls,
			Self:         term.Self,
			FromChapter:  term.FromChapter,
			UntilChapter: term.UntilChapter,
			Reason:       term.Reason,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].FromChapter != rows[j].FromChapter {
			return rows[i].FromChapter < rows[j].FromChapter
		}
		return rows[i].ID < rows[j].ID
	})
	return rows
}

// naoRelationRows dựng lịch sử quan hệ theo chương: mỗi dữ kiện là một dòng
// "trước → sau". Dữ kiện cũ chưa có reason (chỉ dữ kiện người dùng sửa mới có).
func naoRelationRows(chars []domain.Character, log []domain.RelationshipChange) []naoRelation {
	rows := make([]naoRelation, 0, len(log))
	previous := map[string]string{}
	for _, change := range log {
		key := naoPairKey(change.From, change.To)
		before := previous[key]
		previous[key] = change.Relation
		// Dữ kiện trùng (ghi lại sau khi chạy lại bản nộp) không phải một lần đổi.
		if before == change.Relation {
			continue
		}
		rows = append(rows, naoRelation{
			Chapter: change.Chapter,
			A:       change.From,
			AName:   naoName(chars, change.From),
			B:       change.To,
			BName:   naoName(chars, change.To),
			Before:  before,
			After:   change.Relation,
			Reason:  change.Reason,
			Type:    domain.NormalizeRelationshipType(change.Type),
		})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Chapter < rows[j].Chapter })
	return rows
}

func naoPairKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "\x00" + b
}

// naoEntitySlug chuẩn hoá tên địa điểm/vật thành phần mã ổn định qua các chương:
// bỏ dấu tiếng Việt, chữ thường, các từ nối bằng "-". Tên không còn ký tự ASCII
// (tên Hán…) thì lấy băm ngắn, để hai nơi khác tên không chung một mã.
func naoEntitySlug(name string) string {
	var b strings.Builder
	separator := false
	for _, r := range norm.NFD.String(strings.ToLower(name)) {
		switch {
		case unicode.Is(unicode.Mn, r):
			continue
		case r == 'đ':
			r = 'd'
		}
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if separator && b.Len() > 0 {
				b.WriteByte('-')
			}
			separator = false
			b.WriteRune(r)
			continue
		}
		separator = true
	}
	if b.Len() == 0 {
		sum := sha256.Sum256([]byte(strings.TrimSpace(name)))
		return hex.EncodeToString(sum[:4])
	}
	return b.String()
}

// ── Khoá khi engine đang viết ──

// naoWriting trả trạng thái engine đang viết của truyện. Không tự khởi động worker:
// không có worker sống nghĩa là không có ai đang viết.
func (g *Gateway) naoWriting(st library.Story) *naoWriting {
	b := g.slot(st.ID)
	b.mu.Lock()
	worker := b.worker
	if worker != nil && workerDead(worker) {
		b.worker, b.running = nil, false
		worker = nil
	}
	b.mu.Unlock()
	if worker == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), naoSnapshotTimeout)
	defer cancel()
	status, body, err := g.workerCall(ctx, worker, http.MethodGet, "snapshot", nil)
	if err != nil || status != http.StatusOK {
		return nil
	}
	var snap struct {
		IsRunning         bool   `json:"isRunning"`
		CurrentChapter    int    `json:"currentChapter"`
		InProgressChapter int    `json:"inProgressChapter"`
		Flow              string `json:"flow"`
		Agents            []struct {
			Name  string `json:"name"`
			State string `json:"state"`
		} `json:"agents"`
	}
	if json.Unmarshal(body, &snap) != nil || !snap.IsRunning {
		return nil
	}
	writing := &naoWriting{Chapter: snap.InProgressChapter, Role: "writer", State: naoFlowState(snap.Flow)}
	if writing.Chapter <= 0 {
		writing.Chapter = snap.CurrentChapter
	}
	for _, agent := range snap.Agents {
		if agent.State != "working" {
			continue
		}
		if role := naoAgentRole(agent.Name); role != "" {
			writing.Role = role
			break
		}
	}
	return writing
}

// naoAgentRole thu gọn tên agent của engine về ba vai của màn Não truyện.
func naoAgentRole(name string) string {
	name = strings.ToLower(name)
	switch {
	case strings.HasPrefix(name, "architect"):
		return "architect"
	case strings.Contains(name, "editor"):
		return "editor"
	case strings.Contains(name, "writer"):
		return "writer"
	}
	return ""
}

var naoRoleLabels = map[string]string{"writer": "Người viết", "editor": "Biên tập viên", "architect": "Kiến trúc sư"}

// naoFlowState quy flow của engine về trạng thái chương đang khoá: viết lại và gọt
// giũa đều là "rewriting" (chương đã có bản cuối), còn lại là "writing" lần đầu.
func naoFlowState(flow string) string {
	if flow == string(domain.FlowRewriting) || flow == string(domain.FlowPolishing) {
		return "rewriting"
	}
	return "writing"
}

// naoLockedMessage dùng đúng câu của artboard D8/Brain: "<ai> đang viết lại chương N,
// nên xưng hô và quan hệ đang khoá. Hãy tạm dừng để sửa."
func naoLockedMessage(w *naoWriting) string {
	label := naoRoleLabels[w.Role]
	if label == "" {
		label = "Trợ lý"
	}
	if w.Role == "architect" {
		return label + " đang sửa đại cương, nên xưng hô và quan hệ đang khoá. Hãy tạm dừng để sửa."
	}
	verb := "viết"
	if w.State == "rewriting" {
		verb = "viết lại"
	}
	return fmt.Sprintf("%s đang %s chương %d, nên xưng hô và quan hệ đang khoá. Hãy tạm dừng để sửa.", label, verb, w.Chapter)
}

func naoLocked(w http.ResponseWriter, writing *naoWriting) {
	writeJSON(w, http.StatusLocked, map[string]any{"error": naoLockedMessage(writing), "writing": writing})
}

// ── Ghi ──

// naoAffected trả những chương đã viết bị ảnh hưởng bởi một thay đổi bắt đầu ở fromChapter.
func naoAffected(progress *domain.Progress, fromChapter int) []int {
	if progress == nil || fromChapter <= 0 {
		return nil
	}
	var affected []int
	for _, ch := range progress.CompletedChapters {
		if ch >= fromChapter {
			affected = append(affected, ch)
		}
	}
	slices.Sort(affected)
	return slices.Compact(affected)
}

func naoNeedsApply(w http.ResponseWriter, affected []int) bool {
	if len(affected) == 0 {
		return false
	}
	writeJSON(w, http.StatusConflict, map[string]any{
		"error":    fmt.Sprintf("Sửa này chạm tới %d chương đã viết", len(affected)),
		"affected": affected,
	})
	return true
}

// naoQueueRewrite đưa các chương vào hàng đợi viết lại kèm ghi chú giới hạn phạm vi sửa.
// Gộp với hàng đợi đang có để không làm rơi việc khác; flow polishing giữ nguyên.
func naoQueueRewrite(s *store.Store, chapters []int, note string) error {
	progress, err := s.Progress.Load()
	if err != nil {
		return err
	}
	if progress == nil {
		return nil
	}
	merged := slices.Clone(chapters)
	for _, ch := range progress.PendingRewrites {
		if !slices.Contains(merged, ch) {
			merged = append(merged, ch)
		}
	}
	slices.Sort(merged)
	reason := note
	if progress.RewriteReason != "" && progress.RewriteReason != note {
		reason = note + " " + progress.RewriteReason
	}
	flow := domain.FlowRewriting
	if progress.Flow == domain.FlowPolishing {
		flow = domain.FlowPolishing
	}
	_, err = s.Progress.ApplyReviewOutcome(flow, merged, reason, store.RewriteCauseUser, 0)
	return err
}

func naoSingleLine(value string, max int) bool {
	return !strings.ContainsAny(value, "\r\n") && utf8.RuneCountInString(value) <= max
}

func naoApplyValue(apply string) (string, bool) {
	switch apply {
	case "", "next", "mark":
		return apply, true
	}
	return "", false
}

// naoReasonMissing trả 400 kèm tên trường để form web mở đúng ô lý do (Câu 21):
// sửa xưng hô và quan hệ trên web bắt buộc có lý do.
func naoReasonMissing(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusBadRequest, map[string]any{"error": message, "field": "reason"})
}

func (g *Gateway) naoPutAddressTerm(w http.ResponseWriter, r *http.Request, st library.Story, sr *host.StoryReader) {
	var req naoTermRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	term := req.Term
	term.From = strings.TrimSpace(term.From)
	term.To = strings.TrimSpace(term.To)
	term.Call = strings.TrimSpace(term.Call)
	term.Self = strings.TrimSpace(term.Self)
	term.Reason = strings.TrimSpace(term.Reason)
	if writing := g.naoWriting(st); writing != nil {
		naoLocked(w, writing)
		return
	}
	apply, ok := naoApplyValue(req.Apply)
	if !ok {
		jsonError(w, http.StatusBadRequest, "Giá trị áp dụng không hợp lệ")
		return
	}
	switch {
	case term.From == "" || term.To == "":
		jsonError(w, http.StatusBadRequest, "Cần chọn hai nhân vật cho dòng xưng hô")
		return
	case term.Call == "" || term.Self == "":
		jsonError(w, http.StatusBadRequest, "Cần điền cách gọi và cách tự xưng")
		return
	case term.Reason == "":
		naoReasonMissing(w, "Cần ghi lý do cho thay đổi xưng hô")
		return
	case !naoSingleLine(term.Call, naoFormMaxRunes) || !naoSingleLine(term.Self, naoFormMaxRunes):
		jsonError(w, http.StatusBadRequest, "Cách gọi và cách tự xưng phải trên một dòng, tối đa 200 ký tự")
		return
	case !naoSingleLine(term.Reason, naoReasonMaxRunes):
		jsonError(w, http.StatusBadRequest, "Lý do phải trên một dòng, tối đa 500 ký tự")
		return
	case term.FromChapter <= 0:
		jsonError(w, http.StatusBadRequest, "Từ chương phải lớn hơn 0")
		return
	}
	s := sr.Store()
	chars, err := s.Characters.Load()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	from, okFrom := store.FindCharacter(chars, term.From)
	to, okTo := store.FindCharacter(chars, term.To)
	if !okFrom || !okTo {
		jsonError(w, http.StatusBadRequest, "Không tìm thấy nhân vật trong hồ sơ")
		return
	}
	if from.ID == to.ID {
		jsonError(w, http.StatusBadRequest, "Nhân vật không thể xưng hô với chính mình")
		return
	}
	terms, err := s.World.LoadAddressTerms()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	active, found := naoActiveTerm(terms, from.ID, to.ID)
	if term.ID != "" {
		target, ok := naoTermByID(terms, term.ID)
		if !ok {
			jsonError(w, http.StatusBadRequest, "Dòng xưng hô không còn tồn tại, hãy tải lại")
			return
		}
		// Dòng đã hết hiệu lực là lịch sử, không phải dữ liệu đang dùng (Câu 21).
		if target.UntilChapter != 0 {
			jsonError(w, http.StatusConflict, fmt.Sprintf("Dòng xưng hô này đã hết hiệu lực từ sau chương %d, không sửa được; hãy thêm dòng mới", target.UntilChapter))
			return
		}
		if found && naoTermID(active) != term.ID {
			jsonError(w, http.StatusBadRequest, "Mỗi chiều chỉ sửa được dòng đang dùng; dòng khác hãy thêm mới")
			return
		}
	}
	progress, err := s.Progress.Load()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	latestWritten := 0
	if progress != nil {
		latestWritten = progress.LatestCompleted()
	}
	affected := naoAffected(progress, term.FromChapter)
	if apply == "" && naoNeedsApply(w, affected) {
		return
	}
	// "next" = chỉ áp dụng cho chương chưa viết: chương đã viết phải giữ cách gọi cũ,
	// nên dòng mới chỉ có hiệu lực từ chương chưa viết đầu tiên. "mark" giữ đúng chương
	// người dùng chọn và đưa các chương đã viết vào hàng đợi sửa.
	effectiveFrom := term.FromChapter
	if apply == "next" && effectiveFrom <= latestWritten {
		effectiveFrom = latestWritten + 1
	}
	if err := s.World.ValidateSetAddressTerm(effectiveFrom, term.From, term.To, term.Call, term.Self, term.Reason); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	before := ""
	if found {
		before = active.Calls
		if before == term.Call {
			before = active.Self
		}
	}
	// Xếp hàng đợi trước khi ghi: hàng đợi lỗi thì dữ liệu xưng hô không bị nửa vời.
	if apply == "mark" && len(affected) > 0 {
		note := fmt.Sprintf("Chỉ sửa câu có cách gọi %s → %s: “%s” → “%s”.", from.Name, to.Name, before, term.Call)
		if err := naoQueueRewrite(s, affected, note); err != nil {
			jsonError(w, http.StatusConflict, err.Error())
			return
		}
	}
	if _, err := s.World.SetAddressTerm(effectiveFrom, term.From, term.To, term.Call, term.Self, term.Reason); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	g.storyChanged(st.Project, st.Story, "address", "brain", "snapshot", "chapters")
	resp, err := g.naoAddressTerms(st, sr)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func naoActiveTerm(terms []domain.AddressTerm, from, to string) (domain.AddressTerm, bool) {
	for _, term := range terms {
		if term.From == from && term.To == to && term.UntilChapter == 0 {
			return term, true
		}
	}
	return domain.AddressTerm{}, false
}

// naoTermByID trả dòng xưng hô đúng mã hiển thị (from|to|fromChapter).
func naoTermByID(terms []domain.AddressTerm, id string) (domain.AddressTerm, bool) {
	for _, term := range terms {
		if naoTermID(term) == id {
			return term, true
		}
	}
	return domain.AddressTerm{}, false
}

func (g *Gateway) naoPostRelation(w http.ResponseWriter, r *http.Request, st library.Story, sr *host.StoryReader) {
	var req naoRelationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.A = strings.TrimSpace(req.A)
	req.B = strings.TrimSpace(req.B)
	req.After = strings.TrimSpace(req.After)
	req.Reason = strings.TrimSpace(req.Reason)
	if writing := g.naoWriting(st); writing != nil {
		naoLocked(w, writing)
		return
	}
	apply, ok := naoApplyValue(req.Apply)
	if !ok {
		jsonError(w, http.StatusBadRequest, "Giá trị áp dụng không hợp lệ")
		return
	}
	switch {
	case req.A == "" || req.B == "":
		jsonError(w, http.StatusBadRequest, "Cần chọn hai nhân vật cho quan hệ")
		return
	case req.After == "":
		jsonError(w, http.StatusBadRequest, "Cần điền quan hệ mới")
		return
	case req.Reason == "":
		naoReasonMissing(w, "Cần ghi lý do cho thay đổi quan hệ")
		return
	case !naoSingleLine(req.After, naoFormMaxRunes):
		jsonError(w, http.StatusBadRequest, "Quan hệ mới phải trên một dòng, tối đa 200 ký tự")
		return
	case !naoSingleLine(req.Reason, naoReasonMaxRunes):
		jsonError(w, http.StatusBadRequest, "Lý do phải trên một dòng, tối đa 500 ký tự")
		return
	case req.FromChapter <= 0:
		jsonError(w, http.StatusBadRequest, "Từ chương phải lớn hơn 0")
		return
	case req.Type != "" && !domain.ValidRelationshipType(req.Type):
		jsonError(w, http.StatusBadRequest, "Kiểu quan hệ không hợp lệ")
		return
	}
	typ := domain.NormalizeRelationshipType(strings.TrimSpace(req.Type))
	s := sr.Store()
	chars, err := s.Characters.Load()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a, okA := store.FindCharacter(chars, req.A)
	b, okB := store.FindCharacter(chars, req.B)
	if !okA || !okB {
		jsonError(w, http.StatusBadRequest, "Không tìm thấy nhân vật trong hồ sơ")
		return
	}
	if a.ID == b.ID {
		jsonError(w, http.StatusBadRequest, "Quan hệ cần hai nhân vật khác nhau")
		return
	}
	progress, err := s.Progress.Load()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	latestWritten := 0
	if progress != nil {
		latestWritten = progress.LatestCompleted()
	}
	affected := naoAffected(progress, req.FromChapter)
	if apply == "" && naoNeedsApply(w, affected) {
		return
	}
	// next: chương đã viết giữ quan hệ cũ, dữ kiện mới chỉ có hiệu lực từ chương chưa viết.
	effectiveFrom := req.FromChapter
	if apply == "next" && effectiveFrom <= latestWritten {
		effectiveFrom = latestWritten + 1
	}
	if err := s.World.ValidateRelationshipChange(effectiveFrom, req.A, req.B, req.After, typ); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	previous := naoRelationBefore(s, a.ID, b.ID)
	// Xếp hàng đợi trước khi ghi: hàng đợi lỗi thì dữ kiện quan hệ không bị nửa vời.
	if apply == "mark" && len(affected) > 0 {
		note := fmt.Sprintf("Chỉ sửa câu có quan hệ %s – %s: “%s” → “%s”.", a.Name, b.Name, previous, req.After)
		if err := naoQueueRewrite(s, affected, note); err != nil {
			jsonError(w, http.StatusConflict, err.Error())
			return
		}
	}
	if err := s.World.AppendRelationshipChange(effectiveFrom, req.A, req.B, req.After, req.Reason, typ); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	g.storyChanged(st.Project, st.Story, "address", "brain", "snapshot", "chapters")
	resp, err := g.naoAddressTerms(st, sr)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// naoRelationBefore đọc quan hệ gần nhất của cặp trước khi ghi dữ kiện mới.
func naoRelationBefore(s *store.Store, a, b string) string {
	log, err := s.World.LoadRelationshipLog()
	if err != nil {
		return ""
	}
	latest := ""
	for _, change := range log {
		if naoPairKey(change.From, change.To) == naoPairKey(a, b) {
			latest = change.Relation
		}
	}
	return latest
}

// ── Brain ──

func (g *Gateway) naoBrain(sr *host.StoryReader, upTo int) (naoBrainResponse, error) {
	s := sr.Store()
	progress, err := s.Progress.Load()
	if err != nil {
		return naoBrainResponse{}, err
	}
	chars, err := s.Characters.Load()
	if err != nil {
		return naoBrainResponse{}, err
	}
	cast, err := s.Cast.Load()
	if err != nil {
		return naoBrainResponse{}, err
	}
	ledger, err := s.World.LoadForeshadowLedger()
	if err != nil {
		return naoBrainResponse{}, err
	}
	timeline, err := s.World.LoadTimeline()
	if err != nil {
		return naoBrainResponse{}, err
	}
	rules, err := s.World.LoadWorldRules()
	if err != nil {
		return naoBrainResponse{}, err
	}
	log, err := s.World.LoadRelationshipLog()
	if err != nil {
		return naoBrainResponse{}, err
	}
	terms, err := s.World.LoadAddressTerms()
	if err != nil {
		return naoBrainResponse{}, err
	}
	chapters, err := naoBrainChapters(s, progress)
	if err != nil {
		return naoBrainResponse{}, err
	}
	charAppears, fsAppears, factItems := naoAppearances(s, progress)
	entities := naoBrainEntities(chars, cast, ledger, timeline, rules, charAppears, fsAppears)
	entities = append(entities, naoFactEntities(factItems)...)
	edges := naoMergeEdges(naoBrainEdges(log), naoFactEdges(factItems, naoEntityIndex(entities)))
	naoAttachTimelines(entities, edges, terms, chars)
	resp := naoBrainResponse{Chapters: chapters, Entities: entities, Edges: edges}
	if upTo > 0 {
		resp = naoTrimBrain(resp, upTo)
	}
	return resp, nil
}

// naoBrainChapters lấy chương từ đại cương; chương đã viết ngoài đại cương vẫn phải có mặt.
func naoBrainChapters(s *store.Store, progress *domain.Progress) ([]naoBrainChapter, error) {
	entries, err := s.Outline.LoadOutline()
	if err != nil {
		return nil, err
	}
	byNumber := make(map[int]naoBrainChapter, len(entries))
	titleOf := func(ch int) string {
		if sum, err := s.Summaries.LoadSummary(ch); err == nil && sum != nil {
			return sum.Title
		}
		return ""
	}
	for _, entry := range entries {
		if entry.Chapter <= 0 {
			continue
		}
		byNumber[entry.Chapter] = naoBrainChapter{N: entry.Chapter, Title: entry.Title, Status: naoChapterStatus(entry.Chapter, progress)}
	}
	if progress != nil {
		extra := append(slices.Clone(progress.CompletedChapters), progress.PendingRewrites...)
		if progress.InProgressChapter > 0 {
			extra = append(extra, progress.InProgressChapter)
		}
		for _, ch := range extra {
			if ch <= 0 {
				continue
			}
			if _, ok := byNumber[ch]; ok {
				continue
			}
			byNumber[ch] = naoBrainChapter{N: ch, Title: titleOf(ch), Status: naoChapterStatus(ch, progress)}
		}
	}
	out := make([]naoBrainChapter, 0, len(byNumber))
	for _, chapter := range byNumber {
		out = append(out, chapter)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].N < out[j].N })
	return out, nil
}

// naoChapterStatus thu về bốn trạng thái của thanh chương Não truyện (Câu 23):
// đã viết (xanh) · đang viết lại (tím) · đang viết lần đầu (tím) · chưa viết (vòng rỗng).
// Chương đang trong hàng đợi viết lại thắng trạng thái "đang viết": engine đang sửa
// bản đã có, không phải viết mới.
func naoChapterStatus(ch int, progress *domain.Progress) string {
	if progress == nil {
		return "unwritten"
	}
	if slices.Contains(progress.PendingRewrites, ch) {
		return "rewriting"
	}
	if progress.InProgressChapter == ch {
		return "writing"
	}
	if slices.Contains(progress.CompletedChapters, ch) {
		return "written"
	}
	return "unwritten"
}

// naoAppearances dựng "xuất hiện ở chương nào" từ bản ghi nghiệm thu từng chương
// (nguồn: facts.characters và facts.foreshadow_updates). Truyện cũ chưa có bản ghi
// thì lùi về tóm tắt chương (summaries/NN.json characters). Cùng vòng đọc gom luôn
// places/objects (GAP-49) để Não truyện chỉ đọc mỗi bản ghi một lần.
func naoAppearances(s *store.Store, progress *domain.Progress) (chars, foreshadow map[string][]int, facts []naoFactEntity) {
	chars = map[string][]int{}
	foreshadow = map[string][]int{}
	if progress == nil {
		return chars, foreshadow, facts
	}
	collector := naoFactCollector{byID: map[string]*naoFactEntity{}}
	completed := slices.Clone(progress.CompletedChapters)
	slices.Sort(completed)
	for _, ch := range completed {
		if ch <= 0 {
			continue
		}
		var names []string
		record, err := s.ChapterRecords.Load(ch)
		if err == nil && record != nil {
			names = record.Facts.Characters
			for _, update := range record.Facts.ForeshadowUpdates {
				id := strings.TrimSpace(update.ID)
				if id == "" || slices.Contains(foreshadow[id], ch) {
					continue
				}
				foreshadow[id] = append(foreshadow[id], ch)
			}
			for _, item := range record.Facts.Places {
				collector.add("dd", "place", item, ch)
			}
			for _, item := range record.Facts.Objects {
				collector.add("vat", "obj", item, ch)
			}
		} else if sum, err := s.Summaries.LoadSummary(ch); err == nil && sum != nil {
			names = sum.Characters
		}
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name == "" || slices.Contains(chars[name], ch) {
				continue
			}
			chars[name] = append(chars[name], ch)
		}
	}
	return chars, foreshadow, collector.entities()
}

// naoFactEntity là một địa điểm hoặc vật đã gộp từ chapterfacts của mọi chương đã
// viết (GAP-49). Mã ổn định qua các chương: dd:<slug> cho địa điểm, vat:<slug> cho vật.
type naoFactEntity struct {
	ID          string
	Type        string
	Name        string
	Desc        string
	FromChapter int
	Appears     []int
	// Links là những gì đi kèm địa điểm/vật, theo chương, để dựng cạnh "Liên quan".
	Links []naoFactLink
}

// naoFactLink là một tên trong facts `with[]` của đúng chương đó.
type naoFactLink struct {
	Chapter int
	Name    string
}

// naoFactCollector gộp places/objects của nhiều chương theo mã: cùng một địa điểm
// xuất hiện ở nhiều chương là một nút, không phải nhiều nút trùng tên.
type naoFactCollector struct {
	byID  map[string]*naoFactEntity
	order []string
}

func (c *naoFactCollector) add(prefix, typ string, item domain.FactEntity, chapter int) {
	name := strings.TrimSpace(item.Name)
	if name == "" {
		return
	}
	id := prefix + ":" + naoEntitySlug(name)
	entity := c.byID[id]
	if entity == nil {
		entity = &naoFactEntity{ID: id, Type: typ, Name: name, FromChapter: chapter}
		c.byID[id] = entity
		c.order = append(c.order, id)
	}
	if entity.Desc == "" {
		entity.Desc = strings.TrimSpace(item.Desc)
	}
	// firstChapter do model khai có thể sớm hơn chương đang đọc (địa điểm đã có từ
	// chương trước); chỉ nhận khi nó sớm hơn, không bao giờ muộn hơn lần xuất hiện thật.
	if item.FirstChapter > 0 && item.FirstChapter < entity.FromChapter {
		entity.FromChapter = item.FirstChapter
	}
	if !slices.Contains(entity.Appears, chapter) {
		entity.Appears = append(entity.Appears, chapter)
	}
	for _, ref := range item.With {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		entity.Links = append(entity.Links, naoFactLink{Chapter: chapter, Name: ref})
	}
}

func (c *naoFactCollector) entities() []naoFactEntity {
	out := make([]naoFactEntity, 0, len(c.order))
	for _, id := range c.order {
		entity := c.byID[id]
		slices.Sort(entity.Appears)
		entity.Appears = slices.Compact(entity.Appears)
		out = append(out, *entity)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].FromChapter != out[j].FromChapter {
			return out[i].FromChapter < out[j].FromChapter
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// naoFactEntities chuyển địa điểm/vật đã gộp thành thực thể Não truyện (GAP-49).
// Không có dữ kiện thì trả rỗng: FE tự ẩn chip lọc và nhóm của loại đó.
func naoFactEntities(facts []naoFactEntity) []naoBrainEntity {
	entities := make([]naoBrainEntity, 0, len(facts))
	for _, fact := range facts {
		entities = append(entities, naoBrainEntity{
			ID: fact.ID, Type: fact.Type, Name: fact.Name, Desc: fact.Desc,
			FromChapter: fact.FromChapter, Appears: naoChapterList(fact.Appears),
		})
	}
	return entities
}

// naoRelLabel là nhãn nhóm "Liên quan" của Não truyện (artboard Brain, ET.rel):
// with[] của places/objects không phải quan hệ có tên nên chỉ mang nhóm này.
const naoRelLabel = "Liên quan"

// naoFactEdges nối địa điểm/vật với những gì đi kèm nó trong chương (GAP-49): mỗi
// tên trong with[] là một pha kiểu "rel" bắt đầu ở chương đó. Tên không có thực thể
// tương ứng thì bỏ — Não truyện không vẽ nút cho thứ chưa có dữ kiện.
func naoFactEdges(facts []naoFactEntity, byName map[string]string) []naoBrainEdge {
	var edges []naoBrainEdge
	for _, fact := range facts {
		for _, link := range fact.Links {
			other, ok := byName[link.Name]
			if !ok || other == fact.ID {
				continue
			}
			edges = append(edges, naoBrainEdge{A: fact.ID, B: other, Phases: []naoBrainPhase{
				{FromChapter: link.Chapter, Type: "rel", Label: naoRelLabel},
			}})
		}
	}
	return edges
}

// naoEntityIndex tra tên/biệt danh đã thấy về mã thực thể để nối with[] của
// places/objects (GAP-49). Tên trùng giữa hai loại: bản ghi trước thắng — nhân vật
// đứng trước trong danh sách dựng, nên tên nhân vật không bị địa điểm chiếm mất.
func naoEntityIndex(entities []naoBrainEntity) map[string]string {
	index := make(map[string]string, len(entities)*2)
	for _, entity := range entities {
		for _, ref := range append([]string{entity.Name}, entity.Aliases...) {
			ref = strings.TrimSpace(ref)
			if ref == "" {
				continue
			}
			if _, ok := index[ref]; !ok {
				index[ref] = entity.ID
			}
		}
	}
	return index
}

// naoMergeEdges gộp các cạnh cùng cặp thực thể (relationship_log và with[] của
// places/objects): một cặp chỉ một cạnh, mỗi lần đổi là một pha xếp theo chương.
func naoMergeEdges(groups ...[]naoBrainEdge) []naoBrainEdge {
	states := map[string]*naoBrainEdge{}
	order := make([]string, 0, len(groups))
	for _, group := range groups {
		for _, edge := range group {
			key := naoPairKey(edge.A, edge.B)
			state := states[key]
			if state == nil {
				state = &naoBrainEdge{A: edge.A, B: edge.B}
				states[key] = state
				order = append(order, key)
			}
			for _, phase := range edge.Phases {
				if slices.ContainsFunc(state.Phases, func(p naoBrainPhase) bool {
					return p.FromChapter == phase.FromChapter && p.Type == phase.Type && p.Label == phase.Label
				}) {
					continue
				}
				state.Phases = append(state.Phases, phase)
			}
		}
	}
	edges := make([]naoBrainEdge, 0, len(order))
	for _, key := range order {
		state := states[key]
		sort.SliceStable(state.Phases, func(i, j int) bool { return state.Phases[i].FromChapter < state.Phases[j].FromChapter })
		edges = append(edges, *state)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].A != edges[j].A {
			return edges[i].A < edges[j].A
		}
		return edges[i].B < edges[j].B
	})
	return edges
}

func naoCharacterAppearances(c domain.Character, byName map[string][]int) []int {
	out := []int{}
	for _, ref := range append([]string{c.Name}, c.Aliases...) {
		out = append(out, byName[strings.TrimSpace(ref)]...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// naoChapterList giữ `appears` luôn là mảng JSON (không null) như hợp đồng GAP-39.
func naoChapterList(chapters []int) []int {
	if chapters == nil {
		return []int{}
	}
	return chapters
}

// naoAliases chuẩn hoá biệt danh của Não identity: bỏ rỗng, bỏ trùng, giữ thứ tự khai báo.
func naoAliases(aliases []string) []string {
	out := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" || slices.Contains(out, alias) {
			continue
		}
		out = append(out, alias)
	}
	return out
}

// naoAttachTimelines dựng "Diễn biến" cho từng thực thể từ hai nguồn dữ kiện đã có:
// relationship_log (mỗi lần đổi quan hệ là một mốc, kèm tên đối phương) và
// address_terms (lần đổi cách gọi sau chương 1). Đây đúng là hai nguồn artboard
// Brain/MBrain dùng cho danh sách "Diễn biến"; tóm tắt chương và timeline.json không
// được trộn vào vì artboard không hiển thị chúng.
func naoAttachTimelines(entities []naoBrainEntity, edges []naoBrainEdge, terms []domain.AddressTerm, chars []domain.Character) {
	index := make(map[string]int, len(entities)*2)
	for i, entity := range entities {
		if entity.ID != "" {
			index[entity.ID] = i
		}
	}
	for i, entity := range entities {
		if _, ok := index[entity.Name]; !ok && entity.Name != "" {
			index[entity.Name] = i
		}
	}
	moments := make([][]naoBrainMoment, len(entities))
	add := func(ref string, chapter int, text string) {
		i, ok := index[ref]
		if !ok || chapter <= 0 || text == "" {
			return
		}
		if slices.ContainsFunc(moments[i], func(m naoBrainMoment) bool { return m.Chapter == chapter && m.Text == text }) {
			return
		}
		moments[i] = append(moments[i], naoBrainMoment{Chapter: chapter, Text: text})
	}
	for _, edge := range edges {
		for i, phase := range edge.Phases {
			prefix := ""
			if i > 0 {
				prefix = "Đổi thành "
			}
			add(edge.A, phase.FromChapter, prefix+phase.Label+" · "+naoName(chars, edge.B))
			add(edge.B, phase.FromChapter, prefix+phase.Label+" · "+naoName(chars, edge.A))
		}
	}
	for _, term := range terms {
		// Cách gọi có từ chương 1 là dữ kiện gốc, không phải một lần đổi.
		if term.FromChapter <= 1 {
			continue
		}
		add(term.From, term.FromChapter,
			naoName(chars, term.From)+" gọi "+naoName(chars, term.To)+" là “"+term.Calls+"”")
	}
	for i := range entities {
		slices.SortStableFunc(moments[i], func(a, b naoBrainMoment) int { return a.Chapter - b.Chapter })
		entities[i].Timeline = naoChapterMoments(moments[i])
		entities[i].Aliases = naoAliases(entities[i].Aliases)
	}
}

func naoChapterMoments(moments []naoBrainMoment) []naoBrainMoment {
	if moments == nil {
		return []naoBrainMoment{}
	}
	return moments
}

// naoTrimBrain cắt dữ kiện Não truyện theo chương đang đọc (M8 "Chỉ hiện những gì
// đã xảy ra tới chương đang đọc, không lộ chương sau"): bỏ thực thể có mặt sau
// chương N, bỏ mốc xuất hiện/diễn biến sau N, bỏ pha quan hệ bắt đầu sau N.
// Cạnh còn lại phải nối hai thực thể đang thấy, nếu không thì bỏ luôn.
func naoTrimBrain(resp naoBrainResponse, upTo int) naoBrainResponse {
	visible := make(map[string]bool, len(resp.Entities)*2)
	entities := make([]naoBrainEntity, 0, len(resp.Entities))
	for _, entity := range resp.Entities {
		if entity.FromChapter > upTo {
			continue
		}
		appears := make([]int, 0, len(entity.Appears))
		for _, ch := range entity.Appears {
			if ch <= upTo {
				appears = append(appears, ch)
			}
		}
		timeline := make([]naoBrainMoment, 0, len(entity.Timeline))
		for _, moment := range entity.Timeline {
			if moment.Chapter <= upTo {
				timeline = append(timeline, moment)
			}
		}
		entity.Appears, entity.Timeline = appears, timeline
		visible[entity.ID], visible[entity.Name] = true, true
		entities = append(entities, entity)
	}
	edges := make([]naoBrainEdge, 0, len(resp.Edges))
	for _, edge := range resp.Edges {
		if !visible[edge.A] || !visible[edge.B] {
			continue
		}
		phases := make([]naoBrainPhase, 0, len(edge.Phases))
		for _, phase := range edge.Phases {
			if phase.FromChapter <= upTo {
				phases = append(phases, phase)
			}
		}
		if len(phases) == 0 {
			continue
		}
		edge.Phases = phases
		edges = append(edges, edge)
	}
	resp.Entities, resp.Edges = entities, edges
	return resp
}

func naoBrainEntities(chars []domain.Character, cast []domain.CastEntry, ledger []domain.ForeshadowEntry,
	timeline []domain.TimelineEvent, rules []domain.WorldRule, charAppears, fsAppears map[string][]int) []naoBrainEntity {
	entities := make([]naoBrainEntity, 0, len(chars)+len(cast)+len(ledger)+len(timeline)+len(rules))
	known := make(map[string]bool, len(chars))
	for _, c := range chars {
		known[c.Name] = true
		appears := naoCharacterAppearances(c, charAppears)
		from := 0
		if len(appears) > 0 {
			from = appears[0]
		}
		entities = append(entities, naoBrainEntity{
			ID: c.ID, Type: "char", Name: c.Name, Role: c.Role, Ini: c.Ini,
			Kind: string(c.Kind), RevealAt: c.RevealAt, Desc: c.Description,
			FromChapter: from, Appears: appears, Aliases: naoAliases(c.Aliases),
		})
	}
	for _, entry := range cast {
		name := strings.TrimSpace(entry.Name)
		if name == "" || known[name] {
			continue
		}
		known[name] = true
		appears := slices.Clone(entry.AppearanceChapters)
		slices.Sort(appears)
		from := entry.FirstSeenChapter
		if len(appears) > 0 && (from <= 0 || appears[0] < from) {
			from = appears[0]
		}
		entities = append(entities, naoBrainEntity{
			ID: "cast:" + name, Type: "char", Name: name, Role: entry.BriefRole,
			Kind:        string(store.CharacterKindOf(entry.BriefRole)),
			FromChapter: from, Appears: naoChapterList(slices.Compact(appears)), Aliases: naoAliases(entry.Aliases),
		})
	}
	for _, entry := range ledger {
		appears := slices.Clone(fsAppears[entry.ID])
		if entry.PlantedAt > 0 && !slices.Contains(appears, entry.PlantedAt) {
			appears = append(appears, entry.PlantedAt)
		}
		slices.Sort(appears)
		entities = append(entities, naoBrainEntity{
			ID: "fs:" + entry.ID, Type: "fs", Name: entry.Description,
			FromChapter: entry.PlantedAt, Appears: naoChapterList(slices.Compact(appears)),
		})
	}
	for i, event := range timeline {
		entities = append(entities, naoBrainEntity{
			ID: fmt.Sprintf("ev:%d-%d", event.Chapter, i), Type: "ev", Name: event.Event,
			FromChapter: event.Chapter, Appears: []int{event.Chapter},
		})
	}
	for i, rule := range rules {
		entities = append(entities, naoBrainEntity{
			ID: fmt.Sprintf("rule:%d", i), Type: "rule", Name: rule.Rule, Desc: rule.Boundary,
			// Luật thế giới thuộc phần thiết lập, có trước chương 1; không có dữ kiện theo chương.
			FromChapter: 1, Appears: []int{},
		})
	}
	return entities
}

// naoBrainEdges gộp relationship_log thành cạnh theo cặp, mỗi lần đổi là một pha.
// Type lấy từ dữ kiện engine ghi (D10); dữ kiện cũ thiếu kiểu đọc là "rel".
func naoBrainEdges(log []domain.RelationshipChange) []naoBrainEdge {
	type edgeState struct {
		a, b   string
		phases []naoBrainPhase
	}
	states := map[string]*edgeState{}
	last := map[string]string{}
	for _, change := range log {
		key := naoPairKey(change.From, change.To)
		state := states[key]
		if state == nil {
			state = &edgeState{a: change.From, b: change.To}
			states[key] = state
		}
		if label, ok := last[key]; ok && label == change.Relation {
			continue
		}
		last[key] = change.Relation
		state.phases = append(state.phases, naoBrainPhase{
			FromChapter: change.Chapter, Type: domain.NormalizeRelationshipType(change.Type), Label: change.Relation,
		})
	}
	edges := make([]naoBrainEdge, 0, len(states))
	for _, state := range states {
		sort.SliceStable(state.phases, func(i, j int) bool { return state.phases[i].FromChapter < state.phases[j].FromChapter })
		edges = append(edges, naoBrainEdge{A: state.a, B: state.b, Phases: state.phases})
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].A != edges[j].A {
			return edges[i].A < edges[j].A
		}
		return edges[i].B < edges[j].B
	})
	return edges
}
