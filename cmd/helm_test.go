package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelmCmd_Structure(t *testing.T) {
	assert.NotNil(t, helmCmd)
	assert.Equal(t, "helm", helmCmd.Use)
	assert.NotEmpty(t, helmCmd.Short)
	assert.NotEmpty(t, helmCmd.Long)
	assert.NotEmpty(t, helmCmd.Example)
}

func TestHelmCmd_RunE(t *testing.T) {
	assert.NotNil(t, helmCmd.RunE, "helm command should have RunE function")
}

func TestHelmCmd_DisableFlagParsing(t *testing.T) {
	assert.True(t, helmCmd.DisableFlagParsing, "helm command should have DisableFlagParsing=true")
}

func TestHelmCmd_Examples(t *testing.T) {
	examples := helmCmd.Example
	assert.Contains(t, examples, "helm list")
	assert.Contains(t, examples, "helm install")
	assert.Contains(t, examples, "helm upgrade")
	assert.Contains(t, examples, "helm repo add")
	assert.Contains(t, examples, "helm search")
	assert.Contains(t, examples, "helm status")
	assert.Contains(t, examples, "helm uninstall")
	assert.Contains(t, examples, "--kubeconfig")
}

func TestHelmCmd_LongDescription(t *testing.T) {
	long := helmCmd.Long
	assert.Contains(t, long, "Helm")
	assert.Contains(t, long, "binary")
	assert.Contains(t, long, "PATH")
	assert.Contains(t, long, "kubeconfig")
	assert.Contains(t, long, "KUBECONFIG")
	assert.Contains(t, long, ".kube/config")
}

func TestHelmCmd_ShortDescription(t *testing.T) {
	short := helmCmd.Short
	assert.Contains(t, short, "Helm")
	assert.Contains(t, short, "helm")
	assert.Contains(t, short, "PATH")
}

func TestHelmCmd_RegisteredWithRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "helm" {
			found = true
			break
		}
	}
	assert.True(t, found, "helm command should be registered with root")
}

func TestHelmCmd_RequiresBinary(t *testing.T) {
	long := helmCmd.Long
	assert.Contains(t, long, "requires", "helm command should mention it requires helm binary")
	assert.Contains(t, long, "https://helm.sh", "helm command should mention installation URL")
}

func TestHelmCmd_KubeconfigOptions(t *testing.T) {
	long := helmCmd.Long
	// Should document all kubeconfig resolution methods
	assert.Contains(t, long, "--kubeconfig")
	assert.Contains(t, long, "KUBECONFIG")
	assert.Contains(t, long, "~/.kube/config")
}

func TestHelmCmd_ExampleCommands(t *testing.T) {
	examples := helmCmd.Example

	// Should show various helm operations
	helmOperations := []string{
		"list",
		"install",
		"upgrade",
		"repo add",
		"search",
		"status",
		"uninstall",
	}

	for _, op := range helmOperations {
		assert.Contains(t, examples, op, "Should show example for helm %s", op)
	}
}

func TestHelmCmd_BinaryCheck(t *testing.T) {
	// Test that runHelm function exists
	assert.NotNil(t, runHelm, "runHelm function should be defined")
}

func TestHelmCmd_UsesKubeconfigPath(t *testing.T) {
	long := helmCmd.Long
	// Should mention automatic kubeconfig usage
	assert.Contains(t, long, "automatically", "Should mention automatic kubeconfig handling")
}

func TestHelmCmd_SupportsAllHelmCommands(t *testing.T) {
	long := helmCmd.Long
	assert.Contains(t, long, "All standard Helm", "Should mention support for all Helm commands")
	assert.Contains(t, long, "v3", "Should mention Helm v3 support")
}
