package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/bakito/crd-gen/internal/flags"
	"github.com/bakito/crd-gen/internal/openapi"
)

func prepareDescription(desc string, field bool) string {
	indent := "// "
	if field {
		indent = "\t" + indent
	}
	return strings.ReplaceAll(desc, "\n", "\n"+indent)
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
	var crdVersion string
	var names []openapi.CRDNames

	var files []outFile
	for i, crd := range crds {
		// Read first crd file
		data, err := os.ReadFile(crd)
		if err != nil {
			slog.Error("Error reading file", "error", err)
			return
		}

		cr, err := openapi.Parse(data, crdVersion)
		if err != nil {
			slog.Error("Error parsing crd", "error", err)
			return
		}
		names = append(names, openapi.CRDNames{Kind: cr.Kind, List: cr.List})

		if i > 0 && crdGroup != cr.Group {
			slog.Error(
				"Not all CRD have the same group",
				"group-a", crdGroup, "kind-a", crdKind,
				"group-b", cr.Group, "kind-b", cr.Kind,
			)
			return
		}

		if version != "" && version != cr.Version {
			slog.Error(
				"Not all CRD have the same version",
				"group-a", crdGroup, "version-a", version, "kind-a", crdKind,
				"group-b", cr.Group, "version-b", cr.Version, "kind-b", cr.Kind,
			)
			return
		}
		version = cr.Version
		crdKind = cr.Kind
		crdGroup = cr.Group

		// Generate types code
		typesCode := generateTypesCode(cr)

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
	gvi, err := generateGroupVersionInfoCode(crdGroup, version, names)
	if err != nil {
		slog.Error("Error writing group_version_kind.go", "error", err)
		return
	}

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
		dir := filepath.Dir(f.name)

		// Create the directory if it doesn't exist
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			slog.Error("Error creating directory", "error", err)
			return
		}

		if err := os.WriteFile(f.name, []byte(f.content), 0o644); err != nil {
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
