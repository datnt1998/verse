// Một dòng về thứ làm chậm một lần dựng dài trên Mac: ít RAM trống, swap
// đang dùng (unified memory nghĩa là áp lực RAM GPU đổ về đây), throttle
// nhiệt/hiệu năng của macOS, và RAM các process mang marker (Chrome dựng và
// helper của nó) đang giữ.
package tools

import (
	"context"
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

const gb = 1024.0 * 1024 * 1024

var (
	swapUsedRe       = regexp.MustCompile(`used = ([\d.]+)([KMG])`)
	thermalIdleRe    = regexp.MustCompile(`^Note: No .* recorded\.?$`)
	memoryFreePctRe  = regexp.MustCompile(`free percentage: (\d+)%`)
	vmStatFreeRe     = regexp.MustCompile(`Pages free:\s+(\d+)`)
	vmStatPageSizeRe = regexp.MustCompile(`page size of (\d+) bytes`)
	psRssRe          = regexp.MustCompile(`^\s*(\d+)\s+(.*)$`)
)

// ParseSwapUsedGb đọc `sysctl vm.swapusage` → swap đang dùng, theo GB.
func ParseSwapUsedGb(output string) (float64, bool) {
	m := swapUsedRe.FindStringSubmatch(output)
	if m == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	switch m[2] {
	case "K":
		v /= 1024 * 1024
	case "M":
		v /= 1024
	}
	return v, true
}

// ParseThermalWarnings đọc `pmset -g therm`: mỗi kênh in dòng "No … recorded"
// tới khi macOS throttle; bất kỳ dòng nào khác là cảnh báo.
func ParseThermalWarnings(output string) []string {
	var out []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" && !thermalIdleRe.MatchString(line) {
			out = append(out, line)
		}
	}
	return out
}

// ParseMemoryFreePercent đọc `memory_pressure -Q` → phần trăm RAM trống của
// chính macOS (tính cả cache thu hồi được, khác freemem).
func ParseMemoryFreePercent(output string) (int, bool) {
	m := memoryFreePctRe.FindStringSubmatch(output)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	return n, err == nil
}

// ParseVmStatFreeGb đọc `vm_stat` → RAM trống theo GB (tương đương freemem).
func ParseVmStatFreeGb(output string) (float64, bool) {
	fm := vmStatFreeRe.FindStringSubmatch(output)
	if fm == nil {
		return 0, false
	}
	pages, err := strconv.ParseFloat(fm[1], 64)
	if err != nil {
		return 0, false
	}
	pageSize := 16384.0
	if pm := vmStatPageSizeRe.FindStringSubmatch(output); pm != nil {
		if v, err := strconv.ParseFloat(pm[1], 64); err == nil {
			pageSize = v
		}
	}
	return pages * pageSize / gb, true
}

// SumRssGb đọc `ps -axo rss=,command=` → tổng GB resident của process có
// command line chứa marker.
func SumRssGb(output, marker string) float64 {
	var kb float64
	for _, line := range strings.Split(output, "\n") {
		if m := psRssRe.FindStringSubmatch(line); m != nil && strings.Contains(m[2], marker) {
			if v, err := strconv.ParseFloat(m[1], 64); err == nil {
				kb += v
			}
		}
	}
	return kb / 1024 / 1024
}

// MachineLoad đo tải máy qua các lệnh hệ thống của Darwin.
type MachineLoad struct {
	// Command chạy lệnh đọc thông tin hệ thống; nil → dùng ps/sysctl/pmset
	// thật. Test cấp bảng trả lời giả.
	Command func(ctx context.Context, name string, args ...string) (string, error)
}

func (m MachineLoad) read(ctx context.Context, name string, args ...string) string {
	if m.Command != nil {
		if out, err := m.Command(ctx, name, args...); err == nil {
			return out
		}
		return ""
	}
	out, err := execProcessOutput(ctx, name, args...)
	if err != nil {
		return ""
	}
	return out
}

// Line trả một dòng tiếng Việt về tải máy cho bước render, theo
// machineLoadLine của bản cũ. processMarker lọc process để cộng RSS (đường
// dẫn profile Chrome của lần render, ví dụ).
func (m MachineLoad) Line(ctx context.Context, processMarker string) string {
	darwin := runtime.GOOS == "darwin"
	var ps string
	havePS := false
	if processMarker != "" {
		if out := m.read(ctx, "ps", "-axo", "rss=,command="); out != "" {
			ps, havePS = out, true
		}
	}
	var pressure, swap, therm string
	var havePressure, haveSwap, haveTherm bool
	if darwin {
		if out := m.read(ctx, "memory_pressure", "-Q"); out != "" {
			pressure, havePressure = out, true
		}
		if out := m.read(ctx, "sysctl", "-n", "vm.swapusage"); out != "" {
			swap, haveSwap = out, true
		}
		if out := m.read(ctx, "pmset", "-g", "therm"); out != "" {
			therm, haveTherm = out, true
		}
	}
	totalGb := totalMemGb()
	var parts []string
	if pct, ok := ParseMemoryFreePercent(pressure); ok && havePressure {
		parts = append(parts, fmt.Sprintf("RAM trống %d%% của %d GB", pct, totalGb))
	} else if out := m.read(ctx, "vm_stat"); out != "" {
		if free, ok := ParseVmStatFreeGb(out); ok {
			parts = append(parts, fmt.Sprintf("RAM trống %.1f/%d GB", free, totalGb))
		}
	} else {
		parts = append(parts, fmt.Sprintf("RAM trống không rõ/%d GB", totalGb))
	}
	if havePS && processMarker != "" {
		parts = append(parts, fmt.Sprintf("Chrome dựng %.1f GB", SumRssGb(ps, processMarker)))
	}
	if haveSwap {
		if used, ok := ParseSwapUsedGb(swap); ok {
			parts = append(parts, fmt.Sprintf("swap %.1f GB", used))
		}
	}
	if haveTherm {
		warnings := ParseThermalWarnings(therm)
		if len(warnings) > 0 {
			parts = append(parts, "nhiệt: "+strings.Join(warnings, "; "))
		} else {
			parts = append(parts, "nhiệt: bình thường")
		}
	}
	return strings.Join(parts, " · ")
}

func totalMemGb() int {
	if runtime.GOOS == "darwin" {
		if out, err := syscall.Sysctl("hw.memsize"); err == nil {
			var v uint64
			fmt.Sscanf(out, "%d", &v)
			return int((float64(v) / gb) + 0.5)
		}
	}
	return 0
}
