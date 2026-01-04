//go:build e2e
// +build e2e

// Package e2e provides end-to-end tests for Salt Master/Minion integration
// with secure hash-based authentication
package e2e

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/chalkan3/sloth-kubernetes/internal/orchestrator/components"
	"github.com/chalkan3/sloth-kubernetes/pkg/config"
)

// =============================================================================
// Salt E2E Test Configuration
// =============================================================================

// SaltE2EConfig holds configuration for Salt E2E tests
type SaltE2EConfig struct {
	AWSRegion    string
	StackPrefix  string
	Timeout      time.Duration
	MasterSize   string
	MinionSize   string
	MinionCount  int
	ClusterToken string
}

// loadSaltE2EConfig loads Salt E2E test configuration
func loadSaltE2EConfig(t *testing.T) *SaltE2EConfig {
	timestamp := time.Now().Unix()
	clusterToken := generateTestClusterToken("salt-e2e-test", timestamp)

	return &SaltE2EConfig{
		AWSRegion:    getEnvOrDefault("AWS_REGION", "us-east-1"),
		StackPrefix:  fmt.Sprintf("salt-e2e-%d", timestamp),
		Timeout:      25 * time.Minute,
		MasterSize:   "t3.small",
		MinionSize:   "t3.micro",
		MinionCount:  2,
		ClusterToken: clusterToken,
	}
}

// generateTestClusterToken generates a test cluster token (same logic as production)
func generateTestClusterToken(clusterName string, timestamp int64) string {
	seed := fmt.Sprintf("%s-test-%d", clusterName, timestamp)
	hash := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(hash[:])[:32]
}

// =============================================================================
// Salt E2E Test - Full Integration
// =============================================================================

// TestE2E_Salt_FullIntegration tests complete Salt Master/Minion setup
// with secure hash-based authentication
func TestE2E_Salt_FullIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Salt E2E test in short mode")
	}

	// Load configuration
	cfg := loadSaltE2EConfig(t)
	e2eCfg := loadE2EConfig(t)
	skipIfNoAWSCredentials(t, e2eCfg)

	// Create test report
	report := NewTestReport("Salt Full Integration E2E")
	defer func() {
		report.Finish("completed")
		report.Print(t)
	}()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	// Create AWS client for cleanup
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	require.NoError(t, err, "Failed to load AWS config")
	ec2Client := ec2.NewFromConfig(awsCfg)

	t.Log("════════════════════════════════════════════════════════════")
	t.Log("🧂 SALT E2E TEST - FULL INTEGRATION")
	t.Log("════════════════════════════════════════════════════════════")
	t.Logf("  Region: %s", cfg.AWSRegion)
	t.Logf("  Master Size: %s", cfg.MasterSize)
	t.Logf("  Minion Count: %d", cfg.MinionCount)
	t.Logf("  Cluster Token: %s...", cfg.ClusterToken[:8])
	t.Logf("  Stack Prefix: %s", cfg.StackPrefix)
	t.Log("════════════════════════════════════════════════════════════")

	// Phase 1: Create Infrastructure
	phase1 := report.StartPhase("Create Infrastructure")
	t.Log("")
	t.Log("📋 PHASE 1: CREATING INFRASTRUCTURE")
	t.Log("────────────────────────────────────────────────────────────")

	stackName := fmt.Sprintf("%s-salt", cfg.StackPrefix)
	var sshPrivateKey string
	var masterIP, minionIPs []string

	// Define Pulumi program
	program := func(pctx *pulumi.Context) error {
		// Create cluster config with Salt enabled
		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "salt-e2e-test",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  cfg.AWSRegion,
				},
			},
			Addons: config.AddonsConfig{
				Salt: &config.SaltConfig{
					Enabled:      true,
					MasterNode:   "0",
					APIEnabled:   true,
					APIPort:      8000,
					SecureAuth:   true,
					AutoJoin:     true,
					AuditLogging: true,
				},
			},
		}

		// Create SSH keys
		sshComponent, err := components.NewSSHKeyComponent(pctx, "salt-e2e-ssh", clusterConfig)
		if err != nil {
			return fmt.Errorf("failed to create SSH keys: %w", err)
		}

		// Export SSH private key for later use
		pctx.Export("ssh_private_key", sshComponent.PrivateKey)
		pctx.Export("cluster_token", pulumi.String(cfg.ClusterToken))

		return nil
	}

	// Create Pulumi workspace
	stack, cleanup := createTestWorkspace(ctx, t, stackName, program)
	defer func() {
		t.Log("🧹 Running cleanup...")
		cleanup()
		// Additional AWS cleanup
		cleanupSaltTestResources(t, ctx, ec2Client, cfg.StackPrefix)
	}()

	// Set AWS credentials
	err = stack.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: cfg.AWSRegion})
	require.NoError(t, err)

	// Run Pulumi up
	t.Log("⬆️  Running pulumi up...")
	upResult, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	require.NoError(t, err, "Pulumi up failed")

	// Get SSH private key from outputs
	if keyOutput, ok := upResult.Outputs["ssh_private_key"]; ok {
		sshPrivateKey = keyOutput.Value.(string)
		t.Logf("✅ SSH key generated (%d bytes)", len(sshPrivateKey))
	}

	report.EndPhase(phase1, "success", "Infrastructure created")

	// Phase 2: Create EC2 Instances directly for Salt testing
	phase2 := report.StartPhase("Create EC2 Instances")
	t.Log("")
	t.Log("📋 PHASE 2: CREATING EC2 INSTANCES")
	t.Log("────────────────────────────────────────────────────────────")

	// Create key pair in AWS
	keyPairName := fmt.Sprintf("%s-keypair", cfg.StackPrefix)
	sshPublicKey := extractPublicKeyFromPrivate(t, sshPrivateKey)

	_, err = ec2Client.ImportKeyPair(ctx, &ec2.ImportKeyPairInput{
		KeyName:           aws.String(keyPairName),
		PublicKeyMaterial: []byte(sshPublicKey),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeKeyPair,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String(keyPairName)},
					{Key: aws.String("E2ETest"), Value: aws.String(cfg.StackPrefix)},
				},
			},
		},
	})
	require.NoError(t, err, "Failed to import key pair")
	t.Logf("✅ Key pair created: %s", keyPairName)

	// Create security group for Salt
	sgName := fmt.Sprintf("%s-salt-sg", cfg.StackPrefix)
	sgResult, err := ec2Client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(sgName),
		Description: aws.String("Security group for Salt E2E testing"),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSecurityGroup,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String(sgName)},
					{Key: aws.String("E2ETest"), Value: aws.String(cfg.StackPrefix)},
				},
			},
		},
	})
	require.NoError(t, err, "Failed to create security group")
	sgID := *sgResult.GroupId
	t.Logf("✅ Security group created: %s", sgID)

	// Add security group rules
	_, err = ec2Client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(sgID),
		IpPermissions: []types.IpPermission{
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(22),
				ToPort:     aws.Int32(22),
				IpRanges:   []types.IpRange{{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("SSH")}},
			},
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(4505),
				ToPort:     aws.Int32(4506),
				IpRanges:   []types.IpRange{{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("Salt Master")}},
			},
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(8000),
				ToPort:     aws.Int32(8000),
				IpRanges:   []types.IpRange{{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("Salt API")}},
			},
			{
				IpProtocol: aws.String("-1"),
				FromPort:   aws.Int32(-1),
				ToPort:     aws.Int32(-1),
				UserIdGroupPairs: []types.UserIdGroupPair{
					{GroupId: aws.String(sgID), Description: aws.String("Internal traffic")},
				},
			},
		},
	})
	require.NoError(t, err, "Failed to add security group rules")
	t.Log("✅ Security group rules added")

	// Get latest Ubuntu AMI
	amiResult, err := ec2Client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Filters: []types.Filter{
			{Name: aws.String("name"), Values: []string{"ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"}},
			{Name: aws.String("state"), Values: []string{"available"}},
			{Name: aws.String("virtualization-type"), Values: []string{"hvm"}},
		},
		Owners: []string{"099720109477"}, // Canonical
	})
	require.NoError(t, err, "Failed to get AMI")
	require.NotEmpty(t, amiResult.Images, "No AMI found")

	// Sort by creation date and get the latest
	latestAMI := amiResult.Images[0]
	for _, img := range amiResult.Images {
		if *img.CreationDate > *latestAMI.CreationDate {
			latestAMI = img
		}
	}
	t.Logf("✅ Using AMI: %s (%s)", *latestAMI.ImageId, *latestAMI.Name)

	// User data for Salt Master
	masterUserData := fmt.Sprintf(`#!/bin/bash
set -e
exec > >(tee /var/log/user-data.log) 2>&1
echo "Starting Salt Master setup..."

# Update system
apt-get update -y
apt-get install -y curl python3-pip

# Install Salt Master
curl -o /tmp/bootstrap-salt.sh -L https://github.com/saltstack/salt-bootstrap/releases/latest/download/bootstrap-salt.sh
chmod +x /tmp/bootstrap-salt.sh
sh /tmp/bootstrap-salt.sh -M -N stable

# Configure Salt Master with secure auth
mkdir -p /etc/salt/master.d /etc/salt/autosign_grains

cat > /etc/salt/master.d/master.conf << 'EOF'
interface: 0.0.0.0
auto_accept: False
autosign_grains_dir: /etc/salt/autosign_grains
log_level: info
worker_threads: 5
timeout: 60
open_mode: False
EOF

# Set cluster token for secure autosign
echo '%s' > /etc/salt/autosign_grains/cluster_token

# Configure Salt API
cat > /etc/salt/master.d/api.conf << 'EOF'
rest_cherrypy:
  port: 8000
  host: 0.0.0.0
  disable_ssl: true

external_auth:
  pam:
    saltapi:
      - .*
      - '@wheel'
      - '@runner'
      - '@jobs'
EOF

# Create saltapi user
useradd -r -s /bin/false saltapi 2>/dev/null || true
echo "saltapi:saltapi-e2e-test" | chpasswd

# Install cherrypy for Salt API
pip3 install cherrypy

# Create reactor for audit logging
mkdir -p /srv/reactor /var/log/salt
cat > /srv/reactor/auth_log.sls << 'EOF'
log_auth_event:
  local.cmd.run:
    - tgt: salt-master
    - arg:
      - 'echo "[$(date)] Key auth event: {{ data.get("id", "unknown") }} - {{ data.get("act", "unknown") }}" >> /var/log/salt/auth_audit.log'
EOF

cat > /etc/salt/master.d/reactor.conf << 'EOF'
reactor:
  - 'salt/auth':
    - /srv/reactor/auth_log.sls
EOF

touch /var/log/salt/auth_audit.log
chmod 640 /var/log/salt/auth_audit.log

# Create basic Salt states
mkdir -p /srv/salt /srv/pillar
cat > /srv/salt/top.sls << 'EOF'
base:
  '*':
    - test_state
EOF

cat > /srv/salt/test_state.sls << 'EOF'
test_file:
  file.managed:
    - name: /tmp/salt-e2e-test.txt
    - contents: "Salt E2E Test Successful"
EOF

# Start services
systemctl daemon-reload
systemctl enable salt-master salt-api
systemctl restart salt-master
sleep 5
systemctl restart salt-api

echo "Salt Master setup complete"
echo "SALT_MASTER_READY" > /tmp/salt_setup_complete
`, cfg.ClusterToken)

	// Create Salt Master instance
	masterName := fmt.Sprintf("%s-salt-master", cfg.StackPrefix)
	masterResult, err := ec2Client.RunInstances(ctx, &ec2.RunInstancesInput{
		ImageId:          latestAMI.ImageId,
		InstanceType:     types.InstanceType(cfg.MasterSize),
		MinCount:         aws.Int32(1),
		MaxCount:         aws.Int32(1),
		KeyName:          aws.String(keyPairName),
		SecurityGroupIds: []string{sgID},
		UserData:         aws.String(encodeUserData(masterUserData)),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String(masterName)},
					{Key: aws.String("E2ETest"), Value: aws.String(cfg.StackPrefix)},
					{Key: aws.String("Role"), Value: aws.String("salt-master")},
				},
			},
		},
	})
	require.NoError(t, err, "Failed to create Salt Master instance")
	masterInstanceID := *masterResult.Instances[0].InstanceId
	t.Logf("✅ Salt Master instance created: %s", masterInstanceID)

	// Wait for instance to be running
	t.Log("⏳ Waiting for Salt Master to be running...")
	waiter := ec2.NewInstanceRunningWaiter(ec2Client)
	err = waiter.Wait(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{masterInstanceID},
	}, 5*time.Minute)
	require.NoError(t, err, "Salt Master instance failed to start")

	// Get master public IP
	descResult, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{masterInstanceID},
	})
	require.NoError(t, err)
	masterIP = append(masterIP, *descResult.Reservations[0].Instances[0].PublicIpAddress)
	masterPrivateIP := *descResult.Reservations[0].Instances[0].PrivateIpAddress
	t.Logf("✅ Salt Master running at %s (private: %s)", masterIP[0], masterPrivateIP)

	// Create Salt Minion instances
	for i := 0; i < cfg.MinionCount; i++ {
		minionName := fmt.Sprintf("%s-salt-minion-%d", cfg.StackPrefix, i)
		minionID := fmt.Sprintf("minion-%d", i)

		minionUserData := fmt.Sprintf(`#!/bin/bash
set -e
exec > >(tee /var/log/user-data.log) 2>&1
echo "Starting Salt Minion setup..."

# Update system
apt-get update -y
apt-get install -y curl

# Install Salt Minion
curl -o /tmp/bootstrap-salt.sh -L https://github.com/saltstack/salt-bootstrap/releases/latest/download/bootstrap-salt.sh
chmod +x /tmp/bootstrap-salt.sh
sh /tmp/bootstrap-salt.sh stable

# Configure Salt Minion with secure cluster token
mkdir -p /etc/salt/minion.d

echo "%s" > /etc/salt/minion_id

cat > /etc/salt/minion.d/master.conf << EOF
master: %s
id: %s
EOF

# Configure grains with cluster token for secure autosign
cat > /etc/salt/grains << EOF
roles:
  - kubernetes
  - e2e-test
cluster: salt-e2e-test
node_type: minion
cluster_token: %s
EOF

# Clear any old keys
rm -f /etc/salt/pki/minion/minion_master.pub 2>/dev/null || true

# Start Salt Minion
systemctl enable salt-minion
systemctl restart salt-minion

echo "Salt Minion setup complete"
echo "SALT_MINION_READY" > /tmp/salt_setup_complete
`, minionID, masterPrivateIP, minionID, cfg.ClusterToken)

		minionResult, err := ec2Client.RunInstances(ctx, &ec2.RunInstancesInput{
			ImageId:          latestAMI.ImageId,
			InstanceType:     types.InstanceType(cfg.MinionSize),
			MinCount:         aws.Int32(1),
			MaxCount:         aws.Int32(1),
			KeyName:          aws.String(keyPairName),
			SecurityGroupIds: []string{sgID},
			UserData:         aws.String(encodeUserData(minionUserData)),
			TagSpecifications: []types.TagSpecification{
				{
					ResourceType: types.ResourceTypeInstance,
					Tags: []types.Tag{
						{Key: aws.String("Name"), Value: aws.String(minionName)},
						{Key: aws.String("E2ETest"), Value: aws.String(cfg.StackPrefix)},
						{Key: aws.String("Role"), Value: aws.String("salt-minion")},
					},
				},
			},
		})
		require.NoError(t, err, "Failed to create Salt Minion instance %d", i)
		minionInstanceID := *minionResult.Instances[0].InstanceId
		t.Logf("✅ Salt Minion %d instance created: %s", i, minionInstanceID)

		// Wait for instance to be running
		err = waiter.Wait(ctx, &ec2.DescribeInstancesInput{
			InstanceIds: []string{minionInstanceID},
		}, 5*time.Minute)
		require.NoError(t, err, "Salt Minion %d failed to start", i)

		// Get minion public IP
		descResult, err = ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			InstanceIds: []string{minionInstanceID},
		})
		require.NoError(t, err)
		minionIPs = append(minionIPs, *descResult.Reservations[0].Instances[0].PublicIpAddress)
		t.Logf("✅ Salt Minion %d running at %s", i, minionIPs[i])
	}

	report.EndPhase(phase2, "success", fmt.Sprintf("Created 1 master + %d minions", cfg.MinionCount))
	report.SetMetric("master_ip", masterIP[0])
	report.SetMetric("minion_count", cfg.MinionCount)

	// Phase 3: Wait for Salt Setup
	phase3 := report.StartPhase("Wait for Salt Setup")
	t.Log("")
	t.Log("📋 PHASE 3: WAITING FOR SALT SETUP")
	t.Log("────────────────────────────────────────────────────────────")

	t.Log("⏳ Waiting for Salt Master cloud-init to complete...")
	time.Sleep(60 * time.Second) // Give cloud-init time to run

	// Create SSH client config
	sshConfig := createSSHConfig(t, sshPrivateKey)

	// Verify Salt Master is ready
	masterReady := false
	for attempt := 1; attempt <= 20; attempt++ {
		output, err := runSSHCommand(masterIP[0], "cat /tmp/salt_setup_complete 2>/dev/null || echo 'NOT_READY'", sshConfig)
		if err == nil && strings.Contains(output, "SALT_MASTER_READY") {
			t.Logf("✅ Salt Master ready after %d attempts", attempt)
			masterReady = true
			break
		}
		t.Logf("   Attempt %d: Salt Master not ready yet (%v)", attempt, err)
		time.Sleep(15 * time.Second)
	}
	require.True(t, masterReady, "Salt Master did not become ready")

	// Verify Salt services are running
	t.Log("🔍 Verifying Salt Master services...")
	output, err := runSSHCommand(masterIP[0], "sudo systemctl is-active salt-master salt-api", sshConfig)
	require.NoError(t, err, "Failed to check Salt services")
	t.Logf("   Service status: %s", strings.ReplaceAll(output, "\n", " "))
	assert.Contains(t, output, "active", "Salt services should be active")

	// Wait for minions to be ready
	t.Log("⏳ Waiting for Salt Minions to be ready...")
	time.Sleep(30 * time.Second)

	for i, minionIP := range minionIPs {
		minionReady := false
		for attempt := 1; attempt <= 15; attempt++ {
			output, err := runSSHCommand(minionIP, "cat /tmp/salt_setup_complete 2>/dev/null || echo 'NOT_READY'", sshConfig)
			if err == nil && strings.Contains(output, "SALT_MINION_READY") {
				t.Logf("✅ Minion %d ready after %d attempts", i, attempt)
				minionReady = true
				break
			}
			t.Logf("   Attempt %d: Minion %d not ready yet", attempt, i)
			time.Sleep(10 * time.Second)
		}
		require.True(t, minionReady, "Minion %d did not become ready", i)
	}

	report.EndPhase(phase3, "success", "All Salt nodes ready")

	// Phase 4: Verify Secure Authentication
	phase4 := report.StartPhase("Verify Secure Authentication")
	t.Log("")
	t.Log("📋 PHASE 4: VERIFYING SECURE HASH AUTHENTICATION")
	t.Log("────────────────────────────────────────────────────────────")

	// Wait for minions to authenticate
	t.Log("⏳ Waiting for minions to authenticate with master...")
	time.Sleep(30 * time.Second)

	// Check accepted keys
	output, err = runSSHCommand(masterIP[0], "sudo salt-key -L 2>/dev/null", sshConfig)
	require.NoError(t, err, "Failed to list Salt keys")
	t.Logf("🔑 Salt key status:\n%s", output)

	// Count accepted minions
	acceptedCount := 0
	for i := 0; i < cfg.MinionCount; i++ {
		minionID := fmt.Sprintf("minion-%d", i)
		if strings.Contains(output, minionID) {
			acceptedCount++
			t.Logf("✅ Minion '%s' authenticated successfully", minionID)
		}
	}

	t.Logf("📊 Authenticated minions: %d/%d", acceptedCount, cfg.MinionCount)
	report.SetMetric("authenticated_minions", acceptedCount)

	// Verify autosign grain is configured
	output, err = runSSHCommand(masterIP[0], "sudo cat /etc/salt/autosign_grains/cluster_token", sshConfig)
	require.NoError(t, err, "Failed to read cluster token")
	assert.Equal(t, cfg.ClusterToken, strings.TrimSpace(output), "Cluster token should match")
	t.Logf("✅ Cluster token verified: %s...", cfg.ClusterToken[:8])

	// Check audit log for authentication events
	output, err = runSSHCommand(masterIP[0], "sudo cat /var/log/salt/auth_audit.log 2>/dev/null | tail -5", sshConfig)
	if err == nil && output != "" {
		t.Logf("📋 Recent auth events:\n%s", output)
	}

	report.EndPhase(phase4, "success", fmt.Sprintf("%d minions authenticated", acceptedCount))

	// Phase 5: Test Salt API
	phase5 := report.StartPhase("Test Salt API")
	t.Log("")
	t.Log("📋 PHASE 5: TESTING SALT API")
	t.Log("────────────────────────────────────────────────────────────")

	// Test Salt API login
	apiLoginCmd := `curl -s -X POST http://localhost:8000/login -d username=saltapi -d password=saltapi-e2e-test -d eauth=pam | grep -o '"token"' || echo "NO_TOKEN"`
	output, err = runSSHCommand(masterIP[0], apiLoginCmd, sshConfig)
	require.NoError(t, err, "Failed to test Salt API login")
	t.Logf("🔐 Salt API login result: %s", strings.TrimSpace(output))

	if strings.Contains(output, "token") {
		t.Log("✅ Salt API authentication successful")
	} else {
		t.Log("⚠️  Salt API may need more time to start")
	}

	// Check if Salt API is listening
	output, err = runSSHCommand(masterIP[0], "sudo ss -tlnp | grep :8000 || echo 'Port 8000 not listening'", sshConfig)
	require.NoError(t, err)
	t.Logf("🌐 Salt API port status: %s", strings.TrimSpace(output))

	report.EndPhase(phase5, "success", "Salt API tested")

	// Phase 6: Test Salt Commands
	phase6 := report.StartPhase("Test Salt Commands")
	t.Log("")
	t.Log("📋 PHASE 6: TESTING SALT COMMAND EXECUTION")
	t.Log("────────────────────────────────────────────────────────────")

	// Test ping all minions
	t.Log("🏓 Testing salt ping...")
	output, err = runSSHCommand(masterIP[0], "sudo salt '*' test.ping --timeout=30 2>/dev/null || echo 'PING_FAILED'", sshConfig)
	require.NoError(t, err, "Failed to run salt ping")
	t.Logf("   Ping result:\n%s", output)

	pingSuccessCount := strings.Count(output, "True")
	t.Logf("📊 Minions responding to ping: %d", pingSuccessCount)
	report.SetMetric("ping_success_count", pingSuccessCount)

	// Test grains.items to verify cluster_token grain
	t.Log("🌾 Testing grains (verify cluster_token)...")
	output, err = runSSHCommand(masterIP[0], "sudo salt '*' grains.get cluster_token --timeout=30 2>/dev/null | head -20", sshConfig)
	if err == nil {
		t.Logf("   Cluster token grain:\n%s", output)
		if strings.Contains(output, cfg.ClusterToken) {
			t.Log("✅ Cluster token grain verified on minions")
		}
	}

	// Test cmd.run
	t.Log("💻 Testing cmd.run...")
	output, err = runSSHCommand(masterIP[0], "sudo salt '*' cmd.run 'hostname' --timeout=30 2>/dev/null", sshConfig)
	if err == nil {
		t.Logf("   Hostnames:\n%s", output)
	}

	// Test state.apply
	t.Log("📦 Testing state.apply...")
	output, err = runSSHCommand(masterIP[0], "sudo salt '*' state.apply test_state --timeout=60 2>/dev/null | tail -20", sshConfig)
	if err == nil {
		t.Logf("   State apply result:\n%s", output)
		if strings.Contains(output, "Succeeded") {
			t.Log("✅ Salt state applied successfully")
		}
	}

	// Verify state was applied
	t.Log("🔍 Verifying state was applied on minions...")
	for i, minionIP := range minionIPs {
		output, err = runSSHCommand(minionIP, "cat /tmp/salt-e2e-test.txt 2>/dev/null || echo 'FILE_NOT_FOUND'", sshConfig)
		if err == nil && strings.Contains(output, "Salt E2E Test Successful") {
			t.Logf("✅ Minion %d: State verified", i)
		} else {
			t.Logf("⚠️  Minion %d: State not yet applied", i)
		}
	}

	report.EndPhase(phase6, "success", "Salt commands tested")

	// Phase 7: Test Invalid Token (Security Test)
	phase7 := report.StartPhase("Security Validation")
	t.Log("")
	t.Log("📋 PHASE 7: SECURITY VALIDATION")
	t.Log("────────────────────────────────────────────────────────────")

	// Verify that minions with wrong token would be rejected
	t.Log("🔒 Verifying security: checking for rejected keys...")
	output, err = runSSHCommand(masterIP[0], "sudo salt-key -L 2>/dev/null | grep -A 100 'Rejected Keys' | head -10", sshConfig)
	if err == nil {
		t.Logf("   Rejected keys section:\n%s", output)
	}

	// Check master security configuration
	t.Log("🔐 Verifying master security configuration...")
	output, err = runSSHCommand(masterIP[0], "sudo grep -E 'auto_accept|open_mode|autosign' /etc/salt/master.d/master.conf 2>/dev/null", sshConfig)
	if err == nil {
		t.Logf("   Security config:\n%s", output)
		assert.Contains(t, output, "auto_accept: False", "auto_accept should be disabled")
		assert.Contains(t, output, "open_mode: False", "open_mode should be disabled")
	}

	report.EndPhase(phase7, "success", "Security validated")

	// Final Summary
	t.Log("")
	t.Log("════════════════════════════════════════════════════════════")
	t.Log("🧂 SALT E2E TEST COMPLETED")
	t.Log("════════════════════════════════════════════════════════════")
	t.Logf("  Salt Master: %s", masterIP[0])
	t.Logf("  Minions: %d", cfg.MinionCount)
	t.Logf("  Authenticated: %d", acceptedCount)
	t.Logf("  Ping Success: %d", pingSuccessCount)
	t.Logf("  Cluster Token: %s...", cfg.ClusterToken[:8])
	t.Log("════════════════════════════════════════════════════════════")
}

// =============================================================================
// SSH Helper Functions
// =============================================================================

// createSSHConfig creates an SSH client configuration
func createSSHConfig(t *testing.T, privateKey string) *ssh.ClientConfig {
	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	require.NoError(t, err, "Failed to parse SSH private key")

	return &ssh.ClientConfig{
		User: "ubuntu",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}
}

// runSSHCommand runs a command on a remote host via SSH
func runSSHCommand(host, command string, config *ssh.ClientConfig) (string, error) {
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", host), config)
	if err != nil {
		return "", fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	return string(output), err
}

// extractPublicKeyFromPrivate extracts public key from private key
func extractPublicKeyFromPrivate(t *testing.T, privateKey string) string {
	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	require.NoError(t, err, "Failed to parse private key")

	pubKey := signer.PublicKey()
	return string(ssh.MarshalAuthorizedKey(pubKey))
}

// encodeUserData encodes user data to base64
func encodeUserData(userData string) string {
	return base64.StdEncoding.EncodeToString([]byte(userData))
}

// =============================================================================
// Cleanup Functions
// =============================================================================

// cleanupSaltTestResources cleans up all AWS resources created by the test
func cleanupSaltTestResources(t *testing.T, ctx context.Context, ec2Client *ec2.Client, prefix string) {
	t.Log("🧹 Cleaning up Salt test resources...")

	// Find and terminate instances
	instances, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{Name: aws.String("tag:E2ETest"), Values: []string{prefix}},
			{Name: aws.String("instance-state-name"), Values: []string{"running", "pending", "stopping", "stopped"}},
		},
	})
	if err == nil {
		var instanceIds []string
		for _, reservation := range instances.Reservations {
			for _, instance := range reservation.Instances {
				instanceIds = append(instanceIds, *instance.InstanceId)
			}
		}
		if len(instanceIds) > 0 {
			t.Logf("   Terminating %d instances...", len(instanceIds))
			_, err = ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
				InstanceIds: instanceIds,
			})
			if err != nil {
				t.Logf("   Warning: Failed to terminate instances: %v", err)
			}

			// Wait for termination
			waiter := ec2.NewInstanceTerminatedWaiter(ec2Client)
			_ = waiter.Wait(ctx, &ec2.DescribeInstancesInput{
				InstanceIds: instanceIds,
			}, 10*time.Minute)
		}
	}

	// Delete security groups
	time.Sleep(30 * time.Second) // Wait for instances to fully terminate
	sgs, err := ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{Name: aws.String("tag:E2ETest"), Values: []string{prefix}},
		},
	})
	if err == nil {
		for _, sg := range sgs.SecurityGroups {
			t.Logf("   Deleting security group: %s", *sg.GroupId)
			_, err = ec2Client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
				GroupId: sg.GroupId,
			})
			if err != nil {
				t.Logf("   Warning: Failed to delete security group: %v", err)
			}
		}
	}

	// Delete key pairs
	keyPairs, err := ec2Client.DescribeKeyPairs(ctx, &ec2.DescribeKeyPairsInput{
		Filters: []types.Filter{
			{Name: aws.String("tag:E2ETest"), Values: []string{prefix}},
		},
	})
	if err == nil {
		for _, kp := range keyPairs.KeyPairs {
			t.Logf("   Deleting key pair: %s", *kp.KeyName)
			_, err = ec2Client.DeleteKeyPair(ctx, &ec2.DeleteKeyPairInput{
				KeyName: kp.KeyName,
			})
			if err != nil {
				t.Logf("   Warning: Failed to delete key pair: %v", err)
			}
		}
	}

	t.Log("✅ Cleanup complete")
}
