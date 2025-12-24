package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Network Commands Tests

func TestNetworkCmd_Structure(t *testing.T) {
	assert.NotNil(t, networkCmd)
	assert.Equal(t, "network", networkCmd.Use)
	assert.NotEmpty(t, networkCmd.Short)
	assert.NotEmpty(t, networkCmd.Long)
}

func TestNetworkCmd_HasSubcommands(t *testing.T) {
	assert.True(t, networkCmd.HasAvailableSubCommands())
	commands := networkCmd.Commands()
	assert.NotEmpty(t, commands)
}

func TestNetworkPingCmd_Structure(t *testing.T) {
	assert.NotNil(t, networkPingCmd)
	assert.Equal(t, "ping <host> [count]", networkPingCmd.Use)
	assert.NotEmpty(t, networkPingCmd.Short)
	assert.NotEmpty(t, networkPingCmd.Example)
	assert.NotNil(t, networkPingCmd.RunE)
}

func TestNetworkTracerouteCmd_Structure(t *testing.T) {
	assert.NotNil(t, networkTracerouteCmd)
	assert.Equal(t, "traceroute <host>", networkTracerouteCmd.Use)
	assert.NotEmpty(t, networkTracerouteCmd.Short)
	assert.NotNil(t, networkTracerouteCmd.RunE)
}

func TestNetworkNetstatCmd_Structure(t *testing.T) {
	assert.NotNil(t, networkNetstatCmd)
	assert.Equal(t, "netstat", networkNetstatCmd.Use)
	assert.NotEmpty(t, networkNetstatCmd.Short)
	assert.NotNil(t, networkNetstatCmd.RunE)
}

func TestNetworkConnectionsCmd_Structure(t *testing.T) {
	assert.NotNil(t, networkConnectionsCmd)
	assert.Equal(t, "connections", networkConnectionsCmd.Use)
	assert.NotEmpty(t, networkConnectionsCmd.Short)
	assert.NotNil(t, networkConnectionsCmd.RunE)
}

func TestNetworkRoutesCmd_Structure(t *testing.T) {
	assert.NotNil(t, networkRoutesCmd)
	assert.Equal(t, "routes", networkRoutesCmd.Use)
	assert.NotEmpty(t, networkRoutesCmd.Short)
	assert.NotNil(t, networkRoutesCmd.RunE)
}

func TestNetworkARPCmd_Structure(t *testing.T) {
	assert.NotNil(t, networkARPCmd)
	assert.Equal(t, "arp", networkARPCmd.Use)
	assert.NotEmpty(t, networkARPCmd.Short)
	assert.NotNil(t, networkARPCmd.RunE)
}

// Process Commands Tests

func TestProcessCmd_Structure(t *testing.T) {
	assert.NotNil(t, processCmd)
	assert.Equal(t, "process", processCmd.Use)
	assert.NotEmpty(t, processCmd.Short)
	assert.NotEmpty(t, processCmd.Long)
}

func TestProcessCmd_HasSubcommands(t *testing.T) {
	assert.True(t, processCmd.HasAvailableSubCommands())
	commands := processCmd.Commands()
	assert.NotEmpty(t, commands)
}

func TestProcessListCmd_Structure(t *testing.T) {
	assert.NotNil(t, processListCmd)
	assert.Equal(t, "list", processListCmd.Use)
	assert.NotEmpty(t, processListCmd.Short)
	assert.NotNil(t, processListCmd.RunE)
}

func TestProcessTopCmd_Structure(t *testing.T) {
	assert.NotNil(t, processTopCmd)
	assert.Equal(t, "top", processTopCmd.Use)
	assert.NotEmpty(t, processTopCmd.Short)
	assert.NotNil(t, processTopCmd.RunE)
}

func TestProcessKillCmd_Structure(t *testing.T) {
	assert.NotNil(t, processKillCmd)
	assert.Equal(t, "kill <pid> [signal]", processKillCmd.Use)
	assert.NotEmpty(t, processKillCmd.Short)
	assert.NotEmpty(t, processKillCmd.Example)
	assert.NotNil(t, processKillCmd.RunE)
}

func TestProcessInfoCmd_Structure(t *testing.T) {
	assert.NotNil(t, processInfoCmd)
	assert.Equal(t, "info <pid>", processInfoCmd.Use)
	assert.NotEmpty(t, processInfoCmd.Short)
	assert.NotNil(t, processInfoCmd.RunE)
}

// Cron Commands Tests

func TestCronCmd_Structure(t *testing.T) {
	assert.NotNil(t, cronCmd)
	assert.Equal(t, "cron", cronCmd.Use)
	assert.NotEmpty(t, cronCmd.Short)
	assert.NotEmpty(t, cronCmd.Long)
}

func TestCronCmd_HasSubcommands(t *testing.T) {
	assert.True(t, cronCmd.HasAvailableSubCommands())
	commands := cronCmd.Commands()
	assert.NotEmpty(t, commands)
}

func TestCronListCmd_Structure(t *testing.T) {
	assert.NotNil(t, cronListCmd)
	assert.Equal(t, "list <user>", cronListCmd.Use)
	assert.NotEmpty(t, cronListCmd.Short)
	assert.NotNil(t, cronListCmd.RunE)
}

func TestCronAddCmd_Structure(t *testing.T) {
	assert.NotNil(t, cronAddCmd)
	assert.Contains(t, cronAddCmd.Use, "add")
	assert.NotEmpty(t, cronAddCmd.Short)
	assert.NotEmpty(t, cronAddCmd.Example)
	assert.NotNil(t, cronAddCmd.RunE)
}

func TestCronRemoveCmd_Structure(t *testing.T) {
	assert.NotNil(t, cronRemoveCmd)
	assert.Equal(t, "remove <user> <command>", cronRemoveCmd.Use)
	assert.NotEmpty(t, cronRemoveCmd.Short)
	assert.NotNil(t, cronRemoveCmd.RunE)
}

// Archive Commands Tests

func TestArchiveCmd_Structure(t *testing.T) {
	assert.NotNil(t, archiveCmd)
	assert.Equal(t, "archive", archiveCmd.Use)
	assert.NotEmpty(t, archiveCmd.Short)
	assert.NotEmpty(t, archiveCmd.Long)
}

func TestArchiveCmd_HasSubcommands(t *testing.T) {
	assert.True(t, archiveCmd.HasAvailableSubCommands())
	commands := archiveCmd.Commands()
	assert.NotEmpty(t, commands)
}

func TestArchiveTarCmd_Structure(t *testing.T) {
	assert.NotNil(t, archiveTarCmd)
	assert.Equal(t, "tar <source> <destination>", archiveTarCmd.Use)
	assert.NotEmpty(t, archiveTarCmd.Short)
	assert.NotEmpty(t, archiveTarCmd.Example)
	assert.NotNil(t, archiveTarCmd.RunE)
}

func TestArchiveUntarCmd_Structure(t *testing.T) {
	assert.NotNil(t, archiveUntarCmd)
	assert.Equal(t, "untar <source> <destination>", archiveUntarCmd.Use)
	assert.NotEmpty(t, archiveUntarCmd.Short)
	assert.NotNil(t, archiveUntarCmd.RunE)
}

func TestArchiveZipCmd_Structure(t *testing.T) {
	assert.NotNil(t, archiveZipCmd)
	assert.Equal(t, "zip <source> <destination>", archiveZipCmd.Use)
	assert.NotEmpty(t, archiveZipCmd.Short)
	assert.NotNil(t, archiveZipCmd.RunE)
}

func TestArchiveUnzipCmd_Structure(t *testing.T) {
	assert.NotNil(t, archiveUnzipCmd)
	assert.Equal(t, "unzip <source> <destination>", archiveUnzipCmd.Use)
	assert.NotEmpty(t, archiveUnzipCmd.Short)
	assert.NotNil(t, archiveUnzipCmd.RunE)
}

// Monitor Commands Tests

func TestMonitorCmd_Structure(t *testing.T) {
	assert.NotNil(t, monitorCmd)
	assert.Equal(t, "monitor", monitorCmd.Use)
	assert.NotEmpty(t, monitorCmd.Short)
	assert.NotEmpty(t, monitorCmd.Long)
}

func TestMonitorCmd_HasSubcommands(t *testing.T) {
	assert.True(t, monitorCmd.HasAvailableSubCommands())
	commands := monitorCmd.Commands()
	assert.NotEmpty(t, commands)
}

func TestMonitorLoadCmd_Structure(t *testing.T) {
	assert.NotNil(t, monitorLoadCmd)
	assert.Equal(t, "load", monitorLoadCmd.Use)
	assert.NotEmpty(t, monitorLoadCmd.Short)
	assert.NotNil(t, monitorLoadCmd.RunE)
}

func TestMonitorIOCmd_Structure(t *testing.T) {
	assert.NotNil(t, monitorIOCmd)
	assert.Equal(t, "iostat", monitorIOCmd.Use)
	assert.NotEmpty(t, monitorIOCmd.Short)
	assert.NotNil(t, monitorIOCmd.RunE)
}

func TestMonitorNetIOCmd_Structure(t *testing.T) {
	assert.NotNil(t, monitorNetIOCmd)
	assert.Equal(t, "netstats", monitorNetIOCmd.Use)
	assert.NotEmpty(t, monitorNetIOCmd.Short)
	assert.NotNil(t, monitorNetIOCmd.RunE)
}

func TestMonitorInfoCmd_Structure(t *testing.T) {
	assert.NotNil(t, monitorInfoCmd)
	assert.Equal(t, "info", monitorInfoCmd.Use)
	assert.NotEmpty(t, monitorInfoCmd.Short)
	assert.NotNil(t, monitorInfoCmd.RunE)
}

// SSH Commands Tests

func TestSSHCmd_Structure(t *testing.T) {
	assert.NotNil(t, sshCmd)
	assert.Equal(t, "ssh", sshCmd.Use)
	assert.NotEmpty(t, sshCmd.Short)
	assert.NotEmpty(t, sshCmd.Long)
}

func TestSSHCmd_HasSubcommands(t *testing.T) {
	assert.True(t, sshCmd.HasAvailableSubCommands())
	commands := sshCmd.Commands()
	assert.NotEmpty(t, commands)
}

func TestSSHKeyGenCmd_Structure(t *testing.T) {
	assert.NotNil(t, sshKeyGenCmd)
	assert.Equal(t, "keygen <user> <type>", sshKeyGenCmd.Use)
	assert.NotEmpty(t, sshKeyGenCmd.Short)
	assert.NotEmpty(t, sshKeyGenCmd.Example)
	assert.NotNil(t, sshKeyGenCmd.RunE)
}

func TestSSHAuthKeysCmd_Structure(t *testing.T) {
	assert.NotNil(t, sshAuthKeysCmd)
	assert.Equal(t, "authkeys <user>", sshAuthKeysCmd.Use)
	assert.NotEmpty(t, sshAuthKeysCmd.Short)
	assert.NotNil(t, sshAuthKeysCmd.RunE)
}

func TestSSHSetKeyCmd_Structure(t *testing.T) {
	assert.NotNil(t, sshSetKeyCmd)
	assert.Equal(t, "setkey <user> <key>", sshSetKeyCmd.Use)
	assert.NotEmpty(t, sshSetKeyCmd.Short)
	assert.NotNil(t, sshSetKeyCmd.RunE)
}

// Git Commands Tests

func TestGitCmd_Structure(t *testing.T) {
	assert.NotNil(t, gitCmd)
	assert.Equal(t, "git", gitCmd.Use)
	assert.NotEmpty(t, gitCmd.Short)
	assert.NotEmpty(t, gitCmd.Long)
}

func TestGitCmd_HasSubcommands(t *testing.T) {
	assert.True(t, gitCmd.HasAvailableSubCommands())
	commands := gitCmd.Commands()
	assert.NotEmpty(t, commands)
}

func TestGitCloneCmd_Structure(t *testing.T) {
	assert.NotNil(t, gitCloneCmd)
	assert.Equal(t, "clone <repo> <destination>", gitCloneCmd.Use)
	assert.NotEmpty(t, gitCloneCmd.Short)
	assert.NotEmpty(t, gitCloneCmd.Example)
	assert.NotNil(t, gitCloneCmd.RunE)
}

func TestGitPullCmd_Structure(t *testing.T) {
	assert.NotNil(t, gitPullCmd)
	assert.Equal(t, "pull <path>", gitPullCmd.Use)
	assert.NotEmpty(t, gitPullCmd.Short)
	assert.NotNil(t, gitPullCmd.RunE)
}

// Kubernetes Commands Tests

func TestK8sCmd_Structure(t *testing.T) {
	assert.NotNil(t, k8sCmd)
	assert.Equal(t, "k8s", k8sCmd.Use)
	assert.NotEmpty(t, k8sCmd.Short)
	assert.NotEmpty(t, k8sCmd.Long)
	assert.Contains(t, k8sCmd.Aliases, "kubectl")
}

func TestK8sCmd_HasSubcommands(t *testing.T) {
	assert.True(t, k8sCmd.HasAvailableSubCommands())
	commands := k8sCmd.Commands()
	assert.NotEmpty(t, commands)
}

func TestK8sGetCmd_Structure(t *testing.T) {
	assert.NotNil(t, k8sGetCmd)
	assert.Equal(t, "get <resource>", k8sGetCmd.Use)
	assert.NotEmpty(t, k8sGetCmd.Short)
	assert.NotEmpty(t, k8sGetCmd.Example)
	assert.NotNil(t, k8sGetCmd.RunE)
}

func TestK8sApplyCmd_Structure(t *testing.T) {
	assert.NotNil(t, k8sApplyCmd)
	assert.Equal(t, "apply <manifest>", k8sApplyCmd.Use)
	assert.NotEmpty(t, k8sApplyCmd.Short)
	assert.NotNil(t, k8sApplyCmd.RunE)
}

func TestK8sDeleteCmd_Structure(t *testing.T) {
	assert.NotNil(t, k8sDeleteCmd)
	assert.Equal(t, "delete <resource> <name>", k8sDeleteCmd.Use)
	assert.NotEmpty(t, k8sDeleteCmd.Short)
	assert.NotNil(t, k8sDeleteCmd.RunE)
}

// Pillar Commands Tests

func TestPillarCmd_Structure(t *testing.T) {
	assert.NotNil(t, pillarCmd)
	assert.Equal(t, "pillar", pillarCmd.Use)
	assert.NotEmpty(t, pillarCmd.Short)
	assert.NotEmpty(t, pillarCmd.Long)
}

func TestPillarCmd_HasSubcommands(t *testing.T) {
	assert.True(t, pillarCmd.HasAvailableSubCommands())
	commands := pillarCmd.Commands()
	assert.NotEmpty(t, commands)
}

func TestPillarGetCmd_Structure(t *testing.T) {
	assert.NotNil(t, pillarGetCmd)
	assert.Equal(t, "get <key>", pillarGetCmd.Use)
	assert.NotEmpty(t, pillarGetCmd.Short)
	assert.NotNil(t, pillarGetCmd.RunE)
}

func TestPillarListCmd_Structure(t *testing.T) {
	assert.NotNil(t, pillarListCmd)
	assert.Equal(t, "list", pillarListCmd.Use)
	assert.NotEmpty(t, pillarListCmd.Short)
	assert.NotNil(t, pillarListCmd.RunE)
}

// Mount Commands Tests

func TestMountCmd_Structure(t *testing.T) {
	assert.NotNil(t, mountCmd)
	assert.Equal(t, "mount", mountCmd.Use)
	assert.NotEmpty(t, mountCmd.Short)
	assert.NotEmpty(t, mountCmd.Long)
}

func TestMountCmd_HasSubcommands(t *testing.T) {
	assert.True(t, mountCmd.HasAvailableSubCommands())
	commands := mountCmd.Commands()
	assert.NotEmpty(t, commands)
}

func TestMountListCmd_Structure(t *testing.T) {
	assert.NotNil(t, mountListCmd)
	assert.Equal(t, "list", mountListCmd.Use)
	assert.NotEmpty(t, mountListCmd.Short)
	assert.NotNil(t, mountListCmd.RunE)
}

func TestMountMountCmd_Structure(t *testing.T) {
	assert.NotNil(t, mountMountCmd)
	assert.Equal(t, "mount <device> <mountpoint> <fstype>", mountMountCmd.Use)
	assert.NotEmpty(t, mountMountCmd.Short)
	assert.NotEmpty(t, mountMountCmd.Example)
	assert.NotNil(t, mountMountCmd.RunE)
}

func TestMountUnmountCmd_Structure(t *testing.T) {
	assert.NotNil(t, mountUnmountCmd)
	assert.Equal(t, "umount <mountpoint>", mountUnmountCmd.Use)
	assert.NotEmpty(t, mountUnmountCmd.Short)
	assert.NotNil(t, mountUnmountCmd.RunE)
}

// Test all salt advanced subcommands are registered
func TestSaltAdvancedSubcommands_RegisteredWithSalt(t *testing.T) {
	saltSubcommands := saltCmd.Commands()
	subcommandNames := make(map[string]bool)
	for _, cmd := range saltSubcommands {
		subcommandNames[cmd.Name()] = true
	}

	expectedSubcommands := []string{
		"network",
		"process",
		"cron",
		"archive",
		"monitor",
		"ssh",
		"git",
		"k8s",
		"pillar",
		"mount",
	}

	for _, name := range expectedSubcommands {
		assert.True(t, subcommandNames[name], "Expected salt subcommand '%s' should exist", name)
	}
}

// Test command descriptions contain expected keywords
func TestSaltAdvancedCmd_Descriptions(t *testing.T) {
	tests := []struct {
		name     string
		short    string
		keywords []string
	}{
		{"network", networkCmd.Short, []string{"network"}},
		{"process", processCmd.Short, []string{"Process"}},
		{"cron", cronCmd.Short, []string{"Cron"}},
		{"archive", archiveCmd.Short, []string{"Archive"}},
		{"monitor", monitorCmd.Short, []string{"Monitor"}},
		{"ssh", sshCmd.Short, []string{"SSH"}},
		{"git", gitCmd.Short, []string{"Git"}},
		{"k8s", k8sCmd.Short, []string{"Kubernetes"}},
		{"pillar", pillarCmd.Short, []string{"pillar"}},
		{"mount", mountCmd.Short, []string{"mount"}},
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

// Test network command examples
func TestNetworkCmd_Examples(t *testing.T) {
	examples := networkPingCmd.Example
	assert.Contains(t, examples, "ping")
	assert.Contains(t, examples, "8.8.8.8")
}

// Test archive command formats
func TestArchiveCmd_SupportedFormats(t *testing.T) {
	formats := []string{"tar", "zip"}

	archiveSubcommands := archiveCmd.Commands()
	subcommandNames := make(map[string]bool)
	for _, cmd := range archiveSubcommands {
		subcommandNames[cmd.Name()] = true
	}

	for _, format := range formats {
		assert.True(t, subcommandNames[format], "Archive should support %s format", format)
		assert.True(t, subcommandNames["un"+format], "Archive should support un%s format", format)
	}
}

// Test k8s command aliases
func TestK8sCmd_Aliases(t *testing.T) {
	assert.Contains(t, k8sCmd.Aliases, "kubectl", "k8s should have kubectl alias")
}

// Test SSH key types in example
func TestSSHKeyGenCmd_KeyTypes(t *testing.T) {
	examples := sshKeyGenCmd.Example
	assert.Contains(t, examples, "rsa")
	assert.Contains(t, examples, "ed25519")
}

// Test git command operations
func TestGitCmd_Operations(t *testing.T) {
	gitSubcommands := gitCmd.Commands()
	operations := make(map[string]bool)
	for _, cmd := range gitSubcommands {
		operations[cmd.Name()] = true
	}

	assert.True(t, operations["clone"], "Git should have clone command")
	assert.True(t, operations["pull"], "Git should have pull command")
}

// Test mount command filesystem examples
func TestMountCmd_FilesystemExample(t *testing.T) {
	examples := mountMountCmd.Example
	assert.Contains(t, examples, "ext4")
	assert.Contains(t, examples, "/dev/")
	assert.Contains(t, examples, "/mnt/")
}

// Test cron schedule format in example
func TestCronAddCmd_ScheduleExample(t *testing.T) {
	examples := cronAddCmd.Example
	// Should contain cron time fields
	assert.Contains(t, examples, "0")
	assert.Contains(t, examples, "2")
	assert.Contains(t, examples, "*")
}

// Test process kill signals
func TestProcessKillCmd_SignalExample(t *testing.T) {
	examples := processKillCmd.Example
	assert.Contains(t, examples, "SIGTERM")
}

// Test k8s get resources example
func TestK8sGetCmd_ResourcesExample(t *testing.T) {
	examples := k8sGetCmd.Example
	assert.Contains(t, examples, "nodes")
	assert.Contains(t, examples, "pods")
	assert.Contains(t, examples, "deployments")
}
