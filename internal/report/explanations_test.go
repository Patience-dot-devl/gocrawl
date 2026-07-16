package report

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestExplanationsComplete guards against half-filled entries: every registered
// explanation must populate all three fields so the HTML report never shows a
// blank What/Impact/Fix row.
func TestExplanationsComplete(t *testing.T) {
	for code, e := range explanations {
		if e.What == "" {
			t.Errorf("explanation %q has empty What", code)
		}
		if e.Impact == "" {
			t.Errorf("explanation %q has empty Impact", code)
		}
		if e.Fix == "" {
			t.Errorf("explanation %q has empty Fix", code)
		}
	}
}

func TestExplainUnknownCode(t *testing.T) {
	if got := explain("no-such-code-exists"); got != nil {
		t.Errorf("explain(unknown) = %v, want nil", got)
	}
}

func TestExplainKnownCode(t *testing.T) {
	got := explain("seo-missing-title")
	if got == nil {
		t.Fatal("explain(missing-title) = nil, want explanation")
		return
	}
	if got.Fix == "" {
		t.Error("expected a non-empty Fix for missing-title")
	}
}

// TestAllAnalyzerCodesHaveExplanations statically scans every analyzer package's source for
// issue codes it emits (as string literals, either in an analyze.Issue{Code: "..."}
// composite literal or as the code argument to an add(analyze.Error|Warning|Info, "code", ...)
// closure) and asserts each one has an explanations entry. This is a contract test, not a
// manually-maintained list, so a new analyzer code added without an explanation fails here
// instead of silently shipping a blank What/Impact/Fix block in the HTML report — exactly the
// gap that let all 18 datalayer-* codes go unexplained.
//
// Limitation: a code built dynamically (fmt.Sprintf, a variable) rather than as a string
// literal is invisible to this scan. Every analyzer's codes are literals today; a future
// analyzer that builds codes dynamically would need a different check.
func TestAllAnalyzerCodesHaveExplanations(t *testing.T) {
	codes, err := scanAnalyzerCodes("../analyze")
	if err != nil {
		t.Fatalf("scanning analyzer codes: %v", err)
	}
	if len(codes) == 0 {
		t.Fatal("scan found zero issue codes; the scanner is likely broken")
	}

	var missing []string
	for c := range codes {
		if _, ok := explanations[c]; !ok {
			missing = append(missing, c)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("issue codes emitted by an analyzer with no explanations entry: %v", missing)
	}
}

// scanAnalyzerCodes walks every non-test .go file under root and collects string-literal issue
// codes from analyze.Issue{Code: "..."} composite literals and from add(analyze.Error|Warning|
// Info, "code", ...)-style calls.
func scanAnalyzerCodes(root string) (map[string]bool, error) {
	codes := make(map[string]bool)
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.CompositeLit:
				if !isAnalyzeIssueType(v.Type) {
					return true
				}
				for _, elt := range v.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					key, ok := kv.Key.(*ast.Ident)
					if !ok || key.Name != "Code" {
						continue
					}
					if code, ok := stringLiteral(kv.Value); ok {
						codes[code] = true
					}
				}
			case *ast.CallExpr:
				// add()-style closures vary in argument order across analyzers (some take
				// (severity, code, ...), others (url, severity, code, ...)), so look for a
				// string literal immediately following a severity argument at any position
				// rather than assuming severity is always first.
				for i, arg := range v.Args {
					if i+1 >= len(v.Args) || !isAnalyzeSeverity(arg) {
						continue
					}
					if code, ok := stringLiteral(v.Args[i+1]); ok {
						codes[code] = true
					}
				}
			}
			return true
		})
		return nil
	})
	return codes, err
}

// isAnalyzeIssueType reports whether typ is the selector expression analyze.Issue.
func isAnalyzeIssueType(typ ast.Expr) bool {
	sel, ok := typ.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Issue" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "analyze"
}

// isAnalyzeSeverity reports whether expr is one of analyze.Error, analyze.Warning, or
// analyze.Info.
func isAnalyzeSeverity(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok || id.Name != "analyze" {
		return false
	}
	switch sel.Sel.Name {
	case "Error", "Warning", "Info":
		return true
	}
	return false
}

func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}
