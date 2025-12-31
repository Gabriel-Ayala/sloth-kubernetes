package hooks

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mocks
// =============================================================================

// MockExecutor implements Executor for testing
type MockExecutor struct {
	executeError   error
	executeCalls   int
	lastAction     *config.HookAction
	lastData       interface{}
}

func (m *MockExecutor) Execute(ctx context.Context, action *config.HookAction, data interface{}) error {
	m.executeCalls++
	m.lastAction = action
	m.lastData = data
	return m.executeError
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

// =============================================================================
// Engine Tests
// =============================================================================

func TestNewEngine(t *testing.T) {
	engine := NewEngine(&EngineConfig{})

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.hooks)
	assert.NotNil(t, engine.executors)

	// Should have default executors
	assert.Contains(t, engine.executors, "script")
	assert.Contains(t, engine.executors, "kubectl")
	assert.Contains(t, engine.executors, "http")
}

func TestNewEngine_WithEventEmitter(t *testing.T) {
	eventEmitter := &MockEventEmitter{}
	engine := NewEngine(&EngineConfig{
		EventEmitter: eventEmitter,
	})

	assert.Equal(t, eventEmitter, engine.eventEmitter)
}

func TestEngine_RegisterExecutor(t *testing.T) {
	engine := NewEngine(&EngineConfig{})
	mockExecutor := &MockExecutor{}

	engine.RegisterExecutor("custom", mockExecutor)

	assert.Contains(t, engine.executors, "custom")
}

func TestEngine_RegisterHook(t *testing.T) {
	engine := NewEngine(&EngineConfig{})

	action := &config.HookAction{
		Type:    "script",
		Script:  "echo hello",
		Timeout: 30,
	}

	hookID := engine.RegisterHook(provisioning.HookEventPostNodeCreate, action, 0)

	assert.NotEmpty(t, hookID)

	hooks := engine.GetHooks(provisioning.HookEventPostNodeCreate)
	assert.Len(t, hooks, 1)
	assert.Equal(t, hookID, hooks[0].ID)
	assert.Equal(t, action, hooks[0].Hook)
}

func TestEngine_RegisterHook_Priority(t *testing.T) {
	engine := NewEngine(&EngineConfig{})

	// Register hooks with different priorities (lower = higher priority)
	action1 := &config.HookAction{Script: "script1"}
	action2 := &config.HookAction{Script: "script2"}
	action3 := &config.HookAction{Script: "script3"}

	engine.RegisterHook(provisioning.HookEventPostNodeCreate, action1, 10) // Low priority
	engine.RegisterHook(provisioning.HookEventPostNodeCreate, action2, 0)  // High priority
	engine.RegisterHook(provisioning.HookEventPostNodeCreate, action3, 5)  // Medium priority

	hooks := engine.GetHooks(provisioning.HookEventPostNodeCreate)
	assert.Len(t, hooks, 3)

	// Verify sorted by priority (bubble sort ascending)
	assert.Equal(t, "script2", hooks[0].Hook.Script) // Priority 0
}

func TestEngine_RegisterHooksFromConfig(t *testing.T) {
	engine := NewEngine(&EngineConfig{})

	hooksConfig := &config.HooksConfig{
		PostNodeCreate: []config.HookAction{
			{Type: "script", Script: "node-created.sh"},
		},
		PreClusterDestroy: []config.HookAction{
			{Type: "script", Script: "pre-destroy.sh"},
		},
		PostClusterReady: []config.HookAction{
			{Type: "kubectl", Command: "get nodes"},
		},
		PreNodeDelete: []config.HookAction{
			{Type: "script", Script: "pre-delete.sh"},
		},
		PostUpgrade: []config.HookAction{
			{Type: "http", URL: "https://webhook.example.com"},
		},
	}

	engine.RegisterHooksFromConfig(hooksConfig)

	assert.Len(t, engine.GetHooks(provisioning.HookEventPostNodeCreate), 1)
	assert.Len(t, engine.GetHooks(provisioning.HookEventPreClusterDestroy), 1)
	assert.Len(t, engine.GetHooks(provisioning.HookEventPostClusterReady), 1)
	assert.Len(t, engine.GetHooks(provisioning.HookEventPreNodeDelete), 1)
	assert.Len(t, engine.GetHooks(provisioning.HookEventPostUpgrade), 1)
}

func TestEngine_RegisterHooksFromConfig_Nil(t *testing.T) {
	engine := NewEngine(&EngineConfig{})

	// Should not panic
	engine.RegisterHooksFromConfig(nil)
}

func TestEngine_UnregisterHook_Success(t *testing.T) {
	engine := NewEngine(&EngineConfig{})

	action := &config.HookAction{Script: "test.sh"}
	hookID := engine.RegisterHook(provisioning.HookEventPostNodeCreate, action, 0)

	result := engine.UnregisterHook(provisioning.HookEventPostNodeCreate, hookID)

	assert.True(t, result)
	assert.Empty(t, engine.GetHooks(provisioning.HookEventPostNodeCreate))
}

func TestEngine_UnregisterHook_NotFound(t *testing.T) {
	engine := NewEngine(&EngineConfig{})

	result := engine.UnregisterHook(provisioning.HookEventPostNodeCreate, "nonexistent")

	assert.False(t, result)
}

func TestEngine_TriggerHooks_NoHooks(t *testing.T) {
	engine := NewEngine(&EngineConfig{})

	err := engine.TriggerHooks(context.Background(), provisioning.HookEventPostNodeCreate, nil)

	require.NoError(t, err)
}

func TestEngine_TriggerHooks_Success(t *testing.T) {
	eventEmitter := &MockEventEmitter{}
	engine := NewEngine(&EngineConfig{EventEmitter: eventEmitter})

	mockExecutor := &MockExecutor{}
	engine.RegisterExecutor("script", mockExecutor)

	action := &config.HookAction{Type: "script", Script: "test.sh"}
	engine.RegisterHook(provisioning.HookEventPostNodeCreate, action, 0)

	data := map[string]interface{}{"node": "test-node"}
	err := engine.TriggerHooks(context.Background(), provisioning.HookEventPostNodeCreate, data)

	require.NoError(t, err)
	assert.Equal(t, 1, mockExecutor.executeCalls)
	assert.Equal(t, action, mockExecutor.lastAction)

	// Verify events emitted
	eventTypes := make(map[string]int)
	for _, event := range eventEmitter.events {
		eventTypes[event.Type]++
	}
	assert.Equal(t, 1, eventTypes["hooks_triggered"])
	assert.Equal(t, 1, eventTypes["hook_completed"])
}

func TestEngine_TriggerHooks_ExecutorError(t *testing.T) {
	eventEmitter := &MockEventEmitter{}
	engine := NewEngine(&EngineConfig{EventEmitter: eventEmitter})

	mockExecutor := &MockExecutor{executeError: errors.New("execution failed")}
	engine.RegisterExecutor("script", mockExecutor)

	action := &config.HookAction{Type: "script", Script: "test.sh"}
	engine.RegisterHook(provisioning.HookEventPostNodeCreate, action, 0)

	err := engine.TriggerHooks(context.Background(), provisioning.HookEventPostNodeCreate, nil)

	// Trigger continues even on error
	require.NoError(t, err)

	// Verify failure event
	hasFailure := false
	for _, event := range eventEmitter.events {
		if event.Type == "hook_failed" {
			hasFailure = true
		}
	}
	assert.True(t, hasFailure)
}

func TestEngine_TriggerHooks_MultipleHooks(t *testing.T) {
	engine := NewEngine(&EngineConfig{})

	mockExecutor := &MockExecutor{}
	engine.RegisterExecutor("script", mockExecutor)

	engine.RegisterHook(provisioning.HookEventPostNodeCreate, &config.HookAction{Type: "script", Script: "1.sh"}, 0)
	engine.RegisterHook(provisioning.HookEventPostNodeCreate, &config.HookAction{Type: "script", Script: "2.sh"}, 0)
	engine.RegisterHook(provisioning.HookEventPostNodeCreate, &config.HookAction{Type: "script", Script: "3.sh"}, 0)

	err := engine.TriggerHooks(context.Background(), provisioning.HookEventPostNodeCreate, nil)

	require.NoError(t, err)
	assert.Equal(t, 3, mockExecutor.executeCalls)
}

func TestEngine_TriggerHooks_UnknownExecutor(t *testing.T) {
	eventEmitter := &MockEventEmitter{}
	engine := NewEngine(&EngineConfig{EventEmitter: eventEmitter})

	action := &config.HookAction{Type: "unknown_type", Script: "test.sh"}
	engine.RegisterHook(provisioning.HookEventPostNodeCreate, action, 0)

	err := engine.TriggerHooks(context.Background(), provisioning.HookEventPostNodeCreate, nil)

	// Should continue despite unknown executor
	require.NoError(t, err)

	// Verify failure event
	hasFailure := false
	for _, event := range eventEmitter.events {
		if event.Type == "hook_failed" {
			hasFailure = true
		}
	}
	assert.True(t, hasFailure)
}

func TestEngine_TriggerHooks_InferType_Script(t *testing.T) {
	engine := NewEngine(&EngineConfig{})

	mockExecutor := &MockExecutor{}
	engine.RegisterExecutor("script", mockExecutor)

	// Action without explicit type but with Script
	action := &config.HookAction{Script: "test.sh"}
	engine.RegisterHook(provisioning.HookEventPostNodeCreate, action, 0)

	err := engine.TriggerHooks(context.Background(), provisioning.HookEventPostNodeCreate, nil)

	require.NoError(t, err)
	assert.Equal(t, 1, mockExecutor.executeCalls)
}

func TestEngine_TriggerHooks_InferType_HTTP(t *testing.T) {
	engine := NewEngine(&EngineConfig{})

	mockExecutor := &MockExecutor{}
	engine.RegisterExecutor("http", mockExecutor)

	// Action without explicit type but with URL
	action := &config.HookAction{URL: "https://example.com/webhook"}
	engine.RegisterHook(provisioning.HookEventPostNodeCreate, action, 0)

	err := engine.TriggerHooks(context.Background(), provisioning.HookEventPostNodeCreate, nil)

	require.NoError(t, err)
	assert.Equal(t, 1, mockExecutor.executeCalls)
}

func TestEngine_TriggerHooks_InferType_Kubectl(t *testing.T) {
	engine := NewEngine(&EngineConfig{})

	mockExecutor := &MockExecutor{}
	engine.RegisterExecutor("kubectl", mockExecutor)

	// Action with kubectl command
	action := &config.HookAction{Command: "kubectl get nodes"}
	engine.RegisterHook(provisioning.HookEventPostNodeCreate, action, 0)

	err := engine.TriggerHooks(context.Background(), provisioning.HookEventPostNodeCreate, nil)

	require.NoError(t, err)
	assert.Equal(t, 1, mockExecutor.executeCalls)
}

func TestEngine_GetHooks_Empty(t *testing.T) {
	engine := NewEngine(&EngineConfig{})

	hooks := engine.GetHooks(provisioning.HookEventPostNodeCreate)

	assert.Empty(t, hooks)
}

func TestEngine_GetHooks_ReturnsCopy(t *testing.T) {
	engine := NewEngine(&EngineConfig{})

	action := &config.HookAction{Script: "test.sh"}
	engine.RegisterHook(provisioning.HookEventPostNodeCreate, action, 0)

	hooks1 := engine.GetHooks(provisioning.HookEventPostNodeCreate)
	hooks2 := engine.GetHooks(provisioning.HookEventPostNodeCreate)

	// Modify one slice
	hooks1[0].Priority = 999

	// Other should be unaffected
	assert.NotEqual(t, hooks1[0].Priority, hooks2[0].Priority)
}

// =============================================================================
// ScriptExecutor Tests
// =============================================================================

func TestNewScriptExecutor(t *testing.T) {
	executor := NewScriptExecutor()

	assert.NotNil(t, executor)
	assert.Equal(t, "/bin/bash", executor.shell)
}

func TestScriptExecutor_Execute_NoScript(t *testing.T) {
	executor := NewScriptExecutor()

	action := &config.HookAction{}
	err := executor.Execute(context.Background(), action, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no script or command specified")
}

// Note: Full ScriptExecutor tests would require actual command execution
// In production, you'd use an interface for exec.Command to allow mocking

// =============================================================================
// KubectlExecutor Tests
// =============================================================================

func TestNewKubectlExecutor(t *testing.T) {
	executor := NewKubectlExecutor("/path/to/kubeconfig")

	assert.NotNil(t, executor)
	assert.Equal(t, "/path/to/kubeconfig", executor.kubeconfigPath)
}

func TestKubectlExecutor_Execute_NoCommand(t *testing.T) {
	executor := NewKubectlExecutor("")

	action := &config.HookAction{}
	err := executor.Execute(context.Background(), action, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no kubectl command specified")
}

// =============================================================================
// HTTPExecutor Tests
// =============================================================================

func TestNewHTTPExecutor(t *testing.T) {
	executor := NewHTTPExecutor()

	assert.NotNil(t, executor)
	assert.NotNil(t, executor.client)
}

func TestHTTPExecutor_Execute_NoURL(t *testing.T) {
	executor := NewHTTPExecutor()

	action := &config.HookAction{}
	err := executor.Execute(context.Background(), action, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no URL specified")
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestSplitCommand_Simple(t *testing.T) {
	cmd := "get nodes -o wide"
	result := splitCommand(cmd)

	assert.Equal(t, []string{"get", "nodes", "-o", "wide"}, result)
}

func TestSplitCommand_WithQuotes(t *testing.T) {
	cmd := `apply -f "file with spaces.yaml"`
	result := splitCommand(cmd)

	assert.Equal(t, []string{"apply", "-f", "file with spaces.yaml"}, result)
}

func TestSplitCommand_WithSingleQuotes(t *testing.T) {
	cmd := `patch node test -p '{"metadata":{"labels":{"key":"value"}}}'`
	result := splitCommand(cmd)

	assert.Len(t, result, 5)
	assert.Equal(t, "patch", result[0])
	assert.Equal(t, `{"metadata":{"labels":{"key":"value"}}}`, result[4])
}

func TestSplitCommand_Empty(t *testing.T) {
	cmd := ""
	result := splitCommand(cmd)

	assert.Empty(t, result)
}

func TestSplitCommand_MultipleSpaces(t *testing.T) {
	cmd := "get    nodes"
	result := splitCommand(cmd)

	assert.Equal(t, []string{"get", "nodes"}, result)
}

func TestIsKubectlCommand(t *testing.T) {
	tests := []struct {
		cmd      string
		expected bool
	}{
		{"kubectl get nodes", true},
		{"apply -f file.yaml", true},
		{"get pods", true},
		{"echo hello", false},
		{"bash script.sh", false},
		{"", false},
		{"kub", false},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			result := isKubectlCommand(tt.cmd)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// Predefined Hooks Tests
// =============================================================================

func TestGetPredefinedHooks(t *testing.T) {
	hooks := GetPredefinedHooks()

	assert.NotEmpty(t, hooks)
	assert.GreaterOrEqual(t, len(hooks), 5)

	// Verify expected templates exist
	hookNames := make(map[string]bool)
	for _, hook := range hooks {
		hookNames[hook.Name] = true
		assert.NotEmpty(t, hook.Name)
		assert.NotEmpty(t, hook.Description)
	}

	assert.True(t, hookNames["install-monitoring-agent"])
	assert.True(t, hookNames["configure-node-security"])
	assert.True(t, hookNames["notify-slack-ready"])
	assert.True(t, hookNames["backup-before-destroy"])
	assert.True(t, hookNames["apply-essential-manifests"])
}

func TestGetPredefinedHooks_ValidActions(t *testing.T) {
	hooks := GetPredefinedHooks()

	for _, hook := range hooks {
		t.Run(hook.Name, func(t *testing.T) {
			// Each hook should have a valid action type
			assert.NotEmpty(t, hook.Action.Type)

			// Type-specific validation
			switch hook.Action.Type {
			case "script":
				assert.NotEmpty(t, hook.Action.Script)
			case "kubectl":
				assert.NotEmpty(t, hook.Action.Command)
			case "http":
				assert.NotEmpty(t, hook.Action.URL)
			}
		})
	}
}

func TestGetPredefinedHooks_Events(t *testing.T) {
	hooks := GetPredefinedHooks()

	events := make(map[provisioning.HookEvent]int)
	for _, hook := range hooks {
		events[hook.Event]++
	}

	// Should have hooks for multiple events
	assert.Greater(t, len(events), 1)
}

// =============================================================================
// Integration-like Tests
// =============================================================================

func TestEngine_FullWorkflow(t *testing.T) {
	eventEmitter := &MockEventEmitter{}
	engine := NewEngine(&EngineConfig{EventEmitter: eventEmitter})

	// Replace executors with mocks
	scriptExecutor := &MockExecutor{}
	kubectlExecutor := &MockExecutor{}
	httpExecutor := &MockExecutor{}

	engine.RegisterExecutor("script", scriptExecutor)
	engine.RegisterExecutor("kubectl", kubectlExecutor)
	engine.RegisterExecutor("http", httpExecutor)

	// Register hooks from config
	hooksConfig := &config.HooksConfig{
		PostNodeCreate: []config.HookAction{
			{Type: "script", Script: "install-agent.sh"},
		},
		PostClusterReady: []config.HookAction{
			{Type: "kubectl", Command: "apply -f manifests/"},
			{Type: "http", URL: "https://webhook.example.com/ready"},
		},
	}
	engine.RegisterHooksFromConfig(hooksConfig)

	ctx := context.Background()

	// Trigger post-node-create
	err := engine.TriggerHooks(ctx, provisioning.HookEventPostNodeCreate, map[string]interface{}{
		"node_name": "worker-1",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, scriptExecutor.executeCalls)

	// Trigger post-cluster-ready
	err = engine.TriggerHooks(ctx, provisioning.HookEventPostClusterReady, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, kubectlExecutor.executeCalls)
	assert.Equal(t, 1, httpExecutor.executeCalls)

	// Verify events
	triggerCount := 0
	completeCount := 0
	for _, event := range eventEmitter.events {
		if event.Type == "hooks_triggered" {
			triggerCount++
		}
		if event.Type == "hook_completed" {
			completeCount++
		}
	}
	assert.Equal(t, 2, triggerCount)
	assert.Equal(t, 3, completeCount) // 1 + 2 hooks
}

func TestEngine_HookRetry(t *testing.T) {
	engine := NewEngine(&EngineConfig{})

	// Create executor that fails first 2 times then succeeds
	callCount := 0
	mockExecutor := &MockExecutor{
		executeError: nil,
	}

	// Custom executor that tracks retries
	customExecutor := &retryCountingExecutor{
		failUntil: 2,
	}
	engine.RegisterExecutor("script", customExecutor)

	action := &config.HookAction{
		Type:       "script",
		Script:     "test.sh",
		RetryCount: 3,
	}
	engine.RegisterHook(provisioning.HookEventPostNodeCreate, action, 0)

	err := engine.TriggerHooks(context.Background(), provisioning.HookEventPostNodeCreate, nil)

	require.NoError(t, err)
	assert.Equal(t, 3, customExecutor.calls) // Should have tried 3 times

	_ = callCount
	_ = mockExecutor
}

// retryCountingExecutor is a helper for testing retries
type retryCountingExecutor struct {
	calls     int
	failUntil int
}

func (e *retryCountingExecutor) Execute(ctx context.Context, action *config.HookAction, data interface{}) error {
	e.calls++
	if e.calls <= e.failUntil {
		return errors.New("simulated failure")
	}
	return nil
}

func TestEngine_Timeout(t *testing.T) {
	engine := NewEngine(&EngineConfig{})

	// Create executor that blocks
	blockingExecutor := &blockingExecutor{
		blockDuration: 5 * time.Second,
	}
	engine.RegisterExecutor("script", blockingExecutor)

	action := &config.HookAction{
		Type:    "script",
		Script:  "test.sh",
		Timeout: 1, // 1 second timeout
	}
	engine.RegisterHook(provisioning.HookEventPostNodeCreate, action, 0)

	start := time.Now()
	err := engine.TriggerHooks(context.Background(), provisioning.HookEventPostNodeCreate, nil)
	duration := time.Since(start)

	// Should have timed out
	require.NoError(t, err) // TriggerHooks continues even on failure

	// Should have taken roughly 1 second (the timeout), not 5 seconds
	assert.Less(t, duration, 3*time.Second)
}

// blockingExecutor blocks for testing timeouts
type blockingExecutor struct {
	blockDuration time.Duration
}

func (e *blockingExecutor) Execute(ctx context.Context, action *config.HookAction, data interface{}) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(e.blockDuration):
		return nil
	}
}

func TestEngine_DifferentEvents(t *testing.T) {
	engine := NewEngine(&EngineConfig{})

	mockExecutor := &MockExecutor{}
	engine.RegisterExecutor("script", mockExecutor)

	// Register hooks for different events
	events := []provisioning.HookEvent{
		provisioning.HookEventPostNodeCreate,
		provisioning.HookEventPreNodeDelete,
		provisioning.HookEventPostClusterReady,
		provisioning.HookEventPreClusterDestroy,
		provisioning.HookEventPostUpgrade,
	}

	for _, event := range events {
		engine.RegisterHook(event, &config.HookAction{Type: "script", Script: string(event) + ".sh"}, 0)
	}

	// Trigger only one event
	err := engine.TriggerHooks(context.Background(), provisioning.HookEventPostNodeCreate, nil)
	require.NoError(t, err)

	// Should only have executed one hook
	assert.Equal(t, 1, mockExecutor.executeCalls)

	// Verify correct script
	assert.Equal(t, "post_node_create.sh", mockExecutor.lastAction.Script)
}
