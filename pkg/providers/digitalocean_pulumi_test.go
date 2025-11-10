package providers

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

type mocks int

// NewResource creates a new mock resource
func (mocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	outputs := args.Inputs.Copy()

	// Add mock outputs based on resource type
	switch args.TypeToken {
	case "digitalocean:index/sshKey:SshKey":
		outputs["fingerprint"] = resource.NewStringProperty("mock:ssh:fingerprint:123")
		outputs["id"] = resource.NewStringProperty("12345")
	case "digitalocean:index/vpc:Vpc":
		outputs["id"] = resource.NewStringProperty("vpc-12345")
		outputs["urn"] = resource.NewStringProperty("do:vpc:nyc1:vpc-12345")
		outputs["ipRange"] = args.Inputs["ipRange"]
	case "digitalocean:index/firewall:Firewall":
		outputs["id"] = resource.NewStringProperty("fw-12345")
		outputs["status"] = resource.NewStringProperty("active")
	case "digitalocean:index/droplet:Droplet":
		outputs["id"] = resource.NewStringProperty("droplet-12345")
		outputs["ipv4Address"] = resource.NewStringProperty("192.0.2.1")
		outputs["ipv4AddressPrivate"] = resource.NewStringProperty("10.0.0.1")
		outputs["status"] = resource.NewStringProperty("active")
	}

	return args.Name + "_id", outputs, nil
}

// Call mocks function calls
func (mocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	outputs := resource.PropertyMap{}
	return outputs, nil
}

func TestDigitalOceanProvider_Initialize(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// Create test config
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled:      true,
					Region:       "nyc1",
					SSHKeys:      []string{"existing-key-fingerprint"},
					SSHPublicKey: nil, // Test with existing keys
				},
			},
		}

		// Create provider instance
		provider := NewDigitalOceanProvider()

		// Initialize provider
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err, "Initialize should not return error")
		assert.NotNil(t, provider.config, "Provider config should be set")
		assert.Equal(t, "digitalocean", provider.GetName(), "Provider name should be 'digitalocean'")
		assert.NotEmpty(t, provider.sshKeys, "SSH keys should be set")

		return nil
	}, pulumi.WithMocks("project", "stack", mocks(0)))

	assert.NoError(t, err)
}

func TestDigitalOceanProvider_InitializeWithNewSSHKey(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// Create test config with new SSH key
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled:      true,
					Region:       "nyc1",
					SSHPublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC... test@example.com",
					SSHKeys:      []string{},
				},
			},
		}

		// Create provider instance
		provider := NewDigitalOceanProvider()

		// Initialize provider (should create new SSH key)
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err, "Initialize with new SSH key should not return error")
		assert.NotNil(t, provider.sshKeys, "SSH keys should be created")

		return nil
	}, pulumi.WithMocks("project", "stack", mocks(0)))

	assert.NoError(t, err)
}

func TestDigitalOceanProvider_InitializeNotEnabled(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// Create test config with provider disabled
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: false,
				},
			},
		}

		provider := NewDigitalOceanProvider()
		err := provider.Initialize(ctx, clusterConfig)

		// Should return error since provider is not enabled
		assert.Error(t, err, "Initialize should return error when provider is disabled")
		assert.Contains(t, err.Error(), "not enabled", "Error should mention provider is not enabled")

		return nil
	}, pulumi.WithMocks("project", "stack", mocks(0)))

	assert.NoError(t, err)
}

func TestDigitalOceanProvider_InitializeNoSSHKeys(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// Create test config without SSH keys
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled:      true,
					Region:       "nyc1",
					SSHPublicKey: nil,
					SSHKeys:      []string{}, // No SSH keys
				},
			},
		}

		provider := NewDigitalOceanProvider()
		err := provider.Initialize(ctx, clusterConfig)

		// Should return error since no SSH keys are configured
		assert.Error(t, err, "Initialize should return error when no SSH keys configured")
		assert.Contains(t, err.Error(), "no SSH keys", "Error should mention missing SSH keys")

		return nil
	}, pulumi.WithMocks("project", "stack", mocks(0)))

	assert.NoError(t, err)
}

func TestDigitalOceanProvider_CreateNode(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// Setup provider
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Region:  "nyc1",
					SSHKeys: []string{"test-key"},
				},
			},
		}

		provider := NewDigitalOceanProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		// Create test node config
		nodeConfig := &config.NodeConfig{
			Name:        "test-node",
			Provider:    "digitalocean",
			Region:      "nyc1",
			Size:        "s-2vcpu-4gb",
			Roles:       []string{"master"},
			WireGuardIP: "10.10.0.1",
		}

		// Create node
		nodeOutput, err := provider.CreateNode(ctx, nodeConfig)
		assert.NoError(t, err, "CreateNode should not return error")
		assert.NotNil(t, nodeOutput, "Node output should not be nil")
		assert.Equal(t, "test-node", nodeOutput.Name, "Node name should match")

		return nil
	}, pulumi.WithMocks("project", "stack", mocks(0)))

	assert.NoError(t, err)
}

func TestDigitalOceanProvider_CreateVPC(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// Setup provider
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Region:  "nyc1",
					SSHKeys: []string{"test-key"},
					VPC: &config.VPCConfig{
						Name: "test-vpc",
						CIDR: "10.0.0.0/16",
					},
				},
			},
		}

		provider := NewDigitalOceanProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		// Note: CreateVPC method doesn't exist on DigitalOceanProvider
		// VPC is created separately via VPCComponent
		// This test just verifies provider initialization with VPC config
		assert.NotNil(t, provider.config.VPC, "VPC config should be set")
		assert.Equal(t, "test-vpc", provider.config.VPC.Name, "VPC name should match")

		return nil
	}, pulumi.WithMocks("project", "stack", mocks(0)))

	assert.NoError(t, err)
}

func TestDigitalOceanProvider_Concurrency(t *testing.T) {
	// Test sequential node creation (Pulumi mocks are not thread-safe)
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Region:  "nyc1",
					SSHKeys: []string{"test-key"},
				},
			},
		}

		provider := NewDigitalOceanProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		// Create multiple nodes sequentially (Pulumi mocks are not thread-safe)
		nodeConfigs := []config.NodeConfig{
			{Name: "node1", Provider: "digitalocean", Region: "nyc1", Size: "s-2vcpu-4gb", Roles: []string{"master"}, WireGuardIP: "10.10.0.1"},
			{Name: "node2", Provider: "digitalocean", Region: "nyc1", Size: "s-2vcpu-4gb", Roles: []string{"worker"}, WireGuardIP: "10.10.0.2"},
			{Name: "node3", Provider: "digitalocean", Region: "nyc1", Size: "s-2vcpu-4gb", Roles: []string{"worker"}, WireGuardIP: "10.10.0.3"},
		}

		for _, nc := range nodeConfigs {
			_, err := provider.CreateNode(ctx, &nc)
			assert.NoError(t, err)
		}

		return nil
	}, pulumi.WithMocks("project", "stack", mocks(0)))

	assert.NoError(t, err)
}

func TestDigitalOceanProvider_GetRegionsAndSizes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Region:  "nyc1",
					SSHKeys: []string{"test-key"},
				},
			},
		}

		provider := NewDigitalOceanProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		// Test GetRegions
		regions := provider.GetRegions()
		assert.NotEmpty(t, regions, "Should have available regions")

		// Test GetSizes
		sizes := provider.GetSizes()
		assert.NotEmpty(t, sizes, "Should have available sizes")

		return nil
	}, pulumi.WithMocks("project", "stack", mocks(0)))

	assert.NoError(t, err)
}
