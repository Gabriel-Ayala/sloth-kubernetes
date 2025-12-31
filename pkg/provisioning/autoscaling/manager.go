package autoscaling

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning"
)

// =============================================================================
// Auto-Scaling Manager - Orchestrates scaling operations
// =============================================================================

// Manager orchestrates auto-scaling operations
type Manager struct {
	config           *config.AutoScalingConfig
	strategy         provisioning.ScalingStrategy
	strategyRegistry *StrategyRegistry
	scaler           provisioning.NodeScaler
	metrics          provisioning.MetricsCollector
	eventEmitter     provisioning.EventEmitter

	// State
	currentNodes  int
	lastScaleUp   time.Time
	lastScaleDown time.Time
	isRunning     bool
	stopChan      chan struct{}

	mu sync.RWMutex
}

// ManagerConfig holds manager configuration
type ManagerConfig struct {
	AutoScalingConfig *config.AutoScalingConfig
	StrategyName      string
	Scaler            provisioning.NodeScaler
	Metrics           provisioning.MetricsCollector
	EventEmitter      provisioning.EventEmitter
	CheckInterval     time.Duration
}

// NewManager creates a new auto-scaling manager
func NewManager(cfg *ManagerConfig) (*Manager, error) {
	registry := NewStrategyRegistry()

	strategy, err := registry.Get(cfg.StrategyName)
	if err != nil {
		// Default to composite strategy
		strategy, _ = registry.Get("composite")
	}

	return &Manager{
		config:           cfg.AutoScalingConfig,
		strategy:         strategy,
		strategyRegistry: registry,
		scaler:           cfg.Scaler,
		metrics:          cfg.Metrics,
		eventEmitter:     cfg.EventEmitter,
		stopChan:         make(chan struct{}),
	}, nil
}

// Start begins the auto-scaling loop
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.isRunning {
		m.mu.Unlock()
		return fmt.Errorf("manager is already running")
	}
	m.isRunning = true
	m.mu.Unlock()

	checkInterval := time.Duration(m.config.Cooldown) * time.Second
	if checkInterval == 0 {
		checkInterval = 60 * time.Second
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-m.stopChan:
			return nil
		case <-ticker.C:
			if err := m.evaluate(ctx); err != nil {
				m.emitEvent("autoscaling_error", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}
	}
}

// Stop stops the auto-scaling loop
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isRunning {
		close(m.stopChan)
		m.isRunning = false
	}
}

// evaluate checks if scaling is needed and performs it
func (m *Manager) evaluate(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for scale up
	shouldScaleUp, upCount, err := m.strategy.ShouldScaleUp(ctx, m.metrics, m.config)
	if err != nil {
		return fmt.Errorf("scale up evaluation failed: %w", err)
	}

	if shouldScaleUp {
		return m.scaleUp(ctx, upCount)
	}

	// Check for scale down
	shouldScaleDown, downCount, err := m.strategy.ShouldScaleDown(ctx, m.metrics, m.config)
	if err != nil {
		return fmt.Errorf("scale down evaluation failed: %w", err)
	}

	if shouldScaleDown {
		return m.scaleDown(ctx, downCount)
	}

	return nil
}

// scaleUp adds nodes to the cluster
func (m *Manager) scaleUp(ctx context.Context, count int) error {
	// Check cooldown
	cooldown := time.Duration(m.config.Cooldown) * time.Second
	if cooldown == 0 {
		cooldown = 5 * time.Minute
	}

	if time.Since(m.lastScaleUp) < cooldown {
		return nil // Still in cooldown
	}

	// Check max nodes
	currentCount := m.scaler.GetCurrentCount()
	newCount := currentCount + count

	if newCount > m.config.MaxNodes {
		count = m.config.MaxNodes - currentCount
		if count <= 0 {
			return nil // Already at max
		}
	}

	// Emit pre-scale event
	m.emitEvent("autoscaling_scale_up_start", map[string]interface{}{
		"current_nodes": currentCount,
		"nodes_to_add":  count,
	})

	// Perform scale up
	if err := m.scaler.ScaleUp(ctx, count); err != nil {
		m.emitEvent("autoscaling_scale_up_failed", map[string]interface{}{
			"error": err.Error(),
		})
		return fmt.Errorf("scale up failed: %w", err)
	}

	m.lastScaleUp = time.Now()
	m.currentNodes = currentCount + count

	// Emit success event
	m.emitEvent("autoscaling_scale_up_complete", map[string]interface{}{
		"previous_nodes": currentCount,
		"current_nodes":  m.currentNodes,
		"nodes_added":    count,
	})

	return nil
}

// scaleDown removes nodes from the cluster
func (m *Manager) scaleDown(ctx context.Context, count int) error {
	// Check cooldown
	scaleDownDelay := time.Duration(m.config.ScaleDownDelay) * time.Second
	if scaleDownDelay == 0 {
		scaleDownDelay = 10 * time.Minute
	}

	if time.Since(m.lastScaleDown) < scaleDownDelay {
		return nil // Still in cooldown
	}

	// Check min nodes
	currentCount := m.scaler.GetCurrentCount()
	newCount := currentCount - count

	if newCount < m.config.MinNodes {
		count = currentCount - m.config.MinNodes
		if count <= 0 {
			return nil // Already at min
		}
	}

	// Emit pre-scale event
	m.emitEvent("autoscaling_scale_down_start", map[string]interface{}{
		"current_nodes":    currentCount,
		"nodes_to_remove":  count,
	})

	// Perform scale down
	if err := m.scaler.ScaleDown(ctx, count); err != nil {
		m.emitEvent("autoscaling_scale_down_failed", map[string]interface{}{
			"error": err.Error(),
		})
		return fmt.Errorf("scale down failed: %w", err)
	}

	m.lastScaleDown = time.Now()
	m.currentNodes = currentCount - count

	// Emit success event
	m.emitEvent("autoscaling_scale_down_complete", map[string]interface{}{
		"previous_nodes": currentCount,
		"current_nodes":  m.currentNodes,
		"nodes_removed":  count,
	})

	return nil
}

// SetStrategy changes the scaling strategy
func (m *Manager) SetStrategy(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	strategy, err := m.strategyRegistry.Get(name)
	if err != nil {
		return err
	}

	m.strategy = strategy
	return nil
}

// GetStatus returns current auto-scaling status
func (m *Manager) GetStatus() *AutoScalingStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &AutoScalingStatus{
		Enabled:       m.config.Enabled,
		CurrentNodes:  m.currentNodes,
		MinNodes:      m.config.MinNodes,
		MaxNodes:      m.config.MaxNodes,
		Strategy:      m.strategy.Name(),
		LastScaleUp:   m.lastScaleUp,
		LastScaleDown: m.lastScaleDown,
		IsRunning:     m.isRunning,
	}
}

// emitEvent sends an event to the event emitter
func (m *Manager) emitEvent(eventType string, data map[string]interface{}) {
	if m.eventEmitter != nil {
		m.eventEmitter.Emit(provisioning.Event{
			Type:      eventType,
			Timestamp: time.Now().Unix(),
			Data:      data,
			Source:    "autoscaling_manager",
		})
	}
}

// AutoScalingStatus represents the current auto-scaling state
type AutoScalingStatus struct {
	Enabled       bool
	CurrentNodes  int
	MinNodes      int
	MaxNodes      int
	Strategy      string
	LastScaleUp   time.Time
	LastScaleDown time.Time
	IsRunning     bool
}

// =============================================================================
// Node Scaler Implementation
// =============================================================================

// NodePoolScaler implements NodeScaler for a node pool
type NodePoolScaler struct {
	poolName     string
	nodeCreator  NodeCreator
	nodeDeleter  NodeDeleter
	nodeCounter  NodeCounter
	nodeSelector NodeSelector
}

// NodeCreator creates new nodes
type NodeCreator interface {
	CreateNodes(ctx context.Context, poolName string, count int) error
}

// NodeDeleter deletes nodes
type NodeDeleter interface {
	DeleteNodes(ctx context.Context, nodeIDs []string) error
}

// NodeCounter counts nodes
type NodeCounter interface {
	GetNodeCount(ctx context.Context, poolName string) (int, error)
}

// NodeSelector selects nodes for deletion
type NodeSelector interface {
	SelectNodesForDeletion(ctx context.Context, poolName string, count int) ([]string, error)
}

// NewNodePoolScaler creates a new node pool scaler
func NewNodePoolScaler(
	poolName string,
	creator NodeCreator,
	deleter NodeDeleter,
	counter NodeCounter,
	selector NodeSelector,
) *NodePoolScaler {
	return &NodePoolScaler{
		poolName:     poolName,
		nodeCreator:  creator,
		nodeDeleter:  deleter,
		nodeCounter:  counter,
		nodeSelector: selector,
	}
}

// ScaleUp adds nodes to the pool
func (s *NodePoolScaler) ScaleUp(ctx context.Context, count int) error {
	return s.nodeCreator.CreateNodes(ctx, s.poolName, count)
}

// ScaleDown removes nodes from the pool
func (s *NodePoolScaler) ScaleDown(ctx context.Context, count int) error {
	// Select nodes to delete
	nodeIDs, err := s.nodeSelector.SelectNodesForDeletion(ctx, s.poolName, count)
	if err != nil {
		return fmt.Errorf("failed to select nodes for deletion: %w", err)
	}

	// Delete selected nodes
	return s.nodeDeleter.DeleteNodes(ctx, nodeIDs)
}

// GetCurrentCount returns current node count
func (s *NodePoolScaler) GetCurrentCount() int {
	count, _ := s.nodeCounter.GetNodeCount(context.Background(), s.poolName)
	return count
}

// GetDesiredCount returns desired node count (same as current for now)
func (s *NodePoolScaler) GetDesiredCount() int {
	return s.GetCurrentCount()
}
