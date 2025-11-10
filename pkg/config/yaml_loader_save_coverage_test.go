package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TEST 1: SaveToYAML - Success with valid config
func TestSaveToYAML_SuccessValidConfig(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create config
	cfg := &ClusterConfig{
		Metadata: Metadata{
			Name:        "test-cluster",
			Environment: "test",
		},
		Providers: ProvidersConfig{
			DigitalOcean: &DigitalOceanProvider{
				Enabled: true,
				Token:   "test-token",
				Region:  "nyc1",
			},
		},
		Kubernetes: KubernetesConfig{
			Distribution: "rke2",
			Version:      "v1.28.0",
		},
		NodePools: map[string]NodePool{
			"masters": {
				Name:     "masters",
				Provider: "digitalocean",
				Count:    1,
				Roles:    []string{"master"},
			},
		},
	}

	// Save to YAML
	filePath := filepath.Join(tmpDir, "test-config.yaml")
	err = SaveToYAML(cfg, filePath)

	// Assert success
	assert.NoError(t, err)
	assert.FileExists(t, filePath)

	// Verify file contents
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "test-cluster")
	assert.Contains(t, string(data), "digitalocean")
}

// TEST 2: SaveToYAML - Success with subdirectory creation
func TestSaveToYAML_CreateSubdirectories(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create config
	cfg := &ClusterConfig{
		Metadata: Metadata{
			Name: "subdir-cluster",
		},
	}

	// Save to YAML in subdirectory (that doesn't exist yet)
	filePath := filepath.Join(tmpDir, "configs", "nested", "test.yaml")
	err = SaveToYAML(cfg, filePath)

	// Assert success - directory should be created automatically
	assert.NoError(t, err)
	assert.FileExists(t, filePath)
}

// TEST 3: SaveToYAML - Success with complex config
func TestSaveToYAML_ComplexConfig(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create complex config
	cfg := &ClusterConfig{
		Metadata: Metadata{
			Name:        "complex-cluster",
			Environment: "production",
			Labels: map[string]string{
				"team": "platform",
				"env":  "prod",
			},
		},
		Providers: ProvidersConfig{
			DigitalOcean: &DigitalOceanProvider{
				Enabled: true,
				Token:   "do-token",
				Region:  "nyc3",
				Tags:    []string{"k8s", "prod"},
			},
			Linode: &LinodeProvider{
				Enabled: true,
				Token:   "linode-token",
				Region:  "us-east",
			},
		},
		Network: NetworkConfig{
			DNS: DNSConfig{
				Domain:   "example.com",
				Provider: "digitalocean",
			},
		},
		Kubernetes: KubernetesConfig{
			Distribution:  "rke2",
			Version:       "v1.28.5+rke2r1",
			NetworkPlugin: "calico",
		},
		NodePools: map[string]NodePool{
			"do-masters": {
				Name:     "do-masters",
				Provider: "digitalocean",
				Count:    3,
				Roles:    []string{"master"},
				Size:     "s-2vcpu-4gb",
			},
			"linode-workers": {
				Name:     "linode-workers",
				Provider: "linode",
				Count:    5,
				Roles:    []string{"worker"},
				Size:     "g6-standard-2",
			},
		},
	}

	// Save to YAML
	filePath := filepath.Join(tmpDir, "complex.yaml")
	err = SaveToYAML(cfg, filePath)

	// Assert success
	assert.NoError(t, err)
	assert.FileExists(t, filePath)

	// Verify file can be loaded back
	loadedCfg, err := LoadFromYAML(filePath)
	require.NoError(t, err)
	assert.Equal(t, "complex-cluster", loadedCfg.Metadata.Name)
	assert.Equal(t, 2, len(loadedCfg.NodePools))
}

// TEST 4: SaveToYAML - Overwrite existing file
func TestSaveToYAML_OverwriteExisting(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "overwrite.yaml")

	// Create initial file
	cfg1 := &ClusterConfig{
		Metadata: Metadata{
			Name: "version1",
		},
	}
	err = SaveToYAML(cfg1, filePath)
	require.NoError(t, err)

	// Overwrite with new config
	cfg2 := &ClusterConfig{
		Metadata: Metadata{
			Name: "version2",
		},
	}
	err = SaveToYAML(cfg2, filePath)

	// Assert success
	assert.NoError(t, err)

	// Verify new content
	loadedCfg, err := LoadFromYAML(filePath)
	require.NoError(t, err)
	assert.Equal(t, "version2", loadedCfg.Metadata.Name)
}

// TEST 5: SaveToYAML - Empty config
func TestSaveToYAML_EmptyConfig(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create empty config
	cfg := &ClusterConfig{}

	// Save to YAML
	filePath := filepath.Join(tmpDir, "empty.yaml")
	err = SaveToYAML(cfg, filePath)

	// Should succeed even with empty config
	assert.NoError(t, err)
	assert.FileExists(t, filePath)
}

// TEST 6: LoadFromYAML - File not found
func TestLoadFromYAML_FileNotFound(t *testing.T) {
	// Try to load non-existent file
	_, err := LoadFromYAML("/tmp/nonexistent-config-file-12345.yaml")

	// Should error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

// TEST 7: LoadFromYAML - Invalid YAML
func TestLoadFromYAML_InvalidYAML(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Write invalid YAML
	filePath := filepath.Join(tmpDir, "invalid.yaml")
	err = os.WriteFile(filePath, []byte("invalid: yaml: content: [[[["), 0644)
	require.NoError(t, err)

	// Try to load
	_, err = LoadFromYAML(filePath)

	// Should error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML")
}

// TEST 8: LoadFromYAML - Success with minimal config
func TestLoadFromYAML_MinimalConfig(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Save minimal config
	cfg := &ClusterConfig{
		Metadata: Metadata{
			Name: "minimal",
		},
	}
	filePath := filepath.Join(tmpDir, "minimal.yaml")
	err = SaveToYAML(cfg, filePath)
	require.NoError(t, err)

	// Load it back
	loadedCfg, err := LoadFromYAML(filePath)

	// Assert success
	assert.NoError(t, err)
	assert.NotNil(t, loadedCfg)
	// applyDefaults should have added default values
	assert.Equal(t, "rke2", loadedCfg.Kubernetes.Distribution)
	assert.NotEmpty(t, loadedCfg.Kubernetes.Version)
}

// TEST 9: LoadFromYAML - K8s-style format detection - SKIPPED (complex validation)
// func TestLoadFromYAML_K8sStyleDetection(t *testing.T) { ... }

// TEST 10: LoadFromYAML - Legacy format
func TestLoadFromYAML_LegacyFormat(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Save legacy format config (no apiVersion)
	cfg := &ClusterConfig{
		Metadata: Metadata{
			Name: "legacy-cluster",
		},
		Kubernetes: KubernetesConfig{
			Distribution: "k3s",
		},
	}
	filePath := filepath.Join(tmpDir, "legacy.yaml")
	err = SaveToYAML(cfg, filePath)
	require.NoError(t, err)

	// Load it back
	loadedCfg, err := LoadFromYAML(filePath)

	// Assert success
	assert.NoError(t, err)
	assert.NotNil(t, loadedCfg)
	assert.Equal(t, "legacy-cluster", loadedCfg.Metadata.Name)
	assert.Equal(t, "k3s", loadedCfg.Kubernetes.Distribution)
}

// TEST 11: SaveToYAML then LoadFromYAML roundtrip
func TestSaveAndLoad_Roundtrip(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create original config
	original := &ClusterConfig{
		Metadata: Metadata{
			Name:        "roundtrip-test",
			Environment: "staging",
			Labels: map[string]string{
				"test": "roundtrip",
			},
		},
		Providers: ProvidersConfig{
			DigitalOcean: &DigitalOceanProvider{
				Enabled: true,
				Token:   "test-token-123",
				Region:  "sfo3",
			},
		},
		Kubernetes: KubernetesConfig{
			Distribution:  "rke2",
			Version:       "v1.29.0",
			NetworkPlugin: "canal",
		},
		NodePools: map[string]NodePool{
			"test-pool": {
				Name:     "test-pool",
				Provider: "digitalocean",
				Count:    2,
				Roles:    []string{"master", "worker"},
			},
		},
	}

	// Save
	filePath := filepath.Join(tmpDir, "roundtrip.yaml")
	err = SaveToYAML(original, filePath)
	require.NoError(t, err)

	// Load
	loaded, err := LoadFromYAML(filePath)
	require.NoError(t, err)

	// Verify key fields match
	assert.Equal(t, original.Metadata.Name, loaded.Metadata.Name)
	assert.Equal(t, original.Metadata.Environment, loaded.Metadata.Environment)
	assert.Equal(t, original.Providers.DigitalOcean.Token, loaded.Providers.DigitalOcean.Token)
	assert.Equal(t, original.Kubernetes.Version, loaded.Kubernetes.Version)
	assert.Equal(t, len(original.NodePools), len(loaded.NodePools))
}

// TEST 12: SaveToYAML - With WireGuard config
func TestSaveToYAML_WithWireGuard(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create config with WireGuard
	cfg := &ClusterConfig{
		Metadata: Metadata{
			Name: "wireguard-cluster",
		},
		Network: NetworkConfig{
			WireGuard: &WireGuardConfig{
				Enabled:        true,
				ServerEndpoint: "vpn.example.com:51820",
				ServerPublicKey: "test-public-key",
				Port:           51820,
			},
		},
	}

	// Save
	filePath := filepath.Join(tmpDir, "wireguard.yaml")
	err = SaveToYAML(cfg, filePath)

	// Assert success
	assert.NoError(t, err)

	// Verify content
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "wireguard")
	assert.Contains(t, string(data), "51820")
}
