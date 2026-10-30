//go:build unix

package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// signalGroup gửi tín hiệu cho cả nhóm process (pgid âm theo quy ước kill).
func signalGroup(pgid int, sig Signal) error {
	return syscall.Kill(-pgid, syscall.Signal(sig))
}

// groupAlive đúng khi nhóm còn tồn tại; EPERM vẫn nghĩa là "tồn tại".
func groupAlive(pgid int) bool {
	err := syscall.Kill(-pgid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// pidAlive đúng khi process còn tồn tại; EPERM vẫn nghĩa là "tồn tại".
func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// makeDetached đặt process con vào nhóm process riêng (pgid = pid của nó).
func makeDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// execProcessOutput chạy một lệnh đọc thông tin hệ thống và trả stdout.
func execProcessOutput(ctx context.Context, name string, args ...string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return out.String(), nil
}

// freeDiskBytes trả số byte trống của filesystem chứa dir (leo lên tổ tiên
// gần nhất còn tồn tại). -1 khi không đọc được.
func freeDiskBytes(dir string) int64 {
	probe := dir
	for range 10 {
		if _, err := os.Stat(probe); err == nil {
			break
		}
		parent := parentDir(probe)
		if parent == probe {
			break
		}
		probe = parent
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(probe, &st); err != nil {
		return -1
	}
	return int64(st.Bavail) * int64(st.Bsize)
}

// FreeDiskBytes trả số byte trống của filesystem chứa dir (leo lên tổ tiên
// gần nhất còn tồn tại). -1 khi không đọc được.
func FreeDiskBytes(dir string) int64 { return freeDiskBytes(dir) }

// fileIdentity nhận dạng ổn định của một tệp trên đĩa (device + inode + kích
// cỡ + mtime): đổi nội dung hay thay tệp đều đổi identity.
func fileIdentity(path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return "", false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano()), true
	}
	return fmt.Sprintf("%d:%d:%d:%d", st.Dev, st.Ino, st.Size, st.Mtimespec.Nano()), true
}

// setBackgroundPriority hạ độ ưu tiên CPU của process con (nice); lỗi bị bỏ
// qua vì ưu tiên chỉ là tối ưu, không phải thất bại.
func setBackgroundPriority(pid int, nice int) {
	_ = syscall.Setpriority(syscall.PRIO_PROCESS, pid, nice)
}
