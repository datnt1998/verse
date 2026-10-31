package render

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

const MaxCrashRestarts = 2
const HeartbeatTimeout = 45 * time.Second
const ClaimTimeout = 2 * time.Minute
const StallTimeout = 5 * time.Minute
const ExportTimeout = 4 * time.Hour
const MeasurementTimeout = 10 * time.Minute

type Supervisor struct {
	Store                                      *Store
	Runner                                     tools.Runner
	ChromePath                                 func(context.Context) (string, error)
	Origin                                     string
	Ready                                      func(context.Context, string, string, string) error
	Clock                                      tools.Clock
	ProcessOps                                 tools.ProcessOps
	PollInterval, ClaimDeadline, StallDeadline time.Duration
	DeleteStaged                               func(context.Context, string, string) error
	Remeasure                                  func(context.Context, string, string) (bool, []string, error)
	MeasurementPublished                       func(context.Context, MeasurementRequest) (bool, error)
	lane                                       chan struct{}
	init                                       sync.Once
}
type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }
func (wallClock) Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
func (s *Supervisor) defaults() {
	s.init.Do(func() {
		if s.Runner == nil {
			s.Runner = &tools.ExecRunner{}
		}
		if s.Clock == nil {
			s.Clock = wallClock{}
		}
		if s.ProcessOps == nil {
			s.ProcessOps = tools.SystemOps{}
		}
		if s.PollInterval <= 0 {
			s.PollInterval = time.Second
		}
		if s.ClaimDeadline <= 0 {
			s.ClaimDeadline = ClaimTimeout
		}
		if s.StallDeadline <= 0 {
			s.StallDeadline = StallTimeout
		}
		s.lane = make(chan struct{}, 1)
	})
}
func renderError(code, message string) error { return &domain.CloneError{Code: code, Message: message} }
func (s *Supervisor) terminate(ctx context.Context, chrome *ChromeProcess) error {
	if chrome == nil {
		return nil
	}
	_, err := tools.TerminateGroup(ctx, tools.GroupTarget{PGID: chrome.PGID, Identity: []string{chrome.ProfileDir}}, tools.TerminateOptions{Ops: s.ProcessOps, Clock: s.Clock})
	return err
}

// Sweep is called while the process holds the pipeline's exclusive startup lease.
// Startup deliberately ignores dead invocation leases: kill -9 must not require
// waiting a minute before the exact request can be rerun from frame zero.
func (s *Supervisor) Sweep(ctx context.Context, startup bool) error {
	s.defaults()
	requests, err := s.Store.ListExports()
	if err != nil {
		return err
	}
	for _, r := range requests {
		if !startup && !r.Terminal() && !r.LeaseExpired(s.Clock.Now()) {
			continue
		}
		if err = s.terminate(ctx, r.Chrome); err != nil {
			return err
		}
		if !r.Terminal() {
			if s.DeleteStaged != nil {
				if err = s.DeleteStaged(ctx, r.ProjectID, r.ID); err != nil {
					return err
				}
			}
			if startup {
				_, err = s.Store.RecoverExport(r.ID)
			} else {
				_, err = s.Store.StaleExport(r.ID, "Yêu cầu xuất bị bỏ rơi — đã quét dọn")
			}
			if err != nil {
				return err
			}
		}
		if err = os.RemoveAll(s.Store.ProfileDir(r.ID)); err != nil {
			return err
		}
	}
	measures, err := s.Store.AllMeasurements()
	if err != nil {
		return err
	}
	for _, r := range measures {
		if !startup && !r.Terminal() && !r.LeaseExpired(s.Clock.Now()) {
			continue
		}
		if err = s.terminate(ctx, r.Chrome); err != nil {
			return err
		}
		if !r.Terminal() {
			if startup {
				_, err = s.Store.RecoverMeasurement(r.ID)
			} else {
				_, err = s.Store.CancelMeasurement(r.ID, r.Nonce)
			}
			if err != nil {
				return err
			}
		}
		if err = os.RemoveAll(s.Store.ProfileDir(r.ID)); err != nil {
			return err
		}
	}
	// Profiles without a record never authorize signalling an unrelated group.
	known := map[string]bool{}
	for _, r := range requests {
		known[r.ID] = true
	}
	for _, r := range measures {
		known[r.ID] = true
	}
	entries, err := os.ReadDir(filepath.Join(s.Store.Root, "chrome-profiles"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() && !known[e.Name()] {
			if err = os.RemoveAll(filepath.Join(s.Store.Root, "chrome-profiles", e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

type ExportLaunchResult struct {
	ExportID   string `json:"exportId"`
	ProjectID  string `json:"projectId"`
	DurationMS int64  `json:"durationMs"`
	RequestID  string `json:"requestId"`
}

func (s *Supervisor) LaunchExport(ctx context.Context, projectID, jobID string, onProgress func(ExportProgress, bool)) (ExportLaunchResult, error) {
	s.defaults()
	select {
	case s.lane <- struct{}{}:
		defer func() { <-s.lane }()
	case <-ctx.Done():
		return ExportLaunchResult{}, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(ctx, ExportTimeout)
	defer cancel()
	started := s.Clock.Now()
	driftRecovered := false
	for restarts := 0; ; {
		var r ExportRequest
		var err error
		if restarts == 0 && !driftRecovered {
			pending, e := s.Store.ListExports()
			if e != nil {
				return ExportLaunchResult{}, e
			}
			for _, candidate := range pending {
				if candidate.ProjectID == projectID && candidate.JobID == jobID && candidate.Status == "queued" {
					r = candidate
					break
				}
			}
		}
		if r.ID == "" {
			r, err = s.Store.CreateExport(projectID, jobID)
			if err != nil {
				return ExportLaunchResult{}, err
			}
		}
		done, crash, err := s.runExportAttempt(ctx, r, restarts > 0 || driftRecovered, onProgress)
		if err == nil {
			return ExportLaunchResult{ExportID: done.ExportID, ProjectID: projectID, RequestID: done.ID, DurationMS: s.Clock.Now().Sub(started).Milliseconds()}, nil
		}
		if ctx.Err() != nil {
			return ExportLaunchResult{}, ctx.Err()
		}
		latest, readErr := s.Store.GetExport(r.ID)
		if readErr != nil {
			return ExportLaunchResult{}, readErr
		}
		if latest.Error != nil && latest.Error.Code == "gate" && jobID != "" && !driftRecovered && s.Remeasure != nil {
			driftRecovered = true
			allowed, reasons, measureErr := s.Remeasure(ctx, jobID, projectID)
			if measureErr != nil {
				return ExportLaunchResult{}, measureErr
			}
			if !allowed {
				return ExportLaunchResult{}, renderError(domain.CloneErrQualityGate, fmt.Sprintf("Video chưa đạt Độ hoàn thiện để xuất: %v", reasons))
			}
			continue
		}
		if !crash {
			return ExportLaunchResult{}, err
		}
		if restarts >= MaxCrashRestarts {
			return ExportLaunchResult{}, renderError(domain.CloneErrInterrupted, fmt.Sprintf("Xuất ngầm thất bại sau %d lần: %v", restarts+1, err))
		}
		restarts++
	}
}
func (s *Supervisor) runExportAttempt(ctx context.Context, r ExportRequest, rerun bool, onProgress func(ExportProgress, bool)) (ExportRequest, bool, error) {
	chrome, err := s.ChromePath(ctx)
	if err != nil {
		return r, false, err
	}
	if chrome == "" {
		return r, false, renderError(domain.CloneErrToolMissing, "Không tìm thấy trình duyệt để xuất ngầm")
	}
	if s.Ready != nil {
		if err = s.Ready(ctx, "render", r.ID, r.Token); err != nil {
			return r, false, err
		}
	}
	profile := s.Store.ProfileDir(r.ID)
	if err = os.MkdirAll(profile, 0700); err != nil {
		return r, false, err
	}
	requestID, nonce, token := r.ID, r.Nonce, r.Token
	childCtx, stop := context.WithCancel(ctx)
	exited := make(chan error, 1)
	go func() {
		_, e := s.Runner.Run(childCtx, chrome, []string{"--headless=new", "--use-mock-keychain", "--password-store=basic", "--no-first-run", "--no-default-browser-check", "--autoplay-policy=no-user-gesture-required", "--mute-audio", "--window-size=1920,1080", "--user-data-dir=" + profile, s.Origin + "/#/render/" + url.PathEscape(requestID) + "?token=" + url.QueryEscape(token)}, tools.Spec{Match: profile, Label: "export-chrome", OnStart: func(pid int) {
			_, e := s.Store.AttachChrome(requestID, nonce, ChromeProcess{PID: pid, PGID: pid, ProfileDir: profile})
			if e != nil {
				stop()
			}
		}})
		exited <- e
	}()
	defer func() {
		stop()
		<-exited
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		latest, e := s.Store.GetExport(r.ID)
		if e == nil {
			_ = s.terminate(cleanupCtx, latest.Chrome)
			if !latest.Terminal() {
				_, _ = s.Store.CancelExport(r.ID, r.Nonce)
			}
			if latest.Status != "done" && s.DeleteStaged != nil {
				_ = s.DeleteStaged(cleanupCtx, r.ProjectID, r.ID)
			}
		}
		_ = os.RemoveAll(profile)
	}()
	began := s.Clock.Now()
	moved := began
	lastMove := ""
	childExited := false
	for {
		if err = s.Clock.Sleep(ctx, s.PollInterval); err != nil {
			return r, false, err
		}
		latest, e := s.Store.GetExport(r.ID)
		if e != nil {
			return r, false, e
		}
		r = latest
		if r.Progress != nil && onProgress != nil {
			onProgress(*r.Progress, rerun)
		}
		progressBytes, _ := json.Marshal(r.Progress)
		move := r.Status + "|" + string(progressBytes)
		if move != lastMove {
			moved = s.Clock.Now()
			lastMove = move
		}
		if r.Status == "done" && r.ExportID != "" {
			return r, false, nil
		}
		if r.Terminal() {
			message := "Xuất ngầm báo lỗi"
			if r.Error != nil {
				message = r.Error.Message
			}
			return r, false, renderError(domain.CloneErrInternal, message)
		}
		select {
		case e := <-exited:
			exited <- e
			childExited = true
		default:
		}
		if childExited {
			return r, true, fmt.Errorf("Chrome headless dừng giữa chừng")
		}
		if r.Status == "queued" && s.Clock.Now().Sub(began) > s.ClaimDeadline {
			return r, true, fmt.Errorf("trang xuất không nhận yêu cầu")
		}
		if r.Status != "queued" && s.Clock.Now().Sub(moved) > s.StallDeadline {
			return r, true, fmt.Errorf("xuất ngầm đứng yên")
		}
		if r.HeartbeatStale(s.Clock.Now()) {
			return r, true, fmt.Errorf("trang xuất ngừng heartbeat")
		}
	}
}

// LaunchMeasurement gives the PCM page the same process ownership and bounded
// crash recovery as export. Measurement completion itself runs ebur128 server-side.
func (s *Supervisor) LaunchMeasurement(ctx context.Context, request MeasurementRequest) (MeasurementRequest, error) {
	s.defaults()
	ctx, cancel := context.WithTimeout(ctx, MeasurementTimeout)
	defer cancel()
	for restart := 0; ; restart++ {
		r, crash, err := s.runMeasurementAttempt(ctx, request)
		if err == nil {
			return r, nil
		}
		if ctx.Err() != nil {
			_, _ = s.Store.CancelMeasurement(request.ID, request.Nonce)
			return r, ctx.Err()
		}
		if !crash || restart >= MaxCrashRestarts {
			if !r.Terminal() {
				_, _ = s.Store.CancelMeasurement(request.ID, request.Nonce)
			}
			return r, err
		}
		request, err = s.Store.RecoverMeasurement(request.ID)
		if err != nil {
			return r, err
		}
	}
}
func (s *Supervisor) runMeasurementAttempt(ctx context.Context, r MeasurementRequest) (MeasurementRequest, bool, error) {
	chrome, err := s.ChromePath(ctx)
	if err != nil {
		return r, false, err
	}
	if chrome == "" {
		return r, false, renderError(domain.CloneErrToolMissing, "Không tìm thấy trình duyệt để đo âm lượng")
	}
	profile := s.Store.ProfileDir(r.ID)
	if err = os.MkdirAll(profile, 0700); err != nil {
		return r, false, err
	}
	if s.Ready != nil {
		if err = s.Ready(ctx, "measure", r.ID, r.Token); err != nil {
			return r, false, err
		}
	}
	requestID, nonce, token := r.ID, r.Nonce, r.Token
	childCtx, stop := context.WithCancel(ctx)
	exited := make(chan error, 1)
	go func() {
		_, e := s.Runner.Run(childCtx, chrome, []string{"--headless=new", "--use-mock-keychain", "--password-store=basic", "--disable-gpu", "--disable-blink-features=WebGPU", "--no-first-run", "--no-default-browser-check", "--autoplay-policy=no-user-gesture-required", "--mute-audio", "--user-data-dir=" + profile, s.Origin + "/#/measure/" + url.PathEscape(requestID) + "?token=" + url.QueryEscape(token)}, tools.Spec{Match: profile, Label: "measure-chrome", OnStart: func(pid int) {
			_, e := s.Store.AttachMeasurementChrome(requestID, nonce, ChromeProcess{PID: pid, PGID: pid, ProfileDir: profile})
			if e != nil {
				stop()
			}
		}})
		exited <- e
	}()
	defer func() {
		stop()
		<-exited
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		latest, e := s.Store.GetMeasurement(requestID)
		if e == nil {
			_ = s.terminate(cleanupCtx, latest.Chrome)
		}
		_ = os.RemoveAll(profile)
	}()
	began := s.Clock.Now()
	moved := began
	var bytes int64 = -1
	for {
		if err = s.Clock.Sleep(ctx, s.PollInterval); err != nil {
			return r, false, err
		}
		r, err = s.Store.GetMeasurement(r.ID)
		if err != nil {
			return r, false, err
		}
		if r.Status == "measured" && r.Result != nil {
			if s.MeasurementPublished == nil {
				return r, false, nil
			}
			published, e := s.MeasurementPublished(ctx, r)
			if e != nil {
				return r, false, e
			}
			if published {
				return r, false, nil
			}
			continue
		}
		if r.Terminal() {
			return r, false, fmt.Errorf("runtime đo báo lỗi: %v", r.Error)
		}
		if r.BytesReceived != bytes {
			bytes = r.BytesReceived
			moved = s.Clock.Now()
		}
		select {
		case e := <-exited:
			exited <- e
			return r, true, fmt.Errorf("Chrome headless dừng trước khi đo xong")
		default:
		}
		if r.Status == "queued" && s.Clock.Now().Sub(began) > s.ClaimDeadline {
			return r, true, fmt.Errorf("trang đo không nhận yêu cầu")
		}
		if r.Status != "queued" && (r.HeartbeatStale(s.Clock.Now()) || s.Clock.Now().Sub(moved) > s.StallDeadline) {
			return r, true, fmt.Errorf("trang đo ngừng tiến độ")
		}
	}
}
