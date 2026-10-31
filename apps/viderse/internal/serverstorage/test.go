package serverstorage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/aws/smithy-go"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobhttp"
	"github.com/datnt1998/verse/platform/blobstore"
)

var probeBytes = bytes.Repeat([]byte("viderse-storage-range-probe-"), 80)

func errorCode(err error) string {
	if err == nil {
		return ""
	}
	var api smithy.APIError
	if errors.As(err, &api) {
		switch api.ErrorCode() {
		case "InvalidAccessKeyId", "SignatureDoesNotMatch", "InvalidToken":
			return "wrong_key"
		case "NoSuchBucket":
			return "no_bucket"
		case "AccessDenied":
			return "no_write_permission"
		case "RequestTimeTooSkewed", "RequestExpired":
			return "clock_skew"
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "unreachable_endpoint"
	}
	var transport interface{ Timeout() bool }
	if errors.As(err, &transport) {
		return "unreachable_endpoint"
	}
	return "storage_error"
}
func (s *Service) candidate(ctx context.Context, c S3Config) (blobstore.Config, media.State, error) {
	state, err := s.opts.Repository.State(ctx)
	if err != nil {
		return blobstore.Config{}, state, err
	}
	id := state.Active
	if state.Pending != "" && !s.opts.EnvLocked {
		id = state.Pending
	}
	d, err := s.opts.Repository.Descriptor(ctx, id)
	if err != nil {
		return blobstore.Config{}, state, err
	}
	old := d.Config
	// Explicit candidates can be tested even while the machine key needs
	// repair. Omitted credentials require the original sealed pair.
	if c.AccessKeyID == "" || c.SecretAccessKey == "" {
		old, err = s.effectiveDescriptor(d)
		if err != nil {
			return old, state, &Error{423, "Không đọc được cặp khoá đã lưu; hãy khôi phục khoá mã hoá hoặc nhập cặp khoá để kiểm tra"}
		}
	}
	cfg, err := normalizeConfig(c, &old)
	if err != nil {
		return cfg, state, &Error{400, err.Error()}
	}
	if s.opts.EnvLocked && configFingerprint(cfg) != configFingerprint(s.opts.Effective) {
		return cfg, state, &Error{403, "Cấu hình do máy chủ đặt; chỉ kiểm tra cấu hình đang dùng"}
	}
	return cfg, state, nil
}
func (s *Service) Test(ctx context.Context, actor string, c S3Config) (TestResult, error) {
	cfg, state, err := s.candidate(ctx, c)
	if err != nil {
		return TestResult{}, err
	}
	id, err := randomID()
	if err != nil {
		return TestResult{}, err
	}
	store, err := blobstore.Open(cfg)
	if err != nil {
		return TestResult{}, &Error{400, "Cấu hình kho chưa hợp lệ"}
	}
	store = blobstore.NewMetrics(store, nil, time.Second)
	fifth := "app_range"
	if cfg.Delivery == "redirect" {
		fifth = "cors_probe"
	}
	result := TestResult{TestID: id, Steps: []Step{{ID: "write", Status: "skip"}, {ID: "read", Status: "skip"}, {ID: "range", Status: "skip"}, {ID: "presign", Status: "skip"}, {ID: fifth, Status: "skip"}, {ID: "delete", Status: "skip"}}}
	session := &testSession{actor: actor, fingerprint: configFingerprint(cfg), revision: state.Revision, expires: time.Now().Add(3 * time.Minute), config: cfg, store: store, key: "staging/connection-test/" + id, result: result}
	session.mu.Lock()
	defer session.mu.Unlock()
	s.mu.Lock()
	if len(s.tests) >= 64 {
		s.mu.Unlock()
		return result, &Error{429, "Có quá nhiều lượt kiểm tra; vui lòng thử lại sau"}
	}
	s.tests[id] = session
	s.mu.Unlock()
	bounded, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	run := func(index int, fn func() error) bool {
		start := time.Now()
		err := fn()
		ms := time.Since(start).Milliseconds()
		session.result.Steps[index].MS = &ms
		session.result.Steps[index].Status = "ok"
		if err != nil {
			session.result.Steps[index].Status = "fail"
			session.result.Error = &TestError{Code: errorCode(err), Step: session.result.Steps[index].ID}
			return false
		}
		return true
	}
	if !run(0, func() error {
		_, err := store.Put(bounded, session.key, bytes.NewReader(probeBytes), blobstore.PutOptions{ContentType: "application/octet-stream"})
		return err
	}) {
		s.finishTest(session)
		return session.result, nil
	}
	if !run(1, func() error { return verifyProbe(bounded, store, session.key, blobstore.FullRange, probeBytes) }) {
		s.finishTest(session)
		return session.result, nil
	}
	if !run(2, func() error {
		return verifyProbe(bounded, store, session.key, blobstore.ByteRange{Offset: 0, Length: 1024}, probeBytes[:1024])
	}) {
		s.finishTest(session)
		return session.result, nil
	}
	if cfg.Delivery == "redirect" {
		if !run(3, func() error {
			signed, err := store.PresignGet(bounded, session.key, 2*time.Minute, blobstore.DownloadOptions{})
			if err == nil {
				session.result.ProbeURL = signed
			}
			return err
		}) {
			s.finishTest(session)
			return session.result, nil
		}
		session.result.Steps[4].Status = "pending"
		return session.result, nil
	}
	session.result.Steps[3].Status = "na"
	run(4, func() error {
		req := httptest.NewRequest(http.MethodGet, "http://viderse.invalid/probe", nil).WithContext(bounded)
		req.Header.Set("Range", "bytes=0-1023")
		rec := httptest.NewRecorder()
		info, err := store.Stat(bounded, session.key)
		if err != nil {
			return err
		}
		if err := blobhttp.Serve(rec, req, store, info, blobhttp.Options{Delivery: "proxy"}); err != nil {
			return err
		}
		if rec.Code != 206 || !bytes.Equal(rec.Body.Bytes(), probeBytes[:1024]) {
			return blobstore.ErrDigestMismatch
		}
		return nil
	})
	s.finishTest(session)
	return session.result, nil
}
func verifyProbe(ctx context.Context, store blobstore.BlobStore, key string, rng blobstore.ByteRange, expected []byte) error {
	body, _, err := store.Open(ctx, key, rng)
	if err != nil {
		return err
	}
	got, readErr := io.ReadAll(io.LimitReader(body, int64(len(expected)+1)))
	closeErr := body.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	if !bytes.Equal(got, expected) {
		return blobstore.ErrDigestMismatch
	}
	return nil
}
func (s *Service) finishTest(session *testSession) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	err := session.store.Delete(ctx, session.key)
	ms := time.Since(start).Milliseconds()
	session.result.Steps[5].MS = &ms
	session.result.Steps[5].Status = "ok"
	session.result.ProbeURL = ""
	if err != nil {
		session.result.Steps[5].Status = "fail"
		code := errorCode(err)
		if code == "no_write_permission" {
			code = "no_delete_permission"
		}
		session.result.Error = &TestError{Code: code, Step: "delete"}
	}
	session.success = session.result.Error == nil
}
func (s *Service) Probe(ctx context.Context, actor, id string, ok bool, reason string) (TestResult, error) {
	if reason != "" && reason != "cors" && reason != "network" {
		return TestResult{}, &Error{400, "Kết quả kiểm tra trình duyệt không hợp lệ"}
	}
	s.mu.Lock()
	session := s.tests[id]
	s.mu.Unlock()
	if session == nil {
		return TestResult{}, conflict("Lượt kiểm tra đã hết hạn")
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session == nil || session.actor != actor || time.Now().After(session.expires) {
		return TestResult{}, conflict("Lượt kiểm tra đã hết hạn")
	}
	state, err := s.opts.Repository.State(ctx)
	if err != nil {
		return TestResult{}, err
	}
	if state.Revision != session.revision {
		return TestResult{}, conflict("Cấu hình đã thay đổi; hãy kiểm tra lại")
	}
	if session.config.Delivery != "redirect" || session.result.Steps[4].Status != "pending" {
		return TestResult{}, conflict("Lượt kiểm tra không chờ trình duyệt")
	}
	session.result.Steps[4].Status = "ok"
	if !ok {
		session.result.Steps[4].Status = "fail"
		code := "storage_error"
		if reason == "cors" {
			code = "cors_blocked"
		} else if reason == "network" {
			code = "browser_unreachable"
		}
		session.result.Error = &TestError{Code: code, Step: "cors_probe"}
	}
	s.finishTest(session)
	return session.result, nil
}
func (s *Service) cleanupTests() {
	now := time.Now()
	s.mu.Lock()
	var expired []*testSession
	for id, test := range s.tests {
		if now.After(test.expires) {
			expired = append(expired, test)
			delete(s.tests, id)
		}
	}
	s.mu.Unlock()
	for _, test := range expired {
		test.mu.Lock()
		if test.result.Steps[5].Status == "skip" {
			s.finishTest(test)
			if test.result.Error != nil {
				s.logFailure("expired_probe_cleanup", errors.New(test.result.Error.Code))
			}
		}
		test.mu.Unlock()
	}
}

func internalBrowserHost(endpoint string) bool {
	return strings.HasPrefix(endpoint, "http://127.") || strings.Contains(endpoint, "localhost")
}
