package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddonsCmd_Structure(t *testing.T) {
	assert.NotNil(t, addonsCmd)
	assert.Equal(t, "addons", addonsCmd.Use)
	assert.NotEmpty(t, addonsCmd.Short)
	assert.NotEmpty(t, addonsCmd.Long)
}

func TestAddonsCmd_HasSubcommands(t *testing.T) {
	assert.True(t, addonsCmd.HasAvailableSubCommands(), "addons command should have subcommands")

	commands := addonsCmd.Commands()
	assert.NotEmpty(t, commands, "addons command should have registered subcommands")

	// Verify expected subcommands exist
	commandNames := make(map[string]bool)
	for _, cmd := range commands {
		commandNames[cmd.Name()] = true
	}

	expectedCommands := []string{"bootstrap", "list", "sync", "status", "template"}
	for _, cmdName := range expectedCommands {
		assert.True(t, commandNames[cmdName], "Expected subcommand '%s' should exist", cmdName)
	}
}

func TestAddonsBootstrapCmd_Structure(t *testing.T) {
	assert.NotNil(t, addonsBootstrapCmd)
	assert.Equal(t, "bootstrap", addonsBootstrapCmd.Use)
	assert.NotEmpty(t, addonsBootstrapCmd.Short)
	assert.NotEmpty(t, addonsBootstrapCmd.Long)
	assert.NotEmpty(t, addonsBootstrapCmd.Example)
	assert.NotNil(t, addonsBootstrapCmd.RunE)
}

func TestAddonsBootstrapCmd_Flags(t *testing.T) {
	repoFlag := addonsBootstrapCmd.Flags().Lookup("repo")
	assert.NotNil(t, repoFlag, "bootstrap should have --repo flag")

	branchFlag := addonsBootstrapCmd.Flags().Lookup("branch")
	assert.NotNil(t, branchFlag, "bootstrap should have --branch flag")
	assert.Equal(t, "main", branchFlag.DefValue, "branch default should be 'main'")

	pathFlag := addonsBootstrapCmd.Flags().Lookup("path")
	assert.NotNil(t, pathFlag, "bootstrap should have --path flag")
	assert.Equal(t, "addons/", pathFlag.DefValue, "path default should be 'addons/'")

	privateKeyFlag := addonsBootstrapCmd.Flags().Lookup("private-key")
	assert.NotNil(t, privateKeyFlag, "bootstrap should have --private-key flag")
}

func TestAddonsBootstrapCmd_RequiredFlags(t *testing.T) {
	// The repo flag is marked required via MarkFlagRequired
	// We can't easily test this without running the command, but we can verify the flag exists
	repoFlag := addonsBootstrapCmd.Flags().Lookup("repo")
	assert.NotNil(t, repoFlag, "repo flag should exist")
	assert.NotEmpty(t, repoFlag.Usage, "repo flag should have usage text")
}

func TestAddonsListCmd_Structure(t *testing.T) {
	assert.NotNil(t, addonsListCmd)
	assert.Equal(t, "list", addonsListCmd.Use)
	assert.NotEmpty(t, addonsListCmd.Short)
	assert.NotEmpty(t, addonsListCmd.Long)
	assert.NotEmpty(t, addonsListCmd.Example)
	assert.NotNil(t, addonsListCmd.RunE)
}

func TestAddonsListCmd_Examples(t *testing.T) {
	examples := addonsListCmd.Example
	assert.Contains(t, examples, "list")
	assert.Contains(t, examples, "stack")
}

func TestAddonsSyncCmd_Structure(t *testing.T) {
	assert.NotNil(t, addonsSyncCmd)
	assert.Equal(t, "sync", addonsSyncCmd.Use)
	assert.NotEmpty(t, addonsSyncCmd.Short)
	assert.NotEmpty(t, addonsSyncCmd.Long)
	assert.NotEmpty(t, addonsSyncCmd.Example)
	assert.NotNil(t, addonsSyncCmd.RunE)
}

func TestAddonsSyncCmd_Flags(t *testing.T) {
	appFlag := addonsSyncCmd.Flags().Lookup("app")
	assert.NotNil(t, appFlag, "sync should have --app flag")
}

func TestAddonsStatusCmd_Structure(t *testing.T) {
	assert.NotNil(t, addonsStatusCmd)
	assert.Equal(t, "status", addonsStatusCmd.Use)
	assert.NotEmpty(t, addonsStatusCmd.Short)
	assert.NotEmpty(t, addonsStatusCmd.Long)
	assert.NotEmpty(t, addonsStatusCmd.Example)
	assert.NotNil(t, addonsStatusCmd.RunE)
}

func TestAddonsStatusCmd_LongDescription(t *testing.T) {
	long := addonsStatusCmd.Long
	assert.Contains(t, long, "ArgoCD")
	assert.Contains(t, long, "status")
	assert.Contains(t, long, "Applications")
}

func TestAddonsTemplateCmd_Structure(t *testing.T) {
	assert.NotNil(t, addonsTemplateCmd)
	assert.Equal(t, "template", addonsTemplateCmd.Use)
	assert.NotEmpty(t, addonsTemplateCmd.Short)
	assert.NotEmpty(t, addonsTemplateCmd.Long)
	assert.NotEmpty(t, addonsTemplateCmd.Example)
	assert.NotNil(t, addonsTemplateCmd.RunE)
}

func TestAddonsTemplateCmd_Flags(t *testing.T) {
	outputFlag := addonsTemplateCmd.Flags().Lookup("output")
	assert.NotNil(t, outputFlag, "template should have --output flag")
	assert.Equal(t, "o", outputFlag.Shorthand, "output flag should have 'o' shorthand")
}

func TestAddonsBootstrapCmd_Examples(t *testing.T) {
	examples := addonsBootstrapCmd.Example
	assert.Contains(t, examples, "bootstrap")
	assert.Contains(t, examples, "--repo")
	assert.Contains(t, examples, "--branch")
	assert.Contains(t, examples, "--path")
	assert.Contains(t, examples, "--private-key")
}

func TestAddonsSyncCmd_Examples(t *testing.T) {
	examples := addonsSyncCmd.Example
	assert.Contains(t, examples, "sync")
	assert.Contains(t, examples, "--app")
}

func TestAddonsTemplateCmd_Examples(t *testing.T) {
	examples := addonsTemplateCmd.Example
	assert.Contains(t, examples, "template")
	assert.Contains(t, examples, "--output")
}

func TestAddonsCmd_LongDescription(t *testing.T) {
	long := addonsCmd.Long
	assert.Contains(t, long, "GitOps")
	assert.Contains(t, long, "Git repository")
	assert.Contains(t, long, "ArgoCD")
	assert.Contains(t, long, "manifests")
}

func TestAddonsBootstrapCmd_LongDescription(t *testing.T) {
	long := addonsBootstrapCmd.Long
	assert.Contains(t, long, "Bootstrap")
	assert.Contains(t, long, "ArgoCD")
	assert.Contains(t, long, "GitOps")
	assert.Contains(t, long, "Clone")
	assert.Contains(t, long, "repository")
}

func TestAddonsListCmd_LongDescription(t *testing.T) {
	long := addonsListCmd.Long
	assert.Contains(t, long, "List")
	assert.Contains(t, long, "addons")
	assert.Contains(t, long, "ArgoCD")
	assert.Contains(t, long, "kubectl")
}

func TestAddonsSyncCmd_LongDescription(t *testing.T) {
	long := addonsSyncCmd.Long
	assert.Contains(t, long, "sync")
	assert.Contains(t, long, "ArgoCD")
}

func TestAddonsTemplateCmd_LongDescription(t *testing.T) {
	long := addonsTemplateCmd.Long
	assert.Contains(t, long, "template")
	assert.Contains(t, long, "GitOps")
	assert.Contains(t, long, "repository")
	assert.Contains(t, long, "structure")
}

func TestPrintAddonTable(t *testing.T) {
	// Test that printAddonTable doesn't panic
	assert.NotPanics(t, func() {
		printAddonTable()
	})
}

func TestGenerateTemplateStructure(t *testing.T) {
	// Test with temporary directory
	tempDir := t.TempDir()

	err := generateTemplateStructure(tempDir)
	assert.NoError(t, err, "generateTemplateStructure should not error with valid temp dir")

	// Verify directory structure was created
	// This is a basic smoke test
}

func TestAddonsGlobalVariables(t *testing.T) {
	// Verify global variables are of correct type
	assert.IsType(t, "", gitopsRepo)
	assert.IsType(t, "", gitopsBranch)
	assert.IsType(t, "", gitopsPath)
	assert.IsType(t, "", gitopsPrivateKey)
	assert.IsType(t, "", addonNamespace)
	assert.IsType(t, "", addonValues)
}
