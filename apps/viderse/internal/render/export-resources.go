package render

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/platform/toolchain"
)

// LogExportResources observes the Mac production host without changing memory,
// process priority or storage. Callers bound sampling to once per 30 seconds and
// terminal transitions; unavailable probes are omitted, never reported as zero.
func LogExportResources(ctx context.Context, request ExportRequest) {
	if runtime.GOOS != "darwin" {
		return
	}
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	attrs := []any{"request_id", request.ID, "status", request.Status}
	if request.Progress != nil {
		attrs = append(attrs, "phase", request.Progress.Phase)
		if request.Progress.FramesDone != nil {
			attrs = append(attrs, "frames", *request.Progress.FramesDone)
		}
		if request.Progress.BytesSent != nil {
			attrs = append(attrs, "uploaded_bytes", *request.Progress.BytesSent)
		}
	}
	if started, err := time.Parse(time.RFC3339Nano, request.CreatedAt); err == nil {
		attrs = append(attrs, "elapsed_ms", time.Since(started).Milliseconds())
	}
	chromePID := 0
	path := os.TempDir()
	if request.Chrome != nil {
		chromePID = request.Chrome.PID
		path = request.Chrome.ProfileDir
	}
	if free := toolchain.FreeDiskBytes(path); free >= 0 {
		attrs = append(attrs, "scratch_free_bytes", free)
	}
	// Numeric process columns only: never read command arguments containing the
	// request token. RSS sums shared pages and is not unique physical memory.
	if out, err := exec.CommandContext(probeCtx, "/bin/ps", "-axo", "pid=,ppid=,rss=").Output(); err == nil {
		appRSS, chromeRSS := exportProcessRSS(string(out), os.Getpid(), chromePID)
		if appRSS >= 0 {
			attrs = append(attrs, "app_rss_bytes", appRSS)
		}
		if chromeRSS >= 0 {
			attrs = append(attrs, "chrome_tree_rss_bytes", chromeRSS)
		}
	}
	if out, err := exec.CommandContext(probeCtx, "/usr/sbin/sysctl", "-n", "vm.swapusage").Output(); err == nil {
		if used, ok := exportSwapUsed(string(out)); ok {
			attrs = append(attrs, "swap_used_bytes", used)
		}
	}
	slog.InfoContext(ctx, "export_resources", attrs...)
}

func exportProcessRSS(output string, appPID, chromePID int) (int64, int64) {
	type process struct {
		pid, parent int
		rss         int64
	}
	var rows []process
	appRSS, chromeRSS := int64(-1), int64(-1)
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		pid, e1 := strconv.Atoi(fields[0])
		parent, e2 := strconv.Atoi(fields[1])
		rss, e3 := strconv.ParseInt(fields[2], 10, 64)
		if e1 != nil || e2 != nil || e3 != nil || pid <= 0 || rss < 0 {
			continue
		}
		rows = append(rows, process{pid, parent, rss * 1024})
		if pid == appPID {
			appRSS = rss * 1024
		}
		if pid == chromePID {
			chromeRSS = 0
		}
	}
	if chromeRSS < 0 {
		return appRSS, chromeRSS
	}
	owned := map[int]bool{chromePID: true}
	for changed := true; changed; {
		changed = false
		for _, row := range rows {
			if !owned[row.pid] && owned[row.parent] {
				owned[row.pid] = true
				changed = true
			}
		}
	}
	for _, row := range rows {
		if owned[row.pid] {
			chromeRSS += row.rss
		}
	}
	return appRSS, chromeRSS
}

func exportSwapUsed(output string) (int64, bool) {
	fields := strings.Fields(output)
	for i := 0; i+2 < len(fields); i++ {
		if fields[i] != "used" || fields[i+1] != "=" {
			continue
		}
		value := fields[i+2]
		if len(value) < 2 {
			return 0, false
		}
		multiplier := float64(1)
		switch value[len(value)-1] {
		case 'K':
			multiplier = 1024
		case 'M':
			multiplier = 1024 * 1024
		case 'G':
			multiplier = 1024 * 1024 * 1024
		default:
			return 0, false
		}
		number, err := strconv.ParseFloat(value[:len(value)-1], 64)
		if err != nil || number < 0 || number != number || number > float64(1<<63-1)/multiplier {
			return 0, false
		}
		return int64(number * multiplier), true
	}
	return 0, false
}
