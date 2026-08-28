// Package code holds the in-process analysers. These never leave the binary:
// everything about the repository is remote, and everything that reasons about
// the code is local.
package code

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/giri-ms19/testplan-agent/internal/model"
	"github.com/giri-ms19/testplan-agent/internal/tool"
	"github.com/giri-ms19/testplan-agent/internal/tool/repo"
)

// FileAnalysis is what a parser extracts from one source file.
type FileAnalysis struct {
	Path            string              `json:"path"`
	Language        model.Language      `json:"language"`
	Depth           model.AnalysisDepth `json:"depth"`
	PackageName     string              `json:"packageName,omitempty"`
	Imports         []string            `json:"imports,omitempty"`
	Symbols         []model.Symbol      `json:"symbols"`
	ComplexityScore int                 `json:"complexityScore"`
}

// ParseGoTool extracts structure from Go source using the standard library's
// own parser, which is the reason Go analysis is deeper than any tree-sitter
// path could be.
type ParseGoTool struct {
	Source repo.Source
}

func (parseTool *ParseGoTool) Name() string { return "code.parse_go" }

func (parseTool *ParseGoTool) Description() string {
	return "Parse a Go file and return its package, imports, exported symbols with signatures, " +
		"error-returning functions, and a branch-complexity score. Use this instead of reading " +
		"a Go file line by line."
}

func (parseTool *ParseGoTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type":"object",
  "properties":{"path":{"type":"string","description":"Path to a .go file, as listed by repo.tree"}},
  "required":["path"],
  "additionalProperties":false
}`)
}

func (parseTool *ParseGoTool) Idempotent() bool { return true }

func (parseTool *ParseGoTool) Invoke(ctx context.Context, arguments json.RawMessage) (tool.Result, error) {
	var parsedArguments struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(arguments, &parsedArguments); err != nil || parsedArguments.Path == "" {
		return tool.Result{}, tool.Correctable(parseTool.Name(),
			"arguments must be a JSON object with a required string field \"path\"")
	}
	if !strings.HasSuffix(parsedArguments.Path, ".go") {
		return tool.Result{}, tool.Correctable(parseTool.Name(), fmt.Sprintf(
			"%q is not a Go file; use code.parse_python for .py files", parsedArguments.Path))
	}

	sourceText, err := parseTool.Source.ReadFile(ctx, parsedArguments.Path)
	if err != nil {
		return tool.Result{}, tool.Correctable(parseTool.Name(), fmt.Sprintf(
			"could not read %q; call repo.tree to list valid paths", parsedArguments.Path))
	}

	analysis, err := AnalyseGoSource(parsedArguments.Path, sourceText)
	if err != nil {
		// A syntax error is the repository's problem, not the model's, and it
		// should not stop the run: record it and move on.
		return tool.Result{}, tool.NewFailure(tool.FailureDegradable, parseTool.Name(),
			fmt.Sprintf("%q did not parse as Go (%v); it will be skipped", parsedArguments.Path, err), err)
	}
	return tool.JSON(analysis, tool.Internal())
}

// AnalyseGoSource parses one Go file. Exported so the Surveyor can call it
// directly without going through the tool layer, and so tests can assert on it.
func AnalyseGoSource(path, sourceText string) (FileAnalysis, error) {
	fileSet := token.NewFileSet()
	parsedFile, err := parser.ParseFile(fileSet, path, sourceText, parser.ParseComments)
	if err != nil {
		return FileAnalysis{}, err
	}

	analysis := FileAnalysis{
		Path:        path,
		Language:    model.LanguageGo,
		Depth:       model.DepthTypeResolved,
		PackageName: parsedFile.Name.Name,
	}
	for _, importSpec := range parsedFile.Imports {
		analysis.Imports = append(analysis.Imports, strings.Trim(importSpec.Path.Value, `"`))
	}

	for _, declaration := range parsedFile.Decls {
		switch typedDeclaration := declaration.(type) {
		case *ast.FuncDecl:
			analysis.Symbols = append(analysis.Symbols, symbolFromFunc(fileSet, path, typedDeclaration))
		case *ast.GenDecl:
			analysis.Symbols = append(analysis.Symbols, symbolsFromGenDecl(fileSet, path, typedDeclaration)...)
		}
	}
	for _, symbol := range analysis.Symbols {
		analysis.ComplexityScore += symbol.BranchCount
	}
	return analysis, nil
}

func symbolFromFunc(fileSet *token.FileSet, path string, functionDeclaration *ast.FuncDecl) model.Symbol {
	symbol := model.Symbol{
		Name:     functionDeclaration.Name.Name,
		Kind:     "func",
		Exported: functionDeclaration.Name.IsExported(),
		Ref: model.SourceRef{
			Path:      path,
			Symbol:    functionDeclaration.Name.Name,
			StartLine: fileSet.Position(functionDeclaration.Pos()).Line,
			EndLine:   fileSet.Position(functionDeclaration.End()).Line,
		},
	}
	if functionDeclaration.Doc != nil {
		symbol.DocComment = strings.TrimSpace(functionDeclaration.Doc.Text())
	}
	if functionDeclaration.Recv != nil && len(functionDeclaration.Recv.List) > 0 {
		symbol.Kind = "method"
		symbol.Receiver = renderExpression(functionDeclaration.Recv.List[0].Type)
		symbol.Ref.Symbol = symbol.Receiver + "." + symbol.Name
	}
	symbol.Signature = renderSignature(functionDeclaration)
	symbol.ParameterCount = countFields(functionDeclaration.Type.Params)
	symbol.ReturnsError = signatureReturnsError(functionDeclaration.Type.Results)
	symbol.BranchCount = countBranches(functionDeclaration.Body)
	return symbol
}

func symbolsFromGenDecl(fileSet *token.FileSet, path string, genericDeclaration *ast.GenDecl) []model.Symbol {
	symbols := []model.Symbol{}
	for _, specification := range genericDeclaration.Specs {
		typeSpecification, isTypeSpec := specification.(*ast.TypeSpec)
		if !isTypeSpec {
			continue
		}
		symbol := model.Symbol{
			Name:     typeSpecification.Name.Name,
			Kind:     "type",
			Exported: typeSpecification.Name.IsExported(),
			Ref: model.SourceRef{
				Path:      path,
				Symbol:    typeSpecification.Name.Name,
				StartLine: fileSet.Position(typeSpecification.Pos()).Line,
				EndLine:   fileSet.Position(typeSpecification.End()).Line,
			},
		}
		if genericDeclaration.Doc != nil {
			symbol.DocComment = strings.TrimSpace(genericDeclaration.Doc.Text())
		}
		if _, isInterface := typeSpecification.Type.(*ast.InterfaceType); isInterface {
			symbol.Kind = "interface"
		}
		symbols = append(symbols, symbol)
	}
	return symbols
}

// countBranches is a cheap cyclomatic-complexity proxy. It is deliberately
// deterministic and model-free: risk scoring must be reproducible across runs,
// or a re-run silently reprioritises the whole plan.
func countBranches(functionBody *ast.BlockStmt) int {
	if functionBody == nil {
		return 0
	}
	branchCount := 1
	ast.Inspect(functionBody, func(node ast.Node) bool {
		switch typedNode := node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
			branchCount++
		case *ast.BinaryExpr:
			if typedNode.Op == token.LAND || typedNode.Op == token.LOR {
				branchCount++
			}
		}
		return true
	})
	return branchCount
}

func signatureReturnsError(results *ast.FieldList) bool {
	if results == nil {
		return false
	}
	for _, resultField := range results.List {
		if renderExpression(resultField.Type) == "error" {
			return true
		}
	}
	return false
}

func countFields(fieldList *ast.FieldList) int {
	if fieldList == nil {
		return 0
	}
	fieldCount := 0
	for _, field := range fieldList.List {
		if len(field.Names) == 0 {
			fieldCount++
			continue
		}
		fieldCount += len(field.Names)
	}
	return fieldCount
}

func renderSignature(functionDeclaration *ast.FuncDecl) string {
	var signatureBuilder strings.Builder
	signatureBuilder.WriteString("func ")
	if functionDeclaration.Recv != nil && len(functionDeclaration.Recv.List) > 0 {
		fmt.Fprintf(&signatureBuilder, "(%s) ", renderExpression(functionDeclaration.Recv.List[0].Type))
	}
	signatureBuilder.WriteString(functionDeclaration.Name.Name)
	signatureBuilder.WriteString("(")
	signatureBuilder.WriteString(renderFieldList(functionDeclaration.Type.Params))
	signatureBuilder.WriteString(")")
	if results := renderFieldList(functionDeclaration.Type.Results); results != "" {
		fmt.Fprintf(&signatureBuilder, " (%s)", results)
	}
	return signatureBuilder.String()
}

func renderFieldList(fieldList *ast.FieldList) string {
	if fieldList == nil || len(fieldList.List) == 0 {
		return ""
	}
	renderedFields := make([]string, 0, len(fieldList.List))
	for _, field := range fieldList.List {
		renderedType := renderExpression(field.Type)
		if len(field.Names) == 0 {
			renderedFields = append(renderedFields, renderedType)
			continue
		}
		names := make([]string, 0, len(field.Names))
		for _, fieldName := range field.Names {
			names = append(names, fieldName.Name)
		}
		renderedFields = append(renderedFields, strings.Join(names, ", ")+" "+renderedType)
	}
	return strings.Join(renderedFields, ", ")
}

func renderExpression(expression ast.Expr) string {
	switch typedExpression := expression.(type) {
	case *ast.Ident:
		return typedExpression.Name
	case *ast.StarExpr:
		return "*" + renderExpression(typedExpression.X)
	case *ast.SelectorExpr:
		return renderExpression(typedExpression.X) + "." + typedExpression.Sel.Name
	case *ast.ArrayType:
		return "[]" + renderExpression(typedExpression.Elt)
	case *ast.MapType:
		return "map[" + renderExpression(typedExpression.Key) + "]" + renderExpression(typedExpression.Value)
	case *ast.Ellipsis:
		return "..." + renderExpression(typedExpression.Elt)
	case *ast.FuncType:
		return "func(" + renderFieldList(typedExpression.Params) + ")"
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.ChanType:
		return "chan " + renderExpression(typedExpression.Value)
	default:
		return "expr"
	}
}
