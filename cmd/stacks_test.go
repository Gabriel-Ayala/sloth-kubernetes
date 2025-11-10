package cmd

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestStacksCmd_Structure(t *testing.T) {
	assert.NotNil(t, stacksCmd)
	assert.Equal(t, "stacks", stacksCmd.Use)
	assert.NotEmpty(t, stacksCmd.Short)
	assert.NotEmpty(t, stacksCmd.Long)
}

func TestStacksCmd_HasSubcommands(t *testing.T) {
	assert.True(t, stacksCmd.HasAvailableSubCommands(), "stacks command should have subcommands")

	commands := stacksCmd.Commands()
	assert.NotEmpty(t, commands, "stacks command should have registered subcommands")
}

func TestListStacksCmd_Structure(t *testing.T) {
	assert.NotNil(t, listStacksCmd)
	assert.Equal(t, "list", listStacksCmd.Use)
	assert.NotEmpty(t, listStacksCmd.Short)
	assert.NotEmpty(t, listStacksCmd.Long)
	assert.NotEmpty(t, listStacksCmd.Example)
	assert.NotNil(t, listStacksCmd.RunE)
}

func TestStackInfoCmd_Structure(t *testing.T) {
	assert.NotNil(t, stackInfoCmd)
	assert.Equal(t, "info [stack-name]", stackInfoCmd.Use)
	assert.NotEmpty(t, stackInfoCmd.Short)
	assert.NotEmpty(t, stackInfoCmd.Long)
	assert.NotEmpty(t, stackInfoCmd.Example)
	assert.NotNil(t, stackInfoCmd.RunE)
}

func TestDeleteStackCmd_Structure(t *testing.T) {
	assert.NotNil(t, deleteStackCmd)
	assert.Equal(t, "delete [stack-name]", deleteStackCmd.Use)
	assert.NotEmpty(t, deleteStackCmd.Short)
	assert.NotEmpty(t, deleteStackCmd.Long)
	assert.NotEmpty(t, deleteStackCmd.Example)
	assert.NotNil(t, deleteStackCmd.RunE)
}

func TestDeleteStackCmd_Flags(t *testing.T) {
	destroyFlag := deleteStackCmd.Flags().Lookup("destroy")
	assert.NotNil(t, destroyFlag, "delete should have --destroy flag")
	assert.Equal(t, "false", destroyFlag.DefValue, "destroy flag should default to false")
}

func TestOutputCmd_Structure(t *testing.T) {
	assert.NotNil(t, outputCmd)
	assert.Equal(t, "output [stack-name]", outputCmd.Use)
	assert.NotEmpty(t, outputCmd.Short)
	assert.NotEmpty(t, outputCmd.Long)
	assert.NotEmpty(t, outputCmd.Example)
	assert.NotNil(t, outputCmd.RunE)
}

func TestOutputCmd_Flags(t *testing.T) {
	keyFlag := outputCmd.Flags().Lookup("key")
	assert.NotNil(t, keyFlag, "output should have --key flag")

	jsonFlag := outputCmd.Flags().Lookup("json")
	assert.NotNil(t, jsonFlag, "output should have --json flag")
	assert.Equal(t, "false", jsonFlag.DefValue, "json flag should default to false")
}

func TestSelectStackCmd_Structure(t *testing.T) {
	assert.NotNil(t, selectStackCmd)
	assert.Equal(t, "select [stack-name]", selectStackCmd.Use)
	assert.NotEmpty(t, selectStackCmd.Short)
	assert.NotEmpty(t, selectStackCmd.Long)
	assert.NotEmpty(t, selectStackCmd.Example)
	assert.NotNil(t, selectStackCmd.RunE)
}

func TestExportStackCmd_Structure(t *testing.T) {
	assert.NotNil(t, exportStackCmd)
	assert.Equal(t, "export [stack-name]", exportStackCmd.Use)
	assert.NotEmpty(t, exportStackCmd.Short)
	assert.NotEmpty(t, exportStackCmd.Long)
	assert.NotEmpty(t, exportStackCmd.Example)
	assert.NotNil(t, exportStackCmd.RunE)
}

func TestExportStackCmd_Flags(t *testing.T) {
	outputFlag := exportStackCmd.Flags().Lookup("output")
	assert.NotNil(t, outputFlag, "export should have --output flag")
	assert.Equal(t, "o", outputFlag.Shorthand, "output flag should have 'o' shorthand")
}

func TestImportStackCmd_Structure(t *testing.T) {
	assert.NotNil(t, importStackCmd)
	assert.Equal(t, "import [stack-name] [file]", importStackCmd.Use)
	assert.NotEmpty(t, importStackCmd.Short)
	assert.NotEmpty(t, importStackCmd.Long)
	assert.NotEmpty(t, importStackCmd.Example)
	assert.NotNil(t, importStackCmd.RunE)
}

func TestCurrentStackCmd_Structure(t *testing.T) {
	assert.NotNil(t, currentStackCmd)
	assert.Equal(t, "current", currentStackCmd.Use)
	assert.NotEmpty(t, currentStackCmd.Short)
	assert.NotEmpty(t, currentStackCmd.Long)
	assert.NotEmpty(t, currentStackCmd.Example)
	assert.NotNil(t, currentStackCmd.RunE)
}

func TestRenameStackCmd_Structure(t *testing.T) {
	assert.NotNil(t, renameStackCmd)
	assert.Equal(t, "rename [old-name] [new-name]", renameStackCmd.Use)
	assert.NotEmpty(t, renameStackCmd.Short)
	assert.NotEmpty(t, renameStackCmd.Long)
	assert.NotEmpty(t, renameStackCmd.Example)
	assert.NotNil(t, renameStackCmd.RunE)
}

func TestCancelCmd_Structure(t *testing.T) {
	assert.NotNil(t, cancelCmd)
	assert.Equal(t, "cancel [stack-name]", cancelCmd.Use)
	assert.NotEmpty(t, cancelCmd.Short)
	assert.NotEmpty(t, cancelCmd.Long)
	assert.NotEmpty(t, cancelCmd.Example)
	assert.NotNil(t, cancelCmd.RunE)
}

func TestStateCmd_Structure(t *testing.T) {
	assert.NotNil(t, stateCmd)
	assert.Equal(t, "state", stateCmd.Use)
	assert.NotEmpty(t, stateCmd.Short)
	assert.NotEmpty(t, stateCmd.Long)
}

func TestStateDeleteCmd_Structure(t *testing.T) {
	assert.NotNil(t, stateDeleteCmd)
	assert.Equal(t, "delete [stack-name] [urn]", stateDeleteCmd.Use)
	assert.NotEmpty(t, stateDeleteCmd.Short)
	assert.NotEmpty(t, stateDeleteCmd.Long)
	assert.NotEmpty(t, stateDeleteCmd.Example)
	assert.NotNil(t, stateDeleteCmd.RunE)
}

func TestStateDeleteCmd_FlagsDetail(t *testing.T) {
	forceFlag := stateDeleteCmd.Flags().Lookup("force")
	assert.NotNil(t, forceFlag, "state delete should have --force flag")
	assert.Equal(t, "f", forceFlag.Shorthand, "force flag should have 'f' shorthand")
	assert.Equal(t, "false", forceFlag.DefValue, "force flag should default to false")
}

func TestStateListCmd_StructureDetail(t *testing.T) {
	assert.NotNil(t, stateListCmd)
	assert.Equal(t, "list [stack-name]", stateListCmd.Use)
	assert.NotEmpty(t, stateListCmd.Short)
	assert.NotEmpty(t, stateListCmd.Long)
	assert.NotEmpty(t, stateListCmd.Example)
	assert.NotNil(t, stateListCmd.RunE)
}

func TestStateListCmd_FlagsDetail(t *testing.T) {
	typeFlag := stateListCmd.Flags().Lookup("type")
	assert.NotNil(t, typeFlag, "state list should have --type flag")
}

func TestCreateWorkspaceWithS3Support(t *testing.T) {
	// Test that the function exists and has correct signature
	assert.NotNil(t, createWorkspaceWithS3Support, "createWorkspaceWithS3Support should be defined")
}

func TestFormatTime(t *testing.T) {
	// Test formatTime function with various durations
	testCases := []struct {
		name     string
		input    time.Duration
		expected string
	}{
		{"JustNow", 30 * time.Second, "just now"},
		{"Minutes", 5 * time.Minute, "5m ago"},
		{"Hours", 3 * time.Hour, "3h ago"},
		{"Days", 2 * 24 * time.Hour, "2d ago"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pastTime := time.Now().Add(-tc.input)
			result := formatTime(pastTime)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestStacksGlobalVariables(t *testing.T) {
	// Verify global variables are of correct type
	assert.IsType(t, false, destroyStack)
	assert.IsType(t, "", outputKey)
	assert.IsType(t, false, outputJSON)
	assert.IsType(t, "", exportOutput)
	assert.IsType(t, false, forceDelete)
	assert.IsType(t, "", resourceType)
}

func TestStacksCmd_SubcommandsList(t *testing.T) {
	commands := stacksCmd.Commands()
	commandNames := make(map[string]bool)
	for _, cmd := range commands {
		commandNames[cmd.Name()] = true
	}

	expectedCommands := []string{
		"list", "info", "delete", "output", "select",
		"current", "export", "import", "rename", "cancel", "state",
	}

	for _, cmdName := range expectedCommands {
		assert.True(t, commandNames[cmdName], "Expected subcommand '%s' should exist", cmdName)
	}
}

func TestStateCmd_SubcommandsList(t *testing.T) {
	commands := stateCmd.Commands()
	commandNames := make(map[string]bool)
	for _, cmd := range commands {
		commandNames[cmd.Name()] = true
	}

	expectedCommands := []string{"delete", "list"}
	for _, cmdName := range expectedCommands {
		assert.True(t, commandNames[cmdName], "Expected state subcommand '%s' should exist", cmdName)
	}
}

func TestStackCommands_Examples(t *testing.T) {
	testCases := []struct {
		name    string
		cmd     *cobra.Command
		keyword string
	}{
		{"list", listStacksCmd, "list"},
		{"info", stackInfoCmd, "info"},
		{"delete", deleteStackCmd, "delete"},
		{"output", outputCmd, "output"},
		{"select", selectStackCmd, "select"},
		{"export", exportStackCmd, "export"},
		{"import", importStackCmd, "import"},
		{"current", currentStackCmd, "current"},
		{"rename", renameStackCmd, "rename"},
		{"cancel", cancelCmd, "cancel"},
		{"state-list", stateListCmd, "list"},
		{"state-delete", stateDeleteCmd, "delete"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotEmpty(t, tc.cmd.Example, "%s command should have examples", tc.name)
			assert.Contains(t, tc.cmd.Example, tc.keyword, "%s example should contain '%s'", tc.name, tc.keyword)
		})
	}
}

func TestStackCommands_LongDescriptions(t *testing.T) {
	testCases := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"list", listStacksCmd},
		{"info", stackInfoCmd},
		{"delete", deleteStackCmd},
		{"output", outputCmd},
		{"select", selectStackCmd},
		{"export", exportStackCmd},
		{"import", importStackCmd},
		{"current", currentStackCmd},
		{"rename", renameStackCmd},
		{"cancel", cancelCmd},
		{"state", stateCmd},
		{"state-delete", stateDeleteCmd},
		{"state-list", stateListCmd},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotEmpty(t, tc.cmd.Long, "%s command should have Long description", tc.name)
		})
	}
}

func TestCancelCmd_Examples(t *testing.T) {
	examples := cancelCmd.Example
	assert.Contains(t, examples, "cancel")
	assert.Contains(t, examples, "home-cluster")
}

func TestStateDeleteCmd_WarningInDescription(t *testing.T) {
	long := stateDeleteCmd.Long
	assert.Contains(t, long, "WARNING", "state delete should have WARNING in description")
	assert.Contains(t, long, "dangerous", "state delete should warn about danger")
}

func TestStateDeleteCmd_Examples(t *testing.T) {
	examples := stateDeleteCmd.Example
	assert.Contains(t, examples, "delete")
	assert.Contains(t, examples, "urn")
	assert.Contains(t, examples, "--force")
}

func TestOutputCmd_Examples(t *testing.T) {
	examples := outputCmd.Example
	assert.Contains(t, examples, "output")
	assert.Contains(t, examples, "--key")
	assert.Contains(t, examples, "--json")
}
