package network

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/stretchr/testify/assert"
)

// TestManager_GetKubernetesFirewallRules tests the getKubernetesFirewallRules method
func TestManager_GetKubernetesFirewallRules(t *testing.T) {
	m := &Manager{
		config: &config.NetworkConfig{
			EnableNodePorts: false,
		},
	}

	rules := m.getKubernetesFirewallRules()

	// Verify we have standard Kubernetes rules
	assert.NotEmpty(t, rules)
	assert.Greater(t, len(rules), 5, "Should have at least 5 basic Kubernetes rules")

	// Check for API server rule
	hasAPIServerRule := false
	for _, rule := range rules {
		if rule.Port == "6443" {
			hasAPIServerRule = true
			assert.Equal(t, "tcp", rule.Protocol)
			assert.Contains(t, rule.Description, "Kubernetes API")
			break
		}
	}
	assert.True(t, hasAPIServerRule, "Should have Kubernetes API server rule")
}

// TestManager_GetKubernetesFirewallRules_WithNodePorts tests rules with NodePorts enabled
func TestManager_GetKubernetesFirewallRules_WithNodePorts(t *testing.T) {
	m := &Manager{
		config: &config.NetworkConfig{
			EnableNodePorts: true,
		},
	}

	rules := m.getKubernetesFirewallRules()

	// Check for NodePort rule
	hasNodePortRule := false
	for _, rule := range rules {
		if rule.Port == "30000-32767" {
			hasNodePortRule = true
			assert.Equal(t, "tcp", rule.Protocol)
			assert.Contains(t, rule.Description, "NodePort")
			break
		}
	}
	assert.True(t, hasNodePortRule, "Should have NodePort rule when enabled")
}

// TestManager_GetKubernetesFirewallRules_WithoutNodePorts tests rules without NodePorts
func TestManager_GetKubernetesFirewallRules_WithoutNodePorts(t *testing.T) {
	m := &Manager{
		config: &config.NetworkConfig{
			EnableNodePorts: false,
		},
	}

	rules := m.getKubernetesFirewallRules()

	// Check that NodePort rule is NOT present
	for _, rule := range rules {
		assert.NotEqual(t, "30000-32767", rule.Port, "Should not have NodePort rule when disabled")
	}
}

// TestManager_GetKubernetesFirewallRules_EtcdRules tests etcd-specific rules
func TestManager_GetKubernetesFirewallRules_EtcdRules(t *testing.T) {
	m := &Manager{
		config: &config.NetworkConfig{
			EnableNodePorts: false,
		},
	}

	rules := m.getKubernetesFirewallRules()

	// Check for etcd rule
	hasEtcdRule := false
	for _, rule := range rules {
		if rule.Port == "2379-2380" {
			hasEtcdRule = true
			assert.Equal(t, "tcp", rule.Protocol)
			assert.Contains(t, rule.Description, "etcd")
			break
		}
	}
	assert.True(t, hasEtcdRule, "Should have etcd rules")
}

// TestManager_GetKubernetesFirewallRules_KubeletRules tests kubelet-specific rules
func TestManager_GetKubernetesFirewallRules_KubeletRules(t *testing.T) {
	m := &Manager{
		config: &config.NetworkConfig{
			EnableNodePorts: false,
		},
	}

	rules := m.getKubernetesFirewallRules()

	// Check for Kubelet rule
	hasKubeletRule := false
	for _, rule := range rules {
		if rule.Port == "10250" {
			hasKubeletRule = true
			assert.Equal(t, "tcp", rule.Protocol)
			assert.Contains(t, rule.Description, "Kubelet")
			break
		}
	}
	assert.True(t, hasKubeletRule, "Should have Kubelet rule")
}

// TestManager_GetKubernetesFirewallRules_NetworkingRules tests CNI networking rules
func TestManager_GetKubernetesFirewallRules_NetworkingRules(t *testing.T) {
	m := &Manager{
		config: &config.NetworkConfig{
			EnableNodePorts: false,
		},
	}

	rules := m.getKubernetesFirewallRules()

	// Check for Flannel VXLAN rule
	hasFlannelRule := false
	for _, rule := range rules {
		if rule.Port == "8472" && rule.Protocol == "udp" {
			hasFlannelRule = true
			assert.Contains(t, rule.Description, "Flannel")
			break
		}
	}
	assert.True(t, hasFlannelRule, "Should have Flannel VXLAN rule")

	// Check for Calico BGP rule
	hasCalicoRule := false
	for _, rule := range rules {
		if rule.Port == "179" && rule.Protocol == "tcp" {
			hasCalicoRule = true
			assert.Contains(t, rule.Description, "Calico")
			break
		}
	}
	assert.True(t, hasCalicoRule, "Should have Calico BGP rule")
}

// TestManager_GetKubernetesFirewallRules_PrivateNetworkSources tests source restrictions
func TestManager_GetKubernetesFirewallRules_PrivateNetworkSources(t *testing.T) {
	m := &Manager{
		config: &config.NetworkConfig{
			EnableNodePorts: false,
		},
	}

	rules := m.getKubernetesFirewallRules()

	// Verify that most rules restrict to private networks
	for _, rule := range rules {
		if rule.Port == "6443" || rule.Port == "2379-2380" || rule.Port == "10250" {
			// These should only allow private network sources
			assert.NotEmpty(t, rule.Source, "Critical Kubernetes ports should have source restrictions")

			// Check that it includes private network ranges
			hasPrivateRange := false
			for _, source := range rule.Source {
				if source == "10.0.0.0/8" || source == "172.16.0.0/12" || source == "192.168.0.0/16" {
					hasPrivateRange = true
					break
				}
			}
			assert.True(t, hasPrivateRange, "Should include private network ranges for port %s", rule.Port)
		}
	}
}

// TestManager_GetKubernetesFirewallRules_ControlPlaneComponents tests control plane component rules
func TestManager_GetKubernetesFirewallRules_ControlPlaneComponents(t *testing.T) {
	m := &Manager{
		config: &config.NetworkConfig{
			EnableNodePorts: false,
		},
	}

	rules := m.getKubernetesFirewallRules()

	// Check for kube-scheduler
	hasSchedulerRule := false
	for _, rule := range rules {
		if rule.Port == "10251" {
			hasSchedulerRule = true
			assert.Equal(t, "tcp", rule.Protocol)
			assert.Contains(t, rule.Description, "scheduler")
			break
		}
	}
	assert.True(t, hasSchedulerRule, "Should have kube-scheduler rule")

	// Check for kube-controller-manager
	hasControllerRule := false
	for _, rule := range rules {
		if rule.Port == "10252" {
			hasControllerRule = true
			assert.Equal(t, "tcp", rule.Protocol)
			assert.Contains(t, rule.Description, "controller")
			break
		}
	}
	assert.True(t, hasControllerRule, "Should have kube-controller-manager rule")
}

// TestManager_GetKubernetesFirewallRules_RuleProtocols tests protocol settings
func TestManager_GetKubernetesFirewallRules_RuleProtocols(t *testing.T) {
	m := &Manager{
		config: &config.NetworkConfig{
			EnableNodePorts: false,
		},
	}

	rules := m.getKubernetesFirewallRules()

	tcpCount := 0
	udpCount := 0

	for _, rule := range rules {
		assert.NotEmpty(t, rule.Protocol, "All rules should have a protocol")

		if rule.Protocol == "tcp" {
			tcpCount++
		} else if rule.Protocol == "udp" {
			udpCount++
		}
	}

	assert.Greater(t, tcpCount, 0, "Should have TCP rules")
	assert.Greater(t, udpCount, 0, "Should have UDP rules")
}

// TestManager_GetKubernetesFirewallRules_RuleDescriptions tests that all rules have descriptions
func TestManager_GetKubernetesFirewallRules_RuleDescriptions(t *testing.T) {
	m := &Manager{
		config: &config.NetworkConfig{
			EnableNodePorts: false,
		},
	}

	rules := m.getKubernetesFirewallRules()

	for _, rule := range rules {
		assert.NotEmpty(t, rule.Description, "Rule for port %s should have a description", rule.Port)
		assert.Greater(t, len(rule.Description), 3, "Description should be meaningful")
	}
}

// TestManager_GetKubernetesFirewallRules_Consistency tests consistency across calls
func TestManager_GetKubernetesFirewallRules_Consistency(t *testing.T) {
	m := &Manager{
		config: &config.NetworkConfig{
			EnableNodePorts: true,
		},
	}

	rules1 := m.getKubernetesFirewallRules()
	rules2 := m.getKubernetesFirewallRules()

	assert.Equal(t, len(rules1), len(rules2), "Should return consistent number of rules")

	// Verify same ports appear in both calls
	ports1 := make(map[string]bool)
	ports2 := make(map[string]bool)

	for _, rule := range rules1 {
		ports1[rule.Port] = true
	}

	for _, rule := range rules2 {
		ports2[rule.Port] = true
	}

	assert.Equal(t, ports1, ports2, "Should have same ports across calls")
}

// TestManager_GetKubernetesFirewallRules_UniqueRules tests that rules don't conflict
func TestManager_GetKubernetesFirewallRules_UniqueRules(t *testing.T) {
	m := &Manager{
		config: &config.NetworkConfig{
			EnableNodePorts: true,
		},
	}

	rules := m.getKubernetesFirewallRules()

	// Check for duplicate port/protocol combinations
	seen := make(map[string]bool)

	for _, rule := range rules {
		key := rule.Protocol + ":" + rule.Port
		assert.False(t, seen[key], "Should not have duplicate rule for %s", key)
		seen[key] = true
	}
}

// TestManager_GetKubernetesFirewallRules_PortRanges tests port range formats
func TestManager_GetKubernetesFirewallRules_PortRanges(t *testing.T) {
	m := &Manager{
		config: &config.NetworkConfig{
			EnableNodePorts: true,
		},
	}

	rules := m.getKubernetesFirewallRules()

	for _, rule := range rules {
		assert.NotEmpty(t, rule.Port, "Port should not be empty")

		// Port should be either a single number or a range
		assert.Regexp(t, `^\d+(-\d+)?$`, rule.Port, "Port %s should be valid format", rule.Port)
	}
}

// TestManager_GetKubernetesFirewallRules_SourceFormats tests source CIDR formats
func TestManager_GetKubernetesFirewallRules_SourceFormats(t *testing.T) {
	m := &Manager{
		config: &config.NetworkConfig{
			EnableNodePorts: false,
		},
	}

	rules := m.getKubernetesFirewallRules()

	for _, rule := range rules {
		for _, source := range rule.Source {
			assert.NotEmpty(t, source, "Source should not be empty")
			// Should be a valid CIDR notation
			assert.Contains(t, source, "/", "Source %s should be in CIDR format", source)
		}
	}
}

// TestManager_GetKubernetesFirewallRules_EmptyConfig tests with minimal config
func TestManager_GetKubernetesFirewallRules_EmptyConfig(t *testing.T) {
	m := &Manager{
		config: &config.NetworkConfig{},
	}

	rules := m.getKubernetesFirewallRules()

	// Should still return basic Kubernetes rules
	assert.NotEmpty(t, rules)
	assert.Greater(t, len(rules), 3, "Should have basic Kubernetes rules even with empty config")
}

// TestManager_GetKubernetesFirewallRules_RequiredPorts tests that all essential ports are present
func TestManager_GetKubernetesFirewallRules_RequiredPorts(t *testing.T) {
	m := &Manager{
		config: &config.NetworkConfig{
			EnableNodePorts: false,
		},
	}

	rules := m.getKubernetesFirewallRules()

	requiredPorts := []string{"6443", "2379-2380", "10250", "10251", "10252"}

	for _, requiredPort := range requiredPorts {
		hasPort := false
		for _, rule := range rules {
			if rule.Port == requiredPort {
				hasPort = true
				break
			}
		}
		assert.True(t, hasPort, "Should have required Kubernetes port %s", requiredPort)
	}
}
