package commands

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestSplitArgs(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"simple", "a b", []string{"a", "b"}},
		{"extra spaces", "  a\t\tb\n", []string{"a", "b"}},
		{"double quotes", `"a b" c`, []string{"a b", "c"}},
		{"single quotes", `'a b' c`, []string{"a b", "c"}},
		{"quoted prefix value", `--guide="Hồi thứ X" --yes`, []string{"--guide=Hồi thứ X", "--yes"}},
		{"quoted path", `"ideas/y tuong moi.md"`, []string{"ideas/y tuong moi.md"}},
		{"empty quotes", `""`, []string{""}},
		{"escaped double quote", `"a \"b\" c"`, []string{`a "b" c`}},
		{"escaped single quote", `'it\'s'`, []string{"it's"}},
		{"escaped backslash", `"a\\b"`, []string{`a\b`}},
		{"windows path unquoted", `C:\Users\x`, []string{`C:\Users\x`}},
		{"quote mid token", `from="1 2"`, []string{"from=1 2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SplitArgs(tt.in)
			if err != nil {
				t.Fatalf("SplitArgs(%q) error: %v", tt.in, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("SplitArgs(%q) = %q, want %q", tt.in, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("SplitArgs(%q) = %q, want %q", tt.in, got, tt.want)
				}
			}
		})
	}
}

func TestSplitArgsUnterminatedQuote(t *testing.T) {
	for _, in := range []string{`"abc`, `'abc`, `--guide="abc`} {
		if _, err := SplitArgs(in); !errors.Is(err, ErrUnterminatedQuote) {
			t.Fatalf("SplitArgs(%q) error = %v, want ErrUnterminatedQuote", in, err)
		}
	}
}

func TestSplit(t *testing.T) {
	name, args, err := Split(`/EXPORT "a b.epub" --overwrite`)
	if err != nil {
		t.Fatal(err)
	}
	if name != "export" || len(args) != 2 || args[0] != "a b.epub" || args[1] != "--overwrite" {
		t.Fatalf("Split = %q %q", name, args)
	}
	if _, _, err := Split("hello"); !errors.Is(err, ErrNotCommand) {
		t.Fatalf("Split(non command) error = %v", err)
	}
	if _, _, err := Split("/"); !errors.Is(err, ErrNotCommand) {
		t.Fatalf("Split(/) error = %v", err)
	}
	if _, _, err := Split(`/export "a`); !errors.Is(err, ErrUnterminatedQuote) {
		t.Fatalf("Split(unterminated) error = %v", err)
	}
}

func TestParseInvocations(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantName   string
		wantValues map[string]string
		wantFlags  []string
	}{
		{name: "review on", line: "/review on", wantName: "review", wantValues: map[string]string{"mode": "on"}},
		{name: "review off upper", line: "/REVIEW off", wantName: "review", wantValues: map[string]string{"mode": "off"}},
		{name: "alias plan", line: "/plan", wantName: "cocreate"},
		{name: "model role", line: "/model writer", wantName: "model", wantValues: map[string]string{"role": "writer"}},
		{name: "model empty", line: "/model", wantName: "model"},
		{name: "start quoted path", line: `/start "ideas/y tuong.md"`, wantName: "start", wantValues: map[string]string{"path": "ideas/y tuong.md"}},
		{name: "reopen text", line: "/reopen viết thêm tập 2", wantName: "reopen", wantValues: map[string]string{"dir": "viết thêm tập 2"}},
		{name: "reopen quoted text", line: `/reopen "viết thêm tập 2"`, wantName: "reopen", wantValues: map[string]string{"dir": "viết thêm tập 2"}},
		{
			name:     "import all options",
			line:     `/import nhap/tra-xanh.txt --yes --story=open --continue --guide=Hồi thứ X cũng là chương`,
			wantName: "import",
			wantValues: map[string]string{
				"path":  "nhap/tra-xanh.txt",
				"story": "open",
				"guide": "Hồi thứ X cũng là chương",
			},
			wantFlags: []string{"yes", "cont"},
		},
		{name: "import guide only", line: `/import --guide="Hồi thứ X" "cũng là chương"`, wantName: "import", wantValues: map[string]string{"guide": "Hồi thứ X cũng là chương"}},
		{name: "import continue", line: "/import", wantName: "import"},
		{
			name:     "export full",
			line:     `/export "exports/tra xanh.epub" from=1 to=24 --overwrite`,
			wantName: "export",
			wantValues: map[string]string{
				"path": "exports/tra xanh.epub",
				"from": "1",
				"to":   "24",
			},
			wantFlags: []string{"ow"},
		},
		{name: "export empty", line: "/export", wantName: "export"},
		{name: "sync check", line: "/sync --check", wantName: "sync", wantFlags: []string{"check"}},
		{name: "sync plain", line: "/sync", wantName: "sync"},
		{name: "importsim quoted", line: `/importsim "van phong/co-trang.json"`, wantName: "importsim", wantValues: map[string]string{"path": "van phong/co-trang.json"}},
		{name: "plain commands", line: "/diag", wantName: "diag"},
		{name: "simulate", line: "/simulate", wantName: "simulate"},
		{name: "config", line: "/config", wantName: "config"},
		{name: "next", line: "/next", wantName: "next"},
		{name: "help", line: "/help", wantName: "help"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv, err := Parse(tt.line)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.line, err)
			}
			if inv.Name != tt.wantName {
				t.Fatalf("Parse(%q) name = %q, want %q", tt.line, inv.Name, tt.wantName)
			}
			for key, want := range tt.wantValues {
				if got := inv.Values[key]; got != want {
					t.Fatalf("Parse(%q) value[%s] = %q, want %q", tt.line, key, got, want)
				}
			}
			for _, key := range tt.wantFlags {
				if !inv.Flags[key] {
					t.Fatalf("Parse(%q) flag[%s] not set", tt.line, key)
				}
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		line     string
		want     string
		notUsage bool
	}{
		{line: "/review", want: "Cách dùng: /review on|off"},
		{line: "/review yes", want: "Cách dùng: /review on|off"},
		{line: "/review on off", want: "Cách dùng: /review on|off"},
		{line: "/config extra", want: "Cách dùng: /config"},
		{line: "/help extra", want: "Cách dùng: /help"},
		{line: "/model writter", want: "Cách dùng: /model [vai-trò]"},
		{line: "/start", want: "Cách dùng: /start <đường-dẫn-tệp>"},
		{line: "/importsim", want: "Cách dùng: /importsim <tệp-profile.json>"},
		{line: "/import --story=mo", want: "Cách dùng: " + findUsage("import")},
		{line: "/import --auto", want: "Cách dùng: " + findUsage("import")},
		{line: "/import a.txt b.txt", want: "Cách dùng: " + findUsage("import")},
		{line: "/import --guide=", want: "Cách dùng: " + findUsage("import")},
		{line: "/export from=abc", want: "Cách dùng: " + findUsage("export")},
		{line: "/export from=-1", want: "Cách dùng: " + findUsage("export")},
		{line: "/export a.txt b.txt", want: "Cách dùng: " + findUsage("export")},
		{line: "/export --force", want: "Cách dùng: " + findUsage("export")},
		{line: `/export "a.txt`, want: "Cách dùng: " + findUsage("export")},
		{line: "/sync --dry", want: "Cách dùng: /sync [--check]"},
		{line: "/exprt", want: "Lệnh không xác định: /exprt", notUsage: true},
		{line: "/", want: "Lệnh không xác định: /", notUsage: true},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			_, err := Parse(tt.line)
			if err == nil {
				t.Fatalf("Parse(%q) = nil error", tt.line)
			}
			if got := err.Error(); got != tt.want {
				t.Fatalf("Parse(%q) error = %q, want %q", tt.line, got, tt.want)
			}
			var usage *UsageError
			if tt.notUsage && errors.As(err, &usage) {
				t.Fatalf("Parse(%q) should not be a UsageError: %v", tt.line, err)
			}
			if !tt.notUsage && !errors.As(err, &usage) {
				t.Fatalf("Parse(%q) should be a UsageError: %v", tt.line, err)
			}
		})
	}
}

func TestParseNotCommand(t *testing.T) {
	if _, err := Parse("xin chào"); !errors.Is(err, ErrNotCommand) {
		t.Fatalf("Parse(non command) error = %v", err)
	}
}

func findUsage(name string) string {
	spec, _ := Find(name)
	return spec.Usage
}

func TestSpecsCoverEveryCommand(t *testing.T) {
	want := []string{"help", "model", "config", "diag", "review", "next", "start", "import", "reopen", "cocreate", "simulate", "importsim", "sync", "export"}
	specs := Specs()
	if len(specs) != len(want) {
		t.Fatalf("Specs() has %d commands, want %d", len(specs), len(want))
	}
	for i, name := range want {
		if specs[i].Name != name {
			t.Fatalf("Specs()[%d] = %q, want %q", i, specs[i].Name, name)
		}
		if !strings.HasPrefix(specs[i].Usage, "/"+name) {
			t.Fatalf("%s usage = %q", name, specs[i].Usage)
		}
		if specs[i].Description == "" {
			t.Fatalf("%s has no description", name)
		}
		for _, p := range specs[i].Params {
			switch p.Kind {
			case KindFlag:
				if p.Flag == "" || p.Prefix != "" {
					t.Fatalf("%s flag param %q = %+v", name, p.Key, p)
				}
			case KindEnum:
				if len(p.Options) == 0 {
					t.Fatalf("%s enum param %q has no options", name, p.Key)
				}
			case KindPath, KindNum, KindText:
			default:
				t.Fatalf("%s param %q has unknown kind %q", name, p.Key, p.Kind)
			}
			if p.Kind != KindFlag && p.Flag != "" {
				t.Fatalf("%s param %q has flag on non-flag kind", name, p.Key)
			}
		}
	}
}

func TestSpecPolicies(t *testing.T) {
	policies := map[string]struct {
		group     string
		auto      bool
		needsIdle bool
		only      string
	}{
		"help":      {group: GroupSys, auto: true},
		"model":     {group: GroupSys, auto: true},
		"config":    {group: GroupSys, auto: true},
		"diag":      {group: GroupWrite, auto: true},
		"review":    {group: GroupWrite},
		"next":      {group: GroupWrite, auto: true, needsIdle: true},
		"start":     {group: GroupBegin, only: OnlyWelcome},
		"import":    {group: GroupBegin, needsIdle: true},
		"reopen":    {group: GroupEnd, needsIdle: true},
		"cocreate":  {group: GroupWrite, auto: true, only: OnlyRunning},
		"simulate":  {group: GroupStyle, needsIdle: true},
		"importsim": {group: GroupStyle, needsIdle: true},
		"sync":      {group: GroupWrite, auto: true, needsIdle: true},
		"export":    {group: GroupEnd, auto: true},
	}
	for name, want := range policies {
		spec, ok := Find(name)
		if !ok {
			t.Fatalf("%s not found", name)
		}
		if spec.Group != want.group || spec.AutoExecute != want.auto || spec.NeedsIdle != want.needsIdle || spec.Only != want.only {
			t.Fatalf("%s policy = %+v, want %+v", name, spec, want)
		}
	}
	if spec, _ := Find("plan"); spec.Name != "cocreate" {
		t.Fatalf("alias plan resolves to %q", spec.Name)
	}
}

func TestSpecsJSONShape(t *testing.T) {
	raw, err := json.Marshal(Specs())
	if err != nil {
		t.Fatal(err)
	}
	var specs []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &specs); err != nil {
		t.Fatal(err)
	}
	if len(specs) != 14 {
		t.Fatalf("marshalled %d specs", len(specs))
	}
	for _, spec := range specs {
		for _, key := range []string{"name", "aliases", "usage", "group", "autoExecute", "needsIdle", "params"} {
			if _, ok := spec[key]; !ok {
				t.Fatalf("spec missing %q: %s", key, raw)
			}
		}
		if _, ok := spec["description"]; ok {
			t.Fatalf("spec must not expose description: %s", raw)
		}
	}
	if strings.Contains(string(raw), `"aliases":null`) || strings.Contains(string(raw), `"params":null`) {
		t.Fatalf("aliases/params must be [] not null: %s", raw)
	}
	only := map[string]bool{}
	for _, spec := range specs {
		var name string
		_ = json.Unmarshal(spec["name"], &name)
		if v, ok := spec["only"]; ok {
			var value string
			_ = json.Unmarshal(v, &value)
			only[name] = value == OnlyWelcome || value == OnlyRunning
		}
	}
	if !only["start"] || !only["cocreate"] {
		t.Fatalf("only flags = %v", only)
	}
	if len(only) != 2 {
		t.Fatalf("only present on %d specs, want 2", len(only))
	}
	var exportParams []Param
	for _, spec := range specs {
		var name string
		_ = json.Unmarshal(spec["name"], &name)
		if name == "export" {
			if err := json.Unmarshal(spec["params"], &exportParams); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(exportParams) != 7 {
		t.Fatalf("export params = %+v", exportParams)
	}
	if exportParams[1].Prefix != "from=" || exportParams[4].Flag != "--overwrite" || exportParams[5].Flag != "--keep" || exportParams[6].Flag != "doc-so" || exportParams[3].Prefix != "format=" || exportParams[1].Required {
		t.Fatalf("export params = %+v", exportParams)
	}
}
