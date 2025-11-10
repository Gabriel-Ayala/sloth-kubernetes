package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVPNCmd_Structure(t *testing.T) {
	assert.NotNil(t, vpnCmd)
	assert.Equal(t, "vpn", vpnCmd.Use)
	assert.NotEmpty(t, vpnCmd.Short)
	assert.NotEmpty(t, vpnCmd.Long)
}

func TestVPNCmd_HasSubcommands(t *testing.T) {
	assert.True(t, vpnCmd.HasAvailableSubCommands(), "vpn command should have subcommands")
	commands := vpnCmd.Commands()
	assert.NotEmpty(t, commands, "vpn command should have registered subcommands")
}

func TestVPNStatusCmd_Structure(t *testing.T) {
	assert.NotNil(t, vpnStatusCmd)
	assert.Equal(t, "status [stack-name]", vpnStatusCmd.Use)
	assert.NotEmpty(t, vpnStatusCmd.Short)
	assert.NotEmpty(t, vpnStatusCmd.Long)
	assert.NotEmpty(t, vpnStatusCmd.Example)
	assert.NotNil(t, vpnStatusCmd.RunE)
}

func TestVPNStatusCmd_Examples(t *testing.T) {
	examples := vpnStatusCmd.Example
	assert.Contains(t, examples, "vpn status")
	assert.Contains(t, examples, "production")
}

func TestVPNPeersCmd_Structure(t *testing.T) {
	assert.NotNil(t, vpnPeersCmd)
	assert.Equal(t, "peers [stack-name]", vpnPeersCmd.Use)
	assert.NotEmpty(t, vpnPeersCmd.Short)
	assert.NotEmpty(t, vpnPeersCmd.Long)
	assert.NotEmpty(t, vpnPeersCmd.Example)
	assert.NotNil(t, vpnPeersCmd.RunE)
}

func TestVPNPeersCmd_Examples(t *testing.T) {
	examples := vpnPeersCmd.Example
	assert.Contains(t, examples, "vpn peers")
	assert.Contains(t, examples, "production")
}

func TestVPNConfigCmd_Structure(t *testing.T) {
	assert.NotNil(t, vpnConfigCmd)
	assert.Equal(t, "config [stack-name] [node-name]", vpnConfigCmd.Use)
	assert.NotEmpty(t, vpnConfigCmd.Short)
	assert.NotEmpty(t, vpnConfigCmd.Long)
	assert.NotEmpty(t, vpnConfigCmd.Example)
	assert.NotNil(t, vpnConfigCmd.RunE)
}

func TestVPNConfigCmd_Examples(t *testing.T) {
	examples := vpnConfigCmd.Example
	assert.Contains(t, examples, "vpn config")
	assert.Contains(t, examples, "master-1")
}

func TestVPNTestCmd_Structure(t *testing.T) {
	assert.NotNil(t, vpnTestCmd)
	assert.Equal(t, "test [stack-name]", vpnTestCmd.Use)
	assert.NotEmpty(t, vpnTestCmd.Short)
	assert.NotEmpty(t, vpnTestCmd.Long)
	assert.NotEmpty(t, vpnTestCmd.Example)
	assert.NotNil(t, vpnTestCmd.RunE)
}

func TestVPNJoinCmd_Structure(t *testing.T) {
	assert.NotNil(t, vpnJoinCmd)
	assert.Equal(t, "join [stack-name]", vpnJoinCmd.Use)
	assert.NotEmpty(t, vpnJoinCmd.Short)
	assert.NotEmpty(t, vpnJoinCmd.Long)
	assert.NotEmpty(t, vpnJoinCmd.Example)
	assert.NotNil(t, vpnJoinCmd.RunE)
}

func TestVPNJoinCmd_Flags(t *testing.T) {
	remoteFlag := vpnJoinCmd.Flags().Lookup("remote")
	assert.NotNil(t, remoteFlag, "join should have --remote flag")

	vpnIPFlag := vpnJoinCmd.Flags().Lookup("vpn-ip")
	assert.NotNil(t, vpnIPFlag, "join should have --vpn-ip flag")

	labelFlag := vpnJoinCmd.Flags().Lookup("label")
	assert.NotNil(t, labelFlag, "join should have --label flag")

	installFlag := vpnJoinCmd.Flags().Lookup("install")
	assert.NotNil(t, installFlag, "join should have --install flag")
}

func TestVPNJoinCmd_Examples(t *testing.T) {
	examples := vpnJoinCmd.Example
	assert.Contains(t, examples, "vpn join")
	assert.Contains(t, examples, "--remote")
	assert.Contains(t, examples, "--vpn-ip")
	assert.Contains(t, examples, "--install")
}

func TestVPNLeaveCmd_Structure(t *testing.T) {
	assert.NotNil(t, vpnLeaveCmd)
	assert.Equal(t, "leave [stack-name]", vpnLeaveCmd.Use)
	assert.NotEmpty(t, vpnLeaveCmd.Short)
	assert.NotNil(t, vpnLeaveCmd.RunE)
}

func TestVPNLeaveCmd_Flags(t *testing.T) {
	vpnIPFlag := vpnLeaveCmd.Flags().Lookup("vpn-ip")
	assert.NotNil(t, vpnIPFlag, "leave should have --vpn-ip flag")
}

func TestVPNClientConfigCmd_Structure(t *testing.T) {
	assert.NotNil(t, vpnClientConfigCmd)
	assert.Equal(t, "client-config [stack-name]", vpnClientConfigCmd.Use)
	assert.NotEmpty(t, vpnClientConfigCmd.Short)
	assert.NotNil(t, vpnClientConfigCmd.RunE)
}

func TestVPNClientConfigCmd_Flags(t *testing.T) {
	outputFlag := vpnClientConfigCmd.Flags().Lookup("output")
	assert.NotNil(t, outputFlag, "client-config should have --output flag")

	qrFlag := vpnClientConfigCmd.Flags().Lookup("qr")
	assert.NotNil(t, qrFlag, "client-config should have --qr flag")
}

func TestVPNCmd_SubcommandsList(t *testing.T) {
	commands := vpnCmd.Commands()
	commandNames := make(map[string]bool)
	for _, cmd := range commands {
		commandNames[cmd.Name()] = true
	}

	expectedCommands := []string{"status", "peers", "config", "test", "join", "leave", "client-config"}
	for _, cmdName := range expectedCommands {
		assert.True(t, commandNames[cmdName], "Expected VPN subcommand '%s' should exist", cmdName)
	}
}

func TestVPNGlobalVariables(t *testing.T) {
	assert.IsType(t, "", vpnJoinRemote)
	assert.IsType(t, "", vpnJoinIP)
	assert.IsType(t, "", vpnJoinLabel)
	assert.IsType(t, false, vpnJoinInstall)
	assert.IsType(t, "", vpnLeaveIP)
	assert.IsType(t, "", vpnConfigOutput)
	assert.IsType(t, false, vpnConfigQR)
}

func TestVPNCmd_LongDescription(t *testing.T) {
	long := vpnCmd.Long
	assert.Contains(t, long, "WireGuard")
	assert.Contains(t, long, "VPN")
}

func TestVPNStatusCmd_LongDescription(t *testing.T) {
	long := vpnStatusCmd.Long
	assert.Contains(t, long, "status")
	assert.Contains(t, long, "WireGuard")
	assert.Contains(t, long, "VPN")
	assert.Contains(t, long, "tunnels")
}

func TestVPNPeersCmd_LongDescription(t *testing.T) {
	long := vpnPeersCmd.Long
	assert.Contains(t, long, "peers")
	assert.Contains(t, long, "VPN")
	assert.Contains(t, long, "mesh")
}

func TestVPNConfigCmd_LongDescription(t *testing.T) {
	long := vpnConfigCmd.Long
	assert.Contains(t, long, "WireGuard")
	assert.Contains(t, long, "configuration")
}

func TestVPNTestCmd_LongDescription(t *testing.T) {
	long := vpnTestCmd.Long
	assert.Contains(t, long, "connectivity")
	assert.Contains(t, long, "VPN")
	assert.Contains(t, long, "mesh")
}

func TestVPNJoinCmd_LongDescription(t *testing.T) {
	long := vpnJoinCmd.Long
	assert.Contains(t, long, "machine")
	assert.Contains(t, long, "VPN")
	assert.Contains(t, long, "WireGuard")
	assert.Contains(t, long, "mesh")
}

func TestVPNLeaveCmd_LongDescription(t *testing.T) {
	long := vpnLeaveCmd.Long
	assert.Contains(t, long, "Remove")
	assert.Contains(t, long, "VPN")
}

func TestVPNClientConfigCmd_LongDescription(t *testing.T) {
	long := vpnClientConfigCmd.Long
	assert.Contains(t, long, "WireGuard")
	assert.Contains(t, long, "configuration")
}

func TestVPNCmd_RegisteredWithRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "vpn" {
			found = true
			break
		}
	}
	assert.True(t, found, "vpn command should be registered with root")
}

func TestVPNTestCmd_Examples(t *testing.T) {
	examples := vpnTestCmd.Example
	assert.Contains(t, examples, "vpn test")
	assert.Contains(t, examples, "production")
}

func TestVPNLeaveCmd_Examples(t *testing.T) {
	examples := vpnLeaveCmd.Example
	assert.Contains(t, examples, "vpn leave")
}

func TestVPNClientConfigCmd_Examples(t *testing.T) {
	examples := vpnClientConfigCmd.Example
	assert.Contains(t, examples, "client-config")
}
