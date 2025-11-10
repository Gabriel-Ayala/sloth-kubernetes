package health

import (
	"fmt"
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// TestNewHealthChecker_Creation tests health checker creation
func TestNewHealthChecker_Creation(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewHealthChecker(ctx)

		assert.NotNil(t, checker)
		assert.Equal(t, ctx, checker.ctx)
		assert.NotNil(t, checker.nodes)
		assert.NotNil(t, checker.statuses)
		assert.Len(t, checker.nodes, 0)
		assert.Len(t, checker.statuses, 0)
		assert.Empty(t, checker.sshKeyPath)

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthTestMockProvider{}))

	assert.NoError(t, err)
}

// TestAddNode_Single tests adding a single node
func TestAddNode_Single(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewHealthChecker(ctx)

		node := &providers.NodeOutput{
			Name: "test-node-1",
		}

		checker.AddNode(node)

		assert.Len(t, checker.nodes, 1)
		assert.Equal(t, node, checker.nodes[0])
		assert.Contains(t, checker.statuses, "test-node-1")
		assert.Equal(t, "test-node-1", checker.statuses["test-node-1"].NodeName)

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthTestMockProvider{}))

	assert.NoError(t, err)
}

// TestAddNode_Multiple tests adding multiple nodes
func TestAddNode_Multiple(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewHealthChecker(ctx)

		for i := 1; i <= 5; i++ {
			node := &providers.NodeOutput{
				Name: fmt.Sprintf("node-%d", i),
			}
			checker.AddNode(node)
		}

		assert.Len(t, checker.nodes, 5)
		assert.Len(t, checker.statuses, 5)

		for i := 1; i <= 5; i++ {
			nodeName := fmt.Sprintf("node-%d", i)
			assert.Contains(t, checker.statuses, nodeName)
		}

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthTestMockProvider{}))

	assert.NoError(t, err)
}

// TestSetSSHKeyPath_Basic tests setting SSH key path
func TestSetSSHKeyPath_Basic(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewHealthChecker(ctx)

		assert.Empty(t, checker.sshKeyPath)

		keyPath := "/home/user/.ssh/id_rsa"
		checker.SetSSHKeyPath(keyPath)

		assert.Equal(t, keyPath, checker.sshKeyPath)

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthTestMockProvider{}))

	assert.NoError(t, err)
}

// TestSetSSHKeyPath_Update tests updating SSH key path
func TestSetSSHKeyPath_Update(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewHealthChecker(ctx)

		keyPath1 := "/home/user/.ssh/id_rsa"
		checker.SetSSHKeyPath(keyPath1)
		assert.Equal(t, keyPath1, checker.sshKeyPath)

		keyPath2 := "/home/user/.ssh/id_ed25519"
		checker.SetSSHKeyPath(keyPath2)
		assert.Equal(t, keyPath2, checker.sshKeyPath)

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthTestMockProvider{}))

	assert.NoError(t, err)
}

// TestGetNodeStatus_NoNodes tests getting status with no nodes
func TestGetNodeStatus_NoNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewHealthChecker(ctx)

		status, err := checker.GetNodeStatus("non-existent")

		assert.Error(t, err)
		assert.Nil(t, status)

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthTestMockProvider{}))

	assert.NoError(t, err)
}

// TestGetNodeStatus_WithNode tests getting status with a node added
func TestGetNodeStatus_WithNode(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewHealthChecker(ctx)

		node := &providers.NodeOutput{
			Name: "test-node",
		}
		checker.AddNode(node)

		status, err := checker.GetNodeStatus("test-node")

		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.Equal(t, "test-node", status.NodeName)

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthTestMockProvider{}))

	assert.NoError(t, err)
}

// TestGetAllStatuses_Empty tests getting all statuses with no nodes
func TestGetAllStatuses_Empty(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewHealthChecker(ctx)

		statuses := checker.GetAllStatuses()

		assert.NotNil(t, statuses)
		assert.Empty(t, statuses)

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthTestMockProvider{}))

	assert.NoError(t, err)
}

// TestGetAllStatuses_WithNodes tests getting all statuses with nodes
func TestGetAllStatuses_WithNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewHealthChecker(ctx)

		for i := 1; i <= 3; i++ {
			node := &providers.NodeOutput{
				Name: fmt.Sprintf("node-%d", i),
			}
			checker.AddNode(node)
		}

		statuses := checker.GetAllStatuses()

		assert.Len(t, statuses, 3)
		assert.Contains(t, statuses, "node-1")
		assert.Contains(t, statuses, "node-2")
		assert.Contains(t, statuses, "node-3")

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthTestMockProvider{}))

	assert.NoError(t, err)
}

// TestNodeStatus_Structure tests node status structure
func TestNodeStatus_Structure(t *testing.T) {
	status := &NodeStatus{
		NodeName:  "test-node",
		IsHealthy: true,
		Services:  map[string]bool{"docker": true, "kubelet": true},
		Error:     nil,
	}

	assert.Equal(t, "test-node", status.NodeName)
	assert.True(t, status.IsHealthy)
	assert.Len(t, status.Services, 2)
	assert.True(t, status.Services["docker"])
	assert.True(t, status.Services["kubelet"])
	assert.NoError(t, status.Error)
}

// TestNewPrerequisiteValidator_Creation tests validator creation
func TestNewPrerequisiteValidator_Creation(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		validator := NewPrerequisiteValidator(ctx)

		assert.NotNil(t, validator)
		assert.NotNil(t, validator.results)
		assert.Empty(t, validator.results)
		assert.Equal(t, ctx, validator.ctx)

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthTestMockProvider{}))

	assert.NoError(t, err)
}

// TestPrerequisiteValidator_GetResults_Empty tests getting results with no validations
func TestPrerequisiteValidator_GetResults_Empty(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		validator := NewPrerequisiteValidator(ctx)

		results := validator.GetResults()

		assert.NotNil(t, results)
		assert.Empty(t, results)

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthTestMockProvider{}))

	assert.NoError(t, err)
}

// TestPrerequisiteValidator_GetResults_WithData tests getting results with data
func TestPrerequisiteValidator_GetResults_WithData(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		validator := NewPrerequisiteValidator(ctx)

		// Manually add some results for testing
		validator.results["test-check"] = &ValidationResult{
			Name:    "test-check",
			Success: true,
			Message: "test message",
		}

		results := validator.GetResults()

		assert.Len(t, results, 1)
		assert.Contains(t, results, "test-check")
		assert.Equal(t, "test-check", results["test-check"].Name)
		assert.True(t, results["test-check"].Success)
		assert.Equal(t, "test message", results["test-check"].Message)

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthTestMockProvider{}))

	assert.NoError(t, err)
}

// TestValidationResult_Structure tests validation result structure
func TestValidationResult_Structure(t *testing.T) {
	result := &ValidationResult{
		Name:    "network-connectivity",
		Success: true,
		Message: "All nodes can communicate",
		Error:   nil,
	}

	assert.Equal(t, "network-connectivity", result.Name)
	assert.True(t, result.Success)
	assert.Equal(t, "All nodes can communicate", result.Message)
	assert.NoError(t, result.Error)
}

// TestValidationResult_Failed tests validation result with failure
func TestValidationResult_Failed(t *testing.T) {
	testError := fmt.Errorf("docker not found")
	result := &ValidationResult{
		Name:    "docker-installation",
		Success: false,
		Message: "Docker not found on node",
		Error:   testError,
	}

	assert.Equal(t, "docker-installation", result.Name)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "Docker not found")
	assert.Error(t, result.Error)
	assert.Equal(t, testError, result.Error)
}

// TestHealthChecker_MultipleNodesManagement tests managing multiple nodes
func TestHealthChecker_MultipleNodesManagement(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewHealthChecker(ctx)

		// Add multiple nodes
		nodes := []*providers.NodeOutput{
			{Name: "master-1"},
			{Name: "master-2"},
			{Name: "worker-1"},
			{Name: "worker-2"},
			{Name: "worker-3"},
		}

		for _, node := range nodes {
			checker.AddNode(node)
		}

		assert.Len(t, checker.nodes, 5)

		// Get all statuses
		statuses := checker.GetAllStatuses()
		assert.Len(t, statuses, 5)

		// Get individual statuses
		for _, node := range nodes {
			status, err := checker.GetNodeStatus(node.Name)
			assert.NoError(t, err)
			assert.NotNil(t, status)
			assert.Equal(t, node.Name, status.NodeName)
		}

		// Try to get non-existent node
		nonExistent, err := checker.GetNodeStatus("does-not-exist")
		assert.Error(t, err)
		assert.Nil(t, nonExistent)

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthTestMockProvider{}))

	assert.NoError(t, err)
}

// HealthTestMockProvider implements pulumi.ResourceState for testing
type HealthTestMockProvider struct {
	pulumi.ResourceState
}

func (m *HealthTestMockProvider) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	return args.Name + "_id", args.Inputs, nil
}

func (m *HealthTestMockProvider) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}
