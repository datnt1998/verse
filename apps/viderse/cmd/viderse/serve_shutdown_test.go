package main

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/platform/events"
)

// Một tab đang mở /api/events không được giữ lệnh dừng tới hết hạn: dừng phải
// xong ngay và không trả lỗi.
func TestStopHTTPEndsOpenEventStreams(t *testing.T) {
	hub := events.New(func(any, events.Event) bool { return true }, events.PingEvery(time.Hour))
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sub, replay, stale := hub.Subscribe("u", 0)
		defer hub.Unsubscribe(sub)
		_ = hub.Serve(w, r, sub, replay, stale, events.Hooks{})
	})}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve(ln) }()

	resp, err := http.Get("http://" + ln.Addr().String() + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	// Chờ frame hello để chắc luồng đã mở trước khi dừng.
	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil || !strings.HasPrefix(line, "event: hello") {
		t.Fatalf("frame đầu = %q, %v", line, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	if err := <-stopHTTP(ctx, server, hub); err != nil {
		t.Fatalf("dừng trả lỗi sau %s: %v", time.Since(start), err)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("dừng mất %s khi còn luồng SSE mở", took)
	}
}
