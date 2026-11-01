package ttsworker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Durable operator-command acknowledgement.
//
// The server keeps the last command row indefinitely; only this machine knows
// whether it ran. Keeping that knowledge in memory alone means every restart
// (launchd, crash, upgrade) replays the old install command, and that replay
// also clears a hash_mismatch stop nobody overrode — the retry budget would
// restart by itself. So the executed sequence is stored on disk, per
// credential:
//
//	<StateDir>/command-<sha256(token)[:8]>.json   {"seq": 7}
//
// The token never appears in a file name, a file, or a log line; the marker is
// written to a temp file (0600) and renamed into place, so a crash can never
// leave a half-written record. The marker is written before the command runs:
// losing one click on a crash is recoverable by clicking again, replaying a
// consumed command is not.
//
// Failure handling: if the marker cannot be written the command is not run and
// the poll returns an error; the row is still queued server-side, so the next
// poll retries the write. With no state directory at all (no HOME and no
// --state-dir) the worker falls back to the in-memory sequence only, which is
// exactly the pre-existing behaviour on a machine that cannot hold state.

const commandSeqPrefix = "command-"

// commandSeqRecord is the on-disk shape. Seq only grows; a lower value on disk
// than in memory is ignored rather than trusted.
type commandSeqRecord struct {
	Seq int64 `json:"seq"`
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

// loadCommandSeq restores the last executed sequence at construction time, so a
// replacement worker never re-runs a command the previous process consumed.
func (w *Worker) loadCommandSeq() {
	path := w.commandSeqPath()
	if path == "" {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		// Absent marker: nothing was consumed by a previous run (or the file is
		// unreadable, in which case the next write repairs it).
		return
	}
	var rec commandSeqRecord
	if err := json.Unmarshal(raw, &rec); err != nil || rec.Seq < 0 {
		return
	}
	w.mu.Lock()
	if rec.Seq > w.lastSeq {
		w.lastSeq = rec.Seq
	}
	w.mu.Unlock()
}

// persistCommandSeq records seq as consumed before the command runs.
func (w *Worker) persistCommandSeq(seq int64) error {
	path := w.commandSeqPath()
	if path == "" {
		// No durable location: keep the in-memory guarantee only.
		return nil
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("tạo thư mục trạng thái: %w", err)
	}
	data, err := json.Marshal(commandSeqRecord{Seq: seq})
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
