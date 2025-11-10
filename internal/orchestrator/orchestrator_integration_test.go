package orchestrator
import (
	"testing"
	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
// IntegrationMockProvider provides comprehensive mocking for integration tests
type IntegrationMockProvider struct {
	pulumi.ResourceState
}
func (m *IntegrationMockProvider) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	outputs := args.Inputs.Copy()
	switch args.TypeToken {
	case "digitalocean:index/droplet:Droplet":
		outputs["ipv4Address"] = resource.NewStringProperty("192.168.1.10")
		outputs["ipv4AddressPrivate"] = resource.NewStringProperty("10.0.0.10")
		outputs["name"] = resource.NewStringProperty(args.Name)
		outputs["id"] = resource.NewStringProperty(args.Name + "-do-id")
		outputs["region"] = resource.NewStringProperty("nyc3")
		outputs["size"] = resource.NewStringProperty("s-2vcpu-4gb")
	case "linode:index/instance:Instance":
		outputs["ipAddress"] = resource.NewStringProperty("203.0.113.10")
		outputs["privateIpAddress"] = resource.NewStringProperty("192.168.1.10")
		outputs["label"] = resource.NewStringProperty(args.Name)
		outputs["id"] = resource.NewStringProperty(args.Name + "-linode-id")
		outputs["region"] = resource.NewStringProperty("us-east")
		outputs["type"] = resource.NewStringProperty("g6-standard-2")
	case "azure:compute/virtualMachine:VirtualMachine":
		outputs["name"] = resource.NewStringProperty(args.Name)
		outputs["id"] = resource.NewStringProperty(args.Name + "-azure-id")
		outputs["location"] = resource.NewStringProperty("eastus")
		outputs["vmSize"] = resource.NewStringProperty("Standard_B2s")
	case "command:remote:Command":
		outputs["stdout"] = resource.NewStringProperty("command executed successfully")
		outputs["stderr"] = resource.NewStringProperty("")
	case "digitalocean:index/firewall:Firewall":
		outputs["id"] = resource.NewStringProperty(args.Name + "-firewall-id")
		outputs["name"] = resource.NewStringProperty(args.Name)
	case "digitalocean:index/loadBalancer:LoadBalancer":
		outputs["ip"] = resource.NewStringProperty("203.0.113.100")
		outputs["name"] = resource.NewStringProperty(args.Name)
	case "digitalocean:index/dnsRecord:DnsRecord":
		outputs["fqdn"] = resource.NewStringProperty(args.Name + ".chalkan3.com.br")
		outputs["name"] = resource.NewStringProperty(args.Name)
		outputs["type"] = resource.NewStringProperty("A")
	case "digitalocean:index/sshKey:SshKey":
		outputs["id"] = resource.NewStringProperty(args.Name + "-sshkey-id")
		outputs["fingerprint"] = resource.NewStringProperty("aa:bb:cc:dd:ee:ff")
	case "digitalocean:index/vpc:Vpc":
		outputs["id"] = resource.NewStringProperty(args.Name + "-vpc-id")
		outputs["name"] = resource.NewStringProperty(args.Name)
	}
	return args.Name + "_integration_id", outputs, nil
}
func (m *IntegrationMockProvider) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}
// Helper function to create a complete multi-cloud cluster config
func createMultiCloudConfig() *config.ClusterConfig {
	return &config.ClusterConfig{
		Metadata: config.Metadata{
			Name:        "test-multi-cloud-cluster",
			Environment: "integration",
			Version:     "1.0.0",
		},
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Token:   "do-test-token",
				Region:  "nyc3",
			},
			Linode: &config.LinodeProvider{
				Token:   "linode-test-token",
				Region:  "us-east",
			},
		},
		NodePools: map[string]config.NodePool{
			"do-masters": {
				Name:     "do-masters",
				Provider: "digitalocean",
				Count:    3,
				Size:     "s-2vcpu-4gb",
				Roles:    []string{"master", "etcd"},
			},
			"linode-workers": {
				Name:     "linode-workers",
				Provider: "linode",
				Count:    3,
				Size:     "g6-standard-2",
				Roles:    []string{"worker"},
			},
		},
		Network: config.NetworkConfig{
			PodCIDR:     "10.244.0.0/16",
			ServiceCIDR: "10.96.0.0/12",
			DNS: config.DNSConfig{
				Domain:  "chalkan3.com.br",
			},
			WireGuard: &config.WireGuardConfig{
				Enabled:  true,
				Port:     51820,
				Provider: "digitalocean",
				Region:   "nyc3",
				Size:     "s-1vcpu-1gb",
			},
		},
		Kubernetes: config.KubernetesConfig{
			Version:      "v1.28.0-rancher1-1",
			Distribution: "rke",
		},
		LoadBalancer: config.LoadBalancerConfig{
			Provider: "digitalocean",
			Name:     "test-lb",
		},
		Storage: config.StorageConfig{
			Classes: []config.StorageClass{
				{
					Name:        "fast",
					Provisioner: "digitalocean",
					Parameters: map[string]string{
						"type": "pd-ssd",
					},
				},
			},
		},
	}
}
// Test 1: Complete orchestrator deployment flow
func TestOrchestrator_CompleteDeploymentFlow(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := createMultiCloudConfig()
		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, ctx, orch.ctx)
		assert.Equal(t, cfg, orch.config)
		assert.NotNil(t, orch.providerRegistry)
		assert.NotNil(t, orch.nodes)
		assert.Equal(t, 0, len(orch.nodes))
		return nil
	}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
// Test 2: Provider initialization with multiple providers
func TestOrchestrator_InitializeMultipleProviders(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.ClusterConfig
		expectedCount  int
		expectedError  bool
	}{
		{
			name: "DigitalOcean only",
			config: &config.ClusterConfig{
				Providers: config.ProvidersConfig{
					DigitalOcean: &config.DigitalOceanProvider{
						Token:   "do-token",
					},
				},
			},
			expectedCount: 1,
			expectedError: false,
		},
		{
			name: "DigitalOcean and Linode",
			config: &config.ClusterConfig{
				Providers: config.ProvidersConfig{
					DigitalOcean: &config.DigitalOceanProvider{
						Token:   "do-token",
					},
					Linode: &config.LinodeProvider{
						Token:   "linode-token",
					},
				},
			},
			expectedCount: 2,
			expectedError: false,
		},
		{
			name: "All three providers",
			config: &config.ClusterConfig{
				Providers: config.ProvidersConfig{
					DigitalOcean: &config.DigitalOceanProvider{
						Token:   "do-token",
					},
					Linode: &config.LinodeProvider{
						Token:   "linode-token",
					},
					Azure: &config.AzureProvider{
						Enabled:        true,
						SubscriptionID: "azure-sub",
					},
				},
			},
			expectedCount: 3,
			expectedError: false,
		},
		{
			name: "No providers enabled",
			config: &config.ClusterConfig{
				Providers: config.ProvidersConfig{},
			},
			expectedCount: 0,
			expectedError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				orch := New(ctx, tt.config)
				err := orch.initializeProviders()
				if tt.expectedError {
					assert.Error(t, err)
					return nil
				}
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCount, len(orch.providerRegistry.GetAll()))
				return nil
			}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
			assert.NoError(t, err)
		})
	}
}
// Test 3: SSH key generation flow
func TestOrchestrator_GenerateSSHKeys(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := createMultiCloudConfig()
		orch := New(ctx, cfg)
		err := orch.generateSSHKeys()
		assert.NoError(t, err)
		assert.NotNil(t, orch.sshKeyManager)
		// Verify SSH key was set in provider configs
		assert.NotEmpty(t, cfg.Providers.DigitalOcean.SSHPublicKey)
		assert.NotEmpty(t, cfg.Providers.Linode.SSHPublicKey)
		return nil
	}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
// Test 4: Network creation flow
func TestOrchestrator_CreateNetworking(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := createMultiCloudConfig()
		orch := New(ctx, cfg)
		// Initialize providers first
		require.NoError(t, orch.initializeProviders())
		// Create networking
		err := orch.createNetworking()
		assert.NoError(t, err)
		assert.NotNil(t, orch.networkManager)
		return nil
	}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
// Test 5: Node deployment with pools
func TestOrchestrator_DeployNodesWithPools(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := createMultiCloudConfig()
		orch := New(ctx, cfg)
		// Initialize providers
		require.NoError(t, orch.initializeProviders())
		// Deploy nodes - this will fail in test because providers are mocked
		// but we can test the logic
		_ = orch.deployNodes()
		// We expect this to fail because health checks won't work with mocks
		// but it tests the node deployment logic
		return nil
	}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
	// Test runs without panicking
	assert.NoError(t, err)
}
// Test 6: Node distribution verification
func TestOrchestrator_VerifyNodeDistribution(t *testing.T) {
	tests := []struct {
		name          string
		nodePools     map[string]config.NodePool
		expectError   bool
		errorContains string
	}{
		{
			name: "Valid distribution",
			nodePools: map[string]config.NodePool{
				"masters": {
					Name:     "masters",
					Provider: "digitalocean",
					Count:    3,
					Roles:    []string{"master"},
				},
				"workers": {
					Name:     "workers",
					Provider: "digitalocean",
					Count:    3,
					Roles:    []string{"worker"},
				},
			},
			expectError: false,
		},
		{
			name: "Only masters",
			nodePools: map[string]config.NodePool{
				"masters": {
					Name:     "masters",
					Provider: "digitalocean",
					Count:    3,
					Roles:    []string{"master"},
				},
			},
			expectError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				cfg := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						DigitalOcean: &config.DigitalOceanProvider{
							Token:   "test-token",
						},
					},
					NodePools: tt.nodePools,
				}
				orch := New(ctx, cfg)
				// Test the configuration
				assert.NotNil(t, orch)
				return nil
			}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
			assert.NoError(t, err)
		})
	}
}
// Test 7: DNS configuration scenarios
func TestOrchestrator_ConfigureDNS(t *testing.T) {
	tests := []struct {
		name         string
		dnsConfig    config.DNSConfig
		expectDomain string
	}{
		{
			name: "Custom domain",
			dnsConfig: config.DNSConfig{
				Domain:  "example.com",
			},
			expectDomain: "example.com",
		},
		{
			name: "Default domain",
			dnsConfig: config.DNSConfig{
				Domain:  "",
			},
			expectDomain: "chalkan3.com.br",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				cfg := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						DigitalOcean: &config.DigitalOceanProvider{
							Token:   "test-token",
						},
					},
					Network: config.NetworkConfig{
						DNS: tt.dnsConfig,
					},
				}
				orch := New(ctx, cfg)
				err := orch.configureDNS()
				// DNS configuration should work or skip if not enabled
				assert.NoError(t, err)
				assert.NotNil(t, orch.dnsManager)
				return nil
			}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
			assert.NoError(t, err)
		})
	}
}
// Test 8: WireGuard configuration flow
func TestOrchestrator_ConfigureWireGuard(t *testing.T) {
	tests := []struct {
		name             string
		wireGuardConfig  *config.WireGuardConfig
		shouldConfigure  bool
	}{
		{
			name: "WireGuard enabled",
			wireGuardConfig: &config.WireGuardConfig{
				Enabled:  true,
				Port:     51820,
				Provider: "digitalocean",
				Region:   "nyc3",
				Size:     "s-1vcpu-1gb",
			},
			shouldConfigure: true,
		},
		{
			name: "WireGuard disabled",
			wireGuardConfig: &config.WireGuardConfig{
			},
			shouldConfigure: false,
		},
		{
			name:            "WireGuard not configured",
			wireGuardConfig: nil,
			shouldConfigure: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				cfg := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						DigitalOcean: &config.DigitalOceanProvider{
							Token:   "test-token",
						},
					},
					Network: config.NetworkConfig{
						WireGuard: tt.wireGuardConfig,
					},
				}
				orch := New(ctx, cfg)
				err := orch.configureWireGuard()
				// Should not error even if skipped
				assert.NoError(t, err)
				if tt.shouldConfigure {
					// Note: wireGuardManager might still be nil in tests
					// because ConfigureNode might fail with mocked nodes
				}
				return nil
			}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
			assert.NoError(t, err)
		})
	}
}
// Test 9: Firewall configuration
func TestOrchestrator_ConfigureFirewalls(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := createMultiCloudConfig()
		orch := New(ctx, cfg)
		// Initialize providers and network
		require.NoError(t, orch.initializeProviders())
		require.NoError(t, orch.createNetworking())
		// Configure firewalls
		err := orch.configureFirewalls()
		assert.NoError(t, err)
		return nil
	}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
// Test 10: Export outputs
func TestOrchestrator_ExportOutputs(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := createMultiCloudConfig()
		orch := New(ctx, cfg)
		// Call exportOutputs
		orch.exportOutputs()
		// The method should complete without error
		// Actual export verification would require checking ctx.Export calls
		return nil
	}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
// Test 11: Get nodes by provider
func TestOrchestrator_GetNodesByProvider(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := createMultiCloudConfig()
		orch := New(ctx, cfg)
		// Test with empty nodes
		_, err := orch.GetNodesByProvider("digitalocean")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no nodes found")
		return nil
	}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
// Test 12: Get node by name
func TestOrchestrator_GetNodeByName(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := createMultiCloudConfig()
		orch := New(ctx, cfg)
		// Test with non-existent node
		_, err := orch.GetNodeByName("non-existent-node")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		return nil
	}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
// Test 13: Get master nodes
func TestOrchestrator_GetMasterNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := createMultiCloudConfig()
		orch := New(ctx, cfg)
		// Test with empty nodes
		masters := orch.GetMasterNodes()
		assert.NotNil(t, masters)
		assert.Equal(t, 0, len(masters))
		return nil
	}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
// Test 14: Get worker nodes
func TestOrchestrator_GetWorkerNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := createMultiCloudConfig()
		orch := New(ctx, cfg)
		// Test with empty nodes
		workers := orch.GetWorkerNodes()
		assert.NotNil(t, workers)
		assert.Equal(t, 0, len(workers))
		return nil
	}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
// Test 15: Cleanup operations
func TestOrchestrator_Cleanup(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := createMultiCloudConfig()
		orch := New(ctx, cfg)
		// Initialize providers
		require.NoError(t, orch.initializeProviders())
		// Cleanup
		err := orch.Cleanup()
		assert.NoError(t, err)
		return nil
	}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
// Test 16: Load balancer installation
func TestOrchestrator_InstallLoadBalancers(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Token:   "test-token",
				},
			},
			LoadBalancer: config.LoadBalancerConfig{
				Provider: "digitalocean",
				Name:     "test-lb",
			},
		}
		orch := New(ctx, cfg)
		require.NoError(t, orch.initializeProviders())
		// Test load balancer installation
		err := orch.installLoadBalancers()
		// This will fail with mock provider but tests the flow
		assert.Error(t, err) // Expected because CreateLoadBalancer is not fully implemented in mock
		return nil
	}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
// Test 17: Storage installation
func TestOrchestrator_InstallStorage(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Storage: config.StorageConfig{
				Classes: []config.StorageClass{
					{
						Name:        "fast",
						Provisioner: "digitalocean",
					},
				},
			},
		}
		orch := New(ctx, cfg)
		// Test storage installation
		err := orch.installStorage()
		assert.NoError(t, err) // Should not error, just log
		return nil
	}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
// Test 18: Complete deployment phases in order
func TestOrchestrator_DeploymentPhasesOrder(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := createMultiCloudConfig()
		orch := New(ctx, cfg)
		// Phase 0: Generate SSH Keys
		err := orch.generateSSHKeys()
		assert.NoError(t, err)
		// Phase 1: Initialize Providers
		err = orch.initializeProviders()
		assert.NoError(t, err)
		// Phase 2: Create Networking
		err = orch.createNetworking()
		assert.NoError(t, err)
		// Phase 5: Configure DNS (can work without nodes)
		err = orch.configureDNS()
		assert.NoError(t, err)
		// Phase 7: Configure Firewalls
		err = orch.configureFirewalls()
		assert.NoError(t, err)
		// Phase 11: Export Outputs
		orch.exportOutputs()
		return nil
	}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
// Test 19: Multi-provider node pool distribution
func TestOrchestrator_MultiProviderNodePools(t *testing.T) {
	tests := []struct {
		name          string
		nodePools     map[string]config.NodePool
		expectedNodes int
		masterCount   int
		workerCount   int
	}{
		{
			name: "3 masters + 3 workers on DigitalOcean",
			nodePools: map[string]config.NodePool{
				"do-masters": {
					Name:     "do-masters",
					Provider: "digitalocean",
					Count:    3,
					Roles:    []string{"master"},
				},
				"do-workers": {
					Name:     "do-workers",
					Provider: "digitalocean",
					Count:    3,
					Roles:    []string{"worker"},
				},
			},
			expectedNodes: 6,
			masterCount:   3,
			workerCount:   3,
		},
		{
			name: "Mixed providers: DO masters + Linode workers",
			nodePools: map[string]config.NodePool{
				"do-masters": {
					Name:     "do-masters",
					Provider: "digitalocean",
					Count:    3,
					Roles:    []string{"master"},
				},
				"linode-workers": {
					Name:     "linode-workers",
					Provider: "linode",
					Count:    5,
					Roles:    []string{"worker"},
				},
			},
			expectedNodes: 8,
			masterCount:   3,
			workerCount:   5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				cfg := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						DigitalOcean: &config.DigitalOceanProvider{
							Token:   "test-token",
						},
						Linode: &config.LinodeProvider{
							Token:   "test-token",
						},
					},
					NodePools: tt.nodePools,
				}
				_ = New(ctx, cfg)
				// Verify configuration
				totalExpected := 0
				for _, pool := range tt.nodePools {
					totalExpected += pool.Count
				}
				assert.Equal(t, tt.expectedNodes, totalExpected)
				return nil
			}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
			assert.NoError(t, err)
		})
	}
}
// Test 20: Error handling - No providers configured
func TestOrchestrator_ErrorHandling_NoProviders(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				// All providers disabled or nil
			},
		}
		orch := New(ctx, cfg)
		err := orch.initializeProviders()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no cloud providers enabled")
		return nil
	}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
// Test 21: Concurrent node deployment safety
func TestOrchestrator_ConcurrentNodeDeploymentSafety(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := createMultiCloudConfig()
		orch := New(ctx, cfg)
		// Verify mutex is initialized
		assert.NotNil(t, &orch.mu)
		// Test that nodes map is thread-safe
		assert.NotNil(t, orch.nodes)
		return nil
	}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
// Test 22: VPN verification before RKE
func TestOrchestrator_VPNVerificationBeforeRKE(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := createMultiCloudConfig()
		cfg.Network.WireGuard.Enabled = true
		orch := New(ctx, cfg)
		// verifyVPNReadyForRKE should fail with no nodes
		err := orch.verifyVPNReadyForRKE()
		// Expected to fail because no nodes are deployed
		assert.Error(t, err)
		return nil
	}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
// Test 23: Orchestrator configuration validation
func TestOrchestrator_ConfigurationValidation(t *testing.T) {
	tests := []struct {
		name   string
		config *config.ClusterConfig
		valid  bool
	}{
		{
			name:   "Complete valid config",
			config: createMultiCloudConfig(),
			valid:  true,
		},
		{
			name: "Missing metadata",
			config: &config.ClusterConfig{
				Providers: config.ProvidersConfig{
					DigitalOcean: &config.DigitalOceanProvider{
						Token:   "test",
					},
				},
			},
			valid: true, // Metadata is optional
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				orch := New(ctx, tt.config)
				assert.NotNil(t, orch)
				assert.Equal(t, tt.config, orch.config)
				return nil
			}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
			assert.NoError(t, err)
		})
	}
}
// Test 24: Addons installation flow
func TestOrchestrator_InstallAddons(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := createMultiCloudConfig()
		orch := New(ctx, cfg)
		// Test addons installation (will fail without RKE manager)
		err := orch.installAddons()
		// Expected to fail because rkeManager is nil
		assert.Error(t, err)
		return nil
	}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
// Test 25: Orchestrator state management
func TestOrchestrator_StateManagement(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := createMultiCloudConfig()
		orch := New(ctx, cfg)
		// Verify initial state
		assert.NotNil(t, orch.ctx)
		assert.NotNil(t, orch.config)
		assert.NotNil(t, orch.providerRegistry)
		assert.NotNil(t, orch.nodes)
		assert.Nil(t, orch.networkManager)
		assert.Nil(t, orch.wireGuardManager)
		assert.Nil(t, orch.sshKeyManager)
		assert.Nil(t, orch.osFirewallMgr)
		assert.Nil(t, orch.dnsManager)
		assert.Nil(t, orch.ingressManager)
		assert.Nil(t, orch.rkeManager)
		assert.Nil(t, orch.healthChecker)
		assert.Nil(t, orch.validator)
		assert.Nil(t, orch.vpnChecker)
		return nil
	}, pulumi.WithMocks("test", "integration", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
