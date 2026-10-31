package domain_test

import (
	"errors"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

func TestAutonomyNormalizeDerivesPresetFromRows(t *testing.T) {
	for name, want := range map[string]string{domain.PresetCareful: domain.PresetCareful, domain.PresetBalanced: domain.PresetBalanced, domain.PresetAuto: domain.PresetAuto} {
		p, _ := domain.PresetAutonomy(name)
		p.Preset = "client-lies"
		p.Approve.Min = 95 // ngưỡng điểm không đổi mức
		if got := p.Normalize().Preset; got != want {
			t.Fatalf("%s → %s", name, got)
		}
	}
	custom, _ := domain.PresetAutonomy(domain.PresetBalanced)
	custom.Approve.Mode = domain.ApproveAll
	if got := custom.Normalize().Preset; got != domain.PresetCustom {
		t.Fatalf("Tự duyệt hết phải là Tuỳ chỉnh, got %s", got)
	}
}

func TestAutonomyValidateThreshold(t *testing.T) {
	a := domain.DefaultAutonomy()
	for min, ok := range map[int]bool{65: false, 70: true, 85: true, 95: true, 100: false, 83: false} {
		a.Approve.Min = min
		err := a.Validate()
		if (err == nil) != ok {
			t.Fatalf("min=%d err=%v", min, err)
		}
		var fe *domain.AutonomyFieldError
		if err != nil && (!errors.As(err, &fe) || fe.Field != "autonomy.approve.min") {
			t.Fatalf("min=%d field error = %v", min, err)
		}
	}
}

func TestAutonomyRaisesTrustOnlyForAutoPresetAndApproveAll(t *testing.T) {
	balanced := domain.DefaultAutonomy()
	auto, _ := domain.PresetAutonomy(domain.PresetAuto)
	careful, _ := domain.PresetAutonomy(domain.PresetCareful)
	all := balanced
	all.Approve.Mode = domain.ApproveAll
	all = all.Normalize()
	if !balanced.RaisesTrust(auto) || !balanced.RaisesTrust(all) {
		t.Fatal("Tự chạy và Tự duyệt hết phải cần xác nhận")
	}
	if balanced.RaisesTrust(careful) || auto.RaisesTrust(auto) || all.RaisesTrust(all) {
		t.Fatal("hạ mức hoặc giữ nguyên không cần xác nhận")
	}
}

func TestAutoFixCodesAreLimitedToRecoverableErrors(t *testing.T) {
	cases := []struct {
		step domain.CloneStep
		code string
		want string
	}{
		{domain.CloneDownload, domain.CloneErrSourceUnavailable, domain.AutoFixSourceDownload},
		{domain.CloneImages, domain.CloneErrInternal, domain.AutoFixImage},
		{domain.CloneBuild, domain.CloneErrInternal, domain.AutoFixRender},
		{domain.CloneExport, domain.CloneErrInternal, domain.AutoFixRender},
		{domain.CloneDownload, domain.CloneErrToolMissing, ""},
		{domain.CloneImages, domain.CloneErrQuota, ""},
		{domain.CloneBuild, domain.CloneErrTemplateChanged, ""},
		{domain.CloneMetadata, domain.CloneErrInternal, ""},
		{domain.CloneScore, domain.CloneErrQualityGate, domain.AutoFixQuality},
	}
	for _, c := range cases {
		if got := domain.AutoFixCodeOf(c.step, &domain.CloneError{Code: c.code}); got != c.want {
			t.Fatalf("%s/%s = %q, want %q", c.step, c.code, got, c.want)
		}
	}
}

func TestParseAutonomyFallsBackToBalanced(t *testing.T) {
	if got := domain.ParseAutonomy([]byte(`{"pick":"x"}`)); got.Preset != domain.PresetBalanced {
		t.Fatalf("got %+v", got)
	}
}
