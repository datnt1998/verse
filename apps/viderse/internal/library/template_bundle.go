package library

// Định dạng tệp template cũ (`.ytf-template.zip`): `template.json` ở gốc gói,
// rồi `media/<n>-<tên tệp>` cho từng media template dùng. Bộ đọc bám đúng hợp
// đồng của bản xuất đã có — cùng khoá manifest, cùng cách chịu lỗi của tệp bị
// nén lại trên máy khác:
//
//	template.json          { format: "ytf-template", version: 1, exportedAt,
//	                         name, project, media: [{ id, entry, fileName, mime,
//	                         size, contentHash, tags?, attribution?, overlay? }] }
//	media/<n>-<tên tệp>    đúng `media[].entry`
//
// Import không tin đường dẫn trong gói: chỉ đọc đúng entry manifest chỉ ra,
// kiểm size và sha256 từng tệp, và không bao giờ dùng tên entry để tạo đường
// dẫn trên đĩa. Gói bị nén lại (thêm một thư mục gốc, `__MACOSX/`, `._*`) vẫn
// đọc được như bản gốc.
//
// Cùng bộ đọc phục vụ thư mục editor cũ (`projects/<id>/project.json` +
// `media/<id>/`), để CLI nhập thẳng thư viện cũ mà không phải chạy công cụ
// trước đây lần nào.

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
)

const (
	// TemplateBundleFormat là dấu nhận dạng trong `template.json`.
	TemplateBundleFormat = "ytf-template"
	// TemplateBundleVersion là phiên bản manifest bộ đọc này hiểu được.
	TemplateBundleVersion = 1
	// MaxTemplateBundleBytes là trần tệp tải lên (4 GiB): đủ cho một template
	// vài chục video nền 1080p, vẫn chặn được tệp rác khổng lồ.
	MaxTemplateBundleBytes = int64(4) << 30
	// maxBundleManifestBytes là trần `template.json` (cùng hằng của bản xuất cũ).
	maxBundleManifestBytes = 32 << 20
	// maxBundleMedia là trần số media một template (cùng hằng của bản xuất cũ).
	maxBundleMedia = 10_000
)

const bundleManifestName = "template.json"

// BundleMedia là một tệp media template dùng, kèm cách mở nó từ nguồn.
type BundleMedia struct {
	// ID là id media bên bản xuất cũ; project trỏ tới id này.
	ID          string
	FileName    string
	MIME        string
	Size        int64
	ContentHash string // "sha256:<hex>"
	Tags        []string
	Attribution *editor.MediaAttribution
	Overlay     bool

	// entry là mục trong gói zip, hoặc đường dẫn tệp khi đọc thẳng thư mục.
	entry string
	file  string
}

// TemplateBundle là một template đọc từ tệp xuất hoặc từ thư mục editor cũ.
type TemplateBundle struct {
	// Name là tên template lúc xuất.
	Name string
	// Project là project editor schema 15 y nguyên bản xuất.
	Project editor.Project
	// Media theo đúng thứ tự manifest.
	Media []BundleMedia

	archive *zip.ReadCloser
}

// Close giải phóng tệp zip đang mở. An toàn khi gọi nhiều lần.
func (b *TemplateBundle) Close() error {
	if b == nil || b.archive == nil {
		return nil
	}
	err := b.archive.Close()
	b.archive = nil
	return err
}

// OpenMedia mở bytes của Media[i] để kiểm sha256 rồi nhập vào kho media.
func (b *TemplateBundle) OpenMedia(i int) (io.ReadCloser, error) {
	if i < 0 || i >= len(b.Media) {
		return nil, fmt.Errorf("media index %d ngoài phạm vi", i)
	}
	media := b.Media[i]
	if media.file != "" {
		return os.Open(media.file)
	}
	if b.archive == nil {
		return nil, fmt.Errorf("gói template đã đóng")
	}
	for _, entry := range b.archive.File {
		if entry.Name == media.entry {
			return entry.Open()
		}
	}
	return nil, fmt.Errorf("gói template thiếu mục %s", media.entry)
}

// ── manifest ───────────────────────────────────────────────────────────────

type bundleManifest struct {
	Format     string            `json:"format"`
	Version    int               `json:"version"`
	ExportedAt string            `json:"exportedAt"`
	Name       string            `json:"name"`
	Project    json.RawMessage   `json:"project"`
	Media      []bundleManifestM `json:"media"`
}

type bundleManifestM struct {
	ID          string                   `json:"id"`
	Entry       string                   `json:"entry"`
	FileName    string                   `json:"fileName"`
	MIME        string                   `json:"mime"`
	Size        int64                    `json:"size"`
	ContentHash string                   `json:"contentHash"`
	Tags        []string                 `json:"tags"`
	Attribution *editor.MediaAttribution `json:"attribution"`
	Overlay     bool                     `json:"overlay"`
}

// notTemplateFile là lỗi 400 dùng chung cho mọi tệp không đúng định dạng.
func notTemplateFile(message, recovery string) *Error {
	return invalidInput(message, recovery)
}

func bundleRecovery() string {
	return "Chọn tệp .ytf-template.zip (bản xuất template). Nếu Safari đã tự giải nén thành thư mục, chuột phải thư mục → Nén, rồi chọn tệp .zip vừa tạo"
}

// OpenTemplateBundle đọc một template từ tệp `.ytf-template.zip` hoặc từ thư
// mục project cũ (có `project.json`).
func OpenTemplateBundle(target string) (*TemplateBundle, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, invalidInput("Không đọc được tệp template", "Kiểm tra tệp còn ở đó rồi chọn lại")
	}
	if info.IsDir() {
		return openTemplateDir(target)
	}
	return openTemplateZip(target)
}

func openTemplateZip(target string) (*TemplateBundle, error) {
	if info, err := os.Stat(target); err == nil && info.Size() > MaxTemplateBundleBytes {
		return nil, invalidInput("Tệp template quá lớn", fmt.Sprintf("Mỗi tệp tối đa %d GiB", MaxTemplateBundleBytes>>30))
	}
	archive, err := zip.OpenReader(target)
	if err != nil {
		return nil, notTemplateFile("Tệp không phải tệp template", bundleRecovery())
	}
	bundle, err := readZipBundle(archive)
	if err != nil {
		_ = archive.Close()
		return nil, err
	}
	bundle.archive = archive
	return bundle, nil
}

// bundleFiles lọc mục rác của lần nén lại rồi trả bản đồ tên → mục, tính từ
// thư mục gốc chứa `template.json`.
func bundleFiles(entries []*zip.File) (map[string]*zip.File, error) {
	files := make([]*zip.File, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name
		if strings.HasSuffix(name, "/") || strings.HasPrefix(name, "__MACOSX/") {
			continue
		}
		if strings.HasPrefix(path.Base(name), "._") {
			continue
		}
		files = append(files, entry)
	}
	var manifests []*zip.File
	for _, entry := range files {
		if entry.Name == bundleManifestName || strings.HasSuffix(entry.Name, "/"+bundleManifestName) {
			manifests = append(manifests, entry)
		}
	}
	if len(manifests) == 0 {
		return nil, notTemplateFile("Tệp không phải tệp template", bundleRecovery())
	}
	if len(manifests) > 1 {
		return nil, notTemplateFile("Tệp chứa nhiều template", "Nhập từng tệp một")
	}
	root := strings.TrimSuffix(manifests[0].Name, bundleManifestName)
	view := make(map[string]*zip.File, len(files))
	for _, entry := range files {
		if !strings.HasPrefix(entry.Name, root) {
			continue
		}
		name := normalizeBundleName(strings.TrimPrefix(entry.Name, root))
		if _, seen := view[name]; seen {
			return nil, notTemplateFile("Tệp nén có hai mục cùng tên: "+name, bundleRecovery())
		}
		view[name] = entry
	}
	return view, nil
}

// normalizeBundleName bỏ tiền tố đường dẫn và ký tự điều khiển khỏi tên mục.
// Tên NFC/NFD không cần đổi: hai bên dùng cùng chuỗi từ manifest.
func normalizeBundleName(name string) string {
	cleaned := strings.ReplaceAll(name, "\\", "/")
	cleaned = strings.TrimPrefix(cleaned, "./")
	for strings.HasPrefix(cleaned, "/") {
		cleaned = strings.TrimPrefix(cleaned, "/")
	}
	return strings.ToValidUTF8(cleaned, "")
}

func readZipBundle(archive *zip.ReadCloser) (*TemplateBundle, error) {
	files, err := bundleFiles(archive.File)
	if err != nil {
		return nil, err
	}
	manifestEntry, ok := files[bundleManifestName]
	if !ok {
		return nil, notTemplateFile("Tệp không phải tệp template", bundleRecovery())
	}
	manifest, project, err := decodeBundleManifest(manifestEntry)
	if err != nil {
		return nil, err
	}
	bundle := &TemplateBundle{Name: manifest.Name, Project: project}
	for _, media := range manifest.Media {
		entry, ok := files[normalizeBundleName(media.Entry)]
		if !ok || int64(entry.UncompressedSize64) != media.Size {
			return nil, notTemplateFile("Template thiếu tệp media "+media.FileName, bundleRecovery())
		}
		bundle.Media = append(bundle.Media, BundleMedia{
			ID: media.ID, FileName: media.FileName, MIME: media.MIME, Size: media.Size,
			ContentHash: media.ContentHash, Tags: media.Tags, Attribution: media.Attribution,
			Overlay: media.Overlay, entry: entry.Name,
		})
	}
	return bundle, nil
}

// decodeBundleManifest đọc `template.json` và kiểm hợp đồng tối thiểu: đúng
// dấu nhận dạng, phiên bản bộ đọc này hiểu được, project có metadata + timeline.
func decodeBundleManifest(entry *zip.File) (bundleManifest, editor.Project, error) {
	var manifest bundleManifest
	if int64(entry.UncompressedSize64) > maxBundleManifestBytes {
		return manifest, nil, notTemplateFile("Nội dung template quá lớn", bundleRecovery())
	}
	reader, err := entry.Open()
	if err != nil {
		return manifest, nil, notTemplateFile("Không đọc được nội dung template", bundleRecovery())
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, maxBundleManifestBytes+1))
	if err != nil {
		return manifest, nil, notTemplateFile("Không đọc được nội dung template", bundleRecovery())
	}
	var head struct {
		Format  string `json:"format"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return manifest, nil, notTemplateFile("Không đọc được nội dung template", bundleRecovery())
	}
	if head.Format != TemplateBundleFormat {
		return manifest, nil, notTemplateFile("Tệp không phải tệp template", bundleRecovery())
	}
	if head.Version > TemplateBundleVersion {
		return manifest, nil, invalidInput("Tệp template thuộc phiên bản mới hơn", "Cập nhật Viderse rồi nhập lại")
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, nil, notTemplateFile("Nội dung template không hợp lệ", bundleRecovery())
	}
	project, err := validateBundleManifest(&manifest)
	return manifest, project, err
}

// validateBundleManifest kiểm phần manifest + project mà mọi đường nhập đều đi
// qua, rồi trả project đã parse.
func validateBundleManifest(manifest *bundleManifest) (editor.Project, error) {
	name := strings.TrimSpace(manifest.Name)
	if name == "" || len([]rune(name)) > 200 {
		return nil, notTemplateFile("Template thiếu tên", bundleRecovery())
	}
	manifest.Name = name
	if len(manifest.Media) > maxBundleMedia {
		return nil, notTemplateFile("Template có quá nhiều media", bundleRecovery())
	}
	project, err := editor.Parse(manifest.Project)
	if err != nil {
		return nil, notTemplateFile("Nội dung template không hợp lệ", bundleRecovery())
	}
	if err := validateBundleProject(project); err != nil {
		return nil, err
	}
	ids := make(map[string]bool, len(manifest.Media))
	for i := range manifest.Media {
		media := &manifest.Media[i]
		if media.ID == "" || len(media.ID) > 64 {
			return nil, notTemplateFile("Template có media thiếu mã", bundleRecovery())
		}
		if ids[media.ID] {
			return nil, notTemplateFile("Template liệt kê một media hai lần", bundleRecovery())
		}
		ids[media.ID] = true
		if media.FileName == "" || len(media.FileName) > 512 || len(media.Entry) > 512 {
			return nil, notTemplateFile("Template có tên tệp media không hợp lệ", bundleRecovery())
		}
		if media.MIME == "" || len(media.MIME) > 255 {
			return nil, notTemplateFile("Template có media thiếu loại tệp", bundleRecovery())
		}
		if media.Size < 0 {
			return nil, notTemplateFile("Template có media sai kích thước", bundleRecovery())
		}
		if !validSHA256(media.ContentHash) {
			return nil, notTemplateFile("Template có media thiếu mã kiểm nội dung", bundleRecovery())
		}
		if strings.Contains(media.Entry, "..") || path.IsAbs(strings.ReplaceAll(media.Entry, "\\", "/")) {
			return nil, notTemplateFile("Tệp nén có đường dẫn không hợp lệ", bundleRecovery())
		}
	}
	// Mọi media project trỏ tới phải có trong manifest, nếu không project sẽ
	// trỏ tới một id không tồn tại sau khi nhập.
	for _, id := range projectMediaIDs(project) {
		if !ids[id] {
			return nil, notTemplateFile("Template dùng media không có trong tệp", bundleRecovery())
		}
	}
	return project, nil
}

// validSHA256 kiểm dạng "sha256:<64 hex>".
func validSHA256(hash string) bool {
	if !strings.HasPrefix(hash, "sha256:") || len(hash) != 71 {
		return false
	}
	for _, char := range hash[7:] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

// validateBundleProject chặn project không phải schema 15 hoặc thiếu khối bắt
// buộc — ràng buộc `templates_project_schema` của DB cũng đòi đúng vậy.
func validateBundleProject(project editor.Project) error {
	version, _ := project["schemaVersion"].(float64)
	if int(version) != editor.SchemaVersion {
		return notTemplateFile("Template thuộc phiên bản project khác", fmt.Sprintf("Chỉ nhập được project schema %d", editor.SchemaVersion))
	}
	if _, ok := project.Metadata()["width"]; !ok {
		return notTemplateFile("Template không có khung hình", bundleRecovery())
	}
	if _, ok := project.Timeline()["tracks"]; !ok {
		return notTemplateFile("Template không có timeline", bundleRecovery())
	}
	return nil
}

// ── thư mục editor cũ ──────────────────────────────────────────────────────

// openTemplateDir đọc một project editor đang nằm trên đĩa:
//
//	<dir>/project.json            project schema 15
//	<dir>/../media/<id>/          metadata.json + tệp nguồn (workspace cũ)
//
// Media thiếu metadata hoặc thiếu tệp là lỗi: project trỏ tới nó sẽ hỏng sau
// khi nhập, nên báo ngay thay vì tạo template khuyết.
func openTemplateDir(dir string) (*TemplateBundle, error) {
	data, err := os.ReadFile(filepath.Join(dir, "project.json"))
	if err != nil {
		return nil, notTemplateFile("Thư mục không có project.json", bundleRecovery())
	}
	project, err := editor.Parse(data)
	if err != nil {
		return nil, notTemplateFile("Project không đọc được", bundleRecovery())
	}
	if err := validateBundleProject(project); err != nil {
		return nil, err
	}
	name, _ := project["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		name = filepath.Base(dir)
	}
	bundle := &TemplateBundle{Name: name, Project: project}
	mediaRoot := workspaceMediaRoot(dir)
	for _, id := range projectMediaIDs(project) {
		media, err := readWorkspaceMedia(mediaRoot, id)
		if err != nil {
			return nil, err
		}
		bundle.Media = append(bundle.Media, media)
	}
	return bundle, nil
}

// workspaceMediaRoot tìm thư mục media của workspace cũ: project nằm ở
// `<workspace>/projects/<id>` nên media ở `<workspace>/media` (hai cấp trên).
// Nhận thêm `../media` để thư mục project đặt cạnh media cũng đọc được.
func workspaceMediaRoot(projectDir string) string {
	for _, candidate := range []string{
		filepath.Join(projectDir, "..", "..", "media"),
		filepath.Join(projectDir, "..", "media"),
	} {
		if isDirectory(candidate) {
			return candidate
		}
	}
	return filepath.Join(projectDir, "..", "..", "media")
}

// readWorkspaceMedia đọc `media/<id>/metadata.json` + tệp nguồn của thư viện cũ.
func readWorkspaceMedia(mediaRoot, id string) (BundleMedia, error) {
	if strings.ContainsAny(id, `/\`) || id == "" || id == "." || id == ".." {
		return BundleMedia{}, notTemplateFile("Project trỏ tới media có mã không hợp lệ", bundleRecovery())
	}
	dir := filepath.Join(mediaRoot, id)
	data, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		return BundleMedia{}, invalidInput("Project dùng media không còn trong thư viện cũ",
			"Xuất template thành tệp .ytf-template.zip rồi nhập tệp đó")
	}
	var meta struct {
		FileName    string                   `json:"fileName"`
		MIME        string                   `json:"mime"`
		ContentHash string                   `json:"contentHash"`
		Tags        []string                 `json:"tags"`
		Attribution *editor.MediaAttribution `json:"attribution"`
		Overlay     bool                     `json:"overlay"`
		SourceFile  string                   `json:"sourceFile"`
	}
	if err := json.Unmarshal(data, &meta); err != nil || meta.FileName == "" || !validSHA256(meta.ContentHash) {
		return BundleMedia{}, notTemplateFile("Media của project có metadata không hợp lệ", bundleRecovery())
	}
	source := meta.SourceFile
	if source == "" {
		source = meta.FileName
	}
	if strings.ContainsAny(source, `/\`) {
		return BundleMedia{}, notTemplateFile("Media của project có tên tệp không hợp lệ", bundleRecovery())
	}
	file := filepath.Join(dir, source)
	info, err := os.Stat(file)
	if err != nil {
		return BundleMedia{}, invalidInput("Tệp media "+meta.FileName+" không còn trên đĩa", "Chép lại thư mục media của thư viện cũ rồi nhập lại")
	}
	return BundleMedia{
		ID: id, FileName: meta.FileName, MIME: meta.MIME, Size: info.Size(),
		ContentHash: meta.ContentHash, Tags: meta.Tags, Attribution: meta.Attribution,
		Overlay: meta.Overlay, file: file,
	}, nil
}

// ── media của project ──────────────────────────────────────────────────────

// projectMediaIDs trả mọi id media project trỏ tới (mediaId, coverMediaId lồng
// trong timeline và danh sách mediaItems), đã sắp xếp — cùng phép thu thập của
// bindTemplateMedia và của bản xuất cũ `referencedMediaIds`.
func projectMediaIDs(project editor.Project) []string {
	ids := make(map[string]bool)
	var collect func(any)
	collect = func(value any) {
		switch v := value.(type) {
		case []any:
			for _, child := range v {
				collect(child)
			}
		case map[string]any:
			for key, child := range v {
				if key == "mediaId" || key == "coverMediaId" {
					if id, ok := child.(string); ok && id != "" {
						ids[id] = true
					}
					continue
				}
				collect(child)
			}
		}
	}
	collect(project.Timeline())
	if refs, ok := project["mediaItems"].([]any); ok {
		for _, ref := range refs {
			if object, ok := ref.(map[string]any); ok {
				if id, ok := object["id"].(string); ok && id != "" {
					ids[id] = true
				}
			}
		}
	}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	return ordered
}

// ── tiện ích ───────────────────────────────────────────────────────────────

// copyBundleMedia ghi media ra tệp tạm để lớp media đọc và probe như tệp thật,
// đồng thời kiểm sha256: tệp lệch hash là hỏng, không nhập nửa vời.
func copyBundleMedia(bundle *TemplateBundle, index int, destination string) error {
	reader, err := bundle.OpenMedia(index)
	if err != nil {
		return err
	}
	defer reader.Close()
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(reader, bundle.Media[index].Size+1))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	media := bundle.Media[index]
	if written != media.Size {
		return notTemplateFile("Tệp media "+media.FileName+" bị thiếu", bundleRecovery())
	}
	if "sha256:"+hex.EncodeToString(hash.Sum(nil)) != media.ContentHash {
		return notTemplateFile("Tệp media "+media.FileName+" bị hỏng", bundleRecovery())
	}
	return nil
}
