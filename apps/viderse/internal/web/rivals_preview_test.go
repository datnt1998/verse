// Ca test cho map lỗi Khám Phá: request bị huỷ (đóng tab, hết hạn) không được
// ghi 500 hay ghi nhật ký nội bộ.
package web

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWriteRivalErrorIgnoresCanceledRequest(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelExpired()

	cases := []struct {
		name string
		ctx  context.Context
		err  error
	}{
		{name: "lỗi huỷ", ctx: context.Background(), err: context.Canceled},
		{name: "lỗi hết hạn", ctx: context.Background(), err: context.DeadlineExceeded},
		{name: "ctx đã huỷ", ctx: canceled, err: errors.New("bất kỳ")},
		{name: "ctx hết hạn", ctx: expired, err: errors.New("bất kỳ")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			diagged := false
			writeRivalError(rec, c.ctx, c.err, nil, func(string) { diagged = true })
			if rec.Body.Len() != 0 {
				t.Fatalf("body = %q, muốn rỗng", rec.Body.String())
			}
			if diagged {
				t.Fatal("request bị huỷ không được ghi nhật ký nội bộ")
			}
		})
	}
}
