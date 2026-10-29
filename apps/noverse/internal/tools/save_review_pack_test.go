package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/llmcontract"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/tools"
	"golang.org/x/text/unicode/norm"
)

func reviewArgs(t *testing.T, entry domain.ReviewEntry) json.RawMessage {
	t.Helper()
	args, err := llmcontract.EncodeArgs(tools.NewSaveReviewTool(nil, nil).Schema(), entry)
	if err != nil {
		t.Fatalf("marshal review: %v", err)
	}
	return args
}

func TestSaveReviewPackLabels(t *testing.T) {
	pack := assets.Load("nu-tan-bao-thu", assets.LoadOptions{}).Pack
	base := domain.ConsistencyIssue{Type: "hook", Severity: domain.SeverityError, Description: "Kết an toàn", Evidence: "Tôi đi ngủ.", RuleRef: "Master §7", RuleText: "Không kết chương ở trạng thái an toàn", Quote: "Tôi đi ngủ.", RequiresChange: true}
	for _, tc := range []struct {
		name             string
		change           func(*domain.ConsistencyIssue)
		verdict, wantErr string
		noPack           bool
	}{
		{"error severity warning", func(i *domain.ConsistencyIssue) { i.Severity = domain.SeverityWarning }, "polish", "sửa severity", false},
		{"error no change", func(i *domain.ConsistencyIssue) { i.RequiresChange = false }, "accept", "requires_change=true", false},
		{"error no quote", func(i *domain.ConsistencyIssue) { i.Quote = " \n " }, "polish", "quote", false},
		{"correct error", nil, "polish", "", false},
		{"correct critical", func(i *domain.ConsistencyIssue) { i.Severity = domain.SeverityCritical }, "rewrite", "", false},
		{"verdict guard", nil, "accept", "accept review cannot", false},
		{"warning only section", func(i *domain.ConsistencyIssue) {
			i.RuleRef = "Master §15.1"
			i.RuleText = ""
			i.Severity = domain.SeverityWarning
			i.RequiresChange = false
			i.Quote = ""
		}, "accept", "", false},
		{"mixed warning", func(i *domain.ConsistencyIssue) {
			i.RuleRef = "Master §13"
			i.RuleText = norm.NFD.String("THOẠI   CHIẾM KHOẢNG 40–55% MỖI CHƯƠNG.")
			i.Severity = domain.SeverityWarning
			i.RequiresChange = false
			i.Quote = ""
		}, "accept", "", false},
		{"mixed error warning", func(i *domain.ConsistencyIssue) {
			i.RuleRef = "Master §13"
			i.RuleText = "Cảnh đối chất ở cao trào phải diễn bằng thoại qua lại"
			i.Severity = domain.SeverityWarning
		}, "polish", "sửa severity", false},
		{"mixed missing rule text", func(i *domain.ConsistencyIssue) { i.RuleText = "" }, "polish", "rule_text", false},
		{"mixed short rule text", func(i *domain.ConsistencyIssue) { i.RuleText = "Không kết" }, "polish", "ít nhất 15 ký tự", false},
		{"mixed no match", func(i *domain.ConsistencyIssue) { i.RuleText = "Câu luật này không tồn tại trong mục" }, "polish", "chép đoạn dài hơn", false},
		{"mixed ambiguous", func(i *domain.ConsistencyIssue) {
			i.RuleText = "một trong bốn kiểu móc"
			i.RuleRef = "Master §7"
		}, "polish", "chép đoạn dài hơn", false},
		{"unknown ref", func(i *domain.ConsistencyIssue) { i.RuleRef = "Master §999" }, "polish", "<nhãn tệp> §<số mục>", false},
		{"unlabelled section", func(i *domain.ConsistencyIssue) {
			i.RuleRef = "Định dạng §1"
			i.RuleText = ""
			i.Severity = domain.SeverityWarning
			i.RequiresChange = false
			i.Quote = ""
		}, "accept", "", false},
		{"no ref unchanged", func(i *domain.ConsistencyIssue) {
			i.RuleRef = ""
			i.RuleText = ""
			i.Severity = domain.SeverityWarning
			i.RequiresChange = false
			i.Quote = ""
		}, "accept", "", false},
		{"no pack unchanged", func(i *domain.ConsistencyIssue) {
			i.RuleRef = "Bất kỳ §999"
			i.RuleText = ""
			i.Severity = domain.SeverityWarning
			i.RequiresChange = false
			i.Quote = ""
		}, "accept", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := store.NewStore(t.TempDir())
			if err := s.Init(); err != nil {
				t.Fatal(err)
			}
			if err := s.Progress.Init(8); err != nil {
				t.Fatal(err)
			}
			if err := s.Progress.MarkChapterComplete(3, 700, "", ""); err != nil {
				t.Fatal(err)
			}
			// Trích dẫn luật gói phải tồn tại nguyên văn trong chính văn đã nộp.
			if err := s.Drafts.SaveFinalChapter(3, "Đêm khuya rồi. Tôi đi ngủ. Cửa khép lại sau lưng."); err != nil {
				t.Fatal(err)
			}
			var source tools.PackRuleSource = pack
			if tc.noPack {
				source = nil
			}
			tool := tools.NewSaveReviewTool(s, source)
			if err := llmcontract.ValidateStrictReady(tool.Schema()); err != nil {
				t.Fatal(err)
			}
			issue := base
			if tc.change != nil {
				tc.change(&issue)
			}
			// The ambiguous case duplicates an actual matching rule, not a model echo.
			if tc.name == "mixed ambiguous" {
				copyPack := *pack
				copyPack.Luat = append([]assets.PackRuleFile(nil), pack.Luat...)
				copyPack.Luat[0].Content = strings.Replace(copyPack.Luat[0].Content, "## 8. Vùng giữa", "- [CẢNH BÁO] Chọn một trong bốn kiểu móc theo bối cảnh.\n\n## 8. Vùng giữa", 1)
				tool = tools.NewSaveReviewTool(s, &copyPack)
			}
			entry := domain.ReviewEntry{Chapter: 3, Scope: "chapter", Issues: []domain.ConsistencyIssue{issue}, Dimensions: []domain.DimensionScore{{Dimension: "hook", Score: 80, Comment: "Đã đọc chương"}}, Verdict: tc.verdict, Summary: "Đánh giá chương ba"}
			_, err := tool.Execute(context.Background(), reviewArgs(t, entry))
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("muốn lỗi chứa %q, nhận %v", tc.wantErr, err)
				}
				if saved, err := s.World.LoadReview(3); err != nil || saved != nil {
					t.Fatalf("review bị từ chối không được lưu: %+v, %v", saved, err)
				}
				p, err := s.Progress.Load()
				if err != nil || len(p.PendingRewrites) != 0 {
					t.Fatalf("không được đổi hàng đợi: %+v, %v", p, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			saved, err := s.World.LoadReview(3)
			if err != nil || saved == nil || saved.Verdict != tc.verdict || saved.Issues[0].RuleRef != issue.RuleRef || saved.Issues[0].RuleText != issue.RuleText || saved.Issues[0].Quote != issue.Quote {
				t.Fatalf("lưu thiếu verdict/tham chiếu/trích dẫn: %+v, %v", saved, err)
			}
		})
	}
}
