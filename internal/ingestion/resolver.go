package ingestion

import "strings"

// symbolEntry holds the resolved metadata for a known symbol.
type symbolEntry struct {
	FQN  string
	File string
	Line int
}

// SymbolTable indexes all known symbols from a set of parsed files.
// It supports three lookup strategies used by the resolution algorithm:
//   - byFQN: exact FQN match (fast path)
//   - byPkgName: "package.Name" or "ClassName.Method" qualified lookup
//   - bySimple: unqualified name → all matching FQNs (method-on-receiver lookup)
type SymbolTable struct {
	byFQN     map[string]*symbolEntry
	byPkgName map[string]string   // "pkg.Name" → FQN (first wins on duplicates)
	bySimple  map[string][]string // "Name" → []FQN
}

// CallResolver resolves raw call sites into directed CallEdge entries.
type CallResolver struct{}

// NewCallResolver creates a CallResolver.
func NewCallResolver() *CallResolver { return &CallResolver{} }

// --- Go symbol table & resolution ---

// BuildGoSymbolTable indexes all functions and methods from the provided Go files.
func BuildGoSymbolTable(files []*GoFile) *SymbolTable {
	st := &SymbolTable{
		byFQN:     make(map[string]*symbolEntry),
		byPkgName: make(map[string]string),
		bySimple:  make(map[string][]string),
	}
	for _, f := range files {
		pkg := f.Package
		add := func(fqn, simpleName, file string, line int) {
			e := &symbolEntry{FQN: fqn, File: file, Line: line}
			st.byFQN[fqn] = e
			pkgKey := pkg + "." + simpleName
			if _, exists := st.byPkgName[pkgKey]; !exists {
				st.byPkgName[pkgKey] = fqn
			}
			st.bySimple[simpleName] = append(st.bySimple[simpleName], fqn)
		}
		for _, fn := range f.Functions {
			add(buildGoFQN(pkg, "", fn.Name), fn.Name, f.Path, fn.StartLine)
		}
		for _, m := range f.Methods {
			add(buildGoFQN(pkg, m.Receiver, m.Name), m.Name, f.Path, m.StartLine)
		}
	}
	return st
}

// ResolveGoEdges resolves raw Go call sites against st and returns directed edges.
func ResolveGoEdges(calls []GoCallSite, st *SymbolTable) []CallEdge {
	edges := make([]CallEdge, 0, len(calls))
	for _, cs := range calls {
		edges = append(edges, CallEdge{
			CallerFQN:    cs.CallerFQN,
			CalleeFQN:    resolveGoExpr(cs.Expr, cs.Package, st),
			CallSiteFile: cs.File,
			CallSiteLine: cs.Line,
		})
	}
	return edges
}

// resolveGoExpr maps a raw call expression string to an FQN or "EXTERNAL::<expr>".
func resolveGoExpr(expr, callerPkg string, st *SymbolTable) string {
	// Exact FQN hit (uncommon in raw source, but fast path).
	if _, ok := st.byFQN[expr]; ok {
		return expr
	}

	if idx := strings.Index(expr, "."); idx >= 0 {
		qualifier := expr[:idx]
		name := expr[idx+1:]

		// Try "qualifier.name" as a package-qualified symbol.
		if fqn, ok := st.byPkgName[qualifier+"."+name]; ok {
			return fqn
		}
		// qualifier is likely a variable/receiver, not a package name.
		// Look up the method name in bySimple; resolve if unambiguous.
		if fqns, ok := st.bySimple[name]; ok {
			if len(fqns) == 1 {
				return fqns[0]
			}
			// Ambiguous — prefer same-package match.
			for _, fqn := range fqns {
				if strings.HasPrefix(fqn, callerPkg+".") {
					return fqn
				}
			}
		}
		return "EXTERNAL::" + expr
	}

	// Unqualified name — look in the caller's own package.
	if fqn, ok := st.byPkgName[callerPkg+"."+expr]; ok {
		return fqn
	}
	return "EXTERNAL::" + expr
}

// ResolveGo builds the symbol table from files, collects all call sites, and
// returns resolved edges. Convenience wrapper around BuildGoSymbolTable + ResolveGoEdges.
func (r *CallResolver) ResolveGo(files []*GoFile) []CallEdge {
	st := BuildGoSymbolTable(files)
	var calls []GoCallSite
	for _, f := range files {
		calls = append(calls, f.Calls...)
	}
	return ResolveGoEdges(calls, st)
}

// --- C# FQN helper (pure string, no CGo dependency) ---

// buildCSharpFQN constructs an Azimuth FQN for a C# method.
func buildCSharpFQN(ns, class, method string) string {
	switch {
	case ns != "" && class != "":
		return ns + "." + class + "." + method
	case class != "":
		return class + "." + method
	default:
		return method
	}
}

// --- C# symbol table & resolution ---

// BuildCSharpSymbolTable indexes all methods from the provided C# files.
func BuildCSharpSymbolTable(files []*CSharpFile) *SymbolTable {
	st := &SymbolTable{
		byFQN:     make(map[string]*symbolEntry),
		byPkgName: make(map[string]string),
		bySimple:  make(map[string][]string),
	}
	for _, f := range files {
		for _, cls := range f.Classes {
			for _, m := range cls.Methods {
				fqn := buildCSharpFQN(cls.Namespace, cls.Name, m.Name)
				e := &symbolEntry{FQN: fqn, File: f.Path, Line: m.StartLine}
				st.byFQN[fqn] = e
				classKey := cls.Name + "." + m.Name
				if _, exists := st.byPkgName[classKey]; !exists {
					st.byPkgName[classKey] = fqn
				}
				st.bySimple[m.Name] = append(st.bySimple[m.Name], fqn)
			}
		}
		for _, iface := range f.Interfaces {
			for _, m := range iface.Methods {
				fqn := buildCSharpFQN(iface.Namespace, iface.Name, m.Name)
				e := &symbolEntry{FQN: fqn, File: f.Path, Line: 0}
				st.byFQN[fqn] = e
				st.bySimple[m.Name] = append(st.bySimple[m.Name], fqn)
			}
		}
	}
	return st
}

// ResolveCSharpEdges resolves raw C# call sites against st and returns directed edges.
func ResolveCSharpEdges(calls []CSharpCallSite, st *SymbolTable) []CallEdge {
	edges := make([]CallEdge, 0, len(calls))
	for _, cs := range calls {
		edges = append(edges, CallEdge{
			CallerFQN:    cs.CallerFQN,
			CalleeFQN:    resolveCSharpExpr(cs, st),
			CallSiteFile: cs.File,
			CallSiteLine: cs.Line,
		})
	}
	return edges
}

func resolveCSharpExpr(cs CSharpCallSite, st *SymbolTable) string {
	expr := cs.Expr

	// "this.Method" → resolve within the caller's own class.
	if strings.HasPrefix(expr, "this.") {
		methodName := expr[5:]
		fqn := buildCSharpFQN(cs.Namespace, cs.ClassName, methodName)
		if _, ok := st.byFQN[fqn]; ok {
			return fqn
		}
		return "EXTERNAL::" + expr
	}

	if idx := strings.Index(expr, "."); idx >= 0 {
		qualifier := expr[:idx]
		name := expr[idx+1:]

		// Try as "ClassName.Method" within the symbol table.
		if fqn, ok := st.byPkgName[qualifier+"."+name]; ok {
			return fqn
		}
		// Qualifier is likely a variable; resolve by method name if unambiguous.
		if fqns, ok := st.bySimple[name]; ok && len(fqns) == 1 {
			return fqns[0]
		}
		return "EXTERNAL::" + expr
	}

	// Unqualified: try same class first, then any single match.
	fqn := buildCSharpFQN(cs.Namespace, cs.ClassName, expr)
	if _, ok := st.byFQN[fqn]; ok {
		return fqn
	}
	if fqns, ok := st.bySimple[expr]; ok && len(fqns) == 1 {
		return fqns[0]
	}
	return "EXTERNAL::" + expr
}

// ResolveCSharp builds the C# symbol table from files and returns resolved edges.
func (r *CallResolver) ResolveCSharp(files []*CSharpFile) []CallEdge {
	st := BuildCSharpSymbolTable(files)
	var calls []CSharpCallSite
	for _, f := range files {
		calls = append(calls, f.Calls...)
	}
	return ResolveCSharpEdges(calls, st)
}
