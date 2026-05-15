package ingestion

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"strings"
)

// GoParser parses .go source files using the standard go/ast package and extracts
// structural symbols. It uses go/parser rather than Tree-sitter because go/parser
// is the authoritative Go frontend (no CGo required, supports all Go syntax including
// generics). Tree-sitter is used for C# files in Task 1.2.2.
type GoParser struct {
	fset *token.FileSet
}

// NewGoParser creates a new GoParser.
func NewGoParser() *GoParser {
	return &GoParser{fset: token.NewFileSet()}
}

// ParseFile reads a .go file from disk and extracts its structural symbols.
// Syntax errors are logged as warnings; extraction continues on the valid AST.
func (p *GoParser) ParseFile(path string) (*GoFile, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("go parser: read %s: %w", path, err)
	}
	return p.ParseBytes(path, src)
}

// ParseBytes parses a .go source from an in-memory byte slice.
// Syntax errors are logged as warnings; extraction continues on partial AST.
func (p *GoParser) ParseBytes(path string, src []byte) (*GoFile, error) {
	fset := token.NewFileSet()
	// parser.AllErrors: collect all errors, not just the first
	// parser.ParseComments: keep comments for future doc extraction
	astFile, err := parser.ParseFile(fset, path, src, parser.AllErrors|parser.ParseComments)

	file := &GoFile{Path: path}

	if err != nil {
		// go/parser returns a non-nil *ast.File even on syntax errors — keep going
		slog.Warn("go parser: syntax errors detected, extracting partial results",
			"path", path, "err", err)
		file.HasErrors = true
	}

	if astFile == nil {
		return nil, fmt.Errorf("go parser: nil AST for %s", path)
	}

	file.Package = astFile.Name.Name
	file.Imports = extractImports(astFile)

	for _, decl := range astFile.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			fn := extractFuncDecl(d, fset)
			if d.Recv != nil && len(d.Recv.List) > 0 {
				file.Methods = append(file.Methods, fn)
			} else {
				file.Functions = append(file.Functions, fn)
			}
		case *ast.GenDecl:
			structs, ifaces := extractGenDecl(d)
			file.Structs = append(file.Structs, structs...)
			file.Interfaces = append(file.Interfaces, ifaces...)
		}
	}

	return file, nil
}

// --- imports ---

func extractImports(f *ast.File) []GoImport {
	var imports []GoImport
	for _, spec := range f.Imports {
		imp := GoImport{
			Path: strings.Trim(spec.Path.Value, `"`),
		}
		if spec.Name != nil {
			imp.Alias = spec.Name.Name
		}
		imports = append(imports, imp)
	}
	return imports
}

// --- functions and methods ---

func extractFuncDecl(d *ast.FuncDecl, fset *token.FileSet) GoFunction {
	fn := GoFunction{
		Name:      d.Name.Name,
		StartLine: fset.Position(d.Pos()).Line,
		EndLine:   fset.Position(d.End()).Line,
	}
	if d.Recv != nil && len(d.Recv.List) > 0 {
		fn.Receiver = receiverType(d.Recv.List[0])
	}
	if d.Type.Params != nil {
		fn.Parameters = extractFieldList(d.Type.Params)
	}
	if d.Type.Results != nil {
		fn.ReturnTypes = extractTypeList(d.Type.Results)
	}
	return fn
}

func receiverType(field *ast.Field) string {
	return typeString(field.Type)
}

// --- type declarations ---

func extractGenDecl(d *ast.GenDecl) ([]GoStruct, []GoInterface) {
	var structs []GoStruct
	var ifaces []GoInterface

	for _, spec := range d.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		name := ts.Name.Name
		switch t := ts.Type.(type) {
		case *ast.StructType:
			structs = append(structs, GoStruct{
				Name:   name,
				Fields: extractStructFields(t),
			})
		case *ast.InterfaceType:
			ifaces = append(ifaces, GoInterface{
				Name:    name,
				Methods: extractInterfaceMethods(t),
			})
		}
	}
	return structs, ifaces
}

func extractStructFields(st *ast.StructType) []GoField {
	if st.Fields == nil {
		return nil
	}
	var fields []GoField
	for _, field := range st.Fields.List {
		typStr := typeString(field.Type)
		if len(field.Names) == 0 {
			// embedded field
			fields = append(fields, GoField{Name: "", Type: typStr})
			continue
		}
		for _, name := range field.Names {
			fields = append(fields, GoField{Name: name.Name, Type: typStr})
		}
	}
	return fields
}

func extractInterfaceMethods(it *ast.InterfaceType) []GoInterfaceMethod {
	if it.Methods == nil {
		return nil
	}
	var methods []GoInterfaceMethod
	for _, method := range it.Methods.List {
		ft, ok := method.Type.(*ast.FuncType)
		if !ok {
			continue // embedded interface — skip for now
		}
		for _, name := range method.Names {
			m := GoInterfaceMethod{Name: name.Name}
			if ft.Params != nil {
				m.Parameters = extractFieldList(ft.Params)
			}
			if ft.Results != nil {
				m.ReturnTypes = extractTypeList(ft.Results)
			}
			methods = append(methods, m)
		}
	}
	return methods
}

// --- parameter helpers ---

func extractFieldList(fl *ast.FieldList) []GoParam {
	if fl == nil {
		return nil
	}
	var params []GoParam
	for _, field := range fl.List {
		typStr := typeString(field.Type)
		if len(field.Names) == 0 {
			params = append(params, GoParam{Type: typStr})
			continue
		}
		for _, name := range field.Names {
			params = append(params, GoParam{Name: name.Name, Type: typStr})
		}
	}
	return params
}

func extractTypeList(fl *ast.FieldList) []string {
	if fl == nil {
		return nil
	}
	var types []string
	for _, field := range fl.List {
		typStr := typeString(field.Type)
		if len(field.Names) == 0 {
			types = append(types, typStr)
			continue
		}
		// named return: one type entry per name
		for range field.Names {
			types = append(types, typStr)
		}
	}
	return types
}

// typeString converts an ast.Expr to its source representation.
func typeString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + typeString(t.X)
	case *ast.SelectorExpr:
		return typeString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + typeString(t.Elt)
		}
		return "[...]" + typeString(t.Elt)
	case *ast.MapType:
		return "map[" + typeString(t.Key) + "]" + typeString(t.Value)
	case *ast.ChanType:
		return "chan " + typeString(t.Value)
	case *ast.FuncType:
		return "func(...)"
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.Ellipsis:
		return "..." + typeString(t.Elt)
	case *ast.IndexExpr:
		// generic: T[A]
		return typeString(t.X) + "[" + typeString(t.Index) + "]"
	case *ast.IndexListExpr:
		// generic: T[A, B] (go/ast adds this in Go 1.18)
		var parts []string
		for _, idx := range t.Indices {
			parts = append(parts, typeString(idx))
		}
		return typeString(t.X) + "[" + strings.Join(parts, ", ") + "]"
	default:
		return fmt.Sprintf("%T", expr)
	}
}
