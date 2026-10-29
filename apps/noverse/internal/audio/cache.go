package audio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

// WAV 读取 / 峰值计算 / 静音生成。全部纯 Go，不依赖 ffmpeg——界面画波形、
// 估算时长、拼接静音都靠它。

// WAVInfo 描述一个 PCM WAV 的基本参数。
type WAVInfo struct {
	SampleRate    int
	Channels      int
	BitsPerSample int
	DataOffset    int
	DataLen       int
}

// DurationSec 返回音频时长。
func (w WAVInfo) DurationSec() float64 {
	if w.SampleRate <= 0 || w.Channels <= 0 || w.BitsPerSample <= 0 {
		return 0
	}
	bytesPerFrame := w.Channels * w.BitsPerSample / 8
	if bytesPerFrame == 0 {
		return 0
	}
	frames := w.DataLen / bytesPerFrame
	return float64(frames) / float64(w.SampleRate)
}

// ParseWAV 解析 RIFF/WAVE 头，定位 data 块。
func ParseWAV(data []byte) (WAVInfo, error) {
	var info WAVInfo
	if len(data) < 44 {
		return info, errors.New("tệp wav quá ngắn")
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return info, errors.New("không phải tệp wav")
	}
	pos := 12
	for pos+8 <= len(data) {
		id := string(data[pos : pos+4])
		size := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		body := pos + 8
		if body+size > len(data) {
			// 有些流式写入的 wav 声明的 size 偏大，按实际长度截断。
			size = len(data) - body
		}
		switch id {
		case "fmt ":
			if size < 16 {
				return info, errors.New("tệp wav hỏng phần định dạng")
			}
			info.Channels = int(binary.LittleEndian.Uint16(data[body+2 : body+4]))
			info.SampleRate = int(binary.LittleEndian.Uint32(data[body+4 : body+8]))
			info.BitsPerSample = int(binary.LittleEndian.Uint16(data[body+14 : body+16]))
		case "data":
			info.DataOffset = body
			info.DataLen = size
			return info, nil
		}
		pos = body + size
		if size%2 == 1 {
			pos++
		}
	}
	return info, errors.New("không tìm thấy data trong wav")
}

// WAVPeaks 按每秒 perSecond 个点算峰值（0..1）。界面 Waveform 直接用。
// 只支持 16-bit PCM；其它位深返回错误（引擎输出固定 s16，导出中间文件也是 s16）。
func WAVPeaks(data []byte, perSecond float64) ([]float64, error) {
	info, err := ParseWAV(data)
	if err != nil {
		return nil, err
	}
	if info.BitsPerSample != 16 {
		return nil, fmt.Errorf("chỉ hỗ trợ wav 16-bit, nhận %d-bit", info.BitsPerSample)
	}
	if perSecond <= 0 {
		perSecond = 2
	}
	frameBytes := info.Channels * 2
	if frameBytes == 0 || info.SampleRate == 0 {
		return nil, errors.New("tham số wav không hợp lệ")
	}
	frames := info.DataLen / frameBytes
	window := int(math.Round(float64(info.SampleRate) / perSecond))
	if window < 1 {
		window = 1
	}
	count := (frames + window - 1) / window
	if count <= 0 {
		return []float64{}, nil
	}
	peaks := make([]float64, 0, count)
	base := info.DataOffset
	var maxAbs int32
	var inWindow int
	flush := func() {
		if inWindow == 0 {
			return
		}
		peaks = append(peaks, math.Min(1, float64(maxAbs)/32768))
		maxAbs = 0
		inWindow = 0
	}
	for f := range frames {
		off := base + f*frameBytes
		for c := range info.Channels {
			v := int32(int16(binary.LittleEndian.Uint16(data[off+c*2 : off+c*2+2])))
			if v < 0 {
				v = -v
			}
			if v > maxAbs {
				maxAbs = v
			}
		}
		inWindow++
		if inWindow >= window {
			flush()
		}
	}
	flush()
	return peaks, nil
}

// WAVDurationSec 从字节读时长；解析失败返回 0（调用方自己决定报错）。
func WAVDurationSec(data []byte) float64 {
	info, err := ParseWAV(data)
	if err != nil {
		return 0
	}
	return info.DurationSec()
}

// silenceWAV 生成 ms 毫秒的静音 WAV（s16、单声道）。
func silenceWAV(ms, sampleRate int) []byte {
	if ms <= 0 {
		return nil
	}
	if sampleRate <= 0 {
		sampleRate = 48000
	}
	frames := sampleRate * ms / 1000
	dataLen := frames * 2
	var buf bytes.Buffer
	buf.Grow(44 + dataLen)
	writeWAVHeader(&buf, dataLen, sampleRate, 1, 16)
	buf.Write(make([]byte, dataLen))
	return buf.Bytes()
}

// writeWAVHeader 写 44 字节标准头（PCM）。
func writeWAVHeader(buf *bytes.Buffer, dataLen, sampleRate, channels, bits int) {
	byteRate := sampleRate * channels * bits / 8
	blockAlign := channels * bits / 8
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+dataLen))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(1))
	binary.Write(buf, binary.LittleEndian, uint16(channels))
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(buf, binary.LittleEndian, uint32(byteRate))
	binary.Write(buf, binary.LittleEndian, uint16(blockAlign))
	binary.Write(buf, binary.LittleEndian, uint16(bits))
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(dataLen))
}

// ReadSegmentAudio 读缓存音频；缺失返回 (nil, nil)。
func ReadSegmentAudio(l Layout, volume, chapter int, key string) ([]byte, error) {
	data, err := os.ReadFile(l.CachePath(volume, chapter, key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

// WriteSegmentAudio 把一段音频写进缓存（已存在则视为命中，不再覆盖）。
func WriteSegmentAudio(l Layout, volume, chapter int, key string, wav []byte) error {
	path := l.CachePath(volume, chapter, key)
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return writeFileAtomic(path, wav)
}

// cachedDuration 返回缓存段时长（0 表示没有）。
func cachedDuration(l Layout, volume, chapter int, key string) float64 {
	data, err := ReadSegmentAudio(l, volume, chapter, key)
	if err != nil || data == nil {
		return 0
	}
	return WAVDurationSec(data)
}

// removeSegmentAudio 删掉一段缓存（换参数重生成时用）。
func removeSegmentAudio(l Layout, volume, chapter int, key string) error {
	err := os.Remove(l.CachePath(volume, chapter, key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// audioExts 是导出目录里算"音频"的扩展名（与 storyread 的 ExportFile.format 对齐）。
var audioExts = map[string]bool{".mp3": true, ".wav": true, ".m4a": true, ".m4b": true}

// isAudioFile 判断路径是否音频产物。
func isAudioFile(name string) bool {
	return audioExts[filepath.Ext(name)]
}
