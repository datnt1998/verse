package tui

import (
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/host/exp"
)

// TestParseExportArgs 表驱动：格式 token、doc-so、from/to 与路径的绑定。
func TestParseExportArgs(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantFmt  exp.Format
		wantPath string
		wantRead bool
		wantErr  bool
	}{
		{"bare path", []string{"out.txt"}, "", "out.txt", false, false},
		{"narration token", []string{"narration"}, exp.FormatNarration, "", false, false},
		{"narration + doc-so", []string{"narration", "doc-so"}, exp.FormatNarration, "", true, false},
		{"narration + --doc-so", []string{"narration", "--doc-so"}, exp.FormatNarration, "", true, false},
		{"format= epub", []string{"format=epub"}, exp.FormatEPUB, "", false, false},
		{"narration + path + range", []string{"narration", "/tmp/x.txt", "from=1", "to=3"}, exp.FormatNarration, "/tmp/x.txt", false, false},
		{"epub then path", []string{"epub", "sách.epub"}, exp.FormatEPUB, "sách.epub", false, false},
		{"conflicting formats", []string{"txt", "epub"}, "", "", false, true},
		{"bad flag", []string{"--bogus"}, "", "", false, true},
		{"bad kv", []string{"xx=1"}, "", "", false, true},
		{"two paths", []string{"a.txt", "b.txt"}, "", "", false, true},
	}
	for _, c := range cases {
		opts, err := parseExportArgs(c.args)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: want error, got opts %+v", c.name, opts)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", c.name, err)
			continue
		}
		if opts.Format != c.wantFmt || opts.OutPath != c.wantPath || opts.ReadNumbers != c.wantRead {
			t.Errorf("%s: opts = {fmt:%q path:%q read:%v}, want {fmt:%q path:%q read:%v}",
				c.name, opts.Format, opts.OutPath, opts.ReadNumbers, c.wantFmt, c.wantPath, c.wantRead)
		}
	}
}
