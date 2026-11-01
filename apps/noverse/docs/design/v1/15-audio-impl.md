# 15 · Audio — thi công (VieNeu-TTS, module giọng đọc mở rộng được)

Hợp đồng màn hình & API: `10-audio.md` (GAP-17…21) — giữ nguyên tên endpoint, trường, trạng thái. Tài liệu này chốt kiến trúc.

## 1. Tầng giọng đọc dùng chung — `internal/voice`

Không gắn chặt vào Audio truyện: mọi module khác (đọc to trong Trình đọc, thông báo, nghe thử cách đọc…) dùng cùng dịch vụ.

```go
type Capabilities struct{ Cloning, EmotionCues, Streaming bool; SampleRate int; Qualities []string }
type Voice struct{ ID, Name, Gender, Source string /* builtin|project|clone */; File string; DurationSec float64 }
type SynthRequest struct{ Text, VoiceID, RefAudio, Tone, Quality string; Speed float64 }
type Audio struct{ WAV []byte; SampleRate int; DurationSec float64 }
type Status struct{ Ready bool; Engine, Model, Device, State /* missing|installing|stopped|starting|ready|error */, Error string; RTF *float64 }

type Engine interface {
    Name() string
    Capabilities() Capabilities
    Status(ctx) Status
    Voices(ctx) ([]Voice, error)
    Synthesize(ctx, SynthRequest) (Audio, error)
    AddVoice(ctx, name, wavPath string) (Voice, error)   // clone; ErrUnsupported nếu không hỗ trợ
    Close() error
}
func Register(name string, f Factory)        // engine mới chỉ cần đăng ký
type Service                                  // hàng đợi duy nhất mỗi engine: ưu tiên preview > render, Pause/Resume/Cancel theo job, đo RTF
```

- **Giao thức chuẩn = OpenAI-compatible Speech API** — đúng máy chủ chính thức của VieNeu (`apps/openai_speech.py` trong https://github.com/pnnbao97/VieNeu-TTS): `POST /v1/audio/speech {model, input, voice, response_format:"wav", sample_rate, temperature…}` → audio, `GET /v1/voices` (`[{id,name,description,gender,featured,aliases}]`), `POST /v1/voices` (multipart, clone từ clip; 409 nếu trùng giọng có sẵn), `GET /v1/models`, `GET /health`, Bearer `VIENEU_API_KEY`. Engine Go `openai-speech` nói chuyện với **bất kỳ** máy chủ nào theo chuẩn này: VieNeu cục bộ do Noverse quản lý, VieNeu chạy Docker (`--profile api-cpu|api-gpu`) hay máy khác trong mạng, hoặc dịch vụ TTS tương thích OpenAI khác — cắm mô hình mới không cần sửa Go. Không viết sidecar riêng.
- **Engine `vieneu`** (mặc định) = `openai-speech` + trình quản lý tiến trình: cài bằng một pipeline ghim — tải `uv` ghim vào `~/.ainovel/voice/bin/uv` (không cần `brew`/uv hệ thống), `uv python install 3.12` vào `~/.ainovel/voice/python`, tarball mã nguồn đúng tag từ codeload với SHA-256 ghim (lệch hash là lỗi cứng; codeload lỗi mới fallback `git clone --depth 1 --branch <tag>`) bung nguyên tử vào `~/.ainovel/voice/vieneu/src`, rồi `uv sync --frozen --python 3.12` với các biến `UV_*` trỏ hết vào thư mục dữ liệu của app; chạy `uv run python -m apps.openai_speech` với `HOST=127.0.0.1 PORT=<ngẫu nhiên> VIENEU_API_KEY=<token mỗi lần chạy> VIENEU_PRECISION=<theo chất lượng> HF_HOME=~/.ainovel/voice/vieneu/hf`; chờ `/health`, tắt sau 10 phút rảnh, khởi động lại khi chết. Mô hình v3 Turbo 48 kHz; chất lượng `fast` → `VIENEU_PRECISION=int8` (chỉ khi CPU có VNNI, ngược lại giữ fp32 và báo) · `standard` → fp32 · `high` → fp32 + `sample_rate` 48000 + khử ồn mẫu giọng khi clone. Máy chủ bỏ qua `speed` ⇒ tốc độ đọc chỉnh bằng `ffmpeg atempo` ở tầng audio. Cấu hình `tts.endpoint` (+ `tts.api_key`) để dùng máy chủ VieNeu có sẵn thay vì tự quản lý.
- **Cài đặt bộ đọc** (`vieneu.Provision`): một đường duy nhất cho worker TTS, `POST /api/tts/install` và `ainovel-cli voice install`. Các bước `tools → python → source → deps → model`; `percent` 0–100, `-1` khi bước không đo được. Một lượt giữ khoá `~/.ainovel/voice/.install.lock` (flock; lượt thứ hai chờ rồi tự thấy đã xong), cần ≥4 GiB trống (thiếu ⇒ `no_disk`), và ghi `~/.ainovel/voice/vieneu/install-state.json` (`tag`, `uvVersion`, `pythonVersion`, `installedAt`, `bytes`, `state`). Đã cài đúng tag ghim ⇒ no-op (<50ms, không chạm mạng). Tag ghim đổi ⇒ chỉ làm lại mã nguồn + thư viện, giữ `hf/` (mô hình) và `voices/` (giọng đã nạp). `Provision` chạy đúng **một lần thử** và báo trạng thái qua `voice.ProvisionState` (`internal/voice/provision.go`); thử lại/giãn cách là việc của worker.
- Cấu hình `tts: {engine: "vieneu"|"openai-speech", model, device: cpu|gpu, quality, endpoint?, api_key?}` trong `config.json`, kế thừa 3 cấp như mọi khoá khác (`api_key` chỉ ở cấp toàn cục, không gửi ra trình duyệt).
- CLI: `ainovel-cli voice install | status | voices | say "<chữ>" [--voice] [-o file.wav]`.
- API: `GET /api/tts/status` · `GET /api/tts/engines` · `POST /api/tts/install` (job, tiến độ qua SSE `tts.install`) · `GET /api/tts/voices` · `POST /api/tts/preview {voiceId, text, tone?}` → `audio/wav`.

## 2. Audio truyện — `internal/audio`

- **Bản đọc** (`audio/ban-doc/tap-NN/CC.json`, schema `10-audio.md` §3): chuẩn hoá tất định bằng Go (số, ngày giờ, tiền, ký hiệu, từ điển cách đọc dự án `.ainovel/cach-doc.json`) + AI (model của truyện theo cấu hình hiện có) tách lời thoại, gán người nói kèm xác suất, sắc thái, ngừng; cờ `long_sentence`, `unknown_speaker`, `english`, `no_voice`. `sourceHash` → `stale`.
- **Phân vai** `audio/vai.json`, **giọng dự án** `<assets>/giong/*.wav + giong.json`, **nhạc** `<assets>/nhac/`.
- **Bộ nhớ đoạn** `audio/cache/tap-NN/CC/<hash>.wav`, khoá = hash(text+voice+tone+speed+quality+engine+model).
- **Hàng đợi tạo** dùng `voice.Service`; chốt chương ⇒ tự xếp hàng (khi `autoOnLock`); trạng thái job lưu `audio/queue.json` để sống qua khởi động lại; SSE `audio.render`.
- **Ghép & xuất** bằng `ffmpeg`: nối đoạn + khoảng lặng, đọc tên chương, nhạc mở/kết, `loudnorm` −16 LUFS, MP3/WAV; sinh `.srt` từ thời lượng đoạn, `moc-chuong.txt`, `ban-doc.txt`; từng chương trong `chuong/`. Đỉnh sóng tính sẵn (≈ 2 giá trị/giây).
- Không phụ thuộc worker/engine viết truyện ⇒ chạy ở gateway, dùng được cả khi TUI đang giữ truyện.

### Xuất lại, đặt tên và chất lượng tệp

- Tên truyện lấy từ metadata sách (`meta/book.json`), rồi mới dùng tên hiển thị trong `story.json`; không dùng tên thư mục/dự án. `shortTitle` đã lưu được ưu tiên cho tên tệp/bìa. Khi suy ra, bỏ nhãn nhập `RULES KỊCH BẢN -`/`:`/gạch dài và ngoặc kép bọc ngoài, giữ nguyên hoa/thường, giới hạn 32 ký tự tại ranh giới từ; không giữ một nửa cụm trong ngoặc. Ký tự cấm trong tên tệp đổi thành khoảng trắng. Tên không dùng được trở về `Truyện`; đây là cách xử lý bảo thủ cho GAP tên rỗng/một từ vượt 32 ký tự.
- `audio/export-cache/tap-NN/manifest.json` lưu bộ đệm PCM/tệp chương để tiếp tục việc dở. `results.json` lưu kết quả xuất hoàn tất, dấu vân tay đầu vào và dấu tệp đầu ra, sống qua khởi động lại. Cùng chương thực tế và cùng lựa chọn trả tệp ngay; khác tiêu đề, bìa, nhãn chương hoặc tên tệp chỉ ghi lại tag/sao chép container, không nối hay mã hoá lại giọng. MP3 thay ID3 trực tiếp; MP4/WAV dùng `-c:a copy`. Đổi nội dung, khoảng nghỉ, nhạc, mức chất lượng hoặc lựa chọn mã hoá làm mất hiệu lực đúng lớp bộ đệm. Thời gian sửa của đoạn nguồn là một phần dấu vân tay; thời gian truy cập/khởi động máy chủ không phải đầu vào.
- Tệp mã hoá, tag/bìa và metadata được ghi ra tệp tạm cùng thư mục rồi đổi tên nguyên tử; lỗi ghi tag không làm mất bản xuất đã công bố. Trước khi sao chép kiểm tra `os.SameFile`: đổi chỉ hoa/thường đi qua tên tạm rồi đổi sang tên đích (cả trên APFS không phân biệt hoa/thường); cùng inode nhưng không phải đổi hoa/thường thì không sao chép. Không mở đích bằng `O_TRUNC`.
- Toàn tập được mã hoá một lần từ PCM, không nối từ MP3 của các chương. Chuỗi PCM giữ mono/48 kHz/16-bit của VieNeu; không tạo stereo giả. `standard`/`fast` giữ hồ sơ xuất hiện có 44,1 kHz, MP3/AAC 128 kbps; `high` giữ 48 kHz, MP3/AAC 192 kbps; WAV dùng PCM 16-bit ở cùng tần số. **GAP:** A7 chưa chốt bảng bitrate xuất; đây là chính sách bảo thủ, không phải cam kết cải thiện giọng TTS. VieNeu `standard` và `high` hiện đều dùng fp32/48 kHz; xuất lại không tạo lại đoạn giọng. `loudness` vẫn dùng hai lượt đo/áp dụng −16 LUFS, trần đỉnh −1,5 dBTP.
- Album luôn dùng tên truyện đầy đủ và tập, không dùng `shortTitle` hoặc tên tập trong dàn ý. Artist/album artist là `Noverse Audio`; không tự gán người sáng tác khi hợp đồng không có tác giả. Track chương rời giữ vị trí trong tập đầy đủ kể cả khi chỉ xuất một phần. Mốc chương giữ thời gian gốc và cập nhật nhãn khi ghi lại tag. Tải tệp hỗ trợ GET/HEAD và `Content-Disposition: filename*` UTF-8.

## 3. Web

- `internal/entry/web/audio.go` (GAP-17…20, SSE `GET S/audio/events`) · `internal/entry/web/tts.go` (GAP-21).
- `web/src/screens/audio/*` theo `Transcript.dc.html` (06) + `AudioExport.dc.html` (07), tiến độ nền và điều khiển công việc nằm trong **jobs dock chung** qua `job.*` (không có thẻ audio nổi riêng), mục nav **Audio** bật, **Cài đặt › Giọng đọc** (trạng thái bộ đọc, cài đặt, engine, chất lượng, thiết bị, tự tạo khi chốt, giọng có sẵn + nghe thử).
- Thiếu `ffmpeg` hoặc bộ đọc chưa cài ⇒ màn Audio hiện thẻ hướng dẫn cài (nút **Cài bộ đọc trên máy**), các phần không cần giọng (soạn/duyệt bản đọc) vẫn dùng được.
