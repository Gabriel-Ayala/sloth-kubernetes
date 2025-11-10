package providers

import (
	"fmt"
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// Massive test suite for DigitalOcean Provider - 100 tests

func TestDigitalOceanProvider_InitializeVariations(t *testing.T) {
	testCases := []struct {
		name        string
		config      *config.DigitalOceanProvider
		shouldError bool
		errorMsg    string
	}{
		{"Valid_NYC1", &config.DigitalOceanProvider{Enabled: true, Region: "nyc1", SSHKeys: []string{"key1"}}, false, ""},
		{"Valid_NYC3", &config.DigitalOceanProvider{Enabled: true, Region: "nyc3", SSHKeys: []string{"key1"}}, false, ""},
		{"Valid_SFO1", &config.DigitalOceanProvider{Enabled: true, Region: "sfo1", SSHKeys: []string{"key1"}}, false, ""},
		{"Valid_SFO2", &config.DigitalOceanProvider{Enabled: true, Region: "sfo2", SSHKeys: []string{"key1"}}, false, ""},
		{"Valid_SFO3", &config.DigitalOceanProvider{Enabled: true, Region: "sfo3", SSHKeys: []string{"key1"}}, false, ""},
		{"Valid_AMS2", &config.DigitalOceanProvider{Enabled: true, Region: "ams2", SSHKeys: []string{"key1"}}, false, ""},
		{"Valid_AMS3", &config.DigitalOceanProvider{Enabled: true, Region: "ams3", SSHKeys: []string{"key1"}}, false, ""},
		{"Valid_SGP1", &config.DigitalOceanProvider{Enabled: true, Region: "sgp1", SSHKeys: []string{"key1"}}, false, ""},
		{"Valid_LON1", &config.DigitalOceanProvider{Enabled: true, Region: "lon1", SSHKeys: []string{"key1"}}, false, ""},
		{"Valid_FRA1", &config.DigitalOceanProvider{Enabled: true, Region: "fra1", SSHKeys: []string{"key1"}}, false, ""},
		{"Valid_TOR1", &config.DigitalOceanProvider{Enabled: true, Region: "tor1", SSHKeys: []string{"key1"}}, false, ""},
		{"Valid_BLR1", &config.DigitalOceanProvider{Enabled: true, Region: "blr1", SSHKeys: []string{"key1"}}, false, ""},
		{"Disabled", &config.DigitalOceanProvider{Enabled: false}, true, "not enabled"},
		{"NoSSHKeys", &config.DigitalOceanProvider{Enabled: true, Region: "nyc1"}, true, "no SSH keys"},
		{"MultipleKeys", &config.DigitalOceanProvider{Enabled: true, Region: "nyc1", SSHKeys: []string{"key1", "key2", "key3"}}, false, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						DigitalOcean: tc.config,
					},
				}

				provider := NewDigitalOceanProvider()
				err := provider.Initialize(ctx, clusterConfig)

				if tc.shouldError {
					assert.Error(t, err)
					if tc.errorMsg != "" {
						assert.Contains(t, err.Error(), tc.errorMsg)
					}
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tc.config.Region, provider.config.Region)
				}

				return nil
			}, pulumi.WithMocks("project", "stack", mocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestDigitalOceanProvider_NodeSizes(t *testing.T) {
	sizes := []string{
		"s-1vcpu-512mb-10gb", "s-1vcpu-1gb", "s-1vcpu-2gb", "s-1vcpu-3gb",
		"s-2vcpu-2gb", "s-2vcpu-4gb", "s-2vcpu-8gb",
		"s-4vcpu-8gb", "s-4vcpu-16gb",
		"s-6vcpu-16gb",
		"s-8vcpu-32gb",
		"s-12vcpu-48gb",
		"s-16vcpu-64gb",
		"s-20vcpu-96gb",
		"s-24vcpu-128gb",
		"s-32vcpu-192gb",
		"c-2", "c-4", "c-8", "c-16", "c-32", "c-48",
		"g-2vcpu-8gb", "g-4vcpu-16gb", "g-8vcpu-32gb",
		"m-2vcpu-16gb", "m-4vcpu-32gb", "m-8vcpu-64gb",
	}

	for _, size := range sizes {
		t.Run("Size_"+size, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						DigitalOcean: &config.DigitalOceanProvider{
							Enabled: true,
							Region:  "nyc1",
							SSHKeys: []string{"key1"},
						},
					},
				}

				provider := NewDigitalOceanProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)

				nodeConfig := &config.NodeConfig{
					Name:        fmt.Sprintf("node-%s", size),
					Provider:    "digitalocean",
					Region:      "nyc1",
					Size:        size,
					Image:       "ubuntu-22-04-x64",
					Roles:       []string{"worker"},
					WireGuardIP: "10.10.0.1",
				}

				node, err := provider.CreateNode(ctx, nodeConfig)
				assert.NoError(t, err)
				assert.Equal(t, size, node.Size)

				return nil
			}, pulumi.WithMocks("project", "stack", mocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestDigitalOceanProvider_NodeImages(t *testing.T) {
	images := []string{
		"ubuntu-22-04-x64", "ubuntu-20-04-x64", "ubuntu-18-04-x64",
		"debian-11-x64", "debian-10-x64",
		"centos-stream-9-x64", "centos-stream-8-x64",
		"rocky-9-x64", "rocky-8-x64",
		"fedora-38-x64", "fedora-37-x64",
		"almalinux-9-x64", "almalinux-8-x64",
	}

	for _, image := range images {
		t.Run("Image_"+image, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						DigitalOcean: &config.DigitalOceanProvider{
							Enabled: true,
							Region:  "nyc1",
							SSHKeys: []string{"key1"},
						},
					},
				}

				provider := NewDigitalOceanProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)

				nodeConfig := &config.NodeConfig{
					Name:        fmt.Sprintf("node-%s", image),
					Provider:    "digitalocean",
					Region:      "nyc1",
					Size:        "s-1vcpu-1gb",
					Image:       image,
					Roles:       []string{"worker"},
					WireGuardIP: "10.10.0.1",
				}

				node, err := provider.CreateNode(ctx, nodeConfig)
				assert.NoError(t, err)
				assert.NotNil(t, node)

				return nil
			}, pulumi.WithMocks("project", "stack", mocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestDigitalOceanProvider_NodeRoleCombinations(t *testing.T) {
	roleCombinations := []struct {
		name  string
		roles []string
	}{
		{"Master", []string{"master"}},
		{"Worker", []string{"worker"}},
		{"Etcd", []string{"etcd"}},
		{"MasterEtcd", []string{"master", "etcd"}},
		{"MasterWorker", []string{"master", "worker"}},
		{"All", []string{"master", "worker", "etcd"}},
		{"ControlPlane", []string{"control-plane"}},
		{"ControlPlaneEtcd", []string{"control-plane", "etcd"}},
	}

	for _, tc := range roleCombinations {
		t.Run("Roles_"+tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						DigitalOcean: &config.DigitalOceanProvider{
							Enabled: true,
							Region:  "nyc1",
							SSHKeys: []string{"key1"},
						},
					},
				}

				provider := NewDigitalOceanProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)

				nodeConfig := &config.NodeConfig{
					Name:        fmt.Sprintf("node-%s", tc.name),
					Provider:    "digitalocean",
					Region:      "nyc1",
					Size:        "s-2vcpu-4gb",
					Image:       "ubuntu-22-04-x64",
					Roles:       tc.roles,
					WireGuardIP: "10.10.0.1",
				}

				node, err := provider.CreateNode(ctx, nodeConfig)
				assert.NoError(t, err)
				assert.NotNil(t, node)

				return nil
			}, pulumi.WithMocks("project", "stack", mocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestDigitalOceanProvider_NodeLabels(t *testing.T) {
	labelSets := []map[string]string{
		{"environment": "production"},
		{"environment": "staging"},
		{"environment": "development"},
		{"tier": "frontend"},
		{"tier": "backend"},
		{"tier": "database"},
		{"app": "nginx"},
		{"app": "postgres"},
		{"team": "devops"},
		{"cost-center": "engineering"},
		{"environment": "production", "tier": "frontend", "app": "web"},
		{"environment": "staging", "tier": "backend", "version": "v1.0"},
	}

	for i, labels := range labelSets {
		t.Run(fmt.Sprintf("Labels_%d", i), func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						DigitalOcean: &config.DigitalOceanProvider{
							Enabled: true,
							Region:  "nyc1",
							SSHKeys: []string{"key1"},
						},
					},
				}

				provider := NewDigitalOceanProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)

				nodeConfig := &config.NodeConfig{
					Name:        fmt.Sprintf("node-labels-%d", i),
					Provider:    "digitalocean",
					Region:      "nyc1",
					Size:        "s-1vcpu-1gb",
					Image:       "ubuntu-22-04-x64",
					Roles:       []string{"worker"},
					Labels:      labels,
					WireGuardIP: "10.10.0.1",
				}

				node, err := provider.CreateNode(ctx, nodeConfig)
				assert.NoError(t, err)
				assert.Equal(t, labels, node.Labels)

				return nil
			}, pulumi.WithMocks("project", "stack", mocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestDigitalOceanProvider_VPCConfigurations(t *testing.T) {
	vpcConfigs := []struct {
		name    string
		vpcCIDR string
	}{
		{"Small_24", "10.0.0.0/24"},
		{"Medium_20", "10.0.0.0/20"},
		{"Large_16", "10.0.0.0/16"},
		{"XLarge_12", "10.0.0.0/12"},
		{"Range_172", "172.16.0.0/16"},
		{"Range_192", "192.168.0.0/16"},
	}

	for _, tc := range vpcConfigs {
		t.Run("VPC_"+tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						DigitalOcean: &config.DigitalOceanProvider{
							Enabled: true,
							Region:  "nyc1",
							SSHKeys: []string{"key1"},
							VPC: &config.VPCConfig{
								Name: fmt.Sprintf("vpc-%s", tc.name),
								CIDR: tc.vpcCIDR,
							},
						},
					},
				}

				provider := NewDigitalOceanProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)
				assert.Equal(t, tc.vpcCIDR, provider.config.VPC.CIDR)

				return nil
			}, pulumi.WithMocks("project", "stack", mocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestDigitalOceanProvider_TagCombinations(t *testing.T) {
	tagSets := [][]string{
		{"production"},
		{"staging"},
		{"development"},
		{"production", "web"},
		{"staging", "api"},
		{"development", "testing"},
		{"kubernetes", "master"},
		{"kubernetes", "worker"},
		{"kubernetes", "etcd"},
		{"production", "kubernetes", "web-tier"},
	}

	for i, tags := range tagSets {
		t.Run(fmt.Sprintf("Tags_%d", i), func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						DigitalOcean: &config.DigitalOceanProvider{
							Enabled: true,
							Region:  "nyc1",
							SSHKeys: []string{"key1"},
							Tags:    tags,
						},
					},
				}

				provider := NewDigitalOceanProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)
				assert.Equal(t, tags, provider.config.Tags)

				return nil
			}, pulumi.WithMocks("project", "stack", mocks(0)))

			assert.NoError(t, err)
		})
	}
}
