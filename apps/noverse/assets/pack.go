package assets

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed packs
var packsFS embed.FS

// Pack biểu thị một gói thể loại (thư mục `packs/<tên>/`), chọn qua khoá `style`.
// Thứ tự nguồn giống overlayStyles: nhúng → ~/.ainovel/style/packs → <sách>/style/packs;
// tầng sau thay nguyên thư mục của tầng trước (không trộn từng file).
type Pack struct {
	Name      string     // tên thư mục gói (= giá trị `style`)
	Ten       string     // tên hiển thị
	TenNgan   string     // tên ngắn cho thẻ gói (goi.json `ten_ngan`); rỗng thì UI tự rút gọn `Ten`
	MoTa      string     // một dòng mô tả thể loại (goi.json `mo_ta`); rỗng thì UI không hiện
	SoChuong  int        // số chương chuẩn của thể loại
	NguoiNghe string     // nhóm người nghe chính (dùng cho P1-2)
	Thoai     *PackThoai // khoảng % thoại mỗi chương (omitempty khi gói không khai)
	// TiengMoiChuong là khoảng số tiếng mỗi chương (goi.json `tieng_moi_chuong`),
	// khớp dòng "Độ dài" trong luật bộ của gói; nil khi gói không khai.
	TiengMoiChuong *PackTiengMoiChuong
	Voice          map[string]*string // ghi đè mục voice: null = tắt mục, chuỗi = thay nội dung
	Luat           []PackRuleFile     // các file luật nạp nguyên văn, theo đúng thứ tự goi.json khai
	VanPhong       string             // nội dung van-phong.md (tuỳ chọn); rỗng nếu gói không có
	Raw            json.RawMessage    // goi.json nguyên byte — các lát sau đọc section riêng qua Section()
	files          fs.FS
}

// Section giải mã một khoá top-level của goi.json vào v. Trả false khi gói
// không khai khoá đó; lỗi khi JSON của khoá hỏng. Đây là điểm mở rộng chung:
// mỗi lát sau sở hữu khoá section của riêng mình và không cần sửa pack.go —
// `fatigue_words` (ngưỡng theo 1.000 tiếng), `do_do` (phép đo), `giu_chan`
// (trường outline bắt buộc), `audio` (giọng/từ điển mặc định).
func (p *Pack) Section(key string, v any) (bool, error) {
	if p == nil || len(p.Raw) == 0 {
		return false, nil
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(p.Raw, &top); err != nil {
		return false, fmt.Errorf("goi.json không hợp lệ: %w", err)
	}
	raw, ok := top[key]
	if !ok {
		return false, nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return true, fmt.Errorf("khoá %q trong goi.json không hợp lệ: %w", key, err)
	}
	return true, nil
}

// ReadFile đọc file trong đúng nguồn của gói (không trộn các tầng ghi đè).
func (p *Pack) ReadFile(name string) ([]byte, error) {
	if p == nil || p.files == nil {
		return nil, fs.ErrNotExist
	}
	return fs.ReadFile(p.files, name)
}

// PackThoai là khoảng phần trăm thoại mỗi chương mà gói quy định.
type PackThoai struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// PackTiengMoiChuong là khoảng số tiếng mỗi chương mà gói quy định
// (goi.json `tieng_moi_chuong`). Cùng hình dạng với PackThoai nhưng đơn vị là
// tiếng, không phải phần trăm — đừng gộp hai thứ khác nghĩa vào một kiểu.
type PackTiengMoiChuong struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// PackRuleFile là một file luật trong thư mục luat/ của gói.
type PackRuleFile struct {
	Name    string // tên file trong luat/ (ví dụ "master-rules-v3.md")
	Content string // nội dung nguyên văn, đã TrimSpace hai đầu
}

// packManifest là biểu diễn JSON của goi.json. Trường lạ bị bỏ qua để các lát
// sau thêm field không phá gói cũ; "voice" dùng con trỏ để phân biệt null (tắt
// mục) với chuỗi thay thế.
type packManifest struct {
	Ten            string              `json:"ten"`
	TenNgan        string              `json:"ten_ngan"`
	MoTa           string              `json:"mo_ta"`
	SoChuong       int                 `json:"so_chuong"`
	NguoiNghe      string              `json:"nguoi_nghe"`
	Thoai          *PackThoai          `json:"thoai"`
	TiengMoiChuong *PackTiengMoiChuong `json:"tieng_moi_chuong"`
	Voice          map[string]*string  `json:"voice"`
	Luat           []string            `json:"luat"`
}

// loadPack tìm gói thể loại trùng tên `style`. Không có thư mục gói nào thì trả
// (nil, nil) — caller rơi về styles/<tên>.md như cũ. Gói có mặt mà lỗi
// (goi.json hỏng, thiếu file luat) thì trả lỗi tiếng Việt nêu tên gói; lỗi gói
// là lỗi cấu hình của người dùng, không được panic cả tiến trình.
func loadPack(style string, opts LoadOptions) (*Pack, error) {
	if !styleNameRe.MatchString(style) {
		return nil, nil
	}
	var root fs.FS
	if dirExists(packsFS, "packs/"+style) {
		sub, err := fs.Sub(packsFS, "packs/"+style)
		if err != nil {
			return nil, fmt.Errorf("gói thể loại %q nhúng không đọc được: %w", style, err)
		}
		root = sub
	}
	for _, base := range []string{opts.HomeStyleDir, opts.BookStyleDir} {
		if base == "" {
			continue
		}
		dir := filepath.Join(base, "packs", style)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			root = os.DirFS(dir)
		}
	}
	if root == nil {
		return nil, nil
	}
	pack, err := parsePack(style, root)
	if err != nil {
		return nil, fmt.Errorf("gói thể loại %q: %w", style, err)
	}
	return pack, nil
}

func dirExists(fsys fs.FS, dir string) bool {
	info, err := fs.Stat(fsys, dir)
	return err == nil && info.IsDir()
}

// parsePack đọc goi.json rồi nạp từng file trong danh sách `luat` (nguyên văn)
// và van-phong.md nếu có.
func parsePack(name string, fsys fs.FS) (*Pack, error) {
	data, err := fs.ReadFile(fsys, "goi.json")
	if err != nil {
		return nil, fmt.Errorf("thiếu goi.json: %w", err)
	}
	var m packManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("goi.json không hợp lệ: %w", err)
	}
	pack := &Pack{
		Name:           name,
		Ten:            m.Ten,
		TenNgan:        strings.TrimSpace(m.TenNgan),
		MoTa:           strings.TrimSpace(m.MoTa),
		SoChuong:       m.SoChuong,
		NguoiNghe:      m.NguoiNghe,
		Thoai:          m.Thoai,
		TiengMoiChuong: m.TiengMoiChuong,
		Voice:          m.Voice,
		Raw:            append(json.RawMessage(nil), data...),
		files:          fsys,
	}
	for _, file := range m.Luat {
		if strings.ContainsAny(file, `/\`) || file == "" || file == "." || file == ".." {
			return nil, fmt.Errorf("tên file luat không hợp lệ: %q", file)
		}
		content, err := fs.ReadFile(fsys, "luat/"+file)
		if err != nil {
			return nil, fmt.Errorf("thiếu file luat/%s: %w", file, err)
		}
		pack.Luat = append(pack.Luat, PackRuleFile{Name: file, Content: strings.TrimSpace(string(content))})
	}
	if vp, err := fs.ReadFile(fsys, "van-phong.md"); err == nil {
		pack.VanPhong = strings.TrimSpace(string(vp))
	}
	return pack, nil
}

// applyPackVoice ráp voice theo khoá, áp ghi đè của gói (P0-6) rồi mới tới
// 3 tầng resolveAppendable. Khoá lạ trong goi.json là lỗi nạp — trả error tiếng
// Việt nêu tên gói và khoá để lỗi chính tả lộ ngay lúc khởi động.
func applyPackVoice(sections []VoiceSection, pack *Pack) (string, error) {
	var overrides map[string]*string
	if pack != nil && len(pack.Voice) > 0 {
		overrides = pack.Voice
	}
	voice, err := AssembleVoice(sections, overrides)
	if err != nil {
		name := ""
		if pack != nil {
			name = pack.Name
		}
		return "", fmt.Errorf("gói thể loại %q: %w", name, err)
	}
	return voice, nil
}

// packRulesHeading là mở đầu của khối luật gói thể loại trong system prompt,
// kèm ghi chú thứ tự ưu tiên theo PRD §Nguồn chuẩn (S2b/P0-3).
const packRulesHeading = `## Luật gói thể loại (nạp nguyên văn)

Thứ tự ưu tiên khi mâu thuẫn: Master > gói thể loại (trừ mục ghi "Ghi đè §x") > quy tắc viết của người dùng (chỉ được đè mục [CẢNH BÁO] hoặc không nhãn, không được đè [LỖI]) > văn phong mặc định. Riêng luật giao thức thực thi (thứ tự công cụ, checkpoint, commit) không bị ghi đè.`

// RulesSection dựng khối luật của gói để nối vào system prompt của một vai:
// heading + ghi chú ưu tiên, rồi từng file luat/ nguyên văn theo thứ tự khai.
// Gói không có luật trả chuỗi rỗng.
func (p *Pack) RulesSection() string {
	if p == nil || len(p.Luat) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(packRulesHeading)
	for _, f := range p.Luat {
		b.WriteString("\n\n---\n\n<!-- nguồn: luat/")
		b.WriteString(f.Name)
		b.WriteString(" -->\n\n")
		b.WriteString(f.Content)
	}
	return b.String()
}

// AppendPackRules nối khối luật gói vào cuối system prompt của một vai
// (writer, editor và mọi biến thể architect). Không có gói hoặc gói không có
// luật thì prompt giữ nguyên.
func AppendPackRules(systemPrompt string, pack *Pack) string {
	section := pack.RulesSection()
	if section == "" {
		return systemPrompt
	}
	return systemPrompt + "\n\n" + section
}

// ListPacks liệt kê mọi gói đang có: nhúng sẵn + tầng home (~/.ainovel/style/packs),
// home ghi đè nguyên gói nhúng cùng tên — giống quy tắc của loadPack. Gói hỏng
// (goi.json lỗi, thiếu luat) bị bỏ qua kèm cảnh báo, không làm hỏng danh sách.
// Phục vụ endpoint liệt kê style của API; tầng sách không thuộc danh sách này.
func ListPacks(opts LoadOptions) []*Pack {
	byName := make(map[string]*Pack)
	if entries, err := packsFS.ReadDir("packs"); err == nil {
		for _, e := range entries {
			if !e.IsDir() || !styleNameRe.MatchString(e.Name()) {
				continue
			}
			sub, err := fs.Sub(packsFS, "packs/"+e.Name())
			if err != nil {
				continue
			}
			if p, err := parsePack(e.Name(), sub); err == nil {
				byName[e.Name()] = p
			} else {
				slog.Warn("bỏ qua gói thể loại nhúng lỗi", "module", "assets", "pack", e.Name(), "err", err)
			}
		}
	}
	if opts.HomeStyleDir != "" {
		dir := filepath.Join(opts.HomeStyleDir, "packs")
		if entries, err := os.ReadDir(dir); err == nil {
			for _, e := range entries {
				if !e.IsDir() || !styleNameRe.MatchString(e.Name()) {
					continue
				}
				if p, err := parsePack(e.Name(), os.DirFS(filepath.Join(dir, e.Name()))); err == nil {
					byName[e.Name()] = p
				} else {
					slog.Warn("bỏ qua gói thể loại lỗi", "module", "assets", "pack", e.Name(), "err", err)
				}
			}
		}
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]*Pack, 0, len(names))
	for _, name := range names {
		out = append(out, byName[name])
	}
	return out
}
