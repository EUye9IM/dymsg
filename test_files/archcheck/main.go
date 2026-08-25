// Command archcheck performs static architecture analysis on a Go package
// directory (default: ../workspace relative to this file).
//
// It parses all non-test .go files with go/parser, builds the package with
// go/types, and reports:
//   - per-function cyclomatic complexity and line span
//   - per-file line/function/export counts and package-level vars
//   - an exact file dependency matrix (via types.Info.Uses object resolution)
//   - strongly-connected components (dependency cycles) of the file graph
//
// Output is a JSON report on stdout. Standard library only; no third-party
// dependencies. Run from the eval module root: go run ./archcheck ../workspace
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type FuncInfo struct {
	Name       string `json:"name"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Lines      int    `json:"lines"`
	Complexity int    `json:"complexity"`
	Exported   bool   `json:"exported"`
}

type VarInfo struct {
	Name     string `json:"name"`
	File     string `json:"file"`
	Sentinel bool   `json:"sentinel"`
}

type FileInfo struct {
	Name    string    `json:"name"`
	Lines   int       `json:"lines"`
	Funcs   int       `json:"funcs"`
	Exports int       `json:"exports"`
	Vars    []VarInfo `json:"vars"`
}

type DepInfo struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Count int    `json:"count"`
}

type ArchReport struct {
	Files       []FileInfo `json:"files"`
	Functions   []FuncInfo `json:"functions"`
	Deps        []DepInfo  `json:"deps"`
	DepCycles   [][]string `json:"dep_cycles"`
	PackageVars []VarInfo  `json:"package_vars"`
}

func main() {
	dir := "../workspace"
	if len(os.Args) >= 2 {
		dir = os.Args[1]
	}
	report := analyze(dir)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "archcheck:", err)
	os.Exit(1)
}

func analyze(dir string) *ArchReport {
	names := []string{}
	ents, err := os.ReadDir(dir)
	if err != nil {
		fatal(err)
	}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	fset := token.NewFileSet()
	astFiles := make([]*ast.File, 0, len(names))
	for _, name := range names {
		path := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			fatal(err)
		}
		astFiles = append(astFiles, f)
	}

	conf := types.Config{
		Importer: importer.Default(),
		Error:    func(error) {},
	}
	info := &types.Info{
		Uses: map[*ast.Ident]types.Object{},
		Defs: map[*ast.Ident]types.Object{},
	}
	pkg, _ := conf.Check("dymsg", fset, astFiles, info)

	fileLines := make(map[string]int, len(astFiles))
	for i, f := range astFiles {
		fileLines[names[i]] = fset.Position(f.End()).Line
	}

	funcs := []FuncInfo{}
	for i, f := range astFiles {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			start := fset.Position(fn.Pos()).Line
			end := fset.Position(fn.End()).Line
			funcs = append(funcs, FuncInfo{
				Name:       fn.Name.Name,
				File:       names[i],
				Line:       start,
				Lines:      end - start + 1,
				Complexity: complexity(fn),
				Exported:   fn.Name.IsExported(),
			})
		}
	}

	vars := []VarInfo{}
	for i, f := range astFiles {
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				sentinel := isSentinelError(vs)
				for _, n := range vs.Names {
					vars = append(vars, VarInfo{Name: n.Name, File: names[i], Sentinel: sentinel})
				}
			}
		}
	}

	dep := make(map[string]map[string]int, len(names))
	for _, name := range names {
		dep[name] = map[string]int{}
	}
	for i, f := range astFiles {
		src := names[i]
		ast.Inspect(f, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			obj := info.Uses[id]
			if obj == nil || obj.Pkg() != pkg || !obj.Pos().IsValid() {
				return true
			}
			defFile := filepath.Base(fset.Position(obj.Pos()).Filename)
			if defFile == src {
				return true
			}
			if _, ok := dep[defFile]; ok {
				dep[src][defFile]++
			}
			return true
		})
	}

	depList := []DepInfo{}
	for _, from := range names {
		tos := dep[from]
		keys := make([]string, 0, len(tos))
		for to := range tos {
			keys = append(keys, to)
		}
		sort.Strings(keys)
		for _, to := range keys {
			depList = append(depList, DepInfo{From: from, To: to, Count: tos[to]})
		}
	}

	edges := make(map[string]map[string]bool, len(names))
	for _, from := range names {
		edges[from] = map[string]bool{}
		for to, c := range dep[from] {
			if c > 0 {
				edges[from][to] = true
			}
		}
	}
	comps := scc(names, edges)
	cycles := [][]string{}
	for _, comp := range comps {
		if len(comp) > 1 {
			cycles = append(cycles, comp)
		}
	}

	filesInfo := make([]FileInfo, 0, len(names))
	for _, name := range names {
		nf, ne := 0, 0
		for _, fn := range funcs {
			if fn.File != name {
				continue
			}
			nf++
			if fn.Exported {
				ne++
			}
		}
		var fv []VarInfo
		for _, v := range vars {
			if v.File == name {
				fv = append(fv, v)
			}
		}
		filesInfo = append(filesInfo, FileInfo{
			Name: name, Lines: fileLines[name], Funcs: nf, Exports: ne, Vars: fv,
		})
	}

	return &ArchReport{
		Files:       filesInfo,
		Functions:   funcs,
		Deps:        depList,
		DepCycles:   cycles,
		PackageVars: vars,
	}
}

func complexity(fn *ast.FuncDecl) int {
	n := 1
	ast.Inspect(fn, func(n2 ast.Node) bool {
		switch n2.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt,
			*ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt,
			*ast.CaseClause:
			n++
		}
		return true
	})
	return n
}

func isSentinelError(vs *ast.ValueSpec) bool {
	for _, v := range vs.Values {
		call, ok := v.(*ast.CallExpr)
		if !ok {
			continue
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		if x, ok := sel.X.(*ast.Ident); ok && x.Name == "errors" && sel.Sel.Name == "New" {
			return true
		}
	}
	return false
}

func scc(nodes []string, edges map[string]map[string]bool) [][]string {
	index := 0
	indices := map[string]int{}
	low := map[string]int{}
	onStack := map[string]bool{}
	stack := []string{}
	result := [][]string{}

	var strongconnect func(v string)
	strongconnect = func(v string) {
		indices[v] = index
		low[v] = index
		index++
		stack = append(stack, v)
		onStack[v] = true
		for w := range edges[v] {
			if _, ok := indices[w]; !ok {
				strongconnect(w)
				if low[w] < low[v] {
					low[v] = low[w]
				}
			} else if onStack[w] {
				if indices[w] < low[v] {
					low[v] = indices[w]
				}
			}
		}
		if low[v] == indices[v] {
			comp := []string{}
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				comp = append(comp, w)
				if w == v {
					break
				}
			}
			result = append(result, comp)
		}
	}

	for _, n := range nodes {
		if _, ok := indices[n]; !ok {
			strongconnect(n)
		}
	}
	return result
}
