package backdrop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	catalogue "github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

type catalogueStore struct {
	catalogue.Store
	row      domain.BackdropChannelVideo
	marked   string
	probeErr error
}

func (s *catalogueStore) ChannelVideo(context.Context, string, string) (domain.BackdropChannelVideo, bool, error) {
	return s.row, s.row.VideoID != "", nil
}
func (s *catalogueStore) UpdateChannelVideoProbe(_ context.Context, _, _ string, p catalogue.ProbeUpdate) error {
	if s.probeErr != nil {
		return s.probeErr
	}
	s.row.DurationSec = p.DurationSec
	if p.Title != nil {
		s.row.Title = *p.Title
	}
	s.row.ProbedAt = &p.ProbedAt
	return nil
}
func (s *catalogueStore) MarkChannelVideoUnavailable(_ context.Context, _, _, reason string, _ time.Time) error {
	s.marked = reason
	return nil
}

type noProcess struct{}

func (noProcess) Run(context.Context, string, []string, tools.Spec) (tools.Result, error) {
	return tools.Result{}, errors.New("unexpected raw process")
}
func TestEnsureClipIsolatesWindowsAndReusesCompletedStaging(t *testing.T) {
	store := &catalogueStore{}
	calls := 0
	service := catalogue.New(catalogue.Deps{Store: store, Now: func() time.Time { return testNow }, RunYtDlp: func(ctx context.Context, _ domain.PlatformID, args []string, _ int, _ time.Duration, _ func(string)) (string, error) {
		if catalogue.ContextRunner(ctx) == nil {
			t.Fatal("caller runner lost")
		}
		calls++
		output := ""
		window := ""
		for i, arg := range args {
			if arg == "-o" {
				output = args[i+1]
			}
			if arg == "--download-sections" {
				window = args[i+1]
			}
		}
		if output == "" {
			t.Fatal(args)
		}
		file := strings.Replace(output, "%(ext)s", "mp4", 1)
		return "", os.WriteFile(file, []byte(window), 0o600)
	}})
	dir := t.TempDir()
	segment := domain.BackdropSegment{FromSec: 10, ToSec: 40}
	first, err := service.EnsureClip(context.Background(), "https://www.twitch.tv/videos/123", segment, dir, noProcess{})
	if err != nil {
		t.Fatal(err)
	}
	again, err := service.EnsureClip(context.Background(), "https://www.twitch.tv/videos/123", segment, dir, noProcess{})
	if err != nil || again.File != first.File || calls != 1 {
		t.Fatal(again, err, calls)
	}
	second, err := service.EnsureClip(context.Background(), "https://www.twitch.tv/videos/123", domain.BackdropSegment{FromSec: 50, ToSec: 80}, dir, noProcess{})
	if err != nil || second.File == first.File || second.Path == first.Path || calls != 2 {
		t.Fatal(second, err, calls)
	}
	b, err := os.ReadFile(first.File)
	if err != nil || string(b) != "*10-40" {
		t.Fatal(string(b), err)
	}
	if !strings.HasPrefix(first.File, dir+string(filepath.Separator)) {
		t.Fatal(first.File)
	}
}
func TestEnsureClipCleansFailedDownloadsAndRetries(t *testing.T) {
	store := &catalogueStore{}
	fail := true
	service := catalogue.New(catalogue.Deps{Store: store, RunYtDlp: func(_ context.Context, _ domain.PlatformID, args []string, _ int, _ time.Duration, _ func(string)) (string, error) {
		if fail {
			return "", errors.New("interrupted")
		}
		for i, arg := range args {
			if arg == "-o" {
				return "", os.WriteFile(strings.Replace(args[i+1], "%(ext)s", "mp4", 1), []byte("clip"), 0o600)
			}
		}
		return "", errors.New("missing output")
	}})
	dir := t.TempDir()
	segment := domain.BackdropSegment{ToSec: 30}
	if _, err := service.EnsureClip(context.Background(), "https://www.twitch.tv/videos/123", segment, dir, noProcess{}); err == nil {
		t.Fatal("failure swallowed")
	}
	if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(entry.Name(), ".download-") {
			t.Fatal("orphan directory", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	fail = false
	if _, err := service.EnsureClip(context.Background(), "https://www.twitch.tv/videos/123", segment, dir, noProcess{}); err != nil {
		t.Fatal(err)
	}
}
func TestProbeChannelVideoCachesDurationAndMarksUnavailable(t *testing.T) {
	store := &catalogueStore{row: domain.BackdropChannelVideo{ChannelID: "twitch:a", VideoID: "v123", URL: "https://www.twitch.tv/videos/123"}}
	var failure error
	service := catalogue.New(catalogue.Deps{Store: store, Now: func() time.Time { return testNow }, RunYtDlp: func(context.Context, domain.PlatformID, []string, int, time.Duration, func(string)) (string, error) {
		if failure != nil {
			return "", failure
		}
		return `{"title":"Probed","duration":180}`, nil
	}})
	row, err := service.ProbeChannelVideo(context.Background(), "twitch:a", "v123")
	if err != nil || row.DurationSec == nil || *row.DurationSec != 180 || row.Title != "Probed" || row.ProbedAt == nil {
		t.Fatal(row, err)
	}
	failure = &tools.YtDlpError{Code: tools.YtDlpUnavailable, Message: "gone"}
	if _, err := service.ProbeChannelVideo(context.Background(), "twitch:a", "v123"); err == nil || store.marked != "unavailable" {
		t.Fatal(err, store.marked)
	}
}
