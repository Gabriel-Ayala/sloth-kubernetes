//go:build integration
// +build integration

package providers_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// TEST CONFIGURATION
// =============================================================================

const (
	testPrefix     = "sloth-k8s-test"
	defaultRegion  = "us-east-1"
	testTimeout    = 5 * time.Minute
	cleanupTimeout = 2 * time.Minute
)

// TestResources tracks created resources for cleanup
type TestResources struct {
	VPCIDs             []string
	SubnetIDs          []string
	SecurityGroupIDs   []string
	InternetGatewayIDs []string
	InstanceIDs        []string
	KeyPairNames       []string
	SpotRequestIDs     []string
	LoadBalancerARNs   []string
	TargetGroupARNs    []string
}

// AWSTestSuite holds AWS clients for integration tests
type AWSTestSuite struct {
	cfg       aws.Config
	ec2Client *ec2.Client
	stsClient *sts.Client
	elbClient *elasticloadbalancingv2.Client
	region    string
	accountID string
	resources *TestResources
}

// =============================================================================
// TEST SETUP AND HELPERS
// =============================================================================

func skipIfNoCredentials(t *testing.T) {
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" || os.Getenv("AWS_SECRET_ACCESS_KEY") == "" {
		t.Skip("Skipping integration test: AWS credentials not configured. Set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY")
	}
}

func setupAWSTestSuite(t *testing.T) *AWSTestSuite {
	skipIfNoCredentials(t)

	ctx := context.Background()
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = defaultRegion
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	require.NoError(t, err, "Failed to load AWS config")

	suite := &AWSTestSuite{
		cfg:       cfg,
		ec2Client: ec2.NewFromConfig(cfg),
		stsClient: sts.NewFromConfig(cfg),
		elbClient: elasticloadbalancingv2.NewFromConfig(cfg),
		region:    region,
		resources: &TestResources{},
	}

	// Get account ID
	identity, err := suite.stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	require.NoError(t, err, "Failed to get caller identity")
	suite.accountID = *identity.Account

	return suite
}

func (s *AWSTestSuite) cleanup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()

	t.Log("=== Cleaning up test resources ===")

	// Terminate instances first
	if len(s.resources.InstanceIDs) > 0 {
		t.Logf("Terminating %d instances...", len(s.resources.InstanceIDs))
		_, err := s.ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
			InstanceIds: s.resources.InstanceIDs,
		})
		if err != nil {
			t.Logf("Warning: Failed to terminate instances: %v", err)
		}
		// Wait for termination
		time.Sleep(10 * time.Second)
	}

	// Cancel spot requests
	if len(s.resources.SpotRequestIDs) > 0 {
		t.Logf("Cancelling %d spot requests...", len(s.resources.SpotRequestIDs))
		_, err := s.ec2Client.CancelSpotInstanceRequests(ctx, &ec2.CancelSpotInstanceRequestsInput{
			SpotInstanceRequestIds: s.resources.SpotRequestIDs,
		})
		if err != nil {
			t.Logf("Warning: Failed to cancel spot requests: %v", err)
		}
	}

	// Delete load balancers
	for _, arn := range s.resources.LoadBalancerARNs {
		t.Logf("Deleting load balancer: %s", arn)
		_, err := s.elbClient.DeleteLoadBalancer(ctx, &elasticloadbalancingv2.DeleteLoadBalancerInput{
			LoadBalancerArn: aws.String(arn),
		})
		if err != nil {
			t.Logf("Warning: Failed to delete load balancer: %v", err)
		}
	}

	// Delete target groups
	for _, arn := range s.resources.TargetGroupARNs {
		t.Logf("Deleting target group: %s", arn)
		_, err := s.elbClient.DeleteTargetGroup(ctx, &elasticloadbalancingv2.DeleteTargetGroupInput{
			TargetGroupArn: aws.String(arn),
		})
		if err != nil {
			t.Logf("Warning: Failed to delete target group: %v", err)
		}
	}

	// Delete key pairs
	for _, name := range s.resources.KeyPairNames {
		t.Logf("Deleting key pair: %s", name)
		_, err := s.ec2Client.DeleteKeyPair(ctx, &ec2.DeleteKeyPairInput{
			KeyName: aws.String(name),
		})
		if err != nil {
			t.Logf("Warning: Failed to delete key pair: %v", err)
		}
	}

	// Delete security groups
	for _, id := range s.resources.SecurityGroupIDs {
		t.Logf("Deleting security group: %s", id)
		_, err := s.ec2Client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
			GroupId: aws.String(id),
		})
		if err != nil {
			t.Logf("Warning: Failed to delete security group: %v", err)
		}
	}

	// Detach and delete internet gateways
	for i, igwID := range s.resources.InternetGatewayIDs {
		if i < len(s.resources.VPCIDs) {
			t.Logf("Detaching internet gateway: %s", igwID)
			_, _ = s.ec2Client.DetachInternetGateway(ctx, &ec2.DetachInternetGatewayInput{
				InternetGatewayId: aws.String(igwID),
				VpcId:             aws.String(s.resources.VPCIDs[i]),
			})
		}
		t.Logf("Deleting internet gateway: %s", igwID)
		_, err := s.ec2Client.DeleteInternetGateway(ctx, &ec2.DeleteInternetGatewayInput{
			InternetGatewayId: aws.String(igwID),
		})
		if err != nil {
			t.Logf("Warning: Failed to delete internet gateway: %v", err)
		}
	}

	// Delete subnets
	for _, id := range s.resources.SubnetIDs {
		t.Logf("Deleting subnet: %s", id)
		_, err := s.ec2Client.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{
			SubnetId: aws.String(id),
		})
		if err != nil {
			t.Logf("Warning: Failed to delete subnet: %v", err)
		}
	}

	// Delete VPCs
	for _, id := range s.resources.VPCIDs {
		t.Logf("Deleting VPC: %s", id)
		_, err := s.ec2Client.DeleteVpc(ctx, &ec2.DeleteVpcInput{
			VpcId: aws.String(id),
		})
		if err != nil {
			t.Logf("Warning: Failed to delete VPC: %v", err)
		}
	}

	t.Log("=== Cleanup completed ===")
}

func testTag(name string) types.Tag {
	return types.Tag{
		Key:   aws.String("Name"),
		Value: aws.String(fmt.Sprintf("%s-%s", testPrefix, name)),
	}
}

func testTags(name string) []types.Tag {
	return []types.Tag{
		testTag(name),
		{Key: aws.String("Environment"), Value: aws.String("test")},
		{Key: aws.String("ManagedBy"), Value: aws.String("sloth-kubernetes-integration-test")},
	}
}

// =============================================================================
// CONNECTIVITY TESTS
// =============================================================================

func TestIntegration_AWS_Connectivity(t *testing.T) {
	suite := setupAWSTestSuite(t)

	t.Log("=== Testing AWS Connectivity ===")

	ctx := context.Background()

	// Test STS GetCallerIdentity
	t.Run("STS_GetCallerIdentity", func(t *testing.T) {
		identity, err := suite.stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
		require.NoError(t, err)

		t.Logf("Account ID: %s", *identity.Account)
		t.Logf("User ARN: %s", *identity.Arn)
		t.Logf("User ID: %s", *identity.UserId)

		assert.NotEmpty(t, *identity.Account)
		assert.NotEmpty(t, *identity.Arn)
	})

	// Test EC2 DescribeRegions
	t.Run("EC2_DescribeRegions", func(t *testing.T) {
		regions, err := suite.ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
		require.NoError(t, err)

		t.Logf("Available regions: %d", len(regions.Regions))
		assert.Greater(t, len(regions.Regions), 10, "Should have multiple regions available")

		// Verify current region is in list
		found := false
		for _, r := range regions.Regions {
			if *r.RegionName == suite.region {
				found = true
				break
			}
		}
		assert.True(t, found, "Current region should be in available regions")
	})

	// Test EC2 DescribeAvailabilityZones
	t.Run("EC2_DescribeAvailabilityZones", func(t *testing.T) {
		azs, err := suite.ec2Client.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{})
		require.NoError(t, err)

		t.Logf("Availability zones in %s: %d", suite.region, len(azs.AvailabilityZones))
		for _, az := range azs.AvailabilityZones {
			t.Logf("  - %s (%s)", *az.ZoneName, az.State)
		}

		assert.Greater(t, len(azs.AvailabilityZones), 0, "Should have at least one AZ")
	})
}

// =============================================================================
// EC2 INSTANCE TESTS
// =============================================================================

func TestIntegration_AWS_EC2_DescribeInstances(t *testing.T) {
	suite := setupAWSTestSuite(t)

	t.Log("=== Testing EC2 Instance Operations ===")

	ctx := context.Background()

	// Describe existing instances
	t.Run("DescribeInstances", func(t *testing.T) {
		result, err := suite.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			MaxResults: aws.Int32(10),
		})
		require.NoError(t, err)

		instanceCount := 0
		for _, reservation := range result.Reservations {
			instanceCount += len(reservation.Instances)
		}

		t.Logf("Found %d instances in account", instanceCount)
	})

	// Describe AMIs (Ubuntu)
	t.Run("DescribeUbuntuAMIs", func(t *testing.T) {
		result, err := suite.ec2Client.DescribeImages(ctx, &ec2.DescribeImagesInput{
			Owners: []string{"099720109477"}, // Canonical
			Filters: []types.Filter{
				{Name: aws.String("name"), Values: []string{"ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"}},
				{Name: aws.String("state"), Values: []string{"available"}},
				{Name: aws.String("architecture"), Values: []string{"x86_64"}},
			},
		})
		require.NoError(t, err)
		require.Greater(t, len(result.Images), 0, "Should find Ubuntu AMIs")

		// Get latest AMI
		latestAMI := result.Images[0]
		for _, img := range result.Images {
			if *img.CreationDate > *latestAMI.CreationDate {
				latestAMI = img
			}
		}

		t.Logf("Latest Ubuntu 22.04 AMI: %s", *latestAMI.ImageId)
		t.Logf("  Name: %s", *latestAMI.Name)
		t.Logf("  Created: %s", *latestAMI.CreationDate)
	})

	// Describe instance types
	t.Run("DescribeInstanceTypes", func(t *testing.T) {
		result, err := suite.ec2Client.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{
			InstanceTypes: []types.InstanceType{
				types.InstanceTypeT3Micro,
				types.InstanceTypeT3Small,
				types.InstanceTypeT3Medium,
				types.InstanceTypeM5Large,
			},
		})
		require.NoError(t, err)

		t.Logf("Instance type details:")
		for _, it := range result.InstanceTypes {
			t.Logf("  - %s: %d vCPUs, %d MB RAM", it.InstanceType, *it.VCpuInfo.DefaultVCpus, *it.MemoryInfo.SizeInMiB)
		}

		assert.Equal(t, 4, len(result.InstanceTypes))
	})
}

func TestIntegration_AWS_EC2_CreateInstance(t *testing.T) {
	suite := setupAWSTestSuite(t)
	defer suite.cleanup(t)

	t.Log("=== Testing EC2 Instance Creation ===")

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Step 1: Create VPC
	t.Log("Step 1: Creating VPC...")
	vpc, err := suite.ec2Client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.100.0.0/16"),
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeVpc, Tags: testTags("vpc")},
		},
	})
	require.NoError(t, err)
	suite.resources.VPCIDs = append(suite.resources.VPCIDs, *vpc.Vpc.VpcId)
	t.Logf("  Created VPC: %s", *vpc.Vpc.VpcId)

	// Enable DNS hostnames
	_, err = suite.ec2Client.ModifyVpcAttribute(ctx, &ec2.ModifyVpcAttributeInput{
		VpcId:              vpc.Vpc.VpcId,
		EnableDnsHostnames: &types.AttributeBooleanValue{Value: aws.Bool(true)},
	})
	require.NoError(t, err)

	// Step 2: Create Internet Gateway
	t.Log("Step 2: Creating Internet Gateway...")
	igw, err := suite.ec2Client.CreateInternetGateway(ctx, &ec2.CreateInternetGatewayInput{
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeInternetGateway, Tags: testTags("igw")},
		},
	})
	require.NoError(t, err)
	suite.resources.InternetGatewayIDs = append(suite.resources.InternetGatewayIDs, *igw.InternetGateway.InternetGatewayId)
	t.Logf("  Created IGW: %s", *igw.InternetGateway.InternetGatewayId)

	// Attach IGW to VPC
	_, err = suite.ec2Client.AttachInternetGateway(ctx, &ec2.AttachInternetGatewayInput{
		InternetGatewayId: igw.InternetGateway.InternetGatewayId,
		VpcId:             vpc.Vpc.VpcId,
	})
	require.NoError(t, err)

	// Step 3: Create Subnet
	t.Log("Step 3: Creating Subnet...")
	subnet, err := suite.ec2Client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
		VpcId:            vpc.Vpc.VpcId,
		CidrBlock:        aws.String("10.100.1.0/24"),
		AvailabilityZone: aws.String(suite.region + "a"),
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeSubnet, Tags: testTags("subnet")},
		},
	})
	require.NoError(t, err)
	suite.resources.SubnetIDs = append(suite.resources.SubnetIDs, *subnet.Subnet.SubnetId)
	t.Logf("  Created Subnet: %s", *subnet.Subnet.SubnetId)

	// Enable auto-assign public IP
	_, err = suite.ec2Client.ModifySubnetAttribute(ctx, &ec2.ModifySubnetAttributeInput{
		SubnetId:            subnet.Subnet.SubnetId,
		MapPublicIpOnLaunch: &types.AttributeBooleanValue{Value: aws.Bool(true)},
	})
	require.NoError(t, err)

	// Step 4: Create Security Group
	t.Log("Step 4: Creating Security Group...")
	sg, err := suite.ec2Client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(fmt.Sprintf("%s-sg-%d", testPrefix, time.Now().Unix())),
		Description: aws.String("Security group for sloth-kubernetes integration tests"),
		VpcId:       vpc.Vpc.VpcId,
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeSecurityGroup, Tags: testTags("sg")},
		},
	})
	require.NoError(t, err)
	suite.resources.SecurityGroupIDs = append(suite.resources.SecurityGroupIDs, *sg.GroupId)
	t.Logf("  Created Security Group: %s", *sg.GroupId)

	// Add SSH ingress rule
	_, err = suite.ec2Client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: sg.GroupId,
		IpPermissions: []types.IpPermission{
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(22),
				ToPort:     aws.Int32(22),
				IpRanges:   []types.IpRange{{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("SSH access")}},
			},
		},
	})
	require.NoError(t, err)

	// Step 5: Create Key Pair
	t.Log("Step 5: Creating Key Pair...")
	keyName := fmt.Sprintf("%s-key-%d", testPrefix, time.Now().Unix())
	keyPair, err := suite.ec2Client.CreateKeyPair(ctx, &ec2.CreateKeyPairInput{
		KeyName: aws.String(keyName),
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeKeyPair, Tags: testTags("keypair")},
		},
	})
	require.NoError(t, err)
	suite.resources.KeyPairNames = append(suite.resources.KeyPairNames, keyName)
	t.Logf("  Created Key Pair: %s", keyName)
	t.Logf("  Key Fingerprint: %s", *keyPair.KeyFingerprint)

	// Step 6: Get latest Ubuntu AMI
	t.Log("Step 6: Finding latest Ubuntu AMI...")
	amiResult, err := suite.ec2Client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Owners: []string{"099720109477"},
		Filters: []types.Filter{
			{Name: aws.String("name"), Values: []string{"ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"}},
			{Name: aws.String("state"), Values: []string{"available"}},
		},
	})
	require.NoError(t, err)
	require.Greater(t, len(amiResult.Images), 0)

	latestAMI := amiResult.Images[0]
	for _, img := range amiResult.Images {
		if *img.CreationDate > *latestAMI.CreationDate {
			latestAMI = img
		}
	}
	t.Logf("  Using AMI: %s", *latestAMI.ImageId)

	// Step 7: Create EC2 Instance
	t.Log("Step 7: Creating EC2 Instance...")
	userData := base64Encode(`#!/bin/bash
echo "Hello from sloth-kubernetes integration test"
apt-get update -y
hostname sloth-k8s-test-node
`)

	instance, err := suite.ec2Client.RunInstances(ctx, &ec2.RunInstancesInput{
		ImageId:          latestAMI.ImageId,
		InstanceType:     types.InstanceTypeT3Micro,
		MinCount:         aws.Int32(1),
		MaxCount:         aws.Int32(1),
		KeyName:          aws.String(keyName),
		SubnetId:         subnet.Subnet.SubnetId,
		SecurityGroupIds: []string{*sg.GroupId},
		UserData:         aws.String(userData),
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeInstance, Tags: testTags("instance")},
		},
	})
	require.NoError(t, err)
	require.Len(t, instance.Instances, 1)

	instanceID := *instance.Instances[0].InstanceId
	suite.resources.InstanceIDs = append(suite.resources.InstanceIDs, instanceID)
	t.Logf("  Created Instance: %s", instanceID)

	// Step 8: Wait for instance to be running
	t.Log("Step 8: Waiting for instance to be running...")
	waiter := ec2.NewInstanceRunningWaiter(suite.ec2Client)
	err = waiter.Wait(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}, 3*time.Minute)
	require.NoError(t, err)

	// Get instance details
	descResult, err := suite.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	require.NoError(t, err)
	require.Len(t, descResult.Reservations, 1)
	require.Len(t, descResult.Reservations[0].Instances, 1)

	runningInstance := descResult.Reservations[0].Instances[0]
	t.Logf("  Instance State: %s", runningInstance.State.Name)
	t.Logf("  Private IP: %s", *runningInstance.PrivateIpAddress)
	if runningInstance.PublicIpAddress != nil {
		t.Logf("  Public IP: %s", *runningInstance.PublicIpAddress)
	}

	assert.Equal(t, types.InstanceStateNameRunning, runningInstance.State.Name)
	assert.NotEmpty(t, *runningInstance.PrivateIpAddress)

	t.Log("\n=== EC2 Instance Creation Test PASSED ===")
}

// =============================================================================
// VPC AND NETWORKING TESTS
// =============================================================================

func TestIntegration_AWS_VPC_FullStack(t *testing.T) {
	suite := setupAWSTestSuite(t)
	defer suite.cleanup(t)

	t.Log("=== Testing VPC Full Stack Creation ===")

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Create VPC
	t.Log("Creating VPC with full networking stack...")

	vpc, err := suite.ec2Client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.200.0.0/16"),
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeVpc, Tags: testTags("fullstack-vpc")},
		},
	})
	require.NoError(t, err)
	suite.resources.VPCIDs = append(suite.resources.VPCIDs, *vpc.Vpc.VpcId)

	// Enable DNS
	_, err = suite.ec2Client.ModifyVpcAttribute(ctx, &ec2.ModifyVpcAttributeInput{
		VpcId:              vpc.Vpc.VpcId,
		EnableDnsHostnames: &types.AttributeBooleanValue{Value: aws.Bool(true)},
	})
	require.NoError(t, err)

	// Create Internet Gateway
	igw, err := suite.ec2Client.CreateInternetGateway(ctx, &ec2.CreateInternetGatewayInput{
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeInternetGateway, Tags: testTags("fullstack-igw")},
		},
	})
	require.NoError(t, err)
	suite.resources.InternetGatewayIDs = append(suite.resources.InternetGatewayIDs, *igw.InternetGateway.InternetGatewayId)

	_, err = suite.ec2Client.AttachInternetGateway(ctx, &ec2.AttachInternetGatewayInput{
		InternetGatewayId: igw.InternetGateway.InternetGatewayId,
		VpcId:             vpc.Vpc.VpcId,
	})
	require.NoError(t, err)

	// Create subnets in multiple AZs
	azs, err := suite.ec2Client.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{
		Filters: []types.Filter{
			{Name: aws.String("state"), Values: []string{"available"}},
		},
	})
	require.NoError(t, err)

	subnets := make([]string, 0)
	for i, az := range azs.AvailabilityZones {
		if i >= 3 {
			break // Create max 3 subnets
		}

		cidr := fmt.Sprintf("10.200.%d.0/24", i+1)
		subnet, err := suite.ec2Client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
			VpcId:            vpc.Vpc.VpcId,
			CidrBlock:        aws.String(cidr),
			AvailabilityZone: az.ZoneName,
			TagSpecifications: []types.TagSpecification{
				{ResourceType: types.ResourceTypeSubnet, Tags: testTags(fmt.Sprintf("subnet-%s", *az.ZoneName))},
			},
		})
		require.NoError(t, err)
		suite.resources.SubnetIDs = append(suite.resources.SubnetIDs, *subnet.Subnet.SubnetId)
		subnets = append(subnets, *subnet.Subnet.SubnetId)
		t.Logf("  Created Subnet %s in %s", *subnet.Subnet.SubnetId, *az.ZoneName)
	}

	// Create Route Table
	rt, err := suite.ec2Client.CreateRouteTable(ctx, &ec2.CreateRouteTableInput{
		VpcId: vpc.Vpc.VpcId,
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeRouteTable, Tags: testTags("rt")},
		},
	})
	require.NoError(t, err)
	t.Logf("  Created Route Table: %s", *rt.RouteTable.RouteTableId)

	// Add default route to IGW
	_, err = suite.ec2Client.CreateRoute(ctx, &ec2.CreateRouteInput{
		RouteTableId:         rt.RouteTable.RouteTableId,
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:            igw.InternetGateway.InternetGatewayId,
	})
	require.NoError(t, err)

	// Associate subnets with route table
	for _, subnetID := range subnets {
		_, err = suite.ec2Client.AssociateRouteTable(ctx, &ec2.AssociateRouteTableInput{
			RouteTableId: rt.RouteTable.RouteTableId,
			SubnetId:     aws.String(subnetID),
		})
		require.NoError(t, err)
	}

	// Create Security Groups for K8s
	masterSG, err := suite.ec2Client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(fmt.Sprintf("%s-master-sg-%d", testPrefix, time.Now().Unix())),
		Description: aws.String("K8s Master Security Group"),
		VpcId:       vpc.Vpc.VpcId,
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeSecurityGroup, Tags: testTags("master-sg")},
		},
	})
	require.NoError(t, err)
	suite.resources.SecurityGroupIDs = append(suite.resources.SecurityGroupIDs, *masterSG.GroupId)

	workerSG, err := suite.ec2Client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(fmt.Sprintf("%s-worker-sg-%d", testPrefix, time.Now().Unix())),
		Description: aws.String("K8s Worker Security Group"),
		VpcId:       vpc.Vpc.VpcId,
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeSecurityGroup, Tags: testTags("worker-sg")},
		},
	})
	require.NoError(t, err)
	suite.resources.SecurityGroupIDs = append(suite.resources.SecurityGroupIDs, *workerSG.GroupId)

	// Add K8s-specific rules to master SG
	_, err = suite.ec2Client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: masterSG.GroupId,
		IpPermissions: []types.IpPermission{
			{IpProtocol: aws.String("tcp"), FromPort: aws.Int32(22), ToPort: aws.Int32(22), IpRanges: []types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
			{IpProtocol: aws.String("tcp"), FromPort: aws.Int32(6443), ToPort: aws.Int32(6443), IpRanges: []types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
			{IpProtocol: aws.String("tcp"), FromPort: aws.Int32(2379), ToPort: aws.Int32(2380), IpRanges: []types.IpRange{{CidrIp: aws.String("10.200.0.0/16")}}},
			{IpProtocol: aws.String("udp"), FromPort: aws.Int32(51820), ToPort: aws.Int32(51820), IpRanges: []types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
		},
	})
	require.NoError(t, err)

	t.Log("\n=== VPC Full Stack Test Summary ===")
	t.Logf("VPC: %s (10.200.0.0/16)", *vpc.Vpc.VpcId)
	t.Logf("Internet Gateway: %s", *igw.InternetGateway.InternetGatewayId)
	t.Logf("Subnets: %d across AZs", len(subnets))
	t.Logf("Master SG: %s", *masterSG.GroupId)
	t.Logf("Worker SG: %s", *workerSG.GroupId)
	t.Log("=== VPC Full Stack Test PASSED ===")
}

// =============================================================================
// SPOT INSTANCE TESTS
// =============================================================================

func TestIntegration_AWS_SpotInstance(t *testing.T) {
	suite := setupAWSTestSuite(t)
	defer suite.cleanup(t)

	t.Log("=== Testing Spot Instance Operations ===")

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Get current spot prices
	t.Run("DescribeSpotPriceHistory", func(t *testing.T) {
		result, err := suite.ec2Client.DescribeSpotPriceHistory(ctx, &ec2.DescribeSpotPriceHistoryInput{
			InstanceTypes:       []types.InstanceType{types.InstanceTypeT3Medium, types.InstanceTypeT3Large},
			ProductDescriptions: []string{"Linux/UNIX"},
			StartTime:           aws.Time(time.Now().Add(-1 * time.Hour)),
			MaxResults:          aws.Int32(20),
		})
		require.NoError(t, err)

		t.Logf("Current Spot Prices in %s:", suite.region)
		priceMap := make(map[string]map[string]string)
		for _, price := range result.SpotPriceHistory {
			if _, ok := priceMap[string(price.InstanceType)]; !ok {
				priceMap[string(price.InstanceType)] = make(map[string]string)
			}
			priceMap[string(price.InstanceType)][*price.AvailabilityZone] = *price.SpotPrice
		}

		for instanceType, azPrices := range priceMap {
			for az, price := range azPrices {
				t.Logf("  %s in %s: $%s/hour", instanceType, az, price)
			}
		}
	})

	// Create infrastructure for spot request
	t.Log("\nCreating infrastructure for Spot Instance test...")

	// Create VPC
	vpc, err := suite.ec2Client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.150.0.0/16"),
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeVpc, Tags: testTags("spot-vpc")},
		},
	})
	require.NoError(t, err)
	suite.resources.VPCIDs = append(suite.resources.VPCIDs, *vpc.Vpc.VpcId)

	// Create Subnet
	subnet, err := suite.ec2Client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
		VpcId:            vpc.Vpc.VpcId,
		CidrBlock:        aws.String("10.150.1.0/24"),
		AvailabilityZone: aws.String(suite.region + "a"),
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeSubnet, Tags: testTags("spot-subnet")},
		},
	})
	require.NoError(t, err)
	suite.resources.SubnetIDs = append(suite.resources.SubnetIDs, *subnet.Subnet.SubnetId)

	// Create Security Group
	sg, err := suite.ec2Client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(fmt.Sprintf("%s-spot-sg-%d", testPrefix, time.Now().Unix())),
		Description: aws.String("Spot instance security group"),
		VpcId:       vpc.Vpc.VpcId,
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeSecurityGroup, Tags: testTags("spot-sg")},
		},
	})
	require.NoError(t, err)
	suite.resources.SecurityGroupIDs = append(suite.resources.SecurityGroupIDs, *sg.GroupId)

	// Get AMI
	amiResult, err := suite.ec2Client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Owners: []string{"099720109477"},
		Filters: []types.Filter{
			{Name: aws.String("name"), Values: []string{"ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"}},
			{Name: aws.String("state"), Values: []string{"available"}},
		},
	})
	require.NoError(t, err)
	require.Greater(t, len(amiResult.Images), 0)

	latestAMI := amiResult.Images[0]
	for _, img := range amiResult.Images {
		if *img.CreationDate > *latestAMI.CreationDate {
			latestAMI = img
		}
	}

	// Request Spot Instance
	t.Log("Requesting Spot Instance...")
	spotRequest, err := suite.ec2Client.RequestSpotInstances(ctx, &ec2.RequestSpotInstancesInput{
		InstanceCount: aws.Int32(1),
		Type:          types.SpotInstanceTypeOneTime,
		LaunchSpecification: &types.RequestSpotLaunchSpecification{
			ImageId:          latestAMI.ImageId,
			InstanceType:     types.InstanceTypeT3Micro,
			SubnetId:         subnet.Subnet.SubnetId,
			SecurityGroupIds: []string{*sg.GroupId},
		},
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeSpotInstancesRequest, Tags: testTags("spot-request")},
		},
	})
	require.NoError(t, err)
	require.Len(t, spotRequest.SpotInstanceRequests, 1)

	spotRequestID := *spotRequest.SpotInstanceRequests[0].SpotInstanceRequestId
	suite.resources.SpotRequestIDs = append(suite.resources.SpotRequestIDs, spotRequestID)

	t.Logf("  Spot Request ID: %s", spotRequestID)
	t.Logf("  Status: %s", *spotRequest.SpotInstanceRequests[0].Status.Code)

	// Wait briefly and check status
	time.Sleep(5 * time.Second)

	descResult, err := suite.ec2Client.DescribeSpotInstanceRequests(ctx, &ec2.DescribeSpotInstanceRequestsInput{
		SpotInstanceRequestIds: []string{spotRequestID},
	})
	require.NoError(t, err)

	if len(descResult.SpotInstanceRequests) > 0 {
		req := descResult.SpotInstanceRequests[0]
		t.Logf("  Current Status: %s - %s", *req.Status.Code, aws.ToString(req.Status.Message))

		if req.InstanceId != nil {
			t.Logf("  Instance ID: %s", *req.InstanceId)
			suite.resources.InstanceIDs = append(suite.resources.InstanceIDs, *req.InstanceId)
		}
	}

	t.Log("\n=== Spot Instance Test PASSED ===")
}

// =============================================================================
// LOAD BALANCER TESTS
// =============================================================================

func TestIntegration_AWS_LoadBalancer(t *testing.T) {
	suite := setupAWSTestSuite(t)
	defer suite.cleanup(t)

	t.Log("=== Testing Load Balancer Operations ===")

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Create VPC and Subnets
	t.Log("Creating VPC infrastructure for Load Balancer...")

	vpc, err := suite.ec2Client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.180.0.0/16"),
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeVpc, Tags: testTags("lb-vpc")},
		},
	})
	require.NoError(t, err)
	suite.resources.VPCIDs = append(suite.resources.VPCIDs, *vpc.Vpc.VpcId)

	// Create IGW
	igw, err := suite.ec2Client.CreateInternetGateway(ctx, &ec2.CreateInternetGatewayInput{
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeInternetGateway, Tags: testTags("lb-igw")},
		},
	})
	require.NoError(t, err)
	suite.resources.InternetGatewayIDs = append(suite.resources.InternetGatewayIDs, *igw.InternetGateway.InternetGatewayId)

	_, err = suite.ec2Client.AttachInternetGateway(ctx, &ec2.AttachInternetGatewayInput{
		InternetGatewayId: igw.InternetGateway.InternetGatewayId,
		VpcId:             vpc.Vpc.VpcId,
	})
	require.NoError(t, err)

	// Get AZs and create subnets
	azs, err := suite.ec2Client.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{
		Filters: []types.Filter{{Name: aws.String("state"), Values: []string{"available"}}},
	})
	require.NoError(t, err)

	var subnetIDs []string
	for i := 0; i < 2 && i < len(azs.AvailabilityZones); i++ {
		az := azs.AvailabilityZones[i]
		cidr := fmt.Sprintf("10.180.%d.0/24", i+1)

		subnet, err := suite.ec2Client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
			VpcId:            vpc.Vpc.VpcId,
			CidrBlock:        aws.String(cidr),
			AvailabilityZone: az.ZoneName,
			TagSpecifications: []types.TagSpecification{
				{ResourceType: types.ResourceTypeSubnet, Tags: testTags(fmt.Sprintf("lb-subnet-%d", i))},
			},
		})
		require.NoError(t, err)
		suite.resources.SubnetIDs = append(suite.resources.SubnetIDs, *subnet.Subnet.SubnetId)
		subnetIDs = append(subnetIDs, *subnet.Subnet.SubnetId)

		// Enable public IP
		_, err = suite.ec2Client.ModifySubnetAttribute(ctx, &ec2.ModifySubnetAttributeInput{
			SubnetId:            subnet.Subnet.SubnetId,
			MapPublicIpOnLaunch: &types.AttributeBooleanValue{Value: aws.Bool(true)},
		})
		require.NoError(t, err)

		t.Logf("  Created Subnet: %s in %s", *subnet.Subnet.SubnetId, *az.ZoneName)
	}

	// Create Security Group for LB
	lbSG, err := suite.ec2Client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(fmt.Sprintf("%s-lb-sg-%d", testPrefix, time.Now().Unix())),
		Description: aws.String("Load Balancer Security Group"),
		VpcId:       vpc.Vpc.VpcId,
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeSecurityGroup, Tags: testTags("lb-sg")},
		},
	})
	require.NoError(t, err)
	suite.resources.SecurityGroupIDs = append(suite.resources.SecurityGroupIDs, *lbSG.GroupId)

	_, err = suite.ec2Client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: lbSG.GroupId,
		IpPermissions: []types.IpPermission{
			{IpProtocol: aws.String("tcp"), FromPort: aws.Int32(6443), ToPort: aws.Int32(6443), IpRanges: []types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
		},
	})
	require.NoError(t, err)

	// Create Network Load Balancer (for K8s API)
	t.Log("Creating Network Load Balancer...")
	nlbName := fmt.Sprintf("%s-nlb-%d", testPrefix, time.Now().Unix())
	nlbName = strings.ReplaceAll(nlbName, "_", "-")
	if len(nlbName) > 32 {
		nlbName = nlbName[:32]
	}

	nlb, err := suite.elbClient.CreateLoadBalancer(ctx, &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name:           aws.String(nlbName),
		Type:           elbtypes.LoadBalancerTypeEnumNetwork,
		Scheme:         elbtypes.LoadBalancerSchemeEnumInternetFacing,
		Subnets:        subnetIDs,
		SecurityGroups: []string{*lbSG.GroupId},
		Tags: []elbtypes.Tag{
			{Key: aws.String("Name"), Value: aws.String(fmt.Sprintf("%s-k8s-api-nlb", testPrefix))},
			{Key: aws.String("Environment"), Value: aws.String("test")},
		},
	})
	require.NoError(t, err)
	require.Len(t, nlb.LoadBalancers, 1)

	nlbARN := *nlb.LoadBalancers[0].LoadBalancerArn
	suite.resources.LoadBalancerARNs = append(suite.resources.LoadBalancerARNs, nlbARN)

	t.Logf("  Created NLB: %s", *nlb.LoadBalancers[0].LoadBalancerName)
	t.Logf("  DNS Name: %s", *nlb.LoadBalancers[0].DNSName)
	t.Logf("  ARN: %s", nlbARN)

	// Create Target Group
	t.Log("Creating Target Group...")
	tgName := fmt.Sprintf("%s-tg-%d", testPrefix, time.Now().Unix())
	tgName = strings.ReplaceAll(tgName, "_", "-")
	if len(tgName) > 32 {
		tgName = tgName[:32]
	}

	tg, err := suite.elbClient.CreateTargetGroup(ctx, &elasticloadbalancingv2.CreateTargetGroupInput{
		Name:                       aws.String(tgName),
		Port:                       aws.Int32(6443),
		Protocol:                   elbtypes.ProtocolEnumTcp,
		VpcId:                      vpc.Vpc.VpcId,
		TargetType:                 elbtypes.TargetTypeEnumInstance,
		HealthCheckProtocol:        elbtypes.ProtocolEnumTcp,
		HealthCheckPort:            aws.String("6443"),
		HealthCheckIntervalSeconds: aws.Int32(30),
		HealthyThresholdCount:      aws.Int32(2),
		UnhealthyThresholdCount:    aws.Int32(2),
		Tags: []elbtypes.Tag{
			{Key: aws.String("Name"), Value: aws.String(fmt.Sprintf("%s-k8s-api-tg", testPrefix))},
		},
	})
	require.NoError(t, err)
	require.Len(t, tg.TargetGroups, 1)

	tgARN := *tg.TargetGroups[0].TargetGroupArn
	suite.resources.TargetGroupARNs = append(suite.resources.TargetGroupARNs, tgARN)

	t.Logf("  Created Target Group: %s", *tg.TargetGroups[0].TargetGroupName)

	// Create Listener
	t.Log("Creating Listener...")
	listener, err := suite.elbClient.CreateListener(ctx, &elasticloadbalancingv2.CreateListenerInput{
		LoadBalancerArn: aws.String(nlbARN),
		Port:            aws.Int32(6443),
		Protocol:        elbtypes.ProtocolEnumTcp,
		DefaultActions: []elbtypes.Action{
			{
				Type:           elbtypes.ActionTypeEnumForward,
				TargetGroupArn: aws.String(tgARN),
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, listener.Listeners, 1)

	t.Logf("  Created Listener on port 6443")

	t.Log("\n=== Load Balancer Test Summary ===")
	t.Logf("NLB DNS: %s", *nlb.LoadBalancers[0].DNSName)
	t.Logf("Target Group: %s", *tg.TargetGroups[0].TargetGroupName)
	t.Log("=== Load Balancer Test PASSED ===")
}

// =============================================================================
// HELPERS
// =============================================================================

func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
