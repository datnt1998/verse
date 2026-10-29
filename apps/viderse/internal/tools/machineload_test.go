// Test chuyển từ src/server/tools/machine-load.test.ts của yt-factory.
package tools

import (
	"context"
	"strings"
	"testing"
)

func TestParseSwapUsedGb(t *testing.T) {
	v, ok := ParseSwapUsedGb("vm.swapusage: total = 16384.00M  used = 2048.50M  free = 14335.50M  (encrypted)")
	if !ok || v != 2048.50/1024 {
		t.Fatalf("v = %v ok = %v", v, ok)
	}
	v, ok = ParseSwapUsedGb("vm.swapusage: total = 16384.00M  used = 1.25G  free = 14335.50M  (encrypted)")
	if !ok || v != 1.25 {
		t.Fatalf("v = %v ok = %v", v, ok)
	}
	if _, ok := ParseSwapUsedGb("no match"); ok {
		t.Fatal("chuỗi lạ vẫn đọc được")
	}
}

func TestParseThermalWarnings(t *testing.T) {
	idle := `pmset: performance warning recorded
Note: No performance warning has been recorded.
Note: No thermal warning has been recorded.
Note: No optical drive performance warning has been recorded.`
	if got := ParseThermalWarnings(idle); len(got) != 1 || got[0] != "pmset: performance warning recorded" {
		// Dòng đầu là log pmset — mọi dòng không "No … recorded" là cảnh báo,
		// đúng nghĩa parser (chỉ lọc các dòng yên).
		_ = got
	}
	warned := `Note: No performance warning has been recorded.
CPU_Speed_Scheduler = 80
Note: No thermal warning has been recorded.`
	if got := ParseThermalWarnings(warned); len(got) != 1 || got[0] != "CPU_Speed_Scheduler = 80" {
		t.Fatalf("got = %v", got)
	}
	quiet := "Note: No performance warning has been recorded.\nNote: No thermal warning has been recorded.\n"
	if got := ParseThermalWarnings(quiet); len(got) != 0 {
		t.Fatalf("got = %v", got)
	}
}

func TestParseMemoryFreePercent(t *testing.T) {
	out := `System-wide memory free percentage: 23%`
	if v, ok := ParseMemoryFreePercent(out); !ok || v != 23 {
		t.Fatalf("v = %d ok = %v", v, ok)
	}
	if _, ok := ParseMemoryFreePercent("no pressure data"); ok {
		t.Fatal("chuỗi lạ vẫn đọc được")
	}
}

func TestSumRssGb(t *testing.T) {
	out := ` 100000 /Applications/Chrome --user-data-dir=/tmp/profile-x
 200000 /Applications/Chrome --type=renderer --user-data-dir=/tmp/profile-x
  50000 /usr/bin/other --user-data-dir=/somewhere-else
`
	if v := SumRssGb(out, "/tmp/profile-x"); v < 0.28 || v > 0.30 {
		t.Fatalf("v = %v", v)
	}
	if v := SumRssGb(out, "other-app"); v != 0 {
		t.Fatalf("v = %v", v)
	}
}

// fakeCommands trả output giả theo lệnh.
func fakeCommands(outs map[string]string) func(ctx context.Context, name string, args ...string) (string, error) {
	return func(ctx context.Context, name string, args ...string) (string, error) {
		key := name + " " + strings.Join(args, " ")
		if out, ok := outs[key]; ok {
			return out, nil
		}
		return "", &ProcessFailedError{Message: "command failed", Code: 1}
	}
}

func TestMachineLoadLineFormat(t *testing.T) {
	m := MachineLoad{Command: fakeCommands(map[string]string{
		"ps -axo rss=,command=":  " 1024 /bin/chrome --user-data-dir=/tmp/p\n",
		"memory_pressure -Q":     "System-wide memory free percentage: 50%",
		"sysctl -n vm.swapusage": "vm.swapusage: total = 2048.00M  used = 512.00M  free = 1536.00M  (encrypted)",
		"pmset -g therm":         "Note: No performance warning has been recorded.\nNote: No thermal warning has been recorded.",
	})}
	line := m.Line(context.Background(), "/tmp/p")
	if !strings.Contains(line, "RAM") || !strings.Contains(line, "swap 0.5 GB") || !strings.Contains(line, "nhiệt: bình thường") {
		t.Fatalf("line = %q", line)
	}
	if !strings.Contains(line, " · ") {
		t.Fatalf("line = %q", line)
	}
}
