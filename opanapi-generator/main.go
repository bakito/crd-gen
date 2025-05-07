package main

import (
	"flag"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/bakito/extract-crd-api/internal/flags"
	"github.com/jinzhu/inflection"
	apiv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const myName = "opanapi-generator"

// SchemaProperty represents a property in an OpenAPI schema.
type SchemaProperty struct {
	Type        any            `yaml:"type"`
	Format      string         `yaml:"format,omitempty"`
	Description string         `yaml:"description,omitempty"`
	Properties  map[string]any `yaml:"properties,omitempty"`
	Items       map[string]any `yaml:"items,omitempty"`
	Ref         string         `yaml:"$ref,omitempty"`
}

// StructDef represents a Go struct definition.
type StructDef struct {
	Name        string
	Fields      []FieldDef
	Description string
	Root        bool
}

// FieldDef represents a field in a Go struct.
type FieldDef struct {
	Name        string
	Type        string
	JSONTag     string
	Description string
	Enums       []EnumDef
	EnumName    string
	EnumType    string
}

type EnumDef struct {
	Name  string
	Value string
}

// Helper function to convert string to CamelCase.
func toCamelCase(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})

	for i, word := range words {
		if word != "" {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}

	return strings.Join(words, "")
}

// Helper function to map OpenAPI types to Go types.
func mapType(prop apiv1.JSONSchemaProps) string {
	if prop.Type == "" {
		if prop.Ref != nil {
			parts := strings.Split(*prop.Ref, "/")
			return toCamelCase(parts[len(parts)-1])
		}
		return "any"
	}

	switch prop.Type {
	case "string":
		if prop.Format != "" {
			switch prop.Format {
			case "date-time":
				return "metav1.Time"
			case "byte", "binary":
				return "[]byte"
			}
		}
		return "string"
	case "integer", "number":
		if prop.Format != "" {
			switch prop.Format {
			case "int32":
				return "int32"
			case "int64":
				return "int64"
			case "float":
				return "float32"
			case "double":
				return "float64"
			}
		}
		if prop.Type == "integer" {
			return "int64"
		}
		return "float64"
	case "boolean":
		return "bool"
	case "array":
		if prop.Items != nil && prop.Items.Schema != nil {
			itemType := mapType(*prop.Items.Schema)
			return "[]" + itemType
		}
		return "[]any"
	case "object":
		// We don't need to mark this for later replacement since we'll handle object types
		// directly in the generateStructs function
		return "map[string]any"
	default:
		return "any"
	}
}

// Extract schemas from CRD.
func extractSchemas(crd apiv1.CustomResourceDefinition, desiredVersion string) (*apiv1.JSONSchemaProps, string) {
	// Try to get schema from new CRD format first (v1)
	if len(crd.Spec.Versions) > 0 {
		for _, v := range crd.Spec.Versions {
			if v.Storage && (desiredVersion == "" || desiredVersion == v.Name) {
				return v.Schema.OpenAPIV3Schema, v.Name
			}
		}
	}

	return nil, ""
}

// Process schema and generate enums.
func generateEnum(prop *apiv1.JSONSchemaProps, fieldName string) (enums []EnumDef) {
	for _, e := range prop.Enum {
		val := string(e.Raw)
		enums = append(enums, EnumDef{
			Name:  fieldName + toCamelCase(strings.ReplaceAll(val, `"`, "")),
			Value: val,
		})
	}
	return enums
}

// Process schema and generate structs.
func generateStructs(schema *apiv1.JSONSchemaProps, name string, structMap map[string]*StructDef, path string, root bool) {
	structDef := &StructDef{
		Root:        root,
		Name:        name,
		Description: fmt.Sprintf("%s represents a %s", name, path),
	}
	structMap[name] = structDef

	for propName, prop := range schema.Properties {
		fieldName := toCamelCase(propName)
		var fieldType string

		if prop.Type != "" { //nolint:gocritic
			fieldType = mapType(prop)

			// Handle nested objects by creating a new struct
			switch prop.Type {
			case "object":
				if len(prop.Properties) > 0 {
					nestedName := name + fieldName
					fieldType = nestedName
					generateStructs(&prop, nestedName, structMap, path+"."+propName, false)
				} else {
					if prop.AdditionalProperties != nil && prop.AdditionalProperties.Schema != nil {
						fieldType = "map[string]" + mapType(*prop.AdditionalProperties.Schema)
					} else {
						// Object with no properties, use map
						fieldType = "map[string]any"
					}
				}
			case "array":
				if prop.Items != nil && prop.Items.Schema != nil && prop.Items.Schema.Type == "object" {
					nestedName := name + fieldName
					generateStructs(prop.Items.Schema, nestedName, structMap, path+"."+propName, false)
					fieldType = "[]" + nestedName
				}
			default:
				fieldType = mapType(prop)
			}
		} else if prop.Ref != nil {
			// Handle references
			parts := strings.Split(*prop.Ref, "/")
			fieldType = toCamelCase(parts[len(parts)-1])
		} else {
			fieldType = "any"
		}

		field := FieldDef{
			Name:        fieldName,
			JSONTag:     propName,
			Description: prop.Description,
		}

		if prop.Items != nil && len(prop.Items.Schema.Enum) > 0 {
			nestedName := name + fieldName
			field.Enums = generateEnum(prop.Items.Schema, nestedName)
			field.EnumType = prop.Items.Schema.Type
			fieldType = "[]" + nestedName
			field.EnumName = nestedName
		} else if len(prop.Enum) > 0 {
			nestedName := name + fieldName
			field.Enums = generateEnum(&prop, nestedName)
			field.EnumType = prop.Type
			field.EnumName = nestedName
			fieldType = nestedName
		}

		field.Type = fieldType

		structDef.Fields = append(structDef.Fields, field)
	}
}

// Generate Go code from struct definitions.
func generateTypesCode(structMap map[string]*StructDef, version, crdKind, crdGroup, crdVersion string) string {
	var sb strings.Builder

	_, _ = sb.WriteString(fmt.Sprintf("// Code generated by %s. DO NOT EDIT.\n\n", myName))
	_, _ = sb.WriteString(fmt.Sprintf("package %s\n\n", version))

	// Add imports
	_, _ = sb.WriteString("import metav1 \"k8s.io/apimachinery/pkg/apis/meta/v1\"\n\n")

	// Add comment header
	_, _ = sb.WriteString(fmt.Sprintf("// Generated from %s.%s/%s CRD\n\n", crdKind, crdGroup, crdVersion))

	// Sort and generate structs
	sortedStructNames := slices.Sorted(maps.Keys(structMap))

	for _, structName := range sortedStructNames {
		structDef := structMap[structName]

		if structDef.Root {
			_, _ = sb.WriteString("func init() {\n")
			_, _ = sb.WriteString("\t// Register types with the scheme\n")
			_, _ = sb.WriteString(fmt.Sprintf("\tSchemeBuilder.Register(&%s{}, &%sList{})\n", structDef.Name, structDef.Name))
			_, _ = sb.WriteString("}\n\n")

			_, _ = sb.WriteString("// +kubebuilder:object:root=true\n\n")
			_, _ = sb.WriteString(
				fmt.Sprintf("// %sList is a list of %s. \n", structDef.Name, inflection.Plural(structDef.Name)),
			)
			_, _ = sb.WriteString(fmt.Sprintf("type %sList struct {\n", structDef.Name))
			_, _ = sb.WriteString("\tmetav1.TypeMeta   `json:\",inline\"`\n")
			_, _ = sb.WriteString("\tmetav1.ObjectMeta `json:\"metadata,omitempty\"`\n")
			_, _ = sb.WriteString(fmt.Sprintf("\tItems []%s `json:\"items\"`\n", structDef.Name))
			_, _ = sb.WriteString("}\n\n")

			_, _ = sb.WriteString("// +kubebuilder:object:root=true\n\n")
		}

		// Add struct comment
		if structDef.Description != "" {
			d := prepareDescription(structDef.Description, false)
			_, _ = sb.WriteString(fmt.Sprintf("// %s\n", d))
		}

		// Start struct definition
		_, _ = sb.WriteString(fmt.Sprintf("type %s struct {\n", structDef.Name))

		// Add fields
		if structDef.Root {
			_, _ = sb.WriteString("\tmetav1.TypeMeta   `json:\",inline\"`\n")
			_, _ = sb.WriteString("\tmetav1.ObjectMeta `json:\"metadata,omitempty\"`\n")
		}
		sort.Slice(structDef.Fields, func(i, j int) bool {
			return structDef.Fields[i].Name < structDef.Fields[j].Name
		})
		for _, field := range structDef.Fields {
			if !structDef.Root || field.Name == "Spec" || field.Name == "Status" {
				if field.Description != "" {
					d := prepareDescription(field.Description, true)
					_, _ = sb.WriteString(fmt.Sprintf("\t// %s\n", d))
				}
				_, _ = sb.WriteString(fmt.Sprintf("\t%s %s `json:\"%s,omitempty\"`\n", field.Name, field.Type, field.JSONTag))
			}
		}

		// Close struct definition
		_, _ = sb.WriteString("}\n\n")

		// Enums
		for _, field := range structDef.Fields {
			if len(field.Enums) == 0 {
				continue
			}
			// Start enum definition
			_, _ = sb.WriteString(fmt.Sprintf("// %s represents an enumeration for %s\n", field.EnumName, field.Name))
			_, _ = sb.WriteString(fmt.Sprintf("type %s %s\n\n", field.EnumName, field.EnumType))
			_, _ = sb.WriteString("var (\n")
			for _, e := range field.Enums {
				_, _ = sb.WriteString(fmt.Sprintf("\t// %s %s enum value %s\n", e.Name, field.Name, e.Value))
				_, _ = sb.WriteString(fmt.Sprintf("\t%s %s = %s\n", e.Name, field.EnumName, e.Value))
			}
			_, _ = sb.WriteString(")\n\n")
		}
	}

	return sb.String()
}

func prepareDescription(desc string, field bool) string {
	indent := ""
	if field {
		indent = "\t"
	}
	return strings.ReplaceAll(desc, "\n", fmt.Sprintf("\n%s// ", indent))
}

func generateGroupVersionInfoCode(group, version string) string {
	var sb strings.Builder

	_, _ = sb.WriteString(fmt.Sprintf("// Code generated by %s. DO NOT EDIT.\n\n", myName))
	_, _ = sb.WriteString(fmt.Sprintf("package %s\n\n", version))

	_, _ = sb.WriteString("// +kubebuilder:object:generate=true\n\n")

	_, _ = sb.WriteString("import (\n")
	_, _ = sb.WriteString("\t\"k8s.io/apimachinery/pkg/runtime/schema\"\n")
	_, _ = sb.WriteString("\t\"sigs.k8s.io/controller-runtime/pkg/scheme\"\n")
	_, _ = sb.WriteString(")\n\n")

	_, _ = sb.WriteString("var (\n")
	_, _ = sb.WriteString("\t// GroupVersion is group version used to register these objects.\n")
	_, _ = sb.WriteString(
		fmt.Sprintf("\tGroupVersion = schema.GroupVersion{Group: %q, Version: %q}\n\n", group, version),
	)

	_, _ = sb.WriteString("\t// SchemeBuilder is used to add go types to the GroupVersionKind scheme.\n")
	_, _ = sb.WriteString("\tSchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}\n\n")

	_, _ = sb.WriteString("\t		// AddToScheme adds the types in this group-version to the given scheme.\n")
	_, _ = sb.WriteString("\tAddToScheme = SchemeBuilder.AddToScheme\n")
	_, _ = sb.WriteString(")\n\n")

	return sb.String()
}

var (
	crds    flags.ArrayFlags
	target  string
	version string
)

func main() {
	flag.Var(&crds, "crd", "CRD file to process")
	flag.StringVar(&target, "target", "", "The target directory to copyFile the files to")
	flag.StringVar(&version, "version", "", "The version to select from the CRD; If not defined, the first version is used")
	flag.Parse()

	if strings.TrimSpace(target) == "" {
		slog.Error("Flag must be defined", "flag", "target")
		return
	}
	if len(crds) == 0 {
		slog.Error("At lease on CRD must be defined", "flag", "target")
		return
	}

	var crdGroup string
	var crdKind string

	var files []outFile
	for i, crd := range crds {
		// Read first crd file
		data, err := os.ReadFile(crd)
		if err != nil {
			slog.Error("Error reading file", "error", err)
			return
		}

		// Parse CRD YAML
		var crd apiv1.CustomResourceDefinition
		err = yaml.Unmarshal(data, &crd)
		if err != nil {
			slog.Error("Error parsing YAML", "error", err)
			return
		}
		// Extract CRD info

		if i > 0 && crdGroup != crd.Spec.Group {
			slog.Error(
				"Not all CRD have the same group",
				"group-a", crdGroup, "kind-a", crdKind,
				"group-b", crd.Spec.Group, "kind-b", crd.Spec.Names.Kind,
			)
			return
		}
		crdKind = crd.Spec.Names.Kind
		crdGroup = crd.Spec.Group

		// Extract schema
		schema, foundVersion := extractSchemas(crd, version)
		if schema == nil {
			slog.Error("Could not find OpenAPI schema in CRD", "version", version)
			return
		}

		if version != "" && version != foundVersion {
			slog.Error(
				"Not all CRD have the same verion",
				"group-a", crdGroup, "version-a", version, "kind-a", crdKind,
				"group-b", crd.Spec.Group, "version-b", foundVersion, "kind-b", crd.Spec.Names.Kind,
			)
			return
		}
		version = foundVersion

		// Generate structs
		structMap := make(map[string]*StructDef)
		rootName := crdKind
		generateStructs(schema, rootName, structMap, crdKind, true)

		// Generate types code
		typesCode := generateTypesCode(structMap, version, crdKind, crdGroup, version)

		// Write output file
		outputFile := filepath.Join(target, version, fmt.Sprintf("types_%s.go", strings.ToLower(crdKind)))
		files = append(files, outFile{
			name:       outputFile,
			content:    typesCode,
			successMsg: "Successfully generated Go structs",
			successArgs: []any{
				"group", crdGroup,
				"version", version,
				"kind", crdKind,
				"file", outputFile,
			},
		})
	}

	// Generate GroupVersionInfo code
	gvi := generateGroupVersionInfoCode(crdGroup, version)

	// Write output file
	outputFile := filepath.Join(target, version, "group_version_info.go")

	files = append(files, outFile{
		name:       outputFile,
		content:    gvi,
		successMsg: "Successfully generated GroupVersionInfo",
		successArgs: []any{
			"group", crdGroup, "version", version, "file", outputFile,
		},
	})

	writeFiles(files)
}

func writeFiles(files []outFile) {
	for _, f := range files {
		err := os.WriteFile(f.name, []byte(f.content), 0o644)
		if err != nil {
			slog.Error("Error writing output file", "error", err)
			return
		}

		slog.With(f.successArgs...).Info(f.successMsg)
	}
}

type outFile struct {
	name        string
	content     string
	successMsg  string
	successArgs []any
}
