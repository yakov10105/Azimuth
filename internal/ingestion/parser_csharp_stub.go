//go:build !cgo

package ingestion

import "errors"

// ErrCGoRequired is returned by the C# parser on platforms where CGo is unavailable.
// Build with CGO_ENABLED=1 and a C compiler (GCC or Clang) to enable C# parsing.
var ErrCGoRequired = errors.New("csharp parser: CGo is required (build with CGO_ENABLED=1 and a C compiler)")

// CSharpParser is a no-op stub for non-CGo builds.
type CSharpParser struct{}

// NewCSharpParser returns a stub parser. All parse calls will return ErrCGoRequired.
func NewCSharpParser() *CSharpParser { return &CSharpParser{} }

// ParseFile always returns ErrCGoRequired on non-CGo builds.
func (p *CSharpParser) ParseFile(_ string) (*CSharpFile, error) {
	return nil, ErrCGoRequired
}

// ParseBytes always returns ErrCGoRequired on non-CGo builds.
func (p *CSharpParser) ParseBytes(_ string, _ []byte) (*CSharpFile, error) {
	return nil, ErrCGoRequired
}
