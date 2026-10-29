package exp

import (
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
)

func TestStripChapterTitleHeader(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		title string
		want  string
	}{
		{"plain body untouched", "他望着窗外。", "雨夜归人", "他望着窗外。"},
		{"strip h1 chinese title", "# 第 1 章  雨夜归人\n\n他望着窗外。", "雨夜归人", "他望着窗外。"},
		{"strip h2 with chapter token", "## 第二章\n\n他望着窗外。", "", "他望着窗外。"},
		{"keep body even if no header", "Câu đầu tiên của chính văn.\nCâu thứ hai.", "", "Câu đầu tiên của chính văn.\nCâu thứ hai."},
		{"do not strip non-chapter heading", "# 序章\n他望着窗外。", "Biên Thôn Phù Sinh", "# 序章\n他望着窗外。"},
		{"single line header only", "# 第 1 章", "", ""},
		// writer 把纯章节名当标题写进首行 → 与导出器统一标题重复，应剥掉
		{"strip h1 matching chapter title", "# Biên Thôn Phù Sinh\n\nTrời còn chưa sáng.", "Biên Thôn Phù Sinh", "Trời còn chưa sáng."},
		// 首行 h1 但文字不等于本章标题 → 视为正文，保留
		{"keep h1 not matching title", "# Tiêu đề phụ khác\nChính văn.", "Biên Thôn Phù Sinh", "# Tiêu đề phụ khác\nChính văn."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stripChapterTitleHeader(c.in, c.title)
			if got != c.want {
				t.Fatalf("stripChapterTitleHeader\nin   = %q\ntitle= %q\nwant = %q\ngot  = %q", c.in, c.title, c.want, got)
			}
		})
	}
}

func TestBuildTitleIndex(t *testing.T) {
	outline := []domain.OutlineEntry{
		{Chapter: 1, Title: "雨夜归人"},
		{Chapter: 2, Title: ""}, // 空标题应被过滤
		{Chapter: 3, Title: "Phá Hiểu"},
	}
	idx := buildTitleIndex(outline)
	if got := idx[1]; got != "雨夜归人" {
		t.Errorf("tiêu đề ch1: nhận %q, mong đợi 雨夜归人", got)
	}
	if _, ok := idx[2]; ok {
		t.Errorf("ch2 should be absent (empty title)")
	}
	if got := idx[3]; got != "Phá Hiểu" {
		t.Errorf("tiêu đề ch3: nhận %q, mong đợi Phá Hiểu", got)
	}
}

func TestBuildLocations(t *testing.T) {
	volumes := []domain.VolumeOutline{
		{Index: 1, Title: "Khởi Nguyên", Arcs: []domain.ArcOutline{
			{Index: 1, Title: "Thiếu niên lần đầu xuất hiện", Chapters: []domain.OutlineEntry{{}, {}}}, // 2 章
			{Index: 2, Title: "Thí luyện tông môn", Chapters: []domain.OutlineEntry{{}}},               // 1 章
		}},
		{Index: 2, Title: "Quật Khởi", Arcs: []domain.ArcOutline{
			{Index: 1, Title: "Trận đầu", Chapters: []domain.OutlineEntry{{}}},
		}},
	}
	locs := buildLocations(volumes)

	// 只验卷归属：弧不再进 location，但弧层仍参与全局章号累加。
	if loc := locs[1]; !loc.IsFirstOfVolume || loc.VolumeIdx != 1 {
		t.Errorf("ch1 should be first of volume 1: %+v", loc)
	}
	if loc := locs[2]; loc.IsFirstOfVolume || loc.VolumeIdx != 1 {
		t.Errorf("ch2 should be volume 1, not first: %+v", loc)
	}
	// ch3 是弧 2 的首章，但仍在卷 1 内 → 不是卷首。
	if loc := locs[3]; loc.IsFirstOfVolume || loc.VolumeIdx != 1 {
		t.Errorf("ch3 (arc 2, same volume) should not be first of volume: %+v", loc)
	}
	if loc := locs[4]; !loc.IsFirstOfVolume || loc.VolumeIdx != 2 {
		t.Errorf("ch4 should start volume 2: %+v", loc)
	}
}

func TestRenderTXT_TitleAndChapter(t *testing.T) {
	got := renderTXT(
		"Đốm Sáng",
		[]int{1, 2},
		chapterTitleIndex{1: "Đêm mưa về quê", 2: "Bình minh"},
		nil,
		map[int]string{
			1: "# Chương 1 Đêm mưa về quê\n\nAnh nhìn ra cửa sổ.",
			2: "Chị đẩy cửa bước vào.",
		},
		exportLabels,
	)
	if !strings.HasPrefix(got, "Đốm Sáng\n\n") {
		t.Errorf("missing book title at start:\n%s", got)
	}
	// premise 不进导出：书名后应直接是章节，不夹任何前情提要
	if !strings.Contains(got, "Chương 1  Đêm mưa về quê") {
		t.Errorf("missing ch1 header")
	}
	if !strings.Contains(got, "Anh nhìn ra cửa sổ.") {
		t.Errorf("missing ch1 body")
	}
	if strings.Contains(got, "# Chương 1") {
		t.Errorf("body markdown header not stripped:\n%s", got)
	}
	if !strings.Contains(got, "Chương 2  Bình minh") {
		t.Errorf("missing ch2 header")
	}
}

func TestRenderTXT_EmptyBookTitleNoTitleLine(t *testing.T) {
	got := renderTXT(
		"",
		[]int{1},
		chapterTitleIndex{1: "Đêm mưa về quê"},
		nil,
		map[int]string{1: "Đoạn văn."},
		exportLabels,
	)
	if strings.Contains(got, "《") {
		t.Errorf("should not contain book title brackets: %s", got)
	}
	if !strings.HasPrefix(got, "Chương 1  Đêm mưa về quê") {
		t.Errorf("expect chapter header at very start: %s", got)
	}
}

// TestRenderTXT_LayeredVolume 验证分层大纲只在卷首插卷分隔，弧分隔永不出现
// （issue #27：版式定为"书名→卷分隔→章节正文"）。
func TestRenderTXT_LayeredVolume(t *testing.T) {
	locs := map[int]chapterLocation{
		1: {VolumeIdx: 1, VolumeTitle: "Khởi nguyên", IsFirstOfVolume: true},
		2: {VolumeIdx: 1, VolumeTitle: "Khởi nguyên"},
	}
	got := renderTXT(
		"X", []int{1, 2},
		chapterTitleIndex{1: "A", 2: "B"},
		locs,
		map[int]string{1: "Đoạn một.", 2: "Đoạn hai."},
		exportLabels,
	)
	if !strings.Contains(got, "Tập 1  Khởi nguyên") {
		t.Errorf("missing volume header: %s", got)
	}
	// 卷标题只在第一章前出现一次
	if strings.Count(got, "Tập 1") != 1 {
		t.Errorf("volume header should appear exactly once: %s", got)
	}
}

func TestRenderTXT_ChapterWithoutTitleFallsBackToNumberOnly(t *testing.T) {
	got := renderTXT(
		"", []int{5},
		chapterTitleIndex{}, // 没有标题
		nil,
		map[int]string{5: "Đoạn văn."},
		exportLabels,
	)
	if !strings.Contains(got, "Chương 5\n\n") {
		t.Errorf("expect 'Chương 5' fallback header: %s", got)
	}
}
