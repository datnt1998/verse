package backdrop

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// recipeBackdropDoc là hình recipe tối thiểu mà mảng Video nền đọc: phần nền
// và tốc độ phát chung (recipe.appearance.speed).
type recipeBackdropDoc struct {
	Backdrop   *domain.RecipeBackdrop `json:"backdrop"`
	Appearance struct {
		Speed *float64 `json:"speed"`
	} `json:"appearance"`
}

// parseRecipeBackdrop đọc recipe của một template. Recipe hỏng hoặc thiếu
// backdrop trả nil — template đó không gắn nền, như mảng khác xử lý.
func parseRecipeBackdrop(recipe []byte) (*domain.RecipeBackdrop, float64) {
	if len(recipe) == 0 {
		return nil, 1
	}
	var doc recipeBackdropDoc
	if err := json.Unmarshal(recipe, &doc); err != nil {
		return nil, 1
	}
	speed := 1.0
	if doc.Appearance.Speed != nil && *doc.Appearance.Speed > 0 {
		speed = *doc.Appearance.Speed
	}
	return doc.Backdrop, speed
}

// linkedChannel trả dòng "Lấy nền từ" của một kênh trong recipe.
func linkedChannel(backdrop *domain.RecipeBackdrop, channelID string) *domain.RecipeBackdropChannel {
	if backdrop == nil {
		return nil
	}
	for i := range backdrop.Channels {
		if backdrop.Channels[i].ChannelID == channelID {
			return &backdrop.Channels[i]
		}
	}
	return nil
}

// templateUsageIndex đọc recipe MỘT lần cho cả danh sách: tránh parse lặp theo
// từng kênh/video (N+1) khi dựng usedBy/chosenBy.
type templateUsageIndex struct {
	channels map[string][]TemplateUsage
	chosen   map[string]map[string][]TemplateRef
	fixed    map[string][]TemplateRef
}

func buildUsageIndex(rows []TemplateRecipeRow) *templateUsageIndex {
	idx := &templateUsageIndex{
		channels: make(map[string][]TemplateUsage, len(rows)),
		chosen:   make(map[string]map[string][]TemplateRef),
		fixed:    make(map[string][]TemplateRef),
	}
	for _, row := range rows {
		backdrop, _ := parseRecipeBackdrop(row.Recipe)
		if backdrop == nil {
			continue
		}
		for _, link := range backdrop.Channels {
			mode := "random"
			if len(link.Pinned) > 0 {
				mode = "pick"
			}
			idx.channels[link.ChannelID] = append(idx.channels[link.ChannelID], TemplateUsage{
				TemplateID: row.ID, Name: row.Name, Mode: mode,
				Kinds: append([]domain.PlatformKind(nil), link.Kinds...), ChosenCount: len(link.Pinned),
			})
			if len(link.Pinned) > 0 {
				perChannel := idx.chosen[link.ChannelID]
				if perChannel == nil {
					perChannel = make(map[string][]TemplateRef)
					idx.chosen[link.ChannelID] = perChannel
				}
				for _, videoID := range link.Pinned {
					perChannel[videoID] = append(perChannel[videoID], TemplateRef{TemplateID: row.ID, Name: row.Name})
				}
			}
		}
		for _, id := range backdrop.FixedVideoIDs {
			idx.fixed[id] = append(idx.fixed[id], TemplateRef{TemplateID: row.ID, Name: row.Name})
		}
	}
	for channelID := range idx.channels {
		sortTemplateUsage(idx.channels[channelID])
	}
	for channelID := range idx.chosen {
		for videoID := range idx.chosen[channelID] {
			sortTemplateRefs(idx.chosen[channelID][videoID])
		}
	}
	for videoID := range idx.fixed {
		sortTemplateRefs(idx.fixed[videoID])
	}
	return idx
}

func sortTemplateUsage(items []TemplateUsage) {
	sort.SliceStable(items, func(i, j int) bool { return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name) })
}

func sortTemplateRefs(items []TemplateRef) {
	sort.SliceStable(items, func(i, j int) bool { return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name) })
}

func (idx *templateUsageIndex) channelUsages(channelID string) []TemplateUsage {
	return append([]TemplateUsage(nil), idx.channels[channelID]...)
}

func (idx *templateUsageIndex) channelChosen(channelID string) map[string][]TemplateRef {
	return idx.chosen[channelID]
}

func (idx *templateUsageIndex) fixedUsages(videoID string) []TemplateRef {
	return append([]TemplateRef(nil), idx.fixed[videoID]...)
}

// usageOfChannel liệt kê template đang gắn một kênh nền, A→Z theo tên.
func usageOfChannel(templates []TemplateRecipeRow, channelID string) []TemplateUsage {
	out := make([]TemplateUsage, 0, 2)
	for _, row := range templates {
		backdrop, _ := parseRecipeBackdrop(row.Recipe)
		link := linkedChannel(backdrop, channelID)
		if link == nil {
			continue
		}
		mode := "random"
		if len(link.Pinned) > 0 {
			mode = "pick"
		}
		out = append(out, TemplateUsage{
			TemplateID:  row.ID,
			Name:        row.Name,
			Mode:        mode,
			Kinds:       append([]domain.PlatformKind(nil), link.Kinds...),
			ChosenCount: len(link.Pinned),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// chosenByChannel gom template ghim từng video của một kênh: mã video → danh
// sách template ghim nó.
func chosenByChannel(templates []TemplateRecipeRow, channelID string) map[string][]TemplateRef {
	out := make(map[string][]TemplateRef)
	for _, row := range templates {
		backdrop, _ := parseRecipeBackdrop(row.Recipe)
		link := linkedChannel(backdrop, channelID)
		if link == nil {
			continue
		}
		for _, videoID := range link.Pinned {
			out[videoID] = append(out[videoID], TemplateRef{TemplateID: row.ID, Name: row.Name})
		}
	}
	return out
}

// usageOfFixedVideo liệt kê template ghim một video lẻ trong fixedVideoIds.
func usageOfFixedVideo(templates []TemplateRecipeRow, videoID string) []TemplateRef {
	out := make([]TemplateRef, 0, 1)
	for _, row := range templates {
		backdrop, _ := parseRecipeBackdrop(row.Recipe)
		if backdrop == nil {
			continue
		}
		for _, id := range backdrop.FixedVideoIDs {
			if id == videoID {
				out = append(out, TemplateRef{TemplateID: row.ID, Name: row.Name})
				break
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// templatesUsingChannel trả template đang gắn kênh (dùng để chặn xoá).
func templatesUsingChannel(templates []TemplateRecipeRow, channelID string) []TemplateRef {
	usage := usageOfChannel(templates, channelID)
	out := make([]TemplateRef, 0, len(usage))
	for _, u := range usage {
		out = append(out, TemplateRef{TemplateID: u.TemplateID, Name: u.Name})
	}
	return out
}
