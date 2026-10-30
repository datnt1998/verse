// Tách tiếng và tạo phụ đề: venv Python 3.11 riêng của app với faster-whisper
// ghim hash (CPU) + các mô hình Systran trong cache HF của app. Port của
// src/server/tools/transcription.ts — uv và phần dựng env dùng lại Uv.
package tools

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// transcriptRequirements là khoá requirements của env tách tiếng (bản
// copy của scripts/clone-transcript-requirements.txt trong yt-factory,
// khóa hash cho aarch64-apple-darwin).
//
//go:embed transcript-requirements.txt
var transcriptRequirements []byte

const transcriptionToolID = "transcription"

// transcriptionVersion là phiên bản hiển thị của bộ cài tách tiếng.
const transcriptionVersion = "faster-whisper-1.2.1"

// Tên giai đoạn của lifecycle — giữ nguyên id của TRANSCRIPTION_STAGES để
// nhãn pha phía web khớp copy của bản cũ.
const (
	transcriptionStageUv        = "uv"
	transcriptionStageEnv       = "python 3.11 + venv + pinned wheels"
	transcriptionStageModels    = "faster-whisper models"
	transcriptionStageSelfCheck = "silero VAD load"
)

// whisperModelSizes là các mô hình faster-whisper ghim: small lấp lời nói
// không phụ đề, base neo thời điểm từng từ.
var whisperModelSizes = []string{"small", "base"}

// TranscriptionState là trạng thái của môi trường tách tiếng.
type TranscriptionState string

const (
	TranscriptionReady   TranscriptionState = "ready"
	TranscriptionMissing TranscriptionState = "missing"
	TranscriptionBroken  TranscriptionState = "broken"
)

// TranscriptionStatus tóm tắt trạng thái môi trường cho Cài đặt và setup.
type TranscriptionStatus struct {
	State         TranscriptionState
	EnvDir        string
	Python        string
	FasterWhisper string
	UvReady       bool
	Error         string
}

// Transcription quản lý venv tách tiếng của app.
type Transcription struct {
	d         Deps
	uv        *Uv
	installer *Installer
}

// NewTranscription dựng bộ quản lý; installer nil → Installer trên d.BaseDir.
func NewTranscription(d Deps, uv *Uv, installer *Installer) *Transcription {
	if installer == nil {
		installer = &Installer{Client: d.Client, BaseDir: d.BaseDir}
	}
	return &Transcription{d: d, uv: uv, installer: installer}
}

// EnvDir là thư mục venv tách tiếng (<base>/envs/transcription).
func (t *Transcription) EnvDir() string {
	return filepath.Join(t.d.BaseDir, "envs", transcriptionToolID)
}

// requirementsFile là bản copy của khoá requirements trong thư mục dữ liệu —
// uv pip sync và fingerprint đọc từ ổ, nội dung ghim trong binary.
func (t *Transcription) requirementsFile() string {
	return filepath.Join(t.d.BaseDir, "python", "transcript-requirements.txt")
}

// ensureRequirementsFile ghi khoá requirements ra ổ khi chưa có hoặc lệch
// nội dung (ghi tạm + rename nguyên tử).
func (t *Transcription) ensureRequirementsFile() (string, error) {
	target := t.requirementsFile()
	if current, err := os.ReadFile(target); err == nil && string(current) == string(transcriptRequirements) {
		return target, nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	tmp := fmt.Sprintf("%s.%d.tmp", target, os.Getpid())
	if err := os.WriteFile(tmp, transcriptRequirements, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return target, nil
}

// ModelDir là cache Hugging Face của app (<base>/models/hf) — mọi tải mô hình
// nhận giọng nói nằm trong thư mục dữ liệu, không chạm ~/.cache của người dùng.
func (t *Transcription) ModelDir() string {
	return filepath.Join(t.d.BaseDir, "models", "hf")
}

// RunEnv là môi trường chạy của venv: UV_* trỏ thư mục app, HF_HOME vào cache
// của app, tắt song song hoá tokenizer và giới hạn luồng torch trên CPU.
func (t *Transcription) RunEnv() map[string]string {
	env := t.uv.Env()
	env["HF_HOME"] = t.ModelDir()
	env["XDG_CACHE_HOME"] = filepath.Join(t.d.BaseDir, "cache")
	env["OMP_NUM_THREADS"] = "4"
	env["TOKENIZERS_PARALLELISM"] = "false"
	return env
}

// ModelsPresent đúng khi mọi mô hình ghim đã nằm trong cache HF
// (layout download_model: models--Systran--faster-whisper-<size>).
func (t *Transcription) ModelsPresent() bool {
	for _, size := range whisperModelSizes {
		snapshots := filepath.Join(t.ModelDir(),
			"models--Systran--faster-whisper-"+size, "snapshots")
		entries, err := os.ReadDir(snapshots)
		if err != nil {
			return false
		}
		ok := false
		for _, rev := range entries {
			if fileExists(filepath.Join(snapshots, rev.Name(), "model.bin")) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

// downloadModelsScript tải các mô hình ghim vào cache HF của app.
const downloadModelsScript = `import os
from faster_whisper.utils import download_model
for size in ["small", "base"]:
    download_model(size, cache_dir=os.environ['HF_HOME'])
`

// selfCheckScript chứng minh env chạy được: import thật + forward pass Silero
// VAD trên CPU, in phiên bản faster-whisper ở dòng cuối.
const selfCheckScript = `import importlib.metadata as md
import faster_whisper, ctranslate2, onnxruntime
from silero_vad import load_silero_vad
import torch
m = load_silero_vad()
m(torch.zeros(1, 512), 16000)
print('transcription-ok', md.version('faster-whisper'))
`

// Status tổng hợp trạng thái môi trường: env còn khớp fingerprint và đủ mô
// hình thì probe phiên bản faster-whisper thật để kết luận ready/broken.
func (t *Transcription) Status(ctx context.Context) TranscriptionStatus {
	requirements := t.requirementsFile()
	info, err := t.uv.EnvInfo(t.EnvDir(), requirements)
	if err != nil {
		return TranscriptionStatus{State: TranscriptionBroken, EnvDir: t.EnvDir(), Error: err.Error()}
	}
	built := t.uv.EnvBuiltFingerprint(info)
	if built != info.Fingerprint || !fileExists(info.Python) || !t.ModelsPresent() {
		return TranscriptionStatus{State: TranscriptionMissing, UvReady: t.uv.Resolve(ctx) != ""}
	}
	res, err := t.d.Runner.Run(ctx, info.Python,
		[]string{"-c", "import importlib.metadata as m; print(m.version('faster-whisper'))"},
		Spec{Env: t.RunEnv(), Timeout: time.Minute})
	if err != nil {
		return TranscriptionStatus{State: TranscriptionBroken, EnvDir: t.EnvDir(), Python: info.Python, Error: err.Error()}
	}
	version := strings.TrimSpace(res.Stdout)
	if i := strings.LastIndex(version, "\n"); i >= 0 {
		version = version[i+1:]
	}
	return TranscriptionStatus{
		State: TranscriptionReady, EnvDir: info.EnvDir,
		Python: info.Python, FasterWhisper: version,
	}
}

// Install là thân lifecycle của công cụ tách tiếng, đăng ký qua
// Managed.RegisterLifecycle: uv (tự cài nếu thiếu) → venv ghim → tải mô hình
// → tự kiểm. Cài uv lồng trong lượt của transcription qua spec trực tiếp,
// không qua lượt riêng của sổ công cụ.
func (t *Transcription) Install(ctx context.Context, lc *LifecycleContext) error {
	lc.State(PhaseChecking, transcriptionStageUv, "")
	uvBin := t.uv.Resolve(ctx)
	if uvBin == "" {
		lc.State(PhaseInstalling, transcriptionStageUv, "")
		spec := t.uv.InstallSpec()
		result, err := t.installer.Install(ctx, spec)
		if err != nil {
			return err
		}
		uvBin = filepath.Join(result.Path, "uv")
	}
	if err := lc.Check(); err != nil {
		return err
	}

	lc.State(PhaseInstalling, transcriptionStageEnv, "")
	requirements, err := t.ensureRequirementsFile()
	if err != nil {
		return err
	}
	info, err := t.uv.EnsurePythonEnv(ctx, uvBin, t.EnvDir(), requirements,
		func(detail string) {
			lc.State(PhaseInstalling, transcriptionStageEnv+": "+detail, "")
		})
	if err != nil {
		return err
	}
	if err := lc.Check(); err != nil {
		return err
	}

	lc.State(PhaseDownloading, transcriptionStageModels, "")
	if _, err := t.d.Runner.Run(ctx, info.Python, []string{"-c", downloadModelsScript},
		Spec{Env: t.RunEnv(), Timeout: time.Hour}); err != nil {
		return err
	}
	if err := lc.Check(); err != nil {
		return err
	}

	lc.State(PhaseSelfCheck, transcriptionStageSelfCheck, "")
	if _, err := t.d.Runner.Run(ctx, info.Python, []string{"-c", selfCheckScript},
		Spec{Env: t.RunEnv(), Timeout: 10 * time.Minute}); err != nil {
		return err
	}
	return nil
}
