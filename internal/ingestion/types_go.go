// Package ingestion implements the ETL pipeline for repository indexing.
package ingestion

// GoFile holds all structural symbols extracted from a single .go source file.
type GoFile struct {
	Path       string
	Package    string
	Imports    []GoImport
	Functions  []GoFunction // top-level functions (no receiver)
	Methods    []GoFunction // methods (Receiver is set)
	Structs    []GoStruct
	Interfaces []GoInterface
	Calls      []GoCallSite // call expressions found in function/method bodies
	HasErrors  bool         // true if go/parser detected syntax errors; extraction still proceeds
}

// GoFunction represents a top-level function or a method declaration.
// For methods, Receiver is non-empty.
type GoFunction struct {
	Name        string
	Receiver    string // type name of receiver, e.g. "*User"; empty for functions
	Parameters  []GoParam
	ReturnTypes []string
	StartLine   int
	EndLine     int
}

// GoParam is a single parameter (named or unnamed).
type GoParam struct {
	Name string // empty for unnamed parameters
	Type string
}

// GoStruct holds the name and fields of a struct declaration.
type GoStruct struct {
	Name   string
	Fields []GoField
}

// GoField is a single field inside a struct.
type GoField struct {
	Name string
	Type string
}

// GoInterface holds the name and method signatures of an interface declaration.
type GoInterface struct {
	Name    string
	Methods []GoInterfaceMethod
}

// GoInterfaceMethod is a method signature inside an interface.
type GoInterfaceMethod struct {
	Name        string
	Parameters  []GoParam
	ReturnTypes []string
}

// GoImport is a single import path with its optional alias.
type GoImport struct {
	Path  string // e.g. "fmt", "github.com/foo/bar"
	Alias string // "." dot-import, "_" blank import, or named alias; empty = none
}
