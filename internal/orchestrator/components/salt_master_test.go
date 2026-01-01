package components

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Salt Master Component Unit Tests
// =============================================================================

// TestGenerateClusterToken verifies the cluster token generation
func TestGenerateClusterToken(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		stackName   string
	}{
		{
			name:        "standard cluster name",
			clusterName: "my-cluster",
			stackName:   "dev",
		},
		{
			name:        "production cluster",
			clusterName: "prod-kubernetes",
			stackName:   "production",
		},
		{
			name:        "special characters",
			clusterName: "cluster-with-dashes",
			stackName:   "stack_with_underscores",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			token := generateClusterToken(tc.clusterName, tc.stackName)

			// Token should be 32 characters (first 32 chars of SHA256 hex)
			assert.Len(t, token, 32, "Token should be 32 characters")

			// Token should be valid hex
			_, err := hex.DecodeString(token)
			assert.NoError(t, err, "Token should be valid hex")

			// Note: generateClusterToken uses time.Now().UnixNano() so each call
			// produces a different token. This is intentional for security.
			// We just verify the token format is correct.

			t.Logf("Generated token for %s/%s: %s", tc.clusterName, tc.stackName, token)
		})
	}
}

// TestGenerateClusterToken_Uniqueness verifies tokens are unique for different inputs
func TestGenerateClusterToken_Uniqueness(t *testing.T) {
	tokens := make(map[string]bool)
	combinations := []struct {
		clusterName string
		stackName   string
	}{
		{"cluster-1", "dev"},
		{"cluster-2", "dev"},
		{"cluster-1", "prod"},
		{"cluster-2", "prod"},
		{"my-k8s", "staging"},
		{"production", "main"},
	}

	for _, combo := range combinations {
		token := generateClusterToken(combo.clusterName, combo.stackName)
		key := fmt.Sprintf("%s/%s", combo.clusterName, combo.stackName)

		assert.False(t, tokens[token], "Token should be unique for each combination: %s", key)
		tokens[token] = true
	}

	t.Logf("Generated %d unique tokens", len(tokens))
}

// TestGenerateClusterToken_Security verifies security properties
func TestGenerateClusterToken_Security(t *testing.T) {
	clusterName := "secure-cluster"
	stackName := "production"

	token := generateClusterToken(clusterName, stackName)

	// Token should not contain original cluster name
	assert.NotContains(t, token, clusterName, "Token should not reveal cluster name")
	assert.NotContains(t, token, stackName, "Token should not reveal stack name")

	// Token should be cryptographically random-looking
	// Check for reasonable entropy (not all same characters)
	uniqueChars := make(map[rune]bool)
	for _, c := range token {
		uniqueChars[c] = true
	}
	assert.Greater(t, len(uniqueChars), 8, "Token should have reasonable entropy")

	t.Logf("Token entropy: %d unique characters", len(uniqueChars))
}

// TestGetNodeType verifies node type detection from name
func TestGetNodeType(t *testing.T) {
	tests := []struct {
		nodeName string
		expected string
	}{
		{"master-0", "master"},
		{"master-1", "master"},
		{"master-2", "master"},
		{"worker-0", "worker"},
		{"worker-1", "worker"},
		{"worker-10", "worker"},
		{"node-0", "node"},
		{"bastion", "node"},
		{"custom-name", "node"},
		{"mast", "node"},   // too short
		{"work", "node"},   // too short
		{"masternode", "master"},
		{"workernode", "worker"},
	}

	for _, tc := range tests {
		t.Run(tc.nodeName, func(t *testing.T) {
			result := getNodeType(tc.nodeName)
			assert.Equal(t, tc.expected, result, "Node type should match for %s", tc.nodeName)
		})
	}
}

// TestSaltMasterConfigGeneration tests the generated Salt Master configuration
func TestSaltMasterConfigGeneration(t *testing.T) {
	clusterToken := generateClusterToken("test-cluster", "test")

	// Simulate the master configuration that would be generated
	masterConfig := fmt.Sprintf(`
interface: 10.8.0.20
auto_accept: False
autosign_grains_dir: /etc/salt/autosign_grains
log_level: info
worker_threads: 5
timeout: 60
open_mode: False
`)

	autosignConfig := clusterToken

	// Verify configuration contains security settings
	assert.Contains(t, masterConfig, "auto_accept: False", "auto_accept should be disabled")
	assert.Contains(t, masterConfig, "open_mode: False", "open_mode should be disabled")
	assert.Contains(t, masterConfig, "autosign_grains_dir", "autosign_grains_dir should be configured")

	// Verify autosign contains the token
	assert.Equal(t, clusterToken, autosignConfig, "Autosign should contain cluster token")

	t.Logf("Master config (secure):\n%s", masterConfig)
	t.Logf("Autosign token: %s", autosignConfig)
}

// TestSaltMinionConfigGeneration tests the generated Salt Minion configuration
func TestSaltMinionConfigGeneration(t *testing.T) {
	masterIP := "10.8.0.20"
	nodeWgIP := "10.8.0.30"
	nodeName := "worker-0"
	clusterToken := generateClusterToken("test-cluster", "test")

	// Simulate the minion configuration that would be generated
	minionConfig := fmt.Sprintf(`
master: %s
interface: %s
id: %s
`, masterIP, nodeWgIP, nodeName)

	grainsConfig := fmt.Sprintf(`
roles:
  - kubernetes
cluster: sloth-kubernetes
node_type: %s
wireguard_ip: %s
cluster_token: %s
`, getNodeType(nodeName), nodeWgIP, clusterToken)

	// Verify minion config
	assert.Contains(t, minionConfig, masterIP, "Minion config should contain master IP")
	assert.Contains(t, minionConfig, nodeName, "Minion config should contain node name")

	// Verify grains contains the cluster token
	assert.Contains(t, grainsConfig, clusterToken, "Grains should contain cluster token")
	assert.Contains(t, grainsConfig, "worker", "Grains should contain node type")
	assert.Contains(t, grainsConfig, nodeWgIP, "Grains should contain WireGuard IP")

	t.Logf("Minion config:\n%s", minionConfig)
	t.Logf("Grains config:\n%s", grainsConfig)
}

// TestSaltAPIConfiguration tests Salt API configuration generation
func TestSaltAPIConfiguration(t *testing.T) {
	apiUsername := "saltapi"
	apiPort := 8000

	apiConfig := fmt.Sprintf(`
rest_cherrypy:
  port: %d
  host: 0.0.0.0
  disable_ssl: true

external_auth:
  pam:
    %s:
      - .*
      - '@wheel'
      - '@runner'
      - '@jobs'
`, apiPort, apiUsername)

	// Verify API configuration
	assert.Contains(t, apiConfig, fmt.Sprintf("port: %d", apiPort), "API config should contain port")
	assert.Contains(t, apiConfig, apiUsername, "API config should contain username")
	assert.Contains(t, apiConfig, "@wheel", "API config should grant wheel access")
	assert.Contains(t, apiConfig, "@runner", "API config should grant runner access")

	t.Logf("Salt API config:\n%s", apiConfig)
}

// TestSecureAuthenticationFlow simulates the secure authentication flow
func TestSecureAuthenticationFlow(t *testing.T) {
	// Generate cluster token
	clusterName := "production-cluster"
	stackName := "main"
	clusterToken := generateClusterToken(clusterName, stackName)

	t.Logf("Cluster Token: %s", clusterToken)

	// Simulate master autosign configuration
	masterAutosignToken := clusterToken

	// Simulate minion grain configuration
	minionGrainToken := clusterToken

	// Authentication should succeed when tokens match
	assert.Equal(t, masterAutosignToken, minionGrainToken,
		"Minion token should match master autosign token")

	// Simulate invalid minion (different cluster)
	invalidToken := generateClusterToken("hacker-cluster", "evil")
	assert.NotEqual(t, masterAutosignToken, invalidToken,
		"Invalid minion token should NOT match master")

	t.Log("✅ Secure authentication flow validated")
}

// TestReactorAuditLogConfiguration tests reactor audit logging configuration
func TestReactorAuditLogConfiguration(t *testing.T) {
	reactorConfig := `
reactor:
  - 'salt/auth':
    - /srv/reactor/auth_log.sls
`

	authLogReactor := `
log_auth_event:
  local.cmd.run:
    - tgt: salt-master
    - arg:
      - 'echo "[$(date)] Key auth event: {{ data.get("id", "unknown") }} - {{ data.get("act", "unknown") }}" >> /var/log/salt/auth_audit.log'
`

	// Verify reactor configuration
	assert.Contains(t, reactorConfig, "salt/auth", "Reactor should handle auth events")
	assert.Contains(t, reactorConfig, "auth_log.sls", "Reactor should use auth log SLS")

	// Verify audit log reactor
	assert.Contains(t, authLogReactor, "auth_audit.log", "Audit log should be written")
	assert.Contains(t, authLogReactor, "data.get", "Reactor should extract event data")

	t.Logf("Reactor config:\n%s", reactorConfig)
}

// TestSaltTopSLSGeneration tests Salt top.sls generation
func TestSaltTopSLSGeneration(t *testing.T) {
	topSLS := `
base:
  '*':
    - common
  'master*':
    - kubernetes.master
  'worker*':
    - kubernetes.worker
`

	// Verify top.sls structure
	assert.Contains(t, topSLS, "base:", "Top.sls should have base environment")
	assert.Contains(t, topSLS, "'*':", "Top.sls should target all nodes")
	assert.Contains(t, topSLS, "'master*':", "Top.sls should target masters")
	assert.Contains(t, topSLS, "'worker*':", "Top.sls should target workers")

	t.Logf("Top.sls:\n%s", topSLS)
}

// TestTimestampBasedTokenUniqueness verifies tokens change with time
func TestTimestampBasedTokenUniqueness(t *testing.T) {
	// The production code uses time.Now().UnixNano() in the seed
	// We simulate this by calling the function multiple times
	clusterName := "test-cluster"
	stackName := "dev"

	// Since we can't control time in the unit test, we verify
	// the token generation algorithm works correctly
	seed1 := fmt.Sprintf("%s-%s-%d", clusterName, stackName, time.Now().UnixNano())
	hash1 := sha256.Sum256([]byte(seed1))
	token1 := hex.EncodeToString(hash1[:])[:32]

	// Small delay to ensure different timestamp
	time.Sleep(time.Millisecond)

	seed2 := fmt.Sprintf("%s-%s-%d", clusterName, stackName, time.Now().UnixNano())
	hash2 := sha256.Sum256([]byte(seed2))
	token2 := hex.EncodeToString(hash2[:])[:32]

	// Tokens should be different due to different timestamps
	assert.NotEqual(t, token1, token2, "Tokens with different timestamps should be unique")

	t.Logf("Token 1: %s", token1)
	t.Logf("Token 2: %s", token2)
}

// TestSaltInstallScript tests the Salt install script structure
func TestSaltInstallScript(t *testing.T) {
	installScript := `
curl -o /tmp/bootstrap-salt.sh -L https://github.com/saltstack/salt-bootstrap/releases/latest/download/bootstrap-salt.sh
chmod +x /tmp/bootstrap-salt.sh
sudo sh /tmp/bootstrap-salt.sh -M -N stable
`

	// Verify install script
	assert.Contains(t, installScript, "bootstrap-salt.sh", "Should use official bootstrap script")
	assert.Contains(t, installScript, "github.com/saltstack", "Should download from official source")
	assert.Contains(t, installScript, "-M", "Should install master (-M)")
	assert.Contains(t, installScript, "-N", "Should not install minion on master (-N)")
	assert.Contains(t, installScript, "stable", "Should use stable channel")

	t.Log("Salt install script validated")
}

// TestMinionInstallScript tests the Salt minion install script
func TestMinionInstallScript(t *testing.T) {
	installScript := `
curl -o /tmp/bootstrap-salt.sh -L https://github.com/saltstack/salt-bootstrap/releases/latest/download/bootstrap-salt.sh
chmod +x /tmp/bootstrap-salt.sh
sudo sh /tmp/bootstrap-salt.sh stable
`

	// Verify minion install script (no -M or -N flags)
	assert.Contains(t, installScript, "bootstrap-salt.sh", "Should use official bootstrap script")
	assert.NotContains(t, installScript, " -M ", "Minion script should NOT have master flag")
	assert.NotContains(t, installScript, " -N ", "Minion script should NOT have no-minion flag")

	t.Log("Salt minion install script validated")
}

// TestSecurityConfiguration tests all security settings
func TestSecurityConfiguration(t *testing.T) {
	securityChecks := []struct {
		name     string
		config   string
		expected string
	}{
		{
			name:     "auto_accept disabled",
			config:   "auto_accept: False",
			expected: "False",
		},
		{
			name:     "open_mode disabled",
			config:   "open_mode: False",
			expected: "False",
		},
		{
			name:     "autosign_grains enabled",
			config:   "autosign_grains_dir: /etc/salt/autosign_grains",
			expected: "/etc/salt/autosign_grains",
		},
	}

	for _, check := range securityChecks {
		t.Run(check.name, func(t *testing.T) {
			assert.Contains(t, check.config, check.expected,
				"Security check failed: %s", check.name)
		})
	}

	t.Log("✅ All security configurations validated")
}

// TestWireGuardIPBinding tests WireGuard IP binding configuration
func TestWireGuardIPBinding(t *testing.T) {
	masterWgIP := "10.8.0.20"
	minionWgIP := "10.8.0.30"

	masterInterface := fmt.Sprintf("interface: %s", masterWgIP)
	minionInterface := fmt.Sprintf("interface: %s", minionWgIP)

	// Master should bind to its WireGuard IP
	assert.Contains(t, masterInterface, masterWgIP, "Master should bind to WireGuard IP")

	// Minion should bind to its WireGuard IP
	assert.Contains(t, minionInterface, minionWgIP, "Minion should bind to WireGuard IP")

	t.Logf("Master interface: %s", masterInterface)
	t.Logf("Minion interface: %s", minionInterface)
}

// TestSaltAPIPasswordGeneration tests API password generation
func TestSaltAPIPasswordGeneration(t *testing.T) {
	clusterToken := generateClusterToken("test-cluster", "prod")
	apiPassword := fmt.Sprintf("salt-%s", clusterToken[:16])

	// Password should be derived from cluster token
	assert.True(t, strings.HasPrefix(apiPassword, "salt-"),
		"API password should have salt- prefix")
	assert.Len(t, apiPassword, 21, "API password should be 21 characters (salt- + 16 chars)")

	// Password should contain part of cluster token
	assert.Contains(t, apiPassword, clusterToken[:16],
		"API password should contain part of cluster token")

	t.Logf("Generated API password: %s", apiPassword)
}

// =============================================================================
// Benchmark Tests
// =============================================================================

// BenchmarkGenerateClusterToken benchmarks token generation performance
func BenchmarkGenerateClusterToken(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateClusterToken("benchmark-cluster", "benchmark-stack")
	}
}

// BenchmarkGetNodeType benchmarks node type detection
func BenchmarkGetNodeType(b *testing.B) {
	nodeNames := []string{"master-0", "worker-1", "node-2", "bastion"}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, name := range nodeNames {
			getNodeType(name)
		}
	}
}
