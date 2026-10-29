package version

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrint(t *testing.T) {
	var buf bytes.Buffer
	Print(&buf, Info{Version: "1.2.3", Commit: "abc123", Date: "2026-06-20"})
	got := buf.String()
	for _, want := range []string{"Noverse v1.2.3", "commit: abc123", "built: 2026-06-20"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Print missing %q in %q", want, got)
		}
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"":        "dev",
		"(devel)": "dev",
		"dev":     "dev",
		"1.2.3":   "v1.2.3",
		"v1.2.3":  "v1.2.3",
	}
	for input, want := range cases {
		if got := Normalize(input); got != want {
			t.Fatalf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestBuildStamp(t *testing.T) {
	SetBuild(Info{Version: "1.2.3", Commit: "abcdef1234567890"})
	defer SetBuild(Info{})
	if got := Build(); got != "v1.2.3+abcdef1" {
		t.Fatalf("Build() = %q, want v1.2.3+abcdef1", got)
	}
	SetBuild(Info{Version: "9.9.9", Commit: "unknown"})
	if got := Build(); got != "v9.9.9+unknown" {
		t.Fatalf("Build() = %q, want v9.9.9+unknown", got)
	}
}
