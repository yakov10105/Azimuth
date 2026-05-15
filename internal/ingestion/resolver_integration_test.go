//go:build integration

package ingestion

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveGoEdges_CallgraphFixture(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "testdata", "callgraph")

	p := NewGoParser()
	files := make([]*GoFile, 0, 3)
	for _, name := range []string{"caller.go", "callee.go", "greeter.go"} {
		f, err := p.ParseFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", name, err)
		}
		files = append(files, f)
	}

	r := NewCallResolver()
	edges := r.ResolveGo(files)

	type edge struct{ caller, callee string }
	want := map[edge]bool{
		{"callgraph.Caller", "callgraph.Callee"}:                        true,
		{"callgraph.Caller", "EXTERNAL::fmt.Println"}:                   true,
		{"callgraph.Caller", "callgraph.EnglishGreeter.Greet"}:          true,
		{"callgraph.Callee", "callgraph.helper"}:                        true,
	}

	found := map[edge]bool{}
	for _, e := range edges {
		found[edge{e.CallerFQN, e.CalleeFQN}] = true
	}

	for w := range want {
		if !found[w] {
			t.Errorf("missing edge: %q → %q", w.caller, w.callee)
		}
	}
}
