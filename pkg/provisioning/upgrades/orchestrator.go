// Package upgrades provides cluster upgrade orchestration
package upgrades

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning"
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
)

// =============================================================================
// Upgrade Orchestrator
// =============================================================================

// Orchestrator coordinates cluster upgrades
type Orchestrator struct {
	config        *config.UpgradeConfig
	strategy      Strategy
	strategyReg   *StrategyRegistry
	drainer       provisioning.NodeDrainer
	healthChecker HealthChecker
	eventEmitter  provisioning.EventEmitter

	// State
	currentPlan   *provisioning.UpgradePlan
	currentStatus *provisioning.UpgradeStatus
	isRunning     bool
	pauseChan     chan struct{}
	resumeChan    chan struct{}
	stopChan      chan struct{}

	mu sync.RWMutex
}

// OrchestratorConfig holds orchestrator configuration
type OrchestratorConfig struct {
	UpgradeConfig *config.UpgradeConfig
	Drainer       provisioning.NodeDrainer
	HealthChecker HealthChecker
	EventEmitter  provisioning.EventEmitter
}

// NewOrchestrator creates a new upgrade orchestrator
func NewOrchestrator(cfg *OrchestratorConfig) *Orchestrator {
	registry := NewStrategyRegistry()

	strategyName := "rolling"
	if cfg.UpgradeConfig != nil && cfg.UpgradeConfig.Strategy != "" {
		strategyName = cfg.UpgradeConfig.Strategy
	}

	strategy, _ := registry.Get(strategyName)
	if strategy == nil {
		strategy, _ = registry.Get("rolling")
	}

	return &Orchestrator{
		config:        cfg.UpgradeConfig,
		strategy:      strategy,
		strategyReg:   registry,
		drainer:       cfg.Drainer,
		healthChecker: cfg.HealthChecker,
		eventEmitter:  cfg.EventEmitter,
		pauseChan:     make(chan struct{}),
		resumeChan:    make(chan struct{}),
		stopChan:      make(chan struct{}),
	}
}

// Plan creates an upgrade plan
func (o *Orchestrator) Plan(ctx context.Context, targetVersion string, nodes []*providers.NodeOutput) (*provisioning.UpgradePlan, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.isRunning {
		return nil, fmt.Errorf("an upgrade is already in progress")
	}

	currentVersion := o.getCurrentVersion(nodes)

	// Calculate batch size
	maxUnavailable := 1
	if o.config != nil && o.config.MaxUnavailable > 0 {
		maxUnavailable = o.config.MaxUnavailable
	}

	batchSize := o.strategy.GetBatchSize(len(nodes), maxUnavailable)

	// Create node plans
	nodePlans := make([]provisioning.UpgradeNodePlan, 0, len(nodes))
	batch := 0
	for i, node := range nodes {
		if i > 0 && i%batchSize == 0 {
			batch++
		}
		nodePlans = append(nodePlans, provisioning.UpgradeNodePlan{
			NodeName: node.Name,
			NodeID:   node.Name, // Will be populated with actual ID
			Order:    i,
			Batch:    batch,
			Status:   "pending",
		})
	}

	plan := &provisioning.UpgradePlan{
		ID:             generatePlanID(),
		CurrentVersion: currentVersion,
		TargetVersion:  targetVersion,
		Strategy:       o.strategy.Name(),
		Nodes:          nodePlans,
		CreatedAt:      time.Now().Unix(),
		EstimatedTime:  o.estimateTime(len(nodes), batchSize),
	}

	o.currentPlan = plan
	o.currentStatus = &provisioning.UpgradeStatus{
		PlanID:         plan.ID,
		Phase:          "planned",
		Progress:       0,
		CompletedNodes: make([]string, 0),
		FailedNodes:    make([]string, 0),
	}

	o.emitEvent("upgrade_planned", map[string]interface{}{
		"plan_id":        plan.ID,
		"target_version": targetVersion,
		"node_count":     len(nodes),
		"estimated_time": plan.EstimatedTime,
	})

	return plan, nil
}

// Execute executes an upgrade plan
func (o *Orchestrator) Execute(ctx context.Context, plan *provisioning.UpgradePlan) error {
	o.mu.Lock()
	if o.isRunning {
		o.mu.Unlock()
		return fmt.Errorf("an upgrade is already in progress")
	}
	o.isRunning = true
	o.currentPlan = plan
	o.currentStatus = &provisioning.UpgradeStatus{
		PlanID:          plan.ID,
		Phase:           "executing",
		Progress:        0,
		CompletedNodes:  make([]string, 0),
		FailedNodes:     make([]string, 0),
		StartedAt:       time.Now().Unix(),
		EstimatedFinish: time.Now().Unix() + int64(plan.EstimatedTime),
	}
	o.mu.Unlock()

	defer func() {
		o.mu.Lock()
		o.isRunning = false
		o.mu.Unlock()
	}()

	o.emitEvent("upgrade_started", map[string]interface{}{
		"plan_id": plan.ID,
	})

	// Group nodes by batch
	batches := make(map[int][]provisioning.UpgradeNodePlan)
	for _, node := range plan.Nodes {
		batches[node.Batch] = append(batches[node.Batch], node)
	}

	// Process batches sequentially
	for batchNum := 0; batchNum <= len(batches); batchNum++ {
		nodes, exists := batches[batchNum]
		if !exists {
			continue
		}

		// Check for pause/stop signals
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-o.stopChan:
			o.updateStatus("stopped", "")
			return fmt.Errorf("upgrade stopped by user")
		case <-o.pauseChan:
			o.updateStatus("paused", "")
			o.emitEvent("upgrade_paused", map[string]interface{}{"plan_id": plan.ID})
			<-o.resumeChan
			o.updateStatus("executing", "")
			o.emitEvent("upgrade_resumed", map[string]interface{}{"plan_id": plan.ID})
		default:
		}

		// Process batch
		if err := o.processBatch(ctx, nodes, plan.TargetVersion); err != nil {
			if o.config.PauseOnFailure {
				o.updateStatus("paused_on_failure", err.Error())
				o.emitEvent("upgrade_paused_on_failure", map[string]interface{}{
					"plan_id": plan.ID,
					"error":   err.Error(),
				})
				return err
			}

			if o.config.AutoRollback {
				o.emitEvent("upgrade_auto_rollback_triggered", map[string]interface{}{
					"plan_id": plan.ID,
					"error":   err.Error(),
				})
				if rbErr := o.Rollback(ctx, plan); rbErr != nil {
					return fmt.Errorf("upgrade failed and rollback failed: %v, rollback: %v", err, rbErr)
				}
				return fmt.Errorf("upgrade failed, rolled back: %w", err)
			}

			return err
		}

		// Update progress
		progress := ((batchNum + 1) * 100) / len(batches)
		o.updateProgress(progress)
	}

	o.updateStatus("completed", "")
	o.emitEvent("upgrade_completed", map[string]interface{}{
		"plan_id": plan.ID,
	})

	return nil
}

// processBatch upgrades a batch of nodes
func (o *Orchestrator) processBatch(ctx context.Context, nodes []provisioning.UpgradeNodePlan, targetVersion string) error {
	for _, nodePlan := range nodes {
		o.updateCurrentNode(nodePlan.NodeName)

		// Create a mock node output for the strategy
		node := &providers.NodeOutput{
			Name: nodePlan.NodeName,
		}

		// Prepare node (drain, etc.)
		if err := o.strategy.PrepareNode(ctx, node); err != nil {
			o.addFailedNode(nodePlan.NodeName)
			return fmt.Errorf("failed to prepare node %s: %w", nodePlan.NodeName, err)
		}

		// Upgrade node
		if err := o.strategy.UpgradeNode(ctx, node, targetVersion); err != nil {
			o.addFailedNode(nodePlan.NodeName)
			return fmt.Errorf("failed to upgrade node %s: %w", nodePlan.NodeName, err)
		}

		// Validate node
		if err := o.strategy.ValidateNode(ctx, node); err != nil {
			o.addFailedNode(nodePlan.NodeName)
			return fmt.Errorf("validation failed for node %s: %w", nodePlan.NodeName, err)
		}

		// Wait for health check
		if err := o.waitForNodeHealth(ctx, nodePlan.NodeName); err != nil {
			o.addFailedNode(nodePlan.NodeName)
			return fmt.Errorf("health check failed for node %s: %w", nodePlan.NodeName, err)
		}

		o.addCompletedNode(nodePlan.NodeName)
		o.emitEvent("node_upgraded", map[string]interface{}{
			"node_name":      nodePlan.NodeName,
			"target_version": targetVersion,
		})
	}

	return nil
}

// Rollback rolls back a failed upgrade
func (o *Orchestrator) Rollback(ctx context.Context, plan *provisioning.UpgradePlan) error {
	o.mu.Lock()
	o.currentStatus.Phase = "rolling_back"
	o.mu.Unlock()

	o.emitEvent("upgrade_rollback_started", map[string]interface{}{
		"plan_id": plan.ID,
	})

	// Rollback upgraded nodes in reverse order
	for i := len(o.currentStatus.CompletedNodes) - 1; i >= 0; i-- {
		nodeName := o.currentStatus.CompletedNodes[i]

		node := &providers.NodeOutput{
			Name: nodeName,
		}

		// Prepare for rollback
		if err := o.strategy.PrepareNode(ctx, node); err != nil {
			return fmt.Errorf("failed to prepare node %s for rollback: %w", nodeName, err)
		}

		// Rollback to previous version
		if err := o.strategy.UpgradeNode(ctx, node, plan.CurrentVersion); err != nil {
			return fmt.Errorf("failed to rollback node %s: %w", nodeName, err)
		}

		// Validate
		if err := o.strategy.ValidateNode(ctx, node); err != nil {
			return fmt.Errorf("validation failed for rolled back node %s: %w", nodeName, err)
		}

		o.emitEvent("node_rolled_back", map[string]interface{}{
			"node_name": nodeName,
		})
	}

	o.mu.Lock()
	o.currentStatus.Phase = "rolled_back"
	o.mu.Unlock()

	o.emitEvent("upgrade_rollback_completed", map[string]interface{}{
		"plan_id": plan.ID,
	})

	return nil
}

// Pause pauses an in-progress upgrade
func (o *Orchestrator) Pause() error {
	o.mu.RLock()
	if !o.isRunning {
		o.mu.RUnlock()
		return fmt.Errorf("no upgrade in progress")
	}
	o.mu.RUnlock()

	o.pauseChan <- struct{}{}
	return nil
}

// Resume resumes a paused upgrade
func (o *Orchestrator) Resume() error {
	o.mu.RLock()
	if o.currentStatus.Phase != "paused" && o.currentStatus.Phase != "paused_on_failure" {
		o.mu.RUnlock()
		return fmt.Errorf("upgrade is not paused")
	}
	o.mu.RUnlock()

	o.resumeChan <- struct{}{}
	return nil
}

// Stop stops an in-progress upgrade
func (o *Orchestrator) Stop() error {
	o.mu.RLock()
	if !o.isRunning {
		o.mu.RUnlock()
		return fmt.Errorf("no upgrade in progress")
	}
	o.mu.RUnlock()

	close(o.stopChan)
	o.stopChan = make(chan struct{}) // Reset for future use
	return nil
}

// GetStatus returns current upgrade status
func (o *Orchestrator) GetStatus(ctx context.Context) (*provisioning.UpgradeStatus, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.currentStatus == nil {
		return nil, fmt.Errorf("no upgrade status available")
	}

	// Return a copy
	status := *o.currentStatus
	return &status, nil
}

// SetStrategy changes the upgrade strategy
func (o *Orchestrator) SetStrategy(name string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.isRunning {
		return fmt.Errorf("cannot change strategy while upgrade is in progress")
	}

	strategy, err := o.strategyReg.Get(name)
	if err != nil {
		return err
	}

	o.strategy = strategy
	return nil
}

// Helper methods

func (o *Orchestrator) getCurrentVersion(nodes []*providers.NodeOutput) string {
	// In a real implementation, this would query actual node versions
	return "v1.28.0"
}

func (o *Orchestrator) estimateTime(nodeCount, batchSize int) int {
	// Estimate ~5 minutes per node for drain + upgrade + validation
	nodesPerBatch := batchSize
	batches := (nodeCount + nodesPerBatch - 1) / nodesPerBatch
	timePerBatch := 5 * 60 * nodesPerBatch // 5 minutes per node
	return batches * timePerBatch
}

func (o *Orchestrator) waitForNodeHealth(ctx context.Context, nodeName string) error {
	if o.healthChecker == nil {
		return nil
	}

	interval := 30 * time.Second
	if o.config != nil && o.config.HealthCheckInterval > 0 {
		interval = time.Duration(o.config.HealthCheckInterval) * time.Second
	}

	timeout := 5 * time.Minute
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		healthy, err := o.healthChecker.IsNodeHealthy(ctx, nodeName)
		if err == nil && healthy {
			return nil
		}

		time.Sleep(interval)
	}

	return fmt.Errorf("node %s did not become healthy within timeout", nodeName)
}

func (o *Orchestrator) updateStatus(phase, errorMsg string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.currentStatus.Phase = phase
	o.currentStatus.Error = errorMsg
}

func (o *Orchestrator) updateProgress(progress int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.currentStatus.Progress = progress
}

func (o *Orchestrator) updateCurrentNode(nodeName string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.currentStatus.CurrentNode = nodeName
}

func (o *Orchestrator) addCompletedNode(nodeName string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.currentStatus.CompletedNodes = append(o.currentStatus.CompletedNodes, nodeName)
}

func (o *Orchestrator) addFailedNode(nodeName string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.currentStatus.FailedNodes = append(o.currentStatus.FailedNodes, nodeName)
}

func (o *Orchestrator) emitEvent(eventType string, data map[string]interface{}) {
	if o.eventEmitter != nil {
		o.eventEmitter.Emit(provisioning.Event{
			Type:      eventType,
			Timestamp: time.Now().Unix(),
			Data:      data,
			Source:    "upgrade_orchestrator",
		})
	}
}

func generatePlanID() string {
	return fmt.Sprintf("upgrade-%d", time.Now().UnixNano())
}

// HealthChecker checks node health
type HealthChecker interface {
	IsNodeHealthy(ctx context.Context, nodeName string) (bool, error)
}
