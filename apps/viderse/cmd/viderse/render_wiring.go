package main

import (
	"context"
	"path/filepath"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/render"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
)

func mountRender(ctx context.Context, g *web.Gateway, store *postgres.CloneStore, files *media.Service, edit *editor.Service, cfgDir, origin string, toolKit *tools.Tools, deps tools.Deps, tasks *activity.Service) *render.Service {
	requests := &render.Store{Root: filepath.Join(cfgDir, "render")}
	supervisor := &render.Supervisor{Store: requests, Runner: deps.Runner, Origin: origin, Tasks: tasks, ObserveResources: render.LogExportResources, ChromePath: func(ctx context.Context) (string, error) {
		path := toolKit.Chrome.ManagedPath()
		if path == "" {
			return "", &domain.CloneError{Code: domain.CloneErrToolMissing, Message: "Cài Chrome for Testing trong Cài đặt → Công cụ"}
		}
		return path, nil
	}, DeleteStaged: func(ctx context.Context, pid, rid string) error {
		_, err := edit.DeleteStagedExportsForRequest(ctx, pid, rid)
		return err
	}}
	runtime := &render.Service{Store: requests, Supervisor: supervisor, Editor: edit, Jobs: store, Media: files, Runner: deps.Runner, Resume: store.PrepareCloneExport, ResolveFFmpeg: func(ctx context.Context) (string, string, error) {
		resolved := toolKit.Ffmpeg.Resolve(ctx)
		if resolved == nil {
			return "", "", &domain.CloneError{Code: domain.CloneErrToolMissing, Message: "Thiếu FFmpeg"}
		}
		return resolved.Ffmpeg, resolved.Ffprobe, nil
	}}
	supervisor.Ready = func(ctx context.Context, kind, id, token string) error {
		workspace, err := edit.WorkspaceID(ctx)
		if err != nil {
			return err
		}
		return (render.AppServer{Origin: origin, WorkspaceID: workspace}).WaitReady(ctx, kind, id, token)
	}
	supervisor.Remeasure = func(ctx context.Context, jobID, pid string) (bool, []string, error) {
		_, _, err := runtime.Measure(ctx, jobID)
		if err != nil {
			return false, nil, err
		}
		gate, err := runtime.Gate(ctx, jobID)
		return gate.Allowed, gate.Reasons, err
	}
	supervisor.MeasurementPublished = func(ctx context.Context, r render.MeasurementRequest) (bool, error) {
		state, err := runtime.State(r.JobID)
		return state.Score != nil && state.Score.MeasurementID != nil && *state.Score.MeasurementID == r.ID && state.Score.Binding == r.Binding, err
	}
	edit.Gate = runtime
	web.MountEditor(g, edit, runtime)
	web.MountRender(g, runtime, ctx)
	return runtime
}
