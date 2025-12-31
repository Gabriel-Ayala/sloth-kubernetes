// Package autoscaling provides auto-scaling functionality for node pools
package autoscaling

import (
	"context"
	"fmt"
	"sync"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning"
)

// =============================================================================
// Strategy Registry - Factory Pattern
// =============================================================================

// StrategyRegistry manages available scaling strategies
type StrategyRegistry struct {
	strategies map[string]provisioning.ScalingStrategy
	mu         sync.RWMutex
}

// NewStrategyRegistry creates a new strategy registry with default strategies
func NewStrategyRegistry() *StrategyRegistry {
	registry := &StrategyRegistry{
		strategies: make(map[string]provisioning.ScalingStrategy),
	}

	// Register default strategies
	registry.Register(&CPUBasedStrategy{})
	registry.Register(&MemoryBasedStrategy{})
	registry.Register(&CompositeStrategy{})

	return registry
}

// Register adds a strategy to the registry
func (r *StrategyRegistry) Register(strategy provisioning.ScalingStrategy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.strategies[strategy.Name()] = strategy
}

// Get retrieves a strategy by name
func (r *StrategyRegistry) Get(name string) (provisioning.ScalingStrategy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	strategy, exists := r.strategies[name]
	if !exists {
		return nil, fmt.Errorf("strategy %s not found", name)
	}
	return strategy, nil
}

// List returns all available strategy names
func (r *StrategyRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.strategies))
	for name := range r.strategies {
		names = append(names, name)
	}
	return names
}

// =============================================================================
// CPU-Based Strategy
// =============================================================================

// CPUBasedStrategy scales based on CPU utilization
type CPUBasedStrategy struct{}

// Name returns the strategy name
func (s *CPUBasedStrategy) Name() string {
	return "cpu"
}

// ShouldScaleUp determines if cluster should scale up based on CPU
func (s *CPUBasedStrategy) ShouldScaleUp(
	ctx context.Context,
	metrics provisioning.MetricsCollector,
	cfg *config.AutoScalingConfig,
) (bool, int, error) {
	if !cfg.Enabled {
		return false, 0, nil
	}

	cpu, err := metrics.GetCPUUtilization(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("failed to get CPU utilization: %w", err)
	}

	targetCPU := float64(cfg.TargetCPU)
	if targetCPU == 0 {
		targetCPU = 70 // Default target
	}

	// Scale up if CPU is above target + 10% buffer
	if cpu > targetCPU+10 {
		// Calculate how many nodes to add
		// Using a simple linear formula: add 1 node for every 20% over target
		excess := cpu - targetCPU
		nodesToAdd := int(excess/20) + 1

		// Cap at maxNodes
		if nodesToAdd > cfg.MaxNodes {
			nodesToAdd = cfg.MaxNodes
		}

		return true, nodesToAdd, nil
	}

	return false, 0, nil
}

// ShouldScaleDown determines if cluster should scale down based on CPU
func (s *CPUBasedStrategy) ShouldScaleDown(
	ctx context.Context,
	metrics provisioning.MetricsCollector,
	cfg *config.AutoScalingConfig,
) (bool, int, error) {
	if !cfg.Enabled {
		return false, 0, nil
	}

	cpu, err := metrics.GetCPUUtilization(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("failed to get CPU utilization: %w", err)
	}

	targetCPU := float64(cfg.TargetCPU)
	if targetCPU == 0 {
		targetCPU = 70
	}

	// Scale down if CPU is below target - 20%
	if cpu < targetCPU-20 {
		// Calculate how many nodes to remove
		deficit := targetCPU - cpu
		nodesToRemove := int(deficit / 25)

		if nodesToRemove < 1 {
			nodesToRemove = 1
		}

		return true, nodesToRemove, nil
	}

	return false, 0, nil
}

// =============================================================================
// Memory-Based Strategy
// =============================================================================

// MemoryBasedStrategy scales based on memory utilization
type MemoryBasedStrategy struct{}

// Name returns the strategy name
func (s *MemoryBasedStrategy) Name() string {
	return "memory"
}

// ShouldScaleUp determines if cluster should scale up based on memory
func (s *MemoryBasedStrategy) ShouldScaleUp(
	ctx context.Context,
	metrics provisioning.MetricsCollector,
	cfg *config.AutoScalingConfig,
) (bool, int, error) {
	if !cfg.Enabled {
		return false, 0, nil
	}

	memory, err := metrics.GetMemoryUtilization(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("failed to get memory utilization: %w", err)
	}

	targetMemory := float64(cfg.TargetMemory)
	if targetMemory == 0 {
		targetMemory = 75 // Default target
	}

	// Scale up if memory is above target + 10% buffer
	if memory > targetMemory+10 {
		excess := memory - targetMemory
		nodesToAdd := int(excess/15) + 1

		if nodesToAdd > cfg.MaxNodes {
			nodesToAdd = cfg.MaxNodes
		}

		return true, nodesToAdd, nil
	}

	return false, 0, nil
}

// ShouldScaleDown determines if cluster should scale down based on memory
func (s *MemoryBasedStrategy) ShouldScaleDown(
	ctx context.Context,
	metrics provisioning.MetricsCollector,
	cfg *config.AutoScalingConfig,
) (bool, int, error) {
	if !cfg.Enabled {
		return false, 0, nil
	}

	memory, err := metrics.GetMemoryUtilization(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("failed to get memory utilization: %w", err)
	}

	targetMemory := float64(cfg.TargetMemory)
	if targetMemory == 0 {
		targetMemory = 75
	}

	// Scale down if memory is below target - 25%
	if memory < targetMemory-25 {
		deficit := targetMemory - memory
		nodesToRemove := int(deficit / 20)

		if nodesToRemove < 1 {
			nodesToRemove = 1
		}

		return true, nodesToRemove, nil
	}

	return false, 0, nil
}

// =============================================================================
// Composite Strategy - Combines CPU and Memory
// =============================================================================

// CompositeStrategy combines multiple strategies
type CompositeStrategy struct {
	strategies []provisioning.ScalingStrategy
}

// NewCompositeStrategy creates a new composite strategy
func NewCompositeStrategy(strategies ...provisioning.ScalingStrategy) *CompositeStrategy {
	if len(strategies) == 0 {
		strategies = []provisioning.ScalingStrategy{
			&CPUBasedStrategy{},
			&MemoryBasedStrategy{},
		}
	}
	return &CompositeStrategy{strategies: strategies}
}

// Name returns the strategy name
func (s *CompositeStrategy) Name() string {
	return "composite"
}

// ShouldScaleUp checks all strategies and returns if any suggests scaling up
func (s *CompositeStrategy) ShouldScaleUp(
	ctx context.Context,
	metrics provisioning.MetricsCollector,
	cfg *config.AutoScalingConfig,
) (bool, int, error) {
	maxNodes := 0

	for _, strategy := range s.strategies {
		shouldScale, nodes, err := strategy.ShouldScaleUp(ctx, metrics, cfg)
		if err != nil {
			return false, 0, err
		}
		if shouldScale && nodes > maxNodes {
			maxNodes = nodes
		}
	}

	return maxNodes > 0, maxNodes, nil
}

// ShouldScaleDown checks all strategies - only scales down if all agree
func (s *CompositeStrategy) ShouldScaleDown(
	ctx context.Context,
	metrics provisioning.MetricsCollector,
	cfg *config.AutoScalingConfig,
) (bool, int, error) {
	minNodes := -1

	for _, strategy := range s.strategies {
		shouldScale, nodes, err := strategy.ShouldScaleDown(ctx, metrics, cfg)
		if err != nil {
			return false, 0, err
		}
		if !shouldScale {
			return false, 0, nil // If any strategy says no, don't scale down
		}
		if minNodes == -1 || nodes < minNodes {
			minNodes = nodes
		}
	}

	return minNodes > 0, minNodes, nil
}

// =============================================================================
// Custom Metric Strategy
// =============================================================================

// CustomMetricStrategy scales based on custom metrics
type CustomMetricStrategy struct {
	metricName string
	threshold  float64
	scaleUp    bool
}

// NewCustomMetricStrategy creates a new custom metric strategy
func NewCustomMetricStrategy(metricName string, threshold float64, scaleUp bool) *CustomMetricStrategy {
	return &CustomMetricStrategy{
		metricName: metricName,
		threshold:  threshold,
		scaleUp:    scaleUp,
	}
}

// Name returns the strategy name
func (s *CustomMetricStrategy) Name() string {
	return "custom:" + s.metricName
}

// ShouldScaleUp determines if cluster should scale up based on custom metric
func (s *CustomMetricStrategy) ShouldScaleUp(
	ctx context.Context,
	metrics provisioning.MetricsCollector,
	cfg *config.AutoScalingConfig,
) (bool, int, error) {
	if !cfg.Enabled || !s.scaleUp {
		return false, 0, nil
	}

	value, err := metrics.GetCustomMetric(ctx, s.metricName)
	if err != nil {
		return false, 0, fmt.Errorf("failed to get custom metric %s: %w", s.metricName, err)
	}

	if value > s.threshold {
		return true, 1, nil
	}

	return false, 0, nil
}

// ShouldScaleDown determines if cluster should scale down based on custom metric
func (s *CustomMetricStrategy) ShouldScaleDown(
	ctx context.Context,
	metrics provisioning.MetricsCollector,
	cfg *config.AutoScalingConfig,
) (bool, int, error) {
	if !cfg.Enabled || s.scaleUp {
		return false, 0, nil
	}

	value, err := metrics.GetCustomMetric(ctx, s.metricName)
	if err != nil {
		return false, 0, fmt.Errorf("failed to get custom metric %s: %w", s.metricName, err)
	}

	if value < s.threshold {
		return true, 1, nil
	}

	return false, 0, nil
}
