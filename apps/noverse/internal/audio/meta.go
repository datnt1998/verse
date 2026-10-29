package audio

// Gói AF (10-audio A7 + 06-data-contract §AF): một bộ thông tin dùng cho cả
// trình phát web và tệp xuất — tên, người đọc "Noverse Audio", album
// "{Tên truyện} · Tập N", thể loại "Sách nói", bìa, mốc chương.
//
// 这里实现后端半边：tagSet (字段值) → 三种容器的写法。
//   - MP3: ID3v2.4 (Go 手写帧：TIT2 TPE1 TALB TPE2 TRCK TPOS TCON TDRC APIC COMM
//     TLAN + 整卷的 CHAP/CTOC)。ffmpeg 只编码不写字段（-id3v2_version 0）。
//   - WAV: LIST/INFO 由 ffmpeg 按元数据写 + 同一个 ID3v2.4 序列化塞进 `id3 `块。
//   - M4B: ffmpeg remux（©nam ©ART ©alb aART trkn disk ©gen ©day covr + stik=2
//     + 章节轨）。

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// ArtistAudio 是 §AF 锁死的"người đọc"：永远是 Noverse Audio，kể cả chương
// chưa chọn giọng (lock screen 旧 bug: "Chưa chọn giọng" 被当成 artist)。
const ArtistAudio = "Noverse Audio"

const (
	genreAudiobook = "Sách nói"
	commentAudio   = "Tạo bằng Noverse"
	languageAudio  = "vie"
)

// shortTitleMax 是 story.json `shortTitle` 的字符上限（06 §AF）。
const shortTitleMax = 32

// tagSet 是一个导出文件要写的全部标签；Cover 是方形 PNG 字节（可为空）。
type tagSet struct {
	Title   string // TIT2 / ©nam
	Artist  string // TPE1 / ©ART (恒 "Noverse Audio")
	Album   string // TALB / ©alb = "{Tên truyện} · Tập N"
	Track   int    // TRCK/trkn 分子（整卷文件 = 1）
	Total   int    // TRCK 分母（卷内总章数）
	Disc    int    // TPOS/disk = số tập
	Genre   string // TCON / ©gen
	Year    int    // TDRC / ©day
	Comment string // COMM
	Lang    string // TLAN (MP3/WAV)
	Cover   []byte // APIC / covr (PNG)
}

// chapterTitleTag 是单章/bản thử 的 TIT2： "Chương N · {tên}" (+ " (bản thử)")。
func chapterTitleTag(sc SourceChapter, trial bool) string {
	title := strings.TrimSpace(sc.Title)
	t := fmt.Sprintf("Chương %d", sc.Chapter)
	if title != "" {
		t = fmt.Sprintf("Chương %d · %s", sc.Chapter, title)
	}
	if trial {
		t += " (bản thử)"
	}
	return t
}

// CleanStoryTitle removes import-document labels and wrapping quotes, without
// guessing the author's casing. It is shared by export, cover and album names.
func CleanStoryTitle(title string) string {
	title = strings.Join(strings.Fields(title), " ")
	for {
		title = importTitlePrefix.ReplaceAllString(title, "")
		unwrapped := false
		for _, pair := range titleQuotes {
			if strings.HasPrefix(title, pair[0]) && strings.HasSuffix(title, pair[1]) && len(title) >= len(pair[0])+len(pair[1]) {
				title = strings.TrimSpace(title[len(pair[0]) : len(title)-len(pair[1])])
				unwrapped = true
				break
			}
		}
		if !unwrapped {
			return title
		}
	}
}

var importTitlePrefix = regexp.MustCompile(`(?i)^rules\s+kịch\s+bản\s*[:\-–—]\s*`)
var titleQuotes = [][2]string{{"“", "”"}, {"‘", "’"}, {"«", "»"}, {`"`, `"`}, {"'", "'"}}

// ShortName uses the saved shortTitle first, then the canonical story title.
// Sanitize before measuring; never retain half a word or a quoted phrase.
func ShortName(shortTitle, title string) string {
	name := cleanFileNamePart(CleanStoryTitle(shortTitle))
	if name == "" {
		name = cleanFileNamePart(CleanStoryTitle(title))
	}
	name = truncateAtWord(name, shortTitleMax)
	if name == "" {
		return "Truyện"
	}
	return name
}

func truncateAtWord(s string, max int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	end, count := 0, 0
	for _, word := range strings.Fields(s) {
		length := utf8.RuneCountInString(word)
		if count > 0 {
			length++
		}
		if count+length > max {
			break
		}
		count += length
		end += len(word)
		if end < len(s) {
			end++
		}
	}
	cut := strings.TrimSpace(s[:end])
	for _, pair := range titleQuotes {
		if pair[0] == pair[1] {
			if strings.Count(cut, pair[0])%2 != 0 {
				cut = strings.TrimSpace(cut[:strings.LastIndex(cut, pair[0])])
			}
		} else if strings.Count(cut, pair[0]) > strings.Count(cut, pair[1]) {
			cut = strings.TrimSpace(cut[:strings.LastIndex(cut, pair[0])])
		}
	}
	return cut
}

func cleanFileNamePart(name string) string {
	return strings.Trim(strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if r < ' ' || strings.ContainsRune(`/\:*?"<>|`, r) {
			return ' '
		}
		return r
	}, name)), " "), ". ")
}

// exportName 是全量/分章/zip 的 {Tên ngắn} 命名（A7）：
//
//	chương:  "{短} · Tập V · Chương NN · {tên chương}.{ext}"
//	tập/subset: "{短} · Tập V{ · Chương NN}….{ext}"（见 volumeFileName 后缀）
//	trial:   同 chương + " (bản thử)" 在扩展名前
//	zip:     "{短} · Tập V · N chương.zip"
func exportChapterFileName(short string, volume int, sc SourceChapter, trial bool, format string) string {
	base := fmt.Sprintf("%s · Tập %d · Chương %02d", short, volume, sc.Chapter)
	if title := cleanFileNamePart(sc.Title); title != "" {
		base += " · " + title
	}
	if trial {
		base += " (bản thử)"
	}
	return sanitizeFileName(base + "." + format)
}

// exportVolumeFileName 是整卷/合并导出（split=false）的成品名：
//   - 整卷："{Tên ngắn} · Tập V.{ext}"
//   - 选章合并："{Tên ngắn} · Tập V{-ch1-9|-ch1-3_5}.{ext}"（10-audio:259 +
//     FE audioVolumeFileName 的 suffix 参数；即使只选一章也带 -chN）。
//
// 章名式 "{… · Chương NN · {tên}}" 只用于 split 的单章文件。
func exportVolumeFileName(short string, volume int, chapters []int, format string) string {
	base := fmt.Sprintf("%s · Tập %d%s", short, volume, exportChapterSuffix(chapters))
	return sanitizeFileName(base + "." + format)
}

// exportZipName 是 split+pack=zip 的名字："{短} · Tập V · N chương.zip"。
func exportZipName(short string, volume, count int) string {
	return sanitizeFileName(fmt.Sprintf("%s · Tập %d · %d chương.zip", short, volume, count))
}

// asciiFileName 生成 RFC 6266 filename="…" 的 ASCII 退路：NFD 去声调、đ→d、
// 丢非 ASCII、· → -、压空白、去双引号与反斜杠。
func asciiFileName(name string) string {
	replacer := newASCIIRunes()
	var b bytes.Buffer
	lastSpace := false
	for _, r := range norm.NFD.String(name) {
		if r < 0x80 && (r >= '!' || r == ' ') && r != '"' && r != '\\' {
			if r == ' ' {
				if lastSpace || b.Len() == 0 {
					continue
				}
				lastSpace = true
			} else {
				lastSpace = false
			}
			b.WriteByte(byte(r))
			continue
		}
		if ascii, ok := replacer[r]; ok {
			if strings.HasPrefix(ascii, " ") && lastSpace {
				ascii = ascii[1:]
			}
			b.WriteString(ascii)
			lastSpace = strings.HasSuffix(ascii, " ")
		}
	}
	out := strings.TrimSpace(b.String())
	for strings.Contains(out, "- -") {
		out = strings.ReplaceAll(out, "- -", "-")
	}
	if out == "" {
		out = "noverse-audio"
	}
	return out
}

// asciiRunes 是 NFD 后仍 >0x7F 的字符的 ASCII 映射（đ/Đ + 常用标点）。
func newASCIIRunes() map[rune]string {
	return map[rune]string{
		'đ': "d", 'Đ': "D",
		'·': " - ", '—': " - ", '–': "-",
		'“': "", '”': "", '‘': "", '’': "",
		'ı': "i", 'ł': "l", 'Ł': "L", 'ø': "o", 'Ø': "O",
	}
}

// ─── ID3v2.4 ───

// id3v24 把一个 tagSet + 可选章节序列化成 ID3v2.4 标签字节（MP3 头、WAV id3 块）。
// ID3v2.4：头 "ID3" 04 00 flags size(synchsafe)；帧头 10B id+size(synchsafe)+flags。
// 文本帧 encoding byte 0x03 = UTF-8。
func id3v24(t tagSet, marks []ChapterMark, durationSec float64) []byte {
	var frames bytes.Buffer
	text := func(id, val string) {
		if val == "" {
			return
		}
		writeID3Frame(&frames, id, append([]byte{0x03}, []byte(val)...))
	}
	text("TIT2", t.Title)
	text("TPE1", t.Artist)
	text("TALB", t.Album)
	text("TPE2", ArtistAudio)
	if t.Track > 0 {
		trk := fmt.Sprintf("%d", t.Track)
		if t.Total > 0 {
			trk = fmt.Sprintf("%d/%d", t.Track, t.Total)
		}
		text("TRCK", trk)
	}
	if t.Disc > 0 {
		text("TPOS", fmt.Sprintf("%d", t.Disc))
	}
	text("TCON", t.Genre)
	if t.Year > 0 {
		text("TDRC", fmt.Sprintf("%d", t.Year))
	}
	text("TLAN", t.Lang)
	if len(t.Cover) > 0 {
		apic := bytes.NewBuffer(nil)
		apic.WriteByte(0x03)
		apic.WriteString("image/png")
		apic.WriteByte(0)
		apic.WriteByte(0x03) // front cover
		apic.WriteByte(0)    // empty description
		apic.Write(t.Cover)
		writeID3Frame(&frames, "APIC", apic.Bytes())
	}
	if t.Comment != "" {
		comm := bytes.NewBuffer(nil)
		comm.WriteByte(0x03)
		comm.WriteString("vie")
		comm.WriteByte(0)
		comm.WriteString(t.Comment)
		writeID3Frame(&frames, "COMM", comm.Bytes())
	}
	if len(marks) > 0 {
		ids := make([]string, 0, len(marks))
		for i, m := range marks {
			end := durationSec
			if i+1 < len(marks) {
				end = marks[i+1].AtSec
			}
			chapID := fmt.Sprintf("ch%02d", i)
			ids = append(ids, chapID)
			var body bytes.Buffer
			body.WriteString(chapID)
			body.WriteByte(0)
			binary.Write(&body, binary.BigEndian, uint32(m.AtSec*1000))
			binary.Write(&body, binary.BigEndian, uint32(end*1000))
			// start/end offset ignored
			body.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
			var sub bytes.Buffer
			writeID3Frame(&sub, "TIT2", append([]byte{0x03}, []byte(m.Label)...))
			body.Write(sub.Bytes())
			writeID3Frame(&frames, "CHAP", body.Bytes())
		}
		var ctoc bytes.Buffer
		ctoc.WriteString("toc")
		ctoc.WriteByte(0)
		ctoc.WriteByte(0x03) // top-level and ordered
		ctoc.WriteByte(byte(len(ids)))
		for _, id := range ids {
			ctoc.WriteString(id)
			ctoc.WriteByte(0)
		}
		writeID3Frame(&frames, "CTOC", ctoc.Bytes())
	}
	var out bytes.Buffer
	out.WriteString("ID3")
	out.WriteByte(0x04)
	out.WriteByte(0x00)
	out.WriteByte(0x00)
	out.Write(synchsafe(frames.Len()))
	out.Write(frames.Bytes())
	return out.Bytes()
}

func writeID3Frame(buf *bytes.Buffer, id string, payload []byte) {
	buf.WriteString(id)
	buf.Write(synchsafe(len(payload)))
	buf.Write([]byte{0, 0})
	buf.Write(payload)
}

func synchsafe(n int) []byte {
	return []byte{byte(n >> 21 & 0x7F), byte(n >> 14 & 0x7F), byte(n >> 7 & 0x7F), byte(n & 0x7F)}
}

// ─── ffmpeg 元数据参数（M4B/WAV INFO）──

// tagArgs 生成 -metadata k=v 列表（mp4 muxer 自动映射 ©nam/©ART/trkn/…；
// wav muxer 落到 LIST/INFO）。coverPath 非空时作为第二个输入做 covr (M4B)。
func tagArgs(t tagSet) []string {
	args := []string{
		"-metadata", "title=" + t.Title,
		"-metadata", "artist=" + t.Artist,
		"-metadata", "album=" + t.Album,
		"-metadata", "album_artist=" + ArtistAudio,
		"-metadata", "genre=" + t.Genre,
		"-metadata", "comment=" + t.Comment,
		"-metadata", "language=" + t.Lang,
	}
	if t.Track > 0 {
		trk := fmt.Sprintf("%d", t.Track)
		if t.Total > 0 {
			trk = fmt.Sprintf("%d/%d", t.Track, t.Total)
		}
		args = append(args, "-metadata", "track="+trk)
	}
	if t.Disc > 0 {
		args = append(args, "-metadata", fmt.Sprintf("disc=%d", t.Disc))
	}
	if t.Year > 0 {
		args = append(args, "-metadata", fmt.Sprintf("date=%d", t.Year))
	}
	return args
}

// m4bExtraArgs: stik=2 (sách nói) — ffmpeg mp4 muxer 把 media_type → stik。
const m4bMediaType = "2"

// ─── 标签指纹（写进导出缓存键，保证换封面/元数据规则会重新编码）──

// tagFingerprint 是"这一批导出文件的标签输入"哈希：artist/album/封面字节
// (内容哈希即可代表文件变化)/规则版本。encKey 叠加它，封面一改缓存就失效。
func tagFingerprint(coverPNG []byte) string {
	h := sha256.New()
	h.Write([]byte("af-tag-v2"))
	h.Write([]byte(ArtistAudio))
	if len(coverPNG) > 0 {
		sum := sha256.Sum256(coverPNG)
		h.Write(sum[:])
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// ─── WAV `id3 ` 块 ───

// appendWavID3 把序列化好的 ID3v2.4 tag 包成 RIFF `id3 ` 块附加到 WAV。
// RIFF 块 2-byte 对齐；RIFF size 字段同步更新。文件原地重写。
func appendWavID3(path string, id3 []byte) error {
	data, err := readFileBytes(path)
	if err != nil {
		return err
	}
	if len(data) < 12 || !bytes.Equal(data[:4], []byte("RIFF")) || !bytes.Equal(data[8:12], []byte("WAVE")) {
		return fmt.Errorf("%w: %s", errBadWOFF, "không phải tệp WAV")
	}
	// 幂等：已存在 `id3 ` 块就整块重写（大小可能变）——先剥掉尾部旧块。
	for i := 12; i+8 <= len(data); {
		size := int(binary.LittleEndian.Uint32(data[i+4:]))
		end := i + 8 + size
		padded := end + size%2
		if padded > len(data) {
			break
		}
		if string(data[i:i+4]) == "id3 " {
			data = append(data[:i], data[padded:]...)
			continue
		}
		i = padded
	}
	var chunk bytes.Buffer
	chunk.WriteString("id3 ")
	binary.Write(&chunk, binary.LittleEndian, uint32(len(id3)))
	chunk.Write(id3)
	if len(id3)%2 == 1 {
		chunk.WriteByte(0)
	}
	data = append(data, chunk.Bytes()...)
	binary.LittleEndian.PutUint32(data[4:], uint32(len(data)-8))
	return writeFileBytes(path, data)
}

// readFileBytes/writeFileBytes: điểm duy nhất đọc/ghi nguyên tệp (WAV ≤ vài trăm MB
// nhưng id3 chỉ thêm một lần ở cuối export — chấp nhận được, tránh đục RIFF thủ công).
func readFileBytes(path string) ([]byte, error) { return os.ReadFile(path) }

func writeFileBytes(path string, data []byte) error { return writeFileAtomic(path, data) }
