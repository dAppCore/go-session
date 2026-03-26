// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var testNamePattern = regexp.MustCompile(`^Test[A-Za-z0-9]+(?:_[A-Za-z0-9]+)+_(Good|Bad|Ugly)$`)

func TestConventions_BannedImports_Good(t *testing.T) {
	files := parseGoFiles(t, ".")

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
	files := parseGoFiles(t, ".")

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
			if !isTestingTFunc(file, fn) {
				continue
			}
			if !testNamePattern.MatchString(fn.Name.Name) {
				t.Errorf("%s contains %s; expected TestFunctionName_Context_Good/Bad/Ugly", file.path, fn.Name.Name)
			}
		}
	}
}

func TestConventions_UsageComments_Good(t *testing.T) {
	files := parseGoFiles(t, ".")

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
	path                string
	ast                 *ast.File
	testingImportNames  map[string]struct{}
	hasTestingDotImport bool
}

func parseGoFiles(t *testing.T, dir string) []parsedFile {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatalf("glob Go files: %v", err)
	}
	if len(paths) == 0 {
		t.Fatalf("no Go files found in %s", dir)
	}

	slices.Sort(paths)

	fset := token.NewFileSet()
	files := make([]parsedFile, 0, len(paths))
	for _, path := range paths {
		fileAST, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		testingImportNames, hasTestingDotImport := testingImports(fileAST)
		files = append(files, parsedFile{
			path:                filepath.Base(path),
			ast:                 fileAST,
			testingImportNames:  testingImportNames,
			hasTestingDotImport: hasTestingDotImport,
		})
	}
	return files
}

func TestParseGoFiles_MultiplePackages_Good(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "session.go"), "package session\n")
	writeTestFile(t, filepath.Join(dir, "session_external_test.go"), "package session_test\n")
	writeTestFile(t, filepath.Join(dir, "README.md"), "# ignored\n")

	files := parseGoFiles(t, dir)
	if len(files) != 2 {
		t.Fatalf("expected 2 Go files, got %d", len(files))
	}

	names := []string{files[0].path, files[1].path}
	slices.Sort(names)
	if names[0] != "session.go" || names[1] != "session_external_test.go" {
		t.Fatalf("unexpected files: %v", names)
	}
}

func TestIsTestingTFunc_AliasedImport_Good(t *testing.T) {
	fileAST, fn := parseTestFunc(t, `
package session_test

import t "testing"

func TestAliasedImport_Context_Good(testcase *t.T) {}
`, "TestAliasedImport_Context_Good")

	names, hasDotImport := testingImports(fileAST)
	file := parsedFile{
		ast:                 fileAST,
		testingImportNames:  names,
		hasTestingDotImport: hasDotImport,
	}

	if !isTestingTFunc(file, fn) {
		t.Fatal("expected aliased *testing.T signature to be recognised")
	}
}

func TestIsTestingTFunc_DotImport_Good(t *testing.T) {
	fileAST, fn := parseTestFunc(t, `
package session_test

import . "testing"

func TestDotImport_Context_Good(testcase *T) {}
`, "TestDotImport_Context_Good")

	names, hasDotImport := testingImports(fileAST)
	file := parsedFile{
		ast:                 fileAST,
		testingImportNames:  names,
		hasTestingDotImport: hasDotImport,
	}

	if !isTestingTFunc(file, fn) {
		t.Fatal("expected dot-imported *testing.T signature to be recognised")
	}
}

func testingImports(file *ast.File) (map[string]struct{}, bool) {
	names := make(map[string]struct{})
	hasDotImport := false

	for _, spec := range file.Imports {
		path := strings.Trim(spec.Path.Value, `"`)
		if path != "testing" {
			continue
		}
		if spec.Name == nil {
			names["testing"] = struct{}{}
			continue
		}
		switch spec.Name.Name {
		case ".":
			hasDotImport = true
		case "_":
			continue
		default:
			names[spec.Name.Name] = struct{}{}
		}
	}

	return names, hasDotImport
}

func isTestingTFunc(file parsedFile, fn *ast.FuncDecl) bool {
	if fn.Type == nil || fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
		return false
	}

	param := fn.Type.Params.List[0]
	star, ok := param.Type.(*ast.StarExpr)
	if !ok {
		return false
	}

	switch expr := star.X.(type) {
	case *ast.Ident:
		return file.hasTestingDotImport && expr.Name == "T"
	case *ast.SelectorExpr:
		pkg, ok := expr.X.(*ast.Ident)
		if !ok {
			return false
		}
		if expr.Sel.Name != "T" {
			return false
		}
		_, ok = file.testingImportNames[pkg.Name]
		return ok
	default:
		return false
	}
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

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func parseTestFunc(t *testing.T, src, name string) (*ast.File, *ast.FuncDecl) {
	t.Helper()

	fset := token.NewFileSet()
	fileAST, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse test source: %v", err)
	}

	for _, decl := range fileAST.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == name {
			return fileAST, fn
		}
	}

	t.Fatalf("function %s not found", name)
	return nil, nil
}
