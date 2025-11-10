package health

import (
	"fmt"
	"testing"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// TestHealthCheckIntegration_CompleteFlow tests complete health check workflow
func TestHealthCheckIntegration_CompleteFlow(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// Create health checker
		checker := NewHealthChecker(ctx)
		checker.SetSSHKeyPath("/home/user/.ssh/id_rsa")

		// Add multiple nodes with different roles
		nodes := []*providers.NodeOutput{
			{
				Name:        "master-1",
				Labels:      map[string]string{"role": "master"},
				PublicIP:    pulumi.String("203.0.113.1").ToStringOutput(),
				PrivateIP:   pulumi.String("10.0.0.10").ToStringOutput(),
				WireGuardIP: "10.8.0.1",
			},
			{
				Name:        "worker-1",
				Labels:      map[string]string{"role": "worker"},
				PublicIP:    pulumi.String("203.0.113.2").ToStringOutput(),
				PrivateIP:   pulumi.String("10.0.0.20").ToStringOutput(),
				WireGuardIP: "10.8.0.2",
			},
			{
				Name:        "worker-2",
				Labels:      map[string]string{"role": "worker"},
				PublicIP:    pulumi.String("203.0.113.3").ToStringOutput(),
				PrivateIP:   pulumi.String("10.0.0.21").ToStringOutput(),
				WireGuardIP: "10.8.0.3",
			},
		}

		for _, node := range nodes {
			checker.AddNode(node)
		}

		// Verify nodes were added
		assert.Len(t, checker.nodes, 3)

		// Get all statuses
		statuses := checker.GetAllStatuses()
		assert.Len(t, statuses, 3)

		// Verify each node has a status
		for _, node := range nodes {
			status, err := checker.GetNodeStatus(node.Name)
			assert.NoError(t, err)
			assert.NotNil(t, status)
			assert.Equal(t, node.Name, status.NodeName)
			assert.NotNil(t, status.Services)
		}

		// Test health check script building for different services
		services := []string{"docker", "kubelet", "kubernetes"}
		script := checker.buildHealthCheckScript(services)

		assert.NotEmpty(t, script)
		assert.Contains(t, script, "docker")
		assert.Contains(t, script, "kubelet")
		assert.Contains(t, script, "kubernetes")
		assert.Contains(t, script, "#!/bin/bash")

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthIntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestPrerequisiteValidationIntegration_RKE tests RKE validation flow
func TestPrerequisiteValidationIntegration_RKE(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		validator := NewPrerequisiteValidator(ctx)

		// Create mock nodes for RKE cluster
		nodes := []*providers.NodeOutput{
			{Name: "master-1", Labels: map[string]string{"role": "master"}},
			{Name: "master-2", Labels: map[string]string{"role": "master"}},
			{Name: "master-3", Labels: map[string]string{"role": "master"}},
			{Name: "worker-1", Labels: map[string]string{"role": "worker"}},
			{Name: "worker-2", Labels: map[string]string{"role": "worker"}},
		}

		// Get initial results (empty)
		results := validator.GetResults()
		assert.Empty(t, results)

		// Manually test validation functions
		nodeCountResult := validator.validateNodeCount(nodes)
		assert.NotNil(t, nodeCountResult)
		assert.Equal(t, "node-count", nodeCountResult.Name)
		assert.True(t, nodeCountResult.Success) // 5 nodes is valid

		masterResult := validator.validateMasterNodes(nodes)
		assert.NotNil(t, masterResult)
		assert.Equal(t, "master-nodes", masterResult.Name)
		assert.True(t, masterResult.Success) // 3 masters is valid

		workerResult := validator.validateWorkerNodes(nodes)
		assert.NotNil(t, workerResult)
		assert.Equal(t, "worker-nodes", workerResult.Name)
		assert.True(t, workerResult.Success) // 2 workers is valid

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthIntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestPrerequisiteValidationIntegration_Ingress tests ingress validation
func TestPrerequisiteValidationIntegration_Ingress(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		validator := NewPrerequisiteValidator(ctx)

		nodes := []*providers.NodeOutput{
			{Name: "node-1", Labels: map[string]string{"role": "master"}},
		}

		// Test individual validation functions
		k8sResult := validator.validateKubernetesRunning(nodes)
		assert.NotNil(t, k8sResult)
		assert.Equal(t, "kubernetes-running", k8sResult.Name)

		podsResult := validator.validateKubernetesPods(nodes)
		assert.NotNil(t, podsResult)
		assert.Equal(t, "kubernetes-pods", podsResult.Name)

		helmResult := validator.validateHelmInstalled(nodes)
		assert.NotNil(t, helmResult)
		assert.Equal(t, "helm-installed", helmResult.Name)

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthIntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestPrerequisiteValidationIntegration_WireGuard tests WireGuard validation
func TestPrerequisiteValidationIntegration_WireGuard(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		validator := NewPrerequisiteValidator(ctx)

		nodes := []*providers.NodeOutput{
			{Name: "node-1"},
			{Name: "node-2"},
		}

		// Test WireGuard-specific validations
		wgResult := validator.validateWireGuardInstalled(nodes)
		assert.NotNil(t, wgResult)
		assert.Equal(t, "wireguard-installed", wgResult.Name)

		kernelResult := validator.validateKernelSupport(nodes)
		assert.NotNil(t, kernelResult)
		assert.Equal(t, "kernel-support", kernelResult.Name)

		netResult := validator.validateNetworkInterfaces(nodes)
		assert.NotNil(t, netResult)
		assert.Equal(t, "network-interfaces", netResult.Name)

		ipForwardResult := validator.validateIPForwarding(nodes)
		assert.NotNil(t, ipForwardResult)
		assert.Equal(t, "ip-forwarding", ipForwardResult.Name)

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthIntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestHealthCheckIntegration_MultiNodeScenarios tests various multi-node scenarios
func TestHealthCheckIntegration_MultiNodeScenarios(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		scenarios := []struct {
			name      string
			nodeCount int
			roles     map[string]int // role -> count
		}{
			{
				name:      "Single master",
				nodeCount: 1,
				roles:     map[string]int{"master": 1},
			},
			{
				name:      "HA masters",
				nodeCount: 3,
				roles:     map[string]int{"master": 3},
			},
			{
				name:      "Mixed cluster",
				nodeCount: 5,
				roles:     map[string]int{"master": 2, "worker": 3},
			},
			{
				name:      "Large cluster",
				nodeCount: 10,
				roles:     map[string]int{"master": 3, "worker": 7},
			},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				checker := NewHealthChecker(ctx)

				// Create nodes based on scenario
				nodeIdx := 0
				for role, count := range scenario.roles {
					for i := 0; i < count; i++ {
						node := &providers.NodeOutput{
							Name:   fmt.Sprintf("%s-%d", role, i+1),
							Labels: map[string]string{"role": role},
						}
						checker.AddNode(node)
						nodeIdx++
					}
				}

				// Verify correct number of nodes
				assert.Len(t, checker.nodes, scenario.nodeCount)

				// Get all statuses
				statuses := checker.GetAllStatuses()
				assert.Len(t, statuses, scenario.nodeCount)
			})
		}

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthIntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestHealthCheckIntegration_ServiceValidation tests service health validation
func TestHealthCheckIntegration_ServiceValidation(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewHealthChecker(ctx)

		// Test different service combinations
		serviceTests := []struct {
			name     string
			services []string
		}{
			{
				name:     "Basic services",
				services: []string{"docker", "kubelet"},
			},
			{
				name:     "Master services",
				services: []string{"docker", "kubelet", "kubernetes", "etcd"},
			},
			{
				name:     "Worker services",
				services: []string{"docker", "kubelet"},
			},
			{
				name:     "Ingress services",
				services: []string{"nginx", "kubernetes"},
			},
			{
				name:     "All services",
				services: []string{"docker", "kubelet", "kubernetes", "etcd", "nginx", "wireguard"},
			},
		}

		for _, tt := range serviceTests {
			t.Run(tt.name, func(t *testing.T) {
				script := checker.buildHealthCheckScript(tt.services)

				assert.NotEmpty(t, script)
				assert.Contains(t, script, "#!/bin/bash")

				// Verify all services are checked
				for _, service := range tt.services {
					assert.Contains(t, script, service)
				}
			})
		}

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthIntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestValidationResultsIntegration tests validation results aggregation
func TestValidationResultsIntegration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		validator := NewPrerequisiteValidator(ctx)

		// Simulate multiple validation results
		testResults := map[string]*ValidationResult{
			"check-1": {
				Name:      "check-1",
				Success:   true,
				Message:   "Check 1 passed",
				Timestamp: time.Now(),
			},
			"check-2": {
				Name:      "check-2",
				Success:   true,
				Message:   "Check 2 passed",
				Timestamp: time.Now(),
			},
			"check-3": {
				Name:      "check-3",
				Success:   false,
				Message:   "Check 3 failed",
				Error:     fmt.Errorf("validation error"),
				Timestamp: time.Now(),
			},
		}

		// Add results to validator
		for key, result := range testResults {
			validator.results[key] = result
		}

		// Get all results
		results := validator.GetResults()
		assert.Len(t, results, 3)

		// Verify each result
		for key, expectedResult := range testResults {
			actualResult, exists := results[key]
			assert.True(t, exists, "Result %s should exist", key)
			assert.Equal(t, expectedResult.Name, actualResult.Name)
			assert.Equal(t, expectedResult.Success, actualResult.Success)
			assert.Equal(t, expectedResult.Message, actualResult.Message)
		}

		// Count successes and failures
		successes := 0
		failures := 0
		for _, result := range results {
			if result.Success {
				successes++
			} else {
				failures++
			}
		}

		assert.Equal(t, 2, successes)
		assert.Equal(t, 1, failures)

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthIntegrationMockProvider{}))

	assert.NoError(t, err)
}

// TestNodeStatusIntegration tests node status management
func TestNodeStatusIntegration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewHealthChecker(ctx)

		// Add nodes and manually update their statuses
		nodes := []*providers.NodeOutput{
			{Name: "healthy-node"},
			{Name: "unhealthy-node"},
			{Name: "pending-node"},
		}

		for _, node := range nodes {
			checker.AddNode(node)
		}

		// Simulate different health states
		checker.statuses["healthy-node"].IsHealthy = true
		checker.statuses["healthy-node"].Services = map[string]bool{
			"docker":   true,
			"kubelet":  true,
			"kubernetes": true,
		}

		checker.statuses["unhealthy-node"].IsHealthy = false
		checker.statuses["unhealthy-node"].Error = fmt.Errorf("service unavailable")
		checker.statuses["unhealthy-node"].Services = map[string]bool{
			"docker":  true,
			"kubelet": false,
		}

		// Get and verify statuses
		healthyStatus, err := checker.GetNodeStatus("healthy-node")
		assert.NoError(t, err)
		assert.True(t, healthyStatus.IsHealthy)
		assert.Len(t, healthyStatus.Services, 3)
		assert.True(t, healthyStatus.Services["docker"])

		unhealthyStatus, err := checker.GetNodeStatus("unhealthy-node")
		assert.NoError(t, err)
		assert.False(t, unhealthyStatus.IsHealthy)
		assert.Error(t, unhealthyStatus.Error)
		assert.False(t, unhealthyStatus.Services["kubelet"])

		pendingStatus, err := checker.GetNodeStatus("pending-node")
		assert.NoError(t, err)
		assert.False(t, pendingStatus.IsHealthy) // Default is false
		assert.Empty(t, pendingStatus.Services)

		return nil
	}, pulumi.WithMocks("test", "stack", &HealthIntegrationMockProvider{}))

	assert.NoError(t, err)
}

// HealthIntegrationMockProvider implements pulumi mock provider
type HealthIntegrationMockProvider struct {
	pulumi.ResourceState
}

func (m *HealthIntegrationMockProvider) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	return args.Name + "_id", args.Inputs, nil
}

func (m *HealthIntegrationMockProvider) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}
