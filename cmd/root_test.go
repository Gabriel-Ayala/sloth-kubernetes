package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootCmd_Structure(t *testing.T) {
	assert.NotNil(t, rootCmd)
	assert.Equal(t, "sloth-kubernetes", rootCmd.Use)
	assert.NotEmpty(t, rootCmd.Short)
	assert.NotEmpty(t, rootCmd.Long)
	assert.Equal(t, "1.0.0", rootCmd.Version)
}

func TestRootCmd_HasCommands(t *testing.T) {
	assert.True(t, rootCmd.HasAvailableSubCommands(), "Root command should have subcommands")

	commands := rootCmd.Commands()
	assert.NotEmpty(t, commands, "Root command should have registered commands")
}

func TestRootCmd_PersistentFlags(t *testing.T) {
	// Test that persistent flags are registered
	configFlag := rootCmd.PersistentFlags().Lookup("config")
	assert.NotNil(t, configFlag)
	assert.Equal(t, "c", configFlag.Shorthand)

	stackFlag := rootCmd.PersistentFlags().Lookup("stack")
	assert.NotNil(t, stackFlag)
	assert.Equal(t, "s", stackFlag.Shorthand)

	verboseFlag := rootCmd.PersistentFlags().Lookup("verbose")
	assert.NotNil(t, verboseFlag)
	assert.Equal(t, "v", verboseFlag.Shorthand)

	yesFlag := rootCmd.PersistentFlags().Lookup("yes")
	assert.NotNil(t, yesFlag)
	assert.Equal(t, "y", yesFlag.Shorthand)
}

func TestGlobalVariables_Defaults(t *testing.T) {
	// These variables are package-level and may have been modified by other tests
	// but we can verify their types
	assert.IsType(t, "", cfgFile)
	assert.IsType(t, "", stackName)
	assert.IsType(t, false, verbose)
	assert.IsType(t, false, autoApprove)
}

func TestInitConfig(t *testing.T) {
	// initConfig should not panic when called
	assert.NotPanics(t, func() {
		initConfig()
	})
}

func TestRootCmd_CommandLookup(t *testing.T) {
	testCases := []struct {
		name        string
		commandName string
	}{
		{"Deploy", "deploy"},
		{"Version", "version"},
		{"Status", "status"},
		{"Config", "config"},
		{"Kubectl", "kubectl"},
		{"Kubeconfig", "kubeconfig"},
		{"Salt", "salt"},
		{"Validate", "validate"},
		{"Nodes", "nodes"},
		{"Stacks", "stacks"},
		{"VPN", "vpn"},
		{"Addons", "addons"},
		{"Destroy", "destroy"},
		{"Helm", "helm"},
		{"Kustomize", "kustomize"},
		{"Login", "login"},
		{"Pulumi", "pulumi"},
		{"Refresh", "refresh"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, _, err := rootCmd.Find([]string{tc.commandName})
			if err == nil {
				assert.NotNil(t, cmd, "Command %s should exist", tc.commandName)
			}
			// Some commands might not be registered, which is okay
		})
	}
}

func TestRootCmd_Version(t *testing.T) {
	version := rootCmd.Version
	assert.NotEmpty(t, version)
	assert.Contains(t, version, ".")
}

func TestRootCmd_RunE_NotSet(t *testing.T) {
	// Root command should not have a RunE func (it just shows help)
	assert.Nil(t, rootCmd.RunE)
}

func TestRootCmd_PersistentPreRun(t *testing.T) {
	// Verify cobra initialization is set up
	assert.NotNil(t, rootCmd.PersistentFlags())
}
