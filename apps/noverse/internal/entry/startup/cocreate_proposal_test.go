package startup

import (
	"encoding/json"
	"testing"

	"github.com/datnt1998/noverse/internal/host"
)

func TestCoCreateSessionProposalRevision(t *testing.T) {
	s := NewCoCreateSession("Bàn hướng đi")
	if s.Proposal() != nil {
		t.Fatal("new session must not invent proposal")
	}
	p := &host.CoCreateProposal{SchemaVersion: 1, Title: "Hướng đi", Revision: 99,
		Chapters: []host.CoCreateProposalChapter{{SchemaVersion: 1, Chapter: 25, Title: "Cánh cửa", Summary: "Nhân vật chọn một hướng."}},
		Impacts:  []string{"Sửa dàn ý ch.25"}}
	s.ApplyReply(host.CoCreateReply{Message: "Đã rõ", Raw: "<reply>Đã rõ</reply><proposal>nội dung mô hình</proposal>", Prompt: "## Hướng đi", Proposal: p, Ready: true})
	if s.Proposal().Revision != 1 {
		t.Fatalf("first proposal revision = %d; want 1", s.Proposal().Revision)
	}
	p.Chapters[0].Title = "Không được làm đổi phiên"
	p.Impacts[0] = "Không được làm đổi tác động"
	if s.Proposal().Chapters[0].Title != "Cánh cửa" || s.Proposal().Impacts[0] != "Sửa dàn ý ch.25" {
		t.Fatal("session must own retained proposal slices")
	}
	s.AppendUser("Chỉnh hướng đi")
	p.Title = "Hướng đã chỉnh"
	s.ApplyReply(host.CoCreateReply{Message: "Đã chỉnh", Prompt: "## Hướng đã chỉnh", Proposal: p})
	if s.Proposal().Revision != 2 || s.Proposal().Title != "Hướng đã chỉnh" {
		t.Fatalf("feedback must produce v2: %+v", s.Proposal())
	}
	s.ApplyReply(host.CoCreateReply{Message: "Cần hỏi thêm"})
	if s.Proposal().Revision != 2 || s.DraftPrompt() != "## Hướng đã chỉnh" {
		t.Fatal("unstructured reply erased real proposal or draft")
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored CoCreateSession
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Proposal().Revision != 2 || restored.Proposal().Title != "Hướng đã chỉnh" || restored.DraftPrompt() != s.DraftPrompt() || len(restored.History()) != len(s.History()) {
		t.Fatalf("session roundtrip lost model state: %s", data)
	}
	restored.ApplyReply(host.CoCreateReply{Proposal: p})
	if restored.Proposal().Revision != 3 {
		t.Fatalf("restored revision did not advance: %+v", restored.Proposal())
	}
	if err := json.Unmarshal([]byte(`{"schemaVersion":2}`), &restored); err == nil {
		t.Fatal("unsupported persisted schema accepted")
	}
}
