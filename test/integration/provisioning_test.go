//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/autoscaling"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/costs"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/distribution"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/hooks"
)

func TestCostEstimator_RealCluster(t *testing.T) {
	estimator := costs.NewEstimator(&costs.EstimatorConfig{})

	clusterConfig := &config.ClusterConfig{
		Metadata: config.Metadata{
			Name:        "test-cluster",
			Environment: "test",
		},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Provider: "aws",
				Count:    3,
				Size:     "t3.medium",
				Region:   "us-east-1",
				Roles:    []string{"master", "etcd"},
			},
			"workers": {
				Name:         "workers",
				Provider:     "aws",
				Count:        3,
				Size:         "t3.large",
				Region:       "us-east-1",
				Roles:        []string{"worker"},
				SpotInstance: true,
			},
		},
		LoadBalancer: config.LoadBalancerConfig{
			Provider: "aws",
			Type:     "nlb",
		},
	}

	ctx := context.Background()
	estimate, err := estimator.EstimateClusterCost(ctx, clusterConfig)
	if err != nil {
		t.Fatalf("Failed to estimate cluster cost: %v", err)
	}

	fmt.Printf("\n=== Cluster Cost Estimation ===\n")
	fmt.Printf("Total Monthly Cost: $%.2f\n", estimate.TotalMonthlyCost)
	fmt.Printf("Total Yearly Cost: $%.2f\n", estimate.TotalYearlyCost)
	fmt.Printf("Spot Savings: %.1f%%\n", estimate.SpotSavings)
	fmt.Printf("\nNode Breakdown:\n")
	for _, node := range estimate.NodeCosts {
		spotStatus := ""
		if node.IsSpot {
			spotStatus = " (SPOT)"
		}
		fmt.Printf("  - %s: $%.2f/month%s\n", node.Resource, node.MonthlyCost, spotStatus)
	}
	fmt.Printf("\nRecommendations:\n")
	for _, rec := range estimate.Recommendations {
		fmt.Printf("  - [%s] %s - Potential savings: $%.2f\n", rec.Type, rec.Description, rec.PotentialSavings)
	}
}

func TestZoneDistribution_MultiAZ(t *testing.T) {
	cfg := &distribution.DistributorConfig{
		StrategyName: "spread",
	}

	distributor, err := distribution.NewDistributor(cfg)
	if err != nil {
		t.Fatalf("Failed to create distributor: %v", err)
	}

	zones := []string{"us-east-1a", "us-east-1b", "us-east-1c"}

	// Test with 5 workers across 3 AZs
	result, err := distributor.Distribute(context.Background(), 5, zones)
	if err != nil {
		t.Fatalf("Failed to distribute: %v", err)
	}

	fmt.Printf("\n=== Multi-AZ Distribution ===\n")
	fmt.Printf("Strategy: spread\n")
	fmt.Printf("Total nodes: 5\n")
	fmt.Printf("Distribution:\n")
	for zone, count := range result {
		fmt.Printf("  - %s: %d nodes\n", zone, count)
	}

	// Verify total
	total := 0
	for _, count := range result {
		total += count
	}
	if total != 5 {
		t.Errorf("Expected 5 total nodes, got %d", total)
	}

	// Verify spread (each zone should have at least 1 node)
	for zone, count := range result {
		if count < 1 {
			t.Errorf("Zone %s should have at least 1 node", zone)
		}
	}
}

func TestAutoScalingStrategies(t *testing.T) {
	registry := autoscaling.NewStrategyRegistry()

	strategies := []string{"cpu", "memory", "composite"}

	fmt.Printf("\n=== Auto-Scaling Strategies ===\n")
	for _, name := range strategies {
		strategy, err := registry.Get(name)
		if err != nil {
			t.Errorf("Failed to get strategy %s: %v", name, err)
			continue
		}
		fmt.Printf("✓ Strategy '%s' loaded successfully\n", strategy.Name())
	}
}

func TestHookEngine(t *testing.T) {
	engine := hooks.NewEngine(&hooks.EngineConfig{})

	// Register a test hook
	testHookAction := &config.HookAction{
		Type:    "script",
		Command: "echo 'Hook executed successfully'",
		Timeout: 30,
	}

	hookID := engine.RegisterHook(provisioning.HookEventPostClusterReady, testHookAction, 0)

	fmt.Printf("\n=== Hook Engine ===\n")
	fmt.Printf("Registered hook: %s\n", hookID)

	// Get registered hooks
	registeredHooks := engine.GetHooks(provisioning.HookEventPostClusterReady)
	fmt.Printf("Total hooks for PostClusterReady: %d\n", len(registeredHooks))

	// List predefined hooks
	templates := hooks.GetPredefinedHooks()
	fmt.Printf("\nAvailable hook templates:\n")
	for _, tmpl := range templates {
		fmt.Printf("  - %s: %s\n", tmpl.Name, tmpl.Description)
	}
}

func TestProvisioningManager(t *testing.T) {
	clusterConfig := &config.ClusterConfig{
		Metadata: config.Metadata{
			Name: "test-cluster",
		},
		Backup: &config.BackupConfig{
			Enabled:  true,
			Schedule: "0 2 * * *",
		},
	}

	manager, err := provisioning.NewManagerBuilder(clusterConfig).Build()
	if err != nil {
		t.Fatalf("Failed to create provisioning manager: %v", err)
	}

	fmt.Printf("\n=== Provisioning Manager ===\n")
	fmt.Printf("✓ Manager created successfully\n")

	// Get event emitter
	emitter := manager.GetEventEmitter()
	if emitter != nil {
		fmt.Printf("✓ Event emitter available\n")
	}
}

func TestEventSystem(t *testing.T) {
	emitter := provisioning.NewEventEmitter()

	received := make(chan bool, 1)

	// Subscribe to events
	subID := emitter.Subscribe("test_event", func(event provisioning.Event) {
		fmt.Printf("  Received event: %s (source: %s)\n", event.Type, event.Source)
		received <- true
	})

	fmt.Printf("\n=== Event System ===\n")
	fmt.Printf("Subscription ID: %s\n", subID)

	// Emit an event
	emitter.Emit(provisioning.Event{
		Type:      "test_event",
		Timestamp: time.Now().Unix(),
		Data:      map[string]interface{}{"test": true},
		Source:    "integration_test",
	})

	// Wait for event
	select {
	case <-received:
		fmt.Printf("✓ Event system working correctly\n")
	case <-time.After(time.Second):
		t.Error("Event was not received within timeout")
	}
}
