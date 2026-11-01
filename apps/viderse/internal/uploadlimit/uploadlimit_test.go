package uploadlimit

import "testing"

// TestCapKeepsLocalAndLowersHosted pins the switch: only a PUBLIC_URL run has an
// edge in front, so only that run needs the lower ceiling.
func TestCapKeepsLocalAndLowersHosted(t *testing.T) {
	t.Setenv("PUBLIC_URL", "")
	if got := Cap(20 << 30); got != 20<<30 {
		t.Fatalf("Cap(20 GiB) local = %d", got)
	}
	if got := Cap(4 << 30); got != 4<<30 {
		t.Fatalf("Cap(4 GiB) local = %d", got)
	}

	t.Setenv("PUBLIC_URL", "https://vid.example")
	if got := Cap(20 << 30); got != PublicMaxBytes {
		t.Fatalf("Cap(20 GiB) hosted = %d, muốn %d", got, PublicMaxBytes)
	}
	if got := Cap(256 << 20); got != PublicMaxBytes {
		t.Fatalf("Cap(256 MiB) hosted = %d, muốn %d", got, PublicMaxBytes)
	}
	// A ceiling already below the public one is not raised.
	if got := Cap(50 << 20); got != 50<<20 {
		t.Fatalf("Cap(50 MiB) hosted = %d, không được nâng", got)
	}
}

// TestLabelsAndMessage pins the Vietnamese copy the UI shows: it must name the
// ceiling the server enforces, in the same units.
func TestLabelsAndMessage(t *testing.T) {
	for _, tc := range []struct {
		limit int64
		want  string
	}{
		{PublicMaxBytes, "90 MB"},
		{256 << 20, "256 MB"},
		{50 << 20, "50 MB"},
		{4 << 30, "4 GB"},
		{1, "1 MB"},
	} {
		if got := Label(tc.limit); got != tc.want {
			t.Fatalf("Label(%d) = %q, muốn %q", tc.limit, got, tc.want)
		}
	}
	if got := TooLargeMessage(PublicMaxBytes); got != "Tải lên qua internet tối đa 90 MB" {
		t.Fatalf("TooLargeMessage = %q", got)
	}
}
