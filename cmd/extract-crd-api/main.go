package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"
)

var (
	excludeFlags []string
	module       string
	path         string
	target       string
	clearTarget  = false
	useGit       = false
)

var rootCmd = &cobra.Command{
	Use:   "extract-crd-api",
	Short: "Extract CRD API files from a Go module",
	RunE:  run,
}

func init() {
	rootCmd.Flags().StringSliceVarP(&excludeFlags, "exclude", "e", nil, "Regex pattern for file excludes")
	rootCmd.Flags().StringVarP(&module, "module", "m", "", "The go module to get the api files from")
	rootCmd.Flags().StringVarP(&path, "path", "p", "", "The path within the module to the api files")
	rootCmd.Flags().StringVarP(&target, "target", "t", "", "The target directory to copyFile the files to")
	rootCmd.Flags().BoolVarP(&clearTarget, "clear", "c", false, "Clear target dir")
	rootCmd.Flags().BoolVarP(&useGit, "use-git", "g", false, "Use git instead of go mod (of module is not proper versioned)")

	_ = rootCmd.MarkFlagRequired("module")
	_ = rootCmd.MarkFlagRequired("path")
	_ = rootCmd.MarkFlagRequired("target")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	var excludes []*regexp.Regexp
	for _, excludeFlag := range excludeFlags {
		excludes = append(excludes, regexp.MustCompile(excludeFlag))
	}

	slog.With("target", target, "path", path, "module", module,
		"exclude", excludeFlags, "clear", clearTarget, "use-git", useGit).Info("generate-crd-api")
	defer fmt.Println()

	tmp, err := os.MkdirTemp("", "extract-crd-api")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	moduleRoot := tmp

	if useGit {
		slog.With("module", module, "tmp", tmp).Info("Cloning module")
		info := strings.Split(module, "@")

		var out bytes.Buffer
		r, err := git.PlainClone(tmp, false, &git.CloneOptions{
			URL:      "https://" + info[0],
			Progress: &out,
		})
		slog.Debug("Git clone output", "output", out.String())
		if err != nil {
			return fmt.Errorf("failed to clone module: %w", err)
		}
		w, err := r.Worktree()
		if err != nil {
			return fmt.Errorf("failed to get worktree: %w", err)
		}
		if len(info) > 0 {
			err = w.Checkout(&git.CheckoutOptions{
				Branch: plumbing.NewTagReferenceName(info[1]),
			})
			if err != nil {
				return fmt.Errorf("failed to checkout tag %s: %w", info[1], err)
			}
		}
	} else {
		var execOut bytes.Buffer
		var execErr bytes.Buffer
		cmd := exec.Command("go", "mod", "download", module)
		cmd.Stdout = &execOut
		cmd.Stderr = &execErr

		cmd.Env = append(os.Environ(), "GOMODCACHE="+tmp)

		slog.With("module", module, "tmp", tmp).Info("Downloading")
		err = cmd.Run()
		slog.Debug("go mod download output", "output", execOut.String())
		if err != nil {
			return fmt.Errorf("failed to download module: %w\nstdout: %s\nstderr: %s",
				err, execOut.String(), execErr.String())
		}

		execOut = bytes.Buffer{}
		execErr = bytes.Buffer{}

		cmd = exec.Command("chmod", "+w", "-R", tmp)
		cmd.Stdout = &execOut
		cmd.Stderr = &execErr
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to set permissions: %w\nstdout: %s\nstderr: %s",
				err, execOut.String(), execErr.String())
		}
		moduleRoot = filepath.Join(tmp, module)
	}
	slog.Info("Module downloaded successfully!")

	apiPath := filepath.Join(moduleRoot, path)
	entries, err := os.ReadDir(filepath.Join(moduleRoot, path))
	if err != nil {
		return fmt.Errorf("failed to read api path %s: %w", apiPath, err)
	}

	if clearTarget {
		_ = os.RemoveAll(target)
	}
	err = os.MkdirAll(target, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create target dir %s: %w", target, err)
	}

	for _, e := range entries {
		if keep(e.Name(), excludes) {
			err = copyFile(filepath.Join(apiPath, e.Name()), filepath.Join(target, e.Name()))
			if err != nil {
				return fmt.Errorf("failed to copy file %s: %w", e.Name(), err)
			}
		}
	}
	return nil
}

func keep(name string, excludes []*regexp.Regexp) bool {
	for _, exclude := range excludes {
		if exclude.MatchString(name) {
			return false
		}
	}
	return true
}

func copyFile(src, dst string) error {
	slog.With("from", src, "to", dst).Info("Copy file")
	// Read all content of src to data, may cause OOM for a large file.
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	// Write data to dst
	return os.WriteFile(dst, data, 0o644)
}
