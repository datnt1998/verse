package suggest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
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
	// ReclaimMP4 xoá MP4 của video đã đăng; trả số byte lấy lại.
	ReclaimMP4(ctx context.Context) (int64, error)
}

type Service struct {
	Store   Store
	Prefs   Prefs
	Ops     Ops
	Cleanup CleanupSource
	Now     func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) enabled(ctx context.Context) (bool, error) {
	if s.Prefs == nil {
		return true, nil
	}
	return s.Prefs.SuggestionsEnabled(ctx)
}

// ---- Đọc ----------------------------------------------------------------

// List trả đề xuất mở xếp theo giá trị (tối đa 5), lọc theo kênh nếu có.
// total là số đề xuất mở sau khi lọc. Công tắc chung tắt → rỗng.
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

// openVisible: mở, chưa hết hạn, thuộc loại còn sống, cắt còn MaxOpen.
func (s *Service) openVisible(ctx context.Context) ([]Suggestion, error) {
	if on, err := s.enabled(ctx); err != nil {
		return nil, err
	} else if !on {
		return nil, nil
	}
	list, err := s.Store.Open(ctx, s.now())
	if err != nil {
		return nil, err
	}
	live := make(map[string]bool, len(Kinds))
	for _, kind := range Kinds {
		live[kind] = true
	}
	out := make([]Suggestion, 0, MaxOpen)
	for _, x := range list {
		if !live[x.Kind] {
			continue
		}
		out = append(out, x)
		if len(out) == MaxOpen {
			break
		}
	}
	return out, nil
}

// ---- Sinh ---------------------------------------------------------------

// Refresh chạy bộ sinh: một lần mỗi chu kỳ quét đối thủ. Một loại lỗi không
// chặn các loại khác; lỗi gộp lại để log. Công tắc chung tắt → không sinh gì.
func (s *Service) Refresh(ctx context.Context) error {
	on, err := s.enabled(ctx)
	if err != nil {
		return err
	}
	if !on {
		return nil
	}
	now := s.now()
	if err := s.Store.ExpireStale(ctx, now); err != nil {
		return err
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
	if s.Cleanup != nil {
		gen(KindCleanup, func() ([]Draft, error) { return s.cleanupDrafts(ctx) })
	}
	if err := s.Store.TrimOpen(ctx, MaxOpen, now); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// ---- Hành động ----------------------------------------------------------

type AcceptResult struct {
	OK      bool   `json:"ok"`
	Summary string `json:"summary"`
	Href    string `json:"href,omitempty"`
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

// Dismiss bỏ qua đề xuất: không hiện lại trong 7 ngày.
func (s *Service) Dismiss(ctx context.Context, id string) error {
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
	_, err = s.Store.Resolve(ctx, id, StateDismissed, &until, now)
	return err
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
