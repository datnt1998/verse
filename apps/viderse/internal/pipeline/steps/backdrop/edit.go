package backdrop

import (
	"context"
	"errors"
	"fmt"
	"math"

	catalogue "github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

type VideoChoice struct {
	ChannelID string   `json:"channelId"`
	VideoID   string   `json:"videoId"`
	StartSec  *float64 `json:"startSec,omitempty"`
	EndSec    *float64 `json:"endSec,omitempty"`
}
type EditRequest struct {
	Action   string `json:"action"`
	ItemID   string `json:"itemId,omitempty"`
	Revision string `json:"revision,omitempty"`
	VideoChoice
}

// EditState is read inside the caller's job-edit transaction. Busy must include
// backdrop plus every step whose output this edit would invalidate.
type EditState struct {
	JobID      string
	Manifest   *Manifest
	Template   Template
	VideoSec   *float64
	Busy       bool
	Superseded bool
}
type EditResult struct {
	Manifest       Manifest
	InvalidateFrom domain.CloneStep
}
type Editor struct{ Deps Deps }

func conflict(message string) error {
	return &domain.CloneError{Code: domain.CloneErrConflict, Message: message}
}
func notFound(message string) error {
	return &domain.CloneError{Code: domain.CloneErrNotFound, Message: message}
}
func OwnerWindow(choice VideoChoice, duration *float64, clipSec int, salt uint32) (int, int, error) {
	if choice.StartSec != nil && choice.EndSec != nil {
		start, end := *choice.StartSec, *choice.EndSec
		if math.IsNaN(start) || math.IsNaN(end) || math.IsInf(start, 0) || math.IsInf(end, 0) || math.Trunc(start) != start || math.Trunc(end) != end || start < 0 || end <= start {
			return 0, 0, requestError("Khoảng cắt phải là số giây nguyên, \"Đến\" lớn hơn \"Từ\"")
		}
		if end-start < 10 || end-start > catalogue.MaxBackdropSegmentSec {
			return 0, 0, requestError("Khoảng cắt phải dài từ 10 giây đến 10 phút")
		}
		if duration != nil && end > math.Floor(*duration) {
			return 0, 0, requestError(fmt.Sprintf("Video chỉ dài %.0f giây — chọn khoảng cắt trong đó", math.Floor(*duration)))
		}
		return int(start), int(end), nil
	}
	if choice.StartSec != nil || choice.EndSec != nil {
		return 0, 0, requestError("Nhập cả \"Từ\" và \"Đến\" hoặc để trống cả hai")
	}
	if duration == nil {
		return 0, 0, requestError("Chưa biết độ dài video — nhập khoảng cắt \"Từ\" và \"Đến\"")
	}
	cut := catalogue.CutRange(*duration, float64(clipSec), catalogue.Mulberry32((salt+1)*0x9e3779b1))
	if cut == nil {
		return 0, 0, requestError("Video ngắn hơn 10 giây — chọn video khác")
	}
	return int(math.Floor(cut.StartSec)), int(math.Floor(cut.EndSec)), nil
}
func (e Editor) ownerItem(ctx context.Context, state EditState, m Manifest, choice VideoChoice, id string) (Item, error) {
	video, found, err := e.Deps.Store.ChannelVideo(ctx, choice.ChannelID, choice.VideoID)
	if err != nil {
		return Item{}, err
	}
	if !found {
		return Item{}, notFound("Video này không có trong thư viện kênh nền")
	}
	if video.UnavailableAt != nil {
		return Item{}, requestError("Video này không còn xem được — chọn video khác")
	}
	if video.DurationSec == nil {
		video, err = e.Deps.ProbeVideo(ctx, choice.ChannelID, choice.VideoID)
		if err != nil {
			if ctx.Err() != nil {
				return Item{}, ctx.Err()
			}
			f := failure(err)
			code := domain.CloneErrInvalidRequest
			recovery := probeRecovery(f.Code)
			if f.Code == "rate_limited" {
				code = domain.CloneErrQuota
			}
			return Item{}, &domain.CloneError{Code: code, Message: "Không đọc được video này: " + f.Message, Recovery: &recovery}
		}
	}
	clipSec := 60
	if state.Template.Backdrop != nil {
		clipSec = state.Template.Backdrop.ClipSec
	}
	start, end, err := OwnerWindow(choice, video.DurationSec, clipSec, m.Salt)
	if err != nil {
		return Item{}, err
	}
	title := video.Title
	return Item{ID: id, ChannelID: video.ChannelID, VideoID: video.VideoID, Kind: video.Kind, URL: video.URL, Title: &title, StartSec: start, EndSec: end, Source: "owner", Status: "pending"}, nil
}

func probeRecovery(code string) string {
	switch code {
	case "login_required":
		return "Cần tài khoản Google phụ đã đăng nhập YouTube: chọn hồ sơ trình duyệt trong Cài đặt → Cookies rồi thử lại."
	case "cookies_failed":
		return "Khi máy hỏi quyền truy cập, chọn “Luôn cho phép”, hoặc chọn hồ sơ khác trong Cài đặt → Cookies."
	case "unavailable":
		return "Video có thể là riêng tư, đã bị xoá hoặc bị chặn theo vùng. Hãy chọn video khác."
	case "no_subtitles":
		return "Video chưa có phụ đề để lấy tự động. Hãy dùng lời thoại từ nguồn khác."
	case "rate_limited":
		return "Nền tảng đang chặn tạm vì có quá nhiều yêu cầu. Đợi ít phút rồi thử lại."
	case "failed":
		return "Kiểm tra link và kết nối mạng, hoặc cập nhật công cụ tải trong Cài đặt rồi thử lại."
	default:
		return "Thử lại sau hoặc chọn video khác"
	}
}

// Edit computes one edit. The caller MUST atomically compare Revision again,
// store the returned checkpoint, invalidate from InvalidateFrom and launch.
// No manifest or queue state is mutated before that transaction commits.
func (e Editor) Edit(ctx context.Context, state EditState, input EditRequest) (EditResult, error) {
	if state.Superseded {
		return EditResult{}, notFound("Video đã được thay thế bởi lượt làm mới")
	}
	if state.Busy {
		return EditResult{}, conflict("Video đang xử lý nền hoặc dựng video")
	}
	if err := ctx.Err(); err != nil {
		return EditResult{}, err
	}
	m, _ := InitialManifest(state.Template, state.VideoSec)
	if state.Manifest != nil {
		m = *state.Manifest
		m.Items = append([]Item{}, m.Items...)
		m.Warnings = append([]string{}, m.Warnings...)
	}
	if input.Revision != "" && input.Revision != m.Revision {
		return EditResult{}, conflict("Danh sách clip nền đã thay đổi — tải lại rồi thử lại")
	}
	index := -1
	for i, v := range m.Items {
		if v.ID == input.ItemID {
			index = i
			break
		}
	}
	if input.Action == "remove" || input.Action == "swap" || input.Action == "reroll" && input.ItemID != "" {
		if index < 0 {
			return EditResult{}, notFound("Không tìm thấy clip nền " + input.ItemID)
		}
	}
	h := New(e.Deps)
	r := &runState{h: h, jobID: state.JobID, skipped: map[domain.PlatformID]bool{}}
	randomSource := func() (*domain.BackdropRecipe, error) {
		if state.Template.Backdrop == nil {
			return nil, requestError("Template chưa gắn kênh nền — chọn một video cụ thể")
		}
		return state.Template.Backdrop, nil
	}
	switch input.Action {
	case "remove":
		removed := m.Items[index]
		m.Items = append(m.Items[:index], m.Items[index+1:]...)
		m.WantedCount = max(0, m.WantedCount-1)
		if m.TargetSec != nil {
			n := max(0, min(*m.TargetSec-ItemSec(removed), PlannedSec(m.Items)))
			m.TargetSec = &n
		}
	case "swap":
		if input.ChannelID == "" || input.VideoID == "" {
			return EditResult{}, requestError("Chọn cả kênh và video")
		}
		item, err := e.ownerItem(ctx, state, m, input.VideoChoice, input.ItemID)
		if err != nil {
			return EditResult{}, err
		}
		m.Items[index] = item
	case "add":
		if len(m.Items) >= MaxItems || m.WantedCount >= MaxItems {
			return EditResult{}, requestError("Tối đa 40 clip nền")
		}
		var item Item
		if input.ChannelID != "" || input.VideoID != "" {
			if input.ChannelID == "" || input.VideoID == "" {
				return EditResult{}, requestError("Chọn cả kênh và video")
			}
			v, err := e.ownerItem(ctx, state, m, input.VideoChoice, NextItemID(m.Items))
			if err != nil {
				return EditResult{}, err
			}
			item = v
		} else {
			if input.StartSec != nil || input.EndSec != nil {
				return EditResult{}, requestError("Chọn video trước khi nhập khoảng cắt")
			}
			b, err := randomSource()
			if err != nil {
				return EditResult{}, err
			}
			if err := r.refresh(ctx, b); err != nil {
				return EditResult{}, err
			}
			fresh, salt, err := r.pick(ctx, b, m.Items, m.Salt, 1, nil)
			if err != nil {
				return EditResult{}, err
			}
			if len(fresh) == 0 {
				return EditResult{}, notFound("Không còn video khác để chọn từ các kênh nền")
			}
			item = fresh[0]
			m.Salt = salt
		}
		m.Items = append(m.Items, item)
		m.WantedCount++
		if m.TargetSec != nil {
			n := *m.TargetSec + ItemSec(item)
			m.TargetSec = &n
		}
	case "reroll":
		b, err := randomSource()
		if err != nil {
			return EditResult{}, err
		}
		kept := []Item{}
		replaced := []Item{}
		avoid := map[string]bool{}
		for _, v := range m.Items {
			if input.ItemID != "" && v.ID == input.ItemID || input.ItemID == "" && v.Source == "random" {
				replaced = append(replaced, v)
				avoid[v.VideoID] = true
			} else {
				kept = append(kept, v)
			}
		}
		count := m.WantedCount - len(kept)
		target := m.TargetSec
		if input.ItemID != "" {
			count = 1
			target = nil
		}
		if len(replaced) == 0 || target == nil && count < 1 {
			return EditResult{}, requestError("Không có clip ngẫu nhiên nào để lấy lại")
		}
		if err := r.refresh(ctx, b); err != nil {
			return EditResult{}, err
		}
		fresh, salt, err := r.pickGoal(ctx, b, kept, m.Salt, count, target, avoid)
		if err != nil {
			return EditResult{}, err
		}
		if len(fresh) == 0 {
			return EditResult{}, notFound("Không còn video khác để chọn từ các kênh nền")
		}
		m.Salt = salt
		if input.ItemID != "" {
			fresh[0].ID = input.ItemID
			m.Items[index] = fresh[0]
		} else {
			m.Items = append(kept, fresh...)
			if m.TargetSec != nil {
				m.WantedCount = len(m.Items)
			}
		}
	default:
		return EditResult{}, requestError("Thao tác clip nền không hợp lệ")
	}
	if len(ReadyItems(m)) > 0 {
		m.Fallback = "none"
	}
	m.Warnings = ShortfallWarnings(m.Warnings, m.Items, m.WantedCount, m.TargetSec)
	m = Seal(m)
	invalidate := domain.CloneBuild
	for _, v := range m.Items {
		if v.Status == "pending" {
			invalidate = domain.CloneBackdrop
			break
		}
	}
	return EditResult{Manifest: m, InvalidateFrom: invalidate}, nil
}

type Coverage struct {
	Mode       string   `json:"mode"`
	VideoSec   *float64 `json:"videoSec"`
	FixedSec   float64  `json:"fixedSec"`
	ChannelSec float64  `json:"channelSec"`
}
type EvidenceItem struct {
	Item
	EditorMediaID    string             `json:"editorMediaId,omitempty"`
	Platform         *domain.PlatformID `json:"platform"`
	ChannelHandle    *string            `json:"channelHandle"`
	ChannelTitle     *string            `json:"channelTitle"`
	ThumbnailURL     *string            `json:"thumbnailUrl"`
	VideoDurationSec *float64           `json:"videoDurationSec"`
}
type EvidenceView struct {
	Revision        string         `json:"revision"`
	WantedCount     int            `json:"wantedCount"`
	ReadyCount      int            `json:"readyCount"`
	Fallback        string         `json:"fallback"`
	Warnings        []string       `json:"warnings"`
	RandomPicks     bool           `json:"randomPicks"`
	Busy            bool           `json:"busy"`
	Coverage        Coverage       `json:"coverage"`
	Items           []EvidenceItem `json:"items"`
	Rebuilding      bool           `json:"rebuilding"`
	ProjectRevision *string        `json:"projectRevision"`
}
type EvidenceInput struct {
	Manifest         *Manifest
	TemplateSnapshot []byte
	VideoSec         *float64
	Busy             bool
}

// Evidence reads only stored catalogue metadata. A missing snapshot retains
// identity playback coverage for already-picked windows.
func Evidence(ctx context.Context, store Store, input EvidenceInput) (*EvidenceView, error) {
	t, err := ResolveTemplate(ctx, input.TemplateSnapshot, store)
	if err != nil {
		var e *domain.CloneError
		if !errors.As(err, &e) {
			return nil, err
		}
		t = Template{PlaybackSpeed: 1}
	}
	if input.Manifest == nil && t.Backdrop == nil {
		return nil, nil
	}
	m := Empty(0, nil)
	if input.Manifest != nil {
		m = *input.Manifest
	} else if t.Backdrop.ClipCount != nil {
		m = Empty(*t.Backdrop.ClipCount, nil)
	}
	mode := "count"
	if input.Manifest != nil && m.TargetSec != nil || input.Manifest == nil && t.Backdrop != nil && t.Backdrop.ClipCount == nil {
		mode = "fill"
	}
	view := &EvidenceView{Revision: m.Revision, WantedCount: m.WantedCount, ReadyCount: len(ReadyItems(m)), Fallback: m.Fallback, Warnings: m.Warnings, RandomPicks: t.Backdrop != nil, Busy: input.Busy, Coverage: Coverage{Mode: mode, VideoSec: input.VideoSec, FixedSec: t.FixedSec}, Items: []EvidenceItem{}}
	channels := map[string]domain.BackdropChannel{}
	loaded := map[string]bool{}
	for _, v := range m.Items {
		if !loaded[v.ChannelID] {
			row, found, err := store.Channel(ctx, v.ChannelID)
			if err != nil {
				return nil, err
			}
			if found {
				channels[v.ChannelID] = row
			}
			loaded[v.ChannelID] = true
		}
		item := EvidenceItem{Item: v}
		if channel, ok := channels[v.ChannelID]; ok {
			p := channel.Platform
			handle := channel.Handle
			item.Platform = &p
			item.ChannelHandle = &handle
			item.ChannelTitle = channel.Title
		}
		video, found, err := store.ChannelVideo(ctx, v.ChannelID, v.VideoID)
		if err != nil {
			return nil, err
		}
		if found {
			item.ThumbnailURL = video.ThumbnailURL
			item.VideoDurationSec = video.DurationSec
		}
		view.Items = append(view.Items, item)
		if v.Status == "ready" && v.MediaID != nil {
			view.Coverage.ChannelSec += float64(ItemSec(v)) / t.PlaybackSpeed
		}
	}
	return view, nil
}
