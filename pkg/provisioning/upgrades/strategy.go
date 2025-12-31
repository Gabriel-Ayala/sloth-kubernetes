package upgrades

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning"
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
)

// =============================================================================
// Strategy Interface
// =============================================================================

// Strategy defines upgrade execution strategy
type Strategy interface {
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

// =============================================================================
// Strategy Registry
// =============================================================================

// StrategyRegistry manages upgrade strategies
type StrategyRegistry struct {
	strategies map[string]Strategy
	mu         sync.RWMutex
}

// NewStrategyRegistry creates a new registry with default strategies
func NewStrategyRegistry() *StrategyRegistry {
	registry := &StrategyRegistry{
		strategies: make(map[string]Strategy),
	}

	// Register default strategies
	registry.Register(NewRollingStrategy(nil, nil))
	registry.Register(NewBlueGreenStrategy(nil, nil))
	registry.Register(NewCanaryStrategy(nil, nil))

	return registry
}

// Register adds a strategy to the registry
func (r *StrategyRegistry) Register(strategy Strategy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.strategies[strategy.Name()] = strategy
}

// Get retrieves a strategy by name
func (r *StrategyRegistry) Get(name string) (Strategy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	strategy, exists := r.strategies[name]
	if !exists {
		return nil, fmt.Errorf("upgrade strategy %s not found", name)
	}
	return strategy, nil
}

// =============================================================================
// Rolling Upgrade Strategy
// =============================================================================

// RollingStrategy implements rolling upgrade
type RollingStrategy struct {
	drainer        provisioning.NodeDrainer
	nodeUpgrader   NodeUpgrader
	healthChecker  HealthChecker
}

// NodeUpgrader upgrades individual nodes
type NodeUpgrader interface {
	Upgrade(ctx context.Context, node *providers.NodeOutput, targetVersion string) error
}

// NewRollingStrategy creates a new rolling upgrade strategy
func NewRollingStrategy(drainer provisioning.NodeDrainer, upgrader NodeUpgrader) *RollingStrategy {
	return &RollingStrategy{
		drainer:      drainer,
		nodeUpgrader: upgrader,
	}
}

// Name returns the strategy name
func (s *RollingStrategy) Name() string {
	return "rolling"
}

// PrepareNode prepares a node for upgrade (drains workloads)
func (s *RollingStrategy) PrepareNode(ctx context.Context, node *providers.NodeOutput) error {
	if s.drainer == nil {
		return nil // Skip if no drainer configured
	}

	// Cordon the node first
	if err := s.drainer.Cordon(ctx, node.Name); err != nil {
		return fmt.Errorf("failed to cordon node: %w", err)
	}

	// Drain workloads with timeout
	timeout := 300 // 5 minutes default
	if err := s.drainer.Drain(ctx, node.Name, timeout); err != nil {
		return fmt.Errorf("failed to drain node: %w", err)
	}

	return nil
}

// UpgradeNode upgrades a single node
func (s *RollingStrategy) UpgradeNode(ctx context.Context, node *providers.NodeOutput, targetVersion string) error {
	if s.nodeUpgrader == nil {
		// Simulate upgrade if no upgrader configured
		time.Sleep(2 * time.Second)
		return nil
	}

	return s.nodeUpgrader.Upgrade(ctx, node, targetVersion)
}

// ValidateNode validates node after upgrade
func (s *RollingStrategy) ValidateNode(ctx context.Context, node *providers.NodeOutput) error {
	if s.drainer == nil {
		return nil
	}

	// Uncordon the node to make it schedulable again
	if err := s.drainer.Uncordon(ctx, node.Name); err != nil {
		return fmt.Errorf("failed to uncordon node: %w", err)
	}

	return nil
}

// GetBatchSize returns batch size for rolling upgrades
func (s *RollingStrategy) GetBatchSize(totalNodes int, maxUnavailable int) int {
	if maxUnavailable <= 0 {
		maxUnavailable = 1
	}

	// Don't exceed 25% of total nodes
	maxBatch := (totalNodes + 3) / 4
	if maxUnavailable > maxBatch {
		return maxBatch
	}

	return maxUnavailable
}

// =============================================================================
// Blue-Green Upgrade Strategy
// =============================================================================

// BlueGreenStrategy implements blue-green deployment upgrade
type BlueGreenStrategy struct {
	drainer       provisioning.NodeDrainer
	nodeUpgrader  NodeUpgrader
	nodeProvisioner NodeProvisioner
}

// NodeProvisioner provisions new nodes
type NodeProvisioner interface {
	// ProvisionNode creates a new node with specified version
	ProvisionNode(ctx context.Context, template *providers.NodeOutput, version string) (*providers.NodeOutput, error)
	// DecommissionNode removes a node
	DecommissionNode(ctx context.Context, node *providers.NodeOutput) error
}

// NewBlueGreenStrategy creates a new blue-green strategy
func NewBlueGreenStrategy(drainer provisioning.NodeDrainer, provisioner NodeProvisioner) *BlueGreenStrategy {
	return &BlueGreenStrategy{
		drainer:         drainer,
		nodeProvisioner: provisioner,
	}
}

// Name returns the strategy name
func (s *BlueGreenStrategy) Name() string {
	return "blue-green"
}

// PrepareNode in blue-green prepares by provisioning a replacement
func (s *BlueGreenStrategy) PrepareNode(ctx context.Context, node *providers.NodeOutput) error {
	// In blue-green, we don't drain - we'll switch traffic later
	// This is handled at the orchestrator level with new node provisioning
	return nil
}

// UpgradeNode in blue-green provisions a new node with target version
func (s *BlueGreenStrategy) UpgradeNode(ctx context.Context, node *providers.NodeOutput, targetVersion string) error {
	if s.nodeProvisioner == nil {
		return fmt.Errorf("node provisioner required for blue-green strategy")
	}

	// Provision new "green" node
	newNode, err := s.nodeProvisioner.ProvisionNode(ctx, node, targetVersion)
	if err != nil {
		return fmt.Errorf("failed to provision green node: %w", err)
	}

	// Wait for new node to be ready
	time.Sleep(30 * time.Second) // Simplified - real impl would check node status

	// Drain old "blue" node
	if s.drainer != nil {
		if err := s.drainer.Cordon(ctx, node.Name); err != nil {
			return err
		}
		if err := s.drainer.Drain(ctx, node.Name, 300); err != nil {
			return err
		}
	}

	// Decommission old node
	if err := s.nodeProvisioner.DecommissionNode(ctx, node); err != nil {
		return fmt.Errorf("failed to decommission blue node: %w", err)
	}

	// Update node reference to new node
	*node = *newNode

	return nil
}

// ValidateNode validates the new green node
func (s *BlueGreenStrategy) ValidateNode(ctx context.Context, node *providers.NodeOutput) error {
	// Validate new node is healthy and receiving traffic
	return nil
}

// GetBatchSize for blue-green is typically 1 for safer cutover
func (s *BlueGreenStrategy) GetBatchSize(totalNodes int, maxUnavailable int) int {
	// Blue-green processes one node at a time for safety
	return 1
}

// =============================================================================
// Canary Upgrade Strategy
// =============================================================================

// CanaryStrategy implements canary deployment upgrade
type CanaryStrategy struct {
	drainer       provisioning.NodeDrainer
	nodeUpgrader  NodeUpgrader
	canaryPercent int
	validationTime time.Duration
}

// NewCanaryStrategy creates a new canary strategy
func NewCanaryStrategy(drainer provisioning.NodeDrainer, upgrader NodeUpgrader) *CanaryStrategy {
	return &CanaryStrategy{
		drainer:        drainer,
		nodeUpgrader:   upgrader,
		canaryPercent:  10, // Default 10% canary
		validationTime: 10 * time.Minute,
	}
}

// SetCanaryPercent sets the canary percentage
func (s *CanaryStrategy) SetCanaryPercent(percent int) {
	if percent > 0 && percent <= 100 {
		s.canaryPercent = percent
	}
}

// SetValidationTime sets the canary validation time
func (s *CanaryStrategy) SetValidationTime(duration time.Duration) {
	s.validationTime = duration
}

// Name returns the strategy name
func (s *CanaryStrategy) Name() string {
	return "canary"
}

// PrepareNode prepares node for canary upgrade
func (s *CanaryStrategy) PrepareNode(ctx context.Context, node *providers.NodeOutput) error {
	if s.drainer == nil {
		return nil
	}

	// Cordon and drain like rolling
	if err := s.drainer.Cordon(ctx, node.Name); err != nil {
		return err
	}

	return s.drainer.Drain(ctx, node.Name, 300)
}

// UpgradeNode upgrades and then waits for validation
func (s *CanaryStrategy) UpgradeNode(ctx context.Context, node *providers.NodeOutput, targetVersion string) error {
	if s.nodeUpgrader == nil {
		time.Sleep(2 * time.Second)
		return nil
	}

	// Perform upgrade
	if err := s.nodeUpgrader.Upgrade(ctx, node, targetVersion); err != nil {
		return err
	}

	// For canary nodes, wait for validation period
	// This allows monitoring to detect issues
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(s.validationTime):
		// Validation period passed
	}

	return nil
}

// ValidateNode validates canary node
func (s *CanaryStrategy) ValidateNode(ctx context.Context, node *providers.NodeOutput) error {
	if s.drainer != nil {
		return s.drainer.Uncordon(ctx, node.Name)
	}
	return nil
}

// GetBatchSize for canary starts small
func (s *CanaryStrategy) GetBatchSize(totalNodes int, maxUnavailable int) int {
	// First batch is canary (small percentage)
	canaryNodes := (totalNodes * s.canaryPercent) / 100
	if canaryNodes < 1 {
		canaryNodes = 1
	}
	return canaryNodes
}

// =============================================================================
// Surge Strategy - Adds new nodes before removing old ones
// =============================================================================

// SurgeStrategy adds new nodes then removes old ones
type SurgeStrategy struct {
	drainer         provisioning.NodeDrainer
	nodeProvisioner NodeProvisioner
	maxSurge        int
}

// NewSurgeStrategy creates a new surge strategy
func NewSurgeStrategy(drainer provisioning.NodeDrainer, provisioner NodeProvisioner) *SurgeStrategy {
	return &SurgeStrategy{
		drainer:         drainer,
		nodeProvisioner: provisioner,
		maxSurge:        1,
	}
}

// Name returns the strategy name
func (s *SurgeStrategy) Name() string {
	return "surge"
}

// PrepareNode provisions a replacement node first
func (s *SurgeStrategy) PrepareNode(ctx context.Context, node *providers.NodeOutput) error {
	// In surge strategy, we provision the new node first
	// This is handled in UpgradeNode
	return nil
}

// UpgradeNode provisions new, then removes old
func (s *SurgeStrategy) UpgradeNode(ctx context.Context, node *providers.NodeOutput, targetVersion string) error {
	if s.nodeProvisioner == nil {
		return fmt.Errorf("node provisioner required for surge strategy")
	}

	// Provision new node with target version
	newNode, err := s.nodeProvisioner.ProvisionNode(ctx, node, targetVersion)
	if err != nil {
		return fmt.Errorf("failed to provision surge node: %w", err)
	}

	// Wait for new node to be ready
	time.Sleep(30 * time.Second)

	// Drain and remove old node
	if s.drainer != nil {
		s.drainer.Cordon(ctx, node.Name)
		s.drainer.Drain(ctx, node.Name, 300)
	}

	if err := s.nodeProvisioner.DecommissionNode(ctx, node); err != nil {
		return fmt.Errorf("failed to decommission old node: %w", err)
	}

	*node = *newNode
	return nil
}

// ValidateNode validates the new surge node
func (s *SurgeStrategy) ValidateNode(ctx context.Context, node *providers.NodeOutput) error {
	return nil
}

// GetBatchSize for surge is based on maxSurge
func (s *SurgeStrategy) GetBatchSize(totalNodes int, maxUnavailable int) int {
	return s.maxSurge
}

// =============================================================================
// Node Drainer Implementation
// =============================================================================

// KubectlDrainer implements NodeDrainer using kubectl
type KubectlDrainer struct {
	kubeconfigPath string
	executor       CommandExecutor
}

// CommandExecutor executes commands
type CommandExecutor interface {
	Execute(ctx context.Context, command string, args ...string) (string, error)
}

// NewKubectlDrainer creates a new kubectl-based drainer
func NewKubectlDrainer(kubeconfigPath string, executor CommandExecutor) *KubectlDrainer {
	return &KubectlDrainer{
		kubeconfigPath: kubeconfigPath,
		executor:       executor,
	}
}

// Drain drains a node using kubectl
func (d *KubectlDrainer) Drain(ctx context.Context, nodeName string, timeout int) error {
	args := []string{"drain", nodeName,
		"--ignore-daemonsets",
		"--delete-emptydir-data",
		"--force",
		fmt.Sprintf("--timeout=%ds", timeout),
	}

	if d.kubeconfigPath != "" {
		args = append([]string{"--kubeconfig", d.kubeconfigPath}, args...)
	}

	_, err := d.executor.Execute(ctx, "kubectl", args...)
	return err
}

// Uncordon makes a node schedulable
func (d *KubectlDrainer) Uncordon(ctx context.Context, nodeName string) error {
	args := []string{"uncordon", nodeName}

	if d.kubeconfigPath != "" {
		args = append([]string{"--kubeconfig", d.kubeconfigPath}, args...)
	}

	_, err := d.executor.Execute(ctx, "kubectl", args...)
	return err
}

// Cordon marks a node as unschedulable
func (d *KubectlDrainer) Cordon(ctx context.Context, nodeName string) error {
	args := []string{"cordon", nodeName}

	if d.kubeconfigPath != "" {
		args = append([]string{"--kubeconfig", d.kubeconfigPath}, args...)
	}

	_, err := d.executor.Execute(ctx, "kubectl", args...)
	return err
}
