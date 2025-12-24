package providers

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// Massive test suite for AWS Provider - 100+ tests

func TestAWSProvider_InitializeVariations(t *testing.T) {
	testCases := []struct {
		name        string
		config      *config.AWSProvider
		shouldError bool
		errorMsg    string
	}{
		{"Valid_USEast1", &config.AWSProvider{Enabled: true, Region: "us-east-1", KeyPair: "key1"}, false, ""},
		{"Valid_USEast2", &config.AWSProvider{Enabled: true, Region: "us-east-2", KeyPair: "key1"}, false, ""},
		{"Valid_USWest1", &config.AWSProvider{Enabled: true, Region: "us-west-1", KeyPair: "key1"}, false, ""},
		{"Valid_USWest2", &config.AWSProvider{Enabled: true, Region: "us-west-2", KeyPair: "key1"}, false, ""},
		{"Valid_EUWest1", &config.AWSProvider{Enabled: true, Region: "eu-west-1", KeyPair: "key1"}, false, ""},
		{"Valid_EUWest2", &config.AWSProvider{Enabled: true, Region: "eu-west-2", KeyPair: "key1"}, false, ""},
		{"Valid_EUWest3", &config.AWSProvider{Enabled: true, Region: "eu-west-3", KeyPair: "key1"}, false, ""},
		{"Valid_EUCentral1", &config.AWSProvider{Enabled: true, Region: "eu-central-1", KeyPair: "key1"}, false, ""},
		{"Valid_APSoutheast1", &config.AWSProvider{Enabled: true, Region: "ap-southeast-1", KeyPair: "key1"}, false, ""},
		{"Valid_APSoutheast2", &config.AWSProvider{Enabled: true, Region: "ap-southeast-2", KeyPair: "key1"}, false, ""},
		{"Valid_APNortheast1", &config.AWSProvider{Enabled: true, Region: "ap-northeast-1", KeyPair: "key1"}, false, ""},
		{"Valid_APNortheast2", &config.AWSProvider{Enabled: true, Region: "ap-northeast-2", KeyPair: "key1"}, false, ""},
		{"Valid_APSouth1", &config.AWSProvider{Enabled: true, Region: "ap-south-1", KeyPair: "key1"}, false, ""},
		{"Valid_SAEast1", &config.AWSProvider{Enabled: true, Region: "sa-east-1", KeyPair: "key1"}, false, ""},
		{"Valid_CACentral1", &config.AWSProvider{Enabled: true, Region: "ca-central-1", KeyPair: "key1"}, false, ""},
		{"Disabled", &config.AWSProvider{Enabled: false}, true, "not enabled"},
		{"NoRegion", &config.AWSProvider{Enabled: true, Region: ""}, true, "region is required"},
		{"NilConfig", nil, true, "not enabled"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						AWS: tc.config,
					},
				}

				provider := NewAWSProvider()
				err := provider.Initialize(ctx, clusterConfig)

				if tc.shouldError {
					assert.Error(t, err)
					if tc.errorMsg != "" {
						assert.Contains(t, err.Error(), tc.errorMsg)
					}
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tc.config.Region, provider.config.Region)
				}

				return nil
			}, pulumi.WithMocks("project", "stack", awsMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAWSProvider_EC2InstanceTypes(t *testing.T) {
	instanceTypes := []string{
		// T2 family
		"t2.micro", "t2.small", "t2.medium", "t2.large", "t2.xlarge",
		// T3 family
		"t3.micro", "t3.small", "t3.medium", "t3.large", "t3.xlarge",
		// M5 family
		"m5.large", "m5.xlarge", "m5.2xlarge", "m5.4xlarge",
		// C5 family
		"c5.large", "c5.xlarge", "c5.2xlarge", "c5.4xlarge",
		// R5 family
		"r5.large", "r5.xlarge", "r5.2xlarge",
	}

	for _, instanceType := range instanceTypes {
		t.Run("InstanceType_"+instanceType, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						AWS: &config.AWSProvider{
							Enabled: true,
							Region:  "us-east-1",
							KeyPair: "test-key",
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
					Name:        fmt.Sprintf("node-%s", strings.ReplaceAll(instanceType, ".", "-")),
					Provider:    "aws",
					Region:      "us-east-1",
					Size:        instanceType,
					Image:       "ubuntu-22-04",
					Roles:       []string{"worker"},
					WireGuardIP: "10.8.0.30",
				}

				node, err := provider.CreateNode(ctx, nodeConfig)
				assert.NoError(t, err)
				assert.Equal(t, instanceType, node.Size)

				return nil
			}, pulumi.WithMocks("project", "stack", awsMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAWSProvider_AMIMapping(t *testing.T) {
	regions := []struct {
		region      string
		expectedAMI string
	}{
		{"us-east-1", "ami-0c7217cdde317cfec"},
		{"us-east-2", "ami-0e83be366243f524a"},
		{"us-west-1", "ami-0ce2cb35386fc22e9"},
		{"us-west-2", "ami-008fe2fc65df48dac"},
		{"eu-west-1", "ami-0905a3c97561e0b69"},
		{"eu-west-2", "ami-0e5f882be1900e43b"},
		{"eu-west-3", "ami-01d21b7be69801c2f"},
		{"eu-central-1", "ami-0faab6bdbac9486fb"},
		{"ap-southeast-1", "ami-078c1149d8ad719a7"},
		{"ap-southeast-2", "ami-04f5097681773b989"},
		{"ap-northeast-1", "ami-07c589821f2b353aa"},
		{"ap-northeast-2", "ami-0c9c942bd7bf113a2"},
		{"ap-south-1", "ami-03f4878755434977f"},
		{"sa-east-1", "ami-0fb4cf3a99aa89f72"},
		{"ca-central-1", "ami-0a2e7efb4257c0907"},
		{"unknown-region", "ami-0c7217cdde317cfec"}, // Should default to us-east-1
	}

	for _, tc := range regions {
		t.Run("AMI_"+tc.region, func(t *testing.T) {
			provider := NewAWSProvider()
			provider.config = &config.AWSProvider{
				Region: tc.region,
			}

			ami := provider.getUbuntuAMI("")
			assert.Equal(t, tc.expectedAMI, ami)
		})
	}
}

func TestAWSProvider_NodeRoleCombinations(t *testing.T) {
	roleCombinations := []struct {
		name  string
		roles []string
	}{
		{"Master", []string{"master"}},
		{"Worker", []string{"worker"}},
		{"Etcd", []string{"etcd"}},
		{"MasterEtcd", []string{"master", "etcd"}},
		{"MasterWorker", []string{"master", "worker"}},
		{"WorkerEtcd", []string{"worker", "etcd"}},
		{"All", []string{"master", "worker", "etcd"}},
		{"ControlPlane", []string{"controlplane"}},
		{"ControlPlaneEtcd", []string{"controlplane", "etcd"}},
	}

	for _, tc := range roleCombinations {
		t.Run("Roles_"+tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						AWS: &config.AWSProvider{
							Enabled: true,
							Region:  "us-east-1",
							KeyPair: "test-key",
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

				// Setup
				_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{CIDR: "10.0.0.0/16"})
				assert.NoError(t, err)
				err = provider.CreateFirewall(ctx, &config.FirewallConfig{Name: "test"}, nil)
				assert.NoError(t, err)

				nodeConfig := &config.NodeConfig{
					Name:        fmt.Sprintf("node-%s", tc.name),
					Provider:    "aws",
					Region:      "us-east-1",
					Size:        "t3.medium",
					Image:       "ubuntu-22-04",
					Roles:       tc.roles,
					WireGuardIP: "10.8.0.20",
				}

				node, err := provider.CreateNode(ctx, nodeConfig)
				assert.NoError(t, err)
				assert.NotNil(t, node)

				return nil
			}, pulumi.WithMocks("project", "stack", awsMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAWSProvider_NodeLabels(t *testing.T) {
	labelSets := []map[string]string{
		{"environment": "production"},
		{"environment": "staging"},
		{"environment": "development"},
		{"tier": "frontend"},
		{"tier": "backend"},
		{"tier": "database"},
		{"app": "nginx"},
		{"app": "postgres"},
		{"app": "redis"},
		{"team": "devops"},
		{"team": "platform"},
		{"cost-center": "engineering"},
		{"environment": "production", "tier": "frontend", "app": "web"},
		{"environment": "staging", "tier": "backend", "version": "v1.0"},
		{"kubernetes.io/role": "master", "node.kubernetes.io/instance-type": "t3.large"},
	}

	for i, labels := range labelSets {
		t.Run(fmt.Sprintf("Labels_%d", i), func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						AWS: &config.AWSProvider{
							Enabled: true,
							Region:  "us-east-1",
							KeyPair: "test-key",
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

				_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{CIDR: "10.0.0.0/16"})
				assert.NoError(t, err)
				err = provider.CreateFirewall(ctx, &config.FirewallConfig{Name: "test"}, nil)
				assert.NoError(t, err)

				nodeConfig := &config.NodeConfig{
					Name:        fmt.Sprintf("node-labels-%d", i),
					Provider:    "aws",
					Region:      "us-east-1",
					Size:        "t3.medium",
					Image:       "ubuntu-22-04",
					Roles:       []string{"worker"},
					Labels:      labels,
					WireGuardIP: "10.8.0.30",
				}

				node, err := provider.CreateNode(ctx, nodeConfig)
				assert.NoError(t, err)
				assert.Equal(t, labels, node.Labels)

				return nil
			}, pulumi.WithMocks("project", "stack", awsMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAWSProvider_VPCConfigurations(t *testing.T) {
	vpcConfigs := []struct {
		name    string
		vpcCIDR string
	}{
		{"Small_24", "10.0.0.0/24"},
		{"Medium_20", "10.0.0.0/20"},
		{"Large_16", "10.0.0.0/16"},
		{"XLarge_12", "10.0.0.0/12"},
		{"Range_172", "172.16.0.0/16"},
		{"Range_192", "192.168.0.0/16"},
		{"Custom_10_1", "10.1.0.0/16"},
		{"Custom_10_2", "10.2.0.0/16"},
	}

	for _, tc := range vpcConfigs {
		t.Run("VPC_"+tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						AWS: &config.AWSProvider{
							Enabled: true,
							Region:  "us-east-1",
							KeyPair: "test-key",
							VPC: &config.VPCConfig{
								Create: true,
								Name:   fmt.Sprintf("vpc-%s", tc.name),
								CIDR:   tc.vpcCIDR,
							},
						},
					},
				}

				provider := NewAWSProvider()
				err := provider.Initialize(ctx, clusterConfig)
				assert.NoError(t, err)
				assert.Equal(t, tc.vpcCIDR, provider.config.VPC.CIDR)

				return nil
			}, pulumi.WithMocks("project", "stack", awsMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAWSProvider_NodePoolSizes(t *testing.T) {
	poolSizes := []int{1, 2, 3, 5, 10}

	for _, size := range poolSizes {
		t.Run(fmt.Sprintf("PoolSize_%d", size), func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						AWS: &config.AWSProvider{
							Enabled: true,
							Region:  "us-east-1",
							KeyPair: "test-key",
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

				_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{CIDR: "10.0.0.0/16"})
				assert.NoError(t, err)
				err = provider.CreateFirewall(ctx, &config.FirewallConfig{Name: "test"}, nil)
				assert.NoError(t, err)

				pool := &config.NodePool{
					Name:  fmt.Sprintf("pool-%d", size),
					Count: size,
					Roles: []string{"worker"},
					Size:  "t3.medium",
					Image: "ubuntu-22-04",
				}

				outputs, err := provider.CreateNodePool(ctx, pool)
				assert.NoError(t, err)
				assert.Len(t, outputs, size)

				return nil
			}, pulumi.WithMocks("project", "stack", awsMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAWSProvider_SpotInstanceVariations(t *testing.T) {
	testCases := []struct {
		name         string
		spotInstance bool
		spotMaxPrice string
	}{
		{"OnDemand", false, ""},
		{"Spot_NoPriceLimit", true, ""},
		{"Spot_WithPriceLimit", true, "0.05"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						AWS: &config.AWSProvider{
							Enabled: true,
							Region:  "us-east-1",
							KeyPair: "test-key",
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

				_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{CIDR: "10.0.0.0/16"})
				assert.NoError(t, err)
				err = provider.CreateFirewall(ctx, &config.FirewallConfig{Name: "test"}, nil)
				assert.NoError(t, err)

				nodeConfig := &config.NodeConfig{
					Name:         fmt.Sprintf("node-%s", tc.name),
					Provider:     "aws",
					Region:       "us-east-1",
					Size:         "t3.medium",
					Image:        "ubuntu-22-04",
					Roles:        []string{"worker"},
					SpotInstance: tc.spotInstance,
					SpotMaxPrice: tc.spotMaxPrice,
					WireGuardIP:  "10.8.0.30",
				}

				node, err := provider.CreateNode(ctx, nodeConfig)
				assert.NoError(t, err)
				assert.NotNil(t, node)

				return nil
			}, pulumi.WithMocks("project", "stack", awsMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAWSProvider_UserDataGeneration(t *testing.T) {
	testCases := []struct {
		name        string
		nodeConfig  *config.NodeConfig
		shouldMatch []string
	}{
		{
			name: "BasicMaster",
			nodeConfig: &config.NodeConfig{
				Name:        "master-1",
				Region:      "us-east-1",
				WireGuardIP: "10.8.0.20",
				Roles:       []string{"master"},
			},
			shouldMatch: []string{"NODE_NAME=master-1", "NODE_ROLE_master=true", "WIREGUARD_IP=10.8.0.20"},
		},
		{
			name: "BasicWorker",
			nodeConfig: &config.NodeConfig{
				Name:        "worker-1",
				Region:      "us-west-2",
				WireGuardIP: "10.8.0.30",
				Roles:       []string{"worker"},
			},
			shouldMatch: []string{"NODE_NAME=worker-1", "NODE_ROLE_worker=true", "us-west-2"},
		},
		{
			name: "MultiRole",
			nodeConfig: &config.NodeConfig{
				Name:        "multi-1",
				Region:      "eu-west-1",
				WireGuardIP: "10.8.0.40",
				Roles:       []string{"master", "etcd", "worker"},
			},
			shouldMatch: []string{"NODE_ROLE_master=true", "NODE_ROLE_etcd=true", "NODE_ROLE_worker=true"},
		},
		{
			name: "WithLabels",
			nodeConfig: &config.NodeConfig{
				Name:        "labeled-1",
				Region:      "us-east-1",
				WireGuardIP: "10.8.0.50",
				Roles:       []string{"worker"},
				Labels:      map[string]string{"env": "prod", "tier": "frontend"},
			},
			shouldMatch: []string{"NODE_LABEL_env=prod", "NODE_LABEL_tier=frontend"},
		},
		{
			name: "WithCustomUserData",
			nodeConfig: &config.NodeConfig{
				Name:        "custom-1",
				Region:      "us-east-1",
				WireGuardIP: "10.8.0.60",
				Roles:       []string{"worker"},
				UserData:    "echo 'Custom initialization script'",
			},
			shouldMatch: []string{"Custom initialization script"},
		},
		{
			name: "EssentialPackages",
			nodeConfig: &config.NodeConfig{
				Name:        "packages-1",
				Region:      "us-east-1",
				WireGuardIP: "10.8.0.70",
				Roles:       []string{"worker"},
			},
			shouldMatch: []string{"apt-get update", "wireguard", "docker-ce", "kubectl", "swapoff", "ip_forward"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := NewAWSProvider()
			provider.config = &config.AWSProvider{
				Region: tc.nodeConfig.Region,
			}

			userData := provider.generateUserData(tc.nodeConfig)

			for _, expected := range tc.shouldMatch {
				assert.Contains(t, userData, expected, "User data should contain: %s", expected)
			}
		})
	}
}

func TestAWSProvider_FirewallRules(t *testing.T) {
	testCases := []struct {
		name           string
		firewallConfig *config.FirewallConfig
	}{
		{
			name:           "DefaultRules",
			firewallConfig: &config.FirewallConfig{Name: "default"},
		},
		{
			name: "CustomInboundRules",
			firewallConfig: &config.FirewallConfig{
				Name: "custom",
				InboundRules: []config.FirewallRule{
					{Protocol: "tcp", Port: "8080", Source: []string{"0.0.0.0/0"}},
					{Protocol: "tcp", Port: "3306", Source: []string{"10.0.0.0/8"}},
				},
			},
		},
		{
			name: "MultipleSourceCIDRs",
			firewallConfig: &config.FirewallConfig{
				Name: "multi-source",
				InboundRules: []config.FirewallRule{
					{Protocol: "tcp", Port: "443", Source: []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}},
				},
			},
		},
		{
			name: "UDPRules",
			firewallConfig: &config.FirewallConfig{
				Name: "udp",
				InboundRules: []config.FirewallRule{
					{Protocol: "udp", Port: "53", Source: []string{"0.0.0.0/0"}},
					{Protocol: "udp", Port: "123", Source: []string{"0.0.0.0/0"}},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						AWS: &config.AWSProvider{
							Enabled: true,
							Region:  "us-east-1",
							KeyPair: "test-key",
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

				_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{CIDR: "10.0.0.0/16"})
				assert.NoError(t, err)

				err = provider.CreateFirewall(ctx, tc.firewallConfig, nil)
				assert.NoError(t, err)
				assert.NotNil(t, provider.securityGroup)

				return nil
			}, pulumi.WithMocks("project", "stack", awsMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAWSProvider_SubnetDistribution(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "test-key",
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

		output, err := provider.CreateNetwork(ctx, &config.NetworkConfig{CIDR: "10.0.0.0/16"})
		assert.NoError(t, err)
		assert.Len(t, output.Subnets, 2)
		assert.Equal(t, "10.0.1.0/24", output.Subnets[0].CIDR)
		assert.Equal(t, "10.0.2.0/24", output.Subnets[1].CIDR)

		// Verify zones
		assert.Equal(t, "us-east-1a", output.Subnets[0].Zone)
		assert.Equal(t, "us-east-1b", output.Subnets[1].Zone)

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_NodePoolZoneDistribution(t *testing.T) {
	testCases := []struct {
		name      string
		poolCount int
		zones     []string
	}{
		{"SingleZone", 3, []string{"us-east-1a"}},
		{"TwoZones", 4, []string{"us-east-1a", "us-east-1b"}},
		{"ThreeZones", 6, []string{"us-east-1a", "us-east-1b", "us-east-1c"}},
		{"DefaultZones", 5, nil}, // Should use default zones
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						AWS: &config.AWSProvider{
							Enabled: true,
							Region:  "us-east-1",
							KeyPair: "test-key",
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

				_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{CIDR: "10.0.0.0/16"})
				assert.NoError(t, err)
				err = provider.CreateFirewall(ctx, &config.FirewallConfig{Name: "test"}, nil)
				assert.NoError(t, err)

				pool := &config.NodePool{
					Name:  "zone-test-pool",
					Count: tc.poolCount,
					Roles: []string{"worker"},
					Size:  "t3.medium",
					Zones: tc.zones,
				}

				outputs, err := provider.CreateNodePool(ctx, pool)
				assert.NoError(t, err)
				assert.Len(t, outputs, tc.poolCount)

				return nil
			}, pulumi.WithMocks("project", "stack", awsMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAWSProvider_WireGuardIPAssignment(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "test-key",
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

		_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{CIDR: "10.0.0.0/16"})
		assert.NoError(t, err)
		err = provider.CreateFirewall(ctx, &config.FirewallConfig{Name: "test"}, nil)
		assert.NoError(t, err)

		// Create master pool
		masterPool := &config.NodePool{
			Name:  "masters",
			Count: 3,
			Roles: []string{"master"},
			Size:  "t3.large",
		}
		masterOutputs, err := provider.CreateNodePool(ctx, masterPool)
		assert.NoError(t, err)

		// Verify master WireGuard IPs (10.8.0.20, 10.8.0.21, 10.8.0.22)
		for i, output := range masterOutputs {
			expectedIP := fmt.Sprintf("10.8.0.%d", 20+i)
			assert.Equal(t, expectedIP, output.WireGuardIP)
		}

		// Clear nodes for worker pool test
		provider.nodes = nil

		// Create worker pool
		workerPool := &config.NodePool{
			Name:  "workers",
			Count: 5,
			Roles: []string{"worker"},
			Size:  "t3.medium",
		}
		workerOutputs, err := provider.CreateNodePool(ctx, workerPool)
		assert.NoError(t, err)

		// Verify worker WireGuard IPs (10.8.0.30, 10.8.0.31, ...)
		for i, output := range workerOutputs {
			expectedIP := fmt.Sprintf("10.8.0.%d", 30+i)
			assert.Equal(t, expectedIP, output.WireGuardIP)
		}

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_MixedNodePoolsWithSpot(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "test-key",
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

		_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{CIDR: "10.0.0.0/16"})
		assert.NoError(t, err)
		err = provider.CreateFirewall(ctx, &config.FirewallConfig{Name: "test"}, nil)
		assert.NoError(t, err)

		// Create on-demand masters
		masterPool := &config.NodePool{
			Name:         "masters",
			Count:        3,
			Roles:        []string{"master", "etcd"},
			Size:         "t3.large",
			SpotInstance: false,
		}
		masterOutputs, err := provider.CreateNodePool(ctx, masterPool)
		assert.NoError(t, err)
		assert.Len(t, masterOutputs, 3)

		// Create spot workers
		workerPool := &config.NodePool{
			Name:         "spot-workers",
			Count:        5,
			Roles:        []string{"worker"},
			Size:         "t3.medium",
			SpotInstance: true,
		}
		workerOutputs, err := provider.CreateNodePool(ctx, workerPool)
		assert.NoError(t, err)
		assert.Len(t, workerOutputs, 5)

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_GetSizesContainsExpectedTypes(t *testing.T) {
	provider := NewAWSProvider()
	sizes := provider.GetSizes()

	expectedSizes := []string{
		"t2.micro", "t2.small", "t2.medium", "t2.large",
		"t3.micro", "t3.small", "t3.medium", "t3.large",
		"m5.large", "m5.xlarge", "m5.2xlarge",
		"c5.large", "c5.xlarge",
		"r5.large", "r5.xlarge",
	}

	for _, expected := range expectedSizes {
		assert.Contains(t, sizes, expected, "Sizes should contain: %s", expected)
	}
}

func TestAWSProvider_GetRegionsContainsExpected(t *testing.T) {
	provider := NewAWSProvider()
	regions := provider.GetRegions()

	expectedRegions := []string{
		"us-east-1", "us-east-2", "us-west-1", "us-west-2",
		"eu-west-1", "eu-west-2", "eu-central-1",
		"ap-southeast-1", "ap-southeast-2", "ap-northeast-1",
		"sa-east-1",
	}

	for _, expected := range expectedRegions {
		assert.Contains(t, regions, expected, "Regions should contain: %s", expected)
	}
}

func TestAWSProvider_CleanupIsIdempotent(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "test-key",
				},
			},
		}

		provider := NewAWSProvider()
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		// Cleanup should be idempotent
		err = provider.Cleanup(ctx)
		assert.NoError(t, err)

		err = provider.Cleanup(ctx)
		assert.NoError(t, err)

		err = provider.Cleanup(ctx)
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_LoadBalancerWithMasterNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "test-key",
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

		_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{CIDR: "10.0.0.0/16"})
		assert.NoError(t, err)
		err = provider.CreateFirewall(ctx, &config.FirewallConfig{Name: "test"}, nil)
		assert.NoError(t, err)

		// Create master nodes
		for i := 0; i < 3; i++ {
			nodeConfig := &config.NodeConfig{
				Name:        fmt.Sprintf("master-%d", i+1),
				Roles:       []string{"master"},
				Size:        "t3.large",
				WireGuardIP: fmt.Sprintf("10.8.0.%d", 20+i),
				Labels:      map[string]string{"role": "master"},
			}
			_, err := provider.CreateNode(ctx, nodeConfig)
			assert.NoError(t, err)
		}

		// Create load balancer
		lbConfig := &config.LoadBalancerConfig{
			Name: "k8s-api",
			Type: "network",
		}

		output, err := provider.CreateLoadBalancer(ctx, lbConfig)
		assert.NoError(t, err)
		assert.NotNil(t, output)

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_SSHUserIsUbuntu(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "test-key",
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

		_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{CIDR: "10.0.0.0/16"})
		assert.NoError(t, err)
		err = provider.CreateFirewall(ctx, &config.FirewallConfig{Name: "test"}, nil)
		assert.NoError(t, err)

		nodeConfig := &config.NodeConfig{
			Name:        "test-node",
			Roles:       []string{"worker"},
			Size:        "t3.medium",
			WireGuardIP: "10.8.0.30",
		}

		output, err := provider.CreateNode(ctx, nodeConfig)
		assert.NoError(t, err)
		assert.Equal(t, "ubuntu", output.SSHUser)

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_ProviderName(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "test-key",
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

		_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{CIDR: "10.0.0.0/16"})
		assert.NoError(t, err)
		err = provider.CreateFirewall(ctx, &config.FirewallConfig{Name: "test"}, nil)
		assert.NoError(t, err)

		nodeConfig := &config.NodeConfig{
			Name:        "test-node",
			Roles:       []string{"worker"},
			Size:        "t3.medium",
			WireGuardIP: "10.8.0.30",
		}

		output, err := provider.CreateNode(ctx, nodeConfig)
		assert.NoError(t, err)
		assert.Equal(t, "aws", output.Provider)

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_NodeRegionMatch(t *testing.T) {
	regions := []string{"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1"}

	for _, region := range regions {
		t.Run("Region_"+region, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				clusterConfig := &config.ClusterConfig{
					Providers: config.ProvidersConfig{
						AWS: &config.AWSProvider{
							Enabled: true,
							Region:  region,
							KeyPair: "test-key",
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

				_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{CIDR: "10.0.0.0/16"})
				assert.NoError(t, err)
				err = provider.CreateFirewall(ctx, &config.FirewallConfig{Name: "test"}, nil)
				assert.NoError(t, err)

				nodeConfig := &config.NodeConfig{
					Name:        "test-node",
					Roles:       []string{"worker"},
					Size:        "t3.medium",
					WireGuardIP: "10.8.0.30",
				}

				output, err := provider.CreateNode(ctx, nodeConfig)
				assert.NoError(t, err)
				assert.Equal(t, region, output.Region)

				return nil
			}, pulumi.WithMocks("project", "stack", awsMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestAWSProvider_ReadSSHPublicKey_Errors(t *testing.T) {
	testCases := []struct {
		name     string
		path     string
		hasError bool
	}{
		{"NonExistentFile", "/nonexistent/path/key.pub", true},
		{"InvalidPath", "", true},
		{"DirectoryPath", "/tmp", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := readSSHPublicKey(tc.path)
			if tc.hasError {
				assert.Error(t, err)
			}
		})
	}
}

func TestAWSProvider_FullClusterDeployment(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name:        "test-cluster",
				Environment: "test",
			},
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "test-key",
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

		// Initialize
		err := provider.Initialize(ctx, clusterConfig)
		assert.NoError(t, err)

		// Create network
		networkOutput, err := provider.CreateNetwork(ctx, &config.NetworkConfig{
			Mode: "vpc",
			CIDR: "10.0.0.0/16",
		})
		assert.NoError(t, err)
		assert.NotNil(t, networkOutput)

		// Create firewall
		err = provider.CreateFirewall(ctx, &config.FirewallConfig{
			Name: "test-cluster-firewall",
			InboundRules: []config.FirewallRule{
				{Protocol: "tcp", Port: "22", Source: []string{"0.0.0.0/0"}},
				{Protocol: "tcp", Port: "6443", Source: []string{"0.0.0.0/0"}},
			},
		}, nil)
		assert.NoError(t, err)

		// Create master pool
		masterPool := &config.NodePool{
			Name:  "masters",
			Count: 3,
			Roles: []string{"master", "etcd"},
			Size:  "t3.large",
		}
		masterOutputs, err := provider.CreateNodePool(ctx, masterPool)
		assert.NoError(t, err)
		assert.Len(t, masterOutputs, 3)

		// Create worker pool
		workerPool := &config.NodePool{
			Name:  "workers",
			Count: 5,
			Roles: []string{"worker"},
			Size:  "t3.medium",
		}
		workerOutputs, err := provider.CreateNodePool(ctx, workerPool)
		assert.NoError(t, err)
		assert.Len(t, workerOutputs, 5)

		// Create load balancer
		lbOutput, err := provider.CreateLoadBalancer(ctx, &config.LoadBalancerConfig{
			Name: "k8s-api",
			Type: "network",
		})
		assert.NoError(t, err)
		assert.NotNil(t, lbOutput)

		// Cleanup
		err = provider.Cleanup(ctx)
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}

func TestAWSProvider_InterfaceCompliance(t *testing.T) {
	provider := NewAWSProvider()

	// Verify it implements Provider interface
	var _ Provider = provider

	// Verify methods exist
	assert.NotEmpty(t, provider.GetName())
	assert.NotEmpty(t, provider.GetRegions())
	assert.NotEmpty(t, provider.GetSizes())
}

func TestAWSProvider_EmptyNodePool(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Providers: config.ProvidersConfig{
				AWS: &config.AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
					KeyPair: "test-key",
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

		_, err = provider.CreateNetwork(ctx, &config.NetworkConfig{CIDR: "10.0.0.0/16"})
		assert.NoError(t, err)
		err = provider.CreateFirewall(ctx, &config.FirewallConfig{Name: "test"}, nil)
		assert.NoError(t, err)

		// Create empty pool (count = 0)
		pool := &config.NodePool{
			Name:  "empty-pool",
			Count: 0,
			Roles: []string{"worker"},
			Size:  "t3.medium",
		}

		outputs, err := provider.CreateNodePool(ctx, pool)
		assert.NoError(t, err)
		assert.Len(t, outputs, 0)

		return nil
	}, pulumi.WithMocks("project", "stack", awsMocks(0)))

	assert.NoError(t, err)
}
