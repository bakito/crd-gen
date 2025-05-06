package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

type StructField struct {
	Name     string `json:"name"`
	JSONName string `json:"json-name"`
	Type     string `json:"type"`
	Comment  string `json:"comment,omitempty"`
}

type StructDef struct {
	Name   string
	Fields []StructField
}

func main() {
	// Load CRD YAML file
	crdData, err := os.ReadFile("mycrd.yaml")
	if err != nil {
		log.Fatalf("Failed to read CRD file: %v", err)
	}

	var crd map[string]interface{}
	if err := yaml.Unmarshal(crdData, &crd); err != nil {
		log.Fatalf("Failed to unmarshal CRD YAML: %v", err)
	}

	// Extract openAPIV3Schema from CRD
	versions := crd["spec"].(map[string]interface{})["versions"].([]interface{})
	version := versions[0].(map[string]interface{})
	schema := version["schema"].(map[string]interface{})
	openAPIV3Schema := schema["openAPIV3Schema"]

	jsonData, err := json.Marshal(openAPIV3Schema)
	if err != nil {
		log.Fatalf("Failed to marshal schema to JSON: %v", err)
	}

	schemaRef := &openapi3.SchemaRef{}
	if err := json.Unmarshal(jsonData, schemaRef); err != nil {
		log.Fatalf("Failed to parse schema: %v", err)
	}

	if err := schemaRef.Validate(context.Background()); err != nil {
		log.Fatalf("Schema validation failed: %v", err)
	}

	// Flattened struct extraction
	structDef := extractFlatStruct("Spec", schemaRef)

	// Print Go struct
	b, _ := json.Marshal(structDef)
	fmt.Println(string(b))
}

func extractFlatStruct(name string, schema *openapi3.SchemaRef) StructDef {
	def := StructDef{Name: name}

	for propName, propSchema := range schema.Value.Properties {
		goField := toCamelCase(propName)
		fieldType := resolveType(propSchema)

		typ := getFirstType(propSchema.Value)

		if typ == "object" && len(propSchema.Value.Properties) > 0 {
			for nestedName, nestedProp := range propSchema.Value.Properties {
				nestedField := toCamelCase(propName + "_" + nestedName)
				nestedType := resolveType(nestedProp)
				def.Fields = append(def.Fields, StructField{
					Name:     nestedField,
					JSONName: fmt.Sprintf("%s.%s", propName, nestedName),
					Type:     nestedType,
				})
			}
		} else {
			def.Fields = append(def.Fields, StructField{
				Name:     goField,
				JSONName: propName,
				Type:     fieldType,
			})
		}
	}

	return def
}

func resolveType(ref *openapi3.SchemaRef) string {
	if ref == nil || ref.Value == nil {
		return "interface{}"
	}

	schema := ref.Value
	typ := getFirstType(schema)

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
		return "[]" + resolveType(schema.Items)
	case "object":
		return "struct"
	default:
		return "interface{}"
	}
}

func getFirstType(schema *openapi3.Schema) string {
	if schema.Type != nil && len(*schema.Type) > 0 {
		return (*schema.Type)[0]
	}
	return ""
}

func toCamelCase(input string) string {
	parts := strings.FieldsFunc(input, func(r rune) bool {
		return r == '_' || r == '-' || r == '.'
	})
	for i := range parts {
		parts[i] = strings.Title(parts[i])
	}
	return strings.Join(parts, "")
}
