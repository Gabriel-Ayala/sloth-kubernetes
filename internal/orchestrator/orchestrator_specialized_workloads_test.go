package orchestrator

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// TestOrchestrator_VideoStreamingPlatform tests video streaming and transcoding infrastructure
func TestOrchestrator_VideoStreamingPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "video-streaming-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled:         true,
					AccessKeyID:     "test-key",
					SecretAccessKey: "test-secret",
					Region:          "us-west-2",
				},
			},
			NodePools: map[string]config.NodePool{
				"transcoding": {
					Name:     "transcoding",
					Provider: "aws",
					Count:    10,
					Size:     "c5.4xlarge",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload":      "transcoding",
						"cpu-optimized": "true",
						"ffmpeg":        "enabled",
					},
				},
				"streaming": {
					Name:     "streaming",
					Provider: "aws",
					Count:    20,
					Size:     "m5.2xlarge",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload": "streaming",
						"hls":      "enabled",
						"dash":     "enabled",
					},
				},
				"storage": {
					Name:     "storage",
					Provider: "aws",
					Count:    5,
					Size:     "i3.2xlarge",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload":   "storage",
						"local-nvme": "true",
					},
				},
			},
			Storage: config.StorageConfig{
				Classes: []config.StorageClass{
					{
						Name:        "video-storage",
						Provisioner: "ebs.csi.aws.com",
						Parameters: map[string]string{
							"type":       "st1",
							"throughput": "500",
						},
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, "transcoding", cfg.NodePools["transcoding"].Labels["workload"])
		assert.Equal(t, "streaming", cfg.NodePools["streaming"].Labels["workload"])
		assert.Equal(t, 10, cfg.NodePools["transcoding"].Count)
		assert.Equal(t, 20, cfg.NodePools["streaming"].Count)

		return nil
	}, pulumi.WithMocks("test", "video-streaming", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_CDNEdgeDeployment tests CDN edge node deployment
func TestOrchestrator_CDNEdgeDeployment(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "cdn-edge-cluster",
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
				"edge-nyc": {
					Name:     "edge-nyc",
					Provider: "digitalocean",
					Count:    5,
					Size:     "c-8",
					Region:   "nyc3",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"cdn-role": "edge",
						"location": "nyc",
						"cache":    "enabled",
					},
				},
				"edge-sfo": {
					Name:     "edge-sfo",
					Provider: "digitalocean",
					Count:    5,
					Size:     "c-8",
					Region:   "sfo3",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"cdn-role": "edge",
						"location": "sfo",
						"cache":    "enabled",
					},
				},
				"edge-ams": {
					Name:     "edge-ams",
					Provider: "digitalocean",
					Count:    5,
					Size:     "c-8",
					Region:   "ams3",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"cdn-role": "edge",
						"location": "ams",
						"cache":    "enabled",
					},
				},
			},
			Network: config.NetworkConfig{
				Mode:                    "vpc",
				CrossProviderNetworking: true,
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, "edge", cfg.NodePools["edge-nyc"].Labels["cdn-role"])
		assert.Equal(t, "nyc", cfg.NodePools["edge-nyc"].Labels["location"])
		assert.Equal(t, 3, len(cfg.NodePools))

		return nil
	}, pulumi.WithMocks("test", "cdn-edge", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_GamingServersCluster tests gaming server infrastructure
func TestOrchestrator_GamingServersCluster(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "gaming-servers-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled:         true,
					AccessKeyID:     "test-key",
					SecretAccessKey: "test-secret",
					Region:          "us-east-1",
				},
			},
			NodePools: map[string]config.NodePool{
				"game-servers": {
					Name:        "game-servers",
					Provider:    "aws",
					Count:       50,
					MinCount:    20,
					MaxCount:    200,
					Size:        "c5.xlarge",
					Roles:       []string{"worker"},
					AutoScaling: true,
					Labels: map[string]string{
						"workload":       "game-server",
						"low-latency":    "true",
						"dedicated-core": "true",
					},
				},
				"matchmaking": {
					Name:     "matchmaking",
					Provider: "aws",
					Count:    5,
					Size:     "m5.2xlarge",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload": "matchmaking",
						"redis":    "enabled",
					},
				},
			},
			Cluster: config.ClusterSpec{
				Type:    "rke2",
				Version: "v1.28.0",
				AutoScaling: config.AutoScalingConfig{
					Enabled:  true,
					MinNodes: 25,
					MaxNodes: 205,
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.True(t, cfg.NodePools["game-servers"].AutoScaling)
		assert.Equal(t, 200, cfg.NodePools["game-servers"].MaxCount)
		assert.Equal(t, "game-server", cfg.NodePools["game-servers"].Labels["workload"])

		return nil
	}, pulumi.WithMocks("test", "gaming-servers", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_FinancialTradingPlatform tests high-frequency trading infrastructure
func TestOrchestrator_FinancialTradingPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "financial-trading-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled:         true,
					AccessKeyID:     "test-key",
					SecretAccessKey: "test-secret",
					Region:          "us-east-1",
				},
			},
			NodePools: map[string]config.NodePool{
				"trading-engines": {
					Name:     "trading-engines",
					Provider: "aws",
					Count:    10,
					Size:     "c5n.9xlarge",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload":          "trading",
						"ultra-low-latency": "true",
						"network-optimized": "true",
						"cpu-pinning":       "enabled",
					},
				},
				"risk-analytics": {
					Name:     "risk-analytics",
					Provider: "aws",
					Count:    5,
					Size:     "r5.4xlarge",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload":         "risk-analytics",
						"memory-optimized": "true",
					},
				},
			},
			Security: config.SecurityConfig{
				Compliance: config.ComplianceConfig{
					Standards: []string{"SOX", "MiFID-II", "SEC"},
					Scanning:  true,
					Reporting: true,
				},
				Audit: config.AuditConfig{
					Enabled:  true,
					Level:    "RequestResponse",
					Backend:  "s3",
					Rotation: "hourly",
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, "trading", cfg.NodePools["trading-engines"].Labels["workload"])
		assert.Contains(t, cfg.Security.Compliance.Standards, "SOX")
		assert.True(t, cfg.Security.Audit.Enabled)

		return nil
	}, pulumi.WithMocks("test", "financial-trading", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_HealthcareFHIRPlatform tests healthcare FHIR infrastructure
func TestOrchestrator_HealthcareFHIRPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "healthcare-fhir-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				Azure: &config.AzureProvider{
					Enabled:        true,
					SubscriptionID: "test-sub",
					TenantID:       "test-tenant",
					ClientID:       "test-client",
					ClientSecret:   "test-secret",
					Location:       "eastus",
				},
			},
			NodePools: map[string]config.NodePool{
				"fhir-servers": {
					Name:     "fhir-servers",
					Provider: "azure",
					Count:    5,
					Size:     "Standard_D8s_v3",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload": "fhir",
						"hipaa":    "compliant",
					},
				},
			},
			Security: config.SecurityConfig{
				Compliance: config.ComplianceConfig{
					Standards: []string{"HIPAA", "HITECH", "GDPR"},
					Scanning:  true,
					Reporting: true,
				},
				TLS: config.TLSConfig{
					Enabled:     true,
					CertManager: true,
					Provider:    "cert-manager",
				},
				Secrets: config.SecretsConfig{
					Provider:        "vault",
					Encryption:      true,
					KeyManagement:   "azure-key-vault",
					ExternalSecrets: true,
				},
				Audit: config.AuditConfig{
					Enabled:  true,
					Level:    "RequestResponse",
					Backend:  "azure-monitor",
					Rotation: "daily",
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Contains(t, cfg.Security.Compliance.Standards, "HIPAA")
		assert.True(t, cfg.Security.Secrets.Encryption)
		assert.Equal(t, "fhir", cfg.NodePools["fhir-servers"].Labels["workload"])

		return nil
	}, pulumi.WithMocks("test", "healthcare-fhir", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_BlockchainNodesPlatform tests blockchain node infrastructure
func TestOrchestrator_BlockchainNodesPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "blockchain-nodes-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				GCP: &config.GCPProvider{
					Enabled:     true,
					ProjectID:   "blockchain-project",
					Region:      "us-central1",
					Credentials: "test-creds",
				},
			},
			NodePools: map[string]config.NodePool{
				"validator-nodes": {
					Name:     "validator-nodes",
					Provider: "gcp",
					Count:    3,
					Size:     "n2-standard-8",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"blockchain-role": "validator",
						"network":         "mainnet",
					},
				},
				"full-nodes": {
					Name:     "full-nodes",
					Provider: "gcp",
					Count:    10,
					Size:     "n2-standard-4",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"blockchain-role": "full-node",
						"network":         "mainnet",
					},
				},
			},
			Storage: config.StorageConfig{
				Classes: []config.StorageClass{
					{
						Name:        "blockchain-data",
						Provisioner: "pd.csi.storage.gke.io",
						Parameters: map[string]string{
							"type": "pd-ssd",
						},
						ReclaimPolicy: "Retain",
					},
				},
				PersistentVolumes: []config.PersistentVolume{
					{
						Name:         "validator-data",
						Size:         "2Ti",
						StorageClass: "blockchain-data",
						AccessModes:  []string{"ReadWriteOnce"},
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, "validator", cfg.NodePools["validator-nodes"].Labels["blockchain-role"])
		assert.Equal(t, "full-node", cfg.NodePools["full-nodes"].Labels["blockchain-role"])
		assert.Len(t, cfg.Storage.PersistentVolumes, 1)

		return nil
	}, pulumi.WithMocks("test", "blockchain-nodes", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_ScienceComputingCluster tests scientific computing infrastructure
func TestOrchestrator_ScienceComputingCluster(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "science-computing-cluster",
				Environment: "research",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled:         true,
					AccessKeyID:     "test-key",
					SecretAccessKey: "test-secret",
					Region:          "us-west-2",
				},
			},
			NodePools: map[string]config.NodePool{
				"cpu-compute": {
					Name:     "cpu-compute",
					Provider: "aws",
					Count:    20,
					Size:     "c5n.18xlarge",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload":  "hpc",
						"cpu-cores": "72",
						"mpi":       "enabled",
					},
				},
				"gpu-compute": {
					Name:     "gpu-compute",
					Provider: "aws",
					Count:    10,
					Size:     "p3dn.24xlarge",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload":  "gpu-hpc",
						"gpu-type":  "V100",
						"gpu-count": "8",
						"nvlink":    "enabled",
					},
				},
			},
			Storage: config.StorageConfig{
				Classes: []config.StorageClass{
					{
						Name:        "parallel-fs",
						Provisioner: "fsx.csi.aws.com",
						Parameters: map[string]string{
							"type":       "lustre",
							"throughput": "1000",
						},
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, "hpc", cfg.NodePools["cpu-compute"].Labels["workload"])
		assert.Equal(t, "gpu-hpc", cfg.NodePools["gpu-compute"].Labels["workload"])
		assert.Equal(t, 20, cfg.NodePools["cpu-compute"].Count)

		return nil
	}, pulumi.WithMocks("test", "science-computing", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_RenderFarmCluster tests 3D rendering farm infrastructure
func TestOrchestrator_RenderFarmCluster(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "render-farm-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled:         true,
					AccessKeyID:     "test-key",
					SecretAccessKey: "test-secret",
					Region:          "us-west-2",
				},
			},
			NodePools: map[string]config.NodePool{
				"render-cpu": {
					Name:         "render-cpu",
					Provider:     "aws",
					Count:        10,
					MinCount:     5,
					MaxCount:     100,
					Size:         "c5.24xlarge",
					Roles:        []string{"worker"},
					SpotInstance: true,
					AutoScaling:  true,
					Labels: map[string]string{
						"workload": "cpu-render",
						"blender":  "enabled",
						"arnold":   "enabled",
					},
				},
				"render-gpu": {
					Name:         "render-gpu",
					Provider:     "aws",
					Count:        5,
					MinCount:     2,
					MaxCount:     50,
					Size:         "g4dn.12xlarge",
					Roles:        []string{"worker"},
					SpotInstance: true,
					AutoScaling:  true,
					Labels: map[string]string{
						"workload": "gpu-render",
						"octane":   "enabled",
						"redshift": "enabled",
					},
				},
			},
			Storage: config.StorageConfig{
				Classes: []config.StorageClass{
					{
						Name:        "render-assets",
						Provisioner: "efs.csi.aws.com",
						Parameters: map[string]string{
							"throughputMode": "bursting",
						},
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.True(t, cfg.NodePools["render-cpu"].SpotInstance)
		assert.True(t, cfg.NodePools["render-gpu"].AutoScaling)
		assert.Equal(t, 100, cfg.NodePools["render-cpu"].MaxCount)

		return nil
	}, pulumi.WithMocks("test", "render-farm", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_WeatherForecastingCluster tests weather forecasting infrastructure
func TestOrchestrator_WeatherForecastingCluster(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "weather-forecasting-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				GCP: &config.GCPProvider{
					Enabled:     true,
					ProjectID:   "weather-project",
					Region:      "us-central1",
					Credentials: "test-creds",
				},
			},
			NodePools: map[string]config.NodePool{
				"model-compute": {
					Name:     "model-compute",
					Provider: "gcp",
					Count:    30,
					Size:     "c2-standard-60",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload": "weather-model",
						"wrf":      "enabled",
						"gfs":      "enabled",
					},
				},
				"data-ingestion": {
					Name:     "data-ingestion",
					Provider: "gcp",
					Count:    5,
					Size:     "n2-standard-8",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload": "data-ingestion",
						"kafka":    "enabled",
					},
				},
			},
			Storage: config.StorageConfig{
				Classes: []config.StorageClass{
					{
						Name:        "weather-data",
						Provisioner: "pd.csi.storage.gke.io",
						Parameters: map[string]string{
							"type": "pd-balanced",
						},
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, "weather-model", cfg.NodePools["model-compute"].Labels["workload"])
		assert.Equal(t, 30, cfg.NodePools["model-compute"].Count)

		return nil
	}, pulumi.WithMocks("test", "weather-forecasting", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_SatelliteImageryProcessing tests satellite imagery processing
func TestOrchestrator_SatelliteImageryProcessing(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "satellite-imagery-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled:         true,
					AccessKeyID:     "test-key",
					SecretAccessKey: "test-secret",
					Region:          "us-west-2",
				},
			},
			NodePools: map[string]config.NodePool{
				"image-processing": {
					Name:     "image-processing",
					Provider: "aws",
					Count:    15,
					Size:     "r5.8xlarge",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload": "image-processing",
						"gdal":     "enabled",
						"opencv":   "enabled",
					},
				},
				"ml-inference": {
					Name:     "ml-inference",
					Provider: "aws",
					Count:    8,
					Size:     "p3.8xlarge",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload":         "ml-inference",
						"object-detection": "enabled",
						"segmentation":     "enabled",
					},
				},
			},
			Storage: config.StorageConfig{
				Classes: []config.StorageClass{
					{
						Name:        "imagery-storage",
						Provisioner: "ebs.csi.aws.com",
						Parameters: map[string]string{
							"type":       "io2",
							"iops":       "64000",
							"throughput": "1000",
						},
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, "image-processing", cfg.NodePools["image-processing"].Labels["workload"])
		assert.Equal(t, "ml-inference", cfg.NodePools["ml-inference"].Labels["workload"])

		return nil
	}, pulumi.WithMocks("test", "satellite-imagery", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_AutonomousVehicleSimulation tests autonomous vehicle simulation
func TestOrchestrator_AutonomousVehicleSimulation(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "av-simulation-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled:         true,
					AccessKeyID:     "test-key",
					SecretAccessKey: "test-secret",
					Region:          "us-east-1",
				},
			},
			NodePools: map[string]config.NodePool{
				"simulation-engines": {
					Name:     "simulation-engines",
					Provider: "aws",
					Count:    20,
					Size:     "g4dn.16xlarge",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload": "simulation",
						"carla":    "enabled",
						"unreal":   "enabled",
						"gpu":      "tesla-t4",
					},
				},
				"scenario-runners": {
					Name:     "scenario-runners",
					Provider: "aws",
					Count:    10,
					Size:     "c5.9xlarge",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload": "scenario-runner",
						"ros2":     "enabled",
					},
				},
			},
			Storage: config.StorageConfig{
				Classes: []config.StorageClass{
					{
						Name:        "simulation-data",
						Provisioner: "fsx.csi.aws.com",
						Parameters: map[string]string{
							"type":       "lustre",
							"throughput": "500",
						},
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, "simulation", cfg.NodePools["simulation-engines"].Labels["workload"])
		assert.Equal(t, 20, cfg.NodePools["simulation-engines"].Count)

		return nil
	}, pulumi.WithMocks("test", "av-simulation", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_GreenComputingOptimization tests sustainable/green computing
func TestOrchestrator_GreenComputingOptimization(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "green-computing-cluster",
				Environment: "production",
				Labels: map[string]string{
					"sustainability": "carbon-aware",
					"renewable":      "solar-powered",
				},
			},
			Providers: config.ProvidersConfig{
				GCP: &config.GCPProvider{
					Enabled:     true,
					ProjectID:   "green-project",
					Region:      "europe-north1",
					Credentials: "test-creds",
				},
			},
			NodePools: map[string]config.NodePool{
				"low-carbon": {
					Name:     "low-carbon",
					Provider: "gcp",
					Count:    10,
					Size:     "e2-standard-8",
					Region:   "europe-north1",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"carbon-aware":     "true",
						"energy-efficient": "true",
						"renewable-region": "true",
					},
				},
				"batch-flexible": {
					Name:         "batch-flexible",
					Provider:     "gcp",
					Count:        5,
					MinCount:     0,
					MaxCount:     50,
					Size:         "e2-standard-4",
					Region:       "europe-north1",
					Roles:        []string{"worker"},
					SpotInstance: true,
					AutoScaling:  true,
					Labels: map[string]string{
						"workload":        "batch",
						"time-flexible":   "true",
						"carbon-optimize": "true",
					},
				},
			},
			Cluster: config.ClusterSpec{
				Type:    "rke2",
				Version: "v1.28.0",
				AutoScaling: config.AutoScalingConfig{
					Enabled:  true,
					MinNodes: 10,
					MaxNodes: 60,
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, "carbon-aware", cfg.Metadata.Labels["sustainability"])
		assert.Equal(t, "true", cfg.NodePools["low-carbon"].Labels["renewable-region"])
		assert.True(t, cfg.NodePools["batch-flexible"].SpotInstance)

		return nil
	}, pulumi.WithMocks("test", "green-computing", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}
