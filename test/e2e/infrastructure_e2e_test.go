//go:build e2e
// +build e2e

// Package e2e provides comprehensive end-to-end tests for infrastructure components
package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
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
// AWS Full Infrastructure E2E Test
// Tests: VPC + Subnets + Internet Gateway + Security Group + EC2 Instance
// =============================================================================

// TestE2E_AWS_FullInfrastructure tests complete AWS infrastructure provisioning
// using the REAL AWSProvider code
func TestE2E_AWS_FullInfrastructure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	stackName := fmt.Sprintf("%s-full-infra", cfg.StackPrefix)
	report := NewTestReport("AWS Full Infrastructure")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	// Define Pulumi program that creates full AWS infrastructure
	program := func(pctx *pulumi.Context) error {
		// Phase 1: Create cluster configuration
		phase1 := report.StartPhase("Configuration")
		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "e2e-full-infra-test",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  cfg.AWSRegion,
					VPC: &config.VPCConfig{
						Create: true,
						Name:   "e2e-full-infra-vpc",
						CIDR:   "10.200.0.0/16",
					},
					KeyPair: "e2e-test-key",
				},
			},
			Security: config.SecurityConfig{
				SSHConfig: config.SSHConfig{
					PublicKeyPath: "",
				},
			},
			Network: config.NetworkConfig{
				CIDR: "10.200.0.0/16",
				Firewall: &config.FirewallConfig{
					InboundRules: []config.FirewallRule{
						{Port: "22", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
						{Port: "6443", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
						{Port: "80", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
						{Port: "443", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
					},
				},
			},
		}
		report.EndPhase(phase1, "passed", "Configuration created")

		// Phase 2: Create SSH Key Component
		phase2 := report.StartPhase("SSH Key Generation")
		sshComponent, err := components.NewSSHKeyComponent(pctx, "e2e-infra-ssh", clusterConfig)
		if err != nil {
			report.AddError(fmt.Sprintf("SSH key creation failed: %v", err))
			return fmt.Errorf("failed to create SSH key: %w", err)
		}
		pctx.Export("ssh_public_key", sshComponent.PublicKey)
		report.EndPhase(phase2, "passed", "SSH keys generated")

		// Phase 2.5: Create AWS Key Pair using the generated SSH key
		phase25 := report.StartPhase("AWS Key Pair Creation")
		keyPairName := fmt.Sprintf("e2e-test-key-%s", pctx.Stack())
		keyPair, err := ec2.NewKeyPair(pctx, "e2e-keypair", &ec2.KeyPairArgs{
			KeyName:   pulumi.String(keyPairName),
			PublicKey: sshComponent.PublicKey,
			Tags: pulumi.StringMap{
				"Name":        pulumi.String(keyPairName),
				"Environment": pulumi.String("e2e-test"),
			},
		})
		if err != nil {
			report.AddError(fmt.Sprintf("AWS Key Pair creation failed: %v", err))
			return fmt.Errorf("failed to create AWS key pair: %w", err)
		}
		pctx.Export("key_pair_name", keyPair.KeyName)
		pctx.Export("key_pair_id", keyPair.ID())

		// Update cluster config to use the created key pair
		clusterConfig.Providers.AWS.KeyPair = keyPairName
		report.EndPhase(phase25, "passed", "AWS Key Pair created")

		// Phase 3: Initialize AWS Provider
		phase3 := report.StartPhase("AWS Provider Init")
		awsProvider := providers.NewAWSProvider()
		if err := awsProvider.Initialize(pctx, clusterConfig); err != nil {
			report.AddError(fmt.Sprintf("AWS provider init failed: %v", err))
			return fmt.Errorf("AWSProvider.Initialize failed: %w", err)
		}
		report.EndPhase(phase3, "passed", "AWS provider initialized")

		// Phase 4: Create Network (VPC, Subnets, IGW, Route Tables)
		phase4 := report.StartPhase("Network Creation")
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
		report.EndPhase(phase4, "passed", fmt.Sprintf("VPC created with %d subnets", len(networkOutput.Subnets)))

		// Phase 5: Create Security Group (Firewall)
		phase5 := report.StartPhase("Security Group Creation")
		err = awsProvider.CreateFirewall(pctx, clusterConfig.Network.Firewall, nil)
		if err != nil {
			report.AddError(fmt.Sprintf("Security group creation failed: %v", err))
			return fmt.Errorf("AWSProvider.CreateFirewall failed: %w", err)
		}
		report.EndPhase(phase5, "passed", "Security group created with ingress rules")

		// Phase 6: Create EC2 Instance (Master Node)
		phase6 := report.StartPhase("EC2 Instance Creation")
		nodeConfig := &config.NodeConfig{
			Name:        "e2e-master-1",
			Size:        "t3.micro",
			Image:       "ubuntu-22.04",
			Roles:       []string{"master", "etcd"},
			Region:      cfg.AWSRegion,
			WireGuardIP: "10.8.0.10",
			Labels: map[string]string{
				"environment": "e2e-test",
				"role":        "master",
			},
		}
		nodeOutput, err := awsProvider.CreateNode(pctx, nodeConfig)
		if err != nil {
			report.AddError(fmt.Sprintf("EC2 instance creation failed: %v", err))
			return fmt.Errorf("AWSProvider.CreateNode failed: %w", err)
		}
		pctx.Export("node_id", nodeOutput.ID)
		pctx.Export("node_name", pulumi.String(nodeOutput.Name))
		pctx.Export("node_public_ip", nodeOutput.PublicIP)
		pctx.Export("node_private_ip", nodeOutput.PrivateIP)
		pctx.Export("node_provider", pulumi.String(nodeOutput.Provider))
		report.EndPhase(phase6, "passed", fmt.Sprintf("EC2 instance '%s' created", nodeOutput.Name))

		// Set metrics
		report.SetMetric("vpc_cidr", clusterConfig.Providers.AWS.VPC.CIDR)
		report.SetMetric("subnet_count", len(networkOutput.Subnets))
		report.SetMetric("node_size", nodeConfig.Size)
		report.SetMetric("node_roles", nodeConfig.Roles)

		return nil
	}

	// Create stack
	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	// Set AWS configuration
	err := stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	// Execute stack.Up()
	t.Log("========================================")
	t.Log("RUNNING: AWS Full Infrastructure E2E Test")
	t.Log("========================================")
	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi Up failed")

	// Validate outputs
	t.Log("Validating infrastructure outputs...")

	// Validate VPC
	vpcID, ok := result.Outputs["vpc_id"]
	assert.True(t, ok, "vpc_id output should exist")
	assert.NotEmpty(t, vpcID.Value, "VPC ID should not be empty")
	t.Logf("VPC ID: %v", vpcID.Value)

	// Validate Subnets
	subnetCount, ok := result.Outputs["subnet_count"]
	assert.True(t, ok, "subnet_count output should exist")
	assert.GreaterOrEqual(t, subnetCount.Value.(float64), float64(2), "Should have at least 2 subnets")
	t.Logf("Subnet count: %v", subnetCount.Value)

	// Validate EC2 Instance
	nodeID, ok := result.Outputs["node_id"]
	assert.True(t, ok, "node_id output should exist")
	assert.NotEmpty(t, nodeID.Value, "Node ID should not be empty")
	t.Logf("Node ID: %v", nodeID.Value)

	publicIP, ok := result.Outputs["node_public_ip"]
	assert.True(t, ok, "node_public_ip output should exist")
	t.Logf("Node Public IP: %v", publicIP.Value)

	privateIP, ok := result.Outputs["node_private_ip"]
	assert.True(t, ok, "node_private_ip output should exist")
	t.Logf("Node Private IP: %v", privateIP.Value)

	t.Log("========================================")
	t.Log("AWS Full Infrastructure E2E Test PASSED")
	t.Log("========================================")
}

// =============================================================================
// AWS Multi-Node Cluster E2E Test
// Tests: Multiple EC2 instances with different roles
// =============================================================================

// TestE2E_AWS_MultiNodeCluster tests deploying multiple nodes with different roles
func TestE2E_AWS_MultiNodeCluster(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	stackName := fmt.Sprintf("%s-multinode", cfg.StackPrefix)

	program := func(pctx *pulumi.Context) error {
		// Create cluster configuration
		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "e2e-multinode-test",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  cfg.AWSRegion,
					VPC: &config.VPCConfig{
						Create: true,
						Name:   "e2e-multinode-vpc",
						CIDR:   "10.201.0.0/16",
					},
					KeyPair: "e2e-test-key",
				},
			},
			Security: config.SecurityConfig{
				SSHConfig: config.SSHConfig{
					PublicKeyPath: "",
				},
			},
			Network: config.NetworkConfig{
				CIDR: "10.201.0.0/16",
				Firewall: &config.FirewallConfig{
					InboundRules: []config.FirewallRule{
						{Port: "22", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
						{Port: "6443", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
						{Port: "10250", Protocol: "tcp", Source: []string{"10.201.0.0/16"}},
						{Port: "2379-2380", Protocol: "tcp", Source: []string{"10.201.0.0/16"}},
					},
				},
			},
		}

		// Create SSH Key
		sshComponent, err := components.NewSSHKeyComponent(pctx, "e2e-multinode-ssh", clusterConfig)
		if err != nil {
			return fmt.Errorf("failed to create SSH key: %w", err)
		}
		pctx.Export("ssh_public_key", sshComponent.PublicKey)

		// Initialize AWS Provider
		awsProvider := providers.NewAWSProvider()
		if err := awsProvider.Initialize(pctx, clusterConfig); err != nil {
			return fmt.Errorf("AWSProvider.Initialize failed: %w", err)
		}

		// Create Network
		networkOutput, err := awsProvider.CreateNetwork(pctx, &config.NetworkConfig{
			CIDR: clusterConfig.Providers.AWS.VPC.CIDR,
		})
		if err != nil {
			return fmt.Errorf("AWSProvider.CreateNetwork failed: %w", err)
		}
		pctx.Export("vpc_id", networkOutput.ID)

		// Create Security Group
		err = awsProvider.CreateFirewall(pctx, clusterConfig.Network.Firewall, nil)
		if err != nil {
			return fmt.Errorf("AWSProvider.CreateFirewall failed: %w", err)
		}

		// Create Master Node Pool
		masterPool := &config.NodePool{
			Name:  "masters",
			Count: 1,
			Size:  "t3.micro",
			Image: "ubuntu-22.04",
			Roles: []string{"master", "etcd", "controlplane"},
			Labels: map[string]string{
				"node-role.kubernetes.io/master": "true",
				"environment":                    "e2e-test",
			},
		}

		masterNodes, err := awsProvider.CreateNodePool(pctx, masterPool)
		if err != nil {
			return fmt.Errorf("failed to create master pool: %w", err)
		}
		pctx.Export("master_count", pulumi.Int(len(masterNodes)))

		// Create Worker Node Pool
		workerPool := &config.NodePool{
			Name:  "workers",
			Count: 2,
			Size:  "t3.micro",
			Image: "ubuntu-22.04",
			Roles: []string{"worker"},
			Labels: map[string]string{
				"node-role.kubernetes.io/worker": "true",
				"environment":                    "e2e-test",
			},
		}

		workerNodes, err := awsProvider.CreateNodePool(pctx, workerPool)
		if err != nil {
			return fmt.Errorf("failed to create worker pool: %w", err)
		}
		pctx.Export("worker_count", pulumi.Int(len(workerNodes)))

		// Export total node count
		totalNodes := len(masterNodes) + len(workerNodes)
		pctx.Export("total_nodes", pulumi.Int(totalNodes))

		// Export node details
		for i, node := range masterNodes {
			pctx.Export(fmt.Sprintf("master_%d_id", i), node.ID)
			pctx.Export(fmt.Sprintf("master_%d_public_ip", i), node.PublicIP)
		}
		for i, node := range workerNodes {
			pctx.Export(fmt.Sprintf("worker_%d_id", i), node.ID)
			pctx.Export(fmt.Sprintf("worker_%d_public_ip", i), node.PublicIP)
		}

		return nil
	}

	// Create stack
	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	// Set AWS configuration
	err := stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	// Execute stack.Up()
	t.Log("==========================================")
	t.Log("RUNNING: AWS Multi-Node Cluster E2E Test")
	t.Log("==========================================")
	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi Up failed")

	// Validate outputs
	masterCount, ok := result.Outputs["master_count"]
	assert.True(t, ok, "master_count output should exist")
	assert.Equal(t, float64(1), masterCount.Value, "Should have 1 master node")

	workerCount, ok := result.Outputs["worker_count"]
	assert.True(t, ok, "worker_count output should exist")
	assert.Equal(t, float64(2), workerCount.Value, "Should have 2 worker nodes")

	totalNodes, ok := result.Outputs["total_nodes"]
	assert.True(t, ok, "total_nodes output should exist")
	assert.Equal(t, float64(3), totalNodes.Value, "Should have 3 total nodes")

	t.Log("==========================================")
	t.Log("AWS Multi-Node Cluster E2E Test PASSED")
	t.Log("==========================================")
}

// =============================================================================
// AWS Spot Instance E2E Test
// Tests: Spot instance creation for cost savings
// =============================================================================

// TestE2E_AWS_SpotInstance tests creating spot instances
func TestE2E_AWS_SpotInstance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	stackName := fmt.Sprintf("%s-spot", cfg.StackPrefix)

	program := func(pctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "e2e-spot-test",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  cfg.AWSRegion,
					VPC: &config.VPCConfig{
						Create: true,
						Name:   "e2e-spot-vpc",
						CIDR:   "10.202.0.0/16",
					},
					KeyPair: "e2e-test-key",
				},
			},
			Security: config.SecurityConfig{},
			Network: config.NetworkConfig{
				CIDR: "10.202.0.0/16",
				Firewall: &config.FirewallConfig{
					InboundRules: []config.FirewallRule{
						{Port: "22", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
					},
				},
			},
		}

		// Create SSH Key
		_, err := components.NewSSHKeyComponent(pctx, "e2e-spot-ssh", clusterConfig)
		if err != nil {
			return fmt.Errorf("failed to create SSH key: %w", err)
		}

		// Initialize AWS Provider
		awsProvider := providers.NewAWSProvider()
		if err := awsProvider.Initialize(pctx, clusterConfig); err != nil {
			return fmt.Errorf("AWSProvider.Initialize failed: %w", err)
		}

		// Create Network
		networkOutput, err := awsProvider.CreateNetwork(pctx, &config.NetworkConfig{
			CIDR: clusterConfig.Providers.AWS.VPC.CIDR,
		})
		if err != nil {
			return fmt.Errorf("AWSProvider.CreateNetwork failed: %w", err)
		}
		pctx.Export("vpc_id", networkOutput.ID)

		// Create Security Group
		err = awsProvider.CreateFirewall(pctx, clusterConfig.Network.Firewall, nil)
		if err != nil {
			return fmt.Errorf("AWSProvider.CreateFirewall failed: %w", err)
		}

		// Create Spot Instance
		spotNode := &config.NodeConfig{
			Name:         "e2e-spot-worker-1",
			Size:         "t3.micro",
			Image:        "ubuntu-22.04",
			Roles:        []string{"worker"},
			Region:       cfg.AWSRegion,
			SpotInstance: true, // Enable spot instance
			WireGuardIP:  "10.8.0.30",
			Labels: map[string]string{
				"spot-instance": "true",
				"environment":   "e2e-test",
			},
		}

		nodeOutput, err := awsProvider.CreateNode(pctx, spotNode)
		if err != nil {
			return fmt.Errorf("failed to create spot instance: %w", err)
		}

		pctx.Export("spot_node_id", nodeOutput.ID)
		pctx.Export("spot_node_public_ip", nodeOutput.PublicIP)
		pctx.Export("is_spot_instance", pulumi.Bool(true))

		return nil
	}

	// Create stack
	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	// Set AWS configuration
	err := stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	// Execute stack.Up()
	t.Log("====================================")
	t.Log("RUNNING: AWS Spot Instance E2E Test")
	t.Log("====================================")
	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi Up failed")

	// Validate outputs
	spotNodeID, ok := result.Outputs["spot_node_id"]
	assert.True(t, ok, "spot_node_id output should exist")
	assert.NotEmpty(t, spotNodeID.Value, "Spot node ID should not be empty")

	t.Log("====================================")
	t.Log("AWS Spot Instance E2E Test PASSED")
	t.Log("====================================")
}
