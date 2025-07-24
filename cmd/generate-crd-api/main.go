package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/bakito/crd-gen/internal/openapi"
	"github.com/bakito/crd-gen/internal/render"
)

var (
	crds     []string
	target   string
	version  string
	pointers bool

	clientConfig clientcmd.ClientConfig

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

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := clientcmd.ConfigOverrides{}
	clientConfig = clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, _ []string) error {
	if len(crds) == 0 {
		return errors.New("at least one CRD must be defined")
	}

	slog.With("target", target, "crd", crds, "version", version).InfoContext(cmd.Context(), "generate-crd-api")
	defer fmt.Println()

	resources, success := openapi.Parse(cmd.Context(), clientConfig, crds, version, pointers)
	if !success {
		return errors.New("failed to parse CRDs")
	}

	return render.WriteCrdFiles(cmd.Context(), resources, target)
}
