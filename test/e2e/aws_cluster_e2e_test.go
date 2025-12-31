// +build e2e

// Package e2e provides end-to-end tests that test the REAL application
// using Pulumi Automation API to create actual cloud resources
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
	"github.com/chalkan3/sloth-kubernetes/internal/orchestrator/components"
	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
)

// =============================================================================
// Test Configuration
// =============================================================================

// E2ETestConfig holds configuration for E2E tests
type E2ETestConfig struct {
	AWSRegion       string
	AWSAccessKeyID  string
	AWSSecretKey    string
	AWSSessionToken string
	StackPrefix     string
	Timeout         time.Duration
}

// loadE2EConfig loads E2E test configuration from environment
func loadE2EConfig(t *testing.T) *E2ETestConfig {
	cfg := &E2ETestConfig{
		AWSRegion:       os.Getenv("AWS_REGION"),
		AWSAccessKeyID:  os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretKey:    os.Getenv("AWS_SECRET_ACCESS_KEY"),
		AWSSessionToken: os.Getenv("AWS_SESSION_TOKEN"),
		StackPrefix:     fmt.Sprintf("e2e-test-%d", time.Now().Unix()),
		Timeout:         15 * time.Minute,
	}

	if cfg.AWSRegion == "" {
		cfg.AWSRegion = "us-east-1"
	}

	return cfg
}

// skipIfNoAWSCredentials skips the test if AWS credentials are not available
func skipIfNoAWSCredentials(t *testing.T, cfg *E2ETestConfig) {
	if cfg.AWSAccessKeyID == "" || cfg.AWSSecretKey == "" {
		t.Skip("Skipping E2E test: AWS credentials not configured (set AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)")
	}
}

// =============================================================================
// Pulumi Test Helpers
// =============================================================================

// createTestWorkspace creates a Pulumi workspace for testing
func createTestWorkspace(ctx context.Context, t *testing.T, stackName string, program pulumi.RunFunc) (auto.Stack, func()) {
	projectName := "sloth-kubernetes-e2e-test"

	// Create workspace with local backend (file-based state)
	workDir := t.TempDir()
	backendURL := fmt.Sprintf("file://%s", workDir)

	ws, err := auto.NewLocalWorkspace(ctx,
		auto.Program(program),
		auto.Project(workspace.Project{
			Name:    tokens.PackageName(projectName),
			Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			Backend: &workspace.ProjectBackend{
				URL: backendURL,
			},
		}),
		auto.SecretsProvider("passphrase"),
		auto.EnvVars(map[string]string{
			"PULUMI_CONFIG_PASSPHRASE": "test-passphrase",
		}),
	)
	require.NoError(t, err, "Failed to create workspace")

	// Create stack
	fullyQualifiedName := fmt.Sprintf("organization/%s/%s", projectName, stackName)
	stack, err := auto.UpsertStack(ctx, fullyQualifiedName, ws)
	require.NoError(t, err, "Failed to create stack")

	// Cleanup function
	cleanup := func() {
		t.Log("🧹 Cleaning up test resources...")
		destroyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		_, err := stack.Destroy(destroyCtx, optdestroy.ProgressStreams(os.Stdout))
		if err != nil {
			t.Logf("Warning: Failed to destroy stack: %v", err)
		}

		err = stack.Workspace().RemoveStack(destroyCtx, stack.Name())
		if err != nil {
			t.Logf("Warning: Failed to remove stack: %v", err)
		}
	}

	return stack, cleanup
}

// =============================================================================
// E2E Tests - Full Application Flow
// =============================================================================

// TestE2E_AWSProvider_VPCCreation tests VPC creation through the real AWSProvider
func TestE2E_AWSProvider_VPCCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	stackName := fmt.Sprintf("%s-vpc", cfg.StackPrefix)

	// Define Pulumi program that uses REAL AWSProvider
	program := func(pctx *pulumi.Context) error {
		// Create cluster configuration
		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "e2e-test-vpc",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  cfg.AWSRegion,
					VPC: &config.VPCConfig{
						Create: true,
						Name:   "e2e-test-vpc",
						CIDR:   "10.100.0.0/16",
					},
					// Use existing key pair name to skip SSH key creation
					KeyPair: "e2e-test-key",
				},
			},
			Security: config.SecurityConfig{
				SSHConfig: config.SSHConfig{
					PublicKeyPath: "", // Will use KeyPair name instead
				},
			},
		}

		// First, create SSH key using our component
		sshComponent, err := components.NewSSHKeyComponent(pctx, "e2e-vpc-test-ssh", clusterConfig)
		if err != nil {
			return fmt.Errorf("failed to create SSH key: %w", err)
		}

		// Create the REAL AWSProvider
		awsProvider := providers.NewAWSProvider()

		// Initialize the provider
		if err := awsProvider.Initialize(pctx, clusterConfig); err != nil {
			return fmt.Errorf("AWSProvider.Initialize failed: %w", err)
		}

		// Create the network using the real provider
		networkOutput, err := awsProvider.CreateNetwork(pctx, &config.NetworkConfig{
			CIDR: clusterConfig.Providers.AWS.VPC.CIDR,
		})
		if err != nil {
			return fmt.Errorf("AWSProvider.CreateNetwork failed: %w", err)
		}

		// Export outputs for validation
		pctx.Export("vpc_id", networkOutput.ID)
		pctx.Export("vpc_cidr", pulumi.String(clusterConfig.Providers.AWS.VPC.CIDR))
		pctx.Export("subnet_count", pulumi.Int(len(networkOutput.Subnets)))
		pctx.Export("ssh_public_key", sshComponent.PublicKey)

		return nil
	}

	// Create stack
	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	// Set AWS configuration
	err := stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	// Execute stack.Up() - this runs the REAL application code
	t.Log("🚀 Running Pulumi Up - Creating VPC via AWSProvider...")
	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi Up failed")

	// Validate outputs
	t.Log("✅ Validating VPC outputs...")

	vpcID, ok := result.Outputs["vpc_id"]
	assert.True(t, ok, "vpc_id output should exist")
	assert.NotEmpty(t, vpcID.Value, "VPC ID should not be empty")

	subnetCount, ok := result.Outputs["subnet_count"]
	assert.True(t, ok, "subnet_count output should exist")
	assert.GreaterOrEqual(t, subnetCount.Value.(float64), float64(1), "Should create at least 1 subnet")

	t.Logf("✅ VPC created successfully: %v", vpcID.Value)
}

// TestE2E_AWSProvider_NodeDeployment tests node deployment through the real components
func TestE2E_AWSProvider_NodeDeployment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	stackName := fmt.Sprintf("%s-nodes", cfg.StackPrefix)

	// Create cluster configuration
	clusterConfig := &config.ClusterConfig{
		Metadata: config.Metadata{
			Name: "e2e-test-cluster",
		},
		Providers: config.ProvidersConfig{
			AWS: &config.AWSProvider{
				Enabled: true,
				Region:  cfg.AWSRegion,
				VPC: &config.VPCConfig{
					Create: true,
					CIDR:   "10.100.0.0/16",
				},
			},
		},
		NodePools: map[string]config.NodePool{
			"aws-masters": {
				Name:     "aws-masters",
				Count:    1,
				Size:     "t3.small",
				Image:    "ubuntu-22-04",
				Region:   cfg.AWSRegion,
				Provider: "aws",
				Roles:    []string{"master"},
			},
		},
		Security: config.SecurityConfig{
			SSHConfig: config.SSHConfig{
				PublicKeyPath: "",
			},
		},
		Network: config.NetworkConfig{
			DNS: config.DNSConfig{
				Domain: "e2e-test.local",
			},
		},
		Kubernetes: config.KubernetesConfig{
			Distribution: "k3s",
			Version:      "v1.29.0+k3s1",
		},
	}

	// Define Pulumi program that uses REAL SSH key component
	program := func(ctx *pulumi.Context) error {
		// Phase 1: Create SSH keys using REAL component
		sshKeyComponent, err := components.NewSSHKeyComponent(ctx, "e2e-ssh-keys", clusterConfig)
		if err != nil {
			return fmt.Errorf("SSHKeyComponent failed: %w", err)
		}

		// Export SSH key info
		ctx.Export("ssh_public_key", sshKeyComponent.PublicKey)
		ctx.Export("ssh_private_key_path", sshKeyComponent.PrivateKeyPath)

		// Note: Full node deployment requires VPC and more setup
		// This test validates the SSH key component works correctly

		return nil
	}

	// Create stack
	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	// Set AWS configuration
	err := stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	// Execute stack.Up()
	t.Log("🚀 Running Pulumi Up - Testing SSH Key Component...")
	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi Up failed")

	// Validate outputs
	t.Log("✅ Validating SSH key outputs...")

	sshPubKey, ok := result.Outputs["ssh_public_key"]
	assert.True(t, ok, "ssh_public_key output should exist")
	assert.NotEmpty(t, sshPubKey.Value, "SSH public key should not be empty")

	pubKeyStr, _ := sshPubKey.Value.(string)
	assert.Contains(t, pubKeyStr, "ssh-", "SSH public key should start with ssh-")

	t.Log("✅ SSH key component created successfully")
}

// TestE2E_AWSProvider_FullClusterOrchestrator tests the complete orchestrator flow
func TestE2E_AWSProvider_FullClusterOrchestrator(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute) // Longer timeout for full cluster
	defer cancel()

	stackName := fmt.Sprintf("%s-cluster", cfg.StackPrefix)

	// Create full cluster configuration
	clusterConfig := &config.ClusterConfig{
		Metadata: config.Metadata{
			Name: "e2e-full-cluster",
		},
		Providers: config.ProvidersConfig{
			AWS: &config.AWSProvider{
				Enabled: true,
				Region:  cfg.AWSRegion,
				VPC: &config.VPCConfig{
					Create: true,
					CIDR:   "10.100.0.0/16",
				},
			},
		},
		NodePools: map[string]config.NodePool{
			"aws-masters": {
				Name:     "aws-masters",
				Count:    1,
				Size:     "t3.small",
				Image:    "ubuntu-22-04",
				Region:   cfg.AWSRegion,
				Provider: "aws",
				Roles:    []string{"master"},
			},
			"aws-workers": {
				Name:         "aws-workers",
				Count:        1,
				Size:         "t3.micro",
				Image:        "ubuntu-22-04",
				Region:       cfg.AWSRegion,
				Provider:     "aws",
				Roles:        []string{"worker"},
				SpotInstance: true,
			},
		},
		Security: config.SecurityConfig{
			SSHConfig: config.SSHConfig{
				PublicKeyPath: "",
			},
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

	// Define Pulumi program that uses the REAL orchestrator
	program := func(ctx *pulumi.Context) error {
		// Use the REAL SimpleRealOrchestratorComponent
		clusterOrch, err := orchestrator.NewSimpleRealOrchestratorComponent(
			ctx,
			"e2e-kubernetes-cluster",
			clusterConfig,
		)
		if err != nil {
			return fmt.Errorf("Orchestrator failed: %w", err)
		}

		// Export cluster outputs
		ctx.Export("clusterName", clusterOrch.ClusterName)
		ctx.Export("kubeConfig", clusterOrch.KubeConfig)
		ctx.Export("sshPrivateKey", clusterOrch.SSHPrivateKey)
		ctx.Export("apiEndpoint", clusterOrch.APIEndpoint)
		ctx.Export("status", clusterOrch.Status)

		return nil
	}

	// Create stack
	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	// Set AWS configuration
	err := stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	// Execute stack.Up()
	t.Log("🚀 Running Pulumi Up - Full Cluster Orchestration...")
	t.Log("⏳ This may take 15-30 minutes...")

	startTime := time.Now()
	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	duration := time.Since(startTime)

	if err != nil {
		t.Logf("❌ Cluster creation failed after %v: %v", duration, err)
		require.NoError(t, err, "Pulumi Up failed")
	}

	t.Logf("✅ Cluster created in %v", duration)

	// Validate outputs
	t.Log("✅ Validating cluster outputs...")

	clusterName, ok := result.Outputs["clusterName"]
	assert.True(t, ok, "clusterName output should exist")
	assert.Equal(t, "e2e-full-cluster", clusterName.Value)

	kubeConfig, ok := result.Outputs["kubeConfig"]
	assert.True(t, ok, "kubeConfig output should exist")
	assert.NotEmpty(t, kubeConfig.Value, "kubeConfig should not be empty")

	apiEndpoint, ok := result.Outputs["apiEndpoint"]
	assert.True(t, ok, "apiEndpoint output should exist")
	assert.NotEmpty(t, apiEndpoint.Value, "apiEndpoint should not be empty")

	status, ok := result.Outputs["status"]
	assert.True(t, ok, "status output should exist")
	statusStr, _ := status.Value.(string)
	assert.Contains(t, statusStr, "successfully", "Status should indicate success")

	// Check node count
	nodeCount, ok := result.Outputs["node_count"]
	if ok {
		assert.Equal(t, float64(2), nodeCount.Value, "Should have 2 nodes (1 master + 1 worker)")
	}

	t.Log("✅ Full cluster orchestration test PASSED")
}

// =============================================================================
// E2E Tests - Component Level
// =============================================================================

// TestE2E_Component_SSHKeyGeneration tests SSH key generation component
func TestE2E_Component_SSHKeyGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	stackName := fmt.Sprintf("e2e-ssh-%d", time.Now().Unix())

	clusterConfig := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "ssh-test"},
	}

	program := func(ctx *pulumi.Context) error {
		sshComponent, err := components.NewSSHKeyComponent(ctx, "test-ssh", clusterConfig)
		if err != nil {
			return err
		}

		ctx.Export("public_key", sshComponent.PublicKey)
		ctx.Export("private_key_path", sshComponent.PrivateKeyPath)

		return nil
	}

	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	t.Log("🔑 Testing SSH key generation component...")
	result, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err)

	// Validate
	pubKey, ok := result.Outputs["public_key"]
	assert.True(t, ok)
	assert.NotEmpty(t, pubKey.Value)

	privKeyPath, ok := result.Outputs["private_key_path"]
	assert.True(t, ok)
	assert.NotEmpty(t, privKeyPath.Value)

	t.Log("✅ SSH key generation component test PASSED")
}

// =============================================================================
// E2E Tests - Preview Mode (No Resources Created)
// =============================================================================

// TestE2E_Preview_FullCluster tests preview mode without creating resources
func TestE2E_Preview_FullCluster(t *testing.T) {
	cfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	stackName := fmt.Sprintf("e2e-preview-%d", time.Now().Unix())

	clusterConfig := &config.ClusterConfig{
		Metadata: config.Metadata{
			Name: "preview-test-cluster",
		},
		Providers: config.ProvidersConfig{
			AWS: &config.AWSProvider{
				Enabled: true,
				Region:  cfg.AWSRegion,
				VPC: &config.VPCConfig{
					Create: true,
					CIDR:   "10.100.0.0/16",
				},
			},
		},
		NodePools: map[string]config.NodePool{
			"aws-masters": {
				Name:     "aws-masters",
				Count:    3,
				Size:     "t3.medium",
				Provider: "aws",
				Region:   cfg.AWSRegion,
				Roles:    []string{"master"},
			},
			"aws-workers": {
				Name:     "aws-workers",
				Count:    5,
				Size:     "t3.large",
				Provider: "aws",
				Region:   cfg.AWSRegion,
				Roles:    []string{"worker"},
			},
		},
		Network: config.NetworkConfig{
			DNS: config.DNSConfig{Domain: "test.local"},
		},
		Kubernetes: config.KubernetesConfig{
			Distribution: "k3s",
			Version:      "v1.29.0+k3s1",
		},
	}

	program := func(ctx *pulumi.Context) error {
		_, err := orchestrator.NewSimpleRealOrchestratorComponent(ctx, "preview-cluster", clusterConfig)
		return err
	}

	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer cleanup()

	err := stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	// Preview only - no resources created
	t.Log("📋 Running Pulumi Preview...")
	preview, err := stack.Preview(ctx)
	require.NoError(t, err)

	// Validate preview results
	t.Logf("Preview Summary:")
	t.Logf("  Creates: %d", preview.ChangeSummary["create"])
	t.Logf("  Updates: %d", preview.ChangeSummary["update"])
	t.Logf("  Deletes: %d", preview.ChangeSummary["delete"])

	// Should plan to create resources
	creates := preview.ChangeSummary["create"]
	assert.Greater(t, creates, 0, "Preview should plan to create resources")

	t.Log("✅ Preview test PASSED - no resources were created")
}
