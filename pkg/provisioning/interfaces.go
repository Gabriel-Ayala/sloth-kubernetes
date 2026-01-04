// Package provisioning provides advanced provisioning features for Kubernetes clusters
package provisioning

import (
	"context"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
)

// =============================================================================
// Core Interfaces - Interface Segregation Principle
// =============================================================================

// NodeScaler handles node scaling operations
type NodeScaler interface {
	// ScaleUp adds nodes to the cluster
	ScaleUp(ctx context.Context, count int) error
	// ScaleDown removes nodes from the cluster
	ScaleDown(ctx context.Context, count int) error
	// GetCurrentCount returns current node count
	GetCurrentCount() int
	// GetDesiredCount returns desired node count
	GetDesiredCount() int
}

// MetricsCollector collects cluster metrics for scaling decisions
type MetricsCollector interface {
	// GetCPUUtilization returns average CPU utilization (0-100)
	GetCPUUtilization(ctx context.Context) (float64, error)
	// GetMemoryUtilization returns average memory utilization (0-100)
	GetMemoryUtilization(ctx context.Context) (float64, error)
	// GetCustomMetric returns a custom metric value
	GetCustomMetric(ctx context.Context, name string) (float64, error)
}

// ScalingStrategy defines the strategy for auto-scaling decisions
type ScalingStrategy interface {
	// Name returns the strategy name
	Name() string
	// ShouldScaleUp determines if cluster should scale up
	ShouldScaleUp(ctx context.Context, metrics MetricsCollector, config *config.AutoScalingConfig) (bool, int, error)
	// ShouldScaleDown determines if cluster should scale down
	ShouldScaleDown(ctx context.Context, metrics MetricsCollector, config *config.AutoScalingConfig) (bool, int, error)
}

// =============================================================================
// Spot Instance Interfaces
// =============================================================================

// SpotInstanceManager manages spot/preemptible instances
type SpotInstanceManager interface {
	// CreateSpotInstance creates a spot instance
	CreateSpotInstance(ctx context.Context, nodeConfig *config.NodeConfig, spotConfig *config.SpotConfig) (*providers.NodeOutput, error)
	// GetSpotPrice returns current spot price for instance type
	GetSpotPrice(ctx context.Context, instanceType string, zone string) (float64, error)
	// HandleInterruption handles spot instance interruption
	HandleInterruption(ctx context.Context, nodeID string) error
	// IsSpotAvailable checks if spot capacity is available
	IsSpotAvailable(ctx context.Context, instanceType string, zone string) (bool, error)
}

// SpotStrategy defines spot instance allocation strategy
type SpotStrategy interface {
	// Name returns strategy name
	Name() string
	// SelectInstanceType selects best instance type for spot
	SelectInstanceType(ctx context.Context, requirements *InstanceRequirements) (string, error)
	// ShouldUseFallback determines if should fall back to on-demand
	ShouldUseFallback(ctx context.Context, spotConfig *config.SpotConfig, spotPrice float64) bool
}

// InstanceRequirements defines requirements for instance selection
type InstanceRequirements struct {
	MinCPU      int
	MinMemoryGB int
	MaxPrice    float64
	Zone        string
	Labels      map[string]string
}

// =============================================================================
// Zone Distribution Interfaces
// =============================================================================

// ZoneDistributor distributes nodes across availability zones
type ZoneDistributor interface {
	// Distribute calculates node distribution across zones
	Distribute(ctx context.Context, totalNodes int, zones []string) (map[string]int, error)
	// Rebalance rebalances existing nodes across zones
	Rebalance(ctx context.Context, currentDistribution map[string]int) (map[string]int, error)
}

// DistributionStrategy defines zone distribution strategy
type DistributionStrategy interface {
	// Name returns strategy name
	Name() string
	// Calculate calculates distribution for given constraints
	Calculate(totalNodes int, zones []string, weights map[string]int) map[string]int
}

// =============================================================================
// Upgrade Interfaces
// =============================================================================

// UpgradeOrchestrator orchestrates cluster upgrades
type UpgradeOrchestrator interface {
	// Plan creates an upgrade plan
	Plan(ctx context.Context, targetVersion string) (*UpgradePlan, error)
	// Execute executes the upgrade plan
	Execute(ctx context.Context, plan *UpgradePlan) error
	// Rollback rolls back a failed upgrade
	Rollback(ctx context.Context, plan *UpgradePlan) error
	// GetStatus returns current upgrade status
	GetStatus(ctx context.Context) (*UpgradeStatus, error)
}

// UpgradeStrategy defines upgrade execution strategy
type UpgradeStrategy interface {
	// Name returns strategy name
	Name() string
	// PrepareNode prepares a node for upgrade
	PrepareNode(ctx context.Context, node *providers.NodeOutput) error
	// UpgradeNode upgrades a single node
	UpgradeNode(ctx context.Context, node *providers.NodeOutput, targetVersion string) error
	// ValidateNode validates node after upgrade
	ValidateNode(ctx context.Context, node *providers.NodeOutput) error
	// GetBatchSize returns number of nodes to upgrade simultaneously
	GetBatchSize(totalNodes int, maxUnavailable int) int
}

// NodeDrainer handles node drain operations
type NodeDrainer interface {
	// Drain drains a node of workloads
	Drain(ctx context.Context, nodeName string, timeout int) error
	// Uncordon makes a node schedulable again
	Uncordon(ctx context.Context, nodeName string) error
	// Cordon marks a node as unschedulable
	Cordon(ctx context.Context, nodeName string) error
}

// UpgradePlan represents an upgrade execution plan
type UpgradePlan struct {
	ID             string
	CurrentVersion string
	TargetVersion  string
	Strategy       string
	Nodes          []UpgradeNodePlan
	CreatedAt      int64
	EstimatedTime  int // seconds
}

// UpgradeNodePlan represents upgrade plan for a single node
type UpgradeNodePlan struct {
	NodeName string
	NodeID   string
	Order    int
	Batch    int
	Status   string
}

// UpgradeStatus represents current upgrade status
type UpgradeStatus struct {
	PlanID          string
	Phase           string // planning, executing, completed, failed, rolled_back
	Progress        int    // 0-100
	CurrentNode     string
	CompletedNodes  []string
	FailedNodes     []string
	StartedAt       int64
	EstimatedFinish int64
	Error           string
}

// =============================================================================
// Backup Interfaces
// =============================================================================

// BackupManager manages cluster backups
type BackupManager interface {
	// CreateBackup creates a new backup
	CreateBackup(ctx context.Context, components []string) (*Backup, error)
	// RestoreBackup restores from a backup
	RestoreBackup(ctx context.Context, backupID string) error
	// ListBackups lists available backups
	ListBackups(ctx context.Context) ([]*Backup, error)
	// DeleteBackup deletes a backup
	DeleteBackup(ctx context.Context, backupID string) error
	// ScheduleBackup schedules automatic backups
	ScheduleBackup(ctx context.Context, schedule string) error
}

// BackupStorage defines backup storage backend
type BackupStorage interface {
	// Name returns storage name
	Name() string
	// Upload uploads backup data
	Upload(ctx context.Context, backupID string, data []byte) error
	// Download downloads backup data
	Download(ctx context.Context, backupID string) ([]byte, error)
	// Delete deletes backup data
	Delete(ctx context.Context, backupID string) error
	// List lists available backups
	List(ctx context.Context) ([]string, error)
}

// BackupComponent defines a component that can be backed up
type BackupComponent interface {
	// Name returns component name
	Name() string
	// Backup creates backup of the component
	Backup(ctx context.Context) ([]byte, error)
	// Restore restores component from backup
	Restore(ctx context.Context, data []byte) error
}

// Backup represents a cluster backup
type Backup struct {
	ID         string
	Components []string
	Size       int64
	CreatedAt  int64
	ExpiresAt  int64
	Location   string
	Status     string
	Metadata   map[string]string
}

// =============================================================================
// Hook Interfaces
// =============================================================================

// HookEngine executes provisioning hooks
type HookEngine interface {
	// RegisterHook registers a hook
	RegisterHook(event HookEvent, hook Hook) error
	// UnregisterHook removes a hook
	UnregisterHook(event HookEvent, hookID string) error
	// TriggerHooks triggers all hooks for an event
	TriggerHooks(ctx context.Context, event HookEvent, data interface{}) error
}

// Hook represents a single hook
type Hook interface {
	// ID returns unique hook identifier
	ID() string
	// Execute executes the hook
	Execute(ctx context.Context, data interface{}) error
	// Timeout returns hook timeout in seconds
	Timeout() int
	// RetryCount returns number of retries on failure
	RetryCount() int
}

// HookExecutor executes different types of hooks
type HookExecutor interface {
	// Execute runs the hook action
	Execute(ctx context.Context, action *config.HookAction, data interface{}) error
}

// HookEvent represents hook trigger events
type HookEvent string

const (
	HookEventPostNodeCreate    HookEvent = "post_node_create"
	HookEventPreNodeDelete     HookEvent = "pre_node_delete"
	HookEventPreClusterDestroy HookEvent = "pre_cluster_destroy"
	HookEventPostClusterReady  HookEvent = "post_cluster_ready"
	HookEventPostUpgrade       HookEvent = "post_upgrade"
	HookEventPreUpgrade        HookEvent = "pre_upgrade"
	HookEventBackupComplete    HookEvent = "backup_complete"
	HookEventScaleUp           HookEvent = "scale_up"
	HookEventScaleDown         HookEvent = "scale_down"
)

// =============================================================================
// Cost Estimation Interfaces
// =============================================================================

// CostEstimator estimates infrastructure costs
type CostEstimator interface {
	// EstimateNodeCost estimates cost for a node
	EstimateNodeCost(ctx context.Context, nodeConfig *config.NodeConfig) (*CostEstimate, error)
	// EstimateClusterCost estimates total cluster cost
	EstimateClusterCost(ctx context.Context, clusterConfig *config.ClusterConfig) (*ClusterCostEstimate, error)
	// GetCurrentSpend returns current month spending
	GetCurrentSpend(ctx context.Context) (float64, error)
}

// PriceProvider provides pricing information
type PriceProvider interface {
	// Name returns provider name
	Name() string
	// GetInstancePrice returns hourly price for instance type
	GetInstancePrice(ctx context.Context, instanceType string, region string) (float64, error)
	// GetStoragePrice returns price per GB/month for storage
	GetStoragePrice(ctx context.Context, storageType string, region string) (float64, error)
	// GetNetworkPrice returns price per GB for network transfer
	GetNetworkPrice(ctx context.Context, region string) (float64, error)
}

// CostEstimate represents cost estimation for a resource
type CostEstimate struct {
	Resource    string
	HourlyCost  float64
	MonthlyCost float64
	YearlyCost  float64
	Currency    string
	IsSpot      bool
	SpotSavings float64
	Breakdown   map[string]float64
}

// ClusterCostEstimate represents total cluster cost estimation
type ClusterCostEstimate struct {
	TotalMonthlyCost float64
	TotalYearlyCost  float64
	Currency         string
	NodeCosts        []*CostEstimate
	StorageCost      float64
	NetworkCost      float64
	LoadBalancerCost float64
	SpotSavings      float64
	Recommendations  []CostRecommendation
}

// CostRecommendation represents a cost optimization recommendation
type CostRecommendation struct {
	Type              string // right_sizing, spot_usage, reserved_instances
	Description       string
	PotentialSavings  float64
	Resource          string
	CurrentConfig     string
	RecommendedConfig string
}

// =============================================================================
// Private Cluster Interfaces
// =============================================================================

// PrivateClusterManager manages private cluster networking
type PrivateClusterManager interface {
	// SetupPrivateNetwork sets up private networking
	SetupPrivateNetwork(ctx context.Context, config *config.PrivateClusterConfig) error
	// CreateNATGateway creates NAT gateway for egress
	CreateNATGateway(ctx context.Context) error
	// ConfigurePrivateEndpoint configures private API endpoint
	ConfigurePrivateEndpoint(ctx context.Context) error
	// ValidateAccess validates access to private cluster
	ValidateAccess(ctx context.Context, sourceCIDR string) (bool, error)
}

// =============================================================================
// Taint Manager Interfaces
// =============================================================================

// TaintManager manages node taints
type TaintManager interface {
	// ApplyTaints applies taints to a node
	ApplyTaints(ctx context.Context, nodeName string, taints []config.TaintConfig) error
	// RemoveTaints removes taints from a node
	RemoveTaints(ctx context.Context, nodeName string, taintKeys []string) error
	// GetTaints gets current taints on a node
	GetTaints(ctx context.Context, nodeName string) ([]config.TaintConfig, error)
}

// =============================================================================
// Custom Image Interfaces
// =============================================================================

// ImageManager manages custom machine images
type ImageManager interface {
	// GetImage retrieves image details
	GetImage(ctx context.Context, imageID string) (*Image, error)
	// ValidateImage validates image compatibility
	ValidateImage(ctx context.Context, imageID string) error
	// ListImages lists available images
	ListImages(ctx context.Context, filters map[string]string) ([]*Image, error)
}

// Image represents a machine image
type Image struct {
	ID           string
	Name         string
	Provider     string
	Region       string
	Architecture string
	OS           string
	OSVersion    string
	CreatedAt    int64
	Size         int64
	Status       string
	Tags         map[string]string
}

// =============================================================================
// Observer Pattern - Event System
// =============================================================================

// EventEmitter emits provisioning events
type EventEmitter interface {
	// Emit emits an event
	Emit(event Event)
	// Subscribe subscribes to events
	Subscribe(eventType string, handler EventHandler) string
	// Unsubscribe removes a subscription
	Unsubscribe(subscriptionID string)
}

// EventHandler handles provisioning events
type EventHandler func(event Event)

// Event represents a provisioning event
type Event struct {
	Type      string
	Timestamp int64
	Data      interface{}
	Source    string
	Metadata  map[string]string
}
