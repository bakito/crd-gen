package main

import (
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var (
		inputFile  = flag.String("input", "", "Input .go file (default: stdin)")
		outputFile = flag.String("output", "", "Output .go file (default: <input>_compare-gen.go or stdout)")
		typeFilter = flag.String("type", "", "Comma-separated list of struct names to generate for (default: all)")
		header     = flag.Bool("header", true, "Emit a 'Code generated' header comment")
	)
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, `compare-gen - Generate DeepEqual and DeepCompare methods for Go structs

Usage:
  compare-gen [flags]

Flags:
`)
		flag.PrintDefaults()
		fmt.Fprint(os.Stderr, `
Examples:
  compare-gen -input types.go
  compare-gen -input types.go -output zz_compare-gen.go
  compare-gen -input types.go -type Person,Address
  cat types.go | compare-gen
`)
	}
	flag.Parse()

	var src []byte
	var err error

	if *inputFile != "" {
		src, err = os.ReadFile(*inputFile)
		if err != nil {
			fatalf("reading input: %v", err)
		}
	} else {
		src, err = readStdin()
		if err != nil {
			fatalf("reading stdin: %v", err)
		}
	}

	// Parse
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		fatalf("parsing Go source: %v", err)
	}

	// Build a full index of all structs in the file
	allByName := collectAllStructs(file)

	// Collect the explicit filter set
	filter := map[string]bool{}
	if *typeFilter != "" {
		for t := range strings.SplitSeq(*typeFilter, ",") {
			filter[strings.TrimSpace(t)] = true
		}
	}

	// If a filter is set, expand it transitively so every nested struct
	// that lives in this file also gets methods generated.
	if len(filter) > 0 {
		expandDeps(filter, allByName)
	}

	structs := collectStructs(file, filter)
	if len(structs) == 0 {
		fatalf("no matching structs found in input")
	}

	// Generate
	g := &Generator{
		pkg:        file.Name.Name,
		addHeader:  *header,
		inputFile:  *inputFile,
		allStructs: allStructNames(file),
	}
	output := g.generate(structs)

	// Format
	formatted, err := format.Source([]byte(output))
	if err != nil {
		// Emit unformatted with error so user can debug
		fmt.Fprintf(os.Stderr, "warning: gofmt failed (%v), writing unformatted output\n", err)
		formatted = []byte(output)
	}

	// Write output
	outPath := *outputFile
	if outPath == "" && *inputFile != "" {
		ext := filepath.Ext(*inputFile)
		outPath = strings.TrimSuffix(*inputFile, ext) + "_compare-gen.go"
	}

	if outPath != "" {
		if err := os.WriteFile(outPath, formatted, 0o644); err != nil {
			fatalf("writing output: %v", err)
		}
		fmt.Fprintf(os.Stderr, "compare-gen: wrote %s (%d bytes, %d struct(s))\n", outPath, len(formatted), len(structs))
	} else {
		os.Stdout.Write(formatted)
	}
}

func fatalf(f string, args ...any) {
	fmt.Fprintf(os.Stderr, "compare-gen: "+f+"\n", args...)
	os.Exit(1)
}

func readStdin() ([]byte, error) {
	info, err := os.Stdin.Stat()
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return nil, errors.New("no input file specified and stdin is a terminal")
	}
	return os.ReadFile("/dev/stdin")
}

// StructInfo holds parsed struct metadata.
type StructInfo struct {
	Name   string
	Fields []FieldInfo
}

type FieldInfo struct {
	Name string
	Type ast.Expr
}

// collectAllStructs returns a map of struct name -> StructInfo for every
// struct defined in the file. Used for dependency resolution.
func collectAllStructs(file *ast.File) map[string]StructInfo {
	result := map[string]StructInfo{}
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			info := StructInfo{Name: ts.Name.Name}
			for _, field := range st.Fields.List {
				for _, name := range field.Names {
					if name.IsExported() {
						info.Fields = append(info.Fields, FieldInfo{Name: name.Name, Type: field.Type})
					}
				}
				if len(field.Names) == 0 {
					if ident, ok := field.Type.(*ast.Ident); ok && ast.IsExported(ident.Name) {
						info.Fields = append(info.Fields, FieldInfo{Name: ident.Name, Type: field.Type})
					}
				}
			}
			result[ts.Name.Name] = info
		}
	}
	return result
}

// expandDeps does a BFS/DFS from every name in `want`, adding any struct
// types referenced in fields that are also defined in allByName.
func expandDeps(want map[string]bool, allByName map[string]StructInfo) {
	queue := make([]string, 0, len(want))
	for name := range want {
		queue = append(queue, name)
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		info, ok := allByName[name]
		if !ok {
			continue
		}
		for _, f := range info.Fields {
			deps := structDepsOf(f.Type)
			for _, dep := range deps {
				if _, inFile := allByName[dep]; inFile && !want[dep] {
					want[dep] = true
					queue = append(queue, dep)
				}
			}
		}
	}
}

// structDepsOf returns struct type names referenced by an ast.Expr.
// It unwraps pointers, slices, arrays, and maps recursively.
func structDepsOf(expr ast.Expr) []string {
	switch t := expr.(type) {
	case *ast.Ident:
		if ast.IsExported(t.Name) {
			return []string{t.Name}
		}
	case *ast.StarExpr:
		return structDepsOf(t.X)
	case *ast.ArrayType:
		return structDepsOf(t.Elt)
	case *ast.MapType:
		return append(structDepsOf(t.Key), structDepsOf(t.Value)...)
	}
	return nil
}

func collectStructs(file *ast.File, filter map[string]bool) []StructInfo {
	var result []StructInfo
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			if len(filter) > 0 && !filter[ts.Name.Name] {
				continue
			}
			info := StructInfo{Name: ts.Name.Name}
			for _, field := range st.Fields.List {
				for _, name := range field.Names {
					if name.IsExported() {
						info.Fields = append(info.Fields, FieldInfo{
							Name: name.Name,
							Type: field.Type,
						})
					}
				}
				// Embedded exported struct (no field name)
				if len(field.Names) == 0 {
					if ident, ok := field.Type.(*ast.Ident); ok && ast.IsExported(ident.Name) {
						info.Fields = append(info.Fields, FieldInfo{
							Name: ident.Name,
							Type: field.Type,
						})
					}
				}
			}
			result = append(result, info)
		}
	}
	return result
}

func allStructNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if _, ok := ts.Type.(*ast.StructType); ok {
				names[ts.Name.Name] = true
			}
		}
	}
	return names
}
