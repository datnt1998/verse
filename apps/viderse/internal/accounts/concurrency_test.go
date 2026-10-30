// Kiểm tra: GET đồng thời trên cache đăng nhập không đua dữ liệu, và khoá
// thao tác không chờ — busy/fallback ngay.
package accounts

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// TestConcurrentLoginReadsRaceFree chạy GET (bilibiliLogin + CookieHeader)
// song song với mọi thao tác ghi; định chạy cùng -race để phát hiện đọc/ghi
// đua trên snapshot cache.
func TestConcurrentLoginReadsRaceFree(t *testing.T) {
	e := newQREnv(t)
	var navCalls int32
	e.setHandle(func(p string, r *http.Request, w http.ResponseWriter) bool {
		if p == "/x/web-interface/nav" {
			navCalls++
			writeJSON(w, `{"code":0,"data":{"isLogin":true,"uname":"phu"}}`)
			return true
		}
		return false
	})
	seedQRState(t, e.svc, bilibiliLoginState{})

	ctx := context.Background()
	var wg sync.WaitGroup
	stop := make(chan struct{})
	// Độc: status view + cookie header song song, liên tục.
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					if _, err := e.svc.bilibiliLogin(ctx); err != nil {
						t.Errorf("read: %v", err)
						return
					}
					if _, err := e.svc.CookieHeader(ctx); err != nil {
						t.Errorf("cookie: %v", err)
						return
					}
					if _, err := e.svc.Login(ctx, domain.PlatformBilibili); err != nil {
						t.Errorf("login view: %v", err)
						return
					}
				}
			}
		}()
	}
	// Ghi: save manual, check, clear, lặp lại. Tất cả dưới khoá thao tác.
	for i := 0; i < 8; i++ {
		cookie := "manual-" + string(rune('a'+i))
		if _, err := e.svc.SaveManualCookie(ctx, cookie); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
		if i%2 == 0 {
			if _, err := e.svc.Check(ctx, domain.PlatformBilibili); err != nil {
				t.Fatalf("check %d: %v", i, err)
			}
		}
	}
	close(stop)
	wg.Wait()
}

// TestBusyGateDoesNotQueue giữ khoá đăng nhập bằng một lời gọi mạng chặn, rồi
// chứng minh các thao tác khác trả busy ngay (không xếp hàng) và EnsureFresh
// vẫn trả cookie hiện có.
func TestBusyGateDoesNotQueue(t *testing.T) {
	e := newQREnv(t)
	release := make(chan struct{})
	entered := make(chan struct{})
	var enterOnce sync.Once
	e.setHandle(func(p string, r *http.Request, w http.ResponseWriter) bool {
		if p == "/cookie/info" {
			// Lời gọi mạng chạy khi đang giữ khoá đăng nhập: báo cho test rồi
			// giữ nguyên chỗ cho tới khi test cho phép.
			enterOnce.Do(func() { close(entered) })
			<-release
			writeJSON(w, `{"code":-101,"data":null}`)
			return true
		}
		return false
	})
	seedQRState(t, e.svc, bilibiliLoginState{})

	done := make(chan error, 1)
	go func() {
		_, _, err := e.svc.Refresh(context.Background())
		done <- err
	}()
	// Chờ lời gọi đầu vào tới lời gọi mạng (đang giữ khoá). Không dò bằng
	// TryLock: cổng busy không xếp hàng, nên lần dò có thể làm lời gọi đầu
	// trả busy và không bao giờ giữ khoá.
	select {
	case <-entered:
	case err := <-done:
		t.Fatalf("lời gọi đầu kết thúc trước khi giữ khoá: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("lời gọi đầu không giữ khoá được")
	}

	start := time.Now()
	if _, _, err := e.svc.Refresh(context.Background()); err == nil {
		t.Fatal("refresh thứ hai phải busy")
	} else if d := time.Since(start); d > 50*time.Millisecond {
		t.Fatalf("busy trả sau %v — phải ngay", d)
	}
	var le *LoginError
	if _, _, err := e.svc.Refresh(context.Background()); !errors.As(err, &le) || le.Reason != ReasonBusy {
		t.Fatalf("reason = %v", err)
	}
	// EnsureFresh không chờ lượt refresh đang chạy mà trả cookie hiện có.
	if got := e.svc.EnsureFresh(context.Background()); got != oldCookie {
		t.Fatalf("EnsureFresh = %q, muốn cookie cũ", got)
	}
	// GET/đọc bên dưới cacheMu không bị chặn bởi chuỗi mạng đang giữ khoá.
	if _, err := e.svc.Login(context.Background(), domain.PlatformBilibili); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("GET bị chặn %v bởi mu", d)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("lời gọi đầu: %v", err)
	}
}

// TestEnsureFreshReturnsCookieWhileLocked kiểm nhanh EnsureFresh không chờ
// khoá mà vẫn trả cookie hiện có khi một thao tác khác giữ mu.
func TestEnsureFreshReturnsCookieWhileLocked(t *testing.T) {
	e := newQREnv(t)
	seedQRState(t, e.svc, bilibiliLoginState{})
	e.svc.mu.Lock()
	got := e.svc.EnsureFresh(context.Background())
	e.svc.mu.Unlock()
	if got != oldCookie {
		t.Fatalf("EnsureFresh = %q, muốn %q", got, oldCookie)
	}
	if len(e.calls) != 0 {
		t.Fatalf("không được gọi mạng khi khoá bận, được %v", e.calls)
	}
}
