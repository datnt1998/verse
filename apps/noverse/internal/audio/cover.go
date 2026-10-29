package audio

// Gói AF / A7: GET S/cover?size= — bìa truyện đã tải (Tư liệu › Bìa truyện,
// project asset dir `bia/`) hoặc bìa tự tạo (tên truyện + tập + Noverse Audio
// trên nền thương hiệu — KHÔNG chữ viết tắt). Vuông, PNG, có cache trên đĩa
// (audio/bia/<key>.png): đổi tên truyện/tập/bìa tải lên → key đổi → render lại.
//
// 渲染用 go:embed 的 Be Vietnam Pro (WOFF → sfnt): latin + latin-ext + vietnamese
// 三个子集，逐 rune 选 face。NFC 归一化避免越南语组合符走形。

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // decode support
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	imagedraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	_ "golang.org/x/image/webp"
	"golang.org/x/text/unicode/norm"
)

//go:embed assets/be-vietnam-pro-latin-700-normal.woff
var woffLatin []byte

//go:embed assets/be-vietnam-pro-latin-ext-700-normal.woff
var woffLatinExt []byte

//go:embed assets/be-vietnam-pro-vietnamese-700-normal.woff
var woffVietnamese []byte

// coverMasterSize 是 bìa tự tạo 与 APIC/covr 的母版尺寸（A7: "vuông 1400 px"）。
const coverMasterSize = 1400

// coverCacheVersion 变了 → 旧缓存整批作废（调色/版式/字体变化时 +1）。
const coverCacheVersion = "v2"

var errBadCover = errors.New("bìa không đọc được")

var (
	coverFacesOnce sync.Once
	coverFaces     []*sfnt.Font
	coverFacesErr  error
)

// loadCoverFaces 解码三个 WOFF 子集 → sfnt 列表（latin, latin-ext, vietnamese）。
func loadCoverFaces() ([]*sfnt.Font, error) {
	coverFacesOnce.Do(func() {
		for _, data := range [][]byte{woffLatin, woffLatinExt, woffVietnamese} {
			sfntBytes, err := decodeWOFF(data)
			if err != nil {
				coverFacesErr = err
				return
			}
			f, err := sfnt.Parse(sfntBytes)
			if err != nil {
				coverFacesErr = err
				return
			}
			coverFaces = append(coverFaces, f)
		}
	})
	return coverFaces, coverFacesErr
}

// faceForRune 逐 rune 选有 glyph 的 face（latin 缺 đ/组合声调时落到后两个子集）。
func faceForRune(faces []*sfnt.Font, r rune) int {
	for i, f := range faces {
		idx, err := f.GlyphIndex(nil, r)
		if err == nil && idx != 0 {
			return i
		}
	}
	return 0
}

// measureWidth 量一行字宽（逐 rune 换 face；子集间 kern 差异可忽略）。
func measureWidth(faces []*sfnt.Font, size float64, s string) fixed.Int26_6 {
	var w fixed.Int26_6
	for _, r := range norm.NFC.String(s) {
		face, err := coverFace(faces, r, size)
		if err != nil {
			continue
		}
		if adv, ok := face.GlyphAdvance(r); ok {
			w += adv
		}
	}
	return w
}

// coverFace 是某 rune 在某字号下的 face（font.Face 只对应一个字号，新建
// 只是查表——代价小）。
func coverFace(faces []*sfnt.Font, r rune, size float64) (font.Face, error) {
	return opentype.NewFace(faces[faceForRune(faces, r)], &opentype.FaceOptions{
		Size: size, DPI: 72, Hinting: font.HintingFull,
	})
}

// drawLine 在 (x, y baseline) 画一行（逐 rune 换 face）。col 按非预乘 alpha 解释。
func drawLine(dst *image.RGBA, faces []*sfnt.Font, size float64, x0, y0 int, s string, col color.RGBA) {
	dx := x0
	for _, r := range norm.NFC.String(s) {
		face, err := coverFace(faces, r, size)
		if err != nil {
			continue
		}
		d := font.Drawer{Dst: dst, Src: image.NewUniform(color.NRGBA(col)), Face: face, Dot: fixed.P(dx, y0)}
		rs := string(r)
		d.DrawString(rs)
		dx += d.MeasureString(rs).Ceil()
	}
}

// wrapTitle 把 tên truyện 折行（单词边界），最多 maxLines 行；塞不下时末行补 "…"。
func wrapTitle(faces []*sfnt.Font, size float64, title string, maxWidth, maxLines int) []string {
	words := strings.Fields(title)
	var lines []string
	var cur string
	full := false
	for _, w := range words {
		cand := w
		if cur != "" {
			cand = cur + " " + w
		}
		if measureWidth(faces, size, cand).Ceil() <= maxWidth || cur == "" {
			cur = cand
			continue
		}
		lines = append(lines, cur)
		cur = w
		if len(lines) == maxLines {
			full = true
			break
		}
	}
	if !full && cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		full = true
	}
	// 还有词没排进去 → 末行补 "…"（先缩到放得下）。
	if full || strings.Join(lines, " ") != strings.Join(words, " ") {
		last := lines[len(lines)-1] + "…"
		runes := []rune(last)
		for len(runes) > 2 && measureWidth(faces, size, string(runes)).Ceil() > maxWidth {
			runes = append(runes[:len(runes)-2], '…')
		}
		lines[len(lines)-1] = string(runes)
	}
	return lines
}

// generatedCover 画 bìa tự tạo（A7: tên truyện + tập + Noverse Audio, nền thương
// hiệu, không chữ viết tắt）。确定性：同输入 → 同像素。
func generatedCover(title string, volume, size int) (*image.RGBA, error) {
	faces, err := loadCoverFaces()
	if err != nil {
		return nil, err
	}
	master := image.NewRGBA(image.Rect(0, 0, coverMasterSize, coverMasterSize))
	// 底：对角渐变 #1C1426 → #5A2E5E，右上 #F6CB8C 高光，左下 #C9457F 晕染
	// （贴近 artboard .cov.up 的三层 radial-gradient）。纯数学，无需噪声图。
	for y := range coverMasterSize {
		for x := range coverMasterSize {
			fx, fy := float64(x)/coverMasterSize, float64(y)/coverMasterSize
			t := math.Min(1, math.Max(0, (fx*0.94+fy*0.34)*0.9))
			r := lerp(0x1C, 0x5A, t)
			g := lerp(0x14, 0x2E, t)
			b := lerp(0x26, 0x5E, t)
			w1 := math.Max(0, 1-math.Hypot(fx-0.8, fy-0.1)/0.55) * 0.55
			r = lerp(r, 0xF6, w1)
			g = lerp(g, 0xCB, w1)
			b = lerp(b, 0x8C, w1*0.85)
			w2 := math.Max(0, 1-math.Hypot(fx-0.08, fy-0.98)/0.7) * 0.5
			r = lerp(r, 0xC9, w2)
			g = lerp(g, 0x45, w2)
			b = lerp(b, 0x7F, w2)
			master.SetRGBA(x, y, color.RGBA{uint8(r), uint8(g), uint8(b), 0xFF})
		}
	}
	pad := coverMasterSize / 9 * 62 / 100 // ≈ 0.62em (字号 = s/9)
	titleSize := float64(coverMasterSize) / 9 * 0.55
	footer := float64(coverMasterSize) / 9 * 0.34
	lines := wrapTitle(faces, titleSize, title, coverMasterSize-2*pad, 3)
	y := pad + int(titleSize)
	for _, line := range lines {
		drawLine(master, faces, titleSize, pad, y, line, color.RGBA{0xFF, 0xFF, 0xFF, 0xF2})
		y += int(titleSize * 1.18)
	}
	// 底栏：左边 "Noverse Audio"，右边 "Tập N · sách nói"。
	bottomY := coverMasterSize - pad
	drawLine(master, faces, footer, pad, bottomY, ArtistAudio, color.RGBA{0xFF, 0xFF, 0xFF, 0xE6})
	sub := fmt.Sprintf("Tập %d · sách nói", volume)
	subW := measureWidth(faces, footer*0.92, sub).Ceil()
	drawLine(master, faces, footer*0.92, coverMasterSize-pad-subW, bottomY, sub, color.RGBA{0xFF, 0xFF, 0xFF, 0xBF})
	if size == coverMasterSize {
		return master, nil
	}
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	imagedraw.CatmullRom.Scale(dst, dst.Bounds(), master, master.Bounds(), imagedraw.Over, nil)
	return dst, nil
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

// uploadedCover 找 <assets>/bia 里最新的图片（jpg/jpeg/png/webp）。
func uploadedCover(dir string) (string, os.FileInfo) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil
	}
	var best os.FileInfo
	bestPath := ""
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".jpg", ".jpeg", ".png", ".webp":
		default:
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if best == nil || info.ModTime().After(best.ModTime()) {
			best, bestPath = info, filepath.Join(dir, e.Name())
		}
	}
	return bestPath, best
}

// coverCachePath 是某请求的缓存文件（key 已含 size/版本/来源指纹）。
func (l Layout) coverCachePath(key string) string {
	return filepath.Join(l.AudioDir, "bia", key+".png")
}

// CoverPNG 返回 cover PNG 的缓存路径：命中直接给；否则渲染/缩放并写盘。
// 上传图先中心裁方再缩到 size；生成图按 A7 版式画（title = {Tên ngắn}）。
// 输入变化（换上传图/改 tên ngắn/卷号/尺寸）→ key 变 → 自动重渲。
func (l Layout) CoverPNG(short string, volume, size int) (string, error) {
	up, info := uploadedCover(l.CoverDir())
	h := sha256.New()
	h.Write([]byte(coverCacheVersion))
	fmt.Fprintf(h, "%d|%d|", size, volume)
	if info != nil {
		fmt.Fprintf(h, "up:%s:%d:%d", up, info.Size(), info.ModTime().UnixNano())
	} else {
		fmt.Fprintf(h, "gen:%s", short)
	}
	cacheKey := fmt.Sprintf("%x", h.Sum(nil))[:16]
	dst := l.coverCachePath(cacheKey)
	if _, err := os.Stat(dst); err == nil {
		return dst, nil
	}
	gen := func() (string, error) {
		img, gerr := generatedCover(short, volume, size)
		if gerr != nil {
			return "", gerr
		}
		return cacheWrite(l, cacheKey, img)
	}
	if up == "" {
		return gen()
	}
	img, err := squareScaled(up, size)
	if err != nil {
		// 上传图读不出 → 退化为生成封面（同一缓存槽位，不重复失败）。
		return gen()
	}
	return cacheWrite(l, cacheKey, img)
}

func cacheWrite(l Layout, key string, img *image.RGBA) (string, error) {
	dst := l.coverCachePath(key)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	if err := writeFileAtomic(dst, buf.Bytes()); err != nil {
		return "", err
	}
	return dst, nil
}

// squareScaled 解码图片 → 中心裁方 → 缩到 size×size。
func squareScaled(path string, size int) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, errBadCover
	}
	b := src.Bounds()
	w, hgt := b.Dx(), b.Dy()
	side := min(w, hgt)
	sx := b.Min.X + (w-side)/2
	sy := b.Min.Y + (hgt-side)/2
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	imagedraw.CatmullRom.Scale(dst, dst.Bounds(), src, image.Rect(sx, sy, sx+side, sy+side), imagedraw.Over, nil)
	return dst, nil
}
