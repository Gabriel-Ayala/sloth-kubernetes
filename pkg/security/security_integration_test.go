package security

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// SecurityIntegrationMockProvider implements pulumi mock provider for integration tests
type SecurityIntegrationMockProvider struct {
	pulumi.ResourceState
}

func (m *SecurityIntegrationMockProvider) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	outputs := args.Inputs.Copy()

	// For TLS resources
	if args.TypeToken == "tls:index/privateKey:PrivateKey" {
		outputs["privateKeyPem"] = resource.NewStringProperty("-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----")
		outputs["publicKeyOpenssh"] = resource.NewStringProperty("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...")
	}

	// For remote command resources
	if args.TypeToken == "command:remote:Command" {
		outputs["stdout"] = resource.NewStringProperty("SUCCESS\nFIREWALL_TYPE:ufw\nRules applied successfully")
		outputs["stderr"] = resource.NewStringProperty("")
	}

	return args.Name + "_id", outputs, nil
}

func (m *SecurityIntegrationMockProvider) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

// TestSecurityIntegration_SSHAndWireGuard tests integration of SSH keys and WireGuard
func TestSecurityIntegration_SSHAndWireGuard(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// 1. Generate SSH keys
		sshManager := NewSSHKeyManager(ctx)
		err := sshManager.GenerateKeyPair()
		assert.NoError(t, err)

		publicKey := sshManager.GetPublicKey()
		assert.NotNil(t, publicKey)

		// 2. Setup WireGuard
		wgConfig := &config.WireGuardConfig{
			Enabled:             true,
			Port:                51820,
			MTU:                 1420,
			DNS:                 []string{"1.1.1.1", "8.8.8.8"},
			ServerPublicKey:     "TEST_SERVER_PUBLIC_KEY",
			ServerEndpoint:      "vpn.example.com",
			AllowedIPs:          []string{"10.8.0.0/24"},
			PersistentKeepalive: 25,
			SSHPrivateKeyPath:   "/path/to/key",
		}

		wgManager := NewWireGuardManager(ctx, wgConfig)

		// 3. Validate configuration
		err = wgManager.ValidateConfiguration()
		assert.NoError(t, err)

		// 4. Configure nodes
		nodes := []*providers.NodeOutput{
			{
				Name:        "master-1",
				PublicIP:    pulumi.String("203.0.113.1").ToStringOutput(),
				WireGuardIP: "10.8.0.1",
				SSHUser:     "root",
			},
			{
				Name:        "worker-1",
				PublicIP:    pulumi.String("203.0.113.2").ToStringOutput(),
				WireGuardIP: "10.8.0.2",
				SSHUser:     "root",
			},
			{
				Name:        "worker-2",
				PublicIP:    pulumi.String("203.0.113.3").ToStringOutput(),
				WireGuardIP: "10.8.0.3",
				SSHUser:     "root",
			},
		}

		for _, node := range nodes {
			err := wgManager.ConfigureNode(node)
			assert.NoError(t, err)
		}

		assert.Len(t, wgManager.nodes, 3)

		// 5. Export information
		wgManager.ExportWireGuardInfo()

		nodeIPs := []string{}
		for _, node := range nodes {
			nodeIPs = append(nodeIPs, node.WireGuardIP)
		}
		sshManager.ExportSSHAccess(nodeIPs)

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityIntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestSecurityIntegration_FirewallConfiguration tests firewall configuration workflow
func TestSecurityIntegration_FirewallConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// 1. Create firewall manager
		fwManager := NewOSFirewallManager(ctx)
		fwManager.SetSSHKeyPath("/path/to/key")

		// 2. Add nodes with different roles
		masterNode := &providers.NodeOutput{
			Name:     "master-1",
			PublicIP: pulumi.String("203.0.113.1").ToStringOutput(),
			Labels:   map[string]string{"role": "master"},
			SSHUser:  "root",
		}

		workerNode := &providers.NodeOutput{
			Name:     "worker-1",
			PublicIP: pulumi.String("203.0.113.2").ToStringOutput(),
			Labels:   map[string]string{"role": "worker"},
			SSHUser:  "root",
		}

		fwManager.AddNode(masterNode)
		fwManager.AddNode(workerNode)

		assert.Len(t, fwManager.nodes, 2)

		// 3. Verify rules are correct for each node type
		masterRules := fwManager.getRulesForNode(masterNode)
		workerRules := fwManager.getRulesForNode(workerNode)

		// Master should have more rules
		assert.Greater(t, len(masterRules), len(workerRules))

		// Master should have API server rule
		hasMasterRules := false
		for _, rule := range masterRules {
			if rule.Port == "6443" { // API Server
				hasMasterRules = true
				break
			}
		}
		assert.True(t, hasMasterRules)

		// Worker should have NodePort rules
		hasNodePortRules := false
		for _, rule := range workerRules {
			if rule.Port == "30000:32767" { // NodePort
				hasNodePortRules = true
				break
			}
		}
		assert.True(t, hasNodePortRules)

		// 4. Get results
		results := fwManager.GetResults()
		assert.Len(t, results, 2)

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityIntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestSecurityIntegration_CompleteSecuritySetup tests complete security setup
func TestSecurityIntegration_CompleteSecuritySetup(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// 1. SSH Keys
		sshManager := NewSSHKeyManager(ctx)
		err := sshManager.GenerateKeyPair()
		assert.NoError(t, err)

		// 2. WireGuard VPN
		wgConfig := &config.WireGuardConfig{
			Enabled:             true,
			Port:                51820,
			MTU:                 1420,
			DNS:                 []string{"1.1.1.1"},
			ServerPublicKey:     "TEST_PUBLIC_KEY",
			ServerEndpoint:      "vpn.example.com",
			AllowedIPs:          []string{"10.8.0.0/24"},
			PersistentKeepalive: 25,
			SSHPrivateKeyPath:   "/path/to/key",
			MeshNetworking:      true,
		}

		wgManager := NewWireGuardManager(ctx, wgConfig)
		err = wgManager.ValidateConfiguration()
		assert.NoError(t, err)

		// 3. OS Firewall
		fwManager := NewOSFirewallManager(ctx)
		fwManager.SetSSHKeyPath("/path/to/key")

		// 4. Create cluster nodes
		nodes := []*providers.NodeOutput{
			{
				Name:        "master-1",
				PublicIP:    pulumi.String("203.0.113.1").ToStringOutput(),
				WireGuardIP: "10.8.0.1",
				Labels:      map[string]string{"role": "master"},
				SSHUser:     "root",
			},
			{
				Name:        "master-2",
				PublicIP:    pulumi.String("203.0.113.2").ToStringOutput(),
				WireGuardIP: "10.8.0.2",
				Labels:      map[string]string{"role": "master"},
				SSHUser:     "root",
			},
			{
				Name:        "worker-1",
				PublicIP:    pulumi.String("203.0.113.3").ToStringOutput(),
				WireGuardIP: "10.8.0.3",
				Labels:      map[string]string{"role": "worker"},
				SSHUser:     "root",
			},
		}

		// 5. Configure each node
		for _, node := range nodes {
			// Configure WireGuard
			err := wgManager.ConfigureNode(node)
			assert.NoError(t, err)

			// Add to firewall manager
			fwManager.AddNode(node)

			// Check reachability
			reachable := wgManager.IsNodeReachable(node)
			assert.NotNil(t, reachable)
		}

		assert.Len(t, wgManager.nodes, 3)
		assert.Len(t, fwManager.nodes, 3)

		// 6. Configure server peers
		err = wgManager.ConfigureServerPeers()
		assert.NoError(t, err)

		// 7. Export all information
		wgManager.ExportWireGuardInfo()

		nodeIPs := []string{}
		for _, node := range nodes {
			nodeIPs = append(nodeIPs, node.WireGuardIP)
		}
		sshManager.ExportSSHAccess(nodeIPs)

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityIntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestSecurityIntegration_WireGuardMeshNetworking tests mesh networking setup
func TestSecurityIntegration_WireGuardMeshNetworking(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		wgConfig := &config.WireGuardConfig{
			Enabled:             true,
			Port:                51820,
			MTU:                 1420,
			DNS:                 []string{"8.8.8.8"},
			ServerPublicKey:     "SERVER_KEY",
			ServerEndpoint:      "vpn.example.com",
			AllowedIPs:          []string{"10.8.0.0/24"},
			PersistentKeepalive: 25,
			MeshNetworking:      true, // Enable mesh
		}

		manager := NewWireGuardManager(ctx, wgConfig)

		// Create mesh of 5 nodes
		nodeCount := 5
		for i := 0; i < nodeCount; i++ {
			node := &providers.NodeOutput{
				Name:        "node-" + string(rune('1'+i)),
				PublicIP:    pulumi.Sprintf("203.0.113.%d", i+1).ToStringOutput(),
				WireGuardIP: "10.8.0." + string(rune('1'+i)),
				SSHUser:     "root",
			}

			err := manager.ConfigureNode(node)
			assert.NoError(t, err)
		}

		assert.Len(t, manager.nodes, nodeCount)

		// Each node should be able to find other nodes by WireGuard IP
		node, err := manager.GetNodeByWireGuardIP("10.8.0.1")
		assert.NoError(t, err)
		assert.NotNil(t, node)

		// Export mesh info
		manager.ExportWireGuardInfo()

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityIntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestSecurityIntegration_FirewallScriptGeneration tests firewall script generation
func TestSecurityIntegration_FirewallScriptGeneration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewOSFirewallManager(ctx)

		// Test different node configurations
		testNodes := []*providers.NodeOutput{
			{
				Name:   "master-node",
				Labels: map[string]string{"role": "master"},
			},
			{
				Name:   "worker-node",
				Labels: map[string]string{"role": "worker"},
			},
			{
				Name:   "control-plane",
				Labels: map[string]string{"role": "controlplane"},
			},
		}

		for _, node := range testNodes {
			rules := manager.getRulesForNode(node)
			assert.NotEmpty(t, rules)

			script := manager.generateFirewallScript(node, rules)
			assert.NotEmpty(t, script)

			// Verify script contains required elements
			assert.Contains(t, script, "#!/bin/bash")
			assert.Contains(t, script, node.Name)
			assert.Contains(t, script, "configure_ufw")
			assert.Contains(t, script, "configure_firewalld")
			assert.Contains(t, script, "configure_iptables")
			assert.Contains(t, script, "SUCCESS")

			// Verify all rules are in the script
			for _, rule := range rules {
				assert.Contains(t, script, rule.Port)
			}
		}

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityIntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestSecurityIntegration_MultiRoleNode tests node with multiple roles
func TestSecurityIntegration_MultiRoleNode(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewOSFirewallManager(ctx)

		// Single node acting as both master and worker (single-node cluster)
		node := &providers.NodeOutput{
			Name: "single-node",
			Labels: map[string]string{
				"role": "master",
			},
		}

		manager.AddNode(node)

		rules := manager.getRulesForNode(node)
		assert.NotEmpty(t, rules)

		// Should have master-specific rules
		hasMasterRules := false
		for _, rule := range rules {
			if rule.Port == "6443" || rule.Port == "2379" {
				hasMasterRules = true
				break
			}
		}
		assert.True(t, hasMasterRules)

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityIntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestSecurityIntegration_WireGuardValidation tests WireGuard validation
func TestSecurityIntegration_WireGuardValidation(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		tests := []struct {
			name        string
			config      *config.WireGuardConfig
			expectError bool
		}{
			{
				name: "Valid configuration",
				config: &config.WireGuardConfig{
					Enabled:         true,
					ServerEndpoint:  "vpn.example.com",
					ServerPublicKey: "PUBLIC_KEY",
					AllowedIPs:      []string{"10.8.0.0/24"},
				},
				expectError: false,
			},
			{
				name: "Missing endpoint",
				config: &config.WireGuardConfig{
					Enabled:         true,
					ServerEndpoint:  "",
					ServerPublicKey: "PUBLIC_KEY",
					AllowedIPs:      []string{"10.8.0.0/24"},
				},
				expectError: true,
			},
			{
				name: "Missing public key",
				config: &config.WireGuardConfig{
					Enabled:         true,
					ServerEndpoint:  "vpn.example.com",
					ServerPublicKey: "",
					AllowedIPs:      []string{"10.8.0.0/24"},
				},
				expectError: true,
			},
			{
				name: "Missing allowed IPs",
				config: &config.WireGuardConfig{
					Enabled:         true,
					ServerEndpoint:  "vpn.example.com",
					ServerPublicKey: "PUBLIC_KEY",
					AllowedIPs:      []string{},
				},
				expectError: true,
			},
			{
				name: "Disabled configuration",
				config: &config.WireGuardConfig{
					Enabled: false,
				},
				expectError: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				manager := NewWireGuardManager(ctx, tt.config)
				err := manager.ValidateConfiguration()

				if tt.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityIntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestSecurityIntegration_LocalKeyGeneration tests local key generation fallback
func TestSecurityIntegration_LocalKeyGeneration(t *testing.T) {
	// Test local SSH key generation (non-Pulumi)
	privateKey, publicKey, err := GenerateLocalKeyPair()
	assert.NoError(t, err)
	assert.NotEmpty(t, privateKey)
	assert.NotEmpty(t, publicKey)

	// Verify key format
	assert.Contains(t, privateKey, "BEGIN RSA PRIVATE KEY")
	assert.Contains(t, privateKey, "END RSA PRIVATE KEY")
	assert.Contains(t, publicKey, "ssh-rsa")

	// Test WireGuard key generation
	wgPrivate, wgPublic, err := GenerateKeyPair()
	assert.NoError(t, err)
	assert.NotEmpty(t, wgPrivate)
	assert.NotEmpty(t, wgPublic)
}
