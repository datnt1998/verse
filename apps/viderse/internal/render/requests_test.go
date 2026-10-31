package render

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func requestTestStore(t *testing.T) (*Store, *time.Time) {
	t.Helper()
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	return &Store{Root: t.TempDir(), Now: func() time.Time { return now }}, &now
}

func requireRequestCode(t *testing.T, err error, code string) {
	t.Helper()
	var e RequestError
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("error = %v; want request error %s", err, code)
	}
}

func claimedExportForTest(t *testing.T, s *Store) ExportRequest {
	t.Helper()
	created, err := s.CreateExport("project-1", "job-1")
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.ClaimExport(created.ID, created.Nonce)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func writeRequestFixture(t *testing.T, file string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestExportClaimRequiresNonce(t *testing.T) {
	s, _ := requestTestStore(t)
	created, err := s.CreateExport("project-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != "queued" || created.JobID != "" || created.Fence != "" || created.Version != 1 {
		t.Fatalf("created: %+v", created)
	}
	if _, err := s.ClaimExport(created.ID, "wrong"); err == nil {
		t.Fatal("wrong nonce accepted")
	} else {
		requireRequestCode(t, err, "conflict")
	}
	r, err := s.ClaimExport(created.ID, created.Nonce)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "running" || r.Fence == "" || r.LeaseExpiresAt == "" || r.ClaimedAt == "" {
		t.Fatalf("claimed: %+v", r)
	}
	if _, err := s.ClaimExport(created.ID, created.Nonce); err == nil {
		t.Fatal("live claim stolen")
	} else {
		requireRequestCode(t, err, "conflict")
	}
}

func TestExportFenceAndExpiredLeaseReclaim(t *testing.T) {
	s, now := requestTestStore(t)
	first := claimedExportForTest(t, s)
	if _, err := s.HeartbeatExport(first.ID, "wrong"); err == nil {
		t.Fatal("wrong fence accepted")
	} else {
		requireRequestCode(t, err, "conflict")
	}
	*now = now.Add(ExportRequestLease)
	if !first.LeaseExpired(*now) {
		t.Fatal("lease must expire at its boundary")
	}
	next, err := s.ClaimExport(first.ID, first.Nonce)
	if err != nil {
		t.Fatal(err)
	}
	if next.Fence == "" || next.Fence == first.Fence || next.ClaimedAt != first.ClaimedAt {
		t.Fatalf("reclaim: %+v", next)
	}
	if _, err := s.HeartbeatExport(first.ID, first.Fence); err == nil {
		t.Fatal("superseded holder accepted")
	} else {
		requireRequestCode(t, err, "conflict")
	}
	*now = now.Add(20 * time.Second)
	beat, err := s.HeartbeatExport(next.ID, next.Fence)
	if err != nil {
		t.Fatal(err)
	}
	if beat.LeaseExpired(now.Add(59*time.Second)) || !beat.LeaseExpired(now.Add(time.Minute)) {
		t.Fatal("heartbeat did not renew lease")
	}
}

func TestExportBindingWriteOnceAndCompletion(t *testing.T) {
	s, _ := requestTestStore(t)
	r := claimedExportForTest(t, s)
	bound, err := s.BindExport(r.ID, r.Fence, "exp-1")
	if err != nil {
		t.Fatal(err)
	}
	if bound.ExportID != "exp-1" {
		t.Fatalf("bound: %+v", bound)
	}
	if _, err := s.BindExport(r.ID, r.Fence, "exp-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindExport(r.ID, r.Fence, "exp-2"); err == nil {
		t.Fatal("binding changed")
	} else {
		requireRequestCode(t, err, "conflict")
	}
	// The source preserves the already-bound ID even if a completion carries
	// a different one; completion cannot silently replace the staged export.
	done, err := s.CompleteExport(r.ID, r.Fence, "exp-other")
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != "done" || done.ExportID != "exp-1" || done.Fence != "" || done.LeaseExpiresAt != "" || done.HeartbeatAt != "" || done.FinishedAt == "" {
		t.Fatalf("done: %+v", done)
	}
	if _, err := s.HeartbeatExport(r.ID, r.Fence); err == nil {
		t.Fatal("terminal heartbeat accepted")
	} else {
		requireRequestCode(t, err, "conflict")
	}
	unbound := claimedExportForTest(t, s)
	completed, err := s.CompleteExport(unbound.ID, unbound.Fence, "exp-new")
	if err != nil || completed.ExportID != "exp-new" {
		t.Fatalf("unbound completion: %+v %v", completed, err)
	}
}

func TestExportProgressUploadingNeverWalksBack(t *testing.T) {
	s, now := requestTestStore(t)
	r := claimedExportForTest(t, s)
	ten, hundred := int64(10), int64(100)
	p := ExportProgress{Phase: "rendering", FramesDone: &ten, FramesTotal: &hundred}
	*now = now.Add(20 * time.Second)
	rendered, err := s.ProgressExport(r.ID, r.Fence, p)
	if err != nil || rendered.Status != "running" {
		t.Fatalf("render progress: %+v %v", rendered, err)
	}
	p = ExportProgress{Phase: "uploading", BytesSent: &ten, BytesTotal: &hundred}
	uploading, err := s.ProgressExport(r.ID, r.Fence, p)
	if err != nil || uploading.Status != "uploading" || !reflect.DeepEqual(uploading.Progress, &p) {
		t.Fatalf("upload progress: %+v %v", uploading, err)
	}
	if uploading.LeaseExpired(now.Add(59 * time.Second)) {
		t.Fatal("progress did not renew lease")
	}
	later, err := s.ProgressExport(r.ID, r.Fence, ExportProgress{Phase: "rendering"})
	if err != nil || later.Status != "uploading" {
		t.Fatalf("backwards transition: %+v %v", later, err)
	}
	negative := int64(-1)
	if _, err := s.ProgressExport(r.ID, r.Fence, ExportProgress{FramesDone: &negative}); err == nil {
		t.Fatal("negative counter accepted")
	} else {
		requireRequestCode(t, err, "invalid_request")
	}
}

func TestExportTerminalStatesRemainImmutable(t *testing.T) {
	s, _ := requestTestStore(t)
	for _, status := range []string{"failed", "cancelled", "stale"} {
		t.Run(status, func(t *testing.T) {
			r := claimedExportForTest(t, s)
			var settled ExportRequest
			var err error
			switch status {
			case "failed":
				settled, err = s.FailExport(r.ID, r.Fence, RequestError{Code: "x", Message: "boom"})
			case "cancelled":
				settled, err = s.CancelExport(r.ID, r.Nonce)
			case "stale":
				settled, err = s.StaleExport(r.ID, "swept")
			}
			if err != nil || settled.Status != status || !settled.Terminal() {
				t.Fatalf("settle: %+v %v", settled, err)
			}
			cancelled, err := s.CancelExport(r.ID, "")
			if err != nil || !reflect.DeepEqual(cancelled, settled) {
				t.Fatalf("cancel changed terminal: %+v %v", cancelled, err)
			}
			stale, err := s.StaleExport(r.ID, "again")
			if err != nil || !reflect.DeepEqual(stale, settled) {
				t.Fatalf("stale changed terminal: %+v %v", stale, err)
			}
			read, err := s.GetExport(r.ID)
			if err != nil || !reflect.DeepEqual(read, settled) {
				t.Fatalf("persisted: %+v %v", read, err)
			}
		})
	}
}

func TestExportChromeNonceAndProfile(t *testing.T) {
	s, _ := requestTestStore(t)
	r, err := s.CreateExport("project-1", "")
	if err != nil {
		t.Fatal(err)
	}
	chrome := ChromeProcess{PID: 123, PGID: 123, ProfileDir: s.ProfileDir(r.ID)}
	attached, err := s.AttachChrome(r.ID, r.Nonce, chrome)
	if err != nil {
		t.Fatal(err)
	}
	chrome.StartedAt = attached.Chrome.StartedAt
	if chrome.StartedAt == "" || !reflect.DeepEqual(attached.Chrome, &chrome) {
		t.Fatalf("chrome: %+v", attached.Chrome)
	}
	if chrome.ProfileDir != filepath.Join(s.Root, "chrome-profiles", r.ID) || s.ProfileDir("../escape") != "" {
		t.Fatal("unsafe profile path")
	}
	if _, err := s.AttachChrome(r.ID, "wrong", chrome); err == nil {
		t.Fatal("wrong nonce accepted")
	} else {
		requireRequestCode(t, err, "conflict")
	}
}

func TestExportHeartbeatStaleThreshold(t *testing.T) {
	s, now := requestTestStore(t)
	r := claimedExportForTest(t, s)
	if r.HeartbeatStale(now.Add(45*time.Second)) || !r.HeartbeatStale(now.Add(45*time.Second+time.Nanosecond)) {
		t.Fatal("incorrect stale threshold")
	}
	r.Status = "uploading"
	if !r.HeartbeatStale(now.Add(time.Minute)) {
		t.Fatal("upload heartbeat not monitored")
	}
	r.Status = "done"
	if r.HeartbeatStale(now.Add(time.Hour)) {
		t.Fatal("terminal heartbeat monitored")
	}
	r.Status = "running"
	r.HeartbeatAt = ""
	if r.HeartbeatStale(now.Add(time.Hour)) {
		t.Fatal("missing heartbeat reported stale")
	}
}

func TestExportScopedTokensExpireAndRevoke(t *testing.T) {
	s, now := requestTestStore(t)
	r, err := s.CreateExport("project-1", "job-1")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := hex.DecodeString(r.Token)
	if err != nil || len(decoded) != 32 {
		t.Fatalf("token is not a 32-byte secret: %v", err)
	}
	if pid, err := s.CheckToken(r.ID, r.Token, "export"); err != nil || pid != "project-1" {
		t.Fatalf("queued authorization: %q %v", pid, err)
	}
	for _, token := range []string{"", r.Nonce, "not-hex", r.Token[:62], r.Token + "00"} {
		if _, err := s.CheckToken(r.ID, token, "export"); err == nil {
			t.Fatal("invalid token accepted")
		} else {
			requireRequestCode(t, err, "unauthorized")
		}
	}
	other, err := s.CreateExport("project-2", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CheckToken(other.ID, r.Token, "export"); err == nil {
		t.Fatal("token crossed request scope")
	}
	if _, err := s.CheckToken(r.ID, r.Token, "measure"); err == nil {
		t.Fatal("token crossed request kind")
	}
	*now = now.Add(RequestTokenTTL)
	if _, err := s.CheckToken(r.ID, r.Token, "export"); err == nil {
		t.Fatal("expired token accepted")
	}
	*now = now.Add(-RequestTokenTTL)
	if _, err := s.CancelExport(r.ID, r.Nonce); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CheckToken(r.ID, r.Token, "export"); err == nil {
		t.Fatal("terminal token accepted")
	}
}

func TestExportRecoveryResetsRunAndInvalidatesCapabilities(t *testing.T) {
	s, _ := requestTestStore(t)
	r := claimedExportForTest(t, s)
	if _, err := s.BindExport(r.ID, r.Fence, "exp-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ProgressExport(r.ID, r.Fence, ExportProgress{Phase: "uploading"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AttachChrome(r.ID, r.Nonce, ChromeProcess{PID: 1, PGID: 1, ProfileDir: s.ProfileDir(r.ID)}); err != nil {
		t.Fatal(err)
	}
	recovered, err := s.RecoverExport(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.ID != r.ID || recovered.CreatedAt != r.CreatedAt || recovered.Status != "queued" || recovered.Nonce == r.Nonce || recovered.Token == r.Token || recovered.Fence != "" || recovered.ExportID != "" || recovered.Progress != nil || recovered.Chrome != nil || recovered.ClaimedAt != "" {
		t.Fatalf("recovery: %+v", recovered)
	}
	if _, err := s.CheckToken(r.ID, r.Token, "export"); err == nil {
		t.Fatal("old token survived recovery")
	}
	if _, err := s.ClaimExport(r.ID, r.Nonce); err == nil {
		t.Fatal("old nonce survived recovery")
	}
	if _, err := s.HeartbeatExport(r.ID, r.Fence); err == nil {
		t.Fatal("old fence survived recovery")
	}
	if _, err := s.ClaimExport(r.ID, recovered.Nonce); err != nil {
		t.Fatal(err)
	}
}

func TestExportListingAndRetention(t *testing.T) {
	s, now := requestTestStore(t)
	old, err := s.CreateExport("project-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CancelExport(old.ID, ""); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Second)
	live, err := s.CreateExport("project-2", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Root, "export-requests", "corrupt.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	requests, err := s.ListExports()
	if err != nil || len(requests) != 2 || requests[0].ID != live.ID || requests[1].ID != old.ID {
		t.Fatalf("listing: %+v %v", requests, err)
	}
	prunable, err := s.ListPrunableExports(now.Add(ExportRequestRetention+time.Second), 0)
	if err != nil || len(prunable) != 1 || prunable[0].ID != old.ID {
		t.Fatalf("prunable: %+v %v", prunable, err)
	}
	files := s.ExportRequestFiles(old.ID)
	if len(files) != 2 || files[0] != s.exportPath(old.ID) || files[1] != s.ProfileDir(old.ID) {
		t.Fatalf("files: %v", files)
	}
}

func TestExportMutationsSerializeAcrossStoreInstances(t *testing.T) {
	s, _ := requestTestStore(t)
	r, err := s.CreateExport("project-1", "")
	if err != nil {
		t.Fatal(err)
	}
	other := &Store{Root: s.Root, Now: s.Now}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, store := range []*Store{s, other} {
		wg.Add(1)
		go func(store *Store) { defer wg.Done(); _, err := store.ClaimExport(r.ID, r.Nonce); results <- err }(store)
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else {
			requireRequestCode(t, err, "conflict")
			conflicts++
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("claims: success=%d conflicts=%d", success, conflicts)
	}
}

func TestRequestPathsRejectTraversal(t *testing.T) {
	s, _ := requestTestStore(t)
	if _, err := s.CreateExport("../project", ""); err == nil {
		t.Fatal("project path accepted")
	}
	if _, err := s.GetExport("../escape"); err == nil {
		t.Fatal("request path accepted")
	} else {
		requireRequestCode(t, err, "invalid_request")
	}
	if _, err := s.ClaimExport("../escape", "n"); err == nil {
		t.Fatal("mutation path accepted")
	}
	if _, err := s.GetExport("missing"); err == nil {
		t.Fatal("missing request found")
	} else {
		requireRequestCode(t, err, "not_found")
	}
}

func TestRequestRecoveryRotatesExpiredTokens(t *testing.T) {
	for _, kind := range []string{"export", "measurement"} {
		t.Run(kind, func(t *testing.T) {
			s, now := requestTestStore(t)
			var id, originalToken, createdAt string
			var recoverToken func() (string, error)
			if kind == "export" {
				r := claimedExportForTest(t, s)
				id, originalToken, createdAt = r.ID, r.Token, r.CreatedAt
				recoverToken = func() (string, error) {
					recovered, err := s.RecoverExport(id)
					if err == nil && recovered.CreatedAt != createdAt {
						t.Fatal("recovery changed request creation time")
					}
					return recovered.Token, err
				}
			} else {
				r := claimedMeasurementForTest(t, s)
				id, originalToken, createdAt = r.ID, r.Token, r.CreatedAt
				recoverToken = func() (string, error) {
					recovered, err := s.RecoverMeasurement(id)
					if err == nil && recovered.CreatedAt != createdAt {
						t.Fatal("recovery changed request creation time")
					}
					return recovered.Token, err
				}
			}
			*now = now.Add(RequestTokenTTL + time.Hour)
			if _, err := s.CheckToken(id, originalToken, kind); err == nil {
				t.Fatal("original expired token accepted")
			}
			rotated, err := recoverToken()
			if err != nil {
				t.Fatal(err)
			}
			if rotated == originalToken {
				t.Fatal("recovery retained expired token")
			}
			if projectID, err := s.CheckToken(id, rotated, kind); err != nil || projectID != "project-1" {
				t.Fatalf("fresh recovery token rejected after long downtime: project=%q error=%v", projectID, err)
			}
			if _, err := s.CheckToken(id, originalToken, kind); err == nil {
				t.Fatal("original token survived recovery")
			}
			*now = now.Add(RequestTokenTTL)
			if _, err := s.CheckToken(id, rotated, kind); err == nil {
				t.Fatal("recovery token exceeded its four-hour lifetime")
			}
		})
	}
}

func TestRequestTokenIssuedAtLegacyFallback(t *testing.T) {
	for _, kind := range []string{"export", "measurement"} {
		t.Run(kind, func(t *testing.T) {
			s, now := requestTestStore(t)
			var id, token, file string
			var record any
			if kind == "export" {
				r := claimedExportForTest(t, s)
				id, token, file, record = r.ID, r.Token, s.exportPath(r.ID), r
			} else {
				r := claimedMeasurementForTest(t, s)
				id, token, file, record = r.ID, r.Token, s.measurementPath(r.JobID, r.ID), r
			}
			data, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			var legacy map[string]json.RawMessage
			if err := json.Unmarshal(data, &legacy); err != nil {
				t.Fatal(err)
			}
			delete(legacy, "tokenIssuedAt")
			writeRequestFixture(t, file, legacy)
			if _, err := s.CheckToken(id, token, kind); err != nil {
				t.Fatalf("legacy live token rejected: %v", err)
			}
			*now = now.Add(RequestTokenTTL)
			if _, err := s.CheckToken(id, token, kind); err == nil {
				t.Fatal("legacy token ignored creation-time expiry")
			}
		})
	}
}
