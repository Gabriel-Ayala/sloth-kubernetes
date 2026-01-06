package providers

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// FactoryTestMockProvider implements pulumi mock provider for factory tests
type FactoryTestMockProvider struct {
	pulumi.ResourceState
}

func (m *FactoryTestMockProvider) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	return args.Name + "_id", args.Inputs, nil
}

func (m *FactoryTestMockProvider) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

// TestNewProviderFactory tests factory creation
func TestNewProviderFactory(t *testing.T) {
	factory := NewProviderFactory()

	assert.NotNil(t, factory)
	assert.NotNil(t, factory.registry)

	// Verify all providers are registered
	providers := factory.GetSupportedProviders()
	assert.Len(t, providers, 6)
	assert.Contains(t, providers, "digitalocean")
	assert.Contains(t, providers, "linode")
	assert.Contains(t, providers, "aws")
	assert.Contains(t, providers, "gcp")
	assert.Contains(t, providers, "azure")
	assert.Contains(t, providers, "hetzner")
}

// TestProviderFactory_registerAllProviders tests provider registration
func TestProviderFactory_registerAllProviders(t *testing.T) {
	factory := NewProviderFactory()

	// Verify each provider can be retrieved
	providerNames := []string{"digitalocean", "linode", "aws", "gcp", "azure", "hetzner"}
	for _, name := range providerNames {
		provider, err := factory.GetProvider(name)
		assert.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, name, provider.GetName())
	}
}

// TestProviderFactory_GetRegistry tests getting the registry
func TestProviderFactory_GetRegistry(t *testing.T) {
	factory := NewProviderFactory()

	registry := factory.GetRegistry()
	assert.NotNil(t, registry)

	// Verify registry has providers
	provider, ok := registry.Get("digitalocean")
	assert.True(t, ok)
	assert.NotNil(t, provider)
}

// TestProviderFactory_GetProvider tests getting a specific provider
func TestProviderFactory_GetProvider(t *testing.T) {
	factory := NewProviderFactory()

	tests := []struct {
		name         string
		providerName string
		expectError  bool
		expectedName string
	}{
		{
			name:         "DigitalOcean provider",
			providerName: "digitalocean",
			expectError:  false,
			expectedName: "digitalocean",
		},
		{
			name:         "Linode provider",
			providerName: "linode",
			expectError:  false,
			expectedName: "linode",
		},
		{
			name:         "AWS provider",
			providerName: "aws",
			expectError:  false,
			expectedName: "aws",
		},
		{
			name:         "GCP provider",
			providerName: "gcp",
			expectError:  false,
			expectedName: "gcp",
		},
		{
			name:         "Azure provider",
			providerName: "azure",
			expectError:  false,
			expectedName: "azure",
		},
		{
			name:         "Hetzner provider",
			providerName: "hetzner",
			expectError:  false,
			expectedName: "hetzner",
		},
		{
			name:         "Non-existent provider",
			providerName: "invalid",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := factory.GetProvider(tt.providerName)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, provider)
				assert.Contains(t, err.Error(), "not found")
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
				assert.Equal(t, tt.expectedName, provider.GetName())
			}
		})
	}
}

// TestProviderFactory_GetEnabledProviders tests getting enabled providers
func TestProviderFactory_GetEnabledProviders(t *testing.T) {
	factory := NewProviderFactory()

	tests := []struct {
		name          string
		config        *config.ClusterConfig
		expectedCount int
		expectError   bool
		expectedNames []string
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
			expectError:   false,
			expectedNames: []string{"digitalocean"},
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
			expectError:   false,
			expectedNames: []string{"digitalocean", "linode"},
		},
		{
			name: "All providers enabled",
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
					AWS: &config.AWSProvider{
						Enabled: true,
						Region:  "us-east-1",
					},
					GCP: &config.GCPProvider{
						Enabled:   true,
						ProjectID: "test-project",
						Region:    "us-central1",
					},
					Azure: &config.AzureProvider{
						Enabled:  true,
						Location: "eastus",
					},
				},
			},
			expectedCount: 5,
			expectError:   false,
			expectedNames: []string{"digitalocean", "linode", "aws", "gcp", "azure"},
		},
		{
			name: "No providers enabled",
			config: &config.ClusterConfig{
				Providers: config.ProvidersConfig{},
			},
			expectedCount: 0,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providers, err := factory.GetEnabledProviders(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "no providers enabled")
			} else {
				assert.NoError(t, err)
				assert.Len(t, providers, tt.expectedCount)

				// Verify provider names
				for _, expectedName := range tt.expectedNames {
					found := false
					for _, provider := range providers {
						if provider.GetName() == expectedName {
							found = true
							break
						}
					}
					assert.True(t, found, "Expected provider %s not found", expectedName)
				}
			}
		})
	}
}

// TestProviderFactory_InitializeEnabledProviders tests provider initialization
func TestProviderFactory_InitializeEnabledProviders(t *testing.T) {
	t.Skip("Skipping - requires SSH keys and full provider initialization")
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		factory := NewProviderFactory()

		cfg := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				DigitalOcean: &config.DigitalOceanProvider{
					Enabled: true,
					Token:   "test-token",
					Region:  "nyc1",
				},
			},
		}

		providers, err := factory.InitializeEnabledProviders(ctx, cfg)
		assert.NoError(t, err)
		assert.Len(t, providers, 1)
		assert.Equal(t, "digitalocean", providers[0].GetName())

		return nil
	}, pulumi.WithMocks("test", "stack", &FactoryTestMockProvider{}))

	assert.NoError(t, err)
}

// TestProviderFactory_InitializeEnabledProviders_NoProviders tests initialization with no providers
func TestProviderFactory_InitializeEnabledProviders_NoProviders(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		factory := NewProviderFactory()

		cfg := &config.ClusterConfig{
			Providers: config.ProvidersConfig{},
		}

		providers, err := factory.InitializeEnabledProviders(ctx, cfg)
		assert.Error(t, err)
		assert.Nil(t, providers)
		assert.Contains(t, err.Error(), "no providers enabled")

		return nil
	}, pulumi.WithMocks("test", "stack", &FactoryTestMockProvider{}))

	assert.NoError(t, err)
}

// TestProviderFactory_GetProviderForNodePool tests getting provider for node pool
func TestProviderFactory_GetProviderForNodePool(t *testing.T) {
	factory := NewProviderFactory()

	tests := []struct {
		name         string
		pool         *config.NodePool
		expectError  bool
		expectedName string
	}{
		{
			name: "DigitalOcean node pool",
			pool: &config.NodePool{
				Name:     "do-pool",
				Provider: "digitalocean",
			},
			expectError:  false,
			expectedName: "digitalocean",
		},
		{
			name: "Linode node pool",
			pool: &config.NodePool{
				Name:     "linode-pool",
				Provider: "linode",
			},
			expectError:  false,
			expectedName: "linode",
		},
		{
			name: "Node pool with no provider",
			pool: &config.NodePool{
				Name:     "no-provider-pool",
				Provider: "",
			},
			expectError: true,
		},
		{
			name: "Node pool with invalid provider",
			pool: &config.NodePool{
				Name:     "invalid-pool",
				Provider: "invalid",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := factory.GetProviderForNodePool(tt.pool)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
				assert.Equal(t, tt.expectedName, provider.GetName())
			}
		})
	}
}

// TestProviderFactory_ValidateProviderConfig tests provider config validation
func TestProviderFactory_ValidateProviderConfig(t *testing.T) {
	factory := NewProviderFactory()

	tests := []struct {
		name        string
		config      *config.ClusterConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid DigitalOcean config",
			config: &config.ClusterConfig{
				Providers: config.ProvidersConfig{
					DigitalOcean: &config.DigitalOceanProvider{
						Enabled: true,
						Token:   "test-token",
					},
				},
			},
			expectError: false,
		},
		{
			name: "Missing DigitalOcean token",
			config: &config.ClusterConfig{
				Providers: config.ProvidersConfig{
					DigitalOcean: &config.DigitalOceanProvider{
						Enabled: true,
						Token:   "",
					},
				},
			},
			expectError: true,
			errorMsg:    "DigitalOcean: token is required",
		},
		{
			name: "Missing Linode token",
			config: &config.ClusterConfig{
				Providers: config.ProvidersConfig{
					Linode: &config.LinodeProvider{
						Enabled: true,
						Token:   "",
					},
				},
			},
			expectError: true,
			errorMsg:    "Linode: token is required",
		},
		{
			name: "Missing AWS region",
			config: &config.ClusterConfig{
				Providers: config.ProvidersConfig{
					AWS: &config.AWSProvider{
						Enabled: true,
						Region:  "",
					},
				},
			},
			expectError: true,
			errorMsg:    "AWS: region is required",
		},
		{
			name: "Missing GCP project ID",
			config: &config.ClusterConfig{
				Providers: config.ProvidersConfig{
					GCP: &config.GCPProvider{
						Enabled:   true,
						ProjectID: "",
						Region:    "us-central1",
					},
				},
			},
			expectError: true,
			errorMsg:    "GCP: projectID is required",
		},
		{
			name: "Missing GCP region",
			config: &config.ClusterConfig{
				Providers: config.ProvidersConfig{
					GCP: &config.GCPProvider{
						Enabled:   true,
						ProjectID: "test-project",
						Region:    "",
					},
				},
			},
			expectError: true,
			errorMsg:    "GCP: region is required",
		},
		{
			name: "Missing Azure location",
			config: &config.ClusterConfig{
				Providers: config.ProvidersConfig{
					Azure: &config.AzureProvider{
						Enabled:  true,
						Location: "",
					},
				},
			},
			expectError: true,
			errorMsg:    "Azure: location is required",
		},
		{
			name: "Multiple validation errors",
			config: &config.ClusterConfig{
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
			},
			expectError: true,
			errorMsg:    "DigitalOcean: token is required",
		},
		{
			name: "No providers enabled - should pass",
			config: &config.ClusterConfig{
				Providers: config.ProvidersConfig{},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := factory.ValidateProviderConfig(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestProviderFactory_GetSupportedProviders tests getting supported providers list
func TestProviderFactory_GetSupportedProviders(t *testing.T) {
	factory := NewProviderFactory()

	providers := factory.GetSupportedProviders()

	assert.Len(t, providers, 6)
	assert.Contains(t, providers, "digitalocean")
	assert.Contains(t, providers, "linode")
	assert.Contains(t, providers, "aws")
	assert.Contains(t, providers, "gcp")
	assert.Contains(t, providers, "azure")
	assert.Contains(t, providers, "hetzner")
}

// TestProviderFactory_GetProviderInfo tests getting provider information
func TestProviderFactory_GetProviderInfo(t *testing.T) {
	factory := NewProviderFactory()

	providerInfo := factory.GetProviderInfo()

	assert.Len(t, providerInfo, 6)

	// Test DigitalOcean info
	doInfo, exists := providerInfo["digitalocean"]
	assert.True(t, exists)
	assert.Equal(t, "DigitalOcean", doInfo.Name)
	assert.Equal(t, "digitalocean", doInfo.Code)
	assert.NotEmpty(t, doInfo.Description)
	assert.NotEmpty(t, doInfo.Regions)
	assert.NotEmpty(t, doInfo.Sizes)

	// Test Linode info
	linodeInfo, exists := providerInfo["linode"]
	assert.True(t, exists)
	assert.Equal(t, "Linode", linodeInfo.Name)
	assert.Equal(t, "linode", linodeInfo.Code)

	// Test AWS info
	awsInfo, exists := providerInfo["aws"]
	assert.True(t, exists)
	assert.Equal(t, "Amazon Web Services", awsInfo.Name)
	assert.Equal(t, "aws", awsInfo.Code)

	// Test GCP info
	gcpInfo, exists := providerInfo["gcp"]
	assert.True(t, exists)
	assert.Equal(t, "Google Cloud Platform", gcpInfo.Name)
	assert.Equal(t, "gcp", gcpInfo.Code)

	// Test Azure info
	azureInfo, exists := providerInfo["azure"]
	assert.True(t, exists)
	assert.Equal(t, "Microsoft Azure", azureInfo.Name)
	assert.Equal(t, "azure", azureInfo.Code)

	// Test Hetzner info
	hetznerInfo, exists := providerInfo["hetzner"]
	assert.True(t, exists)
	assert.Equal(t, "Hetzner Cloud", hetznerInfo.Name)
	assert.Equal(t, "hetzner", hetznerInfo.Code)
}

// TestJoinErrors tests the joinErrors helper function
func TestJoinErrors(t *testing.T) {
	tests := []struct {
		name     string
		errors   []string
		expected string
	}{
		{
			name:     "Single error",
			errors:   []string{"error 1"},
			expected: "error 1",
		},
		{
			name:     "Multiple errors",
			errors:   []string{"error 1", "error 2", "error 3"},
			expected: "error 1\n  - error 2\n  - error 3",
		},
		{
			name:     "No errors",
			errors:   []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinErrors(tt.errors)
			assert.Equal(t, tt.expected, result)
		})
	}
}
