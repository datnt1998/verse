package web

// Một chính sách chuyển lỗi sang HTTP duy nhất cho mọi boundary của gói web.
//
// Chính sách (được liệt kê đầy đủ, không suy đoán thêm):
//
//	dữ liệu không hợp lệ                     → 400
//	chưa đăng nhập / phiên hết hạn            → 401
//	thiếu quyền trên tài nguyên có quyền đọc  → 403
//	không thấy trong phạm vi đã resolve       → 404 (tầng repository quyết định)
//	xung đột phiên bản / trùng                → 409
//	backend không hỗ trợ capability           → 501
//	phụ thuộc tạm thời không sẵn sàng         → 503
//	khác                                      → 500 câu chung tiếng Việt
//
// Câu chung của 500 là hằng duy nhất: nguyên nhân (SQL/table/constraint, host,
// đường dẫn, DSN) chỉ được ghi ở log phía máy chủ, không bao giờ nằm trong body.
// Handler giữ envelope riêng của miền (ví dụ `needsConfig`) nhưng không tự đặt
// text của một cause mà nó không sở hữu.
//
// L2 (giữ nguyên, không đổi): route guard ngoài cùng vẫn trả 403 cho cả dự án
// không tồn tại và không có quyền; tầng repository trả 404 cho truy vấn chéo
// tenant. Chính sách này chỉ chuẩn hoá adapter, không đổi hợp đồng đó.

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/puddle/v2"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/comments"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/platform/identity"
)

// errInternalMessage là body duy nhất của một 500 từ boundary này.
const errInternalMessage = "Máy chủ gặp lỗi, vui lòng thử lại sau."

// Worker/gateway failures: the upstream cause (child process, dial error, port)
// is logged, while the client gets a stable sentence.
const (
	errWorkerUnavailableMessage = "Máy viết của truyện chưa sẵn sàng, vui lòng thử lại."
	errWorkerUnreachableMessage = "Máy viết của truyện không phản hồi, vui lòng thử lại."
)

// httpErrorRule maps one sentinel (or sentinel family) to its transport status
// and safe Vietnamese text. `ownText` means the sentinel's own message is a
// curated Vietnamese sentence that is safe to show (validation detail); it must
// never be set for a shared storage sentinel, whose chain can carry SQL text,
// constraint/table names or slugs.
type httpErrorRule struct {
	err     error
	status  int
	message string
	ownText bool
}

// Order matters: the shared storage family is checked first so a wrapped cause
// never wins over its category. Domain sentinels follow.
var httpErrorRules = []httpErrorRule{
	// Client-visible cancellation/timeout is a transport fact, not a server fault.
	{context.Canceled, 499, "Đã huỷ", false},
	{context.DeadlineExceeded, http.StatusGatewayTimeout, "Quá thời gian chờ. Thử lại sau.", false},

	// Shared storage contract (file + postgres backends). Fixed text only:
	// err.Error() may embed SQLSTATE, table/constraint names, hosts or slugs.
	{errs.ErrStorageNotFound, http.StatusNotFound, "Không tìm thấy.", false},
	{errs.ErrStorageForbidden, http.StatusForbidden, "Bạn không có quyền thực hiện thao tác này.", false},
	{errs.ErrStorageConflict, http.StatusConflict, "Dữ liệu vừa thay đổi, vui lòng thử lại.", false},
	{errs.ErrStorageUnsupported, http.StatusNotImplemented, "Tính năng này không khả dụng trên kho dữ liệu hiện tại.", false},
	// Filesystem absence on the file backend is the same client-facing fact.
	{fs.ErrNotExist, http.StatusNotFound, "Không tìm thấy.", false},

	// Accounts/sessions (file and postgres repositories share these sentinels;
	// their texts are curated Vietnamese, never a driver error).
	{identity.ErrInvalidInput, http.StatusBadRequest, "", true},
	{identity.ErrInvalidCredentials, http.StatusUnauthorized, "", true},
	{identity.ErrInvalidPairing, http.StatusUnauthorized, "", true},
	{identity.ErrUnauthenticated, http.StatusUnauthorized, "", true},
	{identity.ErrInvalidRecovery, http.StatusBadRequest, "", true},
	{identity.ErrForbidden, http.StatusForbidden, "", true},
	{identity.ErrNotFound, http.StatusNotFound, "", true},
	{identity.ErrExists, http.StatusConflict, "", true},
	{identity.ErrLastAdmin, http.StatusConflict, "", true},
	{identity.ErrLastOwner, http.StatusConflict, "", true},
	{identity.ErrOwnerTaken, http.StatusConflict, "", true},
	{identity.ErrRecoveryExists, http.StatusConflict, "", true},
	{identity.ErrConflict, http.StatusConflict, "", true},
	{identity.ErrUnsupported, http.StatusNotImplemented, "", true},

	// Library / project selection.
	{library.ErrInvalidName, http.StatusBadRequest, "", true},
	{library.ErrNotCandidate, http.StatusBadRequest, "", true},
	{library.ErrNotFound, http.StatusNotFound, "", true},
	{library.ErrIdeaNotFound, http.StatusNotFound, "", true},
	{library.ErrExists, http.StatusConflict, "", true},
	{library.ErrLegacy, http.StatusConflict, "", true},

	// Comments.
	{comments.ErrInvalid, http.StatusBadRequest, "", true},
	{comments.ErrNotFound, http.StatusNotFound, "", true},

	// Audio domain.
	{audio.ErrNotFound, http.StatusNotFound, "", true},
	{audio.ErrChapterIncomplete, http.StatusConflict, "", true},
	{audio.ErrNoModel, http.StatusConflict, "", true},
	{audio.ErrFFmpegMissing, http.StatusServiceUnavailable, "", true},
	{audio.ErrEngineMissing, http.StatusServiceUnavailable, "", true},
}

// mapHTTPError applies the shared table. A database/network dependency that
// is not a domain sentinel reports 503 (temporarily unavailable, never a
// fabricated client error); ok=false means the error is unexpected at the
// transport layer and the caller answers the generic 500 after logging it.
func mapHTTPError(err error) (status int, message string, ok bool) {
	if err == nil {
		return 0, "", false
	}
	for _, rule := range httpErrorRules {
		if errors.Is(err, rule.err) {
			msg := rule.message
			if rule.ownText {
				msg = err.Error()
			}
			return rule.status, msg, true
		}
	}
	if dependencyUnavailable(err) {
		return http.StatusServiceUnavailable, "Máy chủ tạm thời không sẵn sàng, vui lòng thử lại sau.", true
	}
	return 0, "", false
}

// dependencyUnavailable reports whether err is a storage/network dependency
// failure rather than a domain decision. Repository errors are already
// credential-sanitized before they reach this boundary (see the identity
// pool), so matching by type is safe; the raw text still never leaves the
// server log.
func dependencyUnavailable(err error) bool {
	var pgErr *pgconn.PgError
	var connErr *pgconn.ConnectError
	var netErr net.Error
	return errors.As(err, &pgErr) || errors.As(err, &connErr) || errors.As(err, &netErr) ||
		errors.Is(err, puddle.ErrClosedPool)
}

// jsonMappedError is the single entry point handlers use for a caught error:
// classified errors keep their policy status/envelope, everything else becomes
// a generic 500 with the cause logged server-side only.
func jsonMappedError(w http.ResponseWriter, err error) {
	status, msg, ok := mapHTTPError(err)
	if !ok {
		logRequestFailure(err)
		jsonError(w, http.StatusInternalServerError, errInternalMessage)
		return
	}
	jsonError(w, status, msg)
}

// logRequestFailure records a failed request server-side: stable context plus
// the sanitized cause (pool.sanitizeErr already removed credential material on
// the DB path). Never call this with a DSN, token or raw credential.
func logRequestFailure(err error) {
	slog.Error("yêu cầu thất bại", "module", "web", "err", err)
}
