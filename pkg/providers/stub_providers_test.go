package providers

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// Mock for stub provider tests
type StubMocks struct {
	pulumi.MockResourceMonitor
}

func (m *StubMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	return args.Name + "_id", args.Inputs, nil
}

func (m *StubMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

// ==================== AWS Stub Tests ====================

func TestStubAWSProvider_GetName(t *testing.T) {
	provider := NewAWSProvider()
	assert.Equal(t, "aws", provider.GetName())
}

func TestStubAWSProvider_Initialize(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		provider := NewAWSProvider()
		cfg := &config.ClusterConfig{}

		err := provider.Initialize(ctx, cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "AWS provider not available")

		return nil
	}, pulumi.WithMocks("test", "stack", &StubMocks{}))

	assert.NoError(t, err)
}

func TestStubAWSProvider_CreateNode(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		provider := NewAWSProvider()
		nodeConfig := &config.NodeConfig{
			Name: "test-node",
		}

		node, err := provider.CreateNode(ctx, nodeConfig)
		assert.Nil(t, node)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "AWS provider not available")

		return nil
	}, pulumi.WithMocks("test", "stack", &StubMocks{}))

	assert.NoError(t, err)
}

func TestStubAWSProvider_CreateNodePool(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		provider := NewAWSProvider()
		pool := &config.NodePool{
			Name:  "test-pool",
			Count: 3,
		}

		nodes, err := provider.CreateNodePool(ctx, pool)
		assert.Nil(t, nodes)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "AWS provider not available")

		return nil
	}, pulumi.WithMocks("test", "stack", &StubMocks{}))

	assert.NoError(t, err)
}

func TestStubAWSProvider_CreateNetwork(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		provider := NewAWSProvider()
		network := &config.NetworkConfig{}

		net, err := provider.CreateNetwork(ctx, network)
		assert.Nil(t, net)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "AWS provider not available")

		return nil
	}, pulumi.WithMocks("test", "stack", &StubMocks{}))

	assert.NoError(t, err)
}

func TestStubAWSProvider_CreateFirewall(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		provider := NewAWSProvider()
		firewall := &config.FirewallConfig{}

		err := provider.CreateFirewall(ctx, firewall, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "AWS provider not available")

		return nil
	}, pulumi.WithMocks("test", "stack", &StubMocks{}))

	assert.NoError(t, err)
}

func TestStubAWSProvider_CreateLoadBalancer(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		provider := NewAWSProvider()
		lb := &config.LoadBalancerConfig{}

		output, err := provider.CreateLoadBalancer(ctx, lb)
		assert.Nil(t, output)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "AWS provider not available")

		return nil
	}, pulumi.WithMocks("test", "stack", &StubMocks{}))

	assert.NoError(t, err)
}

func TestStubAWSProvider_GetRegions(t *testing.T) {
	provider := NewAWSProvider()
	regions := provider.GetRegions()
	assert.Empty(t, regions)
}

func TestStubAWSProvider_GetSizes(t *testing.T) {
	provider := NewAWSProvider()
	sizes := provider.GetSizes()
	assert.Empty(t, sizes)
}

func TestStubAWSProvider_Cleanup(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		provider := NewAWSProvider()

		err := provider.Cleanup(ctx)
		assert.NoError(t, err) // Cleanup returns nil

		return nil
	}, pulumi.WithMocks("test", "stack", &StubMocks{}))

	assert.NoError(t, err)
}

// ==================== GCP Stub Tests ====================

func TestStubGCPProvider_GetName(t *testing.T) {
	provider := NewGCPProvider()
	assert.Equal(t, "gcp", provider.GetName())
}

func TestStubGCPProvider_Initialize(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		provider := NewGCPProvider()
		cfg := &config.ClusterConfig{}

		err := provider.Initialize(ctx, cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "GCP provider not available")

		return nil
	}, pulumi.WithMocks("test", "stack", &StubMocks{}))

	assert.NoError(t, err)
}

func TestStubGCPProvider_CreateNode(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		provider := NewGCPProvider()
		nodeConfig := &config.NodeConfig{
			Name: "test-node",
		}

		node, err := provider.CreateNode(ctx, nodeConfig)
		assert.Nil(t, node)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "GCP provider not available")

		return nil
	}, pulumi.WithMocks("test", "stack", &StubMocks{}))

	assert.NoError(t, err)
}

func TestStubGCPProvider_CreateNodePool(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		provider := NewGCPProvider()
		pool := &config.NodePool{
			Name:  "test-pool",
			Count: 2,
		}

		nodes, err := provider.CreateNodePool(ctx, pool)
		assert.Nil(t, nodes)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "GCP provider not available")

		return nil
	}, pulumi.WithMocks("test", "stack", &StubMocks{}))

	assert.NoError(t, err)
}

func TestStubGCPProvider_CreateNetwork(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		provider := NewGCPProvider()
		network := &config.NetworkConfig{}

		net, err := provider.CreateNetwork(ctx, network)
		assert.Nil(t, net)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "GCP provider not available")

		return nil
	}, pulumi.WithMocks("test", "stack", &StubMocks{}))

	assert.NoError(t, err)
}

func TestStubGCPProvider_CreateFirewall(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		provider := NewGCPProvider()
		firewall := &config.FirewallConfig{}

		err := provider.CreateFirewall(ctx, firewall, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "GCP provider not available")

		return nil
	}, pulumi.WithMocks("test", "stack", &StubMocks{}))

	assert.NoError(t, err)
}

func TestStubGCPProvider_CreateLoadBalancer(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		provider := NewGCPProvider()
		lb := &config.LoadBalancerConfig{}

		output, err := provider.CreateLoadBalancer(ctx, lb)
		assert.Nil(t, output)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "GCP provider not available")

		return nil
	}, pulumi.WithMocks("test", "stack", &StubMocks{}))

	assert.NoError(t, err)
}

func TestStubGCPProvider_GetRegions(t *testing.T) {
	provider := NewGCPProvider()
	regions := provider.GetRegions()
	assert.Empty(t, regions)
}

func TestStubGCPProvider_GetSizes(t *testing.T) {
	provider := NewGCPProvider()
	sizes := provider.GetSizes()
	assert.Empty(t, sizes)
}

func TestStubGCPProvider_Cleanup(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		provider := NewGCPProvider()

		err := provider.Cleanup(ctx)
		assert.NoError(t, err) // Cleanup returns nil

		return nil
	}, pulumi.WithMocks("test", "stack", &StubMocks{}))

	assert.NoError(t, err)
}
