package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

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
	crds    []string
	target  string
	version string

	rootCmd = &cobra.Command{
		Use:   "generate-crd-api",
		Short: "Generate Go API code from CRD files",
		RunE:  run,
	}
)

func init() {
	rootCmd.Flags().StringSliceVar(&crds, "crd", nil, "CRD file to process")
	rootCmd.Flags().StringVar(&target, "target", "", "The target directory to copyFile the files to")
	rootCmd.Flags().
		StringVar(&version, "version", "", "The version to select from the CRD; If not defined, the first version is used")
	_ = rootCmd.MarkFlagRequired("target")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	if len(crds) == 0 {
		return errors.New("at least one CRD must be defined")
	}

	slog.With("target", target, "crd", crds, "version", version).Info("generate-crd-api")
	defer println()

	resources, success := openapi.Parse(crds, version)
	if !success {
		return errors.New("failed to parse CRDs")
	}

	var files []outFile
	for _, cr := range resources.Items {
		// Generate types code
		typesCode, err := generateTypesCode(cr, resources.Group, resources.Version)
		if err != nil {
			return fmt.Errorf("error generating types content: %w", err)
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
		return fmt.Errorf("error writing group_version_kind.go: %w", err)
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

	return writeFiles(files)
}

func writeFiles(files []outFile) error {
	for _, f := range files {
		dir := filepath.Dir(f.name)

		// Create the directory if it doesn't exist
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return fmt.Errorf("error creating directory: %w", err)
		}

		if err := os.WriteFile(f.name, []byte(f.content), 0o644); err != nil {
			return fmt.Errorf("error writing output file: %w", err)
		}

		slog.With(f.successArgs...).Info(f.successMsg)
	}
	return nil
}

type outFile struct {
	name        string
	content     string
	successMsg  string
	successArgs []any
}
