package openapi

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"

	apiv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func Parse(crdData []byte, desiredVersion string) (*CustomResource, error) {
	// Parse CRD YAML
	var crd apiv1.CustomResourceDefinition
	err := yaml.Unmarshal(crdData, &crd)
	if err != nil {
		return nil, err
	}
	// Extract CRD info

	// Extract schema
	schema, version, err := extractSchemas(crd, desiredVersion)
	if err != nil {
		return nil, err
	}

	cr := &CustomResource{
		Kind:             crd.Spec.Names.Kind,
		Plural:           crd.Spec.Names.Plural,
		List:             crd.Spec.Names.ListKind,
		Group:            crd.Spec.Group,
		Version:          version,
		Structs:          make(map[string]*StructDef),
		Imports:          map[string]bool{`metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"`: true},
		structSignatures: make(map[string]string),
		structNamesCnt:   make(map[string]int),
	}

	// Generate structs
	generateStructs(schema, cr, cr.Kind, cr.Kind, true)
	return cr, nil
}

// Process schema and generate structs.
func generateStructs(schema *apiv1.JSONSchemaProps, cr *CustomResource, name, path string, root bool) {
	structDef := &StructDef{
		Root:        root,
		Name:        name,
		Description: fmt.Sprintf("%s represents a %s", name, path),
	}
	if root {
		cr.Root = structDef
	} else {
		cr.Structs[name] = structDef
	}

	for propName, prop := range schema.Properties {
		fieldName := ToCamelCase(propName)
		var fieldType string

		if prop.Type != "" { //nolint:gocritic
			fieldType = mapType(prop)

			// Handle nested objects by creating a new struct
			switch prop.Type {
			case "object":
				if len(prop.Properties) > 0 {
					signature := sign(prop.Properties)

					if ft, ok := cr.structSignatures[signature]; ok {
						fieldType = ft
					} else {
						kindFieldName := cr.Kind + fieldName
						var trueFieldName string
						if cnt, ok := cr.structNamesCnt[kindFieldName]; ok {
							trueFieldName = fmt.Sprintf("%s%d", kindFieldName, cnt)
							cr.structNamesCnt[kindFieldName] = cnt + 1
						} else {
							trueFieldName = kindFieldName
							cr.structNamesCnt[kindFieldName] = 1
						}
						fieldType = trueFieldName
						cr.structSignatures[signature] = fieldType
						generateStructs(&prop, cr, trueFieldName, path+"."+propName, false)
					}
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
					signature := sign(prop.Items.Schema.Properties)

					if ft, ok := cr.structSignatures[signature]; ok {
						fieldType = "[]" + ft
					} else {
						kindFieldName := cr.Kind + fieldName
						var trueFieldName string
						if cnt, ok := cr.structNamesCnt[kindFieldName]; ok {
							trueFieldName = fmt.Sprintf("%s%d", kindFieldName, cnt)
							cr.structNamesCnt[kindFieldName] = cnt + 1
						} else {
							trueFieldName = kindFieldName
							cr.structNamesCnt[kindFieldName] = 1
						}
						fieldType = "[]" + trueFieldName
						cr.structSignatures[signature] = trueFieldName
						generateStructs(prop.Items.Schema, cr, trueFieldName, path+"."+propName, false)
					}
				}
			default:
				fieldType = mapType(prop)
			}
		} else if prop.Ref != nil {
			// Handle references
			parts := strings.Split(*prop.Ref, "/")
			fieldType = ToCamelCase(parts[len(parts)-1])
		} else {
			fieldType = "*apiextensionsv1.JSON"
			cr.Imports[`apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"`] = true
		}

		field := FieldDef{
			Name:        fieldName,
			JSONTag:     propName,
			Description: prop.Description,
		}

		if prop.Items != nil && len(prop.Items.Schema.Enum) > 0 {
			signature := sign(prop.Items.Schema.Enum)

			if ft, ok := cr.structSignatures[signature]; ok {
				fieldType = ft
			} else {
				kindFieldName := cr.Kind + fieldName
				var trueFieldName string
				if cnt, ok := cr.structNamesCnt[kindFieldName]; ok {
					trueFieldName = fmt.Sprintf("%s%d", kindFieldName, cnt)
					cr.structNamesCnt[kindFieldName] = cnt + 1
				} else {
					trueFieldName = kindFieldName
					cr.structNamesCnt[kindFieldName] = 1
				}

				field.Enums = generateEnum(prop.Items.Schema, trueFieldName)
				field.EnumType = prop.Items.Schema.Type
				fieldType = "[]" + trueFieldName
				field.EnumName = trueFieldName
				cr.structSignatures[signature] = trueFieldName
			}
		} else if len(prop.Enum) > 0 {
			signature := sign(prop.Enum)
			if ft, ok := cr.structSignatures[signature]; ok {
				fieldType = ft
			} else {
				kindFieldName := cr.Kind + fieldName
				var trueFieldName string
				if cnt, ok := cr.structNamesCnt[kindFieldName]; ok {
					trueFieldName = fmt.Sprintf("%s%d", kindFieldName, cnt)
					cr.structNamesCnt[kindFieldName] = cnt + 1
				} else {
					trueFieldName = kindFieldName
					cr.structNamesCnt[kindFieldName] = 1
				}
				field.Enums = generateEnum(&prop, trueFieldName)
				field.EnumType = prop.Type
				field.EnumName = trueFieldName
				fieldType = trueFieldName
				cr.structSignatures[signature] = trueFieldName
			}
		}

		field.Type = fieldType

		structDef.Fields = append(structDef.Fields, field)
	}
}

// ToCamelCase convert string to CamelCase.
func ToCamelCase(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})

	for i, word := range words {
		if word != "" {
			words[i] = strings.ToUpper(string(word[0])) + word[1:]
		}
	}

	return strings.Join(words, "")
}

// Extract schemas from CRD.
func extractSchemas(
	crd apiv1.CustomResourceDefinition,
	desiredVersion string,
) (schema *apiv1.JSONSchemaProps, version string, err error) {
	// Try to get schema from new CRD format first (v1)
	if len(crd.Spec.Versions) > 0 {
		for _, v := range crd.Spec.Versions {
			if v.Storage && (desiredVersion == "" || desiredVersion == v.Name) {
				return v.Schema.OpenAPIV3Schema, v.Name, nil
			}
		}
	}

	return nil, "", fmt.Errorf("could not find desired version %q in CRD", desiredVersion)
}

// Helper function to map OpenAPI types to Go types.
func mapType(prop apiv1.JSONSchemaProps) string {
	if prop.Type == "" {
		if prop.Ref != nil {
			parts := strings.Split(*prop.Ref, "/")
			return ToCamelCase(parts[len(parts)-1])
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

// Process schema and generate enums.
func generateEnum(prop *apiv1.JSONSchemaProps, fieldName string) (enums []EnumDef) {
	for _, e := range prop.Enum {
		val := string(e.Raw)
		enums = append(enums, EnumDef{
			Name:  fieldName + ToCamelCase(strings.ReplaceAll(val, `"`, "")),
			Value: val,
		})
	}
	return enums
}

func sign(y any) string {
	b, _ := json.Marshal(y)
	hash := md5.Sum(b)
	return hex.EncodeToString(hash[:])
}
