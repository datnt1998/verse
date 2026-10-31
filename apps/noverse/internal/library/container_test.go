package library

import "testing"

func TestLibraryMountInfoBoundaries(t *testing.T) {
	mounts := "1 0 0:1 / / rw - overlay overlay rw\n2 1 8:1 /data /library rw - ext4 /dev/sda rw\n3 1 8:1 /space /other\\040stories rw - virtiofs share rw\n4 1 0:9 / /volatile rw - tmpfs tmpfs rw\n5 2 0:9 / /library/volatile rw - tmpfs tmpfs rw\n6 5 8:1 /data /library/volatile/durable rw - ext4 /dev/sda rw\n"
	for _, tc := range []struct {
		path    string
		durable bool
	}{
		{"/library", true}, {"/library/sub/new", true}, {"/library-other", false},
		{"/other stories/new", true}, {"/home/noverse/Truyện Noverse", false}, {"/volatile/story", false},
		{"/library/volatile/story", false}, {"/library/volatile/durable/story", true},
	} {
		t.Run(tc.path, func(t *testing.T) {
			if got := libraryPathMounted(tc.path, mounts); got != tc.durable {
				t.Fatalf("mounted(%q)=%v, want %v", tc.path, got, tc.durable)
			}
		})
	}
}
