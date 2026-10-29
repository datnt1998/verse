package library

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// LoadProjectFile 读取 <project>/project.json；不存在返回 (nil, nil)。
func LoadProjectFile(dir string) (*ProjectFile, error) {
	var f ProjectFile
	found, err := readJSONFile(filepath.Join(dir, ProjectFileName), &f)
	if err != nil {
		return nil, fmt.Errorf("project.json hỏng: %w", err)
	}
	if !found {
		return nil, nil
	}
	return &f, nil
}

// WriteProjectFile 写入 <project>/project.json（0600，原子替换）。
func WriteProjectFile(dir string, f ProjectFile) error {
	return writeJSONFile(filepath.Join(dir, ProjectFileName), f)
}

// LoadStoryFile 读取 <story>/story.json；不存在返回 (nil, nil)。
func LoadStoryFile(dir string) (*StoryFile, error) {
	var f StoryFile
	found, err := readJSONFile(filepath.Join(dir, StoryFileName), &f)
	if err != nil {
		return nil, fmt.Errorf("story.json hỏng: %w", err)
	}
	if !found {
		return nil, nil
	}
	return &f, nil
}

// WriteStoryFile 写入 <story>/story.json。
func WriteStoryFile(dir string, f StoryFile) error {
	return writeJSONFile(filepath.Join(dir, StoryFileName), f)
}

// SaveConfig 写入 <root>/.ainovel/library.json。
func SaveConfig(root string, cfg Config) error {
	cfg.Version = ConfigVersion
	return writeJSONFile(ConfigPath(root), cfg)
}

// readJSONFile 读取 JSON 文件；不存在返回 (false, nil)。
func readJSONFile(path string, v any) (bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return false, err
	}
	return true, nil
}

// writeJSONFile 原子写入 JSON（先写临时文件再 rename）。
func writeJSONFile(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
