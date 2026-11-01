// Bộ cài đặt chung cho công cụ do app quản lý. Bố cục dưới <data>/bin/<tool>/:
//
//	v-<version>/          payload đã bung và tự kiểm (bất biến)
//	current               tệp chữ ghi tên thư mục phiên bản đang dùng (commit cuối)
//	staging-<uuid>/       việc đang chạy; không bao giờ được coi là đã cài
//	install-state.json    tiến độ cho UI/CLI + phục hồi sau sập
//	cancel-request        tệp sentinel; lượt cài đang chạy huỷ khi thấy
//
// Một lần cài một lúc trong process qua khóa in-flight; ctx và tệp sentinel
// là hai đường huỷ. Lượt bị huỷ/thất bại không đụng vào phiên bản đang dùng.
package toolchain

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrCancelled đánh dấu mọi lỗi do huỷ (ctx của caller hoặc sentinel cài đặt);
// dùng errors.Is. Bộ cài và bộ chạy process con của các app cùng dùng một
// sentinel để tầng trên nhận ra lượt bị huỷ.
var ErrCancelled = errors.New("cancelled")

// CancelledErr dựng lỗi huỷ mang theo nguyên nhân của ctx (nếu có).
func CancelledErr(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%w: %w", ErrCancelled, err)
	}
	return ErrCancelled
}

// cancelledErr là dạng nội bộ của CancelledErr trong gói này.
func cancelledErr(ctx context.Context) error { return CancelledErr(ctx) }

// InstallPhase là một giai đoạn của tiến trình cài, ghi vào install-state.json.
type InstallPhase string

const (
	PhaseChecking    InstallPhase = "checking"
	PhaseDownloading InstallPhase = "downloading"
	PhaseInstalling  InstallPhase = "installing"
	PhaseSelfCheck   InstallPhase = "self-check"
	PhaseActivating  InstallPhase = "activating"
	PhaseDone        InstallPhase = "done"
	PhaseFailed      InstallPhase = "failed"
	PhaseCancelled   InstallPhase = "cancelled"
)

// InstallState là hình dạng install-state.json: tiến độ trực tiếp cho UI/CLI
// và bằng chứng phục hồi sau sập.
type InstallState struct {
	Phase      InstallPhase `json:"phase"`
	Version    string       `json:"version"`
	BytesDone  int64        `json:"bytesDone"`
	BytesTotal *int64       `json:"bytesTotal"`
	Detail     *string      `json:"detail"`
	Error      *string      `json:"error"`
	PID        int          `json:"pid"`
	StartedAt  time.Time    `json:"startedAt"`
	UpdatedAt  time.Time    `json:"updatedAt"`
}

// ChecksumError là từ chối khi SHA-256 tải về lệch manifest — tệp lỗi bị xoá.
type ChecksumError struct {
	Asset    string
	Expected string
	Actual   string
}

func (e *ChecksumError) Error() string {
	return fmt.Sprintf("checksum của %s không khớp: mong đợi %s, nhận %s", e.Asset, e.Expected, e.Actual)
}

// ToolRoot trả <baseDir>/bin/<toolId>.
func ToolRoot(baseDir, toolID string) string {
	return filepath.Join(baseDir, "bin", toolID)
}

// ToolVersionDir trả thư mục phiên bản `v-<version>` (ký tự lạ → "_").
func ToolVersionDir(baseDir, toolID, version string) string {
	clean := strings.Map(func(r rune) rune {
		if r == '.' || r == '-' || r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			return r
		}
		return '_'
	}, version)
	return filepath.Join(ToolRoot(baseDir, toolID), "v-"+clean)
}

// ActiveVersionDir trả thư mục của bản cài đang dùng, hoặc "" khi chưa có.
func ActiveVersionDir(baseDir, toolID string) string {
	root := ToolRoot(baseDir, toolID)
	data, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
		return ""
	}
	name := strings.TrimSpace(string(data))
	if name == "" || strings.ContainsAny(name, `/\`) {
		return ""
	}
	dir := filepath.Join(root, name)
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	return ""
}

// InstalledSizeFile là tên tệp ghi dung lượng cài đặt đo được sau khi xong
// (bên cạnh install-state.json trong thư mục công cụ). GET chỉ đọc số này —
// đo cây thư mục chỉ chạy trong tác vụ cài, không bao giờ trong handler.
const InstalledSizeFile = "installed-size"

// InstalledSizePath là đường tệp số đo của một công cụ.
func InstalledSizePath(baseDir, toolID string) string {
	return filepath.Join(ToolRoot(baseDir, toolID), InstalledSizeFile)
}

// WriteInstalledSize ghi số đo cài đặt (tạm + rename nguyên tử); lỗi ghi
// không lỗi chặn lượt cài nên caller chỉ cần log.
func WriteInstalledSize(baseDir, toolID string, size int64) error {
	if size <= 0 {
		return nil
	}
	root := ToolRoot(baseDir, toolID)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(root, fmt.Sprintf(".installed-size-%d.tmp", time.Now().UnixNano()))
	if err := os.WriteFile(tmp, []byte(fmt.Sprintf("%d", size)), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, InstalledSizePath(baseDir, toolID))
}

// ReadInstalledSize đọc số đo đã ghi; 0 khi vắng/hỏng/chưa đo (cài trước
// khi có tính năng đo, hoặc cài bên ngoài lượt chạy — hàng hiện “Đã cài”
// không số thay vì bịa con số).
func ReadInstalledSize(baseDir, toolID string) int64 {
	data, err := os.ReadFile(InstalledSizePath(baseDir, toolID))
	if err != nil {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// DirSizeBytes là tổng kích thước tệp dưới dir (không theo symlink); trả 0
// khi đường dẫn không tồn tại.
func DirSizeBytes(dir string) int64 {
	if dir == "" {
		return 0
	}
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return nil
		}
		info, ierr := e.Info()
		if ierr != nil {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

// Installer cài các công cụ ghim. HTTP và thư mục đích được inject nên test
// dùng httptest và t.TempDir; không có trạng thái toàn cục. Không sao chép
// Installer sau khi dùng (chứa mutex).
type Installer struct {
	Client  *http.Client // nil → http.DefaultClient
	BaseDir string
	// DownloadTimeout là trần một lượt tải asset; 0 → 30 phút. Bảo vệ trước
	// máy chủ treo giữ kết nối mở — lượt cài không thể kẹt vô hạn.
	DownloadTimeout time.Duration

	mu       sync.Mutex
	inFlight map[string]*inFlight
}

// defaultDownloadTimeout là trần tải mặc định khi caller không đặt.
const defaultDownloadTimeout = 30 * time.Minute

// InstallSpec mô tả pipeline cài của một công cụ.
type InstallSpec struct {
	// ToolID là tên tài nguyên cho khóa in-flight + thư mục bin/<id>.
	ToolID string
	Pin    ToolPin
	// Prepare làm việc thêm trong staging sau khi bung asset (dựng venv…).
	Prepare func(ctx context.Context, ic *InstallContext, staging string) error
	// SelfCheck chứng minh payload đã bung chạy được; trả phiên bản hiển thị.
	SelfCheck func(ctx context.Context, ic *InstallContext, staging string) (string, error)
	// Finalize chạy sau khi `current` lật (ví dụ sao chép giấy phép ra ngoài).
	Finalize func(ctx context.Context, ic *InstallContext, versionDir string) error
}

// InstallContext là các lời gọi lại mà lifecycle của lượt cài cung cấp cho
// Prepare/SelfCheck/Finalize.
type InstallContext struct {
	// CheckCancelled ném lỗi khi có yêu cầu huỷ (ctx hoặc sentinel).
	CheckCancelled func() error
	// Report gộp một cập nhật tiến độ vào install-state.json.
	Report func(phase InstallPhase, patch func(*InstallState))
	// BaseDir là thư mục dữ liệu của app.
	BaseDir string
}

// InstallResult tóm tắt một lượt cài.
type InstallResult struct {
	ToolID   string
	Version  string
	Path     string
	Previous string // phiên bản đang dùng trước lượt này; "" lần đầu cài
	State    string // "installed" | "already-current"
}

// inFlight giữ một lượt cài đang chạy: các caller đồng thời nhận cùng kết
// quả; lượt huỷ khi caller cuối rút đi (ctx done hoặc xong).
type inFlight struct {
	done    chan struct{}
	result  InstallResult
	err     error
	waiters int
	cancel  context.CancelFunc
}

// ReadInstallState đọc install-state.json của một công cụ; nil khi vắng/hỏng.
func (i *Installer) ReadInstallState(toolID string) *InstallState {
	data, err := os.ReadFile(filepath.Join(ToolRoot(i.BaseDir, toolID), "install-state.json"))
	if err != nil {
		return nil
	}
	var st InstallState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil
	}
	return &st
}

func writeInstallState(root string, st InstallState) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return
	}
	data, err := json.Marshal(st)
	if err != nil {
		return
	}
	tmp := filepath.Join(root, fmt.Sprintf(".install-state-%d.tmp", time.Now().UnixNano()))
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, filepath.Join(root, "install-state.json"))
}

// RequestCancel yêu cầu lượt cài đang chạy (kể cả ở process khác) dừng; được
// quan sát ở điểm kiểm tra kế tiếp.
func (i *Installer) RequestCancel(toolID string) {
	root := ToolRoot(i.BaseDir, toolID)
	_ = os.MkdirAll(root, 0o755)
	_ = os.WriteFile(filepath.Join(root, "cancel-request"), []byte(time.Now().UTC().Format(time.RFC3339Nano)), 0o644)
}

// CancelRequested đúng khi tệp sentinel cancel-request còn đứng.
func (i *Installer) CancelRequested(toolID string) bool {
	_, err := os.Stat(filepath.Join(ToolRoot(i.BaseDir, toolID), "cancel-request"))
	return err == nil
}

// claimCancelSentinel nhận nguyên tử một `cancel-request` ghi trước lượt này:
// rename hoặc thắng (cancel đã tiêu thụ, lượt phải huỷ) hoặc lỗi ENOENT
// (không có gì đang chờ). Cancel ghi sau điểm này nằm ở cancelFile, nơi các
// kiểm tra của lượt nhìn thấy — kiểm-rồi-xoá sẽ âm thầm làm mất nó.
func claimCancelSentinel(cancelFile string) (bool, error) {
	claimed := filepath.Join(filepath.Dir(cancelFile), fmt.Sprintf(".claimed-%d", time.Now().UnixNano()))
	if err := os.Rename(cancelFile, claimed); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	_ = os.Remove(claimed)
	return true, nil
}

// Sha256File băm nội dung một tệp.
func Sha256File(file string) (string, error) {
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

// DownloadVerified stream asset.URL xuống file, kiểm SHA-256 trong khi byte
// chảy về. URL `file:` được sao chép trực tiếp (test). Ném ChecksumError và
// xoá tệp khi lệch. tick chạy mỗi chunk — trả lỗi để hủy tải.
//
// Tải được tiếp sau khi app chết: byte nhận dở nằm ở <file>.part, lượt sau
// gửi Range: bytes=<size>- và băm lại phần đã có trước khi nối. Máy chủ
// không hỗ trợ Range (200) thì tải lại từ đầu. Huỷ/lỗi mạng giữ .part; chỉ
// checksum lệch mới xoá để lượt sau không kế thừa dữ liệu sai.
func (i *Installer) DownloadVerified(ctx context.Context, asset PinnedAsset, file string, tick func(done int64) error) error {
	parentCtx := ctx
	dlTimeout := i.DownloadTimeout
	if dlTimeout <= 0 {
		dlTimeout = defaultDownloadTimeout
	}
	var rngCancel context.CancelFunc
	ctx, rngCancel = context.WithTimeout(ctx, dlTimeout)
	defer rngCancel()
	// Phân biệt ba kết thúc: huỷ của caller (ErrCancelled), tải hết hạn (lỗi
	// thường → phase failed) và lỗi truyền thô.
	ctxErr := func(fallback error) error {
		if parentCtx.Err() != nil {
			return cancelledErr(parentCtx)
		}
		if ctx.Err() != nil {
			return fmt.Errorf("tải %s hết %s: %w", asset.Name, dlTimeout, ctx.Err())
		}
		return fallback
	}

	part := file + ".part"
	var offset int64
	h := sha256.New()
	if st, err := os.Stat(part); err == nil && st.Size() > 0 {
		if err := hashFile(part, h); err != nil {
			return err
		}
		offset = st.Size()
	}

	var src io.ReadCloser
	resumed := false
	if strings.HasPrefix(asset.URL, "file://") {
		u, err := url.Parse(asset.URL)
		if err != nil {
			return err
		}
		f, err := os.Open(u.Path)
		if err != nil {
			return err
		}
		src = f
		offset = 0
		h = sha256.New()
	} else {
		client := i.Client
		if client == nil {
			client = http.DefaultClient
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
		if err != nil {
			return err
		}
		if offset > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
		}
		res, err := client.Do(req)
		if err != nil {
			return ctxErr(err)
		}
		if offset > 0 && res.StatusCode == http.StatusPartialContent {
			resumed = true
		} else if offset > 0 && res.StatusCode == http.StatusOK {
			// Máy chủ bỏ qua Range: bắt đầu lại, bỏ .part cũ.
			offset = 0
			h = sha256.New()
		} else if res.StatusCode < 200 || res.StatusCode >= 300 || res.Body == nil {
			res.Body.Close()
			return fmt.Errorf("tải thất bại: HTTP %d %s", res.StatusCode, asset.URL)
		}
		src = res.Body
	}
	defer src.Close()

	flags := os.O_CREATE | os.O_WRONLY
	if resumed {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	out, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return err
	}
	buf := make([]byte, 256*1024)
	done := offset
	copyErr := func() error {
		for {
			n, rerr := src.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				h.Write(chunk)
				if _, werr := out.Write(chunk); werr != nil {
					return werr
				}
				done += int64(n)
				if tick != nil {
					if terr := tick(done); terr != nil {
						return terr
					}
				}
			}
			if rerr != nil {
				if rerr == io.EOF {
					return nil
				}
				return rerr
			}
		}
	}()
	closeErr := out.Close()
	if copyErr != nil {
		return ctxErr(copyErr)
	}
	if closeErr != nil {
		return ctxErr(closeErr)
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, asset.SHA256) {
		_ = os.Remove(part)
		return &ChecksumError{Asset: asset.Name, Expected: asset.SHA256, Actual: actual}
	}
	if err := os.Rename(part, file); err != nil {
		return err
	}
	return nil
}

// hashFile băm toàn bộ tệp vào h (phần .part đã tải trước khi nối).
func hashFile(file string, h io.Writer) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(h, f)
	return err
}

// partialPath là chỗ giữ byte dở của một asset, bền qua các lần chạy: khoá
// theo SHA-256 để tên trùng giữa các công cụ không đụng nhau.
func (i *Installer) partialPath(asset PinnedAsset) string {
	key := asset.SHA256
	if len(key) > 16 {
		key = key[:16]
	}
	if key == "" {
		key = "unknown"
	}
	return filepath.Join(i.BaseDir, "downloads", key+"-"+asset.Name)
}

// unpackAsset giải quyết một asset đã tải vào staging theo Kind.
func unpackAsset(asset PinnedAsset, archive, staging string) error {
	dest := filepath.Join(staging, asset.Name)
	switch asset.Kind {
	case AssetFile:
		if err := os.MkdirAll(staging, 0o755); err != nil {
			return err
		}
		if err := os.Rename(archive, dest); err != nil {
			return err
		}
	case AssetTarGZ, AssetZip:
		if asset.Member != "" {
			// Trích một member: bung vào thư con tạm rồi đổi tên vào đích —
			// tương đương `mv` của bản cũ (member thư mục được mang nguyên cây).
			tmp := filepath.Join(staging, fmt.Sprintf(".unpack-%d", time.Now().UnixNano()))
			if err := os.MkdirAll(tmp, 0o755); err != nil {
				return err
			}
			defer os.RemoveAll(tmp)
			if err := extractArchive(asset.Kind, archive, tmp, asset.Member); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			if err := os.Rename(filepath.Join(tmp, filepath.FromSlash(asset.Member)), dest); err != nil {
				return err
			}
		} else {
			if err := extractArchive(asset.Kind, archive, staging, ""); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("loại asset không hỗ trợ: %s", asset.Kind)
	}
	if asset.Executable {
		if info, err := os.Stat(dest); err == nil && info.Mode().IsRegular() {
			_ = os.Chmod(dest, 0o755)
		}
	}
	return nil
}

// ExtractArchive bung archive KindTarGZ/KindZip vào dest; member khác rỗng thì
// chỉ trích đúng member đó. Đây là phần dùng chung cho các app tự quản mã
// nguồn (Noverse tải tarball của bộ đọc) nên phải đi qua đúng đường kiểm
// ZipSlip như bộ cài.
func ExtractArchive(kind AssetKind, archive, dest, member string) error {
	return extractArchive(kind, archive, dest, member)
}

// extractArchive bung archive KindTarGZ/KindZip vào dest. Toàn bộ trích xuất
// đi qua os.Root nên member với ".." hay symlink thoát khỏi dest bị từ chối
// (ZipSlip), kể cả trường hợp symlink trước/file sau trong cùng một archive.
func extractArchive(kind AssetKind, archive, dest, member string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer root.Close()
	if kind == AssetTarGZ {
		return untarGz(archive, root, member)
	}
	return unzip(archive, root, member)
}

// symlinkTarget kiểm target của một link trong archive: tuyệt đối hoặc trỏ
// ra ngoài dest đều bị từ chối (os.Root chặn write-through nhưng link thoát
// vẫn được tạo nếu không kiểm đây).
func symlinkTarget(target string) error {
	clean := filepath.Clean(filepath.FromSlash(target))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("symlink archive không an toàn: %s", target)
	}
	return nil
}

// localMember chuyển tên member archive (dấu / ) sang đường dẫn local hợp lệ
// dưới root: từ chối tuyệt đối và "..".
func localMember(name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("đường dẫn archive không an toàn: %s", name)
	}
	return clean, nil
}

// untarGz bung một .tar.gz vào root; member khác rỗng thì chỉ trích đúng member
// đó (vẫn giữ đường dẫn tương đối của nó dưới root).
func untarGz(archive string, root *os.Root, member string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if member != "" && hdr.Name != member {
			continue
		}
		out, err := localMember(hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(out, fs.FileMode(hdr.Mode)&0o777); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := root.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return err
			}
			w, err := root.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fs.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, tr); err != nil {
				w.Close()
				return err
			}
			if err := w.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := symlinkTarget(hdr.Linkname); err != nil {
				return err
			}
			if err := root.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return err
			}
			_ = root.Remove(out)
			if err := root.Symlink(hdr.Linkname, out); err != nil {
				return err
			}
		}
	}
	return nil
}

// unzip bung một .zip vào root, phục hồi cả symlink (creator unix của zip
// giữ mode 0777 << 16 cho link trong .app bundle).
func unzip(archive string, root *os.Root, member string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, zf := range r.File {
		if member != "" && zf.Name != member {
			continue
		}
		out, err := localMember(zf.Name)
		if err != nil {
			return err
		}
		mode := zf.Mode()
		switch {
		case mode.IsDir() || strings.HasSuffix(zf.Name, "/"):
			if err := root.MkdirAll(out, mode.Perm()|0o755); err != nil {
				return err
			}
		case mode&os.ModeSymlink != 0:
			rc, err := zf.Open()
			if err != nil {
				return err
			}
			target, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return err
			}
			if err := symlinkTarget(string(target)); err != nil {
				return err
			}
			if err := root.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return err
			}
			_ = root.Remove(out)
			if err := root.Symlink(string(target), out); err != nil {
				return err
			}
		default:
			rc, err := zf.Open()
			if err != nil {
				return err
			}
			if err := root.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				rc.Close()
				return err
			}
			perm := mode.Perm()
			if perm == 0 {
				perm = 0o644
			}
			w, err := root.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
			if err != nil {
				rc.Close()
				return err
			}
			_, copyErr := io.Copy(w, rc)
			rc.Close()
			if err := w.Close(); err != nil {
				return err
			}
			if copyErr != nil {
				return copyErr
			}
		}
	}
	return nil
}

// joinFlight chia một lượt chạy nặng giữa các caller cùng toolID. Caller đầu
// tạo lượt; caller sau gộp vào và nhận cùng kết quả; lượt bị huỷ khi caller
// cuối rút đi mà chưa xong (không ai còn chờ kết quả). Trả flight đã xong để
// caller đọc result/err, hoặc lỗi khi ctx của riêng caller đó hết hạn.
func (i *Installer) joinFlight(ctx context.Context, toolID string, start func(runCtx context.Context, f *inFlight)) (*inFlight, error) {
	if err := ctx.Err(); err != nil {
		return nil, cancelledErr(ctx)
	}
	i.mu.Lock()
	if i.inFlight == nil {
		i.inFlight = map[string]*inFlight{}
	}
	flight, ok := i.inFlight[toolID]
	if !ok {
		runCtx, cancel := context.WithCancel(context.Background())
		flight = &inFlight{done: make(chan struct{}), cancel: cancel}
		i.inFlight[toolID] = flight
		go func() {
			start(runCtx, flight)
			close(flight.done)
			i.mu.Lock()
			delete(i.inFlight, toolID)
			i.mu.Unlock()
		}()
	}
	flight.waiters++
	i.mu.Unlock()
	defer func() {
		i.mu.Lock()
		flight.waiters--
		if flight.waiters == 0 {
			select {
			case <-flight.done:
			default:
				flight.cancel()
			}
		}
		i.mu.Unlock()
	}()
	select {
	case <-flight.done:
		return flight, nil
	case <-ctx.Done():
		return nil, cancelledErr(ctx)
	}
}

// Install chạy pipeline cài của spec. Lời gọi đồng thời cho cùng công cụ gộp
// vào lượt đang chạy (mỗi caller vẫn rút được bằng ctx của mình).
func (i *Installer) Install(ctx context.Context, spec InstallSpec) (InstallResult, error) {
	flight, err := i.joinFlight(ctx, spec.ToolID, func(runCtx context.Context, f *inFlight) {
		f.result, f.err = i.runInstall(runCtx, spec)
	})
	if err != nil {
		return InstallResult{}, err
	}
	return flight.result, flight.err
}

// InstallInProgress đúng khi một lượt cài của toolID đang chạy trong process
// này (install-state.json có thể trễ).
func (i *Installer) InstallInProgress(toolID string) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	_, ok := i.inFlight[toolID]
	return ok
}

// FlightWaiters đếm số caller đang chờ lượt cài của toolID (0 khi không có
// lượt). Dùng để quan sát việc gộp lời gọi đồng thời.
func (i *Installer) FlightWaiters(toolID string) int {
	i.mu.Lock()
	defer i.mu.Unlock()
	if f := i.inFlight[toolID]; f != nil {
		return f.waiters
	}
	return 0
}

func (i *Installer) runInstall(ctx context.Context, spec InstallSpec) (InstallResult, error) {
	root := ToolRoot(i.BaseDir, spec.ToolID)
	staging := filepath.Join(root, fmt.Sprintf("staging-%d", time.Now().UnixNano()))
	cancelFile := filepath.Join(root, "cancel-request")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return InstallResult{}, err
	}
	// Nhận nguyên tử một cancel-request đang đợi (bản cũ: pre-start sentinel
	// hủy ngay lượt mới). Cancel ghi sau điểm này thuộc lượt này.
	preCancel, err := claimCancelSentinel(cancelFile)
	if err != nil {
		return InstallResult{}, err
	}

	state := InstallState{
		Phase:     PhaseChecking,
		Version:   spec.Pin.Version,
		PID:       os.Getpid(),
		StartedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	var stateMu sync.Mutex
	report := func(phase InstallPhase, patch func(*InstallState)) {
		stateMu.Lock()
		defer stateMu.Unlock()
		state.Phase = phase
		if patch != nil {
			patch(&state)
		}
		state.UpdatedAt = time.Now().UTC()
		writeInstallState(root, state)
	}
	// Watcher: một cancel-request ghi giữa chừng phải cắt đang tải (HTTP body
	// đang chảy), không chỉ đợi tới điểm kiểm tra kế tiếp.
	ctx, cancelWatch := context.WithCancel(ctx)
	defer cancelWatch()
	go func() {
		tick := time.NewTicker(200 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if fileExists(cancelFile) {
					cancelWatch()
					return
				}
			}
		}
	}()
	checkCancelled := func() error {
		if ctx.Err() != nil || fileExists(cancelFile) {
			return cancelledErr(ctx)
		}
		return nil
	}
	ic := &InstallContext{CheckCancelled: checkCancelled, Report: report, BaseDir: i.BaseDir}

	cleanup := func() {
		_ = os.RemoveAll(staging)
		_ = os.Remove(cancelFile)
	}

	fail := func(err error) (InstallResult, error) {
		phase := PhaseFailed
		if errors.Is(err, ErrCancelled) {
			phase = PhaseCancelled
		}
		msg := err.Error()
		report(phase, func(s *InstallState) { s.Error = &msg })
		cleanup()
		return InstallResult{}, err
	}

	if preCancel {
		return fail(cancelledErr(ctx))
	}
	if err := checkCancelled(); err != nil {
		return fail(err)
	}
	assets, err := PinnedAssets(spec.Pin, CurrentPlatformKey())
	if err != nil {
		return fail(err)
	}
	var total int64
	for _, a := range assets {
		total += a.SizeBytes
	}
	report(PhaseChecking, func(s *InstallState) {
		s.Version = spec.Pin.Version
		if total > 0 {
			s.BytesTotal = &total
		}
	})

	// Cùng phiên bản đã active và không có bước prepare: tự kiểm coi như còn mới.
	current := ActiveVersionDir(i.BaseDir, spec.ToolID)
	if current == ToolVersionDir(i.BaseDir, spec.ToolID, spec.Pin.Version) && spec.Prepare == nil {
		report(PhaseDone, func(s *InstallState) { d := "already current"; s.Detail = &d })
		cleanup()
		return InstallResult{
			ToolID:   spec.ToolID,
			Version:  spec.Pin.Version,
			Path:     current,
			Previous: spec.Pin.Version,
			State:    "already-current",
		}, nil
	}
	previousVersion := ""
	if current != "" {
		if data, err := os.ReadFile(filepath.Join(current, "installed.json")); err == nil {
			var marker struct {
				Version *string `json:"version"`
			}
			if json.Unmarshal(data, &marker) == nil && marker.Version != nil {
				previousVersion = *marker.Version
			}
		}
		if previousVersion == "" {
			previousVersion = strings.TrimPrefix(filepath.Base(current), "v-")
		}
	}

	if free := freeDiskBytes(root); free >= 0 && free < spec.Pin.RequiredBytes {
		return fail(fmt.Errorf("không đủ dung lượng: còn %d MB, cần %d MB", free/1024/1024, spec.Pin.RequiredBytes/1024/1024))
	}

	if err := os.MkdirAll(staging, 0o755); err != nil {
		return fail(err)
	}
	var bytesDone int64
	for _, asset := range assets {
		if err := checkCancelled(); err != nil {
			return fail(err)
		}
		archive := i.partialPath(asset)
		if err := os.MkdirAll(filepath.Dir(archive), 0o755); err != nil {
			return fail(err)
		}
		report(PhaseDownloading, func(s *InstallState) { d := asset.Name; s.Detail = &d })
		// n là số byte đã tải của asset này, không phải delta: cộng vào tổng
		// các asset đã xong thay vì tự cộng dồn (lỗi bội số trước đây).
		var assetDone int64
		err := i.DownloadVerified(ctx, asset, archive, func(n int64) error {
			assetDone = n
			report(PhaseDownloading, func(s *InstallState) { s.BytesDone = bytesDone + n })
			return checkCancelled()
		})
		if err != nil {
			return fail(err)
		}
		bytesDone += assetDone
		if err := checkCancelled(); err != nil {
			return fail(err)
		}
		if err := unpackAsset(asset, archive, staging); err != nil {
			return fail(err)
		}
		_ = os.Remove(archive)
	}

	report(PhaseInstalling, nil)
	if spec.Prepare != nil {
		if err := spec.Prepare(ctx, ic, staging); err != nil {
			return fail(err)
		}
	}
	if err := checkCancelled(); err != nil {
		return fail(err)
	}

	report(PhaseSelfCheck, nil)
	version, err := spec.SelfCheck(ctx, ic, staging)
	if err != nil {
		return fail(err)
	}
	if err := checkCancelled(); err != nil {
		return fail(err)
	}

	report(PhaseActivating, nil)
	versionDir := ToolVersionDir(i.BaseDir, spec.ToolID, spec.Pin.Version)
	if err := os.RemoveAll(versionDir); err != nil {
		return fail(err)
	}
	if err := os.Rename(staging, versionDir); err != nil {
		return fail(err)
	}
	// Marker commit: `current` lật cuối để sập giữa rename không cài nửa chừng.
	tmpMarker := filepath.Join(root, fmt.Sprintf(".current-%d.tmp", time.Now().UnixNano()))
	if err := os.WriteFile(tmpMarker, []byte(filepath.Base(versionDir)+"\n"), 0o644); err != nil {
		return fail(err)
	}
	if err := os.Rename(tmpMarker, filepath.Join(root, "current")); err != nil {
		return fail(err)
	}
	markerData, _ := json.MarshalIndent(map[string]string{
		"toolId":      spec.ToolID,
		"version":     version,
		"pin":         spec.Pin.Version,
		"release":     spec.Pin.ReleaseURL,
		"license":     spec.Pin.License.SPDX,
		"installedAt": time.Now().UTC().Format(time.RFC3339Nano),
		"platform":    currentPlatformString(),
	}, "", "  ")
	if err := os.WriteFile(filepath.Join(versionDir, "installed.json"), markerData, 0o644); err != nil {
		return fail(err)
	}

	if spec.Finalize != nil {
		if err := spec.Finalize(ctx, ic, versionDir); err != nil {
			return fail(err)
		}
	}
	report(PhaseDone, func(s *InstallState) { s.Detail = nil; s.Error = nil })
	cleanup()
	return InstallResult{
		ToolID:   spec.ToolID,
		Version:  version,
		Path:     versionDir,
		Previous: previousVersion,
		State:    "installed",
	}, nil
}

// LifecycleContext là các lời gọi lại của một lifecycle tự viết.
type LifecycleContext struct {
	// Check ném lỗi khi có yêu cầu huỷ.
	Check func() error
	// State ghi một cập nhật giai đoạn vào install-state.json.
	State func(phase InstallPhase, detail, errMsg string)
}

// RunLifecycle là phần lifecycle chung cho công cụ kiểu môi trường
// (transcription): khóa in-flight, state file, sentinel huỷ — caller cung cấp
// thân của các giai đoạn.
func (i *Installer) RunLifecycle(ctx context.Context, toolID, version string, run func(runCtx context.Context, lc *LifecycleContext) error) error {
	flight, err := i.joinFlight(ctx, toolID, func(runCtx context.Context, f *inFlight) {
		f.err = i.runLifecycle(runCtx, toolID, version, run)
	})
	if err != nil {
		return err
	}
	return flight.err
}

func (i *Installer) runLifecycle(ctx context.Context, toolID, version string, run func(runCtx context.Context, lc *LifecycleContext) error) error {
	root := ToolRoot(i.BaseDir, toolID)
	cancelFile := filepath.Join(root, "cancel-request")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	preCancel, err := claimCancelSentinel(cancelFile)
	if err != nil {
		return err
	}
	check := func() error {
		if ctx.Err() != nil || fileExists(cancelFile) {
			return cancelledErr(ctx)
		}
		return nil
	}
	// ctx ở đây là runCtx của lượt chia sẻ — caller cuối rút đi hủy được.
	state := func(phase InstallPhase, detail, errMsg string) {
		prev := i.ReadInstallState(toolID)
		started := time.Now().UTC()
		if prev != nil {
			started = prev.StartedAt
		}
		st := InstallState{
			Phase:     phase,
			Version:   version,
			PID:       os.Getpid(),
			StartedAt: started,
			UpdatedAt: time.Now().UTC(),
		}
		if detail != "" {
			st.Detail = &detail
		}
		if errMsg != "" {
			st.Error = &errMsg
		}
		writeInstallState(root, st)
	}
	defer os.Remove(cancelFile)
	if preCancel {
		err := cancelledErr(ctx)
		state(PhaseCancelled, "", err.Error())
		return err
	}
	if err := check(); err != nil {
		state(PhaseCancelled, "", err.Error())
		return err
	}
	if err := run(ctx, &LifecycleContext{Check: check, State: state}); err != nil {
		phase := PhaseFailed
		if errors.Is(err, ErrCancelled) {
			phase = PhaseCancelled
		}
		state(phase, "", err.Error())
		return err
	}
	state(PhaseDone, "", "")
	return nil
}

// FileExists đúng khi path tồn tại (tệp hoặc thư mục). Dùng chung cho mọi
// app: tầng trên cần cùng một định nghĩa "đã có" với bộ cài.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func currentPlatformString() string {
	key := CurrentPlatformKey()
	if key == "" {
		return "unknown/unknown"
	}
	return string(key)
}
