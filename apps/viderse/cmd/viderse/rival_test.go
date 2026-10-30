package main

// Ca test của `viderse rival …`: parse cờ và vỏ lỗi JSON — các đường đi không
// chạm DB (kiểm tham số trước khi mở pool). Đường đi cần DB/YouTube nằm ở
// smoke của luồng quét.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/rivals"
	"github.com/datnt1998/verse/apps/viderse/internal/scan"
)

func decodeErrBody(t *testing.T, stderr *bytes.Buffer) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(stderr.Bytes()), &body); err != nil {
		t.Fatalf("stderr không phải JSON: %q", stderr.String())
	}
	return body
}

func TestRivalHelpIsHumanText(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := runRivalCommand([]string{"--help"}, &out, &errBuf); code != 0 || !strings.Contains(out.String(), "rival add") {
		t.Fatalf("help = %d %q", code, out.String())
	}
	// Thiếu lệnh con là lỗi máy đọc: JSON + exit 1.
	out.Reset()
	errBuf.Reset()
	if code := runRivalCommand(nil, &out, &errBuf); code != 1 {
		t.Fatalf("thiếu lệnh = %d", code)
	}
	body := decodeErrBody(t, &errBuf)
	if body["code"] != scan.CodeInvalidInput || !strings.Contains(body["error"].(string), "thiếu lệnh con") {
		t.Fatalf("body = %v", body)
	}
	errBuf.Reset()
	if code := runRivalCommand([]string{"lạ"}, &out, &errBuf); code != 1 {
		t.Fatalf("lệnh lạ = %d", code)
	}
	if body := decodeErrBody(t, &errBuf); body["code"] != scan.CodeInvalidInput {
		t.Fatalf("lệnh lạ body = %v", body)
	}
}

func TestRivalFlagValidation(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"cap thiếu số", []string{"scan", "UCx", "--cap"}, "--cap cần một số"},
		{"cap thấp", []string{"scan", "UCx", "--cap", "10"}, "--cap phải là số nguyên từ 150 tới 10000"},
		{"cap cao", []string{"scan", "UCx", "--cap", "20000"}, "--cap phải là số nguyên từ 150 tới 10000"},
		{"cap không số", []string{"scan", "UCx", "--cap", "nhiều"}, "--cap phải là số nguyên từ 150 tới 10000"},
		{"action lạ", []string{"scan", "UCx", "--action", "start"}, "--action chỉ nhận pause|resume|cancel"},
		{"action + history", []string{"scan", "UCx", "--history", "--action", "pause"}, "--action không đi cùng --history"},
		{"sort lạ", []string{"videos", "UCx", "--sort", "hot"}, "--sort phải là outlier|newest|views"},
		{"limit âm", []string{"videos", "UCx", "--limit", "-3"}, "--limit phải là số nguyên từ 1 tới 200"},
		{"limit quá lớn", []string{"videos", "UCx", "--limit", "201"}, "--limit phải là số nguyên từ 1 tới 200"},
		{"zone lạ", []string{"posting-slots", "UCx", "--zone", "tokyo"}, "--zone phải là vn|ny"},
		{"flag lạ", []string{"videos", "UCx", "--weird"}, "tham số không hợp lệ"},
	}
	for _, c := range cases {
		var out, errBuf bytes.Buffer
		code := runRivalCommand(c.args, &out, &errBuf)
		if code != 1 {
			t.Fatalf("%s: exit = %d", c.name, code)
		}
		body := decodeErrBody(t, &errBuf)
		if body["code"] != scan.CodeInvalidInput {
			t.Fatalf("%s: code = %v", c.name, body["code"])
		}
		if !strings.Contains(body["error"].(string), c.wantErr) {
			t.Fatalf("%s: error = %v muốn chứa %q", c.name, body["error"], c.wantErr)
		}
	}
}

func TestRivalErrOutMapsScanError(t *testing.T) {
	retry := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	var errBuf bytes.Buffer
	code := rivalErrOut(&errBuf, &scan.Error{
		Code:     scan.CodeRateLimited,
		Message:  "YouTube giới hạn tốc độ",
		Recovery: "thử lại sau",
		RetryAt:  &retry,
	})
	if code != 1 {
		t.Fatalf("exit = %d", code)
	}
	body := decodeErrBody(t, &errBuf)
	if body["code"] != "rate_limited" || body["recovery"] != "thử lại sau" || body["retryAt"] != "2026-10-30T12:00:00Z" {
		t.Fatalf("body = %v", body)
	}
}

func TestRivalErrOutMapsReadErrors(t *testing.T) {
	cases := []struct {
		err  error
		code string
	}{
		{rivals.ErrRivalNotFound, "not_found"},
		{rivals.ErrInvalidCursor, "invalid_cursor"},
		{rivals.ErrInvalidSort, "invalid_input"},
		{rivals.ErrInvalidQuery, "invalid_input"},
		{errors.New("lỗi lạ không mã"), ""},
	}
	for _, c := range cases {
		var errBuf bytes.Buffer
		rivalErrOut(&errBuf, c.err)
		body := decodeErrBody(t, &errBuf)
		if c.code == "" {
			if _, ok := body["code"]; ok {
				t.Fatalf("%v: code không mong đợi = %v", c.err, body["code"])
			}
			continue
		}
		if body["code"] != c.code {
			t.Fatalf("%v: code = %v muốn %s", c.err, body["code"], c.code)
		}
	}
}

// fakeAddMutations là rivalMutations tối thiểu cho đường add: chỉ Add được
// gọi; các phương thức còn lại trả lỗi để test không vô tình đi nhánh khác.
type fakeAddMutations struct {
	channel domain.RivalChannel
	err     error
}

func (f fakeAddMutations) Add(context.Context, string) (domain.RivalChannel, error) {
	if f.err != nil {
		return domain.RivalChannel{}, f.err
	}
	return f.channel, nil
}

func (fakeAddMutations) Remove(context.Context, string) error { return errors.New("không dùng") }

func (fakeAddMutations) Status(context.Context, string) (domain.ScanStatus, error) {
	return domain.ScanStatus{}, errors.New("không dùng")
}

func (fakeAddMutations) Control(context.Context, string, domain.ScanAction) (domain.ScanStatus, error) {
	return domain.ScanStatus{}, errors.New("không dùng")
}

func (fakeAddMutations) SetCap(context.Context, string, int) (domain.ScanStatus, error) {
	return domain.ScanStatus{}, errors.New("không dùng")
}

func (fakeAddMutations) RunInline(context.Context, string) (bool, error) {
	return false, errors.New("không dùng")
}

// fakeRivalReads trả dòng danh sách cài sẵn cho Rival.
type fakeRivalReads struct {
	entry *rivals.ListEntry
	last  string
}

func (f *fakeRivalReads) Rival(_ context.Context, id string) (*rivals.ListEntry, error) {
	f.last = id
	if f.entry == nil {
		return nil, rivals.ErrRivalNotFound
	}
	return f.entry, nil
}

// TestRivalAddPrintsStoredProjection khoá hình JSON của `rival add` = hình
// Rival của HTTP lấy từ kho, không phải số 0 giả: kênh có sẵn baseline +
// outlier + đã quét hết phải in đúng số đếm và trạng thái đó.
func TestRivalAddPrintsStoredProjection(t *testing.T) {
	scanned := time.Date(2026, 10, 30, 6, 0, 0, 0, time.UTC)
	subs := int64(1240000)
	entry := &rivals.ListEntry{
		Channel: domain.RivalChannel{
			ID: "UCnocturne01", Handle: "@nocturne", Title: "Nocturne", URL: "https://youtube.com/@nocturne",
			SubscriberCount: &subs, AddedAt: time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC), ScannedAt: &scanned,
		},
		VideoCount: 57, Outliers: 4, NewOutliers: 1,
		HistoryState: "complete", DiscoveryRunning: false,
	}
	mutations := fakeAddMutations{channel: entry.Channel}
	reads := &fakeRivalReads{entry: entry}
	var out, errBuf bytes.Buffer
	if code := runRivalAdd(context.Background(), "@nocturne", mutations, reads, &out, &errBuf); code != 0 {
		t.Fatalf("add = %d: %s", code, errBuf.String())
	}
	if reads.last != "UCnocturne01" {
		t.Fatalf("đọc lại id %q", reads.last)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("stdout không phải JSON: %q", out.String())
	}
	if got["videoCount"].(float64) != 57 || got["outliers"].(float64) != 4 || got["newOutliers"].(float64) != 1 {
		t.Fatalf("số đếm = %v", got)
	}
	if got["scannedAt"] != "2026-10-30T06:00:00Z" {
		t.Fatalf("scannedAt = %v", got["scannedAt"])
	}
	if scanWire := got["scan"].(map[string]any); scanWire["state"] != "complete" {
		t.Fatalf("scan = %v", scanWire)
	}
	// Đọc lại hỏng → lỗi JSON, không in kênh thô.
	reads.entry = nil
	out.Reset()
	errBuf.Reset()
	if code := runRivalAdd(context.Background(), "@nocturne", mutations, reads, &out, &errBuf); code != 1 {
		t.Fatalf("đọc lại hỏng = %d", code)
	}
	if out.Len() != 0 {
		t.Fatalf("không được in kênh thô: %q", out.String())
	}
	if body := decodeErrBody(t, &errBuf); body["code"] != "not_found" {
		t.Fatalf("body = %v", body)
	}
}

// TestScanRunIntent: chỉ --history/start, --action resume hoặc mặc định
// không cờ nào mới chạy worker inline — chỉ --cap, pause và cancel không.
func TestScanRunIntent(t *testing.T) {
	cases := []struct {
		name         string
		history, cap bool
		action       domain.ScanAction
		want         bool
	}{
		{"không cờ nào (check)", false, false, "", true},
		{"--history", true, false, "", true},
		{"--history --cap", true, true, "", true},
		{"--action resume", false, false, domain.ScanActionResume, true},
		{"--cap --action resume", false, true, domain.ScanActionResume, true},
		{"chỉ --cap", false, true, "", false},
		{"--action pause", false, false, domain.ScanActionPause, false},
		{"--cap --action pause", false, true, domain.ScanActionPause, false},
		{"--action cancel", false, false, domain.ScanActionCancel, false},
	}
	for _, c := range cases {
		if got := scanRunIntent(c.history, c.cap, c.action); got != c.want {
			t.Fatalf("%s: scanRunIntent(%v,%v,%q) = %v muốn %v", c.name, c.history, c.cap, c.action, got, c.want)
		}
	}
}
