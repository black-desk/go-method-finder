package finder

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"path"
	"strings"
	"sync"

	. "github.com/black-desk/go-method-finder/types"
	. "github.com/black-desk/go-method-finder/utils"
)

type Finder struct {
	structMemoryLock    sync.Mutex
	structMethodsMemory map[string]Methods
	structBasesMemory   map[string]([]string)

	packageVisitedLock   sync.Mutex
	packageVisitedMemory map[string]struct{}

	packageImportWithOutPrefixLock   sync.Mutex
	packageImportWithOutPrefixMemory map[string]ImportInfos

	workerCnt int
	wg        sync.WaitGroup

	limit int

	dirty bool
}

func NewFinder(limit int) *Finder {
	return &Finder{
		structMemoryLock:    sync.Mutex{},
		structMethodsMemory: map[string]Methods{},
		structBasesMemory:   map[string][]string{},

		packageVisitedLock:   sync.Mutex{},
		packageVisitedMemory: map[string]struct{}{},

		packageImportWithOutPrefixLock:   sync.Mutex{},
		packageImportWithOutPrefixMemory: map[string]ImportInfos{},

		workerCnt: 0,
		wg:        sync.WaitGroup{},

		dirty: false,
		limit: limit,
	}
}

func (f *Finder) Find(pathToPackage string, structNames []string) Result {
	if f.dirty {
		log.Fatal("You are trying to use a dirty finder, which is not allowed!")
	}

	f.walkDir(pathToPackage)
	f.wg.Wait()

	return f.genResult(pathToPackage, structNames)
}

func (f *Finder) walkDir(pathToPackage string) {
	if f.workerCnt <= f.limit {
		f.wg.Add(1)
		go f.doWalkDir(pathToPackage, true)
	} else {
		f.doWalkDir(pathToPackage, false)
	}
}

func (f *Finder) doWalkDir(pathToPackage string, flag bool) {
	if flag {
		f.workerCnt++
		defer f.wg.Done()
	}

	f.packageVisitedLock.Lock()
	if _, ok := f.packageVisitedMemory[pathToPackage]; ok {
		return
	}
	f.packageVisitedMemory[pathToPackage] = struct{}{}
	f.packageImportWithOutPrefixMemory[pathToPackage] = ImportInfos{}
	f.packageVisitedLock.Unlock()

	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, pathToPackage, nil, parser.AllErrors|parser.ParseComments)

	if err != nil {
		panic(fmt.Sprintf("Unable to parse package in %s", pathToPackage))
	}
	var targetPkg *ast.Package
	for _, pkg := range pkgs {
		if !strings.HasSuffix(pkg.Name, "_test") {
			targetPkg = pkg
			break
		}
	}
	if targetPkg == nil {
		log.Fatalf("Unable to find package in %s", pathToPackage)
	}

	for _, file := range targetPkg.Files {
		f.walkFile(pathToPackage, file)
	}
}

func (f *Finder) walkFile(pathToPackage string, file *ast.File) {
	if f.workerCnt <= f.limit {
		f.wg.Add(1)
		go f.doWalkFile(pathToPackage, file, true)
	} else {
		f.doWalkFile(pathToPackage, file, false)
	}
}

func (f *Finder) doWalkFile(pathToPackage string, file *ast.File, flag bool) {
	if flag {
		f.workerCnt++
		defer f.wg.Done()
	}

	importInfos := map[string]string{} // package local name -> abs path

	f.packageImportWithOutPrefixLock.Lock()
	f.packageImportWithOutPrefixMemory[pathToPackage] = []string{}
	f.packageImportWithOutPrefixLock.Unlock()

	imports := file.Imports
	for _, importSpec := range imports {
		raw := importSpec.Path.Value
		pathToNewPackage := ResolvePackagePath(string(raw[1 : len(raw)-1]))
		packageLocalName := importSpec.Name.String()
		if packageLocalName == "<nil>" {
			packageLocalName = path.Base(importSpec.Path.Value)
		} else if packageLocalName == "." {
			f.packageImportWithOutPrefixLock.Lock()
			f.packageImportWithOutPrefixMemory[pathToPackage] = append(f.packageImportWithOutPrefixMemory[pathToPackage], pathToNewPackage)
			f.packageImportWithOutPrefixLock.Unlock()
			f.walkDir(pathToNewPackage)
		}
		if strings.HasSuffix(packageLocalName, "\"") {
			packageLocalName = packageLocalName[:len(packageLocalName)-1]
		}
		importInfos[packageLocalName] = pathToNewPackage
	}
	ast.Inspect(file, func(n ast.Node) bool {
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			name := pathToPackage + ":" + typeSpec.Name.Name
			f.structMemoryLock.Lock()
			if _, ok := f.structMethodsMemory[name]; !ok {
				f.structMethodsMemory[name] = Methods{}
			}
			f.structMemoryLock.Unlock()

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				return false
			}
			for _, field := range structType.Fields.List {
				if field.Names == nil {
					var embeddedStructID string
					if embeddedStruct, ok := field.Type.(*ast.Ident); ok {
						embeddedStructName := embeddedStruct.Name
						embeddedStructID = pathToPackage + ":" + embeddedStructName
					} else if selectorExpr, ok := field.Type.(*ast.SelectorExpr); ok {
						pathToNewPackage := importInfos[selectorExpr.X.(*ast.Ident).Name]
						embeddedStructID = pathToNewPackage + ":" + selectorExpr.Sel.Name
						f.walkDir(pathToNewPackage)
					}
					f.structMemoryLock.Lock()
					f.structBasesMemory[name] = append(f.structBasesMemory[name], embeddedStructID)
					f.structMemoryLock.Unlock()
				}
			}
		}
		if fun, ok := n.(*ast.FuncDecl); ok {
			if fun.Name.IsExported() && fun.Recv != nil && len(fun.Recv.List) == 1 {
				if r, ok := fun.Recv.List[0].Type.(*ast.StarExpr); ok {
					structName := r.X.(*ast.Ident).Name
					key := pathToPackage + ":" + structName
					f.structMemoryLock.Lock()
					if _, ok := f.structMethodsMemory[key]; !ok {
						f.structMethodsMemory[key] = Methods{}
					}
					f.structMethodsMemory[key] = append(f.structMethodsMemory[key], fun)
					f.structMemoryLock.Unlock()
				}
			}
		}
		return true
	})
}

func (f *Finder) dfs(structID string) Methods {

	methods, ok := f.structMethodsMemory[structID]
	if !ok {
		return nil
	}

	for _, embeddedStructID := range f.structBasesMemory[structID] {
		if _, ok := f.structMethodsMemory[embeddedStructID]; ok {
			methods = append(methods, f.dfs(embeddedStructID)...)
		} else {
			name := strings.Split(embeddedStructID, ":")[1]
			for _, pkg := range f.packageImportWithOutPrefixMemory[strings.Split(structID, ":")[0]] {
				methods = append(methods, f.dfs(pkg+":"+name)...)
			}
		}
	}
	return methods
}

func (f *Finder) genResult(pathToPackage string, structNames []string) Result {
	result := Result{}
	for _, structName := range structNames {
		result[structName] = f.dfs(pathToPackage + ":" + structName)
	}

	return result
}
