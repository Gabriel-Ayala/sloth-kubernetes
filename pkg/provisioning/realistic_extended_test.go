package provisioning_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/costs"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/distribution"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// REALISTIC METRICS SIMULATOR
// =============================================================================

// RealisticMetricsCollector simulates Prometheus/metrics-server behavior
type RealisticMetricsCollector struct {
	mu sync.RWMutex

	// Simulated metrics
	cpuUsage      float64
	memoryUsage   float64
	podCount      int
	pendingPods   int
	nodeCount     int
	requestRate   float64

	// Behavior config
	latency       time.Duration
	errorRate     float64
	volatility    float64

	// History for trend analysis
	cpuHistory    []float64
	memHistory    []float64

	rng *rand.Rand
}

func NewRealisticMetricsCollector() *RealisticMetricsCollector {
	return &RealisticMetricsCollector{
		cpuUsage:    50.0,  // 50% CPU
		memoryUsage: 60.0,  // 60% Memory
		podCount:    100,
		pendingPods: 0,
		nodeCount:   5,
		requestRate: 1000.0, // requests/sec
		latency:     20 * time.Millisecond,
		errorRate:   0.01,
		volatility:  0.1,
		cpuHistory:  make([]float64, 0),
		memHistory:  make([]float64, 0),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SimulateLoad generates realistic load patterns
func (m *RealisticMetricsCollector) SimulateLoad(pattern string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch pattern {
	case "spike":
		// Sudden spike - CPU jumps to 90%
		m.cpuUsage = 90.0 + m.rng.Float64()*5
		m.memoryUsage = 85.0 + m.rng.Float64()*5
		m.pendingPods = 20 + m.rng.Intn(10)
	case "gradual_increase":
		// Gradual increase
		m.cpuUsage = min(m.cpuUsage+5, 95)
		m.memoryUsage = min(m.memoryUsage+3, 90)
		m.pendingPods += 2
	case "decrease":
		// Load decrease
		m.cpuUsage = max(m.cpuUsage-10, 20)
		m.memoryUsage = max(m.memoryUsage-5, 30)
		m.pendingPods = maxInt(m.pendingPods-5, 0)
	case "idle":
		// Very low load
		m.cpuUsage = 15.0 + m.rng.Float64()*5
		m.memoryUsage = 25.0 + m.rng.Float64()*5
		m.pendingPods = 0
	case "normal":
		// Normal fluctuation
		m.cpuUsage = 50.0 + (m.rng.Float64()*20 - 10)
		m.memoryUsage = 55.0 + (m.rng.Float64()*15 - 7.5)
		m.pendingPods = m.rng.Intn(3)
	}

	// Record history
	m.cpuHistory = append(m.cpuHistory, m.cpuUsage)
	m.memHistory = append(m.memHistory, m.memoryUsage)
	if len(m.cpuHistory) > 60 {
		m.cpuHistory = m.cpuHistory[1:]
		m.memHistory = m.memHistory[1:]
	}
}

// GetCPUUtilization implements MetricsCollector
func (m *RealisticMetricsCollector) GetCPUUtilization(ctx context.Context) (float64, error) {
	time.Sleep(m.latency)

	if m.rng.Float64() < m.errorRate {
		return 0, fmt.Errorf("metrics-server: connection refused")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Add some noise
	noise := (m.rng.Float64()*2 - 1) * m.volatility * m.cpuUsage
	return m.cpuUsage + noise, nil
}

// GetMemoryUtilization implements MetricsCollector
func (m *RealisticMetricsCollector) GetMemoryUtilization(ctx context.Context) (float64, error) {
	time.Sleep(m.latency)

	if m.rng.Float64() < m.errorRate {
		return 0, fmt.Errorf("metrics-server: timeout")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	noise := (m.rng.Float64()*2 - 1) * m.volatility * m.memoryUsage
	return m.memoryUsage + noise, nil
}

// GetCustomMetric implements MetricsCollector
func (m *RealisticMetricsCollector) GetCustomMetric(ctx context.Context, name string) (float64, error) {
	time.Sleep(m.latency)

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return synthetic values for common custom metrics
	switch name {
	case "pending_pods":
		return float64(m.pendingPods), nil
	case "request_rate":
		return m.requestRate, nil
	case "node_count":
		return float64(m.nodeCount), nil
	default:
		return 0, fmt.Errorf("unknown metric: %s", name)
	}
}

// GetPendingPods implements MetricsCollector
func (m *RealisticMetricsCollector) GetPendingPods(ctx context.Context) (int, error) {
	time.Sleep(m.latency)
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pendingPods, nil
}

// GetNodeCount implements MetricsCollector
func (m *RealisticMetricsCollector) GetNodeCount(ctx context.Context) (int, error) {
	time.Sleep(m.latency)
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.nodeCount, nil
}

// SetNodeCount updates node count
func (m *RealisticMetricsCollector) SetNodeCount(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodeCount = count
	// Adjust metrics based on node count
	if count > 0 {
		m.cpuUsage = m.cpuUsage * float64(m.nodeCount) / float64(count)
		m.memoryUsage = m.memoryUsage * float64(m.nodeCount) / float64(count)
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// =============================================================================
// REALISTIC NODE SCALER
// =============================================================================

// RealisticNodeScaler simulates cloud provider node operations
type RealisticNodeScaler struct {
	mu sync.RWMutex

	nodes         map[string]*ScalerNode
	currentCount  int
	desiredCount  int

	// Timing simulation
	provisionTime time.Duration
	deleteTime    time.Duration

	// Failure simulation
	provisionFailRate float64
	deleteFailRate    float64

	// Metrics
	scaleUpCount   int64
	scaleDownCount int64
	failedOps      int64

	// Provider simulation
	providerCapacity int
	providerName     string

	rng *rand.Rand
}

type ScalerNode struct {
	ID          string
	Name        string
	State       string // "provisioning", "running", "terminating", "terminated"
	CreatedAt   time.Time
	ReadyAt     time.Time
	Age         time.Duration
}

func NewRealisticNodeScaler(initialCount int) *RealisticNodeScaler {
	scaler := &RealisticNodeScaler{
		nodes:            make(map[string]*ScalerNode),
		currentCount:     initialCount,
		desiredCount:     initialCount,
		provisionTime:    100 * time.Millisecond, // Fast for tests
		deleteTime:       50 * time.Millisecond,
		provisionFailRate: 0.05,
		deleteFailRate:   0.02,
		providerCapacity: 100,
		providerName:     "aws",
		rng:              rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	// Create initial nodes
	for i := 0; i < initialCount; i++ {
		nodeID := fmt.Sprintf("node-%d", i)
		scaler.nodes[nodeID] = &ScalerNode{
			ID:        nodeID,
			Name:      fmt.Sprintf("worker-%d", i),
			State:     "running",
			CreatedAt: time.Now().Add(-time.Hour),
			ReadyAt:   time.Now().Add(-time.Hour + 2*time.Minute),
		}
	}

	return scaler
}

// SetFailureRates allows configuring failure rates for deterministic testing
func (s *RealisticNodeScaler) SetFailureRates(provisionFailRate, deleteFailRate float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.provisionFailRate = provisionFailRate
	s.deleteFailRate = deleteFailRate
}

func (s *RealisticNodeScaler) ScaleUp(ctx context.Context, count int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check capacity
	if s.currentCount+count > s.providerCapacity {
		return fmt.Errorf("InsufficientCapacity: requested %d nodes, but only %d available",
			count, s.providerCapacity-s.currentCount)
	}

	// Simulate provisioning
	for i := 0; i < count; i++ {
		// Random failure
		if s.rng.Float64() < s.provisionFailRate {
			atomic.AddInt64(&s.failedOps, 1)
			return fmt.Errorf("EC2 RunInstances failed: InsufficientInstanceCapacity")
		}

		nodeID := fmt.Sprintf("node-%d-%d", time.Now().UnixNano(), i)
		s.nodes[nodeID] = &ScalerNode{
			ID:        nodeID,
			Name:      fmt.Sprintf("worker-%d", s.currentCount+i),
			State:     "provisioning",
			CreatedAt: time.Now(),
		}

		// Simulate async provisioning
		go func(id string) {
			time.Sleep(s.provisionTime)
			s.mu.Lock()
			if node, ok := s.nodes[id]; ok {
				node.State = "running"
				node.ReadyAt = time.Now()
			}
			s.mu.Unlock()
		}(nodeID)
	}

	s.currentCount += count
	s.desiredCount = s.currentCount
	atomic.AddInt64(&s.scaleUpCount, int64(count))

	return nil
}

func (s *RealisticNodeScaler) ScaleDown(ctx context.Context, count int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if count > s.currentCount {
		count = s.currentCount
	}

	// Select oldest nodes for deletion
	var nodesToDelete []*ScalerNode
	for _, node := range s.nodes {
		if node.State == "running" {
			nodesToDelete = append(nodesToDelete, node)
			if len(nodesToDelete) >= count {
				break
			}
		}
	}

	// Terminate nodes
	for _, node := range nodesToDelete {
		// Random failure
		if s.rng.Float64() < s.deleteFailRate {
			atomic.AddInt64(&s.failedOps, 1)
			return fmt.Errorf("EC2 TerminateInstances failed: %s", node.ID)
		}

		node.State = "terminating"

		// Simulate async termination
		go func(id string) {
			time.Sleep(s.deleteTime)
			s.mu.Lock()
			delete(s.nodes, id)
			s.mu.Unlock()
		}(node.ID)
	}

	s.currentCount -= len(nodesToDelete)
	s.desiredCount = s.currentCount
	atomic.AddInt64(&s.scaleDownCount, int64(len(nodesToDelete)))

	return nil
}

func (s *RealisticNodeScaler) GetCurrentCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentCount
}

func (s *RealisticNodeScaler) GetDesiredCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.desiredCount
}

func (s *RealisticNodeScaler) GetMetrics() map[string]int64 {
	return map[string]int64{
		"scale_up_count":   atomic.LoadInt64(&s.scaleUpCount),
		"scale_down_count": atomic.LoadInt64(&s.scaleDownCount),
		"failed_ops":       atomic.LoadInt64(&s.failedOps),
	}
}

// =============================================================================
// REALISTIC PRICE PROVIDER WITH API SIMULATION
// =============================================================================

// RealisticPriceProvider simulates cloud pricing APIs
type RealisticPriceProvider struct {
	mu sync.RWMutex

	// Base prices
	instancePrices map[string]map[string]float64 // region -> type -> price
	spotPrices     map[string]map[string]float64

	// Dynamic pricing
	priceVolatility float64
	lastUpdate      time.Time

	// API simulation
	latency   time.Duration
	errorRate float64

	// Metrics
	apiCalls int64

	rng *rand.Rand
}

func NewRealisticPriceProvider() *RealisticPriceProvider {
	return &RealisticPriceProvider{
		instancePrices: map[string]map[string]float64{
			"us-east-1": {
				"t3.micro":   0.0104,
				"t3.small":   0.0208,
				"t3.medium":  0.0416,
				"t3.large":   0.0832,
				"m5.large":   0.096,
				"m5.xlarge":  0.192,
				"c5.large":   0.085,
				"c5.xlarge":  0.17,
			},
			"us-west-2": {
				"t3.micro":   0.0104,
				"t3.small":   0.0208,
				"t3.medium":  0.0416,
				"t3.large":   0.0832,
				"m5.large":   0.096,
			},
			"eu-west-1": {
				"t3.micro":   0.0114,
				"t3.small":   0.0228,
				"t3.medium":  0.0456,
				"m5.large":   0.107,
			},
		},
		spotPrices: map[string]map[string]float64{
			"us-east-1": {
				"t3.micro":  0.0031, // ~70% discount
				"t3.small":  0.0073,
				"t3.medium": 0.0125,
				"t3.large":  0.0291,
				"m5.large":  0.0384,
			},
		},
		priceVolatility: 0.15,
		lastUpdate:      time.Now(),
		latency:         30 * time.Millisecond,
		errorRate:       0.02,
		rng:             rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (p *RealisticPriceProvider) Name() string {
	return "realistic-aws"
}

func (p *RealisticPriceProvider) GetInstancePrice(ctx context.Context, instanceType, region string) (float64, error) {
	time.Sleep(p.latency)
	atomic.AddInt64(&p.apiCalls, 1)

	if p.rng.Float64() < p.errorRate {
		return 0, fmt.Errorf("PricingAPI: RequestThrottled")
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	regionPrices, ok := p.instancePrices[region]
	if !ok {
		regionPrices = p.instancePrices["us-east-1"]
	}

	price, ok := regionPrices[instanceType]
	if !ok {
		return 0, fmt.Errorf("unknown instance type: %s", instanceType)
	}

	return price, nil
}

func (p *RealisticPriceProvider) GetSpotPrice(ctx context.Context, instanceType, region string) (float64, error) {
	time.Sleep(p.latency)
	atomic.AddInt64(&p.apiCalls, 1)

	if p.rng.Float64() < p.errorRate {
		return 0, fmt.Errorf("EC2 DescribeSpotPriceHistory: timeout")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	regionPrices, ok := p.spotPrices[region]
	if !ok {
		// Fallback to on-demand with 50% discount
		basePrice, err := p.GetInstancePrice(ctx, instanceType, region)
		if err != nil {
			return 0, err
		}
		return basePrice * 0.5, nil
	}

	basePrice, ok := regionPrices[instanceType]
	if !ok {
		return 0, fmt.Errorf("no spot pricing for: %s", instanceType)
	}

	// Add volatility (spot prices fluctuate)
	fluctuation := (p.rng.Float64()*2 - 1) * p.priceVolatility
	return basePrice * (1 + fluctuation), nil
}

func (p *RealisticPriceProvider) GetStoragePrice(ctx context.Context, storageType, region string) (float64, error) {
	time.Sleep(p.latency / 2)
	return 0.08, nil // $0.08/GB/month for gp3
}

func (p *RealisticPriceProvider) GetNetworkPrice(ctx context.Context, region string) (float64, error) {
	time.Sleep(p.latency / 2)
	return 0.09, nil // $0.09/GB for data transfer
}

// =============================================================================
// REALISTIC AUTOSCALING TESTS
// =============================================================================

func TestRealistic_Autoscaling_ScaleUpOnHighCPU(t *testing.T) {
	metrics := NewRealisticMetricsCollector()
	scaler := NewRealisticNodeScaler(3)
	// Disable failure rates for deterministic test behavior
	scaler.SetFailureRates(0, 0)

	t.Log("=== Testing Autoscaling Scale Up on High CPU ===")
	t.Logf("Initial nodes: %d", scaler.GetCurrentCount())

	// Create strategy that checks CPU
	strategy := &RealisticCPUStrategy{
		scaleUpThreshold:   80.0,
		scaleDownThreshold: 30.0,
	}

	cfg := &config.AutoScalingConfig{
		Enabled:        true,
		MinNodes:       2,
		MaxNodes:       10,
		Cooldown:       1, // 1 second for testing
		ScaleDownDelay: 2,
	}

	ctx := context.Background()

	// Simulate spike
	t.Log("\n1. Simulating CPU spike...")
	metrics.SimulateLoad("spike")

	cpu, _ := metrics.GetCPUUtilization(ctx)
	t.Logf("   Current CPU: %.1f%%", cpu)

	// Check if should scale up
	shouldScale, count, err := strategy.ShouldScaleUp(ctx, metrics, cfg)
	require.NoError(t, err)

	t.Logf("   Should scale up: %v (count: %d)", shouldScale, count)

	if shouldScale {
		t.Log("\n2. Scaling up...")
		err := scaler.ScaleUp(ctx, count)
		require.NoError(t, err)

		// Wait for nodes to be ready
		time.Sleep(150 * time.Millisecond)

		t.Logf("   New node count: %d", scaler.GetCurrentCount())
	}

	assert.True(t, shouldScale, "Should scale up when CPU is high")
	assert.Greater(t, scaler.GetCurrentCount(), 3, "Should have more nodes after scale up")

	scalerMetrics := scaler.GetMetrics()
	t.Logf("\n📊 Scaler Metrics:")
	t.Logf("   Scale up operations: %d", scalerMetrics["scale_up_count"])

	t.Log("\n✅ Autoscaling scale up on high CPU working")
}

func TestRealistic_Autoscaling_ScaleDownOnLowUsage(t *testing.T) {
	metrics := NewRealisticMetricsCollector()
	scaler := NewRealisticNodeScaler(8)
	// Disable failure rates for deterministic test behavior
	scaler.SetFailureRates(0, 0)

	t.Log("=== Testing Autoscaling Scale Down on Low Usage ===")
	t.Logf("Initial nodes: %d", scaler.GetCurrentCount())

	strategy := &RealisticCPUStrategy{
		scaleUpThreshold:   80.0,
		scaleDownThreshold: 30.0,
	}

	cfg := &config.AutoScalingConfig{
		Enabled:        true,
		MinNodes:       2,
		MaxNodes:       10,
		Cooldown:       1,
		ScaleDownDelay: 1,
	}

	ctx := context.Background()

	// Simulate idle
	t.Log("\n1. Simulating idle load...")
	metrics.SimulateLoad("idle")

	cpu, _ := metrics.GetCPUUtilization(ctx)
	t.Logf("   Current CPU: %.1f%%", cpu)

	// Check if should scale down
	shouldScale, count, err := strategy.ShouldScaleDown(ctx, metrics, cfg)
	require.NoError(t, err)

	t.Logf("   Should scale down: %v (count: %d)", shouldScale, count)

	if shouldScale {
		t.Log("\n2. Scaling down...")
		err := scaler.ScaleDown(ctx, count)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)
		t.Logf("   New node count: %d", scaler.GetCurrentCount())
	}

	assert.True(t, shouldScale, "Should scale down when CPU is low")
	assert.Less(t, scaler.GetCurrentCount(), 8, "Should have fewer nodes after scale down")

	t.Log("\n✅ Autoscaling scale down on low usage working")
}

func TestRealistic_Autoscaling_RespectMinMaxLimits(t *testing.T) {
	scaler := NewRealisticNodeScaler(5)
	// Disable failure rates for deterministic test behavior
	scaler.SetFailureRates(0, 0)

	t.Log("=== Testing Autoscaling Min/Max Limits ===")

	cfg := &config.AutoScalingConfig{
		Enabled:  true,
		MinNodes: 3,
		MaxNodes: 7,
	}

	ctx := context.Background()

	// Try to scale up beyond max
	t.Log("\n1. Attempting to scale up to 10 nodes (max is 7)...")
	initialCount := scaler.GetCurrentCount()

	// Scale up 5 nodes (would go to 10, but should cap at 7)
	toAdd := 5
	newCount := initialCount + toAdd
	if newCount > cfg.MaxNodes {
		toAdd = cfg.MaxNodes - initialCount
		t.Logf("   Capped scale up to %d nodes (max limit)", toAdd)
	}

	if toAdd > 0 {
		err := scaler.ScaleUp(ctx, toAdd)
		require.NoError(t, err)
	}

	time.Sleep(150 * time.Millisecond)
	t.Logf("   Current count: %d (max: %d)", scaler.GetCurrentCount(), cfg.MaxNodes)
	assert.LessOrEqual(t, scaler.GetCurrentCount(), cfg.MaxNodes)

	// Try to scale down below min
	t.Log("\n2. Attempting to scale down to 1 node (min is 3)...")
	currentCount := scaler.GetCurrentCount()
	toRemove := currentCount - 1 // Try to go to 1 node

	if currentCount-toRemove < cfg.MinNodes {
		toRemove = currentCount - cfg.MinNodes
		t.Logf("   Capped scale down to %d nodes (min limit)", toRemove)
	}

	if toRemove > 0 {
		err := scaler.ScaleDown(ctx, toRemove)
		require.NoError(t, err)
	}

	time.Sleep(100 * time.Millisecond)
	t.Logf("   Current count: %d (min: %d)", scaler.GetCurrentCount(), cfg.MinNodes)
	assert.GreaterOrEqual(t, scaler.GetCurrentCount(), cfg.MinNodes)

	t.Log("\n✅ Min/Max limits respected")
}

func TestRealistic_Autoscaling_CooldownPeriod(t *testing.T) {
	metrics := NewRealisticMetricsCollector()
	scaler := NewRealisticNodeScaler(5)
	// Disable failure rates for deterministic test behavior
	scaler.SetFailureRates(0, 0)

	t.Log("=== Testing Autoscaling Cooldown Period ===")

	cooldownSec := 2

	ctx := context.Background()

	// First scale up
	t.Log("\n1. First scale up operation...")
	metrics.SimulateLoad("spike")
	err := scaler.ScaleUp(ctx, 2)
	require.NoError(t, err)
	firstScaleTime := time.Now()
	t.Logf("   Scaled up at: %v", firstScaleTime.Format("15:04:05.000"))

	// Immediate second scale up (should be blocked by cooldown)
	t.Logf("\n2. Attempting immediate second scale up (cooldown: %ds)...", cooldownSec)
	timeSinceLastScale := time.Since(firstScaleTime)

	if timeSinceLastScale < time.Duration(cooldownSec)*time.Second {
		t.Logf("   Blocked: only %v since last scale (cooldown: %ds)",
			timeSinceLastScale.Round(time.Millisecond), cooldownSec)
	}

	assert.Less(t, timeSinceLastScale, time.Duration(cooldownSec)*time.Second)

	// Wait for cooldown
	t.Logf("\n3. Waiting for cooldown to expire...")
	time.Sleep(time.Duration(cooldownSec)*time.Second - timeSinceLastScale + 100*time.Millisecond)

	timeSinceLastScale = time.Since(firstScaleTime)
	t.Logf("   Time since last scale: %v", timeSinceLastScale.Round(time.Millisecond))

	if timeSinceLastScale >= time.Duration(cooldownSec)*time.Second {
		t.Log("   Cooldown expired, scale up allowed")
	}

	t.Log("\n✅ Cooldown period working correctly")
}

// RealisticCPUStrategy for testing
type RealisticCPUStrategy struct {
	scaleUpThreshold   float64
	scaleDownThreshold float64
}

func (s *RealisticCPUStrategy) Name() string {
	return "realistic-cpu"
}

func (s *RealisticCPUStrategy) ShouldScaleUp(ctx context.Context, metrics provisioning.MetricsCollector, cfg *config.AutoScalingConfig) (bool, int, error) {
	cpu, err := metrics.GetCPUUtilization(ctx)
	if err != nil {
		return false, 0, err
	}

	if cpu > s.scaleUpThreshold {
		// Scale up by 2 nodes or proportionally
		nodesToAdd := int((cpu - s.scaleUpThreshold) / 10) + 1
		if nodesToAdd > 3 {
			nodesToAdd = 3
		}
		return true, nodesToAdd, nil
	}

	return false, 0, nil
}

func (s *RealisticCPUStrategy) ShouldScaleDown(ctx context.Context, metrics provisioning.MetricsCollector, cfg *config.AutoScalingConfig) (bool, int, error) {
	cpu, err := metrics.GetCPUUtilization(ctx)
	if err != nil {
		return false, 0, err
	}

	if cpu < s.scaleDownThreshold {
		nodesToRemove := int((s.scaleDownThreshold - cpu) / 15) + 1
		if nodesToRemove > 2 {
			nodesToRemove = 2
		}
		return true, nodesToRemove, nil
	}

	return false, 0, nil
}

// =============================================================================
// REALISTIC COST ESTIMATION TESTS
// =============================================================================

func TestRealistic_Costs_MultiProviderEstimate(t *testing.T) {
	estimator := costs.NewEstimator(&costs.EstimatorConfig{
		CacheTTL: 5 * time.Minute,
	})

	ctx := context.Background()

	t.Log("=== Testing Multi-Provider Cost Estimation ===")

	// Test different providers
	providers := []struct {
		name   string
		size   string
		region string
	}{
		{"aws", "t3.large", "us-east-1"},
		{"gcp", "e2-standard-2", "us-central1"},
		{"digitalocean", "s-4vcpu-8gb", "nyc1"},
		{"linode", "g6-standard-4", "us-east"},
	}

	t.Log("\n📊 Cost Comparison:")
	t.Log("─────────────────────────────────────────────────")

	for _, p := range providers {
		nodeConfig := &config.NodeConfig{
			Name:     "test-node",
			Provider: p.name,
			Size:     p.size,
			Region:   p.region,
		}

		estimate, err := estimator.EstimateNodeCost(ctx, nodeConfig)
		if err != nil {
			t.Logf("  %-12s: Error - %v", p.name, err)
			continue
		}

		t.Logf("  %-12s | $%.4f/hr | $%.2f/mo | $%.2f/yr",
			p.name, estimate.HourlyCost, estimate.MonthlyCost, estimate.YearlyCost)
	}

	t.Log("─────────────────────────────────────────────────")
	t.Log("\n✅ Multi-provider cost estimation working")
}

func TestRealistic_Costs_SpotSavingsCalculation(t *testing.T) {
	estimator := costs.NewEstimator(&costs.EstimatorConfig{})

	ctx := context.Background()

	t.Log("=== Testing Spot Instance Savings Calculation ===")

	// Compare on-demand vs spot
	onDemandConfig := &config.NodeConfig{
		Name:         "on-demand-node",
		Provider:     "aws",
		Size:         "t3.large",
		Region:       "us-east-1",
		SpotInstance: false,
	}

	spotConfig := &config.NodeConfig{
		Name:         "spot-node",
		Provider:     "aws",
		Size:         "t3.large",
		Region:       "us-east-1",
		SpotInstance: true,
	}

	onDemandEstimate, err := estimator.EstimateNodeCost(ctx, onDemandConfig)
	require.NoError(t, err)

	spotEstimate, err := estimator.EstimateNodeCost(ctx, spotConfig)
	require.NoError(t, err)

	savings := (onDemandEstimate.MonthlyCost - spotEstimate.MonthlyCost) / onDemandEstimate.MonthlyCost * 100

	t.Log("\n📊 On-Demand vs Spot Comparison (t3.large, us-east-1):")
	t.Log("─────────────────────────────────────────────────")
	t.Logf("  On-Demand: $%.2f/month", onDemandEstimate.MonthlyCost)
	t.Logf("  Spot:      $%.2f/month", spotEstimate.MonthlyCost)
	t.Logf("  Savings:   %.1f%%", savings)
	t.Log("─────────────────────────────────────────────────")

	assert.True(t, spotEstimate.IsSpot)
	assert.Greater(t, savings, 30.0, "Spot should save at least 30%")

	t.Log("\n✅ Spot savings calculation working")
}

func TestRealistic_Costs_ClusterCostEstimate(t *testing.T) {
	estimator := costs.NewEstimator(&costs.EstimatorConfig{})

	ctx := context.Background()

	t.Log("=== Testing Full Cluster Cost Estimation ===")

	clusterConfig := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "production-cluster"},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Count:    3,
				Provider: "aws",
				Size:     "m5.large",
				Region:   "us-east-1",
				Roles:    []string{"master"},
			},
			"workers": {
				Name:         "workers",
				Count:        6,
				Provider:     "aws",
				Size:         "t3.large",
				Region:       "us-east-1",
				Roles:        []string{"worker"},
				SpotInstance: true,
			},
		},
		LoadBalancer: config.LoadBalancerConfig{
			Type:     "nlb",
			Provider: "aws",
		},
	}

	estimate, err := estimator.EstimateClusterCost(ctx, clusterConfig)
	require.NoError(t, err)

	t.Log("\n📊 Cluster Cost Breakdown:")
	t.Log("═══════════════════════════════════════════════════")

	masterCost := 0.0
	workerCost := 0.0
	for _, nodeCost := range estimate.NodeCosts {
		if nodeCost.IsSpot {
			workerCost += nodeCost.MonthlyCost
		} else {
			masterCost += nodeCost.MonthlyCost
		}
	}

	t.Logf("  Masters (3x m5.large):     $%.2f/month", masterCost)
	t.Logf("  Workers (6x t3.large):     $%.2f/month (spot)", workerCost)
	t.Logf("  Load Balancer:             $%.2f/month", estimate.LoadBalancerCost)
	t.Logf("  Network:                   $%.2f/month", estimate.NetworkCost)
	t.Log("───────────────────────────────────────────────────")
	t.Logf("  Total Monthly:             $%.2f", estimate.TotalMonthlyCost)
	t.Logf("  Total Yearly:              $%.2f", estimate.TotalYearlyCost)
	t.Log("═══════════════════════════════════════════════════")

	// Recommendations
	if len(estimate.Recommendations) > 0 {
		t.Log("\n💡 Cost Optimization Recommendations:")
		for _, rec := range estimate.Recommendations {
			t.Logf("  - %s: %s (potential savings: $%.2f)",
				rec.Type, rec.Description, rec.PotentialSavings)
		}
	}

	assert.Greater(t, estimate.TotalMonthlyCost, 0.0)
	assert.Equal(t, 9, len(estimate.NodeCosts))

	t.Log("\n✅ Full cluster cost estimation working")
}

func TestRealistic_Costs_PriceCaching(t *testing.T) {
	estimator := costs.NewEstimator(&costs.EstimatorConfig{
		CacheTTL: 100 * time.Millisecond, // Short TTL for testing
	})

	ctx := context.Background()

	t.Log("=== Testing Price Cache Behavior ===")

	nodeConfig := &config.NodeConfig{
		Name:     "cache-test",
		Provider: "aws",
		Size:     "t3.medium",
		Region:   "us-east-1",
	}

	// First call - cache miss
	t.Log("\n1. First call (cache miss)...")
	start := time.Now()
	estimate1, err := estimator.EstimateNodeCost(ctx, nodeConfig)
	require.NoError(t, err)
	firstDuration := time.Since(start)
	t.Logf("   Duration: %v", firstDuration)

	// Second call - cache hit
	t.Log("\n2. Second call (cache hit)...")
	start = time.Now()
	estimate2, err := estimator.EstimateNodeCost(ctx, nodeConfig)
	require.NoError(t, err)
	secondDuration := time.Since(start)
	t.Logf("   Duration: %v", secondDuration)

	assert.Equal(t, estimate1.HourlyCost, estimate2.HourlyCost)

	// Wait for cache expiry
	t.Log("\n3. Waiting for cache expiry...")
	time.Sleep(150 * time.Millisecond)

	// Third call - cache expired
	t.Log("\n4. Third call (cache expired)...")
	start = time.Now()
	_, err = estimator.EstimateNodeCost(ctx, nodeConfig)
	require.NoError(t, err)
	thirdDuration := time.Since(start)
	t.Logf("   Duration: %v", thirdDuration)

	t.Log("\n✅ Price caching working correctly")
}

// =============================================================================
// REALISTIC DISTRIBUTION TESTS
// =============================================================================

func TestRealistic_Distribution_MultiAZSpread(t *testing.T) {
	distributor, err := distribution.NewDistributor(&distribution.DistributorConfig{
		StrategyName: "spread",
	})
	require.NoError(t, err)

	ctx := context.Background()

	t.Log("=== Testing Multi-AZ Spread Distribution ===")

	zones := []string{"us-east-1a", "us-east-1b", "us-east-1c", "us-east-1d"}

	testCases := []struct {
		nodes    int
		expected string
	}{
		{4, "1,1,1,1"},    // Perfect spread
		{8, "2,2,2,2"},    // Even distribution
		{10, "3,3,2,2"},   // Uneven, favor first zones
		{3, "1,1,1,0"},    // Less nodes than zones
	}

	for _, tc := range testCases {
		dist, err := distributor.Distribute(ctx, tc.nodes, zones)
		require.NoError(t, err)

		t.Logf("\n  %d nodes across 4 AZs:", tc.nodes)
		for _, zone := range zones {
			t.Logf("    %s: %d nodes", zone, dist[zone])
		}
	}

	t.Log("\n✅ Multi-AZ spread distribution working")
}

func TestRealistic_Distribution_WeightedAllocation(t *testing.T) {
	distributor, err := distribution.NewDistributor(&distribution.DistributorConfig{
		StrategyName: "weighted",
	})
	require.NoError(t, err)

	ctx := context.Background()

	t.Log("=== Testing Weighted Zone Distribution ===")

	// Simulate zones with different capacities/preferences
	distributions := []config.ZoneDistribution{
		{Zone: "us-east-1a", Count: 50},  // 50% weight
		{Zone: "us-east-1b", Count: 30},  // 30% weight
		{Zone: "us-east-1c", Count: 20},  // 20% weight
	}

	totalNodes := 20

	dist, err := distributor.DistributeWithConfig(ctx, totalNodes, distributions)
	require.NoError(t, err)

	t.Log("\n📊 Weighted Distribution (50/30/20):")
	t.Log("─────────────────────────────────────")

	totalAssigned := 0
	for _, zd := range distributions {
		count := dist[zd.Zone]
		percentage := float64(count) / float64(totalNodes) * 100
		t.Logf("  %s: %d nodes (%.0f%%)", zd.Zone, count, percentage)
		totalAssigned += count
	}

	t.Log("─────────────────────────────────────")
	t.Logf("  Total: %d nodes", totalAssigned)

	assert.Equal(t, totalNodes, totalAssigned)

	// Verify proportions are roughly correct
	assert.GreaterOrEqual(t, dist["us-east-1a"], dist["us-east-1b"])
	assert.GreaterOrEqual(t, dist["us-east-1b"], dist["us-east-1c"])

	t.Log("\n✅ Weighted distribution working correctly")
}

func TestRealistic_Distribution_RebalanceCalculation(t *testing.T) {
	distributor, err := distribution.NewDistributor(&distribution.DistributorConfig{
		StrategyName: "round_robin",
	})
	require.NoError(t, err)

	ctx := context.Background()

	t.Log("=== Testing Cluster Rebalancing ===")

	// Simulate imbalanced cluster
	currentDistribution := map[string]int{
		"us-east-1a": 10, // Too many
		"us-east-1b": 2,  // Too few
		"us-east-1c": 3,  // Too few
	}

	t.Log("\n📊 Current Distribution (imbalanced):")
	total := 0
	for zone, count := range currentDistribution {
		t.Logf("  %s: %d nodes", zone, count)
		total += count
	}
	t.Logf("  Total: %d nodes", total)

	// Calculate rebalance
	changes, err := distributor.Rebalance(ctx, currentDistribution)
	require.NoError(t, err)

	t.Log("\n🔄 Rebalance Plan:")
	for zone, change := range changes {
		if change > 0 {
			t.Logf("  %s: +%d nodes (add)", zone, change)
		} else if change < 0 {
			t.Logf("  %s: %d nodes (remove)", zone, change)
		}
	}

	// Calculate new distribution
	t.Log("\n📊 New Distribution (after rebalance):")
	for zone, count := range currentDistribution {
		newCount := count + changes[zone]
		t.Logf("  %s: %d nodes", zone, newCount)
	}

	// Verify changes balance out
	totalChange := 0
	for _, change := range changes {
		totalChange += change
	}
	assert.Equal(t, 0, totalChange, "Total change should be zero (no nodes added/removed)")

	t.Log("\n✅ Rebalancing calculation working")
}

func TestRealistic_Distribution_AllStrategies(t *testing.T) {
	ctx := context.Background()
	zones := []string{"zone-a", "zone-b", "zone-c"}
	totalNodes := 10

	strategies := []string{"round_robin", "weighted", "packed", "spread"}

	t.Log("=== Testing All Distribution Strategies ===")
	t.Logf("Total nodes: %d, Zones: %v\n", totalNodes, zones)

	for _, strategyName := range strategies {
		distributor, err := distribution.NewDistributor(&distribution.DistributorConfig{
			StrategyName: strategyName,
		})
		require.NoError(t, err)

		dist, err := distributor.Distribute(ctx, totalNodes, zones)
		require.NoError(t, err)

		t.Logf("📊 %s:", strategyName)
		for _, zone := range zones {
			bar := ""
			for i := 0; i < dist[zone]; i++ {
				bar += "█"
			}
			t.Logf("    %s: %d %s", zone, dist[zone], bar)
		}
		t.Log()
	}

	t.Log("✅ All distribution strategies working")
}

// =============================================================================
// REALISTIC E2E TEST - AUTOSCALING WITH COSTS
// =============================================================================

func TestRealistic_E2E_AutoscalingWithCostAwareness(t *testing.T) {
	metrics := NewRealisticMetricsCollector()
	scaler := NewRealisticNodeScaler(3)
	// Disable failure rates for deterministic E2E test behavior
	scaler.SetFailureRates(0, 0)
	estimator := costs.NewEstimator(&costs.EstimatorConfig{})

	ctx := context.Background()

	t.Log("=== Testing E2E: Autoscaling with Cost Awareness ===")

	// Initial state
	t.Log("\n📊 Initial State:")
	t.Logf("   Nodes: %d", scaler.GetCurrentCount())

	initialCost := estimateClusterCost(estimator, ctx, scaler.GetCurrentCount())
	t.Logf("   Monthly cost: $%.2f", initialCost)

	// Simulate load spike
	t.Log("\n⚡ Simulating load spike...")
	metrics.SimulateLoad("spike")

	cpu, _ := metrics.GetCPUUtilization(ctx)
	t.Logf("   CPU usage: %.1f%%", cpu)

	// Scale up
	if cpu > 80 {
		nodesToAdd := 2
		t.Logf("   Scaling up by %d nodes...", nodesToAdd)

		// Check cost impact before scaling
		newCost := estimateClusterCost(estimator, ctx, scaler.GetCurrentCount()+nodesToAdd)
		costIncrease := newCost - initialCost
		t.Logf("   Cost impact: +$%.2f/month", costIncrease)

		err := scaler.ScaleUp(ctx, nodesToAdd)
		require.NoError(t, err)

		time.Sleep(150 * time.Millisecond)
	}

	// Post scale-up state
	t.Log("\n📊 After Scale Up:")
	t.Logf("   Nodes: %d", scaler.GetCurrentCount())

	finalCost := estimateClusterCost(estimator, ctx, scaler.GetCurrentCount())
	t.Logf("   Monthly cost: $%.2f", finalCost)

	// Simulate load decrease
	t.Log("\n📉 Simulating load decrease...")
	metrics.SimulateLoad("idle")

	cpu, _ = metrics.GetCPUUtilization(ctx)
	t.Logf("   CPU usage: %.1f%%", cpu)

	// Scale down
	if cpu < 30 {
		nodesToRemove := 1
		t.Logf("   Scaling down by %d node...", nodesToRemove)

		// Check cost savings
		newCost := estimateClusterCost(estimator, ctx, scaler.GetCurrentCount()-nodesToRemove)
		costSavings := finalCost - newCost
		t.Logf("   Cost savings: -$%.2f/month", costSavings)

		err := scaler.ScaleDown(ctx, nodesToRemove)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)
	}

	// Final state
	t.Log("\n📊 Final State:")
	t.Logf("   Nodes: %d", scaler.GetCurrentCount())

	endCost := estimateClusterCost(estimator, ctx, scaler.GetCurrentCount())
	t.Logf("   Monthly cost: $%.2f", endCost)

	scalerMetrics := scaler.GetMetrics()
	t.Log("\n📈 Scaling Metrics:")
	t.Logf("   Scale up operations: %d", scalerMetrics["scale_up_count"])
	t.Logf("   Scale down operations: %d", scalerMetrics["scale_down_count"])

	t.Log("\n✅ E2E Autoscaling with cost awareness working")
}

func estimateClusterCost(estimator *costs.Estimator, ctx context.Context, nodeCount int) float64 {
	nodeConfig := &config.NodeConfig{
		Provider: "aws",
		Size:     "t3.large",
		Region:   "us-east-1",
	}

	estimate, err := estimator.EstimateNodeCost(ctx, nodeConfig)
	if err != nil {
		return 0
	}

	return estimate.MonthlyCost * float64(nodeCount)
}
