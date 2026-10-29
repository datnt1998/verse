// Package audio 实现"truyện → bản đọc → tạo giọng → ghép & xuất"。
//
// 设计契约见 docs/design/v1/10-audio.md（§3 数据/目录、§4 GAP-17…20）与
// docs/design/v1/15-audio-impl.md（§2）。三条铁律：
//
//  1. 不依赖 worker：所有文件都在故事目录里，网关进程直接读写，TUI 持锁时照常可用。
//  2. 只生成一次就留着：每段音频按内容哈希缓存（cache/tap-NN/CC/<hash>.wav），
//     改一个字只重生成那一段；"nghe thử" 也是缓存。
//  3. 确定性优先：文本规范化、说话人归属的兜底、SRT 时间轴全部用 Go 算；
//     只有"把章节拆成朗读段 + 判断谁在说话"交给故事配置的模型。
package audio

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
)

// 目录约定（10-audio §3）。audio 子目录名来自书库配置 dirs.audio（默认 "audio"）。
const (
	scriptsSubdir   = "ban-doc"
	cacheSubdir     = "cache"
	chaptersSubdir  = "chuong"
	queueFile       = "queue.json"
	settingsFile    = "settings.json"
	castFile        = "vai.json"
	flagsFile       = "flags.json"
	candidatesDir   = "ung-vien"
	voicesSubdir    = "giong"
	voicesIndexFile = "giong.json"
	musicSubdir     = "nhac"
	coverSubdir     = "bia"
	pronounceFile   = "cach-doc.json"
	defaultAudioDir = "audio"
)

// Layout 是某个故事的音频工作目录集合。所有路径都是绝对路径。
type Layout struct {
	StoryDir      string // <story>
	AudioDir      string // <story>/<dirs.audio>
	AssetsDir     string // <project>/<dirs.assets>
	ExportsDir    string // <story>/<dirs.exports>
	PronouncePath string // <project>/.ainovel/cach-doc.json（项目级读音词典）

	Pack *PackDefaults // mặc định audio của gói thể loại; nil nếu không dùng gói
}

// ResolveLayout 读取书库配置，解析故事的音频相关目录。只做路径计算，不建目录。
func ResolveLayout(st library.Story) (Layout, error) {
	cfg, err := library.LoadConfig(st.Root)
	if err != nil {
		return Layout{}, err
	}
	audioDir := cfg.Dirs.Audio
	if audioDir == "" {
		audioDir = defaultAudioDir
	}
	assets := cfg.Dirs.Assets
	if assets == "" {
		assets = "tai-san"
	}
	exports := cfg.Dirs.Exports
	if exports == "" {
		exports = "xuat-ban"
	}
	projectDir := st.ProjectDir
	if projectDir == "" {
		projectDir = st.Dir
	}
	return Layout{
		StoryDir:      st.Dir,
		AudioDir:      filepath.Join(st.Dir, audioDir),
		AssetsDir:     filepath.Join(projectDir, assets),
		ExportsDir:    filepath.Join(st.Dir, exports),
		PronouncePath: filepath.Join(projectDir, ".ainovel", pronounceFile),
	}, nil
}

// ScriptPath 返回某卷某章 bản đọc 的落盘路径。
func (l Layout) ScriptPath(volume, chapter int) string {
	return filepath.Join(l.AudioDir, scriptsSubdir, fmt.Sprintf("tap-%02d", volume), fmt.Sprintf("%02d.json", chapter))
}

// CachePath 返回一段音频的缓存路径（按内容哈希，永不覆盖别人）。
func (l Layout) CachePath(volume, chapter int, key string) string {
	return filepath.Join(l.AudioDir, cacheSubdir, fmt.Sprintf("tap-%02d", volume), fmt.Sprintf("%02d", chapter), key+".wav")
}

// ChapterAudioPath 返回整章播放文件的路径。
func (l Layout) ChapterAudioPath(volume, chapter int, format string) string {
	return filepath.Join(l.AudioDir, chaptersSubdir, fmt.Sprintf("tap-%02d", volume), fmt.Sprintf("%02d.%s", chapter, format))
}

// VolumeExportDir 返回 <exports>/tap-NN。
func (l Layout) VolumeExportDir(volume int) string {
	return filepath.Join(l.ExportsDir, fmt.Sprintf("tap-%02d", volume))
}

// MusicDir 返回项目音乐目录 <assets>/nhac。
func (l Layout) MusicDir() string { return filepath.Join(l.AssetsDir, musicSubdir) }

// CoverDir 返回项目封面目录 <assets>/bia。
func (l Layout) CoverDir() string { return filepath.Join(l.AssetsDir, coverSubdir) }

// VoicesDir 返回项目声音样本目录 <assets>/giong。
func (l Layout) VoicesDir() string { return filepath.Join(l.AssetsDir, voicesSubdir) }

func (l Layout) queuePath() string    { return filepath.Join(l.AudioDir, queueFile) }
func (l Layout) settingsPath() string { return filepath.Join(l.AudioDir, settingsFile) }
func (l Layout) castPath() string     { return filepath.Join(l.AudioDir, castFile) }

// CastPath 是 vai.json 路径（网关算 version 用）。
func (l Layout) CastPath() string { return l.castPath() }

// SettingsPath 是 settings.json 路径（网关算 version 用）。
func (l Layout) SettingsPath() string { return l.settingsPath() }

func (l Layout) flagsPath() string      { return filepath.Join(l.AudioDir, flagsFile) }
func (l Layout) candidatesPath() string { return filepath.Join(l.AudioDir, candidatesDir) }

// ensureAudioDirs 建出 audio 子目录（幂等）。
func (l Layout) ensureAudioDirs() error {
	for _, dir := range []string{
		l.AudioDir,
		filepath.Join(l.AudioDir, scriptsSubdir),
		filepath.Join(l.AudioDir, cacheSubdir),
		filepath.Join(l.AudioDir, chaptersSubdir),
		l.candidatesPath(),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}
