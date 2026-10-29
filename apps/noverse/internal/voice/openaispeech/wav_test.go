package openaispeech

import (
	"encoding/binary"
	"net/http"
	"strings"
	"testing"
)

// pcmWAV dựng WAV 16-bit mono với header kiểu "phát luồng" (0xFFFFFFFF) như máy chủ
// VieNeu trả về, hoặc header bình thường khi exact = true.
func pcmWAV(samples int, sampleRate int, streaming bool) []byte {
	data := make([]byte, samples*2)
	body := make([]byte, 0, 44+len(data))
	body = append(body, "RIFF"...)
	size := uint32(36 + len(data))
	if streaming {
		size = 0xFFFFFFFF
	}
	body = binary.LittleEndian.AppendUint32(body, size)
	body = append(body, "WAVEfmt "...)
	body = binary.LittleEndian.AppendUint32(body, 16)
	body = binary.LittleEndian.AppendUint16(body, 1) // PCM
	body = binary.LittleEndian.AppendUint16(body, 1) // mono
	body = binary.LittleEndian.AppendUint32(body, uint32(sampleRate))
	body = binary.LittleEndian.AppendUint32(body, uint32(sampleRate*2))
	body = binary.LittleEndian.AppendUint16(body, 2)
	body = binary.LittleEndian.AppendUint16(body, 16)
	body = append(body, "data"...)
	dataSize := uint32(len(data))
	if streaming {
		dataSize = 0xFFFFFFFF
	}
	body = binary.LittleEndian.AppendUint32(body, dataSize)
	return append(body, data...)
}

func TestNormalizeWAVFixesStreamingHeader(t *testing.T) {
	raw := pcmWAV(48000, 48000, true)
	wav, info, err := NormalizeWAV(raw)
	if err != nil {
		t.Fatalf("NormalizeWAV: %v", err)
	}
	if got := binary.LittleEndian.Uint32(wav[4:8]); got != uint32(len(wav)-8) {
		t.Fatalf("RIFF size = %d, muốn %d", got, len(wav)-8)
	}
	if got := binary.LittleEndian.Uint32(wav[info.DataOffset-4 : info.DataOffset]); got != uint32(info.DataLen) {
		t.Fatalf("data size = %d, muốn %d", got, info.DataLen)
	}
	if info.SampleRate != 48000 || info.Channels != 1 || info.Bits != 16 {
		t.Fatalf("fmt = %+v", info)
	}
	if d := info.DurationSec(); d < 0.99 || d > 1.01 {
		t.Fatalf("thời lượng = %v, muốn 1 giây", d)
	}
}

func TestInspectWAVRejectsNonAudio(t *testing.T) {
	if _, err := InspectWAV([]byte(`{"error":{"message":"boom"}}`)); err == nil {
		t.Fatal("JSON không được coi là WAV")
	}
	if _, err := InspectWAV([]byte("RIFF0000WAVE")); err == nil {
		t.Fatal("WAV thiếu chunk data phải báo lỗi")
	}
}

func TestAtempoFilterChains(t *testing.T) {
	cases := []struct {
		speed float64
		want  string
	}{
		{1.5, "atempo=1.5000"},
		{0.75, "atempo=0.7500"},
		{4, "atempo=2,atempo=2.0000"},
		{0.25, "atempo=0.5,atempo=0.5000"},
		{0, "atempo=1"},
	}
	for _, c := range cases {
		if got := atempoFilter(c.speed); got != c.want {
			t.Fatalf("atempoFilter(%v) = %q, muốn %q", c.speed, got, c.want)
		}
	}
}

func TestHTTPErrorUsesServerMessage(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusBadRequest, Header: http.Header{}}
	err := httpError(resp, []byte(`{"error":{"message":"unknown voice 'X'"}}`))
	if err == nil || !strings.Contains(err.Error(), "unknown voice") {
		t.Fatalf("lỗi = %v, muốn giữ thông điệp máy chủ", err)
	}
}

func TestNormalizeGender(t *testing.T) {
	cases := map[string]string{"Male": "male", "female": "female", "Nữ": "female", "nam": "male", "": "", "khác": ""}
	for in, want := range cases {
		if got := normalizeGender(in); got != want {
			t.Fatalf("normalizeGender(%q) = %q, muốn %q", in, got, want)
		}
	}
}
