package web

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

// GAP-33: mở/đóng listener cho mạng nội bộ lúc đang chạy.
//
// `serve` luôn giữ listener loopback; Network thêm/bớt một http.Server thứ hai (mỗi địa chỉ IPv4 của
// máy một listener, cùng cổng) dùng chung handler là Gateway. Đóng server này không đụng tới
// worker, phiên đăng nhập hay listener loopback.

var errNoLANAddress = errors.New("Chưa tìm thấy mạng Wi-Fi nhà trên máy này")

// NetworkOptions cấu hình Network; trường để trống dùng mặc định.
type NetworkOptions struct {
	// Port là cổng dùng chung với listener loopback ("0": hệ điều hành chọn, dùng trong test).
	Port string
	// Addresses trả về các IPv4 của máy cần lắng nghe; mặc định LANAddresses.
	Addresses func() []string
	// Save lưu trạng thái bật/tắt; mặc định ghi `lan` vào ~/.ainovel/serve.json.
	Save func(lan bool) error
}

// Network quản lý listener mạng nội bộ của một Gateway.
type Network struct {
	handler http.Handler
	opt     NetworkOptions

	mu   sync.Mutex
	srv  *http.Server
	urls []string
}

// EnableNetwork gắn bộ điều khiển listener LAN vào gateway. Gọi một lần, trước khi nhận request.
func (g *Gateway) EnableNetwork(opt NetworkOptions) *Network {
	if opt.Port == "" {
		opt.Port = "0"
	}
	if opt.Addresses == nil {
		opt.Addresses = LANAddresses
	}
	if opt.Save == nil {
		opt.Save = library.SaveServeLAN
	}
	g.network = &Network{handler: g, opt: opt}
	return g.network
}

// LANAddresses liệt kê IPv4 không phải loopback của máy (Wi-Fi nhà, Tailscale...).
func LANAddresses() []string {
	var out []string
	seen := make(map[string]bool)
	interfaces, _ := net.Interfaces()
	for _, device := range interfaces {
		if device.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, _ := device.Addrs()
		for _, a := range addrs {
			ip, _, err := net.ParseCIDR(a.String())
			if err != nil || ip.To4() == nil || ip.IsLoopback() || seen[ip.String()] {
				continue
			}
			seen[ip.String()] = true
			out = append(out, ip.String())
		}
	}
	return out
}

// Open cho biết listener LAN đang mở không.
func (n *Network) Open() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.srv != nil
}

// URLs trả các địa chỉ gốc mà điện thoại truy cập được khi LAN đang mở.
func (n *Network) URLs() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.urls...)
}

// Set mở (on) hoặc đóng listener LAN; lặp lại cùng trạng thái là không làm gì. Không ghi serve.json.
// Một địa chỉ bind lỗi bị bỏ qua nếu còn địa chỉ khác mở được; không địa chỉ nào mở được thì báo lỗi.
func (n *Network) Set(on bool) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if !on {
		if n.srv != nil {
			// Close (không phải Shutdown): luồng SSE của thiết bị ngoài mạng sẽ không bao giờ tự kết thúc.
			_ = n.srv.Close()
			n.srv, n.urls = nil, nil
		}
		return nil
	}
	if n.srv != nil {
		return nil
	}
	addrs := n.opt.Addresses()
	if len(addrs) == 0 {
		return errNoLANAddress
	}
	var listeners []net.Listener
	var urls []string
	var firstErr error
	for _, addr := range addrs {
		ln, err := net.Listen("tcp", net.JoinHostPort(addr, n.opt.Port))
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		listeners = append(listeners, ln)
		_, port, _ := net.SplitHostPort(ln.Addr().String())
		urls = append(urls, "http://"+net.JoinHostPort(addr, port))
	}
	if len(listeners) == 0 {
		return fmt.Errorf("Không mở được cổng cho mạng nội bộ: %w", firstErr)
	}
	srv := &http.Server{Handler: n.handler}
	for _, ln := range listeners {
		go func() { _ = srv.Serve(ln) }()
	}
	n.srv, n.urls = srv, urls
	return nil
}

// closeNetwork được Gateway.Close gọi khi tắt máy chủ.
func (g *Gateway) closeNetwork() {
	if g.network != nil {
		_ = g.network.Set(false)
	}
}

// lanEnabled là giá trị `lan` của GET /api/system: điện thoại có thể vào được không.
// Máy chủ chạy với --addr cố định (không có Network) chỉ cần có địa chỉ công khai.
func (g *Gateway) lanEnabled() bool {
	if g.network != nil {
		return g.network.Open()
	}
	return len(g.PublicURLs) > 0
}

// serveNetwork xử lý PUT /api/system/network {lan}; false nghĩa là đường dẫn không thuộc nó.
func (g *Gateway) serveNetwork(w http.ResponseWriter, r *http.Request, user webauth.User) bool {
	if r.URL.Path != "/api/system/network" {
		return false
	}
	if r.Method != http.MethodPut {
		jsonError(w, http.StatusNotFound, "không tìm thấy")
		return true
	}
	if !isAdmin(user) {
		jsonError(w, http.StatusForbidden, "Không có quyền")
		return true
	}
	if !isDirectLoopback(r) {
		jsonError(w, http.StatusForbidden, "Chỉ bật hoặc tắt được khi dùng ngay trên máy chủ")
		return true
	}
	var req struct {
		LAN *bool `json:"lan"`
	}
	if !decodeJSON(w, r, &req) {
		return true
	}
	if req.LAN == nil {
		jsonError(w, http.StatusBadRequest, "thiếu trường lan")
		return true
	}
	if g.network == nil {
		jsonError(w, http.StatusConflict, "Máy chủ được khởi động với địa chỉ cố định nên không đổi được ở đây")
		return true
	}
	was := g.network.Open()
	if err := g.network.Set(*req.LAN); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errNoLANAddress) {
			status = http.StatusConflict
		}
		jsonError(w, status, err.Error())
		return true
	}
	if err := g.network.opt.Save(*req.LAN); err != nil {
		_ = g.network.Set(was) // không để trạng thái đang chạy lệch với tệp đã lưu
		jsonError(w, http.StatusInternalServerError, err.Error())
		return true
	}
	out := map[string]any{"lan": *req.LAN, "links": []pairingLink{}}
	if *req.LAN {
		code, expires, links, err := g.pairingLinks(r, user.Username, user.Username)
		if err != nil {
			accountError(w, err)
			return true
		}
		out["links"], out["code"], out["expiresAt"] = links, code, expires
	}
	writeJSON(w, http.StatusOK, out)
	return true
}
