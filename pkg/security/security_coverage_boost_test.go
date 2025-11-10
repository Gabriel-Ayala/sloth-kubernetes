package security

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// SecurityTestMockProvider implements pulumi mock provider for security tests
type SecurityTestMockProvider struct {
	pulumi.ResourceState
}

func (m *SecurityTestMockProvider) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	outputs := args.Inputs.Copy()

	// For TLS resources, add required outputs
	if args.TypeToken == "tls:index/privateKey:PrivateKey" {
		outputs["privateKeyPem"] = resource.NewStringProperty("-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----")
		outputs["publicKeyOpenssh"] = resource.NewStringProperty("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...")
		outputs["publicKeyPem"] = resource.NewStringProperty("-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----")
	}

	// For remote command resources
	if args.TypeToken == "command:remote:Command" {
		outputs["stdout"] = resource.NewStringProperty("SUCCESS\nFIREWALL_TYPE:ufw")
		outputs["stderr"] = resource.NewStringProperty("")
	}

	return args.Name + "_id", outputs, nil
}

func (m *SecurityTestMockProvider) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

// TestNewSSHKeyManager tests SSH key manager creation
func TestNewSSHKeyManager(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewSSHKeyManager(ctx)

		assert.NotNil(t, manager)
		assert.Equal(t, ctx, manager.ctx)

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestSSHKeyManager_GenerateKeyPair tests key pair generation
func TestSSHKeyManager_GenerateKeyPair(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewSSHKeyManager(ctx)

		err := manager.GenerateKeyPair()
		assert.NoError(t, err)

		assert.NotNil(t, manager.keyPair)

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestSSHKeyManager_GetPublicKey tests getting public key
func TestSSHKeyManager_GetPublicKey(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewSSHKeyManager(ctx)
		manager.GenerateKeyPair()

		publicKey := manager.GetPublicKey()
		assert.NotNil(t, publicKey)

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestSSHKeyManager_GetPrivateKey tests getting private key
func TestSSHKeyManager_GetPrivateKey(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewSSHKeyManager(ctx)
		manager.GenerateKeyPair()

		privateKey := manager.GetPrivateKey()
		assert.NotNil(t, privateKey)

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestSSHKeyManager_GetPublicKeyString tests getting public key as string
func TestSSHKeyManager_GetPublicKeyString(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewSSHKeyManager(ctx)
		manager.GenerateKeyPair()

		publicKeyString := manager.GetPublicKeyString()
		assert.NotNil(t, publicKeyString)

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestSSHKeyManager_GetPrivateKeyString tests getting private key as string
func TestSSHKeyManager_GetPrivateKeyString(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewSSHKeyManager(ctx)
		manager.GenerateKeyPair()

		privateKeyString := manager.GetPrivateKeyString()
		assert.NotNil(t, privateKeyString)

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestSSHKeyManager_ExportSSHAccess tests exporting SSH access info
func TestSSHKeyManager_ExportSSHAccess(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewSSHKeyManager(ctx)
		manager.GenerateKeyPair()

		nodes := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}
		manager.ExportSSHAccess(nodes)

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestWireGuardManager_ConfigureNode tests WireGuard node configuration
func TestWireGuardManager_ConfigureNode(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		wgConfig := &config.WireGuardConfig{
			Enabled:             true,
			Port:                51820,
			MTU:                 1420,
			DNS:                 []string{"1.1.1.1", "8.8.8.8"},
			ServerPublicKey:     "SERVER_PUBLIC_KEY",
			ServerEndpoint:      "vpn.example.com",
			AllowedIPs:          []string{"10.8.0.0/24"},
			PersistentKeepalive: 25,
			MeshNetworking:      false,
		}

		manager := NewWireGuardManager(ctx, wgConfig)

		node := &providers.NodeOutput{
			Name:        "worker-1",
			PublicIP:    pulumi.String("203.0.113.10").ToStringOutput(),
			WireGuardIP: "10.8.0.10",
			SSHUser:     "root",
		}

		err := manager.ConfigureNode(node)
		assert.NoError(t, err)

		// Verify node was added
		assert.Len(t, manager.nodes, 1)

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestWireGuardManager_ConfigureNode_Disabled tests when WireGuard is disabled
func TestWireGuardManager_ConfigureNode_Disabled(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		wgConfig := &config.WireGuardConfig{
			Enabled: false,
		}

		manager := NewWireGuardManager(ctx, wgConfig)

		node := &providers.NodeOutput{
			Name:        "worker-1",
			PublicIP:    pulumi.String("203.0.113.10").ToStringOutput(),
			WireGuardIP: "10.8.0.10",
		}

		err := manager.ConfigureNode(node)
		assert.NoError(t, err)

		// Node should not be added when disabled
		assert.Len(t, manager.nodes, 0)

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestWireGuardManager_ConfigureServerPeers tests server peer configuration
func TestWireGuardManager_ConfigureServerPeers(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		wgConfig := &config.WireGuardConfig{
			Enabled:        true,
			ServerEndpoint: "vpn.example.com",
		}

		manager := NewWireGuardManager(ctx, wgConfig)

		// Add some nodes first
		manager.nodes = append(manager.nodes, &providers.NodeOutput{
			Name:        "worker-1",
			WireGuardIP: "10.8.0.10",
		})
		manager.nodes = append(manager.nodes, &providers.NodeOutput{
			Name:        "worker-2",
			WireGuardIP: "10.8.0.11",
		})

		err := manager.ConfigureServerPeers()
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestWireGuardManager_ConfigureServerPeers_Disabled tests when disabled
func TestWireGuardManager_ConfigureServerPeers_Disabled(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		wgConfig := &config.WireGuardConfig{
			Enabled: false,
		}

		manager := NewWireGuardManager(ctx, wgConfig)

		err := manager.ConfigureServerPeers()
		assert.NoError(t, err) // Should return nil when disabled

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestWireGuardManager_ConfigureServerPeers_NoEndpoint tests when no endpoint
func TestWireGuardManager_ConfigureServerPeers_NoEndpoint(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		wgConfig := &config.WireGuardConfig{
			Enabled:        true,
			ServerEndpoint: "",
		}

		manager := NewWireGuardManager(ctx, wgConfig)

		err := manager.ConfigureServerPeers()
		assert.NoError(t, err) // Should return nil when no endpoint

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestWireGuardManager_ExportWireGuardInfo tests exporting WireGuard info
func TestWireGuardManager_ExportWireGuardInfo(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		wgConfig := &config.WireGuardConfig{
			Enabled:        true,
			ServerEndpoint: "vpn.example.com",
			Port:           51820,
		}

		manager := NewWireGuardManager(ctx, wgConfig)

		// Add nodes
		manager.nodes = append(manager.nodes, &providers.NodeOutput{
			Name:        "worker-1",
			WireGuardIP: "10.8.0.10",
		})
		manager.nodes = append(manager.nodes, &providers.NodeOutput{
			Name:        "worker-2",
			WireGuardIP: "10.8.0.11",
		})

		manager.ExportWireGuardInfo()

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestWireGuardManager_ExportWireGuardInfo_Disabled tests export when disabled
func TestWireGuardManager_ExportWireGuardInfo_Disabled(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		wgConfig := &config.WireGuardConfig{
			Enabled: false,
		}

		manager := NewWireGuardManager(ctx, wgConfig)
		manager.ExportWireGuardInfo()

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestWireGuardManager_IsNodeReachable tests node reachability check
func TestWireGuardManager_IsNodeReachable(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		wgConfig := &config.WireGuardConfig{
			Enabled: true,
		}

		manager := NewWireGuardManager(ctx, wgConfig)

		node := &providers.NodeOutput{
			Name:     "worker-1",
			PublicIP: pulumi.String("203.0.113.10").ToStringOutput(),
		}

		reachable := manager.IsNodeReachable(node)
		assert.NotNil(t, reachable)

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestOSFirewallManager_ConfigureAllNodesFirewall tests firewall configuration on all nodes
func TestOSFirewallManager_ConfigureAllNodesFirewall(t *testing.T) {
	t.Skip("Skipping test that requires actual SSH connections and complex goroutine coordination")
}

// TestOSFirewallManager_PrintFirewallSummary tests printing firewall summary
func TestOSFirewallManager_PrintFirewallSummary(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewOSFirewallManager(ctx)

		// Add some mock results
		manager.results["node1"] = &FirewallResult{
			NodeName:     "node1",
			Success:      true,
			FirewallType: "ufw",
			RulesApplied: []FirewallRule{
				KubernetesFirewallPorts.SSH,
				KubernetesFirewallPorts.WireGuard,
			},
		}

		manager.results["node2"] = &FirewallResult{
			NodeName:     "node2",
			Success:      false,
			FirewallType: "iptables",
			Error:        assert.AnError,
		}

		// This should not panic
		manager.printFirewallSummary()

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestWireGuardManager_getSSHPrivateKey tests getting SSH private key
func TestWireGuardManager_getSSHPrivateKey(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		wgConfig := &config.WireGuardConfig{
			Enabled:           true,
			SSHPrivateKeyPath: "/path/to/key",
		}

		manager := NewWireGuardManager(ctx, wgConfig)

		key := manager.getSSHPrivateKey()
		assert.NotEmpty(t, key)
		assert.Equal(t, "SSH_PRIVATE_KEY_CONTENT", key)

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestWireGuardManager_getSSHPrivateKey_Empty tests when no SSH key path
func TestWireGuardManager_getSSHPrivateKey_Empty(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		wgConfig := &config.WireGuardConfig{
			Enabled:           true,
			SSHPrivateKeyPath: "",
		}

		manager := NewWireGuardManager(ctx, wgConfig)

		key := manager.getSSHPrivateKey()
		assert.Empty(t, key)

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestOSFirewallManager_getSSHPrivateKey tests getting SSH private key
func TestOSFirewallManager_getSSHPrivateKey(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewOSFirewallManager(ctx)
		manager.SetSSHKeyPath("/path/to/key")

		key := manager.getSSHPrivateKey()
		assert.NotEmpty(t, key)
		assert.Equal(t, "SSH_PRIVATE_KEY_CONTENT", key)

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestWireGuardManager_ConfigureNode_WithMeshNetworking tests with mesh networking enabled
func TestWireGuardManager_ConfigureNode_WithMeshNetworking(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		wgConfig := &config.WireGuardConfig{
			Enabled:             true,
			Port:                51820,
			MTU:                 1420,
			DNS:                 []string{"1.1.1.1"},
			ServerPublicKey:     "SERVER_PUBLIC_KEY",
			ServerEndpoint:      "vpn.example.com",
			AllowedIPs:          []string{"10.8.0.0/24"},
			PersistentKeepalive: 25,
			MeshNetworking:      true, // Enable mesh
		}

		manager := NewWireGuardManager(ctx, wgConfig)

		// Add first node
		node1 := &providers.NodeOutput{
			Name:        "worker-1",
			PublicIP:    pulumi.String("203.0.113.10").ToStringOutput(),
			WireGuardIP: "10.8.0.10",
			SSHUser:     "root",
		}

		err := manager.ConfigureNode(node1)
		assert.NoError(t, err)

		// Add second node
		node2 := &providers.NodeOutput{
			Name:        "worker-2",
			PublicIP:    pulumi.String("203.0.113.11").ToStringOutput(),
			WireGuardIP: "10.8.0.11",
			SSHUser:     "root",
		}

		err = manager.ConfigureNode(node2)
		assert.NoError(t, err)

		assert.Len(t, manager.nodes, 2)

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestWireGuardManager_generateNodeConfig tests node config generation
func TestWireGuardManager_generateNodeConfig(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		wgConfig := &config.WireGuardConfig{
			Enabled:             true,
			Port:                51820,
			MTU:                 1420,
			DNS:                 []string{"1.1.1.1", "8.8.8.8"},
			ServerPublicKey:     "SERVER_PUBLIC_KEY",
			ServerEndpoint:      "vpn.example.com",
			AllowedIPs:          []string{"10.8.0.0/24"},
			PersistentKeepalive: 25,
			MeshNetworking:      false,
		}

		manager := NewWireGuardManager(ctx, wgConfig)

		node := &providers.NodeOutput{
			Name:        "worker-1",
			WireGuardIP: "10.8.0.10",
		}

		config := manager.generateNodeConfig(node)

		assert.NotEmpty(t, config)
		assert.Contains(t, config, "[Interface]")
		assert.Contains(t, config, "Address = 10.8.0.10/24")
		assert.Contains(t, config, "ListenPort = 51820")
		assert.Contains(t, config, "MTU = 1420")
		assert.Contains(t, config, "[Peer]")
		assert.Contains(t, config, "SERVER_PUBLIC_KEY")

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}

// TestWireGuardManager_generateServerPeerConfig tests server peer config generation
func TestWireGuardManager_generateServerPeerConfig(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		wgConfig := &config.WireGuardConfig{
			Enabled: true,
		}

		manager := NewWireGuardManager(ctx, wgConfig)

		// Add nodes
		manager.nodes = append(manager.nodes, &providers.NodeOutput{
			Name:        "worker-1",
			WireGuardIP: "10.8.0.10",
		})
		manager.nodes = append(manager.nodes, &providers.NodeOutput{
			Name:        "worker-2",
			WireGuardIP: "10.8.0.11",
		})

		config := manager.generateServerPeerConfig()

		assert.NotEmpty(t, config)
		assert.Contains(t, config, "[Peer]")
		assert.Contains(t, config, "worker-1")
		assert.Contains(t, config, "worker-2")
		assert.Contains(t, config, "10.8.0.10/32")
		assert.Contains(t, config, "10.8.0.11/32")

		return nil
	}, pulumi.WithMocks("test", "stack", &SecurityTestMockProvider{}))

	assert.NoError(t, err)
}
