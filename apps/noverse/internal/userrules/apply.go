package userrules

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/datnt1998/noverse/internal/rules"
	"github.com/datnt1998/noverse/internal/store"
)

// ErrRuntimeRuleNotFound: id không có trong danh sách luật học từ góp ý.
var ErrRuntimeRuleNotFound = errors.New("Không tìm thấy quy tắc này")

// RuleItem là một quy tắc (một dòng trong quy-tac.md) kèm cách Noverse hiểu nó.
type RuleItem struct {
	Text    string       `json:"text"`
	Scope   string       `json:"scope"` // global | project | story
	Chips   []rules.Chip `json:"chips"`
	Options []string     `json:"options"` // 0–3 cách hiểu (quy tắc viết lại); [] khi không có điều chưa chắc hoặc đã hạ cấp
	Status  rules.Status `json:"status"`  // ready | degraded
	// Uncertain: Noverse chưa chắc hiểu đúng quy tắc này (Câu 26a). UncertainNote
	// là câu hiện trong khung cam ("Noverse chưa chắc “quá dài” là bao nhiêu.");
	// quy tắc đã hiểu chắc thì false + chuỗi rỗng.
	Uncertain     bool   `json:"uncertain"`
	UncertainNote string `json:"uncertainNote"`
}

func newRuleItem(text, scope string, c rules.Candidate) RuleItem {
	status := rules.StatusReady
	if c.Degraded {
		status = rules.StatusDegraded
	}
	note := strings.TrimSpace(c.UncertainNote)
	if note == "" && len(c.Uncertain) > 0 {
		note = c.Uncertain[0]
	}
	if !c.UncertainFlag || c.Degraded {
		note = ""
	}
	return RuleItem{
		Text: text, Scope: scope, Chips: rules.Chips(c), Options: optionsOrEmpty(c), Status: status,
		Uncertain: status == rules.StatusReady && c.UncertainFlag, UncertainNote: note,
	}
}

// optionsOrEmpty: luôn là mảng; ứng viên hạ cấp không có cách hiểu.
func optionsOrEmpty(c rules.Candidate) []string {
	if c.Degraded || c.Options == nil {
		return []string{}
	}
	return c.Options
}

// RuntimeView là một luật học từ góp ý, dạng hiển thị.
type RuntimeView struct {
	ID            string       `json:"id"`
	Text          string       `json:"text"`
	Chapter       int          `json:"chapter"`
	CreatedAt     time.Time    `json:"createdAt"`
	Chips         []rules.Chip `json:"chips"`
	Uncertain     []string     `json:"uncertain"`     // lý do chưa chắc (dữ liệu đã lưu, giữ nguyên dạng mảng)
	UncertainNote string       `json:"uncertainNote"` // câu hiện trong khung cam; "" khi đã hiểu chắc
	Options       []string     `json:"options"`
	Status        rules.Status `json:"status"`
}

// ChapterTiming mô tả "khi nào có hiệu lực": từ chương EffectiveFrom; chương InProgress (nếu có)
// đang viết dở nên giữ luật cũ; Completed chương đã viết không bị sửa.
type ChapterTiming struct {
	EffectiveFrom int `json:"effectiveFrom"`
	InProgress    int `json:"inProgress"`
	Completed     int `json:"completed"`
}

// Effective là dữ liệu cho bảng "Noverse hiểu thế nào?" và nhóm "Học từ góp ý".
// Snapshot là bản đã áp dụng (cái trợ lý thật sự nhận); Items đọc thẳng từ các tệp quy-tac.md
// hiện tại nên có thể đi trước Snapshot cho tới khi bấm Áp dụng.
type Effective struct {
	Snapshot *rules.Snapshot `json:"snapshot"`
	Items    []RuleItem      `json:"items"`
	Runtime  []RuntimeView   `json:"runtime"`
	Timing   ChapterTiming   `json:"timing"`
}

// Timing tính mốc hiệu lực từ tiến độ hiện tại.
func (s *Service) Timing() ChapterTiming {
	p, err := s.store.Progress.Load()
	if err != nil || p == nil {
		return ChapterTiming{EffectiveFrom: 1}
	}
	t := ChapterTiming{InProgress: p.InProgressChapter, Completed: len(p.CompletedChapters)}
	last := 0
	for _, ch := range p.CompletedChapters {
		last = max(last, ch)
	}
	if p.InProgressChapter > 0 {
		t.EffectiveFrom = p.InProgressChapter + 1
	} else {
		t.EffectiveFrom = last + 1
	}
	return t
}

// Effective gom snapshot hiện hành, từng quy tắc trong quy-tac.md (chuẩn hoá từng dòng,
// dòng nào đã có trong bộ nhớ đệm thì không gọi model) và các luật học từ góp ý.
// Không ghi snapshot.
func (s *Service) Effective(ctx context.Context) (*Effective, error) {
	snap, err := s.store.UserRules.LoadCurrent()
	if err != nil {
		return nil, err
	}
	if snap == nil {
		def := rules.BuildSnapshot([]rules.Candidate{rules.SystemDefaults()})
		snap = &def
	}
	if snap.Uncertain == nil { // snapshot cũ lưu null
		snap.Uncertain = []string{}
	}
	runtime, err := s.store.UserRules.Runtime()
	if err != nil {
		return nil, err
	}
	b := s.newBatch()
	items := []RuleItem{}
	for _, rs := range rules.RawFileSources(s.rulesOpts) {
		if rs.Name != rules.ManagedRulesFile {
			continue
		}
		_, fileItems := b.fileCandidate(ctx, rs)
		items = append(items, fileItems...)
	}
	lessonRules, _, err := s.store.UserRules.LessonRules()
	if err != nil {
		return nil, err
	}
	for _, candidate := range lessonRules {
		scope := "global"
		if strings.HasPrefix(candidate.Source, "pack:") {
			scope = "pack"
		}
		items = append(items, newRuleItem(candidate.Preferences, scope, candidate))
	}
	views := make([]RuntimeView, 0, len(runtime))
	for _, r := range runtime {
		cand := r.Candidate
		if r.Legacy && strings.TrimSpace(r.Candidate.Preferences) != "" {
			cand = b.normalize(ctx, store.RuntimeRuleSource, r.Text)
		}
		item := newRuleItem(r.Text, "", cand)
		reasons := cand.Uncertain
		if reasons == nil {
			reasons = []string{}
		}
		views = append(views, RuntimeView{ID: r.ID, Text: r.Text, Chapter: r.Chapter, CreatedAt: r.CreatedAt,
			Chips: item.Chips, Uncertain: reasons, UncertainNote: item.UncertainNote, Options: item.Options, Status: item.Status})
	}
	b.flush()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &Effective{Snapshot: snap, Items: items, Runtime: views, Timing: s.Timing()}, nil
}

// ApplyResult là kết quả Apply: snapshot mới và chương đầu tiên dùng nó. PinnedChapter > 0 là
// chương đang viết dở, vẫn giữ snapshot cũ cho đến khi viết xong.
type ApplyResult struct {
	Snapshot      *rules.Snapshot `json:"snapshot"`
	EffectiveFrom int             `json:"effectiveFrom"`
	PinnedChapter int             `json:"pinnedChapter,omitempty"`
}

// Apply dựng lại snapshot = system_defaults + các tệp quy tắc (chuẩn hoá, có bộ nhớ đệm theo
// nội dung) + startup_prompt gốc + các luật runtime, rồi áp dụng từ chương kế tiếp: chương đang
// viết giữ snapshot cũ. startupPrompt là yêu cầu sáng tác gốc (RunMeta.StartPrompt), có thể rỗng.
// Nếu ctx bị huỷ giữa chừng thì không ghi gì (tránh lưu một snapshot hạ cấp do bị ngắt).
func (s *Service) Apply(ctx context.Context, startupPrompt string) (*ApplyResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	runtime, err := s.store.UserRules.Runtime()
	if err != nil {
		return nil, err
	}
	b := s.newBatch()
	cands, _, updatedRuntime := b.staticCandidates(ctx, startupPrompt, runtime)
	lessonRules, _, err := s.store.UserRules.LessonRules()
	if err != nil {
		return nil, err
	}
	cands = append(cands, lessonRules...)
	b.flush()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	snap := rules.BuildSnapshot(cands)
	pinned, err := s.store.UserRules.ApplySnapshot(&snap)
	if err != nil {
		return nil, err
	}
	if updatedRuntime != nil {
		if err := s.store.UserRules.ReplaceRuntime(updatedRuntime); err != nil {
			return nil, err
		}
	}
	return &ApplyResult{Snapshot: &snap, EffectiveFrom: s.Timing().EffectiveFrom, PinnedChapter: pinned}, nil
}

// storyRulesDir là nơi quy-tac.md của truyện: thư mục luật cấp truyện, hoặc cấp dự án khi
// thư mục sách chính là thư mục dự án (loader không quét hai lần cùng một thư mục).
func (s *Service) storyRulesDir() string {
	if s.rulesOpts.StoryRulesDir != "" {
		return s.rulesOpts.StoryRulesDir
	}
	return s.rulesOpts.ProjectRulesDir
}

// KeepRuntime chuyển luật học từ góp ý thành một dòng "- …" trong quy-tac.md của truyện rồi
// bỏ nó khỏi danh sách runtime. Snapshot không đổi cho tới lần Áp dụng sau (lúc đó dòng trong
// tệp thay thế mục runtime).
func (s *Service) KeepRuntime(id string) (store.RuntimeRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.store.UserRules.Runtime()
	if err != nil {
		return store.RuntimeRule{}, err
	}
	idx := slices.IndexFunc(list, func(r store.RuntimeRule) bool { return r.ID == id })
	if idx < 0 {
		return store.RuntimeRule{}, ErrRuntimeRuleNotFound
	}
	rule := list[idx]
	dir := s.storyRulesDir()
	if dir == "" {
		return rule, fmt.Errorf("Không xác định được thư mục quy tắc của truyện")
	}
	if err := appendRuleLine(filepath.Join(dir, rules.ManagedRulesFile), rule.Text); err != nil {
		return rule, fmt.Errorf("Không ghi được quy tắc vào truyện: %w", err)
	}
	if _, _, err := s.store.UserRules.RemoveRuntime(id); err != nil {
		return rule, err
	}
	return rule, nil
}

// appendRuleLine thêm "- text" vào cuối tệp (tạo tệp/thư mục nếu cần). Dòng đã có thì bỏ qua,
// để thử lại sau lỗi giữa chừng không tạo dòng trùng.
func appendRuleLine(path, text string) error {
	line := collapseSpace(text)
	if line == "" {
		return fmt.Errorf("quy tắc rỗng")
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	existing := string(data)
	if slices.Contains(rules.ParseRuleLines(existing), line) {
		return nil
	}
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(path, []byte(existing+"- "+line+"\n"))
}

func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// DropRuntime bỏ một luật học từ góp ý khỏi danh sách; nó biến khỏi snapshot ở lần Áp dụng sau.
func (s *Service) DropRuntime(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, found, err := s.store.UserRules.RemoveRuntime(id)
	if err != nil {
		return err
	}
	if !found {
		return ErrRuntimeRuleNotFound
	}
	return nil
}

// RecheckResult tóm tắt một lần kiểm tra lại các chương đã viết.
type RecheckResult struct {
	Checked        int `json:"checked"`
	WithViolations int `json:"withViolations"`
	Violations     int `json:"violations"`
}

// Recheck chạy kiểm tra máy trên mọi chương đã lưu theo snap và ghi vi phạm của từng chương
// (cạnh các bản ghi từ commit_chapter). Không bao giờ sửa chương. progress được gọi sau mỗi chương.
func (s *Service) Recheck(ctx context.Context, snap *rules.Snapshot, progress func(done, total, chapter int)) (*RecheckResult, error) {
	structured := rules.Effective(snap, s.pack)
	p, err := s.store.Progress.Load()
	if err != nil {
		return nil, err
	}
	var chapters []int
	if p != nil {
		chapters = slices.Clone(p.CompletedChapters)
		slices.Sort(chapters)
	}
	res := &RecheckResult{}
	for i, ch := range chapters {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		text, err := s.store.Drafts.LoadChapterText(ch)
		if err != nil {
			return res, fmt.Errorf("Đọc chương %d: %w", ch, err)
		}
		if strings.TrimSpace(text) != "" {
			var openings []rules.Opening
			if structured.Measurements != nil {
				openings = rules.SiblingOpeningsFor(s.store.Dir(), ch)
			}
			violations := rules.CheckChapter(text, structured, s.previousChapters(ch, structured.Measurements != nil), openings...)
			if err := s.store.World.SaveRuleViolations(ch, violations); err != nil {
				return res, fmt.Errorf("Ghi kết quả kiểm tra chương %d: %w", ch, err)
			}
			res.Checked++
			if len(violations) > 0 {
				res.WithViolations++
				res.Violations += len(violations)
			}
		}
		if progress != nil {
			progress(i+1, len(chapters), ch)
		}
	}
	return res, nil
}

// previousChapters lấy hai chương gần nhất, hoặc toàn bộ tiền văn cho phép đo toàn truyện.
func (s *Service) previousChapters(chapter int, all bool) []string {
	var out []string
	for ch := chapter - 1; ch >= 1; ch-- {
		if !all && ch < chapter-2 {
			break
		}
		if text, err := s.store.Drafts.LoadChapterText(ch); err == nil && strings.TrimSpace(text) != "" {
			out = append(out, text)
		}
	}
	return out
}
