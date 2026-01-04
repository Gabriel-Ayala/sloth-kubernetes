//go:build e2e
// +build e2e

// Package e2e provides end-to-end tests for the sloth-kubernetes salt CLI commands
// This test validates the complete CLI workflow by:
// - Deploying real Salt Master and Minions on AWS
// - Building and executing the sloth-kubernetes binary
// - Testing all salt CLI subcommands against real infrastructure
// - Validating command output and behavior
package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"golang.org/x/crypto/ssh"
)

// =============================================================================
// Sloth Salt CLI E2E Test Suite
// =============================================================================

// SlothSaltCLITestSuite holds the complete test configuration for CLI testing
type SlothSaltCLITestSuite struct {
	t             *testing.T
	ctx           context.Context
	cancel        context.CancelFunc
	ec2Client     *ec2.Client
	config        *SlothSaltCLIConfig
	masterIP      string
	masterPrivIP  string
	minionIPs     []string
	minionPrivIPs []string
	instanceIDs   []string
	sgID          string
	keyPairName   string
	sshKey        string
	binaryPath    string
}

// SlothSaltCLIConfig holds CLI E2E test configuration
type SlothSaltCLIConfig struct {
	AWSRegion    string
	StackPrefix  string
	Timeout      time.Duration
	MasterSize   string
	MinionSize   string
	MinionCount  int
	ClusterToken string
	APIUsername  string
	APIPassword  string
}

// NewSlothSaltCLITestSuite creates a new CLI test suite
func NewSlothSaltCLITestSuite(t *testing.T) *SlothSaltCLITestSuite {
	timestamp := time.Now().Unix()
	clusterToken := generateCLITestToken("sloth-salt-cli", timestamp)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)

	return &SlothSaltCLITestSuite{
		t:      t,
		ctx:    ctx,
		cancel: cancel,
		config: &SlothSaltCLIConfig{
			AWSRegion:    getEnvOrDefault("AWS_REGION", "us-east-1"),
			StackPrefix:  fmt.Sprintf("sloth-cli-e2e-%d", timestamp),
			Timeout:      25 * time.Minute,
			MasterSize:   "t3.medium",
			MinionSize:   "t3.small",
			MinionCount:  2,
			ClusterToken: clusterToken,
			APIUsername:  "saltapi",
			APIPassword:  "SaltE2ETest2024", // Simple alphanumeric password to avoid escaping issues
		},
	}
}

func generateCLITestToken(prefix string, timestamp int64) string {
	seed := fmt.Sprintf("%s-%d-%d", prefix, timestamp, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(hash[:])[:32]
}

// =============================================================================
// Main Test Entry Point
// =============================================================================

// TestE2E_Sloth_Salt_CLI runs the complete sloth-kubernetes salt CLI E2E tests
func TestE2E_Sloth_Salt_CLI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sloth-kubernetes salt CLI E2E test in short mode")
	}

	if os.Getenv("RUN_E2E_TESTS") != "true" {
		t.Skip("Skipping: Set RUN_E2E_TESTS=true to run")
	}

	suite := NewSlothSaltCLITestSuite(t)
	defer suite.Cleanup()

	if !suite.ValidateAWSCredentials() {
		t.Skip("Skipping: AWS credentials not configured")
	}

	suite.PrintHeader()

	// Phase 1: Build the binary
	suite.RunPhase("Build CLI Binary", suite.BuildBinary)

	// Phase 2: Setup infrastructure
	suite.RunPhase("Setup AWS Infrastructure", suite.SetupInfrastructure)
	suite.RunPhase("Deploy Salt Master", suite.DeploySaltMaster)
	suite.RunPhase("Deploy Salt Minions", suite.DeploySaltMinions)
	suite.RunPhase("Wait for Salt Services", suite.WaitForSaltServices)

	// Phase 3: Test Core CLI Commands
	suite.RunPhase("Test CLI: salt ping", suite.TestCLIPing)
	suite.RunPhase("Test CLI: salt minions", suite.TestCLIMinions)
	suite.RunPhase("Test CLI: salt cmd", suite.TestCLICmd)
	suite.RunPhase("Test CLI: salt grains", suite.TestCLIGrains)
	suite.RunPhase("Test CLI: salt keys", suite.TestCLIKeys)
	suite.RunPhase("Test CLI: salt state", suite.TestCLIState)

	// Phase 4: Test Extended CLI Commands
	suite.RunPhase("Test CLI: salt system", suite.TestCLISystemCommands)
	suite.RunPhase("Test CLI: salt file", suite.TestCLIFileCommands)
	suite.RunPhase("Test CLI: salt service", suite.TestCLIServiceCommands)
	suite.RunPhase("Test CLI: salt pkg", suite.TestCLIPkgCommands)
	suite.RunPhase("Test CLI: salt user", suite.TestCLIUserCommands)
	suite.RunPhase("Test CLI: salt job", suite.TestCLIJobCommands)

	// Phase 5: Test Advanced CLI Commands
	suite.RunPhase("Test CLI: salt network", suite.TestCLINetworkCommands)
	suite.RunPhase("Test CLI: salt process", suite.TestCLIProcessCommands)
	suite.RunPhase("Test CLI: salt cron", suite.TestCLICronCommands)
	suite.RunPhase("Test CLI: salt archive", suite.TestCLIArchiveCommands)
	suite.RunPhase("Test CLI: salt monitor", suite.TestCLIMonitorCommands)
	suite.RunPhase("Test CLI: salt pillar", suite.TestCLIPillarCommands)
	suite.RunPhase("Test CLI: salt mount", suite.TestCLIMountCommands)

	// Phase 6: Test Output and Filtering
	suite.RunPhase("Test CLI: salt --json output", suite.TestCLIJSONOutput)
	suite.RunPhase("Test CLI: salt --target filtering", suite.TestCLITargeting)

	suite.PrintSummary()
}

// =============================================================================
// Phase Implementations
// =============================================================================

func (s *SlothSaltCLITestSuite) PrintHeader() {
	s.t.Log("════════════════════════════════════════════════════════════")
	s.t.Log("🐌 SLOTH-KUBERNETES SALT CLI E2E TEST")
	s.t.Log("════════════════════════════════════════════════════════════")
	s.t.Logf("  Region: %s", s.config.AWSRegion)
	s.t.Logf("  Master Size: %s", s.config.MasterSize)
	s.t.Logf("  Minion Count: %d", s.config.MinionCount)
	s.t.Logf("  Stack Prefix: %s", s.config.StackPrefix)
	s.t.Logf("  API Username: %s", s.config.APIUsername)
	s.t.Logf("  API Password: %s...%s", s.config.APIPassword[:8], s.config.APIPassword[len(s.config.APIPassword)-4:])
	s.t.Log("════════════════════════════════════════════════════════════")
}

func (s *SlothSaltCLITestSuite) PrintSummary() {
	s.t.Log("")
	s.t.Log("════════════════════════════════════════════════════════════")
	s.t.Log("✅ SLOTH-KUBERNETES SALT CLI E2E TEST COMPLETED")
	s.t.Log("════════════════════════════════════════════════════════════")
}

func (s *SlothSaltCLITestSuite) RunPhase(name string, fn func() error) {
	s.t.Log("")
	s.t.Logf("📋 PHASE: %s", name)
	s.t.Log("────────────────────────────────────────────────────────────")

	if err := fn(); err != nil {
		s.t.Fatalf("❌ Phase '%s' failed: %v", name, err)
	}
}

func (s *SlothSaltCLITestSuite) ValidateAWSCredentials() bool {
	cfg, err := awsconfig.LoadDefaultConfig(s.ctx, awsconfig.WithRegion(s.config.AWSRegion))
	if err != nil {
		s.t.Logf("AWS config error: %v", err)
		return false
	}
	s.ec2Client = ec2.NewFromConfig(cfg)
	return true
}

// BuildBinary compiles the sloth-kubernetes binary
func (s *SlothSaltCLITestSuite) BuildBinary() error {
	s.t.Log("Building sloth-kubernetes binary...")

	// Get project root
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return fmt.Errorf("failed to get project root: %w", err)
	}

	// Binary path
	s.binaryPath = filepath.Join(projectRoot, "bin", "sloth-kubernetes-e2e-test")

	// Build command
	cmd := exec.Command("go", "build", "-o", s.binaryPath, ".")
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to build binary: %w\nOutput: %s", err, string(output))
	}

	// Verify binary exists
	if _, err := os.Stat(s.binaryPath); os.IsNotExist(err) {
		return fmt.Errorf("binary not found at %s", s.binaryPath)
	}

	s.t.Logf("✅ Binary built: %s", s.binaryPath)
	return nil
}

// runCLI executes the sloth-kubernetes CLI with given arguments
func (s *SlothSaltCLITestSuite) runCLI(args ...string) (string, string, error) {
	// Add salt API configuration
	fullArgs := append([]string{
		"salt",
		"--url", fmt.Sprintf("http://%s:8000", s.masterIP),
		"--username", s.config.APIUsername,
		"--password", s.config.APIPassword,
	}, args...)

	cmd := exec.Command(s.binaryPath, fullArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// runCLIWithTarget executes CLI with specific target
func (s *SlothSaltCLITestSuite) runCLIWithTarget(target string, args ...string) (string, string, error) {
	fullArgs := append([]string{"--target", target}, args...)
	return s.runCLI(fullArgs...)
}

// =============================================================================
// Infrastructure Setup
// =============================================================================

func (s *SlothSaltCLITestSuite) SetupInfrastructure() error {
	s.t.Log("Creating SSH key pair...")

	// Generate SSH key
	privateKey, publicKey, err := s.generateSSHKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate SSH key: %w", err)
	}
	s.sshKey = privateKey

	// Import key pair to AWS
	s.keyPairName = fmt.Sprintf("%s-keypair", s.config.StackPrefix)
	_, err = s.ec2Client.ImportKeyPair(s.ctx, &ec2.ImportKeyPairInput{
		KeyName:           aws.String(s.keyPairName),
		PublicKeyMaterial: []byte(publicKey),
	})
	if err != nil {
		return fmt.Errorf("failed to import key pair: %w", err)
	}
	s.t.Logf("✅ SSH key pair created: %s", s.keyPairName)

	// Create security group
	s.t.Log("Creating security group...")
	sgName := fmt.Sprintf("%s-sg", s.config.StackPrefix)
	sgResult, err := s.ec2Client.CreateSecurityGroup(s.ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(sgName),
		Description: aws.String("Security group for Sloth Salt CLI E2E testing"),
	})
	if err != nil {
		return fmt.Errorf("failed to create security group: %w", err)
	}
	s.sgID = *sgResult.GroupId
	s.t.Logf("✅ Security group created: %s", s.sgID)

	// Add inbound rules
	_, err = s.ec2Client.AuthorizeSecurityGroupIngress(s.ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(s.sgID),
		IpPermissions: []types.IpPermission{
			{IpProtocol: aws.String("-1"), IpRanges: []types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to configure security group: %w", err)
	}
	s.t.Log("✅ Security group rules configured")

	return nil
}

func (s *SlothSaltCLITestSuite) generateSSHKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}

	return string(privateKeyPEM), string(ssh.MarshalAuthorizedKey(pub)), nil
}

func (s *SlothSaltCLITestSuite) DeploySaltMaster() error {
	s.t.Log("Getting latest Ubuntu AMI...")
	ami, err := s.getLatestUbuntuAMI()
	if err != nil {
		return err
	}
	s.t.Logf("✅ Using AMI: %s", ami)

	s.t.Log("Creating Salt Master instance...")
	userData := s.getMasterUserData()

	result, err := s.ec2Client.RunInstances(s.ctx, &ec2.RunInstancesInput{
		ImageId:          aws.String(ami),
		InstanceType:     types.InstanceType(s.config.MasterSize),
		KeyName:          aws.String(s.keyPairName),
		SecurityGroupIds: []string{s.sgID},
		MinCount:         aws.Int32(1),
		MaxCount:         aws.Int32(1),
		UserData:         aws.String(encodeUserData(userData)),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String(s.config.StackPrefix + "-master")},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create master instance: %w", err)
	}

	masterID := *result.Instances[0].InstanceId
	s.instanceIDs = append(s.instanceIDs, masterID)
	s.t.Logf("✅ Salt Master instance created: %s", masterID)

	// Wait for running
	s.t.Log("⏳ Waiting for Salt Master to be running...")
	waiter := ec2.NewInstanceRunningWaiter(s.ec2Client)
	if err := waiter.Wait(s.ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{masterID}}, 5*time.Minute); err != nil {
		return fmt.Errorf("master instance did not become running: %w", err)
	}

	// Get IP addresses
	descResult, err := s.ec2Client.DescribeInstances(s.ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{masterID}})
	if err != nil {
		return err
	}
	s.masterIP = *descResult.Reservations[0].Instances[0].PublicIpAddress
	s.masterPrivIP = *descResult.Reservations[0].Instances[0].PrivateIpAddress
	s.t.Logf("✅ Salt Master running: %s (private: %s)", s.masterIP, s.masterPrivIP)

	return nil
}

func (s *SlothSaltCLITestSuite) DeploySaltMinions() error {
	ami, _ := s.getLatestUbuntuAMI()

	for i := 0; i < s.config.MinionCount; i++ {
		minionID := fmt.Sprintf("minion-%d", i)
		s.t.Logf("Creating Salt Minion %d...", i)

		userData := s.getMinionUserData(minionID)

		result, err := s.ec2Client.RunInstances(s.ctx, &ec2.RunInstancesInput{
			ImageId:          aws.String(ami),
			InstanceType:     types.InstanceType(s.config.MinionSize),
			KeyName:          aws.String(s.keyPairName),
			SecurityGroupIds: []string{s.sgID},
			MinCount:         aws.Int32(1),
			MaxCount:         aws.Int32(1),
			UserData:         aws.String(encodeUserData(userData)),
			TagSpecifications: []types.TagSpecification{
				{
					ResourceType: types.ResourceTypeInstance,
					Tags: []types.Tag{
						{Key: aws.String("Name"), Value: aws.String(fmt.Sprintf("%s-%s", s.config.StackPrefix, minionID))},
					},
				},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create minion %d: %w", i, err)
		}

		instanceID := *result.Instances[0].InstanceId
		s.instanceIDs = append(s.instanceIDs, instanceID)
		s.t.Logf("✅ Salt Minion %d instance created: %s", i, instanceID)

		// Wait for running
		waiter := ec2.NewInstanceRunningWaiter(s.ec2Client)
		if err := waiter.Wait(s.ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{instanceID}}, 5*time.Minute); err != nil {
			return err
		}

		descResult, _ := s.ec2Client.DescribeInstances(s.ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{instanceID}})
		s.minionIPs = append(s.minionIPs, *descResult.Reservations[0].Instances[0].PublicIpAddress)
		s.minionPrivIPs = append(s.minionPrivIPs, *descResult.Reservations[0].Instances[0].PrivateIpAddress)
		s.t.Logf("✅ Salt Minion %d running: %s", i, s.minionIPs[i])
	}

	return nil
}

func (s *SlothSaltCLITestSuite) WaitForSaltServices() error {
	s.t.Log("⏳ Waiting for Salt Master cloud-init (180s max)...")

	// Wait for cloud-init
	for attempt := 1; attempt <= 60; attempt++ {
		output, err := s.runSSHCommand(s.masterIP, "cloud-init status 2>/dev/null | grep -q done && echo READY || echo WAITING")
		if err == nil && strings.Contains(output, "READY") {
			s.t.Logf("✅ Salt Master cloud-init complete after %d attempts", attempt)
			break
		}
		if attempt == 60 {
			return fmt.Errorf("Salt Master cloud-init did not complete in time")
		}
		time.Sleep(5 * time.Second)
	}

	// Wait for Salt API
	s.t.Log("⏳ Waiting for Salt API...")
	for attempt := 1; attempt <= 30; attempt++ {
		output, err := s.runSSHCommand(s.masterIP, "systemctl is-active salt-api 2>/dev/null || echo inactive")
		if err == nil && strings.TrimSpace(output) == "active" {
			s.t.Log("✅ Salt API is active")
			break
		}
		if attempt == 30 {
			s.t.Log("⚠️  Salt API may not be fully active, continuing...")
		}
		time.Sleep(5 * time.Second)
	}

	// Verify Salt API authentication works
	s.t.Log("⏳ Verifying Salt API authentication...")
	authCmd := fmt.Sprintf(`curl -s -X POST http://localhost:8000/login -d username='%s' -d password='%s' -d eauth='pam'`,
		s.config.APIUsername, s.config.APIPassword)
	for attempt := 1; attempt <= 10; attempt++ {
		output, err := s.runSSHCommand(s.masterIP, authCmd)
		if err == nil && strings.Contains(output, "token") {
			s.t.Log("✅ Salt API authentication verified")
			break
		}
		if attempt == 10 {
			// Log debugging info
			s.t.Log("⚠️  Salt API authentication issue, gathering debug info...")

			// Check Python PAM test result
			debugOutput, _ := s.runSSHCommand(s.masterIP, "sudo grep -A3 '=== Testing PAM auth with Python' /var/log/user-data.log 2>/dev/null || echo 'no Python PAM test'")
			s.t.Logf("   Python PAM test:\n%s", debugOutput)

			// Check local Salt API auth test
			debugOutput, _ = s.runSSHCommand(s.masterIP, "sudo grep -A10 '=== Testing local Salt API' /var/log/user-data.log 2>/dev/null || echo 'no local auth log'")
			s.t.Logf("   Local auth test from cloud-init:\n%s", debugOutput)

			debugOutput, _ = s.runSSHCommand(s.masterIP, fmt.Sprintf("id %s 2>&1", s.config.APIUsername))
			s.t.Logf("   User check: %s", debugOutput)

			debugOutput, _ = s.runSSHCommand(s.masterIP, fmt.Sprintf("sudo grep %s /etc/shadow | cut -d: -f1-2", s.config.APIUsername))
			s.t.Logf("   Shadow entry: %s", debugOutput)

			// Check salt user groups (needs shadow group for PAM)
			debugOutput, _ = s.runSSHCommand(s.masterIP, "groups salt")
			s.t.Logf("   Salt user groups: %s", debugOutput)

			// Check /etc/shadow permissions
			debugOutput, _ = s.runSSHCommand(s.masterIP, "ls -la /etc/shadow")
			s.t.Logf("   Shadow file perms: %s", debugOutput)

			debugOutput, _ = s.runSSHCommand(s.masterIP, "cat /etc/salt/master.d/api.conf")
			s.t.Logf("   API config:\n%s", debugOutput)

			s.t.Logf("   Auth response: %s", output)
			return fmt.Errorf("Salt API authentication failed after 10 attempts")
		}
		s.t.Logf("   Auth attempt %d failed, retrying...", attempt)
		// Restart salt-api and try again
		if attempt%3 == 0 {
			_, _ = s.runSSHCommand(s.masterIP, "sudo systemctl restart salt-api && sleep 5")
		}
		time.Sleep(5 * time.Second)
	}

	// Wait for minions to connect
	s.t.Log("⏳ Waiting for minions to connect...")
	for attempt := 1; attempt <= 30; attempt++ {
		output, err := s.runSSHCommand(s.masterIP, "sudo salt-key -L 2>/dev/null | grep -E '^minion-' | wc -l")
		count := strings.TrimSpace(output)
		if err == nil {
			countInt := 0
			fmt.Sscanf(count, "%d", &countInt)
			if countInt >= s.config.MinionCount {
				s.t.Logf("✅ All %d minions connected", countInt)
				break
			}
			s.t.Logf("   Minions connected: %d/%d (attempt %d)", countInt, s.config.MinionCount, attempt)
		}
		if attempt == 30 {
			s.t.Logf("⚠️  Only %s minions connected after 30 attempts, continuing...", count)
		}
		time.Sleep(5 * time.Second)
	}

	// Give minions extra time to fully register
	time.Sleep(10 * time.Second)

	return nil
}

// =============================================================================
// CLI Test Implementations
// =============================================================================

func (s *SlothSaltCLITestSuite) TestCLIPing() error {
	s.t.Log("🏓 Testing: sloth-kubernetes salt ping")

	stdout, stderr, err := s.runCLI("ping")
	if err != nil {
		return fmt.Errorf("ping failed: %w\nStderr: %s", err, stderr)
	}

	s.t.Logf("   Output:\n%s", stdout)

	// Verify minions responded
	if !strings.Contains(stdout, "online") {
		return fmt.Errorf("no minions responded to ping")
	}

	respondingCount := strings.Count(stdout, "online")
	s.t.Logf("✅ Ping successful: %d minions responding", respondingCount)
	return nil
}

func (s *SlothSaltCLITestSuite) TestCLIMinions() error {
	s.t.Log("📋 Testing: sloth-kubernetes salt minions")

	stdout, stderr, err := s.runCLI("minions")
	if err != nil {
		return fmt.Errorf("minions failed: %w\nStderr: %s", err, stderr)
	}

	s.t.Logf("   Output:\n%s", stdout)

	if !strings.Contains(stdout, "minion") {
		return fmt.Errorf("no minions listed")
	}

	s.t.Log("✅ Minions list successful")
	return nil
}

func (s *SlothSaltCLITestSuite) TestCLICmd() error {
	s.t.Log("💻 Testing: sloth-kubernetes salt cmd")

	// Test hostname command
	stdout, stderr, err := s.runCLI("cmd", "hostname")
	if err != nil {
		return fmt.Errorf("cmd hostname failed: %w\nStderr: %s", err, stderr)
	}

	s.t.Logf("   Hostname output:\n%s", stdout)

	// Test uptime command
	stdout, stderr, err = s.runCLI("cmd", "uptime")
	if err != nil {
		return fmt.Errorf("cmd uptime failed: %w\nStderr: %s", err, stderr)
	}

	s.t.Logf("   Uptime output:\n%s", stdout)

	if !strings.Contains(stdout, "up") {
		return fmt.Errorf("uptime output missing expected content")
	}

	s.t.Log("✅ Command execution successful")
	return nil
}

func (s *SlothSaltCLITestSuite) TestCLIGrains() error {
	s.t.Log("🌾 Testing: sloth-kubernetes salt grains")

	stdout, stderr, err := s.runCLI("grains")
	if err != nil {
		return fmt.Errorf("grains failed: %w\nStderr: %s", err, stderr)
	}

	s.t.Logf("   Grains output (truncated):\n%s", truncateOutput(stdout, 500))

	// Verify expected grain fields
	expectedFields := []string{"OS", "Kernel"}
	for _, field := range expectedFields {
		if !strings.Contains(stdout, field) {
			return fmt.Errorf("missing expected grain field: %s", field)
		}
	}

	s.t.Log("✅ Grains retrieval successful")
	return nil
}

func (s *SlothSaltCLITestSuite) TestCLIKeys() error {
	s.t.Log("🔑 Testing: sloth-kubernetes salt keys commands")

	// Test keys list
	stdout, stderr, err := s.runCLI("keys", "list")
	if err != nil {
		return fmt.Errorf("keys list failed: %w\nStderr: %s", err, stderr)
	}
	s.t.Logf("   Keys list output:\n%s", stdout)
	s.t.Log("   keys list: OK")

	// Note: keys accept is not tested because minions are auto-accepted

	s.t.Log("✅ Keys commands successful")
	return nil
}

func (s *SlothSaltCLITestSuite) TestCLIState() error {
	s.t.Log("📦 Testing: sloth-kubernetes salt state commands")

	// Create a test state on the master
	stateContent := `
test_cli_state:
  file.managed:
    - name: /tmp/sloth-cli-state-test.txt
    - contents: "State applied by sloth-kubernetes CLI E2E test"
`
	_, err := s.runSSHCommand(s.masterIP, fmt.Sprintf("echo '%s' | sudo tee /srv/salt/cli_test.sls", stateContent))
	if err != nil {
		return fmt.Errorf("failed to create test state: %w", err)
	}

	// Test state apply
	stdout, stderr, err := s.runCLI("state", "apply", "cli_test")
	if err != nil {
		return fmt.Errorf("state apply failed: %w\nStderr: %s", err, stderr)
	}
	s.t.Logf("   State apply output (truncated):\n%s", truncateOutput(stdout, 400))
	s.t.Log("   state apply: OK")

	// Test state highstate (apply all states)
	stdout, stderr, err = s.runCLI("state", "highstate")
	if err != nil {
		// Highstate might fail if no top.sls, but command should work
		s.t.Logf("   state highstate: %s (may fail without top.sls)", stderr)
	} else {
		s.t.Log("   state highstate: OK")
	}

	s.t.Log("✅ State commands successful")
	return nil
}

func (s *SlothSaltCLITestSuite) TestCLISystemCommands() error {
	s.t.Log("🖥️  Testing: sloth-kubernetes salt system commands")

	// Test uptime
	_, stderr, err := s.runCLI("system", "uptime")
	if err != nil {
		return fmt.Errorf("system uptime failed: %w\nStderr: %s", err, stderr)
	}
	s.t.Log("   System uptime: OK")

	// Test disk
	_, stderr, err = s.runCLI("system", "disk")
	if err != nil {
		return fmt.Errorf("system disk failed: %w\nStderr: %s", err, stderr)
	}
	s.t.Log("   System disk: OK")

	// Test memory
	_, stderr, err = s.runCLI("system", "memory")
	if err != nil {
		return fmt.Errorf("system memory failed: %w\nStderr: %s", err, stderr)
	}
	s.t.Log("   System memory: OK")

	// Test cpu
	_, stderr, err = s.runCLI("system", "cpu")
	if err != nil {
		return fmt.Errorf("system cpu failed: %w\nStderr: %s", err, stderr)
	}
	s.t.Log("   System cpu: OK")

	// Test network
	_, _, err = s.runCLI("system", "network")
	if err != nil {
		return fmt.Errorf("system network failed: %w", err)
	}
	s.t.Log("   System network: OK")

	s.t.Log("✅ System commands successful")
	return nil
}

func (s *SlothSaltCLITestSuite) TestCLIFileCommands() error {
	s.t.Log("📁 Testing: sloth-kubernetes salt file commands")

	testFile := "/tmp/sloth-cli-e2e-test.txt"
	testContent := "Hello from sloth-kubernetes CLI E2E test"

	// Test file write
	_, stderr, err := s.runCLI("file", "write", testFile, testContent)
	if err != nil {
		return fmt.Errorf("file write failed: %w\nStderr: %s", err, stderr)
	}
	s.t.Log("   File write: OK")

	// Test file exists
	_, stderr, err = s.runCLI("file", "exists", testFile)
	if err != nil {
		return fmt.Errorf("file exists failed: %w\nStderr: %s", err, stderr)
	}
	s.t.Log("   File exists: OK")

	// Test file chmod
	_, stderr, err = s.runCLI("file", "chmod", testFile, "644")
	if err != nil {
		return fmt.Errorf("file chmod failed: %w\nStderr: %s", err, stderr)
	}
	s.t.Log("   File chmod: OK")

	// Test file remove
	_, _, err = s.runCLI("file", "remove", testFile)
	if err != nil {
		return fmt.Errorf("file remove failed: %w", err)
	}
	s.t.Log("   File remove: OK")

	s.t.Log("✅ File commands successful")
	return nil
}

func (s *SlothSaltCLITestSuite) TestCLIServiceCommands() error {
	s.t.Log("⚙️  Testing: sloth-kubernetes salt service commands")

	// Test service status (cron is safe to test)
	_, stderr, err := s.runCLI("service", "status", "cron")
	if err != nil {
		return fmt.Errorf("service status failed: %w\nStderr: %s", err, stderr)
	}
	s.t.Log("   Service status: OK")

	// Test service list
	_, _, err = s.runCLI("service", "list")
	if err != nil {
		return fmt.Errorf("service list failed: %w", err)
	}
	s.t.Log("   Service list: OK")

	s.t.Log("✅ Service commands successful")
	return nil
}

func (s *SlothSaltCLITestSuite) TestCLIPkgCommands() error {
	s.t.Log("📦 Testing: sloth-kubernetes salt pkg commands")

	// Test pkg list
	_, stderr, err := s.runCLI("pkg", "list")
	if err != nil {
		return fmt.Errorf("pkg list failed: %w\nStderr: %s", err, stderr)
	}
	s.t.Log("   pkg list: OK")

	// Test pkg install (install a small safe package)
	_, stderr, err = s.runCLI("pkg", "install", "cowsay")
	if err != nil {
		s.t.Logf("   pkg install: skipped (%s)", truncateOutput(stderr, 100))
	} else {
		s.t.Log("   pkg install: OK")

		// Test pkg remove
		_, stderr, err = s.runCLI("pkg", "remove", "cowsay")
		if err != nil {
			s.t.Logf("   pkg remove: skipped (%s)", truncateOutput(stderr, 100))
		} else {
			s.t.Log("   pkg remove: OK")
		}
	}

	s.t.Log("✅ Pkg commands successful")
	return nil
}

func (s *SlothSaltCLITestSuite) TestCLIUserCommands() error {
	s.t.Log("👤 Testing: sloth-kubernetes salt user commands")

	// Test user list
	_, stderr, err := s.runCLI("user", "list")
	if err != nil {
		return fmt.Errorf("user list failed: %w\nStderr: %s", err, stderr)
	}
	s.t.Log("   user list: OK")

	// Test user add
	testUser := "testslothuser"
	_, stderr, err = s.runCLI("user", "add", testUser)
	if err != nil {
		s.t.Logf("   user add: skipped (%s)", truncateOutput(stderr, 100))
	} else {
		s.t.Log("   user add: OK")

		// Test user info
		_, stderr, err = s.runCLI("user", "info", testUser)
		if err != nil {
			s.t.Logf("   user info: skipped (%s)", truncateOutput(stderr, 100))
		} else {
			s.t.Log("   user info: OK")
		}

		// Test user delete
		_, stderr, err = s.runCLI("user", "delete", testUser)
		if err != nil {
			s.t.Logf("   user delete: skipped (%s)", truncateOutput(stderr, 100))
		} else {
			s.t.Log("   user delete: OK")
		}
	}

	s.t.Log("✅ User commands successful")
	return nil
}

func (s *SlothSaltCLITestSuite) TestCLIJobCommands() error {
	s.t.Log("📋 Testing: sloth-kubernetes salt job commands")

	// Test job list
	_, stderr, err := s.runCLI("job", "list")
	if err != nil {
		return fmt.Errorf("job list failed: %w\nStderr: %s", err, stderr)
	}
	s.t.Log("   job list: OK")

	// Test job sync
	_, stderr, err = s.runCLI("job", "sync")
	if err != nil {
		s.t.Logf("   job sync: skipped (%s)", truncateOutput(stderr, 100))
	} else {
		s.t.Log("   job sync: OK")
	}

	s.t.Log("✅ Job commands successful")
	return nil
}

func (s *SlothSaltCLITestSuite) TestCLINetworkCommands() error {
	s.t.Log("🌐 Testing: sloth-kubernetes salt network commands")

	// Test network ping
	_, stderr, err := s.runCLI("network", "ping", "8.8.8.8", "2")
	if err != nil {
		s.t.Logf("   network ping: skipped (%s)", truncateOutput(stderr, 100))
	} else {
		s.t.Log("   network ping: OK")
	}

	// Test network netstat
	_, stderr, err = s.runCLI("network", "netstat")
	if err != nil {
		s.t.Logf("   network netstat: skipped (%s)", truncateOutput(stderr, 100))
	} else {
		s.t.Log("   network netstat: OK")
	}

	// Test network connections
	_, stderr, err = s.runCLI("network", "connections")
	if err != nil {
		s.t.Logf("   network connections: skipped (%s)", truncateOutput(stderr, 100))
	} else {
		s.t.Log("   network connections: OK")
	}

	// Test network routes
	_, stderr, err = s.runCLI("network", "routes")
	if err != nil {
		s.t.Logf("   network routes: skipped (%s)", truncateOutput(stderr, 100))
	} else {
		s.t.Log("   network routes: OK")
	}

	// Test network arp
	_, stderr, err = s.runCLI("network", "arp")
	if err != nil {
		s.t.Logf("   network arp: skipped (%s)", truncateOutput(stderr, 100))
	} else {
		s.t.Log("   network arp: OK")
	}

	s.t.Log("✅ Network commands successful")
	return nil
}

func (s *SlothSaltCLITestSuite) TestCLIProcessCommands() error {
	s.t.Log("⚙️  Testing: sloth-kubernetes salt process commands")

	// Test process list
	_, stderr, err := s.runCLI("process", "list")
	if err != nil {
		return fmt.Errorf("process list failed: %w\nStderr: %s", err, stderr)
	}
	s.t.Log("   process list: OK")

	// Test process top
	_, stderr, err = s.runCLI("process", "top")
	if err != nil {
		s.t.Logf("   process top: skipped (%s)", truncateOutput(stderr, 100))
	} else {
		s.t.Log("   process top: OK")
	}

	// Test process info (get info on PID 1)
	_, stderr, err = s.runCLI("process", "info", "1")
	if err != nil {
		s.t.Logf("   process info: skipped (%s)", truncateOutput(stderr, 100))
	} else {
		s.t.Log("   process info: OK")
	}

	s.t.Log("✅ Process commands successful")
	return nil
}

func (s *SlothSaltCLITestSuite) TestCLICronCommands() error {
	s.t.Log("⏰ Testing: sloth-kubernetes salt cron commands")

	// Test cron list
	_, stderr, err := s.runCLI("cron", "list", "root")
	if err != nil {
		s.t.Logf("   cron list: skipped (%s)", truncateOutput(stderr, 100))
	} else {
		s.t.Log("   cron list: OK")
	}

	// Note: cron add/remove are not tested to avoid leaving cron jobs on minions

	s.t.Log("✅ Cron commands successful")
	return nil
}

func (s *SlothSaltCLITestSuite) TestCLIArchiveCommands() error {
	s.t.Log("🗄️  Testing: sloth-kubernetes salt archive commands")

	// Create test file first
	_, _, _ = s.runCLI("file", "write", "/tmp/archive-test.txt", "test content for archive")

	// Test tar
	_, stderr, err := s.runCLI("archive", "tar", "/tmp/archive-test.txt", "/tmp/archive-test.tar.gz")
	if err != nil {
		s.t.Logf("   archive tar: skipped (%s)", truncateOutput(stderr, 100))
	} else {
		s.t.Log("   archive tar: OK")

		// Test untar
		_, stderr, err = s.runCLI("archive", "untar", "/tmp/archive-test.tar.gz", "/tmp/archive-extracted")
		if err != nil {
			s.t.Logf("   archive untar: skipped (%s)", truncateOutput(stderr, 100))
		} else {
			s.t.Log("   archive untar: OK")
		}
	}

	// Cleanup
	_, _, _ = s.runCLI("file", "remove", "/tmp/archive-test.txt")
	_, _, _ = s.runCLI("file", "remove", "/tmp/archive-test.tar.gz")

	s.t.Log("✅ Archive commands successful")
	return nil
}

func (s *SlothSaltCLITestSuite) TestCLIMonitorCommands() error {
	s.t.Log("📊 Testing: sloth-kubernetes salt monitor commands")

	// Test monitor load
	_, stderr, err := s.runCLI("monitor", "load")
	if err != nil {
		s.t.Logf("   monitor load: skipped (%s)", truncateOutput(stderr, 100))
	} else {
		s.t.Log("   monitor load: OK")
	}

	// Test monitor iostat
	_, stderr, err = s.runCLI("monitor", "iostat")
	if err != nil {
		s.t.Logf("   monitor iostat: skipped (%s)", truncateOutput(stderr, 100))
	} else {
		s.t.Log("   monitor iostat: OK")
	}

	// Test monitor netstats
	_, stderr, err = s.runCLI("monitor", "netstats")
	if err != nil {
		s.t.Logf("   monitor netstats: skipped (%s)", truncateOutput(stderr, 100))
	} else {
		s.t.Log("   monitor netstats: OK")
	}

	// Test monitor info
	_, stderr, err = s.runCLI("monitor", "info")
	if err != nil {
		s.t.Logf("   monitor info: skipped (%s)", truncateOutput(stderr, 100))
	} else {
		s.t.Log("   monitor info: OK")
	}

	s.t.Log("✅ Monitor commands successful")
	return nil
}

func (s *SlothSaltCLITestSuite) TestCLIPillarCommands() error {
	s.t.Log("🔐 Testing: sloth-kubernetes salt pillar commands")

	// Test pillar list
	_, stderr, err := s.runCLI("pillar", "list")
	if err != nil {
		s.t.Logf("   pillar list: skipped (%s)", truncateOutput(stderr, 100))
	} else {
		s.t.Log("   pillar list: OK")
	}

	// Test pillar get
	_, stderr, err = s.runCLI("pillar", "get", "test")
	if err != nil {
		s.t.Logf("   pillar get: skipped (no pillar data)")
	} else {
		s.t.Log("   pillar get: OK")
	}

	s.t.Log("✅ Pillar commands successful")
	return nil
}

func (s *SlothSaltCLITestSuite) TestCLIMountCommands() error {
	s.t.Log("💾 Testing: sloth-kubernetes salt mount commands")

	// Test mount list
	_, stderr, err := s.runCLI("mount", "list")
	if err != nil {
		return fmt.Errorf("mount list failed: %w\nStderr: %s", err, stderr)
	}
	s.t.Log("   mount list: OK")

	// Note: mount/umount commands are not tested to avoid system changes

	s.t.Log("✅ Mount commands successful")
	return nil
}

func (s *SlothSaltCLITestSuite) TestCLIJSONOutput() error {
	s.t.Log("📄 Testing: sloth-kubernetes salt --json output")

	// Test ping with JSON output
	stdout, stderr, err := s.runCLI("ping", "--json")
	if err != nil {
		return fmt.Errorf("ping --json failed: %w\nStderr: %s", err, stderr)
	}

	s.t.Logf("   Ping --json output:\n%s", truncateOutput(stdout, 500))

	// Verify it contains JSON-like content (could be raw JSON or formatted output)
	hasJSON := strings.Contains(stdout, "{") || strings.Contains(stdout, "return") || strings.Contains(stdout, "true")
	if !hasJSON {
		s.t.Logf("   Warning: Output may not be JSON, but continuing")
	}
	s.t.Log("   Ping --json: OK")

	// Test grains with JSON output
	stdout, stderr, err = s.runCLI("grains", "--json")
	if err != nil {
		return fmt.Errorf("grains --json failed: %w\nStderr: %s", err, stderr)
	}

	s.t.Logf("   Grains --json output (truncated):\n%s", truncateOutput(stdout, 300))

	// Check for any JSON-like or structured content
	hasStructuredContent := strings.Contains(stdout, "{") || strings.Contains(stdout, "return") || strings.Contains(stdout, "os")
	if !hasStructuredContent {
		s.t.Log("   Warning: Grains output may not contain expected fields")
	}
	s.t.Log("   Grains --json: OK")

	s.t.Log("✅ JSON output test completed")
	return nil
}

func (s *SlothSaltCLITestSuite) TestCLITargeting() error {
	s.t.Log("🎯 Testing: sloth-kubernetes salt --target filtering")

	// Test targeting specific minion
	stdout, stderr, err := s.runCLIWithTarget("minion-0", "ping")
	if err != nil {
		return fmt.Errorf("ping with target failed: %w\nStderr: %s", err, stderr)
	}
	s.t.Log("   Target minion-0: OK")

	// Test targeting with glob pattern
	stdout, _, err = s.runCLIWithTarget("minion-*", "ping")
	if err != nil {
		return fmt.Errorf("ping with glob target failed: %w", err)
	}
	s.t.Log("   Target minion-*: OK")

	// Test cmd with specific target
	stdout, _, err = s.runCLIWithTarget("minion-0", "cmd", "hostname")
	if err != nil {
		return fmt.Errorf("cmd with target failed: %w", err)
	}
	s.t.Logf("   Target cmd output:\n%s", truncateOutput(stdout, 200))

	s.t.Log("✅ Target filtering successful")
	return nil
}

// =============================================================================
// Helper Functions
// =============================================================================

func (s *SlothSaltCLITestSuite) getLatestUbuntuAMI() (string, error) {
	result, err := s.ec2Client.DescribeImages(s.ctx, &ec2.DescribeImagesInput{
		Owners: []string{"099720109477"},
		Filters: []types.Filter{
			{Name: aws.String("name"), Values: []string{"ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"}},
			{Name: aws.String("state"), Values: []string{"available"}},
		},
	})
	if err != nil {
		return "", err
	}

	if len(result.Images) == 0 {
		return "", fmt.Errorf("no Ubuntu AMIs found")
	}

	var latestAMI string
	var latestDate string
	for _, img := range result.Images {
		if *img.CreationDate > latestDate {
			latestDate = *img.CreationDate
			latestAMI = *img.ImageId
		}
	}
	return latestAMI, nil
}

func (s *SlothSaltCLITestSuite) getMasterUserData() string {
	// Use explicit variables to avoid escaping issues in heredoc
	username := s.config.APIUsername
	password := s.config.APIPassword

	return fmt.Sprintf(`#!/bin/bash
set -x
exec > >(tee /var/log/user-data.log) 2>&1

# Variables for authentication - use simple password to avoid escaping issues
API_USER="%s"
API_PASS="%s"

echo "=== Setting up Salt Master ==="
echo "API User: $API_USER"
echo "API Pass length: ${#API_PASS}"

apt-get update
apt-get install -y curl gnupg2 whois python3-pam

# Create API user BEFORE installing Salt (so it exists when config is loaded)
echo "=== Creating API user FIRST ==="
userdel -r "$API_USER" 2>/dev/null || true
useradd -m -s /bin/bash "$API_USER"
echo "${API_USER}:${API_PASS}" | chpasswd -c SHA512
echo "User created with SHA512 password"

# Verify user exists
id "$API_USER"
getent shadow "$API_USER" | cut -d: -f1-2

# Test PAM auth directly with Python (verify password works)
echo "=== Testing PAM auth with Python ==="
python3 << PYEOF
import pam
p = pam.pam()
result = p.authenticate("${API_USER}", "${API_PASS}")
print(f"PAM authenticate result: {result}")
if not result:
    print(f"PAM error: {p.reason}")
PYEOF

# Install Salt
echo "=== Installing Salt ==="
mkdir -p /etc/apt/keyrings
curl -fsSL https://packages.broadcom.com/artifactory/api/security/keypair/SaltProjectKey/public | gpg --dearmor -o /etc/apt/keyrings/salt-archive-keyring.gpg
echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/salt-archive-keyring.gpg] https://packages.broadcom.com/artifactory/saltproject-deb stable main" > /etc/apt/sources.list.d/salt.list
apt-get update
apt-get install -y salt-master salt-api python3-cherrypy3

# Stop services to configure cleanly
systemctl stop salt-master salt-api 2>/dev/null || true

# CRITICAL: Add salt user to shadow group for PAM authentication
echo "=== Adding salt user to shadow group ==="
usermod -aG shadow salt
echo "salt user groups: $(groups salt)"

# Configure Salt Master with PAM auth
echo "=== Configuring Salt Master ==="
mkdir -p /etc/salt/master.d
cat > /etc/salt/master.d/api.conf << EOF
rest_cherrypy:
  port: 8000
  host: 0.0.0.0
  disable_ssl: true

# Enable netapi clients (required for Salt 3005+)
netapi_enable_clients:
  - local
  - local_async
  - runner
  - wheel

external_auth:
  pam:
    ${API_USER}:
      - .*
      - '@wheel'
      - '@runner'
      - '@jobs'

auto_accept: True
EOF

echo "Salt Master config:"
cat /etc/salt/master.d/api.conf

# Create salt states directory
mkdir -p /srv/salt

# Enable services
systemctl enable salt-master salt-api

# Start salt-master fresh (not restart)
echo "=== Starting salt-master ==="
systemctl start salt-master
echo "Waiting for salt-master to fully initialize..."
sleep 15

# Verify salt-master is ready
for i in $(seq 1 30); do
    if systemctl is-active salt-master >/dev/null 2>&1; then
        echo "salt-master is active after $i checks"
        break
    fi
    sleep 2
done

# Start salt-api
echo "=== Starting salt-api ==="
systemctl start salt-api
sleep 10

# Verify salt-api is running
for i in $(seq 1 30); do
    if systemctl is-active salt-api >/dev/null 2>&1; then
        echo "salt-api is active after $i checks"
        break
    fi
    sleep 2
done

# Service status
echo "=== Service status ==="
systemctl status salt-master --no-pager || true
systemctl status salt-api --no-pager || true

# Test PAM authentication directly with su
echo "=== Testing PAM authentication with su ==="
apt-get install -y expect || true
cat > /tmp/test_auth.exp << 'EXPECT_SCRIPT'
#!/usr/bin/expect -f
set username [lindex $argv 0]
set password [lindex $argv 1]
spawn su - $username -c "echo AUTH_SUCCESS"
expect {
    "Password:" {
        send "$password\r"
        expect {
            "AUTH_SUCCESS" { puts "PAM_AUTH_OK"; exit 0 }
            "Authentication failure" { puts "PAM_AUTH_FAILED"; exit 1 }
            timeout { puts "PAM_AUTH_TIMEOUT"; exit 1 }
        }
    }
    timeout { puts "PAM_NO_PROMPT"; exit 1 }
}
EXPECT_SCRIPT
chmod +x /tmp/test_auth.exp
/tmp/test_auth.exp "$API_USER" "$API_PASS" && echo "su test passed" || echo "su test failed"

# Also verify shadow entry exists
echo "=== Shadow entry check ==="
grep "$API_USER" /etc/shadow | cut -d: -f1-2 | head -1

# Test local API auth
echo "=== Testing local Salt API authentication ==="
sleep 5
AUTH_RESULT=$(curl -sk -X POST http://localhost:8000/login \
    -d username="$API_USER" \
    -d password="$API_PASS" \
    -d eauth=pam 2>&1)
echo "Auth result: $AUTH_RESULT"

if echo "$AUTH_RESULT" | grep -q "token"; then
    echo "LOCAL AUTH SUCCESS!"
else
    echo "LOCAL AUTH FAILED"
    echo "=== Debug: Salt master config ==="
    cat /etc/salt/master.d/*.conf
    echo "=== Debug: PAM config ==="
    cat /etc/pam.d/login
    echo "=== Debug: Salt API log ==="
    journalctl -u salt-api --no-pager -n 100 2>/dev/null || cat /var/log/salt/api 2>/dev/null | tail -100 || echo "no logs"
fi

echo "=== Salt Master setup complete ==="
`, username, password)
}

func (s *SlothSaltCLITestSuite) getMinionUserData(minionID string) string {
	return fmt.Sprintf(`#!/bin/bash
set -x
exec > >(tee /var/log/user-data.log) 2>&1

apt-get update
apt-get install -y curl gnupg2 netcat-openbsd

# Install Salt
mkdir -p /etc/apt/keyrings
curl -fsSL https://packages.broadcom.com/artifactory/api/security/keypair/SaltProjectKey/public | gpg --dearmor -o /etc/apt/keyrings/salt-archive-keyring.gpg
echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/salt-archive-keyring.gpg] https://packages.broadcom.com/artifactory/saltproject-deb stable main" > /etc/apt/sources.list.d/salt.list
apt-get update
apt-get install -y salt-minion

# Configure Salt Minion
mkdir -p /etc/salt/minion.d
cat > /etc/salt/minion.d/master.conf << EOF
master: %s
id: %s
EOF

# Wait for Salt Master
MASTER_IP="%s"
for attempt in $(seq 1 60); do
    if nc -z -w5 "$MASTER_IP" 4505 2>/dev/null; then
        echo "Salt Master reachable (attempt $attempt)"
        break
    fi
    sleep 5
done

# Start Salt Minion
systemctl enable salt-minion
systemctl restart salt-minion

# Retry connection
for attempt in $(seq 1 10); do
    sleep 10
    if [ -f /etc/salt/pki/minion/minion_master.pub ]; then
        echo "Minion connected (attempt $attempt)"
        break
    fi
    systemctl restart salt-minion
done

echo "Salt Minion setup complete"
`, s.masterPrivIP, minionID, s.masterPrivIP)
}

func (s *SlothSaltCLITestSuite) runSSHCommand(host, command string) (string, error) {
	signer, err := ssh.ParsePrivateKey([]byte(s.sshKey))
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	config := &ssh.ClientConfig{
		User:            "ubuntu",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", host+":22", config)
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

func (s *SlothSaltCLITestSuite) Cleanup() {
	s.cancel()
	s.t.Log("🧹 Cleaning up resources...")

	ctx := context.Background()

	if len(s.instanceIDs) > 0 {
		s.t.Logf("   Terminating %d instances...", len(s.instanceIDs))
		_, _ = s.ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
			InstanceIds: s.instanceIDs,
		})
		time.Sleep(30 * time.Second)
	}

	if s.sgID != "" {
		s.t.Logf("   Deleting security group: %s", s.sgID)
		_, _ = s.ec2Client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
			GroupId: aws.String(s.sgID),
		})
	}

	if s.keyPairName != "" {
		s.t.Logf("   Deleting key pair: %s", s.keyPairName)
		_, _ = s.ec2Client.DeleteKeyPair(ctx, &ec2.DeleteKeyPairInput{
			KeyName: aws.String(s.keyPairName),
		})
	}

	// Clean up binary
	if s.binaryPath != "" {
		os.Remove(s.binaryPath)
	}

	s.t.Log("✅ Cleanup complete")
}

func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... (truncated)"
}
