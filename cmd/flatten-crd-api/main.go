package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/printer"
	"go/token"
	"log"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/tools/go/packages"
)

var (
	srcPkg    = flag.String("src", "", "Source package path or file")
	typeNames = flag.String("type", "", "Comma-separated list of struct names to extract")
	outPath   = flag.String("out", "", "Output file path")
	outPkg    = flag.String("pkg", "generated", "Output package name")
	pointers  = flag.Bool("pointers", false, "Generate all struct variables as pointers")
)

// Allowed packages that we don't flatten.
var allowedPkgs = map[string]string{
	"k8s.io/apimachinery/pkg/apis/meta/v1": "metav1",
	"time":                                 "time",
	"encoding/json":                        "json",
}

type Extractor struct {
	pkgs           map[string]*packages.Package
	processed      map[string]string // Type ID -> Local Name
	usedLocalNames map[string]bool   // Local Name -> Used
	queue          []TypeRequest
	imports        map[string]string // Import Path -> Local Name
	localDecls     []ast.Decl
	rootPkg        string
	pointers       bool
}

type TypeRequest struct {
	pkg  *packages.Package
	name string
}

func main() {
	flag.Parse()

	if *srcPkg == "" || *typeNames == "" || *outPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
	}

	// Try loading the package normally first
	pkgs, err := packages.Load(cfg, *srcPkg)

	// If it fails because the module is missing, try fetching it in a temp directory
	if err != nil || len(pkgs) == 0 || pkgs[0].Name == "" ||
		(len(pkgs) > 0 && len(pkgs[0].Errors) > 0 && strings.Contains(pkgs[0].Errors[0].Msg, "no required module provides")) {
		log.Printf("Package not found in current module. Attempting to fetch %s in a temporary workspace...", *srcPkg)
		tempDir, err := os.MkdirTemp("", "crd-extractor-*")
		if err != nil {
			log.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		ctx := context.TODO()

		// Initialize a temporary module
		runCmd(ctx, tempDir, "go", "mod", "init", "temp-extract")
		runCmd(ctx, tempDir, "go", "get", *srcPkg)

		cfg.Dir = tempDir
		pkgs, err = packages.Load(cfg, *srcPkg)
		if err != nil {
			_ = os.RemoveAll(tempDir)
			//nolint:gocritic // RemoveAll will also be called before log fatal
			log.Fatalf("Failed to load package after fetch: %v", err)
		}
	}

	if packages.PrintErrors(pkgs) > 0 {
		os.Exit(1)
	}

	ex := &Extractor{
		pkgs:           make(map[string]*packages.Package),
		processed:      make(map[string]string),
		usedLocalNames: make(map[string]bool),
		imports:        make(map[string]string),
		pointers:       *pointers,
	}

	packages.Visit(pkgs, nil, func(p *packages.Package) {
		ex.pkgs[p.PkgPath] = p
	})

	rootPkg := pkgs[0]
	ex.rootPkg = rootPkg.PkgPath
	typesToExtract := strings.Split(*typeNames, ",")

	for _, t := range typesToExtract {
		name := strings.TrimSpace(t)
		ex.enqueue(rootPkg, name)

		// Automatically try to extract the List type if it exists
		listName := name + "List"
		if rootPkg.Types.Scope().Lookup(listName) != nil {
			ex.enqueue(rootPkg, listName)
		}
	}

	ex.process()
	ex.generate()
}

func runCmd(ctx context.Context, dir, name string, args ...string) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("Command failed: %s %v\nOutput: %s", name, args, string(out))
	}
}

func (ex *Extractor) enqueue(pkg *packages.Package, name string) string {
	key := typeKey(pkg.PkgPath, name)
	if ln, ok := ex.processed[key]; ok {
		return ln
	}

	localName := name
	if ex.usedLocalNames[localName] {
		// Collision! Try to disambiguate.
		pkgName := pkg.Name
		if pkgName == "" {
			pkgName = "Pkg"
		}
		// ensure first letter is upper
		pkgName = strings.ToUpper(pkgName[:1]) + pkgName[1:]

		candidate := pkgName + name
		if ex.usedLocalNames[candidate] {
			// fallback loop
			i := 2
			for {
				c := fmt.Sprintf("%s%d", candidate, i)
				if !ex.usedLocalNames[c] {
					candidate = c
					break
				}
				i++
			}
		}
		localName = candidate
	}

	ex.usedLocalNames[localName] = true
	ex.processed[key] = localName
	ex.queue = append(ex.queue, TypeRequest{pkg: pkg, name: name})
	return localName
}

func (ex *Extractor) process() {
	for len(ex.queue) > 0 {
		req := ex.queue[0]
		ex.queue = ex.queue[1:]
		ex.extractType(req.pkg, req.name)
	}
}

func (ex *Extractor) extractType(pkg *packages.Package, name string) {
	var typeSpec *ast.TypeSpec
	var parentDecl *ast.GenDecl

	for _, f := range pkg.Syntax {
		for _, decl := range f.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if ok && ts.Name.Name == name {
					typeSpec = ts
					parentDecl = genDecl
					break
				}
			}
			if typeSpec != nil {
				break
			}
		}
		if typeSpec != nil {
			break
		}
	}

	if typeSpec == nil {
		log.Printf("Warning: type %s not found in package %s", name, pkg.PkgPath)
		return
	}

	localName := ex.processed[typeKey(pkg.PkgPath, name)]
	newType := ex.rewriteType(pkg, typeSpec.Type)

	doc := copyDoc(parentDecl.Doc)

	// Check if it's a root object (contains TypeMeta)
	isRoot := false
	if st, ok := typeSpec.Type.(*ast.StructType); ok {
		for _, f := range st.Fields.List {
			if sel, ok := f.Type.(*ast.SelectorExpr); ok {
				if x, ok := sel.X.(*ast.Ident); ok && x.Name == "metav1" && sel.Sel.Name == "TypeMeta" {
					isRoot = true
					break
				}
			}
		}
	}

	if isRoot || strings.HasSuffix(name, "List") {
		if doc == nil {
			doc = &ast.CommentGroup{}
		}
		found := false
		for _, c := range doc.List {
			if strings.Contains(c.Text, "+kubebuilder:object:root=true") {
				found = true
				break
			}
		}
		if !found {
			doc.List = append([]*ast.Comment{{Text: "// +kubebuilder:object:root=true"}}, doc.List...)
		}
	}

	newDecl := &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{
			&ast.TypeSpec{
				Name: ast.NewIdent(localName),
				Type: newType,
			},
		},
		Doc: doc,
	}

	ex.localDecls = append(ex.localDecls, newDecl)
}

func copyDoc(doc *ast.CommentGroup) *ast.CommentGroup {
	if doc == nil {
		return nil
	}
	newDoc := &ast.CommentGroup{}
	for _, c := range doc.List {
		newDoc.List = append(newDoc.List, &ast.Comment{
			Text:  c.Text,
			Slash: 1, // Set to a non-zero value to ensure it's printed
		})
	}
	return newDoc
}

func (ex *Extractor) wrap(expr ast.Expr) ast.Expr {
	if !ex.pointers {
		return expr
	}
	switch e := expr.(type) {
	case *ast.StarExpr, *ast.ArrayType, *ast.MapType:
		return expr
	case *ast.SelectorExpr:
		if x, ok := e.X.(*ast.Ident); ok && x.Name == "metav1" {
			if e.Sel.Name == "TypeMeta" || e.Sel.Name == "ObjectMeta" {
				return expr
			}
		}
	}
	return &ast.StarExpr{X: expr}
}

func (ex *Extractor) rewriteType(pkg *packages.Package, expr ast.Expr) ast.Expr {
	switch t := expr.(type) {
	case *ast.StructType:
		newFields := &ast.FieldList{}
		for _, f := range t.Fields.List {
			newField := &ast.Field{
				Tag: copyTag(f.Tag),
			}
			for _, n := range f.Names {
				newField.Names = append(newField.Names, ast.NewIdent(n.Name))
			}
			newField.Type = ex.wrap(ex.rewriteType(pkg, f.Type))
			newFields.List = append(newFields.List, newField)
		}
		return &ast.StructType{Fields: newFields}

	case *ast.StarExpr:
		return &ast.StarExpr{X: ex.rewriteType(pkg, t.X)}

	case *ast.ArrayType:
		return &ast.ArrayType{
			Len: t.Len,
			Elt: ex.wrap(ex.rewriteType(pkg, t.Elt)),
		}

	case *ast.MapType:
		return &ast.MapType{
			Key:   ex.rewriteType(pkg, t.Key),
			Value: ex.wrap(ex.rewriteType(pkg, t.Value)),
		}

	case *ast.Ident:
		if isBasicType(t.Name) {
			return ast.NewIdent(t.Name)
		}
		ln := ex.enqueue(pkg, t.Name)
		return ast.NewIdent(ln)

	case *ast.SelectorExpr:
		if xIdent, ok := t.X.(*ast.Ident); ok {
			file := ex.findFileForNode(pkg, t)
			if file == nil {
				return ast.NewIdent(t.Sel.Name)
			}

			importPath := resolveImport(file, xIdent.Name)
			if importPath == "" {
				return ast.NewIdent(t.Sel.Name)
			}

			alias := ex.getImportAlias(importPath)
			if alias != "" {
				return &ast.SelectorExpr{
					X:   ast.NewIdent(alias),
					Sel: ast.NewIdent(t.Sel.Name),
				}
			}

			depPkg := ex.pkgs[importPath]
			if depPkg == nil {
				return ast.NewIdent(t.Sel.Name)
			}

			ln := ex.enqueue(depPkg, t.Sel.Name)
			return ast.NewIdent(ln)
		}
		return ast.NewIdent(t.Sel.Name)

	default:
		return expr
	}
}

func (ex *Extractor) getImportAlias(path string) string {
	if alias, ok := ex.imports[path]; ok {
		return alias
	}

	if alias, ok := allowedPkgs[path]; ok {
		ex.imports[path] = alias
		return alias
	}

	if isStdLib(path) {
		parts := strings.Split(path, "/")
		base := parts[len(parts)-1]
		base = strings.ReplaceAll(base, ".", "")
		base = strings.ReplaceAll(base, "-", "")

		alias := base
		count := 0
		for {
			collision := false
			for p, a := range ex.imports {
				if a == alias && p != path {
					collision = true
					break
				}
			}
			if !collision {
				break
			}
			count++
			alias = fmt.Sprintf("%s%d", base, count)
		}

		ex.imports[path] = alias
		return alias
	}
	return ""
}

func isStdLib(path string) bool {
	parts := strings.Split(path, "/")
	return !strings.Contains(parts[0], ".")
}

func copyTag(tag *ast.BasicLit) *ast.BasicLit {
	if tag == nil {
		return nil
	}
	return &ast.BasicLit{
		Kind:  tag.Kind,
		Value: tag.Value,
	}
}

func (ex *Extractor) findFileForNode(pkg *packages.Package, node ast.Node) *ast.File {
	for _, f := range pkg.Syntax {
		if f.Pos() <= node.Pos() && f.End() >= node.End() {
			return f
		}
	}
	return nil
}

func resolveImport(f *ast.File, alias string) string {
	for _, imp := range f.Imports {
		var name string
		if imp.Name != nil {
			name = imp.Name.Name
		} else {
			path := strings.Trim(imp.Path.Value, "\"")
			parts := strings.Split(path, "/")
			name = parts[len(parts)-1]
		}
		if name == alias {
			return strings.Trim(imp.Path.Value, "\"")
		}
	}
	return ""
}

func isBasicType(name string) bool {
	switch name {
	case "bool", "uint", "uint8", "uint16", "uint32", "uint64",
		"int", "int8", "int16", "int32", "int64",
		"float32", "float64", "complex64", "complex128",
		"string", "byte", "rune", "uintptr", "error":
		return true
	}
	return false
}

func typeKey(pkgPath, name string) string {
	return pkgPath + "." + name
}

func (ex *Extractor) generate() {
	f := &ast.File{
		Name:  ast.NewIdent(*outPkg),
		Decls: ex.localDecls,
	}

	importSpecs := []ast.Spec{}
	for path, alias := range ex.imports {
		importSpecs = append(importSpecs, &ast.ImportSpec{
			Name: ast.NewIdent(alias),
			Path: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", path)},
		})
	}
	if len(importSpecs) > 0 {
		f.Decls = append([]ast.Decl{
			&ast.GenDecl{
				Tok:   token.IMPORT,
				Specs: importSpecs,
			},
		}, f.Decls...)
	}

	var buf bytes.Buffer
	buf.WriteString("//go:build !ignore_autogenerated\n\n")
	buf.WriteString("// Code generated by crd-extractor. DO NOT EDIT.\n\n")
	buf.WriteString("// +kubebuilder:object:generate=true\n")

	fset := token.NewFileSet()
	if err := printer.Fprint(&buf, fset, f); err != nil {
		log.Fatalf("Failed to print: %v", err)
	}

	src, err := format.Source(buf.Bytes())
	if err != nil {
		log.Printf("Formatting error: %v", err)
		src = buf.Bytes()
	}

	if err := os.WriteFile(*outPath, src, 0o644); err != nil {
		log.Fatalf("Failed to write output: %v", err)
	}
}
