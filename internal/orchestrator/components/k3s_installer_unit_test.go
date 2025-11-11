package components

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// k3sInstallerMocks provides mock implementation for k3s installer tests
type k3sInstallerMocks int

// NewResource creates mock resources for k3s installer tests
func (k3sInstallerMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	outputs := args.Inputs.Copy()
	return args.Name + "_id", outputs, nil
}

// Call mocks function calls
func (k3sInstallerMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return resource.PropertyMap{}, nil
}

// TestGetSSHUserForProviderK3s_AllProviders tests SSH user selection for different providers
func TestGetSSHUserForProviderK3s_AllProviders(t *testing.T) {
	testCases := []struct {
		name         string
		provider     string
		expectedUser string
	}{
		{"Azure provider", "azure", "azureuser"},
		{"AWS provider", "aws", "ubuntu"},
		{"GCP provider", "gcp", "ubuntu"},
		{"DigitalOcean provider", "digitalocean", "root"},
		{"Linode provider", "linode", "root"},
		{"Unknown provider", "unknown", "root"},
		{"Empty provider", "", "root"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				providerOutput := pulumi.String(tc.provider).ToStringOutput()
				result := getSSHUserForProviderK3s(providerOutput)

				// Verify the result
				result.ApplyT(func(user string) error {
					assert.Equal(t, tc.expectedUser, user, "SSH user should match expected for provider %s", tc.provider)
					return nil
				})

				return nil
			}, pulumi.WithMocks("test", "stack", k3sInstallerMocks(0)))

			assert.NoError(t, err)
		})
	}
}

// TestGetSSHUserForProviderK3s_Azure tests Azure-specific behavior
func TestGetSSHUserForProviderK3s_Azure(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		azureProvider := pulumi.String("azure").ToStringOutput()
		result := getSSHUserForProviderK3s(azureProvider)

		result.ApplyT(func(user string) error {
			assert.Equal(t, "azureuser", user, "Azure should use 'azureuser'")
			assert.NotEqual(t, "root", user, "Azure should not use 'root'")
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", k3sInstallerMocks(0)))

	assert.NoError(t, err)
}

// TestGetSSHUserForProviderK3s_AWS tests AWS-specific behavior
func TestGetSSHUserForProviderK3s_AWS(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		awsProvider := pulumi.String("aws").ToStringOutput()
		result := getSSHUserForProviderK3s(awsProvider)

		result.ApplyT(func(user string) error {
			assert.Equal(t, "ubuntu", user, "AWS should use 'ubuntu'")
			assert.NotEqual(t, "root", user, "AWS should not use 'root'")
			assert.NotEqual(t, "azureuser", user, "AWS should not use 'azureuser'")
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", k3sInstallerMocks(0)))

	assert.NoError(t, err)
}

// TestGetSSHUserForProviderK3s_GCP tests GCP-specific behavior
func TestGetSSHUserForProviderK3s_GCP(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		gcpProvider := pulumi.String("gcp").ToStringOutput()
		result := getSSHUserForProviderK3s(gcpProvider)

		result.ApplyT(func(user string) error {
			assert.Equal(t, "ubuntu", user, "GCP should use 'ubuntu'")
			assert.NotEqual(t, "root", user, "GCP should not use 'root'")
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", k3sInstallerMocks(0)))

	assert.NoError(t, err)
}

// TestGetSSHUserForProviderK3s_DigitalOcean tests DigitalOcean-specific behavior
func TestGetSSHUserForProviderK3s_DigitalOcean(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		doProvider := pulumi.String("digitalocean").ToStringOutput()
		result := getSSHUserForProviderK3s(doProvider)

		result.ApplyT(func(user string) error {
			assert.Equal(t, "root", user, "DigitalOcean should use 'root'")
			assert.NotEqual(t, "ubuntu", user, "DigitalOcean should not use 'ubuntu'")
			assert.NotEqual(t, "azureuser", user, "DigitalOcean should not use 'azureuser'")
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", k3sInstallerMocks(0)))

	assert.NoError(t, err)
}

// TestGetSSHUserForProviderK3s_Linode tests Linode-specific behavior
func TestGetSSHUserForProviderK3s_Linode(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		linodeProvider := pulumi.String("linode").ToStringOutput()
		result := getSSHUserForProviderK3s(linodeProvider)

		result.ApplyT(func(user string) error {
			assert.Equal(t, "root", user, "Linode should use 'root'")
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", k3sInstallerMocks(0)))

	assert.NoError(t, err)
}

// TestGetSSHUserForProviderK3s_DefaultBehavior tests default behavior for unknown providers
func TestGetSSHUserForProviderK3s_DefaultBehavior(t *testing.T) {
	unknownProviders := []string{"vultr", "hetzner", "scaleway", "ovh", "custom"}

	for _, provider := range unknownProviders {
		t.Run(provider, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				providerOutput := pulumi.String(provider).ToStringOutput()
				result := getSSHUserForProviderK3s(providerOutput)

				result.ApplyT(func(user string) error {
					assert.Equal(t, "root", user, "Unknown provider %s should default to 'root'", provider)
					return nil
				})

				return nil
			}, pulumi.WithMocks("test", "stack", k3sInstallerMocks(0)))

			assert.NoError(t, err)
		})
	}
}

// TestGetSSHUserForProviderK3s_CaseSensitivity tests case sensitivity
func TestGetSSHUserForProviderK3s_CaseSensitivity(t *testing.T) {
	testCases := []struct {
		name         string
		provider     string
		expectedUser string
	}{
		{"Lowercase azure", "azure", "azureuser"},
		{"Uppercase AZURE", "AZURE", "root"}, // Should default to root
		{"Mixed case Azure", "Azure", "root"}, // Should default to root
		{"Lowercase aws", "aws", "ubuntu"},
		{"Uppercase AWS", "AWS", "root"}, // Should default to root
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				providerOutput := pulumi.String(tc.provider).ToStringOutput()
				result := getSSHUserForProviderK3s(providerOutput)

				result.ApplyT(func(user string) error {
					assert.Equal(t, tc.expectedUser, user)
					return nil
				})

				return nil
			}, pulumi.WithMocks("test", "stack", k3sInstallerMocks(0)))

			assert.NoError(t, err)
		})
	}
}

// TestGetSSHUserForProviderK3s_MultipleCallsConsistency tests consistency across multiple calls
func TestGetSSHUserForProviderK3s_MultipleCallsConsistency(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		azureProvider := pulumi.String("azure").ToStringOutput()

		// Call the function multiple times
		result1 := getSSHUserForProviderK3s(azureProvider)
		result2 := getSSHUserForProviderK3s(azureProvider)
		result3 := getSSHUserForProviderK3s(azureProvider)

		// All should return the same result
		pulumi.All(result1, result2, result3).ApplyT(func(args []interface{}) error {
			user1 := args[0].(string)
			user2 := args[1].(string)
			user3 := args[2].(string)

			assert.Equal(t, user1, user2, "Multiple calls should return same result")
			assert.Equal(t, user1, user3, "Multiple calls should return same result")
			assert.Equal(t, "azureuser", user1)
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", k3sInstallerMocks(0)))

	assert.NoError(t, err)
}

// TestGetSSHUserForProviderK3s_EdgeCases tests edge cases
func TestGetSSHUserForProviderK3s_EdgeCases(t *testing.T) {
	testCases := []struct {
		name         string
		provider     string
		expectedUser string
	}{
		{"Empty string", "", "root"},
		{"Whitespace", "   ", "root"},
		{"Special chars", "@#$%", "root"},
		{"Numbers only", "12345", "root"},
		{"Very long name", "averylongprovidernamethatdoesntexist", "root"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				providerOutput := pulumi.String(tc.provider).ToStringOutput()
				result := getSSHUserForProviderK3s(providerOutput)

				result.ApplyT(func(user string) error {
					assert.Equal(t, tc.expectedUser, user)
					return nil
				})

				return nil
			}, pulumi.WithMocks("test", "stack", k3sInstallerMocks(0)))

			assert.NoError(t, err)
		})
	}
}

// TestGetSSHUserForProviderK3s_AllCloudProviders tests all major cloud providers
func TestGetSSHUserForProviderK3s_AllCloudProviders(t *testing.T) {
	testCases := map[string]struct {
		provider     string
		expectedUser string
		description  string
	}{
		"Azure":        {"azure", "azureuser", "Azure VMs use azureuser by default"},
		"AWS":          {"aws", "ubuntu", "AWS Ubuntu AMIs use ubuntu user"},
		"GCP":          {"gcp", "ubuntu", "GCP Ubuntu images use ubuntu user"},
		"DigitalOcean": {"digitalocean", "root", "DigitalOcean droplets use root"},
		"Linode":       {"linode", "root", "Linode instances use root"},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				providerOutput := pulumi.String(tc.provider).ToStringOutput()
				result := getSSHUserForProviderK3s(providerOutput)

				result.ApplyT(func(user string) error {
					assert.Equal(t, tc.expectedUser, user, tc.description)
					return nil
				})

				return nil
			}, pulumi.WithMocks("test", "stack", k3sInstallerMocks(0)))

			assert.NoError(t, err)
		})
	}
}
