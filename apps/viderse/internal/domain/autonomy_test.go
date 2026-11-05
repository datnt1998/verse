package domain_test

import (
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

func TestAutoFixCodesAreLimitedToRecoverableErrors(t *testing.T) {
	cases := []struct {
		step domain.CloneStep
		code string
		want string
	}{
		{domain.CloneDownload, domain.CloneErrSourceUnavailable, domain.AutoFixSourceDownload},
		{domain.CloneImages, domain.CloneErrInternal, domain.AutoFixImage},
		{domain.CloneBuild, domain.CloneErrInternal, domain.AutoFixRender},
		{domain.CloneScore, domain.CloneErrInternal, domain.AutoFixRender},
		{domain.CloneExport, domain.CloneErrInternal, domain.AutoFixRender},
		{domain.CloneDownload, domain.CloneErrToolMissing, ""},
		{domain.CloneImages, domain.CloneErrQuota, ""},
		{domain.CloneBuild, domain.CloneErrTemplateChanged, ""},
		{domain.CloneMetadata, domain.CloneErrInternal, ""},
		{domain.CloneScore, domain.CloneErrQualityGate, ""},
		{domain.CloneDownload, domain.CloneErrStale, ""},
		{domain.CloneImages, domain.CloneErrInvalidRequest, ""},
	}
	for _, c := range cases {
		if got := domain.AutoFixCodeOf(c.step, &domain.CloneError{Code: c.code}); got != c.want {
			t.Fatalf("%s/%s = %q, want %q", c.step, c.code, got, c.want)
		}
	}
	if got := domain.AutoFixCodeOf(domain.CloneDownload, nil); got != "" {
		t.Fatalf("nil error classified as %q", got)
	}
}
