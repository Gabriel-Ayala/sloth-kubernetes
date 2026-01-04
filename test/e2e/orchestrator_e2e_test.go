// +build e2e

// Package e2e provides comprehensive end-to-end tests for orchestrator components
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
// SSH Key Component E2E Test
// Tests: SSH key generation using the REAL SSHKeyComponent
// =============================================================================

// TestE2E_SSHKeyComponent tests the SSH key generation component
func TestE2E_SSHKeyComponent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	cfg := loadE2EConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	stackName := fmt.Sprintf("%s-sshkey", cfg.StackPrefix)
	report := NewTestReport("SSH Key Component")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	program := func(pctx *pulumi.Context) error {
		phase1 := report.StartPhase("SSH Key Generation")

		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "e2e-sshkey-test",
			},
			Security: config.SecurityConfig{
				SSHConfig: config.SSHConfig{
					PublicKeyPath: "",
				},
			},
		}

		sshComponent, err := components.NewSSHKeyComponent(pctx, "e2e-ssh-test", clusterConfig)
		if err != nil {
			report.AddError(fmt.Sprintf("SSH key creation failed: %v", err))
			return fmt.Errorf("failed to create SSH key: %w", err)
		}

		pctx.Export("public_key", sshComponent.PublicKey)
		pctx.Export("private_key_path", sshComponent.PrivateKeyPath)

		report.EndPhase(phase1, "passed", "SSH keys generated successfully")
		return nil
	}

	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	t.Log("======================================")
	t.Log("RUNNING: SSH Key Component E2E Test")
	t.Log("======================================")
	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi Up failed")

	// Validate outputs
	publicKey, ok := result.Outputs["public_key"]
	assert.True(t, ok, "public_key output should exist")
	assert.NotEmpty(t, publicKey.Value, "Public key should not be empty")
	assert.Contains(t, publicKey.Value.(string), "ssh-rsa", "Should be an RSA public key")

	t.Logf("Generated public key: %v...", publicKey.Value.(string)[:50])

	t.Log("======================================")
	t.Log("SSH Key Component E2E Test PASSED")
	t.Log("======================================")
}

// =============================================================================
// AWS Provider Integration E2E Test
// Tests: Full integration with AWSProvider - VPC + Firewall + NodePool
// =============================================================================

// TestE2E_AWSProvider_Integration tests full AWS provider workflow
func TestE2E_AWSProvider_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	stackName := fmt.Sprintf("%s-provider-integ", cfg.StackPrefix)
	report := NewTestReport("AWS Provider Integration")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	program := func(pctx *pulumi.Context) error {
		// Phase 1: Create configuration
		phase1 := report.StartPhase("Configuration")
		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "e2e-provider-integ-test",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  cfg.AWSRegion,
					VPC: &config.VPCConfig{
						Create: true,
						Name:   "e2e-integ-vpc",
						CIDR:   "10.210.0.0/16",
					},
					KeyPair: "e2e-integ-key",
				},
			},
			Security: config.SecurityConfig{
				SSHConfig: config.SSHConfig{
					PublicKeyPath: "",
				},
			},
			Network: config.NetworkConfig{
				CIDR: "10.210.0.0/16",
				Firewall: &config.FirewallConfig{
					InboundRules: []config.FirewallRule{
						{Port: "22", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
						{Port: "6443", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
						{Port: "80", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
						{Port: "443", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
						{Port: "51820", Protocol: "udp", Source: []string{"0.0.0.0/0"}},
					},
				},
			},
		}
		report.EndPhase(phase1, "passed", "Configuration created")

		// Phase 2: Create SSH Keys
		phase2 := report.StartPhase("SSH Key Generation")
		sshComponent, err := components.NewSSHKeyComponent(pctx, "e2e-integ-ssh", clusterConfig)
		if err != nil {
			report.AddError(fmt.Sprintf("SSH key creation failed: %v", err))
			return fmt.Errorf("failed to create SSH key: %w", err)
		}
		pctx.Export("ssh_public_key", sshComponent.PublicKey)
		report.EndPhase(phase2, "passed", "SSH keys generated")

		// Phase 3: Initialize AWS Provider
		phase3 := report.StartPhase("AWS Provider Init")
		awsProvider := providers.NewAWSProvider()
		if err := awsProvider.Initialize(pctx, clusterConfig); err != nil {
			report.AddError(fmt.Sprintf("AWS provider init failed: %v", err))
			return fmt.Errorf("AWSProvider.Initialize failed: %w", err)
		}

		// Verify provider metadata
		providerName := awsProvider.GetName()
		if providerName != "aws" {
			report.AddError(fmt.Sprintf("Expected provider name 'aws', got '%s'", providerName))
		}

		regions := awsProvider.GetRegions()
		if len(regions) == 0 {
			report.AddError("Provider returned no regions")
		}

		sizes := awsProvider.GetSizes()
		if len(sizes) == 0 {
			report.AddError("Provider returned no sizes")
		}

		pctx.Export("provider_name", pulumi.String(providerName))
		pctx.Export("region_count", pulumi.Int(len(regions)))
		pctx.Export("size_count", pulumi.Int(len(sizes)))
		report.EndPhase(phase3, "passed", fmt.Sprintf("AWS provider initialized (%d regions, %d sizes)", len(regions), len(sizes)))

		// Phase 4: Create Network
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

		// Phase 5: Create Security Group
		phase5 := report.StartPhase("Security Group Creation")
		err = awsProvider.CreateFirewall(pctx, clusterConfig.Network.Firewall, nil)
		if err != nil {
			report.AddError(fmt.Sprintf("Security group creation failed: %v", err))
			return fmt.Errorf("AWSProvider.CreateFirewall failed: %w", err)
		}
		report.EndPhase(phase5, "passed", "Security group created")

		// Phase 6: Create Node Pool (multiple nodes)
		phase6 := report.StartPhase("Node Pool Creation")
		nodePool := &config.NodePool{
			Name:  "e2e-integration-pool",
			Count: 2,
			Size:  "t3.micro",
			Image: "ubuntu-22.04",
			Roles: []string{"worker"},
			Labels: map[string]string{
				"environment": "e2e-test",
				"test-type":   "integration",
			},
		}

		poolNodes, err := awsProvider.CreateNodePool(pctx, nodePool)
		if err != nil {
			report.AddError(fmt.Sprintf("Node pool creation failed: %v", err))
			return fmt.Errorf("AWSProvider.CreateNodePool failed: %w", err)
		}

		pctx.Export("pool_node_count", pulumi.Int(len(poolNodes)))
		for i, node := range poolNodes {
			pctx.Export(fmt.Sprintf("pool_node_%d_id", i), node.ID)
			pctx.Export(fmt.Sprintf("pool_node_%d_public_ip", i), node.PublicIP)
		}
		report.EndPhase(phase6, "passed", fmt.Sprintf("Node pool created with %d nodes", len(poolNodes)))

		// Set metrics
		report.SetMetric("vpc_cidr", clusterConfig.Providers.AWS.VPC.CIDR)
		report.SetMetric("subnet_count", len(networkOutput.Subnets))
		report.SetMetric("pool_size", nodePool.Count)

		return nil
	}

	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	err := stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	t.Log("==============================================")
	t.Log("RUNNING: AWS Provider Integration E2E Test")
	t.Log("==============================================")
	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi Up failed")

	// Validate outputs
	providerName, ok := result.Outputs["provider_name"]
	assert.True(t, ok, "provider_name output should exist")
	assert.Equal(t, "aws", providerName.Value)

	poolNodeCount, ok := result.Outputs["pool_node_count"]
	assert.True(t, ok, "pool_node_count output should exist")
	assert.Equal(t, float64(2), poolNodeCount.Value, "Should have 2 nodes in pool")

	t.Log("==============================================")
	t.Log("AWS Provider Integration E2E Test PASSED")
	t.Log("==============================================")
}

// =============================================================================
// Config Validation E2E Test
// Tests: Configuration validation without creating resources
// =============================================================================

// TestE2E_ConfigValidation tests the configuration validation logic
func TestE2E_ConfigValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	report := NewTestReport("Config Validation")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	testCases := []struct {
		name        string
		config      *config.ClusterConfig
		shouldPass  bool
		description string
	}{
		{
			name: "Valid AWS Configuration",
			config: &config.ClusterConfig{
				Metadata: config.Metadata{
					Name: "valid-cluster",
				},
				Providers: config.ProvidersConfig{
					AWS: &config.AWSProvider{
					Enabled: true,
						Region:  "us-east-1",
						VPC: &config.VPCConfig{
							Create: true,
							CIDR:   "10.0.0.0/16",
						},
					},
				},
				Network: config.NetworkConfig{
					CIDR: "10.0.0.0/16",
				},
			},
			shouldPass:  true,
			description: "Valid AWS config with VPC",
		},
		{
			name: "Valid Multi-Provider Configuration",
			config: &config.ClusterConfig{
				Metadata: config.Metadata{
					Name: "multi-provider-cluster",
				},
				Providers: config.ProvidersConfig{
					AWS: &config.AWSProvider{
					Enabled: true,
						Region:  "us-east-1",
					},
					DigitalOcean: &config.DigitalOceanProvider{
						Region:  "nyc1",
					},
				},
				Network: config.NetworkConfig{
					CIDR: "10.0.0.0/16",
				},
			},
			shouldPass:  true,
			description: "Valid multi-cloud config",
		},
		{
			name: "Configuration with Security Settings",
			config: &config.ClusterConfig{
				Metadata: config.Metadata{
					Name: "secure-cluster",
				},
				Providers: config.ProvidersConfig{
					AWS: &config.AWSProvider{
					Enabled: true,
						Region:  "eu-west-1",
					},
				},
				Security: config.SecurityConfig{
					Bastion: &config.BastionConfig{
						Enabled:  true,
						Provider: "aws",
						Name:     "bastion-host",
						Region:   "eu-west-1",
						Size:     "t3.micro",
					},
				},
				Network: config.NetworkConfig{
					CIDR: "10.0.0.0/16",
					Firewall: &config.FirewallConfig{
						InboundRules: []config.FirewallRule{
							{Port: "22", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
							{Port: "6443", Protocol: "tcp", Source: []string{"10.0.0.0/8"}},
						},
					},
				},
			},
			shouldPass:  true,
			description: "Valid config with bastion and firewall",
		},
		{
			name: "Configuration with Node Pools",
			config: &config.ClusterConfig{
				Metadata: config.Metadata{
					Name: "node-pool-cluster",
				},
				Providers: config.ProvidersConfig{
					AWS: &config.AWSProvider{
					Enabled: true,
						Region:  "us-west-2",
					},
				},
				Nodes: []config.NodeConfig{
					{
						Name:  "master-1",
						Size:  "t3.medium",
						Roles: []string{"master", "etcd"},
					},
					{
						Name:  "worker-1",
						Size:  "t3.large",
						Roles: []string{"worker"},
					},
					{
						Name:  "worker-2",
						Size:  "t3.large",
						Roles: []string{"worker"},
					},
				},
			},
			shouldPass:  true,
			description: "Valid config with defined nodes",
		},
		{
			name: "Configuration with Kubernetes Settings",
			config: &config.ClusterConfig{
				Metadata: config.Metadata{
					Name: "k8s-cluster",
				},
				Kubernetes: config.KubernetesConfig{
					Version: "v1.28.0",
					RKE2: &config.RKE2Config{
						ClusterToken: "my-cluster-token",
					},
				},
				Providers: config.ProvidersConfig{
					AWS: &config.AWSProvider{
					Enabled: true,
						Region:  "ap-southeast-1",
					},
				},
			},
			shouldPass:  true,
			description: "Valid config with Kubernetes settings",
		},
	}

	phase1 := report.StartPhase("Configuration Validation")
	passCount := 0
	failCount := 0

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Validate configuration structure
			valid := validateConfig(tc.config)

			if tc.shouldPass {
				if valid {
					t.Logf("PASS: %s - %s", tc.name, tc.description)
					passCount++
				} else {
					t.Errorf("FAIL: %s should be valid but validation failed", tc.name)
					failCount++
				}
			} else {
				if !valid {
					t.Logf("PASS: %s - correctly rejected invalid config", tc.name)
					passCount++
				} else {
					t.Errorf("FAIL: %s should be invalid but validation passed", tc.name)
					failCount++
				}
			}
		})
	}

	report.EndPhase(phase1, "passed", fmt.Sprintf("%d/%d tests passed", passCount, passCount+failCount))
	report.SetMetric("tests_passed", passCount)
	report.SetMetric("tests_failed", failCount)

	t.Logf("======================================")
	t.Logf("Config Validation E2E Test: %d passed, %d failed", passCount, failCount)
	t.Logf("======================================")
}

// validateConfig validates a cluster configuration
func validateConfig(cfg *config.ClusterConfig) bool {
	if cfg == nil {
		return false
	}
	if cfg.Metadata.Name == "" {
		return false
	}
	// Check that at least one provider is enabled
	hasProvider := false
	if cfg.Providers.AWS != nil && cfg.Providers.AWS.Enabled {
		hasProvider = true
	}
	if cfg.Providers.DigitalOcean != nil && cfg.Providers.DigitalOcean.Enabled {
		hasProvider = true
	}
	if cfg.Providers.Linode != nil && cfg.Providers.Linode.Enabled {
		hasProvider = true
	}
	if cfg.Providers.Azure != nil && cfg.Providers.Azure.Enabled {
		hasProvider = true
	}
	return hasProvider || len(cfg.Nodes) > 0
}

// =============================================================================
// Provider Factory E2E Test
// Tests: Provider factory and registration
// =============================================================================

// TestE2E_ProviderFactory tests the provider factory pattern
func TestE2E_ProviderFactory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	report := NewTestReport("Provider Factory")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	phase1 := report.StartPhase("Provider Instantiation")

	// Test AWS Provider
	t.Run("AWS Provider", func(t *testing.T) {
		awsProvider := providers.NewAWSProvider()
		assert.NotNil(t, awsProvider, "AWS provider should not be nil")
		assert.Equal(t, "aws", awsProvider.GetName())
		assert.Greater(t, len(awsProvider.GetRegions()), 0, "Should have AWS regions")
		assert.Greater(t, len(awsProvider.GetSizes()), 0, "Should have AWS instance sizes")
		t.Logf("AWS Provider: %d regions, %d sizes", len(awsProvider.GetRegions()), len(awsProvider.GetSizes()))
	})

	// Test DigitalOcean Provider
	t.Run("DigitalOcean Provider", func(t *testing.T) {
		doProvider := providers.NewDigitalOceanProvider()
		assert.NotNil(t, doProvider, "DigitalOcean provider should not be nil")
		assert.Equal(t, "digitalocean", doProvider.GetName())
		assert.Greater(t, len(doProvider.GetRegions()), 0, "Should have DO regions")
		assert.Greater(t, len(doProvider.GetSizes()), 0, "Should have DO droplet sizes")
		t.Logf("DigitalOcean Provider: %d regions, %d sizes", len(doProvider.GetRegions()), len(doProvider.GetSizes()))
	})

	// Test Linode Provider
	t.Run("Linode Provider", func(t *testing.T) {
		linodeProvider := providers.NewLinodeProvider()
		assert.NotNil(t, linodeProvider, "Linode provider should not be nil")
		assert.Equal(t, "linode", linodeProvider.GetName())
		assert.Greater(t, len(linodeProvider.GetRegions()), 0, "Should have Linode regions")
		assert.Greater(t, len(linodeProvider.GetSizes()), 0, "Should have Linode instance sizes")
		t.Logf("Linode Provider: %d regions, %d sizes", len(linodeProvider.GetRegions()), len(linodeProvider.GetSizes()))
	})

	// Test Azure Provider
	t.Run("Azure Provider", func(t *testing.T) {
		azureProvider := providers.NewAzureProvider()
		assert.NotNil(t, azureProvider, "Azure provider should not be nil")
		assert.Equal(t, "azure", azureProvider.GetName())
		assert.Greater(t, len(azureProvider.GetRegions()), 0, "Should have Azure regions")
		assert.Greater(t, len(azureProvider.GetSizes()), 0, "Should have Azure VM sizes")
		t.Logf("Azure Provider: %d regions, %d sizes", len(azureProvider.GetRegions()), len(azureProvider.GetSizes()))
	})

	report.EndPhase(phase1, "passed", "All providers instantiated successfully")

	t.Log("======================================")
	t.Log("Provider Factory E2E Test PASSED")
	t.Log("======================================")
}

// =============================================================================
// AWS Load Balancer E2E Test
// Tests: Network Load Balancer creation
// =============================================================================

// TestE2E_AWS_LoadBalancer tests load balancer creation
func TestE2E_AWS_LoadBalancer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	stackName := fmt.Sprintf("%s-loadbalancer", cfg.StackPrefix)
	report := NewTestReport("AWS Load Balancer")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	program := func(pctx *pulumi.Context) error {
		phase1 := report.StartPhase("Configuration")
		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "e2e-lb-test",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  cfg.AWSRegion,
					VPC: &config.VPCConfig{
						Create: true,
						Name:   "e2e-lb-vpc",
						CIDR:   "10.220.0.0/16",
					},
				},
			},
			Security: config.SecurityConfig{},
			Network: config.NetworkConfig{
				CIDR: "10.220.0.0/16",
				Firewall: &config.FirewallConfig{
					InboundRules: []config.FirewallRule{
						{Port: "22", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
						{Port: "6443", Protocol: "tcp", Source: []string{"0.0.0.0/0"}},
					},
				},
			},
		}
		report.EndPhase(phase1, "passed", "Configuration created")

		// Phase 2: Create SSH Keys
		phase2 := report.StartPhase("SSH Key Generation")
		_, err := components.NewSSHKeyComponent(pctx, "e2e-lb-ssh", clusterConfig)
		if err != nil {
			report.AddError(fmt.Sprintf("SSH key creation failed: %v", err))
			return fmt.Errorf("failed to create SSH key: %w", err)
		}
		report.EndPhase(phase2, "passed", "SSH keys generated")

		// Phase 3: Initialize Provider and Create Network
		phase3 := report.StartPhase("Network Setup")
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

		// Phase 5: Create Load Balancer
		phase5 := report.StartPhase("Load Balancer Creation")
		lbConfig := &config.LoadBalancerConfig{
			Name:     "e2e-test-nlb",
			Type:     "network",
			Provider: "aws",
			Ports: []config.PortConfig{
				{Name: "k8s-api", Port: 6443},
			},
		}

		lbOutput, err := awsProvider.CreateLoadBalancer(pctx, lbConfig)
		if err != nil {
			report.AddError(fmt.Sprintf("Load balancer creation failed: %v", err))
			return fmt.Errorf("AWSProvider.CreateLoadBalancer failed: %w", err)
		}

		pctx.Export("lb_id", lbOutput.ID)
		pctx.Export("lb_hostname", lbOutput.Hostname)
		pctx.Export("lb_ip", lbOutput.IP)
		pctx.Export("lb_status", lbOutput.Status)
		report.EndPhase(phase5, "passed", "Load balancer created")

		return nil
	}

	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	err := stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	t.Log("========================================")
	t.Log("RUNNING: AWS Load Balancer E2E Test")
	t.Log("========================================")
	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi Up failed")

	// Validate outputs
	lbHostname, ok := result.Outputs["lb_hostname"]
	assert.True(t, ok, "lb_hostname output should exist")
	t.Logf("Load Balancer Hostname: %v", lbHostname.Value)

	lbID, ok := result.Outputs["lb_id"]
	assert.True(t, ok, "lb_id output should exist")
	t.Logf("Load Balancer ID: %v", lbID.Value)

	t.Log("========================================")
	t.Log("AWS Load Balancer E2E Test PASSED")
	t.Log("========================================")
}
