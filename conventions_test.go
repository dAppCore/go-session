// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var testNamePattern = regexp.MustCompile(`^Test[A-Za-z0-9]+(?:_[A-Za-z0-9]+)+_(Good|Bad|Ugly)$`)

func TestConventions_BannedImports_Good(t *testing.T) {
	files := parsePackageFiles(t)

	banned := map[string]string{
		"errors":                "use coreerr.E(op, msg, err) for package errors",
		"github.com/pkg/errors": "use coreerr.E(op, msg, err) for package errors",
	}

	for _, file := range files {
		if strings.HasSuffix(file.path, "_test.go") {
			continue
		}

		for _, spec := range file.ast.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			if strings.HasPrefix(path, "forge.lthn.ai/") {
				t.Errorf("%s imports %q; use dappco.re/go/core/... paths instead", file.path, path)
				continue
			}
			if reason, ok := banned[path]; ok {
				t.Errorf("%s imports %q; %s", file.path, path, reason)
			}
		}
	}
}

func TestConventions_TestNaming_Good(t *testing.T) {
	files := parsePackageFiles(t)

	for _, file := range files {
		if !strings.HasSuffix(file.path, "_test.go") {
			continue
		}

		for _, decl := range file.ast.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			if !strings.HasPrefix(fn.Name.Name, "Test") || fn.Name.Name == "TestMain" {
				continue
			}
			if !isTestingTFunc(fn) {
				continue
			}
			if !testNamePattern.MatchString(fn.Name.Name) {
				t.Errorf("%s contains %s; expected TestFunctionName_Context_Good/Bad/Ugly", file.path, fn.Name.Name)
			}
		}
	}
}

func TestConventions_UsageComments_Good(t *testing.T) {
	files := parsePackageFiles(t)

	for _, file := range files {
		if strings.HasSuffix(file.path, "_test.go") {
			continue
		}

		for _, decl := range file.ast.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv != nil || !d.Name.IsExported() {
					continue
				}
				if !hasDocPrefix(commentText(d.Doc), d.Name.Name) {
					t.Errorf("%s: exported function %s needs a usage comment starting with %s", file.path, d.Name.Name, d.Name.Name)
				}
			case *ast.GenDecl:
				for i, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if !s.Name.IsExported() {
							continue
						}
						if !hasDocPrefix(commentText(typeDocGroup(d, s, i)), s.Name.Name) {
							t.Errorf("%s: exported type %s needs a usage comment starting with %s", file.path, s.Name.Name, s.Name.Name)
						}
					case *ast.ValueSpec:
						doc := valueDocGroup(d, s, i)
						for _, name := range s.Names {
							if !name.IsExported() {
								continue
							}
							if !hasDocPrefix(commentText(doc), name.Name) {
								t.Errorf("%s: exported declaration %s needs a usage comment starting with %s", file.path, name.Name, name.Name)
							}
						}
					}
				}
			}
		}
	}
}

type parsedFile struct {
	path string
	ast  *ast.File
}

func parsePackageFiles(t *testing.T) []parsedFile {
	t.Helper()

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	pkg, ok := pkgs["session"]
	if !ok {
		t.Fatal("package session not found")
	}

	paths := filePaths(pkg.Files)
	slices.Sort(paths)
	files := make([]parsedFile, 0, len(paths))
	for _, path := range paths {
		files = append(files, parsedFile{
			path: filepath.Base(path),
			ast:  pkg.Files[path],
		})
	}
	return files
}

func filePaths(files map[string]*ast.File) []string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	return paths
}

func isTestingTFunc(fn *ast.FuncDecl) bool {
	if fn.Type == nil || fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
		return false
	}

	param := fn.Type.Params.List[0]
	star, ok := param.Type.(*ast.StarExpr)
	if !ok {
		return false
	}

	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	return pkg.Name == "testing" && sel.Sel.Name == "T"
}

func typeDocGroup(decl *ast.GenDecl, spec *ast.TypeSpec, index int) *ast.CommentGroup {
	if spec.Doc != nil {
		return spec.Doc
	}
	if len(decl.Specs) == 1 && index == 0 {
		return decl.Doc
	}
	return nil
}

func valueDocGroup(decl *ast.GenDecl, spec *ast.ValueSpec, index int) *ast.CommentGroup {
	if spec.Doc != nil {
		return spec.Doc
	}
	if len(decl.Specs) == 1 && index == 0 {
		return decl.Doc
	}
	return nil
}

func commentText(group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}
	return strings.TrimSpace(group.Text())
}

func hasDocPrefix(text, name string) bool {
	if text == "" || !strings.HasPrefix(text, name) {
		return false
	}
	if len(text) == len(name) {
		return true
	}

	next := text[len(name)]
	return (next < 'A' || next > 'Z') && (next < 'a' || next > 'z') && (next < '0' || next > '9') && next != '_'
}
