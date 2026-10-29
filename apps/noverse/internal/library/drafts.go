package library

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// DraftsDirName 是项目内存放共创草稿的目录（相对项目根）。
const DraftsDirName = ".ainovel/drafts"

// draftIDPattern 限制草稿 id 为单段安全名（gateway 生成 "YYYYMMDD-HHmmss-xxxxxx"）。
var draftIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// ValidDraftID 校验共创草稿 id。
func ValidDraftID(id string) bool { return draftIDPattern.MatchString(id) }

// DraftsDir 返回项目的草稿根目录。
func DraftsDir(projectDir string) string { return filepath.Join(projectDir, DraftsDirName) }

// DraftDir 返回草稿目录；id 非法返回 ErrInvalidName，项目不存在返回 ErrNotFound。
// 匿名项目（_）没有 .ainovel 层，返回 ErrLegacy。
func (l *Library) DraftDir(project, id string) (string, error) {
	if project == LegacyProjectID {
		return "", ErrLegacy
	}
	if !ValidDraftID(id) {
		return "", fmt.Errorf("%w: %q", ErrInvalidName, id)
	}
	pdir, err := l.projectDirForWrite(project)
	if err != nil {
		return "", err
	}
	return filepath.Join(DraftsDir(pdir), id), nil
}

// CreateDraft 建一个空草稿目录（幂等）；返回其绝对路径。
func (l *Library) CreateDraft(project, id string) (string, error) {
	dir, err := l.DraftDir(project, id)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// RemoveDraft 删除草稿目录；不存在视为成功。
func (l *Library) RemoveDraft(project, id string) error {
	dir, err := l.DraftDir(project, id)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return nil
}

// CommitDraft 把草稿目录变成正式故事：移动到 <dirs.stories>/NN-<slug>，
// 写入 story.json（title=name，status=idea），保留草稿里的引擎输出（output/novel）。
// name 为空时 slug 取 "truyen-moi"（tempName=true）。目标已存在则顺延编号。
func (l *Library) CommitDraft(project, id, name string) (Story, bool, error) {
	if project == LegacyProjectID {
		return Story{}, false, ErrLegacy
	}
	dir, err := l.DraftDir(project, id)
	if err != nil {
		return Story{}, false, err
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return Story{}, false, ErrNotFound
	}
	pdir, err := l.projectDirForWrite(project)
	if err != nil {
		return Story{}, false, err
	}
	parent := filepath.Join(pdir, l.Config.Dirs.Stories)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return Story{}, false, err
	}
	name = strings.TrimSpace(name)
	slug := "truyen-moi"
	if name != "" {
		slug = Slugify(name, l.Config.SlugWords)
	}
	if utf8.RuneCountInString(slug) > 96 {
		slug = strings.TrimRight(string([]rune(slug)[:96]), "-")
	}
	tempName := slug == "truyen-moi"
	n := nextStoryNumber(parent) + 1
	var dirName, target string
	for {
		dirName = fmt.Sprintf("%02d-%s", n, slug)
		target = filepath.Join(parent, dirName)
		if _, err := os.Lstat(target); os.IsNotExist(err) {
			break
		}
		n++
	}
	if err := os.Rename(dir, target); err != nil {
		return Story{}, false, err
	}
	now := time.Now()
	f := StoryFile{Title: name, Status: "idea", Tags: []string{}, CreatedAt: &now}
	if err := WriteStoryFile(target, f); err != nil {
		// 回滚到草稿目录：调用方还能重试 commit，不会留下半成品故事。
		_ = os.Rename(target, dir)
		return Story{}, false, err
	}
	st := Story{Project: project, Story: dirName, ID: project + "/" + dirName, Dir: target, ProjectDir: pdir, Root: l.Root}
	return st, tempName, nil
}

// CleanupDrafts 删除所有项目里超过 maxAge 未修改的草稿目录；返回删除数量。
// 扫描失败的项目被跳过（一个坏项目不能阻止 serve 启动）。
func (l *Library) CleanupDrafts(maxAge time.Duration) (int, error) {
	scan, err := l.Scan()
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for _, p := range scan.Projects {
		if p.Legacy {
			continue
		}
		entries, err := os.ReadDir(DraftsDir(p.Dir))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || !ValidDraftID(e.Name()) {
				continue
			}
			info, err := e.Info()
			if err != nil || info.ModTime().After(cutoff) {
				continue
			}
			if err := os.RemoveAll(filepath.Join(DraftsDir(p.Dir), e.Name())); err == nil {
				removed++
			}
		}
	}
	return removed, nil
}
