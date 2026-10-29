package auth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func renderCallback(t *testing.T, kind callbackKind, from, raw string) (int, http.Header, string) {
	return renderCallbackBrand(t, kind, from, raw, chatGPTBrand)
}

func renderCallbackBrand(t *testing.T, kind callbackKind, from, raw string, brand callbackBrand) (int, http.Header, string) {
	useTestApp(t)
	t.Helper()
	w := httptest.NewRecorder()
	writeCallbackPage(w, kind, from, raw, brand)
	res := w.Result()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, res.Header, string(body)
}

// TestCallbackPageMarkSize22 守护 Câu 2/23-brand §4: biểu tượng ở thanh trên cùng cao 22 px
// trên máy tính (bỏ cỡ 21 px), bề rộng theo đúng tỉ lệ artboard OAuth 30,2 px.
func TestCallbackPageMarkSize22(t *testing.T) {
	for _, kind := range []callbackKind{callbackSuccess, callbackStateMismatch} {
		_, _, body := renderCallback(t, kind, "web", "")
		for _, want := range []string{
			`width:30.2px;height:22px`,
			`top:4.94px;width:12.23px;height:12.23px`,
			`left:7.89px;top:0.0px;width:14.38px;height:14.38px`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%v: biểu tượng thiếu %q", kind, want)
			}
		}
		if strings.Contains(body, "height:21px") || strings.Contains(body, "width:28.8px") {
			t.Errorf("%v: biểu tượng vẫn ở cỡ 21 px cũ", kind)
		}
	}
}

// Trang success (A4): tiêu đề kết nối, 3 bước, không lộ chi tiết kỹ thuật.
func TestCallbackPageSuccessCLI(t *testing.T) {
	status, header, body := renderCallback(t, callbackSuccess, "cli", "")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	for _, want := range []string{
		"Đã kết nối ChatGPT",
		"Cho phép Viderse sử dụng gói của bạn",
		"Chọn model để bắt đầu viết",
		"Bạn có thể đóng tab này",
		"Viderse không nhìn thấy mật khẩu ChatGPT của bạn",
		// 23-brand §4: logo thanh trên cùng = biểu tượng + tên app 16/700.
		`font-size:16px;font-weight:700;letter-spacing:-0.012em">Viderse</span>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("trang success thiếu %q", want)
		}
	}
	if strings.Contains(body, "Noverse") || strings.Contains(body, "ainovel") {
		t.Error("trang còn chuỗi của app khác")
	}
	if strings.Contains(body, "Chi tiết kỹ thuật") {
		t.Error("trang success không nên có mục Chi tiết kỹ thuật")
	}
	if header.Get("Cache-Control") != "no-store" || header.Get("Referrer-Policy") != "no-referrer" {
		t.Errorf("thiếu header bảo mật: %v", header)
	}
	if !strings.Contains(header.Get("Content-Security-Policy"), "default-src 'none'") {
		t.Errorf("thiếu CSP: %v", header.Get("Content-Security-Policy"))
	}
	// Bản cli không có đoạn JS đóng tab.
	if strings.Contains(body, "window.close") {
		t.Error("bản cli không nên có script đóng tab")
	}
}

// Biến thể web: nút quay lại + auto-đóng + xoá code/state khỏi address bar.
func TestCallbackPageSuccessWeb(t *testing.T) {
	status, _, body := renderCallback(t, callbackSuccess, "web", "")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	for _, want := range []string{"Quay lại Viderse", "tự đóng sau 3 giây", "window.close", "history.replaceState"} {
		if !strings.Contains(body, want) {
			t.Errorf("trang web thiếu %q", want)
		}
	}
	// from=web là tham số server-side — không xuất hiện dạng query trong trang.
	if strings.Contains(body, "from=web") {
		t.Error("from không được lộ ra trang")
	}
}

// denied: lý do huỷ + gợi ý mã thiết bị + Chi tiết kỹ thuật chứa lỗi gốc & lệnh cli.
func TestCallbackPageDenied(t *testing.T) {
	status, _, body := renderCallback(t, callbackDenied, "cli", "máy chủ OAuth từ chối: access_denied")
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
	for _, want := range []string{
		"Chưa kết nối được ChatGPT",
		"Bạn đã huỷ hoặc chưa bấm",
		"Đăng nhập bằng mã",
		"Chi tiết kỹ thuật",
		"access_denied",
		"viderse login --device",
		"Thử lại trong cửa sổ Viderse",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("trang denied thiếu %q", want)
		}
	}
}

// stateMismatch: lý do "liên kết đã cũ".
func TestCallbackPageStateMismatch(t *testing.T) {
	_, _, body := renderCallback(t, callbackStateMismatch, "web", "OAuth state không khớp — huỷ để tránh giả mạo")
	for _, want := range []string{"Liên kết đăng nhập này đã cũ", "Đóng tab này"} {
		if !strings.Contains(body, want) {
			t.Errorf("trang stateMismatch thiếu %q", want)
		}
	}
}

// missingCode: lý do "thiếu thông tin" + gợi ý đăng nhập bằng mã.
func TestCallbackPageMissingCode(t *testing.T) {
	_, _, body := renderCallback(t, callbackMissingCode, "cli", "callback thiếu mã xác thực")
	for _, want := range []string{"Thông tin đăng nhập chưa đầy đủ", "Đăng nhập bằng mã"} {
		if !strings.Contains(body, want) {
			t.Errorf("trang missingCode thiếu %q", want)
		}
	}
}

// Lỗi gốc phải được escape (không chèn HTML).
func TestCallbackPageEscapesRawError(t *testing.T) {
	_, _, body := renderCallback(t, callbackDenied, "cli", `<script>alert(1)</script>`)
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("lỗi gốc phải được escape trong Chi tiết kỹ thuật")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Error("lỗi gốc escape kỳ vọng hiện trong Chi tiết kỹ thuật")
	}
}

var callbackFailKinds = []callbackKind{callbackDenied, callbackStateMismatch, callbackMissingCode}

// Chi tiết kỹ thuật (OAuth.dc.html:135-141): nút aria-expanded="false" ban đầu, khối
// lệnh/lỗi ẩn, hai icon arrow-right-2 (đóng) / arrow-down-2 (mở) 14px và JS đảo trạng thái.
func TestCallbackTechToggle(t *testing.T) {
	for _, kind := range callbackFailKinds {
		for _, from := range []string{"cli", "web"} {
			_, _, body := renderCallback(t, kind, from, "raw-error")
			name := string(kind) + "/" + from
			for _, want := range []string{
				`<button id="tech-toggle" type="button" aria-expanded="false"`,
				`id="tech-body" class="mono" hidden`,
				`id="tech-ic-closed">` + string(callbackIcon("arrow-right-2", 14)),
				`id="tech-ic-open" hidden>` + string(callbackIcon("arrow-down-2", 14)),
				`setAttribute('aria-expanded', open ? 'true' : 'false')`,
				`getElementById('tech-body').hidden = !open`,
				`getElementById('tech-ic-closed').hidden = open`,
				`getElementById('tech-ic-open').hidden = !open`,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("%s: thiếu %q", name, want)
				}
			}
			if strings.Contains(body, "<details") || strings.Contains(body, "<summary") {
				t.Errorf("%s: không dùng <details>/<summary> — artboard là button aria-expanded", name)
			}
		}
	}
	_, _, ok := renderCallback(t, callbackSuccess, "cli", "")
	if strings.Contains(ok, "tech-toggle") {
		t.Error("success không có nút Chi tiết kỹ thuật")
	}
}

// Dòng kỹ thuật (OAuth.dc.html:175): cli có 2 dòng lệnh, web chỉ có dòng lỗi.
func TestCallbackTechLines(t *testing.T) {
	for _, kind := range callbackFailKinds {
		_, _, cli := renderCallback(t, kind, "cli", "boom")
		for _, want := range []string{
			`<div style="color:#FFC9CE">Lỗi: boom</div>`,
			`<div style="color:#C9A6F5">$ viderse login</div>`,
			`<div style="color:#C9A6F5">$ viderse login --device   # đăng nhập bằng mã</div>`,
		} {
			if !strings.Contains(cli, want) {
				t.Errorf("%s/cli: thiếu %q", kind, want)
			}
		}
		_, _, web := renderCallback(t, kind, "web", "boom")
		if !strings.Contains(web, `Lỗi: boom</div>`) {
			t.Errorf("%s/web: thiếu dòng Lỗi", kind)
		}
		if strings.Contains(web, "viderse login") || strings.Contains(web, "--device") {
			t.Errorf("%s/web: không được lộ lệnh login (chỉ from=cli)", kind)
		}
	}
	_, _, claude := renderCallbackBrand(t, callbackDenied, "cli", "boom", claudeBrand)
	if !strings.Contains(claude, `$ viderse login anthropic-oauth --device   # đăng nhập bằng mã</div>`) {
		t.Error("Claude/cli: thiếu dòng --device")
	}
}

// Mỗi icon trên trang phải là đúng file icon + đúng cỡ px của artboard
// (OAuth.dc.html: style width/height trên từng span.ic; mặc định .ic = 18px).
func TestCallbackIconsPerState(t *testing.T) {
	icon := func(file string, size int) string { return string(callbackIcon(file, size)) }
	type tc struct {
		kind callbackKind
		from string
		want []string
		not  []string
	}
	common := []string{icon("login", 14), icon("shield-security", 14)}
	cases := []tc{
		{callbackSuccess, "web", append([]string{icon("tick-circle-bulk", 18), icon("arrow-right-2", 18), icon("coin-1", 16), icon("refresh-2", 16), icon("setting-2", 16), icon("arrow-left-2", 17)}, common...), []string{icon("mobile", 16), icon("refresh-2", 17)}},
		{callbackSuccess, "cli", append([]string{icon("tick-circle-bulk", 18), icon("arrow-right-2", 18), icon("coin-1", 16), icon("refresh-2", 16), icon("setting-2", 16), icon("arrow-left-2", 18)}, common...), []string{icon("arrow-left-2", 17)}},
	}
	for _, kind := range callbackFailKinds {
		cases = append(cases,
			tc{kind, "web", append([]string{icon("mobile", 16), icon("refresh-2", 17), icon("arrow-right-2", 14), icon("arrow-down-2", 14)}, common...), []string{icon("tick-circle-bulk", 18), icon("coin-1", 16), icon("refresh-2", 18)}},
			tc{kind, "cli", append([]string{icon("mobile", 16), icon("refresh-2", 18), icon("arrow-right-2", 14), icon("arrow-down-2", 14)}, common...), []string{icon("tick-circle-bulk", 18), icon("coin-1", 16), icon("refresh-2", 17)}},
		)
	}
	for _, c := range cases {
		_, _, body := renderCallback(t, c.kind, c.from, "raw")
		for _, want := range c.want {
			if !strings.Contains(body, want) {
				t.Errorf("%s/%s: thiếu icon %.90q…", c.kind, c.from, want)
			}
		}
		for _, no := range c.not {
			if strings.Contains(body, no) {
				t.Errorf("%s/%s: không được có icon %.90q…", c.kind, c.from, no)
			}
		}
	}
}
