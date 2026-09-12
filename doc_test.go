package evtc

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDocComments fails when an exported declaration of the module lacks a
// doc comment, so that go doc ./... stays complete: package clauses, types,
// functions, methods, constants, variables, struct fields and interface
// methods.
func TestDocComments(t *testing.T) {
	for _, dir := range []string{".", "timeline", "extensions/healingstats", "cmd/evtcparser", "examples/sabetha"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		fset := token.NewFileSet()
		documented := false
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			documented = documented || f.Doc != nil
			for _, m := range undocumented(f) {
				t.Errorf("%s: %s has no doc comment", fset.Position(m.pos), m.what)
			}
		}
		if !documented {
			t.Errorf("%s: no package comment", dir)
		}
	}
}

type missingDoc struct {
	pos  token.Pos
	what string
}

// undocumented lists the exported declarations of a file without a doc
// comment. Struct fields and interface methods accept a trailing comment.
func undocumented(f *ast.File) []missingDoc {
	var out []missingDoc
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Name.IsExported() && (d.Recv == nil || exportedReceiver(d.Recv)) && d.Doc == nil {
				out = append(out, missingDoc{d.Pos(), "func " + d.Name.Name})
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if !s.Name.IsExported() {
						continue
					}
					if s.Doc == nil && d.Doc == nil {
						out = append(out, missingDoc{s.Pos(), "type " + s.Name.Name})
					}
					switch t := s.Type.(type) {
					case *ast.StructType:
						for _, fl := range t.Fields.List {
							if names := exportedNames(fl.Names); len(names) > 0 && fl.Doc == nil && fl.Comment == nil {
								out = append(out, missingDoc{fl.Pos(), "field " + s.Name.Name + "." + strings.Join(names, ",")})
							}
						}
					case *ast.InterfaceType:
						for _, m := range t.Methods.List {
							if names := exportedNames(m.Names); len(names) > 0 && m.Doc == nil && m.Comment == nil {
								out = append(out, missingDoc{m.Pos(), "method " + s.Name.Name + "." + names[0]})
							}
						}
					}
				case *ast.ValueSpec:
					if names := exportedNames(s.Names); len(names) > 0 && s.Doc == nil && s.Comment == nil && d.Doc == nil {
						out = append(out, missingDoc{s.Pos(), "value " + strings.Join(names, ",")})
					}
				}
			}
		}
	}
	return out
}

func exportedNames(idents []*ast.Ident) []string {
	var names []string
	for _, id := range idents {
		if id.IsExported() {
			names = append(names, id.Name)
		}
	}
	return names
}

// exportedReceiver reports whether a receiver names an exported type,
// generic receivers included.
func exportedReceiver(recv *ast.FieldList) bool {
	if len(recv.List) == 0 {
		return false
	}
	t := recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	switch idx := t.(type) {
	case *ast.IndexExpr:
		t = idx.X
	case *ast.IndexListExpr:
		t = idx.X
	}
	id, ok := t.(*ast.Ident)
	return ok && id.IsExported()
}
