package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKubeconfigCmd_Structure(t *testing.T) {
	assert.NotNil(t, kubeconfigCmd)
	assert.Equal(t, "kubeconfig [stack-name]", kubeconfigCmd.Use)
	assert.NotEmpty(t, kubeconfigCmd.Short)
	assert.NotEmpty(t, kubeconfigCmd.Long)
	assert.NotEmpty(t, kubeconfigCmd.Example)
}

func TestKubeconfigCmd_RunE(t *testing.T) {
	assert.NotNil(t, kubeconfigCmd.RunE, "kubeconfig command should have RunE function")
}

func TestKubeconfigCmd_Flags(t *testing.T) {
	outputFlag := kubeconfigCmd.Flags().Lookup("output")
	assert.NotNil(t, outputFlag, "kubeconfig should have --output flag")
	assert.Equal(t, "o", outputFlag.Shorthand, "output flag should have 'o' shorthand")

	mergeFlag := kubeconfigCmd.Flags().Lookup("merge")
	assert.NotNil(t, mergeFlag, "kubeconfig should have --merge flag")
	assert.Equal(t, "false", mergeFlag.DefValue, "merge flag should default to false")
}

func TestKubeconfigCmd_Examples(t *testing.T) {
	examples := kubeconfigCmd.Example
	assert.Contains(t, examples, "kubeconfig")
	assert.Contains(t, examples, "stdout")
	assert.Contains(t, examples, "-o")
	assert.Contains(t, examples, ".kube/config")
}

func TestKubeconfigCmd_LongDescription(t *testing.T) {
	long := kubeconfigCmd.Long
	assert.Contains(t, long, "kubeconfig")
	assert.Contains(t, long, "Kubernetes")
	assert.Contains(t, long, "cluster")
	assert.Contains(t, long, "stdout")
	assert.Contains(t, long, "file")
}

func TestKubeconfigCmd_ShortDescription(t *testing.T) {
	short := kubeconfigCmd.Short
	assert.Contains(t, short, "kubeconfig")
	assert.Contains(t, short, "kubectl")
}

func TestKubeconfigCmd_OutputOptions(t *testing.T) {
	examples := kubeconfigCmd.Example
	// Should show both stdout and file output options
	assert.Contains(t, examples, "Print to stdout")
	assert.Contains(t, examples, "Save to file")
}

func TestKubeconfigCmd_RegisteredWithRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "kubeconfig" {
			found = true
			break
		}
	}
	assert.True(t, found, "kubeconfig command should be registered with root")
}

func TestKubeconfigCmd_GlobalVariables(t *testing.T) {
	// Verify global variables are of correct type
	assert.IsType(t, "", outputFile)
	assert.IsType(t, false, merge)
}

func TestKubeconfigCmd_FlagDefaults(t *testing.T) {
	outputFlag := kubeconfigCmd.Flags().Lookup("output")
	assert.Equal(t, "", outputFlag.DefValue, "output flag should default to empty (stdout)")

	mergeFlag := kubeconfigCmd.Flags().Lookup("merge")
	assert.Equal(t, "false", mergeFlag.DefValue, "merge flag should default to false")
}

func TestKubeconfigCmd_ExampleFormats(t *testing.T) {
	examples := kubeconfigCmd.Example
	// Should demonstrate various output formats
	assert.Contains(t, examples, "sloth-kubernetes kubeconfig")
	assert.Contains(t, examples, "-o")
}

func TestRunKubeconfig_Exists(t *testing.T) {
	// Test that runKubeconfig function exists
	assert.NotNil(t, runKubeconfig, "runKubeconfig function should be defined")
}

func TestKubeconfigCmd_DefaultBehavior(t *testing.T) {
	examples := kubeconfigCmd.Example

	// Should document that default is stdout
	assert.Contains(t, examples, "Print to stdout")

	// Output flag should be optional
	outputFlag := kubeconfigCmd.Flags().Lookup("output")
	assert.False(t, outputFlag.Changed, "output flag should not be required")
}

func TestKubeconfigCmd_MergeFeature(t *testing.T) {
	mergeFlag := kubeconfigCmd.Flags().Lookup("merge")
	assert.NotNil(t, mergeFlag)

	// Check that merge flag has usage text
	assert.NotEmpty(t, mergeFlag.Usage, "merge flag should have usage text")
}
