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

	slog.With("target", target, "crd", crds, "version", version).Info("generate-crd-api")
	defer println()

	resources, success := openapi.Parse(crds, version)
	if !success {
		return
	}

	var files []outFile
	for _, cr := range resources.Items {
		// Generate types code
		typesCode, err := generateTypesCode(cr, resources.Group, resources.Version)
		if err != nil {
			slog.Error("Error generating types content", "error", err)
			return
		}

		// Write output file
		outputFile := filepath.Join(target, resources.Version, fmt.Sprintf("types_%s.go", strings.ToLower(cr.Kind)))
		files = append(files, outFile{
			name:       outputFile,
			content:    typesCode,
			successMsg: "Successfully generated Go structs",
			successArgs: []any{
				"group", resources.Group,
				"version", resources.Version,
				"kind", cr.Kind,
				"file", outputFile,
			},
		})
	}

	// Generate GroupVersionInfo code
	gvi, err := generateGroupVersionInfoCode(resources)
	if err != nil {
		slog.Error("Error writing group_version_kind.go", "error", err)
		return
	}

	// Write output file
	outputFile := filepath.Join(target, resources.Version, "group_version_info.go")

	files = append(files, outFile{
		name:       outputFile,
		content:    gvi,
		successMsg: "Successfully generated GroupVersionInfo",
		successArgs: []any{
			"group", resources.Group, "version", resources.Version, "file", outputFile,
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
