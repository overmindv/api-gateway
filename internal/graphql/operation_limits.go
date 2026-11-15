package graphql

import (
	"context"

	gqlgen "github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// operationLimits сдерживает расширение fragments до рекурсивного расчёта complexity.
type operationLimits struct{}

func (operationLimits) ExtensionName() string                  { return "OperationLimits" }
func (operationLimits) Validate(gqlgen.ExecutableSchema) error { return nil }
func (operationLimits) MutateOperationContext(_ context.Context, op *gqlgen.OperationContext) *gqlerror.Error {
	fields, aliases, roots := 0, 0, 0
	var walk func(ast.SelectionSet, int) bool
	walk = func(set ast.SelectionSet, depth int) bool {
		if depth > 16 {
			return false
		}
		for _, selection := range set {
			fields++
			if fields > 1000 {
				return false
			}
			switch item := selection.(type) {
			case *ast.Field:
				if depth == 0 {
					roots++
				}
				if item.Alias != "" && item.Alias != item.Name {
					aliases++
				}
				if roots > 20 || aliases > 100 || !walk(item.SelectionSet, depth+1) {
					return false
				}
			case *ast.FragmentSpread:
				if !walk(item.Definition.SelectionSet, depth) {
					return false
				}
			case *ast.InlineFragment:
				if !walk(item.SelectionSet, depth) {
					return false
				}
			}
		}
		return true
	}
	if !walk(op.Operation.SelectionSet, 0) {
		return &gqlerror.Error{Message: "Не удалось выполнить действие.", Extensions: map[string]any{"code": "OPERATION_LIMIT_EXCEEDED"}}
	}
	return nil
}
