package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	clonebackdrop "github.com/datnt1998/verse/apps/viderse/internal/clone/backdrop"
	cloneimages "github.com/datnt1998/verse/apps/viderse/internal/clone/images"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

type imageHTTPFixture struct {
	*cloneimages.Service
	input cloneimages.ReviewInput
	calls int
	err   error
}

func (f *imageHTTPFixture) Read(context.Context, string) (cloneimages.View, error) {
	revision := 7
	return cloneimages.View{Manifest: &cloneimages.ManifestView{Revision: revision}, Items: []cloneimages.ItemView{{ID: "sc-f0", Kind: "scene"}}, Approved: false}, f.err
}
func (f *imageHTTPFixture) Review(_ context.Context, _ string, input cloneimages.ReviewInput) (cloneimages.ReviewResult, error) {
	f.input = input
	f.calls++
	return cloneimages.ReviewResult{}, f.err
}
func (f *imageHTTPFixture) UploadCover(context.Context, string, string, cloneimages.RevisionInput) (cloneimages.CoverResult, error) {
	f.calls++
	return cloneimages.CoverResult{}, f.err
}

func TestCloneImageReviewRoutePreservesRevisionAndFeedback(t *testing.T) {
	tg := newTestGateway(t)
	svc := &imageHTTPFixture{}
	MountCloneOperations(tg.g, CloneOperations{Images: svc})
	MountClone(tg.g, &fakeCloneService{}, nil, nil)
	cookie := tg.sessionCookie()
	w := tg.do(http.MethodPost, "/api/clone/job/images/sc-f0", cookie, `{"op":"editPrompt","prompt":"Giữ gương mặt, đổi ánh sáng","expectedRevision":7}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if svc.calls != 1 || svc.input.ExpectedRevision == nil || *svc.input.ExpectedRevision != 7 || len(svc.input.Actions) != 1 || svc.input.Actions[0].ID != "sc-f0" || svc.input.Actions[0].Prompt == nil || *svc.input.Actions[0].Prompt != "Giữ gương mặt, đổi ánh sáng" {
		t.Fatalf("lost review DTO: %+v", svc.input)
	}
	svc.err = &domain.CloneError{Code: domain.CloneErrConflict, Message: "Ảnh đã thay đổi"}
	w = tg.do(http.MethodPost, "/api/clone/job/images/sc-f0", cookie, `{"op":"approve","version":1,"expectedRevision":6}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("stale revision status %d: %s", w.Code, w.Body.String())
	}
}
func TestCloneImageReadAndValidation(t *testing.T) {
	tg := newTestGateway(t)
	svc := &imageHTTPFixture{}
	MountCloneOperations(tg.g, CloneOperations{Images: svc})
	MountClone(tg.g, &fakeCloneService{}, nil, nil)
	cookie := tg.sessionCookie()
	w := tg.do(http.MethodGet, "/api/clone/job/images/sc-f0", cookie, "")
	var item struct {
		Item     cloneimages.ItemView `json:"item"`
		Revision int                  `json:"revision"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil || w.Code != 200 || item.Item.ID != "sc-f0" || item.Revision != 7 {
		t.Fatalf("read: %d %s %v", w.Code, w.Body.String(), err)
	}
	for _, body := range []string{`{"op":"approve","expectedRevision":7}`, `{"op":"editPrompt","prompt":"","expectedRevision":7}`, `{"op":"regenerate","expectedRevision":0}`, `{"op":"bogus","expectedRevision":7}`, `{"op":"skip","expectedRevision":7,"promptId":"injected"}`} {
		w = tg.do(http.MethodPost, "/api/clone/job/images/sc-f0", cookie, body)
		if w.Code != 400 {
			t.Fatalf("body %s returned %d: %s", body, w.Code, w.Body.String())
		}
	}
	if svc.calls != 0 {
		t.Fatalf("invalid review reached service %d times", svc.calls)
	}
}
func TestCloneCoverBinaryExceptionIsExact(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		allowed      bool
	}{{"POST", "/api/clone/job/cover", true}, {"POST", "/api/clone/job/cover/extra", false}, {"POST", "/api/clone/job/images", false}, {"PUT", "/api/clone/job/cover", false}, {"POST", "/api/clone//cover", false}, {"POST", "/other/clone/job/cover", false}} {
		request := httptest.NewRequest(tc.method, tc.path, strings.NewReader("png"))
		request.Header.Set("Content-Type", "image/png")
		if got := allowFileUpload(request); got != tc.allowed {
			t.Fatalf("%s %s: got %v want %v", tc.method, tc.path, got, tc.allowed)
		}
	}
}

type backdropHTTPFixture struct {
	*clonebackdrop.Service
	input clonebackdrop.EditRequest
}

func (f *backdropHTTPFixture) Read(context.Context, string) (*clonebackdrop.EvidenceView, error) {
	return &clonebackdrop.EvidenceView{Revision: "revision-hash", RandomPicks: true}, nil
}
func (f *backdropHTTPFixture) Edit(_ context.Context, _ string, input clonebackdrop.EditRequest) (clonebackdrop.Result, error) {
	f.input = input
	return clonebackdrop.Result{Accepted: true, JobID: "job", Revision: "next-revision"}, nil
}
func TestCloneBackdropPreservesRevisionAndOwnerWindow(t *testing.T) {
	tg := newTestGateway(t)
	svc := &backdropHTTPFixture{}
	MountCloneOperations(tg.g, CloneOperations{Backdrops: svc})
	MountClone(tg.g, &fakeCloneService{}, nil, nil)
	cookie := tg.sessionCookie()
	response := tg.do(http.MethodPost, "/api/clone/job/backdrop", cookie, `{"op":"swap","itemId":"clip-1","revision":"revision-hash","channelId":"channel","videoId":"video","startSec":10,"endSec":20}`)
	if response.Code != 202 || svc.input.Revision != "revision-hash" || svc.input.ItemID != "clip-1" || svc.input.StartSec == nil || *svc.input.StartSec != 10 || svc.input.EndSec == nil || *svc.input.EndSec != 20 {
		t.Fatalf("backdrop: %d %s %+v", response.Code, response.Body.String(), svc.input)
	}
	for _, body := range []string{`{"op":"swap","channelId":"channel","videoId":"video"}`, `{"op":"reroll","videoId":"untrusted"}`, `{"op":"add","channelId":"only-channel"}`, `{"op":"remove","itemId":"../../escape"}`} {
		response = tg.do(http.MethodPost, "/api/clone/job/backdrop", cookie, body)
		if response.Code != 400 {
			t.Fatalf("invalid %s -> %d %s", body, response.Code, response.Body.String())
		}
	}
}
func TestCloneCoverRejectsKnownOversizeBeforeStaging(t *testing.T) {
	tg := newTestGateway(t)
	svc := &imageHTTPFixture{}
	MountCloneOperations(tg.g, CloneOperations{Images: svc})
	MountClone(tg.g, &fakeCloneService{}, nil, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/clone/job/cover?expectedRevision=7", strings.NewReader("image"))
	request.RemoteAddr = "192.0.2.1:1234"
	request.Host = "127.0.0.1:8789"
	request.Header.Set("Content-Type", "image/png")
	request.AddCookie(&http.Cookie{Name: testCookie, Value: tg.sessionCookie()})
	request.ContentLength = (20 << 20) + 1
	response := httptest.NewRecorder()
	tg.g.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge || svc.calls != 0 {
		t.Fatalf("oversize: %d %s calls=%d", response.Code, response.Body.String(), svc.calls)
	}
}
func TestCloneDownloadRejectsTooManyIDsBeforeOpeningFiles(t *testing.T) {
	tg := newTestGateway(t)
	MountCloneOperations(tg.g, CloneOperations{})
	MountClone(tg.g, &fakeCloneService{}, nil, nil)
	response := tg.do(http.MethodGet, "/api/clone/download?ids="+strings.TrimSuffix(strings.Repeat("job,", 51), ","), tg.sessionCookie(), "")
	if response.Code != 400 {
		t.Fatalf("unbounded zip: %d %s", response.Code, response.Body.String())
	}
}
func TestCloneOperationsServeAuthenticatedHTTPOnPrivatePort(t *testing.T) {
	tg := newTestGateway(t)
	svc := &imageHTTPFixture{}
	MountCloneOperations(tg.g, CloneOperations{Images: svc})
	MountClone(tg.g, &fakeCloneService{}, nil, nil)
	server := httptest.NewServer(tg.g)
	defer server.Close()
	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/clone/job/images/sc-f0", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.AddCookie(&http.Cookie{Name: testCookie, Value: tg.sessionCookie()})
	request.Header.Set("Origin", server.URL)
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var result struct {
		Item     cloneimages.ItemView `json:"item"`
		Revision int                  `json:"revision"`
	}
	if err = json.NewDecoder(response.Body).Decode(&result); err != nil || response.StatusCode != 200 || result.Item.ID != "sc-f0" || result.Revision != 7 {
		t.Fatalf("server %s: %d %+v %v", server.URL, response.StatusCode, result, err)
	}
}
