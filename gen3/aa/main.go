package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// SchemaProperty represents a property in an OpenAPI schema
type SchemaProperty struct {
	Type        any            `yaml:"type"`
	Format      string         `yaml:"format,omitempty"`
	Description string         `yaml:"description,omitempty"`
	Properties  map[string]any `yaml:"properties,omitempty"`
	Items       map[string]any `yaml:"items,omitempty"`
	Ref         string         `yaml:"$ref,omitempty"`
}

// CustomResourceDefinition represents a simplified K8s CRD structure
type CustomResourceDefinition struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Group    string `yaml:"group"`
		Version  string `yaml:"version,omitempty"`
		Versions []struct {
			Name    string `yaml:"name"`
			Served  bool   `yaml:"served"`
			Storage bool   `yaml:"storage"`
			Schema  struct {
				OpenAPIV3Schema map[string]any `yaml:"openAPIV3Schema"`
			} `yaml:"schema,omitempty"`
		} `yaml:"versions,omitempty"`
		Names struct {
			Kind     string `yaml:"kind"`
			ListKind string `yaml:"listKind,omitempty"`
			Plural   string `yaml:"plural"`
			Singular string `yaml:"singular,omitempty"`
		} `yaml:"names"`
		Validation struct {
			OpenAPIV3Schema map[string]any `yaml:"openAPIV3Schema,omitempty"`
		} `yaml:"validation,omitempty"`
	} `yaml:"spec"`
}

// StructDef represents a Go struct definition
type StructDef struct {
	Name        string
	Fields      []FieldDef
	Description string
	Root        bool
}

// FieldDef represents a field in a Go struct
type FieldDef struct {
	Name        string
	Type        string
	JsonTag     string
	Description string
}

// Helper function to convert string to CamelCase
func toCamelCase(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})

	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}

	return strings.Join(words, "")
}

// Helper function to map OpenAPI types to Go types
func mapType(property map[string]any) string {
	typeValue, hasType := property["type"]
	if !hasType {
		return "any"
	}

	switch t := typeValue.(type) {
	case string:
		switch t {
		case "string":
			format, ok := property["format"]
			if ok {
				switch format {
				case "date-time":
					return "time.Time"
				case "byte", "binary":
					return "[]byte"
				}
			}
			return "string"
		case "integer", "number":
			format, ok := property["format"]
			if ok {
				switch format {
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
			if t == "integer" {
				return "int64"
			}
			return "float64"
		case "boolean":
			return "bool"
		case "array":
			items, ok := property["items"].(map[string]any)
			if ok {
				itemType := mapType(items)
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
	default:
		return "any"
	}
}

// Extract schemas from CRD
func extractSchemas(crd CustomResourceDefinition) (map[string]any, string) {
	var schema map[string]any
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

	// Fallback to old format (v1beta1)
	if schema == nil && crd.Spec.Validation.OpenAPIV3Schema != nil {
		schema = crd.Spec.Validation.OpenAPIV3Schema
		version = crd.Spec.Version
	}

	return schema, version
}

// Process schema and generate structs
func generateStructs(schema map[string]any, name string, structMap map[string]*StructDef, path string, root bool) {
	props, hasProps := schema["properties"].(map[string]any)
	if !hasProps {
		return
	}

	structDef := &StructDef{
		Root:        root,
		Name:        name,
		Description: fmt.Sprintf("%s represents a %s", name, path),
	}
	structMap[name] = structDef

	for propName, propValue := range props {
		propMap, ok := propValue.(map[string]any)
		if !ok {
			continue
		}

		fieldName := toCamelCase(propName)
		var fieldType string
		description := ""

		if desc, ok := propMap["description"].(string); ok {
			description = desc
		}

		typeVal, hasType := propMap["type"]
		if hasType {
			fieldType = mapType(propMap)

			// Handle nested objects by creating a new struct
			if typeStr, ok := typeVal.(string); ok && typeStr == "object" {
				if nestedProps, ok := propMap["properties"].(map[string]any); ok && len(nestedProps) > 0 {
					nestedName := name + fieldName
					fieldType = nestedName
					generateStructs(propMap, nestedName, structMap, path+"."+propName, false)
				} else {
					// Object with no properties, use map
					fieldType = "map[string]interface{}"
				}
			} else {
				fieldType = mapType(propMap)
			}
		} else if ref, ok := propMap["$ref"].(string); ok {
			// Handle references
			parts := strings.Split(ref, "/")
			fieldType = toCamelCase(parts[len(parts)-1])
		} else {
			fieldType = "any"
		}

		field := FieldDef{
			Name:        fieldName,
			Type:        fieldType,
			JsonTag:     propName,
			Description: description,
		}

		structDef.Fields = append(structDef.Fields, field)
	}
}

// Generate Go code from struct definitions
func generateGoCode(structMap map[string]*StructDef, packageName, crdKind, crdGroup, crdVersion string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("// Code generated by crd-parser. DO NOT EDIT.\n\n"))
	sb.WriteString(fmt.Sprintf("package %s\n\n", packageName))

	// Add imports
	sb.WriteString("import (\n")
	sb.WriteString("\t\"time\"\n")
	sb.WriteString("\n")
	sb.WriteString("\tmetav1 \"k8s.io/apimachinery/pkg/apis/meta/v1\"\n")
	sb.WriteString(")\n\n")

	// Add comment header
	sb.WriteString(fmt.Sprintf("// Generated from %s.%s/%s CRD\n\n", crdKind, crdGroup, crdVersion))

	// Sort and generate structs
	for _, structDef := range structMap {

		if structDef.Root {
			sb.WriteString("// +kubebuilder:object:root=true\n\n")
		}

		// Add struct comment
		if structDef.Description != "" {
			d := prepareDescription(structDef.Description, false)
			sb.WriteString(fmt.Sprintf("// %s\n", d))
		}

		// Start struct definition
		sb.WriteString(fmt.Sprintf("type %s struct {\n", structDef.Name))

		// Add fields
		if structDef.Root {
			sb.WriteString("\tmetav1.TypeMeta   `json:\",inline\"`\n")
			sb.WriteString("\tmetav1.ObjectMeta `json:\"metadata,omitempty\"`\n")
		}
		for _, field := range structDef.Fields {
			if !structDef.Root || field.Name == "Spec" || field.Name == "Status" {
				if field.Description != "" {
					d := prepareDescription(field.Description, true)
					sb.WriteString(fmt.Sprintf("\t// %s\n", d))
				}
				sb.WriteString(fmt.Sprintf("\t%s %s `json:\"%s,omitempty\"`\n", field.Name, field.Type, field.JsonTag))
			}
		}

		// Close struct definition
		sb.WriteString("}\n\n")
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

// Extract package name from a path
func getPackageName(filePath string) string {
	path := strings.Split(filePath, string(os.PathSeparator))
	if len(path) > 1 {
		return path[len(path)-2]
	}
	baseName := filepath.Base(filePath)
	ext := filepath.Ext(baseName)
	name := strings.TrimSuffix(baseName, ext)

	// Clean up the name to be a valid Go package name
	reg := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	name = reg.ReplaceAllString(name, "")

	// Ensure it starts with a letter
	if len(name) == 0 || !unicode.IsLetter(rune(name[0])) {
		name = "crd" + name
	}

	return strings.ToLower(name)
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: crd-parser <crd-yaml-file> <output-go-file>")
		os.Exit(1)
	}

	inputFile := os.Args[1]
	outputFile := os.Args[2]

	// Read input file
	data, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	// Parse CRD YAML
	var crd CustomResourceDefinition
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

	// Generate Go code
	packageName := getPackageName(outputFile)
	goCode := generateGoCode(structMap, packageName, crdKind, crdGroup, version)

	// Write output file
	err = os.WriteFile(outputFile, []byte(goCode), 0644)
	if err != nil {
		log.Fatalf("Error writing output file: %v", err)
	}

	fmt.Printf("Successfully generated Go structs from %s CRD to %s\n", crdKind, outputFile)
}
