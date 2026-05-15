package ingestion

import (
	"go/ast"
	"go/token"
	"strings"
)

// buildGoFQN constructs the Azimuth FQN for a Go function or method.
// A pointer receiver ("*User") has its leading "*" stripped so the FQN is stable
// regardless of whether the method is declared with a pointer or value receiver.
func buildGoFQN(pkg, receiver, name string) string {
	if receiver != "" {
		recv := strings.TrimPrefix(receiver, "*")
		return pkg + "." + recv + "." + name
	}
	return pkg + "." + name
}

// extractGoCallSites walks body via ast.Inspect and returns one GoCallSite per
// call expression whose target can be expressed as a simple or selector name.
// Calls inside anonymous function literals are attributed to the enclosing callerFQN.
func extractGoCallSites(callerFQN, pkg, path string, body *ast.BlockStmt, fset *token.FileSet) []GoCallSite {
	if body == nil {
		return nil
	}
	var sites []GoCallSite
	ast.Inspect(body, func(n ast.Node) bool {
		ce, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		expr := goCallExprText(ce.Fun)
		if expr == "" {
			return true
		}
		sites = append(sites, GoCallSite{
			CallerFQN: callerFQN,
			Package:   pkg,
			Expr:      expr,
			File:      path,
			Line:      fset.Position(ce.Pos()).Line,
		})
		return true
	})
	return sites
}

// goCallExprText converts the Fun field of an ast.CallExpr to its textual form.
// Returns "" for expressions that cannot be meaningfully named (type assertions,
// index expressions, immediately-invoked function literals, etc.).
func goCallExprText(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		qualifier := goCallExprText(f.X)
		if qualifier == "" {
			return f.Sel.Name
		}
		return qualifier + "." + f.Sel.Name
	case *ast.ParenExpr:
		return goCallExprText(f.X)
	default:
		// FuncLit (immediately invoked), IndexExpr, TypeAssertExpr, etc. — skip.
		return ""
	}
}
