package domain

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestStoryRetentionAudienceArray(t *testing.T) {
	for _, raw := range []string{
		`{"nguoi_nghe":[2,1],"diem_khac":"nghề mới"}`,
		`{"nguoi_nghe":"Nam trung niên · Nữ 18–35","diem_khac":"nghề mới"}`,
	} {
		var fields StoryRetention
		if err := json.Unmarshal([]byte(raw), &fields); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(fields.NguoiNghe, []int{2, 1}) {
			t.Fatalf("audience = %v, want [2 1]", fields.NguoiNghe)
		}
		// Partial updates retain the chosen variant(s), including their order.
		if err := json.Unmarshal([]byte(`{"hat_giong_phan_hai":"Ai gửi thư?"}`), &fields); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(encoded), `"nguoi_nghe":[2,1]`) {
			t.Fatalf("stored audience must be an array: %s", encoded)
		}
	}
}
