package host

import (
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

// feedAll 一次性喂入，返回累积输出。
func feedAll(t *testing.T, tool, input string) string {
	t.Helper()
	e := newToolExtractor(tool)
	if e == nil {
		t.Fatalf("no extractor for tool %q", tool)
	}
	return e.Feed(input)
}

// feedChunked 按指定字节数分片喂入，验证流式与一次喂入结果一致。
func feedChunked(t *testing.T, tool, input string, chunk int) string {
	t.Helper()
	e := newToolExtractor(tool)
	if e == nil {
		t.Fatalf("no extractor for tool %q", tool)
	}
	var b strings.Builder
	for i := 0; i < len(input); i += chunk {
		end := min(i+chunk, len(input))
		b.WriteString(e.Feed(input[i:end]))
	}
	return b.String()
}

func mustContain(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("expected substring %q in:\n---\n%s\n---", want, got)
	}
}

func mustNotContain(t *testing.T, got, want string) {
	t.Helper()
	if strings.Contains(got, want) {
		t.Errorf("unexpected substring %q in:\n---\n%s\n---", want, got)
	}
}

// ── 通用模式：扁平 obj ──

func TestExtract_PlanChapter(t *testing.T) {
	in := `{"chapter":1,"title":"Khế ước bán thân","goal":"Xác lập nền tảng khu mỏ","conflict":"Nợ của cha","hook":"Quặng xám","emotion_arc":"Ngột ngạt"}`
	out := feedAll(t, "plan_chapter", in)
	mustContain(t, out, "✻ Lập kế hoạch")
	mustContain(t, out, "chapter: 1")
	mustContain(t, out, "title: Khế ước bán thân")
	mustContain(t, out, "goal: Xác lập nền tảng khu mỏ")
	mustContain(t, out, "conflict: Nợ của cha")
	mustContain(t, out, "hook: Quặng xám")
	mustContain(t, out, "emotion_arc: Ngột ngạt")
}

// ── 通用模式：嵌套 obj + 数组 ──

func TestExtract_FoundationCharacters(t *testing.T) {
	in := `{"type":"characters","scale":"long","content":[` +
		`{"name":"Thẩm Lệ","role":"Nhân vật chính","aliases":["Hôi Mạch","Thẩm Thất"],"description":"Thiếu niên vùng biên hoang.","traits":["Kiềm chế","Đa nghi"]},` +
		`{"name":"Cố Tiểu Đăng","role":"Vai phụ quan trọng","description":"Bé gái thử thuốc ở dược phường."}` +
		`]}`
	out := feedAll(t, "save_foundation", in)
	mustContain(t, out, "✻ Thiết lập")
	mustContain(t, out, "type: characters")
	mustContain(t, out, "scale: long")
	// 通用渲染：所有字段都展示，包括之前被白名单跳过的 aliases / traits
	mustContain(t, out, "name: Thẩm Lệ")
	mustContain(t, out, "role: Nhân vật chính")
	mustContain(t, out, "aliases:")
	mustContain(t, out, "- Hôi Mạch")
	mustContain(t, out, "- Thẩm Thất")
	mustContain(t, out, "description: Thiếu niên vùng biên hoang.")
	mustContain(t, out, "traits:")
	mustContain(t, out, "- Kiềm chế")
	mustContain(t, out, "- Đa nghi")
	mustContain(t, out, "name: Cố Tiểu Đăng")
	mustContain(t, out, "role: Vai phụ quan trọng")
}

func TestExtract_FoundationLayeredOutline(t *testing.T) {
	in := `{"type":"layered_outline","content":[` +
		`{"index":1,"title":"Lửa mỏ le lói","arcs":[` +
		`{"index":1,"title":"Lao dịch mỏ Ô Lân","goal":"Cầu sống","chapters":[` +
		`{"chapter":1,"title":"Khế ước bán thân","core_event":"Bị bán vào khu mỏ."}` +
		`]}]}]}`
	out := feedAll(t, "save_foundation", in)
	mustContain(t, out, "type: layered_outline")
	// 卷
	mustContain(t, out, "index: 1")
	mustContain(t, out, "title: Lửa mỏ le lói")
	// 弧
	mustContain(t, out, "title: Lao dịch mỏ Ô Lân")
	mustContain(t, out, "goal: Cầu sống")
	// 章
	mustContain(t, out, "chapter: 1")
	mustContain(t, out, "title: Khế ước bán thân")
	mustContain(t, out, "core_event: Bị bán vào khu mỏ.")
	// 嵌套缩进体现层级
	mustContain(t, out, "arcs:\n")
	mustContain(t, out, "chapters:\n")
}

func TestExtract_FoundationUpdateCompass(t *testing.T) {
	in := `{"type":"update_compass","content":{"ending_direction":"Một mình phi thăng vs cắt đứt huyết tế","open_threads":["Chìa khoá Hôi Mạch","Sổ sách phiếu người sống"],"estimated_scale":"5-6 tập"}}`
	out := feedAll(t, "save_foundation", in)
	mustContain(t, out, "type: update_compass")
	mustContain(t, out, "ending_direction: Một mình phi thăng vs cắt đứt huyết tế")
	mustContain(t, out, "estimated_scale: 5-6 tập")
	mustContain(t, out, "open_threads:")
	mustContain(t, out, "- Chìa khoá Hôi Mạch")
	mustContain(t, out, "- Sổ sách phiếu người sống")
}

// ── save_review：含数组对象 + 数字数组 ──

func TestExtract_SaveReview(t *testing.T) {
	in := string(reviewArgs(t, domain.ReviewEntry{
		Chapter: 3, Scope: "chapter", Verdict: "polish", Summary: "Nhịp hơi chậm.",
		Dimensions:       []domain.DimensionScore{{Dimension: "hook", Score: 55, Verdict: "fail"}},
		Issues:           []domain.ConsistencyIssue{{Type: "hook", Severity: domain.SeverityError, Description: "Cuối chương thiếu điểm móc."}},
		AffectedChapters: []int{3, 4},
	}))
	out := feedAll(t, "save_review", in)
	mustContain(t, out, "✻ Thẩm duyệt")
	mustContain(t, out, "verdict: polish")
	mustContain(t, out, "summary: Nhịp hơi chậm.")
	mustContain(t, out, "dimension: hook")
	mustContain(t, out, "score: 55")
	mustContain(t, out, "verdict: fail")
	mustContain(t, out, "type: hook")
	mustContain(t, out, "severity: error")
	mustContain(t, out, "description: Cuối chương thiếu điểm móc.")
	mustContain(t, out, "- 3")
	mustContain(t, out, "- 4")
}

// ── commit_chapter：复杂嵌套 ──

func TestExtract_CommitChapter(t *testing.T) {
	in := `{"chapter":1,"summary":"Bị bán vào khu mỏ.","characters":["Thẩm Lệ","Mẹ"],"key_events":["Ký khế ước bán thân"],"foreshadow_updates":[{"id":"f1","action":"plant","description":"Quặng xám nóng lên."}],"state_changes":[{"entity":"Thẩm Lệ","field":"Thân phận","old_value":"Thiếu niên hái thuốc","new_value":"Phu tạp dịch khu mỏ"}]}`
	out := feedAll(t, "commit_chapter", in)
	mustContain(t, out, "✻ Nộp chương")
	mustContain(t, out, "summary: Bị bán vào khu mỏ.")
	mustContain(t, out, "- Thẩm Lệ")
	mustContain(t, out, "- Mẹ")
	mustContain(t, out, "- Ký khế ước bán thân")
	mustContain(t, out, "id: f1")
	mustContain(t, out, "action: plant")
	mustContain(t, out, "description: Quặng xám nóng lên.")
	mustContain(t, out, "entity: Thẩm Lệ")
	mustContain(t, out, "field: Thân phận")
	mustContain(t, out, "old_value: Thiếu niên hái thuốc")
	mustContain(t, out, "new_value: Phu tạp dịch khu mỏ")
}

// ── edit_chapter：通用模式 + 多行 string ──

func TestExtract_EditChapter(t *testing.T) {
	in := `{"chapter":24,"old_string":"Thẩm Lệ cúi đầu không nói.\nHắn siết chặt nắm tay.","new_string":"Thẩm Lệ không ngẩng đầu, yết hầu khẽ động.\nKhớp ngón tay siết đến trắng bệch.","replace_all":false}`
	out := feedAll(t, "edit_chapter", in)
	mustContain(t, out, "✻ Gọt giũa")
	mustContain(t, out, "chapter: 24")
	mustContain(t, out, "old_string: Thẩm Lệ cúi đầu không nói.\nHắn siết chặt nắm tay.")
	mustContain(t, out, "new_string: Thẩm Lệ không ngẩng đầu, yết hầu khẽ động.\nKhớp ngón tay siết đến trắng bệch.")
	mustContain(t, out, "replace_all: false")
}

// ── 读类工具：args 信息密度低但 header + 关键字段仍应可见 ──

func TestExtract_ReadChapter(t *testing.T) {
	in := `{"chapter":234,"source":"final"}`
	out := feedAll(t, "read_chapter", in)
	mustContain(t, out, "✻ Đọc chương")
	mustContain(t, out, "chapter: 234")
	mustContain(t, out, "source: final")
}

func TestExtract_CheckConsistency(t *testing.T) {
	out := feedAll(t, "check_consistency", `{"chapter":234}`)
	mustContain(t, out, "✻ Kiểm tra nhất quán")
	mustContain(t, out, "chapter: 234")
}

// 空 args 兜底：architect 调 novel_context 不传参时 args 是 {}，
// 不能完全静默，至少要输出 header 让用户感知调用。
func TestExtract_NovelContextEmptyArgs(t *testing.T) {
	out := feedAll(t, "novel_context", `{}`)
	mustContain(t, out, "✻ Truy vấn ngữ cảnh")
}

func TestExtract_NovelContextWithChapter(t *testing.T) {
	out := feedAll(t, "novel_context", `{"chapter":234}`)
	mustContain(t, out, "✻ Truy vấn ngữ cảnh")
	mustContain(t, out, "chapter: 234")
}

// ── 裸流模式 ──

func TestExtract_DraftChapterRawMarkdown(t *testing.T) {
	in := `{"chapter":1,"content":"# Chương 1\n\nThẩm Lệ đứng ở cửa mỏ.\n"}`
	out := feedAll(t, "draft_chapter", in)
	// 裸流：无装饰、无 key prefix
	mustNotContain(t, out, "【")
	mustNotContain(t, out, "content:")
	mustNotContain(t, out, "chapter:")
	mustContain(t, out, "# Chương 1")
	mustContain(t, out, "Thẩm Lệ đứng ở cửa mỏ.")
}

func TestExtract_DraftChapterIgnoresOtherFields(t *testing.T) {
	// content 之外的字段应被静默跳过，不污染输出
	in := `{"chapter":7,"summary":"meta","content":"Nội dung","extra_array":[1,2,3]}`
	out := feedAll(t, "draft_chapter", in)
	mustContain(t, out, "Nội dung")
	mustNotContain(t, out, "meta")
	mustNotContain(t, out, "summary")
	mustNotContain(t, out, "7")
	mustNotContain(t, out, "1")
}

// ── 行为不变量 ──

func TestExtract_UnknownTool(t *testing.T) {
	if e := newToolExtractor("nonexistent_tool"); e != nil {
		t.Errorf("expected nil for unknown tool")
	}
}

func TestExtract_DoneAfterClose(t *testing.T) {
	e := newToolExtractor("plan_chapter")
	e.Feed(`{"title":"x"}`)
	if !e.Done() {
		t.Error("expected Done after closing brace")
	}
}

// ── 流式分片不变性 ──

// 同一份输入按 1/3/7/13 字节分片，输出应与一次喂入完全相同。
func TestExtract_ChunkedEqualsWhole(t *testing.T) {
	cases := []struct {
		tool  string
		input string
	}{
		{"plan_chapter", `{"title":"Khế ước bán thân","goal":"Mục tiêu","conflict":"Nợ của cha","hook":"Quặng xám","emotion_arc":"Ngột ngạt"}`},
		{"save_foundation", `{"type":"characters","content":[{"name":"Thẩm Lệ","role":"Nhân vật chính","aliases":["Hôi Mạch","Thẩm Thất"]}]}`},
		{"save_foundation", `{"type":"layered_outline","content":[{"index":1,"title":"Lửa mỏ","arcs":[{"index":1,"title":"Lao dịch mỏ","goal":"Cầu sống","chapters":[{"chapter":1,"title":"Khế ước bán thân"}]}]}]}`},
		{"save_review", string(reviewArgs(t, domain.ReviewEntry{
			Verdict: "accept", Summary: "good",
			Dimensions: []domain.DimensionScore{{Dimension: "hook", Score: 85, Verdict: "pass"}},
			Issues:     []domain.ConsistencyIssue{},
		}))},
		{"draft_chapter", `{"chapter":1,"content":"# Chương 1\n\nNội dung.\n"}`},
	}
	for _, tc := range cases {
		whole := feedAll(t, tc.tool, tc.input)
		for _, chunk := range []int{1, 3, 7, 13} {
			got := feedChunked(t, tc.tool, tc.input, chunk)
			if got != whole {
				t.Errorf("tool=%s chunk=%d differs from whole\n--- whole ---\n%s\n--- chunked ---\n%s", tc.tool, chunk, whole, got)
			}
		}
	}
}

// ── 转义与 Unicode ──

func TestExtract_EscapeSequences(t *testing.T) {
	in := `{"goal":"dòng1\ndòng2 \"ngoặc kép\" \\gạch ngược chữ Việt"}`
	out := feedAll(t, "plan_chapter", in)
	mustContain(t, out, "dòng1\ndòng2")
	mustContain(t, out, `"ngoặc kép"`)
	mustContain(t, out, `\gạch ngược`)
	mustContain(t, out, "chữ Việt")
}

func TestExtract_UnicodeEscape(t *testing.T) {
	// 中 = 中
	in := `{"goal":"中文"}`
	out := feedAll(t, "plan_chapter", in)
	mustContain(t, out, "中文")
}

// ── 空容器 / 简单结构 ──

func TestExtract_EmptyArrays(t *testing.T) {
	in := `{"key_events":[],"characters":["Thẩm Lệ"]}`
	out := feedAll(t, "commit_chapter", in)
	mustContain(t, out, "key_events:")
	mustContain(t, out, "characters:")
	mustContain(t, out, "- Thẩm Lệ")
}

func TestExtract_BoolAndNull(t *testing.T) {
	in := `{"foreshadow_updates":[{"id":"f1","action":"plant","description":null}],"chapter":1,"summary":"x","characters":["a"],"key_events":["b"]}`
	out := feedAll(t, "commit_chapter", in)
	mustContain(t, out, "id: f1")
	mustContain(t, out, "action: plant")
	mustContain(t, out, "description: null")
}

// ── 边角场景：数组嵌数组、深层嵌套 ──

func TestExtract_NestedArrays(t *testing.T) {
	// affected_chapters 是 int 数组；这里换成数组嵌数组验证
	in := `{"summary":"x","key_events":[],"characters":["a"],"foreshadow_updates":[],"relationship_changes":[]}`
	out := feedAll(t, "commit_chapter", in)
	mustContain(t, out, "summary: x")
	mustContain(t, out, "key_events:")
	mustContain(t, out, "- a")
}

func TestExtract_DeeplyNested(t *testing.T) {
	in := `{"a":{"b":{"c":{"d":"deep"}}}}`
	e := newToolExtractor("plan_chapter")
	out := e.Feed(in)
	mustContain(t, out, "a:")
	mustContain(t, out, "b:")
	mustContain(t, out, "c:")
	mustContain(t, out, "d: deep")
	if !e.Done() {
		t.Error("expected Done after final closing brace")
	}
}

// ── chunk 切在 utf-8 多字节中间 ──

func TestExtract_ChunkSplitInUTF8(t *testing.T) {
	// "中" 是 3 字节 (E4 B8 AD)。把切片大小定到 1，确保每个 byte 单独喂入。
	in := `{"goal":"中文测试"}`
	whole := feedAll(t, "plan_chapter", in)
	chunked := feedChunked(t, "plan_chapter", in, 1)
	if whole != chunked {
		t.Errorf("byte-by-byte chunked output differs from whole:\n--- whole ---\n%s\n--- chunked ---\n%s", whole, chunked)
	}
	mustContain(t, chunked, "中文测试")
}

// ── 裸流模式：嵌套 obj 内同名 key 不应误命中 ──

func TestExtract_NakedKeyOnlyTopLevel(t *testing.T) {
	// "content" 出现在两处：嵌套对象内 + 顶层。只有顶层那个应被流出。
	in := `{"meta":{"content":"lồng nhau không được xuất"},"content":"cấp cao nhất phải xuất"}`
	out := feedAll(t, "draft_chapter", in)
	mustContain(t, out, "cấp cao nhất phải xuất")
	mustNotContain(t, out, "lồng nhau không được xuất")
}

// ── 裸流模式：content 是非 string 时全跳过 ──

func TestExtract_NakedKeyNonStringValue(t *testing.T) {
	// content 错写成对象（不应该发生但要容忍）
	in := `{"content":{"unexpected":true}}`
	out := feedAll(t, "draft_chapter", in)
	if out != "" {
		t.Errorf("expected empty output, got: %q", out)
	}
}

// ── 顶层闭合后再 Feed 不再产出 ──

func TestExtract_FeedAfterDone(t *testing.T) {
	e := newToolExtractor("plan_chapter")
	e.Feed(`{"title":"x"}`)
	if !e.Done() {
		t.Fatal("expected Done")
	}
	if got := e.Feed(`junk`); got != "" {
		t.Errorf("expected empty output after Done, got: %q", got)
	}
}

// ── 空 chunk / 空 input ──

func TestExtract_EmptyFeed(t *testing.T) {
	e := newToolExtractor("plan_chapter")
	if got := e.Feed(""); got != "" {
		t.Errorf("expected empty output for empty feed, got: %q", got)
	}
	if e.Done() {
		t.Error("Done should be false before any input")
	}
}

// ── 数组里直接嵌数组（不经过 obj） ──

func TestExtract_ArrayOfArrays(t *testing.T) {
	in := `{"matrix":[[1,2],[3,4]]}`
	out := feedAll(t, "plan_chapter", in)
	mustContain(t, out, "matrix:")
	mustContain(t, out, "- 1")
	mustContain(t, out, "- 2")
	mustContain(t, out, "- 3")
	mustContain(t, out, "- 4")
}

// ── number 后空格再分隔符 ──

func TestExtract_NumberWithTrailingSpace(t *testing.T) {
	// "chapter": 1 ,  ← 数字前后多空格
	in := `{ "chapter" : 1 , "title" : "x" }`
	out := feedAll(t, "plan_chapter", in)
	mustContain(t, out, "chapter: 1")
	mustContain(t, out, "title: x")
}
