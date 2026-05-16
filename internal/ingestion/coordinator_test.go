package ingestion

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const goSrc = `package mypkg

func Hello() string { return Greet() }
func Greet() string { return "hi" }
`

const goSrc2 = `package mypkg

type Server struct{ addr string }
func (s *Server) Start() {}
`

const brokenGoSrc = `package mypkg

func Bad( {`

func TestCoordinator_Run_GoOnly(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.go"), goSrc)
	writeFile(t, filepath.Join(root, "b.go"), goSrc2)

	c := NewCoordinator(DefaultCoordinatorConfig())
	report, err := c.Run(context.Background(), root)
	require.NoError(t, err)

	assert.Equal(t, 2, report.FilesProcessed)
	assert.Equal(t, 2, report.GoFilesFound)
	assert.Equal(t, 0, report.CSharpFilesFound)
	assert.Greater(t, report.SymbolsExtracted, 0)
	assert.Empty(t, report.Errors)
}

func TestCoordinator_Run_MixedFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "main.go"), goSrc)
	writeFile(t, filepath.Join(root, "README.md"), "# readme")
	writeFile(t, filepath.Join(root, "data.txt"), "some text")

	c := NewCoordinator(DefaultCoordinatorConfig())
	report, err := c.Run(context.Background(), root)
	require.NoError(t, err)

	// only the .go file should be processed
	assert.Equal(t, 1, report.FilesProcessed)
	assert.Equal(t, 1, report.GoFilesFound)
}

func TestCoordinator_Run_EmptyDir(t *testing.T) {
	root := t.TempDir()

	c := NewCoordinator(DefaultCoordinatorConfig())
	report, err := c.Run(context.Background(), root)
	require.NoError(t, err)

	assert.Equal(t, 0, report.FilesProcessed)
	assert.Equal(t, 0, report.SymbolsExtracted)
	assert.Equal(t, 0, report.EdgesFound)
	assert.Empty(t, report.Errors)
}

func TestCoordinator_Run_BadFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "good.go"), goSrc)
	writeFile(t, filepath.Join(root, "bad.go"), brokenGoSrc)

	c := NewCoordinator(DefaultCoordinatorConfig())
	// Run must not return an error — bad files are collected, not fatal.
	report, err := c.Run(context.Background(), root)
	require.NoError(t, err)

	// Both files are processed (go/parser recovers partial ASTs on syntax errors).
	// bad.go's syntax error is surfaced in report.Errors.
	assert.Equal(t, 2, report.FilesProcessed)
	assert.Len(t, report.Errors, 1)
}

func TestCoordinator_Run_ContextCancelled(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "main.go"), goSrc)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	c := NewCoordinator(DefaultCoordinatorConfig())
	_, err := c.Run(ctx, root)
	require.Error(t, err)
}

func TestCoordinator_Run_IgnorePatterns(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "main.go"), goSrc)
	vendorDir := filepath.Join(root, "vendor")
	require.NoError(t, os.MkdirAll(vendorDir, 0o755))
	writeFile(t, filepath.Join(vendorDir, "lib.go"), goSrc)

	cfg := DefaultCoordinatorConfig()
	cfg.WalkConfig.IgnorePatterns = []string{"vendor/"}

	c := NewCoordinator(cfg)
	report, err := c.Run(context.Background(), root)
	require.NoError(t, err)

	assert.Equal(t, 1, report.FilesProcessed)
}

func TestCoordinator_Run_Concurrent(t *testing.T) {
	root := t.TempDir()
	// Write 50 distinct .go files, each with a unique function name.
	for i := 0; i < 50; i++ {
		src := fmt.Sprintf("package bench\nfunc Fn%d() {}\n", i)
		writeFile(t, filepath.Join(root, fmt.Sprintf("file%d.go", i)), src)
	}

	cfg := DefaultCoordinatorConfig()
	cfg.Workers = 8
	c := NewCoordinator(cfg)
	report, err := c.Run(context.Background(), root)
	require.NoError(t, err)

	assert.Equal(t, 50, report.FilesProcessed)
	assert.Greater(t, report.SymbolsExtracted, 0)
	assert.Empty(t, report.Errors)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
