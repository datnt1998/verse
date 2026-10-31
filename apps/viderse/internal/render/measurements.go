package render

// Ported from yt-factory 2c201e6 clone/measurements.ts. The extra project,
// token and Chrome facts scope a headless measurement to its persisted request.
import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
)

const (
	MeasurementLease                = time.Minute
	MeasurementCompleteLease        = 10 * time.Minute
	MeasurementMaxPCMBytes    int64 = 8 * 1024 * 1024 * 1024
	MeasurementOrphanPCMGrace       = time.Hour
	MeasurementSampleRate           = 48000
	MeasurementChannels             = 2
)

type MeasurementResult struct {
	LUFS        float64 `json:"lufs"`
	Samples     int64   `json:"samples"`
	DurationSec float64 `json:"durationSec"`
	MeasuredAt  string  `json:"measuredAt"`
}

type MeasurementRequest struct {
	Version        int                `json:"version"`
	ID             string             `json:"id"`
	Nonce          string             `json:"nonce"`
	Token          string             `json:"token"`
	TokenIssuedAt  string             `json:"tokenIssuedAt"`
	Status         string             `json:"status"`
	CreatedAt      string             `json:"createdAt"`
	JobID          string             `json:"jobId"`
	ProjectID      string             `json:"projectId"`
	Binding        quality.Binding    `json:"binding"`
	OwnerID        string             `json:"ownerId"`
	LeaseExpiresAt string             `json:"leaseExpiresAt"`
	Fence          string             `json:"fence"`
	ClaimedAt      string             `json:"claimedAt"`
	HeartbeatAt    string             `json:"heartbeatAt"`
	Chrome         *ChromeProcess     `json:"chrome"`
	ChunksReceived int64              `json:"chunksReceived"`
	BytesReceived  int64              `json:"bytesReceived"`
	SampleRate     *int               `json:"sampleRate"`
	Channels       *int               `json:"channels"`
	Result         *MeasurementResult `json:"result"`
	Error          *RequestError      `json:"error"`
	FinishedAt     string             `json:"finishedAt"`
}

// ChunkResult retains the source API acknowledgment shape, including duplicate
// last-chunk responses; it does not expose or advance a second sequence number.
type ChunkResult struct {
	ChunksReceived int64 `json:"chunksReceived"`
	BytesReceived  int64 `json:"bytesReceived"`
}

type OrphanPCMReport struct {
	Files []string `json:"files"`
	Bytes int64    `json:"bytes"`
}

func (r MeasurementRequest) Terminal() bool {
	switch r.Status {
	case "measured", "failed", "cancelled", "stale":
		return true
	}
	return false
}
func (r MeasurementRequest) LeaseExpired(now time.Time) bool {
	return expiredTimestamp(r.LeaseExpiresAt, now)
}
func (r MeasurementRequest) HeartbeatStale(now time.Time) bool {
	return r.Status == "running" && staleHeartbeat(r.HeartbeatAt, now)
}

// SelectRelevantMeasurement expects the newest-first ordering from ListMeasurements.
func SelectRelevantMeasurement(requests []MeasurementRequest, now time.Time) *MeasurementRequest {
	for i := range requests {
		if !requests[i].Terminal() && !requests[i].LeaseExpired(now) {
			return &requests[i]
		}
	}
	if len(requests) == 0 {
		return nil
	}
	return &requests[0]
}

func validMeasurementBinding(b quality.Binding) bool {
	return b.ProjectRevision != "" && b.SettingsHash != "" && b.ArtifactHash != ""
}
func (s *Store) measurementsDir(jobID string) string {
	return filepath.Join(s.Root, "clone-jobs", jobID, "measurements")
}
func (s *Store) measurementPath(jobID, id string) string {
	return filepath.Join(s.measurementsDir(jobID), id+".json")
}
func (s *Store) pcmPath(jobID, id string) string {
	return filepath.Join(s.measurementsDir(jobID), id+".pcm")
}

func (s *Store) CreateMeasurement(jobID, projectID string, binding quality.Binding) (MeasurementRequest, error) {
	if !validRequestComponent(jobID) || !validRequestComponent(projectID) || !validMeasurementBinding(binding) {
		return MeasurementRequest{}, requestError("invalid_request", "Công việc, dự án hoặc binding đo không hợp lệ")
	}
	id, err := randomRequestID()
	if err != nil {
		return MeasurementRequest{}, err
	}
	nonce, token, err := requestCredentials()
	if err != nil {
		return MeasurementRequest{}, err
	}
	now := timestamp(s.now())
	r := MeasurementRequest{Version: 1, ID: id, Nonce: nonce, Token: token, TokenIssuedAt: now, Status: "queued", CreatedAt: now, JobID: jobID, ProjectID: projectID, Binding: binding}
	err = s.withRequestLock("measure-"+id, func() error { return writeRequestJSON(s.measurementPath(jobID, id), r) })
	return r, err
}

func (s *Store) readMeasurement(jobID, id string) (MeasurementRequest, error) {
	var r MeasurementRequest
	data, err := os.ReadFile(s.measurementPath(jobID, id))
	if errors.Is(err, os.ErrNotExist) {
		return r, requestError("not_found", "Yêu cầu đo không tồn tại")
	}
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return r, requestError("not_found", "Yêu cầu đo không hợp lệ")
	}
	if r.ID != id || r.JobID != jobID || r.Version != 1 || !validRequestComponent(r.ProjectID) || r.Nonce == "" || r.CreatedAt == "" || !validMeasurementBinding(r.Binding) ||
		r.ChunksReceived < 0 || r.BytesReceived < 0 || r.BytesReceived > MeasurementMaxPCMBytes ||
		(r.Status != "queued" && r.Status != "running" && !r.Terminal()) ||
		(r.SampleRate != nil && *r.SampleRate != MeasurementSampleRate) || (r.Channels != nil && *r.Channels != MeasurementChannels) {
		return MeasurementRequest{}, requestError("not_found", "Yêu cầu đo không hợp lệ")
	}
	return r, nil
}

func (s *Store) measurementJobs() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(s.Root, "clone-jobs"))
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	jobs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && validRequestComponent(entry.Name()) {
			jobs = append(jobs, entry.Name())
		}
	}
	return jobs, nil
}

func (s *Store) GetMeasurement(id string) (MeasurementRequest, error) {
	if !validRequestComponent(id) {
		return MeasurementRequest{}, requestError("invalid_request", "Mã yêu cầu đo không hợp lệ")
	}
	jobs, err := s.measurementJobs()
	if err != nil {
		return MeasurementRequest{}, err
	}
	for _, jobID := range jobs {
		r, err := s.readMeasurement(jobID, id)
		if err == nil {
			return r, nil
		}
		if requestErrorCode(err) != "not_found" {
			return MeasurementRequest{}, err
		}
	}
	return MeasurementRequest{}, requestError("not_found", "Yêu cầu đo không tồn tại")
}

func (s *Store) ListMeasurements(jobID string) ([]MeasurementRequest, error) {
	if !validRequestComponent(jobID) {
		return nil, requestError("invalid_request", "Mã công việc không hợp lệ")
	}
	out := []MeasurementRequest{}
	entries, err := os.ReadDir(s.measurementsDir(jobID))
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !validRequestComponent(id) {
			continue
		}
		r, err := s.readMeasurement(jobID, id)
		if err != nil {
			if requestErrorCode(err) == "not_found" {
				continue
			}
			return nil, err
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return newerRequest(out[i].CreatedAt, out[j].CreatedAt) })
	return out, nil
}

func (s *Store) AllMeasurements() ([]MeasurementRequest, error) {
	jobs, err := s.measurementJobs()
	if err != nil {
		return nil, err
	}
	out := []MeasurementRequest{}
	for _, jobID := range jobs {
		requests, err := s.ListMeasurements(jobID)
		if err != nil {
			return nil, err
		}
		out = append(out, requests...)
	}
	sort.Slice(out, func(i, j int) bool { return newerRequest(out[i].CreatedAt, out[j].CreatedAt) })
	return out, nil
}

func (s *Store) mutateMeasurement(id string, change func(*MeasurementRequest) error) (MeasurementRequest, error) {
	var r MeasurementRequest
	if !validRequestComponent(id) {
		return r, requestError("invalid_request", "Mã yêu cầu đo không hợp lệ")
	}
	err := s.withRequestLock("measure-"+id, func() error {
		var err error
		r, err = s.GetMeasurement(id)
		if err != nil {
			return err
		}
		if err := change(&r); err != nil {
			return err
		}
		if err := writeRequestJSON(s.measurementPath(r.JobID, id), r); err != nil {
			return err
		}
		if r.Terminal() {
			if err := os.Remove(s.pcmPath(r.JobID, id)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		return nil
	})
	return r, err
}

func measurementFence(r *MeasurementRequest, fence string) error {
	if r.Fence == "" || r.Fence != fence {
		return requestError("conflict", "Fence đo không khớp — claim đã mất")
	}
	return nil
}
func activeMeasurement(r *MeasurementRequest, fence string) error {
	if err := measurementFence(r, fence); err != nil {
		return err
	}
	if r.Status != "running" {
		return requestError("conflict", "Yêu cầu đo không còn chạy")
	}
	return nil
}
func renewMeasurement(r *MeasurementRequest, now time.Time) {
	r.LeaseExpiresAt = timestamp(now.Add(MeasurementLease))
	r.HeartbeatAt = timestamp(now)
}

func (s *Store) ClaimMeasurement(id, nonce, owner string, current quality.Binding) (MeasurementRequest, error) {
	if owner == "" || !validMeasurementBinding(current) {
		return MeasurementRequest{}, requestError("invalid_request", "Chủ runtime hoặc binding không hợp lệ")
	}
	return s.mutateMeasurement(id, func(r *MeasurementRequest) error {
		if r.Nonce != nonce {
			return requestError("conflict", "Nonce đo không khớp — tạo yêu cầu đo mới")
		}
		now := s.now()
		if !quality.SameBinding(r.Binding, current) {
			// Preserve the source claim semantics: even a completed measurement is
			// stale once its current binding differs from the evidence binding.
			r.Status = "stale"
			r.FinishedAt = timestamp(now)
			return nil
		}
		sameOwner := r.OwnerID == owner
		if r.Status != "queued" && !(r.Status == "running" && (r.LeaseExpired(now) || sameOwner)) {
			return requestError("conflict", "Yêu cầu đo đang được runtime khác giữ")
		}
		if r.Fence == "" || !sameOwner {
			fence, err := randomRequestID()
			if err != nil {
				return err
			}
			r.Fence = fence
		}
		r.Status = "running"
		r.OwnerID = owner
		if r.ClaimedAt == "" {
			r.ClaimedAt = timestamp(now)
		}
		renewMeasurement(r, now)
		return nil
	})
}

func (s *Store) HeartbeatMeasurement(id, fence string) (MeasurementRequest, error) {
	return s.mutateMeasurement(id, func(r *MeasurementRequest) error {
		if err := activeMeasurement(r, fence); err != nil {
			return err
		}
		renewMeasurement(r, s.now())
		return nil
	})
}

func (s *Store) AttachMeasurementChrome(id, nonce string, chrome ChromeProcess) (MeasurementRequest, error) {
	if chrome.PID <= 0 || chrome.PGID <= 0 || chrome.ProfileDir == "" {
		return MeasurementRequest{}, requestError("invalid_request", "Tiến trình Chrome không hợp lệ")
	}
	return s.mutateMeasurement(id, func(r *MeasurementRequest) error {
		if r.Nonce != nonce {
			return requestError("conflict", "Nonce đo không khớp")
		}
		chrome.StartedAt = timestamp(s.now())
		r.Chrome = &chrome
		return nil
	})
}

// reconcilePCM discards bytes appended before a crash but never acknowledged in
// JSON. Missing acknowledged data is corruption, not permission to start over.
func reconcilePCM(file string, committed int64) error {
	stat, err := os.Stat(file)
	if errors.Is(err, os.ErrNotExist) {
		if committed == 0 {
			return nil
		}
		return requestError("internal_error", "File PCM bị mất dữ liệu — tạo yêu cầu đo mới")
	}
	if err != nil {
		return err
	}
	if !stat.Mode().IsRegular() || stat.Size() < committed {
		return requestError("internal_error", "File PCM bị mất dữ liệu — tạo yêu cầu đo mới")
	}
	if stat.Size() > committed {
		return os.Truncate(file, committed)
	}
	return nil
}

func (s *Store) AppendPCM(id, fence string, seq int64, bytes []byte, sampleRate, channels int) (ChunkResult, error) {
	if seq < 0 || sampleRate <= 0 || channels <= 0 || channels > len(bytes)/4 || len(bytes) == 0 || len(bytes)%(channels*4) != 0 {
		return ChunkResult{}, requestError("invalid_request", "seq hoặc định dạng chunk PCM không hợp lệ")
	}
	var ack ChunkResult
	_, err := s.mutateMeasurement(id, func(r *MeasurementRequest) error {
		if err := activeMeasurement(r, fence); err != nil {
			return err
		}
		if seq < r.ChunksReceived {
			if seq != r.ChunksReceived-1 {
				return requestError("conflict", "Chunk đã quá cũ")
			}
			ack = ChunkResult{ChunksReceived: r.ChunksReceived, BytesReceived: r.BytesReceived}
			return nil
		}
		if seq != r.ChunksReceived {
			return requestError("conflict", "Chunk lệch thứ tự")
		}
		if r.ChunksReceived == 0 {
			if sampleRate != MeasurementSampleRate || channels != MeasurementChannels {
				return requestError("invalid_request", "PCM phải là f32le stereo 48 kHz")
			}
			r.SampleRate = &sampleRate
			r.Channels = &channels
		} else if r.SampleRate == nil || r.Channels == nil || *r.SampleRate != sampleRate || *r.Channels != channels {
			return requestError("conflict", "Định dạng PCM đổi giữa chừng")
		}
		file := s.pcmPath(r.JobID, id)
		if err := reconcilePCM(file, r.BytesReceived); err != nil {
			return err
		}
		if int64(len(bytes)) > MeasurementMaxPCMBytes-r.BytesReceived {
			return requestError("invalid_request", "Luồng PCM vượt giới hạn đo")
		}
		out, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
		if err != nil {
			return err
		}
		n, writeErr := out.Write(bytes)
		if writeErr == nil && n != len(bytes) {
			writeErr = io.ErrShortWrite
		}
		if writeErr == nil {
			writeErr = out.Sync()
		}
		if err := errors.Join(writeErr, out.Close()); err != nil {
			return err
		}
		r.ChunksReceived++
		r.BytesReceived += int64(len(bytes))
		renewMeasurement(r, s.now())
		ack = ChunkResult{ChunksReceived: r.ChunksReceived, BytesReceived: r.BytesReceived}
		return nil
	})
	return ack, err
}

func (s *Store) CompleteMeasurement(ctx context.Context, id, fence string, measure func(context.Context, string, int, int) (float64, error)) (MeasurementRequest, error) {
	if measure == nil {
		return MeasurementRequest{}, requestError("invalid_request", "Thiếu bộ đo LUFS")
	}
	if err := ctx.Err(); err != nil {
		return MeasurementRequest{}, err
	}
	var file string
	var sampleRate, channels int
	var bytes int64
	_, err := s.mutateMeasurement(id, func(r *MeasurementRequest) error {
		if err := activeMeasurement(r, fence); err != nil {
			return err
		}
		if r.ChunksReceived == 0 || r.BytesReceived == 0 || r.SampleRate == nil || r.Channels == nil {
			return requestError("invalid_request", "Chưa nhận được mẫu âm thanh nào")
		}
		file = s.pcmPath(r.JobID, id)
		if err := reconcilePCM(file, r.BytesReceived); err != nil {
			return err
		}
		sampleRate, channels, bytes = *r.SampleRate, *r.Channels, r.BytesReceived
		r.LeaseExpiresAt = timestamp(s.now().Add(MeasurementCompleteLease))
		return nil
	})
	if err != nil {
		return MeasurementRequest{}, err
	}
	// Do not hold either lock across ebur128: cancellation and lease takeover
	// must remain possible while the external process is working.
	lufs, err := measure(ctx, file, sampleRate, channels)
	if err != nil {
		return MeasurementRequest{}, err
	}
	if err := ctx.Err(); err != nil {
		return MeasurementRequest{}, err
	}
	return s.mutateMeasurement(id, func(r *MeasurementRequest) error {
		if r.Status != "running" || r.BytesReceived != bytes {
			return requestError("conflict", "Luồng PCM đổi trong lúc ebur128 — đo lại")
		}
		if math.IsNaN(lufs) || math.IsInf(lufs, 0) {
			return requestError("invalid_request", "LUFS không hữu hạn — chưa đo được mix")
		}
		if err := measurementFence(r, fence); err != nil {
			return err
		}
		now := timestamp(s.now())
		samples := bytes / int64(channels*4)
		r.Status = "measured"
		r.OwnerID = ""
		r.LeaseExpiresAt = ""
		r.HeartbeatAt = ""
		r.FinishedAt = now
		// Keep the fence so the publisher can still invalidate this result if
		// binding freshness changes after the server-side loudness pass.
		r.Result = &MeasurementResult{LUFS: lufs, Samples: samples, DurationSec: float64(samples) / float64(sampleRate), MeasuredAt: now}
		r.Error = nil
		return nil
	})
}

func finishMeasurement(r *MeasurementRequest, status string, now time.Time) {
	r.Status = status
	r.OwnerID = ""
	r.LeaseExpiresAt = ""
	r.HeartbeatAt = ""
	r.Fence = ""
	r.FinishedAt = timestamp(now)
}

func (s *Store) StaleMeasurement(id, fence, reason string) (MeasurementRequest, error) {
	return s.mutateMeasurement(id, func(r *MeasurementRequest) error {
		if err := measurementFence(r, fence); err != nil {
			return err
		}
		if r.Status != "running" && r.Status != "queued" && r.Status != "measured" {
			return nil
		}
		finishMeasurement(r, "stale", s.now())
		r.Error = &RequestError{Code: "stale", Message: reason}
		return nil
	})
}

func (s *Store) FailMeasurement(id, fence string, failure RequestError) (MeasurementRequest, error) {
	return s.mutateMeasurement(id, func(r *MeasurementRequest) error {
		if err := measurementFence(r, fence); err != nil {
			return err
		}
		if r.Status != "running" && r.Status != "queued" {
			return nil
		}
		finishMeasurement(r, "failed", s.now())
		r.Error = &failure
		return nil
	})
}

func (s *Store) CancelMeasurement(id, nonce string) (MeasurementRequest, error) {
	return s.mutateMeasurement(id, func(r *MeasurementRequest) error {
		if nonce != "" && r.Nonce != nonce {
			return requestError("conflict", "Nonce đo không khớp")
		}
		if !r.Terminal() {
			finishMeasurement(r, "cancelled", s.now())
		}
		return nil
	})
}

// RecoverMeasurement starts the same durable request over after its Chrome
// process has been stopped. No acknowledged PCM survives a new browser run.
func (s *Store) RecoverMeasurement(id string) (MeasurementRequest, error) {
	return s.mutateMeasurement(id, func(r *MeasurementRequest) error {
		if r.Terminal() {
			return requestError("conflict", "Yêu cầu đo đã kết thúc")
		}
		nonce, token, err := requestCredentials()
		if err != nil {
			return err
		}
		if err := os.Remove(s.pcmPath(r.JobID, id)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		r.Nonce = nonce
		r.Token = token
		r.TokenIssuedAt = timestamp(s.now())
		r.Status = "queued"
		r.OwnerID = ""
		r.Fence = ""
		r.ClaimedAt = ""
		r.LeaseExpiresAt = ""
		r.HeartbeatAt = ""
		r.Chrome = nil
		r.ChunksReceived = 0
		r.BytesReceived = 0
		r.SampleRate = nil
		r.Channels = nil
		r.Result = nil
		r.Error = nil
		r.FinishedAt = ""
		return nil
	})
}

// ReclaimOrphanPCM ports the source sweep, with a fresh check under the same
// per-request lock as append/claim so a recovered live upload cannot be deleted.
func (s *Store) ReclaimOrphanPCM(dryRun bool, now time.Time) (OrphanPCMReport, error) {
	report := OrphanPCMReport{Files: []string{}}
	jobs, err := s.measurementJobs()
	if err != nil {
		return report, err
	}
	for _, jobID := range jobs {
		entries, err := os.ReadDir(s.measurementsDir(jobID))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return report, err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".pcm") {
				continue
			}
			id := strings.TrimSuffix(entry.Name(), ".pcm")
			if !validRequestComponent(id) {
				continue
			}
			err := s.withRequestLock("measure-"+id, func() error {
				r, readErr := s.readMeasurement(jobID, id)
				if readErr != nil && requestErrorCode(readErr) != "not_found" {
					return readErr
				}
				var lease time.Time
				if r.LeaseExpiresAt != "" {
					lease, _ = time.Parse(time.RFC3339Nano, r.LeaseExpiresAt)
				}
				if readErr == nil && !r.Terminal() && now.Sub(lease) < MeasurementOrphanPCMGrace {
					return nil
				}
				file := s.pcmPath(jobID, id)
				stat, err := os.Stat(file)
				if errors.Is(err, os.ErrNotExist) {
					return nil
				}
				if err != nil {
					return err
				}
				if !stat.Mode().IsRegular() {
					return nil
				}
				if !dryRun {
					if err := os.Remove(file); err != nil {
						return err
					}
				}
				report.Files = append(report.Files, file)
				report.Bytes += stat.Size()
				return nil
			})
			if err != nil {
				return report, err
			}
		}
	}
	return report, nil
}
