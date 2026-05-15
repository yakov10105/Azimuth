package ingestion

// GoCallSite records a single call expression found inside a Go function or method body.
// The Expr field is the raw call target as written in source ("Bar", "fmt.Println", "g.Greet").
type GoCallSite struct {
	CallerFQN string // "package.Receiver.Method" of the enclosing function
	Package   string // package of the call site, used for same-package resolution
	Expr      string // raw call target text
	File      string
	Line      int
}

// CSharpCallSite records a single call expression found inside a C# method body.
type CSharpCallSite struct {
	CallerFQN string // "namespace.ClassName.MethodName"
	Namespace string
	ClassName string
	Expr      string // "Method", "ClassName.Method", or "this.Method"
	File      string
	Line      int
}

// CallEdge is a resolved directed edge in the call graph.
// CalleeFQN uses the prefix "EXTERNAL::" for calls that could not be resolved
// to a symbol in the parsed file set (stdlib calls, third-party libs, reflection, etc.).
type CallEdge struct {
	CallerFQN    string
	CalleeFQN    string // "pkg.Name" or "EXTERNAL::<expr>"
	CallSiteFile string
	CallSiteLine int
}
