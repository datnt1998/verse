package domain

import (
	"reflect"
	"testing"
)

func TestRequiresChangeFor(t *testing.T) {
	cases := []struct {
		name      string
		severity  string
		requested bool
		want      bool
	}{
		{name: "error luôn bắt sửa", severity: SeverityError, requested: false, want: true},
		{name: "critical luôn bắt sửa", severity: SeverityCritical, requested: false, want: true},
		{name: "warning một mình chỉ là cảnh báo", severity: SeverityWarning, requested: false, want: false},
		{name: "warning Editor tự bật", severity: SeverityWarning, requested: true, want: true},
		{name: "severity lạ giữ nguyên yêu cầu của Editor", severity: "", requested: true, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RequiresChangeFor(tc.severity, tc.requested); got != tc.want {
				t.Fatalf("RequiresChangeFor(%q, %v) = %v, muốn %v", tc.severity, tc.requested, got, tc.want)
			}
		})
	}
}

func TestRestoreOwnPlants(t *testing.T) {
	cases := []struct {
		name string
		prev []ForeshadowUpdate
		next []ForeshadowUpdate
		want []ForeshadowUpdate
	}{
		{
			name: "重写把本章 plant 写成 advance：plant 补回队首",
			prev: []ForeshadowUpdate{{ID: "f_photo", Action: "plant", Description: "泄洪道旧照"}},
			next: []ForeshadowUpdate{{ID: "f_photo", Action: "advance"}},
			want: []ForeshadowUpdate{
				{ID: "f_photo", Action: "plant", Description: "泄洪道旧照"},
				{ID: "f_photo", Action: "advance"},
			},
		},
		{
			name: "重写把本章 plant 整条丢了：仍补回",
			prev: []ForeshadowUpdate{{ID: "f_photo", Action: "plant", Description: "泄洪道旧照"}},
			next: nil,
			want: []ForeshadowUpdate{{ID: "f_photo", Action: "plant", Description: "泄洪道旧照"}},
		},
		{
			name: "新记录已自行声明 plant：不重复补",
			prev: []ForeshadowUpdate{{ID: "f_photo", Action: "plant", Description: "旧描述"}},
			next: []ForeshadowUpdate{{ID: "f_photo", Action: "plant", Description: "新描述"}},
			want: []ForeshadowUpdate{{ID: "f_photo", Action: "plant", Description: "新描述"}},
		},
		{
			name: "旧记录只有 advance/resolve：无 plant 可补",
			prev: []ForeshadowUpdate{{ID: "f_photo", Action: "advance"}, {ID: "f_key", Action: "resolve"}},
			next: []ForeshadowUpdate{{ID: "f_photo", Action: "resolve"}},
			want: []ForeshadowUpdate{{ID: "f_photo", Action: "resolve"}},
		},
		{
			name: "多条 plant 按原序补回，且都排在 advance 之前",
			prev: []ForeshadowUpdate{
				{ID: "f_a", Action: "plant", Description: "甲"},
				{ID: "f_b", Action: "plant", Description: "乙"},
			},
			next: []ForeshadowUpdate{{ID: "f_a", Action: "advance"}},
			want: []ForeshadowUpdate{
				{ID: "f_a", Action: "plant", Description: "甲"},
				{ID: "f_b", Action: "plant", Description: "乙"},
				{ID: "f_a", Action: "advance"},
			},
		},
		{
			name: "首次提交无旧记录：原样返回",
			prev: nil,
			next: []ForeshadowUpdate{{ID: "f_photo", Action: "plant", Description: "泄洪道旧照"}},
			want: []ForeshadowUpdate{{ID: "f_photo", Action: "plant", Description: "泄洪道旧照"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RestoreOwnPlants(tc.prev, tc.next)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("RestoreOwnPlants() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
