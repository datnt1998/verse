package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// VersionTTL là nhịp hỏi bản mới của app (Kiểm tra bản mới bỏ qua TTL).
const VersionTTL = 6 * time.Hour

// VersionSource trả tag bản mới nhất của app ("" khi không có bản phát hành).
type VersionSource interface {
	Latest(ctx context.Context) (string, error)
}

// GitHubVersion đọc release mới nhất qua API công khai của GitHub. Repo
// riêng tư/không có release trả lỗi — màn Cài đặt coi như "Bản mới nhất".
type GitHubVersion struct {
	Client *http.Client
	URL    string
}

// NewGitHubVersion dựng nguồn cho repo owner/name trên api.github.com.
func NewGitHubVersion(owner, name string) *GitHubVersion {
	return &GitHubVersion{
		Client: &http.Client{Timeout: 10 * time.Second},
		URL:    fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, name),
	}
}

// Latest trả tag_name của release mới nhất.
func (g *GitHubVersion) Latest(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := g.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub trả HTTP %d", res.StatusCode)
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return "", err
	}
	return strings.TrimPrefix(strings.TrimSpace(body.TagName), "v"), nil
}

// VersionInfo là hình của GET /api/version.
type VersionInfo struct {
	Version   string
	Latest    *string
	CheckedAt time.Time
}

// VersionChecker cache kết quả kiểm bản mới; một lượt hỏi một lúc.
type VersionChecker struct {
	current string
	source  VersionSource
	now     func() time.Time

	mu        sync.Mutex
	latest    *string
	checkedAt time.Time
	inflight  chan struct{}
}

// NewVersionChecker dựng bộ kiểm; current là phiên bản app đang chạy.
func NewVersionChecker(current string, source VersionSource) *VersionChecker {
	return &VersionChecker{current: current, source: source, now: time.Now}
}

// Get trả thông tin đã cache — không gọi mạng (GET phải thuần đọc).
func (v *VersionChecker) Get(context.Context) VersionInfo {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.infoLocked()
}

// Check hỏi lại nguồn (nút Kiểm tra bản mới); gọi đồng thời gộp một lượt.
func (v *VersionChecker) Check(ctx context.Context) VersionInfo {
	v.mu.Lock()
	if v.inflight != nil {
		wait := v.inflight
		v.mu.Unlock()
		select {
		case <-wait:
		case <-ctx.Done():
		}
		v.mu.Lock()
		info := v.infoLocked()
		v.mu.Unlock()
		return info
	}
	done := make(chan struct{})
	v.inflight = done
	v.mu.Unlock()

	latest, err := v.source.Latest(ctx)

	v.mu.Lock()
	if err == nil {
		v.latest = nil
		if latest != "" {
			tag := latest
			v.latest = &tag
		}
	}
	v.checkedAt = v.now()
	v.inflight = nil
	close(done)
	info := v.infoLocked()
	v.mu.Unlock()
	return info
}

func (v *VersionChecker) infoLocked() VersionInfo {
	info := VersionInfo{Version: v.current, CheckedAt: v.checkedAt}
	if v.latest != nil {
		tag := *v.latest
		info.Latest = &tag
	}
	return info
}
