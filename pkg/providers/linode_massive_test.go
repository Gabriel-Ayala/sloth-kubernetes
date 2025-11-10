package providers

import (
	"fmt"
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// Massive test suite for Linode Provider - 100 tests

func TestLinodeProvider_InitializeRegions(t *testing.T) {
	regions := []string{
		"us-east", "us-west", "us-central", "us-southeast",
		"ca-central",
		"eu-west", "eu-central",
		"ap-south", "ap-west", "ap-southeast", "ap-northeast",
		"us-iad", "us-ord", "us-lax", "us-mia",
	}

	for _, region := range regions {
		t.Run("Region_"+region, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Linode: &config.LinodeProvider{
							Enabled:        true,
							Region:         region,
							AuthorizedKeys: []string{"ssh-rsa AAAAB3... test"},
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

func TestLinodeProvider_InstanceTypes(t *testing.T) {
	instanceTypes := []struct {
		name string
		size string
	}{
		{"Nanode", "g6-nanode-1"},
		{"Standard_1GB", "g6-standard-1"},
		{"Standard_2GB", "g6-standard-2"},
		{"Standard_4GB", "g6-standard-4"},
		{"Standard_6GB", "g6-standard-6"},
		{"Standard_8GB", "g6-standard-8"},
		{"Standard_16GB", "g6-standard-16"},
		{"Standard_20GB", "g6-standard-20"},
		{"Standard_24GB", "g6-standard-24"},
		{"Standard_32GB", "g6-standard-32"},
		{"Dedicated_4GB", "g6-dedicated-2"},
		{"Dedicated_8GB", "g6-dedicated-4"},
		{"Dedicated_16GB", "g6-dedicated-8"},
		{"Dedicated_32GB", "g6-dedicated-16"},
		{"HighMem_16GB", "g7-highmem-1"},
		{"HighMem_32GB", "g7-highmem-2"},
		{"HighMem_64GB", "g7-highmem-4"},
		{"HighMem_128GB", "g7-highmem-8"},
		{"HighMem_256GB", "g7-highmem-16"},
		{"Premium_2GB", "g1-gpu-rtx6000-1"},
	}

	for _, tt := range instanceTypes {
		t.Run(tt.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Linode: &config.LinodeProvider{
							Enabled:        true,
							Region:         "us-east",
							AuthorizedKeys: []string{"ssh-rsa AAAAB3... test"},
						},
					},
				}

				provider := NewLinodeProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)

				nodeConfig := &config.NodeConfig{
					Name:        fmt.Sprintf("node-%s", tt.name),
					Provider:    "linode",
					Region:      "us-east",
					Size:        tt.size,
					Image:       "linode/ubuntu22.04",
					Roles:       []string{"worker"},
					WireGuardIP: "10.10.0.1",
				}

				node, err := provider.CreateNode(ctx, nodeConfig)
				assert.NoError(t, err)
				assert.Equal(t, tt.size, node.Size)

				return nil
			}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestLinodeProvider_Images(t *testing.T) {
	images := []string{
		"linode/ubuntu22.04",
		"linode/ubuntu20.04",
		"linode/ubuntu18.04",
		"linode/debian11",
		"linode/debian10",
		"linode/centos-stream9",
		"linode/centos-stream8",
		"linode/almalinux9",
		"linode/almalinux8",
		"linode/rocky9",
		"linode/rocky8",
		"linode/fedora38",
		"linode/fedora37",
		"linode/alpine3.17",
		"linode/alpine3.16",
	}

	for _, image := range images {
		t.Run("Image_"+image, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Linode: &config.LinodeProvider{
							Enabled:        true,
							Region:         "us-east",
							AuthorizedKeys: []string{"ssh-rsa AAAAB3... test"},
						},
					},
				}

				provider := NewLinodeProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)

				nodeConfig := &config.NodeConfig{
					Name:        fmt.Sprintf("node-%s", image),
					Provider:    "linode",
					Region:      "us-east",
					Size:        "g6-standard-2",
					Image:       image,
					Roles:       []string{"worker"},
					WireGuardIP: "10.10.0.1",
				}

				node, err := provider.CreateNode(ctx, nodeConfig)
				assert.NoError(t, err)
				assert.NotNil(t, node)

				return nil
			}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestLinodeProvider_PrivateIPSettings(t *testing.T) {
	testCases := []struct {
		name      string
		privateIP bool
	}{
		{"PrivateIP_Enabled", true},
		{"PrivateIP_Disabled", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Linode: &config.LinodeProvider{
							Enabled:        true,
							Region:         "us-east",
							AuthorizedKeys: []string{"ssh-rsa AAAAB3... test"},
							PrivateIP:      tc.privateIP,
						},
					},
				}

				provider := NewLinodeProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)
				assert.Equal(t, tc.privateIP, provider.config.PrivateIP)

				return nil
			}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestLinodeProvider_NodePoolSizes(t *testing.T) {
	poolSizes := []int{1, 2, 3, 5, 7, 10, 15, 20, 25, 50}

	for _, size := range poolSizes {
		t.Run(fmt.Sprintf("PoolSize_%d", size), func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Linode: &config.LinodeProvider{
							Enabled:        true,
							Region:         "us-east",
							AuthorizedKeys: []string{"ssh-rsa AAAAB3... test"},
						},
					},
				}

				provider := NewLinodeProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)

				pool := &config.NodePool{
					Name:     fmt.Sprintf("pool-%d", size),
					Provider: "linode",
					Region:   "us-east",
					Size:     "g6-standard-2",
					Image:    "linode/ubuntu22.04",
					Count:    size,
					Roles:    []string{"worker"},
				}

				nodes, err := provider.CreateNodePool(ctx, pool)
				assert.NoError(t, err)
				assert.Len(t, nodes, size)

				return nil
			}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestLinodeProvider_AuthorizedKeysCombinations(t *testing.T) {
	keySets := []struct {
		name string
		keys []string
	}{
		{"SingleKey", []string{"ssh-rsa AAAAB3... key1"}},
		{"TwoKeys", []string{"ssh-rsa AAAAB3... key1", "ssh-rsa AAAAB3... key2"}},
		{"ThreeKeys", []string{"ssh-rsa AAAAB3... key1", "ssh-rsa AAAAB3... key2", "ssh-rsa AAAAB3... key3"}},
		{"FiveKeys", []string{"ssh-rsa AAAAB3... key1", "ssh-rsa AAAAB3... key2", "ssh-rsa AAAAB3... key3", "ssh-rsa AAAAB3... key4", "ssh-rsa AAAAB3... key5"}},
	}

	for _, tc := range keySets {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Linode: &config.LinodeProvider{
							Enabled:        true,
							Region:         "us-east",
							AuthorizedKeys: tc.keys,
						},
					},
				}

				provider := NewLinodeProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)
				assert.Equal(t, tc.keys, provider.config.AuthorizedKeys)

				return nil
			}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestLinodeProvider_TagVariations(t *testing.T) {
	tagSets := [][]string{
		{"production"},
		{"staging"},
		{"development"},
		{"test"},
		{"production", "web"},
		{"staging", "api"},
		{"development", "database"},
		{"kubernetes", "master"},
		{"kubernetes", "worker"},
		{"kubernetes", "etcd"},
	}

	for i, tags := range tagSets {
		t.Run(fmt.Sprintf("Tags_%d", i), func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Linode: &config.LinodeProvider{
							Enabled:        true,
							Region:         "us-east",
							AuthorizedKeys: []string{"ssh-rsa AAAAB3... test"},
							Tags:           tags,
						},
					},
				}

				provider := NewLinodeProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)
				assert.Equal(t, tags, provider.config.Tags)

				return nil
			}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestLinodeProvider_NodeRoles(t *testing.T) {
	roleConfigs := []struct {
		name  string
		roles []string
	}{
		{"SingleMaster", []string{"master"}},
		{"SingleWorker", []string{"worker"}},
		{"SingleEtcd", []string{"etcd"}},
		{"MasterEtcd", []string{"master", "etcd"}},
		{"MasterWorkerEtcd", []string{"master", "worker", "etcd"}},
		{"ControlPlane", []string{"control-plane"}},
		{"ControlPlaneEtcd", []string{"control-plane", "etcd"}},
	}

	for _, tc := range roleConfigs {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Linode: &config.LinodeProvider{
							Enabled:        true,
							Region:         "us-east",
							AuthorizedKeys: []string{"ssh-rsa AAAAB3... test"},
						},
					},
				}

				provider := NewLinodeProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)

				nodeConfig := &config.NodeConfig{
					Name:        fmt.Sprintf("node-%s", tc.name),
					Provider:    "linode",
					Region:      "us-east",
					Size:        "g6-standard-2",
					Image:       "linode/ubuntu22.04",
					Roles:       tc.roles,
					WireGuardIP: "10.10.0.1",
				}

				node, err := provider.CreateNode(ctx, nodeConfig)
				assert.NoError(t, err)
				assert.NotNil(t, node)

				return nil
			}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestLinodeProvider_MultiZonePools(t *testing.T) {
	zoneConfigs := []struct {
		name  string
		zones []string
		count int
	}{
		{"TwoZones_4Nodes", []string{"us-east-1a", "us-east-1b"}, 4},
		{"ThreeZones_6Nodes", []string{"us-east-1a", "us-east-1b", "us-east-1c"}, 6},
		{"ThreeZones_9Nodes", []string{"us-east-1a", "us-east-1b", "us-east-1c"}, 9},
		{"FourZones_8Nodes", []string{"us-east-1a", "us-east-1b", "us-east-1c", "us-east-1d"}, 8},
	}

	for _, tc := range zoneConfigs {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Linode: &config.LinodeProvider{
							Enabled:        true,
							Region:         "us-east",
							AuthorizedKeys: []string{"ssh-rsa AAAAB3... test"},
						},
					},
				}

				provider := NewLinodeProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)

				pool := &config.NodePool{
					Name:     tc.name,
					Provider: "linode",
					Region:   "us-east",
					Zones:    tc.zones,
					Size:     "g6-standard-2",
					Count:    tc.count,
					Roles:    []string{"worker"},
				}

				nodes, err := provider.CreateNodePool(ctx, pool)
				assert.NoError(t, err)
				assert.Len(t, nodes, tc.count)

				return nil
			}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestLinodeProvider_UserDataVariations(t *testing.T) {
	userDataScripts := []string{
		"#!/bin/bash\necho 'test1'",
		"#!/bin/bash\napt-get update",
		"#!/bin/bash\nyum update -y",
		"#!/bin/bash\nsystemctl enable docker",
		"#!/bin/bash\nkubeadm init",
	}

	for i, userData := range userDataScripts {
		t.Run(fmt.Sprintf("UserData_%d", i), func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						Linode: &config.LinodeProvider{
							Enabled:        true,
							Region:         "us-east",
							AuthorizedKeys: []string{"ssh-rsa AAAAB3... test"},
						},
					},
				}

				provider := NewLinodeProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)

				nodeConfig := &config.NodeConfig{
					Name:        fmt.Sprintf("node-userdata-%d", i),
					Provider:    "linode",
					Region:      "us-east",
					Size:        "g6-standard-2",
					Image:       "linode/ubuntu22.04",
					Roles:       []string{"worker"},
					UserData:    userData,
					WireGuardIP: "10.10.0.1",
				}

				node, err := provider.CreateNode(ctx, nodeConfig)
				assert.NoError(t, err)
				assert.NotNil(t, node)

				return nil
			}, pulumi.WithMocks("project", "stack", linodeMocks(0)))

			assert.NoError(t, err)
		})
	}
}
