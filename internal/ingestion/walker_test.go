package ingestion

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mkTree creates a temporary directory tree for walker tests.
// entries is a list of relative paths; directories are created implicitly.
func mkTree(t *testing.T, entries []string) string {
	t.Helper()
	root := t.TempDir()
	for _, e := range entries {
		full := filepath.Join(root, filepath.FromSlash(e))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(""), 0o644))
	}
	return root
}

func TestWalk_FindsGoAndCsFiles(t *testing.T) {
	root := mkTree(t, []string{
		"main.go",
		"util.go",
		"App.cs",
		"readme.txt",
		"Makefile",
	})
	paths, err := Walk(root, WalkConfig{})
	require.NoError(t, err)
	bases := basenames(paths)
	assert.ElementsMatch(t, []string{"main.go", "util.go", "App.cs"}, bases)
}

func TestWalk_RespectsVendorIgnore(t *testing.T) {
	root := mkTree(t, []string{
		"main.go",
		"vendor/dep/lib.go",
		"vendor/dep/helper.go",
		"internal/core.go",
	})
	paths, err := Walk(root, WalkConfig{IgnorePatterns: []string{"vendor/"}})
	require.NoError(t, err)
	bases := basenames(paths)
	assert.ElementsMatch(t, []string{"main.go", "core.go"}, bases)
}

func TestWalk_RespectsNodeModulesIgnore(t *testing.T) {
	root := mkTree(t, []string{
		"server.go",
		"node_modules/pkg/index.go",
	})
	paths, err := Walk(root, WalkConfig{IgnorePatterns: []string{"node_modules/"}})
	require.NoError(t, err)
	bases := basenames(paths)
	assert.ElementsMatch(t, []string{"server.go"}, bases)
}

func TestWalk_RespectsTestFileGlob(t *testing.T) {
	root := mkTree(t, []string{
		"main.go",
		"main_test.go",
		"util.go",
		"util_test.go",
	})
	paths, err := Walk(root, WalkConfig{IgnorePatterns: []string{"*_test.go"}})
	require.NoError(t, err)
	bases := basenames(paths)
	assert.ElementsMatch(t, []string{"main.go", "util.go"}, bases)
}

func TestWalk_RespectsGeneratedCsGlob(t *testing.T) {
	root := mkTree(t, []string{
		"App.cs",
		"Dto.generated.cs",
		"Model.generated.cs",
	})
	paths, err := Walk(root, WalkConfig{IgnorePatterns: []string{"*.generated.cs"}})
	require.NoError(t, err)
	bases := basenames(paths)
	assert.ElementsMatch(t, []string{"App.cs"}, bases)
}

func TestWalk_MultipleIgnorePatterns(t *testing.T) {
	root := mkTree(t, []string{
		"main.go",
		"main_test.go",
		"vendor/lib.go",
	})
	paths, err := Walk(root, WalkConfig{
		IgnorePatterns: []string{"vendor/", "*_test.go"},
	})
	require.NoError(t, err)
	bases := basenames(paths)
	assert.ElementsMatch(t, []string{"main.go"}, bases)
}

func TestWalk_EmptyDir(t *testing.T) {
	root := t.TempDir()
	paths, err := Walk(root, WalkConfig{})
	require.NoError(t, err)
	assert.Empty(t, paths)
}

func TestWalk_NonExistentRoot(t *testing.T) {
	_, err := Walk("/nonexistent/path/xyz", WalkConfig{})
	require.Error(t, err)
}

func TestWalk_CustomExtensions(t *testing.T) {
	root := mkTree(t, []string{
		"main.go",
		"App.cs",
		"script.py",
		"data.json",
	})
	paths, err := Walk(root, WalkConfig{Extensions: []string{".py", ".json"}})
	require.NoError(t, err)
	bases := basenames(paths)
	assert.ElementsMatch(t, []string{"script.py", "data.json"}, bases)
}

func TestWalk_ResultsAreSorted(t *testing.T) {
	root := mkTree(t, []string{
		"z.go",
		"a.go",
		"m.go",
	})
	paths, err := Walk(root, WalkConfig{})
	require.NoError(t, err)
	for i := 1; i < len(paths); i++ {
		assert.LessOrEqual(t, paths[i-1], paths[i], "results should be sorted")
	}
}

// basenames extracts the base filename from each path.
func basenames(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = filepath.Base(p)
	}
	return out
}
