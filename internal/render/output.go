package render

import (
	_ "embed"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
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

func WriteCrdFiles(resources *openapi.CustomResources, targetDir string) error {
	var files []outFile
	for _, cr := range resources.Items {
		// Generate types code
		typesCode, err := generateTypesCode(cr, resources.Group, resources.Version)
		if err != nil {
			return fmt.Errorf("error generating types content: %w", err)
		}

		// Write output file
		outputFile := filepath.Join(targetDir, resources.Version, fmt.Sprintf("types_%s.go", strings.ToLower(cr.Kind)))
		files = append(files, outFile{
			name:       outputFile,
			content:    typesCode,
			successMsg: "Successfully generated Go structs",
			successArgs: []any{
				"group", resources.Group,
				"version", resources.Version,
				"kind", cr.Kind,
				"file", outputFile,
			},
		})
	}

	// Generate GroupVersionInfo code
	gvi, err := generateGroupVersionInfoCode(resources)
	if err != nil {
		return fmt.Errorf("error writing group_version_kind.go: %w", err)
	}

	// Write output file
	outputFile := filepath.Join(targetDir, resources.Version, "group_version_info.go")

	files = append(files, outFile{
		name:       outputFile,
		content:    gvi,
		successMsg: "Successfully generated GroupVersionInfo",
		successArgs: []any{
			"group", resources.Group, "version", resources.Version, "file", outputFile,
		},
	})

	return writeFiles(files)
}

func writeFiles(files []outFile) error {
	for _, f := range files {
		dir := filepath.Dir(f.name)

		// Create the directory if it doesn't exist
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return fmt.Errorf("error creating directory: %w", err)
		}

		if err := os.WriteFile(f.name, []byte(f.content), 0o644); err != nil {
			return fmt.Errorf("error writing output file: %w", err)
		}

		slog.With(f.successArgs...).Info(f.successMsg)
	}
	return nil
}

type outFile struct {
	name        string
	content     string
	successMsg  string
	successArgs []any
}

// Generate Go code from struct definitions.
func generateTypesCode(cr *openapi.CustomResource, group, version string) (string, error) {
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
		"Group":   group,
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

func prepareDescription(desc string, field bool) string {
	indent := "// "
	if field {
		indent = "\t" + indent
	}
	return strings.ReplaceAll(desc, "\n", "\n"+indent)
}

func generateGroupVersionInfoCode(res *openapi.CustomResources) (string, error) {
	var sb strings.Builder
	t := template.Must(template.New("group_version_into.go.tpl").Parse(gviTpl))
	if err := t.Execute(&sb, map[string]any{
		"AppName":  myName,
		"Version":  res.Version,
		"Group":    res.Group,
		"CRDNames": res.Names,
	}); err != nil {
		return "", err
	}

	return sb.String(), nil
}
