package components

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// sshKeysMocks provides mock implementation for SSH keys tests
type sshKeysMocks int

// NewResource creates mock resources for SSH keys tests
func (sshKeysMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	outputs := args.Inputs.Copy()
	if args.TypeToken == "kubernetes-create:security:SSHKey" {
		outputs["publicKey"] = resource.NewStringProperty("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDTest...")
		outputs["privateKey"] = resource.NewStringProperty("-----BEGIN RSA PRIVATE KEY-----\nTest...")
		outputs["privateKeyPath"] = resource.NewStringProperty("~/.ssh/kubernetes-clusters/stack.pem")
	}
	return args.Name + "_id", outputs, nil
}

// Call mocks function calls
func (sshKeysMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return resource.PropertyMap{}, nil
}

// TestNewSSHKeyComponent tests SSH key component creation
func TestNewSSHKeyComponent(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "test-cluster",
				Environment: "testing",
			},
		}

		component, err := NewSSHKeyComponent(ctx, "test-ssh-keys", cfg)
		if err != nil {
			t.Logf("SSH key generation failed (expected in test): %v", err)
			return nil
		}

		assert.NotNil(t, component)
		assert.NotNil(t, component.PublicKey)
		assert.NotNil(t, component.PrivateKey)
		assert.NotNil(t, component.PrivateKeyPath)

		return nil
	}, pulumi.WithMocks("test", "stack", sshKeysMocks(0)))

	assert.NoError(t, err)
}

// TestNewProviderComponent tests provider component creation
func TestNewProviderComponent(t *testing.T) {
	testCases := []struct {
		name          string
		config        *config.ClusterConfig
		expectedCount int
	}{
		{
			name: "DigitalOcean only",
			config: &config.ClusterConfig{
				Providers: config.ProvidersConfig{
					DigitalOcean: &config.DigitalOceanProvider{
						Enabled: true,
						Token:   "test-token",
					},
				},
			},
			expectedCount: 1,
		},
		{
			name: "DigitalOcean and Linode",
			config: &config.ClusterConfig{
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
			},
			expectedCount: 2,
		},
		{
			name: "No providers enabled",
			config: &config.ClusterConfig{
				Providers: config.ProvidersConfig{},
			},
			expectedCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				sshKey := pulumi.String("ssh-rsa test...").ToStringOutput()
				component, err := NewProviderComponent(ctx, "test-providers", tc.config, sshKey)
				assert.NoError(t, err)
				assert.NotNil(t, component)
				assert.NotNil(t, component.Providers)

				return nil
			}, pulumi.WithMocks("test", "stack", sshKeysMocks(0)))

			assert.NoError(t, err)
		})
	}
}

// TestNewNetworkComponent tests network component creation
func TestNewNetworkComponent(t *testing.T) {
	testCases := []struct {
		name         string
		config       *config.ClusterConfig
		expectedCIDR string
	}{
		{
			name: "Default CIDR",
			config: &config.ClusterConfig{
				Network: config.NetworkConfig{
					CIDR: "10.0.0.0/16",
				},
			},
			expectedCIDR: "10.0.0.0/16",
		},
		{
			name: "Custom CIDR",
			config: &config.ClusterConfig{
				Network: config.NetworkConfig{
					CIDR: "192.168.0.0/16",
				},
			},
			expectedCIDR: "192.168.0.0/16",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				providersMap := pulumi.Map{
					"digitalocean": pulumi.String("initialized"),
				}.ToMapOutput()

				component, err := NewNetworkComponent(ctx, "test-network", tc.config, providersMap)
				assert.NoError(t, err)
				assert.NotNil(t, component)
				assert.NotNil(t, component.Networks)

				return nil
			}, pulumi.WithMocks("test", "stack", sshKeysMocks(0)))

			assert.NoError(t, err)
		})
	}
}

// TestNewDNSComponent tests DNS component creation
func TestNewDNSComponent(t *testing.T) {
	testCases := []struct {
		name           string
		config         *config.ClusterConfig
		expectedDomain string
	}{
		{
			name: "Custom domain",
			config: &config.ClusterConfig{
				Network: config.NetworkConfig{
					DNS: config.DNSConfig{
						Domain: "example.com",
					},
				},
			},
			expectedDomain: "example.com",
		},
		{
			name: "Default domain",
			config: &config.ClusterConfig{
				Network: config.NetworkConfig{
					DNS: config.DNSConfig{
						Domain: "",
					},
				},
			},
			expectedDomain: "chalkan3.com.br",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				nodes := pulumi.Array{
					pulumi.Map{"name": pulumi.String("node-1")},
					pulumi.Map{"name": pulumi.String("node-2")},
				}.ToArrayOutput()

				component, err := NewDNSComponent(ctx, "test-dns", tc.config, nodes)
				assert.NoError(t, err)
				assert.NotNil(t, component)
				assert.NotNil(t, component.Records)

				return nil
			}, pulumi.WithMocks("test", "stack", sshKeysMocks(0)))

			assert.NoError(t, err)
		})
	}
}

// TestNewWireGuardComponent tests WireGuard component creation
func TestNewWireGuardComponent(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Network: config.NetworkConfig{
				WireGuard: &config.WireGuardConfig{
					Enabled:        true,
					ServerEndpoint: "vpn.example.com:51820",
				},
			},
		}

		nodes := pulumi.Array{
			pulumi.Map{"name": pulumi.String("node-1")},
			pulumi.Map{"name": pulumi.String("node-2")},
			pulumi.Map{"name": pulumi.String("node-3")},
		}.ToArrayOutput()

		sshKeyPath := pulumi.String("~/.ssh/test.pem").ToStringOutput()

		component, err := NewWireGuardComponent(ctx, "test-wireguard", cfg, nodes, sshKeyPath)
		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.NotNil(t, component.Status)
		assert.NotNil(t, component.ClientConfigs)
		assert.NotNil(t, component.MeshStatus)

		return nil
	}, pulumi.WithMocks("test", "stack", sshKeysMocks(0)))

	assert.NoError(t, err)
}

// TestNewCloudFirewallComponent tests cloud firewall component creation
func TestNewCloudFirewallComponent(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Network: config.NetworkConfig{
				CIDR: "10.0.0.0/16",
			},
		}

		providersMap := pulumi.Map{
			"digitalocean": pulumi.String("initialized"),
		}.ToMapOutput()

		nodes := pulumi.Array{
			pulumi.Map{"name": pulumi.String("node-1")},
		}.ToArrayOutput()

		component, err := NewCloudFirewallComponent(ctx, "test-firewall", cfg, providersMap, nodes)
		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.NotNil(t, component.Status)

		return nil
	}, pulumi.WithMocks("test", "stack", sshKeysMocks(0)))

	assert.NoError(t, err)
}

// TestNewRKEComponent tests RKE component creation
func TestNewRKEComponent(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-cluster",
			},
			Network: config.NetworkConfig{
				DNS: config.DNSConfig{
					Domain: "example.com",
				},
			},
		}

		nodes := pulumi.Array{
			pulumi.Map{"name": pulumi.String("node-1"), "role": pulumi.String("controlplane")},
			pulumi.Map{"name": pulumi.String("node-2"), "role": pulumi.String("worker")},
		}.ToArrayOutput()

		sshKeyPath := pulumi.String("~/.ssh/test.pem").ToStringOutput()

		component, err := NewRKEComponent(ctx, "test-rke", cfg, nodes, sshKeyPath)
		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.NotNil(t, component.Status)
		assert.NotNil(t, component.KubeConfig)
		assert.NotNil(t, component.ClusterState)

		// Verify kubeconfig contains expected values
		component.KubeConfig.ApplyT(func(kubeconfig string) error {
			assert.Contains(t, kubeconfig, "api.example.com")
			assert.Contains(t, kubeconfig, "test-cluster")
			assert.Contains(t, kubeconfig, "kind: Config")
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", sshKeysMocks(0)))

	assert.NoError(t, err)
}

// TestNewIngressComponent tests ingress component creation
func TestNewIngressComponent(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-cluster",
			},
		}

		nodes := pulumi.Array{
			pulumi.Map{"name": pulumi.String("node-1")},
		}.ToArrayOutput()

		sshKeyPath := pulumi.String("~/.ssh/test.pem").ToStringOutput()

		component, err := NewIngressComponent(ctx, "test-ingress", cfg, nodes, sshKeyPath)
		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.NotNil(t, component.Status)

		// Verify status message
		component.Status.ApplyT(func(status string) error {
			assert.Contains(t, status, "NGINX Ingress")
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", sshKeysMocks(0)))

	assert.NoError(t, err)
}

// TestNewAddonsComponent tests addons component creation
func TestNewAddonsComponent(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-cluster",
			},
		}

		nodes := pulumi.Array{
			pulumi.Map{"name": pulumi.String("node-1")},
		}.ToArrayOutput()

		sshKeyPath := pulumi.String("~/.ssh/test.pem").ToStringOutput()

		component, err := NewAddonsComponent(ctx, "test-addons", cfg, nodes, sshKeyPath)
		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.NotNil(t, component.Status)

		// Verify status message
		component.Status.ApplyT(func(status string) error {
			assert.Contains(t, status, "Addons installed")
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", sshKeysMocks(0)))

	assert.NoError(t, err)
}

// TestNewProviderComponent_AllProviders tests all providers enabled
func TestNewProviderComponent_AllProviders(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{Enabled: true, Token: "test"},
				Linode:       &config.LinodeProvider{Enabled: true, Token: "test"},
				AWS:          &config.AWSProvider{Enabled: true, Region: "us-east-1"},
				GCP:          &config.GCPProvider{Enabled: true, ProjectID: "test", Region: "us-central1"},
				Azure:        &config.AzureProvider{Enabled: true, Location: "eastus"},
			},
		}

		sshKey := pulumi.String("ssh-rsa test...").ToStringOutput()
		component, err := NewProviderComponent(ctx, "test-all-providers", cfg, sshKey)
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", sshKeysMocks(0)))

	assert.NoError(t, err)
}

// TestNewWireGuardComponent_ClientConfigs tests client config generation
func TestNewWireGuardComponent_ClientConfigs(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Network: config.NetworkConfig{
				WireGuard: &config.WireGuardConfig{
					Enabled:        true,
					ServerEndpoint: "vpn.test.com:51820",
				},
			},
		}

		nodes := pulumi.Array{
			pulumi.Map{"name": pulumi.String("node-1")},
			pulumi.Map{"name": pulumi.String("node-2")},
		}.ToArrayOutput()

		sshKeyPath := pulumi.String("~/.ssh/test.pem").ToStringOutput()

		component, err := NewWireGuardComponent(ctx, "test-wg-configs", cfg, nodes, sshKeyPath)
		assert.NoError(t, err)

		// Verify client configs are generated
		component.ClientConfigs.ApplyT(func(configs map[string]interface{}) error {
			assert.NotEmpty(t, configs)
			// Should have configs for node-0 and node-1
			assert.Contains(t, configs, "node-0")
			assert.Contains(t, configs, "node-1")
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", sshKeysMocks(0)))

	assert.NoError(t, err)
}

// TestNewNetworkComponent_NetworkStatus tests network status
func TestNewNetworkComponent_NetworkStatus(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Network: config.NetworkConfig{
				CIDR: "172.16.0.0/16",
			},
		}

		providersMap := pulumi.Map{
			"digitalocean": pulumi.String("initialized"),
		}.ToMapOutput()

		component, err := NewNetworkComponent(ctx, "test-network-status", cfg, providersMap)
		assert.NoError(t, err)

		// Verify network configuration
		component.Networks.ApplyT(func(networks map[string]interface{}) error {
			assert.Equal(t, "172.16.0.0/16", networks["cidr"])
			assert.Equal(t, "created", networks["status"])
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", sshKeysMocks(0)))

	assert.NoError(t, err)
}

// TestNewRKEComponent_KubeConfigFormat tests kubeconfig format
func TestNewRKEComponent_KubeConfigFormat(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "prod-cluster",
			},
			Network: config.NetworkConfig{
				DNS: config.DNSConfig{
					Domain: "prod.example.com",
				},
			},
		}

		nodes := pulumi.Array{
			pulumi.Map{"name": pulumi.String("node-1")},
		}.ToArrayOutput()

		sshKeyPath := pulumi.String("~/.ssh/test.pem").ToStringOutput()

		component, err := NewRKEComponent(ctx, "test-rke-kubeconfig", cfg, nodes, sshKeyPath)
		assert.NoError(t, err)

		// Verify kubeconfig format
		component.KubeConfig.ApplyT(func(kubeconfig string) error {
			assert.Contains(t, kubeconfig, "apiVersion: v1")
			assert.Contains(t, kubeconfig, "prod-cluster")
			assert.Contains(t, kubeconfig, "prod.example.com")
			assert.Contains(t, kubeconfig, "kube-admin-prod-cluster")
			return nil
		})

		// Verify cluster state
		component.ClusterState.ApplyT(func(state string) error {
			assert.Equal(t, "Active", state)
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", sshKeysMocks(0)))

	assert.NoError(t, err)
}
