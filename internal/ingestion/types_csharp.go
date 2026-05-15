package ingestion

// CSharpFile holds the structural symbols extracted from a single .cs source file.
type CSharpFile struct {
	Path       string
	Namespace  string
	Usings     []CSharpUsing
	Classes    []CSharpClass
	Interfaces []CSharpInterface
	Calls      []CSharpCallSite // call expressions found in method bodies
	HasErrors  bool
}

// CSharpClass represents a class, record, or struct declaration.
type CSharpClass struct {
	Name           string
	Namespace      string
	AccessModifier string
	BaseClass      string
	Interfaces     []string
	IsPartial      bool
	Methods        []CSharpMethod
	Properties     []CSharpProperty
	Attributes     []CSharpAttribute
}

// CSharpMethod represents a method declaration within a class or struct.
type CSharpMethod struct {
	Name           string
	ReturnType     string
	Parameters     []CSharpParam
	AccessModifier string
	IsOverride     bool
	IsVirtual      bool
	IsStatic       bool
	IsAbstract     bool
	StartLine      int
	EndLine        int
	Attributes     []CSharpAttribute
}

// CSharpInterface represents an interface declaration.
type CSharpInterface struct {
	Name      string
	Namespace string
	Methods   []CSharpInterfaceMethod
}

// CSharpInterfaceMethod is a method signature declared in an interface.
type CSharpInterfaceMethod struct {
	Name       string
	ReturnType string
	Parameters []CSharpParam
}

// CSharpProperty represents a property declaration (with getter/setter presence).
type CSharpProperty struct {
	Name      string
	Type      string
	HasGetter bool
	HasSetter bool
}

// CSharpAttribute is a single attribute applied to a class, method, or property.
type CSharpAttribute struct {
	Name      string   // e.g. "HttpGet", "Route"
	Arguments []string // raw argument text, if any
	Target    string   // "class", "method", or "property"
}

// CSharpParam is one parameter in a method or interface-method signature.
type CSharpParam struct {
	Name string
	Type string
}

// CSharpUsing is a using directive (import).
type CSharpUsing struct {
	Namespace string
	Alias     string // non-empty for "using Alias = Namespace;"
}
