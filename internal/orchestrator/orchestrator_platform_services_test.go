package orchestrator

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// TestOrchestrator_MonitoringAndObservabilityPlatform tests a comprehensive monitoring setup
func TestOrchestrator_MonitoringAndObservabilityPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "monitoring-platform",
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
				"monitoring": {
					Name:     "monitoring",
					Provider: "digitalocean",
					Count:    3,
					Size:     "s-4vcpu-8gb",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"role":         "monitoring",
						"prometheus":   "true",
						"grafana":      "true",
						"alertmanager": "true",
					},
				},
			},
			Monitoring: config.MonitoringConfig{
				Enabled:  true,
				Provider: "prometheus",
				Prometheus: &config.PrometheusConfig{
					Enabled:     true,
					Retention:   "30d",
					StorageSize: "100Gi",
					Replicas:    3,
				},
				Grafana: &config.GrafanaConfig{
					Enabled:       true,
					AdminPassword: "secret",
					Ingress:       true,
					Domain:        "grafana.example.com",
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.True(t, cfg.Monitoring.Enabled)
		assert.True(t, cfg.Monitoring.Prometheus.Enabled)
		assert.Equal(t, "30d", cfg.Monitoring.Prometheus.Retention)
		assert.Equal(t, "monitoring", cfg.NodePools["monitoring"].Labels["role"])

		return nil
	}, pulumi.WithMocks("test", "monitoring", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_CentralizedLoggingPlatform tests logging aggregation setup
func TestOrchestrator_CentralizedLoggingPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "logging-platform",
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
				"logging": {
					Name:     "logging",
					Provider: "linode",
					Count:    3,
					Size:     "g6-standard-4",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"role":          "logging",
						"elasticsearch": "true",
						"fluentd":       "true",
						"kibana":        "true",
					},
				},
			},
			Monitoring: config.MonitoringConfig{
				Enabled:  true,
				Provider: "elasticsearch",
				Logging: &config.LoggingConfig{
					Provider:    "elasticsearch",
					Backend:     "elasticsearch",
					Retention:   "30d",
					Aggregation: true,
					Parsers:     []string{"json", "multiline"},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.True(t, cfg.Monitoring.Enabled)
		assert.Equal(t, "elasticsearch", cfg.Monitoring.Logging.Provider)
		assert.Equal(t, "30d", cfg.Monitoring.Logging.Retention)
		assert.Equal(t, "logging", cfg.NodePools["logging"].Labels["role"])

		return nil
	}, pulumi.WithMocks("test", "logging", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_SecureSecretsConfiguration tests secrets management
func TestOrchestrator_SecureSecretsConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "secure-secrets-cluster",
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
				"workers": {
					Name:     "workers",
					Provider: "aws",
					Count:    3,
					Size:     "t3.medium",
					Roles:    []string{"worker"},
				},
			},
			Security: config.SecurityConfig{
				Secrets: config.SecretsConfig{
					Provider:   "vault",
					Encryption: true,
					KeyManagement: "kms",
					ExternalSecrets: true,
				},
				TLS: config.TLSConfig{
					Enabled:         true,
					CertManager:    true,
					Provider:       "letsencrypt",
					Email:          "admin@example.com",
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, "vault", cfg.Security.Secrets.Provider)
		assert.True(t, cfg.Security.Secrets.Encryption)
		assert.True(t, cfg.Security.TLS.Enabled)

		return nil
	}, pulumi.WithMocks("test", "secrets", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_PrivateRegistryConfiguration tests private container registry
func TestOrchestrator_PrivateRegistryConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "registry-cluster",
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
				"workers": {
					Name:     "workers",
					Provider: "digitalocean",
					Count:    5,
					Size:     "s-2vcpu-4gb",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"registry": "harbor",
					},
				},
			},
			Storage: config.StorageConfig{
				PersistentVolumes: []config.PersistentVolume{
					{
						Name:         "registry-storage",
						Size:         "500Gi",
						StorageClass: "do-block-storage",
						AccessModes:  []string{"ReadWriteOnce"},
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Len(t, cfg.Storage.PersistentVolumes, 1)
		assert.Equal(t, "registry-storage", cfg.Storage.PersistentVolumes[0].Name)
		assert.Equal(t, "500Gi", cfg.Storage.PersistentVolumes[0].Size)

		return nil
	}, pulumi.WithMocks("test", "registry", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_BackupAndRestorePlatform tests backup solution integration
func TestOrchestrator_BackupAndRestorePlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "backup-platform",
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
				"workers": {
					Name:     "workers",
					Provider: "linode",
					Count:    3,
					Size:     "g6-standard-2",
					Roles:    []string{"worker"},
				},
			},
			Cluster: config.ClusterSpec{
				Type:             "rke2",
				Version:          "v1.28.0",
				HighAvailability: true,
				BackupConfig: config.BackupConfig{
					Enabled:        true,
					Provider:       "velero",
					Schedule:       "0 2 * * *",
					Retention:      30,
					Location:       "s3://cluster-backups",
					IncludeEtcd:    true,
					IncludeVolumes: true,
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.True(t, cfg.Cluster.BackupConfig.Enabled)
		assert.Equal(t, "velero", cfg.Cluster.BackupConfig.Provider)
		assert.Equal(t, "0 2 * * *", cfg.Cluster.BackupConfig.Schedule)
		assert.Equal(t, 30, cfg.Cluster.BackupConfig.Retention)
		assert.True(t, cfg.Cluster.BackupConfig.IncludeEtcd)

		return nil
	}, pulumi.WithMocks("test", "backup", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_AutoScalingConfiguration tests cluster autoscaler
func TestOrchestrator_AutoScalingConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "autoscaling-cluster",
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
				"autoscaling-workers": {
					Name:        "autoscaling-workers",
					Provider:    "aws",
					Count:       3,
					MinCount:    3,
					MaxCount:    10,
					Size:        "t3.large",
					Roles:       []string{"worker"},
					AutoScaling: true,
					Labels: map[string]string{
						"autoscaling": "enabled",
					},
				},
			},
			Cluster: config.ClusterSpec{
				Type:    "rke2",
				Version: "v1.28.0",
				AutoScaling: config.AutoScalingConfig{
					Enabled:           true,
					MinNodes:          3,
					MaxNodes:          10,
					TargetCPU:    70,
					ScaleDown:    "5m",
					ScaleUp:      "1m",
					
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.True(t, cfg.NodePools["autoscaling-workers"].AutoScaling)
		assert.Equal(t, 3, cfg.NodePools["autoscaling-workers"].MinCount)
		assert.Equal(t, 10, cfg.NodePools["autoscaling-workers"].MaxCount)
		assert.True(t, cfg.Cluster.AutoScaling.Enabled)
		assert.Equal(t, 70, cfg.Cluster.AutoScaling.TargetCPU)

		return nil
	}, pulumi.WithMocks("test", "autoscaling", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_HighAvailabilityMonitoring tests HA monitoring stack
func TestOrchestrator_HighAvailabilityMonitoring(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "ha-monitoring-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				GCP: &config.GCPProvider{
					Enabled:     true,
					ProjectID:   "test-project",
					Region:      "us-central1",
					Credentials: "test-credentials",
				},
			},
			NodePools: map[string]config.NodePool{
				"monitoring-ha": {
					Name:     "monitoring-ha",
					Provider: "gcp",
					Count:    5,
					Size:     "n1-standard-4",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"monitoring": "ha",
					},
				},
			},
			Monitoring: config.MonitoringConfig{
				Enabled:  true,
				Provider: "prometheus",
				Prometheus: &config.PrometheusConfig{
					Enabled:        true,
					Retention:      "90d",
					StorageSize:    "500Gi",
					Replicas:       3,
					ScrapeInterval: "30s",
				},
				AlertManager: &config.AlertManagerConfig{
					Enabled:  true,
					Replicas: 3,
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
		assert.True(t, cfg.Cluster.HighAvailability)
		assert.Equal(t, 3, cfg.Monitoring.Prometheus.Replicas)
		assert.Equal(t, 3, cfg.Monitoring.AlertManager.Replicas)
		assert.Equal(t, "90d", cfg.Monitoring.Prometheus.Retention)

		return nil
	}, pulumi.WithMocks("test", "ha-monitoring", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_ServiceDiscoveryPlatform tests service discovery and DNS
func TestOrchestrator_ServiceDiscoveryPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "service-discovery",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				Azure: &config.AzureProvider{
					Enabled:        true,
					SubscriptionID: "test-subscription",
					TenantID:       "test-tenant",
					ClientID:       "test-client",
					ClientSecret:   "test-secret",
					Location:       "eastus",
				},
			},
			NodePools: map[string]config.NodePool{
				"workers": {
					Name:     "workers",
					Provider: "azure",
					Count:    3,
					Size:     "Standard_D4s_v3",
					Roles:    []string{"worker"},
				},
			},
			Network: config.NetworkConfig{
				DNS: config.DNSConfig{
					Domain:      "cluster.local",
					Servers:     []string{"8.8.8.8", "8.8.4.4"},
					ExternalDNS: true,
					Provider:    "azure",
				},
				ServiceMesh: &config.ServiceMeshConfig{
					Type:    "consul",
					Version: "1.16.0",
					MTLS:    true,
					Tracing: true,
				},
			},
			Kubernetes: config.KubernetesConfig{
				Version:       "v1.28.0",
				Distribution:  "rke2",
				ClusterDomain: "cluster.local",
				ClusterDNS:    "10.43.0.10",
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.True(t, cfg.Network.DNS.ExternalDNS)
		assert.Equal(t, "cluster.local", cfg.Network.DNS.Domain)
		assert.Equal(t, "consul", cfg.Network.ServiceMesh.Type)
		assert.True(t, cfg.Network.ServiceMesh.MTLS)

		return nil
	}, pulumi.WithMocks("test", "service-discovery", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_APIGatewayConfiguration tests API gateway integration
func TestOrchestrator_APIGatewayConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "api-gateway-cluster",
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
				"gateway": {
					Name:     "gateway",
					Provider: "digitalocean",
					Count:    3,
					Size:     "s-4vcpu-8gb",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"role":    "gateway",
						"kong":    "true",
						"ingress": "true",
					},
				},
			},
			Network: config.NetworkConfig{
				Ingress: config.IngressConfig{
					Controller: "kong",
					Class:      "kong",
					TLS:        true,
					Replicas:   3,
				},
				LoadBalancers: []config.LoadBalancerConfig{{
					Name:     "api-gateway-lb",
					Provider: "digitalocean",
					Type:     "external",
				}},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, "kong", cfg.Network.Ingress.Controller)
		assert.Equal(t, "kong", cfg.Network.Ingress.Class)
		assert.True(t, cfg.Network.Ingress.TLS)
		assert.Equal(t, "gateway", cfg.NodePools["gateway"].Labels["role"])

		return nil
	}, pulumi.WithMocks("test", "api-gateway", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_MessageQueueIntegration tests message queue services
func TestOrchestrator_MessageQueueIntegration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "message-queue-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				Linode: &config.LinodeProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "us-west",
				},
			},
			NodePools: map[string]config.NodePool{
				"rabbitmq": {
					Name:     "rabbitmq",
					Provider: "linode",
					Count:    3,
					Size:     "g6-standard-4",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"role":     "messaging",
						"rabbitmq": "true",
					},
				},
				"redis": {
					Name:     "redis",
					Provider: "linode",
					Count:    3,
					Size:     "g6-standard-2",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"role":  "cache",
						"redis": "true",
					},
				},
			},
			Storage: config.StorageConfig{
				PersistentVolumes: []config.PersistentVolume{
					{
						Name:         "rabbitmq-data",
						Size:         "100Gi",
						StorageClass: "linode-block-storage-retain",
						AccessModes:  []string{"ReadWriteOnce"},
					},
					{
						Name:         "redis-data",
						Size:         "50Gi",
						StorageClass: "linode-block-storage-retain",
						AccessModes:  []string{"ReadWriteOnce"},
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, "messaging", cfg.NodePools["rabbitmq"].Labels["role"])
		assert.Equal(t, "cache", cfg.NodePools["redis"].Labels["role"])
		assert.Len(t, cfg.Storage.PersistentVolumes, 2)
		assert.Equal(t, "rabbitmq-data", cfg.Storage.PersistentVolumes[0].Name)

		return nil
	}, pulumi.WithMocks("test", "message-queue", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_CachingLayerConfiguration tests distributed caching setup
func TestOrchestrator_CachingLayerConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "caching-cluster",
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
				"cache-nodes": {
					Name:     "cache-nodes",
					Provider: "aws",
					Count:    5,
					Size:     "r5.xlarge",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"role":      "cache",
						"redis":     "true",
						"memcached": "true",
					},
					Taints: []config.TaintConfig{
						{
							Key:    "workload",
							Value:  "cache",
							Effect: "NoSchedule",
						},
					},
				},
			},
			Storage: config.StorageConfig{
				Classes: []config.StorageClass{
					{
						Name:        "fast-ssd",
						Provisioner: "ebs.csi.aws.com",
						Parameters: map[string]string{
							"type":       "gp3",
							"iops":       "16000",
							"throughput": "1000",
						},
						ReclaimPolicy: "Retain",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, "cache", cfg.NodePools["cache-nodes"].Labels["role"])
		assert.Equal(t, "r5.xlarge", cfg.NodePools["cache-nodes"].Size)
		assert.Len(t, cfg.NodePools["cache-nodes"].Taints, 1)
		assert.Equal(t, "workload", cfg.NodePools["cache-nodes"].Taints[0].Key)
		assert.Len(t, cfg.Storage.Classes, 1)
		assert.Equal(t, "fast-ssd", cfg.Storage.Classes[0].Name)

		return nil
	}, pulumi.WithMocks("test", "caching", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_GitOpsDeploymentPlatform tests GitOps with ArgoCD
func TestOrchestrator_GitOpsDeploymentPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "gitops-platform",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				GCP: &config.GCPProvider{
					Enabled:     true,
					ProjectID:   "test-project",
					Region:      "us-central1",
					Credentials: "test-credentials",
				},
			},
			NodePools: map[string]config.NodePool{
				"workers": {
					Name:     "workers",
					Provider: "gcp",
					Count:    3,
					Size:     "n1-standard-2",
					Roles:    []string{"worker"},
				},
			},
			Addons: config.AddonsConfig{
				ArgoCD: &config.ArgoCDConfig{
					Enabled:          true,
					Version:          "v2.9.0",
					GitOpsRepoURL:    "https://github.com/example/config-repo",
					GitOpsRepoBranch: "main",
					AppsPath:         "applications/",
					Namespace:        "argocd",
					AdminPassword:    "secret-password",
				},
			},
			Security: config.SecurityConfig{
				RBAC: config.RBACConfig{
					Enabled: true,
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.True(t, cfg.Addons.ArgoCD.Enabled)
		assert.Equal(t, "v2.9.0", cfg.Addons.ArgoCD.Version)
		assert.Equal(t, "main", cfg.Addons.ArgoCD.GitOpsRepoBranch)
		assert.Equal(t, "https://github.com/example/config-repo", cfg.Addons.ArgoCD.GitOpsRepoURL)
		assert.True(t, cfg.Security.RBAC.Enabled)

		return nil
	}, pulumi.WithMocks("test", "gitops", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}
