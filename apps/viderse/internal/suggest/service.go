package suggest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/datnt1998/verse/apps/viderse/internal/learn"
)

// Ops là các thao tác có sẵn mà đề xuất được chạy qua: cùng service với UI.
// Chỉ gọi sau khi người dùng bấm nút chính của thẻ.
type Ops interface {
	// Prioritize đưa một mục trong hàng lên đầu hàng của kênh.
	Prioritize(ctx context.Context, channelID, queueID string) error
	// PlanImages ước lượng số ảnh trả phí khi làm video này (nil: chưa ước lượng được).
	PlanImages(ctx context.Context, channelID, videoID string) (*int, error)
	// Pick đưa một video đối thủ vào làm (hàng Pick của kênh).
	Pick(ctx context.Context, channelID, videoID string) error
	// SetPace đặt nhịp mỗi ngày và trả nhịp cũ.
	SetPace(ctx context.Context, channelID string, pace int) (previous int, err error)
	// SetApproveScore bật Duyệt theo điểm và trả mức tự chủ cũ (JSON để hoàn tác).
	SetApproveScore(ctx context.Context, channelID string, min int) (previous json.RawMessage, err error)
	// RestoreAutonomy đặt lại mức tự chủ từ JSON của SetApproveScore.
	RestoreAutonomy(ctx context.Context, channelID string, previous json.RawMessage) error
	// RemoveQueue bỏ các mục khỏi hàng chờ (hoàn tác “Tự lấy”; mục đã chạy thì bỏ qua êm).
	RemoveQueue(ctx context.Context, ids []string) error
	// ReclaimMP4 xoá MP4 của video đã đăng; trả số byte lấy lại.
	ReclaimMP4(ctx context.Context) (int64, error)
}

type Service struct {
	Store    Store
	Prefs    Prefs
	Learn    *learn.Service
	Ops      Ops
	Perf     PerfSource
	Autonomy AutonomySource
	Cleanup  CleanupSource
	Now      func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// ---- Đọc ----------------------------------------------------------------

// List trả đề xuất mở xếp theo giá trị (tối đa 5), lọc theo kênh nếu có.
// total là số đề xuất mở sau khi lọc.
func (s *Service) List(ctx context.Context, channelID string, limit int) ([]Suggestion, int, error) {
	all, err := s.openVisible(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := all[:0:0]
	for _, x := range all {
		if channelID == "" || x.ChannelID == nil || *x.ChannelID == channelID {
			out = append(out, x)
		}
	}
	total := len(out)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, total, nil
}

// openVisible: mở, chưa hết hạn, loại chưa bị tắt, cắt còn MaxOpen.
func (s *Service) openVisible(ctx context.Context) ([]Suggestion, error) {
	list, err := s.Store.Open(ctx, s.now())
	if err != nil {
		return nil, err
	}
	var offs []Off
	if s.Prefs != nil {
		if offs, err = s.Prefs.SuggestionOff(ctx); err != nil {
			return nil, err
		}
	}
	out := make([]Suggestion, 0, MaxOpen)
	for _, x := range list {
		cid := ""
		if x.ChannelID != nil {
			cid = *x.ChannelID
		}
		if off(offs, x.Kind, cid) {
			continue
		}
		out = append(out, x)
		if len(out) == MaxOpen {
			break
		}
	}
	return out, nil
}

// ForRecap là tối đa 3 đề xuất đầu cho Bản tin.
func (s *Service) ForRecap(ctx context.Context) ([]any, error) {
	list, _, err := s.List(ctx, "", 3)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(list))
	for i := range list {
		out[i] = list[i]
	}
	return out, nil
}

// LearnFor trả đề xuất learn_rule đang mở mà video này góp phần (nil: không có).
func (s *Service) LearnFor(ctx context.Context, jobID, agent string) (*Suggestion, error) {
	x, ok, err := s.Store.OpenLearnForJob(ctx, jobID, agent, s.now())
	if err != nil || !ok {
		return nil, err
	}
	return &x, nil
}

// ---- Sinh ---------------------------------------------------------------

// OfferLearn tạo (hoặc làm mới) đề xuất learn_rule cho các kiểu sửa vừa lặp đủ
// số lần, và trả đề xuất đầu tiên đang mở. Không bao giờ toast, không tự lưu.
func (s *Service) OfferLearn(ctx context.Context, cands []learn.Candidate) (*Suggestion, error) {
	var first *Suggestion
	for _, c := range cands {
		if s.disabled(ctx, KindLearnRule, c.ChannelID) {
			continue
		}
		d, err := s.learnDraft(ctx, c)
		if err != nil {
			return nil, err
		}
		x, _, err := s.Store.Upsert(ctx, d, s.now())
		if err != nil {
			return nil, err
		}
		if x.State == StateOpen && first == nil {
			first = &x
		}
	}
	if first != nil {
		if err := s.Store.TrimOpen(ctx, MaxOpen, s.now()); err != nil {
			return nil, err
		}
		if again, ok, err := s.Store.Get(ctx, first.ID); err != nil {
			return nil, err
		} else if !ok || again.State != StateOpen {
			return nil, nil
		}
	}
	return first, nil
}

func (s *Service) disabled(ctx context.Context, kind, channelID string) bool {
	if s.Prefs == nil {
		return false
	}
	offs, err := s.Prefs.SuggestionOff(ctx)
	return err == nil && off(offs, kind, channelID)
}

// Refresh chạy bộ sinh: một lần mỗi chu kỳ quét đối thủ. Một loại lỗi không
// chặn các loại khác; lỗi gộp lại để log.
func (s *Service) Refresh(ctx context.Context) error {
	now := s.now()
	if err := s.Store.ExpireStale(ctx, now); err != nil {
		return err
	}
	var offs []Off
	if s.Prefs != nil {
		var err error
		if offs, err = s.Prefs.SuggestionOff(ctx); err != nil {
			return err
		}
	}
	names, err := s.Store.ChannelNames(ctx)
	if err != nil {
		return err
	}
	var errs []error
	gen := func(kind string, f func() ([]Draft, error)) {
		drafts, err := f()
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", kind, err))
			return
		}
		var keep []string
		for _, d := range drafts {
			if off(offs, kind, d.ChannelID) {
				continue
			}
			if d.ChannelName == "" {
				d.ChannelName = names[d.ChannelID]
			}
			x, _, err := s.Store.Upsert(ctx, d, now)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", kind, err))
				return
			}
			keep = append(keep, x.ID)
		}
		if err := s.Store.CloseOpenExcept(ctx, kind, keep, now); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", kind, err))
		}
	}
	gen(KindHotVideo, func() ([]Draft, error) { return s.hotDrafts(ctx, now) })
	gen(KindTemplateFail, func() ([]Draft, error) { return s.failingDrafts(ctx, now) })
	gen(KindLearnRule, func() ([]Draft, error) { return s.learnDrafts(ctx) })
	gen(KindPace, func() ([]Draft, error) { return s.paceDrafts(ctx, now) })
	if s.Autonomy != nil {
		gen(KindRaiseAutonomy, func() ([]Draft, error) { return s.autonomyDrafts(ctx, names) })
	}
	if s.Perf != nil {
		gen(KindPerfInsight, func() ([]Draft, error) { return s.perfDrafts(ctx, names) })
	}
	if s.Cleanup != nil {
		gen(KindCleanup, func() ([]Draft, error) { return s.cleanupDrafts(ctx) })
	}
	if err := s.Store.TrimOpen(ctx, MaxOpen, now); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// ---- Hành động ----------------------------------------------------------

type Undo struct {
	Op     string          `json:"op"`
	Params json.RawMessage `json:"params"`
}
type AcceptResult struct {
	OK      bool   `json:"ok"`
	Summary string `json:"summary"`
	Undo    *Undo  `json:"undo,omitempty"`
	Href    string `json:"href,omitempty"`
	RuleID  string `json:"ruleId,omitempty"`
}

// Accept chạy thao tác của nút chính. Thao tác tốn ảnh hoặc xoá file cần
// confirm=true; thiếu thì trả *ConfirmRequiredError và không chạy gì.
func (s *Service) Accept(ctx context.Context, id string, confirm bool) (AcceptResult, error) {
	x, ok, err := s.Store.Get(ctx, id)
	if err != nil {
		return AcceptResult{}, err
	}
	if !ok {
		return AcceptResult{}, ErrNotFound
	}
	if x.State != StateOpen {
		return AcceptResult{}, ErrGone
	}
	if x.Primary.Href != "" {
		return AcceptResult{}, ErrNavigation
	}
	var res AcceptResult
	switch x.Primary.Op {
	case "add_rule":
		res, err = s.acceptRule(ctx, x)
	case "prioritize":
		var p struct{ ChannelID, QueueID string }
		if err = json.Unmarshal(x.Primary.Params, &p); err == nil {
			if err = s.Ops.Prioritize(ctx, p.ChannelID, p.QueueID); err == nil {
				res = AcceptResult{OK: true, Summary: "Đã đưa lên đầu hàng"}
			}
		}
	case "pick":
		var p struct{ ChannelID, VideoID string }
		if err = json.Unmarshal(x.Primary.Params, &p); err != nil {
			break
		}
		if !confirm {
			images, _ := s.Ops.PlanImages(ctx, p.ChannelID, p.VideoID)
			n := 0
			if images != nil {
				n = *images
			}
			return AcceptResult{}, &ConfirmRequiredError{Images: n}
		}
		if err = s.Ops.Pick(ctx, p.ChannelID, p.VideoID); err == nil {
			res = AcceptResult{OK: true, Summary: "Đã đưa vào hàng làm", Href: "#/queue"}
		}
	case "set_pace":
		var p struct {
			ChannelID string `json:"channelId"`
			Pace      int    `json:"pace"`
		}
		if err = json.Unmarshal(x.Primary.Params, &p); err != nil {
			break
		}
		var prev int
		if prev, err = s.Ops.SetPace(ctx, p.ChannelID, p.Pace); err == nil {
			raw, _ := json.Marshal(map[string]any{"channelId": p.ChannelID, "pace": prev})
			res = AcceptResult{OK: true, Summary: fmt.Sprintf("Đã tăng nhịp lên %d video/ngày", p.Pace), Undo: &Undo{Op: "set_pace", Params: raw}}
		}
	case "set_autonomy":
		var p struct {
			ChannelID string `json:"channelId"`
			Min       int    `json:"min"`
		}
		if err = json.Unmarshal(x.Primary.Params, &p); err != nil {
			break
		}
		var prev json.RawMessage
		if prev, err = s.Ops.SetApproveScore(ctx, p.ChannelID, p.Min); err == nil {
			raw, _ := json.Marshal(map[string]any{"channelId": p.ChannelID, "autonomy": prev})
			res = AcceptResult{OK: true, Summary: fmt.Sprintf("Đã bật Duyệt theo điểm ≥ %d", p.Min), Undo: &Undo{Op: "restore_autonomy", Params: raw}}
		}
	case "cleanup":
		if !confirm {
			return AcceptResult{}, &ConfirmRequiredError{}
		}
		var freed int64
		if freed, err = s.Ops.ReclaimMP4(ctx); err == nil {
			res = AcceptResult{OK: true, Summary: "Đã dọn MP4, lấy lại " + FormatBytes(freed)}
		}
	default:
		return AcceptResult{}, ErrBadOp
	}
	if err != nil {
		return AcceptResult{}, err
	}
	now := s.now()
	until := now.Add(Cooldown)
	if _, err := s.Store.Resolve(ctx, id, StateAccepted, &until, now); err != nil {
		return AcceptResult{}, err
	}
	return res, nil
}

type ruleParams struct {
	Scope struct {
		ChannelID     string `json:"channelId"`
		ImagePromptID string `json:"imagePromptId"`
	} `json:"scope"`
	Agent       string            `json:"agent"`
	Text        string            `json:"text"`
	Key         string            `json:"key"`
	PromptName  string            `json:"promptName"`
	LearnedFrom learn.LearnedFrom `json:"learnedFrom"`
}

func (s *Service) acceptRule(ctx context.Context, x Suggestion) (AcceptResult, error) {
	var p ruleParams
	if err := json.Unmarshal(x.Primary.Params, &p); err != nil {
		return AcceptResult{}, err
	}
	rule, err := s.Learn.SaveCandidate(ctx, learn.Candidate{
		Key: p.Key, Agent: p.Agent, ChannelID: p.Scope.ChannelID, ImagePromptID: p.Scope.ImagePromptID,
		Text: p.Text, Count: p.LearnedFrom.Count, From: p.LearnedFrom.From, To: p.LearnedFrom.To, Examples: p.LearnedFrom.Examples,
	})
	if err != nil {
		return AcceptResult{}, err
	}
	raw, _ := json.Marshal(map[string]string{"id": rule.ID})
	summary := "Đã lưu quy tắc"
	if p.Agent == learn.AgentArtist && p.PromptName != "" {
		summary = "Đã thêm vào Thư viện › Prompt ảnh › " + p.PromptName + " › Ghi chú đã học"
	}
	return AcceptResult{OK: true, Summary: summary, RuleID: rule.ID, Undo: &Undo{Op: "remove_rule", Params: raw}}, nil
}

// Dismiss bỏ qua đề xuất: không hiện lại trong 7 ngày. forever tắt loại này
// cho kênh của đề xuất (hoặc mọi kênh nếu đề xuất không thuộc kênh nào).
func (s *Service) Dismiss(ctx context.Context, id string, forever bool) error {
	x, ok, err := s.Store.Get(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	if x.State != StateOpen {
		return ErrGone
	}
	now := s.now()
	until := now.Add(Cooldown)
	if _, err := s.Store.Resolve(ctx, id, StateDismissed, &until, now); err != nil {
		return err
	}
	if forever && s.Prefs != nil {
		cid := ""
		if x.ChannelID != nil {
			cid = *x.ChannelID
		}
		return s.Prefs.DisableSuggestion(ctx, x.Kind, cid)
	}
	return nil
}

// Undo chạy một thao tác hoàn tác trong danh sách cho phép.
func (s *Service) Undo(ctx context.Context, op string, params json.RawMessage) error {
	switch op {
	case "remove_rule":
		var p struct{ ID string }
		if err := json.Unmarshal(params, &p); err != nil {
			return ErrBadOp
		}
		if err := s.Learn.Delete(ctx, p.ID); err != nil {
			if errors.Is(err, learn.ErrRuleNotFound) {
				return ErrNoUndo
			}
			return err
		}
		return nil
	case "set_pace":
		var p struct {
			ChannelID string `json:"channelId"`
			Pace      int    `json:"pace"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Pace < 1 {
			return ErrBadOp
		}
		_, err := s.Ops.SetPace(ctx, p.ChannelID, p.Pace)
		return err
	case "remove_queue":
		var p struct {
			IDs []string `json:"ids"`
		}
		if err := json.Unmarshal(params, &p); err != nil || len(p.IDs) == 0 {
			return ErrBadOp
		}
		return s.Ops.RemoveQueue(ctx, p.IDs)
	case "restore_autonomy":
		var p struct {
			ChannelID string          `json:"channelId"`
			Autonomy  json.RawMessage `json:"autonomy"`
		}
		if err := json.Unmarshal(params, &p); err != nil || len(p.Autonomy) == 0 {
			return ErrBadOp
		}
		return s.Ops.RestoreAutonomy(ctx, p.ChannelID, p.Autonomy)
	}
	return ErrBadOp
}

// ---- Soạn chữ -------------------------------------------------------------

// FormatBytes: “12,4 GB”, “850 MB”, số thập phân dùng dấu phẩy.
func FormatBytes(n int64) string {
	const gb, mb = int64(1) << 30, int64(1) << 20
	if n >= gb {
		return strings.Replace(fmt.Sprintf("%.1f GB", float64(n)/float64(gb)), ".", ",", 1)
	}
	return fmt.Sprintf("%d MB", (n+mb/2)/mb)
}

// ratio: ×4,2
func ratio(x float64) string { return "×" + strings.Replace(fmt.Sprintf("%.1f", x), ".", ",", 1) }

// shortTitle giữ phần đuôi của tiêu đề dài, như artboard: “…Station That Shut Down in 1962”.
func shortTitle(t string) string {
	t = strings.TrimSpace(t)
	if utf8.RuneCountInString(t) <= 36 {
		return t
	}
	r := []rune(t)
	tail := string(r[len(r)-32:])
	if i := strings.Index(tail, " "); i >= 0 && i < 12 {
		tail = tail[i+1:]
	}
	return "…" + tail
}
