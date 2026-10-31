package backdrop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// apiDoer giả lập Bilibili: ghi lại đường dẫn + header, trả phản hồi theo hàm.
type apiDoer struct {
	calls    []string
	headers  []http.Header
	response func(path string) (*http.Response, error)
}

func (d *apiDoer) Do(req *http.Request) (*http.Response, error) {
	d.calls = append(d.calls, req.URL.Path)
	d.headers = append(d.headers, req.Header.Clone())
	return d.response(req.URL.Path)
}

func apiJSON(code int, data string) *http.Response {
	body := fmt.Sprintf(`{"code":%d,"data":%s}`, code, data)
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}

func apiStatus(status int, body string, retryAfter string) *http.Response {
	header := http.Header{}
	if retryAfter != "" {
		header.Set("Retry-After", retryAfter)
	}
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body))}
}

func writeDevice(t *testing.T, dir string, state deviceState) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "bilibili-device.json"), mustJSON(t, state), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func newTestClient(t *testing.T, dir string, doer HTTPDoer, now time.Time, cookie string) *bilibiliClient {
	t.Helper()
	client, err := newBilibiliClient(doer, func() time.Time { return now },
		func(ctx context.Context, _ bool, fn func(context.Context) error) error { return fn(ctx) },
		newDeviceStore(dir), func(context.Context) (string, error) { return cookie, nil })
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	return client
}

func TestBilibiliClientUsesCachedKeysAndDeviceCookies(t *testing.T) {
	dir := t.TempDir()
	writeDevice(t, dir, deviceState{
		Buvid3: "device-three", Buvid4: "device-four",
		ImgKey: wbiImg, SubKey: wbiSub, KeysExpireAt: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
	})
	doer := &apiDoer{response: func(string) (*http.Response, error) { return apiJSON(0, pageFixture), nil }}
	now := time.Unix(1702204169, 0)
	client := newTestClient(t, dir, doer, now, "SESSDATA=private")

	for _, pn := range []int{1, 2} {
		if _, err := client.Page(context.Background(), "3816626", pn); err != nil {
			t.Fatalf("page %d: %v", pn, err)
		}
	}
	if len(doer.calls) != 2 || doer.calls[0] != "/x/space/wbi/arc/search" {
		t.Fatalf("calls = %v, muốn hai trang và không gọi nav", doer.calls)
	}
	for _, header := range doer.headers {
		cookie := header.Get("Cookie")
		if !strings.Contains(cookie, "SESSDATA=private") || !strings.Contains(cookie, "buvid3=device-three") {
			t.Fatalf("cookie = %q", cookie)
		}
		if header.Get("Referer") != "https://space.bilibili.com/3816626/video" {
			t.Fatalf("referer = %q", header.Get("Referer"))
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "bilibili-device.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "private") {
		t.Fatal("cookie đăng nhập không được ghi vào tệp thiết bị")
	}
	mode := fileMode(t, filepath.Join(dir, "bilibili-device.json"))
	if mode != 0o600 {
		t.Fatalf("mode = %o, muốn 0600", mode)
	}
}

func TestBilibiliClientBootstrapsAnonymousNavAndPersistsKeys(t *testing.T) {
	dir := t.TempDir()
	doer := &apiDoer{response: func(path string) (*http.Response, error) {
		if strings.HasSuffix(path, "/nav") {
			return apiJSON(-101, fmt.Sprintf(`{"wbi_img":{"img_url":"https://i.test/%s.png","sub_url":"https://i.test/%s.png"}}`, wbiImg, wbiSub)), nil
		}
		return apiJSON(0, pageFixture), nil
	}}
	client := newTestClient(t, dir, doer, time.Unix(1702204169, 0), "")
	if _, err := client.Page(context.Background(), "3816626", 1); err != nil {
		t.Fatalf("page: %v", err)
	}
	if len(doer.calls) != 2 || !strings.HasSuffix(doer.calls[0], "/nav") {
		t.Fatalf("calls = %v, muốn nav rồi trang", doer.calls)
	}
	var state deviceState
	raw, err := os.ReadFile(filepath.Join(dir, "bilibili-device.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	if state.ImgKey != wbiImg || state.SubKey != wbiSub || state.KeysExpireAt == 0 {
		t.Fatalf("state = %+v, muốn khoá WBI đã đệm", state)
	}
}

func TestBilibiliClientBlockedPageCarriesRetryAfterWithoutRetry(t *testing.T) {
	dir := t.TempDir()
	writeDevice(t, dir, deviceState{Buvid3: "a", Buvid4: "b", ImgKey: wbiImg, SubKey: wbiSub, KeysExpireAt: 9999999999999})
	doer := &apiDoer{response: func(string) (*http.Response, error) {
		return apiStatus(http.StatusPreconditionFailed, "secret response", "1800"), nil
	}}
	client := newTestClient(t, dir, doer, time.Unix(1702204169, 0), "")
	_, err := client.Page(context.Background(), "3816626", 2)
	var apiErr *BilibiliAPIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, muốn BilibiliAPIError", err)
	}
	if apiErr.ErrorCode() != CodeRateLimited || apiErr.RetryAfter() != 30*time.Minute {
		t.Fatalf("err = %+v", apiErr)
	}
	if strings.Contains(apiErr.Error(), "secret") {
		t.Fatal("câu lỗi không được lộ nội dung phản hồi")
	}
	if len(doer.calls) != 1 {
		t.Fatalf("calls = %d, muốn 1 (không thử lại)", len(doer.calls))
	}
}

func TestBilibiliClientRiskControlInvalidatesKeys(t *testing.T) {
	dir := t.TempDir()
	writeDevice(t, dir, deviceState{Buvid3: "a", Buvid4: "b", ImgKey: wbiImg, SubKey: wbiSub, KeysExpireAt: 9999999999999})
	doer := &apiDoer{response: func(string) (*http.Response, error) {
		return apiJSON(-352, "null"), nil
	}}
	client := newTestClient(t, dir, doer, time.Unix(1702204169, 0), "")
	_, err := client.Page(context.Background(), "3816626", 1)
	var apiErr *BilibiliAPIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode() != CodeRateLimited {
		t.Fatalf("err = %v", err)
	}
	if len(doer.calls) != 1 {
		t.Fatalf("calls = %d, muốn 1", len(doer.calls))
	}
	var state deviceState
	raw, err := os.ReadFile(filepath.Join(dir, "bilibili-device.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	if state.KeysExpireAt != 0 {
		t.Fatalf("keysExpireAt = %d, muốn 0 (khoá bị vô hiệu)", state.KeysExpireAt)
	}
}

func TestBilibiliErrorCodesMapToSharedContract(t *testing.T) {
	for _, code := range []int{412, 429, -412, -352, -401, -403, -799} {
		if got := (&BilibiliAPIError{APICode: code}).ErrorCode(); got != CodeRateLimited {
			t.Fatalf("code %d = %q, muốn rate_limited", code, got)
		}
	}
	if got := (&BilibiliAPIError{APICode: -404}).ErrorCode(); got != CodeUnavailable {
		t.Fatalf("-404 = %q", got)
	}
	if got := (&BilibiliAPIError{APICode: -500}).ErrorCode(); got != CodeFailed {
		t.Fatalf("-500 = %q", got)
	}
}

func TestParseBilibiliPageRejectsBadUploadID(t *testing.T) {
	_, err := parseBilibiliPage(json.RawMessage(`{"page":{"count":3},"list":{"vlist":[{"bvid":"bad"}]}}`))
	if err == nil {
		t.Fatal("bvid sai phải làm hỏng cả trang")
	}
}

func TestReadAllLimitedRejectsOversize(t *testing.T) {
	big := strings.NewReader(strings.Repeat("x", 9<<20))
	if _, err := readAllLimited(big); err == nil {
		t.Fatal("phản hồi quá trần phải lỗi")
	}
	if raw, err := readAllLimited(strings.NewReader("nhỏ")); err != nil || string(raw) != "nhỏ" {
		t.Fatalf("phản hồi nhỏ: %q %v", raw, err)
	}
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}
