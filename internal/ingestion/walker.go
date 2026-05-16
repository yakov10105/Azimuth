package ingestion

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// WalkConfig controls which files the walker includes or skips.
type WalkConfig struct {
	// Extensions is the set of file extensions to include (e.g. ".go", ".cs").
	// Defaults to [".go", ".cs"] when empty.
	Extensions []string

	// IgnorePatterns is a list of glob patterns matched against individual path
	// segments. A pattern ending in "/" is matched against directory names only
	// (the trailing slash is stripped before matching). All other patterns are
	// matched against file basenames. Examples: "vendor/", "node_modules/",
	// "*_test.go", "*.generated.cs".
	IgnorePatterns []string
}

var defaultExtensions = []string{".go", ".cs"}

// Walk recursively discovers files under root that pass the WalkConfig filters.
// It returns a sorted slice of absolute paths and wraps any filesystem error.
func Walk(root string, cfg WalkConfig) ([]string, error) {
	exts := cfg.Extensions
	if len(exts) == 0 {
		exts = defaultExtensions
	}

	extSet := make(map[string]bool, len(exts))
	for _, e := range exts {
		extSet[e] = true
	}

	dirPatterns, filePatterns := splitPatterns(cfg.IgnorePatterns)

	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk %s: %w", path, err)
		}
		if d.IsDir() {
			if matchesAny(d.Name(), dirPatterns) {
				return filepath.SkipDir
			}
			return nil
		}
		if matchesAny(d.Name(), filePatterns) {
			return nil
		}
		if extSet[filepath.Ext(d.Name())] {
			abs, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("walk abs %s: %w", path, err)
			}
			paths = append(paths, abs)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}
	sort.Strings(paths)
	return paths, nil
}

// splitPatterns separates ignore patterns into directory-name patterns (had a
// trailing "/") and filename glob patterns.
func splitPatterns(patterns []string) (dirPats, filePats []string) {
	for _, p := range patterns {
		if strings.HasSuffix(p, "/") {
			dirPats = append(dirPats, strings.TrimSuffix(p, "/"))
		} else {
			filePats = append(filePats, p)
		}
	}
	return
}

// matchesAny reports whether name matches any of the glob patterns.
// An invalid pattern is treated as a non-match.
func matchesAny(name string, patterns []string) bool {
	for _, p := range patterns {
		matched, err := filepath.Match(p, name)
		if err == nil && matched {
			return true
		}
	}
	return false
}
