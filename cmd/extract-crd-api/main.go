package main

import (
	"bytes"
	"context"
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
	includeFlags []string
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
	rootCmd.Flags().
		StringSliceVarP(&excludeFlags, "exclude", "e", nil, "Regex pattern for file excludes (not considered if includes are defined)")
	rootCmd.Flags().StringSliceVarP(&includeFlags, "include", "i", nil, "Regex pattern for file includes")
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

func run(cmd *cobra.Command, _ []string) error {
	var includes, excludes []*regexp.Regexp
	l := slog.With("target", target, "path", path, "module", module,
		"clear", clearTarget, "use-git", useGit)
	if len(includeFlags) > 0 {
		for _, includeFlag := range includeFlags {
			includes = append(includes, regexp.MustCompile(includeFlag))
		}
		l = l.With("include", includeFlags)
	} else {
		for _, excludeFlag := range excludeFlags {
			excludes = append(excludes, regexp.MustCompile(excludeFlag))
		}
		l = l.With("exclude", excludeFlags)
	}

	l.InfoContext(cmd.Context(), "generate-crd-api")
	defer fmt.Println()

	tmp, err := os.MkdirTemp("", "extract-crd-api")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	moduleRoot := tmp

	if useGit {
		slog.With("module", module, "tmp", tmp).InfoContext(cmd.Context(), "Cloning module")
		info := strings.Split(module, "@")

		var out bytes.Buffer
		r, err := git.PlainClone(tmp, false, &git.CloneOptions{
			URL:      "https://" + info[0],
			Progress: &out,
		})
		slog.DebugContext(cmd.Context(), "Git clone output", "output", out.String())
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
		goCmd := exec.CommandContext(cmd.Context(), "go", "mod", "download", module)
		goCmd.Stdout = &execOut
		goCmd.Stderr = &execErr

		goCmd.Env = append(os.Environ(), "GOMODCACHE="+tmp)

		slog.With("module", module, "tmp", tmp).InfoContext(cmd.Context(), "Downloading")
		err = goCmd.Run()
		slog.DebugContext(cmd.Context(), "go mod download output", "output", execOut.String())
		if err != nil {
			return fmt.Errorf("failed to download module: %w\nstdout: %s\nstderr: %s",
				err, execOut.String(), execErr.String())
		}

		execOut = bytes.Buffer{}
		execErr = bytes.Buffer{}
		chmodCmd := exec.CommandContext(cmd.Context(), "chmod", "+w", "-R", tmp)
		chmodCmd.Stdout = &execOut
		chmodCmd.Stderr = &execErr
		err = chmodCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to set permissions: %w\nstdout: %s\nstderr: %s",
				err, execOut.String(), execErr.String())
		}
		moduleRoot = filepath.Join(tmp, module)
	}
	slog.InfoContext(cmd.Context(), "Module downloaded successfully!")

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
		if keep(e.Name(), includes, excludes) {
			err = copyFile(cmd.Context(), filepath.Join(apiPath, e.Name()), filepath.Join(target, e.Name()))
			if err != nil {
				return fmt.Errorf("failed to copy file %s: %w", e.Name(), err)
			}
		}
	}
	return nil
}

func keep(name string, includes, excludes []*regexp.Regexp) bool {
	if len(includes) > 0 {
		for _, include := range includes {
			if include.MatchString(name) {
				return true
			}
		}
		return false
	}
	for _, exclude := range excludes {
		if exclude.MatchString(name) {
			return false
		}
	}
	return true
}

func copyFile(ctx context.Context, src, dst string) error {
	slog.With("from", src, "to", dst).InfoContext(ctx, "Copy file")
	// Read all content of src to data, may cause OOM for a large file.
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	// Write data to dst
	return os.WriteFile(dst, data, 0o644)
}
