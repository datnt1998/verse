package host

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/host/exp"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
)

// 自动导出（契约 §5，GAP-4）。
//
// worker 启动时调用 EnableAutoExport：读 <root>/.ainovel/library.json 的
// autoExport 配置，按 onChapter/onVolume/onBook 触发导出，写入
// <story>/<dirs.exports>/ 下按 pattern 命名的文件，每次导出冒一条
// SYSTEM 事件（成功 success，失败 warn）。
//
// 实现选型：轮询进度事实（meta/progress.json + summaries/）而不是订阅
// h.Events()——事件通道是单消费者语义，多挂一个读者会分流掉 SSE 广播的
// 事件；事实是持久化的，轮询天然幂等、可重放，也不会漏掉"进程重启前已经
// 发生"的状态迁移。轮询粒度 1.5s，导出本身只读 store，可与 Engine 并发。

// autoExportPollInterval 进度事实轮询间隔（包级变量，测试可缩短）。
var autoExportPollInterval = 1500 * time.Millisecond

// EnableAutoExport 启用自动导出。root 为书库根（gateway 经
// AINOVEL_LIBRARY_ROOT 传入）；root 为空（TUI 在书库外运行）时不启用并
// 返回 nil——契约要求离开书库的 TUI 不做自动导出。
func (h *Host) EnableAutoExport(root string) error {
	if root == "" {
		return nil
	}
	cfg, err := library.LoadConfig(root)
	if err != nil {
		return fmt.Errorf("đọc library.json thất bại: %w", err)
	}
	ae := cfg.AutoExport
	if !ae.OnChapter && !ae.OnVolume && !ae.OnBook {
		return nil
	}
	var formats []exp.Format
	for _, f := range ae.Formats {
		switch exp.Format(f) {
		case exp.FormatTXT, exp.FormatEPUB, exp.FormatNarration:
			formats = append(formats, exp.Format(f))
		default:
			slog.Warn("bỏ qua định dạng xuất tự động không hỗ trợ", "module", "autoexport", "format", f)
		}
	}
	if len(formats) == 0 {
		return nil
	}

	engineDir, err := filepath.Abs(h.store.Dir())
	if err != nil {
		return fmt.Errorf("xác định thư mục truyện: %w", err)
	}
	storyDir := filepath.Dir(filepath.Dir(engineDir)) // <story>/output/novel → <story>
	w := &autoExportWatcher{
		h:           h,
		cfg:         ae,
		formats:     formats,
		storySlug:   filepath.Base(storyDir),
		exportsDir:  filepath.Join(storyDir, cfg.Dirs.Exports),
		reader:      NewStoryReaderWithStore(h.store),
		notify:      func(level, summary string) { h.emitAutoExportEvent(level, summary) },
		completed:   map[int]bool{},
		volumesDone: map[int]bool{},
		volumesArm:  map[int]VolumeRange{},
	}
	w.snapshotBaseline()
	h.asyncWG.Add(1)
	go w.run(h.runCtx)
	return nil
}

// emitAutoExportEvent 把自动导出结果冒成 SYSTEM 事件（成功 success，失败 warn）。
func (h *Host) emitAutoExportEvent(level, summary string) {
	h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Level: level, Summary: summary})
}

// autoExportWatcher 以"上次见过的进度事实"为基线做增量判定。
// notify 是 UI 事件出口（worker 里接到 Host 事件通道）。除 asyncWG 生命周期
// 外 watcher 不依赖 Host 的其它字段，便于测试直接驱动 poll。
type autoExportWatcher struct {
	h          *Host
	cfg        library.AutoExport
	formats    []exp.Format
	storySlug  string
	exportsDir string
	reader     *StoryReader
	notify     func(level, summary string)

	completed   map[int]bool        // 已见过的完成章（重启不重复导出）
	volumesDone map[int]bool        // 已导出的卷（每卷一次）
	volumesArm  map[int]VolumeRange // 已到卷末、等待卷摘要落盘的卷
	bookDone    bool                // 全书完结已导出
}

// snapshotBaseline 以当前事实为基线：进程重启后之前已发生的事件不重复导出。
func (w *autoExportWatcher) snapshotBaseline() {
	progress, err := w.reader.Progress()
	if err != nil {
		slog.Warn("đọc tiến độ khởi tạo xuất tự động thất bại", "module", "autoexport", "err", err)
		return
	}
	if progress == nil {
		return
	}
	for _, ch := range progress.CompletedChapters {
		w.completed[ch] = true
	}
	// 已到卷末且卷摘要已落盘的卷视为已导出，避免 worker 重启重复落文件。
	for _, ch := range progress.CompletedChapters {
		b, err := w.reader.ArcBoundaryOf(ch)
		if err != nil || b == nil || !b.IsVolumeEnd {
			continue
		}
		if w.volumeSummaryExists(b.Volume) {
			w.volumesDone[b.Volume] = true
		}
	}
	w.bookDone = progress.Phase == domain.PhaseComplete
}

func (w *autoExportWatcher) run(ctx context.Context) {
	if w.h != nil {
		defer w.h.asyncWG.Done()
	}
	ticker := time.NewTicker(autoExportPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

// poll 对比进度事实，触发增量导出。每 tick 最多做三类事情：
// 新提交章（onChapter）、卷完成（onVolume，等卷摘要落盘）、全书完结（onBook）。
func (w *autoExportWatcher) poll(ctx context.Context) {
	progress, err := w.reader.Progress()
	if err != nil {
		slog.Warn("đọc tiến độ xuất tự động thất bại", "module", "autoexport", "err", err)
		return
	}
	if progress == nil {
		return
	}

	var newChapters []int
	for _, ch := range progress.CompletedChapters {
		if !w.completed[ch] {
			w.completed[ch] = true
			newChapters = append(newChapters, ch)
		}
	}

	if w.cfg.OnChapter && len(newChapters) > 0 {
		if vr, err := w.chapterVolumeRange(progress, latestOf(newChapters)); err != nil {
			slog.Warn("xác định tập cho xuất tự động thất bại", "module", "autoexport", "err", err)
		} else if vr != nil {
			w.exportRange(ctx, *vr)
		}
	}

	if w.cfg.OnVolume {
		for _, ch := range newChapters {
			b, err := w.reader.ArcBoundaryOf(ch)
			if err != nil {
				slog.Warn("kiểm tra biên tập thất bại", "module", "autoexport", "chapter", ch, "err", err)
				continue
			}
			if b == nil || !b.IsVolumeEnd || w.volumesDone[b.Volume] {
				continue
			}
			vr, err := w.volumeRange(b.Volume)
			if err != nil || vr == nil {
				continue
			}
			w.volumesArm[b.Volume] = *vr
		}
		// 卷末章已提交但卷摘要可能还在生成：每轮复查，摘要落盘即导出。
		for vol, vr := range w.volumesArm {
			if w.volumesDone[vol] || !w.volumeSummaryExists(vol) {
				continue
			}
			w.volumesDone[vol] = true
			delete(w.volumesArm, vol)
			w.exportRange(ctx, vr)
		}
	}

	if w.cfg.OnBook && !w.bookDone && progress.Phase == domain.PhaseComplete {
		w.bookDone = true
		if vr, err := w.wholeBookRange(progress); err != nil {
			slog.Warn("xác định phạm vi toàn truyện thất bại", "module", "autoexport", "err", err)
		} else if vr != nil {
			w.exportRange(ctx, *vr)
		}
	}
}

func latestOf(chs []int) int {
	latest := 0
	for _, c := range chs {
		if c > latest {
			latest = c
		}
	}
	return latest
}

// volumeSummaryExists 判定卷摘要已落盘（卷真正完成的聚合信号）。
func (w *autoExportWatcher) volumeSummaryExists(vol int) bool {
	sum, err := w.reader.Store().Summaries.LoadVolumeSummary(vol)
	return err == nil && sum != nil
}

// chapterVolumeRange 返回某章所属卷的章号区间；扁平故事返回整书区间。
func (w *autoExportWatcher) chapterVolumeRange(progress *domain.Progress, chapter int) (*VolumeRange, error) {
	ranges, err := w.reader.VolumeRanges()
	if err != nil {
		return nil, err
	}
	if !progress.Layered {
		if len(ranges) > 0 {
			return &ranges[0], nil
		}
		return nil, nil
	}
	for _, vr := range ranges {
		if chapter >= vr.From && chapter <= vr.To {
			return &vr, nil
		}
	}
	// 章在大纲之外（重写溢出章等）：导出其所在进度卷。
	for _, vr := range ranges {
		if vr.Volume == progress.CurrentVolume {
			return &vr, nil
		}
	}
	return nil, nil
}

// volumeRange 返回指定卷的章号区间。
func (w *autoExportWatcher) volumeRange(volume int) (*VolumeRange, error) {
	ranges, err := w.reader.VolumeRanges()
	if err != nil {
		return nil, err
	}
	for _, vr := range ranges {
		if vr.Volume == volume {
			return &vr, nil
		}
	}
	return nil, nil
}

// wholeBookRange 返回全书章号区间（卷号取 1：{tap} 变量在整书导出中表示"全本"）。
func (w *autoExportWatcher) wholeBookRange(progress *domain.Progress) (*VolumeRange, error) {
	to := progress.LatestCompleted()
	if to == 0 {
		return nil, nil
	}
	title := w.reader.BookTitle()
	return &VolumeRange{Volume: 1, Title: title, From: 1, To: to}, nil
}

// exportRange 对一个章号区间执行全部配置的格式导出，每种格式一条 SYSTEM 事件。
func (w *autoExportWatcher) exportRange(ctx context.Context, vr VolumeRange) {
	for _, format := range w.formats {
		vars := exp.PatternVars{
			Story:  w.storySlug,
			Volume: vr.Volume,
			From:   vr.From,
			To:     vr.To,
			Date:   time.Now(),
			Format: format.FileExt(),
		}
		outPath, err := exp.ResolvePatternPath(w.exportsDir, w.cfg.Pattern, vars)
		if err != nil {
			w.emitEvent("warn", fmt.Sprintf("Xuất tự động lỗi: %v", err))
			continue
		}
		ectx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		res, err := w.reader.Export(ectx, exp.Options{
			Format:   format,
			OutPath:  outPath,
			From:     vr.From,
			To:       vr.To,
			OnExists: w.cfg.OnExists,
		})
		cancel()
		if err != nil {
			w.emitEvent("warn", fmt.Sprintf("Xuất tự động %s lỗi: %v", filepath.Base(outPath), err))
			continue
		}
		summary := fmt.Sprintf("Đã xuất %s (%d chương)", filepath.Base(res.Path), res.Chapters)
		if len(res.Skipped) > 0 {
			summary += fmt.Sprintf(", bỏ qua %d", len(res.Skipped))
		}
		w.emitEvent("success", summary)
	}
}

func (w *autoExportWatcher) emitEvent(level, summary string) {
	if w.notify != nil {
		w.notify(level, summary)
	}
}
