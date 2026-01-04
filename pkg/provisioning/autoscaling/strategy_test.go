package autoscaling

import (
	"context"
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockMetricsCollector implements MetricsCollector for testing
type MockMetricsCollector struct {
	cpu    float64
	memory float64
}

func (m *MockMetricsCollector) GetCPUUtilization(ctx context.Context) (float64, error) {
	return m.cpu, nil
}

func (m *MockMetricsCollector) GetMemoryUtilization(ctx context.Context) (float64, error) {
	return m.memory, nil
}

func (m *MockMetricsCollector) GetCustomMetric(ctx context.Context, name string) (float64, error) {
	return 0, nil
}

func TestStrategyRegistry_DefaultStrategies(t *testing.T) {
	registry := NewStrategyRegistry()

	strategies := registry.List()
	assert.Contains(t, strategies, "cpu")
	assert.Contains(t, strategies, "memory")
	assert.Contains(t, strategies, "composite")
}

func TestStrategyRegistry_Get(t *testing.T) {
	registry := NewStrategyRegistry()

	strategy, err := registry.Get("cpu")
	require.NoError(t, err)
	assert.Equal(t, "cpu", strategy.Name())

	_, err = registry.Get("nonexistent")
	assert.Error(t, err)
}

func TestCPUBasedStrategy_ShouldScaleUp(t *testing.T) {
	tests := []struct {
		name      string
		cpu       float64
		targetCPU int
		enabled   bool
		wantScale bool
		wantNodes int
	}{
		{
			name:      "High CPU triggers scale up",
			cpu:       90,
			targetCPU: 70,
			enabled:   true,
			wantScale: true,
			wantNodes: 1,
		},
		{
			name:      "Normal CPU no scale up",
			cpu:       65,
			targetCPU: 70,
			enabled:   true,
			wantScale: false,
			wantNodes: 0,
		},
		{
			name:      "Disabled auto-scaling",
			cpu:       95,
			targetCPU: 70,
			enabled:   false,
			wantScale: false,
			wantNodes: 0,
		},
		{
			name:      "Very high CPU scales up more",
			cpu:       95,
			targetCPU: 50,
			enabled:   true,
			wantScale: true,
			wantNodes: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &CPUBasedStrategy{}
			metrics := &MockMetricsCollector{cpu: tt.cpu}
			cfg := &config.AutoScalingConfig{
				Enabled:   tt.enabled,
				TargetCPU: tt.targetCPU,
				MaxNodes:  10,
			}

			shouldScale, nodes, err := strategy.ShouldScaleUp(context.Background(), metrics, cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.wantScale, shouldScale)
			if tt.wantScale {
				assert.GreaterOrEqual(t, nodes, tt.wantNodes)
			}
		})
	}
}

func TestCPUBasedStrategy_ShouldScaleDown(t *testing.T) {
	tests := []struct {
		name      string
		cpu       float64
		targetCPU int
		enabled   bool
		wantScale bool
	}{
		{
			name:      "Low CPU triggers scale down",
			cpu:       30,
			targetCPU: 70,
			enabled:   true,
			wantScale: true,
		},
		{
			name:      "Normal CPU no scale down",
			cpu:       65,
			targetCPU: 70,
			enabled:   true,
			wantScale: false,
		},
		{
			name:      "Disabled auto-scaling",
			cpu:       20,
			targetCPU: 70,
			enabled:   false,
			wantScale: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &CPUBasedStrategy{}
			metrics := &MockMetricsCollector{cpu: tt.cpu}
			cfg := &config.AutoScalingConfig{
				Enabled:   tt.enabled,
				TargetCPU: tt.targetCPU,
				MinNodes:  1,
			}

			shouldScale, _, err := strategy.ShouldScaleDown(context.Background(), metrics, cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.wantScale, shouldScale)
		})
	}
}

func TestMemoryBasedStrategy_ShouldScaleUp(t *testing.T) {
	strategy := &MemoryBasedStrategy{}
	metrics := &MockMetricsCollector{memory: 95}
	cfg := &config.AutoScalingConfig{
		Enabled:      true,
		TargetMemory: 75,
		MaxNodes:     10,
	}

	shouldScale, nodes, err := strategy.ShouldScaleUp(context.Background(), metrics, cfg)
	require.NoError(t, err)
	assert.True(t, shouldScale)
	assert.Greater(t, nodes, 0)
}

func TestCompositeStrategy_ScaleUp(t *testing.T) {
	strategy := NewCompositeStrategy()

	// High CPU should trigger scale up
	metrics := &MockMetricsCollector{cpu: 90, memory: 60}
	cfg := &config.AutoScalingConfig{
		Enabled:      true,
		TargetCPU:    70,
		TargetMemory: 75,
		MaxNodes:     10,
	}

	shouldScale, nodes, err := strategy.ShouldScaleUp(context.Background(), metrics, cfg)
	require.NoError(t, err)
	assert.True(t, shouldScale)
	assert.Greater(t, nodes, 0)
}

func TestCompositeStrategy_ScaleDown_AllMustAgree(t *testing.T) {
	strategy := NewCompositeStrategy()

	// Only CPU is low, memory is normal - should NOT scale down
	metrics := &MockMetricsCollector{cpu: 30, memory: 60}
	cfg := &config.AutoScalingConfig{
		Enabled:      true,
		TargetCPU:    70,
		TargetMemory: 75,
		MinNodes:     1,
	}

	shouldScale, _, err := strategy.ShouldScaleDown(context.Background(), metrics, cfg)
	require.NoError(t, err)
	assert.False(t, shouldScale) // Memory is not low enough

	// Both CPU and memory are low - should scale down
	metrics.memory = 40
	shouldScale, _, err = strategy.ShouldScaleDown(context.Background(), metrics, cfg)
	require.NoError(t, err)
	assert.True(t, shouldScale)
}

func TestCustomMetricStrategy(t *testing.T) {
	strategy := NewCustomMetricStrategy("queue_length", 100, true)

	assert.Equal(t, "custom:queue_length", strategy.Name())
}
