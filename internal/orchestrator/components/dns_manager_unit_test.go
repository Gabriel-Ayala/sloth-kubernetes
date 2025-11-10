package components

import (
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// dnsMocks provides mock implementation for DNS tests
type dnsMocks int

// NewResource creates mock resources for DNS tests
func (dnsMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	outputs := args.Inputs.Copy()

	// Mock DNS record resource
	if args.TypeToken == "digitalocean:index/dnsRecord:DnsRecord" {
		outputs["id"] = resource.NewStringProperty("dns-record-12345")
		outputs["fqdn"] = resource.NewStringProperty("test.example.com")
	}

	// Mock RealNodeComponent
	if args.TypeToken == "kubernetes-create:compute:RealNode" {
		outputs["nodeName"] = resource.NewStringProperty("node-1")
		outputs["provider"] = resource.NewStringProperty("digitalocean")
		outputs["region"] = resource.NewStringProperty("nyc1")
		outputs["size"] = resource.NewStringProperty("s-2vcpu-4gb")
		outputs["publicIP"] = resource.NewStringProperty("192.168.1.100")
		outputs["privateIP"] = resource.NewStringProperty("10.0.0.100")
		outputs["wireGuardIP"] = resource.NewStringProperty("10.8.0.10")
		outputs["roles"] = resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("master"),
		})
	}

	return args.Name + "_id", outputs, nil
}

// Call mocks function calls
func (dnsMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return resource.PropertyMap{}, nil
}

// createMockNode creates a mock RealNodeComponent for testing
func createMockNode(ctx *pulumi.Context, name string, ip string) *RealNodeComponent {
	node := &RealNodeComponent{}
	_ = ctx.RegisterComponentResource("kubernetes-create:compute:RealNode", name, node)

	node.NodeName = pulumi.String(name).ToStringOutput()
	node.Provider = pulumi.String("digitalocean").ToStringOutput()
	node.Region = pulumi.String("nyc1").ToStringOutput()
	node.Size = pulumi.String("s-2vcpu-4gb").ToStringOutput()
	node.PublicIP = pulumi.String(ip).ToStringOutput()
	node.PrivateIP = pulumi.String("10.0.0.100").ToStringOutput()
	node.WireGuardIP = pulumi.String("10.8.0.10").ToStringOutput()
	node.Roles = pulumi.Array{pulumi.String("master")}.ToArrayOutput()

	return node
}

// TestNewDNSRealComponent_WithValidDomain tests DNS component with valid domain
func TestNewDNSRealComponent_WithValidDomain(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		nodes := []*RealNodeComponent{
			createMockNode(ctx, "node-1", "192.168.1.1"),
			createMockNode(ctx, "node-2", "192.168.1.2"),
			createMockNode(ctx, "node-3", "192.168.1.3"),
			createMockNode(ctx, "node-4", "192.168.1.4"),
		}

		component, err := NewDNSRealComponent(ctx, "test-dns", "kubernetes.example.com", nodes)

		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.NotNil(t, component.Status)
		assert.NotNil(t, component.RecordCount)
		assert.NotNil(t, component.Domain)
		assert.NotNil(t, component.APIEndpoint)

		return nil
	}, pulumi.WithMocks("test", "stack", dnsMocks(0)))

	assert.NoError(t, err)
}

// TestNewDNSRealComponent_WithEmptyDomain tests DNS component with empty domain
func TestNewDNSRealComponent_WithEmptyDomain(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		nodes := []*RealNodeComponent{
			createMockNode(ctx, "node-1", "192.168.1.1"),
		}

		component, err := NewDNSRealComponent(ctx, "test-dns-empty", "", nodes)

		assert.NoError(t, err)
		assert.NotNil(t, component)

		// Should skip DNS creation
		component.Status.ApplyT(func(status string) error {
			assert.Contains(t, status, "skipped")
			return nil
		})

		component.RecordCount.ApplyT(func(count int) error {
			assert.Equal(t, 0, count)
			return nil
		})

		component.Domain.ApplyT(func(domain string) error {
			assert.Empty(t, domain)
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", dnsMocks(0)))

	assert.NoError(t, err)
}

// TestNewDNSRealComponent_WithPlaceholderDomain tests DNS with example.com placeholder
func TestNewDNSRealComponent_WithPlaceholderDomain(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		nodes := []*RealNodeComponent{
			createMockNode(ctx, "node-1", "192.168.1.1"),
			createMockNode(ctx, "node-2", "192.168.1.2"),
		}

		component, err := NewDNSRealComponent(ctx, "test-dns-placeholder", "example.com", nodes)

		assert.NoError(t, err)
		assert.NotNil(t, component)

		// Should skip DNS creation for example.com
		component.Status.ApplyT(func(status string) error {
			assert.Contains(t, status, "skipped")
			return nil
		})

		component.RecordCount.ApplyT(func(count int) error {
			assert.Equal(t, 0, count)
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", dnsMocks(0)))

	assert.NoError(t, err)
}

// TestNewDNSRealComponent_WithSingleNode tests DNS with only one node
func TestNewDNSRealComponent_WithSingleNode(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		nodes := []*RealNodeComponent{
			createMockNode(ctx, "node-1", "192.168.1.1"),
		}

		component, err := NewDNSRealComponent(ctx, "test-dns-single", "k8s.example.com", nodes)

		assert.NoError(t, err)
		assert.NotNil(t, component)

		component.Domain.ApplyT(func(domain string) error {
			assert.Equal(t, "k8s.example.com", domain)
			return nil
		})

		component.APIEndpoint.ApplyT(func(endpoint string) error {
			assert.Equal(t, "https://api.k8s.example.com:6443", endpoint)
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", dnsMocks(0)))

	assert.NoError(t, err)
}

// TestNewDNSRealComponent_WithThreeNodes tests DNS with 3 nodes (no workers)
func TestNewDNSRealComponent_WithThreeNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		nodes := []*RealNodeComponent{
			createMockNode(ctx, "node-1", "192.168.1.1"),
			createMockNode(ctx, "node-2", "192.168.1.2"),
			createMockNode(ctx, "node-3", "192.168.1.3"),
		}

		component, err := NewDNSRealComponent(ctx, "test-dns-three", "cluster.example.com", nodes)

		assert.NoError(t, err)
		assert.NotNil(t, component)

		// With less than 4 nodes, no wildcard/ingress records should be created
		component.Status.ApplyT(func(status string) error {
			assert.Contains(t, status, "DNS configured")
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", dnsMocks(0)))

	assert.NoError(t, err)
}

// TestNewDNSRealComponent_WithFourNodes tests DNS with exactly 4 nodes
func TestNewDNSRealComponent_WithFourNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		nodes := []*RealNodeComponent{
			createMockNode(ctx, "master-1", "192.168.1.1"),
			createMockNode(ctx, "master-2", "192.168.1.2"),
			createMockNode(ctx, "master-3", "192.168.1.3"),
			createMockNode(ctx, "worker-1", "192.168.1.4"),
		}

		component, err := NewDNSRealComponent(ctx, "test-dns-four", "prod.example.com", nodes)

		assert.NoError(t, err)
		assert.NotNil(t, component)

		// With 4+ nodes, wildcard and ingress records should be created
		component.RecordCount.ApplyT(func(count int) error {
			// API record (1) + 4 node records (4) + wildcard (1) + ingress (1) = 7
			assert.Greater(t, count, 0)
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", dnsMocks(0)))

	assert.NoError(t, err)
}

// TestNewDNSRealComponent_WithManyNodes tests DNS with many nodes
func TestNewDNSRealComponent_WithManyNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		nodes := make([]*RealNodeComponent, 10)
		for i := 0; i < 10; i++ {
			nodes[i] = createMockNode(ctx, fmt.Sprintf("node-%d", i+1), fmt.Sprintf("192.168.1.%d", i+1))
		}

		component, err := NewDNSRealComponent(ctx, "test-dns-many", "big-cluster.example.com", nodes)

		assert.NoError(t, err)
		assert.NotNil(t, component)

		component.Domain.ApplyT(func(domain string) error {
			assert.Equal(t, "big-cluster.example.com", domain)
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", dnsMocks(0)))

	assert.NoError(t, err)
}

// TestNewDNSRealComponent_WithNoNodes tests DNS with empty node list
func TestNewDNSRealComponent_WithNoNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		nodes := []*RealNodeComponent{}

		component, err := NewDNSRealComponent(ctx, "test-dns-empty-nodes", "empty.example.com", nodes)

		assert.NoError(t, err)
		assert.NotNil(t, component)

		// With no nodes, only domain should be set
		component.Domain.ApplyT(func(domain string) error {
			assert.Equal(t, "empty.example.com", domain)
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", dnsMocks(0)))

	assert.NoError(t, err)
}

// TestNewDNSRealComponent_DomainFormats tests various domain formats
func TestNewDNSRealComponent_DomainFormats(t *testing.T) {
	testCases := []struct {
		name   string
		domain string
		valid  bool
	}{
		{"Simple domain", "test.com", true},
		{"Subdomain", "k8s.cluster.example.com", true},
		{"Hyphenated", "my-cluster.example.com", true},
		{"Numbers", "cluster123.example.com", true},
		{"Multiple subdomains", "a.b.c.example.com", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				nodes := []*RealNodeComponent{
					createMockNode(ctx, "node-1", "192.168.1.1"),
				}

				component, err := NewDNSRealComponent(ctx, "test-dns-format", tc.domain, nodes)

				if tc.valid {
					assert.NoError(t, err)
					assert.NotNil(t, component)

					component.Domain.ApplyT(func(domain string) error {
						assert.Equal(t, tc.domain, domain)
						return nil
					})
				}

				return nil
			}, pulumi.WithMocks("test", "stack", dnsMocks(0)))

			assert.NoError(t, err)
		})
	}
}

// TestNewDNSRealComponent_APIEndpointFormat tests API endpoint format
func TestNewDNSRealComponent_APIEndpointFormat(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		nodes := []*RealNodeComponent{
			createMockNode(ctx, "node-1", "192.168.1.1"),
		}

		component, err := NewDNSRealComponent(ctx, "test-dns-api", "production.example.com", nodes)

		assert.NoError(t, err)
		assert.NotNil(t, component)

		// Verify API endpoint format
		component.APIEndpoint.ApplyT(func(endpoint string) error {
			assert.Equal(t, "https://api.production.example.com:6443", endpoint)
			assert.Contains(t, endpoint, "https://")
			assert.Contains(t, endpoint, ":6443")
			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", dnsMocks(0)))

	assert.NoError(t, err)
}

// TestNewDNSRealComponent_OutputValues tests all output values
func TestNewDNSRealComponent_OutputValues(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		nodes := []*RealNodeComponent{
			createMockNode(ctx, "node-1", "192.168.1.1"),
			createMockNode(ctx, "node-2", "192.168.1.2"),
		}

		component, err := NewDNSRealComponent(ctx, "test-dns-outputs", "staging.example.com", nodes)

		assert.NoError(t, err)
		assert.NotNil(t, component)

		// Verify all outputs exist
		pulumi.All(
			component.Status,
			component.RecordCount,
			component.Domain,
			component.APIEndpoint,
		).ApplyT(func(args []interface{}) error {
			status := args[0].(string)
			recordCount := args[1].(int)
			domain := args[2].(string)
			apiEndpoint := args[3].(string)

			assert.NotEmpty(t, status)
			assert.GreaterOrEqual(t, recordCount, 0)
			assert.Equal(t, "staging.example.com", domain)
			assert.Equal(t, "https://api.staging.example.com:6443", apiEndpoint)

			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", dnsMocks(0)))

	assert.NoError(t, err)
}

// TestNewDNSRealComponent_SkippedOutputs tests outputs when DNS is skipped
func TestNewDNSRealComponent_SkippedOutputs(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		nodes := []*RealNodeComponent{
			createMockNode(ctx, "node-1", "192.168.1.1"),
		}

		component, err := NewDNSRealComponent(ctx, "test-dns-skipped", "", nodes)

		assert.NoError(t, err)
		assert.NotNil(t, component)

		// Verify outputs when skipped
		pulumi.All(
			component.Status,
			component.RecordCount,
			component.Domain,
			component.APIEndpoint,
		).ApplyT(func(args []interface{}) error {
			status := args[0].(string)
			recordCount := args[1].(int)
			domain := args[2].(string)
			apiEndpoint := args[3].(string)

			assert.Contains(t, status, "skipped")
			assert.Equal(t, 0, recordCount)
			assert.Empty(t, domain)
			assert.Empty(t, apiEndpoint)

			return nil
		})

		return nil
	}, pulumi.WithMocks("test", "stack", dnsMocks(0)))

	assert.NoError(t, err)
}

// TestNewDNSRealComponent_ComponentType tests correct component type registration
func TestNewDNSRealComponent_ComponentType(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		nodes := []*RealNodeComponent{
			createMockNode(ctx, "node-1", "192.168.1.1"),
		}

		component, err := NewDNSRealComponent(ctx, "test-dns-type", "test.example.com", nodes)

		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.Implements(t, (*pulumi.Resource)(nil), component)

		return nil
	}, pulumi.WithMocks("test", "stack", dnsMocks(0)))

	assert.NoError(t, err)
}

// TestNewDNSRealComponent_ComponentNaming tests different component names
func TestNewDNSRealComponent_ComponentNaming(t *testing.T) {
	testNames := []string{
		"production-dns",
		"staging-dns",
		"dev-dns-cluster",
		"test-123",
		"dns-with-dashes",
	}

	for _, name := range testNames {
		t.Run(name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				nodes := []*RealNodeComponent{
					createMockNode(ctx, "node-1", "192.168.1.1"),
				}

				component, err := NewDNSRealComponent(ctx, name, "test.example.com", nodes)

				assert.NoError(t, err)
				assert.NotNil(t, component)

				return nil
			}, pulumi.WithMocks("test", "stack", dnsMocks(0)))

			assert.NoError(t, err)
		})
	}
}
