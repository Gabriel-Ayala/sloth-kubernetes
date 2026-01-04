// Package distribution provides zone distribution strategies for node pools
package distribution

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning"
)

// =============================================================================
// Strategy Registry
// =============================================================================

// StrategyRegistry manages distribution strategies
type StrategyRegistry struct {
	strategies map[string]provisioning.DistributionStrategy
	mu         sync.RWMutex
}

// NewStrategyRegistry creates a new registry with default strategies
func NewStrategyRegistry() *StrategyRegistry {
	registry := &StrategyRegistry{
		strategies: make(map[string]provisioning.DistributionStrategy),
	}

	// Register default strategies
	registry.Register(&RoundRobinStrategy{})
	registry.Register(&WeightedStrategy{})
	registry.Register(&PackedStrategy{})
	registry.Register(&SpreadStrategy{})

	return registry
}

// Register adds a strategy to the registry
func (r *StrategyRegistry) Register(strategy provisioning.DistributionStrategy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.strategies[strategy.Name()] = strategy
}

// Get retrieves a strategy by name
func (r *StrategyRegistry) Get(name string) (provisioning.DistributionStrategy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	strategy, exists := r.strategies[name]
	if !exists {
		return nil, fmt.Errorf("distribution strategy %s not found", name)
	}
	return strategy, nil
}

// =============================================================================
// Round Robin Strategy
// =============================================================================

// RoundRobinStrategy distributes nodes evenly across zones
type RoundRobinStrategy struct{}

// Name returns the strategy name
func (s *RoundRobinStrategy) Name() string {
	return "round_robin"
}

// Calculate distributes nodes evenly across zones
func (s *RoundRobinStrategy) Calculate(totalNodes int, zones []string, weights map[string]int) map[string]int {
	if len(zones) == 0 {
		return map[string]int{}
	}

	result := make(map[string]int)

	// Initialize all zones with 0
	for _, zone := range zones {
		result[zone] = 0
	}

	// Distribute nodes round-robin style
	for i := 0; i < totalNodes; i++ {
		zone := zones[i%len(zones)]
		result[zone]++
	}

	return result
}

// =============================================================================
// Weighted Strategy
// =============================================================================

// WeightedStrategy distributes nodes based on weights
type WeightedStrategy struct{}

// Name returns the strategy name
func (s *WeightedStrategy) Name() string {
	return "weighted"
}

// Calculate distributes nodes based on zone weights
func (s *WeightedStrategy) Calculate(totalNodes int, zones []string, weights map[string]int) map[string]int {
	if len(zones) == 0 {
		return map[string]int{}
	}

	result := make(map[string]int)

	// Calculate total weight
	totalWeight := 0
	for _, zone := range zones {
		weight := 1 // Default weight
		if w, ok := weights[zone]; ok && w > 0 {
			weight = w
		}
		totalWeight += weight
	}

	// Distribute based on weight ratio
	assigned := 0
	for _, zone := range zones {
		weight := 1
		if w, ok := weights[zone]; ok && w > 0 {
			weight = w
		}

		// Calculate proportion
		nodes := (totalNodes * weight) / totalWeight
		result[zone] = nodes
		assigned += nodes
	}

	// Distribute remaining nodes
	remaining := totalNodes - assigned
	for i := 0; remaining > 0; i++ {
		zone := zones[i%len(zones)]
		result[zone]++
		remaining--
	}

	return result
}

// =============================================================================
// Packed Strategy - Fills zones to capacity before moving to next
// =============================================================================

// PackedStrategy fills zones sequentially
type PackedStrategy struct {
	zoneCapacity int
}

// NewPackedStrategy creates a new packed strategy
func NewPackedStrategy(zoneCapacity int) *PackedStrategy {
	if zoneCapacity <= 0 {
		zoneCapacity = 10 // Default capacity per zone
	}
	return &PackedStrategy{zoneCapacity: zoneCapacity}
}

// Name returns the strategy name
func (s *PackedStrategy) Name() string {
	return "packed"
}

// Calculate fills zones to capacity before moving to next
func (s *PackedStrategy) Calculate(totalNodes int, zones []string, weights map[string]int) map[string]int {
	if len(zones) == 0 {
		return map[string]int{}
	}

	result := make(map[string]int)
	for _, zone := range zones {
		result[zone] = 0
	}

	remaining := totalNodes
	for _, zone := range zones {
		capacity := s.zoneCapacity
		if w, ok := weights[zone]; ok && w > 0 {
			capacity = w // Use weight as capacity if provided
		}

		toAssign := capacity
		if toAssign > remaining {
			toAssign = remaining
		}

		result[zone] = toAssign
		remaining -= toAssign

		if remaining <= 0 {
			break
		}
	}

	return result
}

// =============================================================================
// Spread Strategy - Maximizes spread across zones
// =============================================================================

// SpreadStrategy maximizes distribution across all zones
type SpreadStrategy struct{}

// Name returns the strategy name
func (s *SpreadStrategy) Name() string {
	return "spread"
}

// Calculate ensures maximum spread across zones
func (s *SpreadStrategy) Calculate(totalNodes int, zones []string, weights map[string]int) map[string]int {
	if len(zones) == 0 {
		return map[string]int{}
	}

	result := make(map[string]int)
	for _, zone := range zones {
		result[zone] = 0
	}

	// Each zone gets at least 1 node if possible
	nodesPerZone := totalNodes / len(zones)
	remaining := totalNodes % len(zones)

	for _, zone := range zones {
		result[zone] = nodesPerZone
	}

	// Distribute remaining nodes to zones with highest weights
	if remaining > 0 {
		// Sort zones by weight (descending)
		sortedZones := make([]string, len(zones))
		copy(sortedZones, zones)
		sort.Slice(sortedZones, func(i, j int) bool {
			wi := weights[sortedZones[i]]
			wj := weights[sortedZones[j]]
			return wi > wj
		})

		for i := 0; remaining > 0 && i < len(sortedZones); i++ {
			result[sortedZones[i]]++
			remaining--
		}
	}

	return result
}

// =============================================================================
// Zone Distributor Manager
// =============================================================================

// Distributor manages node distribution across zones
type Distributor struct {
	strategy     provisioning.DistributionStrategy
	registry     *StrategyRegistry
	eventEmitter provisioning.EventEmitter
	mu           sync.RWMutex
}

// DistributorConfig holds distributor configuration
type DistributorConfig struct {
	StrategyName string
	EventEmitter provisioning.EventEmitter
}

// NewDistributor creates a new zone distributor
func NewDistributor(cfg *DistributorConfig) (*Distributor, error) {
	registry := NewStrategyRegistry()

	strategyName := cfg.StrategyName
	if strategyName == "" {
		strategyName = "round_robin"
	}

	strategy, err := registry.Get(strategyName)
	if err != nil {
		strategy, _ = registry.Get("round_robin") // Fallback
	}

	return &Distributor{
		strategy:     strategy,
		registry:     registry,
		eventEmitter: cfg.EventEmitter,
	}, nil
}

// Distribute calculates node distribution for zones
func (d *Distributor) Distribute(ctx context.Context, totalNodes int, zones []string) (map[string]int, error) {
	d.mu.RLock()
	strategy := d.strategy
	d.mu.RUnlock()

	if len(zones) == 0 {
		return nil, fmt.Errorf("no zones provided for distribution")
	}

	distribution := strategy.Calculate(totalNodes, zones, nil)

	return distribution, nil
}

// DistributeWithConfig calculates distribution based on zone distribution config
func (d *Distributor) DistributeWithConfig(
	ctx context.Context,
	totalNodes int,
	distributions []config.ZoneDistribution,
) (map[string]int, error) {
	d.mu.RLock()
	strategy := d.strategy
	d.mu.RUnlock()

	if len(distributions) == 0 {
		return nil, fmt.Errorf("no zone distributions provided")
	}

	// Check if explicit counts are provided
	explicitTotal := 0
	for _, dist := range distributions {
		explicitTotal += dist.Count
	}

	// If explicit counts match total, use those
	if explicitTotal == totalNodes {
		result := make(map[string]int)
		for _, dist := range distributions {
			result[dist.Zone] = dist.Count
		}
		return result, nil
	}

	// Otherwise, use counts as weights
	zones := make([]string, 0, len(distributions))
	weights := make(map[string]int)

	for _, dist := range distributions {
		zones = append(zones, dist.Zone)
		if dist.Count > 0 {
			weights[dist.Zone] = dist.Count
		}
	}

	return strategy.Calculate(totalNodes, zones, weights), nil
}

// Rebalance recalculates distribution for existing nodes
func (d *Distributor) Rebalance(ctx context.Context, currentDistribution map[string]int) (map[string]int, error) {
	d.mu.RLock()
	strategy := d.strategy
	d.mu.RUnlock()

	// Calculate total nodes
	totalNodes := 0
	zones := make([]string, 0, len(currentDistribution))
	for zone, count := range currentDistribution {
		totalNodes += count
		zones = append(zones, zone)
	}

	// Calculate ideal distribution
	idealDistribution := strategy.Calculate(totalNodes, zones, nil)

	// Calculate changes needed
	changes := make(map[string]int)
	for zone, idealCount := range idealDistribution {
		currentCount := currentDistribution[zone]
		if idealCount != currentCount {
			changes[zone] = idealCount - currentCount // Positive = add, Negative = remove
		}
	}

	return changes, nil
}

// SetStrategy changes the distribution strategy
func (d *Distributor) SetStrategy(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	strategy, err := d.registry.Get(name)
	if err != nil {
		return err
	}

	d.strategy = strategy
	return nil
}

// GetDistributionPlan creates a detailed distribution plan
func (d *Distributor) GetDistributionPlan(
	totalNodes int,
	zones []string,
	weights map[string]int,
) *DistributionPlan {
	d.mu.RLock()
	strategy := d.strategy
	d.mu.RUnlock()

	distribution := strategy.Calculate(totalNodes, zones, weights)

	plan := &DistributionPlan{
		TotalNodes:   totalNodes,
		Strategy:     strategy.Name(),
		Zones:        make([]ZonePlan, 0, len(zones)),
		Distribution: distribution,
	}

	for _, zone := range zones {
		count := distribution[zone]
		percentage := float64(count) / float64(totalNodes) * 100

		plan.Zones = append(plan.Zones, ZonePlan{
			Zone:       zone,
			NodeCount:  count,
			Percentage: percentage,
			Weight:     weights[zone],
		})
	}

	return plan
}

// DistributionPlan represents a detailed distribution plan
type DistributionPlan struct {
	TotalNodes   int
	Strategy     string
	Zones        []ZonePlan
	Distribution map[string]int
}

// ZonePlan represents plan for a single zone
type ZonePlan struct {
	Zone       string
	NodeCount  int
	Percentage float64
	Weight     int
}
