package network

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// TestNetworkManagerIntegration_CompleteFlow tests complete network setup flow
func TestNetworkManagerIntegration_CompleteFlow(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// Setup network configuration
		networkConfig := &config.NetworkConfig{
			CIDR:       "10.0.0.0/16",
			PodCIDR:    "10.244.0.0/16",
			ServiceCIDR: "10.96.0.0/12",
			WireGuard: &config.WireGuardConfig{
				Enabled: true,
				Port:    51820,
			},
			EnableNodePorts:         true,
			CrossProviderNetworking: true,
			DNSServers:              []string{"1.1.1.1", "8.8.8.8"},
		}

		// Create network manager
		manager := NewManager(ctx, networkConfig)
		assert.NotNil(t, manager)

		// Register mock provider
		mockProvider := &MockNetworkProvider{name: "test-provider"}
		manager.RegisterProvider("test-provider", mockProvider)

		// Validate CIDRs
		err := manager.ValidateCIDRs()
		assert.NoError(t, err, "CIDR validation should pass")

		// Allocate IPs for nodes
		ips, err := manager.AllocateNodeIPs(5)
		assert.NoError(t, err)
		assert.Len(t, ips, 5)

		// Get DNS servers
		dns := manager.GetDNSServers()
		assert.Len(t, dns, 2)
		assert.Equal(t, "1.1.1.1", dns[0])

		// Create firewall config
		fwConfig := manager.createFirewallConfig("test-provider")
		assert.NotNil(t, fwConfig)

		// Verify firewall has all required rules
		hasK8s := false
		hasWireGuard := false
		hasNodePort := false

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
		}

		assert.True(t, hasK8s, "Should have Kubernetes rules")
		assert.True(t, hasWireGuard, "Should have WireGuard rules")
		assert.True(t, hasNodePort, "Should have NodePort rules")

		return nil
	}, pulumi.WithMocks("test", "stack", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestNetworkManagerIntegration_MultiProvider tests multi-provider setup
func TestNetworkManagerIntegration_MultiProvider(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		networkConfig := &config.NetworkConfig{
			CIDR:                    "10.0.0.0/16",
			CrossProviderNetworking: true,
		}

		manager := NewManager(ctx, networkConfig)

		// Register multiple providers
		providers := []string{"digitalocean", "linode", "azure"}
		for _, providerName := range providers {
			mockProvider := &MockNetworkProvider{name: providerName}
			manager.RegisterProvider(providerName, mockProvider)
		}

		// Verify all providers registered
		assert.Len(t, manager.providers, 3)

		// Create cross-provider peering
		err := manager.createCrossProviderPeering()
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test", "stack", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestNetworkManagerIntegration_IPAllocation tests IP allocation across network
func TestNetworkManagerIntegration_IPAllocation(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		networkConfig := &config.NetworkConfig{
			CIDR: "10.0.0.0/24", // Small network for testing
		}

		manager := NewManager(ctx, networkConfig)

		// Allocate IPs for different node groups
		masterIPs, err := manager.AllocateNodeIPs(3)
		assert.NoError(t, err)
		assert.Len(t, masterIPs, 3)

		// Verify IPs are within network range
		for _, ip := range masterIPs {
			assert.Contains(t, ip, "10.0.0.")
		}

		// Verify IPs are sequential (after network, gateway, and reserved IP)
		assert.Equal(t, "10.0.0.3", masterIPs[0]) // First usable IP after skipping first 2
		assert.Equal(t, "10.0.0.4", masterIPs[1])
		assert.Equal(t, "10.0.0.5", masterIPs[2])

		return nil
	}, pulumi.WithMocks("test", "stack", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestNetworkManagerIntegration_FirewallRulesGeneration tests firewall rule generation
func TestNetworkManagerIntegration_FirewallRulesGeneration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		tests := []struct {
			name           string
			config         *config.NetworkConfig
			expectK8s      bool
			expectWG       bool
			expectNodePort bool
			expectCustom   bool
		}{
			{
				name: "Minimal setup",
				config: &config.NetworkConfig{
					CIDR: "10.0.0.0/16",
				},
				expectK8s:      true,
				expectWG:       false,
				expectNodePort: false,
				expectCustom:   false,
			},
			{
				name: "Full setup with WireGuard",
				config: &config.NetworkConfig{
					CIDR: "10.0.0.0/16",
					WireGuard: &config.WireGuardConfig{
						Enabled: true,
						Port:    51820,
					},
					EnableNodePorts: true,
				},
				expectK8s:      true,
				expectWG:       true,
				expectNodePort: true,
				expectCustom:   false,
			},
			{
				name: "With custom rules",
				config: &config.NetworkConfig{
					CIDR: "10.0.0.0/16",
					Firewall: &config.FirewallConfig{
						InboundRules: []config.FirewallRule{
							{
								Protocol:    "tcp",
								Port:        "9090",
								Source:      []string{"0.0.0.0/0"},
								Description: "Custom monitoring",
							},
						},
					},
				},
				expectK8s:      true,
				expectWG:       false,
				expectNodePort: false,
				expectCustom:   true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				manager := NewManager(ctx, tt.config)
				fwConfig := manager.createFirewallConfig("test")

				hasK8s := false
				hasWG := false
				hasNodePort := false
				hasCustom := false

				for _, rule := range fwConfig.InboundRules {
					if rule.Port == "6443" {
						hasK8s = true
					}
					if rule.Description == "WireGuard VPN" {
						hasWG = true
					}
					if rule.Port == "30000-32767" {
						hasNodePort = true
					}
					if rule.Port == "9090" {
						hasCustom = true
					}
				}

				assert.Equal(t, tt.expectK8s, hasK8s, "K8s rules mismatch")
				assert.Equal(t, tt.expectWG, hasWG, "WireGuard rules mismatch")
				assert.Equal(t, tt.expectNodePort, hasNodePort, "NodePort rules mismatch")
				assert.Equal(t, tt.expectCustom, hasCustom, "Custom rules mismatch")
			})
		}

		return nil
	}, pulumi.WithMocks("test", "stack", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestVPNConnectivityIntegration_NodeManagement tests VPN connectivity with node management
func TestVPNConnectivityIntegration_NodeManagement(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewVPNConnectivityChecker(ctx)

		// Setup SSH
		checker.SetSSHKeyPath("/home/user/.ssh/id_rsa")

		// Add multiple nodes with different roles
		nodes := []*providers.NodeOutput{
			{Name: "master-1", WireGuardIP: "10.8.0.1"},
			{Name: "master-2", WireGuardIP: "10.8.0.2"},
			{Name: "master-3", WireGuardIP: "10.8.0.3"},
			{Name: "worker-1", WireGuardIP: "10.8.0.10"},
			{Name: "worker-2", WireGuardIP: "10.8.0.11"},
			{Name: "worker-3", WireGuardIP: "10.8.0.12"},
		}

		for _, node := range nodes {
			checker.AddNode(node)
		}

		// Verify all nodes added
		assert.Len(t, checker.nodes, 6)

		// Get connectivity matrix
		matrix := checker.GetConnectivityMatrix()
		assert.Len(t, matrix, 6)

		// Verify each node has an entry
		for _, node := range nodes {
			assert.Contains(t, matrix, node.Name)
		}

		return nil
	}, pulumi.WithMocks("test", "stack", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestCIDRValidationIntegration tests CIDR validation across different scenarios
func TestCIDRValidationIntegration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		tests := []struct {
			name        string
			config      *config.NetworkConfig
			expectError bool
			errorMsg    string
		}{
			{
				name: "Valid non-overlapping CIDRs",
				config: &config.NetworkConfig{
					CIDR:        "10.0.0.0/16",
					PodCIDR:     "10.244.0.0/16",
					ServiceCIDR: "10.96.0.0/12",
				},
				expectError: false,
			},
			{
				name: "Overlapping network and pod CIDR",
				config: &config.NetworkConfig{
					CIDR:        "10.0.0.0/16",
					PodCIDR:     "10.0.1.0/24", // Overlaps with network
					ServiceCIDR: "10.96.0.0/12",
				},
				expectError: true,
				errorMsg:    "overlap",
			},
			{
				name: "Overlapping pod and service CIDR",
				config: &config.NetworkConfig{
					CIDR:        "10.0.0.0/16",
					PodCIDR:     "10.96.0.0/16", // Overlaps with service
					ServiceCIDR: "10.96.0.0/12",
				},
				expectError: true,
				errorMsg:    "overlap",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				manager := NewManager(ctx, tt.config)
				err := manager.ValidateCIDRs()

				if tt.expectError {
					assert.Error(t, err)
					if tt.errorMsg != "" {
						assert.Contains(t, err.Error(), tt.errorMsg)
					}
				} else {
					assert.NoError(t, err)
				}
			})
		}

		return nil
	}, pulumi.WithMocks("test", "stack", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// MockNetworkProvider implements providers.Provider for testing
type MockNetworkProvider struct {
	name string
}

func (m *MockNetworkProvider) GetName() string {
	return m.name
}

func (m *MockNetworkProvider) Initialize(ctx *pulumi.Context, cfg *config.ClusterConfig) error {
	return nil
}

func (m *MockNetworkProvider) CreateNode(ctx *pulumi.Context, cfg *config.NodeConfig) (*providers.NodeOutput, error) {
	return nil, nil
}

func (m *MockNetworkProvider) CreateNodePool(ctx *pulumi.Context, pool *config.NodePool) ([]*providers.NodeOutput, error) {
	return nil, nil
}

func (m *MockNetworkProvider) CreateNetwork(ctx *pulumi.Context, cfg *config.NetworkConfig) (*providers.NetworkOutput, error) {
	return &providers.NetworkOutput{
		Name:   m.name + "-network",
		CIDR:   cfg.CIDR,
		Region: "test-region",
	}, nil
}

func (m *MockNetworkProvider) CreateFirewall(ctx *pulumi.Context, cfg *config.FirewallConfig, nodeIDs []pulumi.IDOutput) error {
	return nil
}

func (m *MockNetworkProvider) CreateLoadBalancer(ctx *pulumi.Context, lb *config.LoadBalancerConfig) (*providers.LoadBalancerOutput, error) {
	return nil, nil
}

func (m *MockNetworkProvider) GetRegions() []string {
	return []string{"test-region-1", "test-region-2"}
}

func (m *MockNetworkProvider) GetSizes() []string {
	return []string{"small", "medium", "large"}
}

func (m *MockNetworkProvider) Cleanup(ctx *pulumi.Context) error {
	return nil
}

// IntegrationMockProvider implements pulumi mock provider
type IntegrationMockProvider struct {
	pulumi.ResourceState
}

func (m *IntegrationMockProvider) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	return args.Name + "_id", args.Inputs, nil
}

func (m *IntegrationMockProvider) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}
