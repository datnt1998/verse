// Ca test HTTP cho /api/rivals*: hình wire Rival/Video/ScanStatus/
// PostingSchedule, phân giải route, và map lỗi scan.Error + lỗi đọc sang
// status + code theo hợp đồng.
package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/rivals"
	"github.com/datnt1998/verse/apps/viderse/internal/scan"
)

// fakeRivalReads là RivalReads giả ghi lại lời gọi và trả dữ liệu cài sẵn.
type fakeRivalReads struct {
	err      error
	list     *rivals.ListResult
	entry    *rivals.ListEntry
	page     *rivals.VideoPage
	schedule *domain.PostingSchedule
	preview  *rivals.VideoPreview
	lastID   string
	lastZone string
	lastQ    rivals.VideoQuery
}

func (f *fakeRivalReads) List(context.Context) (*rivals.ListResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.list, nil
}

func (f *fakeRivalReads) Rival(_ context.Context, id string) (*rivals.ListEntry, error) {
	f.lastID = id
	if f.err != nil {
		return nil, f.err
	}
	if f.entry == nil || f.entry.Channel.ID != id {
		return nil, rivals.ErrRivalNotFound
	}
	return f.entry, nil
}

func (f *fakeRivalReads) Videos(_ context.Context, id string, q rivals.VideoQuery) (*rivals.VideoPage, error) {
	f.lastID, f.lastQ = id, q
	if f.err != nil {
		return nil, f.err
	}
	if id != domain.AllRivals && (f.entry == nil || f.entry.Channel.ID != id) {
		return nil, rivals.ErrRivalNotFound
	}
	if q.Cursor == "tampered" {
		return nil, rivals.ErrInvalidCursor
	}
	// Fake giữ kiểm tham số như service thật.
	switch q.Sort {
	case rivals.SortOutlier, rivals.SortNewest, rivals.SortViews:
	default:
		return nil, rivals.ErrInvalidSort
	}
	switch q.WithinDays {
	case 0, 7, 30, 90:
	default:
		return nil, rivals.ErrInvalidWithin
	}
	if q.Limit <= 0 || q.Limit > rivals.VideoPageMax {
		return nil, rivals.ErrInvalidLimit
	}
	if len([]rune(q.Query)) > rivals.QueryMaxRunes {
		return nil, rivals.ErrInvalidQuery
	}
	return f.page, nil
}

func (f *fakeRivalReads) PostingSchedule(_ context.Context, id, zone string) (*domain.PostingSchedule, error) {
	f.lastID, f.lastZone = id, zone
	if f.err != nil {
		return nil, f.err
	}
	if f.schedule == nil || f.schedule.ChannelID != id {
		return nil, rivals.ErrRivalNotFound
	}
	if zone != domain.ZoneVN && zone != domain.ZoneNY {
		return nil, rivals.ErrInvalidZone
	}
	return f.schedule, nil
}

func (f *fakeRivalReads) Preview(_ context.Context, id, videoID string) (*rivals.VideoPreview, error) {
	f.lastID = id
	if f.err != nil {
		return nil, f.err
	}
	if f.preview == nil {
		return nil, rivals.ErrVideoNotFound
	}
	return f.preview, nil
}

// fakeRivalMutations là RivalMutations giả trả scan.Error theo mã cài sẵn.
type fakeRivalMutations struct {
	err     error
	status  domain.ScanStatus
	channel domain.RivalChannel
	lastID  string
	lastIn  string
	lastAct domain.ScanAction
	lastCap int
	removed string
}

func (f *fakeRivalMutations) Add(_ context.Context, input string) (domain.RivalChannel, error) {
	f.lastIn = input
	if f.err != nil {
		return domain.RivalChannel{}, f.err
	}
	return f.channel, nil
}

func (f *fakeRivalMutations) Remove(_ context.Context, id string) error {
	f.removed = id
	return f.err
}

func (f *fakeRivalMutations) Status(_ context.Context, id string) (domain.ScanStatus, error) {
	f.lastID = id
	if f.err != nil {
		return domain.ScanStatus{}, f.err
	}
	return f.status, nil
}

func (f *fakeRivalMutations) Control(_ context.Context, id string, action domain.ScanAction) (domain.ScanStatus, error) {
	f.lastID, f.lastAct = id, action
	if f.err != nil {
		return domain.ScanStatus{}, f.err
	}
	return f.status, nil
}

func (f *fakeRivalMutations) SetCap(_ context.Context, id string, cap int) (domain.ScanStatus, error) {
	f.lastID, f.lastCap = id, cap
	if f.err != nil {
		return domain.ScanStatus{}, f.err
	}
	return f.status, nil
}

func sampleEntry() *rivals.ListEntry {
	scanned := time.Date(2026, 10, 30, 6, 0, 0, 0, time.UTC)
	subs := int64(1240000)
	median := int64(286000)
	return &rivals.ListEntry{
		Channel: domain.RivalChannel{
			ID: "UCnocturne01", Handle: "@nocturnestories", Title: "Nocturne Stories",
			AvatarURL: "https://yt3/avatar.jpg", URL: "https://youtube.com/@nocturnestories",
			SubscriberCount: &subs, MedianViews: &median,
			AddedAt: time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC), ScannedAt: &scanned,
		},
		VideoCount: 1284, Outliers: 11, NewOutliers: 3,
		HistoryState: "running", DiscoveryRunning: true,
	}
}

func mountRivalFixture(t *testing.T) (*testGateway, *fakeRivalReads, *fakeRivalMutations, string) {
	t.Helper()
	tg := newTestGateway(t)
	reads := &fakeRivalReads{
		list:  &rivals.ListResult{Rivals: []rivals.ListEntry{*sampleEntry()}, TotalVideos: 1284, NewOutliers: 3},
		entry: sampleEntry(),
		page: &rivals.VideoPage{Videos: []domain.RivalVideo{{
			ID: "vidid000001", ChannelID: "UCnocturne01", Title: "Đêm khuya",
		}}, Total: 1284},
		schedule: &domain.PostingSchedule{
			ChannelID: "UCnocturne01", Zone: domain.ZoneVN, Sample: 30, Total: 18, Pending: 12,
			Slots: []domain.PostingSlot{{Weekdays: []int{0}, Start: "21:00", End: "22:00", Count: 6, WeeksHit: 6, WeeksTotal: 6, Rhythm: domain.RhythmWeekly, IsTop: true}},
		},
	}
	mutations := &fakeRivalMutations{
		channel: sampleEntry().Channel,
		status: domain.ScanStatus{
			ChannelID: "UCnocturne01", HistoryCap: 2000, IndexedVideos: 1284, RescanHours: 1,
			History:   domain.HistoryStatus{State: "running", JobID: "job-1", Scanned: 1284, Added: 1284, Pages: 7},
			Discovery: domain.DiscoveryStatus{CaughtUp: true},
		},
	}
	MountRivals(tg.g, reads, mutations)
	return tg, reads, mutations, tg.sessionCookie()
}

func TestRivalsListWire(t *testing.T) {
	tg, _, _, cookie := mountRivalFixture(t)
	w := tg.do(http.MethodGet, "/api/rivals", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["totalVideos"].(float64) != 1284 || body["newOutliers"].(float64) != 3 {
		t.Fatalf("totals = %v", body)
	}
	list := body["rivals"].([]any)
	if len(list) != 1 {
		t.Fatalf("rivals = %d", len(list))
	}
	r := list[0].(map[string]any)
	if r["id"] != "UCnocturne01" || r["handle"] != "@nocturnestories" ||
		r["videoCount"].(float64) != 1284 || r["outliers"].(float64) != 11 || r["newOutliers"].(float64) != 3 {
		t.Fatalf("rival = %v", r)
	}
	if r["addedAt"] != "2026-10-20T08:00:00Z" || r["scannedAt"] != "2026-10-30T06:00:00Z" {
		t.Fatalf("times = %v %v", r["addedAt"], r["scannedAt"])
	}
	scanW := r["scan"].(map[string]any)
	if scanW["state"] != "running" || scanW["discoveryRunning"] != true {
		t.Fatalf("scan = %v", scanW)
	}
}

func TestRivalAddWire(t *testing.T) {
	tg, reads, mutations, cookie := mountRivalFixture(t)
	w := tg.do(http.MethodPost, "/api/rivals", cookie, `{"input":"@nocturnestories"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if mutations.lastIn != "@nocturnestories" {
		t.Fatalf("input = %q", mutations.lastIn)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["id"] != "UCnocturne01" {
		t.Fatalf("rival = %v", body)
	}
	// 409 kèm rival đọc lại từ kho — không dựng số đếm giả.
	mutations.err = &scan.Error{Code: scan.CodeExists, Message: "đã theo dõi", Existing: &mutations.channel}
	w = tg.do(http.MethodPost, "/api/rivals", cookie, `{"input":"@nocturnestories"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("exists status %d: %s", w.Code, w.Body.String())
	}
	body = nil
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["code"] != "exists" {
		t.Fatalf("code = %v", body["code"])
	}
	rival := body["rival"].(map[string]any)
	if rival["id"] != "UCnocturne01" || rival["videoCount"].(float64) != 1284 {
		t.Fatalf("rival = %v", rival)
	}
	// Đọc lại hỏng thì đó là lỗi nội bộ — không trả kênh với số 0.
	reads.entry = nil
	w = tg.do(http.MethodPost, "/api/rivals", cookie, `{"input":"@nocturnestories"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("exists + đọc lại hỏng = %d: %s", w.Code, w.Body.String())
	}
	body = nil
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["code"] != "internal" {
		t.Fatalf("code = %v", body["code"])
	}
	if _, ok := body["rival"]; ok {
		t.Fatalf("không được trả rival khi đọc lại hỏng: %v", body)
	}
}

func TestRivalVideosRoute(t *testing.T) {
	tg, reads, _, cookie := mountRivalFixture(t)
	reads.page.NextCursor = "opaque-cursor"
	w := tg.do(http.MethodGet, "/api/rivals/UCnocturne01/videos?sort=views&long=1&within=30&q=đêm&limit=10", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	q := reads.lastQ
	if q.Sort != rivals.SortViews || !q.Long || q.WithinDays != 30 || q.Query != "đêm" || q.Limit != 10 {
		t.Fatalf("query = %+v", q)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["total"].(float64) != 1284 || body["nextCursor"] != "opaque-cursor" {
		t.Fatalf("page = %v", body)
	}
	v := body["videos"].([]any)[0].(map[string]any)
	if v["id"] != "vidid000001" || v["thumbnailUrl"] != "https://i.ytimg.com/vi/vidid000001/mqdefault.jpg" ||
		v["url"] != "https://www.youtube.com/watch?v=vidid000001" || v["exactTime"] != false {
		t.Fatalf("video = %v", v)
	}
	// all: không kiểm tồn tại kênh.
	reads.entry = nil
	w = tg.do(http.MethodGet, "/api/rivals/all/videos", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("all status %d: %s", w.Code, w.Body.String())
	}
	// Mặc định sort=outlier limit=60.
	if reads.lastQ.Sort != rivals.SortOutlier || reads.lastQ.Limit != rivals.VideoPageDefault {
		t.Fatalf("default query = %+v", reads.lastQ)
	}
}

func TestRivalVideosInvalidParams(t *testing.T) {
	tg, _, _, cookie := mountRivalFixture(t)
	for _, tc := range []struct {
		path string
		code string
	}{
		{"/api/rivals/UCnocturne01/videos?sort=magic", "invalid_input"},
		{"/api/rivals/UCnocturne01/videos?within=14", "invalid_input"},
		{"/api/rivals/UCnocturne01/videos?limit=999", "invalid_input"},
		{"/api/rivals/UCnocturne01/videos?limit=abc", "invalid_input"},
		{"/api/rivals/UCnocturne01/videos?within=abc", "invalid_input"},
		{"/api/rivals/UCnocturne01/videos?cursor=tampered", "invalid_cursor"},
		{"/api/rivals/UCnocturne01/videos?long=banana", "invalid_input"},
		{"/api/rivals/UCnocturne01/videos?long=2", "invalid_input"},
		{"/api/rivals/UCnocturne01/videos?long=true", "invalid_input"},
	} {
		w := tg.do(http.MethodGet, tc.path, cookie, "")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s status %d", tc.path, w.Code)
		}
		var body map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if body["code"] != tc.code {
			t.Fatalf("%s code = %v", tc.path, body["code"])
		}
	}
}

func TestRivalUnknownGoesNotFound(t *testing.T) {
	tg, reads, mutations, cookie := mountRivalFixture(t)
	mutations.err = &scan.Error{Code: scan.CodeNotFound, Message: "Không tìm thấy kênh đối thủ"}
	reads.entry = nil

	for _, tc := range []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/rivals/UClạ/videos", ""},
		{http.MethodGet, "/api/rivals/UClạ/scan", ""},
		{http.MethodPost, "/api/rivals/UClạ/scan", `{"action":"check"}`},
		{http.MethodPatch, "/api/rivals/UClạ/scan", `{"historyCap":2000}`},
		{http.MethodDelete, "/api/rivals/UClạ", ""},
		{http.MethodGet, "/api/rivals/UClạ/posting-slots", ""},
	} {
		w := tg.do(tc.method, tc.path, cookie, tc.body)
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s %s status %d", tc.method, tc.path, w.Code)
		}
		var body map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if body["code"] != "not_found" {
			t.Fatalf("%s %s code = %v", tc.method, tc.path, body["code"])
		}
	}
}

func TestRivalScanStatusAndControl(t *testing.T) {
	tg, _, mutations, cookie := mountRivalFixture(t)
	w := tg.do(http.MethodGet, "/api/rivals/UCnocturne01/scan", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["channelId"] != "UCnocturne01" || body["historyCap"].(float64) != 2000 || body["indexedVideos"].(float64) != 1284 {
		t.Fatalf("status = %v", body)
	}
	hist := body["history"].(map[string]any)
	if hist["state"] != "running" || hist["jobId"] != "job-1" || hist["scanned"].(float64) != 1284 {
		t.Fatalf("history = %v", hist)
	}

	w = tg.do(http.MethodPost, "/api/rivals/UCnocturne01/scan", cookie, `{"action":"check"}`)
	if w.Code != http.StatusOK || mutations.lastAct != domain.ScanActionCheck {
		t.Fatalf("check = %d %v", w.Code, mutations.lastAct)
	}
	w = tg.do(http.MethodPost, "/api/rivals/UCnocturne01/scan", cookie, `{"action":"start"}`)
	if w.Code != http.StatusOK || mutations.lastAct != domain.ScanActionStart {
		t.Fatalf("start = %d", w.Code)
	}
	w = tg.do(http.MethodPost, "/api/rivals/UCnocturne01/scan", cookie, `{"action":"weird"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("action lạ = %d", w.Code)
	}
	var badBody map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &badBody)
	if badBody["code"] != "invalid_input" {
		t.Fatalf("action lạ code = %v", badBody)
	}
}

func TestRivalScanCapValidation(t *testing.T) {
	tg, _, mutations, cookie := mountRivalFixture(t)
	w := tg.do(http.MethodPatch, "/api/rivals/UCnocturne01/scan", cookie, `{"historyCap":500}`)
	if w.Code != http.StatusOK || mutations.lastCap != 500 {
		t.Fatalf("cap = %d %v", w.Code, mutations.lastCap)
	}
	mutations.err = &scan.Error{Code: scan.CodeInvalidCap, Message: "cap phải từ 150 tới 10000"}
	w = tg.do(http.MethodPatch, "/api/rivals/UCnocturne01/scan", cookie, `{"historyCap":10}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("cap lạ = %d", w.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["code"] != "invalid_cap" {
		t.Fatalf("code = %v", body["code"])
	}
}

func TestRivalRemove(t *testing.T) {
	tg, _, mutations, cookie := mountRivalFixture(t)
	w := tg.do(http.MethodDelete, "/api/rivals/UCnocturne01", cookie, "")
	if w.Code != http.StatusNoContent || mutations.removed != "UCnocturne01" {
		t.Fatalf("remove = %d %q", w.Code, mutations.removed)
	}
}

func TestRivalPostingSlots(t *testing.T) {
	tg, reads, _, cookie := mountRivalFixture(t)
	w := tg.do(http.MethodGet, "/api/rivals/UCnocturne01/posting-slots?zone=ny", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if reads.lastZone != domain.ZoneNY {
		t.Fatalf("zone = %q", reads.lastZone)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["channelId"] != "UCnocturne01" || body["sample"].(float64) != 30 ||
		body["total"].(float64) != 18 || body["pending"].(float64) != 12 {
		t.Fatalf("schedule = %v", body)
	}
	slots := body["slots"].([]any)
	if len(slots) != 1 {
		t.Fatalf("slots = %v", slots)
	}
	slot := slots[0].(map[string]any)
	if slot["start"] != "21:00" || slot["end"] != "22:00" || slot["rhythm"] != "weekly" || slot["isTop"] != true {
		t.Fatalf("slot = %v", slot)
	}
	grid := body["grid"].([]any)
	if len(grid) != 7 || len(grid[0].([]any)) != 12 {
		t.Fatalf("grid = %v", grid)
	}
	// Zone mặc định vn; zone lạ → invalid_input.
	w = tg.do(http.MethodGet, "/api/rivals/UCnocturne01/posting-slots", cookie, "")
	if w.Code != http.StatusOK || reads.lastZone != domain.ZoneVN {
		t.Fatalf("default zone = %d %q", w.Code, reads.lastZone)
	}
	w = tg.do(http.MethodGet, "/api/rivals/UCnocturne01/posting-slots?zone=berlin", cookie, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("zone lạ = %d", w.Code)
	}
}

func TestRivalErrorMappingForEveryScanCode(t *testing.T) {
	tg, _, mutations, cookie := mountRivalFixture(t)
	retry := time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC)
	cases := []struct {
		code      string
		status    int
		recovery  string
		retryAt   *time.Time
		wantRetry bool
	}{
		{scan.CodeInvalidInput, http.StatusBadRequest, "", nil, false},
		{scan.CodeNotFound, http.StatusNotFound, "", nil, false},
		{scan.CodeExists, http.StatusConflict, "", nil, false},
		{scan.CodeRateLimited, http.StatusTooManyRequests, "", &retry, true},
		{scan.CodeBlocked, http.StatusServiceUnavailable, "đổi tài khoản", nil, false},
		{scan.CodeUnexpectedPage, http.StatusBadGateway, "dữ liệu không đọc được", nil, false},
		{scan.CodeTransient, http.StatusBadGateway, "", nil, false},
		{scan.CodeInvalidCap, http.StatusBadRequest, "", nil, false},
		{scan.CodeLeaseBusy, http.StatusServiceUnavailable, "", &retry, true},
	}
	for _, c := range cases {
		mutations.err = &scan.Error{Code: c.code, Message: "lỗi " + c.code, Recovery: c.recovery, RetryAt: c.retryAt}
		w := tg.do(http.MethodPost, "/api/rivals/UCnocturne01/scan", cookie, `{"action":"check"}`)
		if w.Code != c.status {
			t.Fatalf("%s status %d muốn %d (%s)", c.code, w.Code, c.status, w.Body.String())
		}
		var body map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if body["code"] != c.code {
			t.Fatalf("%s code = %v", c.code, body["code"])
		}
		if c.recovery != "" && body["recovery"] != c.recovery {
			t.Fatalf("%s recovery = %v", c.code, body["recovery"])
		}
		if c.wantRetry && body["retryAt"] != "2026-10-30T14:00:00Z" {
			t.Fatalf("%s retryAt = %v", c.code, body["retryAt"])
		}
	}
}

func TestRivalInternalErrorNeutralEnvelope(t *testing.T) {
	reads := &fakeRivalReads{err: errors.New(`postgres://u:p@db/viderse secret-token`)}
	var diagged string
	tg := newTestGateway(t)
	MountRivalsDiag(tg.g, reads, &fakeRivalMutations{}, func(msg string) { diagged = msg })
	cookie := tg.sessionCookie()

	w := tg.do(http.MethodGet, "/api/rivals", cookie, "")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("list 500 = %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body không phải JSON: %q", w.Body.String())
	}
	// Vỏ trung lập có mã ổn định, thông điệp trung lập.
	if body["code"] != "internal" || body["error"] != "lỗi nội bộ" {
		t.Fatalf("body = %v", body)
	}
	// Nguyên nhân (kể cả DSN/bí mật) không được xuất hiện ở BẤT KỲ vị trí nào
	// trong body — kiểm chuỗi con chứ không so cả body.
	for _, secret := range []string{"postgres://", "secret-token", "u:p@db"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatalf("lộ %q trong body: %q", secret, w.Body.String())
		}
	}
	if diagged == "" || !strings.Contains(diagged, "secret-token") {
		t.Fatalf("diag phải nhận nguyên nhân, được %q", diagged)
	}
}
