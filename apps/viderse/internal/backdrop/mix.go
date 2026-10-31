package backdrop

import (
	"context"
	"math"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// MixChannel là phần góp của một kênh vào ước lượng "Nền của một video".
type MixChannel struct {
	ChannelID   string
	Clips       int
	FreshVideos int
	Repeats     bool
}

// MixReport là câu trả lời cho thanh Nền của editor: bao nhiêu giây clip cố
// định, mỗi kênh góp bao nhiêu clip, còn thiếu bao nhiêu và có bị trần 40 clip
// không.
type MixReport struct {
	FixedSec   float64
	PerChannel []MixChannel
	LoopSec    float64
	Need       int
	Capped     bool
}

// MixReader đọc template + danh mục để tính ước lượng nền. Chỉ đọc — không
// ghi, không gọi mạng.
type MixReader struct {
	store Store
}

// NewMixReader dựng bộ đọc ước lượng nền trên store.
func NewMixReader(store Store) *MixReader {
	return &MixReader{store: store}
}

// Mix tính ước lượng cho một video dài lengthSec (0 = chỉ báo phần đã biết).
// Dùng đúng bộ chia của PickBackdropSources để số trên thanh khớp video làm ra.
func (m *MixReader) Mix(ctx context.Context, templateID string, lengthSec int) (MixReport, error) {
	rows, err := m.store.Templates(ctx)
	if err != nil {
		return MixReport{}, err
	}
	var row *TemplateRecipeRow
	for i := range rows {
		if rows[i].ID == templateID {
			row = &rows[i]
			break
		}
	}
	if row == nil {
		return MixReport{}, ErrorfRecovery(CodeNotFound, "Template không tồn tại", "Mở lại Thư viện để xem danh sách template")
	}
	backdropRecipe, speed := parseRecipeBackdrop(row.Recipe)
	if backdropRecipe == nil || len(backdropRecipe.Channels) == 0 {
		return MixReport{}, ErrorfRecovery(CodeNotFound, "Template chưa gắn kênh nền nào", "Gắn một kênh nền trước khi xem ước lượng")
	}
	if speed <= 0 {
		speed = 1
	}
	clipSec := float64(backdropRecipe.ClipSec)
	if clipSec <= 0 {
		clipSec = 60
	}
	fixedSec, err := m.fixedSeconds(ctx, backdropRecipe.FixedVideoIDs, speed)
	if err != nil {
		return MixReport{}, err
	}
	clipPlaybackSec := clipSec / speed
	report := MixReport{FixedSec: fixedSec, PerChannel: make([]MixChannel, 0, len(backdropRecipe.Channels))}
	target := float64(lengthSec) - fixedSec
	required := 0
	if lengthSec > 0 && clipPlaybackSec > 0 {
		if target > 0 {
			required = int(math.Ceil(target / clipPlaybackSec))
		}
	}
	if backdropRecipe.ClipCount != nil {
		required = *backdropRecipe.ClipCount
	}
	report.Capped = required > MaxBackdropPicks
	need := required
	if need > MaxBackdropPicks {
		need = MaxBackdropPicks
	}
	if need < 0 {
		need = 0
	}
	report.Need = need
	candidates, err := m.candidates(ctx, backdropRecipe)
	if err != nil {
		return MixReport{}, err
	}
	for _, channel := range backdropRecipe.Channels {
		fresh := len(EligibleCandidates(channel, candidates[channel.ChannelID]))
		report.PerChannel = append(report.PerChannel, MixChannel{ChannelID: channel.ChannelID, FreshVideos: fresh})
	}
	if need > 0 {
		picks := PickBackdropSources(PickRequest{
			Channels:   backdropRecipe.Channels,
			Candidates: candidates,
			Seed:       0,
			Count:      need,
			ClipSec:    clipSec,
		})
		seen := make(map[string]bool, len(picks))
		perChannel := make(map[string]*MixChannel, len(report.PerChannel))
		for i := range report.PerChannel {
			perChannel[report.PerChannel[i].ChannelID] = &report.PerChannel[i]
		}
		played := 0.0
		for _, pick := range picks {
			if self := perChannel[pick.ChannelID]; self != nil {
				self.Clips++
				if seen[pick.VideoID] {
					self.Repeats = true
				}
			}
			seen[pick.VideoID] = true
			if pick.Cut != nil {
				played += (pick.Cut.EndSec - pick.Cut.StartSec) / speed
			} else {
				// Không biết độ dài nguồn: ước theo clip cấu hình, không coi là
				// trọn video.
				played += clipPlaybackSec
			}
		}
		report.LoopSec = math.Max(0, target-played)
	} else {
		report.LoopSec = math.Max(0, target)
	}
	return report, nil
}

// fixedSeconds tính số giây phát của các video lẻ ghim, chia tốc độ chung.
func (m *MixReader) fixedSeconds(ctx context.Context, ids []string, speed float64) (float64, error) {
	total := 0.0
	for _, id := range ids {
		row, found, err := m.store.Video(ctx, id)
		if err != nil {
			return 0, err
		}
		if !found || row.State != domain.BackdropReady {
			continue
		}
		total += row.Segment.Duration() / speed
	}
	return total, nil
}

// candidates nạp danh mục đủ điều kiện của từng kênh đã gắn.
func (m *MixReader) candidates(ctx context.Context, recipe *domain.RecipeBackdrop) (map[string][]Candidate, error) {
	out := make(map[string][]Candidate, len(recipe.Channels))
	for _, channel := range recipe.Channels {
		rows, err := m.store.ChannelVideos(ctx, channel.ChannelID)
		if err != nil {
			return nil, err
		}
		list := make([]Candidate, 0, len(rows))
		for _, row := range rows {
			duration := row.DurationSec
			list = append(list, Candidate{
				ChannelID: row.ChannelID, VideoID: row.VideoID, Kind: row.Kind, URL: row.URL,
				Title: new(row.Title), DurationSec: duration, Unavailable: row.UnavailableAt != nil,
			})
		}
		out[channel.ChannelID] = list
	}
	return out, nil
}

// Picker dựng bối cảnh chọn nguồn cho một lần dựng video: danh mục thật của
// các kênh đã gắn, không có mạng. Lỗi thiếu kênh được bỏ qua (kênh đã bị xoá
// thì chỉ còn các kênh khác).
func (s *Service) Picker(ctx context.Context, recipe *domain.RecipeBackdrop) (map[string][]Candidate, error) {
	return (&MixReader{store: s.d.Store}).candidates(ctx, recipe)
}

// DescribeLink trả số video dùng được của một kênh theo bộ lọc của template —
// dùng cho màn template hiện "còn N video".
func (s *Service) DescribeLink(ctx context.Context, channelID string, kinds []domain.PlatformKind, pinned []string) (int, error) {
	rows, err := s.d.Store.ChannelVideos(ctx, channelID)
	if err != nil {
		return 0, err
	}
	kindSet := make(map[string]bool, len(kinds))
	for _, kind := range kinds {
		kindSet[kind] = true
	}
	pinSet := make(map[string]bool, len(pinned))
	for _, id := range pinned {
		pinSet[id] = true
	}
	count := 0
	for _, row := range rows {
		if len(kindSet) > 0 && !kindSet[row.Kind] {
			continue
		}
		if row.UnavailableAt != nil {
			continue
		}
		if len(pinSet) > 0 && !pinSet[row.VideoID] {
			continue
		}
		count++
	}
	return count, nil
}
