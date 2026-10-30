package storage

import (
	"fmt"
	"strings"
)

// formatBytes in dung lượng kiểu người Việt: "81,1 GB", "640 MB", "1,8 GB"
// (dấu phẩy thập phân; dưới 1 KB ghi byte).
func formatBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := float64(n)
	unit := 0
	for value >= 1000 && unit < len(units)-1 {
		value /= 1000
		unit++
	}
	text := fmt.Sprintf("%.1f", value)
	if unit == 0 {
		text = fmt.Sprintf("%d", n)
	}
	text = strings.TrimSuffix(text, ".0")
	return strings.ReplaceAll(text, ".", ",") + " " + units[unit]
}
