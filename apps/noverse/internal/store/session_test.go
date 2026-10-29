package store

import (
	llmtypes "github.com/datnt1998/verse/apps/noverse/internal/llm"

	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSessionStore_MetaInjected_AssistantWithUsage 验证只有"assistant + has Usage"
// 的消息才被附加 _meta，这是 replay 路径精确算价的前提。
func TestSessionStore_MetaInjected_AssistantWithUsage(t *testing.T) {
	dir := t.TempDir()
	s := newFileSessionStore(newIO(dir))
	lookup := ModelLookup(func(agentName string) (string, string) {
		return "meme", "gpt-5.4"
	})
	logger := s.SubAgentLogger(lookup)

	logger("writer", "写第 1 章", llmtypes.Message{
		Role:  llmtypes.RoleUser,
		Usage: nil,
	})
	logger("writer", "写第 1 章", llmtypes.Message{
		Role: llmtypes.RoleAssistant,
		Usage: &llmtypes.Usage{
			Input: 1000, Output: 200, CacheRead: 800, TotalTokens: 1200,
		},
	})
	logger("writer", "写第 1 章", llmtypes.Message{
		Role:  llmtypes.RoleAssistant,
		Usage: nil, // assistant 但无 usage（流式未带 final usage chunk）
	})

	entries := readJSONL(t, filepath.Join(dir, "meta/sessions/agents/writer-ch01.jsonl"))
	if len(entries) != 3 {
		t.Fatalf("entries=%d want 3", len(entries))
	}
	if _, has := entries[0]["_meta"]; has {
		t.Errorf("user message should NOT have _meta")
	}
	if _, has := entries[2]["_meta"]; has {
		t.Errorf("assistant without Usage should NOT have _meta")
	}
	meta, ok := entries[1]["_meta"].(map[string]any)
	if !ok {
		t.Fatalf("assistant+Usage should have _meta map, got %T %v", entries[1]["_meta"], entries[1]["_meta"])
	}
	if meta["provider"] != "meme" || meta["model"] != "gpt-5.4" {
		t.Errorf("_meta = %v want provider=meme model=gpt-5.4", meta)
	}
}

// TestSessionStore_MetaModelSwitch 验证运行中切换模型后，后续消息的 _meta 也跟着变。
// 这是 B 方案对"同进程内 /model 切换"的精确支持。
func TestSessionStore_MetaModelSwitch(t *testing.T) {
	dir := t.TempDir()
	s := newFileSessionStore(newIO(dir))

	current := "model-a"
	lookup := ModelLookup(func(agentName string) (string, string) {
		return "meme", current
	})
	logger := s.SubAgentLogger(lookup)

	logger("writer", "写第 1 章", makeAssistantWithUsage())
	current = "model-b" // 模拟 /model 切换
	logger("writer", "写第 1 章", makeAssistantWithUsage())

	entries := readJSONL(t, filepath.Join(dir, "meta/sessions/agents/writer-ch01.jsonl"))
	if len(entries) != 2 {
		t.Fatalf("entries=%d want 2", len(entries))
	}
	for i, want := range []string{"model-a", "model-b"} {
		meta, ok := entries[i]["_meta"].(map[string]any)
		if !ok {
			t.Fatalf("entry[%d] missing _meta", i)
		}
		if got := meta["model"]; got != want {
			t.Errorf("entry[%d] model = %v want %s", i, got, want)
		}
	}
}

// TestSessionStore_NilLookup 验证 lookup=nil 时写入仍然正常，
// 只是不带 _meta。
func TestSessionStore_NilLookup(t *testing.T) {
	dir := t.TempDir()
	s := newFileSessionStore(newIO(dir))
	logger := s.SubAgentLogger(nil)
	logger("writer", "写第 1 章", makeAssistantWithUsage())

	rel, err := s.subAgentPath("writer", "写第 1 章")
	if err != nil {
		t.Fatal(err)
	}
	entries := readJSONL(t, filepath.Join(dir, rel))
	if len(entries) != 1 {
		t.Fatalf("entries=%d want 1", len(entries))
	}
	if _, has := entries[0]["_meta"]; has {
		t.Errorf("nil lookup should not produce _meta")
	}
	// 但其他字段（role/usage）必须正常
	if entries[0]["role"] != "assistant" {
		t.Errorf("role lost: %v", entries[0]["role"])
	}
}

func TestSessionStoreContinuesAgentSequenceAcrossRestarts(t *testing.T) {
	dir := t.TempDir()
	first := newFileSessionStore(newIO(dir)).SubAgentLogger(nil)
	first("architect_long", "处理反馈", makeAssistantWithUsage())

	second := newFileSessionStore(newIO(dir)).SubAgentLogger(nil)
	second("architect_long", "扩展大纲", makeAssistantWithUsage())

	if got := len(readJSONL(t, filepath.Join(dir, "meta/sessions/agents/architect_long-001.jsonl"))); got != 1 {
		t.Fatalf("first session entries = %d, want 1", got)
	}
	if got := len(readJSONL(t, filepath.Join(dir, "meta/sessions/agents/architect_long-002.jsonl"))); got != 1 {
		t.Fatalf("second session entries = %d, want 1", got)
	}
}

func makeAssistantWithUsage() llmtypes.Message {
	return llmtypes.Message{
		Role:  llmtypes.RoleAssistant,
		Usage: &llmtypes.Usage{Input: 1000, Output: 200, TotalTokens: 1200},
	}
}

func readJSONL(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var out []map[string]any
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatalf("unmarshal line: %v\n%s", err, string(line))
		}
		out = append(out, m)
	}
	return out
}

// 压缩占位符是模型读到的东西：越南语 + 按词计长（旧实现是「%d字 | 见 chapters/」）。
func TestCompactTextPlaceholderIsVietnameseWordCount(t *testing.T) {
	big := strings.Repeat("Hắn bỏ đi. ", 500) // 1500 词，远超 4096 字节
	got := compactText(llmtypes.RoleTool, "read_chapter", big)
	want := "[session_compact: read_chapter 1500 chữ | xem chapters/]"
	if got != want {
		t.Fatalf("placeholder = %q, want %q", got, want)
	}
}

// KeepToolResults 登记的工具结果原文落盘（评测采集）；未登记与未配置的默认路径
// 照旧压缩——生产会话的压缩语义不变。
func TestKeepToolResultsRecordsVerbatim(t *testing.T) {
	dir := t.TempDir()
	s := newFileSessionStore(newIO(dir))
	logger := s.SubAgentLogger(nil)
	big := strings.Repeat("phục bút đã quá hạn ", 300) // ~5.7KB，远超 4096 阈值

	toolMsg := func(name string) llmtypes.Message {
		return llmtypes.Message{
			Role:     llmtypes.RoleTool,
			Content:  []llmtypes.ContentBlock{llmtypes.TextBlock(big)},
			Metadata: map[string]any{"tool_name": name},
		}
	}
	logger("writer", "写第 3 章", toolMsg("novel_context")) // 未登记：压缩
	s.KeepToolResults("novel_context")
	logger("writer", "写第 3 章", toolMsg("novel_context")) // 已登记：原文
	logger("writer", "写第 3 章", toolMsg("read_chapter"))  // 未登记：压缩

	entries := readJSONL(t, filepath.Join(dir, "meta/sessions/agents/writer-ch03.jsonl"))
	if len(entries) != 3 {
		t.Fatalf("entries=%d want 3", len(entries))
	}
	textAt := func(i int) string {
		blocks, _ := entries[i]["content"].([]any)
		first, _ := blocks[0].(map[string]any)
		text, _ := first["text"].(string)
		return text
	}
	if !IsCompacted(textAt(0)) {
		t.Fatalf("未登记的 novel_context 应压缩: %q", textAt(0)[:60])
	}
	if got := textAt(1); got != big {
		t.Fatalf("登记的 novel_context 应原文落盘（%dB），实际 %dB", len(big), len(got))
	}
	if !IsCompacted(textAt(2)) {
		t.Fatalf("未登记的 read_chapter 应压缩: %q", textAt(2)[:60])
	}
}

// 任务名里的章号两种写法都要认：新任务写 Chương 3，旧会话仍是 第3章。
func TestExtractChapterAcceptsBothLanguages(t *testing.T) {
	cases := map[string]string{
		"viết Chương 3 cho truyện": "ch03",
		"写第3章":                     "ch03",
		"Chapter 7 rewrite":        "ch07",
		"không có số chương":       "",
	}
	for task, want := range cases {
		if got := extractChapter(task); got != want {
			t.Errorf("extractChapter(%q) = %q, want %q", task, got, want)
		}
	}
}
