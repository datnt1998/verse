package netguard

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// proxyUser là phần tên trong Proxy-Authorization; mật khẩu là token ngẫu
// nhiên của process nên tiến trình khác trên cùng máy không dùng được proxy
// này làm relay mở.
const proxyUser = "viderse"

// proxyServer là proxy chuyển tiếp trong process: nhận HTTP thường (absolute
// URI) và CONNECT, mọi lần nối tới đích đều qua Dialer đã kiểm địa chỉ.
type proxyServer struct {
	listener  net.Listener
	server    *http.Server
	dialer    *Dialer
	transport *http.Transport
	token     string
	done      chan struct{}
	proxied   atomic.Uint64
	blocked   atomic.Uint64
}

// startProxy mở proxy trên loopback với cổng và credential ngẫu nhiên.
func startProxy(d *Dialer) (*proxyServer, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	transport := d.Transport()
	transport.DisableCompression = true
	p := &proxyServer{dialer: d, transport: transport, token: hex.EncodeToString(raw), done: make(chan struct{})}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}
	p.listener = listener
	p.server = &http.Server{
		Handler:           p,
		ReadHeaderTimeout: 30 * time.Second,
		// CONNECT là đường hầm hai chiều: không có deadline đọc/ghi tổng.
		IdleTimeout: 5 * time.Minute,
	}
	go func() {
		defer close(p.done)
		_ = p.server.Serve(listener)
	}()
	return p, nil
}

// url trả URL proxy kèm credential để truyền cho tiến trình con (user:pass
// nằm ngay trong URL — đó là dạng yt-dlp và ffmpeg nhận).
func (p *proxyServer) url() string {
	return fmt.Sprintf("http://%s:%s@%s", proxyUser, p.token, p.listener.Addr().String())
}

func (p *proxyServer) close() {
	_ = p.server.Close()
	<-p.done
	p.transport.CloseIdleConnections()
}

// ServeHTTP xử lý một request proxy: kiểm credential, rồi CONNECT (https) hay
// chuyển tiếp (http).
func (p *proxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !p.authorized(r) {
		w.Header().Set("Proxy-Authenticate", `Basic realm="viderse"`)
		http.Error(w, "proxy authentication required", http.StatusProxyAuthRequired)
		return
	}
	if r.Method == http.MethodConnect {
		p.tunnel(w, r)
		return
	}
	if r.URL == nil || (r.URL.Scheme != "http" && r.URL.Scheme != "https") || r.URL.Host == "" {
		http.Error(w, "unsupported proxy request", http.StatusBadRequest)
		return
	}
	p.forward(w, r)
}

// authorized kiểm Proxy-Authorization theo thời gian hằng.
func (p *proxyServer) authorized(r *http.Request) bool {
	const prefix = "Basic "
	header := r.Header.Get("Proxy-Authorization")
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	got, err := base64.StdEncoding.DecodeString(strings.TrimSpace(header[len(prefix):]))
	if err != nil {
		return false
	}
	want := []byte(proxyUser + ":" + p.token)
	return subtle.ConstantTimeCompare(got, want) == 1
}

// tunnel mở đường hầm CONNECT tới đích đã kiểm địa chỉ. Cổng đích không bị
// giới hạn: thành viên được tải từ mọi trang công khai, kể cả khi trang phục
// vụ ở cổng lạ; thứ cần chặn là ĐỊA CHỈ, không phải cổng.
func (p *proxyServer) tunnel(w http.ResponseWriter, r *http.Request) {
	target := r.Host
	if target == "" && r.URL != nil {
		target = r.URL.Host
	}
	if target == "" {
		http.Error(w, "missing CONNECT target", http.StatusBadRequest)
		return
	}
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		host, port = strings.Trim(target, "[]"), "443"
	}
	conn, err := p.dialer.DialContext(r.Context(), "tcp", net.JoinHostPort(host, port))
	if err != nil {
		p.fail(w, target, err)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		conn.Close()
		http.Error(w, "proxy cannot hijack", http.StatusInternalServerError)
		return
	}
	client, buffered, err := hijacker.Hijack()
	if err != nil {
		conn.Close()
		return
	}
	p.proxied.Add(1)
	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		client.Close()
		conn.Close()
		return
	}
	// buffered.Reader có thể đã giữ sẵn byte đầu tiên của client (TLS
	// ClientHello): đọc từ nó trước, không đọc thẳng client.
	go func() {
		_, _ = io.Copy(conn, buffered)
		_ = conn.Close()
		_ = client.Close()
	}()
	_, _ = io.Copy(client, conn)
	_ = client.Close()
	_ = conn.Close()
}

// forward chuyển tiếp một request HTTP thường (absolute URI) tới đích đã kiểm.
func (p *proxyServer) forward(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Hostname()
	if host == "" {
		http.Error(w, "missing proxy target", http.StatusBadRequest)
		return
	}
	port := r.URL.Port()
	if port == "" {
		if r.URL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	// Kiểm địa chỉ đúng một lần, ở DialContext của transport bên dưới: tên
	// được phân giải rồi nối bằng IP đã kiểm, nên DNS đổi giữa hai bước không
	// tạo ra đường vòng.
	out := r.Clone(r.Context())
	out.RequestURI = ""
	out.Header = r.Header.Clone()
	// hopHeaders gồm cả Proxy-Authorization/Proxy-Connection: credential của
	// process không được đi tiếp tới đích.
	for _, key := range hopHeaders {
		out.Header.Del(key)
	}
	response, err := p.transport.RoundTrip(out)
	if err != nil {
		p.fail(w, net.JoinHostPort(host, port), err)
		return
	}
	defer response.Body.Close()
	p.proxied.Add(1)
	header := w.Header()
	for key, values := range response.Header {
		if hopHeader(key) {
			continue
		}
		for _, value := range values {
			header.Add(key, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}

// fail trả lỗi cho client. Đích nội bộ nhận reason phrase đánh dấu để lỗi của
// tiến trình con được nhận ra là do cổng chặn, không lẫn với đích trả 403 thật;
// địa chỉ nội bộ không bao giờ nằm trong thân phản hồi.
func (p *proxyServer) fail(w http.ResponseWriter, target string, err error) {
	if errors.Is(err, ErrBlocked) || IsBlockedOutput(err.Error()) {
		p.blocked.Add(1)
		slog.Warn("netguard: chặn đích nội bộ", "target", target)
		writeBlocked(w)
		return
	}
	if errors.Is(err, ErrUnresolvable) {
		slog.Warn("netguard: không phân giải được đích", "target", target)
		http.Error(w, "unresolvable destination", http.StatusBadGateway)
		return
	}
	http.Error(w, "proxy upstream error", http.StatusBadGateway)
}

var hopHeaders = []string{
	"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate",
	"Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
}

func hopHeader(key string) bool {
	for _, h := range hopHeaders {
		if strings.EqualFold(key, h) {
			return true
		}
	}
	return false
}

// writeBlocked ghi phản hồi 403 với reason phrase đánh dấu. Phải tự ghi dòng
// trạng thái vì net/http luôn dùng reason chuẩn.
func writeBlocked(w http.ResponseWriter) {
	body := blockedMarker
	raw := fmt.Sprintf(
		"HTTP/1.1 403 %s\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Length: %d\r\nX-Viderse-Egress: blocked\r\nConnection: close\r\n\r\n%s",
		blockedMarker, len(body), body)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		w.Header().Set("X-Viderse-Egress", "blocked")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, body)
		return
	}
	conn, _, err := hijacker.Hijack()
	if err != nil {
		w.Header().Set("X-Viderse-Egress", "blocked")
		w.WriteHeader(http.StatusForbidden)
		return
	}
	_, _ = io.WriteString(conn, raw)
	_ = conn.Close()
}
