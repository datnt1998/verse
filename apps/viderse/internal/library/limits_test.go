package library

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/uploadlimit"
)

// TestOverlayLimitIsLoweredWhenHosted pins the ceiling switch and the copy that
// goes with it: over the internet the refusal names the limit that actually
// applies, on the desktop it stays the library's own 256 MiB.
func TestOverlayLimitIsLoweredWhenHosted(t *testing.T) {
	ctx := context.Background()
	svc := testService(t, newMemStore())
	t.Setenv("PUBLIC_URL", "")
	if got := OverlayLimit(); got != MaxOverlayBytes {
		t.Fatalf("OverlayLimit() local = %d", got)
	}
	_, _, err := svc.AddOverlayReader(ctx, "x.mp4", "", bytes.NewReader(mp4Content()), MaxOverlayBytes+1)
	if CodeOf(err) != CodeInvalidInput {
		t.Fatalf("local quá lớn: %v", err)
	}
	var apiErr *Error
	if !asLibraryError(err, &apiErr) || !strings.Contains(apiErr.Recovery, "Chọn tệp nhỏ hơn 256 MB") {
		t.Fatalf("recovery local = %+v", apiErr)
	}

	t.Setenv("PUBLIC_URL", "https://vid.example")
	if got := OverlayLimit(); got != uploadlimit.PublicMaxBytes {
		t.Fatalf("OverlayLimit() hosted = %d, muốn %d", got, uploadlimit.PublicMaxBytes)
	}
	_, _, err = svc.AddOverlayReader(ctx, "x.mp4", "", bytes.NewReader(mp4Content()), OverlayLimit()+1)
	if CodeOf(err) != CodeInvalidInput {
		t.Fatalf("hosted quá lớn: %v", err)
	}
	if !asLibraryError(err, &apiErr) || !strings.Contains(apiErr.Recovery, "Tải lên qua internet tối đa 90 MB") {
		t.Fatalf("recovery hosted = %+v", apiErr)
	}
}

// TestTemplateBundleLimitIsLoweredWhenHosted pins the same switch for the
// template bundle import ceiling.
func TestTemplateBundleLimitIsLoweredWhenHosted(t *testing.T) {
	t.Setenv("PUBLIC_URL", "")
	if got := TemplateBundleLimit(); got != MaxTemplateBundleBytes {
		t.Fatalf("TemplateBundleLimit() local = %d", got)
	}
	if message, recovery := TemplateBundleTooLarge(); message != "Tệp template quá lớn" || !strings.Contains(recovery, "4 GB") {
		t.Fatalf("local: %q %q", message, recovery)
	}

	t.Setenv("PUBLIC_URL", "https://vid.example")
	if got := TemplateBundleLimit(); got != uploadlimit.PublicMaxBytes {
		t.Fatalf("TemplateBundleLimit() hosted = %d, muốn %d", got, uploadlimit.PublicMaxBytes)
	}
	if message, recovery := TemplateBundleTooLarge(); message != "Tệp template quá lớn" || !strings.Contains(recovery, "Tải lên qua internet tối đa 90 MB") {
		t.Fatalf("hosted: %q %q", message, recovery)
	}
}

// asLibraryError is errors.As restricted to *Error without importing errors in
// every test that needs it.
func asLibraryError(err error, target **Error) bool {
	e, ok := err.(*Error)
	if ok {
		*target = e
	}
	return ok
}
