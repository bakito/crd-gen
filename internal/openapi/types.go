package openapi

// SchemaProperty represents a property in an OpenAPI schema.
type SchemaProperty struct {
	Type        any            `yaml:"type"`
	Format      string         `yaml:"format,omitempty"`
	Description string         `yaml:"description,omitempty"`
	Properties  map[string]any `yaml:"properties,omitempty"`
	Items       map[string]any `yaml:"items,omitempty"`
	Ref         string         `yaml:"$ref,omitempty"`
}

type CustomResources struct {
	Items   []*CustomResource
	Names   []CRDNames
	Group   string
	Version string
}

type CustomResource struct {
	Kind    string
	Root    *StructDef
	Structs map[string]*StructDef
	Imports map[string]bool
	Plural  string
	List    string
	Group   string
	Version string

	structSignatures map[string]string
	structNamesCnt   map[string]int
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

type CRDNames struct {
	Kind string
	List string
}
