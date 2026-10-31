package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

func cloneTestEnv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestCloneParseEnqueueAndExistingJob(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		channel  string
		priority domain.ClonePriority
		wait     bool
		noImages bool
		urls     int
	}{
		{"manual URL", []string{"run", "https://youtube.com/watch?v=video1", "--channel", "owner"}, "owner", domain.ClonePick, false, false, 1},
		{"source owner flag", []string{"run", "--for", "owner", "https://youtu.be/video1", "--no-images"}, "owner", domain.ClonePick, false, true, 1},
		{"batch URLs", []string{"batch", "https://youtu.be/video1", "--channel=owner", "https://youtu.be/video2"}, "owner", domain.CloneBatch, false, false, 2},
		{"manual batch priority", []string{"run", "https://youtu.be/video1", "--channel", "owner", "--priority=batch"}, "owner", domain.CloneBatch, false, false, 1},
		{"batch pick priority", []string{"batch", "--priority", "pick", "--channel", "owner", "https://youtu.be/video1"}, "owner", domain.ClonePick, false, false, 1},
		{"existing job", []string{"run", "job-1"}, "", domain.ClonePick, false, false, 1},
		{"existing job wait", []string{"run", "job-1", "--wait", "--timeout", "0.5"}, "", domain.ClonePick, true, false, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o, err := parseCloneOptions(tc.args, cloneTestEnv(nil))
			if err != nil {
				t.Fatal(err)
			}
			if o.channel != tc.channel || o.priority != tc.priority || o.wait != tc.wait || o.noImages != tc.noImages || len(o.operands) != tc.urls {
				t.Fatalf("options = %+v", o)
			}
			if tc.wait && o.timeout != 500*time.Millisecond {
				t.Fatalf("timeout = %s", o.timeout)
			}
		})
	}
}

func TestCloneParseRejectsInvalidInputBeforeHTTP(t *testing.T) {
	cases := [][]string{
		nil,
		{"unknown"},
		{"run"},
		{"run", "job-1", "job-2"},
		{"run", "https://youtu.be/video1"},
		{"run", "job-1", "--priority", "pick"},
		{"run", "job-1", "--no-images"},
		{"run", "https://youtu.be/video1", "--channel", "owner", "--priority", "urgent"},
		{"run", "https://youtu.be/video1", "--channel", "owner", "--priority"},
		{"run", "https://youtu.be/video1", "--channel", "owner", "--for", "other"},
		{"run", "https://youtu.be/video1", "--channel", ""},
		{"run", "ftp://youtube.com/video", "--for", "owner"},
		{"run", "https://user:password@youtube.com/video", "--channel", "owner"},
		{"run", "not-a-url", "--channel", "owner"},
		{"run", "https://youtu.be/video1", "--channel", "owner", "--wait"},
		{"batch"},
		{"batch", "https://youtu.be/video1"},
		{"batch", "https://youtu.be/video1", "--for", "owner", "--all"},
		{"batch", "https://youtu.be/video1", "--for", "owner", "--wait"},
		{"pause"},
		{"pause", "job-1", "--wait"},
		{"resume", "job-1", "extra"},
		{"cancel", "../job"},
		{"cancel", ".."},
		{"cancel", "job?id=other"},
		{"status", "job-1", "job-2"},
		{"status", "job-1", "--counts"},
		{"status", "job-1", "--channel", "owner"},
		{"status", "--counts", "--limit", "5"},
		{"status", "--watch"},
		{"status", "--limit", "0"},
		{"status", "--limit", "101"},
		{"status", "--limit", "1.5"},
		{"status", "--limit"},
		{"wait"},
		{"wait", "job-1", "--watch"},
		{"wait", "job-1", "--timeout", "NaN"},
		{"wait", "job-1", "--timeout", "Inf"},
		{"wait", "job-1", "--timeout", "0"},
		{"wait", "job-1", "--timeout", "-1"},
		{"wait", "job-1", "--timeout", "1m"},
		{"wait", "job-1", "--timeout", "1e100"},
		{"wait", "job-1", "--timeout", "0.00000000001"},
		{"wait", "job-1", "--timeout"},
		{"wait", "job-1", "--counts"},
		{"run", "job-1", "--timeout", "10"},
		{"run", "job-1", "--wait=true"},
		{"status", "--counts=true"},
		{"status", "--limit", "3", "--limit", "4"},
		{"status", "--unknown"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, err := parseCloneOptions(args, cloneTestEnv(nil))
			var cliErr *cloneCLIError
			if !errors.As(err, &cliErr) || cliErr.Code != domain.CloneErrInvalidRequest {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestCloneConnectionSettingsAndOverrides(t *testing.T) {
	env := cloneTestEnv(map[string]string{
		"PUBLIC_URL": "https://viderse.example.test/", "VIDERSE_ADDR": "127.0.0.1:8821", "VIDERSE_COOKIE": "verse_session=session-fixture",
	})
	o, err := parseCloneOptions([]string{"status"}, env)
	if err != nil || o.api != "https://viderse.example.test" || o.cookie != "verse_session=session-fixture" || o.limit != 50 {
		t.Fatalf("environment = %+v, %v", o, err)
	}
	o, err = parseCloneOptions([]string{"status", "--api", "http://127.0.0.1:8822", "--cookie", "verse_session=override-fixture"}, env)
	if err != nil || o.api != "http://127.0.0.1:8822" || o.cookie != "verse_session=override-fixture" {
		t.Fatalf("flags = %+v, %v", o, err)
	}
	o, err = parseCloneOptions([]string{"status"}, cloneTestEnv(map[string]string{"VIDERSE_ADDR": "127.0.0.1:8821"}))
	if err != nil || o.api != "http://127.0.0.1:8821" {
		t.Fatalf("address = %+v, %v", o, err)
	}
	for _, api := range []string{"ftp://example.test", "https://user:secret@example.test", "https://example.test?key=x", "https://example.test#fragment", "://invalid"} {
		if _, err := parseCloneOptions([]string{"status", "--api", api}, cloneTestEnv(nil)); err == nil {
			t.Fatalf("accepted invalid endpoint %q", api)
		}
	}
	if _, err := parseCloneOptions([]string{"status"}, cloneTestEnv(map[string]string{"VIDERSE_COOKIE": "verse_session=x\r\nX-Evil: yes"})); err == nil {
		t.Fatal("accepted header injection")
	}
}

func TestCloneCommandEnqueuesAuthenticatedDTOAndPreservesDuplicates(t *testing.T) {
	for _, command := range []string{"run", "batch"} {
		t.Run(command, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost || r.URL.Path != "/api/clone/enqueue" || r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("request = %s %s %v", r.Method, r.URL, r.Header)
				}
				cookie, err := r.Cookie("verse_session")
				if err != nil || cookie.Value != "session-fixture" {
					t.Error("missing authenticated session")
				}
				var in domain.CloneEnqueueRequest
				if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
					t.Error(err)
				}
				priority := domain.ClonePick
				if command == "batch" {
					priority = domain.CloneBatch
				}
				if in.ChannelID != "owner" || len(in.URLs) != 1 || in.URLs[0] != "https://youtu.be/video1" || in.Priority != priority || !in.NoImages {
					t.Errorf("enqueue = %+v", in)
				}
				w.WriteHeader(http.StatusCreated)
				io.WriteString(w, `{"items":[],"duplicates":[{"url":"https://youtu.be/video1","jobId":"job-1","state":"started"}]}`)
			}))
			defer server.Close()
			t.Setenv("PUBLIC_URL", server.URL)
			t.Setenv("VIDERSE_COOKIE", "verse_session=session-fixture")
			var out, errBuf bytes.Buffer
			code := runCloneCommandContext(context.Background(), []string{command, "https://youtu.be/video1", "--for", "owner", "--no-images"}, &out, &errBuf)
			if code != 0 || calls != 1 || errBuf.Len() != 0 || !strings.Contains(out.String(), `"jobId":"job-1"`) {
				t.Fatalf("enqueue = %d calls=%d stdout=%s stderr=%s", code, calls, &out, &errBuf)
			}
		})
	}
}

func TestCloneJobActionsUseOneActionRequest(t *testing.T) {
	for _, action := range []string{"run", "pause", "resume", "cancel"} {
		t.Run(action, func(t *testing.T) {
			calls := 0
			client := cloneTestClient(func(r *http.Request) (int, string, error) {
				calls++
				if r.Method != http.MethodPost || r.URL.Path != "/api/clone/job-1" {
					t.Fatalf("request = %s %s", r.Method, r.URL)
				}
				var body struct {
					Action string `json:"action"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Action != action {
					t.Fatalf("action = %q, %v", body.Action, err)
				}
				return 200, `{"id":"job-1","status":"running"}`, nil
			})
			o, err := parseCloneOptions([]string{action, "job-1"}, cloneTestEnv(nil))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.execute(context.Background(), o); err != nil || calls != 1 {
				t.Fatalf("action: calls=%d, error=%v", calls, err)
			}
		})
	}
}

func TestCloneStatusReadsOneListOrCounts(t *testing.T) {
	cases := []struct {
		args []string
		path string
	}{
		{[]string{"status", "job-1"}, "/api/clone/job-1"},
		{[]string{"status"}, "/api/clone?limit=50"},
		{[]string{"status", "--channel", "owner & one", "--limit", "12"}, "/api/clone?channelId=owner+%26+one&limit=12"},
		{[]string{"status", "--counts"}, "/api/clone/counts"},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			client := cloneTestClient(func(r *http.Request) (int, string, error) {
				if r.Method != http.MethodGet || r.URL.RequestURI() != tc.path {
					t.Fatalf("request = %s %s", r.Method, r.URL.RequestURI())
				}
				return 200, `{"stored":"projection"}`, nil
			})
			o, err := parseCloneOptions(tc.args, cloneTestEnv(nil))
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.execute(context.Background(), o)
			if err != nil || string(result) != `{"stored":"projection"}` {
				t.Fatalf("result=%s error=%v", result, err)
			}
		})
	}
}

func TestCloneWaitPollsUntilTerminalAndReview(t *testing.T) {
	// Ported from cli/clone.test.ts: cancelled, completed and review checkpoints
	// settle normally, while retry/paused states must not masquerade as completion.
	for _, terminal := range []domain.CloneStatus{domain.CloneSucceeded, domain.CloneCancelled, domain.CloneWaiting} {
		t.Run(string(terminal), func(t *testing.T) {
			calls := 0
			client := cloneTestClient(func(r *http.Request) (int, string, error) {
				if r.Method != http.MethodGet || r.URL.Path != "/api/clone/job-1" {
					t.Fatalf("request = %s %s", r.Method, r.URL)
				}
				calls++
				view := domain.CloneJobView{CloneJob: domain.CloneJob{ID: "job-1", Status: domain.CloneRunning}}
				switch calls {
				case 1:
					view.Status = domain.ClonePending
				case 2:
					view.Status = domain.CloneWaiting
					view.Steps = []domain.CloneStepRecord{{Step: domain.CloneImages, Status: domain.CloneWaiting, Error: &domain.CloneError{Code: domain.CloneErrQuota}}}
				case 3:
					now := time.Now()
					view.Status, view.PausedAt = domain.CloneWaiting, &now
					view.Steps = []domain.CloneStepRecord{{Step: domain.CloneBuild, Status: domain.CloneWaiting, Error: &domain.CloneError{Code: domain.CloneErrAwaitingReview}}}
				default:
					view.Status = terminal
					if terminal == domain.CloneWaiting {
						view.StageGroup = domain.StageReview
						view.Steps = []domain.CloneStepRecord{{Step: domain.CloneBuild, Status: domain.CloneWaiting, Error: &domain.CloneError{Code: domain.CloneErrAwaitingReview}}}
					}
				}
				return 200, cloneTestViewJSON(t, view), nil
			})
			result, err := client.wait(context.Background(), "job-1", time.Second)
			if err != nil || calls != 4 {
				t.Fatalf("wait calls=%d result=%s error=%v", calls, result, err)
			}
			var got domain.CloneJobView
			if err := json.Unmarshal(result, &got); err != nil || got.Status != terminal {
				t.Fatalf("terminal = %s, %v", got.Status, err)
			}
		})
	}
}

func TestCloneWaitFailedJobReturnsStepRecoveryAndCleanStdout(t *testing.T) {
	// Ported failure contract: a failed --watch is exit 1 with job/step/code/
	// recovery on stderr, never a successful snapshot document on stdout.
	recovery := "Chạy lại sau khi sửa"
	view := domain.CloneJobView{
		CloneJob: domain.CloneJob{ID: "job-1", Status: domain.CloneFailed},
		Steps: []domain.CloneStepRecord{{Step: domain.CloneDownload, Status: domain.CloneFailed, Error: &domain.CloneError{
			Code: domain.CloneErrSourceUnavailable, Message: "Không tải được video", Recovery: &recovery,
		}}},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, cloneTestViewJSON(t, view))
	}))
	defer server.Close()
	var out, errBuf bytes.Buffer
	code := runCloneCommandContext(context.Background(), []string{"status", "job-1", "--watch", "--api", server.URL, "--timeout", "1"}, &out, &errBuf)
	var envelope cloneCLIError
	if err := json.Unmarshal(errBuf.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if code != 1 || out.Len() != 0 || envelope.Code != domain.CloneErrSourceUnavailable || envelope.Recovery == nil || *envelope.Recovery != recovery {
		t.Fatalf("failed wait = %d stdout=%s stderr=%s", code, &out, &errBuf)
	}
	var details struct {
		JobID  string `json:"jobId"`
		Step   string `json:"step"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(envelope.Details, &details); err != nil || details.JobID != "job-1" || details.Step != "download" || details.Status != "failed" {
		t.Fatalf("failure details = %+v, %v", details, err)
	}
}

func TestCloneRunWaitStartsOnceThenPolls(t *testing.T) {
	posts, gets := 0, 0
	client := cloneTestClient(func(r *http.Request) (int, string, error) {
		if r.Method == http.MethodPost {
			posts++
			return 202, `{"id":"job-1","status":"running"}`, nil
		}
		gets++
		return 200, `{"id":"job-1","status":"succeeded"}`, nil
	})
	o, err := parseCloneOptions([]string{"run", "job-1", "--wait"}, cloneTestEnv(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.execute(context.Background(), o); err != nil || posts != 1 || gets != 1 {
		t.Fatalf("run wait: posts=%d gets=%d error=%v", posts, gets, err)
	}
}

func TestCloneWaitHonorsCancellationTimeoutAndReadErrors(t *testing.T) {
	t.Run("cancel while polling", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		client := cloneTestClient(func(*http.Request) (int, string, error) {
			cancel()
			return 200, `{"id":"job-1","status":"running"}`, nil
		})
		_, err := client.wait(ctx, "job-1", time.Second)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("timeout in request", func(t *testing.T) {
		client := cloneTestClient(func(r *http.Request) (int, string, error) {
			<-r.Context().Done()
			return 0, "", r.Context().Err()
		})
		_, err := client.wait(context.Background(), "job-1", time.Millisecond)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error = %v", err)
		}
		var stderr bytes.Buffer
		cloneErrOut(&stderr, err)
		var body cloneCLIError
		if err := json.Unmarshal(stderr.Bytes(), &body); err != nil || body.Code != domain.CloneErrInterrupted {
			t.Fatalf("timeout output = %s, %v", &stderr, err)
		}
	})
	for _, raw := range []string{`{"id":"job-1","status":"unexpected"}`, `{"id":"other","status":"succeeded"}`, `{"status":"succeeded"}`, `null`, `not json`} {
		t.Run(raw, func(t *testing.T) {
			client := cloneTestClient(func(*http.Request) (int, string, error) { return 200, raw, nil })
			if _, err := client.wait(context.Background(), "job-1", time.Second); err == nil {
				t.Fatal("malformed status accepted as completion")
			}
		})
	}
}

func TestCloneHTTPFailuresPreserveServerEnvelopeAndDoNotLeakCookie(t *testing.T) {
	cases := []struct {
		status int
		body   string
		code   string
	}{
		{401, `{"error":"Cần đăng nhập","code":"unauthorized","recovery":"Đăng nhập lại"}`, "unauthorized"},
		{404, `{"error":"Không tìm thấy job","code":"not_found"}`, domain.CloneErrNotFound},
		{409, `{"error":"Job đang chạy","code":"conflict"}`, domain.CloneErrConflict},
		{502, `<html>upstream failed</html>`, domain.CloneErrInternal},
		{200, `invalid json`, domain.CloneErrInternal},
	}
	for _, tc := range cases {
		t.Run(tc.body, func(t *testing.T) {
			client := cloneTestClient(func(*http.Request) (int, string, error) { return tc.status, tc.body, nil })
			client.cookie = "verse_session=never-print-fixture"
			_, err := client.request(context.Background(), http.MethodGet, "/api/clone/job-1", nil)
			var body cloneCLIError
			var stderr bytes.Buffer
			if err == nil {
				t.Fatal("expected HTTP/JSON error")
			}
			cloneErrOut(&stderr, err)
			if err := json.Unmarshal(stderr.Bytes(), &body); err != nil || body.Code != tc.code || strings.Contains(stderr.String(), "never-print-fixture") {
				t.Fatalf("error output = %s, %v", &stderr, err)
			}
		})
	}
	t.Run("network failure", func(t *testing.T) {
		failure := errors.New("connection unavailable")
		client := cloneTestClient(func(*http.Request) (int, string, error) { return 0, "", failure })
		if _, err := client.wait(context.Background(), "job-1", time.Second); !errors.Is(err, failure) {
			t.Fatalf("network error = %v", err)
		}
	})
}

func TestCloneHelpAndInvalidCommandDoNotNeedDaemon(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}, {"help"}} {
		var out, stderr bytes.Buffer
		if code := runCloneCommandContext(context.Background(), args, &out, &stderr); code != 0 || !strings.Contains(out.String(), "clone run") || stderr.Len() != 0 {
			t.Fatalf("help = %d stdout=%s stderr=%s", code, &out, &stderr)
		}
	}
	var out, stderr bytes.Buffer
	if code := runCloneCommandContext(context.Background(), []string{"invalid"}, &out, &stderr); code != 1 || out.Len() != 0 {
		t.Fatalf("invalid = %d stdout=%s stderr=%s", code, &out, &stderr)
	}
	var envelope cloneCLIError
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil || envelope.Code != domain.CloneErrInvalidRequest {
		t.Fatalf("usage error = %s, %v", &stderr, err)
	}
}

type cloneTestTransport func(*http.Request) (*http.Response, error)

func (f cloneTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func cloneTestClient(handler func(*http.Request) (int, string, error)) *cloneHTTPClient {
	return &cloneHTTPClient{
		base: "http://daemon.example.test", pollInterval: time.Millisecond,
		http: &http.Client{Transport: cloneTestTransport(func(r *http.Request) (*http.Response, error) {
			status, body, err := handler(r)
			if err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
		})},
	}
}

func cloneTestViewJSON(t *testing.T, view domain.CloneJobView) string {
	t.Helper()
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestCloneWaitCancelledBeforeRequestNeverCallsDaemon(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	client := cloneTestClient(func(*http.Request) (int, string, error) {
		calls++
		return 200, `{"id":"job-1","status":"succeeded"}`, nil
	})
	if _, err := client.wait(ctx, "job-1", time.Second); !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatalf("cancelled wait: calls=%d error=%v", calls, err)
	}
}

func TestCloneCommandDoesNotFollowLoginRedirect(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Redirect(w, r, "/api/login", http.StatusSeeOther)
	}))
	defer server.Close()
	var out, stderr bytes.Buffer
	code := runCloneCommandContext(context.Background(), []string{"status", "job-1", "--api", server.URL}, &out, &stderr)
	if code != 1 || calls != 1 || out.Len() != 0 || !strings.Contains(stderr.String(), "303") {
		t.Fatalf("redirect: exit=%d calls=%d stdout=%s stderr=%s", code, calls, &out, &stderr)
	}
}

type cloneBrokenBody struct{ err error }

func (b cloneBrokenBody) Read([]byte) (int, error) { return 0, b.err }
func (b cloneBrokenBody) Close() error             { return nil }

func TestCloneHTTPResponseReadFailureIsReturned(t *testing.T) {
	failure := errors.New("response interrupted")
	client := &cloneHTTPClient{
		base: "http://daemon.example.test",
		http: &http.Client{Transport: cloneTestTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: cloneBrokenBody{err: failure}, Request: r}, nil
		})},
	}
	if _, err := client.request(context.Background(), http.MethodGet, "/api/clone/job-1", nil); !errors.Is(err, failure) {
		t.Fatalf("response error = %v", err)
	}
}

type cloneFailedWriter struct{}

func (cloneFailedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestCloneOutputWriteFailureIsNotSuccess(t *testing.T) {
	var stderr bytes.Buffer
	if code := runCloneCommandContext(context.Background(), []string{"help"}, cloneFailedWriter{}, &stderr); code != 1 {
		t.Fatalf("write failure exit = %d", code)
	}
	var envelope cloneCLIError
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil || envelope.Code != domain.CloneErrInternal {
		t.Fatalf("write failure = %s, %v", &stderr, err)
	}
}
