package render

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
	"github.com/gofrs/flock"
)

var measurementTestBinding = quality.Binding{ProjectRevision: "sha256:r", SettingsHash: "sha256:s", ArtifactHash: "sha256:a"}

func claimedMeasurementForTest(t *testing.T, s *Store) MeasurementRequest {
	t.Helper()
	created, err := s.CreateMeasurement("job-1", "project-1", measurementTestBinding)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.ClaimMeasurement(created.ID, created.Nonce, "owner", measurementTestBinding)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func appendMeasurementForTest(t *testing.T, s *Store, r MeasurementRequest, seq int64) ChunkResult {
	t.Helper()
	ack, err := s.AppendPCM(r.ID, r.Fence, seq, make([]byte, 8), 48000, 2)
	if err != nil {
		t.Fatal(err)
	}
	return ack
}

func measureMinus14(context.Context, string, int, int) (float64, error) { return -14, nil }

func TestMeasurementClaimNonceBindingOwnerAndLease(t *testing.T) {
	s, now := requestTestStore(t)
	created, err := s.CreateMeasurement("job-1", "project-1", measurementTestBinding)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClaimMeasurement(created.ID, "wrong", "owner", measurementTestBinding); err == nil {
		t.Fatal("wrong nonce accepted")
	} else {
		requireRequestCode(t, err, "conflict")
	}
	r, err := s.ClaimMeasurement(created.ID, created.Nonce, "owner", measurementTestBinding)
	if err != nil {
		t.Fatal(err)
	}
	if r.Fence == "" || r.Status != "running" {
		t.Fatalf("claim: %+v", r)
	}
	same, err := s.ClaimMeasurement(r.ID, r.Nonce, "owner", measurementTestBinding)
	if err != nil || same.Fence != r.Fence || same.ClaimedAt != r.ClaimedAt {
		t.Fatalf("same owner recovery: %+v %v", same, err)
	}
	if _, err := s.ClaimMeasurement(r.ID, r.Nonce, "other", measurementTestBinding); err == nil {
		t.Fatal("other owner stole live lease")
	} else {
		requireRequestCode(t, err, "conflict")
	}
	*now = now.Add(MeasurementLease)
	next, err := s.ClaimMeasurement(r.ID, r.Nonce, "new-owner", measurementTestBinding)
	if err != nil || next.Fence == r.Fence || next.Fence == "" {
		t.Fatalf("expired recovery: %+v %v", next, err)
	}
	if _, err := s.AppendPCM(r.ID, r.Fence, 0, make([]byte, 8), 48000, 2); err == nil {
		t.Fatal("old runtime appended")
	} else {
		requireRequestCode(t, err, "conflict")
	}
	if _, err := s.HeartbeatMeasurement(next.ID, "wrong"); err == nil {
		t.Fatal("wrong heartbeat fence accepted")
	}
	*now = now.Add(20 * time.Second)
	beat, err := s.HeartbeatMeasurement(next.ID, next.Fence)
	if err != nil || beat.LeaseExpired(now.Add(59*time.Second)) {
		t.Fatalf("heartbeat: %+v %v", beat, err)
	}
	if beat.HeartbeatStale(now.Add(45*time.Second)) || !beat.HeartbeatStale(now.Add(46*time.Second)) {
		t.Fatal("incorrect heartbeat threshold")
	}

	appendMeasurementForTest(t, s, next, 0)
	changed := measurementTestBinding
	changed.SettingsHash = "changed"
	stale, err := s.ClaimMeasurement(next.ID, next.Nonce, "new-owner", changed)
	if err != nil || stale.Status != "stale" || stale.FinishedAt == "" {
		t.Fatalf("binding drift: %+v %v", stale, err)
	}
	if _, err := os.Stat(s.pcmPath(stale.JobID, stale.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale PCM remains: %v", err)
	}
}

func TestMeasurementChunkSequenceIdempotenceAndAcknowledgment(t *testing.T) {
	s, _ := requestTestStore(t)
	r := claimedMeasurementForTest(t, s)
	if _, err := s.AppendPCM(r.ID, r.Fence, 1, make([]byte, 8), 48000, 2); err == nil {
		t.Fatal("out-of-order chunk accepted")
	} else {
		requireRequestCode(t, err, "conflict")
	}
	ack := appendMeasurementForTest(t, s, r, 0)
	if ack.ChunksReceived != 1 || ack.BytesReceived != 8 {
		t.Fatalf("ack: %+v", ack)
	}
	data, err := json.Marshal(ack)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"chunksReceived":1,"bytesReceived":8}` {
		t.Fatalf("source acknowledgment shape: %s", data)
	}
	duplicate, err := s.AppendPCM(r.ID, r.Fence, 0, bytes.Repeat([]byte{1}, 16), 48000, 2)
	if err != nil || duplicate != ack {
		t.Fatalf("duplicate: %+v %v", duplicate, err)
	}
	file := s.pcmPath(r.JobID, r.ID)
	content, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(content, make([]byte, 8)) {
		t.Fatalf("duplicate changed PCM: %v %v", content, err)
	}
	appendMeasurementForTest(t, s, r, 1)
	if _, err := s.AppendPCM(r.ID, r.Fence, 0, make([]byte, 8), 48000, 2); err == nil {
		t.Fatal("old chunk accepted")
	} else {
		requireRequestCode(t, err, "conflict")
	}
}

func TestMeasurementCrashTailReconcilesAndFormatLocks(t *testing.T) {
	s, _ := requestTestStore(t)
	r := claimedMeasurementForTest(t, s)
	appendMeasurementForTest(t, s, r, 0)
	file := s.pcmPath(r.JobID, r.ID)
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Write(bytes.Repeat([]byte{1}, 11))
	if closeErr := f.Close(); err != nil || closeErr != nil {
		t.Fatalf("crash tail fixture: %v %v", err, closeErr)
	}
	ack, err := s.AppendPCM(r.ID, r.Fence, 1, bytes.Repeat([]byte{2}, 8), 48000, 2)
	if err != nil || ack.BytesReceived != 16 {
		t.Fatalf("append: %+v %v", ack, err)
	}
	content, err := os.ReadFile(file)
	if err != nil || len(content) != 16 || !bytes.Equal(content[8:], bytes.Repeat([]byte{2}, 8)) {
		t.Fatalf("tail not reconciled: %v %v", content, err)
	}
	if _, err := s.AppendPCM(r.ID, r.Fence, 2, make([]byte, 8), 44100, 2); err == nil {
		t.Fatal("PCM format changed")
	} else {
		requireRequestCode(t, err, "conflict")
	}
}

func TestMeasurementMissingAcknowledgedPCMRejectsAppendAndCompletion(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "short", true: "missing"}[missing], func(t *testing.T) {
			s, _ := requestTestStore(t)
			r := claimedMeasurementForTest(t, s)
			appendMeasurementForTest(t, s, r, 0)
			file := s.pcmPath(r.JobID, r.ID)
			var err error
			if missing {
				err = os.Remove(file)
			} else {
				err = os.Truncate(file, 4)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.AppendPCM(r.ID, r.Fence, 1, make([]byte, 8), 48000, 2); err == nil {
				t.Fatal("lost PCM accepted")
			} else {
				requireRequestCode(t, err, "internal_error")
			}
			called := false
			_, err = s.CompleteMeasurement(context.Background(), r.ID, r.Fence, func(context.Context, string, int, int) (float64, error) { called = true; return -14, nil })
			requireRequestCode(t, err, "internal_error")
			if called {
				t.Fatal("measured incomplete file")
			}
		})
	}
}

func TestMeasurementValidatesFixedPCMAndByteLimit(t *testing.T) {
	s, _ := requestTestStore(t)
	r := claimedMeasurementForTest(t, s)
	cases := []struct {
		seq            int64
		data           []byte
		rate, channels int
	}{
		{-1, make([]byte, 8), 48000, 2}, {0, nil, 48000, 2}, {0, make([]byte, 7), 48000, 2},
		{0, make([]byte, 8), 0, 2}, {0, make([]byte, 8), 44100, 2}, {0, make([]byte, 8), 48000, 1},
	}
	for _, tc := range cases {
		if _, err := s.AppendPCM(r.ID, r.Fence, tc.seq, tc.data, tc.rate, tc.channels); err == nil {
			t.Fatalf("invalid chunk accepted: %+v", tc)
		} else {
			requireRequestCode(t, err, "invalid_request")
		}
	}
	appendMeasurementForTest(t, s, r, 0)
	current, err := s.GetMeasurement(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	current.BytesReceived = MeasurementMaxPCMBytes
	writeRequestFixture(t, s.measurementPath(r.JobID, r.ID), current)
	// Sparse length tests the actual 8 GiB boundary without allocating or
	// writing an 8 GiB buffer, just like a real fully acknowledged stream.
	if err := os.Truncate(s.pcmPath(r.JobID, r.ID), MeasurementMaxPCMBytes); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendPCM(r.ID, r.Fence, 1, make([]byte, 8), 48000, 2); err == nil {
		t.Fatal("8 GiB limit exceeded")
	} else {
		requireRequestCode(t, err, "invalid_request")
	}
	persisted, err := s.GetMeasurement(r.ID)
	if err != nil || persisted.BytesReceived != MeasurementMaxPCMBytes || persisted.ChunksReceived != 1 {
		t.Fatalf("failed append changed counters: %+v %v", persisted, err)
	}
}

func TestMeasurementCompletionPublishesEvidenceAndKeepsStaleFence(t *testing.T) {
	s, now := requestTestStore(t)
	r := claimedMeasurementForTest(t, s)
	appendMeasurementForTest(t, s, r, 0)
	measured, err := s.CompleteMeasurement(context.Background(), r.ID, r.Fence, func(ctx context.Context, file string, rate, channels int) (float64, error) {
		if file != s.pcmPath(r.JobID, r.ID) || rate != 48000 || channels != 2 || ctx != context.Background() {
			t.Fatalf("measure arguments: %s %d %d", file, rate, channels)
		}
		inflight, err := s.GetMeasurement(r.ID)
		if err != nil {
			return 0, err
		}
		if inflight.LeaseExpired(now.Add(MeasurementCompleteLease - time.Second)) {
			t.Fatal("completion lease not extended")
		}
		return -14, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if measured.Status != "measured" || measured.OwnerID != "" || measured.LeaseExpiresAt != "" || measured.Fence != r.Fence || measured.Result == nil || measured.Result.LUFS != -14 || measured.Result.Samples != 1 || measured.Result.DurationSec != 1.0/48000 || measured.Result.MeasuredAt == "" {
		t.Fatalf("measured: %+v", measured)
	}
	if _, err := os.Stat(s.pcmPath(r.JobID, r.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("terminal PCM remains: %v", err)
	}
	stale, err := s.StaleMeasurement(r.ID, r.Fence, "project moved")
	if err != nil || stale.Status != "stale" || stale.Fence != "" || stale.Error == nil || stale.Error.Code != "stale" {
		t.Fatalf("post-measure invalidation: %+v %v", stale, err)
	}
}

func TestMeasurementCancelDuringLoudnessCannotPublish(t *testing.T) {
	s, _ := requestTestStore(t)
	r := claimedMeasurementForTest(t, s)
	appendMeasurementForTest(t, s, r, 0)
	_, err := s.CompleteMeasurement(context.Background(), r.ID, r.Fence, func(context.Context, string, int, int) (float64, error) {
		// A mutation inside the external pass also proves completion released
		// its locks while the measurement dependency was running.
		_, err := s.CancelMeasurement(r.ID, r.Nonce)
		if err != nil {
			return 0, err
		}
		return -14, nil
	})
	requireRequestCode(t, err, "conflict")
	persisted, err := s.GetMeasurement(r.ID)
	if err != nil || persisted.Status != "cancelled" || persisted.Result != nil {
		t.Fatalf("cancelled evidence published: %+v %v", persisted, err)
	}
}

func TestMeasurementCompletionRechecksByteCountAndFence(t *testing.T) {
	for _, changed := range []string{"bytes", "fence"} {
		t.Run(changed, func(t *testing.T) {
			s, now := requestTestStore(t)
			r := claimedMeasurementForTest(t, s)
			appendMeasurementForTest(t, s, r, 0)
			_, err := s.CompleteMeasurement(context.Background(), r.ID, r.Fence, func(context.Context, string, int, int) (float64, error) {
				if changed == "bytes" {
					_, err := s.AppendPCM(r.ID, r.Fence, 1, make([]byte, 8), 48000, 2)
					return -14, err
				}
				*now = now.Add(MeasurementCompleteLease)
				_, err := s.ClaimMeasurement(r.ID, r.Nonce, "takeover", measurementTestBinding)
				return -14, err
			})
			requireRequestCode(t, err, "conflict")
			persisted, err := s.GetMeasurement(r.ID)
			if err != nil || persisted.Status != "running" || persisted.Result != nil {
				t.Fatalf("stale result published: %+v %v", persisted, err)
			}
		})
	}
}

func TestMeasurementRejectsIncompleteAndNonfiniteEvidence(t *testing.T) {
	s, _ := requestTestStore(t)
	r := claimedMeasurementForTest(t, s)
	if _, err := s.CompleteMeasurement(context.Background(), r.ID, r.Fence, measureMinus14); err == nil {
		t.Fatal("empty measurement completed")
	} else {
		requireRequestCode(t, err, "invalid_request")
	}
	appendMeasurementForTest(t, s, r, 0)
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, err := s.CompleteMeasurement(context.Background(), r.ID, r.Fence, func(context.Context, string, int, int) (float64, error) { return value, nil })
		requireRequestCode(t, err, "invalid_request")
	}
	failure := errors.New("ebur128 failed")
	if _, err := s.CompleteMeasurement(context.Background(), r.ID, r.Fence, func(context.Context, string, int, int) (float64, error) { return 0, failure }); !errors.Is(err, failure) {
		t.Fatalf("measure error hidden: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := s.CompleteMeasurement(ctx, r.ID, r.Fence, func(context.Context, string, int, int) (float64, error) { cancel(); return -14, nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation hidden: %v", err)
	}
	persisted, err := s.GetMeasurement(r.ID)
	if err != nil || persisted.Status != "running" || persisted.Result != nil {
		t.Fatalf("invalid evidence committed: %+v %v", persisted, err)
	}
}

func TestMeasurementStagedPCMDeletedOnEveryTerminalState(t *testing.T) {
	for _, status := range []string{"cancelled", "failed", "stale", "measured"} {
		t.Run(status, func(t *testing.T) {
			s, _ := requestTestStore(t)
			r := claimedMeasurementForTest(t, s)
			appendMeasurementForTest(t, s, r, 0)
			file := s.pcmPath(r.JobID, r.ID)
			if _, err := os.Stat(file); err != nil {
				t.Fatal(err)
			}
			var settled MeasurementRequest
			var err error
			switch status {
			case "cancelled":
				settled, err = s.CancelMeasurement(r.ID, r.Nonce)
			case "failed":
				settled, err = s.FailMeasurement(r.ID, r.Fence, RequestError{Code: "internal_error", Message: "boom"})
			case "stale":
				settled, err = s.StaleMeasurement(r.ID, r.Fence, "project moved")
			case "measured":
				settled, err = s.CompleteMeasurement(context.Background(), r.ID, r.Fence, measureMinus14)
			}
			if err != nil || settled.Status != status || !settled.Terminal() {
				t.Fatalf("settled: %+v %v", settled, err)
			}
			if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("PCM remains after %s: %v", status, err)
			}
			unchanged, err := s.CancelMeasurement(r.ID, "")
			if err != nil || !reflect.DeepEqual(unchanged, settled) {
				t.Fatalf("terminal cancellation changed evidence: %+v %v", unchanged, err)
			}
			if _, err := s.AppendPCM(r.ID, r.Fence, 1, make([]byte, 8), 48000, 2); err == nil {
				t.Fatal("terminal append accepted")
			}
		})
	}
}

func TestMeasurementOrphanSweepDryRunAndLiveGrace(t *testing.T) {
	s, now := requestTestStore(t)
	live := claimedMeasurementForTest(t, s)
	appendMeasurementForTest(t, s, live, 0)
	settled := claimedMeasurementForTest(t, s)
	if _, err := s.CancelMeasurement(settled.ID, ""); err != nil {
		t.Fatal(err)
	}
	settledFile := s.pcmPath(settled.JobID, settled.ID)
	if err := os.WriteFile(settledFile, make([]byte, 16), 0o600); err != nil {
		t.Fatal(err)
	}
	missingRecord := s.pcmPath("job-1", "no-record")
	if err := os.WriteFile(missingRecord, make([]byte, 32), 0o600); err != nil {
		t.Fatal(err)
	}
	expired := claimedMeasurementForTest(t, s)
	appendMeasurementForTest(t, s, expired, 0)
	expired.LeaseExpiresAt = timestamp(now.Add(-MeasurementOrphanPCMGrace))
	expired.BytesReceived = 8
	expired.ChunksReceived = 1
	rate, channels := 48000, 2
	expired.SampleRate = &rate
	expired.Channels = &channels
	writeRequestFixture(t, s.measurementPath(expired.JobID, expired.ID), expired)
	dry, err := s.ReclaimOrphanPCM(true, *now)
	if err != nil || len(dry.Files) != 3 || dry.Bytes != 56 {
		t.Fatalf("dry report: %+v %v", dry, err)
	}
	for _, file := range dry.Files {
		if _, err := os.Stat(file); err != nil {
			t.Fatalf("dry run deleted %s: %v", file, err)
		}
	}
	report, err := s.ReclaimOrphanPCM(false, *now)
	if err != nil || !reflect.DeepEqual(report, dry) {
		t.Fatalf("sweep: %+v %v", report, err)
	}
	for _, file := range report.Files {
		if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("orphan still exists: %s %v", file, err)
		}
	}
	if _, err := os.Stat(s.pcmPath(live.JobID, live.ID)); err != nil {
		t.Fatalf("live PCM removed: %v", err)
	}
}

func TestMeasurementRecoveryChromeAndScopedToken(t *testing.T) {
	s, _ := requestTestStore(t)
	r := claimedMeasurementForTest(t, s)
	appendMeasurementForTest(t, s, r, 0)
	chrome := ChromeProcess{PID: 456, PGID: 456, ProfileDir: s.ProfileDir(r.ID)}
	attached, err := s.AttachMeasurementChrome(r.ID, r.Nonce, chrome)
	if err != nil || attached.Chrome == nil || attached.Chrome.StartedAt == "" {
		t.Fatalf("chrome: %+v %v", attached, err)
	}
	if _, err := s.AttachMeasurementChrome(r.ID, "wrong", chrome); err == nil {
		t.Fatal("wrong chrome nonce accepted")
	}
	if pid, err := s.CheckToken(r.ID, r.Token, "measure"); err != nil || pid != r.ProjectID {
		t.Fatalf("measurement token: %q %v", pid, err)
	}
	if _, err := s.CheckToken(r.ID, r.Token, "render"); err == nil {
		t.Fatal("measurement token authorized export")
	}
	recovered, err := s.RecoverMeasurement(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Status != "queued" || recovered.ID != r.ID || recovered.Binding != r.Binding || recovered.CreatedAt != r.CreatedAt || recovered.Nonce == r.Nonce || recovered.Token == r.Token || recovered.Fence != "" || recovered.BytesReceived != 0 || recovered.ChunksReceived != 0 || recovered.Channels != nil || recovered.SampleRate != nil || recovered.Chrome != nil || recovered.OwnerID != "" || recovered.ClaimedAt != "" {
		t.Fatalf("recovery: %+v", recovered)
	}
	if _, err := os.Stat(s.pcmPath(r.JobID, r.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("recovery retained PCM: %v", err)
	}
	if _, err := s.CheckToken(r.ID, r.Token, "measure"); err == nil {
		t.Fatal("old token survived recovery")
	}
	if _, err := s.ClaimMeasurement(r.ID, r.Nonce, "owner", r.Binding); err == nil {
		t.Fatal("old nonce survived recovery")
	}
	next, err := s.ClaimMeasurement(r.ID, recovered.Nonce, "owner", r.Binding)
	if err != nil {
		t.Fatal(err)
	}
	appendMeasurementForTest(t, s, next, 0)
	if _, err := s.CancelMeasurement(r.ID, next.Nonce); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CheckToken(r.ID, next.Token, "measure"); err == nil {
		t.Fatal("terminal token accepted")
	}
}

func TestMeasurementListsJobsAndSelectsLiveRequest(t *testing.T) {
	s, now := requestTestStore(t)
	first := claimedMeasurementForTest(t, s)
	*now = now.Add(time.Second)
	queued, err := s.CreateMeasurement("job-1", "project-1", measurementTestBinding)
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Second)
	other, err := s.CreateMeasurement("job-2", "project-2", measurementTestBinding)
	if err != nil {
		t.Fatal(err)
	}
	list, err := s.ListMeasurements("job-1")
	if err != nil || len(list) != 2 || list[0].ID != queued.ID || list[1].ID != first.ID {
		t.Fatalf("job list: %+v %v", list, err)
	}
	all, err := s.AllMeasurements()
	if err != nil || len(all) != 3 || all[0].ID != other.ID {
		t.Fatalf("all list: %+v %v", all, err)
	}
	selected := SelectRelevantMeasurement(list, *now)
	if selected == nil || selected.ID != first.ID {
		t.Fatalf("live selection: %+v", selected)
	}
	selected = SelectRelevantMeasurement(list, now.Add(2*time.Minute))
	if selected == nil || selected.ID != queued.ID {
		t.Fatalf("newest fallback: %+v", selected)
	}
	if SelectRelevantMeasurement(nil, *now) != nil {
		t.Fatal("empty selection was nonnil")
	}
	if _, err := s.ListMeasurements("../job"); err == nil {
		t.Fatal("job path accepted")
	}
	if _, err := s.GetMeasurement("../request"); err == nil {
		t.Fatal("request path accepted")
	}
	if _, err := s.CreateMeasurement("job-1", "project-1", quality.Binding{}); err == nil {
		t.Fatal("empty binding accepted")
	}
}

func TestMeasurementConcurrentDuplicateSerializedAcrossStores(t *testing.T) {
	s, _ := requestTestStore(t)
	r := claimedMeasurementForTest(t, s)
	other := &Store{Root: s.Root, Now: s.Now}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, store := range []*Store{s, other} {
		wg.Add(1)
		go func(store *Store) {
			defer wg.Done()
			_, err := store.AppendPCM(r.ID, r.Fence, 0, make([]byte, 8), 48000, 2)
			results <- err
		}(store)
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	persisted, err := s.GetMeasurement(r.ID)
	if err != nil || persisted.BytesReceived != 8 || persisted.ChunksReceived != 1 {
		t.Fatalf("duplicate race changed counters: %+v %v", persisted, err)
	}
	stat, err := os.Stat(s.pcmPath(r.JobID, r.ID))
	if err != nil || stat.Size() != 8 {
		t.Fatalf("duplicate race changed file: %v %v", stat, err)
	}
}

func TestRequestStoreHonorsFilesystemFlock(t *testing.T) {
	s, _ := requestTestStore(t)
	r, err := s.CreateExport("project-1", "")
	if err != nil {
		t.Fatal(err)
	}
	lock := flock.New(filepath.Join(s.Root, ".locks", "export-"+r.ID+".lock"))
	if err := lock.Lock(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lock.Unlock(); err != nil {
			t.Error(err)
		}
	}()
	started := make(chan struct{})
	finished := make(chan error, 1)
	go func() { close(started); _, err := s.ClaimExport(r.ID, r.Nonce); finished <- err }()
	<-started
	select {
	case err := <-finished:
		t.Fatalf("claim did not respect external file lock: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	if err := lock.Unlock(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("claim remained blocked after external lock released")
	}
}
