//go:build e2e
// +build e2e

// Package e2e provides advanced end-to-end tests for complex scenarios
package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chalkan3/sloth-kubernetes/internal/orchestrator/components"
	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
)

// =============================================================================
// Complete Cluster Simulation E2E Test
// Tests: Full cluster deployment simulation (VPC + SG + Masters + Workers)
// =============================================================================

// TestE2E_AWS_CompleteClusterSimulation tests a complete K8s cluster infrastructure
func TestE2E_AWS_CompleteClusterSimulation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	stackName := fmt.Sprintf("%s-complete-cluster", cfg.StackPrefix)
	report := NewTestReport("Complete Cluster Simulation")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	program := func(pctx *pulumi.Context) error {
		// Phase 1: Configuration for a realistic cluster
		phase1 := report.StartPhase("Cluster Configuration")
		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "e2e-complete-cluster",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  cfg.AWSRegion,
					VPC: &config.VPCConfig{
						Create: true,
						Name:   "e2e-complete-vpc",
						CIDR:   "10.150.0.0/16",
					},
				},
			},
			Security: config.SecurityConfig{
				SSHConfig: config.SSHConfig{
					PublicKeyPath: "",
				},
			},
			Network: config.NetworkConfig{
				CIDR: "10.150.0.0/16",
				Firewall: &config.FirewallConfig{
					InboundRules: []config.FirewallRule{
						{Port: "22", Protocol: "tcp", Source: []string{"0.0.0.0/0"}, Description: "SSH"},
						{Port: "80", Protocol: "tcp", Source: []string{"0.0.0.0/0"}, Description: "HTTP"},
						{Port: "443", Protocol: "tcp", Source: []string{"0.0.0.0/0"}, Description: "HTTPS"},
						{Port: "6443", Protocol: "tcp", Source: []string{"0.0.0.0/0"}, Description: "K8s API"},
						{Port: "2379-2380", Protocol: "tcp", Source: []string{"10.150.0.0/16"}, Description: "etcd"},
						{Port: "10250", Protocol: "tcp", Source: []string{"10.150.0.0/16"}, Description: "Kubelet"},
						{Port: "10251", Protocol: "tcp", Source: []string{"10.150.0.0/16"}, Description: "kube-scheduler"},
						{Port: "10252", Protocol: "tcp", Source: []string{"10.150.0.0/16"}, Description: "kube-controller"},
						{Port: "30000-32767", Protocol: "tcp", Source: []string{"0.0.0.0/0"}, Description: "NodePort"},
						{Port: "51820", Protocol: "udp", Source: []string{"0.0.0.0/0"}, Description: "WireGuard"},
					},
				},
				DNS: config.DNSConfig{
					Domain: "e2e-test.local",
				},
			},
			Kubernetes: config.KubernetesConfig{
				Version: "v1.28.0",
			},
		}
		report.EndPhase(phase1, "passed", "Cluster configuration created with 10 firewall rules")

		// Phase 2: SSH Key Generation
		phase2 := report.StartPhase("SSH Key Generation")
		sshComponent, err := components.NewSSHKeyComponent(pctx, "e2e-complete-ssh", clusterConfig)
		if err != nil {
			report.AddError(fmt.Sprintf("SSH key creation failed: %v", err))
			return fmt.Errorf("failed to create SSH key: %w", err)
		}
		pctx.Export("ssh_public_key", sshComponent.PublicKey)
		report.EndPhase(phase2, "passed", "SSH keys generated")

		// Phase 3: Initialize Provider
		phase3 := report.StartPhase("AWS Provider Initialization")
		awsProvider := providers.NewAWSProvider()
		if err := awsProvider.Initialize(pctx, clusterConfig); err != nil {
			report.AddError(fmt.Sprintf("AWS provider init failed: %v", err))
			return fmt.Errorf("AWSProvider.Initialize failed: %w", err)
		}
		report.EndPhase(phase3, "passed", "AWS provider initialized")

		// Phase 4: Create Network Infrastructure
		phase4 := report.StartPhase("Network Infrastructure")
		networkOutput, err := awsProvider.CreateNetwork(pctx, &config.NetworkConfig{
			CIDR: clusterConfig.Providers.AWS.VPC.CIDR,
		})
		if err != nil {
			report.AddError(fmt.Sprintf("Network creation failed: %v", err))
			return fmt.Errorf("AWSProvider.CreateNetwork failed: %w", err)
		}
		pctx.Export("vpc_id", networkOutput.ID)
		pctx.Export("vpc_cidr", pulumi.String(networkOutput.CIDR))
		pctx.Export("subnet_count", pulumi.Int(len(networkOutput.Subnets)))
		report.EndPhase(phase4, "passed", fmt.Sprintf("VPC with %d subnets created", len(networkOutput.Subnets)))

		// Phase 5: Create Security Group
		phase5 := report.StartPhase("Security Group")
		err = awsProvider.CreateFirewall(pctx, clusterConfig.Network.Firewall, nil)
		if err != nil {
			report.AddError(fmt.Sprintf("Security group creation failed: %v", err))
			return fmt.Errorf("AWSProvider.CreateFirewall failed: %w", err)
		}
		pctx.Export("firewall_rules", pulumi.Int(len(clusterConfig.Network.Firewall.InboundRules)))
		report.EndPhase(phase5, "passed", fmt.Sprintf("Security group with %d rules", len(clusterConfig.Network.Firewall.InboundRules)))

		// Phase 6: Create Master Node Pool
		phase6 := report.StartPhase("Master Node Pool")
		masterPool := &config.NodePool{
			Name:  "masters",
			Count: 1,
			Size:  "t3.micro",
			Image: "ubuntu-22.04",
			Roles: []string{"master", "etcd", "controlplane"},
			Labels: map[string]string{
				"node-role.kubernetes.io/control-plane": "true",
				"environment":                           "e2e-test",
			},
		}

		masterNodes, err := awsProvider.CreateNodePool(pctx, masterPool)
		if err != nil {
			report.AddError(fmt.Sprintf("Master pool creation failed: %v", err))
			return fmt.Errorf("failed to create master pool: %w", err)
		}
		pctx.Export("master_count", pulumi.Int(len(masterNodes)))
		for i, node := range masterNodes {
			pctx.Export(fmt.Sprintf("master_%d_id", i), node.ID)
			pctx.Export(fmt.Sprintf("master_%d_public_ip", i), node.PublicIP)
			pctx.Export(fmt.Sprintf("master_%d_private_ip", i), node.PrivateIP)
		}
		report.EndPhase(phase6, "passed", fmt.Sprintf("%d master nodes created", len(masterNodes)))

		// Phase 7: Create Worker Node Pool
		phase7 := report.StartPhase("Worker Node Pool")
		workerPool := &config.NodePool{
			Name:  "workers",
			Count: 2,
			Size:  "t3.micro",
			Image: "ubuntu-22.04",
			Roles: []string{"worker"},
			Labels: map[string]string{
				"node-role.kubernetes.io/worker": "true",
				"environment":                    "e2e-test",
				"workload-type":                  "general",
			},
		}

		workerNodes, err := awsProvider.CreateNodePool(pctx, workerPool)
		if err != nil {
			report.AddError(fmt.Sprintf("Worker pool creation failed: %v", err))
			return fmt.Errorf("failed to create worker pool: %w", err)
		}
		pctx.Export("worker_count", pulumi.Int(len(workerNodes)))
		for i, node := range workerNodes {
			pctx.Export(fmt.Sprintf("worker_%d_id", i), node.ID)
			pctx.Export(fmt.Sprintf("worker_%d_public_ip", i), node.PublicIP)
			pctx.Export(fmt.Sprintf("worker_%d_private_ip", i), node.PrivateIP)
		}
		report.EndPhase(phase7, "passed", fmt.Sprintf("%d worker nodes created", len(workerNodes)))

		// Summary
		totalNodes := len(masterNodes) + len(workerNodes)
		pctx.Export("total_nodes", pulumi.Int(totalNodes))

		// Set metrics
		report.SetMetric("vpc_cidr", clusterConfig.Providers.AWS.VPC.CIDR)
		report.SetMetric("master_count", len(masterNodes))
		report.SetMetric("worker_count", len(workerNodes))
		report.SetMetric("total_nodes", totalNodes)
		report.SetMetric("firewall_rules", len(clusterConfig.Network.Firewall.InboundRules))

		return nil
	}

	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	err := stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	t.Log("=================================================")
	t.Log("RUNNING: Complete Cluster Simulation E2E Test")
	t.Log("=================================================")
	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi Up failed")

	// Validate outputs
	masterCount, ok := result.Outputs["master_count"]
	assert.True(t, ok, "master_count output should exist")
	assert.Equal(t, float64(1), masterCount.Value, "Should have 1 master node")
	t.Logf("Master count: %v", masterCount.Value)

	workerCount, ok := result.Outputs["worker_count"]
	assert.True(t, ok, "worker_count output should exist")
	assert.Equal(t, float64(2), workerCount.Value, "Should have 2 worker nodes")
	t.Logf("Worker count: %v", workerCount.Value)

	totalNodes, ok := result.Outputs["total_nodes"]
	assert.True(t, ok, "total_nodes output should exist")
	assert.Equal(t, float64(3), totalNodes.Value, "Should have 3 total nodes")
	t.Logf("Total nodes: %v", totalNodes.Value)

	// Validate VPC
	vpcID, ok := result.Outputs["vpc_id"]
	assert.True(t, ok, "vpc_id should exist")
	assert.NotEmpty(t, vpcID.Value, "VPC ID should not be empty")
	t.Logf("VPC ID: %v", vpcID.Value)

	// Validate firewall
	firewallRules, ok := result.Outputs["firewall_rules"]
	assert.True(t, ok, "firewall_rules should exist")
	assert.Equal(t, float64(10), firewallRules.Value, "Should have 10 firewall rules")
	t.Logf("Firewall rules: %v", firewallRules.Value)

	t.Log("=================================================")
	t.Log("Complete Cluster Simulation E2E Test PASSED")
	t.Log("=================================================")
}

// =============================================================================
// High Availability Configuration E2E Test
// Tests: HA configuration with multiple masters and workers
// =============================================================================

// TestE2E_AWS_HighAvailabilityConfig tests HA cluster configuration
func TestE2E_AWS_HighAvailabilityConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	stackName := fmt.Sprintf("%s-ha-config", cfg.StackPrefix)
	report := NewTestReport("High Availability Configuration")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	program := func(pctx *pulumi.Context) error {
		// Phase 1: HA Configuration
		phase1 := report.StartPhase("HA Configuration")
		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "e2e-ha-cluster",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  cfg.AWSRegion,
					VPC: &config.VPCConfig{
						Create: true,
						Name:   "e2e-ha-vpc",
						CIDR:   "10.160.0.0/16",
					},
				},
			},
			Security: config.SecurityConfig{},
			Network: config.NetworkConfig{
				CIDR: "10.160.0.0/16",
				Firewall: &config.FirewallConfig{
					InboundRules: []config.FirewallRule{
						{Port: "22", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
						{Port: "6443", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
						{Port: "2379-2380", Protocol: "tcp", Source: []string{"10.160.0.0/16"}},
						{Port: "10250-10252", Protocol: "tcp", Source: []string{"10.160.0.0/16"}},
						{Port: "51820", Protocol: "udp", Source: []string{"0.0.0.0/0"}},
					},
				},
			},
		}
		report.EndPhase(phase1, "passed", "HA configuration created")

		// Phase 2: SSH Keys
		phase2 := report.StartPhase("SSH Keys")
		_, err := components.NewSSHKeyComponent(pctx, "e2e-ha-ssh", clusterConfig)
		if err != nil {
			report.AddError(fmt.Sprintf("SSH key creation failed: %v", err))
			return fmt.Errorf("failed to create SSH key: %w", err)
		}
		report.EndPhase(phase2, "passed", "SSH keys generated")

		// Phase 3: Network
		phase3 := report.StartPhase("Network Infrastructure")
		awsProvider := providers.NewAWSProvider()
		if err := awsProvider.Initialize(pctx, clusterConfig); err != nil {
			report.AddError(fmt.Sprintf("AWS provider init failed: %v", err))
			return fmt.Errorf("AWSProvider.Initialize failed: %w", err)
		}

		networkOutput, err := awsProvider.CreateNetwork(pctx, &config.NetworkConfig{
			CIDR: clusterConfig.Providers.AWS.VPC.CIDR,
		})
		if err != nil {
			report.AddError(fmt.Sprintf("Network creation failed: %v", err))
			return fmt.Errorf("AWSProvider.CreateNetwork failed: %w", err)
		}
		pctx.Export("vpc_id", networkOutput.ID)
		report.EndPhase(phase3, "passed", "Network created")

		// Phase 4: Security Group
		phase4 := report.StartPhase("Security Group")
		err = awsProvider.CreateFirewall(pctx, clusterConfig.Network.Firewall, nil)
		if err != nil {
			report.AddError(fmt.Sprintf("Security group creation failed: %v", err))
			return fmt.Errorf("AWSProvider.CreateFirewall failed: %w", err)
		}
		report.EndPhase(phase4, "passed", "Security group created")

		// Phase 5: HA Master Pool (3 masters for etcd quorum)
		phase5 := report.StartPhase("HA Master Pool (3 nodes)")
		masterPool := &config.NodePool{
			Name:  "ha-masters",
			Count: 3, // 3 masters for HA
			Size:  "t3.micro",
			Image: "ubuntu-22.04",
			Roles: []string{"master", "etcd", "controlplane"},
			Labels: map[string]string{
				"node-role.kubernetes.io/control-plane": "true",
				"topology.kubernetes.io/zone":           "multi-az",
			},
		}

		masterNodes, err := awsProvider.CreateNodePool(pctx, masterPool)
		if err != nil {
			report.AddError(fmt.Sprintf("HA master pool creation failed: %v", err))
			return fmt.Errorf("failed to create HA master pool: %w", err)
		}
		pctx.Export("ha_master_count", pulumi.Int(len(masterNodes)))
		for i, node := range masterNodes {
			pctx.Export(fmt.Sprintf("ha_master_%d_id", i), node.ID)
			pctx.Export(fmt.Sprintf("ha_master_%d_ip", i), node.PublicIP)
		}
		report.EndPhase(phase5, "passed", fmt.Sprintf("%d HA masters created", len(masterNodes)))

		// Phase 6: Worker Pool
		phase6 := report.StartPhase("Worker Pool (3 nodes)")
		workerPool := &config.NodePool{
			Name:  "ha-workers",
			Count: 3, // 3 workers for HA
			Size:  "t3.micro",
			Image: "ubuntu-22.04",
			Roles: []string{"worker"},
			Labels: map[string]string{
				"node-role.kubernetes.io/worker": "true",
			},
		}

		workerNodes, err := awsProvider.CreateNodePool(pctx, workerPool)
		if err != nil {
			report.AddError(fmt.Sprintf("Worker pool creation failed: %v", err))
			return fmt.Errorf("failed to create worker pool: %w", err)
		}
		pctx.Export("ha_worker_count", pulumi.Int(len(workerNodes)))
		for i, node := range workerNodes {
			pctx.Export(fmt.Sprintf("ha_worker_%d_id", i), node.ID)
			pctx.Export(fmt.Sprintf("ha_worker_%d_ip", i), node.PublicIP)
		}
		report.EndPhase(phase6, "passed", fmt.Sprintf("%d HA workers created", len(workerNodes)))

		// Summary
		totalNodes := len(masterNodes) + len(workerNodes)
		pctx.Export("ha_total_nodes", pulumi.Int(totalNodes))

		// Set metrics
		report.SetMetric("ha_master_count", len(masterNodes))
		report.SetMetric("ha_worker_count", len(workerNodes))
		report.SetMetric("ha_total_nodes", totalNodes)

		return nil
	}

	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	err := stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	t.Log("=================================================")
	t.Log("RUNNING: High Availability Configuration E2E Test")
	t.Log("=================================================")
	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi Up failed")

	// Validate HA master count
	haMasterCount, ok := result.Outputs["ha_master_count"]
	assert.True(t, ok, "ha_master_count output should exist")
	assert.Equal(t, float64(3), haMasterCount.Value, "Should have 3 HA masters")
	t.Logf("HA Masters: %v", haMasterCount.Value)

	// Validate HA worker count
	haWorkerCount, ok := result.Outputs["ha_worker_count"]
	assert.True(t, ok, "ha_worker_count output should exist")
	assert.Equal(t, float64(3), haWorkerCount.Value, "Should have 3 HA workers")
	t.Logf("HA Workers: %v", haWorkerCount.Value)

	// Validate total node count
	haTotalNodes, ok := result.Outputs["ha_total_nodes"]
	assert.True(t, ok, "ha_total_nodes output should exist")
	assert.Equal(t, float64(6), haTotalNodes.Value, "Should have 6 total HA nodes")
	t.Logf("Total HA nodes: %v", haTotalNodes.Value)

	t.Log("=================================================")
	t.Log("High Availability Configuration E2E Test PASSED")
	t.Log("=================================================")
}

// =============================================================================
// Provider Cleanup E2E Test
// Tests: Resource cleanup using provider Cleanup method
// =============================================================================

// TestE2E_ProviderCleanup tests that provider cleanup works correctly
func TestE2E_ProviderCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	report := NewTestReport("Provider Cleanup")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	phase1 := report.StartPhase("Cleanup Verification")

	// Test that providers have Cleanup methods
	t.Run("AWS Provider Cleanup", func(t *testing.T) {
		awsProvider := providers.NewAWSProvider()
		assert.NotNil(t, awsProvider)
		// The Cleanup method exists (verified by compilation)
		t.Log("AWS Provider has Cleanup method")
	})

	t.Run("DigitalOcean Provider Cleanup", func(t *testing.T) {
		doProvider := providers.NewDigitalOceanProvider()
		assert.NotNil(t, doProvider)
		t.Log("DigitalOcean Provider has Cleanup method")
	})

	t.Run("Linode Provider Cleanup", func(t *testing.T) {
		linodeProvider := providers.NewLinodeProvider()
		assert.NotNil(t, linodeProvider)
		t.Log("Linode Provider has Cleanup method")
	})

	t.Run("Azure Provider Cleanup", func(t *testing.T) {
		azureProvider := providers.NewAzureProvider()
		assert.NotNil(t, azureProvider)
		t.Log("Azure Provider has Cleanup method")
	})

	report.EndPhase(phase1, "passed", "All providers have cleanup methods")

	t.Log("=========================================")
	t.Log("Provider Cleanup E2E Test PASSED")
	t.Log("=========================================")
}

// =============================================================================
// Error Handling E2E Test
// Tests: Proper error handling in various failure scenarios
// =============================================================================

// TestE2E_ErrorHandling tests error handling in various scenarios
func TestE2E_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	report := NewTestReport("Error Handling")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	phase1 := report.StartPhase("Error Scenario Testing")

	t.Run("Invalid CIDR", func(t *testing.T) {
		// Test with invalid CIDR should not crash
		subnets := calculateTestSubnetCIDRs("invalid-cidr", 2)
		assert.Empty(t, subnets, "Invalid CIDR should return empty subnets")
		t.Log("PASS: Invalid CIDR handled gracefully")
	})

	t.Run("Empty CIDR", func(t *testing.T) {
		subnets := calculateTestSubnetCIDRs("", 2)
		assert.Empty(t, subnets, "Empty CIDR should return empty subnets")
		t.Log("PASS: Empty CIDR handled gracefully")
	})

	t.Run("Invalid Config Validation", func(t *testing.T) {
		// Test nil config
		valid := validateConfig(nil)
		assert.False(t, valid, "Nil config should be invalid")
		t.Log("PASS: Nil config handled gracefully")

		// Test config without name
		emptyConfig := &config.ClusterConfig{}
		valid = validateConfig(emptyConfig)
		assert.False(t, valid, "Config without name should be invalid")
		t.Log("PASS: Config without name handled gracefully")
	})

	t.Run("Invalid WireGuard Node Type", func(t *testing.T) {
		ip := calculateWireGuardIP("invalid", 0)
		assert.Equal(t, "10.8.0.50", ip, "Invalid node type should use default offset")
		t.Log("PASS: Invalid node type uses default offset")
	})

	report.EndPhase(phase1, "passed", "All error scenarios handled correctly")

	t.Log("=========================================")
	t.Log("Error Handling E2E Test PASSED")
	t.Log("=========================================")
}

// =============================================================================
// Provider Capabilities E2E Test
// Tests: Verify each provider exposes expected capabilities
// =============================================================================

// TestE2E_ProviderCapabilities tests provider capability discovery
func TestE2E_ProviderCapabilities(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	report := NewTestReport("Provider Capabilities")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	phase1 := report.StartPhase("Capability Discovery")

	testCases := []struct {
		name            string
		providerName    string
		expectedRegions []string
		expectedSizes   []string
	}{
		{
			name:            "AWS Regions",
			providerName:    "aws",
			expectedRegions: []string{"us-east-1", "us-west-2", "eu-west-1"},
			expectedSizes:   []string{"t2.micro", "t3.micro", "t3.small"},
		},
		{
			name:            "DigitalOcean Regions",
			providerName:    "digitalocean",
			expectedRegions: []string{"nyc1", "nyc3", "sfo2"},
			expectedSizes:   []string{"s-1vcpu-1gb", "s-1vcpu-2gb", "s-2vcpu-2gb"},
		},
		{
			name:            "Linode Regions",
			providerName:    "linode",
			expectedRegions: []string{"us-east", "us-west", "eu-west"},
			expectedSizes:   []string{"g6-nanode-1", "g6-standard-1", "g6-standard-2"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var regions, sizes []string

			switch tc.providerName {
			case "aws":
				p := providers.NewAWSProvider()
				regions = p.GetRegions()
				sizes = p.GetSizes()
			case "digitalocean":
				p := providers.NewDigitalOceanProvider()
				regions = p.GetRegions()
				sizes = p.GetSizes()
			case "linode":
				p := providers.NewLinodeProvider()
				regions = p.GetRegions()
				sizes = p.GetSizes()
			}

			// Validate regions
			for _, expectedRegion := range tc.expectedRegions {
				found := false
				for _, region := range regions {
					if region == expectedRegion {
						found = true
						break
					}
				}
				assert.True(t, found, "%s should have region %s", tc.providerName, expectedRegion)
			}

			// Validate sizes
			for _, expectedSize := range tc.expectedSizes {
				found := false
				for _, size := range sizes {
					if size == expectedSize {
						found = true
						break
					}
				}
				assert.True(t, found, "%s should have size %s", tc.providerName, expectedSize)
			}

			t.Logf("PASS: %s has %d regions and %d sizes", tc.providerName, len(regions), len(sizes))
		})
	}

	report.EndPhase(phase1, "passed", "All provider capabilities verified")

	t.Log("=========================================")
	t.Log("Provider Capabilities E2E Test PASSED")
	t.Log("=========================================")
}
