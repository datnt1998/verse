package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/datnt1998/noverse/internal/commands"
	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/entry/startup"
	"github.com/datnt1998/noverse/internal/host"
)

type slashCommand struct {
	name string
	args []string
}

// parseSlashCommand 使用共享分词器（G12）：引号内的空格属于同一个参数。
// 非斜杠输入返回 commands.ErrNotCommand。
func parseSlashCommand(text string) (slashCommand, error) {
	name, args, err := commands.Split(text)
	if err != nil {
		return slashCommand{}, err
	}
	return slashCommand{name: name, args: args}, nil
}

type commandHandler func(m Model, args []string) (tea.Model, tea.Cmd)

// commandHandlers 与 internal/commands 的注册表按名称一一对应；TestEveryCommandHasHandler 守护。
var commandHandlers = map[string]commandHandler{
	"help":      Model.runHelp,
	"model":     Model.runModel,
	"config":    Model.runConfig,
	"diag":      Model.runDiag,
	"review":    Model.runReview,
	"next":      Model.runNext,
	"start":     Model.runStart,
	"import":    Model.runImport,
	"reopen":    Model.runReopen,
	"cocreate":  Model.runCocreate,
	"simulate":  Model.runSimulate,
	"importsim": Model.runImportSim,
	"sync":      Model.runSync,
	"export":    Model.runExport,
}

func prepareFileStart(args []string) (string, error) {
	path := strings.TrimSpace(strings.Join(args, " "))
	if path == "" {
		return "", fmt.Errorf("Cách dùng: /start <đường dẫn tệp thiết lập hoặc đại cương>")
	}
	prompt, err := startup.LoadPromptFile(path)
	if err != nil {
		return "", err
	}
	return startup.PrepareQuick(prompt)
}

func (m Model) handleSlashCommand(cmd slashCommand) (tea.Model, tea.Cmd) {
	spec, ok := commands.Find(cmd.name)
	if !ok {
		m.applyEvent(host.Event{
			Time: time.Now(), Category: "ERROR", Summary: "Lệnh không xác định: /" + cmd.name, Level: "error",
		})
		m.refreshEventViewport()
		return m, nil
	}
	if spec.NeedsIdle && m.snapshot.IsRunning {
		m.applyEvent(host.Event{
			Time: time.Now(), Category: "ERROR", Summary: "Lệnh chỉ có thể thực hiện khi hệ thống đang rảnh: /" + spec.Name, Level: "error",
		})
		m.refreshEventViewport()
		return m, nil
	}
	run, ok := commandHandlers[spec.Name]
	if !ok {
		m.applyEvent(host.Event{
			Time: time.Now(), Category: "ERROR", Summary: "Lệnh không xác định: /" + spec.Name, Level: "error",
		})
		m.refreshEventViewport()
		return m, nil
	}
	line := "/" + spec.Name
	if spec.Name != "config" && len(cmd.args) > 0 {
		line += " " + strings.Join(cmd.args, " ")
	}
	m.applyEvent(host.Event{Time: time.Now(), Category: "USER", Agent: "user", Summary: line, Kind: "user", Level: "info"})
	return run(m, cmd.args)
}

func (m Model) runHelp(_ []string) (tea.Model, tea.Cmd) {
	m.help = newHelpState(m.width, m.height)
	m.textarea.Blur()
	return m, nil
}

func (m Model) runModel(args []string) (tea.Model, tea.Cmd) {
	roleHint := ""
	if len(args) > 0 {
		roleHint = args[0]
		if normalizeRoleKey(roleHint) == "" {
			m.applyEvent(host.Event{
				Time: time.Now(), Category: "ERROR", Summary: "Vai trò không hợp lệ: " + roleHint, Level: "error",
			})
			m.refreshEventViewport()
			return m, nil
		}
	}
	m.modelSwitch = newModelSwitchState(m.runtime, roleHint)
	m.textarea.Blur()
	return m, nil
}

func (m Model) runConfig(args []string) (tea.Model, tea.Cmd) {
	if len(args) != 0 {
		m.applyEvent(host.Event{Time: time.Now(), Category: "ERROR", Summary: "Cách dùng: /config", Level: "error"})
		m.refreshEventViewport()
		return m, nil
	}
	m.modelConfig = newModelConfigState(m.runtime)
	m.textarea.Blur()
	return m, nil
}

func (m Model) runDiag(_ []string) (tea.Model, tea.Cmd) {
	m.reportSeq++
	m.report = newReportState(m.width, m.height, m.reportSeq, time.Now())
	m.textarea.Blur()
	return m, loadReport(m.runtime.Dir(), m.reportSeq)
}

func (m Model) runReview(args []string) (tea.Model, tea.Cmd) {
	if len(args) != 1 || (args[0] != "on" && args[0] != "off") {
		m.applyEvent(host.Event{Time: time.Now(), Category: "ERROR", Summary: "Cách dùng: /review on|off", Level: "error"})
		m.refreshEventViewport()
		return m, nil
	}
	mode := domain.ChapterAdvanceReview
	if args[0] == "off" {
		mode = domain.ChapterAdvanceAuto
	}
	if err := m.runtime.SetAdvanceMode(mode); err != nil {
		m.applyEvent(host.Event{Time: time.Now(), Category: "ERROR", Summary: "Chuyển chế độ tiến độ thất bại: " + err.Error(), Level: "error"})
		m.refreshEventViewport()
		return m, nil
	}
	return m, fetchSnapshot(m.runtime)
}

func (m Model) runNext(args []string) (tea.Model, tea.Cmd) {
	if len(args) != 0 {
		m.applyEvent(host.Event{Time: time.Now(), Category: "ERROR", Summary: "Cách dùng: /next", Level: "error"})
		m.refreshEventViewport()
		return m, nil
	}
	if err := m.runtime.AdvanceOneChapter(); err != nil {
		m.applyEvent(host.Event{Time: time.Now(), Category: "ERROR", Summary: "Cho phép viết chương tiếp theo thất bại: " + err.Error(), Level: "error"})
		m.refreshEventViewport()
		return m, nil
	}
	return m, tea.Batch(fetchSnapshot(m.runtime), listenDone(m.runtime), m.textarea.Focus())
}

func (m Model) runStart(args []string) (tea.Model, tea.Cmd) {
	if m.mode != modeNew {
		m.applyEvent(host.Event{
			Time: time.Now(), Category: "ERROR", Summary: "Lệnh /start chỉ có thể dùng ở màn hình chào mừng", Level: "error",
		})
		m.refreshEventViewport()
		return m, nil
	}
	prompt, err := prepareFileStart(args)
	if err != nil {
		m.err = err
		return m, nil
	}
	cmd := m.enterStarting(prompt)
	return m, tea.Batch(startRuntime(m.runtime, prompt), cmd)
}

func (m Model) runImport(args []string) (tea.Model, tea.Cmd) {
	m.importSeq++
	state, listenCmd, err := startImport(m.runtime, m.importSeq, args, m.width, m.height)
	if err != nil {
		m.applyEvent(host.Event{
			Time: time.Now(), Category: "ERROR", Summary: "Khởi động nhập truyện thất bại: " + err.Error(), Level: "error",
		})
		m.refreshEventViewport()
		return m, nil
	}
	m.importer = state
	m.importHint = ""
	m.textarea.Blur()
	return m, listenCmd
}

func (m Model) runReopen(args []string) (tea.Model, tea.Cmd) {
	if err := m.runtime.Reopen(strings.Join(args, " ")); err != nil {
		m.applyEvent(host.Event{
			Time: time.Now(), Category: "ERROR", Summary: "Mở lại truyện thất bại: " + err.Error(), Level: "error",
		})
		m.refreshEventViewport()
		return m, nil
	}
	return m, tea.Batch(m.textarea.Focus(), resumeBook(m.runtime))
}

func (m Model) runCocreate(_ []string) (tea.Model, tea.Cmd) {
	if m.mode != modeRunning {
		m.applyEvent(host.Event{
			Time: time.Now(), Category: "ERROR", Summary: "Đồng sáng tác giai đoạn chỉ khả dụng khi đang viết truyện", Level: "error",
		})
		m.refreshEventViewport()
		return m, nil
	}
	if !m.runtime.PauseForCoCreate() {
		m.applyEvent(host.Event{
			Time: time.Now(), Category: "ERROR", Summary: "Không thể vào đồng sáng tác: truyện đã hoàn thành hoặc đang trong đồng sáng tác", Level: "error",
		})
		m.refreshEventViewport()
		return m, nil
	}
	m.cocreate = newStageCoCreateState()
	m.resizeTextarea()
	m.textarea.Blur()
	return m, m.sendCoCreate()
}

func (m Model) runSimulate(args []string) (tea.Model, tea.Cmd) {
	m.simSeq++
	state, listenCmd, err := startSimulate(m.runtime, m.simSeq, args, m.width, m.height)
	if err != nil {
		m.applyEvent(host.Event{
			Time: time.Now(), Category: "ERROR", Summary: "Khởi động mô phỏng văn phong thất bại: " + err.Error(), Level: "error",
		})
		m.refreshEventViewport()
		return m, nil
	}
	m.simulator = state
	m.textarea.Blur()
	return m, listenCmd
}

func (m Model) runImportSim(args []string) (tea.Model, tea.Cmd) {
	m.simSeq++
	state, listenCmd, err := startImportSimulation(m.runtime, m.simSeq, args, m.width, m.height)
	if err != nil {
		m.applyEvent(host.Event{
			Time: time.Now(), Category: "ERROR", Summary: "Nhập hồ sơ mô phỏng thất bại: " + err.Error(), Level: "error",
		})
		m.refreshEventViewport()
		return m, nil
	}
	m.simulator = state
	m.textarea.Blur()
	return m, listenCmd
}

func (m Model) runSync(args []string) (tea.Model, tea.Cmd) {
	cmd, checkOnly, err := startRevisionSync(m.runtime, args)
	if err != nil {
		m.applyEvent(host.Event{Time: time.Now(), Category: "ERROR", Summary: "Khởi động đồng bộ chương thất bại: " + err.Error(), Level: "error"})
		m.refreshEventViewport()
		return m, nil
	}
	summary := "Đang phân tích và tiếp nhận các chương tu chỉnh..."
	if checkOnly {
		summary = "Đang kiểm tra các chương có chỉnh sửa từ bên ngoài..."
	}
	m.applyEvent(host.Event{Time: time.Now(), Category: "SYSTEM", Summary: summary, Level: "info"})
	m.refreshEventViewport()
	return m, cmd
}

func (m Model) runExport(args []string) (tea.Model, tea.Cmd) {
	cmd, err := startExport(m.runtime, args)
	if err != nil {
		m.applyEvent(host.Event{
			Time: time.Now(), Category: "ERROR", Summary: "Khởi động xuất bản thất bại: " + err.Error(), Level: "error",
		})
		m.refreshEventViewport()
		return m, nil
	}
	m.applyEvent(host.Event{
		Time: time.Now(), Category: "SYSTEM", Summary: "Đang tiến hành xuất bản...", Level: "info",
	})
	m.refreshEventViewport()
	return m, cmd
}
