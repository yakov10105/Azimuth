//go:build cgo

package ingestion

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/csharp"
)

// CSharpParser parses .cs source files using the Tree-sitter C# grammar and extracts
// structural symbols. Requires CGo and a C compiler. On systems where CGo is unavailable
// the stub in parser_csharp_stub.go is compiled instead.
type CSharpParser struct{}

// NewCSharpParser creates a new CSharpParser.
func NewCSharpParser() *CSharpParser {
	return &CSharpParser{}
}

// ParseFile reads a .cs file from disk and extracts its structural symbols.
// Syntax errors are logged as warnings; extraction continues on the partial tree.
func (p *CSharpParser) ParseFile(path string) (*CSharpFile, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("csharp parser: read %s: %w", path, err)
	}
	return p.ParseBytes(path, src)
}

// ParseBytes parses a .cs source from an in-memory byte slice.
// Syntax errors are logged as warnings; extraction continues on the partial tree.
func (p *CSharpParser) ParseBytes(path string, src []byte) (*CSharpFile, error) {
	root, err := sitter.ParseCtx(context.Background(), src, csharp.GetLanguage())
	if err != nil {
		return nil, fmt.Errorf("csharp parser: parse %s: %w", path, err)
	}

	file := &CSharpFile{Path: path}
	if root.HasError() {
		slog.Warn("csharp parser: syntax errors detected, extracting partial results",
			"path", path)
		file.HasErrors = true
	}

	p.walkCompilationUnit(root, src, file, path)
	return file, nil
}

// --- tree walkers ---

func (p *CSharpParser) walkCompilationUnit(root *sitter.Node, src []byte, file *CSharpFile, path string) {
	var currentNS string
	nsSet := false

	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		if child == nil || child.IsNull() {
			continue
		}
		switch child.Type() {
		case "using_directive":
			file.Usings = append(file.Usings, csExtractUsing(child, src))
		case "namespace_declaration":
			ns := csNodeText(child.ChildByFieldName("name"), src)
			if !nsSet {
				file.Namespace = ns
				nsSet = true
			}
			if body := child.ChildByFieldName("body"); body != nil {
				p.walkDeclList(body, src, file, ns, path)
			}
		case "file_scoped_namespace_declaration":
			ns := csNodeText(child.ChildByFieldName("name"), src)
			currentNS = ns
			if !nsSet {
				file.Namespace = ns
				nsSet = true
			}
		case "class_declaration", "record_declaration", "struct_declaration":
			file.Classes = append(file.Classes, p.extractClasses(child, src, currentNS, path, file)...)
		case "interface_declaration":
			file.Interfaces = append(file.Interfaces, p.extractInterface(child, src, currentNS))
		}
	}
}

// walkDeclList walks the declaration_list that forms a namespace body.
func (p *CSharpParser) walkDeclList(body *sitter.Node, src []byte, file *CSharpFile, ns, path string) {
	for i := 0; i < int(body.ChildCount()); i++ {
		child := body.Child(i)
		if child == nil || child.IsNull() {
			continue
		}
		switch child.Type() {
		case "class_declaration", "record_declaration", "struct_declaration":
			file.Classes = append(file.Classes, p.extractClasses(child, src, ns, path, file)...)
		case "interface_declaration":
			file.Interfaces = append(file.Interfaces, p.extractInterface(child, src, ns))
		}
	}
}

// extractClasses extracts a class (or record/struct) declaration and any nested classes.
// Returns a flat slice: the outer class at index 0, then any nested classes.
// Call sites found in method bodies are appended directly to file.Calls.
func (p *CSharpParser) extractClasses(n *sitter.Node, src []byte, ns, path string, file *CSharpFile) []CSharpClass {
	cls := CSharpClass{Namespace: ns}
	var nested []CSharpClass

	if name := n.ChildByFieldName("name"); name != nil {
		cls.Name = csNodeText(name, src)
	}

	// Collect modifiers and attribute lists from direct children.
	for i := 0; i < int(n.ChildCount()); i++ {
		child := n.Child(i)
		if child == nil || child.IsNull() {
			continue
		}
		switch child.Type() {
		case "modifier":
			switch csNodeText(child, src) {
			case "public", "protected", "private", "internal":
				if cls.AccessModifier == "" {
					cls.AccessModifier = csNodeText(child, src)
				}
			case "partial":
				cls.IsPartial = true
			}
		case "attribute_list":
			cls.Attributes = append(cls.Attributes, csExtractAttrs(child, src, "class")...)
		}
	}

	// Base class and implemented interfaces from the "bases" field (base_list node).
	if bases := csBasesNode(n); bases != nil {
		for i := 0; i < int(bases.NamedChildCount()); i++ {
			bc := bases.NamedChild(i)
			if bc == nil || bc.IsNull() {
				continue
			}
			text := csNodeText(bc, src)
			if text == "" {
				continue
			}
			if cls.BaseClass == "" {
				cls.BaseClass = text
			} else {
				cls.Interfaces = append(cls.Interfaces, text)
			}
		}
	}

	// Walk the class body for methods, properties, and nested type declarations.
	if body := n.ChildByFieldName("body"); body != nil {
		for i := 0; i < int(body.ChildCount()); i++ {
			child := body.Child(i)
			if child == nil || child.IsNull() {
				continue
			}
			switch child.Type() {
			case "method_declaration":
				m := csExtractMethod(child, src)
				cls.Methods = append(cls.Methods, m)
				// Extract call sites from the method body.
				if m.Name != "" {
					methodFQN := buildCSharpFQN(ns, cls.Name, m.Name)
					if bodyNode := child.ChildByFieldName("body"); bodyNode != nil {
						sites := extractCSharpCallSites(methodFQN, ns, cls.Name, path, bodyNode, src)
						file.Calls = append(file.Calls, sites...)
					}
				}
			case "property_declaration":
				cls.Properties = append(cls.Properties, csExtractProperty(child, src))
			case "class_declaration", "record_declaration", "struct_declaration":
				nested = append(nested, p.extractClasses(child, src, ns, path, file)...)
			}
		}
	}

	return append([]CSharpClass{cls}, nested...)
}

// extractInterface extracts an interface_declaration node.
func (p *CSharpParser) extractInterface(n *sitter.Node, src []byte, ns string) CSharpInterface {
	iface := CSharpInterface{Namespace: ns}

	if name := n.ChildByFieldName("name"); name != nil {
		iface.Name = csNodeText(name, src)
	}

	if body := n.ChildByFieldName("body"); body != nil {
		for i := 0; i < int(body.ChildCount()); i++ {
			child := body.Child(i)
			if child == nil || child.IsNull() || child.Type() != "method_declaration" {
				continue
			}
			m := CSharpInterfaceMethod{}
			if name := child.ChildByFieldName("name"); name != nil {
				m.Name = csNodeText(name, src)
			}
			if ret := child.ChildByFieldName("type"); ret != nil {
				m.ReturnType = csNodeText(ret, src)
			}
			if params := child.ChildByFieldName("parameters"); params != nil {
				m.Parameters = csExtractParams(params, src)
			}
			if m.Name != "" {
				iface.Methods = append(iface.Methods, m)
			}
		}
	}

	return iface
}

// --- declaration-level helpers ---

func csExtractMethod(n *sitter.Node, src []byte) CSharpMethod {
	m := CSharpMethod{
		StartLine: int(n.StartPoint().Row) + 1,
		EndLine:   int(n.EndPoint().Row) + 1,
	}
	if name := n.ChildByFieldName("name"); name != nil {
		m.Name = csNodeText(name, src)
	}
	if ret := n.ChildByFieldName("type"); ret != nil {
		m.ReturnType = csNodeText(ret, src)
	}
	if params := n.ChildByFieldName("parameters"); params != nil {
		m.Parameters = csExtractParams(params, src)
	}
	for i := 0; i < int(n.ChildCount()); i++ {
		child := n.Child(i)
		if child == nil || child.IsNull() {
			continue
		}
		switch child.Type() {
		case "modifier":
			switch csNodeText(child, src) {
			case "public", "protected", "private", "internal":
				if m.AccessModifier == "" {
					m.AccessModifier = csNodeText(child, src)
				}
			case "override":
				m.IsOverride = true
			case "virtual":
				m.IsVirtual = true
			case "static":
				m.IsStatic = true
			case "abstract":
				m.IsAbstract = true
			}
		case "attribute_list":
			m.Attributes = append(m.Attributes, csExtractAttrs(child, src, "method")...)
		}
	}
	return m
}

func csExtractProperty(n *sitter.Node, src []byte) CSharpProperty {
	prop := CSharpProperty{}
	if name := n.ChildByFieldName("name"); name != nil {
		prop.Name = csNodeText(name, src)
	}
	if typ := n.ChildByFieldName("type"); typ != nil {
		prop.Type = csNodeText(typ, src)
	}
	// Use "accessors" field (points to accessor_list node).
	// Fall back to searching for an accessor_list child if the field isn't found.
	accessors := n.ChildByFieldName("accessors")
	if accessors == nil {
		for i := 0; i < int(n.ChildCount()); i++ {
			c := n.Child(i)
			if c != nil && !c.IsNull() && c.Type() == "accessor_list" {
				accessors = c
				break
			}
		}
	}
	if accessors != nil {
		text := csNodeText(accessors, src)
		prop.HasGetter = strings.Contains(text, "get")
		prop.HasSetter = strings.Contains(text, "set") || strings.Contains(text, "init")
	}
	return prop
}

func csExtractParams(n *sitter.Node, src []byte) []CSharpParam {
	var params []CSharpParam
	for i := 0; i < int(n.NamedChildCount()); i++ {
		child := n.NamedChild(i)
		if child == nil || child.IsNull() || child.Type() != "parameter" {
			continue
		}
		p := CSharpParam{}
		if typ := child.ChildByFieldName("type"); typ != nil {
			p.Type = csNodeText(typ, src)
		}
		if name := child.ChildByFieldName("name"); name != nil {
			p.Name = csNodeText(name, src)
		}
		params = append(params, p)
	}
	return params
}

func csExtractAttrs(attrList *sitter.Node, src []byte, target string) []CSharpAttribute {
	var attrs []CSharpAttribute
	for i := 0; i < int(attrList.NamedChildCount()); i++ {
		child := attrList.NamedChild(i)
		if child == nil || child.IsNull() || child.Type() != "attribute" {
			continue
		}
		attr := CSharpAttribute{Target: target}
		// First named child is the attribute name.
		if child.NamedChildCount() > 0 {
			attr.Name = csNodeText(child.NamedChild(0), src)
		}
		// Second named child (if any) is the argument list.
		if child.NamedChildCount() > 1 {
			attr.Arguments = []string{csNodeText(child.NamedChild(1), src)}
		}
		if attr.Name != "" {
			attrs = append(attrs, attr)
		}
	}
	return attrs
}

func csExtractUsing(n *sitter.Node, src []byte) CSharpUsing {
	// Strip "using", optional "static", trailing ";", then check for alias "=".
	text := strings.TrimSpace(csNodeText(n, src))
	text = strings.TrimSuffix(text, ";")
	text = strings.TrimPrefix(text, "using ")
	text = strings.TrimPrefix(text, "static ")
	text = strings.TrimSpace(text)

	u := CSharpUsing{}
	if idx := strings.Index(text, " = "); idx >= 0 {
		u.Alias = strings.TrimSpace(text[:idx])
		u.Namespace = strings.TrimSpace(text[idx+3:])
	} else {
		u.Namespace = text
	}
	return u
}

// csBasesNode returns the base_list node from a class/record/struct declaration,
// trying the "bases" field first and falling back to scanning children.
func csBasesNode(n *sitter.Node) *sitter.Node {
	if b := n.ChildByFieldName("bases"); b != nil && !b.IsNull() {
		return b
	}
	for i := 0; i < int(n.ChildCount()); i++ {
		c := n.Child(i)
		if c != nil && !c.IsNull() && c.Type() == "base_list" {
			return c
		}
	}
	return nil
}

// csNodeText returns the source text of a node (nil-safe).
func csNodeText(n *sitter.Node, src []byte) string {
	if n == nil || n.IsNull() {
		return ""
	}
	return string(src[n.StartByte():n.EndByte()])
}
