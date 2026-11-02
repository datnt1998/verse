package library

// Bộ ghi tệp template (`.ytf-template.zip`) — cùng hợp đồng với bộ đọc trong
// template_bundle.go: `template.json` ở gốc gói rồi `media/<n>-<tên tệp>` cho
// đúng những media project tham chiếu. Mọi media được kiểm kích thước và
// sha256 trước khi ghi byte đầu tiên; tên mục chỉ do máy sinh, không lấy từ đĩa.

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

// Phần khung zip không phải bytes media: mỗi mục Store khi ghi trực tiếp có
// header cục bộ 30 byte + descriptor 24 byte + header thư mục trung tâm 46
// byte, cộng thêm extra zip64 khi gói vượt 4 GiB. Tên mục bị chặn ở 100 rune
// (exportSafePart), nên 2 KiB mỗi mục là trần an toàn; phần cố định đủ cho
// bản ghi kết thúc zip và phần phình tối đa của deflate trên manifest.
const (
	exportZipEntryOverhead = 2 << 10
	exportZipFixedOverhead = 64 << 10
)

// TemplateExportInfo là kết quả một lần xuất: tên tệp gợi ý và số media.
type TemplateExportInfo struct {
	FileName   string
	Name       string
	MediaCount int
}

// MediaReader là mặt đọc media của editor mà bộ xuất cần. Service Thư viện
// được dựng trước service Editor nên mặt này gắn muộn, như MediaImporter.
type MediaReader interface {
	GetMediaMetadata(ctx context.Context, id string) (editor.MediaRecord, error)
	AcquireMedia(ctx context.Context, id string) (*media.Handle, error)
}

// BindMediaReader gắn mặt đọc media cho bộ xuất template.
func (s *Service) BindMediaReader(reader MediaReader) { s.mediaReader = reader }

type exportMediaFile struct {
	entry  string
	record editor.MediaRecord
	handle *media.Handle
}

// TemplateExport là một gói đã kiểm xong, sẵn sàng ghi: mọi media đã mở và
// manifest đã dựng, nên lỗi thiếu media xảy ra trước byte đầu tiên.
type TemplateExport struct {
	Info     TemplateExportInfo
	manifest []byte
	files    []exportMediaFile
}

// Close giải phóng mọi Handle đang giữ. An toàn khi gọi nhiều lần.
func (e *TemplateExport) Close() {
	if e == nil {
		return
	}
	for _, file := range e.files {
		if file.handle != nil {
			file.handle.Release()
			file.handle = nil
		}
	}
}

// OpenTemplateExport đọc template, kiểm đủ media và dựng manifest. Caller
// phải Close.
func (s *Service) OpenTemplateExport(ctx context.Context, id string) (*TemplateExport, error) {
	tpl, err := s.store.GetTemplate(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, notFound("Mẫu video")
		}
		return nil, err
	}
	if tpl == nil {
		return nil, notFound("Mẫu video")
	}
	tpl, err = s.ensureTemplateMedia(ctx, tpl)
	if err != nil {
		return nil, err
	}
	if s.mediaReader == nil {
		return nil, invalidInput("Máy chủ chưa sẵn sàng xuất mẫu video", "Khởi động lại Viderse rồi thử lại")
	}
	project, err := editor.Parse(tpl.Project)
	if err != nil {
		return nil, invalidInput("Mẫu video không đọc được", "Mở mẫu video trong Biên tập rồi lưu lại")
	}
	ids := projectMediaIDs(project)
	budget, okBudget := templateExportContentBudget(len(ids))
	if !okBudget {
		return nil, templateExportTooLarge()
	}
	export := &TemplateExport{Info: TemplateExportInfo{FileName: templateExportFileName(tpl.Name), Name: tpl.Name}}
	ok := false
	defer func() {
		if !ok {
			export.Close()
		}
	}()
	var total int64
	missing := 0
	for index, mediaID := range ids {
		record, err := s.mediaReader.GetMediaMetadata(ctx, mediaID)
		if err != nil {
			if templateMediaGone(err) {
				missing++
				continue
			}
			return nil, fmt.Errorf("đọc metadata media %s: %w", mediaID, err)
		}
		if record.Size < 0 {
			return nil, fmt.Errorf("media %s có kích thước không hợp lệ", mediaID)
		}
		if !validSHA256(record.ContentHash) {
			return nil, fmt.Errorf("media %s thiếu mã nội dung sha256", mediaID)
		}
		// Từ chối trước khi mở tệp: gói quá trần nhập thì người nhận không mở
		// được, nên đừng ghi ra một tệp vô dụng.
		if record.Size > budget-total {
			return nil, templateExportTooLarge()
		}
		handle, err := s.mediaReader.AcquireMedia(ctx, mediaID)
		if err != nil {
			if templateMediaGone(err) {
				missing++
				continue
			}
			return nil, fmt.Errorf("đọc media %s: %w", mediaID, err)
		}
		if handle.Ref.Bytes != record.Size {
			handle.Release()
			return nil, fmt.Errorf("media %s sai kích thước", mediaID)
		}
		// Bộ nhập kiểm sha256 chứ không chỉ kích thước; kiểm ở đây để tệp hỏng
		// cùng kích thước bị chặn trước byte đầu tiên của gói. Đọc theo luồng,
		// một lượt có chặn, không dựng cả tệp trong bộ nhớ.
		digest, err := digestTemplateFile(handle)
		if err != nil {
			handle.Release()
			if templateMediaGone(err) {
				missing++
				continue
			}
			return nil, fmt.Errorf("đọc media %s: %w", mediaID, err)
		}
		if digest != strings.TrimPrefix(record.ContentHash, "sha256:") {
			handle.Release()
			return nil, fmt.Errorf("media %s không khớp sha256", mediaID)
		}
		total += record.Size
		export.files = append(export.files, exportMediaFile{
			entry:  fmt.Sprintf("media/%d-%s", index+1, exportSafePart(record.FileName, "media")),
			record: record, handle: handle,
		})
	}
	if missing > 0 {
		return nil, invalidInput(
			fmt.Sprintf("Mẫu video đang dùng %d media đã bị xoá khỏi thư viện", missing),
			"Mở mẫu video trong Biên tập, xoá hoặc thay các lớp báo thiếu media, rồi xuất lại")
	}
	manifest := bundleManifest{
		Format:     TemplateBundleFormat,
		Version:    TemplateBundleVersion,
		ExportedAt: s.now().UTC().Format(time.RFC3339Nano),
		Name:       tpl.Name,
		Project:    tpl.Project,
		Media:      make([]bundleManifestM, 0, len(export.files)),
	}
	for _, file := range export.files {
		manifest.Media = append(manifest.Media, bundleManifestM{
			ID: file.record.ID, Entry: file.entry, FileName: file.record.FileName,
			MIME: file.record.MIME, Size: file.record.Size, ContentHash: file.record.ContentHash,
			Tags: file.record.Tags, Attribution: file.record.Attribution, Overlay: file.record.Overlay,
		})
	}
	export.manifest, err = json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	export.manifest = append(export.manifest, '\n')
	if int64(len(export.manifest)) > budget-total {
		return nil, templateExportTooLarge()
	}
	export.Info.MediaCount = len(export.files)
	ok = true
	return export, nil
}

// templateExportContentBudget là số byte media + manifest còn vừa trần nhập.
// Trần lấy từ đúng hằng của bộ nhập (TemplateBundleLimit), không chép lại 4
// GiB/90 MiB; phép trừ có chặn để không tràn số.
func templateExportContentBudget(files int) (int64, bool) {
	limit := TemplateBundleLimit()
	if limit <= exportZipFixedOverhead {
		return 0, false
	}
	budget := limit - exportZipFixedOverhead
	if files > 0 {
		if int64(files) > budget/exportZipEntryOverhead {
			return 0, false
		}
		budget -= int64(files) * exportZipEntryOverhead
	}
	if budget < 0 {
		return 0, false
	}
	return budget, true
}

// templateExportTooLarge dùng lại câu trần dung lượng của bộ nhập để hai đầu
// nói cùng một giới hạn.
func templateExportTooLarge() *Error {
	_, recovery := TemplateBundleTooLarge()
	return &Error{Code: CodeInvalidInput, Message: "Mẫu video quá lớn để xuất", Recovery: recovery}
}

// templateMediaGone chỉ coi lỗi "không có" là media đã bị xoá. Lỗi DB, huỷ
// request hay lỗi đọc tạm thời phải nổi lên như lỗi nội bộ, không được gộp
// thành "mọi media đã bị xoá".
func templateMediaGone(err error) bool {
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrNotFound) || errors.Is(err, blobstore.ErrNotFound) {
		return true
	}
	var editorErr *editor.Error
	return errors.As(err, &editorErr) && editorErr.Code == "not_found"
}

// WriteTo ghi gói ra w.
func (e *TemplateExport) WriteTo(ctx context.Context, w io.Writer) error {
	archive := zip.NewWriter(w)
	head, err := archive.CreateHeader(&zip.FileHeader{Name: bundleManifestName, Method: zip.Deflate})
	if err != nil {
		return err
	}
	if _, err := head.Write(e.manifest); err != nil {
		return err
	}
	for _, file := range e.files {
		if err := ctx.Err(); err != nil {
			return err
		}
		entry, err := archive.CreateHeader(&zip.FileHeader{Name: file.entry, Method: zip.Store})
		if err != nil {
			return err
		}
		source, err := os.Open(file.handle.Path)
		if err != nil {
			return err
		}
		written, copyErr := io.Copy(entry, source)
		closeErr := source.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if written != file.record.Size {
			return fmt.Errorf("media %s đổi kích thước khi xuất", file.record.ID)
		}
	}
	return archive.Close()
}

// ExportTemplate ghi gói template ra w (dùng cho CLI/test).
func (s *Service) ExportTemplate(ctx context.Context, id string, w io.Writer) (TemplateExportInfo, error) {
	export, err := s.OpenTemplateExport(ctx, id)
	if err != nil {
		return TemplateExportInfo{}, err
	}
	defer export.Close()
	if err := export.WriteTo(ctx, w); err != nil {
		return TemplateExportInfo{}, err
	}
	return export.Info, nil
}

// TemplateExportFileName là tên tải xuống an toàn cho một template.
func TemplateExportFileName(name string) string { return templateExportFileName(name) }

// exportSafePart là phần tên tệp an toàn cho zip và hệ tệp: không dấu phân
// cách, không đi lên thư mục, không ký tự điều khiển.
func exportSafePart(raw, fallback string) string {
	var b strings.Builder
	for _, r := range raw {
		if r == '\\' || r == '/' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' || unicode.IsControl(r) {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(r)
	}
	clean := strings.Join(strings.Fields(b.String()), " ")
	clean = strings.ReplaceAll(clean, "..", ".")
	clean = strings.TrimSpace(clean)
	if clean == "" || clean == "." {
		return fallback
	}
	runes := []rune(clean)
	if len(runes) > 100 {
		clean = strings.TrimSpace(string(runes[:100]))
	}
	return clean
}

// templateExportFileName là tên tải xuống gợi ý, luôn kết thúc bằng đuôi gói.
func templateExportFileName(name string) string {
	return exportSafePart(name, "template") + ".ytf-template.zip"
}
