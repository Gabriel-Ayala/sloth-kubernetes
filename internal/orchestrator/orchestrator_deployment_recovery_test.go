package orchestrator

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// Test 1: Rolling update configuration
func TestOrchestrator_RollingUpdateConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "rolling-update-cluster",
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
					Size:     "s-4vcpu-8gb",
					Roles:    []string{"worker"},
				},
			},
		}

		orch := New(ctx, cfg)
		orch.nodes = make(map[string][]*providers.NodeOutput)

		// Simulate rolling update by versioning nodes
		orch.nodes["digitalocean"] = []*providers.NodeOutput{
			{
				Name: "worker-1",
				Labels: map[string]string{
					"role":    "worker",
					"version": "v1.28.0",
					"update":  "completed",
				},
			},
			{
				Name: "worker-2",
				Labels: map[string]string{
					"role":    "worker",
					"version": "v1.28.0",
					"update":  "in-progress",
				},
			},
			{
				Name: "worker-3",
				Labels: map[string]string{
					"role":    "worker",
					"version": "v1.27.0",
					"update":  "pending",
				},
			},
		}

		// Verify rolling update state
		updatedCount := 0
		for _, node := range orch.nodes["digitalocean"] {
			if node.Labels["version"] == "v1.28.0" {
				updatedCount++
			}
		}

		assert.Equal(t, 2, updatedCount)
		assert.Equal(t, 3, len(orch.nodes["digitalocean"]))

		return nil
	}, pulumi.WithMocks("test", "rolling-update", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 2: Disaster recovery with backup restoration
func TestOrchestrator_DisasterRecoveryConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "dr-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc3",
				},
				Linode: &config.LinodeProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "us-east",
				},
			},
			Cluster: config.ClusterSpec{
				BackupConfig: config.BackupConfig{
					Enabled:     true,
					Schedule:    "0 */6 * * *",
					Retention:   30,
					Provider:    "s3",
					Location:    "s3://cluster-backups/dr-cluster",
					IncludeEtcd: true,
				},
			},
		}

		orch := New(ctx, cfg)

		// Verify DR configuration
		assert.True(t, cfg.Cluster.BackupConfig.Enabled)
		assert.True(t, cfg.Cluster.BackupConfig.IncludeEtcd)
		assert.Equal(t, 30, cfg.Cluster.BackupConfig.Retention)
		assert.NotNil(t, cfg.Providers.DigitalOcean)
		assert.NotNil(t, cfg.Providers.Linode)

		orch.nodes = make(map[string][]*providers.NodeOutput)
		orch.nodes["digitalocean"] = []*providers.NodeOutput{
			{
				Name: "primary-master",
				Labels: map[string]string{
					"role":   "master",
					"site":   "primary",
					"backup": "enabled",
				},
			},
		}
		orch.nodes["linode"] = []*providers.NodeOutput{
			{
				Name: "dr-master",
				Labels: map[string]string{
					"role": "master",
					"site": "dr",
				},
			},
		}

		// Verify multi-site setup for DR
		assert.Equal(t, 1, len(orch.nodes["digitalocean"]))
		assert.Equal(t, 1, len(orch.nodes["linode"]))

		return nil
	}, pulumi.WithMocks("test", "disaster-recovery", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 3: Ingress controller configuration
func TestOrchestrator_IngressControllerConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		controller string
		tls        bool
	}{
		{
			name:       "Nginx Ingress with TLS",
			controller: "nginx",
			tls:        true,
		},
		{
			name:       "Traefik Ingress",
			controller: "traefik",
			tls:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				cfg := &config.ClusterConfig{
					Metadata: config.Metadata{
						Name:        "ingress-cluster",
						Environment: "production",
					},
					Providers: config.ProvidersConfig{
						DigitalOcean: &config.DigitalOceanProvider{
							Enabled: true,
							Token:   "test-token",
							Region:  "nyc3",
						},
					},
					Network: config.NetworkConfig{
						Ingress: config.IngressConfig{
							Controller: tt.controller,
							Class:      tt.controller,
							TLS:        tt.tls,
							Replicas:   3,
						},
					},
				}

				_ = New(ctx, cfg)

				assert.Equal(t, tt.controller, cfg.Network.Ingress.Controller)
				assert.Equal(t, tt.tls, cfg.Network.Ingress.TLS)
				assert.Equal(t, 3, cfg.Network.Ingress.Replicas)

				return nil
			}, pulumi.WithMocks("test", "ingress-controller", &IntegrationMockProvider{}))
			assert.NoError(t, err)
		})
	}
}

// Test 4: DNS configuration and management
func TestOrchestrator_DNSConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "dns-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc3",
				},
			},
			Network: config.NetworkConfig{
				DNS: config.DNSConfig{
					Domain:      "cluster.example.com",
					Servers:     []string{"8.8.8.8", "8.8.4.4"},
					ExternalDNS: true,
					Provider:    "digitalocean",
				},
			},
		}

		orch := New(ctx, cfg)
		orch.nodes = make(map[string][]*providers.NodeOutput)
		orch.nodes["digitalocean"] = []*providers.NodeOutput{
			{
				Name: "dns-node-1",
				Labels: map[string]string{
					"role":         "worker",
					"dns-provider": "digitalocean",
					"external-dns": "enabled",
				},
			},
		}

		// Verify DNS configuration
		assert.Equal(t, "cluster.example.com", cfg.Network.DNS.Domain)
		assert.True(t, cfg.Network.DNS.ExternalDNS)
		assert.Equal(t, 2, len(cfg.Network.DNS.Servers))
		assert.Equal(t, "enabled", orch.nodes["digitalocean"][0].Labels["external-dns"])

		return nil
	}, pulumi.WithMocks("test", "dns-config", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 5: Persistent volume configuration
func TestOrchestrator_PersistentVolumeConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "pv-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc3",
				},
			},
			Storage: config.StorageConfig{
				DefaultClass: "do-block-storage",
				Classes: []config.StorageClass{
					{
						Name:              "do-block-storage",
						Provisioner:       "dobs.csi.digitalocean.com",
						ReclaimPolicy:     "Retain",
						VolumeBindingMode: "WaitForFirstConsumer",
					},
				},
				PersistentVolumes: []config.PersistentVolume{
					{
						Name:         "data-volume-1",
						Size:         "100Gi",
						StorageClass: "do-block-storage",
						AccessModes:  []string{"ReadWriteOnce"},
					},
					{
						Name:         "shared-volume-1",
						Size:         "50Gi",
						StorageClass: "do-block-storage",
						AccessModes:  []string{"ReadWriteMany"},
					},
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify persistent volume configuration
		assert.Equal(t, "do-block-storage", cfg.Storage.DefaultClass)
		assert.Equal(t, 2, len(cfg.Storage.PersistentVolumes))
		assert.Equal(t, "100Gi", cfg.Storage.PersistentVolumes[0].Size)
		assert.Equal(t, "ReadWriteMany", cfg.Storage.PersistentVolumes[1].AccessModes[0])

		return nil
	}, pulumi.WithMocks("test", "persistent-volumes", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 6: Kubernetes addons configuration
func TestOrchestrator_KubernetesAddonsConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "addons-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc3",
				},
			},
			Kubernetes: config.KubernetesConfig{
				Version:       "1.28.0",
				Distribution:  "rke2",
				NetworkPlugin: "calico",
				Addons: []config.AddonConfig{
					{
						Name:      "cert-manager",
						Enabled:   true,
						Version:   "v1.13.0",
						Namespace: "cert-manager",
					},
					{
						Name:      "metrics-server",
						Enabled:   true,
						Version:   "v0.6.4",
						Namespace: "kube-system",
					},
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify Kubernetes addons configuration
		assert.Equal(t, "calico", cfg.Kubernetes.NetworkPlugin)
		assert.Equal(t, 2, len(cfg.Kubernetes.Addons))
		assert.Equal(t, "cert-manager", cfg.Kubernetes.Addons[0].Name)
		assert.True(t, cfg.Kubernetes.Addons[0].Enabled)

		return nil
	}, pulumi.WithMocks("test", "k8s-addons", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 7: Resource quotas and limits
func TestOrchestrator_ResourceQuotasConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "quota-cluster",
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
				"limited-workers": {
					Name:     "limited-workers",
					Provider: "digitalocean",
					Count:    3,
					Size:     "s-2vcpu-4gb",
					Roles:    []string{"worker"},
				},
			},
		}

		orch := New(ctx, cfg)
		orch.nodes = make(map[string][]*providers.NodeOutput)
		orch.nodes["digitalocean"] = []*providers.NodeOutput{
			{
				Name: "quota-worker-1",
				Labels: map[string]string{
					"role":         "worker",
					"cpu-limit":    "2000m",
					"memory-limit": "4Gi",
					"quota":        "enabled",
				},
			},
		}

		// Verify resource quota configuration
		pool := cfg.NodePools["limited-workers"]
		assert.Equal(t, "s-2vcpu-4gb", pool.Size)
		assert.Equal(t, "enabled", orch.nodes["digitalocean"][0].Labels["quota"])

		return nil
	}, pulumi.WithMocks("test", "resource-quotas", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 8: Advanced scheduling configuration
func TestOrchestrator_AdvancedSchedulingConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "scheduling-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc3",
				},
			},
			Kubernetes: config.KubernetesConfig{
				Scheduler: config.SchedulerConfig{
					Profile: "low-node-utilization",
				},
			},
			NodePools: map[string]config.NodePool{
				"priority-workers": {
					Name:     "priority-workers",
					Provider: "digitalocean",
					Count:    3,
					Size:     "s-4vcpu-8gb",
					Roles:    []string{"worker"},
					Labels: map[string]string{
						"workload-priority": "high",
						"scheduling":        "preemptible",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		orch.nodes = make(map[string][]*providers.NodeOutput)
		orch.nodes["digitalocean"] = []*providers.NodeOutput{
			{
				Name: "sched-worker-1",
				Labels: map[string]string{
					"role":              "worker",
					"workload-priority": "high",
					"scheduling":        "preemptible",
					"priority-class":    "system-cluster-critical",
				},
			},
		}

		// Verify scheduling configuration
		assert.Equal(t, "low-node-utilization", cfg.Kubernetes.Scheduler.Profile)
		assert.Equal(t, "high", orch.nodes["digitalocean"][0].Labels["workload-priority"])

		return nil
	}, pulumi.WithMocks("test", "advanced-scheduling", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 9: Observability stack configuration
func TestOrchestrator_ObservabilityStackConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "observability-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc3",
				},
			},
			Monitoring: config.MonitoringConfig{
				Enabled:  true,
				Provider: "prometheus",
				Prometheus: &config.PrometheusConfig{
					Enabled:        true,
					Retention:      "15d",
					StorageSize:    "50Gi",
					Replicas:       2,
					ScrapeInterval: "30s",
				},
				Grafana: &config.GrafanaConfig{
					Enabled: true,
					Ingress: true,
					Domain:  "grafana.cluster.example.com",
				},
				Tracing: &config.TracingConfig{
					Provider: "jaeger",
					Endpoint: "http://jaeger-collector:14268",
					Sampling: 0.1,
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify observability stack
		assert.True(t, cfg.Monitoring.Enabled)
		assert.NotNil(t, cfg.Monitoring.Prometheus)
		assert.Equal(t, "15d", cfg.Monitoring.Prometheus.Retention)
		assert.NotNil(t, cfg.Monitoring.Grafana)
		assert.True(t, cfg.Monitoring.Grafana.Enabled)
		assert.NotNil(t, cfg.Monitoring.Tracing)
		assert.Equal(t, "jaeger", cfg.Monitoring.Tracing.Provider)

		return nil
	}, pulumi.WithMocks("test", "observability", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 10: Compliance and audit configuration
func TestOrchestrator_ComplianceAuditConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "compliance-cluster",
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
				Compliance: config.ComplianceConfig{
					Standards: []string{"pci-dss", "hipaa", "soc2"},
					Scanning:  true,
					Reporting: true,
				},
				Audit: config.AuditConfig{
					Enabled:  true,
					Level:    "RequestResponse",
					Backend:  "webhook",
					Rotation: "daily",
				},
			},
		}

		_ = New(ctx, cfg)

		// Verify compliance and audit configuration
		assert.Equal(t, 3, len(cfg.Security.Compliance.Standards))
		assert.True(t, cfg.Security.Compliance.Scanning)
		assert.True(t, cfg.Security.Audit.Enabled)
		assert.Equal(t, "RequestResponse", cfg.Security.Audit.Level)

		return nil
	}, pulumi.WithMocks("test", "compliance-audit", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 11: Etcd cluster configuration
func TestOrchestrator_EtcdClusterConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "etcd-cluster",
				Environment: "production",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc3",
				},
			},
			Kubernetes: config.KubernetesConfig{
				Etcd: config.EtcdConfig{
					BackupRetention: 7,
					Snapshot: &config.SnapshotConfig{
						Schedule:  "0 */12 * * *",
						Retention: 14,
						S3Bucket:  "etcd-backups-bucket",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		orch.nodes = make(map[string][]*providers.NodeOutput)
		orch.nodes["digitalocean"] = []*providers.NodeOutput{
			{
				Name: "etcd-1",
				Labels: map[string]string{
					"role":            "master",
					"etcd":            "true",
					"etcd-member":     "etcd-1",
					"backup-enabled":  "true",
				},
			},
			{
				Name: "etcd-2",
				Labels: map[string]string{
					"role":        "master",
					"etcd":        "true",
					"etcd-member": "etcd-2",
				},
			},
			{
				Name: "etcd-3",
				Labels: map[string]string{
					"role":        "master",
					"etcd":        "true",
					"etcd-member": "etcd-3",
				},
			},
		}

		// Verify etcd configuration
		assert.Equal(t, 7, cfg.Kubernetes.Etcd.BackupRetention)
		assert.NotNil(t, cfg.Kubernetes.Etcd.Snapshot)
		assert.Equal(t, 14, cfg.Kubernetes.Etcd.Snapshot.Retention)
		assert.Equal(t, 3, len(orch.nodes["digitalocean"]))

		etcdCount := 0
		for _, node := range orch.nodes["digitalocean"] {
			if node.Labels["etcd"] == "true" {
				etcdCount++
			}
		}
		assert.Equal(t, 3, etcdCount)

		return nil
	}, pulumi.WithMocks("test", "etcd-cluster", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}

// Test 12: Spot instances and preemptible nodes
func TestOrchestrator_SpotInstancesConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "spot-cluster",
				Environment: "development",
			},
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc3",
				},
			},
			NodePools: map[string]config.NodePool{
				"spot-workers": {
					Name:         "spot-workers",
					Provider:     "digitalocean",
					Count:        5,
					MinCount:     3,
					MaxCount:     10,
					Size:         "s-2vcpu-4gb",
					Roles:        []string{"worker"},
					SpotInstance: true,
					AutoScaling:  true,
					Labels: map[string]string{
						"workload-type": "batch",
						"spot":          "true",
					},
				},
				"on-demand-workers": {
					Name:         "on-demand-workers",
					Provider:     "digitalocean",
					Count:        2,
					Size:         "s-4vcpu-8gb",
					Roles:        []string{"worker"},
					SpotInstance: false,
					Labels: map[string]string{
						"workload-type": "critical",
						"spot":          "false",
					},
				},
			},
		}

		orch := New(ctx, cfg)

		// Verify spot instances configuration
		spotPool := cfg.NodePools["spot-workers"]
		onDemandPool := cfg.NodePools["on-demand-workers"]

		assert.True(t, spotPool.SpotInstance)
		assert.True(t, spotPool.AutoScaling)
		assert.Equal(t, "batch", spotPool.Labels["workload-type"])

		assert.False(t, onDemandPool.SpotInstance)
		assert.Equal(t, "critical", onDemandPool.Labels["workload-type"])

		orch.nodes = make(map[string][]*providers.NodeOutput)
		orch.nodes["digitalocean"] = []*providers.NodeOutput{
			{
				Name: "spot-worker-1",
				Labels: map[string]string{
					"role":          "worker",
					"spot":          "true",
					"workload-type": "batch",
					"preemptible":   "true",
				},
			},
			{
				Name: "on-demand-worker-1",
				Labels: map[string]string{
					"role":          "worker",
					"spot":          "false",
					"workload-type": "critical",
				},
			},
		}

		// Verify mixed node types
		assert.Equal(t, 2, len(orch.nodes["digitalocean"]))
		assert.Equal(t, "true", orch.nodes["digitalocean"][0].Labels["spot"])
		assert.Equal(t, "false", orch.nodes["digitalocean"][1].Labels["spot"])

		return nil
	}, pulumi.WithMocks("test", "spot-instances", &IntegrationMockProvider{}))
	assert.NoError(t, err)
}
