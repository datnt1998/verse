package host

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/datnt1998/noverse/internal/domain"
)

// SimulationProfileRelPath 是仿写画像相对书目录的位置（store.SimulationStore 同一文件）。
const SimulationProfileRelPath = "meta/simulation_profile.json"

// SimulationView 是 web 端“hồ sơ văn phong”卡片的数据（/simulate 完成卡、GET …/simulate/profile）。
// Words 为现存语料文件的词数合计（domain.WordCount 口径；已删除的语料不计）。
type SimulationView struct {
	SourceDir   string                     `json:"sourceDir"`
	Sources     int                        `json:"sources"`
	Words       int                        `json:"words"`
	ProfilePath string                     `json:"profilePath"`
	UpdatedAt   string                     `json:"updatedAt"`
	Synthesis   domain.SimulationSynthesis `json:"synthesis"`
}

// SimulationProfileView 读当前仿写画像；没有画像返回 nil, nil。
func (h *Host) SimulationProfileView() (*SimulationView, error) {
	profile, err := h.store.Simulation.Load()
	if err != nil || profile == nil {
		return nil, err
	}
	v := &SimulationView{
		SourceDir:   simulationSourceDisplay(profile.Corpus.SourceDir),
		Sources:     len(profile.Corpus.Sources),
		ProfilePath: SimulationProfileRelPath,
		UpdatedAt:   profile.UpdatedAt,
		Synthesis:   profile.Synthesis,
	}
	if v.UpdatedAt == "" {
		v.UpdatedAt = profile.CreatedAt
	}
	dir := profile.Corpus.SourceDir
	if dir != "" && !filepath.IsAbs(dir) {
		dir = filepath.Join(h.store.Dir(), dir)
	}
	for _, src := range profile.Corpus.Sources {
		path := src.RelativePath
		if dir != "" {
			path = filepath.Join(dir, filepath.FromSlash(src.RelativePath))
		}
		if data, err := os.ReadFile(path); err == nil {
			v.Words += domain.WordCount(string(data))
		}
	}
	return v, nil
}

// simulationSourceDisplay hides host directories: paths below the story working
// directory stay story-relative, while any other absolute path shows only its name.
func simulationSourceDisplay(dir string) string {
	if dir == "" || !filepath.IsAbs(filepath.FromSlash(dir)) {
		return dir
	}
	dir = filepath.Clean(filepath.FromSlash(dir))
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, dir); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.Base(dir)
}

// SimulationProfileFile 返回仿写画像文件的绝对路径（下载用）；不存在返回 os.ErrNotExist。
func (h *Host) SimulationProfileFile() (string, error) {
	abs := filepath.Join(h.store.Dir(), filepath.FromSlash(SimulationProfileRelPath))
	if _, err := os.Stat(abs); err != nil {
		return "", err
	}
	return abs, nil
}
