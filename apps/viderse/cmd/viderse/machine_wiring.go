package main

// Ráp mảng máy (V7/V9/V17/V22): việc nền + thông báo, dịch vụ công cụ trên
// internal/tools, nhật ký/gói báo lỗi, và điều phối bước Chuẩn bị máy. Chỉ cmd
// biết cả bốn gói; mỗi gói chỉ thấy interface nhỏ do phía dùng khai.
// ctx là vòng đời serve: mọi goroutine (probe nền, lượt cài, đóng log) dừng
// khi serve tắt.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/logs"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/setup"
	"github.com/datnt1998/verse/apps/viderse/internal/status"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/apps/viderse/internal/toolsvc"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/datnt1998/verse/platform/events"
)

// mountMachine dựng mảng "máy" trên pool/hub/status, dùng chung bộ binary
// (kit/managed) mà serve.go đã dựng để khối AIConn cũng thấy đúng một bản.
func mountMachine(g *web.Gateway, pool *pgxpool.Pool, settings *postgres.SettingsStore, hub *events.Hub, statusSvc *status.Service, ctx context.Context, dir string, logStore *logs.Store, deps tools.Deps, kit *tools.Tools, managed *tools.Managed, chatGPT toolsvc.ChatGPTSession, activities *activity.Service, notifyActivity func(ref string), platformRequests func(context.Context) ([]backdrop.PlatformRequestRecord, error)) (*machineRuntime, error) {

	notifyTools := web.Notifier(hub, "tools")

	// Phục hồi sau sập: việc còn "running" của process trước thành failed kèm
	// Thử lại, để bảng việc nền không treo trạng thái chạy mãi.
	if n, err := activities.Reconcile(ctx); err != nil {
		return nil, fmt.Errorf("phục hồi việc nền: %w", err)
	} else if n > 0 {
		slog.Warn("việc nền bị gián đoạn khi khởi động lại", "module", "activity", "count", n)
	}

	transcription := tools.NewTranscription(deps, kit.Uv, nil)
	managed.RegisterLifecycle(tools.ManagedTranscription, transcription.Install)

	touchTools := func(ref string) {
		notifyTools(ref)
		if ref == "" {
			statusSvc.Bump()
		}
	}
	toolSvc := toolsvc.New(toolsvc.Deps{
		Tools:         kit,
		Managed:       managed,
		Transcription: transcription,
		Tasks:         activities,
		Log:           logStore,
		Notify:        touchTools,
		ChatGPT:       chatGPT,
		DataDir:       dir,
	})

	setupSvc := setup.New(ctx, toolSvc, dir, func(ref string) {
		notifyTools(ref)
		notifyActivity(ref)
		statusSvc.Bump()
	})

	web.MountActivity(g, activities)
	web.MountTools(g, toolSvc)
	web.MountSetup(g, setupSvc)

	version := logs.NewVersionChecker(appVersion(), logs.NewGitHubVersion("datnt1998", "verse"))
	web.MountLogs(g, machineLogs{
		store: logStore, version: version, tools: toolSvc, activity: activities, settings: settings,
		requests: platformRequests,
	}, version)

	statusSvc.RegisterSource(machineStatus{activity: activities, tools: toolSvc})

	// Probe nền ngay sau khởi động: GET /api/tools đầu tiên phục vụ cache,
	// không spawn binary trong request.
	toolSvc.Warm(ctx)
	return &machineRuntime{tools: toolSvc, setup: setupSvc}, nil
}

// machineRuntime giữ các worker của mảng máy để serve chờ chúng dừng (kể cả
// tiến trình con của installer) trước khi đóng hub/log/pool.
type machineRuntime struct {
	tools *toolsvc.Service
	setup *setup.Service
}

func (m *machineRuntime) Shutdown(ctx context.Context) error {
	var first error
	if err := m.tools.Shutdown(ctx); err != nil && first == nil {
		first = err
	}
	if err := m.setup.Shutdown(ctx); err != nil && first == nil {
		first = err
	}
	return first
}

// machineLogs là adapter HTTP của log store: đọc phân trang + dựng gói báo
// lỗi có settings/phiên bản/việc nền đã lọc.
type machineLogs struct {
	store    *logs.Store
	version  *logs.VersionChecker
	tools    *toolsvc.Service
	activity *activity.Service
	settings *postgres.SettingsStore
	// requests đọc yêu cầu hỗ trợ nền tảng đã ghi (mảng Video nền); nil = bỏ
	// qua tệp phụ.
	requests func(ctx context.Context) ([]backdrop.PlatformRequestRecord, error)
}

func (m machineLogs) Read(tool string, minLevel string, limit int, before time.Time) ([]logs.Entry, *time.Time, error) {
	return m.store.Read(tool, minLevel, limit, before)
}

func (m machineLogs) Bundle(ctx context.Context) (*logs.Bundle, error) {
	extras := []logs.ExtraFile{
		func(ctx context.Context) (string, []byte, error) {
			// Chỉ cờ không bí mật; mọi khoá/cookie nằm ngoài tệp này.
			paused, err := m.settings.QueuePaused(ctx)
			if err != nil {
				return "", nil, err
			}
			raw, err := json.Marshal(map[string]any{"queuePaused": paused})
			return "settings.json", raw, err
		},
		func(ctx context.Context) (string, []byte, error) {
			snap, err := m.tools.Snapshot(ctx, false)
			if err != nil {
				return "", nil, err
			}
			type row struct {
				ID      string `json:"id"`
				State   string `json:"state"`
				Version string `json:"version,omitempty"`
			}
			rows := make([]row, 0, len(snap.Tools))
			for _, r := range snap.Tools {
				rows = append(rows, row{ID: r.ID, State: r.State, Version: r.Version})
			}
			raw, err := json.Marshal(map[string]any{"checkedAt": snap.CheckedAt, "tools": rows})
			return "tools.json", raw, err
		},
		func(ctx context.Context) (string, []byte, error) {
			snap, err := m.activity.Snapshot(ctx)
			if err != nil {
				return "", nil, err
			}
			raw, err := json.Marshal(map[string]any{
				"running": snap.Tasks, "videoJobs": snap.VideoJobs,
				"queued": snap.Queued, "unread": snap.Unread,
			})
			return "activity.json", raw, err
		},
		platformRequestsExtra(m.requests),
	}
	return m.store.BuildBundle(ctx, logs.BundleOptions{
		Version: m.version.Get(ctx).Version,
		Extras:  extras,
	})
}

// platformRequestsExtra dựng tệp phụ "platform-requests.json" từ yêu cầu hỗ
// trợ nền tảng đã ghi; không có bộ đọc thì gói bỏ qua tệp này.
func platformRequestsExtra(read func(ctx context.Context) ([]backdrop.PlatformRequestRecord, error)) logs.ExtraFile {
	return func(ctx context.Context) (string, []byte, error) {
		if read == nil {
			return "", nil, nil
		}
		rows, err := read(ctx)
		if err != nil {
			return "", nil, err
		}
		type row struct {
			Host    string `json:"host"`
			Label   string `json:"label"`
			Count   int    `json:"count"`
			FirstAt string `json:"firstAt"`
			LastAt  string `json:"lastAt"`
		}
		out := make([]row, 0, len(rows))
		for _, r := range rows {
			out = append(out, row{
				Host: r.Host, Label: r.Label, Count: r.Count,
				FirstAt: r.FirstAt.UTC().Format(time.RFC3339),
				LastAt:  r.LastAt.UTC().Format(time.RFC3339),
			})
		}
		raw, err := json.Marshal(map[string]any{"requests": out})
		return "platform-requests.json", raw, err
	}
}

// machineStatus cộng số video chạy/chờ và mục cần xử lý của máy vào snapshot
// đầu trang — adapter ở chỗ ráp để gói nghiệp vụ không import internal/status.
type machineStatus struct {
	activity *activity.Service
	tools    *toolsvc.Service
}

func (m machineStatus) Report(ctx context.Context) (status.Report, error) {
	snap, err := m.activity.Snapshot(ctx)
	if err != nil {
		return status.Report{}, err
	}
	rep := status.Report{Running: snap.VideoJobs, Queued: snap.Queued}
	for _, a := range m.tools.Attention(ctx) {
		rep.Attention = append(rep.Attention, status.Attention{
			Kind: domain.AttentionKind(a.Kind), Text: a.Text, Href: a.Href,
		})
	}
	return rep, nil
}

// appVersion đọc phiên bản module của binary; bản chạy từ mã nguồn là "dev".
func appVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
