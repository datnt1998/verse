package host

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
)

// LogEvent 通过进程 logger 记录一条 UI / 运行时事件。Summary 是展示文本；
// Detail 是完整诊断，只要存在就作为日志正文。tui.log 是 web Nhật ký 的数据源，
// 所以落盘前把绝对路径改写成相对故事目录（工作目录）/ 主目录的形式。
func LogEvent(ev Event) {
	logEvent(slog.Default(), ev)
}

func logEvent(log *slog.Logger, ev Event) {
	if ev.Category == "USER" && ev.Username == "" {
		ev.Username = os.Getenv("USER")
		ev.Agent = "user"
	}
	if ev.Summary == "" && ev.Detail == "" {
		return
	}
	root := logRoot()
	ev = MapEventText(ev, func(s string) string { return RootRelativeText(s, root) })

	message := ev.Detail
	if message == "" {
		message = ev.Summary
	}
	attrs := []any{"module", "event"}
	// Additive JSON envelope keeps text logs readable and old fields intact.
	if raw, err := json.Marshal(ev); err == nil {
		attrs = append(attrs, "event_data", string(raw))
	}
	if ev.Category != "" {
		attrs = append(attrs, "category", ev.Category)
	}
	if ev.Agent != "" {
		attrs = append(attrs, "agent", ev.Agent)
	}
	if ev.Kind != "" {
		attrs = append(attrs, "kind", ev.Kind)
	}
	if ev.ID != "" {
		attrs = append(attrs, "event_id", ev.ID)
		if ev.hasLifecycle() {
			state := "running"
			if !ev.FinishedAt.IsZero() {
				state = "completed"
			}
			attrs = append(attrs, "state", state)
		}
	}
	if ev.Depth > 0 {
		attrs = append(attrs, "depth", ev.Depth)
	}
	if ev.Failed {
		attrs = append(attrs, "failed", true)
	}
	if ev.Duration > 0 {
		attrs = append(attrs, "duration", ev.Duration)
	}
	if ev.Detail != "" && ev.Summary != "" && ev.Detail != ev.Summary {
		attrs = append(attrs, "summary", ev.Summary)
	}

	level := slog.LevelInfo
	switch ev.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	log.Log(context.Background(), level, message, attrs...)
}
