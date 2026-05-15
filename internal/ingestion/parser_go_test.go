package ingestion

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoParser_StandardFile(t *testing.T) {
	p := NewGoParser()
	f, err := p.ParseFile("testdata/standard.go")
	require.NoError(t, err)

	assert.Equal(t, "main", f.Package)
	assert.False(t, f.HasErrors)

	// imports
	require.Len(t, f.Imports, 2)
	paths := []string{f.Imports[0].Path, f.Imports[1].Path}
	assert.Contains(t, paths, "fmt")
	assert.Contains(t, paths, "strings")

	// top-level functions
	require.Len(t, f.Functions, 2)
	fnNames := []string{f.Functions[0].Name, f.Functions[1].Name}
	assert.Contains(t, fnNames, "NewUser")
	assert.Contains(t, fnNames, "greet")

	// methods
	require.Len(t, f.Methods, 2)
	mNames := []string{f.Methods[0].Name, f.Methods[1].Name}
	assert.Contains(t, mNames, "String")
	assert.Contains(t, mNames, "Validate")
	for _, m := range f.Methods {
		assert.NotEmpty(t, m.Receiver, "method %s should have a receiver", m.Name)
	}

	// struct
	require.Len(t, f.Structs, 1)
	assert.Equal(t, "User", f.Structs[0].Name)
	assert.Len(t, f.Structs[0].Fields, 3)

	// interface
	require.Len(t, f.Interfaces, 1)
	assert.Equal(t, "Greeter", f.Interfaces[0].Name)
	assert.Len(t, f.Interfaces[0].Methods, 2)

	// line numbers are set
	for _, fn := range f.Functions {
		assert.Greater(t, fn.StartLine, 0)
		assert.GreaterOrEqual(t, fn.EndLine, fn.StartLine)
	}
}

func TestGoParser_BuildTags(t *testing.T) {
	p := NewGoParser()
	f, err := p.ParseFile("testdata/build_tags.go")
	require.NoError(t, err)

	assert.Equal(t, "main", f.Package)
	assert.False(t, f.HasErrors)
	assert.Empty(t, f.Imports)

	require.Len(t, f.Functions, 1)
	assert.Equal(t, "platformSpecific", f.Functions[0].Name)
	assert.Empty(t, f.Methods)
	assert.Empty(t, f.Structs)
	assert.Empty(t, f.Interfaces)
}

func TestGoParser_Generics(t *testing.T) {
	p := NewGoParser()
	f, err := p.ParseFile("testdata/generics.go")
	require.NoError(t, err)

	assert.Equal(t, "main", f.Package)
	assert.False(t, f.HasErrors)

	require.Len(t, f.Structs, 1)
	assert.Equal(t, "Stack", f.Structs[0].Name)

	require.Len(t, f.Methods, 2)
	mNames := []string{f.Methods[0].Name, f.Methods[1].Name}
	assert.Contains(t, mNames, "Push")
	assert.Contains(t, mNames, "Pop")

	require.Len(t, f.Functions, 1)
	assert.Equal(t, "Map", f.Functions[0].Name)
}

func TestGoParser_EmptyFile(t *testing.T) {
	p := NewGoParser()
	f, err := p.ParseFile("testdata/empty.go")
	require.NoError(t, err)

	assert.Equal(t, "main", f.Package)
	assert.False(t, f.HasErrors)
	assert.Empty(t, f.Imports)
	assert.Empty(t, f.Functions)
	assert.Empty(t, f.Methods)
	assert.Empty(t, f.Structs)
	assert.Empty(t, f.Interfaces)
}

func TestGoParser_SyntaxError(t *testing.T) {
	p := NewGoParser()
	f, err := p.ParseFile("testdata/broken.go")

	// parser must not return an error — tree-sitter recovers, we log and continue
	require.NoError(t, err)
	assert.True(t, f.HasErrors, "HasErrors should be true for a file with syntax errors")
	assert.Equal(t, "main", f.Package)
}

func TestGoParser_ParseFile_NotFound(t *testing.T) {
	p := NewGoParser()
	_, err := p.ParseFile("testdata/nonexistent.go")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "go parser: read")
}

func BenchmarkGoParser_LargeFile(b *testing.B) {
	src, err := os.ReadFile("testdata/large.go")
	if err != nil {
		b.Fatal(err)
	}
	p := NewGoParser()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p.ParseBytes("testdata/large.go", src); err != nil {
			b.Fatal(err)
		}
	}

	avg := b.Elapsed() / time.Duration(b.N)
	if avg > 50*time.Millisecond {
		b.Errorf("parse too slow: avg %v, want < 50ms", avg)
	}
}
