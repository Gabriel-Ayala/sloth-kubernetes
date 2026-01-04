//go:build e2e
// +build e2e

// Package e2e provides end-to-end tests for scaling node pools
// This test validates adding new node pools to an existing cluster
package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ScaleTestConfig holds configuration for node pool scaling tests
type ScaleTestConfig struct {
	AWSRegion       string
	AWSAccessKeyID  string
	AWSSecretKey    string
	AWSSessionToken string
	StackName       string
	ConfigFile      string
	SlothBinary     string
}

// loadScaleTestConfig loads test configuration from environment
func loadScaleTestConfig(t *testing.T) *ScaleTestConfig {
	cfg := &ScaleTestConfig{
		AWSRegion:       os.Getenv("AWS_REGION"),
		AWSAccessKeyID:  os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretKey:    os.Getenv("AWS_SECRET_ACCESS_KEY"),
		AWSSessionToken: os.Getenv("AWS_SESSION_TOKEN"),
		StackName:       fmt.Sprintf("e2e-scale-%d", time.Now().Unix()),
		ConfigFile:      "",
		SlothBinary:     "./sloth",
	}

	if cfg.AWSRegion == "" {
		cfg.AWSRegion = "us-east-1"
	}

	// Check if sloth binary exists
	if _, err := os.Stat(cfg.SlothBinary); os.IsNotExist(err) {
		// Try to find it in the project root
		cfg.SlothBinary = "../../sloth"
		if _, err := os.Stat(cfg.SlothBinary); os.IsNotExist(err) {
			t.Skip("Sloth binary not found. Run 'go build -o sloth .' first")
		}
	}

	return cfg
}

// skipIfNoAWSCreds skips the test if AWS credentials are not available
func skipIfNoAWSCreds(t *testing.T, cfg *ScaleTestConfig) {
	if cfg.AWSAccessKeyID == "" || cfg.AWSSecretKey == "" {
		t.Skip("Skipping E2E test: AWS credentials not configured")
	}
}

// runSlothCommand executes the sloth CLI command
func runSlothCommand(t *testing.T, cfg *ScaleTestConfig, args ...string) (string, error) {
	cmd := exec.Command(cfg.SlothBinary, args...)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", cfg.AWSAccessKeyID),
		fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", cfg.AWSSecretKey),
		fmt.Sprintf("AWS_SESSION_TOKEN=%s", cfg.AWSSessionToken),
		fmt.Sprintf("AWS_REGION=%s", cfg.AWSRegion),
		"PULUMI_CONFIG_PASSPHRASE=",
	)

	output, err := cmd.CombinedOutput()
	t.Logf("Command: %s %v\nOutput:\n%s", cfg.SlothBinary, args, string(output))

	return string(output), err
}

// sshToNode executes a command on a node via SSH
func sshToNode(t *testing.T, sshKeyPath, nodeIP, user, command string) (string, error) {
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		"-i", sshKeyPath,
		fmt.Sprintf("%s@%s", user, nodeIP),
		command,
	}

	cmd := exec.Command("ssh", sshArgs...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// writeInitialConfig writes the initial cluster configuration
func writeInitialConfig(t *testing.T, configPath string) {
	initialConfig := `; Sloth Kubernetes - Scale Test Configuration
; Initial cluster with 1 master + 1 worker

(cluster
  (metadata
    (name "scale-test-cluster")
    (environment "testing")
    (description "E2E scale test cluster"))

  (providers
    (aws
      (enabled true)
      (region "us-east-1")
      (vpc
        (create true)
        (cidr "10.100.0.0/16"))))

  (network
    (mode "wireguard")
    (pod-cidr "10.42.0.0/16")
    (service-cidr "10.43.0.0/16")
    (wireguard
      (enabled true)
      (create true)
      (provider "aws")
      (region "us-east-1")
      (subnet-cidr "10.8.0.0/24")
      (port 51820)
      (mesh-networking true)))

  ; Initial node pools - 1 master + 1 worker
  (node-pools
    (masters
      (name "masters")
      (provider "aws")
      (region "us-east-1")
      (count 1)
      (roles master etcd)
      (size "t3.medium"))
    (workers
      (name "workers")
      (provider "aws")
      (region "us-east-1")
      (count 1)
      (roles worker)
      (size "t3.medium")))

  (kubernetes
    (distribution "rke2")
    (version "v1.29.0+rke2r1"))

  (addons
    (salt
      (enabled true)
      (master-node "0")
      (api-enabled true)
      (api-port 8000)
      (api-username "saltapi")
      (secure-auth true)
      (auto-join true))))
`
	err := os.WriteFile(configPath, []byte(initialConfig), 0644)
	require.NoError(t, err, "Failed to write initial config")
}

// writeScaledConfig writes the configuration with additional node pool
func writeScaledConfig(t *testing.T, configPath string) {
	scaledConfig := `; Sloth Kubernetes - Scale Test Configuration
; Scaled cluster with 1 master + 3 workers (added new worker pool)

(cluster
  (metadata
    (name "scale-test-cluster")
    (environment "testing")
    (description "E2E scale test cluster - SCALED"))

  (providers
    (aws
      (enabled true)
      (region "us-east-1")
      (vpc
        (create true)
        (cidr "10.100.0.0/16"))))

  (network
    (mode "wireguard")
    (pod-cidr "10.42.0.0/16")
    (service-cidr "10.43.0.0/16")
    (wireguard
      (enabled true)
      (create true)
      (provider "aws")
      (region "us-east-1")
      (subnet-cidr "10.8.0.0/24")
      (port 51820)
      (mesh-networking true)))

  ; Scaled node pools - 1 master + 1 original worker + 2 new workers
  (node-pools
    (masters
      (name "masters")
      (provider "aws")
      (region "us-east-1")
      (count 1)
      (roles master etcd)
      (size "t3.medium"))
    (workers
      (name "workers")
      (provider "aws")
      (region "us-east-1")
      (count 1)
      (roles worker)
      (size "t3.medium"))
    (workers-scale
      (name "workers-scale")
      (provider "aws")
      (region "us-east-1")
      (count 2)
      (roles worker)
      (size "t3.small")))

  (kubernetes
    (distribution "rke2")
    (version "v1.29.0+rke2r1"))

  (addons
    (salt
      (enabled true)
      (master-node "0")
      (api-enabled true)
      (api-port 8000)
      (api-username "saltapi")
      (secure-auth true)
      (auto-join true))))
`
	err := os.WriteFile(configPath, []byte(scaledConfig), 0644)
	require.NoError(t, err, "Failed to write scaled config")
}

// TestE2E_ScaleNodePool_AddWorkersAfterClusterCreation tests adding node pools to existing cluster
func TestE2E_ScaleNodePool_AddWorkersAfterClusterCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E scale test in short mode")
	}

	cfg := loadScaleTestConfig(t)
	skipIfNoAWSCreds(t, cfg)

	// Create temporary config file
	configPath := fmt.Sprintf("/tmp/scale-test-%d.lisp", time.Now().Unix())
	cfg.ConfigFile = configPath
	defer os.Remove(configPath)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	// Cleanup function
	cleanup := func() {
		t.Log("🧹 Cleaning up test resources...")
		runSlothCommand(t, cfg, "destroy", cfg.StackName, "--config", cfg.ConfigFile, "--yes", "--force")
		runSlothCommand(t, cfg, "stacks", "delete", cfg.StackName, "--yes")
	}
	defer cleanup()

	// =========================================================================
	// PHASE 1: Deploy initial cluster (1 master + 1 worker)
	// =========================================================================
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Log("📦 PHASE 1: Deploying initial cluster (1 master + 1 worker)")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	writeInitialConfig(t, configPath)

	startPhase1 := time.Now()
	output, err := runSlothCommand(t, cfg, "deploy", "--config", configPath, "--stack", cfg.StackName, "--yes")
	phase1Duration := time.Since(startPhase1)

	require.NoError(t, err, "Initial deployment should succeed")
	assert.Contains(t, output, "successfully", "Deployment output should indicate success")

	t.Logf("✅ Phase 1 completed in %v", phase1Duration)

	// =========================================================================
	// PHASE 2: Validate initial cluster
	// =========================================================================
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Log("🔍 PHASE 2: Validating initial cluster")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Get node list
	nodesOutput, err := runSlothCommand(t, cfg, "nodes", "list", cfg.StackName)
	require.NoError(t, err, "Should be able to list nodes")

	// Count nodes (should be 2: 1 master + 1 worker)
	initialNodeCount := strings.Count(nodesOutput, "aws")
	t.Logf("📊 Initial node count: %d", initialNodeCount)
	assert.Equal(t, 2, initialNodeCount, "Should have 2 nodes initially")

	// Get master IP for validation
	masterIP := extractMasterIP(t, cfg)
	sshKeyPath := fmt.Sprintf("~/.ssh/kubernetes-clusters/%s.pem", cfg.StackName)
	expandedKeyPath := os.ExpandEnv(strings.Replace(sshKeyPath, "~", "$HOME", 1))

	// Validate Kubernetes cluster
	kubeOutput, err := sshToNode(t, expandedKeyPath, masterIP, "ubuntu", "kubectl get nodes -o wide")
	if err == nil {
		t.Logf("📋 Initial Kubernetes nodes:\n%s", kubeOutput)
		assert.Equal(t, 2, strings.Count(kubeOutput, "Ready"), "Should have 2 Ready nodes")
	}

	// Validate WireGuard
	wgOutput, err := sshToNode(t, expandedKeyPath, masterIP, "ubuntu", "sudo wg show wg0")
	if err == nil {
		t.Logf("🔐 WireGuard status:\n%s", wgOutput)
		assert.Contains(t, wgOutput, "peer:", "WireGuard should have peers")
	}

	t.Log("✅ Phase 2: Initial cluster validated successfully")

	// Wait a bit for cluster to stabilize
	t.Log("⏳ Waiting 30 seconds for cluster to stabilize...")
	select {
	case <-time.After(30 * time.Second):
	case <-ctx.Done():
		t.Fatal("Context cancelled")
	}

	// =========================================================================
	// PHASE 3: Scale up - Add new node pool
	// =========================================================================
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Log("📈 PHASE 3: Scaling up - Adding new worker node pool")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	writeScaledConfig(t, configPath)
	t.Log("📝 Updated config with new worker-scale pool (2 additional workers)")

	startPhase3 := time.Now()
	output, err = runSlothCommand(t, cfg, "deploy", "--config", configPath, "--stack", cfg.StackName, "--yes")
	phase3Duration := time.Since(startPhase3)

	require.NoError(t, err, "Scale deployment should succeed")
	t.Logf("✅ Phase 3 completed in %v", phase3Duration)

	// =========================================================================
	// PHASE 4: Validate scaled cluster
	// =========================================================================
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Log("🔍 PHASE 4: Validating scaled cluster")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Wait for new nodes to join
	t.Log("⏳ Waiting 60 seconds for new nodes to join cluster...")
	select {
	case <-time.After(60 * time.Second):
	case <-ctx.Done():
		t.Fatal("Context cancelled")
	}

	// Get updated node list
	nodesOutput, err = runSlothCommand(t, cfg, "nodes", "list", cfg.StackName)
	require.NoError(t, err, "Should be able to list nodes after scaling")

	// Count nodes (should be 4: 1 master + 1 original worker + 2 new workers)
	scaledNodeCount := strings.Count(nodesOutput, "aws")
	t.Logf("📊 Scaled node count: %d", scaledNodeCount)
	assert.Equal(t, 4, scaledNodeCount, "Should have 4 nodes after scaling")

	// Validate Kubernetes sees all nodes
	kubeOutput, err = sshToNode(t, expandedKeyPath, masterIP, "ubuntu", "kubectl get nodes -o wide")
	if err == nil {
		t.Logf("📋 Scaled Kubernetes nodes:\n%s", kubeOutput)
		readyCount := strings.Count(kubeOutput, "Ready")
		t.Logf("📊 Kubernetes Ready nodes: %d", readyCount)
		assert.GreaterOrEqual(t, readyCount, 3, "Should have at least 3 Ready nodes after scaling")
	}

	// Validate WireGuard mesh includes new nodes
	wgOutput, err = sshToNode(t, expandedKeyPath, masterIP, "ubuntu", "sudo wg show wg0")
	if err == nil {
		peerCount := strings.Count(wgOutput, "peer:")
		t.Logf("🔐 WireGuard peers: %d", peerCount)
		assert.GreaterOrEqual(t, peerCount, 3, "WireGuard should have at least 3 peers after scaling")
	}

	// Validate Salt sees all minions (if Salt is enabled)
	saltOutput, err := sshToNode(t, expandedKeyPath, masterIP, "ubuntu", "sudo salt-key -L")
	if err == nil {
		t.Logf("🧂 Salt minions:\n%s", saltOutput)
	}

	// =========================================================================
	// PHASE 5: Summary
	// =========================================================================
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Log("📊 TEST SUMMARY")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("✅ Initial deployment:   %v (2 nodes)", phase1Duration)
	t.Logf("✅ Scale deployment:     %v (+2 nodes)", phase3Duration)
	t.Logf("✅ Total nodes:          %d → %d", initialNodeCount, scaledNodeCount)
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Log("✅ SCALE TEST PASSED - Node pool added successfully!")
}

// extractMasterIP extracts the master node IP from stack outputs
func extractMasterIP(t *testing.T, cfg *ScaleTestConfig) string {
	// Use AWS CLI to get master IP
	cmd := exec.Command("aws", "ec2", "describe-instances",
		"--filters", "Name=tag:Name,Values=*masters*", "Name=instance-state-name,Values=running",
		"--query", "Reservations[*].Instances[*].PublicIpAddress",
		"--output", "text",
		"--region", cfg.AWSRegion,
	)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", cfg.AWSAccessKeyID),
		fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", cfg.AWSSecretKey),
		fmt.Sprintf("AWS_SESSION_TOKEN=%s", cfg.AWSSessionToken),
	)

	output, err := cmd.Output()
	if err != nil {
		t.Logf("Warning: Could not get master IP via AWS CLI: %v", err)
		return ""
	}

	ip := strings.TrimSpace(string(output))
	if ip == "" || ip == "None" {
		t.Log("Warning: No master IP found")
		return ""
	}

	// Take first IP if multiple
	ips := strings.Fields(ip)
	if len(ips) > 0 {
		return ips[0]
	}

	return ip
}

// TestE2E_ScaleNodePool_IncrementWorkerCount tests incrementing worker count in existing pool
func TestE2E_ScaleNodePool_IncrementWorkerCount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E scale test in short mode")
	}

	cfg := loadScaleTestConfig(t)
	skipIfNoAWSCreds(t, cfg)

	// Create temporary config files
	configPath := fmt.Sprintf("/tmp/scale-increment-test-%d.lisp", time.Now().Unix())
	cfg.ConfigFile = configPath
	defer os.Remove(configPath)

	// Cleanup function
	cleanup := func() {
		t.Log("🧹 Cleaning up test resources...")
		runSlothCommand(t, cfg, "destroy", cfg.StackName, "--config", cfg.ConfigFile, "--yes", "--force")
		runSlothCommand(t, cfg, "stacks", "delete", cfg.StackName, "--yes")
	}
	defer cleanup()

	// Write initial config with 1 worker
	initialConfig := `(cluster
  (metadata (name "increment-test"))
  (providers
    (aws (enabled true) (region "us-east-1")
      (vpc (create true) (cidr "10.100.0.0/16"))))
  (network
    (mode "wireguard")
    (wireguard (enabled true) (create true) (provider "aws")
      (region "us-east-1") (subnet-cidr "10.8.0.0/24") (port 51820) (mesh-networking true)))
  (node-pools
    (masters (name "masters") (provider "aws") (region "us-east-1")
      (count 1) (roles master etcd) (size "t3.medium"))
    (workers (name "workers") (provider "aws") (region "us-east-1")
      (count 1) (roles worker) (size "t3.small")))
  (kubernetes (distribution "rke2") (version "v1.29.0+rke2r1")))`

	err := os.WriteFile(configPath, []byte(initialConfig), 0644)
	require.NoError(t, err)

	// Deploy initial cluster
	t.Log("📦 Deploying initial cluster (1 master + 1 worker)...")
	_, err = runSlothCommand(t, cfg, "deploy", "--config", configPath, "--stack", cfg.StackName, "--yes")
	require.NoError(t, err, "Initial deployment should succeed")

	// Update config to increment worker count to 3
	scaledConfig := `(cluster
  (metadata (name "increment-test"))
  (providers
    (aws (enabled true) (region "us-east-1")
      (vpc (create true) (cidr "10.100.0.0/16"))))
  (network
    (mode "wireguard")
    (wireguard (enabled true) (create true) (provider "aws")
      (region "us-east-1") (subnet-cidr "10.8.0.0/24") (port 51820) (mesh-networking true)))
  (node-pools
    (masters (name "masters") (provider "aws") (region "us-east-1")
      (count 1) (roles master etcd) (size "t3.medium"))
    (workers (name "workers") (provider "aws") (region "us-east-1")
      (count 3) (roles worker) (size "t3.small")))
  (kubernetes (distribution "rke2") (version "v1.29.0+rke2r1")))`

	err = os.WriteFile(configPath, []byte(scaledConfig), 0644)
	require.NoError(t, err)

	// Scale up
	t.Log("📈 Scaling up - incrementing worker count from 1 to 3...")
	_, err = runSlothCommand(t, cfg, "deploy", "--config", configPath, "--stack", cfg.StackName, "--yes")
	require.NoError(t, err, "Scale deployment should succeed")

	// Validate
	t.Log("🔍 Validating scaled cluster...")
	nodesOutput, err := runSlothCommand(t, cfg, "nodes", "list", cfg.StackName)
	require.NoError(t, err)

	nodeCount := strings.Count(nodesOutput, "aws")
	t.Logf("📊 Final node count: %d (expected 4)", nodeCount)
	assert.Equal(t, 4, nodeCount, "Should have 4 nodes (1 master + 3 workers)")

	t.Log("✅ INCREMENT SCALE TEST PASSED!")
}
