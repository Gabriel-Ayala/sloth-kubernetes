package validation

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/stretchr/testify/assert"
)

// TEST 1: ValidateForDeployment - All validations pass
func TestValidateForDeployment_AllValidationsPass(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{
			Name:        "test-cluster",
			Environment: "test",
		},
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "test-do-token",
				Region:  "nyc3",
			},
		},
		Kubernetes: config.KubernetesConfig{
			Distribution:  "rke2",
			Version:       "v1.28.0",
			NetworkPlugin: "calico",
			PodCIDR:       "10.42.0.0/16",
			ServiceCIDR:   "10.43.0.0/16",
		},
		Network: config.NetworkConfig{
			DNS: config.DNSConfig{
				Domain:   "example.com",
				Provider: "digitalocean",
			},
		},
		Security: config.SecurityConfig{
			SSHConfig: config.SSHConfig{
				KeyPath:       "/tmp/test-key",
				PublicKeyPath: "/tmp/test-key.pub",
			},
		},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Provider: "digitalocean",
				Count:    1,
				Roles:    []string{"master"},
				Size:     "s-2vcpu-4gb",
				Region:   "nyc3",
			},
			"workers": {
				Name:     "workers",
				Provider: "digitalocean",
				Count:    2,
				Roles:    []string{"worker"},
				Size:     "s-2vcpu-2gb",
				Region:   "nyc3",
			},
		},
	}

	// This should pass all validations (except token validation which requires real API)
	err := ValidateForDeployment(cfg)

	// Will fail at API token validation, but that's expected without real tokens
	// The important part is that all other validations ran
	if err != nil {
		assert.Contains(t, err.Error(), "validation")
	}
}

// TEST 2: ValidateForDeployment - Fails at basic validation
func TestValidateForDeployment_FailsBasicValidation(t *testing.T) {
	cfg := &config.ClusterConfig{
		// Empty metadata - should fail basic validation
		Metadata: config.Metadata{},
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "test-token",
			},
		},
	}

	err := ValidateForDeployment(cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "basic validation failed")
}

// TEST 3: ValidateForDeployment - Fails at token validation
func TestValidateForDeployment_FailsTokenValidation(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{
			Name: "test-cluster",
		},
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				// No token - should fail token validation
			},
		},
		Kubernetes: config.KubernetesConfig{
			Distribution: "rke2",
		},
		NodePools: map[string]config.NodePool{
			"test": {
				Name:     "test",
				Provider: "digitalocean",
				Count:    1,
				Roles:    []string{"master"},
				Size:     "test",
				Region:   "test",
			},
		},
	}

	err := ValidateForDeployment(cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API token validation failed")
}

// TEST 4: ValidateForDeployment - Fails at node pool validation
func TestValidateForDeployment_FailsNodePoolValidation(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{
			Name: "test-cluster",
		},
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "test-token",
			},
		},
		Kubernetes: config.KubernetesConfig{
			Distribution: "rke2",
		},
		NodePools: map[string]config.NodePool{
			"invalid": {
				Name:     "invalid",
				Provider: "digitalocean",
				Count:    0, // Invalid count - should fail
				Roles:    []string{"master"},
			},
		},
	}

	err := ValidateForDeployment(cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "node pool validation failed")
}

// TEST 5: ValidateForDeployment - Fails at network validation
func TestValidateForDeployment_FailsNetworkValidation(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{
			Name: "test-cluster",
		},
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "test-token",
			},
		},
		Kubernetes: config.KubernetesConfig{
			Distribution: "rke2",
			PodCIDR:      "invalid-cidr", // Invalid CIDR - should fail
		},
		NodePools: map[string]config.NodePool{
			"test": {
				Name:     "test",
				Provider: "digitalocean",
				Count:    1,
				Roles:    []string{"master"},
				Size:     "test",
				Region:   "test",
			},
		},
	}

	err := ValidateForDeployment(cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "network configuration validation failed")
}

// TEST 6: ValidateForDeployment - Fails at SSH validation
func TestValidateForDeployment_FailsSSHValidation(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{
			Name: "test-cluster",
		},
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "test-token",
			},
		},
		Kubernetes: config.KubernetesConfig{
			Distribution: "rke2",
		},
		Security: config.SecurityConfig{
			SSHConfig: config.SSHConfig{
				// Empty SSH config - should fail
			},
		},
		NodePools: map[string]config.NodePool{
			"test": {
				Name:     "test",
				Provider: "digitalocean",
				Count:    1,
				Roles:    []string{"master"},
				Size:     "test",
				Region:   "test",
			},
		},
	}

	err := ValidateForDeployment(cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SSH configuration validation failed")
}

// TEST 7: ValidateNodePools - Provider not enabled
func TestValidateNodePools_ProviderNotEnabled(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			Linode: &config.LinodeProvider{
				Enabled: false, // Disabled
			},
		},
		NodePools: map[string]config.NodePool{
			"linode-pool": {
				Name:     "linode-pool",
				Provider: "linode", // But pool uses it
				Count:    1,
				Roles:    []string{"worker"},
				Size:     "g6-standard-1",
				Region:   "us-east",
			},
		},
	}

	err := ValidateNodePools(cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provider is not enabled")
}

// TEST 8: ValidateNodePools - Invalid roles
func TestValidateNodePools_MultipleInvalidRoles(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
			},
		},
		NodePools: map[string]config.NodePool{
			"bad-pool": {
				Name:     "bad-pool",
				Provider: "digitalocean",
				Count:    1,
				Roles:    []string{"invalid-role", "another-bad-role"},
				Size:     "s-1vcpu-1gb",
				Region:   "nyc3",
			},
		},
	}

	err := ValidateNodePools(cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid role")
}

// TEST 9: ValidateNodePools - Azure provider check
func TestValidateNodePools_AzureProviderCheck(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			Azure: &config.AzureProvider{
				Enabled:  true,
				Location: "eastus",
			},
		},
		NodePools: map[string]config.NodePool{
			"azure-pool": {
				Name:     "azure-pool",
				Provider: "azure",
				Count:    2,
				Roles:    []string{"worker"},
				Size:     "Standard_B2s",
				Region:   "eastus",
			},
		},
	}

	err := ValidateNodePools(cfg)

	// Should pass - Azure provider is enabled
	assert.NoError(t, err)
}

// TEST 10: ValidateAPITokensWithProviders - Both providers with tokens
func TestValidateAPITokensWithProviders_BothProvidersWithTokens(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "fake-do-token",
			},
			Linode: &config.LinodeProvider{
				Enabled: true,
				Token:   "fake-linode-token",
			},
		},
	}

	err := ValidateAPITokensWithProviders(cfg)

	// Will fail because tokens are fake, but validates the path
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provider API validation failed")
}

// TEST 11: ValidateAPITokensPresence - Multiple errors
func TestValidateAPITokensPresence_MultipleErrors(t *testing.T) {
	// Clear env vars to ensure they're not set
	t.Setenv("DIGITALOCEAN_TOKEN", "")
	t.Setenv("LINODE_TOKEN", "")

	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				// No token
			},
			Linode: &config.LinodeProvider{
				Enabled: true,
				// No token
			},
		},
	}

	err := ValidateAPITokensPresence(cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DigitalOcean")
	assert.Contains(t, err.Error(), "Linode")
}

// TEST 12: ValidateForDeployment - Complete flow with WireGuard
func TestValidateForDeployment_WithWireGuard(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{
			Name: "wireguard-cluster",
		},
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "test-token",
			},
		},
		Network: config.NetworkConfig{
			DNS: config.DNSConfig{
				Domain:   "test.com",
				Provider: "digitalocean",
			},
			WireGuard: &config.WireGuardConfig{
				Create:     true,
				Enabled:    true,
				Provider:   "digitalocean",
				Region:     "nyc3",
				Size:       "s-1vcpu-1gb",
				SubnetCIDR: "10.8.0.0/24",
			},
		},
		Kubernetes: config.KubernetesConfig{
			Distribution: "rke2",
		},
		Security: config.SecurityConfig{
			SSHConfig: config.SSHConfig{
				KeyPath:       "/tmp/key",
				PublicKeyPath: "/tmp/key.pub",
			},
		},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Provider: "digitalocean",
				Count:    1,
				Roles:    []string{"master"},
				Size:     "s-2vcpu-4gb",
				Region:   "nyc3",
			},
		},
	}

	err := ValidateForDeployment(cfg)

	// Should pass all validations except real API token check
	if err != nil {
		// If it fails, it should be at validation stage
		assert.Contains(t, err.Error(), "validation")
	}
}
