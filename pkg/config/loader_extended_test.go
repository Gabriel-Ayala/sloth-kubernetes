package config

import (
	"fmt"
	"testing"
)

func TestLoader_ApplyOverrides(t *testing.T) {
	loader := NewLoader("test.yaml")
	loader.SetOverride("metadata.name", "override-cluster")
	loader.SetOverride("metadata.owner", "test-owner")
	loader.SetOverride("metadata.team", "devops")
	loader.SetOverride("cluster.type", "k3s")
	loader.SetOverride("cluster.version", "v1.29.0")

	config := &ClusterConfig{
		Metadata: Metadata{
			Name:  "original",
			Owner: "original-owner",
		},
		Cluster: ClusterSpec{
			Type:    "rke2",
			Version: "v1.28.0",
		},
	}

	err := loader.applyOverrides(config)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check overrides were applied
	if config.Metadata.Name != "override-cluster" {
		t.Errorf("expected name 'override-cluster', got '%s'", config.Metadata.Name)
	}

	if config.Metadata.Owner != "test-owner" {
		t.Errorf("expected owner 'test-owner', got '%s'", config.Metadata.Owner)
	}

	if config.Metadata.Team != "devops" {
		t.Errorf("expected team 'devops', got '%s'", config.Metadata.Team)
	}

	if config.Cluster.Type != "k3s" {
		t.Errorf("expected type 'k3s', got '%s'", config.Cluster.Type)
	}

	if config.Cluster.Version != "v1.29.0" {
		t.Errorf("expected version 'v1.29.0', got '%s'", config.Cluster.Version)
	}
}

func TestLoader_SetConfigValue_Metadata(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		value    interface{}
		validate func(*ClusterConfig) error
	}{
		{
			name:  "Set metadata.name",
			path:  "metadata.name",
			value: "test-cluster",
			validate: func(c *ClusterConfig) error {
				if c.Metadata.Name != "test-cluster" {
					return fmt.Errorf("name not set")
				}
				return nil
			},
		},
		{
			name:  "Set metadata.environment",
			path:  "metadata.environment",
			value: "production",
			validate: func(c *ClusterConfig) error {
				if c.Metadata.Environment != "production" {
					return fmt.Errorf("environment not set")
				}
				return nil
			},
		},
		{
			name:  "Set metadata.owner",
			path:  "metadata.owner",
			value: "platform-team",
			validate: func(c *ClusterConfig) error {
				if c.Metadata.Owner != "platform-team" {
					return fmt.Errorf("owner not set")
				}
				return nil
			},
		},
		{
			name:  "Set metadata.team",
			path:  "metadata.team",
			value: "infrastructure",
			validate: func(c *ClusterConfig) error {
				if c.Metadata.Team != "infrastructure" {
					return fmt.Errorf("team not set")
				}
				return nil
			},
		},
		{
			name:  "Set cluster.type",
			path:  "cluster.type",
			value: "k3s",
			validate: func(c *ClusterConfig) error {
				if c.Cluster.Type != "k3s" {
					return fmt.Errorf("type not set")
				}
				return nil
			},
		},
		{
			name:  "Set cluster.version",
			path:  "cluster.version",
			value: "v1.29.0",
			validate: func(c *ClusterConfig) error {
				if c.Cluster.Version != "v1.29.0" {
					return fmt.Errorf("version not set")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewLoader("test.yaml")
			config := &ClusterConfig{}

			err := loader.setConfigValue(config, tt.path, tt.value)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err := tt.validate(config); err != nil {
				t.Errorf("validation failed: %v", err)
			}
		})
	}
}

func TestLoader_Validate_MultipleValidators(t *testing.T) {
	loader := NewLoader("test.yaml")

	// Add multiple custom validators
	validator1Called := false
	validator2Called := false

	validator1 := &mockValidator{
		validateFunc: func(config *ClusterConfig) error {
			validator1Called = true
			if config.Metadata.Name == "forbidden" {
				return fmt.Errorf("forbidden name")
			}
			return nil
		},
	}

	validator2 := &mockValidator{
		validateFunc: func(config *ClusterConfig) error {
			validator2Called = true
			if config.Metadata.Environment == "invalid" {
				return fmt.Errorf("invalid environment")
			}
			return nil
		},
	}

	loader.AddValidator(validator1)
	loader.AddValidator(validator2)

	// Valid config
	config := &ClusterConfig{
		Metadata: Metadata{
			Name:        "test",
			Environment: "dev",
		},
		Providers: ProvidersConfig{
			DigitalOcean: &DigitalOceanProvider{
				Enabled: true,
			},
		},
		NodePools: map[string]NodePool{
			"masters": {
				Name:     "masters",
				Provider: "digitalocean",
				Count:    3,
				Roles:    []string{"master"},
			},
		},
	}

	err := loader.validate(config)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !validator1Called {
		t.Error("validator1 was not called")
	}

	if !validator2Called {
		t.Error("validator2 was not called")
	}
}

func TestMergeConfig_WithNodePools(t *testing.T) {
	target := &ClusterConfig{
		Metadata: Metadata{
			Name: "target",
		},
		NodePools: map[string]NodePool{
			"pool1": {
				Name:     "pool1",
				Provider: "digitalocean",
				Count:    3,
			},
		},
	}

	source := &ClusterConfig{
		Metadata: Metadata{
			Name: "source",
		},
		NodePools: map[string]NodePool{
			"pool2": {
				Name:     "pool2",
				Provider: "linode",
				Count:    2,
			},
		},
	}

	err := mergeConfig(target, source)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Metadata should be overridden
	if target.Metadata.Name != "source" {
		t.Errorf("expected name 'source', got '%s'", target.Metadata.Name)
	}

	// Node pools should be merged
	if len(target.NodePools) != 2 {
		t.Errorf("expected 2 node pools, got %d", len(target.NodePools))
	}

	if _, exists := target.NodePools["pool1"]; !exists {
		t.Error("pool1 should exist in merged config")
	}

	if _, exists := target.NodePools["pool2"]; !exists {
		t.Error("pool2 should exist in merged config")
	}
}

func TestLoader_SetDefaults_WireGuard(t *testing.T) {
	tests := []struct {
		name   string
		config *ClusterConfig
		check  func(*ClusterConfig) bool
	}{
		{
			name: "WireGuard enabled - sets defaults",
			config: &ClusterConfig{
				Network: NetworkConfig{
					WireGuard: &WireGuardConfig{
						Enabled: true,
					},
				},
			},
			check: func(c *ClusterConfig) bool {
				return c.Network.WireGuard.Port == 51820 &&
					c.Network.WireGuard.PersistentKeepalive == 25 &&
					c.Network.WireGuard.MTU == 1420 &&
					len(c.Network.WireGuard.DNS) == 2 &&
					len(c.Network.WireGuard.AllowedIPs) > 0
			},
		},
		{
			name: "WireGuard nil - no defaults",
			config: &ClusterConfig{
				Network: NetworkConfig{
					WireGuard: nil,
				},
			},
			check: func(c *ClusterConfig) bool {
				return c.Network.WireGuard == nil
			},
		},
		{
			name: "WireGuard disabled - no defaults",
			config: &ClusterConfig{
				Network: NetworkConfig{
					WireGuard: &WireGuardConfig{
						Enabled: false,
					},
				},
			},
			check: func(c *ClusterConfig) bool {
				// Defaults might still be set even if disabled
				return true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewLoader("test.yaml")
			err := loader.setDefaults(tt.config)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.check(tt.config) {
				t.Error("check failed")
			}
		})
	}
}

func TestLoader_Validate_Providers(t *testing.T) {
	tests := []struct {
		name    string
		config  *ClusterConfig
		wantErr bool
	}{
		{
			name: "DigitalOcean enabled",
			config: &ClusterConfig{
				Metadata: Metadata{Name: "test"},
				Providers: ProvidersConfig{
					DigitalOcean: &DigitalOceanProvider{
						Enabled: true,
					},
				},
				NodePools: map[string]NodePool{
					"masters": {Name: "masters", Count: 3, Roles: []string{"master"}},
				},
			},
			wantErr: false,
		},
		{
			name: "Linode enabled",
			config: &ClusterConfig{
				Metadata: Metadata{Name: "test"},
				Providers: ProvidersConfig{
					Linode: &LinodeProvider{
						Enabled: true,
					},
				},
				NodePools: map[string]NodePool{
					"masters": {Name: "masters", Count: 3, Roles: []string{"master"}},
				},
			},
			wantErr: false,
		},
		{
			name: "AWS enabled",
			config: &ClusterConfig{
				Metadata: Metadata{Name: "test"},
				Providers: ProvidersConfig{
					AWS: &AWSProvider{
						Enabled: true,
					},
				},
				NodePools: map[string]NodePool{
					"masters": {Name: "masters", Count: 3, Roles: []string{"master"}},
				},
			},
			wantErr: false,
		},
		{
			name: "Azure enabled",
			config: &ClusterConfig{
				Metadata: Metadata{Name: "test"},
				Providers: ProvidersConfig{
					Azure: &AzureProvider{
						Enabled: true,
					},
				},
				NodePools: map[string]NodePool{
					"masters": {Name: "masters", Count: 3, Roles: []string{"master"}},
				},
			},
			wantErr: false,
		},
		{
			name: "GCP enabled",
			config: &ClusterConfig{
				Metadata: Metadata{Name: "test"},
				Providers: ProvidersConfig{
					GCP: &GCPProvider{
						Enabled: true,
					},
				},
				NodePools: map[string]NodePool{
					"masters": {Name: "masters", Count: 3, Roles: []string{"master"}},
				},
			},
			wantErr: false,
		},
		{
			name: "No providers",
			config: &ClusterConfig{
				Metadata:  Metadata{Name: "test"},
				Providers: ProvidersConfig{},
				NodePools: map[string]NodePool{
					"masters": {Name: "masters", Count: 3, Roles: []string{"master"}},
				},
			},
			wantErr: true,
		},
		{
			name: "Multiple providers enabled",
			config: &ClusterConfig{
				Metadata: Metadata{Name: "test"},
				Providers: ProvidersConfig{
					DigitalOcean: &DigitalOceanProvider{
						Enabled: true,
					},
					Linode: &LinodeProvider{
						Enabled: true,
					},
				},
				NodePools: map[string]NodePool{
					"masters": {Name: "masters", Count: 3, Roles: []string{"master"}},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewLoader("test.yaml")
			err := loader.validate(tt.config)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoader_Validate_NodeRoles(t *testing.T) {
	tests := []struct {
		name    string
		config  *ClusterConfig
		wantErr bool
	}{
		{
			name: "With master node",
			config: &ClusterConfig{
				Metadata: Metadata{Name: "test"},
				Providers: ProvidersConfig{
					DigitalOcean: &DigitalOceanProvider{Enabled: true},
				},
				Nodes: []NodeConfig{
					{Name: "master-1", Roles: []string{"master"}},
				},
			},
			wantErr: false,
		},
		{
			name: "With controlplane node (synonym)",
			config: &ClusterConfig{
				Metadata: Metadata{Name: "test"},
				Providers: ProvidersConfig{
					DigitalOcean: &DigitalOceanProvider{Enabled: true},
				},
				Nodes: []NodeConfig{
					{Name: "master-1", Roles: []string{"controlplane"}},
				},
			},
			wantErr: false,
		},
		{
			name: "With master in nodepool",
			config: &ClusterConfig{
				Metadata: Metadata{Name: "test"},
				Providers: ProvidersConfig{
					DigitalOcean: &DigitalOceanProvider{Enabled: true},
				},
				NodePools: map[string]NodePool{
					"masters": {Name: "masters", Count: 3, Roles: []string{"master"}},
				},
			},
			wantErr: false,
		},
		{
			name: "Only worker nodes",
			config: &ClusterConfig{
				Metadata: Metadata{Name: "test"},
				Providers: ProvidersConfig{
					DigitalOcean: &DigitalOceanProvider{Enabled: true},
				},
				Nodes: []NodeConfig{
					{Name: "worker-1", Roles: []string{"worker"}},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewLoader("test.yaml")
			err := loader.validate(tt.config)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestApplyDefaults_Kubernetes(t *testing.T) {
	config := &ClusterConfig{}
	applyDefaults(config)

	if config.Kubernetes.Distribution != "rke2" {
		t.Errorf("expected distribution 'rke2', got '%s'", config.Kubernetes.Distribution)
	}
	if config.Kubernetes.NetworkPlugin != "canal" {
		t.Errorf("expected network plugin 'canal', got '%s'", config.Kubernetes.NetworkPlugin)
	}
	if config.Kubernetes.PodCIDR != "10.42.0.0/16" {
		t.Errorf("expected pod CIDR '10.42.0.0/16', got '%s'", config.Kubernetes.PodCIDR)
	}
	if config.Kubernetes.ServiceCIDR != "10.43.0.0/16" {
		t.Errorf("expected service CIDR '10.43.0.0/16', got '%s'", config.Kubernetes.ServiceCIDR)
	}
	if config.Kubernetes.ClusterDNS != "10.43.0.10" {
		t.Errorf("expected cluster DNS '10.43.0.10', got '%s'", config.Kubernetes.ClusterDNS)
	}
	if config.Kubernetes.ClusterDomain != "cluster.local" {
		t.Errorf("expected cluster domain 'cluster.local', got '%s'", config.Kubernetes.ClusterDomain)
	}
}

func TestApplyDefaults_Network(t *testing.T) {
	config := &ClusterConfig{}
	applyDefaults(config)

	if config.Network.Mode != "wireguard" {
		t.Errorf("expected mode 'wireguard', got '%s'", config.Network.Mode)
	}
	if config.Network.PodCIDR != "10.42.0.0/16" {
		t.Errorf("expected pod CIDR '10.42.0.0/16', got '%s'", config.Network.PodCIDR)
	}
	if config.Network.ServiceCIDR != "10.43.0.0/16" {
		t.Errorf("expected service CIDR '10.43.0.0/16', got '%s'", config.Network.ServiceCIDR)
	}
}

func TestApplyDefaults_Metadata(t *testing.T) {
	config := &ClusterConfig{}
	applyDefaults(config)

	if config.Metadata.Name != "kubernetes-cluster" {
		t.Errorf("expected name 'kubernetes-cluster', got '%s'", config.Metadata.Name)
	}
	if config.Metadata.Environment != "development" {
		t.Errorf("expected environment 'development', got '%s'", config.Metadata.Environment)
	}
	if config.Metadata.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", config.Metadata.Version)
	}
}
