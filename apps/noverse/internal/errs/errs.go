// Package errs provides application-level error sentinels for ainovel-cli.
// Callers wrap errors with fmt.Errorf("...: %w", errs.ErrXxx) and use
// errors.Is to detect categories.
//
// Provider runtime errors (rate_limit / timeout / stream_idle / network / auth
// / context_overflow) live in agentcore — use agentcore.ClassifyProvider,
// agentcore.IsFailoverEligible, agentcore.FailoverReason, and
// agentcore.IsStreamIdleMessage directly.
package errs

import "errors"

var (
	ErrConfig           = errors.New("lỗi cấu hình")
	ErrProvider         = errors.New("lỗi khởi tạo nhà cung cấp") // provider initialization / wiring
	ErrStoreRead        = errors.New("store read error")
	ErrStoreWrite       = errors.New("store write error")
	ErrToolArgs         = errors.New("tool args invalid")
	ErrToolPrecondition = errors.New("tool precondition failed")
	ErrToolConflict     = errors.New("tool conflict")
	ErrPhaseTransition  = errors.New("invalid phase transition")
	ErrFlowTransition   = errors.New("invalid flow transition")

	// Storage backend contract errors. Backends (file, postgres) return these
	// sentinels so HTTP/tool layers can map them to stable statuses regardless
	// of driver; errors.Is matching is the contract, not message text.
	ErrStorageNotFound    = errors.New("resource not found")
	ErrStorageConflict    = errors.New("conflicting concurrent write")
	ErrStorageForbidden   = errors.New("action forbidden for this actor")
	ErrStorageUnsupported = errors.New("backend does not support this capability")
)
