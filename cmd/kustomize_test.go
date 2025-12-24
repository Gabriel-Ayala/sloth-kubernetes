package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKustomizeCmd_Structure(t *testing.T) {
	assert.NotNil(t, kustomizeCmd)
	assert.Equal(t, "kustomize", kustomizeCmd.Use)
	assert.NotEmpty(t, kustomizeCmd.Short)
	assert.NotEmpty(t, kustomizeCmd.Long)
	assert.NotEmpty(t, kustomizeCmd.Example)
}

func TestKustomizeCmd_RunE(t *testing.T) {
	assert.NotNil(t, kustomizeCmd.RunE, "kustomize command should have RunE function")
}

func TestKustomizeCmd_DisableFlagParsing(t *testing.T) {
	assert.True(t, kustomizeCmd.DisableFlagParsing, "kustomize command should have DisableFlagParsing=true")
}

func TestKustomizeCmd_Examples(t *testing.T) {
	examples := kustomizeCmd.Example
	assert.Contains(t, examples, "kustomize build")
	assert.Contains(t, examples, "kustomize create")
	assert.Contains(t, examples, "kustomize edit")
	assert.Contains(t, examples, "add resource")
	assert.Contains(t, examples, "add configmap")
	assert.Contains(t, examples, "set image")
}

func TestKustomizeCmd_LongDescription(t *testing.T) {
	long := kustomizeCmd.Long
	assert.Contains(t, long, "Kustomize")
	assert.Contains(t, long, "binary")
	assert.Contains(t, long, "PATH")
	assert.Contains(t, long, "YAML")
	assert.Contains(t, long, "customize")
}

func TestKustomizeCmd_ShortDescription(t *testing.T) {
	short := kustomizeCmd.Short
	assert.Contains(t, short, "Kustomize")
	assert.Contains(t, short, "kustomize")
	assert.Contains(t, short, "PATH")
}

func TestKustomizeCmd_RegisteredWithRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "kustomize" {
			found = true
			break
		}
	}
	assert.True(t, found, "kustomize command should be registered with root")
}

func TestKustomizeCmd_RequiresBinary(t *testing.T) {
	long := kustomizeCmd.Long
	assert.Contains(t, long, "requires", "kustomize command should mention it requires kustomize binary")
	assert.Contains(t, long, "kubectl.docs.kubernetes.io", "kustomize command should mention installation URL")
}

func TestKustomizeCmd_BuildOperation(t *testing.T) {
	examples := kustomizeCmd.Example
	assert.Contains(t, examples, "build", "Should show build operation example")
	assert.Contains(t, examples, "overlays", "Should mention overlays")
}

func TestKustomizeCmd_CreateOperation(t *testing.T) {
	examples := kustomizeCmd.Example
	assert.Contains(t, examples, "create", "Should show create operation example")
	assert.Contains(t, examples, "autodetect", "Should mention autodetect flag")
}

func TestKustomizeCmd_EditOperations(t *testing.T) {
	examples := kustomizeCmd.Example
	editOperations := []string{
		"edit set image",
		"edit add resource",
		"edit add configmap",
	}

	for _, op := range editOperations {
		assert.Contains(t, examples, op, "Should show example for kustomize %s", op)
	}
}

func TestKustomizeCmd_PipelineExample(t *testing.T) {
	examples := kustomizeCmd.Example
	// Should show how to pipe kustomize output to kubectl
	assert.Contains(t, examples, "|", "Should show pipeline example")
	assert.Contains(t, examples, "kubectl apply", "Should show integration with kubectl")
}

func TestKustomizeCmd_YAMLCustomization(t *testing.T) {
	long := kustomizeCmd.Long
	assert.Contains(t, long, "template-free", "Should mention template-free YAML")
	assert.Contains(t, long, "untouched", "Should mention original YAML stays untouched")
}

func TestRunKustomize_Exists(t *testing.T) {
	// Test that runKustomize function exists
	assert.NotNil(t, runKustomize, "runKustomize function should be defined")
}

func TestKustomizeCmd_Purposes(t *testing.T) {
	long := kustomizeCmd.Long
	// Note: "multiple" and "purposes" are on separate lines in the Long description
	assert.Contains(t, long, "multiple", "Should mention multiple")
	assert.Contains(t, long, "purposes", "Should mention purposes")
	assert.Contains(t, long, "customize", "Should mention customization")
}
