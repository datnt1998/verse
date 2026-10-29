package audio

// WOFF (Web Open Font Format, không phải WOFF2) → sfnt bytes cho
// golang.org/x/image/font/sfnt.Parse. Fontsource phát hành Be Vietnam Pro dưới
// dạng .woff; giải nén bảng zlib tại chỗ để dựng bìa tự tạo (Gói AF, A7).
//
// Cấu trúc: header 44 byte (signature 'wOFF', flavor, length, numTables,
// reserved, totalSfntSize, version, meta*, priv*) + numTables entry 20 byte
// (tag, offset, compLength, origLength, checksum) + dữ liệu bảng 4-byte align.
// Dữ liệu bảng là zlib nếu compLength < origLength, nguyên văn nếu bằng.

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

var errBadWOFF = errors.New("font không đọc được")

func decodeWOFF(data []byte) ([]byte, error) {
	if len(data) < 44 || binary.BigEndian.Uint32(data) != 0x774F4646 {
		return nil, errBadWOFF
	}
	flavor := binary.BigEndian.Uint32(data[4:])
	numTables := int(binary.BigEndian.Uint16(data[12:]))
	sfntSize := int(binary.BigEndian.Uint32(data[16:]))
	dir := data[44:]
	if len(dir) < numTables*20 {
		return nil, errBadWOFF
	}
	type tab struct {
		tag        uint32
		off, comp  uint32
		orig, chks uint32
	}
	tables := make([]tab, numTables)
	var sfntLen uint32
	for i := range numTables {
		e := dir[i*20:]
		t := tab{
			tag:  binary.BigEndian.Uint32(e),
			off:  binary.BigEndian.Uint32(e[4:]),
			comp: binary.BigEndian.Uint32(e[8:]),
			orig: binary.BigEndian.Uint32(e[12:]),
			chks: binary.BigEndian.Uint32(e[16:]),
		}
		if t.comp > t.orig || int64(t.off)+int64(t.comp) > int64(len(data)) {
			return nil, errBadWOFF
		}
		tables[i] = t
		// Mỗi bảng căn 4 byte trong sfnt.
		sfntLen += 16 + (t.orig+3)&^3
	}
	if sfntSize != 0 && int64(sfntSize) < int64(sfntLen) {
		sfntLen = uint32(sfntSize)
	}
	out := bytes.NewBuffer(make([]byte, 0, sfntLen))
	if err := binary.Write(out, binary.BigEndian, flavor); err != nil {
		return nil, err
	}
	// searchRange/entrySelector/rangeShift chuẩn sfnt.
	maxPow := uint16(1)
	entrySelector := uint16(0)
	for maxPow*2 <= uint16(numTables) {
		maxPow *= 2
		entrySelector++
	}
	for _, v := range []uint16{uint16(numTables), maxPow * 16, entrySelector, uint16(numTables)*16 - maxPow*16} {
		if err := binary.Write(out, binary.BigEndian, v); err != nil {
			return nil, err
		}
	}
	for _, t := range tables {
		for _, v := range []uint32{t.tag, t.chks, uint32(out.Len() + 16), t.orig} {
			// offset ghi tạm dưới — vá lại sau khi biết vị trí thật.
			if err := binary.Write(out, binary.BigEndian, v); err != nil {
				return nil, err
			}
		}
	}
	raw := out.Bytes()
	for i, t := range tables {
		binary.BigEndian.PutUint32(raw[12+i*16+8:], uint32(out.Len()))
		var payload []byte
		if t.comp < t.orig {
			zr, err := zlib.NewReader(bytes.NewReader(data[t.off : t.off+t.comp]))
			if err != nil {
				return nil, fmt.Errorf("%w: %v", errBadWOFF, err)
			}
			payload, err = io.ReadAll(zr)
			if cerr := zr.Close(); err == nil {
				err = cerr
			}
			if err != nil || uint32(len(payload)) != t.orig {
				return nil, errBadWOFF
			}
		} else {
			payload = data[t.off : t.off+t.comp]
		}
		out.Write(payload)
		if pad := (4 - out.Len()%4) % 4; pad > 0 {
			out.Write(make([]byte, pad))
		}
	}
	return out.Bytes(), nil
}
