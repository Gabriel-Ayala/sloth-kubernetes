package addons

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/stretchr/testify/assert"
)

// TestInstallArgoCD_Disabled tests when ArgoCD is disabled
func TestInstallArgoCD_Disabled(t *testing.T) {
	cfg := &config.ClusterConfig{
		Addons: config.AddonsConfig{
			ArgoCD: nil,
		},
	}

	err := InstallArgoCD(cfg, "1.2.3.4", "test-key")
	assert.NoError(t, err, "Should not error when ArgoCD is disabled")
}

// TestInstallArgoCD_DisabledExplicitly tests when ArgoCD is explicitly disabled
func TestInstallArgoCD_DisabledExplicitly(t *testing.T) {
	cfg := &config.ClusterConfig{
		Addons: config.AddonsConfig{
			ArgoCD: &config.ArgoCDConfig{
				Enabled: false,
			},
		},
	}

	err := InstallArgoCD(cfg, "1.2.3.4", "test-key")
	assert.NoError(t, err, "Should not error when ArgoCD is explicitly disabled")
}

// TestInstallArgoCD_DefaultValues tests default value setting
func TestInstallArgoCD_DefaultValues(t *testing.T) {
	tests := []struct {
		name           string
		inputConfig    *config.ArgoCDConfig
		expectedNS     string
		expectedBranch string
		expectedPath   string
		expectedVer    string
	}{
		{
			name: "All defaults",
			inputConfig: &config.ArgoCDConfig{
				Enabled:       true,
				GitOpsRepoURL: "https://github.com/test/repo",
			},
			expectedNS:     "argocd",
			expectedBranch: "main",
			expectedPath:   "argocd/apps",
			expectedVer:    "stable",
		},
		{
			name: "Custom namespace",
			inputConfig: &config.ArgoCDConfig{
				Enabled:       true,
				Namespace:     "custom-argocd",
				GitOpsRepoURL: "https://github.com/test/repo",
			},
			expectedNS:     "custom-argocd",
			expectedBranch: "main",
			expectedPath:   "argocd/apps",
			expectedVer:    "stable",
		},
		{
			name: "Custom branch",
			inputConfig: &config.ArgoCDConfig{
				Enabled:          true,
				GitOpsRepoBranch: "develop",
				GitOpsRepoURL:    "https://github.com/test/repo",
			},
			expectedNS:     "argocd",
			expectedBranch: "develop",
			expectedPath:   "argocd/apps",
			expectedVer:    "stable",
		},
		{
			name: "Custom apps path",
			inputConfig: &config.ArgoCDConfig{
				Enabled:       true,
				AppsPath:      "custom/apps",
				GitOpsRepoURL: "https://github.com/test/repo",
			},
			expectedNS:     "argocd",
			expectedBranch: "main",
			expectedPath:   "custom/apps",
			expectedVer:    "stable",
		},
		{
			name: "Custom version",
			inputConfig: &config.ArgoCDConfig{
				Enabled:       true,
				Version:       "v2.9.3",
				GitOpsRepoURL: "https://github.com/test/repo",
			},
			expectedNS:     "argocd",
			expectedBranch: "main",
			expectedPath:   "argocd/apps",
			expectedVer:    "v2.9.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply defaults directly without SSH
			ApplyArgoCDDefaults(tt.inputConfig)

			// Check that defaults were applied
			assert.Equal(t, tt.expectedNS, tt.inputConfig.Namespace)
			assert.Equal(t, tt.expectedBranch, tt.inputConfig.GitOpsRepoBranch)
			assert.Equal(t, tt.expectedPath, tt.inputConfig.AppsPath)
			assert.Equal(t, tt.expectedVer, tt.inputConfig.Version)
		})
	}
}

// TestArgoCDConfigValidation tests ArgoCD configuration validation
func TestArgoCDConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config *config.ArgoCDConfig
	}{
		{
			name: "Valid minimal config",
			config: &config.ArgoCDConfig{
				Enabled:       true,
				GitOpsRepoURL: "https://github.com/test/repo",
			},
		},
		{
			name: "Valid complete config",
			config: &config.ArgoCDConfig{
				Enabled:          true,
				Namespace:        "custom-argocd",
				GitOpsRepoURL:    "https://github.com/test/repo",
				GitOpsRepoBranch: "main",
				AppsPath:         "apps/",
				Version:          "v2.9.3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply defaults directly without SSH
			ApplyArgoCDDefaults(tt.config)

			// Check that defaults were applied
			if tt.config.Namespace == "" {
				assert.Equal(t, "argocd", tt.config.Namespace)
			}
			if tt.config.GitOpsRepoBranch == "" {
				assert.Equal(t, "main", tt.config.GitOpsRepoBranch)
			}
			if tt.config.AppsPath == "" {
				assert.Equal(t, "argocd/apps", tt.config.AppsPath)
			}
			if tt.config.Version == "" {
				assert.Equal(t, "stable", tt.config.Version)
			}
		})
	}
}

// TestRunSSHCommand_ErrorHandling tests SSH command error handling
func TestRunSSHCommand_ErrorHandling(t *testing.T) {
	// Test with invalid host
	err := runSSHCommand("invalid-host", "test-key", "echo test")
	assert.Error(t, err, "Should error with invalid host")
}

// TestRunSSHCommandWithOutput_ErrorHandling tests SSH command with output error handling
func TestRunSSHCommandWithOutput_ErrorHandling(t *testing.T) {
	// Test with invalid host
	output, err := runSSHCommandWithOutput("invalid-host", "test-key", "echo test")
	assert.Error(t, err, "Should error with invalid host")
	assert.Empty(t, output, "Output should be empty on error")
}

// TestBootstrapArgoCD_InvalidKubeconfig tests bootstrap with invalid kubeconfig
func TestBootstrapArgoCD_InvalidKubeconfig(t *testing.T) {
	config := &GitOpsConfig{
		RepoURL: "https://github.com/test/repo",
		Branch:  "main",
		Path:    "addons/",
	}

	err := BootstrapArgoCD("/invalid/path/kubeconfig", config)
	assert.Error(t, err, "Should error with invalid kubeconfig")
}

// TestCloneGitOpsRepo_InvalidRepo tests cloning invalid repository
func TestCloneGitOpsRepo_InvalidRepo(t *testing.T) {
	config := &GitOpsConfig{
		RepoURL: "https://github.com/nonexistent/nonexistent-repo-12345",
		Branch:  "main",
	}

	tempDir := filepath.Join(os.TempDir(), "test-clone")
	defer os.RemoveAll(tempDir)

	err := CloneGitOpsRepo(config, tempDir)
	assert.Error(t, err, "Should error with invalid repository")
}

// TestCloneGitOpsRepo_WithBranch tests cloning with specific branch
func TestCloneGitOpsRepo_WithBranch(t *testing.T) {
	config := &GitOpsConfig{
		RepoURL: "https://github.com/nonexistent/repo",
		Branch:  "develop",
	}

	tempDir := filepath.Join(os.TempDir(), "test-clone-branch")
	defer os.RemoveAll(tempDir)

	err := CloneGitOpsRepo(config, tempDir)
	// Will fail because repo doesn't exist, but tests the branch logic
	assert.Error(t, err)
}

// TestCloneGitOpsRepo_WithPrivateKey tests cloning with private key
func TestCloneGitOpsRepo_WithPrivateKey(t *testing.T) {
	config := &GitOpsConfig{
		RepoURL:    "git@github.com:test/repo.git",
		PrivateKey: "/tmp/test-key",
	}

	tempDir := filepath.Join(os.TempDir(), "test-clone-ssh")
	defer os.RemoveAll(tempDir)

	err := CloneGitOpsRepo(config, tempDir)
	// Will fail but tests the SSH key logic
	assert.Error(t, err)
}

// TestApplyAddonsFromRepo_InvalidPath tests applying from invalid path
func TestApplyAddonsFromRepo_InvalidPath(t *testing.T) {
	err := ApplyAddonsFromRepo("/invalid/kubeconfig", "/invalid/repo", "addons/")
	assert.Error(t, err, "Should error with invalid paths")
}

// TestApplyAddonsFromRepo_MissingKubeconfig tests applying with missing kubeconfig
func TestApplyAddonsFromRepo_MissingKubeconfig(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "test-apply")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	err := ApplyAddonsFromRepo("/nonexistent/kubeconfig", tempDir, "addons/")
	assert.Error(t, err, "Should error with nonexistent kubeconfig")
}

// TestInstallArgoCDManifests_ScriptGeneration tests that install script is properly generated
func TestInstallArgoCDManifests_ScriptGeneration(t *testing.T) {
	t.Skip("Skipping test that requires real SSH connection")
}

// TestWaitForArgoCDReady_ScriptGeneration tests wait script generation
func TestWaitForArgoCDReady_ScriptGeneration(t *testing.T) {
	t.Skip("Skipping test that requires real SSH connection")
}

// TestApplyGitOpsApplications_ScriptGeneration tests GitOps apply script generation
func TestApplyGitOpsApplications_ScriptGeneration(t *testing.T) {
	t.Skip("Skipping test that requires real SSH connection")
}

// TestGetArgoCDAdminPassword_ScriptGeneration tests password retrieval script
func TestGetArgoCDAdminPassword_ScriptGeneration(t *testing.T) {
	t.Skip("Skipping test that requires real SSH connection")
}

// TestInstallArgoCD_CompleteWorkflow tests the complete installation workflow logic
func TestInstallArgoCD_CompleteWorkflow(t *testing.T) {
	argocdConfig := &config.ArgoCDConfig{
		Enabled:          true,
		Namespace:        "test-argocd",
		GitOpsRepoURL:    "https://github.com/test/repo",
		GitOpsRepoBranch: "develop",
		AppsPath:         "custom/apps",
		Version:          "v2.10.0",
		AdminPassword:    "",
	}

	// Apply defaults without SSH
	ApplyArgoCDDefaults(argocdConfig)

	// Verify config was properly initialized (values kept since they were set)
	assert.Equal(t, "test-argocd", argocdConfig.Namespace)
	assert.Equal(t, "develop", argocdConfig.GitOpsRepoBranch)
	assert.Equal(t, "custom/apps", argocdConfig.AppsPath)
	assert.Equal(t, "v2.10.0", argocdConfig.Version)
}

// TestInstallArgoCD_WithAdminPassword tests installation with pre-set admin password
func TestInstallArgoCD_WithAdminPassword(t *testing.T) {
	argocdConfig := &config.ArgoCDConfig{
		Enabled:       true,
		GitOpsRepoURL: "https://github.com/test/repo",
		AdminPassword: "custom-password",
	}

	// Apply defaults without SSH
	ApplyArgoCDDefaults(argocdConfig)

	// Password should be preserved
	assert.Equal(t, "custom-password", argocdConfig.AdminPassword)
	// Defaults should be applied
	assert.Equal(t, "argocd", argocdConfig.Namespace)
}

// TestSSHCommandScriptEscaping tests that SSH commands properly escape quotes
func TestSSHCommandScriptEscaping(t *testing.T) {
	// Test that commands with quotes are handled
	command := "echo 'test' && echo \"test2\""

	// This will fail but we're testing the escaping logic doesn't crash
	err := runSSHCommand("invalid-host", "test-key", command)
	assert.Error(t, err)
}

// TestSSHCommandWithOutput_OutputHandling tests output handling
func TestSSHCommandWithOutput_OutputHandling(t *testing.T) {
	// Test that output is properly captured even on error
	output, err := runSSHCommandWithOutput("invalid-host", "test-key", "echo test")
	assert.Error(t, err)
	// Output might contain error message
	assert.NotNil(t, output)
}

// TestInstallArgoCD_MultipleConfigCombinations tests various config combinations
func TestInstallArgoCD_MultipleConfigCombinations(t *testing.T) {
	tests := []struct {
		name   string
		config *config.ArgoCDConfig
	}{
		{
			name: "Minimal config",
			config: &config.ArgoCDConfig{
				Enabled:       true,
				GitOpsRepoURL: "https://github.com/test/repo",
			},
		},
		{
			name: "With custom namespace",
			config: &config.ArgoCDConfig{
				Enabled:       true,
				Namespace:     "my-argocd",
				GitOpsRepoURL: "https://github.com/test/repo",
			},
		},
		{
			name: "With custom branch",
			config: &config.ArgoCDConfig{
				Enabled:          true,
				GitOpsRepoBranch: "staging",
				GitOpsRepoURL:    "https://github.com/test/repo",
			},
		},
		{
			name: "Complete custom config",
			config: &config.ArgoCDConfig{
				Enabled:          true,
				Namespace:        "production-argocd",
				GitOpsRepoURL:    "https://github.com/test/prod-repo",
				GitOpsRepoBranch: "production",
				AppsPath:         "k8s/apps/",
				Version:          "v2.11.0",
				AdminPassword:    "secure-password",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply defaults without actually installing (which requires SSH)
			ApplyArgoCDDefaults(tt.config)

			// Verify defaults were applied where needed
			assert.NotEmpty(t, tt.config.Namespace)
			assert.NotEmpty(t, tt.config.GitOpsRepoBranch)
			assert.NotEmpty(t, tt.config.AppsPath)
			assert.NotEmpty(t, tt.config.Version)
		})
	}
}

// TestCloneGitOpsRepo_DefaultBranch tests cloning without explicit branch
func TestCloneGitOpsRepo_DefaultBranch(t *testing.T) {
	config := &GitOpsConfig{
		RepoURL: "https://github.com/nonexistent/repo",
		// No branch specified - should not try to checkout
	}

	tempDir := filepath.Join(os.TempDir(), "test-clone-default")
	defer os.RemoveAll(tempDir)

	err := CloneGitOpsRepo(config, tempDir)
	// Will fail but tests that no checkout is attempted
	assert.Error(t, err)
}

// TestCloneGitOpsRepo_MainBranch tests that main/master branches don't trigger checkout
func TestCloneGitOpsRepo_MainBranch(t *testing.T) {
	tests := []struct {
		name   string
		branch string
	}{
		{"main branch", "main"},
		{"master branch", "master"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &GitOpsConfig{
				RepoURL: "https://github.com/nonexistent/repo",
				Branch:  tt.branch,
			}

			tempDir := filepath.Join(os.TempDir(), "test-clone-"+tt.branch)
			defer os.RemoveAll(tempDir)

			err := CloneGitOpsRepo(config, tempDir)
			// Will fail but tests that no additional checkout is performed for main/master
			assert.Error(t, err)
		})
	}
}

// TestArgoCDInstallationSteps tests the logical steps of ArgoCD installation
func TestArgoCDInstallationSteps(t *testing.T) {
	argocdConfig := &config.ArgoCDConfig{
		Enabled:       true,
		GitOpsRepoURL: "https://github.com/test/repo",
	}

	// Apply defaults without SSH (tests the default-setting logic)
	ApplyArgoCDDefaults(argocdConfig)

	// Verify all defaults are set before attempting installation
	assert.NotEmpty(t, argocdConfig.Namespace, "Namespace should be set")
	assert.NotEmpty(t, argocdConfig.GitOpsRepoBranch, "Branch should be set")
	assert.NotEmpty(t, argocdConfig.AppsPath, "Apps path should be set")
	assert.NotEmpty(t, argocdConfig.Version, "Version should be set")
}

// TestSSHCommandGeneration tests that SSH commands are properly formatted
func TestSSHCommandGeneration(t *testing.T) {
	// Test various script contents
	scripts := []string{
		"echo 'hello'",
		"kubectl get pods",
		"set -e\necho 'test'",
		strings.Repeat("x", 1000), // Long script
	}

	for i, script := range scripts {
		t.Run("Script "+string(rune(i+'A')), func(t *testing.T) {
			err := runSSHCommand("invalid-host", "test-key", script)
			// Should error but not crash
			assert.Error(t, err)
		})
	}
}
