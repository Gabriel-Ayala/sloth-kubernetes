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
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/distribution"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/spot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// REALISTIC AWS SIMULATOR - Simulates real AWS behavior
// =============================================================================

// InstanceState represents EC2 instance lifecycle states
type InstanceState string

const (
	StatePending     InstanceState = "pending"
	StateRunning     InstanceState = "running"
	StateStopping    InstanceState = "stopping"
	StateStopped     InstanceState = "stopped"
	StateTerminating InstanceState = "terminating"
	StateTerminated  InstanceState = "terminated"
)

// AWSSimulatorConfig configures the simulator behavior
type AWSSimulatorConfig struct {
	// Latency simulation
	MinAPILatency    time.Duration
	MaxAPILatency    time.Duration
	InstanceBootTime time.Duration

	// Rate limiting
	MaxRequestsPerSec int
	BurstLimit        int

	// Capacity simulation
	InitialCapacity   map[string]int // zone -> available capacity
	SpotCapacityRatio float64        // % of capacity available for spot

	// Pricing simulation
	BasePrices          map[string]float64 // zone -> base spot price
	PriceVolatility     float64            // 0-1, how much prices fluctuate
	PriceUpdateInterval time.Duration

	// Failure simulation
	APIErrorRate      float64 // 0-1, chance of random API error
	SpotInterruptRate float64 // 0-1, chance of spot interruption per minute
}

// DefaultAWSSimulatorConfig returns realistic AWS defaults
func DefaultAWSSimulatorConfig() *AWSSimulatorConfig {
	return &AWSSimulatorConfig{
		MinAPILatency:     50 * time.Millisecond,
		MaxAPILatency:     200 * time.Millisecond,
		InstanceBootTime:  30 * time.Second,
		MaxRequestsPerSec: 100,
		BurstLimit:        200,
		InitialCapacity: map[string]int{
			"us-east-1a": 50,
			"us-east-1b": 40,
			"us-east-1c": 45,
		},
		SpotCapacityRatio: 0.7, // 70% of capacity available for spot
		BasePrices: map[string]float64{
			"us-east-1a": 0.035,
			"us-east-1b": 0.042,
			"us-east-1c": 0.038,
		},
		PriceVolatility:     0.3,
		PriceUpdateInterval: 5 * time.Minute,
		APIErrorRate:        0.01, // 1% error rate
		SpotInterruptRate:   0.05, // 5% chance per hour
	}
}

// RealisticAWSSimulator simulates AWS EC2 behavior
type RealisticAWSSimulator struct {
	mu               sync.RWMutex
	config           *AWSSimulatorConfig
	instances        map[string]*SimulatedInstance
	spotPrices       map[string]float64
	spotCapacity     map[string]int
	onDemandCapacity map[string]int

	// Rate limiting
	requestCount  int64
	requestWindow time.Time
	rateLimitMu   sync.Mutex

	// Metrics
	totalRequests  int64
	failedRequests int64
	throttledReqs  int64

	// Event channels
	interruptionCh chan *SpotInterruptionEvent
	stateChangeCh  chan *InstanceStateChange

	// Random source (seeded for reproducibility in tests)
	rng *rand.Rand

	// Lifecycle management
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// SimulatedInstance represents an EC2 instance with full lifecycle
type SimulatedInstance struct {
	ID              string
	Name            string
	Zone            string
	InstanceType    string
	State           InstanceState
	IsSpot          bool
	SpotRequestID   string
	SpotPrice       float64
	LaunchTime      time.Time
	StateChangeTime time.Time
	PrivateIP       string
	PublicIP        string
	SubnetID        string
	VPCID           string
	SecurityGroups  []string
	Tags            map[string]string
}

// SpotInterruptionEvent represents a spot interruption notification
type SpotInterruptionEvent struct {
	InstanceID    string
	InterruptTime time.Time
	Action        string        // "terminate", "stop", "hibernate"
	WarningTime   time.Duration // AWS gives 2-minute warning
}

// InstanceStateChange represents an instance state transition
type InstanceStateChange struct {
	InstanceID string
	OldState   InstanceState
	NewState   InstanceState
	Timestamp  time.Time
}

// NewRealisticAWSSimulator creates a new AWS simulator
func NewRealisticAWSSimulator(cfg *AWSSimulatorConfig) *RealisticAWSSimulator {
	if cfg == nil {
		cfg = DefaultAWSSimulatorConfig()
	}

	sim := &RealisticAWSSimulator{
		config:           cfg,
		instances:        make(map[string]*SimulatedInstance),
		spotPrices:       make(map[string]float64),
		spotCapacity:     make(map[string]int),
		onDemandCapacity: make(map[string]int),
		requestWindow:    time.Now(),
		interruptionCh:   make(chan *SpotInterruptionEvent, 100),
		stateChangeCh:    make(chan *InstanceStateChange, 100),
		rng:              rand.New(rand.NewSource(time.Now().UnixNano())),
		stopCh:           make(chan struct{}),
	}

	// Initialize capacity and prices
	for zone, cap := range cfg.InitialCapacity {
		sim.spotCapacity[zone] = int(float64(cap) * cfg.SpotCapacityRatio)
		sim.onDemandCapacity[zone] = cap
		sim.spotPrices[zone] = cfg.BasePrices[zone]
	}

	return sim
}

// Start begins background processes (price fluctuation, interruptions)
func (s *RealisticAWSSimulator) Start() {
	s.wg.Add(2)
	go s.priceFluctuationLoop()
	go s.spotInterruptionLoop()
}

// Stop halts all background processes
func (s *RealisticAWSSimulator) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

// priceFluctuationLoop simulates spot price changes
func (s *RealisticAWSSimulator) priceFluctuationLoop() {
	defer s.wg.Done()

	// Use shorter interval for tests
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.fluctuatePrices()
		}
	}
}

// fluctuatePrices updates spot prices with random fluctuation
func (s *RealisticAWSSimulator) fluctuatePrices() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for zone, basePrice := range s.config.BasePrices {
		// Random fluctuation within volatility range
		fluctuation := (s.rng.Float64()*2 - 1) * s.config.PriceVolatility
		newPrice := basePrice * (1 + fluctuation)

		// Ensure price doesn't go below minimum
		if newPrice < basePrice*0.5 {
			newPrice = basePrice * 0.5
		}
		// Or above maximum
		if newPrice > basePrice*2.0 {
			newPrice = basePrice * 2.0
		}

		s.spotPrices[zone] = newPrice
	}
}

// spotInterruptionLoop randomly interrupts spot instances
func (s *RealisticAWSSimulator) spotInterruptionLoop() {
	defer s.wg.Done()

	// Check every 500ms for test speed
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkForInterruptions()
		}
	}
}

// checkForInterruptions randomly selects spot instances to interrupt
func (s *RealisticAWSSimulator) checkForInterruptions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, inst := range s.instances {
		if inst.IsSpot && inst.State == StateRunning {
			// Adjusted rate for test timing (per check, not per minute)
			if s.rng.Float64() < s.config.SpotInterruptRate/120 {
				// Send 2-minute warning
				event := &SpotInterruptionEvent{
					InstanceID:    id,
					InterruptTime: time.Now().Add(2 * time.Minute),
					Action:        "terminate",
					WarningTime:   2 * time.Minute,
				}

				select {
				case s.interruptionCh <- event:
				default:
					// Channel full, skip
				}
			}
		}
	}
}

// simulateLatency adds realistic API latency
func (s *RealisticAWSSimulator) simulateLatency() {
	latencyRange := s.config.MaxAPILatency - s.config.MinAPILatency
	latency := s.config.MinAPILatency + time.Duration(s.rng.Int63n(int64(latencyRange)))
	time.Sleep(latency)
}

// checkRateLimit enforces API rate limiting
func (s *RealisticAWSSimulator) checkRateLimit() error {
	s.rateLimitMu.Lock()
	defer s.rateLimitMu.Unlock()

	now := time.Now()

	// Reset window if needed
	if now.Sub(s.requestWindow) >= time.Second {
		s.requestWindow = now
		s.requestCount = 0
	}

	s.requestCount++
	atomic.AddInt64(&s.totalRequests, 1)

	if s.requestCount > int64(s.config.MaxRequestsPerSec) {
		atomic.AddInt64(&s.throttledReqs, 1)
		return fmt.Errorf("RequestLimitExceeded: Rate exceeded")
	}

	return nil
}

// maybeInjectError randomly returns API errors
func (s *RealisticAWSSimulator) maybeInjectError() error {
	if s.rng.Float64() < s.config.APIErrorRate {
		atomic.AddInt64(&s.failedRequests, 1)
		errorMsgs := []string{
			"InternalError: An internal error has occurred",
			"ServiceUnavailable: Service is temporarily unavailable",
			"RequestTimeout: The request timed out",
		}
		return fmt.Errorf("%s", errorMsgs[s.rng.Intn(len(errorMsgs))])
	}
	return nil
}

// RunInstances creates new EC2 instances (simulates ec2:RunInstances)
func (s *RealisticAWSSimulator) RunInstances(ctx context.Context, req *RunInstancesRequest) (*RunInstancesResponse, error) {
	// Rate limiting
	if err := s.checkRateLimit(); err != nil {
		return nil, err
	}

	// Simulate latency
	s.simulateLatency()

	// Random API error
	if err := s.maybeInjectError(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check capacity
	capacityMap := s.onDemandCapacity
	if req.IsSpot {
		capacityMap = s.spotCapacity
	}

	available := capacityMap[req.Zone]
	if available < req.Count {
		return nil, fmt.Errorf("InsufficientInstanceCapacity: There is no Spot capacity available that matches your request")
	}

	// Check spot price
	if req.IsSpot && req.MaxSpotPrice > 0 {
		currentPrice := s.spotPrices[req.Zone]
		if currentPrice > req.MaxSpotPrice {
			return nil, fmt.Errorf("SpotMaxPriceTooLow: Your Spot request price of %.4f is lower than the current Spot price of %.4f",
				req.MaxSpotPrice, currentPrice)
		}
	}

	// Create instances
	response := &RunInstancesResponse{
		Instances: make([]*SimulatedInstance, 0, req.Count),
	}

	for i := 0; i < req.Count; i++ {
		instanceID := fmt.Sprintf("i-%s-%d", randomString(8), time.Now().UnixNano())

		inst := &SimulatedInstance{
			ID:              instanceID,
			Name:            req.Name,
			Zone:            req.Zone,
			InstanceType:    req.InstanceType,
			State:           StatePending, // Starts in pending state
			IsSpot:          req.IsSpot,
			SpotPrice:       s.spotPrices[req.Zone],
			LaunchTime:      time.Now(),
			StateChangeTime: time.Now(),
			PrivateIP:       fmt.Sprintf("10.0.%d.%d", s.rng.Intn(256), s.rng.Intn(256)),
			PublicIP:        fmt.Sprintf("54.%d.%d.%d", s.rng.Intn(256), s.rng.Intn(256), s.rng.Intn(256)),
			SubnetID:        fmt.Sprintf("subnet-%s", randomString(8)),
			VPCID:           req.VPCID,
			SecurityGroups:  req.SecurityGroups,
			Tags:            req.Tags,
		}

		s.instances[instanceID] = inst
		capacityMap[req.Zone]--

		response.Instances = append(response.Instances, inst)

		// Schedule state transition to running
		go s.scheduleStateTransition(instanceID, StateRunning, s.config.InstanceBootTime)
	}

	return response, nil
}

// scheduleStateTransition transitions instance state after delay
func (s *RealisticAWSSimulator) scheduleStateTransition(instanceID string, newState InstanceState, delay time.Duration) {
	// Use shorter delay for tests
	testDelay := delay / 100 // 30s becomes 300ms
	if testDelay < 10*time.Millisecond {
		testDelay = 10 * time.Millisecond
	}

	time.Sleep(testDelay)

	s.mu.Lock()
	defer s.mu.Unlock()

	inst, ok := s.instances[instanceID]
	if !ok {
		return
	}

	oldState := inst.State
	inst.State = newState
	inst.StateChangeTime = time.Now()

	// Send state change event
	select {
	case s.stateChangeCh <- &InstanceStateChange{
		InstanceID: instanceID,
		OldState:   oldState,
		NewState:   newState,
		Timestamp:  time.Now(),
	}:
	default:
	}
}

// DescribeInstances returns instance information (simulates ec2:DescribeInstances)
func (s *RealisticAWSSimulator) DescribeInstances(ctx context.Context, instanceIDs []string) ([]*SimulatedInstance, error) {
	if err := s.checkRateLimit(); err != nil {
		return nil, err
	}

	s.simulateLatency()

	if err := s.maybeInjectError(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*SimulatedInstance, 0)

	if len(instanceIDs) == 0 {
		// Return all instances
		for _, inst := range s.instances {
			result = append(result, inst)
		}
	} else {
		for _, id := range instanceIDs {
			if inst, ok := s.instances[id]; ok {
				result = append(result, inst)
			}
		}
	}

	return result, nil
}

// TerminateInstances terminates instances (simulates ec2:TerminateInstances)
func (s *RealisticAWSSimulator) TerminateInstances(ctx context.Context, instanceIDs []string) error {
	if err := s.checkRateLimit(); err != nil {
		return err
	}

	s.simulateLatency()

	if err := s.maybeInjectError(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range instanceIDs {
		if inst, ok := s.instances[id]; ok {
			inst.State = StateTerminating
			inst.StateChangeTime = time.Now()

			// Return capacity
			if inst.IsSpot {
				s.spotCapacity[inst.Zone]++
			} else {
				s.onDemandCapacity[inst.Zone]++
			}

			// Schedule final termination
			go func(instanceID string) {
				time.Sleep(50 * time.Millisecond)
				s.mu.Lock()
				if inst, ok := s.instances[instanceID]; ok {
					inst.State = StateTerminated
				}
				s.mu.Unlock()
			}(id)
		}
	}

	return nil
}

// GetSpotPrice returns current spot price for a zone
func (s *RealisticAWSSimulator) GetSpotPrice(zone string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.spotPrices[zone]
}

// GetMetrics returns simulator metrics
func (s *RealisticAWSSimulator) GetMetrics() SimulatorMetrics {
	return SimulatorMetrics{
		TotalRequests:    atomic.LoadInt64(&s.totalRequests),
		FailedRequests:   atomic.LoadInt64(&s.failedRequests),
		ThrottledReqs:    atomic.LoadInt64(&s.throttledReqs),
		RunningInstances: s.countInstancesByState(StateRunning),
		PendingInstances: s.countInstancesByState(StatePending),
	}
}

func (s *RealisticAWSSimulator) countInstancesByState(state InstanceState) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, inst := range s.instances {
		if inst.State == state {
			count++
		}
	}
	return count
}

// Request/Response types
type RunInstancesRequest struct {
	Name           string
	Zone           string
	InstanceType   string
	Count          int
	IsSpot         bool
	MaxSpotPrice   float64
	VPCID          string
	SecurityGroups []string
	Tags           map[string]string
}

type RunInstancesResponse struct {
	Instances []*SimulatedInstance
}

type SimulatorMetrics struct {
	TotalRequests    int64
	FailedRequests   int64
	ThrottledReqs    int64
	RunningInstances int
	PendingInstances int
}

// Helper functions
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// =============================================================================
// REALISTIC SPOT ADAPTER - Connects AWS Simulator to spot.Manager
// =============================================================================

type RealisticSpotAdapter struct {
	simulator *RealisticAWSSimulator
}

func NewRealisticSpotAdapter(sim *RealisticAWSSimulator) *RealisticSpotAdapter {
	return &RealisticSpotAdapter{simulator: sim}
}

func (a *RealisticSpotAdapter) CreateSpotInstance(ctx context.Context, nodeConfig *config.NodeConfig, spotConfig *config.SpotConfig) (*providers.NodeOutput, error) {
	resp, err := a.simulator.RunInstances(ctx, &RunInstancesRequest{
		Name:         nodeConfig.Name,
		Zone:         nodeConfig.Zone,
		InstanceType: nodeConfig.Size,
		Count:        1,
		IsSpot:       true,
		MaxSpotPrice: spotConfig.MaxSpotPrice,
		Tags: map[string]string{
			"Name": nodeConfig.Name,
			"Role": "worker",
		},
	})

	if err != nil {
		return nil, err
	}

	if len(resp.Instances) == 0 {
		return nil, fmt.Errorf("no instances created")
	}

	inst := resp.Instances[0]
	return &providers.NodeOutput{
		Name:     inst.Name,
		Provider: nodeConfig.Provider,
		Region:   nodeConfig.Region,
	}, nil
}

func (a *RealisticSpotAdapter) GetSpotPrice(ctx context.Context, instanceType, zone string) (float64, error) {
	return a.simulator.GetSpotPrice(zone), nil
}

func (a *RealisticSpotAdapter) IsSpotAvailable(ctx context.Context, instanceType, zone string) (bool, error) {
	a.simulator.mu.RLock()
	defer a.simulator.mu.RUnlock()
	return a.simulator.spotCapacity[zone] > 0, nil
}

func (a *RealisticSpotAdapter) HandleInterruption(ctx context.Context, nodeID string) error {
	return a.simulator.TerminateInstances(ctx, []string{nodeID})
}

// =============================================================================
// REALISTIC INTEGRATION TESTS
// =============================================================================

// TestRealistic_InstanceLifecycle tests the full instance lifecycle
func TestRealistic_InstanceLifecycle(t *testing.T) {
	sim := NewRealisticAWSSimulator(&AWSSimulatorConfig{
		MinAPILatency:     5 * time.Millisecond,
		MaxAPILatency:     20 * time.Millisecond,
		InstanceBootTime:  100 * time.Millisecond, // Fast for tests
		MaxRequestsPerSec: 1000,
		InitialCapacity: map[string]int{
			"us-east-1a": 10,
		},
		SpotCapacityRatio: 0.7,
		BasePrices: map[string]float64{
			"us-east-1a": 0.035,
		},
		APIErrorRate: 0, // No random errors for this test
	})

	ctx := context.Background()

	t.Log("=== Testing Realistic Instance Lifecycle ===")

	// Create instance
	t.Log("\n1. Creating instance...")
	resp, err := sim.RunInstances(ctx, &RunInstancesRequest{
		Name:         "test-instance",
		Zone:         "us-east-1a",
		InstanceType: "t3.medium",
		Count:        1,
		IsSpot:       false,
	})
	require.NoError(t, err)
	require.Len(t, resp.Instances, 1)

	inst := resp.Instances[0]
	t.Logf("   Created: %s (state: %s)", inst.ID, inst.State)
	assert.Equal(t, StatePending, inst.State, "Instance should start in pending state")

	// Wait for instance to become running
	t.Log("\n2. Waiting for instance to become running...")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		instances, err := sim.DescribeInstances(ctx, []string{inst.ID})
		require.NoError(t, err)

		if len(instances) > 0 && instances[0].State == StateRunning {
			t.Logf("   Instance is now running (took %v)", time.Since(inst.LaunchTime))
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Verify running state
	instances, err := sim.DescribeInstances(ctx, []string{inst.ID})
	require.NoError(t, err)
	assert.Equal(t, StateRunning, instances[0].State)

	// Terminate instance
	t.Log("\n3. Terminating instance...")
	err = sim.TerminateInstances(ctx, []string{inst.ID})
	require.NoError(t, err)

	// Verify termination
	time.Sleep(100 * time.Millisecond)
	instances, err = sim.DescribeInstances(ctx, []string{inst.ID})
	require.NoError(t, err)
	assert.Equal(t, StateTerminated, instances[0].State)
	t.Logf("   Instance terminated successfully")

	t.Log("\n✅ Instance lifecycle test passed")
}

// TestRealistic_SpotPriceFluctuation tests spot price changes
func TestRealistic_SpotPriceFluctuation(t *testing.T) {
	sim := NewRealisticAWSSimulator(&AWSSimulatorConfig{
		MinAPILatency: 1 * time.Millisecond,
		MaxAPILatency: 5 * time.Millisecond,
		BasePrices: map[string]float64{
			"us-east-1a": 0.050,
		},
		PriceVolatility: 0.5, // High volatility for testing
		InitialCapacity: map[string]int{
			"us-east-1a": 10,
		},
		SpotCapacityRatio: 1.0,
	})

	sim.Start()
	defer sim.Stop()

	t.Log("=== Testing Spot Price Fluctuation ===")
	t.Log("Base price: $0.050/hr, Volatility: 50%")

	// Collect price samples
	prices := make([]float64, 0)
	for i := 0; i < 20; i++ {
		price := sim.GetSpotPrice("us-east-1a")
		prices = append(prices, price)
		time.Sleep(50 * time.Millisecond)
	}

	// Analyze price variation
	minPrice, maxPrice := prices[0], prices[0]
	for _, p := range prices {
		if p < minPrice {
			minPrice = p
		}
		if p > maxPrice {
			maxPrice = p
		}
	}

	variation := (maxPrice - minPrice) / prices[0] * 100

	t.Logf("\nPrice samples collected: %d", len(prices))
	t.Logf("Min price: $%.4f", minPrice)
	t.Logf("Max price: $%.4f", maxPrice)
	t.Logf("Variation: %.1f%%", variation)

	// With 50% volatility, we expect some variation
	assert.True(t, variation > 0, "Prices should fluctuate")
	t.Log("\n✅ Spot price fluctuation working correctly")
}

// TestRealistic_RateLimiting tests API rate limiting
func TestRealistic_RateLimiting(t *testing.T) {
	sim := NewRealisticAWSSimulator(&AWSSimulatorConfig{
		MinAPILatency:     1 * time.Millisecond,
		MaxAPILatency:     2 * time.Millisecond,
		MaxRequestsPerSec: 10, // Low limit for testing
		BurstLimit:        15,
		InitialCapacity: map[string]int{
			"us-east-1a": 100,
		},
		SpotCapacityRatio: 1.0,
		BasePrices: map[string]float64{
			"us-east-1a": 0.05,
		},
		APIErrorRate: 0,
	})

	ctx := context.Background()

	t.Log("=== Testing API Rate Limiting ===")
	t.Logf("Rate limit: %d requests/second", sim.config.MaxRequestsPerSec)

	// Fire many requests quickly
	successCount := 0
	throttledCount := 0

	for i := 0; i < 50; i++ {
		_, err := sim.DescribeInstances(ctx, nil)
		if err != nil {
			if err.Error() == "RequestLimitExceeded: Rate exceeded" {
				throttledCount++
			}
		} else {
			successCount++
		}
	}

	t.Logf("\nResults:")
	t.Logf("  Successful requests: %d", successCount)
	t.Logf("  Throttled requests: %d", throttledCount)

	// Should have some throttled requests
	assert.True(t, throttledCount > 0, "Some requests should be throttled")
	assert.True(t, successCount > 0, "Some requests should succeed")

	metrics := sim.GetMetrics()
	t.Logf("\nSimulator metrics:")
	t.Logf("  Total requests: %d", metrics.TotalRequests)
	t.Logf("  Throttled: %d", metrics.ThrottledReqs)

	t.Log("\n✅ Rate limiting working correctly")
}

// TestRealistic_CapacityExhaustion tests behavior when capacity is exhausted
func TestRealistic_CapacityExhaustion(t *testing.T) {
	sim := NewRealisticAWSSimulator(&AWSSimulatorConfig{
		MinAPILatency:     1 * time.Millisecond,
		MaxAPILatency:     5 * time.Millisecond,
		InstanceBootTime:  10 * time.Millisecond,
		MaxRequestsPerSec: 1000,
		InitialCapacity: map[string]int{
			"us-east-1a": 3, // Only 3 instances available
		},
		SpotCapacityRatio: 1.0,
		BasePrices: map[string]float64{
			"us-east-1a": 0.05,
		},
		APIErrorRate: 0,
	})

	ctx := context.Background()

	t.Log("=== Testing Capacity Exhaustion ===")
	t.Log("Available capacity: 3 spot instances")

	// Create instances until capacity exhausted
	createdCount := 0
	for i := 0; i < 5; i++ {
		resp, err := sim.RunInstances(ctx, &RunInstancesRequest{
			Name:         fmt.Sprintf("worker-%d", i),
			Zone:         "us-east-1a",
			InstanceType: "t3.medium",
			Count:        1,
			IsSpot:       true,
			MaxSpotPrice: 1.0,
		})

		if err != nil {
			t.Logf("  Instance %d: Failed - %v", i, err)
			assert.Contains(t, err.Error(), "InsufficientInstanceCapacity")
		} else {
			t.Logf("  Instance %d: Created (%s)", i, resp.Instances[0].ID)
			createdCount++
		}
	}

	assert.Equal(t, 3, createdCount, "Should only create 3 instances before exhaustion")

	t.Log("\n✅ Capacity exhaustion handled correctly")
}

// TestRealistic_SpotPriceRejection tests max price enforcement
func TestRealistic_SpotPriceRejection(t *testing.T) {
	sim := NewRealisticAWSSimulator(&AWSSimulatorConfig{
		MinAPILatency:     1 * time.Millisecond,
		MaxAPILatency:     5 * time.Millisecond,
		MaxRequestsPerSec: 1000,
		InitialCapacity: map[string]int{
			"us-east-1a": 10,
		},
		SpotCapacityRatio: 1.0,
		BasePrices: map[string]float64{
			"us-east-1a": 0.10, // $0.10/hr
		},
		PriceVolatility: 0, // No fluctuation for predictable test
		APIErrorRate:    0,
	})

	ctx := context.Background()

	t.Log("=== Testing Spot Price Rejection ===")
	t.Log("Current spot price: $0.10/hr")

	// Try with max price below current
	t.Log("\n1. Attempting with max price $0.05 (too low)...")
	_, err := sim.RunInstances(ctx, &RunInstancesRequest{
		Name:         "worker-1",
		Zone:         "us-east-1a",
		InstanceType: "t3.medium",
		Count:        1,
		IsSpot:       true,
		MaxSpotPrice: 0.05, // Below current price
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SpotMaxPriceTooLow")
	t.Logf("   Rejected: %v", err)

	// Try with max price above current
	t.Log("\n2. Attempting with max price $0.15 (acceptable)...")
	resp, err := sim.RunInstances(ctx, &RunInstancesRequest{
		Name:         "worker-2",
		Zone:         "us-east-1a",
		InstanceType: "t3.medium",
		Count:        1,
		IsSpot:       true,
		MaxSpotPrice: 0.15, // Above current price
	})

	assert.NoError(t, err)
	assert.Len(t, resp.Instances, 1)
	t.Logf("   Created: %s at price $%.4f", resp.Instances[0].ID, resp.Instances[0].SpotPrice)

	t.Log("\n✅ Spot price enforcement working correctly")
}

// TestRealistic_ConcurrentProvisioning tests parallel instance creation
func TestRealistic_ConcurrentProvisioning(t *testing.T) {
	sim := NewRealisticAWSSimulator(&AWSSimulatorConfig{
		MinAPILatency:     10 * time.Millisecond,
		MaxAPILatency:     50 * time.Millisecond,
		InstanceBootTime:  100 * time.Millisecond,
		MaxRequestsPerSec: 100,
		InitialCapacity: map[string]int{
			"us-east-1a": 20,
			"us-east-1b": 20,
			"us-east-1c": 20,
		},
		SpotCapacityRatio: 1.0,
		BasePrices: map[string]float64{
			"us-east-1a": 0.03,
			"us-east-1b": 0.04,
			"us-east-1c": 0.035,
		},
		APIErrorRate: 0,
	})

	ctx := context.Background()
	zones := []string{"us-east-1a", "us-east-1b", "us-east-1c"}

	t.Log("=== Testing Concurrent Provisioning ===")
	t.Log("Creating 15 instances across 3 zones concurrently")

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]*SimulatedInstance, 0)
	errors := make([]error, 0)

	startTime := time.Now()

	// Launch 15 concurrent instance creations
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			zone := zones[idx%len(zones)]
			resp, err := sim.RunInstances(ctx, &RunInstancesRequest{
				Name:         fmt.Sprintf("worker-%d", idx),
				Zone:         zone,
				InstanceType: "t3.medium",
				Count:        1,
				IsSpot:       true,
				MaxSpotPrice: 0.10,
			})

			mu.Lock()
			if err != nil {
				errors = append(errors, err)
			} else {
				results = append(results, resp.Instances[0])
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	t.Logf("\nResults:")
	t.Logf("  Duration: %v", duration)
	t.Logf("  Successful: %d", len(results))
	t.Logf("  Failed: %d", len(errors))

	// Count by zone
	zoneCounts := make(map[string]int)
	for _, inst := range results {
		zoneCounts[inst.Zone]++
	}
	t.Logf("  By zone: %v", zoneCounts)

	// Verify all succeeded
	assert.Len(t, results, 15, "All 15 instances should be created")
	assert.Len(t, errors, 0, "No errors expected")

	// Verify zone distribution
	for zone, count := range zoneCounts {
		assert.Equal(t, 5, count, "Zone %s should have 5 instances", zone)
	}

	// Wait for all to become running
	t.Log("\nWaiting for instances to become running...")
	time.Sleep(200 * time.Millisecond)

	metrics := sim.GetMetrics()
	t.Logf("\nFinal metrics:")
	t.Logf("  Running instances: %d", metrics.RunningInstances)
	t.Logf("  Total API requests: %d", metrics.TotalRequests)

	t.Log("\n✅ Concurrent provisioning working correctly")
}

// TestRealistic_FullClusterWithDistribution tests realistic cluster deployment
func TestRealistic_FullClusterWithDistribution(t *testing.T) {
	sim := NewRealisticAWSSimulator(&AWSSimulatorConfig{
		MinAPILatency:     5 * time.Millisecond,
		MaxAPILatency:     20 * time.Millisecond,
		InstanceBootTime:  50 * time.Millisecond,
		MaxRequestsPerSec: 100,
		InitialCapacity: map[string]int{
			"us-east-1a": 20,
			"us-east-1b": 20,
			"us-east-1c": 20,
		},
		SpotCapacityRatio: 0.8,
		BasePrices: map[string]float64{
			"us-east-1a": 0.030,
			"us-east-1b": 0.045,
			"us-east-1c": 0.038,
		},
		PriceVolatility: 0.1,
		APIErrorRate:    0,
	})

	sim.Start()
	defer sim.Stop()

	ctx := context.Background()
	zones := []string{"us-east-1a", "us-east-1b", "us-east-1c"}

	t.Log("=== Testing Realistic Full Cluster Deployment ===")
	t.Log("Target: 3 masters (on-demand) + 9 workers (spot)")

	// Setup distribution
	distributor, err := distribution.NewDistributor(&distribution.DistributorConfig{
		StrategyName: "round_robin",
	})
	require.NoError(t, err)

	// Setup spot manager
	spotManager := spot.NewManager(&spot.ManagerConfig{
		Strategy: &spot.DefaultStrategy{},
	})
	spotManager.RegisterProvider("aws", NewRealisticSpotAdapter(sim))

	// Step 1: Deploy masters (on-demand)
	t.Log("\n📋 Step 1: Deploying master nodes (on-demand)...")
	masterDist, err := distributor.Distribute(ctx, 3, zones)
	require.NoError(t, err)

	masters := make([]*SimulatedInstance, 0)
	for zone, count := range masterDist {
		for i := 0; i < count; i++ {
			resp, err := sim.RunInstances(ctx, &RunInstancesRequest{
				Name:         fmt.Sprintf("master-%s-%d", zone, i),
				Zone:         zone,
				InstanceType: "m5.large",
				Count:        1,
				IsSpot:       false, // On-demand for masters
			})
			require.NoError(t, err)
			masters = append(masters, resp.Instances[0])
			t.Logf("   Created %s in %s", resp.Instances[0].Name, zone)
		}
	}

	// Step 2: Deploy workers (spot)
	t.Log("\n⚡ Step 2: Deploying worker nodes (spot)...")
	workerDist, err := distributor.Distribute(ctx, 9, zones)
	require.NoError(t, err)

	workers := make([]*providers.NodeOutput, 0)
	for zone, count := range workerDist {
		currentPrice := sim.GetSpotPrice(zone)
		for i := 0; i < count; i++ {
			output, err := spotManager.CreateSpotInstance(ctx, &config.NodeConfig{
				Name:     fmt.Sprintf("worker-%s-%d", zone, i),
				Provider: "aws",
				Region:   "us-east-1",
				Zone:     zone,
				Size:     "t3.large",
			}, &config.SpotConfig{
				Enabled:      true,
				MaxSpotPrice: 0.10,
			})
			require.NoError(t, err)
			workers = append(workers, output)
			t.Logf("   Created %s in %s (spot @ $%.4f)", output.Name, zone, currentPrice)
		}
	}

	// Step 3: Wait for all instances to be running
	t.Log("\n⏳ Step 3: Waiting for instances to become running...")
	time.Sleep(200 * time.Millisecond)

	// Step 4: Verify cluster state
	t.Log("\n📊 Step 4: Verifying cluster state...")
	allInstances, err := sim.DescribeInstances(ctx, nil)
	require.NoError(t, err)

	runningCount := 0
	spotCount := 0
	onDemandCount := 0
	totalSpotCost := 0.0

	for _, inst := range allInstances {
		if inst.State == StateRunning {
			runningCount++
		}
		if inst.IsSpot {
			spotCount++
			totalSpotCost += inst.SpotPrice
		} else {
			onDemandCount++
		}
	}

	metrics := sim.GetMetrics()

	t.Log("\n=== Final Cluster State ===")
	t.Logf("  Total instances: %d", len(allInstances))
	t.Logf("  Running: %d", runningCount)
	t.Logf("  Masters (on-demand): %d", onDemandCount)
	t.Logf("  Workers (spot): %d", spotCount)
	t.Logf("  Hourly spot cost: $%.4f", totalSpotCost)
	t.Logf("  API requests made: %d", metrics.TotalRequests)

	// Verify counts
	assert.Equal(t, 12, len(allInstances), "Should have 12 total instances")
	assert.Equal(t, 3, onDemandCount, "Should have 3 on-demand masters")
	assert.Equal(t, 9, spotCount, "Should have 9 spot workers")

	// Calculate savings
	onDemandPrice := 0.10
	spotSavings := (onDemandPrice*9 - totalSpotCost) / (onDemandPrice * 9) * 100
	t.Logf("  Spot savings: %.1f%%", spotSavings)

	t.Log("\n✅ Full cluster deployment completed successfully!")
}

// TestRealistic_SpotInterruptionRecovery tests handling spot interruptions
func TestRealistic_SpotInterruptionRecovery(t *testing.T) {
	sim := NewRealisticAWSSimulator(&AWSSimulatorConfig{
		MinAPILatency:     5 * time.Millisecond,
		MaxAPILatency:     10 * time.Millisecond,
		InstanceBootTime:  50 * time.Millisecond,
		MaxRequestsPerSec: 100,
		InitialCapacity: map[string]int{
			"us-east-1a": 10,
			"us-east-1b": 10,
		},
		SpotCapacityRatio: 1.0,
		BasePrices:        map[string]float64{"us-east-1a": 0.03, "us-east-1b": 0.04},
		SpotInterruptRate: 0, // Manual interruption for this test
		APIErrorRate:      0,
	})

	ctx := context.Background()

	t.Log("=== Testing Spot Interruption Recovery ===")

	// Create initial spot instance
	t.Log("\n1. Creating initial spot instance...")
	resp, err := sim.RunInstances(ctx, &RunInstancesRequest{
		Name:         "worker-1",
		Zone:         "us-east-1a",
		InstanceType: "t3.medium",
		Count:        1,
		IsSpot:       true,
		MaxSpotPrice: 0.10,
	})
	require.NoError(t, err)
	originalID := resp.Instances[0].ID
	t.Logf("   Created: %s", originalID)

	// Wait for running
	time.Sleep(100 * time.Millisecond)

	// Simulate interruption
	t.Log("\n2. Simulating spot interruption...")
	err = sim.TerminateInstances(ctx, []string{originalID})
	require.NoError(t, err)
	t.Logf("   Instance %s terminated", originalID)

	// Verify termination
	time.Sleep(100 * time.Millisecond)
	instances, err := sim.DescribeInstances(ctx, []string{originalID})
	require.NoError(t, err)
	assert.Equal(t, StateTerminated, instances[0].State)

	// Create replacement in different zone (diversification)
	t.Log("\n3. Creating replacement instance in different zone...")
	resp, err = sim.RunInstances(ctx, &RunInstancesRequest{
		Name:         "worker-1-replacement",
		Zone:         "us-east-1b", // Different zone
		InstanceType: "t3.medium",
		Count:        1,
		IsSpot:       true,
		MaxSpotPrice: 0.10,
	})
	require.NoError(t, err)
	replacementID := resp.Instances[0].ID
	t.Logf("   Created replacement: %s in us-east-1b", replacementID)

	// Wait for running
	time.Sleep(100 * time.Millisecond)

	instances, err = sim.DescribeInstances(ctx, []string{replacementID})
	require.NoError(t, err)
	assert.Equal(t, StateRunning, instances[0].State)

	t.Log("\n✅ Spot interruption recovery successful")
}

// TestRealistic_APIErrorHandling tests recovery from API errors
func TestRealistic_APIErrorHandling(t *testing.T) {
	sim := NewRealisticAWSSimulator(&AWSSimulatorConfig{
		MinAPILatency:     1 * time.Millisecond,
		MaxAPILatency:     5 * time.Millisecond,
		MaxRequestsPerSec: 1000,
		InitialCapacity: map[string]int{
			"us-east-1a": 10,
		},
		SpotCapacityRatio: 1.0,
		BasePrices:        map[string]float64{"us-east-1a": 0.05},
		APIErrorRate:      0.3, // 30% error rate for testing
	})

	ctx := context.Background()

	t.Log("=== Testing API Error Handling (30% error rate) ===")

	successCount := 0
	errorCount := 0
	maxRetries := 5

	// Try to create instance with retries
	for attempt := 1; attempt <= 10; attempt++ {
		t.Logf("\nAttempt %d:", attempt)

		var lastErr error
		for retry := 0; retry < maxRetries; retry++ {
			_, err := sim.RunInstances(ctx, &RunInstancesRequest{
				Name:         fmt.Sprintf("worker-%d", attempt),
				Zone:         "us-east-1a",
				InstanceType: "t3.medium",
				Count:        1,
				IsSpot:       false,
				MaxSpotPrice: 0.10,
			})

			if err == nil {
				t.Logf("  Success on retry %d", retry)
				successCount++
				lastErr = nil
				break
			}

			lastErr = err
			t.Logf("  Retry %d failed: %v", retry, err)
			time.Sleep(10 * time.Millisecond) // Backoff
		}

		if lastErr != nil {
			t.Logf("  Failed after %d retries", maxRetries)
			errorCount++
		}
	}

	t.Logf("\n📊 Results:")
	t.Logf("  Successful operations: %d", successCount)
	t.Logf("  Failed operations: %d", errorCount)

	metrics := sim.GetMetrics()
	t.Logf("  Total API requests: %d", metrics.TotalRequests)
	t.Logf("  Failed requests: %d", metrics.FailedRequests)

	// With retries, most should succeed even with 30% error rate
	assert.True(t, successCount >= 7, "Most operations should succeed with retries")

	t.Log("\n✅ API error handling working correctly")
}
