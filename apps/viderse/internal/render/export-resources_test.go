package render

import "testing"

func TestExportProcessRSSIncludesOnlyOwnedChromeTree(t *testing.T) {
	// Descendants can precede parents; another Chrome tree must not inflate
	// this export's RSS. ps reports KiB, while the receipt reports bytes.
	app, chrome := exportProcessRSS("30 20 8\n99 1 1000\n20 10 4\n21 10 2\n10 1 16\n1 0 32\n", 1, 10)
	if app != 32*1024 || chrome != 30*1024 {
		t.Fatalf("app/chrome RSS = %d/%d", app, chrome)
	}
	_, absent := exportProcessRSS("1 0 32\n99 1 1000\n", 1, 10)
	if absent != -1 {
		t.Fatalf("missing Chrome reported as observed RSS %d", absent)
	}
}

func TestExportSwapUsageKeepsUnitsAndUnknownDistinctFromZero(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  int64
		ok    bool
	}{
		{"total = 8192.00M used = 6144.50M free = 2047.50M", 6144*1024*1024 + 512*1024, true},
		{"total = 8.00G used = 6.25G free = 1.75G", 6*1024*1024*1024 + 256*1024*1024, true},
		{"total = 0.00M used = 0.00M free = 0.00M", 0, true},
		{"sysctl unavailable", 0, false},
		{"used = NaNM", 0, false},
	} {
		got, ok := exportSwapUsed(tc.input)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("%q: %d,%v; want %d,%v", tc.input, got, ok, tc.want, tc.ok)
		}
	}
}
