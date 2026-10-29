// Chính sách duy nhất để kết thúc process con, và sổ đăng ký để process kế
// tiếp dọn nốt khi process này chết giữa chừng.
//
// Kết thúc: SIGTERM, chờ một khoảng ân hạn, rồi SIGKILL — luôn nhắm vào cả
// nhóm process (mọi con chạy lâu đều vào nhóm riêng bằng Setpgid, nên nhóm là
// đơn vị sở hữu các con cháu). Ai chỉ cầm pgid đã ghi phải chứng minh danh
// tính: một thành viên của nhóm phải chứa đủ mọi mảnh command-line đã ghi,
// nếu không pgid có thể đã bị dùng lại và không gì được signal. Bằng chứng
// được đọc lại trước SIGKILL.
//
// Sổ đăng ký: mỗi nhóm spawn được ghi vào
// <data>/process-groups/<ownerPid>.json ({ownerPid, groups:[{pgid, match,
// label}]}); ReapOrphaned kết thúc nhóm của các owner không còn chạy, với
// cùng phép chứng minh danh tính.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// TerminateGrace là khoảng mặc định giữa SIGTERM và SIGKILL.
	TerminateGrace = 5 * time.Second
	reapWait       = 1 * time.Second
	pollInterval   = 50 * time.Millisecond
)

// Signal là tín hiệu POSIX gửi cho cả nhóm process.
type Signal int

const (
	SigTerm Signal = 15
	SigKill Signal = 9
)

// ProcessRow là một hàng của bảng process.
type ProcessRow struct {
	PID     int
	PGID    int
	Command string
}

// ProcessOps là bảng process và quyền signal — test cấp bảng giả.
type ProcessOps interface {
	Processes(ctx context.Context) ([]ProcessRow, error)
	// SignalGroup gửi tín hiệu cho cả nhóm; lỗi ESRCH khi nhóm đã mất.
	SignalGroup(pgid int, sig Signal) error
	// GroupAlive đúng khi nhóm còn thành viên (kể cả khi leader đã thoát).
	GroupAlive(pgid int) bool
}

// SystemOps dùng ps + kill thật của máy.
type SystemOps struct{}

// Processes đọc `ps -A -ww -o pid=,pgid=,command=`.
func (SystemOps) Processes(ctx context.Context) ([]ProcessRow, error) {
	out, err := execProcessOutput(ctx, "ps", "-A", "-ww", "-o", "pid=,pgid=,command=")
	if err != nil {
		return nil, err
	}
	var rows []ProcessRow
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		pgid, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			continue
		}
		rows = append(rows, ProcessRow{PID: pid, PGID: pgid, Command: strings.Join(fields[2:], " ")})
	}
	return rows, nil
}

func (SystemOps) SignalGroup(pgid int, sig Signal) error { return signalGroup(pgid, sig) }
func (SystemOps) GroupAlive(pgid int) bool               { return groupAlive(pgid) }

// Clock cho phép test điều khiển thời gian chờ.
type Clock interface {
	Now() time.Time
	Sleep(ctx context.Context, d time.Duration) error
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }
func (systemClock) Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// systemClock là Clock mặc định khi caller không inject (Clock nil).

// GroupTarget xác định một nhóm cần kết thúc. Identity là các mảnh mà một
// command-line của thành viên phải chứa đủ; để trống khi người gọi sở hữu
// nhóm qua process con mình spawn.
type GroupTarget struct {
	PGID     int
	Identity []string
}

// TerminateOptions chỉnh nhịp kết thúc nhóm.
type TerminateOptions struct {
	Grace time.Duration // giữa SIGTERM và SIGKILL; 0 → TerminateGrace
	Reap  time.Duration // chờ nhóm biến mất sau SIGKILL; 0 → 1s
	Clock Clock         // nil → đồng hồ hệ thống
	Ops   ProcessOps    // nil → SystemOps
}

func identityProven(ctx context.Context, ops ProcessOps, target GroupTarget) (bool, error) {
	if len(target.Identity) == 0 {
		return true, nil
	}
	rows, err := ops.Processes(ctx)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if row.PGID != target.PGID {
			continue
		}
		// Nhóm chứa chính process này không bao giờ là con lạc: signal nó sẽ
		// tự dừng mình.
		if row.PID == os.Getpid() {
			return false, nil
		}
	}
	for _, row := range rows {
		if row.PGID != target.PGID {
			continue
		}
		all := true
		for _, part := range target.Identity {
			if !strings.Contains(row.Command, part) {
				all = false
				break
			}
		}
		if all {
			return true, nil
		}
	}
	return false, nil
}

func trySignal(ops ProcessOps, pgid int, sig Signal) bool {
	return ops.SignalGroup(pgid, sig) == nil
}

func waitGone(ctx context.Context, ops ProcessOps, clock Clock, pgid int, d time.Duration) (bool, error) {
	deadline := clock.Now().Add(d)
	for ops.GroupAlive(pgid) && clock.Now().Before(deadline) {
		if err := clock.Sleep(ctx, pollInterval); err != nil {
			return !ops.GroupAlive(pgid), err
		}
	}
	return !ops.GroupAlive(pgid), nil
}

// TerminateGroup kết thúc một nhóm: SIGTERM → ân hạn → SIGKILL. Trả về khi
// nhóm đã mất (hoặc SIGKILL đã hết cửa sổ reap), nên caller xoá được thứ mà
// process đang giữ. Trả false khi không signal nào được gửi — nhóm đã mất
// hoặc danh tính không chứng minh được.
func TerminateGroup(ctx context.Context, target GroupTarget, opts TerminateOptions) (bool, error) {
	ops := opts.Ops
	if ops == nil {
		ops = SystemOps{}
	}
	clock := opts.Clock
	if clock == nil {
		clock = systemClock{}
	}
	grace := opts.Grace
	if grace <= 0 {
		grace = TerminateGrace
	}
	reap := opts.Reap
	if reap <= 0 {
		reap = reapWait
	}
	pgid := target.PGID
	// pgid 0/1 sẽ nhắm cả nhóm của caller hoặc của hệ thống.
	if pgid <= 1 || !ops.GroupAlive(pgid) {
		return false, nil
	}
	ok, err := identityProven(ctx, ops, target)
	if err != nil || !ok {
		return false, err
	}
	if !trySignal(ops, pgid, SigTerm) {
		return false, nil
	}
	gone, err := waitGone(ctx, ops, clock, pgid, grace)
	if err != nil {
		return true, err
	}
	if !gone && ops.GroupAlive(pgid) {
		// Nhóm có thể đã rỗng và pgid bị dùng lại trong lúc chờ.
		ok, err = identityProven(ctx, ops, target)
		if err != nil {
			return true, err
		}
		if ok {
			trySignal(ops, pgid, SigKill)
			_, _ = waitGone(ctx, ops, clock, pgid, reap)
		}
	}
	return true, nil
}

/* --------------------------------- sổ đăng ký --------------------------------- */

// GroupRecord là một nhóm đã ghi: pgid + mảnh command-line làm bằng chứng
// danh tính cho bộ dọn.
type GroupRecord struct {
	PGID  int    `json:"pgid"`
	Match string `json:"match"`
	Label string `json:"label"`
}

// ReapedGroup là một nhóm đã bị bộ dọn kết thúc.
type ReapedGroup struct {
	OwnerPID int
	PGID     int
	Label    string
}

type registryFile struct {
	OwnerPID int           `json:"ownerPid"`
	Groups   []GroupRecord `json:"groups"`
}

const registryDirName = "process-groups"

// Registry ghi các nhóm process mà process này spawn; process sau (hoặc lần
// chạy sau) dùng sổ này để dọn nhóm mồ côi. Ghi trạng thái ra đĩa nguyên tử;
// sổ chỉ là lưới an toàn nên lỗi ghi không làm hỏng việc spawn nó bảo vệ.
type Registry struct {
	dir    string // thư mục process-groups
	ops    ProcessOps
	mu     sync.Mutex
	groups map[int]GroupRecord
}

// NewRegistry mở sổ đăng ký dưới <baseDir>/process-groups. ops nil → SystemOps.
func NewRegistry(baseDir string, ops ProcessOps) *Registry {
	if ops == nil {
		ops = SystemOps{}
	}
	return &Registry{dir: filepath.Join(baseDir, registryDirName), ops: ops, groups: map[int]GroupRecord{}}
}

// persist ghi sổ của process này ra đĩa (atomic qua tệp tạm + rename); gọi
// với mu đã giữ. Lỗi bị nuốt — đĩa đầy không được phép làm hỏng spawn.
func (r *Registry) persist() {
	file := filepath.Join(r.dir, fmt.Sprintf("%d.json", os.Getpid()))
	if len(r.groups) == 0 {
		_ = os.Remove(file)
		return
	}
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return
	}
	groups := make([]GroupRecord, 0, len(r.groups))
	for _, g := range r.groups {
		groups = append(groups, g)
	}
	data, err := json.Marshal(registryFile{OwnerPID: os.Getpid(), Groups: groups})
	if err != nil {
		return
	}
	staged := file + ".tmp"
	if err := os.WriteFile(staged, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(staged, file)
}

// Track ghi nhóm của một process con đã spawn (pgid = pid của leader). Khi
// nhóm biến mất bản ghi tự xoá; leader thoát mà con cháu còn sống thì bản ghi
// vẫn ở lại cho bộ dọn.
func (r *Registry) Track(pgid int, match, label string) {
	if pgid <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked()
	r.groups[pgid] = GroupRecord{PGID: pgid, Match: match, Label: label}
	r.persist()
}

// UntrackForget xoá bản ghi của nhóm khi process biết chắc nhóm đã mất.
func (r *Registry) UntrackForget(pgid int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.groups[pgid]; ok {
		delete(r.groups, pgid)
		r.persist()
	}
}

// Prune bỏ bản ghi của các nhóm đã mất, để pgid bị dùng lại không bị nhầm là
// của mình.
func (r *Registry) Prune() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked()
}

func (r *Registry) pruneLocked() {
	changed := false
	for pgid := range r.groups {
		if !r.ops.GroupAlive(pgid) {
			delete(r.groups, pgid)
			changed = true
		}
	}
	if changed {
		r.persist()
	}
}

func readRegistryFile(file string) *registryFile {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var rf registryFile
	if err := json.Unmarshal(data, &rf); err != nil || rf.OwnerPID <= 0 {
		return nil
	}
	return &rf
}

// ReapOwner kết thúc mọi nhóm owner đã ghi (có kiểm danh tính) rồi xoá bản
// ghi. dryRun chỉ báo cáo. Dùng khi owner được biết đã chết hoặc vừa bị dừng.
func (r *Registry) ReapOwner(ctx context.Context, ownerPID int, dryRun bool, opts TerminateOptions) ([]ReapedGroup, error) {
	ops := opts.Ops
	if ops == nil {
		ops = r.ops
	}
	opts.Ops = ops
	file := filepath.Join(r.dir, fmt.Sprintf("%d.json", ownerPID))
	parsed := readRegistryFile(file)
	var reaped []ReapedGroup
	if parsed != nil {
		for _, g := range parsed.Groups {
			target := GroupTarget{PGID: g.PGID, Identity: []string{g.Match}}
			var hit bool
			if dryRun {
				if !ops.GroupAlive(g.PGID) {
					continue
				}
				ok, err := identityProven(ctx, ops, target)
				if err != nil {
					return reaped, err
				}
				hit = ok
			} else {
				var err error
				hit, err = TerminateGroup(ctx, target, opts)
				if err != nil {
					return reaped, err
				}
			}
			if hit {
				reaped = append(reaped, ReapedGroup{OwnerPID: ownerPID, PGID: g.PGID, Label: g.Label})
			}
		}
	}
	if !dryRun {
		_ = os.Remove(file)
	}
	return reaped, nil
}

// ReapOrphaned kết thúc nhóm đã ghi của mọi owner không còn chạy (dọn lúc
// khởi động). Bản ghi không đọc được bị xoá. ownerAlive nil → dùng kill -0.
func (r *Registry) ReapOrphaned(ctx context.Context, dryRun bool, ownerAlive func(pid int) bool, opts TerminateOptions) ([]ReapedGroup, error) {
	if ownerAlive == nil {
		ownerAlive = pidAlive
	}
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, nil
	}
	var reaped []ReapedGroup
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		ownerPID, err := strconv.Atoi(strings.TrimSuffix(name, ".json"))
		if err != nil {
			continue
		}
		if ownerPID == os.Getpid() || ownerAlive(ownerPID) {
			continue
		}
		got, err := r.ReapOwner(ctx, ownerPID, dryRun, opts)
		if err != nil {
			return reaped, err
		}
		reaped = append(reaped, got...)
	}
	return reaped, nil
}
