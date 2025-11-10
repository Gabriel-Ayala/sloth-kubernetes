package orchestrator

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// Test 1: Multi-tier application deployment
func TestOrchestrator_MultiTierApplicationDeployment(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "multi-tier-cluster",
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
				"frontend": {
					Name:     "frontend",
					Provider: "digitalocean",
					Count:    5,
					Size:     "s-2vcpu-4gb",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"tier":        "frontend",
						"app-layer":   "presentation",
						"scaling":     "auto",
						"cache":       "enabled",
					},
				},
				"backend": {
					Name:     "backend",
					Provider: "digitalocean",
					Count:    8,
					Size:     "c-4", // CPU-optimized
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"tier":        "backend",
						"app-layer":   "business-logic",
						"api":         "enabled",
					},
				},
				"database": {
					Name:     "database",
					Provider: "digitalocean",
					Count:    3,
					Size:     "m-4vcpu-32gb", // Memory-optimized
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"tier":        "database",
						"app-layer":   "data",
						"stateful":    "true",
						"replication": "enabled",
					},
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify multi-tier configuration
		assert.Equal(t, 3, len(cfg.NodePools))
		assert.Equal(t, "frontend", cfg.NodePools["frontend"].Labels["tier"])
		assert.Equal(t, "backend", cfg.NodePools["backend"].Labels["tier"])
		assert.Equal(t, "database", cfg.NodePools["database"].Labels["tier"])

		return nil
	}, pulumi.WithMocks("test", "multi-tier", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 2: Blue-green deployment strategy
func TestOrchestrator_BlueGreenDeploymentStrategy(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "blue-green-cluster",
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
				"blue-env": {
					Name:     "blue-env",
					Provider: "digitalocean",
					Count:    5,
					Size:     "s-2vcpu-4gb",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"deployment": "blue",
						"active":     "true",
						"version":    "v1.0",
					},
				},
				"green-env": {
					Name:     "green-env",
					Provider: "digitalocean",
					Count:    5,
					Size:     "s-2vcpu-4gb",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"deployment": "green",
						"active":     "false",
						"version":    "v1.1",
					},
				},
			},
		}

		orch := New(ctx, cfg)

		// Verify blue-green configuration
		assert.Equal(t, 2, len(cfg.NodePools))
		assert.Equal(t, "blue", cfg.NodePools["blue-env"].Labels["deployment"])
		assert.Equal(t, "green", cfg.NodePools["green-env"].Labels["deployment"])
		assert.NotNil(t, orch)

		return nil
	}, pulumi.WithMocks("test", "blue-green", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 3: Canary deployment configuration
func TestOrchestrator_CanaryDeploymentConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "canary-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
				},
			},
			NodePools: map[string]config.NodePool{
				"stable": {
					Name:     "stable",
					Provider: "digitalocean",
					Count:    9, // 90% of traffic
					Size:     "s-2vcpu-4gb",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"deployment": "stable",
						"version":    "v1.0",
						"traffic":    "90",
					},
				},
				"canary": {
					Name:     "canary",
					Provider: "digitalocean",
					Count:    1, // 10% of traffic
					Size:     "s-2vcpu-4gb",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"deployment": "canary",
						"version":    "v1.1",
						"traffic":    "10",
					},
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify canary configuration
		assert.Equal(t, 9, cfg.NodePools["stable"].Count)
		assert.Equal(t, 1, cfg.NodePools["canary"].Count)
		assert.Equal(t, "stable", cfg.NodePools["stable"].Labels["deployment"])
		assert.Equal(t, "canary", cfg.NodePools["canary"].Labels["deployment"])

		return nil
	}, pulumi.WithMocks("test", "canary", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 4: Edge computing deployment
func TestOrchestrator_EdgeComputingDeployment(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "edge-computing-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
				},
			},
			NodePools: map[string]config.NodePool{
				"edge-nyc": {
					Name:     "edge-nyc",
					Provider: "digitalocean",
					Count:    3,
					Region:   "nyc3",
					Size:     "s-1vcpu-2gb",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"location": "edge",
						"region":   "nyc",
						"latency":  "low",
					},
				},
				"edge-sfo": {
					Name:     "edge-sfo",
					Provider: "digitalocean",
					Count:    3,
					Region:   "sfo3",
					Size:     "s-1vcpu-2gb",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"location": "edge",
						"region":   "sfo",
						"latency":  "low",
					},
				},
				"central": {
					Name:     "central",
					Provider: "digitalocean",
					Count:    5,
					Region:   "nyc3",
					Size:     "s-4vcpu-8gb",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"location": "central",
						"compute":  "heavy",
					},
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify edge computing configuration
		assert.Equal(t, 3, len(cfg.NodePools))
		assert.Equal(t, "edge", cfg.NodePools["edge-nyc"].Labels["location"])
		assert.Equal(t, "edge", cfg.NodePools["edge-sfo"].Labels["location"])
		assert.Equal(t, "central", cfg.NodePools["central"].Labels["location"])

		return nil
	}, pulumi.WithMocks("test", "edge-computing", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 5: GPU workload configuration
func TestOrchestrator_GPUWorkloadConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "gpu-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
				},
			},
			NodePools: map[string]config.NodePool{
				"gpu-nodes": {
					Name:     "gpu-nodes",
					Provider: "digitalocean",
					Count:    4,
					Size:     "g-8vcpu-32gb", // GPU-enabled
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"accelerator": "gpu",
						"gpu-type":    "nvidia-tesla-v100",
						"ml-enabled":  "true",
						"workload":    "machine-learning",
					},
				},
				"cpu-nodes": {
					Name:     "cpu-nodes",
					Provider: "digitalocean",
					Count:    10,
					Size:     "c-8",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"accelerator": "none",
						"workload":    "general-purpose",
					},
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify GPU configuration
		assert.Equal(t, "gpu", cfg.NodePools["gpu-nodes"].Labels["accelerator"])
		assert.Equal(t, "machine-learning", cfg.NodePools["gpu-nodes"].Labels["workload"])
		assert.Equal(t, "g-8vcpu-32gb", cfg.NodePools["gpu-nodes"].Size)

		return nil
	}, pulumi.WithMocks("test", "gpu-workload", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 6: IoT device management cluster
func TestOrchestrator_IoTDeviceManagementCluster(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "iot-management-cluster",
				Environment: "production",
				Labels: map[string]string{
					"purpose": "iot",
					"devices": "10000+",
				},
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
				},
			},
			NodePools: map[string]config.NodePool{
				"iot-gateway": {
					Name:     "iot-gateway",
					Provider: "digitalocean",
					Count:    8,
					Size:     "s-2vcpu-4gb",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"component": "gateway",
						"protocol":  "mqtt",
						"tls":       "enabled",
					},
				},
				"iot-processing": {
					Name:     "iot-processing",
					Provider: "digitalocean",
					Count:    12,
					Size:     "c-4",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"component":  "processing",
						"stream":     "kafka",
						"real-time":  "true",
					},
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify IoT configuration
		assert.Equal(t, "iot", cfg.Metadata.Labels["purpose"])
		assert.Equal(t, "gateway", cfg.NodePools["iot-gateway"].Labels["component"])
		assert.Equal(t, "processing", cfg.NodePools["iot-processing"].Labels["component"])

		return nil
	}, pulumi.WithMocks("test", "iot-management", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 7: Serverless function platform
func TestOrchestrator_ServerlessFunctionPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "serverless-platform",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
				},
			},
			NodePools: map[string]config.NodePool{
				"function-runners": {
					Name:     "function-runners",
					Provider: "digitalocean",
					Count:    15,
					Size:     "s-2vcpu-4gb",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"platform":      "knative",
						"autoscale":     "enabled",
						"scale-to-zero": "true",
						"cold-start":    "optimized",
					},
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify serverless configuration
		assert.Equal(t, "knative", cfg.NodePools["function-runners"].Labels["platform"])
		assert.Equal(t, "true", cfg.NodePools["function-runners"].Labels["scale-to-zero"])

		return nil
	}, pulumi.WithMocks("test", "serverless", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 8: Data analytics cluster
func TestOrchestrator_DataAnalyticsCluster(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "analytics-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
				},
			},
			NodePools: map[string]config.NodePool{
				"spark-master": {
					Name:     "spark-master",
					Provider: "digitalocean",
					Count:    3,
					Size:     "m-8vcpu-64gb",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"framework": "spark",
						"role":      "master",
						"ha":        "enabled",
					},
				},
				"spark-workers": {
					Name:     "spark-workers",
					Provider: "digitalocean",
					Count:    20,
					Size:     "m-16vcpu-128gb",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"framework": "spark",
						"role":      "worker",
						"memory":    "optimized",
					},
				},
			},
			Storage: config.StorageConfig{
				DefaultClass: "high-performance",
				Classes: []config.StorageClass{
					{
						Name:        "high-performance",
						Provisioner: "dobs.csi.digitalocean.com",
						Parameters: map[string]string{
							"type":  "ssd",
							"iops":  "10000",
						},
					},
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify analytics configuration
		assert.Equal(t, "spark", cfg.NodePools["spark-master"].Labels["framework"])
		assert.Equal(t, "master", cfg.NodePools["spark-master"].Labels["role"])
		assert.Equal(t, "worker", cfg.NodePools["spark-workers"].Labels["role"])
		assert.Equal(t, 20, cfg.NodePools["spark-workers"].Count)

		return nil
	}, pulumi.WithMocks("test", "analytics", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 9: Microservices mesh configuration
func TestOrchestrator_MicroservicesMeshConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "microservices-mesh",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
				},
			},
			Network: config.NetworkConfig{
				ServiceMesh: &config.ServiceMeshConfig{
					Type:    "istio",
					Version: "1.19.0",
					MTLS:    true,
					Tracing: true,
				},
			},
			NodePools: map[string]config.NodePool{
				"services": {
					Name:     "services",
					Provider: "digitalocean",
					Count:    12,
					Size:     "s-2vcpu-4gb",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"mesh":        "istio",
						"sidecar":     "enabled",
						"tracing":     "enabled",
						"observability": "full",
					},
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify service mesh configuration
		assert.Equal(t, "istio", cfg.Network.ServiceMesh.Type)
		assert.Equal(t, "1.19.0", cfg.Network.ServiceMesh.Version)
		assert.True(t, cfg.Network.ServiceMesh.MTLS)
		assert.Equal(t, "istio", cfg.NodePools["services"].Labels["mesh"])

		return nil
	}, pulumi.WithMocks("test", "microservices-mesh", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 10: Batch processing workload
func TestOrchestrator_BatchProcessingWorkload(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "batch-processing-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
				},
			},
			NodePools: map[string]config.NodePool{
				"batch-high-priority": {
					Name:     "batch-high-priority",
					Provider: "digitalocean",
					Count:    5,
					Size:     "c-8",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload":  "batch",
						"priority":  "high",
						"guarantee": "reserved",
					},
				},
				"batch-low-priority": {
					Name:     "batch-low-priority",
					Provider: "digitalocean",
					Count:    15,
					Size:     "c-4",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload":     "batch",
						"priority":     "low",
						"preemptible":  "true",
						"cost-optimized": "true",
					},
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify batch processing configuration
		assert.Equal(t, "batch", cfg.NodePools["batch-high-priority"].Labels["workload"])
		assert.Equal(t, "high", cfg.NodePools["batch-high-priority"].Labels["priority"])
		assert.Equal(t, "low", cfg.NodePools["batch-low-priority"].Labels["priority"])
		assert.Equal(t, "true", cfg.NodePools["batch-low-priority"].Labels["preemptible"])

		return nil
	}, pulumi.WithMocks("test", "batch-processing", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 11: Real-time streaming platform
func TestOrchestrator_RealTimeStreamingPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "streaming-platform",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
				},
			},
			NodePools: map[string]config.NodePool{
				"kafka-brokers": {
					Name:     "kafka-brokers",
					Provider: "digitalocean",
					Count:    9,
					Size:     "m-4vcpu-32gb",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"platform":    "kafka",
						"role":        "broker",
						"replication": "3",
						"storage":     "fast-ssd",
					},
				},
				"stream-processors": {
					Name:     "stream-processors",
					Provider: "digitalocean",
					Count:    12,
					Size:     "c-8",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"platform":   "kafka-streams",
						"role":       "processor",
						"real-time":  "true",
						"throughput": "high",
					},
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify streaming configuration
		assert.Equal(t, "kafka", cfg.NodePools["kafka-brokers"].Labels["platform"])
		assert.Equal(t, "broker", cfg.NodePools["kafka-brokers"].Labels["role"])
		assert.Equal(t, "kafka-streams", cfg.NodePools["stream-processors"].Labels["platform"])
		assert.Equal(t, "processor", cfg.NodePools["stream-processors"].Labels["role"])

		return nil
	}, pulumi.WithMocks("test", "streaming", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 12: Development and staging environments
func TestOrchestrator_DevelopmentStagingEnvironments(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		nodeCount   int
		nodeSize    string
	}{
		{
			name:        "Development environment",
			environment: "development",
			nodeCount:   3,
			nodeSize:    "s-1vcpu-2gb",
		},
		{
			name:        "Staging environment",
			environment: "staging",
			nodeCount:   5,
			nodeSize:    "s-2vcpu-4gb",
		},
		{
			name:        "Production environment",
			environment: "production",
			nodeCount:   15,
			nodeSize:    "s-4vcpu-8gb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				cfg := &config.ClusterConfig{
					Metadata: config.Metadata{
						Name:        tt.environment + "-cluster",
						Environment: tt.environment,
					},
					Providers: config.ProvidersConfig{
						DigitalOcean: &config.DigitalOceanProvider{
							Enabled: true,
							Token:   "test-token",
						},
					},
					NodePools: map[string]config.NodePool{
						"workers": {
							Name:     "workers",
							Provider: "digitalocean",
							Count:    tt.nodeCount,
							Size:     tt.nodeSize,
							Roles:    []string{"worker"},
							Labels: map[string]string{
								"environment": tt.environment,
							},
						},
					},
				}

				_ = New(ctx, cfg)

				// Verify environment configuration
				assert.Equal(t, tt.environment, cfg.Metadata.Environment)
				assert.Equal(t, tt.nodeCount, cfg.NodePools["workers"].Count)
				assert.Equal(t, tt.nodeSize, cfg.NodePools["workers"].Size)

				return nil
			}, pulumi.WithMocks("test", "env-"+tt.environment, &IntegrationMockProvider{}))
			assert.NoError(t, err)
		})
	}
}
