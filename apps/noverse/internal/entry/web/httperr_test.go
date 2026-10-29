package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/comments"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

// errBody decodes the standard error envelope.
func errBody(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body không phải JSON: %v (%q)", err, rec.Body.String())
	}
	msg, _ := body["error"].(string)
	return msg
}

// captureLogs redirects the default slog logger to a buffer for one test.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

// TestJSONErrorFiveHundredNeverEchoesCause: a 500 body is one generic sentence
// and the cause (SQL/table/constraint text) exists only in the server log.
func TestJSONErrorFiveHundredNeverEchoesCause(t *testing.T) {
	logs := captureLogs(t)
	rec := httptest.NewRecorder()
	cause := `pq: relation "audio_scripts" does not exist (SQLSTATE 42P01)`

	jsonError(rec, http.StatusInternalServerError, cause)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	msg := errBody(t, rec)
	if msg != errInternalMessage {
		t.Fatalf("body = %q, want generic %q", msg, errInternalMessage)
	}
	for _, leak := range []string{"audio_scripts", "SQLSTATE", "relation"} {
		if strings.Contains(msg, leak) {
			t.Fatalf("body leaks internals %q: %q", leak, msg)
		}
	}
	logged := logs.String()
	if !strings.Contains(logged, `"module":"web"`) || !strings.Contains(logged, "audio_scripts") {
		t.Fatalf("cause not logged server-side with module=web: %s", logged)
	}
}

// TestErrorPolicyClassifiesSharedStorageFamily pins the single transport
// policy for the sentinels both backends return: statuses stay stable and the
// wrapped cause never becomes body text.
func TestErrorPolicyClassifiesSharedStorageFamily(t *testing.T) {
	cause := fmt.Errorf(`truy vấn bảng "stories" thất bại (constraint chapters_pkey): %w`, errs.ErrStorageNotFound)

	cases := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{"invalid", fmt.Errorf("tên dự án: %w", library.ErrInvalidName), 400, ""},
		{"unauthenticated", webauth.ErrUnauthenticated, 401, ""},
		{"invalid credentials", webauth.ErrInvalidCredentials, 401, ""},
		{"forbidden", fmt.Errorf("đọc dự án: %w", errs.ErrStorageForbidden), 403, ""},
		{"not found (repository scope)", cause, 404, "Không tìm thấy."},
		{"not found (member of another tenant)", fmt.Errorf("truyện b: %w", errs.ErrStorageNotFound), 404, ""},
		{"conflict", fmt.Errorf(`ràng buộc duy nhất chapters_pkey: %w`, errs.ErrStorageConflict), 409, ""},
		{"unsupported", fmt.Errorf("audio trên Postgres: %w", errs.ErrStorageUnsupported), 501, ""},
		{"comment not found", fmt.Errorf("góp ý: %w", comments.ErrNotFound), 404, ""},
		{"deadline", fmt.Errorf("gọi DB: %w", context.DeadlineExceeded), 504, ""},
		{"unexpected", errors.New(`dial tcp 10.0.0.5:5432: connect: connection refused`), 500, errInternalMessage},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureLogs(t)
			rec := httptest.NewRecorder()
			jsonMappedError(rec, tc.err)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.status, rec.Body.String())
			}
			msg := errBody(t, rec)
			if tc.body != "" && msg != tc.body {
				t.Fatalf("body = %q, want %q", msg, tc.body)
			}
			for _, leak := range []string{"stories", "chapters_pkey", "constraint", "10.0.0.5"} {
				if strings.Contains(msg, leak) {
					t.Fatalf("body leaks %q: %q", leak, msg)
				}
			}
			if tc.status == http.StatusInternalServerError && !strings.Contains(logs.String(), "module") {
				t.Fatalf("unexpected cause not logged: %s", logs.String())
			}
		})
	}
}

// TestCommentErrorStorageConflictIsRecoverable: the audit's concrete mismatch
// (a storage conflict on comments answered as 500 with the cause) is a 409.
func TestCommentErrorStorageConflictIsRecoverable(t *testing.T) {
	captureLogs(t)
	rec := httptest.NewRecorder()
	commentError(rec, fmt.Errorf("ghi góp ý: %w", errs.ErrStorageConflict))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	if msg := errBody(t, rec); strings.Contains(msg, "ghi góp ý") {
		t.Fatalf("body echoes cause: %q", msg)
	}
}

// TestAudioErrorStorageForbiddenIs403: the audio backend passes
// ErrStorageForbidden through unchanged; it must not become a 500.
func TestAudioErrorStorageForbiddenIs403(t *testing.T) {
	captureLogs(t)
	rec := httptest.NewRecorder()
	audioError(rec, fmt.Errorf("%s: %w", "nghe truyện", errs.ErrStorageForbidden))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (%s)", rec.Code, rec.Body.String())
	}
}

// TestAccountErrorKeepsCuratedValidationText: domain validation detail is
// user-facing (Vietnamese, no internals), so it is preserved on the 400.
func TestAccountErrorKeepsCuratedValidationText(t *testing.T) {
	rec := httptest.NewRecorder()
	accountError(rec, fmt.Errorf("%w: mật khẩu cần ít nhất 8 ký tự", webauth.ErrInvalidInput))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if msg := errBody(t, rec); !strings.Contains(msg, "mật khẩu") {
		t.Fatalf("validation detail lost: %q", msg)
	}
}
