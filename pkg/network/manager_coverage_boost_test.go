package network

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// TestNewManager_BasicCreation tests basic manager creation
func TestNewManager_BasicCreation(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		networkConfig := &config.NetworkConfig{
			CIDR:       "10.0.0.0/16",
			PodCIDR:    "10.244.0.0/16",
			ServiceCIDR: "10.96.0.0/12",
		}

		manager := NewManager(ctx, networkConfig)

		assert.NotNil(t, manager)
		assert.Equal(t, networkConfig, manager.config)
		assert.NotNil(t, manager.providers)
		assert.NotNil(t, manager.networks)
		assert.Equal(t, ctx, manager.ctx)
		assert.Empty(t, manager.providers)
		assert.Empty(t, manager.networks)

		return nil
	}, pulumi.WithMocks("test", "stack", &TestMockProvider{}))

	assert.NoError(t, err)
}

// TestCreateFirewallConfig_BasicSetup tests firewall config without extras
func TestCreateFirewallConfig_BasicSetup(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := &Manager{
			ctx: ctx,
			config: &config.NetworkConfig{
				CIDR:            "10.0.0.0/16",
				EnableNodePorts: false,
			},
		}

		fwConfig := manager.createFirewallConfig("test-provider")

		assert.NotNil(t, fwConfig)
		assert.Contains(t, fwConfig.Name, "-firewall")
		assert.NotEmpty(t, fwConfig.InboundRules)

		// Should have Kubernetes rules
		hasK8sAPI := false
		hasEtcd := false
		hasKubelet := false
		for _, rule := range fwConfig.InboundRules {
			if rule.Port == "6443" {
				hasK8sAPI = true
			}
			if rule.Port == "2379-2380" {
				hasEtcd = true
			}
			if rule.Port == "10250" {
				hasKubelet = true
			}
		}

		assert.True(t, hasK8sAPI, "Should have Kubernetes API rule")
		assert.True(t, hasEtcd, "Should have etcd rule")
		assert.True(t, hasKubelet, "Should have Kubelet rule")

		return nil
	}, pulumi.WithMocks("test", "stack", &TestMockProvider{}))

	assert.NoError(t, err)
}

// TestCreateFirewallConfig_WireGuardEnabled tests firewall config with WireGuard
func TestCreateFirewallConfig_WireGuardEnabled(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := &Manager{
			ctx: ctx,
			config: &config.NetworkConfig{
				CIDR:            "10.0.0.0/16",
				EnableNodePorts: false,
				WireGuard: &config.WireGuardConfig{
					Enabled: true,
					Port:    51820,
				},
			},
		}

		fwConfig := manager.createFirewallConfig("test-provider")

		assert.NotNil(t, fwConfig)

		// Should have WireGuard rules
		hasWireGuardPort := false
		hasWireGuardNetwork := false
		for _, rule := range fwConfig.InboundRules {
			if rule.Protocol == "udp" && rule.Port == "51820" {
				hasWireGuardPort = true
			}
			if rule.Description == "Allow all from WireGuard network" {
				hasWireGuardNetwork = true
			}
		}

		assert.True(t, hasWireGuardPort, "Should have WireGuard port rule")
		assert.True(t, hasWireGuardNetwork, "Should have WireGuard network rule")

		return nil
	}, pulumi.WithMocks("test", "stack", &TestMockProvider{}))

	assert.NoError(t, err)
}

// TestCreateFirewallConfig_NodePortsEnabled tests firewall config with NodePorts
func TestCreateFirewallConfig_NodePortsEnabled(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := &Manager{
			ctx: ctx,
			config: &config.NetworkConfig{
				CIDR:            "10.0.0.0/16",
				EnableNodePorts: true,
			},
		}

		fwConfig := manager.createFirewallConfig("test-provider")

		assert.NotNil(t, fwConfig)

		// Should have NodePort rule
		hasNodePort := false
		for _, rule := range fwConfig.InboundRules {
			if rule.Port == "30000-32767" {
				hasNodePort = true
				assert.Equal(t, "NodePort Services", rule.Description)
			}
		}

		assert.True(t, hasNodePort, "Should have NodePort Services rule")

		return nil
	}, pulumi.WithMocks("test", "stack", &TestMockProvider{}))

	assert.NoError(t, err)
}

// TestCreateFirewallConfig_CustomFirewallRules tests adding custom rules
func TestCreateFirewallConfig_CustomFirewallRules(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		customInbound := []config.FirewallRule{
			{
				Protocol:    "tcp",
				Port:        "8080",
				Source:      []string{"0.0.0.0/0"},
				Description: "Custom App Port",
			},
		}

		customOutbound := []config.FirewallRule{
			{
				Protocol:    "tcp",
				Port:        "443",
				Source:      []string{"0.0.0.0/0"},
				Description: "Custom Outbound HTTPS",
			},
		}

		manager := &Manager{
			ctx: ctx,
			config: &config.NetworkConfig{
				CIDR:            "10.0.0.0/16",
				EnableNodePorts: false,
				Firewall: &config.FirewallConfig{
					InboundRules:  customInbound,
					OutboundRules: customOutbound,
				},
			},
		}

		fwConfig := manager.createFirewallConfig("test-provider")

		assert.NotNil(t, fwConfig)

		// Should have custom rules
		hasCustomInbound := false
		hasCustomOutbound := false

		for _, rule := range fwConfig.InboundRules {
			if rule.Port == "8080" && rule.Description == "Custom App Port" {
				hasCustomInbound = true
			}
		}

		for _, rule := range fwConfig.OutboundRules {
			if rule.Port == "443" && rule.Description == "Custom Outbound HTTPS" {
				hasCustomOutbound = true
			}
		}

		assert.True(t, hasCustomInbound, "Should have custom inbound rule")
		assert.True(t, hasCustomOutbound, "Should have custom outbound rule")

		return nil
	}, pulumi.WithMocks("test", "stack", &TestMockProvider{}))

	assert.NoError(t, err)
}

// TestCreateFirewallConfig_AllFeatures tests firewall config with all features
func TestCreateFirewallConfig_AllFeatures(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := &Manager{
			ctx: ctx,
			config: &config.NetworkConfig{
				CIDR:            "10.0.0.0/16",
				EnableNodePorts: true,
				WireGuard: &config.WireGuardConfig{
					Enabled: true,
					Port:    51820,
				},
				Firewall: &config.FirewallConfig{
					InboundRules: []config.FirewallRule{
						{
							Protocol:    "tcp",
							Port:        "9090",
							Source:      []string{"10.0.0.0/8"},
							Description: "Monitoring",
						},
					},
				},
			},
		}

		fwConfig := manager.createFirewallConfig("test-provider")

		assert.NotNil(t, fwConfig)

		// Verify all rule types are present
		hasK8s := false
		hasWireGuard := false
		hasNodePort := false
		hasCustom := false

		for _, rule := range fwConfig.InboundRules {
			if rule.Port == "6443" {
				hasK8s = true
			}
			if rule.Port == "51820" {
				hasWireGuard = true
			}
			if rule.Port == "30000-32767" {
				hasNodePort = true
			}
			if rule.Port == "9090" {
				hasCustom = true
			}
		}

		assert.True(t, hasK8s, "Should have Kubernetes rules")
		assert.True(t, hasWireGuard, "Should have WireGuard rules")
		assert.True(t, hasNodePort, "Should have NodePort rules")
		assert.True(t, hasCustom, "Should have custom rules")

		return nil
	}, pulumi.WithMocks("test", "stack", &TestMockProvider{}))

	assert.NoError(t, err)
}

// TestCrossProviderPeering_Basic tests basic cross-provider peering
func TestCrossProviderPeering_Basic(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := &Manager{
			ctx: ctx,
			config: &config.NetworkConfig{
				CrossProviderNetworking: true,
			},
		}

		err := manager.createCrossProviderPeering()
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test", "stack", &TestMockProvider{}))

	assert.NoError(t, err)
}

// TestExportNetworkOutputs_EmptyNetworks tests exporting with no networks
func TestExportNetworkOutputs_EmptyNetworks(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := &Manager{
			ctx: ctx,
			config: &config.NetworkConfig{
				CIDR: "10.0.0.0/16",
			},
			networks: map[string]*providers.NetworkOutput{},
		}

		// Should not panic with empty networks
		manager.ExportNetworkOutputs()

		return nil
	}, pulumi.WithMocks("test", "stack", &TestMockProvider{}))

	assert.NoError(t, err)
}

// TestExportNetworkOutputs_WireGuardDisabled tests exporting without WireGuard
func TestExportNetworkOutputs_WireGuardDisabled(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := &Manager{
			ctx: ctx,
			config: &config.NetworkConfig{
				CIDR: "10.0.0.0/16",
			},
			networks: map[string]*providers.NetworkOutput{},
		}

		manager.ExportNetworkOutputs()

		return nil
	}, pulumi.WithMocks("test", "stack", &TestMockProvider{}))

	assert.NoError(t, err)
}

// TestExportNetworkOutputs_WireGuardEnabled tests exporting with WireGuard
func TestExportNetworkOutputs_WireGuardEnabled(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := &Manager{
			ctx: ctx,
			config: &config.NetworkConfig{
				CIDR: "10.0.0.0/16",
				WireGuard: &config.WireGuardConfig{
					Enabled: true,
					Port:    51820,
				},
			},
			networks: map[string]*providers.NetworkOutput{},
		}

		manager.ExportNetworkOutputs()

		return nil
	}, pulumi.WithMocks("test", "stack", &TestMockProvider{}))

	assert.NoError(t, err)
}
