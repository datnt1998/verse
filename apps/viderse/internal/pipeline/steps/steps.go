// Package steps holds helpers shared by the clone step handlers: the stable
// JSON fingerprint every manifest/signature uses, file hashing, atomic JSON
// writes and small error factories. No I/O policy lives here — steps still
// move every published file through media Publish/Acquire.
package steps

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// CloneError dựng lỗi step với code/recovery ổn định (port CloneError của
// yt-factory errors.ts: {code,message,recovery,retryAt,retryAtSource}).
func CloneError(code, message, recovery string) *domain.CloneError {
	var rec *string
	if recovery != "" {
		rec = &recovery
	}
	return &domain.CloneError{Code: code, Message: message, Recovery: rec}
}

// StableJSON là JSON.stringify với key object đã sắp xếp — hai bản cùng nội
// dung luôn ra một chuỗi (port stableJson).
func StableJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	var decoded any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&decoded); err != nil {
		return ""
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(decoded); err != nil {
		return ""
	}
	return strings.TrimSuffix(out.String(), "\n")
}

// InputFingerprint là khoá dedup của một tập đầu vào step: "sha256:<hex>" của
// stableJson (port inputFingerprint).
func InputFingerprint(input any) string {
	sum := sha256.Sum256([]byte(StableJSON(input)))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// FingerprintHex trả sha256 hex của stableJson(input) — dùng cho
// sourceFingerprint không mang tiền tố.
func FingerprintHex(input any) string {
	sum := sha256.Sum256([]byte(StableJSON(input)))
	return hex.EncodeToString(sum[:])
}

// SHA256File băm nội dung một tệp (port sha256File của artifacts.ts).
func SHA256File(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// WriteJSONAtomic ghi JSON vào file qua tệp tạm + rename — nguyên tử trong
// cùng filesystem (port writeJsonAtomic).
func WriteJSONAtomic(file string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	tmp := file + ".part"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

// DecodeJSONFile đọc và giải mã một tệp JSON vào T; trường lạ bị bỏ qua như
// zod mặc định (chỉ schema .strict() mới kiểm riêng). Sai lệch báo lỗi với
// message của caller (port parseJsonArtifact).
func DecodeJSONFile[T any](file, invalid string) (T, error) {
	var zero T
	data, err := os.ReadFile(file)
	if err != nil {
		return zero, err
	}
	if err := json.Unmarshal(data, &zero); err != nil {
		return zero, fmt.Errorf("%s", invalid)
	}
	return zero, nil
}

// CopyFile stages an acquired file without retaining an S3/DB read lease.
// Streaming avoids loading media-sized byte slices into memory.
func CopyFile(source, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.Create(target + ".part")
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	syncErr := output.Sync()
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(target+".part", target)
}
