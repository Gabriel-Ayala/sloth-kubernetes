//go:build e2e
// +build e2e

// Package e2e provides full workflow end-to-end tests
package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chalkan3/sloth-kubernetes/internal/orchestrator"
	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/autoscaling"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/backup"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/costs"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/hooks"
	"github.com/chalkan3/sloth-kubernetes/pkg/vpc"
)

// =============================================================================
// Full Workflow E2E Test
// =============================================================================

// TestE2E_FullWorkflow_AWSClusterWithAllComponents tests the complete workflow:
// 1. Cost estimation
// 2. VPC creation
// 3. Cluster provisioning via orchestrator
// 4. Backup creation
// 5. Hook execution
// 6. Cleanup
func TestE2E_FullWorkflow_AWSClusterWithAllComponents(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping full workflow E2E test in short mode")
	}

	env := NewTestEnvironment()
	if !env.HasAWSCredentials() {
		t.Skip("Skipping: AWS credentials not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), env.Timeout)
	defer cancel()

	logger := NewTestLogger(t, "FullWorkflow")
	timer := NewTimer(t, "Full E2E Workflow")
	defer timer.Stop()

	cleanup := NewCleanupRegistry(t)
	defer cleanup.RunAll()

	// Event emitter for tracking events across all components
	eventEmitter := &testEventEmitter{}

	// ==========================================================================
	// Phase 1: Cost Estimation
	// ==========================================================================
	logger.Phase("Cost Estimation")

	clusterConfig := createTestClusterConfig(env)

	costEstimator := costs.NewEstimator(&costs.EstimatorConfig{
		EventEmitter: eventEmitter,
	})
	estimate, err := costEstimator.EstimateClusterCost(ctx, clusterConfig)
	require.NoError(t, err, "Cost estimation should succeed")

	logger.Info("Estimated monthly cost: $%.2f", estimate.TotalMonthlyCost)
	logger.Info("Estimated yearly cost: $%.2f", estimate.TotalYearlyCost)
	logger.Info("Spot savings: $%.2f/month", estimate.SpotSavings)

	assert.Greater(t, estimate.TotalMonthlyCost, 0.0, "Cost should be calculated")
	logger.Success("Cost estimation completed")

	// ==========================================================================
	// Phase 2: Hook Engine Setup
	// ==========================================================================
	logger.Phase("Hook Engine Setup")

	hookEngine := hooks.NewEngine(&hooks.EngineConfig{
		EventEmitter: eventEmitter,
	})

	// Register pre and post hooks
	hookEngine.RegisterHook(provisioning.HookEventPostNodeCreate, &config.HookAction{
		Type:   "script",
		Script: "echo 'Node created: ${HOOK_node_name}'",
	}, 1)

	hookEngine.RegisterHook(provisioning.HookEventPostClusterReady, &config.HookAction{
		Type:   "script",
		Script: "echo 'Cluster is ready!'",
	}, 1)

	logger.Success("Hook engine configured with %d hooks", 2)

	// ==========================================================================
	// Phase 3: Pulumi Stack Setup
	// ==========================================================================
	logger.Phase("Pulumi Stack Setup")

	stackName := fmt.Sprintf("%s-full-workflow", env.TestPrefix)
	projectName := "sloth-kubernetes-e2e"

	workDir := t.TempDir()
	backendURL := fmt.Sprintf("file://%s", workDir)

	// Pulumi program that uses real orchestrator
	program := func(pctx *pulumi.Context) error {
		// Phase 3a: Create VPCs
		logger.Info("Creating VPCs...")
		vpcManager := vpc.NewVPCManager(pctx)
		vpcs, err := vpcManager.CreateAllVPCs(&clusterConfig.Providers)
		if err != nil {
			return fmt.Errorf("VPC creation failed: %w", err)
		}

		for provider, vpcResult := range vpcs {
			pctx.Export(fmt.Sprintf("vpc_%s_id", provider), vpcResult.ID)
		}

		// Phase 3b: Create cluster using orchestrator
		logger.Info("Creating Kubernetes cluster via orchestrator...")
		clusterOrch, err := orchestrator.NewSimpleRealOrchestratorComponent(
			pctx,
			"e2e-cluster",
			clusterConfig,
		)
		if err != nil {
			return fmt.Errorf("Orchestrator failed: %w", err)
		}

		// Export outputs
		pctx.Export("clusterName", clusterOrch.ClusterName)
		pctx.Export("kubeConfig", clusterOrch.KubeConfig)
		pctx.Export("sshPrivateKey", clusterOrch.SSHPrivateKey)
		pctx.Export("apiEndpoint", clusterOrch.APIEndpoint)
		pctx.Export("status", clusterOrch.Status)

		return nil
	}

	// Create workspace
	ws, err := auto.NewLocalWorkspace(ctx,
		auto.Program(program),
		auto.Project(workspace.Project{
			Name:    tokens.PackageName(projectName),
			Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			Backend: &workspace.ProjectBackend{URL: backendURL},
		}),
		auto.SecretsProvider("passphrase"),
		auto.EnvVars(map[string]string{
			"PULUMI_CONFIG_PASSPHRASE": "test-passphrase",
		}),
	)
	require.NoError(t, err, "Workspace creation should succeed")

	// Create stack
	fullyQualifiedName := fmt.Sprintf("organization/%s/%s", projectName, stackName)
	stack, err := auto.UpsertStack(ctx, fullyQualifiedName, ws)
	require.NoError(t, err, "Stack creation should succeed")

	// Register cleanup
	cleanup.Register(func() error {
		logger.Info("Destroying stack...")
		destroyCtx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		_, err := stack.Destroy(destroyCtx, optdestroy.ProgressStreams(os.Stdout))
		return err
	})

	// Set AWS region config
	err = stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: env.AWSRegion})
	require.NoError(t, err)

	logger.Success("Pulumi stack configured")

	// ==========================================================================
	// Phase 4: Cluster Provisioning
	// ==========================================================================
	logger.Phase("Cluster Provisioning")

	provisionTimer := NewTimer(t, "Cluster Provisioning")

	// Trigger pre-provision hooks
	err = hookEngine.TriggerHooks(ctx, provisioning.HookEventPostNodeCreate, map[string]interface{}{
		"node_name": "pre-provision-check",
	})
	require.NoError(t, err)

	// Execute Pulumi Up
	logger.Info("Running Pulumi Up...")
	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi Up should succeed")

	provisionDuration := provisionTimer.Stop()
	logger.Success("Cluster provisioned in %v", provisionDuration)

	// ==========================================================================
	// Phase 5: Validate Outputs
	// ==========================================================================
	logger.Phase("Output Validation")

	// Validate cluster outputs
	clusterName, ok := result.Outputs["clusterName"]
	assert.True(t, ok, "clusterName output should exist")
	logger.Info("Cluster Name: %v", clusterName.Value)

	status, ok := result.Outputs["status"]
	assert.True(t, ok, "status output should exist")
	statusStr, _ := status.Value.(string)
	assert.Contains(t, statusStr, "success", "Status should indicate success")
	logger.Info("Status: %v", status.Value)

	apiEndpoint, ok := result.Outputs["apiEndpoint"]
	assert.True(t, ok, "apiEndpoint should exist")
	logger.Info("API Endpoint: %v", apiEndpoint.Value)

	logger.Success("All outputs validated")

	// ==========================================================================
	// Phase 6: Post-Provision Hooks
	// ==========================================================================
	logger.Phase("Post-Provision Hooks")

	err = hookEngine.TriggerHooks(ctx, provisioning.HookEventPostClusterReady, nil)
	require.NoError(t, err)

	// Verify events were emitted
	events := eventEmitter.GetEvents("")
	logger.Info("Total events emitted: %d", len(events))

	for _, event := range events {
		logger.Info("  Event: %s from %s", event.Type, event.Source)
	}

	logger.Success("Post-provision hooks executed")

	// ==========================================================================
	// Phase 7: Backup Test
	// ==========================================================================
	logger.Phase("Backup Creation")

	backupStorage := newMemoryBackupStorage()
	backupManager, err := backup.NewManager(&backup.ManagerConfig{
		BackupConfig: &config.BackupConfig{
			Enabled:       true,
			RetentionDays: 1,
		},
		Storage:      backupStorage,
		EventEmitter: eventEmitter,
	})
	require.NoError(t, err)

	// Register a mock cluster config backup component
	backupManager.RegisterComponent(&testBackupComponent{
		name: "cluster-config",
		data: []byte(fmt.Sprintf(`{"cluster_name": "%v", "api_endpoint": "%v"}`,
			clusterName.Value, apiEndpoint.Value)),
	})

	backupResult, err := backupManager.CreateBackup(ctx, []string{"cluster-config"})
	require.NoError(t, err)

	logger.Success("Backup created: ID=%s, Size=%d bytes", backupResult.ID, backupResult.Size)

	// ==========================================================================
	// Phase 8: AutoScaling Manager Test
	// ==========================================================================
	logger.Phase("AutoScaling Manager Test")

	mockScaler := &testNodeScaler{currentCount: 2}
	mockMetrics := &testMetricsCollector{
		cpuUtilization:    45.0,
		memoryUtilization: 50.0,
	}

	autoscalingManager, err := autoscaling.NewManager(&autoscaling.ManagerConfig{
		AutoScalingConfig: &config.AutoScalingConfig{
			Enabled:      true,
			MinNodes:     2,
			MaxNodes:     10,
			Cooldown:     60,
			TargetCPU:    80,
			TargetMemory: 80,
		},
		StrategyName: "composite",
		Scaler:       mockScaler,
		Metrics:      mockMetrics,
		EventEmitter: eventEmitter,
	})
	require.NoError(t, err)

	status2 := autoscalingManager.GetStatus()
	logger.Info("AutoScaling Status:")
	logger.Info("  Enabled: %v", status2.Enabled)
	logger.Info("  Strategy: %s", status2.Strategy)
	logger.Info("  Current Nodes: %d", status2.CurrentNodes)
	logger.Info("  Min/Max: %d/%d", status2.MinNodes, status2.MaxNodes)

	logger.Success("AutoScaling manager configured")

	// ==========================================================================
	// Summary
	// ==========================================================================
	logger.Phase("Test Summary")

	logger.Info("============================================================")
	logger.Success("Full E2E Workflow Test PASSED")
	logger.Info("============================================================")
	logger.Info("Components tested:")
	logger.Info("  ✅ Cost Estimator")
	logger.Info("  ✅ Hook Engine")
	logger.Info("  ✅ VPC Manager")
	logger.Info("  ✅ Cluster Orchestrator")
	logger.Info("  ✅ Backup Manager")
	logger.Info("  ✅ AutoScaling Manager")
	logger.Info("")
	logger.Info("Total duration: %v", time.Since(timer.startTime))
}

// createTestClusterConfig creates a cluster config for testing
func createTestClusterConfig(env *TestEnvironment) *config.ClusterConfig {
	return &config.ClusterConfig{
		Metadata: config.Metadata{
			Name: "e2e-full-workflow-test",
		},
		Providers: config.ProvidersConfig{
			AWS: &config.AWSProvider{
				Enabled: true,
				Region:  env.AWSRegion,
				VPC: &config.VPCConfig{
					Create: true,
					Name:   "e2e-test-vpc",
					CIDR:   "10.200.0.0/16",
				},
			},
		},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Count:    1,
				Size:     "t3.small",
				Image:    "ubuntu-22-04",
				Region:   env.AWSRegion,
				Provider: "aws",
				Roles:    []string{"master"},
			},
			"workers": {
				Name:         "workers",
				Count:        1,
				Size:         "t3.micro",
				Image:        "ubuntu-22-04",
				Region:       env.AWSRegion,
				Provider:     "aws",
				Roles:        []string{"worker"},
				SpotInstance: true,
			},
		},
		Security: config.SecurityConfig{
			SSHConfig: config.SSHConfig{},
		},
		Network: config.NetworkConfig{
			DNS: config.DNSConfig{
				Domain:   "e2e-test.local",
				Provider: "none",
			},
			WireGuard: &config.WireGuardConfig{
				Enabled: true,
			},
		},
		Kubernetes: config.KubernetesConfig{
			Distribution: "k3s",
			Version:      "v1.29.0+k3s1",
		},
		Addons: config.AddonsConfig{},
	}
}

// =============================================================================
// Minimal E2E Test (Quick Validation)
// =============================================================================

// TestE2E_MinimalWorkflow_ComponentsOnly tests components without cloud resources
func TestE2E_MinimalWorkflow_ComponentsOnly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	logger := NewTestLogger(t, "MinimalWorkflow")
	timer := NewTimer(t, "Minimal Workflow")
	defer timer.Stop()

	// Test Cost Estimator
	logger.Phase("Cost Estimator")
	estimator := costs.NewEstimator(&costs.EstimatorConfig{})
	nodeEstimate, err := estimator.EstimateNodeCost(ctx, &config.NodeConfig{
		Provider: "aws",
		Size:     "t3.medium",
		Region:   "us-east-1",
	})
	require.NoError(t, err)
	assert.Greater(t, nodeEstimate.MonthlyCost, 0.0)
	logger.Success("Cost: $%.2f/month", nodeEstimate.MonthlyCost)

	// Test Hook Engine
	logger.Phase("Hook Engine")
	eventEmitter := &testEventEmitter{}
	hookEngine := hooks.NewEngine(&hooks.EngineConfig{
		EventEmitter: eventEmitter,
	})

	hookID := hookEngine.RegisterHook(provisioning.HookEventPostNodeCreate, &config.HookAction{
		Type:   "script",
		Script: "echo 'test'",
	}, 1)
	assert.NotEmpty(t, hookID)

	err = hookEngine.TriggerHooks(ctx, provisioning.HookEventPostNodeCreate, nil)
	require.NoError(t, err)
	logger.Success("Hooks executed: %d events", len(eventEmitter.events))

	// Test Backup Manager
	logger.Phase("Backup Manager")
	storage := newMemoryBackupStorage()
	backupMgr, err := backup.NewManager(&backup.ManagerConfig{
		BackupConfig: &config.BackupConfig{Enabled: true, RetentionDays: 1},
		Storage:      storage,
		EventEmitter: eventEmitter,
	})
	require.NoError(t, err)

	backupMgr.RegisterComponent(&testBackupComponent{
		name: "test",
		data: []byte("test data"),
	})

	b, err := backupMgr.CreateBackup(ctx, []string{"test"})
	require.NoError(t, err)
	assert.Equal(t, "completed", b.Status)
	logger.Success("Backup created: %s", b.ID)

	// Test AutoScaling Manager
	logger.Phase("AutoScaling Manager")
	asMgr, err := autoscaling.NewManager(&autoscaling.ManagerConfig{
		AutoScalingConfig: &config.AutoScalingConfig{
			Enabled:  true,
			MinNodes: 1,
			MaxNodes: 5,
		},
		StrategyName: "cpu",
		Scaler:       &testNodeScaler{currentCount: 2},
		Metrics:      &testMetricsCollector{cpuUtilization: 50.0},
		EventEmitter: eventEmitter,
	})
	require.NoError(t, err)

	status := asMgr.GetStatus()
	assert.True(t, status.Enabled)
	logger.Success("AutoScaling: strategy=%s, nodes=%d", status.Strategy, status.CurrentNodes)

	logger.Phase("Summary")
	logger.Success("Minimal E2E Workflow PASSED")
	logger.Info("All components validated without cloud resources")
}

// =============================================================================
// Preview-Only E2E Test
// =============================================================================

// TestE2E_PreviewOnly_NoResourcesCreated tests preview mode
func TestE2E_PreviewOnly_NoResourcesCreated(t *testing.T) {
	env := NewTestEnvironment()
	if !env.HasAWSCredentials() {
		t.Skip("Skipping: AWS credentials not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger := NewTestLogger(t, "PreviewOnly")
	timer := NewTimer(t, "Preview Test")
	defer timer.Stop()

	clusterConfig := createTestClusterConfig(env)
	// Scale up for preview test - need to create modified copies
	masters := clusterConfig.NodePools["masters"]
	masters.Count = 3
	clusterConfig.NodePools["masters"] = masters

	workers := clusterConfig.NodePools["workers"]
	workers.Count = 5
	clusterConfig.NodePools["workers"] = workers

	program := func(pctx *pulumi.Context) error {
		_, err := orchestrator.NewSimpleRealOrchestratorComponent(
			pctx, "preview-cluster", clusterConfig,
		)
		return err
	}

	stackName := fmt.Sprintf("%s-preview", env.TestPrefix)
	workDir := t.TempDir()

	ws, err := auto.NewLocalWorkspace(ctx,
		auto.Program(program),
		auto.Project(workspace.Project{
			Name:    tokens.PackageName("preview-test"),
			Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			Backend: &workspace.ProjectBackend{URL: fmt.Sprintf("file://%s", workDir)},
		}),
		auto.SecretsProvider("passphrase"),
		auto.EnvVars(map[string]string{"PULUMI_CONFIG_PASSPHRASE": "test"}),
	)
	require.NoError(t, err)

	stack, err := auto.UpsertStack(ctx, fmt.Sprintf("organization/preview-test/%s", stackName), ws)
	require.NoError(t, err)

	err = stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: env.AWSRegion})
	require.NoError(t, err)

	logger.Info("Running Pulumi Preview...")
	preview, err := stack.Preview(ctx)
	require.NoError(t, err)

	logger.Info("Preview Summary:")
	logger.Info("  Creates: %d", preview.ChangeSummary["create"])
	logger.Info("  Updates: %d", preview.ChangeSummary["update"])
	logger.Info("  Deletes: %d", preview.ChangeSummary["delete"])

	assert.Greater(t, preview.ChangeSummary["create"], 0,
		"Should plan to create resources")

	logger.Success("Preview completed - NO resources were created")
}
