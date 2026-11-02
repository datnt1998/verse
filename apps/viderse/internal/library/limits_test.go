package library

import (
	"bytes"
	"context"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/uploadlimit"
)

// Hosted uploads must reject the public ceiling even when the local import
// ceiling permits larger files.
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

	t.Setenv("PUBLIC_URL", "https://vid.example")
	if got := OverlayLimit(); got != uploadlimit.PublicMaxBytes {
		t.Fatalf("OverlayLimit() hosted = %d, muốn %d", got, uploadlimit.PublicMaxBytes)
	}
	_, _, err = svc.AddOverlayReader(ctx, "x.mp4", "", bytes.NewReader(mp4Content()), OverlayLimit()+1)
	if CodeOf(err) != CodeInvalidInput {
		t.Fatalf("hosted quá lớn: %v", err)
	}
}

// TestTemplateBundleLimitIsLoweredWhenHosted pins the same switch for the
// template bundle import ceiling.
func TestTemplateBundleLimitIsLoweredWhenHosted(t *testing.T) {
	t.Setenv("PUBLIC_URL", "")
	if got := TemplateBundleLimit(); got != MaxTemplateBundleBytes {
		t.Fatalf("TemplateBundleLimit() local = %d", got)
	}

	t.Setenv("PUBLIC_URL", "https://vid.example")
	if got := TemplateBundleLimit(); got != uploadlimit.PublicMaxBytes {
		t.Fatalf("TemplateBundleLimit() hosted = %d, muốn %d", got, uploadlimit.PublicMaxBytes)
	}
}
