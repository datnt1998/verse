//go:build darwin

package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
)

// diskutilInfo là subset của `diskutil info -plist` mà phân loại ổ cần.
type diskutilInfo struct {
	Internal      bool   `json:"Internal"`
	VolumeName    string `json:"VolumeName"`
	MountPoint    string `json:"MountPoint"`
	BusProtocol   string `json:"BusProtocol"`
	Filesystem    string `json:"FilesystemType"`
	FilesystemNam string `json:"FilesystemName"`
}

// systemVolumes đọc ổ đĩa của máy: statfs cho fs/không gian/mount, `diskutil
// info -plist | plutil -convert json` để phân loại internal/external. exec
// được tiêm để test không cần ổ thật.
type systemVolumes struct {
	home     string
	statfs   func(path string, st *syscall.Statfs_t) error
	diskutil func(ctx context.Context, mountPoint string) (diskutilInfo, error)

	// Phân loại nhớ theo fsid: tên và internal/external ổn định suốt một lần
	// gắn, nên diskutil chỉ chạy lần đầu thấy fsid. Nhịp watch gọi List mỗi
	// 2 giây mà không spawn process nào.
	mu    sync.Mutex
	kinds map[[2]int32]classification
}

// classification là kết quả diskutil của một fsid; ok=false ghi nhớ cả lần
// lỗi để nhịp watch không thử lại liên tục.
type classification struct {
	info diskutilInfo
	ok   bool
}

// DefaultVolumes dựng backend ổ đĩa của hệ thống (darwin).
func DefaultVolumes(home string) Volumes {
	return &systemVolumes{
		home:   home,
		statfs: syscall.Statfs,
		diskutil: func(ctx context.Context, mountPoint string) (diskutilInfo, error) {
			return diskutilInfoFor(ctx, mountPoint)
		},
	}
}

// diskutilInfoFor gọi `diskutil info -plist <mount>` và chuyển plist → JSON
// bằng plutil của hệ thống (có sẵn trên macOS, không cần thêm thư viện plist).
func diskutilInfoFor(ctx context.Context, mountPoint string) (diskutilInfo, error) {
	info, err := exec.CommandContext(ctx, "diskutil", "info", "-plist", mountPoint).Output()
	if err != nil {
		return diskutilInfo{}, err
	}
	cmd := exec.CommandContext(ctx, "plutil", "-convert", "json", "-o", "-", "-")
	cmd.Stdin = bytes.NewReader(info)
	raw, err := cmd.Output()
	if err != nil {
		return diskutilInfo{}, err
	}
	var parsed diskutilInfo
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return diskutilInfo{}, err
	}
	return parsed, nil
}

// cstring đọc mảng int8 kết thúc-NUL của statfs.
func cstring(b []int8) string {
	raw := make([]byte, 0, len(b))
	for _, c := range b {
		if c == 0 {
			break
		}
		raw = append(raw, byte(c))
	}
	return string(raw)
}

// statfsAt statfs path hoặc tổ tiên gần nhất còn tồn tại của nó; trả statfs,
// điểm gắn và tổ tiên đã dùng.
func (v *systemVolumes) statfsAt(path string) (st syscall.Statfs_t, base string, err error) {
	probe := path
	for {
		err = v.statfs(probe, &st)
		if err == nil {
			return st, probe, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return st, probe, err
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return st, probe, err
		}
		probe = parent
	}
}

// fstypename giống statfs của một mount (không cần tổ tiên).
func (v *systemVolumes) fstypenameOf(mount string) string {
	var st syscall.Statfs_t
	if err := v.statfs(mount, &st); err != nil {
		return ""
	}
	return cstring(st.Fstypename[:])
}

// volumeFromMount dựng Volume cho một điểm gắn: statfs lấy fs + không gian,
// diskutil phân loại (lỗi diskutil → suy đoán từ mount, nhớ theo fsid).
func (v *systemVolumes) volumeFromMount(ctx context.Context, mount string) (Volume, error) {
	var st syscall.Statfs_t
	if err := v.statfs(mount, &st); err != nil {
		return Volume{}, err
	}
	return v.volumeFor(ctx, mount, st), nil
}

// volumeFor dựng Volume từ statfs đã có; phân loại chỉ chạy diskutil khi fsid
// chưa từng thấy (đường watch, không phải GET).
func (v *systemVolumes) volumeFor(ctx context.Context, mount string, st syscall.Statfs_t) Volume {
	vol := Volume{
		FS:        displayFS(cstring(st.Fstypename[:])),
		FSType:    cstring(st.Fstypename[:]),
		FreeBytes: int64(st.Bavail) * int64(st.Bsize),
		Mount:     mount,
		id:        st.Fsid.Val,
	}
	seen, ok := v.memo(st.Fsid.Val)
	if !ok {
		info, err := v.diskutil(ctx, mount)
		seen = classification{info: info, ok: err == nil}
		v.remember(st.Fsid.Val, seen)
	}
	vol.Name, vol.Kind, vol.Bus = v.label(mount, vol.FSType, seen)
	return vol
}

// probe dựng Volume cho một path bằng đúng một statfs, KHÔNG spawn process:
// tên/loại lấy từ phân loại đã nhớ, chưa có thì suy đoán từ mount (ổ vừa cắm
// được nhịp watch phân loại ngay sau đó).
func (v *systemVolumes) probe(path string) (Volume, error) {
	st, base, err := v.statfsAt(path)
	if err != nil {
		return Volume{}, err
	}
	mount := cstring(st.Mntonname[:])
	if mount == "" {
		mount = base
	}
	// Data volume của "/" gắn lặp: người dùng coi đó là ổ trong máy.
	if mount == "/System/Volumes/Data" {
		mount = "/"
	}
	seen, _ := v.memo(st.Fsid.Val)
	vol := Volume{
		FS:        displayFS(cstring(st.Fstypename[:])),
		FSType:    cstring(st.Fstypename[:]),
		FreeBytes: int64(st.Bavail) * int64(st.Bsize),
		Mount:     mount,
		id:        st.Fsid.Val,
	}
	vol.Name, vol.Kind, vol.Bus = v.label(mount, vol.FSType, seen)
	return vol, nil
}

// memo đọc phân loại đã nhớ của một fsid.
func (v *systemVolumes) memo(fsid [2]int32) (classification, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	c, ok := v.kinds[fsid]
	return c, ok
}

// remember ghi phân loại của một fsid (lần thấy đầu tiên).
func (v *systemVolumes) remember(fsid [2]int32, c classification) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.kinds == nil {
		v.kinds = map[[2]int32]classification{}
	}
	v.kinds[fsid] = c
}

// label đổi phân loại đã nhớ (hoặc suy đoán) thành tên + loại ổ + cổng nối
// (chỉ ổ ngoài đã phân loại mới có cổng).
func (v *systemVolumes) label(mount, fstype string, seen classification) (string, VolumeKind, string) {
	name := filepath.Base(mount)
	kind := guessKind(mount, fstype)
	bus := ""
	if seen.ok {
		if seen.info.VolumeName != "" {
			name = seen.info.VolumeName
		}
		kind = classifyKind(mount, fstype, seen.info)
		if kind == VolumeExternal {
			bus = seen.info.BusProtocol
		}
	}
	if name == "" || name == "/" || mount == "/" {
		name = "Macintosh HD"
	}
	return name, kind, bus
}

// networkFSTypes là các hệ tệp gắn qua mạng.
var networkFSTypes = map[string]bool{
	"smbfs": true, "nfs": true, "afpfs": true, "webdav": true,
}

// classifyKind quyết loại ổ từ fstypename, mount và diskutil.
func classifyKind(mount, fstype string, info diskutilInfo) VolumeKind {
	if networkFSTypes[fstype] {
		return VolumeNetwork
	}
	if info.Internal {
		return VolumeInternal
	}
	switch info.BusProtocol {
	case "USB", "Thunderbolt", "Secure Digital", "FireWire", "SATA":
		return VolumeExternal
	}
	return guessKind(mount, fstype)
}

// guessKind là đường lùi khi diskutil không trả lời được.
func guessKind(mount, fstype string) VolumeKind {
	if networkFSTypes[fstype] {
		return VolumeNetwork
	}
	if mount == "/" || strings.HasPrefix(mount, "/System") {
		return VolumeInternal
	}
	if strings.HasPrefix(mount, "/Volumes/") {
		return VolumeExternal
	}
	return VolumeExternal
}

// displayFS đổi fstypename của statfs sang tên hiển thị.
func displayFS(fstype string) string {
	switch fstype {
	case "apfs":
		return "APFS"
	case "hfs":
		return "HFS+"
	case "exfat":
		return "exFAT"
	case "msdos":
		return "FAT32"
	case "smbfs":
		return "SMB"
	case "nfs":
		return "NFS"
	case "afpfs":
		return "AFP"
	default:
		return fstype
	}
}

// mountRoots trả các điểm gắn người dùng nhìn thấy: "/" cộng mọi mục không ẩn
// của /Volumes, loại trùng theo fsid (Data volume của "/" bị gắn lặp thì giữ
// bản đầu).
func (v *systemVolumes) mountRoots() []string {
	mounts := []string{"/"}
	seen := map[[2]int32]bool{}
	var root syscall.Statfs_t
	if err := v.statfs("/", &root); err == nil {
		seen[root.Fsid.Val] = true
	}
	entries, err := os.ReadDir("/Volumes")
	if err != nil {
		return mounts
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") || !e.IsDir() {
			continue
		}
		mount := filepath.Join("/Volumes", e.Name())
		resolved, err := filepath.EvalSymlinks(mount)
		if err != nil {
			continue
		}
		var st syscall.Statfs_t
		if err := v.statfs(resolved, &st); err != nil {
			continue
		}
		if seen[st.Fsid.Val] {
			continue // symlink về "/" (Macintosh HD) hoặc mount lặp
		}
		seen[st.Fsid.Val] = true
		mounts = append(mounts, resolved)
	}
	return mounts
}

// List trả các ổ đang gắn hiển thị được (gốc + /Volumes).
func (v *systemVolumes) List(ctx context.Context) ([]Volume, error) {
	mounts := v.mountRoots()
	vols := make([]Volume, 0, len(mounts))
	var firstErr error
	for _, m := range mounts {
		vol, err := v.volumeFromMount(ctx, m)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		vols = append(vols, vol)
	}
	if len(vols) == 0 {
		return nil, fmt.Errorf("không đọc được ổ nào: %w", firstErr)
	}
	sort.Slice(vols, func(i, j int) bool {
		if vols[i].Kind == VolumeInternal && vols[j].Kind != VolumeInternal {
			return true
		}
		if vols[i].Kind != VolumeInternal && vols[j].Kind == VolumeInternal {
			return false
		}
		return vols[i].Mount < vols[j].Mount
	})
	return vols, nil
}

// Info trả ổ chứa path: path được canonical hoá, mount là ứng viên dài nhất
// có cùng fsid.
func (v *systemVolumes) Info(ctx context.Context, path string) (Volume, error) {
	abs, err := canonicalPath(path, v.home)
	if err != nil {
		return Volume{}, err
	}
	// statfs phải lấy st của chính path: nếu dùng biến rỗng thì mọi fsid đều
	// bằng 0 và mọi path bị coi là "mount riêng" (tên ổ sai, tạo subdir oan).
	st, _, err := v.statfsAt(abs)
	if err != nil {
		return Volume{}, err
	}
	// Ưu tiên mount sâu nhất cùng fsid (đúng cho path dưới mount lồng nhau).
	best := Volume{}
	bestLen := -1
	for _, m := range v.mountRoots() {
		if !onMount(abs, m) {
			continue
		}
		var mst syscall.Statfs_t
		if err := v.statfs(m, &mst); err != nil || mst.Fsid != st.Fsid {
			continue
		}
		if len(m) > bestLen {
			vol, err := v.volumeFromMount(ctx, m)
			if err != nil {
				return Volume{}, err
			}
			best, bestLen = vol, len(m)
		}
	}
	if bestLen < 0 {
		// Path nằm ngoài các mount người-dùng-thấy: trên macOS /private/tmp và
		// /Users là firmlink vào Data volume, nên statfs trả /System/Volumes/Data.
		// Người dùng coi đó là ổ trong máy ("/") — báo đúng như vậy.
		mount := cstring(st.Mntonname[:])
		if mount == "" {
			_, mount, _ = v.statfsAt(abs)
		}
		if mount == "/System/Volumes/Data" {
			mount = "/"
		}
		if vol, err := v.volumeFromMount(ctx, mount); err == nil {
			return vol, nil
		}
		best = Volume{
			Name:      filepath.Base(mount),
			FS:        displayFS(cstring(st.Fstypename[:])),
			FSType:    cstring(st.Fstypename[:]),
			FreeBytes: int64(st.Bavail) * int64(st.Bsize),
			Mount:     mount,
			id:        st.Fsid.Val,
			Kind:      guessKind(mount, cstring(st.Fstypename[:])),
		}
	}
	return best, nil
}

// volumeCache giữ snapshot ổ đĩa đã phân loại và không bao giờ spawn process
// trên đường request: List trả snapshot, Info là một statfs + tra fsid. Việc
// phân loại ổ mới chỉ chạy trong refreshVolumes (lúc dựng service và mỗi nhịp
// watch 2 giây).
type volumeCache struct {
	inner Volumes
	// probe (darwin) dựng Volume bằng đúng một statfs, không gọi diskutil.
	probe func(path string) (Volume, error)

	mu   sync.Mutex
	vols []Volume
	byID map[[2]int32]Volume
}

// CachedVolumes bọc backend hệ thống một snapshot: đọc từ bộ nhớ, làm mới
// ngoài đường request.
func CachedVolumes(inner Volumes) Volumes {
	c := &volumeCache{inner: inner}
	if sys, ok := inner.(*systemVolumes); ok {
		c.probe = sys.probe
	}
	return c
}

// refreshVolumes dựng lại snapshot và báo tập ổ đã đổi (cắm/rút) để service
// phát sự kiện storage. Chạy ở Start và mỗi nhịp watch; List của systemVolumes
// chỉ gọi diskutil cho fsid chưa từng thấy.
func (c *volumeCache) refreshVolumes(ctx context.Context) bool {
	vols, err := c.inner.List(ctx)
	if err != nil {
		return false
	}
	byID := make(map[[2]int32]Volume, len(vols))
	for _, v := range vols {
		byID[v.id] = v
	}
	c.mu.Lock()
	changed := c.byID != nil && !sameVolumeIDs(c.byID, byID)
	c.vols, c.byID = vols, byID
	c.mu.Unlock()
	return changed
}

// sameVolumeIDs so hai tập ổ theo fsid (không gian đổi không tính là đổi tập).
func sameVolumeIDs(a, b map[[2]int32]Volume) bool {
	if len(a) != len(b) {
		return false
	}
	for id := range a {
		if _, ok := b[id]; !ok {
			return false
		}
	}
	return true
}

func (c *volumeCache) List(context.Context) ([]Volume, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Volume, len(c.vols))
	copy(out, c.vols)
	return out, nil
}

// Info trả ổ chứa path từ snapshot: darwin dùng statfs (một syscall) rồi tra
// theo fsid, nên số trống luôn mới mà không spawn gì; backend khác khớp tiền
// tố mount dài nhất. Ổ vừa cắm chưa kịp phân loại thì tên/loại là suy đoán.
func (c *volumeCache) Info(_ context.Context, path string) (Volume, error) {
	abs, err := canonicalPath(path, "")
	if err != nil {
		return Volume{}, err
	}
	if c.probe != nil {
		vol, err := c.probe(abs)
		if err != nil {
			return Volume{}, err
		}
		c.mu.Lock()
		known, ok := c.byID[vol.id]
		c.mu.Unlock()
		if !ok {
			return vol, nil
		}
		known.FreeBytes = vol.FreeBytes
		return known, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	best := Volume{}
	bestLen := -1
	for _, v := range c.vols {
		if onMount(abs, v.Mount) && len(v.Mount) > bestLen {
			best, bestLen = v, len(v.Mount)
		}
	}
	if bestLen < 0 {
		return Volume{}, codeError(ErrNotFound, "không tìm thấy ổ cho %s", abs)
	}
	return best, nil
}
