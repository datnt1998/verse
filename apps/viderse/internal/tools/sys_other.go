//go:build !unix

package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

var errNoGroupSignals = errors.New("nhóm process không được hỗ trợ trên nền tảng này")

func signalGroup(pgid int, sig Signal) error { return errNoGroupSignals }

// groupAlive không có cách portable ngoài unix; báo sống để bộ dọn không giết nhầm.
func groupAlive(pgid int) bool { return true }
func pidAlive(pid int) bool    { return true }

func makeDetached(cmd *exec.Cmd) {}

func execProcessOutput(ctx context.Context, name string, args ...string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, name, args...).Output()
	return string(out), err
}

func fileIdentity(path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano()), true
}

func setBackgroundPriority(pid int, nice int) {}
