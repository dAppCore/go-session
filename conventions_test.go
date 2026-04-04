// SPDX-Licence-Identifier: EUPL-1.2
package session

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"regexp"
	"slices"
	"testing"

	core "dappco.re/go/core"
)

var testNamePattern = regexp.MustCompile(`^Test[A-Za-z0-9]+_[A-Za-z0-9]+_(Good|Bad|Ugly)$`)

func TestConventions_BannedImports_Good(t *testing.T) {
	files := parseGoFiles(t, ".")

	banned := map[string]string{
		core.Concat("encoding", "/json"): "use dappco.re/go/core JSON helpers instead",
		core.Concat("error", "s"):        "use core.E/op-aware errors instead",
		core.Concat("f", "mt"):           "use dappco.re/go/core formatting helpers instead",
		"github.com/pkg/errors":          "use coreerr.E(op, msg, err) for package errors",
		core.Concat("o", "s"):            "use dappco.re/go/core filesystem helpers instead",
		core.Concat("o", "s/exec"):       "use session command helpers or core process abstractions instead",
		core.Concat("path", "/filepath"): "use path or dappco.re/go/core path helpers instead",
		core.Concat("string", "s"):       "use dappco.re/go/core string helpers or local helpers instead",
	}

	for _, file := range files {
		for _, spec := range file.ast.Imports {
			importPath := trimQuotes(spec.Path.Value)
			if core.HasPrefix(importPath, "forge.lthn.ai/") {
				t.Errorf("%s imports %q; use dappco.re/go/core/... paths instead", file.path, importPath)
				continue
			}
			if reason, ok := banned[importPath]; ok {
				t.Errorf("%s imports %q; %s", file.path, importPath, reason)
			}
		}
	}
}

func TestConventions_ErrorHandling_Good(t *testing.T) {
	files := parseGoFiles(t, ".")

	for _, file := range files {
		if core.HasSuffix(file.path, "_test.go") {
			continue
		}

		ast.Inspect(file.ast, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}

			switch {
			case pkg.Name == "core" && sel.Sel.Name == "NewError":
				t.Errorf("%s uses core.NewError; use core.E(op, msg, err)", file.path)
			case pkg.Name == "fmt" && sel.Sel.Name == "Errorf":
				t.Errorf("%s uses fmt.Errorf; use core.E(op, msg, err)", file.path)
			case pkg.Name == "errors" && sel.Sel.Name == "New":
				t.Errorf("%s uses errors.New; use core.E(op, msg, err)", file.path)
			}

			return true
		})
	}
}

func TestConventions_TestNaming_Good(t *testing.T) {
	files := parseGoFiles(t, ".")

	for _, file := range files {
		if !core.HasSuffix(file.path, "_test.go") {
			continue
		}

		for _, decl := range file.ast.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			if !core.HasPrefix(fn.Name.Name, "Test") || fn.Name.Name == "TestMain" {
				continue
			}
			if !isTestingTFunc(file, fn) {
				continue
			}
			expectedPrefix := core.Concat("Test", testFileToken(file.path), "_")
			if !core.HasPrefix(fn.Name.Name, expectedPrefix) {
				t.Errorf("%s contains %s; expected prefix %s", file.path, fn.Name.Name, expectedPrefix)
				continue
			}
			if !testNamePattern.MatchString(fn.Name.Name) {
				t.Errorf("%s contains %s; expected TestFile_Function_Good/Bad/Ugly", file.path, fn.Name.Name)
			}
		}
	}
}

func TestConventions_UsageComments_Good(t *testing.T) {
	files := parseGoFiles(t, ".")

	for _, file := range files {
		if core.HasSuffix(file.path, "_test.go") {
			continue
		}

		for _, decl := range file.ast.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv != nil || !d.Name.IsExported() {
					continue
				}
				text := commentText(d.Doc)
				if !hasDocPrefix(text, d.Name.Name) || !hasUsageExample(text) {
					t.Errorf("%s: exported function %s needs a usage comment starting with %s and containing Example:", file.path, d.Name.Name, d.Name.Name)
				}
			case *ast.GenDecl:
				for i, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if !s.Name.IsExported() {
							continue
						}
						text := commentText(typeDocGroup(d, s, i))
						if !hasDocPrefix(text, s.Name.Name) || !hasUsageExample(text) {
							t.Errorf("%s: exported type %s needs a usage comment starting with %s and containing Example:", file.path, s.Name.Name, s.Name.Name)
						}
					case *ast.ValueSpec:
						doc := valueDocGroup(d, s, i)
						for _, name := range s.Names {
							if !name.IsExported() {
								continue
							}
							text := commentText(doc)
							if !hasDocPrefix(text, name.Name) || !hasUsageExample(text) {
								t.Errorf("%s: exported declaration %s needs a usage comment starting with %s and containing Example:", file.path, name.Name, name.Name)
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

	paths := core.PathGlob(path.Join(dir, "*.go"))
	if len(paths) == 0 {
		t.Fatalf("no Go files found in %s", dir)
	}

	slices.Sort(paths)

	fset := token.NewFileSet()
	files := make([]parsedFile, 0, len(paths))
	for _, filePath := range paths {
		fileAST, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", filePath, err)
		}

		testingImportNames, hasTestingDotImport := testingImports(fileAST)
		files = append(files, parsedFile{
			path:                path.Base(filePath),
			ast:                 fileAST,
			testingImportNames:  testingImportNames,
			hasTestingDotImport: hasTestingDotImport,
		})
	}
	return files
}

func TestConventions_ParseGoFilesMultiplePackages_Good(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, path.Join(dir, "session.go"), "package session\n")
	writeTestFile(t, path.Join(dir, "session_external_test.go"), "package session_test\n")
	writeTestFile(t, path.Join(dir, "README.md"), "# ignored\n")

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

func TestConventions_IsTestingTFuncAliasedImport_Good(t *testing.T) {
	fileAST, fn := parseTestFunc(t, `
package session_test

import t "testing"

func TestConventions_AliasedImportContext_Good(testcase *t.T) {}
`, "TestConventions_AliasedImportContext_Good")

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

func TestConventions_IsTestingTFuncDotImport_Good(t *testing.T) {
	fileAST, fn := parseTestFunc(t, `
package session_test

import . "testing"

func TestConventions_DotImportContext_Good(testcase *T) {}
`, "TestConventions_DotImportContext_Good")

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
		importPath := trimQuotes(spec.Path.Value)
		if importPath != "testing" {
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
	return core.Trim(group.Text())
}

func hasDocPrefix(text, name string) bool {
	if text == "" || !core.HasPrefix(text, name) {
		return false
	}
	if len(text) == len(name) {
		return true
	}

	next := text[len(name)]
	return (next < 'A' || next > 'Z') && (next < 'a' || next > 'z') && (next < '0' || next > '9') && next != '_'
}

func hasUsageExample(text string) bool {
	if text == "" {
		return false
	}
	return core.HasPrefix(text, "Example:") || core.Contains(text, "\nExample:")
}

func testFileToken(filePath string) string {
	stem := core.TrimSuffix(path.Base(filePath), "_test.go")
	switch stem {
	case "html":
		return "HTML"
	default:
		if stem == "" {
			return ""
		}
		return core.Concat(core.Upper(stem[:1]), stem[1:])
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	writeResult := hostFS.Write(path, content)
	if !writeResult.OK {
		t.Fatalf("write %s: %v", path, resultError(writeResult))
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
