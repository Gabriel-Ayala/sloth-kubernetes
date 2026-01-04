package spot

import (
	"context"
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockProviderAdapter implements ProviderAdapter for testing
type MockProviderAdapter struct {
	spotAvailable  bool
	spotPrice      float64
	createError    error
	interruptError error
}

func (m *MockProviderAdapter) CreateSpotInstance(ctx context.Context, nodeConfig *config.NodeConfig, spotConfig *config.SpotConfig) (*providers.NodeOutput, error) {
	if m.createError != nil {
		return nil, m.createError
	}
	return &providers.NodeOutput{
		Name:     nodeConfig.Name,
		Provider: nodeConfig.Provider,
		Region:   nodeConfig.Region,
	}, nil
}

func (m *MockProviderAdapter) GetSpotPrice(ctx context.Context, instanceType, zone string) (float64, error) {
	return m.spotPrice, nil
}

func (m *MockProviderAdapter) IsSpotAvailable(ctx context.Context, instanceType, zone string) (bool, error) {
	return m.spotAvailable, nil
}

func (m *MockProviderAdapter) HandleInterruption(ctx context.Context, nodeID string) error {
	return m.interruptError
}

// MockInterruptionHandler implements InterruptionHandler for testing
type MockInterruptionHandler struct {
	name  string
	err   error
	calls int
}

func (m *MockInterruptionHandler) Name() string {
	return m.name
}

func (m *MockInterruptionHandler) HandleInterruption(ctx context.Context, nodeID string) error {
	m.calls++
	return m.err
}

func TestNewManager(t *testing.T) {
	manager := NewManager(&ManagerConfig{})

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.providers)
	assert.NotNil(t, manager.strategy)
	assert.IsType(t, &DefaultStrategy{}, manager.strategy)
}

func TestNewManager_WithCustomStrategy(t *testing.T) {
	strategy := &AggressiveStrategy{}
	manager := NewManager(&ManagerConfig{
		Strategy: strategy,
	})

	assert.NotNil(t, manager)
	assert.Equal(t, strategy, manager.strategy)
}

func TestManager_RegisterProvider(t *testing.T) {
	manager := NewManager(&ManagerConfig{})
	adapter := &MockProviderAdapter{}

	manager.RegisterProvider("aws", adapter)

	assert.Len(t, manager.providers, 1)
}

func TestManager_CreateSpotInstance_Success(t *testing.T) {
	adapter := &MockProviderAdapter{
		spotAvailable: true,
		spotPrice:     0.05,
	}

	manager := NewManager(&ManagerConfig{})
	manager.RegisterProvider("aws", adapter)

	nodeConfig := &config.NodeConfig{
		Name:     "test-node",
		Provider: "aws",
		Size:     "t3.medium",
		Zone:     "us-east-1a",
	}

	spotConfig := &config.SpotConfig{
		Enabled:      true,
		MaxSpotPrice: 0.10,
	}

	output, err := manager.CreateSpotInstance(context.Background(), nodeConfig, spotConfig)
	require.NoError(t, err)
	assert.Equal(t, "test-node", output.Name)
}

func TestManager_CreateSpotInstance_ProviderNotRegistered(t *testing.T) {
	manager := NewManager(&ManagerConfig{})

	nodeConfig := &config.NodeConfig{
		Name:     "test-node",
		Provider: "gcp",
		Size:     "n1-standard-1",
	}

	spotConfig := &config.SpotConfig{Enabled: true}

	_, err := manager.CreateSpotInstance(context.Background(), nodeConfig, spotConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provider gcp not registered")
}

func TestManager_CreateSpotInstance_SpotUnavailable_WithFallback(t *testing.T) {
	adapter := &MockProviderAdapter{
		spotAvailable: false,
	}

	manager := NewManager(&ManagerConfig{})
	manager.RegisterProvider("aws", adapter)

	nodeConfig := &config.NodeConfig{
		Name:     "test-node",
		Provider: "aws",
		Size:     "t3.medium",
		Zone:     "us-east-1a",
	}

	spotConfig := &config.SpotConfig{
		Enabled:          true,
		FallbackOnDemand: true,
	}

	_, err := manager.CreateSpotInstance(context.Background(), nodeConfig, spotConfig)
	assert.Error(t, err)

	spotErr, ok := err.(*SpotUnavailableError)
	assert.True(t, ok)
	assert.True(t, spotErr.ShouldFallback())
}

func TestManager_CreateSpotInstance_SpotUnavailable_NoFallback(t *testing.T) {
	adapter := &MockProviderAdapter{
		spotAvailable: false,
	}

	manager := NewManager(&ManagerConfig{})
	manager.RegisterProvider("aws", adapter)

	nodeConfig := &config.NodeConfig{
		Name:     "test-node",
		Provider: "aws",
		Size:     "t3.medium",
		Zone:     "us-east-1a",
	}

	spotConfig := &config.SpotConfig{
		Enabled:          true,
		FallbackOnDemand: false,
	}

	_, err := manager.CreateSpotInstance(context.Background(), nodeConfig, spotConfig)
	assert.Error(t, err)

	spotErr, ok := err.(*SpotUnavailableError)
	assert.True(t, ok)
	assert.False(t, spotErr.ShouldFallback())
}

func TestManager_CreateSpotInstance_PriceTooHigh(t *testing.T) {
	adapter := &MockProviderAdapter{
		spotAvailable: true,
		spotPrice:     0.20, // Higher than max
	}

	manager := NewManager(&ManagerConfig{})
	manager.RegisterProvider("aws", adapter)

	nodeConfig := &config.NodeConfig{
		Name:     "test-node",
		Provider: "aws",
		Size:     "t3.medium",
		Zone:     "us-east-1a",
	}

	spotConfig := &config.SpotConfig{
		Enabled:          true,
		MaxSpotPrice:     0.10,
		FallbackOnDemand: true,
	}

	_, err := manager.CreateSpotInstance(context.Background(), nodeConfig, spotConfig)
	assert.Error(t, err)

	spotErr, ok := err.(*SpotUnavailableError)
	assert.True(t, ok)
	assert.Equal(t, "price_too_high", spotErr.Reason)
}

func TestManager_GetSpotPrice(t *testing.T) {
	adapter := &MockProviderAdapter{
		spotPrice: 0.05,
	}

	manager := NewManager(&ManagerConfig{})
	manager.RegisterProvider("aws", adapter)

	price, err := manager.GetSpotPrice(context.Background(), "aws", "t3.medium", "us-east-1a")
	require.NoError(t, err)
	assert.Equal(t, 0.05, price)
}

func TestManager_GetSpotPrice_ProviderNotRegistered(t *testing.T) {
	manager := NewManager(&ManagerConfig{})

	_, err := manager.GetSpotPrice(context.Background(), "aws", "t3.medium", "us-east-1a")
	assert.Error(t, err)
}

func TestManager_IsSpotAvailable(t *testing.T) {
	adapter := &MockProviderAdapter{
		spotAvailable: true,
	}

	manager := NewManager(&ManagerConfig{})
	manager.RegisterProvider("aws", adapter)

	available, err := manager.IsSpotAvailable(context.Background(), "aws", "t3.medium", "us-east-1a")
	require.NoError(t, err)
	assert.True(t, available)
}

func TestManager_HandleInterruption(t *testing.T) {
	adapter := &MockProviderAdapter{}
	handler := &MockInterruptionHandler{name: "test-handler"}

	manager := NewManager(&ManagerConfig{})
	manager.RegisterProvider("aws", adapter)
	manager.RegisterInterruptionHandler(handler)

	err := manager.HandleInterruption(context.Background(), "aws", "node-1")
	require.NoError(t, err)
	assert.Equal(t, 1, handler.calls)
}

func TestManager_RegisterInterruptionHandler(t *testing.T) {
	manager := NewManager(&ManagerConfig{})
	handler1 := &MockInterruptionHandler{name: "handler1"}
	handler2 := &MockInterruptionHandler{name: "handler2"}

	manager.RegisterInterruptionHandler(handler1)
	manager.RegisterInterruptionHandler(handler2)

	assert.Len(t, manager.interruptionHandlers, 2)
}

// Strategy Tests

func TestDefaultStrategy_IsPriceAcceptable(t *testing.T) {
	strategy := &DefaultStrategy{}

	tests := []struct {
		name         string
		maxPrice     float64
		currentPrice float64
		expected     bool
	}{
		{"Price below max", 0.10, 0.05, true},
		{"Price equal to max", 0.10, 0.10, true},
		{"Price above max", 0.10, 0.15, false},
		{"No max price set", 0, 0.50, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spotConfig := &config.SpotConfig{MaxSpotPrice: tt.maxPrice}
			result := strategy.IsPriceAcceptable(spotConfig, tt.currentPrice)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultStrategy_ShouldUseSpot(t *testing.T) {
	strategy := &DefaultStrategy{}

	tests := []struct {
		name       string
		enabled    bool
		percentage int
		nodeIndex  int
		totalNodes int
		expected   bool
	}{
		{"Disabled", false, 100, 0, 5, false},
		{"100% spot - first node", true, 100, 0, 5, true},
		{"100% spot - last node", true, 100, 4, 5, true},
		{"50% spot - first node", true, 50, 0, 4, true},
		{"50% spot - third node", true, 50, 2, 4, false},
		{"Default 100% when not set", true, 0, 0, 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spotConfig := &config.SpotConfig{
				Enabled:        tt.enabled,
				SpotPercentage: tt.percentage,
			}
			result := strategy.ShouldUseSpot(spotConfig, tt.nodeIndex, tt.totalNodes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultStrategy_SelectBestZone(t *testing.T) {
	strategy := &DefaultStrategy{}
	ctx := context.Background()

	tests := []struct {
		name     string
		zones    []string
		prices   map[string]float64
		expected string
	}{
		{
			"Single zone",
			[]string{"us-east-1a"},
			map[string]float64{"us-east-1a": 0.05},
			"us-east-1a",
		},
		{
			"Multiple zones - select cheapest",
			[]string{"us-east-1a", "us-east-1b", "us-east-1c"},
			map[string]float64{"us-east-1a": 0.05, "us-east-1b": 0.03, "us-east-1c": 0.07},
			"us-east-1b",
		},
		{
			"Empty prices - fallback to first",
			[]string{"us-east-1a", "us-east-1b"},
			map[string]float64{},
			"us-east-1a",
		},
		{
			"Empty zones",
			[]string{},
			map[string]float64{},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strategy.SelectBestZone(ctx, tt.zones, tt.prices)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAggressiveStrategy_IsPriceAcceptable(t *testing.T) {
	strategy := &AggressiveStrategy{}

	tests := []struct {
		name         string
		maxPrice     float64
		currentPrice float64
		expected     bool
	}{
		{"Price below max", 0.10, 0.05, true},
		{"Price at 120% of max", 0.10, 0.12, true},
		{"Price above 120% of max", 0.10, 0.15, false},
		{"No max price set", 0, 0.50, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spotConfig := &config.SpotConfig{MaxSpotPrice: tt.maxPrice}
			result := strategy.IsPriceAcceptable(spotConfig, tt.currentPrice)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAggressiveStrategy_ShouldUseSpot(t *testing.T) {
	strategy := &AggressiveStrategy{}

	spotConfig := &config.SpotConfig{Enabled: true, SpotPercentage: 50}

	// Aggressive strategy always uses spot if enabled
	assert.True(t, strategy.ShouldUseSpot(spotConfig, 0, 10))
	assert.True(t, strategy.ShouldUseSpot(spotConfig, 9, 10))

	// Unless disabled
	spotConfig.Enabled = false
	assert.False(t, strategy.ShouldUseSpot(spotConfig, 0, 10))
}

func TestConservativeStrategy_IsPriceAcceptable(t *testing.T) {
	strategy := &ConservativeStrategy{}

	tests := []struct {
		name         string
		maxPrice     float64
		currentPrice float64
		expected     bool
	}{
		{"Price at 80% of max", 0.10, 0.08, true},
		{"Price below 80% of max", 0.10, 0.05, true},
		{"Price above 80% of max", 0.10, 0.09, false},
		{"No max price set - reject", 0, 0.05, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spotConfig := &config.SpotConfig{MaxSpotPrice: tt.maxPrice}
			result := strategy.IsPriceAcceptable(spotConfig, tt.currentPrice)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConservativeStrategy_ShouldUseSpot(t *testing.T) {
	strategy := &ConservativeStrategy{}

	tests := []struct {
		name       string
		percentage int
		nodeIndex  int
		totalNodes int
		expected   bool
	}{
		{"50% with safety margin - first node", 50, 0, 5, true},
		{"50% with safety margin - second node", 50, 1, 5, true},
		{"50% with safety margin - third node", 50, 2, 5, false},
		{"Default 50% when not set", 0, 0, 4, true},
		{"Default 50% when not set - second node", 0, 1, 4, false}, // 50%-10%=40%, so 4*40/100=1 spot node
		{"Default 50% when not set - third node", 0, 2, 4, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spotConfig := &config.SpotConfig{
				Enabled:        true,
				SpotPercentage: tt.percentage,
			}
			result := strategy.ShouldUseSpot(spotConfig, tt.nodeIndex, tt.totalNodes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConservativeStrategy_SelectBestZone(t *testing.T) {
	strategy := &ConservativeStrategy{}
	ctx := context.Background()

	// With 3 zones, should select median price zone
	zones := []string{"us-east-1a", "us-east-1b", "us-east-1c"}
	prices := map[string]float64{
		"us-east-1a": 0.03, // cheapest
		"us-east-1b": 0.05, // median
		"us-east-1c": 0.07, // most expensive
	}

	result := strategy.SelectBestZone(ctx, zones, prices)
	assert.Equal(t, "us-east-1b", result)
}

func TestSpotUnavailableError(t *testing.T) {
	err := &SpotUnavailableError{
		InstanceType:       "t3.medium",
		Zone:               "us-east-1a",
		FallbackToOnDemand: true,
		Reason:             "price_too_high",
	}

	assert.Contains(t, err.Error(), "t3.medium")
	assert.Contains(t, err.Error(), "us-east-1a")
	assert.Contains(t, err.Error(), "price_too_high")
	assert.True(t, err.ShouldFallback())
}
