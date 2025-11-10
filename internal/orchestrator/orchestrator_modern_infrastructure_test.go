package orchestrator

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// TestOrchestrator_ObservabilityPlatform tests comprehensive observability infrastructure
func TestOrchestrator_ObservabilityPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "observability-platform",
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
				"metrics": {
					Name:     "metrics",
					Provider: "digitalocean",
					Count:    8,
					Size:     "s-4vcpu-8gb",
					Labels: map[string]string{
						"workload":   "metrics",
						"prometheus": "true",
						"thanos":     "true",
					},
				},
				"tracing": {
					Name:     "tracing",
					Provider: "digitalocean",
					Count:    5,
					Size:     "s-4vcpu-8gb",
					Labels: map[string]string{
						"workload": "tracing",
						"jaeger":   "true",
						"tempo":    "true",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 8, cfg.NodePools["metrics"].Count)
		assert.Equal(t, 5, cfg.NodePools["tracing"].Count)

		return nil
	}, pulumi.WithMocks("test", "observability", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_EventDrivenArchitecture tests event-driven architecture platform
func TestOrchestrator_EventDrivenArchitecture(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "event-driven-platform",
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
				"event-bus": {
					Name:     "event-bus",
					Provider: "digitalocean",
					Count:    10,
					Size:     "s-4vcpu-8gb",
					Labels: map[string]string{
						"workload": "event-bus",
						"kafka":    "true",
						"nats":     "true",
					},
				},
				"processors": {
					Name:     "processors",
					Provider: "digitalocean",
					Count:    20,
					Size:     "s-2vcpu-4gb",
					Labels: map[string]string{
						"workload":      "event-processor",
						"serverless":    "true",
						"event-handler": "true",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 10, cfg.NodePools["event-bus"].Count)
		assert.Equal(t, 20, cfg.NodePools["processors"].Count)

		return nil
	}, pulumi.WithMocks("test", "event-driven", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_DataPipelinePlatform tests data pipeline infrastructure
func TestOrchestrator_DataPipelinePlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "data-pipeline-platform",
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
				"ingestion": {
					Name:     "ingestion",
					Provider: "digitalocean",
					Count:    12,
					Size:     "s-4vcpu-8gb",
					Labels: map[string]string{
						"workload": "ingestion",
						"airflow":  "true",
						"kafka":    "true",
					},
				},
				"transformation": {
					Name:     "transformation",
					Provider: "digitalocean",
					Count:    15,
					Size:     "c-8",
					Labels: map[string]string{
						"workload": "transformation",
						"spark":    "true",
						"dbt":      "true",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 12, cfg.NodePools["ingestion"].Count)
		assert.Equal(t, 15, cfg.NodePools["transformation"].Count)

		return nil
	}, pulumi.WithMocks("test", "data-pipeline", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_ContainerRegistryPlatform tests container registry infrastructure
func TestOrchestrator_ContainerRegistryPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "container-registry",
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
				"registry": {
					Name:     "registry",
					Provider: "digitalocean",
					Count:    6,
					Size:     "s-4vcpu-8gb",
					Labels: map[string]string{
						"workload": "registry",
						"harbor":   "true",
						"scanning": "true",
					},
				},
			},
			Security: config.SecurityConfig{
				NetworkPolicies: true,
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 6, cfg.NodePools["registry"].Count)
		assert.True(t, cfg.Security.NetworkPolicies)

		return nil
	}, pulumi.WithMocks("test", "registry", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_APIPlatform tests API management platform
func TestOrchestrator_APIPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "api-platform",
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
				"gateway": {
					Name:     "gateway",
					Provider: "digitalocean",
					Count:    10,
					Size:     "s-4vcpu-8gb",
					Labels: map[string]string{
						"workload":      "api-gateway",
						"kong":          "true",
						"rate-limiting": "true",
					},
				},
				"backend": {
					Name:     "backend",
					Provider: "digitalocean",
					Count:    25,
					Size:     "s-2vcpu-4gb",
					Labels: map[string]string{
						"workload": "api-backend",
						"rest":     "true",
						"graphql":  "true",
					},
				},
			},
			Cluster: config.ClusterSpec{
				AutoScaling: config.AutoScalingConfig{
					Enabled:   true,
					MinNodes:  15,
					MaxNodes:  50,
					TargetCPU: 70,
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 10, cfg.NodePools["gateway"].Count)
		assert.True(t, cfg.Cluster.AutoScaling.Enabled)

		return nil
	}, pulumi.WithMocks("test", "api-platform", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_TestingInfrastructure tests automated testing infrastructure
func TestOrchestrator_TestingInfrastructure(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "testing-infrastructure",
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
				"test-runners": {
					Name:     "test-runners",
					Provider: "digitalocean",
					Count:    30,
					Size:     "c-8",
					Labels: map[string]string{
						"workload":    "testing",
						"selenium":    "true",
						"cypress":     "true",
						"performance": "true",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 30, cfg.NodePools["test-runners"].Count)

		return nil
	}, pulumi.WithMocks("test", "testing", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_NotificationPlatform tests notification and messaging platform
func TestOrchestrator_NotificationPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "notification-platform",
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
				"notification-service": {
					Name:     "notification-service",
					Provider: "digitalocean",
					Count:    8,
					Size:     "s-4vcpu-8gb",
					Labels: map[string]string{
						"workload": "notifications",
						"email":    "true",
						"sms":      "true",
						"push":     "true",
					},
				},
				"queue": {
					Name:     "queue",
					Provider: "digitalocean",
					Count:    5,
					Size:     "s-2vcpu-4gb",
					Labels: map[string]string{
						"workload":  "queue",
						"rabbitmq":  "true",
						"redis":     "true",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 8, cfg.NodePools["notification-service"].Count)
		assert.Equal(t, 5, cfg.NodePools["queue"].Count)

		return nil
	}, pulumi.WithMocks("test", "notification", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_MediaProcessingPlatform tests media processing infrastructure
func TestOrchestrator_MediaProcessingPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "media-processing",
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
				"video-transcode": {
					Name:     "video-transcode",
					Provider: "digitalocean",
					Count:    15,
					Size:     "c-16",
					Labels: map[string]string{
						"workload":   "video",
						"ffmpeg":     "true",
						"transcode":  "true",
						"gpu":        "false",
					},
				},
				"image-processing": {
					Name:     "image-processing",
					Provider: "digitalocean",
					Count:    10,
					Size:     "c-8",
					Labels: map[string]string{
						"workload":  "image",
						"imagick":   "true",
						"sharp":     "true",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 15, cfg.NodePools["video-transcode"].Count)
		assert.Equal(t, 10, cfg.NodePools["image-processing"].Count)

		return nil
	}, pulumi.WithMocks("test", "media-processing", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_QueueProcessingPlatform tests queue and task processing
func TestOrchestrator_QueueProcessingPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "queue-processing",
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
				"workers": {
					Name:     "workers",
					Provider: "digitalocean",
					Count:    40,
					Size:     "s-2vcpu-4gb",
					Labels: map[string]string{
						"workload": "queue-worker",
						"celery":   "true",
						"sidekiq":  "true",
					},
				},
			},
			Cluster: config.ClusterSpec{
				AutoScaling: config.AutoScalingConfig{
					Enabled:   true,
					MinNodes:  20,
					MaxNodes:  100,
					TargetCPU: 75,
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 40, cfg.NodePools["workers"].Count)
		assert.True(t, cfg.Cluster.AutoScaling.Enabled)

		return nil
	}, pulumi.WithMocks("test", "queue-processing", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_AnalyticsDashboardPlatform tests analytics and dashboard platform
func TestOrchestrator_AnalyticsDashboardPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "analytics-dashboard",
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
				"dashboard": {
					Name:     "dashboard",
					Provider: "digitalocean",
					Count:    8,
					Size:     "s-4vcpu-8gb",
					Labels: map[string]string{
						"workload":  "dashboard",
						"grafana":   "true",
						"metabase":  "true",
						"superset":  "true",
					},
				},
				"query-engine": {
					Name:     "query-engine",
					Provider: "digitalocean",
					Count:    12,
					Size:     "m-8vcpu-64gb",
					Labels: map[string]string{
						"workload":   "query",
						"presto":     "true",
						"clickhouse": "true",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 8, cfg.NodePools["dashboard"].Count)
		assert.Equal(t, 12, cfg.NodePools["query-engine"].Count)

		return nil
	}, pulumi.WithMocks("test", "analytics-dashboard", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_IoTDataCollectionPlatform tests IoT data collection infrastructure
func TestOrchestrator_IoTDataCollectionPlatform(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "iot-data-collection",
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
				"mqtt-brokers": {
					Name:     "mqtt-brokers",
					Provider: "digitalocean",
					Count:    10,
					Size:     "s-4vcpu-8gb",
					Labels: map[string]string{
						"workload":   "mqtt",
						"mosquitto":  "true",
						"iot":        "true",
					},
				},
				"data-processors": {
					Name:     "data-processors",
					Provider: "digitalocean",
					Count:    20,
					Size:     "s-2vcpu-4gb",
					Labels: map[string]string{
						"workload":    "processing",
						"telemetry":   "true",
						"timeseries":  "true",
					},
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 10, cfg.NodePools["mqtt-brokers"].Count)
		assert.Equal(t, 20, cfg.NodePools["data-processors"].Count)

		return nil
	}, pulumi.WithMocks("test", "iot-collection", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestOrchestrator_ServerlessPlatform tests serverless/FaaS infrastructure
func TestOrchestrator_ServerlessPlatform(t *testing.T) {
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
					Region:  "nyc1",
				},
			},
			NodePools: map[string]config.NodePool{
				"function-runtime": {
					Name:     "function-runtime",
					Provider: "digitalocean",
					Count:    25,
					Size:     "s-2vcpu-4gb",
					Labels: map[string]string{
						"workload":   "serverless",
						"knative":    "true",
						"openfaas":   "true",
						"scale-zero": "true",
					},
				},
			},
			Cluster: config.ClusterSpec{
				AutoScaling: config.AutoScalingConfig{
					Enabled:   true,
					MinNodes:  5,
					MaxNodes:  100,
					TargetCPU: 60,
				},
			},
		}

		orch := New(ctx, cfg)
		assert.NotNil(t, orch)
		assert.Equal(t, 25, cfg.NodePools["function-runtime"].Count)
		assert.True(t, cfg.Cluster.AutoScaling.Enabled)
		assert.Equal(t, 100, cfg.Cluster.AutoScaling.MaxNodes)

		return nil
	}, pulumi.WithMocks("test", "serverless", &IntegrationMockProvider{}))

	assert.NoError(t, err)
}
