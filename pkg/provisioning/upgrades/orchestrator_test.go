package upgrades

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mocks
// =============================================================================

// MockNodeDrainer implements provisioning.NodeDrainer for testing
type MockNodeDrainer struct {
	drainError    error
	cordonError   error
	uncordonError error
	drainCalls    []string
	cordonCalls   []string
	uncordonCalls []string
}

func (m *MockNodeDrainer) Drain(ctx context.Context, nodeName string, timeout int) error {
	m.drainCalls = append(m.drainCalls, nodeName)
	return m.drainError
}

func (m *MockNodeDrainer) Cordon(ctx context.Context, nodeName string) error {
	m.cordonCalls = append(m.cordonCalls, nodeName)
	return m.cordonError
}

func (m *MockNodeDrainer) Uncordon(ctx context.Context, nodeName string) error {
	m.uncordonCalls = append(m.uncordonCalls, nodeName)
	return m.uncordonError
}

// MockHealthChecker implements HealthChecker for testing
type MockHealthChecker struct {
	healthy bool
	err     error
	calls   int
}

func (m *MockHealthChecker) IsNodeHealthy(ctx context.Context, nodeName string) (bool, error) {
	m.calls++
	return m.healthy, m.err
}

// MockEventEmitter implements EventEmitter for testing
type MockEventEmitter struct {
	events []provisioning.Event
}

func (m *MockEventEmitter) Emit(event provisioning.Event) {
	m.events = append(m.events, event)
}

func (m *MockEventEmitter) Subscribe(eventType string, handler provisioning.EventHandler) string {
	return "sub-1"
}

func (m *MockEventEmitter) Unsubscribe(subscriptionID string) {}

// MockNodeUpgrader implements NodeUpgrader for testing
type MockNodeUpgrader struct {
	err          error
	upgradeCalls int
}

func (m *MockNodeUpgrader) Upgrade(ctx context.Context, node *providers.NodeOutput, targetVersion string) error {
	m.upgradeCalls++
	return m.err
}

// MockNodeProvisioner implements NodeProvisioner for testing
type MockNodeProvisioner struct {
	provisionError    error
	decommissionError error
	provisionCalls    int
	decommissionCalls int
}

func (m *MockNodeProvisioner) ProvisionNode(ctx context.Context, template *providers.NodeOutput, version string) (*providers.NodeOutput, error) {
	m.provisionCalls++
	if m.provisionError != nil {
		return nil, m.provisionError
	}
	return &providers.NodeOutput{
		Name:     template.Name + "-new",
		Provider: template.Provider,
		Region:   template.Region,
	}, nil
}

func (m *MockNodeProvisioner) DecommissionNode(ctx context.Context, node *providers.NodeOutput) error {
	m.decommissionCalls++
	return m.decommissionError
}

// MockCommandExecutor implements CommandExecutor for testing
type MockCommandExecutor struct {
	output string
	err    error
	calls  [][]string
}

func (m *MockCommandExecutor) Execute(ctx context.Context, command string, args ...string) (string, error) {
	m.calls = append(m.calls, append([]string{command}, args...))
	return m.output, m.err
}

// =============================================================================
// Orchestrator Tests
// =============================================================================

func TestNewOrchestrator(t *testing.T) {
	cfg := &OrchestratorConfig{
		UpgradeConfig: &config.UpgradeConfig{
			Strategy: "rolling",
		},
	}

	orchestrator := NewOrchestrator(cfg)

	assert.NotNil(t, orchestrator)
	assert.NotNil(t, orchestrator.strategy)
	assert.Equal(t, "rolling", orchestrator.strategy.Name())
}

func TestNewOrchestrator_DefaultStrategy(t *testing.T) {
	cfg := &OrchestratorConfig{
		UpgradeConfig: nil,
	}

	orchestrator := NewOrchestrator(cfg)

	assert.NotNil(t, orchestrator)
	assert.Equal(t, "rolling", orchestrator.strategy.Name())
}

func TestNewOrchestrator_WithHealthChecker(t *testing.T) {
	healthChecker := &MockHealthChecker{healthy: true}
	cfg := &OrchestratorConfig{
		HealthChecker: healthChecker,
	}

	orchestrator := NewOrchestrator(cfg)

	assert.NotNil(t, orchestrator)
	assert.Equal(t, healthChecker, orchestrator.healthChecker)
}

func TestOrchestrator_Plan_Success(t *testing.T) {
	eventEmitter := &MockEventEmitter{}
	cfg := &OrchestratorConfig{
		UpgradeConfig: &config.UpgradeConfig{
			MaxUnavailable: 2,
		},
		EventEmitter: eventEmitter,
	}

	orchestrator := NewOrchestrator(cfg)

	nodes := []*providers.NodeOutput{
		{Name: "node-1"},
		{Name: "node-2"},
		{Name: "node-3"},
		{Name: "node-4"},
	}

	plan, err := orchestrator.Plan(context.Background(), "v1.29.0", nodes)
	require.NoError(t, err)

	assert.NotEmpty(t, plan.ID)
	assert.Equal(t, "v1.28.0", plan.CurrentVersion) // Default current version
	assert.Equal(t, "v1.29.0", plan.TargetVersion)
	assert.Equal(t, "rolling", plan.Strategy)
	assert.Len(t, plan.Nodes, 4)
	assert.Greater(t, plan.EstimatedTime, 0)

	// Verify event was emitted
	assert.Len(t, eventEmitter.events, 1)
	assert.Equal(t, "upgrade_planned", eventEmitter.events[0].Type)
}

func TestOrchestrator_Plan_AlreadyRunning(t *testing.T) {
	orchestrator := NewOrchestrator(&OrchestratorConfig{})
	orchestrator.isRunning = true

	nodes := []*providers.NodeOutput{{Name: "node-1"}}

	_, err := orchestrator.Plan(context.Background(), "v1.29.0", nodes)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already in progress")
}

func TestOrchestrator_Plan_BatchAssignment(t *testing.T) {
	cfg := &OrchestratorConfig{
		UpgradeConfig: &config.UpgradeConfig{
			MaxUnavailable: 2,
		},
	}

	orchestrator := NewOrchestrator(cfg)

	nodes := []*providers.NodeOutput{
		{Name: "node-1"},
		{Name: "node-2"},
		{Name: "node-3"},
		{Name: "node-4"},
		{Name: "node-5"},
	}

	plan, err := orchestrator.Plan(context.Background(), "v1.29.0", nodes)
	require.NoError(t, err)

	// Verify batch assignment
	batches := make(map[int]int)
	for _, node := range plan.Nodes {
		batches[node.Batch]++
	}

	// Should have multiple batches
	assert.Greater(t, len(batches), 1)
}

func TestOrchestrator_Execute_Success(t *testing.T) {
	healthChecker := &MockHealthChecker{healthy: true}
	eventEmitter := &MockEventEmitter{}

	cfg := &OrchestratorConfig{
		HealthChecker: healthChecker,
		EventEmitter:  eventEmitter,
	}

	orchestrator := NewOrchestrator(cfg)

	plan := &provisioning.UpgradePlan{
		ID:             "test-plan-1",
		CurrentVersion: "v1.28.0",
		TargetVersion:  "v1.29.0",
		Strategy:       "rolling",
		Nodes: []provisioning.UpgradeNodePlan{
			{NodeName: "node-1", Batch: 0, Status: "pending"},
			{NodeName: "node-2", Batch: 1, Status: "pending"},
		},
	}

	err := orchestrator.Execute(context.Background(), plan)
	require.NoError(t, err)

	status, _ := orchestrator.GetStatus(context.Background())
	assert.Equal(t, "completed", status.Phase)
	assert.Len(t, status.CompletedNodes, 2)
}

func TestOrchestrator_Execute_AlreadyRunning(t *testing.T) {
	orchestrator := NewOrchestrator(&OrchestratorConfig{})
	orchestrator.isRunning = true

	plan := &provisioning.UpgradePlan{ID: "test"}

	err := orchestrator.Execute(context.Background(), plan)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already in progress")
}

func TestOrchestrator_Execute_ContextCanceled(t *testing.T) {
	cfg := &OrchestratorConfig{}
	orchestrator := NewOrchestrator(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	plan := &provisioning.UpgradePlan{
		ID: "test-plan",
		Nodes: []provisioning.UpgradeNodePlan{
			{NodeName: "node-1", Batch: 0},
		},
	}

	err := orchestrator.Execute(ctx, plan)
	assert.Error(t, err)
}

func TestOrchestrator_GetStatus_NoUpgrade(t *testing.T) {
	orchestrator := NewOrchestrator(&OrchestratorConfig{})

	_, err := orchestrator.GetStatus(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no upgrade status available")
}

func TestOrchestrator_SetStrategy_Success(t *testing.T) {
	orchestrator := NewOrchestrator(&OrchestratorConfig{})

	err := orchestrator.SetStrategy("canary")
	require.NoError(t, err)

	assert.Equal(t, "canary", orchestrator.strategy.Name())
}

func TestOrchestrator_SetStrategy_Invalid(t *testing.T) {
	orchestrator := NewOrchestrator(&OrchestratorConfig{})

	err := orchestrator.SetStrategy("invalid-strategy")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestOrchestrator_SetStrategy_WhileRunning(t *testing.T) {
	orchestrator := NewOrchestrator(&OrchestratorConfig{})
	orchestrator.isRunning = true

	err := orchestrator.SetStrategy("canary")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot change strategy")
}

func TestOrchestrator_Pause_NotRunning(t *testing.T) {
	orchestrator := NewOrchestrator(&OrchestratorConfig{})

	err := orchestrator.Pause()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no upgrade in progress")
}

func TestOrchestrator_Resume_NotPaused(t *testing.T) {
	orchestrator := NewOrchestrator(&OrchestratorConfig{})
	orchestrator.currentStatus = &provisioning.UpgradeStatus{Phase: "executing"}

	err := orchestrator.Resume()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not paused")
}

func TestOrchestrator_Stop_NotRunning(t *testing.T) {
	orchestrator := NewOrchestrator(&OrchestratorConfig{})

	err := orchestrator.Stop()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no upgrade in progress")
}

func TestOrchestrator_Rollback(t *testing.T) {
	eventEmitter := &MockEventEmitter{}
	cfg := &OrchestratorConfig{
		EventEmitter: eventEmitter,
	}

	orchestrator := NewOrchestrator(cfg)
	orchestrator.currentStatus = &provisioning.UpgradeStatus{
		CompletedNodes: []string{"node-1", "node-2"},
	}

	plan := &provisioning.UpgradePlan{
		ID:             "test-plan",
		CurrentVersion: "v1.28.0",
		TargetVersion:  "v1.29.0",
	}

	err := orchestrator.Rollback(context.Background(), plan)
	require.NoError(t, err)

	assert.Equal(t, "rolled_back", orchestrator.currentStatus.Phase)

	// Verify rollback events
	hasRollbackStart := false
	hasRollbackComplete := false
	for _, event := range eventEmitter.events {
		if event.Type == "upgrade_rollback_started" {
			hasRollbackStart = true
		}
		if event.Type == "upgrade_rollback_completed" {
			hasRollbackComplete = true
		}
	}
	assert.True(t, hasRollbackStart)
	assert.True(t, hasRollbackComplete)
}

// =============================================================================
// Strategy Registry Tests
// =============================================================================

func TestNewStrategyRegistry(t *testing.T) {
	registry := NewStrategyRegistry()

	assert.NotNil(t, registry)
	assert.NotNil(t, registry.strategies)

	// Should have default strategies
	rolling, err := registry.Get("rolling")
	require.NoError(t, err)
	assert.Equal(t, "rolling", rolling.Name())

	blueGreen, err := registry.Get("blue-green")
	require.NoError(t, err)
	assert.Equal(t, "blue-green", blueGreen.Name())

	canary, err := registry.Get("canary")
	require.NoError(t, err)
	assert.Equal(t, "canary", canary.Name())
}

func TestStrategyRegistry_Register(t *testing.T) {
	registry := NewStrategyRegistry()

	// Create custom strategy using surge
	surgeStrategy := NewSurgeStrategy(nil, nil)
	registry.Register(surgeStrategy)

	strategy, err := registry.Get("surge")
	require.NoError(t, err)
	assert.Equal(t, "surge", strategy.Name())
}

func TestStrategyRegistry_Get_NotFound(t *testing.T) {
	registry := NewStrategyRegistry()

	_, err := registry.Get("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// =============================================================================
// Rolling Strategy Tests
// =============================================================================

func TestRollingStrategy_Name(t *testing.T) {
	strategy := NewRollingStrategy(nil, nil)
	assert.Equal(t, "rolling", strategy.Name())
}

func TestRollingStrategy_PrepareNode_WithDrainer(t *testing.T) {
	drainer := &MockNodeDrainer{}
	strategy := NewRollingStrategy(drainer, nil)

	node := &providers.NodeOutput{Name: "test-node"}
	err := strategy.PrepareNode(context.Background(), node)
	require.NoError(t, err)

	assert.Contains(t, drainer.cordonCalls, "test-node")
	assert.Contains(t, drainer.drainCalls, "test-node")
}

func TestRollingStrategy_PrepareNode_NoDrainer(t *testing.T) {
	strategy := NewRollingStrategy(nil, nil)

	node := &providers.NodeOutput{Name: "test-node"}
	err := strategy.PrepareNode(context.Background(), node)
	require.NoError(t, err)
}

func TestRollingStrategy_PrepareNode_CordonError(t *testing.T) {
	drainer := &MockNodeDrainer{cordonError: errors.New("cordon failed")}
	strategy := NewRollingStrategy(drainer, nil)

	node := &providers.NodeOutput{Name: "test-node"}
	err := strategy.PrepareNode(context.Background(), node)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to cordon")
}

func TestRollingStrategy_PrepareNode_DrainError(t *testing.T) {
	drainer := &MockNodeDrainer{drainError: errors.New("drain failed")}
	strategy := NewRollingStrategy(drainer, nil)

	node := &providers.NodeOutput{Name: "test-node"}
	err := strategy.PrepareNode(context.Background(), node)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to drain")
}

func TestRollingStrategy_UpgradeNode_WithUpgrader(t *testing.T) {
	upgrader := &MockNodeUpgrader{}
	strategy := NewRollingStrategy(nil, upgrader)

	node := &providers.NodeOutput{Name: "test-node"}
	err := strategy.UpgradeNode(context.Background(), node, "v1.29.0")
	require.NoError(t, err)

	assert.Equal(t, 1, upgrader.upgradeCalls)
}

func TestRollingStrategy_UpgradeNode_NoUpgrader(t *testing.T) {
	strategy := NewRollingStrategy(nil, nil)

	node := &providers.NodeOutput{Name: "test-node"}
	err := strategy.UpgradeNode(context.Background(), node, "v1.29.0")
	require.NoError(t, err) // Should simulate upgrade
}

func TestRollingStrategy_ValidateNode_WithDrainer(t *testing.T) {
	drainer := &MockNodeDrainer{}
	strategy := NewRollingStrategy(drainer, nil)

	node := &providers.NodeOutput{Name: "test-node"}
	err := strategy.ValidateNode(context.Background(), node)
	require.NoError(t, err)

	assert.Contains(t, drainer.uncordonCalls, "test-node")
}

func TestRollingStrategy_GetBatchSize(t *testing.T) {
	strategy := NewRollingStrategy(nil, nil)

	tests := []struct {
		totalNodes     int
		maxUnavailable int
		expected       int
	}{
		{10, 2, 2},    // Normal case
		{10, 0, 1},    // Zero max unavailable defaults to 1
		{10, -1, 1},   // Negative max unavailable defaults to 1
		{10, 5, 3},    // Capped at 25% of total
		{4, 2, 1},     // 25% of 4 = 1
		{100, 50, 25}, // Capped at 25% of total (103/4=25)
	}

	for _, tt := range tests {
		result := strategy.GetBatchSize(tt.totalNodes, tt.maxUnavailable)
		assert.Equal(t, tt.expected, result)
	}
}

// =============================================================================
// Blue-Green Strategy Tests
// =============================================================================

func TestBlueGreenStrategy_Name(t *testing.T) {
	strategy := NewBlueGreenStrategy(nil, nil)
	assert.Equal(t, "blue-green", strategy.Name())
}

func TestBlueGreenStrategy_PrepareNode(t *testing.T) {
	strategy := NewBlueGreenStrategy(nil, nil)

	node := &providers.NodeOutput{Name: "test-node"}
	err := strategy.PrepareNode(context.Background(), node)
	require.NoError(t, err) // Blue-green doesn't drain in prepare
}

func TestBlueGreenStrategy_UpgradeNode_NoProvisioner(t *testing.T) {
	strategy := NewBlueGreenStrategy(nil, nil)

	node := &providers.NodeOutput{Name: "test-node"}
	err := strategy.UpgradeNode(context.Background(), node, "v1.29.0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "node provisioner required")
}

func TestBlueGreenStrategy_GetBatchSize(t *testing.T) {
	strategy := NewBlueGreenStrategy(nil, nil)

	// Blue-green always returns 1 for safety
	assert.Equal(t, 1, strategy.GetBatchSize(10, 5))
	assert.Equal(t, 1, strategy.GetBatchSize(100, 25))
}

// =============================================================================
// Canary Strategy Tests
// =============================================================================

func TestCanaryStrategy_Name(t *testing.T) {
	strategy := NewCanaryStrategy(nil, nil)
	assert.Equal(t, "canary", strategy.Name())
}

func TestCanaryStrategy_SetCanaryPercent(t *testing.T) {
	strategy := NewCanaryStrategy(nil, nil)

	strategy.SetCanaryPercent(25)
	assert.Equal(t, 25, strategy.canaryPercent)

	// Invalid values should not change
	strategy.SetCanaryPercent(0)
	assert.Equal(t, 25, strategy.canaryPercent)

	strategy.SetCanaryPercent(101)
	assert.Equal(t, 25, strategy.canaryPercent)
}

func TestCanaryStrategy_SetValidationTime(t *testing.T) {
	strategy := NewCanaryStrategy(nil, nil)

	strategy.SetValidationTime(5 * time.Minute)
	assert.Equal(t, 5*time.Minute, strategy.validationTime)
}

func TestCanaryStrategy_PrepareNode_WithDrainer(t *testing.T) {
	drainer := &MockNodeDrainer{}
	strategy := NewCanaryStrategy(drainer, nil)

	node := &providers.NodeOutput{Name: "test-node"}
	err := strategy.PrepareNode(context.Background(), node)
	require.NoError(t, err)

	assert.Contains(t, drainer.cordonCalls, "test-node")
	assert.Contains(t, drainer.drainCalls, "test-node")
}

func TestCanaryStrategy_GetBatchSize(t *testing.T) {
	strategy := NewCanaryStrategy(nil, nil)
	strategy.SetCanaryPercent(10)

	tests := []struct {
		totalNodes int
		expected   int
	}{
		{100, 10}, // 10% of 100
		{50, 5},   // 10% of 50
		{5, 1},    // Minimum 1
		{1, 1},    // Minimum 1
	}

	for _, tt := range tests {
		result := strategy.GetBatchSize(tt.totalNodes, 0)
		assert.Equal(t, tt.expected, result)
	}
}

// =============================================================================
// Surge Strategy Tests
// =============================================================================

func TestSurgeStrategy_Name(t *testing.T) {
	strategy := NewSurgeStrategy(nil, nil)
	assert.Equal(t, "surge", strategy.Name())
}

func TestSurgeStrategy_PrepareNode(t *testing.T) {
	strategy := NewSurgeStrategy(nil, nil)

	node := &providers.NodeOutput{Name: "test-node"}
	err := strategy.PrepareNode(context.Background(), node)
	require.NoError(t, err)
}

func TestSurgeStrategy_UpgradeNode_NoProvisioner(t *testing.T) {
	strategy := NewSurgeStrategy(nil, nil)

	node := &providers.NodeOutput{Name: "test-node"}
	err := strategy.UpgradeNode(context.Background(), node, "v1.29.0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "node provisioner required")
}

func TestSurgeStrategy_GetBatchSize(t *testing.T) {
	strategy := NewSurgeStrategy(nil, nil)

	// Default maxSurge is 1
	assert.Equal(t, 1, strategy.GetBatchSize(10, 5))
}

// =============================================================================
// KubectlDrainer Tests
// =============================================================================

func TestNewKubectlDrainer(t *testing.T) {
	executor := &MockCommandExecutor{}
	drainer := NewKubectlDrainer("/path/to/kubeconfig", executor)

	assert.NotNil(t, drainer)
	assert.Equal(t, "/path/to/kubeconfig", drainer.kubeconfigPath)
}

func TestKubectlDrainer_Drain(t *testing.T) {
	executor := &MockCommandExecutor{output: "node drained"}
	drainer := NewKubectlDrainer("/path/to/kubeconfig", executor)

	err := drainer.Drain(context.Background(), "test-node", 300)
	require.NoError(t, err)

	assert.Len(t, executor.calls, 1)
	call := executor.calls[0]
	assert.Equal(t, "kubectl", call[0])
	assert.Contains(t, call, "--kubeconfig")
	assert.Contains(t, call, "drain")
	assert.Contains(t, call, "test-node")
	assert.Contains(t, call, "--ignore-daemonsets")
	assert.Contains(t, call, "--delete-emptydir-data")
	assert.Contains(t, call, "--force")
	assert.Contains(t, call, "--timeout=300s")
}

func TestKubectlDrainer_Drain_NoKubeconfig(t *testing.T) {
	executor := &MockCommandExecutor{output: "node drained"}
	drainer := NewKubectlDrainer("", executor)

	err := drainer.Drain(context.Background(), "test-node", 300)
	require.NoError(t, err)

	call := executor.calls[0]
	assert.NotContains(t, call, "--kubeconfig")
}

func TestKubectlDrainer_Drain_Error(t *testing.T) {
	executor := &MockCommandExecutor{err: errors.New("drain failed")}
	drainer := NewKubectlDrainer("", executor)

	err := drainer.Drain(context.Background(), "test-node", 300)
	assert.Error(t, err)
}

func TestKubectlDrainer_Cordon(t *testing.T) {
	executor := &MockCommandExecutor{output: "node cordoned"}
	drainer := NewKubectlDrainer("/path/to/kubeconfig", executor)

	err := drainer.Cordon(context.Background(), "test-node")
	require.NoError(t, err)

	call := executor.calls[0]
	assert.Equal(t, "kubectl", call[0])
	assert.Contains(t, call, "cordon")
	assert.Contains(t, call, "test-node")
}

func TestKubectlDrainer_Uncordon(t *testing.T) {
	executor := &MockCommandExecutor{output: "node uncordoned"}
	drainer := NewKubectlDrainer("/path/to/kubeconfig", executor)

	err := drainer.Uncordon(context.Background(), "test-node")
	require.NoError(t, err)

	call := executor.calls[0]
	assert.Equal(t, "kubectl", call[0])
	assert.Contains(t, call, "uncordon")
	assert.Contains(t, call, "test-node")
}

// =============================================================================
// Integration-like Tests
// =============================================================================

func TestOrchestrator_FullUpgradeWorkflow(t *testing.T) {
	drainer := &MockNodeDrainer{}
	healthChecker := &MockHealthChecker{healthy: true}
	eventEmitter := &MockEventEmitter{}

	cfg := &OrchestratorConfig{
		UpgradeConfig: &config.UpgradeConfig{
			Strategy:       "rolling",
			MaxUnavailable: 1,
		},
		Drainer:       drainer,
		HealthChecker: healthChecker,
		EventEmitter:  eventEmitter,
	}

	orchestrator := NewOrchestrator(cfg)

	// Step 1: Plan
	nodes := []*providers.NodeOutput{
		{Name: "master-1"},
		{Name: "worker-1"},
		{Name: "worker-2"},
	}

	plan, err := orchestrator.Plan(context.Background(), "v1.29.0", nodes)
	require.NoError(t, err)

	assert.NotEmpty(t, plan.ID)
	assert.Len(t, plan.Nodes, 3)

	// Step 2: Execute
	err = orchestrator.Execute(context.Background(), plan)
	require.NoError(t, err)

	// Step 3: Verify status
	status, err := orchestrator.GetStatus(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "completed", status.Phase)
	assert.Len(t, status.CompletedNodes, 3)
	assert.Empty(t, status.FailedNodes)

	// Verify events
	eventTypes := make(map[string]int)
	for _, event := range eventEmitter.events {
		eventTypes[event.Type]++
	}
	assert.Greater(t, eventTypes["upgrade_planned"], 0)
	assert.Greater(t, eventTypes["upgrade_started"], 0)
	assert.Greater(t, eventTypes["upgrade_completed"], 0)
	assert.Greater(t, eventTypes["node_upgraded"], 0)
}

func TestOrchestrator_MultipleStrategies(t *testing.T) {
	orchestrator := NewOrchestrator(&OrchestratorConfig{})

	strategies := []string{"rolling", "blue-green", "canary"}

	for _, strategyName := range strategies {
		err := orchestrator.SetStrategy(strategyName)
		require.NoError(t, err)
		assert.Equal(t, strategyName, orchestrator.strategy.Name())
	}
}
