package types

import (
	"go/ast"
)

type Methods []*ast.FuncDecl

type Result map[string]Methods

type ImportInfos []string
