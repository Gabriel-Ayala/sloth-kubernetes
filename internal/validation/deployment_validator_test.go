package validation

import (
	"os"
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestValidateForDeployment_Complete(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{
			Name:    "test-cluster",
			Version: "1.0.0",
		},
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "fake-token",
			},
		},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Provider: "digitalocean",
				Count:    3,
				Size:     "s-2vcpu-4gb",
				Region:   "nyc3",
				Roles:    []string{"master"},
			},
		},
	}

	// Should validate all aspects
	// (will fail on API token validation, but that's expected in tests)
	err := ValidateForDeployment(cfg)
	// We expect it to fail because tokens aren't real
	assert.Error(t, err)
}

func TestValidateAPITokensPresence_DigitalOcean(t *testing.T) {
	// Test with token in config
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "test-token",
			},
		},
	}

	err := ValidateAPITokensPresence(cfg)
	assert.NoError(t, err, "Should pass when token is in config")

	// Test with missing token
	cfg.Providers.DigitalOcean.Token = ""
	os.Unsetenv("DIGITALOCEAN_TOKEN")

	err = ValidateAPITokensPresence(cfg)
	assert.Error(t, err, "Should fail when token is missing")
	assert.Contains(t, err.Error(), "DigitalOcean token is required")
}

func TestValidateAPITokensPresence_Linode(t *testing.T) {
	// Test with token in config
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			Linode: &config.LinodeProvider{
				Enabled: true,
				Token:   "test-token",
			},
		},
	}

	err := ValidateAPITokensPresence(cfg)
	assert.NoError(t, err, "Should pass when token is in config")

	// Test with missing token
	cfg.Providers.Linode.Token = ""
	os.Unsetenv("LINODE_TOKEN")

	err = ValidateAPITokensPresence(cfg)
	assert.Error(t, err, "Should fail when token is missing")
	assert.Contains(t, err.Error(), "Linode token is required")
}

func TestValidateAPITokensPresence_Azure(t *testing.T) {
	// Azure should not require explicit token validation
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			Azure: &config.AzureProvider{
				Enabled:       true,
				Location:      "eastus",
				ResourceGroup: "test-rg",
			},
		},
	}

	err := ValidateAPITokensPresence(cfg)
	assert.NoError(t, err, "Azure should pass even without explicit credentials")
}

func TestValidateAPITokensPresence_MultipleProviders(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "",
			},
			Linode: &config.LinodeProvider{
				Enabled: true,
				Token:   "",
			},
		},
	}

	os.Unsetenv("DIGITALOCEAN_TOKEN")
	os.Unsetenv("LINODE_TOKEN")

	err := ValidateAPITokensPresence(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DigitalOcean")
	assert.Contains(t, err.Error(), "Linode")
}

func TestValidateNodePools_ValidPool(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "test",
			},
		},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Provider: "digitalocean",
				Count:    3,
				Size:     "s-2vcpu-4gb",
				Region:   "nyc3",
				Roles:    []string{"master"},
			},
		},
	}

	err := ValidateNodePools(cfg)
	assert.NoError(t, err, "Valid node pool should pass")
}

func TestValidateNodePools_InvalidProvider(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: false,
			},
		},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Provider: "digitalocean",
				Count:    3,
				Size:     "s-2vcpu-4gb",
				Region:   "nyc3",
				Roles:    []string{"master"},
			},
		},
	}

	err := ValidateNodePools(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DigitalOcean but provider is not enabled")
}

func TestValidateNodePools_InvalidCount(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "test",
			},
		},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Provider: "digitalocean",
				Count:    0,
				Size:     "s-2vcpu-4gb",
				Region:   "nyc3",
				Roles:    []string{"master"},
			},
		},
	}

	err := ValidateNodePools(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid count")
}

func TestValidateNodePools_MissingSize(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "test",
			},
		},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Provider: "digitalocean",
				Count:    3,
				Size:     "",
				Region:   "nyc3",
				Roles:    []string{"master"},
			},
		},
	}

	err := ValidateNodePools(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no size specified")
}

func TestValidateNodePools_MissingRegion(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "test",
			},
		},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Provider: "digitalocean",
				Count:    3,
				Size:     "s-2vcpu-4gb",
				Region:   "",
				Roles:    []string{"master"},
			},
		},
	}

	err := ValidateNodePools(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no region specified")
}

func TestValidateNodePools_NoRoles(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "test",
			},
		},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Provider: "digitalocean",
				Count:    3,
				Size:     "s-2vcpu-4gb",
				Region:   "nyc3",
				Roles:    []string{},
			},
		},
	}

	err := ValidateNodePools(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no roles specified")
}

func TestValidateNodePools_InvalidRole(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "test",
			},
		},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Provider: "digitalocean",
				Count:    3,
				Size:     "s-2vcpu-4gb",
				Region:   "nyc3",
				Roles:    []string{"invalid-role"},
			},
		},
	}

	err := ValidateNodePools(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid role")
}

func TestValidateNodePools_ValidRoles(t *testing.T) {
	validRoles := [][]string{
		{"master"},
		{"controlplane"},
		{"worker"},
		{"etcd"},
		{"master", "etcd"},
		{"controlplane", "etcd"},
	}

	for _, roles := range validRoles {
		cfg := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test",
				},
			},
			NodePools: map[string]config.NodePool{
				"test-pool": {
					Name:     "test-pool",
					Provider: "digitalocean",
					Count:    1,
					Size:     "s-2vcpu-4gb",
					Region:   "nyc3",
					Roles:    roles,
				},
			},
		}

		err := ValidateNodePools(cfg)
		assert.NoError(t, err, "Roles %v should be valid", roles)
	}
}

func TestValidateNodePools_IndividualNodes(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			Linode: &config.LinodeProvider{
				Enabled: true,
				Token:   "test",
			},
		},
		Nodes: []config.NodeConfig{
			{
				Provider: "linode",
				Size:     "g6-standard-2",
				Region:   "us-east",
				Roles:    []string{"master"},
			},
		},
	}

	err := ValidateNodePools(cfg)
	assert.NoError(t, err, "Valid individual node should pass")
}

func TestValidateNodePools_IndividualNode_InvalidProvider(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			Linode: &config.LinodeProvider{
				Enabled: false,
			},
		},
		Nodes: []config.NodeConfig{
			{
				Provider: "linode",
				Size:     "g6-standard-2",
				Region:   "us-east",
				Roles:    []string{"master"},
			},
		},
	}

	err := ValidateNodePools(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Linode but provider is not enabled")
}

func TestValidateNetworkingConfig_ValidDNS(t *testing.T) {
	cfg := &config.ClusterConfig{
		Network: config.NetworkConfig{
			DNS: config.DNSConfig{
				Domain:   "example.com",
				Provider: "digitalocean",
			},
		},
	}

	err := ValidateNetworkingConfig(cfg)
	assert.NoError(t, err)
}

func TestValidateNetworkingConfig_InvalidDNSDomain(t *testing.T) {
	cfg := &config.ClusterConfig{
		Network: config.NetworkConfig{
			DNS: config.DNSConfig{
				Domain:   "invalid domain",
				Provider: "digitalocean",
			},
		},
	}

	err := ValidateNetworkingConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid DNS domain format")
}

func TestValidateNetworkingConfig_InvalidDNSProvider(t *testing.T) {
	cfg := &config.ClusterConfig{
		Network: config.NetworkConfig{
			DNS: config.DNSConfig{
				Domain:   "example.com",
				Provider: "invalid-provider",
			},
		},
	}

	err := ValidateNetworkingConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid DNS provider")
}

func TestValidateNetworkingConfig_ValidCIDRs(t *testing.T) {
	cfg := &config.ClusterConfig{
		Kubernetes: config.KubernetesConfig{
			PodCIDR:     "10.244.0.0/16",
			ServiceCIDR: "10.96.0.0/12",
		},
	}

	err := ValidateNetworkingConfig(cfg)
	assert.NoError(t, err)
}

func TestValidateNetworkingConfig_InvalidPodCIDR(t *testing.T) {
	cfg := &config.ClusterConfig{
		Kubernetes: config.KubernetesConfig{
			PodCIDR: "invalid-cidr",
		},
	}

	err := ValidateNetworkingConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Pod CIDR")
}

func TestValidateNetworkingConfig_InvalidServiceCIDR(t *testing.T) {
	cfg := &config.ClusterConfig{
		Kubernetes: config.KubernetesConfig{
			ServiceCIDR: "10.96.0.0/999",
		},
	}

	err := ValidateNetworkingConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Service CIDR")
}

func TestValidateCIDR_Valid(t *testing.T) {
	validCIDRs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"10.244.0.0/16",
		"10.96.0.0/12",
	}

	for _, cidr := range validCIDRs {
		err := validateCIDR(cidr)
		assert.NoError(t, err, "CIDR %s should be valid", cidr)
	}
}

func TestValidateCIDR_Invalid(t *testing.T) {
	invalidCIDRs := []string{
		"invalid",
		"10.0.0.0",
		"10.0.0.0/",
		"not-a-cidr",
		"",
	}

	for _, cidr := range invalidCIDRs {
		err := validateCIDR(cidr)
		assert.Error(t, err, "CIDR %s should be invalid", cidr)
	}
}

func TestValidateSSHConfig(t *testing.T) {
	cfg := &config.ClusterConfig{}
	err := ValidateSSHConfig(cfg)
	assert.NoError(t, err, "SSH config validation should always pass")
}

func TestValidateResourceSizes_SmallMaster(t *testing.T) {
	cfg := &config.ClusterConfig{
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:  "masters",
				Count: 3,
				Size:  "s-1vcpu-1gb",
				Roles: []string{"master"},
			},
		},
	}

	// Should not error, just warn
	err := ValidateResourceSizes(cfg)
	assert.NoError(t, err, "Resource validation should not return error, only warnings")
}

func TestValidateResourceSizes_LargeMaster(t *testing.T) {
	cfg := &config.ClusterConfig{
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:  "masters",
				Count: 3,
				Size:  "s-4vcpu-8gb",
				Roles: []string{"master"},
			},
		},
	}

	err := ValidateResourceSizes(cfg)
	assert.NoError(t, err)
}

func TestValidateResourceSizes_WorkerNodes(t *testing.T) {
	cfg := &config.ClusterConfig{
		NodePools: map[string]config.NodePool{
			"workers": {
				Name:  "workers",
				Count: 5,
				Size:  "s-1vcpu-1gb",
				Roles: []string{"worker"},
			},
		},
	}

	// Workers can be small, no warnings
	err := ValidateResourceSizes(cfg)
	assert.NoError(t, err)
}

func TestValidateNodePools_UnknownProvider(t *testing.T) {
	cfg := &config.ClusterConfig{
		NodePools: map[string]config.NodePool{
			"test": {
				Name:     "test",
				Provider: "unknown-provider",
				Count:    1,
				Size:     "small",
				Region:   "region",
				Roles:    []string{"worker"},
			},
		},
	}

	err := ValidateNodePools(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid provider")
}

func TestValidateNodePools_AzureProvider(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			Azure: &config.AzureProvider{
				Enabled:       true,
				Location:      "eastus",
				ResourceGroup: "test-rg",
			},
		},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Provider: "azure",
				Count:    3,
				Size:     "Standard_D2s_v3",
				Region:   "eastus",
				Roles:    []string{"master"},
			},
		},
	}

	err := ValidateNodePools(cfg)
	assert.NoError(t, err)
}

// Tests for ValidateAPITokensWithProviders
func TestValidateAPITokensWithProviders_DigitalOcean(t *testing.T) {
	t.Skip("Skipping - requires real DigitalOcean API access")

	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "invalid-token",
			},
		},
	}

	err := ValidateAPITokensWithProviders(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DigitalOcean token validation failed")
}

func TestValidateAPITokensWithProviders_Linode(t *testing.T) {
	t.Skip("Skipping - requires real Linode API access")

	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			Linode: &config.LinodeProvider{
				Enabled: true,
				Token:   "invalid-token",
			},
		},
	}

	err := ValidateAPITokensWithProviders(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Linode token validation failed")
}

func TestValidateAPITokensWithProviders_MultipleProviders(t *testing.T) {
	t.Skip("Skipping - requires real API access")

	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "invalid-do-token",
			},
			Linode: &config.LinodeProvider{
				Enabled: true,
				Token:   "invalid-linode-token",
			},
		},
	}

	err := ValidateAPITokensWithProviders(cfg)
	assert.Error(t, err)
}

func TestValidateAPITokensWithProviders_NoTokens(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "",
			},
		},
	}

	// Should not error when token is empty (will be caught by ValidateAPITokensPresence)
	err := ValidateAPITokensWithProviders(cfg)
	assert.NoError(t, err)
}

func TestValidateAPITokensWithProviders_DisabledProvider(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: false,
				Token:   "some-token",
			},
		},
	}

	// Should not validate disabled providers
	err := ValidateAPITokensWithProviders(cfg)
	assert.NoError(t, err)
}

func TestValidateAPITokensWithProviders_TokenFromEnv(t *testing.T) {
	t.Skip("Skipping - requires real API access")

	// Set invalid token in env
	os.Setenv("DIGITALOCEAN_TOKEN", "invalid-env-token")
	defer os.Unsetenv("DIGITALOCEAN_TOKEN")

	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "", // Should use env var
			},
		},
	}

	err := ValidateAPITokensWithProviders(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DigitalOcean token validation failed")
}

// Tests for validateDigitalOceanToken
func TestValidateDigitalOceanToken_Invalid(t *testing.T) {
	t.Skip("Skipping - requires real DigitalOcean API access")

	err := validateDigitalOceanToken("invalid-token-12345")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token or API error")
}

func TestValidateDigitalOceanToken_Empty(t *testing.T) {
	t.Skip("Skipping - requires real DigitalOcean API access")

	err := validateDigitalOceanToken("")
	assert.Error(t, err)
}

// Tests for validateLinodeToken
func TestValidateLinodeToken_Invalid(t *testing.T) {
	t.Skip("Skipping - requires real Linode API access")

	err := validateLinodeToken("invalid-token-12345")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token or API error")
}

func TestValidateLinodeToken_Empty(t *testing.T) {
	t.Skip("Skipping - requires real Linode API access")

	err := validateLinodeToken("")
	assert.Error(t, err)
}

// Additional tests for edge cases
func TestValidateAPITokensWithProviders_NilProvider(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: nil,
			Linode:       nil,
		},
	}

	err := ValidateAPITokensWithProviders(cfg)
	assert.NoError(t, err)
}

func TestValidateAPITokensWithProviders_EmptyConfig(t *testing.T) {
	cfg := &config.ClusterConfig{}

	err := ValidateAPITokensWithProviders(cfg)
	assert.NoError(t, err)
}
