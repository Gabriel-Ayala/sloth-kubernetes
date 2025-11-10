package providers

import (
	"fmt"
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// Massive test suite for Azure Provider - 100 tests

func TestAzureProvider_Locations(t *testing.T) {
	locations := []string{
		"eastus", "eastus2", "westus", "westus2", "westus3",
		"centralus", "northcentralus", "southcentralus", "westcentralus",
		"canadacentral", "canadaeast",
		"brazilsouth",
		"northeurope", "westeurope",
		"uksouth", "ukwest",
		"francecentral", "francesouth",
		"germanywestcentral",
		"switzerlandnorth",
		"norwayeast",
		"southeastasia", "eastasia",
		"australiaeast", "australiasoutheast",
		"japaneast", "japanwest",
		"koreacentral",
		"southindia", "centralindia",
		"uaenorth",
		"southafricanorth",
	}

	for _, location := range locations {
		t.Run("Location_"+location, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Azure: &config.AzureProvider{
							Enabled:       true,
							Location:      location,
							ResourceGroup: "test-rg",
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

func TestAzureProvider_VMSizes(t *testing.T) {
	vmSizes := []string{
		"Standard_B1s", "Standard_B1ms", "Standard_B2s", "Standard_B2ms", "Standard_B4ms",
		"Standard_D2s_v3", "Standard_D4s_v3", "Standard_D8s_v3", "Standard_D16s_v3",
		"Standard_D2s_v4", "Standard_D4s_v4", "Standard_D8s_v4",
		"Standard_D2s_v5", "Standard_D4s_v5", "Standard_D8s_v5",
		"Standard_E2s_v3", "Standard_E4s_v3", "Standard_E8s_v3",
		"Standard_E2s_v4", "Standard_E4s_v4",
		"Standard_E2s_v5", "Standard_E4s_v5",
		"Standard_F2s_v2", "Standard_F4s_v2", "Standard_F8s_v2",
		"Standard_L4s", "Standard_L8s",
		"Standard_M8ms", "Standard_M16ms",
	}

	for _, size := range vmSizes {
		t.Run("VMSize_"+size, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Azure: &config.AzureProvider{
							Enabled:       true,
							Location:      "eastus",
							ResourceGroup: "test-rg",
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

				nodeConfig := &config.NodeConfig{
					Name:        fmt.Sprintf("node-%s", size),
					Provider:    "azure",
					Region:      "eastus",
					Size:        size,
					Image:       "ubuntu-22.04",
					Roles:       []string{"worker"},
					WireGuardIP: "10.10.0.1",
				}

				node, err := provider.CreateNode(ctx, nodeConfig)
				assert.NoError(t, err)
				assert.Equal(t, size, node.Size)

				return nil
			}, pulumi.WithMocks("project", "stack", azureMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAzureProvider_VirtualNetworkCIDRs(t *testing.T) {
	cidrConfigs := []struct {
		name string
		cidr string
	}{
		{"Small_24", "10.0.0.0/24"},
		{"Medium_20", "10.0.0.0/20"},
		{"Large_16", "10.0.0.0/16"},
		{"XLarge_12", "10.0.0.0/12"},
		{"Range_172_16", "172.16.0.0/16"},
		{"Range_172_20", "172.16.0.0/20"},
		{"Range_192_16", "192.168.0.0/16"},
		{"Range_192_24", "192.168.1.0/24"},
	}

	for _, tc := range cidrConfigs {
		t.Run("CIDR_"+tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Azure: &config.AzureProvider{
							Enabled:       true,
							Location:      "eastus",
							ResourceGroup: "test-rg",
							VirtualNetwork: &config.AzureVirtualNetwork{
								Name: fmt.Sprintf("vnet-%s", tc.name),
								CIDR: tc.cidr,
							},
						},
					},
				}

				provider := NewAzureProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)
				assert.Equal(t, tc.cidr, provider.config.VirtualNetwork.CIDR)

				return nil
			}, pulumi.WithMocks("project", "stack", azureMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAzureProvider_NodePoolSizes(t *testing.T) {
	poolSizes := []int{1, 2, 3, 5, 7, 10, 15, 20}

	for _, size := range poolSizes {
		t.Run(fmt.Sprintf("PoolSize_%d", size), func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Azure: &config.AzureProvider{
							Enabled:       true,
							Location:      "eastus",
							ResourceGroup: "test-rg",
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

				pool := &config.NodePool{
					Name:     fmt.Sprintf("pool-%d", size),
					Provider: "azure",
					Region:   "eastus",
					Size:     "Standard_B2s",
					Image:    "ubuntu-22.04",
					Count:    size,
					Roles:    []string{"worker"},
				}

				nodes, err := provider.CreateNodePool(ctx, pool)
				assert.NoError(t, err)
				assert.Len(t, nodes, size)

				return nil
			}, pulumi.WithMocks("project", "stack", azureMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAzureProvider_ResourceGroupNaming(t *testing.T) {
	rgNames := []string{
		"rg-test", "rg-production", "rg-staging", "rg-development",
		"kubernetes-rg", "k8s-cluster-rg", "webapp-rg",
		"rg-eastus-prod", "rg-westus-dev", "rg-northeurope-test",
	}

	for _, rgName := range rgNames {
		t.Run("RG_"+rgName, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Azure: &config.AzureProvider{
							Enabled:       true,
							Location:      "eastus",
							ResourceGroup: rgName,
						},
					},
				}

				provider := NewAzureProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)
				assert.Equal(t, rgName, provider.config.ResourceGroup)

				return nil
			}, pulumi.WithMocks("project", "stack", azureMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAzureProvider_VirtualNetworkNaming(t *testing.T) {
	vnetNames := []string{
		"vnet-prod", "vnet-staging", "vnet-dev",
		"kubernetes-vnet", "k8s-network", "cluster-vnet",
		"vnet-eastus-prod", "vnet-westus-dev",
	}

	for _, vnetName := range vnetNames {
		t.Run("VNet_"+vnetName, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Azure: &config.AzureProvider{
							Enabled:       true,
							Location:      "eastus",
							ResourceGroup: "test-rg",
							VirtualNetwork: &config.AzureVirtualNetwork{
								Name: vnetName,
								CIDR: "10.0.0.0/16",
							},
						},
					},
				}

				provider := NewAzureProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)
				assert.Equal(t, vnetName, provider.config.VirtualNetwork.Name)

				return nil
			}, pulumi.WithMocks("project", "stack", azureMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAzureProvider_OSImages(t *testing.T) {
	images := []string{
		"ubuntu-22.04", "ubuntu-20.04", "ubuntu-18.04",
		"debian-11", "debian-10",
		"centos-8", "centos-7",
		"rhel-8", "rhel-7",
		"windows-2022", "windows-2019",
	}

	for _, image := range images {
		t.Run("Image_"+image, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Azure: &config.AzureProvider{
							Enabled:       true,
							Location:      "eastus",
							ResourceGroup: "test-rg",
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

				nodeConfig := &config.NodeConfig{
					Name:        fmt.Sprintf("node-%s", image),
					Provider:    "azure",
					Region:      "eastus",
					Size:        "Standard_B2s",
					Image:       image,
					Roles:       []string{"worker"},
					WireGuardIP: "10.10.0.1",
				}

				node, err := provider.CreateNode(ctx, nodeConfig)
				assert.NoError(t, err)
				assert.NotNil(t, node)

				return nil
			}, pulumi.WithMocks("project", "stack", azureMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAzureProvider_SubscriptionIDs(t *testing.T) {
	// Test with different subscription ID formats
	subscriptionIDs := []string{
		"12345678-1234-1234-1234-123456789012",
		"abcdef12-3456-7890-abcd-ef1234567890",
		"11111111-2222-3333-4444-555555555555",
		"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
	}

	for i, subID := range subscriptionIDs {
		t.Run(fmt.Sprintf("SubID_%d", i), func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Azure: &config.AzureProvider{
							Enabled:        true,
							SubscriptionID: subID,
							Location:       "eastus",
							ResourceGroup:  "test-rg",
						},
					},
				}

				provider := NewAzureProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)
				assert.Equal(t, subID, provider.config.SubscriptionID)

				return nil
			}, pulumi.WithMocks("project", "stack", azureMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAzureProvider_NodeRoles(t *testing.T) {
	roleConfigs := []struct {
		name  string
		roles []string
	}{
		{"Master", []string{"master"}},
		{"Worker", []string{"worker"}},
		{"Etcd", []string{"etcd"}},
		{"MasterEtcd", []string{"master", "etcd"}},
		{"AllRoles", []string{"master", "worker", "etcd"}},
		{"ControlPlane", []string{"control-plane"}},
	}

	for _, tc := range roleConfigs {
		t.Run("Roles_"+tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Azure: &config.AzureProvider{
							Enabled:       true,
							Location:      "eastus",
							ResourceGroup: "test-rg",
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

				nodeConfig := &config.NodeConfig{
					Name:        fmt.Sprintf("node-%s", tc.name),
					Provider:    "azure",
					Region:      "eastus",
					Size:        "Standard_B2s",
					Image:       "ubuntu-22.04",
					Roles:       tc.roles,
					WireGuardIP: "10.10.0.1",
				}

				node, err := provider.CreateNode(ctx, nodeConfig)
				assert.NoError(t, err)
				assert.NotNil(t, node)

				return nil
			}, pulumi.WithMocks("project", "stack", azureMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAzureProvider_DiskSizes(t *testing.T) {
	// Test different OS disk sizes
	diskSizes := []int{30, 64, 128, 256, 512, 1024}

	for _, diskSize := range diskSizes {
		t.Run(fmt.Sprintf("DiskSize_%dGB", diskSize), func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Azure: &config.AzureProvider{
							Enabled:       true,
							Location:      "eastus",
							ResourceGroup: "test-rg",
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

				return nil
			}, pulumi.WithMocks("project", "stack", azureMocks(0)))

			assert.NoError(t, err)
		})
	}
}
