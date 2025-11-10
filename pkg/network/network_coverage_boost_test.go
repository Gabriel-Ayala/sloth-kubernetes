package network

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// NetworkTestMockProvider implements pulumi mock provider for network tests
type NetworkTestMockProvider struct {
	pulumi.ResourceState
}

func (m *NetworkTestMockProvider) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	outputs := args.Inputs.Copy()

	// Mock VPC/Network resources
	if args.TypeToken == "digitalocean:index/vpc:Vpc" ||
		args.TypeToken == "linode:index/vpc:Vpc" ||
		args.TypeToken == "aws:ec2/vpc:Vpc" {
		outputs["id"] = resource.NewStringProperty("vpc-12345")
		outputs["urn"] = resource.NewStringProperty("urn:pulumi:stack::project::vpc")
	}

	// Mock Subnet resources
	if args.TypeToken == "digitalocean:index/subnet:Subnet" ||
		args.TypeToken == "aws:ec2/subnet:Subnet" {
		outputs["id"] = resource.NewStringProperty("subnet-12345")
	}

	// Mock Firewall resources
	if args.TypeToken == "digitalocean:index/firewall:Firewall" ||
		args.TypeToken == "linode:index/firewall:Firewall" ||
		args.TypeToken == "aws:ec2/securityGroup:SecurityGroup" {
		outputs["id"] = resource.NewStringProperty("firewall-12345")
	}

	// Mock remote command
	if args.TypeToken == "command:remote:Command" {
		outputs["stdout"] = resource.NewStringProperty("PING_STATUS:SUCCESS\nPACKET_LOSS:0\nAVG_LATENCY:1.5\nHANDSHAKE:ACTIVE\nWIREGUARD:READY\nPEER_COUNT:3")
	}

	return args.Name + "_id", outputs, nil
}

func (m *NetworkTestMockProvider) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

// MockProvider implements providers.Provider interface for testing
type MockProvider struct {
	name string
}

func (p *MockProvider) GetName() string {
	return p.name
}

func (p *MockProvider) Initialize(ctx *pulumi.Context, cfg *config.ClusterConfig) error {
	return nil
}

func (p *MockProvider) CreateNode(ctx *pulumi.Context, nodeConfig *config.NodeConfig) (*providers.NodeOutput, error) {
	return &providers.NodeOutput{
		ID:          pulumi.ID("node-123").ToIDOutput(),
		Name:        nodeConfig.Name,
		PublicIP:    pulumi.String("1.2.3.4").ToStringOutput(),
		PrivateIP:   pulumi.String("10.0.0.1").ToStringOutput(),
		WireGuardIP: "10.8.0.1",
		SSHUser:     "root",
	}, nil
}

func (p *MockProvider) CreateNetwork(ctx *pulumi.Context, networkConfig *config.NetworkConfig) (*providers.NetworkOutput, error) {
	return &providers.NetworkOutput{
		ID:     pulumi.ID("network-123").ToIDOutput(),
		CIDR:   "10.0.0.0/16",
		Region: "nyc1",
		Subnets: []providers.SubnetOutput{
			{
				ID:   pulumi.ID("subnet-123").ToIDOutput(),
				CIDR: "10.0.1.0/24",
			},
		},
	}, nil
}

func (p *MockProvider) CreateFirewall(ctx *pulumi.Context, firewallConfig *config.FirewallConfig, nodeIds []pulumi.IDOutput) error {
	return nil
}

func (p *MockProvider) DeleteNode(ctx *pulumi.Context, nodeID string) error {
	return nil
}

func (p *MockProvider) GetNodeInfo(ctx *pulumi.Context, nodeID string) (*providers.NodeOutput, error) {
	return nil, nil
}

func (p *MockProvider) GetRegions() []string {
	return []string{"nyc1", "sfo1"}
}

func (p *MockProvider) GetSizes() []string {
	return []string{"s-1vcpu-1gb", "s-2vcpu-2gb"}
}

func (p *MockProvider) Cleanup(ctx *pulumi.Context) error {
	return nil
}

func (p *MockProvider) CreateLoadBalancer(ctx *pulumi.Context, lb *config.LoadBalancerConfig) (*providers.LoadBalancerOutput, error) {
	return nil, nil
}

func (p *MockProvider) CreateNodePool(ctx *pulumi.Context, pool *config.NodePool) ([]*providers.NodeOutput, error) {
	return nil, nil
}

// TestManager_CreateNetworks tests network creation
func TestManager_CreateNetworks(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		networkConfig := &config.NetworkConfig{
			CIDR:                    "10.0.0.0/16",
			CrossProviderNetworking: false,
		}

		manager := NewManager(ctx, networkConfig)
		manager.RegisterProvider("digitalocean", &MockProvider{name: "digitalocean"})

		err := manager.CreateNetworks()
		assert.NoError(t, err)

		// Verify network was created
		network, err := manager.GetNetworkByProvider("digitalocean")
		assert.NoError(t, err)
		assert.NotNil(t, network)
		assert.Equal(t, "10.0.0.0/16", network.CIDR)

		return nil
	}, pulumi.WithMocks("test", "stack", &NetworkTestMockProvider{}))

	assert.NoError(t, err)
}

// TestManager_CreateNetworks_CrossProvider tests cross-provider networking
func TestManager_CreateNetworks_CrossProvider(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		networkConfig := &config.NetworkConfig{
			CIDR:                    "10.0.0.0/16",
			CrossProviderNetworking: true,
		}

		manager := NewManager(ctx, networkConfig)
		manager.RegisterProvider("digitalocean", &MockProvider{name: "digitalocean"})
		manager.RegisterProvider("linode", &MockProvider{name: "linode"})

		err := manager.CreateNetworks()
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test", "stack", &NetworkTestMockProvider{}))

	assert.NoError(t, err)
}

// TestManager_CreateFirewalls tests firewall creation
func TestManager_CreateFirewalls(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		networkConfig := &config.NetworkConfig{
			CIDR: "10.0.0.0/16",
			WireGuard: &config.WireGuardConfig{
				Enabled: true,
				Port:    51820,
			},
		}

		manager := NewManager(ctx, networkConfig)
		manager.RegisterProvider("digitalocean", &MockProvider{name: "digitalocean"})

		nodes := map[string][]*providers.NodeOutput{
			"digitalocean": {
				{
					ID:          pulumi.ID("node-1").ToIDOutput(),
					Name:        "master-1",
					PublicIP:    pulumi.String("1.2.3.4").ToStringOutput(),
					PrivateIP:   pulumi.String("10.0.0.1").ToStringOutput(),
					WireGuardIP: "10.8.0.1",
				},
			},
		}

		err := manager.CreateFirewalls(nodes)
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test", "stack", &NetworkTestMockProvider{}))

	assert.NoError(t, err)
}

// TestManager_CreateFirewalls_UnregisteredProvider tests error handling
func TestManager_CreateFirewalls_UnregisteredProvider(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		networkConfig := &config.NetworkConfig{
			CIDR: "10.0.0.0/16",
		}

		manager := NewManager(ctx, networkConfig)

		nodes := map[string][]*providers.NodeOutput{
			"nonexistent": {
				{ID: pulumi.ID("node-1").ToIDOutput()},
			},
		}

		err := manager.CreateFirewalls(nodes)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not registered")

		return nil
	}, pulumi.WithMocks("test", "stack", &NetworkTestMockProvider{}))

	assert.NoError(t, err)
}

// TestManager_CreateFirewalls_WithNodePorts tests firewall with NodePorts enabled
func TestManager_CreateFirewalls_WithNodePorts(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		networkConfig := &config.NetworkConfig{
			CIDR:            "10.0.0.0/16",
			EnableNodePorts: true,
		}

		manager := NewManager(ctx, networkConfig)
		manager.RegisterProvider("digitalocean", &MockProvider{name: "digitalocean"})

		nodes := map[string][]*providers.NodeOutput{
			"digitalocean": {
				{ID: pulumi.ID("node-1").ToIDOutput()},
			},
		}

		err := manager.CreateFirewalls(nodes)
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test", "stack", &NetworkTestMockProvider{}))

	assert.NoError(t, err)
}

// TestManager_ExportNetworkOutputs tests network output exports
func TestManager_ExportNetworkOutputs(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		networkConfig := &config.NetworkConfig{
			CIDR: "10.0.0.0/16",
			WireGuard: &config.WireGuardConfig{
				Enabled: true,
				Port:    51820,
			},
		}

		manager := NewManager(ctx, networkConfig)
		manager.RegisterProvider("digitalocean", &MockProvider{name: "digitalocean"})

		// Create networks first
		err := manager.CreateNetworks()
		assert.NoError(t, err)

		// Export outputs
		manager.ExportNetworkOutputs()

		return nil
	}, pulumi.WithMocks("test", "stack", &NetworkTestMockProvider{}))

	assert.NoError(t, err)
}

// TestManager_ExportNetworkOutputs_NoWireGuard tests exports without WireGuard
func TestManager_ExportNetworkOutputs_NoWireGuard(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		networkConfig := &config.NetworkConfig{
			CIDR: "10.0.0.0/16",
		}

		manager := NewManager(ctx, networkConfig)
		manager.RegisterProvider("linode", &MockProvider{name: "linode"})

		err := manager.CreateNetworks()
		assert.NoError(t, err)

		manager.ExportNetworkOutputs()

		return nil
	}, pulumi.WithMocks("test", "stack", &NetworkTestMockProvider{}))

	assert.NoError(t, err)
}

// TestVPNConnectivityChecker_countConnections tests counting connections
func TestVPNConnectivityChecker_countConnections(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewVPNConnectivityChecker(ctx)

		result := &ConnectivityResult{
			SourceNode: "master-1",
			Connections: map[string]*ConnectionStatus{
				"worker-1": {IsConnected: true},
				"worker-2": {IsConnected: true},
				"worker-3": {IsConnected: false},
			},
		}

		count := checker.countConnections(result)
		assert.Equal(t, 2, count)

		return nil
	}, pulumi.WithMocks("test", "stack", &NetworkTestMockProvider{}))

	assert.NoError(t, err)
}

// TestVPNConnectivityChecker_GetConnectivityMatrix tests connectivity matrix
func TestVPNConnectivityChecker_GetConnectivityMatrix(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewVPNConnectivityChecker(ctx)

		node1 := &providers.NodeOutput{Name: "master-1", WireGuardIP: "10.8.0.1"}
		node2 := &providers.NodeOutput{Name: "worker-1", WireGuardIP: "10.8.0.2"}

		checker.AddNode(node1)
		checker.AddNode(node2)

		// Manually set up some connectivity results
		checker.results["master-1"].Connections["worker-1"] = &ConnectionStatus{
			IsConnected: true,
			TargetNode:  "worker-1",
		}

		matrix := checker.GetConnectivityMatrix()
		assert.NotNil(t, matrix)
		assert.Contains(t, matrix, "master-1")
		assert.Contains(t, matrix["master-1"], "worker-1")
		assert.True(t, matrix["master-1"]["worker-1"])

		return nil
	}, pulumi.WithMocks("test", "stack", &NetworkTestMockProvider{}))

	assert.NoError(t, err)
}

// TestVPNConnectivityChecker_AddNode tests adding nodes
func TestVPNConnectivityChecker_AddNode(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewVPNConnectivityChecker(ctx)

		node := &providers.NodeOutput{
			Name:        "master-1",
			WireGuardIP: "10.8.0.1",
		}

		checker.AddNode(node)
		assert.Len(t, checker.nodes, 1)
		assert.Contains(t, checker.results, "master-1")

		return nil
	}, pulumi.WithMocks("test", "stack", &NetworkTestMockProvider{}))

	assert.NoError(t, err)
}

// TestVPNConnectivityChecker_SetSSHKeyPath tests setting SSH key path
func TestVPNConnectivityChecker_SetSSHKeyPath(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewVPNConnectivityChecker(ctx)

		keyPath := "/path/to/key"
		checker.SetSSHKeyPath(keyPath)
		assert.Equal(t, keyPath, checker.sshKeyPath)

		return nil
	}, pulumi.WithMocks("test", "stack", &NetworkTestMockProvider{}))

	assert.NoError(t, err)
}

// TestVPNConnectivityChecker_getSSHPrivateKey tests SSH key retrieval
func TestVPNConnectivityChecker_getSSHPrivateKey(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewVPNConnectivityChecker(ctx)

		key := checker.getSSHPrivateKey()
		assert.NotEmpty(t, key)
		assert.Equal(t, "SSH_PRIVATE_KEY_CONTENT", key)

		return nil
	}, pulumi.WithMocks("test", "stack", &NetworkTestMockProvider{}))

	assert.NoError(t, err)
}

// TestVPNConnectivityChecker_buildConnectivityCheckScript tests script generation
func TestVPNConnectivityChecker_buildConnectivityCheckScript(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewVPNConnectivityChecker(ctx)

		targetIP := "10.8.0.2"
		script := checker.buildConnectivityCheckScript(targetIP)

		assert.NotEmpty(t, script)
		assert.Contains(t, script, targetIP)
		assert.Contains(t, script, "ping")
		assert.Contains(t, script, "wg show")
		assert.Contains(t, script, "PING_STATUS")

		return nil
	}, pulumi.WithMocks("test", "stack", &NetworkTestMockProvider{}))

	assert.NoError(t, err)
}

// TestVPNConnectivityChecker_parseConnectivityOutput tests output parsing
func TestVPNConnectivityChecker_parseConnectivityOutput(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewVPNConnectivityChecker(ctx)

		tests := []struct {
			name           string
			output         string
			expectedStatus bool
			expectedLoss   float64
		}{
			{
				name:           "Successful ping",
				output:         "PING_STATUS:SUCCESS\nPACKET_LOSS:0\nAVG_LATENCY:1.5\nHANDSHAKE:ACTIVE",
				expectedStatus: true,
				expectedLoss:   0,
			},
			{
				name:           "Failed ping",
				output:         "PING_STATUS:FAILED\nPACKET_LOSS:100",
				expectedStatus: false,
				expectedLoss:   100,
			},
			{
				name:           "Partial packet loss",
				output:         "PING_STATUS:SUCCESS\nPACKET_LOSS:10\nAVG_LATENCY:2.5",
				expectedStatus: true,
				expectedLoss:   10,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				status := &ConnectionStatus{}
				checker.parseConnectivityOutput(tt.output, status)

				assert.Equal(t, tt.expectedStatus, status.IsConnected)
				assert.Equal(t, tt.expectedLoss, status.PacketLoss)

				if tt.expectedStatus && contains(tt.output, "HANDSHAKE:ACTIVE") {
					assert.NotNil(t, status.WireGuardStats)
				}
			})
		}

		return nil
	}, pulumi.WithMocks("test", "stack", &NetworkTestMockProvider{}))

	assert.NoError(t, err)
}

// TestManager_NetworkCreationWorkflow tests complete network workflow
func TestManager_NetworkCreationWorkflow(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		networkConfig := &config.NetworkConfig{
			CIDR:        "10.0.0.0/16",
			PodCIDR:     "10.244.0.0/16",
			ServiceCIDR: "10.96.0.0/12",
			DNSServers:  []string{"1.1.1.1", "8.8.8.8"},
			WireGuard: &config.WireGuardConfig{
				Enabled: true,
				Port:    51820,
			},
			EnableNodePorts: true,
		}

		// Create manager
		manager := NewManager(ctx, networkConfig)
		assert.NotNil(t, manager)

		// Register providers
		manager.RegisterProvider("digitalocean", &MockProvider{name: "digitalocean"})
		manager.RegisterProvider("linode", &MockProvider{name: "linode"})

		// Validate CIDRs
		err := manager.ValidateCIDRs()
		assert.NoError(t, err)

		// Create networks
		err = manager.CreateNetworks()
		assert.NoError(t, err)

		// Create nodes
		nodes := map[string][]*providers.NodeOutput{
			"digitalocean": {
				{
					ID:          pulumi.ID("node-1").ToIDOutput(),
					Name:        "master-1",
					PublicIP:    pulumi.String("1.2.3.4").ToStringOutput(),
					PrivateIP:   pulumi.String("10.0.0.1").ToStringOutput(),
					WireGuardIP: "10.8.0.1",
				},
			},
			"linode": {
				{
					ID:          pulumi.ID("node-2").ToIDOutput(),
					Name:        "worker-1",
					PublicIP:    pulumi.String("5.6.7.8").ToStringOutput(),
					PrivateIP:   pulumi.String("10.0.0.2").ToStringOutput(),
					WireGuardIP: "10.8.0.2",
				},
			},
		}

		// Create firewalls
		err = manager.CreateFirewalls(nodes)
		assert.NoError(t, err)

		// Export outputs
		manager.ExportNetworkOutputs()

		// Verify DNS servers
		dnsServers := manager.GetDNSServers()
		assert.Len(t, dnsServers, 2)
		assert.Contains(t, dnsServers, "1.1.1.1")

		// Allocate IPs
		ips, err := manager.AllocateNodeIPs(5)
		assert.NoError(t, err)
		assert.Len(t, ips, 5)

		return nil
	}, pulumi.WithMocks("test", "stack", &NetworkTestMockProvider{}))

	assert.NoError(t, err)
}

// TestVPNConnectivityChecker_FullWorkflow tests VPN connectivity checking workflow
func TestVPNConnectivityChecker_FullWorkflow(t *testing.T) {
	t.Skip("Skipping full connectivity check - requires real SSH connections")

	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewVPNConnectivityChecker(ctx)
		checker.SetSSHKeyPath("/path/to/key")

		// Add nodes
		nodes := []*providers.NodeOutput{
			{
				Name:        "master-1",
				PublicIP:    pulumi.String("1.2.3.4").ToStringOutput(),
				PrivateIP:   pulumi.String("10.0.0.1").ToStringOutput(),
				WireGuardIP: "10.8.0.1",
				SSHUser:     "root",
			},
			{
				Name:        "worker-1",
				PublicIP:    pulumi.String("5.6.7.8").ToStringOutput(),
				PrivateIP:   pulumi.String("10.0.0.2").ToStringOutput(),
				WireGuardIP: "10.8.0.2",
				SSHUser:     "root",
			},
		}

		for _, node := range nodes {
			checker.AddNode(node)
		}

		// Wait for tunnels
		err := checker.WaitForTunnelEstablishment()
		assert.NoError(t, err)

		// Verify connectivity
		err = checker.VerifyFullMeshConnectivity()
		assert.NoError(t, err)

		// Print matrix
		checker.PrintConnectivityMatrix()

		return nil
	}, pulumi.WithMocks("test", "stack", &NetworkTestMockProvider{}))

	assert.NoError(t, err)
}

// TestManager_createFirewallConfig tests firewall config generation
func TestManager_createFirewallConfig(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		tests := []struct {
			name           string
			networkConfig  *config.NetworkConfig
			expectedRules  int
			checkWireGuard bool
			checkNodePorts bool
		}{
			{
				name: "Basic config without WireGuard",
				networkConfig: &config.NetworkConfig{
					CIDR: "10.0.0.0/16",
				},
				expectedRules:  7, // Kubernetes rules only
				checkWireGuard: false,
				checkNodePorts: false,
			},
			{
				name: "Config with WireGuard",
				networkConfig: &config.NetworkConfig{
					CIDR: "10.0.0.0/16",
					WireGuard: &config.WireGuardConfig{
						Enabled: true,
						Port:    51820,
					},
				},
				expectedRules:  10, // Kubernetes + WireGuard rules
				checkWireGuard: true,
				checkNodePorts: false,
			},
			{
				name: "Config with NodePorts",
				networkConfig: &config.NetworkConfig{
					CIDR:            "10.0.0.0/16",
					EnableNodePorts: true,
				},
				expectedRules:  8, // Kubernetes + NodePort rules
				checkWireGuard: false,
				checkNodePorts: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				manager := NewManager(ctx, tt.networkConfig)
				firewallConfig := manager.createFirewallConfig("test-provider")

				assert.NotNil(t, firewallConfig)
				assert.NotEmpty(t, firewallConfig.Name)
				assert.Len(t, firewallConfig.InboundRules, tt.expectedRules)

				if tt.checkWireGuard {
					// Check for WireGuard rule
					hasWireGuard := false
					for _, rule := range firewallConfig.InboundRules {
						if rule.Port == "51820" && rule.Protocol == "udp" {
							hasWireGuard = true
							break
						}
					}
					assert.True(t, hasWireGuard, "WireGuard rule not found")
				}

				if tt.checkNodePorts {
					// Check for NodePort rule
					hasNodePorts := false
					for _, rule := range firewallConfig.InboundRules {
						if rule.Port == "30000-32767" {
							hasNodePorts = true
							break
						}
					}
					assert.True(t, hasNodePorts, "NodePort rule not found")
				}
			})
		}

		return nil
	}, pulumi.WithMocks("test", "stack", &NetworkTestMockProvider{}))

	assert.NoError(t, err)
}
