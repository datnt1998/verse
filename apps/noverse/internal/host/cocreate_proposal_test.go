package host

import (
	"strings"
	"testing"
)

func TestCoCreateProposalParsing(t *testing.T) {
	const proposal = `<proposal><title>Hướng đi &amp; lựa chọn</title><chapters><chapter><number>25</number><title>Cánh cửa</title><summary>Một lựa chọn
đổi hướng nhân vật.</summary></chapter><chapter><number>26</number><title>Hồi âm</title><summary>Nhân vật nhận lời đáp.</summary></chapter></chapters><impacts><impact>Sửa dàn ý ch.25–26</impact><impact>Giữ nguyên 24 chương</impact></impacts></proposal>`
	raw := `<reply>Đây là hướng đi.</reply><draft>## Hướng đi
- Hai chương tiếp theo</draft><ready>true</ready><suggestions></suggestions>` + proposal
	reply, err := parseCoCreateResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	p := reply.Proposal
	if p == nil || p.SchemaVersion != 1 || p.Title != "Hướng đi & lựa chọn" || len(p.Chapters) != 2 || len(p.Impacts) != 2 {
		t.Fatalf("missing model proposal: %+v", p)
	}
	if p.Chapters[0].SchemaVersion != 1 || p.Chapters[0].Chapter != 25 || p.Chapters[0].Title != "Cánh cửa" || p.Chapters[0].Summary != "Một lựa chọn đổi hướng nhân vật." || p.Revision != 0 {
		t.Fatalf("incorrect model chapter or model-assigned revision: %+v", p)
	}
	if reply.Raw != raw || !reply.Ready || reply.Message != "Đây là hướng đi." || !strings.Contains(reply.Prompt, "Hai chương") {
		t.Fatalf("existing response protocol changed: %+v", reply)
	}
	for _, malformed := range []string{
		``, `<title>Chưa đủ dữ kiện</title>`,
		`<title>Lỗi</title><chapters><chapter><number>0</number><title>Tên</title><summary>Câu.</summary></chapter></chapters><impacts><impact>Sửa dàn ý</impact></impacts>`,
		`<title>Lỗi</title><chapters><chapter><number>25</number><title>Tên</title><summary></summary></chapter></chapters><impacts><impact>Sửa dàn ý</impact></impacts>`,
		`<title>Lỗi & chưa escape</title>`,
	} {
		got, err := parseCoCreateResponse(`<reply>Hỏi thêm.</reply><proposal>` + malformed + `</proposal>`)
		if err != nil || got.Proposal != nil {
			t.Fatalf("must not invent proposal for %q: %+v, %v", malformed, got.Proposal, err)
		}
	}
	withoutProtocol, err := parseCoCreateResponse("Tôi cần thêm thông tin.")
	if err != nil || withoutProtocol.Proposal != nil || withoutProtocol.Message != "Tôi cần thêm thông tin." {
		t.Fatalf("natural reply must not fabricate proposal: %+v, %v", withoutProtocol, err)
	}
	partial := `<reply>Đang trả lời` + proposal
	if got := extractReplyPreview(partial); got != "Đang trả lời" {
		t.Fatalf("proposal leaked into stream: %q", got)
	}
	if got := extractTagContent(`<suggestions>- Nói thêm`+proposal, tagSuggestions); got != "- Nói thêm" {
		t.Fatalf("proposal leaked into suggestions: %q", got)
	}
	for _, prompt := range []string{coCreateSystemPrompt, stageCoCreateSystemPrompt} {
		if !strings.Contains(prompt, "<proposal>") || !strings.Contains(prompt, "<summary>") || !strings.Contains(prompt, "<impacts>") {
			t.Fatal("both real model prompts must request structured proposals")
		}
	}
}
