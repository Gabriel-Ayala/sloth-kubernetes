// Package spot provides spot/preemptible instance management
package spot

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning"
)

// =============================================================================
// Spot Instance Manager
// =============================================================================

// Manager manages spot instances across providers
type Manager struct {
	providers    map[string]ProviderAdapter
	strategy     Strategy
	eventEmitter provisioning.EventEmitter

	// Interruption handling
	interruptionHandlers []InterruptionHandler

	mu sync.RWMutex
}

// ManagerConfig holds configuration for the spot manager
type ManagerConfig struct {
	EventEmitter provisioning.EventEmitter
	Strategy     Strategy
}

// NewManager creates a new spot instance manager
func NewManager(cfg *ManagerConfig) *Manager {
	strategy := cfg.Strategy
	if strategy == nil {
		strategy = &DefaultStrategy{}
	}

	return &Manager{
		providers:            make(map[string]ProviderAdapter),
		strategy:             strategy,
		eventEmitter:         cfg.EventEmitter,
		interruptionHandlers: make([]InterruptionHandler, 0),
	}
}

// RegisterProvider registers a provider adapter
func (m *Manager) RegisterProvider(name string, adapter ProviderAdapter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[name] = adapter
}

// CreateSpotInstance creates a spot instance using the appropriate provider
func (m *Manager) CreateSpotInstance(
	ctx context.Context,
	nodeConfig *config.NodeConfig,
	spotConfig *config.SpotConfig,
) (*providers.NodeOutput, error) {
	m.mu.RLock()
	adapter, exists := m.providers[nodeConfig.Provider]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("provider %s not registered", nodeConfig.Provider)
	}

	// Check spot availability
	available, err := adapter.IsSpotAvailable(ctx, nodeConfig.Size, nodeConfig.Zone)
	if err != nil {
		return nil, fmt.Errorf("failed to check spot availability: %w", err)
	}

	// If spot not available and fallback is enabled, create on-demand
	if !available && spotConfig.FallbackOnDemand {
		m.emitEvent("spot_fallback_to_ondemand", map[string]interface{}{
			"instance_type": nodeConfig.Size,
			"zone":          nodeConfig.Zone,
			"reason":        "spot_unavailable",
		})

		// Return nil to indicate on-demand should be used
		return nil, &SpotUnavailableError{
			InstanceType:       nodeConfig.Size,
			Zone:               nodeConfig.Zone,
			FallbackToOnDemand: true,
		}
	}

	if !available {
		return nil, &SpotUnavailableError{
			InstanceType:       nodeConfig.Size,
			Zone:               nodeConfig.Zone,
			FallbackToOnDemand: false,
		}
	}

	// Get current spot price
	spotPrice, err := adapter.GetSpotPrice(ctx, nodeConfig.Size, nodeConfig.Zone)
	if err != nil {
		return nil, fmt.Errorf("failed to get spot price: %w", err)
	}

	// Check if price is acceptable
	if !m.strategy.IsPriceAcceptable(spotConfig, spotPrice) {
		if spotConfig.FallbackOnDemand {
			m.emitEvent("spot_price_too_high", map[string]interface{}{
				"instance_type": nodeConfig.Size,
				"spot_price":    spotPrice,
				"max_price":     spotConfig.MaxPrice,
			})
			return nil, &SpotUnavailableError{
				InstanceType:       nodeConfig.Size,
				Zone:               nodeConfig.Zone,
				FallbackToOnDemand: true,
				Reason:             "price_too_high",
			}
		}
		return nil, fmt.Errorf("spot price $%.4f exceeds max price %s", spotPrice, spotConfig.MaxPrice)
	}

	// Create spot instance
	m.emitEvent("spot_instance_creating", map[string]interface{}{
		"instance_type": nodeConfig.Size,
		"zone":          nodeConfig.Zone,
		"spot_price":    spotPrice,
	})

	output, err := adapter.CreateSpotInstance(ctx, nodeConfig, spotConfig)
	if err != nil {
		m.emitEvent("spot_instance_create_failed", map[string]interface{}{
			"instance_type": nodeConfig.Size,
			"error":         err.Error(),
		})
		return nil, err
	}

	m.emitEvent("spot_instance_created", map[string]interface{}{
		"instance_id":   output.Name,
		"instance_type": nodeConfig.Size,
		"spot_price":    spotPrice,
	})

	return output, nil
}

// GetSpotPrice returns the current spot price for an instance type
func (m *Manager) GetSpotPrice(ctx context.Context, provider, instanceType, zone string) (float64, error) {
	m.mu.RLock()
	adapter, exists := m.providers[provider]
	m.mu.RUnlock()

	if !exists {
		return 0, fmt.Errorf("provider %s not registered", provider)
	}

	return adapter.GetSpotPrice(ctx, instanceType, zone)
}

// HandleInterruption handles a spot instance interruption
func (m *Manager) HandleInterruption(ctx context.Context, provider, nodeID string) error {
	m.mu.RLock()
	adapter, exists := m.providers[provider]
	handlers := m.interruptionHandlers
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("provider %s not registered", provider)
	}

	m.emitEvent("spot_interruption_received", map[string]interface{}{
		"node_id":  nodeID,
		"provider": provider,
	})

	// Execute all registered handlers
	for _, handler := range handlers {
		if err := handler.HandleInterruption(ctx, nodeID); err != nil {
			m.emitEvent("spot_interruption_handler_failed", map[string]interface{}{
				"node_id": nodeID,
				"handler": handler.Name(),
				"error":   err.Error(),
			})
		}
	}

	// Provider-specific handling
	return adapter.HandleInterruption(ctx, nodeID)
}

// RegisterInterruptionHandler adds an interruption handler
func (m *Manager) RegisterInterruptionHandler(handler InterruptionHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.interruptionHandlers = append(m.interruptionHandlers, handler)
}

// IsSpotAvailable checks if spot capacity is available
func (m *Manager) IsSpotAvailable(ctx context.Context, provider, instanceType, zone string) (bool, error) {
	m.mu.RLock()
	adapter, exists := m.providers[provider]
	m.mu.RUnlock()

	if !exists {
		return false, fmt.Errorf("provider %s not registered", provider)
	}

	return adapter.IsSpotAvailable(ctx, instanceType, zone)
}

// emitEvent sends an event
func (m *Manager) emitEvent(eventType string, data map[string]interface{}) {
	if m.eventEmitter != nil {
		m.eventEmitter.Emit(provisioning.Event{
			Type:      eventType,
			Timestamp: time.Now().Unix(),
			Data:      data,
			Source:    "spot_manager",
		})
	}
}

// =============================================================================
// Provider Adapter Interface
// =============================================================================

// ProviderAdapter adapts cloud providers for spot instance operations
type ProviderAdapter interface {
	// CreateSpotInstance creates a spot instance
	CreateSpotInstance(ctx context.Context, nodeConfig *config.NodeConfig, spotConfig *config.SpotConfig) (*providers.NodeOutput, error)
	// GetSpotPrice returns current spot price
	GetSpotPrice(ctx context.Context, instanceType, zone string) (float64, error)
	// IsSpotAvailable checks spot availability
	IsSpotAvailable(ctx context.Context, instanceType, zone string) (bool, error)
	// HandleInterruption handles interruption
	HandleInterruption(ctx context.Context, nodeID string) error
}

// InterruptionHandler handles spot instance interruptions
type InterruptionHandler interface {
	Name() string
	HandleInterruption(ctx context.Context, nodeID string) error
}

// =============================================================================
// Strategy Pattern for Spot Decisions
// =============================================================================

// Strategy defines spot instance allocation strategy
type Strategy interface {
	// IsPriceAcceptable checks if spot price is acceptable
	IsPriceAcceptable(spotConfig *config.SpotConfig, currentPrice float64) bool
	// ShouldUseSpot determines if spot should be used
	ShouldUseSpot(spotConfig *config.SpotConfig, nodeIndex, totalNodes int) bool
	// SelectBestZone selects the best zone for spot
	SelectBestZone(ctx context.Context, zones []string, prices map[string]float64) string
}

// DefaultStrategy implements the default spot strategy
type DefaultStrategy struct{}

// IsPriceAcceptable checks if the spot price is acceptable
func (s *DefaultStrategy) IsPriceAcceptable(spotConfig *config.SpotConfig, currentPrice float64) bool {
	if spotConfig.MaxSpotPrice > 0 {
		return currentPrice <= spotConfig.MaxSpotPrice
	}
	// If no max price set, accept any price
	return true
}

// ShouldUseSpot determines if this node should be spot based on percentage
func (s *DefaultStrategy) ShouldUseSpot(spotConfig *config.SpotConfig, nodeIndex, totalNodes int) bool {
	if !spotConfig.Enabled {
		return false
	}

	percentage := spotConfig.SpotPercentage
	if percentage == 0 {
		percentage = 100 // Default to all spot if enabled
	}

	// Calculate how many nodes should be spot
	spotNodes := (totalNodes * percentage) / 100
	return nodeIndex < spotNodes
}

// SelectBestZone selects the zone with the lowest price
func (s *DefaultStrategy) SelectBestZone(ctx context.Context, zones []string, prices map[string]float64) string {
	var bestZone string
	bestPrice := float64(-1)

	for _, zone := range zones {
		if price, ok := prices[zone]; ok {
			if bestPrice == -1 || price < bestPrice {
				bestPrice = price
				bestZone = zone
			}
		}
	}

	if bestZone == "" && len(zones) > 0 {
		return zones[0] // Fallback to first zone
	}

	return bestZone
}

// AggressiveStrategy prefers spot instances aggressively
type AggressiveStrategy struct{}

// IsPriceAcceptable is more lenient with pricing
func (s *AggressiveStrategy) IsPriceAcceptable(spotConfig *config.SpotConfig, currentPrice float64) bool {
	if spotConfig.MaxSpotPrice > 0 {
		// Accept up to 120% of max price
		return currentPrice <= spotConfig.MaxSpotPrice*1.2
	}
	return true
}

// ShouldUseSpot always uses spot if enabled
func (s *AggressiveStrategy) ShouldUseSpot(spotConfig *config.SpotConfig, nodeIndex, totalNodes int) bool {
	return spotConfig.Enabled
}

// SelectBestZone selects based on lowest price
func (s *AggressiveStrategy) SelectBestZone(ctx context.Context, zones []string, prices map[string]float64) string {
	return (&DefaultStrategy{}).SelectBestZone(ctx, zones, prices)
}

// ConservativeStrategy is more cautious with spot instances
type ConservativeStrategy struct{}

// IsPriceAcceptable is strict with pricing
func (s *ConservativeStrategy) IsPriceAcceptable(spotConfig *config.SpotConfig, currentPrice float64) bool {
	if spotConfig.MaxSpotPrice > 0 {
		// Only accept if price is at most 80% of max
		return currentPrice <= spotConfig.MaxSpotPrice*0.8
	}
	return false // Require explicit max price
}

// ShouldUseSpot limits spot to percentage with safety margin
func (s *ConservativeStrategy) ShouldUseSpot(spotConfig *config.SpotConfig, nodeIndex, totalNodes int) bool {
	if !spotConfig.Enabled {
		return false
	}

	percentage := spotConfig.SpotPercentage
	if percentage == 0 {
		percentage = 50 // Default to 50% for conservative
	}

	// Reduce percentage by 10% as safety margin
	safePercentage := percentage - 10
	if safePercentage < 0 {
		safePercentage = 0
	}

	spotNodes := (totalNodes * safePercentage) / 100
	return nodeIndex < spotNodes
}

// SelectBestZone prefers zones with stable pricing history
func (s *ConservativeStrategy) SelectBestZone(ctx context.Context, zones []string, prices map[string]float64) string {
	// For conservative strategy, avoid the cheapest (might be volatile)
	// and also the most expensive
	if len(zones) <= 2 {
		return (&DefaultStrategy{}).SelectBestZone(ctx, zones, prices)
	}

	// Find median price zone
	type zonePricePair struct {
		zone  string
		price float64
	}

	pairs := make([]zonePricePair, 0, len(zones))
	for _, zone := range zones {
		if price, ok := prices[zone]; ok {
			pairs = append(pairs, zonePricePair{zone, price})
		}
	}

	if len(pairs) == 0 {
		return zones[0]
	}

	// Sort by price (bubble sort for simplicity)
	for i := 0; i < len(pairs)-1; i++ {
		for j := 0; j < len(pairs)-i-1; j++ {
			if pairs[j].price > pairs[j+1].price {
				pairs[j], pairs[j+1] = pairs[j+1], pairs[j]
			}
		}
	}

	// Return median zone
	return pairs[len(pairs)/2].zone
}

// =============================================================================
// Custom Errors
// =============================================================================

// SpotUnavailableError indicates spot capacity is not available
type SpotUnavailableError struct {
	InstanceType       string
	Zone               string
	FallbackToOnDemand bool
	Reason             string
}

func (e *SpotUnavailableError) Error() string {
	reason := e.Reason
	if reason == "" {
		reason = "capacity_unavailable"
	}
	return fmt.Sprintf("spot instance %s not available in zone %s: %s", e.InstanceType, e.Zone, reason)
}

// ShouldFallback returns true if fallback to on-demand is recommended
func (e *SpotUnavailableError) ShouldFallback() bool {
	return e.FallbackToOnDemand
}
