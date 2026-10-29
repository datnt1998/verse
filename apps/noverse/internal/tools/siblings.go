package tools

import (
	"github.com/datnt1998/noverse/internal/library"
	"github.com/datnt1998/noverse/internal/store"
)

// siblingStories lists the other stories of the same project for comparison.
// It is a file-library capability: the file layout is what defines "same
// project directory". Hosted stores have no cross-story directory to scan, so
// they answer an empty list — callers treat that as "nothing to compare",
// never as permission to read the host filesystem.
func siblingStories(s *store.Store) ([]library.StoryDir, error) {
	dir, ok := s.FileRoot()
	if !ok {
		return nil, nil
	}
	return library.SiblingStories(dir)
}

// fileLibrary reports whether the store is file-backed and sits inside a
// project directory (the only mode with project-level comparison sources).
func fileLibrary(s *store.Store) bool {
	dir, ok := s.FileRoot()
	if !ok {
		return false
	}
	return library.FindProjectDir(dir) != ""
}
