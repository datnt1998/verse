// Sổ các công cụ app cài cho người dùng. Một bảng map mỗi id tới cỗ máy cài
// (spec tải-và-ghi hoặc lifecycle riêng) và key cấu hình đường dẫn; cài, huỷ,
// đọc trạng thái và ghi đường dẫn đều đi qua module này để Cài đặt, CLI và
// readiness không giữ bản đồ công cụ riêng.
//
// yt-dlp nằm ngoài: nó sở hữu pipeline tự cập nhật theo release.
package tools

import (
	"context"
	"fmt"
	"sync"
)

// ManagedToolID là id của một công cụ do app quản.
type ManagedToolID string

const (
	ManagedFfmpeg        ManagedToolID = "ffmpeg"
	ManagedUv            ManagedToolID = "uv"
	ManagedTranscription ManagedToolID = "transcription"
	ManagedCodex         ManagedToolID = "codex"
	ManagedChrome        ManagedToolID = "chrome"
)

// managedToolIDs liệt kê đúng các công cụ app cài được.
var managedToolIDs = []ManagedToolID{
	ManagedFfmpeg, ManagedUv, ManagedTranscription, ManagedCodex, ManagedChrome,
}

// IsManagedToolID đúng khi id nằm trong danh sách.
func IsManagedToolID(id string) bool {
	for _, known := range managedToolIDs {
		if string(known) == id {
			return true
		}
	}
	return false
}

// managedEntry là một dòng của sổ: đường cài và key PathStore của binary
// ngoài do người dùng chọn ("" = không có).
type managedEntry struct {
	spec       *InstallSpec // cài theo pipeline ghim; nil → lifecycle map
	settingKey string
}

// Managed sổ công cụ trên một bộ Deps + Installer.
type Managed struct {
	d         Deps
	installer *Installer
	entries   map[ManagedToolID]managedEntry

	mu         sync.Mutex
	lifecycles map[ManagedToolID]func(ctx context.Context, lc *LifecycleContext) error
}

// NewManaged dựng sổ công cụ; installer nil → một Installer trên d.BaseDir.
func NewManaged(d Deps, installer *Installer, t *Tools) *Managed {
	if installer == nil {
		installer = &Installer{Client: d.Client, BaseDir: d.BaseDir}
	}
	m := &Managed{d: d, installer: installer, lifecycles: map[ManagedToolID]func(ctx context.Context, lc *LifecycleContext) error{}}
	m.entries = map[ManagedToolID]managedEntry{
		ManagedFfmpeg:        {spec: new(t.Ffmpeg.InstallSpec()), settingKey: FfmpegSettingKey},
		ManagedUv:            {spec: new(t.Uv.InstallSpec()), settingKey: UvSettingKey},
		ManagedCodex:         {spec: new(t.Codex.InstallSpec()), settingKey: CodexSettingKey},
		ManagedChrome:        {spec: new(t.Chrome.InstallSpec())},
		ManagedTranscription: {},
	}
	return m
}

// RegisterLifecycle cắm thân cài cho một công cụ kiểu lifecycle
// (transcription đăng ký từ gói của nó khi được thêm). runCtx là context của
// lượt chia sẻ: hủy khi caller cuối rút đi, không phải ctx của một caller.
func (m *Managed) RegisterLifecycle(id ManagedToolID, run func(ctx context.Context, lc *LifecycleContext) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lifecycles[id] = run
}

// Install cài hoặc sửa một công cụ; lời gọi đồng thời gộp vào lượt đang chạy.
func (m *Managed) Install(ctx context.Context, id ManagedToolID) (InstallResult, error) {
	if !IsManagedToolID(string(id)) {
		return InstallResult{}, fmt.Errorf("công cụ không được quản lý: %q", id)
	}
	entry := m.entries[id]
	if entry.spec != nil {
		return m.installer.Install(ctx, *entry.spec)
	}
	m.mu.Lock()
	run := m.lifecycles[id]
	m.mu.Unlock()
	if run == nil {
		return InstallResult{}, fmt.Errorf("công cụ %q chưa đăng ký lifecycle", id)
	}
	if err := m.installer.RunLifecycle(ctx, string(id), "managed", func(runCtx context.Context, lc *LifecycleContext) error {
		if err := lc.Check(); err != nil {
			return err
		}
		lc.State(PhaseInstalling, "", "")
		return run(runCtx, lc)
	}); err != nil {
		return InstallResult{}, err
	}
	return InstallResult{ToolID: string(id), State: "installed"}, nil
}

// Cancel yêu cầu lượt cài đang chạy (mọi process) dừng; quan sát trong ~1s.
func (m *Managed) Cancel(id ManagedToolID) error {
	if !IsManagedToolID(string(id)) {
		return fmt.Errorf("công cụ không được quản lý: %q", id)
	}
	m.installer.RequestCancel(string(id))
	return nil
}

// State trả tiến độ trực tiếp cho thẻ poll trong lúc cài, hoặc trạng thái
// cuối đã ghi; nil khi chưa từng cài.
func (m *Managed) State(id ManagedToolID) (*InstallState, error) {
	if !IsManagedToolID(string(id)) {
		return nil, fmt.Errorf("công cụ không được quản lý: %q", id)
	}
	return m.installer.ReadInstallState(string(id)), nil
}

// SetPath lưu binary do người dùng chọn cho công cụ; "" xoá ghi đè. Công cụ
// không có key cấu hình bị từ chối.
func (m *Managed) SetPath(id ManagedToolID, exe string) error {
	if !IsManagedToolID(string(id)) {
		return fmt.Errorf("công cụ không được quản lý: %q", id)
	}
	key := m.entries[id].settingKey
	if key == "" {
		return fmt.Errorf("công cụ %q không có đường dẫn cấu hình được", id)
	}
	return m.d.SetConfiguredPath(key, exe)
}

// ManagedToolIDs trả bản sao danh sách: dữ liệu ghim/bảng hằng là bất biến,
// caller không chỉnh được bản gốc trong process.
func ManagedToolIDs() []ManagedToolID { return append([]ManagedToolID(nil), managedToolIDs...) }
