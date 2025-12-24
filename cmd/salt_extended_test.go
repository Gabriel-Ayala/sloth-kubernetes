package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Package Commands Tests

func TestPkgCmd_Structure(t *testing.T) {
	assert.NotNil(t, pkgCmd)
	assert.Equal(t, "pkg", pkgCmd.Use)
	assert.NotEmpty(t, pkgCmd.Short)
	assert.NotEmpty(t, pkgCmd.Long)
}

func TestPkgCmd_HasSubcommands(t *testing.T) {
	assert.True(t, pkgCmd.HasAvailableSubCommands())
	commands := pkgCmd.Commands()
	assert.NotEmpty(t, commands)
}

func TestPkgInstallCmd_Structure(t *testing.T) {
	assert.NotNil(t, pkgInstallCmd)
	assert.Equal(t, "install <package...>", pkgInstallCmd.Use)
	assert.NotEmpty(t, pkgInstallCmd.Short)
	assert.NotEmpty(t, pkgInstallCmd.Example)
	assert.NotNil(t, pkgInstallCmd.RunE)
}

func TestPkgRemoveCmd_Structure(t *testing.T) {
	assert.NotNil(t, pkgRemoveCmd)
	assert.Equal(t, "remove <package...>", pkgRemoveCmd.Use)
	assert.NotEmpty(t, pkgRemoveCmd.Short)
	assert.NotNil(t, pkgRemoveCmd.RunE)
}

func TestPkgUpgradeCmd_Structure(t *testing.T) {
	assert.NotNil(t, pkgUpgradeCmd)
	assert.Contains(t, pkgUpgradeCmd.Use, "upgrade")
	assert.NotEmpty(t, pkgUpgradeCmd.Short)
	assert.NotNil(t, pkgUpgradeCmd.RunE)
}

func TestPkgListCmd_Structure(t *testing.T) {
	assert.NotNil(t, pkgListCmd)
	assert.Equal(t, "list", pkgListCmd.Use)
	assert.NotEmpty(t, pkgListCmd.Short)
	assert.NotNil(t, pkgListCmd.RunE)
}

// Service Commands Tests

func TestServiceCmd_Structure(t *testing.T) {
	assert.NotNil(t, serviceCmd)
	assert.Equal(t, "service", serviceCmd.Use)
	assert.NotEmpty(t, serviceCmd.Short)
	assert.NotEmpty(t, serviceCmd.Long)
}

func TestServiceCmd_HasSubcommands(t *testing.T) {
	assert.True(t, serviceCmd.HasAvailableSubCommands())
	commands := serviceCmd.Commands()
	assert.NotEmpty(t, commands)
}

func TestServiceStartCmd_Structure(t *testing.T) {
	assert.NotNil(t, serviceStartCmd)
	assert.Equal(t, "start <service>", serviceStartCmd.Use)
	assert.NotEmpty(t, serviceStartCmd.Short)
	assert.NotEmpty(t, serviceStartCmd.Example)
	assert.NotNil(t, serviceStartCmd.RunE)
}

func TestServiceStopCmd_Structure(t *testing.T) {
	assert.NotNil(t, serviceStopCmd)
	assert.Equal(t, "stop <service>", serviceStopCmd.Use)
	assert.NotEmpty(t, serviceStopCmd.Short)
	assert.NotNil(t, serviceStopCmd.RunE)
}

func TestServiceRestartCmd_Structure(t *testing.T) {
	assert.NotNil(t, serviceRestartCmd)
	assert.Equal(t, "restart <service>", serviceRestartCmd.Use)
	assert.NotEmpty(t, serviceRestartCmd.Short)
	assert.NotNil(t, serviceRestartCmd.RunE)
}

func TestServiceStatusCmd_Structure(t *testing.T) {
	assert.NotNil(t, serviceStatusCmd)
	assert.Equal(t, "status <service>", serviceStatusCmd.Use)
	assert.NotEmpty(t, serviceStatusCmd.Short)
	assert.NotNil(t, serviceStatusCmd.RunE)
}

func TestServiceEnableCmd_Structure(t *testing.T) {
	assert.NotNil(t, serviceEnableCmd)
	assert.Equal(t, "enable <service>", serviceEnableCmd.Use)
	assert.NotEmpty(t, serviceEnableCmd.Short)
	assert.NotNil(t, serviceEnableCmd.RunE)
}

func TestServiceDisableCmd_Structure(t *testing.T) {
	assert.NotNil(t, serviceDisableCmd)
	assert.Equal(t, "disable <service>", serviceDisableCmd.Use)
	assert.NotEmpty(t, serviceDisableCmd.Short)
	assert.NotNil(t, serviceDisableCmd.RunE)
}

func TestServiceListCmd_Structure(t *testing.T) {
	assert.NotNil(t, serviceListCmd)
	assert.Equal(t, "list", serviceListCmd.Use)
	assert.NotEmpty(t, serviceListCmd.Short)
	assert.NotNil(t, serviceListCmd.RunE)
}

// File Commands Tests

func TestFileCmd_Structure(t *testing.T) {
	assert.NotNil(t, fileCmd)
	assert.Equal(t, "file", fileCmd.Use)
	assert.NotEmpty(t, fileCmd.Short)
	assert.NotEmpty(t, fileCmd.Long)
}

func TestFileCmd_HasSubcommands(t *testing.T) {
	assert.True(t, fileCmd.HasAvailableSubCommands())
	commands := fileCmd.Commands()
	assert.NotEmpty(t, commands)
}

func TestFileReadCmd_Structure(t *testing.T) {
	assert.NotNil(t, fileReadCmd)
	assert.Equal(t, "read <path>", fileReadCmd.Use)
	assert.NotEmpty(t, fileReadCmd.Short)
	assert.NotNil(t, fileReadCmd.RunE)
}

func TestFileWriteCmd_Structure(t *testing.T) {
	assert.NotNil(t, fileWriteCmd)
	assert.Equal(t, "write <path> <content>", fileWriteCmd.Use)
	assert.NotEmpty(t, fileWriteCmd.Short)
	assert.NotNil(t, fileWriteCmd.RunE)
}

func TestFileRemoveCmd_Structure(t *testing.T) {
	assert.NotNil(t, fileRemoveCmd)
	assert.Equal(t, "remove <path>", fileRemoveCmd.Use)
	assert.NotEmpty(t, fileRemoveCmd.Short)
	assert.NotNil(t, fileRemoveCmd.RunE)
}

func TestFileExistsCmd_Structure(t *testing.T) {
	assert.NotNil(t, fileExistsCmd)
	assert.Equal(t, "exists <path>", fileExistsCmd.Use)
	assert.NotEmpty(t, fileExistsCmd.Short)
	assert.NotNil(t, fileExistsCmd.RunE)
}

func TestFileChmodCmd_Structure(t *testing.T) {
	assert.NotNil(t, fileChmodCmd)
	assert.Equal(t, "chmod <path> <mode>", fileChmodCmd.Use)
	assert.NotEmpty(t, fileChmodCmd.Short)
	assert.NotEmpty(t, fileChmodCmd.Example)
	assert.NotNil(t, fileChmodCmd.RunE)
}

// System Commands Tests

func TestSystemCmd_Structure(t *testing.T) {
	assert.NotNil(t, systemCmd)
	assert.Equal(t, "system", systemCmd.Use)
	assert.NotEmpty(t, systemCmd.Short)
	assert.NotEmpty(t, systemCmd.Long)
}

func TestSystemCmd_HasSubcommands(t *testing.T) {
	assert.True(t, systemCmd.HasAvailableSubCommands())
	commands := systemCmd.Commands()
	assert.NotEmpty(t, commands)
}

func TestSystemRebootCmd_Structure(t *testing.T) {
	assert.NotNil(t, systemRebootCmd)
	assert.Equal(t, "reboot", systemRebootCmd.Use)
	assert.NotEmpty(t, systemRebootCmd.Short)
	assert.Contains(t, systemRebootCmd.Long, "WARNING")
	assert.NotNil(t, systemRebootCmd.RunE)
}

func TestSystemUptimeCmd_Structure(t *testing.T) {
	assert.NotNil(t, systemUptimeCmd)
	assert.Equal(t, "uptime", systemUptimeCmd.Use)
	assert.NotEmpty(t, systemUptimeCmd.Short)
	assert.NotNil(t, systemUptimeCmd.RunE)
}

func TestSystemDiskCmd_Structure(t *testing.T) {
	assert.NotNil(t, systemDiskCmd)
	assert.Equal(t, "disk", systemDiskCmd.Use)
	assert.NotEmpty(t, systemDiskCmd.Short)
	assert.NotNil(t, systemDiskCmd.RunE)
}

func TestSystemMemoryCmd_Structure(t *testing.T) {
	assert.NotNil(t, systemMemoryCmd)
	assert.Equal(t, "memory", systemMemoryCmd.Use)
	assert.NotEmpty(t, systemMemoryCmd.Short)
	assert.NotNil(t, systemMemoryCmd.RunE)
}

func TestSystemCPUCmd_Structure(t *testing.T) {
	assert.NotNil(t, systemCPUCmd)
	assert.Equal(t, "cpu", systemCPUCmd.Use)
	assert.NotEmpty(t, systemCPUCmd.Short)
	assert.NotNil(t, systemCPUCmd.RunE)
}

func TestSystemNetworkCmd_Structure(t *testing.T) {
	assert.NotNil(t, systemNetworkCmd)
	assert.Equal(t, "network", systemNetworkCmd.Use)
	assert.NotEmpty(t, systemNetworkCmd.Short)
	assert.NotNil(t, systemNetworkCmd.RunE)
}

// User Commands Tests

func TestUserCmd_Structure(t *testing.T) {
	assert.NotNil(t, userCmd)
	assert.Equal(t, "user", userCmd.Use)
	assert.NotEmpty(t, userCmd.Short)
	assert.NotEmpty(t, userCmd.Long)
}

func TestUserCmd_HasSubcommands(t *testing.T) {
	assert.True(t, userCmd.HasAvailableSubCommands())
	commands := userCmd.Commands()
	assert.NotEmpty(t, commands)
}

func TestUserAddCmd_Structure(t *testing.T) {
	assert.NotNil(t, userAddCmd)
	assert.Equal(t, "add <username>", userAddCmd.Use)
	assert.NotEmpty(t, userAddCmd.Short)
	assert.NotNil(t, userAddCmd.RunE)
}

func TestUserDeleteCmd_Structure(t *testing.T) {
	assert.NotNil(t, userDeleteCmd)
	assert.Equal(t, "delete <username>", userDeleteCmd.Use)
	assert.NotEmpty(t, userDeleteCmd.Short)
	assert.NotNil(t, userDeleteCmd.RunE)
}

func TestUserListCmd_Structure(t *testing.T) {
	assert.NotNil(t, userListCmd)
	assert.Equal(t, "list", userListCmd.Use)
	assert.NotEmpty(t, userListCmd.Short)
	assert.NotNil(t, userListCmd.RunE)
}

func TestUserInfoCmd_Structure(t *testing.T) {
	assert.NotNil(t, userInfoCmd)
	assert.Equal(t, "info <username>", userInfoCmd.Use)
	assert.NotEmpty(t, userInfoCmd.Short)
	assert.NotNil(t, userInfoCmd.RunE)
}

// Docker Commands Tests

func TestDockerCmd_Structure(t *testing.T) {
	assert.NotNil(t, dockerCmd)
	assert.Equal(t, "docker", dockerCmd.Use)
	assert.NotEmpty(t, dockerCmd.Short)
	assert.NotEmpty(t, dockerCmd.Long)
}

func TestDockerCmd_HasSubcommands(t *testing.T) {
	assert.True(t, dockerCmd.HasAvailableSubCommands())
	commands := dockerCmd.Commands()
	assert.NotEmpty(t, commands)
}

func TestDockerPSCmd_Structure(t *testing.T) {
	assert.NotNil(t, dockerPSCmd)
	assert.Equal(t, "ps", dockerPSCmd.Use)
	assert.NotEmpty(t, dockerPSCmd.Short)
	assert.NotNil(t, dockerPSCmd.RunE)
}

func TestDockerStartCmd_Structure(t *testing.T) {
	assert.NotNil(t, dockerStartCmd)
	assert.Equal(t, "start <container>", dockerStartCmd.Use)
	assert.NotEmpty(t, dockerStartCmd.Short)
	assert.NotNil(t, dockerStartCmd.RunE)
}

func TestDockerStopCmd_Structure(t *testing.T) {
	assert.NotNil(t, dockerStopCmd)
	assert.Equal(t, "stop <container>", dockerStopCmd.Use)
	assert.NotEmpty(t, dockerStopCmd.Short)
	assert.NotNil(t, dockerStopCmd.RunE)
}

func TestDockerRestartCmd_Structure(t *testing.T) {
	assert.NotNil(t, dockerRestartCmd)
	assert.Equal(t, "restart <container>", dockerRestartCmd.Use)
	assert.NotEmpty(t, dockerRestartCmd.Short)
	assert.NotNil(t, dockerRestartCmd.RunE)
}

// Job Commands Tests

func TestJobCmd_Structure(t *testing.T) {
	assert.NotNil(t, jobCmd)
	assert.Equal(t, "job", jobCmd.Use)
	assert.NotEmpty(t, jobCmd.Short)
	assert.NotEmpty(t, jobCmd.Long)
}

func TestJobCmd_HasSubcommands(t *testing.T) {
	assert.True(t, jobCmd.HasAvailableSubCommands())
	commands := jobCmd.Commands()
	assert.NotEmpty(t, commands)
}

func TestJobListCmd_Structure(t *testing.T) {
	assert.NotNil(t, jobListCmd)
	assert.Equal(t, "list", jobListCmd.Use)
	assert.NotEmpty(t, jobListCmd.Short)
	assert.NotNil(t, jobListCmd.RunE)
}

func TestJobKillCmd_Structure(t *testing.T) {
	assert.NotNil(t, jobKillCmd)
	assert.Equal(t, "kill <jid>", jobKillCmd.Use)
	assert.NotEmpty(t, jobKillCmd.Short)
	assert.NotNil(t, jobKillCmd.RunE)
}

func TestJobSyncCmd_Structure(t *testing.T) {
	assert.NotNil(t, jobSyncCmd)
	assert.Equal(t, "sync", jobSyncCmd.Use)
	assert.NotEmpty(t, jobSyncCmd.Short)
	assert.NotNil(t, jobSyncCmd.RunE)
}

// Test all salt extended subcommands are registered
func TestSaltExtendedSubcommands_RegisteredWithSalt(t *testing.T) {
	saltSubcommands := saltCmd.Commands()
	subcommandNames := make(map[string]bool)
	for _, cmd := range saltSubcommands {
		subcommandNames[cmd.Name()] = true
	}

	expectedSubcommands := []string{
		"pkg",
		"service",
		"file",
		"system",
		"user",
		"docker",
		"job",
	}

	for _, name := range expectedSubcommands {
		assert.True(t, subcommandNames[name], "Expected salt subcommand '%s' should exist", name)
	}
}

// Test command descriptions contain expected keywords
func TestSaltExtendedCmd_Descriptions(t *testing.T) {
	tests := []struct {
		name     string
		short    string
		keywords []string
	}{
		{"pkg", pkgCmd.Short, []string{"Package"}},
		{"service", serviceCmd.Short, []string{"Service"}},
		{"file", fileCmd.Short, []string{"File"}},
		{"system", systemCmd.Short, []string{"System"}},
		{"user", userCmd.Short, []string{"User"}},
		{"docker", dockerCmd.Short, []string{"Docker"}},
		{"job", jobCmd.Short, []string{"Job"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, keyword := range tt.keywords {
				assert.Contains(t, tt.short, keyword,
					"Short description of %s should contain '%s'", tt.name, keyword)
			}
		})
	}
}

// Test package install example
func TestPkgInstallCmd_Example(t *testing.T) {
	examples := pkgInstallCmd.Example
	assert.Contains(t, examples, "nginx")
	assert.Contains(t, examples, "--target")
}

// Test service start example
func TestServiceStartCmd_Example(t *testing.T) {
	examples := serviceStartCmd.Example
	assert.Contains(t, examples, "nginx")
	assert.Contains(t, examples, "k3s")
}

// Test file chmod example
func TestFileChmodCmd_Example(t *testing.T) {
	examples := fileChmodCmd.Example
	assert.Contains(t, examples, "chmod")
	assert.Contains(t, examples, "755")
}

// Test system reboot warning
func TestSystemRebootCmd_Warning(t *testing.T) {
	longDesc := systemRebootCmd.Long
	assert.Contains(t, longDesc, "WARNING")
	assert.Contains(t, longDesc, "reboot")
}

// Test service lifecycle commands exist
func TestServiceCmd_LifecycleOperations(t *testing.T) {
	serviceSubcommands := serviceCmd.Commands()
	operations := make(map[string]bool)
	for _, cmd := range serviceSubcommands {
		operations[cmd.Name()] = true
	}

	lifecycleOps := []string{"start", "stop", "restart", "status", "enable", "disable"}
	for _, op := range lifecycleOps {
		assert.True(t, operations[op], "Service should have %s command", op)
	}
}

// Test file CRUD operations exist
func TestFileCmd_CRUDOperations(t *testing.T) {
	fileSubcommands := fileCmd.Commands()
	operations := make(map[string]bool)
	for _, cmd := range fileSubcommands {
		operations[cmd.Name()] = true
	}

	crudOps := []string{"read", "write", "remove", "exists", "chmod"}
	for _, op := range crudOps {
		assert.True(t, operations[op], "File should have %s command", op)
	}
}

// Test system monitoring commands exist
func TestSystemCmd_MonitoringOperations(t *testing.T) {
	systemSubcommands := systemCmd.Commands()
	operations := make(map[string]bool)
	for _, cmd := range systemSubcommands {
		operations[cmd.Name()] = true
	}

	monitoringOps := []string{"uptime", "disk", "memory", "cpu", "network"}
	for _, op := range monitoringOps {
		assert.True(t, operations[op], "System should have %s command", op)
	}
}

// Test docker container operations exist
func TestDockerCmd_ContainerOperations(t *testing.T) {
	dockerSubcommands := dockerCmd.Commands()
	operations := make(map[string]bool)
	for _, cmd := range dockerSubcommands {
		operations[cmd.Name()] = true
	}

	containerOps := []string{"ps", "start", "stop", "restart"}
	for _, op := range containerOps {
		assert.True(t, operations[op], "Docker should have %s command", op)
	}
}

// Test user management operations exist
func TestUserCmd_ManagementOperations(t *testing.T) {
	userSubcommands := userCmd.Commands()
	operations := make(map[string]bool)
	for _, cmd := range userSubcommands {
		operations[cmd.Name()] = true
	}

	userOps := []string{"add", "delete", "list", "info"}
	for _, op := range userOps {
		assert.True(t, operations[op], "User should have %s command", op)
	}
}

// Test job management operations exist
func TestJobCmd_ManagementOperations(t *testing.T) {
	jobSubcommands := jobCmd.Commands()
	operations := make(map[string]bool)
	for _, cmd := range jobSubcommands {
		operations[cmd.Name()] = true
	}

	jobOps := []string{"list", "kill", "sync"}
	for _, op := range jobOps {
		assert.True(t, operations[op], "Job should have %s command", op)
	}
}

// Test all commands have valid argument specifications
func TestSaltExtendedCmd_ArgumentSpecs(t *testing.T) {
	tests := []struct {
		name    string
		use     string
		hasArgs bool
	}{
		{"pkg install", pkgInstallCmd.Use, true},
		{"service start", serviceStartCmd.Use, true},
		{"file read", fileReadCmd.Use, true},
		{"system uptime", systemUptimeCmd.Use, false},
		{"user add", userAddCmd.Use, true},
		{"docker ps", dockerPSCmd.Use, false},
		{"job list", jobListCmd.Use, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.hasArgs {
				assert.Contains(t, tt.use, "<", "Command %s should have argument placeholders", tt.name)
			}
		})
	}
}
