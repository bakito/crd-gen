package openapi

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"slices"
	"strings"
	"unicode"

	apiv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	wildcardEnum    = "*"
	wildcardReplace = "All"
	enumEmptyValue  = "EmptyValue"
)

var k8sConfig clientcmd.ClientConfig

func Parse(
	ctx context.Context,
	k8sCfg clientcmd.ClientConfig,
	crds []string,
	version string,
	pointerVars bool,
) (res *CustomResources, success bool) {
	k8sConfig = k8sCfg
	res = &CustomResources{
		structHashes: make(map[string]string),
		structNames:  make(map[string]bool),
		Version:      version,
	}
	var crdKind string

	for i, crd := range crds {
		var ok bool
		if crdKind, ok = prepareCRD(ctx, crd, res, crdKind, version, i == 0); !ok {
			return nil, false
		}
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

func prepareCRD(ctx context.Context, crd string, res *CustomResources, crdKind, version string, isFirst bool) (string, bool) {
	data, ok := readCRD(ctx, crd)
	if !ok {
		return "", false
	}

	cr, err := res.parseCRD(data, res.Version)
	if err != nil {
		slog.ErrorContext(ctx, "Error parsing crd", "error", err)
		return "", false
	}
	res.Names = append(res.Names, CRDNames{Kind: cr.Kind, List: cr.List})

	if !isFirst && res.Group != cr.group {
		slog.ErrorContext(ctx,
			"Not all CRD have the same group",
			"group-a", res.Group, "kind-a", crdKind,
			"group-b", cr.group, "kind-b", cr.Kind,
		)
		return "", false
	}

	if version != "" && version != cr.version {
		slog.ErrorContext(ctx,
			"Not all CRD have the same version",
			"group-a", res.Group, "version-a", version, "kind-a", crdKind,
			"group-b", cr.group, "version-b", cr.version, "kind-b", cr.Kind,
		)
		return "", false
	}
	res.Version = cr.version
	res.Group = cr.group
	res.Items = append(res.Items, cr)
	return cr.Kind, true
}

func readCRD(ctx context.Context, crd string) ([]byte, bool) {
	// Read the first crd file
	var data []byte
	var err error
	switch {
	case strings.HasPrefix(crd, "http://") || strings.HasPrefix(crd, "https://"):
		// Download the file to a temp location
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, crd, http.NoBody)
		if err != nil {
			slog.ErrorContext(ctx, "Error creating request", "error", err)
			return nil, false
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			slog.ErrorContext(ctx, "Error downloading file", "error", err)
			return nil, false
		}
		defer resp.Body.Close()

		data, err = io.ReadAll(resp.Body)
		if err != nil {
			slog.ErrorContext(ctx, "Error reading downloaded file", "error", err)
			return nil, false
		}

	case strings.HasPrefix(crd, "k8s:"):
		// Fetch CRD via k8s client
		conf, err := k8sConfig.ClientConfig()
		if err != nil {
			slog.ErrorContext(ctx, "Error creating k8s client config", "error", err)
			return nil, false
		}

		crdName := strings.TrimPrefix(crd, "k8s:")
		client, err := clientset.NewForConfig(conf)
		if err != nil {
			slog.ErrorContext(ctx, "Error creating k8s client", "error", err)
			return nil, false
		}

		crdDef, err := client.ApiextensionsV1().
			CustomResourceDefinitions().
			Get(context.Background(), crdName, metav1.GetOptions{})
		if err != nil {
			slog.ErrorContext(ctx, "Error getting CRD", "error", err)
			return nil, false
		}

		data, err = json.Marshal(crdDef)
		if err != nil {
			slog.ErrorContext(ctx, "Error marshaling CRD", "error", err)
			return nil, false
		}

	default:
		// Read the local file
		data, err = os.ReadFile(crd)
		if err != nil {
			slog.ErrorContext(ctx, "Error reading file", "error", err)
			return nil, false
		}
	}
	return data, true
}

func (r *CustomResources) parseCRD(crdData []byte, desiredVersion string) (*CustomResource, error) {
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
		structDef.Path = strings.Join(strings.Split(path, ".")[1:], ".")
	}

	for _, propName := range slices.Sorted(maps.Keys(schema.Properties)) {
		prop := schema.Properties[propName]
		fieldName := ToCamelCase(propName)
		var fieldType string

		if prop.Type != "" { //nolint:gocritic
			fieldType = mapType(prop, cr)

			// Handle nested objects by creating a new struct
			switch prop.Type {
			case "object":
				if len(prop.Properties) > 0 {
					fieldType = r.generateStructProperty(cr, &prop, fieldName, path, propName, root)
				} else {
					switch {
					case prop.AdditionalProperties != nil && prop.AdditionalProperties.Schema != nil:
						additional := mapType(*prop.AdditionalProperties.Schema, cr)
						if additional == "map[string]any" {
							additional = r.generateStructProperty(
								cr,
								prop.AdditionalProperties.Schema,
								fieldName,
								path,
								propName,
								root,
							)
						}
						fieldType = "map[string]" + additional
					case propName != "metadata":
						fieldType = "runtime.RawExtension"
						cr.Imports[`"k8s.io/apimachinery/pkg/runtime"`] = true
					default:
						fieldType = "metav1.ObjectMeta"
					}
				}
			case "array":
				if prop.Items != nil && prop.Items.Schema != nil && prop.Items.Schema.Type == "object" {
					fieldType = "[]" + r.generateStructProperty(cr, prop.Items.Schema, fieldName, path, propName, root)
				}
			default:
				fieldType = mapType(prop, cr)
			}
		} else if prop.Ref != nil {
			// Handle references
			parts := strings.Split(*prop.Ref, "/")
			fieldType = ToCamelCase(parts[len(parts)-1])
		} else if prop.XIntOrString {
			fieldType = "intstr.IntOrString"
			cr.Imports[`"k8s.io/apimachinery/pkg/util/intstr"`] = true
		} else {
			fieldType = "apiextensionsv1.JSON"
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
		field.EnumType = mapType(*prop, cr)
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
		// Check if the current property is a metav1.Condition
		if isMetav1Condition(prop) {
			cr.Imports[`metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"`] = true
			return "metav1.Condition"
		}

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

func isMetav1Condition(schema *apiv1.JSONSchemaProps) bool {
	if schema == nil || schema.Type != "object" || schema.Properties == nil {
		return false
	}

	requiredProps := map[string]struct {
		Type   string
		Format string
		Enum   []string
	}{
		"type":               {Type: "string"},
		"status":             {Type: "string", Enum: []string{"True", "False", "Unknown"}},
		"reason":             {Type: "string"},
		"message":            {Type: "string"},
		"lastTransitionTime": {Type: "string", Format: "date-time"},
	}

	for propName, expected := range requiredProps {
		prop, ok := schema.Properties[propName]
		if !ok || prop.Type != expected.Type {
			return false
		}
		if expected.Format != "" && prop.Format != expected.Format {
			return false
		}
		if len(expected.Enum) > 0 {
			if len(prop.Enum) != len(expected.Enum) {
				return false
			}
			for _, enumVal := range expected.Enum {
				found := false
				for _, pEnumVal := range prop.Enum {
					if string(pEnumVal.Raw) == `"`+enumVal+`"` {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			}
		}
	}
	return true
}

// Helper function to map OpenAPI types to Go types.
func mapType(prop apiv1.JSONSchemaProps, cr *CustomResource) string {
	if prop.Type == "" {
		if prop.Ref != nil {
			parts := strings.Split(*prop.Ref, "/")
			return ToCamelCase(parts[len(parts)-1])
		}
		if prop.XIntOrString {
			cr.Imports[`"k8s.io/apimachinery/pkg/util/intstr"`] = true
			return "intstr.IntOrString"
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
			default:
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
			default:
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
			itemType := mapType(*prop.Items.Schema, cr)
			return "[]" + itemType
		}
		return "[]any"
	case "object":
		if isMetav1Condition(&prop) {
			cr.Imports[`metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"`] = true
			return "metav1.Condition"
		}
		// We don't need to mark this for later replacement since we'll handle object types
		// directly in the generateStructs function
		return "map[string]any"
	default:
		return "any"
	}
}

// createEnumName creates a cleaned and formatted enum name by combining field name and suffix.
func createEnumName(fieldName, enumValue string) string {
	cleanedValue := strings.ReplaceAll(enumValue, `"`, "")
	switch cleanedValue {
	case wildcardEnum:
		cleanedValue = wildcardReplace
	case "":
		cleanedValue = enumEmptyValue
	default:
	}
	return fieldName + ToCamelCase(cleanedValue)
}

func generateEnum(prop *apiv1.JSONSchemaProps, fieldName string) (enums []EnumDef) {
	for _, enumRaw := range prop.Enum {
		enumValue := string(enumRaw.Raw)
		enums = append(enums, EnumDef{
			Name:  createEnumName(fieldName, enumValue),
			Value: enumValue,
		})
	}
	return enums
}

func getHash(y any) string {
	b, _ := json.Marshal(y)
	hash := md5.Sum(b)
	return hex.EncodeToString(hash[:])
}
