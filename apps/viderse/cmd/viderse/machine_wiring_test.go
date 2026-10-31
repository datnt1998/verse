// Ca test nhỏ cho tệp phụ "platform-requests.json" của gói Báo lỗi: dựng
// đúng hình JSON, bỏ qua khi không có bộ đọc, và giữ nguyên lỗi đọc.
package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/logs"
)

func TestPlatformRequestsExtra(t *testing.T) {
	first := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	last := first.Add(7 * time.Minute)
	extra := platformRequestsExtra(func(context.Context) ([]backdrop.PlatformRequestRecord, error) {
		return []backdrop.PlatformRequestRecord{
			{Host: "douyin.com", Label: "Douyin", Count: 3, FirstAt: first, LastAt: last},
			{Host: "tiktok.com", Label: "TikTok", Count: 1, FirstAt: first, LastAt: first},
		}, nil
	})
	name, data, err := extra(context.Background())
	if err != nil {
		t.Fatalf("extra: %v", err)
	}
	if name != "platform-requests.json" {
		t.Fatalf("tên tệp = %q", name)
	}
	var body struct {
		Requests []struct {
			Host    string `json:"host"`
			Label   string `json:"label"`
			Count   int    `json:"count"`
			FirstAt string `json:"firstAt"`
			LastAt  string `json:"lastAt"`
		} `json:"requests"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("JSON: %v (%s)", err, data)
	}
	if len(body.Requests) != 2 {
		t.Fatalf("requests = %+v", body.Requests)
	}
	row := body.Requests[0]
	if row.Host != "douyin.com" || row.Label != "Douyin" || row.Count != 3 {
		t.Fatalf("dòng đầu = %+v", row)
	}
	if row.FirstAt != "2026-10-29T08:00:00Z" || row.LastAt != "2026-10-29T08:07:00Z" {
		t.Fatalf("mốc thời gian = %+v", row)
	}

	// Không có bộ đọc (pool chưa sẵn sàng): gói bỏ qua tệp, không lỗi.
	if name, data, err := platformRequestsExtra(nil)(context.Background()); err != nil || name != "" || data != nil {
		t.Fatalf("nil: name=%q data=%v err=%v", name, data, err)
	}

	want := errors.New("mất kết nối")
	if _, _, err := platformRequestsExtra(func(context.Context) ([]backdrop.PlatformRequestRecord, error) {
		return nil, want
	})(context.Background()); !errors.Is(err, want) {
		t.Fatalf("lỗi đọc phải giữ nguyên, được %v", err)
	}

	// Gói báo lỗi thật (zip trong bộ nhớ) chứa tệp phụ này.
	store := logs.New(t.TempDir(), t.TempDir())
	bundle, err := store.BuildBundle(context.Background(), logs.BundleOptions{
		Version: "dev",
		Extras:  []logs.ExtraFile{extra},
	})
	if err != nil {
		t.Fatalf("dựng gói: %v", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(bundle.Bytes), int64(len(bundle.Bytes)))
	if err != nil {
		t.Fatalf("mở zip: %v", err)
	}
	found := false
	for _, file := range reader.File {
		if file.Name != "platform-requests.json" {
			continue
		}
		found = true
		rc, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(raw, []byte("douyin.com")) {
			t.Fatalf("tệp phụ = %s", raw)
		}
	}
	if !found {
		t.Fatal("gói thiếu platform-requests.json")
	}
}
