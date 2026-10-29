package audio

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// SRT 生成：字幕时间轴完全由"段时长 + 段间停顿"累加而来，不依赖模型。
// 章节标题、片头音乐、章间停顿都会推进时间轴，所以字幕与导出的整卷音频对齐。

// SRTItem 是一条字幕。
type SRTItem struct {
	Index   int
	StartMs int64
	EndMs   int64
	Text    string
}

// TimedSegment 是"已经知道时长"的一段（章节内偏移由调用方累加）。
type TimedSegment struct {
	Text        string
	DurationSec float64
	PauseMs     int
}

// BuildSRT 渲染 SRT 文本。时间格式 HH:MM:SS,mmm；序号从 1 递增。
func BuildSRT(items []SRTItem) string {
	var b strings.Builder
	for i, it := range items {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n", it.Index, srtTime(it.StartMs), srtTime(it.EndMs), strings.TrimSpace(it.Text))
	}
	return b.String()
}

// srtTime 把毫秒格式化成 SRT 时间戳。
func srtTime(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	h := ms / 3_600_000
	ms -= h * 3_600_000
	m := ms / 60_000
	ms -= m * 60_000
	s := ms / 1000
	ms -= s * 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

// SegmentTimeline 把章节内各段（含停顿）铺到时间轴上，起点是 startMs。
// 停顿算在"上一段之后"，所以字幕不会覆盖静音。
func SegmentTimeline(startMs int64, segs []TimedSegment) []SRTItem {
	items := make([]SRTItem, 0, len(segs))
	cursor := startMs
	for _, seg := range segs {
		dur := seg.DurationSec
		if dur < 0 {
			dur = 0
		}
		end := cursor + int64(math.Round(dur*1000))
		items = append(items, SRTItem{
			Index:   len(items) + 1,
			StartMs: cursor,
			EndMs:   end,
			Text:    seg.Text,
		})
		cursor = end + int64(seg.PauseMs)
	}
	return items
}

// ChapterMark 是"mốc chương"（贴到视频简介里的那种）。
type ChapterMark struct {
	AtSec float64 `json:"atSec"`
	Label string  `json:"label"`
}

// FormatMarkTime 输出 "00:12" / "1:02:03"（mốc chương 用，不带毫秒）。
func FormatMarkTime(sec int) string {
	if sec < 0 {
		sec = 0
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// FormatDurationVN 输出界面用的时长："2 giờ 24 phút 31 giây"。
func FormatDurationVN(sec float64) string {
	total := int(math.Round(sec))
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	var parts []string
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%d giờ", h))
	}
	if m > 0 || h > 0 {
		parts = append(parts, fmt.Sprintf("%d phút", m))
	}
	parts = append(parts, fmt.Sprintf("%d giây", s))
	return strings.Join(parts, " ")
}

// exportedAtLayout 是导出清单里记录的时间格式。
const exportedAtLayout = time.RFC3339
