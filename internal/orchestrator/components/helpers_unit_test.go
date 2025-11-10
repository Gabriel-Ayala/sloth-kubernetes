package components

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// helperMocks provides mock implementation for helper function tests
type helperMocks int

// NewResource creates mock resources for helper tests
func (helperMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	outputs := args.Inputs.Copy()
	return args.Name + "_id", outputs, nil
}

// Call mocks function calls
func (helperMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return resource.PropertyMap{}, nil
}

// TestGetSSHUserForProvider tests SSH user selection for different providers
func TestGetSSHUserForProvider(t *testing.T) {
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
				result := getSSHUserForProvider(providerOutput)

				// Verify the result
				result.ApplyT(func(user string) error {
					assert.Equal(t, tc.expectedUser, user, "SSH user should match expected for provider %s", tc.provider)
					return nil
				})

				return nil
			}, pulumi.WithMocks("test", "stack", helperMocks(0)))

			assert.NoError(t, err)
		})
	}
}

// TestGetSSHUserForVPNValidator tests VPN validator SSH user selection
func TestGetSSHUserForVPNValidator(t *testing.T) {
	testCases := []struct {
		name         string
		provider     string
		expectedUser string
	}{
		{"Azure VPN", "azure", "azureuser"},
		{"AWS VPN", "aws", "ubuntu"},
		{"GCP VPN", "gcp", "ubuntu"},
		{"DigitalOcean VPN", "digitalocean", "root"},
		{"Linode VPN", "linode", "root"},
		{"Default VPN", "other", "root"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				providerOutput := pulumi.String(tc.provider).ToStringOutput()
				result := getSSHUserForVPNValidator(providerOutput)

				result.ApplyT(func(user string) error {
					assert.Equal(t, tc.expectedUser, user)
					return nil
				})

				return nil
			}, pulumi.WithMocks("test", "stack", helperMocks(0)))

			assert.NoError(t, err)
		})
	}
}

// TestGetSSHUserForWireGuard tests WireGuard SSH user selection
func TestGetSSHUserForWireGuard(t *testing.T) {
	testCases := []struct {
		name         string
		provider     string
		expectedUser string
	}{
		{"Azure WireGuard", "azure", "azureuser"},
		{"AWS WireGuard", "aws", "ubuntu"},
		{"GCP WireGuard", "gcp", "ubuntu"},
		{"DigitalOcean WireGuard", "digitalocean", "root"},
		{"Linode WireGuard", "linode", "root"},
		{"Other provider WireGuard", "vultr", "root"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				providerOutput := pulumi.String(tc.provider).ToStringOutput()
				result := getSSHUserForWireGuard(providerOutput)

				result.ApplyT(func(user string) error {
					assert.Equal(t, tc.expectedUser, user)
					return nil
				})

				return nil
			}, pulumi.WithMocks("test", "stack", helperMocks(0)))

			assert.NoError(t, err)
		})
	}
}

// TestGetSudoPrefixForUser tests sudo prefix selection based on provider
func TestGetSudoPrefixForUser(t *testing.T) {
	testCases := []struct {
		name           string
		provider       string
		expectedPrefix string
	}{
		{"Azure needs sudo", "azure", "sudo "},
		{"AWS needs sudo", "aws", "sudo "},
		{"GCP needs sudo", "gcp", "sudo "},
		{"DigitalOcean root no sudo", "digitalocean", ""},
		{"Linode root no sudo", "linode", ""},
		{"Other provider root no sudo", "other", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				providerOutput := pulumi.String(tc.provider).ToStringOutput()
				result := getSudoPrefixForUser(providerOutput)

				result.ApplyT(func(prefix string) error {
					assert.Equal(t, tc.expectedPrefix, prefix)
					return nil
				})

				return nil
			}, pulumi.WithMocks("test", "stack", helperMocks(0)))

			assert.NoError(t, err)
		})
	}
}

// TestGetSSHUserForProvider_MultipleProviders tests behavior with multiple providers in sequence
func TestGetSSHUserForProvider_MultipleProviders(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		providers := []string{"azure", "aws", "gcp", "digitalocean", "linode"}
		expectedUsers := []string{"azureuser", "ubuntu", "ubuntu", "root", "root"}

		for i, provider := range providers {
			providerOutput := pulumi.String(provider).ToStringOutput()
			result := getSSHUserForProvider(providerOutput)

			expectedUser := expectedUsers[i]
			result.ApplyT(func(user string) error {
				assert.Equal(t, expectedUser, user)
				return nil
			})
		}

		return nil
	}, pulumi.WithMocks("test", "stack", helperMocks(0)))

	assert.NoError(t, err)
}

// TestGetSSHUserForProvider_CaseSensitivity tests case sensitivity of provider names
func TestGetSSHUserForProvider_CaseSensitivity(t *testing.T) {
	testCases := []struct {
		name         string
		provider     string
		expectedUser string
	}{
		{"Lowercase azure", "azure", "azureuser"},
		{"Uppercase AZURE", "AZURE", "root"}, // Should default to root for unknown
		{"Mixed case Azure", "Azure", "root"}, // Should default to root for unknown
		{"Lowercase aws", "aws", "ubuntu"},
		{"Uppercase AWS", "AWS", "root"}, // Should default to root for unknown
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				providerOutput := pulumi.String(tc.provider).ToStringOutput()
				result := getSSHUserForProvider(providerOutput)

				result.ApplyT(func(user string) error {
					assert.Equal(t, tc.expectedUser, user)
					return nil
				})

				return nil
			}, pulumi.WithMocks("test", "stack", helperMocks(0)))

			assert.NoError(t, err)
		})
	}
}

// TestGetSudoPrefixForUser_RootProviders tests that root-based providers don't get sudo
func TestGetSudoPrefixForUser_RootProviders(t *testing.T) {
	rootProviders := []string{"digitalocean", "linode", "vultr", "hetzner", "ovh", "scaleway"}

	for _, provider := range rootProviders {
		t.Run(provider, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				providerOutput := pulumi.String(provider).ToStringOutput()
				result := getSudoPrefixForUser(providerOutput)

				result.ApplyT(func(prefix string) error {
					assert.Empty(t, prefix, "Root provider %s should not have sudo prefix", provider)
					return nil
				})

				return nil
			}, pulumi.WithMocks("test", "stack", helperMocks(0)))

			assert.NoError(t, err)
		})
	}
}

// TestGetSudoPrefixForUser_NonRootProviders tests that non-root providers get sudo
func TestGetSudoPrefixForUser_NonRootProviders(t *testing.T) {
	nonRootProviders := []string{"azure", "aws", "gcp"}

	for _, provider := range nonRootProviders {
		t.Run(provider, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				providerOutput := pulumi.String(provider).ToStringOutput()
				result := getSudoPrefixForUser(providerOutput)

				result.ApplyT(func(prefix string) error {
					assert.Equal(t, "sudo ", prefix, "Non-root provider %s should have sudo prefix", provider)
					return nil
				})

				return nil
			}, pulumi.WithMocks("test", "stack", helperMocks(0)))

			assert.NoError(t, err)
		})
	}
}

// TestGetSSHUserForProvider_ConsistencyAcrossFunctions tests that all three functions return same user
func TestGetSSHUserForProvider_ConsistencyAcrossFunctions(t *testing.T) {
	providers := []string{"azure", "aws", "gcp", "digitalocean", "linode"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				providerOutput := pulumi.String(provider).ToStringOutput()

				result1 := getSSHUserForProvider(providerOutput)
				result2 := getSSHUserForVPNValidator(providerOutput)
				result3 := getSSHUserForWireGuard(providerOutput)

				// All three functions should return the same user for the same provider
				pulumi.All(result1, result2, result3).ApplyT(func(args []interface{}) error {
					user1 := args[0].(string)
					user2 := args[1].(string)
					user3 := args[2].(string)

					assert.Equal(t, user1, user2, "getSSHUserForProvider and getSSHUserForVPNValidator should return same user")
					assert.Equal(t, user1, user3, "getSSHUserForProvider and getSSHUserForWireGuard should return same user")
					assert.Equal(t, user2, user3, "getSSHUserForVPNValidator and getSSHUserForWireGuard should return same user")

					return nil
				})

				return nil
			}, pulumi.WithMocks("test", "stack", helperMocks(0)))

			assert.NoError(t, err)
		})
	}
}

// TestGetSudoPrefixForUser_CorrespondsToSSHUser tests that sudo prefix matches SSH user expectations
func TestGetSudoPrefixForUser_CorrespondsToSSHUser(t *testing.T) {
	testCases := []struct {
		provider    string
		shouldHaveSudo bool
	}{
		{"azure", true},      // azureuser needs sudo
		{"aws", true},        // ubuntu needs sudo
		{"gcp", true},        // ubuntu needs sudo
		{"digitalocean", false}, // root doesn't need sudo
		{"linode", false},    // root doesn't need sudo
	}

	for _, tc := range testCases {
		t.Run(tc.provider, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				providerOutput := pulumi.String(tc.provider).ToStringOutput()

				user := getSSHUserForProvider(providerOutput)
				sudo := getSudoPrefixForUser(providerOutput)

				pulumi.All(user, sudo).ApplyT(func(args []interface{}) error {
					sshUser := args[0].(string)
					sudoPrefix := args[1].(string)

					if tc.shouldHaveSudo {
						assert.NotEmpty(t, sudoPrefix, "Provider %s with user %s should have sudo", tc.provider, sshUser)
						assert.NotEqual(t, "root", sshUser, "Non-root user should have sudo")
					} else {
						assert.Empty(t, sudoPrefix, "Provider %s with user %s should not have sudo", tc.provider, sshUser)
						assert.Equal(t, "root", sshUser, "Root user should not need sudo")
					}

					return nil
				})

				return nil
			}, pulumi.WithMocks("test", "stack", helperMocks(0)))

			assert.NoError(t, err)
		})
	}
}

// TestGetSSHUserForProvider_EdgeCases tests edge cases and special characters
func TestGetSSHUserForProvider_EdgeCases(t *testing.T) {
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
				result := getSSHUserForProvider(providerOutput)

				result.ApplyT(func(user string) error {
					assert.Equal(t, tc.expectedUser, user)
					return nil
				})

				return nil
			}, pulumi.WithMocks("test", "stack", helperMocks(0)))

			assert.NoError(t, err)
		})
	}
}

// TestGetSudoPrefixForUser_Format tests the exact format of sudo prefix
func TestGetSudoPrefixForUser_Format(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// Test that sudo prefix includes trailing space
		azureProvider := pulumi.String("azure").ToStringOutput()
		result := getSudoPrefixForUser(azureProvider)

		result.ApplyT(func(prefix string) error {
			assert.Equal(t, "sudo ", prefix, "Sudo prefix should be 'sudo ' with trailing space")
			assert.Contains(t, prefix, " ", "Sudo prefix should contain space")
			assert.Equal(t, 5, len(prefix), "Sudo prefix should be exactly 5 characters")
			return nil
		})

		// Test that empty prefix is truly empty
		doProvider := pulumi.String("digitalocean").ToStringOutput()
		result2 := getSudoPrefixForUser(doProvider)

		result2.ApplyT(func(prefix string) error {
			assert.Equal(t, "", prefix, "Empty prefix should be empty string")
			assert.Equal(t, 0, len(prefix), "Empty prefix length should be 0")
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", helperMocks(0)))

	assert.NoError(t, err)
}
