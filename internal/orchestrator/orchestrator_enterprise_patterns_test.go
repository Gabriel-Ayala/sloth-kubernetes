package orchestrator

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// TestOrchestrator_MicroservicesArchitecture tests a microservices platform with API gateway
func TestOrchestrator_MicroservicesArchitecture(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "microservices-platform",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc1",
				},
			},
			NodePools: map[string]config.NodePool{
				"api-gateway": {
					Name:     "api-gateway",
					Provider: "digitalocean",
					Count:    5,
					Size:     "s-4vcpu-8gb",
					Labels: map[string]string{
						"tier":    "gateway",
						"service": "api-gateway",
					},
				},
				"services": {
					Name:     "services",
					Provider: "digitalocean",
					Count:    20,
					Size:     "s-2vcpu-4gb",
					Labels: map[string]string{
						"tier":          "application",
						"microservices": "enabled",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 2, len(cfg.NodePools))

		return nil
	}, pulumi.WithMocks("test", "microservices", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_DatabaseClusterPlatform tests a database cluster with primary and replicas
func TestOrchestrator_DatabaseClusterPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "database-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc1",
				},
			},
			NodePools: map[string]config.NodePool{
				"database-primary": {
					Name:     "database-primary",
					Provider: "digitalocean",
					Count:    3,
					Size:     "s-8vcpu-16gb",
					Labels: map[string]string{
						"database": "postgres",
						"role":     "primary",
					},
				},
				"database-replica": {
					Name:     "database-replica",
					Provider: "digitalocean",
					Count:    6,
					Size:     "s-4vcpu-8gb",
					Labels: map[string]string{
						"database": "postgres",
						"role":     "replica",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 3, cfg.NodePools["database-primary"].Count)

		return nil
	}, pulumi.WithMocks("test", "database", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_EdgeComputingIoTPlatform tests edge computing for IoT workloads
func TestOrchestrator_EdgeComputingIoTPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "edge-iot-platform",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc1",
				},
			},
			NodePools: map[string]config.NodePool{
				"edge-gateway": {
					Name:     "edge-gateway",
					Provider: "digitalocean",
					Count:    50,
					Size:     "s-2vcpu-4gb",
					Labels: map[string]string{
						"workload":    "edge",
						"iot-gateway": "enabled",
						"mqtt":        "enabled",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 50, cfg.NodePools["edge-gateway"].Count)

		return nil
	}, pulumi.WithMocks("test", "edge-iot", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_CICDPipelinePlatform tests CI/CD infrastructure
func TestOrchestrator_CICDPipelinePlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "cicd-platform",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc1",
				},
			},
			NodePools: map[string]config.NodePool{
				"jenkins-master": {
					Name:     "jenkins-master",
					Provider: "digitalocean",
					Count:    2,
					Size:     "s-4vcpu-8gb",
					Labels: map[string]string{
						"cicd":    "jenkins",
						"role":    "master",
						"ha-mode": "active-passive",
					},
				},
				"build-agents": {
					Name:     "build-agents",
					Provider: "digitalocean",
					Count:    20,
					Size:     "c-8",
					Labels: map[string]string{
						"cicd":   "jenkins",
						"role":   "agent",
						"docker": "enabled",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 20, cfg.NodePools["build-agents"].Count)

		return nil
	}, pulumi.WithMocks("test", "cicd", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_MultiRegionDisasterRecovery tests multi-region DR setup
func TestOrchestrator_MultiRegionDisasterRecovery(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "dr-multi-region",
				Environment: "production",
			},
			Cluster: config.ClusterSpec{
				HighAvailability: true,
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc1",
				},
			},
			NodePools: map[string]config.NodePool{
				"primary-region": {
					Name:     "primary-region",
					Provider: "digitalocean",
					Count:    30,
					Size:     "s-8vcpu-16gb",
					Labels: map[string]string{
						"region": "primary",
						"dr":     "active",
					},
				},
				"dr-region": {
					Name:     "dr-region",
					Provider: "digitalocean",
					Count:    30,
					Size:     "s-8vcpu-16gb",
					Labels: map[string]string{
						"region": "disaster-recovery",
						"dr":     "standby",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.True(t, cfg.Cluster.HighAvailability)

		return nil
	}, pulumi.WithMocks("test", "dr-multi-region", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_BatchProcessingPlatform tests batch processing workloads
func TestOrchestrator_BatchProcessingPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "batch-processing",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc1",
				},
			},
			NodePools: map[string]config.NodePool{
				"batch-workers": {
					Name:     "batch-workers",
					Provider: "digitalocean",
					Count:    50,
					Size:     "c-32",
					Labels: map[string]string{
						"workload":      "batch",
						"cpu-optimized": "true",
						"apache-spark":  "enabled",
					},
				},
			},
			Cluster: config.ClusterSpec{
				AutoScaling: config.AutoScalingConfig{
					Enabled:   true,
					MinNodes:  10,
					MaxNodes:  200,
					TargetCPU: 70,
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.True(t, cfg.Cluster.AutoScaling.Enabled)

		return nil
	}, pulumi.WithMocks("test", "batch-processing", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_RealTimeAnalyticsPlatform tests real-time analytics infrastructure
func TestOrchestrator_RealTimeAnalyticsPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "realtime-analytics",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc1",
				},
			},
			NodePools: map[string]config.NodePool{
				"kafka-cluster": {
					Name:     "kafka-cluster",
					Provider: "digitalocean",
					Count:    12,
					Size:     "s-8vcpu-16gb",
					Labels: map[string]string{
						"workload":  "streaming",
						"kafka":     "enabled",
						"zookeeper": "enabled",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 12, cfg.NodePools["kafka-cluster"].Count)

		return nil
	}, pulumi.WithMocks("test", "realtime-analytics", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_EcommercePlatform tests e-commerce infrastructure
func TestOrchestrator_EcommercePlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "ecommerce-platform",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc1",
				},
			},
			NodePools: map[string]config.NodePool{
				"web-frontend": {
					Name:     "web-frontend",
					Provider: "digitalocean",
					Count:    15,
					Size:     "c-8",
					Labels: map[string]string{
						"tier":       "frontend",
						"nextjs":     "enabled",
						"cdn-origin": "true",
					},
				},
				"api-backend": {
					Name:     "api-backend",
					Provider: "digitalocean",
					Count:    25,
					Size:     "s-4vcpu-8gb",
					Labels: map[string]string{
						"tier":    "backend",
						"api":     "rest",
						"graphql": "enabled",
					},
				},
			},
			Security: config.SecurityConfig{
				NetworkPolicies: true,
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.True(t, cfg.Security.NetworkPolicies)

		return nil
	}, pulumi.WithMocks("test", "ecommerce", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_ContentManagementPlatform tests CMS infrastructure
func TestOrchestrator_ContentManagementPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "cms-platform",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc1",
				},
			},
			NodePools: map[string]config.NodePool{
				"cms-servers": {
					Name:     "cms-servers",
					Provider: "digitalocean",
					Count:    8,
					Size:     "s-4vcpu-8gb",
					Labels: map[string]string{
						"workload":  "cms",
						"wordpress": "enabled",
						"headless":  "true",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 8, cfg.NodePools["cms-servers"].Count)

		return nil
	}, pulumi.WithMocks("test", "cms", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_SearchIndexingPlatform tests search and indexing infrastructure
func TestOrchestrator_SearchIndexingPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "search-platform",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc1",
				},
			},
			NodePools: map[string]config.NodePool{
				"elasticsearch": {
					Name:     "elasticsearch",
					Provider: "digitalocean",
					Count:    15,
					Size:     "m-16vcpu-128gb",
					Labels: map[string]string{
						"workload":         "search",
						"elasticsearch":    "enabled",
						"memory-optimized": "true",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 15, cfg.NodePools["elasticsearch"].Count)

		return nil
	}, pulumi.WithMocks("test", "search", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_MachineLearningTrainingPlatform tests ML training infrastructure
func TestOrchestrator_MachineLearningTrainingPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "ml-training-platform",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc1",
				},
			},
			NodePools: map[string]config.NodePool{
				"gpu-training": {
					Name:     "gpu-training",
					Provider: "digitalocean",
					Count:    20,
					Size:     "g-8vcpu-48gb",
					Labels: map[string]string{
						"workload":   "ml-training",
						"gpu":        "enabled",
						"tensorflow": "enabled",
						"pytorch":    "enabled",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 20, cfg.NodePools["gpu-training"].Count)

		return nil
	}, pulumi.WithMocks("test", "ml-training", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_DevStagingProdEnvironments tests multi-environment setup
func TestOrchestrator_DevStagingProdEnvironments(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "multi-environment",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc1",
				},
			},
			NodePools: map[string]config.NodePool{
				"production": {
					Name:     "production",
					Provider: "digitalocean",
					Count:    30,
					Size:     "s-8vcpu-16gb",
					Labels: map[string]string{
						"environment": "production",
						"tier":        "critical",
					},
					Taints: []config.TaintConfig{
						{
							Key:    "environment",
							Value:  "production",
							Effect: "NoSchedule",
						},
					},
				},
				"staging": {
					Name:     "staging",
					Provider: "digitalocean",
					Count:    10,
					Size:     "s-4vcpu-8gb",
					Labels: map[string]string{
						"environment": "staging",
						"tier":        "testing",
					},
					Taints: []config.TaintConfig{
						{
							Key:    "environment",
							Value:  "staging",
							Effect: "NoSchedule",
						},
					},
				},
				"development": {
					Name:     "development",
					Provider: "digitalocean",
					Count:    5,
					Size:     "s-2vcpu-4gb",
					Labels: map[string]string{
						"environment": "development",
						"tier":        "dev",
					},
					Taints: []config.TaintConfig{
						{
							Key:    "environment",
							Value:  "development",
							Effect: "NoSchedule",
						},
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 3, len(cfg.NodePools))

		return nil
	}, pulumi.WithMocks("test", "multi-env", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}
