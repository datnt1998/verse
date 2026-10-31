package backdrop_test

import (
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

func candidate(channelID, videoID string, duration float64) backdrop.Candidate {
	return backdrop.Candidate{
		ChannelID: channelID, VideoID: videoID, Kind: domain.PlatformKindVod,
		URL: "https://example.test/" + videoID, DurationSec: &duration,
	}
}

// mulberry32 phải cho cùng dãy với bản TS (hạt 42).
func TestMulberry32Deterministic(t *testing.T) {
	first := backdrop.Mulberry32(42)
	second := backdrop.Mulberry32(42)
	for range 5 {
		if a, b := first(), second(); a != b {
			t.Fatalf("cùng hạt phải cùng dãy: %v != %v", a, b)
		}
	}
	// Dãy tham chiếu của mulberry32(1) (JS): 0.6270739405881613, 0.002735721180215478, 0.5274470399599522.
	values := []float64{0.6270739405881613, 0.002735721180215478, 0.5274470399599522}
	gen := backdrop.Mulberry32(1)
	for i, want := range values {
		if got := gen(); got != want {
			t.Fatalf("value[%d] = %v, muốn %v", i, got, want)
		}
	}
}

func TestCutRange(t *testing.T) {
	if cut := backdrop.CutRange(9, 60, func() float64 { return 0.5 }); cut != nil {
		t.Fatalf("nguồn < 10 giây phải bị bỏ, được %+v", cut)
	}
	short := backdrop.CutRange(30, 60, func() float64 { return 0.9 })
	if short == nil || short.StartSec != 0 || short.EndSec != 30 {
		t.Fatalf("nguồn ngắn hơn clip phải dùng trọn: %+v", short)
	}
	cut := backdrop.CutRange(100, 60, func() float64 { return 0.5 })
	if cut == nil || cut.StartSec != 20 || cut.EndSec != 80 {
		t.Fatalf("cửa sổ = %+v, muốn 20..80", cut)
	}
	edge := backdrop.CutRange(100, 60, func() float64 { return 0.999 })
	if edge.StartSec != 40 || edge.EndSec != 100 {
		t.Fatalf("cửa sổ mép = %+v, muốn 40..100", edge)
	}
	unknown := backdrop.Candidate{ChannelID: "c", VideoID: "v", Kind: domain.PlatformKindVod}
	picks := backdrop.PickBackdropSources(backdrop.PickRequest{
		Channels:   []domain.RecipeBackdropChannel{{ChannelID: "c", Kinds: []string{"vod"}}},
		Candidates: map[string][]backdrop.Candidate{"c": {unknown}},
		Count:      1, ClipSec: 60,
	})
	if len(picks) != 1 || picks[0].Cut != nil {
		t.Fatalf("độ dài chưa biết thì chưa cắt được: %+v", picks)
	}
}

func TestPickRoundRobinAcrossChannels(t *testing.T) {
	channels := []domain.RecipeBackdropChannel{
		{ChannelID: "a", Kinds: []string{"vod"}},
		{ChannelID: "b", Kinds: []string{"vod"}},
	}
	candidates := map[string][]backdrop.Candidate{
		"a": {candidate("a", "a1", 600)},
		"b": {candidate("b", "b1", 600)},
	}
	picks := backdrop.PickBackdropSources(backdrop.PickRequest{Channels: channels, Candidates: candidates, Count: 4, ClipSec: 60})
	if len(picks) != 4 {
		t.Fatalf("picks = %+v", picks)
	}
	// Thứ tự kênh do hạt quyết định; luật là xen kẽ a→b→a→b hoặc b→a→b→a.
	for i := 1; i < len(picks); i++ {
		if picks[i].ChannelID == picks[i-1].ChannelID {
			t.Fatalf("phải xen kẽ kênh: %+v", picks)
		}
	}
	if picks[0].ChannelID != picks[2].ChannelID || picks[1].ChannelID != picks[3].ChannelID {
		t.Fatalf("vòng tròn phải lặp lại thứ tự: %+v", picks)
	}
}

func TestPickFreshBeforeReuse(t *testing.T) {
	channels := []domain.RecipeBackdropChannel{{ChannelID: "a", Kinds: []string{"vod"}}}
	candidates := map[string][]backdrop.Candidate{
		"a": {candidate("a", "a1", 600), candidate("a", "a2", 600)},
	}
	picks := backdrop.PickBackdropSources(backdrop.PickRequest{Channels: channels, Candidates: candidates, Count: 3, ClipSec: 60})
	seen := map[string]bool{}
	for _, pick := range picks[:2] {
		if seen[pick.VideoID] {
			t.Fatalf("hai lượt đầu phải là hai video khác nhau: %+v", picks)
		}
		seen[pick.VideoID] = true
	}
	if len(picks) != 3 {
		t.Fatalf("lượt thứ ba dùng lại được video dài: %+v", picks)
	}
}

func TestPickStopsWhenOnlyShortVideosRemain(t *testing.T) {
	channels := []domain.RecipeBackdropChannel{{ChannelID: "a", Kinds: []string{"vod"}}}
	candidates := map[string][]backdrop.Candidate{"a": {candidate("a", "short", 45)}}
	picks := backdrop.PickBackdropSources(backdrop.PickRequest{Channels: channels, Candidates: candidates, Count: 5, ClipSec: 60})
	if len(picks) != 1 {
		t.Fatalf("clip ngắn dùng trọn không lặp: %+v", picks)
	}
}

func TestPickRespectsPinsKindsAndSkips(t *testing.T) {
	channels := []domain.RecipeBackdropChannel{
		{ChannelID: "a", Kinds: []string{"clip"}, Pinned: []string{"a2"}},
		{ChannelID: "b", Kinds: []string{"vod"}},
		{ChannelID: "c", Kinds: []string{"vod"}},
	}
	candidates := map[string][]backdrop.Candidate{
		"a": {candidate("a", "a1", 600), candidate("a", "a2", 600)},
		"b": {candidate("b", "b1", 600)},
		"c": {candidate("c", "c1", 600)},
	}
	candidates["a"][0].Kind = domain.PlatformKindClip
	candidates["a"][1].Kind = domain.PlatformKindClip
	picks := backdrop.PickBackdropSources(backdrop.PickRequest{
		Channels: channels, Candidates: candidates, Count: 3, ClipSec: 60,
		SkippedChannelIDs: map[string]bool{"b": true},
	})
	if len(picks) < 2 {
		t.Fatalf("picks = %+v", picks)
	}
	used := map[string]bool{}
	for _, pick := range picks {
		if pick.ChannelID == "b" {
			t.Fatal("kênh bị bỏ qua vẫn được chọn")
		}
		if pick.ChannelID == "a" && pick.VideoID != "a2" {
			t.Fatalf("kênh ghim phải chỉ chọn video ghim, được %q", pick.VideoID)
		}
		if pick.ChannelID == "a" {
			used["a"] = true
		}
		if pick.ChannelID == "c" {
			used["c"] = true
		}
	}
	if !used["a"] || !used["c"] {
		t.Fatalf("phải chọn từ cả hai kênh còn lại: %+v", picks)
	}
}

func TestPickHonorsExcludedAndUsed(t *testing.T) {
	channels := []domain.RecipeBackdropChannel{{ChannelID: "a", Kinds: []string{"vod"}}}
	candidates := map[string][]backdrop.Candidate{"a": {
		candidate("a", "used", 600), candidate("a", "failed", 600), candidate("a", "ok", 600),
	}}
	picks := backdrop.PickBackdropSources(backdrop.PickRequest{
		Channels: channels, Candidates: candidates, Count: 1, ClipSec: 60,
		UsedVideoIDs:     map[string]bool{"used": true},
		ExcludedVideoIDs: map[string]bool{"failed": true},
	})
	if len(picks) != 1 || picks[0].VideoID != "ok" {
		t.Fatalf("picks = %+v, muốn ok", picks)
	}
}
