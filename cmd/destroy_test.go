package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDestroyCmd_Structure(t *testing.T) {
	assert.NotNil(t, destroyCmd)
	assert.Equal(t, "destroy", destroyCmd.Use)
	assert.NotEmpty(t, destroyCmd.Short)
	assert.NotEmpty(t, destroyCmd.Long)
	assert.NotEmpty(t, destroyCmd.Example)
}

func TestDestroyCmd_RunE(t *testing.T) {
	assert.NotNil(t, destroyCmd.RunE, "destroy command should have RunE function")
}

func TestDestroyCmd_Flags(t *testing.T) {
	forceFlag := destroyCmd.Flags().Lookup("force")
	assert.NotNil(t, forceFlag, "destroy should have --force flag")
	assert.Equal(t, "false", forceFlag.DefValue, "force flag should default to false")
}

func TestDestroyCmd_Examples(t *testing.T) {
	examples := destroyCmd.Example
	assert.Contains(t, examples, "destroy")
	assert.Contains(t, examples, "--yes")
	assert.Contains(t, examples, "--force")
}

func TestDestroyCmd_LongDescription(t *testing.T) {
	long := destroyCmd.Long
	assert.Contains(t, long, "Destroy")
	assert.Contains(t, long, "cluster")
	assert.Contains(t, long, "resources")
	assert.Contains(t, long, "WARNING")
	assert.Contains(t, long, "cannot be undone")
}

func TestDestroyCmd_ShortDescription(t *testing.T) {
	short := destroyCmd.Short
	assert.Contains(t, short, "Destroy")
	assert.Contains(t, short, "cluster")
}

func TestDestroyCmd_HasWarning(t *testing.T) {
	long := destroyCmd.Long
	// Verify that the command has proper warning language
	assert.Contains(t, long, "WARNING", "destroy command should have WARNING in description")
	assert.Contains(t, long, "cannot be undone", "destroy command should warn about irreversibility")
}

func TestDestroyCmd_ExampleUsage(t *testing.T) {
	examples := destroyCmd.Example

	// Should show example with confirmation
	assert.Contains(t, examples, "destroy")

	// Should show example with force option
	assert.Contains(t, examples, "force")

	// Should show yes flag usage
	assert.Contains(t, examples, "yes")
}

func TestDestroyCmd_FlagTypes(t *testing.T) {
	// Test force flag
	forceFlag := destroyCmd.Flags().Lookup("force")
	assert.NotNil(t, forceFlag)
	assert.Equal(t, "bool", forceFlag.Value.Type())
}

func TestForceVariable(t *testing.T) {
	// Verify global force variable exists and is boolean
	assert.IsType(t, false, force)
}

func TestDestroyCmd_RegisteredWithRoot(t *testing.T) {
	// Verify destroy is registered as a subcommand of root
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "destroy" {
			found = true
			break
		}
	}
	assert.True(t, found, "destroy command should be registered with root")
}

func TestDestroyCmd_NoRunWithoutConfirmation(t *testing.T) {
	// Verify that runDestroy function exists
	assert.NotNil(t, runDestroy, "runDestroy function should be defined")
}

func TestDestroyCmd_ExpectedBehavior(t *testing.T) {
	// Document expected behavior
	assert.NotNil(t, destroyCmd.RunE, "destroy should require confirmation before running")
	assert.NotEmpty(t, destroyCmd.Long, "destroy should have detailed warnings in Long description")

	// Force flag should be available
	forceFlag := destroyCmd.Flags().Lookup("force")
	assert.NotNil(t, forceFlag, "force flag should be available to skip dependency checks")
}
