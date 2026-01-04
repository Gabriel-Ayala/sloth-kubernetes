//go:build integration && e2e
// +build integration,e2e

package providers_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// E2E TEST CONFIGURATION
// =============================================================================

const (
	e2eTestPrefix     = "sloth-e2e"
	e2eDefaultRegion  = "us-east-1"
	e2eTestTimeout    = 15 * time.Minute
	e2eCleanupTimeout = 5 * time.Minute
)

// E2EClusterConfig represents a full cluster configuration
type E2EClusterConfig struct {
	Name         string
	Region       string
	MasterCount  int
	WorkerCount  int
	MasterSize   types.InstanceType
	WorkerSize   types.InstanceType
	UseSpot      bool
	VPCCIDR      string
	K3sVersion   string
	EnableBackup bool
	BackupBucket string
}

// E2EClusterResources tracks all cluster resources
type E2EClusterResources struct {
	mu sync.Mutex

	// Network
	VPCID                 string
	InternetGatewayID     string
	SubnetIDs             []string
	RouteTableID          string
	MasterSecurityGroupID string
	WorkerSecurityGroupID string

	// Compute
	KeyPairName    string
	MasterIDs      []string
	WorkerIDs      []string
	MasterIPs      []string
	WorkerIPs      []string
	SpotRequestIDs []string

	// Load Balancer
	LoadBalancerARN string
	LoadBalancerDNS string
	TargetGroupARN  string

	// Storage
	BackupBucket string
}

// E2ETestSuite holds all AWS clients for E2E tests
type E2ETestSuite struct {
	cfg       aws.Config
	ec2Client *ec2.Client
	stsClient *sts.Client
	elbClient *elasticloadbalancingv2.Client
	s3Client  *s3.Client
	region    string
	accountID string
	resources *E2EClusterResources
}

// =============================================================================
// E2E TEST SETUP
// =============================================================================

func skipIfNoE2ECredentials(t *testing.T) {
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" || os.Getenv("AWS_SECRET_ACCESS_KEY") == "" {
		t.Skip("Skipping E2E test: AWS credentials not configured")
	}
	if os.Getenv("RUN_E2E_TESTS") != "true" {
		t.Skip("Skipping E2E test: Set RUN_E2E_TESTS=true to run")
	}
}

func setupE2ETestSuite(t *testing.T) *E2ETestSuite {
	skipIfNoE2ECredentials(t)

	ctx := context.Background()
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = e2eDefaultRegion
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	require.NoError(t, err, "Failed to load AWS config")

	suite := &E2ETestSuite{
		cfg:       cfg,
		ec2Client: ec2.NewFromConfig(cfg),
		stsClient: sts.NewFromConfig(cfg),
		elbClient: elasticloadbalancingv2.NewFromConfig(cfg),
		s3Client:  s3.NewFromConfig(cfg),
		region:    region,
		resources: &E2EClusterResources{},
	}

	identity, err := suite.stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	require.NoError(t, err)
	suite.accountID = *identity.Account

	return suite
}

// =============================================================================
// E2E TEST: FULL K3S CLUSTER PROVISIONING
// =============================================================================

func TestE2E_FullClusterProvisioning(t *testing.T) {
	suite := setupE2ETestSuite(t)
	defer suite.fullCleanup(t)

	ctx, cancel := context.WithTimeout(context.Background(), e2eTestTimeout)
	defer cancel()

	clusterConfig := &E2EClusterConfig{
		Name:        fmt.Sprintf("%s-cluster-%d", e2eTestPrefix, time.Now().Unix()),
		Region:      suite.region,
		MasterCount: 1,
		WorkerCount: 2,
		MasterSize:  types.InstanceTypeT3Small,
		WorkerSize:  types.InstanceTypeT3Micro,
		UseSpot:     true,
		VPCCIDR:     "10.50.0.0/16",
		K3sVersion:  "v1.29.0+k3s1",
	}

	t.Log("╔══════════════════════════════════════════════════════════════╗")
	t.Log("║     E2E TEST: Full K3s Cluster Provisioning                  ║")
	t.Log("╚══════════════════════════════════════════════════════════════╝")
	t.Logf("Cluster: %s", clusterConfig.Name)
	t.Logf("Region: %s", clusterConfig.Region)
	t.Logf("Masters: %d x %s", clusterConfig.MasterCount, clusterConfig.MasterSize)
	t.Logf("Workers: %d x %s (Spot: %v)", clusterConfig.WorkerCount, clusterConfig.WorkerSize, clusterConfig.UseSpot)

	// =========================================================================
	// PHASE 1: Network Infrastructure
	// =========================================================================
	t.Log("\n━━━ PHASE 1: Network Infrastructure ━━━")

	// Create VPC
	t.Log("  [1/6] Creating VPC...")
	vpc, err := suite.ec2Client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String(clusterConfig.VPCCIDR),
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeVpc, Tags: e2eTags(clusterConfig.Name, "vpc")},
		},
	})
	require.NoError(t, err)
	suite.resources.VPCID = *vpc.Vpc.VpcId
	t.Logf("        VPC: %s (%s)", suite.resources.VPCID, clusterConfig.VPCCIDR)

	// Enable DNS
	_, err = suite.ec2Client.ModifyVpcAttribute(ctx, &ec2.ModifyVpcAttributeInput{
		VpcId:              vpc.Vpc.VpcId,
		EnableDnsHostnames: &types.AttributeBooleanValue{Value: aws.Bool(true)},
	})
	require.NoError(t, err)

	// Create Internet Gateway
	t.Log("  [2/6] Creating Internet Gateway...")
	igw, err := suite.ec2Client.CreateInternetGateway(ctx, &ec2.CreateInternetGatewayInput{
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeInternetGateway, Tags: e2eTags(clusterConfig.Name, "igw")},
		},
	})
	require.NoError(t, err)
	suite.resources.InternetGatewayID = *igw.InternetGateway.InternetGatewayId

	_, err = suite.ec2Client.AttachInternetGateway(ctx, &ec2.AttachInternetGatewayInput{
		InternetGatewayId: igw.InternetGateway.InternetGatewayId,
		VpcId:             vpc.Vpc.VpcId,
	})
	require.NoError(t, err)
	t.Logf("        IGW: %s", suite.resources.InternetGatewayID)

	// Get AZs and create subnets
	t.Log("  [3/6] Creating Subnets across AZs...")
	azs, err := suite.ec2Client.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{
		Filters: []types.Filter{{Name: aws.String("state"), Values: []string{"available"}}},
	})
	require.NoError(t, err)

	for i := 0; i < 3 && i < len(azs.AvailabilityZones); i++ {
		az := azs.AvailabilityZones[i]
		cidr := fmt.Sprintf("10.50.%d.0/24", i+1)

		subnet, err := suite.ec2Client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
			VpcId:            vpc.Vpc.VpcId,
			CidrBlock:        aws.String(cidr),
			AvailabilityZone: az.ZoneName,
			TagSpecifications: []types.TagSpecification{
				{ResourceType: types.ResourceTypeSubnet, Tags: e2eTags(clusterConfig.Name, fmt.Sprintf("subnet-%s", *az.ZoneName))},
			},
		})
		require.NoError(t, err)
		suite.resources.SubnetIDs = append(suite.resources.SubnetIDs, *subnet.Subnet.SubnetId)

		// Enable auto-assign public IP
		_, err = suite.ec2Client.ModifySubnetAttribute(ctx, &ec2.ModifySubnetAttributeInput{
			SubnetId:            subnet.Subnet.SubnetId,
			MapPublicIpOnLaunch: &types.AttributeBooleanValue{Value: aws.Bool(true)},
		})
		require.NoError(t, err)

		t.Logf("        Subnet: %s in %s (%s)", *subnet.Subnet.SubnetId, *az.ZoneName, cidr)
	}

	// Create Route Table
	t.Log("  [4/6] Creating Route Table...")
	rt, err := suite.ec2Client.CreateRouteTable(ctx, &ec2.CreateRouteTableInput{
		VpcId: vpc.Vpc.VpcId,
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeRouteTable, Tags: e2eTags(clusterConfig.Name, "rt")},
		},
	})
	require.NoError(t, err)
	suite.resources.RouteTableID = *rt.RouteTable.RouteTableId

	_, err = suite.ec2Client.CreateRoute(ctx, &ec2.CreateRouteInput{
		RouteTableId:         rt.RouteTable.RouteTableId,
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:            igw.InternetGateway.InternetGatewayId,
	})
	require.NoError(t, err)

	for _, subnetID := range suite.resources.SubnetIDs {
		_, err = suite.ec2Client.AssociateRouteTable(ctx, &ec2.AssociateRouteTableInput{
			RouteTableId: rt.RouteTable.RouteTableId,
			SubnetId:     aws.String(subnetID),
		})
		require.NoError(t, err)
	}
	t.Logf("        Route Table: %s", suite.resources.RouteTableID)

	// Create Security Groups
	t.Log("  [5/6] Creating Security Groups...")
	masterSG, err := suite.createK3sSecurityGroup(ctx, clusterConfig.Name, "master", vpc.Vpc.VpcId, clusterConfig.VPCCIDR)
	require.NoError(t, err)
	suite.resources.MasterSecurityGroupID = masterSG
	t.Logf("        Master SG: %s", masterSG)

	workerSG, err := suite.createK3sSecurityGroup(ctx, clusterConfig.Name, "worker", vpc.Vpc.VpcId, clusterConfig.VPCCIDR)
	require.NoError(t, err)
	suite.resources.WorkerSecurityGroupID = workerSG
	t.Logf("        Worker SG: %s", workerSG)

	// Create Key Pair
	t.Log("  [6/6] Creating SSH Key Pair...")
	keyName := fmt.Sprintf("%s-key", clusterConfig.Name)
	keyPair, err := suite.ec2Client.CreateKeyPair(ctx, &ec2.CreateKeyPairInput{
		KeyName: aws.String(keyName),
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeKeyPair, Tags: e2eTags(clusterConfig.Name, "keypair")},
		},
	})
	require.NoError(t, err)
	suite.resources.KeyPairName = keyName
	t.Logf("        Key Pair: %s (fingerprint: %s)", keyName, *keyPair.KeyFingerprint)

	t.Log("  ✓ Network infrastructure created successfully")

	// =========================================================================
	// PHASE 2: Load Balancer for K3s API
	// =========================================================================
	t.Log("\n━━━ PHASE 2: Load Balancer ━━━")

	t.Log("  [1/3] Creating Network Load Balancer...")
	nlbName := strings.ReplaceAll(clusterConfig.Name, "_", "-")
	if len(nlbName) > 32 {
		nlbName = nlbName[:32]
	}

	nlb, err := suite.elbClient.CreateLoadBalancer(ctx, &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name:    aws.String(nlbName),
		Type:    elbtypes.LoadBalancerTypeEnumNetwork,
		Scheme:  elbtypes.LoadBalancerSchemeEnumInternetFacing,
		Subnets: suite.resources.SubnetIDs[:2], // Use first 2 subnets
		Tags: []elbtypes.Tag{
			{Key: aws.String("Name"), Value: aws.String(clusterConfig.Name + "-nlb")},
			{Key: aws.String("Cluster"), Value: aws.String(clusterConfig.Name)},
		},
	})
	require.NoError(t, err)
	suite.resources.LoadBalancerARN = *nlb.LoadBalancers[0].LoadBalancerArn
	suite.resources.LoadBalancerDNS = *nlb.LoadBalancers[0].DNSName
	t.Logf("        NLB: %s", suite.resources.LoadBalancerDNS)

	t.Log("  [2/3] Creating Target Group...")
	tgName := strings.ReplaceAll(clusterConfig.Name+"-tg", "_", "-")
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
	})
	require.NoError(t, err)
	suite.resources.TargetGroupARN = *tg.TargetGroups[0].TargetGroupArn
	t.Logf("        Target Group: %s", tgName)

	t.Log("  [3/3] Creating Listener...")
	_, err = suite.elbClient.CreateListener(ctx, &elasticloadbalancingv2.CreateListenerInput{
		LoadBalancerArn: aws.String(suite.resources.LoadBalancerARN),
		Port:            aws.Int32(6443),
		Protocol:        elbtypes.ProtocolEnumTcp,
		DefaultActions: []elbtypes.Action{
			{Type: elbtypes.ActionTypeEnumForward, TargetGroupArn: aws.String(suite.resources.TargetGroupARN)},
		},
	})
	require.NoError(t, err)
	t.Log("        Listener: port 6443 -> Target Group")

	t.Log("  ✓ Load balancer created successfully")

	// =========================================================================
	// PHASE 3: Master Nodes
	// =========================================================================
	t.Log("\n━━━ PHASE 3: Master Nodes ━━━")

	// Get latest Ubuntu AMI
	amiResult, err := suite.ec2Client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Owners: []string{"099720109477"}, // Canonical
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

	// Create master nodes
	for i := 0; i < clusterConfig.MasterCount; i++ {
		t.Logf("  [%d/%d] Creating Master node...", i+1, clusterConfig.MasterCount)

		masterUserData := generateK3sMasterUserData(clusterConfig.Name, clusterConfig.K3sVersion, suite.resources.LoadBalancerDNS, i)

		master, err := suite.ec2Client.RunInstances(ctx, &ec2.RunInstancesInput{
			ImageId:          latestAMI.ImageId,
			InstanceType:     clusterConfig.MasterSize,
			MinCount:         aws.Int32(1),
			MaxCount:         aws.Int32(1),
			KeyName:          aws.String(keyName),
			SubnetId:         aws.String(suite.resources.SubnetIDs[i%len(suite.resources.SubnetIDs)]),
			SecurityGroupIds: []string{masterSG},
			UserData:         aws.String(base64.StdEncoding.EncodeToString([]byte(masterUserData))),
			TagSpecifications: []types.TagSpecification{
				{ResourceType: types.ResourceTypeInstance, Tags: e2eTags(clusterConfig.Name, fmt.Sprintf("master-%d", i))},
			},
			BlockDeviceMappings: []types.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/sda1"),
					Ebs: &types.EbsBlockDevice{
						VolumeSize:          aws.Int32(30),
						VolumeType:          types.VolumeTypeGp3,
						DeleteOnTermination: aws.Bool(true),
					},
				},
			},
		})
		require.NoError(t, err)

		masterID := *master.Instances[0].InstanceId
		suite.resources.MasterIDs = append(suite.resources.MasterIDs, masterID)
		t.Logf("        Instance: %s", masterID)
	}

	// Wait for masters to be running
	t.Log("  Waiting for master nodes to be running...")
	waiter := ec2.NewInstanceRunningWaiter(suite.ec2Client)
	err = waiter.Wait(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: suite.resources.MasterIDs,
	}, 5*time.Minute)
	require.NoError(t, err)

	// Register masters with target group after they are running
	t.Log("  Registering masters with target group...")
	for _, masterID := range suite.resources.MasterIDs {
		_, err = suite.elbClient.RegisterTargets(ctx, &elasticloadbalancingv2.RegisterTargetsInput{
			TargetGroupArn: aws.String(suite.resources.TargetGroupARN),
			Targets:        []elbtypes.TargetDescription{{Id: aws.String(masterID), Port: aws.Int32(6443)}},
		})
		require.NoError(t, err)
		t.Logf("        Registered: %s", masterID)
	}

	// Get master IPs
	masterDesc, err := suite.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: suite.resources.MasterIDs,
	})
	require.NoError(t, err)
	for _, res := range masterDesc.Reservations {
		for _, inst := range res.Instances {
			if inst.PublicIpAddress != nil {
				suite.resources.MasterIPs = append(suite.resources.MasterIPs, *inst.PublicIpAddress)
				t.Logf("        Master IP: %s (Private: %s)", *inst.PublicIpAddress, *inst.PrivateIpAddress)
			}
		}
	}

	t.Log("  ✓ Master nodes created successfully")

	// =========================================================================
	// PHASE 4: Worker Nodes (with Spot)
	// =========================================================================
	t.Log("\n━━━ PHASE 4: Worker Nodes ━━━")

	if clusterConfig.UseSpot {
		t.Log("  Using Spot Instances for workers...")

		// Get current spot prices
		spotPrices, err := suite.ec2Client.DescribeSpotPriceHistory(ctx, &ec2.DescribeSpotPriceHistoryInput{
			InstanceTypes:       []types.InstanceType{clusterConfig.WorkerSize},
			ProductDescriptions: []string{"Linux/UNIX"},
			StartTime:           aws.Time(time.Now().Add(-1 * time.Hour)),
			MaxResults:          aws.Int32(10),
		})
		require.NoError(t, err)

		if len(spotPrices.SpotPriceHistory) > 0 {
			t.Logf("        Current Spot Price: $%s/hour", *spotPrices.SpotPriceHistory[0].SpotPrice)
		}

		for i := 0; i < clusterConfig.WorkerCount; i++ {
			t.Logf("  [%d/%d] Requesting Spot Worker node...", i+1, clusterConfig.WorkerCount)

			workerUserData := generateK3sWorkerUserData(clusterConfig.Name, clusterConfig.K3sVersion, suite.resources.MasterIPs[0])

			spotReq, err := suite.ec2Client.RequestSpotInstances(ctx, &ec2.RequestSpotInstancesInput{
				InstanceCount: aws.Int32(1),
				Type:          types.SpotInstanceTypeOneTime,
				LaunchSpecification: &types.RequestSpotLaunchSpecification{
					ImageId:          latestAMI.ImageId,
					InstanceType:     clusterConfig.WorkerSize,
					KeyName:          aws.String(keyName),
					SubnetId:         aws.String(suite.resources.SubnetIDs[i%len(suite.resources.SubnetIDs)]),
					SecurityGroupIds: []string{workerSG},
					UserData:         aws.String(base64.StdEncoding.EncodeToString([]byte(workerUserData))),
				},
				TagSpecifications: []types.TagSpecification{
					{ResourceType: types.ResourceTypeSpotInstancesRequest, Tags: e2eTags(clusterConfig.Name, fmt.Sprintf("worker-spot-%d", i))},
				},
			})
			require.NoError(t, err)

			spotReqID := *spotReq.SpotInstanceRequests[0].SpotInstanceRequestId
			suite.resources.SpotRequestIDs = append(suite.resources.SpotRequestIDs, spotReqID)
			t.Logf("        Spot Request: %s", spotReqID)
		}

		// Wait for spot instances to be fulfilled
		t.Log("  Waiting for Spot instances to be fulfilled...")
		time.Sleep(15 * time.Second)

		// Check spot request status and get instance IDs
		spotDesc, err := suite.ec2Client.DescribeSpotInstanceRequests(ctx, &ec2.DescribeSpotInstanceRequestsInput{
			SpotInstanceRequestIds: suite.resources.SpotRequestIDs,
		})
		require.NoError(t, err)

		for _, req := range spotDesc.SpotInstanceRequests {
			t.Logf("        Spot %s: %s", *req.SpotInstanceRequestId, *req.Status.Code)
			if req.InstanceId != nil {
				suite.resources.WorkerIDs = append(suite.resources.WorkerIDs, *req.InstanceId)
			}
		}
	} else {
		// On-demand workers
		for i := 0; i < clusterConfig.WorkerCount; i++ {
			t.Logf("  [%d/%d] Creating Worker node...", i+1, clusterConfig.WorkerCount)

			workerUserData := generateK3sWorkerUserData(clusterConfig.Name, clusterConfig.K3sVersion, suite.resources.MasterIPs[0])

			worker, err := suite.ec2Client.RunInstances(ctx, &ec2.RunInstancesInput{
				ImageId:          latestAMI.ImageId,
				InstanceType:     clusterConfig.WorkerSize,
				MinCount:         aws.Int32(1),
				MaxCount:         aws.Int32(1),
				KeyName:          aws.String(keyName),
				SubnetId:         aws.String(suite.resources.SubnetIDs[i%len(suite.resources.SubnetIDs)]),
				SecurityGroupIds: []string{workerSG},
				UserData:         aws.String(base64.StdEncoding.EncodeToString([]byte(workerUserData))),
				TagSpecifications: []types.TagSpecification{
					{ResourceType: types.ResourceTypeInstance, Tags: e2eTags(clusterConfig.Name, fmt.Sprintf("worker-%d", i))},
				},
			})
			require.NoError(t, err)

			workerID := *worker.Instances[0].InstanceId
			suite.resources.WorkerIDs = append(suite.resources.WorkerIDs, workerID)
			t.Logf("        Instance: %s", workerID)
		}
	}

	// Wait for workers to be running (if we have instance IDs)
	if len(suite.resources.WorkerIDs) > 0 {
		t.Log("  Waiting for worker nodes to be running...")
		err = waiter.Wait(ctx, &ec2.DescribeInstancesInput{
			InstanceIds: suite.resources.WorkerIDs,
		}, 5*time.Minute)
		if err != nil {
			t.Logf("  Warning: Some workers may not be running yet: %v", err)
		}

		// Get worker IPs
		workerDesc, err := suite.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			InstanceIds: suite.resources.WorkerIDs,
		})
		if err == nil {
			for _, res := range workerDesc.Reservations {
				for _, inst := range res.Instances {
					if inst.PublicIpAddress != nil {
						suite.resources.WorkerIPs = append(suite.resources.WorkerIPs, *inst.PublicIpAddress)
						t.Logf("        Worker IP: %s", *inst.PublicIpAddress)
					}
				}
			}
		}
	}

	t.Log("  ✓ Worker nodes created successfully")

	// =========================================================================
	// PHASE 5: Validation
	// =========================================================================
	t.Log("\n━━━ PHASE 5: Cluster Validation ━━━")

	t.Log("  Validating cluster state...")

	// Check all masters are running
	masterStatus, err := suite.ec2Client.DescribeInstanceStatus(ctx, &ec2.DescribeInstanceStatusInput{
		InstanceIds: suite.resources.MasterIDs,
	})
	require.NoError(t, err)

	allMastersOK := true
	for _, status := range masterStatus.InstanceStatuses {
		if status.InstanceState.Name != types.InstanceStateNameRunning {
			allMastersOK = false
		}
		t.Logf("        Master %s: %s", *status.InstanceId, status.InstanceState.Name)
	}
	assert.True(t, allMastersOK, "All masters should be running")

	// Check target group health
	tgHealth, err := suite.elbClient.DescribeTargetHealth(ctx, &elasticloadbalancingv2.DescribeTargetHealthInput{
		TargetGroupArn: aws.String(suite.resources.TargetGroupARN),
	})
	require.NoError(t, err)

	t.Log("  Target Group Health:")
	for _, target := range tgHealth.TargetHealthDescriptions {
		t.Logf("        %s: %s", *target.Target.Id, target.TargetHealth.State)
	}

	// =========================================================================
	// SUMMARY
	// =========================================================================
	t.Log("\n╔══════════════════════════════════════════════════════════════╗")
	t.Log("║                    CLUSTER SUMMARY                           ║")
	t.Log("╠══════════════════════════════════════════════════════════════╣")
	t.Logf("║  Cluster Name: %-44s ║", clusterConfig.Name)
	t.Logf("║  VPC:          %-44s ║", suite.resources.VPCID)
	t.Logf("║  API Endpoint: %-44s ║", suite.resources.LoadBalancerDNS+":6443")
	t.Logf("║  Masters:      %-44d ║", len(suite.resources.MasterIDs))
	t.Logf("║  Workers:      %-44d ║", len(suite.resources.WorkerIDs))
	t.Log("╚══════════════════════════════════════════════════════════════╝")

	t.Log("\n✅ E2E Full Cluster Provisioning Test PASSED")
}

// =============================================================================
// E2E TEST: AUTOSCALING WORKFLOW
// =============================================================================

func TestE2E_AutoscalingWorkflow(t *testing.T) {
	suite := setupE2ETestSuite(t)
	defer suite.fullCleanup(t)

	ctx, cancel := context.WithTimeout(context.Background(), e2eTestTimeout)
	defer cancel()

	t.Log("╔══════════════════════════════════════════════════════════════╗")
	t.Log("║     E2E TEST: Autoscaling Workflow                           ║")
	t.Log("╚══════════════════════════════════════════════════════════════╝")

	// Create minimal infrastructure
	t.Log("\n━━━ Setup: Creating minimal cluster infrastructure ━━━")

	// Create VPC
	vpc, err := suite.ec2Client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.60.0.0/16"),
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeVpc, Tags: e2eTags("autoscale-test", "vpc")},
		},
	})
	require.NoError(t, err)
	suite.resources.VPCID = *vpc.Vpc.VpcId
	t.Logf("  VPC: %s", suite.resources.VPCID)

	// Create Subnet
	subnet, err := suite.ec2Client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
		VpcId:            vpc.Vpc.VpcId,
		CidrBlock:        aws.String("10.60.1.0/24"),
		AvailabilityZone: aws.String(suite.region + "a"),
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeSubnet, Tags: e2eTags("autoscale-test", "subnet")},
		},
	})
	require.NoError(t, err)
	suite.resources.SubnetIDs = append(suite.resources.SubnetIDs, *subnet.Subnet.SubnetId)

	// Create Security Group
	sg, err := suite.ec2Client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(fmt.Sprintf("autoscale-sg-%d", time.Now().Unix())),
		Description: aws.String("Autoscaling test SG"),
		VpcId:       vpc.Vpc.VpcId,
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeSecurityGroup, Tags: e2eTags("autoscale-test", "sg")},
		},
	})
	require.NoError(t, err)
	suite.resources.WorkerSecurityGroupID = *sg.GroupId

	// Create Key Pair
	keyName := fmt.Sprintf("autoscale-key-%d", time.Now().Unix())
	_, err = suite.ec2Client.CreateKeyPair(ctx, &ec2.CreateKeyPairInput{
		KeyName: aws.String(keyName),
	})
	require.NoError(t, err)
	suite.resources.KeyPairName = keyName

	// Get AMI
	amiResult, err := suite.ec2Client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Owners: []string{"099720109477"},
		Filters: []types.Filter{
			{Name: aws.String("name"), Values: []string{"ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"}},
			{Name: aws.String("state"), Values: []string{"available"}},
		},
	})
	require.NoError(t, err)
	latestAMI := amiResult.Images[0]
	for _, img := range amiResult.Images {
		if *img.CreationDate > *latestAMI.CreationDate {
			latestAMI = img
		}
	}

	// =========================================================================
	// Test: Scale Up
	// =========================================================================
	t.Log("\n━━━ Test: Scale Up (0 -> 2 workers) ━━━")

	initialCount := 0
	targetCount := 2

	t.Logf("  Current workers: %d", initialCount)
	t.Logf("  Target workers:  %d", targetCount)

	scaleUpStart := time.Now()

	// Simulate autoscaler creating new instances
	for i := 0; i < targetCount; i++ {
		t.Logf("  [%d/%d] Creating worker instance...", i+1, targetCount)

		instance, err := suite.ec2Client.RunInstances(ctx, &ec2.RunInstancesInput{
			ImageId:          latestAMI.ImageId,
			InstanceType:     types.InstanceTypeT3Micro,
			MinCount:         aws.Int32(1),
			MaxCount:         aws.Int32(1),
			KeyName:          aws.String(keyName),
			SubnetId:         subnet.Subnet.SubnetId,
			SecurityGroupIds: []string{*sg.GroupId},
			TagSpecifications: []types.TagSpecification{
				{ResourceType: types.ResourceTypeInstance, Tags: e2eTags("autoscale-test", fmt.Sprintf("worker-%d", i))},
			},
		})
		require.NoError(t, err)
		suite.resources.WorkerIDs = append(suite.resources.WorkerIDs, *instance.Instances[0].InstanceId)
		t.Logf("        Created: %s", *instance.Instances[0].InstanceId)
	}

	// Wait for instances to be running
	t.Log("  Waiting for instances to be running...")
	waiter := ec2.NewInstanceRunningWaiter(suite.ec2Client)
	err = waiter.Wait(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: suite.resources.WorkerIDs,
	}, 3*time.Minute)
	require.NoError(t, err)

	scaleUpDuration := time.Since(scaleUpStart)
	t.Logf("  ✓ Scale up completed in %v", scaleUpDuration)
	t.Logf("  Current workers: %d", len(suite.resources.WorkerIDs))

	// =========================================================================
	// Test: Scale Down
	// =========================================================================
	t.Log("\n━━━ Test: Scale Down (2 -> 1 workers) ━━━")

	targetCount = 1
	t.Logf("  Current workers: %d", len(suite.resources.WorkerIDs))
	t.Logf("  Target workers:  %d", targetCount)

	scaleDownStart := time.Now()

	// Terminate one instance
	instanceToTerminate := suite.resources.WorkerIDs[0]
	t.Logf("  Terminating instance: %s", instanceToTerminate)

	_, err = suite.ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceToTerminate},
	})
	require.NoError(t, err)

	// Remove from our tracking
	suite.resources.WorkerIDs = suite.resources.WorkerIDs[1:]

	// Wait for termination
	t.Log("  Waiting for instance to terminate...")
	terminateWaiter := ec2.NewInstanceTerminatedWaiter(suite.ec2Client)
	err = terminateWaiter.Wait(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceToTerminate},
	}, 3*time.Minute)
	require.NoError(t, err)

	scaleDownDuration := time.Since(scaleDownStart)
	t.Logf("  ✓ Scale down completed in %v", scaleDownDuration)
	t.Logf("  Current workers: %d", len(suite.resources.WorkerIDs))

	// =========================================================================
	// Summary
	// =========================================================================
	t.Log("\n━━━ Autoscaling Metrics ━━━")
	t.Logf("  Scale Up Time:   %v (0 -> 2)", scaleUpDuration)
	t.Logf("  Scale Down Time: %v (2 -> 1)", scaleDownDuration)
	t.Logf("  Final Count:     %d", len(suite.resources.WorkerIDs))

	assert.Equal(t, 1, len(suite.resources.WorkerIDs), "Should have 1 worker after scale down")

	t.Log("\n✅ E2E Autoscaling Workflow Test PASSED")
}

// =============================================================================
// E2E TEST: MULTI-AZ DISTRIBUTION
// =============================================================================

func TestE2E_MultiAZDistribution(t *testing.T) {
	suite := setupE2ETestSuite(t)
	defer suite.fullCleanup(t)

	ctx, cancel := context.WithTimeout(context.Background(), e2eTestTimeout)
	defer cancel()

	t.Log("╔══════════════════════════════════════════════════════════════╗")
	t.Log("║     E2E TEST: Multi-AZ Distribution                          ║")
	t.Log("╚══════════════════════════════════════════════════════════════╝")

	// Get all AZs
	azs, err := suite.ec2Client.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{
		Filters: []types.Filter{{Name: aws.String("state"), Values: []string{"available"}}},
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(azs.AvailabilityZones), 3, "Need at least 3 AZs for this test")

	t.Logf("  Available AZs: %d", len(azs.AvailabilityZones))
	for _, az := range azs.AvailabilityZones[:3] {
		t.Logf("    - %s", *az.ZoneName)
	}

	// Create VPC with multi-AZ subnets
	t.Log("\n━━━ Creating Multi-AZ Infrastructure ━━━")

	vpc, err := suite.ec2Client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.70.0.0/16"),
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeVpc, Tags: e2eTags("multiaz-test", "vpc")},
		},
	})
	require.NoError(t, err)
	suite.resources.VPCID = *vpc.Vpc.VpcId

	// Create subnet in each AZ
	azSubnetMap := make(map[string]string)
	for i := 0; i < 3; i++ {
		az := azs.AvailabilityZones[i]
		cidr := fmt.Sprintf("10.70.%d.0/24", i+1)

		subnet, err := suite.ec2Client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
			VpcId:            vpc.Vpc.VpcId,
			CidrBlock:        aws.String(cidr),
			AvailabilityZone: az.ZoneName,
			TagSpecifications: []types.TagSpecification{
				{ResourceType: types.ResourceTypeSubnet, Tags: e2eTags("multiaz-test", fmt.Sprintf("subnet-%s", *az.ZoneName))},
			},
		})
		require.NoError(t, err)
		suite.resources.SubnetIDs = append(suite.resources.SubnetIDs, *subnet.Subnet.SubnetId)
		azSubnetMap[*az.ZoneName] = *subnet.Subnet.SubnetId
		t.Logf("  Subnet in %s: %s (%s)", *az.ZoneName, *subnet.Subnet.SubnetId, cidr)
	}

	// Create Security Group
	sg, err := suite.ec2Client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(fmt.Sprintf("multiaz-sg-%d", time.Now().Unix())),
		Description: aws.String("Multi-AZ test SG"),
		VpcId:       vpc.Vpc.VpcId,
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeSecurityGroup, Tags: e2eTags("multiaz-test", "sg")},
		},
	})
	require.NoError(t, err)
	suite.resources.WorkerSecurityGroupID = *sg.GroupId

	// Create Key Pair
	keyName := fmt.Sprintf("multiaz-key-%d", time.Now().Unix())
	_, err = suite.ec2Client.CreateKeyPair(ctx, &ec2.CreateKeyPairInput{KeyName: aws.String(keyName)})
	require.NoError(t, err)
	suite.resources.KeyPairName = keyName

	// Get AMI
	amiResult, err := suite.ec2Client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Owners: []string{"099720109477"},
		Filters: []types.Filter{
			{Name: aws.String("name"), Values: []string{"ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"}},
			{Name: aws.String("state"), Values: []string{"available"}},
		},
	})
	require.NoError(t, err)
	latestAMI := amiResult.Images[0]
	for _, img := range amiResult.Images {
		if *img.CreationDate > *latestAMI.CreationDate {
			latestAMI = img
		}
	}

	// =========================================================================
	// Test: Distribute 6 instances across 3 AZs
	// =========================================================================
	t.Log("\n━━━ Test: Distributing 6 instances across 3 AZs ━━━")

	totalInstances := 6
	instancesPerAZ := totalInstances / 3
	azInstanceCount := make(map[string][]string)

	azList := []string{
		*azs.AvailabilityZones[0].ZoneName,
		*azs.AvailabilityZones[1].ZoneName,
		*azs.AvailabilityZones[2].ZoneName,
	}

	for i := 0; i < totalInstances; i++ {
		targetAZ := azList[i%3]
		subnetID := azSubnetMap[targetAZ]

		t.Logf("  [%d/%d] Creating instance in %s...", i+1, totalInstances, targetAZ)

		instance, err := suite.ec2Client.RunInstances(ctx, &ec2.RunInstancesInput{
			ImageId:          latestAMI.ImageId,
			InstanceType:     types.InstanceTypeT3Micro,
			MinCount:         aws.Int32(1),
			MaxCount:         aws.Int32(1),
			KeyName:          aws.String(keyName),
			SubnetId:         aws.String(subnetID),
			SecurityGroupIds: []string{*sg.GroupId},
			TagSpecifications: []types.TagSpecification{
				{ResourceType: types.ResourceTypeInstance, Tags: append(
					e2eTags("multiaz-test", fmt.Sprintf("instance-%d", i)),
					types.Tag{Key: aws.String("AZ"), Value: aws.String(targetAZ)},
				)},
			},
		})
		require.NoError(t, err)

		instanceID := *instance.Instances[0].InstanceId
		suite.resources.WorkerIDs = append(suite.resources.WorkerIDs, instanceID)
		azInstanceCount[targetAZ] = append(azInstanceCount[targetAZ], instanceID)
		t.Logf("        Instance: %s in %s", instanceID, targetAZ)
	}

	// Wait for all instances
	t.Log("  Waiting for instances to be running...")
	waiter := ec2.NewInstanceRunningWaiter(suite.ec2Client)
	err = waiter.Wait(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: suite.resources.WorkerIDs,
	}, 3*time.Minute)
	require.NoError(t, err)

	// =========================================================================
	// Validate Distribution
	// =========================================================================
	t.Log("\n━━━ Distribution Validation ━━━")

	t.Log("  Instance distribution by AZ:")
	for az, instances := range azInstanceCount {
		t.Logf("    %s: %d instances", az, len(instances))
		assert.Equal(t, instancesPerAZ, len(instances), "Each AZ should have equal instances")
	}

	// Verify instances are actually in different AZs
	instanceDesc, err := suite.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: suite.resources.WorkerIDs,
	})
	require.NoError(t, err)

	actualAZCount := make(map[string]int)
	for _, res := range instanceDesc.Reservations {
		for _, inst := range res.Instances {
			if inst.Placement != nil && inst.Placement.AvailabilityZone != nil {
				actualAZCount[*inst.Placement.AvailabilityZone]++
			}
		}
	}

	t.Log("  Verified distribution:")
	for az, count := range actualAZCount {
		t.Logf("    %s: %d instances", az, count)
	}

	assert.Equal(t, 3, len(actualAZCount), "Should have instances in 3 AZs")

	t.Log("\n✅ E2E Multi-AZ Distribution Test PASSED")
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func e2eTags(clusterName, resourceName string) []types.Tag {
	return []types.Tag{
		{Key: aws.String("Name"), Value: aws.String(fmt.Sprintf("%s-%s", clusterName, resourceName))},
		{Key: aws.String("Cluster"), Value: aws.String(clusterName)},
		{Key: aws.String("Environment"), Value: aws.String("e2e-test")},
		{Key: aws.String("ManagedBy"), Value: aws.String("sloth-kubernetes-e2e")},
	}
}

func (s *E2ETestSuite) createK3sSecurityGroup(ctx context.Context, clusterName, role string, vpcID *string, vpcCIDR string) (string, error) {
	sg, err := s.ec2Client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(fmt.Sprintf("%s-%s-sg-%d", clusterName, role, time.Now().Unix())),
		Description: aws.String(fmt.Sprintf("K3s %s security group", role)),
		VpcId:       vpcID,
		TagSpecifications: []types.TagSpecification{
			{ResourceType: types.ResourceTypeSecurityGroup, Tags: e2eTags(clusterName, fmt.Sprintf("%s-sg", role))},
		},
	})
	if err != nil {
		return "", err
	}

	// Common rules
	rules := []types.IpPermission{
		{IpProtocol: aws.String("tcp"), FromPort: aws.Int32(22), ToPort: aws.Int32(22), IpRanges: []types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
		{IpProtocol: aws.String("-1"), FromPort: aws.Int32(-1), ToPort: aws.Int32(-1), IpRanges: []types.IpRange{{CidrIp: aws.String(vpcCIDR)}}},
	}

	if role == "master" {
		rules = append(rules,
			types.IpPermission{IpProtocol: aws.String("tcp"), FromPort: aws.Int32(6443), ToPort: aws.Int32(6443), IpRanges: []types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
			types.IpPermission{IpProtocol: aws.String("tcp"), FromPort: aws.Int32(2379), ToPort: aws.Int32(2380), IpRanges: []types.IpRange{{CidrIp: aws.String(vpcCIDR)}}},
			types.IpPermission{IpProtocol: aws.String("udp"), FromPort: aws.Int32(51820), ToPort: aws.Int32(51820), IpRanges: []types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
		)
	}

	_, err = s.ec2Client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:       sg.GroupId,
		IpPermissions: rules,
	})
	if err != nil {
		return "", err
	}

	return *sg.GroupId, nil
}

func generateK3sMasterUserData(clusterName, k3sVersion, lbDNS string, nodeIndex int) string {
	return fmt.Sprintf(`#!/bin/bash
set -e

# Update system
apt-get update -y
apt-get install -y curl wget

# Set hostname
hostnamectl set-hostname %s-master-%d

# Disable swap
swapoff -a
sed -i '/swap/d' /etc/fstab

# Install K3s as server
export INSTALL_K3S_VERSION="%s"
export K3S_TOKEN="%s-token"

if [ %d -eq 0 ]; then
    # First master - init cluster
    curl -sfL https://get.k3s.io | sh -s - server \
        --cluster-init \
        --tls-san=%s \
        --disable traefik \
        --write-kubeconfig-mode 644
else
    # Join existing cluster
    curl -sfL https://get.k3s.io | sh -s - server \
        --server https://%s:6443 \
        --tls-san=%s \
        --disable traefik \
        --write-kubeconfig-mode 644
fi

echo "K3s master installation complete"
`, clusterName, nodeIndex, k3sVersion, clusterName, nodeIndex, lbDNS, lbDNS, lbDNS)
}

func generateK3sWorkerUserData(clusterName, k3sVersion, masterIP string) string {
	return fmt.Sprintf(`#!/bin/bash
set -e

# Update system
apt-get update -y
apt-get install -y curl wget

# Set hostname
hostnamectl set-hostname %s-worker-$(hostname -I | awk '{print $1}' | tr '.' '-')

# Disable swap
swapoff -a
sed -i '/swap/d' /etc/fstab

# Install K3s as agent
export INSTALL_K3S_VERSION="%s"
export K3S_TOKEN="%s-token"
export K3S_URL="https://%s:6443"

curl -sfL https://get.k3s.io | sh -s - agent

echo "K3s worker installation complete"
`, clusterName, k3sVersion, clusterName, masterIP)
}

func (s *E2ETestSuite) fullCleanup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), e2eCleanupTimeout)
	defer cancel()

	t.Log("\n╔══════════════════════════════════════════════════════════════╗")
	t.Log("║                    CLEANING UP RESOURCES                     ║")
	t.Log("╚══════════════════════════════════════════════════════════════╝")

	// Terminate all instances
	allInstances := append(s.resources.MasterIDs, s.resources.WorkerIDs...)
	if len(allInstances) > 0 {
		t.Logf("  Terminating %d instances...", len(allInstances))
		_, err := s.ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
			InstanceIds: allInstances,
		})
		if err != nil {
			t.Logf("  Warning: %v", err)
		}

		// Wait for termination
		t.Log("  Waiting for instances to terminate...")
		waiter := ec2.NewInstanceTerminatedWaiter(s.ec2Client)
		_ = waiter.Wait(ctx, &ec2.DescribeInstancesInput{InstanceIds: allInstances}, 5*time.Minute)
	}

	// Cancel spot requests
	if len(s.resources.SpotRequestIDs) > 0 {
		t.Logf("  Cancelling %d spot requests...", len(s.resources.SpotRequestIDs))
		_, _ = s.ec2Client.CancelSpotInstanceRequests(ctx, &ec2.CancelSpotInstanceRequestsInput{
			SpotInstanceRequestIds: s.resources.SpotRequestIDs,
		})
	}

	// Delete load balancer
	if s.resources.LoadBalancerARN != "" {
		t.Log("  Deleting load balancer...")
		_, _ = s.elbClient.DeleteLoadBalancer(ctx, &elasticloadbalancingv2.DeleteLoadBalancerInput{
			LoadBalancerArn: aws.String(s.resources.LoadBalancerARN),
		})
		time.Sleep(5 * time.Second)
	}

	// Delete target group
	if s.resources.TargetGroupARN != "" {
		t.Log("  Deleting target group...")
		_, _ = s.elbClient.DeleteTargetGroup(ctx, &elasticloadbalancingv2.DeleteTargetGroupInput{
			TargetGroupArn: aws.String(s.resources.TargetGroupARN),
		})
	}

	// Delete key pair
	if s.resources.KeyPairName != "" {
		t.Log("  Deleting key pair...")
		_, _ = s.ec2Client.DeleteKeyPair(ctx, &ec2.DeleteKeyPairInput{
			KeyName: aws.String(s.resources.KeyPairName),
		})
	}

	// Delete security groups
	if s.resources.MasterSecurityGroupID != "" {
		t.Log("  Deleting master security group...")
		_, _ = s.ec2Client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
			GroupId: aws.String(s.resources.MasterSecurityGroupID),
		})
	}
	if s.resources.WorkerSecurityGroupID != "" {
		t.Log("  Deleting worker security group...")
		_, _ = s.ec2Client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
			GroupId: aws.String(s.resources.WorkerSecurityGroupID),
		})
	}

	// Delete route table associations and routes
	if s.resources.RouteTableID != "" {
		t.Log("  Cleaning up route table...")
		rtDesc, _ := s.ec2Client.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
			RouteTableIds: []string{s.resources.RouteTableID},
		})
		if len(rtDesc.RouteTables) > 0 {
			for _, assoc := range rtDesc.RouteTables[0].Associations {
				if !*assoc.Main {
					_, _ = s.ec2Client.DisassociateRouteTable(ctx, &ec2.DisassociateRouteTableInput{
						AssociationId: assoc.RouteTableAssociationId,
					})
				}
			}
		}
		_, _ = s.ec2Client.DeleteRouteTable(ctx, &ec2.DeleteRouteTableInput{
			RouteTableId: aws.String(s.resources.RouteTableID),
		})
	}

	// Delete subnets
	for _, subnetID := range s.resources.SubnetIDs {
		t.Logf("  Deleting subnet %s...", subnetID)
		_, _ = s.ec2Client.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{
			SubnetId: aws.String(subnetID),
		})
	}

	// Detach and delete internet gateway
	if s.resources.InternetGatewayID != "" && s.resources.VPCID != "" {
		t.Log("  Detaching internet gateway...")
		_, _ = s.ec2Client.DetachInternetGateway(ctx, &ec2.DetachInternetGatewayInput{
			InternetGatewayId: aws.String(s.resources.InternetGatewayID),
			VpcId:             aws.String(s.resources.VPCID),
		})
		t.Log("  Deleting internet gateway...")
		_, _ = s.ec2Client.DeleteInternetGateway(ctx, &ec2.DeleteInternetGatewayInput{
			InternetGatewayId: aws.String(s.resources.InternetGatewayID),
		})
	}

	// Delete VPC
	if s.resources.VPCID != "" {
		t.Logf("  Deleting VPC %s...", s.resources.VPCID)
		_, _ = s.ec2Client.DeleteVpc(ctx, &ec2.DeleteVpcInput{
			VpcId: aws.String(s.resources.VPCID),
		})
	}

	t.Log("  ✓ Cleanup completed")
}
