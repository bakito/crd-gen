package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/bakito/crd-gen/internal/openapi"
	"github.com/bakito/crd-gen/internal/render"
)

var (
	crds     []string
	target   string
	version  string
	pointers bool

	rootCmd = &cobra.Command{
		Use:   "generate-crd-api",
		Short: "Generate Go API code from CRD files",
		RunE:  run,
	}
)

func init() {
	rootCmd.Flags().StringSliceVar(&crds, "crd", nil, "CRD file to process")
	rootCmd.Flags().StringVar(&target, "target", "", "The target directory to copyFile the files to")
	rootCmd.Flags().BoolVar(&pointers, "pointer", false, "If enabled, struct variables are generated as pointers")
	rootCmd.Flags().
		StringVar(&version, "version", "", "The version to select from the CRD; If not defined, the first version is used")
	_ = rootCmd.MarkFlagRequired("target")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(_ *cobra.Command, _ []string) error {
	if len(crds) == 0 {
		return errors.New("at least one CRD must be defined")
	}

	slog.With("target", target, "crd", crds, "version", version).Info("generate-crd-api")
	defer fmt.Println()

	resources, success := openapi.Parse(crds, version, pointers)
	if !success {
		return errors.New("failed to parse CRDs")
	}

	return render.WriteCrdFiles(resources, target)
}
