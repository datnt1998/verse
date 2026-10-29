package web

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// E2ELIB-1: a slow proxied worker action must not hold any gateway lock that
// library reads (directly or behind a concurrent delete) need.
func TestSlowWorkerInputDoesNotWedgeLibrary(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	done := make(chan struct{})
	var stop sync.Once
	tg.g.launch = func(_, _, _, _ string) (*launchedWorker, error) {
		return &launchedWorker{addr: "127.0.0.1:1", stdin: io.NopCloser(strings.NewReader("")), done: done, kill: func() error { stop.Do(func() { close(done) }); return nil }}, nil
	}
	inputSeen := make(chan struct{})
	release := make(chan struct{})
	tg.g.setTransport(transportFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/input":
			close(inputSeen)
			select {
			case <-release:
			case <-r.Context().Done():
			}
			return &http.Response{StatusCode: 202, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"ok":true,"action":"start","pending":true}`))}, nil
		case "/api/events":
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("event: jobs.reset\ndata: {\"jobs\":[]}\n\n"))}, nil
		default:
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"isRunning":false}`))}, nil
		}
	}))
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		stop.Do(func() { close(done) })
	})

	inputDone := make(chan int, 1)
	go func() {
		inputDone <- tg.do("POST", "/api/projects/_/stories/oldbook/input", admin, `{"text":"Bắt đầu"}`).Code
	}()
	select {
	case <-inputSeen:
	case <-time.After(2 * time.Second):
		t.Fatal("input never reached the worker")
	}

	// The worker request stays blocked until release is closed below, so any
	// lock held across the proxy makes these calls wait forever. hangLimit only
	// turns that into a failure; it is not a performance threshold.
	const hangLimit = 10 * time.Second
	deleteDone := make(chan int, 1)
	go func() {
		deleteDone <- doFrom(t, tg.g, "DELETE", "/api/projects/_/stories/oldbook", admin, `{"confirm":"oldbook"}`, "192.0.2.1:1234", map[string]string{"Content-Type": "application/json"}).Code
	}()
	select {
	case code := <-deleteDone:
		if code != 409 {
			t.Fatalf("delete during in-flight action: %d", code)
		}
	case <-time.After(hangLimit):
		t.Fatal("delete queued behind the in-flight worker request")
	}

	libraryDone := make(chan int, 1)
	go func() { libraryDone <- tg.do("GET", "/api/library", admin, "").Code }()
	select {
	case code := <-libraryDone:
		if code != 200 {
			t.Fatalf("library: %d", code)
		}
	case <-time.After(hangLimit):
		t.Fatal("/api/library queued behind the in-flight worker request")
	}

	close(release)
	if code := <-inputDone; code != 202 {
		t.Fatalf("input: %d", code)
	}
}
