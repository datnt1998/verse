package backdrop

import (
	"math"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// MinBackdropClipSec là sàn độ dài nguồn: ngắn hơn thì làm nền vô nghĩa nên
// không bao giờ được chọn.
const MinBackdropClipSec = 10.0

// MaxBackdropPicks là trần số clip một video dùng (khớp thanh Nền của editor).
const MaxBackdropPicks = 40

// Mulberry32 là bộ sinh số giả ngẫu nhiên có hạt, chép nguyên thuật toán
// mulberry32 của template-fill.ts để cùng hạt cho cùng dãy số.
func Mulberry32(seed uint32) func() float64 {
	a := seed
	return func() float64 {
		a += 0x6d2b79f5
		t := (a ^ (a >> 15)) * (a | 1)
		t = (t + (t^(t>>7))*(t|61)) ^ t
		return float64(t^(t>>14)) / 4294967296
	}
}

// Cut là cửa sổ nguồn sẽ tải về cho một clip nền.
type Cut struct {
	StartSec float64
	EndSec   float64
}

// CutRange trả cửa sổ để tải từ nguồn dài durationSec, hoặc nil khi nguồn ngắn
// hơn sàn. Nguồn đủ dài clipSec nhận một cửa sổ ngẫu nhiên clipSec; nguồn ngắn
// hơn được dùng trọn.
func CutRange(durationSec, clipSec float64, rand func() float64) *Cut {
	if durationSec < MinBackdropClipSec {
		return nil
	}
	if durationSec < clipSec {
		return &Cut{StartSec: 0, EndSec: durationSec}
	}
	start := math.Floor(rand() * (math.Floor(durationSec-clipSec) + 1))
	return &Cut{StartSec: start, EndSec: start + clipSec}
}

// Candidate là một video trong danh mục đủ điều kiện làm nguồn nền.
type Candidate struct {
	ChannelID   string
	VideoID     string
	Kind        domain.PlatformKind
	URL         string
	Title       *string
	DurationSec *float64
	Unavailable bool
}

// Pick là một nguồn đã chọn. Cut nil đúng khi DurationSec nil: phải dò độ dài
// rồi mới gọi CutRange.
type Pick struct {
	Candidate
	Cut *Cut
}

// PickRequest là đầu vào thuần của bộ chọn: không DB, không I/O, không đồng hồ.
type PickRequest struct {
	// Channels là các dòng "Lấy nền từ" của template, theo thứ tự khai báo.
	Channels []domain.RecipeBackdropChannel
	// Candidates là danh mục theo mã kênh; kênh không có mục thì không có video.
	Candidates map[string][]Candidate
	Seed       uint32
	Count      int
	// UsedVideoIDs là các mã đã dùng trong video này (mã là duy nhất toàn cục).
	UsedVideoIDs map[string]bool
	// ExcludedVideoIDs không bao giờ được chọn lại (ví dụ video tải hỏng).
	ExcludedVideoIDs map[string]bool
	// SkippedChannelIDs bỏ qua (ví dụ nền tảng đang bị tạm chặn).
	SkippedChannelIDs map[string]bool
	// ClipSec là độ dài nguồn cấu hình cho mỗi clip.
	ClipSec float64
}

// EligibleCandidates trả đúng tập video bộ chọn sẽ xét cho một dòng kênh:
// đúng loại, còn dùng được, không bị loại, nằm trong pins (nếu có) và không
// ngắn hơn sàn clip. Mix dùng hàm này để đếm "còn N video" khớp bộ chọn.
func EligibleCandidates(channel domain.RecipeBackdropChannel, source []Candidate) []Candidate {
	return eligibleCandidates(channel, source, nil)
}

func eligibleCandidates(channel domain.RecipeBackdropChannel, source []Candidate, excluded map[string]bool) []Candidate {
	kinds := make(map[string]bool, len(channel.Kinds))
	for _, kind := range channel.Kinds {
		kinds[kind] = true
	}
	pins := make(map[string]bool, len(channel.Pinned))
	for _, id := range channel.Pinned {
		pins[id] = true
	}
	pinned := len(channel.Pinned) > 0
	allowed := make([]Candidate, 0, len(source))
	for _, video := range source {
		if !kinds[video.Kind] || video.Unavailable || excluded[video.VideoID] {
			continue
		}
		if pinned && !pins[video.VideoID] {
			continue
		}
		if video.DurationSec != nil && *video.DurationSec < MinBackdropClipSec {
			continue
		}
		allowed = append(allowed, video)
	}
	return allowed
}

func shuffled[T any](items []T, rand func() float64) []T {
	out := append([]T(nil), items...)
	for i := len(out) - 1; i > 0; i-- {
		j := int(math.Floor(rand() * float64(i+1)))
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// PickBackdropSources chọn tối đa Count nguồn, vòng tròn trên các kênh đã xáo:
// kênh xếp sau nhường kênh trước, video chưa dùng được ưu tiên, chỉ khi mọi
// kênh hết video mới dùng lại video cũ (có cửa sổ khác). Chép nguyên
// pickBackdropSources của pick.ts.
func PickBackdropSources(req PickRequest) []Pick {
	rand := Mulberry32(req.Seed)
	used := make(map[string]bool, len(req.UsedVideoIDs))
	for id := range req.UsedVideoIDs {
		used[id] = true
	}

	linked := make([]domain.RecipeBackdropChannel, 0, len(req.Channels))
	for _, channel := range req.Channels {
		if !req.SkippedChannelIDs[channel.ChannelID] {
			linked = append(linked, channel)
		}
	}
	pools := make([][]Candidate, 0, len(linked))
	for _, channel := range shuffled(linked, rand) {
		pools = append(pools, eligibleCandidates(channel, req.Candidates[channel.ChannelID], req.ExcludedVideoIDs))
	}

	// Video dùng lại cần chỗ cho một cửa sổ khác; clip ngắn dùng trọn sẽ chỉ lặp.
	reusable := func(video Candidate) bool {
		return video.DurationSec == nil || *video.DurationSec > req.ClipSec
	}
	fresh := func(video Candidate) bool { return !used[video.VideoID] }

	picks := make([]Pick, 0, req.Count)
	cursor := 0
	for len(picks) < req.Count {
		type choice struct {
			options []Candidate
			index   int
		}
		var chosen *choice
		for _, allow := range []func(Candidate) bool{fresh, reusable} {
			for step := 0; step < len(pools) && chosen == nil; step++ {
				index := (cursor + step) % len(pools)
				options := make([]Candidate, 0, len(pools[index]))
				for _, video := range pools[index] {
					if allow(video) {
						options = append(options, video)
					}
				}
				if len(options) > 0 {
					chosen = &choice{options: options, index: index}
				}
			}
			if chosen != nil {
				break
			}
		}
		if chosen == nil {
			break
		}
		video := chosen.options[int(math.Floor(rand()*float64(len(chosen.options))))]
		used[video.VideoID] = true
		cursor = chosen.index + 1
		pick := Pick{Candidate: video}
		if video.DurationSec != nil {
			pick.Cut = CutRange(*video.DurationSec, req.ClipSec, rand)
		}
		picks = append(picks, pick)
	}
	return picks
}
