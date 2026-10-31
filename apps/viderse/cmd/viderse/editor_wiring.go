package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newEditorService(pool *pgxpool.Pool, files *media.Service, cfgDir string, toolKit *tools.Tools, toolDeps tools.Deps) *editor.Service {
	repository := postgres.NewEditorRepository(pool)
	staging := filepath.Join(cfgDir, "staging", "editor")
	service := &editor.Service{
		Store: editor.Store{Media: files, Repository: repository, StagingDir: staging},
		Media: files, Repository: repository, StagingDir: staging, Runner: toolDeps.Runner,
		OverlayAssets: library.BuiltinOverlayAssets(),
		ExportOwner:   repository.ProjectOwner, MediaReferences: repository.MediaReferences,
		ProjectDelete: repository.DeleteProjectPolicy,
	}
	if toolKit != nil {
		if toolKit.Ffmpeg != nil {
			if ff := toolKit.Ffmpeg.Resolve(context.Background()); ff != nil {
				service.FFmpeg = ff.Ffmpeg
				service.FFprobe = ff.Ffprobe
			}
		}
		if toolKit.YtDlp != nil {
			service.YTDLP = toolKit.YtDlp.Resolve()
		}
	}
	service.Anchors = func(ctx context.Context, projectID string) (editor.Object, error) {
		path, err := repository.AnchorsPath(ctx, projectID)
		if err != nil {
			return nil, err
		}
		handle, err := files.Acquire(ctx, path)
		if err != nil {
			return nil, err
		}
		defer handle.Release()
		file, err := os.Open(handle.Path)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		var anchors editor.Object
		if err = json.NewDecoder(file).Decode(&anchors); err != nil {
			return nil, fmt.Errorf("đọc mốc giọng gốc: %w", err)
		}
		return anchors, nil
	}
	return service
}
