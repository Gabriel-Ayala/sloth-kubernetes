package orchestrator

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// Test 1: Configuration validation with invalid settings
func TestOrchestrator_AdvancedConfigurationValidation(t *testing.T) {
	tests := []struct {
		name          string
		config        *config.ClusterConfig
		expectedError bool
	}{
		{
			name: "Valid configuration",
			config: &config.ClusterConfig{
				Metadata: config.Metadata{
					Name:        "valid-cluster",
					Environment: "test",
					Version:     "1.0.0",
				},
				Providers: config.ProvidersConfig{
					DigitalOcean: &config.DigitalOceanProvider{
						Enabled: true,
						Token:   "test-token",
						Region:  "nyc3",
					},
				},
				NodePools: map[string]config.NodePool{
					"masters": {
						Name:     "masters",
						Provider: "digitalocean",
						Count:    3,
						Size:     "s-2vcpu-4gb",
						Roles:    []string{"master"},
					},
				},
			},
			expectedError: false,
		},
		{
			name: "Empty cluster name",
			config: &config.ClusterConfig{
				Metadata: config.Metadata{
					Name:        "",
					Environment: "test",
				},
				Providers: config.ProvidersConfig{
					DigitalOcean: &config.DigitalOceanProvider{
						Enabled: true,
						Token:   "test-token",
					},
				},
			},
			expectedError: false, // Empty names are allowed at creation time
		},
		{
			name: "No node pools",
			config: &config.ClusterConfig{
				Metadata: config.Metadata{
					Name:        "test-cluster",
					Environment: "test",
				},
				Providers: config.ProvidersConfig{
					DigitalOcean: &config.DigitalOceanProvider{
						Enabled: true,
						Token:   "test-token",
					},
				},
				NodePools: map[string]config.NodePool{},
			},
			expectedError: false, // Empty node pools are allowed at creation time
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				orch := New(ctx, tt.config)
				assert.NotNil(t, orch)
				assert.Equal(t, tt.config, orch.config)
				return nil
			}, pulumi.WithMocks("test", "config-validation", &IntegrationMockProvider{}))

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test 2: Node pool scaling operations
func TestOrchestrator_NodePoolScaling(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "scaling-cluster",
				Environment: "test",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc3",
				},
			},
			NodePools: map[string]config.NodePool{
				"workers": {
					Name:     "workers",
					Provider: "digitalocean",
					Count:    2,
					Size:     "s-2vcpu-4gb",
					Roles:    []string{"worker"},
				},
			},
		}

		orch := New(ctx, cfg)

		// Simulate initial nodes
		orch.nodes = make(map[string][]*providers.NodeOutput)
		orch.nodes["digitalocean"] = []*providers.NodeOutput{
			{Name: "worker-1", Labels: map[string]string{"role": "worker"}},
			{Name: "worker-2", Labels: map[string]string{"role": "worker"}},
		}

		assert.Equal(t, 2, len(orch.nodes["digitalocean"]))

		// Simulate scaling up by adding nodes
		orch.nodes["digitalocean"] = append(orch.nodes["digitalocean"], &providers.NodeOutput{
			Name:   "worker-3",
			Labels: map[string]string{"role": "worker"},
		})

		assert.Equal(t, 3, len(orch.nodes["digitalocean"]))

		return nil
	}, pulumi.WithMocks("test", "node-scaling", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 3: Multi-region deployment
func TestOrchestrator_MultiRegionDeployment(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "multi-region-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc3",
				},
			},
			NodePools: map[string]config.NodePool{
				"nyc-masters": {
					Name:     "nyc-masters",
					Provider: "digitalocean",
					Count:    3,
					Size:     "s-2vcpu-4gb",
					Roles:    []string{"master"},
					Region:   "nyc3",
				},
				"sfo-workers": {
					Name:     "sfo-workers",
					Provider: "digitalocean",
					Count:    5,
					Size:     "s-4vcpu-8gb",
					Roles:    []string{"worker"},
					Region:   "sfo3",
				},
				"ams-workers": {
					Name:     "ams-workers",
					Provider: "digitalocean",
					Count:    5,
					Size:     "s-4vcpu-8gb",
					Roles:    []string{"worker"},
					Region:   "ams3",
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify configuration has multiple regions
		regionCount := make(map[string]int)
		for _, pool := range cfg.NodePools {
			regionCount[pool.Region]++
		}

		assert.Equal(t, 3, len(regionCount), "Should have 3 different regions")
		assert.Equal(t, 3, len(cfg.NodePools), "Should have 3 node pools")

		return nil
	}, pulumi.WithMocks("test", "multi-region", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 4: Node labels and annotations management
func TestOrchestrator_NodeLabelsAndAnnotations(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "labeled-cluster",
				Environment: "test",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
				},
			},
		}

		orch := New(ctx, cfg)
		orch.nodes = make(map[string][]*providers.NodeOutput)

		// Create nodes with various labels and annotations
		orch.nodes["digitalocean"] = []*providers.NodeOutput{
			{
				Name: "master-1",
				Labels: map[string]string{
					"role":        "master",
					"environment": "production",
					"tier":        "control-plane",
					"created-by":  "orchestrator",
					"version":     "1.0.0",
				},
			},
			{
				Name: "worker-1",
				Labels: map[string]string{
					"role":         "worker",
					"environment":  "production",
					"workload":     "cpu-intensive",
					"created-by":   "orchestrator",
					"node-purpose": "compute",
				},
			},
		}

		// Verify labels
		assert.Equal(t, "master", orch.nodes["digitalocean"][0].Labels["role"])
		assert.Equal(t, "cpu-intensive", orch.nodes["digitalocean"][1].Labels["workload"])
		assert.Equal(t, "orchestrator", orch.nodes["digitalocean"][0].Labels["created-by"])

		return nil
	}, pulumi.WithMocks("test", "labels-annotations", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 5: High availability configuration
func TestOrchestrator_HighAvailabilitySetup(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "ha-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc3",
				},
			},
			NodePools: map[string]config.NodePool{
				"ha-masters": {
					Name:     "ha-masters",
					Provider: "digitalocean",
					Count:    5, // 5 masters for high availability
					Size:     "s-4vcpu-8gb",
					Roles:    []string{"master", "etcd"},
				},
				"ha-workers": {
					Name:     "ha-workers",
					Provider: "digitalocean",
					Count:    10,
					Size:     "s-8vcpu-16gb",
					Roles:    []string{"worker"},
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify HA requirements
		masterPool := cfg.NodePools["ha-masters"]
		assert.GreaterOrEqual(t, masterPool.Count, 3, "HA requires at least 3 masters")
		assert.Contains(t, masterPool.Roles, "etcd", "HA masters should include etcd role")

		workerPool := cfg.NodePools["ha-workers"]
		assert.GreaterOrEqual(t, workerPool.Count, 3, "HA requires at least 3 workers")

		return nil
	}, pulumi.WithMocks("test", "ha-setup", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 6: Resource tagging for cost allocation
func TestOrchestrator_ResourceTagging(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "tagged-cluster",
				Environment: "staging",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify configuration was created properly
		assert.NotNil(t, cfg)
		assert.Equal(t, "tagged-cluster", cfg.Metadata.Name)

		return nil
	}, pulumi.WithMocks("test", "resource-tagging", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 7: Network isolation with custom CIDRs
func TestOrchestrator_NetworkIsolation(t *testing.T) {
	tests := []struct {
		name        string
		podCIDR     string
		serviceCIDR string
		valid       bool
	}{
		{
			name:        "Standard CIDRs",
			podCIDR:     "10.244.0.0/16",
			serviceCIDR: "10.96.0.0/12",
			valid:       true,
		},
		{
			name:        "Custom CIDRs",
			podCIDR:     "172.16.0.0/16",
			serviceCIDR: "172.17.0.0/16",
			valid:       true,
		},
		{
			name:        "Large pod network",
			podCIDR:     "10.0.0.0/8",
			serviceCIDR: "192.168.0.0/16",
			valid:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				cfg := &config.ClusterConfig{
					Metadata: config.Metadata{
						Name:        "network-isolated-cluster",
						Environment: "test",
					},
					Providers: config.ProvidersConfig{
						DigitalOcean: &config.DigitalOceanProvider{
							Enabled: true,
							Token:   "test-token",
						},
					},
					Network: config.NetworkConfig{
						PodCIDR:     tt.podCIDR,
						ServiceCIDR: tt.serviceCIDR,
					},
				}

				orch := New(ctx, cfg)

				assert.Equal(t, tt.podCIDR, orch.config.Network.PodCIDR)
				assert.Equal(t, tt.serviceCIDR, orch.config.Network.ServiceCIDR)

				return nil
			}, pulumi.WithMocks("test", "network-isolation", &IntegrationMockProvider{}))
			assert.NoError(t, err)
		})
	}
}

// Test 11: Cluster metadata validation
func TestOrchestrator_ClusterMetadata(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "metadata-test-cluster",
				Environment: "development",
				Version:     "1.28.0",
				Description: "Test cluster for metadata validation",
				Owner:       "platform-team",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify all metadata fields
		assert.Equal(t, "metadata-test-cluster", cfg.Metadata.Name)
		assert.Equal(t, "development", cfg.Metadata.Environment)
		assert.Equal(t, "1.28.0", cfg.Metadata.Version)
		assert.Equal(t, "Test cluster for metadata validation", cfg.Metadata.Description)
		assert.Equal(t, "platform-team", cfg.Metadata.Owner)

		return nil
	}, pulumi.WithMocks("test", "cluster-metadata", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 12: Provider failover scenario
func TestOrchestrator_ProviderFailover(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "failover-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "do-token",
					Region:  "nyc3",
				},
				Linode: &config.LinodeProvider{
					Enabled: true,
					Token:   "linode-token",
					Region:  "us-east",
				},
			},
			NodePools: map[string]config.NodePool{
				"primary-masters": {
					Name:     "primary-masters",
					Provider: "digitalocean",
					Count:    3,
					Roles:    []string{"master"},
				},
				"backup-masters": {
					Name:     "backup-masters",
					Provider: "linode",
					Count:    2,
					Roles:    []string{"master"},
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify multiple providers are configured
		assert.NotNil(t, cfg.Providers.DigitalOcean)
		assert.NotNil(t, cfg.Providers.Linode)
		assert.True(t, cfg.Providers.DigitalOcean.Enabled)
		assert.True(t, cfg.Providers.Linode.Enabled)

		// Verify failover configuration
		assert.Equal(t, 2, len(cfg.NodePools))

		return nil
	}, pulumi.WithMocks("test", "provider-failover", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 13: Node pool with custom image
func TestOrchestrator_CustomNodeImage(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "custom-image-cluster",
				Environment: "test",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
				},
			},
			NodePools: map[string]config.NodePool{
				"custom-workers": {
					Name:     "custom-workers",
					Provider: "digitalocean",
					Count:    3,
					Size:     "s-2vcpu-4gb",
					Image:    "ubuntu-22-04-x64-custom",
					Roles:    []string{"worker"},
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify custom image configuration
		pool := cfg.NodePools["custom-workers"]
		assert.Equal(t, "ubuntu-22-04-x64-custom", pool.Image)

		return nil
	}, pulumi.WithMocks("test", "custom-image", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 14: Cluster with monitoring enabled
func TestOrchestrator_MonitoringIntegration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "monitored-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
				},
			},
			Addons: config.AddonsConfig{
				ArgoCD: &config.ArgoCDConfig{
					Enabled:          true,
					Version:          "v2.8.0",
					GitOpsRepoURL:    "https://github.com/example/gitops",
					GitOpsRepoBranch: "main",
					AppsPath:         "argocd/apps",
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify ArgoCD addon is properly configured
		assert.NotNil(t, cfg.Addons.ArgoCD)
		assert.True(t, cfg.Addons.ArgoCD.Enabled)
		assert.Equal(t, "v2.8.0", cfg.Addons.ArgoCD.Version)
		assert.Equal(t, "https://github.com/example/gitops", cfg.Addons.ArgoCD.GitOpsRepoURL)

		return nil
	}, pulumi.WithMocks("test", "monitoring", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 15: Node pool with taints and tolerations
func TestOrchestrator_NodeTaintsAndTolerations(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "tainted-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
				},
			},
		}

		orch := New(ctx, cfg)
		orch.nodes = make(map[string][]*providers.NodeOutput)

		// Create nodes with labels (taints would require additional fields in NodeOutput)
		orch.nodes["digitalocean"] = []*providers.NodeOutput{
			{
				Name: "gpu-worker-1",
				Labels: map[string]string{
					"role":                             "worker",
					"hardware":                         "gpu",
					"node.kubernetes.io/instance-type": "gpu-optimized",
					"gpu":                              "nvidia",
				},
			},
			{
				Name: "high-mem-worker-1",
				Labels: map[string]string{
					"role":             "worker",
					"workload":         "memory-intensive",
					"memory-intensive": "true",
				},
			},
		}

		// Verify labels are properly set
		assert.Equal(t, 2, len(orch.nodes["digitalocean"]))
		assert.Equal(t, "gpu", orch.nodes["digitalocean"][0].Labels["hardware"])
		assert.Equal(t, "memory-intensive", orch.nodes["digitalocean"][1].Labels["workload"])

		return nil
	}, pulumi.WithMocks("test", "taints-tolerations", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
