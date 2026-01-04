package orchestrator

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// TestOrchestrator_MultiTenancyIsolation tests multi-tenant cluster configuration
func TestOrchestrator_MultiTenancyIsolation(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "multi-tenant-cluster",
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
				"tenant-a": {
					Name:     "tenant-a",
					Provider: "aws",
					Count:    3,
					Size:     "t3.large",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"tenant":      "tenant-a",
						"environment": "production",
					},
					Taints: []config.TaintConfig{
						{
							Key:    "tenant",
							Value:  "tenant-a",
							Effect: "NoSchedule",
						},
					},
				},
				"tenant-b": {
					Name:     "tenant-b",
					Provider: "aws",
					Count:    3,
					Size:     "t3.large",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"tenant":      "tenant-b",
						"environment": "production",
					},
					Taints: []config.TaintConfig{
						{
							Key:    "tenant",
							Value:  "tenant-b",
							Effect: "NoSchedule",
						},
					},
				},
			},
			Security: config.SecurityConfig{
				NetworkPolicies: true,
				RBAC: config.RBACConfig{
					Enabled:       true,
					DefaultPolicy: "deny-all",
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, "tenant-a", cfg.NodePools["tenant-a"].Labels["tenant"])
		assert.Equal(t, "tenant-b", cfg.NodePools["tenant-b"].Labels["tenant"])
		assert.True(t, cfg.Security.NetworkPolicies)
		assert.True(t, cfg.Security.RBAC.Enabled)

		return nil
	}, pulumi.WithMocks("test", "multi-tenancy", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_AdvancedNetworkSegmentation tests network segmentation with VLANs
func TestOrchestrator_AdvancedNetworkSegmentation(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "network-segmented-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				GCP: &config.GCPProvider{
					Enabled:     true,
					ProjectID:   "test-project",
					Region:      "us-central1",
					Credentials: "test-creds",
				},
			},
			NodePools: map[string]config.NodePool{
				"dmz": {
					Name:     "dmz",
					Provider: "gcp",
					Count:    2,
					Size:     "n1-standard-2",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"zone": "dmz",
						"tier": "public",
					},
				},
				"internal": {
					Name:     "internal",
					Provider: "gcp",
					Count:    5,
					Size:     "n1-standard-4",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"zone": "internal",
						"tier": "private",
					},
				},
			},
			Network: config.NetworkConfig{
				Mode:    "vpc",
				CIDR:    "10.0.0.0/16",
				PodCIDR: "10.244.0.0/16",
				Subnets: []config.SubnetConfig{
					{
						Name: "dmz-subnet",
						CIDR: "10.0.1.0/24",
						Zone: "us-central1-a",
					},
					{
						Name: "internal-subnet",
						CIDR: "10.0.2.0/24",
						Zone: "us-central1-a",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, "vpc", cfg.Network.Mode)
		assert.Equal(t, "10.0.0.0/16", cfg.Network.CIDR)
		assert.Len(t, cfg.Network.Subnets, 2)
		assert.Equal(t, "dmz-subnet", cfg.Network.Subnets[0].Name)

		return nil
	}, pulumi.WithMocks("test", "network-segmentation", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_ZeroTrustSecurityModel tests zero-trust security implementation
func TestOrchestrator_ZeroTrustSecurityModel(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "zero-trust-cluster",
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
				"secured": {
					Name:     "secured",
					Provider: "azure",
					Count:    5,
					Size:     "Standard_D4s_v3",
					Roles:    []string{"worker"},
				},
			},
			Security: config.SecurityConfig{
				TLS: config.TLSConfig{
					Enabled:     true,
					CertManager: true,
					Provider:    "cert-manager",
				},
				RBAC: config.RBACConfig{
					Enabled:       true,
					DefaultPolicy: "deny-all",
				},
				NetworkPolicies: true,
				Secrets: config.SecretsConfig{
					Provider:        "vault",
					Encryption:      true,
					KeyManagement:   "azure-key-vault",
					ExternalSecrets: true,
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
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.True(t, cfg.Security.TLS.Enabled)
		assert.True(t, cfg.Security.RBAC.Enabled)
		assert.True(t, cfg.Security.NetworkPolicies)
		assert.True(t, cfg.Security.Secrets.Encryption)
		assert.True(t, cfg.Network.ServiceMesh.MTLS)

		return nil
	}, pulumi.WithMocks("test", "zero-trust", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_StatefulApplicationsCluster tests stateful workloads with advanced storage
func TestOrchestrator_StatefulApplicationsCluster(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "stateful-apps-cluster",
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
				"stateful": {
					Name:     "stateful",
					Provider: "digitalocean",
					Count:    5,
					Size:     "s-4vcpu-8gb",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload": "stateful",
						"storage":  "local-ssd",
					},
				},
			},
			Storage: config.StorageConfig{
				DefaultClass: "fast-storage",
				Classes: []config.StorageClass{
					{
						Name:              "fast-storage",
						Provisioner:       "dobs.csi.digitalocean.com",
						ReclaimPolicy:     "Retain",
						VolumeBindingMode: "WaitForFirstConsumer",
						Parameters: map[string]string{
							"type": "pd-ssd",
						},
					},
				},
				PersistentVolumes: []config.PersistentVolume{
					{
						Name:         "postgres-data",
						Size:         "500Gi",
						StorageClass: "fast-storage",
						AccessModes:  []string{"ReadWriteOnce"},
						Labels: map[string]string{
							"app": "postgres",
						},
					},
					{
						Name:         "mongodb-data",
						Size:         "1Ti",
						StorageClass: "fast-storage",
						AccessModes:  []string{"ReadWriteOnce"},
						Labels: map[string]string{
							"app": "mongodb",
						},
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, "fast-storage", cfg.Storage.DefaultClass)
		assert.Len(t, cfg.Storage.PersistentVolumes, 2)
		assert.Equal(t, "postgres-data", cfg.Storage.PersistentVolumes[0].Name)
		assert.Equal(t, "500Gi", cfg.Storage.PersistentVolumes[0].Size)

		return nil
	}, pulumi.WithMocks("test", "stateful-apps", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_AIMLWorkloadCluster tests AI/ML workload configuration
func TestOrchestrator_AIMLWorkloadCluster(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "aiml-cluster",
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
				"gpu-training": {
					Name:     "gpu-training",
					Provider: "aws",
					Count:    4,
					Size:     "p3.8xlarge",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload":    "ml-training",
						"gpu":         "nvidia-v100",
						"gpu-count":   "4",
						"accelerator": "gpu",
					},
					Taints: []config.TaintConfig{
						{
							Key:    "nvidia.com/gpu",
							Value:  "true",
							Effect: "NoSchedule",
						},
					},
				},
				"gpu-inference": {
					Name:     "gpu-inference",
					Provider: "aws",
					Count:    8,
					Size:     "p3.2xlarge",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload":    "ml-inference",
						"gpu":         "nvidia-v100",
						"gpu-count":   "1",
						"accelerator": "gpu",
					},
				},
			},
			Storage: config.StorageConfig{
				Classes: []config.StorageClass{
					{
						Name:        "ml-datasets",
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
		assert.Equal(t, "ml-training", cfg.NodePools["gpu-training"].Labels["workload"])
		assert.Equal(t, "p3.8xlarge", cfg.NodePools["gpu-training"].Size)
		assert.Len(t, cfg.NodePools["gpu-training"].Taints, 1)
		assert.Equal(t, 4, cfg.NodePools["gpu-training"].Count)

		return nil
	}, pulumi.WithMocks("test", "aiml", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_ComplianceAuditingPlatform tests compliance and auditing setup
func TestOrchestrator_ComplianceAuditingPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "compliance-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				GCP: &config.GCPProvider{
					Enabled:     true,
					ProjectID:   "compliance-project",
					Region:      "us-east1",
					Credentials: "test-creds",
				},
			},
			NodePools: map[string]config.NodePool{
				"compliant": {
					Name:     "compliant",
					Provider: "gcp",
					Count:    3,
					Size:     "n2-standard-4",
					Roles:    []string{"worker"},
				},
			},
			Security: config.SecurityConfig{
				Compliance: config.ComplianceConfig{
					Standards: []string{"PCI-DSS", "HIPAA", "SOC2"},
					Scanning:  true,
					Reporting: true,
				},
				Audit: config.AuditConfig{
					Enabled:  true,
					Level:    "Metadata",
					Backend:  "webhook",
					Rotation: "daily",
					Filters:  []string{"RequestReceived", "ResponseComplete"},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Contains(t, cfg.Security.Compliance.Standards, "PCI-DSS")
		assert.Contains(t, cfg.Security.Compliance.Standards, "HIPAA")
		assert.True(t, cfg.Security.Compliance.Scanning)
		assert.True(t, cfg.Security.Audit.Enabled)
		assert.Equal(t, "Metadata", cfg.Security.Audit.Level)

		return nil
	}, pulumi.WithMocks("test", "compliance", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_HybridCloudBurstingScenario tests cloud bursting from on-prem
func TestOrchestrator_HybridCloudBurstingScenario(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "hybrid-bursting-cluster",
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
				"on-prem-primary": {
					Name:     "on-prem-primary",
					Provider: "aws",
					Count:    10,
					Size:     "m5.2xlarge",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"location": "on-premises",
						"priority": "high",
					},
				},
				"cloud-burst": {
					Name:         "cloud-burst",
					Provider:     "aws",
					Count:        0,
					MinCount:     0,
					MaxCount:     50,
					Size:         "m5.xlarge",
					Roles:        []string{"worker"},
					SpotInstance: true,
					Labels: map[string]string{
						"location": "cloud",
						"burst":    "true",
						"priority": "low",
					},
					AutoScaling: true,
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
		assert.Equal(t, "on-premises", cfg.NodePools["on-prem-primary"].Labels["location"])
		assert.Equal(t, "cloud", cfg.NodePools["cloud-burst"].Labels["location"])
		assert.True(t, cfg.NodePools["cloud-burst"].SpotInstance)
		assert.True(t, cfg.NodePools["cloud-burst"].AutoScaling)
		assert.Equal(t, 50, cfg.NodePools["cloud-burst"].MaxCount)

		return nil
	}, pulumi.WithMocks("test", "hybrid-bursting", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_DistributedTracing tests distributed tracing platform
func TestOrchestrator_DistributedTracing(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "tracing-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				Linode: &config.LinodeProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "us-east",
				},
			},
			NodePools: map[string]config.NodePool{
				"tracing": {
					Name:     "tracing",
					Provider: "linode",
					Count:    3,
					Size:     "g6-standard-4",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"role":   "observability",
						"jaeger": "true",
						"tempo":  "true",
					},
				},
			},
			Monitoring: config.MonitoringConfig{
				Enabled:  true,
				Provider: "prometheus",
				Tracing: &config.TracingConfig{
					Provider: "jaeger",
					Endpoint: "jaeger-collector.observability.svc:14268",
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
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.True(t, cfg.Monitoring.Enabled)
		assert.Equal(t, "jaeger", cfg.Monitoring.Tracing.Provider)
		assert.True(t, cfg.Network.ServiceMesh.Tracing)

		return nil
	}, pulumi.WithMocks("test", "tracing", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_CostOptimizationPlatform tests cost optimization strategies
func TestOrchestrator_CostOptimizationPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "cost-optimized-cluster",
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
				"spot-workers": {
					Name:         "spot-workers",
					Provider:     "aws",
					Count:        10,
					MinCount:     5,
					MaxCount:     20,
					Size:         "m5.large",
					Roles:        []string{"worker"},
					SpotInstance: true,
					Labels: map[string]string{
						"cost-tier":     "spot",
						"workload":      "batch",
						"interruptible": "true",
					},
				},
				"on-demand-critical": {
					Name:     "on-demand-critical",
					Provider: "aws",
					Count:    3,
					Size:     "m5.xlarge",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"cost-tier":  "on-demand",
						"workload":   "critical",
						"guaranteed": "true",
					},
				},
				"reserved-baseline": {
					Name:     "reserved-baseline",
					Provider: "aws",
					Count:    5,
					Size:     "m5.large",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"cost-tier": "reserved",
						"workload":  "baseline",
					},
				},
			},
			Cluster: config.ClusterSpec{
				Type:    "rke2",
				Version: "v1.28.0",
				AutoScaling: config.AutoScalingConfig{
					Enabled:   true,
					MinNodes:  8,
					MaxNodes:  28,
					TargetCPU: 70,
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.True(t, cfg.NodePools["spot-workers"].SpotInstance)
		assert.Equal(t, "spot", cfg.NodePools["spot-workers"].Labels["cost-tier"])
		assert.Equal(t, "on-demand", cfg.NodePools["on-demand-critical"].Labels["cost-tier"])
		assert.Equal(t, "reserved", cfg.NodePools["reserved-baseline"].Labels["cost-tier"])

		return nil
	}, pulumi.WithMocks("test", "cost-optimization", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_ChaosengineeringPlatform tests chaos engineering setup
func TestOrchestrator_ChaosEngineeringPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "chaos-testing-cluster",
				Environment: "staging",
			},
			Providers: config.ProvidersConfig{
				GCP: &config.GCPProvider{
					Enabled:     true,
					ProjectID:   "chaos-project",
					Region:      "us-central1",
					Credentials: "test-creds",
				},
			},
			NodePools: map[string]config.NodePool{
				"chaos-pool": {
					Name:     "chaos-pool",
					Provider: "gcp",
					Count:    5,
					Size:     "n1-standard-4",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"chaos-enabled": "true",
						"environment":   "staging",
						"testing":       "chaos",
					},
				},
			},
			Cluster: config.ClusterSpec{
				Type:             "rke2",
				Version:          "v1.28.0",
				HighAvailability: true,
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, "true", cfg.NodePools["chaos-pool"].Labels["chaos-enabled"])
		assert.Equal(t, "staging", cfg.Metadata.Environment)
		assert.True(t, cfg.Cluster.HighAvailability)

		return nil
	}, pulumi.WithMocks("test", "chaos-engineering", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_DataLakePlatform tests big data and data lake infrastructure
func TestOrchestrator_DataLakePlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "datalake-cluster",
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
				"spark-master": {
					Name:     "spark-master",
					Provider: "aws",
					Count:    3,
					Size:     "r5.2xlarge",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"spark-role": "master",
						"workload":   "spark",
					},
				},
				"spark-workers": {
					Name:        "spark-workers",
					Provider:    "aws",
					Count:       10,
					MinCount:    5,
					MaxCount:    50,
					Size:        "r5.4xlarge",
					Roles:       []string{"worker"},
					AutoScaling: true,
					Labels: map[string]string{
						"spark-role":      "worker",
						"workload":        "spark",
						"data-processing": "true",
					},
				},
				"presto-coordinators": {
					Name:     "presto-coordinators",
					Provider: "aws",
					Count:    2,
					Size:     "r5.2xlarge",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"presto-role": "coordinator",
						"workload":    "presto",
					},
				},
			},
			Storage: config.StorageConfig{
				Classes: []config.StorageClass{
					{
						Name:        "datalake-storage",
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
		assert.Equal(t, "master", cfg.NodePools["spark-master"].Labels["spark-role"])
		assert.Equal(t, "worker", cfg.NodePools["spark-workers"].Labels["spark-role"])
		assert.True(t, cfg.NodePools["spark-workers"].AutoScaling)
		assert.Equal(t, 50, cfg.NodePools["spark-workers"].MaxCount)

		return nil
	}, pulumi.WithMocks("test", "datalake", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_GeoReplicationCluster tests geo-distributed data replication
func TestOrchestrator_GeoReplicationCluster(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "geo-replicated-cluster",
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
				"us-east": {
					Name:     "us-east",
					Provider: "aws",
					Count:    5,
					Size:     "m5.xlarge",
					Region:   "us-east-1",
					Zones:    []string{"us-east-1a", "us-east-1b", "us-east-1c"},
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"region":          "us-east",
						"geo-replication": "primary",
					},
				},
				"eu-west": {
					Name:     "eu-west",
					Provider: "aws",
					Count:    5,
					Size:     "m5.xlarge",
					Region:   "eu-west-1",
					Zones:    []string{"eu-west-1a", "eu-west-1b", "eu-west-1c"},
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"region":          "eu-west",
						"geo-replication": "secondary",
					},
				},
				"ap-southeast": {
					Name:     "ap-southeast",
					Provider: "aws",
					Count:    5,
					Size:     "m5.xlarge",
					Region:   "ap-southeast-1",
					Zones:    []string{"ap-southeast-1a", "ap-southeast-1b", "ap-southeast-1c"},
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"region":          "ap-southeast",
						"geo-replication": "secondary",
					},
				},
			},
			Cluster: config.ClusterSpec{
				Type:             "rke2",
				Version:          "v1.28.0",
				HighAvailability: true,
				MultiCloud:       false,
			},
			Network: config.NetworkConfig{
				Mode:                    "vpc",
				CrossProviderNetworking: true,
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, "primary", cfg.NodePools["us-east"].Labels["geo-replication"])
		assert.Equal(t, "secondary", cfg.NodePools["eu-west"].Labels["geo-replication"])
		assert.Len(t, cfg.NodePools["us-east"].Zones, 3)
		assert.True(t, cfg.Network.CrossProviderNetworking)

		return nil
	}, pulumi.WithMocks("test", "geo-replication", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}
