package eval

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/entry/startup"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
)

// RunOptions 控制单次 case 运行。
type RunOptions struct {
	OutputDir   string        // 隔离输出目录（必填）
	FixturesDir string        // fixture 根目录（默认 evals/fixtures），fixture 模式必填
	Timeout     time.Duration // 单 case 墙钟上限；0 表示不限
	Progress    io.Writer     // 进度行输出（可选，nil 则不打印）
}

// defaultFixturesDir 是仓库内 fixture 的默认根目录。
const defaultFixturesDir = "evals/fixtures"

// RunCase 驱动一次 case：装配 host → 启动 → 按章数上限推进 → 到点 Abort。
// bundle 由调用方做过 variant 覆盖（如有）。返回的 error 即"运行时错误"（hard fail 依据）；
// 正常写完或正常截停都返回 nil。
//
// RunCase 独占并重置 OutputDir：StartPrepared 只重置 progress/checkpoints，不清 chapters/
// foundation 等工件，复用旧目录会让残留产物污染 diag 与 novel_context。故运行前清空，保证隔离。
func RunCase(cfg bootstrap.Config, bundle assets.Bundle, c Case, opts RunOptions) error {
	if strings.TrimSpace(opts.OutputDir) == "" {
		return fmt.Errorf("RunCase: thiếu OutputDir")
	}
	if c.FixtureMode() {
		return runFixtureCase(cfg, bundle, c, opts)
	}
	if err := resetOutputDir(opts.OutputDir); err != nil {
		return err
	}
	cfg.OutputDir = opts.OutputDir
	if c.Style != "" {
		cfg.Style = c.Style
	}

	eng, err := host.New(cfg, bundle, host.WithFileLog("headless.log", false),
		host.WithToolResultRecording("novel_context"))
	if err != nil {
		return fmt.Errorf("lắp ghép host: %w", err)
	}
	defer eng.Close()
	if logErr := eng.FileLogError(); logErr != nil {
		return fmt.Errorf("log tệp của eval không dùng được: %w", logErr)
	}

	prompt, err := startup.PrepareQuick(c.Prompt)
	if err != nil {
		return err
	}
	if err := eng.PrepareUserRules(prompt); err != nil {
		return fmt.Errorf("chuẩn bị luật người dùng: %w", err)
	}
	if err := eng.StartPrepared(prompt); err != nil {
		return fmt.Errorf("khởi động: %w", err)
	}

	return drive(eng, c.MaxChapters, opts)
}

// runFixtureCase 跑 fixture 模式：清目录 → seed 固定书稿 → 起 host → 只跑一条
// instruction → drain 到 Done。不 PrepareQuick、不跑 Arbiter，也不需要章数上限。
func runFixtureCase(cfg bootstrap.Config, bundle assets.Bundle, c Case, opts RunOptions) error {
	fixturesDir := opts.FixturesDir
	if strings.TrimSpace(fixturesDir) == "" {
		fixturesDir = defaultFixturesDir
	}
	// 必须先判"fixture 与输出目录不重叠"，再清空输出目录：反了就会把 fixture 删掉。
	if err := CheckFixtureOutput(fixturesDir, c.Fixture, opts.OutputDir); err != nil {
		return err
	}
	if err := resetOutputDir(opts.OutputDir); err != nil {
		return err
	}
	if err := SeedFixture(fixturesDir, c.Fixture, opts.OutputDir, c.Gate.MaxSeverity); err != nil {
		return fmt.Errorf("seed fixture %q: %w", c.Fixture, err)
	}
	cfg.OutputDir = opts.OutputDir
	if c.Style != "" {
		cfg.Style = c.Style
	}

	eng, err := host.New(cfg, bundle, host.WithFileLog("headless.log", false),
		host.WithToolResultRecording("novel_context"))
	if err != nil {
		return fmt.Errorf("lắp ghép host: %w", err)
	}
	defer eng.Close()
	if logErr := eng.FileLogError(); logErr != nil {
		return fmt.Errorf("log tệp của eval không dùng được: %w", logErr)
	}
	if err := eng.RunInstructionOnce(c.Instruction.Agent, c.Instruction.Task); err != nil {
		return fmt.Errorf("chạy instruction %s: %w", c.Instruction.Agent, err)
	}
	return driveOnce(eng, opts)
}

// resetOutputDir 独占并重置输出目录：StartPrepared 只重置 progress/checkpoints，
// 不清 chapters/foundation 等工件，复用旧目录会让残留产物污染 diag 与 novel_context。
func resetOutputDir(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("dọn thư mục đầu ra: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("tạo thư mục đầu ra: %w", err)
	}
	return nil
}

// driveEngine 是 drive 消费的最小引擎接口（*host.Host 天然满足）。抽出来是为了给
// drain-to-Done 纪律写确定性测试——这段并发逻辑出过 send-on-closed-channel 的坑。
type driveEngine interface {
	Events() <-chan host.Event
	Stream() <-chan string
	Done() <-chan struct{}
	Snapshot() host.UISnapshot
	Abort() bool
}

// drive 消费引擎事件流，到章数上限或超时即 Abort，等 Done 收场。
//
// 关键纪律：无论正常完成、章数截停还是超时，都必须 drain 到 Done 才返回。host 后台 waitDone
// 会向 done 发送一次，而 eng.Close()（RunCase 的 defer）会 close(done)——提前返回触发 Close
// 会与 waitDone 的发送竞争关闭通道而 panic（send on closed channel）。headless 同样靠"先 Done
// 后 Close"。同时必须排空 Events 与 Stream，避免阻塞引擎。
func drive(eng driveEngine, maxChapters int, opts RunOptions) error {
	var timeoutCh <-chan time.Time
	if opts.Timeout > 0 {
		t := time.NewTimer(opts.Timeout)
		defer t.Stop()
		timeoutCh = t.C
	}

	aborted, timedOut := false, false
	// finish 在 drain 到 Done（或通道关闭）后调用：超时则返回 error，否则正常结束。
	finish := func() error {
		if timedOut {
			return fmt.Errorf("chạy quá thời gian (%s)", opts.Timeout)
		}
		return nil
	}
	for {
		select {
		case ev, ok := <-eng.Events():
			if !ok {
				return finish()
			}
			if opts.Progress != nil && strings.TrimSpace(ev.Summary) != "" {
				fmt.Fprintf(opts.Progress, "    [%s] %s\n", ev.Category, ev.Summary)
			}
			if !aborted && capReached(eng.Snapshot(), maxChapters) {
				eng.Abort()
				aborted = true
				timeoutCh = nil // 已达截停条件，转入正常收尾，不再受超时约束（避免把成功截停误判为超时）
			}
		case <-eng.Stream():
			// 排空流式增量，不消费内容——eval 不关心正文流，只看落盘事实。
		case _, ok := <-eng.Done():
			if !ok {
				return finish()
			}
			return finish()
		case <-timeoutCh:
			eng.Abort() // 此处 aborted 必为 false（cap 截停会把 timeoutCh 置 nil）
			aborted, timedOut = true, true
			timeoutCh = nil // 禁用计时器，继续 drain 直至 Done，再由 finish 返回超时错误
		}
	}
}

// driveOnce 消费单指令运行的事件流：没有章数上限，只等 Done、超时或被取消。
// Done 之后再看快照里的 worker 回收结果——worker 自己报错时引擎同样会正常收场，
// 不读 OnceResult 就会把失败当成功。
func driveOnce(eng driveEngine, opts RunOptions) error {
	var timeoutCh <-chan time.Time
	if opts.Timeout > 0 {
		t := time.NewTimer(opts.Timeout)
		defer t.Stop()
		timeoutCh = t.C
	}

	timedOut := false
	finish := func() error {
		if timedOut {
			return fmt.Errorf("chạy quá thời gian (%s)", opts.Timeout)
		}
		if once := eng.Snapshot().OnceResult; once != nil && strings.TrimSpace(once.Err) != "" {
			return fmt.Errorf("worker %s lỗi: %s", once.Agent, once.Err)
		}
		return nil
	}
	for {
		select {
		case ev, ok := <-eng.Events():
			if !ok {
				return finish()
			}
			if opts.Progress != nil && strings.TrimSpace(ev.Summary) != "" {
				fmt.Fprintf(opts.Progress, "    [%s] %s\n", ev.Category, ev.Summary)
			}
		case <-eng.Stream():
			// 排空流式增量，不消费内容——eval 只关心落盘事实。
		case _, ok := <-eng.Done():
			if !ok {
				return finish()
			}
			return finish()
		case <-timeoutCh:
			eng.Abort()
			timedOut = true
			timeoutCh = nil // 继续 drain 直至 Done，再由 finish 返回超时错误
		}
	}
}

// capReached 判断是否达到截停条件。maxChapters>0 按已完成章数；<=0 视为"规划类"，
// 规划完成（进入 writing 或已 complete）即停。
func capReached(snap host.UISnapshot, maxChapters int) bool {
	if maxChapters <= 0 {
		return snap.Phase == string(domain.PhaseWriting) || snap.Phase == string(domain.PhaseComplete)
	}
	return snap.CompletedCount >= maxChapters
}
