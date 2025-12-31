package taints

import (
	"context"
	"errors"
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mocks
// =============================================================================

// MockKubeClient implements KubeClient for testing
type MockKubeClient struct {
	taints        map[string][]*Taint
	applyError    error
	removeError   error
	getError      error
	executeError  error
	applyCalls    int
	removeCalls   int
	executeOutput string
}

func NewMockKubeClient() *MockKubeClient {
	return &MockKubeClient{
		taints: make(map[string][]*Taint),
	}
}

func (m *MockKubeClient) ApplyTaint(ctx context.Context, nodeName string, taint *Taint) error {
	m.applyCalls++
	if m.applyError != nil {
		return m.applyError
	}
	if m.taints[nodeName] == nil {
		m.taints[nodeName] = make([]*Taint, 0)
	}
	// Update existing or add new
	for i, t := range m.taints[nodeName] {
		if t.Key == taint.Key {
			m.taints[nodeName][i] = taint
			return nil
		}
	}
	m.taints[nodeName] = append(m.taints[nodeName], taint)
	return nil
}

func (m *MockKubeClient) RemoveTaint(ctx context.Context, nodeName string, taintKey string) error {
	m.removeCalls++
	if m.removeError != nil {
		return m.removeError
	}
	if m.taints[nodeName] == nil {
		return nil
	}
	filtered := make([]*Taint, 0)
	for _, t := range m.taints[nodeName] {
		if t.Key != taintKey {
			filtered = append(filtered, t)
		}
	}
	m.taints[nodeName] = filtered
	return nil
}

func (m *MockKubeClient) GetNodeTaints(ctx context.Context, nodeName string) ([]*Taint, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	return m.taints[nodeName], nil
}

func (m *MockKubeClient) ExecuteCommand(ctx context.Context, args ...string) (string, error) {
	if m.executeError != nil {
		return "", m.executeError
	}
	return m.executeOutput, nil
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
// Manager Tests
// =============================================================================

func TestNewManager(t *testing.T) {
	kubeClient := NewMockKubeClient()
	eventEmitter := &MockEventEmitter{}

	manager := NewManager(&ManagerConfig{
		KubeClient:   kubeClient,
		EventEmitter: eventEmitter,
	})

	assert.NotNil(t, manager)
	assert.Equal(t, kubeClient, manager.kubeClient)
	assert.Equal(t, eventEmitter, manager.eventEmitter)
}

func TestManager_ApplyTaints_Success(t *testing.T) {
	kubeClient := NewMockKubeClient()
	eventEmitter := &MockEventEmitter{}

	manager := NewManager(&ManagerConfig{
		KubeClient:   kubeClient,
		EventEmitter: eventEmitter,
	})

	taints := []config.TaintConfig{
		{Key: "dedicated", Value: "gpu", Effect: "NoSchedule"},
		{Key: "special", Value: "true", Effect: "PreferNoSchedule"},
	}

	err := manager.ApplyTaints(context.Background(), "node-1", taints)
	require.NoError(t, err)

	assert.Equal(t, 2, kubeClient.applyCalls)
	assert.Len(t, kubeClient.taints["node-1"], 2)
	assert.Len(t, eventEmitter.events, 2)
	assert.Equal(t, "taint_applied", eventEmitter.events[0].Type)
}

func TestManager_ApplyTaints_Error(t *testing.T) {
	kubeClient := NewMockKubeClient()
	kubeClient.applyError = errors.New("apply failed")
	eventEmitter := &MockEventEmitter{}

	manager := NewManager(&ManagerConfig{
		KubeClient:   kubeClient,
		EventEmitter: eventEmitter,
	})

	taints := []config.TaintConfig{
		{Key: "dedicated", Value: "gpu", Effect: "NoSchedule"},
	}

	err := manager.ApplyTaints(context.Background(), "node-1", taints)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to apply taint")
	assert.Equal(t, "taint_apply_failed", eventEmitter.events[0].Type)
}

func TestManager_RemoveTaints_Success(t *testing.T) {
	kubeClient := NewMockKubeClient()
	kubeClient.taints["node-1"] = []*Taint{
		{Key: "dedicated", Value: "gpu", Effect: TaintEffectNoSchedule},
		{Key: "special", Value: "true", Effect: TaintEffectPreferNoSchedule},
	}
	eventEmitter := &MockEventEmitter{}

	manager := NewManager(&ManagerConfig{
		KubeClient:   kubeClient,
		EventEmitter: eventEmitter,
	})

	err := manager.RemoveTaints(context.Background(), "node-1", []string{"dedicated"})
	require.NoError(t, err)

	assert.Equal(t, 1, kubeClient.removeCalls)
	assert.Len(t, kubeClient.taints["node-1"], 1)
	assert.Equal(t, "special", kubeClient.taints["node-1"][0].Key)
	assert.Equal(t, "taint_removed", eventEmitter.events[0].Type)
}

func TestManager_RemoveTaints_Error(t *testing.T) {
	kubeClient := NewMockKubeClient()
	kubeClient.removeError = errors.New("remove failed")
	eventEmitter := &MockEventEmitter{}

	manager := NewManager(&ManagerConfig{
		KubeClient:   kubeClient,
		EventEmitter: eventEmitter,
	})

	err := manager.RemoveTaints(context.Background(), "node-1", []string{"dedicated"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove taint")
	assert.Equal(t, "taint_remove_failed", eventEmitter.events[0].Type)
}

func TestManager_GetTaints_Success(t *testing.T) {
	kubeClient := NewMockKubeClient()
	kubeClient.taints["node-1"] = []*Taint{
		{Key: "dedicated", Value: "gpu", Effect: TaintEffectNoSchedule},
		{Key: "special", Value: "", Effect: TaintEffectNoExecute},
	}

	manager := NewManager(&ManagerConfig{
		KubeClient: kubeClient,
	})

	taints, err := manager.GetTaints(context.Background(), "node-1")
	require.NoError(t, err)

	assert.Len(t, taints, 2)
	assert.Equal(t, "dedicated", taints[0].Key)
	assert.Equal(t, "gpu", taints[0].Value)
	assert.Equal(t, "NoSchedule", taints[0].Effect)
	assert.Equal(t, "special", taints[1].Key)
	assert.Equal(t, "NoExecute", taints[1].Effect)
}

func TestManager_GetTaints_Error(t *testing.T) {
	kubeClient := NewMockKubeClient()
	kubeClient.getError = errors.New("get failed")

	manager := NewManager(&ManagerConfig{
		KubeClient: kubeClient,
	})

	_, err := manager.GetTaints(context.Background(), "node-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get taints")
}

func TestManager_SyncTaints_AddNew(t *testing.T) {
	kubeClient := NewMockKubeClient()
	eventEmitter := &MockEventEmitter{}

	manager := NewManager(&ManagerConfig{
		KubeClient:   kubeClient,
		EventEmitter: eventEmitter,
	})

	desiredTaints := []config.TaintConfig{
		{Key: "dedicated", Value: "gpu", Effect: "NoSchedule"},
	}

	err := manager.SyncTaints(context.Background(), "node-1", desiredTaints)
	require.NoError(t, err)

	assert.Equal(t, 1, kubeClient.applyCalls)
	assert.Len(t, kubeClient.taints["node-1"], 1)
}

func TestManager_SyncTaints_RemoveOld(t *testing.T) {
	kubeClient := NewMockKubeClient()
	kubeClient.taints["node-1"] = []*Taint{
		{Key: "old-taint", Value: "remove", Effect: TaintEffectNoSchedule},
	}
	eventEmitter := &MockEventEmitter{}

	manager := NewManager(&ManagerConfig{
		KubeClient:   kubeClient,
		EventEmitter: eventEmitter,
	})

	// Sync with empty desired taints - should remove old
	err := manager.SyncTaints(context.Background(), "node-1", []config.TaintConfig{})
	require.NoError(t, err)

	assert.Equal(t, 1, kubeClient.removeCalls)
	assert.Len(t, kubeClient.taints["node-1"], 0)
}

func TestManager_SyncTaints_UpdateExisting(t *testing.T) {
	kubeClient := NewMockKubeClient()
	kubeClient.taints["node-1"] = []*Taint{
		{Key: "dedicated", Value: "old-value", Effect: TaintEffectNoSchedule},
	}
	eventEmitter := &MockEventEmitter{}

	manager := NewManager(&ManagerConfig{
		KubeClient:   kubeClient,
		EventEmitter: eventEmitter,
	})

	desiredTaints := []config.TaintConfig{
		{Key: "dedicated", Value: "new-value", Effect: "NoSchedule"},
	}

	err := manager.SyncTaints(context.Background(), "node-1", desiredTaints)
	require.NoError(t, err)

	// Should update since value changed
	assert.Equal(t, 1, kubeClient.applyCalls)
	assert.Equal(t, "new-value", kubeClient.taints["node-1"][0].Value)
}

func TestManager_SyncTaints_NoChanges(t *testing.T) {
	kubeClient := NewMockKubeClient()
	kubeClient.taints["node-1"] = []*Taint{
		{Key: "dedicated", Value: "gpu", Effect: TaintEffectNoSchedule},
	}
	eventEmitter := &MockEventEmitter{}

	manager := NewManager(&ManagerConfig{
		KubeClient:   kubeClient,
		EventEmitter: eventEmitter,
	})

	desiredTaints := []config.TaintConfig{
		{Key: "dedicated", Value: "gpu", Effect: "NoSchedule"},
	}

	err := manager.SyncTaints(context.Background(), "node-1", desiredTaints)
	require.NoError(t, err)

	// No changes needed
	assert.Equal(t, 0, kubeClient.applyCalls)
	assert.Equal(t, 0, kubeClient.removeCalls)
}

func TestManager_EventEmitter_Nil(t *testing.T) {
	kubeClient := NewMockKubeClient()

	// No event emitter configured - should not panic
	manager := NewManager(&ManagerConfig{
		KubeClient:   kubeClient,
		EventEmitter: nil,
	})

	taints := []config.TaintConfig{
		{Key: "dedicated", Value: "gpu", Effect: "NoSchedule"},
	}

	err := manager.ApplyTaints(context.Background(), "node-1", taints)
	require.NoError(t, err)
}

// =============================================================================
// Taint Tests
// =============================================================================

func TestTaint_String(t *testing.T) {
	tests := []struct {
		name     string
		taint    *Taint
		expected string
	}{
		{
			"With value",
			&Taint{Key: "dedicated", Value: "gpu", Effect: TaintEffectNoSchedule},
			"dedicated=gpu:NoSchedule",
		},
		{
			"Without value",
			&Taint{Key: "node-role.kubernetes.io/master", Value: "", Effect: TaintEffectNoSchedule},
			"node-role.kubernetes.io/master:NoSchedule",
		},
		{
			"NoExecute effect",
			&Taint{Key: "maintenance", Value: "true", Effect: TaintEffectNoExecute},
			"maintenance=true:NoExecute",
		},
		{
			"PreferNoSchedule effect",
			&Taint{Key: "spot", Value: "true", Effect: TaintEffectPreferNoSchedule},
			"spot=true:PreferNoSchedule",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.taint.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseTaintEffect(t *testing.T) {
	tests := []struct {
		input    string
		expected TaintEffect
	}{
		{"NoSchedule", TaintEffectNoSchedule},
		{"noschedule", TaintEffectNoSchedule},
		{"NOSCHEDULE", TaintEffectNoSchedule},
		{"PreferNoSchedule", TaintEffectPreferNoSchedule},
		{"prefernoschedule", TaintEffectPreferNoSchedule},
		{"NoExecute", TaintEffectNoExecute},
		{"noexecute", TaintEffectNoExecute},
		{"invalid", TaintEffectNoSchedule}, // default
		{"", TaintEffectNoSchedule},        // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseTaintEffect(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// KubectlClient Tests
// =============================================================================

func TestNewKubectlClient(t *testing.T) {
	executor := &MockCommandExecutor{}
	client := NewKubectlClient("/path/to/kubeconfig", executor)

	assert.NotNil(t, client)
	assert.Equal(t, "/path/to/kubeconfig", client.kubeconfigPath)
	assert.Equal(t, executor, client.executor)
}

func TestKubectlClient_ApplyTaint(t *testing.T) {
	executor := &MockCommandExecutor{output: "node/test-node tainted"}
	client := NewKubectlClient("/path/to/kubeconfig", executor)

	taint := &Taint{Key: "dedicated", Value: "gpu", Effect: TaintEffectNoSchedule}
	err := client.ApplyTaint(context.Background(), "test-node", taint)
	require.NoError(t, err)

	assert.Len(t, executor.calls, 1)
	call := executor.calls[0]
	assert.Equal(t, "kubectl", call[0])
	assert.Contains(t, call, "--kubeconfig")
	assert.Contains(t, call, "/path/to/kubeconfig")
	assert.Contains(t, call, "taint")
	assert.Contains(t, call, "nodes")
	assert.Contains(t, call, "test-node")
	assert.Contains(t, call, "dedicated=gpu:NoSchedule")
	assert.Contains(t, call, "--overwrite")
}

func TestKubectlClient_ApplyTaint_NoKubeconfig(t *testing.T) {
	executor := &MockCommandExecutor{output: "node/test-node tainted"}
	client := NewKubectlClient("", executor) // No kubeconfig

	taint := &Taint{Key: "dedicated", Value: "gpu", Effect: TaintEffectNoSchedule}
	err := client.ApplyTaint(context.Background(), "test-node", taint)
	require.NoError(t, err)

	call := executor.calls[0]
	assert.NotContains(t, call, "--kubeconfig")
}

func TestKubectlClient_ApplyTaint_Error(t *testing.T) {
	executor := &MockCommandExecutor{err: errors.New("command failed")}
	client := NewKubectlClient("", executor)

	taint := &Taint{Key: "dedicated", Value: "gpu", Effect: TaintEffectNoSchedule}
	err := client.ApplyTaint(context.Background(), "test-node", taint)
	assert.Error(t, err)
}

func TestKubectlClient_RemoveTaint(t *testing.T) {
	executor := &MockCommandExecutor{output: "node/test-node untainted"}
	client := NewKubectlClient("/path/to/kubeconfig", executor)

	err := client.RemoveTaint(context.Background(), "test-node", "dedicated")
	require.NoError(t, err)

	call := executor.calls[0]
	assert.Contains(t, call, "taint")
	assert.Contains(t, call, "nodes")
	assert.Contains(t, call, "test-node")
	assert.Contains(t, call, "dedicated-") // Key with - suffix
}

func TestKubectlClient_GetNodeTaints(t *testing.T) {
	executor := &MockCommandExecutor{
		output: `[{"key":"dedicated","value":"gpu","effect":"NoSchedule"}]`,
	}
	client := NewKubectlClient("", executor)

	taints, err := client.GetNodeTaints(context.Background(), "test-node")
	require.NoError(t, err)

	call := executor.calls[0]
	assert.Contains(t, call, "get")
	assert.Contains(t, call, "node")
	assert.Contains(t, call, "test-node")
	assert.Contains(t, call, "-o")
	assert.Contains(t, call, "jsonpath={.spec.taints}")

	// Verify parsing
	assert.Len(t, taints, 1)
	assert.Equal(t, "dedicated", taints[0].Key)
}

func TestKubectlClient_ExecuteCommand(t *testing.T) {
	executor := &MockCommandExecutor{output: "command output"}
	client := NewKubectlClient("/path/to/kubeconfig", executor)

	output, err := client.ExecuteCommand(context.Background(), "get", "nodes")
	require.NoError(t, err)
	assert.Equal(t, "command output", output)

	call := executor.calls[0]
	assert.Equal(t, "kubectl", call[0])
	assert.Contains(t, call, "--kubeconfig")
	assert.Contains(t, call, "get")
	assert.Contains(t, call, "nodes")
}

// =============================================================================
// parseTaintsOutput Tests
// =============================================================================

func TestParseTaintsOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			"Single taint",
			`{"key":"dedicated","value":"gpu","effect":"NoSchedule"}`,
			1,
		},
		{
			"Multiple taints",
			`{"key":"dedicated","value":"gpu","effect":"NoSchedule"},{"key":"spot","value":"true","effect":"PreferNoSchedule"}`,
			2,
		},
		{
			"Empty",
			"",
			0,
		},
		{
			"Null",
			"null",
			0,
		},
		{
			"Empty brackets",
			"[]",
			0,
		},
		{
			"Taint without value",
			`{"key":"master","effect":"NoSchedule"}`,
			1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTaintsOutput(tt.input)
			assert.Len(t, result, tt.expected)
		})
	}
}

func TestParseTaintsOutput_ValidateFields(t *testing.T) {
	input := `{"key":"dedicated","value":"gpu","effect":"NoSchedule"}`
	taints := parseTaintsOutput(input)

	require.Len(t, taints, 1)
	assert.Equal(t, "dedicated", taints[0].Key)
	assert.Equal(t, "gpu", taints[0].Value)
	assert.Equal(t, TaintEffect("NoSchedule"), taints[0].Effect)
}

// =============================================================================
// Predefined Templates Tests
// =============================================================================

func TestGetPredefinedTemplates(t *testing.T) {
	templates := GetPredefinedTemplates()

	assert.NotEmpty(t, templates)
	assert.GreaterOrEqual(t, len(templates), 5)

	// Verify expected templates exist
	templateNames := make(map[string]bool)
	for _, tmpl := range templates {
		templateNames[tmpl.Name] = true
		assert.NotEmpty(t, tmpl.Name)
		assert.NotEmpty(t, tmpl.Description)
		assert.NotEmpty(t, tmpl.Taints)
	}

	assert.True(t, templateNames["master-only"])
	assert.True(t, templateNames["gpu-workloads"])
	assert.True(t, templateNames["spot-instance"])
	assert.True(t, templateNames["dedicated-team"])
	assert.True(t, templateNames["maintenance"])
}

func TestGetTemplate_Exists(t *testing.T) {
	tests := []struct {
		name           string
		expectedTaints int
	}{
		{"master-only", 2},
		{"gpu-workloads", 1},
		{"spot-instance", 1},
		{"dedicated-team", 1},
		{"maintenance", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := GetTemplate(tt.name)
			require.NotNil(t, tmpl)
			assert.Equal(t, tt.name, tmpl.Name)
			assert.Len(t, tmpl.Taints, tt.expectedTaints)
		})
	}
}

func TestGetTemplate_NotExists(t *testing.T) {
	tmpl := GetTemplate("nonexistent")
	assert.Nil(t, tmpl)
}

func TestTemplate_MasterOnly(t *testing.T) {
	tmpl := GetTemplate("master-only")
	require.NotNil(t, tmpl)

	// Should have both master and control-plane taints
	keys := make(map[string]bool)
	for _, taint := range tmpl.Taints {
		keys[taint.Key] = true
		assert.Equal(t, "NoSchedule", taint.Effect)
	}

	assert.True(t, keys["node-role.kubernetes.io/master"])
	assert.True(t, keys["node-role.kubernetes.io/control-plane"])
}

func TestTemplate_SpotInstance(t *testing.T) {
	tmpl := GetTemplate("spot-instance")
	require.NotNil(t, tmpl)

	require.Len(t, tmpl.Taints, 1)
	assert.Equal(t, "kubernetes.io/spot-instance", tmpl.Taints[0].Key)
	assert.Equal(t, "true", tmpl.Taints[0].Value)
	assert.Equal(t, "PreferNoSchedule", tmpl.Taints[0].Effect)
}

func TestTemplate_Maintenance(t *testing.T) {
	tmpl := GetTemplate("maintenance")
	require.NotNil(t, tmpl)

	require.Len(t, tmpl.Taints, 1)
	assert.Equal(t, "NoExecute", tmpl.Taints[0].Effect) // Should evict pods
}

// =============================================================================
// Integration-like Tests
// =============================================================================

func TestManager_FullWorkflow(t *testing.T) {
	kubeClient := NewMockKubeClient()
	eventEmitter := &MockEventEmitter{}

	manager := NewManager(&ManagerConfig{
		KubeClient:   kubeClient,
		EventEmitter: eventEmitter,
	})

	ctx := context.Background()
	nodeName := "worker-1"

	// Step 1: Apply initial taints
	initialTaints := []config.TaintConfig{
		{Key: "dedicated", Value: "team-a", Effect: "NoSchedule"},
		{Key: "environment", Value: "production", Effect: "PreferNoSchedule"},
	}
	err := manager.ApplyTaints(ctx, nodeName, initialTaints)
	require.NoError(t, err)
	assert.Len(t, kubeClient.taints[nodeName], 2)

	// Step 2: Verify taints
	taints, err := manager.GetTaints(ctx, nodeName)
	require.NoError(t, err)
	assert.Len(t, taints, 2)

	// Step 3: Sync to new state (modify one, remove one, add one)
	newDesired := []config.TaintConfig{
		{Key: "dedicated", Value: "team-b", Effect: "NoSchedule"}, // modified
		{Key: "new-taint", Value: "value", Effect: "NoExecute"},   // added
		// "environment" removed
	}
	err = manager.SyncTaints(ctx, nodeName, newDesired)
	require.NoError(t, err)

	// Verify final state
	finalTaints, err := manager.GetTaints(ctx, nodeName)
	require.NoError(t, err)
	assert.Len(t, finalTaints, 2)

	// Step 4: Remove all taints
	err = manager.SyncTaints(ctx, nodeName, []config.TaintConfig{})
	require.NoError(t, err)

	finalTaints, err = manager.GetTaints(ctx, nodeName)
	require.NoError(t, err)
	assert.Len(t, finalTaints, 0)
}

func TestManager_MultipleNodes(t *testing.T) {
	kubeClient := NewMockKubeClient()
	eventEmitter := &MockEventEmitter{}

	manager := NewManager(&ManagerConfig{
		KubeClient:   kubeClient,
		EventEmitter: eventEmitter,
	})

	ctx := context.Background()

	// Apply different taints to different nodes
	nodes := map[string][]config.TaintConfig{
		"master-1": {
			{Key: "node-role.kubernetes.io/master", Effect: "NoSchedule"},
		},
		"worker-1": {
			{Key: "spot", Value: "true", Effect: "PreferNoSchedule"},
		},
		"worker-2": {
			{Key: "dedicated", Value: "gpu", Effect: "NoSchedule"},
		},
	}

	for nodeName, taints := range nodes {
		err := manager.ApplyTaints(ctx, nodeName, taints)
		require.NoError(t, err)
	}

	// Verify each node has its taints
	for nodeName, expectedTaints := range nodes {
		actualTaints, err := manager.GetTaints(ctx, nodeName)
		require.NoError(t, err)
		assert.Len(t, actualTaints, len(expectedTaints))
	}
}
