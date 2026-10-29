package openaispeech

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// errNotWAV báo dữ liệu trả về không phải WAV.
var errNotWAV = errors.New("dữ liệu trả về không phải WAV")

// WAVInfo là thông tin đọc được từ header WAV.
type WAVInfo struct {
	SampleRate int
	Channels   int
	Bits       int
	DataOffset int
	DataLen    int
}

// DurationSec là thời lượng audio suy từ header.
func (w WAVInfo) DurationSec() float64 {
	bytesPerSec := w.SampleRate * w.Channels * w.Bits / 8
	if bytesPerSec <= 0 {
		return 0
	}
	return float64(w.DataLen) / float64(bytesPerSec)
}

// InspectWAV đọc header WAV 16-bit PCM (kể cả header "độ dài vô hạn" của máy chủ
// phát luồng, khi đó độ dài data lấy theo phần thân thực tế).
func InspectWAV(b []byte) (WAVInfo, error) {
	var info WAVInfo
	if len(b) < 12 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return info, errNotWAV
	}
	off := 12
	for off+8 <= len(b) {
		id := string(b[off : off+4])
		size := int(binary.LittleEndian.Uint32(b[off+4 : off+8]))
		body := off + 8
		switch id {
		case "fmt ":
			if body+16 > len(b) {
				return info, fmt.Errorf("%w: fmt thiếu dữ liệu", errNotWAV)
			}
			info.Channels = int(binary.LittleEndian.Uint16(b[body+2 : body+4]))
			info.SampleRate = int(binary.LittleEndian.Uint32(b[body+4 : body+8]))
			info.Bits = int(binary.LittleEndian.Uint16(b[body+14 : body+16]))
		case "data":
			info.DataOffset = body
			// 0xFFFFFFFF (hoặc vượt phần thân) = header phát luồng: độ dài thật là phần còn lại.
			if size == 0xFFFFFFFF || size > len(b)-body {
				info.DataLen = len(b) - body
			} else {
				info.DataLen = size
			}
			if info.SampleRate == 0 {
				return info, fmt.Errorf("%w: thiếu fmt trước data", errNotWAV)
			}
			return info, nil
		default:
			if size == 0xFFFFFFFF || size > len(b)-body {
				return info, fmt.Errorf("%w: chunk %q hỏng", errNotWAV, id)
			}
			off = body + size
			if size%2 == 1 {
				off++
			}
			continue
		}
		off = body + size
		if size%2 == 1 {
			off++
		}
	}
	return info, fmt.Errorf("%w: không thấy chunk data", errNotWAV)
}

// NormalizeWAV sửa kích thước RIFF/data cho khớp phần thân thật. Máy chủ VieNeu ghi
// 0xFFFFFFFF để phát ngay, ffprobe/trình phát coi đó là tệp hỏng — nên mọi WAV ra
// khỏi tầng này đều được vá lại.
func NormalizeWAV(b []byte) ([]byte, WAVInfo, error) {
	info, err := InspectWAV(b)
	if err != nil {
		return nil, info, err
	}
	out := b
	if info.DataOffset+info.DataLen != len(b) {
		out = b[:info.DataOffset+info.DataLen]
	}
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	binary.LittleEndian.PutUint32(out[info.DataOffset-4:info.DataOffset], uint32(info.DataLen))
	return out, info, nil
}

// WAVDuration đọc thời lượng một tệp/clip WAV.
func WAVDuration(b []byte) (float64, error) {
	info, err := InspectWAV(b)
	if err != nil {
		return 0, err
	}
	return info.DurationSec(), nil
}
