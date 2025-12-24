package providers

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

type awsMocks int

// NewResource creates mock resources for AWS tests
func (awsMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	outputs := args.Inputs.Copy()

	switch args.TypeToken {
	case "aws:ec2/vpc:Vpc":
		outputs["id"] = resource.NewStringProperty("vpc-12345678")
		outputs["cidrBlock"] = args.Inputs["cidrBlock"]
		outputs["enableDnsHostnames"] = resource.NewBoolProperty(true)
		outputs["enableDnsSupport"] = resource.NewBoolProperty(true)
		outputs["arn"] = resource.NewStringProperty("arn:aws:ec2:us-east-1:123456789012:vpc/vpc-12345678")

	case "aws:ec2/internetGateway:InternetGateway":
		outputs["id"] = resource.NewStringProperty("igw-12345678")
		outputs["arn"] = resource.NewStringProperty("arn:aws:ec2:us-east-1:123456789012:internet-gateway/igw-12345678")

	case "aws:ec2/subnet:Subnet":
		outputs["id"] = resource.NewStringProperty("subnet-12345678")
		outputs["cidrBlock"] = args.Inputs["cidrBlock"]
		outputs["availabilityZone"] = args.Inputs["availabilityZone"]
		outputs["arn"] = resource.NewStringProperty("arn:aws:ec2:us-east-1:123456789012:subnet/subnet-12345678")

	case "aws:ec2/routeTable:RouteTable":
		outputs["id"] = resource.NewStringProperty("rtb-12345678")
		outputs["arn"] = resource.NewStringProperty("arn:aws:ec2:us-east-1:123456789012:route-table/rtb-12345678")

	case "aws:ec2/routeTableAssociation:RouteTableAssociation":
		outputs["id"] = resource.NewStringProperty("rtbassoc-12345678")

	case "aws:ec2/securityGroup:SecurityGroup":
		outputs["id"] = resource.NewStringProperty("sg-12345678")
		outputs["name"] = args.Inputs["name"]
		outputs["arn"] = resource.NewStringProperty("arn:aws:ec2:us-east-1:123456789012:security-group/sg-12345678")

	case "aws:ec2/keyPair:KeyPair":
		outputs["id"] = resource.NewStringProperty("key-12345678")
		outputs["keyName"] = args.Inputs["keyName"]
		outputs["fingerprint"] = resource.NewStringProperty("ab:cd:ef:12:34:56:78:90")

	case "aws:ec2/instance:Instance":
		outputs["id"] = resource.NewStringProperty("i-12345678")
		outputs["instanceState"] = resource.NewStringProperty("running")
		outputs["publicIp"] = resource.NewStringProperty("54.123.45.67")
		outputs["privateIp"] = resource.NewStringProperty("10.0.1.100")
		outputs["arn"] = resource.NewStringProperty("arn:aws:ec2:us-east-1:123456789012:instance/i-12345678")

	case "aws:ec2/spotInstanceRequest:SpotInstanceRequest":
		outputs["id"] = resource.NewStringProperty("sir-12345678")
		outputs["spotInstanceId"] = resource.NewStringProperty("i-spot-12345678")
		outputs["spotRequestState"] = resource.NewStringProperty("active")
		outputs["publicIp"] = resource.NewStringProperty("54.123.45.68")
		outputs["privateIp"] = resource.NewStringProperty("10.0.1.101")

	case "aws:lb/loadBalancer:LoadBalancer":
		outputs["id"] = resource.NewStringProperty("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/net/my-nlb/1234567890123456")
		outputs["dnsName"] = resource.NewStringProperty("my-nlb-1234567890.us-east-1.elb.amazonaws.com")
		outputs["arn"] = resource.NewStringProperty("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/net/my-nlb/1234567890123456")
		outputs["zoneId"] = resource.NewStringProperty("Z123456789")

	case "aws:lb/targetGroup:TargetGroup":
		outputs["id"] = resource.NewStringProperty("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-tg/1234567890123456")
		outputs["arn"] = resource.NewStringProperty("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-tg/1234567890123456")

	case "aws:lb/targetGroupAttachment:TargetGroupAttachment":
		outputs["id"] = resource.NewStringProperty("tga-12345678")

	case "aws:lb/listener:Listener":
		outputs["id"] = resource.NewStringProperty("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/net/my-nlb/1234567890123456/abcdef1234567890")
		outputs["arn"] = resource.NewStringProperty("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/net/my-nlb/1234567890123456/abcdef1234567890")
	}

	return args.Name + "_id", outputs, nil
}

// Call mocks function calls
func (awsMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return resource.PropertyMap{}, nil
}

func TestAWSProvider_NewAWSProvider(t *testing.T) {
	provider := NewAWSProvider()
	assert.NotNil(t, provider)
	assert.NotNil(t, provider.nodes)
	assert.NotNil(t, provider.subnets)
	assert.Empty(t, provider.nodes)
	assert.Empty(t, provider.subnets)
}

func TestAWSProvider_GetName(t *testing.T) {
	provider := NewAWSProvider()
	assert.Equal(t, "aws", provider.GetName())
}

func TestAWSProvider_GetRegions(t *testing.T) {
	provider := NewAWSProvider()
	regions := provider.GetRegions()

	assert.NotEmpty(t, regions)
	assert.Contains(t, regions, "us-east-1")
	assert.Contains(t, regions, "us-west-2")
	assert.Contains(t, regions, "eu-west-1")
	assert.Contains(t, regions, "ap-southeast-1")
	assert.Contains(t, regions, "sa-east-1")
}

func TestAWSProvider_GetSizes(t *testing.T) {
	provider := NewAWSProvider()
	sizes := provider.GetSizes()

	assert.NotEmpty(t, sizes)
	assert.Contains(t, sizes, "t2.micro")
	assert.Contains(t, sizes, "t3.medium")
	assert.Contains(t, sizes, "m5.large")
	assert.Contains(t, sizes, "c5.xlarge")
	assert.Contains(t, sizes, "r5.large")
}

func TestAWSProvider_Initialize(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "existing-key",
				},
			},
		}

		provider := NewAWSProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)
		assert.Equal(t, "us-east-1", provider.config.Region)

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_Initialize_Disabled(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: false,
					Region:  "us-east-1",
				},
			},
		}

		provider := NewAWSProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not enabled")

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_Initialize_NoRegion(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "",
				},
			},
		}

		provider := NewAWSProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "region is required")

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_Initialize_NilConfig(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: nil,
			},
		}

		provider := NewAWSProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not enabled")

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_CreateNetwork(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "existing-key",
					VPC: &config.VPCConfig{
						Create: true,
						CIDR:   "10.0.0.0/16",
					},
				},
			},
		}

		provider := NewAWSProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		networkConfig := &config.NetworkConfig{
			Mode: "vpc",
			CIDR: "10.0.0.0/16",
		}

		output, err := provider.CreateNetwork(ctx, networkConfig)
		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.NotNil(t, provider.vpc)
		assert.NotNil(t, provider.subnet)
		assert.Len(t, provider.subnets, 2)

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_CreateNetwork_DefaultVPC(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "existing-key",
				},
			},
		}

		provider := NewAWSProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		networkConfig := &config.NetworkConfig{
			Mode: "vpc",
		}

		output, err := provider.CreateNetwork(ctx, networkConfig)
		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, "10.0.0.0/16", output.CIDR)

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_CreateFirewall(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "existing-key",
					VPC: &config.VPCConfig{
						Create: true,
						CIDR:   "10.0.0.0/16",
					},
				},
			},
		}

		provider := NewAWSProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		// Create network first
		_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{
			Mode: "vpc",
			CIDR: "10.0.0.0/16",
		})
		assert.NoError(t, err)

		// Create firewall
		firewallConfig := &config.FirewallConfig{
			Name: "test-firewall",
			InboundRules: []config.FirewallRule{
				{Protocol: "tcp", Port: "8080", Source: []string{"0.0.0.0/0"}},
			},
		}

		err = provider.CreateFirewall(ctx, firewallConfig, nil)
		assert.NoError(t, err)
		assert.NotNil(t, provider.securityGroup)

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_CreateFirewall_NoVPC(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "existing-key",
				},
			},
		}

		provider := NewAWSProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		// Try to create firewall without VPC
		firewallConfig := &config.FirewallConfig{
			Name: "test-firewall",
		}

		err = provider.CreateFirewall(ctx, firewallConfig, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "VPC must be created")

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_CreateNode(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "existing-key",
					VPC: &config.VPCConfig{
						Create: true,
						CIDR:   "10.0.0.0/16",
					},
				},
			},
			Security: config.SecurityConfig{
				SSHConfig: config.SSHConfig{
					KeyPath: "/path/to/key",
				},
			},
		}

		provider := NewAWSProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		// Create network
		_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{
			Mode: "vpc",
			CIDR: "10.0.0.0/16",
		})
		assert.NoError(t, err)

		// Create firewall
		err = provider.CreateFirewall(ctx, &config.FirewallConfig{Name: "test"}, nil)
		assert.NoError(t, err)

		// Create node
		nodeConfig := &config.NodeConfig{
			Name:        "test-node-1",
			Roles:       []string{"master"},
			Size:        "t3.medium",
			Image:       "ubuntu-22-04",
			Region:      "us-east-1",
			WireGuardIP: "10.8.0.20",
			Labels:      map[string]string{"env": "test"},
		}

		output, err := provider.CreateNode(ctx, nodeConfig)
		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, "test-node-1", output.Name)
		assert.Equal(t, "aws", output.Provider)
		assert.Equal(t, "us-east-1", output.Region)
		assert.Equal(t, "ubuntu", output.SSHUser)
		assert.Len(t, provider.nodes, 1)

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_CreateNode_NoSecurityGroup(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "existing-key",
				},
			},
		}

		provider := NewAWSProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		nodeConfig := &config.NodeConfig{
			Name:   "test-node-1",
			Roles:  []string{"worker"},
			Size:   "t3.medium",
			Region: "us-east-1",
		}

		output, err := provider.CreateNode(ctx, nodeConfig)
		assert.Error(t, err)
		assert.Nil(t, output)
		assert.Contains(t, err.Error(), "security group must be created")

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_CreateNode_SpotInstance(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "existing-key",
					VPC: &config.VPCConfig{
						Create: true,
						CIDR:   "10.0.0.0/16",
					},
				},
			},
			Security: config.SecurityConfig{
				SSHConfig: config.SSHConfig{
					KeyPath: "/path/to/key",
				},
			},
		}

		provider := NewAWSProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		// Create network
		_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{
			Mode: "vpc",
			CIDR: "10.0.0.0/16",
		})
		assert.NoError(t, err)

		// Create firewall
		err = provider.CreateFirewall(ctx, &config.FirewallConfig{Name: "test"}, nil)
		assert.NoError(t, err)

		// Create spot instance
		nodeConfig := &config.NodeConfig{
			Name:         "spot-worker-1",
			Roles:        []string{"worker"},
			Size:         "t3.medium",
			Image:        "ubuntu-22-04",
			Region:       "us-east-1",
			WireGuardIP:  "10.8.0.30",
			SpotInstance: true,
		}

		output, err := provider.CreateNode(ctx, nodeConfig)
		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, "spot-worker-1", output.Name)

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_CreateNodePool(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "existing-key",
					VPC: &config.VPCConfig{
						Create: true,
						CIDR:   "10.0.0.0/16",
					},
				},
			},
			Security: config.SecurityConfig{
				SSHConfig: config.SSHConfig{
					KeyPath: "/path/to/key",
				},
			},
		}

		provider := NewAWSProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		// Create network
		_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{
			Mode: "vpc",
			CIDR: "10.0.0.0/16",
		})
		assert.NoError(t, err)

		// Create firewall
		err = provider.CreateFirewall(ctx, &config.FirewallConfig{Name: "test"}, nil)
		assert.NoError(t, err)

		// Create node pool
		pool := &config.NodePool{
			Name:   "worker-pool",
			Count:  3,
			Roles:  []string{"worker"},
			Size:   "t3.medium",
			Image:  "ubuntu-22-04",
			Labels: map[string]string{"pool": "workers"},
		}

		outputs, err := provider.CreateNodePool(ctx, pool)
		assert.NoError(t, err)
		assert.Len(t, outputs, 3)

		for i, output := range outputs {
			assert.Equal(t, "aws", output.Provider)
			assert.Equal(t, "us-east-1", output.Region)
			assert.Contains(t, output.Name, "worker-pool")
			// Check WireGuard IPs are assigned
			assert.NotEmpty(t, output.WireGuardIP)
			assert.Contains(t, output.WireGuardIP, "10.8.0.3")
			_ = i
		}

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_CreateNodePool_Masters(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "existing-key",
					VPC: &config.VPCConfig{
						Create: true,
						CIDR:   "10.0.0.0/16",
					},
				},
			},
			Security: config.SecurityConfig{
				SSHConfig: config.SSHConfig{
					KeyPath: "/path/to/key",
				},
			},
		}

		provider := NewAWSProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		// Create network
		_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{
			Mode: "vpc",
			CIDR: "10.0.0.0/16",
		})
		assert.NoError(t, err)

		// Create firewall
		err = provider.CreateFirewall(ctx, &config.FirewallConfig{Name: "test"}, nil)
		assert.NoError(t, err)

		// Create master pool
		pool := &config.NodePool{
			Name:  "master-pool",
			Count: 3,
			Roles: []string{"master"},
			Size:  "t3.large",
			Image: "ubuntu-22-04",
		}

		outputs, err := provider.CreateNodePool(ctx, pool)
		assert.NoError(t, err)
		assert.Len(t, outputs, 3)

		// Check master WireGuard IPs (10.8.0.20, 10.8.0.21, 10.8.0.22)
		for i, output := range outputs {
			assert.Contains(t, output.WireGuardIP, "10.8.0.2")
			_ = i
		}

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_CreateLoadBalancer(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "existing-key",
					VPC: &config.VPCConfig{
						Create: true,
						CIDR:   "10.0.0.0/16",
					},
				},
			},
			Security: config.SecurityConfig{
				SSHConfig: config.SSHConfig{
					KeyPath: "/path/to/key",
				},
			},
		}

		provider := NewAWSProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		// Create network
		_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{
			Mode: "vpc",
			CIDR: "10.0.0.0/16",
		})
		assert.NoError(t, err)

		// Create firewall
		err = provider.CreateFirewall(ctx, &config.FirewallConfig{Name: "test"}, nil)
		assert.NoError(t, err)

		// Create some nodes
		nodeConfig := &config.NodeConfig{
			Name:        "master-1",
			Roles:       []string{"master"},
			Size:        "t3.medium",
			WireGuardIP: "10.8.0.20",
			Labels:      map[string]string{"role": "master"},
		}
		_, err = provider.CreateNode(ctx, nodeConfig)
		assert.NoError(t, err)

		// Create load balancer
		lbConfig := &config.LoadBalancerConfig{
			Name: "k8s-api-lb",
			Type: "network",
			Ports: []config.PortConfig{
				{Port: 6443, TargetPort: 6443, Protocol: "TCP"},
			},
		}

		output, err := provider.CreateLoadBalancer(ctx, lbConfig)
		assert.NoError(t, err)
		assert.NotNil(t, output)

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_CreateLoadBalancer_NoVPC(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "existing-key",
				},
			},
		}

		provider := NewAWSProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		lbConfig := &config.LoadBalancerConfig{
			Name: "test-lb",
		}

		output, err := provider.CreateLoadBalancer(ctx, lbConfig)
		assert.Error(t, err)
		assert.Nil(t, output)
		assert.Contains(t, err.Error(), "VPC and subnets must be created")

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_Cleanup(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "existing-key",
				},
			},
		}

		provider := NewAWSProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		err = provider.Cleanup(ctx)
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_GetUbuntuAMI(t *testing.T) {
	tests := []struct {
		region   string
		expected string
	}{
		{"us-east-1", "ami-0c7217cdde317cfec"},
		{"us-west-2", "ami-008fe2fc65df48dac"},
		{"eu-west-1", "ami-0905a3c97561e0b69"},
		{"ap-southeast-1", "ami-078c1149d8ad719a7"},
		{"unknown-region", "ami-0c7217cdde317cfec"}, // defaults to us-east-1
	}

	for _, tt := range tests {
		t.Run("AMI_"+tt.region, func(t *testing.T) {
			provider := NewAWSProvider()
			provider.config = &config.AWSProvider{
				Region: tt.region,
			}

			ami := provider.getUbuntuAMI("")
			assert.Equal(t, tt.expected, ami)
		})
	}
}

func TestAWSProvider_GenerateUserData(t *testing.T) {
	provider := NewAWSProvider()
	provider.config = &config.AWSProvider{
		Region: "us-east-1",
	}

	nodeConfig := &config.NodeConfig{
		Name:        "test-node",
		Region:      "us-east-1",
		WireGuardIP: "10.8.0.20",
		Roles:       []string{"master", "etcd"},
		Labels:      map[string]string{"env": "prod", "tier": "control-plane"},
	}

	userData := provider.generateUserData(nodeConfig)

	// Verify user data contains expected elements
	assert.Contains(t, userData, "NODE_NAME=test-node")
	assert.Contains(t, userData, "NODE_PROVIDER=aws")
	assert.Contains(t, userData, "NODE_REGION=us-east-1")
	assert.Contains(t, userData, "WIREGUARD_IP=10.8.0.20")
	assert.Contains(t, userData, "NODE_ROLE_master=true")
	assert.Contains(t, userData, "NODE_ROLE_etcd=true")
	assert.Contains(t, userData, "NODE_LABEL_env=prod")
	assert.Contains(t, userData, "apt-get update")
	assert.Contains(t, userData, "wireguard")
	assert.Contains(t, userData, "docker-ce")
	assert.Contains(t, userData, "kubectl")
	assert.Contains(t, userData, "swapoff")
	assert.Contains(t, userData, "ip_forward")
}

func TestAWSProvider_GenerateUserData_CustomUserData(t *testing.T) {
	provider := NewAWSProvider()
	provider.config = &config.AWSProvider{
		Region: "us-east-1",
	}

	nodeConfig := &config.NodeConfig{
		Name:        "custom-node",
		Region:      "us-east-1",
		WireGuardIP: "10.8.0.30",
		Roles:       []string{"worker"},
		UserData:    "echo 'Custom setup'",
	}

	userData := provider.generateUserData(nodeConfig)

	assert.Contains(t, userData, "echo 'Custom setup'")
}

func TestAWSProvider_Regions(t *testing.T) {
	regions := []string{
		"us-east-1", "us-east-2", "us-west-1", "us-west-2",
		"eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1",
		"ap-southeast-1", "ap-southeast-2", "ap-northeast-1",
		"sa-east-1",
	}

	for _, region := range regions {
		t.Run("Region_"+region, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						AWS: &config.AWSProvider{
							Enabled: true,
							Region:  region,
							KeyPair: "existing-key",
						},
					},
				}

				provider := NewAWSProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)
				assert.Equal(t, region, provider.config.Region)

				return nil
			}, pulumi.WithMocks("project", "stack", awsMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAWSProvider_InstanceTypes(t *testing.T) {
	instanceTypes := []string{
		"t2.micro", "t2.small", "t2.medium", "t2.large",
		"t3.micro", "t3.small", "t3.medium", "t3.large",
		"m5.large", "m5.xlarge", "m5.2xlarge",
		"c5.large", "c5.xlarge",
	}

	for _, instanceType := range instanceTypes {
		t.Run("InstanceType_"+instanceType, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						AWS: &config.AWSProvider{
							Enabled: true,
							Region:  "us-east-1",
							KeyPair: "existing-key",
							VPC: &config.VPCConfig{
								Create: true,
								CIDR:   "10.0.0.0/16",
							},
						},
					},
					Security: config.SecurityConfig{
						SSHConfig: config.SSHConfig{
							KeyPath: "/path/to/key",
						},
					},
				}

				provider := NewAWSProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)

				// Create network and firewall
				_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{CIDR: "10.0.0.0/16"})
				assert.NoError(t, err)
				err = provider.CreateFirewall(ctx, &config.FirewallConfig{Name: "test"}, nil)
				assert.NoError(t, err)

				nodeConfig := &config.NodeConfig{
					Name:        "test-" + instanceType,
					Roles:       []string{"worker"},
					Size:        instanceType,
					WireGuardIP: "10.8.0.30",
				}

				output, err := provider.CreateNode(ctx, nodeConfig)
				assert.NoError(t, err)
				assert.NotNil(t, output)
				assert.Equal(t, instanceType, output.Size)

				return nil
			}, pulumi.WithMocks("project", "stack", awsMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAWSProvider_ReadSSHPublicKey(t *testing.T) {
	// Test with non-existent file
	_, err := readSSHPublicKey("/nonexistent/path/key.pub")
	assert.Error(t, err)
}

func TestAWSProvider_ImplementsInterface(t *testing.T) {
	provider := NewAWSProvider()

	// Verify it implements Provider interface
	var _ Provider = provider
}

func TestAWSProvider_MultipleNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "existing-key",
					VPC: &config.VPCConfig{
						Create: true,
						CIDR:   "10.0.0.0/16",
					},
				},
			},
			Security: config.SecurityConfig{
				SSHConfig: config.SSHConfig{
					KeyPath: "/path/to/key",
				},
			},
		}

		provider := NewAWSProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		// Create network
		_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{CIDR: "10.0.0.0/16"})
		assert.NoError(t, err)

		// Create firewall
		err = provider.CreateFirewall(ctx, &config.FirewallConfig{Name: "test"}, nil)
		assert.NoError(t, err)

		// Create multiple nodes
		for i := 0; i < 5; i++ {
			nodeConfig := &config.NodeConfig{
				Name:        "node-" + string(rune('a'+i)),
				Roles:       []string{"worker"},
				Size:        "t3.medium",
				WireGuardIP: "10.8.0." + string(rune('3'+i)) + "0",
			}

			output, err := provider.CreateNode(ctx, nodeConfig)
			assert.NoError(t, err)
			assert.NotNil(t, output)
		}

		assert.Len(t, provider.nodes, 5)

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}
