// Test chuyển từ src/server/tools/process-lifecycle.test.ts của yt-factory.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/platform/toolchain"
)

// fakeClock điều khiển thời gian ảo: Sleep tăng đồng hồ ngay nên ân hạn không
// tốn thời gian thật.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
	return nil
}

func (c *fakeClock) elapsed() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now.Sub(time.Time{})
}

// fakeGroup là một nhóm process giả: diesOn liệt kê tín hiệu làm nó chết.
type fakeGroup struct {
	pgid   int
	rows   []ProcessRow
	diesOn []Signal
}

type fakeOps struct {
	group   *fakeGroup
	signals []Signal
	alive   bool
}

func newFakeOps(group *fakeGroup) *fakeOps {
	return &fakeOps{group: group, alive: true}
}

func (f *fakeOps) Processes(ctx context.Context) ([]ProcessRow, error) {
	if !f.alive {
		return nil, nil
	}
	return f.group.rows, nil
}

func (f *fakeOps) SignalGroup(pgid int, sig Signal) error {
	if pgid != f.group.pgid {
		panic("chỉ nhóm mục tiêu được signal")
	}
	if !f.alive {
		return os.ErrNotExist
	}
	f.signals = append(f.signals, sig)
	for _, s := range f.group.diesOn {
		if s == sig {
			f.alive = false
		}
	}
	return nil
}

func (f *fakeOps) GroupAlive(pgid int) bool { return f.alive }

func chromeRows(pgid int) []ProcessRow {
	return []ProcessRow{
		{PID: pgid, PGID: pgid, Command: "/Applications/Chrome --user-data-dir=/data/chrome-profiles/req-1"},
		{PID: pgid + 1, PGID: pgid, Command: "/Applications/Chrome --type=renderer"},
	}
}

func TestGroupExitingOnSigtermNeverGetsSigkill(t *testing.T) {
	ops := newFakeOps(&fakeGroup{pgid: 4242, rows: chromeRows(4242), diesOn: []Signal{SigTerm, SigKill}})
	hit, err := TerminateGroup(context.Background(), GroupTarget{PGID: 4242}, TerminateOptions{
		Ops: ops, Clock: &fakeClock{}, Grace: 5 * time.Second,
	})
	if err != nil || !hit {
		t.Fatalf("hit=%v err=%v", hit, err)
	}
	if len(ops.signals) != 1 || ops.signals[0] != SigTerm {
		t.Fatalf("signals = %v", ops.signals)
	}
}

func TestGroupIgnoringSigtermKilledAfterGrace(t *testing.T) {
	ops := newFakeOps(&fakeGroup{pgid: 4242, rows: chromeRows(4242), diesOn: []Signal{SigKill}})
	clock := &fakeClock{}
	hit, err := TerminateGroup(context.Background(), GroupTarget{PGID: 4242}, TerminateOptions{
		Ops: ops, Clock: clock, Grace: 5 * time.Second,
	})
	if err != nil || !hit {
		t.Fatalf("hit=%v err=%v", hit, err)
	}
	if len(ops.signals) != 2 || ops.signals[0] != SigTerm || ops.signals[1] != SigKill {
		t.Fatalf("signals = %v", ops.signals)
	}
	if clock.elapsed() < 5*time.Second {
		t.Fatal("SIGKILL không chờ hết ân hạn")
	}
}

func TestIdentityCheckedGroupOnlySignalledOnMatch(t *testing.T) {
	mismatched := newFakeOps(&fakeGroup{pgid: 4242, rows: chromeRows(4242), diesOn: []Signal{SigTerm}})
	hit, err := TerminateGroup(context.Background(),
		GroupTarget{PGID: 4242, Identity: []string{"--user-data-dir=/data/chrome-profiles/req-2"}},
		TerminateOptions{Ops: mismatched, Clock: &fakeClock{}})
	if err != nil || hit {
		t.Fatalf("hit=%v err=%v", hit, err)
	}
	if len(mismatched.signals) != 0 {
		t.Fatal("pgid dùng lại với command khác vẫn bị signal")
	}

	matched := newFakeOps(&fakeGroup{pgid: 4242, rows: chromeRows(4242), diesOn: []Signal{SigTerm}})
	hit, err = TerminateGroup(context.Background(),
		GroupTarget{PGID: 4242, Identity: []string{"--user-data-dir=/data/chrome-profiles/req-1", "Chrome"}},
		TerminateOptions{Ops: matched, Clock: &fakeClock{}})
	if err != nil || !hit {
		t.Fatalf("hit=%v err=%v", hit, err)
	}
	if len(matched.signals) != 1 || matched.signals[0] != SigTerm {
		t.Fatalf("signals = %v", matched.signals)
	}
}

// multiFakeOps trả lời bảng process theo lượt đọc — giả pgid bị dùng lại.
type multiFakeOps struct {
	reads   int
	first   []ProcessRow
	rest    []ProcessRow
	signals []Signal
}

func (m *multiFakeOps) Processes(ctx context.Context) ([]ProcessRow, error) {
	m.reads++
	if m.reads == 1 {
		return m.first, nil
	}
	return m.rest, nil
}

func (m *multiFakeOps) SignalGroup(pgid int, sig Signal) error {
	m.signals = append(m.signals, sig)
	return nil
}

func (m *multiFakeOps) GroupAlive(pgid int) bool { return true }

func TestIdentityReprovenBeforeSigkill(t *testing.T) {
	// Đọc đầu: nhóm là của ta. Đọc sau: id thuộc thứ khác → không SIGKILL.
	ops := &multiFakeOps{
		first: chromeRows(4242),
		rest:  []ProcessRow{{PID: 4242, PGID: 4242, Command: "unrelated"}},
	}
	_, err := TerminateGroup(context.Background(),
		GroupTarget{PGID: 4242, Identity: []string{"--user-data-dir=/data/chrome-profiles/req-1"}},
		TerminateOptions{Ops: ops, Clock: &fakeClock{}, Grace: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if len(ops.signals) != 1 || ops.signals[0] != SigTerm {
		t.Fatalf("signals = %v", ops.signals)
	}
}

func TestOwnProcessAndSystemGroupIdsNeverSignalled(t *testing.T) {
	own := newFakeOps(&fakeGroup{pgid: 4242, rows: []ProcessRow{
		{PID: os.Getpid(), PGID: 4242, Command: "app --user-data-dir=x"},
	}, diesOn: []Signal{SigTerm}})
	hit, err := TerminateGroup(context.Background(),
		GroupTarget{PGID: 4242, Identity: []string{"--user-data-dir=x"}},
		TerminateOptions{Ops: own, Clock: &fakeClock{}})
	if err != nil || hit {
		t.Fatalf("hit=%v err=%v", hit, err)
	}
	if len(own.signals) != 0 {
		t.Fatal("nhóm chứa chính mình vẫn bị signal")
	}
	for _, pgid := range []int{0, 1, -5} {
		none := newFakeOps(&fakeGroup{pgid: pgid})
		hit, err := TerminateGroup(context.Background(), GroupTarget{PGID: pgid},
			TerminateOptions{Ops: none, Clock: &fakeClock{}})
		if err != nil || hit {
			t.Fatalf("pgid %d: hit=%v err=%v", pgid, hit, err)
		}
		if len(none.signals) != 0 {
			t.Fatalf("pgid hệ thống %d bị signal", pgid)
		}
	}
}

// realTerminateGroup gọi TerminateGroup trên một nhóm thật qua helper nhỏ
// bọc ops + khửa pgid từ con đã spawn.
func terminateChildForTest(t *testing.T, pid int, opts TerminateOptions) bool {
	t.Helper()
	hit, err := TerminateGroup(context.Background(), GroupTarget{PGID: pid}, opts)
	if err != nil {
		t.Fatal(err)
	}
	return hit
}

func TestTerminateChildStopsRealSigtermIgnoringTree(t *testing.T) {
	if testing.Short() || isWindows() {
		t.Skip("POSIX")
	}
	// Leader bỏ qua SIGTERM và nở một con cháu cũng bỏ qua: chỉ SIGKILL cả
	// nhóm mới dứt cả hai.
	program := `trap '' TERM; sh -c 'trap "" TERM; while :; do sleep 1; done' & echo ready; while :; do sleep 1; done`
	cmd := exec.Command(testShell, "-c", program)
	makeDetached(cmd)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64)
	_, _ = out.Read(buf) // đợi "ready"
	if !groupAlive(cmd.Process.Pid) {
		t.Fatal("nhóm chưa sống")
	}
	if !terminateChildForTest(t, cmd.Process.Pid, TerminateOptions{Grace: 300 * time.Millisecond, Reap: 5 * time.Second}) {
		t.Fatal("terminateGroup trả false")
	}
	_ = cmd.Wait()
	if groupAlive(cmd.Process.Pid) {
		t.Fatal("thành viên nhóm còn sống")
	}
}

func TestTerminateChildLetsSigtermHandlerFinish(t *testing.T) {
	if isWindows() {
		t.Skip("POSIX")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "finalized")
	program := fmt.Sprintf(`trap 'echo done > %s; exit 0' TERM; echo ready; while :; do sleep 1; done`, marker)
	cmd := exec.Command(testShell, "-c", program)
	makeDetached(cmd)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64)
	_, _ = out.Read(buf)
	if !terminateChildForTest(t, cmd.Process.Pid, TerminateOptions{Grace: 10 * time.Second}) {
		t.Fatal("terminateGroup trả false")
	}
	_ = cmd.Wait()
	data, err := os.ReadFile(marker)
	if err != nil || string(data) != "done\n" {
		t.Fatalf("handler SIGTERM không chạy xong: %q %v", data, err)
	}
}

func isWindows() bool { return testShell == "cmd" || os.PathSeparator == '\\' }

func writeRegistry(t *testing.T, baseDir string, ownerPID int, groups []GroupRecord) string {
	t.Helper()
	dir := filepath.Join(baseDir, registryDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, fmt.Sprintf("%d.json", ownerPID))
	data, _ := json.Marshal(registryFile{OwnerPID: ownerPID, Groups: groups})
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

// pairedFakeOps giả hai nhóm trong cùng một bảng process.
type pairedFakeOps struct {
	a, b *fakeOps
}

func (p *pairedFakeOps) Processes(ctx context.Context) ([]ProcessRow, error) {
	rowsA, _ := p.a.Processes(ctx)
	rowsB, _ := p.b.Processes(ctx)
	return append(rowsA, rowsB...), nil
}

func (p *pairedFakeOps) SignalGroup(pgid int, sig Signal) error {
	if pgid == p.a.group.pgid {
		return p.a.SignalGroup(pgid, sig)
	}
	return p.b.SignalGroup(pgid, sig)
}

func (p *pairedFakeOps) GroupAlive(pgid int) bool {
	if pgid == p.a.group.pgid {
		return p.a.GroupAlive(pgid)
	}
	return p.b.GroupAlive(pgid)
}

func TestReapingStopsRecordedGroupsOfDeadOwnersOnly(t *testing.T) {
	baseDir := t.TempDir()
	reg := NewRegistry(baseDir, nil)
	ours := newFakeOps(&fakeGroup{pgid: 5001, rows: chromeRows(5001), diesOn: []Signal{SigTerm}})
	reused := newFakeOps(&fakeGroup{pgid: 5002, rows: []ProcessRow{
		{PID: 5002, PGID: 5002, Command: "something else entirely"},
	}, diesOn: []Signal{SigTerm}})
	ops := &pairedFakeOps{a: ours, b: reused}

	ownerOfBoth := writeRegistry(t, baseDir, 70001, []GroupRecord{
		{PGID: 5001, Match: "--user-data-dir=/data/chrome-profiles/req-1", Label: "export-chrome"},
		{PGID: 5002, Match: "--user-data-dir=/data/chrome-profiles/req-9", Label: "export-chrome"},
	})
	liveOwner := writeRegistry(t, baseDir, 70002, []GroupRecord{
		{PGID: 5001, Match: "Chrome", Label: "export-chrome"},
	})

	ownerAlive := func(pid int) bool { return pid == 70002 }
	ctx := context.Background()

	dry, err := reg.ReapOrphaned(ctx, true, ownerAlive, TerminateOptions{Ops: ops, Clock: &fakeClock{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(dry) != 1 || dry[0].PGID != 5001 {
		t.Fatalf("dry = %+v", dry)
	}
	if len(ours.signals) != 0 {
		t.Fatal("dry run vẫn signal")
	}
	if !toolchain.FileExists(ownerOfBoth) {
		t.Fatal("dry run xoá bản ghi")
	}

	reaped, err := reg.ReapOrphaned(ctx, false, ownerAlive, TerminateOptions{Ops: ops, Clock: &fakeClock{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(reaped) != 1 || reaped[0].PGID != 5001 {
		t.Fatalf("reaped = %+v", reaped)
	}
	if len(ours.signals) != 1 || ours.signals[0] != SigTerm {
		t.Fatalf("signals = %v", ours.signals)
	}
	if len(reused.signals) != 0 {
		t.Fatal("pgid dùng lại bị signal")
	}
	if toolchain.FileExists(ownerOfBoth) {
		t.Fatal("bản ghi đã xử lý còn sót")
	}
	if !toolchain.FileExists(liveOwner) {
		t.Fatal("bản ghi của owner còn sống bị đụng")
	}
}

func TestReapingOneOwnerStopsThatOwnersGroups(t *testing.T) {
	baseDir := t.TempDir()
	reg := NewRegistry(baseDir, nil)
	tool := newFakeOps(&fakeGroup{pgid: 6001, rows: []ProcessRow{
		{PID: 6001, PGID: 6001, Command: "/usr/bin/ffmpeg -i in.mp4"},
	}, diesOn: []Signal{SigTerm}})
	file := writeRegistry(t, baseDir, 70010, []GroupRecord{
		{PGID: 6001, Match: "/usr/bin/ffmpeg", Label: "ffmpeg"},
	})
	reaped, err := reg.ReapOwner(context.Background(), 70010, false, TerminateOptions{Ops: tool, Clock: &fakeClock{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(reaped) != 1 || reaped[0].Label != "ffmpeg" {
		t.Fatalf("reaped = %+v", reaped)
	}
	if toolchain.FileExists(file) {
		t.Fatal("bản ghi còn sót")
	}
}

func TestTrackedChildRecordedThenForgottenOnceGroupGone(t *testing.T) {
	if isWindows() {
		t.Skip("POSIX")
	}
	baseDir := t.TempDir()
	reg := NewRegistry(baseDir, nil)
	cmd := exec.Command(testShell, "-c", "while :; do sleep 1; done")
	makeDetached(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	reg.Track(cmd.Process.Pid, "sleep", "probe")
	file := filepath.Join(baseDir, registryDirName, fmt.Sprintf("%d.json", os.Getpid()))
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var record registryFile
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if record.OwnerPID != os.Getpid() {
		t.Fatalf("ownerPid = %d", record.OwnerPID)
	}
	if len(record.Groups) != 1 || record.Groups[0].PGID != cmd.Process.Pid || record.Groups[0].Label != "probe" {
		t.Fatalf("groups = %+v", record.Groups)
	}

	if !terminateChildForTest(t, cmd.Process.Pid, TerminateOptions{Grace: 5 * time.Second}) {
		t.Fatal("terminateGroup trả false")
	}
	_ = cmd.Wait()
	reg.UntrackForget(cmd.Process.Pid)
	if toolchain.FileExists(file) {
		t.Fatal("bản ghi sống lâu hơn nhóm của nó")
	}
}
