package distribution

import (
	"context"
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoundRobinStrategy_Calculate(t *testing.T) {
	tests := []struct {
		name       string
		totalNodes int
		zones      []string
		want       map[string]int
	}{
		{
			name:       "Even distribution",
			totalNodes: 6,
			zones:      []string{"us-east-1a", "us-east-1b", "us-east-1c"},
			want:       map[string]int{"us-east-1a": 2, "us-east-1b": 2, "us-east-1c": 2},
		},
		{
			name:       "Uneven distribution",
			totalNodes: 5,
			zones:      []string{"us-east-1a", "us-east-1b"},
			want:       map[string]int{"us-east-1a": 3, "us-east-1b": 2},
		},
		{
			name:       "Single zone",
			totalNodes: 3,
			zones:      []string{"us-east-1a"},
			want:       map[string]int{"us-east-1a": 3},
		},
		{
			name:       "More zones than nodes",
			totalNodes: 2,
			zones:      []string{"us-east-1a", "us-east-1b", "us-east-1c"},
			want:       map[string]int{"us-east-1a": 1, "us-east-1b": 1, "us-east-1c": 0},
		},
		{
			name:       "Empty zones",
			totalNodes: 3,
			zones:      []string{},
			want:       map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &RoundRobinStrategy{}
			result := strategy.Calculate(tt.totalNodes, tt.zones, nil)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestWeightedStrategy_Calculate(t *testing.T) {
	tests := []struct {
		name       string
		totalNodes int
		zones      []string
		weights    map[string]int
		wantTotal  int // Verify total equals input
	}{
		{
			name:       "Weighted distribution",
			totalNodes: 10,
			zones:      []string{"us-east-1a", "us-east-1b"},
			weights:    map[string]int{"us-east-1a": 3, "us-east-1b": 1},
			wantTotal:  10,
		},
		{
			name:       "Equal weights",
			totalNodes: 6,
			zones:      []string{"us-east-1a", "us-east-1b", "us-east-1c"},
			weights:    map[string]int{"us-east-1a": 1, "us-east-1b": 1, "us-east-1c": 1},
			wantTotal:  6,
		},
		{
			name:       "No weights defaults to equal",
			totalNodes: 6,
			zones:      []string{"us-east-1a", "us-east-1b"},
			weights:    nil,
			wantTotal:  6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &WeightedStrategy{}
			result := strategy.Calculate(tt.totalNodes, tt.zones, tt.weights)

			// Verify total
			total := 0
			for _, count := range result {
				total += count
			}
			assert.Equal(t, tt.wantTotal, total)

			// Verify weighted distribution (zone with higher weight gets more nodes)
			if tt.weights != nil && len(tt.zones) > 1 {
				maxWeight := 0
				maxWeightZone := ""
				for zone, weight := range tt.weights {
					if weight > maxWeight {
						maxWeight = weight
						maxWeightZone = zone
					}
				}
				for zone, count := range result {
					if zone != maxWeightZone {
						assert.LessOrEqual(t, count, result[maxWeightZone])
					}
				}
			}
		})
	}
}

func TestPackedStrategy_Calculate(t *testing.T) {
	strategy := NewPackedStrategy(5) // 5 nodes per zone capacity

	tests := []struct {
		name       string
		totalNodes int
		zones      []string
		want       map[string]int
	}{
		{
			name:       "Fills first zone before second",
			totalNodes: 8,
			zones:      []string{"us-east-1a", "us-east-1b"},
			want:       map[string]int{"us-east-1a": 5, "us-east-1b": 3},
		},
		{
			name:       "All in one zone",
			totalNodes: 3,
			zones:      []string{"us-east-1a", "us-east-1b"},
			want:       map[string]int{"us-east-1a": 3, "us-east-1b": 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strategy.Calculate(tt.totalNodes, tt.zones, nil)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestSpreadStrategy_Calculate(t *testing.T) {
	strategy := &SpreadStrategy{}

	tests := []struct {
		name       string
		totalNodes int
		zones      []string
		wantMin    int // Minimum nodes per zone
	}{
		{
			name:       "Spread across 3 zones",
			totalNodes: 6,
			zones:      []string{"us-east-1a", "us-east-1b", "us-east-1c"},
			wantMin:    2,
		},
		{
			name:       "Spread with uneven count",
			totalNodes: 7,
			zones:      []string{"us-east-1a", "us-east-1b", "us-east-1c"},
			wantMin:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strategy.Calculate(tt.totalNodes, tt.zones, nil)

			// Verify minimum nodes per zone
			for _, count := range result {
				assert.GreaterOrEqual(t, count, tt.wantMin)
			}

			// Verify total
			total := 0
			for _, count := range result {
				total += count
			}
			assert.Equal(t, tt.totalNodes, total)
		})
	}
}

func TestDistributor_Distribute(t *testing.T) {
	cfg := &DistributorConfig{
		StrategyName: "round_robin",
	}

	distributor, err := NewDistributor(cfg)
	require.NoError(t, err)

	result, err := distributor.Distribute(context.Background(), 6, []string{"a", "b", "c"})
	require.NoError(t, err)

	assert.Equal(t, 2, result["a"])
	assert.Equal(t, 2, result["b"])
	assert.Equal(t, 2, result["c"])
}

func TestDistributor_DistributeWithConfig(t *testing.T) {
	distributor, _ := NewDistributor(&DistributorConfig{})

	// Explicit counts
	distributions := []config.ZoneDistribution{
		{Zone: "us-east-1a", Count: 3},
		{Zone: "us-east-1b", Count: 2},
	}

	result, err := distributor.DistributeWithConfig(context.Background(), 5, distributions)
	require.NoError(t, err)

	assert.Equal(t, 3, result["us-east-1a"])
	assert.Equal(t, 2, result["us-east-1b"])
}

func TestDistributor_SetStrategy(t *testing.T) {
	distributor, _ := NewDistributor(&DistributorConfig{})

	// Default strategy
	plan := distributor.GetDistributionPlan(6, []string{"a", "b", "c"}, nil)
	assert.Equal(t, "round_robin", plan.Strategy)

	// Change strategy
	err := distributor.SetStrategy("spread")
	require.NoError(t, err)

	plan = distributor.GetDistributionPlan(6, []string{"a", "b", "c"}, nil)
	assert.Equal(t, "spread", plan.Strategy)

	// Invalid strategy
	err = distributor.SetStrategy("nonexistent")
	assert.Error(t, err)
}

func TestDistributor_Rebalance(t *testing.T) {
	distributor, _ := NewDistributor(&DistributorConfig{StrategyName: "round_robin"})

	// Unbalanced distribution
	current := map[string]int{
		"a": 5,
		"b": 1,
		"c": 0,
	}

	changes, err := distributor.Rebalance(context.Background(), current)
	require.NoError(t, err)

	// Should suggest rebalancing
	assert.Less(t, changes["a"], 0)    // Remove from a
	assert.Greater(t, changes["b"], 0) // Add to b
	assert.Greater(t, changes["c"], 0) // Add to c
}

func TestDistributionPlan(t *testing.T) {
	distributor, _ := NewDistributor(&DistributorConfig{})

	weights := map[string]int{"a": 2, "b": 1}
	plan := distributor.GetDistributionPlan(9, []string{"a", "b"}, weights)

	assert.Equal(t, 9, plan.TotalNodes)
	assert.Len(t, plan.Zones, 2)

	// Find zone percentages
	for _, zonePlan := range plan.Zones {
		if zonePlan.Zone == "a" {
			assert.Greater(t, zonePlan.Percentage, 50.0)
		}
	}
}

func TestStrategyRegistry(t *testing.T) {
	registry := NewStrategyRegistry()

	// All default strategies should be registered
	strategies := []string{"round_robin", "weighted", "packed", "spread"}
	for _, name := range strategies {
		strategy, err := registry.Get(name)
		require.NoError(t, err)
		assert.Equal(t, name, strategy.Name())
	}
}
