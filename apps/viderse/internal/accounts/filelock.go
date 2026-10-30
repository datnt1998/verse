package accounts

// Khoá đăng nhập (port withBilibiliLoginLock): một khoá chung cho đăng nhập,
// hỏi mã, gia hạn và kiểm tra — không chạy song song. Mutex trong process đứng
// trước để tuần tự hoá goroutine; khoá tệp flock chặn process khác (CLI chạy
// cùng serve) can thiệp vào cùng tệp đăng nhập. flock tự nhả khi process chết,
// nên không còn khái niệm khoá cũ 5 phút của bản TS.

import (
	"os"

	"github.com/gofrs/flock"
)

// ensureDir tạo thư mục dữ liệu (0700) khi chưa có.
func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0o700)
}

// withLock chạy fn dưới khoá đăng nhập chung: mutex process (TryLock) + khoá
// tệp ~/.viderse/bilibili-cookie.txt.lock. Cả hai đều không chờ: đang bận thì
// trả ReasonBusy ngay thay vì xếp hàng phía sau một lời gọi mạng chậm — 409.
func (s *Service) withLock(fn func() error) error {
	if !s.mu.TryLock() {
		return errBusy
	}
	defer s.mu.Unlock()
	return s.withFileLock(fn)
}

// withFileLock chỉ giữ khoá tệp (caller đã giữ s.mu).
func (s *Service) withFileLock(fn func() error) error {
	if err := ensureDir(s.d.Dir); err != nil {
		return err
	}
	fl := flock.New(s.lockFile())
	ok, err := fl.TryLock()
	if err != nil {
		return err
	}
	if !ok {
		return errBusy
	}
	defer func() { _ = fl.Unlock() }()
	return fn()
}
