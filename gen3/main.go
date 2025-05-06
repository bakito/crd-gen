package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

func main() {
	crdData, err := os.ReadFile("mycrd.yaml")
	if err != nil {
		log.Fatalf("Failed to read CRD file: %v", err)
	}

	var crd map[string]any
	if err := yaml.Unmarshal(crdData, &crd); err != nil {
		log.Fatalf("Failed to unmarshal CRD YAML: %v", err)
	}

	// Extract openAPIV3Schema
	versions := crd["spec"].(map[string]any)["versions"].([]any)
	version := versions[0].(map[string]any)
	schema := version["schema"].(map[string]any)
	openAPIV3Schema := schema["openAPIV3Schema"]

	names := crd["spec"].(map[string]any)["names"].(map[string]any)
	kind := names["kind"].(string)
	//listKind := names["kind"].(string)

	jsonData, err := json.Marshal(openAPIV3Schema)
	if err != nil {
		log.Fatalf("Failed to marshal openAPIV3Schema to JSON: %v", err)
	}

	schemaRef := &openapi3.SchemaRef{}
	if err := json.Unmarshal(jsonData, schemaRef); err != nil {
		log.Fatalf("Failed to parse JSON into SchemaRef: %v", err)
	}

	if err := schemaRef.Validate(context.Background()); err != nil {
		log.Fatalf("Schema validation error: %v", err)
	}

	fmt.Print("package main\n// Auto-generated Go structs from openAPIV3Schema\n\n")
	generateStruct(kind, schemaRef.Value, 0, make(map[string]bool))
}

// Recursively generates structs
func generateStruct(name string, schema *openapi3.Schema, depth int, generated map[string]bool) {
	if generated[name] {
		return
	}
	generated[name] = true

	fmt.Printf("type %s struct {\n", name)

	keys := make([]string, 0, len(schema.Properties))
	for k := range schema.Properties {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, propName := range keys {
		prop := schema.Properties[propName]
		fieldName := toCamelCase(propName)
		fieldType := mapType(prop, propName, depth+1, generated)
		fmt.Printf("    %s %s `json:\"%s,omitempty\"`\n", fieldName, fieldType, propName)
	}

	fmt.Println("}")
	fmt.Println()
}

func mapType(ref *openapi3.SchemaRef, propName string, depth int, generated map[string]bool) string {
	if ref == nil || ref.Value == nil {
		return "any"
	}

	schema := ref.Value

	typ := "object"
	types := *schema.Type
	if len(types) > 0 {
		typ = types[0]
	}

	switch typ {
	case "string":
		return "string"
	case "integer":
		return "int"
	case "number":
		return "float64"
	case "boolean":
		return "bool"
	case "array":
		return "[]" + mapType(schema.Items, propName, depth, generated)
	case "object":
		if len(schema.Properties) > 0 {
			nestedName := toCamelCase(propName)
			generateStruct(nestedName, schema, depth, generated)
			return nestedName
		}
		return "map[string]any"
	default:
		return "any"
	}
}

// Converts snake_case or kebab-case to CamelCase
func toCamelCase(input string) string {
	parts := strings.FieldsFunc(input, func(r rune) bool {
		return r == '_' || r == '-' || r == '.'
	})
	for i := range parts {
		parts[i] = strings.Title(parts[i])
	}
	return strings.Join(parts, "")
}
