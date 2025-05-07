package main

import (
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"

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
func extractSchemas(crd apiv1.CustomResourceDefinition) (*apiv1.JSONSchemaProps, string) {
	var schema *apiv1.JSONSchemaProps
	var version string

	// Try to get schema from new CRD format first (v1)
	if len(crd.Spec.Versions) > 0 {
		for _, v := range crd.Spec.Versions {
			if v.Storage {
				schema = v.Schema.OpenAPIV3Schema
				version = v.Name
				break
			}
		}
	}

	return schema, version
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
						fieldType = "map[string]" + prop.AdditionalProperties.Schema.Type
					} else {
						// Object with no properties, use map
						fieldType = "map[string]interface{}"
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

func main() {
	if len(os.Args) < 3 {
		_, _ = fmt.Println("Usage: crd-parser <crd-yaml-file> <output-directory>")
		os.Exit(1)
	}

	inputFile := os.Args[1]
	outputDir := os.Args[2]

	// Read input file
	data, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	// Parse CRD YAML
	var crd apiv1.CustomResourceDefinition
	err = yaml.Unmarshal(data, &crd)
	if err != nil {
		log.Fatalf("Error parsing YAML: %v", err)
	}

	// Extract schema
	schema, version := extractSchemas(crd)
	if schema == nil {
		log.Fatalf("Could not find OpenAPI schema in CRD")
	}

	// Extract CRD info
	crdKind := crd.Spec.Names.Kind
	crdGroup := crd.Spec.Group

	// Generate structs
	structMap := make(map[string]*StructDef)
	rootName := crdKind
	generateStructs(schema, rootName, structMap, crdKind, true)

	// Generate types code
	typesCode := generateTypesCode(structMap, version, crdKind, crdGroup, version)

	// Write output file
	outputFile := filepath.Join(outputDir, version, fmt.Sprintf("types_%s.go", strings.ToLower(crdKind)))
	err = os.WriteFile(outputFile, []byte(typesCode), 0o644)
	if err != nil {
		log.Fatalf("Error writing output file: %v", err)
	}

	_, _ = fmt.Printf("Successfully generated Go structs from %s CRD to %s\n", crdKind, outputFile)

	// Generate GroupVersionInfo code
	gvi := generateGroupVersionInfoCode(crdGroup, version)

	// Write output file
	outputFile = filepath.Join(outputDir, version, "group_version_info.go")
	err = os.WriteFile(outputFile, []byte(gvi), 0o644)
	if err != nil {
		log.Fatalf("Error writing output file: %v", err)
	}

	_, _ = fmt.Printf("Successfully generated GroupVersionInfo from %s CRD to %s\n", crdKind, outputFile)
}
