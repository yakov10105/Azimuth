//go:build cgo

package ingestion

import (
	sitter "github.com/smacker/go-tree-sitter"
)

// extractCSharpCallSites recursively walks a tree-sitter node subtree (typically
// a method body) and returns one CSharpCallSite per invocation_expression found.
func extractCSharpCallSites(callerFQN, ns, className, path string, bodyNode *sitter.Node, src []byte) []CSharpCallSite {
	if bodyNode == nil || bodyNode.IsNull() {
		return nil
	}
	var sites []CSharpCallSite
	walkCSNode(bodyNode, src, func(n *sitter.Node) {
		if n.Type() != "invocation_expression" {
			return
		}
		exprNode := n.ChildByFieldName("expression")
		if exprNode == nil || exprNode.IsNull() {
			return
		}
		expr := csharpCallExprText(exprNode, src)
		if expr == "" {
			return
		}
		sites = append(sites, CSharpCallSite{
			CallerFQN: callerFQN,
			Namespace: ns,
			ClassName: className,
			Expr:      expr,
			File:      path,
			Line:      int(n.StartPoint().Row) + 1,
		})
	})
	return sites
}

// csharpCallExprText converts a tree-sitter expression node to a call target string.
// Returns "" for expressions that cannot be named (e.g. delegate invocations).
func csharpCallExprText(n *sitter.Node, src []byte) string {
	if n == nil || n.IsNull() {
		return ""
	}
	switch n.Type() {
	case "this_expression":
		// Caller is "this" — name will come from the parent member_access_expression.
		return ""
	case "identifier":
		return csNodeText(n, src)
	case "member_access_expression":
		objNode := n.ChildByFieldName("expression")
		nameNode := n.ChildByFieldName("name")
		if nameNode == nil || nameNode.IsNull() {
			return csNodeText(n, src)
		}
		name := csNodeText(nameNode, src)
		if objNode != nil && !objNode.IsNull() && objNode.Type() == "this_expression" {
			return "this." + name
		}
		qualifier := csNodeText(objNode, src)
		if qualifier == "" {
			return name
		}
		return qualifier + "." + name
	default:
		return ""
	}
}

// walkCSNode traverses a tree-sitter node depth-first, calling fn on every node.
func walkCSNode(n *sitter.Node, src []byte, fn func(*sitter.Node)) {
	if n == nil || n.IsNull() {
		return
	}
	fn(n)
	for i := 0; i < int(n.ChildCount()); i++ {
		child := n.Child(i)
		if child != nil && !child.IsNull() {
			walkCSNode(child, src, fn)
		}
	}
}
