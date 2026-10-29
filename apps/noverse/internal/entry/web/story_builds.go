package web

import (
	"slices"
	"strings"
	"time"

	"github.com/datnt1998/noverse/internal/host"
)

// StoryBuildRange 是 D1 “Viết bằng phiên bản” 的一行：一段连续章节由同一构建写出。
// Build 戳（ChapterRecord.Build）形如 "<version>+<short commit>"（version.Build）：
// Version 去掉 "+commit" 后缀并去掉前导 "v"（artboard 写 "Noverse 0.9.2"），
// Commit 是短哈希；记录缺失、读坏、旧格式（v1 无 build）或 commit 为 "unknown"
// 时对应字段为空串——UI 显示“Không rõ”.
type StoryBuildRange struct {
	From    int    `json:"from"`
	To      int    `json:"to"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	// At 是这段第一项 bản ghi nghiệm thu 的受理时间（RFC3339），未知时为空串。
	At string `json:"at"`
}

// splitBuild 把 "<version>+<commit>" 拆成显示用版本与短哈希。
func splitBuild(build string) (version, commit string) {
	build = strings.TrimSpace(build)
	if build == "" {
		return "", ""
	}
	if idx := strings.LastIndex(build, "+"); idx >= 0 {
		version, commit = build[:idx], build[idx+1:]
	} else {
		version = build
	}
	version = strings.TrimPrefix(version, "v")
	if commit == "unknown" || commit == "dev" {
		commit = ""
	}
	if len(commit) > 7 {
		commit = commit[:7]
	}
	return version, commit
}

// storyBuilds 把已完成章节按构建戳归并成连续区间（P0-2 的写入戳）。
// 相邻章节的版本+commit 相同即合并；不同或非相邻则新开一段。章节升序。
// 读进度/记录失败一律降级成“不知道哪个构建写的”，不让故事信息页整体 500。
func storyBuilds(sr *host.StoryReader) []StoryBuildRange {
	out := []StoryBuildRange{}
	progress, err := sr.Progress()
	if err != nil || progress == nil || len(progress.CompletedChapters) == 0 {
		return out
	}
	chapters := append([]int(nil), progress.CompletedChapters...)
	slices.Sort(chapters)
	for _, ch := range chapters {
		build, at := "", ""
		if record, err := sr.Store().ChapterRecords.Load(ch); err == nil && record != nil {
			build = record.Build
			if !record.AcceptedAt.IsZero() {
				at = record.AcceptedAt.UTC().Format(time.RFC3339)
			}
		}
		version, commit := splitBuild(build)
		if n := len(out); n > 0 && out[n-1].To == ch-1 && out[n-1].Version == version && out[n-1].Commit == commit {
			out[n-1].To = ch
			continue
		}
		out = append(out, StoryBuildRange{From: ch, To: ch, Version: version, Commit: commit, At: at})
	}
	return out
}
