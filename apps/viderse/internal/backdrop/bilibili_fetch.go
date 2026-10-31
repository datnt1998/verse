package backdrop

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"
)

// riskControlCodes là các mã JSON Bilibili nghĩa là bị chặn/risk-control.
var riskControlCodes = map[int]bool{-352: true, -412: true, -799: true, -401: true, -403: true}

// pacedBilibiliDoer bọc một doer HTTP để mọi lời gọi (kể cả đọc chuyển hướng
// b23.tv) dùng chung nhịp và giờ chờ của Bilibili, và để phản hồi risk-control
// thành lỗi bị giới hạn thay vì dữ liệu rác cho caller.
type pacedBilibiliDoer struct {
	base    HTTPDoer
	request func(ctx context.Context, resetBackoff bool, fn func(context.Context) error) error
	now     func() time.Time
}

func (d *pacedBilibiliDoer) Do(req *http.Request) (*http.Response, error) {
	var response *http.Response
	err := d.request(req.Context(), false, func(ctx context.Context) error {
		resp, err := d.base.Do(req)
		if err != nil {
			return err
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}
		resp.Body = io.NopCloser(bytes.NewReader(body))
		if resp.StatusCode == http.StatusPreconditionFailed || resp.StatusCode == http.StatusTooManyRequests {
			return &RateLimitError{Message: "Bilibili đang giới hạn yêu cầu", Retry: parseRetryAfter(resp.Header.Get("Retry-After"), d.now())}
		}
		var envelope struct {
			Code int `json:"code"`
		}
		if json.Unmarshal(body, &envelope) == nil && riskControlCodes[envelope.Code] {
			return &RateLimitError{Message: "Bilibili đang giới hạn yêu cầu", Retry: parseRetryAfter(resp.Header.Get("Retry-After"), d.now())}
		}
		response = resp
		return nil
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

// Location đọc đúng MỘT lần chuyển hướng b23.tv: client riêng cấm theo
// redirect ở tầng client, deadline 10 giây, chỉ đọc header — không đọc body
// và không bao giờ chạm địa chỉ đích (không request tới bilibili.com).
//
// QUAN TRỌNG: đường parse không đi qua `d.request`/pacer. Đọc Location là
// một lời gọi đọc đơn lẻ do người dùng chủ động kích hoạt; thất bại của nó
// (kể cả 412/429) là kết quả trả về cho UI, không được ghi vào trạng thái
// chặn của nền tảng — ngược lại một link rút gọn lỗi sẽ vô cớ khoá toàn bộ
// nền tảng hàng giờ. `RateLimitError` vẫn được trả để hiện "đang giới hạn"
// nhưng không ai ghi pause từ nó.
func (d *pacedBilibiliDoer) Location(ctx context.Context, u *url.URL) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, b23Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{
		Timeout:   b23Timeout,
		Transport: transportOf(d.base),
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close() // chỉ đọc header Location, không đọc body
	if resp.StatusCode == http.StatusPreconditionFailed || resp.StatusCode == http.StatusTooManyRequests {
		return "", &RateLimitError{Message: "Bilibili đang giới hạn yêu cầu", Retry: parseRetryAfter(resp.Header.Get("Retry-After"), d.now())}
	}
	return resp.Header.Get("Location"), nil
}

func NewBilibiliDoer(base HTTPDoer, request func(ctx context.Context, resetBackoff bool, fn func(context.Context) error) error, now func() time.Time) HTTPDoer {
	return &pacedBilibiliDoer{base: base, request: request, now: now}
}
