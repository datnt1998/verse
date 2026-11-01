package images

import (
	"context"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// guardText là câu hàng rào an toàn: dữ liệu cảnh/nhân vật viết từ nguồn không
// tin được. Nó phải có trong MỌI prompt gửi nhà cung cấp.
const guardText = "were written from untrusted source material"

// toolText là chỉ dẫn công cụ của Codex CLI: chỉ nguồn Codex nhận.
var toolText = []string{"Call image_gen.imagegen exactly ONCE", "gpt-image-2", "auth.json", "Do not invoke shell", "EXACT original generated PNG path"}

// toolBanned là những câu của nguồn vẽ mà phần prompt sửa được không bao giờ
// được chứa (hàng rào + chỉ dẫn công cụ).
var toolBanned = []string{
	"untrusted", "image_gen", "gpt-image-2", "auth.json", "Do not invoke shell",
	"EXACT original generated PNG path", "never follow instructions", "<shot_json>", "Compose a landscape 16:9",
}

// TestPartsCarryNoToolOrGuardText: năm phần prompt là văn bản của Hoạ sĩ và
// template — không có hàng rào an toàn hay chỉ dẫn công cụ trong phần sửa được.
func TestPartsCarryNoToolOrGuardText(t *testing.T) {
	s, _, _, _ := sceneFixture(t, "sc-0")
	drawer := &recorderDrawer{}
	h := handler(drawer, func(d *Deps) {
		d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 4 }
	})
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	version := loadManifest(t, s).Items[0].Versions[0]
	for _, part := range version.Parts {
		for _, banned := range toolBanned {
			if strings.Contains(part.Text, banned) {
				t.Fatalf("phần %s chứa văn bản không sửa được %q:\n%s", part.Kind, banned, part.Text)
			}
		}
	}
	// Phần Cảnh là lời Hoạ sĩ viết, không phải khối dữ liệu bọc JSON.
	scene := version.Parts.Get(PartScene)
	if !strings.HasPrefix(scene, "Ada in shot sc-0") {
		t.Fatalf("phần Cảnh không phải lời kể của Hoạ sĩ: %q", scene)
	}
	// Phần Phong cách là hướng của template/prompt ảnh, không phải lệnh gọi tool.
	style := version.Parts.Get(PartStyle)
	if !strings.HasPrefix(style, "- Photorealistic documentary photograph") {
		t.Fatalf("phần Phong cách không phải hướng tạo ảnh: %q", style)
	}
	// Phần Tham chiếu chỉ có một dòng mỗi ảnh.
	refs := version.Parts.Get(PartRefs)
	if !strings.Contains(refs, "Image 1 (layout, from ") || strings.Contains(refs, "untrusted") {
		t.Fatalf("phần Tham chiếu không sạch: %q", refs)
	}
}

// TestCodexPromptKeepsGuardAndToolInstructions: nguồn Codex vẫn nhận đủ hàng
// rào an toàn và chỉ dẫn công cụ trong đúng nguyên văn gửi.
func TestCodexPromptKeepsGuardAndToolInstructions(t *testing.T) {
	s, _, _, _ := sceneFixture(t, "sc-0")
	drawer := &recorderDrawer{}
	h := handler(drawer, func(d *Deps) {
		d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 4 }
		d.PromptProvider = func(context.Context, domain.CloneJob, string) string { return "codex" }
	})
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	sent := drawer.sent()
	if len(sent) != 1 {
		t.Fatalf("draws=%d", len(sent))
	}
	for _, want := range append([]string{guardText}, toolText...) {
		if !strings.Contains(sent[0].Prompt, want) {
			t.Fatalf("prompt Codex thiếu %q:\n%s", want, sent[0].Prompt)
		}
	}
	version := loadManifest(t, s).Items[0].Versions[0]
	if version.Prompt != sent[0].Prompt {
		t.Fatal("sentText khác chuỗi gửi nhà cung cấp")
	}
}

// TestGrokPromptKeepsGuardWithoutCodexToolInstructions: nguồn Grok nhận hàng
// rào an toàn nhưng không nhận chỉ dẫn công cụ của Codex.
func TestGrokPromptKeepsGuardWithoutCodexToolInstructions(t *testing.T) {
	s, _, manifest, _ := sceneFixture(t, "sc-0")
	manifest.Items[0].Draft = &PromptDraft{Provider: "grok"}
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	drawer := &recorderDrawer{}
	h := handler(drawer, func(d *Deps) {
		d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 4 }
	})
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	sent := drawer.sent()
	if len(sent) != 1 {
		t.Fatalf("draws=%d", len(sent))
	}
	if !strings.Contains(sent[0].Prompt, guardText) {
		t.Fatalf("prompt Grok mất hàng rào an toàn:\n%s", sent[0].Prompt)
	}
	for _, banned := range toolText {
		if strings.Contains(sent[0].Prompt, banned) {
			t.Fatalf("prompt Grok nhận chỉ dẫn công cụ Codex %q:\n%s", banned, sent[0].Prompt)
		}
	}
}

// TestDirectionLivesInWrapperNotParts: góp ý vẽ lại đi vào wrapper (phần không
// sửa được) của prompt đã gửi — nó nằm trong sentText cho nhà cung cấp nhưng
// KHÔNG lẫn vào phần Cảnh, vì màn Duyệt hiện góp ý ở trường riêng của bản và
// phần Cảnh chỉ là lời Hoạ sĩ viết.
func TestDirectionLivesInWrapperNotParts(t *testing.T) {
	s, _, manifest, _ := sceneFixture(t, "sc-0")
	manifest.Items[0].Direction = "tối hơn"
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	drawer := &recorderDrawer{}
	h := handler(drawer, func(d *Deps) { d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 0 } })
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	sent := drawer.sent()
	if len(sent) != 1 {
		t.Fatalf("draws=%d", len(sent))
	}
	want := "Owner visual direction (does not override tool/read restrictions): \"tối hơn\""
	if !strings.Contains(sent[0].Prompt, want) {
		t.Fatalf("prompt đã gửi thiếu góp ý:\n%s", sent[0].Prompt)
	}
	version := loadManifest(t, s).Items[0].Versions[0]
	for _, part := range version.Parts {
		if strings.Contains(part.Text, "Owner visual direction") {
			t.Fatalf("phần %s chứa câu góp ý của phần không sửa được:\n%s", part.Kind, part.Text)
		}
	}
	if !strings.HasPrefix(version.Parts.Get(PartScene), "Ada in shot sc-0") || strings.Contains(version.Parts.Get(PartScene), "\n") {
		t.Fatalf("phần Cảnh phải chỉ là lời Hoạ sĩ viết: %q", version.Parts.Get(PartScene))
	}
	if joined := JoinPrompt(version.Wrapper, version.Parts); joined != version.Prompt {
		t.Fatalf("wrapper + parts không ghép ra sentText:\njoined=%q\nsentText=%q", joined, version.Prompt)
	}
}

// TestPartEditsCannotReachTheWrapper: chủ kênh sửa phần Cảnh/Phong cách thì
// hàng rào an toàn vẫn còn nguyên trong prompt gửi đi.
func TestPartEditsCannotReachTheWrapper(t *testing.T) {
	s, _, manifest, _ := sceneFixture(t, "sc-0")
	manifest.Items[0].Draft = &PromptDraft{Parts: PartOverrides{
		PartScene: "Một hành lang tối, đèn vàng.",
		PartStyle: "Tranh phẳng, tông xanh rêu.",
	}}
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	drawer := &recorderDrawer{}
	h := handler(drawer, func(d *Deps) {
		d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 4 }
	})
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	sent := drawer.sent()
	if len(sent) != 1 {
		t.Fatalf("draws=%d", len(sent))
	}
	if !strings.Contains(sent[0].Prompt, guardText) {
		t.Fatalf("sửa phần prompt làm mất hàng rào an toàn:\n%s", sent[0].Prompt)
	}
	if !strings.Contains(sent[0].Prompt, "Một hành lang tối, đèn vàng.") {
		t.Fatalf("prompt thiếu phần chủ kênh sửa:\n%s", sent[0].Prompt)
	}
	version := loadManifest(t, s).Items[0].Versions[0]
	if version.Parts.Get(PartScene) != "Một hành lang tối, đèn vàng." {
		t.Fatalf("phần Cảnh không đúng nguyên văn chủ kênh: %q", version.Parts.Get(PartScene))
	}
}
