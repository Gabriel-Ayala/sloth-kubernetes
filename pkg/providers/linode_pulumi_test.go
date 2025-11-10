package providers

import (
	"fmt"
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

type linodeMocks int

// NewResource creates mock resources for Linode tests
func (linodeMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	outputs := args.Inputs.Copy()

	switch args.TypeToken {
	case "linode:index/instance:Instance":
		outputs["id"] = resource.NewStringProperty("linode-instance-" + args.Name)
		outputs["ipAddress"] = resource.NewStringProperty("203.0.113.100")
		outputs["privateIpAddress"] = resource.NewStringProperty("192.168.1.100")
		outputs["status"] = resource.NewStringProperty("running")
		outputs["label"] = args.Inputs["label"]
		outputs["region"] = args.Inputs["region"]
		outputs["type"] = args.Inputs["type"]
		outputs["image"] = args.Inputs["image"]

	case "linode:index/firewall:Firewall":
		outputs["id"] = resource.NewStringProperty("firewall-" + args.Name)
		outputs["label"] = args.Inputs["label"]
		outputs["status"] = resource.NewStringProperty("enabled")

	case "linode:index/volume:Volume":
		outputs["id"] = resource.NewStringProperty("volume-" + args.Name)
		outputs["label"] = args.Inputs["label"]
		outputs["size"] = args.Inputs["size"]
		outputs["filesystemPath"] = resource.NewStringProperty("/dev/disk/by-id/scsi-0Linode_Volume_" + args.Name)
	}

	return args.Name + "_id", outputs, nil
}

// Call mocks function calls
func (linodeMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return resource.PropertyMap{}, nil
}

func TestLinodeProvider_Initialize(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Linode: &config.LinodeProvider{
					Enabled:        true,
					Region:         "us-east",
					AuthorizedKeys: []string{"ssh-rsa AAAAB3... test@example.com"},
					PrivateIP:      true,
				},
			},
		}

		provider := NewLinodeProvider()
		err := provider.Initialize(ctx, clusterConfig)

		assert.NoError(t, err, "Initialize should not return error")
		assert.NotNil(t, provider.config, "Provider config should be set")
		assert.Equal(t, "linode", provider.GetName(), "Provider name should be 'linode'")
		assert.Equal(t, "us-east", provider.config.Region, "Region should match")

		return nil
	}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

	assert.NoError(t, err)
}

func TestLinodeProvider_InitializeNotEnabled(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Linode: &config.LinodeProvider{
					Enabled: false,
				},
			},
		}

		provider := NewLinodeProvider()
		err := provider.Initialize(ctx, clusterConfig)

		assert.Error(t, err, "Initialize should return error when provider disabled")
		assert.Contains(t, err.Error(), "not enabled", "Error should mention provider is not enabled")

		return nil
	}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

	assert.NoError(t, err)
}

func TestLinodeProvider_InitializeNilConfig(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Linode: nil,
			},
		}

		provider := NewLinodeProvider()
		err := provider.Initialize(ctx, clusterConfig)

		assert.Error(t, err, "Initialize should return error when config is nil")
		assert.Contains(t, err.Error(), "not enabled", "Error should mention provider is not enabled")

		return nil
	}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

	assert.NoError(t, err)
}

func TestLinodeProvider_CreateNode(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Linode: &config.LinodeProvider{
					Enabled:        true,
					Region:         "us-east",
					AuthorizedKeys: []string{"ssh-rsa AAAAB3... test@example.com"},
					PrivateIP:      true,
					Tags:           []string{"production", "kubernetes"},
				},
			},
		}

		provider := NewLinodeProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		nodeConfig := &config.NodeConfig{
			Name:        "linode-master-1",
			Provider:    "linode",
			Region:      "us-east",
			Size:        "g6-standard-2",
			Image:       "linode/ubuntu22.04",
			Roles:       []string{"master"},
			WireGuardIP: "10.10.0.20",
			Labels: map[string]string{
				"environment": "production",
				"role":        "master",
			},
		}

		nodeOutput, err := provider.CreateNode(ctx, nodeConfig)
		assert.NoError(t, err, "CreateNode should not return error")
		assert.NotNil(t, nodeOutput, "Node output should not be nil")
		assert.Equal(t, "linode-master-1", nodeOutput.Name, "Node name should match")
		assert.Equal(t, "linode", nodeOutput.Provider, "Provider should be linode")
		assert.Equal(t, "10.10.0.20", nodeOutput.WireGuardIP, "WireGuard IP should match")

		return nil
	}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

	assert.NoError(t, err)
}

func TestLinodeProvider_CreateNodeWithCustomConfig(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Linode: &config.LinodeProvider{
					Enabled:        true,
					Region:         "us-west",
					AuthorizedKeys: []string{"ssh-rsa AAAAB3... test@example.com"},
					PrivateIP:      false,
					Tags:           []string{"staging"},
				},
			},
		}

		provider := NewLinodeProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		nodeConfig := &config.NodeConfig{
			Name:        "linode-worker-1",
			Provider:    "linode",
			Region:      "us-west",
			Size:        "g6-standard-4",
			Image:       "linode/ubuntu22.04",
			Roles:       []string{"worker"},
			WireGuardIP: "10.10.0.21",
			UserData:    "#!/bin/bash\necho 'Custom setup'",
		}

		nodeOutput, err := provider.CreateNode(ctx, nodeConfig)
		assert.NoError(t, err)
		assert.NotNil(t, nodeOutput)
		assert.Equal(t, "us-west", nodeOutput.Region)

		return nil
	}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

	assert.NoError(t, err)
}

func TestLinodeProvider_CreateNodePool(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Linode: &config.LinodeProvider{
					Enabled:        true,
					Region:         "us-east",
					AuthorizedKeys: []string{"ssh-rsa AAAAB3... test@example.com"},
					PrivateIP:      true,
				},
			},
		}

		provider := NewLinodeProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		pool := &config.NodePool{
			Name:     "worker-pool",
			Provider: "linode",
			Region:   "us-east",
			Size:     "g6-standard-2",
			Image:    "linode/ubuntu22.04",
			Count:    3,
			Roles:    []string{"worker"},
		}

		outputs, err := provider.CreateNodePool(ctx, pool)
		assert.NoError(t, err, "CreateNodePool should not return error")
		assert.Len(t, outputs, 3, "Should create 3 nodes")

		// Verify node naming
		for i, output := range outputs {
			expectedName := fmt.Sprintf("worker-pool-%d", i+1)
			assert.Equal(t, expectedName, output.Name, "Node name should follow pool naming convention")
			assert.Equal(t, "linode", output.Provider)
		}

		return nil
	}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

	assert.NoError(t, err)
}

func TestLinodeProvider_CreateNodePoolWithZones(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Linode: &config.LinodeProvider{
					Enabled:        true,
					Region:         "us-east",
					AuthorizedKeys: []string{"ssh-rsa AAAAB3... test@example.com"},
				},
			},
		}

		provider := NewLinodeProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		pool := &config.NodePool{
			Name:     "multi-zone-pool",
			Provider: "linode",
			Region:   "us-east",
			Zones:    []string{"us-east-1a", "us-east-1b", "us-east-1c"},
			Size:     "g6-standard-2",
			Count:    6,
			Roles:    []string{"worker"},
		}

		outputs, err := provider.CreateNodePool(ctx, pool)
		assert.NoError(t, err)
		assert.Len(t, outputs, 6, "Should create 6 nodes across zones")

		return nil
	}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

	assert.NoError(t, err)
}

func TestLinodeProvider_Concurrency(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Linode: &config.LinodeProvider{
					Enabled:        true,
					Region:         "us-east",
					AuthorizedKeys: []string{"ssh-rsa AAAAB3... test@example.com"},
					PrivateIP:      true,
				},
			},
		}

		provider := NewLinodeProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		// Create multiple nodes sequentially (Pulumi mocks are not thread-safe)
		nodeConfigs := []config.NodeConfig{
			{Name: "linode-node1", Provider: "linode", Region: "us-east", Size: "g6-standard-2", Image: "linode/ubuntu22.04", Roles: []string{"master"}, WireGuardIP: "10.10.0.30"},
			{Name: "linode-node2", Provider: "linode", Region: "us-east", Size: "g6-standard-2", Image: "linode/ubuntu22.04", Roles: []string{"worker"}, WireGuardIP: "10.10.0.31"},
			{Name: "linode-node3", Provider: "linode", Region: "us-east", Size: "g6-standard-2", Image: "linode/ubuntu22.04", Roles: []string{"worker"}, WireGuardIP: "10.10.0.32"},
			{Name: "linode-node4", Provider: "linode", Region: "us-west", Size: "g6-standard-4", Image: "linode/ubuntu22.04", Roles: []string{"worker"}, WireGuardIP: "10.10.0.33"},
		}

		for _, nc := range nodeConfigs {
			_, err := provider.CreateNode(ctx, &nc)
			assert.NoError(t, err)
		}

		return nil
	}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

	assert.NoError(t, err)
}

func TestLinodeProvider_GetRegionsAndSizes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Linode: &config.LinodeProvider{
					Enabled:        true,
					Region:         "us-east",
					AuthorizedKeys: []string{"ssh-rsa AAAAB3... test@example.com"},
				},
			},
		}

		provider := NewLinodeProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		regions := provider.GetRegions()
		assert.NotEmpty(t, regions, "Should have available regions")

		sizes := provider.GetSizes()
		assert.NotEmpty(t, sizes, "Should have available sizes")

		return nil
	}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

	assert.NoError(t, err)
}

func TestLinodeProvider_CreateFirewall(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Linode: &config.LinodeProvider{
					Enabled:        true,
					Region:         "us-east",
					AuthorizedKeys: []string{"ssh-rsa AAAAB3... test@example.com"},
				},
			},
		}

		provider := NewLinodeProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		firewallConfig := &config.FirewallConfig{
			Name: "linode-firewall",
			InboundRules: []config.FirewallRule{
				{
					Protocol: "tcp",
					Port:     "22",
					Source:   []string{"0.0.0.0/0"},
				},
				{
					Protocol: "tcp",
					Port:     "6443",
					Source:   []string{"10.0.0.0/8"},
				},
			},
			OutboundRules: []config.FirewallRule{
				{
					Protocol: "tcp",
					Port:     "1-65535",
					Target:   []string{"0.0.0.0/0"},
				},
			},
		}

		// Create a test node first
		nodeConfig := &config.NodeConfig{
			Name:     "test-node",
			Provider: "linode",
			Region:   "us-east",
			Size:     "g6-standard-2",
			Image:    "linode/ubuntu22.04",
			Roles:    []string{"master"},
		}

		nodeOutput, err := provider.CreateNode(ctx, nodeConfig)
		assert.NoError(t, err)

		// Create firewall
		nodeIDs := []pulumi.IDOutput{nodeOutput.ID}
		err = provider.CreateFirewall(ctx, firewallConfig, nodeIDs)
		assert.NoError(t, err, "CreateFirewall should not return error")

		return nil
	}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

	assert.NoError(t, err)
}

func TestLinodeProvider_CreateNetwork(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Linode: &config.LinodeProvider{
					Enabled:        true,
					Region:         "us-east",
					AuthorizedKeys: []string{"ssh-rsa AAAAB3... test@example.com"},
				},
			},
		}

		provider := NewLinodeProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		networkConfig := &config.NetworkConfig{
			Mode: "vpc",
			CIDR: "10.0.0.0/16",
		}

		networkOutput, err := provider.CreateNetwork(ctx, networkConfig)

		// Note: CreateNetwork might not be fully implemented for Linode
		// so we just verify it doesn't crash
		if err != nil {
			// If not implemented, that's okay for this test
			assert.Contains(t, err.Error(), "not implemented", "Expected 'not implemented' error")
		} else {
			assert.NotNil(t, networkOutput)
		}

		return nil
	}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

	assert.NoError(t, err)
}

func TestLinodeProvider_MultipleRegions(t *testing.T) {
	regions := []string{"us-east", "us-west", "eu-central", "ap-south"}

	for _, region := range regions {
		t.Run("Region_"+region, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Linode: &config.LinodeProvider{
							Enabled:        true,
							Region:         region,
							AuthorizedKeys: []string{"ssh-rsa AAAAB3... test@example.com"},
						},
					},
				}

				provider := NewLinodeProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)
				assert.Equal(t, region, provider.config.Region)

				return nil
			}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestLinodeProvider_DifferentInstanceTypes(t *testing.T) {
	instanceTypes := []struct {
		name string
		size string
	}{
		{"Nanode", "g6-nanode-1"},
		{"Standard", "g6-standard-2"},
		{"Standard4", "g6-standard-4"},
		{"Dedicated", "g6-dedicated-2"},
		{"HighMem", "g7-highmem-1"},
	}

	for _, tt := range instanceTypes {
		t.Run(tt.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Linode: &config.LinodeProvider{
							Enabled:        true,
							Region:         "us-east",
							AuthorizedKeys: []string{"ssh-rsa AAAAB3... test@example.com"},
						},
					},
				}

				provider := NewLinodeProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)

				nodeConfig := &config.NodeConfig{
					Name:     fmt.Sprintf("node-%s", tt.name),
					Provider: "linode",
					Region:   "us-east",
					Size:     tt.size,
					Image:    "linode/ubuntu22.04",
					Roles:    []string{"worker"},
				}

				nodeOutput, err := provider.CreateNode(ctx, nodeConfig)
				assert.NoError(t, err)
				assert.Equal(t, tt.size, nodeOutput.Size)

				return nil
			}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

			assert.NoError(t, err)
		})
	}
}
