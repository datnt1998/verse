// Package status tổng hợp trạng thái đầu trang của Viderse cho GET /api/status:
// số video đang chạy/chờ, cờ tạm dừng hàng chờ, số mục chờ duyệt và các mục
// cần xử lý.
//
// Mỗi tính năng đăng ký một Source (interface nhỏ đặt ở phía dùng); service
// cộng các Report khi render. Mọi thay đổi đi qua revision tăng dần: caller gọi
// Bump khi dữ liệu bên dưới đổi, service phát sự kiện "status" mang rev mới qua
// publish để client SSE vô hiệu cache — và ETag của /api/status chính là rev.
package status

import (
	"context"
	"log/slog"
	"sync"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Source là một nguồn số liệu trạng thái do tính năng đăng ký (queue, review,
// tools…). Report được gọi mỗi lần render snapshot, trên ctx của request.
type Source interface {
	Report(ctx context.Context) (Report, error)
}

// Report là phần đóng góp của một nguồn vào snapshot tổng.
type Report struct {
	Running     int
	Queued      int
	ReviewCount int
	Attention   []Attention
}

// Attention là một mục cần xử lý (chấm trên bánh răng); Kind thuộc tập cố định
// domain.Attention*. Kiểu thuần domain — tầng web tự map sang wire DTO của nó.
type Attention struct {
	Kind domain.AttentionKind
	Text string
	Href string
}

// Settings là phần store status cần: đọc cờ tạm dừng hàng chờ đã lưu.
type Settings interface {
	QueuePaused(ctx context.Context) (bool, error)
}

// Snapshot là hình dữ liệu đầy đủ của trạng thái đầu trang kèm revision. Kiểu
// thuần domain — tầng web map sang wire DTO khi trả GET /api/status.
type Snapshot struct {
	Rev         uint64
	Running     int
	Queued      int
	Paused      bool
	ReviewCount int
	Attention   []Attention
}

// Service giữ revision và danh sách nguồn; tất cả phương thức an toàn cho
// goroutine.
type Service struct {
	store     Settings
	publish   func(rev uint64)
	mu        sync.RWMutex
	rev       uint64
	sources   []Source
	sourceErr func(name string, err error) // điểm log khi một nguồn lỗi
}

// New dựng service. publish được gọi với rev mới sau mỗi Bump (nil = không
// phát sự kiện — dùng trong test).
func New(store Settings, publish func(rev uint64)) *Service {
	if publish == nil {
		publish = func(uint64) {}
	}
	return &Service{
		store:   store,
		publish: publish,
		sourceErr: func(name string, err error) {
			slog.Warn("nguồn trạng thái lỗi", "module", "status", "source", name, "err", err)
		},
	}
}

// RegisterSource thêm một nguồn số liệu; gọi ở bootstrap theo thứ tự ổn định.
func (s *Service) RegisterSource(src Source) {
	s.mu.Lock()
	s.sources = append(s.sources, src)
	s.mu.Unlock()
}

// Rev trả revision hiện tại.
func (s *Service) Rev() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rev
}

// Bump tăng revision và phát sự kiện status cho subscriber SSE. Caller gọi khi
// dữ liệu mà Report đọc đã đổi — service không tự dò thay đổi.
func (s *Service) Bump() uint64 {
	s.mu.Lock()
	s.rev++
	rev := s.rev
	s.mu.Unlock()
	s.publish(rev)
	return rev
}

// Snapshot render hình trạng thái hiện tại: cộng Report của mọi nguồn, đọc cờ
// paused từ settings. Một nguồn lỗi chỉ bị bỏ qua (đã log) — trang thái không
// vì một nguồn hỏng mà mất toàn bộ số liệu; lỗi đọc settings trả ra ngoài.
func (s *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	s.mu.RLock()
	rev := s.rev
	sources := append([]Source{}, s.sources...)
	s.mu.RUnlock()

	snap := Snapshot{Rev: rev, Attention: []Attention{}}
	for _, src := range sources {
		rep, err := src.Report(ctx)
		if err != nil {
			s.sourceErr(sourceName(src), err)
			continue
		}
		snap.Running += rep.Running
		snap.Queued += rep.Queued
		snap.ReviewCount += rep.ReviewCount
		snap.Attention = append(snap.Attention, rep.Attention...)
	}
	paused, err := s.store.QueuePaused(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	snap.Paused = paused
	return snap, nil
}

func sourceName(src Source) string {
	type named interface{ Name() string }
	if n, ok := src.(named); ok {
		return n.Name()
	}
	return "source"
}
