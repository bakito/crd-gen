package main

import (
	_ "embed"
	"maps"
	"slices"
	"sort"
	"strings"
	"text/template"

	"github.com/bakito/crd-gen/internal/openapi"
)

const myName = "opanapi-generator"

var (
	//go:embed group_version_into.go.tpl
	gviTpl string
	//go:embed types.go.tpl
	typeTpl string
)

// Generate Go code from struct definitions.
func generateTypesCode(cr *openapi.CustomResource) (string, error) {
	// Sort and generate structs
	sortedStructNames := slices.Sorted(maps.Keys(cr.Structs))

	var structs []*openapi.StructDef

	importList := slices.Sorted(maps.Keys(cr.Imports))

	prepare(cr.Root)

	// keep only spec and status
	var rootFields []openapi.FieldDef
	for _, field := range cr.Root.Fields {
		if field.JSONTag == "spec" || field.JSONTag == "status" {
			rootFields = append(rootFields, field)
		}
	}
	cr.Root.Fields = rootFields

	for _, structName := range sortedStructNames {
		structDef := cr.Structs[structName]
		prepare(structDef)

		structs = append(structs, structDef)
	}

	var sb strings.Builder
	t := template.Must(template.New("types.go.tpl").Parse(typeTpl))
	err := t.Execute(&sb, map[string]any{
		"AppName": myName,
		"Version": version,
		"Group":   cr.Group,
		"Kind":    cr.Kind,
		"List":    cr.List,
		"Plural":  openapi.ToCamelCase(cr.Plural),
		"Root":    cr.Root,
		"Structs": structs,
		"Imports": importList,
	})
	return sb.String(), err
}

func prepare(structDef *openapi.StructDef) {
	structDef.Description = prepareDescription(structDef.Description, false)

	sort.Slice(structDef.Fields, func(i, j int) bool {
		return structDef.Fields[i].Name < structDef.Fields[j].Name
	})

	for i, f := range structDef.Fields {
		structDef.Fields[i].Description = prepareDescription(f.Description, true)
	}
}

func generateGroupVersionInfoCode(group, version string, names []openapi.CRDNames) (string, error) {
	var sb strings.Builder
	t := template.Must(template.New("group_version_into.go.tpl").Parse(gviTpl))
	if err := t.Execute(&sb, map[string]any{
		"AppName":  myName,
		"Version":  version,
		"Group":    group,
		"CRDNames": names,
	}); err != nil {
		return "", err
	}

	return sb.String(), nil
}
