package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPulumiCmd_Structure(t *testing.T) {
	assert.NotNil(t, pulumiCmd)
	assert.Equal(t, "pulumi [command]", pulumiCmd.Use)
	assert.NotEmpty(t, pulumiCmd.Short)
	assert.NotEmpty(t, pulumiCmd.Long)
	assert.NotEmpty(t, pulumiCmd.Example)
}

func TestPulumiCmd_RunE(t *testing.T) {
	assert.NotNil(t, pulumiCmd.RunE, "pulumi command should have RunE function")
}

func TestPulumiCmd_HasSubcommands(t *testing.T) {
	assert.True(t, pulumiCmd.HasAvailableSubCommands(), "pulumi command should have subcommands")
	commands := pulumiCmd.Commands()
	assert.NotEmpty(t, commands, "pulumi command should have registered subcommands")
}

func TestStackCmd_Structure(t *testing.T) {
	assert.NotNil(t, stackCmd)
	assert.Equal(t, "stack", stackCmd.Use)
	assert.NotEmpty(t, stackCmd.Short)
	assert.NotEmpty(t, stackCmd.Long)
}

func TestPreviewPulumiCmd_Structure(t *testing.T) {
	assert.NotNil(t, previewPulumiCmd)
	assert.Equal(t, "preview", previewPulumiCmd.Use)
	assert.NotEmpty(t, previewPulumiCmd.Short)
	assert.NotEmpty(t, previewPulumiCmd.Long)
	assert.NotEmpty(t, previewPulumiCmd.Example)
	assert.NotNil(t, previewPulumiCmd.RunE)
}

func TestPulumiCmd_Examples(t *testing.T) {
	examples := pulumiCmd.Example
	assert.Contains(t, examples, "pulumi stack output")
	assert.Contains(t, examples, "pulumi stack list")
	assert.Contains(t, examples, "pulumi stack export")
	assert.Contains(t, examples, "pulumi stack import")
	assert.Contains(t, examples, "pulumi refresh")
}

func TestPulumiCmd_LongDescription(t *testing.T) {
	long := pulumiCmd.Long
	assert.Contains(t, long, "Automation API")
	assert.Contains(t, long, "Pulumi")
	assert.Contains(t, long, "no CLI required")
	assert.Contains(t, long, "embedded")
	assert.Contains(t, long, "sloth-kubernetes")
}

func TestPulumiCmd_AvailableOperations(t *testing.T) {
	long := pulumiCmd.Long

	operations := []string{
		"stack list",
		"stack output",
		"stack export",
		"stack import",
		"stack info",
		"stack delete",
		"stack select",
		"stack current",
		"stack rename",
		"stack cancel",
		"stack state",
		"preview",
		"refresh",
	}

	for _, op := range operations {
		assert.Contains(t, long, op, "Should list operation: %s", op)
	}
}

func TestPulumiCmd_NoCLIRequired(t *testing.T) {
	long := pulumiCmd.Long
	assert.Contains(t, long, "No Pulumi CLI", "Should emphasize no CLI requirement")
	assert.Contains(t, long, "🦥", "Should have sloth emoji")
}

func TestPulumiCmd_RegisteredWithRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "pulumi" {
			found = true
			break
		}
	}
	assert.True(t, found, "pulumi command should be registered with root")
}

func TestStackCmd_HasSubcommands(t *testing.T) {
	assert.True(t, stackCmd.HasAvailableSubCommands(), "stack command should have subcommands")
	commands := stackCmd.Commands()
	assert.NotEmpty(t, commands, "stack command should have registered subcommands")
}

func TestStackCmd_RegisteredWithPulumi(t *testing.T) {
	found := false
	for _, cmd := range pulumiCmd.Commands() {
		if cmd.Name() == "stack" {
			found = true
			break
		}
	}
	assert.True(t, found, "stack command should be registered with pulumi")
}

func TestPreviewPulumiCmd_Examples(t *testing.T) {
	examples := previewPulumiCmd.Example
	assert.Contains(t, examples, "preview")
	assert.Contains(t, examples, "--stack")
	assert.Contains(t, examples, "production")
}

func TestPreviewPulumiCmd_LongDescription(t *testing.T) {
	long := previewPulumiCmd.Long
	assert.Contains(t, long, "Preview")
	assert.Contains(t, long, "changes")
	assert.Contains(t, long, "infrastructure")
}

func TestPulumiCmd_GlobalVariables(t *testing.T) {
	assert.IsType(t, "", pulumiStackName)
}

func TestRunPulumiCommand_Exists(t *testing.T) {
	// Test that runPulumiCommand function exists
	assert.NotNil(t, runPulumiCommand, "runPulumiCommand function should be defined")
}

func TestRunPreview_Exists(t *testing.T) {
	// Test that runPreview function exists
	assert.NotNil(t, runPreview, "runPreview function should be defined")
}

func TestPulumiCmd_AutomationAPIEmphasis(t *testing.T) {
	short := pulumiCmd.Short
	long := pulumiCmd.Long

	assert.Contains(t, short, "Automation API")
	assert.Contains(t, long, "Automation API")
	assert.Contains(t, long, "embedded")
}

func TestPulumiCmd_StackOperationsComplete(t *testing.T) {
	// Verify that all expected stack operations are registered
	stackCommands := stackCmd.Commands()
	commandNames := make(map[string]bool)
	for _, cmd := range stackCommands {
		commandNames[cmd.Name()] = true
	}

	// Check for essential stack commands
	essentialCommands := []string{"list", "output", "info"}
	for _, cmdName := range essentialCommands {
		assert.True(t, commandNames[cmdName], "Stack should have '%s' subcommand", cmdName)
	}
}

func TestPreviewPulumiCmd_RequiresDeploymentContext(t *testing.T) {
	long := previewPulumiCmd.Long
	assert.Contains(t, long, "deployment context", "Should mention deployment context requirement")
}

func TestPulumiCmd_ShortDescription(t *testing.T) {
	short := pulumiCmd.Short
	assert.Contains(t, short, "Pulumi")
	assert.Contains(t, short, "Automation API")
}

func TestStackCmd_ShortDescription(t *testing.T) {
	short := stackCmd.Short
	assert.Contains(t, short, "Stack")
	assert.Contains(t, short, "operations")
}

func TestStackCmd_LongDescription(t *testing.T) {
	long := stackCmd.Long
	assert.Contains(t, long, "Pulumi")
	assert.Contains(t, long, "stacks")
}
