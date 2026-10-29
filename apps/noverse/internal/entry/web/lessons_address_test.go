package web

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/lessons"
	"github.com/datnt1998/verse/apps/noverse/internal/naolint"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// lessonsAddressStory dựng một truyện có gói + một bài thẩm duyệt chương 1 + một
// vi phạm xưng hô mức error chỉ nằm trong meta/rule_violations.jsonl. merged=true
// thì engine đã gộp mục đó vào issues[] như save_review vẫn làm.
func lessonsAddressStory(t *testing.T, tg *configTestGateway, name, pack string, merged bool) {
	t.Helper()
	dir := filepath.Join(tg.project, "kich_ban", name)
	io := store.NewIO(dir)
	if err := io.WriteJSON("story.json", map[string]string{"title": "Truyện " + name}); err != nil {
		t.Fatalf("story.json: %v", err)
	}
	if err := io.WriteJSON(".ainovel/config.json", map[string]string{"style": pack}); err != nil {
		t.Fatalf("config: %v", err)
	}
	violation := rules.Violation{
		Rule: naolint.AddressRule, Target: "Tri Hạ → Nhược Vy", Actual: "em", Limit: "cô",
		Severity: rules.SeverityError,
		Message:  fmt.Sprintf("[LỖI] chương 1, dòng 5: Tri Hạ tự xưng \"em\". Trích: Em về trước đi (%s).", name),
	}
	rv := domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Verdict: "polish",
		Issues: []domain.ConsistencyIssue{{
			Type: "hook", Severity: "warning", RuleRef: "Master §7", Description: "Nhịp chậm", Quote: "đoạn 2",
		}},
	}
	if merged {
		rv.Issues = append(rv.Issues, naolint.Issue(violation, 1))
	}
	st := store.NewStore(filepath.Join(dir, "output", "novel"))
	if err := st.World.SaveReview(rv); err != nil {
		t.Fatalf("review %s: %v", name, err)
	}
	if err := st.World.SaveRuleViolations(1, []rules.Violation{violation}); err != nil {
		t.Fatalf("violations %s: %v", name, err)
	}
}

// TestSystemLessonsAddressFromStoredViolations pin câu 15 §H cho đường cũ: một vi
// phạm xưng hô chỉ nằm trong meta/rule_violations.jsonl (bài thẩm duyệt chưa gộp
// vào issues[]) vẫn thành bài học "Xưng hô" nhóm qua mọi gói — và không bị đếm hai
// lần khi bài đã gộp sẵn mục đó.
func TestSystemLessonsAddressFromStoredViolations(t *testing.T) {
	tg := newConfigTestGateway(t)
	lessonsAddressStory(t, tg, "01-a1", "nu-tan-bao-thu", false)
	lessonsAddressStory(t, tg, "02-b1", "nguoc-hon-nhan", false)
	lessonsAddressStory(t, tg, "03-a2", "nu-tan-bao-thu", true)

	admin := tg.login("admin", "admin-pass")
	list := lessonResponse(t, tg, admin, "pending")
	var address *lessons.Lesson
	for i := range list {
		if len(list[i].Refs) > 0 && list[i].Refs[0] == naolint.IssueRef {
			address = &list[i]
		}
	}
	if address == nil {
		t.Fatalf("không có bài học %q từ vi phạm xưng hô đã lưu: %+v", naolint.IssueRef, list)
	}
	// 3 truyện, mỗi truyện đúng một quan sát: truyện thứ ba đã gộp mục này vào
	// issues[] nên view không dựng thêm thẻ xưng hô cho nó.
	if address.Evidence.Stories != 3 || address.Evidence.Errors != 3 {
		t.Fatalf("xưng hô phải đếm 3 truyện/3 mục Bắt sửa: %+v", address.Evidence)
	}
	if address.Scope != "all" || address.Evidence.PacksCount != 2 {
		t.Fatalf("xưng hô phải nhóm qua mọi gói: %+v", address)
	}
	// Tên mục cố định, không lấy chiều gọi (có tên nhân vật).
	if !strings.Contains(address.Text, naolint.IssueName) {
		t.Fatalf("tên mục xưng hô phải cố định: %s", address.Text)
	}
}
