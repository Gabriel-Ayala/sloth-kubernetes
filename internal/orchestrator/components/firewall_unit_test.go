package components

import (
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// firewallMocks provides mock implementation for firewall tests
type firewallMocks int

// NewResource creates mock resources for firewall tests
func (firewallMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	outputs := args.Inputs.Copy()

	// Mock firewall resource
	if args.TypeToken == "digitalocean:index/firewall:Firewall" {
		outputs["id"] = resource.NewStringProperty("fw-12345")
		outputs["name"] = resource.NewStringProperty("kubernetes-bastion-fw-test")
		outputs["status"] = resource.NewStringProperty("active")
	}

	return args.Name + "_id", outputs, nil
}

// Call mocks function calls
func (firewallMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return resource.PropertyMap{}, nil
}

// TestNewFirewallComponent tests basic firewall component creation
func TestNewFirewallComponent(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionIP := pulumi.String("192.168.1.10").ToStringOutput()
		nodeDropletIDs := []string{"droplet-1", "droplet-2", "droplet-3"}
		allowedCIDRs := []string{"10.0.0.0/24", "192.168.0.0/16"}

		component, err := NewFirewallComponent(
			ctx,
			"test-firewall",
			bastionIP,
			nodeDropletIDs,
			allowedCIDRs,
		)

		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.NotNil(t, component.FirewallID)
		assert.NotNil(t, component.FirewallName)
		assert.NotNil(t, component.Status)

		return nil
	}, pulumi.WithMocks("test", "stack", firewallMocks(0)))

	assert.NoError(t, err)
}

// TestNewFirewallComponent_WithSingleNode tests firewall with single node
func TestNewFirewallComponent_WithSingleNode(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionIP := pulumi.String("10.20.30.40").ToStringOutput()
		nodeDropletIDs := []string{"single-node-1"}
		allowedCIDRs := []string{"0.0.0.0/0"}

		component, err := NewFirewallComponent(
			ctx,
			"single-node-firewall",
			bastionIP,
			nodeDropletIDs,
			allowedCIDRs,
		)

		assert.NoError(t, err)
		assert.NotNil(t, component)

		// Verify outputs are set
		component.FirewallID.ApplyT(func(id string) error {
			assert.NotEmpty(t, id)
			return nil
		})

		component.FirewallName.ApplyT(func(name string) error {
			assert.NotEmpty(t, name)
			return nil
		})

		component.Status.ApplyT(func(status string) error {
			assert.Equal(t, "active", status)
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", firewallMocks(0)))

	assert.NoError(t, err)
}

// TestNewFirewallComponent_WithMultipleNodes tests firewall with many nodes
func TestNewFirewallComponent_WithMultipleNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionIP := pulumi.String("172.16.0.1").ToStringOutput()
		nodeDropletIDs := []string{
			"node-1", "node-2", "node-3", "node-4", "node-5",
			"node-6", "node-7", "node-8", "node-9", "node-10",
		}
		allowedCIDRs := []string{"10.0.0.0/8"}

		component, err := NewFirewallComponent(
			ctx,
			"multi-node-firewall",
			bastionIP,
			nodeDropletIDs,
			allowedCIDRs,
		)

		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.NotNil(t, component.FirewallID)

		return nil
	}, pulumi.WithMocks("test", "stack", firewallMocks(0)))

	assert.NoError(t, err)
}

// TestNewFirewallComponent_WithEmptyNodeList tests firewall with no nodes
func TestNewFirewallComponent_WithEmptyNodeList(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionIP := pulumi.String("192.168.100.1").ToStringOutput()
		nodeDropletIDs := []string{}
		allowedCIDRs := []string{"0.0.0.0/0"}

		component, err := NewFirewallComponent(
			ctx,
			"empty-node-firewall",
			bastionIP,
			nodeDropletIDs,
			allowedCIDRs,
		)

		// Should still create firewall even with no nodes
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", firewallMocks(0)))

	assert.NoError(t, err)
}

// TestNewFirewallComponent_WithMultipleCIDRs tests firewall with multiple allowed CIDRs
func TestNewFirewallComponent_WithMultipleCIDRs(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionIP := pulumi.String("10.0.0.1").ToStringOutput()
		nodeDropletIDs := []string{"node-1", "node-2"}
		allowedCIDRs := []string{
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"100.64.0.0/10",
		}

		component, err := NewFirewallComponent(
			ctx,
			"multi-cidr-firewall",
			bastionIP,
			nodeDropletIDs,
			allowedCIDRs,
		)

		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", firewallMocks(0)))

	assert.NoError(t, err)
}

// TestNewFirewallComponent_WithEmptyCIDRList tests firewall with no allowed CIDRs
func TestNewFirewallComponent_WithEmptyCIDRList(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionIP := pulumi.String("10.1.2.3").ToStringOutput()
		nodeDropletIDs := []string{"node-1"}
		allowedCIDRs := []string{}

		component, err := NewFirewallComponent(
			ctx,
			"no-cidr-firewall",
			bastionIP,
			nodeDropletIDs,
			allowedCIDRs,
		)

		// Should create firewall with default rules
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", firewallMocks(0)))

	assert.NoError(t, err)
}

// TestNewFirewallComponent_BastionIPFormats tests different bastion IP formats
func TestNewFirewallComponent_BastionIPFormats(t *testing.T) {
	testCases := []struct {
		name       string
		bastionIP  string
		shouldPass bool
	}{
		{"IPv4 private", "10.0.0.1", true},
		{"IPv4 public", "203.0.113.1", true},
		{"IPv4 local", "127.0.0.1", true},
		{"IPv4 zero", "0.0.0.0", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				bastionIP := pulumi.String(tc.bastionIP).ToStringOutput()
				nodeDropletIDs := []string{"node-1"}
				allowedCIDRs := []string{"0.0.0.0/0"}

				component, err := NewFirewallComponent(
					ctx,
					"ip-format-firewall",
					bastionIP,
					nodeDropletIDs,
					allowedCIDRs,
				)

				if tc.shouldPass {
					assert.NoError(t, err)
					assert.NotNil(t, component)
				}

				return nil
			}, pulumi.WithMocks("test", "stack", firewallMocks(0)))

			assert.NoError(t, err)
		})
	}
}

// TestNewFirewallComponent_OutputValues tests firewall output values
func TestNewFirewallComponent_OutputValues(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionIP := pulumi.String("192.168.1.1").ToStringOutput()
		nodeDropletIDs := []string{"node-1", "node-2"}
		allowedCIDRs := []string{"10.0.0.0/24"}

		component, err := NewFirewallComponent(
			ctx,
			"output-test-firewall",
			bastionIP,
			nodeDropletIDs,
			allowedCIDRs,
		)

		assert.NoError(t, err)
		assert.NotNil(t, component)

		// Verify all outputs are present
		pulumi.All(
			component.FirewallID,
			component.FirewallName,
			component.Status,
		).ApplyT(func(args []interface{}) error {
			firewallID := args[0].(string)
			firewallName := args[1].(string)
			status := args[2].(string)

			assert.NotEmpty(t, firewallID)
			assert.NotEmpty(t, firewallName)
			assert.Equal(t, "active", status)

			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", firewallMocks(0)))

	assert.NoError(t, err)
}

// TestNewFirewallComponent_ResourceType tests correct resource type registration
func TestNewFirewallComponent_ResourceType(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionIP := pulumi.String("10.20.30.40").ToStringOutput()
		nodeDropletIDs := []string{"node-1"}
		allowedCIDRs := []string{"0.0.0.0/0"}

		component, err := NewFirewallComponent(
			ctx,
			"resource-type-test",
			bastionIP,
			nodeDropletIDs,
			allowedCIDRs,
		)

		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.Implements(t, (*pulumi.Resource)(nil), component)

		return nil
	}, pulumi.WithMocks("test", "stack", firewallMocks(0)))

	assert.NoError(t, err)
}

// TestNewFirewallComponent_WithNilAllowedCIDRs tests firewall with nil CIDRs
func TestNewFirewallComponent_WithNilAllowedCIDRs(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionIP := pulumi.String("192.168.50.1").ToStringOutput()
		nodeDropletIDs := []string{"node-1", "node-2"}
		var allowedCIDRs []string = nil

		component, err := NewFirewallComponent(
			ctx,
			"nil-cidr-firewall",
			bastionIP,
			nodeDropletIDs,
			allowedCIDRs,
		)

		// Should handle nil CIDRs gracefully
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", firewallMocks(0)))

	assert.NoError(t, err)
}

// TestNewFirewallComponent_WithNilNodeList tests firewall with nil node list
func TestNewFirewallComponent_WithNilNodeList(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionIP := pulumi.String("10.100.200.1").ToStringOutput()
		var nodeDropletIDs []string = nil
		allowedCIDRs := []string{"0.0.0.0/0"}

		component, err := NewFirewallComponent(
			ctx,
			"nil-node-firewall",
			bastionIP,
			nodeDropletIDs,
			allowedCIDRs,
		)

		// Should handle nil node list gracefully
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", firewallMocks(0)))

	assert.NoError(t, err)
}

// TestNewFirewallComponent_StatusVerification tests status is always active
func TestNewFirewallComponent_StatusVerification(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionIP := pulumi.String("172.20.30.40").ToStringOutput()
		nodeDropletIDs := []string{"node-1"}
		allowedCIDRs := []string{"10.0.0.0/8"}

		component, err := NewFirewallComponent(
			ctx,
			"status-verify-firewall",
			bastionIP,
			nodeDropletIDs,
			allowedCIDRs,
		)

		assert.NoError(t, err)
		assert.NotNil(t, component)

		// Verify status is "active"
		component.Status.ApplyT(func(status string) error {
			assert.Equal(t, "active", status, "Firewall status should be 'active'")
			assert.NotEqual(t, "pending", status)
			assert.NotEqual(t, "failed", status)
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", firewallMocks(0)))

	assert.NoError(t, err)
}

// TestNewFirewallComponent_ComponentNaming tests component naming convention
func TestNewFirewallComponent_ComponentNaming(t *testing.T) {
	testNames := []string{
		"production-firewall",
		"staging-firewall",
		"dev-firewall",
		"test-firewall-1",
		"firewall-with-dashes",
	}

	for _, name := range testNames {
		t.Run(name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				bastionIP := pulumi.String("10.0.0.1").ToStringOutput()
				nodeDropletIDs := []string{"node-1"}
				allowedCIDRs := []string{"0.0.0.0/0"}

				component, err := NewFirewallComponent(
					ctx,
					name,
					bastionIP,
					nodeDropletIDs,
					allowedCIDRs,
				)

				assert.NoError(t, err)
				assert.NotNil(t, component)

				return nil
			}, pulumi.WithMocks("test", "stack", firewallMocks(0)))

			assert.NoError(t, err)
		})
	}
}

// TestNewFirewallComponent_LargeNodePool tests firewall with very large node pool
func TestNewFirewallComponent_LargeNodePool(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		bastionIP := pulumi.String("10.0.0.1").ToStringOutput()

		// Create large node pool (100 nodes)
		nodeDropletIDs := make([]string, 100)
		for i := 0; i < 100; i++ {
			nodeDropletIDs[i] = fmt.Sprintf("node-%d", i+1)
		}

		allowedCIDRs := []string{"0.0.0.0/0"}

		component, err := NewFirewallComponent(
			ctx,
			"large-pool-firewall",
			bastionIP,
			nodeDropletIDs,
			allowedCIDRs,
		)

		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", firewallMocks(0)))

	assert.NoError(t, err)
}

// TestNewFirewallComponent_PrivateIPRanges tests firewall with various private IP ranges
func TestNewFirewallComponent_PrivateIPRanges(t *testing.T) {
	privateRanges := []struct {
		name string
		cidr string
	}{
		{"Class A private", "10.0.0.0/8"},
		{"Class B private", "172.16.0.0/12"},
		{"Class C private", "192.168.0.0/16"},
		{"Carrier-grade NAT", "100.64.0.0/10"},
		{"Localhost", "127.0.0.0/8"},
		{"Link-local", "169.254.0.0/16"},
	}

	for _, pr := range privateRanges {
		t.Run(pr.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				bastionIP := pulumi.String("10.0.0.1").ToStringOutput()
				nodeDropletIDs := []string{"node-1"}
				allowedCIDRs := []string{pr.cidr}

				component, err := NewFirewallComponent(
					ctx,
					"private-range-firewall",
					bastionIP,
					nodeDropletIDs,
					allowedCIDRs,
				)

				assert.NoError(t, err)
				assert.NotNil(t, component)

				return nil
			}, pulumi.WithMocks("test", "stack", firewallMocks(0)))

			assert.NoError(t, err)
		})
	}
}
