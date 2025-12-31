package costs

import (
	"context"
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEstimator(t *testing.T) {
	estimator := NewEstimator(&EstimatorConfig{})

	assert.NotNil(t, estimator)
	assert.NotNil(t, estimator.cache)
	assert.NotNil(t, estimator.providers)
}

func TestEstimator_DefaultProviders(t *testing.T) {
	estimator := NewEstimator(&EstimatorConfig{})

	providers := []string{"aws", "gcp", "digitalocean", "linode"}
	for _, name := range providers {
		_, exists := estimator.providers[name]
		assert.True(t, exists, "Provider %s should be registered", name)
	}
}

func TestEstimator_EstimateNodeCost(t *testing.T) {
	estimator := NewEstimator(&EstimatorConfig{})

	tests := []struct {
		name       string
		nodeConfig *config.NodeConfig
		wantErr    bool
	}{
		{
			name: "AWS t3.medium",
			nodeConfig: &config.NodeConfig{
				Name:     "test-node",
				Provider: "aws",
				Size:     "t3.medium",
				Region:   "us-east-1",
			},
			wantErr: false,
		},
		{
			name: "GCP e2-medium",
			nodeConfig: &config.NodeConfig{
				Name:     "test-node",
				Provider: "gcp",
				Size:     "e2-medium",
				Region:   "us-central1",
			},
			wantErr: false,
		},
		{
			name: "Unknown provider",
			nodeConfig: &config.NodeConfig{
				Name:     "test-node",
				Provider: "unknown",
				Size:     "small",
				Region:   "us-east-1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimate, err := estimator.EstimateNodeCost(context.Background(), tt.nodeConfig)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, estimate)
			assert.Greater(t, estimate.HourlyCost, 0.0)
			assert.Greater(t, estimate.MonthlyCost, 0.0)
			assert.Greater(t, estimate.YearlyCost, 0.0)
			assert.Equal(t, "USD", estimate.Currency)
		})
	}
}

func TestEstimator_EstimateNodeCost_SpotInstance(t *testing.T) {
	estimator := NewEstimator(&EstimatorConfig{})

	nodeConfig := &config.NodeConfig{
		Name:         "spot-node",
		Provider:     "aws",
		Size:         "t3.medium",
		Region:       "us-east-1",
		SpotInstance: true,
	}

	estimate, err := estimator.EstimateNodeCost(context.Background(), nodeConfig)
	require.NoError(t, err)

	assert.True(t, estimate.IsSpot)
	assert.Greater(t, estimate.SpotSavings, 0.0)
}

func TestEstimator_EstimateClusterCost(t *testing.T) {
	estimator := NewEstimator(&EstimatorConfig{})

	clusterConfig := &config.ClusterConfig{
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Provider: "aws",
				Count:    3,
				Size:     "t3.medium",
				Region:   "us-east-1",
				Roles:    []string{"master"},
			},
			"workers": {
				Name:     "workers",
				Provider: "aws",
				Count:    5,
				Size:     "t3.large",
				Region:   "us-east-1",
				Roles:    []string{"worker"},
			},
		},
		LoadBalancer: config.LoadBalancerConfig{
			Provider: "aws",
		},
	}

	estimate, err := estimator.EstimateClusterCost(context.Background(), clusterConfig)
	require.NoError(t, err)

	assert.Greater(t, estimate.TotalMonthlyCost, 0.0)
	assert.Greater(t, estimate.TotalYearlyCost, 0.0)
	assert.Len(t, estimate.NodeCosts, 8) // 3 masters + 5 workers
	assert.Equal(t, "USD", estimate.Currency)
}

func TestEstimator_GeneratesRecommendations(t *testing.T) {
	estimator := NewEstimator(&EstimatorConfig{})

	// Create cluster config with non-spot workers
	clusterConfig := &config.ClusterConfig{
		NodePools: map[string]config.NodePool{
			"workers": {
				Name:         "workers",
				Provider:     "aws",
				Count:        5,
				Size:         "m5.xlarge",
				Region:       "us-east-1",
				Roles:        []string{"worker"},
				SpotInstance: false, // Not using spot
			},
		},
	}

	estimate, err := estimator.EstimateClusterCost(context.Background(), clusterConfig)
	require.NoError(t, err)

	// Should generate spot usage recommendation
	hasSpotRecommendation := false
	for _, rec := range estimate.Recommendations {
		if rec.Type == "spot_usage" {
			hasSpotRecommendation = true
			assert.Greater(t, rec.PotentialSavings, 0.0)
		}
	}
	assert.True(t, hasSpotRecommendation)
}

func TestAWSPriceProvider(t *testing.T) {
	provider := NewAWSPriceProvider()

	assert.Equal(t, "aws", provider.Name())

	tests := []struct {
		instanceType string
		region       string
		wantPrice    float64
	}{
		{"t3.micro", "us-east-1", 0.0104},
		{"t3.medium", "us-east-1", 0.0416},
		{"m5.large", "us-east-1", 0.096},
	}

	for _, tt := range tests {
		t.Run(tt.instanceType, func(t *testing.T) {
			price, err := provider.GetInstancePrice(context.Background(), tt.instanceType, tt.region)
			require.NoError(t, err)
			assert.Equal(t, tt.wantPrice, price)
		})
	}
}

func TestAWSPriceProvider_SpotPrice(t *testing.T) {
	provider := NewAWSPriceProvider()

	basePrice, _ := provider.GetInstancePrice(context.Background(), "t3.medium", "us-east-1")
	spotPrice, err := provider.GetSpotPrice(context.Background(), "t3.medium", "us-east-1")
	require.NoError(t, err)

	// Spot price should be less than on-demand
	assert.Less(t, spotPrice, basePrice)
}

func TestAWSPriceProvider_StoragePrice(t *testing.T) {
	provider := NewAWSPriceProvider()

	tests := []struct {
		storageType string
		wantPrice   float64
	}{
		{"ssd", 0.08},
		{"gp3", 0.08},
		{"hdd", 0.045},
		{"io1", 0.125},
	}

	for _, tt := range tests {
		t.Run(tt.storageType, func(t *testing.T) {
			price, err := provider.GetStoragePrice(context.Background(), tt.storageType, "us-east-1")
			require.NoError(t, err)
			assert.Equal(t, tt.wantPrice, price)
		})
	}
}

func TestGCPPriceProvider(t *testing.T) {
	provider := NewGCPPriceProvider()

	assert.Equal(t, "gcp", provider.Name())

	price, err := provider.GetInstancePrice(context.Background(), "e2-medium", "us-central1")
	require.NoError(t, err)
	assert.Greater(t, price, 0.0)
}

func TestDigitalOceanPriceProvider(t *testing.T) {
	provider := NewDigitalOceanPriceProvider()

	assert.Equal(t, "digitalocean", provider.Name())

	price, err := provider.GetInstancePrice(context.Background(), "s-2vcpu-4gb", "nyc1")
	require.NoError(t, err)
	assert.Equal(t, 0.028, price)
}

func TestLinodePriceProvider(t *testing.T) {
	provider := NewLinodePriceProvider()

	assert.Equal(t, "linode", provider.Name())

	price, err := provider.GetInstancePrice(context.Background(), "g6-standard-2", "us-east")
	require.NoError(t, err)
	assert.Equal(t, 0.030, price)
}

func TestPriceCache(t *testing.T) {
	estimator := NewEstimator(&EstimatorConfig{})

	nodeConfig := &config.NodeConfig{
		Name:     "test-node",
		Provider: "aws",
		Size:     "t3.medium",
		Region:   "us-east-1",
	}

	// First call - should cache
	estimate1, err := estimator.EstimateNodeCost(context.Background(), nodeConfig)
	require.NoError(t, err)

	// Second call - should use cache
	estimate2, err := estimator.EstimateNodeCost(context.Background(), nodeConfig)
	require.NoError(t, err)

	// Results should be identical
	assert.Equal(t, estimate1.HourlyCost, estimate2.HourlyCost)
	assert.Equal(t, estimate1.MonthlyCost, estimate2.MonthlyCost)
}

func TestCostEstimate_Breakdown(t *testing.T) {
	estimator := NewEstimator(&EstimatorConfig{})

	nodeConfig := &config.NodeConfig{
		Name:     "test-node",
		Provider: "aws",
		Size:     "t3.medium",
		Region:   "us-east-1",
	}

	estimate, err := estimator.EstimateNodeCost(context.Background(), nodeConfig)
	require.NoError(t, err)

	// Should have breakdown
	assert.Contains(t, estimate.Breakdown, "compute")
	assert.Contains(t, estimate.Breakdown, "storage")
	assert.Greater(t, estimate.Breakdown["compute"], 0.0)
}

func TestEstimator_UnknownInstanceType(t *testing.T) {
	estimator := NewEstimator(&EstimatorConfig{})

	nodeConfig := &config.NodeConfig{
		Name:     "test-node",
		Provider: "aws",
		Size:     "unknown-instance-type",
		Region:   "us-east-1",
	}

	_, err := estimator.EstimateNodeCost(context.Background(), nodeConfig)
	assert.Error(t, err)
}
