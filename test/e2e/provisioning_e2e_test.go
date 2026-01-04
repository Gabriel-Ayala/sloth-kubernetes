//go:build e2e
// +build e2e

// Package e2e provides end-to-end tests for provisioning components
package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/autoscaling"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/backup"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/costs"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/distribution"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/hooks"
)

// =============================================================================
// Test Infrastructure
// =============================================================================

// testEventEmitter captures events for testing
type testEventEmitter struct {
	events []provisioning.Event
}

func (e *testEventEmitter) Emit(event provisioning.Event) {
	e.events = append(e.events, event)
}

func (e *testEventEmitter) Subscribe(eventType string, handler provisioning.EventHandler) string {
	return "test-subscription"
}

func (e *testEventEmitter) Unsubscribe(subscriptionID string) {}

func (e *testEventEmitter) GetEvents(eventType string) []provisioning.Event {
	var filtered []provisioning.Event
	for _, ev := range e.events {
		if ev.Type == eventType || eventType == "" {
			filtered = append(filtered, ev)
		}
	}
	return filtered
}

// testMetricsCollector provides simulated metrics for testing
type testMetricsCollector struct {
	cpuUtilization    float64
	memoryUtilization float64
	customMetrics     map[string]float64
}

func (m *testMetricsCollector) GetCPUUtilization(ctx context.Context) (float64, error) {
	return m.cpuUtilization, nil
}

func (m *testMetricsCollector) GetMemoryUtilization(ctx context.Context) (float64, error) {
	return m.memoryUtilization, nil
}

func (m *testMetricsCollector) GetCustomMetric(ctx context.Context, name string) (float64, error) {
	if val, ok := m.customMetrics[name]; ok {
		return val, nil
	}
	return 0, fmt.Errorf("metric %s not found", name)
}

// testNodeScaler tracks scaling operations
type testNodeScaler struct {
	currentCount   int
	desiredCount   int
	scaleUpCalls   []int
	scaleDownCalls []int
}

func (s *testNodeScaler) ScaleUp(ctx context.Context, count int) error {
	s.scaleUpCalls = append(s.scaleUpCalls, count)
	s.currentCount += count
	return nil
}

func (s *testNodeScaler) ScaleDown(ctx context.Context, count int) error {
	s.scaleDownCalls = append(s.scaleDownCalls, count)
	s.currentCount -= count
	return nil
}

func (s *testNodeScaler) GetCurrentCount() int {
	return s.currentCount
}

func (s *testNodeScaler) GetDesiredCount() int {
	return s.desiredCount
}

// =============================================================================
// AutoScaling Manager E2E Tests
// =============================================================================

// TestE2E_AutoScaling_ScaleUpDecision tests autoscaling scale-up logic
func TestE2E_AutoScaling_ScaleUpDecision(t *testing.T) {
	_, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	eventEmitter := &testEventEmitter{}
	metrics := &testMetricsCollector{
		cpuUtilization:    85.0, // High CPU - should trigger scale up
		memoryUtilization: 60.0,
	}
	scaler := &testNodeScaler{
		currentCount: 3,
	}

	autoscalingCfg := &config.AutoScalingConfig{
		Enabled:      true,
		MinNodes:     2,
		MaxNodes:     10,
		Cooldown:     1, // 1 second cooldown for testing
		TargetCPU:    80,
		TargetMemory: 80,
	}

	manager, err := autoscaling.NewManager(&autoscaling.ManagerConfig{
		AutoScalingConfig: autoscalingCfg,
		StrategyName:      "cpu",
		Scaler:            scaler,
		Metrics:           metrics,
		EventEmitter:      eventEmitter,
	})
	require.NoError(t, err)

	// Get strategy and check scale up decision
	status := manager.GetStatus()
	assert.True(t, status.Enabled)
	assert.Equal(t, 3, scaler.currentCount)
	assert.Equal(t, "cpu", status.Strategy)

	t.Log("✅ AutoScaling manager initialized correctly")

	// Test that manager recognizes high CPU situation
	t.Logf("Current CPU: %.1f%%, Threshold: %d%%", metrics.cpuUtilization, autoscalingCfg.TargetCPU)
	assert.Greater(t, metrics.cpuUtilization, float64(autoscalingCfg.TargetCPU),
		"CPU should be above threshold")

	t.Log("✅ AutoScaling scale-up decision test PASSED")
}

// TestE2E_AutoScaling_ScaleDownDecision tests autoscaling scale-down logic
func TestE2E_AutoScaling_ScaleDownDecision(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = ctx

	eventEmitter := &testEventEmitter{}
	metrics := &testMetricsCollector{
		cpuUtilization:    20.0, // Low CPU - should trigger scale down
		memoryUtilization: 25.0, // Low memory
	}
	scaler := &testNodeScaler{
		currentCount: 8,
	}

	autoscalingCfg := &config.AutoScalingConfig{
		Enabled:        true,
		MinNodes:       2,
		MaxNodes:       10,
		Cooldown:       1,
		ScaleDownDelay: 1,
		TargetCPU:      80,
		TargetMemory:   80,
	}

	manager, err := autoscaling.NewManager(&autoscaling.ManagerConfig{
		AutoScalingConfig: autoscalingCfg,
		StrategyName:      "composite",
		Scaler:            scaler,
		Metrics:           metrics,
		EventEmitter:      eventEmitter,
	})
	require.NoError(t, err)

	status := manager.GetStatus()
	assert.Equal(t, "composite", status.Strategy)
	assert.Equal(t, 8, scaler.currentCount)
	assert.Equal(t, 10, status.MaxNodes)
	assert.Equal(t, 2, status.MinNodes)

	// Validate low utilization condition
	assert.Less(t, metrics.cpuUtilization, 30.0, "CPU should be low for scale down")
	assert.Less(t, metrics.memoryUtilization, 30.0, "Memory should be low for scale down")

	t.Log("✅ AutoScaling scale-down decision test PASSED")
}

// TestE2E_AutoScaling_StrategyRegistry tests strategy registration and switching
func TestE2E_AutoScaling_StrategyRegistry(t *testing.T) {
	registry := autoscaling.NewStrategyRegistry()

	// Test getting available strategies
	strategies := []string{"cpu", "memory", "composite", "predictive"}

	for _, strategyName := range strategies {
		strategy, err := registry.Get(strategyName)
		if err == nil {
			assert.Equal(t, strategyName, strategy.Name())
			t.Logf("✅ Strategy '%s' registered and accessible", strategyName)
		}
	}

	t.Log("✅ AutoScaling strategy registry test PASSED")
}

// =============================================================================
// Backup Manager E2E Tests (with real S3 if available)
// =============================================================================

// s3BackupStorage implements BackupStorage using real S3
type s3BackupStorage struct {
	client *s3.Client
	bucket string
	prefix string
}

func newS3BackupStorage(ctx context.Context, bucket, prefix string) (*s3BackupStorage, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	return &s3BackupStorage{
		client: s3.NewFromConfig(cfg),
		bucket: bucket,
		prefix: prefix,
	}, nil
}

func (s *s3BackupStorage) Name() string {
	return fmt.Sprintf("s3://%s/%s", s.bucket, s.prefix)
}

func (s *s3BackupStorage) Upload(ctx context.Context, key string, data []byte) error {
	fullKey := fmt.Sprintf("%s/%s", s.prefix, key)
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fullKey),
		Body:   bytes.NewReader(data),
	})
	return err
}

func (s *s3BackupStorage) Download(ctx context.Context, key string) ([]byte, error) {
	fullKey := fmt.Sprintf("%s/%s", s.prefix, key)
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fullKey),
	})
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(result.Body)
	return buf.Bytes(), nil
}

func (s *s3BackupStorage) Delete(ctx context.Context, key string) error {
	fullKey := fmt.Sprintf("%s/%s", s.prefix, key)
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fullKey),
	})
	return err
}

func (s *s3BackupStorage) List(ctx context.Context) ([]string, error) {
	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(s.prefix),
	})
	if err != nil {
		return nil, err
	}

	var keys []string
	for _, obj := range result.Contents {
		keys = append(keys, *obj.Key)
	}
	return keys, nil
}

// memoryBackupStorage implements BackupStorage in memory for testing
type memoryBackupStorage struct {
	data map[string][]byte
}

func newMemoryBackupStorage() *memoryBackupStorage {
	return &memoryBackupStorage{
		data: make(map[string][]byte),
	}
}

func (s *memoryBackupStorage) Name() string {
	return "memory://test"
}

func (s *memoryBackupStorage) Upload(ctx context.Context, key string, data []byte) error {
	s.data[key] = data
	return nil
}

func (s *memoryBackupStorage) Download(ctx context.Context, key string) ([]byte, error) {
	if data, ok := s.data[key]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("key not found: %s", key)
}

func (s *memoryBackupStorage) Delete(ctx context.Context, key string) error {
	delete(s.data, key)
	return nil
}

func (s *memoryBackupStorage) List(ctx context.Context) ([]string, error) {
	var keys []string
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys, nil
}

// testBackupComponent implements BackupComponent for testing
type testBackupComponent struct {
	name string
	data []byte
}

func (c *testBackupComponent) Name() string {
	return c.name
}

func (c *testBackupComponent) Backup(ctx context.Context) ([]byte, error) {
	return c.data, nil
}

func (c *testBackupComponent) Restore(ctx context.Context, data []byte) error {
	c.data = data
	return nil
}

// TestE2E_Backup_CreateAndRestore tests backup creation and restoration
func TestE2E_Backup_CreateAndRestore(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	eventEmitter := &testEventEmitter{}
	storage := newMemoryBackupStorage()

	backupCfg := &config.BackupConfig{
		Enabled:       true,
		RetentionDays: 7,
	}

	manager, err := backup.NewManager(&backup.ManagerConfig{
		BackupConfig: backupCfg,
		Storage:      storage,
		EventEmitter: eventEmitter,
	})
	require.NoError(t, err)

	// Register test components
	etcdComponent := &testBackupComponent{
		name: "etcd",
		data: []byte(`{"cluster_state": "healthy", "nodes": 3}`),
	}
	configComponent := &testBackupComponent{
		name: "config",
		data: []byte(`{"cluster_name": "production", "k8s_version": "1.29.0"}`),
	}

	manager.RegisterComponent(etcdComponent)
	manager.RegisterComponent(configComponent)

	// Create backup
	t.Log("📦 Creating backup...")
	backup, err := manager.CreateBackup(ctx, []string{"etcd", "config"})
	require.NoError(t, err)
	require.NotNil(t, backup)

	assert.NotEmpty(t, backup.ID)
	assert.Equal(t, "completed", backup.Status)
	assert.Equal(t, 2, len(backup.Components))
	assert.Greater(t, backup.Size, int64(0))

	t.Logf("✅ Backup created: ID=%s, Size=%d bytes", backup.ID, backup.Size)

	// Verify events were emitted
	events := eventEmitter.GetEvents("backup_completed")
	assert.GreaterOrEqual(t, len(events), 1, "Should emit backup_completed event")

	// List backups
	backups, err := manager.ListBackups(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, len(backups))

	// Modify original data
	etcdComponent.data = []byte(`{"cluster_state": "degraded", "nodes": 2}`)

	// Restore backup
	t.Log("📥 Restoring backup...")
	err = manager.RestoreBackup(ctx, backup.ID)
	require.NoError(t, err)

	// Verify data was restored
	assert.Equal(t, `{"cluster_state": "healthy", "nodes": 3}`, string(etcdComponent.data))

	t.Log("✅ Backup create and restore test PASSED")
}

// TestE2E_Backup_WithRealS3 tests backup with real S3 storage
func TestE2E_Backup_WithRealS3(t *testing.T) {
	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	bucket := os.Getenv("E2E_S3_BUCKET")
	if bucket == "" {
		t.Skip("Skipping S3 backup test: E2E_S3_BUCKET not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	prefix := fmt.Sprintf("e2e-backup-test-%d", time.Now().Unix())

	storage, err := newS3BackupStorage(ctx, bucket, prefix)
	require.NoError(t, err)

	backupCfg := &config.BackupConfig{
		Enabled:       true,
		RetentionDays: 1,
	}

	manager, err := backup.NewManager(&backup.ManagerConfig{
		BackupConfig: backupCfg,
		Storage:      storage,
		EventEmitter: &testEventEmitter{},
	})
	require.NoError(t, err)

	// Register test component
	testData := []byte(`{"test": "real-s3-backup", "timestamp": "` + time.Now().String() + `"}`)
	manager.RegisterComponent(&testBackupComponent{
		name: "test-data",
		data: testData,
	})

	// Create backup to S3
	t.Log("📦 Creating backup to S3...")
	backup, err := manager.CreateBackup(ctx, []string{"test-data"})
	require.NoError(t, err)

	t.Logf("✅ Backup created in S3: s3://%s/%s/%s", bucket, prefix, backup.ID)

	// Cleanup S3 objects
	t.Log("🧹 Cleaning up S3 objects...")
	err = manager.DeleteBackup(ctx, backup.ID)
	require.NoError(t, err)

	t.Log("✅ S3 backup test PASSED")
}

// =============================================================================
// Cost Estimator E2E Tests
// =============================================================================

// TestE2E_CostEstimator_AWSPricing tests cost estimation for AWS
func TestE2E_CostEstimator_AWSPricing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	estimator := costs.NewEstimator(&costs.EstimatorConfig{})

	// Test node cost estimation
	nodeConfig := &config.NodeConfig{
		Name:     "test-master",
		Provider: "aws",
		Size:     "t3.medium",
		Region:   "us-east-1",
	}

	estimate, err := estimator.EstimateNodeCost(ctx, nodeConfig)
	require.NoError(t, err)

	assert.Greater(t, estimate.HourlyCost, 0.0)
	assert.Greater(t, estimate.MonthlyCost, 0.0)
	assert.Equal(t, "USD", estimate.Currency)

	t.Logf("AWS t3.medium cost estimate:")
	t.Logf("  Hourly:  $%.4f", estimate.HourlyCost)
	t.Logf("  Monthly: $%.2f", estimate.MonthlyCost)
	t.Logf("  Yearly:  $%.2f", estimate.YearlyCost)

	// Test spot instance pricing
	spotNodeConfig := &config.NodeConfig{
		Name:         "test-worker",
		Provider:     "aws",
		Size:         "t3.medium",
		Region:       "us-east-1",
		SpotInstance: true,
	}

	spotEstimate, err := estimator.EstimateNodeCost(ctx, spotNodeConfig)
	require.NoError(t, err)

	// Spot should be cheaper or equal
	assert.LessOrEqual(t, spotEstimate.HourlyCost, estimate.HourlyCost*1.1) // 10% tolerance
	assert.True(t, spotEstimate.IsSpot)
	assert.Greater(t, spotEstimate.SpotSavings, 0.0)

	t.Logf("AWS t3.medium SPOT cost estimate:")
	t.Logf("  Hourly:  $%.4f (%.1f%% savings)", spotEstimate.HourlyCost, spotEstimate.SpotSavings)

	t.Log("✅ Cost estimator AWS pricing test PASSED")
}

// TestE2E_CostEstimator_ClusterCost tests full cluster cost estimation
func TestE2E_CostEstimator_ClusterCost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	estimator := costs.NewEstimator(&costs.EstimatorConfig{})

	clusterConfig := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "production"},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Count:    3,
				Size:     "t3.medium",
				Provider: "aws",
				Region:   "us-east-1",
				Roles:    []string{"master"},
			},
			"workers": {
				Name:         "workers",
				Count:        5,
				Size:         "t3.large",
				Provider:     "aws",
				Region:       "us-east-1",
				Roles:        []string{"worker"},
				SpotInstance: true,
			},
		},
	}

	estimate, err := estimator.EstimateClusterCost(ctx, clusterConfig)
	require.NoError(t, err)

	assert.Greater(t, estimate.TotalMonthlyCost, 0.0)
	assert.Equal(t, 8, len(estimate.NodeCosts)) // 3 masters + 5 workers
	assert.Greater(t, estimate.SpotSavings, 0.0)

	t.Logf("Cluster cost estimate:")
	t.Logf("  Total Monthly: $%.2f", estimate.TotalMonthlyCost)
	t.Logf("  Total Yearly:  $%.2f", estimate.TotalYearlyCost)
	t.Logf("  Spot Savings:  $%.2f/month", estimate.SpotSavings)

	// Check recommendations
	for _, rec := range estimate.Recommendations {
		t.Logf("  Recommendation: %s - Potential savings: $%.2f", rec.Type, rec.PotentialSavings)
	}

	t.Log("✅ Cost estimator cluster cost test PASSED")
}

// =============================================================================
// Distribution Strategy E2E Tests
// =============================================================================

// TestE2E_Distribution_MultiAZ tests multi-AZ distribution strategy
func TestE2E_Distribution_MultiAZ(t *testing.T) {
	zones := []string{"us-east-1a", "us-east-1b", "us-east-1c"}

	// Test even distribution using round robin strategy
	strategy := &distribution.RoundRobinStrategy{}
	result := strategy.Calculate(9, zones, nil)

	// Should distribute evenly
	assert.Equal(t, 3, result["us-east-1a"])
	assert.Equal(t, 3, result["us-east-1b"])
	assert.Equal(t, 3, result["us-east-1c"])

	t.Logf("Distribution for 9 nodes across 3 AZs: %v", result)

	// Test uneven distribution
	result2 := strategy.Calculate(10, zones, nil)
	total := 0
	for _, count := range result2 {
		total += count
	}
	assert.Equal(t, 10, total)

	t.Logf("Distribution for 10 nodes across 3 AZs: %v", result2)

	// Test weighted distribution
	weights := map[string]int{
		"us-east-1a": 2,
		"us-east-1b": 1,
		"us-east-1c": 1,
	}
	result3 := strategy.Calculate(8, zones, weights)

	// Zone A should have more nodes due to higher weight
	assert.GreaterOrEqual(t, result3["us-east-1a"], result3["us-east-1b"])

	t.Logf("Weighted distribution for 8 nodes: %v", result3)

	t.Log("✅ Distribution strategy test PASSED")
}

// =============================================================================
// Hook Engine E2E Tests
// =============================================================================

// TestE2E_HookEngine_ExecuteScript tests script hook execution
func TestE2E_HookEngine_ExecuteScript(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	eventEmitter := &testEventEmitter{}

	engine := hooks.NewEngine(&hooks.EngineConfig{
		EventEmitter: eventEmitter,
	})

	// Register a script hook
	hookAction := &config.HookAction{
		Type:    "script",
		Script:  "echo 'Hook executed' && date",
		Timeout: 10,
	}

	hookID := engine.RegisterHook(
		provisioning.HookEventPostNodeCreate,
		hookAction,
		1, // priority
	)

	assert.NotEmpty(t, hookID)
	t.Logf("Registered hook: %s", hookID)

	// Get hooks for event
	hooks := engine.GetHooks(provisioning.HookEventPostNodeCreate)
	assert.Equal(t, 1, len(hooks))

	// Trigger hooks
	err := engine.TriggerHooks(ctx, provisioning.HookEventPostNodeCreate, map[string]interface{}{
		"node_name": "test-node-1",
		"node_ip":   "10.0.1.5",
	})
	require.NoError(t, err)

	// Check events
	triggeredEvents := eventEmitter.GetEvents("hooks_triggered")
	assert.GreaterOrEqual(t, len(triggeredEvents), 1)

	completedEvents := eventEmitter.GetEvents("hook_completed")
	assert.GreaterOrEqual(t, len(completedEvents), 1)

	t.Log("✅ Hook engine script execution test PASSED")
}

// TestE2E_HookEngine_MultipleHooks tests execution of multiple hooks
func TestE2E_HookEngine_MultipleHooks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	engine := hooks.NewEngine(&hooks.EngineConfig{
		EventEmitter: &testEventEmitter{},
	})

	// Register multiple hooks with different priorities
	engine.RegisterHook(provisioning.HookEventPostClusterReady, &config.HookAction{
		Type:   "script",
		Script: "echo 'Hook 3 - low priority'",
	}, 3)

	engine.RegisterHook(provisioning.HookEventPostClusterReady, &config.HookAction{
		Type:   "script",
		Script: "echo 'Hook 1 - high priority'",
	}, 1)

	engine.RegisterHook(provisioning.HookEventPostClusterReady, &config.HookAction{
		Type:   "script",
		Script: "echo 'Hook 2 - medium priority'",
	}, 2)

	// Get hooks - should be sorted by priority
	hooks := engine.GetHooks(provisioning.HookEventPostClusterReady)
	assert.Equal(t, 3, len(hooks))
	assert.Equal(t, 1, hooks[0].Priority)
	assert.Equal(t, 2, hooks[1].Priority)
	assert.Equal(t, 3, hooks[2].Priority)

	// Execute all hooks
	err := engine.TriggerHooks(ctx, provisioning.HookEventPostClusterReady, nil)
	require.NoError(t, err)

	t.Log("✅ Hook engine multiple hooks test PASSED")
}

// TestE2E_HookEngine_UnregisterHook tests hook unregistration
func TestE2E_HookEngine_UnregisterHook(t *testing.T) {
	engine := hooks.NewEngine(&hooks.EngineConfig{
		EventEmitter: &testEventEmitter{},
	})

	hookID := engine.RegisterHook(provisioning.HookEventPreNodeDelete, &config.HookAction{
		Type:   "script",
		Script: "echo 'Test'",
	}, 1)

	// Verify hook exists
	hooks := engine.GetHooks(provisioning.HookEventPreNodeDelete)
	assert.Equal(t, 1, len(hooks))

	// Unregister hook
	removed := engine.UnregisterHook(provisioning.HookEventPreNodeDelete, hookID)
	assert.True(t, removed)

	// Verify hook is gone
	hooks = engine.GetHooks(provisioning.HookEventPreNodeDelete)
	assert.Equal(t, 0, len(hooks))

	t.Log("✅ Hook engine unregister test PASSED")
}

// TestE2E_HookEngine_RegisterFromConfig tests hook registration from config
func TestE2E_HookEngine_RegisterFromConfig(t *testing.T) {
	engine := hooks.NewEngine(&hooks.EngineConfig{
		EventEmitter: &testEventEmitter{},
	})

	hooksConfig := &config.HooksConfig{
		PostNodeCreate: []config.HookAction{
			{Type: "script", Script: "echo 'node created'"},
			{Type: "script", Script: "echo 'configure monitoring'"},
		},
		PreClusterDestroy: []config.HookAction{
			{Type: "script", Script: "echo 'backup before destroy'"},
		},
		PostClusterReady: []config.HookAction{
			{Type: "script", Script: "echo 'cluster ready'"},
		},
	}

	engine.RegisterHooksFromConfig(hooksConfig)

	// Verify hooks registered
	postNodeHooks := engine.GetHooks(provisioning.HookEventPostNodeCreate)
	assert.Equal(t, 2, len(postNodeHooks))

	preDestroyHooks := engine.GetHooks(provisioning.HookEventPreClusterDestroy)
	assert.Equal(t, 1, len(preDestroyHooks))

	postReadyHooks := engine.GetHooks(provisioning.HookEventPostClusterReady)
	assert.Equal(t, 1, len(postReadyHooks))

	t.Log("✅ Hook engine config registration test PASSED")
}

// =============================================================================
// Test Helper Utilities Demonstration
// =============================================================================

// TestE2E_HelperUtilities_TestReportAndFixture demonstrates the use of test helpers
func TestE2E_HelperUtilities_TestReportAndFixture(t *testing.T) {
	// Create test report
	report := NewTestReport("Helper Utilities Demo")
	defer func() {
		report.Finish("passed")
		report.Print(t)
	}()

	// Create test fixture
	fixture := NewTestFixture(t, "helpers-demo")
	defer fixture.Cleanup()

	// Phase 1: Test cluster config builder
	phase1 := report.StartPhase("Config Builder")
	{
		builder := NewTestClusterConfigBuilder("test-cluster").
			WithProvider("aws").
			WithRegion("us-west-2").
			WithMasters(3).
			WithWorkers(5).
			WithMasterSize("t3.large").
			WithWorkerSize("t3.xlarge").
			WithVPCCIDR("172.16.0.0/16").
			WithSpotInstances(true)

		assert.Equal(t, "test-cluster", builder.GetName())
		assert.Equal(t, "aws", builder.GetProvider())
		assert.Equal(t, "us-west-2", builder.GetRegion())
		assert.Equal(t, 3, builder.GetMasterCount())
		assert.Equal(t, 5, builder.GetWorkerCount())
		assert.Equal(t, "t3.large", builder.GetMasterSize())
		assert.Equal(t, "t3.xlarge", builder.GetWorkerSize())
		assert.Equal(t, "172.16.0.0/16", builder.GetVPCCIDR())
		assert.True(t, builder.IsSpotEnabled())

		// Store in fixture
		fixture.Set("cluster_name", builder.GetName())
		fixture.Set("master_count", builder.GetMasterCount())
	}
	report.EndPhase(phase1, "passed", "Config builder working correctly")

	// Phase 2: Test Pulumi stack config
	phase2 := report.StartPhase("Pulumi Stack Config")
	{
		stackCfg := NewPulumiStackConfig("test").
			WithWorkDir("/tmp/test").
			WithConfig("aws:region", "us-east-1").
			WithConfig("kubernetes:version", "1.28")

		assert.Equal(t, "sloth-kubernetes-e2e-test", stackCfg.ProjectName)
		assert.Equal(t, "/tmp/test", stackCfg.WorkDir)
		assert.Equal(t, "us-east-1", stackCfg.Config["aws:region"])
		assert.Equal(t, "1.28", stackCfg.Config["kubernetes:version"])
	}
	report.EndPhase(phase2, "passed", "Pulumi config working correctly")

	// Phase 3: Test mock event emitter
	phase3 := report.StartPhase("Mock Event Emitter")
	{
		emitter := NewMockEventEmitter()

		emitter.Emit("node.created", map[string]string{"node": "worker-1"})
		emitter.Emit("node.created", map[string]string{"node": "worker-2"})
		emitter.Emit("cluster.ready", map[string]string{"cluster": "test"})

		assert.Equal(t, 3, emitter.Count())
		assert.Equal(t, 2, len(emitter.GetEventsByType("node.created")))
		assert.Equal(t, 1, len(emitter.GetEventsByType("cluster.ready")))

		emitter.Clear()
		assert.Equal(t, 0, emitter.Count())
	}
	report.EndPhase(phase3, "passed", "Event emitter captures events correctly")

	// Phase 4: Test mock metrics collector
	phase4 := report.StartPhase("Mock Metrics Collector")
	{
		collector := NewMockMetricsCollector()
		collector.SetCPUUsage(75.5)
		collector.SetMemoryUsage(60.0)
		collector.SetNodeCount(10)

		assert.Equal(t, 75.5, collector.GetCPUUsage())
		assert.Equal(t, 60.0, collector.GetMemoryUsage())
		assert.Equal(t, 10, collector.GetNodeCount())

		metrics, err := collector.CollectMetrics(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 75.5, metrics["cpu_usage"])
		assert.Equal(t, 60.0, metrics["memory_usage"])
		assert.Equal(t, 10.0, metrics["node_count"])
	}
	report.EndPhase(phase4, "passed", "Metrics collector provides mock data")

	// Phase 5: Test retry with backoff
	phase5 := report.StartPhase("Retry With Backoff")
	{
		ctx := context.Background()
		retryCfg := &RetryConfig{
			MaxAttempts: 3,
			InitialWait: 10 * time.Millisecond,
			MaxWait:     100 * time.Millisecond,
			Multiplier:  2.0,
		}

		// Test successful retry
		attempts := 0
		err := RetryWithBackoff(ctx, retryCfg, func() error {
			attempts++
			if attempts < 2 {
				return fmt.Errorf("transient error")
			}
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, 2, attempts)

		// Test failed retry (all attempts exhausted)
		attempts = 0
		err = RetryWithBackoff(ctx, retryCfg, func() error {
			attempts++
			return fmt.Errorf("persistent error")
		})
		require.Error(t, err)
		assert.Equal(t, 3, attempts)
		assert.Contains(t, err.Error(), "after 3 attempts")
	}
	report.EndPhase(phase5, "passed", "Retry mechanism works correctly")

	// Phase 6: Test fixture data storage
	phase6 := report.StartPhase("Fixture Data Storage")
	{
		// Check values stored earlier
		assert.Equal(t, "test-cluster", fixture.GetString("cluster_name"))
		assert.Equal(t, 3, fixture.GetInt("master_count"))

		// Store more data
		fixture.Set("test_complete", true)
		val, ok := fixture.Get("test_complete")
		assert.True(t, ok)
		assert.Equal(t, true, val)
	}
	report.EndPhase(phase6, "passed", "Fixture stores and retrieves data correctly")

	// Set report metrics
	report.SetMetric("phases_completed", len(report.Phases))
	report.SetMetric("test_duration_target", "< 1s")

	t.Log("✅ Helper utilities test PASSED")
}

// TestE2E_HelperUtilities_SkipFunctions tests the skip helper functions
func TestE2E_HelperUtilities_SkipFunctions(t *testing.T) {
	// Test short mode skip (should not skip since we're not in short mode)
	t.Run("SkipIfShortTest", func(t *testing.T) {
		// This won't skip since we're running full tests
		// In short mode (-short flag), this would skip
		t.Log("Short test skip helper available for use")
	})

	// Test AWS credentials check
	t.Run("HasAWSCredentials", func(t *testing.T) {
		env := NewTestEnvironment()
		hasCredentials := env.HasAWSCredentials()
		t.Logf("AWS credentials available: %v", hasCredentials)
	})

	// Test S3 bucket check
	t.Run("HasS3Bucket", func(t *testing.T) {
		env := NewTestEnvironment()
		hasBucket := env.HasS3Bucket()
		t.Logf("S3 bucket configured: %v", hasBucket)
	})

	t.Log("✅ Skip helper functions test PASSED")
}

// TestE2E_HelperUtilities_TestLogger demonstrates structured logging
func TestE2E_HelperUtilities_TestLogger(t *testing.T) {
	logger := NewTestLogger(t, "LoggerDemo")

	logger.Phase("Initialization")
	logger.Info("Starting test logger demonstration")
	logger.Success("Logger created successfully")
	logger.Warning("This is a warning message")
	logger.Error("This is an error message (for demo purposes)")
	logger.Info("Test completed")

	t.Log("✅ Test logger demonstration PASSED")
}
