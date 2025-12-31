package provisioning_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/distribution"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/spot"
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// MOCK INFRASTRUCTURE - Simulates real cloud provider behavior
// =============================================================================

// MockCloudProvider simulates a cloud provider with realistic behavior
type MockCloudProvider struct {
	mu              sync.Mutex
	instances       map[string]*MockInstance
	spotPrices      map[string]float64 // zone -> price
	spotAvailable   map[string]bool    // zone -> available
	interruptionsCh chan string        // channel to simulate spot interruptions
}

type MockInstance struct {
	ID           string
	Name         string
	Zone         string
	IsSpot       bool
	SpotPrice    float64
	Status       string
	CreatedAt    time.Time
	PrivateIP    string
	PublicIP     string
	InstanceType string
}

func NewMockCloudProvider() *MockCloudProvider {
	return &MockCloudProvider{
		instances: make(map[string]*MockInstance),
		spotPrices: map[string]float64{
			"us-east-1a": 0.03,
			"us-east-1b": 0.05,
			"us-east-1c": 0.04,
			"us-west-2a": 0.025,
			"us-west-2b": 0.035,
		},
		spotAvailable: map[string]bool{
			"us-east-1a": true,
			"us-east-1b": true,
			"us-east-1c": true,
			"us-west-2a": true,
			"us-west-2b": false, // Simulate unavailable zone
		},
		interruptionsCh: make(chan string, 100),
	}
}

func (m *MockCloudProvider) CreateInstance(ctx context.Context, name, zone, instanceType string, isSpot bool) (*MockInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check spot availability
	if isSpot {
		if available, ok := m.spotAvailable[zone]; !ok || !available {
			return nil, fmt.Errorf("spot capacity not available in zone %s", zone)
		}
	}

	instance := &MockInstance{
		ID:           fmt.Sprintf("i-%s-%d", name, time.Now().UnixNano()),
		Name:         name,
		Zone:         zone,
		IsSpot:       isSpot,
		SpotPrice:    m.spotPrices[zone],
		Status:       "running",
		CreatedAt:    time.Now(),
		PrivateIP:    fmt.Sprintf("10.0.%d.%d", len(m.instances)/256, len(m.instances)%256+1),
		PublicIP:     fmt.Sprintf("54.%d.%d.%d", len(m.instances)/65536, (len(m.instances)/256)%256, len(m.instances)%256),
		InstanceType: instanceType,
	}

	m.instances[instance.ID] = instance
	return instance, nil
}

func (m *MockCloudProvider) TerminateInstance(ctx context.Context, instanceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.instances[instanceID]; !ok {
		return fmt.Errorf("instance %s not found", instanceID)
	}

	delete(m.instances, instanceID)
	return nil
}

func (m *MockCloudProvider) GetSpotPrice(zone string) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.spotPrices[zone]
}

func (m *MockCloudProvider) IsSpotAvailable(zone string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if available, ok := m.spotAvailable[zone]; ok {
		return available
	}
	return false
}

func (m *MockCloudProvider) SetSpotAvailable(zone string, available bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.spotAvailable[zone] = available
}

func (m *MockCloudProvider) SetSpotPrice(zone string, price float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.spotPrices[zone] = price
}

func (m *MockCloudProvider) GetInstances() []*MockInstance {
	m.mu.Lock()
	defer m.mu.Unlock()

	instances := make([]*MockInstance, 0, len(m.instances))
	for _, inst := range m.instances {
		instances = append(instances, inst)
	}
	return instances
}

// =============================================================================
// SPOT INSTANCE ADAPTER - Connects MockCloudProvider to spot.Manager
// =============================================================================

type MockSpotAdapter struct {
	provider *MockCloudProvider
}

func (a *MockSpotAdapter) CreateSpotInstance(ctx context.Context, nodeConfig *config.NodeConfig, spotConfig *config.SpotConfig) (*providers.NodeOutput, error) {
	instance, err := a.provider.CreateInstance(ctx, nodeConfig.Name, nodeConfig.Zone, nodeConfig.Size, true)
	if err != nil {
		return nil, err
	}

	return &providers.NodeOutput{
		Name:     instance.Name,
		Provider: nodeConfig.Provider,
		Region:   nodeConfig.Region,
	}, nil
}

func (a *MockSpotAdapter) GetSpotPrice(ctx context.Context, instanceType, zone string) (float64, error) {
	return a.provider.GetSpotPrice(zone), nil
}

func (a *MockSpotAdapter) IsSpotAvailable(ctx context.Context, instanceType, zone string) (bool, error) {
	return a.provider.IsSpotAvailable(zone), nil
}

func (a *MockSpotAdapter) HandleInterruption(ctx context.Context, nodeID string) error {
	return a.provider.TerminateInstance(ctx, nodeID)
}

// =============================================================================
// INTEGRATION TESTS - SPOT + MULTI-AZ
// =============================================================================

// TestIntegration_SpotWithMultiAZDistribution tests spot instances distributed across AZs
func TestIntegration_SpotWithMultiAZDistribution(t *testing.T) {
	ctx := context.Background()
	provider := NewMockCloudProvider()

	// Setup spot manager
	spotManager := spot.NewManager(&spot.ManagerConfig{
		Strategy: &spot.DefaultStrategy{},
	})
	spotManager.RegisterProvider("aws", &MockSpotAdapter{provider: provider})

	// Setup distribution
	distributor, err := distribution.NewDistributor(&distribution.DistributorConfig{
		StrategyName: "round_robin",
	})
	require.NoError(t, err)

	// Define zones and node count
	zones := []string{"us-east-1a", "us-east-1b", "us-east-1c"}
	nodeCount := 6

	// Calculate distribution
	zoneCounts, err := distributor.Distribute(ctx, nodeCount, zones)
	require.NoError(t, err)

	t.Logf("Distribution plan: %v", zoneCounts)

	// Verify even distribution
	assert.Equal(t, 2, zoneCounts["us-east-1a"])
	assert.Equal(t, 2, zoneCounts["us-east-1b"])
	assert.Equal(t, 2, zoneCounts["us-east-1c"])

	// Create spot instances according to distribution
	createdNodes := make([]*providers.NodeOutput, 0)
	for zone, count := range zoneCounts {
		for i := 0; i < count; i++ {
			nodeConfig := &config.NodeConfig{
				Name:     fmt.Sprintf("worker-%s-%d", zone, i),
				Provider: "aws",
				Region:   "us-east-1",
				Zone:     zone,
				Size:     "t3.medium",
			}
			spotConfig := &config.SpotConfig{
				Enabled:      true,
				MaxSpotPrice: 0.10,
			}

			output, err := spotManager.CreateSpotInstance(ctx, nodeConfig, spotConfig)
			require.NoError(t, err)
			createdNodes = append(createdNodes, output)
			t.Logf("Created spot instance: %s in zone %s", output.Name, zone)
		}
	}

	// Verify all nodes created
	assert.Len(t, createdNodes, 6)

	// Verify instances in provider
	instances := provider.GetInstances()
	assert.Len(t, instances, 6)

	// Verify zone distribution
	zoneDistribution := make(map[string]int)
	for _, inst := range instances {
		zoneDistribution[inst.Zone]++
	}
	assert.Equal(t, 2, zoneDistribution["us-east-1a"])
	assert.Equal(t, 2, zoneDistribution["us-east-1b"])
	assert.Equal(t, 2, zoneDistribution["us-east-1c"])

	// All should be spot instances
	for _, inst := range instances {
		assert.True(t, inst.IsSpot, "Instance %s should be spot", inst.Name)
	}

	t.Log("✅ All 6 spot instances created and distributed across 3 AZs")
}

// TestIntegration_SpotFallbackToOnDemand tests fallback when spot is unavailable
func TestIntegration_SpotFallbackToOnDemand(t *testing.T) {
	ctx := context.Background()
	provider := NewMockCloudProvider()

	// Make zone b unavailable for spot
	provider.SetSpotAvailable("us-east-1b", false)
	t.Log("Zone us-east-1b marked as unavailable for spot instances")

	spotManager := spot.NewManager(&spot.ManagerConfig{
		Strategy: &spot.DefaultStrategy{},
	})
	spotManager.RegisterProvider("aws", &MockSpotAdapter{provider: provider})

	// Try to create spot in unavailable zone
	nodeConfig := &config.NodeConfig{
		Name:     "worker-1",
		Provider: "aws",
		Region:   "us-east-1",
		Zone:     "us-east-1b",
		Size:     "t3.medium",
	}
	spotConfig := &config.SpotConfig{
		Enabled:          true,
		MaxSpotPrice:     0.10,
		FallbackOnDemand: true,
	}

	_, err := spotManager.CreateSpotInstance(ctx, nodeConfig, spotConfig)

	// Should fail with SpotUnavailableError
	assert.Error(t, err)

	spotErr, ok := err.(*spot.SpotUnavailableError)
	require.True(t, ok, "Error should be SpotUnavailableError")
	assert.True(t, spotErr.ShouldFallback(), "Should indicate fallback is allowed")

	t.Logf("Spot creation failed as expected: %v", err)
	t.Log("FallbackOnDemand is enabled, creating on-demand instance...")

	// Now create on-demand in that zone (simulating fallback)
	instance, err := provider.CreateInstance(ctx, "worker-1-ondemand", "us-east-1b", "t3.medium", false)
	require.NoError(t, err)
	assert.False(t, instance.IsSpot)

	t.Logf("✅ Fallback successful: Created on-demand instance %s in zone %s", instance.Name, instance.Zone)
}

// TestIntegration_SpotPriceTooHigh tests rejection when spot price exceeds max
func TestIntegration_SpotPriceTooHigh(t *testing.T) {
	ctx := context.Background()
	provider := NewMockCloudProvider()

	// Set high spot price
	provider.SetSpotPrice("us-east-1a", 0.50)
	t.Logf("Zone us-east-1a spot price set to $0.50/hr")

	spotManager := spot.NewManager(&spot.ManagerConfig{
		Strategy: &spot.DefaultStrategy{},
	})
	spotManager.RegisterProvider("aws", &MockSpotAdapter{provider: provider})

	nodeConfig := &config.NodeConfig{
		Name:     "worker-1",
		Provider: "aws",
		Region:   "us-east-1",
		Zone:     "us-east-1a",
		Size:     "t3.medium",
	}
	spotConfig := &config.SpotConfig{
		Enabled:          true,
		MaxSpotPrice:     0.10, // Max is $0.10, but price is $0.50
		FallbackOnDemand: true,
	}

	t.Logf("Attempting to create spot instance with max price $0.10/hr...")

	_, err := spotManager.CreateSpotInstance(ctx, nodeConfig, spotConfig)

	assert.Error(t, err)
	spotErr, ok := err.(*spot.SpotUnavailableError)
	require.True(t, ok)
	assert.Equal(t, "price_too_high", spotErr.Reason)

	t.Logf("✅ Spot creation rejected: %s (price $0.50 > max $0.10)", spotErr.Reason)
}

// TestIntegration_SpotSelectCheapestZone tests zone selection based on price
func TestIntegration_SpotSelectCheapestZone(t *testing.T) {
	strategy := &spot.DefaultStrategy{}

	zones := []string{"us-east-1a", "us-east-1b", "us-east-1c"}
	prices := map[string]float64{
		"us-east-1a": 0.05,
		"us-east-1b": 0.02, // Cheapest
		"us-east-1c": 0.08,
	}

	t.Log("Zone prices:")
	for zone, price := range prices {
		t.Logf("  %s: $%.3f/hr", zone, price)
	}

	bestZone := strategy.SelectBestZone(context.Background(), zones, prices)
	assert.Equal(t, "us-east-1b", bestZone, "Should select cheapest zone")

	t.Logf("✅ Strategy selected cheapest zone: %s ($%.3f/hr)", bestZone, prices[bestZone])
}

// TestIntegration_SpotInterruptionHandling tests handling of spot interruptions
func TestIntegration_SpotInterruptionHandling(t *testing.T) {
	ctx := context.Background()
	provider := NewMockCloudProvider()

	// Track interruption handling
	interruptionHandled := make(chan string, 10)

	spotManager := spot.NewManager(&spot.ManagerConfig{})
	spotManager.RegisterProvider("aws", &MockSpotAdapter{provider: provider})

	// Register interruption handler
	handler := &MockInterruptionHandler{
		onInterrupt: func(nodeID string) error {
			interruptionHandled <- nodeID
			t.Logf("Interruption handler called for node: %s", nodeID)
			return nil
		},
	}
	spotManager.RegisterInterruptionHandler(handler)

	// Create spot instance
	instance, err := provider.CreateInstance(ctx, "worker-spot-1", "us-east-1a", "t3.medium", true)
	require.NoError(t, err)
	t.Logf("Created spot instance: %s", instance.ID)

	// Simulate interruption
	t.Log("Simulating AWS 2-minute interruption warning...")
	err = spotManager.HandleInterruption(ctx, "aws", instance.ID)
	require.NoError(t, err)

	// Verify handler was called
	select {
	case handledID := <-interruptionHandled:
		assert.Equal(t, instance.ID, handledID)
		t.Logf("✅ Interruption handled successfully for: %s", handledID)
	case <-time.After(time.Second):
		t.Fatal("Interruption handler not called")
	}

	// Verify instance was terminated
	instances := provider.GetInstances()
	assert.Len(t, instances, 0, "Instance should be terminated after interruption")
	t.Log("✅ Instance terminated after interruption handling")
}

type MockInterruptionHandler struct {
	onInterrupt func(nodeID string) error
}

func (h *MockInterruptionHandler) Name() string {
	return "mock-handler"
}

func (h *MockInterruptionHandler) HandleInterruption(ctx context.Context, nodeID string) error {
	if h.onInterrupt != nil {
		return h.onInterrupt(nodeID)
	}
	return nil
}

// =============================================================================
// FULL CLUSTER PROVISIONING SCENARIO
// =============================================================================

// TestIntegration_FullClusterProvisioningScenario tests a complete cluster provisioning scenario
func TestIntegration_FullClusterProvisioningScenario(t *testing.T) {
	ctx := context.Background()
	provider := NewMockCloudProvider()

	t.Log("=== Starting Full Cluster Provisioning Scenario ===")
	t.Log("Target: 3 masters (on-demand) + 6 workers (spot) across 3 AZs")

	// Setup distribution
	zones := []string{"us-east-1a", "us-east-1b", "us-east-1c"}
	distributor, err := distribution.NewDistributor(&distribution.DistributorConfig{
		StrategyName: "round_robin",
	})
	require.NoError(t, err)

	// Step 1: Plan distribution
	t.Log("\n📋 Step 1: Planning distribution...")
	masterDist, err := distributor.Distribute(ctx, 3, zones)
	require.NoError(t, err)
	workerDist, err := distributor.Distribute(ctx, 6, zones)
	require.NoError(t, err)

	t.Logf("  Master distribution: %v", masterDist)
	t.Logf("  Worker distribution: %v", workerDist)

	// Step 2: Create masters (on-demand for stability)
	t.Log("\n🖥️  Step 2: Creating master nodes (on-demand)...")
	masterInstances := make([]*MockInstance, 0)
	masterIdx := 0
	for zone, count := range masterDist {
		for i := 0; i < count; i++ {
			instance, err := provider.CreateInstance(ctx,
				fmt.Sprintf("master-%d", masterIdx),
				zone,
				"m5.large",
				false, // On-demand
			)
			require.NoError(t, err)
			masterInstances = append(masterInstances, instance)
			t.Logf("  Created master-%d in %s (on-demand)", masterIdx, zone)
			masterIdx++
		}
	}
	assert.Len(t, masterInstances, 3)

	// Step 3: Create workers (spot instances)
	t.Log("\n⚡ Step 3: Creating worker nodes (spot)...")
	spotManager := spot.NewManager(&spot.ManagerConfig{
		Strategy: &spot.DefaultStrategy{},
	})
	spotManager.RegisterProvider("aws", &MockSpotAdapter{provider: provider})

	workerIdx := 0
	for zone, count := range workerDist {
		spotPrice := provider.GetSpotPrice(zone)
		for i := 0; i < count; i++ {
			nodeConfig := &config.NodeConfig{
				Name:     fmt.Sprintf("worker-%d", workerIdx),
				Provider: "aws",
				Region:   "us-east-1",
				Zone:     zone,
				Size:     "t3.large",
			}
			spotConfig := &config.SpotConfig{
				Enabled:      true,
				MaxSpotPrice: 0.10,
			}

			_, err := spotManager.CreateSpotInstance(ctx, nodeConfig, spotConfig)
			require.NoError(t, err)
			t.Logf("  Created worker-%d in %s (spot @ $%.3f/hr)", workerIdx, zone, spotPrice)
			workerIdx++
		}
	}

	// Step 4: Verify cluster state
	t.Log("\n📊 Step 4: Verifying cluster state...")
	allInstances := provider.GetInstances()
	assert.Len(t, allInstances, 9, "Should have 3 masters + 6 workers")

	// Count spot vs on-demand
	spotCount := 0
	onDemandCount := 0
	totalSpotCost := 0.0
	for _, inst := range allInstances {
		if inst.IsSpot {
			spotCount++
			totalSpotCost += inst.SpotPrice
		} else {
			onDemandCount++
		}
	}

	// Verify counts
	assert.Equal(t, 3, onDemandCount, "Should have 3 on-demand masters")
	assert.Equal(t, 6, spotCount, "Should have 6 spot workers")

	// Verify zone distribution
	zoneDistribution := make(map[string]int)
	for _, inst := range allInstances {
		zoneDistribution[inst.Zone]++
	}

	t.Log("\n=== Final Cluster State ===")
	t.Logf("  Total instances: %d", len(allInstances))
	t.Logf("  Masters (on-demand): %d", onDemandCount)
	t.Logf("  Workers (spot): %d", spotCount)
	t.Logf("  Zone distribution: %v", zoneDistribution)
	t.Logf("  Estimated hourly spot cost: $%.3f", totalSpotCost)

	// Calculate savings
	onDemandPrice := 0.10 // Assumed on-demand price per worker
	spotSavings := (onDemandPrice*6 - totalSpotCost) / (onDemandPrice * 6) * 100
	t.Logf("  Spot savings vs on-demand: %.1f%%", spotSavings)

	// Each zone should have 3 instances (1 master + 2 workers)
	for zone, count := range zoneDistribution {
		assert.Equal(t, 3, count, "Zone %s should have 3 instances", zone)
	}

	t.Log("\n✅ Full cluster provisioning completed successfully!")
}

// TestIntegration_MultiAZFailover tests failover when a zone becomes unavailable
func TestIntegration_MultiAZFailover(t *testing.T) {
	ctx := context.Background()
	provider := NewMockCloudProvider()

	t.Log("=== Testing Multi-AZ Failover Scenario ===")

	// Initially all zones available
	zones := []string{"us-east-1a", "us-east-1b", "us-east-1c"}

	distributor, err := distribution.NewDistributor(&distribution.DistributorConfig{
		StrategyName: "round_robin",
	})
	require.NoError(t, err)

	// Create initial distribution
	t.Log("\n📋 Initial state: 6 nodes across 3 zones")
	dist, err := distributor.Distribute(ctx, 6, zones)
	require.NoError(t, err)

	for zone, count := range dist {
		for i := 0; i < count; i++ {
			_, err := provider.CreateInstance(ctx, fmt.Sprintf("node-%s-%d", zone, i), zone, "t3.medium", false)
			require.NoError(t, err)
		}
	}

	initialInstances := provider.GetInstances()
	t.Logf("  Created %d instances", len(initialInstances))

	// Simulate zone failure
	t.Log("\n⚠️  Simulating zone failure: us-east-1c becomes unavailable")
	provider.SetSpotAvailable("us-east-1c", false)

	// Plan new nodes only in available zones
	remainingZones := []string{"us-east-1a", "us-east-1b"}
	t.Log("\n📋 Planning 4 new nodes in remaining zones...")
	newDist, err := distributor.Distribute(ctx, 4, remainingZones)
	require.NoError(t, err)

	t.Logf("  New distribution (excluding failed zone): %v", newDist)

	// Should distribute evenly across remaining zones
	assert.Equal(t, 2, newDist["us-east-1a"])
	assert.Equal(t, 2, newDist["us-east-1b"])

	t.Log("\n✅ Failover planning successful - workload redistributed to healthy zones")
}

// TestIntegration_CostOptimization tests cost-aware spot instance selection
func TestIntegration_CostOptimization(t *testing.T) {
	provider := NewMockCloudProvider()

	t.Log("=== Testing Cost Optimization ===")

	// Set varying prices across zones
	provider.SetSpotPrice("us-east-1a", 0.08)
	provider.SetSpotPrice("us-east-1b", 0.03) // Cheapest
	provider.SetSpotPrice("us-east-1c", 0.05)

	t.Log("\nZone spot prices:")
	t.Logf("  us-east-1a: $0.08/hr")
	t.Logf("  us-east-1b: $0.03/hr (cheapest)")
	t.Logf("  us-east-1c: $0.05/hr")

	strategy := &spot.DefaultStrategy{}

	zones := []string{"us-east-1a", "us-east-1b", "us-east-1c"}
	prices := map[string]float64{
		"us-east-1a": provider.GetSpotPrice("us-east-1a"),
		"us-east-1b": provider.GetSpotPrice("us-east-1b"),
		"us-east-1c": provider.GetSpotPrice("us-east-1c"),
	}

	// Test consistent zone selection
	t.Log("\nTesting zone selection consistency (10 iterations)...")
	for i := 0; i < 10; i++ {
		bestZone := strategy.SelectBestZone(context.Background(), zones, prices)
		assert.Equal(t, "us-east-1b", bestZone, "Should always select cheapest zone")
	}
	t.Log("  ✓ Strategy consistently selects cheapest zone")

	// Calculate potential savings
	onDemandPrice := 0.10 // Assumed on-demand price
	spotPrice := prices["us-east-1b"]
	savings := (onDemandPrice - spotPrice) / onDemandPrice * 100

	t.Log("\n📊 Cost Analysis:")
	t.Logf("  On-demand price: $%.2f/hr", onDemandPrice)
	t.Logf("  Spot price (best zone): $%.3f/hr", spotPrice)
	t.Logf("  Savings per instance: %.1f%%", savings)

	// For a 6-node worker pool running 24/7 for a month
	monthlyOnDemand := onDemandPrice * 6 * 24 * 30
	monthlySpot := spotPrice * 6 * 24 * 30
	monthlySavings := monthlyOnDemand - monthlySpot

	t.Logf("\n  Monthly cost (6 workers, 24/7):")
	t.Logf("    On-demand: $%.2f", monthlyOnDemand)
	t.Logf("    Spot: $%.2f", monthlySpot)
	t.Logf("    Savings: $%.2f (%.1f%%)", monthlySavings, savings)

	assert.True(t, savings > 50, "Should save more than 50%% with spot")
	t.Log("\n✅ Cost optimization verified - significant savings achieved with spot instances")
}

// TestIntegration_SpotPercentageStrategy tests mixed spot/on-demand deployment
func TestIntegration_SpotPercentageStrategy(t *testing.T) {
	ctx := context.Background()
	provider := NewMockCloudProvider()

	t.Log("=== Testing Spot Percentage Strategy (50% spot) ===")

	spotManager := spot.NewManager(&spot.ManagerConfig{
		Strategy: &spot.DefaultStrategy{},
	})
	spotManager.RegisterProvider("aws", &MockSpotAdapter{provider: provider})

	// Create 10 workers with 50% spot target
	totalWorkers := 10
	spotPercentage := 50
	expectedSpot := totalWorkers * spotPercentage / 100

	t.Logf("\nTarget: %d workers, %d%% spot = %d spot + %d on-demand",
		totalWorkers, spotPercentage, expectedSpot, totalWorkers-expectedSpot)

	spotConfig := &config.SpotConfig{
		Enabled:        true,
		MaxSpotPrice:   0.10,
		SpotPercentage: spotPercentage,
	}

	strategy := &spot.DefaultStrategy{}

	for i := 0; i < totalWorkers; i++ {
		useSpot := strategy.ShouldUseSpot(spotConfig, i, totalWorkers)

		nodeConfig := &config.NodeConfig{
			Name:     fmt.Sprintf("worker-%d", i),
			Provider: "aws",
			Region:   "us-east-1",
			Zone:     "us-east-1a",
			Size:     "t3.medium",
		}

		if useSpot {
			_, err := spotManager.CreateSpotInstance(ctx, nodeConfig, spotConfig)
			require.NoError(t, err)
			t.Logf("  worker-%d: spot", i)
		} else {
			_, err := provider.CreateInstance(ctx, nodeConfig.Name, nodeConfig.Zone, nodeConfig.Size, false)
			require.NoError(t, err)
			t.Logf("  worker-%d: on-demand", i)
		}
	}

	// Count results
	instances := provider.GetInstances()
	spotCount := 0
	for _, inst := range instances {
		if inst.IsSpot {
			spotCount++
		}
	}

	t.Logf("\n📊 Results:")
	t.Logf("  Total instances: %d", len(instances))
	t.Logf("  Spot instances: %d (%.0f%%)", spotCount, float64(spotCount)/float64(len(instances))*100)
	t.Logf("  On-demand instances: %d (%.0f%%)", len(instances)-spotCount, float64(len(instances)-spotCount)/float64(len(instances))*100)

	assert.Equal(t, expectedSpot, spotCount, "Should have %d spot instances", expectedSpot)
	t.Log("\n✅ Spot percentage strategy working correctly")
}
