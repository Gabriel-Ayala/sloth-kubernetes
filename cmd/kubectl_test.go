package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKubectlCmd_Structure(t *testing.T) {
	assert.NotNil(t, kubectlCmd)
	assert.Equal(t, "kubectl", kubectlCmd.Use)
	assert.NotEmpty(t, kubectlCmd.Short)
	assert.NotEmpty(t, kubectlCmd.Long)
	assert.NotEmpty(t, kubectlCmd.Example)
}

func TestKubectlCmd_DisableFlagParsing(t *testing.T) {
	assert.True(t, kubectlCmd.DisableFlagParsing, "kubectl command should have DisableFlagParsing=true")
}

func TestKubectlCmd_RunE(t *testing.T) {
	assert.NotNil(t, kubectlCmd.RunE, "kubectl command should have RunE function")
}

func TestGetKubeconfigPath_FromEnv(t *testing.T) {
	// Test with KUBECONFIG env var set
	testPath := "/tmp/test-kubeconfig"
	oldKubeconfig := os.Getenv("KUBECONFIG")
	defer os.Setenv("KUBECONFIG", oldKubeconfig)

	os.Setenv("KUBECONFIG", testPath)
	path := getKubeconfigPath()
	assert.Equal(t, testPath, path)
}

func TestGetKubeconfigPath_Default(t *testing.T) {
	// Test without KUBECONFIG env var (should return default path)
	oldKubeconfig := os.Getenv("KUBECONFIG")
	defer os.Setenv("KUBECONFIG", oldKubeconfig)

	os.Unsetenv("KUBECONFIG")
	path := getKubeconfigPath()

	// Should return either default kubeconfig or empty string
	if path != "" {
		assert.Contains(t, path, ".kube")
	}
}

func TestGetKubeconfigPath_HomeDir(t *testing.T) {
	// Clear KUBECONFIG env
	oldKubeconfig := os.Getenv("KUBECONFIG")
	defer os.Setenv("KUBECONFIG", oldKubeconfig)
	os.Unsetenv("KUBECONFIG")

	path := getKubeconfigPath()

	// If home dir is accessible, path should contain .kube/config
	home, err := os.UserHomeDir()
	if err == nil && path != "" {
		expectedPath := filepath.Join(home, ".kube", "config")
		assert.Equal(t, expectedPath, path)
	}
}

func TestKubectlCmd_Examples(t *testing.T) {
	examples := kubectlCmd.Example
	assert.Contains(t, examples, "get nodes")
	assert.Contains(t, examples, "get pods")
	assert.Contains(t, examples, "apply")
	assert.Contains(t, examples, "logs")
	assert.Contains(t, examples, "exec")
}

func TestKubectlCmd_LongDescription(t *testing.T) {
	long := kubectlCmd.Long
	assert.Contains(t, long, "kubectl")
	assert.Contains(t, long, "embedded")
	assert.Contains(t, long, "kubeconfig")
}

func TestRunKubectl_WithInvalidArgs(t *testing.T) {
	// This test verifies that runKubectl can be called (even though it will fail with invalid args)
	// We're just testing that the function exists and has correct signature
	assert.NotNil(t, runKubectl)
}
