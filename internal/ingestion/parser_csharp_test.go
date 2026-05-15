//go:build cgo

package ingestion

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCSharpParser_StandardFile(t *testing.T) {
	p := NewCSharpParser()
	f, err := p.ParseFile("testdata/standard.cs")
	require.NoError(t, err)

	assert.Equal(t, "Azimuth.Services", f.Namespace)
	assert.False(t, f.HasErrors)

	// using directives
	require.Len(t, f.Usings, 2)
	uPaths := []string{f.Usings[0].Namespace, f.Usings[1].Namespace}
	assert.Contains(t, uPaths, "System")
	assert.Contains(t, uPaths, "System.Collections.Generic")

	// class
	require.Len(t, f.Classes, 1)
	cls := f.Classes[0]
	assert.Equal(t, "UserService", cls.Name)
	assert.Equal(t, "Azimuth.Services", cls.Namespace)
	assert.Equal(t, "public", cls.AccessModifier)
	assert.Equal(t, "BaseService", cls.BaseClass)
	require.Len(t, cls.Interfaces, 1)
	assert.Equal(t, "IUserService", cls.Interfaces[0])
	assert.False(t, cls.IsPartial)

	// properties
	assert.Len(t, cls.Properties, 2)
	propNames := make([]string, len(cls.Properties))
	for i, pr := range cls.Properties {
		propNames[i] = pr.Name
	}
	assert.Contains(t, propNames, "ServiceName")
	assert.Contains(t, propNames, "RetryCount")

	// methods: GetUser, Validate (override), IsValid
	require.Len(t, cls.Methods, 3)
	methodNames := make([]string, len(cls.Methods))
	for i, m := range cls.Methods {
		methodNames[i] = m.Name
	}
	assert.Contains(t, methodNames, "GetUser")
	assert.Contains(t, methodNames, "Validate")
	assert.Contains(t, methodNames, "IsValid")

	var validateMethod CSharpMethod
	for _, m := range cls.Methods {
		if m.Name == "Validate" {
			validateMethod = m
		}
	}
	assert.True(t, validateMethod.IsOverride)

	// line numbers are set
	for _, m := range cls.Methods {
		assert.Greater(t, m.StartLine, 0, "method %s should have StartLine > 0", m.Name)
		assert.GreaterOrEqual(t, m.EndLine, m.StartLine)
	}
}

func TestCSharpParser_PartialClass(t *testing.T) {
	p := NewCSharpParser()
	f, err := p.ParseFile("testdata/partial.cs")
	require.NoError(t, err)
	assert.False(t, f.HasErrors)

	require.Len(t, f.Classes, 1)
	cls := f.Classes[0]
	assert.Equal(t, "OrderService", cls.Name)
	assert.True(t, cls.IsPartial, "OrderService should be marked partial")
	assert.Equal(t, "public", cls.AccessModifier)
}

func TestCSharpParser_NestedClass(t *testing.T) {
	p := NewCSharpParser()
	f, err := p.ParseFile("testdata/nested.cs")
	require.NoError(t, err)
	assert.False(t, f.HasErrors)

	// Both the outer class and the nested class should be extracted (flat list).
	require.GreaterOrEqual(t, len(f.Classes), 2, "expected outer + nested class")

	names := make([]string, len(f.Classes))
	for i, c := range f.Classes {
		names[i] = c.Name
	}
	assert.Contains(t, names, "PaymentProcessor")
	assert.Contains(t, names, "PaymentConfig")
}

func TestCSharpParser_RecordType(t *testing.T) {
	p := NewCSharpParser()
	f, err := p.ParseFile("testdata/record.cs")
	require.NoError(t, err)
	assert.False(t, f.HasErrors)

	// record types are extracted into Classes
	require.NotEmpty(t, f.Classes, "record should be extracted as a class")
	assert.Equal(t, "Person", f.Classes[0].Name)
}

func TestCSharpParser_FileScopedNamespace(t *testing.T) {
	p := NewCSharpParser()
	f, err := p.ParseFile("testdata/file_scoped_ns.cs")
	require.NoError(t, err)
	assert.False(t, f.HasErrors)

	assert.Equal(t, "Azimuth.Handlers", f.Namespace,
		"file-scoped namespace should be extracted correctly")

	require.Len(t, f.Classes, 1)
	assert.Equal(t, "RequestHandler", f.Classes[0].Name)
	assert.Equal(t, "Azimuth.Handlers", f.Classes[0].Namespace)
}

func TestCSharpParser_Attributes(t *testing.T) {
	p := NewCSharpParser()
	f, err := p.ParseFile("testdata/attributes.cs")
	require.NoError(t, err)
	assert.False(t, f.HasErrors)

	require.Len(t, f.Classes, 1)
	cls := f.Classes[0]
	assert.Equal(t, "UsersController", cls.Name)

	// Class-level attributes: [ApiController] and [Route(...)]
	require.NotEmpty(t, cls.Attributes, "class should have attributes")
	classAttrNames := make([]string, len(cls.Attributes))
	for i, a := range cls.Attributes {
		classAttrNames[i] = a.Name
	}
	assert.Contains(t, classAttrNames, "ApiController")
	assert.Contains(t, classAttrNames, "Route")

	// Methods: GetUser, CreateUser, DeleteUser
	require.Len(t, cls.Methods, 3)

	var getUserMethod CSharpMethod
	for _, m := range cls.Methods {
		if m.Name == "GetUser" {
			getUserMethod = m
		}
	}
	require.NotEmpty(t, getUserMethod.Name, "GetUser method should be found")
	assert.NotEmpty(t, getUserMethod.Attributes, "GetUser should have method attributes")

	methodAttrNames := make([]string, len(getUserMethod.Attributes))
	for i, a := range getUserMethod.Attributes {
		methodAttrNames[i] = a.Name
	}
	assert.Contains(t, methodAttrNames, "HttpGet")
}

func TestCSharpParser_EmptyFile(t *testing.T) {
	p := NewCSharpParser()
	f, err := p.ParseFile("testdata/empty.cs")
	require.NoError(t, err)
	assert.False(t, f.HasErrors)

	assert.Equal(t, "Azimuth.Empty", f.Namespace)
	assert.Empty(t, f.Usings)
	assert.Empty(t, f.Classes)
	assert.Empty(t, f.Interfaces)
}

func TestCSharpParser_SyntaxError(t *testing.T) {
	p := NewCSharpParser()
	f, err := p.ParseFile("testdata/broken.cs")

	// Parser must not return an error — tree-sitter recovers partial results.
	require.NoError(t, err)
	assert.True(t, f.HasErrors, "HasErrors should be true for a file with syntax errors")
	// Namespace should still be extractable from partial AST.
	assert.Equal(t, "Azimuth.Broken", f.Namespace)
}

func TestCSharpParser_ParseFile_NotFound(t *testing.T) {
	p := NewCSharpParser()
	_, err := p.ParseFile("testdata/nonexistent.cs")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "csharp parser: read")
}

func BenchmarkCSharpParser_LargeFile(b *testing.B) {
	src, err := os.ReadFile("testdata/large.cs")
	if err != nil {
		b.Fatal(err)
	}
	p := NewCSharpParser()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p.ParseBytes("testdata/large.cs", src); err != nil {
			b.Fatal(err)
		}
	}

	avg := b.Elapsed() / time.Duration(b.N)
	if avg > 50*time.Millisecond {
		b.Errorf("parse too slow: avg %v, want < 50ms", avg)
	}
}
