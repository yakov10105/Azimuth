package cli

import (
	"strings"

	"github.com/azimuth/azimuth/internal/graph"
	"github.com/azimuth/azimuth/internal/ingestion"
)

// buildWriteBatch converts PipelineData from the ingestion coordinator into a
// graph.WriteBatch ready for Neo4j upsert. Keeps the ingestion and graph
// packages decoupled — conversion lives at the CLI (wiring) layer.
func buildWriteBatch(data *ingestion.PipelineData) graph.WriteBatch {
	var batch graph.WriteBatch

	// Build an interface FQN lookup so we can create IMPLEMENTS edges for
	// interfaces whose definition is present in the parsed file set.
	ifaceFQNByName := buildIfaceLookup(data)

	for _, f := range data.GoFiles {
		batch.Files = append(batch.Files, graph.NodeFile{
			Path:     f.Path,
			Language: "go",
			Package:  f.Package,
		})

		for _, fn := range f.Functions {
			fqn := goFQN(f.Package, "", fn.Name)
			batch.Functions = append(batch.Functions, graph.NodeFunction{
				FQN:       fqn,
				Name:      fn.Name,
				FilePath:  f.Path,
				StartLine: fn.StartLine,
				EndLine:   fn.EndLine,
				Language:  "go",
			})
			batch.DefinedIn = append(batch.DefinedIn, graph.EdgeDefinedIn{
				NodeFQN:   fqn,
				NodeLabel: "Function",
				FilePath:  f.Path,
			})
		}

		for _, m := range f.Methods {
			fqn := goFQN(f.Package, m.Receiver, m.Name)
			batch.Methods = append(batch.Methods, graph.NodeMethod{
				FQN:       fqn,
				Name:      m.Name,
				Receiver:  m.Receiver,
				FilePath:  f.Path,
				StartLine: m.StartLine,
				EndLine:   m.EndLine,
			})
			batch.DefinedIn = append(batch.DefinedIn, graph.EdgeDefinedIn{
				NodeFQN:   fqn,
				NodeLabel: "Method",
				FilePath:  f.Path,
			})
		}

		for _, s := range f.Structs {
			fqn := goFQN(f.Package, "", s.Name)
			batch.Structs = append(batch.Structs, graph.NodeStruct{
				FQN:      fqn,
				Name:     s.Name,
				Package:  f.Package,
				FilePath: f.Path,
			})
			batch.DefinedIn = append(batch.DefinedIn, graph.EdgeDefinedIn{
				NodeFQN:   fqn,
				NodeLabel: "Struct",
				FilePath:  f.Path,
			})

			// HAS_METHOD: match methods in this file whose receiver is this struct.
			structBase := strings.TrimPrefix(s.Name, "*")
			for _, m := range f.Methods {
				if strings.TrimPrefix(m.Receiver, "*") == structBase {
					batch.HasMethod = append(batch.HasMethod, graph.EdgeHasMethod{
						OwnerFQN:  fqn,
						MethodFQN: goFQN(f.Package, m.Receiver, m.Name),
					})
				}
			}
		}

		for _, iface := range f.Interfaces {
			fqn := goFQN(f.Package, "", iface.Name)
			batch.Interfaces = append(batch.Interfaces, graph.NodeInterface{
				FQN:                fqn,
				Name:               iface.Name,
				PackageOrNamespace: f.Package,
				FilePath:           f.Path,
			})
			batch.DefinedIn = append(batch.DefinedIn, graph.EdgeDefinedIn{
				NodeFQN:   fqn,
				NodeLabel: "Interface",
				FilePath:  f.Path,
			})
		}
	}

	for _, f := range data.CSharpFiles {
		ns := f.Namespace
		batch.Files = append(batch.Files, graph.NodeFile{
			Path:     f.Path,
			Language: "csharp",
			Package:  ns,
		})

		for _, cls := range f.Classes {
			clsNS := cls.Namespace
			if clsNS == "" {
				clsNS = ns
			}
			clsFQN := csharpFQN(clsNS, cls.Name, "")
			batch.Classes = append(batch.Classes, graph.NodeClass{
				FQN:       clsFQN,
				Name:      cls.Name,
				Namespace: clsNS,
				FilePath:  f.Path,
			})
			batch.DefinedIn = append(batch.DefinedIn, graph.EdgeDefinedIn{
				NodeFQN:   clsFQN,
				NodeLabel: "Class",
				FilePath:  f.Path,
			})

			for _, m := range cls.Methods {
				mFQN := csharpFQN(clsNS, cls.Name, m.Name)
				batch.Methods = append(batch.Methods, graph.NodeMethod{
					FQN:       mFQN,
					Name:      m.Name,
					Receiver:  cls.Name,
					FilePath:  f.Path,
					StartLine: m.StartLine,
					EndLine:   m.EndLine,
				})
				batch.DefinedIn = append(batch.DefinedIn, graph.EdgeDefinedIn{
					NodeFQN:   mFQN,
					NodeLabel: "Method",
					FilePath:  f.Path,
				})
				batch.HasMethod = append(batch.HasMethod, graph.EdgeHasMethod{
					OwnerFQN:  clsFQN,
					MethodFQN: mFQN,
				})
			}

			// IMPLEMENTS edges for interfaces present in the parsed file set.
			for _, ifaceName := range cls.Interfaces {
				if ifaceFQN, ok := ifaceFQNByName[ifaceName]; ok {
					batch.Implements = append(batch.Implements, graph.EdgeImplements{
						ImplementorFQN: clsFQN,
						InterfaceFQN:   ifaceFQN,
					})
				}
			}
		}

		for _, iface := range f.Interfaces {
			ifaceNS := iface.Namespace
			if ifaceNS == "" {
				ifaceNS = ns
			}
			ifaceFQN := csharpFQN(ifaceNS, iface.Name, "")
			batch.Interfaces = append(batch.Interfaces, graph.NodeInterface{
				FQN:                ifaceFQN,
				Name:               iface.Name,
				PackageOrNamespace: ifaceNS,
				FilePath:           f.Path,
			})
			batch.DefinedIn = append(batch.DefinedIn, graph.EdgeDefinedIn{
				NodeFQN:   ifaceFQN,
				NodeLabel: "Interface",
				FilePath:  f.Path,
			})
		}
	}

	allEdges := append(data.GoEdges, data.CSharpEdges...)
	for _, e := range allEdges {
		batch.Calls = append(batch.Calls, graph.EdgeCalls{
			CallerFQN:    e.CallerFQN,
			CalleeFQN:    e.CalleeFQN,
			CallSiteFile: e.CallSiteFile,
			CallSiteLine: e.CallSiteLine,
		})
	}

	return batch
}

// buildIfaceLookup returns a map from simple interface name → FQN for every
// C# interface in the parsed file set. Used to resolve IMPLEMENTS edges.
func buildIfaceLookup(data *ingestion.PipelineData) map[string]string {
	m := make(map[string]string)
	for _, f := range data.CSharpFiles {
		ns := f.Namespace
		for _, iface := range f.Interfaces {
			ifaceNS := iface.Namespace
			if ifaceNS == "" {
				ifaceNS = ns
			}
			m[iface.Name] = csharpFQN(ifaceNS, iface.Name, "")
		}
	}
	return m
}

// goFQN builds the Azimuth FQN for a Go function, method, struct, or interface.
// Pointer receiver prefixes ("*") are stripped for stable FQNs.
func goFQN(pkg, receiver, name string) string {
	if receiver != "" {
		recv := strings.TrimPrefix(receiver, "*")
		return pkg + "." + recv + "." + name
	}
	return pkg + "." + name
}

// csharpFQN builds the Azimuth FQN for a C# type or method.
// When method is empty, returns the type FQN (class or interface).
func csharpFQN(ns, typeName, method string) string {
	base := typeName
	if ns != "" {
		base = ns + "." + typeName
	}
	if method != "" {
		return base + "." + method
	}
	return base
}
