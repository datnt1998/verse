package ttsworker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Durable per-credential worker state: the last operator command sequence this
// machine executed, plus any safety stop that must survive a restart.
//
// The server keeps the last command row indefinitely; only this machine knows
// whether it ran. Keeping that knowledge in memory alone means every restart
// (launchd, crash, upgrade) replays the old install command. The same is true
// for the hash_mismatch stop: it is a safety decision (the bytes did not match
// the pin), so an ordinary restart must not silently retry the download. Both
// facts live in one marker per credential:
//
//	<StateDir>/command-<sha256(token)[:8]>.json   {"seq":7,"blocked":"hash_mismatch"}
//
// The token never appears in a file name, a file, or a log line; the marker is
// written to a temp file (0600) and renamed into place, so a crash can never
// leave a half-written record. The marker is written before the command runs:
// losing one click on a crash is recoverable by clicking again, replaying a
// consumed command (or retrying a checksum stop) is not.
//
// Failure handling is fail-closed: if the marker cannot be written, the command
// does not run, and a checksum stop that cannot be recorded disables automatic
// provisioning for this process (the next start re-probes the store and also
// refuses). An unreadable/corrupt marker counts as "stop unknown", never as
// "clean". Only a genuinely new operator command rewrites it.

const commandSeqPrefix = "command-"

// errNoStateDir reports that no durable state directory can be resolved.
var errNoStateDir = errors.New("không có thư mục trạng thái worker")

// commandStateRecord is the on-disk shape. Seq only grows; a lower value on
// disk than in memory is ignored rather than trusted. Blocked holds the stable
// provision error code of a safety stop (currently hash_mismatch); empty means
// automatic provisioning is allowed.
type commandStateRecord struct {
	Seq     int64  `json:"seq"`
	Blocked string `json:"blocked,omitempty"`
}

// commandSeqPath resolves the marker for this worker's credential. It returns
// "" when no state directory can be determined.
func (w *Worker) commandSeqPath() string {
	dir := w.cfg.StateDir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return ""
		}
		dir = filepath.Join(home, ".ainovel", "worker")
	}
	sum := sha256.Sum256([]byte(w.cfg.Token))
	return filepath.Join(dir, commandSeqPrefix+hex.EncodeToString(sum[:8])+".json")
}

// loadCommandState restores the consumed sequence and any safety stop at
// construction time, before the startup provisioning pass can run.
func (w *Worker) loadCommandState() {
	path := w.commandSeqPath()
	if path == "" {
		// No durable store: New's probe fails closed for automatic provisioning.
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			// We cannot prove there is no stop; never auto-provision on a guess.
			w.mu.Lock()
			w.blockUnknown = true
			w.mu.Unlock()
		}
		return
	}
	var rec commandStateRecord
	if err := json.Unmarshal(raw, &rec); err != nil || rec.Seq < 0 {
		w.mu.Lock()
		w.blockUnknown = true
		w.mu.Unlock()
		return
	}
	w.mu.Lock()
	if rec.Seq > w.lastSeq {
		w.lastSeq = rec.Seq
	}
	if rec.Blocked != "" {
		w.blocked, w.blockOnDisk = rec.Blocked, true
	}
	w.mu.Unlock()
}

// persistCommandState writes seq/blocked and, on success, adopts them in
// memory. A write failure changes no memory value: the caller decides what that
// means (a command waits, a safety stop fails closed).
func (w *Worker) persistCommandState(seq int64, blocked string) error {
	path := w.commandSeqPath()
	if path == "" {
		w.setStoreFailed()
		return errNoStateDir
	}
	if err := writeCommandState(path, commandStateRecord{Seq: seq, Blocked: blocked}); err != nil {
		w.setStoreFailed()
		return err
	}
	w.mu.Lock()
	w.lastSeq, w.blocked = seq, blocked
	w.blockOnDisk = blocked != ""
	w.stateOK, w.blockUnknown = true, false
	w.mu.Unlock()
	return nil
}

// writeCommandState replaces the marker atomically: a crash leaves either the
// previous record or the new one, never a truncated file.
func writeCommandState(path string, rec commandStateRecord) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("tạo thư mục trạng thái: %w", err)
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".command-*.tmp")
	if err != nil {
		return fmt.Errorf("tạo tệp tạm: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("ghi tệp tạm: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("đồng bộ tệp tạm: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("đóng tệp tạm: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("đổi tên tệp trạng thái: %w", err)
	}
	return nil
}

// setStoreFailed marks the durable store unusable, which forbids automatic
// provisioning (fail closed).
func (w *Worker) setStoreFailed() {
	w.mu.Lock()
	w.stateOK = false
	w.mu.Unlock()
}

// safetyStop reports whether automatic provisioning is currently forbidden:
// code is the durable stop reason ("" when none) and unknown means the store
// could not be read or written, so the absence of a stop cannot be proven.
func (w *Worker) safetyStop() (code string, unknown bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.blocked != "" {
		return w.blocked, false
	}
	return "", w.blockUnknown || !w.stateOK
}

func (w *Worker) blockedCode() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.blocked
}

func (w *Worker) blockSaved() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.blockOnDisk
}

func (w *Worker) blockedUnknown() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.blockUnknown
}

func (w *Worker) seqSnapshot() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastSeq
}
