package tui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/noverse/internal/commands"
)

func TestEveryCommandHasHandler(t *testing.T) {
	specs := commands.Specs()
	if len(commandHandlers) != len(specs) {
		t.Fatalf("commandHandlers has %d entries, registry has %d", len(commandHandlers), len(specs))
	}
	for _, spec := range specs {
		if _, ok := commandHandlers[spec.Name]; !ok {
			t.Fatalf("no TUI handler for /%s", spec.Name)
		}
	}
}

func TestParseSlashCommandQuotedArgs(t *testing.T) {
	tests := []struct {
		line string
		name string
		args []string
	}{
		{line: `/export "exports/tra xanh.epub" from=1`, name: "export", args: []string{"exports/tra xanh.epub", "from=1"}},
		{line: `/import 'nhap/tra xanh.txt' --yes`, name: "import", args: []string{"nhap/tra xanh.txt", "--yes"}},
		{line: `/reopen "viết thêm tập 2"`, name: "reopen", args: []string{"viết thêm tập 2"}},
		{line: `/start "ideas/y tuong.md"`, name: "start", args: []string{"ideas/y tuong.md"}},
		{line: `/EXPORT a.txt`, name: "export", args: []string{"a.txt"}},
		{line: `  /sync --check  `, name: "sync", args: []string{"--check"}},
	}
	for _, tt := range tests {
		cmd, err := parseSlashCommand(tt.line)
		if err != nil {
			t.Fatalf("parseSlashCommand(%q) error: %v", tt.line, err)
		}
		if cmd.name != tt.name || len(cmd.args) != len(tt.args) {
			t.Fatalf("parseSlashCommand(%q) = %q %q, want %q %q", tt.line, cmd.name, cmd.args, tt.name, tt.args)
		}
		for i := range tt.args {
			if cmd.args[i] != tt.args[i] {
				t.Fatalf("parseSlashCommand(%q) args = %q, want %q", tt.line, cmd.args, tt.args)
			}
		}
	}
}

func TestParseSlashCommandErrors(t *testing.T) {
	if _, err := parseSlashCommand("xin chào"); !errors.Is(err, commands.ErrNotCommand) {
		t.Fatalf("plain text error = %v", err)
	}
	if _, err := parseSlashCommand("/"); !errors.Is(err, commands.ErrNotCommand) {
		t.Fatalf("bare slash error = %v", err)
	}
	if _, err := parseSlashCommand(`/export "a b.txt`); !errors.Is(err, commands.ErrUnterminatedQuote) {
		t.Fatalf("unterminated quote error = %v", err)
	}
}

func TestPrepareFileStartAcceptsQuotedPathToken(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ideas folder")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "story outline.md")
	if err := os.WriteFile(path, []byte("dàn ý chương 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 分词器把 "/start \"...\"" 变成一个含空格的 token。
	prompt, err := prepareFileStart([]string{path})
	if err != nil {
		t.Fatalf("prepareFileStart(%q) error: %v", path, err)
	}
	if prompt != "dàn ý chương 1" {
		t.Fatalf("prompt = %q", prompt)
	}
}
