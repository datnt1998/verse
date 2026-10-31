// Package platforms giữ registry của các nền tảng nguồn và trạng thái bền của
// chúng (chuyển từ backdrop_platform_state của yt-factory): nhịp gọi, giờ chờ
// bị chặn tăng dần 15 → 120 phút theo số lần chặn liên tiếp, và lỗi gần nhất
// cần người xử lý. Thêm một nền tảng = thêm một mục registry.
//
// Phụ thuộc (store, bộ đếm usage, thao tác đăng nhập, đồng hồ, ngủ, hàm phát
// sự kiện) đều tiêm qua constructor — không toàn cục, không init().
package platforms

import "github.com/datnt1998/verse/apps/viderse/internal/domain"

// Descriptor mô tả một nền tảng trong registry (§24): danh tính, việc app
// dùng cho, loại video lấy được, độ cao clip mục tiêu, cách đăng nhập và hạn
// mức cuộn (rỗng = nền tảng không đặt hạn mức, chỉ có khoảng cách tối thiểu).
type Descriptor struct {
	ID           domain.PlatformID
	Label        string
	Uses         []domain.PlatformUse
	Kinds        []domain.PlatformKind
	TargetHeight int
	Login        domain.PlatformLoginType
	Budget       []BudgetWindow
}

// registry là danh sách nền tảng theo thứ tự hiển thị. Bất biến; Registry()
// trả bản sao nên caller không đổi được bản gốc.
var registry = []Descriptor{
	{
		ID:           domain.PlatformYouTube,
		Label:        "YouTube",
		Uses:         []domain.PlatformUse{domain.PlatformUseCompetitor},
		Kinds:        []domain.PlatformKind{domain.PlatformKindVideo},
		TargetHeight: 360,
		Login:        domain.LoginBrowserCookie,
	},
	{
		ID:           domain.PlatformTwitch,
		Label:        "Twitch",
		Uses:         []domain.PlatformUse{domain.PlatformUseBackdrop},
		Kinds:        []domain.PlatformKind{domain.PlatformKindClip, domain.PlatformKindHighlight, domain.PlatformKindVod},
		TargetHeight: 480,
		Login:        domain.LoginNone,
	},
	{
		ID:           domain.PlatformBilibili,
		Label:        "Bilibili",
		Uses:         []domain.PlatformUse{domain.PlatformUseBackdrop},
		Kinds:        []domain.PlatformKind{domain.PlatformKindVideo},
		TargetHeight: 1080,
		Login:        domain.LoginQR,
		// 20 yêu cầu mỗi 10 phút và 200 mỗi 24 giờ: hạn mức an toàn để đọc
		// danh mục mà không bị chặn tài khoản (chuyển từ pacing.ts).
		Budget: BilibiliWindows,
	},
}

// Registry trả bản sao danh sách nền tảng theo thứ tự hiển thị.
func Registry() []Descriptor {
	return append([]Descriptor(nil), registry...)
}

// Lookup trả descriptor của một nền tảng; found=false khi id không thuộc
// registry (caller map sang 404).
func Lookup(id domain.PlatformID) (Descriptor, bool) {
	for _, d := range registry {
		if d.ID == id {
			return d, true
		}
	}
	return Descriptor{}, false
}
