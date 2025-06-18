package openapi

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"
	"unicode"

	apiv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func Parse(crds []string, version string, pointerVars bool) (res *CustomResources, success bool) {
	res = &CustomResources{
		structHashes: make(map[string]string),
		structNames:  make(map[string]bool),
		Version:      version,
	}
	var crdKind string

	for i, crd := range crds {
		// Read first crd file
		data, err := os.ReadFile(crd)
		if err != nil {
			slog.Error("Error reading file", "error", err)
			return nil, false
		}

		cr, err := res.parseSingleCRD(data, res.Version)
		if err != nil {
			slog.Error("Error parsing crd", "error", err)
			return nil, false
		}
		res.Names = append(res.Names, CRDNames{Kind: cr.Kind, List: cr.List})

		if i > 0 && res.Group != cr.group {
			slog.Error(
				"Not all CRD have the same group",
				"group-a", res.Group, "kind-a", crdKind,
				"group-b", cr.group, "kind-b", cr.Kind,
			)
			return nil, false
		}

		if version != "" && version != cr.version {
			slog.Error(
				"Not all CRD have the same version",
				"group-a", res.Group, "version-a", version, "kind-a", crdKind,
				"group-b", cr.group, "version-b", cr.version, "kind-b", cr.Kind,
			)
			return nil, false
		}
		res.Version = cr.version
		crdKind = cr.Kind
		res.Group = cr.group
		res.Items = append(res.Items, cr)
	}

	if pointerVars {
		// convert fields to pointers - there is room for improvement here, but it works for now
		for i, item := range res.Items {
			for s, def := range item.Structs {
				for f, field := range def.Fields {
					if strings.Contains(field.Type, "]") {
						// handle slice and maps
						res.Items[i].Structs[s].Fields[f].Type = strings.Replace(field.Type, "]", "]*", 1)
					} else {
						res.Items[i].Structs[s].Fields[f].Type = "*" + field.Type
					}
				}
			}
		}
	}

	return res, true
}

func (r *CustomResources) parseSingleCRD(crdData []byte, desiredVersion string) (*CustomResource, error) {
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
		Kind:    crd.Spec.Names.Kind,
		Plural:  crd.Spec.Names.Plural,
		List:    crd.Spec.Names.ListKind,
		group:   crd.Spec.Group,
		version: version,
		Structs: make(map[string]*StructDef),
		Imports: map[string]bool{`metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"`: true},
	}

	// Generate structs
	r.generateStructs(schema, cr, cr.Kind, cr.Kind, true)
	return cr, nil
}

// Process schema and generate structs.
func (r *CustomResources) generateStructs(schema *apiv1.JSONSchemaProps, cr *CustomResource, name, path string, root bool) {
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

	for _, propName := range slices.Sorted(maps.Keys(schema.Properties)) {
		prop := schema.Properties[propName]
		fieldName := ToCamelCase(propName)
		var fieldType string

		if prop.Type != "" { //nolint:gocritic
			fieldType = mapType(prop)

			// Handle nested objects by creating a new struct
			switch prop.Type {
			case "object":
				if len(prop.Properties) > 0 {
					fieldType = r.generateStructProperty(cr, &prop, fieldName, path, propName, root)
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
					fieldType = "[]" + r.generateStructProperty(cr, prop.Items.Schema, fieldName, path, propName, root)
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
			fieldType = "[]" + r.generateEnumStruct(cr, prop.Items.Schema, fieldName, &field, path)
		} else if len(prop.Enum) > 0 {
			fieldType = r.generateEnumStruct(cr, &prop, fieldName, &field, path)
		}

		field.Type = fieldType

		structDef.Fields = append(structDef.Fields, field)
	}
}

func (r *CustomResources) generateEnumStruct(
	cr *CustomResource,
	prop *apiv1.JSONSchemaProps,
	fieldName string,
	field *FieldDef,
	path string,
) (fieldType string) {
	hash := getHash(prop.Enum)
	if ft, ok := r.structHashes[hash]; ok {
		fieldType = ft
	} else {
		uniqFieldName := r.newUniqFieldName(cr, fieldName, false, path)
		field.Enums = generateEnum(prop, uniqFieldName)
		field.EnumType = mapType(*prop)
		field.EnumName = uniqFieldName
		fieldType = uniqFieldName
		r.structHashes[hash] = uniqFieldName
	}
	return fieldType
}

func (r *CustomResources) newUniqFieldName(cr *CustomResource, fieldName string, root bool, path string) string {
	name := fieldName
	if !root { // root structs should have kind prefix
		if _, ok := r.structNames[name]; !ok {
			r.structNames[name] = true
			return name
		}
	}
	name = cr.Kind + fieldName
	if _, ok := r.structNames[name]; !ok {
		r.structNames[name] = true
		return name
	}

	paths := strings.Split(path, ".")
	var prefix string
	for i := len(paths) - 1; i >= 0; i-- {
		prefix = ToCamelCase(paths[i]) + prefix
		name = prefix + fieldName
		if _, ok := r.structNames[name]; !ok {
			r.structNames[name] = true
			return name
		}
	}

	hash := md5.Sum([]byte(path + "." + fieldName))
	return fieldName + "_" + hex.EncodeToString(hash[:])
}

func (r *CustomResources) generateStructProperty(
	cr *CustomResource,
	prop *apiv1.JSONSchemaProps,
	fieldName, path, propName string,
	root bool,
) (fieldType string) {
	hash := getHash(prop.Properties)

	if ft, ok := r.structHashes[hash]; ok {
		fieldType = ft
	} else {
		uniqFieldName := r.newUniqFieldName(cr, fieldName, root, path)
		fieldType = uniqFieldName
		r.structHashes[hash] = uniqFieldName
		r.generateStructs(prop, cr, uniqFieldName, path+"."+propName, false)
	}
	return fieldType
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

func getHash(y any) string {
	b, _ := json.Marshal(y)
	hash := md5.Sum(b)
	return hex.EncodeToString(hash[:])
}
