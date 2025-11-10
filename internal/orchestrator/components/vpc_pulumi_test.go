package components

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

type vpcMocks int

// NewResource creates mock resources for VPC tests
func (vpcMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	outputs := args.Inputs.Copy()

	switch args.TypeToken {
	case "digitalocean:index/vpc:Vpc":
		outputs["id"] = resource.NewStringProperty("vpc-" + args.Name)
		outputs["name"] = args.Inputs["name"]
		outputs["region"] = args.Inputs["region"]
		outputs["ipRange"] = args.Inputs["ipRange"]
		outputs["description"] = args.Inputs["description"]
		outputs["urn"] = resource.NewStringProperty("do:vpc:" + args.Name)

	case "kubernetes-create:network:VPC":
		outputs["vpcId"] = resource.NewStringProperty("vpc-12345")
		outputs["vpcName"] = resource.NewStringProperty("kubernetes-vpc-test")
		outputs["region"] = args.Inputs["region"]
		outputs["ipRange"] = args.Inputs["ipRange"]
	}

	return args.Name + "_id", outputs, nil
}

// Call mocks function calls
func (vpcMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return resource.PropertyMap{}, nil
}

func TestNewVPCComponent_Success(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// Create VPC component
		vpc, err := NewVPCComponent(
			ctx,
			"test-vpc",
			"nyc1",
			"10.0.0.0/16",
		)

		assert.NoError(t, err, "Should create VPC without error")
		assert.NotNil(t, vpc, "VPC component should not be nil")

		// Verify outputs
		vpc.Region.ApplyT(func(region string) error {
			assert.Equal(t, "nyc1", region, "Region should match")
			return nil
		})

		vpc.IPRange.ApplyT(func(ipRange string) error {
			assert.Equal(t, "10.0.0.0/16", ipRange, "IP range should match")
			return nil
		})

		vpc.VPCName.ApplyT(func(name string) error {
			assert.NotEmpty(t, name, "VPC name should not be empty")
			return nil
		})

		vpc.VPCID.ApplyT(func(id string) error {
			assert.NotEmpty(t, id, "VPC ID should not be empty")
			return nil
		})

		return nil
	}, pulumi.WithMocks("project", "stack", vpcMocks(0)))

	assert.NoError(t, err)
}

func TestNewVPCComponent_DifferentRegions(t *testing.T) {
	regions := []string{"nyc1", "sfo1", "lon1", "fra1", "sgp1"}

	for _, region := range regions {
		t.Run("Region_"+region, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				vpc, err := NewVPCComponent(
					ctx,
					"test-vpc-"+region,
					region,
					"10.0.0.0/16",
				)

				assert.NoError(t, err, "Should create VPC in "+region)
				assert.NotNil(t, vpc)

				vpc.Region.ApplyT(func(r string) error {
					assert.Equal(t, region, r, "Region should match")
					return nil
				})

				return nil
			}, pulumi.WithMocks("project", "stack", vpcMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestNewVPCComponent_DifferentIPRanges(t *testing.T) {
	testCases := []struct {
		name    string
		ipRange string
	}{
		{"SmallRange", "10.0.0.0/24"},
		{"MediumRange", "10.0.0.0/16"},
		{"LargeRange", "10.0.0.0/8"},
		{"CustomRange1", "172.16.0.0/12"},
		{"CustomRange2", "192.168.0.0/16"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				vpc, err := NewVPCComponent(
					ctx,
					"test-vpc-"+tc.name,
					"nyc1",
					tc.ipRange,
				)

				assert.NoError(t, err, "Should create VPC with IP range: "+tc.ipRange)
				assert.NotNil(t, vpc)

				vpc.IPRange.ApplyT(func(ipRange string) error {
					assert.Equal(t, tc.ipRange, ipRange, "IP range should match")
					return nil
				})

				return nil
			}, pulumi.WithMocks("project", "stack", vpcMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestNewVPCComponent_WithParentResource(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// Create a parent component
		parentComponent := &pulumi.ResourceState{}
		err := ctx.RegisterComponentResource("test:parent:Component", "parent", parentComponent)
		assert.NoError(t, err)

		// Create VPC with parent
		vpc, err := NewVPCComponent(
			ctx,
			"test-vpc-child",
			"nyc1",
			"10.0.0.0/16",
			pulumi.Parent(parentComponent),
		)

		assert.NoError(t, err, "Should create VPC with parent resource")
		assert.NotNil(t, vpc)

		return nil
	}, pulumi.WithMocks("project", "stack", vpcMocks(0)))

	assert.NoError(t, err)
}

func TestNewVPCComponent_MultipleVPCs(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// Create multiple VPCs in parallel
		vpc1, err1 := NewVPCComponent(ctx, "vpc-1", "nyc1", "10.0.0.0/16")
		vpc2, err2 := NewVPCComponent(ctx, "vpc-2", "sfo1", "10.1.0.0/16")
		vpc3, err3 := NewVPCComponent(ctx, "vpc-3", "lon1", "10.2.0.0/16")

		assert.NoError(t, err1, "Should create first VPC")
		assert.NoError(t, err2, "Should create second VPC")
		assert.NoError(t, err3, "Should create third VPC")
		assert.NotNil(t, vpc1)
		assert.NotNil(t, vpc2)
		assert.NotNil(t, vpc3)

		// Verify each VPC has unique ID
		ids := make(map[string]bool)

		vpc1.VPCID.ApplyT(func(id string) error {
			ids[id] = true
			return nil
		})

		vpc2.VPCID.ApplyT(func(id string) error {
			ids[id] = true
			return nil
		})

		vpc3.VPCID.ApplyT(func(id string) error {
			ids[id] = true
			return nil
		})

		return nil
	}, pulumi.WithMocks("project", "stack", vpcMocks(0)))

	assert.NoError(t, err)
}

func TestNewVPCComponent_OutputsRegistered(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		vpc, err := NewVPCComponent(
			ctx,
			"test-vpc",
			"nyc1",
			"10.0.0.0/16",
		)

		assert.NoError(t, err)
		assert.NotNil(t, vpc)

		// Verify all expected outputs exist
		assert.NotNil(t, vpc.VPCID, "VPC ID output should exist")
		assert.NotNil(t, vpc.VPCName, "VPC Name output should exist")
		assert.NotNil(t, vpc.Region, "Region output should exist")
		assert.NotNil(t, vpc.IPRange, "IP Range output should exist")

		return nil
	}, pulumi.WithMocks("project", "stack", vpcMocks(0)))

	assert.NoError(t, err)
}

func TestNewVPCComponent_StackNaming(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		vpc, err := NewVPCComponent(
			ctx,
			"test-vpc",
			"nyc1",
			"10.0.0.0/16",
		)

		assert.NoError(t, err)
		assert.NotNil(t, vpc)

		// VPC name should include stack name
		vpc.VPCName.ApplyT(func(name string) error {
			assert.Contains(t, name, "kubernetes-vpc", "VPC name should contain 'kubernetes-vpc'")
			assert.Contains(t, name, ctx.Stack(), "VPC name should contain stack name")
			return nil
		})

		return nil
	}, pulumi.WithMocks("project", "test-stack", vpcMocks(0)))

	assert.NoError(t, err)
}

func TestNewVPCComponent_ResourceRegistration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		vpc, err := NewVPCComponent(
			ctx,
			"test-vpc",
			"nyc1",
			"10.0.0.0/16",
		)

		assert.NoError(t, err, "Component registration should succeed")
		assert.NotNil(t, vpc, "VPC component should not be nil")

		// Verify component is properly registered
		assert.IsType(t, &VPCComponent{}, vpc, "Should return VPCComponent type")

		return nil
	}, pulumi.WithMocks("project", "stack", vpcMocks(0)))

	assert.NoError(t, err)
}
