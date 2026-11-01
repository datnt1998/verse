# ainovel-cli (Bản Tiếng Việt & Đa Ngôn Ngữ)

<p align="center">
  <strong>Công cụ dòng lệnh (CLI) sáng tác tiểu thuyết dài kỳ tự động bằng AI với kiến trúc Đa Agent (Multi-Agent)</strong>
</p>

<p align="center">
  <img src="scripts/sample.gif" alt="ainovel-cli demo" width="800">
  <img src="scripts/novel.png" alt="ainovel-cli bg" width="800">
</p>

---

## 🌟 Điểm Nổi Bật Của Bản Việt Hóa

- 🇻🇳 **Giao diện TUI Việt hóa 100%**: Từ Setup Wizard cài đặt ban đầu, màn hình chào mừng, thanh trạng thái, Activity stream trực tiếp, bảng quản lý Provider/Model (`/config`, `/model`) đến thông báo lỗi và phím tắt đều được dịch sang Tiếng Việt chuẩn mực, tự nhiên.
- 🌐 **Ngôn ngữ sáng tác truyện**: Toàn bộ dàn ý, nhân vật, bối cảnh thế giới và nội dung từng chương được sinh ra bằng **Tiếng Việt** tự nhiên, mượt mà.
- 🦙 **Tích hợp sẵn Ollama Local**: Hỗ trợ chạy 100% offline, miễn phí không tốn tiền API với các model mã nguồn mở chạy trên GPU nội bộ (Qwen 2.5 / 3.5, Llama, v.v.).
- ✍️ **Bộ Prompt Tiếng Việt & Văn phong chống AI sáo rỗng**: 14 prompt hệ thống được tinh chỉnh kỹ lưỡng, kèm file quy chuẩn văn phong tiểu thuyết Tiếng Việt (`assets/voice.md`) giúp hành văn sống động, gãy gọn, có chiều sâu, bài trừ các mẫu câu sáo rỗng của AI.
- 🚀 **Đồng bộ toàn diện Upstream mới nhất**: Sở hữu đầy đủ kiến trúc Đa Agent (`Coordinator` → `Architect` → `Writer` → `Editor` → `Arbiter`), Prompt Caching 3 tầng, quy hoạch cuộn 2 tầng (Rolling planning), điểm phục hồi step-level, và toàn bộ 14 lệnh slash commands.

---

## 📑 Mục Lục
1. [Yêu Cầu Hệ Thống](#1-yêu-cầu-hệ-thống)
2. [Cài Đặt & Khởi Chạy Nhanh](#2-cài-đặt--khởi-chạy-nhanh)
3. [Tùy Chọn Ngôn Ngữ Sáng Tác](#3-tùy-chọn-ngôn-ngữ-sáng-tác)
4. [Cấu Hình Nhà Cung Cấp AI (LLM)](#4-cấu-hình-nhà-cung-cấp-ai-llm)
   - [Dùng Ollama Cục Bộ (100% Offline)](#dùng-ollama-cục-bộ-100-offline)
   - [Dùng gói ChatGPT Plus/Pro (Codex Subscription)](#dùng-gói-chatgpt-pluspro-codex-subscription)
   - [Dùng gói SuperGrok / X Premium+ (xAI, đăng nhập OAuth)](#dùng-gói-supergrok--x-premium-xai-đăng-nhập-oauth)
   - [Dùng gói Claude Pro/Max (Anthropic, đăng nhập OAuth)](#dùng-gói-claude-promax-anthropic-đăng-nhập-oauth)
   - [Dùng Cloud API (OpenRouter, Gemini, Claude, OpenAI, DeepSeek)](#dùng-cloud-api-openrouter-gemini-claude-openai-deepseek)
   - [Phối hợp nhiều Model theo vai trò](#phối-hợp-nhiều-model-theo-vai-trò)
5. [Hướng Dẫn Sử Dụng & Bảng Lệnh TUI](#5-hướng-dẫn-sử-dụng--bảng-lệnh-tui)
   - [Khởi động TUI & Chế độ sáng tác](#khởi-động-tui--chế-độ-sáng-tác)
   - [Danh sách Lệnh Điều Khiển (Slash Commands)](#danh-sách-lệnh-điều-khiển-slash-commands)
   - [Can thiệp thời gian thực (Steer)](#can-thiệp-thời-gian-thực-steer)
   - [Chế độ chạy ngầm (Headless)](#chế-độ-chạy-ngầm-headless)
   - [Giao diện Web / điện thoại (`serve`)](#giao-diện-web--điện-thoại-serve)
6. [Quản Lý Nhiều Bộ Truyện Độc Lập](#6-quản-lý-nhiều-bộ-truyện-độc-lập)
7. [Tính Năng Nâng Cao](#7-tính-năng-nâng-cao)
   - [Chẩn đoán truyện (`/diag`)](#chẩn-đoán-truyện-diag)
   - [Mô phỏng văn phong (`/simulate`)](#mô-phỏng-văn-phong-simulate)
   - [Đồng bộ sửa đổi thủ công (`/sync`)](#đồng-bộ-sửa-đổi-thủ-công-sync)
   - [Nhập truyện bên ngoài (`/import`)](#nhập-truyện-bên-ngoài-import)
   - [Xuất truyện hoàn chỉnh (`/export` TXT/EPUB)](#xuất-truyện-hoàn-chỉnh-export-txtepub)
8. [Kiến Trúc Kỹ Thuật & Nguyên Lý Hoạt Động](#8-kiến-trúc-kỹ-thuật--nguyên-lý-hoạt-động)
   - [Kiến trúc Đa Agent](#kiến-trúc-đa-agent)
   - [Quy hoạch cuộn 2 tầng (Rolling Planning)](#quy-hoạch-cuộn-2-tầng-rolling-planning)
   - [Quản lý & Nén ngữ cảnh 4 cấp](#quản-lý--nén-ngữ-cảnh-4-cấp)
   - [Đánh giá chất lượng 7 chiều của Editor](#đánh-giá-chất-lượng-7-chiều-của-editor)
   - [Khôi phục điểm ngắt (Step-level Recovery)](#khôi-phục-điểm-ngắt-step-level-recovery)
9. [Cấu Trúc Thư Mục Đầu Ra](#9-cấu-trúc-thư-mục-đầu-ra)
10. [Tùy Biến Văn Phong & Quy Tắc Cá Nhân](#10-tùy-biến-văn-phong--quy-tắc-cá-nhân)
11. [Tech Stack & License](#11-tech-stack--license)

---

## 1. Yêu Cầu Hệ Thống

- **Khuyến nghị nhất**: [Docker](https://www.docker.com/) & Docker Desktop (Windows / macOS / Linux).
- **Hoặc chạy từ Source**: Máy tính đã cài đặt [Go](https://go.dev/) ≥ 1.26.
- **LLM**: 
  - Máy có GPU (Nvidia VRAM ≥ 12GB) nếu muốn chạy Ollama cục bộ với model Qwen 2.5 / 3.5.
  - Hoặc API Key từ OpenRouter, OpenCode Go, Anthropic, Google Gemini, OpenAI, DeepSeek; hoặc gói thuê bao **ChatGPT Plus/Pro** (`ainovel-cli login`) / **Claude Pro/Max** (`ainovel-cli login anthropic-oauth`) / **SuperGrok, X Premium+** (`ainovel-cli login xai-oauth`).

---

## 2. Cài Đặt & Khởi Chạy Nhanh

### Bước 1: Clone Repo & Chuẩn Bị Thư Mục
```bash
git clone https://github.com/datnt1998/verse.git
cd verse/apps/noverse

# Tạo thư mục chứa cấu hình và thư mục chứa truyện
mkdir -p config workspace novels
```

### Bước 2: Build Docker Image
```bash
docker compose build
```
*(Nếu build từ source Go: `go build -o ainovel-cli ./cmd/ainovel-cli`, chạy trong `apps/noverse`)*

> Từ 2026-10-29, Noverse nằm trong monorepo `datnt1998/verse`. Repo cũ
> `datnt1998/noverse` được đóng băng; bản phát hành mới và lệnh tự cập nhật
> lấy từ `datnt1998/verse`. Binary đã cài từ repo cũ sẽ không thấy bản mới:
> cài lại một lần bằng `scripts/install.sh` của repo mới.

### Bước 3: Khởi Chạy TUI
```bash
docker compose run --rm ainovel
```
*(Nếu chạy từ file binary: `./ainovel-cli`)*

Lần đầu chạy, hệ thống sẽ tự động bật **Setup Wizard tương tác bằng Tiếng Việt** để bạn chọn Provider, nhập API Key/Base URL và chọn Model.

> Image chạy bằng user không phải root (`UID 10001`). Nếu bind-mount `./config`
> hoặc `./workspace`, cấp quyền ghi cho UID 10001 (ví dụ `chown -R 10001:10001 config workspace`).

### Bước 4 (tùy chọn): Chạy web hosted (Dokploy)

Image đa tầng có sẵn web build, ffmpeg/ffprobe, `/healthz` + `/readyz`,
trusted-proxy và Secure cookie. Hướng dẫn Dokploy (Traefik, Postgres, BlobStore
S3, job `storage migrate up`, worker TTS qua Tailscale, backup/restore, cutover
và rollback) nằm ở [deploy/README.md](deploy/README.md). Chạy thử toàn bộ
topology trên máy local:

```bash
cd deploy && docker compose -f docker-compose.dokploy.yml up -d --build
```

---

## 3. Ngôn Ngữ Sáng Tác

Nội dung truyện luôn được sinh ra bằng **Tiếng Việt**: dàn ý, nhân vật, bối cảnh thế giới, quy chuẩn văn phong chống AI và từng chương. Giao diện TUI cũng hiển thị 100% bằng Tiếng Việt.

> 💡 **Ghi chú**: Bảng điều khiển TUI, thanh trạng thái, menu và các thông báo lỗi luôn hiển thị **100% bằng Tiếng Việt**.

---

## 4. Cấu Hình Nhà Cung Cấp AI (LLM)

File cấu hình đặt tại `config/config.json`:

### Dùng Ollama Cục Bộ (100% Offline)

1. **Tạo model trên Ollama với Context Window lớn (65536 tokens)**:
   - Trên PowerShell (Windows):
     ```powershell
     @"
     FROM qwen2.5:14b
     PARAMETER num_ctx 65536
     "@ | Out-File -FilePath "$env:TEMP\ainovel.Modelfile" -Encoding ascii

     ollama create ainovel-qwen -f "$env:TEMP\ainovel.Modelfile"
     ```
2. **Nội dung `config/config.json`**:
   ```json
   {
     "provider": "ollama",
     "model": "ainovel-qwen",
     "providers": {
       "ollama": {
         "base_url": "http://host.docker.internal:11434/v1",
         "stream_idle_timeout": "300s"
       }
     },
     "context_window": 65536,
     "thinking": "off",
     "style": "default"
   }
   ```
   > ⚠️ **Lưu ý mạng Docker**: Khi chạy bằng Docker, bắt buộc dùng `http://host.docker.internal:11434/v1`. Nếu chạy binary trực tiếp trên máy không qua Docker, dùng `http://localhost:11434/v1`.

---

### Dùng gói ChatGPT Plus/Pro (Codex Subscription)

Dùng hạn mức gói ChatGPT thay cho API Key (cùng cơ chế với Codex CLI / omp):

```bash
ainovel-cli login            # mở trình duyệt đăng nhập ChatGPT (callback localhost:1455)
ainovel-cli login --device   # SSH/headless: nhập mã thiết bị tại auth.openai.com/codex/device
ainovel-cli login --status   # xem tài khoản + danh sách model khả dụng của gói
ainovel-cli logout           # xoá phiên đăng nhập
```

```json
{
  "provider": "openai-codex",
  "model": "gpt-5.6-terra",
  "providers": {
    "openai-codex": {}
  }
}
```

- Credential lưu ở `~/.ainovel/auth.json` (quyền 0600) và tự làm mới khi sắp hết hạn hoặc khi server trả 401; nhiều tiến trình ainovel dùng chung an toàn nhờ khoá file.
- Setup Wizard và `/config` có sẵn mục **ChatGPT Plus/Pro (Codex Subscription)**: chọn rồi đăng nhập ngay trong ứng dụng.
- Danh sách model phụ thuộc gói ChatGPT; tên model trong config phải nằm trong `ainovel-cli login --status`.
- Backend Codex không nhận `temperature`/`top_p`/giới hạn token output: các tham số này (kể cả trong `extra_body`) bị tự động bỏ qua.
- Chi phí hiển thị trong TUI/budget là giá API tương đương (nếu model có trong bảng giá), không phải số tiền thực trả cho gói.

---

### Dùng gói SuperGrok / X Premium+ (xAI, đăng nhập OAuth)

Dùng hạn mức gói thuê bao xAI thay cho API Key (cùng luồng đăng nhập với Grok CLI / Hermes Agent, được xAI công nhận):

```bash
ainovel-cli login xai-oauth   # in link + mã thiết bị: mở link trên bất kỳ máy nào rồi xác nhận
ainovel-cli login --status    # xem tài khoản + danh sách model khả dụng
ainovel-cli logout xai-oauth  # xoá phiên đăng nhập
```

```json
{
  "provider": "xai-oauth",
  "model": "grok-4.6",
  "providers": {
    "xai-oauth": {}
  }
}
```

- xAI **chỉ có luồng mã thiết bị** (RFC 8628), không có callback localhost: Web và TUI hiển thị link + mã để bạn xác nhận trên trình duyệt.
- Gọi `https://api.x.ai/v1` với token OAuth (Chat Completions); token tự làm mới trước khi hết hạn và khi server trả 401. `base_url` nếu đặt phải là https trên miền x.ai.
- Provider này khác `grok` (API key trả phí). Nếu sau khi đăng nhập xAI trả **403**, gói của bạn chưa có quyền API qua OAuth: dùng `grok` với API key hoặc nâng cấp gói.
- Model mặc định `grok-4.6`; danh sách thật lấy từ `GET /v1/models` (rơi về danh sách tĩnh `grok-4.6`, `grok-4.3`, `grok-4.20-0309-reasoning`, `grok-4.20-0309-non-reasoning` nếu không lấy được). Thêm `context_window` cho model trong config nếu biết cửa sổ thực.

---

### Dùng gói Claude Pro/Max (Anthropic, đăng nhập OAuth)

Dùng hạn mức gói thuê bao Claude thay cho API Key (cùng luồng đăng nhập và dấu vân tay Claude Code với omp / oh-my-pi):

```bash
ainovel-cli login anthropic-oauth            # mở trình duyệt trên chính máy này (callback localhost:54545)
ainovel-cli login anthropic-oauth --device   # SSH/headless: mở link ở máy bất kỳ rồi dán mã `code#state` (hoặc URL chuyển hướng) vào terminal
ainovel-cli login --status                   # xem tài khoản (email · tổ chức) + danh sách model khả dụng
ainovel-cli logout anthropic-oauth           # xoá phiên đăng nhập
```

```json
{
  "provider": "anthropic-oauth",
  "model": "claude-sonnet-4-6",
  "providers": {
    "anthropic-oauth": {}
  }
}
```

- **Web**: Tuỳ chỉnh › Nhà cung cấp AI (hoặc bước Nhà cung cấp AI khi thiết lập lần đầu) › Claude Pro/Max. Mở từ thiết bị khác máy chạy `serve`: bấm đăng nhập, mở link, rồi dán mã `code#state` mà trang Claude hiển thị (hoặc cả URL chuyển hướng) vào ô “Dán mã từ trang Claude”.
- Gọi `https://api.anthropic.com/v1/messages?beta=true` với `Authorization: Bearer` (không `x-api-key`), header/anthropic-beta/khối system nhận dạng Claude Code, tiền tố `_` cho tên tool (chỉ trên wire). Token tự làm mới trước khi hết hạn và khi server trả 401. `base_url` nếu đặt phải là https trên miền anthropic.com.
- Provider này khác `anthropic` (API key trả phí, không đổi). **403** = gói chưa có quyền API qua OAuth; **429** = chạm hạn mức gói (theo phiên/tuần) — hãy chờ hoặc dùng `anthropic` với API key. Không gửi beta `context-1m`, nên cửa sổ ngữ cảnh là 200K.
- Model mặc định `claude-sonnet-4-6`; danh sách thật lấy từ `GET /v1/models` (rơi về danh sách tĩnh `claude-opus-4-8`, `claude-sonnet-4-6`, `claude-haiku-4-5`… nếu không lấy được). Đặt biến `AINOVEL_CLAUDE_CODE_VERSION` để ghim version Claude Code trên wire khi Anthropic yêu cầu bản mới hơn.

---

### Dùng Cloud API (OpenRouter, Gemini, Claude, OpenAI, DeepSeek)

#### OpenRouter:
```json
{
  "provider": "openrouter",
  "model": "anthropic/claude-3.5-sonnet",
  "providers": {
    "openrouter": {
      "api_key": "sk-or-v1-YOUR_OPENROUTER_API_KEY"
    }
  },
  "context_window": 128000,
  "thinking": "off",
  "style": "default"
}
```

#### OpenCode Go (gói thuê bao, dùng API key):
Lấy khoá ở https://opencode.ai/auth. Một khoá dùng được Kimi, DeepSeek, GLM, Qwen, MiniMax… Danh sách model: `GET https://opencode.ai/inference/go/v1/models`.
```json
{
  "provider": "opencode-go",
  "model": "kimi-k2.7-code",
  "providers": {
    "opencode-go": {
      "api_key": "YOUR_OPENCODE_API_KEY",
      "models": [{ "name": "kimi-k2.7-code", "context_window": 262144 }]
    }
  }
}
```
Kimi và GLM không cho tắt suy luận (`thinking: "off"` sẽ bị API từ chối); DeepSeek, Qwen cho tắt.

#### Google Gemini:
```json
{
  "provider": "gemini",
  "model": "gemini-2.5-pro",
  "providers": {
    "gemini": {
      "api_key": "YOUR_GEMINI_API_KEY"
    }
  },
  "context_window": 1000000,
  "style": "default"
}
```

#### DeepSeek API:
```json
{
  "provider": "deepseek",
  "model": "deepseek-chat",
  "providers": {
    "deepseek": {
      "api_key": "YOUR_DEEPSEEK_API_KEY"
    }
  },
  "context_window": 64000,
  "style": "default"
}
```

---

### Phối hợp nhiều Model theo vai trò

Hệ thống cho phép gán model mạnh (Claude 3.5 Sonnet / DeepSeek Reasoner) làm **Biên tập / Kiến trúc sư** và model rẻ/nhanh (DeepSeek Chat / Ollama) làm **Người viết**:

```json
{
  "provider": "ollama",
  "model": "ainovel-qwen",
  "providers": {
    "ollama": { "base_url": "http://host.docker.internal:11434/v1" },
    "openrouter": { "api_key": "sk-or-v1-YOUR_KEY" }
  },
  "roles": {
    "architect": { "provider": "openrouter", "model": "anthropic/claude-3.5-sonnet" },
    "editor": { "provider": "openrouter", "model": "anthropic/claude-3.5-sonnet" },
    "writer": { "provider": "ollama", "model": "ainovel-qwen" }
  },
  "context_window": 65536,
  "style": "default"
}
```

---

## 5. Hướng Dẫn Sử Dụng & Bảng Lệnh TUI

### Khởi động TUI & Chế độ sáng tác

Chạy lệnh:
```bash
docker compose run --rm ainovel
```

Tại màn hình chào mừng:
- **Phím Tab**: Chuyển đổi giữa 2 chế độ:
  - **Bắt đầu nhanh**: Nhập 1 câu tóm tắt ý tưởng (ví dụ: *"Tiểu thuyết tiên hiệp phàm nhân, nhân vật chính cơ trí, quyết đoán"*), AI tự động lập dàn ý và sáng tác ngay.
  - **Đồng sáng tác (Co-create)**: AI sẽ trao đổi cùng bạn từng bước để làm rõ thiết lập thế giới, nhân vật, cốt truyện trước khi viết.
- **Phím Enter**: Bắt đầu quá trình sáng tác.
- **Phím `/`**: Mở thanh tìm kiếm và thực thi lệnh nhanh (Slash Commands).
- **`Ctrl+C` 2 lần**: Lưu an toàn toàn bộ trạng thái và thoát ra.

---

### Danh sách Lệnh Điều Khiển (Slash Commands)

Khi đang ở trong giao diện TUI, bạn có thể gõ `/` để mở bảng chọn lệnh:

| Lệnh | Mô tả |
| :--- | :--- |
| `/help` | Mở bảng trợ giúp tra cứu danh sách lệnh và phím tắt |
| `/model` | Chuyển đổi Model hoặc mức độ suy luận (reasoning/thinking) |
| `/config` | Quản lý cấu hình Provider, Model ID, API Key, Base URL, Context Window |
| `/diag` | Xem báo cáo chẩn đoán toàn diện về sức khỏe, tiến độ và chất lượng truyện |
| `/review [on\|off]` | Bật/tắt chế độ nghiệm thu từng chương (dừng lại sau mỗi chương để bạn duyệt) |
| `/next` | Phê duyệt cho phép viết chương tiếp theo (khi ở chế độ nghiệm thu) |
| `/start <tệp>` | Đọc tệp dàn ý / ý tưởng bên ngoài để bắt đầu truyện mới |
| `/import <tệp>` | Nhập tiểu thuyết từ bên ngoài vào để AI phân tích và viết tiếp |
| `/reopen <hướng>` | Viết tiếp tập mới sau khi tác phẩm đã hoàn thành |
| `/cocreate` | Tạm dừng để vào chế độ đồng sáng tác định hướng giai đoạn tiếp theo |
| `/simulate` | Phân tích các file văn mẫu trong `./simulate` để mô phỏng văn phong |
| `/importsim <file>` | Nhập hồ sơ mô phỏng văn phong từ tệp json |
| `/sync` | Đồng bộ các chỉnh sửa thủ công của bạn trên các file chương vào hệ thống |
| `/export` | Xuất tác phẩm thành file văn bản hoàn chỉnh (.txt hoặc .epub) |

---

### Can thiệp thời gian thực (Steer)

Trong lúc AI đang viết, bạn có thể nhập trực tiếp ý kiến sửa đổi vào ô nhập liệu bên dưới bất cứ lúc nào mà **không cần tạm dừng hay khởi động lại**:

```text
❯ Cho nhân vật phụ A hy sinh ở cuối chương này để tạo bước ngoặt cảm xúc lớn
```

Sau khi nhấn Enter, Arbiter sẽ tự động đánh giá phạm vi ảnh hưởng và điều phối Writer/Editor cập nhật mạch truyện.

---

### Chế độ chạy ngầm (Headless)

Dành cho việc chạy tự động trên VPS, Server hoặc CI:
```bash
# Bắt đầu truyện mới
docker compose run --rm ainovel --headless --prompt "Tiểu thuyết huyền nghi đô thị phá án"

# Viết tiếp truyện đang dở trong thư mục hiện tại
docker compose run --rm ainovel --headless
```

---

### Giao diện Web / điện thoại (`serve`)

Chạy một web server cục bộ để viết, đọc, duyệt và xuất truyện từ trình duyệt máy tính hoặc điện thoại:
```bash
cd ~/Projects/truyen                   # thư mục thư viện
ainovel-cli serve                      # chỉ máy này: http://127.0.0.1:8686
ainovel-cli serve --root DIR --addr 127.0.0.1:9000
```
- **Thư viện 3 cấp**: `--root` là thư viện → mỗi thư mục con có `project.json` là một **dự án** → mỗi thư mục con trong `<dự án>/kich_ban/` (đổi được ở Cài đặt › Thư viện) là một **truyện**. Dữ liệu truyện vẫn ở `<thư mục truyện>/output/novel` như khi chạy TUI trong thư mục đó.
  ```
  truyen/                        ← thư viện (.ainovel/library.json)
  └── dit-dang-audio/            ← dự án (project.json, y-tuong/, tai-san/, .ainovel/config.json, .ainovel/rules/)
      └── kich_ban/01-tra-xanh/  ← truyện (story.json, output/novel/, xuat-ban/)
  ```
  Thư mục cũ nằm thẳng trong `--root` (có `output/` hoặc `.ainovel/`) được gom vào dự án ẩn danh `_`. Thư mục có sẵn `kich_ban/` nhưng chưa có `project.json` có thể "nhận làm dự án" trên web.
- **Cấu hình & quy tắc kế thừa 3 cấp** (áp dụng cho cả TUI và web): `~/.ainovel/config.json` → `<dự án>/.ainovel/config.json` → `<truyện>/.ainovel/config.json`; quy tắc `~/.ainovel/rules` → dự án → truyện, cùng tên tệp thì cấp gần hơn thắng. `api_key` chỉ đặt ở cấp toàn cục và không bao giờ gửi ra trình duyệt.
- **Xuất tự động**: bật ở Cài đặt › Thư viện; khi nộp chương / xong tập / xong truyện sẽ xuất TXT/EPUB vào `<truyện>/xuat-ban/`.
- **Tài khoản**: lần chạy đầu `serve` hỏi tạo tài khoản quản trị; lần đăng nhập đầu trên web có trình hướng dẫn (hồ sơ, mật khẩu, nhà cung cấp/OAuth, mô hình, thư viện, dự án đầu tiên, điện thoại & người cùng viết). Link in ra không chứa mã bí mật nào.
  ```bash
  ainovel-cli serve user add thu-am --role member --projects dit-dang-audio=view   # hỏi mật khẩu
  ainovel-cli serve user list | passwd <tên> | rm <tên>
  ```
  Quản trị viên cũng quản lý thành viên trên web: Tuỳ chỉnh › Thành viên (tạo tài khoản với mật khẩu tạm chỉ hiện một lần, sửa quyền, đăng xuất từng thiết bị hoặc tất cả, khoá/xoá kèm lý do, nhật ký quản trị lưu 1 năm). Quyền được kiểm ở từng yêu cầu nên đổi xong có hiệu lực ngay.
  | Vai trò | Quyền |
  |---|---|
  | `admin` | Toàn quyền, mọi dự án, cấu hình toàn cục, quản lý thành viên |
  | `member` | Chỉ các dự án được giao trong `--projects`, theo từng mức bên dưới |

  | Quyền dự án | Được làm |
  |---|---|
  | `owner` | Như `collab`, thêm xoá và chia sẻ |
  | `collab` | Đọc và viết, tạo truyện trong dự án |
  | `view` | Chỉ đọc, nghe, xem tiến độ, xuất |

  Tệp tài khoản cũ (`books[]`, vai trò `writer`/`reader`) tự chuyển sang mô hình mới khi `serve` khởi động với thư viện thật. Quyền `*` được giải thành các dự án trong thư viện đó. Nếu chưa biết thư viện, ví dụ chạy `serve user …` không có `--root`, hoặc quét thư viện lỗi, thì việc chuyển được hoãn: dữ liệu cũ giữ nguyên và vẫn đăng nhập được. Bản gốc được giữ ở `web-accounts.json.v2.bak` (và `web-accounts.json.v3.bak` cho bước v3→v4, khi mật khẩu "phải đổi" được chuyển thành mật khẩu tạm một lần có hạn 24h).

  Chủ dự án (`owner`) cũng quản lý thành viên của riêng dự án mình qua API dành cho owner: `GET|POST /api/projects/{p}/members`, `PATCH|DELETE /api/projects/{p}/members/{user}` (đổi `collab`/`view` hoặc gỡ), `POST /api/projects/{p}/owner` (chuyển quyền sở hữu nguyên tử, luôn giữ đúng một owner). Owner không vào được `/api/users` (chỉ admin) và không thể gỡ owner cuối cùng. `collab` được phép gỡ "Cảnh báo" (DELETE keep); các DELETE khác vẫn chỉ dành cho owner.
- **Điện thoại & máy tính bảng**: giao diện tự đổi theo màn hình (≤ 640 px: 4 tab Viết · Đọc · Nghe · Thêm; 641–1099 px: bố cục máy tính bảng hai cột). Đăng nhập trên máy tính → menu tài khoản → "Đăng nhập điện thoại bằng QR", quét mã bằng camera. Mã QR dùng một lần, hết hạn sau 5 phút; quản trị viên cũng tạo được QR cho tài khoản của người khác. Có thể "Thêm vào màn hình chính" (PWA); ngoại tuyến vẫn giữ góp ý/lệnh và tự gửi lại khi có mạng.
- **Thông báo đẩy** (Web Push, Cài đặt › Thông báo hoặc Thêm › Thông báo): chương chờ duyệt, lỗi, việc nền xong, đăng nhập lạ; có giờ yên lặng và gom thông báo. Trình duyệt chỉ cho Web Push qua HTTPS hoặc `localhost`. Khoá VAPID tự tạo ở `~/.ainovel/push/vapid.json`.
- Mỗi phiên đăng nhập (mật khẩu hoặc QR) hết hạn sau **30 ngày**; trong menu tài khoản có thể đăng xuất từng thiết bị hoặc "Đăng xuất mọi thiết bị khác". Đổi mật khẩu đăng xuất **mọi** thiết bị, kể cả thiết bị đang dùng, rồi đưa về màn đăng nhập. Khi quản trị viên đặt lại mật khẩu, khoá tài khoản hay đổi quyền vào app, người đó cũng bị đăng xuất khỏi mọi thiết bị. Dùng mã khôi phục thì mọi phiên cũ bị thu hồi và thiết bị vừa nhập mã được đăng nhập luôn. Sai mật khẩu 5 lần liên tiếp sẽ bị khoá tạm, thời gian khoá tăng dần (tối đa 15 phút).
- Với `STORE_DRIVER=file` (mặc định của CLI/offline), tài khoản lưu ở `~/.ainovel/web-accounts.json` (quyền 0600, mật khẩu băm argon2id) và cookie phiên là `ainovel_session`, không dùng chung với app nào. File `~/.ainovel/web-token` của phiên bản cũ không còn dùng, có thể xoá.
- **Tài khoản trên PostgreSQL**: `STORE_DRIVER=postgres` chọn trọn bộ backend (tài khoản/phiên/thành viên + dữ kiện truyện), không còn cờ `PG_ACCOUNTS` thứ hai, không còn chế độ hybrid file/postgres.
- **Đăng nhập chung với Viderse** (chỉ `STORE_DRIVER=postgres`): hai app dùng cùng kho tài khoản và cùng cookie `verse_session`, nên đăng nhập ở app này là vào được app kia, đăng xuất hay đổi mật khẩu ở một app là ra khỏi cả hai. Mỗi tài khoản có quyền vào từng app: quản trị viên hệ thống vào được cả hai; thành viên cần được cấp riêng (tài khoản mới mặc định có Noverse). Đăng nhập được nhưng chưa có quyền vào app thì trang hiện "Tài khoản của bạn chưa được cấp quyền dùng Noverse. Vui lòng liên hệ quản trị viên." (403 `app_access_denied`), không đưa về màn đăng nhập. Quản trị viên cấp/thu hồi ở Tuỳ chỉnh › Thành viên (ô Noverse/Viderse) hoặc bằng lệnh; mỗi lần đổi quyền đều ghi nhật ký quản trị và đăng xuất người được đổi khỏi mọi thiết bị:
  ```bash
  ainovel-cli serve user grant ban --app viderse
  ainovel-cli serve user revoke ban --app viderse
  ```
  Chạy cục bộ: hai app bind `127.0.0.1` khác cổng và cùng `VERSE_APP_ORIGINS` (cookie không phân biệt cổng nên đừng mở app khác không tin cậy trên cùng host). Chạy hosted: `VERSE_COOKIE_DOMAIN` (domain cha) + `SECURE_COOKIES`, app chỉ ra ngoài qua Traefik. Biến môi trường, role Postgres và cách chuyển tài khoản cũ: [deploy/README.md](deploy/README.md).
- **Dữ kiện truyện trên PostgreSQL**: backend `postgres.Backend` cung cấp đủ 19 domain của `store.Store` (truyện/chương/version bất biến/outline/nhân vật/thế giới/review/checkpoint/agent session/…), chạy đúng contract suite của file backend. HTTP đọc dữ kiện truyện đi qua `Gateway.SetFactStore` (được `serve` nối vào `bootstrap.OpenStoryStore` khi `STORE_DRIVER=postgres`) — không đọc thư mục ngầm, không fallback. Engine/worker của truyện hosted mở kho scoped qua env (không dùng thư mục file).
- **Cấu hình, quy tắc, văn mẫu và media trên PostgreSQL**: cấu hình cấp dự án/truyện (kế thừa Mọi truyện → dự án → truyện, ETag + 409 khi hai nơi cùng lưu), quy tắc viết ở cả ba cấp, văn mẫu, bìa truyện (`?size=`/`?volume=`, có cache), tài sản dự án (bìa, nhạc nền), peaks sóng âm, xuất văn bản và danh sách bản xuất audio đều chạy trên backend Postgres; worker hosted nạp cấu hình đã trộn và quy tắc/văn mẫu từ DB, không đọc thư mục.
- **Docker: thư viện truyện phải nằm trên volume**: image đặt `NOVERSE_CONTAINER=1` và thư viện mặc định `/var/lib/noverse/library` (đã khai báo `VOLUME`). Nếu thư mục thư viện không nằm trên mount bền, log và web hiện “Thư mục này sẽ mất khi cập nhật container — hãy gắn volume.” Chi tiết và cách chuyển dữ liệu cũ: `deploy/README.md` › *File mode + bind mount*.
- **Nhà cung cấp lỗi không còn khoá app**: một mục nhà cung cấp sai mà không được model mặc định/role nào dùng chỉ hiện cảnh báo ở Cài đặt › Nhà cung cấp; app chỉ chuyển sang chỉ đọc khi model đang dùng thật sự không chạy được, kèm lý do tiếng Việt. Lưu cấu hình có nhà cung cấp sai trả 400. `openai-codex` chấp nhận `"api": "responses"` (thừa, được bỏ khi lưu).
- Thành viên vào Noverse bằng địa chỉ mà quản trị viên gửi (hộp tạo/đặt lại mật khẩu có sẵn dòng “Mở Noverse: <địa chỉ>” và nút chép lời nhắn). Khi chạy sau proxy hãy đặt `PUBLIC_URL` (https); `serve` chỉ nghe ở `127.0.0.1` thì chỉ máy này vào được — không còn chế độ mở cho Wi‑Fi/LAN.
- Mỗi truyện viết trong một tiến trình riêng, mở khi cần. Truyện đang mở ở TUI vẫn **đọc, duyệt, xuất** được trên web; chỉ các thao tác viết bị khoá (báo lỗi 409) cho tới khi đóng TUI.
- Mở truyện không tự viết tiếp; bấm "Tiếp tục" khi có phiên dang dở.
- **Dữ liệu luôn mới**: mọi màn hình nghe một luồng sự kiện chung (`/api/events`); sửa tệp truyện/cấu hình từ bên ngoài (TUI, trình soạn thảo) hiện lên web sau ~2 giây. Hai nơi cùng sửa một cấu hình thì nơi lưu sau nhận thanh "Đã có thay đổi từ nơi khác" (xem bản mới / ghi đè) thay vì âm thầm ghi đè.
- **Luồng viết trực tiếp** là lịch sử sáng tác, không phải bộ đệm của tiến trình worker: tạm dừng, đổi thẻ và khởi động lại worker không được xoá nội dung đã hiện. Xem [lưu trữ và phục hồi luồng](internal/entry/web/stream_history.go) và [kiểm thử với PostgreSQL](internal/entry/web/stream_history_test.go); thư mục tạm của worker hosted không phải nơi lưu lịch sử.
- **Việc chạy nền**: viết, xuất, tạo audio, nhập, đồng bộ, chẩn đoán hiện ở dock góc dưới phải (`⌥J`), có tạm dừng / tiếp / huỷ / thử lại và "Báo khi xong".
- **Máy đọc TTS ở nhà (worker Mac, phase 07)**: VPS không nạp model giọng đọc. Cấp token trên máy chủ (`STORE_DRIVER=postgres`) rồi chạy worker trên Mac — worker chỉ gửi yêu cầu ra ngoài (không mở cổng, không kết nối Postgres) qua Tailscale tới listener riêng của máy chủ:
  ```bash
  # trên máy chủ: token chỉ hiện một lần, DB chỉ giữ SHA-256
  ainovel-cli worker token create --cap tts mac-mini   # cờ đứng TRƯỚC tên máy
  ainovel-cli worker token list                        # không bao giờ in token
  ainovel-cli worker token revoke <uuid|tên>
  ```
  ```bash
  # trên Mac
  export TTS_WORKER_TOKEN='…'                 # bí mật chỉ nằm trong môi trường, không truyền qua tham số
  ainovel-cli worker tts --api http://100.x.y.z:8081   # thêm --project <slug> nếu chỉ nhận một dự án
  ```
  `--api` thắng `TTS_WORKER_API_URL`. `--lease` chỉ là **yêu cầu**; máy chủ kẹp trong 30 giây–10 phút nên một Mac chết không giữ việc quá lâu. Worker nhận việc theo lease 120 giây, gia hạn mỗi 30 giây, tự dừng khi việc đã bị huỷ/thu hồi (không bao giờ phát lại kết quả cũ), và trả việc về hàng đợi khi bạn dừng bằng Ctrl‑C. Máy Mac tắt hay mất mạng thì việc vẫn nằm trong hàng đợi và hiện "đang chờ" — không có kết quả giả, không tự cài model lên VPS.

- **Góp ý theo đoạn**: chọn một đoạn khi đọc (điện thoại: nhấn giữ) để góp ý; khi bấm "Duyệt & viết tiếp" các góp ý được gửi cho Người viết rồi đánh dấu đã áp dụng.
- **Chưa có cấu hình mô hình** vẫn đọc, duyệt, xuất bình thường; các thao tác cần AI báo "Chưa có cấu hình mô hình" kèm đường tới Cài đặt › Nhà cung cấp.
- **Audio / sách nói** (`docs/design/v1/10-audio.md`, `15-audio-impl.md`): dựng kịch bản đọc theo vai, tạo giọng bằng [VieNeu-TTS](https://github.com/pnnbao97/VieNeu-TTS) chạy cục bộ, nghe trong trình duyệt và xuất M4B (có mốc chương) / MP3 / WAV + phụ đề `.srt`, chuẩn hoá −16 LUFS; "Xuất khi xong" tự xuất khi chương cuối có giọng. Cần mạng và ≥4 GiB trống cho lần cài đầu; `uv`, Python 3.12, mã nguồn VieNeu và mô hình được tự tải vào `~/.ainovel/voice` (không cần `brew install uv`). `ffmpeg` vẫn cần cho việc xuất âm thanh. Bộ đọc dùng chung qua gói `internal/voice` (API tương thích OpenAI `/v1/audio/speech`), module khác gọi lại được; cũng trỏ được tới một máy chủ VieNeu/OpenAI-speech có sẵn ở Cài đặt › Giọng đọc.
  ```bash
  ainovel-cli voice install        # tải uv ghim + Python 3.12 + mã nguồn ghim sha256 + model HF vào ~/.ainovel/voice
  ainovel-cli voice status         # đã cài chưa, đang chạy, tốc độ tạo
  ainovel-cli voice voices         # danh sách giọng có sẵn
  ainovel-cli voice say "Xin chào" --voice "Hải Đăng" -o thu.wav
  ```
  **Tạo lại sau khi sửa**: nút **Tạo lại** ở bản đọc và hàng đợi lấy chính văn hiện tại, soạn lại khi nguồn đã đổi rồi tự chốt và tạo các đoạn cần cập nhật. Bản đọc được mở khoá để sửa vẫn tạo lại được, không cần xoá âm thanh hay tự chốt trước. Cảnh báo phân biệt chương đã đổi với bản đọc đã đổi; dữ liệu cũ thiếu dấu vân tay không bị khẳng định là đã sửa, nhưng lần tạo lại tiếp theo vẫn soạn từ chính văn hiện tại. Khi xong, cảnh báo được cập nhật và trình phát tải phiên bản âm thanh mới.
- Thiết kế ở `docs/design/v1/` (hợp đồng API: `11-api-contract.md`). Mã nguồn giao diện ở `web/` (React + Vite); bản build nhúng sẵn vào binary từ `internal/entry/web/dist`; `npm run dev:mock` chạy giao diện với dữ liệu giả, không cần server.

---

## 6. Quản Lý Nhiều Bộ Truyện Độc Lập

Mặc định output sẽ lưu vào thư mục `workspace/`. Để viết nhiều bộ truyện khác nhau mà không bị xung đột, hãy đặt biến môi trường `NOVEL_DIR`:

```powershell
# Trên Windows PowerShell:
$env:NOVEL_DIR = ".\novels\tien-hiep-ky"
docker compose run --rm ainovel

# Viết bộ truyện khác:
$env:NOVEL_DIR = ".\novels\do-thi-di-nang"
docker compose run --rm ainovel
```

Cấu trúc thư mục đầu ra của mỗi truyện:
```text
novels/<tên-truyện>/output/novel/
├── chapters/            # Các chương hoàn chỉnh đã duyệt (.md)
├── drafts/              # Bản nháp và dàn ý chi tiết từng chương
├── reviews/             # Báo cáo đánh giá của Editor
├── summaries/           # Tóm tắt từng Cung và Tập
├── premise.md           # Ý tưởng & tiền đề cốt truyện
├── characters.md        # Hồ sơ thiết lập nhân vật
├── world_rules.md       # Thiết lập quy tắc thế giới
└── meta/                # Checkpoint, tiến độ, nhật ký token
```

---

## 7. Tính Năng Nâng Cao

### Chẩn đoán truyện (`/diag`)
Gõ `/diag` trong TUI để hệ thống tự động kiểm tra toàn bộ truyện theo 4 chiều:
- **Tiến trình**: Phát hiện vòng lặp viết lại, kẹt lệnh can thiệp, nhảy số chương.
- **Chất lượng**: Theo dõi điểm số đánh giá, tỷ lệ tuân thủ cốt truyện, độ dài chương bất thường.
- **Quy hoạch**: Kiểm tra các nút thắt/phục bút bị bỏ quên, dàn ý cạn kiệt, thiếu tóm tắt.
- **Ngữ cảnh**: Nhân vật bị mất tích, đứt gãy dòng thời gian.

### Mô phỏng văn phong (`/simulate`)
Đặt các file văn mẫu (truyện của tác giả bạn yêu thích) vào thư mục `simulate/` rồi gõ `/simulate`. AI sẽ phân tích nhịp điệu, cách dùng từ, cấu trúc câu và tạo hồ sơ mô phỏng để áp dụng vào truyện của bạn.

### Đồng bộ sửa đổi thủ công (`/sync`)
Nếu bạn mở file `chapters/05.md` ra chỉnh sửa câu chữ trực tiếp, gõ `/sync` trong TUI. Hệ thống sẽ quét qua SHA-256, tự động cập nhật lại tóm tắt, trạng thái nhân vật và phục bút mà không làm hỏng tiến trình sáng tác.

### Nhập truyện bên ngoài (`/import`)
Gõ `/import ./truyen_cu.txt`. Hệ thống sẽ:
1. Nhận diện ranh giới từng chương.
2. Trích xuất thiết lập nhân vật, thế giới và dòng sự kiện.
3. Tổng hợp dàn ý và sẵn sàng viết tiếp các chương mới liền mạch.

### Xuất truyện hoàn chỉnh (`/export` TXT/EPUB)
Gõ `/export` trong TUI hoặc chỉ định tham số:
```text
/export ./xuat_ban/truyen_full.txt
/export ./xuat_ban/truyen_ebook.epub
/export from=1 to=50 ./tap_1.epub
```

---

## 8. Kiến Trúc Kỹ Thuật & Nguyên Lý Hoạt Động

### Kiến trúc Đa Agent

```
┌─────────────────────────────────────────────────┐
│              Host / Engine (Xác định)            │
│  Đọc Store → Route → Chạy Worker → Lặp chu trình │
│  Khởi động / Can thiệp / Xử lý kẹt → Arbiter     │
└────┬──────────┬──────────┬─────────────┬────────┘
     │          │          │             │
 ┌───▼────┐ ┌───▼───┐ ┌────▼────┐   ┌────▼────┐
 │Architect│ │Writer │ │ Editor  │   │ Arbiter │
 │(LLM Vòng)│ │(LLM Vòng)│ │(LLM Vòng)│   │(LLM Hàm)│
 └───┬────┘ └───┬───┘ └────┬────┘   └─────────┘
     └──────────┼──────────┘
                │ Gọi Tools (IO + Checkpoint)
┌───────────────▼─────────────────────────────────┐
│                   Store                         │
│  Tiến độ / Checkpoints / Dàn ý / Bản thảo / ... │
└─────────────────────────────────────────────────┘
```

| Vai trò | Trách nhiệm | Công cụ sử dụng |
| :--- | :--- | :--- |
| **Arbiter** | Trọng tài ngữ nghĩa: chọn planner, phân luồng can thiệp, giải quyết bế tắc | Không có (Gọi LLM đơn, trả về quyết định cấu trúc) |
| **Architect** | Kiến trúc sư: sinh tên truyện, tóm tắt, tiền đề, dàn ý, hồ sơ nhân vật, quy tắc thế giới | `novel_context`, `save_book`, `save_foundation` |
| **Writer** | Người viết: tự chủ lên kế hoạch chương, viết nháp, tự kiểm tra và bàn giao | `novel_context`, `read_chapter`, `plan_chapter`, `draft_chapter`, `check_consistency`, `commit_chapter` |
| **Editor** | Biên tập viên: đọc bản thảo, đánh giá chất lượng 7 chiều, lập tóm tắt cung/tập | `novel_context`, `read_chapter`, `save_review`, `save_arc_summary`, `save_volume_summary` |

---

### Quy hoạch cuộn 2 tầng (Rolling Planning)

Khác với các công cụ AI thông thường lên dàn ý cứng một lần cho cả trăm chương khiến câu chuyện càng về sau càng rỗng và loãng, `ainovel-cli` áp dụng cơ chế **La bàn (Compass) + Tầm nhìn cuộn**:
- Ban đầu chỉ dựng khung 2 Tập (Volume) và dàn ý chi tiết cho Cung đầu tiên (Arc 1).
- Khi viết gần hết một Cung, Editor đánh giá tổng kết và Architect mới tiếp tục mở rộng Cung tiếp theo dựa trên diễn biến thực tế của truyện.

---

### Quản lý & Nén ngữ cảnh 4 cấp

Để viết được truyện dài **500+ chương** mà không bị tràn context window hay mất trí nhớ, hệ thống sử dụng đường ống nén 4 cấp:
1. **ToolResultMicrocompact**: Dọn dẹp kết quả trung gian của các lệnh tool cũ.
2. **LightTrim**: Cắt tỉa các đoạn text dài không còn cần thiết.
3. **StoreSummaryCompact**: Thay thế tin nhắn cũ bằng bản tóm tắt đã lưu trong Store (0 tốn LLM token).
4. **FullSummary**: Dùng prompt chuyên dụng tóm tắt ngữ cảnh tự sự (giữ vững trạng thái nhân vật, manh mối phục bút).

---

### Đánh giá chất lượng 7 chiều của Editor

Sau mỗi chương, Editor sẽ thẩm định nghiêm ngặt theo 7 tiêu chí (mỗi nhận xét bắt buộc phải trích dẫn câu văn làm bằng chứng):
1. **Tính nhất quán của thiết lập (Consistency)**
2. **Hành vi & Tính cách nhân vật (Character Behavior)**
3. **Nhịp điệu cốt truyện (Pacing)**
4. **Tính mạch lạc & Chuyển cảnh (Narrative Flow)**
5. **Gài gắm phục bút (Foreshadowing)**
6. **Móc câu giữ chân người đọc cuối chương (Hooks)**
7. **Chất lượng thẩm mỹ văn chương (Aesthetic Quality)**: Chi tiết miêu tả, thủ pháp nghệ thuật, sự khác biệt trong lời thoại, chất lượng từ ngữ, sức truyền cảm.

---

### Khôi phục điểm ngắt (Step-level Recovery)

Mỗi lần một công cụ thực thi thành công, hệ thống sẽ ghi ngay một checkpoint (`meta/checkpoints.jsonl`). Nếu bị mất điện, tắt máy, rớt mạng hay nhấn `Ctrl+C`:
- Khi mở lại, hệ thống đọc `progress.json` và checkpoint gần nhất.
- Tự động tiếp tục chính xác từ bước bị dừng lại (ví dụ: *"Đã xong draft chương 7, tiếp tục bước check_consistency"*).

---

## 9. Cấu Trúc Thư Mục Đầu Ra

Toàn bộ dữ liệu được lưu trong thư mục output:

```text
output/novel/
├── book.md             # Tên sách và tóm tắt giới thiệu
├── chapters/           # Bản thảo hoàn chỉnh từng chương (.md)
│   ├── 01.md
│   └── ...
├── summaries/          # Tóm tắt từng chương, cung, tập (JSON)
├── drafts/             # Bản nháp từng chương
├── reviews/            # Báo cáo đánh giá chi tiết của Editor
├── timeline.jsonl      # Nhật ký dòng thời gian
├── premise.md          # Tiền đề cốt truyện
├── outline.json        # Dàn ý chi tiết các chương
├── layered_outline.json# Dàn ý phân tầng nhiều tập
├── characters.json     # Hồ sơ nhân vật
├── world_rules.json    # Quy tắc thế giới
└── meta/
    ├── book.json       # Metadata tác phẩm
    ├── compass.json    # La bàn định hướng dài hạn
    ├── progress.json   # Trạng thái tiến độ hiện tại
    ├── foreshadow.json# Sổ tay quản lý phục bút
    └── checkpoints.jsonl # Checkpoint khôi phục từng bước
```

Bố cục trên là **hợp đồng đã khóa** (xem `docs/architecture.md` §4.5 và fixture `internal/store/testdata/storage-contract/`): `meta/book.json` là nguồn sự thật duy nhất cho tên/tóm tắt sách (`book.md` chỉ là bản chiếu), `meta/chapter_records/` là bản ghi nghiệm thu, các tệp `.jsonl` chỉ ghi nối đuôi. Kế hoạch nâng cấp đã chốt 2026-10-28: chế độ web chạy trên VPS sẽ đưa dữ liệu nghiệp vụ vào **PostgreSQL** (pgx + goose + sqlc), còn **CLI offline vẫn dùng file** như hiện nay; worker TTS chạy trên máy tại nhà qua Tailscale. Không có thay đổi nào cho người dùng ở giai đoạn hiện tại — việc chuyển đổi sẽ đi kèm importer đọc đúng layout trên.

### BlobStore (`platform/blobstore`, giao qua `platform/blobhttp`) — binary storage độc lập với `STORE_DRIVER`

Âm thanh, ảnh, tệp nhập/xuất đi qua một interface `BlobStore` duy nhất với hai
driver: `local` (tệp nguyên tử, root confine bằng `os.Root`, kháng symlink/`..`)
và `s3` (một driver aws-sdk-go-v2 cho mọi provider S3-compatible). Key object
bất biến, có tiền tố tenant — không sinh từ title/slug/tên file người dùng.
Digest chuẩn là SHA-256 tự tính khi stream; ETag **không bao giờ** được coi là
hash nội dung. Upload >5 MiB đi multipart trong driver; caller không thấy SDK.

Env contract (`STORAGE_DRIVER`, tách với `STORE_DRIVER`):

| Biến | Ý nghĩa |
|---|---|
| `STORAGE_DRIVER` | `local` (mặc định) hoặc `s3` |
| `BLOB_LOCAL_ROOT` | đường dẫn tuyệt đối cho driver local |
| `S3_ENDPOINT` | base endpoint https; trống = AWS resolver; `http://` chỉ với `S3_ALLOW_INSECURE_ENDPOINT=true` (mạng riêng/test, vd Garage) |
| `S3_REGION` | bắt buộc: region bucket; `garage` cho Garage; `auto` cho R2; `us-east-1` cho GCS interop |
| `S3_BUCKET` | bắt buộc |
| `S3_ACCESS_KEY_ID` / `S3_SECRET_ACCESS_KEY` | bắt buộc (GCS: HMAC keys, không phải service-account JSON) |
| `S3_FORCE_PATH_STYLE` | `true\|false` nghiêm ngặt; bắt buộc `true` cho Garage và GCS |

Provider recipes — **một driver, tám cấu hình**. Garage v2.5.0 là S3 đi kèm
và dùng trong integration test; provider ngoài cần smoke trước cutover.

| Provider | `S3_ENDPOINT` | `S3_REGION` | `S3_FORCE_PATH_STYLE` | Ghi chú |
|---|---|---|---|---|
| AWS S3 | (trống) | region bucket | `false` | IAM access/secret, SigV4 |
| Cloudflare R2 | `https://<account>.r2.cloudflarestorage.com` | `auto` | `false` | GET ký bằng endpoint S3; custom domain chỉ là delivery URL |
| GCS (S3 interop) | `https://storage.googleapis.com` | `us-east-1` | `true` | HMAC keys; XML multipart 5MiB–5GiB/part ≤10k parts; không có multipart precondition → khoá ghi đua bằng key bất biến + DB CAS; smoke checksum/presign bắt buộc |
| Garage v2.5.0 | `http://garage:3900` (chỉ mạng riêng/test) | `garage` | `true` | cần `S3_ALLOW_INSECURE_ENDPOINT=true` nếu http; layout một node, SQLite, replication 1 chỉ dành cho local/test |
| MinIO (discontinued) | endpoint cũ | region đã cấu hình | `true` | không còn đi kèm; chuyển sang Garage hoặc provider còn được duy trì |
| Wasabi | endpoint regional do provider cấp | region bucket | `false` | không đoán region từ tên bucket |
| Backblaze B2 | endpoint S3 của bucket | region trong endpoint | `false` | application key có quyền S3 đúng bucket |
| DigitalOcean Spaces | endpoint region (không phải CDN) | region space | `false` | |

Kiểm thử với Postgres + Garage dùng container và volume dùng một lần, chỉ mở
cổng loopback `64632` (PG), `64600` (S3), `64603` (admin). Không dùng credential
hoặc bucket thật:

```sh
export GARAGE_RPC_SECRET=$(openssl rand -hex 32)
export GARAGE_ADMIN_TOKEN=$(openssl rand -hex 32)
export S3_TEST_ACCESS_KEY=GK$(openssl rand -hex 12)
export S3_TEST_SECRET_KEY=$(openssl rand -hex 32)
export S3_TEST_REGION=garage S3_TEST_BUCKET=blobstore-test
export S3_TEST_ENDPOINT=http://127.0.0.1:64600
export S3_TEST_ADMIN_ENDPOINT=http://127.0.0.1:64603
export TEST_DATABASE_URL=postgres://postgres:test@127.0.0.1:64632/ainovel_test
export REQUIRE_POSTGRES_TEST=1 REQUIRE_S3_TEST=1
docker compose -f deploy/compose.test.yml up -d --build --wait
go test -count=1 -timeout=1800s ../../platform/blobstore/... ../../platform/blobhttp/... ./internal/storagegc/... ./internal/audio/... ./internal/persistence/postgres/...
docker compose -f deploy/compose.test.yml down -v
```

Init chờ RPC, áp layout một node nếu chưa có, nhập key từ env và cấp quyền
read/write/owner cho bucket private. Chạy lại không đổi layout đã áp.
Test tạo bucket riêng qua admin API Garage, không cần cấp quyền tạo bucket
cho key S3 ứng dụng; dọn object rồi xoá bucket, không chia sẻ dữ liệu giữa test.

Garage v2.5.0 bỏ qua `If-None-Match: *` khi PUT và hoàn tất multipart. Driver
đo khả năng bằng hai lần ghi vào key staging ngẫu nhiên rồi xoá, không suy ra
hỗ trợ từ HTTP 200; chỉ 412 chứng minh điều kiện được kiểm tra. Provider không
hỗ trợ dùng HEAD trước ghi; multipart luôn HEAD. Key theo nội dung đã được
verify SHA-256 trước upload nên ghi đua chỉ có thể ghi cùng bytes; key mutable
được khoá trong process. Copy luôn đọc lại để so SHA-256 và size.
Snapshot `capabilities.conditionalPut` công bố `unknown` / `supported` / `unsupported`
(local: `not_applicable`). Với Garage, CORS `AllowedHeader` nên dùng chữ thường
(`range`, `if-none-match`, `if-range`) cho preflight trình duyệt; Garage phân biệt
hoa/thường ở bước này. Chế độ truyền qua app tránh phụ thuộc CORS của bucket.

Bucket luôn private, không có đường URL public. Trình duyệt đọc media theo hai
cách tuỳ driver: với S3, app kiểm tra quyền rồi trả **307** sang presigned GET
ngắn hạn (5 phút, trần 15 phút; audio `inline`, tải về `attachment`), byte đi
thẳng từ bucket. Vì vậy bucket cần CORS cho đúng origin của app (xem
`deploy/README.md` › R2). Với local, app tự stream có kiểm tra quyền. Thu hồi
quyền có cửa sổ tối đa bằng TTL của link.

Delivery seam: `internal/entry/web/blob.go` cung cấp
`GET|HEAD /api/blobs/stream/{key}` (Range đơn kể cả suffix, 206/416,
`If-None-Match`/`If-Range`, `Cache-Control: private`, CSP `sandbox`) và
`GET /api/blobs/sign/{key}` (`expiresAt` đúng TTL thực, `no-store`); quyền theo
bảng `blobs` + thành viên dự án.

Copy đối soát giữa hai store (local↔s3, s3↔s3):

```sh
ainovel-cli storage copy \
  --from-config /secure/path/from.json \   # JSON mode 0600, ngoài repo
  --to-config   /secure/path/to.json \
  --manifest    /secure/path/copy.jsonl \
  --resume --concurrency 4
```

File config chứa credentials nên bắt buộc mode `0600` — không có secret trên
argv. Manifest JSONL làm ledger per-key bất biến: `--resume` bỏ qua key đã
`copied`/`matched`; chạy lại không `--resume` trên manifest cũ sẽ báo lỗi.
Mọi object được tải lại từ đích và so SHA-256+size: hash giống → `matched`
(skip), khác → `conflict` (không bao giờ ghi đè), ghi xong đọc lại sai →
`mismatch`; bất kỳ conflict/mismatch/error nào → exit non-zero, không đổi
live pointer. Nguồn không bao giờ bị sửa/xoá.

### Ưu tiên cấu hình (hợp đồng duy nhất)

- **Hạ tầng runtime** (`STORE_DRIVER`, `DATABASE_URL`, `MIGRATION_DATABASE_URL`,
  `DB_MAX_CONNS`, `DB_CONNECT_TIMEOUT`, `STORAGE_DRIVER`, `S3_*`, `HTTP_ADDR`,
  `PUBLIC_URL`, `TRUSTED_PROXIES`, `SECURE_COOKIES`) đọc từ env đã kiểm tra
  nghiêm ngặt lúc khởi động. Giá trị sai là lỗi khởi động, không đoán, không
  kẹp im lặng: `DB_MAX_CONNS` là số nguyên 1..100 đọc trọn chuỗi,
  `DB_CONNECT_TIMEOUT` trong 1s..2m.
- **Cờ CLI rõ ràng thắng env tương ứng của chính nó**: `serve --addr` >
  `HTTP_ADDR` > `127.0.0.1:8686`; `worker tts --api` > `TTS_WORKER_API_URL`.
- **Lệnh bảo trì** (`storage migrate|import|reconcile|export-files`) dùng
  `MIGRATION_DATABASE_URL` trước, rồi `DATABASE_URL` — ưu tiên này chỉ áp cho
  lệnh bảo trì, không phải fallback của tiến trình phục vụ.
- **`storage copy`** chỉ đọc hai file config 0600 tự chứa; nó không trộn env
  blob xung quanh.
- **Kế thừa cấu hình sáng tác** (dự án → truyện → `config.json`/rules) là một
  hệ riêng, không thuộc hợp đồng env ở trên.
- `TRUSTED_PROXIES` chỉ có **một** tên: không còn alias; env lạ không làm thay
  đổi ranh giới tin cậy.

---

## 10. Tùy Biến Văn Phong & Quy Tắc Cá Nhân

### Thêm quy tắc riêng (Không cần sửa code)
Tạo file `.md` bất kỳ trong thư mục `~/.ainovel/rules/` hoặc `./.ainovel/rules/` và viết bằng ngôn ngữ tự nhiên:
- *"Nhân vật chính quyết đoán, không thánh mẫu"*
- *"Tăng cường miêu tả cảm giác cơ thể và khung cảnh xung quanh"*
- *"Mỗi chương khoảng 3000 từ"*
- *"Không dùng các từ sáo rỗng như: 'ở một mức độ nào đó', 'như thể', 'bất giác'"*

Hệ thống sẽ tự động tổng hợp các yêu cầu này vào bộ quy tắc kiểm duyệt của tác phẩm.

---

## 11. Tech Stack & License

- **Ngôn ngữ**: [Go](https://go.dev/) (Hiệu năng cao, kiểm soát chặt chẽ concurrency và I/O)
- **Agent Core**: [core/agentcore](../../core/agentcore) (Tool-calling + Streaming)
- **LLM Interface**: [core/llm](../../core/llm)
- **TUI Framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) & [Lip Gloss](https://github.com/charmbracelet/lipgloss)

### License
Phân phối theo giấy phép **Apache License 2.0** (xem `LICENSE`). Noverse phát triển độc lập; giấy phép, ghi nhận nguồn gốc và các thay đổi được lưu tại `third_party/NOTICE`.
Bản Việt hóa và phát triển bởi cộng đồng. Chúc bạn có những tác phẩm tuyệt vời!
