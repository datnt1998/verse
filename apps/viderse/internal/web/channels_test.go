package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/channels"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// fakeChannelService ghi lại lời gọi và trả dữ liệu/ lỗi theo cấu hình.
type fakeChannelService struct {
	view   channels.ChannelView
	list   []channels.ChannelView
	jobID  string
	paused bool
	err    error
	asset  string
	name   string

	createdIn        channels.CreateRequest
	updatedIn        channels.UpdateRequest
	identityIn       channels.IdentityInput
	deletedID        string
	minDurationRival string
	minDuration      *int
}

func (f *fakeChannelService) List(context.Context) ([]channels.ChannelView, error) {
	return f.list, f.err
}

func (f *fakeChannelService) Get(context.Context, string) (channels.ChannelView, error) {
	return f.view, f.err
}

func (f *fakeChannelService) Create(_ context.Context, in channels.CreateRequest) (channels.ChannelView, string, error) {
	f.createdIn = in
	return f.view, f.jobID, f.err
}

func (f *fakeChannelService) Update(_ context.Context, _ string, in channels.UpdateRequest) (channels.ChannelView, error) {
	f.updatedIn = in
	return f.view, f.err
}

func (f *fakeChannelService) Delete(_ context.Context, id string) error {
	f.deletedID = id
	return f.err
}

func (f *fakeChannelService) Link(context.Context, string, string) (channels.ChannelView, error) {
	return f.view, f.err
}

func (f *fakeChannelService) Unlink(context.Context, string, string) (channels.ChannelView, bool, error) {
	return f.view, f.paused, f.err
}

func (f *fakeChannelService) SetSourceMinDuration(_ context.Context, _, rivalID string, d *int) (channels.ChannelView, error) {
	f.minDurationRival, f.minDuration = rivalID, d
	return f.view, f.err
}

func (f *fakeChannelService) GenerateIdentity(_ context.Context, _ string, in channels.IdentityInput) (string, error) {
	f.identityIn = in
	return f.jobID, f.err
}

func (f *fakeChannelService) AssetFile(_ context.Context, _, file string) (io.ReadSeekCloser, string, int64, error) {
	if f.err != nil {
		return nil, "", 0, f.err
	}
	of, err := os.Open(f.asset)
	if err != nil {
		return nil, "", 0, err
	}
	return of, file, 0, nil
}

// channelFixture dựng gateway + service giả và trả cookie đăng nhập.
func channelFixture(t *testing.T, svc *fakeChannelService) (*testGateway, string) {
	t.Helper()
	tg := newTestGateway(t)
	MountChannels(tg.g, svc)
	return tg, tg.sessionCookie()
}

func sampleChannelView() channels.ChannelView {
	created := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
	retry := created.Add(24 * time.Hour)
	avatar := "channels/ch-1/11111111-2222-4333-8444-555555555555-avatar.png"
	code := "quota"
	msg := "Hết hạn mức ảnh hôm nay."
	return channels.ChannelView{
		Channel: domain.Channel{
			ID: "ch-1", Name: "Chuyện Kể Đêm Mưa", Language: domain.LangVI,
			AlwaysTags: []string{"ma"}, AutoFetch: true, PerDay: 5, DefaultTemplateID: "t-1",
			Branding:          domain.Branding{Brief: "mô tả", Summary: "tông tối", Look: "look", Avatar: "a", Banner: "b"},
			AvatarPath:        &avatar,
			IdentityState:     domain.IdentityFailed,
			IdentityErrorCode: &code, IdentityError: &msg, IdentityRetryAt: &retry,
			CreatedAt: created, UpdatedAt: created,
		},
		Sources: []domain.ChannelSource{{
			RivalID: "UC1", Title: "Chuyện Ma", LinkedAt: created, Reference: true,
		}},
		DefaultTemplate: &domain.ChannelTemplate{ID: "t-1", Name: "Mẫu chuẩn"},
	}
}

func TestChannelListWire(t *testing.T) {
	svc := &fakeChannelService{list: []channels.ChannelView{sampleChannelView()}}
	tg, cookie := channelFixture(t, svc)
	w := tg.do(http.MethodGet, "/api/owner-channels", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	body := decode(t, w)
	list := body["channels"].([]any)
	if len(list) != 1 {
		t.Fatalf("channels = %v", body)
	}
	c := list[0].(map[string]any)
	if c["id"] != "ch-1" || c["language"] != "vi" || c["avatarUrl"] != "/api/owner-channels/ch-1/branding/11111111-2222-4333-8444-555555555555-avatar.png" {
		t.Fatalf("channel = %v", c)
	}
	identity := c["identity"].(map[string]any)
	if identity["state"] != "failed" {
		t.Fatalf("identity = %v", identity)
	}
	if identity["error"].(map[string]any)["code"] != "quota" || identity["retryAt"] != "2026-10-31T08:00:00Z" {
		t.Fatalf("identity error = %v", identity)
	}
	sources := c["sources"].([]any)
	if sources[0].(map[string]any)["reference"] != true {
		t.Fatalf("sources = %v", sources)
	}
}

func TestChannelCreateRequiresCompetitor(t *testing.T) {
	svc := &fakeChannelService{view: sampleChannelView()}
	tg, cookie := channelFixture(t, svc)
	w := tg.do(http.MethodPost, "/api/owner-channels", cookie, `{"name":"Kênh A"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if body["code"] != channels.CodeInvalidInput || !strings.Contains(body["error"].(string), "đối thủ") {
		t.Fatalf("body = %v", body)
	}
}

// competitorIds toàn chuỗi trắng cũng là "chưa chọn đối thủ" → 400.
func TestChannelCreateBlankCompetitorIDs(t *testing.T) {
	svc := &fakeChannelService{view: sampleChannelView()}
	tg, cookie := channelFixture(t, svc)
	w := tg.do(http.MethodPost, "/api/owner-channels", cookie,
		`{"competitorIds":["","   "]}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
}

func TestChannelCreateReturnsJobID(t *testing.T) {
	svc := &fakeChannelService{view: sampleChannelView(), jobID: "task-9"}
	tg, cookie := channelFixture(t, svc)
	w := tg.do(http.MethodPost, "/api/owner-channels", cookie,
		`{"name":"Kênh A","competitorIds":["UC1"],"perDay":7,"autoFetch":false}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if body["identityJobId"] != "task-9" {
		t.Fatalf("identityJobId = %v", body["identityJobId"])
	}
	if svc.createdIn.PerDay == nil || *svc.createdIn.PerDay != 7 {
		t.Fatalf("createdIn = %+v", svc.createdIn)
	}
}

func TestChannelPatchNullable(t *testing.T) {
	svc := &fakeChannelService{view: sampleChannelView()}
	tg, cookie := channelFixture(t, svc)
	w := tg.do(http.MethodPatch, "/api/owner-channels/ch-1", cookie,
		`{"styleNote":null,"youtubeUrl":"https://youtube.com/@a","alwaysTags":[" ma ","MA"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if svc.updatedIn.StyleNote == nil || *svc.updatedIn.StyleNote != nil {
		t.Fatalf("styleNote = %v, muốn gửi null tường minh", svc.updatedIn.StyleNote)
	}
	if svc.updatedIn.YoutubeURL == nil || *svc.updatedIn.YoutubeURL == nil || **svc.updatedIn.YoutubeURL != "https://youtube.com/@a" {
		t.Fatalf("youtubeUrl = %v", svc.updatedIn.YoutubeURL)
	}
	if svc.updatedIn.AlwaysTags == nil || len(*svc.updatedIn.AlwaysTags) != 2 {
		t.Fatalf("tags = %v", svc.updatedIn.AlwaysTags)
	}
}

func TestChannelUnlinkReportsPause(t *testing.T) {
	svc := &fakeChannelService{view: sampleChannelView(), paused: true}
	tg, cookie := channelFixture(t, svc)
	w := tg.do(http.MethodDelete, "/api/owner-channels/ch-1/sources/UC1", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if body["autoFetchPaused"] != true {
		t.Fatalf("body = %s, thiếu trạng thái Tự lấy đã dừng", w.Body.String())
	}
}

func TestChannelPatchSourceMinDuration(t *testing.T) {
	svc := &fakeChannelService{view: sampleChannelView()}
	tg, cookie := channelFixture(t, svc)
	w := tg.do(http.MethodPatch, "/api/owner-channels/ch-1/sources/UC1", cookie, `{"minDurationSec":1200}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if svc.minDurationRival != "UC1" || svc.minDuration == nil || *svc.minDuration != 1200 {
		t.Fatalf("service nhận %q %v", svc.minDurationRival, svc.minDuration)
	}
	if _, ok := decode(t, w)["sources"]; !ok {
		t.Fatalf("trả về không phải kênh: %s", w.Body.String())
	}
	if w = tg.do(http.MethodPatch, "/api/owner-channels/ch-1/sources/UC1", cookie, `{"minDurationSec":null}`); w.Code != http.StatusOK || svc.minDuration != nil {
		t.Fatalf("null phải bỏ lọc: %d %v", w.Code, svc.minDuration)
	}
}

func TestChannelBrandingAccepted(t *testing.T) {
	svc := &fakeChannelService{view: sampleChannelView(), jobID: "task-3"}
	tg, cookie := channelFixture(t, svc)
	w := tg.do(http.MethodPost, "/api/owner-channels/ch-1/branding", cookie,
		`{"part":"avatar","direction":"tông lạnh"}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if svc.identityIn.Part != "avatar" || svc.identityIn.Direction != "tông lạnh" {
		t.Fatalf("identityIn = %+v", svc.identityIn)
	}
	if decode(t, w)["identityJobId"] != "task-3" {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestChannelErrorMapping(t *testing.T) {
	cases := []struct {
		code   string
		status int
	}{
		{channels.CodeInvalidInput, http.StatusBadRequest},
		{channels.CodeNotFound, http.StatusNotFound},
		{channels.CodeConflict, http.StatusConflict},
		{channels.CodeQuota, http.StatusTooManyRequests},
		{channels.CodeToolMissing, http.StatusServiceUnavailable},
		{channels.CodeInternal, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			retry := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
			svc := &fakeChannelService{err: &channels.Error{
				Code: tc.code, Message: "lỗi thử", Recovery: "thử lại", RetryAt: &retry,
			}}
			tg, cookie := channelFixture(t, svc)
			w := tg.do(http.MethodGet, "/api/owner-channels/ch-1", cookie, "")
			if w.Code != tc.status {
				t.Fatalf("status %d, muốn %d: %s", w.Code, tc.status, w.Body.String())
			}
			body := decode(t, w)
			if tc.status == http.StatusInternalServerError {
				if body["error"] != "lỗi nội bộ" || body["code"] != "internal" {
					t.Fatalf("500 phải trung lập: %v", body)
				}
				return
			}
			if body["code"] != tc.code {
				t.Fatalf("code = %v", body["code"])
			}
			if tc.code == channels.CodeQuota && body["retryAt"] != "2026-11-01T00:00:00Z" {
				t.Fatalf("retryAt = %v", body["retryAt"])
			}
		})
	}
}

func TestChannelBrandingServeDownload(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "11111111-2222-4333-8444-555555555555-banner.jpg")
	if err := os.WriteFile(file, []byte("jpeg-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := &fakeChannelService{asset: file, name: filepath.Base(file)}
	tg, cookie := channelFixture(t, svc)
	w := tg.do(http.MethodGet, "/api/owner-channels/ch-1/branding/"+filepath.Base(file)+"?download=1", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("content-type = %q", w.Header().Get("Content-Type"))
	}
	if !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Errorf("content-disposition = %q", w.Header().Get("Content-Disposition"))
	}
	if w.Body.String() != "jpeg-bytes" {
		t.Errorf("body = %q", w.Body.String())
	}
}

func TestChannelUnknownRoute(t *testing.T) {
	svc := &fakeChannelService{view: sampleChannelView()}
	tg, cookie := channelFixture(t, svc)
	if w := tg.do(http.MethodGet, "/api/owner-channels/ch-1/nope", cookie, ""); w.Code != http.StatusNotFound {
		t.Fatalf("status %d", w.Code)
	}
}

// đảm bảo JSON channel là object (không phải null) khi không có gì đặc biệt.
func TestChannelWireEmptyCollections(t *testing.T) {
	view := channels.ChannelView{Channel: domain.Channel{
		ID: "ch-2", IdentityState: domain.IdentityIdle,
		CreatedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(0, 0).UTC(),
	}}
	wire, err := json.Marshal(ChannelToWire(view))
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(wire, &body); err != nil {
		t.Fatal(err)
	}
	if body["sources"] == nil || body["alwaysTags"] == nil {
		t.Fatalf("collection rỗng phải là []: %s", wire)
	}
	if body["identity"].(map[string]any)["state"] != "ready" {
		t.Fatalf("identity = %v", body["identity"])
	}
}
