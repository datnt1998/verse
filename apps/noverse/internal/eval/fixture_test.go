package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/diag"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// writeTestFixture 在 root/<name> 下铺一个 fixture 目录。chapters 以章号→正文给出。
func writeTestFixture(t *testing.T, root, name string, doc fixtureDoc, chapters map[int]string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, fixtureChaptersDir), 0o755); err != nil {
		t.Fatalf("tạo thư mục fixture: %v", err)
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal fixture doc: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, fixtureFileName), data, 0o644); err != nil {
		t.Fatalf("ghi fixture.json: %v", err)
	}
	for chapter, text := range chapters {
		path := filepath.Join(dir, fixtureChaptersDir, fmt.Sprintf("%02d.md", chapter))
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatalf("ghi chương %d: %v", chapter, err)
		}
	}
	return dir
}

// sampleFixture 是三个已完成章的最小干净书稿：大纲还留 5 章空位（否则
// OutlineExhausted 会判 critical），每章都有摘要与时间线事件。
func sampleFixture() fixtureDoc {
	doc := fixtureDoc{
		Title:         "Vết khắc mùa đông",
		Premise:       "# Tiền đề\n\nMột thợ sửa đồng hồ tìm lại bản quyền mẫu khắc của mẹ.",
		TotalChapters: 8,
		Characters: []domain.Character{
			{Name: "Lâm Vãn"},
			{Name: "Tô Khiết"},
		},
		WorldRules: []domain.WorldRule{
			{Category: "society", Rule: "Thợ đồng hồ giữ bí mật mẫu khắc của khách.", Boundary: "Không bán mẫu khắc cho người ngoài."},
		},
		ChapterPlans: []domain.ChapterPlan{
			{Chapter: 4, Title: "Khách lạ", Goal: "Khách lạ đòi lại mẫu khắc.", Conflict: "Lâm Vãn không có bằng chứng.", Hook: "Khách lạ gọi đúng tên mẹ."},
		},
	}
	for chapter := 1; chapter <= 3; chapter++ {
		doc.Outline = append(doc.Outline, domain.OutlineEntry{
			Chapter:   chapter,
			Title:     fmt.Sprintf("Chương %d", chapter),
			CoreEvent: fmt.Sprintf("Sự kiện chính của chương %d.", chapter),
		})
		doc.Completed = append(doc.Completed, fixtureCompleted{
			Chapter: chapter,
			Summary: domain.ChapterSummary{
				Chapter:    chapter,
				Title:      fmt.Sprintf("Chương %d", chapter),
				Summary:    fmt.Sprintf("Tóm tắt chương %d.", chapter),
				Characters: []string{"Lâm Vãn"},
				KeyEvents:  []string{fmt.Sprintf("Lâm Vãn nhận mẫu khắc thứ %d.", chapter)},
			},
		})
		doc.Timeline = append(doc.Timeline, domain.TimelineEvent{
			Chapter: chapter,
			Time:    "chiều",
			Event:   fmt.Sprintf("Khách ghé tiệm lúc chạng vạng, chương %d.", chapter),
		})
		doc.Foreshadow = append(doc.Foreshadow, fixtureForeshadowRound{
			Chapter: chapter,
			Updates: []domain.ForeshadowUpdate{
				{ID: fmt.Sprintf("mau-khac-%d", chapter), Action: "plant", Description: "Vết khắc trên mặt sau đồng hồ."},
			},
		})
		doc.StateChanges = append(doc.StateChanges, domain.StateChange{
			Chapter: chapter, Entity: "Lâm Vãn", Field: "location", NewValue: "tiệm đồng hồ",
		})
	}
	return doc
}

func sampleChapters() map[int]string {
	text := "# Chương\n\nLâm Vãn mở cuốn sổ, ngọn đèn trên quầy khẽ rung. " +
		strings.Repeat("Tô Khiết đứng bên cửa, tay giữ chặt chiếc hộp gỗ. ", 30)
	return map[int]string{1: text, 2: text, 3: text}
}

func TestSeedFixtureRoundTrip(t *testing.T) {
	fixturesRoot := t.TempDir()
	writeTestFixture(t, fixturesRoot, "ed-clean-01", sampleFixture(), sampleChapters())
	out := filepath.Join(t.TempDir(), "book")
	if err := SeedFixture(fixturesRoot, "ed-clean-01", out, "warning"); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}

	s := store.NewStore(out)
	progress, err := s.Progress.Load()
	if err != nil || progress == nil {
		t.Fatalf("đọc progress: %v", err)
	}
	if progress.Phase != domain.PhaseWriting {
		t.Fatalf("phase = %s want writing", progress.Phase)
	}
	if len(progress.CompletedChapters) != 3 {
		t.Fatalf("completed = %v want 3 chương", progress.CompletedChapters)
	}
	if progress.TotalChapters != 8 {
		t.Fatalf("total = %d want 8", progress.TotalChapters)
	}
	if len(progress.ChapterWordCounts) != 3 || progress.TotalWordCount == 0 {
		t.Fatalf("word count chưa được tính: %+v", progress)
	}

	col := Collect(out, nil)
	if len(col.LoadErrors) != 0 {
		t.Fatalf("không được có lỗi đọc: %v", col.LoadErrors)
	}
	if len(col.ChapterTexts) != 3 {
		t.Fatalf("ChapterTexts = %d chương want 3", len(col.ChapterTexts))
	}
	for chapter, want := range sampleChapters() {
		got := col.ChapterTexts[chapter]
		if strings.TrimSpace(got) != strings.TrimSpace(want) {
			t.Fatalf("chương %d đọc lại không khớp fixture", chapter)
		}
		record, err := s.ChapterRecords.Load(chapter)
		if err != nil || record == nil {
			t.Fatalf("đọc bản ghi chương %d: %v", chapter, err)
		}
		if record.ContentSHA256 != domain.ChapterContentSHA256(want) {
			t.Fatalf("chương %d hash không khớp", chapter)
		}
		if _, ok := col.ChapterFacts[chapter]; !ok {
			t.Fatalf("chương %d thiếu ChapterFacts", chapter)
		}
	}
	if len(col.Reviews) != 0 {
		t.Fatalf("fixture sạch không nên có review, có %d", len(col.Reviews))
	}

	report, _ := diag.Diagnose(s)
	if len(report.Findings) != 0 {
		t.Fatalf("fixture seed ra không sạch: %+v", report.Findings)
	}
}

func TestSeedFixtureRejectsSymlink(t *testing.T) {
	fixturesRoot := t.TempDir()
	dir := writeTestFixture(t, fixturesRoot, "bad-link", sampleFixture(), map[int]string{1: "một", 2: "hai", 3: "ba"})
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("ngoài"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, fixtureChaptersDir, "03.md")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	err := SeedFixture(fixturesRoot, "bad-link", filepath.Join(t.TempDir(), "book"), "warning")
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink phải bị từ chối, nhận %v", err)
	}
}

func TestSeedFixtureRejectsOversizedFile(t *testing.T) {
	fixturesRoot := t.TempDir()
	chapters := sampleChapters()
	chapters[2] = strings.Repeat("chữ ", (maxFixtureFileBytes/2)+10)
	writeTestFixture(t, fixturesRoot, "too-big", sampleFixture(), chapters)

	err := SeedFixture(fixturesRoot, "too-big", filepath.Join(t.TempDir(), "book"), "warning")
	if err == nil || !strings.Contains(err.Error(), "vượt giới hạn") {
		t.Fatalf("tệp quá 64 KB phải bị từ chối, nhận %v", err)
	}
}

func TestSeedFixtureRejectsUnknownKey(t *testing.T) {
	fixturesRoot := t.TempDir()
	dir := writeTestFixture(t, fixturesRoot, "typo", sampleFixture(), sampleChapters())
	path := filepath.Join(dir, fixtureFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	raw["chapterz"] = []any{}
	broken, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, broken, 0o644); err != nil {
		t.Fatal(err)
	}

	err = SeedFixture(fixturesRoot, "typo", filepath.Join(t.TempDir(), "book"), "warning")
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("khóa lạ phải bị từ chối, nhận %v", err)
	}
}

func TestSeedFixtureRejectsOutputInsideFixture(t *testing.T) {
	fixturesRoot := t.TempDir()
	dir := writeTestFixture(t, fixturesRoot, "overlap", sampleFixture(), sampleChapters())

	err := SeedFixture(fixturesRoot, "overlap", filepath.Join(dir, "out"), "warning")
	if err == nil || !strings.Contains(err.Error(), "chồng lên") {
		t.Fatalf("OutputDir nằm trong fixture phải bị từ chối, nhận %v", err)
	}
}

func TestSeedFixtureRejectsExhaustedOutline(t *testing.T) {
	doc := sampleFixture()
	doc.TotalChapters = 3 // đã xong 3/3 nhưng vẫn ở phase=writing
	fixturesRoot := t.TempDir()
	writeTestFixture(t, fixturesRoot, "exhausted", doc, sampleChapters())

	err := SeedFixture(fixturesRoot, "exhausted", filepath.Join(t.TempDir(), "book"), "warning")
	if err == nil || !strings.Contains(err.Error(), "tổng số chương") {
		t.Fatalf("đại cương hết chỗ phải bị từ chối, nhận %v", err)
	}
}

// TestFixtureCasesLoadAndSeed 是离线回归：每个 fixture 模式 case 的 fixture 都必须
// 能真的铺成一本可诊断的书。
func TestFixtureCasesLoadAndSeed(t *testing.T) {
	fixturesRoot := filepath.Join("..", "..", "evals", "fixtures")
	for _, casesRoot := range []string{
		filepath.Join("..", "..", "evals", "cases", "calibration"),
		filepath.Join("..", "..", "evals", "cases", "editor-goi-a"),
	} {
		cases, err := LoadCases(casesRoot)
		if err != nil {
			t.Fatalf("tải case %s: %v", casesRoot, err)
		}
		for _, c := range cases {
			if !c.FixtureMode() {
				t.Errorf("%s: case trong %s phải ở fixture mode", c.ID, casesRoot)
				continue
			}
			c := c
			t.Run(c.ID, func(t *testing.T) {
				dir := filepath.Join(t.TempDir(), "book")
				if err := SeedFixture(fixturesRoot, c.Fixture, dir, c.Gate.MaxSeverity); err != nil {
					t.Fatalf("seed fixture %q: %v", c.Fixture, err)
				}
				col := Collect(dir, nil)
				if len(col.LoadErrors) != 0 {
					t.Fatalf("lỗi đọc sau seed: %v", col.LoadErrors)
				}
				if len(col.ChapterTexts) == 0 {
					t.Fatal("fixture không có chương nào đọc được")
				}
			})
		}
	}
}

// 输出目录与 fixture 重叠时必须在删除任何东西之前拦住：runner 先清目录再检查
// 就会把 fixture 一起删掉。
func TestCheckFixtureOutputRejectsSameDir(t *testing.T) {
	fixturesRoot := t.TempDir()
	dir := writeTestFixture(t, fixturesRoot, "self", sampleFixture(), sampleChapters())

	err := CheckFixtureOutput(fixturesRoot, "self", dir)
	if err == nil || !strings.Contains(err.Error(), "chồng lên") {
		t.Fatalf("输出目录等于 fixture 目录必须被拒，得到 %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, fixtureFileName)); statErr != nil {
		t.Fatalf("检查本身不得删除 fixture: %v", statErr)
	}

	if err := CheckFixtureOutput(fixturesRoot, "self", filepath.Join(t.TempDir(), "book")); err != nil {
		t.Fatalf("分离的目录不应被拒: %v", err)
	}
}
