package ingestion

import (
	"testing"
)

// --- buildGoFQN ---

func TestBuildGoFQN(t *testing.T) {
	cases := []struct {
		pkg, receiver, name string
		want                string
	}{
		{"mypkg", "", "Foo", "mypkg.Foo"},
		{"mypkg", "Bar", "Baz", "mypkg.Bar.Baz"},
		{"mypkg", "*Bar", "Baz", "mypkg.Bar.Baz"},
		{"", "", "Foo", ".Foo"},
	}
	for _, tc := range cases {
		got := buildGoFQN(tc.pkg, tc.receiver, tc.name)
		if got != tc.want {
			t.Errorf("buildGoFQN(%q, %q, %q) = %q; want %q",
				tc.pkg, tc.receiver, tc.name, got, tc.want)
		}
	}
}

// --- helper: make a minimal GoFile ---

func goFileWith(pkg string, funcs []GoFunction, methods []GoFunction, calls []GoCallSite) *GoFile {
	return &GoFile{
		Path:      "fake.go",
		Package:   pkg,
		Functions: funcs,
		Methods:   methods,
		Calls:     calls,
	}
}

// --- BuildGoSymbolTable + resolveGoExpr ---

func TestResolveGoEdges_SameFileCall(t *testing.T) {
	f := goFileWith("mypkg",
		[]GoFunction{{Name: "Foo", StartLine: 1}, {Name: "Bar", StartLine: 5}},
		nil,
		[]GoCallSite{{CallerFQN: "mypkg.Foo", Package: "mypkg", Expr: "Bar", File: "fake.go", Line: 2}},
	)
	st := BuildGoSymbolTable([]*GoFile{f})
	edges := ResolveGoEdges(f.Calls, st)
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].CalleeFQN != "mypkg.Bar" {
		t.Errorf("CalleeFQN = %q; want %q", edges[0].CalleeFQN, "mypkg.Bar")
	}
}

func TestResolveGoEdges_CrossFileSamePackage(t *testing.T) {
	f1 := goFileWith("mypkg",
		[]GoFunction{{Name: "Caller", StartLine: 1}},
		nil,
		[]GoCallSite{{CallerFQN: "mypkg.Caller", Package: "mypkg", Expr: "Callee", File: "a.go", Line: 2}},
	)
	f2 := goFileWith("mypkg",
		[]GoFunction{{Name: "Callee", StartLine: 1}},
		nil,
		nil,
	)
	st := BuildGoSymbolTable([]*GoFile{f1, f2})
	edges := ResolveGoEdges(f1.Calls, st)
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].CalleeFQN != "mypkg.Callee" {
		t.Errorf("CalleeFQN = %q; want %q", edges[0].CalleeFQN, "mypkg.Callee")
	}
}

func TestResolveGoEdges_CrossPackageCall(t *testing.T) {
	f := goFileWith("mypkg",
		[]GoFunction{{Name: "Foo", StartLine: 1}},
		nil,
		[]GoCallSite{{CallerFQN: "mypkg.Foo", Package: "mypkg", Expr: "fmt.Println", File: "fake.go", Line: 2}},
	)
	st := BuildGoSymbolTable([]*GoFile{f})
	edges := ResolveGoEdges(f.Calls, st)
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].CalleeFQN != "EXTERNAL::fmt.Println" {
		t.Errorf("CalleeFQN = %q; want %q", edges[0].CalleeFQN, "EXTERNAL::fmt.Println")
	}
}

func TestResolveGoEdges_MethodOnReceiverType(t *testing.T) {
	f := goFileWith("mypkg",
		nil,
		[]GoFunction{{Name: "Greet", Receiver: "EnglishGreeter", StartLine: 10}},
		[]GoCallSite{{CallerFQN: "mypkg.Caller", Package: "mypkg", Expr: "g.Greet", File: "fake.go", Line: 5}},
	)
	st := BuildGoSymbolTable([]*GoFile{f})
	edges := ResolveGoEdges(f.Calls, st)
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	// "g.Greet": qualifier "g" not in byPkgName; bySimple["Greet"] has exactly one entry → resolved
	if edges[0].CalleeFQN != "mypkg.EnglishGreeter.Greet" {
		t.Errorf("CalleeFQN = %q; want %q", edges[0].CalleeFQN, "mypkg.EnglishGreeter.Greet")
	}
}

func TestResolveGoEdges_UnresolvableCall(t *testing.T) {
	f := goFileWith("mypkg",
		[]GoFunction{{Name: "Foo", StartLine: 1}},
		nil,
		[]GoCallSite{{CallerFQN: "mypkg.Foo", Package: "mypkg", Expr: "unknownPkg.DoThing", File: "fake.go", Line: 3}},
	)
	st := BuildGoSymbolTable([]*GoFile{f})
	edges := ResolveGoEdges(f.Calls, st)
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].CalleeFQN != "EXTERNAL::unknownPkg.DoThing" {
		t.Errorf("CalleeFQN = %q; want %q", edges[0].CalleeFQN, "EXTERNAL::unknownPkg.DoThing")
	}
}

func TestResolveGoEdges_AmbiguousMethodPrefersCallerPackage(t *testing.T) {
	// Two packages both define "Process"; caller is in "pkgA"
	f1 := goFileWith("pkgA",
		[]GoFunction{{Name: "Process", StartLine: 1}},
		nil,
		[]GoCallSite{{CallerFQN: "pkgA.Entry", Package: "pkgA", Expr: "x.Process", File: "a.go", Line: 5}},
	)
	f2 := goFileWith("pkgB",
		[]GoFunction{{Name: "Process", StartLine: 1}},
		nil,
		nil,
	)
	st := BuildGoSymbolTable([]*GoFile{f1, f2})
	edges := ResolveGoEdges(f1.Calls, st)
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].CalleeFQN != "pkgA.Process" {
		t.Errorf("CalleeFQN = %q; want %q", edges[0].CalleeFQN, "pkgA.Process")
	}
}
