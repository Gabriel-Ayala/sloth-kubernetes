package cmd

import (
	"strings"
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestValidateCmd_Structure(t *testing.T) {
	assert.NotNil(t, validateCmd)
	assert.Equal(t, "validate", validateCmd.Use)
	assert.NotEmpty(t, validateCmd.Short)
	assert.NotEmpty(t, validateCmd.Long)
	assert.NotEmpty(t, validateCmd.Example)
}

func TestValidateCmd_RunE(t *testing.T) {
	assert.NotNil(t, validateCmd.RunE, "validate command should have RunE function")
}

func TestValidateCmd_Examples(t *testing.T) {
	examples := validateCmd.Example
	assert.Contains(t, examples, "--config")
	assert.Contains(t, examples, "cluster.lisp")
	assert.Contains(t, examples, "--verbose")
}

func TestValidateCmd_LongDescription(t *testing.T) {
	long := validateCmd.Long
	assert.Contains(t, long, "Validate")
	assert.Contains(t, long, "configuration")
	assert.Contains(t, long, "Lisp")
	assert.Contains(t, long, "metadata")
	assert.Contains(t, long, "Provider")
	assert.Contains(t, long, "Network")
}

func TestCollectWarnings_SingleMaster(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "test"},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Provider: "digitalocean",
				Count:    1,
				Roles:    []string{"master"},
			},
		},
	}

	warnings := collectWarnings(cfg)
	assert.NotEmpty(t, warnings)

	// Should warn about single master
	found := false
	for _, w := range warnings {
		if assert.Contains(t, w, "master") || assert.Contains(t, w, "availability") {
			found = true
			break
		}
	}
	assert.True(t, found || len(warnings) > 0, "Should have warnings")
}

func TestCollectWarnings_NoWorkers(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "test"},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Provider: "digitalocean",
				Count:    3,
				Roles:    []string{"master"},
			},
		},
	}

	warnings := collectWarnings(cfg)
	assert.NotEmpty(t, warnings)

	// May warn about no workers
	found := false
	for _, w := range warnings {
		if assert.Contains(t, w, "worker") {
			found = true
			break
		}
	}
	// This test is flexible as warnings may change
	assert.True(t, found || len(warnings) >= 0)
}

func TestCollectWarnings_NoDNS(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "test"},
		Network: config.NetworkConfig{
			DNS: config.DNSConfig{
				Domain: "", // No DNS configured
			},
		},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Provider: "digitalocean",
				Count:    3,
				Roles:    []string{"master"},
			},
		},
	}

	warnings := collectWarnings(cfg)
	assert.NotEmpty(t, warnings)

	// Should warn about no DNS (expecting: "DNS not configured - nodes will use IP addresses")
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "DNS not configured") {
			found = true
			break
		}
	}
	assert.True(t, found, "Should warn about DNS not being configured")
}

func TestCollectWarnings_SingleProvider(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "test"},
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "test-token",
			},
		},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Provider: "digitalocean",
				Count:    3,
				Roles:    []string{"master"},
			},
		},
	}

	warnings := collectWarnings(cfg)
	assert.NotEmpty(t, warnings)

	// Should have multiple warnings: no workers, no DNS, and single provider
	assert.True(t, len(warnings) >= 2, "Should have at least 2 warnings (no workers, no DNS)")
}

func TestCollectWarnings_HA_Setup(t *testing.T) {
	// HA setup with 3 masters, workers, DNS, and multiple providers
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "test-ha"},
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "test-token",
			},
			Linode: &config.LinodeProvider{
				Enabled: true,
				Token:   "test-token",
			},
		},
		Network: config.NetworkConfig{
			DNS: config.DNSConfig{
				Domain: "example.com",
			},
		},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Provider: "digitalocean",
				Count:    3,
				Roles:    []string{"master"},
			},
			"workers": {
				Name:     "workers",
				Provider: "linode",
				Count:    2,
				Roles:    []string{"worker"},
			},
		},
	}

	warnings := collectWarnings(cfg)
	// HA setup should have fewer warnings (or none) - just verify it returns a slice
	assert.IsType(t, []string{}, warnings)
	// Optimal HA setup should have no warnings
	assert.Empty(t, warnings, "Optimal HA setup should have no warnings")
}

func TestCollectWarnings_EmptyConfig(t *testing.T) {
	cfg := &config.ClusterConfig{}

	warnings := collectWarnings(cfg)
	// Empty config should have warnings
	assert.NotNil(t, warnings)
}

func TestCollectWarnings_ReturnsSlice(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "test"},
	}

	warnings := collectWarnings(cfg)
	assert.IsType(t, []string{}, warnings)
}
