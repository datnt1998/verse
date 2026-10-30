// Ca test hồi quy cho các phát hiện trong review: bootstrap dữ liệu xấu không
// panic, waiter bootstrap tôn trọng ctx riêng, ngân sách chờ tích luỹ của pacer,
// lỗi mạng khi đọc thân xếp transient, dữ liệu lưu trữ parse chặt, pacer mặc
// định có jitter thật.
package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"syscall"
	"testing"
	"time"
)

// ---- session data xấu → lỗi đã phân loại, không panic ----

func swPage(device []any) string {
	// Cùng layout sw.js_data: data[0][2] = [[device_info, ...], api_key, ...]
	return ")]}'\n\n" + fmt.Sprintf(`[["yt.sw.adr",null,[[%s,null,null,null,null,null],"FIXTURE_API_KEY",null,null,null,null,null]]]`, mustJSON(device))
}

func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func TestBootstrapShortPrefixIsClassifiedNotPanic(t *testing.T) {
	for _, body := range []string{")]'", ")]}'", ")]}'\n"} {
		ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
			kindSW: func(tracedReq) (*http.Response, error) { return okResponse(body), nil },
		})
		_, err := newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
		fe := fetchError(t, err)
		if fe.Kind != KindTransient {
			t.Fatalf("body %q: kind = %q", body, fe.Kind)
		}
	}
}

func TestBootstrapShortDeviceArrayIsClassifiedNotPanic(t *testing.T) {
	// device_info chỉ 10 phần tử (thiếu cả clientVersion lẫn visitorData).
	short := make([]any, 10)
	for i := range short {
		short[i] = "x"
	}
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindSW: func(tracedReq) (*http.Response, error) { return okResponse(swPage(short)), nil },
	})
	_, err := newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	fe := fetchError(t, err)
	if fe.Kind != KindUnexpectedPage && fe.Kind != KindTransient {
		t.Fatalf("kind = %q", fe.Kind)
	}
}

func TestBootstrapMissingTailFieldsDoesNotPanic(t *testing.T) {
	// device_info đủ clientVersion/visitorData nhưng thiếu các trường đuôi
	// (index 79, 86, 87): đọc lười phải trả "" thay vì panic.
	device := make([]any, 60)
	for i := range device {
		device[i] = ""
	}
	device[13] = "CgtGaXh0dXJlVmlzMSiA8J3HBg%3D%3D"
	device[16] = "2.20261012.01.00"
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindSW: func(tracedReq) (*http.Response, error) { return okResponse(swPage(device)), nil },
	})
	page, err := newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("FirstChannelPage: %v", err)
	}
	if len(page.Videos) != 4 {
		t.Fatalf("videos = %d", len(page.Videos))
	}
}

func TestBootstrapTruncatedJSONIsClassifiedNotPanic(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindSW: func(tracedReq) (*http.Response, error) { return okResponse(")]}'\n\n[[\"yt.sw\""), nil },
	})
	_, err := newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	fe := fetchError(t, err)
	if fe.Kind != KindTransient {
		t.Fatalf("kind = %q", fe.Kind)
	}
}

// ---- waiter bootstrap tôn trọng ctx của chính nó ----

func TestBootstrapWaiterHonorsOwnContext(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindSW: func(tracedReq) (*http.Response, error) {
			select {
			case entered <- struct{}{}:
			default:
			}
			<-release
			return okResponse(fixtureText(t, "sw-js-data.txt")), nil
		},
	})
	c := newFixtureClient(ft)

	ownerDone := make(chan error, 1)
	go func() {
		_, err := c.FirstChannelPage(context.Background(), testChannel)
		ownerDone <- err
	}()
	<-entered // owner đang giữ bootstrap

	// Waiter có deadline ngắn: phải trả ctx của mình chứ không chờ owner mãi.
	ctxWaiter, cancelWaiter := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancelWaiter()
	waiterDone := make(chan error, 1)
	go func() {
		_, err := c.FirstChannelPage(ctxWaiter, testChannel)
		waiterDone <- err
	}()
	select {
	case err := <-waiterDone:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("waiter err = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waiter bị kẹt theo bootstrap của caller khác")
	}

	close(release)
	if err := <-ownerDone; err != nil {
		t.Fatalf("owner err = %v", err)
	}
	// Flight hoàn tất: session chia sẻ được cache, caller kế dùng luôn.
	if _, err := c.session(context.Background(), ""); err != nil {
		t.Fatalf("session sau bootstrap: %v", err)
	}
}

// ---- MaxWait là ngân sách chờ tích luỹ ----

func TestPacerMaxWaitIsCumulative(t *testing.T) {
	p := &Pacer{MinGap: time.Nanosecond, MaxGap: time.Nanosecond, MaxWait: 300 * time.Millisecond}
	hold := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_, _ = p.Run(context.Background(), func(ctx context.Context) (*http.Response, error) {
			close(hold)
			<-release
			return okResp(), nil
		})
	}()
	<-hold // request đầu đang chạy, giữ ô lịch ~ vô hạn

	start := time.Now()
	_, err := p.Run(context.Background(), func(ctx context.Context) (*http.Response, error) {
		return okResp(), nil
	})
	elapsed := time.Since(start)
	close(release)

	var de *DeferredError
	if !errors.As(err, &de) {
		t.Fatalf("mong đợi *DeferredError, nhận %v sau %v", err, elapsed)
	}
	// Trước sửa: mỗi nhịp 250ms xin thêm MaxWait mới → chờ vô hạn khi ô bận.
	if elapsed > 2*time.Second {
		t.Fatalf("chờ %v vượt ngân sách tích luỹ %v", elapsed, 300*time.Millisecond)
	}
}

func TestPacerScheduledWaitAlsoCountsTowardBudget(t *testing.T) {
	now := time.Now()
	clock := &fakeClock{t: now}
	p := &Pacer{Now: clock.Now, MinGap: time.Nanosecond, MaxGap: time.Nanosecond, MaxWait: 300 * time.Millisecond}
	p.penalize(now.Add(10 * time.Second))
	start := time.Now()
	_, err := p.Run(context.Background(), func(ctx context.Context) (*http.Response, error) {
		return okResp(), nil
	})
	var de *DeferredError
	if !errors.As(err, &de) {
		t.Fatalf("mong đợi *DeferredError, nhận %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("DeferredError phải trả ngay khi ô lịch quá xa")
	}
}

// ---- lỗi mạng khi đọc thân → transient, lỗi lạ → unexpected_page ----

type failBody struct{ err error }

func (f *failBody) Read([]byte) (int, error) { return 0, f.err }
func (f *failBody) Close() error             { return nil }

func TestContinuationBodyReadNetworkErrorIsTransient(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, nil)
	c := newFixtureClient(ft)
	first, err := c.FirstChannelPage(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("FirstChannelPage: %v", err)
	}
	ft.install(t, map[reqKind]handlerFunc{
		kindContinuation: func(tracedReq) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": {"application/json"}},
				Body:       &failBody{err: &net.OpError{Op: "read", Net: "tcp", Err: syscall.ECONNRESET}},
			}, nil
		},
	})
	_, err = c.NextChannelPage(context.Background(), *first.NextCursor)
	fe := fetchError(t, err)
	if fe.Kind != KindTransient {
		t.Fatalf("kind = %q, muốn transient (reset khi đọc thân không phải cursor xấu)", fe.Kind)
	}
}

func TestUnknownContinuationErrorIsUnexpectedPageNotCursorInvalid(t *testing.T) {
	// Chỉ lỗi "dữ liệu continuation không dùng được" mới là cursor_invalid;
	// lỗi không rõ loại vẫn là trang bất thường như bản TS.
	err := normalizeError(errors.New("weird internal failure"), true, time.Now())
	fe := fetchError(t, err)
	if fe.Kind != KindUnexpectedPage {
		t.Fatalf("kind = %q", fe.Kind)
	}
	err = normalizeError(errors.New("weird internal failure"), false, time.Now())
	if kind, _ := KindOf(fetchError(t, err)); kind != KindUnexpectedPage {
		t.Fatalf("kind = %q", kind)
	}
}

// ---- dữ liệu lưu trữ parse chặt như schema gốc ----

func TestCursorRequiresSessionKeysAndEOF(t *testing.T) {
	raw, err := json.Marshal(validCursor())
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}

	// visitorData thiếu hẳn (schema là nullable bắt buộc, không optional).
	sess := m["session"].(map[string]any)
	delete(sess, "visitorData")
	bad, _ := json.Marshal(m)
	if _, err := ParseChannelCursor(string(bad)); err == nil {
		t.Fatal("cursor thiếu visitorData được chấp nhận")
	}

	// JSON hợp lệ theo sau rác: phải hết hẳn input.
	if _, err := ParseChannelCursor(string(raw) + "}"); err == nil {
		t.Fatal("cursor có dữ liệu thừa được chấp nhận")
	}
	if _, err := ParseChannelCursor(string(raw) + " null"); err == nil {
		t.Fatal("cursor có giá trị thứ hai được chấp nhận")
	}
}

func TestRemainderRequiresArrayAndFullKeys(t *testing.T) {
	if _, err := ParseScannedVideos("null"); err == nil {
		t.Fatal("null remainder được chấp nhận")
	}
	if _, err := ParseScannedVideos("{}"); err == nil {
		t.Fatal("object remainder được chấp nhận")
	}
	v := testVideo("aaaaaaaaaaa", nil)
	rawV, _ := json.Marshal(v)
	var m map[string]any
	if err := json.Unmarshal(rawV, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"id", "title", "thumbnailUrl", "durationSec", "viewCount", "publishedText", "publishedAt"} {
		miss := map[string]any{}
		for k, val := range m {
			if k != key {
				miss[k] = val
			}
		}
		raw, _ := json.Marshal([]any{miss})
		if _, err := ParseScannedVideos(string(raw)); err == nil {
			t.Fatalf("video thiếu khoá %q được chấp nhận", key)
		}
	}
	// Giá trị nullable vẫn hợp lệ khi key hiện diện.
	okRaw, _ := json.Marshal([]any{m})
	if _, err := ParseScannedVideos(string(okRaw)); err != nil {
		t.Fatalf("video đủ khoá bị từ chối: %v", err)
	}
}

// ---- pacer mặc định có jitter thật ----

func TestNewPacerHasRealJitterByDefault(t *testing.T) {
	p := NewPacer()
	if p.Rand == nil {
		t.Fatal("NewPacer phải có nguồn ngẫu nhiên thật (jitter), nil làm nhịp luôn MinGap")
	}
	j := p.jitter()
	if j < time.Second || j >= 2*time.Second+time.Nanosecond {
		t.Fatalf("jitter = %v ngoài [1s,2s]", j)
	}
	// Client mặc định cũng phải jitter; http client có deadline hữu hạn.
	c := NewClient(Options{})
	if c.pacer.Rand == nil {
		t.Fatal("pacer mặc định của client thiếu jitter")
	}
	if c.http.Timeout <= 0 {
		t.Fatal("http client mặc định cần timeout hữu hạn")
	}
}

// Đảm bảo waiter bị huỷ không kéo theo bootstrap của owner.
func TestCancelledWaiterDoesNotPoisonSharedBootstrap(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	var once sync.Once
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindSW: func(tracedReq) (*http.Response, error) {
			once.Do(func() { entered <- struct{}{} })
			<-release
			return okResponse(fixtureText(t, "sw-js-data.txt")), nil
		},
	})
	c := newFixtureClient(ft)
	go func() {
		_, _ = c.FirstChannelPage(context.Background(), testChannel)
	}()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := c.FirstChannelPage(ctx, testChannel)
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("waiter err = %v", err)
	}
	close(release)
	// Owner vẫn hoàn tất và cache; caller sau đi tiếp bình thường.
	page, err := c.FirstChannelPage(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("caller sau waiter huỷ: %v", err)
	}
	if len(page.Videos) != 4 {
		t.Fatalf("videos = %d", len(page.Videos))
	}
	// Chỉ một bootstrap sw duy nhất.
	count := 0
	for _, r := range ft.trace {
		if r.kind == kindSW {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("sw bootstrap count = %d", count)
	}
}
