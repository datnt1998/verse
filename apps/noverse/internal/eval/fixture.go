package eval

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/noverse/internal/diag"
	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/store"
	"github.com/datnt1998/noverse/internal/version"
)

// fixture 目录的硬约束：单个文件上限与章节数上限。fixture 是评测输入，不是书稿
// 备份；超过这个量说明该拆 case，而不是把评测成本抬到没人愿意跑。
const (
	maxFixtureFileBytes = 64 << 10
	maxFixtureChapters  = 12
	fixtureFileName     = "fixture.json"
	fixtureChaptersDir  = "chapters"
)

var fixtureChapterFilePattern = regexp.MustCompile(`^(\d{2})\.md$`)

// fixtureDoc 是 fixture.json 的解码形状。键与 domain 类型一一对应，未来阶段的
// 新键必须在这里显式登记——未知键一律报错，避免拼错的字段被静默忽略。
type fixtureDoc struct {
	Title           string                   `json:"title"`
	Premise         string                   `json:"premise"`
	TotalChapters   int                      `json:"total_chapters,omitempty"`
	Outline         []domain.OutlineEntry    `json:"outline,omitempty"`
	Characters      []domain.Character       `json:"characters,omitempty"`
	WorldRules      []domain.WorldRule       `json:"world_rules,omitempty"`
	Completed       []fixtureCompleted       `json:"completed,omitempty"`
	Timeline        []domain.TimelineEvent   `json:"timeline,omitempty"`
	Foreshadow      []fixtureForeshadowRound `json:"foreshadow,omitempty"`
	StateChanges    []domain.StateChange     `json:"state_changes,omitempty"`
	ChapterPlans    []domain.ChapterPlan     `json:"chapter_plans,omitempty"`
	Reviews         []domain.ReviewEntry     `json:"reviews,omitempty"`
	PendingRechecks []int                    `json:"pending_rechecks,omitempty"`
}

// fixtureCompleted 是"已完成的一章"：正文在 chapters/NN.md，摘要与事实在 fixture.json。
type fixtureCompleted struct {
	Chapter int                   `json:"chapter"`
	Summary domain.ChapterSummary `json:"summary"`
}

// fixtureForeshadowRound 是某一章对伏笔账本的增量操作。
type fixtureForeshadowRound struct {
	Chapter int                       `json:"chapter"`
	Updates []domain.ForeshadowUpdate `json:"updates"`
}

// SeedFixture 把 evals/fixtures/<name> 写成一个 phase=writing 的书稿目录，全部通过
// store API 落盘，随后用 diag 自检：fixture 自己都不干净，评测结论就没有意义。
//
// fixturesRoot 是 fixture 根目录（默认 evals/fixtures）；outputDir 是本次运行独占的
// 输出目录，必须与 fixture 目录互不包含。
func SeedFixture(fixturesRoot, name, outputDir, maxSeverity string) error {
	dir, err := resolveFixtureDir(fixturesRoot, name)
	if err != nil {
		return err
	}
	doc, chapters, err := loadFixture(dir)
	if err != nil {
		return err
	}
	ordered, err := orderCompleted(doc.Completed)
	if err != nil {
		return err
	}
	out, err := resolveOutputPath(outputDir)
	if err != nil {
		return err
	}
	if err := ensureSeparated(dir, out); err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return fmt.Errorf("tạo thư mục đầu ra: %w", err)
	}

	s := store.NewStore(out)
	if err := s.Init(); err != nil {
		return fmt.Errorf("khởi tạo store: %w", err)
	}
	if err := seedFoundation(s, doc, ordered); err != nil {
		return err
	}
	if err := seedChapters(s, doc, ordered, chapters); err != nil {
		return err
	}
	return checkFixtureDiagnostics(s, maxSeverity)
}

// resolveFixtureDir 把 fixture 名解析成真实目录，并保证它仍然落在 fixture 根下、
// 本身不是符号链接。名字只允许安全字符，路径穿越在拼路径之前就被拒。
func resolveFixtureDir(fixturesRoot, name string) (string, error) {
	if !fixtureNamePattern.MatchString(name) {
		return "", fmt.Errorf("tên fixture không hợp lệ %q: chỉ cho phép chữ thường/số/gạch nối", name)
	}
	root, err := resolveExistingDir(fixturesRoot)
	if err != nil {
		return "", fmt.Errorf("gốc fixture không dùng được: %w", err)
	}
	candidate := filepath.Join(root, name)
	info, err := os.Lstat(candidate)
	if err != nil {
		return "", fmt.Errorf("fixture %q không đọc được: %w", name, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("fixture %q là symlink, không được phép", name)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("fixture %q không phải thư mục", name)
	}
	dir, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("fixture %q không dùng được: %w", name, err)
	}
	if !pathUnder(root, dir) {
		return "", fmt.Errorf("fixture %q trỏ ra ngoài gốc fixture: %s", name, dir)
	}
	if err := checkFixtureTree(dir); err != nil {
		return "", err
	}
	return dir, nil
}

func resolveExistingDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s không phải thư mục", dir)
	}
	return resolved, nil
}

// CheckFixtureOutput 在 runner 删除输出目录之前确认 fixture 与输出目录互不包含。
// 顺序很重要：先清目录再检查，一旦两者重叠就已经把 fixture 删掉了。
func CheckFixtureOutput(fixturesRoot, name, outputDir string) error {
	dir, err := resolveFixtureDir(fixturesRoot, name)
	if err != nil {
		return err
	}
	out, err := resolveOutputPath(outputDir)
	if err != nil {
		return err
	}
	return ensureSeparated(dir, out)
}

func ensureSeparated(fixtureDir, outputDir string) error {
	if overlaps(fixtureDir, outputDir) {
		return fmt.Errorf("thư mục fixture %s chồng lên thư mục đầu ra %s", fixtureDir, outputDir)
	}
	return nil
}

// resolveOutputPath 解析输出目录的真实路径，但不创建它：先在"还不存在的目录"
// 上判重叠，才能在删除旧产物之前拦住危险配置。
func resolveOutputPath(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("thiếu thư mục đầu ra")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	probe := abs
	var missing []string
	for {
		if _, err := os.Lstat(probe); err == nil {
			break
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			break
		}
		missing = append([]string{filepath.Base(probe)}, missing...)
		probe = parent
	}
	resolved, err := filepath.EvalSymlinks(probe)
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{resolved}, missing...)...), nil
}

// pathUnder 判断 child 是否位于 parent 之下（或就是 parent）。
func pathUnder(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func overlaps(a, b string) bool {
	return pathUnder(a, b) || pathUnder(b, a)
}

// checkFixtureTree 拦截符号链接、非普通文件与超限文件。fixture 只能读自己目录里
// 的真实文件：symlink 会把评测指向仓库外，非普通文件让"读多少算多少"失控。
func checkFixtureTree(dir string) error {
	return filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("fixture chứa symlink: %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("fixture chứa tệp không phải tệp thường: %s", path)
		}
		if info.Size() > maxFixtureFileBytes {
			return fmt.Errorf("tệp fixture %s có %d byte, vượt giới hạn %d", path, info.Size(), maxFixtureFileBytes)
		}
		return nil
	})
}

// loadFixture 解码 fixture.json 并读回各章正文。chapter 文件与 completed 声明必须
// 一一对应：多一个少一个都说明 fixture 与它描述的书稿已经漂移。
func loadFixture(dir string) (fixtureDoc, map[int]string, error) {
	data, err := os.ReadFile(filepath.Join(dir, fixtureFileName))
	if err != nil {
		return fixtureDoc{}, nil, fmt.Errorf("đọc %s: %w", fixtureFileName, err)
	}
	if len(data) > maxFixtureFileBytes {
		return fixtureDoc{}, nil, fmt.Errorf("%s có %d byte, vượt giới hạn %d", fixtureFileName, len(data), maxFixtureFileBytes)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var doc fixtureDoc
	if err := dec.Decode(&doc); err != nil {
		return fixtureDoc{}, nil, fmt.Errorf("phân tích %s: %w", fixtureFileName, err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return fixtureDoc{}, nil, fmt.Errorf("%s có nội dung thừa sau object JSON", fixtureFileName)
	}
	if strings.TrimSpace(doc.Title) == "" {
		return fixtureDoc{}, nil, fmt.Errorf("%s thiếu title", fixtureFileName)
	}
	if strings.TrimSpace(doc.Premise) == "" {
		return fixtureDoc{}, nil, fmt.Errorf("%s thiếu premise", fixtureFileName)
	}

	chapters, err := loadFixtureChapters(dir)
	if err != nil {
		return fixtureDoc{}, nil, err
	}
	declared := map[int]bool{}
	for _, item := range doc.Completed {
		if item.Chapter <= 0 {
			return fixtureDoc{}, nil, fmt.Errorf("completed có chapter không hợp lệ: %d", item.Chapter)
		}
		if declared[item.Chapter] {
			return fixtureDoc{}, nil, fmt.Errorf("completed khai báo trùng chương %d", item.Chapter)
		}
		declared[item.Chapter] = true
		if _, ok := chapters[item.Chapter]; !ok {
			return fixtureDoc{}, nil, fmt.Errorf("thiếu %s/%02d.md cho chương %d đã khai báo completed", fixtureChaptersDir, item.Chapter, item.Chapter)
		}
	}
	for chapter := range chapters {
		if !declared[chapter] {
			return fixtureDoc{}, nil, fmt.Errorf("có %s/%02d.md nhưng chương %d không nằm trong completed", fixtureChaptersDir, chapter, chapter)
		}
	}
	if len(chapters) > maxFixtureChapters {
		return fixtureDoc{}, nil, fmt.Errorf("fixture có %d chương, vượt giới hạn %d", len(chapters), maxFixtureChapters)
	}
	return doc, chapters, nil
}

func loadFixtureChapters(dir string) (map[int]string, error) {
	chapters := map[int]string{}
	chapterDir := filepath.Join(dir, fixtureChaptersDir)
	entries, err := os.ReadDir(chapterDir)
	if errors.Is(err, os.ErrNotExist) {
		return chapters, nil
	}
	if err != nil {
		return nil, fmt.Errorf("đọc %s: %w", fixtureChaptersDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := fixtureChapterFilePattern.FindStringSubmatch(entry.Name())
		if match == nil {
			return nil, fmt.Errorf("%s/%s không đúng dạng NN.md", fixtureChaptersDir, entry.Name())
		}
		chapter, err := strconv.Atoi(match[1])
		if err != nil || chapter <= 0 {
			return nil, fmt.Errorf("%s/%s có số chương không hợp lệ", fixtureChaptersDir, entry.Name())
		}
		data, err := os.ReadFile(filepath.Join(chapterDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		if len(data) > maxFixtureFileBytes {
			return nil, fmt.Errorf("%s/%s có %d byte, vượt giới hạn %d", fixtureChaptersDir, entry.Name(), len(data), maxFixtureFileBytes)
		}
		if strings.TrimSpace(string(data)) == "" {
			return nil, fmt.Errorf("%s/%s rỗng", fixtureChaptersDir, entry.Name())
		}
		chapters[chapter] = string(data)
	}
	return chapters, nil
}

// orderCompleted 返回按章号升序的 completed 列表，并要求章节从第 1 章起连续——
// 断章是 diag ChapterGaps 的领域，fixture 自己就不该造出来。
func orderCompleted(completed []fixtureCompleted) ([]fixtureCompleted, error) {
	ordered := append([]fixtureCompleted(nil), completed...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Chapter < ordered[j].Chapter })
	for i, item := range ordered {
		want := i + 1
		if item.Chapter != want {
			return nil, fmt.Errorf("completed phải liên tục từ chương 1: chương %d đứng ở vị trí %d", item.Chapter, want)
		}
	}
	return ordered, nil
}

func seedFoundation(s *store.Store, doc fixtureDoc, completed []fixtureCompleted) error {
	if err := s.Book.Save(domain.BookMetadata{Title: doc.Title, Synopsis: synopsisFromPremise(doc.Premise)}); err != nil {
		return fmt.Errorf("lưu thông tin truyện: %w", err)
	}
	if err := s.Outline.SavePremise(doc.Premise); err != nil {
		return fmt.Errorf("lưu premise: %w", err)
	}
	if err := s.Outline.SaveOutline(doc.Outline); err != nil {
		return fmt.Errorf("lưu đại cương: %w", err)
	}
	if err := s.Characters.Save(doc.Characters); err != nil {
		return fmt.Errorf("lưu nhân vật: %w", err)
	}
	if err := s.World.SaveWorldRules(doc.WorldRules); err != nil {
		return fmt.Errorf("lưu luật thế giới: %w", err)
	}
	total := fixtureTotalChapters(doc, completed)
	if total <= len(completed) {
		return fmt.Errorf("fixture ở phase=writing cần tổng số chương lớn hơn số chương đã hoàn thành (%d); hãy khai báo total_chapters hoặc đại cương dài hơn", len(completed))
	}
	if err := s.Progress.Init(total); err != nil {
		return fmt.Errorf("khởi tạo progress: %w", err)
	}
	if err := s.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		return fmt.Errorf("chuyển phase=writing: %w", err)
	}
	for _, plan := range doc.ChapterPlans {
		if err := s.Drafts.SaveChapterPlan(plan); err != nil {
			return fmt.Errorf("lưu kế hoạch chương %d: %w", plan.Chapter, err)
		}
	}
	return nil
}

func fixtureTotalChapters(doc fixtureDoc, completed []fixtureCompleted) int {
	total := doc.TotalChapters
	if total <= 0 {
		for _, entry := range doc.Outline {
			if entry.Chapter > total {
				total = entry.Chapter
			}
		}
	}
	if len(completed) > total {
		total = len(completed)
	}
	return total
}

func seedChapters(s *store.Store, doc fixtureDoc, completed []fixtureCompleted, chapters map[int]string) error {
	for _, item := range completed {
		chapter := item.Chapter
		content := domain.NormalizeChapterContent(chapters[chapter])
		facts := fixtureFacts(doc, item.Summary, chapter)
		if err := s.Drafts.SaveFinalChapter(chapter, content); err != nil {
			return fmt.Errorf("lưu chương %d: %w", chapter, err)
		}
		record := domain.ChapterRecord{
			Version:       domain.ChapterRecordVersion,
			Chapter:       chapter,
			Revision:      1,
			Build:         version.Build(),
			Origin:        domain.ChapterOriginGenerated,
			Content:       content,
			ContentSHA256: domain.ChapterContentSHA256(content),
			Facts:         facts,
			AcceptedAt:    time.Now(),
		}
		if err := s.ChapterRecords.Save(record); err != nil {
			return fmt.Errorf("lưu bản ghi chương %d: %w", chapter, err)
		}
		summary := item.Summary
		summary.Chapter = chapter
		if err := s.Summaries.SaveSummary(summary); err != nil {
			return fmt.Errorf("lưu tóm tắt chương %d: %w", chapter, err)
		}
		if err := s.Progress.MarkChapterComplete(chapter, domain.WordCount(content), "", ""); err != nil {
			return fmt.Errorf("đánh dấu hoàn thành chương %d: %w", chapter, err)
		}
	}
	if len(doc.Timeline) > 0 {
		if err := s.World.AppendTimelineEvents(doc.Timeline); err != nil {
			return fmt.Errorf("lưu dòng thời gian: %w", err)
		}
	}
	for _, round := range doc.Foreshadow {
		if len(round.Updates) == 0 {
			continue
		}
		if err := s.World.UpdateForeshadow(round.Chapter, round.Updates); err != nil {
			return fmt.Errorf("cập nhật phục bút chương %d: %w", round.Chapter, err)
		}
	}
	if len(doc.StateChanges) > 0 {
		if err := s.World.AppendStateChanges(doc.StateChanges); err != nil {
			return fmt.Errorf("lưu thay đổi trạng thái: %w", err)
		}
	}
	for _, review := range doc.Reviews {
		if review.Scope != "chapter" || review.Chapter <= 0 || review.Chapter > len(completed) || review.Recheck {
			return fmt.Errorf("review nguồn fixture phải là chapter đã hoàn thành, chưa recheck")
		}
		if err := s.World.SaveReview(review); err != nil {
			return fmt.Errorf("lưu review nguồn: %w", err)
		}
	}
	for _, chapter := range doc.PendingRechecks {
		if chapter <= 0 || chapter > len(completed) {
			return fmt.Errorf("pending_rechecks không hợp lệ: %d", chapter)
		}
		review, err := s.World.LoadReview(chapter)
		if err != nil {
			return err
		}
		if review == nil {
			return fmt.Errorf("pending_rechecks thiếu review nguồn chương %d", chapter)
		}
		if err := s.Progress.CompleteRewrite(chapter, true); err != nil {
			return fmt.Errorf("xếp kiểm lại fixture: %w", err)
		}
	}
	return nil
}

// fixtureFacts 从 fixture 已有的事实源拼出章节事实记录：摘要在 completed 里，
// 时间线/伏笔/状态变化按章号归属。评测断言读的是同一份记录，不另造事实。
func fixtureFacts(doc fixtureDoc, summary domain.ChapterSummary, chapter int) domain.ChapterFacts {
	facts := domain.ChapterFacts{
		Title:      summary.Title,
		Summary:    summary.Summary,
		Characters: summary.Characters,
		KeyEvents:  summary.KeyEvents,
	}
	for _, event := range doc.Timeline {
		if event.Chapter == chapter {
			facts.TimelineEvents = append(facts.TimelineEvents, event)
		}
	}
	for _, round := range doc.Foreshadow {
		if round.Chapter == chapter {
			facts.ForeshadowUpdates = append(facts.ForeshadowUpdates, round.Updates...)
		}
	}
	for _, change := range doc.StateChanges {
		if change.Chapter == chapter {
			facts.StateChanges = append(facts.StateChanges, change)
		}
	}
	return facts
}

// checkFixtureDiagnostics 保证 seed 出来的书稿本身不超过 case 的门禁：如果 fixture
// 自己就带着 critical finding，任何后续比较都失去意义。
func checkFixtureDiagnostics(s *store.Store, maxSeverity string) error {
	if strings.TrimSpace(maxSeverity) == "" {
		maxSeverity = "warning"
	}
	report, _ := diag.Diagnose(s)
	limit := severityRank(maxSeverity)
	var violations []string
	for _, finding := range report.Findings {
		if severityRank(string(finding.Severity)) >= limit {
			continue
		}
		violations = append(violations, fmt.Sprintf("%s [%s] %s", finding.Rule, finding.Severity, finding.Title))
	}
	if len(violations) > 0 {
		return fmt.Errorf("fixture không sạch sau khi seed (vượt max_severity=%s): %s",
			maxSeverity, strings.Join(violations, "; "))
	}
	return nil
}

func synopsisFromPremise(premise string) string {
	for _, line := range strings.Split(premise, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if line != "" {
			return line
		}
	}
	return strings.TrimSpace(premise)
}
