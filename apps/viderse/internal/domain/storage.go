package domain

import "time"

// Khoá settings của mảng Lưu trữ (Cài đặt › Lưu trữ, hợp đồng §23). Mọi thứ
// của thư mục dự án đều là settings JSON — không có bảng riêng.
const (
	// KeyStorageLocation là đường dẫn tuyệt đối của thư mục dự án hiện tại
	// (chuỗi JSON). Thiếu → mặc định <config>/projects (hỏi đáp #10).
	KeyStorageLocation = "storage.location"

	// KeyStoragePrevious là bản cũ giữ lại sau một lần chuyển thư mục kiểu
	// "move" (StoragePrevious): đường dẫn, số byte và hạn tự xoá (7 ngày).
	KeyStoragePrevious = "storage.previous"

	// KeyStorageAltRoots là các thư mục dự án cũ vẫn còn dữ liệu đang dùng sau
	// một lần đổi kiểu "chỉ video mới" — resolver đọc file từ đây khi không
	// thấy ở thư mục hiện tại (dự án cũ vẫn mở được).
	KeyStorageAltRoots = "storage.altRoots"

	// KeyStorageMove là nhật ký một lần chuyển đang dở (StorageMove): nguồn,
	// đích, staging và giai đoạn — khởi động lại phục hồi đúng tới điểm đã
	// ghi, kể cả khi vị trí chưa đổi.
	KeyStorageMove = "storage.move"

	// KeyStorageMoveFailed là lần chuyển hỏng/dở mới nhất (StorageMoveFailed):
	// UI mở lại việc (nút Thử lại) thay vì coi là đang chuyển.
	KeyStorageMoveFailed = "storage.move.lastFailed"

	// KeyStorageCleanAt là mốc lần dọn dữ liệu gần nhất (chuỗi RFC3339),
	// hiện ở "lần cuối 22:14" của mục Lưu trữ.
	KeyStorageCleanAt = "storage.clean.lastRunAt"

	// KeyStorageLastSwitch là lần đổi thư mục gần nhất (StorageSwitch) — màn
	// "đã chuyển xong" và "đã đổi cho video mới" khác nhau nên phải nhớ được
	// sau khi tải lại trang.
	KeyStorageLastSwitch = "storage.lastSwitch"

	// KeyStorageHolds là tập lý do đang giữ việc (StorageHolds), persist để
	// mở lại app sau sập vẫn biết hold nào của storage (không trộn với cờ
	// tạm dừng do người dùng bật).
	KeyStorageHolds = "storage.holds"

	// KeyStoragePaused là cờ tạm dừng do hold của storage bật (boolean).
	// Riêng với clone.queuePaused của người dùng: người đọc lấy hiệu lực
	// = người dùng || storage, không bên nào ghi đè bên kia.
	KeyStoragePaused = "storage.queuePaused"
)

// StoragePrevious là bản cũ giữ 7 ngày sau khi chuyển thư mục (§23
// `previous`): người dùng xoá sớm được, quá hạn thì tự xoá. Mode nói bản cũ
// sinh ra từ kiểu đổi nào — "move" là bản sao an toàn; "new_only" không bao
// giờ bị tự xoá (dữ liệu cũ còn đang đọc) nên không được ghi vào previous.
type StoragePrevious struct {
	Path        string    `json:"path"`
	Bytes       int64     `json:"bytes"`
	DeleteAfter time.Time `json:"deleteAfter"`
	Mode        string    `json:"mode,omitempty"`
}

// StorageSwitch là lần đổi thư mục gần nhất: Mode "move" (đã chuyển dữ liệu)
// hay "new_only" (chỉ video mới; dữ liệu cũ ở lại). At là lúc đổi.
type StorageSwitch struct {
	Mode string    `json:"mode"`
	At   time.Time `json:"at"`
}

// Phase của nhật ký chuyển (StorageMove.Phase): bản bền vững của từng mốc
// dữ liệu nên khởi động lại biết chính xác đã làm tới đâu.
const (
	// MovePhaseCopy: đang chép qua staging — xoá staging là đủ phục hồi.
	MovePhaseCopy = "copy"
	// MovePhaseCommit: dữ liệu đã ở đích (đổi tên xong hoặc commit từng mục
	// xong); chỉ còn đổi vị trí và ghi bản cũ — phục hồi nhận đích luôn.
	MovePhaseCommit = "commit"
)

// StorageMove là nhật ký một lần chuyển đang dở:
//   - copy: staging nằm trong To; sập thì xoá staging, xoá marker — From còn
//     nguyên và vẫn là vị trí. From mất mà To đã đầy đủ (rename cùng ổ) thì
//     phục hồi chuyển sang commit.
//   - commit: dữ liệu đã hoàn chỉnh ở To; phục hồi đổi vị trí + ghi bản cũ
//     giống hệt bước cuối của worker (idempotent).
type StorageMove struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Staging string `json:"staging"`
	Phase   string `json:"phase"`
	TaskID  string `json:"taskId,omitempty"`
}

// StorageMoveFailed là lần chuyển không hoàn tất mới nhất: dữ liệu cũ còn
// nguyên nhưng UI phải biết để mở lại (Thử lại) hoặc gác (Huỷ). At/Phase giữ
// câu tiếng Việt cho màn trạng thái.
type StorageMoveFailed struct {
	TaskID string    `json:"taskId,omitempty"`
	Target string    `json:"target"`
	Phase  string    `json:"phase"`
	At     time.Time `json:"at"`
}

// StorageHolds là tập lý do giữ việc persist của storage (JSON []string).
type StorageHolds []Hold

// KeyStorageServerOffline is the server backend's independently owned
// HoldOffline. It must not overwrite the disk watcher's or user's pause.
const KeyStorageServerOffline = "storage.serverOffline"

// Hold là lý do đang giữ việc làm video (scheduler tương lai hỏi trước khi
// bắt đầu bước mới; hàng chờ DB cũng bị khoá qua storage.queuePaused trong
// khi hold còn hiệu lực).
type Hold string

const (
	// HoldMoving: đang chuyển thư mục dự án — video đang làm tạm dừng, hàng
	// chờ chưa chạy tiếp.
	HoldMoving Hold = "moving"
	// HoldOffline: ổ chứa thư mục dự án biến mất — giữ mọi việc tới khi ổ
	// quay lại.
	HoldOffline Hold = "offline"
	// HoldUseDefault: ổ mất nhưng người dùng chọn "Lưu video mới vào máy này"
	// — video mới ghi vào thư mục mặc định trong máy; truy cập dữ liệu của ổ
	// vẫn chặn tới khi ổ quay lại.
	HoldUseDefault Hold = "use_default"
)
