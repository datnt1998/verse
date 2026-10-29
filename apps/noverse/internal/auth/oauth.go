package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// pkce sinh cặp code_verifier/code_challenge (S256) từ 64 byte ngẫu nhiên.
func pkce() (verifier, challenge string, err error) { return pkceBytes(64) }

// pkceBytes sinh cặp code_verifier/code_challenge (S256) từ n byte ngẫu nhiên
// (verifier base64url dài 4n/3 ký tự; RFC 7636 cho phép 43–128).
func pkceBytes(n int) (verifier, challenge string, err error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("sinh PKCE verifier: %w", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func randomState() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("sinh OAuth state: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// callbackServer là HTTP server cục bộ nhận redirect ?code=&state= của OAuth.
type callbackServer struct {
	srv    *http.Server
	result chan callbackResult
}

type callbackResult struct {
	code string
	err  error
}

// startCallbackServer lắng nghe addr (vd. "127.0.0.1:1455") tại path. Cổng cố
// định vì nhà cung cấp chỉ allowlist đúng redirect URI — cổng bận thì báo lỗi
// thay vì lặng lẽ đổi cổng.
func startCallbackServer(addr, path, state, from string, brand callbackBrand) (*callbackServer, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("không mở được cổng callback %s (đang bị chiếm? thử `--device`): %w", addr, err)
	}
	cs := &callbackServer{result: make(chan callbackResult, 1)}
	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		var res callbackResult
		kind := callbackSuccess
		raw := ""
		switch {
		case q.Get("error") != "":
			res.err = fmt.Errorf("máy chủ OAuth từ chối: %s %s", q.Get("error"), q.Get("error_description"))
			kind = callbackDenied
			raw = res.err.Error()
		case q.Get("state") != state:
			res.err = errors.New("OAuth state không khớp — huỷ để tránh giả mạo")
			kind = callbackStateMismatch
			raw = res.err.Error()
		case q.Get("code") == "":
			res.err = errors.New("callback thiếu mã xác thực")
			kind = callbackMissingCode
			raw = res.err.Error()
		default:
			res.code = q.Get("code")
		}
		writeCallbackPage(w, kind, from, raw, brand)
		select {
		case cs.result <- res:
		default:
		}
	})
	cs.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go cs.srv.Serve(ln)
	return cs, nil
}

// wait chờ mã xác thực hoặc ctx bị huỷ.
func (cs *callbackServer) wait(ctx context.Context) (string, error) {
	select {
	case res := <-cs.result:
		return res.code, res.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (cs *callbackServer) close() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = cs.srv.Shutdown(ctx)
}

// ---------- Trang callback (thiết kế A4: docs/design/v1/artboards/OAuth.dc.html) ----------

// callbackKind phân loại trạng thái trang callback.
type callbackKind string

const (
	callbackSuccess       callbackKind = "success"
	callbackDenied        callbackKind = "denied"
	callbackStateMismatch callbackKind = "stateMismatch"
	callbackMissingCode   callbackKind = "missingCode"
)

type callbackStep struct {
	Label string
	Note  string
	Done  bool
	Next  bool
}

type callbackPerk struct {
	Title string
	Desc  string
	Icon  template.HTML
}

type callbackPageData struct {
	OK      bool
	FromWeb bool
	Steps   []callbackStep
	Perks   []callbackPerk
	// Thương hiệu nhà cung cấp (ChatGPT / Claude) để dùng chung một trang.
	Service  string
	Plan     string
	LoginCmd string
	// Lỗi
	ReasonTitle string
	ReasonText  string
	Fixes       []string
	Raw         string
}

// callbackBrand là phần chữ riêng của từng nhà cung cấp trên trang callback.
type callbackBrand struct {
	Service  string // "ChatGPT" / "Claude"
	Plan     string // "ChatGPT Plus/Pro" / "Claude Pro/Max"
	Tier     string // "Plus hoặc Pro" / "Pro hoặc Max"
	LoginCmd string // lệnh đăng nhập CLI hiển thị ở "Chi tiết kỹ thuật" (chỉ bản from=cli)
}

var chatGPTBrand = callbackBrand{Service: "ChatGPT", Plan: "ChatGPT Plus/Pro", Tier: "Plus hoặc Pro", LoginCmd: "ainovel-cli login"}

var claudeBrand = callbackBrand{Service: "Claude", Plan: "Claude Pro/Max", Tier: "Pro hoặc Max", LoginCmd: "ainovel-cli login anthropic-oauth"}

var callbackPageTemplate = template.Must(template.New("oauth-callback").Funcs(template.FuncMap{
	"add":  func(a, b int) int { return a + b },
	"icon": callbackIcon,
}).Parse(callbackPageHTML))

// writeCallbackPage render trang kết quả OAuth (A4). `from` phân biệt nguồn khởi động
// "cli"|"web" — được server truyền vào, KHÔNG lộ trên URL (hợp đồng §6).
func writeCallbackPage(w http.ResponseWriter, kind callbackKind, from, raw string, brand callbackBrand) {
	web := from == "web"
	data := callbackPageData{OK: kind == callbackSuccess, FromWeb: web, Raw: raw, Service: brand.Service, Plan: brand.Plan, LoginCmd: brand.LoginCmd}
	if data.OK {
		note := "trong cửa sổ Noverse"
		if web {
			note = "trong Noverse"
		}
		data.Steps = []callbackStep{
			{Label: "Đăng nhập " + brand.Service, Note: "xong", Done: true},
			{Label: "Cho phép Noverse sử dụng gói của bạn", Note: "xong", Done: true},
			{Label: "Chọn model để bắt đầu viết", Note: note, Next: true},
		}
		data.Perks = []callbackPerk{
			{Title: "Không cần mua API riêng, không tốn thêm phí", Desc: "Các trợ lý AI dùng hạn mức của gói " + brand.Service + " bạn đang có.", Icon: callbackIcon("coin-1", 16)},
			{Title: "Không phải đăng nhập lại mỗi lần", Desc: "Noverse tự giữ kết nối luôn còn hiệu lực.", Icon: callbackIcon("refresh-2", 16)},
			{Title: "Ngắt kết nối bất cứ lúc nào", Desc: "Trong Cài đặt › Nhà cung cấp AI.", Icon: callbackIcon("setting-2", 16)},
		}
	} else {
		again := "Quay lại cửa sổ Noverse và chạy lại bước đăng nhập."
		if web {
			again = "Quay lại Noverse và bấm “Đăng nhập " + brand.Service + "” lần nữa."
		}
		switch kind {
		case callbackDenied:
			data.ReasonTitle = "Bạn đã huỷ hoặc chưa bấm “Cho phép”"
			data.ReasonText = "Noverse cần bạn đồng ý trên trang " + brand.Service + " mới dùng được gói của bạn. Chưa có gì bị thay đổi."
			data.Fixes = []string{again, "Kiểm tra tài khoản đang đăng nhập có gói " + brand.Tier + "."}
		case callbackStateMismatch:
			data.ReasonTitle = "Liên kết đăng nhập này đã cũ"
			data.ReasonText = "Trang này đến từ một lần đăng nhập trước đó, nên Noverse đã chặn lại để bảo vệ tài khoản của bạn."
			data.Fixes = []string{"Đóng tab này.", again}
		default: // callbackMissingCode
			data.ReasonTitle = "Thông tin đăng nhập chưa đầy đủ"
			data.ReasonText = "Trang " + brand.Service + " trả về thiếu thông tin cần thiết, thường do mạng chập chờn."
			data.Fixes = []string{again, "Nếu vẫn không được, chọn “Đăng nhập bằng mã”."}
		}
	}

	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	// URL chứa code/state — không cache, không referrer, CSP khóa ngoại tải.
	h.Set("Cache-Control", "no-store")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'")
	if !data.OK {
		w.WriteHeader(http.StatusBadRequest)
	}
	_ = callbackPageTemplate.Execute(w, data)
}

// callbackPageHTML là bản port 1:1 của OAuth.dc.html: CSS/JS tự chứa, font hệ thống
// (fallback Be Vietnam Pro), không tải tài nguyên ngoài. Biến thể web có nút quay lại
// + tự đóng tab sau 3s và history.replaceState để xoá code/state khỏi thanh địa chỉ.
const callbackPageHTML = `<!doctype html>
<html lang="vi">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Kết nối {{.Service}}</title>
<style>
body{margin:0;background:#0B0B0D}
*{box-sizing:border-box}
button{font:inherit;cursor:pointer}
.ic{display:inline-flex;flex:0 0 auto}
.ic[hidden]{display:none}
:focus-visible{outline:2px solid #C9A6F5;outline-offset:2px}
.card{background:linear-gradient(180deg,#171719 0%,#131315 100%);border:1px solid rgba(255,255,255,.07);box-shadow:inset 0 1px 0 rgba(255,255,255,.045),0 1px 2px rgba(0,0,0,.5)}
.primary:hover{background:#FFFFFF !important}
.mono{font-family:'JetBrains Mono',ui-monospace,SFMono-Regular,Menlo,monospace}
.lbl{font-size:11px;font-weight:600;letter-spacing:.08em;color:#7E7E8A;text-transform:uppercase}
.pop{animation:pop .5s cubic-bezier(.2,.9,.3,1.3) both}
@keyframes pop{from{transform:scale(.6);opacity:0}to{transform:none;opacity:1}}
.ring{animation:ring 1.6s ease-out .2s both}
@keyframes ring{from{transform:scale(.8);opacity:.7}to{transform:scale(1.9);opacity:0}}
.draw{stroke-dasharray:30;stroke-dashoffset:30;animation:draw .45s ease-out .25s forwards}
@keyframes draw{to{stroke-dashoffset:0}}
.rise{animation:rise .5s ease-out both}
@keyframes rise{from{transform:translateY(8px);opacity:0}to{transform:none;opacity:1}}
@media (prefers-reduced-motion:reduce){*{animation:none !important}.draw{stroke-dashoffset:0}}
</style>
</head>
<body>
<div style="min-height:100vh;display:flex;flex-direction:column;background:#0B0B0D;color:#EDEDEF;font-family:'Be Vietnam Pro',ui-sans-serif,system-ui,-apple-system,'Segoe UI',sans-serif;font-size:14px;line-height:1.55;-webkit-font-smoothing:antialiased">
<main style="flex:1;display:flex;align-items:center;justify-content:center;padding:40px 16px">
<div style="width:min(500px,100%);display:flex;flex-direction:column;align-items:center;gap:20px">

<div class="rise" style="display:flex;align-items:center;gap:8px">
<div aria-hidden="true" style="position:relative;flex:0 0 auto;width:30.2px;height:22px"><span style="position:absolute;left:0.05px;top:4.94px;width:12.23px;height:12.23px;border-radius:50%;background:radial-gradient(circle at 34% 28%,#A9C2FA 0%,#7896F2 55%,#5360DC 100%)"></span><span style="position:absolute;left:7.89px;top:0.0px;width:14.38px;height:14.38px;border-radius:50%;background:radial-gradient(circle at 34% 28%,#CDA4F7 0%,#A472EB 55%,#7C46D4 100%)"></span><span style="position:absolute;left:17.87px;top:5.47px;width:12.23px;height:12.23px;border-radius:50%;background:radial-gradient(circle at 34% 28%,#FBC48A 0%,#F29A5A 55%,#D9663A 100%)"></span><span style="position:absolute;left:9.6px;top:11.05px;width:10.95px;height:10.95px;border-radius:50%;background:radial-gradient(circle at 34% 28%,#A0EDCF 0%,#5FCFB0 55%,#36A29C 100%)"></span></div>
<span style="font-family:'Plus Jakarta Sans','Be Vietnam Pro',sans-serif;font-size:16px;font-weight:700;letter-spacing:-0.012em">Noverse</span>
</div>

<div class="card rise" style="width:100%;border-radius:20px;padding:28px 24px 22px;display:flex;flex-direction:column;gap:18px;animation-delay:.05s">

<div style="display:flex;flex-direction:column;align-items:center;gap:14px;text-align:center">
<div aria-hidden="true" style="position:relative;width:72px;height:72px">
<span class="ring" style="position:absolute;inset:0;border-radius:50%;border:2px solid {{if .OK}}rgba(166,237,193,0.6){{else}}rgba(246,203,140,0.55){{end}}"></span>
<span class="pop" style="position:absolute;inset:0;border-radius:50%;background:{{if .OK}}rgba(166,237,193,0.1){{else}}rgba(246,203,140,0.1){{end}};box-shadow:0 0 0 1px {{if .OK}}rgba(166,237,193,0.3){{else}}rgba(246,203,140,0.3){{end}},0 0 40px -6px {{if .OK}}rgba(166,237,193,0.35){{else}}rgba(246,203,140,0.28){{end}};display:flex;align-items:center;justify-content:center">
{{if .OK}}<svg width="34" height="34" viewBox="0 0 24 24" fill="none"><path class="draw" d="M6 12.5l4 4 8-9" stroke="#A6EDC1" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/></svg>{{else}}<svg width="30" height="30" viewBox="0 0 24 24" fill="none"><path class="draw" d="M12 7v6M12 16.5v.5" stroke="#F6CB8C" stroke-width="2.4" stroke-linecap="round"/></svg>{{end}}
</span>
</div>
<span style="height:26px;padding:0 10px;display:inline-flex;align-items:center;gap:6px;border-radius:999px;background:#1E1E21;border:1px solid rgba(255,255,255,0.07);font-size:12px;color:#C9C9D1"><span class="ic" style="color:#C9A6F5">{{icon "login" 14}}</span>Tài khoản {{.Plan}}</span>
<div role="{{if .OK}}status{{else}}alert{{end}}">
<h1 style="margin:0;font-size:24px;font-weight:600;letter-spacing:-0.015em">{{if .OK}}Đã kết nối {{.Service}}{{else}}Chưa kết nối được {{.Service}}{{end}}</h1>
<p style="margin:8px 0 0;font-size:14.5px;color:#A0A0AB">{{if .OK}}Noverse đã được phép dùng gói {{.Service}} của bạn để viết truyện.{{else}}Đừng lo, tài khoản của bạn vẫn an toàn. Làm theo các bước dưới đây để thử lại.{{end}}</p>
</div>
</div>

{{if .OK}}
<ol style="list-style:none;margin:0;padding:4px 14px;border-radius:14px;background:#0F0F11;border:1px solid rgba(255,255,255,0.06)">
{{range $i, $s := .Steps}}
<li style="display:flex;align-items:center;gap:12px;padding:10px 0;{{if gt $i 0}}border-top:1px solid rgba(255,255,255,0.06){{end}}">
{{if $s.Done}}<span class="ic" style="color:#A6EDC1">{{icon "tick-circle-bulk" 18}}</span>{{else if $s.Next}}<span class="ic" style="color:#C9A6F5">{{icon "arrow-right-2" 18}}</span>{{end}}
<span style="flex:1;font-size:13.5px;color:{{if $s.Next}}#EDEDEF{{else}}#A0A0AB{{end}}">{{$s.Label}}</span>
<span style="font-size:12px;color:#7E7E8A">{{$s.Note}}</span>
</li>
{{end}}
</ol>

<div style="display:flex;flex-direction:column;gap:10px">
<span class="lbl">Điều này có nghĩa là</span>
{{range .Perks}}
<div style="display:flex;gap:12px;align-items:flex-start">
<span aria-hidden="true" style="width:30px;height:30px;flex:0 0 auto;border-radius:9px;background:#1E1E21;display:flex;align-items:center;justify-content:center;color:#C9C9D1"><span class="ic">{{.Icon}}</span></span>
<span style="display:flex;flex-direction:column"><span style="font-size:13.5px;font-weight:500">{{.Title}}</span><span style="font-size:12.5px;color:#A0A0AB">{{.Desc}}</span></span>
</div>
{{end}}
</div>
{{else}}
<div style="padding:14px;border-radius:14px;background:rgba(246,203,140,0.05);border:1px solid rgba(246,203,140,0.22);display:flex;flex-direction:column;gap:4px">
<span style="font-size:14px;font-weight:600">{{.ReasonTitle}}</span>
<span style="font-size:13px;color:#C9C9D1">{{.ReasonText}}</span>
</div>
<div style="display:flex;flex-direction:column;gap:10px">
<span class="lbl">Cách khắc phục</span>
<ol style="list-style:none;margin:0;padding:0;display:flex;flex-direction:column;gap:10px">
{{range $i, $f := .Fixes}}
<li style="display:flex;gap:12px;align-items:flex-start"><span style="width:22px;height:22px;flex:0 0 auto;border-radius:50%;background:#222225;display:flex;align-items:center;justify-content:center;font-size:11px;font-weight:600;color:#C9C9D1">{{add $i 1}}</span><span style="font-size:13.5px;color:#C9C9D1;padding-top:1px">{{$f}}</span></li>
{{end}}
</ol>
</div>
<div style="display:flex;gap:10px;padding:10px 12px;border-radius:12px;border:1px solid rgba(255,255,255,0.07);font-size:12.5px;color:#A0A0AB">
<span class="ic" style="margin-top:1px">{{icon "mobile" 16}}</span>
<span>Đang dùng Noverse trên máy khác hoặc điện thoại? Chọn <span style="color:#EDEDEF">“Đăng nhập bằng mã”</span> — chỉ cần nhập một mã ngắn, không phải mở trang trên máy này.</span>
</div>
{{end}}

{{if .FromWeb}}
<div style="display:flex;flex-direction:column;gap:8px">
<button class="primary" type="button" onclick="window.close()" style="height:44px;border-radius:12px;border:none;background:#EDEDEF;color:#0B0B0D;font-size:14.5px;font-weight:600;display:flex;align-items:center;justify-content:center;gap:8px"><span class="ic">{{if .OK}}{{icon "arrow-left-2" 17}}{{else}}{{icon "refresh-2" 17}}{{end}}</span>{{if .OK}}Quay lại Noverse{{else}}Quay lại Noverse để thử lại{{end}}</button>
<span role="status" style="text-align:center;font-size:12.5px;color:#7E7E8A">{{if .OK}}Tab này sẽ tự đóng sau 3 giây{{else}}Có thể đóng tab này{{end}}</span>
</div>
{{else}}
<div style="display:flex;align-items:center;gap:12px;padding:12px 14px;border-radius:14px;background:#1E1E21">
<span class="ic" style="color:#C9A6F5">{{if .OK}}{{icon "arrow-left-2" 18}}{{else}}{{icon "refresh-2" 18}}{{end}}</span>
<span style="flex:1;display:flex;flex-direction:column"><span style="font-size:13.5px;font-weight:500">{{if .OK}}Bạn có thể đóng tab này{{else}}Thử lại trong cửa sổ Noverse{{end}}</span><span style="font-size:12.5px;color:#A0A0AB">{{if .OK}}Quay lại cửa sổ Noverse — danh sách model bạn được dùng sẽ hiện ra ở đó.{{else}}Lệnh cần chạy nằm trong “Chi tiết kỹ thuật” bên dưới.{{end}}</span></span>
</div>
{{end}}

{{if not .OK}}
<div style="border-top:1px solid rgba(255,255,255,0.06);padding-top:12px;display:flex;flex-direction:column;gap:8px">
<button id="tech-toggle" type="button" aria-expanded="false" style="align-self:flex-start;display:flex;align-items:center;gap:6px;padding:0;border:none;background:transparent;color:#7E7E8A;font-size:12.5px"><span class="ic" id="tech-ic-closed">{{icon "arrow-right-2" 14}}</span><span class="ic" id="tech-ic-open" hidden>{{icon "arrow-down-2" 14}}</span>Chi tiết kỹ thuật</button>
<div id="tech-body" class="mono" hidden style="padding:10px 12px;border-radius:10px;background:#0B0B0D;border:1px solid rgba(255,255,255,0.07);font-size:11.5px;line-height:1.8;color:#A0A0AB;word-break:break-word">
<div style="color:#FFC9CE">Lỗi: {{.Raw}}</div>
{{if not .FromWeb}}<div style="color:#C9A6F5">$ {{.LoginCmd}}</div>
<div style="color:#C9A6F5">$ {{.LoginCmd}} --device   # đăng nhập bằng mã</div>{{end}}
</div>
</div>
{{end}}

</div>

<div class="rise" style="display:flex;align-items:flex-start;gap:8px;max-width:460px;font-size:12px;color:#7E7E8A;animation-delay:.1s">
<span class="ic" style="margin-top:2px">{{icon "shield-security" 14}}</span>
<span>Trang này do Noverse mở trên chính máy tính của bạn và sẽ tự tắt sau khi xong. Noverse không nhìn thấy mật khẩu {{.Service}} của bạn.</span>
</div>

</div>
</main>
</div>
{{if not .OK}}<script>
(function () {
  var b = document.getElementById('tech-toggle');
  b.addEventListener('click', function () {
    var open = b.getAttribute('aria-expanded') !== 'true';
    b.setAttribute('aria-expanded', open ? 'true' : 'false');
    document.getElementById('tech-body').hidden = !open;
    document.getElementById('tech-ic-closed').hidden = open;
    document.getElementById('tech-ic-open').hidden = !open;
  });
})();
</script>{{end}}
{{if .FromWeb}}<script>
// Xoá code/state khỏi thanh địa chỉ; tab tự đóng chỉ khi do app mở bằng window.open.
try { history.replaceState(null, '', location.pathname); } catch (e) {}
{{if .OK}}setTimeout(function(){ window.close(); }, 3000);{{end}}
</script>{{end}}
</body>
</html>`

// OpenBrowser mở URL bằng trình duyệt mặc định; lỗi chỉ mang tính tham khảo
// (người dùng luôn được in URL để tự mở).
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// decodeJWTClaims giải mã phần payload của JWT (không xác minh chữ ký — chỉ để
// đọc claim định danh do chính máy chủ token vừa cấp qua TLS).
func decodeJWTClaims(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return nil
	}
	var claims map[string]any
	if json.Unmarshal(data, &claims) != nil {
		return nil
	}
	return claims
}

func claimObject(claims map[string]any, key string) map[string]any {
	obj, _ := claims[key].(map[string]any)
	return obj
}

func claimString(obj map[string]any, key string) string {
	s, _ := obj[key].(string)
	return strings.TrimSpace(s)
}
