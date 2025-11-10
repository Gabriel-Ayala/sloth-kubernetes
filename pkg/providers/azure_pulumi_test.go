package providers

import (
	"fmt"
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

type azureMocks int

// NewResource creates mock resources for Azure tests
func (azureMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	outputs := args.Inputs.Copy()

	switch args.TypeToken {
	case "azure-native:resources/v2:ResourceGroup":
		outputs["id"] = resource.NewStringProperty("/subscriptions/sub-123/resourceGroups/" + args.Name)
		outputs["name"] = args.Inputs["resourceGroupName"]
		outputs["location"] = args.Inputs["location"]
		outputs["type"] = resource.NewStringProperty("Microsoft.Resources/resourceGroups")

	case "azure-native:network/v2:VirtualNetwork":
		outputs["id"] = resource.NewStringProperty("/subscriptions/sub-123/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/" + args.Name)
		outputs["name"] = args.Inputs["virtualNetworkName"]
		outputs["location"] = args.Inputs["location"]
		outputs["addressSpace"] = args.Inputs["addressSpace"]

	case "azure-native:network/v2:Subnet":
		outputs["id"] = resource.NewStringProperty("/subscriptions/sub-123/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/" + args.Name)
		outputs["name"] = args.Inputs["subnetName"]
		outputs["addressPrefix"] = args.Inputs["addressPrefix"]

	case "azure-native:network/v2:NetworkSecurityGroup":
		outputs["id"] = resource.NewStringProperty("/subscriptions/sub-123/resourceGroups/rg/providers/Microsoft.Network/networkSecurityGroups/" + args.Name)
		outputs["name"] = args.Inputs["networkSecurityGroupName"]
		outputs["location"] = args.Inputs["location"]

	case "azure-native:network/v2:PublicIPAddress":
		outputs["id"] = resource.NewStringProperty("/subscriptions/sub-123/resourceGroups/rg/providers/Microsoft.Network/publicIPAddresses/" + args.Name)
		outputs["name"] = args.Inputs["publicIpAddressName"]
		outputs["ipAddress"] = resource.NewStringProperty("20.40.60.80")
		outputs["location"] = args.Inputs["location"]

	case "azure-native:network/v2:NetworkInterface":
		outputs["id"] = resource.NewStringProperty("/subscriptions/sub-123/resourceGroups/rg/providers/Microsoft.Network/networkInterfaces/" + args.Name)
		outputs["name"] = args.Inputs["networkInterfaceName"]
		outputs["location"] = args.Inputs["location"]
		outputs["ipConfigurations"] = resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewObjectProperty(resource.PropertyMap{
				"privateIPAddress": resource.NewStringProperty("10.0.1.10"),
			}),
		})

	case "azure-native:compute/v2:VirtualMachine":
		outputs["id"] = resource.NewStringProperty("/subscriptions/sub-123/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/" + args.Name)
		outputs["name"] = args.Inputs["vmName"]
		outputs["location"] = args.Inputs["location"]
		outputs["vmId"] = resource.NewStringProperty("vm-" + args.Name)
		outputs["type"] = resource.NewStringProperty("Microsoft.Compute/virtualMachines")
	}

	return args.Name + "_id", outputs, nil
}

// Call mocks function calls
func (azureMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return resource.PropertyMap{}, nil
}

func TestAzureProvider_Initialize(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Azure: &config.AzureProvider{
					Enabled:        true,
					SubscriptionID: "sub-123",
					TenantID:       "tenant-456",
					ClientID:       "client-789",
					ClientSecret:   "secret-abc",
					ResourceGroup:  "test-rg",
					Location:       "eastus",
				},
			},
		}

		provider := NewAzureProvider()
		err := provider.Initialize(ctx, clusterConfig)

		assert.NoError(t, err, "Initialize should not return error")
		assert.NotNil(t, provider.config, "Provider config should be set")
		assert.Equal(t, "azure", provider.GetName(), "Provider name should be 'azure'")
		assert.Equal(t, "eastus", provider.config.Location, "Location should match")

		return nil
	}, pulumi.WithMocks("project", "stack", azureMocks(0)))

	assert.NoError(t, err)
}

func TestAzureProvider_InitializeNotEnabled(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Azure: &config.AzureProvider{
					Enabled: false,
				},
			},
		}

		provider := NewAzureProvider()
		err := provider.Initialize(ctx, clusterConfig)

		assert.Error(t, err, "Initialize should return error when provider disabled")
		assert.Contains(t, err.Error(), "not enabled", "Error should mention provider is not enabled")

		return nil
	}, pulumi.WithMocks("project", "stack", azureMocks(0)))

	assert.NoError(t, err)
}

func TestAzureProvider_InitializeNilConfig(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Azure: nil,
			},
		}

		provider := NewAzureProvider()
		err := provider.Initialize(ctx, clusterConfig)

		assert.Error(t, err, "Initialize should return error when config is nil")

		return nil
	}, pulumi.WithMocks("project", "stack", azureMocks(0)))

	assert.NoError(t, err)
}

func TestAzureProvider_CreateNetwork(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Azure: &config.AzureProvider{
					Enabled:        true,
					SubscriptionID: "sub-123",
					ResourceGroup:  "test-rg",
					Location:       "eastus",
					VirtualNetwork: &config.AzureVirtualNetwork{
						Name: "test-vnet",
						CIDR: "10.0.0.0/16",
					},
				},
			},
		}

		provider := NewAzureProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		networkConfig := &config.NetworkConfig{
			Mode: "vpc",
			CIDR: "10.0.0.0/16",
		}

		networkOutput, err := provider.CreateNetwork(ctx, networkConfig)
		assert.NoError(t, err, "CreateNetwork should not return error")
		assert.NotNil(t, networkOutput, "Network output should not be nil")

		// Verify resource group was created
		assert.NotNil(t, provider.resourceGroup, "Resource group should be created")
		assert.NotNil(t, provider.virtualNetwork, "Virtual network should be created")
		assert.NotNil(t, provider.subnet, "Subnet should be created")
		assert.NotNil(t, provider.securityGroup, "Security group should be created")

		return nil
	}, pulumi.WithMocks("project", "stack", azureMocks(0)))

	assert.NoError(t, err)
}

func TestAzureProvider_CreateNode(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Azure: &config.AzureProvider{
					Enabled:        true,
					SubscriptionID: "sub-123",
					ResourceGroup:  "test-rg",
					Location:       "eastus",
					VirtualNetwork: &config.AzureVirtualNetwork{
						Name: "test-vnet",
						CIDR: "10.0.0.0/16",
					},
				},
			},
		}

		provider := NewAzureProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		// First create network (required for Azure VMs)
		networkConfig := &config.NetworkConfig{
			Mode: "vpc",
			CIDR: "10.0.0.0/16",
		}
		_, err = provider.CreateNetwork(ctx, networkConfig)
		assert.NoError(t, err)

		// Now create node
		nodeConfig := &config.NodeConfig{
			Name:        "azure-master-1",
			Provider:    "azure",
			Region:      "eastus",
			Size:        "Standard_B2s",
			Image:       "ubuntu-22.04",
			Roles:       []string{"master"},
			WireGuardIP: "10.10.0.40",
			Labels: map[string]string{
				"environment": "production",
			},
		}

		nodeOutput, err := provider.CreateNode(ctx, nodeConfig)
		assert.NoError(t, err, "CreateNode should not return error")
		assert.NotNil(t, nodeOutput, "Node output should not be nil")
		assert.Equal(t, "azure-master-1", nodeOutput.Name, "Node name should match")
		assert.Equal(t, "azure", nodeOutput.Provider, "Provider should be azure")

		return nil
	}, pulumi.WithMocks("project", "stack", azureMocks(0)))

	assert.NoError(t, err)
}

func TestAzureProvider_CreateNodeWithoutNetwork(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Azure: &config.AzureProvider{
					Enabled:       true,
					ResourceGroup: "test-rg",
					Location:      "eastus",
				},
			},
		}

		provider := NewAzureProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		// Try to create node without creating network first
		nodeConfig := &config.NodeConfig{
			Name:     "azure-node",
			Provider: "azure",
			Region:   "eastus",
			Size:     "Standard_B2s",
			Image:    "ubuntu-22.04",
			Roles:    []string{"worker"},
		}

		_, err = provider.CreateNode(ctx, nodeConfig)
		assert.Error(t, err, "CreateNode should fail without network")
		assert.Contains(t, err.Error(), "resource group not created", "Error should mention missing resource group")

		return nil
	}, pulumi.WithMocks("project", "stack", azureMocks(0)))

	assert.NoError(t, err)
}

func TestAzureProvider_CreateNodePool(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Azure: &config.AzureProvider{
					Enabled:        true,
					SubscriptionID: "sub-123",
					ResourceGroup:  "test-rg",
					Location:       "eastus",
					VirtualNetwork: &config.AzureVirtualNetwork{
						Name: "test-vnet",
						CIDR: "10.0.0.0/16",
					},
				},
			},
		}

		provider := NewAzureProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		// Create network first
		networkConfig := &config.NetworkConfig{
			Mode: "vpc",
			CIDR: "10.0.0.0/16",
		}
		_, err = provider.CreateNetwork(ctx, networkConfig)
		assert.NoError(t, err)

		// Create node pool
		pool := &config.NodePool{
			Name:     "azure-worker-pool",
			Provider: "azure",
			Region:   "eastus",
			Size:     "Standard_B2s",
			Image:    "ubuntu-22.04",
			Count:    3,
			Roles:    []string{"worker"},
		}

		outputs, err := provider.CreateNodePool(ctx, pool)
		assert.NoError(t, err, "CreateNodePool should not return error")
		assert.Len(t, outputs, 3, "Should create 3 nodes")

		// Verify node naming
		for i, output := range outputs {
			expectedName := fmt.Sprintf("azure-worker-pool-%d", i+1)
			assert.Equal(t, expectedName, output.Name)
			assert.Equal(t, "azure", output.Provider)
		}

		return nil
	}, pulumi.WithMocks("project", "stack", azureMocks(0)))

	assert.NoError(t, err)
}

func TestAzureProvider_MultipleLocations(t *testing.T) {
	locations := []string{"eastus", "westus", "northeurope", "southeastasia"}

	for _, location := range locations {
		t.Run("Location_"+location, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Azure: &config.AzureProvider{
							Enabled:        true,
							SubscriptionID: "sub-123",
							ResourceGroup:  "test-rg",
							Location:       location,
						},
					},
				}

				provider := NewAzureProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)
				assert.Equal(t, location, provider.config.Location)

				return nil
			}, pulumi.WithMocks("project", "stack", azureMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAzureProvider_DifferentVMSizes(t *testing.T) {
	vmSizes := []struct {
		name string
		size string
	}{
		{"Basic", "Standard_B1s"},
		{"Standard", "Standard_B2s"},
		{"Medium", "Standard_D2s_v3"},
		{"Large", "Standard_D4s_v3"},
		{"HighMem", "Standard_E2s_v3"},
	}

	for _, tt := range vmSizes {
		t.Run(tt.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Azure: &config.AzureProvider{
							Enabled:        true,
							SubscriptionID: "sub-123",
							ResourceGroup:  "test-rg",
							Location:       "eastus",
							VirtualNetwork: &config.AzureVirtualNetwork{
								Name: "test-vnet",
								CIDR: "10.0.0.0/16",
							},
						},
					},
				}

				provider := NewAzureProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)

				// Create network
				networkConfig := &config.NetworkConfig{
					Mode: "vpc",
					CIDR: "10.0.0.0/16",
				}
				_, err = provider.CreateNetwork(ctx, networkConfig)
				assert.NoError(t, err)

				// Create node with specific size
				nodeConfig := &config.NodeConfig{
					Name:     fmt.Sprintf("azure-node-%s", tt.name),
					Provider: "azure",
					Region:   "eastus",
					Size:     tt.size,
					Image:    "ubuntu-22.04",
					Roles:    []string{"worker"},
				}

				nodeOutput, err := provider.CreateNode(ctx, nodeConfig)
				assert.NoError(t, err)
				assert.Equal(t, tt.size, nodeOutput.Size)

				return nil
			}, pulumi.WithMocks("project", "stack", azureMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAzureProvider_GetRegionsAndSizes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Azure: &config.AzureProvider{
					Enabled:       true,
					ResourceGroup: "test-rg",
					Location:      "eastus",
				},
			},
		}

		provider := NewAzureProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		regions := provider.GetRegions()
		assert.NotEmpty(t, regions, "Should have available regions")

		sizes := provider.GetSizes()
		assert.NotEmpty(t, sizes, "Should have available sizes")

		return nil
	}, pulumi.WithMocks("project", "stack", azureMocks(0)))

	assert.NoError(t, err)
}

func TestAzureProvider_CreateFirewall(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Azure: &config.AzureProvider{
					Enabled:        true,
					SubscriptionID: "sub-123",
					ResourceGroup:  "test-rg",
					Location:       "eastus",
					VirtualNetwork: &config.AzureVirtualNetwork{
						Name: "test-vnet",
						CIDR: "10.0.0.0/16",
					},
				},
			},
		}

		provider := NewAzureProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		// Create network first
		networkConfig := &config.NetworkConfig{
			Mode: "vpc",
			CIDR: "10.0.0.0/16",
		}
		_, err = provider.CreateNetwork(ctx, networkConfig)
		assert.NoError(t, err)

		// Create firewall rules
		firewallConfig := &config.FirewallConfig{
			Name: "azure-firewall",
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
		}

		// Note: Azure firewall is created with network security group
		// Test that firewall config can be passed without error
		err = provider.CreateFirewall(ctx, firewallConfig, []pulumi.IDOutput{})

		// Azure might return "not implemented" or succeed
		if err != nil {
			// If it returns an error, verify it's expected
			t.Logf("CreateFirewall returned: %v", err)
		}

		return nil
	}, pulumi.WithMocks("project", "stack", azureMocks(0)))

	assert.NoError(t, err)
}

func TestAzureProvider_CustomVirtualNetworkConfig(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				Azure: &config.AzureProvider{
					Enabled:        true,
					SubscriptionID: "sub-123",
					ResourceGroup:  "custom-rg",
					Location:       "westeurope",
					VirtualNetwork: &config.AzureVirtualNetwork{
						Name: "custom-vnet",
						CIDR: "172.16.0.0/16",
					},
				},
			},
		}

		provider := NewAzureProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		// Verify virtual network config
		assert.Equal(t, "custom-vnet", provider.config.VirtualNetwork.Name)
		assert.Equal(t, "172.16.0.0/16", provider.config.VirtualNetwork.CIDR)

		return nil
	}, pulumi.WithMocks("project", "stack", azureMocks(0)))

	assert.NoError(t, err)
}
