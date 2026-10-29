package version

import (
	"fmt"
	"io"
	"runtime/debug"
	"strings"
)

// ReleaseRepo is the repository Noverse releases are published to. The code
// moved from datnt1998/noverse (frozen) into the datnt1998/verse monorepo;
// all Go code pointing to releases (the update command, background
// new-version checks) uses this constant.
const ReleaseRepo = "datnt1998/verse"

type Info struct {
	Version string
	Commit  string
	Date    string
}

func Resolve(raw Info) Info {
	return Info{
		Version: displayVersion(raw.Version),
		Commit:  displayCommit(raw.Commit),
		Date:    displayDate(raw.Date),
	}
}

// buildInfo 是 main 启动时写入一次的构建身份，供 store/revision 等深层包给
// ChapterRecord 盖 build 戳，避免把版本参数穿进每个构造函数。
var buildInfo Info

// SetBuild 记录当前进程的构建身份；raw 为 ldflags 原始值（空/unknown 走
// Resolve 的 debug.BuildInfo 兜底），main 应在启动早期调用一次。
func SetBuild(raw Info) {
	buildInfo = Resolve(raw)
}

// Build 返回当前构建的戳记 "<version>+<short commit>"（commit 取前 7 位，
// 无 commit 时为 "unknown"；未调用 SetBuild 时按 dev/debug.BuildInfo 兜底）。
func Build() string {
	info := buildInfo
	if info.Version == "" {
		info = Resolve(Info{})
	}
	commit := info.Commit
	if commit == "" {
		commit = "unknown"
	}
	if len(commit) > 7 {
		commit = commit[:7]
	}
	return info.Version + "+" + commit
}

func Print(w io.Writer, info Info) {
	info = Resolve(info)
	fmt.Fprintf(w, "Noverse %s\ncommit: %s\nbuilt: %s\n", info.Version, info.Commit, info.Date)
}

func displayVersion(v string) string {
	v = Normalize(v)
	if v != "dev" {
		return v
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if mv := Normalize(info.Main.Version); mv != "dev" {
			return mv
		}
	}
	return "dev"
}

func Normalize(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "(devel)" {
		return "dev"
	}
	if v != "dev" && !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

func displayCommit(commit string) string {
	commit = strings.TrimSpace(commit)
	if commit != "" && commit != "unknown" {
		return commit
	}
	if v := buildSetting("vcs.revision"); v != "" {
		return v
	}
	return "unknown"
}

func displayDate(date string) string {
	date = strings.TrimSpace(date)
	if date != "" && date != "unknown" {
		return date
	}
	if v := buildSetting("vcs.time"); v != "" {
		return v
	}
	return "unknown"
}

func buildSetting(key string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, setting := range info.Settings {
		if setting.Key == key {
			return strings.TrimSpace(setting.Value)
		}
	}
	return ""
}
