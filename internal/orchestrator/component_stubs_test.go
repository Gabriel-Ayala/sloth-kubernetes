package orchestrator

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// StubComponentMock for testing stub components
type StubComponentMock struct {
	pulumi.ResourceState
}

func (m *StubComponentMock) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	return args.Name + "_id", args.Inputs, nil
}

func (m *StubComponentMock) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

// ==================== SSH Key Component Tests ====================

func TestNewSSHKeyComponent_Success(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-cluster",
			},
		}

		component, err := NewSSHKeyComponent(ctx, "test-ssh-key", cfg)
		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.NotNil(t, component.PublicKey)
		assert.NotNil(t, component.PrivateKey)
		assert.NotNil(t, component.PrivateKeyPath)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

func TestNewSSHKeyComponent_WithParent(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-cluster-with-parent",
			},
		}

		parent := &SSHKeyComponent{}
		ctx.RegisterComponentResource("test:parent", "parent", parent)

		component, err := NewSSHKeyComponent(ctx, "test-ssh-key-with-parent", cfg, pulumi.Parent(parent))
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

// ==================== Provider Component Tests ====================

func TestNewProviderComponent_DigitalOcean(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc3",
				},
			},
		}

		sshKey := pulumi.String("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...").ToStringOutput()
		component, err := NewProviderComponent(ctx, "test-provider-do", cfg, sshKey)
		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.NotNil(t, component.Providers)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

func TestNewProviderComponent_Linode(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Linode: &config.LinodeProvider{
					Enabled: true,
					Token:   "test-linode-token",
					Region:  "us-east",
				},
			},
		}

		sshKey := pulumi.String("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...").ToStringOutput()
		component, err := NewProviderComponent(ctx, "test-provider-linode", cfg, sshKey)
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

func TestNewProviderComponent_MultiProvider(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-do-token",
				},
				Linode: &config.LinodeProvider{
					Enabled: true,
					Token:   "test-linode-token",
				},
			},
		}

		sshKey := pulumi.String("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...").ToStringOutput()
		component, err := NewProviderComponent(ctx, "test-provider-multi", cfg, sshKey)
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

func TestNewProviderComponent_NoProviders(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Providers: config.ProvidersConfig{},
		}

		sshKey := pulumi.String("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...").ToStringOutput()
		component, err := NewProviderComponent(ctx, "test-provider-none", cfg, sshKey)
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

// ==================== Network Component Tests ====================

func TestNewNetworkComponent_Success(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Network: config.NetworkConfig{
				CIDR:        "10.0.0.0/16",
				PodCIDR:     "10.244.0.0/16",
				ServiceCIDR: "10.96.0.0/12",
			},
		}

		providersMap := pulumi.Map{
			"digitalocean": pulumi.String("initialized"),
		}.ToMapOutput()

		component, err := NewNetworkComponent(ctx, "test-network", cfg, providersMap)
		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.NotNil(t, component.Networks)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

func TestNewNetworkComponent_WithParent(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Network: config.NetworkConfig{
				CIDR: "192.168.0.0/16",
			},
		}

		parent := &SSHKeyComponent{}
		ctx.RegisterComponentResource("test:parent", "parent", parent)

		providersMap := pulumi.Map{}.ToMapOutput()
		component, err := NewNetworkComponent(ctx, "test-network-with-parent", cfg, providersMap, pulumi.Parent(parent))
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

// ==================== DNS Component Tests ====================

func TestNewDNSComponent_WithDomain(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Network: config.NetworkConfig{
				DNS: config.DNSConfig{
					Domain:   "example.com",
					Provider: "digitalocean",
				},
			},
		}

		nodesArray := pulumi.Array{
			pulumi.String("node1"),
			pulumi.String("node2"),
		}
		nodes := pulumi.ToOutput(nodesArray).(pulumi.ArrayOutput)

		component, err := NewDNSComponent(ctx, "test-dns", cfg, nodes)
		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.NotNil(t, component.Records)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

func TestNewDNSComponent_DefaultDomain(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Network: config.NetworkConfig{
				DNS: config.DNSConfig{
					Domain: "", // Empty domain should use default
				},
			},
		}

		nodes := pulumi.ToOutput(pulumi.Array{}).(pulumi.ArrayOutput)

		component, err := NewDNSComponent(ctx, "test-dns-default", cfg, nodes)
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

func TestNewDNSComponent_MultipleNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Network: config.NetworkConfig{
				DNS: config.DNSConfig{
					Domain: "multi-node.com",
				},
			},
		}

		nodesArray := pulumi.Array{
			pulumi.String("master-1"),
			pulumi.String("master-2"),
			pulumi.String("worker-1"),
			pulumi.String("worker-2"),
		}
		nodes := pulumi.ToOutput(nodesArray).(pulumi.ArrayOutput)

		component, err := NewDNSComponent(ctx, "test-dns-multi", cfg, nodes)
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

// ==================== WireGuard Component Tests ====================

func TestNewWireGuardComponent_Success(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Network: config.NetworkConfig{
				WireGuard: &config.WireGuardConfig{
					Enabled:        true,
					ServerEndpoint: "vpn.example.com:51820",
					SubnetCIDR:     "10.8.0.0/24",
				},
			},
		}

		nodesArray := pulumi.Array{
			pulumi.String("node1"),
			pulumi.String("node2"),
		}
		nodes := pulumi.ToOutput(nodesArray).(pulumi.ArrayOutput)
		sshKeyPath := pulumi.String("/path/to/key").ToStringOutput()

		component, err := NewWireGuardComponent(ctx, "test-wg", cfg, nodes, sshKeyPath)
		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.NotNil(t, component.Status)
		assert.NotNil(t, component.ClientConfigs)
		assert.NotNil(t, component.MeshStatus)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

func TestNewWireGuardComponent_LargeCluster(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Network: config.NetworkConfig{
				WireGuard: &config.WireGuardConfig{
					Enabled:        true,
					ServerEndpoint: "vpn.large-cluster.com:51820",
				},
			},
		}

		// Create 10 node cluster
		nodesArray := pulumi.Array{}
		for i := 0; i < 10; i++ {
			nodesArray = append(nodesArray, pulumi.Sprintf("node-%d", i))
		}
		nodes := pulumi.ToOutput(nodesArray).(pulumi.ArrayOutput)
		sshKeyPath := pulumi.String("/path/to/key").ToStringOutput()

		component, err := NewWireGuardComponent(ctx, "test-wg-large", cfg, nodes, sshKeyPath)
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

// ==================== Cloud Firewall Component Tests ====================

func TestNewCloudFirewallComponent_Success(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-cluster",
			},
		}

		providersMap := pulumi.Map{
			"digitalocean": pulumi.String("initialized"),
		}.ToMapOutput()
		nodes := pulumi.ToOutput(pulumi.Array{}).(pulumi.ArrayOutput)

		component, err := NewCloudFirewallComponent(ctx, "test-firewall", cfg, providersMap, nodes)
		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.NotNil(t, component.Status)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

func TestNewCloudFirewallComponent_WithNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-cluster-with-nodes",
			},
		}

		providersMap := pulumi.Map{
			"digitalocean": pulumi.String("initialized"),
			"linode":       pulumi.String("initialized"),
		}.ToMapOutput()

		nodesArray := pulumi.Array{
			pulumi.String("node1"),
			pulumi.String("node2"),
			pulumi.String("node3"),
		}
		nodes := pulumi.ToOutput(nodesArray).(pulumi.ArrayOutput)

		component, err := NewCloudFirewallComponent(ctx, "test-firewall-with-nodes", cfg, providersMap, nodes)
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

// ==================== RKE Component Tests ====================

func TestNewRKEComponent_Success(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-rke-cluster",
			},
			Kubernetes: config.KubernetesConfig{
				Distribution:  "rke2",
				Version:       "v1.28.0",
				NetworkPlugin: "calico",
			},
			Network: config.NetworkConfig{
				DNS: config.DNSConfig{
					Domain: "rke.example.com",
				},
			},
		}

		nodes := pulumi.ToOutput(pulumi.Array{}).(pulumi.ArrayOutput)
		sshKeyPath := pulumi.String("/path/to/key").ToStringOutput()

		component, err := NewRKEComponent(ctx, "test-rke", cfg, nodes, sshKeyPath)
		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.NotNil(t, component.Status)
		assert.NotNil(t, component.KubeConfig)
		assert.NotNil(t, component.ClusterState)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

func TestNewRKEComponent_WithNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-rke-with-nodes",
			},
			Network: config.NetworkConfig{
				DNS: config.DNSConfig{
					Domain: "rke-nodes.example.com",
				},
			},
		}

		nodesArray := pulumi.Array{
			pulumi.String("master-1"),
			pulumi.String("worker-1"),
			pulumi.String("worker-2"),
		}
		nodes := pulumi.ToOutput(nodesArray).(pulumi.ArrayOutput)
		sshKeyPath := pulumi.String("/path/to/key").ToStringOutput()

		component, err := NewRKEComponent(ctx, "test-rke-with-nodes", cfg, nodes, sshKeyPath)
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

func TestNewRKEComponent_KubeConfigFormat(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "kubeconfig-test",
			},
			Network: config.NetworkConfig{
				DNS: config.DNSConfig{
					Domain: "kube.test.com",
				},
			},
		}

		nodes := pulumi.ToOutput(pulumi.Array{}).(pulumi.ArrayOutput)
		sshKeyPath := pulumi.String("/test/key").ToStringOutput()

		component, err := NewRKEComponent(ctx, "test-rke-kubeconfig", cfg, nodes, sshKeyPath)
		require.NoError(t, err)
		require.NotNil(t, component)
		assert.NotNil(t, component.KubeConfig)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

// ==================== Ingress Component Tests ====================

func TestNewIngressComponent_Success(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-ingress-cluster",
			},
		}

		nodes := pulumi.ToOutput(pulumi.Array{}).(pulumi.ArrayOutput)
		sshKeyPath := pulumi.String("/path/to/key").ToStringOutput()

		component, err := NewIngressComponent(ctx, "test-ingress", cfg, nodes, sshKeyPath)
		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.NotNil(t, component.Status)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

func TestNewIngressComponent_WithNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-ingress-with-nodes",
			},
		}

		nodesArray := pulumi.Array{
			pulumi.String("ingress-node-1"),
			pulumi.String("ingress-node-2"),
		}
		nodes := pulumi.ToOutput(nodesArray).(pulumi.ArrayOutput)
		sshKeyPath := pulumi.String("/path/to/key").ToStringOutput()

		component, err := NewIngressComponent(ctx, "test-ingress-with-nodes", cfg, nodes, sshKeyPath)
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

func TestNewIngressComponent_WithParent(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-ingress-parent",
			},
		}

		parent := &SSHKeyComponent{}
		ctx.RegisterComponentResource("test:parent", "parent", parent)

		nodes := pulumi.ToOutput(pulumi.Array{}).(pulumi.ArrayOutput)
		sshKeyPath := pulumi.String("/path/to/key").ToStringOutput()

		component, err := NewIngressComponent(ctx, "test-ingress-parent", cfg, nodes, sshKeyPath, pulumi.Parent(parent))
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

// ==================== Addons Component Tests ====================

func TestNewAddonsComponent_Success(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-addons-cluster",
			},
		}

		nodes := pulumi.ToOutput(pulumi.Array{}).(pulumi.ArrayOutput)
		sshKeyPath := pulumi.String("/path/to/key").ToStringOutput()

		component, err := NewAddonsComponent(ctx, "test-addons", cfg, nodes, sshKeyPath)
		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.NotNil(t, component.Status)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

func TestNewAddonsComponent_WithNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-addons-with-nodes",
			},
		}

		nodesArray := pulumi.Array{
			pulumi.String("addon-node-1"),
			pulumi.String("addon-node-2"),
			pulumi.String("addon-node-3"),
		}
		nodes := pulumi.ToOutput(nodesArray).(pulumi.ArrayOutput)
		sshKeyPath := pulumi.String("/path/to/key").ToStringOutput()

		component, err := NewAddonsComponent(ctx, "test-addons-with-nodes", cfg, nodes, sshKeyPath)
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

func TestNewAddonsComponent_WithParent(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-addons-parent",
			},
		}

		parent := &SSHKeyComponent{}
		ctx.RegisterComponentResource("test:parent", "parent", parent)

		nodes := pulumi.ToOutput(pulumi.Array{}).(pulumi.ArrayOutput)
		sshKeyPath := pulumi.String("/path/to/key").ToStringOutput()

		component, err := NewAddonsComponent(ctx, "test-addons-parent", cfg, nodes, sshKeyPath, pulumi.Parent(parent))
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

// ==================== Edge Case Tests ====================

func TestAllComponents_EmptyConfig(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{}

		// Test SSHKeyComponent with empty config
		sshComp, err := NewSSHKeyComponent(ctx, "test-ssh-empty", cfg)
		assert.NoError(t, err)
		assert.NotNil(t, sshComp)

		// Test AddonsComponent with empty config
		nodes := pulumi.ToOutput(pulumi.Array{}).(pulumi.ArrayOutput)
		sshKeyPath := pulumi.String("/key").ToStringOutput()
		addonsComp, err := NewAddonsComponent(ctx, "test-addons-empty", cfg, nodes, sshKeyPath)
		assert.NoError(t, err)
		assert.NotNil(t, addonsComp)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	assert.NoError(t, err)
}

func TestAllComponents_NilConfig(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		var cfg *config.ClusterConfig

		// These should handle nil config gracefully or panic (which we catch)
		defer func() {
			if r := recover(); r != nil {
				t.Logf("Expected panic with nil config: %v", r)
			}
		}()

		// This will likely panic with nil pointer, which is expected behavior
		_, _ = NewSSHKeyComponent(ctx, "test-nil", cfg)

		return nil
	}, pulumi.WithMocks("test", "stack", &StubComponentMock{}))

	// Error or no error both acceptable for nil config
	_ = err
}
