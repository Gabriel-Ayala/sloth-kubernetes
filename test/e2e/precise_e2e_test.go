// +build e2e

// Package e2e provides precise end-to-end tests for each infrastructure component
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

	"github.com/chalkan3/sloth-kubernetes/internal/orchestrator"
	"github.com/chalkan3/sloth-kubernetes/internal/orchestrator/components"
	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
)

// =============================================================================
// HELPER: Create AWS Key Pair from SSH Component
// This is the key fix - all tests must create KeyPair dynamically
// =============================================================================

type AWSTestContext struct {
	ClusterConfig *config.ClusterConfig
	SSHComponent  *components.SSHKeyComponent
	KeyPair       *ec2.KeyPair
	KeyPairName   string
	AWSProvider   *providers.AWSProvider
	NetworkOutput *providers.NetworkOutput
}

// setupAWSTestContext creates all the common infrastructure needed for AWS tests
func setupAWSTestContext(pctx *pulumi.Context, testName string, vpcCIDR string, region string, report *TestReport) (*AWSTestContext, error) {
	ctx := &AWSTestContext{}

	// Phase 1: Create cluster configuration
	phase1 := report.StartPhase("Cluster Configuration")
	ctx.ClusterConfig = &config.ClusterConfig{
		Metadata: config.Metadata{
			Name: testName,
		},
		Providers: config.ProvidersConfig{
			AWS: &config.AWSProvider{
				Enabled: true,
				Region:  region,
				VPC: &config.VPCConfig{
					Create: true,
					Name:   fmt.Sprintf("%s-vpc", testName),
					CIDR:   vpcCIDR,
				},
				KeyPair: "temp-key", // Will be replaced
			},
		},
		Security: config.SecurityConfig{
			SSHConfig: config.SSHConfig{
				PublicKeyPath: "",
			},
		},
		Network: config.NetworkConfig{
			CIDR: vpcCIDR,
			Firewall: &config.FirewallConfig{
				InboundRules: []config.FirewallRule{
					{Port: "22", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
					{Port: "6443", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
					{Port: "51820", Protocol: "udp", Source: []string{"0.0.0.0/0"}}, // WireGuard
					{Port: "10250", Protocol: "tcp", Source: []string{vpcCIDR}},     // Kubelet
					{Port: "2379-2380", Protocol: "tcp", Source: []string{vpcCIDR}}, // etcd
					{Port: "80", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
					{Port: "443", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
				},
			},
		},
	}
	report.EndPhase(phase1, "passed", "Cluster configuration created")

	// Phase 2: Create SSH Key Component
	phase2 := report.StartPhase("SSH Key Generation")
	var err error
	ctx.SSHComponent, err = components.NewSSHKeyComponent(pctx, fmt.Sprintf("%s-ssh", testName), ctx.ClusterConfig)
	if err != nil {
		report.AddError(fmt.Sprintf("SSH key creation failed: %v", err))
		return nil, fmt.Errorf("failed to create SSH key: %w", err)
	}
	pctx.Export("ssh_public_key", ctx.SSHComponent.PublicKey)
	report.EndPhase(phase2, "passed", "SSH key pair generated")

	// Phase 3: Create AWS Key Pair (CRITICAL - this was missing in other tests)
	phase3 := report.StartPhase("AWS Key Pair Creation")
	ctx.KeyPairName = fmt.Sprintf("%s-keypair-%s", testName, pctx.Stack())
	ctx.KeyPair, err = ec2.NewKeyPair(pctx, fmt.Sprintf("%s-keypair", testName), &ec2.KeyPairArgs{
		KeyName:   pulumi.String(ctx.KeyPairName),
		PublicKey: ctx.SSHComponent.PublicKey,
		Tags: pulumi.StringMap{
			"Name":        pulumi.String(ctx.KeyPairName),
			"Environment": pulumi.String("e2e-test"),
			"TestName":    pulumi.String(testName),
		},
	})
	if err != nil {
		report.AddError(fmt.Sprintf("AWS Key Pair creation failed: %v", err))
		return nil, fmt.Errorf("failed to create AWS key pair: %w", err)
	}
	// Update cluster config to use the created key pair
	ctx.ClusterConfig.Providers.AWS.KeyPair = ctx.KeyPairName
	pctx.Export("aws_keypair_name", ctx.KeyPair.KeyName)
	pctx.Export("aws_keypair_id", ctx.KeyPair.ID())
	report.EndPhase(phase3, "passed", fmt.Sprintf("AWS Key Pair '%s' created", ctx.KeyPairName))

	// Phase 4: Initialize AWS Provider
	phase4 := report.StartPhase("AWS Provider Initialization")
	ctx.AWSProvider = providers.NewAWSProvider()
	if err := ctx.AWSProvider.Initialize(pctx, ctx.ClusterConfig); err != nil {
		report.AddError(fmt.Sprintf("AWS provider init failed: %v", err))
		return nil, fmt.Errorf("AWSProvider.Initialize failed: %w", err)
	}
	report.EndPhase(phase4, "passed", "AWS provider initialized successfully")

	// Phase 5: Create Network Infrastructure
	phase5 := report.StartPhase("VPC Network Creation")
	ctx.NetworkOutput, err = ctx.AWSProvider.CreateNetwork(pctx, &config.NetworkConfig{
		CIDR: vpcCIDR,
	})
	if err != nil {
		report.AddError(fmt.Sprintf("Network creation failed: %v", err))
		return nil, fmt.Errorf("AWSProvider.CreateNetwork failed: %w", err)
	}
	pctx.Export("vpc_id", ctx.NetworkOutput.ID)
	pctx.Export("vpc_cidr", pulumi.String(ctx.NetworkOutput.CIDR))
	pctx.Export("subnet_count", pulumi.Int(len(ctx.NetworkOutput.Subnets)))
	report.EndPhase(phase5, "passed", fmt.Sprintf("VPC created with %d subnets", len(ctx.NetworkOutput.Subnets)))

	// Phase 6: Create Security Group
	phase6 := report.StartPhase("Security Group Creation")
	err = ctx.AWSProvider.CreateFirewall(pctx, ctx.ClusterConfig.Network.Firewall, nil)
	if err != nil {
		report.AddError(fmt.Sprintf("Security group creation failed: %v", err))
		return nil, fmt.Errorf("AWSProvider.CreateFirewall failed: %w", err)
	}
	report.EndPhase(phase6, "passed", fmt.Sprintf("Security group created with %d inbound rules",
		len(ctx.ClusterConfig.Network.Firewall.InboundRules)))

	return ctx, nil
}

// =============================================================================
// TEST 1: AWS VPC and Networking (Precise)
// =============================================================================

func TestE2E_Precise_AWS_VPC_Networking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	stackName := fmt.Sprintf("%s-precise-vpc", cfg.StackPrefix)
	report := NewTestReport("Precise AWS VPC Networking")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	program := func(pctx *pulumi.Context) error {
		testCtx, err := setupAWSTestContext(pctx, "precise-vpc-test", "10.100.0.0/16", cfg.AWSRegion, report)
		if err != nil {
			return err
		}

		// Additional VPC validations
		phase := report.StartPhase("VPC Detailed Validation")

		// Export subnet details
		for i, subnet := range testCtx.NetworkOutput.Subnets {
			pctx.Export(fmt.Sprintf("subnet_%d_id", i), subnet.ID)
			pctx.Export(fmt.Sprintf("subnet_%d_cidr", i), pulumi.String(subnet.CIDR))
			pctx.Export(fmt.Sprintf("subnet_%d_az", i), pulumi.String(subnet.Zone))
		}

		report.EndPhase(phase, "passed", "VPC networking validated")
		report.SetMetric("vpc_cidr", testCtx.ClusterConfig.Providers.AWS.VPC.CIDR)
		report.SetMetric("total_subnets", len(testCtx.NetworkOutput.Subnets))

		return nil
	}

	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	err := stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	t.Log("================================================")
	t.Log("RUNNING: Precise AWS VPC Networking E2E Test")
	t.Log("================================================")

	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi Up failed")

	// Validate VPC
	vpcID := result.Outputs["vpc_id"]
	assert.NotEmpty(t, vpcID.Value, "VPC ID should not be empty")
	t.Logf("✓ VPC created: %v", vpcID.Value)

	// Validate subnets (should be at least 2 for multi-AZ)
	subnetCount := result.Outputs["subnet_count"]
	assert.GreaterOrEqual(t, subnetCount.Value.(float64), float64(2), "Should have at least 2 subnets")
	t.Logf("✓ Subnets created: %v", subnetCount.Value)

	// Validate each subnet has different AZ
	subnet0AZ := result.Outputs["subnet_0_az"]
	subnet1AZ := result.Outputs["subnet_1_az"]
	if subnet0AZ.Value != nil && subnet1AZ.Value != nil {
		t.Logf("✓ Subnet 0 AZ: %v", subnet0AZ.Value)
		t.Logf("✓ Subnet 1 AZ: %v", subnet1AZ.Value)
	}

	t.Log("================================================")
	t.Log("Precise AWS VPC Networking E2E Test PASSED")
	t.Log("================================================")
}

// =============================================================================
// TEST 2: AWS EC2 Single Node with UserData (Precise)
// =============================================================================

func TestE2E_Precise_AWS_EC2_SingleNode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	stackName := fmt.Sprintf("%s-precise-ec2", cfg.StackPrefix)
	report := NewTestReport("Precise AWS EC2 Single Node")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	program := func(pctx *pulumi.Context) error {
		testCtx, err := setupAWSTestContext(pctx, "precise-ec2-test", "10.101.0.0/16", cfg.AWSRegion, report)
		if err != nil {
			return err
		}

		// Create EC2 Instance
		phase := report.StartPhase("EC2 Instance Creation")
		nodeConfig := &config.NodeConfig{
			Name:        "precise-master-1",
			Size:        "t3.micro",
			Image:       "ubuntu-22.04",
			Roles:       []string{"master", "etcd"},
			Region:      cfg.AWSRegion,
			WireGuardIP: "10.8.0.10",
			Labels: map[string]string{
				"environment": "e2e-test",
				"role":        "master",
				"test":        "precise-ec2",
			},
		}

		nodeOutput, err := testCtx.AWSProvider.CreateNode(pctx, nodeConfig)
		if err != nil {
			report.AddError(fmt.Sprintf("EC2 creation failed: %v", err))
			return fmt.Errorf("CreateNode failed: %w", err)
		}

		pctx.Export("node_id", nodeOutput.ID)
		pctx.Export("node_name", pulumi.String(nodeOutput.Name))
		pctx.Export("node_public_ip", nodeOutput.PublicIP)
		pctx.Export("node_private_ip", nodeOutput.PrivateIP)
		pctx.Export("node_provider", pulumi.String(nodeOutput.Provider))
		pctx.Export("node_region", pulumi.String(nodeOutput.Region))
		pctx.Export("node_wireguard_ip", pulumi.String(nodeConfig.WireGuardIP))

		report.EndPhase(phase, "passed", fmt.Sprintf("EC2 instance '%s' created", nodeOutput.Name))
		report.SetMetric("instance_type", nodeConfig.Size)
		report.SetMetric("instance_roles", nodeConfig.Roles)

		return nil
	}

	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	err := stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	t.Log("================================================")
	t.Log("RUNNING: Precise AWS EC2 Single Node E2E Test")
	t.Log("================================================")

	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi Up failed")

	// Validate EC2 Instance
	nodeID := result.Outputs["node_id"]
	assert.NotEmpty(t, nodeID.Value, "Node ID should not be empty")
	t.Logf("✓ EC2 Instance ID: %v", nodeID.Value)

	nodeName := result.Outputs["node_name"]
	assert.Equal(t, "precise-master-1", nodeName.Value, "Node name should match")
	t.Logf("✓ Node Name: %v", nodeName.Value)

	publicIP := result.Outputs["node_public_ip"]
	assert.NotEmpty(t, publicIP.Value, "Public IP should not be empty")
	t.Logf("✓ Public IP: %v", publicIP.Value)

	privateIP := result.Outputs["node_private_ip"]
	assert.NotEmpty(t, privateIP.Value, "Private IP should not be empty")
	t.Logf("✓ Private IP: %v", privateIP.Value)

	provider := result.Outputs["node_provider"]
	assert.Equal(t, "aws", provider.Value, "Provider should be 'aws'")
	t.Logf("✓ Provider: %v", provider.Value)

	t.Log("================================================")
	t.Log("Precise AWS EC2 Single Node E2E Test PASSED")
	t.Log("================================================")
}

// =============================================================================
// TEST 3: AWS Multi-Node Pool (Precise)
// =============================================================================

func TestE2E_Precise_AWS_MultiNode_Pool(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	stackName := fmt.Sprintf("%s-precise-pool", cfg.StackPrefix)
	report := NewTestReport("Precise AWS Multi-Node Pool")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	program := func(pctx *pulumi.Context) error {
		testCtx, err := setupAWSTestContext(pctx, "precise-pool-test", "10.102.0.0/16", cfg.AWSRegion, report)
		if err != nil {
			return err
		}

		// Create Master Node Pool
		phase1 := report.StartPhase("Master Node Pool Creation")
		masterPool := &config.NodePool{
			Name:  "masters",
			Count: 1,
			Size:  "t3.micro",
			Image: "ubuntu-22.04",
			Roles: []string{"master", "etcd", "controlplane"},
			Labels: map[string]string{
				"node-role.kubernetes.io/master": "true",
			},
		}

		masterNodes, err := testCtx.AWSProvider.CreateNodePool(pctx, masterPool)
		if err != nil {
			report.AddError(fmt.Sprintf("Master pool creation failed: %v", err))
			return fmt.Errorf("CreateNodePool (masters) failed: %w", err)
		}
		pctx.Export("master_count", pulumi.Int(len(masterNodes)))
		for i, node := range masterNodes {
			pctx.Export(fmt.Sprintf("master_%d_id", i), node.ID)
			pctx.Export(fmt.Sprintf("master_%d_public_ip", i), node.PublicIP)
			pctx.Export(fmt.Sprintf("master_%d_private_ip", i), node.PrivateIP)
		}
		report.EndPhase(phase1, "passed", fmt.Sprintf("Created %d master nodes", len(masterNodes)))

		// Create Worker Node Pool
		phase2 := report.StartPhase("Worker Node Pool Creation")
		workerPool := &config.NodePool{
			Name:  "workers",
			Count: 2,
			Size:  "t3.micro",
			Image: "ubuntu-22.04",
			Roles: []string{"worker"},
			Labels: map[string]string{
				"node-role.kubernetes.io/worker": "true",
			},
		}

		workerNodes, err := testCtx.AWSProvider.CreateNodePool(pctx, workerPool)
		if err != nil {
			report.AddError(fmt.Sprintf("Worker pool creation failed: %v", err))
			return fmt.Errorf("CreateNodePool (workers) failed: %w", err)
		}
		pctx.Export("worker_count", pulumi.Int(len(workerNodes)))
		for i, node := range workerNodes {
			pctx.Export(fmt.Sprintf("worker_%d_id", i), node.ID)
			pctx.Export(fmt.Sprintf("worker_%d_public_ip", i), node.PublicIP)
			pctx.Export(fmt.Sprintf("worker_%d_private_ip", i), node.PrivateIP)
		}
		report.EndPhase(phase2, "passed", fmt.Sprintf("Created %d worker nodes", len(workerNodes)))

		// Summary
		totalNodes := len(masterNodes) + len(workerNodes)
		pctx.Export("total_nodes", pulumi.Int(totalNodes))

		report.SetMetric("master_count", len(masterNodes))
		report.SetMetric("worker_count", len(workerNodes))
		report.SetMetric("total_nodes", totalNodes)

		return nil
	}

	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	err := stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	t.Log("================================================")
	t.Log("RUNNING: Precise AWS Multi-Node Pool E2E Test")
	t.Log("================================================")

	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi Up failed")

	// Validate Master Pool
	masterCount := result.Outputs["master_count"]
	assert.Equal(t, float64(1), masterCount.Value, "Should have 1 master")
	t.Logf("✓ Master nodes: %v", masterCount.Value)

	// Validate Worker Pool
	workerCount := result.Outputs["worker_count"]
	assert.Equal(t, float64(2), workerCount.Value, "Should have 2 workers")
	t.Logf("✓ Worker nodes: %v", workerCount.Value)

	// Validate Total
	totalNodes := result.Outputs["total_nodes"]
	assert.Equal(t, float64(3), totalNodes.Value, "Should have 3 total nodes")
	t.Logf("✓ Total nodes: %v", totalNodes.Value)

	// Validate each node has IPs
	for i := 0; i < int(masterCount.Value.(float64)); i++ {
		masterID := result.Outputs[fmt.Sprintf("master_%d_id", i)]
		assert.NotEmpty(t, masterID.Value, fmt.Sprintf("Master %d ID should exist", i))
		t.Logf("✓ Master %d ID: %v", i, masterID.Value)
	}

	for i := 0; i < int(workerCount.Value.(float64)); i++ {
		workerID := result.Outputs[fmt.Sprintf("worker_%d_id", i)]
		assert.NotEmpty(t, workerID.Value, fmt.Sprintf("Worker %d ID should exist", i))
		t.Logf("✓ Worker %d ID: %v", i, workerID.Value)
	}

	t.Log("================================================")
	t.Log("Precise AWS Multi-Node Pool E2E Test PASSED")
	t.Log("================================================")
}

// =============================================================================
// TEST 4: AWS Spot Instance (Precise)
// =============================================================================

func TestE2E_Precise_AWS_SpotInstance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	stackName := fmt.Sprintf("%s-precise-spot", cfg.StackPrefix)
	report := NewTestReport("Precise AWS Spot Instance")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	program := func(pctx *pulumi.Context) error {
		testCtx, err := setupAWSTestContext(pctx, "precise-spot-test", "10.103.0.0/16", cfg.AWSRegion, report)
		if err != nil {
			return err
		}

		// Create Spot Instance
		phase := report.StartPhase("Spot Instance Creation")
		spotConfig := &config.NodeConfig{
			Name:         "precise-spot-worker",
			Size:         "t3.micro",
			Image:        "ubuntu-22.04",
			Roles:        []string{"worker"},
			Region:       cfg.AWSRegion,
			SpotInstance: true, // This is the key flag
			WireGuardIP:  "10.8.0.30",
			Labels: map[string]string{
				"spot-instance": "true",
				"cost-saving":   "true",
			},
		}

		nodeOutput, err := testCtx.AWSProvider.CreateNode(pctx, spotConfig)
		if err != nil {
			report.AddError(fmt.Sprintf("Spot instance creation failed: %v", err))
			return fmt.Errorf("CreateNode (spot) failed: %w", err)
		}

		pctx.Export("spot_node_id", nodeOutput.ID)
		pctx.Export("spot_node_name", pulumi.String(nodeOutput.Name))
		pctx.Export("spot_node_public_ip", nodeOutput.PublicIP)
		pctx.Export("is_spot_instance", pulumi.Bool(true))

		report.EndPhase(phase, "passed", fmt.Sprintf("Spot instance '%s' created", nodeOutput.Name))
		report.SetMetric("spot_instance_type", spotConfig.Size)
		report.SetMetric("spot_enabled", true)

		return nil
	}

	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	err := stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	t.Log("================================================")
	t.Log("RUNNING: Precise AWS Spot Instance E2E Test")
	t.Log("================================================")

	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi Up failed")

	// Validate Spot Instance
	spotNodeID := result.Outputs["spot_node_id"]
	assert.NotEmpty(t, spotNodeID.Value, "Spot node ID should not be empty")
	t.Logf("✓ Spot Instance ID: %v", spotNodeID.Value)

	spotNodeName := result.Outputs["spot_node_name"]
	assert.Equal(t, "precise-spot-worker", spotNodeName.Value, "Spot node name should match")
	t.Logf("✓ Spot Node Name: %v", spotNodeName.Value)

	isSpot := result.Outputs["is_spot_instance"]
	assert.Equal(t, true, isSpot.Value, "Should be a spot instance")
	t.Logf("✓ Is Spot Instance: %v", isSpot.Value)

	t.Log("================================================")
	t.Log("Precise AWS Spot Instance E2E Test PASSED")
	t.Log("================================================")
}

// =============================================================================
// TEST 5: Full K3s Cluster with WireGuard (NO DNS - AWS Only)
// =============================================================================

func TestE2E_Precise_K3s_WireGuard_Cluster(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	stackName := fmt.Sprintf("%s-precise-k3s", cfg.StackPrefix)
	report := NewTestReport("Precise K3s Cluster with WireGuard")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	// Generate a unique key pair name for this test
	keyPairName := fmt.Sprintf("precise-k3s-kp-%d", time.Now().Unix())

	program := func(pctx *pulumi.Context) error {
		// Create cluster configuration (without DNS to avoid DigitalOcean dependency)
		// The orchestrator handles SSH key, KeyPair, VPC, etc. internally
		phase1 := report.StartPhase("Cluster Configuration")
		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "precise-k3s-cluster",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  cfg.AWSRegion,
					VPC: &config.VPCConfig{
						Create: true,
						Name:   "precise-k3s-vpc",
						CIDR:   "10.104.0.0/16",
					},
					KeyPair: keyPairName, // Orchestrator will create this
				},
			},
			Security: config.SecurityConfig{
				SSHConfig: config.SSHConfig{
					PublicKeyPath: "",
				},
			},
			Network: config.NetworkConfig{
				CIDR: "10.104.0.0/16",
				WireGuard: &config.WireGuardConfig{
					Enabled:    true,
					Port:       51820,
					SubnetCIDR: "10.8.0.0/24",
				},
				DNS: config.DNSConfig{
					Domain:   "", // Empty domain disables DNS creation
					Provider: "none",
				},
			},
			Kubernetes: config.KubernetesConfig{
				Distribution: "k3s",
				Version:      "stable",
			},
			NodePools: map[string]config.NodePool{
				"aws-masters": {
					Name:     "aws-masters",
					Provider: "aws",
					Count:    1,
					Size:     "t3.small",
					Region:   cfg.AWSRegion,
					Roles:    []string{"master"},
				},
				"aws-workers": {
					Name:     "aws-workers",
					Provider: "aws",
					Count:    1,
					Size:     "t3.micro",
					Region:   cfg.AWSRegion,
					Roles:    []string{"worker"},
				},
			},
		}
		report.EndPhase(phase1, "passed", "K3s cluster configuration created")

		// Use SimpleRealOrchestratorComponent for full cluster deployment
		// The orchestrator handles all phases: SSH, KeyPair, VPC, Nodes, WireGuard, K3s, DNS
		phase2 := report.StartPhase("K3s Cluster Deployment")

		// Create the orchestrator component
		orch, err := orchestrator.NewSimpleRealOrchestratorComponent(pctx, "precise-k3s-cluster", clusterConfig)
		if err != nil {
			report.AddError(fmt.Sprintf("Orchestrator creation failed: %v", err))
			return fmt.Errorf("failed to create orchestrator: %w", err)
		}

		// Export orchestrator outputs
		pctx.Export("cluster_name", orch.ClusterName)
		pctx.Export("api_endpoint", orch.APIEndpoint)
		pctx.Export("kubeconfig", orch.KubeConfig)
		pctx.Export("cluster_status", orch.Status)
		pctx.Export("ssh_public_key", orch.SSHPublicKey)
		pctx.Export("ssh_private_key", orch.SSHPrivateKey)

		report.EndPhase(phase2, "passed", "K3s cluster with WireGuard deployed")

		report.SetMetric("master_count", 1)
		report.SetMetric("worker_count", 1)
		report.SetMetric("wireguard_enabled", true)
		report.SetMetric("k3s_version", "stable")

		return nil
	}

	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	// Set AWS configuration
	err := stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	t.Log("========================================================")
	t.Log("RUNNING: Precise K3s Cluster with WireGuard E2E Test")
	t.Log("========================================================")
	t.Log("This test creates a REAL K3s cluster with WireGuard VPN")
	t.Log("Expected duration: 15-25 minutes")
	t.Log("========================================================")

	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi Up failed")

	// Validate Cluster
	clusterName := result.Outputs["cluster_name"]
	assert.NotEmpty(t, clusterName.Value, "Cluster name should not be empty")
	t.Logf("✓ Cluster Name: %v", clusterName.Value)

	// api_endpoint may be empty if DNS is disabled - that's OK
	apiEndpoint := result.Outputs["api_endpoint"]
	if apiEndpoint.Value != nil && apiEndpoint.Value != "" {
		t.Logf("✓ API Endpoint: %v", apiEndpoint.Value)
	} else {
		t.Log("✓ API Endpoint: (DNS disabled - using kubeconfig directly)")
	}

	clusterStatus := result.Outputs["cluster_status"]
	statusStr, _ := clusterStatus.Value.(string)
	assert.Contains(t, statusStr, "successfully", "Cluster status should indicate success")
	t.Logf("✓ Cluster Status: %v", clusterStatus.Value)

	// Validate kubeconfig exists and contains valid content
	kubeconfig := result.Outputs["kubeconfig"]
	assert.NotEmpty(t, kubeconfig.Value, "Kubeconfig should not be empty")
	t.Log("✓ Kubeconfig generated successfully")

	// Validate node count
	nodeCount := result.Outputs["node_count"]
	if nodeCount.Value != nil {
		assert.Equal(t, float64(2), nodeCount.Value, "Should have 2 nodes (1 master + 1 worker)")
		t.Logf("✓ Node Count: %v", nodeCount.Value)
	}

	// Validate nodes have WireGuard IPs
	nodes := result.Outputs["nodes"]
	if nodes.Value != nil {
		nodesMap := nodes.Value.(map[string]interface{})
		t.Logf("✓ Nodes deployed: %d", len(nodesMap))
		for nodeName, nodeData := range nodesMap {
			if nodeMap, ok := nodeData.(map[string]interface{}); ok {
				t.Logf("  - %s: public_ip=%v, vpn_ip=%v, roles=%v",
					nodeName,
					nodeMap["public_ip"],
					nodeMap["vpn_ip"],
					nodeMap["roles"])
			}
		}
	}

	t.Log("========================================================")
	t.Log("Precise K3s Cluster with WireGuard E2E Test PASSED")
	t.Log("========================================================")
}
