package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "update golden files")

func TestGenerateCrdApiE2E(t *testing.T) {
	tempDir := t.TempDir()

	wd, err := os.Getwd()
	require.NoError(t, err)
	testdata := filepath.Join(wd, "..", "..", "testdata")

	testCases := []struct {
		name               string
		args               []string
		wantErrMsg         string
		expectedFiles      []string
		fileContentChecks  map[string][]string
		expectedFileGolden map[string]string
	}{
		{
			name:       "no_crds_defined",
			args:       []string{},
			wantErrMsg: `at least one CRD must be defined`,
		},
		{
			name:       "target_not_defined",
			args:       []string{"--crd", filepath.Join(testdata, "a.yaml")},
			wantErrMsg: `required flag(s) "target" not set`,
		},
		{
			name: "single_crd",
			args: []string{"--crd", filepath.Join(testdata, "tenants.capsule.clastix.io.yaml")},
			expectedFiles: []string{
				"v1beta2/group_version_info.go",
				"v1beta2/types_tenant.go",
			},
			fileContentChecks: map[string][]string{
				"v1beta2/types_tenant.go": {
					"package v1beta2",
					"type Tenant struct {",
					"Spec TenantSpec",
				},
				"v1beta2/group_version_info.go": {
					`GroupVersion = schema.GroupVersion{Group: "capsule.clastix.io", Version: "v1beta2"}`,
				},
			},
		},
		{
			name: "multiple_crds_same_group",
			args: []string{
				"--crd", filepath.Join(testdata, "certificates.cert-manager.io.yaml"),
				"--crd", filepath.Join(testdata, "clusterissuers.cert-manager.io.yaml"),
			},
			expectedFiles: []string{
				"v1/group_version_info.go",
				"v1/types_certificate.go",
				"v1/types_clusterissuer.go",
			},
		},
		{
			name: "multiple_crds_different_group",
			args: []string{
				"--crd", filepath.Join(testdata, "tenants.capsule.clastix.io.yaml"),
				"--crd", filepath.Join(testdata, "applications.argoproj.io.yaml"),
			},
			wantErrMsg: "failed to parse CRDs",
		},
		{
			name: "with_version_not_storage",
			args: []string{
				"--crd", filepath.Join(testdata, "tenants.capsule.clastix.io.yaml"),
				"--version", "v1beta1",
			},
			wantErrMsg: `failed to parse CRDs`,
		},
		{
			name: "with_pointers",
			args: []string{
				"--crd", filepath.Join(testdata, "tenants.capsule.clastix.io.yaml"),
				"--pointer",
			},
			expectedFiles: []string{
				"v1beta2/group_version_info.go",
				"v1beta2/types_tenant.go",
			},
			fileContentChecks: map[string][]string{
				"v1beta2/types_tenant.go": {"Quota *int32"},
			},
		},
		{
			name: "with_invalid_crd",
			args: []string{
				"--crd", filepath.Join(testdata, "a.yaml"),
			},
			wantErrMsg: "failed to parse CRDs",
		},
		{
			name: "all_cases",
			args: []string{"--crd", filepath.Join(testdata, "all-cases.testing.crd-gen.yaml")},
			expectedFileGolden: map[string]string{
				"v1/group_version_info.go": filepath.Join(testdata, "expected", "all-cases", "group_version_info.go.txt"),
				"v1/types_allcase.go":      filepath.Join(testdata, "expected", "all-cases", "types_allcase.go.txt"),
			},
		},
		{
			name: "all_cases_pointers",
			args: []string{"--crd", filepath.Join(testdata, "all-cases.testing.crd-gen.yaml"), "--pointer"},
			expectedFileGolden: map[string]string{
				"v1/group_version_info.go": filepath.Join(testdata, "expected", "all-cases", "group_version_info.go.txt"),
				"v1/types_allcase.go":      filepath.Join(testdata, "expected", "all-cases", "types_allcase_pointers.go.txt"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			crds = nil
			target = ""
			version = ""
			pointers = false

			targetDir := filepath.Join(tempDir, tc.name)
			require.NoError(t, os.Mkdir(targetDir, 0o755))

			rootCmd := newRootCmd()
			b := new(bytes.Buffer)
			rootCmd.SetOut(b)
			rootCmd.SetErr(b)

			finalArgs := tc.args
			if tc.name != "target_not_defined" {
				finalArgs = append(finalArgs, "--target", targetDir)
			}
			rootCmd.SetArgs(finalArgs)

			err := rootCmd.Execute()

			if tc.wantErrMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrMsg)
			} else {
				require.NoError(t, err)

				if len(tc.expectedFiles) > 0 {
					for _, file := range tc.expectedFiles {
						assert.FileExists(t, filepath.Join(targetDir, file))
					}
				}

				if len(tc.fileContentChecks) > 0 {
					for file, contents := range tc.fileContentChecks {
						data, err := os.ReadFile(filepath.Join(targetDir, file))
						require.NoError(t, err)
						for _, content := range contents {
							assert.Contains(t, string(data), content)
						}
					}
				}
				if len(tc.expectedFileGolden) > 0 {
					for genFile, goldenFile := range tc.expectedFileGolden {
						generated, err := os.ReadFile(filepath.Join(targetDir, genFile))
						require.NoError(t, err)

						if *update {
							require.NoError(t, os.MkdirAll(filepath.Dir(goldenFile), 0o755))
							require.NoError(t, os.WriteFile(goldenFile, generated, 0o644))
						}

						expected, err := os.ReadFile(goldenFile)
						require.NoError(t, err)

						assert.Equal(
							t,
							string(expected),
							string(generated),
							"generated file %s does not match golden file %s",
							genFile,
							goldenFile,
						)
					}
				}
			}
		})
	}
}
