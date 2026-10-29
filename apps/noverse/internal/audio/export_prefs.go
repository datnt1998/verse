package audio

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
)

// ExportPrefs stores form preferences, independently of export-only hidden options.
type ExportPrefs struct {
	Format        string  `json:"format"`
	Intro         bool    `json:"intro"`
	ChapterTitles bool    `json:"chapterTitles"`
	Loudness      bool    `json:"loudness"`
	GapSec        float64 `json:"gapSec"`
	ChapterMarks  bool    `json:"chapterMarks"`
	Subtitles     bool    `json:"subtitles"`
	Pack          string  `json:"pack"`
}

func DefaultExportPrefs() ExportPrefs {
	return ExportPrefs{Format: "mp3", Intro: true, ChapterTitles: true, Loudness: true, GapSec: 2, ChapterMarks: true, Subtitles: true, Pack: "files"}
}

func ReadExportPrefs(l Layout) (ExportPrefs, error) {
	prefs := DefaultExportPrefs()
	data, err := os.ReadFile(filepath.Join(l.AudioDir, "export-prefs.json"))
	if errors.Is(err, os.ErrNotExist) {
		return prefs, nil
	}
	if err != nil {
		return prefs, err
	}
	err = json.Unmarshal(data, &prefs)
	return prefs, err
}

func WriteExportPrefs(l Layout, prefs ExportPrefs) error {
	if prefs.Format != "mp3" && prefs.Format != "m4b" && prefs.Format != "wav" {
		return conflict("Định dạng audio không được hỗ trợ.")
	}
	if prefs.Pack != "files" && prefs.Pack != "zip" {
		return conflict("Cách nhận tệp không hợp lệ.")
	}
	if math.IsNaN(prefs.GapSec) || math.IsInf(prefs.GapSec, 0) || prefs.GapSec < 0 || prefs.GapSec > 10 {
		return conflict("Khoảng lặng phải từ 0 đến 10 giây.")
	}
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(l.AudioDir, "export-prefs.json"), data)
}
