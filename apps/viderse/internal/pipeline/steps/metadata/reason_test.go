package metadata

import "testing"

func TestMetadataReasonRequiresVerifiedSource(t *testing.T) {
	basis := &BasedOn{SourceTitle: "Nguồn thật"}
	got := metadataReasons(basis, "Nguồn thật", "Kênh")
	if len(got) != 3 || got["title"].Agent != "director" || len(got["title"].Facts) != 2 {
		t.Fatalf("reason: %+v", got)
	}
	if metadataReasons(basis, "Nguồn khác", "Kênh") != nil {
		t.Fatal("unverified source accepted")
	}
	if metadataReasons(nil, "Nguồn thật", "Kênh") != nil {
		t.Fatal("missing model basis accepted")
	}
}
