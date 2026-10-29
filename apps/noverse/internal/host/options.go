package host

import "log/slog"

type newOptions struct {
	logFile                string
	logAlsoStderr          bool
	logAttrs               []slog.Attr
	allowUnconfiguredReads bool
	recordToolResults      []string
	storage                *Storage
}

// NewOption 配置 Host 构造过程，运行时资源仍由 Host 持有。
type NewOption func(*newOptions)

// WithFileLog 让 Host 持有一个运行时日志会话。日志只在取得小说目录租约后打开，
// 并在 Host 的所有关闭日志完成后关闭。打开失败时继续使用当前进程 logger，
// 调用方必须通过 FileLogError 显式处理该错误。
func WithFileLog(filename string, alsoStderr bool, attrs ...slog.Attr) NewOption {
	return func(opts *newOptions) {
		opts.logFile = filename
		opts.logAlsoStderr = alsoStderr
		opts.logAttrs = append([]slog.Attr(nil), attrs...)
	}
}

// WithUnconfiguredReads lets web workers open the durable story without an LLM.
// CLI/TUI callers retain strict configuration validation.
func WithUnconfiguredReads() NewOption {
	return func(opts *newOptions) { opts.allowUnconfiguredReads = true }
}

// WithToolResultRecording 指定哪些工具的结果在 session 日志里保留原文而不压缩。
// 仅供评测采集使用：必须在 worker 启动前生效，生产路径不调用——完整上下文
// 落盘只为 measurement，不是持久化配置项。
func WithToolResultRecording(names ...string) NewOption {
	return func(opts *newOptions) { opts.recordToolResults = append(opts.recordToolResults, names...) }
}
