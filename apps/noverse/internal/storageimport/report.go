package storageimport

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// WriteManifest persists the frozen inventory. It is written before any write
// to the target so the run can always be audited against exactly what was
// seen; the same bytes produce the same manifest hash.
func WriteManifest(path string, m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// LoadManifest reads a manifest written by WriteManifest.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("manifest hỏng: %w", err)
	}
	if m.SourceID == "" || m.SHA256 == "" {
		return nil, fmt.Errorf("manifest thiếu sourceId/manifest hash")
	}
	return &m, nil
}

// Summary renders the Vietnamese human summary of an import/dry run.
func (r *Result) Summary() string {
	var b strings.Builder
	mode := "NHẬP"
	if r.DryRun {
		mode = "CHẠY KHÔ: phân loại + giải mã, không ghi DB/blob"
	}
	fmt.Fprintf(&b, "Chế độ: %s\n", mode)
	fmt.Fprintf(&b, "Nguồn: %s\n", r.SourceID)
	fmt.Fprintf(&b, "Manifest: %s\n", short(r.ManifestSHA256))
	fmt.Fprintf(&b, "Tài khoản: %d · Dự án: %d · Truyện: %d\n", r.Users, r.Projects, r.Stories)
	fmt.Fprintf(&b, "Bản ghi: thêm %d, bỏ qua %d · Blob: tải %d, bỏ qua %d (%s)\n",
		r.RowsInserted, r.RowsSkipped, r.BlobsUploaded, r.BlobsSkipped, FormatSize(r.BlobBytes))
	fmt.Fprintf(&b, "Đường dẫn tái tạo được (derived): %d · loại trừ có lý do: %d\n", r.Derived, r.Excluded)
	fmt.Fprintf(&b, "Phiên đăng nhập huỷ một lần: %d · pairing huỷ: %d\n", r.SessionsDrop, r.PairingsDrop)
	if len(r.Domains) > 0 {
		keys := make([]string, 0, len(r.Domains))
		for k := range r.Domains {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString("Theo miền: ")
		for i, k := range keys {
			if i > 0 {
				b.WriteString(" · ")
			}
			stat := r.Domains[k]
			fmt.Fprintf(&b, "%s %d bản ghi", k, stat.Records)
		}
		b.WriteString("\n")
	}
	if len(r.Tolerated) > 0 {
		fmt.Fprintf(&b, "Bỏ qua có ghi nhận (%d, không chặn publish):\n", len(r.Tolerated))
		for _, q := range r.Tolerated {
			fmt.Fprintf(&b, "  - %s\n", q)
		}
	}
	if len(r.Quarantined) > 0 {
		fmt.Fprintf(&b, "CÁCH LY (%d) — cần xử lý trước khi publish:\n", len(r.Quarantined))
		for _, q := range r.Quarantined {
			fmt.Fprintf(&b, "  - %s\n", q)
		}
	}
	if len(r.Conflicts) > 0 {
		fmt.Fprintf(&b, "XUNG ĐỘT (%d):\n", len(r.Conflicts))
		for _, c := range r.Conflicts {
			fmt.Fprintf(&b, "  - %s\n", c)
		}
	}
	if r.Failed() {
		b.WriteString("KẾT LUẬN: DỪNG — chưa được publish.\n")
	} else if r.DryRun {
		b.WriteString("KẾT LUẬN: chạy khô sạch, có thể nhập thật.\n")
	} else {
		fmt.Fprintf(&b, "KẾT LUẬN: import %s đã verified.\n", short(r.RunID))
	}
	return b.String()
}

// FormatReconcile renders the Vietnamese summary of a reconciliation.
func (r *Report) FormatReconcile() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Đối soát nguồn: %s · manifest %s\n", r.SourceID, short(r.ManifestSHA256))
	fmt.Fprintf(&b, "Run verified: %v\n", r.RunVerified)
	fmt.Fprintf(&b, "Đã kiểm: %d đường dẫn, %d bản ghi, %d blob (%d blob xác minh SHA-256), %d khoá đối tượng\n",
		r.FilesChecked, r.ItemsChecked, r.BlobsChecked, r.BlobsVerified, r.BlobKeysChecked)
	if len(r.Counts) > 0 {
		keys := make([]string, 0, len(r.Counts))
		for k := range r.Counts {
			if strings.HasPrefix(k, "ledger:") {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString("Số dòng (DB / nguồn): ")
		for i, k := range keys {
			if i > 0 {
				b.WriteString(" · ")
			}
			fmt.Fprintf(&b, "%s %d/%d", k, r.Counts[k], r.Counts["ledger:"+k])
		}
		b.WriteString("\n")
	}
	if len(r.Missing) > 0 {
		fmt.Fprintf(&b, "THIẾU (%d):\n", len(r.Missing))
		for _, m := range r.Missing {
			fmt.Fprintf(&b, "  - %s\n", m)
		}
	}
	if len(r.Mismatches) > 0 {
		fmt.Fprintf(&b, "LỆCH (%d):\n", len(r.Mismatches))
		for _, m := range r.Mismatches {
			fmt.Fprintf(&b, "  - %s\n", m)
		}
	}
	if len(r.OwnerViolations) > 0 {
		fmt.Fprintf(&b, "VI PHẠM OWNER (%d):\n", len(r.OwnerViolations))
		for _, m := range r.OwnerViolations {
			fmt.Fprintf(&b, "  - %s\n", m)
		}
	}
	if r.OK() {
		b.WriteString("KẾT LUẬN: 0 lệch — dữ liệu khớp nguồn.\n")
	} else {
		b.WriteString("KẾT LUẬN: CÓ LỆCH — không được publish.\n")
	}
	return b.String()
}

// FormatExport renders the reverse-export summary.
func (r *ExportReport) FormatExport() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Xuất ngược về: %s\n", r.Destination)
	fmt.Fprintf(&b, "File: %d · blob: %d · tổng %s\n", r.FilesWritten, r.BlobsWritten, FormatSize(r.BytesWritten))
	if r.SessionsRevoked > 0 || r.PairingsRevoked > 0 {
		fmt.Fprintf(&b, "Đã thu hồi %d phiên, %d pairing (đăng nhập lại sau khi chuyển backend).\n",
			r.SessionsRevoked, r.PairingsRevoked)
	}
	if len(r.Skipped) > 0 {
		fmt.Fprintf(&b, "Bỏ qua có ghi nhận (%d):\n", len(r.Skipped))
		for _, s := range r.Skipped {
			fmt.Fprintf(&b, "  - %s\n", s)
		}
	}
	return b.String()
}

func short(hash string) string {
	if len(hash) > 12 {
		return hash[:12]
	}
	return hash
}
