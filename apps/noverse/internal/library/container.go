package library

import (
	"os"
	"path/filepath"
	"strings"
)

// IsContainer is an explicit image contract; native installs never inherit
// container-only restrictions from the browser's hostname or remote address.
func IsContainer() bool { return os.Getenv("NOVERSE_CONTAINER") == "1" }

func LibraryRootsRestricted() bool { return IsContainer() || BrowseRootsConfigured() }

const ephemeralLibraryWarning = "Thư mục này sẽ mất khi cập nhật container — hãy gắn volume."

// LibraryMountWarning fails visibly when mount metadata is unavailable. The
// overlay root and volatile pseudo-filesystems are not persistent data volumes.
func LibraryMountWarning(path string) string {
	if !IsContainer() || path == "" {
		return ""
	}
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil || !libraryPathMounted(path, string(data)) {
		return ephemeralLibraryWarning
	}
	return ""
}

func libraryPathMounted(path, mountinfo string) bool {
	real, err := ResolvePath(path)
	if err != nil {
		return false
	}
	unescape := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)
	bestLen, durable := 0, false
	for line := range strings.SplitSeq(mountinfo, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		sep := -1
		for i := 6; i < len(fields); i++ {
			if fields[i] == "-" {
				sep = i
				break
			}
		}
		if sep < 0 || sep+1 >= len(fields) {
			continue
		}
		mount := filepath.Clean(unescape.Replace(fields[4]))
		if mount == "/" || len(mount) < bestLen || (real != mount && !strings.HasPrefix(real, mount+string(filepath.Separator))) {
			continue
		}
		bestLen, durable = len(mount), true
		switch fields[sep+1] {
		case "overlay", "tmpfs", "ramfs", "proc", "sysfs", "devtmpfs", "devpts", "cgroup", "cgroup2", "mqueue", "securityfs":
			durable = false
		}
	}
	return durable
}
