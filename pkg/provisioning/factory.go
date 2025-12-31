package provisioning

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
)

// =============================================================================
// Provisioning Manager - Facade Pattern
// =============================================================================

// Manager is the main entry point for all provisioning features
type Manager struct {
	config *config.ClusterConfig

	// Components
	autoScaler      AutoScaler
	spotManager     SpotManager
	distributor     Distributor
	upgrader        Upgrader
	backupManager   BackupManagerInterface
	hookEngine      HookEngineInterface
	costEstimator   CostEstimatorInterface
	taintManager    TaintManagerInterface
	privateCluster  PrivateClusterManagerInterface
	imageManager    ImageManagerInterface

	// Event system
	eventEmitter *EventEmitterImpl

	mu sync.RWMutex
}

// =============================================================================
// Component Interfaces (for Dependency Injection)
// =============================================================================

// AutoScaler interface for auto-scaling operations
type AutoScaler interface {
	Start(ctx context.Context) error
	Stop()
	SetStrategy(name string) error
}

// SpotManager interface for spot instance operations
type SpotManager interface {
	CreateSpotInstance(ctx context.Context, nodeConfig *config.NodeConfig, spotConfig *config.SpotConfig) (interface{}, error)
	GetSpotPrice(ctx context.Context, provider, instanceType, zone string) (float64, error)
	HandleInterruption(ctx context.Context, provider, nodeID string) error
}

// Distributor interface for zone distribution
type Distributor interface {
	Distribute(ctx context.Context, totalNodes int, zones []string) (map[string]int, error)
	SetStrategy(name string) error
}

// Upgrader interface for cluster upgrades
type Upgrader interface {
	Plan(ctx context.Context, targetVersion string) (*UpgradePlan, error)
	Execute(ctx context.Context, plan *UpgradePlan) error
	Rollback(ctx context.Context, plan *UpgradePlan) error
	GetStatus(ctx context.Context) (*UpgradeStatus, error)
}

// BackupManagerInterface for backup operations
type BackupManagerInterface interface {
	CreateBackup(ctx context.Context, components []string) (*Backup, error)
	RestoreBackup(ctx context.Context, backupID string) error
	ListBackups(ctx context.Context) ([]*Backup, error)
	ScheduleBackup(ctx context.Context, schedule string) error
}

// HookEngineInterface for hook execution
type HookEngineInterface interface {
	TriggerHooks(ctx context.Context, event HookEvent, data interface{}) error
	RegisterHook(event HookEvent, hook Hook) error
}

// CostEstimatorInterface for cost estimation
type CostEstimatorInterface interface {
	EstimateNodeCost(ctx context.Context, nodeConfig *config.NodeConfig) (*CostEstimate, error)
	EstimateClusterCost(ctx context.Context, clusterConfig *config.ClusterConfig) (*ClusterCostEstimate, error)
}

// TaintManagerInterface for taint operations
type TaintManagerInterface interface {
	ApplyTaints(ctx context.Context, nodeName string, taints []config.TaintConfig) error
	RemoveTaints(ctx context.Context, nodeName string, taintKeys []string) error
	GetTaints(ctx context.Context, nodeName string) ([]config.TaintConfig, error)
}

// PrivateClusterManagerInterface for private cluster operations
type PrivateClusterManagerInterface interface {
	SetupPrivateNetwork(ctx context.Context, config *config.PrivateClusterConfig) error
	CreateNATGateway(ctx context.Context) error
}

// ImageManagerInterface for custom image operations
type ImageManagerInterface interface {
	GetImage(ctx context.Context, imageID string) (*Image, error)
	ValidateImage(ctx context.Context, imageID string) error
}

// =============================================================================
// Manager Configuration
// =============================================================================

// ManagerConfig holds configuration for the provisioning manager
type ManagerConfig struct {
	ClusterConfig *config.ClusterConfig

	// Optional component overrides (for testing/customization)
	AutoScaler     AutoScaler
	SpotManager    SpotManager
	Distributor    Distributor
	Upgrader       Upgrader
	BackupManager  BackupManagerInterface
	HookEngine     HookEngineInterface
	CostEstimator  CostEstimatorInterface
	TaintManager   TaintManagerInterface
	PrivateCluster PrivateClusterManagerInterface
	ImageManager   ImageManagerInterface
}

// NewManager creates a new provisioning manager
func NewManager(cfg *ManagerConfig) (*Manager, error) {
	if cfg.ClusterConfig == nil {
		return nil, fmt.Errorf("cluster config is required")
	}

	manager := &Manager{
		config:       cfg.ClusterConfig,
		eventEmitter: NewEventEmitter(),
	}

	// Use provided components or create defaults
	if cfg.AutoScaler != nil {
		manager.autoScaler = cfg.AutoScaler
	}
	if cfg.SpotManager != nil {
		manager.spotManager = cfg.SpotManager
	}
	if cfg.Distributor != nil {
		manager.distributor = cfg.Distributor
	}
	if cfg.Upgrader != nil {
		manager.upgrader = cfg.Upgrader
	}
	if cfg.BackupManager != nil {
		manager.backupManager = cfg.BackupManager
	}
	if cfg.HookEngine != nil {
		manager.hookEngine = cfg.HookEngine
	}
	if cfg.CostEstimator != nil {
		manager.costEstimator = cfg.CostEstimator
	}
	if cfg.TaintManager != nil {
		manager.taintManager = cfg.TaintManager
	}
	if cfg.PrivateCluster != nil {
		manager.privateCluster = cfg.PrivateCluster
	}
	if cfg.ImageManager != nil {
		manager.imageManager = cfg.ImageManager
	}

	return manager, nil
}

// =============================================================================
// Manager Methods - Delegates to appropriate components
// =============================================================================

// StartAutoScaling starts the auto-scaling loop
func (m *Manager) StartAutoScaling(ctx context.Context) error {
	if m.autoScaler == nil {
		return fmt.Errorf("auto-scaler not configured")
	}
	return m.autoScaler.Start(ctx)
}

// StopAutoScaling stops the auto-scaling loop
func (m *Manager) StopAutoScaling() {
	if m.autoScaler != nil {
		m.autoScaler.Stop()
	}
}

// CreateSpotInstance creates a spot instance
func (m *Manager) CreateSpotInstance(ctx context.Context, nodeConfig *config.NodeConfig, spotConfig *config.SpotConfig) (interface{}, error) {
	if m.spotManager == nil {
		return nil, fmt.Errorf("spot manager not configured")
	}

	result, err := m.spotManager.CreateSpotInstance(ctx, nodeConfig, spotConfig)
	if err != nil {
		return nil, err
	}

	// Trigger hook
	if m.hookEngine != nil {
		m.hookEngine.TriggerHooks(ctx, HookEventPostNodeCreate, map[string]interface{}{
			"node_name": nodeConfig.Name,
			"is_spot":   true,
		})
	}

	return result, nil
}

// DistributeNodes distributes nodes across zones
func (m *Manager) DistributeNodes(ctx context.Context, totalNodes int, zones []string) (map[string]int, error) {
	if m.distributor == nil {
		return nil, fmt.Errorf("distributor not configured")
	}
	return m.distributor.Distribute(ctx, totalNodes, zones)
}

// PlanUpgrade creates an upgrade plan
func (m *Manager) PlanUpgrade(ctx context.Context, targetVersion string) (*UpgradePlan, error) {
	if m.upgrader == nil {
		return nil, fmt.Errorf("upgrader not configured")
	}
	return m.upgrader.Plan(ctx, targetVersion)
}

// ExecuteUpgrade executes an upgrade plan
func (m *Manager) ExecuteUpgrade(ctx context.Context, plan *UpgradePlan) error {
	if m.upgrader == nil {
		return fmt.Errorf("upgrader not configured")
	}
	return m.upgrader.Execute(ctx, plan)
}

// CreateBackup creates a cluster backup
func (m *Manager) CreateBackup(ctx context.Context, components []string) (*Backup, error) {
	if m.backupManager == nil {
		return nil, fmt.Errorf("backup manager not configured")
	}

	backup, err := m.backupManager.CreateBackup(ctx, components)
	if err != nil {
		return nil, err
	}

	// Trigger hook
	if m.hookEngine != nil {
		m.hookEngine.TriggerHooks(ctx, HookEventBackupComplete, map[string]interface{}{
			"backup_id": backup.ID,
		})
	}

	return backup, nil
}

// RestoreBackup restores from a backup
func (m *Manager) RestoreBackup(ctx context.Context, backupID string) error {
	if m.backupManager == nil {
		return fmt.Errorf("backup manager not configured")
	}
	return m.backupManager.RestoreBackup(ctx, backupID)
}

// EstimateClusterCost estimates total cluster cost
func (m *Manager) EstimateClusterCost(ctx context.Context) (*ClusterCostEstimate, error) {
	if m.costEstimator == nil {
		return nil, fmt.Errorf("cost estimator not configured")
	}
	return m.costEstimator.EstimateClusterCost(ctx, m.config)
}

// ApplyTaints applies taints to a node
func (m *Manager) ApplyTaints(ctx context.Context, nodeName string, taints []config.TaintConfig) error {
	if m.taintManager == nil {
		return fmt.Errorf("taint manager not configured")
	}
	return m.taintManager.ApplyTaints(ctx, nodeName, taints)
}

// TriggerHook triggers hooks for an event
func (m *Manager) TriggerHook(ctx context.Context, event HookEvent, data interface{}) error {
	if m.hookEngine == nil {
		return nil // Hooks not configured is not an error
	}
	return m.hookEngine.TriggerHooks(ctx, event, data)
}

// GetEventEmitter returns the event emitter
func (m *Manager) GetEventEmitter() EventEmitter {
	return m.eventEmitter
}

// =============================================================================
// Event Emitter Implementation
// =============================================================================

// EventEmitterImpl implements EventEmitter
type EventEmitterImpl struct {
	handlers      map[string][]EventHandler
	subscriptions map[string]string
	nextID        int
	mu            sync.RWMutex
}

// NewEventEmitter creates a new event emitter
func NewEventEmitter() *EventEmitterImpl {
	return &EventEmitterImpl{
		handlers:      make(map[string][]EventHandler),
		subscriptions: make(map[string]string),
	}
}

// Emit emits an event to all subscribers
func (e *EventEmitterImpl) Emit(event Event) {
	e.mu.RLock()
	handlers := e.handlers[event.Type]
	e.mu.RUnlock()

	for _, handler := range handlers {
		go handler(event) // Non-blocking event handling
	}
}

// Subscribe subscribes to events of a type
func (e *EventEmitterImpl) Subscribe(eventType string, handler EventHandler) string {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.nextID++
	subscriptionID := fmt.Sprintf("sub-%d", e.nextID)

	e.handlers[eventType] = append(e.handlers[eventType], handler)
	e.subscriptions[subscriptionID] = eventType

	return subscriptionID
}

// Unsubscribe removes a subscription
func (e *EventEmitterImpl) Unsubscribe(subscriptionID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.subscriptions, subscriptionID)
	// Note: This doesn't remove the handler, just the tracking
	// A full implementation would need to track handler references
}

// =============================================================================
// Builder Pattern for Manager
// =============================================================================

// ManagerBuilder builds a provisioning manager with fluent API
type ManagerBuilder struct {
	config *ManagerConfig
}

// NewManagerBuilder creates a new manager builder
func NewManagerBuilder(clusterConfig *config.ClusterConfig) *ManagerBuilder {
	return &ManagerBuilder{
		config: &ManagerConfig{
			ClusterConfig: clusterConfig,
		},
	}
}

// WithAutoScaler adds an auto-scaler
func (b *ManagerBuilder) WithAutoScaler(as AutoScaler) *ManagerBuilder {
	b.config.AutoScaler = as
	return b
}

// WithSpotManager adds a spot manager
func (b *ManagerBuilder) WithSpotManager(sm SpotManager) *ManagerBuilder {
	b.config.SpotManager = sm
	return b
}

// WithDistributor adds a zone distributor
func (b *ManagerBuilder) WithDistributor(d Distributor) *ManagerBuilder {
	b.config.Distributor = d
	return b
}

// WithUpgrader adds an upgrader
func (b *ManagerBuilder) WithUpgrader(u Upgrader) *ManagerBuilder {
	b.config.Upgrader = u
	return b
}

// WithBackupManager adds a backup manager
func (b *ManagerBuilder) WithBackupManager(bm BackupManagerInterface) *ManagerBuilder {
	b.config.BackupManager = bm
	return b
}

// WithHookEngine adds a hook engine
func (b *ManagerBuilder) WithHookEngine(he HookEngineInterface) *ManagerBuilder {
	b.config.HookEngine = he
	return b
}

// WithCostEstimator adds a cost estimator
func (b *ManagerBuilder) WithCostEstimator(ce CostEstimatorInterface) *ManagerBuilder {
	b.config.CostEstimator = ce
	return b
}

// WithTaintManager adds a taint manager
func (b *ManagerBuilder) WithTaintManager(tm TaintManagerInterface) *ManagerBuilder {
	b.config.TaintManager = tm
	return b
}

// Build creates the manager
func (b *ManagerBuilder) Build() (*Manager, error) {
	return NewManager(b.config)
}

// =============================================================================
// Default Metrics Collector Implementation
// =============================================================================

// DefaultMetricsCollector provides a basic metrics collector
type DefaultMetricsCollector struct {
	kubeconfigPath string
}

// NewDefaultMetricsCollector creates a new metrics collector
func NewDefaultMetricsCollector(kubeconfigPath string) *DefaultMetricsCollector {
	return &DefaultMetricsCollector{
		kubeconfigPath: kubeconfigPath,
	}
}

// GetCPUUtilization returns average CPU utilization
func (c *DefaultMetricsCollector) GetCPUUtilization(ctx context.Context) (float64, error) {
	// In production, would use Kubernetes metrics API
	return 50.0, nil
}

// GetMemoryUtilization returns average memory utilization
func (c *DefaultMetricsCollector) GetMemoryUtilization(ctx context.Context) (float64, error) {
	// In production, would use Kubernetes metrics API
	return 60.0, nil
}

// GetCustomMetric returns a custom metric value
func (c *DefaultMetricsCollector) GetCustomMetric(ctx context.Context, name string) (float64, error) {
	return 0, fmt.Errorf("custom metric %s not found", name)
}

// =============================================================================
// Lifecycle Manager
// =============================================================================

// LifecycleManager manages provisioning lifecycle
type LifecycleManager struct {
	manager      *Manager
	isStarted    bool
	stopFuncs    []func()
	mu           sync.Mutex
}

// NewLifecycleManager creates a new lifecycle manager
func NewLifecycleManager(m *Manager) *LifecycleManager {
	return &LifecycleManager{
		manager:   m,
		stopFuncs: make([]func(), 0),
	}
}

// Start starts all provisioning services
func (l *LifecycleManager) Start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.isStarted {
		return fmt.Errorf("lifecycle manager already started")
	}

	// Start auto-scaling if configured
	if l.manager.autoScaler != nil {
		go l.manager.autoScaler.Start(ctx)
		l.stopFuncs = append(l.stopFuncs, l.manager.autoScaler.Stop)
	}

	// Schedule backups if configured
	if l.manager.config.Backup != nil && l.manager.config.Backup.Enabled {
		if l.manager.backupManager != nil {
			schedule := l.manager.config.Backup.Schedule
			if schedule != "" {
				l.manager.backupManager.ScheduleBackup(ctx, schedule)
			}
		}
	}

	l.isStarted = true
	return nil
}

// Stop stops all provisioning services
func (l *LifecycleManager) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, stopFunc := range l.stopFuncs {
		stopFunc()
	}

	l.stopFuncs = make([]func(), 0)
	l.isStarted = false
}

// =============================================================================
// Utility Functions
// =============================================================================

// GenerateEventID generates a unique event ID
func GenerateEventID() string {
	return fmt.Sprintf("evt-%d", time.Now().UnixNano())
}

// MergeLabels merges multiple label maps
func MergeLabels(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}
