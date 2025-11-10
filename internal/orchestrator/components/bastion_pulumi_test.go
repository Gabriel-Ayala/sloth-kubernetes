package components

import (
	"fmt"
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

type bastionMocks int

// NewResource creates mock resources for Bastion tests
func (bastionMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	outputs := args.Inputs.Copy()

	switch args.TypeToken {
	case "kubernetes-create:security:Bastion":
		outputs["bastionName"] = resource.NewStringProperty("bastion")
		outputs["publicIP"] = resource.NewStringProperty("198.51.100.50")
		outputs["privateIP"] = resource.NewStringProperty("10.0.0.5")
		outputs["wireGuardIP"] = resource.NewStringProperty("10.8.0.5")
		outputs["provider"] = resource.NewStringProperty("digitalocean")
		outputs["region"] = resource.NewStringProperty("nyc1")
		outputs["sshPort"] = resource.NewNumberProperty(22)
		outputs["status"] = resource.NewStringProperty("active")

	case "digitalocean:index/sshKey:SshKey":
		outputs["id"] = resource.NewStringProperty("ssh-key-" + args.Name)
		outputs["fingerprint"] = resource.NewStringProperty("ab:cd:ef:12:34:56")
		outputs["publicKey"] = args.Inputs["publicKey"]
		outputs["name"] = args.Inputs["name"]

	case "digitalocean:index/droplet:Droplet":
		outputs["id"] = resource.NewStringProperty("droplet-" + args.Name)
		outputs["ipv4Address"] = resource.NewStringProperty("198.51.100.50")
		outputs["ipv4AddressPrivate"] = resource.NewStringProperty("10.0.0.5")
		outputs["status"] = resource.NewStringProperty("active")
		outputs["urn"] = resource.NewStringProperty("do:droplet:" + args.Name)

	case "linode:index/instance:Instance":
		outputs["id"] = resource.NewStringProperty("linode-" + args.Name)
		outputs["ipAddress"] = resource.NewStringProperty("198.51.100.51")
		outputs["privateIpAddress"] = resource.NewStringProperty("192.168.0.5")
		outputs["status"] = resource.NewStringProperty("running")

	case "azure-native:resources/v2:ResourceGroup":
		outputs["id"] = resource.NewStringProperty("/subscriptions/sub/resourceGroups/" + args.Name)
		outputs["name"] = args.Inputs["resourceGroupName"]
		outputs["location"] = args.Inputs["location"]

	case "azure-native:network/v2:VirtualNetwork":
		outputs["id"] = resource.NewStringProperty("/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/" + args.Name)
		outputs["name"] = args.Inputs["virtualNetworkName"]

	case "azure-native:network/v2:Subnet":
		outputs["id"] = resource.NewStringProperty("/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/" + args.Name)
		outputs["name"] = args.Inputs["subnetName"]

	case "azure-native:network/v2:NetworkSecurityGroup":
		outputs["id"] = resource.NewStringProperty("/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/networkSecurityGroups/" + args.Name)
		outputs["name"] = args.Inputs["networkSecurityGroupName"]

	case "azure-native:network/v2:PublicIPAddress":
		outputs["id"] = resource.NewStringProperty("/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/publicIPAddresses/" + args.Name)
		outputs["ipAddress"] = resource.NewStringProperty("198.51.100.52")

	case "azure-native:network/v2:NetworkInterface":
		outputs["id"] = resource.NewStringProperty("/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/networkInterfaces/" + args.Name)
		outputs["ipConfigurations"] = resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewObjectProperty(resource.PropertyMap{
				"privateIPAddress": resource.NewStringProperty("10.0.0.5"),
			}),
		})

	case "azure-native:compute/v2:VirtualMachine":
		outputs["id"] = resource.NewStringProperty("/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/" + args.Name)
		outputs["name"] = args.Inputs["vmName"]

	case "command:remote:Command":
		outputs["stdout"] = resource.NewStringProperty("Command executed successfully")
		outputs["stderr"] = resource.NewStringProperty("")
	}

	return args.Name + "_id", outputs, nil
}

// Call mocks function calls
func (bastionMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return resource.PropertyMap{}, nil
}

func TestNewBastionComponent_Disabled(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionConfig := &config.BastionConfig{
			Enabled: false,
		}

		sshKeyOutput := pulumi.String("ssh-rsa AAAAB3... test@example.com").ToStringOutput()
		sshPrivateKey := pulumi.String("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----").ToStringOutput()
		doToken := pulumi.String("mock-do-token")
		linodeToken := pulumi.String("mock-linode-token")

		component, err := NewBastionComponent(
			ctx,
			"test-bastion",
			bastionConfig,
			sshKeyOutput,
			sshPrivateKey,
			doToken,
			linodeToken,
		)

		assert.NoError(t, err, "Creating disabled bastion should not error")
		assert.NotNil(t, component, "Component should not be nil")

		// Verify status is disabled
		component.Status.ApplyT(func(status string) error {
			assert.Equal(t, "disabled", status, "Status should be disabled")
			return nil
		})

		return nil
	}, pulumi.WithMocks("project", "stack", bastionMocks(0)))

	assert.NoError(t, err)
}

func TestNewBastionComponent_DigitalOcean(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionConfig := &config.BastionConfig{
			Enabled:  true,
			Provider: "digitalocean",
			Region:   "nyc1",
			Size:     "s-1vcpu-1gb",
			Image:    "ubuntu-22-04-x64",
			Name:     "bastion-do",
			SSHPort:  22,
		}

		sshKeyOutput := pulumi.String("ssh-rsa AAAAB3... test@example.com").ToStringOutput()
		sshPrivateKey := pulumi.String("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----").ToStringOutput()
		doToken := pulumi.String("mock-do-token")
		linodeToken := pulumi.String("mock-linode-token")

		component, err := NewBastionComponent(
			ctx,
			"test-bastion-do",
			bastionConfig,
			sshKeyOutput,
			sshPrivateKey,
			doToken,
			linodeToken,
		)

		assert.NoError(t, err, "Creating DigitalOcean bastion should not error")
		assert.NotNil(t, component, "Component should not be nil")

		// Verify outputs
		component.BastionName.ApplyT(func(name string) error {
			assert.Equal(t, "bastion-do", name, "Bastion name should match")
			return nil
		})

		component.Provider.ApplyT(func(provider string) error {
			assert.Equal(t, "digitalocean", provider, "Provider should be digitalocean")
			return nil
		})

		component.Region.ApplyT(func(region string) error {
			assert.Equal(t, "nyc1", region, "Region should match")
			return nil
		})

		component.WireGuardIP.ApplyT(func(ip string) error {
			assert.Equal(t, "10.8.0.5", ip, "WireGuard IP should be 10.8.0.5")
			return nil
		})

		return nil
	}, pulumi.WithMocks("project", "stack", bastionMocks(0)))

	assert.NoError(t, err)
}

func TestNewBastionComponent_Linode(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionConfig := &config.BastionConfig{
			Enabled:  true,
			Provider: "linode",
			Region:   "us-east",
			Size:     "g6-nanode-1",
			Image:    "linode/ubuntu22.04",
			Name:     "bastion-linode",
			SSHPort:  2222, // Custom SSH port
		}

		sshKeyOutput := pulumi.String("ssh-rsa AAAAB3... test@example.com").ToStringOutput()
		sshPrivateKey := pulumi.String("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----").ToStringOutput()
		doToken := pulumi.String("mock-do-token")
		linodeToken := pulumi.String("mock-linode-token")

		component, err := NewBastionComponent(
			ctx,
			"test-bastion-linode",
			bastionConfig,
			sshKeyOutput,
			sshPrivateKey,
			doToken,
			linodeToken,
		)

		assert.NoError(t, err, "Creating Linode bastion should not error")
		assert.NotNil(t, component, "Component should not be nil")

		// Verify custom SSH port
		component.SSHPort.ApplyT(func(port int) error {
			assert.Equal(t, 2222, port, "SSH port should be custom 2222")
			return nil
		})

		component.Provider.ApplyT(func(provider string) error {
			assert.Equal(t, "linode", provider, "Provider should be linode")
			return nil
		})

		return nil
	}, pulumi.WithMocks("project", "stack", bastionMocks(0)))

	assert.NoError(t, err)
}

func TestNewBastionComponent_Azure(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionConfig := &config.BastionConfig{
			Enabled:  true,
			Provider: "azure",
			Region:   "eastus",
			Size:     "Standard_B1s",
			Image:    "ubuntu-22.04",
			Name:     "bastion-azure",
		}

		sshKeyOutput := pulumi.String("ssh-rsa AAAAB3... test@example.com").ToStringOutput()
		sshPrivateKey := pulumi.String("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----").ToStringOutput()
		doToken := pulumi.String("mock-do-token")
		linodeToken := pulumi.String("mock-linode-token")

		component, err := NewBastionComponent(
			ctx,
			"test-bastion-azure",
			bastionConfig,
			sshKeyOutput,
			sshPrivateKey,
			doToken,
			linodeToken,
		)

		assert.NoError(t, err, "Creating Azure bastion should not error")
		assert.NotNil(t, component, "Component should not be nil")

		component.Provider.ApplyT(func(provider string) error {
			assert.Equal(t, "azure", provider, "Provider should be azure")
			return nil
		})

		return nil
	}, pulumi.WithMocks("project", "stack", bastionMocks(0)))

	assert.NoError(t, err)
}

func TestNewBastionComponent_UnsupportedProvider(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionConfig := &config.BastionConfig{
			Enabled:  true,
			Provider: "aws", // Unsupported provider
			Region:   "us-east-1",
			Size:     "t2.micro",
		}

		sshKeyOutput := pulumi.String("ssh-rsa AAAAB3... test@example.com").ToStringOutput()
		sshPrivateKey := pulumi.String("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----").ToStringOutput()
		doToken := pulumi.String("mock-do-token")
		linodeToken := pulumi.String("mock-linode-token")

		_, err := NewBastionComponent(
			ctx,
			"test-bastion-unsupported",
			bastionConfig,
			sshKeyOutput,
			sshPrivateKey,
			doToken,
			linodeToken,
		)

		assert.Error(t, err, "Should return error for unsupported provider")
		assert.Contains(t, err.Error(), "unsupported bastion provider", "Error should mention unsupported provider")

		return nil
	}, pulumi.WithMocks("project", "stack", bastionMocks(0)))

	assert.NoError(t, err)
}

func TestNewBastionComponent_DefaultValues(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionConfig := &config.BastionConfig{
			Enabled:  true,
			Provider: "digitalocean",
			Region:   "nyc1",
			Size:     "s-1vcpu-1gb",
			// Name and SSHPort not specified - should use defaults
		}

		sshKeyOutput := pulumi.String("ssh-rsa AAAAB3... test@example.com").ToStringOutput()
		sshPrivateKey := pulumi.String("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----").ToStringOutput()
		doToken := pulumi.String("mock-do-token")
		linodeToken := pulumi.String("mock-linode-token")

		component, err := NewBastionComponent(
			ctx,
			"test-bastion-defaults",
			bastionConfig,
			sshKeyOutput,
			sshPrivateKey,
			doToken,
			linodeToken,
		)

		assert.NoError(t, err)
		assert.NotNil(t, component)

		// Verify defaults are applied
		component.BastionName.ApplyT(func(name string) error {
			assert.Equal(t, "bastion", name, "Should use default name 'bastion'")
			return nil
		})

		component.SSHPort.ApplyT(func(port int) error {
			assert.Equal(t, 22, port, "Should use default SSH port 22")
			return nil
		})

		return nil
	}, pulumi.WithMocks("project", "stack", bastionMocks(0)))

	assert.NoError(t, err)
}

func TestNewBastionComponent_WithVPNOnly(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionConfig := &config.BastionConfig{
			Enabled:  true,
			Provider: "digitalocean",
			Region:   "nyc1",
			Size:     "s-1vcpu-1gb",
			VPNOnly:  true, // Only accessible via VPN
		}

		sshKeyOutput := pulumi.String("ssh-rsa AAAAB3... test@example.com").ToStringOutput()
		sshPrivateKey := pulumi.String("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----").ToStringOutput()
		doToken := pulumi.String("mock-do-token")
		linodeToken := pulumi.String("mock-linode-token")

		component, err := NewBastionComponent(
			ctx,
			"test-bastion-vpn-only",
			bastionConfig,
			sshKeyOutput,
			sshPrivateKey,
			doToken,
			linodeToken,
		)

		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("project", "stack", bastionMocks(0)))

	assert.NoError(t, err)
}

func TestNewBastionComponent_WithAllowedCIDRs(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionConfig := &config.BastionConfig{
			Enabled:      true,
			Provider:     "digitalocean",
			Region:       "nyc1",
			Size:         "s-1vcpu-1gb",
			AllowedCIDRs: []string{"192.168.1.0/24", "10.0.0.0/8"}, // Restrict SSH access
		}

		sshKeyOutput := pulumi.String("ssh-rsa AAAAB3... test@example.com").ToStringOutput()
		sshPrivateKey := pulumi.String("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----").ToStringOutput()
		doToken := pulumi.String("mock-do-token")
		linodeToken := pulumi.String("mock-linode-token")

		component, err := NewBastionComponent(
			ctx,
			"test-bastion-restricted",
			bastionConfig,
			sshKeyOutput,
			sshPrivateKey,
			doToken,
			linodeToken,
		)

		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("project", "stack", bastionMocks(0)))

	assert.NoError(t, err)
}

func TestNewBastionComponent_WithAuditLog(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionConfig := &config.BastionConfig{
			Enabled:        true,
			Provider:       "linode",
			Region:         "us-east",
			Size:           "g6-nanode-1",
			EnableAuditLog: true, // Enable SSH session auditing
		}

		sshKeyOutput := pulumi.String("ssh-rsa AAAAB3... test@example.com").ToStringOutput()
		sshPrivateKey := pulumi.String("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----").ToStringOutput()
		doToken := pulumi.String("mock-do-token")
		linodeToken := pulumi.String("mock-linode-token")

		component, err := NewBastionComponent(
			ctx,
			"test-bastion-audit",
			bastionConfig,
			sshKeyOutput,
			sshPrivateKey,
			doToken,
			linodeToken,
		)

		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("project", "stack", bastionMocks(0)))

	assert.NoError(t, err)
}

func TestNewBastionComponent_MultipleProviders(t *testing.T) {
	providers := []struct {
		name     string
		provider string
		region   string
		size     string
	}{
		{"DigitalOcean", "digitalocean", "nyc1", "s-1vcpu-1gb"},
		{"Linode", "linode", "us-east", "g6-nanode-1"},
		{"Azure", "azure", "eastus", "Standard_B1s"},
	}

	for _, tt := range providers {
		t.Run(tt.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				bastionConfig := &config.BastionConfig{
					Enabled:  true,
					Provider: tt.provider,
					Region:   tt.region,
					Size:     tt.size,
				}

				sshKeyOutput := pulumi.String("ssh-rsa AAAAB3... test@example.com").ToStringOutput()
				sshPrivateKey := pulumi.String("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----").ToStringOutput()
				doToken := pulumi.String("mock-do-token")
				linodeToken := pulumi.String("mock-linode-token")

				component, err := NewBastionComponent(
					ctx,
					"test-bastion-"+tt.name,
					bastionConfig,
					sshKeyOutput,
					sshPrivateKey,
					doToken,
					linodeToken,
				)

				assert.NoError(t, err, fmt.Sprintf("Creating %s bastion should not error", tt.name))
				assert.NotNil(t, component)

				component.Provider.ApplyT(func(provider string) error {
					assert.Equal(t, tt.provider, provider)
					return nil
				})

				return nil
			}, pulumi.WithMocks("project", "stack", bastionMocks(0)))

			assert.NoError(t, err)
		})
	}
}
