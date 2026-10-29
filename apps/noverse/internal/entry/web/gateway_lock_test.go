package web

import (
	"errors"
	"fmt"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/host"
)

// Worker 启动失败时 stderr 尾部带着 host.ErrBookInUse 的文案 → 必须归为 errBookLocked（409），
// 其它失败原样返回。匹配与生产者共用同一常量，改词不会让识别静默失效。
func TestWorkerStartErrorRecognizesBookLock(t *testing.T) {
	stderr := "level=INFO msg=boot\n" + fmt.Errorf("%w: /tmp/truyen; hãy đóng terminal khác", host.ErrBookInUse).Error() + "\n"
	err := workerStartError(stderr)
	if !errors.Is(err, errBookLocked) {
		t.Fatalf("workerStartError(lock) = %v, want errBookLocked", err)
	}

	other := workerStartError("panic: cấu hình hỏng")
	if errors.Is(other, errBookLocked) || other.Error() != "panic: cấu hình hỏng" {
		t.Fatalf("workerStartError(other) = %v, want plain message", other)
	}
	if empty := workerStartError("  \n"); empty.Error() != "worker không khởi động được" {
		t.Fatalf("workerStartError(empty) = %v", empty)
	}
}
