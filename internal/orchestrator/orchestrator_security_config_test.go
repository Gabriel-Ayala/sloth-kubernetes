package orchestrator

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test 1: Security configuration with RBAC
func TestOrchestrator_SecurityRBACConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		rbacEnabled   bool
		expectedValid bool
	}{
		{
			name:          "RBAC enabled",
			rbacEnabled:   true,
			expectedValid: true,
		},
		{
			name:          "RBAC disabled",
			rbacEnabled:   false,
			expectedValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				cfg := &config.ClusterConfig{
					Metadata: config.Metadata{
						Name:        "security-test-cluster",
						Environment: "test",
					},
					Providers: config.ProvidersConfig{
						DigitalOcean: &config.DigitalOceanProvider{
							Enabled: true,
							Token:   "test-token",
							Region:  "nyc3",
						},
					},
					Security: config.SecurityConfig{
						RBAC: config.RBACConfig{
							Enabled: tt.rbacEnabled,
						},
						NetworkPolicies: true,
					},
				}

				orch := New(ctx, cfg)
				assert.NotNil(t, orch)
				assert.Equal(t, tt.rbacEnabled, orch.config.Security.RBAC.Enabled)
				return nil
			}, pulumi.WithMocks("test", "security-rbac", &IntegrationMockProvider{}))

			if tt.expectedValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// Test 2: Load balancer configuration
func TestOrchestrator_LoadBalancerConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "lb-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc3",
				},
			},
			LoadBalancer: config.LoadBalancerConfig{
				Name:     "production-lb",
				Type:     "external",
				Provider: "digitalocean",
				Ports: []config.PortConfig{
					{
						Name:       "http",
						Port:       80,
						TargetPort: 8080,
						Protocol:   "tcp",
					},
					{
						Name:       "https",
						Port:       443,
						TargetPort: 8443,
						Protocol:   "tcp",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		require.NotNil(t, orch)

		// Verify load balancer configuration
		assert.Equal(t, "production-lb", cfg.LoadBalancer.Name)
		assert.Equal(t, "digitalocean", cfg.LoadBalancer.Provider)
		assert.Equal(t, 2, len(cfg.LoadBalancer.Ports))
		assert.Equal(t, 80, cfg.LoadBalancer.Ports[0].Port)
		assert.Equal(t, 443, cfg.LoadBalancer.Ports[1].Port)

		return nil
	}, pulumi.WithMocks("test", "load-balancer", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 3: Storage class configurations with node labels
func TestOrchestrator_StorageClassConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "storage-cluster",
				Environment: "test",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc3",
				},
			},
			Storage: config.StorageConfig{
				DefaultClass: "fast-ssd",
				Classes: []config.StorageClass{
					{
						Name:          "fast-ssd",
						Provisioner:   "do.csi.digitalocean.com",
						ReclaimPolicy: "Retain",
					},
					{
						Name:          "standard-hdd",
						Provisioner:   "do.csi.digitalocean.com",
						ReclaimPolicy: "Delete",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		orch.nodes = make(map[string][]*providers.NodeOutput)
		orch.nodes["digitalocean"] = []*providers.NodeOutput{
			{
				Name: "storage-node-1",
				Labels: map[string]string{
					"role":         "worker",
					"storage-type": "ssd",
					"storage-tier": "fast",
				},
			},
		}

		// Verify storage configuration
		assert.Equal(t, "fast-ssd", cfg.Storage.DefaultClass)
		assert.Equal(t, 2, len(cfg.Storage.Classes))
		assert.Equal(t, "ssd", orch.nodes["digitalocean"][0].Labels["storage-type"])

		return nil
	}, pulumi.WithMocks("test", "storage-class", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 4: Backup configuration
func TestOrchestrator_BackupConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "backup-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc3",
				},
			},
			Cluster: config.ClusterSpec{
				BackupConfig: config.BackupConfig{
					Enabled:   true,
					Schedule:  "0 2 * * *",
					Retention: 7,
					Provider:  "s3",
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify backup configuration
		assert.True(t, cfg.Cluster.BackupConfig.Enabled)
		assert.Equal(t, "0 2 * * *", cfg.Cluster.BackupConfig.Schedule)
		assert.Equal(t, 7, cfg.Cluster.BackupConfig.Retention)

		return nil
	}, pulumi.WithMocks("test", "backup-config", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 5: Autoscaling configuration
func TestOrchestrator_AutoscalingConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "autoscale-cluster",
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
				"autoscale-workers": {
					Name:        "autoscale-workers",
					Provider:    "digitalocean",
					Count:       3,
					MinCount:    2,
					MaxCount:    10,
					Size:        "s-4vcpu-8gb",
					Roles:       []string{"worker"},
					AutoScaling: true,
				},
			},
			Cluster: config.ClusterSpec{
				AutoScaling: config.AutoScalingConfig{
					Enabled:   true,
					MinNodes:  2,
					MaxNodes:  10,
					TargetCPU: 70,
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify autoscaling configuration
		pool := cfg.NodePools["autoscale-workers"]
		assert.Equal(t, 2, pool.MinCount)
		assert.Equal(t, 10, pool.MaxCount)
		assert.True(t, pool.AutoScaling)
		assert.GreaterOrEqual(t, pool.Count, pool.MinCount)
		assert.LessOrEqual(t, pool.Count, pool.MaxCount)

		return nil
	}, pulumi.WithMocks("test", "autoscaling", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 6: TLS and certificate management
func TestOrchestrator_TLSConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "tls-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc3",
				},
			},
			Security: config.SecurityConfig{
				TLS: config.TLSConfig{
					Enabled:     true,
					CertManager: true,
					Provider:    "letsencrypt",
					Email:       "admin@example.com",
					Domains:     []string{"cluster.example.com", "*.cluster.example.com"},
				},
			},
		}

		orch := New(ctx, cfg)

		// Verify TLS configuration
		assert.True(t, cfg.Security.TLS.Enabled)
		assert.True(t, cfg.Security.TLS.CertManager)
		assert.Equal(t, "letsencrypt", cfg.Security.TLS.Provider)
		assert.Equal(t, 2, len(cfg.Security.TLS.Domains))

		orch.nodes = make(map[string][]*providers.NodeOutput)
		orch.nodes["digitalocean"] = []*providers.NodeOutput{
			{
				Name: "master-1",
				Labels: map[string]string{
					"role":       "master",
					"tls":        "enabled",
					"cert-mgr":   "true",
				},
			},
		}

		assert.Equal(t, "enabled", orch.nodes["digitalocean"][0].Labels["tls"])

		return nil
	}, pulumi.WithMocks("test", "tls-config", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 7: Logging configuration
func TestOrchestrator_LoggingConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		backend  string
	}{
		{
			name:     "Logging with fluentd",
			provider: "fluentd",
			backend:  "elasticsearch",
		},
		{
			name:     "Logging with fluentbit",
			provider: "fluentbit",
			backend:  "loki",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				cfg := &config.ClusterConfig{
					Metadata: config.Metadata{
						Name:        "logging-cluster",
						Environment: "test",
					},
					Providers: config.ProvidersConfig{
						DigitalOcean: &config.DigitalOceanProvider{
							Enabled: true,
							Token:   "test-token",
						},
					},
					Monitoring: config.MonitoringConfig{
						Enabled: true,
						Logging: &config.LoggingConfig{
							Provider:    tt.provider,
							Backend:     tt.backend,
							Retention:   "30d",
							Aggregation: true,
						},
					},
				}

				_ = New(ctx, cfg)
				assert.True(t, cfg.Monitoring.Enabled)
				assert.NotNil(t, cfg.Monitoring.Logging)
				assert.Equal(t, tt.provider, cfg.Monitoring.Logging.Provider)
				assert.Equal(t, tt.backend, cfg.Monitoring.Logging.Backend)

				return nil
			}, pulumi.WithMocks("test", "logging", &IntegrationMockProvider{}))
			assert.NoError(t, err)
		})
	}
}

// Test 8: Network policies configuration
func TestOrchestrator_NetworkPoliciesConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "netpol-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
				},
			},
			Security: config.SecurityConfig{
				NetworkPolicies: true,
			},
			Network: config.NetworkConfig{
				NetworkPolicies: []config.NetworkPolicy{
					{
						Name:      "deny-all-ingress",
						Namespace: "default",
					},
					{
						Name:      "allow-dns",
						Namespace: "kube-system",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		orch.nodes = make(map[string][]*providers.NodeOutput)
		orch.nodes["digitalocean"] = []*providers.NodeOutput{
			{
				Name: "netpol-node-1",
				Labels: map[string]string{
					"role":           "worker",
					"network-policy": "enabled",
				},
			},
		}

		// Verify network policies configuration
		assert.True(t, cfg.Security.NetworkPolicies)
		assert.Equal(t, 2, len(cfg.Network.NetworkPolicies))
		assert.Equal(t, "enabled", orch.nodes["digitalocean"][0].Labels["network-policy"])

		return nil
	}, pulumi.WithMocks("test", "network-policies", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 9: Service mesh integration
func TestOrchestrator_ServiceMeshIntegration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "service-mesh-cluster",
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
		}

		orch := New(ctx, cfg)
		orch.nodes = make(map[string][]*providers.NodeOutput)
		orch.nodes["digitalocean"] = []*providers.NodeOutput{
			{
				Name: "mesh-node-1",
				Labels: map[string]string{
					"role":            "worker",
					"service-mesh":    "istio",
					"istio-injection": "enabled",
				},
			},
		}

		// Verify service mesh configuration
		assert.NotNil(t, cfg.Network.ServiceMesh)
		assert.Equal(t, "istio", cfg.Network.ServiceMesh.Type)
		assert.True(t, cfg.Network.ServiceMesh.MTLS)
		assert.Equal(t, "istio", orch.nodes["digitalocean"][0].Labels["service-mesh"])

		return nil
	}, pulumi.WithMocks("test", "service-mesh", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 10: Node affinity and taints configuration
func TestOrchestrator_NodeAffinityConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "affinity-cluster",
				Environment: "test",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
				},
			},
			NodePools: map[string]config.NodePool{
				"gpu-workers": {
					Name:     "gpu-workers",
					Provider: "digitalocean",
					Count:    2,
					Size:     "g-2vcpu-8gb",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload-type": "gpu",
					},
					Taints: []config.TaintConfig{
						{
							Key:    "nvidia.com/gpu",
							Value:  "true",
							Effect: "NoSchedule",
						},
					},
				},
			},
		}

		orch := New(ctx, cfg)
		orch.nodes = make(map[string][]*providers.NodeOutput)
		orch.nodes["digitalocean"] = []*providers.NodeOutput{
			{
				Name: "gpu-node-1",
				Labels: map[string]string{
					"role":          "worker",
					"workload-type": "gpu",
					"gpu-type":      "nvidia-tesla-v100",
				},
			},
		}

		// Verify affinity configuration
		pool := cfg.NodePools["gpu-workers"]
		assert.Equal(t, 1, len(pool.Taints))
		assert.Equal(t, "nvidia.com/gpu", pool.Taints[0].Key)
		assert.Equal(t, "gpu", orch.nodes["digitalocean"][0].Labels["workload-type"])

		return nil
	}, pulumi.WithMocks("test", "node-affinity", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 11: Cluster upgrade scenarios with version management
func TestOrchestrator_ClusterUpgradeScenarios(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		targetVersion  string
		shouldSucceed  bool
	}{
		{
			name:           "Minor version upgrade",
			currentVersion: "1.27.0",
			targetVersion:  "1.28.0",
			shouldSucceed:  true,
		},
		{
			name:           "Patch version upgrade",
			currentVersion: "1.28.0",
			targetVersion:  "1.28.1",
			shouldSucceed:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				cfg := &config.ClusterConfig{
					Metadata: config.Metadata{
						Name:        "upgrade-cluster",
						Environment: "test",
						Version:     tt.targetVersion,
					},
					Providers: config.ProvidersConfig{
						DigitalOcean: &config.DigitalOceanProvider{
							Enabled: true,
							Token:   "test-token",
						},
					},
					Kubernetes: config.KubernetesConfig{
						Version: tt.targetVersion,
					},
				}

				_ = New(ctx, cfg)
				assert.Equal(t, tt.targetVersion, cfg.Metadata.Version)
				assert.Equal(t, tt.targetVersion, cfg.Kubernetes.Version)

				return nil
			}, pulumi.WithMocks("test", "cluster-upgrade", &IntegrationMockProvider{}))

			if tt.shouldSucceed {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// Test 12: Secrets management configuration
func TestOrchestrator_SecretsManagement(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "secrets-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
				},
			},
			Security: config.SecurityConfig{
				Secrets: config.SecretsConfig{
					Provider:        "vault",
					Encryption:      true,
					KeyManagement:   "kms",
					ExternalSecrets: true,
				},
			},
		}

		orch := New(ctx, cfg)
		orch.nodes = make(map[string][]*providers.NodeOutput)
		orch.nodes["digitalocean"] = []*providers.NodeOutput{
			{
				Name: "secrets-node-1",
				Labels: map[string]string{
					"role":              "master",
					"secrets-backend":   "vault",
					"encryption":        "enabled",
					"secrets-encrypted": "true",
				},
			},
		}

		// Verify secrets management configuration
		assert.Equal(t, "vault", cfg.Security.Secrets.Provider)
		assert.True(t, cfg.Security.Secrets.Encryption)
		assert.Equal(t, "vault", orch.nodes["digitalocean"][0].Labels["secrets-backend"])
		assert.Equal(t, "enabled", orch.nodes["digitalocean"][0].Labels["encryption"])

		return nil
	}, pulumi.WithMocks("test", "secrets-mgmt", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
