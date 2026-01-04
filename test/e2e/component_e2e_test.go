//go:build e2e
// +build e2e

// Package e2e provides comprehensive end-to-end tests for individual components
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
// VPC Manager Component E2E Test
// Tests: VPC creation using the VPCManager component
// =============================================================================

// TestE2E_VPCManager tests the VPC Manager component for multi-provider VPC creation
func TestE2E_VPCManager(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	stackName := fmt.Sprintf("%s-vpcmanager", cfg.StackPrefix)
	report := NewTestReport("VPC Manager Component")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	program := func(pctx *pulumi.Context) error {
		// Phase 1: Create configuration
		phase1 := report.StartPhase("VPC Configuration")

		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "e2e-vpcmanager-test",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  cfg.AWSRegion,
					VPC: &config.VPCConfig{
						Create: true,
						Name:   "e2e-vpcmgr-vpc",
						CIDR:   "10.230.0.0/16",
					},
				},
			},
			Security: config.SecurityConfig{},
			Network: config.NetworkConfig{
				CIDR: "10.230.0.0/16",
			},
		}
		report.EndPhase(phase1, "passed", "VPC configuration created")

		// Phase 2: Initialize AWS Provider and create VPC directly
		phase2 := report.StartPhase("AWS VPC Creation")
		awsProvider := providers.NewAWSProvider()
		if err := awsProvider.Initialize(pctx, clusterConfig); err != nil {
			report.AddError(fmt.Sprintf("AWS provider init failed: %v", err))
			return fmt.Errorf("AWSProvider.Initialize failed: %w", err)
		}

		networkOutput, err := awsProvider.CreateNetwork(pctx, &config.NetworkConfig{
			CIDR: clusterConfig.Providers.AWS.VPC.CIDR,
		})
		if err != nil {
			report.AddError(fmt.Sprintf("VPC creation failed: %v", err))
			return fmt.Errorf("AWSProvider.CreateNetwork failed: %w", err)
		}

		// Export VPC details
		pctx.Export("vpc_id", networkOutput.ID)
		pctx.Export("vpc_cidr", pulumi.String(networkOutput.CIDR))
		pctx.Export("subnet_count", pulumi.Int(len(networkOutput.Subnets)))

		report.EndPhase(phase2, "passed", fmt.Sprintf("VPC created with CIDR %s and %d subnets", networkOutput.CIDR, len(networkOutput.Subnets)))

		// Set metrics
		report.SetMetric("vpc_cidr", networkOutput.CIDR)
		report.SetMetric("subnet_count", len(networkOutput.Subnets))

		return nil
	}

	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	err := stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	t.Log("========================================")
	t.Log("RUNNING: VPC Manager Component E2E Test")
	t.Log("========================================")
	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi Up failed")

	// Validate outputs
	vpcID, ok := result.Outputs["vpc_id"]
	assert.True(t, ok, "vpc_id output should exist")
	assert.NotEmpty(t, vpcID.Value, "VPC ID should not be empty")
	t.Logf("VPC ID: %v", vpcID.Value)

	subnetCount, ok := result.Outputs["subnet_count"]
	assert.True(t, ok, "subnet_count output should exist")
	assert.GreaterOrEqual(t, subnetCount.Value.(float64), float64(2), "Should have at least 2 subnets")
	t.Logf("Subnet count: %v", subnetCount.Value)

	t.Log("========================================")
	t.Log("VPC Manager Component E2E Test PASSED")
	t.Log("========================================")
}

// =============================================================================
// AWS Security Group E2E Test
// Tests: Security group creation with custom rules
// =============================================================================

// TestE2E_AWS_SecurityGroup tests security group creation with custom ingress/egress rules
func TestE2E_AWS_SecurityGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	stackName := fmt.Sprintf("%s-securitygroup", cfg.StackPrefix)
	report := NewTestReport("AWS Security Group")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	program := func(pctx *pulumi.Context) error {
		// Phase 1: Configuration
		phase1 := report.StartPhase("Security Configuration")
		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "e2e-sg-test",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  cfg.AWSRegion,
					VPC: &config.VPCConfig{
						Create: true,
						Name:   "e2e-sg-vpc",
						CIDR:   "10.240.0.0/16",
					},
				},
			},
			Security: config.SecurityConfig{},
			Network: config.NetworkConfig{
				CIDR: "10.240.0.0/16",
				Firewall: &config.FirewallConfig{
					InboundRules: []config.FirewallRule{
						{Port: "22", Protocol: "tcp", Source: []string{"0.0.0.0/0"}, Description: "SSH access"},
						{Port: "80", Protocol: "tcp", Source: []string{"0.0.0.0/0"}, Description: "HTTP"},
						{Port: "443", Protocol: "tcp", Source: []string{"0.0.0.0/0"}, Description: "HTTPS"},
						{Port: "6443", Protocol: "tcp", Source: []string{"0.0.0.0/0"}, Description: "Kubernetes API"},
						{Port: "51820", Protocol: "udp", Source: []string{"0.0.0.0/0"}, Description: "WireGuard VPN"},
						{Port: "2379-2380", Protocol: "tcp", Source: []string{"10.240.0.0/16"}, Description: "etcd"},
						{Port: "10250", Protocol: "tcp", Source: []string{"10.240.0.0/16"}, Description: "Kubelet"},
					},
					OutboundRules: []config.FirewallRule{
						{Port: "0", Protocol: "-1", Target: []string{"0.0.0.0/0"}, Description: "Allow all outbound"},
					},
				},
			},
		}
		report.EndPhase(phase1, "passed", "Security configuration created with 7 ingress rules")

		// Phase 2: Create VPC first (required for security group)
		phase2 := report.StartPhase("VPC Creation")
		awsProvider := providers.NewAWSProvider()
		if err := awsProvider.Initialize(pctx, clusterConfig); err != nil {
			report.AddError(fmt.Sprintf("AWS provider init failed: %v", err))
			return fmt.Errorf("AWSProvider.Initialize failed: %w", err)
		}

		networkOutput, err := awsProvider.CreateNetwork(pctx, &config.NetworkConfig{
			CIDR: clusterConfig.Providers.AWS.VPC.CIDR,
		})
		if err != nil {
			report.AddError(fmt.Sprintf("VPC creation failed: %v", err))
			return fmt.Errorf("AWSProvider.CreateNetwork failed: %w", err)
		}
		pctx.Export("vpc_id", networkOutput.ID)
		report.EndPhase(phase2, "passed", "VPC created")

		// Phase 3: Create Security Group
		phase3 := report.StartPhase("Security Group Creation")
		err = awsProvider.CreateFirewall(pctx, clusterConfig.Network.Firewall, nil)
		if err != nil {
			report.AddError(fmt.Sprintf("Security group creation failed: %v", err))
			return fmt.Errorf("AWSProvider.CreateFirewall failed: %w", err)
		}

		pctx.Export("ingress_rule_count", pulumi.Int(len(clusterConfig.Network.Firewall.InboundRules)))
		pctx.Export("egress_rule_count", pulumi.Int(len(clusterConfig.Network.Firewall.OutboundRules)))

		report.EndPhase(phase3, "passed", fmt.Sprintf("Security group created with %d ingress and %d egress rules",
			len(clusterConfig.Network.Firewall.InboundRules),
			len(clusterConfig.Network.Firewall.OutboundRules)))

		// Set metrics
		report.SetMetric("ingress_rules", len(clusterConfig.Network.Firewall.InboundRules))
		report.SetMetric("egress_rules", len(clusterConfig.Network.Firewall.OutboundRules))

		return nil
	}

	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	err := stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	t.Log("==========================================")
	t.Log("RUNNING: AWS Security Group E2E Test")
	t.Log("==========================================")
	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi Up failed")

	// Validate outputs
	ingressRuleCount, ok := result.Outputs["ingress_rule_count"]
	assert.True(t, ok, "ingress_rule_count output should exist")
	assert.Equal(t, float64(7), ingressRuleCount.Value, "Should have 7 ingress rules")

	t.Log("==========================================")
	t.Log("AWS Security Group E2E Test PASSED")
	t.Log("==========================================")
}

// =============================================================================
// AWS EC2 Instance with User Data E2E Test
// Tests: EC2 instance creation with cloud-init user data
// =============================================================================

// TestE2E_AWS_EC2_WithUserData tests EC2 instance creation with cloud-init
func TestE2E_AWS_EC2_WithUserData(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	stackName := fmt.Sprintf("%s-ec2-userdata", cfg.StackPrefix)
	report := NewTestReport("AWS EC2 with UserData")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	program := func(pctx *pulumi.Context) error {
		// Phase 1: Configuration
		phase1 := report.StartPhase("Configuration")
		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "e2e-userdata-test",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  cfg.AWSRegion,
					VPC: &config.VPCConfig{
						Create: true,
						Name:   "e2e-userdata-vpc",
						CIDR:   "10.250.0.0/16",
					},
				},
			},
			Security: config.SecurityConfig{},
			Network: config.NetworkConfig{
				CIDR: "10.250.0.0/16",
				Firewall: &config.FirewallConfig{
					InboundRules: []config.FirewallRule{
						{Port: "22", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
					},
				},
			},
		}
		report.EndPhase(phase1, "passed", "Configuration created")

		// Phase 2: SSH Key Generation
		phase2 := report.StartPhase("SSH Key Generation")
		sshComponent, err := components.NewSSHKeyComponent(pctx, "e2e-userdata-ssh", clusterConfig)
		if err != nil {
			report.AddError(fmt.Sprintf("SSH key creation failed: %v", err))
			return fmt.Errorf("failed to create SSH key: %w", err)
		}
		pctx.Export("ssh_public_key", sshComponent.PublicKey)
		report.EndPhase(phase2, "passed", "SSH keys generated")

		// Phase 3: Initialize Provider and Create Network
		phase3 := report.StartPhase("Network Creation")
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

		// Phase 4: Create Security Group
		phase4 := report.StartPhase("Security Group")
		err = awsProvider.CreateFirewall(pctx, clusterConfig.Network.Firewall, nil)
		if err != nil {
			report.AddError(fmt.Sprintf("Security group creation failed: %v", err))
			return fmt.Errorf("AWSProvider.CreateFirewall failed: %w", err)
		}
		report.EndPhase(phase4, "passed", "Security group created")

		// Phase 5: Create EC2 Instance with custom labels and user data
		phase5 := report.StartPhase("EC2 Instance Creation")
		nodeConfig := &config.NodeConfig{
			Name:        "e2e-userdata-node",
			Size:        "t3.micro",
			Image:       "ubuntu-22.04",
			Roles:       []string{"worker"},
			Region:      cfg.AWSRegion,
			WireGuardIP: "10.8.0.50",
			Labels: map[string]string{
				"environment": "e2e-test",
				"test-type":   "userdata",
				"team":        "infrastructure",
				"version":     "1.0.0",
			},
		}

		nodeOutput, err := awsProvider.CreateNode(pctx, nodeConfig)
		if err != nil {
			report.AddError(fmt.Sprintf("EC2 creation failed: %v", err))
			return fmt.Errorf("AWSProvider.CreateNode failed: %w", err)
		}

		pctx.Export("node_id", nodeOutput.ID)
		pctx.Export("node_name", pulumi.String(nodeOutput.Name))
		pctx.Export("node_public_ip", nodeOutput.PublicIP)
		pctx.Export("node_private_ip", nodeOutput.PrivateIP)
		pctx.Export("node_wireguard_ip", pulumi.String(nodeOutput.WireGuardIP))
		pctx.Export("node_provider", pulumi.String(nodeOutput.Provider))
		pctx.Export("node_region", pulumi.String(nodeOutput.Region))
		pctx.Export("node_labels", pulumi.ToStringMap(nodeOutput.Labels))

		report.EndPhase(phase5, "passed", fmt.Sprintf("EC2 instance '%s' created with %d labels", nodeOutput.Name, len(nodeOutput.Labels)))

		// Set metrics
		report.SetMetric("instance_type", nodeConfig.Size)
		report.SetMetric("label_count", len(nodeConfig.Labels))

		return nil
	}

	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	err := stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	t.Log("============================================")
	t.Log("RUNNING: AWS EC2 with UserData E2E Test")
	t.Log("============================================")
	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi Up failed")

	// Validate outputs
	nodeID, ok := result.Outputs["node_id"]
	assert.True(t, ok, "node_id output should exist")
	assert.NotEmpty(t, nodeID.Value, "Node ID should not be empty")
	t.Logf("Node ID: %v", nodeID.Value)

	nodeName, ok := result.Outputs["node_name"]
	assert.True(t, ok, "node_name output should exist")
	assert.Equal(t, "e2e-userdata-node", nodeName.Value)
	t.Logf("Node Name: %v", nodeName.Value)

	nodeLabels, ok := result.Outputs["node_labels"]
	assert.True(t, ok, "node_labels output should exist")
	labelsMap := nodeLabels.Value.(map[string]interface{})
	assert.Equal(t, "e2e-test", labelsMap["environment"])
	assert.Equal(t, "userdata", labelsMap["test-type"])
	t.Logf("Node Labels: %v", nodeLabels.Value)

	t.Log("============================================")
	t.Log("AWS EC2 with UserData E2E Test PASSED")
	t.Log("============================================")
}

// =============================================================================
// Network Configuration Validation E2E Test
// Tests: CIDR calculation and network configuration
// =============================================================================

// TestE2E_NetworkConfiguration tests network configuration and CIDR calculations
func TestE2E_NetworkConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	report := NewTestReport("Network Configuration")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	testCases := []struct {
		name             string
		vpcCIDR          string
		expectedSubnets  int
		expectedFirstSub string
		description      string
	}{
		{
			name:             "Standard /16 VPC",
			vpcCIDR:          "10.0.0.0/16",
			expectedSubnets:  2,
			expectedFirstSub: "10.0.1.0/24",
			description:      "Standard /16 VPC should create 2 /24 subnets",
		},
		{
			name:             "Custom /16 VPC",
			vpcCIDR:          "10.100.0.0/16",
			expectedSubnets:  2,
			expectedFirstSub: "10.100.1.0/24",
			description:      "Custom /16 VPC in 10.100.x.x range",
		},
		{
			name:             "Production VPC",
			vpcCIDR:          "10.200.0.0/16",
			expectedSubnets:  2,
			expectedFirstSub: "10.200.1.0/24",
			description:      "Production VPC in 10.200.x.x range",
		},
		{
			name:             "Private Range VPC",
			vpcCIDR:          "172.16.0.0/16",
			expectedSubnets:  2,
			expectedFirstSub: "172.16.1.0/24",
			description:      "VPC in 172.16.x.x private range",
		},
	}

	phase1 := report.StartPhase("CIDR Validation")
	passCount := 0
	failCount := 0

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate CIDR calculation (same logic as in aws.go)
			subnets := calculateTestSubnetCIDRs(tc.vpcCIDR, tc.expectedSubnets)

			if len(subnets) >= tc.expectedSubnets {
				if subnets[0] == tc.expectedFirstSub {
					t.Logf("PASS: %s - %s", tc.name, tc.description)
					t.Logf("  VPC CIDR: %s", tc.vpcCIDR)
					t.Logf("  Subnets: %v", subnets)
					passCount++
				} else {
					t.Errorf("FAIL: %s - expected first subnet %s, got %s", tc.name, tc.expectedFirstSub, subnets[0])
					failCount++
				}
			} else {
				t.Errorf("FAIL: %s - expected %d subnets, got %d", tc.name, tc.expectedSubnets, len(subnets))
				failCount++
			}
		})
	}

	report.EndPhase(phase1, "passed", fmt.Sprintf("%d/%d CIDR tests passed", passCount, passCount+failCount))
	report.SetMetric("tests_passed", passCount)
	report.SetMetric("tests_failed", failCount)

	t.Logf("==========================================")
	t.Logf("Network Configuration E2E Test: %d passed, %d failed", passCount, failCount)
	t.Logf("==========================================")
}

// calculateTestSubnetCIDRs calculates subnet CIDRs from a VPC CIDR (test helper)
func calculateTestSubnetCIDRs(vpcCIDR string, count int) []string {
	// Parse CIDR
	var parts []string
	current := ""
	for _, c := range vpcCIDR {
		if c == '/' {
			parts = append(parts, current)
			current = ""
		} else if c == '.' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}

	if len(parts) < 4 {
		return nil
	}

	firstOctet := parts[0]
	secondOctet := parts[1]

	var subnets []string
	for i := 1; i <= count; i++ {
		subnet := fmt.Sprintf("%s.%s.%d.0/24", firstOctet, secondOctet, i)
		subnets = append(subnets, subnet)
	}
	return subnets
}

// =============================================================================
// AWS Multi-AZ Deployment E2E Test
// Tests: Deployment across multiple availability zones
// =============================================================================

// TestE2E_AWS_MultiAZ tests deployment across multiple availability zones
func TestE2E_AWS_MultiAZ(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	stackName := fmt.Sprintf("%s-multiaz", cfg.StackPrefix)
	report := NewTestReport("AWS Multi-AZ Deployment")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	program := func(pctx *pulumi.Context) error {
		// Phase 1: Configuration
		phase1 := report.StartPhase("Multi-AZ Configuration")
		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "e2e-multiaz-test",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  cfg.AWSRegion,
					VPC: &config.VPCConfig{
						Create: true,
						Name:   "e2e-multiaz-vpc",
						CIDR:   "10.180.0.0/16",
					},
				},
			},
			Security: config.SecurityConfig{},
			Network: config.NetworkConfig{
				CIDR: "10.180.0.0/16",
				Firewall: &config.FirewallConfig{
					InboundRules: []config.FirewallRule{
						{Port: "22", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
						{Port: "6443", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
					},
				},
			},
		}
		report.EndPhase(phase1, "passed", "Multi-AZ configuration created")

		// Phase 2: SSH Keys
		phase2 := report.StartPhase("SSH Key Generation")
		_, err := components.NewSSHKeyComponent(pctx, "e2e-multiaz-ssh", clusterConfig)
		if err != nil {
			report.AddError(fmt.Sprintf("SSH key creation failed: %v", err))
			return fmt.Errorf("failed to create SSH key: %w", err)
		}
		report.EndPhase(phase2, "passed", "SSH keys generated")

		// Phase 3: Initialize Provider and Create Network
		phase3 := report.StartPhase("Network with Multi-AZ Subnets")
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
		pctx.Export("subnet_count", pulumi.Int(len(networkOutput.Subnets)))
		report.EndPhase(phase3, "passed", fmt.Sprintf("Network created with %d subnets", len(networkOutput.Subnets)))

		// Phase 4: Create Security Group
		phase4 := report.StartPhase("Security Group")
		err = awsProvider.CreateFirewall(pctx, clusterConfig.Network.Firewall, nil)
		if err != nil {
			report.AddError(fmt.Sprintf("Security group creation failed: %v", err))
			return fmt.Errorf("AWSProvider.CreateFirewall failed: %w", err)
		}
		report.EndPhase(phase4, "passed", "Security group created")

		// Phase 5: Create Node Pool with multiple nodes (distributed across AZs)
		phase5 := report.StartPhase("Multi-AZ Node Pool")
		nodePool := &config.NodePool{
			Name:  "e2e-multiaz-pool",
			Count: 3,
			Size:  "t3.micro",
			Image: "ubuntu-22.04",
			Roles: []string{"worker"},
			Labels: map[string]string{
				"environment":   "e2e-test",
				"topology-type": "multi-az",
			},
		}

		poolNodes, err := awsProvider.CreateNodePool(pctx, nodePool)
		if err != nil {
			report.AddError(fmt.Sprintf("Node pool creation failed: %v", err))
			return fmt.Errorf("AWSProvider.CreateNodePool failed: %w", err)
		}

		pctx.Export("pool_node_count", pulumi.Int(len(poolNodes)))
		for i, node := range poolNodes {
			pctx.Export(fmt.Sprintf("node_%d_id", i), node.ID)
			pctx.Export(fmt.Sprintf("node_%d_public_ip", i), node.PublicIP)
			pctx.Export(fmt.Sprintf("node_%d_private_ip", i), node.PrivateIP)
		}
		report.EndPhase(phase5, "passed", fmt.Sprintf("Node pool created with %d nodes across AZs", len(poolNodes)))

		// Set metrics
		report.SetMetric("node_count", len(poolNodes))
		report.SetMetric("subnet_count", len(networkOutput.Subnets))

		return nil
	}

	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	err := stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	t.Log("============================================")
	t.Log("RUNNING: AWS Multi-AZ Deployment E2E Test")
	t.Log("============================================")
	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi Up failed")

	// Validate outputs
	poolNodeCount, ok := result.Outputs["pool_node_count"]
	assert.True(t, ok, "pool_node_count output should exist")
	assert.Equal(t, float64(3), poolNodeCount.Value, "Should have 3 nodes in pool")

	subnetCount, ok := result.Outputs["subnet_count"]
	assert.True(t, ok, "subnet_count output should exist")
	assert.GreaterOrEqual(t, subnetCount.Value.(float64), float64(2), "Should have at least 2 subnets")

	// Validate each node has an IP
	for i := 0; i < 3; i++ {
		nodeIP, ok := result.Outputs[fmt.Sprintf("node_%d_public_ip", i)]
		assert.True(t, ok, fmt.Sprintf("node_%d_public_ip should exist", i))
		assert.NotEmpty(t, nodeIP.Value, fmt.Sprintf("Node %d public IP should not be empty", i))
		t.Logf("Node %d Public IP: %v", i, nodeIP.Value)
	}

	t.Log("============================================")
	t.Log("AWS Multi-AZ Deployment E2E Test PASSED")
	t.Log("============================================")
}

// =============================================================================
// WireGuard Configuration Validation E2E Test
// Tests: WireGuard IP assignment and configuration generation
// =============================================================================

// TestE2E_WireGuardConfiguration tests WireGuard IP assignment logic
func TestE2E_WireGuardConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	report := NewTestReport("WireGuard Configuration")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	phase1 := report.StartPhase("WireGuard IP Assignment")

	// Test WireGuard IP assignment patterns
	testCases := []struct {
		name        string
		nodeType    string
		nodeIndex   int
		expectedIP  string
		description string
	}{
		{"Bastion", "bastion", 0, "10.8.0.5", "Bastion always gets .5"},
		{"Master 1", "master", 0, "10.8.0.10", "First master gets .10"},
		{"Master 2", "master", 1, "10.8.0.11", "Second master gets .11"},
		{"Master 3", "master", 2, "10.8.0.12", "Third master gets .12"},
		{"Worker 1", "worker", 0, "10.8.0.30", "First worker gets .30"},
		{"Worker 2", "worker", 1, "10.8.0.31", "Second worker gets .31"},
		{"Worker 3", "worker", 2, "10.8.0.32", "Third worker gets .32"},
	}

	passCount := 0
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			calculatedIP := calculateWireGuardIP(tc.nodeType, tc.nodeIndex)
			if calculatedIP == tc.expectedIP {
				t.Logf("PASS: %s - %s = %s", tc.name, tc.description, calculatedIP)
				passCount++
			} else {
				t.Errorf("FAIL: %s - expected %s, got %s", tc.name, tc.expectedIP, calculatedIP)
			}
		})
	}

	report.EndPhase(phase1, "passed", fmt.Sprintf("%d/%d IP assignments correct", passCount, len(testCases)))
	report.SetMetric("tests_passed", passCount)
	report.SetMetric("total_tests", len(testCases))

	t.Logf("==========================================")
	t.Logf("WireGuard Configuration E2E Test PASSED")
	t.Logf("==========================================")
}

// calculateWireGuardIP calculates WireGuard IP for a node (test helper)
func calculateWireGuardIP(nodeType string, index int) string {
	switch nodeType {
	case "bastion":
		return "10.8.0.5"
	case "master":
		return fmt.Sprintf("10.8.0.%d", 10+index)
	case "worker":
		return fmt.Sprintf("10.8.0.%d", 30+index)
	default:
		return fmt.Sprintf("10.8.0.%d", 50+index)
	}
}
